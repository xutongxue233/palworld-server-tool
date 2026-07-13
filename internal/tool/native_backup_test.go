package tool

import (
	"archive/zip"
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/spf13/viper"
	"go.etcd.io/bbolt"
)

func TestListAndRestoreNativeBackup(t *testing.T) {
	workspace, worldDir, db := setupNativeBackupTest(t)
	backupID := "2026.07.13-10.00.00"
	backupDir := filepath.Join(worldDir, "backup", "world", backupID)
	writeNativeBackupFixture(t, backupDir, false)

	catalog, err := ListNativeBackups()
	if err != nil {
		t.Fatal(err)
	}
	if !catalog.Configured || !catalog.Available || catalog.WorldID != filepath.Base(worldDir) {
		t.Fatalf("unexpected native backup catalog: %#v", catalog)
	}
	if len(catalog.Backups) != 1 {
		t.Fatalf("expected one native backup, got %#v", catalog.Backups)
	}
	backup := catalog.Backups[0]
	if !backup.Valid || backup.BackupID != backupID || backup.PlayerFiles != 1 || backup.HasWorldOption || backup.Digest == "" {
		t.Fatalf("unexpected native backup: %#v", backup)
	}

	result, err := RestoreNativeBackup(context.Background(), db, NativeBackupRestoreOptions{
		BackupID:             backupID,
		ExpectedDigest:       backup.Digest,
		ConfirmServerStopped: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.RestoredBackup.BackupID != backupID || result.SafetyBackup.Path == "" {
		t.Fatalf("unexpected restore result: %#v", result)
	}
	assertFileContent(t, filepath.Join(worldDir, "Level.sav"), []byte("backup-level"))
	assertFileContent(t, filepath.Join(worldDir, "LevelMeta.sav"), []byte("backup-meta"))
	assertFileContent(t, filepath.Join(worldDir, "Players", "BACKUP.sav"), []byte("backup-player"))
	if _, err := os.Stat(filepath.Join(worldDir, "Players", "CURRENT.sav")); !os.IsNotExist(err) {
		t.Fatalf("current player save was not replaced: %v", err)
	}
	if _, err := os.Stat(filepath.Join(worldDir, "WorldOption.sav")); !os.IsNotExist(err) {
		t.Fatalf("WorldOption.sav should be removed when absent from the restored backup: %v", err)
	}
	if _, err := os.Stat(filepath.Join(backupDir, "Level.sav")); err != nil {
		t.Fatalf("native backup source was modified: %v", err)
	}

	safetyPath := filepath.Join(workspace, "backups", result.SafetyBackup.Path)
	assertZipFileContent(t, safetyPath, "Level.sav", []byte("current-level"))
}

func TestRestoreNativeBackupRejectsChangedSnapshot(t *testing.T) {
	_, worldDir, db := setupNativeBackupTest(t)
	backupID := "2026.07.13-10.01.00"
	backupDir := filepath.Join(worldDir, "backup", "world", backupID)
	writeNativeBackupFixture(t, backupDir, true)

	catalog, err := ListNativeBackups()
	if err != nil {
		t.Fatal(err)
	}
	if len(catalog.Backups) != 1 || !catalog.Backups[0].Valid || !catalog.Backups[0].HasWorldOption {
		t.Fatalf("unexpected catalog: %#v", catalog)
	}

	_, err = RestoreNativeBackup(context.Background(), db, NativeBackupRestoreOptions{
		BackupID:             backupID,
		ExpectedDigest:       "stale-digest",
		ConfirmServerStopped: true,
	})
	if !errors.Is(err, ErrNativeBackupChanged) {
		t.Fatalf("expected ErrNativeBackupChanged, got %v", err)
	}
	assertFileContent(t, filepath.Join(worldDir, "Level.sav"), []byte("current-level"))
	if entries, readErr := os.ReadDir(filepath.Join(filepath.Dir(worldDir), "backups")); readErr == nil && len(entries) != 0 {
		t.Fatalf("stale restore created a safety backup: %#v", entries)
	}
}

func TestNativeBackupValidationAndConfirmation(t *testing.T) {
	_, worldDir, db := setupNativeBackupTest(t)
	backupID := "2026.07.13-10.02.00"
	backupDir := filepath.Join(worldDir, "backup", "world", backupID)
	writeNativeBackupFixture(t, backupDir, false)
	writeTestFile(t, filepath.Join(backupDir, "unexpected.txt"), []byte("unsupported"))

	catalog, err := ListNativeBackups()
	if err != nil {
		t.Fatal(err)
	}
	if len(catalog.Backups) != 1 || catalog.Backups[0].Valid || len(catalog.Backups[0].Issues) == 0 {
		t.Fatalf("invalid backup should be visible with validation issues: %#v", catalog)
	}

	os.Remove(filepath.Join(backupDir, "unexpected.txt"))
	backup, err := inspectNativeBackup(backupDir, backupID)
	if err != nil {
		t.Fatal(err)
	}
	_, err = RestoreNativeBackup(context.Background(), db, NativeBackupRestoreOptions{
		BackupID:       backupID,
		ExpectedDigest: backup.Digest,
	})
	if !errors.Is(err, ErrSaveEditConfirmation) {
		t.Fatalf("expected confirmation error, got %v", err)
	}
	assertFileContent(t, filepath.Join(worldDir, "Level.sav"), []byte("current-level"))
}

func TestNativeBackupDigestCoversPlayerSaveContent(t *testing.T) {
	_, worldDir, _ := setupNativeBackupTest(t)
	backupID := "2026.07.13-10.03.00"
	backupDir := filepath.Join(worldDir, "backup", "world", backupID)
	writeNativeBackupFixture(t, backupDir, false)

	before, err := inspectNativeBackup(backupDir, backupID)
	if err != nil {
		t.Fatal(err)
	}
	playerPath := filepath.Join(backupDir, "Players", "BACKUP.sav")
	info, err := os.Stat(playerPath)
	if err != nil {
		t.Fatal(err)
	}
	original, err := os.ReadFile(playerPath)
	if err != nil {
		t.Fatal(err)
	}
	mutated := bytes.Repeat([]byte{'x'}, len(original))
	if err := os.WriteFile(playerPath, mutated, info.Mode().Perm()); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(playerPath, info.ModTime(), info.ModTime()); err != nil {
		t.Fatal(err)
	}

	after, err := inspectNativeBackup(backupDir, backupID)
	if err != nil {
		t.Fatal(err)
	}
	if before.Digest == after.Digest {
		t.Fatal("native backup digest did not cover changed player save content")
	}
	if _, err := ValidateNativeBackupSelection(backupID, before.Digest); !errors.Is(err, ErrNativeBackupChanged) {
		t.Fatalf("expected stale selection after player save mutation, got %v", err)
	}
}

func TestNativeBackupRejectsWrongTopLevelEntryTypes(t *testing.T) {
	for _, testCase := range []struct {
		name   string
		mutate func(t *testing.T, backupDir string)
	}{
		{
			name: "level directory",
			mutate: func(t *testing.T, backupDir string) {
				t.Helper()
				if err := os.Remove(filepath.Join(backupDir, "Level.sav")); err != nil {
					t.Fatal(err)
				}
				writeTestFile(t, filepath.Join(backupDir, "Level.sav", "nested.sav"), []byte("invalid"))
			},
		},
		{
			name: "players file",
			mutate: func(t *testing.T, backupDir string) {
				t.Helper()
				if err := os.RemoveAll(filepath.Join(backupDir, "Players")); err != nil {
					t.Fatal(err)
				}
				writeTestFile(t, filepath.Join(backupDir, "Players"), []byte("invalid"))
			},
		},
		{
			name: "world option directory",
			mutate: func(t *testing.T, backupDir string) {
				t.Helper()
				writeTestFile(t, filepath.Join(backupDir, "WorldOption.sav", "nested.sav"), []byte("invalid"))
			},
		},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			_, worldDir, _ := setupNativeBackupTest(t)
			backupID := "2026.07.13-10.04.00"
			backupDir := filepath.Join(worldDir, "backup", "world", backupID)
			writeNativeBackupFixture(t, backupDir, false)
			testCase.mutate(t, backupDir)

			if _, err := inspectNativeBackup(backupDir, backupID); !errors.Is(err, ErrNativeBackupInvalid) {
				t.Fatalf("expected invalid native backup, got %v", err)
			}
		})
	}
}

