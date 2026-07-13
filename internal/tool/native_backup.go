package tool

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/spf13/viper"
	"github.com/zaigie/palworld-server-tool/internal/database"
	"github.com/zaigie/palworld-server-tool/internal/system"
	"go.etcd.io/bbolt"
)

var (
	ErrNativeBackupNotConfigured = errors.New("local save.path is required for native backup management")
	ErrNativeBackupInvalid       = errors.New("Palworld native backup is invalid")
	ErrNativeBackupChanged       = errors.New("Palworld native backup changed since it was loaded")
	nativeBackupRestoreMu        sync.Mutex
)

var nativeRestoreEntries = []string{"Level.sav", "LevelMeta.sav", "Players", "WorldOption.sav"}

type NativeBackup struct {
	BackupID       string    `json:"backup_id"`
	CreatedAt      time.Time `json:"created_at"`
	ModifiedAt     time.Time `json:"modified_at"`
	SizeBytes      int64     `json:"size_bytes"`
	FileCount      int       `json:"file_count"`
	PlayerFiles    int       `json:"player_files"`
	HasWorldOption bool      `json:"has_world_option"`
	Digest         string    `json:"digest"`
	Valid          bool      `json:"valid"`
	Issues         []string  `json:"issues,omitempty"`
}

type NativeBackupCatalog struct {
	Configured bool           `json:"configured"`
	Available  bool           `json:"available"`
	WorldID    string         `json:"world_id,omitempty"`
	Backups    []NativeBackup `json:"backups"`
}

type NativeBackupRestoreOptions struct {
	BackupID             string
	ExpectedDigest       string
	ConfirmServerStopped bool
}

type NativeBackupRestoreResult struct {
	RestoredBackup NativeBackup    `json:"restored_backup"`
	SafetyBackup   database.Backup `json:"safety_backup"`
}

func nativeBackupLocation() (worldDir string, backupRoot string, err error) {
	levelPath, err := localLevelSavePath(strings.TrimSpace(viper.GetString("save.path")))
	if err != nil {
		if errors.Is(err, ErrUnsupportedSaveSource) {
			return "", "", ErrNativeBackupNotConfigured
		}
		return "", "", err
	}
	worldDir = filepath.Dir(levelPath)
	return worldDir, filepath.Join(worldDir, "backup", "world"), nil
}

func ListNativeBackups() (NativeBackupCatalog, error) {
	worldDir, backupRoot, err := nativeBackupLocation()
	if err != nil {
		if errors.Is(err, ErrNativeBackupNotConfigured) || strings.Contains(err.Error(), "save path is not configured") {
			return NativeBackupCatalog{Configured: false, Backups: []NativeBackup{}}, nil
		}
		return NativeBackupCatalog{}, err
	}
	catalog := NativeBackupCatalog{
		Configured: true,
		WorldID:    filepath.Base(worldDir),
		Backups:    []NativeBackup{},
	}
	if err := validateNativeBackupRoot(backupRoot); err != nil {
		if os.IsNotExist(err) {
			return catalog, nil
		}
		return NativeBackupCatalog{}, err
	}
	entries, err := os.ReadDir(backupRoot)
	if err != nil {
		if os.IsNotExist(err) {
			return catalog, nil
		}
		return NativeBackupCatalog{}, err
	}
	catalog.Available = true
	for _, entry := range entries {
		if !entry.IsDir() || entry.Type()&os.ModeSymlink != 0 {
			continue
		}
		backup, inspectErr := inspectNativeBackup(filepath.Join(backupRoot, entry.Name()), entry.Name())
		if inspectErr != nil {
			backup.Valid = false
			backup.Issues = append(backup.Issues, inspectErr.Error())
		}
		catalog.Backups = append(catalog.Backups, backup)
	}
	sort.Slice(catalog.Backups, func(i, j int) bool {
		return catalog.Backups[i].CreatedAt.After(catalog.Backups[j].CreatedAt)
	})
	return catalog, nil
}

