package vote

import (
	"encoding/hex"
	"github.com/gnfd-challenger/db/model"
	"github.com/tendermint/tendermint/votepool"
	"time"
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

func EntityToDto(from *votepool.Vote) *model.Vote {
	v := model.Vote{
		PubKey:      from.PubKey[:],
		Signature:   from.Signature[:],
		EventType:   uint32(from.EventType),
		EventHash:   from.EventHash,
		CreatedTime: time.Now().Unix(),
	}
	return &v
}
