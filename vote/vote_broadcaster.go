package vote

import (
	"fmt"
	lru "github.com/hashicorp/golang-lru"
	"strings"
	"time"

	"github.com/bnb-chain/greenfield-challenger/metrics"

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
	cachedLocalVote *lru.Cache
	dataProvider    DataProvider
	metricService   *metrics.MetricService
}

func NewVoteBroadcaster(cfg *config.Config, signer *VoteSigner,
	executor *executor.Executor, broadcasterDataProvider DataProvider, metricService *metrics.MetricService,
) *VoteBroadcaster {
	cacheSize := 1000
	lruCache, _ := lru.New(cacheSize)

	return &VoteBroadcaster{
		config:          cfg,
		signer:          signer,
		executor:        executor,
		dataProvider:    broadcasterDataProvider,
		cachedLocalVote: lruCache,
		blsPublicKey:    executor.BlsPubKey,
		metricService:   metricService,
	}
}

func (p *VoteBroadcaster) BroadcastVotesLoop() {
	for {
		currentHeight := p.executor.GetCachedBlockHeight()
		events, heartbeatEventCount, err := p.dataProvider.FetchEventsForSelfVote(currentHeight)
		if err != nil {
			p.metricService.IncBroadcasterErr(err)
			logging.Logger.Errorf("vote processor failed to fetch unexpired events to collate votes, err=%+v", err.Error())
			continue
		}
		if len(events) == 0 {
			time.Sleep(RetryInterval)
			continue
		}
		if heartbeatEventCount != 0 {
			for i := uint64(0); i < heartbeatEventCount; i++ {
				p.metricService.IncHeartbeatEvents()
			}
		}

		for _, event := range events {
			localVote, found := p.cachedLocalVote.Get(event.ChallengeId)

			if !found {
				localVote, err = p.constructVoteAndSign(event)
				if err != nil {
					if strings.Contains(err.Error(), "Duplicate") {
						logging.Logger.Errorf("[non-blocking error] broadcaster was trying to save a duplicated vote after clearing cache for challengeId: %d, err=%+v", event.ChallengeId, err.Error())
					} else {
						p.metricService.IncBroadcasterErr(err)
						logging.Logger.Errorf("broadcaster ran into error trying to construct vote for challengeId: %d, err=%+v", event.ChallengeId, err.Error())
						continue
					}
				}
				p.cachedLocalVote.Add(event.ChallengeId, localVote)
				// Incrementing this before broadcasting to prevent the same challengeID from being incremented multiple times
				// does not mean that it has been successfully broadcasted, check error metrics for broadcast errors.
				p.metricService.IncBroadcastedChallenges()
				logging.Logger.Infof("broadcaster metrics increased for challengeId %d", event.ChallengeId)
			}

			err = p.broadcastForSingleEvent(localVote.(*votepool.Vote), event)
			if err != nil {
				continue
			}
			time.Sleep(50 * time.Millisecond)
		}

		time.Sleep(RetryInterval)
	}
}

func (p *VoteBroadcaster) broadcastForSingleEvent(localVote *votepool.Vote, event *model.Event) error {
	startTime := time.Now()
	err := p.preCheck(event)
	if err != nil {
		if err.Error() == common.ErrEventExpired.Error() {
			p.cachedLocalVote.Remove(event.ChallengeId)
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
