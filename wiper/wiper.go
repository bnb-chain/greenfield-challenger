package wiper

import (
	"time"

	"github.com/bnb-chain/greenfield-challenger/common"
	"github.com/bnb-chain/greenfield-challenger/db/dao"
)

type DBWiper struct {
	daoManager *dao.DaoManager
}

func NewDBWiper(dao *dao.DaoManager) *DBWiper {
	return &DBWiper{
		daoManager: dao,
	}
}

func (w *DBWiper) DBWipeLoop() {
	ticker := time.NewTicker(DBWipeInterval)
	for range ticker.C {
		err := w.DBWipe()
		if err != nil {
			time.Sleep(common.RetryInterval)
		}
	}
}

func (w *DBWiper) DBWipe() error {
	err := w.daoManager.DeleteEventsBefore(WipeBefore)
	if err != nil {
		return err
	}
	err = w.daoManager.DeleteBlocksBefore(WipeBefore)
	if err != nil {
		return err
	}
	err = w.daoManager.DeleteVotesBefore(WipeBefore)
	if err != nil {
		return err
	}
	return nil
}
