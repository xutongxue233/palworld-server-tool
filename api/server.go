package api

import (
	"errors"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/zaigie/palworld-server-tool/internal/logger"
	"github.com/zaigie/palworld-server-tool/internal/tool"
)

type ServerInfo struct {
	Version     string `json:"version"`
	Name        string `json:"name"`
	Description string `json:"description"`
	WorldGUID   string `json:"world_guid"`
}

type ServerMetrics struct {
	ServerFps        int     `json:"server_fps"`
	CurrentPlayerNum int     `json:"current_player_num"`
	ServerFrameTime  float64 `json:"server_frame_time"`
	MaxPlayerNum     int     `json:"max_player_num"`
	Uptime           int     `json:"uptime"`
	BaseCampNum      int     `json:"base_camp_num"`
	Days             int     `json:"days"`
}

type BroadcastRequest struct {
	Message string `json:"message"`
}

type ShutdownRequest struct {
	Seconds int    `json:"seconds"`
	Message string `json:"message"`
}

type ServerToolResponse struct {
	Version string `json:"version"`
	Latest  string `json:"latest"`
}

// getServerTool godoc
//
//	@Summary		Get PalWorld Server Tool
//	@Description	Get PalWorld Server Tool
//	@Tags			Server
//	@Accept			json
//	@Produce		json
//	@Success		200	{object}	ServerToolResponse
//	@Router			/api/server/tool [get]
func getServerTool(c *gin.Context) {
	version, exists := c.Get("version")
	if !exists {
		version = "Unknown"
	}
	latest, err := tool.GetLatestTag()
	if err != nil {
		logger.Errorf("%v\n", err)
	}
	if latest == "" {
		latest, err = tool.GetLatestTagFromGitee()
		if err != nil {
			logger.Errorf("%v\n", err)
		}
	}
	c.JSON(http.StatusOK, gin.H{"version": version, "latest": latest})
}

// getServer godoc
//
//	@Summary		Get Server Info
//	@Description	Get Server Info
//	@Tags			Server
//	@Accept			json
//	@Produce		json
//	@Success		200	{object}	ServerInfo
//	@Failure		400	{object}	ErrorResponse
//	@Router			/api/server [get]
func getServer(c *gin.Context) {
	info, err := tool.Info()
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, &ServerInfo{
		Version:     info.Version,
		Name:        info.ServerName,
		Description: info.Description,
		WorldGUID:   info.WorldGUID,
	})
}

// getServerMetrics godoc
//
//	@Summary		Get Server Metrics
//	@Description	Get Server Metrics
//	@Tags			Server
//	@Accept			json
//	@Produce		json
//	@Success		200	{object}	ServerMetrics
//	@Failure		400	{object}	ErrorResponse
//	@Router			/api/server/metrics [get]
func getServerMetrics(c *gin.Context) {
	metrics, err := tool.Metrics()
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, &ServerMetrics{
		ServerFps:        metrics.ServerFps,
		CurrentPlayerNum: metrics.CurrentPlayerNum,
		ServerFrameTime:  metrics.ServerFrameTime,
		MaxPlayerNum:     metrics.MaxPlayerNum,
		Uptime:           metrics.Uptime,
		BaseCampNum:      metrics.BaseCampNum,
		Days:             metrics.Days,
	})
}

// getServerSettings godoc
//
//	@Summary		Get Server Settings
//	@Description	Get the active Palworld dedicated server settings
//	@Tags			Server
//	@Produce		json
//	@Security		ApiKeyAuth
//	@Success		200	{object}	map[string]interface{}
//	@Failure		400	{object}	ErrorResponse
//	@Failure		401	{object}	ErrorResponse
//	@Router			/api/server/settings [get]
func getServerSettings(c *gin.Context) {
	settings, err := tool.Settings()
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, settings)
}

// getWorldActorSnapshot godoc
//
//	@Summary		Get World Actor Snapshot
//	@Description	Get the current world actor snapshot, or an Available=false response when the optional PalGameDataBridge API is disabled
//	@Tags			Server
//	@Produce		json
//	@Security		ApiKeyAuth
//	@Success		200	{object}	map[string]interface{}
//	@Failure		400	{object}	ErrorResponse
//	@Failure		401	{object}	ErrorResponse
//	@Router			/api/server/game-data [get]
func getWorldActorSnapshot(c *gin.Context) {
	snapshot, err := tool.WorldActorSnapshot()
	if err != nil {
		var restErr *tool.RESTError
		if errors.As(err, &restErr) && restErr.StatusCode == http.StatusNotFound {
			c.JSON(http.StatusOK, gin.H{
				"Available": false,
				"Message":   "PalGameDataBridge GameData API is not enabled",
			})
			return
		}
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.Data(http.StatusOK, "application/json; charset=utf-8", snapshot)
}

// publishBroadcast godoc
//
//	@Summary		Publish Broadcast
//	@Description	Publish Broadcast
//	@Tags			Server
//	@Accept			json
//	@Produce		json
//	@Security		ApiKeyAuth
//	@Param			broadcast	body		BroadcastRequest	true	"Broadcast"
//
//	@Success		200			{object}	SuccessResponse
//	@Failure		400			{object}	ErrorResponse
//	@Failure		401			{object}	ErrorResponse
//	@Router			/api/server/broadcast [post]
func publishBroadcast(c *gin.Context) {
	var req BroadcastRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if err := validateMessage(req.Message); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if err := tool.Broadcast(req.Message); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true})
}

