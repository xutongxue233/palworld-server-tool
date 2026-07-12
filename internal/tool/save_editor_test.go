package tool

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/spf13/viper"
	"github.com/zaigie/palworld-server-tool/internal/database"
	"go.etcd.io/bbolt"
)

func configureSaveEditorRESTTest(t *testing.T, address string) {
	t.Helper()
	previousAddress := viper.GetString("rest.address")
	previousTimeout := viper.GetInt("rest.timeout")
	viper.Set("rest.address", address)
	viper.Set("rest.timeout", 1)
	t.Cleanup(func() {
		viper.Set("rest.address", previousAddress)
		viper.Set("rest.timeout", previousTimeout)
	})
}

func configureLocalSaveTransactionTest(t *testing.T, savePath string) {
	t.Helper()
	if info, err := os.Stat(savePath); err == nil && info.IsDir() {
		if err := os.MkdirAll(filepath.Join(savePath, "Players"), 0700); err != nil {
			t.Fatal(err)
		}
	}
	previousPath := viper.GetString("save.path")
	viper.Set("save.path", savePath)
	t.Cleanup(func() {
		viper.Set("save.path", previousPath)
	})
	configureSaveEditorRESTTest(t, "")
}

func writeEditedSave(levelPath, outputPath string) error {
	content, err := os.ReadFile(levelPath)
	if err != nil {
		return err
	}
	return os.WriteFile(outputPath, append(content, []byte("-edited")...), 0600)
}

func TestGameServerStatusWithoutRESTAddressIsExplicitOffline(t *testing.T) {
	configureSaveEditorRESTTest(t, "")

	status, err := probeGameServerStatus()
	if err != nil {
		t.Fatal(err)
	}
	if status != gameServerStatusExplicitOffline {
		t.Fatalf("unexpected server status: %d", status)
	}
	if err := ensureGameServerStopped(); err != nil {
		t.Fatalf("unconfigured REST address should allow an explicitly offline edit: %v", err)
	}
}

func TestGameServerRESTErrorMeansOnline(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
	}))
	defer server.Close()
	configureSaveEditorRESTTest(t, server.URL)

	status, err := probeGameServerStatus()
	if err != nil {
		t.Fatal(err)
	}
	if status != gameServerStatusOnline {
		t.Fatalf("unexpected server status: %d", status)
	}
	if err := ensureGameServerStopped(); !errors.Is(err, ErrGameServerRunning) {
		t.Fatalf("expected running server error, got %v", err)
	}
}

func TestGameServerConnectionFailureFailsClosed(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	address := server.URL
	server.Close()
	configureSaveEditorRESTTest(t, address)

	status, err := probeGameServerStatus()
	if status != gameServerStatusUnknown {
		t.Fatalf("unexpected server status: %d", status)
	}
	if !errors.Is(err, ErrGameServerStatusUnknown) {
		t.Fatalf("expected unknown server status error, got %v", err)
	}
	if err := ensureGameServerStopped(); !errors.Is(err, ErrGameServerStatusUnknown) {
		t.Fatalf("connection failure must fail closed, got %v", err)
	}
}

func TestGameServerMetricsParseFailureFailsClosed(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("not-json"))
	}))
	defer server.Close()
	configureSaveEditorRESTTest(t, server.URL)

	status, err := probeGameServerStatus()
	if status != gameServerStatusUnknown {
		t.Fatalf("unexpected server status: %d", status)
	}
	if !errors.Is(err, ErrGameServerStatusUnknown) {
		t.Fatalf("parse failure must fail closed, got %v", err)
	}
}

func TestValidateGiveItemOptions(t *testing.T) {
	valid := GiveItemOptions{
		PlayerUID:            "2119263560",
		ItemID:               "Wood",
		Quantity:             100,
		Container:            "auto",
		ConfirmServerStopped: true,
	}
	if err := validateGiveItemOptions(valid); err != nil {
		t.Fatal(err)
	}
	valid.ConfirmServerStopped = false
	if err := validateGiveItemOptions(valid); !errors.Is(err, ErrSaveEditConfirmation) {
		t.Fatalf("expected confirmation error, got %v", err)
	}
	valid.ConfirmServerStopped = true
	valid.Quantity = 0
	if err := validateGiveItemOptions(valid); err == nil {
		t.Fatal("zero quantity should be rejected")
	}
}

func TestParseDeliveryResult(t *testing.T) {
	output := []byte("INFO: decode complete\nINFO: DELIVERY_RESULT {\"player_uid\":\"1\",\"item_id\":\"Wood\",\"container\":\"main\",\"requested\":20,\"delivered\":20,\"before\":2,\"after\":22,\"modified_slots\":[3]}\n")
	result, err := parseDeliveryResult(output)
	if err != nil {
		t.Fatal(err)
	}
	if result.ItemID != "Wood" || result.After != 22 || len(result.ModifiedSlots) != 1 {
		t.Fatalf("unexpected delivery result: %#v", result)
	}
}

func TestValidateSetItemQuantityOptions(t *testing.T) {
	valid := SetItemQuantityOptions{
		PlayerUID:            "2119263560",
		Container:            "weapons",
		SlotIndex:            3,
		ItemID:               "AssaultRifle_Default1",
		ExpectedDynamicID:    "00000000-0000-0000-0000-000000000000",
		ExpectedQuantity:     1,
		Quantity:             0,
		ConfirmServerStopped: true,
	}
	if err := validateSetItemQuantityOptions(valid); err != nil {
		t.Fatal(err)
	}
	valid.Container = "unknown"
	if err := validateSetItemQuantityOptions(valid); err == nil {
		t.Fatal("unknown container should be rejected")
	}
	valid.Container = "main"
	valid.ExpectedQuantity = 0
	if err := validateSetItemQuantityOptions(valid); err == nil {
		t.Fatal("zero expected quantity should be rejected")
	}
	valid.ExpectedQuantity = 1
	valid.ExpectedDynamicID = ""
	if err := validateSetItemQuantityOptions(valid); err == nil {
		t.Fatal("empty expected dynamic ID should be rejected")
	}
}

