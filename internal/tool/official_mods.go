package tool

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"
	"github.com/spf13/viper"
	"github.com/zaigie/palworld-server-tool/internal/system"
)

const (
	OfficialModGameVersion       = "1.0.0"
	PalworldWorkshopAppID        = 1623730
	maxPalModSettingsSize        = 1 << 20
	maxPalModInfoSize            = 1 << 20
	maxPalModPackages            = 4096
	maxPalModActivePackages      = 256
	maxPalModMetadataString      = 4096
	maxPalModMetadataListEntries = 256
)

var (
	ErrOfficialModsNotConfigured = errors.New("official mod management is not configured")
	ErrOfficialModsUnsupported   = errors.New("official server-side mods are supported only by the Windows dedicated server")
	ErrOfficialModsInvalid       = errors.New("official mod preflight failed")
	ErrOfficialModPlanChanged    = errors.New("official mod plan changed")
	ErrOfficialModApplyFailed    = errors.New("official mod settings update failed")
	ErrOfficialModRollbackFailed = errors.New("official mod settings rollback failed")

	officialModMu          sync.Mutex
	officialModRuntimeOS   = runtime.GOOS
	officialModNow         = time.Now
	officialModBackupDir   = GetBackupDir
	officialModReplaceFile = system.ReplaceFileAtomic
	officialModLinkFile    = os.Link
	officialModRemoveFile  = os.Remove
)

type OfficialModDiagnostic struct {
	Code        string `json:"code"`
	Message     string `json:"message"`
	FolderName  string `json:"folder_name,omitempty"`
	PackageName string `json:"package_name,omitempty"`
	Dependency  string `json:"dependency,omitempty"`
}

type OfficialModSettings struct {
	GlobalEnabled   bool     `json:"global_enabled"`
	WorkshopRootDir string   `json:"workshop_root_dir,omitempty"`
	ActiveModList   []string `json:"active_mod_list"`
}

type OfficialModPackage struct {
	FolderName         string                  `json:"folder_name"`
	Path               string                  `json:"path"`
	WorkshopItemID     string                  `json:"workshop_item_id,omitempty"`
	InfoPath           string                  `json:"info_path,omitempty"`
	InfoSHA256         string                  `json:"info_sha256,omitempty"`
	ModName            string                  `json:"mod_name,omitempty"`
	PackageName        string                  `json:"package_name,omitempty"`
	Version            string                  `json:"version,omitempty"`
	Author             string                  `json:"author,omitempty"`
	Thumbnail          string                  `json:"thumbnail,omitempty"`
	MinRevision        *int                    `json:"min_revision,omitempty"`
	DebugMode          bool                    `json:"debug_mode"`
	Dependencies       []string                `json:"dependencies,omitempty"`
	Tags               []string                `json:"tags,omitempty"`
	InstallTypes       []string                `json:"install_types,omitempty"`
	ServerInstallTypes []string                `json:"server_install_types,omitempty"`
	Valid              bool                    `json:"valid"`
	ServerCompatible   bool                    `json:"server_compatible"`
	Listed             bool                    `json:"listed"`
	EffectiveEnabled   bool                    `json:"effective_enabled"`
	Deployed           bool                    `json:"deployed"`
	PendingRestart     bool                    `json:"pending_restart"`
	PendingRemoval     bool                    `json:"pending_removal"`
	Issues             []OfficialModDiagnostic `json:"issues,omitempty"`
	Warnings           []OfficialModDiagnostic `json:"warnings,omitempty"`
}

type OfficialModInventory struct {
	WorkshopRoot      string                  `json:"workshop_root"`
	WorkshopSource    string                  `json:"workshop_source"`
	WorkshopAvailable bool                    `json:"workshop_available"`
	Packages          []OfficialModPackage    `json:"packages"`
	UnknownActiveMods []string                `json:"unknown_active_mods,omitempty"`
	Issues            []OfficialModDiagnostic `json:"issues,omitempty"`
	Warnings          []OfficialModDiagnostic `json:"warnings,omitempty"`
}

type OfficialModStatus struct {
	GameVersion        string                  `json:"game_version"`
	Platform           string                  `json:"platform"`
	Supported          bool                    `json:"supported"`
	Configured         bool                    `json:"configured"`
	Manageable         bool                    `json:"manageable"`
	InstallDir         string                  `json:"install_dir,omitempty"`
	InstallDirSource   string                  `json:"install_dir_source,omitempty"`
	LauncherPath       string                  `json:"launcher_path,omitempty"`
	SettingsPath       string                  `json:"settings_path,omitempty"`
	SettingsExists     bool                    `json:"settings_exists"`
	SettingsSHA256     string                  `json:"settings_sha256,omitempty"`
	Settings           OfficialModSettings     `json:"settings"`
	ForcedDisabled     bool                    `json:"forced_disabled"`
	LaunchWorkshopRoot string                  `json:"launch_workshop_root,omitempty"`
	Inventory          OfficialModInventory    `json:"inventory"`
	ExistingWorlds     int                     `json:"existing_worlds"`
	SafetyBackupReady  bool                    `json:"safety_backup_ready"`
	SavePath           string                  `json:"save_path,omitempty"`
	StatusDigest       string                  `json:"status_digest"`
	Issues             []OfficialModDiagnostic `json:"issues,omitempty"`
	Warnings           []OfficialModDiagnostic `json:"warnings,omitempty"`

	settingsDocument *palModSettingsDocument
	settingsContent  []byte
	canWrite         bool
}

type OfficialModChangePlan struct {
	Status               OfficialModStatus       `json:"status"`
	DesiredSettings      OfficialModSettings     `json:"desired_settings"`
	TargetInventory      OfficialModInventory    `json:"target_inventory"`
	Changed              bool                    `json:"changed"`
	Changes              []string                `json:"changes,omitempty"`
	SafetyBackupRequired bool                    `json:"safety_backup_required"`
	SafetyBackupReady    bool                    `json:"safety_backup_ready"`
	CanApply             bool                    `json:"can_apply"`
	Issues               []OfficialModDiagnostic `json:"issues,omitempty"`
	Warnings             []OfficialModDiagnostic `json:"warnings,omitempty"`
	PlanDigest           string                  `json:"plan_digest"`
}

type OfficialModApplyOptions struct {
	DesiredSettings    OfficialModSettings
	ExpectedPlanDigest string
	ConfirmServerStop  bool
}

type OfficialModApplyResult struct {
	Plan            OfficialModChangePlan `json:"plan"`
	Status          OfficialModStatus     `json:"status"`
	Changed         bool                  `json:"changed"`
	Created         bool                  `json:"created"`
	RecoveryPath    string                `json:"recovery_path,omitempty"`
	PreviousExists  bool                  `json:"previous_exists"`
	PreviousSHA256  string                `json:"previous_sha256,omitempty"`
	SettingsSHA256  string                `json:"settings_sha256,omitempty"`
	RestartRequired bool                  `json:"restart_required"`
	AppliedAt       time.Time             `json:"applied_at,omitempty"`
	RolledBack      bool                  `json:"rolled_back"`
	RollbackAt      *time.Time            `json:"rollback_at,omitempty"`
}

type palModSettingsDocument struct {
	lines           []string
	newline         string
	hadBOM          bool
	hadFinalNewline bool
	sectionFound    bool
	warnings        []OfficialModDiagnostic
	settings        OfficialModSettings
}

type officialModInfo struct {
	ModName      string                   `json:"ModName"`
	PackageName  string                   `json:"PackageName"`
	Thumbnail    string                   `json:"Thumbnail"`
	Version      string                   `json:"Version"`
	DebugMode    bool                     `json:"DebugMode"`
	MinRevision  *int                     `json:"MinRevision"`
	Author       string                   `json:"Author"`
	Dependencies []string                 `json:"Dependencies"`
	Tags         []string                 `json:"Tags"`
	InstallRule  []officialModInstallRule `json:"InstallRule"`
}

type officialModInstallRule struct {
	Type     string   `json:"Type"`
	IsServer *bool    `json:"IsServer"`
	Targets  []string `json:"Targets"`
}

type officialModLaunchSettings struct {
	forcedDisabled bool
	workshopRoot   string
	warnings       []OfficialModDiagnostic
}

func InspectOfficialMods() OfficialModStatus {
	return inspectOfficialMods("")
}

