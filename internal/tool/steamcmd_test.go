package tool

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/spf13/viper"
)

func TestInspectSteamCMDRequiresConfiguration(t *testing.T) {
	viper.Reset()
	t.Cleanup(viper.Reset)

	plan := InspectSteamCMD()
	if plan.Configured || plan.CanExecute {
		t.Fatalf("unconfigured SteamCMD should not be executable: %#v", plan)
	}
	if plan.AppID != PalworldDedicatedServerAppID || len(plan.PlanDigest) != 64 {
		t.Fatalf("unexpected fixed plan identity: %#v", plan)
	}
	assertSteamIssue(t, plan, "steamcmd.executable")
	assertSteamIssue(t, plan, "steamcmd.install_dir")
}

func TestInspectSteamCMDFreshInstall(t *testing.T) {
	executable, installDir := configureSteamCMDTest(t)

	plan := InspectSteamCMD()
	if !plan.Configured || !plan.CanExecute || plan.Installed || plan.PartialInstallation {
		t.Fatalf("unexpected fresh-install plan: %#v", plan)
	}
	if plan.ExecutablePath != executable || plan.InstallDir != installDir {
		t.Fatalf("configured paths were not preserved: %#v", plan)
	}
	if plan.LauncherPath != steamLauncherPath(installDir) || plan.SafetyBackupRequired || plan.SafetyBackupReady {
		t.Fatalf("unexpected fresh-install safety state: %#v", plan)
	}
}

func TestInspectSteamCMDFreshInstallDoesNotBackUpAnUnrelatedConfiguredSave(t *testing.T) {
	_, _ = configureSteamCMDTest(t)
	otherWorld := filepath.Join(t.TempDir(), "SaveGames", "0", "OTHER")
	steamWriteFile(t, filepath.Join(otherWorld, "Level.sav"), []byte("other-world"))
	viper.Set("save.path", otherWorld)

	plan := InspectSteamCMD()
	if !plan.CanExecute || plan.SafetyBackupRequired || plan.SafetyBackupReady {
		t.Fatalf("fresh install should not back up an unrelated save: %#v", plan)
	}
}

func TestInspectSteamCMDInstalledBuildRequiresMatchingWorldBackup(t *testing.T) {
	_, installDir := configureSteamCMDTest(t)
	worldDir := writeSteamInstallFixture(t, installDir, "123456")
	viper.Set("save.path", worldDir)

	plan := InspectSteamCMD()
	if !plan.CanExecute || !plan.Installed || plan.BuildID != "123456" || plan.ExistingWorlds != 1 {
		t.Fatalf("unexpected installed plan: %#v", plan)
	}
	if !plan.SafetyBackupRequired || !plan.SafetyBackupReady {
		t.Fatalf("installed world must have a ready safety backup: %#v", plan)
	}
}

func TestInspectSteamCMDRejectsBackupFromDifferentInstallation(t *testing.T) {
	_, installDir := configureSteamCMDTest(t)
	writeSteamInstallFixture(t, installDir, "123456")
	otherWorld := filepath.Join(t.TempDir(), "SaveGames", "0", "OTHER")
	steamWriteFile(t, filepath.Join(otherWorld, "Level.sav"), []byte("other-world"))
	viper.Set("save.path", otherWorld)

	plan := InspectSteamCMD()
	if plan.CanExecute || plan.SafetyBackupReady {
		t.Fatalf("an unrelated save must not satisfy the safety-backup requirement: %#v", plan)
	}
	assertSteamIssue(t, plan, "does not point to a world inside steamcmd.install_dir")
}

func TestInspectSteamCMDDetectsPartialAndInvalidInstallation(t *testing.T) {
	_, installDir := configureSteamCMDTest(t)
	steamWriteFile(t, steamLauncherPath(installDir), []byte("launcher"))
	steamWriteFile(t, filepath.Join(installDir, "steamapps", "appmanifest_2394010.acf"), []byte(`"AppState" { "appid" "999" "buildid" "123" }`))

	plan := InspectSteamCMD()
	if plan.CanExecute || plan.Installed || !plan.PartialInstallation {
		t.Fatalf("invalid manifest should be reported as a partial installation: %#v", plan)
	}
	assertSteamIssue(t, plan, "does not describe Palworld Dedicated Server")
}

