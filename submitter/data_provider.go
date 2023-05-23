package submitter

import (
	"github.com/bnb-chain/greenfield-challenger/db/dao"
	"github.com/bnb-chain/greenfield-challenger/db/model"
	"github.com/bnb-chain/greenfield-challenger/executor"
)

type DataProvider interface {
	FetchEventsForSubmit(currentHeight uint64) ([]*model.Event, error)
	FetchVotesForAggregation(eventHash string) ([]*model.Vote, error)
	UpdateEventStatus(challengeId uint64, status model.EventStatus) error
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
