package api

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/spf13/viper"
	"github.com/zaigie/palworld-server-tool/internal/auth"
	"github.com/zaigie/palworld-server-tool/internal/fleet"
	"github.com/zaigie/palworld-server-tool/internal/tool"
)

func TestFleetNodeStatusRequiresDedicatedToken(t *testing.T) {
	gin.SetMode(gin.TestMode)
	viper.Reset()
	viper.Set("web.password", "central-admin-secret")
	viper.Set("fleet.node_id", "primary")
	viper.Set("fleet.node_name", "Primary World")
	viper.Set("fleet.node_token", "0123456789abcdef0123456789abcdef")
	t.Cleanup(viper.Reset)
	resetFleetAPIDependencies(t)
	fleetInfoAPI = func() (tool.ResponseInfo, error) {
		return tool.ResponseInfo{
			Version:     "1.0.0",
			ServerName:  "Primary Palworld",
			Description: "Main world",
			WorldGUID:   "world-primary",
		}, nil
	}
	fleetMetricsAPI = func() (tool.ResponseMetrics, error) {
		return tool.ResponseMetrics{ServerFps: 60, CurrentPlayerNum: 4, MaxPlayerNum: 32}, nil
	}
	fleetControlStatusAPI = func(context.Context) tool.ServerControlStatus {
		return tool.ServerControlStatus{Configured: true, Running: true, Online: true, State: "online"}
	}
	fleetNowAPI = func() time.Time { return time.Date(2026, 7, 13, 20, 0, 0, 0, time.UTC) }

	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Set("version", "v1.8.0-test")
		c.Next()
	})
	RegisterRouter(router)

	request := httptest.NewRequest(http.MethodGet, "/api/fleet/node/status", nil)
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)
	if response.Code != http.StatusUnauthorized {
		t.Fatalf("fleet node status without token returned %d", response.Code)
	}

	request = httptest.NewRequest(http.MethodGet, "/api/fleet/node/status", nil)
	request.Header.Set(fleet.NodeTokenHeader, "0123456789abcdef0123456789abcdef")
	response = httptest.NewRecorder()
	router.ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("fleet node status returned %d: %s", response.Code, response.Body.String())
	}
	var status FleetNodeStatus
	if err := json.Unmarshal(response.Body.Bytes(), &status); err != nil {
		t.Fatal(err)
	}
	if status.ProtocolVersion != fleet.ProtocolVersion || status.NodeID != "primary" ||
		status.ToolVersion != "v1.8.0-test" || !status.ServerOnline || status.Server == nil ||
		status.Metrics == nil || status.Control == nil {
		t.Fatalf("unexpected fleet node status: %#v", status)
	}
}

