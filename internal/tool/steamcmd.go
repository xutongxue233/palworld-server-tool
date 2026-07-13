package tool

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/spf13/viper"
)

const (
	PalworldDedicatedServerAppID = 2394010
	maxSteamManifestSize         = 1024 * 1024
	maxSteamCMDOutput            = 32 * 1024
)

var (
	ErrSteamCMDNotConfigured = errors.New("steamcmd is not configured")
	ErrSteamCMDInvalid       = errors.New("steamcmd preflight failed")
	ErrSteamCMDPlanChanged   = errors.New("steamcmd plan changed")
	ErrSteamCMDUpdateFailed  = errors.New("steamcmd install or update failed")

	steamCMDMu     sync.Mutex
	steamCMDNow    = time.Now
	steamCMDRunner = runSteamCMDProcess
	manifestAppID  = regexp.MustCompile(`(?m)"appid"\s+"([0-9]+)"`)
	manifestBuild  = regexp.MustCompile(`(?m)"buildid"\s+"([0-9]+)"`)
)

type SteamCMDPlan struct {
	Configured           bool     `json:"configured"`
	CanExecute           bool     `json:"can_execute"`
	AppID                int      `json:"app_id"`
	Platform             string   `json:"platform"`
	ExecutablePath       string   `json:"executable_path,omitempty"`
	ExecutableSHA256     string   `json:"executable_sha256,omitempty"`
	InstallDir           string   `json:"install_dir,omitempty"`
	ManifestPath         string   `json:"manifest_path,omitempty"`
	LauncherPath         string   `json:"launcher_path,omitempty"`
	Installed            bool     `json:"installed"`
	PartialInstallation  bool     `json:"partial_installation"`
	BuildID              string   `json:"build_id,omitempty"`
	ExistingWorlds       int      `json:"existing_worlds"`
	SafetyBackupRequired bool     `json:"safety_backup_required"`
	SafetyBackupReady    bool     `json:"safety_backup_ready"`
	SavePath             string   `json:"save_path,omitempty"`
	TimeoutSeconds       int      `json:"timeout_seconds"`
	Issues               []string `json:"issues,omitempty"`
	Warnings             []string `json:"warnings,omitempty"`
	PlanDigest           string   `json:"plan_digest"`

	manifestSHA256 string
	launcherSize   int64
	launcherMTime  int64
}

type SteamCMDUpdateOptions struct {
	ExpectedPlanDigest string
	ValidateFiles      bool
}

type SteamCMDUpdateResult struct {
	Before        SteamCMDPlan `json:"before"`
	After         SteamCMDPlan `json:"after"`
	BuildIDBefore string       `json:"build_id_before,omitempty"`
	BuildIDAfter  string       `json:"build_id_after,omitempty"`
	Changed       bool         `json:"changed"`
	Validated     bool         `json:"validated"`
	OutputTail    string       `json:"output_tail,omitempty"`
	StartedAt     time.Time    `json:"started_at"`
	FinishedAt    time.Time    `json:"finished_at"`
	DurationMS    int64        `json:"duration_ms"`
}

func ConfirmGameServerStopped(ctx context.Context, confirmManualStop bool) error {
	status := GetServerControlStatus(ctx)
	if status.Online || status.Running {
		return ErrGameServerRunning
	}
	if status.Configured {
		state := strings.ToLower(strings.TrimSpace(status.State))
		if status.Detail != "" || state == "" || state == "unknown" || state == "invalid" || state == "unconfigured" {
			return ErrGameServerStatusUnknown
		}
		return nil
	}
	if !confirmManualStop {
		return ErrSaveEditConfirmation
	}
	return nil
}

type steamCMDConfig struct {
	executable string
	installDir string
	timeout    time.Duration
}

type steamManifest struct {
	exists bool
	valid  bool
	appID  string
	build  string
	digest string
}

