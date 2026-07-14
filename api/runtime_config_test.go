package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/spf13/viper"
	"github.com/zaigie/palworld-server-tool/internal/auth"
	"github.com/zaigie/palworld-server-tool/internal/config"
	"github.com/zaigie/palworld-server-tool/internal/database"
	"go.etcd.io/bbolt"
)

func TestRuntimeConfigRoutesPersistAndRedactSecrets(t *testing.T) {
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
	if err := config.ApplyValues(map[string]any{"web.password": "admin-secret"}); err != nil {
		t.Fatal(err)
	}
	token, err := auth.GenerateToken()
	if err != nil {
		t.Fatal(err)
	}
	router := gin.New()
	RegisterRouter(router)

	request := httptest.NewRequest(http.MethodPut, "/api/setup/config", strings.NewReader(`{
		"values": {
			"web.port": 9090,
			"rest.password": "pal-secret"
		}
	}`))
	request.Header.Set("Authorization", "Bearer "+token)
	request.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("runtime config update returned %d: %s", response.Code, response.Body.String())
	}
	if response.Header().Get("Cache-Control") != "no-store" {
		t.Fatal("runtime configuration response was cacheable")
	}
	var payload RuntimeConfigResponse
	if err := json.Unmarshal(response.Body.Bytes(), &payload); err != nil {
		t.Fatal(err)
	}
	if _, exposed := payload.Values["rest.password"]; exposed {
		t.Fatal("REST password was returned by the API")
	}
	if !slices.Contains(payload.ConfiguredSecrets, "rest.password") {
		t.Fatalf("configured secrets = %#v", payload.ConfiguredSecrets)
	}
	if !slices.Equal(payload.RestartRequired, []string{"rest.password", "web.port"}) {
		t.Fatalf("restart-required keys = %#v", payload.RestartRequired)
	}
	stored, err := database.ListConfigValues(db)
	if err != nil {
		t.Fatal(err)
	}
	if stored["rest.password"] != "pal-secret" || stored["web.port"] != float64(9090) {
		t.Fatalf("database values = %#v", stored)
	}
}
