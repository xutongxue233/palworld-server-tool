package api

import (
	"errors"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/zaigie/palworld-server-tool/internal/executor"
	"github.com/zaigie/palworld-server-tool/internal/tool"
)

const maxRconCommandLength = 4096

var executeRCONCommand = tool.CustomCommand

type RconCommandRequest struct {
	Command string `json:"command"`
}

// runRconCommand godoc
//
//	@Summary		Run RCON Command
//	@Description	Run an arbitrary command through Palworld's official RCON interface
//	@Tags			Rcon
//	@Accept			json
//	@Produce		json
//	@Security		ApiKeyAuth
//	@Param			command	body		RconCommandRequest	true	"RCON command"
//	@Success		200		{object}	MessageResponse
//	@Failure		400		{object}	ErrorResponse
//	@Failure		401		{object}	ErrorResponse
//	@Router			/api/rcon [post]
func runRconCommand(c *gin.Context) {
	var request RconCommandRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	command := strings.TrimSpace(request.Command)
	if command == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "command cannot be empty"})
		return
	}
	if len(command) > maxRconCommandLength {
		c.JSON(http.StatusBadRequest, gin.H{"error": "command is too long"})
		return
	}
	response, err := executeRCONCommand(command)
	if err != nil {
		if errors.Is(err, executor.ErrPasswordEmpty) {
			c.JSON(http.StatusBadRequest, gin.H{"error": "RCON password is not configured"})
			return
		}
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": response})
}