func inspectOfficialMods(workshopRootOverride string) OfficialModStatus {
	status := OfficialModStatus{
		GameVersion: OfficialModGameVersion,
		Platform:    officialModRuntimeOS,
		Supported:   officialModRuntimeOS == "windows",
		Settings: OfficialModSettings{
			ActiveModList: make([]string, 0),
		},
		Inventory: OfficialModInventory{Packages: make([]OfficialModPackage, 0)},
	}
	if !status.Supported {
		status.Issues = append(status.Issues, modDiagnostic(
			"platform_unsupported",
			ErrOfficialModsUnsupported.Error(),
		))
	}

	installDir, source, err := configuredOfficialModInstallDir()
	status.InstallDir = installDir
	status.InstallDirSource = source
	status.Configured = installDir != ""
	if err != nil {
		status.Issues = append(status.Issues, modDiagnostic("install_dir_invalid", err.Error()))
	}
	if installDir == "" {
		status.Issues = append(status.Issues, modDiagnostic(
			"install_dir_not_configured",
			"configure mods.install_dir or steamcmd.install_dir with the Palworld Dedicated Server installation",
		))
		status.StatusDigest = officialModStatusDigest(status)
		return status
	}

	status.LauncherPath = filepath.Join(installDir, "PalServer.exe")
	status.SettingsPath = filepath.Join(installDir, "Mods", "PalModSettings.ini")
	if err := inspectOfficialModInstallDir(installDir); err != nil {
		status.Issues = append(status.Issues, modDiagnostic("install_dir_invalid", err.Error()))
	} else {
		status.canWrite = status.Supported
	}

	document, content, exists, err := readPalModSettings(status.SettingsPath)
	status.SettingsExists = exists
	if err != nil {
		status.Issues = append(status.Issues, modDiagnostic("settings_invalid", err.Error()))
		status.canWrite = false
	} else {
		status.settingsDocument = document
		status.settingsContent = content
		status.Settings = document.settings
		status.Warnings = append(status.Warnings, document.warnings...)
		if exists {
			status.SettingsSHA256 = digestBytes(content)
		} else {
			status.Warnings = append(status.Warnings, modDiagnostic(
				"settings_missing",
				"PalModSettings.ini does not exist yet and will be created by the first confirmed change",
			))
		}
	}

	launch := parseOfficialModLaunchArguments(viper.GetStringSlice("palworld.control.arguments"))
	status.ForcedDisabled = launch.forcedDisabled
	status.LaunchWorkshopRoot = launch.workshopRoot
	status.Warnings = append(status.Warnings, launch.warnings...)
	if status.ForcedDisabled {
		status.Warnings = append(status.Warnings, modDiagnostic(
			"mods_forced_disabled",
			"the configured -NoMods launch argument forcibly disables all mods",
		))
	}

	rootSetting := status.Settings.WorkshopRootDir
	if strings.TrimSpace(workshopRootOverride) != "" || workshopRootOverride == "__default__" {
		if workshopRootOverride == "__default__" {
			rootSetting = ""
		} else {
			rootSetting = strings.TrimSpace(workshopRootOverride)
		}
	}
	root, rootSource, rootAvailable, rootIssues, rootWarnings := resolveOfficialModWorkshopRoot(
		installDir,
		rootSetting,
		launch.workshopRoot,
	)
	status.Inventory.WorkshopRoot = root
	status.Inventory.WorkshopSource = rootSource
	status.Inventory.WorkshopAvailable = rootAvailable
	status.Inventory.Issues = append(status.Inventory.Issues, rootIssues...)
	status.Inventory.Warnings = append(status.Inventory.Warnings, rootWarnings...)
	if rootAvailable {
		inventory := scanOfficialModInventory(
			installDir,
			root,
			rootSource,
			status.Settings.GlobalEnabled,
			status.Settings.ActiveModList,
			status.ForcedDisabled,
		)
		status.Inventory = inventory
	}

	status.ExistingWorlds, err = countInstalledWorlds(installDir)
	if err != nil {
		status.Warnings = append(status.Warnings, modDiagnostic("world_scan_failed", err.Error()))
	}
	configuredSave := strings.TrimSpace(viper.GetString("save.path"))
	if configuredSave != "" && configuredSave != "/path/to/your/Pal/Saved" && !isRemoteSaveSource(configuredSave) {
		if absolute, absErr := filepath.Abs(configuredSave); absErr == nil {
			status.SavePath = filepath.Clean(absolute)
		}
		if levelPath, saveErr := inspectSteamSafetyBackupSource(configuredSave); saveErr == nil {
			installedSaveRoot := filepath.Join(installDir, "Pal", "Saved", "SaveGames", "0")
			status.SafetyBackupReady = pathWithin(levelPath, installedSaveRoot)
			if status.ExistingWorlds > 0 && !status.SafetyBackupReady {
				status.Warnings = append(status.Warnings, modDiagnostic(
					"save_outside_install_dir",
					"save.path does not point to a world inside the configured mod installation",
				))
			}
		} else if status.ExistingWorlds > 0 {
			status.Warnings = append(status.Warnings, modDiagnostic(
				"save_backup_unavailable",
				"save.path cannot create the required pre-change safety backup: "+saveErr.Error(),
			))
		}
	}
	if status.ExistingWorlds == 0 {
		status.SafetyBackupReady = true
	}

	status.Issues = uniqueModDiagnostics(status.Issues)
	status.Warnings = uniqueModDiagnostics(status.Warnings)
	status.Inventory.Issues = uniqueModDiagnostics(status.Inventory.Issues)
	status.Inventory.Warnings = uniqueModDiagnostics(status.Inventory.Warnings)
	status.Manageable = status.Configured && status.Supported && status.canWrite
	status.StatusDigest = officialModStatusDigest(status)
	return status
}

func PlanOfficialModSettings(desired OfficialModSettings) OfficialModChangePlan {
	status := InspectOfficialMods()
	plan := OfficialModChangePlan{
		Status:            status,
		DesiredSettings:   desired,
		SafetyBackupReady: status.SafetyBackupReady,
	}
	if !status.Configured {
		plan.Issues = append(plan.Issues, modDiagnostic("install_dir_not_configured", ErrOfficialModsNotConfigured.Error()))
	}
	if !status.Supported {
		plan.Issues = append(plan.Issues, modDiagnostic("platform_unsupported", ErrOfficialModsUnsupported.Error()))
	}
	if !status.canWrite {
		plan.Issues = append(plan.Issues, status.Issues...)
		plan.Issues = append(plan.Issues, status.Inventory.Issues...)
	}

	normalizedRoot, rootErr := normalizeOfficialModRootSetting(desired.WorkshopRootDir)
	if rootErr != nil {
		plan.Issues = append(plan.Issues, modDiagnostic("workshop_root_invalid", rootErr.Error()))
	}
	plan.DesiredSettings.WorkshopRootDir = normalizedRoot

	if status.LaunchWorkshopRoot != "" && !sameFilesystemPath(
		cleanOptionalPath(normalizedRoot),
		cleanOptionalPath(status.Settings.WorkshopRootDir),
	) {
		plan.Issues = append(plan.Issues, modDiagnostic(
			"workshop_root_overridden",
			"the -workshopdir launch argument overrides PalModSettings.ini; remove it before changing WorkshopRootDir",
		))
	}

	targetStatus := inspectOfficialMods(workshopRootOverrideSentinel(normalizedRoot))
	plan.TargetInventory = targetStatus.Inventory
	normalizedActive, activeIssues := normalizeOfficialModActiveList(desired.ActiveModList, targetStatus.Inventory.Packages)
	plan.DesiredSettings.ActiveModList = normalizedActive
	applyOfficialModSelectionToInventory(
		&plan.TargetInventory,
		plan.DesiredSettings.GlobalEnabled,
		normalizedActive,
		targetStatus.ForcedDisabled,
	)
	plan.Issues = append(plan.Issues, activeIssues...)
	plan.Issues = append(plan.Issues, validateOfficialModDependencies(normalizedActive, targetStatus.Inventory.Packages)...)
	plan.Issues = append(plan.Issues, blockingOfficialModRootIssues(targetStatus.Inventory.Issues)...)
	if len(normalizedActive) > 0 {
		plan.Issues = append(plan.Issues, targetStatus.Inventory.Issues...)
	}
	if len(normalizedActive) > 0 && !targetStatus.Inventory.WorkshopAvailable {
		plan.Issues = append(plan.Issues, modDiagnostic(
			"workshop_unavailable",
			"the selected Workshop directory is unavailable",
		))
	}
	if targetStatus.ForcedDisabled && plan.DesiredSettings.GlobalEnabled && len(normalizedActive) > 0 {
		plan.Warnings = append(plan.Warnings, modDiagnostic(
			"mods_forced_disabled",
			"the selected mods will remain disabled while -NoMods is present",
		))
	}
	plan.Warnings = append(plan.Warnings, targetStatus.Inventory.Warnings...)

	if status.Settings.GlobalEnabled != plan.DesiredSettings.GlobalEnabled {
		plan.Changes = append(plan.Changes, "global_enabled")
	}
	if !sameFilesystemPath(cleanOptionalPath(status.Settings.WorkshopRootDir), cleanOptionalPath(normalizedRoot)) {
		plan.Changes = append(plan.Changes, "workshop_root_dir")
	}
	if !equalFoldedStringSlices(status.Settings.ActiveModList, normalizedActive) {
		plan.Changes = append(plan.Changes, "active_mod_list")
	}
	plan.Changed = len(plan.Changes) > 0
	plan.SafetyBackupRequired = plan.Changed && status.ExistingWorlds > 0
	if plan.SafetyBackupRequired && !plan.SafetyBackupReady {
		plan.Issues = append(plan.Issues, modDiagnostic(
			"save_backup_required",
			"configure a local save.path inside this Palworld installation before changing mods for an existing world",
		))
	}
	if plan.Changed {
		plan.Warnings = append(plan.Warnings, modDiagnostic(
			"mods_can_damage_saves",
			"server-side mods can crash the server or corrupt save data; review every selected package before restarting",
		))
	}
	plan.Issues = uniqueModDiagnostics(plan.Issues)
	plan.Warnings = uniqueModDiagnostics(plan.Warnings)
	plan.CanApply = status.Configured && status.Supported && status.canWrite && len(plan.Issues) == 0
	plan.PlanDigest = officialModPlanDigest(plan)
	return plan
}