func TestParseInventoryMutationResult(t *testing.T) {
	output := []byte("INFO: INVENTORY_RESULT {\"player_uid\":\"1\",\"item_id\":\"Wood\",\"container\":\"main\",\"slot_index\":4,\"before\":123,\"after\":7,\"removed\":false,\"dynamic_record_removed\":false}\n")
	result, err := parseInventoryMutationResult(output)
	if err != nil {
		t.Fatal(err)
	}
	if result.ItemID != "Wood" || result.Before != 123 || result.After != 7 {
		t.Fatalf("unexpected mutation result: %#v", result)
	}
}

func TestValidateEditPlayerProfileOptions(t *testing.T) {
	valid := EditPlayerProfileOptions{
		PlayerUID:            "2119263560",
		ExpectedNickname:     "Old name",
		ExpectedLevel:        2,
		Nickname:             "New name",
		Level:                3,
		ConfirmServerStopped: true,
	}
	if err := validateEditPlayerProfileOptions(valid); err != nil {
		t.Fatal(err)
	}
	valid.Level = 81
	if err := validateEditPlayerProfileOptions(valid); err == nil {
		t.Fatal("level above the game-data maximum should be rejected")
	}
	valid.Level = 2
	valid.Nickname = "Old name"
	if err := validateEditPlayerProfileOptions(valid); err == nil {
		t.Fatal("unchanged player profile should be rejected")
	}
}

func TestParsePlayerProfileResult(t *testing.T) {
	output := []byte("INFO: PLAYER_PROFILE_RESULT {\"player_uid\":\"1\",\"nickname_before\":\"Old\",\"nickname_after\":\"New\",\"level_before\":2,\"level_after\":3,\"exp_before\":64,\"exp_after\":125,\"character_records\":1,\"guild_records\":1}\n")
	result, err := parsePlayerProfileResult(output)
	if err != nil {
		t.Fatal(err)
	}
	if result.NicknameAfter != "New" || result.LevelAfter != 3 || result.ExpAfter != 125 {
		t.Fatalf("unexpected profile result: %#v", result)
	}
}

func TestValidateAndParsePlayerStatPoints(t *testing.T) {
	options := EditPlayerStatPointsOptions{
		PlayerUID:                "2119263560",
		ExpectedUnusedStatPoints: 1,
		UnusedStatPoints:         7,
		ConfirmServerStopped:     true,
	}
	if err := validateEditPlayerStatPointsOptions(options); err != nil {
		t.Fatal(err)
	}
	result, err := parsePlayerStatPointsResult([]byte(
		"INFO: PLAYER_STAT_POINTS_RESULT {\"player_uid\":\"2119263560\",\"before\":1,\"after\":7,\"character_records\":1}\n",
	))
	if err != nil {
		t.Fatal(err)
	}
	if result.Before != 1 || result.After != 7 || result.CharacterRecords != 1 {
		t.Fatalf("unexpected stat points result: %#v", result)
	}
	options.UnusedStatPoints = 1
	if err := validateEditPlayerStatPointsOptions(options); err == nil {
		t.Fatal("unchanged stat points should be rejected")
	}
}

func TestValidateAndParsePlayerTechnologyPoints(t *testing.T) {
	options := EditPlayerTechnologyPointsOptions{
		PlayerUID:                       "2119263560",
		ExpectedTechnologyPoints:        1,
		ExpectedAncientTechnologyPoints: 2,
		TechnologyPoints:                7,
		AncientTechnologyPoints:         5,
		ConfirmServerStopped:            true,
	}
	if err := validateEditPlayerTechnologyPointsOptions(options); err != nil {
		t.Fatal(err)
	}
	result, err := parsePlayerTechnologyPointsResult([]byte(
		"INFO: PLAYER_TECHNOLOGY_POINTS_RESULT {\"player_uid\":\"2119263560\",\"technology_before\":1,\"technology_after\":7,\"ancient_before\":2,\"ancient_after\":5,\"created_fields\":[\"TechnologyPoint\"]}\n",
	))
	if err != nil {
		t.Fatal(err)
	}
	if result.TechnologyAfter != 7 || result.AncientAfter != 5 || len(result.CreatedFields) != 1 {
		t.Fatalf("unexpected technology points result: %#v", result)
	}
	options.TechnologyPoints = 1
	options.AncientTechnologyPoints = 2
	if err := validateEditPlayerTechnologyPointsOptions(options); err == nil {
		t.Fatal("unchanged technology points should be rejected")
	}
}

