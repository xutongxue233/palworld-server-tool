package api

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/spf13/viper"
	"github.com/zaigie/palworld-server-tool/internal/fleet"
	"github.com/zaigie/palworld-server-tool/internal/tool"
)

const (
	maxFleetStatusBody       = 256 << 10
	maxFleetProxyRequestBody = 8 << 20
	fleetLongOperationWait   = 2*time.Hour + 15*time.Minute
)

var (
	fleetInfoAPI          = tool.Info
	fleetMetricsAPI       = tool.Metrics
	fleetControlStatusAPI = tool.GetServerControlStatus
	fleetNowAPI           = time.Now
)

type FleetNodeStatus struct {
	ProtocolVersion int                       `json:"protocol_version"`
	NodeID          string                    `json:"node_id"`
	NodeName        string                    `json:"node_name"`
	ToolVersion     string                    `json:"tool_version"`
	ServerOnline    bool                      `json:"server_online"`
	Server          *ServerInfo               `json:"server,omitempty"`
	Metrics         *ServerMetrics            `json:"metrics,omitempty"`
	Control         *tool.ServerControlStatus `json:"control,omitempty"`
	ServerError     string                    `json:"server_error,omitempty"`
	CheckedAt       time.Time                 `json:"checked_at"`
}

type FleetNodeView struct {
	Scope             string                    `json:"scope"`
	ID                string                    `json:"id"`
	Name              string                    `json:"name"`
	ManagementURL     string                    `json:"management_url,omitempty"`
	Local             bool                      `json:"local"`
	Reachable         bool                      `json:"reachable"`
	Selectable        bool                      `json:"selectable"`
	InsecureTransport bool                      `json:"insecure_transport"`
	LatencyMS         int64                     `json:"latency_ms"`
	ProtocolVersion   int                       `json:"protocol_version,omitempty"`
	ToolVersion       string                    `json:"tool_version,omitempty"`
	ServerOnline      bool                      `json:"server_online"`
	Server            *ServerInfo               `json:"server,omitempty"`
	Metrics           *ServerMetrics            `json:"metrics,omitempty"`
	Control           *tool.ServerControlStatus `json:"control,omitempty"`
	ServerError       string                    `json:"server_error,omitempty"`
	ErrorCode         string                    `json:"error_code,omitempty"`
	Error             string                    `json:"error,omitempty"`
	CheckedAt         time.Time                 `json:"checked_at"`
}

type FleetStatusResponse struct {
	ProtocolVersion int                 `json:"protocol_version"`
	LocalScope      string              `json:"local_scope"`
	Nodes           []FleetNodeView     `json:"nodes"`
	Issues          []fleet.ConfigIssue `json:"issues,omitempty"`
	CheckedAt       time.Time           `json:"checked_at"`
}

// getFleetNodeStatus godoc
//
//	@Summary		Get this PST fleet-node status
//	@Description	Return the isolated PST node identity, Palworld summary, metrics, and control status for a trusted controller
//	@Tags			Fleet
//	@Produce		json
//	@Security		FleetNodeAuth
//	@Success		200	{object}	FleetNodeStatus
//	@Failure		401	{object}	ErrorResponse
//	@Failure		503	{object}	ErrorResponse
//	@Router			/api/fleet/node/status [get]
func getFleetNodeStatus(c *gin.Context) {
	c.Header("Cache-Control", "no-store")
	configuration := fleet.LoadConfiguration()
	c.JSON(http.StatusOK, buildFleetNodeStatus(c, configuration.Local))
}

