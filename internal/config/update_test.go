package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/spf13/viper"
	"github.com/zaigie/palworld-server-tool/internal/database"
	"go.etcd.io/bbolt"
)

func openConfigTestDB(t *testing.T) *bbolt.DB {
	t.Helper()
	db, err := bbolt.Open(filepath.Join(t.TempDir(), "pst.db"), 0o600, nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := database.EnsureBuckets(db); err != nil {
		db.Close()
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	return db
}

func TestInitGeneratesAndStoresInitialPassword(t *testing.T) {
	viper.Reset()
	t.Cleanup(viper.Reset)
	db := openConfigTestDB(t)
	var current Config
	result := Init(db, &current)
	if result.InitialAdminPassword == "" {
		t.Fatal("initial administrator password was not generated")
	}
	values, err := database.ListConfigValues(db)
	if err != nil {
		t.Fatal(err)
	}
	if values["web.password"] != result.InitialAdminPassword {
		t.Fatalf("stored password does not match generated password")
	}
	if viper.GetInt("web.port") != 8080 {
		t.Fatalf("default web port = %d", viper.GetInt("web.port"))
	}
}

func TestApplyValuesUpdatesDatabaseAndRuntime(t *testing.T) {
	viper.Reset()
	t.Cleanup(viper.Reset)
	db := openConfigTestDB(t)
	var current Config
	Init(db, &current)

	if err := ApplyValues(map[string]any{
		"palworld.control.mode":   "process",
		"palworld.control.target": "C:/PalServer/PalServer.exe",
		"save.path":               "C:/PalServer/Pal/Saved",
	}); err != nil {
		t.Fatal(err)
	}
	if got := viper.GetString("palworld.control.mode"); got != "process" {
		t.Fatalf("runtime control mode = %q", got)
	}
	values, err := database.ListConfigValues(db)
	if err != nil {
		t.Fatal(err)
	}
	if values["save.path"] != "C:/PalServer/Pal/Saved" {
		t.Fatalf("database save path = %#v", values["save.path"])
	}
}

func TestFrozenRuntimeStoresValuesForNextRestart(t *testing.T) {
	viper.Reset()
	t.Cleanup(viper.Reset)
	db := openConfigTestDB(t)
	var current Config
	Init(db, &current)
	viper.Set("save.path", "C:/Current/Pal/Saved")
	runtimeFrozen.Store(true)
	t.Cleanup(func() { runtimeFrozen.Store(false) })

	applied, err := ApplyValuesWithRuntime(map[string]any{
		"save.path": "D:/Next/Pal/Saved",
	})
	if err != nil {
		t.Fatal(err)
	}
	if applied {
		t.Fatal("frozen runtime was mutated")
	}
	if got := viper.GetString("save.path"); got != "C:/Current/Pal/Saved" {
		t.Fatalf("runtime save path = %q", got)
	}
	values, err := database.ListConfigValues(db)
	if err != nil {
		t.Fatal(err)
	}
	if values["save.path"] != "D:/Next/Pal/Saved" {
		t.Fatalf("stored save path = %#v", values["save.path"])
	}
}

func TestInitImportsLegacyConfigOnce(t *testing.T) {
	viper.Reset()
	t.Cleanup(viper.Reset)
	db := openConfigTestDB(t)
	workingDir := t.TempDir()
	previousDir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(workingDir); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(previousDir) })
	legacy := "web:\n  password: migrated-secret\n  port: 9191\nsave:\n  path: C:/PalServer/Pal/Saved\n"
	if err := os.WriteFile(filepath.Join(workingDir, "config.yaml"), []byte(legacy), 0o600); err != nil {
		t.Fatal(err)
	}

	var current Config
	result := Init(db, &current)
	if result.MigratedFrom == "" || result.InitialAdminPassword != "" {
		t.Fatalf("unexpected migration result: %#v", result)
	}
	if viper.GetInt("web.port") != 9191 || viper.GetString("web.password") != "migrated-secret" {
		t.Fatalf("legacy values were not loaded")
	}
	if err := os.WriteFile(filepath.Join(workingDir, "config.yaml"), []byte("web:\n  port: 9292\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	viper.Reset()
	var second Config
	secondResult := Init(db, &second)
	if secondResult.MigratedFrom != "" || viper.GetInt("web.port") != 9191 {
		t.Fatalf("legacy config was imported more than once: %#v", secondResult)
	}
}
