package tool

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/viper"
	"github.com/zaigie/palworld-server-tool/internal/database"
	"github.com/zaigie/palworld-server-tool/internal/logger"
)

var client = &http.Client{}

type RESTError struct {
	StatusCode int
	Body       string
}

func (e *RESTError) Error() string {
	return fmt.Sprintf("rest: %d %s", e.StatusCode, e.Body)
}

func callAPI(method, endpoint string, body []byte) ([]byte, error) {
	address := strings.TrimSpace(viper.GetString("rest.address"))
	if address == "" {
		return nil, fmt.Errorf("rest address is empty")
	}

	requestURL, err := url.JoinPath(address, endpoint)
	if err != nil {
		return nil, fmt.Errorf("build REST API URL: %w", err)
	}

	ctx := context.Background()
	if timeout := viper.GetInt("rest.timeout"); timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, time.Duration(timeout)*time.Second)
		defer cancel()
	}

	req, err := http.NewRequestWithContext(ctx, method, requestURL, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("create REST API request: %w", err)
	}
	req.Header.Set("Accept", "application/json")
	if len(body) > 0 {
		req.Header.Set("Content-Type", "application/json")
	}
	req.SetBasicAuth(viper.GetString("rest.username"), viper.GetString("rest.password"))

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	responseBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return nil, &RESTError{StatusCode: resp.StatusCode, Body: strings.TrimSpace(string(responseBody))}
	}
	return responseBody, nil
}

type ResponseInfo struct {
	Version     string `json:"version"`
	ServerName  string `json:"servername"`
	Description string `json:"description"`
	WorldGUID   string `json:"worldguid"`
}

func Info() (ResponseInfo, error) {
	resp, err := callAPI(http.MethodGet, "/v1/api/info", nil)
	if err != nil {
		return ResponseInfo{}, err
	}
	var data ResponseInfo
	if err := json.Unmarshal(resp, &data); err != nil {
		return ResponseInfo{}, err
	}
	return data, nil
}

type ResponseMetrics struct {
	ServerFps        int     `json:"serverfps"`
	CurrentPlayerNum int     `json:"currentplayernum"`
	ServerFrameTime  float64 `json:"serverframetime"`
	MaxPlayerNum     int     `json:"maxplayernum"`
	Uptime           int     `json:"uptime"`
	BaseCampNum      int     `json:"basecampnum"`
	Days             int     `json:"days"`
}

func Metrics() (ResponseMetrics, error) {
	resp, err := callAPI(http.MethodGet, "/v1/api/metrics", nil)
	if err != nil {
		return ResponseMetrics{}, err
	}
	var data ResponseMetrics
	if err := json.Unmarshal(resp, &data); err != nil {
		return ResponseMetrics{}, err
	}
	data.ServerFrameTime = math.Round(data.ServerFrameTime*100) / 100
	return data, nil
}

func Settings() (map[string]interface{}, error) {
	resp, err := callAPI(http.MethodGet, "/v1/api/settings", nil)
	if err != nil {
		return nil, err
	}
	var data map[string]interface{}
	if err := json.Unmarshal(resp, &data); err != nil {
		return nil, err
	}
	return data, nil
}

func WorldActorSnapshot() (json.RawMessage, error) {
	resp, err := callAPI(http.MethodGet, "/v1/api/game-data", nil)
	if err != nil {
		return nil, err
	}
	if !json.Valid(resp) {
		return nil, fmt.Errorf("invalid world actor snapshot response")
	}
	return json.RawMessage(resp), nil
}

type ResponsePlayer struct {
	Name          string  `json:"name"`
	AccountName   string  `json:"accountName"`
	PlayerId      string  `json:"playerId"`
	UserId        string  `json:"userId"`
	Ip            string  `json:"ip"`
	Ping          float64 `json:"ping"`
	LocationX     float64 `json:"location_x"`
	LocationY     float64 `json:"location_y"`
	Level         int     `json:"level"`
	BuildingCount int     `json:"building_count"`
}

type ResponsePlayers struct {
	Players []ResponsePlayer `json:"players"`
}

func ShowPlayers() ([]database.OnlinePlayer, error) {
	resp, err := callAPI(http.MethodGet, "/v1/api/players", nil)
	if err != nil {
		return nil, err
	}
	var data ResponsePlayers
	if err := json.Unmarshal(resp, &data); err != nil {
		return nil, err
	}
	onlinePlayers := make([]database.OnlinePlayer, 0, len(data.Players))
	now := time.Now()
	for _, player := range data.Players {
		onlinePlayers = append(onlinePlayers, database.OnlinePlayer{
			PlayerUid:     getPlayerUid(player.PlayerId),
			UserId:        player.UserId,
			AccountName:   player.AccountName,
			SteamId:       getSteamId(player.UserId),
			Nickname:      player.Name,
			Ip:            player.Ip,
			Ping:          player.Ping,
			LocationX:     player.LocationX,
			LocationY:     player.LocationY,
			Level:         int32(player.Level),
			BuildingCount: int32(player.BuildingCount),
			LastOnline:    now,
		})
	}
	return onlinePlayers, nil
}

func getSteamId(userId string) string {
	if userId != "" && strings.HasPrefix(userId, "steam_") {
		return strings.TrimPrefix(userId, "steam_")
	}
	return ""
}

func getPlayerUid(playerId string) string {
	if len(playerId) < 8 {
		logger.Errorf("Parse PlayerId fail: %s\n", playerId)
		return ""
	}
	hexPart := playerId[:8]
	decimalValue, err := strconv.ParseUint(hexPart, 16, 32)
	if err != nil {
		logger.Errorf("Parse PlayerId fail: %s\n", err)
		return ""
	}
	return strconv.FormatUint(decimalValue, 10)
}

type RequestPlayerAction struct {
	UserId  string `json:"userid"`
	Message string `json:"message,omitempty"`
}

func playerAction(endpoint, userID, message string) error {
	if strings.TrimSpace(userID) == "" {
		return fmt.Errorf("user ID cannot be empty")
	}
	body, err := json.Marshal(RequestPlayerAction{UserId: userID, Message: message})
	if err != nil {
		return err
	}
	_, err = callAPI(http.MethodPost, endpoint, body)
	return err
}

func actionMessage(messages []string) string {
	if len(messages) == 0 {
		return ""
	}
	return messages[0]
}

func KickPlayer(userID string, message ...string) error {
	return playerAction("/v1/api/kick", userID, actionMessage(message))
}

func BanPlayer(userID string, message ...string) error {
	return playerAction("/v1/api/ban", userID, actionMessage(message))
}

func UnbanPlayer(userID string) error {
	return playerAction("/v1/api/unban", userID, "")
}

type RequestBroadcast struct {
	Message string `json:"message"`
}

func Broadcast(message string) error {
	body, err := json.Marshal(RequestBroadcast{Message: message})
	if err != nil {
		return err
	}
	_, err = callAPI(http.MethodPost, "/v1/api/announce", body)
	return err
}

type RequestShutdown struct {
	Waittime int    `json:"waittime"`
	Message  string `json:"message,omitempty"`
}

func Shutdown(seconds int, message string) error {
	body, err := json.Marshal(RequestShutdown{Waittime: seconds, Message: message})
	if err != nil {
		return err
	}
	_, err = callAPI(http.MethodPost, "/v1/api/shutdown", body)
	return err
}

func SaveWorld() error {
	_, err := callAPI(http.MethodPost, "/v1/api/save", nil)
	return err
}

func StopServer() error {
	_, err := callAPI(http.MethodPost, "/v1/api/stop", nil)
	return err
}
