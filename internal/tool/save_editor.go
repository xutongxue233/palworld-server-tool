package tool

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode"

	"github.com/spf13/viper"
	"github.com/zaigie/palworld-server-tool/internal/database"
	"github.com/zaigie/palworld-server-tool/internal/source"
	"github.com/zaigie/palworld-server-tool/internal/system"
	"go.etcd.io/bbolt"
)

var (
	ErrSaveEditConfirmation    = errors.New("confirm that the game server is stopped")
	ErrGameServerRunning       = errors.New("the game server is still running")
	ErrGameServerStatusUnknown = errors.New("could not verify that the game server is stopped")
	ErrSaveSourceChanged       = errors.New("save data changed during save editing")
	ErrUnsupportedSaveSource   = errors.New("save editing currently requires a local save path")
	ErrSaveEditInternal        = errors.New("internal save edit failure")
	saveEditorMu               sync.Mutex
	saveEditorTimeout          = 10 * time.Minute
)

const (
	playerMapGameVersion     = "1.0.0"
	playerMapFastTravelTotal = 174
	playerMapAreasTotal      = 123
	playerMapWorldMapsTotal  = 2
)

type gameServerStatus uint8

const (
	gameServerStatusExplicitOffline gameServerStatus = iota
	gameServerStatusOnline
	gameServerStatusUnknown
)

type saveFileFingerprint struct {
	Size    int64
	ModTime time.Time
	SHA256  [sha256.Size]byte
}

type GiveItemOptions struct {
	PlayerUID            string
	ItemID               string
	Quantity             int
	Container            string
	ConfirmServerStopped bool
}

type SetItemQuantityOptions struct {
	PlayerUID            string
	Container            string
	SlotIndex            int
	ItemID               string
	ExpectedDynamicID    string
	ExpectedQuantity     int
	Quantity             int
	ConfirmServerStopped bool
}

type ItemDelivery struct {
	PlayerUID     string         `json:"player_uid"`
	ItemID        string         `json:"item_id"`
	Container     string         `json:"container"`
	Requested     int            `json:"requested"`
	Delivered     int            `json:"delivered"`
	Before        int            `json:"before"`
	After         int            `json:"after"`
	ModifiedSlots []int          `json:"modified_slots"`
	DynamicIDs    map[int]string `json:"dynamic_ids"`
}

type GiveItemResult struct {
	Delivery ItemDelivery    `json:"delivery"`
	Backup   database.Backup `json:"backup"`
}

type InventoryMutation struct {
	PlayerUID            string `json:"player_uid"`
	ItemID               string `json:"item_id"`
	Container            string `json:"container"`
	SlotIndex            int    `json:"slot_index"`
	DynamicID            string `json:"dynamic_id"`
	Before               int    `json:"before"`
	After                int    `json:"after"`
	Removed              bool   `json:"removed"`
	DynamicRecordRemoved bool   `json:"dynamic_record_removed"`
}

type SetItemQuantityResult struct {
	Mutation InventoryMutation `json:"mutation"`
	Backup   database.Backup   `json:"backup"`
}

type EditPlayerProfileOptions struct {
	PlayerUID            string
	ExpectedNickname     string
	ExpectedLevel        int
	Nickname             string
	Level                int
	ConfirmServerStopped bool
}

type PlayerProfileMutation struct {
	PlayerUID        string `json:"player_uid"`
	NicknameBefore   string `json:"nickname_before"`
	NicknameAfter    string `json:"nickname_after"`
	LevelBefore      int    `json:"level_before"`
	LevelAfter       int    `json:"level_after"`
	ExpBefore        int64  `json:"exp_before"`
	ExpAfter         int64  `json:"exp_after"`
	CharacterRecords int    `json:"character_records"`
	GuildRecords     int    `json:"guild_records"`
}

type EditPlayerProfileResult struct {
	Profile PlayerProfileMutation `json:"profile"`
	Backup  database.Backup       `json:"backup"`
}

type EditPlayerStatPointsOptions struct {
	PlayerUID                string
	ExpectedUnusedStatPoints int
	UnusedStatPoints         int
	ConfirmServerStopped     bool
}

type PlayerStatPointsMutation struct {
	PlayerUID        string `json:"player_uid"`
	Before           int    `json:"before"`
	After            int    `json:"after"`
	CharacterRecords int    `json:"character_records"`
}

type saveEditorMachineError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

type EditPlayerStatPointsResult struct {
	StatPoints PlayerStatPointsMutation `json:"stat_points"`
	Backup     database.Backup          `json:"backup"`
}

type EditPlayerTechnologyPointsOptions struct {
	PlayerUID                       string
	ExpectedTechnologyPoints        int
	ExpectedAncientTechnologyPoints int
	TechnologyPoints                int
	AncientTechnologyPoints         int
	ConfirmServerStopped            bool
}

type PlayerTechnologyPointsMutation struct {
	PlayerUID        string   `json:"player_uid"`
	TechnologyBefore int      `json:"technology_before"`
	TechnologyAfter  int      `json:"technology_after"`
	AncientBefore    int      `json:"ancient_before"`
	AncientAfter     int      `json:"ancient_after"`
	CreatedFields    []string `json:"created_fields"`
}

type EditPlayerTechnologyPointsResult struct {
	TechnologyPoints PlayerTechnologyPointsMutation `json:"technology_points"`
	Backup           database.Backup                `json:"backup"`
}

type UnlockPlayerMapOptions struct {
	PlayerUID              string
	ExpectedProgressDigest string
	ConfirmServerStopped   bool
}

type PlayerMapProgressMutation struct {
	PlayerUID            string   `json:"player_uid"`
	FastTravelBefore     int      `json:"fast_travel_before"`
	FastTravelAfter      int      `json:"fast_travel_after"`
	FastTravelTotal      int      `json:"fast_travel_total"`
	AreasBefore          int      `json:"areas_before"`
	AreasAfter           int      `json:"areas_after"`
	AreasTotal           int      `json:"areas_total"`
	WorldMapsBefore      int      `json:"world_maps_before"`
	WorldMapsAfter       int      `json:"world_maps_after"`
	WorldMapsTotal       int      `json:"world_maps_total"`
	CreatedFields        []string `json:"created_fields"`
	ProgressDigestBefore string   `json:"progress_digest_before"`
	ProgressDigestAfter  string   `json:"progress_digest_after"`
	GameVersion          string   `json:"game_version"`
}

type UnlockPlayerMapResult struct {
	MapProgress PlayerMapProgressMutation `json:"map_progress"`
	Backup      database.Backup           `json:"backup"`
}

type RenamePalOptions struct {
	PlayerUID            string
	InstanceID           string
	ExpectedNickname     string
	ExpectedLevel        int
	ExpectedExp          int64
	Nickname             string
	ConfirmServerStopped bool
}

type PalNicknameMutation struct {
	PlayerUID       string `json:"player_uid"`
	InstanceID      string `json:"instance_id"`
	PalType         string `json:"pal_type"`
	NicknameBefore  string `json:"nickname_before"`
	NicknameAfter   string `json:"nickname_after"`
	Level           int    `json:"level"`
	Exp             int64  `json:"exp"`
	NicknameCreated bool   `json:"nickname_created"`
}

type RenamePalResult struct {
	Nickname PalNicknameMutation `json:"nickname"`
	Backup   database.Backup     `json:"backup"`
}

type EditPalLevelOptions struct {
	PlayerUID            string
	InstanceID           string
	ExpectedNickname     string
	ExpectedLevel        int
	ExpectedExp          int64
	ExpectedHP           int64
	ExpectedMaxHP        int64
	Level                int
	ConfirmServerStopped bool
}

type PalLevelMutation struct {
	PlayerUID    string `json:"player_uid"`
	InstanceID   string `json:"instance_id"`
	PalType      string `json:"pal_type"`
	Nickname     string `json:"nickname"`
	LevelBefore  int    `json:"level_before"`
	LevelAfter   int    `json:"level_after"`
	ExpBefore    int64  `json:"exp_before"`
	ExpAfter     int64  `json:"exp_after"`
	HPBefore     int64  `json:"hp_before"`
	HPAfter      int64  `json:"hp_after"`
	MaxHPBefore  int64  `json:"max_hp_before"`
	MaxHPAfter   int64  `json:"max_hp_after"`
	HealthField  string `json:"health_field"`
	MaxHPCreated bool   `json:"max_hp_created"`
}

type EditPalLevelResult struct {
	Level  PalLevelMutation `json:"level"`
	Backup database.Backup  `json:"backup"`
}

type RestorePalHealthOptions struct {
	PlayerUID            string
	InstanceID           string
	ExpectedNickname     string
	ExpectedLevel        int
	ExpectedExp          int64
	ExpectedHP           int64
	ExpectedMaxHP        int64
	ConfirmServerStopped bool
}

