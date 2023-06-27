package model

import (
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
	VerifyResult      VerifyResult `gorm:"NOT NULL;index:idx_verify_result"`
	CreatedTime       int64        `gorm:"NOT NULL"`
	ExpiredHeight     uint64       `gorm:"NOT NULL;index:idx_expired_height"`
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
	Unprocessed          EventStatus = iota // Event is just stored
	Verified                                // Event has been verified, and verify result is stored in VerifyResult
	SelfVoted                               // Event has been voted locally
	EnoughVotesCollected                    // Event has been voted for more than 2/3 validators
	Submitted
	SelfAttested
	Attested // Event has been submitted for tx
	Duplicated
	BucketDeleted
)

type VerifyResult int

const (
	Unknown        VerifyResult = iota // Event not been verified
	HashMatched                        // The challenge failed, hashes are matched
	HashMismatched                     // The challenge succeed, hashed are not matched
)
