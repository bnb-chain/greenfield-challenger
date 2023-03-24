package vote

import (
	"encoding/binary"
	"github.com/bnb-chain/greenfield-challenger/executor"

	"github.com/bnb-chain/greenfield-challenger/logging"

	sdkmath "cosmossdk.io/math"
	"github.com/bnb-chain/greenfield-challenger/db/dao"
	"github.com/bnb-chain/greenfield-challenger/db/model"
	challengetypes "github.com/bnb-chain/greenfield/x/challenge/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
)

const batchSize = 10

type DataProvider interface {
	CalculateEventHash(*model.Event) []byte
	FetchEventsForSelfVote() ([]*model.Event, error)
	FetchEventsForCollateVotes() ([]*model.Event, error)
	UpdateEventStatus(challengeId uint64, status model.EventStatus) error
	SaveVote(vote *model.Vote) error
	IsVoteExists(eventHash []byte, pubKey []byte) (bool, error)
}

type DataHandler struct {
	daoManager        *dao.DaoManager
	executor          *executor.Executor
	lastIdForSelfVote uint64 // some events' status will do not change anymore, so we need to skip them
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

func (h *DataHandler) FetchEventsForSelfVote() ([]*model.Event, error) {
	events, err := h.daoManager.GetEarliestEventsByStatusAndAfter(model.Verified, batchSize, h.lastIdForSelfVote)
	if err != nil {
		logging.Logger.Errorf("failed to fetch events for self vote, err=%s", err.Error())
		return nil, err
	}
	heartbeatInterval, err := h.executor.QueryChallengeHeartbeatInterval()
	if err != nil {
		logging.Logger.Errorf("error querying heartbeat interval, err=%s", err.Error())
		return nil, err
	}
	result := make([]*model.Event, 0)
	for _, e := range events {
		if e.VerifyResult == model.HashMismatched || e.ChallengeId%heartbeatInterval == 0 {
			result = append(result, e)
		}
		// it means if a challenge cannot be handled correctly, it will be skipped
		h.lastIdForSelfVote = e.ChallengeId
	}
	return result, nil
}

func (h *DataHandler) FetchEventsForCollateVotes() ([]*model.Event, error) {
	block, err := h.daoManager.GetLatestBlock()
	if err != nil {
		return nil, err
	}
	return h.daoManager.GetUnexpiredEvents(block.Height)
}

func (h *DataHandler) UpdateEventStatus(challengeId uint64, status model.EventStatus) error {
	return h.daoManager.UpdateEventStatusByChallengeId(challengeId, status)
}

func (h *DataHandler) SaveVote(vote *model.Vote) error {
	return h.daoManager.SaveVote(vote)
}

func (h *DataHandler) IsVoteExists(eventHash []byte, pubKey []byte) (bool, error) {
	return h.daoManager.IsVoteExists(eventHash, pubKey)
}
