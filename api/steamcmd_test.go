package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/spf13/viper"
	"github.com/zaigie/palworld-server-tool/internal/auth"
)

func TestSteamCMDRoutesExposePreflightAndRequireExplicitConfirmation(t *testing.T) {
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

	request := httptest.NewRequest(http.MethodGet, "/api/server/steamcmd", nil)
	request.Header.Set("Authorization", "Bearer "+token)
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("SteamCMD preflight returned %d: %s", response.Code, response.Body.String())
	}
	if cacheControl := response.Header().Get("Cache-Control"); cacheControl != "no-store" {
		t.Fatalf("sensitive preflight response was cacheable: %q", cacheControl)
	}
	var status SteamCMDStatusResponse
	if err := json.Unmarshal(response.Body.Bytes(), &status); err != nil {
		t.Fatal(err)
	}
	if status.Plan.Configured || status.Plan.CanExecute || status.Plan.AppID != 2394010 || len(status.Plan.PlanDigest) != 64 {
		t.Fatalf("unexpected unconfigured preflight: %#v", status)
	}

	request = httptest.NewRequest(
		http.MethodPost,
		"/api/server/steamcmd/update",
		strings.NewReader(`{"expected_plan_digest":"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa","confirm_update":false}`),
	)
	request.Header.Set("Authorization", "Bearer "+token)
	request.Header.Set("Content-Type", "application/json")
	response = httptest.NewRecorder()
	router.ServeHTTP(response, request)
	assertSteamCMDAPIError(t, response, http.StatusConflict, "steamcmd_confirmation")

	request = httptest.NewRequest(
		http.MethodPost,
		"/api/server/steamcmd/update",
		strings.NewReader(`{"expected_plan_digest":"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa","confirm_update":true,"confirm_server_stopped":true}`),
	)
	request.Header.Set("Authorization", "Bearer "+token)
	request.Header.Set("Content-Type", "application/json")
	response = httptest.NewRecorder()
	router.ServeHTTP(response, request)
	assertSteamCMDAPIError(t, response, http.StatusUnprocessableEntity, "steamcmd_not_configured")
}

func assertSteamCMDAPIError(t *testing.T, response *httptest.ResponseRecorder, status int, code string) {
	t.Helper()
	if response.Code != status {
		t.Fatalf("expected status %d, got %d: %s", status, response.Code, response.Body.String())
	}
	var payload SteamCMDUpdateErrorResponse
	if err := json.Unmarshal(response.Body.Bytes(), &payload); err != nil {
		t.Fatal(err)
	}
	if payload.Code != code || payload.Error == "" {
		t.Fatalf("unexpected SteamCMD error: %#v", payload)
	}
}
