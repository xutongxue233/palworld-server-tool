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

const saveMigrationOperationTimeout = 30 * time.Minute

type SaveMigrationPreflightRequest struct {
	SourcePath     string `json:"source_path" binding:"required,max=4096"`
	SourcePlatform string `json:"source_platform" binding:"required,oneof=current windows linux"`
	SourceKind     string `json:"source_kind" binding:"required,oneof=dedicated coop"`
}

type SaveMigrationPreflightResponse struct {
	Plan          tool.SaveMigrationPlan   `json:"plan"`
	ServerControl tool.ServerControlStatus `json:"server_control"`
}

type SaveMigrationApplyRequest struct {
	SourcePath           string `json:"source_path" binding:"required,max=4096"`
	SourcePlatform       string `json:"source_platform" binding:"required,oneof=current windows linux"`
	SourceKind           string `json:"source_kind" binding:"required,oneof=dedicated coop"`
	ExpectedPlanDigest   string `json:"expected_plan_digest" binding:"required,len=64,hexadecimal"`
	ConfirmMigration     bool   `json:"confirm_migration"`
	ConfirmServerStopped bool   `json:"confirm_server_stopped"`
	RestartAfter         bool   `json:"restart_after"`
	ShutdownSeconds      int    `json:"shutdown_seconds" binding:"omitempty,min=0,max=300"`
	ShutdownMessage      string `json:"shutdown_message" binding:"omitempty,max=256"`
}

type SaveMigrationApplyResponse struct {
	Migration    tool.SaveMigrationResult   `json:"migration"`
	Maintenance  tool.MaintenanceStopResult `json:"maintenance"`
	SyncError    string                     `json:"sync_error,omitempty"`
	Restarted    bool                       `json:"restarted"`
	RestartError string                     `json:"restart_error,omitempty"`
}

type SaveMigrationErrorResponse struct {
	Error  string                     `json:"error"`
	Code   string                     `json:"code"`
	Result SaveMigrationApplyResponse `json:"result"`
}

// preflightSaveMigration godoc
//
//	@Summary		Preflight a local Palworld dedicated-server save migration
//	@Description	Resolve and validate a same-platform Palworld 1.0.0 dedicated-server world without modifying either source or destination. Co-op host GUID conversion and cross-platform identity conversion are intentionally blocked.
//	@Tags			Server
//	@Accept			json
//	@Produce		json
//	@Security		ApiKeyAuth
//	@Param			preflight	body		SaveMigrationPreflightRequest	true	"Local migration source"
//	@Success		200			{object}	SaveMigrationPreflightResponse
//	@Failure		400			{object}	ErrorResponse
//	@Failure		401			{object}	ErrorResponse
//	@Router			/api/server/migration/preflight [post]
func preflightSaveMigration(c *gin.Context) {
	c.Header("Cache-Control", "no-store")
	var req SaveMigrationPreflightRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: err.Error(), Code: "invalid_save_migration_request"})
		return
	}
	c.JSON(http.StatusOK, SaveMigrationPreflightResponse{
		Plan: tool.InspectSaveMigration(
			c.Request.Context(),
			req.SourcePath,
			req.SourcePlatform,
			req.SourceKind,
		),
		ServerControl: tool.GetServerControlStatus(c.Request.Context()),
	})
}

