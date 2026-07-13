package tool

import (
	"context"
	"encoding/hex"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestSyncWorldOptionReplacesExistingFileWithSafetyBackup(t *testing.T) {
	workspace, worldDir, db := setupNativeBackupTest(t)
	targetPath := filepath.Join(worldDir, "WorldOption.sav")
	fingerprint, err := fingerprintSaveFile(targetPath)
	if err != nil {
		t.Fatal(err)
	}
	expectedDigest := hex.EncodeToString(fingerprint.SHA256[:])

	originalEditor := worldOptionEditor
	worldOptionEditor = func(sourcePath, outputPath, settingsPath string) (worldOptionEditorResult, error) {
		if sourcePath != targetPath {
			t.Fatalf("unexpected WorldOption source: %s", sourcePath)
		}
		if _, err := os.Stat(settingsPath); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(outputPath, []byte("synchronized-world-option"), 0o600); err != nil {
			return worldOptionEditorResult{}, err
		}
		return validWorldOptionEditorResult(false), nil
	}
	t.Cleanup(func() { worldOptionEditor = originalEditor })

	result, err := SyncWorldOption(context.Background(), db, WorldOptionSyncOptions{
		ConfigContent:        testPalWorldSettings,
		ExpectedSHA256:       expectedDigest,
		ConfirmServerStopped: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.WorldOption.Created || result.WorldOption.GameVersion != "1.0.0" ||
		result.WorldOption.SHA256 == "" || result.SafetyBackup.Path == "" {
		t.Fatalf("unexpected WorldOption sync result: %#v", result)
	}
	assertFileContent(t, targetPath, []byte("synchronized-world-option"))
	assertZipFileContent(
		t,
		filepath.Join(workspace, "backups", result.SafetyBackup.Path),
		"WorldOption.sav",
		[]byte("current-world-option"),
	)
}

func TestSyncWorldOptionGeneratesMissingFile(t *testing.T) {
	_, worldDir, db := setupNativeBackupTest(t)
	targetPath := filepath.Join(worldDir, "WorldOption.sav")
	if err := os.Remove(targetPath); err != nil {
		t.Fatal(err)
	}

	originalEditor := worldOptionEditor
	worldOptionEditor = func(sourcePath, outputPath, _ string) (worldOptionEditorResult, error) {
		if sourcePath != targetPath {
			t.Fatalf("unexpected generated WorldOption source: %s", sourcePath)
		}
		if err := os.WriteFile(outputPath, []byte("generated-world-option"), 0o600); err != nil {
			return worldOptionEditorResult{}, err
		}
		return validWorldOptionEditorResult(true), nil
	}
	t.Cleanup(func() { worldOptionEditor = originalEditor })

	result, err := SyncWorldOption(context.Background(), db, WorldOptionSyncOptions{
		ConfigContent:        testPalWorldSettings,
		ConfirmServerStopped: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if !result.WorldOption.Created {
		t.Fatalf("missing WorldOption.sav was not reported as generated: %#v", result)
	}
	assertFileContent(t, targetPath, []byte("generated-world-option"))
}

func TestWorldOptionSyncRejectsStaleDigestAndRequiresConfirmation(t *testing.T) {
	_, worldDir, db := setupNativeBackupTest(t)
	targetPath := filepath.Join(worldDir, "WorldOption.sav")
	if err := ValidateWorldOptionSync(testPalWorldSettings, "stale"); !errors.Is(err, ErrWorldOptionConflict) {
		t.Fatalf("expected stale WorldOption conflict, got %v", err)
	}
	fingerprint, err := fingerprintSaveFile(targetPath)
	if err != nil {
		t.Fatal(err)
	}
	_, err = SyncWorldOption(context.Background(), db, WorldOptionSyncOptions{
		ConfigContent:  testPalWorldSettings,
		ExpectedSHA256: hex.EncodeToString(fingerprint.SHA256[:]),
	})
	if !errors.Is(err, ErrSaveEditConfirmation) {
		t.Fatalf("expected stopped-server confirmation, got %v", err)
	}
	assertFileContent(t, targetPath, []byte("current-world-option"))
}

func validWorldOptionEditorResult(created bool) worldOptionEditorResult {
	return worldOptionEditorResult{
		Created:        created,
		GameVersion:    "1.0.0",
		UpdatedKeys:    []string{"ServerName"},
		SkippedKeys:    []string{"bEnableVoiceChat"},
		SettingsDigest: "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef",
	}
}
