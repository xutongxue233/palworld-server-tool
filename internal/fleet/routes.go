package fleet

import (
	"net/http"
	"net/url"
	"strings"
	"unicode/utf8"
)

type proxyRoute struct {
	method   string
	segments []string
	long     bool
}

var allowedProxyRoutes = []proxyRoute{
	{http.MethodGet, []string{"server"}, false},
	{http.MethodGet, []string{"server", "tool"}, false},
	{http.MethodGet, []string{"server", "metrics"}, false},
	{http.MethodGet, []string{"server", "settings"}, false},
	{http.MethodGet, []string{"server", "game-data"}, false},
	{http.MethodGet, []string{"server", "config-file"}, false},
	{http.MethodPut, []string{"server", "config-file"}, false},
	{http.MethodPut, []string{"server", "world-option"}, true},
	{http.MethodGet, []string{"server", "control", "status"}, false},
	{http.MethodGet, []string{"server", "steamcmd"}, false},
	{http.MethodPost, []string{"server", "steamcmd", "update"}, true},
	{http.MethodGet, []string{"server", "mods"}, false},
	{http.MethodPost, []string{"server", "mods", "preflight"}, false},
	{http.MethodPost, []string{"server", "mods", "apply"}, true},
	{http.MethodPost, []string{"server", "migration", "preflight"}, true},
	{http.MethodPost, []string{"server", "migration", "apply"}, true},
	{http.MethodGet, []string{"server", "backups", "native"}, false},
	{http.MethodPost, []string{"server", "backups", "native", ":backup", "restore"}, true},
	{http.MethodPost, []string{"server", "start"}, true},
	{http.MethodPost, []string{"server", "restart"}, true},
	{http.MethodPost, []string{"server", "broadcast"}, false},
	{http.MethodPost, []string{"server", "save"}, false},
	{http.MethodPost, []string{"server", "shutdown"}, true},
	{http.MethodPost, []string{"server", "stop"}, true},
	{http.MethodGet, []string{"guild"}, false},
	{http.MethodGet, []string{"guild", ":admin"}, false},
	{http.MethodPut, []string{"guild"}, false},
	{http.MethodGet, []string{"online_player"}, false},
	{http.MethodGet, []string{"player"}, false},
	{http.MethodGet, []string{"player", ":player"}, false},
	{http.MethodPut, []string{"player"}, false},
	{http.MethodPost, []string{"player", ":player", "kick"}, false},
	{http.MethodPost, []string{"player", ":player", "ban"}, false},
	{http.MethodPost, []string{"player", ":player", "unban"}, false},
	{http.MethodPost, []string{"player", ":player", "items"}, true},
	{http.MethodPatch, []string{"player", ":player", "items", ":container", ":slot"}, true},
	{http.MethodPatch, []string{"player", ":player", "profile"}, true},
	{http.MethodPatch, []string{"player", ":player", "stat-points"}, true},
	{http.MethodPatch, []string{"player", ":player", "technology-points"}, true},
	{http.MethodPatch, []string{"player", ":player", "map-progress"}, true},
	{http.MethodPatch, []string{"player", ":player", "pals", ":instance", "nickname"}, true},
	{http.MethodPatch, []string{"player", ":player", "pals", ":instance", "level"}, true},
	{http.MethodPatch, []string{"player", ":player", "pals", ":instance", "health"}, true},
	{http.MethodPost, []string{"sync"}, true},
	{http.MethodGet, []string{"whitelist"}, false},
	{http.MethodPost, []string{"whitelist"}, false},
	{http.MethodPut, []string{"whitelist"}, false},
	{http.MethodDelete, []string{"whitelist"}, false},
	{http.MethodGet, []string{"backup"}, false},
	{http.MethodGet, []string{"backup", ":backup"}, true},
	{http.MethodDelete, []string{"backup", ":backup"}, false},
	{http.MethodGet, []string{"automation", "tasks"}, false},
	{http.MethodPost, []string{"automation", "tasks"}, false},
	{http.MethodPut, []string{"automation", "tasks", ":task"}, false},
	{http.MethodDelete, []string{"automation", "tasks", ":task"}, false},
	{http.MethodPost, []string{"automation", "tasks", ":task", "run"}, true},
	{http.MethodGet, []string{"automation", "runs"}, false},
	{http.MethodGet, []string{"automation", "settings"}, false},
	{http.MethodPut, []string{"automation", "settings"}, false},
	{http.MethodGet, []string{"automation", "status"}, false},
	{http.MethodPost, []string{"automation", "notifications", "test"}, false},
	{http.MethodPost, []string{"automation", "watchdog", "reset"}, false},
	{http.MethodPost, []string{"rcon"}, false},
}

func ProxyRouteAllowed(method, targetPath string) (bool, bool) {
	if !strings.HasPrefix(targetPath, "/api/") || strings.Contains(targetPath, "\\") || strings.Contains(targetPath, "//") {
		return false, false
	}
	trimmed := strings.TrimPrefix(targetPath, "/api/")
	segments := strings.Split(trimmed, "/")
	for _, route := range allowedProxyRoutes {
		if route.method != method || len(route.segments) != len(segments) {
			continue
		}
		matched := true
		for index, expected := range route.segments {
			actual, err := url.PathUnescape(segments[index])
			if err != nil {
				matched = false
				break
			}
			if strings.HasPrefix(expected, ":") {
				if !validProxySegment(actual) {
					matched = false
					break
				}
			} else if actual != expected {
				matched = false
				break
			}
		}
		if matched {
			return true, route.long
		}
	}
	return false, false
}

func validProxySegment(value string) bool {
	if value == "" || value == "." || value == ".." || len(value) > 256 || !utf8.ValidString(value) ||
		strings.ContainsAny(value, "/\\\x00\r\n") {
		return false
	}
	for _, character := range value {
		if character < 0x20 || character == 0x7f {
			return false
		}
	}
	return true
}
