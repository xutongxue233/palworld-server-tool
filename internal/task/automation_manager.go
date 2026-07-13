package task

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/go-co-op/gocron/v2"
	"github.com/google/uuid"
	"github.com/spf13/viper"
	"github.com/zaigie/palworld-server-tool/internal/logger"
	"github.com/zaigie/palworld-server-tool/internal/tool"
	"go.etcd.io/bbolt"
)

type automationDependencies struct {
	now           func() time.Time
	saveWorld     func() error
	broadcast     func(string) error
	startServer   func(context.Context) error
	stopServer    func(context.Context, int, string) error
	restartServer func(context.Context, int, string) error
	syncSave      func() error
	backup        func(*bbolt.DB) (string, error)
	serverBusy    func() bool
	serverStatus  func(context.Context) tool.ServerControlStatus
	recoverServer func(context.Context) error
}

func defaultAutomationDependencies() automationDependencies {
	return automationDependencies{
		now:         func() time.Time { return time.Now().UTC() },
		saveWorld:   tool.SaveWorld,
		broadcast:   tool.Broadcast,
		startServer: tool.StartManagedServer,
		stopServer: func(ctx context.Context, seconds int, message string) error {
			_, err := tool.StopServerForMaintenance(ctx, seconds, message)
			return err
		},
		restartServer: tool.RestartManagedServer,
		syncSave:      SavSyncNow,
		backup: func(db *bbolt.DB) (string, error) {
			backup, err := tool.BackupAndRecord(db)
			if err != nil {
				return "", err
			}
			return backup.Path, nil
		},
		serverBusy:    tool.IsServerControlBusy,
		serverStatus:  tool.GetServerControlStatus,
		recoverServer: tool.RecoverManagedServer,
	}
}

type AutomationManager struct {
	db        *bbolt.DB
	scheduler gocron.Scheduler
	deps      automationDependencies
	notifier  *webhookNotifier

	mu                 sync.RWMutex
	jobs               map[string]gocron.Job
	settings           AutomationSettings
	activeTaskID       string
	watchdogStatus     WatchdogRuntimeStatus
	notificationStatus NotificationRuntimeStatus
	started            bool

	notificationQueue chan NotificationMessage
	watchdogWake      chan struct{}
	stop              chan struct{}
	stopOnce          sync.Once
	workers           sync.WaitGroup
}

var (
	automationManagerMu sync.RWMutex
	automationManager   *AutomationManager
	serverOperationMu   sync.Mutex
	serverOperationBusy atomic.Int32
)

func NewAutomationManager(db *bbolt.DB, scheduler gocron.Scheduler) (*AutomationManager, error) {
	return newAutomationManagerWithDependencies(db, scheduler, defaultAutomationDependencies())
}

func newAutomationManagerWithDependencies(
	db *bbolt.DB,
	scheduler gocron.Scheduler,
	deps automationDependencies,
) (*AutomationManager, error) {
	if db == nil {
		return nil, errors.New("automation database is required")
	}
	if scheduler == nil {
		return nil, errors.New("automation scheduler is required")
	}
	if deps.now == nil {
		deps.now = func() time.Time { return time.Now().UTC() }
	}

	settings, found, err := loadAutomationSettings(db)
	if err != nil {
		return nil, fmt.Errorf("load automation settings: %w", err)
	}
	if !found {
		settings = automationSettingsFromConfig()
	}
	settings, err = normalizeAutomationSettings(settings)
	if err != nil {
		return nil, fmt.Errorf("validate automation settings: %w", err)
	}
	if !found {
		if err := saveAutomationSettings(db, settings); err != nil {
			return nil, fmt.Errorf("save initial automation settings: %w", err)
		}
	}

	manager := &AutomationManager{
		db:                db,
		scheduler:         scheduler,
		deps:              deps,
		notifier:          newWebhookNotifier(viper.GetBool("automation.notification.allow_private_network")),
		jobs:              make(map[string]gocron.Job),
		settings:          settings,
		notificationQueue: make(chan NotificationMessage, 64),
		watchdogWake:      make(chan struct{}, 1),
		stop:              make(chan struct{}),
	}
	manager.resetRuntimeStatusLocked()

	tasks, err := listScheduledTasks(db)
	if err != nil {
		return nil, fmt.Errorf("load scheduled tasks: %w", err)
	}
	for _, scheduledTask := range tasks {
		input := ScheduledTaskInput{
			Name:       scheduledTask.Name,
			Enabled:    scheduledTask.Enabled,
			Action:     scheduledTask.Action,
			Schedule:   scheduledTask.Schedule,
			Parameters: scheduledTask.Parameters,
		}
		if _, err := normalizeScheduledTaskInput(input); err != nil {
			return nil, fmt.Errorf("validate stored task %s: %w", scheduledTask.ID, err)
		}
		if scheduledTask.Enabled {
			if err := manager.registerTaskLocked(scheduledTask); err != nil {
				return nil, fmt.Errorf("schedule stored task %s: %w", scheduledTask.ID, err)
			}
		}
	}
	return manager, nil
}

