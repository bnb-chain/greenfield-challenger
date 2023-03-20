package vote

import (
	"bytes"
	"context"
	"encoding/hex"
	"fmt"
	"github.com/avast/retry-go/v4"
	"time"

	"github.com/bnb-chain/greenfield-challenger/alert"

	"github.com/bnb-chain/greenfield-challenger/common"
	"github.com/bnb-chain/greenfield-challenger/config"
	"github.com/bnb-chain/greenfield-challenger/db/dao"
	"github.com/bnb-chain/greenfield-challenger/db/model"
	"github.com/bnb-chain/greenfield-challenger/executor"
	"github.com/bnb-chain/greenfield-challenger/logging"
	tmtypes "github.com/tendermint/tendermint/types"
	"github.com/tendermint/tendermint/votepool"
	"gorm.io/gorm"
)

type VoteProcessor struct {
	daoManager   *dao.DaoManager
	config       *config.Config
	signer       *VoteSigner
	executor     *executor.Executor
	blsPublicKey []byte
	DataProvider
}

func NewVoteProcessor(cfg *config.Config, dao *dao.DaoManager, signer *VoteSigner,
	executor *executor.Executor, kind DataProvider,
) *VoteProcessor {
	return &VoteProcessor{
		config:       cfg,
		daoManager:   dao,
		signer:       signer,
		executor:     executor,
		DataProvider: kind,
		blsPublicKey: getBlsPubKeyFromPrivKeyStr(cfg.VotePoolConfig.BlsPrivateKey),
	}
}

// SignBroadcastVoteLoop Will sign using the bls private key, broadcast the vote to votepool
func (p *VoteProcessor) SignBroadcastVoteLoop() {
	for {
		err := p.signAndBroadcast()
		if err != nil {
			time.Sleep(RetryInterval)
		}
	}
}

func (p *VoteProcessor) signAndBroadcast() error {
	events, err := p.FetchEventsForSelfVote()
	if err != nil {
		return err
	}
	if len(events) == 0 {
		time.Sleep(RetryInterval)
		return nil
	}

	for _, event := range events {
		err = p.signForSingleEvent(event)
		if err != nil {
			return err
		}
	}
	return nil
}

