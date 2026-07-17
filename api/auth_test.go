package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/spf13/viper"
	"github.com/zaigie/palworld-server-tool/internal/config"
	"github.com/zaigie/palworld-server-tool/internal/database"
	"github.com/zaigie/palworld-server-tool/internal/fleet"
	"go.etcd.io/bbolt"
)

func TestWebPasswordSetupLoginAndChange(t *testing.T) {
	gin.SetMode(gin.TestMode)
	viper.Reset()
	t.Cleanup(viper.Reset)
	db, err := bbolt.Open(filepath.Join(t.TempDir(), "pst.db"), 0o600, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if err := database.EnsureBuckets(db); err != nil {
		t.Fatal(err)
	}
	var current config.Config
	config.Init(db, &current)
	router := gin.New()
	RegisterRouter(router)

	statusResponse := performAuthRequest(router, http.MethodGet, "/api/auth/status", "", "")
	if statusResponse.Code != http.StatusOK {
		t.Fatalf("auth status returned %d: %s", statusResponse.Code, statusResponse.Body.String())
	}
	var status AuthStatusResponse
	if err := json.Unmarshal(statusResponse.Body.Bytes(), &status); err != nil {
		t.Fatal(err)
	}
	if status.PasswordConfigured || !status.PasswordChangeable {
		t.Fatalf("unexpected initial auth status: %#v", status)
	}

	loginResponse := performAuthRequest(router, http.MethodPost, "/api/login", `{"password":""}`, "")
	if loginResponse.Code != http.StatusConflict || !strings.Contains(loginResponse.Body.String(), "web_password_not_configured") {
		t.Fatalf("unconfigured login returned %d: %s", loginResponse.Code, loginResponse.Body.String())
	}

	shortResponse := performAuthRequest(router, http.MethodPost, "/api/auth/password", `{
		"password":"short",
		"password_confirmation":"short"
	}`, "")
	if shortResponse.Code != http.StatusBadRequest || !strings.Contains(shortResponse.Body.String(), "web_password_too_short") {
		t.Fatalf("short password returned %d: %s", shortResponse.Code, shortResponse.Body.String())
	}

	initializeResponse := performAuthRequest(router, http.MethodPost, "/api/auth/password", `{
		"password":"first-secret",
		"password_confirmation":"first-secret"
	}`, "")
	if initializeResponse.Code != http.StatusOK {
		t.Fatalf("password initialization returned %d: %s", initializeResponse.Code, initializeResponse.Body.String())
	}
	var initialized TokenResponse
	if err := json.Unmarshal(initializeResponse.Body.Bytes(), &initialized); err != nil {
		t.Fatal(err)
	}
	if initialized.Token == "" {
		t.Fatal("initialization did not return a token")
	}
	stored, err := database.ListConfigValues(db)
	if err != nil {
		t.Fatal(err)
	}
	if stored["web.password"] != "first-secret" {
		t.Fatalf("stored password = %#v", stored["web.password"])
	}

	repeatResponse := performAuthRequest(router, http.MethodPost, "/api/auth/password", `{
		"password":"replacement-secret",
		"password_confirmation":"replacement-secret"
	}`, "")
	if repeatResponse.Code != http.StatusConflict || !strings.Contains(repeatResponse.Body.String(), "web_password_already_configured") {
		t.Fatalf("repeat initialization returned %d: %s", repeatResponse.Code, repeatResponse.Body.String())
	}

	viper.Set("fleet.node_token", "0123456789abcdef0123456789abcdef")
	fleetRequest := httptest.NewRequest(http.MethodPut, "/api/auth/password", strings.NewReader(`{
		"password":"fleet-secret",
		"password_confirmation":"fleet-secret"
	}`))
	fleetRequest.Header.Set("Content-Type", "application/json")
	fleetRequest.Header.Set(fleet.NodeTokenHeader, "0123456789abcdef0123456789abcdef")
	fleetResponse := httptest.NewRecorder()
	router.ServeHTTP(fleetResponse, fleetRequest)
	if fleetResponse.Code != http.StatusForbidden || !strings.Contains(fleetResponse.Body.String(), "web_password_user_auth_required") {
		t.Fatalf("fleet password change returned %d: %s", fleetResponse.Code, fleetResponse.Body.String())
	}

	changeResponse := performAuthRequest(router, http.MethodPut, "/api/auth/password", `{
		"password":"changed-secret",
		"password_confirmation":"changed-secret"
	}`, initialized.Token)
	if changeResponse.Code != http.StatusOK {
		t.Fatalf("password change returned %d: %s", changeResponse.Code, changeResponse.Body.String())
	}
	var changed TokenResponse
	if err := json.Unmarshal(changeResponse.Body.Bytes(), &changed); err != nil {
		t.Fatal(err)
	}
	if changed.Token == "" || changed.Token == initialized.Token {
		t.Fatal("password change did not rotate the token")
	}

	oldTokenResponse := performAuthRequest(router, http.MethodGet, "/api/setup/config", "", initialized.Token)
	if oldTokenResponse.Code != http.StatusUnauthorized {
		t.Fatalf("old token returned %d after password change", oldTokenResponse.Code)
	}
	newTokenResponse := performAuthRequest(router, http.MethodGet, "/api/setup/config", "", changed.Token)
	if newTokenResponse.Code != http.StatusOK {
		t.Fatalf("new token returned %d: %s", newTokenResponse.Code, newTokenResponse.Body.String())
	}

	oldPasswordLogin := performAuthRequest(router, http.MethodPost, "/api/login", `{"password":"first-secret"}`, "")
	if oldPasswordLogin.Code != http.StatusUnauthorized {
		t.Fatalf("old password login returned %d", oldPasswordLogin.Code)
	}
	newPasswordLogin := performAuthRequest(router, http.MethodPost, "/api/login", `{"password":"changed-secret"}`, "")
	if newPasswordLogin.Code != http.StatusOK {
		t.Fatalf("new password login returned %d: %s", newPasswordLogin.Code, newPasswordLogin.Body.String())
	}
}

func TestWebPasswordConfirmationMustMatch(t *testing.T) {
	gin.SetMode(gin.TestMode)
	viper.Reset()
	t.Cleanup(viper.Reset)
	db, err := bbolt.Open(filepath.Join(t.TempDir(), "pst.db"), 0o600, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if err := database.EnsureBuckets(db); err != nil {
		t.Fatal(err)
	}
	var current config.Config
	config.Init(db, &current)
	router := gin.New()
	RegisterRouter(router)

	response := performAuthRequest(router, http.MethodPost, "/api/auth/password", `{
		"password":"first-secret",
		"password_confirmation":"different-secret"
	}`, "")
	if response.Code != http.StatusBadRequest || !strings.Contains(response.Body.String(), "web_password_confirmation_mismatch") {
		t.Fatalf("confirmation mismatch returned %d: %s", response.Code, response.Body.String())
	}
}

func performAuthRequest(router http.Handler, method, path, body, token string) *httptest.ResponseRecorder {
	request := httptest.NewRequest(method, path, strings.NewReader(body))
	if body != "" {
		request.Header.Set("Content-Type", "application/json")
	}
	if token != "" {
		request.Header.Set("Authorization", "Bearer "+token)
	}
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)
	return response
}
