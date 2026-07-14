package discovery

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/spf13/viper"
	"github.com/zaigie/palworld-server-tool/internal/config"
	"github.com/zaigie/palworld-server-tool/internal/database"
	"go.etcd.io/bbolt"
)

func writeDiscoveryFile(t *testing.T, path, content string, mode os.FileMode) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), mode); err != nil {
		t.Fatal(err)
	}
}

func createDiscoveryInstall(t *testing.T, root string) string {
	t.Helper()
	installDir := filepath.Join(root, "steamapps", "common", "PalServer")
	writeDiscoveryFile(t, filepath.Join(installDir, platformLauncherName()), "launcher", 0o700)
	configDir := filepath.Join(installDir, "Pal", "Saved", "Config", platformConfigDirectory())
	writeDiscoveryFile(t, filepath.Join(configDir, "PalWorldSettings.ini"),
		`[/Script/Pal.PalGameWorldSettings]
OptionSettings=(AdminPassword="secret",RESTAPIEnabled=True,RESTAPIPort=9123,RCONEnabled=True,RCONPort=26575)
`, 0o600)
	writeDiscoveryFile(t, filepath.Join(configDir, "GameUserSettings.ini"), "DedicatedServerName=WORLD_A\n", 0o600)
	worldA := filepath.Join(installDir, "Pal", "Saved", "SaveGames", "0", "WORLD_A", "Level.sav")
	worldB := filepath.Join(installDir, "Pal", "Saved", "SaveGames", "0", "WORLD_B", "Level.sav")
	writeDiscoveryFile(t, worldA, "world-a", 0o600)
	writeDiscoveryFile(t, worldB, "world-b", 0o600)
	newer := time.Now().Add(time.Hour)
	if err := os.Chtimes(worldB, newer, newer); err != nil {
		t.Fatal(err)
	}
	return installDir
}

func TestInspectInstallDirDerivesServerConfiguration(t *testing.T) {
	installDir := createDiscoveryInstall(t, t.TempDir())
	candidate, ok := inspectInstallDir(installDir, "test", "manifest.acf")
	if !ok {
		t.Fatal("valid PalServer installation was not detected")
	}
	if candidate.RESTPort != 9123 || candidate.RCONPort != 26575 || !candidate.RESTEnabled || !candidate.RCONEnabled {
		t.Fatalf("unexpected network settings: %#v", candidate)
	}
	if candidate.adminPassword != "secret" {
		t.Fatalf("admin password = %q", candidate.adminPassword)
	}
	if filepath.Base(candidate.SavePath) != "WORLD_A" {
		t.Fatalf("dedicated world was not preferred: %q", candidate.SavePath)
	}
	if len(candidate.Worlds) != 2 || candidate.Score < 100 {
		t.Fatalf("unexpected candidate score/worlds: %#v", candidate)
	}
}

func TestInspectCandidatePathFindsInstallAncestor(t *testing.T) {
	installDir := createDiscoveryInstall(t, t.TempDir())
	nestedPath := filepath.Join(installDir, "Pal", "Saved", "SaveGames", "0", "WORLD_A", "Level.sav")
	candidate, ok := inspectCandidatePath(nestedPath, "manual", "")
	if !ok {
		t.Fatal("PalServer installation was not derived from a nested path")
	}
	if filepath.Clean(candidate.InstallDir) != filepath.Clean(installDir) {
		t.Fatalf("install directory = %q", candidate.InstallDir)
	}
}

func TestSteamSeedsReadsAdditionalLibrary(t *testing.T) {
	primary := t.TempDir()
	library := t.TempDir()
	installDir := createDiscoveryInstall(t, library)
	manifest := filepath.Join(library, "steamapps", "appmanifest_"+palServerAppID+".acf")
	writeDiscoveryFile(t, manifest, `"AppState" { "installdir" "PalServer" }`, 0o600)
	vdfPath := library
	if runtime.GOOS == "windows" {
		vdfPath = strings.ReplaceAll(vdfPath, `\`, `\\`)
	}
	writeDiscoveryFile(t, filepath.Join(primary, "steamapps", "libraryfolders.vdf"),
		fmt.Sprintf(`"libraryfolders" { "1" { "path" "%s" } }`, vdfPath), 0o600)

	seeds := steamSeeds(primary)
	found := false
	for _, seed := range seeds {
		if filepath.Clean(seed.installDir) == filepath.Clean(installDir) && seed.manifestPath == manifest {
			found = true
		}
	}
	if !found {
		t.Fatalf("additional Steam library was not parsed: %#v", seeds)
	}
}

func TestInferSteamCMDPathFromInstallLayout(t *testing.T) {
	library := t.TempDir()
	installDir := createDiscoveryInstall(t, library)
	steamCMDName := "steamcmd.sh"
	if runtime.GOOS == "windows" {
		steamCMDName = "steamcmd.exe"
	}
	steamCMDPath := filepath.Join(library, steamCMDName)
	writeDiscoveryFile(t, steamCMDPath, "steamcmd", 0o700)
	if got := inferSteamCMDPath(installDir); filepath.Clean(got) != filepath.Clean(steamCMDPath) {
		t.Fatalf("SteamCMD path = %q", got)
	}
}

func TestApplyCandidatePersistsAutomaticPaths(t *testing.T) {
	viper.Reset()
	t.Cleanup(viper.Reset)
	db, err := bbolt.Open(filepath.Join(t.TempDir(), "pst.db"), 0o600, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if err := database.EnsureBuckets(db); err != nil {
		t.Fatal(err)
	}
	var current config.Config
	config.Init(db, &current)

	installDir := createDiscoveryInstall(t, t.TempDir())
	candidate, ok := inspectInstallDir(installDir, "manual", "")
	if !ok {
		t.Fatal("fixture was not detected")
	}
	if applied, err := applyCandidate(candidate, true); err != nil {
		t.Fatal(err)
	} else if !applied {
		t.Fatal("configuration was not applied before runtime freeze")
	}
	if got := viper.GetString("palworld.control.target"); got != candidate.LauncherPath {
		t.Fatalf("control target = %q", got)
	}
	if got := viper.GetString("save.path"); got != candidate.SavePath {
		t.Fatalf("save path = %q", got)
	}
	if got := viper.GetString("rest.address"); got != "http://127.0.0.1:9123" {
		t.Fatalf("REST address = %q", got)
	}
	values, err := database.ListConfigValues(db)
	if err != nil {
		t.Fatal(err)
	}
	if values["palworld.control.mode"] != "process" || values["rest.address"] != "http://127.0.0.1:9123" {
		t.Fatalf("unexpected persisted configuration: %#v", values)
	}
	if values["save.path"] != candidate.SavePath || values["steamcmd.install_dir"] != candidate.InstallDir {
		t.Fatalf("automatic paths were not stored in the database: %#v", values)
	}
}
