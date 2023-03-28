package vote

import (
	"github.com/bnb-chain/greenfield-challenger/executor"

	"github.com/bnb-chain/greenfield-challenger/logging"

	"github.com/bnb-chain/greenfield-challenger/db/dao"
	"github.com/bnb-chain/greenfield-challenger/db/model"
)

const batchSize = 10

type DataProvider interface {
	FetchEventsForSelfVote() ([]*model.Event, error)
	FetchEventsForCollate(expiredHeight uint64) ([]*model.Event, error)
	UpdateEventStatus(challengeId uint64, status model.EventStatus) error
	SaveVote(vote *model.Vote) error
	IsVoteExists(eventHash []byte, pubKey []byte) (bool, error)
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

func (h *DataHandler) FetchEventsForSelfVote() ([]*model.Event, error) {
	events, err := h.daoManager.GetEarliestEventsByStatusAndAfter(model.Verified, batchSize, h.lastIdForSelfVote)
	if err != nil {
		logging.Logger.Errorf("failed to fetch events for self vote, err=%s", err.Error())
		return nil, err
	}
	heartbeatInterval, err := h.executor.QueryChallengeHeartbeatInterval()
	if err != nil {
		logging.Logger.Errorf("error querying heartbeat interval, err=%s", err.Error())
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
	return h.daoManager.GetUnexpiredAndSelfVotedEvents(currentHeight)
}

func (h *DataHandler) UpdateEventStatus(challengeId uint64, status model.EventStatus) error {
	return h.daoManager.UpdateEventStatusByChallengeId(challengeId, status)
}

func (h *DataHandler) SaveVote(vote *model.Vote) error {
	return h.daoManager.SaveVote(vote)
}

func (h *DataHandler) IsVoteExists(eventHash []byte, pubKey []byte) (bool, error) {
	return h.daoManager.IsVoteExists(eventHash, pubKey)
}
