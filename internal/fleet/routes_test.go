package fleet

import (
	"net/http"
	"testing"
)

func TestProxyRouteAllowlistCoversKnownAPIsAndBlocksRecursion(t *testing.T) {
	for _, test := range []struct {
		method string
		path   string
		long   bool
	}{
		{http.MethodGet, "/api/server", false},
		{http.MethodPut, "/api/server/config-file", false},
		{http.MethodPost, "/api/server/mods/apply", true},
		{http.MethodPost, "/api/server/backups/native/2026.07.13-10.00.00/restore", true},
		{http.MethodPatch, "/api/player/ABC/items/Inventory/3", true},
		{http.MethodPost, "/api/automation/tasks/task-1/run", true},
		{http.MethodGet, "/api/backup/backup-id", true},
	} {
		allowed, long := ProxyRouteAllowed(test.method, test.path)
		if !allowed || long != test.long {
			t.Fatalf("expected %s %s to be allowed (long=%v), got allowed=%v long=%v", test.method, test.path, test.long, allowed, long)
		}
	}

	for _, test := range []struct {
		method string
		path   string
	}{
		{http.MethodPost, "/api/login"},
		{http.MethodGet, "/api/fleet/nodes"},
		{http.MethodGet, "/api/fleet/node/status"},
		{http.MethodPost, "/api/server"},
		{http.MethodGet, "/api/unknown"},
		{http.MethodGet, "/api/player/../settings"},
		{http.MethodGet, "/api/player/%2Fetc"},
		{http.MethodGet, "/api//server"},
	} {
		allowed, _ := ProxyRouteAllowed(test.method, test.path)
		if allowed {
			t.Fatalf("unexpected proxy allowance for %s %s", test.method, test.path)
		}
	}
}
