package task

import (
	"errors"
	"fmt"
	"net/url"
	"slices"
	"strings"
	"time"
)

type AutomationAction string

const (
	ActionSaveWorld AutomationAction = "save_world"
	ActionBroadcast AutomationAction = "broadcast"
	ActionStart     AutomationAction = "start_server"
	ActionStop      AutomationAction = "stop_server"
	ActionRestart   AutomationAction = "restart_server"
	ActionSyncSave  AutomationAction = "sync_save"
	ActionBackup    AutomationAction = "pst_backup"
)

var supportedAutomationActions = []AutomationAction{
	ActionSaveWorld,
	ActionBroadcast,
	ActionStart,
	ActionStop,
	ActionRestart,
	ActionSyncSave,
	ActionBackup,
}

type ScheduleKind string

const (
	ScheduleInterval ScheduleKind = "interval"
	ScheduleDaily    ScheduleKind = "daily"
	ScheduleWeekly   ScheduleKind = "weekly"
)

type TaskSchedule struct {
	Kind            ScheduleKind `json:"kind"`
	IntervalMinutes int          `json:"interval_minutes,omitempty"`
	TimeOfDay       string       `json:"time_of_day,omitempty"`
	Weekdays        []int        `json:"weekdays,omitempty"`
}

type ActionParameters struct {
	Message      string `json:"message,omitempty"`
	DelaySeconds int    `json:"delay_seconds,omitempty"`
}

type ScheduledTaskInput struct {
	Name       string           `json:"name"`
	Enabled    bool             `json:"enabled"`
	Action     AutomationAction `json:"action"`
	Schedule   TaskSchedule     `json:"schedule"`
	Parameters ActionParameters `json:"parameters"`
}

type ScheduledTask struct {
	ID         string           `json:"id"`
	Name       string           `json:"name"`
	Enabled    bool             `json:"enabled"`
	Action     AutomationAction `json:"action"`
	Schedule   TaskSchedule     `json:"schedule"`
	Parameters ActionParameters `json:"parameters"`
	CreatedAt  time.Time        `json:"created_at"`
	UpdatedAt  time.Time        `json:"updated_at"`
}

type TaskRunStatus string

const (
	TaskRunRunning   TaskRunStatus = "running"
	TaskRunSucceeded TaskRunStatus = "succeeded"
	TaskRunFailed    TaskRunStatus = "failed"
	TaskRunSkipped   TaskRunStatus = "skipped"
)

type TaskRunTrigger string

const (
	TaskTriggerScheduled TaskRunTrigger = "scheduled"
	TaskTriggerManual    TaskRunTrigger = "manual"
)

type TaskRun struct {
	ID         string           `json:"id"`
	TaskID     string           `json:"task_id"`
	TaskName   string           `json:"task_name"`
	Action     AutomationAction `json:"action"`
	Trigger    TaskRunTrigger   `json:"trigger"`
	Status     TaskRunStatus    `json:"status"`
	StartedAt  time.Time        `json:"started_at"`
	FinishedAt *time.Time       `json:"finished_at,omitempty"`
	Summary    string           `json:"summary,omitempty"`
	Error      string           `json:"error,omitempty"`
}

type ScheduledTaskView struct {
	ScheduledTask
	NextRunAt *time.Time `json:"next_run_at,omitempty"`
	Running   bool       `json:"running"`
	LastRun   *TaskRun   `json:"last_run,omitempty"`
}

type WatchdogSettings struct {
	Enabled                bool `json:"enabled"`
	DesiredRunning         bool `json:"desired_running"`
	CheckIntervalSeconds   int  `json:"check_interval_seconds"`
	FailureThreshold       int  `json:"failure_threshold"`
	RestartCooldownSeconds int  `json:"restart_cooldown_seconds"`
	MaxRecoveryAttempts    int  `json:"max_recovery_attempts"`
	StartupGraceSeconds    int  `json:"startup_grace_seconds"`
}

type NotificationProvider string

const (
	NotificationGeneric NotificationProvider = "generic"
	NotificationDiscord NotificationProvider = "discord"
)

type NotificationEvent string

