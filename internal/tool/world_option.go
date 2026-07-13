package tool

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/zaigie/palworld-server-tool/internal/database"
	"go.etcd.io/bbolt"
)

var (
	ErrWorldOptionConflict = errors.New("WorldOption.sav changed since it was loaded")
	worldOptionSyncMu      sync.Mutex
	worldOptionEditor      = runWorldOptionEditor
)

const maxWorldOptionSize = 16 << 20

type WorldOptionSyncOptions struct {
	ConfigContent        string
	ExpectedSHA256       string
	ConfirmServerStopped bool
}

type WorldOptionMutation struct {
	Created        bool      `json:"created"`
	GameVersion    string    `json:"game_version"`
	UpdatedKeys    []string  `json:"updated_keys"`
	SkippedKeys    []string  `json:"skipped_keys"`
	SettingsDigest string    `json:"settings_digest"`
	SHA256         string    `json:"sha256"`
	ModifiedAt     time.Time `json:"modified_at"`
}

type WorldOptionSyncResult struct {
	WorldOption  WorldOptionMutation `json:"world_option"`
	SafetyBackup database.Backup     `json:"safety_backup"`
}

type worldOptionEditorResult struct {
	Created        bool     `json:"created"`
	GameVersion    string   `json:"game_version"`
	UpdatedKeys    []string `json:"updated_keys"`
	SkippedKeys    []string `json:"skipped_keys"`
	SettingsDigest string   `json:"settings_digest"`
}

func ValidateWorldOptionSync(content, expectedSHA256 string) error {
	_, _, err := inspectWorldOptionSyncTarget(content, expectedSHA256)
	return err
}

func SyncWorldOption(ctx context.Context, db *bbolt.DB, options WorldOptionSyncOptions) (WorldOptionSyncResult, error) {
	worldOptionSyncMu.Lock()
	defer worldOptionSyncMu.Unlock()

	if !options.ConfirmServerStopped {
		return WorldOptionSyncResult{}, ErrSaveEditConfirmation
	}
	if db == nil {
		return WorldOptionSyncResult{}, errors.New("backup database is required")
	}
	status := GetServerControlStatus(ctx)
	if status.Online || status.Running {
		return WorldOptionSyncResult{}, ErrGameServerRunning
	}

	targetPath, expectedFingerprint, err := inspectWorldOptionSyncTarget(
		options.ConfigContent,
		options.ExpectedSHA256,
	)
	if err != nil {
		return WorldOptionSyncResult{}, err
	}
	created := expectedFingerprint == nil

	settingsFile, err := os.CreateTemp("", ".pst-world-option-settings-*.ini")
	if err != nil {
		return WorldOptionSyncResult{}, fmt.Errorf("stage WorldOption settings: %w", err)
	}
	settingsPath := settingsFile.Name()
	defer os.Remove(settingsPath)
	if _, err := settingsFile.WriteString(options.ConfigContent); err != nil {
		_ = settingsFile.Close()
		return WorldOptionSyncResult{}, fmt.Errorf("write WorldOption settings: %w", err)
	}
	if err := settingsFile.Sync(); err != nil {
		_ = settingsFile.Close()
		return WorldOptionSyncResult{}, fmt.Errorf("sync WorldOption settings: %w", err)
	}
	if err := settingsFile.Close(); err != nil {
		return WorldOptionSyncResult{}, fmt.Errorf("close WorldOption settings: %w", err)
	}

	staged, err := os.CreateTemp(filepath.Dir(targetPath), ".pst-WorldOption-*.sav")
	if err != nil {
		return WorldOptionSyncResult{}, fmt.Errorf("stage WorldOption.sav: %w", err)
	}
	stagedPath := staged.Name()
	if err := staged.Close(); err != nil {
		return WorldOptionSyncResult{}, err
	}
	if err := os.Remove(stagedPath); err != nil {
		return WorldOptionSyncResult{}, err
	}
	defer os.Remove(stagedPath)

	mutation, err := worldOptionEditor(targetPath, stagedPath, settingsPath)
	if err != nil {
		return WorldOptionSyncResult{}, err
	}
	if mutation.Created != created {
		return WorldOptionSyncResult{}, internalSaveEditError(
			"validate WorldOption editor result",
			errors.New("editor reported an inconsistent create state"),
		)
	}
	if err := validateWorldOptionEditorResult(mutation); err != nil {
		return WorldOptionSyncResult{}, internalSaveEditError("validate WorldOption editor result", err)
	}
	stagedFingerprint, err := fingerprintSaveFile(stagedPath)
	if err != nil {
		return WorldOptionSyncResult{}, fmt.Errorf("fingerprint staged WorldOption.sav: %w", err)
	}
	if stagedFingerprint.Size <= 0 || stagedFingerprint.Size > maxWorldOptionSize {
		return WorldOptionSyncResult{}, errors.New("generated WorldOption.sav has an invalid size")
	}

	safetyBackup, err := BackupAndRecord(db)
	if err != nil {
		return WorldOptionSyncResult{}, fmt.Errorf("create pre-sync safety backup: %w", err)
	}
	if err := verifyWorldOptionTargetVersion(targetPath, expectedFingerprint); err != nil {
		return WorldOptionSyncResult{}, err
	}
	if created {
		if err := syncRegularFile(stagedPath); err != nil {
			return WorldOptionSyncResult{}, err
		}
		if err := os.Link(stagedPath, targetPath); err != nil {
			return WorldOptionSyncResult{}, fmt.Errorf("install generated WorldOption.sav: %w", err)
		}
		_ = os.Remove(stagedPath)
	} else if err := stageAndReplaceSaveGuarded(stagedPath, targetPath, expectedFingerprint, nil); err != nil {
		return WorldOptionSyncResult{}, fmt.Errorf("replace WorldOption.sav: %w", err)
	}

	installedFingerprint, err := fingerprintSaveFile(targetPath)
	if err != nil {
		return WorldOptionSyncResult{}, fmt.Errorf("verify installed WorldOption.sav: %w", err)
	}
	if !installedFingerprint.sameContent(stagedFingerprint) {
		return WorldOptionSyncResult{}, internalSaveEditError(
			"verify installed WorldOption.sav",
			errors.New("installed file does not match the validated staged file"),
		)
	}
	info, err := os.Stat(targetPath)
	if err != nil {
		return WorldOptionSyncResult{}, err
	}
	worldOption := WorldOptionMutation{
		Created:        mutation.Created,
		GameVersion:    mutation.GameVersion,
		UpdatedKeys:    mutation.UpdatedKeys,
		SkippedKeys:    mutation.SkippedKeys,
		SettingsDigest: mutation.SettingsDigest,
		SHA256:         hex.EncodeToString(installedFingerprint.SHA256[:]),
		ModifiedAt:     info.ModTime(),
	}
	return WorldOptionSyncResult{WorldOption: worldOption, SafetyBackup: safetyBackup}, nil
}