func ApplyOfficialModSettings(ctx context.Context, options OfficialModApplyOptions) (OfficialModApplyResult, error) {
	officialModMu.Lock()
	defer officialModMu.Unlock()

	plan := PlanOfficialModSettings(options.DesiredSettings)
	result := OfficialModApplyResult{Plan: plan}
	if !plan.Status.Configured {
		return result, ErrOfficialModsNotConfigured
	}
	if !plan.Status.Supported {
		return result, ErrOfficialModsUnsupported
	}
	if !plan.CanApply {
		return result, fmt.Errorf("%w: %s", ErrOfficialModsInvalid, joinModDiagnosticMessages(plan.Issues))
	}
	if strings.TrimSpace(options.ExpectedPlanDigest) == "" ||
		!strings.EqualFold(options.ExpectedPlanDigest, plan.PlanDigest) {
		return result, ErrOfficialModPlanChanged
	}
	if !plan.Changed {
		result.Status = plan.Status
		return result, nil
	}
	if err := ConfirmGameServerStopped(ctx, options.ConfirmServerStop); err != nil {
		return result, err
	}

	settingsPath := plan.Status.SettingsPath
	modsDir := filepath.Dir(settingsPath)
	if err := ensureOfficialModsDirectory(plan.Status.InstallDir, modsDir); err != nil {
		return result, fmt.Errorf("%w: prepare Mods directory: %v", ErrOfficialModApplyFailed, err)
	}

	document := plan.Status.settingsDocument
	if document == nil {
		return result, fmt.Errorf("%w: settings document is unavailable", ErrOfficialModApplyFailed)
	}
	next, err := renderPalModSettings(document, plan.DesiredSettings)
	if err != nil {
		return result, fmt.Errorf("%w: render PalModSettings.ini: %v", ErrOfficialModApplyFailed, err)
	}
	if _, err := parsePalModSettings(next); err != nil {
		return result, fmt.Errorf("%w: validate staged PalModSettings.ini: %v", ErrOfficialModApplyFailed, err)
	}

	currentContent, currentExists, err := verifyCurrentPalModSettings(plan.Status)
	if err != nil {
		return result, err
	}
	recoveryPath, err := createPalModSettingsRecovery(settingsPath, currentContent, currentExists)
	if err != nil {
		return result, fmt.Errorf("%w: create settings recovery point: %v", ErrOfficialModApplyFailed, err)
	}
	result.RecoveryPath = recoveryPath
	result.PreviousExists = currentExists
	if currentExists {
		result.PreviousSHA256 = digestBytes(currentContent)
	}

	if err := installPalModSettings(settingsPath, next, currentExists); err != nil {
		return result, fmt.Errorf("%w: %v", ErrOfficialModApplyFailed, err)
	}
	result.SettingsSHA256 = digestBytes(next)
	result.Changed = true
	result.Created = !currentExists
	result.RestartRequired = true
	result.AppliedAt = officialModNow().UTC()

	after := InspectOfficialMods()
	result.Status = after
	if err := verifyAppliedOfficialModSettings(after, plan.DesiredSettings, result.SettingsSHA256); err != nil {
		rollbackErr := rollbackOfficialModSettingsLocked(ctx, &result, true)
		if rollbackErr != nil {
			return result, fmt.Errorf(
				"%w: verify updated settings: %v; rollback failed: %v; recovery point: %s",
				ErrOfficialModApplyFailed,
				err,
				rollbackErr,
				recoveryPath,
			)
		}
		return result, fmt.Errorf("%w: verify updated settings: %v; previous settings restored", ErrOfficialModApplyFailed, err)
	}
	return result, nil
}

func RollbackOfficialModSettings(ctx context.Context, result *OfficialModApplyResult, confirmServerStop bool) error {
	officialModMu.Lock()
	defer officialModMu.Unlock()
	return rollbackOfficialModSettingsLocked(ctx, result, confirmServerStop)
}

func rollbackOfficialModSettingsLocked(ctx context.Context, result *OfficialModApplyResult, confirmServerStop bool) error {
	if result == nil || !result.Changed || strings.TrimSpace(result.RecoveryPath) == "" {
		return errors.New("official mod rollback requires an applied settings recovery point")
	}
	if err := ConfirmGameServerStopped(ctx, confirmServerStop); err != nil {
		return err
	}
	settingsPath := result.Plan.Status.SettingsPath
	current, exists, err := readPalModSettingsRaw(settingsPath)
	if err != nil {
		return fmt.Errorf("%w: inspect current settings: %v", ErrOfficialModRollbackFailed, err)
	}
	if !exists || !strings.EqualFold(digestBytes(current), result.SettingsSHA256) {
		return fmt.Errorf("%w: PalModSettings.ini changed after the managed update", ErrOfficialModRollbackFailed)
	}

	if result.PreviousExists {
		recovery, recoveryExists, err := readPalModSettingsRaw(result.RecoveryPath)
		if err != nil || !recoveryExists {
			if err == nil {
				err = errors.New("recovery file is missing")
			}
			return fmt.Errorf("%w: read recovery file: %v", ErrOfficialModRollbackFailed, err)
		}
		if !strings.EqualFold(digestBytes(recovery), result.PreviousSHA256) {
			return fmt.Errorf("%w: recovery file digest changed", ErrOfficialModRollbackFailed)
		}
		if _, err := parsePalModSettings(recovery); err != nil {
			return fmt.Errorf("%w: recovery file is invalid: %v", ErrOfficialModRollbackFailed, err)
		}
		if err := installPalModSettings(settingsPath, recovery, true); err != nil {
			return fmt.Errorf("%w: restore previous settings: %v", ErrOfficialModRollbackFailed, err)
		}
	} else {
		rollbackPath := filepath.Join(
			filepath.Dir(settingsPath),
			".pst-PalModSettings-rollback-"+uuid.NewString()+".ini",
		)
		if err := os.Rename(settingsPath, rollbackPath); err != nil {
			return fmt.Errorf("%w: restore absent settings state: %v", ErrOfficialModRollbackFailed, err)
		}
		_ = officialModRemoveFile(rollbackPath)
	}

	restored, restoredExists, err := readPalModSettingsRaw(settingsPath)
	if err != nil {
		return fmt.Errorf("%w: verify rollback: %v", ErrOfficialModRollbackFailed, err)
	}
	if restoredExists != result.PreviousExists ||
		(restoredExists && !strings.EqualFold(digestBytes(restored), result.PreviousSHA256)) {
		return fmt.Errorf("%w: restored settings do not match the recovery point", ErrOfficialModRollbackFailed)
	}
	now := officialModNow().UTC()
	result.RolledBack = true
	result.RollbackAt = &now
	result.Status = InspectOfficialMods()
	return nil
}

func configuredOfficialModInstallDir() (string, string, error) {
	path := strings.TrimSpace(viper.GetString("mods.install_dir"))
	source := "mods.install_dir"
	if path == "" {
		path = strings.TrimSpace(viper.GetString("steamcmd.install_dir"))
		source = "steamcmd.install_dir"
	}
	if path == "" {
		return "", "", nil
	}
	if !filepath.IsAbs(path) {
		return filepath.Clean(path), source, errors.New("official mod install directory must be an absolute path")
	}
	absolute, err := filepath.Abs(path)
	if err != nil {
		return "", source, fmt.Errorf("resolve official mod install directory: %w", err)
	}
	return filepath.Clean(absolute), source, nil
}

func inspectOfficialModInstallDir(installDir string) error {
	info, err := os.Lstat(installDir)
	if err != nil {
		return fmt.Errorf("inspect official mod install directory: %w", err)
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
		return errors.New("official mod install directory must be a real directory and not a symbolic link")
	}
	if err := rejectSymlinkComponents(installDir); err != nil {
		return fmt.Errorf("official mod install directory: %w", err)
	}
	launcher := filepath.Join(installDir, "PalServer.exe")
	launcherInfo, err := os.Lstat(launcher)
	if err != nil {
		return fmt.Errorf("PalServer.exe is missing: %w", err)
	}
	if launcherInfo.Mode()&os.ModeSymlink != 0 || !launcherInfo.Mode().IsRegular() || launcherInfo.Size() <= 0 {
		return errors.New("PalServer.exe must be a non-empty regular file and not a symbolic link")
	}
	modsDir := filepath.Join(installDir, "Mods")
	modsInfo, err := os.Lstat(modsDir)
	if err == nil {
		if modsInfo.Mode()&os.ModeSymlink != 0 || !modsInfo.IsDir() {
			return errors.New("Mods must be a real directory and not a symbolic link")
		}
		if err := rejectSymlinkComponents(modsDir); err != nil {
			return fmt.Errorf("Mods directory: %w", err)
		}
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("inspect Mods directory: %w", err)
	}
	return nil
}