func InspectSteamCMD() SteamCMDPlan {
	plan := SteamCMDPlan{
		AppID:          PalworldDedicatedServerAppID,
		Platform:       runtime.GOOS,
		TimeoutSeconds: viper.GetInt("steamcmd.timeout"),
		ExecutablePath: strings.TrimSpace(viper.GetString("steamcmd.executable")),
		InstallDir:     strings.TrimSpace(viper.GetString("steamcmd.install_dir")),
		Issues:         make([]string, 0),
		Warnings:       make([]string, 0),
	}
	if plan.TimeoutSeconds == 0 {
		plan.TimeoutSeconds = 1800
	}
	if runtime.GOOS != "windows" && runtime.GOOS != "linux" {
		plan.Issues = append(plan.Issues, "SteamCMD management is supported only on Windows and Linux")
	}
	if plan.ExecutablePath == "" {
		plan.Issues = append(plan.Issues, "steamcmd.executable must be an absolute path")
	}
	if plan.InstallDir == "" {
		plan.Issues = append(plan.Issues, "steamcmd.install_dir must be an absolute path")
	}
	if plan.TimeoutSeconds < 60 || plan.TimeoutSeconds > 7200 {
		plan.Issues = append(plan.Issues, "steamcmd.timeout must be between 60 and 7200 seconds")
	}

	if plan.ExecutablePath != "" {
		absolute, err := filepath.Abs(plan.ExecutablePath)
		if err != nil || !filepath.IsAbs(plan.ExecutablePath) {
			plan.Issues = append(plan.Issues, "steamcmd.executable must be an absolute path")
		} else {
			plan.ExecutablePath = filepath.Clean(absolute)
			digest, inspectErr := inspectSteamExecutable(plan.ExecutablePath)
			if inspectErr != nil {
				plan.Issues = append(plan.Issues, inspectErr.Error())
			} else {
				plan.ExecutableSHA256 = digest
			}
		}
	}
	if plan.InstallDir != "" {
		absolute, err := filepath.Abs(plan.InstallDir)
		if err != nil || !filepath.IsAbs(plan.InstallDir) {
			plan.Issues = append(plan.Issues, "steamcmd.install_dir must be an absolute path")
		} else {
			plan.InstallDir = filepath.Clean(absolute)
			if err := inspectSteamInstallDir(plan.InstallDir); err != nil {
				plan.Issues = append(plan.Issues, err.Error())
			}
		}
	}

	if plan.InstallDir != "" && filepath.IsAbs(plan.InstallDir) {
		plan.ManifestPath = filepath.Join(plan.InstallDir, "steamapps", "appmanifest_2394010.acf")
		plan.LauncherPath = steamLauncherPath(plan.InstallDir)
		manifest, err := inspectSteamManifest(plan.ManifestPath)
		if err != nil {
			plan.Issues = append(plan.Issues, err.Error())
		}
		plan.manifestSHA256 = manifest.digest
		plan.BuildID = manifest.build
		launcherExists, launcherSize, launcherMTime, launcherErr := inspectSteamLauncher(plan.LauncherPath)
		if launcherErr != nil {
			plan.Issues = append(plan.Issues, launcherErr.Error())
		}
		plan.launcherSize = launcherSize
		plan.launcherMTime = launcherMTime
		plan.Installed = manifest.valid && launcherExists
		palDirExists := pathExists(filepath.Join(plan.InstallDir, "Pal"))
		plan.PartialInstallation = !plan.Installed && (manifest.exists || launcherExists || palDirExists)
		if plan.PartialInstallation {
			plan.Warnings = append(plan.Warnings, "A partial installation was detected and will be repaired by SteamCMD")
		}
		worlds, worldErr := countInstalledWorlds(plan.InstallDir)
		if worldErr != nil {
			plan.Issues = append(plan.Issues, worldErr.Error())
		} else {
			plan.ExistingWorlds = worlds
		}
	}

	configuredSave := strings.TrimSpace(viper.GetString("save.path"))
	if configuredSave != "" && configuredSave != "/path/to/your/Pal/Saved" && !isRemoteSaveSource(configuredSave) {
		if absolute, err := filepath.Abs(configuredSave); err == nil {
			plan.SavePath = filepath.Clean(absolute)
		}
		if levelPath, err := inspectSteamSafetyBackupSource(configuredSave); err == nil {
			if plan.ExistingWorlds > 0 {
				plan.SafetyBackupReady = true
				installedSaveRoot := filepath.Join(plan.InstallDir, "Pal", "Saved", "SaveGames", "0")
				if !pathWithin(levelPath, installedSaveRoot) {
					plan.SafetyBackupReady = false
					plan.Issues = append(plan.Issues, "save.path does not point to a world inside steamcmd.install_dir")
				}
			}
		} else if plan.ExistingWorlds > 0 {
			plan.Issues = append(plan.Issues, "save.path cannot create the required pre-update safety backup: "+err.Error())
		}
	}
	if plan.ExistingWorlds > 0 {
		plan.SafetyBackupRequired = true
		if !plan.SafetyBackupReady {
			plan.Issues = append(plan.Issues, "configure a local save.path before updating an installation that contains world data")
		}
	}

	plan.Configured = plan.ExecutablePath != "" && plan.InstallDir != ""
	plan.Issues = uniqueStrings(plan.Issues)
	plan.Warnings = uniqueStrings(plan.Warnings)
	plan.CanExecute = plan.Configured && len(plan.Issues) == 0
	plan.PlanDigest = steamCMDPlanDigest(plan)
	return plan
}

