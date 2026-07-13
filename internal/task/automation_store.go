package task

import (
	"encoding/json"
	"fmt"
	"sort"
	"time"

	"go.etcd.io/bbolt"
)

var (
	automationTasksBucket    = []byte("automation_tasks")
	automationRunsBucket     = []byte("automation_runs")
	automationSettingsBucket = []byte("automation_settings")
	automationSettingsKey    = []byte("settings")
)

const maxStoredTaskRuns = 500

func ensureAutomationBuckets(tx *bbolt.Tx) error {
	for _, name := range [][]byte{
		automationTasksBucket,
		automationRunsBucket,
		automationSettingsBucket,
	} {
		if _, err := tx.CreateBucketIfNotExists(name); err != nil {
			return err
		}
	}
	return nil
}

func putScheduledTask(db *bbolt.DB, scheduledTask ScheduledTask) error {
	return db.Update(func(tx *bbolt.Tx) error {
		if err := ensureAutomationBuckets(tx); err != nil {
			return err
		}
		encoded, err := json.Marshal(scheduledTask)
		if err != nil {
			return err
		}
		return tx.Bucket(automationTasksBucket).Put([]byte(scheduledTask.ID), encoded)
	})
}

func getScheduledTask(db *bbolt.DB, taskID string) (ScheduledTask, error) {
	var scheduledTask ScheduledTask
	err := db.View(func(tx *bbolt.Tx) error {
		bucket := tx.Bucket(automationTasksBucket)
		if bucket == nil {
			return ErrScheduledTaskNotFound
		}
		encoded := bucket.Get([]byte(taskID))
		if encoded == nil {
			return ErrScheduledTaskNotFound
		}
		return json.Unmarshal(encoded, &scheduledTask)
	})
	return scheduledTask, err
}

func listScheduledTasks(db *bbolt.DB) ([]ScheduledTask, error) {
	tasks := make([]ScheduledTask, 0)
	err := db.View(func(tx *bbolt.Tx) error {
		bucket := tx.Bucket(automationTasksBucket)
		if bucket == nil {
			return nil
		}
		return bucket.ForEach(func(_, value []byte) error {
			var scheduledTask ScheduledTask
			if err := json.Unmarshal(value, &scheduledTask); err != nil {
				return err
			}
			tasks = append(tasks, scheduledTask)
			return nil
		})
	})
	if err != nil {
		return nil, err
	}
	sort.Slice(tasks, func(i, j int) bool {
		if tasks[i].CreatedAt.Equal(tasks[j].CreatedAt) {
			return tasks[i].ID < tasks[j].ID
		}
		return tasks[i].CreatedAt.Before(tasks[j].CreatedAt)
	})
	return tasks, nil
}

func deleteScheduledTask(db *bbolt.DB, taskID string) error {
	return db.Update(func(tx *bbolt.Tx) error {
		if err := ensureAutomationBuckets(tx); err != nil {
			return err
		}
		bucket := tx.Bucket(automationTasksBucket)
		if bucket.Get([]byte(taskID)) == nil {
			return ErrScheduledTaskNotFound
		}
		return bucket.Delete([]byte(taskID))
	})
}

func taskRunKey(run TaskRun) []byte {
	return []byte(fmt.Sprintf("%020d:%s", run.StartedAt.UnixNano(), run.ID))
}

func putTaskRun(db *bbolt.DB, run TaskRun) error {
	return db.Update(func(tx *bbolt.Tx) error {
		if err := ensureAutomationBuckets(tx); err != nil {
			return err
		}
		encoded, err := json.Marshal(run)
		if err != nil {
			return err
		}
		bucket := tx.Bucket(automationRunsBucket)
		if err := bucket.Put(taskRunKey(run), encoded); err != nil {
			return err
		}
		for bucket.Stats().KeyN > maxStoredTaskRuns {
			cursor := bucket.Cursor()
			key, _ := cursor.First()
			if key == nil {
				break
			}
			if err := bucket.Delete(key); err != nil {
				return err
			}
		}
		return nil
	})
}

func listTaskRuns(db *bbolt.DB, taskID string, limit int) ([]TaskRun, error) {
	if limit <= 0 {
		limit = 50
	}
	if limit > maxStoredTaskRuns {
		limit = maxStoredTaskRuns
	}
	runs := make([]TaskRun, 0, limit)
	err := db.View(func(tx *bbolt.Tx) error {
		bucket := tx.Bucket(automationRunsBucket)
		if bucket == nil {
			return nil
		}
		cursor := bucket.Cursor()
		for key, value := cursor.Last(); key != nil && len(runs) < limit; key, value = cursor.Prev() {
			var run TaskRun
			if err := json.Unmarshal(value, &run); err != nil {
				return err
			}
			if taskID != "" && run.TaskID != taskID {
				continue
			}
			runs = append(runs, run)
		}
		return nil
	})
	return runs, err
}

func saveAutomationSettings(db *bbolt.DB, settings AutomationSettings) error {
	return db.Update(func(tx *bbolt.Tx) error {
		if err := ensureAutomationBuckets(tx); err != nil {
			return err
		}
		encoded, err := json.Marshal(settings)
		if err != nil {
			return err
		}
		return tx.Bucket(automationSettingsBucket).Put(automationSettingsKey, encoded)
	})
}

func loadAutomationSettings(db *bbolt.DB) (AutomationSettings, bool, error) {
	settings := DefaultAutomationSettings()
	found := false
	err := db.View(func(tx *bbolt.Tx) error {
		bucket := tx.Bucket(automationSettingsBucket)
		if bucket == nil {
			return nil
		}
		encoded := bucket.Get(automationSettingsKey)
		if encoded == nil {
			return nil
		}
		found = true
		return json.Unmarshal(encoded, &settings)
	})
	return settings, found, err
}

func timePointer(value time.Time) *time.Time {
	if value.IsZero() {
		return nil
	}
	copy := value
	return &copy
}
