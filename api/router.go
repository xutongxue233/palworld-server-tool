package api

import (
	"fmt"
	"strings"

	"github.com/gin-gonic/gin"
	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"
	"github.com/zaigie/palworld-server-tool/internal/auth"
)

type SuccessResponse struct {
	Success bool `json:"success"`
}

type MessageResponse struct {
	Message string `json:"message"`
}

type ErrorResponse struct {
	Error string `json:"error"`
	Code  string `json:"code,omitempty"`
}

type EmptyResponse struct{}

func ignoreLogPrefix(path string) bool {
	prefixes := []string{"/swagger/", "/assets/", "/favicon.ico", "/map"}
	for _, prefix := range prefixes {
		if strings.HasPrefix(path, prefix) {
			return true
		}
	}
	return false
}

func Logger() gin.HandlerFunc {
	return gin.LoggerWithFormatter(func(param gin.LogFormatterParams) string {
		if !ignoreLogPrefix(param.Path) {
			statusColor := param.StatusCodeColor()
			methodColor := param.MethodColor()
			resetColor := param.ResetColor()
			return fmt.Sprintf("[GIN] %v |%s %3d %s| %13v | %15s |%s %-7s %s %#v\n%s",
				param.TimeStamp.Format("2006/01/02 - 15:04:05"),
				statusColor, param.StatusCode, resetColor,
				param.Latency,
				param.ClientIP,
				methodColor, param.Method, resetColor,
				param.Path,
				param.ErrorMessage,
			)
		}
		return ""
	})
}

func RegisterRouter(r *gin.Engine) {
	r.Use(Logger(), gin.Recovery())

	r.POST("/api/login", loginHandler)
	r.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))

	apiGroup := r.Group("/api")

	anonymousGroup := apiGroup.Group("")
	{
		anonymousGroup.GET("/server", getServer)
		anonymousGroup.GET("/server/tool", getServerTool)
		anonymousGroup.GET("/server/metrics", getServerMetrics)
		anonymousGroup.GET("/guild", listGuilds)
		anonymousGroup.GET("/guild/:admin_player_uid", getGuild)
	}
	// 根据登录状态返回不同结果
	OptionalGroup := apiGroup.Group("")
	OptionalGroup.Use(auth.OptionalJWTMiddleware())
	{
		OptionalGroup.GET("/online_player", listOnlinePlayers)
		OptionalGroup.GET("/player", listPlayers)
		OptionalGroup.GET("/player/:player_uid", getPlayer)
	}

	authGroup := apiGroup.Group("")
	authGroup.Use(auth.JWTAuthMiddleware())
	{
		authGroup.GET("/server/settings", getServerSettings)
		authGroup.GET("/server/game-data", getWorldActorSnapshot)
		authGroup.GET("/server/config-file", getGameConfigFile)
		authGroup.PUT("/server/config-file", putGameConfigFile)
		authGroup.PUT("/server/world-option", syncWorldOption)
		authGroup.GET("/server/control/status", getServerControlStatus)
		authGroup.GET("/server/steamcmd", getSteamCMDStatus)
		authGroup.POST("/server/steamcmd/update", updateServerWithSteamCMD)
		authGroup.GET("/server/backups/native", listNativeBackups)
		authGroup.POST("/server/backups/native/:backup_id/restore", restoreNativeBackup)
		authGroup.POST("/server/start", startServer)
		authGroup.POST("/server/restart", restartServer)
		authGroup.GET("/automation/tasks", listAutomationTasks)
		authGroup.POST("/automation/tasks", createAutomationTask)
		authGroup.PUT("/automation/tasks/:task_id", updateAutomationTask)
		authGroup.DELETE("/automation/tasks/:task_id", deleteAutomationTask)
		authGroup.POST("/automation/tasks/:task_id/run", runAutomationTask)
		authGroup.GET("/automation/runs", listAutomationRuns)
		authGroup.GET("/automation/settings", getAutomationSettings)
		authGroup.PUT("/automation/settings", updateAutomationSettings)
		authGroup.GET("/automation/status", getAutomationStatus)
		authGroup.POST("/automation/notifications/test", testAutomationNotification)
		authGroup.POST("/automation/watchdog/reset", resetAutomationWatchdog)
		authGroup.POST("/rcon", runRconCommand)
		authGroup.POST("/server/broadcast", publishBroadcast)
		authGroup.POST("/server/save", saveWorld)
		authGroup.POST("/server/shutdown", shutdownServer)
		authGroup.POST("/server/stop", stopServer)
		authGroup.PUT("/player", putPlayers)
		authGroup.POST("/player/:player_uid/kick", kickPlayer)
		authGroup.POST("/player/:player_uid/ban", banPlayer)
		authGroup.POST("/player/:player_uid/unban", unbanPlayer)
		authGroup.POST("/player/:player_uid/items", givePlayerItem)
		authGroup.PATCH("/player/:player_uid/items/:container/:slot_index", setPlayerItemQuantity)
		authGroup.PATCH("/player/:player_uid/profile", editPlayerProfile)
		authGroup.PATCH("/player/:player_uid/stat-points", editPlayerStatPoints)
		authGroup.PATCH("/player/:player_uid/technology-points", editPlayerTechnologyPoints)
		authGroup.PATCH("/player/:player_uid/map-progress", unlockPlayerMap)
		authGroup.PATCH("/player/:player_uid/pals/:instance_id/nickname", renamePal)
		authGroup.PATCH("/player/:player_uid/pals/:instance_id/level", editPalLevel)
		authGroup.PATCH("/player/:player_uid/pals/:instance_id/health", restorePalHealth)
		authGroup.PUT("/guild", putGuilds)
		authGroup.POST("/sync", syncData)
		authGroup.GET("/whitelist", listWhite)
		authGroup.POST("/whitelist", addWhite)
		authGroup.DELETE("/whitelist", removeWhite)
		authGroup.PUT("/whitelist", putWhite)
		authGroup.GET("/backup", listBackups)
		authGroup.GET("/backup/:backup_id", downloadBackup)
		authGroup.DELETE("/backup/:backup_id", deleteBackup)
	}
}
