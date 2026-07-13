package api

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/zaigie/palworld-server-tool/internal/database"
	"github.com/zaigie/palworld-server-tool/internal/task"
	"github.com/zaigie/palworld-server-tool/internal/tool"
	"github.com/zaigie/palworld-server-tool/service"
)

type PlayerOrderBy string

const (
	OrderByLastOnline PlayerOrderBy = "last_online"
	OrderByLevel      PlayerOrderBy = "level"
)

type PlayerActionRequest struct {
	Message string `json:"message"`
}

type GiveItemRequest struct {
	ItemID               string `json:"item_id" binding:"required,max=128"`
	Quantity             int    `json:"quantity" binding:"required,min=1,max=999999"`
	Container            string `json:"container" binding:"omitempty,oneof=auto main key"`
	ConfirmServerStopped bool   `json:"confirm_server_stopped"`
}

type GiveItemResponse struct {
	Delivery  tool.ItemDelivery `json:"delivery"`
	Backup    database.Backup   `json:"backup"`
	SyncError string            `json:"sync_error,omitempty"`
}

type SetItemQuantityRequest struct {
	ItemID               string `json:"item_id" binding:"required,max=128"`
	ExpectedQuantity     int    `json:"expected_quantity" binding:"required,min=1,max=999999"`
	ExpectedDynamicID    string `json:"expected_dynamic_id" binding:"required,max=36"`
	Quantity             *int   `json:"quantity" binding:"required,min=0,max=999999"`
	ConfirmServerStopped bool   `json:"confirm_server_stopped"`
}

type SetItemQuantityResponse struct {
	Mutation  tool.InventoryMutation `json:"mutation"`
	Backup    database.Backup        `json:"backup"`
	SyncError string                 `json:"sync_error,omitempty"`
}

type EditPlayerProfileRequest struct {
	ExpectedNickname     string `json:"expected_nickname" binding:"required,max=32"`
	ExpectedLevel        int    `json:"expected_level" binding:"required,min=1,max=80"`
	Nickname             string `json:"nickname" binding:"required,max=32"`
	Level                int    `json:"level" binding:"required,min=1,max=80"`
	ConfirmServerStopped bool   `json:"confirm_server_stopped"`
}

type EditPlayerProfileResponse struct {
	Profile   tool.PlayerProfileMutation `json:"profile"`
	Backup    database.Backup            `json:"backup"`
	SyncError string                     `json:"sync_error,omitempty"`
}

type EditPlayerStatPointsRequest struct {
	ExpectedUnusedStatPoints *int `json:"expected_unused_status_points" binding:"required,min=0,max=65535"`
	UnusedStatPoints         *int `json:"unused_status_points" binding:"required,min=0,max=65535"`
	ConfirmServerStopped     bool `json:"confirm_server_stopped"`
}

type EditPlayerStatPointsResponse struct {
	StatPoints tool.PlayerStatPointsMutation `json:"stat_points"`
	Backup     database.Backup               `json:"backup"`
	SyncError  string                        `json:"sync_error,omitempty"`
}

type EditPlayerTechnologyPointsRequest struct {
	ExpectedTechnologyPoints        *int `json:"expected_technology_points" binding:"required,min=0,max=999999"`
	ExpectedAncientTechnologyPoints *int `json:"expected_ancient_technology_points" binding:"required,min=0,max=999999"`
	TechnologyPoints                *int `json:"technology_points" binding:"required,min=0,max=999999"`
	AncientTechnologyPoints         *int `json:"ancient_technology_points" binding:"required,min=0,max=999999"`
	ConfirmServerStopped            bool `json:"confirm_server_stopped"`
}

type EditPlayerTechnologyPointsResponse struct {
	TechnologyPoints tool.PlayerTechnologyPointsMutation `json:"technology_points"`
	Backup           database.Backup                     `json:"backup"`
	SyncError        string                              `json:"sync_error,omitempty"`
}

type UnlockPlayerMapRequest struct {
	ExpectedProgressDigest *string `json:"expected_progress_digest" binding:"required,len=64,hexadecimal"`
	ConfirmServerStopped   bool    `json:"confirm_server_stopped"`
}

type UnlockPlayerMapResponse struct {
	MapProgress tool.PlayerMapProgressMutation `json:"map_progress"`
	Backup      database.Backup                `json:"backup"`
	SyncError   string                         `json:"sync_error,omitempty"`
}

