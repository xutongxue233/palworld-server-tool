package api

import (
	"errors"
	"net/http"
	"os"

	"github.com/gin-gonic/gin"
	"github.com/zaigie/palworld-server-tool/internal/tool"
)

type GameConfigWriteRequest struct {
	Content        string `json:"content"`
	ExpectedSHA256 string `json:"expected_sha256"`
}

// getGameConfigFile godoc
//
//	@Summary		Read PalWorldSettings.ini
//	@Description	Read the configured PalWorldSettings.ini with a concurrency digest
//	@Tags			Server
//	@Produce		json
//	@Security		ApiKeyAuth
//	@Success		200	{object}	tool.GameConfigFile
//	@Failure		401	{object}	ErrorResponse
//	@Failure		404	{object}	ErrorResponse
//	@Router			/api/server/config-file [get]
func getGameConfigFile(c *gin.Context) {
	c.Header("Cache-Control", "no-store")
	result, err := tool.ReadGameConfigFile()
	if err != nil {
		status := http.StatusBadRequest
		if errors.Is(err, os.ErrNotExist) {
			status = http.StatusNotFound
		}
		c.JSON(status, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, result)
}

// putGameConfigFile godoc
//
//	@Summary		Write PalWorldSettings.ini
//	@Description	Validate and atomically replace PalWorldSettings.ini after creating a safety backup
//	@Tags			Server
//	@Accept			json
//	@Produce		json
//	@Security		ApiKeyAuth
//	@Param			config	body		GameConfigWriteRequest	true	"Configuration"
//	@Success		200		{object}	tool.GameConfigWriteResult
//	@Failure		400		{object}	ErrorResponse
//	@Failure		401		{object}	ErrorResponse
//	@Failure		409		{object}	ErrorResponse
//	@Router			/api/server/config-file [put]
func putGameConfigFile(c *gin.Context) {
	c.Header("Cache-Control", "no-store")
	var req GameConfigWriteRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	release, ok := beginManualOperation(c, nil)
	if !ok {
		return
	}
	defer release()
	result, err := tool.WriteGameConfigFile(req.Content, req.ExpectedSHA256)
	if err != nil {
		status := http.StatusBadRequest
		if errors.Is(err, tool.ErrGameConfigConflict) {
			status = http.StatusConflict
		}
		c.JSON(status, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, result)
}
