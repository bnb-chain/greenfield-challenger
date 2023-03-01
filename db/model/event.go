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

// Unprocessed for events that have not been challenged
// ProcessedSucceed, ProcessedFailed for challenged events but not voted
// VotedSucceed, VotedFailed for events that have been challenged AND voted
const (
	Unprocessed      EventStatus = 0
	ProcessedSucceed EventStatus = 1
	ProcessedFailed  EventStatus = 2
	VotedSucceed     EventStatus = 3
	VotedFailed      EventStatus = 4
)

func EventStatusToStr(status EventStatus) (string, error) {
	switch status {
	case 0:
		return "Unprocessed", nil
	case 1:
		return "ProcessedSucceed", nil
	case 2:
		return "ProcessedFailed", nil
	case 3:
		return "VotedSucceed", nil
	case 4:
		return "VotedFailed", nil
	default:
		return "", errors.New("invalid event status (0-4)")
	}
}
