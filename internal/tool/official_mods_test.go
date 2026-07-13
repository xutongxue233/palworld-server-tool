package tool

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/spf13/viper"
	"github.com/zaigie/palworld-server-tool/internal/system"
)

func TestInspectOfficialModsUsesOfficialLoaderMetadata(t *testing.T) {
	installDir := setupOfficialModTest(t)
	workshop := filepath.Join(installDir, "Mods", "Workshop")
	writeOfficialModInfo(t, workshop, "1001", officialModInfo{
		ModName:      "Server Tweaks",
		PackageName:  "ServerTweaks",
		Version:      "1.0.0-3",
		Author:       "Example",
		Dependencies: []string{"UE4SS"},
		Tags:         []string{"Gameplay"},
		InstallRule: []officialModInstallRule{{
			Type:     "Paks",
			IsServer: boolPointer(true),
			Targets:  []string{"./Content"},
		}},
	})
	writeOfficialModInfo(t, workshop, "1002", officialModInfo{
		ModName:     "UE4SS",
		PackageName: "UE4SS",
		Version:     "3.0.1",
		InstallRule: []officialModInstallRule{{
			Type:     "UE4SS",
			IsServer: boolPointer(true),
			Targets:  []string{"./UE4SS"},
		}},
	})
	settingsPath := filepath.Join(installDir, "Mods", "PalModSettings.ini")
	mustWriteFile(t, settingsPath, []byte(strings.Join([]string{
		"[PalModSettings]",
		"bGlobalEnableMod=True",
		"ActiveModList=ServerTweaks",
		"ActiveModList=UE4SS",
		"",
	}, "\r\n")))
	manifest := filepath.Join(installDir, "Mods", "ManagedMods", "ServerTweaks", "InstallManifest.json")
	mustWriteFile(t, manifest, []byte("{}"))

	status := InspectOfficialMods()
	if !status.Supported || !status.Configured || !status.Manageable {
		t.Fatalf("expected manageable Windows status, got %#v", status)
	}
	if status.GameVersion != "1.0.0" || status.Inventory.WorkshopSource != "default" ||
		status.Inventory.WorkshopRoot != workshop || !status.Inventory.WorkshopAvailable {
		t.Fatalf("unexpected official-loader roots: %#v", status)
	}
	if status.SettingsSHA256 == "" || status.StatusDigest == "" || len(status.StatusDigest) != 64 {
		t.Fatalf("expected stable settings/status digests, got %#v", status)
	}
	serverTweaks := findOfficialModPackage(t, status.Inventory.Packages, "ServerTweaks")
	if !serverTweaks.Valid || !serverTweaks.ServerCompatible || !serverTweaks.Listed ||
		!serverTweaks.EffectiveEnabled || !serverTweaks.Deployed || serverTweaks.PendingRestart {
		t.Fatalf("unexpected deployed package status: %#v", serverTweaks)
	}
	ue4ss := findOfficialModPackage(t, status.Inventory.Packages, "UE4SS")
	if !ue4ss.EffectiveEnabled || !ue4ss.PendingRestart || ue4ss.Deployed {
		t.Fatalf("expected undeployed active dependency to require restart: %#v", ue4ss)
	}
	if status.ExistingWorlds != 0 || !status.SafetyBackupReady {
		t.Fatalf("new installation should not require a save backup: %#v", status)
	}
}