type RenamePalRequest struct {
	ExpectedNickname     *string `json:"expected_nickname" binding:"required,max=32"`
	ExpectedLevel        *int    `json:"expected_level" binding:"required,min=1,max=80"`
	ExpectedExp          *int64  `json:"expected_exp" binding:"required,min=0"`
	Nickname             *string `json:"nickname" binding:"required,max=32"`
	ConfirmServerStopped bool    `json:"confirm_server_stopped"`
}

type RenamePalResponse struct {
	Nickname  tool.PalNicknameMutation `json:"nickname"`
	Backup    database.Backup          `json:"backup"`
	SyncError string                   `json:"sync_error,omitempty"`
}

type EditPalLevelRequest struct {
	ExpectedNickname     *string `json:"expected_nickname" binding:"required,max=32"`
	ExpectedLevel        *int    `json:"expected_level" binding:"required,min=1,max=80"`
	ExpectedExp          *int64  `json:"expected_exp" binding:"required,min=0"`
	ExpectedHP           *int64  `json:"expected_hp" binding:"required,min=0"`
	ExpectedMaxHP        *int64  `json:"expected_max_hp" binding:"required,min=0"`
	Level                *int    `json:"level" binding:"required,min=1,max=80"`
	ConfirmServerStopped bool    `json:"confirm_server_stopped"`
}

type EditPalLevelResponse struct {
	Level     tool.PalLevelMutation `json:"level"`
	Backup    database.Backup       `json:"backup"`
	SyncError string                `json:"sync_error,omitempty"`
}

type RestorePalHealthRequest struct {
	ExpectedNickname     *string `json:"expected_nickname" binding:"required,max=32"`
	ExpectedLevel        *int    `json:"expected_level" binding:"required,min=1,max=80"`
	ExpectedExp          *int64  `json:"expected_exp" binding:"required,min=0"`
	ExpectedHP           *int64  `json:"expected_hp" binding:"required,min=0"`
	ExpectedMaxHP        *int64  `json:"expected_max_hp" binding:"required,min=1"`
	ConfirmServerStopped bool    `json:"confirm_server_stopped"`
}

type RestorePalHealthResponse struct {
	Health    tool.PalHealthMutation `json:"health"`
	Backup    database.Backup        `json:"backup"`
	SyncError string                 `json:"sync_error,omitempty"`
}

func writeSaveEditError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, tool.ErrSaveEditConfirmation):
		c.JSON(http.StatusConflict, ErrorResponse{
			Error: err.Error(), Code: "save_edit_confirmation",
		})
	case errors.Is(err, tool.ErrGameServerRunning):
		c.JSON(http.StatusConflict, ErrorResponse{
			Error: err.Error(), Code: "game_server_running",
		})
	case errors.Is(err, tool.ErrSaveSourceChanged):
		c.JSON(http.StatusConflict, ErrorResponse{
			Error: err.Error(), Code: "save_source_changed",
		})
	case errors.Is(err, tool.ErrGameServerStatusUnknown):
		c.JSON(http.StatusServiceUnavailable, ErrorResponse{
			Error: err.Error(), Code: "game_server_status_unknown",
		})
	case errors.Is(err, tool.ErrUnsupportedSaveSource):
		c.JSON(http.StatusUnprocessableEntity, ErrorResponse{
			Error: err.Error(), Code: "unsupported_save_source",
		})
	case errors.Is(err, tool.ErrSaveEditInternal):
		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Error: err.Error(), Code: "save_edit_internal",
		})
	default:
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: err.Error()})
	}
}

func bindOptionalPlayerAction(c *gin.Context) (PlayerActionRequest, error) {
	var req PlayerActionRequest
	if err := c.ShouldBindJSON(&req); err != nil && !errors.Is(err, io.EOF) {
		return PlayerActionRequest{}, err
	}
	return req, nil
}

// getPlayerActionUserId 获取用于 kick/ban/unban 操作的 userId
// 优先使用完整的 UserId（支持跨平台），兜底使用 steam_ + SteamId
func getPlayerActionUserId(player database.Player) string {
	if player.UserId != "" {
		return player.UserId
	}
	if player.SteamId != "" {
		return fmt.Sprintf("steam_%s", player.SteamId)
	}
	return ""
}

// listOnlinePlayers godoc
//
//	@Summary		List Online Players
//	@Description	List Online Players
//	@Tags			Player
//	@Accept			json
//	@Produce		json
//
//	@Success		200	{object}	[]database.OnlinePlayer
//	@Failure		400	{object}	ErrorResponse
//	@Router			/api/online_player [get]
func listOnlinePlayers(c *gin.Context) {
	onlinePLayers, err := tool.ShowPlayers()
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	service.PutPlayersOnline(database.GetDB(), onlinePLayers)
	// 未登录隐藏敏感字段
	if !c.GetBool("loggedIn") {
		for i := range onlinePLayers {
			onlinePLayers[i].Ip = ""
			if onlinePLayers[i].UserId != "" {
				onlinePLayers[i].UserId = strings.Split(onlinePLayers[i].UserId, "_")[0] + "_"
			}
			onlinePLayers[i].SteamId = ""
		}
	}
	c.JSON(http.StatusOK, onlinePLayers)
}

