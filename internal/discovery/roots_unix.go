//go:build !windows

package discovery

import (
	"os"
	"path/filepath"
	"strings"
)

func platformLauncherName() string { return "PalServer.sh" }

func platformConfigDirectory() string { return "LinuxServer" }

func platformSteamRoots() []string {
	home, _ := os.UserHomeDir()
	roots := []string{
		os.Getenv("STEAM_PATH"),
		"/home/steam/.steam/steam",
		"/home/steam/.local/share/Steam",
		"/root/.steam/steam",
		"/opt/steam",
	}
	if strings.TrimSpace(home) != "" {
		roots = append(roots,
			filepath.Join(home, ".steam", "steam"),
			filepath.Join(home, ".local", "share", "Steam"),
		)
	}
	for _, pattern := range []string{"/home/*/.steam/steam", "/home/*/.local/share/Steam"} {
		if matches, err := filepath.Glob(pattern); err == nil {
			roots = append(roots, matches...)
		}
	}
	return compactPaths(roots)
}

func platformDirectInstallDirs() []string {
	paths := []string{
		os.Getenv("PALSERVER_PATH"),
		os.Getenv("PALWORLD_SERVER_PATH"),
		os.Getenv("STEAMCMD_INSTALL_DIR"),
		"/opt/PalServer",
		"/opt/palworld",
		"/srv/PalServer",
		"/srv/palworld",
		"/home/steam/PalServer",
		"/home/steam/palworld",
	}
	if workingDir, err := os.Getwd(); err == nil {
		paths = append(paths, workingDir, filepath.Join(workingDir, "PalServer"))
	}
	if executable, err := os.Executable(); err == nil {
		dir := filepath.Dir(executable)
		paths = append(paths, dir, filepath.Join(dir, "PalServer"), filepath.Join(filepath.Dir(dir), "PalServer"))
	}
	return compactPaths(paths)
}

func platformRunningInstallDirs() []string {
	processes, err := filepath.Glob("/proc/[0-9]*")
	if err != nil {
		return nil
	}
	paths := make([]string, 0, 2)
	for _, processDir := range processes {
		if executable, resolveErr := filepath.EvalSymlinks(filepath.Join(processDir, "exe")); resolveErr == nil {
			name := strings.ToLower(filepath.Base(executable))
			if name == "palserver" || strings.HasPrefix(name, "palserver-linux-shipping") {
				if installDir := findInstallAncestor(executable); installDir != "" {
					paths = append(paths, installDir)
					continue
				}
			}
		}
		cmdline, readErr := os.ReadFile(filepath.Join(processDir, "cmdline"))
		if readErr != nil || len(cmdline) == 0 || len(cmdline) > 1<<20 {
			continue
		}
		for _, argument := range strings.Split(string(cmdline), "\x00") {
			name := strings.ToLower(filepath.Base(argument))
			if name != "palserver.sh" && !strings.HasPrefix(name, "palserver-linux-shipping") {
				continue
			}
			if !filepath.IsAbs(argument) {
				if workingDir, resolveErr := filepath.EvalSymlinks(filepath.Join(processDir, "cwd")); resolveErr == nil {
					argument = filepath.Join(workingDir, argument)
				}
			}
			if installDir := findInstallAncestor(argument); installDir != "" {
				paths = append(paths, installDir)
				break
			}
		}
	}
	return compactPaths(paths)
}

func platformSteamCMDPath(libraryRoot string) string {
	for _, candidate := range []string{
		filepath.Join(libraryRoot, "steamcmd.sh"),
		filepath.Join(libraryRoot, "steamcmd"),
		filepath.Join(filepath.Dir(libraryRoot), "steamcmd.sh"),
	} {
		if validLauncher(candidate) {
			return candidate
		}
	}
	return ""
}

func compactPaths(values []string) []string {
	seen := make(map[string]struct{})
	result := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" || value == "." {
			continue
		}
		value = filepath.Clean(value)
		if _, exists := seen[value]; exists {
			continue
		}
		seen[value] = struct{}{}
		result = append(result, value)
	}
	return result
}
