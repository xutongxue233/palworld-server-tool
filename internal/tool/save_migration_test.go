package tool

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/spf13/viper"
	"github.com/zaigie/palworld-server-tool/internal/database"
	"go.etcd.io/bbolt"
)

func TestInspectSaveMigrationAcceptsDedicatedSamePlatformWorld(t *testing.T) {
	_, destination, source, _ := setupSaveMigrationTest(t)
	writeTestFile(t, filepath.Join(source, "backup", "world", "2026.07.13-10.00.00", "Level.sav"), []byte("ignored-backup"))
	writeTestFile(t, filepath.Join(source, "source-note.txt"), []byte("ignored-note"))

	plan := InspectSaveMigration(context.Background(), source, runtime.GOOS, SaveMigrationDedicated)
	if !plan.Configured || !plan.CanMigrate || !plan.ValidationPassed {
		t.Fatalf("expected a migratable plan, got %#v", plan)
	}
	if plan.GameVersion != SaveMigrationGameVersion || plan.SourcePath != source || plan.DestinationPath != destination {
		t.Fatalf("unexpected migration identity: %#v", plan)
	}
	if plan.SourcePlayerFiles != 2 || plan.SourceFileCount != 4 || plan.SourceDigest == "" || len(plan.PlanDigest) != 64 {
		t.Fatalf("unexpected source summary: %#v", plan)
	}
	if !plan.SourceHasNativeBackups || !containsMigrationNotice(plan.Warnings, "source_native_backups_ignored") {
		t.Fatalf("source native backups were not reported: %#v", plan)
	}
	if !containsMigrationNotice(plan.Warnings, "source_entries_ignored") || !containsString(plan.SourceIgnoredEntries, "source-note.txt") {
		t.Fatalf("ignored source entries were not reported: %#v", plan)
	}
	if !containsMigrationNotice(plan.Warnings, "world_option_removed") {
		t.Fatalf("WorldOption removal was not disclosed: %#v", plan)
	}
}

func TestInspectSaveMigrationResolvesCurrentPlatformSelection(t *testing.T) {
	_, _, source, _ := setupSaveMigrationTest(t)
	plan := InspectSaveMigration(context.Background(), source, SaveMigrationCurrent, SaveMigrationDedicated)
	if !plan.CanMigrate || plan.SourcePlatform != runtime.GOOS {
		t.Fatalf("current platform selection did not resolve to %s: %#v", runtime.GOOS, plan)
	}
}

func TestResolveMigrationWorldSupportedInputsAndAmbiguity(t *testing.T) {
	workspace := t.TempDir()
	root := filepath.Join(workspace, "source-root")
	world := filepath.Join(root, "Pal", "Saved", "SaveGames", "0", "WORLD-A")
	writeMigrationWorld(t, world, "source", false, 1)

	for name, input := range map[string]string{
		"direct world": world,
		"level file":   filepath.Join(world, "Level.sav"),
		"install root": root,
		"saved root":   filepath.Join(root, "Pal", "Saved"),
		"zero root":    filepath.Join(root, "Pal", "Saved", "SaveGames", "0"),
	} {
		t.Run(name, func(t *testing.T) {
			resolved, err := resolveMigrationWorld(input)
			if err != nil {
				t.Fatal(err)
			}
			if !sameFilesystemPath(resolved, world) {
				t.Fatalf("resolved %q, want %q", resolved, world)
			}
		})
	}

	writeMigrationWorld(t, filepath.Join(root, "Pal", "Saved", "SaveGames", "0", "WORLD-B"), "second", false, 1)
	if _, err := resolveMigrationWorld(root); err == nil || !strings.Contains(err.Error(), "multiple Palworld worlds") {
		t.Fatalf("expected an ambiguous-world error, got %v", err)
	}
	if _, err := resolveMigrationWorld("relative-world"); err == nil || !strings.Contains(err.Error(), "absolute") {
		t.Fatalf("expected an absolute-path error, got %v", err)
	}
	if _, err := resolveMigrationWorld("https://example.invalid/Level.sav"); err == nil || !strings.Contains(err.Error(), "remote") {
		t.Fatalf("expected a remote-source error, got %v", err)
	}
}

