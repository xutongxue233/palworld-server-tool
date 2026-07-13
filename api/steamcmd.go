package api

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/zaigie/palworld-server-tool/internal/database"
	"github.com/zaigie/palworld-server-tool/internal/task"
	"github.com/zaigie/palworld-server-tool/internal/tool"
)

type SteamCMDStatusResponse struct {
	Plan          tool.SteamCMDPlan        `json:"plan"`
	ServerControl tool.ServerControlStatus `json:"server_control"`
}

type SteamCMDUpdateRequest struct {
	ExpectedPlanDigest   string `json:"expected_plan_digest" binding:"required,len=64,hexadecimal"`
	ConfirmUpdate        bool   `json:"confirm_update"`
	ConfirmServerStopped bool   `json:"confirm_server_stopped"`
	ValidateFiles        *bool  `json:"validate_files"`
	RestartAfter         bool   `json:"restart_after"`
	ShutdownSeconds      int    `json:"shutdown_seconds" binding:"omitempty,min=0,max=300"`
	ShutdownMessage      string `json:"shutdown_message" binding:"omitempty,max=256"`
}

type SteamCMDUpdateResponse struct {
	Update       tool.SteamCMDUpdateResult  `json:"update"`
	SafetyBackup *database.Backup           `json:"safety_backup,omitempty"`
	Maintenance  tool.MaintenanceStopResult `json:"maintenance"`
	Restarted    bool                       `json:"restarted"`
	RestartError string                     `json:"restart_error,omitempty"`
}

type SteamCMDUpdateErrorResponse struct {
	Error  string                 `json:"error"`
	Code   string                 `json:"code"`
	Result SteamCMDUpdateResponse `json:"result"`
}

// getSteamCMDStatus godoc
//
//	@Summary		Inspect the restricted Palworld SteamCMD install/update plan
//	@Description	Validate fixed App ID 2394010 paths, manifest, launcher, save-backup readiness, and managed server state without running SteamCMD
//	@Tags			Server
//	@Produce		json
//	@Security		ApiKeyAuth
//	@Success		200	{object}	SteamCMDStatusResponse
//	@Failure		401	{object}	ErrorResponse
//	@Router			/api/server/steamcmd [get]
func getSteamCMDStatus(c *gin.Context) {
	c.Header("Cache-Control", "no-store")
	c.JSON(http.StatusOK, SteamCMDStatusResponse{
		Plan:          tool.InspectSteamCMD(),
		ServerControl: tool.GetServerControlStatus(c.Request.Context()),
	})
}