// applySaveMigration godoc
//
//	@Summary		Safely migrate a local Palworld dedicated-server world
//	@Description	Revalidate the Palworld 1.0.0 plan, stop the server, create a mandatory PST safety backup, stage and validate the source, atomically replace only Level.sav, LevelMeta.sav, Players, and optional WorldOption.sav, synchronize decoded data, and optionally restart.
//	@Tags			Server
//	@Accept			json
//	@Produce		json
//	@Security		ApiKeyAuth
//	@Param			migration	body		SaveMigrationApplyRequest	true	"Confirmed local save migration"
//	@Success		200			{object}	SaveMigrationApplyResponse
//	@Failure		400			{object}	SaveMigrationErrorResponse
//	@Failure		401			{object}	ErrorResponse
//	@Failure		409			{object}	SaveMigrationErrorResponse
//	@Failure		422			{object}	SaveMigrationErrorResponse
//	@Router			/api/server/migration/apply [post]
func applySaveMigration(c *gin.Context) {
	c.Header("Cache-Control", "no-store")
	var req SaveMigrationApplyRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, SaveMigrationErrorResponse{Error: err.Error(), Code: "invalid_save_migration_request"})
		return
	}
	if !req.ConfirmMigration {
		writeSaveMigrationError(c, tool.ErrSaveEditConfirmation, SaveMigrationApplyResponse{})
		return
	}

	plan := tool.InspectSaveMigration(c.Request.Context(), req.SourcePath, req.SourcePlatform, req.SourceKind)
	if !plan.Configured {
		writeSaveMigrationError(c, tool.ErrSaveMigrationNotConfigured, SaveMigrationApplyResponse{})
		return
	}
	if plan.CoopHostDetected || hasSaveMigrationIssue(plan, "coop_source_unsupported") {
		writeSaveMigrationError(c, tool.ErrSaveMigrationCoopHost, SaveMigrationApplyResponse{})
		return
	}
	if !plan.CanMigrate {
		writeSaveMigrationError(c, fmt.Errorf("%w: %s", tool.ErrSaveMigrationInvalid, saveMigrationIssueText(plan)), SaveMigrationApplyResponse{})
		return
	}
	if !strings.EqualFold(req.ExpectedPlanDigest, plan.PlanDigest) {
		writeSaveMigrationError(c, tool.ErrSaveMigrationChanged, SaveMigrationApplyResponse{})
		return
	}

	controlStatus := tool.GetServerControlStatus(c.Request.Context())
	if req.RestartAfter && !controlStatus.Configured {
		writeSaveMigrationError(c, errors.New("restart_after requires configured Palworld server control"), SaveMigrationApplyResponse{})
		return
	}
	release, ok := beginManualOperation(c, nil)
	if !ok {
		return
	}
	defer release()

	plan = tool.InspectSaveMigration(c.Request.Context(), req.SourcePath, req.SourcePlatform, req.SourceKind)
	if !plan.CanMigrate || !strings.EqualFold(req.ExpectedPlanDigest, plan.PlanDigest) {
		writeSaveMigrationError(c, tool.ErrSaveMigrationChanged, SaveMigrationApplyResponse{})
		return
	}

	priorDesired, hadDesired := task.GetWatchdogDesiredRunning()
	task.SetWatchdogDesiredRunning(false)
	operationCtx, cancel := context.WithTimeout(context.Background(), saveMigrationOperationTimeout)
	defer cancel()
	response := SaveMigrationApplyResponse{}
	if strings.TrimSpace(req.ShutdownMessage) == "" {
		req.ShutdownMessage = "Palworld Server Tool is migrating the local Palworld world save"
	}

	maintenance, err := tool.StopServerForMaintenance(operationCtx, req.ShutdownSeconds, req.ShutdownMessage)
	response.Maintenance = maintenance
	if err != nil {
		recoverSaveMigrationServer(maintenance, priorDesired, hadDesired, &response)
		writeSaveMigrationError(c, saveMigrationRecoveryError(err, response), response)
		return
	}
	confirmedStopped := req.ConfirmServerStopped || controlStatus.Configured || maintenance.WasRunning
	result, err := tool.ApplySaveMigration(operationCtx, database.GetDB(), tool.SaveMigrationOptions{
		SourcePath:           req.SourcePath,
		SourcePlatform:       req.SourcePlatform,
		SourceKind:           req.SourceKind,
		ExpectedPlanDigest:   req.ExpectedPlanDigest,
		ConfirmMigration:     true,
		ConfirmServerStopped: confirmedStopped,
	})
	response.Migration = result
	if err != nil {
		recoverSaveMigrationServer(maintenance, priorDesired, hadDesired, &response)
		writeSaveMigrationError(c, saveMigrationRecoveryError(err, response), response)
		return
	}

	if err := task.SavSyncNow(); err != nil {
		response.SyncError = err.Error()
	}
	if req.RestartAfter {
		task.SetWatchdogDesiredRunning(true)
		restartCtx, restartCancel := context.WithTimeout(context.Background(), 15*time.Minute)
		defer restartCancel()
		if err := tool.StartManagedServer(restartCtx); err != nil {
			response.RestartError = err.Error()
		} else {
			response.Restarted = true
			task.NotifyAutomationEvent(
				task.EventServerRestarted,
				"Palworld save migration completed",
				fmt.Sprintf("Migrated local world %s into %s.", result.Plan.SourceWorldID, result.Plan.DestinationWorldID),
				map[string]any{"source_world_id": result.Plan.SourceWorldID, "destination_world_id": result.Plan.DestinationWorldID},
			)
		}
	} else {
		task.SetWatchdogDesiredRunning(false)
	}
	c.JSON(http.StatusOK, response)
}

