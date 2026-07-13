package api

import (
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/zaigie/palworld-server-tool/internal/database"
	"github.com/zaigie/palworld-server-tool/internal/task"
	"github.com/zaigie/palworld-server-tool/internal/tool"
)

type NativeBackupListResponse struct {
	NativeBackups tool.NativeBackupCatalog `json:"native_backups"`
	ServerControl tool.ServerControlStatus `json:"server_control"`
}

type NativeBackupRestoreRequest struct {
	ExpectedDigest  string `json:"expected_digest" binding:"required,len=64,hexadecimal"`
	ConfirmRestore  bool   `json:"confirm_restore"`
	RestartAfter    bool   `json:"restart_after"`
	ShutdownSeconds int    `json:"shutdown_seconds" binding:"omitempty,min=0,max=300"`
	ShutdownMessage string `json:"shutdown_message" binding:"omitempty,max=256"`
}

type NativeBackupRestoreResponse struct {
	RestoredBackup tool.NativeBackup          `json:"restored_backup"`
	SafetyBackup   database.Backup            `json:"safety_backup"`
	Maintenance    tool.MaintenanceStopResult `json:"maintenance"`
	SyncError      string                     `json:"sync_error,omitempty"`
	Restarted      bool                       `json:"restarted"`
	RestartError   string                     `json:"restart_error,omitempty"`
}

// listNativeBackups godoc
//
//	@Summary		List Palworld native world backups
//	@Description	Discover and validate backups created by Palworld under backup/world without copying them into PST storage
//	@Tags			backup
//	@Produce		json
//	@Security		ApiKeyAuth
//	@Success		200	{object}	NativeBackupListResponse
//	@Failure		401	{object}	ErrorResponse
//	@Failure		500	{object}	ErrorResponse
//	@Router			/api/server/backups/native [get]
func listNativeBackups(c *gin.Context) {
	catalog, err := tool.ListNativeBackups()
	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: err.Error(), Code: "native_backup_list_failed"})
		return
	}
	c.JSON(http.StatusOK, NativeBackupListResponse{
		NativeBackups: catalog,
		ServerControl: tool.GetServerControlStatus(c.Request.Context()),
	})
}

// restoreNativeBackup godoc
//
//	@Summary		Safely restore a Palworld native world backup
//	@Description	Stop the managed server when possible, create a PST safety backup, validate a selected native backup, atomically restore it, synchronize decoded data, and optionally restart the server
//	@Tags			backup
//	@Accept			json
//	@Produce		json
//	@Security		ApiKeyAuth
//	@Param			backup_id	path		string						true	"Native backup directory ID"
//	@Param			restore		body		NativeBackupRestoreRequest	true	"Restore confirmation and selected snapshot digest"
//	@Success		200			{object}	NativeBackupRestoreResponse
//	@Failure		400			{object}	ErrorResponse
//	@Failure		401			{object}	ErrorResponse
//	@Failure		409			{object}	ErrorResponse
//	@Failure		422			{object}	ErrorResponse
//	@Router			/api/server/backups/native/{backup_id}/restore [post]
func restoreNativeBackup(c *gin.Context) {
	var req NativeBackupRestoreRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: err.Error(), Code: "invalid_restore_request"})
		return
	}
	if !req.ConfirmRestore {
		writeNativeBackupError(c, tool.ErrSaveEditConfirmation)
		return
	}

	controlStatus := tool.GetServerControlStatus(c.Request.Context())
	if req.RestartAfter && !controlStatus.Configured {
		c.JSON(http.StatusConflict, ErrorResponse{
			Error: "restart_after requires configured Palworld server control",
			Code:  "server_control_not_configured",
		})
		return
	}
	if strings.TrimSpace(req.ShutdownMessage) == "" {
		req.ShutdownMessage = "Palworld Server Tool is restoring a native world backup"
	}
	if _, err := tool.ValidateNativeBackupSelection(c.Param("backup_id"), req.ExpectedDigest); err != nil {
		writeNativeBackupError(c, err)
		return
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
		writeNativeBackupError(c, err)
		return
	}

	result, err := tool.RestoreNativeBackup(c.Request.Context(), database.GetDB(), tool.NativeBackupRestoreOptions{
		BackupID:             c.Param("backup_id"),
		ExpectedDigest:       req.ExpectedDigest,
		ConfirmServerStopped: true,
	})
	if err != nil {
		if maintenance.WasRunning && maintenance.CanRestart {
			if restartErr := recoverPreviouslyRunningServer(c); restartErr != nil {
				err = fmt.Errorf("%w; additionally failed to restart the previously running server: %v", err, restartErr)
			}
		}
		writeNativeBackupError(c, err)
		return
	}

	response := NativeBackupRestoreResponse{
		RestoredBackup: result.RestoredBackup,
		SafetyBackup:   result.SafetyBackup,
		Maintenance:    maintenance,
	}
	if err := task.SavSyncNow(); err != nil {
		response.SyncError = err.Error()
	}
	if req.RestartAfter {
		if err := tool.StartManagedServer(c.Request.Context()); err != nil {
			response.RestartError = err.Error()
		} else {
			response.Restarted = true
		}
	}
	c.JSON(http.StatusOK, response)
}

func recoverPreviouslyRunningServer(c *gin.Context) error {
	status := tool.GetServerControlStatus(c.Request.Context())
	if status.Online || status.Running {
		return nil
	}
	return tool.StartManagedServer(c.Request.Context())
}

func writeNativeBackupError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, tool.ErrSaveEditConfirmation):
		c.JSON(http.StatusConflict, ErrorResponse{Error: err.Error(), Code: "native_backup_confirmation"})
	case errors.Is(err, tool.ErrGameServerRunning):
		c.JSON(http.StatusConflict, ErrorResponse{Error: err.Error(), Code: "game_server_running"})
	case errors.Is(err, tool.ErrNativeBackupChanged):
		c.JSON(http.StatusConflict, ErrorResponse{Error: err.Error(), Code: "native_backup_changed"})
	case errors.Is(err, tool.ErrNativeBackupNotConfigured), errors.Is(err, tool.ErrUnsupportedSaveSource):
		c.JSON(http.StatusUnprocessableEntity, ErrorResponse{Error: err.Error(), Code: "native_backup_not_configured"})
	case errors.Is(err, tool.ErrNativeBackupInvalid):
		c.JSON(http.StatusUnprocessableEntity, ErrorResponse{Error: err.Error(), Code: "native_backup_invalid"})
	case errors.Is(err, tool.ErrServerControlNotConfigured):
		c.JSON(http.StatusConflict, ErrorResponse{Error: err.Error(), Code: "server_control_not_configured"})
	default:
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: err.Error(), Code: "native_backup_restore_failed"})
	}
}
