package api

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/spf13/viper"
	"github.com/zaigie/palworld-server-tool/internal/auth"
	"github.com/zaigie/palworld-server-tool/internal/tool"
)

func TestSaveMigrationRecoveryErrorPreservesCauseAndAddsGuidance(t *testing.T) {
	cause := errors.New("migration failed")
	wrapped := saveMigrationRecoveryError(cause, SaveMigrationApplyResponse{
		RestartError: "start PalServer manually",
	})
	if !errors.Is(wrapped, cause) || !strings.Contains(wrapped.Error(), "start PalServer manually") {
		t.Fatalf("unexpected recovery error: %v", wrapped)
	}
	if unchanged := saveMigrationRecoveryError(cause, SaveMigrationApplyResponse{}); unchanged != cause {
		t.Fatalf("an empty recovery result should preserve the original error: %v", unchanged)
	}
}

func TestSaveMigrationRoutesExposeReadOnlyPreflightAndRequireConfirmation(t *testing.T) {
	gin.SetMode(gin.TestMode)
	viper.Reset()
	viper.Set("web.password", "admin-secret")
	viper.Set("save.path", "")
	viper.Set("rest.address", "")
	viper.Set("palworld.control.mode", "disabled")
	t.Cleanup(viper.Reset)

	token, err := auth.GenerateToken()
	if err != nil {
		t.Fatal(err)
	}
	router := gin.New()
	RegisterRouter(router)
	source := filepath.Join(t.TempDir(), "missing-world")

	request := httptest.NewRequest(
		http.MethodPost,
		"/api/server/migration/preflight",
		strings.NewReader(`{"source_path":`+quotedJSON(source)+`,"source_platform":"current","source_kind":"dedicated"}`),
	)
	request.Header.Set("Authorization", "Bearer "+token)
	request.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("save migration preflight returned %d: %s", response.Code, response.Body.String())
	}
	if cacheControl := response.Header().Get("Cache-Control"); cacheControl != "no-store" {
		t.Fatalf("save migration preflight was cacheable: %q", cacheControl)
	}
	var preflight SaveMigrationPreflightResponse
	if err := json.Unmarshal(response.Body.Bytes(), &preflight); err != nil {
		t.Fatal(err)
	}
	if preflight.Plan.GameVersion != tool.SaveMigrationGameVersion || preflight.Plan.SourcePlatform != runtime.GOOS || preflight.Plan.CanMigrate || preflight.Plan.Configured {
		t.Fatalf("unexpected unconfigured migration preflight: %#v", preflight)
	}

	request = httptest.NewRequest(
		http.MethodPost,
		"/api/server/migration/apply",
		strings.NewReader(`{"source_path":`+quotedJSON(source)+`,"source_platform":"current","source_kind":"dedicated","expected_plan_digest":"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa","confirm_migration":false}`),
	)
	request.Header.Set("Authorization", "Bearer "+token)
	request.Header.Set("Content-Type", "application/json")
	response = httptest.NewRecorder()
	router.ServeHTTP(response, request)
	assertSaveMigrationAPIError(t, response, http.StatusConflict, "save_migration_confirmation")

	request = httptest.NewRequest(
		http.MethodPost,
		"/api/server/migration/apply",
		strings.NewReader(`{"source_path":`+quotedJSON(source)+`,"source_platform":"current","source_kind":"dedicated","expected_plan_digest":"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa","confirm_migration":true,"confirm_server_stopped":true}`),
	)
	request.Header.Set("Authorization", "Bearer "+token)
	request.Header.Set("Content-Type", "application/json")
	response = httptest.NewRecorder()
	router.ServeHTTP(response, request)
	assertSaveMigrationAPIError(t, response, http.StatusUnprocessableEntity, "save_migration_not_configured")
}

func TestSaveMigrationPreflightValidatesSourceIdentityFields(t *testing.T) {
	gin.SetMode(gin.TestMode)
	viper.Reset()
	viper.Set("web.password", "admin-secret")
	t.Cleanup(viper.Reset)
	token, err := auth.GenerateToken()
	if err != nil {
		t.Fatal(err)
	}
	router := gin.New()
	RegisterRouter(router)
	request := httptest.NewRequest(
		http.MethodPost,
		"/api/server/migration/preflight",
		strings.NewReader(`{"source_path":"C:\\save","source_platform":"macos","source_kind":"unknown"}`),
	)
	request.Header.Set("Authorization", "Bearer "+token)
	request.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)
	if response.Code != http.StatusBadRequest {
		t.Fatalf("expected invalid source identity fields to return 400, got %d: %s", response.Code, response.Body.String())
	}
}

func quotedJSON(value string) string {
	encoded, _ := json.Marshal(value)
	return string(encoded)
}

func assertSaveMigrationAPIError(t *testing.T, response *httptest.ResponseRecorder, status int, code string) {
	t.Helper()
	if response.Code != status {
		t.Fatalf("expected status %d, got %d: %s", status, response.Code, response.Body.String())
	}
	var payload SaveMigrationErrorResponse
	if err := json.Unmarshal(response.Body.Bytes(), &payload); err != nil {
		t.Fatal(err)
	}
	if payload.Code != code || payload.Error == "" {
		t.Fatalf("unexpected save migration error: %#v", payload)
	}
}