// putPlayers godoc
//
//	@Summary		Put Players
//	@Description	Put Players Only For SavSync,PlayerSync
//	@Tags			Player
//	@Accept			json
//	@Produce		json
//
//	@Security		ApiKeyAuth
//
//	@Param			players	body		[]database.Player	true	"Players"
//
//	@Success		200		{object}	SuccessResponse
//	@Failure		400		{object}	ErrorResponse
//	@Failure		401		{object}	ErrorResponse
//	@Router			/api/player [put]
func putPlayers(c *gin.Context) {
	var players []database.Player
	if err := c.ShouldBindJSON(&players); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if err := service.PutPlayers(database.GetDB(), players); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true})
}

// listPlayers godoc
//
//	@Summary		List Players
//	@Description	List Players
//	@Tags			Player
//	@Accept			json
//	@Produce		json
//
//	@Param			order_by	query		PlayerOrderBy	false	"order by field"	enum(last_online,level)
//	@Param			desc		query		bool			false	"order by desc"
//
//	@Success		200			{object}	[]database.TersePlayer
//	@Failure		400			{object}	ErrorResponse
//	@Router			/api/player [get]
func listPlayers(c *gin.Context) {
	orderBy := c.Query("order_by")
	desc := c.Query("desc")
	players, err := service.ListPlayers(database.GetDB())
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	//未登录隐藏字段
	if !c.GetBool("loggedIn") {
		for i := range players {
			players[i].Ip = ""
			if players[i].UserId != "" {
				players[i].UserId = strings.Split(players[i].UserId, "_")[0] + "_"
			}
			players[i].SteamId = ""
		}
	}
	//排序
	if orderBy == "level" {
		sort.Slice(players, func(i, j int) bool {
			if desc == "true" {
				return players[i].Level > players[j].Level
			}
			return players[i].Level < players[j].Level
		})
	}
	if orderBy == "last_online" {
		sort.Slice(players, func(i, j int) bool {
			if desc == "true" {
				return players[i].LastOnline.Sub(players[j].LastOnline) > 0
			}
			return players[i].LastOnline.Sub(players[j].LastOnline) < 0
		})
	}
	c.JSON(http.StatusOK, players)
}

