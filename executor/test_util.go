package executor

import (
	"github.com/bnb-chain/greenfield-relayer/config"
)

func InitTestConfig() *config.Config {
	return config.ParseConfigFromFile("../integrationtest/config/config_test.json")
}

func InitExecutors() (*BSCExecutor, *Executor) {
	cfg := InitTestConfig()
	gnfdExecutor := NewGreenfieldExecutor(cfg)
	bscExecutor := NewBSCExecutor(cfg)
	gnfdExecutor.SetBSCExecutor(bscExecutor)
	bscExecutor.SetGreenfieldExecutor(gnfdExecutor)
	return bscExecutor, gnfdExecutor
}
