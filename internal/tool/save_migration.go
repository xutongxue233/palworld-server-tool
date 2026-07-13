package tool

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/spf13/viper"
	"github.com/zaigie/palworld-server-tool/internal/database"
	"go.etcd.io/bbolt"
)

const (
	SaveMigrationGameVersion = "1.0.0"
	SaveMigrationDedicated   = "dedicated"
	SaveMigrationCoop        = "coop"
	SaveMigrationCurrent     = "current"
	SaveMigrationWindows     = "windows"
	SaveMigrationLinux       = "linux"

	coopHostPlayerSaveName      = "00000000000000000000000000000001.sav"
	maxMigrationValidatorOutput = 64 * 1024
)

var (
	ErrSaveMigrationNotConfigured = errors.New("local destination save.path is required for save migration")
	ErrSaveMigrationInvalid       = errors.New("save migration preflight failed")
	ErrSaveMigrationChanged       = errors.New("save migration plan or source changed")
	ErrSaveMigrationCoopHost      = errors.New("co-op host GUID conversion is not supported")

	saveMigrationMu            sync.Mutex
	migrationSnapshotValidator = validateMigrationSnapshotWithCLI
	migrationBackupAndRecord   = BackupAndRecord
	migrationStageWorld        = stageNativeBackup
	migrationSwapWorld         = swapNativeBackupIntoWorld
	migrationRollbackWorld     = rollbackSaveMigration
	migrationPlayerSavePattern = regexp.MustCompile(`(?i)^[0-9a-f]{32}\.sav$`)
)

type SaveMigrationNotice struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

type SaveMigrationPlan struct {
	Configured                bool                  `json:"configured"`
	CanMigrate                bool                  `json:"can_migrate"`
	GameVersion               string                `json:"game_version"`
	DestinationPlatform       string                `json:"destination_platform"`
	SourceInput               string                `json:"source_input"`
	SourcePath                string                `json:"source_path,omitempty"`
	SourceWorldID             string                `json:"source_world_id,omitempty"`
	SourcePlatform            string                `json:"source_platform"`
	SourceKind                string                `json:"source_kind"`
	SourceDigest              string                `json:"source_digest,omitempty"`
	SourceSizeBytes           int64                 `json:"source_size_bytes"`
	SourceFileCount           int                   `json:"source_file_count"`
	SourcePlayerFiles         int                   `json:"source_player_files"`
	SourceHasWorldOption      bool                  `json:"source_has_world_option"`
	SourceHasNativeBackups    bool                  `json:"source_has_native_backups"`
	SourceIgnoredEntries      []string              `json:"source_ignored_entries,omitempty"`
	CoopHostDetected          bool                  `json:"coop_host_detected"`
	ValidationPassed          bool                  `json:"validation_passed"`
	DestinationPath           string                `json:"destination_path,omitempty"`
	DestinationWorldID        string                `json:"destination_world_id,omitempty"`
	DestinationDigest         string                `json:"destination_digest,omitempty"`
	DestinationSizeBytes      int64                 `json:"destination_size_bytes"`
	DestinationFileCount      int                   `json:"destination_file_count"`
	DestinationPlayerFiles    int                   `json:"destination_player_files"`
	DestinationHasWorldOption bool                  `json:"destination_has_world_option"`
	Issues                    []SaveMigrationNotice `json:"issues,omitempty"`
	Warnings                  []SaveMigrationNotice `json:"warnings,omitempty"`
	PlanDigest                string                `json:"plan_digest"`
}

type SaveMigrationOptions struct {
	SourcePath           string
	SourcePlatform       string
	SourceKind           string
	ExpectedPlanDigest   string
	ConfirmMigration     bool
	ConfirmServerStopped bool
}

type SaveMigrationResult struct {
	Plan         SaveMigrationPlan `json:"plan"`
	SafetyBackup database.Backup   `json:"safety_backup"`
}

type migrationWorldSnapshot struct {
	WorldDir         string
	WorldID          string
	Digest           string
	SizeBytes        int64
	FileCount        int
	PlayerFiles      int
	HasWorldOption   bool
	HasNativeBackups bool
	CoopHostDetected bool
	IgnoredEntries   []string
}