type PalHealthMutation struct {
	PlayerUID   string `json:"player_uid"`
	InstanceID  string `json:"instance_id"`
	PalType     string `json:"pal_type"`
	Nickname    string `json:"nickname"`
	Level       int    `json:"level"`
	Exp         int64  `json:"exp"`
	HPBefore    int64  `json:"hp_before"`
	HPAfter     int64  `json:"hp_after"`
	MaxHP       int64  `json:"max_hp"`
	HealthField string `json:"health_field"`
}

type RestorePalHealthResult struct {
	Health PalHealthMutation `json:"health"`
	Backup database.Backup   `json:"backup"`
}

func validateGiveItemOptions(options GiveItemOptions) error {
	if strings.TrimSpace(options.PlayerUID) == "" {
		return errors.New("player UID cannot be empty")
	}
	if strings.TrimSpace(options.ItemID) == "" {
		return errors.New("item ID cannot be empty")
	}
	if options.Quantity < 1 || options.Quantity > 999999 {
		return errors.New("quantity must be between 1 and 999999")
	}
	switch options.Container {
	case "", "auto", "main", "key":
	default:
		return fmt.Errorf("unsupported inventory container %q", options.Container)
	}
	if !options.ConfirmServerStopped {
		return ErrSaveEditConfirmation
	}
	return nil
}

func validateSetItemQuantityOptions(options SetItemQuantityOptions) error {
	if strings.TrimSpace(options.PlayerUID) == "" {
		return errors.New("player UID cannot be empty")
	}
	if strings.TrimSpace(options.ItemID) == "" {
		return errors.New("item ID cannot be empty")
	}
	if strings.TrimSpace(options.ExpectedDynamicID) == "" {
		return errors.New("expected dynamic ID cannot be empty")
	}
	if len(options.ExpectedDynamicID) > 36 {
		return errors.New("expected dynamic ID cannot exceed 36 characters")
	}
	if options.SlotIndex < 0 {
		return errors.New("slot index cannot be negative")
	}
	if options.ExpectedQuantity < 1 || options.ExpectedQuantity > 999999 {
		return errors.New("expected quantity must be between 1 and 999999")
	}
	if options.Quantity < 0 || options.Quantity > 999999 {
		return errors.New("quantity must be between 0 and 999999")
	}
	switch options.Container {
	case "main", "key", "weapons", "armor", "food", "drop":
	default:
		return fmt.Errorf("unsupported inventory container %q", options.Container)
	}
	if !options.ConfirmServerStopped {
		return ErrSaveEditConfirmation
	}
	return nil
}

func validateEditPlayerProfileOptions(options EditPlayerProfileOptions) error {
	if strings.TrimSpace(options.PlayerUID) == "" {
		return errors.New("player UID cannot be empty")
	}
	if strings.TrimSpace(options.ExpectedNickname) == "" {
		return errors.New("expected nickname cannot be empty")
	}
	if strings.TrimSpace(options.Nickname) == "" {
		return errors.New("nickname cannot be empty")
	}
	if len([]rune(strings.TrimSpace(options.Nickname))) > 32 {
		return errors.New("nickname cannot exceed 32 characters")
	}
	if options.ExpectedLevel < 1 || options.ExpectedLevel > 80 {
		return errors.New("expected level must be between 1 and 80")
	}
	if options.Level < 1 || options.Level > 80 {
		return errors.New("level must be between 1 and 80")
	}
	if strings.TrimSpace(options.Nickname) == options.ExpectedNickname &&
		options.Level == options.ExpectedLevel {
		return errors.New("player profile is unchanged")
	}
	if !options.ConfirmServerStopped {
		return ErrSaveEditConfirmation
	}
	return nil
}

func validateEditPlayerStatPointsOptions(options EditPlayerStatPointsOptions) error {
	if strings.TrimSpace(options.PlayerUID) == "" {
		return errors.New("player UID cannot be empty")
	}
	if options.ExpectedUnusedStatPoints < 0 || options.ExpectedUnusedStatPoints > 65535 {
		return errors.New("expected unused stat points must be between 0 and 65535")
	}
	if options.UnusedStatPoints < 0 || options.UnusedStatPoints > 65535 {
		return errors.New("unused stat points must be between 0 and 65535")
	}
	if options.UnusedStatPoints == options.ExpectedUnusedStatPoints {
		return errors.New("unused stat points are unchanged")
	}
	if !options.ConfirmServerStopped {
		return ErrSaveEditConfirmation
	}
	return nil
}

func validateEditPlayerTechnologyPointsOptions(options EditPlayerTechnologyPointsOptions) error {
	if strings.TrimSpace(options.PlayerUID) == "" {
		return errors.New("player UID cannot be empty")
	}
	values := []struct {
		label string
		value int
	}{
		{"expected technology points", options.ExpectedTechnologyPoints},
		{"expected ancient technology points", options.ExpectedAncientTechnologyPoints},
		{"technology points", options.TechnologyPoints},
		{"ancient technology points", options.AncientTechnologyPoints},
	}
	for _, value := range values {
		if value.value < 0 || value.value > 999999 {
			return fmt.Errorf("%s must be between 0 and 999999", value.label)
		}
	}
	if options.ExpectedTechnologyPoints == options.TechnologyPoints &&
		options.ExpectedAncientTechnologyPoints == options.AncientTechnologyPoints {
		return errors.New("player technology points are unchanged")
	}
	if !options.ConfirmServerStopped {
		return ErrSaveEditConfirmation
	}
	return nil
}

func canonicalSHA256Digest(value, label string) (string, error) {
	raw := strings.ToLower(strings.TrimSpace(value))
	decoded, err := hex.DecodeString(raw)
	if err != nil || len(decoded) != sha256.Size {
		return "", fmt.Errorf("%s must be a SHA-256 digest", label)
	}
	return raw, nil
}

func validateUnlockPlayerMapOptions(options UnlockPlayerMapOptions) error {
	if err := validatePlayerUID(options.PlayerUID, "player UID"); err != nil {
		return err
	}
	if _, err := canonicalSHA256Digest(options.ExpectedProgressDigest, "expected map progress digest"); err != nil {
		return err
	}
	if !options.ConfirmServerStopped {
		return ErrSaveEditConfirmation
	}
	return nil
}

func canonicalSaveGUID(value, label string, allowZero bool) (string, error) {
	raw := strings.ToLower(strings.ReplaceAll(strings.TrimSpace(value), "-", ""))
	if len(raw) != 32 {
		return "", fmt.Errorf("%s must be a GUID", label)
	}
	decoded, err := hex.DecodeString(raw)
	if err != nil {
		return "", fmt.Errorf("%s must be a GUID", label)
	}
	if !allowZero && bytes.Equal(decoded, make([]byte, len(decoded))) {
		return "", fmt.Errorf("%s cannot be zero", label)
	}
	return raw, nil
}

func validateSaveGUID(value, label string, allowZero bool) error {
	_, err := canonicalSaveGUID(value, label, allowZero)
	return err
}

func validatePalNickname(value, label string) error {
	if len([]rune(value)) > 32 {
		return fmt.Errorf("%s cannot exceed 32 characters", label)
	}
	for _, character := range value {
		if unicode.IsControl(character) {
			return fmt.Errorf("%s cannot contain control characters", label)
		}
	}
	return nil
}

func canonicalPlayerUID(value, label string) (string, error) {
	raw := strings.ToLower(strings.ReplaceAll(strings.TrimSpace(value), "-", ""))
	if raw == "" {
		return "", fmt.Errorf("%s cannot be empty", label)
	}
	if len(raw) <= 10 {
		decimal := true
		for _, character := range raw {
			if character < '0' || character > '9' {
				decimal = false
				break
			}
		}
		if decimal {
			numeric, err := strconv.ParseUint(raw, 10, 32)
			if err != nil || numeric == 0 {
				return "", fmt.Errorf("%s must identify a non-zero player", label)
			}
			return fmt.Sprintf("%08x", numeric) + strings.Repeat("0", 24), nil
		}
	}
	if len(raw) == 8 {
		raw += strings.Repeat("0", 24)
	}
	if len(raw) != 32 {
		return "", fmt.Errorf("%s must be a decimal player ID or GUID", label)
	}
	decoded, err := hex.DecodeString(raw)
	if err != nil || bytes.Equal(decoded, make([]byte, len(decoded))) {
		return "", fmt.Errorf("%s must be a non-zero decimal player ID or GUID", label)
	}
	return raw, nil
}

func validatePlayerUID(value, label string) error {
	_, err := canonicalPlayerUID(value, label)
	return err
}

func validateRenamePalOptions(options RenamePalOptions) error {
	if strings.TrimSpace(options.PlayerUID) == "" {
		return errors.New("player UID cannot be empty")
	}
	if err := validateSaveGUID(options.InstanceID, "Pal instance ID", false); err != nil {
		return err
	}
	if err := validatePalNickname(options.ExpectedNickname, "expected Pal nickname"); err != nil {
		return err
	}
	if err := validatePalNickname(options.Nickname, "Pal nickname"); err != nil {
		return err
	}
	if options.ExpectedLevel < 1 || options.ExpectedLevel > 80 {
		return errors.New("expected Pal level must be between 1 and 80")
	}
	if options.ExpectedExp < 0 {
		return errors.New("expected Pal EXP cannot be negative")
	}
	if options.Nickname == options.ExpectedNickname {
		return errors.New("Pal nickname is unchanged")
	}
	if !options.ConfirmServerStopped {
		return ErrSaveEditConfirmation
	}
	return nil
}