func inspectWorldOptionSyncTarget(content, expectedSHA256 string) (string, *saveFileFingerprint, error) {
	if err := validateGameConfigContent([]byte(content)); err != nil {
		return "", nil, err
	}
	worldDir, _, err := nativeBackupLocation()
	if err != nil {
		return "", nil, err
	}
	targetPath := filepath.Join(worldDir, "WorldOption.sav")
	info, err := os.Lstat(targetPath)
	if os.IsNotExist(err) {
		if strings.TrimSpace(expectedSHA256) != "" {
			return "", nil, ErrWorldOptionConflict
		}
		return targetPath, nil, nil
	}
	if err != nil {
		return "", nil, err
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() {
		return "", nil, errors.New("WorldOption.sav must be a regular file")
	}
	if info.Size() <= 0 || info.Size() > maxWorldOptionSize {
		return "", nil, errors.New("WorldOption.sav has an invalid size")
	}
	fingerprint, err := fingerprintSaveFile(targetPath)
	if err != nil {
		return "", nil, err
	}
	expected := strings.TrimSpace(expectedSHA256)
	if expected == "" || !strings.EqualFold(expected, hex.EncodeToString(fingerprint.SHA256[:])) {
		return "", nil, ErrWorldOptionConflict
	}
	return targetPath, &fingerprint, nil
}

func verifyWorldOptionTargetVersion(path string, expected *saveFileFingerprint) error {
	if expected == nil {
		if _, err := os.Lstat(path); err == nil {
			return ErrWorldOptionConflict
		} else if !os.IsNotExist(err) {
			return err
		}
		return nil
	}
	if err := verifySaveFileVersion(path, *expected); err != nil {
		return fmt.Errorf("%w: %v", ErrWorldOptionConflict, err)
	}
	return nil
}

func validateWorldOptionEditorResult(result worldOptionEditorResult) error {
	if result.GameVersion != "1.0.0" || len(result.UpdatedKeys) == 0 {
		return errors.New("editor did not report Palworld 1.0.0 settings")
	}
	if len(result.SettingsDigest) != 64 {
		return errors.New("editor reported an invalid settings digest")
	}
	if _, err := hex.DecodeString(result.SettingsDigest); err != nil {
		return errors.New("editor reported an invalid settings digest")
	}
	return nil
}

func runWorldOptionEditor(sourcePath, outputPath, settingsPath string) (worldOptionEditorResult, error) {
	output, err := runSaveEditor([]string{
		"--mode", "sync-world-option",
		"--file", sourcePath,
		"--output", outputPath,
		"--settings-file", settingsPath,
	})
	if err != nil {
		return worldOptionEditorResult{}, err
	}
	const marker = "WORLD_OPTION_RESULT "
	for _, line := range strings.Split(string(output), "\n") {
		index := strings.Index(line, marker)
		if index < 0 {
			continue
		}
		var result worldOptionEditorResult
		if err := json.Unmarshal([]byte(strings.TrimSpace(line[index+len(marker):])), &result); err != nil {
			return worldOptionEditorResult{}, fmt.Errorf("decode WorldOption editor result: %w", err)
		}
		return result, nil
	}
	return worldOptionEditorResult{}, errors.New("WorldOption editor did not return a result")
}

func syncRegularFile(path string) error {
	file, err := os.OpenFile(path, os.O_RDWR, 0)
	if err != nil {
		return err
	}
	err = file.Sync()
	if closeErr := file.Close(); err == nil {
		err = closeErr
	}
	return err
}