func InspectSaveMigration(ctx context.Context, sourcePath, sourcePlatform, sourceKind string) SaveMigrationPlan {
	plan := SaveMigrationPlan{
		GameVersion:         SaveMigrationGameVersion,
		DestinationPlatform: runtime.GOOS,
		SourceInput:         strings.TrimSpace(sourcePath),
		SourcePlatform:      strings.ToLower(strings.TrimSpace(sourcePlatform)),
		SourceKind:          strings.ToLower(strings.TrimSpace(sourceKind)),
		Issues:              make([]SaveMigrationNotice, 0),
		Warnings:            make([]SaveMigrationNotice, 0),
	}
	if plan.SourcePlatform == SaveMigrationCurrent {
		plan.SourcePlatform = runtime.GOOS
	}

	if runtime.GOOS != SaveMigrationWindows && runtime.GOOS != SaveMigrationLinux {
		plan.addIssue("destination_platform_unsupported", "save migration is supported only on Windows and Linux")
	}
	if plan.SourceKind != SaveMigrationDedicated && plan.SourceKind != SaveMigrationCoop {
		plan.addIssue("source_kind_required", "source kind must be dedicated or coop")
	}
	if plan.SourcePlatform != SaveMigrationWindows && plan.SourcePlatform != SaveMigrationLinux {
		plan.addIssue("source_platform_required", "source platform must be windows or linux")
	} else if plan.SourcePlatform != runtime.GOOS {
		plan.addIssue(
			"cross_platform_identity_unsupported",
			fmt.Sprintf("automatic %s-to-%s player GUID migration is not supported", plan.SourcePlatform, runtime.GOOS),
		)
	}
	if plan.SourceKind == SaveMigrationCoop {
		plan.addIssue("coop_source_unsupported", "co-op saves require player GUID conversion and cannot be migrated automatically")
	}

	destinationDir, destinationErr := resolveMigrationDestination()
	if destinationErr != nil {
		plan.addIssue("destination_not_configured", destinationErr.Error())
	} else {
		plan.Configured = true
		plan.DestinationPath = destinationDir
		plan.DestinationWorldID = filepath.Base(destinationDir)
		if snapshot, err := inspectMigrationWorld(destinationDir); err != nil {
			plan.addIssue("destination_invalid", err.Error())
		} else {
			plan.DestinationDigest = snapshot.Digest
			plan.DestinationSizeBytes = snapshot.SizeBytes
			plan.DestinationFileCount = snapshot.FileCount
			plan.DestinationPlayerFiles = snapshot.PlayerFiles
			plan.DestinationHasWorldOption = snapshot.HasWorldOption
		}
	}

	sourceDir, sourceErr := resolveMigrationWorld(plan.SourceInput)
	if sourceErr != nil {
		plan.addIssue("source_invalid", sourceErr.Error())
	} else {
		plan.SourcePath = sourceDir
		plan.SourceWorldID = filepath.Base(sourceDir)
		snapshot, err := inspectMigrationWorld(sourceDir)
		if err != nil {
			plan.addIssue("source_invalid", err.Error())
		} else {
			plan.SourceDigest = snapshot.Digest
			plan.SourceSizeBytes = snapshot.SizeBytes
			plan.SourceFileCount = snapshot.FileCount
			plan.SourcePlayerFiles = snapshot.PlayerFiles
			plan.SourceHasWorldOption = snapshot.HasWorldOption
			plan.SourceHasNativeBackups = snapshot.HasNativeBackups
			plan.SourceIgnoredEntries = append([]string(nil), snapshot.IgnoredEntries...)
			plan.CoopHostDetected = snapshot.CoopHostDetected
			if snapshot.CoopHostDetected {
				plan.addIssue(
					"coop_host_detected",
					"Players contains 00000000000000000000000000000001.sav; experimental co-op host GUID conversion is intentionally blocked",
				)
			}
			if snapshot.HasNativeBackups {
				plan.addWarning("source_native_backups_ignored", "the source backup directory is not imported")
			}
			if len(snapshot.IgnoredEntries) > 0 {
				plan.addWarning(
					"source_entries_ignored",
					"unsupported source entries are left untouched and are not imported: "+strings.Join(snapshot.IgnoredEntries, ", "),
				)
			}
			if snapshot.PlayerFiles == 0 {
				plan.addWarning("source_has_no_players", "the source Players directory is empty")
			}
			if err := migrationSnapshotValidator(ctx, sourceDir); err != nil {
				plan.addIssue("source_validation_failed", err.Error())
			} else {
				plan.ValidationPassed = true
			}
		}
	}

	if plan.SourcePath != "" && plan.DestinationPath != "" {
		if sameFilesystemPath(plan.SourcePath, plan.DestinationPath) {
			plan.addIssue("source_is_destination", "source and destination resolve to the same world directory")
		}
		if pathWithin(plan.SourcePath, filepath.Join(plan.DestinationPath, "backup", "world")) {
			plan.addWarning("native_backup_source", "this source is a game-managed backup; the native backup restore workflow is recommended")
		}
		if plan.SourceHasWorldOption {
			plan.addWarning("world_option_imported", "source WorldOption.sav will override PalWorldSettings.ini after migration")
		} else if plan.DestinationHasWorldOption {
			plan.addWarning("world_option_removed", "destination WorldOption.sav will be removed because the source does not contain it")
		}
	}

	plan.Issues = uniqueMigrationNotices(plan.Issues)
	plan.Warnings = uniqueMigrationNotices(plan.Warnings)
	plan.CanMigrate = plan.Configured && plan.SourceDigest != "" && plan.ValidationPassed && len(plan.Issues) == 0
	plan.PlanDigest = saveMigrationPlanDigest(plan)
	return plan
}