func TestInspectSteamCMDRejectsUnsafePathsAndTimeout(t *testing.T) {
	executable, installDir := configureSteamCMDTest(t)
	root := filepath.VolumeName(installDir) + string(os.PathSeparator)

	for _, test := range []struct {
		name       string
		executable string
		installDir string
		timeout    int
		issue      string
	}{
		{name: "relative executable", executable: filepath.Base(executable), installDir: installDir, timeout: 1800, issue: "steamcmd.executable must be an absolute path"},
		{name: "relative install", executable: executable, installDir: "palserver", timeout: 1800, issue: "steamcmd.install_dir must be an absolute path"},
		{name: "filesystem root", executable: executable, installDir: root, timeout: 1800, issue: "cannot be a filesystem root"},
		{name: "short timeout", executable: executable, installDir: installDir, timeout: 59, issue: "steamcmd.timeout must be between 60 and 7200 seconds"},
	} {
		t.Run(test.name, func(t *testing.T) {
			viper.Set("steamcmd.executable", test.executable)
			viper.Set("steamcmd.install_dir", test.installDir)
			viper.Set("steamcmd.timeout", test.timeout)
			plan := InspectSteamCMD()
			if plan.CanExecute {
				t.Fatalf("unsafe plan was executable: %#v", plan)
			}
			assertSteamIssue(t, plan, test.issue)
		})
	}
}

func TestInspectSteamCMDRejectsSymbolicLinks(t *testing.T) {
	executable, installDir := configureSteamCMDTest(t)
	linkedExecutable := filepath.Join(t.TempDir(), filepath.Base(executable))
	if err := os.Symlink(executable, linkedExecutable); err != nil {
		t.Skipf("symbolic links are not available: %v", err)
	}
	viper.Set("steamcmd.executable", linkedExecutable)
	plan := InspectSteamCMD()
	if plan.CanExecute {
		t.Fatalf("symbolic-link executable was accepted: %#v", plan)
	}
	assertSteamIssue(t, plan, "not a symbolic link")

	viper.Set("steamcmd.executable", executable)
	realInstallParent := filepath.Join(t.TempDir(), "real")
	if err := os.MkdirAll(realInstallParent, 0o755); err != nil {
		t.Fatal(err)
	}
	linkedInstallParent := filepath.Join(t.TempDir(), "linked")
	if err := os.Symlink(realInstallParent, linkedInstallParent); err != nil {
		t.Skipf("directory symbolic links are not available: %v", err)
	}
	viper.Set("steamcmd.install_dir", filepath.Join(linkedInstallParent, filepath.Base(installDir)))
	plan = InspectSteamCMD()
	if plan.CanExecute {
		t.Fatalf("symbolic-link install path was accepted: %#v", plan)
	}
	assertSteamIssue(t, plan, "must be a real directory")
}

func TestInspectSteamCMDRejectsSymbolicLinksInsideInstallation(t *testing.T) {
	_, installDir := configureSteamCMDTest(t)
	writeSteamInstalledFiles(t, installDir, "123456")

	outsideSaveGames := filepath.Join(t.TempDir(), "SaveGames")
	outsideWorld := filepath.Join(outsideSaveGames, "0", "WORLD")
	steamWriteFile(t, filepath.Join(outsideWorld, "Level.sav"), []byte("outside world"))
	savedDir := filepath.Join(installDir, "Pal", "Saved")
	if err := os.MkdirAll(savedDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outsideSaveGames, filepath.Join(savedDir, "SaveGames")); err != nil {
		t.Skipf("directory symbolic links are not available: %v", err)
	}
	viper.Set("save.path", filepath.Join(savedDir, "SaveGames", "0", "WORLD"))
	plan := InspectSteamCMD()
	if plan.CanExecute || plan.SafetyBackupReady {
		t.Fatalf("world path through an internal symbolic link was accepted: %#v", plan)
	}
	assertSteamIssue(t, plan, "symbolic-link")
}

