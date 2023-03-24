package submitter

import (
	sdkmath "cosmossdk.io/math"
	"encoding/binary"
	"github.com/bnb-chain/greenfield-challenger/db/dao"
	"github.com/bnb-chain/greenfield-challenger/db/model"
	"github.com/bnb-chain/greenfield-challenger/executor"
	challengetypes "github.com/bnb-chain/greenfield/x/challenge/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/willf/bitset"
)

const batchSize = 10

type DataProvider interface {
	FetchEventsForSubmit() ([]*model.Event, error)
	FetchVotesForAggregation(eventHash []byte) ([]*model.Vote, error)
	UpdateEventStatus(challengeId uint64, status model.EventStatus) error
	SubmitTx(event *model.Event, validatorSet *bitset.BitSet, aggSignature []byte) (string, error)
	CalculateEventHash(*model.Event) []byte
}

type DataHandler struct {
	daoManager *dao.DaoManager
	executor   *executor.Executor
}

func NewDataHandler(daoManager *dao.DaoManager, executor *executor.Executor) *DataHandler {
	return &DataHandler{
		daoManager: daoManager,
		executor:   executor,
	}
}

func (h *DataHandler) CalculateEventHash(event *model.Event) []byte {
	challengeIdBz := make([]byte, 8)
	binary.BigEndian.PutUint64(challengeIdBz, event.ChallengeId)
	objectIdBz := sdkmath.NewUintFromString(event.ObjectId).Bytes()
	resultBz := make([]byte, 8)
	if event.VerifyResult == model.HashMismatched {
		binary.BigEndian.PutUint64(resultBz, uint64(challengetypes.CHALLENGE_SUCCEED))
	} else if event.VerifyResult == model.HashMatched {
		binary.BigEndian.PutUint64(resultBz, uint64(challengetypes.CHALLENGE_FAILED))
	} else {
		panic("cannot convert vote option")
	}

	bs := make([]byte, 0)
	bs = append(bs, challengeIdBz...)
	bs = append(bs, objectIdBz...)
	bs = append(bs, resultBz...)
	bs = append(bs, []byte(event.SpOperatorAddress)...)
	bs = append(bs, []byte(event.ChallengerAddress)...)
	hash := sdk.Keccak256Hash(bs)
	return hash[:]
}

func (h *DataHandler) FetchEventsForSubmit() ([]*model.Event, error) {
	return h.daoManager.GetEarliestEventsByStatus(model.EnoughVotesCollected, batchSize)
}

func (h *DataHandler) FetchVotesForAggregation(eventHash []byte) ([]*model.Vote, error) {
	return h.daoManager.GetVotesByEventHash(eventHash)
}

func (h *DataHandler) UpdateEventStatus(challengeId uint64, status model.EventStatus) error {
	return h.daoManager.UpdateEventStatusByChallengeId(challengeId, status)
}

func (h *DataHandler) SubmitTx(event *model.Event, validatorSet *bitset.BitSet, aggSignature []byte) (string, error) {
	voteResult := challengetypes.CHALLENGE_FAILED
	if event.VerifyResult == model.HashMismatched {
		voteResult = challengetypes.CHALLENGE_SUCCEED
	}
	return h.executor.SendAttestTx(event.ChallengeId, event.ObjectId, event.SpOperatorAddress,
		voteResult, event.ChallengerAddress,
		validatorSet.Bytes(), aggSignature)
}
