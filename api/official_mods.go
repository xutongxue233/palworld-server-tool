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

var (
	inspectOfficialModsAPI         = tool.InspectOfficialMods
	planOfficialModSettingsAPI     = tool.PlanOfficialModSettings
	applyOfficialModSettingsAPI    = tool.ApplyOfficialModSettings
	rollbackOfficialModSettingsAPI = tool.RollbackOfficialModSettings
	backupOfficialModWorldAPI      = tool.BackupAndRecord
	getOfficialModDatabaseAPI      = database.GetDB
	getOfficialModControlAPI       = tool.GetServerControlStatus
	stopOfficialModServerAPI       = tool.StopServerForMaintenance
	startOfficialModServerAPI      = tool.StartManagedServer
	forceStopOfficialModServerAPI  = tool.ForceStopManagedServer
)

type OfficialModSettingsRequest struct {
	GlobalEnabled   bool     `json:"global_enabled"`
	WorkshopRootDir string   `json:"workshop_root_dir" binding:"omitempty,max=4096"`
	ActiveModList   []string `json:"active_mod_list"`
}

type OfficialModPreflightResponse struct {
	Plan          tool.OfficialModChangePlan `json:"plan"`
	ServerControl tool.ServerControlStatus   `json:"server_control"`
}

type OfficialModApplyRequest struct {
	OfficialModSettingsRequest
	ExpectedPlanDigest   string `json:"expected_plan_digest" binding:"required,len=64,hexadecimal"`
	ConfirmApply         bool   `json:"confirm_apply"`
	ConfirmModRisk       bool   `json:"confirm_mod_risk"`
	ConfirmServerStopped bool   `json:"confirm_server_stopped"`
	RestartAfter         bool   `json:"restart_after"`
	ShutdownSeconds      int    `json:"shutdown_seconds" binding:"omitempty,min=0,max=300"`
	ShutdownMessage      string `json:"shutdown_message" binding:"omitempty,max=256"`
}

type OfficialModApplyResponse struct {
	Apply                tool.OfficialModApplyResult `json:"apply"`
	SafetyBackup         *database.Backup            `json:"safety_backup,omitempty"`
	Maintenance          tool.MaintenanceStopResult  `json:"maintenance"`
	Restarted            bool                        `json:"restarted"`
	RestartError         string                      `json:"restart_error,omitempty"`
	RecoveryRestarted    bool                        `json:"recovery_restarted"`
	RecoveryRestartError string                      `json:"recovery_restart_error,omitempty"`
	RollbackError        string                      `json:"rollback_error,omitempty"`
}

type OfficialModApplyErrorResponse struct {
	Error  string                   `json:"error"`
	Code   string                   `json:"code"`
	Result OfficialModApplyResponse `json:"result"`
}

func (request OfficialModSettingsRequest) settings() tool.OfficialModSettings {
	return tool.OfficialModSettings{
		GlobalEnabled:   request.GlobalEnabled,
		WorkshopRootDir: strings.TrimSpace(request.WorkshopRootDir),
		ActiveModList:   append([]string(nil), request.ActiveModList...),
	}
}

// getOfficialMods godoc
//
//	@Summary		Inspect Palworld 1.0.0 official server mods
//	@Description	Read Mods/PalModSettings.ini, scan local Workshop Info.json packages, validate IsServer rules and dependencies, and report deployment state without downloading or executing mod content
//	@Tags			Server
//	@Produce		json
//	@Security		ApiKeyAuth
//	@Success		200	{object}	tool.OfficialModStatus
//	@Failure		401	{object}	ErrorResponse
//	@Router			/api/server/mods [get]
func getOfficialMods(c *gin.Context) {
	c.Header("Cache-Control", "no-store")
	c.JSON(http.StatusOK, inspectOfficialModsAPI())
}

