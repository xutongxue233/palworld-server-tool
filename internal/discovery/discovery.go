package discovery

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/spf13/viper"
	"github.com/zaigie/palworld-server-tool/internal/config"
)

const palServerAppID = "2394010"

var (
	discoveryMu sync.RWMutex
	operationMu sync.Mutex
	cached      Status

	vdfPathPattern       = regexp.MustCompile(`(?i)"path"\s+"([^"]+)"`)
	manifestDirPattern   = regexp.MustCompile(`(?i)"installdir"\s+"([^"]+)"`)
	dedicatedNamePattern = regexp.MustCompile(`(?mi)^\s*DedicatedServerName\s*=\s*([^\r\n]+)`)
)

type World struct {
	ID         string    `json:"id"`
	Path       string    `json:"path"`
	ModifiedAt time.Time `json:"modified_at"`
}

type Candidate struct {
	ID               string  `json:"id"`
	Platform         string  `json:"platform"`
	Source           string  `json:"source"`
	InstallDir       string  `json:"install_dir"`
	LauncherPath     string  `json:"launcher_path"`
	ConfigPath       string  `json:"config_path"`
	ConfigExists     bool    `json:"config_exists"`
	SavePath         string  `json:"save_path,omitempty"`
	SavedDir         string  `json:"saved_dir"`
	SteamCMDPath     string  `json:"steamcmd_path,omitempty"`
	ManifestPath     string  `json:"manifest_path,omitempty"`
	Worlds           []World `json:"worlds,omitempty"`
	Score            int     `json:"score"`
	RESTPort         int     `json:"rest_port"`
	RCONPort         int     `json:"rcon_port"`
	RESTEnabled      bool    `json:"rest_enabled"`
	RCONEnabled      bool    `json:"rcon_enabled"`
	adminPassword    string
	dedicatedWorldID string
}

type EffectiveConfiguration struct {
	ConfigStore    string `json:"config_store"`
	InstallDir     string `json:"install_dir,omitempty"`
	LauncherPath   string `json:"launcher_path,omitempty"`
	GameConfigPath string `json:"game_config_path,omitempty"`
	SavePath       string `json:"save_path,omitempty"`
	ControlMode    string `json:"control_mode,omitempty"`
	SteamCMDPath   string `json:"steamcmd_path,omitempty"`
	RESTAddress    string `json:"rest_address,omitempty"`
	RCONAddress    string `json:"rcon_address,omitempty"`
}

type Status struct {
	Platform            string                 `json:"platform"`
	Configured          bool                   `json:"configured"`
	AutoConfigured      bool                   `json:"auto_configured"`
	NeedsSetup          bool                   `json:"needs_setup"`
	NeedsSelection      bool                   `json:"needs_selection"`
	ManualRequired      bool                   `json:"manual_required"`
	RestartRequired     bool                   `json:"restart_required"`
	SelectedCandidateID string                 `json:"selected_candidate_id,omitempty"`
	Candidates          []Candidate            `json:"candidates"`
	Effective           EffectiveConfiguration `json:"effective"`
	Warnings            []string               `json:"warnings,omitempty"`
	ScannedAt           time.Time              `json:"scanned_at"`
}

type ApplyRequest struct {
	CandidateID string `json:"candidate_id"`
	InstallDir  string `json:"install_dir"`
}

func Initialize() Status {
	operationMu.Lock()
	defer operationMu.Unlock()
	status := scan()
	if !status.Configured {
		if candidate := automaticCandidate(status.Candidates); candidate != nil {
			if applied, err := applyCandidate(*candidate, false); err != nil {
				status.Warnings = append(status.Warnings, "automatic configuration failed: "+err.Error())
			} else if !applied {
				status.Warnings = append(status.Warnings, "automatic configuration was stored but requires a PST restart")
			} else {
				status = scan()
				status.AutoConfigured = true
				status.SelectedCandidateID = candidate.ID
			}
		}
	}
	store(status)
	return status
}

func Current() Status {
	discoveryMu.RLock()
	status := cached
	discoveryMu.RUnlock()
	if status.ScannedAt.IsZero() {
		operationMu.Lock()
		discoveryMu.RLock()
		status = cached
		discoveryMu.RUnlock()
		if status.ScannedAt.IsZero() {
			status = scan()
			store(status)
		}
		operationMu.Unlock()
	}
	return status
}

func Rescan() Status {
	operationMu.Lock()
	defer operationMu.Unlock()
	status := scan()
	store(status)
	return status
}