func TestPlanOfficialModSettingsValidatesServerRulesDependenciesAndDuplicates(t *testing.T) {
	installDir := setupOfficialModTest(t)
	workshop := filepath.Join(installDir, "Mods", "Workshop")
	writeOfficialModInfo(t, workshop, "main", officialModInfo{
		ModName:      "Main",
		PackageName:  "MainMod",
		Dependencies: []string{"Dependency"},
		InstallRule: []officialModInstallRule{{
			Type:     "Lua",
			IsServer: boolPointer(true),
			Targets:  []string{"./Scripts"},
		}},
	})
	writeOfficialModInfo(t, workshop, "dependency", officialModInfo{
		ModName:     "Dependency",
		PackageName: "Dependency",
		InstallRule: []officialModInstallRule{{
			Type:     "UE4SS",
			IsServer: boolPointer(true),
			Targets:  []string{"./UE4SS"},
		}},
	})
	writeOfficialModInfo(t, workshop, "client", officialModInfo{
		ModName:     "Client Only",
		PackageName: "ClientOnly",
		InstallRule: []officialModInstallRule{{
			Type:    "Paks",
			Targets: []string{"./Content"},
		}},
	})

	missingDependency := PlanOfficialModSettings(OfficialModSettings{
		GlobalEnabled: true,
		ActiveModList: []string{"MainMod"},
	})
	assertModDiagnosticCode(t, missingDependency.Issues, "dependency_disabled")
	if missingDependency.CanApply {
		t.Fatal("dependency validation should block the plan")
	}

	clientOnly := PlanOfficialModSettings(OfficialModSettings{
		GlobalEnabled: true,
		ActiveModList: []string{"ClientOnly"},
	})
	assertModDiagnosticCode(t, clientOnly.Issues, "server_rule_missing")

	writeOfficialModInfo(t, workshop, "duplicate", officialModInfo{
		ModName:     "Duplicate",
		PackageName: "Dependency",
		InstallRule: []officialModInstallRule{{
			Type:     "UE4SS",
			IsServer: boolPointer(true),
			Targets:  []string{"./Other"},
		}},
	})
	duplicate := PlanOfficialModSettings(OfficialModSettings{
		GlobalEnabled: true,
		ActiveModList: []string{"Dependency"},
	})
	assertModDiagnosticCode(t, duplicate.Issues, "duplicate_package_name")
}

func TestOfficialModMetadataRejectsUnsafeInstallRules(t *testing.T) {
	installDir := setupOfficialModTest(t)
	workshop := filepath.Join(installDir, "Mods", "Workshop")
	writeOfficialModInfo(t, workshop, "unsafe", officialModInfo{
		ModName:     "Unsafe",
		PackageName: "Unsafe",
		InstallRule: []officialModInstallRule{{
			Type:     "Paks",
			IsServer: boolPointer(true),
			Targets:  []string{"../outside"},
		}},
	})
	writeOfficialModInfo(t, workshop, "unknown", officialModInfo{
		ModName:     "Unknown Type",
		PackageName: "UnknownType",
		InstallRule: []officialModInstallRule{{
			Type:     "ArbitraryExecutable",
			IsServer: boolPointer(true),
			Targets:  []string{"./payload"},
		}},
	})
	writeOfficialModInfo(t, workshop, "empty-targets", officialModInfo{
		ModName:     "Empty Targets",
		PackageName: "EmptyTargets",
		InstallRule: []officialModInstallRule{{
			Type:     "Paks",
			IsServer: boolPointer(true),
		}},
	})

	status := InspectOfficialMods()
	unsafe := findOfficialModPackage(t, status.Inventory.Packages, "Unsafe")
	if unsafe.Valid {
		t.Fatalf("unsafe traversal target must invalidate metadata: %#v", unsafe)
	}
	unknown := findOfficialModPackage(t, status.Inventory.Packages, "UnknownType")
	if !unknown.Valid || unknown.ServerCompatible {
		t.Fatalf("unknown install type must not be server-compatible: %#v", unknown)
	}
	assertModDiagnosticCode(t, unknown.Warnings, "install_type_unknown")
	emptyTargets := findOfficialModPackage(t, status.Inventory.Packages, "EmptyTargets")
	if emptyTargets.Valid || emptyTargets.ServerCompatible {
		t.Fatalf("server rule without targets must be invalid and incompatible: %#v", emptyTargets)
	}
	assertModDiagnosticCode(t, emptyTargets.Issues, "metadata_invalid")
}

