package model

import (
	"errors"

	"gorm.io/gorm"
)

type Event struct {
	Id                int64
	ChallengeId       uint64      `gorm:"NOT NULL"`
	ObjectId          uint64      `gorm:"NOT NULL"`
	SegmentIndex      uint32      `gorm:"NOT NULL"`
	SpOperatorAddress string      `gorm:"NOT NULL"`
	RedundancyIndex   int32       `gorm:"NOT NULL"`
	Height            uint64      `gorm:"NOT NULL;"`
	Status            EventStatus `gorm:"NOT NULL;"`
}

func (*Event) TableName() string {
	return "event"
}

func InitEventTable(db *gorm.DB) {
	if !db.Migrator().HasTable(&Event{}) {
		err := db.Migrator().CreateTable(&Event{})
		if err != nil {
			panic(err)
		}
	}
}

type EventStatus uint32

const (
	Unprocessed                 EventStatus = 0
	EventStatusChallengeSucceed EventStatus = 1
	EventStatusChallengeFailed  EventStatus = 2
)

func EventStatusToStr(status EventStatus) (string, error) {
	switch status {
	case 0:
		return "Unprocessed", nil
	case 1:
		return "EventStatusChallengeSucceed", nil
	case 2:
		return "EventStatusChallengeFailed", nil
	default:
		return "", errors.New("invalid event status (0-2)")
	}
}