// getPlayer godoc
//
//	@Summary		Get Player
//	@Description	Get Player
//	@Tags			Player
//	@Accept			json
//	@Produce		json
//
//	@Param			player_uid	path		string	true	"Player UID"
//
//	@Success		200			{object}	database.Player
//	@Failure		400			{object}	ErrorResponse
//	@Failure		404			{object}	EmptyResponse
//	@Router			/api/player/{player_uid} [get]
func getPlayer(c *gin.Context) {
	player, err := service.GetPlayer(database.GetDB(), c.Param("player_uid"))
	if err != nil {
		if err == service.ErrNoRecord {
			c.JSON(http.StatusNotFound, gin.H{})
			return
		}
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	//未登录隐藏字段
	if !c.GetBool("loggedIn") {
		player.Ip = ""
		if player.UserId != "" {
			player.UserId = strings.Split(player.UserId, "_")[0] + "_"
		}
		player.SteamId = ""
	}
	c.JSON(http.StatusOK, player)
}

// kickPlayer godoc
//
//	@Summary		Kick Player
//	@Description	Kick Player
//	@Tags			Player
//	@Accept			json
//	@Produce		json
//	@Security		ApiKeyAuth
//	@Param			player_uid	path		string	true	"Player UID"
//	@Param			action		body		PlayerActionRequest	false	"Optional kick message"
//
//	@Success		200			{object}	SuccessResponse
//	@Failure		400			{object}	ErrorResponse
//	@Failure		401			{object}	ErrorResponse
//	@Failure		404			{object}	ErrorResponse
//	@Router			/api/player/{player_uid}/kick [post]
func kickPlayer(c *gin.Context) {
	req, err := bindOptionalPlayerAction(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	playerUid := c.Param("player_uid")
	player, err := service.GetPlayer(database.GetDB(), playerUid)
	if err != nil {
		if err == service.ErrNoRecord {
			c.JSON(http.StatusNotFound, gin.H{"error": "Player not found"})
			return
		}
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	err = tool.KickPlayer(getPlayerActionUserId(player), req.Message)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true})
}

// banPlayer godoc
//
//	@Summary		Ban Player
//	@Description	Ban Player
//	@Tags			Player
//	@Accept			json
//	@Produce		json
//	@Security		ApiKeyAuth
//	@Param			player_uid	path		string	true	"Player UID"
//	@Param			action		body		PlayerActionRequest	false	"Optional ban message"
//
//	@Success		200			{object}	SuccessResponse
//	@Failure		400			{object}	ErrorResponse
//	@Failure		401			{object}	ErrorResponse
//	@Failure		404			{object}	ErrorResponse
//	@Router			/api/player/{player_uid}/ban [post]
func banPlayer(c *gin.Context) {
	req, err := bindOptionalPlayerAction(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	playerUid := c.Param("player_uid")
	player, err := service.GetPlayer(database.GetDB(), playerUid)
	if err != nil {
		if err == service.ErrNoRecord {
			c.JSON(http.StatusNotFound, gin.H{"error": "Player not found"})
			return
		}
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	err = tool.BanPlayer(getPlayerActionUserId(player), req.Message)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true})
}

// unbanPlayer godoc
//
//	@Summary		Unban Player
//	@Description	Unban Player
//	@Tags			Player
//	@Accept			json
//	@Produce		json
//	@Security		ApiKeyAuth
//	@Param			player_uid	path		string	true	"Player UID"
//
//	@Success		200			{object}	SuccessResponse
//	@Failure		400			{object}	ErrorResponse
//	@Failure		401			{object}	ErrorResponse
//	@Failure		404			{object}	ErrorResponse
//	@Router			/api/player/{player_uid}/unban [post]
func unbanPlayer(c *gin.Context) {
	playerUid := c.Param("player_uid")
	player, err := service.GetPlayer(database.GetDB(), playerUid)
	if err != nil {
		if err == service.ErrNoRecord {
			c.JSON(http.StatusNotFound, gin.H{"error": "Player not found"})
			return
		}
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	err = tool.UnbanPlayer(getPlayerActionUserId(player))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true})
}

// givePlayerItem godoc
//
//	@Summary		Deliver an item through an offline save transaction
//	@Description	Create a backup, edit a stopped local server save, validate it, and atomically replace Level.sav
//	@Tags			Player
//	@Accept			json
//	@Produce		json
//	@Security		ApiKeyAuth
//	@Param			player_uid	path		string			true	"Player UID"
//	@Param			delivery	body		GiveItemRequest	true	"Item delivery"
//	@Success		200			{object}	GiveItemResponse
//	@Failure		400			{object}	ErrorResponse
//	@Failure		401			{object}	ErrorResponse
//	@Failure		409			{object}	ErrorResponse
//	@Failure		422			{object}	ErrorResponse
//	@Failure		503			{object}	ErrorResponse
//	@Router			/api/player/{player_uid}/items [post]
func givePlayerItem(c *gin.Context) {
	var req GiveItemRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	release, ok := beginManualOperation(c, nil)
	if !ok {
		return
	}
	defer release()
	result, err := tool.GivePlayerItem(database.GetDB(), tool.GiveItemOptions{
		PlayerUID:            c.Param("player_uid"),
		ItemID:               req.ItemID,
		Quantity:             req.Quantity,
		Container:            req.Container,
		ConfirmServerStopped: req.ConfirmServerStopped,
	})
	if err != nil {
		writeSaveEditError(c, err)
		return
	}
	response := GiveItemResponse{
		Delivery: result.Delivery,
		Backup:   result.Backup,
	}
	if err := task.SavSyncNow(); err != nil {
		response.SyncError = err.Error()
	}
	c.JSON(http.StatusOK, response)
}