func TestInspectSteamCMDRejectsSymbolicLinkManifestDirectory(t *testing.T) {
	_, installDir := configureSteamCMDTest(t)
	if err := os.MkdirAll(installDir, 0o755); err != nil {
		t.Fatal(err)
	}
	outsideSteamApps := filepath.Join(t.TempDir(), "steamapps")
	manifest := fmt.Sprintf(`"AppState" { "appid" "%d" "buildid" "123456" }`, PalworldDedicatedServerAppID)
	steamWriteFile(t, filepath.Join(outsideSteamApps, "appmanifest_2394010.acf"), []byte(manifest))
	if err := os.Symlink(outsideSteamApps, filepath.Join(installDir, "steamapps")); err != nil {
		t.Skipf("directory symbolic links are not available: %v", err)
	}
	steamWriteFile(t, steamLauncherPath(installDir), []byte("Palworld launcher"))
	if runtime.GOOS != "windows" {
		if err := os.Chmod(steamLauncherPath(installDir), 0o700); err != nil {
			t.Fatal(err)
		}
	}
	plan := InspectSteamCMD()
	if plan.CanExecute || plan.Installed {
		t.Fatalf("manifest through a symbolic-link directory was accepted: %#v", plan)
	}
	assertSteamIssue(t, plan, "symbolic-link")
}

func TestInspectSteamCMDRequiresExecutablePermissionsOnUnix(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Windows does not use Unix executable permission bits")
	}
	executable, installDir := configureSteamCMDTest(t)
	if err := os.Chmod(executable, 0o600); err != nil {
		t.Fatal(err)
	}
	plan := InspectSteamCMD()
	if plan.CanExecute {
		t.Fatalf("non-executable SteamCMD was accepted: %#v", plan)
	}
	assertSteamIssue(t, plan, "executable permission bit")

	if err := os.Chmod(executable, 0o700); err != nil {
		t.Fatal(err)
	}
	writeSteamInstalledFiles(t, installDir, "123456")
	if err := os.Chmod(steamLauncherPath(installDir), 0o600); err != nil {
		t.Fatal(err)
	}
	plan = InspectSteamCMD()
	if plan.CanExecute || plan.Installed {
		t.Fatalf("non-executable Palworld launcher was accepted: %#v", plan)
	}
	assertSteamIssue(t, plan, "launcher must have an executable permission bit")
}

func TestInspectSteamCMDRequiresEXEOnWindows(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("Windows executable extension rule")
	}
	_, installDir := configureSteamCMDTest(t)
	batchFile := filepath.Join(t.TempDir(), "steamcmd.cmd")
	steamWriteFile(t, batchFile, []byte("@echo off"))
	viper.Set("steamcmd.executable", batchFile)
	viper.Set("steamcmd.install_dir", installDir)
	plan := InspectSteamCMD()
	if plan.CanExecute {
		t.Fatalf("Windows command script was accepted as SteamCMD: %#v", plan)
	}
	assertSteamIssue(t, plan, "must be an .exe file")
}

func TestRunSteamCMDUpdateUsesOnlyFixedArguments(t *testing.T) {
	for _, validate := range []bool{false, true} {
		t.Run(fmt.Sprintf("validate_%t", validate), func(t *testing.T) {
			executable, installDir := configureSteamCMDTest(t)
			plan := InspectSteamCMD()
			called := false
			steamCMDRunner = func(ctx context.Context, actualExecutable string, args []string) (string, error) {
				called = true
				if err := ctx.Err(); err != nil {
					t.Fatalf("runner received an expired context: %v", err)
				}
				if actualExecutable != executable {
					t.Fatalf("unexpected executable: %q", actualExecutable)
				}
				expected := []string{
					"+force_install_dir", installDir,
					"+login", "anonymous",
					"+app_update", "2394010",
				}
				if validate {
					expected = append(expected, "validate")
				}
				expected = append(expected, "+quit")
				if !reflect.DeepEqual(args, expected) {
					t.Fatalf("unexpected SteamCMD arguments:\nwant %#v\n got %#v", expected, args)
				}
				writeSteamInstalledFiles(t, installDir, "200001")
				return "SteamCMD completed", nil
			}

			result, err := RunSteamCMDUpdate(context.Background(), SteamCMDUpdateOptions{
				ExpectedPlanDigest: plan.PlanDigest,
				ValidateFiles:      validate,
			})
			if err != nil {
				t.Fatal(err)
			}
			if !called || !result.After.Installed || result.BuildIDAfter != "200001" || !result.Changed {
				t.Fatalf("unexpected update result: %#v", result)
			}
			if result.Validated != validate || result.OutputTail != "SteamCMD completed" {
				t.Fatalf("result did not preserve execution details: %#v", result)
			}
		})
	}
}

