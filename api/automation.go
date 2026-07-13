package api

import (
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/zaigie/palworld-server-tool/internal/task"
)

func automationManager(c *gin.Context) (*task.AutomationManager, bool) {
	manager, err := task.GetAutomationManager()
	if err != nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": err.Error()})
		return nil, false
	}
	return manager, true
}

func beginManualOperation(c *gin.Context, desiredRunning *bool) (func(), bool) {
	release, err := task.BeginManualServerOperation(desiredRunning)
	if err != nil {
		status := http.StatusInternalServerError
		if errors.Is(err, task.ErrAutomationBusy) {
			status = http.StatusConflict
		}
		c.JSON(status, gin.H{"error": err.Error()})
		return nil, false
	}
	return release, true
}

// listAutomationTasks godoc
//
//	@Summary		List typed scheduled tasks
//	@Description	List persisted allowlisted maintenance tasks with their next and most recent runs
//	@Tags			Automation
//	@Produce		json
//	@Security		ApiKeyAuth
//	@Success		200	{array}	task.ScheduledTaskView
//	@Failure		401	{object}	ErrorResponse
//	@Failure		503	{object}	ErrorResponse
//	@Router			/api/automation/tasks [get]
func listAutomationTasks(c *gin.Context) {
	manager, ok := automationManager(c)
	if !ok {
		return
	}
	tasks, err := manager.ListTasks()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, tasks)
}

// createAutomationTask godoc
//
//	@Summary		Create a typed scheduled task
//	@Description	Create an interval, daily, or weekly task using an allowlisted action; arbitrary commands and shell scripts are rejected
//	@Tags			Automation
//	@Accept			json
//	@Produce		json
//	@Security		ApiKeyAuth
//	@Param			task	body		task.ScheduledTaskInput	true	"Typed task"
//	@Success		201		{object}	task.ScheduledTaskView
//	@Failure		400		{object}	ErrorResponse
//	@Failure		401		{object}	ErrorResponse
//	@Failure		503		{object}	ErrorResponse
//	@Router			/api/automation/tasks [post]
func createAutomationTask(c *gin.Context) {
	manager, ok := automationManager(c)
	if !ok {
		return
	}
	var input task.ScheduledTaskInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	created, err := manager.CreateTask(input)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, created)
}

// updateAutomationTask godoc
//
//	@Summary		Update a typed scheduled task
//	@Description	Replace one persisted task and immediately reschedule or disable it
//	@Tags			Automation
//	@Accept			json
//	@Produce		json
//	@Security		ApiKeyAuth
//	@Param			task_id	path		string					true	"Task ID"
//	@Param			task	body		task.ScheduledTaskInput	true	"Typed task"
//	@Success		200		{object}	task.ScheduledTaskView
//	@Failure		400		{object}	ErrorResponse
//	@Failure		404		{object}	ErrorResponse
//	@Router			/api/automation/tasks/{task_id} [put]
func updateAutomationTask(c *gin.Context) {
	manager, ok := automationManager(c)
	if !ok {
		return
	}
	var input task.ScheduledTaskInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	updated, err := manager.UpdateTask(c.Param("task_id"), input)
	if err != nil {
		if errors.Is(err, task.ErrScheduledTaskNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, updated)
}

// deleteAutomationTask godoc
//
//	@Summary		Delete a typed scheduled task
//	@Tags			Automation
//	@Produce		json
//	@Security		ApiKeyAuth
//	@Param			task_id	path		string	true	"Task ID"
//	@Success		200		{object}	SuccessResponse
//	@Failure		404		{object}	ErrorResponse
//	@Router			/api/automation/tasks/{task_id} [delete]
func deleteAutomationTask(c *gin.Context) {
	manager, ok := automationManager(c)
	if !ok {
		return
	}
	if err := manager.DeleteTask(c.Param("task_id")); err != nil {
		if errors.Is(err, task.ErrScheduledTaskNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true})
}

// runAutomationTask godoc
//
//	@Summary		Run a typed task now
//	@Description	Execute one allowlisted task immediately while enforcing the global automation operation lock
//	@Tags			Automation
//	@Produce		json
//	@Security		ApiKeyAuth
//	@Param			task_id	path		string	true	"Task ID"
//	@Success		200		{object}	task.TaskRun
//	@Failure		400		{object}	ErrorResponse
//	@Failure		404		{object}	ErrorResponse
//	@Failure		409		{object}	ErrorResponse
//	@Router			/api/automation/tasks/{task_id}/run [post]
func runAutomationTask(c *gin.Context) {
	manager, ok := automationManager(c)
	if !ok {
		return
	}
	run, err := manager.RunTask(c.Request.Context(), c.Param("task_id"))
	if err != nil {
		status := http.StatusBadRequest
		if errors.Is(err, task.ErrScheduledTaskNotFound) {
			status = http.StatusNotFound
		} else if errors.Is(err, task.ErrAutomationBusy) {
			status = http.StatusConflict
		}
		c.JSON(status, gin.H{"error": err.Error(), "run": run})
		return
	}
	c.JSON(http.StatusOK, run)
}

// listAutomationRuns godoc
//
//	@Summary		List automation task runs
//	@Tags			Automation
//	@Produce		json
//	@Security		ApiKeyAuth
//	@Param			task_id	query		string	false	"Optional task ID"
//	@Param			limit	query		int		false	"Maximum records, up to 200"
//	@Success		200		{array}	task.TaskRun
//	@Router			/api/automation/runs [get]
func listAutomationRuns(c *gin.Context) {
	manager, ok := automationManager(c)
	if !ok {
		return
	}
	limit := 50
	if raw := strings.TrimSpace(c.Query("limit")); raw != "" {
		parsed, err := strconv.Atoi(raw)
		if err != nil || parsed < 1 || parsed > 200 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "limit must be between 1 and 200"})
			return
		}
		limit = parsed
	}
	runs, err := manager.ListRuns(c.Query("task_id"), limit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, runs)
}

