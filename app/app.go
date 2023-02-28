package app

import (
	"fmt"

	"github.com/bnb-chain/greenfield-challenger/config"
	"github.com/bnb-chain/greenfield-challenger/db/dao"
	"github.com/bnb-chain/greenfield-challenger/db/model"
	"github.com/bnb-chain/greenfield-challenger/executor"
	"github.com/bnb-chain/greenfield-challenger/monitor"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

type App struct {
	m *monitor.Monitor

	//TODO: verifier
	//TODO: heartbeat
}

func NewApp(cfg *config.Config) *App {
	db, err := gorm.Open(mysql.Open(cfg.DBConfig.DBPath), &gorm.Config{})
	if err != nil {
		panic(fmt.Sprintf("open db error, err=%s", err.Error()))
	}
	model.InitBlockTable(db)
	model.InitEventTable(db)
	model.InitVoteTable(db)
	model.InitTxTable(db)

	blockDao := dao.NewBlockDao(db)
	eventDao := dao.NewEventDao(db)
	voteDao := dao.NewVoteDao(db)
	daoManager := dao.NewDaoManager(blockDao, eventDao, voteDao)

	executor := executor.NewExecutor(cfg)

	m := monitor.NewMonitor(executor, daoManager)

	return &App{
		m: m,
	}
}

func (a *App) Start() {
	a.m.StartLoop()
}
