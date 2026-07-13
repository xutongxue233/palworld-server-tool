package tool

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"

	dockerclient "github.com/moby/moby/client"
	"github.com/spf13/viper"
	"github.com/zaigie/palworld-server-tool/internal/logger"
)

var (
	ErrServerControlNotConfigured = errors.New("palworld.control is not configured")
	serverControlMu               sync.Mutex
	managedProcessMu              sync.Mutex
	managedProcess                *os.Process
	controlPollInterval           = time.Second
	controlSaveWorld              = SaveWorld
	controlShutdown               = Shutdown
	controlRESTStop               = StopServer
	serverOnlineProbe             = func() bool {
		_, err := Info()
		return err == nil
	}
	serverControlDriverFactory = newServerControlDriver
)

type ServerControlStatus struct {
	Configured bool   `json:"configured"`
	Mode       string `json:"mode"`
	Target     string `json:"target,omitempty"`
	Online     bool   `json:"online"`
	Running    bool   `json:"running"`
	State      string `json:"state"`
	Detail     string `json:"detail,omitempty"`
}

type serverControlConfig struct {
	Mode             string
	Target           string
	Arguments        []string
	WorkingDirectory string
	Timeout          time.Duration
}

type serverControlDriver interface {
	Start(context.Context) error
	Stop(context.Context) error
	Status(context.Context) (bool, string, error)
}

func getServerControlConfig() (serverControlConfig, error) {
	mode := strings.ToLower(strings.TrimSpace(viper.GetString("palworld.control.mode")))
	if mode == "" || mode == "disabled" {
		return serverControlConfig{Mode: "disabled"}, ErrServerControlNotConfigured
	}
	target := strings.TrimSpace(viper.GetString("palworld.control.target"))
	if target == "" {
		return serverControlConfig{}, errors.New("palworld.control.target is required")
	}
	timeout := viper.GetInt("palworld.control.timeout")
	if timeout == 0 {
		timeout = 120
	}
	if timeout < 10 || timeout > 900 {
		return serverControlConfig{}, errors.New("palworld.control.timeout must be between 10 and 900 seconds")
	}
	return serverControlConfig{
		Mode:             mode,
		Target:           target,
		Arguments:        viper.GetStringSlice("palworld.control.arguments"),
		WorkingDirectory: strings.TrimSpace(viper.GetString("palworld.control.working_directory")),
		Timeout:          time.Duration(timeout) * time.Second,
	}, nil
}

func GetServerControlStatus(ctx context.Context) ServerControlStatus {
	config, configErr := getServerControlConfig()
	status := ServerControlStatus{
		Configured: configErr == nil,
		Mode:       config.Mode,
		Target:     config.Target,
		Online:     serverOnlineProbe(),
		State:      "unconfigured",
	}
	if configErr != nil {
		if !errors.Is(configErr, ErrServerControlNotConfigured) {
			status.Detail = configErr.Error()
		}
		return status
	}
	driver, err := serverControlDriverFactory(config)
	if err != nil {
		status.Detail = err.Error()
		status.State = "invalid"
		return status
	}
	status.Running, status.State, err = driver.Status(ctx)
	if err != nil {
		status.Detail = err.Error()
	}
	if status.Online {
		status.Running = true
		status.State = "online"
	}
	return status
}

func StartManagedServer(ctx context.Context) error {
	serverControlMu.Lock()
	defer serverControlMu.Unlock()

	config, err := getServerControlConfig()
	if err != nil {
		return err
	}
	if serverOnlineProbe() {
		return errors.New("Palworld server is already online")
	}
	driver, err := serverControlDriverFactory(config)
	if err != nil {
		return err
	}
	operationCtx, cancel := context.WithTimeout(ctx, config.Timeout)
	defer cancel()
	running, _, statusErr := driver.Status(operationCtx)
	if statusErr != nil || !running {
		if err := driver.Start(operationCtx); err != nil {
			return err
		}
	}
	return waitForServerState(operationCtx, true)
}

