package vote

import (
	"bytes"
	"context"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"time"

	"github.com/avast/retry-go/v4"
	"github.com/bnb-chain/greenfield-challenger/client/rpc"
	gnfdcommon "github.com/bnb-chain/greenfield-challenger/common"
	"github.com/bnb-chain/greenfield-challenger/config"
	"github.com/bnb-chain/greenfield-challenger/db/dao"
	"github.com/bnb-chain/greenfield-challenger/db/model"
	"github.com/bnb-chain/greenfield-challenger/keys"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/pkg/errors"
	"github.com/prysmaticlabs/prysm/crypto/bls"
	tmtypes "github.com/tendermint/tendermint/types"
	"github.com/tendermint/tendermint/votepool"
	"gorm.io/gorm"
)

type GreenfieldVoteProcessor struct {
	votePoolExecutor *VotePoolExecutor
	daoManager       *dao.DaoManager
	config           *config.Config
	signer           *VoteSigner
	greenfieldClient *rpc.ChallengerClient
	blsPublicKey     []byte
}

func NewGreenfieldVoteProcessor(cfg *config.Config, dao *dao.DaoManager, signer *VoteSigner, greenfieldClient *rpc.ChallengerClient,
	votePoolExecutor *VotePoolExecutor,
) *GreenfieldVoteProcessor {
	return &GreenfieldVoteProcessor{
		config:           cfg,
		daoManager:       dao,
		signer:           signer,
		greenfieldClient: greenfieldClient,
		votePoolExecutor: votePoolExecutor,
		blsPublicKey:     keys.GetBlsPubKeyFromPrivKeyStr(cfg.VotePoolConfig.BlsPrivateKey),
	}
}

// SignAndBroadcast Will sign using the bls private key, broadcast the vote to votepool
func (p *GreenfieldVoteProcessor) SignAndBroadcast() {
	for {
		err := p.signAndBroadcast()
		if err != nil {
			time.Sleep(RetryInterval)
		}
	}
}

func (p *GreenfieldVoteProcessor) signAndBroadcast() error {
	latestBlockHeight, err := p.greenfieldClient.GetLatestBlockHeightWithRetry()
	if err != nil {
		logging.Logger.Errorf("failed to get latest block height, error: %s", err.Error())
		return err
	}

	lowestHeightSavedEvent, err := p.daoManager.EventDao.GetUnprocessedEventWithLowestHeight()
	if err != nil {
		logging.Logger.Errorf("failed to get lowest unprocessed event, error: %s", err.Error())
		return err
	}

	if lowestHeightSavedEvent.Height+p.config.GreenfieldConfig.NumberOfBlocksForFinality > latestBlockHeight {
		return nil
	}

	events, err := p.daoManager.EventDao.GetAllEventsFromHeightWithStatus(lowestHeightSavedEvent.Height, model.Unprocessed)
	if err != nil {
		logging.Logger.Errorf("error retrieving event with lowest challengeId and status=unprocessed", err.Error())
		return err
	}

	if len(events) == 0 {
		time.Sleep(RetryInterval)
		return nil
	}

	// for every tx, we are going to sign it and broadcast vote of it.
	for _, event := range events {
		v, err := p.constructVoteAndSign(event, model.VoteOptChallengeSucceed)
		if err != nil {
			return err
		}

		// broadcast v
		if err = retry.Do(func() error {
			err = p.votePoolExecutor.BroadcastVote(v)
			if err != nil {
				return fmt.Errorf("failed to submit vote for event with challengeId: %d", event.ChallengeId)
			}
			return nil
		}, retry.Context(context.Background()), gnfdcommon.RtyAttem, gnfdcommon.RtyDelay, gnfdcommon.RtyErr); err != nil {
			return err
		}

		// After vote submitted to vote pool, persist vote Data and update the status of tx to 'SELF_VOTED'.
		err = p.daoManager.EventDao.DB.Transaction(func(dbTx *gorm.DB) error {
			err = p.daoManager.EventDao.UpdateEventStatusByChallengeId(event.ChallengeId, model.EventStatusChallengeSucceed)
			if err != nil {
				return err
			}
			err = p.daoManager.VoteDao.SaveVote(EntityToDto(v))
			if err != nil {
				return err
			}
			return nil
		})
		if err != nil {
			return err
		}
	}
	return nil
}

func (p *GreenfieldVoteProcessor) CollectVotes() {
	for {
		err := p.collectVotes()
		if err != nil {
			time.Sleep(RetryInterval)
		}
	}
}

func (p *GreenfieldVoteProcessor) collectVotes() error {
	lowestHeight, err := p.daoManager.EventDao.GetUnprocessedEventWithLowestHeight()
	events, err := p.daoManager.EventDao.GetAllEventsFromHeightWithStatus(lowestHeight.Height, model.Unprocessed)
	if err != nil {
		logging.Logger.Errorf("failed to get voted transactions from db, error: %s", err.Error())
		return err
	}
	for _, event := range events {
		err := p.prepareEnoughValidVotesForTx(event)
		if err != nil {
			return err
		}
		err = p.daoManager.EventDao.UpdateEventStatusByChallengeId(event.ChallengeId, model.EventStatusChallengeSucceed)
		if err != nil {
			return err
		}
	}
	return nil
}