func Apply(request ApplyRequest) (Status, error) {
	operationMu.Lock()
	defer operationMu.Unlock()
	var candidate Candidate
	switch {
	case strings.TrimSpace(request.CandidateID) != "":
		status := scan()
		for _, found := range status.Candidates {
			if found.ID == strings.TrimSpace(request.CandidateID) {
				candidate = found
				break
			}
		}
		if candidate.ID == "" {
			return Status{}, errors.New("discovery candidate no longer exists; scan again")
		}
	case strings.TrimSpace(request.InstallDir) != "":
		found, ok := inspectCandidatePath(request.InstallDir, "manual", "")
		if !ok {
			return Status{}, errors.New("the selected path does not belong to a valid PalServer installation")
		}
		candidate = found
	default:
		return Status{}, errors.New("candidate_id or install_dir is required")
	}

	applied, err := applyCandidate(candidate, true)
	if err != nil {
		return Status{}, err
	}
	status := scan()
	status.SelectedCandidateID = candidate.ID
	if !applied {
		status.RestartRequired = true
		status.Warnings = append(status.Warnings, "configuration saved to pst.db; restart PST to activate it")
	}
	store(status)
	return status, nil
}

func store(status Status) {
	discoveryMu.Lock()
	cached = status
	discoveryMu.Unlock()
}

func scan() Status {
	candidates := discoverCandidates()
	configured := hasMeaningfulConfiguration()
	status := Status{
		Platform:       runtime.GOOS,
		Configured:     configured,
		NeedsSetup:     !configured,
		Candidates:     candidates,
		Effective:      effectiveConfiguration(),
		ScannedAt:      time.Now().UTC(),
		ManualRequired: !configured && len(candidates) == 0,
	}
	if !configured && len(candidates) > 0 && automaticCandidate(candidates) == nil {
		status.NeedsSelection = true
	}
	return status
}

func effectiveConfiguration() EffectiveConfiguration {
	installDir := strings.TrimSpace(viper.GetString("steamcmd.install_dir"))
	if installDir == "" {
		installDir = strings.TrimSpace(viper.GetString("mods.install_dir"))
	}
	return EffectiveConfiguration{
		ConfigStore:    config.StorageName(),
		InstallDir:     installDir,
		LauncherPath:   strings.TrimSpace(viper.GetString("palworld.control.target")),
		GameConfigPath: strings.TrimSpace(viper.GetString("palworld.config_path")),
		SavePath:       strings.TrimSpace(viper.GetString("save.path")),
		ControlMode:    strings.TrimSpace(viper.GetString("palworld.control.mode")),
		SteamCMDPath:   strings.TrimSpace(viper.GetString("steamcmd.executable")),
		RESTAddress:    strings.TrimSpace(viper.GetString("rest.address")),
		RCONAddress:    strings.TrimSpace(viper.GetString("rcon.address")),
	}
}

func hasMeaningfulConfiguration() bool {
	paths := []string{
		viper.GetString("save.path"),
		viper.GetString("palworld.config_path"),
		viper.GetString("palworld.control.target"),
		viper.GetString("steamcmd.install_dir"),
		viper.GetString("mods.install_dir"),
	}
	for _, value := range paths {
		value = strings.TrimSpace(value)
		if value != "" && value != "/path/to/your/Pal/Saved" && !strings.Contains(value, "path/to/PalServer") {
			return true
		}
	}
	return false
}

func automaticCandidate(candidates []Candidate) *Candidate {
	if len(candidates) == 0 {
		return nil
	}
	if len(candidates) == 1 || candidates[0].Score >= candidates[1].Score+15 {
		candidate := candidates[0]
		return &candidate
	}
	return nil
}