func setupNativeBackupTest(t *testing.T) (string, string, *bbolt.DB) {
	t.Helper()
	workspace := t.TempDir()
	worldDir := filepath.Join(workspace, "Saved", "SaveGames", "0", "TESTWORLD")
	writeTestFile(t, filepath.Join(worldDir, "Level.sav"), []byte("current-level"))
	writeTestFile(t, filepath.Join(worldDir, "LevelMeta.sav"), []byte("current-meta"))
	writeTestFile(t, filepath.Join(worldDir, "Players", "CURRENT.sav"), []byte("current-player"))
	writeTestFile(t, filepath.Join(worldDir, "WorldOption.sav"), []byte("current-world-option"))

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
	viper.Set("save.path", worldDir)
	viper.Set("rest.address", "")
	viper.Set("palworld.control.mode", "disabled")
	originalOnlineProbe := serverOnlineProbe
	serverOnlineProbe = func() bool { return false }
	t.Cleanup(func() { serverOnlineProbe = originalOnlineProbe })
	t.Chdir(workspace)
	return workspace, worldDir, db
}

func writeNativeBackupFixture(t *testing.T, backupDir string, withWorldOption bool) {
	t.Helper()
	writeTestFile(t, filepath.Join(backupDir, "Level.sav"), []byte("backup-level"))
	writeTestFile(t, filepath.Join(backupDir, "LevelMeta.sav"), []byte("backup-meta"))
	writeTestFile(t, filepath.Join(backupDir, "Players", "BACKUP.sav"), []byte("backup-player"))
	if withWorldOption {
		writeTestFile(t, filepath.Join(backupDir, "WorldOption.sav"), []byte("backup-world-option"))
	}
}

func writeTestFile(t *testing.T, path string, content []byte) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, content, 0o600); err != nil {
		t.Fatal(err)
	}
}

func assertFileContent(t *testing.T, path string, expected []byte) {
	t.Helper()
	actual, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(actual, expected) {
		t.Fatalf("unexpected content in %s: %q", path, actual)
	}
}

func assertZipFileContent(t *testing.T, archivePath, baseName string, expected []byte) {
	t.Helper()
	archive, err := zip.OpenReader(archivePath)
	if err != nil {
		t.Fatal(err)
	}
	defer archive.Close()
	for _, file := range archive.File {
		if filepath.Base(file.Name) != baseName {
			continue
		}
		reader, err := file.Open()
		if err != nil {
			t.Fatal(err)
		}
		actual, readErr := io.ReadAll(reader)
		closeErr := reader.Close()
		if readErr != nil {
			t.Fatal(readErr)
		}
		if closeErr != nil {
			t.Fatal(closeErr)
		}
		if !bytes.Equal(actual, expected) {
			t.Fatalf("unexpected %s content in %s: %q", baseName, archivePath, actual)
		}
		return
	}
	t.Fatalf("%s was not found in %s", baseName, archivePath)
}
