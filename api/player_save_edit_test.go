package api

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/zaigie/palworld-server-tool/internal/tool"
)

func TestEditPlayerStatPointsRequestRequiresExpectedValue(t *testing.T) {
	gin.SetMode(gin.TestMode)
	request := httptest.NewRequest(
		http.MethodPatch,
		"/api/player/1/stat-points",
		bytes.NewBufferString(`{"unused_status_points":0,"confirm_server_stopped":true}`),
	)
	request.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()
	context, _ := gin.CreateTestContext(response)
	context.Request = request

	var payload EditPlayerStatPointsRequest
	if err := context.ShouldBindJSON(&payload); err == nil {
		t.Fatal("missing expected_unused_status_points should be rejected")
	}
}

func TestEditPlayerTechnologyPointsRequestRequiresAllExpectedValues(t *testing.T) {
	gin.SetMode(gin.TestMode)
	request := httptest.NewRequest(
		http.MethodPatch,
		"/api/player/1/technology-points",
		bytes.NewBufferString(`{"technology_points":1,"ancient_technology_points":2,"confirm_server_stopped":true}`),
	)
	request.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()
	context, _ := gin.CreateTestContext(response)
	context.Request = request

	var payload EditPlayerTechnologyPointsRequest
	if err := context.ShouldBindJSON(&payload); err == nil {
		t.Fatal("missing expected technology point balances should be rejected")
	}
}

func TestUnlockPlayerMapRequestRequiresValidDigest(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, test := range []struct {
		name    string
		body    string
		wantErr bool
	}{
		{
			name: "valid digest",
			body: `{"expected_progress_digest":"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa","confirm_server_stopped":true}`,
		},
		{
			name:    "missing digest",
			body:    `{"confirm_server_stopped":true}`,
			wantErr: true,
		},
		{
			name:    "invalid digest",
			body:    `{"expected_progress_digest":"not-a-digest","confirm_server_stopped":true}`,
			wantErr: true,
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			request := httptest.NewRequest(
				http.MethodPatch,
				"/api/player/1/map-progress",
				bytes.NewBufferString(test.body),
			)
			request.Header.Set("Content-Type", "application/json")
			response := httptest.NewRecorder()
			context, _ := gin.CreateTestContext(response)
			context.Request = request

			var payload UnlockPlayerMapRequest
			err := context.ShouldBindJSON(&payload)
			if test.wantErr && err == nil {
				t.Fatal("invalid request should be rejected")
			}
			if !test.wantErr && err != nil {
				t.Fatalf("valid request rejected: %v", err)
			}
		})
	}
}

func TestRenamePalRequestRequiresFieldsButAllowsEmptyNicknames(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, test := range []struct {
		name    string
		body    string
		wantErr bool
	}{
		{
			name: "clear nickname",
			body: `{"expected_nickname":"Old Pal","expected_level":2,"expected_exp":25,"nickname":"","confirm_server_stopped":true}`,
		},
		{
			name: "name previously absent",
			body: `{"expected_nickname":"","expected_level":2,"expected_exp":25,"nickname":"Named","confirm_server_stopped":true}`,
		},
		{
			name:    "missing expected nickname",
			body:    `{"expected_level":2,"expected_exp":25,"nickname":"Named","confirm_server_stopped":true}`,
			wantErr: true,
		},
		{
			name:    "missing new nickname",
			body:    `{"expected_nickname":"","expected_level":2,"expected_exp":25,"confirm_server_stopped":true}`,
			wantErr: true,
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			request := httptest.NewRequest(
				http.MethodPatch,
				"/api/player/1/pals/c410c416-475c-0638-eb35-269338f2a320/nickname",
				bytes.NewBufferString(test.body),
			)
			request.Header.Set("Content-Type", "application/json")
			response := httptest.NewRecorder()
			context, _ := gin.CreateTestContext(response)
			context.Request = request

			var payload RenamePalRequest
			err := context.ShouldBindJSON(&payload)
			if test.wantErr && err == nil {
				t.Fatal("invalid request should be rejected")
			}
			if !test.wantErr && err != nil {
				t.Fatalf("valid request rejected: %v", err)
			}
		})
	}
}