func ApplySaveMigration(ctx context.Context, db *bbolt.DB, options SaveMigrationOptions) (SaveMigrationResult, error) {
	saveMigrationMu.Lock()
	defer saveMigrationMu.Unlock()

	if !options.ConfirmMigration || !options.ConfirmServerStopped {
		return SaveMigrationResult{}, ErrSaveEditConfirmation
	}
	if db == nil {
		return SaveMigrationResult{}, errors.New("backup database is required")
	}
	plan := InspectSaveMigration(ctx, options.SourcePath, options.SourcePlatform, options.SourceKind)
	if plan.CoopHostDetected || hasMigrationIssue(plan, "coop_source_unsupported") {
		return SaveMigrationResult{Plan: plan}, ErrSaveMigrationCoopHost
	}
	if !plan.CanMigrate {
		return SaveMigrationResult{Plan: plan}, fmt.Errorf("%w: %s", ErrSaveMigrationInvalid, joinMigrationIssues(plan.Issues))
	}
	if strings.TrimSpace(options.ExpectedPlanDigest) == "" ||
		!strings.EqualFold(options.ExpectedPlanDigest, plan.PlanDigest) {
		return SaveMigrationResult{Plan: plan}, ErrSaveMigrationChanged
	}
	if err := ConfirmGameServerStopped(ctx, options.ConfirmServerStopped); err != nil {
		return SaveMigrationResult{Plan: plan}, err
	}

	destinationBefore, err := inspectMigrationWorld(plan.DestinationPath)
	if err != nil {
		return SaveMigrationResult{Plan: plan}, fmt.Errorf("inspect destination before migration: %w", err)
	}
	safetyBackup, err := migrationBackupAndRecord(db)
	if err != nil {
		return SaveMigrationResult{Plan: plan}, fmt.Errorf("create pre-migration safety backup: %w", err)
	}
	result := SaveMigrationResult{Plan: plan, SafetyBackup: safetyBackup}
	if err := ConfirmGameServerStopped(ctx, options.ConfirmServerStopped); err != nil {
		return result, err
	}
	destinationAfterBackup, err := inspectMigrationWorld(plan.DestinationPath)
	if err != nil || destinationAfterBackup.Digest != destinationBefore.Digest {
		if err == nil {
			err = ErrSaveSourceChanged
		}
		return result, fmt.Errorf("%w: destination changed while creating the safety backup: %v", ErrSaveMigrationChanged, err)
	}

	transactionDir := filepath.Join(filepath.Dir(plan.DestinationPath), ".pst-save-migration-"+uuid.NewString())
	newDir := filepath.Join(transactionDir, "new")
	oldDir := filepath.Join(transactionDir, "old")
	if err := os.MkdirAll(newDir, 0o700); err != nil {
		return result, err
	}
	cleanupTransaction := true
	defer func() {
		if cleanupTransaction {
			_ = os.RemoveAll(transactionDir)
		}
	}()
	if err := migrationStageWorld(plan.SourcePath, newDir); err != nil {
		return result, fmt.Errorf("stage migration source: %w", err)
	}
	staged, err := inspectMigrationWorld(newDir)
	if err != nil {
		return result, fmt.Errorf("inspect staged migration source: %w", err)
	}
	if staged.Digest != plan.SourceDigest {
		return result, fmt.Errorf("%w: staged source digest changed from %s to %s", ErrSaveMigrationChanged, plan.SourceDigest, staged.Digest)
	}
	if err := migrationSnapshotValidator(ctx, newDir); err != nil {
		return result, fmt.Errorf("validate staged migration source: %w", err)
	}
	sourceAfterStage, err := inspectMigrationWorld(plan.SourcePath)
	if err != nil || sourceAfterStage.Digest != plan.SourceDigest {
		if err == nil {
			err = ErrSaveSourceChanged
		}
		return result, fmt.Errorf("%w: source changed while it was staged: %v", ErrSaveMigrationChanged, err)
	}
	if err := ConfirmGameServerStopped(ctx, options.ConfirmServerStopped); err != nil {
		return result, err
	}
	destinationBeforeSwap, err := inspectMigrationWorld(plan.DestinationPath)
	if err != nil || destinationBeforeSwap.Digest != destinationBefore.Digest {
		if err == nil {
			err = ErrSaveSourceChanged
		}
		return result, fmt.Errorf("%w: destination changed before the atomic swap: %v", ErrSaveMigrationChanged, err)
	}
	if err := migrationSwapWorld(plan.DestinationPath, newDir, oldDir); err != nil {
		cleanupTransaction = false
		return result, fmt.Errorf(
			"atomically install migration source: %w; transaction files were preserved at %s",
			err,
			transactionDir,
		)
	}
	installed, err := inspectMigrationWorld(plan.DestinationPath)
	if err != nil || installed.Digest != plan.SourceDigest {
		if err == nil {
			err = errors.New("installed digest does not match the source")
		}
		rollbackErr, rollbackComplete := migrationRollbackWorld(
			plan.DestinationPath,
			oldDir,
			fmt.Errorf("post-migration verification failed: %w", err),
		)
		if !rollbackComplete {
			cleanupTransaction = false
			rollbackErr = fmt.Errorf("%w; recovery files were preserved at %s", rollbackErr, transactionDir)
		}
		return result, rollbackErr
	}
	if err := migrationSnapshotValidator(ctx, plan.DestinationPath); err != nil {
		rollbackErr, rollbackComplete := migrationRollbackWorld(
			plan.DestinationPath,
			oldDir,
			fmt.Errorf("post-migration save validation failed: %w", err),
		)
		if !rollbackComplete {
			cleanupTransaction = false
			rollbackErr = fmt.Errorf("%w; recovery files were preserved at %s", rollbackErr, transactionDir)
		}
		return result, rollbackErr
	}
	return result, nil
}

