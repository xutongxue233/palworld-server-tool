package api

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/zaigie/palworld-server-tool/internal/executor"
	"github.com/zaigie/palworld-server-tool/internal/task"
)

func TestRunRconCommand(t *testing.T) {
	gin.SetMode(gin.TestMode)
	original := executeRCONCommand
	t.Cleanup(func() { executeRCONCommand = original })

	var received string
	executeRCONCommand = func(command string) (string, error) {
		received = command
		return "server reply", nil
	}

	router := gin.New()
	router.POST("/api/rcon", runRconCommand)
	request := httptest.NewRequest(http.MethodPost, "/api/rcon", strings.NewReader(`{"command":"  Info  "}`))
	request.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("unexpected status %d: %s", response.Code, response.Body.String())
	}
	if received != "Info" {
		t.Fatalf("unexpected command %q", received)
	}
	if !strings.Contains(response.Body.String(), "server reply") {
		t.Fatalf("unexpected response %s", response.Body.String())
	}
}

func TestRunRconCommandValidation(t *testing.T) {
	gin.SetMode(gin.TestMode)
	tests := []struct {
		name string
		body string
	}{
		{name: "empty", body: `{"command":"  "}`},
		{name: "too long", body: `{"command":"` + strings.Repeat("x", maxRconCommandLength+1) + `"}`},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			router := gin.New()
			router.POST("/api/rcon", runRconCommand)
			request := httptest.NewRequest(http.MethodPost, "/api/rcon", strings.NewReader(test.body))
			request.Header.Set("Content-Type", "application/json")
			response := httptest.NewRecorder()
			router.ServeHTTP(response, request)
			if response.Code != http.StatusBadRequest {
				t.Fatalf("unexpected status %d: %s", response.Code, response.Body.String())
			}
		})
	}
}

func TestRunRconCommandExplainsMissingPassword(t *testing.T) {
	gin.SetMode(gin.TestMode)
	original := executeRCONCommand
	t.Cleanup(func() { executeRCONCommand = original })
	executeRCONCommand = func(string) (string, error) {
		return "", executor.ErrPasswordEmpty
	}

	router := gin.New()
	router.POST("/api/rcon", runRconCommand)
	request := httptest.NewRequest(http.MethodPost, "/api/rcon", strings.NewReader(`{"command":"Info"}`))
	request.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)

	if response.Code != http.StatusBadRequest || !strings.Contains(response.Body.String(), "not configured") {
		t.Fatalf("unexpected response %d: %s", response.Code, response.Body.String())
	}
}

func TestRunRconCommandReturnsExecutionError(t *testing.T) {
	gin.SetMode(gin.TestMode)
	original := executeRCONCommand
	t.Cleanup(func() { executeRCONCommand = original })
	executeRCONCommand = func(string) (string, error) {
		return "", errors.New("dial failed")
	}

	router := gin.New()
	router.POST("/api/rcon", runRconCommand)
	request := httptest.NewRequest(http.MethodPost, "/api/rcon", strings.NewReader(`{"command":"Info"}`))
	request.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)

	if response.Code != http.StatusBadRequest || !strings.Contains(response.Body.String(), "dial failed") {
		t.Fatalf("unexpected response %d: %s", response.Code, response.Body.String())
	}
}

func TestRCONShutdownCommandsRecordIntentionalDowntime(t *testing.T) {
	for _, command := range []string{"Shutdown 60 maintenance", "/DoExit", " doexit "} {
		desired := desiredRunningForRCON(command)
		if desired == nil || *desired {
			t.Fatalf("%q did not record intentional downtime", command)
		}
	}
	for _, command := range []string{"Info", "Save", "Broadcast hello"} {
		if desired := desiredRunningForRCON(command); desired != nil {
			t.Fatalf("%q unexpectedly changed watchdog intent", command)
		}
	}
}

func TestRunRconCommandRejectsOverlappingMaintenance(t *testing.T) {
	release, err := task.BeginManualServerOperation(nil)
	if err != nil {
		t.Fatal(err)
	}
	defer release()

	router := gin.New()
	router.POST("/api/rcon", runRconCommand)
	request := httptest.NewRequest(http.MethodPost, "/api/rcon", strings.NewReader(`{"command":"Info"}`))
	request.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)
	if response.Code != http.StatusConflict {
		t.Fatalf("overlapping RCON command returned %d: %s", response.Code, response.Body.String())
	}
}
