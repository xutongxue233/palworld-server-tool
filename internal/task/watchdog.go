package task

import (
	"context"
	"errors"
	"fmt"
	"time"
)

func (manager *AutomationManager) watchdogLoop() {
	defer manager.workers.Done()
	manager.mu.RLock()
	settings := manager.settings.Watchdog
	manager.mu.RUnlock()
	graceUntil := manager.deps.now().Add(time.Duration(settings.StartupGraceSeconds) * time.Second)
	timer := time.NewTimer(time.Duration(settings.CheckIntervalSeconds) * time.Second)
	defer timer.Stop()
	manager.setNextWatchdogCheck(manager.deps.now().Add(time.Duration(settings.CheckIntervalSeconds) * time.Second))

	for {
		select {
		case <-manager.stop:
			return
		case <-manager.watchdogWake:
			manager.mu.RLock()
			settings = manager.settings.Watchdog
			manager.mu.RUnlock()
			graceUntil = manager.deps.now().Add(time.Duration(settings.StartupGraceSeconds) * time.Second)
			resetTimer(timer, time.Second)
			manager.setNextWatchdogCheck(manager.deps.now().Add(time.Second))
		case <-timer.C:
			ctx, cancel := context.WithTimeout(context.Background(), 15*time.Minute)
			manager.checkWatchdog(ctx, graceUntil)
			cancel()
			manager.mu.RLock()
			settings = manager.settings.Watchdog
			manager.mu.RUnlock()
			delay := time.Duration(settings.CheckIntervalSeconds) * time.Second
			resetTimer(timer, delay)
			manager.setNextWatchdogCheck(manager.deps.now().Add(delay))
		}
	}
}

func resetTimer(timer *time.Timer, duration time.Duration) {
	if !timer.Stop() {
		select {
		case <-timer.C:
		default:
		}
	}
	timer.Reset(duration)
}

func (manager *AutomationManager) setNextWatchdogCheck(next time.Time) {
	manager.mu.Lock()
	manager.watchdogStatus.NextCheckAt = timePointer(next)
	manager.mu.Unlock()
}

func (manager *AutomationManager) checkWatchdog(ctx context.Context, graceUntil time.Time) {
	now := manager.deps.now()
	manager.mu.Lock()
	settings := manager.settings.Watchdog
	manager.watchdogStatus.Enabled = settings.Enabled
	manager.watchdogStatus.DesiredRunning = settings.DesiredRunning
	manager.watchdogStatus.LastCheckAt = timePointer(now)
	manager.watchdogStatus.NextCheckAt = nil
	if !settings.Enabled {
		manager.watchdogStatus.State = "disabled"
		manager.watchdogStatus.ConsecutiveFailures = 0
		manager.watchdogStatus.RecoveryAttempts = 0
		manager.watchdogStatus.LastError = ""
		manager.mu.Unlock()
		return
	}
	if !settings.DesiredRunning {
		manager.watchdogStatus.State = "paused"
		manager.watchdogStatus.ConsecutiveFailures = 0
		manager.watchdogStatus.RecoveryAttempts = 0
		manager.watchdogStatus.LastError = ""
		manager.mu.Unlock()
		return
	}
	if now.Before(graceUntil) {
		manager.watchdogStatus.State = "grace"
		manager.mu.Unlock()
		return
	}
	manager.mu.Unlock()

	if serverOperationBusy.Load() > 0 {
		manager.mu.RLock()
		activeTaskID := manager.activeTaskID
		manager.mu.RUnlock()
		if activeTaskID != "" {
			manager.setWatchdogMaintenanceState("An automation task is running")
		} else {
			manager.setWatchdogMaintenanceState("A maintenance operation is running")
		}
		return
	}
	if manager.deps.serverBusy != nil && manager.deps.serverBusy() {
		manager.setWatchdogMaintenanceState("A server control operation is running")
		return
	}

	status := manager.deps.serverStatus(ctx)
	if !status.Configured {
		detail := status.Detail
		if detail == "" {
			detail = "Managed server control is not configured"
		}
		manager.mu.Lock()
		manager.watchdogStatus.State = "unconfigured"
		manager.watchdogStatus.ConsecutiveFailures = 0
		manager.watchdogStatus.LastError = detail
		manager.mu.Unlock()
		return
	}
	if status.Online {
		manager.markWatchdogHealthy(now, "")
		return
	}

	detail := status.Detail
	if detail == "" && status.Running {
		detail = "Managed server is running but the Palworld REST API is not responding"
	}
	if detail == "" {
		detail = "Managed server is not running"
	}

	manager.mu.Lock()
	manager.watchdogStatus.ConsecutiveFailures++
	failures := manager.watchdogStatus.ConsecutiveFailures
	attempts := manager.watchdogStatus.RecoveryAttempts
	lastRecovery := manager.watchdogStatus.LastRecoveryAt
	previousState := manager.watchdogStatus.State
	manager.watchdogStatus.LastError = detail
	if failures < settings.FailureThreshold {
		manager.watchdogStatus.State = "degraded"
		manager.mu.Unlock()
		return
	}
	manager.watchdogStatus.State = "unhealthy"
	manager.mu.Unlock()

	if failures == settings.FailureThreshold && previousState != "unhealthy" {
		manager.QueueNotification(NotificationMessage{
			Event:      EventWatchdogUnhealthy,
			OccurredAt: now,
			Title:      "Palworld server is unhealthy",
			Message:    detail,
			Data:       map[string]any{"failures": failures, "running": status.Running, "state": status.State},
		})
	}

	if attempts >= settings.MaxRecoveryAttempts {
		manager.mu.Lock()
		manager.watchdogStatus.State = "exhausted"
		manager.mu.Unlock()
		return
	}
	if lastRecovery != nil && now.Sub(*lastRecovery) < time.Duration(settings.RestartCooldownSeconds)*time.Second {
		manager.mu.Lock()
		manager.watchdogStatus.State = "cooldown"
		manager.mu.Unlock()
		return
	}
	releaseOperation, acquired := tryBeginServerOperation()
	if !acquired {
		manager.setWatchdogMaintenanceState("Another automation operation is running")
		return
	}
	defer releaseOperation()
	if manager.deps.serverBusy != nil && manager.deps.serverBusy() {
		manager.setWatchdogMaintenanceState("A server control operation is running")
		return
	}
	manager.mu.RLock()
	stillDesired := manager.settings.Watchdog.Enabled && manager.settings.Watchdog.DesiredRunning
	manager.mu.RUnlock()
	if !stillDesired {
		manager.setWatchdogMaintenanceState("Watchdog recovery was cancelled")
		return
	}

	manager.mu.Lock()
	manager.watchdogStatus.State = "recovering"
	manager.watchdogStatus.RecoveryAttempts++
	attempt := manager.watchdogStatus.RecoveryAttempts
	manager.watchdogStatus.LastRecoveryAt = timePointer(now)
	manager.mu.Unlock()

	err := manager.deps.recoverServer(ctx)
	if err != nil {
		manager.mu.Lock()
		manager.watchdogStatus.LastError = err.Error()
		if attempt >= settings.MaxRecoveryAttempts {
			manager.watchdogStatus.State = "exhausted"
		} else {
			manager.watchdogStatus.State = "unhealthy"
		}
		manager.mu.Unlock()
		manager.QueueNotification(NotificationMessage{
			Event:      EventWatchdogRecoveryFailed,
			OccurredAt: now,
			Title:      "Palworld recovery failed",
			Message:    err.Error(),
			Data:       map[string]any{"attempt": attempt, "max_attempts": settings.MaxRecoveryAttempts},
		})
		return
	}
	manager.markWatchdogHealthy(now, "recovered")
	manager.QueueNotification(NotificationMessage{
		Event:      EventWatchdogRecovered,
		OccurredAt: now,
		Title:      "Palworld server recovered",
		Message:    "The managed server is responding again.",
		Data:       map[string]any{"attempt": attempt},
	})
}