func TestInspectSaveMigrationBlocksUnsafeIdentityConversions(t *testing.T) {
	_, destination, source, _ := setupSaveMigrationTest(t)

	t.Run("co-op kind", func(t *testing.T) {
		plan := InspectSaveMigration(context.Background(), source, runtime.GOOS, SaveMigrationCoop)
		if plan.CanMigrate || !containsMigrationNotice(plan.Issues, "coop_source_unsupported") {
			t.Fatalf("co-op source was not blocked: %#v", plan)
		}
	})

	t.Run("co-op host GUID", func(t *testing.T) {
		writeTestFile(t, filepath.Join(source, "Players", coopHostPlayerSaveName), []byte("host-player"))
		plan := InspectSaveMigration(context.Background(), source, runtime.GOOS, SaveMigrationDedicated)
		if plan.CanMigrate || !plan.CoopHostDetected || !containsMigrationNotice(plan.Issues, "coop_host_detected") {
			t.Fatalf("co-op host save was not blocked: %#v", plan)
		}
	})

	t.Run("cross platform", func(t *testing.T) {
		otherPlatform := SaveMigrationLinux
		if runtime.GOOS == SaveMigrationLinux {
			otherPlatform = SaveMigrationWindows
		}
		plan := InspectSaveMigration(context.Background(), source, otherPlatform, SaveMigrationDedicated)
		if plan.CanMigrate || !containsMigrationNotice(plan.Issues, "cross_platform_identity_unsupported") {
			t.Fatalf("cross-platform source was not blocked: %#v", plan)
		}
	})

	t.Run("same source and destination", func(t *testing.T) {
		plan := InspectSaveMigration(context.Background(), destination, runtime.GOOS, SaveMigrationDedicated)
		if plan.CanMigrate || !containsMigrationNotice(plan.Issues, "source_is_destination") {
			t.Fatalf("same source and destination were not blocked: %#v", plan)
		}
	})
}

func TestInspectSaveMigrationRejectsUnsupportedPlayerEntriesAndSymlinks(t *testing.T) {
	_, _, source, _ := setupSaveMigrationTest(t)
	writeTestFile(t, filepath.Join(source, "Players", "notes.txt"), []byte("unsupported"))
	plan := InspectSaveMigration(context.Background(), source, runtime.GOOS, SaveMigrationDedicated)
	if plan.CanMigrate || !containsMigrationNotice(plan.Issues, "source_invalid") {
		t.Fatalf("unsupported Players entry was accepted: %#v", plan)
	}

	if err := os.Remove(filepath.Join(source, "Players", "notes.txt")); err != nil {
		t.Fatal(err)
	}
	outside := filepath.Join(t.TempDir(), "outside.sav")
	writeTestFile(t, outside, []byte("outside"))
	linked := filepath.Join(source, "Players", strings.Repeat("A", 32)+".sav")
	if err := os.Symlink(outside, linked); err != nil {
		t.Skipf("symbolic links are not available: %v", err)
	}
	plan = InspectSaveMigration(context.Background(), source, runtime.GOOS, SaveMigrationDedicated)
	if plan.CanMigrate || !containsMigrationNotice(plan.Issues, "source_invalid") {
		t.Fatalf("symbolic-link player save was accepted: %#v", plan)
	}
}