func automationSettingsFromConfig() AutomationSettings {
	settings := DefaultAutomationSettings()
	if viper.IsSet("automation.watchdog.enabled") {
		settings.Watchdog.Enabled = viper.GetBool("automation.watchdog.enabled")
	}
	if viper.IsSet("automation.watchdog.desired_running") {
		settings.Watchdog.DesiredRunning = viper.GetBool("automation.watchdog.desired_running")
	}
	if viper.IsSet("automation.watchdog.check_interval_seconds") {
		settings.Watchdog.CheckIntervalSeconds = viper.GetInt("automation.watchdog.check_interval_seconds")
	}
	if viper.IsSet("automation.watchdog.failure_threshold") {
		settings.Watchdog.FailureThreshold = viper.GetInt("automation.watchdog.failure_threshold")
	}
	if viper.IsSet("automation.watchdog.restart_cooldown_seconds") {
		settings.Watchdog.RestartCooldownSeconds = viper.GetInt("automation.watchdog.restart_cooldown_seconds")
	}
	if viper.IsSet("automation.watchdog.max_recovery_attempts") {
		settings.Watchdog.MaxRecoveryAttempts = viper.GetInt("automation.watchdog.max_recovery_attempts")
	}
	if viper.IsSet("automation.watchdog.startup_grace_seconds") {
		settings.Watchdog.StartupGraceSeconds = viper.GetInt("automation.watchdog.startup_grace_seconds")
	}
	if viper.IsSet("automation.notification.enabled") {
		settings.Notification.Enabled = viper.GetBool("automation.notification.enabled")
	}
	if viper.IsSet("automation.notification.provider") {
		settings.Notification.Provider = NotificationProvider(viper.GetString("automation.notification.provider"))
	}
	if viper.IsSet("automation.notification.webhook_url") {
		settings.Notification.WebhookURL = viper.GetString("automation.notification.webhook_url")
	}
	if viper.IsSet("automation.notification.secret") {
		settings.Notification.Secret = viper.GetString("automation.notification.secret")
	}
	if viper.IsSet("automation.notification.timeout_seconds") {
		settings.Notification.TimeoutSeconds = viper.GetInt("automation.notification.timeout_seconds")
	}
	configuredEvents := viper.GetStringSlice("automation.notification.events")
	if len(configuredEvents) > 0 {
		settings.Notification.Events = make([]NotificationEvent, 0, len(configuredEvents))
		for _, event := range configuredEvents {
			settings.Notification.Events = append(settings.Notification.Events, NotificationEvent(event))
		}
	}
	return settings
}

func (manager *AutomationManager) Start() {
	manager.mu.Lock()
	if manager.started {
		manager.mu.Unlock()
		return
	}
	manager.started = true
	manager.mu.Unlock()
	manager.workers.Add(2)
	go manager.notificationWorker()
	go manager.watchdogLoop()
}

func (manager *AutomationManager) Close() {
	manager.stopOnce.Do(func() {
		close(manager.stop)
	})
	manager.workers.Wait()
}

func SetAutomationManager(manager *AutomationManager) {
	automationManagerMu.Lock()
	automationManager = manager
	automationManagerMu.Unlock()
}

func GetAutomationManager() (*AutomationManager, error) {
	automationManagerMu.RLock()
	manager := automationManager
	automationManagerMu.RUnlock()
	if manager == nil {
		return nil, ErrAutomationUnavailable
	}
	return manager, nil
}

