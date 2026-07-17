package config

import (
	"errors"
	"os"
	"strings"
	"unicode/utf8"

	"github.com/spf13/viper"
	"github.com/zaigie/palworld-server-tool/internal/database"
)

const (
	MinimumWebPasswordLength = 8
	MaximumWebPasswordLength = 512
)

var (
	ErrWebPasswordAlreadyConfigured = errors.New("web administrator password is already configured")
	ErrWebPasswordNotConfigured     = errors.New("web administrator password is not configured")
	ErrWebPasswordManagedByEnv      = errors.New("web administrator password is managed by WEB__PASSWORD")
	ErrWebPasswordTooShort          = errors.New("web administrator password must contain at least 8 characters")
	ErrWebPasswordTooLong           = errors.New("web administrator password must not exceed 512 characters")
)

func WebPasswordConfigured() bool {
	return strings.TrimSpace(viper.GetString("web.password")) != ""
}

func WebPasswordManagedByEnvironment() bool {
	value, exists := os.LookupEnv("WEB__PASSWORD")
	return exists && strings.TrimSpace(value) != ""
}

func InitializeWebPassword(password string) error {
	if err := validateWebPassword(password); err != nil {
		return err
	}

	configUpdateMu.Lock()
	defer configUpdateMu.Unlock()
	if activeDB == nil {
		return errors.New("configuration database is not initialized")
	}
	if WebPasswordConfigured() {
		return ErrWebPasswordAlreadyConfigured
	}
	values, err := database.ListConfigValues(activeDB)
	if err != nil {
		return err
	}
	if stored, ok := values["web.password"].(string); ok && strings.TrimSpace(stored) != "" {
		return ErrWebPasswordAlreadyConfigured
	}
	if err := database.PutConfigValues(activeDB, map[string]any{"web.password": password}); err != nil {
		return err
	}
	viper.Set("web.password", password)
	return nil
}

func ChangeWebPassword(password string) error {
	if err := validateWebPassword(password); err != nil {
		return err
	}

	configUpdateMu.Lock()
	defer configUpdateMu.Unlock()
	if activeDB == nil {
		return errors.New("configuration database is not initialized")
	}
	if WebPasswordManagedByEnvironment() {
		return ErrWebPasswordManagedByEnv
	}
	if !WebPasswordConfigured() {
		return ErrWebPasswordNotConfigured
	}
	if err := database.PutConfigValues(activeDB, map[string]any{"web.password": password}); err != nil {
		return err
	}
	viper.Set("web.password", password)
	return nil
}

func validateWebPassword(password string) error {
	if utf8.RuneCountInString(strings.TrimSpace(password)) < MinimumWebPasswordLength {
		return ErrWebPasswordTooShort
	}
	if utf8.RuneCountInString(password) > MaximumWebPasswordLength {
		return ErrWebPasswordTooLong
	}
	return nil
}
