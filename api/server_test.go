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

func TestOfficialServerManagementRoutes(t *testing.T) {
	gin.SetMode(gin.TestMode)
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v1/api/info":
			_, _ = w.Write([]byte(`{"version":"v1","servername":"Test","description":"Desc","worldguid":"guid"}`))
		case "/v1/api/metrics":
			_, _ = w.Write([]byte(`{"serverfps":60,"currentplayernum":1,"serverframetime":16.6,"maxplayernum":32,"uptime":100,"basecampnum":3,"days":4}`))
		case "/v1/api/settings":
			_, _ = w.Write([]byte(`{"ServerName":"Test","RESTAPIEnabled":true}`))
		case "/v1/api/game-data":
			_, _ = w.Write([]byte(`{"Time":"2026-07-10 12:00:00","FPS":60,"AverageFPS":59.5,"ActorData":[]}`))
		case "/v1/api/save", "/v1/api/stop":
			w.WriteHeader(http.StatusOK)
		default:
			http.NotFound(w, r)
		}
	}))
	defer upstream.Close()

	viper.Reset()
	viper.Set("web.password", "admin-secret")
	viper.Set("rest.address", upstream.URL)
	viper.Set("rest.username", "admin")
	viper.Set("rest.password", "server-secret")
	viper.Set("rest.timeout", 5)
	t.Cleanup(viper.Reset)
	originalRCONExecutor := executeRCONCommand
	executeRCONCommand = func(command string) (string, error) { return "ok: " + command, nil }
	t.Cleanup(func() { executeRCONCommand = originalRCONExecutor })

	token, err := auth.GenerateToken()
	if err != nil {
		t.Fatal(err)
	}
	router := gin.New()
	RegisterRouter(router)

	request := httptest.NewRequest(http.MethodGet, "/api/server/settings", nil)
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)
	if response.Code != http.StatusUnauthorized {
		t.Fatalf("settings route should require authentication, got %d", response.Code)
	}
	for _, protected := range []struct {
		method string
		path   string
	}{
		{http.MethodGet, "/api/server/config-file"},
		{http.MethodPut, "/api/server/config-file"},
		{http.MethodPut, "/api/server/world-option"},
		{http.MethodGet, "/api/server/control/status"},
		{http.MethodGet, "/api/server/backups/native"},
		{http.MethodPost, "/api/server/backups/native/2026.07.13-10.00.00/restore"},
		{http.MethodPost, "/api/server/start"},
		{http.MethodPost, "/api/server/restart"},
	} {
		request = httptest.NewRequest(protected.method, protected.path, nil)
		response = httptest.NewRecorder()
		router.ServeHTTP(response, request)
		if response.Code != http.StatusUnauthorized {
			t.Fatalf("%s %s should require authentication, got %d", protected.method, protected.path, response.Code)
		}
	}

	request = httptest.NewRequest(http.MethodPost, "/api/rcon", strings.NewReader(`{"command":"Info"}`))
	request.Header.Set("Content-Type", "application/json")
	response = httptest.NewRecorder()
	router.ServeHTTP(response, request)
	if response.Code != http.StatusUnauthorized {
		t.Fatalf("RCON route should require authentication, got %d", response.Code)
	}

	for _, test := range []struct {
		method string
		path   string
	}{
		{http.MethodGet, "/api/server/settings"},
		{http.MethodGet, "/api/server/game-data"},
		{http.MethodGet, "/api/server/config-file"},
		{http.MethodGet, "/api/server/control/status"},
		{http.MethodPost, "/api/server/save"},
		{http.MethodPost, "/api/server/stop"},
	} {
		request = httptest.NewRequest(test.method, test.path, nil)
		request.Header.Set("Authorization", "Bearer "+token)
		response = httptest.NewRecorder()
		router.ServeHTTP(response, request)
		if response.Code != http.StatusOK {
			t.Fatalf("%s %s returned %d: %s", test.method, test.path, response.Code, response.Body.String())
		}
	}

	request = httptest.NewRequest(http.MethodPost, "/api/rcon", strings.NewReader(`{"command":"Info"}`))
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Authorization", "Bearer "+token)
	response = httptest.NewRecorder()
	router.ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("RCON route returned %d: %s", response.Code, response.Body.String())
	}

	request = httptest.NewRequest(http.MethodGet, "/api/server/metrics", nil)
	response = httptest.NewRecorder()
	router.ServeHTTP(response, request)
	var metrics ServerMetrics
	if err := json.Unmarshal(response.Body.Bytes(), &metrics); err != nil {
		t.Fatal(err)
	}
	if metrics.BaseCampNum != 3 {
		t.Fatalf("base camp metric was not exposed: %#v", metrics)
	}
}

func TestGameDataBridgeUnavailableIsOptional(t *testing.T) {
	gin.SetMode(gin.TestMode)
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "PalGameDataBridge GameData API is not enabled", http.StatusNotFound)
	}))
	defer upstream.Close()

	viper.Reset()
	viper.Set("rest.address", upstream.URL)
	viper.Set("rest.username", "admin")
	viper.Set("rest.password", "server-secret")
	viper.Set("rest.timeout", 5)
	t.Cleanup(viper.Reset)

	router := gin.New()
	router.GET("/api/server/game-data", getWorldActorSnapshot)
	request := httptest.NewRequest(http.MethodGet, "/api/server/game-data", nil)
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("optional GameData API returned %d: %s", response.Code, response.Body.String())
	}
	var payload struct {
		Available bool   `json:"Available"`
		Message   string `json:"Message"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &payload); err != nil {
		t.Fatal(err)
	}
	if payload.Available || !strings.Contains(payload.Message, "not enabled") {
		t.Fatalf("unexpected optional GameData response: %#v", payload)
	}
}
