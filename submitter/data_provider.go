package submitter

import (
	"github.com/bnb-chain/greenfield-challenger/db/dao"
	"github.com/bnb-chain/greenfield-challenger/db/model"
	"github.com/bnb-chain/greenfield-challenger/executor"
	challengetypes "github.com/bnb-chain/greenfield/x/challenge/types"
	"github.com/willf/bitset"
)

const batchSize = 10

type DataProvider interface {
	FetchEventsForSubmit() ([]*model.Event, error)
	FetchVotesForAggregation(challengeId uint64) ([]*model.Vote, error)
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

func (h *DataHandler) FetchEventsForSubmit() ([]*model.Event, error) {
	return h.daoManager.GetEarliestEventByStatus(model.EnoughVotesCollected, 10)
}

func (h *DataHandler) FetchVotesForAggregation(challengeId uint64) ([]*model.Vote, error) {
	return h.daoManager.GetVotesByChallengeId(challengeId)
}

func (h *DataHandler) UpdateEventStatus(challengeId uint64, status model.EventStatus) error {
	return h.daoManager.UpdateEventStatusByChallengeId(challengeId, status)
}

func (h *DataHandler) SubmitTx(event *model.Event, validatorSet *bitset.BitSet, aggSignature []byte) (string, error) {
	voteResult := challengetypes.CHALLENGE_FAILED
	if event.VerifyResult == model.CHALLENGE_SUCCEED {
		voteResult = challengetypes.CHALLENGE_SUCCEED
	}
	return h.executor.SendAttestTx(event.ChallengeId, event.ObjectId, event.SpOperatorAddress,
		voteResult, event.ChallengerAddress,
		validatorSet.Bytes(), aggSignature)
}
