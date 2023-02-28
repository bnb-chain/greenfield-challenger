package monitor

import (
	"github.com/bnb-chain/gnfd-challenger/db/model"
	challengetypes "github.com/bnb-chain/greenfield/x/challenge/types"
)

func EntityToDto(height uint64, from *challengetypes.EventStartChallenge) *model.Event {
	to := model.Event{
		Id:                0,
		ChallengeId:       from.ChallengeId,
		ObjectId:          from.ObjectId,
		SegmentIndex:      from.SegmentIndex,
		SpOperatorAddress: from.SpOperatorAddress,
		RedundancyIndex:   from.RedundancyIndex,
		Height:            height,
		Status:            0,
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
