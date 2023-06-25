package attest

import (
	"github.com/bnb-chain/greenfield-challenger/db/dao"
	"github.com/bnb-chain/greenfield-challenger/db/model"
)

type DataProvider interface {
	GetEventByChallengeId(challengeId uint64) (*model.Event, error)
	UpdateEventStatus(challengeId uint64, status model.EventStatus) error
}

type DataHandler struct {
	daoManager *dao.DaoManager
}

func NewDataHandler(daoManager *dao.DaoManager) *DataHandler {
	return &DataHandler{
		daoManager: daoManager,
	}
}

func (h *DataHandler) UpdateEventStatus(challengeId uint64, status model.EventStatus) error {
	return h.daoManager.UpdateEventStatusByChallengeId(challengeId, status)
}

func (h *DataHandler) GetEventByChallengeId(challengeId uint64) (*model.Event, error) {
	return h.daoManager.GetEventByChallengeId(challengeId)
}