func inspectNativeBackup(path, backupID string) (NativeBackup, error) {
	backup := NativeBackup{BackupID: backupID, Valid: true}
	info, err := os.Lstat(path)
	if err != nil {
		return backup, err
	}
	backup.ModifiedAt = info.ModTime()
	backup.CreatedAt = info.ModTime()
	if parsed, parseErr := time.ParseInLocation("2006.01.02-15.04.05", backupID, time.Local); parseErr == nil {
		backup.CreatedAt = parsed
	}
	if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return backup, fmt.Errorf("%w: backup root is not a regular directory", ErrNativeBackupInvalid)
	}

	hasher := sha256.New()
	required := map[string]bool{"Level.sav": false, "LevelMeta.sav": false, "Players": false}
	err = filepath.WalkDir(path, func(current string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.Type()&os.ModeSymlink != 0 {
			return fmt.Errorf("%w: symbolic links are not supported", ErrNativeBackupInvalid)
		}
		relative, err := filepath.Rel(path, current)
		if err != nil {
			return err
		}
		if relative == "." {
			return nil
		}
		parts := strings.Split(relative, string(os.PathSeparator))
		topLevel := parts[0]
		entryInfo, err := entry.Info()
		if err != nil {
			return err
		}
		if len(parts) == 1 {
			switch topLevel {
			case "Level.sav", "LevelMeta.sav", "WorldOption.sav":
				if entry.IsDir() || !entryInfo.Mode().IsRegular() || entryInfo.Size() <= 0 {
					return fmt.Errorf("%w: %s must be a non-empty regular file", ErrNativeBackupInvalid, topLevel)
				}
			case "Players":
				if !entry.IsDir() {
					return fmt.Errorf("%w: Players must be a directory", ErrNativeBackupInvalid)
				}
			}
		}
		if _, allowed := required[topLevel]; allowed {
			required[topLevel] = true
		} else if topLevel == "WorldOption.sav" {
			backup.HasWorldOption = true
		} else {
			return fmt.Errorf("%w: unsupported top-level entry %q", ErrNativeBackupInvalid, topLevel)
		}
		fmt.Fprintf(hasher, "%s\x00%d\x00%t\n", filepath.ToSlash(relative), entryInfo.Size(), entry.IsDir())
		if entry.IsDir() {
			return nil
		}
		if !entryInfo.Mode().IsRegular() || entryInfo.Size() <= 0 {
			return fmt.Errorf("%w: %q is not a non-empty regular file", ErrNativeBackupInvalid, relative)
		}
		backup.FileCount++
		backup.SizeBytes += entryInfo.Size()
		if topLevel == "Players" && strings.EqualFold(filepath.Ext(entry.Name()), ".sav") {
			backup.PlayerFiles++
		}
		fingerprint, err := fingerprintSaveFile(current)
		if err != nil {
			return err
		}
		hasher.Write(fingerprint.SHA256[:])
		return nil
	})
	if err != nil {
		return backup, err
	}
	for name, present := range required {
		if !present {
			backup.Issues = append(backup.Issues, fmt.Sprintf("missing %s", name))
		}
	}
	if len(backup.Issues) > 0 {
		backup.Valid = false
		return backup, fmt.Errorf("%w: %s", ErrNativeBackupInvalid, strings.Join(backup.Issues, ", "))
	}
	backup.Digest = hex.EncodeToString(hasher.Sum(nil))
	return backup, nil
}

func ValidateNativeBackupSelection(backupID, expectedDigest string) (NativeBackup, error) {
	_, _, backup, err := resolveNativeBackupSelection(backupID, expectedDigest)
	return backup, err
}