func applyCandidate(candidate Candidate, overwrite bool) (bool, error) {
	values := make(map[string]any)
	set := func(key string, value any, missing func() bool) {
		if overwrite || missing() {
			values[key] = value
		}
	}
	blank := func(key string) func() bool {
		return func() bool { return strings.TrimSpace(viper.GetString(key)) == "" }
	}

	set("steamcmd.install_dir", candidate.InstallDir, blank("steamcmd.install_dir"))
	if candidate.SteamCMDPath != "" {
		set("steamcmd.executable", candidate.SteamCMDPath, blank("steamcmd.executable"))
	}
	set("palworld.config_path", candidate.ConfigPath, blank("palworld.config_path"))
	if candidate.SavePath != "" {
		set("save.path", candidate.SavePath, func() bool {
			value := strings.TrimSpace(viper.GetString("save.path"))
			return value == "" || value == "/path/to/your/Pal/Saved"
		})
	}
	set("palworld.control.mode", "process", func() bool {
		mode := strings.ToLower(strings.TrimSpace(viper.GetString("palworld.control.mode")))
		return mode == "" || mode == "disabled"
	})
	set("palworld.control.target", candidate.LauncherPath, blank("palworld.control.target"))
	set("palworld.control.working_directory", candidate.InstallDir, blank("palworld.control.working_directory"))

	if candidate.RESTPort <= 0 {
		candidate.RESTPort = 8212
	}
	set("rest.address", fmt.Sprintf("http://127.0.0.1:%d", candidate.RESTPort), blank("rest.address"))
	if candidate.adminPassword != "" {
		set("rest.password", candidate.adminPassword, blank("rest.password"))
		set("rcon.password", candidate.adminPassword, blank("rcon.password"))
	}
	if candidate.RCONPort > 0 {
		set("rcon.address", fmt.Sprintf("127.0.0.1:%d", candidate.RCONPort), blank("rcon.address"))
	}
	return config.ApplyValuesWithRuntime(values)
}

type candidateSeed struct {
	installDir   string
	source       string
	manifestPath string
	steamCMDPath string
}

func discoverCandidates() []Candidate {
	seeds := configuredSeeds()
	for _, installDir := range platformRunningInstallDirs() {
		seeds = append(seeds, candidateSeed{installDir: installDir, source: "running-process"})
	}
	for _, root := range platformSteamRoots() {
		seeds = append(seeds, steamSeeds(root)...)
	}
	for _, installDir := range platformDirectInstallDirs() {
		seeds = append(seeds, candidateSeed{installDir: installDir, source: "common-path"})
	}

	seen := make(map[string]Candidate)
	for _, seed := range seeds {
		candidate, ok := inspectCandidatePath(seed.installDir, seed.source, seed.manifestPath)
		if !ok {
			continue
		}
		if seed.steamCMDPath != "" {
			candidate.SteamCMDPath = seed.steamCMDPath
			candidate.Score += 3
		} else if steamCMDPath := inferSteamCMDPath(candidate.InstallDir); steamCMDPath != "" {
			candidate.SteamCMDPath = steamCMDPath
			candidate.Score += 3
		}
		key := candidate.InstallDir
		if runtime.GOOS == "windows" {
			key = strings.ToLower(key)
		}
		if previous, exists := seen[key]; !exists || candidate.Score > previous.Score {
			seen[key] = candidate
		}
	}

	candidates := make([]Candidate, 0, len(seen))
	for _, candidate := range seen {
		candidates = append(candidates, candidate)
	}
	sort.Slice(candidates, func(i, j int) bool {
		if candidates[i].Score == candidates[j].Score {
			return candidates[i].InstallDir < candidates[j].InstallDir
		}
		return candidates[i].Score > candidates[j].Score
	})
	return candidates
}

func inferSteamCMDPath(installDir string) string {
	commonDir := filepath.Dir(installDir)
	steamAppsDir := filepath.Dir(commonDir)
	if !strings.EqualFold(filepath.Base(commonDir), "common") || !strings.EqualFold(filepath.Base(steamAppsDir), "steamapps") {
		return ""
	}
	return platformSteamCMDPath(filepath.Dir(steamAppsDir))
}

func inspectCandidatePath(path, source, manifestPath string) (Candidate, bool) {
	if installDir := findInstallAncestor(path); installDir != "" {
		path = installDir
	}
	return inspectInstallDir(path, source, manifestPath)
}

func configuredSeeds() []candidateSeed {
	values := []struct {
		path   string
		source string
	}{
		{viper.GetString("steamcmd.install_dir"), "configured-steamcmd"},
		{viper.GetString("mods.install_dir"), "configured-mods"},
		{viper.GetString("palworld.control.working_directory"), "configured-control"},
		{filepath.Dir(viper.GetString("palworld.control.target")), "configured-control"},
		{viper.GetString("save.path"), "configured-save"},
		{filepath.Dir(viper.GetString("palworld.config_path")), "configured-config"},
	}
	seeds := make([]candidateSeed, 0, len(values))
	for _, value := range values {
		path := strings.TrimSpace(value.path)
		if path == "" || path == "." {
			continue
		}
		if installDir := findInstallAncestor(path); installDir != "" {
			seeds = append(seeds, candidateSeed{installDir: installDir, source: value.source})
		}
	}
	return seeds
}

