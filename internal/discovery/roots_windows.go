//go:build windows

package discovery

import (
	"os"
	"path/filepath"
	"strings"
	"unsafe"

	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/registry"
)

func platformLauncherName() string { return "PalServer.exe" }

func platformConfigDirectory() string { return "WindowsServer" }

func platformSteamRoots() []string {
	roots := []string{os.Getenv("STEAM_PATH")}
	for _, environmentName := range []string{"ProgramFiles(x86)", "ProgramFiles"} {
		if base := strings.TrimSpace(os.Getenv(environmentName)); base != "" {
			roots = append(roots, filepath.Join(base, "Steam"))
		}
	}
	queries := []struct {
		root registry.Key
		path string
		name string
	}{
		{registry.CURRENT_USER, `Software\Valve\Steam`, "SteamPath"},
		{registry.CURRENT_USER, `Software\Valve\Steam`, "InstallPath"},
		{registry.LOCAL_MACHINE, `Software\WOW6432Node\Valve\Steam`, "InstallPath"},
		{registry.LOCAL_MACHINE, `Software\Valve\Steam`, "InstallPath"},
	}
	for _, query := range queries {
		key, err := registry.OpenKey(query.root, query.path, registry.QUERY_VALUE)
		if err != nil {
			continue
		}
		value, _, valueErr := key.GetStringValue(query.name)
		key.Close()
		if valueErr == nil {
			roots = append(roots, value)
		}
	}
	return compactPaths(roots)
}

func platformDirectInstallDirs() []string {
	paths := []string{
		os.Getenv("PALSERVER_PATH"),
		os.Getenv("PALWORLD_SERVER_PATH"),
		os.Getenv("STEAMCMD_INSTALL_DIR"),
	}
	if workingDir, err := os.Getwd(); err == nil {
		paths = append(paths, workingDir, filepath.Join(workingDir, "PalServer"))
	}
	if executable, err := os.Executable(); err == nil {
		dir := filepath.Dir(executable)
		paths = append(paths, dir, filepath.Join(dir, "PalServer"), filepath.Join(filepath.Dir(dir), "PalServer"))
	}
	for _, drive := range windowsDriveRoots() {
		paths = append(paths,
			filepath.Join(drive, "PalServer"),
			filepath.Join(drive, "steamcmd", "steamapps", "common", "PalServer"),
			filepath.Join(drive, "SteamLibrary", "steamapps", "common", "PalServer"),
			filepath.Join(drive, "Steam", "steamapps", "common", "PalServer"),
		)
	}
	return compactPaths(paths)
}

func platformRunningInstallDirs() []string {
	snapshot, err := windows.CreateToolhelp32Snapshot(windows.TH32CS_SNAPPROCESS, 0)
	if err != nil {
		return nil
	}
	defer windows.CloseHandle(snapshot)

	entry := windows.ProcessEntry32{Size: uint32(unsafe.Sizeof(windows.ProcessEntry32{}))}
	if err := windows.Process32First(snapshot, &entry); err != nil {
		return nil
	}
	paths := make([]string, 0, 2)
	for {
		name := strings.ToLower(windows.UTF16ToString(entry.ExeFile[:]))
		if name == "palserver.exe" || strings.HasPrefix(name, "palserver-win64-shipping") {
			if imagePath := processImagePath(entry.ProcessID); imagePath != "" {
				if installDir := findInstallAncestor(imagePath); installDir != "" {
					paths = append(paths, installDir)
				}
			}
		}
		if err := windows.Process32Next(snapshot, &entry); err != nil {
			break
		}
	}
	return compactPaths(paths)
}

func processImagePath(processID uint32) string {
	process, err := windows.OpenProcess(windows.PROCESS_QUERY_LIMITED_INFORMATION, false, processID)
	if err != nil {
		return ""
	}
	defer windows.CloseHandle(process)
	buffer := make([]uint16, windows.MAX_LONG_PATH)
	size := uint32(len(buffer))
	if err := windows.QueryFullProcessImageName(process, 0, &buffer[0], &size); err != nil || size == 0 {
		return ""
	}
	return windows.UTF16ToString(buffer[:size])
}

func platformSteamCMDPath(libraryRoot string) string {
	candidates := []string{
		filepath.Join(libraryRoot, "steamcmd.exe"),
		filepath.Join(filepath.Dir(libraryRoot), "steamcmd.exe"),
	}
	for _, candidate := range candidates {
		if validLauncher(candidate) {
			return candidate
		}
	}
	return ""
}

func windowsDriveRoots() []string {
	roots := make([]string, 0, 8)
	drives, err := windows.GetLogicalDrives()
	if err != nil {
		return roots
	}
	for letter := 'C'; letter <= 'Z'; letter++ {
		if drives&(1<<uint(letter-'A')) == 0 {
			continue
		}
		root := string(letter) + `:\`
		rootPointer, pointerErr := windows.UTF16PtrFromString(root)
		if pointerErr != nil {
			continue
		}
		driveType := windows.GetDriveType(rootPointer)
		if driveType == windows.DRIVE_FIXED || driveType == windows.DRIVE_REMOVABLE {
			roots = append(roots, root)
		}
	}
	return roots
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
		key := strings.ToLower(value)
		if _, exists := seen[key]; exists {
			continue
		}
		seen[key] = struct{}{}
		result = append(result, value)
	}
	return result
}
