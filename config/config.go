package config

import (
	"encoding/json"
	"fmt"
	"math/big"
	"os"
)

type Config struct {
	GreenfieldConfig GreenfieldConfig `json:"greenfield_config"`
	VotePoolConfig   VotePoolConfig   `json:"vote_pool_config"`
	LogConfig        LogConfig        `json:"log_config"`
	AdminConfig      AdminConfig      `json:"admin_config"`
	AlertConfig      AlertConfig      `json:"alert_config"`
	DBConfig         DBConfig         `json:"db_config"`
}

type AdminConfig struct {
	ListenAddr string `json:"listen_addr"`
}

func (cfg *AdminConfig) Validate() {
	if cfg.ListenAddr == "" {
		panic("listen address should not be empty")
	}
}

type VotePoolConfig struct {
	RPCAddr       string `json:"rpc_addr"`
	BlsPrivateKey string `json:"bls_private_key"`
}

type GreenfieldConfig struct {
	KeyType               string   `json:"key_type"`
	AWSRegion             string   `json:"aws_region"`
	AWSSecretName         string   `json:"aws_secret_name"`
	RPCAddrs              []string `json:"rpc_addrs"`
	GRPCAddrs             []string `json:"grpc_addrs"`
	PrivateKey            string   `json:"private_key"`
	GasLimit              uint64   `json:"gas_limit"`
	ChainIdString         string   `json:"chain_id_string"`
	DeduplicationInterval uint64   `json:"deduplication_interval"`
	HeartbeatInterval     uint64   `json:"heartbeat_interval"`
}

type LogConfig struct {
	Level                        string `json:"level"`
	Filename                     string `json:"filename"`
	MaxFileSizeInMB              int    `json:"max_file_size_in_mb"`
	MaxBackupsOfLogFiles         int    `json:"max_backups_of_log_files"`
	MaxAgeToRetainLogFilesInDays int    `json:"max_age_to_retain_log_files_in_days"`
	UseConsoleLogger             bool   `json:"use_console_logger"`
	UseFileLogger                bool   `json:"use_file_logger"`
	Compress                     bool   `json:"compress"`
}

func (cfg *LogConfig) Validate() {
	if cfg.UseFileLogger {
		if cfg.Filename == "" {
			panic("filename should not be empty if use file logger")
		}
		if cfg.MaxFileSizeInMB <= 0 {
			panic("max_file_size_in_mb should be larger than 0 if use file logger")
		}
		if cfg.MaxBackupsOfLogFiles <= 0 {
			panic("max_backups_off_log_files should be larger than 0 if use file logger")
		}
	}
}

type DBConfig struct {
	Dialect string `json:"dialect"`
	DBPath  string `json:"db_path"`
}

func (cfg *DBConfig) Validate() {
	if cfg.Dialect != DBDialectMysql && cfg.Dialect != DBDialectSqlite3 {
		panic(fmt.Sprintf("only %s and %s supported", DBDialectMysql, DBDialectSqlite3))
	}
	if cfg.DBPath == "" {
		panic("db path should not be empty")
	}
}

func (cfg *Config) Validate() {
	cfg.AdminConfig.Validate()
	cfg.LogConfig.Validate()
	cfg.DBConfig.Validate()
}

func ParseConfigFromJson(content string) *Config {
	var config Config
	if err := json.Unmarshal([]byte(content), &config); err != nil {
		panic(err)
	}
	return &config
}

func ParseConfigFromFile(filePath string) *Config {
	bz, err := os.ReadFile(filePath)
	if err != nil {
		panic(err)
	}

	var config Config
	if err := json.Unmarshal(bz, &config); err != nil {
		panic(err)
	}

	config.Validate()

	return &config
}

type AlertConfig struct {
	EnableAlert     bool  `json:"enable_alert"`
	EnableHeartBeat bool  `json:"enable_heart_beat"`
	Interval        int64 `json:"interval"`

	Identity       string `json:"identity"`
	TelegramBotId  string `json:"telegram_bot_id"`
	TelegramChatId string `json:"telegram_chat_id"`

	BalanceThreshold     string `json:"balance_threshold"`
	SequenceGapThreshold uint64 `json:"sequence_gap_threshold"`
}

func (cfg *AlertConfig) Validate() {
	if !cfg.EnableAlert {
		return
	}
	if cfg.Interval <= 0 {
		panic("alert interval should be positive")
	}
	balanceThreshold, ok := big.NewInt(1).SetString(cfg.BalanceThreshold, 10)
	if !ok {
		panic("unrecognized balance_threshold")
	}

	if balanceThreshold.Cmp(big.NewInt(0)) <= 0 {
		panic("balance_threshold should be positive")
	}

	if cfg.SequenceGapThreshold <= 0 {
		panic("sequence_gap_threshold should be positive")
	}
}
