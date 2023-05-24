package verifier

import (
	"github.com/bnb-chain/greenfield-challenger/db/dao"
	"github.com/bnb-chain/greenfield-challenger/db/model"
)

type DataProvider interface {
	FetchEventsForVerification(currentHeight uint64) ([]*model.Event, error)
	UpdateEventStatusVerifyResult(challengeId uint64, status model.EventStatus, verifyResult model.VerifyResult) error
	UpdateEventStatus(challengeId uint64, status model.EventStatus) error
	IsEventExistsBetween(objectId string, spOperatorAddr string, fromChallengeId uint64, toChallengeId uint64) (bool, error)
}

type DataHandler struct {
	daoManager *dao.DaoManager
}

func NewDataHandler(daoManager *dao.DaoManager) *DataHandler {
	return &DataHandler{
		daoManager: daoManager,
	}
}

func (h *DataHandler) FetchEventsForVerification(currentHeight uint64) ([]*model.Event, error) {
	return h.daoManager.EventDao.GetUnexpiredEventsByStatus(currentHeight, model.Unprocessed)
}

func (h *DataHandler) FetchVotesForAggregation(eventHash string) ([]*model.Vote, error) {
	return h.daoManager.GetVotesByEventHash(eventHash)
}

func (h *DataHandler) UpdateEventStatusVerifyResult(challengeId uint64, status model.EventStatus, verifyResult model.VerifyResult) error {
	return h.daoManager.UpdateEventStatusVerifyResultByChallengeId(challengeId, status, verifyResult)
}

func (h *DataHandler) UpdateEventStatus(challengeId uint64, status model.EventStatus) error {
	return h.daoManager.UpdateEventStatusByChallengeId(challengeId, status)
}

func (h *DataHandler) IsEventExistsBetween(objectId string, spOperatorAddr string, fromChallengeId uint64, toChallengeId uint64) (bool, error) {
	return h.daoManager.IsEventExistsBetween(objectId, spOperatorAddr, fromChallengeId, toChallengeId)
}
