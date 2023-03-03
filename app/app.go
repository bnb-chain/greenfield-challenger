package app

import (
	"fmt"

	"github.com/bnb-chain/greenfield-challenger/config"
	"github.com/bnb-chain/greenfield-challenger/db/dao"
	"github.com/bnb-chain/greenfield-challenger/db/model"
	"github.com/bnb-chain/greenfield-challenger/executor"
	"github.com/bnb-chain/greenfield-challenger/monitor"
	"github.com/bnb-chain/greenfield-challenger/submitter"
	"github.com/bnb-chain/greenfield-challenger/vote"
	ethcommon "github.com/ethereum/go-ethereum/common"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

type App struct {
	eventMonitor *monitor.Monitor

	heartbeatProcessor *vote.VoteProcessor
	heartbeatSubmitter *submitter.TxSubmitter

	//TODO: verifier
	//TODO: attest
}

func NewApp(cfg *config.Config) *App {
	db, err := gorm.Open(mysql.Open(cfg.DBConfig.DBPath), &gorm.Config{})
	db = db.Debug()
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

	monitor := monitor.NewMonitor(executor, daoManager)

	signer := vote.NewVoteSigner(ethcommon.Hex2Bytes(cfg.VotePoolConfig.BlsPrivateKey))
	votePoolExecutor := vote.NewVotePoolExecutor(cfg)

	//TODO: config interval
	heartbeatProcessorKind := vote.NewHeartbeatKind(daoManager, 100)
	heartbeatProcessor := vote.NewVoteProcessor(cfg, daoManager, signer, executor, votePoolExecutor, heartbeatProcessorKind)

	heartbeatSubmitterKind := submitter.NewHeartbeatKind(daoManager, executor)
	heartbeatSubmitter := submitter.NewTxSubmitter(cfg, executor, votePoolExecutor, heartbeatSubmitterKind)

	return &App{
		eventMonitor:       monitor,
		heartbeatProcessor: heartbeatProcessor,
		heartbeatSubmitter: heartbeatSubmitter,
	}
}

func (a *App) Start() {
	go a.eventMonitor.StartLoop()

	// for heartbeat
	go a.heartbeatProcessor.SignAndBroadcast()
	go a.heartbeatProcessor.CollectVotes()
	a.heartbeatSubmitter.SubmitTransactionLoop()
}
