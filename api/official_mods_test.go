package api

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/spf13/viper"
	"github.com/zaigie/palworld-server-tool/internal/auth"
	"github.com/zaigie/palworld-server-tool/internal/tool"
)

func TestOfficialModRoutesExposePreflightAndRequireRiskConfirmations(t *testing.T) {
	gin.SetMode(gin.TestMode)
	viper.Reset()
	viper.Set("web.password", "admin-secret")
	viper.Set("mods.install_dir", "")
	viper.Set("steamcmd.install_dir", "")
	viper.Set("rest.address", "")
	viper.Set("palworld.control.mode", "disabled")
	t.Cleanup(viper.Reset)

	token, err := auth.GenerateToken()
	if err != nil {
		t.Fatal(err)
	}
	router := gin.New()
	RegisterRouter(router)

	request := httptest.NewRequest(http.MethodGet, "/api/server/mods", nil)
	request.Header.Set("Authorization", "Bearer "+token)
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("official mod status returned %d: %s", response.Code, response.Body.String())
	}
	if cacheControl := response.Header().Get("Cache-Control"); cacheControl != "no-store" {
		t.Fatalf("official mod status was cacheable: %q", cacheControl)
	}
	var status tool.OfficialModStatus
	if err := json.Unmarshal(response.Body.Bytes(), &status); err != nil {
		t.Fatal(err)
	}
	if status.GameVersion != tool.OfficialModGameVersion || status.Configured || status.StatusDigest == "" {
		t.Fatalf("unexpected unconfigured status: %#v", status)
	}

	request = httptest.NewRequest(
		http.MethodPost,
		"/api/server/mods/preflight",
		strings.NewReader(`{"global_enabled":false,"active_mod_list":[]}`),
	)
	request.Header.Set("Authorization", "Bearer "+token)
	request.Header.Set("Content-Type", "application/json")
	response = httptest.NewRecorder()
	router.ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("official mod preflight returned %d: %s", response.Code, response.Body.String())
	}
	var preflight OfficialModPreflightResponse
	if err := json.Unmarshal(response.Body.Bytes(), &preflight); err != nil {
		t.Fatal(err)
	}
	if preflight.Plan.CanApply || preflight.Plan.PlanDigest == "" {
		t.Fatalf("unexpected unconfigured plan: %#v", preflight)
	}

	digest := strings.Repeat("a", 64)
	request = newAuthenticatedOfficialModApplyRequest(t, token, digest, false, false, false)
	response = httptest.NewRecorder()
	router.ServeHTTP(response, request)
	assertOfficialModAPIError(t, response, http.StatusConflict, "official_mod_confirmation")

	request = newAuthenticatedOfficialModApplyRequest(t, token, digest, true, false, false)
	response = httptest.NewRecorder()
	router.ServeHTTP(response, request)
	assertOfficialModAPIError(t, response, http.StatusConflict, "official_mod_risk_confirmation")

	request = newAuthenticatedOfficialModApplyRequest(t, token, digest, true, true, false)
	response = httptest.NewRecorder()
	router.ServeHTTP(response, request)
	assertOfficialModAPIError(t, response, http.StatusUnprocessableEntity, "official_mod_not_configured")
}

