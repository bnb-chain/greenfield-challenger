package vote

import (
	"bytes"
	"encoding/hex"
	"github.com/bnb-chain/greenfield-challenger/metrics"
	"sync"
	"time"

	"github.com/bnb-chain/greenfield-challenger/config"
	"github.com/bnb-chain/greenfield-challenger/executor"
	"github.com/bnb-chain/greenfield-challenger/logging"
	tmtypes "github.com/cometbft/cometbft/types"
	"github.com/cometbft/cometbft/votepool"
)

type VoteCollector struct {
	config        *config.Config
	executor      *executor.Executor
	mtx           sync.RWMutex
	dataProvider  DataProvider
	metricService *metrics.MetricService
}

func NewVoteCollector(cfg *config.Config, executor *executor.Executor, collectorDataProvider DataProvider, metricService *metrics.MetricService) *VoteCollector {
	return &VoteCollector{
		config:        cfg,
		executor:      executor,
		mtx:           sync.RWMutex{},
		dataProvider:  collectorDataProvider,
		metricService: metricService,
	}
}

func (p *VoteCollector) CollectVotesLoop() {
	for {
		err := p.collectVotes()
		if err != nil {
			time.Sleep(RetryInterval)
		}
		time.Sleep(CollectVotesInterval)
	}
}

func (p *VoteCollector) collectVotes() error {
	eventType := votepool.DataAvailabilityChallengeEvent
	queriedVotes, err := p.executor.QueryVotes(eventType)
	if err != nil {
		logging.Logger.Errorf("vote collector failed to query votes, err=%+v", err.Error())
		return err
	}
	logging.Logger.Infof("number of votes collected: %d", len(queriedVotes))

	if len(queriedVotes) == 0 {
		time.Sleep(RetryInterval)
		return nil
	}

	validators, err := p.executor.QueryCachedLatestValidators()
	if err != nil {
		logging.Logger.Errorf("vote collector ran into error querying validators, err=%+v", err.Error())
		return err
	}

	for _, v := range queriedVotes {
		exists, err := p.dataProvider.IsVoteExists(hex.EncodeToString(v.EventHash), hex.EncodeToString(v.PubKey))
		if err != nil {
			logging.Logger.Errorf("vote collector ran into an error while checking if vote exists, err=%+v", err.Error())
			continue
		}
		if exists {
			continue
		}

		if !p.isVotePubKeyValid(v, validators) {
			logging.Logger.Errorf("vote's pub-key %s does not belong to any validator", hex.EncodeToString(v.PubKey))
			continue
		}

		if err := verifySignature(v, v.EventHash); err != nil {
			logging.Logger.Errorf("verify vote's signature failed,  err=%+v", err)
			continue
		}

		err = p.dataProvider.SaveVote(EntityToDto(v, uint64(0)))
		if err != nil {
			return err
		}
		logging.Logger.Infof("vote saved: %s", hex.EncodeToString(v.Signature))
	}
	return nil
}

func (p *VoteCollector) isVotePubKeyValid(v *votepool.Vote, validators []*tmtypes.Validator) bool {
	for _, validator := range validators {
		if bytes.Equal(v.PubKey[:], validator.BlsKey[:]) {
			return true
		}
	}
	return false
}
