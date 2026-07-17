package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"slices"
	"strings"

	"github.com/zaigie/palworld-server-tool/internal/database"
	"github.com/zaigie/palworld-server-tool/internal/fleet"
)

var runtimeStringKeys = map[string]int{
	"palworld.config_path":               4096,
	"palworld.control.mode":              32,
	"palworld.control.target":            4096,
	"palworld.control.working_directory": 4096,
	"web.cert_path":                      4096,
	"web.key_path":                       4096,
	"web.public_url":                     4096,
	"task.player_login_message":          4096,
	"task.player_logout_message":         4096,
	"steamcmd.executable":                4096,
	"steamcmd.install_dir":               4096,
	"mods.install_dir":                   4096,
	"fleet.node_id":                      48,
	"fleet.node_name":                    128,
	"fleet.node_token":                   512,
	"rcon.address":                       512,
	"rcon.password":                      512,
	"rest.address":                       4096,
	"rest.username":                      128,
	"rest.password":                      512,
	"save.path":                          4096,
	"save.decode_path":                   4096,
}

var runtimeBoolKeys = map[string]struct{}{
	"web.tls":                   {},
	"task.player_logging":       {},
	"rcon.use_base64":           {},
	"manage.kick_non_whitelist": {},
}

var runtimeIntegerRanges = map[string][2]int{
	"palworld.control.timeout": {1, 7200},
	"web.port":                 {1, 65535},
	"task.sync_interval":       {1, 86400},
	"steamcmd.timeout":         {60, 7200},
	"fleet.timeout_seconds":    {2, 120},
	"rcon.timeout":             {1, 120},
	"rest.timeout":             {1, 120},
	"save.sync_interval":       {1, 86400},
	"save.backup_interval":     {0, 604800},
	"save.backup_keep_days":    {1, 3650},
}

var sensitiveRuntimeKeys = map[string]struct{}{
	"web.password":                        {},
	"automation.notification.webhook_url": {},
	"automation.notification.secret":      {},
	"fleet.node_token":                    {},
	"fleet.nodes":                         {},
	"rcon.password":                       {},
	"rest.password":                       {},
}

type RuntimeSnapshot struct {
	Values            map[string]any `json:"values"`
	ConfiguredSecrets []string       `json:"configured_secrets"`
}

func RuntimeValues() (RuntimeSnapshot, error) {
	if activeDB == nil {
		return RuntimeSnapshot{}, errors.New("configuration database is not initialized")
	}
	values, err := database.ListConfigValues(activeDB)
	if err != nil {
		return RuntimeSnapshot{}, err
	}
	secrets := make([]string, 0, len(sensitiveRuntimeKeys))
	for key := range sensitiveRuntimeKeys {
		value, exists := values[key]
		if !exists {
			continue
		}
		if hasConfiguredValue(value) {
			secrets = append(secrets, key)
		}
		delete(values, key)
	}
	slices.Sort(secrets)
	return RuntimeSnapshot{Values: values, ConfiguredSecrets: secrets}, nil
}

func ApplyRuntimeValues(values map[string]any) ([]string, error) {
	clean := make(map[string]any, len(values))
	restartRequired := make([]string, 0, len(values))
	for key, value := range values {
		key = strings.TrimSpace(key)
		normalized, err := validateRuntimeValue(key, value)
		if err != nil {
			return nil, err
		}
		clean[key] = normalized
		restartRequired = append(restartRequired, key)
	}
	_, err := ApplyValuesWithRuntime(clean)
	if err != nil {
		return nil, err
	}
	slices.Sort(restartRequired)
	return restartRequired, nil
}

