package vote

import (
	"time"

	"github.com/bnb-chain/greenfield-challenger/config"
	"github.com/bnb-chain/greenfield-challenger/db/dao"
	"github.com/bnb-chain/greenfield-challenger/executor"
	"github.com/bnb-chain/greenfield-challenger/logging"
	"github.com/tendermint/tendermint/votepool"
)

type VoteCollector struct {
	daoManager *dao.DaoManager
	config     *config.Config
	executor   *executor.Executor
	DataProvider
}

func NewVoteCollector(cfg *config.Config, dao *dao.DaoManager,
	executor *executor.Executor, kind DataProvider,
) *VoteCollector {
	return &VoteCollector{
		config:       cfg,
		daoManager:   dao,
		executor:     executor,
		DataProvider: kind,
	}
}

func (p *VoteCollector) CollectVotesLoop() {
	ticker := time.NewTicker(CollectVotesInterval)
	for range ticker.C {
		err := p.collectVotes()
		if err != nil {
			time.Sleep(RetryInterval)
		}
	}
}

func (p *VoteCollector) collectVotes() error {
	eventType := votepool.DataAvailabilityChallengeEvent
	queriedVotes, err := p.executor.QueryVotes(eventType)
	if err != nil {
		logging.Logger.Errorf("vote collector failed to query votes, err=%s", err.Error())
		return err
	}

	for _, v := range queriedVotes {
		exists, err := p.DataProvider.IsVoteExists(v.EventHash, v.PubKey)
		if err != nil {
			logging.Logger.Errorf("vote collector ran into an error while checking if vote exists, err=%s", err.Error())
			continue
		}
		if exists {
			continue
		}
		err = p.daoManager.SaveVote(EntityToDto(v))
		if err != nil {
			return err
		}
	}
	return nil
}