func findInstallAncestor(path string) string {
	absolute, err := filepath.Abs(path)
	if err != nil {
		return ""
	}
	current := filepath.Clean(absolute)
	if info, statErr := os.Stat(current); statErr == nil && !info.IsDir() {
		current = filepath.Dir(current)
	}
	for depth := 0; depth < 8; depth++ {
		if validLauncher(filepath.Join(current, platformLauncherName())) {
			return current
		}
		parent := filepath.Dir(current)
		if parent == current {
			break
		}
		current = parent
	}
	return ""
}

func steamSeeds(root string) []candidateSeed {
	root = normalizeSteamRoot(root)
	if root == "" {
		return nil
	}
	libraries := []string{root}
	libraryFile := filepath.Join(root, "steamapps", "libraryfolders.vdf")
	if content, err := os.ReadFile(libraryFile); err == nil && len(content) <= 4<<20 {
		for _, match := range vdfPathPattern.FindAllStringSubmatch(string(content), -1) {
			if len(match) == 2 {
				libraries = append(libraries, decodeVDFPath(match[1]))
			}
		}
	}

	seen := make(map[string]struct{})
	seeds := make([]candidateSeed, 0, len(libraries))
	for _, library := range libraries {
		library = normalizeSteamRoot(library)
		if library == "" {
			continue
		}
		key := library
		if runtime.GOOS == "windows" {
			key = strings.ToLower(key)
		}
		if _, exists := seen[key]; exists {
			continue
		}
		seen[key] = struct{}{}
		manifest := filepath.Join(library, "steamapps", "appmanifest_"+palServerAppID+".acf")
		installName := "PalServer"
		if content, err := os.ReadFile(manifest); err == nil && len(content) <= 1<<20 {
			if match := manifestDirPattern.FindStringSubmatch(string(content)); len(match) == 2 {
				installName = decodeVDFPath(match[1])
			}
		} else {
			manifest = ""
		}
		seeds = append(seeds, candidateSeed{
			installDir:   filepath.Join(library, "steamapps", "common", installName),
			source:       "steam-library",
			manifestPath: manifest,
			steamCMDPath: platformSteamCMDPath(library),
		})
	}
	return seeds
}

func normalizeSteamRoot(path string) string {
	path = strings.TrimSpace(path)
	if path == "" {
		return ""
	}
	path = filepath.Clean(path)
	if strings.EqualFold(filepath.Base(path), "steamapps") {
		path = filepath.Dir(path)
	}
	absolute, err := filepath.Abs(path)
	if err != nil {
		return ""
	}
	return filepath.Clean(absolute)
}

