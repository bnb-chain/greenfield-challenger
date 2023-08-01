package vote

import (
	"fmt"
	"github.com/bnb-chain/greenfield-challenger/metrics"
	"strings"
	"time"

	"github.com/bnb-chain/greenfield-challenger/common"
	"github.com/bnb-chain/greenfield-challenger/config"
	"github.com/bnb-chain/greenfield-challenger/db/model"
	"github.com/bnb-chain/greenfield-challenger/executor"
	"github.com/bnb-chain/greenfield-challenger/logging"
	"github.com/cometbft/cometbft/votepool"
)

type VoteBroadcaster struct {
	config          *config.Config
	signer          *VoteSigner
	executor        *executor.Executor
	blsPublicKey    []byte
	cachedLocalVote map[uint64]*votepool.Vote
	dataProvider    DataProvider
	metricService   *metrics.MetricService
}

func NewVoteBroadcaster(cfg *config.Config, signer *VoteSigner,
	executor *executor.Executor, broadcasterDataProvider DataProvider, metricService *metrics.MetricService,
) *VoteBroadcaster {
	return &VoteBroadcaster{
		config:          cfg,
		signer:          signer,
		executor:        executor,
		dataProvider:    broadcasterDataProvider,
		cachedLocalVote: nil,
		blsPublicKey:    executor.BlsPubKey,
		metricService:   metricService,
	}
}

func (p *VoteBroadcaster) BroadcastVotesLoop() {
	// Event lasts for 300 blocks, 2x for redundancy
	p.cachedLocalVote = make(map[uint64]*votepool.Vote, common.CacheSize)
	broadcastLoopCount := 0
	for {
		currentHeight := p.executor.GetCachedBlockHeight()
		// Ask about this function
		events, err := p.dataProvider.FetchEventsForSelfVote(currentHeight)
		if err != nil {
			logging.Logger.Errorf("vote processor failed to fetch unexpired events to collate votes, err=%+v", err.Error())
			continue
		}
		if len(events) == 0 {
			time.Sleep(RetryInterval)
			continue
		}

		for _, event := range events {
			localVote := p.cachedLocalVote[event.ChallengeId]

			if localVote == nil {
				localVote, err = p.constructVoteAndSign(event)
				if err != nil {
					if strings.Contains(err.Error(), "Duplicate") {
						logging.Logger.Errorf("[non-blocking error] broadcaster was trying to save a duplicated vote after clearing cache for challengeId: %d, err=%+v", event.ChallengeId, err.Error())
					} else {
						logging.Logger.Errorf("broadcaster ran into error trying to construct vote for challengeId: %d, err=%+v", event.ChallengeId, err.Error())
						continue
					}
				}
				p.cachedLocalVote[event.ChallengeId] = localVote
			}

			err = p.broadcastForSingleEvent(localVote, event)
			if err != nil {
				continue
			}
			time.Sleep(50 * time.Millisecond)
		}

		broadcastLoopCount++
		if broadcastLoopCount == common.CacheClearIterations {
			// Clear cachedLocalVote every N loops, preCheck cannot catch events expired in between iterations
			p.cachedLocalVote = make(map[uint64]*votepool.Vote, common.CacheSize)
			broadcastLoopCount = 0
		}

		time.Sleep(RetryInterval)
	}
}

func (p *VoteBroadcaster) broadcastForSingleEvent(localVote *votepool.Vote, event *model.Event) error {
	startTime := time.Now()
	err := p.preCheck(event)
	if err != nil {
		if err.Error() == common.ErrEventExpired.Error() {
			delete(p.cachedLocalVote, event.ChallengeId)
			return err
		}
		return err
	}

	logging.Logger.Infof("broadcaster starting time for challengeId: %d %s", event.ChallengeId, time.Now().Format("15:04:05.000000"))
	err = p.executor.BroadcastVote(localVote)
	if err != nil {
		return fmt.Errorf("failed to broadcast vote for challengeId: %d", event.ChallengeId)
	}
	logging.Logger.Infof("vote broadcasted for challengeId: %d, height: %d", event.ChallengeId, event.Height)

	// Metrics
	elaspedTime := time.Since(startTime)
	p.metricService.SetBroadcastedChallenges(event.ChallengeId)
	p.metricService.SetBroadcasterDuration(elaspedTime)
	return nil
}

func (p *VoteBroadcaster) preCheck(event *model.Event) error {
	currentHeight := p.executor.GetCachedBlockHeight()
	if currentHeight > event.ExpiredHeight {
		logging.Logger.Infof("broadcaster for challengeId: %d has expired. expired height: %d, current height: %d, timestamp: %s", event.ChallengeId, event.ExpiredHeight, currentHeight, time.Now().Format("15:04:05.000000"))
		return common.ErrEventExpired
	}

	return nil
}

func (p *VoteBroadcaster) constructVoteAndSign(event *model.Event) (*votepool.Vote, error) {
	var v votepool.Vote
	v.EventType = votepool.DataAvailabilityChallengeEvent
	eventHash := CalculateEventHash(event, p.config.GreenfieldConfig.ChainIdString)
	p.signer.SignVote(&v, eventHash[:])
	err := p.dataProvider.SaveVoteAndUpdateEventStatus(EntityToDto(&v, event.ChallengeId), event.ChallengeId)
	if err != nil {
		return &v, err
	}
	return &v, nil
}