func RunSteamCMDUpdate(ctx context.Context, options SteamCMDUpdateOptions) (SteamCMDUpdateResult, error) {
	steamCMDMu.Lock()
	defer steamCMDMu.Unlock()

	started := steamCMDNow().UTC()
	result := SteamCMDUpdateResult{StartedAt: started, Validated: options.ValidateFiles}
	plan := InspectSteamCMD()
	result.Before = plan
	result.BuildIDBefore = plan.BuildID
	if !plan.Configured {
		return finishSteamCMDResult(result), ErrSteamCMDNotConfigured
	}
	if !plan.CanExecute {
		return finishSteamCMDResult(result), fmt.Errorf("%w: %s", ErrSteamCMDInvalid, strings.Join(plan.Issues, "; "))
	}
	if strings.TrimSpace(options.ExpectedPlanDigest) == "" ||
		!strings.EqualFold(options.ExpectedPlanDigest, plan.PlanDigest) {
		return finishSteamCMDResult(result), ErrSteamCMDPlanChanged
	}

	config := steamCMDConfig{
		executable: plan.ExecutablePath,
		installDir: plan.InstallDir,
		timeout:    time.Duration(plan.TimeoutSeconds) * time.Second,
	}
	if err := os.MkdirAll(config.installDir, 0o755); err != nil {
		return finishSteamCMDResult(result), fmt.Errorf("%w: create install directory: %v", ErrSteamCMDUpdateFailed, err)
	}
	if err := inspectSteamInstallDir(config.installDir); err != nil {
		return finishSteamCMDResult(result), fmt.Errorf("%w: %v", ErrSteamCMDUpdateFailed, err)
	}

	args := []string{
		"+force_install_dir", config.installDir,
		"+login", "anonymous",
		"+app_update", fmt.Sprint(PalworldDedicatedServerAppID),
	}
	if options.ValidateFiles {
		args = append(args, "validate")
	}
	args = append(args, "+quit")

	operationCtx, cancel := context.WithTimeout(ctx, config.timeout)
	output, runErr := steamCMDRunner(operationCtx, config.executable, args)
	cancel()
	result.OutputTail = output
	if runErr != nil {
		if errors.Is(operationCtx.Err(), context.DeadlineExceeded) {
			runErr = fmt.Errorf("timed out after %s", config.timeout)
		}
		return finishSteamCMDResult(result), fmt.Errorf("%w: %v", ErrSteamCMDUpdateFailed, runErr)
	}

	after := InspectSteamCMD()
	result.After = after
	result.BuildIDAfter = after.BuildID
	result.Changed = result.BuildIDBefore != result.BuildIDAfter
	if !after.Installed || after.BuildID == "" {
		return finishSteamCMDResult(result), fmt.Errorf(
			"%w: SteamCMD exited successfully but the Palworld launcher or app manifest is invalid",
			ErrSteamCMDUpdateFailed,
		)
	}
	return finishSteamCMDResult(result), nil
}

