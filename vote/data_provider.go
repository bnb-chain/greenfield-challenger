package vote

import (
	"encoding/binary"
	"errors"

	sdkmath "cosmossdk.io/math"
	"github.com/bnb-chain/greenfield-challenger/db/dao"
	"github.com/bnb-chain/greenfield-challenger/db/model"
	challengetypes "github.com/bnb-chain/greenfield/x/challenge/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
)

type DataProvider interface {
	CalculateEventHash(*model.Event) [32]byte
	FetchEventForSelfVote() (*model.Event, error)
	FetchEventForCollectVotes() (*model.Event, error)
	UpdateEventStatus(challengeId uint64, status model.EventStatus) error
	SaveVote(vote *model.Vote) error
	IsVoteExists(challengeId uint64, pubKey string) (bool, error)
}

type AttestDataProvider struct {
	daoManager        *dao.DaoManager
	heartbeatInterval uint64
}

func NewAttestDataProvider(daoManager *dao.DaoManager, heartbeatInterval uint64) *AttestDataProvider {
	return &AttestDataProvider{
		daoManager:        daoManager,
		heartbeatInterval: heartbeatInterval,
	}
}

func (h *AttestDataProvider) CalculateEventHash(event *model.Event) [32]byte {
	challengeIdBz := make([]byte, 8)
	binary.BigEndian.PutUint64(challengeIdBz, event.ChallengeId)
	objectIdBz := sdkmath.NewUintFromString(event.ObjectId).Bytes()
	resultBz := make([]byte, 8)
	if event.Status == model.VerifiedValidChallenge {
		binary.BigEndian.PutUint64(resultBz, uint64(challengetypes.CHALLENGE_SUCCEED))
	} else if event.Status == model.VerifiedInvalidChallenge {
		binary.BigEndian.PutUint64(resultBz, uint64(challengetypes.CHALLENGE_FAILED))
	}

	bs := make([]byte, 0)
	bs = append(bs, challengeIdBz...)
	bs = append(bs, objectIdBz...)
	bs = append(bs, resultBz...)
	bs = append(bs, []byte(event.SpOperatorAddress)...)
	bs = append(bs, []byte(event.ChallengerAddress)...)
	hash := sdk.Keccak256Hash(bs)
	return hash
}

func (h *AttestDataProvider) FetchEventForSelfVote() (*model.Event, error) {

	earliest, err := h.daoManager.GetEarliestEventByStatuses([]model.EventStatus{model.VerifiedInvalidChallenge,
		model.VerifiedValidChallenge})
	if err != nil {
		return nil, err
	}

	if earliest.Status == model.VerifiedValidChallenge {
		return earliest, nil
	}

	if earliest.ChallengeId%h.heartbeatInterval == 0 {
		return earliest, nil
	}

	return nil, errors.New("no event for normal or heartbeat attest vote")
}

func (h *AttestDataProvider) FetchEventForCollectVotes() (*model.Event, error) {
	return h.daoManager.GetEarliestEventByStatus(model.SelfVoted)
}

func (h *AttestDataProvider) UpdateEventStatus(challengeId uint64, status model.EventStatus) error {
	return h.daoManager.UpdateEventStatusByChallengeId(challengeId, status)
}

func (h *AttestDataProvider) SaveVote(vote *model.Vote) error {
	return h.daoManager.SaveVote(vote)
}

func (h *AttestDataProvider) IsVoteExists(challengeId uint64, pubKey string) (bool, error) {
	return h.daoManager.IsVoteExists(challengeId, pubKey)
}
