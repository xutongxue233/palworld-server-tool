package config

import (
	"strings"

	"github.com/spf13/viper"
	"github.com/zaigie/palworld-server-tool/internal/logger"
)

type Config struct {
	Palworld struct {
		ConfigPath string `mapstructure:"config_path"`
		Control    struct {
			Mode             string   `mapstructure:"mode"`
			Target           string   `mapstructure:"target"`
			Arguments        []string `mapstructure:"arguments"`
			WorkingDirectory string   `mapstructure:"working_directory"`
			Timeout          int      `mapstructure:"timeout"`
		} `mapstructure:"control"`
	} `mapstructure:"palworld"`
	Web struct {
		Password  string `mapstructure:"password"`
		Port      int    `mapstructure:"port"`
		Tls       bool   `mapstructure:"tls"`
		CertPath  string `mapstructure:"cert_path"`
		KeyPath   string `mapstructure:"key_path"`
		PublicUrl string `mapstructure:"public_url"`
	} `mapstructure:"web"`
	Task struct {
		SyncInterval        int    `mapstructure:"sync_interval"`
		PlayerLogging       bool   `mapstructure:"player_logging"`
		PlayerLoginMessage  string `mapstructure:"player_login_message"`
		PlayerLogoutMessage string `mapstructure:"player_logout_message"`
	} `mapstructure:"task"`
	Automation struct {
		Watchdog struct {
			Enabled                bool `mapstructure:"enabled"`
			DesiredRunning         bool `mapstructure:"desired_running"`
			CheckIntervalSeconds   int  `mapstructure:"check_interval_seconds"`
			FailureThreshold       int  `mapstructure:"failure_threshold"`
			RestartCooldownSeconds int  `mapstructure:"restart_cooldown_seconds"`
			MaxRecoveryAttempts    int  `mapstructure:"max_recovery_attempts"`
			StartupGraceSeconds    int  `mapstructure:"startup_grace_seconds"`
		} `mapstructure:"watchdog"`
		Notification struct {
			Enabled             bool     `mapstructure:"enabled"`
			Provider            string   `mapstructure:"provider"`
			WebhookURL          string   `mapstructure:"webhook_url"`
			Secret              string   `mapstructure:"secret"`
			Events              []string `mapstructure:"events"`
			TimeoutSeconds      int      `mapstructure:"timeout_seconds"`
			AllowPrivateNetwork bool     `mapstructure:"allow_private_network"`
		} `mapstructure:"notification"`
	} `mapstructure:"automation"`
	SteamCMD struct {
		Executable string `mapstructure:"executable"`
		InstallDir string `mapstructure:"install_dir"`
		Timeout    int    `mapstructure:"timeout"`
	} `mapstructure:"steamcmd"`
	Rcon struct {
		Address   string `mapstructure:"address"`
		Password  string `mapstructure:"password"`
		UseBase64 bool   `mapstructure:"use_base64"`
		Timeout   int    `mapstructure:"timeout"`
	} `mapstructure:"rcon"`
	Rest struct {
		Address  string `mapstructure:"address"`
		Username string `mapstructure:"username"`
		Password string `mapstructure:"password"`
		Timeout  int    `mapstructure:"timeout"`
	} `mapstructure:"rest"`
	Save struct {
		Path           string `mapstructure:"path"`
		DecodePath     string `mapstructure:"decode_path"`
		SyncInterval   int    `mapstructure:"sync_interval"`
		BackupInterval int    `mapstructure:"backup_interval"`
		BackupKeepDays int    `mapstructure:"backup_keep_days"`
	} `mapstructure:"save"`
	Manage struct {
		KickNonWhitelist bool `mapstructure:"kick_non_whitelist"`
	}
}

func Init(cfgFile string, conf *Config) {
	if cfgFile != "" {
		viper.SetConfigFile(cfgFile)
		viper.SetConfigType("yaml")
	} else {
		viper.AddConfigPath(".")
		viper.SetConfigName("config")
		viper.SetConfigType("yaml")
	}

	err := viper.ReadInConfig()
	if err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); ok {
			logger.Warn("config file not found, try to read from env\n")
		} else {
			logger.Panic("config file was found but another error was produced\n")
		}
	}

	viper.SetDefault("web.port", 8080)

	viper.SetDefault("task.sync_interval", 60)
	viper.SetDefault("automation.watchdog.enabled", false)
	viper.SetDefault("automation.watchdog.desired_running", true)
	viper.SetDefault("automation.watchdog.check_interval_seconds", 30)
	viper.SetDefault("automation.watchdog.failure_threshold", 3)
	viper.SetDefault("automation.watchdog.restart_cooldown_seconds", 120)
	viper.SetDefault("automation.watchdog.max_recovery_attempts", 3)
	viper.SetDefault("automation.watchdog.startup_grace_seconds", 90)
	viper.SetDefault("automation.notification.enabled", false)
	viper.SetDefault("automation.notification.provider", "generic")
	viper.SetDefault("automation.notification.timeout_seconds", 10)
	viper.SetDefault("automation.notification.allow_private_network", false)
	viper.SetDefault("automation.notification.events", []string{
		"task.failed",
		"watchdog.unhealthy",
		"watchdog.recovered",
		"watchdog.recovery_failed",
	})
	viper.SetDefault("steamcmd.timeout", 1800)

	viper.SetDefault("rcon.address", "127.0.0.1:25575")
	viper.SetDefault("rcon.timeout", 5)
	viper.SetDefault("rcon.use_base64", false)

	viper.SetDefault("rest.username", "admin")
	viper.SetDefault("rest.timeout", 5)

	viper.SetDefault("save.sync_interval", 600)
	viper.SetDefault("save.backup_interval", 0)
	viper.SetDefault("save.backup_keep_days", 7)

	viper.SetDefault("palworld.control.mode", "disabled")
	viper.SetDefault("palworld.control.timeout", 120)

	viper.SetEnvPrefix("")
	viper.SetEnvKeyReplacer(strings.NewReplacer(".", "__"))
	viper.AutomaticEnv()

	err = viper.Unmarshal(conf)
	if err != nil {
		logger.Panicf("Unable to decode config into struct, %s", err)
	}
}
