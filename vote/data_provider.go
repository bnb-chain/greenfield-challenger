package vote

import (
	"github.com/bnb-chain/greenfield-challenger/db/dao"
	"github.com/bnb-chain/greenfield-challenger/db/model"
	"github.com/bnb-chain/greenfield-challenger/executor"
	"github.com/bnb-chain/greenfield-challenger/logging"
)

type DataProvider interface {
	FetchEventsForSelfVote(currentHeight uint64) ([]*model.Event, error)
	FetchEventsForCollate(currentHeight uint64) ([]*model.Event, error)
	FetchVotesForCollate(eventHash string) ([]*model.Vote, error)
	UpdateEventStatus(challengeId uint64, status model.EventStatus) error
	SaveVote(vote *model.Vote) error
	SaveVoteAndUpdateEventStatus(vote *model.Vote, challengeId uint64) error
	IsVoteExists(eventHash string, pubKey string) (bool, error)
}

type DataHandler struct {
	daoManager        *dao.DaoManager
	executor          *executor.Executor
	lastIdForSelfVote uint64 // some events' status will do not change anymore, so we need to skip them
}

func NewDataHandler(daoManager *dao.DaoManager, executor *executor.Executor) *DataHandler {
	return &DataHandler{
		daoManager: daoManager,
		executor:   executor,
	}
}

func (h *DataHandler) FetchEventsForSelfVote(currentHeight uint64) ([]*model.Event, error) {
	events, err := h.daoManager.GetUnexpiredEventsByStatus(currentHeight, model.Verified)
	if err != nil {
		logging.Logger.Errorf("failed to fetch events for self vote, err=%+v", err.Error())
		return nil, err
	}
	heartbeatInterval, err := h.executor.QueryChallengeHeartbeatInterval()
	logging.Logger.Infof("heartbeat interval is %d", heartbeatInterval)
	if err != nil {
		logging.Logger.Errorf("error querying heartbeat interval, err=%+v", err.Error())
		return nil, err
	}
	result := make([]*model.Event, 0)
	for _, e := range events {
		if e.VerifyResult == model.HashMismatched || e.ChallengeId%heartbeatInterval == 0 {
			result = append(result, e)
		}
		// it means if a challenge cannot be handled correctly, it will be skipped
		h.lastIdForSelfVote = e.ChallengeId
	}
	return result, nil
}

func (h *DataHandler) FetchEventsForCollate(currentHeight uint64) ([]*model.Event, error) {
	return h.daoManager.GetUnexpiredEventsByStatus(currentHeight, model.SelfVoted)
}

func (h *DataHandler) FetchVotesForCollate(eventHash string) ([]*model.Vote, error) {
	return h.daoManager.GetVotesByEventHash(eventHash)
}

func (h *DataHandler) UpdateEventStatus(challengeId uint64, status model.EventStatus) error {
	return h.daoManager.UpdateEventStatusByChallengeId(challengeId, status)
}

func (h *DataHandler) SaveVote(vote *model.Vote) error {
	return h.daoManager.SaveVote(vote)
}

func (h *DataHandler) SaveVoteAndUpdateEventStatus(vote *model.Vote, challengeId uint64) error {
	return h.daoManager.SaveVoteAndUpdateEventStatus(vote, challengeId)
}

func (h *DataHandler) IsVoteExists(eventHash string, pubKey string) (bool, error) {
	return h.daoManager.IsVoteExists(eventHash, pubKey)
}