func resolveMigrationDestination() (string, error) {
	configured := strings.TrimSpace(viper.GetString("save.path"))
	if configured == "" || configured == "/path/to/your/Pal/Saved" || isRemoteSaveSource(configured) {
		return "", ErrSaveMigrationNotConfigured
	}
	worldDir, err := resolveMigrationWorld(configured)
	if err != nil {
		return "", fmt.Errorf("resolve destination save.path: %w", err)
	}
	return worldDir, nil
}

func resolveMigrationWorld(input string) (string, error) {
	input = strings.TrimSpace(input)
	if input == "" {
		return "", errors.New("an absolute source path is required")
	}
	if isRemoteSaveSource(input) {
		return "", errors.New("remote save sources are not supported for migration")
	}
	if !filepath.IsAbs(input) {
		return "", errors.New("save migration paths must be absolute")
	}
	absolute, err := filepath.Abs(input)
	if err != nil {
		return "", err
	}
	absolute = filepath.Clean(absolute)
	info, err := os.Lstat(absolute)
	if err != nil {
		return "", err
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return "", errors.New("save migration paths cannot be symbolic links")
	}
	if err := rejectSymlinkComponents(absolute); err != nil {
		return "", fmt.Errorf("save migration path: %w", err)
	}
	if info.Mode().IsRegular() {
		if !strings.EqualFold(filepath.Base(absolute), "Level.sav") {
			return "", errors.New("migration source file must be Level.sav")
		}
		return filepath.Dir(absolute), nil
	}
	if !info.IsDir() {
		return "", errors.New("migration source must be a directory or Level.sav")
	}
	if isRegularMigrationLevel(filepath.Join(absolute, "Level.sav")) {
		return absolute, nil
	}

	candidateRoots := []string{
		filepath.Join(absolute, "Pal", "Saved", "SaveGames", "0"),
		filepath.Join(absolute, "Saved", "SaveGames", "0"),
		filepath.Join(absolute, "SaveGames", "0"),
		filepath.Join(absolute, "0"),
	}
	if strings.EqualFold(filepath.Base(absolute), "SaveGames") {
		candidateRoots = append(candidateRoots, filepath.Join(absolute, "0"))
	}
	if filepath.Base(absolute) == "0" {
		candidateRoots = append(candidateRoots, absolute)
	}
	candidates := make(map[string]struct{})
	for _, root := range candidateRoots {
		entries, readErr := os.ReadDir(root)
		if os.IsNotExist(readErr) {
			continue
		}
		if readErr != nil {
			return "", readErr
		}
		for _, entry := range entries {
			if entry.Type()&os.ModeSymlink != 0 || !entry.IsDir() {
				continue
			}
			candidate := filepath.Join(root, entry.Name())
			if isRegularMigrationLevel(filepath.Join(candidate, "Level.sav")) {
				clean, _ := filepath.Abs(candidate)
				candidates[filepath.Clean(clean)] = struct{}{}
			}
		}
	}
	if len(candidates) == 0 {
		return "", errors.New("no Palworld world directory containing Level.sav was found")
	}
	if len(candidates) > 1 {
		paths := make([]string, 0, len(candidates))
		for candidate := range candidates {
			paths = append(paths, candidate)
		}
		sort.Strings(paths)
		return "", fmt.Errorf("multiple Palworld worlds were found; select one world directory directly: %s", strings.Join(paths, ", "))
	}
	for candidate := range candidates {
		if err := rejectSymlinkComponents(candidate); err != nil {
			return "", fmt.Errorf("resolved world directory: %w", err)
		}
		return candidate, nil
	}
	return "", errors.New("could not resolve a Palworld world directory")
}