// preflightOfficialModSettings godoc
//
//	@Summary		Preflight an official Palworld mod selection
//	@Description	Validate the proposed Workshop root, active PackageName list, server InstallRules, dependencies, current settings digest, save recovery readiness, and server control state without changing files
//	@Tags			Server
//	@Accept			json
//	@Produce		json
//	@Security		ApiKeyAuth
//	@Param			settings	body		OfficialModSettingsRequest	true	"Desired official mod settings"
//	@Success		200		{object}	OfficialModPreflightResponse
//	@Failure		400		{object}	ErrorResponse
//	@Failure		401		{object}	ErrorResponse
//	@Router			/api/server/mods/preflight [post]
func preflightOfficialModSettings(c *gin.Context) {
	c.Header("Cache-Control", "no-store")
	var request OfficialModSettingsRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: err.Error(), Code: "invalid_official_mod_request"})
		return
	}
	if len(request.ActiveModList) > 256 {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error: "active_mod_list cannot contain more than 256 entries",
			Code:  "invalid_official_mod_request",
		})
		return
	}
	c.JSON(http.StatusOK, OfficialModPreflightResponse{
		Plan:          planOfficialModSettingsAPI(request.settings()),
		ServerControl: getOfficialModControlAPI(c.Request.Context()),
	})
}