func ensureOfficialModsDirectory(installDir, modsDir string) error {
	if err := inspectOfficialModInstallDir(installDir); err != nil {
		return err
	}
	info, err := os.Lstat(modsDir)
	if os.IsNotExist(err) {
		if err := os.Mkdir(modsDir, 0o755); err != nil {
			return err
		}
		info, err = os.Lstat(modsDir)
	}
	if err != nil {
		return err
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
		return errors.New("Mods must be a real directory and not a symbolic link")
	}
	return rejectSymlinkComponents(modsDir)
}

func readPalModSettings(path string) (*palModSettingsDocument, []byte, bool, error) {
	content, exists, err := readPalModSettingsRaw(path)
	if err != nil {
		return nil, nil, false, err
	}
	if !exists {
		document, parseErr := parsePalModSettings(nil)
		return document, nil, false, parseErr
	}
	document, err := parsePalModSettings(content)
	return document, content, true, err
}

func readPalModSettingsRaw(path string) ([]byte, bool, error) {
	content, _, exists, err := readRegularFileLimited(path, maxPalModSettingsSize)
	return content, exists, err
}

func parsePalModSettings(content []byte) (*palModSettingsDocument, error) {
	if len(content) > maxPalModSettingsSize {
		return nil, fmt.Errorf("PalModSettings.ini exceeds %d bytes", maxPalModSettingsSize)
	}
	if !utf8.Valid(content) || bytes.ContainsRune(content, '\x00') {
		return nil, errors.New("PalModSettings.ini must be valid UTF-8 text")
	}
	document := &palModSettingsDocument{
		newline:  "\n",
		settings: OfficialModSettings{ActiveModList: make([]string, 0)},
	}
	text := string(content)
	if strings.HasPrefix(text, "\ufeff") {
		document.hadBOM = true
		text = strings.TrimPrefix(text, "\ufeff")
	}
	if strings.Contains(text, "\r\n") {
		document.newline = "\r\n"
	}
	document.hadFinalNewline = strings.HasSuffix(text, "\n")
	text = strings.ReplaceAll(text, "\r\n", "\n")
	if text != "" {
		document.lines = strings.Split(text, "\n")
	}
	if document.hadFinalNewline && len(document.lines) > 0 {
		document.lines = document.lines[:len(document.lines)-1]
	}

	inTarget := false
	sectionCount := 0
	globalCount := 0
	rootCount := 0
	for _, line := range document.lines {
		trimmed := strings.TrimSpace(strings.TrimSuffix(line, "\r"))
		if section, ok := parseINISection(trimmed); ok {
			inTarget = strings.EqualFold(section, "PalModSettings")
			if inTarget {
				sectionCount++
				document.sectionFound = true
			}
			continue
		}
		if !inTarget || trimmed == "" || strings.HasPrefix(trimmed, ";") || strings.HasPrefix(trimmed, "#") {
			continue
		}
		key, value, ok := parseINIKeyValue(trimmed)
		if !ok {
			continue
		}
		switch {
		case strings.EqualFold(key, "bGlobalEnableMod"):
			globalCount++
			switch {
			case strings.EqualFold(value, "true"):
				document.settings.GlobalEnabled = true
			case strings.EqualFold(value, "false"):
				document.settings.GlobalEnabled = false
			default:
				return nil, errors.New("bGlobalEnableMod must be True or False")
			}
		case strings.EqualFold(key, "WorkshopRootDir"):
			rootCount++
			document.settings.WorkshopRootDir = unquoteOptionalPath(value)
		case strings.EqualFold(key, "ActiveModList"):
			value = strings.TrimSpace(value)
			if value == "" {
				document.warnings = append(document.warnings, modDiagnostic(
					"empty_active_mod",
					"an empty ActiveModList entry was ignored",
				))
				continue
			}
			document.settings.ActiveModList = append(document.settings.ActiveModList, value)
		}
	}
	if sectionCount > 1 {
		return nil, errors.New("PalModSettings.ini contains multiple [PalModSettings] sections")
	}
	if sectionCount == 0 && len(content) > 0 {
		document.warnings = append(document.warnings, modDiagnostic(
			"settings_section_missing",
			"PalModSettings.ini is missing [PalModSettings] and the section will be appended on the next change",
		))
	}
	if globalCount > 1 {
		document.warnings = append(document.warnings, modDiagnostic(
			"duplicate_global_setting",
			"multiple bGlobalEnableMod entries were found; the last value is effective",
		))
	}
	if rootCount > 1 {
		document.warnings = append(document.warnings, modDiagnostic(
			"duplicate_workshop_root",
			"multiple WorkshopRootDir entries were found; the last value is effective",
		))
	}
	document.settings.ActiveModList = uniqueFoldedStrings(document.settings.ActiveModList)
	return document, nil
}

func renderPalModSettings(document *palModSettingsDocument, settings OfficialModSettings) ([]byte, error) {
	if document == nil {
		return nil, errors.New("settings document is required")
	}
	knownLines := []string{"bGlobalEnableMod=" + boolINI(settings.GlobalEnabled)}
	if strings.TrimSpace(settings.WorkshopRootDir) != "" {
		knownLines = append(knownLines, "WorkshopRootDir="+settings.WorkshopRootDir)
	}
	for _, packageName := range settings.ActiveModList {
		knownLines = append(knownLines, "ActiveModList="+packageName)
	}

	out := make([]string, 0, len(document.lines)+len(knownLines)+2)
	inTarget := false
	inserted := false
	for _, line := range document.lines {
		trimmed := strings.TrimSpace(strings.TrimSuffix(line, "\r"))
		if section, ok := parseINISection(trimmed); ok {
			inTarget = strings.EqualFold(section, "PalModSettings")
			out = append(out, line)
			if inTarget {
				out = append(out, knownLines...)
				inserted = true
			}
			continue
		}
		if inTarget && isKnownPalModSettingLine(trimmed) {
			continue
		}
		out = append(out, line)
	}
	if !inserted {
		if len(out) > 0 && strings.TrimSpace(out[len(out)-1]) != "" {
			out = append(out, "")
		}
		out = append(out, "[PalModSettings]")
		out = append(out, knownLines...)
	}
	newline := document.newline
	if newline == "" {
		newline = "\n"
	}
	text := strings.Join(out, newline) + newline
	if document.hadBOM {
		text = "\ufeff" + text
	}
	content := []byte(text)
	if len(content) > maxPalModSettingsSize {
		return nil, fmt.Errorf("PalModSettings.ini exceeds %d bytes", maxPalModSettingsSize)
	}
	parsed, err := parsePalModSettings(content)
	if err != nil {
		return nil, err
	}
	if parsed.settings.GlobalEnabled != settings.GlobalEnabled ||
		!sameFilesystemPath(cleanOptionalPath(parsed.settings.WorkshopRootDir), cleanOptionalPath(settings.WorkshopRootDir)) ||
		!equalFoldedStringSlices(parsed.settings.ActiveModList, settings.ActiveModList) {
		return nil, errors.New("rendered PalModSettings.ini does not preserve the requested settings")
	}
	return content, nil
}

func resolveOfficialModWorkshopRoot(installDir, configuredRoot, launchRoot string) (
	string,
	string,
	bool,
	[]OfficialModDiagnostic,
	[]OfficialModDiagnostic,
) {
	root := strings.TrimSpace(configuredRoot)
	source := "PalModSettings.ini"
	if strings.TrimSpace(launchRoot) != "" {
		root = strings.TrimSpace(launchRoot)
		source = "launch_argument"
	} else if root == "" {
		root = filepath.Join(installDir, "Mods", "Workshop")
		source = "default"
	}
	if !filepath.IsAbs(root) {
		return filepath.Clean(root), source, false, []OfficialModDiagnostic{modDiagnostic(
			"workshop_root_not_absolute",
			"WorkshopRootDir must be an absolute path",
		)}, nil
	}
	absolute, err := filepath.Abs(root)
	if err != nil {
		return root, source, false, []OfficialModDiagnostic{modDiagnostic(
			"workshop_root_invalid",
			"resolve WorkshopRootDir: "+err.Error(),
		)}, nil
	}
	root = filepath.Clean(absolute)
	info, err := os.Lstat(root)
	if os.IsNotExist(err) {
		return root, source, false, nil, []OfficialModDiagnostic{modDiagnostic(
			"workshop_root_missing",
			"the Workshop directory does not exist yet",
		)}
	}
	if err != nil {
		return root, source, false, []OfficialModDiagnostic{modDiagnostic(
			"workshop_root_unreadable",
			"inspect WorkshopRootDir: "+err.Error(),
		)}, nil
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
		return root, source, false, []OfficialModDiagnostic{modDiagnostic(
			"workshop_root_unsafe",
			"WorkshopRootDir must be a real directory and not a symbolic link",
		)}, nil
	}
	if err := rejectSymlinkComponents(root); err != nil {
		return root, source, false, []OfficialModDiagnostic{modDiagnostic(
			"workshop_root_unsafe",
			"WorkshopRootDir: "+err.Error(),
		)}, nil
	}
	return root, source, true, nil, nil
}

