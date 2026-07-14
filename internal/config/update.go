package config

import (
	"errors"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/spf13/viper"
	"github.com/zaigie/palworld-server-tool/internal/database"
)

var configUpdateMu sync.Mutex
var runtimeFrozen atomic.Bool

func StorageName() string { return "pst.db" }

func FreezeRuntime() { runtimeFrozen.Store(true) }

func ApplyValues(values map[string]any) error {
	_, err := ApplyValuesWithRuntime(values)
	return err
}

func ApplyValuesWithRuntime(values map[string]any) (bool, error) {
	if len(values) == 0 {
		return !runtimeFrozen.Load(), nil
	}
	configUpdateMu.Lock()
	defer configUpdateMu.Unlock()
	if activeDB == nil {
		return false, errors.New("configuration database is not initialized")
	}
	clean := make(map[string]any, len(values))
	for key, value := range values {
		key = strings.TrimSpace(key)
		if key == "" || strings.HasPrefix(key, ".") || strings.HasSuffix(key, ".") {
			return false, errors.New("configuration key is invalid")
		}
		clean[key] = value
	}
	if err := database.PutConfigValues(activeDB, clean); err != nil {
		return false, err
	}
	if runtimeFrozen.Load() {
		return false, nil
	}
	for key, value := range clean {
		viper.Set(key, value)
	}
	return true, nil
}