// applyOfficialModSettings godoc
//
//	@Summary		Apply official Palworld mod settings safely
//	@Description	Revalidate the fixed plan, stop the server, create a mandatory world restore point when saves exist, back up PalModSettings.ini, atomically replace it, optionally restart, and roll back settings if the managed restart fails
//	@Tags			Server
//	@Accept			json
//	@Produce		json
//	@Security		ApiKeyAuth
//	@Param			settings	body		OfficialModApplyRequest	true	"Confirmed official mod settings change"
//	@Success		200		{object}	OfficialModApplyResponse
//	@Failure		400		{object}	OfficialModApplyErrorResponse
//	@Failure		401		{object}	ErrorResponse
//	@Failure		409		{object}	OfficialModApplyErrorResponse
//	@Failure		422		{object}	OfficialModApplyErrorResponse
//	@Failure		500		{object}	OfficialModApplyErrorResponse
//	@Router			/api/server/mods/apply [post]
func applyOfficialModSettings(c *gin.Context) {
	c.Header("Cache-Control", "no-store")
	var request OfficialModApplyRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		writeOfficialModError(c, err, "invalid_official_mod_request", OfficialModApplyResponse{})
		return
	}
	if len(request.ActiveModList) > 256 {
		writeOfficialModError(
			c,
			errors.New("active_mod_list cannot contain more than 256 entries"),
			"invalid_official_mod_request",
			OfficialModApplyResponse{},
		)
		return
	}
	if !request.ConfirmApply {
		writeOfficialModError(c, errors.New("confirm_apply is required"), "official_mod_confirmation", OfficialModApplyResponse{})
		return
	}
	if !request.ConfirmModRisk {
		writeOfficialModError(
			c,
			errors.New("confirm_mod_risk is required because server mods can crash the server or corrupt save data"),
			"official_mod_risk_confirmation",
			OfficialModApplyResponse{},
		)
		return
	}

	desired := request.settings()
	plan := planOfficialModSettingsAPI(desired)
	if !plan.Status.Configured {
		writeOfficialModError(c, tool.ErrOfficialModsNotConfigured, "", OfficialModApplyResponse{})
		return
	}
	if !plan.Status.Supported {
		writeOfficialModError(c, tool.ErrOfficialModsUnsupported, "", OfficialModApplyResponse{})
		return
	}
	if !plan.CanApply {
		writeOfficialModError(
			c,
			fmt.Errorf("%w: %s", tool.ErrOfficialModsInvalid, joinOfficialModPlanIssues(plan.Issues)),
			"",
			OfficialModApplyResponse{},
		)
		return
	}
	if !strings.EqualFold(request.ExpectedPlanDigest, plan.PlanDigest) {
		writeOfficialModError(c, tool.ErrOfficialModPlanChanged, "", OfficialModApplyResponse{})
		return
	}
	if !plan.Changed {
		c.JSON(http.StatusOK, OfficialModApplyResponse{
			Apply: tool.OfficialModApplyResult{Plan: plan, Status: plan.Status},
		})
		return
	}

	controlStatus := getOfficialModControlAPI(c.Request.Context())
	if request.RestartAfter && !controlStatus.Configured {
		writeOfficialModError(
			c,
			errors.New("restart_after requires configured Palworld server control"),
			"server_control_not_configured",
			OfficialModApplyResponse{},
		)
		return
	}
	if (controlStatus.Online || controlStatus.Running) && !controlStatus.Configured {
		writeOfficialModError(
			c,
			errors.New("stop PalServer manually before changing mods, or configure managed server control"),
			"server_control_not_configured",
			OfficialModApplyResponse{},
		)
		return
	}

	release, ok := beginManualOperation(c, nil)
	if !ok {
		return
	}
	defer release()
	plan = planOfficialModSettingsAPI(desired)
	if !strings.EqualFold(request.ExpectedPlanDigest, plan.PlanDigest) {
		writeOfficialModError(c, tool.ErrOfficialModPlanChanged, "", OfficialModApplyResponse{})
		return
	}

	priorDesired, hadDesired := task.GetWatchdogDesiredRunning()
	task.SetWatchdogDesiredRunning(false)
	operationCtx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
	defer cancel()
	response := OfficialModApplyResponse{}
	if strings.TrimSpace(request.ShutdownMessage) == "" {
		request.ShutdownMessage = "Palworld Server Tool is applying official server mod settings"
	}

	if controlStatus.Online || controlStatus.Running {
		maintenance, err := stopOfficialModServerAPI(
			operationCtx,
			request.ShutdownSeconds,
			request.ShutdownMessage,
		)
		response.Maintenance = maintenance
		if err != nil {
			restoreOfficialModServer(maintenance, priorDesired, hadDesired, &response)
			writeOfficialModError(c, err, "", response)
			return
		}
	}
	if err := tool.ConfirmGameServerStopped(operationCtx, request.ConfirmServerStopped); err != nil {
		restoreOfficialModServer(response.Maintenance, priorDesired, hadDesired, &response)
		writeOfficialModError(c, err, "", response)
		return
	}

	if plan.SafetyBackupRequired {
		backup, err := backupOfficialModWorldAPI(getOfficialModDatabaseAPI())
		if err != nil {
			restoreOfficialModServer(response.Maintenance, priorDesired, hadDesired, &response)
			writeOfficialModError(c, fmt.Errorf("create pre-mod safety backup: %w", err), "official_mod_safety_backup_failed", response)
			return
		}
		response.SafetyBackup = &backup
	}
	if err := tool.ConfirmGameServerStopped(operationCtx, request.ConfirmServerStopped); err != nil {
		restoreOfficialModServer(response.Maintenance, priorDesired, hadDesired, &response)
		writeOfficialModError(c, err, "", response)
		return
	}

	applyResult, err := applyOfficialModSettingsAPI(operationCtx, tool.OfficialModApplyOptions{
		DesiredSettings:    desired,
		ExpectedPlanDigest: request.ExpectedPlanDigest,
		ConfirmServerStop:  request.ConfirmServerStopped,
	})
	response.Apply = applyResult
	if err != nil {
		if !applyResult.Changed || applyResult.RolledBack {
			restoreOfficialModServer(response.Maintenance, priorDesired, hadDesired, &response)
		} else {
			task.SetWatchdogDesiredRunning(false)
		}
		writeOfficialModError(c, err, "", response)
		return
	}

	if request.RestartAfter {
		task.SetWatchdogDesiredRunning(true)
		if err := startOfficialModServerAPI(operationCtx); err != nil {
			response.RestartError = err.Error()
			recoverOfficialModRestartFailure(err, &response)
			code := "official_mod_restart_failed"
			if response.Apply.RolledBack && response.RecoveryRestarted {
				code = "official_mod_restart_failed_rolled_back"
			}
			writeOfficialModError(c, err, code, response)
			return
		}
		response.Restarted = true
		task.NotifyAutomationEvent(
			task.EventServerRestarted,
			"Palworld official mods updated",
			fmt.Sprintf("Applied %d official server mod package selections.", len(applyResult.Status.Settings.ActiveModList)),
			map[string]any{
				"active_mods":    applyResult.Status.Settings.ActiveModList,
				"global_enabled": applyResult.Status.Settings.GlobalEnabled,
			},
		)
	} else {
		task.SetWatchdogDesiredRunning(false)
	}
	c.JSON(http.StatusOK, response)
}