func decodeVDFPath(value string) string {
	value = strings.ReplaceAll(value, `\\`, `\`)
	value = strings.ReplaceAll(value, `\/`, `/`)
	return value
}

func inspectInstallDir(path, source, manifestPath string) (Candidate, bool) {
	path = strings.TrimSpace(path)
	if path == "" {
		return Candidate{}, false
	}
	absolute, err := filepath.Abs(path)
	if err != nil {
		return Candidate{}, false
	}
	installDir := filepath.Clean(absolute)
	if resolved, resolveErr := filepath.EvalSymlinks(installDir); resolveErr == nil {
		installDir = filepath.Clean(resolved)
	}
	launcher := filepath.Join(installDir, platformLauncherName())
	if !validLauncher(launcher) {
		return Candidate{}, false
	}

	savedDir := filepath.Join(installDir, "Pal", "Saved")
	configPath := filepath.Join(savedDir, "Config", platformConfigDirectory(), "PalWorldSettings.ini")
	worlds, dedicatedID := discoverWorlds(savedDir, platformConfigDirectory())
	savePath := ""
	if len(worlds) > 0 {
		savePath = worlds[0].Path
		if dedicatedID != "" {
			for _, world := range worlds {
				if strings.EqualFold(world.ID, dedicatedID) {
					savePath = world.Path
					break
				}
			}
		}
	} else if info, statErr := os.Stat(savedDir); statErr == nil && info.IsDir() {
		savePath = savedDir
	}

	candidate := Candidate{
		Platform:         runtime.GOOS,
		Source:           source,
		InstallDir:       installDir,
		LauncherPath:     launcher,
		ConfigPath:       configPath,
		SavedDir:         savedDir,
		SavePath:         savePath,
		ManifestPath:     manifestPath,
		Worlds:           worlds,
		Score:            50,
		dedicatedWorldID: dedicatedID,
		RESTPort:         8212,
		RCONPort:         25575,
	}
	if manifestPath != "" {
		candidate.Score += 20
	}
	if info, statErr := os.Stat(configPath); statErr == nil && info.Mode().IsRegular() {
		candidate.ConfigExists = true
		candidate.Score += 15
		readServerSettings(configPath, &candidate)
	}
	if len(worlds) > 0 {
		candidate.Score += 25 + min(len(worlds), 5)
	}
	if strings.HasPrefix(source, "configured-") {
		candidate.Score += 30
	}
	if source == "running-process" {
		candidate.Score += 45
	}
	hash := sha256.Sum256([]byte(strings.ToLower(installDir)))
	candidate.ID = hex.EncodeToString(hash[:8])
	return candidate, true
}

func validLauncher(path string) bool {
	info, err := os.Lstat(path)
	if err != nil || !info.Mode().IsRegular() || info.Mode()&os.ModeSymlink != 0 || info.Size() == 0 {
		return false
	}
	return runtime.GOOS == "windows" || info.Mode().Perm()&0o111 != 0
}

func discoverWorlds(savedDir, configDirectory string) ([]World, string) {
	worldRoot := filepath.Join(savedDir, "SaveGames", "0")
	entries, err := os.ReadDir(worldRoot)
	if err != nil {
		return nil, readDedicatedServerName(savedDir, configDirectory)
	}
	worlds := make([]World, 0, len(entries))
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		worldPath := filepath.Join(worldRoot, entry.Name())
		levelPath := filepath.Join(worldPath, "Level.sav")
		info, statErr := os.Lstat(levelPath)
		if statErr != nil || !info.Mode().IsRegular() || info.Mode()&os.ModeSymlink != 0 || info.Size() == 0 {
			continue
		}
		worlds = append(worlds, World{ID: entry.Name(), Path: worldPath, ModifiedAt: info.ModTime().UTC()})
	}
	sort.Slice(worlds, func(i, j int) bool { return worlds[i].ModifiedAt.After(worlds[j].ModifiedAt) })
	return worlds, readDedicatedServerName(savedDir, configDirectory)
}

func readDedicatedServerName(savedDir, configDirectory string) string {
	path := filepath.Join(savedDir, "Config", configDirectory, "GameUserSettings.ini")
	content, err := os.ReadFile(path)
	if err != nil || len(content) > 2<<20 {
		return ""
	}
	match := dedicatedNamePattern.FindStringSubmatch(string(content))
	if len(match) != 2 {
		return ""
	}
	return strings.TrimSpace(match[1])
}

func readServerSettings(path string, candidate *Candidate) {
	content, err := os.ReadFile(path)
	if err != nil || len(content) > 4<<20 {
		return
	}
	text := string(content)
	candidate.RESTEnabled = parseBoolSetting(text, "RESTAPIEnabled")
	candidate.RCONEnabled = parseBoolSetting(text, "RCONEnabled")
	if port := parseIntSetting(text, "RESTAPIPort"); port > 0 && port <= 65535 {
		candidate.RESTPort = port
	}
	if port := parseIntSetting(text, "RCONPort"); port > 0 && port <= 65535 {
		candidate.RCONPort = port
	}
	candidate.adminPassword = parseStringSetting(text, "AdminPassword")
}

func parseBoolSetting(text, key string) bool {
	pattern := regexp.MustCompile(`(?i)(?:^|[,\(])\s*` + regexp.QuoteMeta(key) + `\s*=\s*(True|False)`)
	match := pattern.FindStringSubmatch(text)
	return len(match) == 2 && strings.EqualFold(match[1], "true")
}

func parseIntSetting(text, key string) int {
	pattern := regexp.MustCompile(`(?i)(?:^|[,\(])\s*` + regexp.QuoteMeta(key) + `\s*=\s*([0-9]+)`)
	match := pattern.FindStringSubmatch(text)
	if len(match) != 2 {
		return 0
	}
	value, _ := strconv.Atoi(match[1])
	return value
}

func parseStringSetting(text, key string) string {
	pattern := regexp.MustCompile(`(?i)(?:^|[,\(])\s*` + regexp.QuoteMeta(key) + `\s*=\s*("(?:\\.|[^"])*")`)
	match := pattern.FindStringSubmatch(text)
	if len(match) != 2 {
		return ""
	}
	value, err := strconv.Unquote(match[1])
	if err != nil {
		return ""
	}
	return value
}