func validateEditPalLevelOptions(options EditPalLevelOptions) error {
	if err := validatePlayerUID(options.PlayerUID, "player UID"); err != nil {
		return err
	}
	if err := validateSaveGUID(options.InstanceID, "Pal instance ID", false); err != nil {
		return err
	}
	if err := validatePalNickname(options.ExpectedNickname, "expected Pal nickname"); err != nil {
		return err
	}
	if options.ExpectedLevel < 1 || options.ExpectedLevel > 80 {
		return errors.New("expected Pal level must be between 1 and 80")
	}
	if options.ExpectedExp < 0 {
		return errors.New("expected Pal EXP cannot be negative")
	}
	if options.ExpectedHP < 0 {
		return errors.New("expected Pal HP cannot be negative")
	}
	if options.ExpectedMaxHP < 0 {
		return errors.New("expected Pal MaxHP cannot be negative")
	}
	if options.Level < 1 || options.Level > 80 {
		return errors.New("Pal level must be between 1 and 80")
	}
	if options.Level == options.ExpectedLevel {
		return errors.New("Pal level is unchanged")
	}
	if !options.ConfirmServerStopped {
		return ErrSaveEditConfirmation
	}
	return nil
}

func validateRestorePalHealthOptions(options RestorePalHealthOptions) error {
	if err := validatePlayerUID(options.PlayerUID, "player UID"); err != nil {
		return err
	}
	if err := validateSaveGUID(options.InstanceID, "Pal instance ID", false); err != nil {
		return err
	}
	if err := validatePalNickname(options.ExpectedNickname, "expected Pal nickname"); err != nil {
		return err
	}
	if options.ExpectedLevel < 1 || options.ExpectedLevel > 80 {
		return errors.New("expected Pal level must be between 1 and 80")
	}
	if options.ExpectedExp < 0 {
		return errors.New("expected Pal EXP cannot be negative")
	}
	if options.ExpectedHP < 0 {
		return errors.New("expected Pal HP cannot be negative")
	}
	if options.ExpectedMaxHP <= 0 {
		return errors.New("expected Pal MaxHP must be positive")
	}
	if options.ExpectedHP >= options.ExpectedMaxHP {
		return errors.New("Pal HP must be below MaxHP before restoration")
	}
	if !options.ConfirmServerStopped {
		return ErrSaveEditConfirmation
	}
	return nil
}

func isRemoteSaveSource(path string) bool {
	path = strings.ToLower(strings.TrimSpace(path))
	return strings.HasPrefix(path, "http://") ||
		strings.HasPrefix(path, "https://") ||
		strings.HasPrefix(path, "docker://") ||
		strings.HasPrefix(path, "k8s://")
}

func localLevelSavePath(configuredPath string) (string, error) {
	configuredPath = strings.TrimSpace(configuredPath)
	if configuredPath == "" || configuredPath == "/path/to/your/Pal/Saved" {
		return "", errors.New("save path is not configured")
	}
	if isRemoteSaveSource(configuredPath) {
		return "", ErrUnsupportedSaveSource
	}

	info, err := os.Stat(configuredPath)
	if err != nil {
		return "", err
	}
	if !info.IsDir() {
		if !strings.EqualFold(filepath.Base(configuredPath), "Level.sav") {
			return "", errors.New("configured save file is not Level.sav")
		}
		return filepath.Abs(configuredPath)
	}
	levelPath, err := system.GetLevelSavFilePath(configuredPath)
	if err != nil {
		return "", err
	}
	return filepath.Abs(levelPath)
}

func probeGameServerStatus() (gameServerStatus, error) {
	if strings.TrimSpace(viper.GetString("rest.address")) == "" {
		return gameServerStatusExplicitOffline, nil
	}

	_, err := Metrics()
	if err == nil {
		return gameServerStatusOnline, nil
	}
	var restErr *RESTError
	if errors.As(err, &restErr) {
		return gameServerStatusOnline, nil
	}
	return gameServerStatusUnknown, fmt.Errorf(
		"%w: REST metrics probe failed: %v",
		ErrGameServerStatusUnknown,
		err,
	)
}

func ensureGameServerStopped() error {
	status, err := probeGameServerStatus()
	if err != nil {
		return err
	}
	switch status {
	case gameServerStatusExplicitOffline:
		return nil
	case gameServerStatusOnline:
		return ErrGameServerRunning
	default:
		return ErrGameServerStatusUnknown
	}
}

func internalSaveEditError(operation string, err error) error {
	if errors.Is(err, ErrSaveEditInternal) {
		return fmt.Errorf("%s: %w", operation, err)
	}
	return fmt.Errorf("%w: %s: %v", ErrSaveEditInternal, operation, err)
}

func saveTransactionError(operation string, err error) error {
	switch {
	case errors.Is(err, ErrSaveEditConfirmation),
		errors.Is(err, ErrGameServerRunning),
		errors.Is(err, ErrGameServerStatusUnknown),
		errors.Is(err, ErrSaveSourceChanged),
		errors.Is(err, ErrUnsupportedSaveSource):
		return fmt.Errorf("%s: %w", operation, err)
	default:
		return internalSaveEditError(operation, err)
	}
}

func parseDeliveryResult(output []byte) (ItemDelivery, error) {
	const marker = "DELIVERY_RESULT "
	for _, line := range strings.Split(string(output), "\n") {
		index := strings.Index(line, marker)
		if index < 0 {
			continue
		}
		var result ItemDelivery
		if err := json.Unmarshal([]byte(strings.TrimSpace(line[index+len(marker):])), &result); err != nil {
			return ItemDelivery{}, fmt.Errorf("decode save editor result: %w", err)
		}
		if result.Delivered <= 0 || result.Delivered != result.Requested {
			return ItemDelivery{}, errors.New("save editor reported an incomplete delivery")
		}
		return result, nil
	}
	return ItemDelivery{}, errors.New("save editor did not return a delivery result")
}

func parseInventoryMutationResult(output []byte) (InventoryMutation, error) {
	const marker = "INVENTORY_RESULT "
	for _, line := range strings.Split(string(output), "\n") {
		index := strings.Index(line, marker)
		if index < 0 {
			continue
		}
		var result InventoryMutation
		if err := json.Unmarshal([]byte(strings.TrimSpace(line[index+len(marker):])), &result); err != nil {
			return InventoryMutation{}, fmt.Errorf("decode save editor result: %w", err)
		}
		if strings.TrimSpace(result.ItemID) == "" || result.Before < 1 || result.After < 0 {
			return InventoryMutation{}, errors.New("save editor reported an invalid inventory mutation")
		}
		if result.Removed != (result.After == 0) || result.Before == result.After {
			return InventoryMutation{}, errors.New("save editor reported an inconsistent inventory mutation")
		}
		return result, nil
	}
	return InventoryMutation{}, errors.New("save editor did not return an inventory result")
}

func parsePlayerProfileResult(output []byte) (PlayerProfileMutation, error) {
	const marker = "PLAYER_PROFILE_RESULT "
	for _, line := range strings.Split(string(output), "\n") {
		index := strings.Index(line, marker)
		if index < 0 {
			continue
		}
		var result PlayerProfileMutation
		if err := json.Unmarshal([]byte(strings.TrimSpace(line[index+len(marker):])), &result); err != nil {
			return PlayerProfileMutation{}, fmt.Errorf("decode save editor result: %w", err)
		}
		if strings.TrimSpace(result.PlayerUID) == "" ||
			strings.TrimSpace(result.NicknameAfter) == "" ||
			result.LevelBefore < 1 || result.LevelAfter < 1 ||
			result.ExpBefore < 0 || result.ExpAfter < 0 ||
			result.CharacterRecords != 1 || result.GuildRecords < 0 {
			return PlayerProfileMutation{}, errors.New("save editor reported an invalid player profile mutation")
		}
		if result.NicknameBefore == result.NicknameAfter && result.LevelBefore == result.LevelAfter {
			return PlayerProfileMutation{}, errors.New("save editor reported an unchanged player profile")
		}
		return result, nil
	}
	return PlayerProfileMutation{}, errors.New("save editor did not return a player profile result")
}

func parsePlayerStatPointsResult(output []byte) (PlayerStatPointsMutation, error) {
	const marker = "PLAYER_STAT_POINTS_RESULT "
	for _, line := range strings.Split(string(output), "\n") {
		index := strings.Index(line, marker)
		if index < 0 {
			continue
		}
		var result PlayerStatPointsMutation
		if err := json.Unmarshal([]byte(strings.TrimSpace(line[index+len(marker):])), &result); err != nil {
			return PlayerStatPointsMutation{}, fmt.Errorf("decode save editor result: %w", err)
		}
		if strings.TrimSpace(result.PlayerUID) == "" ||
			result.Before < 0 || result.Before > 65535 ||
			result.After < 0 || result.After > 65535 ||
			result.CharacterRecords != 1 {
			return PlayerStatPointsMutation{}, errors.New("save editor reported an invalid player stat points mutation")
		}
		if result.Before == result.After {
			return PlayerStatPointsMutation{}, errors.New("save editor reported unchanged player stat points")
		}
		return result, nil
	}
	return PlayerStatPointsMutation{}, errors.New("save editor did not return a player stat points result")
}