// setPlayerItemQuantity godoc
//
//	@Summary		Update or remove one inventory slot through an offline save transaction
//	@Description	Create a backup, verify the expected item and quantity, edit a stopped local server save, validate it, and atomically replace Level.sav
//	@Tags			Player
//	@Accept			json
//	@Produce		json
//	@Security		ApiKeyAuth
//	@Param			player_uid	path		string					true	"Player UID"
//	@Param			container	path		string					true	"Inventory container" Enums(main,key,weapons,armor,food,drop)
//	@Param			slot_index	path		int						true	"Inventory slot index"
//	@Param			mutation	body		SetItemQuantityRequest	true	"Inventory mutation"
//	@Success		200			{object}	SetItemQuantityResponse
//	@Failure		400			{object}	ErrorResponse
//	@Failure		401			{object}	ErrorResponse
//	@Failure		409			{object}	ErrorResponse
//	@Failure		422			{object}	ErrorResponse
//	@Failure		503			{object}	ErrorResponse
//	@Router			/api/player/{player_uid}/items/{container}/{slot_index} [patch]
func setPlayerItemQuantity(c *gin.Context) {
	var req SetItemQuantityRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	slotIndex, err := strconv.Atoi(c.Param("slot_index"))
	if err != nil || slotIndex < 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "slot index must be zero or greater"})
		return
	}
	release, ok := beginManualOperation(c, nil)
	if !ok {
		return
	}
	defer release()
	result, err := tool.SetPlayerItemQuantity(database.GetDB(), tool.SetItemQuantityOptions{
		PlayerUID:            c.Param("player_uid"),
		Container:            c.Param("container"),
		SlotIndex:            slotIndex,
		ItemID:               req.ItemID,
		ExpectedQuantity:     req.ExpectedQuantity,
		ExpectedDynamicID:    req.ExpectedDynamicID,
		Quantity:             *req.Quantity,
		ConfirmServerStopped: req.ConfirmServerStopped,
	})
	if err != nil {
		writeSaveEditError(c, err)
		return
	}
	response := SetItemQuantityResponse{
		Mutation: result.Mutation,
		Backup:   result.Backup,
	}
	if err := task.SavSyncNow(); err != nil {
		response.SyncError = err.Error()
	}
	c.JSON(http.StatusOK, response)
}

// editPlayerProfile godoc
//
//	@Summary		Edit a player nickname and level through an offline save transaction
//	@Description	Create a backup, verify the expected profile, update the character and guild records, set cumulative EXP from bundled game data when the level changes, validate, and atomically replace Level.sav
//	@Tags			Player
//	@Accept			json
//	@Produce		json
//	@Security		ApiKeyAuth
//	@Param			player_uid	path		string					true	"Player UID"
//	@Param			profile		body		EditPlayerProfileRequest	true	"Player profile mutation"
//	@Success		200			{object}	EditPlayerProfileResponse
//	@Failure		400			{object}	ErrorResponse
//	@Failure		401			{object}	ErrorResponse
//	@Failure		409			{object}	ErrorResponse
//	@Failure		422			{object}	ErrorResponse
//	@Failure		503			{object}	ErrorResponse
//	@Router			/api/player/{player_uid}/profile [patch]
func editPlayerProfile(c *gin.Context) {
	var req EditPlayerProfileRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	release, ok := beginManualOperation(c, nil)
	if !ok {
		return
	}
	defer release()
	result, err := tool.EditPlayerProfile(database.GetDB(), tool.EditPlayerProfileOptions{
		PlayerUID:            c.Param("player_uid"),
		ExpectedNickname:     req.ExpectedNickname,
		ExpectedLevel:        req.ExpectedLevel,
		Nickname:             req.Nickname,
		Level:                req.Level,
		ConfirmServerStopped: req.ConfirmServerStopped,
	})
	if err != nil {
		writeSaveEditError(c, err)
		return
	}
	response := EditPlayerProfileResponse{
		Profile: result.Profile,
		Backup:  result.Backup,
	}
	if err := task.SavSyncNow(); err != nil {
		response.SyncError = err.Error()
	}
	c.JSON(http.StatusOK, response)
}

// editPlayerStatPoints godoc
//
//	@Summary		Edit a player's unspent stat points through an offline save transaction
//	@Description	Create a backup, verify the current UInt16 value, update only unspent stat points in Level.sav, validate, and atomically replace it
//	@Tags			Player
//	@Accept			json
//	@Produce		json
//	@Security		ApiKeyAuth
//	@Param			player_uid	path		string					true	"Player UID"
//	@Param			stat_points	body		EditPlayerStatPointsRequest	true	"Unspent stat points mutation"
//	@Success		200			{object}	EditPlayerStatPointsResponse
//	@Failure		400			{object}	ErrorResponse
//	@Failure		401			{object}	ErrorResponse
//	@Failure		409			{object}	ErrorResponse
//	@Failure		422			{object}	ErrorResponse
//	@Failure		500			{object}	ErrorResponse
//	@Failure		503			{object}	ErrorResponse
//	@Router			/api/player/{player_uid}/stat-points [patch]
func editPlayerStatPoints(c *gin.Context) {
	var req EditPlayerStatPointsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: err.Error()})
		return
	}
	release, ok := beginManualOperation(c, nil)
	if !ok {
		return
	}
	defer release()
	result, err := tool.EditPlayerStatPoints(database.GetDB(), tool.EditPlayerStatPointsOptions{
		PlayerUID:                c.Param("player_uid"),
		ExpectedUnusedStatPoints: *req.ExpectedUnusedStatPoints,
		UnusedStatPoints:         *req.UnusedStatPoints,
		ConfirmServerStopped:     req.ConfirmServerStopped,
	})
	if err != nil {
		writeSaveEditError(c, err)
		return
	}
	response := EditPlayerStatPointsResponse{
		StatPoints: result.StatPoints,
		Backup:     result.Backup,
	}
	if err := task.SavSyncNow(); err != nil {
		response.SyncError = err.Error()
	}
	c.JSON(http.StatusOK, response)
}