func TestValidateAndParsePlayerMapProgress(t *testing.T) {
	beforeDigest := strings.Repeat("a", 64)
	afterDigest := strings.Repeat("b", 64)
	options := UnlockPlayerMapOptions{
		PlayerUID:              "2119263560",
		ExpectedProgressDigest: strings.ToUpper(beforeDigest),
		ConfirmServerStopped:   true,
	}
	if err := validateUnlockPlayerMapOptions(options); err != nil {
		t.Fatal(err)
	}

	valid := PlayerMapProgressMutation{
		PlayerUID:            "2119263560",
		FastTravelBefore:     1,
		FastTravelAfter:      playerMapFastTravelTotal,
		FastTravelTotal:      playerMapFastTravelTotal,
		AreasBefore:          2,
		AreasAfter:           playerMapAreasTotal,
		AreasTotal:           playerMapAreasTotal,
		WorldMapsBefore:      1,
		WorldMapsAfter:       playerMapWorldMapsTotal,
		WorldMapsTotal:       playerMapWorldMapsTotal,
		CreatedFields:        []string{"FindAreaFlagMap"},
		ProgressDigestBefore: strings.ToUpper(beforeDigest),
		ProgressDigestAfter:  afterDigest,
		GameVersion:          playerMapGameVersion,
	}
	encode := func(t *testing.T, mutation PlayerMapProgressMutation) []byte {
		t.Helper()
		payload, err := json.Marshal(mutation)
		if err != nil {
			t.Fatal(err)
		}
		return []byte("INFO: PLAYER_MAP_PROGRESS_RESULT " + string(payload) + "\n")
	}

	result, err := parsePlayerMapProgressResult(encode(t, valid))
	if err != nil {
		t.Fatal(err)
	}
	if result.FastTravelAfter != playerMapFastTravelTotal ||
		result.AreasAfter != playerMapAreasTotal ||
		result.WorldMapsAfter != playerMapWorldMapsTotal ||
		result.ProgressDigestBefore != beforeDigest {
		t.Fatalf("unexpected player map progress result: %#v", result)
	}
	if err := validatePlayerMapProgressResultMatchesOptions(result, options); err != nil {
		t.Fatalf("matching player map progress result was rejected: %v", err)
	}

	invalidOptions := options
	invalidOptions.ExpectedProgressDigest = "not-a-digest"
	if err := validateUnlockPlayerMapOptions(invalidOptions); err == nil {
		t.Fatal("invalid map progress digest was accepted")
	}
	missingConfirmation := options
	missingConfirmation.ConfirmServerStopped = false
	if err := validateUnlockPlayerMapOptions(missingConfirmation); !errors.Is(err, ErrSaveEditConfirmation) {
		t.Fatalf("expected confirmation error, got %v", err)
	}

	tests := []struct {
		name   string
		mutate func(*PlayerMapProgressMutation)
	}{
		{"invalid player UID", func(result *PlayerMapProgressMutation) { result.PlayerUID = "invalid" }},
		{"unsupported version", func(result *PlayerMapProgressMutation) { result.GameVersion = "0.6.9" }},
		{"wrong fast travel total", func(result *PlayerMapProgressMutation) { result.FastTravelTotal-- }},
		{"incomplete fast travel", func(result *PlayerMapProgressMutation) { result.FastTravelAfter-- }},
		{"too many previous areas", func(result *PlayerMapProgressMutation) { result.AreasBefore = result.AreasTotal + 1 }},
		{"already fully unlocked", func(result *PlayerMapProgressMutation) {
			result.FastTravelBefore = result.FastTravelTotal
			result.AreasBefore = result.AreasTotal
			result.WorldMapsBefore = result.WorldMapsTotal
		}},
		{"invalid previous digest", func(result *PlayerMapProgressMutation) { result.ProgressDigestBefore = "bad" }},
		{"unchanged digest", func(result *PlayerMapProgressMutation) { result.ProgressDigestAfter = result.ProgressDigestBefore }},
		{"unexpected created field", func(result *PlayerMapProgressMutation) { result.CreatedFields = []string{"Other"} }},
		{"duplicate created field", func(result *PlayerMapProgressMutation) {
			result.CreatedFields = []string{"FindAreaFlagMap", "FindAreaFlagMap"}
		}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			mutation := valid
			mutation.CreatedFields = append([]string(nil), valid.CreatedFields...)
			test.mutate(&mutation)
			if _, err := parsePlayerMapProgressResult(encode(t, mutation)); err == nil {
				t.Fatal("invalid player map progress result was accepted")
			}
		})
	}
	if _, err := parsePlayerMapProgressResult([]byte("INFO: no result\n")); err == nil {
		t.Fatal("missing player map progress result marker was accepted")
	}

	differentPlayer := result
	differentPlayer.PlayerUID = "1"
	if err := validatePlayerMapProgressResultMatchesOptions(differentPlayer, options); err == nil {
		t.Fatal("map progress for a different player was accepted")
	}
	differentDigest := result
	differentDigest.ProgressDigestBefore = strings.Repeat("c", 64)
	if err := validatePlayerMapProgressResultMatchesOptions(differentDigest, options); err == nil {
		t.Fatal("map progress for a different source digest was accepted")
	}
}

func TestValidateAndParsePalNickname(t *testing.T) {
	options := RenamePalOptions{
		PlayerUID:            "2119263560",
		InstanceID:           "c410c416-475c-0638-eb35-269338f2a320",
		ExpectedNickname:     "Old Pal",
		ExpectedLevel:        2,
		ExpectedExp:          25,
		Nickname:             "",
		ConfirmServerStopped: true,
	}
	if err := validateRenamePalOptions(options); err != nil {
		t.Fatalf("clearing a Pal nickname should be valid: %v", err)
	}
	result, err := parsePalNicknameResult([]byte(
		"INFO: PAL_NICKNAME_RESULT {\"player_uid\":\"2119263560\",\"instance_id\":\"c410c416-475c-0638-eb35-269338f2a320\",\"pal_type\":\"ChickenPal\",\"nickname_before\":\"Old Pal\",\"nickname_after\":\"\",\"level\":2,\"exp\":25,\"nickname_created\":false}\n",
	))
	if err != nil {
		t.Fatal(err)
	}
	if result.InstanceID != options.InstanceID || result.NicknameAfter != "" || result.Level != 2 || result.Exp != 25 {
		t.Fatalf("unexpected Pal nickname result: %#v", result)
	}

	options.InstanceID = "00000000-0000-0000-0000-000000000000"
	if err := validateRenamePalOptions(options); err == nil {
		t.Fatal("zero Pal instance ID should be rejected")
	}
	options.InstanceID = "not-a-guid"
	if err := validateRenamePalOptions(options); err == nil {
		t.Fatal("invalid Pal instance ID should be rejected")
	}
	options.InstanceID = "c410c416-475c-0638-eb35-269338f2a320"
	options.Nickname = "Old Pal"
	if err := validateRenamePalOptions(options); err == nil {
		t.Fatal("unchanged Pal nickname should be rejected")
	}
}

