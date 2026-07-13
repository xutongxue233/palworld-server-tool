package tool

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/viper"
)

const testPalWorldSettings = "[/Script/Pal.PalGameWorldSettings]\nOptionSettings=(ServerName=\"Test\",bIsUseBackupSaveData=True)\n"

func TestGameConfigReadWriteBackupAndConflict(t *testing.T) {
	tempDir := t.TempDir()
	path := filepath.Join(tempDir, "PalWorldSettings.ini")
	if err := os.WriteFile(path, []byte(testPalWorldSettings), 0o640); err != nil {
		t.Fatal(err)
	}
	oldWD, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(tempDir); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(oldWD) })
	viper.Reset()
	viper.Set("palworld.config_path", path)
	t.Cleanup(viper.Reset)

	loaded, err := ReadGameConfigFile()
	if err != nil {
		t.Fatal(err)
	}
	if !loaded.Configured || loaded.SHA256 == "" || loaded.Content != testPalWorldSettings {
		t.Fatalf("unexpected config read: %#v", loaded)
	}

	next := strings.Replace(testPalWorldSettings, "Test", "Production", 1)
	written, err := WriteGameConfigFile(next, loaded.SHA256)
	if err != nil {
		t.Fatal(err)
	}
	if !written.RestartRequired || written.SHA256 == loaded.SHA256 {
		t.Fatalf("unexpected write result: %#v", written)
	}
	backup, err := os.ReadFile(written.BackupPath)
	if err != nil {
		t.Fatal(err)
	}
	if string(backup) != testPalWorldSettings {
		t.Fatalf("backup did not preserve previous config: %q", backup)
	}
	current, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(current) != next {
		t.Fatalf("updated config mismatch: %q", current)
	}
	if _, err := WriteGameConfigFile(testPalWorldSettings, loaded.SHA256); !errors.Is(err, ErrGameConfigConflict) {
		t.Fatalf("expected stale digest conflict, got %v", err)
	}
}

func TestGameConfigReportsUnconfigured(t *testing.T) {
	viper.Reset()
	t.Cleanup(viper.Reset)
	result, err := ReadGameConfigFile()
	if err != nil {
		t.Fatal(err)
	}
	if result.Configured {
		t.Fatalf("expected unconfigured response: %#v", result)
	}
}

func TestGameConfigDetectsWorldOptionOverride(t *testing.T) {
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "PalWorldSettings.ini")
	worldDir := filepath.Join(tempDir, "Saved", "SaveGames", "0", "WORLD")
	if err := os.MkdirAll(worldDir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(configPath, []byte(testPalWorldSettings), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(worldDir, "Level.sav"), []byte("level"), 0o600); err != nil {
		t.Fatal(err)
	}
	worldOptionPath := filepath.Join(worldDir, "WorldOption.sav")
	if err := os.WriteFile(worldOptionPath, []byte("override"), 0o600); err != nil {
		t.Fatal(err)
	}
	viper.Reset()
	t.Cleanup(viper.Reset)
	viper.Set("palworld.config_path", configPath)
	viper.Set("save.path", worldDir)

	result, err := ReadGameConfigFile()
	if err != nil {
		t.Fatal(err)
	}
	if !result.WorldOption.Supported || !result.WorldOption.Present ||
		result.WorldOption.Path != worldOptionPath || result.WorldOption.SizeBytes != int64(len("override")) ||
		result.WorldOption.ModifiedAt == nil || len(result.WorldOption.SHA256) != 64 {
		t.Fatalf("WorldOption.sav override was not detected: %#v", result.WorldOption)
	}

	if err := os.Remove(worldOptionPath); err != nil {
		t.Fatal(err)
	}
	result, err = ReadGameConfigFile()
	if err != nil {
		t.Fatal(err)
	}
	if !result.WorldOption.Supported || result.WorldOption.Present || result.WorldOption.Path != worldOptionPath {
		t.Fatalf("unexpected absent WorldOption.sav status: %#v", result.WorldOption)
	}
}

func TestGameConfigValidationRejectsIncompleteOrEmptyOptions(t *testing.T) {
	for _, content := range []string{
		"[/Script/Pal.PalGameWorldSettings]\nOptionSettings=(ServerName=\"Test\"\n",
		"[/Script/Pal.PalGameWorldSettings]\nOptionSettings=()\n",
	} {
		if err := validateGameConfigContent([]byte(content)); err == nil {
			t.Fatalf("expected invalid configuration to be rejected: %q", content)
		}
	}
}