func TestApplySaveMigrationBacksUpAndAtomicallyReplacesWorld(t *testing.T) {
	workspace, destination, source, db := setupSaveMigrationTest(t)
	writeTestFile(t, filepath.Join(destination, "backup", "world", "keep", "marker.txt"), []byte("native-backup"))
	writeTestFile(t, filepath.Join(destination, "server-note.txt"), []byte("keep-me"))

	plan := InspectSaveMigration(context.Background(), source, runtime.GOOS, SaveMigrationDedicated)
	result, err := ApplySaveMigration(context.Background(), db, SaveMigrationOptions{
		SourcePath:           source,
		SourcePlatform:       runtime.GOOS,
		SourceKind:           SaveMigrationDedicated,
		ExpectedPlanDigest:   plan.PlanDigest,
		ConfirmMigration:     true,
		ConfirmServerStopped: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.SafetyBackup.Path == "" || result.Plan.SourceDigest != plan.SourceDigest {
		t.Fatalf("unexpected migration result: %#v", result)
	}
	assertFileContent(t, filepath.Join(destination, "Level.sav"), []byte("source-level"))
	assertFileContent(t, filepath.Join(destination, "LevelMeta.sav"), []byte("source-meta"))
	assertFileContent(t, filepath.Join(destination, "Players", migrationPlayerName(2)), []byte("source-player-2"))
	assertFileContent(t, filepath.Join(destination, "Players", migrationPlayerName(3)), []byte("source-player-3"))
	if _, err := os.Stat(filepath.Join(destination, "Players", migrationPlayerName(9))); !os.IsNotExist(err) {
		t.Fatalf("old destination player was not removed: %v", err)
	}
	if _, err := os.Stat(filepath.Join(destination, "WorldOption.sav")); !os.IsNotExist(err) {
		t.Fatalf("destination WorldOption.sav should be removed when absent from source: %v", err)
	}
	assertFileContent(t, filepath.Join(destination, "backup", "world", "keep", "marker.txt"), []byte("native-backup"))
	assertFileContent(t, filepath.Join(destination, "server-note.txt"), []byte("keep-me"))
	assertFileContent(t, filepath.Join(source, "Level.sav"), []byte("source-level"))
	assertZipFileContent(t, filepath.Join(workspace, "backups", result.SafetyBackup.Path), "Level.sav", []byte("destination-level"))
}

func TestApplySaveMigrationRejectsChangedPlanBeforeBackup(t *testing.T) {
	workspace, destination, source, db := setupSaveMigrationTest(t)
	plan := InspectSaveMigration(context.Background(), source, runtime.GOOS, SaveMigrationDedicated)
	writeTestFile(t, filepath.Join(source, "Level.sav"), []byte("source-level-changed"))

	_, err := ApplySaveMigration(context.Background(), db, SaveMigrationOptions{
		SourcePath:           source,
		SourcePlatform:       runtime.GOOS,
		SourceKind:           SaveMigrationDedicated,
		ExpectedPlanDigest:   plan.PlanDigest,
		ConfirmMigration:     true,
		ConfirmServerStopped: true,
	})
	if !errors.Is(err, ErrSaveMigrationChanged) {
		t.Fatalf("expected ErrSaveMigrationChanged, got %v", err)
	}
	assertFileContent(t, filepath.Join(destination, "Level.sav"), []byte("destination-level"))
	if entries, readErr := os.ReadDir(filepath.Join(workspace, "backups")); readErr == nil && len(entries) != 0 {
		t.Fatalf("stale plan created a safety backup: %#v", entries)
	}
}

func TestApplySaveMigrationRejectsDestinationChangeDuringBackup(t *testing.T) {
	_, destination, source, db := setupSaveMigrationTest(t)
	plan := InspectSaveMigration(context.Background(), source, runtime.GOOS, SaveMigrationDedicated)
	originalBackup := migrationBackupAndRecord
	migrationBackupAndRecord = func(db *bbolt.DB) (database.Backup, error) {
		backup, err := originalBackup(db)
		if err == nil {
			writeTestFile(t, filepath.Join(destination, "Level.sav"), []byte("externally-changed"))
		}
		return backup, err
	}
	t.Cleanup(func() { migrationBackupAndRecord = originalBackup })

	_, err := ApplySaveMigration(context.Background(), db, SaveMigrationOptions{
		SourcePath:           source,
		SourcePlatform:       runtime.GOOS,
		SourceKind:           SaveMigrationDedicated,
		ExpectedPlanDigest:   plan.PlanDigest,
		ConfirmMigration:     true,
		ConfirmServerStopped: true,
	})
	if !errors.Is(err, ErrSaveMigrationChanged) {
		t.Fatalf("expected destination change rejection, got %v", err)
	}
	assertFileContent(t, filepath.Join(destination, "Level.sav"), []byte("externally-changed"))
	if _, err := os.Stat(filepath.Join(destination, "Players", migrationPlayerName(2))); !os.IsNotExist(err) {
		t.Fatalf("migration source was installed after destination changed: %v", err)
	}
}

func TestApplySaveMigrationRollsBackPostInstallValidationFailure(t *testing.T) {
	_, destination, source, db := setupSaveMigrationTest(t)
	plan := InspectSaveMigration(context.Background(), source, runtime.GOOS, SaveMigrationDedicated)
	validationCalls := 0
	originalValidator := migrationSnapshotValidator
	migrationSnapshotValidator = func(context.Context, string) error {
		validationCalls++
		if validationCalls == 3 {
			return errors.New("forced post-install validation failure")
		}
		return nil
	}
	t.Cleanup(func() { migrationSnapshotValidator = originalValidator })

	_, err := ApplySaveMigration(context.Background(), db, SaveMigrationOptions{
		SourcePath:           source,
		SourcePlatform:       runtime.GOOS,
		SourceKind:           SaveMigrationDedicated,
		ExpectedPlanDigest:   plan.PlanDigest,
		ConfirmMigration:     true,
		ConfirmServerStopped: true,
	})
	if err == nil || !strings.Contains(err.Error(), "forced post-install validation failure") {
		t.Fatalf("expected forced validation failure, got %v", err)
	}
	assertFileContent(t, filepath.Join(destination, "Level.sav"), []byte("destination-level"))
	assertFileContent(t, filepath.Join(destination, "LevelMeta.sav"), []byte("destination-meta"))
	assertFileContent(t, filepath.Join(destination, "Players", migrationPlayerName(9)), []byte("destination-player-9"))
	assertFileContent(t, filepath.Join(destination, "WorldOption.sav"), []byte("destination-world-option"))
}

func TestApplySaveMigrationPreservesRecoveryFilesWhenAtomicSwapFails(t *testing.T) {
	_, destination, source, db := setupSaveMigrationTest(t)
	plan := InspectSaveMigration(context.Background(), source, runtime.GOOS, SaveMigrationDedicated)
	migrationSwapWorld = func(worldDir, _ string, oldDir string) error {
		if err := os.MkdirAll(oldDir, 0o700); err != nil {
			return err
		}
		if err := os.Rename(filepath.Join(worldDir, "Level.sav"), filepath.Join(oldDir, "Level.sav")); err != nil {
			return err
		}
		return errors.New("forced partial atomic swap failure")
	}

	_, err := ApplySaveMigration(context.Background(), db, SaveMigrationOptions{
		SourcePath:           source,
		SourcePlatform:       runtime.GOOS,
		SourceKind:           SaveMigrationDedicated,
		ExpectedPlanDigest:   plan.PlanDigest,
		ConfirmMigration:     true,
		ConfirmServerStopped: true,
	})
	if err == nil || !strings.Contains(err.Error(), "transaction files were preserved") {
		t.Fatalf("expected preserved transaction error, got %v", err)
	}
	transactions, globErr := filepath.Glob(filepath.Join(filepath.Dir(destination), ".pst-save-migration-*"))
	if globErr != nil || len(transactions) != 1 {
		t.Fatalf("expected one preserved transaction directory, got %v: %v", transactions, globErr)
	}
	assertFileContent(t, filepath.Join(transactions[0], "old", "Level.sav"), []byte("destination-level"))
}

func TestApplySaveMigrationPreservesRecoveryFilesWhenRollbackFails(t *testing.T) {
	_, destination, source, db := setupSaveMigrationTest(t)
	plan := InspectSaveMigration(context.Background(), source, runtime.GOOS, SaveMigrationDedicated)
	validationCalls := 0
	migrationSnapshotValidator = func(context.Context, string) error {
		validationCalls++
		if validationCalls == 3 {
			return errors.New("forced post-install failure")
		}
		return nil
	}
	migrationRollbackWorld = func(_ string, _ string, operationErr error) (error, bool) {
		return fmt.Errorf("%w; forced rollback failure", operationErr), false
	}

	_, err := ApplySaveMigration(context.Background(), db, SaveMigrationOptions{
		SourcePath:           source,
		SourcePlatform:       runtime.GOOS,
		SourceKind:           SaveMigrationDedicated,
		ExpectedPlanDigest:   plan.PlanDigest,
		ConfirmMigration:     true,
		ConfirmServerStopped: true,
	})
	if err == nil || !strings.Contains(err.Error(), "recovery files were preserved") {
		t.Fatalf("expected preserved recovery error, got %v", err)
	}
	transactions, globErr := filepath.Glob(filepath.Join(filepath.Dir(destination), ".pst-save-migration-*"))
	if globErr != nil || len(transactions) != 1 {
		t.Fatalf("expected one preserved transaction directory, got %v: %v", transactions, globErr)
	}
	assertFileContent(t, filepath.Join(transactions[0], "old", "Level.sav"), []byte("destination-level"))
}

func TestApplySaveMigrationRequiresBothConfirmations(t *testing.T) {
	_, destination, source, db := setupSaveMigrationTest(t)
	plan := InspectSaveMigration(context.Background(), source, runtime.GOOS, SaveMigrationDedicated)
	for name, options := range map[string]SaveMigrationOptions{
		"migration": {
			SourcePath: source, SourcePlatform: runtime.GOOS, SourceKind: SaveMigrationDedicated,
			ExpectedPlanDigest: plan.PlanDigest, ConfirmServerStopped: true,
		},
		"server stopped": {
			SourcePath: source, SourcePlatform: runtime.GOOS, SourceKind: SaveMigrationDedicated,
			ExpectedPlanDigest: plan.PlanDigest, ConfirmMigration: true,
		},
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := ApplySaveMigration(context.Background(), db, options); !errors.Is(err, ErrSaveEditConfirmation) {
				t.Fatalf("expected confirmation error, got %v", err)
			}
		})
	}
	assertFileContent(t, filepath.Join(destination, "Level.sav"), []byte("destination-level"))
}

