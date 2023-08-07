package vote

import (
	"encoding/hex"
	"fmt"
	"time"

	"github.com/bnb-chain/greenfield-challenger/common"
	"github.com/bnb-chain/greenfield-challenger/config"
	"github.com/bnb-chain/greenfield-challenger/db/model"
	"github.com/bnb-chain/greenfield-challenger/executor"
	"github.com/bnb-chain/greenfield-challenger/logging"
	"github.com/bnb-chain/greenfield-challenger/metrics"
	tmtypes "github.com/cometbft/cometbft/types"
)

type VoteCollator struct {
	config        *config.Config
	signer        *VoteSigner
	executor      *executor.Executor
	blsPublicKey  []byte
	dataProvider  DataProvider
	metricService *metrics.MetricService
}

func NewVoteCollator(cfg *config.Config, signer *VoteSigner,
	executor *executor.Executor, collatorDataProvider DataProvider, metricService *metrics.MetricService,
) *VoteCollator {
	return &VoteCollator{
		config:        cfg,
		signer:        signer,
		executor:      executor,
		dataProvider:  collatorDataProvider,
		blsPublicKey:  executor.BlsPubKey,
		metricService: metricService,
	}
}

func (p *VoteCollator) CollateVotesLoop() {
	for {
		currentHeight := p.executor.GetCachedBlockHeight()
		events, err := p.dataProvider.FetchEventsForCollate(currentHeight)
		logging.Logger.Infof("vote processor fetched %d events for collate", len(events))
		if err != nil {
			p.metricService.IncCollatorErr()
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
	startTime := time.Now()

	err = p.prepareEnoughValidVotesForEvent(event)
	if err != nil {
		return err
	}
	err = p.dataProvider.UpdateEventStatus(event.ChallengeId, model.EnoughVotesCollected)
	if err != nil {
		p.metricService.IncCollatorErr()
		return err
	}

	elaspedTime := time.Since(startTime)
	p.metricService.SetCollatorDuration(elaspedTime)
	p.metricService.IncCollatedChallenges()
	logging.Logger.Infof("collator metrics increased for challengeId %d", event.ChallengeId)
	logging.Logger.Infof("collator completed time for challengeId: %d %s", event.ChallengeId, time.Now().Format("15:04:05.000000"))
	return nil
}

// prepareEnoughValidVotesForEvent fetches and validate votes result, store in vote table
func (p *VoteCollator) prepareEnoughValidVotesForEvent(event *model.Event) error {
	validators, err := p.executor.QueryCachedLatestValidators()
	if err != nil {
		p.metricService.IncCollatorErr()
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
	err := p.preCheck(event)
	if err != nil {
		return err
	}
	eventHash := CalculateEventHash(event, p.config.GreenfieldConfig.ChainIdString)
	queriedVotes, err := p.dataProvider.FetchVotesForCollate(hex.EncodeToString(eventHash))
	if err != nil {
		p.metricService.IncCollatorErr()
		logging.Logger.Errorf("failed to query votes for event %d, err=%+v", event.ChallengeId, err.Error())
		return err
	}
	logging.Logger.Infof("collating for challengeId: %d vote count %d, timestamp %s", event.ChallengeId, len(queriedVotes), time.Now().Format("15:04:05.000000"))
	if len(queriedVotes) > len(validators)*2/3 {
		return nil
	}
	time.Sleep(RetryInterval)
	return fmt.Errorf("failed to query enough votes for event %d", event.ChallengeId)
}
