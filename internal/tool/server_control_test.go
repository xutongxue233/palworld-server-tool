package tool

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/spf13/viper"
)

type fakeControlDriver struct {
	started bool
	stopped bool
}

func (d *fakeControlDriver) Start(context.Context) error {
	d.started = true
	return nil
}

func (d *fakeControlDriver) Stop(context.Context) error {
	d.stopped = true
	return nil
}

func (d *fakeControlDriver) Status(context.Context) (bool, string, error) {
	return d.started && !d.stopped, "test", nil
}

func configureControlTest(t *testing.T, driver *fakeControlDriver) {
	t.Helper()
	viper.Reset()
	viper.Set("palworld.control.mode", "docker")
	viper.Set("palworld.control.target", "palworld")
	viper.Set("palworld.control.timeout", 10)
	oldFactory := serverControlDriverFactory
	oldProbe := serverOnlineProbe
	oldPoll := controlPollInterval
	oldSave := controlSaveWorld
	oldShutdown := controlShutdown
	oldStop := controlRESTStop
	serverControlDriverFactory = func(serverControlConfig) (serverControlDriver, error) { return driver, nil }
	controlPollInterval = time.Millisecond
	t.Cleanup(func() {
		viper.Reset()
		serverControlDriverFactory = oldFactory
		serverOnlineProbe = oldProbe
		controlPollInterval = oldPoll
		controlSaveWorld = oldSave
		controlShutdown = oldShutdown
		controlRESTStop = oldStop
	})
}

func TestStartManagedServerWaitsForReadiness(t *testing.T) {
	driver := &fakeControlDriver{}
	configureControlTest(t, driver)
	serverOnlineProbe = func() bool { return driver.started }
	if err := StartManagedServer(context.Background()); err != nil {
		t.Fatal(err)
	}
	if !driver.started {
		t.Fatal("driver was not started")
	}
}

func TestRestartManagedServerSavesAndRestarts(t *testing.T) {
	driver := &fakeControlDriver{}
	configureControlTest(t, driver)
	online := true
	saved := false
	serverOnlineProbe = func() bool { return online }
	controlSaveWorld = func() error { saved = true; return nil }
	controlShutdown = func(seconds int, message string) error {
		if seconds != 15 || message != "restart" {
			t.Fatalf("unexpected restart request: %d %q", seconds, message)
		}
		online = false
		return nil
	}
	serverControlDriverFactory = func(serverControlConfig) (serverControlDriver, error) {
		return &restartAwareFakeDriver{fakeControlDriver: driver, online: &online}, nil
	}
	if err := RestartManagedServer(context.Background(), 15, "restart"); err != nil {
		t.Fatal(err)
	}
	if !saved || !driver.started || !online {
		t.Fatalf("restart did not complete: saved=%v started=%v online=%v", saved, driver.started, online)
	}
}

type restartAwareFakeDriver struct {
	*fakeControlDriver
	online *bool
}

func (d *restartAwareFakeDriver) Start(context.Context) error {
	d.started = true
	*d.online = true
	return nil
}

func TestForceStopFallsBackToConfiguredDriver(t *testing.T) {
	driver := &fakeControlDriver{}
	configureControlTest(t, driver)
	controlRESTStop = func() error { return errors.New("REST unavailable") }
	if err := ForceStopManagedServer(context.Background()); err != nil {
		t.Fatal(err)
	}
	if !driver.stopped {
		t.Fatal("managed stop fallback was not used")
	}
}