func finishSteamCMDResult(result SteamCMDUpdateResult) SteamCMDUpdateResult {
	finished := steamCMDNow().UTC()
	result.FinishedAt = finished
	result.DurationMS = finished.Sub(result.StartedAt).Milliseconds()
	return result
}

func inspectSteamExecutable(path string) (string, error) {
	info, err := os.Lstat(path)
	if err != nil {
		return "", fmt.Errorf("inspect steamcmd.executable: %w", err)
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() {
		return "", errors.New("steamcmd.executable must be a regular file and not a symbolic link")
	}
	if runtime.GOOS == "windows" && !strings.EqualFold(filepath.Ext(path), ".exe") {
		return "", errors.New("steamcmd.executable must be an .exe file on Windows")
	}
	if runtime.GOOS != "windows" && info.Mode().Perm()&0o111 == 0 {
		return "", errors.New("steamcmd.executable must have an executable permission bit")
	}
	if err := rejectSymlinkComponents(path); err != nil {
		return "", fmt.Errorf("steamcmd.executable: %w", err)
	}
	file, err := os.Open(path)
	if err != nil {
		return "", fmt.Errorf("open steamcmd.executable: %w", err)
	}
	defer file.Close()
	hasher := sha256.New()
	if _, err := io.Copy(hasher, file); err != nil {
		return "", fmt.Errorf("hash steamcmd.executable: %w", err)
	}
	return hex.EncodeToString(hasher.Sum(nil)), nil
}

func inspectSteamInstallDir(path string) error {
	if !filepath.IsAbs(path) {
		return errors.New("steamcmd.install_dir must be an absolute path")
	}
	root := filepath.VolumeName(path) + string(os.PathSeparator)
	if sameFilesystemPath(filepath.Clean(path), filepath.Clean(root)) {
		return errors.New("steamcmd.install_dir cannot be a filesystem root")
	}
	probe := path
	for {
		info, err := os.Lstat(probe)
		if err == nil {
			if info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
				return fmt.Errorf("steamcmd.install_dir ancestor %q must be a real directory", probe)
			}
			if err := rejectSymlinkComponents(probe); err != nil {
				return fmt.Errorf("steamcmd.install_dir: %w", err)
			}
			return nil
		}
		if !os.IsNotExist(err) {
			return fmt.Errorf("inspect steamcmd.install_dir: %w", err)
		}
		parent := filepath.Dir(probe)
		if parent == probe {
			return errors.New("steamcmd.install_dir has no accessible parent directory")
		}
		probe = parent
	}
}

func inspectSteamManifest(path string) (steamManifest, error) {
	info, err := os.Lstat(path)
	if os.IsNotExist(err) {
		return steamManifest{}, nil
	}
	if err != nil {
		return steamManifest{}, fmt.Errorf("inspect Steam app manifest: %w", err)
	}
	manifest := steamManifest{exists: true}
	if info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() {
		return manifest, errors.New("Steam app manifest must be a regular file and not a symbolic link")
	}
	if err := rejectSymlinkComponents(path); err != nil {
		return manifest, fmt.Errorf("Steam app manifest path: %w", err)
	}
	if info.Size() <= 0 || info.Size() > maxSteamManifestSize {
		return manifest, errors.New("Steam app manifest has an invalid size")
	}
	content, err := os.ReadFile(path)
	if err != nil {
		return manifest, fmt.Errorf("read Steam app manifest: %w", err)
	}
	digest := sha256.Sum256(content)
	manifest.digest = hex.EncodeToString(digest[:])
	if match := manifestAppID.FindSubmatch(content); len(match) == 2 {
		manifest.appID = string(match[1])
	}
	if match := manifestBuild.FindSubmatch(content); len(match) == 2 {
		manifest.build = string(match[1])
	}
	if manifest.appID != fmt.Sprint(PalworldDedicatedServerAppID) {
		return manifest, fmt.Errorf("Steam app manifest does not describe Palworld Dedicated Server app %d", PalworldDedicatedServerAppID)
	}
	if manifest.build == "" {
		return manifest, errors.New("Steam app manifest does not contain a build ID")
	}
	manifest.valid = true
	return manifest, nil
}

