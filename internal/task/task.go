package task

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/zaigie/palworld-server-tool/internal/database"
	"github.com/zaigie/palworld-server-tool/internal/system"

	"github.com/go-co-op/gocron/v2"
	"github.com/spf13/viper"
	"github.com/zaigie/palworld-server-tool/internal/logger"
	"github.com/zaigie/palworld-server-tool/internal/tool"
	"github.com/zaigie/palworld-server-tool/service"
	"go.etcd.io/bbolt"
)

var (
	s         gocron.Scheduler
	savSyncMu sync.Mutex
)

func BackupTask(db *bbolt.DB) {
	release, err := BeginManualServerOperation(nil)
	if err != nil {
		logger.Warnf("Skipping scheduled backup while another maintenance operation is running: %v\n", err)
		return
	}
	defer release()

	logger.Info("Scheduling backup...\n")
	backup, err := tool.BackupAndRecord(db)
	if err != nil {
		logger.Errorf("%v\n", err)
		return
	}
	logger.Infof("Auto backup to %s\n", backup.Path)

	keepDays := viper.GetInt("save.backup_keep_days")
	if keepDays == 0 {
		keepDays = 7
	}
	err = tool.CleanOldBackups(db, keepDays)
	if err != nil {
		logger.Errorf("Failed to clean old backups: %v\n", err)
	}
}

func PlayerSync(db *bbolt.DB) {
	logger.Info("Scheduling Player sync...\n")
	onlinePlayers, err := tool.ShowPlayers()
	if err != nil {
		logger.Errorf("%v\n", err)
		return
	}
	err = service.PutPlayersOnline(db, onlinePlayers)
	if err != nil {
		logger.Errorf("%v\n", err)
		return
	}
	logger.Info("Player sync done\n")

	playerLogging := viper.GetBool("task.player_logging")
	if playerLogging {
		go PlayerLogging(onlinePlayers)
	}

	kickInterval := viper.GetBool("manage.kick_non_whitelist")
	if kickInterval {
		go CheckAndKickPlayers(db, onlinePlayers)
	}
}

func isPlayerWhitelisted(player database.OnlinePlayer, whitelist []database.PlayerW) bool {
	for _, whitelistedPlayer := range whitelist {
		if (player.PlayerUid != "" && player.PlayerUid == whitelistedPlayer.PlayerUID) ||
			(player.UserId != "" && player.UserId == whitelistedPlayer.UserID) ||
			(player.SteamId != "" && player.SteamId == whitelistedPlayer.SteamID) {
			return true
		}
	}
	return false
}

var playerCache map[string]string
var firstPoll = true

func PlayerLogging(players []database.OnlinePlayer) {
	loginMsg := viper.GetString("task.player_login_message")
	logoutMsg := viper.GetString("task.player_logout_message")

	tmp := make(map[string]string, len(players))
	for _, player := range players {
		if player.PlayerUid != "" {
			tmp[player.PlayerUid] = player.Nickname
		}
	}
	if !firstPoll {
		for id, name := range tmp {
			if _, ok := playerCache[id]; !ok {
				BroadcastVariableMessage(loginMsg, name, len(players))
			}
		}
		for id, name := range playerCache {
			if _, ok := tmp[id]; !ok {
				BroadcastVariableMessage(logoutMsg, name, len(players))
			}
		}
	}
	firstPoll = false
	playerCache = tmp
}

func BroadcastVariableMessage(message string, username string, onlineNum int) {
	message = strings.ReplaceAll(message, "{username}", username)
	message = strings.ReplaceAll(message, "{online_num}", strconv.Itoa(onlineNum))
	arr := strings.Split(message, "\n")
	for _, msg := range arr {
		err := tool.Broadcast(msg)
		if err != nil {
			logger.Warnf("Broadcast fail, %s \n", err)
		}
		// 连续发送不知道为啥行会错乱, 只能加点延迟
		time.Sleep(1000 * time.Millisecond)
	}
}