// getAutomationSettings godoc
//
//	@Summary		Get watchdog and notification settings
//	@Description	Return redacted automation settings; webhook tokens and HMAC secrets are never returned
//	@Tags			Automation
//	@Produce		json
//	@Security		ApiKeyAuth
//	@Success		200	{object}	task.AutomationSettingsView
//	@Router			/api/automation/settings [get]
func getAutomationSettings(c *gin.Context) {
	manager, ok := automationManager(c)
	if !ok {
		return
	}
	c.JSON(http.StatusOK, manager.Settings())
}

// updateAutomationSettings godoc
//
//	@Summary		Update watchdog and notification settings
//	@Description	Persist validated settings; empty credentials keep existing values unless their clear flag is set
//	@Tags			Automation
//	@Accept			json
//	@Produce		json
//	@Security		ApiKeyAuth
//	@Param			settings	body		task.AutomationSettingsUpdate	true	"Automation settings"
//	@Success		200		{object}	task.AutomationSettingsView
//	@Failure		400		{object}	ErrorResponse
//	@Router			/api/automation/settings [put]
func updateAutomationSettings(c *gin.Context) {
	manager, ok := automationManager(c)
	if !ok {
		return
	}
	var update task.AutomationSettingsUpdate
	if err := c.ShouldBindJSON(&update); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	view, err := manager.UpdateSettings(update)
	if err != nil {
		code := "automation_settings_invalid"
		if errors.Is(err, task.ErrWatchdogControlRequired) {
			code = "watchdog_control_required"
		}
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: err.Error(), Code: code})
		return
	}
	c.JSON(http.StatusOK, view)
}

// getAutomationStatus godoc
//
//	@Summary		Get automation runtime status
//	@Tags			Automation
//	@Produce		json
//	@Security		ApiKeyAuth
//	@Success		200	{object}	task.AutomationStatus
//	@Router			/api/automation/status [get]
func getAutomationStatus(c *gin.Context) {
	manager, ok := automationManager(c)
	if !ok {
		return
	}
	c.JSON(http.StatusOK, manager.Status())
}

// testAutomationNotification godoc
//
//	@Summary		Send a test automation notification
//	@Tags			Automation
//	@Produce		json
//	@Security		ApiKeyAuth
//	@Success		200	{object}	SuccessResponse
//	@Failure		400	{object}	ErrorResponse
//	@Router			/api/automation/notifications/test [post]
func testAutomationNotification(c *gin.Context) {
	manager, ok := automationManager(c)
	if !ok {
		return
	}
	if err := manager.TestNotification(c.Request.Context()); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true})
}

// resetAutomationWatchdog godoc
//
//	@Summary		Reset watchdog failures and recovery backoff
//	@Tags			Automation
//	@Produce		json
//	@Security		ApiKeyAuth
//	@Success		200	{object}	SuccessResponse
//	@Router			/api/automation/watchdog/reset [post]
func resetAutomationWatchdog(c *gin.Context) {
	manager, ok := automationManager(c)
	if !ok {
		return
	}
	manager.ResetWatchdog()
	c.JSON(http.StatusOK, gin.H{"success": true})
}