func isRegularMigrationLevel(path string) bool {
	info, err := os.Lstat(path)
	return err == nil && info.Mode().IsRegular() && info.Mode()&os.ModeSymlink == 0 && info.Size() > 0
}

func inspectMigrationWorld(worldDir string) (migrationWorldSnapshot, error) {
	snapshot := migrationWorldSnapshot{WorldDir: worldDir, WorldID: filepath.Base(worldDir)}
	rootInfo, err := os.Lstat(worldDir)
	if err != nil {
		return snapshot, err
	}
	if rootInfo.Mode()&os.ModeSymlink != 0 || !rootInfo.IsDir() {
		return snapshot, errors.New("world root must be a real directory")
	}
	if err := rejectSymlinkComponents(worldDir); err != nil {
		return snapshot, fmt.Errorf("world root: %w", err)
	}
	entries, err := os.ReadDir(worldDir)
	if err != nil {
		return snapshot, err
	}
	required := map[string]bool{"Level.sav": false, "LevelMeta.sav": false, "Players": false}
	for _, entry := range entries {
		name := entry.Name()
		if entry.Type()&os.ModeSymlink != 0 {
			return snapshot, fmt.Errorf("symbolic links are not supported: %s", name)
		}
		switch name {
		case "Level.sav", "LevelMeta.sav", "Players":
			required[name] = true
		case "WorldOption.sav":
			snapshot.HasWorldOption = true
		case "backup":
			if !entry.IsDir() {
				return snapshot, errors.New("backup must be a directory when present")
			}
			snapshot.HasNativeBackups = true
		default:
			snapshot.IgnoredEntries = append(snapshot.IgnoredEntries, name)
		}
	}
	for name, present := range required {
		if !present {
			return snapshot, fmt.Errorf("world is missing required %s", name)
		}
	}
	sort.Strings(snapshot.IgnoredEntries)

	hasher := sha256.New()
	hasher.Write([]byte("pst-save-migration-v1\n"))
	files := []string{"Level.sav", "LevelMeta.sav"}
	if snapshot.HasWorldOption {
		files = append(files, "WorldOption.sav")
	}
	playersDir := filepath.Join(worldDir, "Players")
	playersInfo, err := os.Lstat(playersDir)
	if err != nil {
		return snapshot, err
	}
	if playersInfo.Mode()&os.ModeSymlink != 0 || !playersInfo.IsDir() {
		return snapshot, errors.New("Players must be a real directory")
	}
	if err := rejectSymlinkComponents(playersDir); err != nil {
		return snapshot, fmt.Errorf("Players: %w", err)
	}
	fmt.Fprint(hasher, "Players\x00directory\n")
	playerEntries, err := os.ReadDir(playersDir)
	if err != nil {
		return snapshot, err
	}
	for _, entry := range playerEntries {
		if entry.Type()&os.ModeSymlink != 0 || entry.IsDir() {
			return snapshot, fmt.Errorf("Players/%s must be a regular save file", entry.Name())
		}
		if !migrationPlayerSavePattern.MatchString(entry.Name()) {
			return snapshot, fmt.Errorf("Players contains unsupported entry %q", entry.Name())
		}
		if strings.EqualFold(entry.Name(), coopHostPlayerSaveName) {
			snapshot.CoopHostDetected = true
		}
		files = append(files, filepath.Join("Players", entry.Name()))
		snapshot.PlayerFiles++
	}
	sort.Slice(files, func(i, j int) bool { return filepath.ToSlash(files[i]) < filepath.ToSlash(files[j]) })
	for _, relative := range files {
		path := filepath.Join(worldDir, relative)
		info, err := os.Lstat(path)
		if err != nil {
			return snapshot, err
		}
		if info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() || info.Size() <= 0 {
			return snapshot, fmt.Errorf("%s must be a non-empty regular file", filepath.ToSlash(relative))
		}
		if err := rejectSymlinkComponents(path); err != nil {
			return snapshot, fmt.Errorf("%s: %w", filepath.ToSlash(relative), err)
		}
		fingerprint, err := fingerprintSaveFile(path)
		if err != nil {
			return snapshot, err
		}
		fmt.Fprintf(hasher, "%s\x00%d\n", filepath.ToSlash(relative), fingerprint.Size)
		hasher.Write(fingerprint.SHA256[:])
		snapshot.FileCount++
		snapshot.SizeBytes += fingerprint.Size
	}
	snapshot.Digest = hex.EncodeToString(hasher.Sum(nil))
	return snapshot, nil
}