func parsePlayerTechnologyPointsResult(output []byte) (PlayerTechnologyPointsMutation, error) {
	const marker = "PLAYER_TECHNOLOGY_POINTS_RESULT "
	for _, line := range strings.Split(string(output), "\n") {
		index := strings.Index(line, marker)
		if index < 0 {
			continue
		}
		var result PlayerTechnologyPointsMutation
		if err := json.Unmarshal([]byte(strings.TrimSpace(line[index+len(marker):])), &result); err != nil {
			return PlayerTechnologyPointsMutation{}, fmt.Errorf("decode save editor result: %w", err)
		}
		if strings.TrimSpace(result.PlayerUID) == "" ||
			result.TechnologyBefore < 0 || result.TechnologyBefore > 999999 ||
			result.TechnologyAfter < 0 || result.TechnologyAfter > 999999 ||
			result.AncientBefore < 0 || result.AncientBefore > 999999 ||
			result.AncientAfter < 0 || result.AncientAfter > 999999 {
			return PlayerTechnologyPointsMutation{}, errors.New("save editor reported invalid player technology points")
		}
		if result.TechnologyBefore == result.TechnologyAfter &&
			result.AncientBefore == result.AncientAfter {
			return PlayerTechnologyPointsMutation{}, errors.New("save editor reported unchanged player technology points")
		}
		for _, field := range result.CreatedFields {
			if field != "TechnologyPoint" && field != "bossTechnologyPoint" {
				return PlayerTechnologyPointsMutation{}, fmt.Errorf("save editor reported unexpected created field %q", field)
			}
		}
		return result, nil
	}
	return PlayerTechnologyPointsMutation{}, errors.New("save editor did not return a player technology points result")
}

func parsePlayerMapProgressResult(output []byte) (PlayerMapProgressMutation, error) {
	const marker = "PLAYER_MAP_PROGRESS_RESULT "
	for _, line := range strings.Split(string(output), "\n") {
		index := strings.Index(line, marker)
		if index < 0 {
			continue
		}
		var result PlayerMapProgressMutation
		if err := json.Unmarshal([]byte(strings.TrimSpace(line[index+len(marker):])), &result); err != nil {
			return PlayerMapProgressMutation{}, fmt.Errorf("decode save editor result: %w", err)
		}
		if err := validatePlayerUID(result.PlayerUID, "player UID"); err != nil {
			return PlayerMapProgressMutation{}, fmt.Errorf("save editor reported invalid player map progress: %w", err)
		}
		if result.GameVersion != playerMapGameVersion ||
			result.FastTravelTotal != playerMapFastTravelTotal ||
			result.AreasTotal != playerMapAreasTotal ||
			result.WorldMapsTotal != playerMapWorldMapsTotal {
			return PlayerMapProgressMutation{}, errors.New("save editor reported unsupported player map metadata")
		}
		if result.FastTravelBefore < 0 || result.FastTravelBefore > result.FastTravelTotal ||
			result.FastTravelAfter != result.FastTravelTotal ||
			result.FastTravelAfter < result.FastTravelBefore ||
			result.AreasBefore < 0 || result.AreasBefore > result.AreasTotal ||
			result.AreasAfter != result.AreasTotal ||
			result.AreasAfter < result.AreasBefore ||
			result.WorldMapsBefore < 0 || result.WorldMapsBefore > result.WorldMapsTotal ||
			result.WorldMapsAfter != result.WorldMapsTotal ||
			result.WorldMapsAfter < result.WorldMapsBefore {
			return PlayerMapProgressMutation{}, errors.New("save editor reported inconsistent player map progress")
		}
		if result.FastTravelBefore == result.FastTravelTotal &&
			result.AreasBefore == result.AreasTotal &&
			result.WorldMapsBefore == result.WorldMapsTotal {
			return PlayerMapProgressMutation{}, errors.New("save editor reported unchanged player map progress")
		}
		beforeDigest, err := canonicalSHA256Digest(result.ProgressDigestBefore, "previous map progress digest")
		if err != nil {
			return PlayerMapProgressMutation{}, fmt.Errorf("save editor reported invalid player map progress: %w", err)
		}
		afterDigest, err := canonicalSHA256Digest(result.ProgressDigestAfter, "new map progress digest")
		if err != nil {
			return PlayerMapProgressMutation{}, fmt.Errorf("save editor reported invalid player map progress: %w", err)
		}
		if beforeDigest == afterDigest {
			return PlayerMapProgressMutation{}, errors.New("save editor reported unchanged player map progress")
		}
		allowedFields := map[string]bool{
			"FastTravelPointUnlockFlag": false,
			"FindAreaFlagMap":           false,
			"UnlockedWorldMapFlags":     false,
		}
		for _, field := range result.CreatedFields {
			seen, ok := allowedFields[field]
			if !ok {
				return PlayerMapProgressMutation{}, fmt.Errorf("save editor reported unexpected created field %q", field)
			}
			if seen {
				return PlayerMapProgressMutation{}, fmt.Errorf("save editor reported duplicate created field %q", field)
			}
			allowedFields[field] = true
		}
		result.ProgressDigestBefore = beforeDigest
		result.ProgressDigestAfter = afterDigest
		return result, nil
	}
	return PlayerMapProgressMutation{}, errors.New("save editor did not return a player map progress result")
}

func validatePlayerMapProgressResultMatchesOptions(result PlayerMapProgressMutation, options UnlockPlayerMapOptions) error {
	resultPlayerUID, err := canonicalPlayerUID(result.PlayerUID, "result player UID")
	if err != nil {
		return err
	}
	expectedPlayerUID, err := canonicalPlayerUID(options.PlayerUID, "expected player UID")
	if err != nil {
		return err
	}
	if resultPlayerUID != expectedPlayerUID {
		return errors.New("save editor reported map progress for a different player")
	}
	resultDigest, err := canonicalSHA256Digest(result.ProgressDigestBefore, "result previous map progress digest")
	if err != nil {
		return err
	}
	expectedDigest, err := canonicalSHA256Digest(options.ExpectedProgressDigest, "expected map progress digest")
	if err != nil {
		return err
	}
	if resultDigest != expectedDigest {
		return errors.New("save editor reported unexpected previous map progress")
	}
	return nil
}

func parsePalNicknameResult(output []byte) (PalNicknameMutation, error) {
	const marker = "PAL_NICKNAME_RESULT "
	for _, line := range strings.Split(string(output), "\n") {
		index := strings.Index(line, marker)
		if index < 0 {
			continue
		}
		var result PalNicknameMutation
		if err := json.Unmarshal([]byte(strings.TrimSpace(line[index+len(marker):])), &result); err != nil {
			return PalNicknameMutation{}, fmt.Errorf("decode save editor result: %w", err)
		}
		if strings.TrimSpace(result.PlayerUID) == "" || strings.TrimSpace(result.PalType) == "" {
			return PalNicknameMutation{}, errors.New("save editor reported an invalid Pal nickname mutation")
		}
		if err := validateSaveGUID(result.InstanceID, "Pal instance ID", false); err != nil {
			return PalNicknameMutation{}, fmt.Errorf("save editor reported an invalid Pal nickname mutation: %w", err)
		}
		if err := validatePalNickname(result.NicknameBefore, "previous Pal nickname"); err != nil {
			return PalNicknameMutation{}, fmt.Errorf("save editor reported an invalid Pal nickname mutation: %w", err)
		}
		if err := validatePalNickname(result.NicknameAfter, "new Pal nickname"); err != nil {
			return PalNicknameMutation{}, fmt.Errorf("save editor reported an invalid Pal nickname mutation: %w", err)
		}
		if result.Level < 1 || result.Level > 80 || result.Exp < 0 ||
			result.NicknameBefore == result.NicknameAfter {
			return PalNicknameMutation{}, errors.New("save editor reported an inconsistent Pal nickname mutation")
		}
		return result, nil
	}
	return PalNicknameMutation{}, errors.New("save editor did not return a Pal nickname result")
}