func TestRunSteamCMDUpdateRejectsChangedPlanBeforeExecution(t *testing.T) {
	executable, _ := configureSteamCMDTest(t)
	plan := InspectSteamCMD()
	steamWriteFile(t, executable, []byte("changed executable"))
	called := false
	steamCMDRunner = func(context.Context, string, []string) (string, error) {
		called = true
		return "", nil
	}

	_, err := RunSteamCMDUpdate(context.Background(), SteamCMDUpdateOptions{ExpectedPlanDigest: plan.PlanDigest})
	if !errors.Is(err, ErrSteamCMDPlanChanged) || called {
		t.Fatalf("changed plan was not rejected before execution: err=%v called=%v", err, called)
	}
}

func TestRunSteamCMDUpdateReportsRunnerAndVerificationFailures(t *testing.T) {
	t.Run("runner failure", func(t *testing.T) {
		_, _ = configureSteamCMDTest(t)
		plan := InspectSteamCMD()
		steamCMDRunner = func(context.Context, string, []string) (string, error) {
			return "last useful line", errors.New("exit status 8")
		}
		result, err := RunSteamCMDUpdate(context.Background(), SteamCMDUpdateOptions{ExpectedPlanDigest: plan.PlanDigest})
		if !errors.Is(err, ErrSteamCMDUpdateFailed) || result.OutputTail != "last useful line" {
			t.Fatalf("unexpected runner failure: result=%#v err=%v", result, err)
		}
	})

	t.Run("missing installed files", func(t *testing.T) {
		_, _ = configureSteamCMDTest(t)
		plan := InspectSteamCMD()
		steamCMDRunner = func(context.Context, string, []string) (string, error) { return "done", nil }
		result, err := RunSteamCMDUpdate(context.Background(), SteamCMDUpdateOptions{ExpectedPlanDigest: plan.PlanDigest})
		if !errors.Is(err, ErrSteamCMDUpdateFailed) || result.After.Installed {
			t.Fatalf("missing post-update files were accepted: result=%#v err=%v", result, err)
		}
	})
}

func TestSteamTailBufferKeepsBoundedValidTail(t *testing.T) {
	buffer := &steamTailBuffer{limit: 8}
	_, _ = buffer.Write([]byte("012345"))
	_, _ = buffer.Write([]byte("6789"))
	if got := buffer.String(); got != "23456789" {
		t.Fatalf("unexpected output tail: %q", got)
	}
	_, _ = buffer.Write([]byte{0xff, 'o', 'k'})
	if got := buffer.String(); !strings.HasSuffix(got, "�ok") {
		t.Fatalf("invalid UTF-8 was not sanitized: %q", got)
	}
}

func TestConfirmGameServerStoppedUsesManagedStateOrManualConfirmation(t *testing.T) {
	t.Run("manual confirmation required", func(t *testing.T) {
		configureSteamStopConfirmationTest(t, false)
		if err := ConfirmGameServerStopped(context.Background(), false); !errors.Is(err, ErrSaveEditConfirmation) {
			t.Fatalf("expected confirmation error, got %v", err)
		}
		if err := ConfirmGameServerStopped(context.Background(), true); err != nil {
			t.Fatalf("confirmed manual stop was rejected: %v", err)
		}
	})

	t.Run("online server rejected", func(t *testing.T) {
		configureSteamStopConfirmationTest(t, true)
		if err := ConfirmGameServerStopped(context.Background(), true); !errors.Is(err, ErrGameServerRunning) {
			t.Fatalf("expected running error, got %v", err)
		}
	})

	t.Run("managed stopped state", func(t *testing.T) {
		driver := &fakeControlDriver{stopped: true}
		configureControlTest(t, driver)
		serverOnlineProbe = func() bool { return false }
		if err := ConfirmGameServerStopped(context.Background(), false); err != nil {
			t.Fatalf("verified managed stop was rejected: %v", err)
		}
	})

	t.Run("managed unknown state", func(t *testing.T) {
		configureSteamStopConfirmationTest(t, false)
		viper.Set("palworld.control.mode", "docker")
		viper.Set("palworld.control.target", "palworld")
		viper.Set("palworld.control.timeout", 10)
		oldFactory := serverControlDriverFactory
		serverControlDriverFactory = func(serverControlConfig) (serverControlDriver, error) {
			return steamUnknownControlDriver{}, nil
		}
		t.Cleanup(func() { serverControlDriverFactory = oldFactory })
		if err := ConfirmGameServerStopped(context.Background(), true); !errors.Is(err, ErrGameServerStatusUnknown) {
			t.Fatalf("expected unknown-status error, got %v", err)
		}
	})
}