func scanOfficialModInventory(
	installDir,
	root,
	source string,
	globalEnabled bool,
	active []string,
	forcedDisabled bool,
) OfficialModInventory {
	inventory := OfficialModInventory{
		WorkshopRoot:      root,
		WorkshopSource:    source,
		WorkshopAvailable: true,
		Packages:          make([]OfficialModPackage, 0),
	}
	entries, err := os.ReadDir(root)
	if err != nil {
		inventory.WorkshopAvailable = false
		inventory.Issues = append(inventory.Issues, modDiagnostic(
			"workshop_scan_failed",
			"read WorkshopRootDir: "+err.Error(),
		))
		return inventory
	}
	if len(entries) > maxPalModPackages {
		inventory.Issues = append(inventory.Issues, modDiagnostic(
			"too_many_packages",
			fmt.Sprintf("WorkshopRootDir contains more than %d entries", maxPalModPackages),
		))
		entries = entries[:maxPalModPackages]
	}

	for _, entry := range entries {
		if !entry.IsDir() && entry.Type()&os.ModeSymlink == 0 {
			continue
		}
		packageEntry := inspectOfficialModPackage(installDir, root, entry)
		inventory.Packages = append(inventory.Packages, packageEntry)
	}
	markDuplicateOfficialModPackages(inventory.Packages)

	activeSet := foldedStringSet(active)
	packageIndex := make(map[string][]int, len(inventory.Packages))
	for index := range inventory.Packages {
		pkg := &inventory.Packages[index]
		if pkg.PackageName != "" {
			key := strings.ToLower(pkg.PackageName)
			packageIndex[key] = append(packageIndex[key], index)
		}
		pkg.Listed = activeSet[strings.ToLower(pkg.PackageName)]
		pkg.EffectiveEnabled = globalEnabled && !forcedDisabled && pkg.Listed
		pkg.PendingRestart = pkg.EffectiveEnabled && !pkg.Deployed
		pkg.PendingRemoval = !pkg.EffectiveEnabled && pkg.Deployed
	}
	for _, activeName := range active {
		if len(packageIndex[strings.ToLower(activeName)]) == 0 {
			inventory.UnknownActiveMods = append(inventory.UnknownActiveMods, activeName)
		}
	}
	for index := range inventory.Packages {
		pkg := &inventory.Packages[index]
		if !pkg.Listed {
			continue
		}
		if !pkg.Valid {
			pkg.Issues = append(pkg.Issues, packageDiagnostic(
				"active_package_invalid",
				"this active package has invalid metadata",
				*pkg,
			))
		}
		if !pkg.ServerCompatible {
			pkg.Issues = append(pkg.Issues, packageDiagnostic(
				"server_rule_missing",
				"Info.json has no InstallRule with IsServer set to true",
				*pkg,
			))
		}
		for _, dependency := range pkg.Dependencies {
			indices := packageIndex[strings.ToLower(dependency)]
			if len(indices) == 0 {
				pkg.Issues = append(pkg.Issues, OfficialModDiagnostic{
					Code:        "dependency_missing",
					Message:     "required dependency is not present in the Workshop directory",
					FolderName:  pkg.FolderName,
					PackageName: pkg.PackageName,
					Dependency:  dependency,
				})
			} else if !activeSet[strings.ToLower(dependency)] {
				pkg.Issues = append(pkg.Issues, OfficialModDiagnostic{
					Code:        "dependency_disabled",
					Message:     "required dependency is not active",
					FolderName:  pkg.FolderName,
					PackageName: pkg.PackageName,
					Dependency:  dependency,
				})
			}
		}
		pkg.Issues = uniqueModDiagnostics(pkg.Issues)
		pkg.Warnings = uniqueModDiagnostics(pkg.Warnings)
	}
	sort.SliceStable(inventory.Packages, func(left, right int) bool {
		leftPackage := inventory.Packages[left]
		rightPackage := inventory.Packages[right]
		if leftPackage.Listed != rightPackage.Listed {
			return leftPackage.Listed
		}
		leftName := leftPackage.PackageName
		if leftName == "" {
			leftName = leftPackage.FolderName
		}
		rightName := rightPackage.PackageName
		if rightName == "" {
			rightName = rightPackage.FolderName
		}
		return strings.ToLower(leftName) < strings.ToLower(rightName)
	})
	inventory.UnknownActiveMods = uniqueFoldedStrings(inventory.UnknownActiveMods)
	return inventory
}

func inspectOfficialModPackage(installDir, root string, entry os.DirEntry) OfficialModPackage {
	path := filepath.Join(root, entry.Name())
	pkg := OfficialModPackage{
		FolderName: entry.Name(),
		Path:       path,
	}
	if isDecimalIdentifier(entry.Name()) {
		pkg.WorkshopItemID = entry.Name()
	}
	info, err := os.Lstat(path)
	if err != nil {
		pkg.Issues = append(pkg.Issues, packageDiagnostic("package_unreadable", err.Error(), pkg))
		return pkg
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
		pkg.Issues = append(pkg.Issues, packageDiagnostic(
			"package_unsafe",
			"Workshop package must be a real directory and not a symbolic link",
			pkg,
		))
		return pkg
	}
	if err := rejectSymlinkComponents(path); err != nil {
		pkg.Issues = append(pkg.Issues, packageDiagnostic("package_unsafe", err.Error(), pkg))
		return pkg
	}
	infoPath := filepath.Join(path, "Info.json")
	pkg.InfoPath = infoPath
	content, _, exists, err := readRegularFileLimited(infoPath, maxPalModInfoSize)
	if err != nil {
		pkg.Issues = append(pkg.Issues, packageDiagnostic("info_invalid", err.Error(), pkg))
		return pkg
	}
	if !exists {
		pkg.Issues = append(pkg.Issues, packageDiagnostic(
			"info_missing",
			"Info.json is missing directly under the package folder",
			pkg,
		))
		return pkg
	}
	pkg.InfoSHA256 = digestBytes(content)
	metadata, err := decodeOfficialModInfo(content)
	if err != nil {
		pkg.Issues = append(pkg.Issues, packageDiagnostic("info_invalid", err.Error(), pkg))
		return pkg
	}
	pkg.ModName = strings.TrimSpace(metadata.ModName)
	pkg.PackageName = strings.TrimSpace(metadata.PackageName)
	pkg.Version = strings.TrimSpace(metadata.Version)
	pkg.Author = strings.TrimSpace(metadata.Author)
	pkg.Thumbnail = strings.TrimSpace(metadata.Thumbnail)
	pkg.MinRevision = metadata.MinRevision
	pkg.DebugMode = metadata.DebugMode
	pkg.Dependencies = uniqueFoldedStrings(trimmedStrings(metadata.Dependencies))
	pkg.Tags = uniqueFoldedStrings(trimmedStrings(metadata.Tags))
	for _, rule := range metadata.InstallRule {
		kind := strings.TrimSpace(rule.Type)
		if kind == "" {
			continue
		}
		pkg.InstallTypes = appendUniqueFolded(pkg.InstallTypes, kind)
		if rule.IsServer != nil && *rule.IsServer && officialModInstallTypeSupported(kind) {
			pkg.ServerCompatible = true
			pkg.ServerInstallTypes = appendUniqueFolded(pkg.ServerInstallTypes, kind)
		}
		if !officialModInstallTypeSupported(kind) {
			pkg.Warnings = append(pkg.Warnings, packageDiagnostic(
				"install_type_unknown",
				"InstallRule.Type is not one of the five types documented for Palworld 1.0.0",
				pkg,
			))
		}
	}
	if err := validateOfficialModMetadata(metadata); err != nil {
		pkg.ServerCompatible = false
		pkg.ServerInstallTypes = nil
		pkg.Issues = append(pkg.Issues, packageDiagnostic("metadata_invalid", err.Error(), pkg))
		return pkg
	}
	pkg.Valid = true
	if !pkg.ServerCompatible {
		pkg.Warnings = append(pkg.Warnings, packageDiagnostic(
			"server_rule_missing",
			"Info.json has no InstallRule with IsServer set to true",
			pkg,
		))
	}
	if pkg.DebugMode {
		pkg.Warnings = append(pkg.Warnings, packageDiagnostic(
			"debug_mode_enabled",
			"DebugMode reinstalls this package on every server launch",
			pkg,
		))
	}
	if validOfficialModIdentifier(pkg.PackageName) {
		manifestPath := filepath.Join(installDir, "Mods", "ManagedMods", pkg.PackageName, "InstallManifest.json")
		if pathWithin(manifestPath, filepath.Join(installDir, "Mods", "ManagedMods")) {
			manifestInfo, manifestErr := os.Lstat(manifestPath)
			if manifestErr == nil && manifestInfo.Mode()&os.ModeSymlink == 0 && manifestInfo.Mode().IsRegular() {
				pkg.Deployed = true
			} else if manifestErr != nil && !os.IsNotExist(manifestErr) {
				pkg.Warnings = append(pkg.Warnings, packageDiagnostic(
					"deployment_status_unavailable",
					"cannot inspect ManagedMods deployment manifest: "+manifestErr.Error(),
					pkg,
				))
			}
		}
	}
	return pkg
}