func parsePalLevelResult(output []byte) (PalLevelMutation, error) {
	const marker = "PAL_LEVEL_RESULT "
	for _, line := range strings.Split(string(output), "\n") {
		index := strings.Index(line, marker)
		if index < 0 {
			continue
		}
		var result PalLevelMutation
		if err := json.Unmarshal([]byte(strings.TrimSpace(line[index+len(marker):])), &result); err != nil {
			return PalLevelMutation{}, fmt.Errorf("decode save editor result: %w", err)
		}
		if strings.TrimSpace(result.PalType) == "" {
			return PalLevelMutation{}, errors.New("save editor reported an invalid Pal level mutation")
		}
		if err := validatePlayerUID(result.PlayerUID, "player UID"); err != nil {
			return PalLevelMutation{}, fmt.Errorf("save editor reported an invalid Pal level mutation: %w", err)
		}
		if err := validateSaveGUID(result.InstanceID, "Pal instance ID", false); err != nil {
			return PalLevelMutation{}, fmt.Errorf("save editor reported an invalid Pal level mutation: %w", err)
		}
		if err := validatePalNickname(result.Nickname, "Pal nickname"); err != nil {
			return PalLevelMutation{}, fmt.Errorf("save editor reported an invalid Pal level mutation: %w", err)
		}
		if result.LevelBefore < 1 || result.LevelBefore > 80 ||
			result.LevelAfter < 1 || result.LevelAfter > 80 ||
			result.LevelBefore == result.LevelAfter ||
			result.ExpBefore < 0 || result.ExpAfter < 0 ||
			result.HPBefore < 0 || result.HPAfter <= 0 ||
			result.MaxHPBefore < 0 || result.MaxHPAfter <= 0 ||
			result.HPAfter != result.MaxHPAfter {
			return PalLevelMutation{}, errors.New("save editor reported an inconsistent Pal level mutation")
		}
		if result.HealthField != "Hp" && result.HealthField != "HP" {
			return PalLevelMutation{}, errors.New("save editor reported an invalid Pal health field")
		}
		if result.MaxHPCreated && result.MaxHPBefore != 0 {
			return PalLevelMutation{}, errors.New("save editor reported an inconsistent MaxHP creation")
		}
		return result, nil
	}
	return PalLevelMutation{}, errors.New("save editor did not return a Pal level result")
}

func parsePalHealthResult(output []byte) (PalHealthMutation, error) {
	const marker = "PAL_HEALTH_RESULT "
	for _, line := range strings.Split(string(output), "\n") {
		index := strings.Index(line, marker)
		if index < 0 {
			continue
		}
		var result PalHealthMutation
		if err := json.Unmarshal([]byte(strings.TrimSpace(line[index+len(marker):])), &result); err != nil {
			return PalHealthMutation{}, fmt.Errorf("decode save editor result: %w", err)
		}
		if strings.TrimSpace(result.PalType) == "" {
			return PalHealthMutation{}, errors.New("save editor reported an invalid Pal health mutation")
		}
		if err := validatePlayerUID(result.PlayerUID, "player UID"); err != nil {
			return PalHealthMutation{}, fmt.Errorf("save editor reported an invalid Pal health mutation: %w", err)
		}
		if err := validateSaveGUID(result.InstanceID, "Pal instance ID", false); err != nil {
			return PalHealthMutation{}, fmt.Errorf("save editor reported an invalid Pal health mutation: %w", err)
		}
		if err := validatePalNickname(result.Nickname, "Pal nickname"); err != nil {
			return PalHealthMutation{}, fmt.Errorf("save editor reported an invalid Pal health mutation: %w", err)
		}
		if result.Level < 1 || result.Level > 80 || result.Exp < 0 ||
			result.HPBefore < 0 || result.MaxHP <= 0 ||
			result.HPAfter != result.MaxHP || result.HPAfter <= result.HPBefore {
			return PalHealthMutation{}, errors.New("save editor reported an inconsistent Pal health mutation")
		}
		if result.HealthField != "Hp" && result.HealthField != "HP" {
			return PalHealthMutation{}, errors.New("save editor reported an invalid Pal health field")
		}
		return result, nil
	}
	return PalHealthMutation{}, errors.New("save editor did not return a Pal health result")
}

func validatePalHealthResultMatchesOptions(result PalHealthMutation, options RestorePalHealthOptions) error {
	resultPlayerUID, err := canonicalPlayerUID(result.PlayerUID, "result player UID")
	if err != nil {
		return err
	}
	expectedPlayerUID, err := canonicalPlayerUID(options.PlayerUID, "expected player UID")
	if err != nil {
		return err
	}
	if resultPlayerUID != expectedPlayerUID {
		return errors.New("save editor reported a Pal health result for a different player")
	}

	resultInstanceID, err := canonicalSaveGUID(result.InstanceID, "result Pal instance ID", false)
	if err != nil {
		return err
	}
	expectedInstanceID, err := canonicalSaveGUID(options.InstanceID, "expected Pal instance ID", false)
	if err != nil {
		return err
	}
	if resultInstanceID != expectedInstanceID {
		return errors.New("save editor reported a Pal health result for a different Pal instance")
	}
	if result.Nickname != options.ExpectedNickname {
		return errors.New("save editor reported an unexpected Pal nickname")
	}
	if result.Level != options.ExpectedLevel {
		return errors.New("save editor reported an unexpected Pal level")
	}
	if result.Exp != options.ExpectedExp {
		return errors.New("save editor reported unexpected Pal EXP")
	}
	if result.HPBefore != options.ExpectedHP {
		return errors.New("save editor reported unexpected Pal HP")
	}
	if result.MaxHP != options.ExpectedMaxHP {
		return errors.New("save editor reported unexpected Pal MaxHP")
	}
	return nil
}

func editorError(output []byte, runErr error) error {
	lines := strings.Split(strings.TrimSpace(string(output)), "\n")
	for index := len(lines) - 1; index >= 0; index-- {
		line := strings.TrimSpace(lines[index])
		if line == "" {
			continue
		}
		if len(line) > 1000 {
			line = line[:1000]
		}
		return fmt.Errorf("save editor failed: %s: %w", line, runErr)
	}
	return fmt.Errorf("save editor failed: %w", runErr)
}

func parseSaveEditorMachineError(output []byte) (saveEditorMachineError, bool) {
	const marker = "SAVE_EDIT_ERROR "
	for _, line := range strings.Split(string(output), "\n") {
		index := strings.Index(line, marker)
		if index < 0 {
			continue
		}
		var result saveEditorMachineError
		if err := json.Unmarshal([]byte(strings.TrimSpace(line[index+len(marker):])), &result); err == nil && result.Code != "" {
			return result, true
		}
	}
	return saveEditorMachineError{}, false
}

func runSaveEditor(args []string) ([]byte, error) {
	savCli, err := getSavCli()
	if err != nil {
		return nil, internalSaveEditError("get save editor", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), saveEditorTimeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, savCli, args...)
	var output bytes.Buffer
	cmd.Stdout = &output
	cmd.Stderr = &output
	if err := cmd.Run(); err != nil {
		if errors.Is(ctx.Err(), context.DeadlineExceeded) {
			return nil, internalSaveEditError(
				"run save editor",
				fmt.Errorf("timed out after %s", saveEditorTimeout),
			)
		}
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			if machineError, ok := parseSaveEditorMachineError(output.Bytes()); ok {
				switch machineError.Code {
				case "stale_state":
					return nil, fmt.Errorf("%w: %s", ErrSaveSourceChanged, machineError.Message)
				case "validation":
					return nil, errors.New(machineError.Message)
				default:
					return nil, internalSaveEditError("save editor returned an unknown error code", errors.New(machineError.Code))
				}
			}
			return nil, internalSaveEditError("save editor exited without a machine-readable error", editorError(output.Bytes(), err))
		}
		return nil, internalSaveEditError("start save editor", err)
	}
	return output.Bytes(), nil
}

func runGiveItemEditor(levelPath, outputPath string, options GiveItemOptions) (ItemDelivery, error) {
	container := options.Container
	if container == "" {
		container = "auto"
	}
	args := []string{
		"--mode", "give-item",
		"--file", levelPath,
		"--output", outputPath,
		"--player-uid", options.PlayerUID,
		"--item-id", options.ItemID,
		"--quantity", fmt.Sprintf("%d", options.Quantity),
		"--container", container,
	}
	output, err := runSaveEditor(args)
	if err != nil {
		return ItemDelivery{}, err
	}
	result, err := parseDeliveryResult(output)
	if err != nil {
		return ItemDelivery{}, internalSaveEditError("parse item delivery result", err)
	}
	return result, nil
}

func runSetItemQuantityEditor(levelPath, outputPath string, options SetItemQuantityOptions) (InventoryMutation, error) {
	args := []string{
		"--mode", "set-item-quantity",
		"--file", levelPath,
		"--output", outputPath,
		"--player-uid", options.PlayerUID,
		"--container", options.Container,
		"--slot-index", fmt.Sprintf("%d", options.SlotIndex),
		"--quantity", fmt.Sprintf("%d", options.Quantity),
		"--expected-item-id", options.ItemID,
		"--expected-dynamic-id", options.ExpectedDynamicID,
		"--expected-quantity", fmt.Sprintf("%d", options.ExpectedQuantity),
	}
	output, err := runSaveEditor(args)
	if err != nil {
		return InventoryMutation{}, err
	}
	result, err := parseInventoryMutationResult(output)
	if err != nil {
		return InventoryMutation{}, internalSaveEditError("parse inventory mutation result", err)
	}
	return result, nil
}