type steamUnknownControlDriver struct{}

func (steamUnknownControlDriver) Start(context.Context) error { return nil }
func (steamUnknownControlDriver) Stop(context.Context) error  { return nil }
func (steamUnknownControlDriver) Status(context.Context) (bool, string, error) {
	return false, "unknown", nil
}

func configureSteamCMDTest(t *testing.T) (string, string) {
	t.Helper()
	viper.Reset()
	root := t.TempDir()
	executableName := "steamcmd"
	if runtime.GOOS == "windows" {
		executableName = "steamcmd.exe"
	}
	executable := filepath.Join(root, executableName)
	steamWriteFile(t, executable, []byte("test SteamCMD executable"))
	if runtime.GOOS != "windows" {
		if err := os.Chmod(executable, 0o700); err != nil {
			t.Fatal(err)
		}
	}
	installDir := filepath.Join(root, "palworld-server")
	viper.Set("steamcmd.executable", executable)
	viper.Set("steamcmd.install_dir", installDir)
	viper.Set("steamcmd.timeout", 1800)

	oldRunner := steamCMDRunner
	oldNow := steamCMDNow
	baseTime := time.Date(2026, time.July, 13, 10, 0, 0, 0, time.UTC)
	clockCalls := 0
	steamCMDNow = func() time.Time {
		clockCalls++
		return baseTime.Add(time.Duration(clockCalls-1) * time.Second)
	}
	t.Cleanup(func() {
		steamCMDRunner = oldRunner
		steamCMDNow = oldNow
		viper.Reset()
	})
	return filepath.Clean(executable), filepath.Clean(installDir)
}

func configureSteamStopConfirmationTest(t *testing.T, online bool) {
	t.Helper()
	viper.Reset()
	viper.Set("palworld.control.mode", "disabled")
	oldProbe := serverOnlineProbe
	serverOnlineProbe = func() bool { return online }
	t.Cleanup(func() {
		serverOnlineProbe = oldProbe
		viper.Reset()
	})
}

func writeSteamInstallFixture(t *testing.T, installDir, buildID string) string {
	t.Helper()
	writeSteamInstalledFiles(t, installDir, buildID)
	worldDir := filepath.Join(installDir, "Pal", "Saved", "SaveGames", "0", "TESTWORLD")
	steamWriteFile(t, filepath.Join(worldDir, "Level.sav"), []byte("level data"))
	return worldDir
}

func writeSteamInstalledFiles(t *testing.T, installDir, buildID string) {
	t.Helper()
	manifest := fmt.Sprintf(`"AppState" { "appid" "%d" "buildid" "%s" }`, PalworldDedicatedServerAppID, buildID)
	steamWriteFile(t, filepath.Join(installDir, "steamapps", "appmanifest_2394010.acf"), []byte(manifest))
	steamWriteFile(t, steamLauncherPath(installDir), []byte("Palworld launcher"))
	if runtime.GOOS != "windows" {
		if err := os.Chmod(steamLauncherPath(installDir), 0o700); err != nil {
			t.Fatal(err)
		}
	}
}

func steamWriteFile(t *testing.T, path string, content []byte) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, content, 0o600); err != nil {
		t.Fatal(err)
	}
}

func assertSteamIssue(t *testing.T, plan SteamCMDPlan, expected string) {
	t.Helper()
	if !strings.Contains(strings.Join(plan.Issues, "\n"), expected) {
		t.Fatalf("expected issue containing %q, got %#v", expected, plan.Issues)
	}
}
