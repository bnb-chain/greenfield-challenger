package model

import (
	sdkmath "cosmossdk.io/math"
	"encoding/binary"
	challengetypes "github.com/bnb-chain/greenfield/x/challenge/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"gorm.io/gorm"
)

type Event struct {
	Id                int64
	ChallengeId       uint64       `gorm:"NOT NULL;uniqueIndex:idx_challenge_id"`
	ObjectId          string       `gorm:"NOT NULL;index:idx_object_id_sp_addr"`
	SegmentIndex      uint32       `gorm:"NOT NULL"`
	SpOperatorAddress string       `gorm:"NOT NULL;index:idx_object_id_sp_addr"`
	RedundancyIndex   int32        `gorm:"NOT NULL"`
	ChallengerAddress string       `gorm:"NOT NULL"`
	Height            uint64       `gorm:"NOT NULL;"`
	Status            EventStatus  `gorm:"NOT NULL;index:idx_status"`
	VerifyResult      VerifyResult `gorm:"NOT NULL;"`
	CreatedTime       int64        `gorm:"NOT NULL"`
	ExpiredHeight     uint64       `gorm:"NOT NULL"`
}

func (*Event) TableName() string {
	return "events"
}

func InitEventTable(db *gorm.DB) {
	if !db.Migrator().HasTable(&Event{}) {
		err := db.Migrator().CreateTable(&Event{})
		if err != nil {
			panic(err)
		}
	}
}

type EventStatus int

const (
	Unprocessed            EventStatus = iota // Event is just stored
	Duplicated                                // Event is duplicated
	Verified                                  // Event has been verified, and verify result is stored in VerifyResult
	SelfVoted                                 // Event has been voted locally
	EnoughVotesCollected                      // Event has been voted for more than 2/3 validators
	NoEnoughVotesCollected                    // Event cannot collect votes for more than 2/3 validators
	Submitted                                 // Event has been submitted for tx
	SubmitFailed                              // Event cannot be submitted for tx
	Skipped                                   // Event has been processed
)

type VerifyResult int

const (
	Unknown        VerifyResult = iota // Event not been verified
	HashMatched                        // The challenge failed, hashes are matched
	HashMismatched VerifyResult = 2    // The challenge succeed, hashed are not matched
)

func (e *Event) CalculateEventHash(event *Event) []byte {
	challengeIdBz := make([]byte, 8)
	binary.BigEndian.PutUint64(challengeIdBz, event.ChallengeId)
	objectIdBz := sdkmath.NewUintFromString(event.ObjectId).Bytes()
	resultBz := make([]byte, 8)
	if event.VerifyResult == HashMismatched {
		binary.BigEndian.PutUint64(resultBz, uint64(challengetypes.CHALLENGE_SUCCEED))
	} else if event.VerifyResult == HashMatched {
		binary.BigEndian.PutUint64(resultBz, uint64(challengetypes.CHALLENGE_FAILED))
	} else {
		panic("cannot convert vote option")
	}

	bs := make([]byte, 0)
	bs = append(bs, challengeIdBz...)
	bs = append(bs, objectIdBz...)
	bs = append(bs, resultBz...)
	bs = append(bs, []byte(event.SpOperatorAddress)...)
	bs = append(bs, []byte(event.ChallengerAddress)...)
	hash := sdk.Keccak256Hash(bs)
	return hash[:]
}