func decodeOfficialModInfo(content []byte) (officialModInfo, error) {
	if len(content) == 0 {
		return officialModInfo{}, errors.New("Info.json cannot be empty")
	}
	if !utf8.Valid(content) || bytes.ContainsRune(content, '\x00') {
		return officialModInfo{}, errors.New("Info.json must be valid UTF-8 JSON")
	}
	decoder := json.NewDecoder(bytes.NewReader(content))
	var metadata officialModInfo
	if err := decoder.Decode(&metadata); err != nil {
		return officialModInfo{}, fmt.Errorf("decode Info.json: %w", err)
	}
	var extra any
	if err := decoder.Decode(&extra); !errors.Is(err, io.EOF) {
		if err == nil {
			return officialModInfo{}, errors.New("Info.json contains multiple JSON values")
		}
		return officialModInfo{}, fmt.Errorf("decode trailing Info.json data: %w", err)
	}
	return metadata, nil
}

func validateOfficialModMetadata(metadata officialModInfo) error {
	if strings.TrimSpace(metadata.ModName) == "" {
		return errors.New("ModName is required")
	}
	if len(metadata.ModName) > maxPalModMetadataString {
		return errors.New("ModName is too long")
	}
	if !validOfficialModIdentifier(metadata.PackageName) {
		return errors.New("PackageName contains unsupported characters or length")
	}
	for name, value := range map[string]string{
		"Thumbnail": metadata.Thumbnail,
		"Version":   metadata.Version,
		"Author":    metadata.Author,
	} {
		if len(value) > maxPalModMetadataString || strings.ContainsAny(value, "\x00\r\n") {
			return fmt.Errorf("%s contains unsupported characters or length", name)
		}
	}
	if metadata.MinRevision != nil && *metadata.MinRevision < 0 {
		return errors.New("MinRevision cannot be negative")
	}
	if len(metadata.Dependencies) > maxPalModMetadataListEntries ||
		len(metadata.Tags) > maxPalModMetadataListEntries ||
		len(metadata.InstallRule) > maxPalModMetadataListEntries {
		return errors.New("Info.json contains too many list entries")
	}
	for _, dependency := range metadata.Dependencies {
		if !validOfficialModIdentifier(dependency) {
			return fmt.Errorf("dependency %q contains unsupported characters or length", dependency)
		}
		if strings.EqualFold(strings.TrimSpace(dependency), strings.TrimSpace(metadata.PackageName)) {
			return fmt.Errorf("dependency %q cannot reference the package itself", dependency)
		}
	}
	for _, rule := range metadata.InstallRule {
		if len(rule.Type) > 64 || strings.ContainsAny(rule.Type, "\x00\r\n") {
			return errors.New("InstallRule.Type contains unsupported characters or length")
		}
		if rule.IsServer != nil && *rule.IsServer && len(rule.Targets) == 0 {
			return errors.New("server InstallRule must contain at least one Target")
		}
		if len(rule.Targets) > maxPalModMetadataListEntries {
			return errors.New("InstallRule contains too many Targets")
		}
		for _, target := range rule.Targets {
			if len(target) > maxPalModMetadataString || strings.ContainsAny(target, "\x00\r\n") {
				return errors.New("InstallRule.Targets contains unsupported characters or length")
			}
			if !validOfficialModTarget(target) {
				return fmt.Errorf("InstallRule target %q must remain inside the package", target)
			}
		}
	}
	return nil
}

func markDuplicateOfficialModPackages(packages []OfficialModPackage) {
	indices := make(map[string][]int)
	for index, pkg := range packages {
		if pkg.PackageName == "" {
			continue
		}
		indices[strings.ToLower(pkg.PackageName)] = append(indices[strings.ToLower(pkg.PackageName)], index)
	}
	for _, duplicates := range indices {
		if len(duplicates) < 2 {
			continue
		}
		for _, index := range duplicates {
			packages[index].Valid = false
			packages[index].Issues = append(packages[index].Issues, packageDiagnostic(
				"duplicate_package_name",
				"multiple Workshop folders use the same PackageName and the official loader order is not guaranteed",
				packages[index],
			))
		}
	}
}

func normalizeOfficialModActiveList(
	active []string,
	packages []OfficialModPackage,
) ([]string, []OfficialModDiagnostic) {
	issues := make([]OfficialModDiagnostic, 0)
	if len(active) > maxPalModActivePackages {
		issues = append(issues, modDiagnostic(
			"too_many_active_mods",
			fmt.Sprintf("ActiveModList cannot contain more than %d packages", maxPalModActivePackages),
		))
		active = active[:maxPalModActivePackages]
	}
	index := make(map[string][]OfficialModPackage)
	for _, pkg := range packages {
		if pkg.PackageName != "" {
			index[strings.ToLower(pkg.PackageName)] = append(index[strings.ToLower(pkg.PackageName)], pkg)
		}
	}
	normalized := make([]string, 0, len(active))
	seen := make(map[string]bool)
	for _, requested := range active {
		requested = strings.TrimSpace(requested)
		if !validOfficialModIdentifier(requested) {
			issues = append(issues, OfficialModDiagnostic{
				Code:        "active_package_name_invalid",
				Message:     "ActiveModList contains an unsupported PackageName",
				PackageName: requested,
			})
			continue
		}
		key := strings.ToLower(requested)
		if seen[key] {
			continue
		}
		seen[key] = true
		matches := index[key]
		if len(matches) == 0 {
			issues = append(issues, OfficialModDiagnostic{
				Code:        "active_package_missing",
				Message:     "the requested PackageName is not present in the selected Workshop directory",
				PackageName: requested,
			})
			normalized = append(normalized, requested)
			continue
		}
		if len(matches) > 1 {
			issues = append(issues, OfficialModDiagnostic{
				Code:        "duplicate_package_name",
				Message:     "multiple Workshop folders use the requested PackageName",
				PackageName: requested,
			})
			normalized = append(normalized, requested)
			continue
		}
		pkg := matches[0]
		normalized = append(normalized, pkg.PackageName)
		if !pkg.Valid {
			issues = append(issues, OfficialModDiagnostic{
				Code:        "active_package_invalid",
				Message:     "the requested package has invalid Info.json metadata",
				FolderName:  pkg.FolderName,
				PackageName: pkg.PackageName,
			})
		}
		if !pkg.ServerCompatible {
			issues = append(issues, OfficialModDiagnostic{
				Code:        "server_rule_missing",
				Message:     "the requested package has no InstallRule with IsServer set to true",
				FolderName:  pkg.FolderName,
				PackageName: pkg.PackageName,
			})
		}
	}
	return normalized, uniqueModDiagnostics(issues)
}

func validateOfficialModDependencies(active []string, packages []OfficialModPackage) []OfficialModDiagnostic {
	issues := make([]OfficialModDiagnostic, 0)
	activeSet := foldedStringSet(active)
	index := make(map[string][]OfficialModPackage)
	for _, pkg := range packages {
		if pkg.PackageName != "" {
			index[strings.ToLower(pkg.PackageName)] = append(index[strings.ToLower(pkg.PackageName)], pkg)
		}
	}
	for _, packageName := range active {
		matches := index[strings.ToLower(packageName)]
		if len(matches) != 1 {
			continue
		}
		pkg := matches[0]
		for _, dependency := range pkg.Dependencies {
			dependencyMatches := index[strings.ToLower(dependency)]
			if len(dependencyMatches) == 0 {
				issues = append(issues, OfficialModDiagnostic{
					Code:        "dependency_missing",
					Message:     "required dependency is not present in the selected Workshop directory",
					FolderName:  pkg.FolderName,
					PackageName: pkg.PackageName,
					Dependency:  dependency,
				})
			} else if len(dependencyMatches) > 1 {
				issues = append(issues, OfficialModDiagnostic{
					Code:        "dependency_ambiguous",
					Message:     "required dependency has a duplicate PackageName",
					FolderName:  pkg.FolderName,
					PackageName: pkg.PackageName,
					Dependency:  dependency,
				})
			} else if !activeSet[strings.ToLower(dependency)] {
				issues = append(issues, OfficialModDiagnostic{
					Code:        "dependency_disabled",
					Message:     "required dependency must also be added to ActiveModList",
					FolderName:  pkg.FolderName,
					PackageName: pkg.PackageName,
					Dependency:  dependency,
				})
			}
		}
	}
	return uniqueModDiagnostics(issues)
}

