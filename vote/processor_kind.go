package vote

import (
	"encoding/binary"
	"errors"

	"github.com/bnb-chain/greenfield-challenger/db/dao"
	"github.com/bnb-chain/greenfield-challenger/db/model"
	"github.com/ethereum/go-ethereum/crypto"
)

type ProcessorKind interface {
	Name() string
	CalculateEventHash(*model.Event) []byte
	FetchEventForSelfVote() (*model.Event, error)
	FetchEventForCollectVotes() (*model.Event, error)
	UpdateEventStatus(challengeId uint64, status model.EventStatus) error
	SaveVote(vote *model.Vote) error
	IsVoteExists(challengeId uint64, pubKey string) (bool, error)
}

type Heartbeat struct {
	daoManager *dao.DaoManager
	interval   uint64
}

func NewHeartbeatKind(daoManager *dao.DaoManager, interval uint64) *Heartbeat {
	return &Heartbeat{
		daoManager: daoManager,
		interval:   interval}
}

func (h *Heartbeat) Name() string {
	return "heartbeat"
}

func (h *Heartbeat) CalculateEventHash(event *model.Event) []byte {
	idBz := make([]byte, 8)
	binary.BigEndian.PutUint64(idBz, event.ChallengeId)

	hash := crypto.Keccak256Hash(idBz)
	return hash.Bytes()
}

func (h *Heartbeat) FetchEventForSelfVote() (*model.Event, error) {
	highest, err := h.daoManager.GetLatestEvent()
	if err != nil {
		return nil, err
	}

	heartbeat, err := h.daoManager.GetLatestHeartbeatEvent(model.SelfVoted)
	if err != nil {
		return nil, err
	}

	nextId := heartbeat.ChallengeId + h.interval
	if highest.ChallengeId < nextId {
		return nil, errors.New("need to wait for next interval")
	}

	event, err := h.daoManager.GetEventByChallengeId(nextId)
	if err != nil {
		return nil, err
	}

	return event, nil
}

func (h *Heartbeat) FetchEventForCollectVotes() (*model.Event, error) {
	return h.daoManager.GetEarliestHeartbeatEvent(model.SelfVoted)
}

func (h *Heartbeat) UpdateEventStatus(challengeId uint64, status model.EventStatus) error {
	return h.daoManager.UpdateVoteByChallengeId(challengeId, map[string]interface{}{"heartbeat_status": status})
}

func (h *Heartbeat) SaveVote(vote *model.Vote) error {
	vote.Kind = h.Name()
	return h.daoManager.SaveVote(vote)
}

func (h *Heartbeat) IsVoteExists(challengeId uint64, pubKey string) (bool, error) {
	return h.daoManager.IsVoteExist(challengeId, pubKey, h.Name())
}