func (manager *AutomationManager) CreateTask(input ScheduledTaskInput) (ScheduledTaskView, error) {
	normalized, err := normalizeScheduledTaskInput(input)
	if err != nil {
		return ScheduledTaskView{}, err
	}
	now := manager.deps.now()
	scheduledTask := ScheduledTask{
		ID:         uuid.NewString(),
		Name:       normalized.Name,
		Enabled:    normalized.Enabled,
		Action:     normalized.Action,
		Schedule:   normalized.Schedule,
		Parameters: normalized.Parameters,
		CreatedAt:  now,
		UpdatedAt:  now,
	}

	manager.mu.Lock()
	defer manager.mu.Unlock()
	if err := putScheduledTask(manager.db, scheduledTask); err != nil {
		return ScheduledTaskView{}, err
	}
	if scheduledTask.Enabled {
		if err := manager.registerTaskLocked(scheduledTask); err != nil {
			_ = deleteScheduledTask(manager.db, scheduledTask.ID)
			return ScheduledTaskView{}, err
		}
	}
	return manager.taskViewLocked(scheduledTask, nil), nil
}

func (manager *AutomationManager) UpdateTask(taskID string, input ScheduledTaskInput) (ScheduledTaskView, error) {
	normalized, err := normalizeScheduledTaskInput(input)
	if err != nil {
		return ScheduledTaskView{}, err
	}

	manager.mu.Lock()
	defer manager.mu.Unlock()
	existing, err := getScheduledTask(manager.db, taskID)
	if err != nil {
		return ScheduledTaskView{}, err
	}
	updated := ScheduledTask{
		ID:         existing.ID,
		Name:       normalized.Name,
		Enabled:    normalized.Enabled,
		Action:     normalized.Action,
		Schedule:   normalized.Schedule,
		Parameters: normalized.Parameters,
		CreatedAt:  existing.CreatedAt,
		UpdatedAt:  manager.deps.now(),
	}

	if err := putScheduledTask(manager.db, updated); err != nil {
		return ScheduledTaskView{}, err
	}
	if err := manager.replaceTaskJobLocked(updated); err != nil {
		_ = putScheduledTask(manager.db, existing)
		_ = manager.replaceTaskJobLocked(existing)
		return ScheduledTaskView{}, err
	}
	return manager.taskViewLocked(updated, nil), nil
}

func (manager *AutomationManager) DeleteTask(taskID string) error {
	manager.mu.Lock()
	defer manager.mu.Unlock()
	existing, err := getScheduledTask(manager.db, taskID)
	if err != nil {
		return err
	}
	if job, exists := manager.jobs[taskID]; exists {
		if err := manager.scheduler.RemoveJob(job.ID()); err != nil {
			return err
		}
		delete(manager.jobs, taskID)
	}
	if err := deleteScheduledTask(manager.db, taskID); err != nil {
		if existing.Enabled {
			if restoreErr := manager.registerTaskLocked(existing); restoreErr != nil {
				return errors.Join(err, fmt.Errorf("restore scheduled job: %w", restoreErr))
			}
		}
		return err
	}
	return nil
}

func (manager *AutomationManager) ListTasks() ([]ScheduledTaskView, error) {
	tasks, err := listScheduledTasks(manager.db)
	if err != nil {
		return nil, err
	}
	runs, err := listTaskRuns(manager.db, "", maxStoredTaskRuns)
	if err != nil {
		return nil, err
	}
	lastRuns := make(map[string]TaskRun)
	for _, run := range runs {
		if _, exists := lastRuns[run.TaskID]; !exists {
			lastRuns[run.TaskID] = run
		}
	}
	manager.mu.RLock()
	defer manager.mu.RUnlock()
	views := make([]ScheduledTaskView, 0, len(tasks))
	for _, scheduledTask := range tasks {
		var lastRun *TaskRun
		if run, exists := lastRuns[scheduledTask.ID]; exists {
			copy := run
			lastRun = &copy
		}
		views = append(views, manager.taskViewLocked(scheduledTask, lastRun))
	}
	return views, nil
}

func (manager *AutomationManager) ListRuns(taskID string, limit int) ([]TaskRun, error) {
	return listTaskRuns(manager.db, strings.TrimSpace(taskID), limit)
}

func (manager *AutomationManager) RunTask(ctx context.Context, taskID string) (TaskRun, error) {
	return manager.executeTask(ctx, taskID, TaskTriggerManual)
}