// editPlayerTechnologyPoints godoc
//
//	@Summary		Edit a player's technology point balances through an offline save transaction
//	@Description	Create a full backup, verify Level.sav and the target Players/*.sav, update only normal and ancient technology point balances, validate, and atomically replace the player save
//	@Tags			Player
//	@Accept			json
//	@Produce		json
//	@Security		ApiKeyAuth
//	@Param			player_uid	path		string							true	"Player UID"
//	@Param			technology	body		EditPlayerTechnologyPointsRequest	true	"Technology points mutation"
//	@Success		200			{object}	EditPlayerTechnologyPointsResponse
//	@Failure		400			{object}	ErrorResponse
//	@Failure		401			{object}	ErrorResponse
//	@Failure		409			{object}	ErrorResponse
//	@Failure		422			{object}	ErrorResponse
//	@Failure		500			{object}	ErrorResponse
//	@Failure		503			{object}	ErrorResponse
//	@Router			/api/player/{player_uid}/technology-points [patch]
func editPlayerTechnologyPoints(c *gin.Context) {
	var req EditPlayerTechnologyPointsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: err.Error()})
		return
	}
	release, ok := beginManualOperation(c, nil)
	if !ok {
		return
	}
	defer release()
	result, err := tool.EditPlayerTechnologyPoints(database.GetDB(), tool.EditPlayerTechnologyPointsOptions{
		PlayerUID:                       c.Param("player_uid"),
		ExpectedTechnologyPoints:        *req.ExpectedTechnologyPoints,
		ExpectedAncientTechnologyPoints: *req.ExpectedAncientTechnologyPoints,
		TechnologyPoints:                *req.TechnologyPoints,
		AncientTechnologyPoints:         *req.AncientTechnologyPoints,
		ConfirmServerStopped:            req.ConfirmServerStopped,
	})
	if err != nil {
		writeSaveEditError(c, err)
		return
	}
	response := EditPlayerTechnologyPointsResponse{
		TechnologyPoints: result.TechnologyPoints,
		Backup:           result.Backup,
	}
	if err := task.SavSyncNow(); err != nil {
		response.SyncError = err.Error()
	}
	c.JSON(http.StatusOK, response)
}

// unlockPlayerMap godoc
//
//	@Summary		Unlock a player's complete 1.0.0 map and fast-travel progress through an offline save transaction
//	@Description	Create a full backup, verify Level.sav and the target Players/*.sav, compare the complete map-progress digest, merge all bundled 1.0.0 fast-travel points, areas, and world-map flags while preserving unknown entries, validate, and atomically replace the player save
//	@Tags			Player
//	@Accept			json
//	@Produce		json
//	@Security		ApiKeyAuth
//	@Param			player_uid	path		string					true	"Player UID"
//	@Param			map_progress	body		UnlockPlayerMapRequest	true	"Map progress unlock"
//	@Success		200			{object}	UnlockPlayerMapResponse
//	@Failure		400			{object}	ErrorResponse
//	@Failure		401			{object}	ErrorResponse
//	@Failure		409			{object}	ErrorResponse
//	@Failure		422			{object}	ErrorResponse
//	@Failure		500			{object}	ErrorResponse
//	@Failure		503			{object}	ErrorResponse
//	@Router			/api/player/{player_uid}/map-progress [patch]
func unlockPlayerMap(c *gin.Context) {
	var req UnlockPlayerMapRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: err.Error()})
		return
	}
	release, ok := beginManualOperation(c, nil)
	if !ok {
		return
	}
	defer release()
	result, err := tool.UnlockPlayerMap(database.GetDB(), tool.UnlockPlayerMapOptions{
		PlayerUID:              c.Param("player_uid"),
		ExpectedProgressDigest: *req.ExpectedProgressDigest,
		ConfirmServerStopped:   req.ConfirmServerStopped,
	})
	if err != nil {
		writeSaveEditError(c, err)
		return
	}
	response := UnlockPlayerMapResponse{
		MapProgress: result.MapProgress,
		Backup:      result.Backup,
	}
	if err := task.SavSyncNow(); err != nil {
		response.SyncError = err.Error()
	}
	c.JSON(http.StatusOK, response)
}