func validateMigrationSnapshotWithCLI(ctx context.Context, worldDir string) error {
	savCLI, err := getSavCli()
	if err != nil {
		return fmt.Errorf("load the bundled sav_cli validator: %w", err)
	}
	validatorInfo, err := os.Lstat(savCLI)
	if err != nil {
		return err
	}
	if validatorInfo.Mode()&os.ModeSymlink != 0 || !validatorInfo.Mode().IsRegular() {
		return errors.New("sav_cli validator must be a regular file and not a symbolic link")
	}
	if runtime.GOOS != SaveMigrationWindows && validatorInfo.Mode().Perm()&0o111 == 0 {
		return errors.New("sav_cli validator must have an executable permission bit")
	}
	validations := []struct {
		path          string
		expectedClass string
	}{
		{filepath.Join(worldDir, "Level.sav"), "/Script/Pal.PalWorldSaveGame"},
		{filepath.Join(worldDir, "LevelMeta.sav"), "/Script/Pal.PalWorldBaseInfoSaveGame"},
	}
	playerEntries, err := os.ReadDir(filepath.Join(worldDir, "Players"))
	if err != nil {
		return err
	}
	for _, entry := range playerEntries {
		validations = append(validations, struct {
			path          string
			expectedClass string
		}{filepath.Join(worldDir, "Players", entry.Name()), "/Script/Pal.PalWorldPlayerSaveGame"})
	}
	worldOption := filepath.Join(worldDir, "WorldOption.sav")
	if _, err := os.Lstat(worldOption); err == nil {
		validations = append(validations, struct {
			path          string
			expectedClass string
		}{worldOption, "/Script/Pal.PalWorldOptionSaveGame"})
	} else if !os.IsNotExist(err) {
		return err
	}
	for _, validation := range validations {
		if err := validateMigrationSaveFile(ctx, savCLI, validation.path, validation.expectedClass); err != nil {
			return err
		}
	}
	return nil
}

func validateMigrationSaveFile(ctx context.Context, savCLI, path, expectedClass string) error {
	validationCtx, cancel := context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()
	command := exec.CommandContext(validationCtx, savCLI, "--mode", "validate", "--file", path)
	command.Dir = filepath.Dir(savCLI)
	output := &steamTailBuffer{limit: maxMigrationValidatorOutput}
	command.Stdout = output
	command.Stderr = output
	if err := command.Run(); err != nil {
		if errors.Is(validationCtx.Err(), context.DeadlineExceeded) {
			return fmt.Errorf("validate %s: timed out", filepath.Base(path))
		}
		return fmt.Errorf("validate %s: %v: %s", filepath.Base(path), err, output.String())
	}
	if !strings.Contains(output.String(), "class="+expectedClass) {
		return fmt.Errorf("validate %s: expected save class %s", filepath.Base(path), expectedClass)
	}
	return nil
}