// listFleetNodes godoc
//
//	@Summary		List isolated Palworld server nodes
//	@Description	Aggregate the local PST node and configured remote PST nodes without sharing their config, database, save directory, scheduler, or maintenance lock
//	@Tags			Fleet
//	@Produce		json
//	@Security		ApiKeyAuth
//	@Success		200	{object}	FleetStatusResponse
//	@Failure		401	{object}	ErrorResponse
//	@Router			/api/fleet/nodes [get]
func listFleetNodes(c *gin.Context) {
	c.Header("Cache-Control", "no-store")
	configuration := fleet.LoadConfiguration()
	localStatus := buildFleetNodeStatus(c, configuration.Local)
	views := make([]FleetNodeView, len(configuration.Nodes)+1)
	views[0] = fleetNodeViewFromStatus(fleet.LocalScope, localStatus.NodeName, "", true, 0, localStatus)

	var waitGroup sync.WaitGroup
	for index, node := range configuration.Nodes {
		waitGroup.Add(1)
		go func(target int, configured fleet.NodeConfig) {
			defer waitGroup.Done()
			views[target] = inspectRemoteFleetNode(c.Request.Context(), configured)
		}(index+1, node)
	}
	waitGroup.Wait()

	c.JSON(http.StatusOK, FleetStatusResponse{
		ProtocolVersion: fleet.ProtocolVersion,
		LocalScope:      fleet.LocalScope,
		Nodes:           views,
		Issues:          configuration.Issues,
		CheckedAt:       fleetNowAPI().UTC(),
	})
}

// proxyFleetNode godoc
//
//	@Summary		Proxy an allowlisted PST API route to a fleet node
//	@Description	Forward only known PST API methods and paths to a configured node using its fleet token; login, fleet recursion, arbitrary URLs, redirects, and unknown routes are rejected
//	@Tags			Fleet
//	@Accept			json
//	@Produce		json
//	@Security		ApiKeyAuth
//	@Param			node_id	path	string	true	"Configured fleet node ID"
//	@Param			path	path	string	true	"Allowlisted PST API path without the /api prefix"
//	@Success		200
//	@Failure		400	{object}	ErrorResponse
//	@Failure		401	{object}	ErrorResponse
//	@Failure		403	{object}	ErrorResponse
//	@Failure		404	{object}	ErrorResponse
//	@Failure		422	{object}	ErrorResponse
//	@Failure		502	{object}	ErrorResponse
//	@Router			/api/fleet/nodes/{node_id}/proxy/{path} [get]
//	@Router			/api/fleet/nodes/{node_id}/proxy/{path} [post]
//	@Router			/api/fleet/nodes/{node_id}/proxy/{path} [put]
//	@Router			/api/fleet/nodes/{node_id}/proxy/{path} [patch]
//	@Router			/api/fleet/nodes/{node_id}/proxy/{path} [delete]
func proxyFleetNode(c *gin.Context) {
	c.Header("Cache-Control", "no-store")
	nodeID := strings.TrimSpace(c.Param("node_id"))
	if !fleet.ValidNodeID(nodeID) {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "invalid fleet node ID", Code: "fleet_node_id_invalid"})
		return
	}
	node, err := fleet.FindNode(nodeID)
	if err != nil {
		status := http.StatusNotFound
		code := "fleet_node_not_found"
		if errors.Is(err, fleet.ErrNodeConfigInvalid) {
			status = http.StatusUnprocessableEntity
			code = "fleet_node_config_invalid"
		}
		c.JSON(status, ErrorResponse{Error: err.Error(), Code: code})
		return
	}
	targetPath := "/api" + c.Param("path")
	allowed, longOperation := fleet.ProxyRouteAllowed(c.Request.Method, targetPath)
	if !allowed {
		c.JSON(http.StatusForbidden, ErrorResponse{
			Error: "the requested method and path are not in the fleet proxy allowlist",
			Code:  "fleet_proxy_route_forbidden",
		})
		return
	}
	if c.Request.ContentLength > maxFleetProxyRequestBody {
		c.JSON(http.StatusRequestEntityTooLarge, ErrorResponse{
			Error: fmt.Sprintf("fleet proxy request body cannot exceed %d bytes", maxFleetProxyRequestBody),
			Code:  "fleet_proxy_body_too_large",
		})
		return
	}
	targetURL, err := fleet.BuildNodeURL(node, targetPath, c.Request.URL.RawQuery)
	if err != nil {
		c.JSON(http.StatusUnprocessableEntity, ErrorResponse{Error: err.Error(), Code: "fleet_node_url_invalid"})
		return
	}
	body, err := io.ReadAll(io.LimitReader(c.Request.Body, maxFleetProxyRequestBody+1))
	if err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: err.Error(), Code: "fleet_proxy_body_invalid"})
		return
	}
	if len(body) > maxFleetProxyRequestBody {
		c.JSON(http.StatusRequestEntityTooLarge, ErrorResponse{
			Error: fmt.Sprintf("fleet proxy request body cannot exceed %d bytes", maxFleetProxyRequestBody),
			Code:  "fleet_proxy_body_too_large",
		})
		return
	}
	request, err := http.NewRequestWithContext(c.Request.Context(), c.Request.Method, targetURL, bytes.NewReader(body))
	if err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: err.Error(), Code: "fleet_proxy_request_invalid"})
		return
	}
	copyFleetRequestHeaders(request.Header, c.Request.Header)
	request.Header.Set(fleet.NodeTokenHeader, node.Token)
	request.Header.Set("User-Agent", "Palworld-Server-Tool/fleet-v1")

	responseWait := node.Timeout
	if longOperation {
		responseWait = fleetLongOperationWait
	}
	client := fleet.NewHTTPClient(node, responseWait)
	defer client.CloseIdleConnections()
	response, err := client.Do(request)
	if err != nil {
		c.JSON(http.StatusBadGateway, ErrorResponse{
			Error: "fleet node request failed: " + err.Error(),
			Code:  "fleet_node_unreachable",
		})
		return
	}
	defer response.Body.Close()
	if response.StatusCode == http.StatusUnauthorized || response.StatusCode == http.StatusForbidden {
		_, _ = io.Copy(io.Discard, io.LimitReader(response.Body, 4096))
		c.JSON(http.StatusBadGateway, ErrorResponse{
			Error: "remote PST node rejected the configured fleet credential",
			Code:  "fleet_node_auth_failed",
		})
		return
	}
	copyFleetResponseHeaders(c.Writer.Header(), response.Header)
	c.Header("X-PST-Fleet-Node", node.ID)
	c.Status(response.StatusCode)
	_, _ = io.Copy(c.Writer, response.Body)
}