func TestValidateEditPalLevelOptions(t *testing.T) {
	valid := EditPalLevelOptions{
		PlayerUID:            "ded7515b-0000-0000-0000-000000000000",
		InstanceID:           "ff38bdad-4710-966d-982f-3ca7cb107b56",
		ExpectedNickname:     "Raid Jet",
		ExpectedLevel:        55,
		ExpectedExp:          6678888,
		ExpectedHP:           7286000,
		ExpectedMaxHP:        0,
		Level:                56,
		ConfirmServerStopped: true,
	}
	if err := validateEditPalLevelOptions(valid); err != nil {
		t.Fatal(err)
	}
	decimalUID := valid
	decimalUID.PlayerUID = "2119263560"
	if err := validateEditPalLevelOptions(decimalUID); err != nil {
		t.Fatalf("decimal player UID should be valid: %v", err)
	}
	emptyNickname := valid
	emptyNickname.ExpectedNickname = ""
	if err := validateEditPalLevelOptions(emptyNickname); err != nil {
		t.Fatalf("an explicitly empty expected nickname should be valid: %v", err)
	}

	tests := []struct {
		name   string
		mutate func(*EditPalLevelOptions)
	}{
		{"invalid player UID", func(options *EditPalLevelOptions) { options.PlayerUID = "not-a-guid" }},
		{"zero player UID", func(options *EditPalLevelOptions) { options.PlayerUID = "00000000-0000-0000-0000-000000000000" }},
		{"invalid instance ID", func(options *EditPalLevelOptions) { options.InstanceID = "not-a-guid" }},
		{"zero instance ID", func(options *EditPalLevelOptions) { options.InstanceID = "00000000-0000-0000-0000-000000000000" }},
		{"long nickname", func(options *EditPalLevelOptions) { options.ExpectedNickname = strings.Repeat("a", 33) }},
		{"nickname control character", func(options *EditPalLevelOptions) { options.ExpectedNickname = "Raid\nJet" }},
		{"expected level below range", func(options *EditPalLevelOptions) { options.ExpectedLevel = 0 }},
		{"expected level above range", func(options *EditPalLevelOptions) { options.ExpectedLevel = 81 }},
		{"negative expected EXP", func(options *EditPalLevelOptions) { options.ExpectedExp = -1 }},
		{"negative expected HP", func(options *EditPalLevelOptions) { options.ExpectedHP = -1 }},
		{"negative expected MaxHP", func(options *EditPalLevelOptions) { options.ExpectedMaxHP = -1 }},
		{"new level below range", func(options *EditPalLevelOptions) { options.Level = 0 }},
		{"new level above range", func(options *EditPalLevelOptions) { options.Level = 81 }},
		{"unchanged level", func(options *EditPalLevelOptions) { options.Level = options.ExpectedLevel }},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			options := valid
			test.mutate(&options)
			if err := validateEditPalLevelOptions(options); err == nil {
				t.Fatal("invalid Pal level options were accepted")
			}
		})
	}

	missingConfirmation := valid
	missingConfirmation.ConfirmServerStopped = false
	if err := validateEditPalLevelOptions(missingConfirmation); !errors.Is(err, ErrSaveEditConfirmation) {
		t.Fatalf("expected confirmation error, got %v", err)
	}
}

func TestParsePalLevelResult(t *testing.T) {
	valid := PalLevelMutation{
		PlayerUID:    "ded7515b-0000-0000-0000-000000000000",
		InstanceID:   "ff38bdad-4710-966d-982f-3ca7cb107b56",
		PalType:      "RAID_JetDragon",
		Nickname:     "Raid Jet",
		LevelBefore:  55,
		LevelAfter:   56,
		ExpBefore:    6678888,
		ExpAfter:     7700000,
		HPBefore:     7286000,
		HPAfter:      7460000,
		MaxHPBefore:  0,
		MaxHPAfter:   7460000,
		HealthField:  "Hp",
		MaxHPCreated: true,
	}
	encode := func(t *testing.T, mutation PalLevelMutation) []byte {
		t.Helper()
		payload, err := json.Marshal(mutation)
		if err != nil {
			t.Fatal(err)
		}
		return []byte("INFO: PAL_LEVEL_RESULT " + string(payload) + "\n")
	}

	result, err := parsePalLevelResult(encode(t, valid))
	if err != nil {
		t.Fatal(err)
	}
	if result.InstanceID != valid.InstanceID || result.LevelBefore != 55 ||
		result.LevelAfter != 56 || result.ExpBefore != 6678888 ||
		result.HPAfter != 7460000 || result.MaxHPAfter != 7460000 ||
		!result.MaxHPCreated {
		t.Fatalf("unexpected Pal level result: %#v", result)
	}
	decimalUID := valid
	decimalUID.PlayerUID = "2119263560"
	if _, err := parsePalLevelResult(encode(t, decimalUID)); err != nil {
		t.Fatalf("decimal player UID result should be valid: %v", err)
	}

	tests := []struct {
		name   string
		mutate func(*PalLevelMutation)
	}{
		{"invalid player UID", func(result *PalLevelMutation) { result.PlayerUID = "not-a-guid" }},
		{"invalid instance ID", func(result *PalLevelMutation) { result.InstanceID = "not-a-guid" }},
		{"empty Pal type", func(result *PalLevelMutation) { result.PalType = "" }},
		{"nickname control character", func(result *PalLevelMutation) { result.Nickname = "Raid\nJet" }},
		{"invalid previous level", func(result *PalLevelMutation) { result.LevelBefore = 0 }},
		{"unchanged level", func(result *PalLevelMutation) { result.LevelAfter = result.LevelBefore }},
		{"negative previous EXP", func(result *PalLevelMutation) { result.ExpBefore = -1 }},
		{"nonpositive new HP", func(result *PalLevelMutation) { result.HPAfter = 0 }},
		{"mismatched HP and MaxHP", func(result *PalLevelMutation) { result.MaxHPAfter++ }},
		{"invalid health field", func(result *PalLevelMutation) { result.HealthField = "Health" }},
		{"inconsistent MaxHP creation", func(result *PalLevelMutation) { result.MaxHPBefore = 1 }},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			mutation := valid
			test.mutate(&mutation)
			if _, err := parsePalLevelResult(encode(t, mutation)); err == nil {
				t.Fatal("invalid Pal level result was accepted")
			}
		})
	}
	if _, err := parsePalLevelResult([]byte("INFO: no result\n")); err == nil {
		t.Fatal("missing Pal level result marker was accepted")
	}
	if _, err := parsePalLevelResult([]byte("PAL_LEVEL_RESULT {\n")); err == nil {
		t.Fatal("malformed Pal level result was accepted")
	}
}

