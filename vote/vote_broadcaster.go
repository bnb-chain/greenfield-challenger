package vote

import (
	"fmt"
	"strings"
	"time"

	"github.com/bnb-chain/greenfield-challenger/config"
	"github.com/bnb-chain/greenfield-challenger/db/dao"
	"github.com/bnb-chain/greenfield-challenger/db/model"
	"github.com/bnb-chain/greenfield-challenger/executor"
	"github.com/bnb-chain/greenfield-challenger/logging"
	"github.com/tendermint/tendermint/votepool"
)

type VoteBroadcaster struct {
	daoManager         *dao.DaoManager
	config             *config.Config
	signer             *VoteSigner
	executor           *executor.Executor
	blsPublicKey       []byte
	blockHeightChan    chan uint64
	cachedChallengeIds map[uint64]bool
	DataProvider
}

func NewVoteBroadcaster(cfg *config.Config, dao *dao.DaoManager, signer *VoteSigner,
	executor *executor.Executor, kind DataProvider,
) *VoteBroadcaster {
	return &VoteBroadcaster{
		config:             cfg,
		daoManager:         dao,
		signer:             signer,
		executor:           executor,
		DataProvider:       kind,
		blockHeightChan:    nil,
		cachedChallengeIds: nil,
		blsPublicKey:       getBlsPubKeyFromPrivKeyStr(cfg.VotePoolConfig.BlsPrivateKey),
	}
}

func (p *VoteBroadcaster) BroadcastVotesLoop() {
	ticker := time.NewTicker(BroadcastVotesInterval)
	for range ticker.C {
		err := p.broadcastVotes()
		if err != nil {
			time.Sleep(RetryInterval)
		}
	}
}

func (p *VoteBroadcaster) broadcastVotes() error {
	currentHeight := p.executor.GetCachedBlockHeight()
	events, err := p.daoManager.GetUnexpiredEvents(currentHeight)
	if err != nil {
		logging.Logger.Errorf("vote processor failed to fetch unexpired events to collate votes, err=%s", err.Error())
		return err
	}
	if len(events) == 0 {
		time.Sleep(RetryInterval)
		return nil
	}

	for _, event := range events {
		if p.cachedChallengeIds[event.ChallengeId] {
			continue
		}
		localVote, err := p.constructVoteAndSign(event)
		if err != nil {
			logging.Logger.Errorf("broadcaster ran into error trying to construct vote for event %d, err=%s", event.ChallengeId, err.Error())
			return err
		}
		go p.broadcastForSingleEventLoop(localVote, event)
	}
	return nil
}

func (p *VoteBroadcaster) broadcastForSingleEventLoop(localVote *votepool.Vote, event *model.Event) {
	p.cachedChallengeIds[event.ChallengeId] = true

	for {
		err := p.broadcastForSingleEvent(localVote, event)
		if err != nil {
			if strings.Contains(err.Error(), "event expired") {
				break
			}
			time.Sleep(RetryInterval)
		}
	}
}

// fetch unexpired and not selfvoted
func (p *VoteBroadcaster) broadcastForSingleEvent(localVote *votepool.Vote, event *model.Event) error {
	err := p.preCheck(event) // check if event expired to end loop
	if err != nil {
		return err
	}

	err = p.executor.BroadcastVote(localVote)
	if err != nil {
		fmt.Errorf("failed to broadcast vote for event with challengeId: %d", event.ChallengeId)
		return err
	}
	return nil
}

func (p *VoteBroadcaster) preCheck(event *model.Event) error {
	currentHeight := p.executor.GetCachedBlockHeight()
	if currentHeight > event.ExpiredHeight {
		delete(p.cachedChallengeIds, event.ChallengeId)
		return fmt.Errorf("event expired")
	}
	return nil
}

func (p *VoteBroadcaster) constructVoteAndSign(event *model.Event) (*votepool.Vote, error) {
	var v votepool.Vote
	v.EventType = votepool.DataAvailabilityChallengeEvent
	eventHash := CalculateEventHash(event)
	p.signer.SignVote(&v, eventHash[:])
	err := p.daoManager.SaveVoteAndUpdateEvent(EntityToDto(&v), event)
	if err != nil {
		return nil, err
	}
	return &v, nil
}
