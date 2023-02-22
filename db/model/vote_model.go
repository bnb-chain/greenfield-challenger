package model

import (
	"gorm.io/gorm"
)

// TODO: EventType string -> EventType EventType
type Vote struct {
	Id          int64
	VoteOption  string `gorm:"NOT NULL"`
	ChallengeId int64  `gorm:"NOT NULL"`
	PubKey      []byte `gorm:"NOT NULL"`
	Signature   []byte `gorm:"NOT NULL"`
	EventType   uint32 `gorm:"NOT NULL"`
	EventHash   []byte `gorm:"NOT NULL"`
	CreatedTime int64  `gorm:"NOT NULL"`
}

func (*Vote) TableName() string {
	return "vote"
}

func InitVoteTables(db *gorm.DB) {
	if !db.Migrator().HasTable(&Vote{}) {
		err := db.Migrator().CreateTable(&Vote{})
		if err != nil {
			panic(err)
		}
	}
}
