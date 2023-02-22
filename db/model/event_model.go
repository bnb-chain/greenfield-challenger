package model

import (
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
	Status            string `gorm:"NOT NULL;"`

	//Height uint64 `gorm:"NOT NULL;index:idx_inscription_relay_transaction_height"`
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
