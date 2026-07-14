package api

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/zaigie/palworld-server-tool/internal/discovery"
)

type DiscoverySetupStatus struct {
	Platform        string `json:"platform"`
	Configured      bool   `json:"configured"`
	NeedsSetup      bool   `json:"needs_setup"`
	ManualRequired  bool   `json:"manual_required"`
	RestartRequired bool   `json:"restart_required"`
}

// getDiscoverySetupStatus godoc
//
//	@Summary		Inspect whether local PalServer setup is required
//	@Description	Return a non-sensitive setup summary used before administrator sign-in
//	@Tags			Setup
//	@Produce		json
//	@Success		200	{object}	DiscoverySetupStatus
//	@Router			/api/setup/status [get]
func getDiscoverySetupStatus(c *gin.Context) {
	status := discovery.Current()
	c.JSON(http.StatusOK, DiscoverySetupStatus{
		Platform:        status.Platform,
		Configured:      status.Configured,
		NeedsSetup:      status.NeedsSetup,
		ManualRequired:  status.ManualRequired,
		RestartRequired: status.RestartRequired,
	})
}

// getServerDiscovery godoc
//
//	@Summary		List discovered local PalServer installations
//	@Description	Scan results include running processes, Steam libraries, common paths, existing database paths, server INI settings, and worlds
//	@Tags			Setup
//	@Produce		json
//	@Security		ApiKeyAuth
//	@Success		200	{object}	discovery.Status
//	@Failure		401	{object}	ErrorResponse
//	@Router			/api/setup/discovery [get]
func getServerDiscovery(c *gin.Context) {
	c.Header("Cache-Control", "no-store")
	c.JSON(http.StatusOK, discovery.Current())
}

// scanServerDiscovery godoc
//
//	@Summary		Rescan local PalServer installations
//	@Tags			Setup
//	@Produce		json
//	@Security		ApiKeyAuth
//	@Success		200	{object}	discovery.Status
//	@Failure		401	{object}	ErrorResponse
//	@Router			/api/setup/discovery/scan [post]
func scanServerDiscovery(c *gin.Context) {
	c.Header("Cache-Control", "no-store")
	c.JSON(http.StatusOK, discovery.Rescan())
}

// applyServerDiscovery godoc
//
//	@Summary		Apply a discovered or manually selected PalServer installation
//	@Description	Persist derived launcher, INI, save, SteamCMD, REST, RCON, and process-control values in pst.db
//	@Tags			Setup
//	@Accept			json
//	@Produce		json
//	@Security		ApiKeyAuth
//	@Param			selection	body		discovery.ApplyRequest	true	"Candidate ID or local path"
//	@Success		200			{object}	discovery.Status
//	@Failure		400			{object}	ErrorResponse
//	@Failure		401			{object}	ErrorResponse
//	@Router			/api/setup/discovery/apply [post]
func applyServerDiscovery(c *gin.Context) {
	c.Header("Cache-Control", "no-store")
	var request discovery.ApplyRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: err.Error()})
		return
	}
	status, err := discovery.Apply(request)
	if err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: err.Error(), Code: "server_discovery_apply_failed"})
		return
	}
	c.JSON(http.StatusOK, status)
}
