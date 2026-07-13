package task

import (
	"errors"
	"path/filepath"
	"testing"
	"time"

	"go.etcd.io/bbolt"
)

func openAutomationTestDB(t *testing.T) *bbolt.DB {
	t.Helper()
	db, err := bbolt.Open(filepath.Join(t.TempDir(), "automation.db"), 0600, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	return db
}

func TestAutomationTaskStore(t *testing.T) {
	db := openAutomationTestDB(t)
	now := time.Date(2026, 7, 13, 8, 0, 0, 0, time.UTC)
	task := ScheduledTask{
		ID:        "task-1",
		Name:      "Save",
		Enabled:   true,
		Action:    ActionSaveWorld,
		Schedule:  TaskSchedule{Kind: ScheduleInterval, IntervalMinutes: 60},
		CreatedAt: now,
		UpdatedAt: now,
	}
	if err := putScheduledTask(db, task); err != nil {
		t.Fatal(err)
	}
	loaded, err := getScheduledTask(db, task.ID)
	if err != nil {
		t.Fatal(err)
	}
	if loaded.Name != task.Name || loaded.Action != task.Action {
		t.Fatalf("unexpected task: %#v", loaded)
	}
	tasks, err := listScheduledTasks(db)
	if err != nil || len(tasks) != 1 {
		t.Fatalf("unexpected task list: %#v, %v", tasks, err)
	}
	if err := deleteScheduledTask(db, task.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := getScheduledTask(db, task.ID); !errors.Is(err, ErrScheduledTaskNotFound) {
		t.Fatalf("deleted task lookup returned %v", err)
	}
}

func TestAutomationRunStoreNewestFirst(t *testing.T) {
	db := openAutomationTestDB(t)
	base := time.Date(2026, 7, 13, 8, 0, 0, 0, time.UTC)
	for index := 0; index < 3; index++ {
		run := TaskRun{
			ID:        string(rune('a' + index)),
			TaskID:    "task-1",
			TaskName:  "Save",
			Action:    ActionSaveWorld,
			Trigger:   TaskTriggerScheduled,
			Status:    TaskRunSucceeded,
			StartedAt: base.Add(time.Duration(index) * time.Minute),
		}
		if err := putTaskRun(db, run); err != nil {
			t.Fatal(err)
		}
	}
	runs, err := listTaskRuns(db, "task-1", 2)
	if err != nil {
		t.Fatal(err)
	}
	if len(runs) != 2 || runs[0].ID != "c" || runs[1].ID != "b" {
		t.Fatalf("runs are not newest-first: %#v", runs)
	}
}

func TestAutomationSettingsStore(t *testing.T) {
	db := openAutomationTestDB(t)
	settings, found, err := loadAutomationSettings(db)
	if err != nil || found || settings.Watchdog.CheckIntervalSeconds == 0 {
		t.Fatalf("unexpected defaults: %#v, found=%v, err=%v", settings, found, err)
	}
	settings.Watchdog.Enabled = true
	settings.Notification.WebhookURL = "https://example.com/hook"
	if err := saveAutomationSettings(db, settings); err != nil {
		t.Fatal(err)
	}
	loaded, found, err := loadAutomationSettings(db)
	if err != nil || !found || !loaded.Watchdog.Enabled {
		t.Fatalf("settings were not persisted: %#v, found=%v, err=%v", loaded, found, err)
	}
}
