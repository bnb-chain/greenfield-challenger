package config

import (
	"cosmossdk.io/math"
	"encoding/json"
	"fmt"
	"os"
)

type Config struct {
	GreenfieldConfig GreenfieldConfig `json:"greenfield_config"`
	LogConfig        LogConfig        `json:"log_config"`
	AlertConfig      AlertConfig      `json:"alert_config"`
	DBConfig         DBConfig         `json:"db_config"`
}

type GreenfieldConfig struct {
	KeyType               string   `json:"key_type"`
	AWSRegion             string   `json:"aws_region"`
	AWSSecretName         string   `json:"aws_secret_name"`
	AWSBlsSecretName      string   `json:"aws_bls_secret_name"`
	PrivateKey            string   `json:"private_key"`
	BlsPrivateKey         string   `json:"bls_private_key"`
	RPCAddrs              []string `json:"rpc_addrs"`
	ChainIdString         string   `json:"chain_id_string"`
	GasLimit              uint64   `json:"gas_limit"`
	FeeAmount             string   `json:"fee_amount"`
	FeeDenom              string   `json:"fee_denom"`
	DeduplicationInterval uint64   `json:"deduplication_interval"`
}

func (cfg *GreenfieldConfig) Validate() {
	if cfg.KeyType == "" {
		panic("key_type should not be empty")
	} else if cfg.KeyType == "aws_private_key" {
		if cfg.AWSRegion == "" {
			panic("aws_region should not be empty")
		}
		if cfg.AWSSecretName == "" {
			panic("aws_secret_name should not be empty")
		}
		if cfg.AWSBlsSecretName == "" {
			panic("aws_bls_secret_name should not be empty")
		}
	} else if cfg.KeyType == "local_private_key" {
		if cfg.PrivateKey == "" {
			panic("private_key should not be empty")
		}
		if cfg.BlsPrivateKey == "" {
			panic("bls_private_key should not be empty")
		}
	} else {
		panic(fmt.Sprintf("key_type %s is not supported", cfg.KeyType))
	}

	if cfg.RPCAddrs == nil || len(cfg.RPCAddrs) == 0 {
		panic("rpc_addrs should not be empty")
	}
	if cfg.ChainIdString == "" {
		panic("chain_id_string should not be empty")
	}
	if cfg.GasLimit == 0 {
		panic("gas_limit should not be 0")
	}
	if cfg.FeeAmount == "" {
		panic("fee_amount should not be empty")
	}
	if cfg.FeeDenom == "" {
		panic("fee_denom should not be empty")
	}
	if cfg.DeduplicationInterval == 0 {
		panic("deduplication_interval should not be 0")
	}
	feeAmount, ok := math.NewIntFromString(cfg.FeeAmount)
	if !ok {
		panic("error converting fee_amount to math.Int")
	}
	if feeAmount.IsNegative() {
		panic("fee_amount should not be negative")
	}
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
	Dialect       string `json:"dialect"`
	DBPath        string `json:"db_path"`
	KeyType       string `json:"key_type"`
	AWSRegion     string `json:"aws_region"`
	AWSSecretName string `json:"aws_secret_name"`
	Password      string `json:"password"`
	Username      string `json:"username"`
	MaxIdleConns  int    `json:"max_idle_conns"`
	MaxOpenConns  int    `json:"max_open_conns"`
	DebugMode     bool   `json:"debug_mode"`
}

func (cfg *DBConfig) Validate() {
	if cfg.Dialect != DBDialectMysql && cfg.Dialect != DBDialectSqlite3 {
		panic(fmt.Sprintf("only %s and %s supported", DBDialectMysql, DBDialectSqlite3))
	}
	if cfg.Username == "" || cfg.DBPath == "" {
		panic("db config is not correct")
	}
}

func (cfg *Config) Validate() {
	cfg.LogConfig.Validate()
	cfg.DBConfig.Validate()
	cfg.GreenfieldConfig.Validate()
}

func ParseConfigFromJson(content string) *Config {
	var config Config
	if err := json.Unmarshal([]byte(content), &config); err != nil {
		panic(err)
	}

	config.Validate()

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
	Identity       string `json:"identity"`
	TelegramBotId  string `json:"telegram_bot_id"`
	TelegramChatId string `json:"telegram_chat_id"`
}
