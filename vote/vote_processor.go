package vote

import (
	"bytes"
	"context"
	"encoding/hex"
	"fmt"
	"github.com/avast/retry-go/v4"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/rlp"
	"github.com/gnfd-challenger/client/rpc"
	gnfdcommon "github.com/gnfd-challenger/common"
	"github.com/gnfd-challenger/config"
	"github.com/gnfd-challenger/db"
	"github.com/gnfd-challenger/db/dao"
	"github.com/gnfd-challenger/db/model"
	"github.com/gnfd-challenger/keys"
	"github.com/prysmaticlabs/prysm/crypto/bls/blst"
	"github.com/tendermint/tendermint/votepool"
	"gorm.io/gorm"
	"time"
)

type GreenfieldVoteProcessor struct {
	votePoolExecutor *VotePoolExecutor
	daoManager       *dao.DaoManager
	config           *config.Config
	signer           *VoteSigner
	greenfieldClient *rpc.GreenfieldChallengerClient
	blsPublicKey     []byte
}

func NewGreenfieldVoteProcessor(cfg *config.Config, dao *dao.DaoManager, signer *VoteSigner, greenfieldClient *rpc.GreenfieldChallengerClient,
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
	latestHeight, err := p.greenfieldClient.GetLatestBlockHeightWithRetry()
	if err != nil {
		gnfdcommon.Logger.Errorf("failed to get latest block height, error: %s", err.Error())
		return err
	}

	leastSavedTxHeight, err := p.daoManager.GreenfieldDao.GetLeastSavedTransactionHeight()
	if err != nil {
		gnfdcommon.Logger.Errorf("failed to get least saved tx, error: %s", err.Error())
		return err
	}
	if leastSavedTxHeight+p.config.GreenfieldConfig.NumberOfBlocksForFinality > latestHeight {
		return nil
	}
	txs, err := p.daoManager.GreenfieldDao.GetTransactionsByStatusAndHeight(db.Saved, leastSavedTxHeight)
	if err != nil {
		gnfdcommon.Logger.Errorf("failed to get transactions at height %d from db, error: %s", leastSavedTxHeight, err.Error())
		return err
	}

	if len(txs) == 0 {
		time.Sleep(RetryInterval)
		return nil
	}

	// for every tx, we are going to sign it and broadcast vote of it.
	for _, tx := range txs {
		v, err := p.constructVoteAndSign(tx)
		if err != nil {
			return err
		}

		// TODO remove testing purpose code
		bs2 := common.Hex2Bytes("0fdb6ed435515cbf4b72558a6f42d881fd99e0eddc719cb5890fbf1ec723bd0c")
		secretKey2, err := blst.SecretKeyFromBytes(bs2)
		if err != nil {
			panic(err)
		}
		eh, _ := p.getEventHash(tx)
		pubKey2 := secretKey2.PublicKey()
		sign2 := secretKey2.Sign(eh).Marshal()

		mockVoteFromRelayer2 := &votepool.Vote{
			PubKey:    pubKey2.Marshal(),
			Signature: sign2,
			EventType: 1,
			EventHash: eh,
		}

		// broadcast v
		if err = retry.Do(func() error {
			err = p.votePoolExecutor.BroadcastVote(mockVoteFromRelayer2)
			err = p.votePoolExecutor.BroadcastVote(v)
			if err != nil {
				return fmt.Errorf("failed to submit vote for event with txhash: %s", tx.TxHash)
			}
			return nil
		}, retry.Context(context.Background()), gnfdcommon.RtyAttem, gnfdcommon.RtyDelay, gnfdcommon.RtyErr); err != nil {
			return err
		}

		// After vote submitted to vote pool, persist vote Data and update the status of tx to 'SELF_VOTED'.
		err = p.daoManager.EventDao.DB.Transaction(func(dbTx *gorm.DB) error {
			err = p.daoManager.GreenfieldDao.UpdateTransactionStatus(tx.Id, db.SelfVoted)
			if err != nil {
				return err
			}
			err = p.daoManager.VoteDao.SaveVote(EntityToDto(v, tx.ChannelId, tx.Sequence, common.Hex2Bytes(tx.PayLoad)))
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
	txs, err := p.daoManager.EventDao.GetTransactionsByStatus(db.SelfVoted)
	if err != nil {
		gnfdcommon.Logger.Errorf("failed to get voted transactions from db, error: %s", err.Error())
		return err
	}
	for _, tx := range txs {
		err := p.prepareEnoughValidVotesForTx(tx)
		if err != nil {
			return err
		}
		err = p.daoManager.GreenfieldDao.UpdateTransactionStatus(tx.Id, db.AllVoted)
		if err != nil {
			return err
		}
	}
	return nil
}

// prepareEnoughValidVotesForTx fetches and validate votes result, store in vote table
func (p *GreenfieldVoteProcessor) prepareEnoughValidVotesForTx(tx *model.EventStartChallenge) error {
	validators, err := p.greenfieldClient.QueryLatestValidators()
	if err != nil {
		return err
	}
	err = p.queryMoreThanTwoThirdVotesForTx(tx, validators)
	if err != nil {
		return err
	}
	return nil
}

// queryMoreThanTwoThirdVotesForTx queries votes from votePool
func (p *GreenfieldVoteProcessor) queryMoreThanTwoThirdVotesForTx(tx *model.EventStartChallenge, validators []stakingtypes.Validator) error {
	triedTimes := 0
	validVotesTotalCount := 1 // assume local vote is valid
	channelId := tx.ChannelId
	seq := tx.Sequence
	localVote, err := p.constructVoteAndSign(tx)
	if err != nil {
		return err
	}
	for {
		// skip current tx if reach the max retry.
		if triedTimes > QueryVotepoolMaxRetry {
			// TODO mark the status to tx to ?
			return nil
		}

		queriedVotes, err := p.votePoolExecutor.QueryVotes(localVote.EventHash, votepool.ToBscCrossChainEvent)
		if err != nil {
			gnfdcommon.Logger.Errorf("encounter error when query votes. will retry.")
			return err
		}
		validVotesCountPerReq := len(queriedVotes)
		if validVotesCountPerReq == 0 {
			continue
		}

		isLocalVoteIncluded := false

		for _, v := range queriedVotes {
			if !p.isVotePubKeyValid(v, validators) {
				gnfdcommon.Logger.Errorf("vote's pub-key %s does not belong to any validator", hex.EncodeToString(v.PubKey[:]))
				validVotesCountPerReq--
				continue
			}

			if err := Verify(v, localVote.EventHash); err != nil {
				gnfdcommon.Logger.Errorf("verify vote's signature failed,  err=%s", err)
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
			exist, err := p.daoManager.VoteDao.IsVoteExist(channelId, seq, hex.EncodeToString(v.PubKey[:]))
			if err != nil {
				return err
			}
			if exist {
				validVotesCountPerReq--
				continue
			}
			// a vote result persisted into DB should be valid, unique.
			err = p.daoManager.VoteDao.SaveVote(EntityToDto(v, channelId, seq, common.Hex2Bytes(tx.PayLoad)))
			if err != nil {
				return err
			}
		}

		validVotesTotalCount += validVotesCountPerReq

		if validVotesTotalCount > len(validators)*2/3 {
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

func (p *GreenfieldVoteProcessor) constructVoteAndSign(tx *model.EventStartChallenge) (*votepool.Vote, error) {
	var v votepool.Vote
	v.EventType = votepool.ToBscCrossChainEvent
	eventHash, err := p.getEventHash(tx)
	if err != nil {
		return nil, err
	}
	p.signer.SignVote(&v, eventHash)
	return &v, nil
}

func (p *GreenfieldVoteProcessor) getEventHash(tx *model.EventStartChallenge) ([]byte, error) {
	b, err := rlp.EncodeToBytes(tx.PayLoad)
	if err != nil {
		return nil, err
	}
	return crypto.Keccak256Hash(b).Bytes(), nil
}

func (p *GreenfieldVoteProcessor) isVotePubKeyValid(v *votepool.Vote, validators []stakingtypes.Validator) bool {
	for _, validator := range validators {
		if bytes.Equal(v.PubKey[:], validator.RelayerBlsKey[:]) {
			return true
		}
	}
	return false
}
