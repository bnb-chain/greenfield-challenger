package model

import (
	"gorm.io/gorm"
)

type Event struct {
	Id                int64
	ChallengeId       uint64       `gorm:"NOT NULL"`
	ObjectId          string       `gorm:"NOT NULL"`
	SegmentIndex      uint32       `gorm:"NOT NULL"`
	SpOperatorAddress string       `gorm:"NOT NULL"`
	RedundancyIndex   int32        `gorm:"NOT NULL"`
	ChallengerAddress string       `gorm:"NOT NULL"`
	Height            uint64       `gorm:"NOT NULL;"`
	Status            EventStatus  `gorm:"NOT NULL;"`
	VerifyResult      VerifyResult `gorm:"NOT NULL;"`
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

// Unprocessed for events that have not been challenged
// ProcessedSucceed, ProcessedFailed for challenged events but not voted
// VotedSucceed, VotedFailed for events that have been challenged AND voted
const (
	Unprocessed            EventStatus = iota // Event is just stored
	Duplicated                                // Event is duplicated
	VerifiedValid                             // Event has been verified, and the challenge is valid
	VerifiedInvalid                           // Event has been verified, and the challenge is invalid
	SelfVoted                                 // Event has been voted locally
	EnoughVotesCollected                      // Event has been voted for more than 2/3 validators
	NoEnoughVotesCollected                    // Event cannot collect votes for more than 2/3 validators
	Submitted                                 // Event has been submitted for tx
	SubmitFailed                              // Event cannot be submitted for tx
)

type VerifyResult int

const (
	// The challenge failed.
	CHALLENGE_FAILED VerifyResult = 0
	// The challenge succeed.
	CHALLENGE_SUCCEED VerifyResult = 1
)