const (
	EventTaskSucceeded          NotificationEvent = "task.succeeded"
	EventTaskFailed             NotificationEvent = "task.failed"
	EventServerStarted          NotificationEvent = "server.started"
	EventServerStopped          NotificationEvent = "server.stopped"
	EventServerRestarted        NotificationEvent = "server.restarted"
	EventWatchdogUnhealthy      NotificationEvent = "watchdog.unhealthy"
	EventWatchdogRecovered      NotificationEvent = "watchdog.recovered"
	EventWatchdogRecoveryFailed NotificationEvent = "watchdog.recovery_failed"
	EventNotificationTest       NotificationEvent = "notification.test"
)

var supportedNotificationEvents = []NotificationEvent{
	EventTaskSucceeded,
	EventTaskFailed,
	EventServerStarted,
	EventServerStopped,
	EventServerRestarted,
	EventWatchdogUnhealthy,
	EventWatchdogRecovered,
	EventWatchdogRecoveryFailed,
}

type NotificationSettings struct {
	Enabled        bool                 `json:"enabled"`
	Provider       NotificationProvider `json:"provider"`
	WebhookURL     string               `json:"webhook_url"`
	Secret         string               `json:"secret"`
	Events         []NotificationEvent  `json:"events"`
	TimeoutSeconds int                  `json:"timeout_seconds"`
}

type AutomationSettings struct {
	Watchdog     WatchdogSettings     `json:"watchdog"`
	Notification NotificationSettings `json:"notification"`
}

type NotificationSettingsUpdate struct {
	Enabled        bool                 `json:"enabled"`
	Provider       NotificationProvider `json:"provider"`
	WebhookURL     string               `json:"webhook_url,omitempty"`
	ClearWebhook   bool                 `json:"clear_webhook,omitempty"`
	Secret         string               `json:"secret,omitempty"`
	ClearSecret    bool                 `json:"clear_secret,omitempty"`
	Events         []NotificationEvent  `json:"events"`
	TimeoutSeconds int                  `json:"timeout_seconds"`
}

type AutomationSettingsUpdate struct {
	Watchdog     WatchdogSettings           `json:"watchdog"`
	Notification NotificationSettingsUpdate `json:"notification"`
}

type NotificationSettingsView struct {
	Enabled           bool                 `json:"enabled"`
	Provider          NotificationProvider `json:"provider"`
	WebhookConfigured bool                 `json:"webhook_configured"`
	WebhookPreview    string               `json:"webhook_preview,omitempty"`
	SecretConfigured  bool                 `json:"secret_configured"`
	Events            []NotificationEvent  `json:"events"`
	TimeoutSeconds    int                  `json:"timeout_seconds"`
}

type AutomationSettingsView struct {
	Watchdog     WatchdogSettings         `json:"watchdog"`
	Notification NotificationSettingsView `json:"notification"`
}

type WatchdogRuntimeStatus struct {
	Enabled             bool       `json:"enabled"`
	DesiredRunning      bool       `json:"desired_running"`
	State               string     `json:"state"`
	ConsecutiveFailures int        `json:"consecutive_failures"`
	RecoveryAttempts    int        `json:"recovery_attempts"`
	LastCheckAt         *time.Time `json:"last_check_at,omitempty"`
	LastHealthyAt       *time.Time `json:"last_healthy_at,omitempty"`
	LastRecoveryAt      *time.Time `json:"last_recovery_at,omitempty"`
	NextCheckAt         *time.Time `json:"next_check_at,omitempty"`
	LastError           string     `json:"last_error,omitempty"`
}

type NotificationRuntimeStatus struct {
	Configured       bool       `json:"configured"`
	Enabled          bool       `json:"enabled"`
	Provider         string     `json:"provider,omitempty"`
	WebhookPreview   string     `json:"webhook_preview,omitempty"`
	SecretConfigured bool       `json:"secret_configured"`
	LastAttemptAt    *time.Time `json:"last_attempt_at,omitempty"`
	LastSuccessAt    *time.Time `json:"last_success_at,omitempty"`
	LastError        string     `json:"last_error,omitempty"`
}

type AutomationStatus struct {
	Location     string                    `json:"location"`
	Busy         bool                      `json:"busy"`
	ActiveTaskID string                    `json:"active_task_id,omitempty"`
	Watchdog     WatchdogRuntimeStatus     `json:"watchdog"`
	Notification NotificationRuntimeStatus `json:"notification"`
}

