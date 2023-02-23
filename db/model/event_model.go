package model

import (
	"errors"
	"gorm.io/gorm"
)

type EventStartChallenge struct {
	Id                int64
	ChallengeId       uint64 `gorm:"NOT NULL"`
	ObjectId          uint64 `gorm:"NOT NULL"`
	SegmentIndex      uint32 `gorm:"NOT NULL"`
	SpOperatorAddress string `gorm:"NOT NULL"`
	RedundancyIndex   int32  `gorm:"NOT NULL"`
	Height            uint64 `gorm:"NOT NULL;"`
	Status            uint8  `gorm:"NOT NULL;"`
}

func (*EventStartChallenge) TableName() string {
	return "event_start_challenge"
}

func InitEventTables(db *gorm.DB) {
	if !db.Migrator().HasTable(&EventStartChallenge{}) {
		err := db.Migrator().CreateTable(&EventStartChallenge{})
		if err != nil {
			panic(err)
		}
	}
}

type EventStatus int

const (
	Unprocessed      EventStatus = 0
	ChallengeSuccess EventStatus = 1
	ChallengeFail    EventStatus = 2
)

func EventStatusToStr(status EventStatus) (string, error) {
	switch status {
	case 0:
		return "Unprocessed", nil
	case 1:
		return "ChallengeSuccess", nil
	case 2:
		return "ChallengeFail", nil
	default:
		return "", errors.New("invalid event status (0-2)")
	}
}
