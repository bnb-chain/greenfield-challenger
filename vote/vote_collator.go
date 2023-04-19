package vote

import (
	"encoding/hex"
	"time"

	"github.com/bnb-chain/greenfield-challenger/common"

	"github.com/bnb-chain/greenfield-challenger/config"
	"github.com/bnb-chain/greenfield-challenger/db/dao"
	"github.com/bnb-chain/greenfield-challenger/db/model"
	"github.com/bnb-chain/greenfield-challenger/executor"
	"github.com/bnb-chain/greenfield-challenger/logging"
	tmtypes "github.com/tendermint/tendermint/types"
)

type VoteCollator struct {
	daoManager   *dao.DaoManager
	config       *config.Config
	signer       *VoteSigner
	executor     *executor.Executor
	blsPublicKey []byte
	DataProvider
}

func NewVoteCollator(cfg *config.Config, dao *dao.DaoManager, signer *VoteSigner,
	executor *executor.Executor, kind DataProvider,
) *VoteCollator {
	return &VoteCollator{
		config:       cfg,
		daoManager:   dao,
		signer:       signer,
		executor:     executor,
		DataProvider: kind,
		blsPublicKey: executor.BlsPubKey,
	}
}

func (p *VoteCollator) CollateVotesLoop() {
	for {
		currentHeight := p.executor.GetCachedBlockHeight()
		events, err := p.FetchEventsForCollate(currentHeight)
		if err != nil {
			logging.Logger.Errorf("vote processor failed to fetch unexpired events to collate votes, err=%+v", err.Error())
			time.Sleep(RetryInterval)
			continue
		}
		if len(events) == 0 {
			time.Sleep(RetryInterval)
			continue
		}

		for _, event := range events {
			err = p.collateForSingleEvent(event)
			if err != nil {
				logging.Logger.Errorf("collator failed to collate for event, err%s", err.Error())
				time.Sleep(RetryInterval)
				continue
			}
			time.Sleep(50 * time.Millisecond)
		}
		time.Sleep(RetryInterval)
	}
}

func (p *VoteCollator) collateForSingleEvent(event *model.Event) error {
	err := p.preCheck(event)
	if err != nil {
		return err
	}
	err = p.prepareEnoughValidVotesForEvent(event)
	if err != nil {
		return err
	}
	err = p.UpdateEventStatus(event.ChallengeId, model.EnoughVotesCollected)
	if err != nil {
		return err
	}
	logging.Logger.Infof("collater completed time for challengeId: %d %s", event.ChallengeId, time.Now().Format("15:04:05.000000"))
	return nil
}

// prepareEnoughValidVotesForEvent fetches and validate votes result, store in vote table
func (p *VoteCollator) prepareEnoughValidVotesForEvent(event *model.Event) error {
	validators, err := p.executor.QueryCachedLatestValidators()
	if err != nil {
		return err
	}
	if len(validators) == 1 {
		return nil
	}
	err = p.queryMoreThanTwoThirdVotesForEvent(event, validators)
	if err != nil {
		return err
	}
	return nil
}

func (p *VoteCollator) preCheck(event *model.Event) error {
	currentHeight := p.executor.GetCachedBlockHeight()
	if currentHeight > event.ExpiredHeight {
		logging.Logger.Infof("collator for challengeId: %d has expired. expired height: %d, current height: %d, timestamp: %s", event.ChallengeId, event.ExpiredHeight, currentHeight, time.Now().Format("15:04:05.000000"))
		return common.ErrEventExpired
	}

	return nil
}

// queryMoreThanTwoThirdVotesForEvent queries votes from votePool
func (p *VoteCollator) queryMoreThanTwoThirdVotesForEvent(event *model.Event, validators []*tmtypes.Validator) error {
	for {
		err := p.preCheck(event)
		if err != nil {
			if err.Error() == common.ErrEventExpired.Error() {
				return err
			}
			return err
		}
		eventHash := CalculateEventHash(event)
		queriedVotes, err := p.daoManager.GetVotesByEventHash(hex.EncodeToString(eventHash))
		if err != nil {
			logging.Logger.Errorf("encounter error when query votes. will retry.")
			time.Sleep(RetryInterval)
			continue
		}
		logging.Logger.Infof("collating for challengeId: %d vote count %d, timestamp %s", event.ChallengeId, len(queriedVotes), time.Now().Format("15:04:05.000000"))
		if len(queriedVotes) > len(validators)*2/3 {
			return nil
		}
		time.Sleep(RetryInterval)
		continue
	}
}
