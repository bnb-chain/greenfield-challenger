package attest

import (
	"github.com/bnb-chain/greenfield-challenger/metrics"
	"sync"
	"time"

	"github.com/bnb-chain/greenfield-challenger/db/model"
	"github.com/bnb-chain/greenfield-challenger/executor"
	"github.com/bnb-chain/greenfield-challenger/logging"
)

type AttestMonitor struct {
	executor             *executor.Executor
	mtx                  sync.RWMutex
	attestedChallengeIds map[uint64]bool // used to save the last attested challenge id
	dataProvider         DataProvider
	metricService        *metrics.MetricService
}

func NewAttestMonitor(executor *executor.Executor, dataProvider DataProvider, metricService *metrics.MetricService) *AttestMonitor {
	return &AttestMonitor{
		executor:             executor,
		mtx:                  sync.RWMutex{},
		attestedChallengeIds: make(map[uint64]bool, 0),
		dataProvider:         dataProvider,
		metricService:        metricService,
	}
}

// UpdateAttestedChallengeIdLoop polls the blockchain for latest attested challengeIds and updates their status
func (a *AttestMonitor) UpdateAttestedChallengeIdLoop() {
	ticker := time.NewTicker(QueryAttestedChallengeInterval)
	queryCount := 0
	for range ticker.C {
		challengeIds, err := a.executor.QueryLatestAttestedChallengeIds()
		// logging.Logger.Infof("latest attested challenge ids: %+v", challengeIds)
		if err != nil {
			logging.Logger.Errorf("update latest attested challenge error, err=%+v", err)
			continue
		}
		a.mtx.Lock()
		a.updateAttestedCacheAndEventStatus(a.attestedChallengeIds, challengeIds)
		for _, id := range challengeIds {
			a.attestedChallengeIds[id] = true
		}
		a.mtx.Unlock()

		queryCount++
		if queryCount > MaxQueryCount {
			a.attestedChallengeIds = make(map[uint64]bool, 0)
		}
	}
}

// updateAttestedCacheAndEventStatus only updates new entries
func (a *AttestMonitor) updateAttestedCacheAndEventStatus(old map[uint64]bool, latest []uint64) {
	for _, challengeId := range latest {
		if _, ok := old[challengeId]; !ok {
			go a.updateEventStatus(challengeId)
		}
	}
}

func (a *AttestMonitor) updateEventStatus(challengeId uint64) {
	event, err := a.dataProvider.GetEventByChallengeId(challengeId)
	if err != nil || event == nil {
		logging.Logger.Errorf("attest monitor failed to get event by challengeId: %d, err=%+v", challengeId, err)
		return
	}
	if event.Status == model.SelfAttested || event.Status == model.Attested {
		return
	}
	var status model.EventStatus
	if event.Status == model.Submitted {
		status = model.SelfAttested
	} else {
		status = model.Attested
	}
	err = a.dataProvider.UpdateEventStatus(challengeId, status)
	if err != nil {
		logging.Logger.Errorf("update attested event status error, err=%s", err.Error())
	}
	a.metricService.SetAttestedChallenges(challengeId)
	a.metricService.IncChallengeAttested()
}