func validateRuntimeValue(key string, value any) (any, error) {
	if limit, ok := runtimeStringKeys[key]; ok {
		text, ok := value.(string)
		if !ok {
			return nil, fmt.Errorf("configuration key %s must be a string", key)
		}
		if len(text) > limit {
			return nil, fmt.Errorf("configuration key %s is too long", key)
		}
		if key == "palworld.control.mode" {
			mode := strings.ToLower(strings.TrimSpace(text))
			if !slices.Contains([]string{"disabled", "process", "docker", "systemd", "windows_service"}, mode) {
				return nil, errors.New("palworld.control.mode is not supported")
			}
			return mode, nil
		}
		if key == "fleet.node_id" && !fleet.ValidNodeID(text) {
			return nil, errors.New("fleet.node_id is invalid")
		}
		if key == "fleet.node_token" && text != "" && !fleet.ValidTokenSecret(text) {
			return nil, errors.New("fleet.node_token must contain 32-512 non-whitespace printable ASCII characters")
		}
		return text, nil
	}
	if _, ok := runtimeBoolKeys[key]; ok {
		flag, ok := value.(bool)
		if !ok {
			return nil, fmt.Errorf("configuration key %s must be a boolean", key)
		}
		return flag, nil
	}
	if bounds, ok := runtimeIntegerRanges[key]; ok {
		number, ok := integerValue(value)
		if !ok || number < bounds[0] || number > bounds[1] {
			return nil, fmt.Errorf("configuration key %s must be an integer between %d and %d", key, bounds[0], bounds[1])
		}
		return number, nil
	}
	switch key {
	case "palworld.control.arguments":
		return validateStringList(key, value)
	case "fleet.nodes":
		return validateFleetNodes(value)
	default:
		return nil, fmt.Errorf("configuration key %s is not editable", key)
	}
}

func integerValue(value any) (int, bool) {
	switch number := value.(type) {
	case int:
		return number, true
	case int64:
		if int64(int(number)) != number {
			return 0, false
		}
		return int(number), true
	case float64:
		if math.Trunc(number) != number || number < float64(math.MinInt) || number > float64(math.MaxInt) {
			return 0, false
		}
		return int(number), true
	default:
		return 0, false
	}
}

func validateStringList(key string, value any) ([]string, error) {
	raw, err := json.Marshal(value)
	if err != nil {
		return nil, fmt.Errorf("configuration key %s must be a string array", key)
	}
	var values []string
	if err := json.Unmarshal(raw, &values); err != nil || len(values) > 64 {
		return nil, fmt.Errorf("configuration key %s must contain at most 64 strings", key)
	}
	for _, entry := range values {
		if len(entry) > 1024 {
			return nil, fmt.Errorf("configuration key %s contains an entry that is too long", key)
		}
	}
	return values, nil
}

func validateFleetNodes(value any) ([]map[string]any, error) {
	raw, err := json.Marshal(value)
	if err != nil {
		return nil, errors.New("fleet.nodes must be an array")
	}
	var nodes []map[string]any
	if err := json.Unmarshal(raw, &nodes); err != nil || len(nodes) > 32 {
		return nil, errors.New("fleet.nodes must contain at most 32 objects")
	}
	for _, node := range nodes {
		for key := range node {
			if !slices.Contains([]string{"id", "name", "base_url", "token", "allow_private_network", "timeout_seconds"}, key) {
				return nil, fmt.Errorf("fleet.nodes contains unsupported field %s", key)
			}
		}
		id, idOK := node["id"].(string)
		if !idOK || !fleet.ValidNodeID(id) {
			return nil, errors.New("fleet.nodes contains an invalid id")
		}
		baseURL, baseURLOK := node["base_url"].(string)
		if !baseURLOK || strings.TrimSpace(baseURL) == "" || len(baseURL) > 2048 {
			return nil, fmt.Errorf("fleet.nodes[%s] contains an invalid base_url", id)
		}
		token, tokenOK := node["token"].(string)
		if !tokenOK || !fleet.ValidTokenSecret(token) {
			return nil, fmt.Errorf("fleet.nodes[%s] contains an invalid token", id)
		}
		if name, exists := node["name"]; exists {
			text, ok := name.(string)
			if !ok || len(text) > 80 {
				return nil, fmt.Errorf("fleet.nodes[%s] contains an invalid name", id)
			}
		}
		if allowPrivate, exists := node["allow_private_network"]; exists {
			if _, ok := allowPrivate.(bool); !ok {
				return nil, fmt.Errorf("fleet.nodes[%s].allow_private_network must be a boolean", id)
			}
		}
		if timeout, exists := node["timeout_seconds"]; exists {
			seconds, ok := integerValue(timeout)
			if !ok || seconds < 2 || seconds > 120 {
				return nil, fmt.Errorf("fleet.nodes[%s].timeout_seconds must be between 2 and 120", id)
			}
			node["timeout_seconds"] = seconds
		}
	}
	return nodes, nil
}

func hasConfiguredValue(value any) bool {
	switch typed := value.(type) {
	case nil:
		return false
	case string:
		return strings.TrimSpace(typed) != ""
	case []any:
		return len(typed) > 0
	default:
		return true
	}
}