func TestValidateRestorePalHealthOptions(t *testing.T) {
	valid := RestorePalHealthOptions{
		PlayerUID:            "ded7515b-0000-0000-0000-000000000000",
		InstanceID:           "ff38bdad-4710-966d-982f-3ca7cb107b56",
		ExpectedNickname:     "",
		ExpectedLevel:        55,
		ExpectedExp:          6678888,
		ExpectedHP:           1,
		ExpectedMaxHP:        7286000,
		ConfirmServerStopped: true,
	}
	for _, playerUID := range []string{
		"2119263560",
		"7e516548",
		"ded7515b-0000-0000-0000-000000000000",
	} {
		options := valid
		options.PlayerUID = playerUID
		if err := validateRestorePalHealthOptions(options); err != nil {
			t.Fatalf("valid player UID %q rejected: %v", playerUID, err)
		}
	}

	tests := []struct {
		name   string
		mutate func(*RestorePalHealthOptions)
	}{
		{"invalid player UID", func(options *RestorePalHealthOptions) { options.PlayerUID = "not-a-guid" }},
		{"zero player UID", func(options *RestorePalHealthOptions) { options.PlayerUID = "00000000-0000-0000-0000-000000000000" }},
		{"invalid instance ID", func(options *RestorePalHealthOptions) { options.InstanceID = "not-a-guid" }},
		{"zero instance ID", func(options *RestorePalHealthOptions) { options.InstanceID = "00000000-0000-0000-0000-000000000000" }},
		{"long nickname", func(options *RestorePalHealthOptions) { options.ExpectedNickname = strings.Repeat("a", 33) }},
		{"nickname control character", func(options *RestorePalHealthOptions) { options.ExpectedNickname = "Raid\nJet" }},
		{"expected level below range", func(options *RestorePalHealthOptions) { options.ExpectedLevel = 0 }},
		{"expected level above range", func(options *RestorePalHealthOptions) { options.ExpectedLevel = 81 }},
		{"negative expected EXP", func(options *RestorePalHealthOptions) { options.ExpectedExp = -1 }},
		{"negative expected HP", func(options *RestorePalHealthOptions) { options.ExpectedHP = -1 }},
		{"zero expected MaxHP", func(options *RestorePalHealthOptions) { options.ExpectedMaxHP = 0 }},
		{"negative expected MaxHP", func(options *RestorePalHealthOptions) { options.ExpectedMaxHP = -1 }},
		{"already full health", func(options *RestorePalHealthOptions) { options.ExpectedHP = options.ExpectedMaxHP }},
		{"health above maximum", func(options *RestorePalHealthOptions) { options.ExpectedHP = options.ExpectedMaxHP + 1 }},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			options := valid
			test.mutate(&options)
			if err := validateRestorePalHealthOptions(options); err == nil {
				t.Fatal("invalid Pal health options were accepted")
			}
		})
	}

	missingConfirmation := valid
	missingConfirmation.ConfirmServerStopped = false
	if err := validateRestorePalHealthOptions(missingConfirmation); !errors.Is(err, ErrSaveEditConfirmation) {
		t.Fatalf("expected confirmation error, got %v", err)
	}
}