func runEditPlayerProfileEditor(levelPath, outputPath string, options EditPlayerProfileOptions) (PlayerProfileMutation, error) {
	args := []string{
		"--mode", "edit-player-profile",
		"--file", levelPath,
		"--output", outputPath,
		"--player-uid", options.PlayerUID,
		"--expected-nickname", options.ExpectedNickname,
		"--expected-level", fmt.Sprintf("%d", options.ExpectedLevel),
		"--nickname", strings.TrimSpace(options.Nickname),
		"--level", fmt.Sprintf("%d", options.Level),
	}
	output, err := runSaveEditor(args)
	if err != nil {
		return PlayerProfileMutation{}, err
	}
	result, err := parsePlayerProfileResult(output)
	if err != nil {
		return PlayerProfileMutation{}, internalSaveEditError("parse player profile result", err)
	}
	return result, nil
}

func runEditPlayerStatPointsEditor(levelPath, outputPath string, options EditPlayerStatPointsOptions) (PlayerStatPointsMutation, error) {
	args := []string{
		"--mode", "edit-player-stat-points",
		"--file", levelPath,
		"--output", outputPath,
		"--player-uid", options.PlayerUID,
		"--expected-unused-stat-points", fmt.Sprintf("%d", options.ExpectedUnusedStatPoints),
		"--unused-stat-points", fmt.Sprintf("%d", options.UnusedStatPoints),
	}
	output, err := runSaveEditor(args)
	if err != nil {
		return PlayerStatPointsMutation{}, err
	}
	result, err := parsePlayerStatPointsResult(output)
	if err != nil {
		return PlayerStatPointsMutation{}, internalSaveEditError("parse player stat points result", err)
	}
	return result, nil
}

func runEditPlayerTechnologyPointsEditor(playerPath, outputPath string, options EditPlayerTechnologyPointsOptions) (PlayerTechnologyPointsMutation, error) {
	args := []string{
		"--mode", "edit-player-technology-points",
		"--file", playerPath,
		"--output", outputPath,
		"--player-uid", options.PlayerUID,
		"--expected-technology-points", fmt.Sprintf("%d", options.ExpectedTechnologyPoints),
		"--expected-ancient-technology-points", fmt.Sprintf("%d", options.ExpectedAncientTechnologyPoints),
		"--technology-points", fmt.Sprintf("%d", options.TechnologyPoints),
		"--ancient-technology-points", fmt.Sprintf("%d", options.AncientTechnologyPoints),
	}
	output, err := runSaveEditor(args)
	if err != nil {
		return PlayerTechnologyPointsMutation{}, err
	}
	result, err := parsePlayerTechnologyPointsResult(output)
	if err != nil {
		return PlayerTechnologyPointsMutation{}, internalSaveEditError("parse player technology points result", err)
	}
	return result, nil
}

func runUnlockPlayerMapEditor(playerPath, outputPath string, options UnlockPlayerMapOptions) (PlayerMapProgressMutation, error) {
	args := []string{
		"--mode", "unlock-player-map",
		"--file", playerPath,
		"--output", outputPath,
		"--player-uid", options.PlayerUID,
		"--expected-map-progress-digest", options.ExpectedProgressDigest,
	}
	output, err := runSaveEditor(args)
	if err != nil {
		return PlayerMapProgressMutation{}, err
	}
	result, err := parsePlayerMapProgressResult(output)
	if err != nil {
		return PlayerMapProgressMutation{}, internalSaveEditError("parse player map progress result", err)
	}
	if err := validatePlayerMapProgressResultMatchesOptions(result, options); err != nil {
		return PlayerMapProgressMutation{}, internalSaveEditError("validate player map progress result", err)
	}
	return result, nil
}

func runRenamePalEditor(levelPath, outputPath string, options RenamePalOptions) (PalNicknameMutation, error) {
	args := []string{
		"--mode", "edit-pal-nickname",
		"--file", levelPath,
		"--output", outputPath,
		"--player-uid", options.PlayerUID,
		"--instance-id", options.InstanceID,
		"--expected-pal-nickname", options.ExpectedNickname,
		"--expected-pal-level", fmt.Sprintf("%d", options.ExpectedLevel),
		"--expected-pal-exp", fmt.Sprintf("%d", options.ExpectedExp),
		"--pal-nickname", options.Nickname,
	}
	output, err := runSaveEditor(args)
	if err != nil {
		return PalNicknameMutation{}, err
	}
	result, err := parsePalNicknameResult(output)
	if err != nil {
		return PalNicknameMutation{}, internalSaveEditError("parse Pal nickname result", err)
	}
	return result, nil
}

func runEditPalLevelEditor(levelPath, outputPath string, options EditPalLevelOptions) (PalLevelMutation, error) {
	args := []string{
		"--mode", "edit-pal-level",
		"--file", levelPath,
		"--output", outputPath,
		"--player-uid", options.PlayerUID,
		"--instance-id", options.InstanceID,
		"--expected-pal-nickname", options.ExpectedNickname,
		"--expected-pal-level", strconv.Itoa(options.ExpectedLevel),
		"--expected-pal-exp", strconv.FormatInt(options.ExpectedExp, 10),
		"--expected-pal-hp", strconv.FormatInt(options.ExpectedHP, 10),
		"--expected-pal-max-hp", strconv.FormatInt(options.ExpectedMaxHP, 10),
		"--pal-level", strconv.Itoa(options.Level),
	}
	output, err := runSaveEditor(args)
	if err != nil {
		return PalLevelMutation{}, err
	}
	result, err := parsePalLevelResult(output)
	if err != nil {
		return PalLevelMutation{}, internalSaveEditError("parse Pal level result", err)
	}
	return result, nil
}

func runRestorePalHealthEditor(levelPath, outputPath string, options RestorePalHealthOptions) (PalHealthMutation, error) {
	args := []string{
		"--mode", "restore-pal-health",
		"--file", levelPath,
		"--output", outputPath,
		"--player-uid", options.PlayerUID,
		"--instance-id", options.InstanceID,
		"--expected-pal-nickname", options.ExpectedNickname,
		"--expected-pal-level", strconv.Itoa(options.ExpectedLevel),
		"--expected-pal-exp", strconv.FormatInt(options.ExpectedExp, 10),
		"--expected-pal-hp", strconv.FormatInt(options.ExpectedHP, 10),
		"--expected-pal-max-hp", strconv.FormatInt(options.ExpectedMaxHP, 10),
	}
	output, err := runSaveEditor(args)
	if err != nil {
		return PalHealthMutation{}, err
	}
	result, err := parsePalHealthResult(output)
	if err != nil {
		return PalHealthMutation{}, internalSaveEditError("parse Pal health result", err)
	}
	if err := validatePalHealthResultMatchesOptions(result, options); err != nil {
		return PalHealthMutation{}, internalSaveEditError("validate Pal health result", err)
	}
	return result, nil
}

func fingerprintSaveFile(path string) (saveFileFingerprint, error) {
	file, err := os.Open(path)
	if err != nil {
		return saveFileFingerprint{}, err
	}
	defer file.Close()

	before, err := file.Stat()
	if err != nil {
		return saveFileFingerprint{}, err
	}
	if !before.Mode().IsRegular() {
		return saveFileFingerprint{}, fmt.Errorf("save source %q is not a regular file", path)
	}

	hasher := sha256.New()
	if _, err := io.Copy(hasher, file); err != nil {
		return saveFileFingerprint{}, err
	}
	after, err := file.Stat()
	if err != nil {
		return saveFileFingerprint{}, err
	}
	pathInfo, err := os.Stat(path)
	if err != nil {
		return saveFileFingerprint{}, fmt.Errorf("%w while reading %q: %v", ErrSaveSourceChanged, path, err)
	}
	if !os.SameFile(after, pathInfo) ||
		before.Size() != after.Size() ||
		!before.ModTime().Equal(after.ModTime()) ||
		after.Size() != pathInfo.Size() ||
		!after.ModTime().Equal(pathInfo.ModTime()) {
		return saveFileFingerprint{}, fmt.Errorf(
			"%w while reading %q",
			ErrSaveSourceChanged,
			path,
		)
	}

	var digest [sha256.Size]byte
	copy(digest[:], hasher.Sum(nil))
	return saveFileFingerprint{
		Size:    after.Size(),
		ModTime: after.ModTime(),
		SHA256:  digest,
	}, nil
}

func (fingerprint saveFileFingerprint) sameContent(other saveFileFingerprint) bool {
	return fingerprint.Size == other.Size && fingerprint.SHA256 == other.SHA256
}

func (fingerprint saveFileFingerprint) sameVersion(other saveFileFingerprint) bool {
	return fingerprint.sameContent(other) && fingerprint.ModTime.Equal(other.ModTime)
}