func blockingOfficialModRootIssues(issues []OfficialModDiagnostic) []OfficialModDiagnostic {
	result := make([]OfficialModDiagnostic, 0, len(issues))
	for _, issue := range issues {
		switch issue.Code {
		case "workshop_root_not_absolute",
			"workshop_root_invalid",
			"workshop_root_unreadable",
			"workshop_root_unsafe",
			"workshop_scan_failed":
			result = append(result, issue)
		}
	}
	return result
}

func parseOfficialModLaunchArguments(arguments []string) officialModLaunchSettings {
	settings := officialModLaunchSettings{}
	for index := 0; index < len(arguments); index++ {
		argument := strings.TrimSpace(arguments[index])
		if strings.EqualFold(argument, "-NoMods") {
			settings.forcedDisabled = true
			continue
		}
		if strings.EqualFold(argument, "-workshopdir") {
			if index+1 >= len(arguments) {
				settings.warnings = append(settings.warnings, modDiagnostic(
					"launch_workshop_root_invalid",
					"-workshopdir is missing its path argument",
				))
				continue
			}
			index++
			settings.workshopRoot = unquoteOptionalPath(arguments[index])
			continue
		}
		if len(argument) > len("-workshopdir=") && strings.EqualFold(argument[:len("-workshopdir=")], "-workshopdir=") {
			settings.workshopRoot = unquoteOptionalPath(argument[len("-workshopdir="):])
		}
	}
	return settings
}

func normalizeOfficialModRootSetting(root string) (string, error) {
	root = unquoteOptionalPath(root)
	if root == "" {
		return "", nil
	}
	if len(root) > maxPalModMetadataString || strings.ContainsAny(root, "\x00\r\n") {
		return "", errors.New("WorkshopRootDir contains unsupported characters or length")
	}
	if !filepath.IsAbs(root) {
		return "", errors.New("WorkshopRootDir must be an absolute path")
	}
	absolute, err := filepath.Abs(root)
	if err != nil {
		return "", err
	}
	return filepath.Clean(absolute), nil
}

func verifyCurrentPalModSettings(status OfficialModStatus) ([]byte, bool, error) {
	current, exists, err := readPalModSettingsRaw(status.SettingsPath)
	if err != nil {
		return nil, false, fmt.Errorf("%w: re-read PalModSettings.ini: %v", ErrOfficialModPlanChanged, err)
	}
	if exists != status.SettingsExists {
		return nil, false, ErrOfficialModPlanChanged
	}
	if exists && !strings.EqualFold(digestBytes(current), status.SettingsSHA256) {
		return nil, false, ErrOfficialModPlanChanged
	}
	return current, exists, nil
}

func createPalModSettingsRecovery(settingsPath string, content []byte, exists bool) (string, error) {
	backupDir, err := officialModBackupDir()
	if err != nil {
		return "", err
	}
	backupDir = filepath.Join(backupDir, "mods")
	if err := os.MkdirAll(backupDir, 0o755); err != nil {
		return "", err
	}
	timestamp := officialModNow().Format("20060102-150405.000000000")
	if exists {
		path := filepath.Join(backupDir, fmt.Sprintf(
			"PalModSettings-%s-%s-%s.ini",
			timestamp,
			digestBytes(content)[:8],
			uuid.NewString()[:8],
		))
		if err := writeExclusiveRegularFile(path, content, 0o600); err != nil {
			return "", err
		}
		return path, nil
	}
	marker := struct {
		SettingsExisted bool   `json:"settings_existed"`
		TargetPath      string `json:"target_path"`
		CreatedAt       string `json:"created_at"`
	}{
		SettingsExisted: false,
		TargetPath:      settingsPath,
		CreatedAt:       officialModNow().UTC().Format(time.RFC3339Nano),
	}
	payload, err := json.MarshalIndent(marker, "", "  ")
	if err != nil {
		return "", err
	}
	payload = append(payload, '\n')
	path := filepath.Join(backupDir, fmt.Sprintf(
		"PalModSettings-%s-absent-%s.json",
		timestamp,
		uuid.NewString()[:8],
	))
	if err := writeExclusiveRegularFile(path, payload, 0o600); err != nil {
		return "", err
	}
	return path, nil
}

func installPalModSettings(path string, content []byte, targetExists bool) error {
	staged, err := os.CreateTemp(filepath.Dir(path), ".pst-PalModSettings-*.tmp")
	if err != nil {
		return fmt.Errorf("stage PalModSettings.ini: %w", err)
	}
	stagedPath := staged.Name()
	defer officialModRemoveFile(stagedPath)
	if _, err := staged.Write(content); err != nil {
		_ = staged.Close()
		return fmt.Errorf("write staged PalModSettings.ini: %w", err)
	}
	if err := staged.Sync(); err != nil {
		_ = staged.Close()
		return fmt.Errorf("sync staged PalModSettings.ini: %w", err)
	}
	if err := staged.Close(); err != nil {
		return fmt.Errorf("close staged PalModSettings.ini: %w", err)
	}
	if targetExists {
		if err := system.PrepareReplacementFile(stagedPath, path); err != nil {
			return fmt.Errorf("preserve PalModSettings.ini metadata: %w", err)
		}
		if err := officialModReplaceFile(stagedPath, path); err != nil {
			return fmt.Errorf("replace PalModSettings.ini: %w", err)
		}
		return nil
	}
	if err := officialModLinkFile(stagedPath, path); err != nil {
		return fmt.Errorf("install PalModSettings.ini: %w", err)
	}
	_ = officialModRemoveFile(stagedPath)
	return nil
}

func verifyAppliedOfficialModSettings(status OfficialModStatus, expected OfficialModSettings, digest string) error {
	if !status.SettingsExists {
		return errors.New("PalModSettings.ini is missing after the update")
	}
	if !strings.EqualFold(status.SettingsSHA256, digest) {
		return errors.New("installed PalModSettings.ini digest does not match the staged file")
	}
	if status.Settings.GlobalEnabled != expected.GlobalEnabled ||
		!sameFilesystemPath(cleanOptionalPath(status.Settings.WorkshopRootDir), cleanOptionalPath(expected.WorkshopRootDir)) ||
		!equalFoldedStringSlices(status.Settings.ActiveModList, expected.ActiveModList) {
		return errors.New("installed PalModSettings.ini does not contain the requested settings")
	}
	return nil
}

func readRegularFileLimited(path string, limit int64) ([]byte, os.FileInfo, bool, error) {
	linkInfo, err := os.Lstat(path)
	if os.IsNotExist(err) {
		return nil, nil, false, nil
	}
	if err != nil {
		return nil, nil, false, err
	}
	if linkInfo.Mode()&os.ModeSymlink != 0 || !linkInfo.Mode().IsRegular() {
		return nil, nil, false, errors.New("file must be regular and not a symbolic link")
	}
	if linkInfo.Size() > limit {
		return nil, nil, false, fmt.Errorf("file exceeds %d bytes", limit)
	}
	file, err := os.Open(path)
	if err != nil {
		return nil, nil, false, err
	}
	defer file.Close()
	openedInfo, err := file.Stat()
	if err != nil {
		return nil, nil, false, err
	}
	if !openedInfo.Mode().IsRegular() || !os.SameFile(linkInfo, openedInfo) {
		return nil, nil, false, errors.New("file changed while it was opened")
	}
	content, err := io.ReadAll(io.LimitReader(file, limit+1))
	if err != nil {
		return nil, nil, false, err
	}
	if int64(len(content)) > limit {
		return nil, nil, false, fmt.Errorf("file exceeds %d bytes", limit)
	}
	return content, openedInfo, true, nil
}

func writeExclusiveRegularFile(path string, content []byte, mode os.FileMode) error {
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, mode)
	if err != nil {
		return err
	}
	if _, err := file.Write(content); err != nil {
		_ = file.Close()
		_ = os.Remove(path)
		return err
	}
	if err := file.Sync(); err != nil {
		_ = file.Close()
		_ = os.Remove(path)
		return err
	}
	if err := file.Close(); err != nil {
		_ = os.Remove(path)
		return err
	}
	return nil
}

