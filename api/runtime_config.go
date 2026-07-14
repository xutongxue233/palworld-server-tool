package api

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/zaigie/palworld-server-tool/internal/config"
)

type RuntimeConfigResponse struct {
	Storage           string         `json:"storage"`
	Values            map[string]any `json:"values"`
	ConfiguredSecrets []string       `json:"configured_secrets"`
	RestartRequired   []string       `json:"restart_required,omitempty"`
}

type RuntimeConfigUpdateRequest struct {
	Values map[string]any `json:"values" binding:"required"`
}

// getRuntimeConfig godoc
//
//	@Summary		Read database-backed PST runtime configuration
//	@Description	Return stored non-sensitive dotted keys and the names of configured secrets without returning secret values
//	@Tags			Setup
//	@Produce		json
//	@Security		ApiKeyAuth
//	@Success		200	{object}	RuntimeConfigResponse
//	@Failure		401	{object}	ErrorResponse
//	@Failure		500	{object}	ErrorResponse
//	@Router			/api/setup/config [get]
func getRuntimeConfig(c *gin.Context) {
	c.Header("Cache-Control", "no-store")
	snapshot, err := config.RuntimeValues()
	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: err.Error()})
		return
	}
	c.JSON(http.StatusOK, RuntimeConfigResponse{
		Storage:           config.StorageName(),
		Values:            snapshot.Values,
		ConfiguredSecrets: snapshot.ConfiguredSecrets,
	})
}

// putRuntimeConfig godoc
//
//	@Summary		Update database-backed PST runtime configuration
//	@Description	Validate allowlisted dotted keys, persist them in pst.db, and report the values that require a PST restart
//	@Tags			Setup
//	@Accept			json
//	@Produce		json
//	@Security		ApiKeyAuth
//	@Param			update	body		RuntimeConfigUpdateRequest	true	"Dotted configuration values"
//	@Success		200		{object}	RuntimeConfigResponse
//	@Failure		400		{object}	ErrorResponse
//	@Failure		401		{object}	ErrorResponse
//	@Failure		500		{object}	ErrorResponse
//	@Router			/api/setup/config [put]
func putRuntimeConfig(c *gin.Context) {
	c.Header("Cache-Control", "no-store")
	var request RuntimeConfigUpdateRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: err.Error()})
		return
	}
	restartRequired, err := config.ApplyRuntimeValues(request.Values)
	if err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: err.Error(), Code: "runtime_config_invalid"})
		return
	}
	snapshot, err := config.RuntimeValues()
	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: err.Error()})
		return
	}
	c.JSON(http.StatusOK, RuntimeConfigResponse{
		Storage:           config.StorageName(),
		Values:            snapshot.Values,
		ConfiguredSecrets: snapshot.ConfiguredSecrets,
		RestartRequired:   restartRequired,
	})
}