func (manager *AutomationManager) executeScheduledTask(taskID string) error {
	_, err := manager.executeTask(context.Background(), taskID, TaskTriggerScheduled)
	return err
}

func (manager *AutomationManager) executeTask(
	ctx context.Context,
	taskID string,
	trigger TaskRunTrigger,
) (TaskRun, error) {
	scheduledTask, err := getScheduledTask(manager.db, taskID)
	if err != nil {
		return TaskRun{}, err
	}
	run := TaskRun{
		ID:        uuid.NewString(),
		TaskID:    scheduledTask.ID,
		TaskName:  scheduledTask.Name,
		Action:    scheduledTask.Action,
		Trigger:   trigger,
		Status:    TaskRunRunning,
		StartedAt: manager.deps.now(),
	}
	if err := putTaskRun(manager.db, run); err != nil {
		return TaskRun{}, err
	}
	if trigger == TaskTriggerScheduled && !scheduledTask.Enabled {
		return manager.finishTaskRun(run, TaskRunSkipped, "Task is disabled", errors.New("scheduled task is disabled"))
	}
	releaseOperation, acquired := tryBeginServerOperation()
	if !acquired {
		return manager.finishTaskRun(run, TaskRunSkipped, "Another automation operation is running", ErrAutomationBusy)
	}
	defer releaseOperation()
	if manager.deps.serverBusy != nil && manager.deps.serverBusy() {
		return manager.finishTaskRun(run, TaskRunSkipped, "A server control operation is running", ErrAutomationBusy)
	}

	manager.mu.Lock()
	manager.activeTaskID = scheduledTask.ID
	manager.mu.Unlock()
	defer func() {
		manager.mu.Lock()
		if manager.activeTaskID == scheduledTask.ID {
			manager.activeTaskID = ""
		}
		manager.mu.Unlock()
	}()

	summary, actionErr := manager.runAction(ctx, scheduledTask)
	if actionErr != nil {
		return manager.finishTaskRun(run, TaskRunFailed, summary, actionErr)
	}
	return manager.finishTaskRun(run, TaskRunSucceeded, summary, nil)
}

func (manager *AutomationManager) runAction(ctx context.Context, scheduledTask ScheduledTask) (string, error) {
	switch scheduledTask.Action {
	case ActionSaveWorld:
		return "World save requested", manager.deps.saveWorld()
	case ActionBroadcast:
		return "Broadcast sent", manager.deps.broadcast(scheduledTask.Parameters.Message)
	case ActionStart:
		if err := manager.SetDesiredRunning(true); err != nil {
			return "", fmt.Errorf("set watchdog desired state: %w", err)
		}
		return "Managed server started", manager.deps.startServer(ctx)
	case ActionStop:
		if err := manager.SetDesiredRunning(false); err != nil {
			return "", fmt.Errorf("set watchdog desired state: %w", err)
		}
		return "Managed server stopped", manager.deps.stopServer(
			ctx,
			scheduledTask.Parameters.DelaySeconds,
			scheduledTask.Parameters.Message,
		)
	case ActionRestart:
		if err := manager.SetDesiredRunning(true); err != nil {
			return "", fmt.Errorf("set watchdog desired state: %w", err)
		}
		return "Managed server restarted", manager.deps.restartServer(
			ctx,
			scheduledTask.Parameters.DelaySeconds,
			scheduledTask.Parameters.Message,
		)
	case ActionSyncSave:
		return "Decoded save data synchronized", manager.deps.syncSave()
	case ActionBackup:
		path, err := manager.deps.backup(manager.db)
		if err != nil {
			return "", err
		}
		return "PST safety backup created: " + path, nil
	default:
		return "", fmt.Errorf("unsupported automation action %q", scheduledTask.Action)
	}
}

