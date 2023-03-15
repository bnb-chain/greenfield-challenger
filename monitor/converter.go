package monitor

import (
	"time"

	"github.com/bnb-chain/greenfield-challenger/db/model"
	challengetypes "github.com/bnb-chain/greenfield/x/challenge/types"
)

func EntityToDto(height uint64, from *challengetypes.EventStartChallenge) *model.Event {
	to := model.Event{
		ChallengeId:       from.ChallengeId,
		ObjectId:          from.ObjectId.String(),
		SegmentIndex:      from.SegmentIndex,
		SpOperatorAddress: from.SpOperatorAddress,
		RedundancyIndex:   from.RedundancyIndex,
		ChallengerAddress: from.ChallengerAddress,
		Height:            height,
		Status:            model.Unprocessed,
		VerifyResult:      model.Unknown,
		CreatedTime:       time.Now().Unix(),
	}
	return &to
}

func EntitiesToDtos(height uint64, froms []*challengetypes.EventStartChallenge) []*model.Event {
	tos := make([]*model.Event, 0, len(froms))
	for _, from := range froms {
		tos = append(tos, EntityToDto(height, from))
	}
	return tos
}