func (manager *AutomationManager) setWatchdogMaintenanceState(detail string) {
	manager.mu.Lock()
	manager.watchdogStatus.State = "maintenance"
	manager.watchdogStatus.LastError = detail
	manager.mu.Unlock()
}

func (manager *AutomationManager) markWatchdogHealthy(now time.Time, source string) {
	manager.mu.Lock()
	wasUnhealthy := manager.watchdogStatus.State == "unhealthy" ||
		manager.watchdogStatus.State == "cooldown" ||
		manager.watchdogStatus.State == "recovering" ||
		manager.watchdogStatus.State == "exhausted"
	manager.watchdogStatus.State = "healthy"
	manager.watchdogStatus.ConsecutiveFailures = 0
	stablePeriod := time.Duration(manager.settings.Watchdog.RestartCooldownSeconds*2) * time.Second
	startupGrace := time.Duration(manager.settings.Watchdog.StartupGraceSeconds) * time.Second
	if startupGrace > stablePeriod {
		stablePeriod = startupGrace
	}
	if manager.watchdogStatus.LastRecoveryAt == nil ||
		now.Sub(*manager.watchdogStatus.LastRecoveryAt) >= stablePeriod {
		manager.watchdogStatus.RecoveryAttempts = 0
		manager.watchdogStatus.LastRecoveryAt = nil
	}
	manager.watchdogStatus.LastHealthyAt = timePointer(now)
	manager.watchdogStatus.LastError = ""
	manager.mu.Unlock()
	if wasUnhealthy && source == "" {
		manager.QueueNotification(NotificationMessage{
			Event:      EventWatchdogRecovered,
			OccurredAt: now,
			Title:      "Palworld server recovered",
			Message:    "The server became healthy without an automatic restart.",
		})
	}
}

func (manager *AutomationManager) ResetWatchdog() {
	manager.mu.Lock()
	manager.watchdogStatus.ConsecutiveFailures = 0
	manager.watchdogStatus.RecoveryAttempts = 0
	manager.watchdogStatus.LastRecoveryAt = nil
	manager.watchdogStatus.LastError = ""
	if !manager.settings.Watchdog.Enabled {
		manager.watchdogStatus.State = "disabled"
	} else if !manager.settings.Watchdog.DesiredRunning {
		manager.watchdogStatus.State = "paused"
	} else {
		manager.watchdogStatus.State = "grace"
	}
	manager.signalWatchdogLocked()
	manager.mu.Unlock()
}

func (manager *AutomationManager) ValidateWatchdogReady() error {
	manager.mu.RLock()
	settings := manager.settings.Watchdog
	manager.mu.RUnlock()
	if !settings.Enabled {
		return errors.New("watchdog is disabled")
	}
	status := manager.deps.serverStatus(context.Background())
	if !status.Configured {
		return fmt.Errorf("managed server control is not configured: %s", status.Detail)
	}
	return nil
}