func TestParsePalHealthResult(t *testing.T) {
	valid := PalHealthMutation{
		PlayerUID:   "ded7515b-0000-0000-0000-000000000000",
		InstanceID:  "ff38bdad-4710-966d-982f-3ca7cb107b56",
		PalType:     "RAID_JetDragon",
		Nickname:    "Raid Jet",
		Level:       55,
		Exp:         6678888,
		HPBefore:    1,
		HPAfter:     7286000,
		MaxHP:       7286000,
		HealthField: "Hp",
	}
	encode := func(t *testing.T, mutation PalHealthMutation) []byte {
		t.Helper()
		payload, err := json.Marshal(mutation)
		if err != nil {
			t.Fatal(err)
		}
		return []byte("INFO: PAL_HEALTH_RESULT " + string(payload) + "\n")
	}

	result, err := parsePalHealthResult(encode(t, valid))
	if err != nil {
		t.Fatal(err)
	}
	if result.InstanceID != valid.InstanceID || result.HPBefore != 1 ||
		result.HPAfter != result.MaxHP || result.HealthField != "Hp" {
		t.Fatalf("unexpected Pal health result: %#v", result)
	}
	uppercase := valid
	uppercase.HealthField = "HP"
	if _, err := parsePalHealthResult(encode(t, uppercase)); err != nil {
		t.Fatalf("legacy uppercase HP field should be valid: %v", err)
	}
	decimalUID := valid
	decimalUID.PlayerUID = "2119263560"
	if _, err := parsePalHealthResult(encode(t, decimalUID)); err != nil {
		t.Fatalf("decimal player UID result should be valid: %v", err)
	}

	tests := []struct {
		name   string
		mutate func(*PalHealthMutation)
	}{
		{"invalid player UID", func(result *PalHealthMutation) { result.PlayerUID = "not-a-guid" }},
		{"invalid instance ID", func(result *PalHealthMutation) { result.InstanceID = "not-a-guid" }},
		{"empty Pal type", func(result *PalHealthMutation) { result.PalType = "" }},
		{"nickname control character", func(result *PalHealthMutation) { result.Nickname = "Raid\nJet" }},
		{"invalid level", func(result *PalHealthMutation) { result.Level = 0 }},
		{"negative EXP", func(result *PalHealthMutation) { result.Exp = -1 }},
		{"negative previous HP", func(result *PalHealthMutation) { result.HPBefore = -1 }},
		{"nonpositive MaxHP", func(result *PalHealthMutation) { result.MaxHP = 0; result.HPAfter = 0 }},
		{"not restored to MaxHP", func(result *PalHealthMutation) { result.HPAfter-- }},
		{"unchanged HP", func(result *PalHealthMutation) { result.HPBefore = result.HPAfter }},
		{"decreased HP", func(result *PalHealthMutation) { result.HPBefore = result.HPAfter + 1 }},
		{"invalid health field", func(result *PalHealthMutation) { result.HealthField = "Health" }},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			mutation := valid
			test.mutate(&mutation)
			if _, err := parsePalHealthResult(encode(t, mutation)); err == nil {
				t.Fatal("invalid Pal health result was accepted")
			}
		})
	}
	if _, err := parsePalHealthResult([]byte("INFO: no result\n")); err == nil {
		t.Fatal("missing Pal health result marker was accepted")
	}
	if _, err := parsePalHealthResult([]byte("PAL_HEALTH_RESULT {\n")); err == nil {
		t.Fatal("malformed Pal health result was accepted")
	}
}

func TestValidatePalHealthResultMatchesOptions(t *testing.T) {
	options := RestorePalHealthOptions{
		PlayerUID:            "2119263560",
		InstanceID:           "FF38BDAD4710966D982F3CA7CB107B56",
		ExpectedNickname:     "Raid Jet",
		ExpectedLevel:        55,
		ExpectedExp:          6678888,
		ExpectedHP:           1,
		ExpectedMaxHP:        7286000,
		ConfirmServerStopped: true,
	}
	valid := PalHealthMutation{
		PlayerUID:   "7e516548-0000-0000-0000-000000000000",
		InstanceID:  "ff38bdad-4710-966d-982f-3ca7cb107b56",
		PalType:     "RAID_JetDragon",
		Nickname:    "Raid Jet",
		Level:       55,
		Exp:         6678888,
		HPBefore:    1,
		HPAfter:     7286000,
		MaxHP:       7286000,
		HealthField: "Hp",
	}
	if err := validatePalHealthResultMatchesOptions(valid, options); err != nil {
		t.Fatalf("canonical-equivalent Pal health result was rejected: %v", err)
	}

	tests := []struct {
		name   string
		mutate func(*PalHealthMutation)
	}{
		{"different player UID", func(result *PalHealthMutation) {
			result.PlayerUID = "00000001-0000-0000-0000-000000000000"
		}},
		{"different instance ID", func(result *PalHealthMutation) {
			result.InstanceID = "ff38bdad-4710-966d-982f-3ca7cb107b57"
		}},
		{"different nickname", func(result *PalHealthMutation) { result.Nickname = "Other" }},
		{"different level", func(result *PalHealthMutation) { result.Level++ }},
		{"different EXP", func(result *PalHealthMutation) { result.Exp++ }},
		{"different previous HP", func(result *PalHealthMutation) { result.HPBefore++ }},
		{"different MaxHP", func(result *PalHealthMutation) {
			result.MaxHP++
			result.HPAfter++
		}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			result := valid
			test.mutate(&result)
			if err := validatePalHealthResultMatchesOptions(result, options); err == nil {
				t.Fatal("Pal health result that did not match the request was accepted")
			}
		})
	}
}

func TestResolvePlayerSavePath(t *testing.T) {
	dir := t.TempDir()
	levelPath := filepath.Join(dir, "Level.sav")
	if err := os.WriteFile(levelPath, []byte("level"), 0600); err != nil {
		t.Fatal(err)
	}
	playersDir := filepath.Join(dir, "Players")
	if err := os.MkdirAll(playersDir, 0700); err != nil {
		t.Fatal(err)
	}
	playerPath := filepath.Join(playersDir, "7E516548000000000000000000000000.sav")
	if err := os.WriteFile(playerPath, []byte("player"), 0600); err != nil {
		t.Fatal(err)
	}

	for _, uid := range []string{
		"2119263560",
		"7e516548",
		"7e516548-0000-0000-0000-000000000000",
	} {
		resolved, err := resolvePlayerSavePath(levelPath, uid)
		if err != nil {
			t.Fatalf("resolve %q: %v", uid, err)
		}
		if !strings.EqualFold(resolved, playerPath) {
			t.Fatalf("resolve %q = %q, want %q", uid, resolved, playerPath)
		}
	}
}

