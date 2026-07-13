package tool

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/spf13/viper"
	"github.com/zaigie/palworld-server-tool/internal/system"
)

const maxGameConfigSize = 1 << 20

var (
	ErrGameConfigNotConfigured = errors.New("palworld.config_path is not configured")
	ErrGameConfigConflict      = errors.New("PalWorldSettings.ini changed since it was loaded")
	gameConfigMu               sync.Mutex
)

type GameConfigFile struct {
	Configured  bool                      `json:"configured"`
	Path        string                    `json:"path,omitempty"`
	Content     string                    `json:"content,omitempty"`
	SHA256      string                    `json:"sha256,omitempty"`
	ModifiedAt  *time.Time                `json:"modified_at,omitempty"`
	WorldOption WorldOptionOverrideStatus `json:"world_option"`
}

type WorldOptionOverrideStatus struct {
	Supported  bool       `json:"supported"`
	Present    bool       `json:"present"`
	Path       string     `json:"path,omitempty"`
	SizeBytes  int64      `json:"size_bytes,omitempty"`
	SHA256     string     `json:"sha256,omitempty"`
	ModifiedAt *time.Time `json:"modified_at,omitempty"`
	Message    string     `json:"message,omitempty"`
}

type GameConfigWriteResult struct {
	SHA256          string    `json:"sha256"`
	BackupPath      string    `json:"backup_path"`
	ModifiedAt      time.Time `json:"modified_at"`
	RestartRequired bool      `json:"restart_required"`
}

func configuredGameConfigPath() (string, error) {
	path := strings.TrimSpace(viper.GetString("palworld.config_path"))
	if path == "" {
		return "", ErrGameConfigNotConfigured
	}
	absPath, err := filepath.Abs(path)
	if err != nil {
		return "", fmt.Errorf("resolve palworld.config_path: %w", err)
	}
	if !strings.EqualFold(filepath.Base(absPath), "PalWorldSettings.ini") {
		return "", errors.New("palworld.config_path must point to PalWorldSettings.ini")
	}
	return filepath.Clean(absPath), nil
}

func ReadGameConfigFile() (GameConfigFile, error) {
	gameConfigMu.Lock()
	defer gameConfigMu.Unlock()

	worldOption := inspectWorldOptionOverride()
	path, err := configuredGameConfigPath()
	if errors.Is(err, ErrGameConfigNotConfigured) {
		return GameConfigFile{Configured: false, WorldOption: worldOption}, nil
	}
	if err != nil {
		return GameConfigFile{}, err
	}
	content, info, err := readAndValidateGameConfig(path)
	if err != nil {
		return GameConfigFile{}, err
	}
	modifiedAt := info.ModTime()
	return GameConfigFile{
		Configured:  true,
		Path:        path,
		Content:     string(content),
		SHA256:      gameConfigDigest(content),
		ModifiedAt:  &modifiedAt,
		WorldOption: worldOption,
	}, nil
}

func inspectWorldOptionOverride() WorldOptionOverrideStatus {
	levelPath, err := localLevelSavePath(strings.TrimSpace(viper.GetString("save.path")))
	if err != nil {
		return WorldOptionOverrideStatus{
			Supported: false,
			Message:   err.Error(),
		}
	}
	path := filepath.Join(filepath.Dir(levelPath), "WorldOption.sav")
	info, err := os.Lstat(path)
	if os.IsNotExist(err) {
		return WorldOptionOverrideStatus{Supported: true, Path: path}
	}
	if err != nil {
		return WorldOptionOverrideStatus{
			Supported: false,
			Path:      path,
			Message:   err.Error(),
		}
	}
	modifiedAt := info.ModTime()
	status := WorldOptionOverrideStatus{
		Supported:  true,
		Present:    true,
		Path:       path,
		SizeBytes:  info.Size(),
		ModifiedAt: &modifiedAt,
	}
	if info.Mode()&os.ModeSymlink != 0 {
		status.Message = "WorldOption.sav is a symbolic link and cannot be managed safely"
	} else if !info.Mode().IsRegular() {
		status.Message = "WorldOption.sav is not a regular file"
	} else if fingerprint, fingerprintErr := fingerprintSaveFile(path); fingerprintErr != nil {
		status.Message = fingerprintErr.Error()
	} else {
		status.SHA256 = hex.EncodeToString(fingerprint.SHA256[:])
	}
	return status
}