func inspectSteamLauncher(path string) (bool, int64, int64, error) {
	if path == "" {
		return false, 0, 0, nil
	}
	info, err := os.Lstat(path)
	if os.IsNotExist(err) {
		return false, 0, 0, nil
	}
	if err != nil {
		return false, 0, 0, fmt.Errorf("inspect Palworld server launcher: %w", err)
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() || info.Size() <= 0 {
		return false, 0, 0, errors.New("Palworld server launcher must be a non-empty regular file and not a symbolic link")
	}
	if runtime.GOOS != "windows" && info.Mode().Perm()&0o111 == 0 {
		return false, 0, 0, errors.New("Palworld server launcher must have an executable permission bit")
	}
	return true, info.Size(), info.ModTime().UTC().UnixNano(), nil
}

func steamLauncherPath(installDir string) string {
	switch runtime.GOOS {
	case "windows":
		return filepath.Join(installDir, "PalServer.exe")
	case "linux":
		return filepath.Join(installDir, "PalServer.sh")
	default:
		return ""
	}
}

func countInstalledWorlds(installDir string) (int, error) {
	root := filepath.Join(installDir, "Pal", "Saved", "SaveGames", "0")
	rootInfo, err := os.Lstat(root)
	if os.IsNotExist(err) {
		return 0, nil
	}
	if err != nil {
		return 0, fmt.Errorf("inspect installed Palworld world root: %w", err)
	}
	if rootInfo.Mode()&os.ModeSymlink != 0 || !rootInfo.IsDir() {
		return 0, errors.New("installed Palworld world root must be a real directory")
	}
	if err := rejectSymlinkComponents(root); err != nil {
		return 0, fmt.Errorf("installed Palworld world root: %w", err)
	}
	entries, err := os.ReadDir(root)
	if err != nil {
		return 0, fmt.Errorf("inspect installed Palworld worlds: %w", err)
	}
	count := 0
	for _, entry := range entries {
		if entry.Type()&os.ModeSymlink != 0 || !entry.IsDir() {
			continue
		}
		levelPath := filepath.Join(root, entry.Name(), "Level.sav")
		info, err := os.Lstat(levelPath)
		if os.IsNotExist(err) {
			continue
		}
		if err != nil {
			return 0, fmt.Errorf("inspect installed world %q: %w", entry.Name(), err)
		}
		if info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() || info.Size() <= 0 {
			return 0, fmt.Errorf("installed world %q has an invalid Level.sav", entry.Name())
		}
		count++
	}
	return count, nil
}

func inspectSteamSafetyBackupSource(configuredPath string) (string, error) {
	info, err := os.Lstat(configuredPath)
	if err != nil {
		return "", err
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return "", errors.New("save.path cannot be a symbolic link")
	}
	if err := rejectSymlinkComponents(configuredPath); err != nil {
		return "", fmt.Errorf("save.path: %w", err)
	}
	levelPath, err := localLevelSavePath(configuredPath)
	if err != nil {
		return "", err
	}
	levelInfo, err := os.Lstat(levelPath)
	if err != nil {
		return "", err
	}
	if levelInfo.Mode()&os.ModeSymlink != 0 || !levelInfo.Mode().IsRegular() || levelInfo.Size() <= 0 {
		return "", errors.New("save.path must resolve to a non-empty regular Level.sav")
	}
	if err := rejectSymlinkComponents(levelPath); err != nil {
		return "", fmt.Errorf("save.path Level.sav: %w", err)
	}
	return levelPath, nil
}

func steamCMDPlanDigest(plan SteamCMDPlan) string {
	payload := struct {
		AppID                int      `json:"app_id"`
		Platform             string   `json:"platform"`
		ExecutablePath       string   `json:"executable_path"`
		ExecutableSHA256     string   `json:"executable_sha256"`
		InstallDir           string   `json:"install_dir"`
		ManifestSHA256       string   `json:"manifest_sha256"`
		LauncherPath         string   `json:"launcher_path"`
		LauncherSize         int64    `json:"launcher_size"`
		LauncherMTime        int64    `json:"launcher_mtime"`
		BuildID              string   `json:"build_id"`
		ExistingWorlds       int      `json:"existing_worlds"`
		SafetyBackupRequired bool     `json:"safety_backup_required"`
		SafetyBackupReady    bool     `json:"safety_backup_ready"`
		SavePath             string   `json:"save_path"`
		TimeoutSeconds       int      `json:"timeout_seconds"`
		Issues               []string `json:"issues"`
	}{
		AppID:                plan.AppID,
		Platform:             plan.Platform,
		ExecutablePath:       plan.ExecutablePath,
		ExecutableSHA256:     plan.ExecutableSHA256,
		InstallDir:           plan.InstallDir,
		ManifestSHA256:       plan.manifestSHA256,
		LauncherPath:         plan.LauncherPath,
		LauncherSize:         plan.launcherSize,
		LauncherMTime:        plan.launcherMTime,
		BuildID:              plan.BuildID,
		ExistingWorlds:       plan.ExistingWorlds,
		SafetyBackupRequired: plan.SafetyBackupRequired,
		SafetyBackupReady:    plan.SafetyBackupReady,
		SavePath:             plan.SavePath,
		TimeoutSeconds:       plan.TimeoutSeconds,
		Issues:               plan.Issues,
	}
	encoded, _ := json.Marshal(payload)
	digest := sha256.Sum256(encoded)
	return hex.EncodeToString(digest[:])
}

func runSteamCMDProcess(ctx context.Context, executable string, args []string) (string, error) {
	cmd := exec.CommandContext(ctx, executable, args...)
	cmd.Dir = filepath.Dir(executable)
	output := &steamTailBuffer{limit: maxSteamCMDOutput}
	cmd.Stdout = output
	cmd.Stderr = output
	err := cmd.Run()
	return output.String(), err
}

type steamTailBuffer struct {
	mu    sync.Mutex
	limit int
	data  []byte
}

func (buffer *steamTailBuffer) Write(data []byte) (int, error) {
	buffer.mu.Lock()
	defer buffer.mu.Unlock()
	written := len(data)
	buffer.data = append(buffer.data, data...)
	if len(buffer.data) > buffer.limit {
		buffer.data = append([]byte(nil), buffer.data[len(buffer.data)-buffer.limit:]...)
	}
	return written, nil
}

func (buffer *steamTailBuffer) String() string {
	buffer.mu.Lock()
	defer buffer.mu.Unlock()
	return strings.TrimSpace(strings.ToValidUTF8(string(buffer.data), "�"))
}

func rejectSymlinkComponents(path string) error {
	evaluated, err := filepath.EvalSymlinks(path)
	if err != nil {
		return err
	}
	absoluteEvaluated, err := filepath.Abs(evaluated)
	if err != nil {
		return err
	}
	absolutePath, err := filepath.Abs(path)
	if err != nil {
		return err
	}
	if !sameFilesystemPath(filepath.Clean(absoluteEvaluated), filepath.Clean(absolutePath)) {
		return errors.New("path cannot contain symbolic-link components")
	}
	return nil
}

func sameFilesystemPath(left, right string) bool {
	if runtime.GOOS == "windows" {
		return strings.EqualFold(left, right)
	}
	return left == right
}

func pathWithin(path, root string) bool {
	absolutePath, err := filepath.Abs(path)
	if err != nil {
		return false
	}
	absoluteRoot, err := filepath.Abs(root)
	if err != nil {
		return false
	}
	relative, err := filepath.Rel(filepath.Clean(absoluteRoot), filepath.Clean(absolutePath))
	if err != nil || relative == ".." || strings.HasPrefix(relative, ".."+string(os.PathSeparator)) {
		return false
	}
	return true
}

func pathExists(path string) bool {
	_, err := os.Lstat(path)
	return err == nil
}

func uniqueStrings(values []string) []string {
	seen := make(map[string]struct{}, len(values))
	result := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		if _, exists := seen[value]; exists {
			continue
		}
		seen[value] = struct{}{}
		result = append(result, value)
	}
	return result
}