func TestPlayerSaveTransactionGuardsBothFilesAndSkipsBackupOnFailure(t *testing.T) {
	dir := t.TempDir()
	levelPath := filepath.Join(dir, "Level.sav")
	if err := os.WriteFile(levelPath, []byte("level"), 0600); err != nil {
		t.Fatal(err)
	}
	playersDir := filepath.Join(dir, "Players")
	if err := os.MkdirAll(playersDir, 0700); err != nil {
		t.Fatal(err)
	}
	playerPath := filepath.Join(playersDir, "7E516548000000000000000000000000.sav")
	if err := os.WriteFile(playerPath, []byte("player"), 0600); err != nil {
		t.Fatal(err)
	}
	configureLocalSaveTransactionTest(t, dir)

	backupCalls := 0
	runnerErr := errors.New("stale technology points")
	_, err := runLocalPlayerSaveTransactionWithBackup(
		nil,
		"2119263560",
		func(_, _ string) error { return runnerErr },
		func(*bbolt.DB) (database.Backup, error) {
			backupCalls++
			return database.Backup{}, nil
		},
	)
	if !errors.Is(err, runnerErr) || backupCalls != 0 {
		t.Fatalf("runner failure = %v, backup calls = %d", err, backupCalls)
	}

	backupCalls = 0
	_, err = runLocalPlayerSaveTransactionWithBackup(
		nil,
		"2119263560",
		func(playerInput, outputPath string) error {
			if err := writeEditedSave(playerInput, outputPath); err != nil {
				return err
			}
			return os.WriteFile(levelPath, []byte("external-level"), 0600)
		},
		func(*bbolt.DB) (database.Backup, error) {
			backupCalls++
			return database.Backup{}, nil
		},
	)
	if !errors.Is(err, ErrSaveSourceChanged) || backupCalls != 0 {
		t.Fatalf("Level guard = %v, backup calls = %d", err, backupCalls)
	}
}

func TestParseSaveEditorMachineError(t *testing.T) {
	result, ok := parseSaveEditorMachineError([]byte(
		`ERROR: SAVE_EDIT_ERROR {"code":"stale_state","message":"slot changed"}`,
	))
	if !ok || result.Code != "stale_state" || result.Message != "slot changed" {
		t.Fatalf("unexpected machine error: %#v, ok=%v", result, ok)
	}
}

func TestRunLocalSaveTransactionRunnerFailuresDoNotCreateBackup(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "Level.sav"), []byte("source"), 0600); err != nil {
		t.Fatal(err)
	}
	configureLocalSaveTransactionTest(t, dir)

	for _, name := range []string{"invalid item", "stale inventory slot", "stale player profile"} {
		t.Run(name, func(t *testing.T) {
			runnerErr := errors.New(name)
			backupCalls := 0
			_, err := runLocalSaveTransactionWithBackup(
				nil,
				func(_, _ string) error { return runnerErr },
				func(*bbolt.DB) (database.Backup, error) {
					backupCalls++
					return database.Backup{}, nil
				},
			)
			if !errors.Is(err, runnerErr) {
				t.Fatalf("expected runner error, got %v", err)
			}
			if backupCalls != 0 {
				t.Fatalf("backup called %d times after runner failure", backupCalls)
			}
		})
	}
}

func TestRunLocalSaveTransactionRechecksTargetBeforeBackup(t *testing.T) {
	dir := t.TempDir()
	targetPath := filepath.Join(dir, "Level.sav")
	if err := os.WriteFile(targetPath, []byte("source"), 0600); err != nil {
		t.Fatal(err)
	}
	configureLocalSaveTransactionTest(t, dir)

	backupCalls := 0
	_, err := runLocalSaveTransactionWithBackup(
		nil,
		func(levelPath, outputPath string) error {
			if err := writeEditedSave(levelPath, outputPath); err != nil {
				return err
			}
			return os.WriteFile(targetPath, []byte("external-update"), 0600)
		},
		func(*bbolt.DB) (database.Backup, error) {
			backupCalls++
			return database.Backup{}, nil
		},
	)
	if !errors.Is(err, ErrSaveSourceChanged) {
		t.Fatalf("expected changed source error, got %v", err)
	}
	if backupCalls != 0 {
		t.Fatalf("backup called %d times after the target changed", backupCalls)
	}
}

func TestRunLocalSaveTransactionClassifiesBackupFailureAsInternal(t *testing.T) {
	dir := t.TempDir()
	targetPath := filepath.Join(dir, "Level.sav")
	if err := os.WriteFile(targetPath, []byte("source"), 0600); err != nil {
		t.Fatal(err)
	}
	configureLocalSaveTransactionTest(t, dir)

	backupCalls := 0
	_, err := runLocalSaveTransactionWithBackup(
		nil,
		writeEditedSave,
		func(*bbolt.DB) (database.Backup, error) {
			backupCalls++
			return database.Backup{}, errors.New("database unavailable")
		},
	)
	if !errors.Is(err, ErrSaveEditInternal) {
		t.Fatalf("expected internal save edit error, got %v", err)
	}
	if backupCalls != 1 {
		t.Fatalf("backup called %d times, want 1", backupCalls)
	}
	content, readErr := os.ReadFile(targetPath)
	if readErr != nil {
		t.Fatal(readErr)
	}
	if string(content) != "source" {
		t.Fatalf("target changed after backup failure: %q", content)
	}
}

func TestRunLocalSaveTransactionClassifiesReplacementFailureAsInternal(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "Level.sav"), []byte("source"), 0600); err != nil {
		t.Fatal(err)
	}
	configureLocalSaveTransactionTest(t, dir)

	_, err := runLocalSaveTransactionWithBackup(
		nil,
		func(_, _ string) error { return nil },
		func(*bbolt.DB) (database.Backup, error) {
			return database.Backup{Path: "pre-edit.zip"}, nil
		},
	)
	if !errors.Is(err, ErrSaveEditInternal) {
		t.Fatalf("expected internal save edit error, got %v", err)
	}
}

func TestRunLocalSaveTransactionClassifiesFilesystemFailureAsInternal(t *testing.T) {
	missingPath := filepath.Join(t.TempDir(), "missing")
	configureLocalSaveTransactionTest(t, missingPath)

	backupCalls := 0
	_, err := runLocalSaveTransactionWithBackup(
		nil,
		writeEditedSave,
		func(*bbolt.DB) (database.Backup, error) {
			backupCalls++
			return database.Backup{}, nil
		},
	)
	if !errors.Is(err, ErrSaveEditInternal) {
		t.Fatalf("expected internal save edit error, got %v", err)
	}
	if backupCalls != 0 {
		t.Fatalf("backup called %d times for a missing save", backupCalls)
	}
}