func TestOfficialModApplyUsesValidatedPlanWithoutExecutingModContent(t *testing.T) {
	gin.SetMode(gin.TestMode)
	viper.Reset()
	viper.Set("web.password", "admin-secret")
	viper.Set("rest.address", "")
	viper.Set("palworld.control.mode", "disabled")
	t.Cleanup(viper.Reset)
	resetOfficialModAPIDependencies(t)

	digest := strings.Repeat("b", 64)
	desired := tool.OfficialModSettings{GlobalEnabled: true, ActiveModList: []string{"SafeServerMod"}}
	planCalls := 0
	planOfficialModSettingsAPI = func(actual tool.OfficialModSettings) tool.OfficialModChangePlan {
		planCalls++
		if actual.GlobalEnabled != desired.GlobalEnabled || len(actual.ActiveModList) != 1 || actual.ActiveModList[0] != "SafeServerMod" {
			t.Fatalf("unexpected desired settings: %#v", actual)
		}
		return usableOfficialModPlan(digest, desired)
	}
	getOfficialModControlAPI = func(context.Context) tool.ServerControlStatus {
		return tool.ServerControlStatus{Configured: false, State: "unconfigured"}
	}
	applyCalls := 0
	applyOfficialModSettingsAPI = func(_ context.Context, options tool.OfficialModApplyOptions) (tool.OfficialModApplyResult, error) {
		applyCalls++
		if options.ExpectedPlanDigest != digest || !options.ConfirmServerStop {
			t.Fatalf("unexpected apply options: %#v", options)
		}
		plan := usableOfficialModPlan(digest, desired)
		return tool.OfficialModApplyResult{
			Plan:            plan,
			Status:          plan.Status,
			Changed:         true,
			RecoveryPath:    `C:\pst\backups\mods\PalModSettings.ini`,
			SettingsSHA256:  strings.Repeat("c", 64),
			RestartRequired: true,
		}, nil
	}
	stopOfficialModServerAPI = func(context.Context, int, string) (tool.MaintenanceStopResult, error) {
		t.Fatal("an already stopped server must not be stopped again")
		return tool.MaintenanceStopResult{}, nil
	}
	startOfficialModServerAPI = func(context.Context) error {
		t.Fatal("restart_after=false must not start the server")
		return nil
	}

	token, err := auth.GenerateToken()
	if err != nil {
		t.Fatal(err)
	}
	router := gin.New()
	RegisterRouter(router)
	body := `{"global_enabled":true,"active_mod_list":["SafeServerMod"],"expected_plan_digest":"` + digest + `","confirm_apply":true,"confirm_mod_risk":true,"confirm_server_stopped":true,"restart_after":false}`
	request := httptest.NewRequest(http.MethodPost, "/api/server/mods/apply", strings.NewReader(body))
	request.Header.Set("Authorization", "Bearer "+token)
	request.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("official mod apply returned %d: %s", response.Code, response.Body.String())
	}
	if planCalls != 2 || applyCalls != 1 {
		t.Fatalf("expected two preflight checks and one apply, got plan=%d apply=%d", planCalls, applyCalls)
	}
	var payload OfficialModApplyResponse
	if err := json.Unmarshal(response.Body.Bytes(), &payload); err != nil {
		t.Fatal(err)
	}
	if !payload.Apply.Changed || payload.Restarted || payload.Apply.RecoveryPath == "" {
		t.Fatalf("unexpected apply response: %#v", payload)
	}
}

func TestOfficialModRestartFailureRollsBackAndRecoversServer(t *testing.T) {
	gin.SetMode(gin.TestMode)
	viper.Reset()
	viper.Set("web.password", "admin-secret")
	viper.Set("rest.address", "")
	viper.Set("palworld.control.mode", "disabled")
	t.Cleanup(viper.Reset)
	resetOfficialModAPIDependencies(t)

	digest := strings.Repeat("d", 64)
	desired := tool.OfficialModSettings{GlobalEnabled: true, ActiveModList: []string{"CrashyMod"}}
	planOfficialModSettingsAPI = func(tool.OfficialModSettings) tool.OfficialModChangePlan {
		return usableOfficialModPlan(digest, desired)
	}
	getOfficialModControlAPI = func(context.Context) tool.ServerControlStatus {
		return tool.ServerControlStatus{Configured: true, State: "stopped"}
	}
	applyOfficialModSettingsAPI = func(_ context.Context, _ tool.OfficialModApplyOptions) (tool.OfficialModApplyResult, error) {
		plan := usableOfficialModPlan(digest, desired)
		return tool.OfficialModApplyResult{
			Plan:            plan,
			Status:          plan.Status,
			Changed:         true,
			RecoveryPath:    `C:\pst\backups\mods\before.ini`,
			SettingsSHA256:  strings.Repeat("e", 64),
			RestartRequired: true,
		}, nil
	}
	startCalls := 0
	startOfficialModServerAPI = func(context.Context) error {
		startCalls++
		if startCalls == 1 {
			return errors.New("server failed health verification after mod deployment")
		}
		return nil
	}
	rollbackCalls := 0
	rollbackOfficialModSettingsAPI = func(_ context.Context, result *tool.OfficialModApplyResult, confirm bool) error {
		rollbackCalls++
		if !confirm {
			t.Fatal("rollback must reconfirm the stopped server")
		}
		result.RolledBack = true
		return nil
	}
	forceStopOfficialModServerAPI = func(context.Context) error {
		t.Fatal("stopped control status should not require a force stop")
		return nil
	}

	token, err := auth.GenerateToken()
	if err != nil {
		t.Fatal(err)
	}
	router := gin.New()
	RegisterRouter(router)
	body := `{"global_enabled":true,"active_mod_list":["CrashyMod"],"expected_plan_digest":"` + digest + `","confirm_apply":true,"confirm_mod_risk":true,"confirm_server_stopped":true,"restart_after":true}`
	request := httptest.NewRequest(http.MethodPost, "/api/server/mods/apply", strings.NewReader(body))
	request.Header.Set("Authorization", "Bearer "+token)
	request.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)
	assertOfficialModAPIError(t, response, http.StatusInternalServerError, "official_mod_restart_failed_rolled_back")
	var payload OfficialModApplyErrorResponse
	if err := json.Unmarshal(response.Body.Bytes(), &payload); err != nil {
		t.Fatal(err)
	}
	if startCalls != 2 || rollbackCalls != 1 || !payload.Result.Apply.RolledBack || !payload.Result.RecoveryRestarted {
		t.Fatalf("runtime recovery did not restore the old settings and server: %#v", payload)
	}
}