func WriteGameConfigFile(content, expectedSHA256 string) (GameConfigWriteResult, error) {
	gameConfigMu.Lock()
	defer gameConfigMu.Unlock()

	path, err := configuredGameConfigPath()
	if err != nil {
		return GameConfigWriteResult{}, err
	}
	current, _, err := readAndValidateGameConfig(path)
	if err != nil {
		return GameConfigWriteResult{}, err
	}
	if strings.TrimSpace(expectedSHA256) == "" || !strings.EqualFold(gameConfigDigest(current), expectedSHA256) {
		return GameConfigWriteResult{}, ErrGameConfigConflict
	}

	next := []byte(content)
	if err := validateGameConfigContent(next); err != nil {
		return GameConfigWriteResult{}, err
	}

	backupDir, err := GetBackupDir()
	if err != nil {
		return GameConfigWriteResult{}, fmt.Errorf("prepare config backup directory: %w", err)
	}
	backupDir = filepath.Join(backupDir, "config")
	if err := os.MkdirAll(backupDir, 0o755); err != nil {
		return GameConfigWriteResult{}, fmt.Errorf("prepare config backup directory: %w", err)
	}
	backupPath := filepath.Join(backupDir, fmt.Sprintf(
		"PalWorldSettings-%s-%s.ini",
		time.Now().Format("20060102-150405.000000000"),
		gameConfigDigest(current)[:8],
	))
	if err := system.CopyFile(path, backupPath); err != nil {
		return GameConfigWriteResult{}, fmt.Errorf("back up PalWorldSettings.ini: %w", err)
	}

	staged, err := os.CreateTemp(filepath.Dir(path), ".palworld-settings-*.tmp")
	if err != nil {
		return GameConfigWriteResult{}, fmt.Errorf("stage PalWorldSettings.ini: %w", err)
	}
	stagedPath := staged.Name()
	defer os.Remove(stagedPath)
	if _, err := staged.Write(next); err != nil {
		_ = staged.Close()
		return GameConfigWriteResult{}, fmt.Errorf("write staged PalWorldSettings.ini: %w", err)
	}
	if err := staged.Sync(); err != nil {
		_ = staged.Close()
		return GameConfigWriteResult{}, fmt.Errorf("sync staged PalWorldSettings.ini: %w", err)
	}
	if err := staged.Close(); err != nil {
		return GameConfigWriteResult{}, fmt.Errorf("close staged PalWorldSettings.ini: %w", err)
	}
	if err := system.PrepareReplacementFile(stagedPath, path); err != nil {
		return GameConfigWriteResult{}, fmt.Errorf("preserve PalWorldSettings.ini metadata: %w", err)
	}
	if err := system.ReplaceFileAtomic(stagedPath, path); err != nil {
		return GameConfigWriteResult{}, fmt.Errorf("replace PalWorldSettings.ini: %w", err)
	}

	info, err := os.Stat(path)
	if err != nil {
		return GameConfigWriteResult{}, fmt.Errorf("stat updated PalWorldSettings.ini: %w", err)
	}
	return GameConfigWriteResult{
		SHA256:          gameConfigDigest(next),
		BackupPath:      backupPath,
		ModifiedAt:      info.ModTime(),
		RestartRequired: true,
	}, nil
}

func readAndValidateGameConfig(path string) ([]byte, os.FileInfo, error) {
	linkInfo, err := os.Lstat(path)
	if err != nil {
		return nil, nil, err
	}
	if linkInfo.Mode()&os.ModeSymlink != 0 {
		return nil, nil, errors.New("palworld.config_path cannot be a symbolic link")
	}
	info, err := os.Stat(path)
	if err != nil {
		return nil, nil, err
	}
	if !info.Mode().IsRegular() {
		return nil, nil, errors.New("palworld.config_path is not a regular file")
	}
	if info.Size() > maxGameConfigSize {
		return nil, nil, fmt.Errorf("PalWorldSettings.ini exceeds %d bytes", maxGameConfigSize)
	}
	content, err := os.ReadFile(path)
	if err != nil {
		return nil, nil, err
	}
	if err := validateGameConfigContent(content); err != nil {
		return nil, nil, err
	}
	return content, info, nil
}

func validateGameConfigContent(content []byte) error {
	if len(content) == 0 {
		return errors.New("PalWorldSettings.ini cannot be empty")
	}
	if len(content) > maxGameConfigSize {
		return fmt.Errorf("PalWorldSettings.ini exceeds %d bytes", maxGameConfigSize)
	}
	if !utf8.Valid(content) || strings.ContainsRune(string(content), '\x00') {
		return errors.New("PalWorldSettings.ini must be valid UTF-8 text")
	}
	text := strings.TrimPrefix(string(content), "\ufeff")
	if !strings.Contains(text, "[/Script/Pal.PalGameWorldSettings]") {
		return errors.New("PalWorldSettings.ini is missing the PalGameWorldSettings section")
	}
	marker := "OptionSettings=("
	start := strings.Index(text, marker)
	if start < 0 {
		return errors.New("PalWorldSettings.ini is missing OptionSettings")
	}
	if sectionStart := strings.Index(text, "[/Script/Pal.PalGameWorldSettings]"); sectionStart > start {
		return errors.New("PalWorldSettings.ini OptionSettings appears before the PalGameWorldSettings section")
	}
	depth := 1
	quoted := false
	escaped := false
	closed := false
	closingIndex := -1
	body := text[start+len(marker):]
	for index, character := range body {
		if escaped {
			escaped = false
			continue
		}
		if character == '\\' && quoted {
			escaped = true
			continue
		}
		if character == '"' {
			quoted = !quoted
			continue
		}
		if quoted {
			continue
		}
		switch character {
		case '(':
			depth++
		case ')':
			depth--
			if depth == 0 {
				closed = true
				closingIndex = index
			}
		}
		if closed {
			break
		}
	}
	if !closed || quoted {
		return errors.New("PalWorldSettings.ini contains an incomplete OptionSettings value")
	}
	if strings.TrimSpace(body[:closingIndex]) == "" {
		return errors.New("PalWorldSettings.ini OptionSettings cannot be empty")
	}
	return nil
}

func gameConfigDigest(content []byte) string {
	digest := sha256.Sum256(content)
	return hex.EncodeToString(digest[:])
}
