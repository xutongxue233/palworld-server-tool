package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/spf13/viper"
	"github.com/zaigie/palworld-server-tool/internal/tool"
)

func TestGameConfigEndpointIsNoStoreAndReportsUnconfigured(t *testing.T) {
	gin.SetMode(gin.TestMode)
	viper.Reset()
	t.Cleanup(viper.Reset)

	router := gin.New()
	router.GET("/api/server/config-file", getGameConfigFile)
	request := httptest.NewRequest(http.MethodGet, "/api/server/config-file", nil)
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("config endpoint returned %d: %s", response.Code, response.Body.String())
	}
	if response.Header().Get("Cache-Control") != "no-store" {
		t.Fatalf("sensitive configuration response may be cached: %#v", response.Header())
	}
	var payload tool.GameConfigFile
	if err := json.Unmarshal(response.Body.Bytes(), &payload); err != nil {
		t.Fatal(err)
	}
	if payload.Configured || payload.ModifiedAt != nil {
		t.Fatalf("unexpected unconfigured payload: %#v", payload)
	}
}