func verifySaveFileVersion(path string, expected saveFileFingerprint) error {
	actual, err := fingerprintSaveFile(path)
	if err != nil {
		if errors.Is(err, ErrSaveSourceChanged) {
			return err
		}
		return fmt.Errorf("%w: inspect %q: %v", ErrSaveSourceChanged, path, err)
	}
	if !expected.sameVersion(actual) {
		return fmt.Errorf("%w: %q no longer matches the edit source", ErrSaveSourceChanged, path)
	}
	return nil
}

func verifyStagedSaveContent(path string, expected saveFileFingerprint) error {
	actual, err := fingerprintSaveFile(path)
	if err != nil {
		return fmt.Errorf("fingerprint staged Level.sav: %w", err)
	}
	if !expected.sameContent(actual) {
		return fmt.Errorf("%w: staged Level.sav does not match the target snapshot", ErrSaveSourceChanged)
	}
	return nil
}

func resolvePlayerSavePath(levelPath, playerUID string) (string, error) {
	rawUID := strings.ReplaceAll(strings.TrimSpace(playerUID), "-", "")
	if rawUID == "" {
		return "", errors.New("player UID cannot be empty")
	}
	var prefix string
	var exactName string
	if numericUID, err := strconv.ParseUint(rawUID, 10, 32); err == nil && len(rawUID) <= 10 {
		prefix = strings.ToUpper(fmt.Sprintf("%08x", numericUID))
		exactName = prefix + strings.Repeat("0", 24) + ".SAV"
	} else if len(rawUID) == 8 {
		if _, err := strconv.ParseUint(rawUID, 16, 32); err != nil {
			return "", errors.New("player UID must be a decimal ID, 8 hexadecimal digits, or a GUID")
		}
		prefix = strings.ToUpper(rawUID)
	} else if len(rawUID) == 32 {
		if _, err := hex.DecodeString(rawUID); err != nil {
			return "", errors.New("player UID must be a decimal ID, 8 hexadecimal digits, or a GUID")
		}
		exactName = strings.ToUpper(rawUID) + ".SAV"
		prefix = strings.ToUpper(rawUID[:8])
	} else {
		return "", errors.New("player UID must be a decimal ID, 8 hexadecimal digits, or a GUID")
	}

	playersDir := filepath.Join(filepath.Dir(levelPath), "Players")
	entries, err := os.ReadDir(playersDir)
	if err != nil {
		return "", err
	}
	var exactMatches []string
	var prefixMatches []string
	for _, entry := range entries {
		if entry.IsDir() || !strings.EqualFold(filepath.Ext(entry.Name()), ".sav") {
			continue
		}
		upperName := strings.ToUpper(entry.Name())
		path := filepath.Join(playersDir, entry.Name())
		if exactName != "" && upperName == exactName {
			exactMatches = append(exactMatches, path)
		}
		if strings.HasPrefix(strings.ToUpper(strings.TrimSuffix(entry.Name(), filepath.Ext(entry.Name()))), prefix) {
			prefixMatches = append(prefixMatches, path)
		}
	}
	if len(exactMatches) == 1 {
		return filepath.Abs(exactMatches[0])
	}
	if len(prefixMatches) == 1 {
		return filepath.Abs(prefixMatches[0])
	}
	if len(prefixMatches) == 0 {
		return "", fmt.Errorf("player save was not found for UID %s", playerUID)
	}
	return "", fmt.Errorf("player UID %s matches multiple save files", playerUID)
}

func stageAndReplaceSave(editedPath, targetPath string) error {
	return stageAndReplaceSaveGuarded(editedPath, targetPath, nil, nil)
}

func stageAndReplaceSaveGuarded(
	editedPath string,
	targetPath string,
	expected *saveFileFingerprint,
	preReplaceCheck func() error,
) error {
	if _, err := os.Stat(targetPath); err != nil {
		return err
	}

	staged, err := os.CreateTemp(filepath.Dir(targetPath), ".pst-Level-*.tmp")
	if err != nil {
		return err
	}
	stagedPath := staged.Name()
	defer os.Remove(stagedPath)

	edited, err := os.Open(editedPath)
	if err != nil {
		staged.Close()
		return err
	}
	_, copyErr := io.Copy(staged, edited)
	closeEditedErr := edited.Close()
	if copyErr == nil {
		copyErr = closeEditedErr
	}
	if copyErr == nil {
		copyErr = system.PrepareReplacementFile(stagedPath, targetPath)
	}
	if copyErr == nil {
		copyErr = staged.Sync()
	}
	if closeErr := staged.Close(); copyErr == nil {
		copyErr = closeErr
	}
	if copyErr != nil {
		return copyErr
	}
	if preReplaceCheck != nil {
		if err := preReplaceCheck(); err != nil {
			return err
		}
	}
	if expected != nil {
		if err := verifySaveFileVersion(targetPath, *expected); err != nil {
			return err
		}
	}
	return system.ReplaceFileAtomic(stagedPath, targetPath)
}

func runLocalSaveTransaction(
	db *bbolt.DB,
	runner func(levelPath, outputPath string) error,
) (database.Backup, error) {
	return runLocalSaveTransactionWithBackup(db, runner, BackupAndRecord)
}

func runLocalSaveTransactionWithBackup(
	db *bbolt.DB,
	runner func(levelPath, outputPath string) error,
	backupAndRecord func(db *bbolt.DB) (database.Backup, error),
) (database.Backup, error) {
	saveEditorMu.Lock()
	defer saveEditorMu.Unlock()
	if backupAndRecord == nil {
		return database.Backup{}, internalSaveEditError(
			"configure save transaction",
			errors.New("backup function is nil"),
		)
	}

	if err := ensureGameServerStopped(); err != nil {
		return database.Backup{}, err
	}
	configuredPath := viper.GetString("save.path")
	targetPath, err := localLevelSavePath(configuredPath)
	if err != nil {
		return database.Backup{}, saveTransactionError("resolve Level.sav", err)
	}

	sourceFingerprint, err := fingerprintSaveFile(targetPath)
	if err != nil {
		return database.Backup{}, saveTransactionError("fingerprint Level.sav", err)
	}

	temporaryLevel, err := source.CopyFromLocal(configuredPath, "edit")
	if err != nil {
		return database.Backup{}, internalSaveEditError("stage save edit", err)
	}
	defer os.RemoveAll(filepath.Dir(temporaryLevel))
	if err := verifyStagedSaveContent(temporaryLevel, sourceFingerprint); err != nil {
		return database.Backup{}, saveTransactionError("verify staged Level.sav", err)
	}
	if err := verifySaveFileVersion(targetPath, sourceFingerprint); err != nil {
		return database.Backup{}, saveTransactionError("verify Level.sav before edit", err)
	}

	editedPath := filepath.Join(filepath.Dir(temporaryLevel), "Level.pst-edited.sav")
	if err := runner(temporaryLevel, editedPath); err != nil {
		return database.Backup{}, err
	}
	if err := verifySaveFileVersion(targetPath, sourceFingerprint); err != nil {
		return database.Backup{}, saveTransactionError("verify Level.sav after edit", err)
	}
	if err := ensureGameServerStopped(); err != nil {
		return database.Backup{}, err
	}

	backup, err := backupAndRecord(db)
	if err != nil {
		return database.Backup{}, internalSaveEditError("create pre-edit backup", err)
	}
	if err := verifySaveFileVersion(targetPath, sourceFingerprint); err != nil {
		return database.Backup{}, saveTransactionError("verify Level.sav after backup", err)
	}
	if err := stageAndReplaceSaveGuarded(
		editedPath,
		targetPath,
		&sourceFingerprint,
		ensureGameServerStopped,
	); err != nil {
		return database.Backup{}, saveTransactionError("replace Level.sav", err)
	}
	return backup, nil
}

func runLocalPlayerSaveTransaction(
	db *bbolt.DB,
	playerUID string,
	runner func(playerPath, outputPath string) error,
) (database.Backup, error) {
	return runLocalPlayerSaveTransactionWithBackup(db, playerUID, runner, BackupAndRecord)
}