// prepareEnoughValidVotesForTx fetches and validate votes result, store in vote table
func (p *GreenfieldVoteProcessor) prepareEnoughValidVotesForTx(event *model.Event) error {
	validators, err := p.greenfieldClient.QueryValidators()
	if err != nil {
		return err
	}
	err = p.queryMoreThanTwoThirdVotesForTx(event, validators)
	if err != nil {
		return err
	}
	return nil
}

// queryMoreThanTwoThirdVotesForTx queries votes from votePool
func (p *GreenfieldVoteProcessor) queryMoreThanTwoThirdVotesForTx(event *model.Event, validators []*tmtypes.Validator) error {
	triedTimes := 0
	validVotesTotalCount := 1 // assume local vote is valid
	localVote, err := p.constructVoteAndSign(event, model.VoteOptChallengeSucceed)
	if err != nil {
		return err
	}
	for {
		// skip current tx if reach the max retry.
		if triedTimes > QueryVotepoolMaxRetry {
			// TODO mark the status to tx to ?
			return nil
		}

		queriedVotes, err := p.votePoolExecutor.QueryVotes(localVote.EventHash, votepool.DataAvailabilityChallengeEvent)
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

			if err := VerifySignature(v, localVote.EventHash); err != nil {
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
			exist, err := p.daoManager.VoteDao.IsVoteExist(event.ChallengeId)
			if err != nil {
				return err
			}
			if exist {
				validVotesCountPerReq--
				continue
			}
			// a vote result persisted into DB should be valid, unique.
			err = p.daoManager.VoteDao.SaveVote(EntityToDto(v))
			if err != nil {
				return err
			}
		}

		validVotesTotalCount += validVotesCountPerReq

		if validVotesTotalCount > len(validators)*2/3 {
			// Send MsgAttest
			return nil
		}
		if !isLocalVoteIncluded {
			err := p.votePoolExecutor.BroadcastVote(localVote)
			if err != nil {
				return err
			}
		}
		triedTimes++
		continue
	}
}

func (p *GreenfieldVoteProcessor) constructVoteAndSign(event *model.Event, option model.VoteOption) (*votepool.Vote, error) {
	var v votepool.Vote
	v.EventType = votepool.DataAvailabilityChallengeEvent
	eventHash, err := p.getEventHash(event, option)
	if err != nil {
		return nil, err
	}
	p.signer.SignVote(&v, eventHash)
	return &v, nil
}

// changed votepool.Vote -> model.Vote
func (p *GreenfieldVoteProcessor) getEventHash(event *model.Event, option model.VoteOption) ([]byte, error) {
	idBz := make([]byte, 8)
	binary.BigEndian.PutUint64(idBz, event.ChallengeId)
	resultBz := make([]byte, 8)
	binary.BigEndian.PutUint64(resultBz, uint64(option))

	bs := make([]byte, 0)
	bs = append(bs, idBz...)
	bs = append(bs, resultBz...)
	hash := crypto.Keccak256Hash(bs)
	return hash.Bytes(), nil
}

//func (p *GreenfieldVoteProcessor) broadcastVotes() error {

//}

// 1. Query votes
// 2. Query validators
// 3. Validate votes
// 4. Count valid votes
// 5. If 2/3 valid, send MsgAttest

//func (p *GreenfieldVoteProcessor) aggregateVotes(challengeId uint64) error {
//	event, err := p.daoManager.EventDao.GetEventsByChallengeId(challengeId)
//	if err != nil {
//		return err
//	}
//
//	validators, err := p.greenfieldClient.QueryValidators()
//	if err != nil {
//		return err
//	}
//
//	challengeIdBz := make([]byte, 8)
//	binary.BigEndian.PutUint64(challengeIdBz, event.ChallengeId)
//	voteOptBz := make([]byte, 8)
//	binary.BigEndian.PutUint64(voteOptBz, uint64(model.VoteOptChallengeSucceed))
//
//	eventHashBz := make([]byte, 0)
//	eventHashBz = append(eventHashBz, challengeIdBz...)
//	eventHashBz = append(eventHashBz, voteOptBz...)
//	eventHash := crypto.Keccak256Hash(eventHashBz)
//
//	votes, err := p.votePoolExecutor.QueryVotes(eventHash.Bytes(), votepool.DataAvailabilityChallengeEvent)
//	if err != nil {
//		return err
//	}
//
//	// Validate votes
//
//	validatorTwoThird := (len(validators) / 3) * 2
//	if len(votes) > validatorTwoThird {
//		// send MsgAttest
//	}
//
//}

func (p *GreenfieldVoteProcessor) isVotePubKeyValid(v *votepool.Vote, validators []*tmtypes.Validator) bool {
	for _, validator := range validators {
		// Why validate by checking key?
		if bytes.Equal(v.PubKey[:], validator.RelayerBlsKey[:]) {
			return true
		}
	}
	return false
}

func VerifySignature(vote *votepool.Vote, eventHash []byte) error {
	blsPubKey, err := bls.PublicKeyFromBytes(vote.PubKey[:])
	if err != nil {
		return errors.Wrap(err, "convert public key from bytes to bls failed")
	}
	sig, err := bls.SignatureFromBytes(vote.Signature[:])
	if err != nil {
		return errors.Wrap(err, "invalid signature")
	}
	if !sig.Verify(blsPubKey, eventHash[:]) {
		return errors.New("verify bls signature failed.")
	}
	return nil
}