func buildFleetNodeStatus(c *gin.Context, identity fleet.LocalIdentity) FleetNodeStatus {
	version := "Unknown"
	if value, exists := c.Get("version"); exists {
		version = fmt.Sprint(value)
	}
	status := FleetNodeStatus{
		ProtocolVersion: fleet.ProtocolVersion,
		NodeID:          identity.ID,
		NodeName:        identity.Name,
		ToolVersion:     version,
		CheckedAt:       fleetNowAPI().UTC(),
	}
	control := fleetControlStatusAPI(c.Request.Context())
	status.Control = &control
	info, infoErr := fleetInfoAPI()
	if infoErr == nil {
		status.ServerOnline = true
		status.Server = &ServerInfo{
			Version:     info.Version,
			Name:        info.ServerName,
			Description: info.Description,
			WorldGUID:   info.WorldGUID,
		}
		if strings.TrimSpace(viper.GetString("fleet.node_name")) == "" && strings.TrimSpace(info.ServerName) != "" {
			status.NodeName = strings.TrimSpace(info.ServerName)
		}
	} else {
		status.ServerError = infoErr.Error()
	}
	if infoErr == nil {
		metrics, metricsErr := fleetMetricsAPI()
		if metricsErr == nil {
			status.Metrics = &ServerMetrics{
				ServerFps:        metrics.ServerFps,
				CurrentPlayerNum: metrics.CurrentPlayerNum,
				ServerFrameTime:  metrics.ServerFrameTime,
				MaxPlayerNum:     metrics.MaxPlayerNum,
				Uptime:           metrics.Uptime,
				BaseCampNum:      metrics.BaseCampNum,
				Days:             metrics.Days,
			}
		} else if status.ServerError == "" {
			status.ServerError = metricsErr.Error()
		}
	}
	return status
}