func TestInspectRealSaveMigrationFixture(t *testing.T) {
	source := strings.TrimSpace(os.Getenv("PST_REAL_MIGRATION_SAVE"))
	if source == "" {
		t.Skip("set PST_REAL_MIGRATION_SAVE to run the read-only real-save migration preflight")
	}
	savCLI := strings.TrimSpace(os.Getenv("PST_SAV_CLI"))
	if savCLI == "" {
		var err error
		savCLI, err = filepath.Abs(filepath.Join("..", "..", "dist", "sav_cli"))
		if err != nil {
			t.Fatal(err)
		}
		if runtime.GOOS == SaveMigrationWindows {
			savCLI += ".exe"
		}
	}
	destination := filepath.Join(t.TempDir(), "Saved", "SaveGames", "0", "DESTINATION")
	writeMigrationWorld(t, destination, "destination", false, 1)

	viper.Reset()
	t.Cleanup(viper.Reset)
	viper.Set("save.path", destination)
	viper.Set("save.decode_path", savCLI)
	viper.Set("rest.address", "")
	viper.Set("palworld.control.mode", "disabled")
	originalValidator := migrationSnapshotValidator
	migrationSnapshotValidator = validateMigrationSnapshotWithCLI
	t.Cleanup(func() { migrationSnapshotValidator = originalValidator })

	plan := InspectSaveMigration(context.Background(), source, runtime.GOOS, SaveMigrationDedicated)
	if !plan.CanMigrate || !plan.ValidationPassed || plan.SourcePlayerFiles != 4 {
		t.Fatalf("real save did not pass read-only migration preflight: %#v", plan)
	}
}