func recoverOfficialModRestartFailure(startErr error, response *OfficialModApplyResponse) {
	task.SetWatchdogDesiredRunning(false)
	recoveryCtx, cancel := context.WithTimeout(context.Background(), 15*time.Minute)
	defer cancel()
	status := getOfficialModControlAPI(recoveryCtx)
	if status.Online || status.Running {
		if err := forceStopOfficialModServerAPI(recoveryCtx); err != nil {
			response.RollbackError = "stop failed mod runtime before rollback: " + err.Error()
			return
		}
	}
	if err := tool.ConfirmGameServerStopped(recoveryCtx, true); err != nil {
		response.RollbackError = "confirm stopped server before rollback: " + err.Error()
		return
	}
	if err := rollbackOfficialModSettingsAPI(recoveryCtx, &response.Apply, true); err != nil {
		response.RollbackError = err.Error()
		return
	}
	task.SetWatchdogDesiredRunning(true)
	if err := startOfficialModServerAPI(recoveryCtx); err != nil {
		response.RecoveryRestartError = fmt.Sprintf("original mod settings were restored after %v, but the server still failed to start: %v", startErr, err)
		task.SetWatchdogDesiredRunning(false)
		return
	}
	response.RecoveryRestarted = true
}

func restoreOfficialModServer(
	maintenance tool.MaintenanceStopResult,
	priorDesired bool,
	hadDesired bool,
	response *OfficialModApplyResponse,
) {
	if maintenance.WasRunning && maintenance.CanRestart {
		task.SetWatchdogDesiredRunning(true)
		recoveryCtx, cancel := context.WithTimeout(context.Background(), 15*time.Minute)
		defer cancel()
		status := getOfficialModControlAPI(recoveryCtx)
		if status.Online || status.Running {
			return
		}
		if err := startOfficialModServerAPI(recoveryCtx); err != nil {
			response.RecoveryRestartError = err.Error()
		} else {
			response.RecoveryRestarted = true
		}
		return
	}
	if hadDesired {
		task.SetWatchdogDesiredRunning(priorDesired)
	}
}

func writeOfficialModError(
	c *gin.Context,
	err error,
	code string,
	result OfficialModApplyResponse,
) {
	status := http.StatusBadRequest
	if code == "" {
		code = "official_mod_apply_failed"
	}
	switch {
	case errors.Is(err, tool.ErrOfficialModsNotConfigured):
		status = http.StatusUnprocessableEntity
		code = "official_mod_not_configured"
	case errors.Is(err, tool.ErrOfficialModsUnsupported):
		status = http.StatusUnprocessableEntity
		code = "official_mod_platform_unsupported"
	case errors.Is(err, tool.ErrOfficialModsInvalid):
		status = http.StatusUnprocessableEntity
		code = "official_mod_preflight_failed"
	case errors.Is(err, tool.ErrOfficialModPlanChanged):
		status = http.StatusConflict
		code = "official_mod_plan_changed"
	case errors.Is(err, tool.ErrSaveEditConfirmation):
		status = http.StatusConflict
		code = "official_mod_server_stop_confirmation"
	case errors.Is(err, tool.ErrGameServerRunning):
		status = http.StatusConflict
		code = "game_server_running"
	case errors.Is(err, tool.ErrGameServerStatusUnknown):
		status = http.StatusConflict
		code = "game_server_status_unknown"
	case errors.Is(err, tool.ErrOfficialModApplyFailed),
		errors.Is(err, tool.ErrOfficialModRollbackFailed):
		status = http.StatusInternalServerError
	case strings.Contains(code, "confirmation"),
		code == "server_control_not_configured":
		status = http.StatusConflict
	case code == "official_mod_restart_failed",
		code == "official_mod_restart_failed_rolled_back",
		code == "official_mod_safety_backup_failed":
		status = http.StatusInternalServerError
	}
	c.JSON(status, OfficialModApplyErrorResponse{Error: err.Error(), Code: code, Result: result})
}

func joinOfficialModPlanIssues(issues []tool.OfficialModDiagnostic) string {
	messages := make([]string, 0, len(issues))
	for _, issue := range issues {
		message := issue.Message
		if issue.PackageName != "" {
			message = issue.PackageName + ": " + message
		}
		if issue.Dependency != "" {
			message += " (dependency: " + issue.Dependency + ")"
		}
		messages = append(messages, message)
	}
	return strings.Join(messages, "; ")
}