func recoverSaveMigrationServer(
	maintenance tool.MaintenanceStopResult,
	priorDesired bool,
	hadDesired bool,
	response *SaveMigrationApplyResponse,
) {
	if maintenance.WasRunning {
		recoveryCtx, cancel := context.WithTimeout(context.Background(), 15*time.Minute)
		defer cancel()
		status := tool.GetServerControlStatus(recoveryCtx)
		if status.Online || status.Running {
			return
		}
		if maintenance.CanRestart {
			task.SetWatchdogDesiredRunning(true)
			if err := tool.StartManagedServer(recoveryCtx); err != nil {
				response.RestartError = err.Error()
			} else {
				response.Restarted = true
			}
			return
		}
		response.RestartError = "the server was running before migration and is now offline; start it manually because managed server control is not configured"
	}
	if hadDesired {
		task.SetWatchdogDesiredRunning(priorDesired)
	}
}

func saveMigrationRecoveryError(err error, response SaveMigrationApplyResponse) error {
	if strings.TrimSpace(response.RestartError) == "" {
		return err
	}
	return fmt.Errorf("%w; server recovery: %s", err, response.RestartError)
}

func writeSaveMigrationError(c *gin.Context, err error, result SaveMigrationApplyResponse) {
	status := http.StatusBadRequest
	code := "save_migration_apply_failed"
	switch {
	case errors.Is(err, tool.ErrSaveEditConfirmation):
		status = http.StatusConflict
		code = "save_migration_confirmation"
	case errors.Is(err, tool.ErrSaveMigrationChanged):
		status = http.StatusConflict
		code = "save_migration_plan_changed"
	case errors.Is(err, tool.ErrSaveMigrationCoopHost):
		status = http.StatusUnprocessableEntity
		code = "save_migration_coop_unsupported"
	case errors.Is(err, tool.ErrSaveMigrationNotConfigured), errors.Is(err, tool.ErrUnsupportedSaveSource):
		status = http.StatusUnprocessableEntity
		code = "save_migration_not_configured"
	case errors.Is(err, tool.ErrSaveMigrationInvalid):
		status = http.StatusUnprocessableEntity
		code = "save_migration_preflight_failed"
	case errors.Is(err, tool.ErrGameServerRunning):
		status = http.StatusConflict
		code = "game_server_running"
	case errors.Is(err, tool.ErrGameServerStatusUnknown):
		status = http.StatusConflict
		code = "game_server_status_unknown"
	case errors.Is(err, tool.ErrServerControlNotConfigured), strings.Contains(err.Error(), "server control"):
		status = http.StatusConflict
		code = "server_control_not_configured"
	}
	c.JSON(status, SaveMigrationErrorResponse{Error: err.Error(), Code: code, Result: result})
}

func hasSaveMigrationIssue(plan tool.SaveMigrationPlan, code string) bool {
	for _, issue := range plan.Issues {
		if issue.Code == code {
			return true
		}
	}
	return false
}

func saveMigrationIssueText(plan tool.SaveMigrationPlan) string {
	messages := make([]string, 0, len(plan.Issues))
	for _, issue := range plan.Issues {
		messages = append(messages, issue.Message)
	}
	return strings.Join(messages, "; ")
}
