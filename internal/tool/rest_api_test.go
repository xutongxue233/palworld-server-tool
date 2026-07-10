package tool

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/spf13/viper"
)

func configureRESTTest(t *testing.T, server *httptest.Server) {
	t.Helper()
	viper.Reset()
	viper.Set("rest.address", server.URL)
	viper.Set("rest.username", "admin")
	viper.Set("rest.password", "secret")
	viper.Set("rest.timeout", 5)
	t.Cleanup(viper.Reset)
}

func requireRESTAuth(t *testing.T, r *http.Request) {
	t.Helper()
	username, password, ok := r.BasicAuth()
	if !ok || username != "admin" || password != "secret" {
		t.Fatalf("unexpected basic auth: %q %q %v", username, password, ok)
	}
	if got := r.Header.Get("Accept"); got != "application/json" {
		t.Fatalf("unexpected Accept header: %q", got)
	}
}

func TestInfoAndMetricsMatchOfficialContract(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requireRESTAuth(t, r)
		switch r.URL.Path {
		case "/v1/api/info":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"version":"v1","servername":"Test","description":"Desc","worldguid":"guid"}`))
		case "/v1/api/metrics":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"serverfps":60,"currentplayernum":2,"serverframetime":16.666,"maxplayernum":32,"uptime":120,"basecampnum":4,"days":8}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()
	configureRESTTest(t, server)

	info, err := Info()
	if err != nil {
		t.Fatal(err)
	}
	if info.ServerName != "Test" || info.Description != "Desc" || info.WorldGUID != "guid" {
		t.Fatalf("unexpected info: %#v", info)
	}

	metrics, err := Metrics()
	if err != nil {
		t.Fatal(err)
	}
	if metrics.BaseCampNum != 4 || metrics.ServerFrameTime != 16.67 {
		t.Fatalf("unexpected metrics: %#v", metrics)
	}
}

func TestPlayerActionsAndServerOperations(t *testing.T) {
	seen := make(map[string]map[string]interface{})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requireRESTAuth(t, r)
		if r.Header.Get("Content-Type") == "application/json" {
			var body map[string]interface{}
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				t.Fatalf("decode request body: %v", err)
			}
			seen[r.URL.Path] = body
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()
	configureRESTTest(t, server)

	if err := KickPlayer("steam_1", "Please reconnect"); err != nil {
		t.Fatal(err)
	}
	if err := BanPlayer("xbox_2", "Rule violation"); err != nil {
		t.Fatal(err)
	}
	if err := UnbanPlayer("xbox_2"); err != nil {
		t.Fatal(err)
	}
	if err := SaveWorld(); err != nil {
		t.Fatal(err)
	}
	if err := StopServer(); err != nil {
		t.Fatal(err)
	}

	if seen["/v1/api/kick"]["message"] != "Please reconnect" {
		t.Fatalf("kick message was not forwarded: %#v", seen["/v1/api/kick"])
	}
	if seen["/v1/api/ban"]["userid"] != "xbox_2" {
		t.Fatalf("cross-platform user ID was not forwarded: %#v", seen["/v1/api/ban"])
	}
	if _, ok := seen["/v1/api/unban"]["message"]; ok {
		t.Fatalf("empty unban message should be omitted: %#v", seen["/v1/api/unban"])
	}
}

func TestSettingsSnapshotAndRESTError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requireRESTAuth(t, r)
		switch r.URL.Path {
		case "/v1/api/settings":
			_, _ = w.Write([]byte(`{"ServerName":"Test","RESTAPIEnabled":true}`))
		case "/v1/api/game-data":
			_, _ = w.Write([]byte(`{"Time":"2026-07-10 12:00:00","FPS":60,"AverageFPS":59.5,"ActorData":[]}`))
		default:
			http.Error(w, "unauthorized", http.StatusUnauthorized)
		}
	}))
	defer server.Close()
	configureRESTTest(t, server)

	settings, err := Settings()
	if err != nil {
		t.Fatal(err)
	}
	if settings["ServerName"] != "Test" {
		t.Fatalf("unexpected settings: %#v", settings)
	}

	snapshot, err := WorldActorSnapshot()
	if err != nil || !json.Valid(snapshot) {
		t.Fatalf("unexpected snapshot: %s, %v", snapshot, err)
	}

	_, err = callAPI(http.MethodGet, "/missing", nil)
	var restErr *RESTError
	if !errors.As(err, &restErr) || restErr.StatusCode != http.StatusUnauthorized {
		t.Fatalf("unexpected REST error: %#v", err)
	}
}