func resolveNativeBackupSelection(backupID, expectedDigest string) (string, string, NativeBackup, error) {
	if err := validateNativeBackupID(backupID); err != nil {
		return "", "", NativeBackup{}, err
	}
	if strings.TrimSpace(expectedDigest) == "" {
		return "", "", NativeBackup{}, errors.New("expected native backup digest is required")
	}
	worldDir, backupRoot, err := nativeBackupLocation()
	if err != nil {
		return "", "", NativeBackup{}, err
	}
	if err := validateNativeBackupRoot(backupRoot); err != nil {
		return "", "", NativeBackup{}, err
	}
	backupPath := filepath.Join(backupRoot, backupID)
	backup, err := inspectNativeBackup(backupPath, backupID)
	if err != nil {
		return "", "", NativeBackup{}, err
	}
	if !strings.EqualFold(backup.Digest, expectedDigest) {
		return "", "", NativeBackup{}, fmt.Errorf(
			"%w: expected %s, found %s",
			ErrNativeBackupChanged,
			expectedDigest,
			backup.Digest,
		)
	}
	return worldDir, backupPath, backup, nil
}

func validateNativeBackupRoot(path string) error {
	info, err := os.Lstat(path)
	if err != nil {
		return err
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
		return fmt.Errorf("%w: backup/world must be a regular directory", ErrNativeBackupInvalid)
	}
	return nil
}

func RestoreNativeBackup(ctx context.Context, db *bbolt.DB, options NativeBackupRestoreOptions) (NativeBackupRestoreResult, error) {
	nativeBackupRestoreMu.Lock()
	defer nativeBackupRestoreMu.Unlock()

	if !options.ConfirmServerStopped {
		return NativeBackupRestoreResult{}, ErrSaveEditConfirmation
	}
	if db == nil {
		return NativeBackupRestoreResult{}, errors.New("backup database is required")
	}
	status := GetServerControlStatus(ctx)
	if status.Online || status.Running {
		return NativeBackupRestoreResult{}, ErrGameServerRunning
	}

	worldDir, backupPath, backup, err := resolveNativeBackupSelection(options.BackupID, options.ExpectedDigest)
	if err != nil {
		return NativeBackupRestoreResult{}, err
	}

	levelPath := filepath.Join(worldDir, "Level.sav")
	currentFingerprint, err := fingerprintSaveFile(levelPath)
	if err != nil {
		return NativeBackupRestoreResult{}, err
	}
	safetyBackup, err := BackupAndRecord(db)
	if err != nil {
		return NativeBackupRestoreResult{}, fmt.Errorf("create pre-restore safety backup: %w", err)
	}
	if err := verifySaveFileVersion(levelPath, currentFingerprint); err != nil {
		return NativeBackupRestoreResult{}, err
	}

	transactionDir := filepath.Join(filepath.Dir(worldDir), ".pst-native-restore-"+uuid.NewString())
	newDir := filepath.Join(transactionDir, "new")
	oldDir := filepath.Join(transactionDir, "old")
	if err := os.MkdirAll(newDir, 0o700); err != nil {
		return NativeBackupRestoreResult{}, err
	}
	defer os.RemoveAll(transactionDir)
	if err := stageNativeBackup(backupPath, newDir); err != nil {
		return NativeBackupRestoreResult{}, err
	}
	stagedBackup, err := inspectNativeBackup(newDir, options.BackupID)
	if err != nil {
		return NativeBackupRestoreResult{}, fmt.Errorf("validate staged native backup: %w", err)
	}
	if stagedBackup.Digest != backup.Digest {
		return NativeBackupRestoreResult{}, fmt.Errorf(
			"%w: staged snapshot digest changed from %s to %s",
			ErrNativeBackupChanged,
			backup.Digest,
			stagedBackup.Digest,
		)
	}
	backupAfterStage, err := inspectNativeBackup(backupPath, options.BackupID)
	if err != nil {
		return NativeBackupRestoreResult{}, fmt.Errorf("%w: revalidate source: %v", ErrNativeBackupChanged, err)
	}
	if backupAfterStage.Digest != backup.Digest {
		return NativeBackupRestoreResult{}, fmt.Errorf(
			"%w: snapshot digest changed from %s to %s",
			ErrNativeBackupChanged,
			backup.Digest,
			backupAfterStage.Digest,
		)
	}
	if err := verifySaveFileVersion(levelPath, currentFingerprint); err != nil {
		return NativeBackupRestoreResult{}, err
	}
	if err := swapNativeBackupIntoWorld(worldDir, newDir, oldDir); err != nil {
		return NativeBackupRestoreResult{}, err
	}
	return NativeBackupRestoreResult{RestoredBackup: backup, SafetyBackup: safetyBackup}, nil
}