func (manager *AutomationManager) finishTaskRun(
	run TaskRun,
	status TaskRunStatus,
	summary string,
	runErr error,
) (TaskRun, error) {
	finished := manager.deps.now()
	run.Status = status
	run.FinishedAt = &finished
	run.Summary = summary
	if runErr != nil {
		run.Error = runErr.Error()
	}
	if err := putTaskRun(manager.db, run); err != nil {
		if runErr != nil {
			return run, errors.Join(runErr, err)
		}
		return run, err
	}
	switch status {
	case TaskRunSucceeded:
		manager.QueueNotification(NotificationMessage{
			Event:      EventTaskSucceeded,
			OccurredAt: finished,
			Title:      "Scheduled task completed",
			Message:    fmt.Sprintf("%s: %s", run.TaskName, summary),
			Data:       map[string]any{"task_id": run.TaskID, "run_id": run.ID, "action": run.Action},
		})
	case TaskRunFailed:
		manager.QueueNotification(NotificationMessage{
			Event:      EventTaskFailed,
			OccurredAt: finished,
			Title:      "Scheduled task failed",
			Message:    fmt.Sprintf("%s: %s", run.TaskName, run.Error),
			Data:       map[string]any{"task_id": run.TaskID, "run_id": run.ID, "action": run.Action},
		})
	}
	return run, runErr
}

func (manager *AutomationManager) replaceTaskJobLocked(scheduledTask ScheduledTask) error {
	if job, exists := manager.jobs[scheduledTask.ID]; exists {
		if err := manager.scheduler.RemoveJob(job.ID()); err != nil {
			return err
		}
		delete(manager.jobs, scheduledTask.ID)
	}
	if !scheduledTask.Enabled {
		return nil
	}
	return manager.registerTaskLocked(scheduledTask)
}

func (manager *AutomationManager) registerTaskLocked(scheduledTask ScheduledTask) error {
	definition, err := scheduleDefinition(scheduledTask.Schedule)
	if err != nil {
		return err
	}
	job, err := manager.scheduler.NewJob(
		definition,
		gocron.NewTask(manager.executeScheduledTask, scheduledTask.ID),
		gocron.WithName("automation:"+scheduledTask.ID),
		gocron.WithTags("automation", scheduledTask.ID),
		gocron.WithSingletonMode(gocron.LimitModeReschedule),
	)
	if err != nil {
		return err
	}
	manager.jobs[scheduledTask.ID] = job
	return nil
}

func scheduleDefinition(schedule TaskSchedule) (gocron.JobDefinition, error) {
	switch schedule.Kind {
	case ScheduleInterval:
		return gocron.DurationJob(time.Duration(schedule.IntervalMinutes) * time.Minute), nil
	case ScheduleDaily:
		hour, minute, err := parseTimeOfDay(schedule.TimeOfDay)
		if err != nil {
			return nil, err
		}
		return gocron.DailyJob(1, gocron.NewAtTimes(gocron.NewAtTime(uint(hour), uint(minute), 0))), nil
	case ScheduleWeekly:
		hour, minute, err := parseTimeOfDay(schedule.TimeOfDay)
		if err != nil {
			return nil, err
		}
		if len(schedule.Weekdays) == 0 {
			return nil, errors.New("weekly schedule requires at least one weekday")
		}
		weekdays := make([]time.Weekday, 0, len(schedule.Weekdays))
		for _, weekday := range schedule.Weekdays {
			weekdays = append(weekdays, time.Weekday(weekday))
		}
		return gocron.WeeklyJob(
			1,
			gocron.NewWeekdays(weekdays[0], weekdays[1:]...),
			gocron.NewAtTimes(gocron.NewAtTime(uint(hour), uint(minute), 0)),
		), nil
	default:
		return nil, fmt.Errorf("unsupported schedule kind %q", schedule.Kind)
	}
}

func (manager *AutomationManager) taskViewLocked(scheduledTask ScheduledTask, lastRun *TaskRun) ScheduledTaskView {
	view := ScheduledTaskView{
		ScheduledTask: scheduledTask,
		Running:       manager.activeTaskID == scheduledTask.ID,
		LastRun:       lastRun,
	}
	if job, exists := manager.jobs[scheduledTask.ID]; exists {
		if nextRun, err := job.NextRun(); err == nil && !nextRun.IsZero() {
			view.NextRunAt = timePointer(nextRun.UTC())
		}
	}
	return view
}

func (manager *AutomationManager) Settings() AutomationSettingsView {
	manager.mu.RLock()
	defer manager.mu.RUnlock()
	return settingsView(manager.settings)
}

func settingsView(settings AutomationSettings) AutomationSettingsView {
	return AutomationSettingsView{
		Watchdog: settings.Watchdog,
		Notification: NotificationSettingsView{
			Enabled:           settings.Notification.Enabled,
			Provider:          settings.Notification.Provider,
			WebhookConfigured: settings.Notification.WebhookURL != "",
			WebhookPreview:    webhookPreview(settings.Notification.WebhookURL),
			SecretConfigured:  settings.Notification.Secret != "",
			Events:            slices.Clone(settings.Notification.Events),
			TimeoutSeconds:    settings.Notification.TimeoutSeconds,
		},
	}
}