// updateServerWithSteamCMD godoc
//
//	@Summary		Install or update Palworld Dedicated Server with SteamCMD
//	@Description	Revalidate a fixed App ID 2394010 plan, stop the server, create a mandatory save restore point when world data exists, run SteamCMD without a shell, verify the manifest and launcher, and optionally restart
//	@Tags			Server
//	@Accept			json
//	@Produce		json
//	@Security		ApiKeyAuth
//	@Param			update	body		SteamCMDUpdateRequest	true	"Confirmed SteamCMD operation"
//	@Success		200		{object}	SteamCMDUpdateResponse
//	@Failure		400		{object}	SteamCMDUpdateErrorResponse
//	@Failure		401		{object}	ErrorResponse
//	@Failure		409		{object}	SteamCMDUpdateErrorResponse
//	@Failure		422		{object}	SteamCMDUpdateErrorResponse
//	@Router			/api/server/steamcmd/update [post]
func updateServerWithSteamCMD(c *gin.Context) {
	c.Header("Cache-Control", "no-store")
	var req SteamCMDUpdateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, SteamCMDUpdateErrorResponse{Error: err.Error(), Code: "invalid_steamcmd_request"})
		return
	}
	if !req.ConfirmUpdate {
		c.JSON(http.StatusConflict, SteamCMDUpdateErrorResponse{
			Error: tool.ErrSaveEditConfirmation.Error(),
			Code:  "steamcmd_confirmation",
		})
		return
	}
	validateFiles := true
	if req.ValidateFiles != nil {
		validateFiles = *req.ValidateFiles
	}
	plan := tool.InspectSteamCMD()
	if !plan.Configured {
		writeSteamCMDError(c, tool.ErrSteamCMDNotConfigured, SteamCMDUpdateResponse{})
		return
	}
	if !plan.CanExecute {
		writeSteamCMDError(c, fmt.Errorf("%w: %s", tool.ErrSteamCMDInvalid, strings.Join(plan.Issues, "; ")), SteamCMDUpdateResponse{})
		return
	}
	if !strings.EqualFold(req.ExpectedPlanDigest, plan.PlanDigest) {
		writeSteamCMDError(c, tool.ErrSteamCMDPlanChanged, SteamCMDUpdateResponse{})
		return
	}

	controlStatus := tool.GetServerControlStatus(c.Request.Context())
	if req.RestartAfter && !controlStatus.Configured {
		writeSteamCMDError(c, errors.New("restart_after requires configured Palworld server control"), SteamCMDUpdateResponse{})
		return
	}
	if (controlStatus.Online || controlStatus.Running) && !controlStatus.Configured {
		writeSteamCMDError(c, errors.New("stop PalServer manually before running SteamCMD, or configure managed server control"), SteamCMDUpdateResponse{})
		return
	}

	release, ok := beginManualOperation(c, nil)
	if !ok {
		return
	}
	defer release()
	plan = tool.InspectSteamCMD()
	if !strings.EqualFold(req.ExpectedPlanDigest, plan.PlanDigest) {
		writeSteamCMDError(c, tool.ErrSteamCMDPlanChanged, SteamCMDUpdateResponse{})
		return
	}

	priorDesired, hadDesired := task.GetWatchdogDesiredRunning()
	task.SetWatchdogDesiredRunning(false)
	operationCtx, cancel := context.WithTimeout(
		context.Background(),
		time.Duration(plan.TimeoutSeconds+1800)*time.Second,
	)
	defer cancel()
	response := SteamCMDUpdateResponse{}
	if strings.TrimSpace(req.ShutdownMessage) == "" {
		req.ShutdownMessage = "Palworld Server Tool is installing a Palworld server update"
	}

	if controlStatus.Online || controlStatus.Running {
		maintenance, err := tool.StopServerForMaintenance(
			operationCtx,
			req.ShutdownSeconds,
			req.ShutdownMessage,
		)
		response.Maintenance = maintenance
		if err != nil {
			restoreSteamCMDServer(maintenance, priorDesired, hadDesired, &response)
			writeSteamCMDError(c, err, response)
			return
		}
	}
	if err := tool.ConfirmGameServerStopped(operationCtx, req.ConfirmServerStopped); err != nil {
		restoreSteamCMDServer(response.Maintenance, priorDesired, hadDesired, &response)
		writeSteamCMDError(c, err, response)
		return
	}

	if plan.SafetyBackupRequired {
		backup, err := tool.BackupAndRecord(database.GetDB())
		if err != nil {
			restoreSteamCMDServer(response.Maintenance, priorDesired, hadDesired, &response)
			writeSteamCMDError(c, fmt.Errorf("create pre-update safety backup: %w", err), response)
			return
		}
		response.SafetyBackup = &backup
	}
	if err := tool.ConfirmGameServerStopped(operationCtx, req.ConfirmServerStopped); err != nil {
		restoreSteamCMDServer(response.Maintenance, priorDesired, hadDesired, &response)
		writeSteamCMDError(c, err, response)
		return
	}

	update, err := tool.RunSteamCMDUpdate(operationCtx, tool.SteamCMDUpdateOptions{
		ExpectedPlanDigest: req.ExpectedPlanDigest,
		ValidateFiles:      validateFiles,
	})
	response.Update = update
	if err != nil {
		restoreSteamCMDServer(response.Maintenance, priorDesired, hadDesired, &response)
		writeSteamCMDError(c, err, response)
		return
	}

	if req.RestartAfter {
		task.SetWatchdogDesiredRunning(true)
		if err := tool.StartManagedServer(operationCtx); err != nil {
			response.RestartError = err.Error()
		} else {
			response.Restarted = true
			task.NotifyAutomationEvent(
				task.EventServerRestarted,
				"Palworld server updated",
				fmt.Sprintf("SteamCMD installed Palworld build %s.", update.BuildIDAfter),
				map[string]any{"build_id": update.BuildIDAfter, "changed": update.Changed},
			)
		}
	} else {
		task.SetWatchdogDesiredRunning(false)
	}
	c.JSON(http.StatusOK, response)
}

func restoreSteamCMDServer(
	maintenance tool.MaintenanceStopResult,
	priorDesired bool,
	hadDesired bool,
	response *SteamCMDUpdateResponse,
) {
	if maintenance.WasRunning && maintenance.CanRestart {
		task.SetWatchdogDesiredRunning(true)
		recoveryCtx, cancel := context.WithTimeout(context.Background(), 15*time.Minute)
		defer cancel()
		status := tool.GetServerControlStatus(recoveryCtx)
		if status.Online || status.Running {
			return
		}
		if err := tool.StartManagedServer(recoveryCtx); err != nil {
			response.RestartError = err.Error()
		} else {
			response.Restarted = true
		}
		return
	}
	if hadDesired {
		task.SetWatchdogDesiredRunning(priorDesired)
	}
}

func writeSteamCMDError(c *gin.Context, err error, result SteamCMDUpdateResponse) {
	status := http.StatusBadRequest
	code := "steamcmd_update_failed"
	switch {
	case errors.Is(err, tool.ErrSteamCMDNotConfigured):
		status = http.StatusUnprocessableEntity
		code = "steamcmd_not_configured"
	case errors.Is(err, tool.ErrSteamCMDInvalid):
		status = http.StatusUnprocessableEntity
		code = "steamcmd_preflight_failed"
	case errors.Is(err, tool.ErrSteamCMDPlanChanged):
		status = http.StatusConflict
		code = "steamcmd_plan_changed"
	case errors.Is(err, tool.ErrSaveEditConfirmation):
		status = http.StatusConflict
		code = "steamcmd_server_stop_confirmation"
	case errors.Is(err, tool.ErrGameServerRunning):
		status = http.StatusConflict
		code = "game_server_running"
	case errors.Is(err, tool.ErrGameServerStatusUnknown):
		status = http.StatusConflict
		code = "game_server_status_unknown"
	case strings.Contains(err.Error(), "server control"):
		status = http.StatusConflict
		code = "server_control_not_configured"
	}
	c.JSON(status, SteamCMDUpdateErrorResponse{Error: err.Error(), Code: code, Result: result})
}
