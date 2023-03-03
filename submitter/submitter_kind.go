package submitter

import (
	"github.com/bnb-chain/greenfield-challenger/db/dao"
	"github.com/bnb-chain/greenfield-challenger/db/model"
	"github.com/bnb-chain/greenfield-challenger/executor"
	"github.com/willf/bitset"
)

type SubmitterKind interface {
	Name() string
	FetchEventForSubmit() (*model.Event, error)
	FetchVotesForAggregation(challengeId uint64) ([]*model.Vote, error)
	UpdateEventStatus(challengeId uint64, status model.EventStatus) error
	SubmitTx(event *model.Event, validatorSet *bitset.BitSet, aggSignature []byte) (string, error)
}

type Heartbeat struct {
	daoManager *dao.DaoManager
	executor   *executor.Executor
}

func NewHeartbeatKind(daoManager *dao.DaoManager, executor *executor.Executor) *Heartbeat {
	return &Heartbeat{
		daoManager: daoManager,
		executor:   executor,
	}
}

func (h *Heartbeat) Name() string {
	return "heartbeat"
}

func (h *Heartbeat) FetchEventForSubmit() (*model.Event, error) {
	return h.daoManager.GetEarliestHeartbeatEvent(model.AllVoted)
}

func (h *Heartbeat) FetchVotesForAggregation(challengeId uint64) ([]*model.Vote, error) {
	return h.daoManager.GetVotesByChallengeId(challengeId, h.Name())
}

func (h *Heartbeat) UpdateEventStatus(challengeId uint64, status model.EventStatus) error {
	return h.daoManager.UpdateEventByChallengeId(challengeId, map[string]interface{}{"heartbeat_status": status})
}

func (h *Heartbeat) SubmitTx(event *model.Event, validatorSet *bitset.BitSet, aggSignature []byte) (string, error) {
	return h.executor.SendHeartbeatTx(event.ChallengeId, validatorSet.Bytes(), aggSignature)
}