func TestEditPalLevelRequestRequiresCASFieldsAndAllowsMissingStoredMaxHP(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, test := range []struct {
		name    string
		body    string
		wantErr bool
	}{
		{
			name: "valid missing stored MaxHP",
			body: `{"expected_nickname":"","expected_level":55,"expected_exp":6678888,"expected_hp":7286000,"expected_max_hp":0,"level":56,"confirm_server_stopped":true}`,
		},
		{
			name:    "missing expected HP",
			body:    `{"expected_nickname":"","expected_level":55,"expected_exp":6678888,"expected_max_hp":0,"level":56,"confirm_server_stopped":true}`,
			wantErr: true,
		},
		{
			name:    "missing expected MaxHP",
			body:    `{"expected_nickname":"","expected_level":55,"expected_exp":6678888,"expected_hp":7286000,"level":56,"confirm_server_stopped":true}`,
			wantErr: true,
		},
		{
			name:    "missing new level",
			body:    `{"expected_nickname":"","expected_level":55,"expected_exp":6678888,"expected_hp":7286000,"expected_max_hp":0,"confirm_server_stopped":true}`,
			wantErr: true,
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			request := httptest.NewRequest(
				http.MethodPatch,
				"/api/player/1/pals/ff38bdad-4710-966d-982f-3ca7cb107b56/level",
				bytes.NewBufferString(test.body),
			)
			request.Header.Set("Content-Type", "application/json")
			response := httptest.NewRecorder()
			context, _ := gin.CreateTestContext(response)
			context.Request = request

			var payload EditPalLevelRequest
			err := context.ShouldBindJSON(&payload)
			if test.wantErr && err == nil {
				t.Fatal("invalid request should be rejected")
			}
			if !test.wantErr && err != nil {
				t.Fatalf("valid request rejected: %v", err)
			}
		})
	}
}

func TestRestorePalHealthRequestRequiresCASFieldsAndAllowsEmptyNicknameAndZeroHP(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, test := range []struct {
		name    string
		body    string
		wantErr bool
	}{
		{
			name: "valid empty nickname and zero HP",
			body: `{"expected_nickname":"","expected_level":2,"expected_exp":25,"expected_hp":0,"expected_max_hp":583000,"confirm_server_stopped":true}`,
		},
		{
			name:    "missing expected nickname",
			body:    `{"expected_level":2,"expected_exp":25,"expected_hp":0,"expected_max_hp":583000,"confirm_server_stopped":true}`,
			wantErr: true,
		},
		{
			name:    "missing expected level",
			body:    `{"expected_nickname":"","expected_exp":25,"expected_hp":0,"expected_max_hp":583000,"confirm_server_stopped":true}`,
			wantErr: true,
		},
		{
			name:    "missing expected EXP",
			body:    `{"expected_nickname":"","expected_level":2,"expected_hp":0,"expected_max_hp":583000,"confirm_server_stopped":true}`,
			wantErr: true,
		},
		{
			name:    "missing expected HP",
			body:    `{"expected_nickname":"","expected_level":2,"expected_exp":25,"expected_max_hp":583000,"confirm_server_stopped":true}`,
			wantErr: true,
		},
		{
			name:    "missing expected MaxHP",
			body:    `{"expected_nickname":"","expected_level":2,"expected_exp":25,"expected_hp":0,"confirm_server_stopped":true}`,
			wantErr: true,
		},
		{
			name:    "zero expected MaxHP",
			body:    `{"expected_nickname":"","expected_level":2,"expected_exp":25,"expected_hp":0,"expected_max_hp":0,"confirm_server_stopped":true}`,
			wantErr: true,
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			request := httptest.NewRequest(
				http.MethodPatch,
				"/api/player/1/pals/c410c416-475c-0638-eb35-269338f2a320/health",
				bytes.NewBufferString(test.body),
			)
			request.Header.Set("Content-Type", "application/json")
			response := httptest.NewRecorder()
			context, _ := gin.CreateTestContext(response)
			context.Request = request

			var payload RestorePalHealthRequest
			err := context.ShouldBindJSON(&payload)
			if test.wantErr && err == nil {
				t.Fatal("invalid request should be rejected")
			}
			if !test.wantErr && err != nil {
				t.Fatalf("valid request rejected: %v", err)
			}
		})
	}
}

func TestWriteSaveEditErrorCodes(t *testing.T) {
	gin.SetMode(gin.TestMode)
	tests := []struct {
		name       string
		err        error
		statusCode int
		code       string
	}{
		{
			"confirmation", tool.ErrSaveEditConfirmation,
			http.StatusConflict, "save_edit_confirmation",
		},
		{
			"server running", tool.ErrGameServerRunning,
			http.StatusConflict, "game_server_running",
		},
		{
			"source changed", tool.ErrSaveSourceChanged,
			http.StatusConflict, "save_source_changed",
		},
		{
			"server status unknown", tool.ErrGameServerStatusUnknown,
			http.StatusServiceUnavailable, "game_server_status_unknown",
		},
		{
			"unsupported source", tool.ErrUnsupportedSaveSource,
			http.StatusUnprocessableEntity, "unsupported_save_source",
		},
		{
			"internal transaction failure", fmt.Errorf("backup failed: %w", tool.ErrSaveEditInternal),
			http.StatusInternalServerError, "save_edit_internal",
		},
		{
			"validation", errors.New("invalid mutation"),
			http.StatusBadRequest, "",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			response := httptest.NewRecorder()
			context, _ := gin.CreateTestContext(response)
			writeSaveEditError(context, test.err)

			if response.Code != test.statusCode {
				t.Fatalf("status = %d, want %d", response.Code, test.statusCode)
			}
			var payload ErrorResponse
			if err := json.Unmarshal(response.Body.Bytes(), &payload); err != nil {
				t.Fatal(err)
			}
			if payload.Code != test.code || payload.Error != test.err.Error() {
				t.Fatalf("unexpected response: %#v", payload)
			}
		})
	}
}
