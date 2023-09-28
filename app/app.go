package app

import (
	"encoding/json"
	"fmt"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"

	"github.com/bnb-chain/greenfield-challenger/attest"
	"github.com/bnb-chain/greenfield-challenger/config"
	"github.com/bnb-chain/greenfield-challenger/db/dao"
	"github.com/bnb-chain/greenfield-challenger/db/model"
	"github.com/bnb-chain/greenfield-challenger/executor"
	"github.com/bnb-chain/greenfield-challenger/metrics"
	"github.com/bnb-chain/greenfield-challenger/monitor"
	"github.com/bnb-chain/greenfield-challenger/submitter"
	"github.com/bnb-chain/greenfield-challenger/verifier"
	"github.com/bnb-chain/greenfield-challenger/vote"
	"github.com/bnb-chain/greenfield-challenger/wiper"
	"github.com/spf13/viper"
)

type App struct {
	executor        *executor.Executor
	eventMonitor    *monitor.Monitor
	hashVerifier    *verifier.Verifier
	voteCollector   *vote.VoteCollector
	voteBroadcaster *vote.VoteBroadcaster
	voteCollator    *vote.VoteCollator
	txSubmitter     *submitter.TxSubmitter
	attestMonitor   *attest.AttestMonitor
	metricService   *metrics.MetricService
	dbWiper         *wiper.DBWiper
}

func NewApp(cfg *config.Config) *App {
	username := cfg.DBConfig.Username
	password := viper.GetString(config.FlagConfigDbPass)
	if password == "" {
		password = getDBPass(&cfg.DBConfig)
	}

	dbPath := fmt.Sprintf("%s:%s@%s", username, password, cfg.DBConfig.DBPath)

	db, err := gorm.Open(mysql.Open(dbPath), &gorm.Config{})

	// only for debug purpose
	//db = db.Debug()

	if err != nil {
		panic(fmt.Sprintf("open db error, err=%+v", err.Error()))
	}

	dbConfig, err := db.DB()
	if err != nil {
		panic(err)
	}
	dbConfig.SetMaxIdleConns(cfg.DBConfig.MaxIdleConns)
	dbConfig.SetMaxOpenConns(cfg.DBConfig.MaxOpenConns)

	// For clearing database during debugging
	//if cfg.DBConfig.DebugMode {
	//	err = ResetDB(db, &model.Block{}, &model.Event{}, &model.Vote{})
	//	if err != nil {
	//		logging.Logger.Errorf("reset db error, err=%+v", err.Error())
	//	}
	//}

	model.InitBlockTable(db)
	model.InitEventTable(db)
	model.InitVoteTable(db)

	blockDao := dao.NewBlockDao(db)
	eventDao := dao.NewEventDao(db)
	voteDao := dao.NewVoteDao(db)
	daoManager := dao.NewDaoManager(blockDao, eventDao, voteDao)

	executor := executor.NewExecutor(cfg)

	metricService := metrics.NewMetricService(cfg)

	monitorDataHandler := monitor.NewDataHandler(daoManager)
	monitor := monitor.NewMonitor(executor, monitorDataHandler, metricService)

	verifierDataHandler := verifier.NewDataHandler(daoManager)
	hashVerifier := verifier.NewHashVerifier(cfg, executor, verifierDataHandler, metricService)

	signer := vote.NewVoteSigner(executor.BlsPrivKey)
	voteDataHandler := vote.NewDataHandler(daoManager, executor)
	voteCollector := vote.NewVoteCollector(cfg, executor, voteDataHandler, metricService)
	voteBroadcaster := vote.NewVoteBroadcaster(cfg, signer, executor, voteDataHandler, metricService)
	voteCollator := vote.NewVoteCollator(cfg, signer, executor, voteDataHandler, metricService)

	txDataHandler := submitter.NewDataHandler(daoManager, executor)
	txSubmitter := submitter.NewTxSubmitter(cfg, executor, txDataHandler, metricService)

	attestDataHandler := attest.NewDataHandler(daoManager)
	attestMonitor := attest.NewAttestMonitor(executor, attestDataHandler, metricService)

	dbWiper := wiper.NewDBWiper(daoManager)

	return &App{
		executor:        executor,
		eventMonitor:    monitor,
		hashVerifier:    hashVerifier,
		voteCollector:   voteCollector,
		voteBroadcaster: voteBroadcaster,
		voteCollator:    voteCollator,
		attestMonitor:   attestMonitor,
		txSubmitter:     txSubmitter,
		metricService:   metricService,
		dbWiper:         dbWiper,
	}
}

func (a *App) Start() {
	go a.executor.UpdateHeartbeatIntervalLoop()
	go a.executor.CacheValidatorsLoop()
	go a.executor.GetHeightLoop()
	go a.eventMonitor.ListenEventLoop()
	go a.hashVerifier.VerifyHashLoop()
	go a.voteCollector.CollectVotesLoop()
	go a.voteBroadcaster.BroadcastVotesLoop()
	go a.voteCollator.CollateVotesLoop()
	go a.attestMonitor.UpdateAttestedChallengeIdLoop()
	go a.metricService.Start()
	a.txSubmitter.SubmitTransactionLoop()
}

func getDBPass(cfg *config.DBConfig) string {
	if cfg.KeyType == config.KeyTypeAWSPrivateKey {
		result, err := config.GetSecret(cfg.AWSSecretName, cfg.AWSRegion)
		if err != nil {
			panic(err)
		}
		type DBPass struct {
			DbPass string `json:"db_pass"`
		}
		var dbPassword DBPass
		err = json.Unmarshal([]byte(result), &dbPassword)
		if err != nil {
			panic(err)
		}
		return dbPassword.DbPass
	}
	return cfg.Password
}

func ResetDB(db *gorm.DB, models ...interface{}) error {
	for _, model := range models {
		// err := db.Session(&gorm.Session{AllowGlobalUpdate: true}).Delete(model).Error
		err := db.Migrator().DropTable(model)
		if err != nil {
			return fmt.Errorf("reset db error, err=%+v", err)
		}
	}
	return nil
}