func runLocalPlayerSaveTransactionWithBackup(
	db *bbolt.DB,
	playerUID string,
	runner func(playerPath, outputPath string) error,
	backupAndRecord func(db *bbolt.DB) (database.Backup, error),
) (database.Backup, error) {
	saveEditorMu.Lock()
	defer saveEditorMu.Unlock()
	if backupAndRecord == nil {
		return database.Backup{}, internalSaveEditError(
			"configure player save transaction",
			errors.New("backup function is nil"),
		)
	}
	if err := ensureGameServerStopped(); err != nil {
		return database.Backup{}, err
	}
	configuredPath := viper.GetString("save.path")
	levelPath, err := localLevelSavePath(configuredPath)
	if err != nil {
		return database.Backup{}, saveTransactionError("resolve Level.sav", err)
	}
	playerPath, err := resolvePlayerSavePath(levelPath, playerUID)
	if err != nil {
		return database.Backup{}, saveTransactionError("resolve player save", err)
	}
	levelFingerprint, err := fingerprintSaveFile(levelPath)
	if err != nil {
		return database.Backup{}, saveTransactionError("fingerprint Level.sav", err)
	}
	playerFingerprint, err := fingerprintSaveFile(playerPath)
	if err != nil {
		return database.Backup{}, saveTransactionError("fingerprint player save", err)
	}

	temporaryLevel, err := source.CopyFromLocal(configuredPath, "edit-player")
	if err != nil {
		return database.Backup{}, internalSaveEditError("stage player save edit", err)
	}
	defer os.RemoveAll(filepath.Dir(temporaryLevel))
	if err := verifyStagedSaveContent(temporaryLevel, levelFingerprint); err != nil {
		return database.Backup{}, saveTransactionError("verify staged Level.sav", err)
	}
	temporaryPlayer, err := resolvePlayerSavePath(temporaryLevel, playerUID)
	if err != nil {
		return database.Backup{}, internalSaveEditError("resolve staged player save", err)
	}
	if err := verifyStagedSaveContent(temporaryPlayer, playerFingerprint); err != nil {
		return database.Backup{}, saveTransactionError("verify staged player save", err)
	}
	if err := verifySaveFileVersion(levelPath, levelFingerprint); err != nil {
		return database.Backup{}, saveTransactionError("verify Level.sav before edit", err)
	}
	if err := verifySaveFileVersion(playerPath, playerFingerprint); err != nil {
		return database.Backup{}, saveTransactionError("verify player save before edit", err)
	}

	editedPath := filepath.Join(filepath.Dir(temporaryLevel), "Player.pst-edited.sav")
	if err := runner(temporaryPlayer, editedPath); err != nil {
		return database.Backup{}, err
	}
	if err := verifySaveFileVersion(levelPath, levelFingerprint); err != nil {
		return database.Backup{}, saveTransactionError("verify Level.sav after edit", err)
	}
	if err := verifySaveFileVersion(playerPath, playerFingerprint); err != nil {
		return database.Backup{}, saveTransactionError("verify player save after edit", err)
	}
	if err := ensureGameServerStopped(); err != nil {
		return database.Backup{}, err
	}

	backup, err := backupAndRecord(db)
	if err != nil {
		return database.Backup{}, internalSaveEditError("create pre-edit backup", err)
	}
	if err := verifySaveFileVersion(levelPath, levelFingerprint); err != nil {
		return database.Backup{}, saveTransactionError("verify Level.sav after backup", err)
	}
	if err := verifySaveFileVersion(playerPath, playerFingerprint); err != nil {
		return database.Backup{}, saveTransactionError("verify player save after backup", err)
	}
	if err := stageAndReplaceSaveGuarded(
		editedPath,
		playerPath,
		&playerFingerprint,
		func() error {
			if err := ensureGameServerStopped(); err != nil {
				return err
			}
			return verifySaveFileVersion(levelPath, levelFingerprint)
		},
	); err != nil {
		return database.Backup{}, saveTransactionError("replace player save", err)
	}
	return backup, nil
}

func GivePlayerItem(db *bbolt.DB, options GiveItemOptions) (GiveItemResult, error) {
	if err := validateGiveItemOptions(options); err != nil {
		return GiveItemResult{}, err
	}

	var delivery ItemDelivery
	backup, err := runLocalSaveTransaction(
		db,
		func(levelPath, outputPath string) error {
			var runErr error
			delivery, runErr = runGiveItemEditor(levelPath, outputPath, options)
			return runErr
		},
	)
	if err != nil {
		return GiveItemResult{}, err
	}

	return GiveItemResult{Delivery: delivery, Backup: backup}, nil
}

func SetPlayerItemQuantity(db *bbolt.DB, options SetItemQuantityOptions) (SetItemQuantityResult, error) {
	if err := validateSetItemQuantityOptions(options); err != nil {
		return SetItemQuantityResult{}, err
	}

	var mutation InventoryMutation
	backup, err := runLocalSaveTransaction(
		db,
		func(levelPath, outputPath string) error {
			var runErr error
			mutation, runErr = runSetItemQuantityEditor(levelPath, outputPath, options)
			return runErr
		},
	)
	if err != nil {
		return SetItemQuantityResult{}, err
	}
	return SetItemQuantityResult{Mutation: mutation, Backup: backup}, nil
}

func EditPlayerProfile(db *bbolt.DB, options EditPlayerProfileOptions) (EditPlayerProfileResult, error) {
	if err := validateEditPlayerProfileOptions(options); err != nil {
		return EditPlayerProfileResult{}, err
	}

	var profile PlayerProfileMutation
	backup, err := runLocalSaveTransaction(
		db,
		func(levelPath, outputPath string) error {
			var runErr error
			profile, runErr = runEditPlayerProfileEditor(levelPath, outputPath, options)
			return runErr
		},
	)
	if err != nil {
		return EditPlayerProfileResult{}, err
	}
	return EditPlayerProfileResult{Profile: profile, Backup: backup}, nil
}

func EditPlayerStatPoints(db *bbolt.DB, options EditPlayerStatPointsOptions) (EditPlayerStatPointsResult, error) {
	if err := validateEditPlayerStatPointsOptions(options); err != nil {
		return EditPlayerStatPointsResult{}, err
	}

	var mutation PlayerStatPointsMutation
	backup, err := runLocalSaveTransaction(
		db,
		func(levelPath, outputPath string) error {
			var runErr error
			mutation, runErr = runEditPlayerStatPointsEditor(levelPath, outputPath, options)
			return runErr
		},
	)
	if err != nil {
		return EditPlayerStatPointsResult{}, err
	}
	return EditPlayerStatPointsResult{StatPoints: mutation, Backup: backup}, nil
}

func EditPlayerTechnologyPoints(db *bbolt.DB, options EditPlayerTechnologyPointsOptions) (EditPlayerTechnologyPointsResult, error) {
	if err := validateEditPlayerTechnologyPointsOptions(options); err != nil {
		return EditPlayerTechnologyPointsResult{}, err
	}

	var mutation PlayerTechnologyPointsMutation
	backup, err := runLocalPlayerSaveTransaction(
		db,
		options.PlayerUID,
		func(playerPath, outputPath string) error {
			var runErr error
			mutation, runErr = runEditPlayerTechnologyPointsEditor(playerPath, outputPath, options)
			return runErr
		},
	)
	if err != nil {
		return EditPlayerTechnologyPointsResult{}, err
	}
	return EditPlayerTechnologyPointsResult{
		TechnologyPoints: mutation,
		Backup:           backup,
	}, nil
}

func UnlockPlayerMap(db *bbolt.DB, options UnlockPlayerMapOptions) (UnlockPlayerMapResult, error) {
	if err := validateUnlockPlayerMapOptions(options); err != nil {
		return UnlockPlayerMapResult{}, err
	}

	var mutation PlayerMapProgressMutation
	backup, err := runLocalPlayerSaveTransaction(
		db,
		options.PlayerUID,
		func(playerPath, outputPath string) error {
			var runErr error
			mutation, runErr = runUnlockPlayerMapEditor(playerPath, outputPath, options)
			return runErr
		},
	)
	if err != nil {
		return UnlockPlayerMapResult{}, err
	}
	return UnlockPlayerMapResult{
		MapProgress: mutation,
		Backup:      backup,
	}, nil
}

func RenamePal(db *bbolt.DB, options RenamePalOptions) (RenamePalResult, error) {
	if err := validateRenamePalOptions(options); err != nil {
		return RenamePalResult{}, err
	}

	var mutation PalNicknameMutation
	backup, err := runLocalSaveTransaction(
		db,
		func(levelPath, outputPath string) error {
			var runErr error
			mutation, runErr = runRenamePalEditor(levelPath, outputPath, options)
			return runErr
		},
	)
	if err != nil {
		return RenamePalResult{}, err
	}
	return RenamePalResult{Nickname: mutation, Backup: backup}, nil
}

func EditPalLevel(db *bbolt.DB, options EditPalLevelOptions) (EditPalLevelResult, error) {
	if err := validateEditPalLevelOptions(options); err != nil {
		return EditPalLevelResult{}, err
	}

	var mutation PalLevelMutation
	backup, err := runLocalSaveTransaction(
		db,
		func(levelPath, outputPath string) error {
			var runErr error
			mutation, runErr = runEditPalLevelEditor(levelPath, outputPath, options)
			return runErr
		},
	)
	if err != nil {
		return EditPalLevelResult{}, err
	}
	return EditPalLevelResult{Level: mutation, Backup: backup}, nil
}

func RestorePalHealth(db *bbolt.DB, options RestorePalHealthOptions) (RestorePalHealthResult, error) {
	if err := validateRestorePalHealthOptions(options); err != nil {
		return RestorePalHealthResult{}, err
	}

	var mutation PalHealthMutation
	backup, err := runLocalSaveTransaction(
		db,
		func(levelPath, outputPath string) error {
			var runErr error
			mutation, runErr = runRestorePalHealthEditor(levelPath, outputPath, options)
			return runErr
		},
	)
	if err != nil {
		return RestorePalHealthResult{}, err
	}
	return RestorePalHealthResult{Health: mutation, Backup: backup}, nil
}