func setupSaveMigrationTest(t *testing.T) (string, string, string, *bbolt.DB) {
	t.Helper()
	workspace := t.TempDir()
	destination := filepath.Join(workspace, "destination", "Pal", "Saved", "SaveGames", "0", "DESTINATION")
	source := filepath.Join(workspace, "source", "Pal", "Saved", "SaveGames", "0", "SOURCE")
	writeMigrationWorld(t, destination, "destination", true, 9)
	writeMigrationWorld(t, source, "source", false, 2, 3)

	db, err := bbolt.Open(filepath.Join(workspace, "test.db"), 0o600, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if err := db.Update(func(tx *bbolt.Tx) error {
		_, err := tx.CreateBucketIfNotExists([]byte("backups"))
		return err
	}); err != nil {
		t.Fatal(err)
	}

	viper.Reset()
	t.Cleanup(viper.Reset)
	viper.Set("save.path", destination)
	viper.Set("rest.address", "")
	viper.Set("palworld.control.mode", "disabled")
	originalOnlineProbe := serverOnlineProbe
	serverOnlineProbe = func() bool { return false }
	t.Cleanup(func() { serverOnlineProbe = originalOnlineProbe })
	originalValidator := migrationSnapshotValidator
	migrationSnapshotValidator = func(context.Context, string) error { return nil }
	t.Cleanup(func() { migrationSnapshotValidator = originalValidator })
	originalBackup := migrationBackupAndRecord
	originalStage := migrationStageWorld
	originalSwap := migrationSwapWorld
	originalRollback := migrationRollbackWorld
	t.Cleanup(func() {
		migrationBackupAndRecord = originalBackup
		migrationStageWorld = originalStage
		migrationSwapWorld = originalSwap
		migrationRollbackWorld = originalRollback
	})
	t.Chdir(workspace)
	return workspace, destination, source, db
}

func writeMigrationWorld(t *testing.T, worldDir, prefix string, worldOption bool, playerIDs ...int) {
	t.Helper()
	writeTestFile(t, filepath.Join(worldDir, "Level.sav"), []byte(prefix+"-level"))
	writeTestFile(t, filepath.Join(worldDir, "LevelMeta.sav"), []byte(prefix+"-meta"))
	if worldOption {
		writeTestFile(t, filepath.Join(worldDir, "WorldOption.sav"), []byte(prefix+"-world-option"))
	}
	if len(playerIDs) == 0 {
		if err := os.MkdirAll(filepath.Join(worldDir, "Players"), 0o700); err != nil {
			t.Fatal(err)
		}
	}
	for _, playerID := range playerIDs {
		writeTestFile(
			t,
			filepath.Join(worldDir, "Players", migrationPlayerName(playerID)),
			[]byte(fmt.Sprintf("%s-player-%d", prefix, playerID)),
		)
	}
}

func migrationPlayerName(id int) string {
	return fmt.Sprintf("%032X.sav", id)
}

func containsMigrationNotice(notices []SaveMigrationNotice, code string) bool {
	for _, notice := range notices {
		if notice.Code == code {
			return true
		}
	}
	return false
}

func containsString(values []string, expected string) bool {
	for _, value := range values {
		if value == expected {
			return true
		}
	}
	return false
}