func (p *VoteProcessor) signForSingleEvent(event *model.Event) error {
	if err := p.preCheck(event); err != nil {
		return err
	}

	v, err := p.constructVoteAndSign(event)
	if err != nil {
		return err
	}

	// broadcast v
	if err = retry.Do(func() error {
		err = p.executor.BroadcastVote(v)
		if err != nil {
			return fmt.Errorf("failed to submit vote for event with challengeId: %d", event.ChallengeId)
		}
		return nil
	}, retry.Context(context.Background()), common.RtyAttem, common.RtyDelay, common.RtyErr); err != nil {
		return err
	}

	// After vote submitted to vote pool, persist vote Data and update the status of event to 'SELF_VOTED'.
	err = p.daoManager.EventDao.DB.Transaction(func(dbTx *gorm.DB) error {
		err = p.UpdateEventStatus(event.ChallengeId, model.SelfVoted)
		if err != nil {
			return err
		}
		err = p.SaveVote(EntityToDto(v, event.ChallengeId))
		if err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		return err
	}

	return nil
}

func (p *VoteProcessor) preCheck(event *model.Event) error {
	// event will be skipped if
	// 1) the challenge with bigger id has been attested
	attestedId, err := p.executor.QueryLatestAttestedChallengeId()
	if err != nil {
		return err
	}
	if attestedId <= event.ChallengeId {
		logging.Logger.Infof("voter skips the event %d, attested id=%d", event.ChallengeId, attestedId)
		return p.daoManager.UpdateEventStatusByChallengeId(event.ChallengeId, model.Skipped)
	}
	return nil
}

func (p *VoteProcessor) CollectVotesLoop() {
	for {
		err := p.collectVotes()
		if err != nil {
			time.Sleep(RetryInterval)
		}
	}
}

func (p *VoteProcessor) collectVotes() error {
	events, err := p.FetchEventsForCollectVotes()
	if err != nil {
		logging.Logger.Errorf("vote processor failed to fetch events to collect votes, err=%s", err.Error())
		return err
	}
	if len(events) == 0 {
		time.Sleep(RetryInterval)
		return nil
	}

	for _, event := range events {
		err = p.collectForSingleEvent(event)
		if err != nil {
			return err
		}
	}

	return nil
}

func (p *VoteProcessor) collectForSingleEvent(event *model.Event) error {
	err := p.prepareEnoughValidVotesForEvent(event)
	if err != nil {
		return err
	}
	return p.UpdateEventStatus(event.ChallengeId, model.EnoughVotesCollected)
}

// prepareEnoughValidVotesForEvent fetches and validate votes result, store in vote table
func (p *VoteProcessor) prepareEnoughValidVotesForEvent(event *model.Event) error {
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
func (p *VoteProcessor) queryMoreThanTwoThirdVotesForEvent(event *model.Event, validators []*tmtypes.Validator) error {
	triedTimes := 0
	validVotesTotalCount := 1 // assume local vote is valid
	localVote, err := p.constructVoteAndSign(event)
	if err != nil {
		logging.Logger.Errorf("vote processor failed to construct vote and sign for event %d, err=%s", event.ChallengeId, err.Error())
		return err
	}
	for {
		// skip current tx if reach the max retry.
		if triedTimes > QueryVotepoolMaxRetry {
			if p.config.AlertConfig.EnableAlert {
				alert.SendTelegramMessage(p.config.AlertConfig.Identity, p.config.AlertConfig.TelegramChatId, p.config.AlertConfig.TelegramBotId, fmt.Sprintf("failed to collect votes for challenge after retry, id: %d", event.ChallengeId))
			}
			logging.Logger.Infof("failed to collect votes for challenge after retry, id: %d", event.ChallengeId)
			return p.UpdateEventStatus(event.ChallengeId, model.NoEnoughVotesCollected)
		}

		queriedVotes, err := p.executor.QueryVotes(localVote.EventHash, votepool.DataAvailabilityChallengeEvent)
		if err != nil {
			logging.Logger.Errorf("encounter error when query votes. will retry.")
			return err
		}
		validVotesCountPerReq := len(queriedVotes)
		if validVotesCountPerReq == 0 {
			continue
		}

		isLocalVoteIncluded := false

		for _, v := range queriedVotes {
			if !p.isVotePubKeyValid(v, validators) {
				logging.Logger.Errorf("vote's pub-key %s does not belong to any validator", hex.EncodeToString(v.PubKey[:]))
				validVotesCountPerReq--
				continue
			}

			if err := verifySignature(v, localVote.EventHash); err != nil {
				logging.Logger.Errorf("verify vote's signature failed,  err=%s", err)
				validVotesCountPerReq--
				continue
			}

			// it is local vote
			if bytes.Equal(v.PubKey[:], p.blsPublicKey) {
				isLocalVoteIncluded = true
				validVotesCountPerReq--
				continue
			}

			// check duplicate, the vote might have been saved in previous request.
			exist, err := p.IsVoteExists(event.ChallengeId, hex.EncodeToString(v.PubKey[:]))
			if err != nil {
				logging.Logger.Errorf("vote processor failed to check if vote exists for event %d, err=%s", event.ChallengeId, err.Error())
				return err
			}
			if exist {
				validVotesCountPerReq--
				continue
			}
			// a vote result persisted into DB should be valid, unique.
			err = p.SaveVote(EntityToDto(v, event.ChallengeId))
			if err != nil {
				return err
			}
		}

		validVotesTotalCount += validVotesCountPerReq

		if validVotesTotalCount > len(validators)*2/3 {
			return nil
		}
		if !isLocalVoteIncluded {
			err := p.executor.BroadcastVote(localVote)
			if err != nil {
				return err
			}
		}
		triedTimes++
		continue
	}
}

func (p *VoteProcessor) constructVoteAndSign(event *model.Event) (*votepool.Vote, error) {
	var v votepool.Vote
	v.EventType = votepool.DataAvailabilityChallengeEvent
	eventHash := p.CalculateEventHash(event)
	p.signer.SignVote(&v, eventHash[:])
	return &v, nil
}

func (p *VoteProcessor) isVotePubKeyValid(v *votepool.Vote, validators []*tmtypes.Validator) bool {
	for _, validator := range validators {
		if bytes.Equal(v.PubKey[:], validator.RelayerBlsKey[:]) {
			return true
		}
	}
	return false
}