func CheckAndKickPlayers(db *bbolt.DB, players []database.OnlinePlayer) {
	whitelist, err := service.ListWhitelist(db)
	if err != nil {
		logger.Errorf("%v\n", err)
	}
	for _, player := range players {
		if !isPlayerWhitelisted(player, whitelist) {
			identifier := player.UserId
			if identifier == "" && player.SteamId != "" {
				identifier = fmt.Sprintf("steam_%s", player.SteamId)
			}
			if identifier == "" {
				logger.Warnf("Kicked %s fail, user ID is empty \n", player.Nickname)
				continue
			}
			err := tool.KickPlayer(identifier)
			if err != nil {
				logger.Warnf("Kicked %s fail, %s \n", player.Nickname, err)
				continue
			}
			logger.Warnf("Kicked %s successful \n", player.Nickname)
		}
	}
	logger.Info("Check whitelist done\n")
}

func SavSync() {
	release, err := BeginManualServerOperation(nil)
	if err != nil {
		logger.Warnf("Skipping scheduled save sync while another maintenance operation is running: %v\n", err)
		return
	}
	defer release()
	_ = SavSyncNow()
}

func SavSyncNow() error {
	savSyncMu.Lock()
	defer savSyncMu.Unlock()

	logger.Info("Scheduling Sav sync...\n")
	err := tool.Decode(viper.GetString("save.path"))
	if err != nil {
		logger.Errorf("%v\n", err)
		return err
	}
	logger.Info("Sav sync done\n")
	return nil
}

func Schedule(db *bbolt.DB) {
	scheduler := getScheduler()

	playerSyncInterval := time.Duration(viper.GetInt("task.sync_interval"))
	savSyncInterval := time.Duration(viper.GetInt("save.sync_interval"))
	backupInterval := time.Duration(viper.GetInt("save.backup_interval"))

	if playerSyncInterval > 0 {
		go PlayerSync(db)
		_, err := scheduler.NewJob(
			gocron.DurationJob(playerSyncInterval*time.Second),
			gocron.NewTask(PlayerSync, db),
		)
		if err != nil {
			logger.Errorf("%v\n", err)
		}
	}

	if savSyncInterval > 0 {
		go SavSync()
		_, err := scheduler.NewJob(
			gocron.DurationJob(savSyncInterval*time.Second),
			gocron.NewTask(SavSync),
		)
		if err != nil {
			logger.Errorf("%v\n", err)
		}
	}

	if backupInterval > 0 {
		go BackupTask(db)
		_, err := scheduler.NewJob(
			gocron.DurationJob(backupInterval*time.Second),
			gocron.NewTask(BackupTask, db),
		)
		if err != nil {
			logger.Error(err)
		}
	}

	_, err := scheduler.NewJob(
		gocron.DurationJob(300*time.Second),
		gocron.NewTask(system.LimitCacheDir, filepath.Join(os.TempDir(), "palworldsav-"), 5),
	)
	if err != nil {
		logger.Errorf("%v\n", err)
	}

	automation, err := NewAutomationManager(db, scheduler)
	if err != nil {
		logger.Errorf("failed to initialize automation manager: %v\n", err)
	} else {
		SetAutomationManager(automation)
		automation.Start()
	}

	scheduler.Start()
}

func Shutdown() {
	if s == nil {
		return
	}
	err := s.Shutdown()
	if err != nil {
		logger.Errorf("%v\n", err)
	}
	manager, managerErr := GetAutomationManager()
	if managerErr == nil {
		manager.Close()
		SetAutomationManager(nil)
	}
}

func getScheduler() gocron.Scheduler {
	if s == nil {
		scheduler, err := gocron.NewScheduler()
		if err != nil {
			logger.Panicf("failed to initialize scheduler: %v", err)
		}
		s = scheduler
	}
	return s
}