func (manager *AutomationManager) UpdateSettings(update AutomationSettingsUpdate) (AutomationSettingsView, error) {
	manager.mu.Lock()
	defer manager.mu.Unlock()
	settings := manager.settings
	settings.Watchdog = update.Watchdog
	settings.Notification.Enabled = update.Notification.Enabled
	settings.Notification.Provider = update.Notification.Provider
	settings.Notification.Events = slices.Clone(update.Notification.Events)
	settings.Notification.TimeoutSeconds = update.Notification.TimeoutSeconds
	if update.Notification.ClearWebhook {
		settings.Notification.WebhookURL = ""
	} else if strings.TrimSpace(update.Notification.WebhookURL) != "" {
		settings.Notification.WebhookURL = strings.TrimSpace(update.Notification.WebhookURL)
	}
	if update.Notification.ClearSecret {
		settings.Notification.Secret = ""
	} else if update.Notification.Secret != "" {
		settings.Notification.Secret = update.Notification.Secret
	}
	normalized, err := normalizeAutomationSettings(settings)
	if err != nil {
		return AutomationSettingsView{}, err
	}
	if normalized.Watchdog.Enabled {
		status := manager.deps.serverStatus(context.Background())
		if !status.Configured {
			detail := strings.TrimSpace(status.Detail)
			if detail == "" {
				detail = "palworld.control is not configured"
			}
			return AutomationSettingsView{}, fmt.Errorf("%w: %s", ErrWatchdogControlRequired, detail)
		}
	}
	if err := saveAutomationSettings(manager.db, normalized); err != nil {
		return AutomationSettingsView{}, err
	}
	manager.settings = normalized
	manager.resetRuntimeStatusLocked()
	manager.signalWatchdogLocked()
	return settingsView(manager.settings), nil
}

func (manager *AutomationManager) SetDesiredRunning(desired bool) error {
	manager.mu.Lock()
	defer manager.mu.Unlock()
	if manager.settings.Watchdog.DesiredRunning == desired {
		return nil
	}
	settings := manager.settings
	settings.Watchdog.DesiredRunning = desired
	if err := saveAutomationSettings(manager.db, settings); err != nil {
		return err
	}
	manager.settings = settings
	manager.watchdogStatus.DesiredRunning = desired
	manager.watchdogStatus.ConsecutiveFailures = 0
	manager.watchdogStatus.RecoveryAttempts = 0
	manager.watchdogStatus.LastRecoveryAt = nil
	manager.watchdogStatus.LastError = ""
	if !desired {
		manager.watchdogStatus.State = "paused"
	} else if settings.Watchdog.Enabled {
		manager.watchdogStatus.State = "grace"
	}
	manager.signalWatchdogLocked()
	return nil
}

func (manager *AutomationManager) resetRuntimeStatusLocked() {
	manager.watchdogStatus.Enabled = manager.settings.Watchdog.Enabled
	manager.watchdogStatus.DesiredRunning = manager.settings.Watchdog.DesiredRunning
	manager.watchdogStatus.ConsecutiveFailures = 0
	manager.watchdogStatus.RecoveryAttempts = 0
	manager.watchdogStatus.LastRecoveryAt = nil
	manager.watchdogStatus.LastError = ""
	manager.notificationStatus.Enabled = manager.settings.Notification.Enabled
	manager.notificationStatus.Configured = manager.settings.Notification.WebhookURL != ""
	manager.notificationStatus.Provider = string(manager.settings.Notification.Provider)
	manager.notificationStatus.WebhookPreview = webhookPreview(manager.settings.Notification.WebhookURL)
	manager.notificationStatus.SecretConfigured = manager.settings.Notification.Secret != ""
	if !manager.settings.Watchdog.Enabled {
		manager.watchdogStatus.State = "disabled"
	} else if !manager.settings.Watchdog.DesiredRunning {
		manager.watchdogStatus.State = "paused"
	} else {
		manager.watchdogStatus.State = "grace"
	}
}

func (manager *AutomationManager) signalWatchdogLocked() {
	select {
	case manager.watchdogWake <- struct{}{}:
	default:
	}
}

