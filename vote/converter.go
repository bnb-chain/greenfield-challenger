package vote

import (
	"encoding/hex"
	"time"

	"github.com/bnb-chain/greenfield-challenger/db/model"
	"github.com/tendermint/tendermint/votepool"
)

func DtoToEntity(v *model.Vote) (*votepool.Vote, error) {
	pubKeyBts, err := hex.DecodeString(string(v.PubKey))
	if err != nil {
		return nil, err
	}
	sigBts, err := hex.DecodeString(string(v.Signature))
	if err != nil {
		return nil, err
	}
	res := votepool.Vote{}
	res.EventType = votepool.EventType(v.EventType)
	res.PubKey = append(res.PubKey, pubKeyBts...)
	res.Signature = append(res.Signature, sigBts...)
	res.EventHash = append(res.EventHash, v.EventHash...)
	return &res, nil
}

func EntityToDto(from *votepool.Vote, challengeId uint64) *model.Vote {
	v := model.Vote{
		PubKey:      hex.EncodeToString(from.PubKey[:]),
		Signature:   hex.EncodeToString(from.Signature[:]),
		EventType:   uint32(from.EventType),
		EventHash:   from.EventHash,
		CreatedTime: time.Now().Unix(),
		ChallengeId: challengeId,
	}
	return &v
}