func officialModStatusDigest(status OfficialModStatus) string {
	type packageDigest struct {
		FolderName  string `json:"folder_name"`
		InfoSHA256  string `json:"info_sha256"`
		PackageName string `json:"package_name"`
		Deployed    bool   `json:"deployed"`
	}
	packages := make([]packageDigest, 0, len(status.Inventory.Packages))
	for _, pkg := range status.Inventory.Packages {
		packages = append(packages, packageDigest{
			FolderName:  pkg.FolderName,
			InfoSHA256:  pkg.InfoSHA256,
			PackageName: pkg.PackageName,
			Deployed:    pkg.Deployed,
		})
	}
	payload := struct {
		Platform           string              `json:"platform"`
		InstallDir         string              `json:"install_dir"`
		SettingsExists     bool                `json:"settings_exists"`
		SettingsSHA256     string              `json:"settings_sha256"`
		Settings           OfficialModSettings `json:"settings"`
		ForcedDisabled     bool                `json:"forced_disabled"`
		LaunchWorkshopRoot string              `json:"launch_workshop_root"`
		WorkshopRoot       string              `json:"workshop_root"`
		WorkshopSource     string              `json:"workshop_source"`
		Packages           []packageDigest     `json:"packages"`
		ExistingWorlds     int                 `json:"existing_worlds"`
		SafetyBackupReady  bool                `json:"safety_backup_ready"`
		SavePath           string              `json:"save_path"`
	}{
		Platform:           status.Platform,
		InstallDir:         status.InstallDir,
		SettingsExists:     status.SettingsExists,
		SettingsSHA256:     status.SettingsSHA256,
		Settings:           status.Settings,
		ForcedDisabled:     status.ForcedDisabled,
		LaunchWorkshopRoot: status.LaunchWorkshopRoot,
		WorkshopRoot:       status.Inventory.WorkshopRoot,
		WorkshopSource:     status.Inventory.WorkshopSource,
		Packages:           packages,
		ExistingWorlds:     status.ExistingWorlds,
		SafetyBackupReady:  status.SafetyBackupReady,
		SavePath:           status.SavePath,
	}
	encoded, _ := json.Marshal(payload)
	return digestBytes(encoded)
}

func officialModPlanDigest(plan OfficialModChangePlan) string {
	type targetPackageDigest struct {
		FolderName  string `json:"folder_name"`
		InfoSHA256  string `json:"info_sha256"`
		PackageName string `json:"package_name"`
	}
	packages := make([]targetPackageDigest, 0, len(plan.TargetInventory.Packages))
	for _, pkg := range plan.TargetInventory.Packages {
		packages = append(packages, targetPackageDigest{
			FolderName:  pkg.FolderName,
			InfoSHA256:  pkg.InfoSHA256,
			PackageName: pkg.PackageName,
		})
	}
	payload := struct {
		StatusDigest   string                  `json:"status_digest"`
		Desired        OfficialModSettings     `json:"desired"`
		TargetRoot     string                  `json:"target_root"`
		TargetSource   string                  `json:"target_source"`
		TargetPackages []targetPackageDigest   `json:"target_packages"`
		Changes        []string                `json:"changes"`
		SafetyRequired bool                    `json:"safety_required"`
		SafetyReady    bool                    `json:"safety_ready"`
		Issues         []OfficialModDiagnostic `json:"issues"`
	}{
		StatusDigest:   plan.Status.StatusDigest,
		Desired:        plan.DesiredSettings,
		TargetRoot:     plan.TargetInventory.WorkshopRoot,
		TargetSource:   plan.TargetInventory.WorkshopSource,
		TargetPackages: packages,
		Changes:        plan.Changes,
		SafetyRequired: plan.SafetyBackupRequired,
		SafetyReady:    plan.SafetyBackupReady,
		Issues:         plan.Issues,
	}
	encoded, _ := json.Marshal(payload)
	return digestBytes(encoded)
}

func modDiagnostic(code, message string) OfficialModDiagnostic {
	return OfficialModDiagnostic{Code: code, Message: message}
}

func packageDiagnostic(code, message string, pkg OfficialModPackage) OfficialModDiagnostic {
	return OfficialModDiagnostic{
		Code:        code,
		Message:     message,
		FolderName:  pkg.FolderName,
		PackageName: pkg.PackageName,
	}
}

func uniqueModDiagnostics(values []OfficialModDiagnostic) []OfficialModDiagnostic {
	seen := make(map[string]bool, len(values))
	result := make([]OfficialModDiagnostic, 0, len(values))
	for _, value := range values {
		key := strings.Join([]string{
			value.Code,
			value.Message,
			value.FolderName,
			value.PackageName,
			value.Dependency,
		}, "\x00")
		if seen[key] {
			continue
		}
		seen[key] = true
		result = append(result, value)
	}
	return result
}

func joinModDiagnosticMessages(values []OfficialModDiagnostic) string {
	messages := make([]string, 0, len(values))
	for _, value := range values {
		message := value.Message
		if value.PackageName != "" {
			message = value.PackageName + ": " + message
		}
		if value.Dependency != "" {
			message += " (dependency: " + value.Dependency + ")"
		}
		messages = append(messages, message)
	}
	return strings.Join(uniqueStrings(messages), "; ")
}

func parseINISection(line string) (string, bool) {
	if len(line) < 2 || line[0] != '[' || line[len(line)-1] != ']' {
		return "", false
	}
	return strings.TrimSpace(line[1 : len(line)-1]), true
}

func parseINIKeyValue(line string) (string, string, bool) {
	index := strings.IndexByte(line, '=')
	if index <= 0 {
		return "", "", false
	}
	return strings.TrimSpace(line[:index]), strings.TrimSpace(line[index+1:]), true
}

func isKnownPalModSettingLine(line string) bool {
	if line == "" || strings.HasPrefix(line, ";") || strings.HasPrefix(line, "#") {
		return false
	}
	key, _, ok := parseINIKeyValue(line)
	if !ok {
		return false
	}
	return strings.EqualFold(key, "bGlobalEnableMod") ||
		strings.EqualFold(key, "WorkshopRootDir") ||
		strings.EqualFold(key, "ActiveModList")
}

func validOfficialModIdentifier(value string) bool {
	original := value
	value = strings.TrimSpace(value)
	if value == "" || value == "." || value == ".." || len(value) > 256 || original != value || !utf8.ValidString(value) {
		return false
	}
	if strings.ContainsAny(value, "\x00\r\n=/\\[];#") {
		return false
	}
	for _, character := range value {
		if character < 0x20 || character == 0x7f {
			return false
		}
	}
	return true
}

func officialModInstallTypeSupported(value string) bool {
	for _, supported := range []string{"UE4SS", "Lua", "PalSchema", "LogicMods", "Paks"} {
		if strings.EqualFold(strings.TrimSpace(value), supported) {
			return true
		}
	}
	return false
}

func validOfficialModTarget(value string) bool {
	value = strings.TrimSpace(value)
	if value == "" || strings.HasPrefix(value, "/") || strings.HasPrefix(value, "\\") {
		return false
	}
	if len(value) >= 2 && ((value[0] >= 'A' && value[0] <= 'Z') || (value[0] >= 'a' && value[0] <= 'z')) && value[1] == ':' {
		return false
	}
	normalized := strings.ReplaceAll(value, "\\", "/")
	for _, component := range strings.Split(normalized, "/") {
		if component == ".." {
			return false
		}
	}
	return true
}

func applyOfficialModSelectionToInventory(
	inventory *OfficialModInventory,
	globalEnabled bool,
	active []string,
	forcedDisabled bool,
) {
	if inventory == nil {
		return
	}
	activeSet := foldedStringSet(active)
	for index := range inventory.Packages {
		pkg := &inventory.Packages[index]
		pkg.Listed = activeSet[strings.ToLower(pkg.PackageName)]
		pkg.EffectiveEnabled = globalEnabled && !forcedDisabled && pkg.Listed
		pkg.PendingRestart = pkg.EffectiveEnabled && !pkg.Deployed
		pkg.PendingRemoval = !pkg.EffectiveEnabled && pkg.Deployed
	}
}

func trimmedStrings(values []string) []string {
	result := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value != "" {
			result = append(result, value)
		}
	}
	return result
}

func uniqueFoldedStrings(values []string) []string {
	seen := make(map[string]bool, len(values))
	result := make([]string, 0, len(values))
	for _, value := range values {
		key := strings.ToLower(strings.TrimSpace(value))
		if key == "" || seen[key] {
			continue
		}
		seen[key] = true
		result = append(result, strings.TrimSpace(value))
	}
	return result
}

func appendUniqueFolded(values []string, value string) []string {
	for _, existing := range values {
		if strings.EqualFold(existing, value) {
			return values
		}
	}
	return append(values, value)
}

func foldedStringSet(values []string) map[string]bool {
	set := make(map[string]bool, len(values))
	for _, value := range values {
		set[strings.ToLower(strings.TrimSpace(value))] = true
	}
	return set
}

func equalFoldedStringSlices(left, right []string) bool {
	if len(left) != len(right) {
		return false
	}
	for index := range left {
		if !strings.EqualFold(strings.TrimSpace(left[index]), strings.TrimSpace(right[index])) {
			return false
		}
	}
	return true
}

func boolINI(value bool) string {
	if value {
		return "True"
	}
	return "False"
}

func cleanOptionalPath(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	return filepath.Clean(value)
}

func unquoteOptionalPath(value string) string {
	value = strings.TrimSpace(value)
	if len(value) >= 2 && value[0] == '"' && value[len(value)-1] == '"' {
		value = strings.TrimSpace(value[1 : len(value)-1])
	}
	return value
}

func workshopRootOverrideSentinel(root string) string {
	if root == "" {
		return "__default__"
	}
	return root
}

func isDecimalIdentifier(value string) bool {
	if value == "" {
		return false
	}
	for _, character := range value {
		if character < '0' || character > '9' {
			return false
		}
	}
	return true
}

func digestBytes(content []byte) string {
	digest := sha256.Sum256(content)
	return hex.EncodeToString(digest[:])
}