// shutdownServer godoc
//
//	@Summary		Shutdown Server
//	@Description	Shutdown Server
//	@Tags			Server
//	@Accept			json
//	@Produce		json
//	@Security		ApiKeyAuth
//	@Param			shutdown	body		ShutdownRequest	true	"Shutdown"
//
//	@Success		200			{object}	SuccessResponse
//	@Failure		400			{object}	ErrorResponse
//	@Failure		401			{object}	ErrorResponse
//	@Router			/api/server/shutdown [post]
func shutdownServer(c *gin.Context) {
	var req ShutdownRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if req.Seconds < 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "seconds cannot be negative"})
		return
	}
	if req.Seconds == 0 {
		req.Seconds = 60
	}
	if err := tool.Shutdown(req.Seconds, req.Message); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true})
}

// saveWorld godoc
//
//	@Summary		Save World
//	@Description	Ask the Palworld server to save the world immediately
//	@Tags			Server
//	@Produce		json
//	@Security		ApiKeyAuth
//	@Success		200	{object}	SuccessResponse
//	@Failure		400	{object}	ErrorResponse
//	@Failure		401	{object}	ErrorResponse
//	@Router			/api/server/save [post]
func saveWorld(c *gin.Context) {
	if err := tool.SaveWorld(); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true})
}

// stopServer godoc
//
//	@Summary		Force Stop Server
//	@Description	Force stop the Palworld server without a graceful shutdown delay
//	@Tags			Server
//	@Produce		json
//	@Security		ApiKeyAuth
//	@Success		200	{object}	SuccessResponse
//	@Failure		400	{object}	ErrorResponse
//	@Failure		401	{object}	ErrorResponse
//	@Router			/api/server/stop [post]
func stopServer(c *gin.Context) {
	if err := tool.ForceStopManagedServer(c.Request.Context()); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true})
}

// getServerControlStatus godoc
//
//	@Summary		Get managed server control status
//	@Description	Get the configured process, Docker, systemd, or Windows service control status
//	@Tags			Server
//	@Produce		json
//	@Security		ApiKeyAuth
//	@Success		200	{object}	tool.ServerControlStatus
//	@Failure		401	{object}	ErrorResponse
//	@Router			/api/server/control/status [get]
func getServerControlStatus(c *gin.Context) {
	c.JSON(http.StatusOK, tool.GetServerControlStatus(c.Request.Context()))
}

// startServer godoc
//
//	@Summary		Start managed Palworld server
//	@Description	Start the configured process, Docker container, systemd unit, or Windows service and wait for REST API readiness
//	@Tags			Server
//	@Produce		json
//	@Security		ApiKeyAuth
//	@Success		200	{object}	SuccessResponse
//	@Failure		400	{object}	ErrorResponse
//	@Failure		401	{object}	ErrorResponse
//	@Router			/api/server/start [post]
func startServer(c *gin.Context) {
	if err := tool.StartManagedServer(c.Request.Context()); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true})
}

// restartServer godoc
//
//	@Summary		Restart managed Palworld server
//	@Description	Save the world, gracefully shut down the server, start it through the configured control driver, and wait for REST API readiness
//	@Tags			Server
//	@Accept			json
//	@Produce		json
//	@Security		ApiKeyAuth
//	@Param			restart	body		ShutdownRequest	true	"Restart"
//	@Success		200		{object}	SuccessResponse
//	@Failure		400		{object}	ErrorResponse
//	@Failure		401		{object}	ErrorResponse
//	@Router			/api/server/restart [post]
func restartServer(c *gin.Context) {
	var req ShutdownRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if req.Seconds < 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "seconds cannot be negative"})
		return
	}
	if err := tool.RestartManagedServer(c.Request.Context(), req.Seconds, req.Message); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true})
}

func validateMessage(message string) error {
	if strings.TrimSpace(message) == "" {
		return errors.New("message cannot be empty")
	}
	return nil
}