func TestOfficialModsHonorNoModsAndWorkshopLaunchOverride(t *testing.T) {
	installDir := setupOfficialModTest(t)
	defaultWorkshop := filepath.Join(installDir, "Mods", "Workshop")
	overrideWorkshop := filepath.Join(t.TempDir(), "external-workshop")
	mustMkdirAll(t, overrideWorkshop)
	writeOfficialModInfo(t, defaultWorkshop, "default", serverCompatibleMod("DefaultMod"))
	writeOfficialModInfo(t, overrideWorkshop, "override", serverCompatibleMod("OverrideMod"))
	mustWriteFile(t, filepath.Join(installDir, "Mods", "PalModSettings.ini"), []byte(strings.Join([]string{
		"[PalModSettings]",
		"bGlobalEnableMod=True",
		"ActiveModList=OverrideMod",
		"",
	}, "\n")))
	viper.Set("palworld.control.arguments", []string{"-NoMods", "-workshopdir=" + overrideWorkshop})

	status := InspectOfficialMods()
	if !status.ForcedDisabled || status.Inventory.WorkshopSource != "launch_argument" ||
		status.Inventory.WorkshopRoot != overrideWorkshop {
		t.Fatalf("launch arguments were not applied: %#v", status)
	}
	override := findOfficialModPackage(t, status.Inventory.Packages, "OverrideMod")
	if !override.Listed || override.EffectiveEnabled {
		t.Fatalf("-NoMods must make listed packages ineffective: %#v", override)
	}

	changeRoot := PlanOfficialModSettings(OfficialModSettings{
		GlobalEnabled:   true,
		WorkshopRootDir: defaultWorkshop,
		ActiveModList:   []string{"OverrideMod"},
	})
	assertModDiagnosticCode(t, changeRoot.Issues, "workshop_root_overridden")
}

func TestPalModSettingsRenderPreservesUnknownContent(t *testing.T) {
	original := []byte("\ufeff[Other]\r\nValue=Keep\r\n\r\n[PalModSettings]\r\n; keep this comment\r\nbGlobalEnableMod=False\r\nbGlobalEnableMod=True\r\nUnknownKey=KeepMe\r\nActiveModList=Old\r\n")
	document, err := parsePalModSettings(original)
	if err != nil {
		t.Fatal(err)
	}
	assertModDiagnosticCode(t, document.warnings, "duplicate_global_setting")
	next, err := renderPalModSettings(document, OfficialModSettings{
		GlobalEnabled:   false,
		WorkshopRootDir: `C:\Steam Workshop\1623730`,
		ActiveModList:   []string{"First", "Second"},
	})
	if err != nil {
		t.Fatal(err)
	}
	text := string(next)
	for _, preserved := range []string{"\ufeff[Other]", "Value=Keep", "; keep this comment", "UnknownKey=KeepMe"} {
		if !strings.Contains(text, preserved) {
			t.Fatalf("rendered settings lost %q:\n%s", preserved, text)
		}
	}
	if strings.Count(text, "bGlobalEnableMod=") != 1 || strings.Count(text, "ActiveModList=") != 2 ||
		!strings.Contains(text, "\r\n") {
		t.Fatalf("known settings were not canonicalized with CRLF preserved:\n%s", text)
	}
	parsed, err := parsePalModSettings(next)
	if err != nil {
		t.Fatal(err)
	}
	if parsed.settings.GlobalEnabled || parsed.settings.WorkshopRootDir != `C:\Steam Workshop\1623730` ||
		!equalFoldedStringSlices(parsed.settings.ActiveModList, []string{"First", "Second"}) {
		t.Fatalf("unexpected round trip: %#v", parsed.settings)
	}
}

