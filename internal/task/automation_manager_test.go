package task

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"github.com/go-co-op/gocron/v2"
	"github.com/spf13/viper"
	"github.com/zaigie/palworld-server-tool/internal/tool"
	"go.etcd.io/bbolt"
)

func newAutomationTestManager(
	t *testing.T,
	deps automationDependencies,
) (*AutomationManager, gocron.Scheduler) {
	t.Helper()
	viper.Reset()
	t.Cleanup(viper.Reset)
	db := openAutomationTestDB(t)
	scheduler, err := gocron.NewScheduler()
	if err != nil {
		t.Fatal(err)
	}
	manager, err := newAutomationManagerWithDependencies(db, scheduler, deps)
	if err != nil {
		t.Fatal(err)
	}
	scheduler.Start()
	t.Cleanup(func() {
		_ = scheduler.Shutdown()
		manager.Close()
	})
	return manager, scheduler
}

func automationTestDependencies() automationDependencies {
	return automationDependencies{
		now:         func() time.Time { return time.Now().UTC() },
		saveWorld:   func() error { return nil },
		broadcast:   func(string) error { return nil },
		startServer: func(context.Context) error { return nil },
		stopServer:  func(context.Context, int, string) error { return nil },
		restartServer: func(context.Context, int, string) error {
			return nil
		},
		syncSave: func() error { return nil },
		backup:   func(*bbolt.DB) (string, error) { return "backup.zip", nil },
		serverBusy: func() bool {
			return false
		},
		serverStatus: func(context.Context) tool.ServerControlStatus {
			return tool.ServerControlStatus{Configured: true, Online: true, Running: true, State: "online"}
		},
		recoverServer: func(context.Context) error { return nil },
	}
}