var (
	ErrScheduledTaskNotFound   = errors.New("scheduled task not found")
	ErrAutomationBusy          = errors.New("another automation operation is already running")
	ErrAutomationUnavailable   = errors.New("automation manager is not initialized")
	ErrWatchdogControlRequired = errors.New("watchdog requires managed server control")
)

func DefaultAutomationSettings() AutomationSettings {
	return AutomationSettings{
		Watchdog: WatchdogSettings{
			Enabled:                false,
			DesiredRunning:         true,
			CheckIntervalSeconds:   30,
			FailureThreshold:       3,
			RestartCooldownSeconds: 120,
			MaxRecoveryAttempts:    3,
			StartupGraceSeconds:    90,
		},
		Notification: NotificationSettings{
			Provider: NotificationGeneric,
			Events: []NotificationEvent{
				EventTaskFailed,
				EventWatchdogUnhealthy,
				EventWatchdogRecovered,
				EventWatchdogRecoveryFailed,
			},
			TimeoutSeconds: 10,
		},
	}
}

func normalizeScheduledTaskInput(input ScheduledTaskInput) (ScheduledTaskInput, error) {
	input.Name = strings.TrimSpace(input.Name)
	if input.Name == "" {
		return ScheduledTaskInput{}, errors.New("task name is required")
	}
	if len([]rune(input.Name)) > 80 {
		return ScheduledTaskInput{}, errors.New("task name cannot exceed 80 characters")
	}
	if !slices.Contains(supportedAutomationActions, input.Action) {
		return ScheduledTaskInput{}, fmt.Errorf("unsupported automation action %q", input.Action)
	}

	normalizedSchedule, err := normalizeTaskSchedule(input.Schedule)
	if err != nil {
		return ScheduledTaskInput{}, err
	}
	input.Schedule = normalizedSchedule

	input.Parameters.Message = strings.TrimSpace(input.Parameters.Message)
	if len([]rune(input.Parameters.Message)) > 500 {
		return ScheduledTaskInput{}, errors.New("action message cannot exceed 500 characters")
	}
	switch input.Action {
	case ActionBroadcast:
		if input.Parameters.Message == "" {
			return ScheduledTaskInput{}, errors.New("broadcast message is required")
		}
		input.Parameters.DelaySeconds = 0
	case ActionStop, ActionRestart:
		if input.Parameters.DelaySeconds == 0 {
			input.Parameters.DelaySeconds = 10
		}
		if input.Parameters.DelaySeconds < 1 || input.Parameters.DelaySeconds > 300 {
			return ScheduledTaskInput{}, errors.New("shutdown delay must be between 1 and 300 seconds")
		}
	default:
		input.Parameters = ActionParameters{}
	}
	return input, nil
}

func normalizeTaskSchedule(schedule TaskSchedule) (TaskSchedule, error) {
	switch schedule.Kind {
	case ScheduleInterval:
		if schedule.IntervalMinutes < 5 || schedule.IntervalMinutes > 10080 {
			return TaskSchedule{}, errors.New("interval must be between 5 and 10080 minutes")
		}
		return TaskSchedule{Kind: ScheduleInterval, IntervalMinutes: schedule.IntervalMinutes}, nil
	case ScheduleDaily, ScheduleWeekly:
		if _, _, err := parseTimeOfDay(schedule.TimeOfDay); err != nil {
			return TaskSchedule{}, err
		}
		normalized := TaskSchedule{Kind: schedule.Kind, TimeOfDay: schedule.TimeOfDay}
		if schedule.Kind == ScheduleWeekly {
			if len(schedule.Weekdays) == 0 {
				return TaskSchedule{}, errors.New("weekly schedule requires at least one weekday")
			}
			seen := make(map[int]struct{}, len(schedule.Weekdays))
			for _, weekday := range schedule.Weekdays {
				if weekday < 0 || weekday > 6 {
					return TaskSchedule{}, errors.New("weekday must be between 0 and 6")
				}
				seen[weekday] = struct{}{}
			}
			for weekday := range seen {
				normalized.Weekdays = append(normalized.Weekdays, weekday)
			}
			slices.Sort(normalized.Weekdays)
		}
		return normalized, nil
	default:
		return TaskSchedule{}, fmt.Errorf("unsupported schedule kind %q", schedule.Kind)
	}
}