func inspectRemoteFleetNode(ctx context.Context, node fleet.NodeConfig) FleetNodeView {
	view := FleetNodeView{
		Scope:             node.ID,
		ID:                node.ID,
		Name:              node.Name,
		ManagementURL:     fleet.ManagementURL(node),
		InsecureTransport: strings.HasPrefix(strings.ToLower(node.BaseURL), "http://"),
		CheckedAt:         fleetNowAPI().UTC(),
	}
	targetURL, err := fleet.BuildNodeURL(node, "/api/fleet/node/status", "")
	if err != nil {
		view.ErrorCode = "fleet_node_url_invalid"
		view.Error = err.Error()
		return view
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, targetURL, nil)
	if err != nil {
		view.ErrorCode = "fleet_node_request_invalid"
		view.Error = err.Error()
		return view
	}
	request.Header.Set("Accept", "application/json")
	request.Header.Set(fleet.NodeTokenHeader, node.Token)
	request.Header.Set("User-Agent", "Palworld-Server-Tool/fleet-v1")
	client := fleet.NewHTTPClient(node, node.Timeout)
	defer client.CloseIdleConnections()
	started := fleetNowAPI()
	response, err := client.Do(request)
	view.LatencyMS = fleetNowAPI().Sub(started).Milliseconds()
	if err != nil {
		view.ErrorCode = "fleet_node_unreachable"
		view.Error = err.Error()
		return view
	}
	defer response.Body.Close()
	body, readErr := io.ReadAll(io.LimitReader(response.Body, maxFleetStatusBody+1))
	if readErr != nil {
		view.ErrorCode = "fleet_node_response_invalid"
		view.Error = readErr.Error()
		return view
	}
	if len(body) > maxFleetStatusBody {
		view.ErrorCode = "fleet_node_response_too_large"
		view.Error = fmt.Sprintf("fleet node status exceeded %d bytes", maxFleetStatusBody)
		return view
	}
	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		view.ErrorCode = "fleet_node_status_failed"
		if response.StatusCode == http.StatusUnauthorized || response.StatusCode == http.StatusForbidden {
			view.ErrorCode = "fleet_node_auth_failed"
		}
		view.Error = fmt.Sprintf("fleet node returned %d", response.StatusCode)
		return view
	}
	var status FleetNodeStatus
	if err := json.Unmarshal(body, &status); err != nil {
		view.ErrorCode = "fleet_node_response_invalid"
		view.Error = err.Error()
		return view
	}
	view.Reachable = true
	view.ProtocolVersion = status.ProtocolVersion
	view.ToolVersion = status.ToolVersion
	view.ServerOnline = status.ServerOnline
	view.Server = status.Server
	view.Metrics = status.Metrics
	view.Control = status.Control
	view.ServerError = status.ServerError
	view.CheckedAt = status.CheckedAt
	if node.Name == node.ID && strings.TrimSpace(status.NodeName) != "" {
		view.Name = status.NodeName
	}
	if status.ProtocolVersion != fleet.ProtocolVersion {
		view.ErrorCode = "fleet_protocol_mismatch"
		view.Error = fmt.Sprintf("remote protocol %d does not match controller protocol %d", status.ProtocolVersion, fleet.ProtocolVersion)
		return view
	}
	if !strings.EqualFold(status.NodeID, node.ID) {
		view.ErrorCode = "fleet_node_identity_mismatch"
		view.Error = fmt.Sprintf("remote node identifies as %q instead of %q", status.NodeID, node.ID)
		return view
	}
	view.Selectable = true
	return view
}

func fleetNodeViewFromStatus(
	scope,
	name,
	managementURL string,
	local bool,
	latency int64,
	status FleetNodeStatus,
) FleetNodeView {
	if strings.TrimSpace(name) == "" {
		name = status.NodeName
	}
	return FleetNodeView{
		Scope:           scope,
		ID:              status.NodeID,
		Name:            name,
		ManagementURL:   managementURL,
		Local:           local,
		Reachable:       true,
		Selectable:      true,
		LatencyMS:       latency,
		ProtocolVersion: status.ProtocolVersion,
		ToolVersion:     status.ToolVersion,
		ServerOnline:    status.ServerOnline,
		Server:          status.Server,
		Metrics:         status.Metrics,
		Control:         status.Control,
		ServerError:     status.ServerError,
		CheckedAt:       status.CheckedAt,
	}
}

func copyFleetRequestHeaders(target, source http.Header) {
	for _, name := range []string{"Accept", "Content-Type", "If-Match", "If-None-Match", "Range"} {
		for _, value := range source.Values(name) {
			target.Add(name, value)
		}
	}
}

func copyFleetResponseHeaders(target, source http.Header) {
	for _, name := range []string{
		"Accept-Ranges",
		"Cache-Control",
		"Content-Disposition",
		"Content-Length",
		"Content-Range",
		"Content-Type",
		"ETag",
		"Last-Modified",
	} {
		for _, value := range source.Values(name) {
			target.Add(name, value)
		}
	}
}
