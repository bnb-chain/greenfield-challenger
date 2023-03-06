package app

import (
	"fmt"

	"github.com/bnb-chain/greenfield-challenger/config"
	"github.com/bnb-chain/greenfield-challenger/db/dao"
	"github.com/bnb-chain/greenfield-challenger/db/model"
	"github.com/bnb-chain/greenfield-challenger/executor"
	"github.com/bnb-chain/greenfield-challenger/monitor"
	"github.com/bnb-chain/greenfield-challenger/submitter"
	"github.com/bnb-chain/greenfield-challenger/verifier"
	"github.com/bnb-chain/greenfield-challenger/vote"
	ethcommon "github.com/ethereum/go-ethereum/common"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

type App struct {
	eventMonitor  *monitor.Monitor
	hashVerifier  *verifier.Verifier
	voteProcessor *vote.VoteProcessor
	txSubmitter   *submitter.TxSubmitter
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

	blockDao := dao.NewBlockDao(db)
	eventDao := dao.NewEventDao(db)
	voteDao := dao.NewVoteDao(db)
	daoManager := dao.NewDaoManager(blockDao, eventDao, voteDao)

	executor := executor.NewExecutor(cfg)

	monitor := monitor.NewMonitor(executor, daoManager)

	hashVerifier := verifier.NewGreenfieldHashVerifier(cfg, daoManager, executor,
		cfg.GreenfieldConfig.DeduplicationInterval, cfg.GreenfieldConfig.HeartbeatInterval)

	signer := vote.NewVoteSigner(ethcommon.Hex2Bytes(cfg.VotePoolConfig.BlsPrivateKey))
	votePoolExecutor := vote.NewVotePoolExecutor(cfg)

	voteDataHandler := vote.NewAttestDataProvider(daoManager, cfg.GreenfieldConfig.HeartbeatInterval)
	voteProcessor := vote.NewVoteProcessor(cfg, daoManager, signer, executor, votePoolExecutor, voteDataHandler)

	txDataHandler := submitter.NewDataHandler(daoManager, executor)
	txSubmitter := submitter.NewTxSubmitter(cfg, executor, votePoolExecutor, txDataHandler)

	return &App{
		eventMonitor:  monitor,
		hashVerifier:  hashVerifier,
		voteProcessor: voteProcessor,
		txSubmitter:   txSubmitter,
	}
}

func (a *App) Start() {
	go a.eventMonitor.ListenEventLoop()
	go a.hashVerifier.VerifyHashLoop()
	go a.voteProcessor.SignBroadcastVoteLoop()
	go a.voteProcessor.CollectVotesLoop()
	a.txSubmitter.SubmitTransactionLoop()
}