// renamePal godoc
//
//	@Summary		Rename one Pal through an offline save transaction
//	@Description	Create a backup, locate one non-player record by instance ID, verify owner and expected nickname/level/EXP, update only NickName, validate, and atomically replace Level.sav
//	@Tags			Player
//	@Accept			json
//	@Produce		json
//	@Security		ApiKeyAuth
//	@Param			player_uid	path		string			true	"Player UID"
//	@Param			instance_id	path		string			true	"Pal instance ID"
//	@Param			nickname	body		RenamePalRequest	true	"Pal nickname mutation"
//	@Success		200			{object}	RenamePalResponse
//	@Failure		400			{object}	ErrorResponse
//	@Failure		401			{object}	ErrorResponse
//	@Failure		409			{object}	ErrorResponse
//	@Failure		422			{object}	ErrorResponse
//	@Failure		500			{object}	ErrorResponse
//	@Failure		503			{object}	ErrorResponse
//	@Router			/api/player/{player_uid}/pals/{instance_id}/nickname [patch]
func renamePal(c *gin.Context) {
	var req RenamePalRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: err.Error()})
		return
	}
	release, ok := beginManualOperation(c, nil)
	if !ok {
		return
	}
	defer release()
	result, err := tool.RenamePal(database.GetDB(), tool.RenamePalOptions{
		PlayerUID:            c.Param("player_uid"),
		InstanceID:           c.Param("instance_id"),
		ExpectedNickname:     *req.ExpectedNickname,
		ExpectedLevel:        *req.ExpectedLevel,
		ExpectedExp:          *req.ExpectedExp,
		Nickname:             *req.Nickname,
		ConfirmServerStopped: req.ConfirmServerStopped,
	})
	if err != nil {
		writeSaveEditError(c, err)
		return
	}
	response := RenamePalResponse{
		Nickname: result.Nickname,
		Backup:   result.Backup,
	}
	if err := task.SavSyncNow(); err != nil {
		response.SyncError = err.Error()
	}
	c.JSON(http.StatusOK, response)
}

// editPalLevel godoc
//
//	@Summary		Edit one Pal level through an offline save transaction
//	@Description	Create a backup, locate one non-player record by instance ID, verify owner and expected nickname/level/EXP/HP/MaxHP, set cumulative Pal EXP and 1.0-derived HP, validate, and atomically replace Level.sav
//	@Tags			Player
//	@Accept			json
//	@Produce		json
//	@Security		ApiKeyAuth
//	@Param			player_uid	path		string				true	"Player UID"
//	@Param			instance_id	path		string				true	"Pal instance ID"
//	@Param			level		body		EditPalLevelRequest	true	"Pal level mutation"
//	@Success		200			{object}	EditPalLevelResponse
//	@Failure		400			{object}	ErrorResponse
//	@Failure		401			{object}	ErrorResponse
//	@Failure		409			{object}	ErrorResponse
//	@Failure		422			{object}	ErrorResponse
//	@Failure		500			{object}	ErrorResponse
//	@Failure		503			{object}	ErrorResponse
//	@Router			/api/player/{player_uid}/pals/{instance_id}/level [patch]
func editPalLevel(c *gin.Context) {
	var req EditPalLevelRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: err.Error()})
		return
	}
	release, ok := beginManualOperation(c, nil)
	if !ok {
		return
	}
	defer release()
	result, err := tool.EditPalLevel(database.GetDB(), tool.EditPalLevelOptions{
		PlayerUID:            c.Param("player_uid"),
		InstanceID:           c.Param("instance_id"),
		ExpectedNickname:     *req.ExpectedNickname,
		ExpectedLevel:        *req.ExpectedLevel,
		ExpectedExp:          *req.ExpectedExp,
		ExpectedHP:           *req.ExpectedHP,
		ExpectedMaxHP:        *req.ExpectedMaxHP,
		Level:                *req.Level,
		ConfirmServerStopped: req.ConfirmServerStopped,
	})
	if err != nil {
		writeSaveEditError(c, err)
		return
	}
	response := EditPalLevelResponse{
		Level:  result.Level,
		Backup: result.Backup,
	}
	if err := task.SavSyncNow(); err != nil {
		response.SyncError = err.Error()
	}
	c.JSON(http.StatusOK, response)
}

