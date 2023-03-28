package vote

import (
	"bytes"
	"encoding/hex"
	"time"

	"github.com/bnb-chain/greenfield-challenger/config"
	"github.com/bnb-chain/greenfield-challenger/db/dao"
	"github.com/bnb-chain/greenfield-challenger/db/model"
	"github.com/bnb-chain/greenfield-challenger/executor"
	"github.com/bnb-chain/greenfield-challenger/logging"
	tmtypes "github.com/tendermint/tendermint/types"
)

type VoteCollator struct {
	daoManager        *dao.DaoManager
	config            *config.Config
	signer            *VoteSigner
	executor          *executor.Executor
	blsPublicKey      []byte
	cachedChallengeId map[uint64]bool
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
		blsPublicKey: getBlsPubKeyFromPrivKeyStr(cfg.VotePoolConfig.BlsPrivateKey),
	}
}

func (p *VoteCollator) CollateVotesLoop() {
	for {
		err := p.collateVotes()
		if err != nil {
			time.Sleep(RetryInterval)
		}
	}
}

func (p *VoteCollator) collateVotes() error {
	currentHeight := p.executor.GetCachedBlockHeight()
	events, err := p.FetchEventsForCollate(currentHeight)
	if err != nil {
		logging.Logger.Errorf("vote processor failed to fetch unexpired events to collate votes, err=%s", err.Error())
		return err
	}
	if len(events) == 0 {
		time.Sleep(RetryInterval)
		return nil
	}

	for _, event := range events {
		if p.cachedChallengeId[event.ChallengeId] {
			continue
		}
		go p.collateForSingleEventLoop(event)
	}
	return nil
}

func (p *VoteCollator) collateForSingleEventLoop(event *model.Event) {
	for {
		err := p.collateForSingleEvent(event)
		if err != nil {
			time.Sleep(RetryInterval)
			continue
		}
		break
	}
}

func (p *VoteCollator) collateForSingleEvent(event *model.Event) error {
	p.cachedChallengeId[event.ChallengeId] = true
	err := p.prepareEnoughValidVotesForEvent(event)
	if err != nil {
		return err
	}
	return p.UpdateEventStatus(event.ChallengeId, model.EnoughVotesCollected)
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

// queryMoreThanTwoThirdVotesForEvent queries votes from votePool
func (p *VoteCollator) queryMoreThanTwoThirdVotesForEvent(event *model.Event, validators []*tmtypes.Validator) error {
	triedTimes := 0
	validVotesTotalCount := 1 // assume local vote is valid

	for {
		time.Sleep(RetryInterval) // sleep a while for waiting the vote is p2p-ed in the network
		// skip current tx if reach the max retry.

		eventHash := CalculateEventHash(event)
		queriedVotes, err := p.daoManager.GetVotesByEventHash(eventHash[:])
		if err != nil {
			logging.Logger.Errorf("encounter error when query votes. will retry.")
			return err
		}
		validVotesCountPerReq := len(queriedVotes)

		for _, v := range queriedVotes {
			if !p.isVotePubKeyValid(v, validators) {
				logging.Logger.Errorf("vote's pub-key %s does not belong to any validator", hex.EncodeToString(v.PubKey[:]))
				validVotesCountPerReq--
				continue
			}

			// it is local vote
			if bytes.Equal(v.PubKey[:], p.blsPublicKey) {
				validVotesCountPerReq--
				continue
			}

			if err := verifySignature(v, eventHash); err != nil {
				logging.Logger.Errorf("verify vote's signature failed,  err=%s", err)
				validVotesCountPerReq--
				continue
			}

			// check duplicate, the vote might have been saved in previous request.
			exist, err := p.IsVoteExists(eventHash[:], v.PubKey[:])
			if err != nil {
				logging.Logger.Errorf("vote processor failed to check if vote exists for event %d, err=%s", event.ChallengeId, err.Error())
				return err
			}
			if exist {
				validVotesCountPerReq--
				continue
			}
			// a vote result persisted into DB should be valid, unique.
			err = p.SaveVote(v)
			if err != nil {
				return err
			}
		}

		validVotesTotalCount += validVotesCountPerReq

		if validVotesTotalCount > len(validators)*2/3 {
			return nil
		}

		triedTimes++
		continue
	}
}

func (p *VoteCollator) isVotePubKeyValid(v *model.Vote, validators []*tmtypes.Validator) bool {
	for _, validator := range validators {
		if bytes.Equal(v.PubKey[:], validator.RelayerBlsKey[:]) {
			return true
		}
	}
	return false
}
