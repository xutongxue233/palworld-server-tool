package api

import (
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/zaigie/palworld-server-tool/internal/database"
	"github.com/zaigie/palworld-server-tool/internal/tool"
)

type WorldOptionSyncRequest struct {
	Content         string `json:"content" binding:"required"`
	ExpectedSHA256  string `json:"expected_sha256" binding:"omitempty,len=64,hexadecimal"`
	ConfirmSync     bool   `json:"confirm_sync"`
	ShutdownSeconds int    `json:"shutdown_seconds" binding:"omitempty,min=0,max=300"`
	ShutdownMessage string `json:"shutdown_message" binding:"omitempty,max=256"`
}

type WorldOptionSyncResponse struct {
	WorldOption  tool.WorldOptionMutation   `json:"world_option"`
	SafetyBackup database.Backup            `json:"safety_backup"`
	Maintenance  tool.MaintenanceStopResult `json:"maintenance"`
	Restarted    bool                       `json:"restarted"`
	RestartError string                     `json:"restart_error,omitempty"`
}

// syncWorldOption godoc
//
//	@Summary		Generate or synchronize WorldOption.sav
//	@Description	Convert validated Palworld 1.0.0 INI settings into a WorldOption.sav, stop the server, create a safety backup, atomically install the validated file, and restart a previously running managed server
//	@Tags			Server
//	@Accept			json
//	@Produce		json
//	@Security		ApiKeyAuth
//	@Param			settings	body		WorldOptionSyncRequest	true	"WorldOption settings"
//	@Success		200			{object}	WorldOptionSyncResponse
//	@Failure		400			{object}	ErrorResponse
//	@Failure		401			{object}	ErrorResponse
//	@Failure		409			{object}	ErrorResponse
//	@Failure		422			{object}	ErrorResponse
//	@Router			/api/server/world-option [put]
func syncWorldOption(c *gin.Context) {
	var req WorldOptionSyncRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: err.Error(), Code: "invalid_world_option_request"})
		return
	}
	if !req.ConfirmSync {
		writeWorldOptionError(c, tool.ErrSaveEditConfirmation)
		return
	}
	if err := tool.ValidateWorldOptionSync(req.Content, req.ExpectedSHA256); err != nil {
		writeWorldOptionError(c, err)
		return
	}
	controlStatus := tool.GetServerControlStatus(c.Request.Context())
	if (controlStatus.Online || controlStatus.Running) && !controlStatus.Configured {
		c.JSON(http.StatusConflict, ErrorResponse{
			Error: "stop PalServer manually before synchronizing WorldOption.sav, or configure managed server control",
			Code:  "server_control_not_configured",
		})
		return
	}
	if strings.TrimSpace(req.ShutdownMessage) == "" {
		req.ShutdownMessage = "Palworld Server Tool is synchronizing WorldOption.sav"
	}

	maintenance, err := tool.StopServerForMaintenance(
		c.Request.Context(),
		req.ShutdownSeconds,
		req.ShutdownMessage,
	)
	if err != nil {
		if maintenance.WasRunning && maintenance.CanRestart {
			if restartErr := recoverPreviouslyRunningServer(c); restartErr != nil {
				err = fmt.Errorf("%w; additionally failed to recover the previously running server: %v", err, restartErr)
			}
		}
		writeWorldOptionError(c, err)
		return
	}

	result, err := tool.SyncWorldOption(c.Request.Context(), database.GetDB(), tool.WorldOptionSyncOptions{
		ConfigContent:        req.Content,
		ExpectedSHA256:       req.ExpectedSHA256,
		ConfirmServerStopped: true,
	})
	if err != nil {
		if maintenance.WasRunning && maintenance.CanRestart {
			if restartErr := recoverPreviouslyRunningServer(c); restartErr != nil {
				err = fmt.Errorf("%w; additionally failed to restart the previously running server: %v", err, restartErr)
			}
		}
		writeWorldOptionError(c, err)
		return
	}

	response := WorldOptionSyncResponse{
		WorldOption:  result.WorldOption,
		SafetyBackup: result.SafetyBackup,
		Maintenance:  maintenance,
	}
	if maintenance.WasRunning && maintenance.CanRestart {
		if err := tool.StartManagedServer(c.Request.Context()); err != nil {
			response.RestartError = err.Error()
		} else {
			response.Restarted = true
		}
	}
	c.JSON(http.StatusOK, response)
}

func writeWorldOptionError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, tool.ErrSaveEditConfirmation):
		c.JSON(http.StatusConflict, ErrorResponse{Error: err.Error(), Code: "world_option_confirmation"})
	case errors.Is(err, tool.ErrWorldOptionConflict):
		c.JSON(http.StatusConflict, ErrorResponse{Error: err.Error(), Code: "world_option_changed"})
	case errors.Is(err, tool.ErrGameServerRunning):
		c.JSON(http.StatusConflict, ErrorResponse{Error: err.Error(), Code: "game_server_running"})
	case errors.Is(err, tool.ErrNativeBackupNotConfigured), errors.Is(err, tool.ErrUnsupportedSaveSource):
		c.JSON(http.StatusUnprocessableEntity, ErrorResponse{Error: err.Error(), Code: "world_option_not_configured"})
	case errors.Is(err, tool.ErrSaveEditInternal):
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: err.Error(), Code: "world_option_internal"})
	default:
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: err.Error(), Code: "world_option_sync_failed"})
	}
}
