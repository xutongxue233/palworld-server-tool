package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/zaigie/palworld-server-tool/internal/database"
	"go.etcd.io/bbolt"
	"gopkg.in/yaml.v3"
)

func importLegacyConfig(db *bbolt.DB) (bool, string, error) {
	for _, name := range []string{"config.yaml", "config.yml"} {
		path, err := filepath.Abs(name)
		if err != nil {
			continue
		}
		content, err := os.ReadFile(path)
		if errors.Is(err, os.ErrNotExist) {
			continue
		}
		if err != nil {
			return false, "", err
		}
		var decoded map[string]any
		if err := yaml.Unmarshal(content, &decoded); err != nil {
			return false, "", fmt.Errorf("parse %s: %w", name, err)
		}
		flattened := make(map[string]any)
		flattenMap("", decoded, flattened)
		if len(flattened) == 0 {
			return false, "", nil
		}
		if err := database.PutConfigValues(db, flattened); err != nil {
			return false, "", err
		}
		return true, path, nil
	}
	return false, "", nil
}

func flattenMap(prefix string, values map[string]any, output map[string]any) {
	for key, value := range values {
		key = strings.TrimSpace(key)
		if key == "" {
			continue
		}
		fullKey := key
		if prefix != "" {
			fullKey = prefix + "." + key
		}
		if nested, ok := value.(map[string]any); ok {
			flattenMap(fullKey, nested, output)
			continue
		}
		output[fullKey] = value
	}
}