func RestartManagedServer(ctx context.Context, seconds int, message string) error {
	serverControlMu.Lock()
	defer serverControlMu.Unlock()

	config, err := getServerControlConfig()
	if err != nil {
		return err
	}
	driver, err := serverControlDriverFactory(config)
	if err != nil {
		return err
	}
	if seconds <= 0 {
		seconds = 10
	}
	if seconds > 300 {
		return errors.New("restart delay cannot exceed 300 seconds")
	}
	if serverOnlineProbe() {
		if err := controlSaveWorld(); err != nil {
			return fmt.Errorf("save world before restart: %w", err)
		}
		if err := controlShutdown(seconds, message); err != nil {
			return fmt.Errorf("request graceful shutdown: %w", err)
		}
		shutdownCtx, shutdownCancel := context.WithTimeout(ctx, time.Duration(seconds+30)*time.Second)
		shutdownErr := waitForServerState(shutdownCtx, false)
		shutdownCancel()
		if shutdownErr != nil {
			stopCtx, stopCancel := context.WithTimeout(ctx, 30*time.Second)
			stopErr := driver.Stop(stopCtx)
			stopCancel()
			if stopErr != nil {
				return fmt.Errorf("wait for shutdown: %v; managed stop: %w", shutdownErr, stopErr)
			}
		}
	}
	if serverOnlineProbe() {
		return nil
	}
	startCtx, startCancel := context.WithTimeout(ctx, config.Timeout)
	defer startCancel()
	if config.Mode == "process" {
		processExitCtx, processExitCancel := context.WithTimeout(startCtx, 5*time.Second)
		processExitErr := waitForDriverStopped(processExitCtx, driver)
		processExitCancel()
		if processExitErr != nil {
			if err := driver.Stop(startCtx); err != nil {
				return fmt.Errorf("Palworld process stayed alive after REST shutdown: %w", err)
			}
		}
	}
	running, _, statusErr := driver.Status(startCtx)
	if statusErr != nil || !running {
		if err := driver.Start(startCtx); err != nil {
			return err
		}
	}
	return waitForServerState(startCtx, true)
}

func waitForDriverStopped(ctx context.Context, driver serverControlDriver) error {
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()
	for {
		running, _, err := driver.Status(ctx)
		if err != nil {
			return err
		}
		if !running {
			return nil
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
		}
	}
}

func ForceStopManagedServer(ctx context.Context) error {
	serverControlMu.Lock()
	defer serverControlMu.Unlock()

	restErr := controlRESTStop()
	if restErr == nil {
		return nil
	}
	config, configErr := getServerControlConfig()
	if configErr != nil {
		return restErr
	}
	driver, err := serverControlDriverFactory(config)
	if err != nil {
		return fmt.Errorf("REST stop failed: %v; managed control failed: %w", restErr, err)
	}
	operationCtx, cancel := context.WithTimeout(ctx, config.Timeout)
	defer cancel()
	if err := driver.Stop(operationCtx); err != nil {
		return fmt.Errorf("REST stop failed: %v; managed stop failed: %w", restErr, err)
	}
	return nil
}

func waitForServerState(ctx context.Context, online bool) error {
	ticker := time.NewTicker(controlPollInterval)
	defer ticker.Stop()
	for {
		if serverOnlineProbe() == online {
			return nil
		}
		select {
		case <-ctx.Done():
			state := "offline"
			if online {
				state = "online"
			}
			return fmt.Errorf("timed out waiting for Palworld server to become %s: %w", state, ctx.Err())
		case <-ticker.C:
		}
	}
}

func newServerControlDriver(config serverControlConfig) (serverControlDriver, error) {
	switch config.Mode {
	case "process":
		if !filepath.IsAbs(config.Target) {
			return nil, errors.New("process control target must be an absolute executable path")
		}
		return &processControlDriver{config: config}, nil
	case "docker":
		return &dockerControlDriver{target: config.Target}, nil
	case "systemd":
		if runtime.GOOS == "windows" {
			return nil, errors.New("systemd control is not available on Windows")
		}
		return &commandControlDriver{command: "systemctl", target: config.Target}, nil
	case "windows_service":
		if runtime.GOOS != "windows" {
			return nil, errors.New("windows_service control is only available on Windows")
		}
		return &commandControlDriver{command: "sc.exe", target: config.Target}, nil
	default:
		return nil, fmt.Errorf("unsupported palworld.control.mode %q", config.Mode)
	}
}

type processControlDriver struct {
	config serverControlConfig
}

