package tool

import (
	"archive/zip"
	"bytes"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"testing"

	"github.com/spf13/viper"
	"github.com/zaigie/palworld-server-tool/internal/database"
	"github.com/zaigie/palworld-server-tool/internal/system"
	"go.etcd.io/bbolt"
)

func TestGivePlayerItemRealSave(t *testing.T) {
	sourceDir := os.Getenv("PST_SAVE_TEST_DIR")
	playerUID := os.Getenv("PST_SAVE_TEST_PLAYER_UID")
	decodePath := os.Getenv("PST_SAVE_TEST_CLI")
	if sourceDir == "" || playerUID == "" || decodePath == "" {
		t.Skip("set PST_SAVE_TEST_DIR, PST_SAVE_TEST_PLAYER_UID, and PST_SAVE_TEST_CLI")
	}

	decodePath, err := filepath.Abs(decodePath)
	if err != nil {
		t.Fatal(err)
	}
	workspace := t.TempDir()
	levelPath := filepath.Join(workspace, "Level.sav")
	if err := system.CopyFile(filepath.Join(sourceDir, "Level.sav"), levelPath); err != nil {
		t.Fatal(err)
	}
	if err := system.CopyDir(
		filepath.Join(sourceDir, "Players"),
		filepath.Join(workspace, "Players"),
	); err != nil {
		t.Fatal(err)
	}
	before, err := os.ReadFile(levelPath)
	if err != nil {
		t.Fatal(err)
	}

	db, err := bbolt.Open(filepath.Join(workspace, "test.db"), 0600, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if err := db.Update(func(tx *bbolt.Tx) error {
		_, err := tx.CreateBucketIfNotExists([]byte("backups"))
		return err
	}); err != nil {
		t.Fatal(err)
	}

	viper.Reset()
	t.Cleanup(viper.Reset)
	viper.Set("save.path", workspace)
	viper.Set("save.decode_path", decodePath)
	viper.Set("rest.address", "")
	t.Chdir(workspace)

	material, err := GivePlayerItem(db, GiveItemOptions{
		PlayerUID:            playerUID,
		ItemID:               "Wood",
		Quantity:             123,
		Container:            "auto",
		ConfirmServerStopped: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	weapon, err := GivePlayerItem(db, GiveItemOptions{
		PlayerUID:            playerUID,
		ItemID:               "AssaultRifle_Default1",
		Quantity:             1,
		Container:            "auto",
		ConfirmServerStopped: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	materialEdit, err := SetPlayerItemQuantity(db, SetItemQuantityOptions{
		PlayerUID:            playerUID,
		Container:            "main",
		SlotIndex:            material.Delivery.ModifiedSlots[0],
		ItemID:               "Wood",
		ExpectedQuantity:     123,
		ExpectedDynamicID:    "00000000-0000-0000-0000-000000000000",
		Quantity:             7,
		ConfirmServerStopped: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	weaponRemoval, err := SetPlayerItemQuantity(db, SetItemQuantityOptions{
		PlayerUID:            playerUID,
		Container:            "main",
		SlotIndex:            weapon.Delivery.ModifiedSlots[0],
		ItemID:               "AssaultRifle_Default1",
		ExpectedQuantity:     1,
		ExpectedDynamicID:    weapon.Delivery.DynamicIDs[weapon.Delivery.ModifiedSlots[0]],
		Quantity:             0,
		ConfirmServerStopped: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if material.Delivery.Delivered != 123 || weapon.Delivery.Delivered != 1 {
		t.Fatalf("unexpected deliveries: %#v %#v", material, weapon)
	}
	if materialEdit.Mutation.After != 7 || !weaponRemoval.Mutation.Removed ||
		!weaponRemoval.Mutation.DynamicRecordRemoved {
		t.Fatalf("unexpected inventory mutations: %#v %#v", materialEdit, weaponRemoval)
	}
	backupPaths := map[string]struct{}{}
	for _, backup := range []database.Backup{
		material.Backup,
		weapon.Backup,
		materialEdit.Backup,
		weaponRemoval.Backup,
	} {
		if _, exists := backupPaths[backup.Path]; exists {
			t.Fatalf("successive backups collided: %s", backup.Path)
		}
		backupPaths[backup.Path] = struct{}{}
		if _, err := os.Stat(filepath.Join(workspace, "backups", backup.Path)); err != nil {
			t.Fatalf("tracked backup is missing: %v", err)
		}
	}
	after, err := os.ReadFile(levelPath)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Equal(before, after) {
		t.Fatal("Level.sav was not replaced")
	}
}

func TestEditPlayerProfileRealSave(t *testing.T) {
	sourceDir := os.Getenv("PST_SAVE_TEST_DIR")
	playerUID := os.Getenv("PST_SAVE_TEST_PLAYER_UID")
	decodePath := os.Getenv("PST_SAVE_TEST_CLI")
	expectedNickname := os.Getenv("PST_SAVE_TEST_PLAYER_NICKNAME")
	expectedLevelText := os.Getenv("PST_SAVE_TEST_PLAYER_LEVEL")
	if sourceDir == "" || playerUID == "" || decodePath == "" ||
		expectedNickname == "" || expectedLevelText == "" {
		t.Skip("set PST_SAVE_TEST_DIR, PST_SAVE_TEST_PLAYER_UID, PST_SAVE_TEST_CLI, PST_SAVE_TEST_PLAYER_NICKNAME, and PST_SAVE_TEST_PLAYER_LEVEL")
	}
	expectedLevel, err := strconv.Atoi(expectedLevelText)
	if err != nil || expectedLevel < 1 || expectedLevel >= 80 {
		t.Fatalf("PST_SAVE_TEST_PLAYER_LEVEL must be between 1 and 79: %q", expectedLevelText)
	}

	decodePath, err = filepath.Abs(decodePath)
	if err != nil {
		t.Fatal(err)
	}
	workspace := t.TempDir()
	levelPath := filepath.Join(workspace, "Level.sav")
	if err := system.CopyFile(filepath.Join(sourceDir, "Level.sav"), levelPath); err != nil {
		t.Fatal(err)
	}
	if err := system.CopyDir(
		filepath.Join(sourceDir, "Players"),
		filepath.Join(workspace, "Players"),
	); err != nil {
		t.Fatal(err)
	}
	before, err := os.ReadFile(levelPath)
	if err != nil {
		t.Fatal(err)
	}

	db, err := bbolt.Open(filepath.Join(workspace, "test.db"), 0600, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if err := db.Update(func(tx *bbolt.Tx) error {
		_, err := tx.CreateBucketIfNotExists([]byte("backups"))
		return err
	}); err != nil {
		t.Fatal(err)
	}

	viper.Reset()
	t.Cleanup(viper.Reset)
	viper.Set("save.path", workspace)
	viper.Set("save.decode_path", decodePath)
	viper.Set("rest.address", "")
	t.Chdir(workspace)

	result, err := EditPlayerProfile(db, EditPlayerProfileOptions{
		PlayerUID:            playerUID,
		ExpectedNickname:     expectedNickname,
		ExpectedLevel:        expectedLevel,
		Nickname:             "PST profile test",
		Level:                expectedLevel + 1,
		ConfirmServerStopped: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.Profile.NicknameBefore != expectedNickname ||
		result.Profile.NicknameAfter != "PST profile test" ||
		result.Profile.LevelBefore != expectedLevel ||
		result.Profile.LevelAfter != expectedLevel+1 ||
		result.Profile.CharacterRecords != 1 {
		t.Fatalf("unexpected profile mutation: %#v", result.Profile)
	}
	if result.Profile.ExpAfter < 0 || result.Profile.ExpAfter == result.Profile.ExpBefore {
		t.Fatalf("level edit did not update experience: %#v", result.Profile)
	}
	if _, err := os.Stat(filepath.Join(workspace, "backups", result.Backup.Path)); err != nil {
		t.Fatalf("tracked backup is missing: %v", err)
	}
	after, err := os.ReadFile(levelPath)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Equal(before, after) {
		t.Fatal("Level.sav was not replaced")
	}
}

func TestEditPlayerTechnologyPointsRealSave(t *testing.T) {
	sourceDir := os.Getenv("PST_SAVE_TEST_DIR")
	playerUID := os.Getenv("PST_SAVE_TEST_PLAYER_UID")
	decodePath := os.Getenv("PST_SAVE_TEST_CLI")
	if sourceDir == "" || playerUID == "" || decodePath == "" {
		t.Skip("set PST_SAVE_TEST_DIR, PST_SAVE_TEST_PLAYER_UID, and PST_SAVE_TEST_CLI")
	}
	decodePath, err := filepath.Abs(decodePath)
	if err != nil {
		t.Fatal(err)
	}
	workspace := t.TempDir()
	levelPath := filepath.Join(workspace, "Level.sav")
	if err := system.CopyFile(filepath.Join(sourceDir, "Level.sav"), levelPath); err != nil {
		t.Fatal(err)
	}
	if err := system.CopyDir(filepath.Join(sourceDir, "Players"), filepath.Join(workspace, "Players")); err != nil {
		t.Fatal(err)
	}
	playerPath, err := resolvePlayerSavePath(levelPath, playerUID)
	if err != nil {
		t.Fatal(err)
	}
	levelBefore, err := os.ReadFile(levelPath)
	if err != nil {
		t.Fatal(err)
	}
	playerBefore, err := os.ReadFile(playerPath)
	if err != nil {
		t.Fatal(err)
	}
	db, err := bbolt.Open(filepath.Join(workspace, "test.db"), 0600, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if err := db.Update(func(tx *bbolt.Tx) error {
		_, err := tx.CreateBucketIfNotExists([]byte("backups"))
		return err
	}); err != nil {
		t.Fatal(err)
	}
	viper.Reset()
	t.Cleanup(viper.Reset)
	viper.Set("save.path", workspace)
	viper.Set("save.decode_path", decodePath)
	viper.Set("rest.address", "")
	t.Chdir(workspace)

	result, err := EditPlayerTechnologyPoints(db, EditPlayerTechnologyPointsOptions{
		PlayerUID:                       playerUID,
		ExpectedTechnologyPoints:        0,
		ExpectedAncientTechnologyPoints: 0,
		TechnologyPoints:                6,
		AncientTechnologyPoints:         3,
		ConfirmServerStopped:            true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.TechnologyPoints.TechnologyBefore != 0 ||
		result.TechnologyPoints.TechnologyAfter != 6 ||
		result.TechnologyPoints.AncientBefore != 0 ||
		result.TechnologyPoints.AncientAfter != 3 {
		t.Fatalf("unexpected technology point mutation: %#v", result.TechnologyPoints)
	}
	levelAfter, err := os.ReadFile(levelPath)
	if err != nil {
		t.Fatal(err)
	}
	playerAfter, err := os.ReadFile(playerPath)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(levelBefore, levelAfter) {
		t.Fatal("Level.sav changed during a player-save-only transaction")
	}
	if bytes.Equal(playerBefore, playerAfter) {
		t.Fatal("player save was not replaced")
	}
	if _, err := os.Stat(filepath.Join(workspace, "backups", result.Backup.Path)); err != nil {
		t.Fatalf("tracked backup is missing: %v", err)
	}
}

func TestRenamePalRealSave(t *testing.T) {
	levelSource := os.Getenv("PST_PAL_SAVE_TEST_LEVEL")
	playerUID := os.Getenv("PST_PAL_SAVE_TEST_PLAYER_UID")
	instanceID := os.Getenv("PST_PAL_SAVE_TEST_INSTANCE_ID")
	decodePath := os.Getenv("PST_SAVE_TEST_CLI")
	expectedLevelText := os.Getenv("PST_PAL_SAVE_TEST_LEVEL_VALUE")
	expectedExpText := os.Getenv("PST_PAL_SAVE_TEST_EXP")
	if levelSource == "" || playerUID == "" || instanceID == "" || decodePath == "" ||
		expectedLevelText == "" || expectedExpText == "" {
		t.Skip("set PST_PAL_SAVE_TEST_LEVEL, PST_PAL_SAVE_TEST_PLAYER_UID, PST_PAL_SAVE_TEST_INSTANCE_ID, PST_PAL_SAVE_TEST_LEVEL_VALUE, PST_PAL_SAVE_TEST_EXP, and PST_SAVE_TEST_CLI")
	}
	expectedLevel, err := strconv.Atoi(expectedLevelText)
	if err != nil || expectedLevel < 1 || expectedLevel > 80 {
		t.Fatalf("invalid Pal level %q", expectedLevelText)
	}
	expectedExp, err := strconv.ParseInt(expectedExpText, 10, 64)
	if err != nil || expectedExp < 0 {
		t.Fatalf("invalid Pal EXP %q", expectedExpText)
	}
	decodePath, err = filepath.Abs(decodePath)
	if err != nil {
		t.Fatal(err)
	}
	levelSource, err = filepath.Abs(levelSource)
	if err != nil {
		t.Fatal(err)
	}

	workspace := t.TempDir()
	levelPath := filepath.Join(workspace, "Level.sav")
	if err := system.CopyFile(levelSource, levelPath); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(workspace, "Players"), 0700); err != nil {
		t.Fatal(err)
	}
	before, err := os.ReadFile(levelPath)
	if err != nil {
		t.Fatal(err)
	}

	db, err := bbolt.Open(filepath.Join(workspace, "test.db"), 0600, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if err := db.Update(func(tx *bbolt.Tx) error {
		_, err := tx.CreateBucketIfNotExists([]byte("backups"))
		return err
	}); err != nil {
		t.Fatal(err)
	}
	viper.Reset()
	t.Cleanup(viper.Reset)
	viper.Set("save.path", workspace)
	viper.Set("save.decode_path", decodePath)
	viper.Set("rest.address", "")
	t.Chdir(workspace)

	expectedNickname := os.Getenv("PST_PAL_SAVE_TEST_NICKNAME")
	newNickname := "PST Pal rename test"
	if expectedNickname == newNickname {
		newNickname = "PST Pal rename test 2"
	}
	result, err := RenamePal(db, RenamePalOptions{
		PlayerUID:            playerUID,
		InstanceID:           instanceID,
		ExpectedNickname:     expectedNickname,
		ExpectedLevel:        expectedLevel,
		ExpectedExp:          expectedExp,
		Nickname:             newNickname,
		ConfirmServerStopped: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.Nickname.InstanceID != instanceID ||
		result.Nickname.NicknameBefore != expectedNickname ||
		result.Nickname.NicknameAfter != newNickname ||
		result.Nickname.Level != expectedLevel || result.Nickname.Exp != expectedExp {
		t.Fatalf("unexpected Pal nickname mutation: %#v", result.Nickname)
	}
	if _, err := os.Stat(filepath.Join(workspace, "backups", result.Backup.Path)); err != nil {
		t.Fatalf("tracked backup is missing: %v", err)
	}
	after, err := os.ReadFile(levelPath)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Equal(before, after) {
		t.Fatal("Level.sav was not replaced")
	}

	_, err = RenamePal(db, RenamePalOptions{
		PlayerUID:            playerUID,
		InstanceID:           instanceID,
		ExpectedNickname:     expectedNickname,
		ExpectedLevel:        expectedLevel,
		ExpectedExp:          expectedExp,
		Nickname:             "stale edit",
		ConfirmServerStopped: true,
	})
	if !errors.Is(err, ErrSaveSourceChanged) {
		t.Fatalf("stale Pal nickname should map to ErrSaveSourceChanged, got %v", err)
	}
}

func TestEditPalLevelV037RealSave(t *testing.T) {
	levelSource := os.Getenv("PST_PAL_V037_SAVE_TEST_LEVEL")
	decodePath := os.Getenv("PST_SAVE_TEST_CLI")
	if levelSource == "" || decodePath == "" {
		t.Skip("set PST_PAL_V037_SAVE_TEST_LEVEL and PST_SAVE_TEST_CLI")
	}

	const (
		playerUID        = "ded7515b-0000-0000-0000-000000000000"
		instanceID       = "ff38bdad-4710-966d-982f-3ca7cb107b56"
		expectedNickname = "Raid Jet"
		expectedLevel    = 55
		expectedExp      = int64(6678888)
		expectedHP       = int64(7286000)
		expectedMaxHP    = int64(0)
		newLevel         = 56
	)

	decodePath, err := filepath.Abs(decodePath)
	if err != nil {
		t.Fatal(err)
	}
	levelSource, err = filepath.Abs(levelSource)
	if err != nil {
		t.Fatal(err)
	}
	workspace := t.TempDir()
	levelPath := filepath.Join(workspace, "Level.sav")
	if err := system.CopyFile(levelSource, levelPath); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(workspace, "Players"), 0700); err != nil {
		t.Fatal(err)
	}
	before, err := os.ReadFile(levelPath)
	if err != nil {
		t.Fatal(err)
	}

	db, err := bbolt.Open(filepath.Join(workspace, "test.db"), 0600, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if err := db.Update(func(tx *bbolt.Tx) error {
		_, err := tx.CreateBucketIfNotExists([]byte("backups"))
		return err
	}); err != nil {
		t.Fatal(err)
	}
	viper.Reset()
	t.Cleanup(viper.Reset)
	viper.Set("save.path", workspace)
	viper.Set("save.decode_path", decodePath)
	viper.Set("rest.address", "")
	t.Chdir(workspace)

	options := EditPalLevelOptions{
		PlayerUID:            playerUID,
		InstanceID:           instanceID,
		ExpectedNickname:     expectedNickname,
		ExpectedLevel:        expectedLevel,
		ExpectedExp:          expectedExp,
		ExpectedHP:           expectedHP,
		ExpectedMaxHP:        expectedMaxHP,
		Level:                newLevel,
		ConfirmServerStopped: true,
	}
	result, err := EditPalLevel(db, options)
	if err != nil {
		t.Fatal(err)
	}
	if result.Level.PlayerUID != playerUID ||
		result.Level.InstanceID != instanceID ||
		result.Level.Nickname != expectedNickname ||
		result.Level.LevelBefore != expectedLevel ||
		result.Level.LevelAfter != newLevel ||
		result.Level.ExpBefore != expectedExp ||
		result.Level.ExpAfter < 0 ||
		result.Level.HPBefore != expectedHP ||
		result.Level.HPAfter <= 0 ||
		result.Level.MaxHPBefore != expectedMaxHP ||
		result.Level.MaxHPAfter != result.Level.HPAfter ||
		result.Level.HealthField != "Hp" ||
		!result.Level.MaxHPCreated {
		t.Fatalf("unexpected Pal level mutation: %#v", result.Level)
	}
	if _, err := os.Stat(filepath.Join(workspace, "backups", result.Backup.Path)); err != nil {
		t.Fatalf("tracked backup is missing: %v", err)
	}
	after, err := os.ReadFile(levelPath)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Equal(before, after) {
		t.Fatal("Level.sav was not replaced")
	}

	_, err = EditPalLevel(db, options)
	if !errors.Is(err, ErrSaveSourceChanged) {
		t.Fatalf("stale Pal level should map to ErrSaveSourceChanged, got %v", err)
	}
}

func TestRestorePalHealthLegacyRealSave(t *testing.T) {
	levelSource := os.Getenv("PST_PAL_LEGACY_SAVE_TEST_LEVEL")
	decodePath := os.Getenv("PST_SAVE_TEST_CLI")
	if levelSource == "" || decodePath == "" {
		t.Skip("set PST_PAL_LEGACY_SAVE_TEST_LEVEL to the legacy tests/testdata/Level.sav fixture and PST_SAVE_TEST_CLI")
	}

	const (
		playerUID        = "00000000-0000-0000-0000-000000000001"
		instanceID       = "c410c416-475c-0638-eb35-269338f2a320"
		expectedNickname = ""
		expectedLevel    = 2
		expectedExp      = int64(25)
		expectedHP       = int64(136741)
		expectedMaxHP    = int64(583000)
	)

	decodePath, err := filepath.Abs(decodePath)
	if err != nil {
		t.Fatal(err)
	}
	levelSource, err = filepath.Abs(levelSource)
	if err != nil {
		t.Fatal(err)
	}
	workspace := t.TempDir()
	levelPath := filepath.Join(workspace, "Level.sav")
	if err := system.CopyFile(levelSource, levelPath); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(workspace, "Players"), 0700); err != nil {
		t.Fatal(err)
	}
	before, err := os.ReadFile(levelPath)
	if err != nil {
		t.Fatal(err)
	}

	db, err := bbolt.Open(filepath.Join(workspace, "test.db"), 0600, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if err := db.Update(func(tx *bbolt.Tx) error {
		_, err := tx.CreateBucketIfNotExists([]byte("backups"))
		return err
	}); err != nil {
		t.Fatal(err)
	}
	viper.Reset()
	t.Cleanup(viper.Reset)
	viper.Set("save.path", workspace)
	viper.Set("save.decode_path", decodePath)
	viper.Set("rest.address", "")
	t.Chdir(workspace)

	options := RestorePalHealthOptions{
		PlayerUID:            playerUID,
		InstanceID:           instanceID,
		ExpectedNickname:     expectedNickname,
		ExpectedLevel:        expectedLevel,
		ExpectedExp:          expectedExp,
		ExpectedHP:           expectedHP,
		ExpectedMaxHP:        expectedMaxHP,
		ConfirmServerStopped: true,
	}
	result, err := RestorePalHealth(db, options)
	if err != nil {
		t.Fatal(err)
	}
	if result.Health.PlayerUID != playerUID ||
		result.Health.InstanceID != instanceID ||
		result.Health.PalType != "ChickenPal" ||
		result.Health.Nickname != expectedNickname ||
		result.Health.Level != expectedLevel ||
		result.Health.Exp != expectedExp ||
		result.Health.HPBefore != expectedHP ||
		result.Health.HPAfter != expectedMaxHP ||
		result.Health.MaxHP != expectedMaxHP ||
		result.Health.HealthField != "HP" {
		t.Fatalf("unexpected Pal health mutation: %#v", result.Health)
	}
	backupInfo, err := os.Stat(filepath.Join(workspace, "backups", result.Backup.Path))
	if err != nil {
		t.Fatalf("tracked backup is missing: %v", err)
	}
	if backupInfo.Size() == 0 {
		t.Fatal("tracked backup is empty")
	}
	archive, err := zip.OpenReader(filepath.Join(workspace, "backups", result.Backup.Path))
	if err != nil {
		t.Fatalf("open tracked backup: %v", err)
	}
	defer archive.Close()
	var backupLevel []byte
	for _, file := range archive.File {
		if filepath.Base(file.Name) != "Level.sav" {
			continue
		}
		reader, openErr := file.Open()
		if openErr != nil {
			t.Fatalf("open backed-up Level.sav: %v", openErr)
		}
		backupLevel, err = io.ReadAll(reader)
		closeErr := reader.Close()
		if err != nil {
			t.Fatalf("read backed-up Level.sav: %v", err)
		}
		if closeErr != nil {
			t.Fatalf("close backed-up Level.sav: %v", closeErr)
		}
		break
	}
	if !bytes.Equal(before, backupLevel) {
		t.Fatal("backup does not contain the pre-edit Level.sav")
	}
	after, err := os.ReadFile(levelPath)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Equal(before, after) {
		t.Fatal("Level.sav was not replaced")
	}

	_, err = RestorePalHealth(db, options)
	if !errors.Is(err, ErrSaveSourceChanged) {
		t.Fatalf("stale Pal health should map to ErrSaveSourceChanged, got %v", err)
	}
}