func parseTimeOfDay(value string) (int, int, error) {
	parsed, err := time.Parse("15:04", value)
	if err != nil || parsed.Format("15:04") != value {
		return 0, 0, errors.New("time_of_day must use HH:MM in 24-hour format")
	}
	return parsed.Hour(), parsed.Minute(), nil
}

func normalizeAutomationSettings(settings AutomationSettings) (AutomationSettings, error) {
	watchdog := settings.Watchdog
	if watchdog.CheckIntervalSeconds < 10 || watchdog.CheckIntervalSeconds > 300 {
		return AutomationSettings{}, errors.New("watchdog check interval must be between 10 and 300 seconds")
	}
	if watchdog.FailureThreshold < 2 || watchdog.FailureThreshold > 10 {
		return AutomationSettings{}, errors.New("watchdog failure threshold must be between 2 and 10")
	}
	if watchdog.RestartCooldownSeconds < 30 || watchdog.RestartCooldownSeconds > 3600 {
		return AutomationSettings{}, errors.New("watchdog restart cooldown must be between 30 and 3600 seconds")
	}
	if watchdog.MaxRecoveryAttempts < 1 || watchdog.MaxRecoveryAttempts > 10 {
		return AutomationSettings{}, errors.New("watchdog max recovery attempts must be between 1 and 10")
	}
	if watchdog.StartupGraceSeconds < 10 || watchdog.StartupGraceSeconds > 900 {
		return AutomationSettings{}, errors.New("watchdog startup grace must be between 10 and 900 seconds")
	}

	notification := settings.Notification
	if notification.Provider == "" {
		notification.Provider = NotificationGeneric
	}
	if notification.Provider != NotificationGeneric && notification.Provider != NotificationDiscord {
		return AutomationSettings{}, fmt.Errorf("unsupported notification provider %q", notification.Provider)
	}
	if notification.TimeoutSeconds == 0 {
		notification.TimeoutSeconds = 10
	}
	if notification.TimeoutSeconds < 3 || notification.TimeoutSeconds > 30 {
		return AutomationSettings{}, errors.New("notification timeout must be between 3 and 30 seconds")
	}
	notification.WebhookURL = strings.TrimSpace(notification.WebhookURL)
	if notification.Enabled && notification.WebhookURL == "" {
		return AutomationSettings{}, errors.New("notification webhook URL is required when notifications are enabled")
	}
	if notification.WebhookURL != "" {
		parsed, err := url.Parse(notification.WebhookURL)
		if err != nil || parsed.Hostname() == "" || parsed.User != nil {
			return AutomationSettings{}, errors.New("notification webhook URL is invalid")
		}
		if parsed.Scheme != "https" {
			return AutomationSettings{}, errors.New("notification webhook URL must use HTTPS")
		}
		if notification.Provider == NotificationDiscord {
			host := strings.ToLower(parsed.Hostname())
			if host != "discord.com" && !strings.HasSuffix(host, ".discord.com") &&
				host != "discordapp.com" && !strings.HasSuffix(host, ".discordapp.com") {
				return AutomationSettings{}, errors.New("Discord webhook URL must use a Discord host")
			}
		}
	}
	if len(notification.Secret) > 256 {
		return AutomationSettings{}, errors.New("notification secret cannot exceed 256 characters")
	}
	seenEvents := make(map[NotificationEvent]struct{}, len(notification.Events))
	normalizedEvents := make([]NotificationEvent, 0, len(notification.Events))
	for _, event := range notification.Events {
		if !slices.Contains(supportedNotificationEvents, event) {
			return AutomationSettings{}, fmt.Errorf("unsupported notification event %q", event)
		}
		if _, exists := seenEvents[event]; exists {
			continue
		}
		seenEvents[event] = struct{}{}
		normalizedEvents = append(normalizedEvents, event)
	}
	slices.Sort(normalizedEvents)
	notification.Events = normalizedEvents
	settings.Notification = notification
	return settings, nil
}

func notificationEventEnabled(settings NotificationSettings, event NotificationEvent) bool {
	return settings.Enabled && slices.Contains(settings.Events, event)
}
