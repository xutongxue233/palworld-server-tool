package task

import (
	"reflect"
	"testing"
)

func TestNormalizeScheduledTaskInput(t *testing.T) {
	tests := []struct {
		name      string
		input     ScheduledTaskInput
		wantError bool
	}{
		{
			name: "interval save",
			input: ScheduledTaskInput{
				Name:     " Save world ",
				Enabled:  true,
				Action:   ActionSaveWorld,
				Schedule: TaskSchedule{Kind: ScheduleInterval, IntervalMinutes: 60},
			},
		},
		{
			name: "weekly restart",
			input: ScheduledTaskInput{
				Name:       "Weekly restart",
				Enabled:    true,
				Action:     ActionRestart,
				Schedule:   TaskSchedule{Kind: ScheduleWeekly, TimeOfDay: "04:30", Weekdays: []int{5, 1, 1}},
				Parameters: ActionParameters{DelaySeconds: 30, Message: "Maintenance"},
			},
		},
		{
			name: "broadcast requires message",
			input: ScheduledTaskInput{
				Name:     "Notice",
				Action:   ActionBroadcast,
				Schedule: TaskSchedule{Kind: ScheduleDaily, TimeOfDay: "12:00"},
			},
			wantError: true,
		},
		{
			name: "raw action rejected",
			input: ScheduledTaskInput{
				Name:     "Shell",
				Action:   "shell",
				Schedule: TaskSchedule{Kind: ScheduleInterval, IntervalMinutes: 60},
			},
			wantError: true,
		},
		{
			name: "invalid time",
			input: ScheduledTaskInput{
				Name:     "Bad time",
				Action:   ActionSaveWorld,
				Schedule: TaskSchedule{Kind: ScheduleDaily, TimeOfDay: "4:30"},
			},
			wantError: true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := normalizeScheduledTaskInput(test.input)
			if test.wantError {
				if err == nil {
					t.Fatal("invalid task was accepted")
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if got.Name == "" {
				t.Fatal("normalized task name is empty")
			}
			if got.Schedule.Kind == ScheduleWeekly && !reflect.DeepEqual(got.Schedule.Weekdays, []int{1, 5}) {
				t.Fatalf("weekdays were not normalized: %#v", got.Schedule.Weekdays)
			}
		})
	}
}

func TestNormalizeAutomationSettings(t *testing.T) {
	settings := DefaultAutomationSettings()
	settings.Notification.Enabled = true
	settings.Notification.WebhookURL = "https://example.com/events"
	settings.Notification.Events = append(settings.Notification.Events, EventTaskFailed)
	normalized, err := normalizeAutomationSettings(settings)
	if err != nil {
		t.Fatal(err)
	}
	if len(normalized.Notification.Events) != 4 {
		t.Fatalf("duplicate events were not removed: %#v", normalized.Notification.Events)
	}

	settings.Notification.Provider = NotificationDiscord
	settings.Notification.WebhookURL = "https://example.com/discord"
	if _, err := normalizeAutomationSettings(settings); err == nil {
		t.Fatal("non-Discord host was accepted for a Discord webhook")
	}
}