func rollbackSaveMigration(worldDir, oldDir string, operationErr error) (error, bool) {
	var rollbackErrors []string
	for _, name := range nativeRestoreEntries {
		if err := os.RemoveAll(filepath.Join(worldDir, name)); err != nil {
			rollbackErrors = append(rollbackErrors, err.Error())
		}
	}
	for _, name := range nativeRestoreEntries {
		oldPath := filepath.Join(oldDir, name)
		if _, err := os.Lstat(oldPath); os.IsNotExist(err) {
			continue
		} else if err != nil {
			rollbackErrors = append(rollbackErrors, err.Error())
			continue
		}
		if err := os.Rename(oldPath, filepath.Join(worldDir, name)); err != nil {
			rollbackErrors = append(rollbackErrors, err.Error())
		}
	}
	if len(rollbackErrors) > 0 {
		return fmt.Errorf("migration failed: %v; rollback failed: %s", operationErr, strings.Join(rollbackErrors, "; ")), false
	}
	return operationErr, true
}

func saveMigrationPlanDigest(plan SaveMigrationPlan) string {
	payload := struct {
		GameVersion          string                `json:"game_version"`
		DestinationPlatform  string                `json:"destination_platform"`
		SourcePath           string                `json:"source_path"`
		SourcePlatform       string                `json:"source_platform"`
		SourceKind           string                `json:"source_kind"`
		SourceDigest         string                `json:"source_digest"`
		SourceWorldID        string                `json:"source_world_id"`
		DestinationPath      string                `json:"destination_path"`
		DestinationWorldID   string                `json:"destination_world_id"`
		SourceHasWorldOption bool                  `json:"source_has_world_option"`
		CoopHostDetected     bool                  `json:"coop_host_detected"`
		ValidationPassed     bool                  `json:"validation_passed"`
		IgnoredEntries       []string              `json:"ignored_entries"`
		Issues               []SaveMigrationNotice `json:"issues"`
	}{
		GameVersion:          plan.GameVersion,
		DestinationPlatform:  plan.DestinationPlatform,
		SourcePath:           plan.SourcePath,
		SourcePlatform:       plan.SourcePlatform,
		SourceKind:           plan.SourceKind,
		SourceDigest:         plan.SourceDigest,
		SourceWorldID:        plan.SourceWorldID,
		DestinationPath:      plan.DestinationPath,
		DestinationWorldID:   plan.DestinationWorldID,
		SourceHasWorldOption: plan.SourceHasWorldOption,
		CoopHostDetected:     plan.CoopHostDetected,
		ValidationPassed:     plan.ValidationPassed,
		IgnoredEntries:       plan.SourceIgnoredEntries,
		Issues:               plan.Issues,
	}
	encoded, _ := json.Marshal(payload)
	digest := sha256.Sum256(encoded)
	return hex.EncodeToString(digest[:])
}

func (plan *SaveMigrationPlan) addIssue(code, message string) {
	plan.Issues = append(plan.Issues, SaveMigrationNotice{Code: code, Message: message})
}

func (plan *SaveMigrationPlan) addWarning(code, message string) {
	plan.Warnings = append(plan.Warnings, SaveMigrationNotice{Code: code, Message: message})
}

func uniqueMigrationNotices(notices []SaveMigrationNotice) []SaveMigrationNotice {
	seen := make(map[string]struct{}, len(notices))
	result := make([]SaveMigrationNotice, 0, len(notices))
	for _, notice := range notices {
		key := notice.Code + "\x00" + notice.Message
		if _, exists := seen[key]; exists {
			continue
		}
		seen[key] = struct{}{}
		result = append(result, notice)
	}
	return result
}

func hasMigrationIssue(plan SaveMigrationPlan, code string) bool {
	for _, issue := range plan.Issues {
		if issue.Code == code {
			return true
		}
	}
	return false
}

func joinMigrationIssues(issues []SaveMigrationNotice) string {
	messages := make([]string, 0, len(issues))
	for _, issue := range issues {
		messages = append(messages, issue.Message)
	}
	return strings.Join(messages, "; ")
}