// restorePalHealth godoc
//
//	@Summary		Restore one Pal to full health through an offline save transaction
//	@Description	Create a backup, locate one non-player record by instance ID, verify owner and expected nickname/level/EXP/HP/MaxHP, restore only the existing Hp or HP field to MaxHP, validate, and atomically replace Level.sav
//	@Tags			Player
//	@Accept			json
//	@Produce		json
//	@Security		ApiKeyAuth
//	@Param			player_uid	path		string					true	"Player UID"
//	@Param			instance_id	path		string					true	"Pal instance ID"
//	@Param			health		body		RestorePalHealthRequest	true	"Pal health restoration"
//	@Success		200			{object}	RestorePalHealthResponse
//	@Failure		400			{object}	ErrorResponse
//	@Failure		401			{object}	ErrorResponse
//	@Failure		409			{object}	ErrorResponse
//	@Failure		422			{object}	ErrorResponse
//	@Failure		500			{object}	ErrorResponse
//	@Failure		503			{object}	ErrorResponse
//	@Router			/api/player/{player_uid}/pals/{instance_id}/health [patch]
func restorePalHealth(c *gin.Context) {
	var req RestorePalHealthRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: err.Error()})
		return
	}
	release, ok := beginManualOperation(c, nil)
	if !ok {
		return
	}
	defer release()
	result, err := tool.RestorePalHealth(database.GetDB(), tool.RestorePalHealthOptions{
		PlayerUID:            c.Param("player_uid"),
		InstanceID:           c.Param("instance_id"),
		ExpectedNickname:     *req.ExpectedNickname,
		ExpectedLevel:        *req.ExpectedLevel,
		ExpectedExp:          *req.ExpectedExp,
		ExpectedHP:           *req.ExpectedHP,
		ExpectedMaxHP:        *req.ExpectedMaxHP,
		ConfirmServerStopped: req.ConfirmServerStopped,
	})
	if err != nil {
		writeSaveEditError(c, err)
		return
	}
	response := RestorePalHealthResponse{
		Health: result.Health,
		Backup: result.Backup,
	}
	if err := task.SavSyncNow(); err != nil {
		response.SyncError = err.Error()
	}
	c.JSON(http.StatusOK, response)
}

// addWhite godoc
//
//	@Summary		Add White List
//	@Description	Add White List
//	@Tags			Player
//	@Accept			json
//	@Produce		json
//	@Security		ApiKeyAuth
//	@Param			player_uid	path		string	true	"Player UID"
//
//	@Success		200			{object}	SuccessResponse
//	@Failure		400			{object}	ErrorResponse
//	@Failure		401			{object}	ErrorResponse
//	@Router			/api/whitelist [post]
func addWhite(c *gin.Context) {
	var player database.PlayerW
	if err := c.ShouldBindJSON(&player); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if err := service.AddWhitelist(database.GetDB(), player); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true})
}

// listWhite godoc
//
//	@Summary		List White List
//	@Description	List White List
//	@Tags			Player
//	@Accept			json
//	@Produce		json
//	@Success		200	{object}	[]database.PlayerW
//	@Failure		400	{object}	ErrorResponse
//	@Router			/api/whitelist [get]
func listWhite(c *gin.Context) {
	players, err := service.ListWhitelist(database.GetDB())
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, players)
}

// removeWhite godoc
//
//	@Summary		Remove White List
//	@Description	Remove White List
//	@Tags			Player
//	@Accept			json
//	@Produce		json
//	@Security		ApiKeyAuth
//	@Param			player_uid	path		string	true	"Player UID"
//
//	@Success		200			{object}	SuccessResponse
//	@Failure		400			{object}	ErrorResponse
//	@Failure		401			{object}	ErrorResponse
//	@Router			/api/whitelist [delete]
func removeWhite(c *gin.Context) {
	var player database.PlayerW
	if err := c.ShouldBindJSON(&player); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if err := service.RemoveWhitelist(database.GetDB(), player); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true})
}

// putWhite godoc
//
//	@Summary		Put White List
//	@Description	Put White List
//	@Tags			Player
//	@Accept			json
//	@Produce		json
//	@Security		ApiKeyAuth
//	@Param			players	body		[]database.PlayerW	true	"Players"
//
//	@Success		200		{object}	SuccessResponse
//	@Failure		400		{object}	ErrorResponse
//	@Failure		401		{object}	ErrorResponse
//	@Router			/api/whitelist [put]
func putWhite(c *gin.Context) {
	var players []database.PlayerW
	if err := c.ShouldBindJSON(&players); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if err := service.PutWhitelist(database.GetDB(), players); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true})
}