func TestAutomationManagerTaskLifecycle(t *testing.T) {
	deps := automationTestDependencies()
	manager, _ := newAutomationTestManager(t, deps)

	created, err := manager.CreateTask(ScheduledTaskInput{
		Name:     "Hourly save",
		Enabled:  true,
		Action:   ActionSaveWorld,
		Schedule: TaskSchedule{Kind: ScheduleInterval, IntervalMinutes: 60},
	})
	if err != nil {
		t.Fatal(err)
	}
	if created.ID == "" || !created.Enabled {
		t.Fatalf("unexpected created task: %#v", created)
	}
	deadline := time.Now().Add(time.Second)
	for created.NextRunAt == nil && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
		listed, listErr := manager.ListTasks()
		if listErr != nil {
			t.Fatal(listErr)
		}
		created = listed[0]
	}
	if created.NextRunAt == nil {
		t.Fatal("enabled task did not expose its next run")
	}

	updated, err := manager.UpdateTask(created.ID, ScheduledTaskInput{
		Name:     "Paused save",
		Enabled:  false,
		Action:   ActionSaveWorld,
		Schedule: TaskSchedule{Kind: ScheduleDaily, TimeOfDay: "04:00"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if updated.Enabled || updated.NextRunAt != nil {
		t.Fatalf("disabled task remained scheduled: %#v", updated)
	}
	if err := manager.DeleteTask(created.ID); err != nil {
		t.Fatal(err)
	}
	listed, err := manager.ListTasks()
	if err != nil || len(listed) != 0 {
		t.Fatalf("deleted task remained: %#v, %v", listed, err)
	}
}

func TestAutomationManagerExecutesAndRecordsTask(t *testing.T) {
	deps := automationTestDependencies()
	var calls atomic.Int32
	deps.saveWorld = func() error {
		calls.Add(1)
		return nil
	}
	manager, _ := newAutomationTestManager(t, deps)
	created, err := manager.CreateTask(ScheduledTaskInput{
		Name:     "Save now",
		Enabled:  false,
		Action:   ActionSaveWorld,
		Schedule: TaskSchedule{Kind: ScheduleInterval, IntervalMinutes: 60},
	})
	if err != nil {
		t.Fatal(err)
	}
	run, err := manager.RunTask(context.Background(), created.ID)
	if err != nil {
		t.Fatal(err)
	}
	if run.Status != TaskRunSucceeded || calls.Load() != 1 || run.FinishedAt == nil {
		t.Fatalf("unexpected task run: %#v calls=%d", run, calls.Load())
	}
	runs, err := manager.ListRuns(created.ID, 10)
	if err != nil || len(runs) != 1 || runs[0].Status != TaskRunSucceeded {
		t.Fatalf("run history was not persisted: %#v, %v", runs, err)
	}
}

func TestAutomationManagerSkipsConcurrentOperations(t *testing.T) {
	deps := automationTestDependencies()
	started := make(chan struct{})
	release := make(chan struct{})
	deps.saveWorld = func() error {
		close(started)
		<-release
		return nil
	}
	manager, _ := newAutomationTestManager(t, deps)
	created, err := manager.CreateTask(ScheduledTaskInput{
		Name:     "Blocking save",
		Enabled:  false,
		Action:   ActionSaveWorld,
		Schedule: TaskSchedule{Kind: ScheduleInterval, IntervalMinutes: 60},
	})
	if err != nil {
		t.Fatal(err)
	}
	firstDone := make(chan error, 1)
	go func() {
		_, runErr := manager.RunTask(context.Background(), created.ID)
		firstDone <- runErr
	}()
	<-started
	if !manager.Status().Busy {
		t.Fatal("active automation task was not reported as busy")
	}
	second, err := manager.RunTask(context.Background(), created.ID)
	if !errors.Is(err, ErrAutomationBusy) || second.Status != TaskRunSkipped {
		t.Fatalf("concurrent task was not skipped: %#v, %v", second, err)
	}
	close(release)
	if err := <-firstDone; err != nil {
		t.Fatal(err)
	}
	if manager.Status().Busy {
		t.Fatal("automation manager remained busy after the task completed")
	}
}

func TestManualOperationLockWorksWithoutAutomationManager(t *testing.T) {
	SetAutomationManager(nil)
	release, err := BeginManualServerOperation(nil)
	if err != nil {
		t.Fatal(err)
	}
	if serverOperationBusy.Load() == 0 {
		t.Fatal("manual operation was not marked busy")
	}
	if _, err := BeginManualServerOperation(nil); !errors.Is(err, ErrAutomationBusy) {
		t.Fatalf("concurrent manual operation returned %v", err)
	}
	release()
	if serverOperationBusy.Load() != 0 {
		t.Fatal("manual operation remained busy after release")
	}
}

func TestAutomationSettingsAreRedactedAndSecretsCanBeCleared(t *testing.T) {
	manager, _ := newAutomationTestManager(t, automationTestDependencies())
	defaults := DefaultAutomationSettings()
	view, err := manager.UpdateSettings(AutomationSettingsUpdate{
		Watchdog: defaults.Watchdog,
		Notification: NotificationSettingsUpdate{
			Enabled:        true,
			Provider:       NotificationGeneric,
			WebhookURL:     "https://example.com/hooks/private-token",
			Secret:         "signing-secret",
			Events:         []NotificationEvent{EventTaskFailed},
			TimeoutSeconds: 8,
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if !view.Notification.WebhookConfigured || !view.Notification.SecretConfigured ||
		view.Notification.WebhookPreview != "https://example.com/…" {
		t.Fatalf("settings view did not redact secrets: %#v", view.Notification)
	}

	_, err = manager.UpdateSettings(AutomationSettingsUpdate{
		Watchdog: defaults.Watchdog,
		Notification: NotificationSettingsUpdate{
			Enabled:        false,
			Provider:       NotificationGeneric,
			ClearWebhook:   true,
			ClearSecret:    true,
			Events:         []NotificationEvent{EventTaskFailed},
			TimeoutSeconds: 8,
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	settings, _, err := loadAutomationSettings(manager.db)
	if err != nil {
		t.Fatal(err)
	}
	if settings.Notification.WebhookURL != "" || settings.Notification.Secret != "" {
		t.Fatal("notification credentials were not cleared")
	}
}

func TestWatchdogRequiresConfiguredServerControl(t *testing.T) {
	deps := automationTestDependencies()
	deps.serverStatus = func(context.Context) tool.ServerControlStatus {
		return tool.ServerControlStatus{
			Configured: false,
			Detail:     "palworld.control is not configured",
		}
	}
	manager, _ := newAutomationTestManager(t, deps)
	settings := DefaultAutomationSettings()
	settings.Watchdog.Enabled = true

	_, err := manager.UpdateSettings(AutomationSettingsUpdate{
		Watchdog: settings.Watchdog,
		Notification: NotificationSettingsUpdate{
			Provider:       NotificationGeneric,
			Events:         settings.Notification.Events,
			TimeoutSeconds: settings.Notification.TimeoutSeconds,
		},
	})
	if !errors.Is(err, ErrWatchdogControlRequired) {
		t.Fatalf("expected ErrWatchdogControlRequired, got %v", err)
	}
}

func TestWatchdogPausesDuringManualMaintenance(t *testing.T) {
	deps := automationTestDependencies()
	var statusCalls atomic.Int32
	deps.serverStatus = func(context.Context) tool.ServerControlStatus {
		statusCalls.Add(1)
		return tool.ServerControlStatus{Configured: true, Online: false, Running: false, State: "stopped"}
	}
	manager, _ := newAutomationTestManager(t, deps)
	settings := DefaultAutomationSettings()
	settings.Watchdog.Enabled = true
	_, err := manager.UpdateSettings(AutomationSettingsUpdate{
		Watchdog: settings.Watchdog,
		Notification: NotificationSettingsUpdate{
			Provider:       NotificationGeneric,
			Events:         settings.Notification.Events,
			TimeoutSeconds: settings.Notification.TimeoutSeconds,
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	statusCalls.Store(0)
	release, acquired := tryBeginServerOperation()
	if !acquired {
		t.Fatal("failed to begin manual maintenance operation")
	}
	manager.checkWatchdog(context.Background(), manager.deps.now().Add(-time.Second))
	if status := manager.Status().Watchdog; status.State != "maintenance" {
		t.Fatalf("watchdog did not pause for maintenance: %#v", status)
	}
	if statusCalls.Load() != 0 {
		t.Fatalf("watchdog probed the server during maintenance %d times", statusCalls.Load())
	}
	release()
	if manager.Status().Busy {
		t.Fatal("manual maintenance remained busy after release")
	}
}

func TestWatchdogRecoversOnlyAfterFailureThreshold(t *testing.T) {
	deps := automationTestDependencies()
	now := time.Date(2026, 7, 13, 8, 0, 0, 0, time.UTC)
	deps.now = func() time.Time { return now }
	deps.serverStatus = func(context.Context) tool.ServerControlStatus {
		return tool.ServerControlStatus{Configured: true, Online: false, Running: false, State: "stopped"}
	}
	var recoveries atomic.Int32
	deps.recoverServer = func(context.Context) error {
		recoveries.Add(1)
		return nil
	}
	manager, _ := newAutomationTestManager(t, deps)
	settings := DefaultAutomationSettings()
	settings.Watchdog.Enabled = true
	settings.Watchdog.FailureThreshold = 2
	_, err := manager.UpdateSettings(AutomationSettingsUpdate{
		Watchdog: settings.Watchdog,
		Notification: NotificationSettingsUpdate{
			Provider:       NotificationGeneric,
			Events:         settings.Notification.Events,
			TimeoutSeconds: settings.Notification.TimeoutSeconds,
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	graceEnded := now.Add(-time.Second)
	manager.checkWatchdog(context.Background(), graceEnded)
	if status := manager.Status().Watchdog; status.State != "degraded" || recoveries.Load() != 0 {
		t.Fatalf("watchdog recovered too early: %#v", status)
	}
	now = now.Add(30 * time.Second)
	manager.checkWatchdog(context.Background(), graceEnded)
	if status := manager.Status().Watchdog; status.State != "healthy" || recoveries.Load() != 1 {
		t.Fatalf("watchdog did not recover at threshold: %#v recoveries=%d", status, recoveries.Load())
	}
}

func TestWatchdogHonorsIntentionalStopAndRecoveryLimit(t *testing.T) {
	deps := automationTestDependencies()
	now := time.Date(2026, 7, 13, 8, 0, 0, 0, time.UTC)
	deps.now = func() time.Time { return now }
	deps.serverStatus = func(context.Context) tool.ServerControlStatus {
		return tool.ServerControlStatus{Configured: true, Online: false, Running: false, State: "stopped"}
	}
	var recoveries atomic.Int32
	deps.recoverServer = func(context.Context) error {
		recoveries.Add(1)
		return errors.New("start failed")
	}
	manager, _ := newAutomationTestManager(t, deps)
	settings := DefaultAutomationSettings()
	settings.Watchdog.Enabled = true
	settings.Watchdog.FailureThreshold = 2
	settings.Watchdog.MaxRecoveryAttempts = 2
	settings.Watchdog.RestartCooldownSeconds = 30
	_, err := manager.UpdateSettings(AutomationSettingsUpdate{
		Watchdog: settings.Watchdog,
		Notification: NotificationSettingsUpdate{
			Provider:       NotificationGeneric,
			Events:         settings.Notification.Events,
			TimeoutSeconds: settings.Notification.TimeoutSeconds,
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := manager.SetDesiredRunning(false); err != nil {
		t.Fatal(err)
	}
	manager.checkWatchdog(context.Background(), now.Add(-time.Second))
	if status := manager.Status().Watchdog; status.State != "paused" || recoveries.Load() != 0 {
		t.Fatalf("intentional stop was not honored: %#v", status)
	}
	if err := manager.SetDesiredRunning(true); err != nil {
		t.Fatal(err)
	}
	graceEnded := now.Add(-time.Second)
	manager.checkWatchdog(context.Background(), graceEnded)
	manager.checkWatchdog(context.Background(), graceEnded)
	now = now.Add(31 * time.Second)
	manager.checkWatchdog(context.Background(), graceEnded)
	if status := manager.Status().Watchdog; status.State != "exhausted" || recoveries.Load() != 2 {
		t.Fatalf("recovery attempts were not bounded: %#v recoveries=%d", status, recoveries.Load())
	}
}

func TestWatchdogBoundsFlappingSuccessfulRecoveries(t *testing.T) {
	deps := automationTestDependencies()
	now := time.Date(2026, 7, 13, 8, 0, 0, 0, time.UTC)
	deps.now = func() time.Time { return now }
	deps.serverStatus = func(context.Context) tool.ServerControlStatus {
		return tool.ServerControlStatus{Configured: true, Online: false, Running: false, State: "stopped"}
	}
	var recoveries atomic.Int32
	deps.recoverServer = func(context.Context) error {
		recoveries.Add(1)
		return nil
	}
	manager, _ := newAutomationTestManager(t, deps)
	settings := DefaultAutomationSettings()
	settings.Watchdog.Enabled = true
	settings.Watchdog.FailureThreshold = 2
	settings.Watchdog.MaxRecoveryAttempts = 2
	settings.Watchdog.RestartCooldownSeconds = 30
	settings.Watchdog.StartupGraceSeconds = 10
	_, err := manager.UpdateSettings(AutomationSettingsUpdate{
		Watchdog: settings.Watchdog,
		Notification: NotificationSettingsUpdate{
			Provider:       NotificationGeneric,
			Events:         settings.Notification.Events,
			TimeoutSeconds: settings.Notification.TimeoutSeconds,
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	graceEnded := now.Add(-time.Second)
	manager.checkWatchdog(context.Background(), graceEnded)
	manager.checkWatchdog(context.Background(), graceEnded)
	if recoveries.Load() != 1 {
		t.Fatalf("first recovery did not run: %d", recoveries.Load())
	}

	now = now.Add(31 * time.Second)
	manager.checkWatchdog(context.Background(), graceEnded)
	manager.checkWatchdog(context.Background(), graceEnded)
	if recoveries.Load() != 2 {
		t.Fatalf("second recovery did not run: %d", recoveries.Load())
	}

	now = now.Add(31 * time.Second)
	manager.checkWatchdog(context.Background(), graceEnded)
	manager.checkWatchdog(context.Background(), graceEnded)
	status := manager.Status().Watchdog
	if recoveries.Load() != 2 || status.State != "exhausted" {
		t.Fatalf("flapping recovery loop was not bounded: %#v recoveries=%d", status, recoveries.Load())
	}
}
