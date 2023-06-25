package monitor

import (
	"github.com/bnb-chain/greenfield-challenger/db/dao"
	"github.com/bnb-chain/greenfield-challenger/db/model"
)

type DataProvider interface {
	SaveBlockAndEvents(block *model.Block, events []*model.Event) error
	GetLatestBlock() (*model.Block, error)
}

type DataHandler struct {
	daoManager *dao.DaoManager
}

func NewDataHandler(daoManager *dao.DaoManager) *DataHandler {
	return &DataHandler{
		daoManager: daoManager,
	}
}

func (h *DataHandler) SaveBlockAndEvents(block *model.Block, events []*model.Event) error {
	return h.daoManager.SaveBlockAndEvents(block, events)
}

func (h *DataHandler) GetLatestBlock() (*model.Block, error) {
	return h.daoManager.GetLatestBlock()
}