func (d *processControlDriver) Start(_ context.Context) error {
	if info, err := os.Stat(d.config.Target); err != nil || info.IsDir() {
		if err == nil {
			err = errors.New("target is a directory")
		}
		return fmt.Errorf("invalid process control target: %w", err)
	}
	cmd := exec.Command(d.config.Target, d.config.Arguments...)
	cmd.Dir = d.config.WorkingDirectory
	if cmd.Dir == "" {
		cmd.Dir = filepath.Dir(d.config.Target)
	}
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("start Palworld process: %w", err)
	}
	managedProcessMu.Lock()
	managedProcess = cmd.Process
	managedProcessMu.Unlock()
	go func(process *os.Process) {
		err := cmd.Wait()
		managedProcessMu.Lock()
		if managedProcess == process {
			managedProcess = nil
		}
		managedProcessMu.Unlock()
		if err != nil {
			logger.Errorf("managed Palworld process exited: %v\n", err)
		}
	}(cmd.Process)
	return nil
}

func (d *processControlDriver) Stop(_ context.Context) error {
	managedProcessMu.Lock()
	process := managedProcess
	if process == nil {
		managedProcessMu.Unlock()
		return errors.New("no Palworld process started by this tool is being tracked")
	}
	managedProcess = nil
	managedProcessMu.Unlock()
	if err := process.Kill(); err != nil {
		managedProcessMu.Lock()
		if managedProcess == nil {
			managedProcess = process
		}
		managedProcessMu.Unlock()
		return err
	}
	return nil
}

func (d *processControlDriver) Status(_ context.Context) (bool, string, error) {
	managedProcessMu.Lock()
	running := managedProcess != nil
	managedProcessMu.Unlock()
	if running {
		return true, "starting", nil
	}
	return false, "stopped", nil
}

type commandControlDriver struct {
	command string
	target  string
}

func (d *commandControlDriver) Start(ctx context.Context) error {
	return runControlCommand(ctx, d.command, "start", d.target)
}

func (d *commandControlDriver) Stop(ctx context.Context) error {
	return runControlCommand(ctx, d.command, "stop", d.target)
}

func (d *commandControlDriver) Status(ctx context.Context) (bool, string, error) {
	if d.command == "systemctl" {
		cmd := exec.CommandContext(ctx, d.command, "is-active", d.target)
		output, err := cmd.CombinedOutput()
		state := strings.TrimSpace(string(output))
		if state == "active" || state == "activating" || state == "inactive" || state == "failed" {
			return state == "active" || state == "activating", state, nil
		}
		return false, state, err
	}
	cmd := exec.CommandContext(ctx, d.command, "query", d.target)
	output, err := cmd.CombinedOutput()
	state := strings.TrimSpace(string(output))
	return strings.Contains(state, "RUNNING"), state, err
}

func runControlCommand(ctx context.Context, name string, args ...string) error {
	output, err := exec.CommandContext(ctx, name, args...).CombinedOutput()
	if err != nil {
		detail := strings.TrimSpace(string(output))
		if len(detail) > 500 {
			detail = detail[:500]
		}
		if detail != "" {
			return fmt.Errorf("%s %s: %w", name, detail, err)
		}
		return fmt.Errorf("%s: %w", name, err)
	}
	return nil
}

type dockerControlDriver struct {
	target string
}

func newDockerClient() (*dockerclient.Client, error) {
	if version := strings.TrimSpace(os.Getenv("DOCKER_API_VERSION")); version != "" {
		return dockerclient.NewClientWithOpts(dockerclient.FromEnv, dockerclient.WithVersion(version))
	}
	return dockerclient.NewClientWithOpts(dockerclient.FromEnv)
}

func (d *dockerControlDriver) Start(ctx context.Context) error {
	cli, err := newDockerClient()
	if err != nil {
		return err
	}
	defer cli.Close()
	_, err = cli.ContainerStart(ctx, d.target, dockerclient.ContainerStartOptions{})
	return err
}

func (d *dockerControlDriver) Stop(ctx context.Context) error {
	cli, err := newDockerClient()
	if err != nil {
		return err
	}
	defer cli.Close()
	timeout := 30
	_, err = cli.ContainerStop(ctx, d.target, dockerclient.ContainerStopOptions{Timeout: &timeout})
	return err
}

func (d *dockerControlDriver) Status(ctx context.Context) (bool, string, error) {
	cli, err := newDockerClient()
	if err != nil {
		return false, "unknown", err
	}
	defer cli.Close()
	result, err := cli.ContainerInspect(ctx, d.target, dockerclient.ContainerInspectOptions{})
	if err != nil {
		return false, "unknown", err
	}
	if result.Container.State == nil {
		return false, "unknown", nil
	}
	return result.Container.State.Running, string(result.Container.State.Status), nil
}