func usableOfficialModPlan(digest string, desired tool.OfficialModSettings) tool.OfficialModChangePlan {
	status := tool.OfficialModStatus{
		GameVersion:  tool.OfficialModGameVersion,
		Platform:     "windows",
		Supported:    true,
		Configured:   true,
		Manageable:   true,
		Settings:     desired,
		StatusDigest: strings.Repeat("f", 64),
	}
	return tool.OfficialModChangePlan{
		Status:          status,
		DesiredSettings: desired,
		Changed:         true,
		Changes:         []string{"active_mod_list"},
		CanApply:        true,
		PlanDigest:      digest,
	}
}

func newAuthenticatedOfficialModApplyRequest(
	t *testing.T,
	token,
	digest string,
	confirmApply,
	confirmRisk,
	restart bool,
) *http.Request {
	t.Helper()
	body := `{"global_enabled":false,"active_mod_list":[],"expected_plan_digest":"` + digest + `","confirm_apply":` +
		jsonBool(confirmApply) + `,"confirm_mod_risk":` + jsonBool(confirmRisk) +
		`,"confirm_server_stopped":true,"restart_after":` + jsonBool(restart) + `}`
	request := httptest.NewRequest(http.MethodPost, "/api/server/mods/apply", strings.NewReader(body))
	request.Header.Set("Authorization", "Bearer "+token)
	request.Header.Set("Content-Type", "application/json")
	return request
}

func jsonBool(value bool) string {
	if value {
		return "true"
	}
	return "false"
}

func assertOfficialModAPIError(t *testing.T, response *httptest.ResponseRecorder, status int, code string) {
	t.Helper()
	if response.Code != status {
		t.Fatalf("expected status %d, got %d: %s", status, response.Code, response.Body.String())
	}
	var payload OfficialModApplyErrorResponse
	if err := json.Unmarshal(response.Body.Bytes(), &payload); err != nil {
		t.Fatal(err)
	}
	if payload.Code != code || payload.Error == "" {
		t.Fatalf("unexpected official mod error: %#v", payload)
	}
}

func resetOfficialModAPIDependencies(t *testing.T) {
	t.Helper()
	previousInspect := inspectOfficialModsAPI
	previousPlan := planOfficialModSettingsAPI
	previousApply := applyOfficialModSettingsAPI
	previousRollback := rollbackOfficialModSettingsAPI
	previousBackup := backupOfficialModWorldAPI
	previousDatabase := getOfficialModDatabaseAPI
	previousControl := getOfficialModControlAPI
	previousStop := stopOfficialModServerAPI
	previousStart := startOfficialModServerAPI
	previousForceStop := forceStopOfficialModServerAPI
	t.Cleanup(func() {
		inspectOfficialModsAPI = previousInspect
		planOfficialModSettingsAPI = previousPlan
		applyOfficialModSettingsAPI = previousApply
		rollbackOfficialModSettingsAPI = previousRollback
		backupOfficialModWorldAPI = previousBackup
		getOfficialModDatabaseAPI = previousDatabase
		getOfficialModControlAPI = previousControl
		stopOfficialModServerAPI = previousStop
		startOfficialModServerAPI = previousStart
		forceStopOfficialModServerAPI = previousForceStop
	})
}
