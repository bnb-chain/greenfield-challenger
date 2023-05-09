package submitter

import (
	"sync"
	"time"

	"github.com/bnb-chain/greenfield-challenger/db/dao"
	"github.com/bnb-chain/greenfield-challenger/db/model"
	"github.com/bnb-chain/greenfield-challenger/executor"
	"github.com/bnb-chain/greenfield-challenger/logging"
)

type AttestMonitor struct {
	daoManager           *dao.DaoManager
	executor             *executor.Executor
	mtx                  sync.RWMutex
	attestedChallengeIds []uint64 // used to save the last attested challenge id
}

func NewAttestMonitor(executor *executor.Executor, daoManager *dao.DaoManager) *AttestMonitor {
	return &AttestMonitor{
		daoManager: daoManager,
		executor:   executor,
		mtx:        sync.RWMutex{},
	}
}

func (a *AttestMonitor) UpdateAttestedChallengeIdLoop() {
	ticker := time.NewTicker(executor.QueryAttestedChallengeInterval)
	for range ticker.C {
		challengeIds, err := a.executor.QueryLatestAttestedChallengeIds()
		logging.Logger.Infof("latest attested challenge ids: %+v", challengeIds)
		if err != nil {
			logging.Logger.Errorf("update latest attested challenge error, err=%+v", err)
			continue
		}
		a.mtx.Lock()
		a.updateAttestedCacheAndEventStatus(a.attestedChallengeIds, challengeIds)
		a.attestedChallengeIds = challengeIds
		a.mtx.Unlock()
	}
}

func (a *AttestMonitor) updateAttestedCacheAndEventStatus(current, queried []uint64) []uint64 {
	m := make(map[uint64]bool)

	for _, challengeId := range queried {
		m[challengeId] = true
	}

	var diff []uint64
	for _, challengeId := range current {
		if _, ok := m[challengeId]; !ok {
			event, err := a.daoManager.GetEventByChallengeId(challengeId)
			if err != nil || event == nil {
				logging.Logger.Errorf("attest monitor failed to get event by challengeId: %d, err=%+v", challengeId, err)
				continue
			}
			if event.Status == model.Submitted {
				err = a.daoManager.UpdateEventStatusByChallengeId(challengeId, model.SelfAttested)
				if err != nil {
					logging.Logger.Errorf("update attested event status error, err=%+v", err)
				}
			}
			err = a.daoManager.UpdateEventStatusByChallengeId(challengeId, model.Attested)
			if err != nil {
				logging.Logger.Errorf("update attested event status error, err=%+v", err)
			}
			diff = append(diff, challengeId)
		}
	}
	return diff
}