func TestFleetControllerAggregatesAndProxiesAllowlistedRoutes(t *testing.T) {
	gin.SetMode(gin.TestMode)
	remoteToken := "fedcba9876543210fedcba9876543210"
	var forwardedBody string
	remote := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		if request.Header.Get(fleet.NodeTokenHeader) != remoteToken {
			response.WriteHeader(http.StatusUnauthorized)
			return
		}
		if request.Header.Get("Authorization") != "" {
			t.Errorf("central user JWT leaked to the remote node")
		}
		switch request.URL.Path {
		case "/api/fleet/node/status":
			response.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(response).Encode(FleetNodeStatus{
				ProtocolVersion: fleet.ProtocolVersion,
				NodeID:          "second",
				NodeName:        "Remote World",
				ToolVersion:     "v1.8.0",
				ServerOnline:    true,
				Server:          &ServerInfo{Name: "Remote Palworld", Version: "1.0.0"},
				Metrics:         &ServerMetrics{ServerFps: 58, CurrentPlayerNum: 7, MaxPlayerNum: 32},
				CheckedAt:       time.Date(2026, 7, 13, 20, 1, 0, 0, time.UTC),
			})
		case "/api/server/settings":
			if request.URL.Query().Get("mode") != "full" {
				t.Errorf("proxy query was not preserved: %s", request.URL.RawQuery)
			}
			response.Header().Set("Content-Type", "application/json")
			_, _ = response.Write([]byte(`{"ServerName":"Remote Palworld"}`))
		case "/api/server/config-file":
			body, _ := io.ReadAll(request.Body)
			forwardedBody = string(body)
			response.Header().Set("Content-Type", "application/json")
			_, _ = response.Write([]byte(`{"changed":true}`))
		case "/api/server/metrics":
			response.WriteHeader(http.StatusUnauthorized)
		default:
			http.NotFound(response, request)
		}
	}))
	defer remote.Close()

	viper.Reset()
	viper.Set("web.password", "central-admin-secret")
	viper.Set("fleet.node_id", "primary")
	viper.Set("fleet.node_name", "Primary World")
	viper.Set("fleet.nodes", []map[string]any{{
		"id":                    "second",
		"name":                  "Second World",
		"base_url":              remote.URL,
		"token":                 remoteToken,
		"allow_private_network": true,
		"timeout_seconds":       5,
	}})
	t.Cleanup(viper.Reset)
	resetFleetAPIDependencies(t)
	fleetInfoAPI = func() (tool.ResponseInfo, error) {
		return tool.ResponseInfo{Version: "1.0.0", ServerName: "Primary Palworld"}, nil
	}
	fleetMetricsAPI = func() (tool.ResponseMetrics, error) {
		return tool.ResponseMetrics{ServerFps: 60, CurrentPlayerNum: 1, MaxPlayerNum: 32}, nil
	}
	fleetControlStatusAPI = func(context.Context) tool.ServerControlStatus {
		return tool.ServerControlStatus{Configured: true, Running: true, Online: true, State: "online"}
	}

	token, err := auth.GenerateToken()
	if err != nil {
		t.Fatal(err)
	}
	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Set("version", "v1.8.0-test")
		c.Next()
	})
	RegisterRouter(router)

	request := httptest.NewRequest(http.MethodGet, "/api/fleet/nodes", nil)
	request.Header.Set("Authorization", "Bearer "+token)
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("fleet list returned %d: %s", response.Code, response.Body.String())
	}
	if strings.Contains(response.Body.String(), remoteToken) {
		t.Fatal("fleet list exposed the configured remote token")
	}
	var fleetStatus FleetStatusResponse
	if err := json.Unmarshal(response.Body.Bytes(), &fleetStatus); err != nil {
		t.Fatal(err)
	}
	if len(fleetStatus.Nodes) != 2 || !fleetStatus.Nodes[0].Local ||
		fleetStatus.Nodes[1].ID != "second" || !fleetStatus.Nodes[1].Reachable ||
		!fleetStatus.Nodes[1].Selectable || fleetStatus.Nodes[1].Metrics == nil {
		t.Fatalf("unexpected fleet aggregate: %#v", fleetStatus)
	}

	request = httptest.NewRequest(http.MethodGet, "/api/fleet/nodes/second/proxy/server/settings?mode=full", nil)
	request.Header.Set("Authorization", "Bearer "+token)
	response = httptest.NewRecorder()
	router.ServeHTTP(response, request)
	if response.Code != http.StatusOK || !strings.Contains(response.Body.String(), "Remote Palworld") ||
		response.Header().Get("X-PST-Fleet-Node") != "second" {
		t.Fatalf("fleet GET proxy returned %d: %s", response.Code, response.Body.String())
	}

	request = httptest.NewRequest(http.MethodPut, "/api/fleet/nodes/second/proxy/server/config-file", strings.NewReader(`{"content":"test"}`))
	request.Header.Set("Authorization", "Bearer "+token)
	request.Header.Set("Content-Type", "application/json")
	response = httptest.NewRecorder()
	router.ServeHTTP(response, request)
	if response.Code != http.StatusOK || forwardedBody != `{"content":"test"}` {
		t.Fatalf("fleet PUT proxy returned %d body=%q forwarded=%q", response.Code, response.Body.String(), forwardedBody)
	}

	request = httptest.NewRequest(http.MethodGet, "/api/fleet/nodes/second/proxy/fleet/nodes", nil)
	request.Header.Set("Authorization", "Bearer "+token)
	response = httptest.NewRecorder()
	router.ServeHTTP(response, request)
	if response.Code != http.StatusForbidden {
		t.Fatalf("recursive fleet proxy route returned %d", response.Code)
	}

	request = httptest.NewRequest(http.MethodGet, "/api/fleet/nodes/second/proxy/server/metrics", nil)
	request.Header.Set("Authorization", "Bearer "+token)
	response = httptest.NewRecorder()
	router.ServeHTTP(response, request)
	if response.Code != http.StatusBadGateway || !strings.Contains(response.Body.String(), "fleet_node_auth_failed") {
		t.Fatalf("remote auth failure was not isolated from the central JWT: %d %s", response.Code, response.Body.String())
	}
}

func resetFleetAPIDependencies(t *testing.T) {
	t.Helper()
	previousInfo := fleetInfoAPI
	previousMetrics := fleetMetricsAPI
	previousControl := fleetControlStatusAPI
	previousNow := fleetNowAPI
	t.Cleanup(func() {
		fleetInfoAPI = previousInfo
		fleetMetricsAPI = previousMetrics
		fleetControlStatusAPI = previousControl
		fleetNowAPI = previousNow
	})
}