func (manager *AutomationManager) Status() AutomationStatus {
	manager.mu.RLock()
	defer manager.mu.RUnlock()
	return AutomationStatus{
		Location: time.Local.String(),
		Busy: serverOperationBusy.Load() > 0 || manager.activeTaskID != "" ||
			manager.deps.serverBusy != nil && manager.deps.serverBusy(),
		ActiveTaskID: manager.activeTaskID,
		Watchdog:     manager.watchdogStatus,
		Notification: manager.notificationStatus,
	}
}

func (manager *AutomationManager) QueueNotification(message NotificationMessage) bool {
	manager.mu.RLock()
	settings := manager.settings.Notification
	manager.mu.RUnlock()
	if message.Event != EventNotificationTest && !notificationEventEnabled(settings, message.Event) {
		return false
	}
	select {
	case manager.notificationQueue <- message:
		return true
	default:
		manager.mu.Lock()
		manager.notificationStatus.LastError = "notification queue is full"
		manager.mu.Unlock()
		return false
	}
}

func (manager *AutomationManager) TestNotification(ctx context.Context) error {
	manager.mu.RLock()
	settings := manager.settings.Notification
	manager.mu.RUnlock()
	if settings.WebhookURL == "" {
		return errors.New("notification webhook URL is not configured")
	}
	return manager.sendNotification(ctx, settings, NotificationMessage{
		Event:      EventNotificationTest,
		OccurredAt: manager.deps.now(),
		Title:      "Palworld Server Tool test",
		Message:    "Notifications are configured correctly.",
	})
}

func (manager *AutomationManager) notificationWorker() {
	defer manager.workers.Done()
	for {
		select {
		case <-manager.stop:
			return
		case message := <-manager.notificationQueue:
			manager.mu.RLock()
			settings := manager.settings.Notification
			manager.mu.RUnlock()
			if message.Event != EventNotificationTest && !notificationEventEnabled(settings, message.Event) {
				continue
			}
			ctx, cancel := context.WithTimeout(context.Background(), time.Duration(settings.TimeoutSeconds)*time.Second)
			err := manager.sendNotification(ctx, settings, message)
			cancel()
			if err != nil {
				logger.Errorf("automation notification failed: %v\n", err)
			}
		}
	}
}

func (manager *AutomationManager) sendNotification(
	ctx context.Context,
	settings NotificationSettings,
	message NotificationMessage,
) error {
	now := manager.deps.now()
	manager.mu.Lock()
	manager.notificationStatus.LastAttemptAt = timePointer(now)
	manager.mu.Unlock()
	err := manager.notifier.Send(ctx, settings, message)
	manager.mu.Lock()
	defer manager.mu.Unlock()
	if err != nil {
		manager.notificationStatus.LastError = err.Error()
		return err
	}
	manager.notificationStatus.LastSuccessAt = timePointer(manager.deps.now())
	manager.notificationStatus.LastError = ""
	return nil
}

func NotifyAutomationEvent(event NotificationEvent, title, message string, data map[string]any) {
	manager, err := GetAutomationManager()
	if err != nil {
		return
	}
	manager.QueueNotification(NotificationMessage{
		Event:      event,
		OccurredAt: manager.deps.now(),
		Title:      title,
		Message:    message,
		Data:       data,
	})
}

func SetWatchdogDesiredRunning(desired bool) {
	manager, err := GetAutomationManager()
	if err != nil {
		return
	}
	if err := manager.SetDesiredRunning(desired); err != nil {
		logger.Errorf("update watchdog desired state: %v\n", err)
	}
}

func BeginManualServerOperation(desiredRunning *bool) (func(), error) {
	release, acquired := tryBeginServerOperation()
	if !acquired {
		return nil, ErrAutomationBusy
	}
	if desiredRunning != nil {
		manager, err := GetAutomationManager()
		if err == nil {
			if err := manager.SetDesiredRunning(*desiredRunning); err != nil {
				release()
				return nil, err
			}
		}
	}
	return release, nil
}

func tryBeginServerOperation() (func(), bool) {
	if !serverOperationMu.TryLock() {
		return nil, false
	}
	serverOperationBusy.Add(1)
	return func() {
		serverOperationMu.Unlock()
		serverOperationBusy.Add(-1)
	}, true
}
