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
	executor        *executor.Executor
	heightTracker   *vote.HeightTracker
	eventMonitor    *monitor.Monitor
	hashVerifier    *verifier.Verifier
	voteCollector   *vote.VoteCollector
	voteBroadcaster *vote.VoteBroadcaster
	voteCollator    *vote.VoteCollator
	txSubmitter     *submitter.TxSubmitter
}

func NewApp(cfg *config.Config) *App {
	db, err := gorm.Open(mysql.Open(cfg.DBConfig.DBPath), &gorm.Config{})
	// db = db.Debug() only for debug purpose
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

	heightTracker := vote.NewHeightTracker(executor)
	monitor := monitor.NewMonitor(executor, daoManager)

	hashVerifier := verifier.NewHashVerifier(cfg, daoManager, executor, cfg.GreenfieldConfig.DeduplicationInterval)

	signer := vote.NewVoteSigner(ethcommon.Hex2Bytes(cfg.VotePoolConfig.BlsPrivateKey))
	voteDataHandler := vote.NewDataHandler(daoManager, executor)
	voteCollector := vote.NewVoteCollector(cfg, daoManager, executor, voteDataHandler)
	voteBroadcaster := vote.NewVoteBroadcaster(cfg, daoManager, signer, executor, voteDataHandler)
	voteCollator := vote.NewVoteCollator(cfg, daoManager, signer, executor, voteDataHandler)

	txDataHandler := submitter.NewDataHandler(daoManager, executor)
	txSubmitter := submitter.NewTxSubmitter(cfg, executor, daoManager, txDataHandler)

	return &App{
		executor:        executor,
		heightTracker:   heightTracker,
		eventMonitor:    monitor,
		hashVerifier:    hashVerifier,
		voteCollector:   voteCollector,
		voteBroadcaster: voteBroadcaster,
		voteCollator:    voteCollator,
		txSubmitter:     txSubmitter,
	}
}

func (a *App) Start() {
	go a.executor.UpdateAttestedChallengeIdLoop()
	go a.executor.UpdateHeartbeatIntervalLoop()
	go a.executor.CacheValidatorsLoop()
	go a.heightTracker.GetHeightLoop()
	go a.eventMonitor.ListenEventLoop()
	go a.hashVerifier.VerifyHashLoop()
	go a.voteCollector.CollectVotesLoop()
	go a.voteBroadcaster.BroadcastVotesLoop()
	go a.voteCollator.CollateVotesLoop()
	a.txSubmitter.SubmitTransactionLoop()
}