func TestApplyAndRollbackOfficialModSettings(t *testing.T) {
	installDir := setupOfficialModTest(t)
	workshop := filepath.Join(installDir, "Mods", "Workshop")
	writeOfficialModInfo(t, workshop, "1001", serverCompatibleMod("First"))
	writeOfficialModInfo(t, workshop, "1002", serverCompatibleMod("Second"))
	settingsPath := filepath.Join(installDir, "Mods", "PalModSettings.ini")
	original := []byte("[PalModSettings]\r\nbGlobalEnableMod=False\r\nActiveModList=First\r\n")
	mustWriteFile(t, settingsPath, original)

	desired := OfficialModSettings{
		GlobalEnabled: true,
		ActiveModList: []string{"First", "Second"},
	}
	plan := PlanOfficialModSettings(desired)
	if !plan.CanApply || !plan.Changed || plan.SafetyBackupRequired {
		t.Fatalf("unexpected plan: %#v", plan)
	}
	result, err := ApplyOfficialModSettings(context.Background(), OfficialModApplyOptions{
		DesiredSettings:    desired,
		ExpectedPlanDigest: plan.PlanDigest,
		ConfirmServerStop:  true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if !result.Changed || result.Created || result.RecoveryPath == "" || !result.RestartRequired || result.RolledBack {
		t.Fatalf("unexpected apply result: %#v", result)
	}
	recovery, err := os.ReadFile(result.RecoveryPath)
	if err != nil {
		t.Fatal(err)
	}
	if string(recovery) != string(original) {
		t.Fatalf("recovery point does not contain original settings:\n%s", recovery)
	}
	updated, err := os.ReadFile(settingsPath)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(updated), "bGlobalEnableMod=True") ||
		!strings.Contains(string(updated), "ActiveModList=Second") {
		t.Fatalf("settings were not updated:\n%s", updated)
	}

	if err := RollbackOfficialModSettings(context.Background(), &result, true); err != nil {
		t.Fatal(err)
	}
	if !result.RolledBack || result.RollbackAt == nil {
		t.Fatalf("rollback status was not recorded: %#v", result)
	}
	restored, err := os.ReadFile(settingsPath)
	if err != nil {
		t.Fatal(err)
	}
	if string(restored) != string(original) {
		t.Fatalf("rollback did not restore the original file:\n%s", restored)
	}
}

func TestApplyAndRollbackNewPalModSettingsFile(t *testing.T) {
	installDir := setupOfficialModTest(t)
	workshop := filepath.Join(installDir, "Mods", "Workshop")
	writeOfficialModInfo(t, workshop, "1001", serverCompatibleMod("First"))
	settingsPath := filepath.Join(installDir, "Mods", "PalModSettings.ini")
	desired := OfficialModSettings{GlobalEnabled: true, ActiveModList: []string{"First"}}
	plan := PlanOfficialModSettings(desired)
	result, err := ApplyOfficialModSettings(context.Background(), OfficialModApplyOptions{
		DesiredSettings:    desired,
		ExpectedPlanDigest: plan.PlanDigest,
		ConfirmServerStop:  true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if !result.Created || result.PreviousExists || filepath.Ext(result.RecoveryPath) != ".json" {
		t.Fatalf("expected an absent-state recovery marker: %#v", result)
	}
	if _, err := os.Stat(settingsPath); err != nil {
		t.Fatal(err)
	}
	if err := RollbackOfficialModSettings(context.Background(), &result, true); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(settingsPath); !os.IsNotExist(err) {
		t.Fatalf("rollback should restore the absent settings state, got %v", err)
	}
}

func TestOfficialModPlanDetectsStaleMetadata(t *testing.T) {
	installDir := setupOfficialModTest(t)
	workshop := filepath.Join(installDir, "Mods", "Workshop")
	writeOfficialModInfo(t, workshop, "1001", serverCompatibleMod("First"))
	desired := OfficialModSettings{GlobalEnabled: true, ActiveModList: []string{"First"}}
	plan := PlanOfficialModSettings(desired)
	updated := serverCompatibleMod("First")
	updated.Version = "changed"
	writeOfficialModInfo(t, workshop, "1001", updated)

	_, err := ApplyOfficialModSettings(context.Background(), OfficialModApplyOptions{
		DesiredSettings:    desired,
		ExpectedPlanDigest: plan.PlanDigest,
		ConfirmServerStop:  true,
	})
	if !errors.Is(err, ErrOfficialModPlanChanged) {
		t.Fatalf("expected stale plan error, got %v", err)
	}
}

func TestOfficialModPlanRequiresExistingWorldSafetyBackup(t *testing.T) {
	installDir := setupOfficialModTest(t)
	workshop := filepath.Join(installDir, "Mods", "Workshop")
	writeOfficialModInfo(t, workshop, "1001", serverCompatibleMod("First"))
	world := filepath.Join(installDir, "Pal", "Saved", "SaveGames", "0", "WORLD")
	mustWriteFile(t, filepath.Join(world, "Level.sav"), []byte("level"))
	viper.Set("save.path", "")

	plan := PlanOfficialModSettings(OfficialModSettings{
		GlobalEnabled: true,
		ActiveModList: []string{"First"},
	})
	if !plan.SafetyBackupRequired || plan.SafetyBackupReady || plan.CanApply {
		t.Fatalf("existing worlds must require a ready backup source: %#v", plan)
	}
	assertModDiagnosticCode(t, plan.Issues, "save_backup_required")

	viper.Set("save.path", world)
	ready := PlanOfficialModSettings(OfficialModSettings{
		GlobalEnabled: true,
		ActiveModList: []string{"First"},
	})
	if !ready.SafetyBackupRequired || !ready.SafetyBackupReady || !ready.CanApply {
		t.Fatalf("local world inside the installation should make backup ready: %#v", ready)
	}
}

func TestOfficialModApplyFailurePreservesOriginalAndRecovery(t *testing.T) {
	installDir := setupOfficialModTest(t)
	workshop := filepath.Join(installDir, "Mods", "Workshop")
	writeOfficialModInfo(t, workshop, "1001", serverCompatibleMod("First"))
	settingsPath := filepath.Join(installDir, "Mods", "PalModSettings.ini")
	original := []byte("[PalModSettings]\nbGlobalEnableMod=False\n")
	mustWriteFile(t, settingsPath, original)
	desired := OfficialModSettings{GlobalEnabled: true, ActiveModList: []string{"First"}}
	plan := PlanOfficialModSettings(desired)
	previousReplace := officialModReplaceFile
	officialModReplaceFile = func(_, _ string) error { return errors.New("injected replace failure") }
	t.Cleanup(func() { officialModReplaceFile = previousReplace })

	result, err := ApplyOfficialModSettings(context.Background(), OfficialModApplyOptions{
		DesiredSettings:    desired,
		ExpectedPlanDigest: plan.PlanDigest,
		ConfirmServerStop:  true,
	})
	if !errors.Is(err, ErrOfficialModApplyFailed) {
		t.Fatalf("expected managed apply failure, got %v", err)
	}
	if result.RecoveryPath == "" {
		t.Fatal("recovery path must be returned after replacement fails")
	}
	current, readErr := os.ReadFile(settingsPath)
	if readErr != nil {
		t.Fatal(readErr)
	}
	if string(current) != string(original) {
		t.Fatalf("failed replacement changed the original file:\n%s", current)
	}
	if _, statErr := os.Stat(result.RecoveryPath); statErr != nil {
		t.Fatalf("recovery file was not preserved: %v", statErr)
	}
}

func TestOfficialModsRejectUnsupportedPlatformAndSymlinkRoot(t *testing.T) {
	installDir := setupOfficialModTest(t)
	previousPlatform := officialModRuntimeOS
	officialModRuntimeOS = "linux"
	status := InspectOfficialMods()
	if status.Supported || status.Manageable {
		t.Fatalf("Linux must remain unsupported by the official server mod loader: %#v", status)
	}
	officialModRuntimeOS = previousPlatform

	realRoot := filepath.Join(t.TempDir(), "real")
	mustMkdirAll(t, realRoot)
	linkedRoot := filepath.Join(t.TempDir(), "linked")
	if err := os.Symlink(realRoot, linkedRoot); err != nil {
		t.Skipf("symlink creation is not available: %v", err)
	}
	settingsPath := filepath.Join(installDir, "Mods", "PalModSettings.ini")
	mustWriteFile(t, settingsPath, []byte("[PalModSettings]\nWorkshopRootDir="+linkedRoot+"\n"))
	status = InspectOfficialMods()
	assertModDiagnosticCode(t, status.Inventory.Issues, "workshop_root_unsafe")
	if !status.Manageable {
		t.Fatalf("an unsafe current root should remain recoverable through a settings change: %#v", status)
	}
	unsafePlan := PlanOfficialModSettings(OfficialModSettings{WorkshopRootDir: linkedRoot})
	assertModDiagnosticCode(t, unsafePlan.Issues, "workshop_root_unsafe")
	if unsafePlan.CanApply {
		t.Fatalf("a new unsafe Workshop root must be blocked even with no active mods: %#v", unsafePlan)
	}
	recoveryPlan := PlanOfficialModSettings(OfficialModSettings{})
	if !recoveryPlan.CanApply || !recoveryPlan.Changed {
		t.Fatalf("the unsafe current root must remain recoverable by returning to the default root: %#v", recoveryPlan)
	}
}

func setupOfficialModTest(t *testing.T) string {
	t.Helper()
	keys := []string{
		"mods.install_dir",
		"steamcmd.install_dir",
		"save.path",
		"rest.address",
		"palworld.control.mode",
		"palworld.control.target",
		"palworld.control.arguments",
	}
	previous := make(map[string]any, len(keys))
	for _, key := range keys {
		previous[key] = viper.Get(key)
	}
	previousPlatform := officialModRuntimeOS
	previousNow := officialModNow
	previousBackupDir := officialModBackupDir
	previousReplace := officialModReplaceFile
	previousLink := officialModLinkFile
	previousRemove := officialModRemoveFile
	t.Cleanup(func() {
		for _, key := range keys {
			viper.Set(key, previous[key])
		}
		officialModRuntimeOS = previousPlatform
		officialModNow = previousNow
		officialModBackupDir = previousBackupDir
		officialModReplaceFile = previousReplace
		officialModLinkFile = previousLink
		officialModRemoveFile = previousRemove
	})

	installDir := filepath.Join(t.TempDir(), "PalServer")
	mustMkdirAll(t, filepath.Join(installDir, "Mods", "Workshop"))
	mustWriteFile(t, filepath.Join(installDir, "PalServer.exe"), []byte("launcher"))
	backupDir := filepath.Join(t.TempDir(), "backups")
	officialModRuntimeOS = "windows"
	officialModNow = func() time.Time { return time.Date(2026, 7, 13, 12, 0, 0, 123456789, time.UTC) }
	officialModBackupDir = func() (string, error) {
		if err := os.MkdirAll(backupDir, 0o755); err != nil {
			return "", err
		}
		return backupDir, nil
	}
	officialModReplaceFile = system.ReplaceFileAtomic
	officialModLinkFile = os.Link
	officialModRemoveFile = os.Remove
	viper.Set("mods.install_dir", installDir)
	viper.Set("steamcmd.install_dir", "")
	viper.Set("save.path", "")
	viper.Set("rest.address", "")
	viper.Set("palworld.control.mode", "disabled")
	viper.Set("palworld.control.target", "")
	viper.Set("palworld.control.arguments", []string{})
	return installDir
}

func serverCompatibleMod(packageName string) officialModInfo {
	return officialModInfo{
		ModName:     packageName,
		PackageName: packageName,
		Version:     "1.0.0",
		InstallRule: []officialModInstallRule{{
			Type:     "Paks",
			IsServer: boolPointer(true),
			Targets:  []string{"./Content"},
		}},
	}
}

func writeOfficialModInfo(t *testing.T, workshop, folder string, metadata officialModInfo) {
	t.Helper()
	content, err := json.Marshal(metadata)
	if err != nil {
		t.Fatal(err)
	}
	mustWriteFile(t, filepath.Join(workshop, folder, "Info.json"), content)
}

func findOfficialModPackage(t *testing.T, packages []OfficialModPackage, packageName string) OfficialModPackage {
	t.Helper()
	for _, pkg := range packages {
		if strings.EqualFold(pkg.PackageName, packageName) {
			return pkg
		}
	}
	t.Fatalf("package %q not found in %#v", packageName, packages)
	return OfficialModPackage{}
}

func assertModDiagnosticCode(t *testing.T, diagnostics []OfficialModDiagnostic, code string) {
	t.Helper()
	for _, diagnostic := range diagnostics {
		if diagnostic.Code == code {
			return
		}
	}
	t.Fatalf("diagnostic %q not found in %#v", code, diagnostics)
}

func boolPointer(value bool) *bool {
	return &value
}

func mustMkdirAll(t *testing.T, path string) {
	t.Helper()
	if err := os.MkdirAll(path, 0o755); err != nil {
		t.Fatal(err)
	}
}

func mustWriteFile(t *testing.T, path string, content []byte) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, content, 0o600); err != nil {
		t.Fatal(err)
	}
}