func TestRunSaveEditorClassifiesLaunchFailureAsInternal(t *testing.T) {
	notExecutable := filepath.Join(t.TempDir(), "not-an-executable")
	if err := os.WriteFile(notExecutable, []byte("not an executable"), 0600); err != nil {
		t.Fatal(err)
	}
	previousPath := viper.GetString("save.decode_path")
	viper.Set("save.decode_path", notExecutable)
	t.Cleanup(func() { viper.Set("save.decode_path", previousPath) })

	_, err := runSaveEditor(nil)
	if !errors.Is(err, ErrSaveEditInternal) {
		t.Fatalf("expected internal save edit error, got %v", err)
	}
}

func TestRunSaveEditorClassifiesUnmarkedExitAsInternal(t *testing.T) {
	testBinary, err := filepath.Abs(os.Args[0])
	if err != nil {
		t.Fatal(err)
	}
	previousPath := viper.GetString("save.decode_path")
	viper.Set("save.decode_path", testBinary)
	t.Cleanup(func() { viper.Set("save.decode_path", previousPath) })

	_, err = runSaveEditor([]string{"-definitely-not-a-go-test-flag"})
	if err == nil {
		t.Fatal("expected editor process to reject the argument")
	}
	if !errors.Is(err, ErrSaveEditInternal) {
		t.Fatalf("expected unmarked editor exit to be internal, got %v", err)
	}
}

func TestRunSaveEditorTimeoutIsInternal(t *testing.T) {
	if os.Getenv("PST_SAVE_EDITOR_TIMEOUT_HELPER") == "1" {
		time.Sleep(5 * time.Second)
		return
	}
	testBinary, err := filepath.Abs(os.Args[0])
	if err != nil {
		t.Fatal(err)
	}
	previousPath := viper.GetString("save.decode_path")
	previousTimeout := saveEditorTimeout
	viper.Set("save.decode_path", testBinary)
	saveEditorTimeout = 20 * time.Millisecond
	t.Setenv("PST_SAVE_EDITOR_TIMEOUT_HELPER", "1")
	t.Cleanup(func() {
		viper.Set("save.decode_path", previousPath)
		saveEditorTimeout = previousTimeout
	})

	_, err = runSaveEditor([]string{"-test.run=TestRunSaveEditorTimeoutIsInternal"})
	if !errors.Is(err, ErrSaveEditInternal) || !strings.Contains(err.Error(), "timed out") {
		t.Fatalf("expected internal timeout error, got %v", err)
	}
}

func TestStageAndReplaceSave(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "Level.sav")
	edited := filepath.Join(dir, "edited.sav")
	if err := os.WriteFile(target, []byte("old"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(edited, []byte("new-save"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := stageAndReplaceSave(edited, target); err != nil {
		t.Fatal(err)
	}
	content, err := os.ReadFile(target)
	if err != nil {
		t.Fatal(err)
	}
	if string(content) != "new-save" {
		t.Fatalf("unexpected replacement content: %q", content)
	}
}

func TestStageAndReplaceSaveRejectsChangedSource(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "Level.sav")
	edited := filepath.Join(dir, "edited.sav")
	if err := os.WriteFile(target, []byte("source-version"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(edited, []byte("edited-version"), 0600); err != nil {
		t.Fatal(err)
	}
	expected, err := fingerprintSaveFile(target)
	if err != nil {
		t.Fatal(err)
	}

	err = stageAndReplaceSaveGuarded(edited, target, &expected, func() error {
		return os.WriteFile(target, []byte("external-update"), 0600)
	})
	if !errors.Is(err, ErrSaveSourceChanged) {
		t.Fatalf("expected changed source error, got %v", err)
	}
	content, readErr := os.ReadFile(target)
	if readErr != nil {
		t.Fatal(readErr)
	}
	if string(content) != "external-update" {
		t.Fatalf("external update was overwritten: %q", content)
	}
}

func TestStageAndReplaceSaveHonorsPreReplaceCheck(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "Level.sav")
	edited := filepath.Join(dir, "edited.sav")
	if err := os.WriteFile(target, []byte("source-version"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(edited, []byte("edited-version"), 0600); err != nil {
		t.Fatal(err)
	}
	expected, err := fingerprintSaveFile(target)
	if err != nil {
		t.Fatal(err)
	}

	err = stageAndReplaceSaveGuarded(edited, target, &expected, func() error {
		return ErrGameServerRunning
	})
	if !errors.Is(err, ErrGameServerRunning) {
		t.Fatalf("expected pre-replace check error, got %v", err)
	}
	content, readErr := os.ReadFile(target)
	if readErr != nil {
		t.Fatal(readErr)
	}
	if string(content) != "source-version" {
		t.Fatalf("target changed despite failed pre-replace check: %q", content)
	}
}

func TestRemoteSaveSourcesAreRejected(t *testing.T) {
	for _, path := range []string{
		"https://agent.example/sync",
		"docker://palworld:/game/Saved",
		"k8s://default/palworld/server:/game/Saved",
	} {
		if !isRemoteSaveSource(path) {
			t.Fatalf("remote source was not detected: %s", path)
		}
	}
}

func TestBackupFileNameIncludesMillisecondsAndUniqueSuffix(t *testing.T) {
	now := time.Date(2026, time.July, 11, 21, 22, 53, 123456789, time.UTC)
	first := backupFileName(now)
	second := backupFileName(now)
	if !strings.HasPrefix(first, "2026-07-11-21-22-53-123-") ||
		!strings.HasSuffix(first, ".zip") {
		t.Fatalf("unexpected backup filename: %s", first)
	}
	if first == second {
		t.Fatalf("backup filename suffixes collided: %s", first)
	}
}