func validateNativeBackupID(backupID string) error {
	backupID = strings.TrimSpace(backupID)
	if backupID == "" || backupID == "." || backupID == ".." || filepath.Base(backupID) != backupID || strings.ContainsAny(backupID, `/\\`) {
		return errors.New("invalid native backup ID")
	}
	return nil
}

func stageNativeBackup(sourceDir, destinationDir string) error {
	for _, name := range nativeRestoreEntries {
		sourcePath := filepath.Join(sourceDir, name)
		if _, err := os.Lstat(sourcePath); err != nil {
			if os.IsNotExist(err) && name == "WorldOption.sav" {
				continue
			}
			return err
		}
		if err := copyRestorePath(sourcePath, filepath.Join(destinationDir, name)); err != nil {
			return fmt.Errorf("stage %s: %w", name, err)
		}
	}
	return nil
}

func copyRestorePath(sourcePath, destinationPath string) error {
	info, err := os.Lstat(sourcePath)
	if err != nil {
		return err
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return errors.New("symbolic links are not supported")
	}
	if info.IsDir() {
		if err := os.MkdirAll(destinationPath, info.Mode().Perm()); err != nil {
			return err
		}
		entries, err := os.ReadDir(sourcePath)
		if err != nil {
			return err
		}
		for _, entry := range entries {
			if err := copyRestorePath(filepath.Join(sourcePath, entry.Name()), filepath.Join(destinationPath, entry.Name())); err != nil {
				return err
			}
		}
		return nil
	}
	if !info.Mode().IsRegular() {
		return errors.New("only regular files and directories can be restored")
	}
	if err := os.MkdirAll(filepath.Dir(destinationPath), 0o700); err != nil {
		return err
	}
	if err := system.CopyFile(sourcePath, destinationPath); err != nil {
		return err
	}
	return os.Chmod(destinationPath, info.Mode().Perm())
}

func swapNativeBackupIntoWorld(worldDir, newDir, oldDir string) error {
	if err := os.MkdirAll(oldDir, 0o700); err != nil {
		return err
	}
	var movedOld []string
	var movedNew []string
	rollback := func(operationErr error) error {
		var rollbackErrors []string
		for index := len(movedNew) - 1; index >= 0; index-- {
			if err := os.RemoveAll(filepath.Join(worldDir, movedNew[index])); err != nil {
				rollbackErrors = append(rollbackErrors, err.Error())
			}
		}
		for index := len(movedOld) - 1; index >= 0; index-- {
			name := movedOld[index]
			if err := os.Rename(filepath.Join(oldDir, name), filepath.Join(worldDir, name)); err != nil {
				rollbackErrors = append(rollbackErrors, err.Error())
			}
		}
		if len(rollbackErrors) > 0 {
			return fmt.Errorf("restore failed: %v; rollback failed: %s", operationErr, strings.Join(rollbackErrors, "; "))
		}
		return operationErr
	}

	for _, name := range nativeRestoreEntries {
		targetPath := filepath.Join(worldDir, name)
		if _, err := os.Lstat(targetPath); err == nil {
			if err := os.Rename(targetPath, filepath.Join(oldDir, name)); err != nil {
				return rollback(err)
			}
			movedOld = append(movedOld, name)
		} else if !os.IsNotExist(err) {
			return rollback(err)
		}
	}
	for _, name := range nativeRestoreEntries {
		stagedPath := filepath.Join(newDir, name)
		if _, err := os.Lstat(stagedPath); err == nil {
			if err := os.Rename(stagedPath, filepath.Join(worldDir, name)); err != nil {
				return rollback(err)
			}
			movedNew = append(movedNew, name)
		} else if !os.IsNotExist(err) {
			return rollback(err)
		}
	}
	return nil
}
