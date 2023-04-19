package submitter

import (
	"github.com/bnb-chain/greenfield-challenger/db/dao"
	"github.com/bnb-chain/greenfield-challenger/db/model"
	"github.com/bnb-chain/greenfield-challenger/executor"
	challengetypes "github.com/bnb-chain/greenfield/x/challenge/types"
	"github.com/willf/bitset"
)

type DataProvider interface {
	FetchEventsForSubmit(currentHeight uint64) ([]*model.Event, error)
	FetchVotesForAggregation(eventHash string) ([]*model.Vote, error)
	UpdateEventStatus(challengeId uint64, status model.EventStatus) error
	SubmitTx(event *model.Event, validatorSet *bitset.BitSet, aggSignature []byte) (string, error)
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

func (h *DataHandler) FetchEventsForSubmit(currentHeight uint64) ([]*model.Event, error) {
	return h.daoManager.GetUnexpiredEventsByStatus(currentHeight, model.EnoughVotesCollected)
}

func (h *DataHandler) FetchVotesForAggregation(eventHash string) ([]*model.Vote, error) {
	return h.daoManager.GetVotesByEventHash(eventHash)
}

func (h *DataHandler) UpdateEventStatus(challengeId uint64, status model.EventStatus) error {
	return h.daoManager.UpdateEventStatusByChallengeId(challengeId, status)
}

func (h *DataHandler) SubmitTx(event *model.Event, validatorSet *bitset.BitSet, aggSignature []byte) (string, error) {
	// TODO: Check if needed since MsgAttest is only submitted when HashMismatched
	voteResult := challengetypes.CHALLENGE_FAILED
	if event.VerifyResult == model.HashMismatched {
		voteResult = challengetypes.CHALLENGE_SUCCEED
	}
	return h.executor.SendAttestTx(event.ChallengeId, event.ObjectId, event.SpOperatorAddress,
		voteResult, event.ChallengerAddress,
		validatorSet.Bytes(), aggSignature)
}
