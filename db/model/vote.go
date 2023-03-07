package model

import (
	"gorm.io/gorm"
)

type Vote struct {
	Id          int64
	ChallengeId uint64 `gorm:"NOT NULL;index:idx_challenge_id"`
	PubKey      string `gorm:"NOT NULL"`
	Signature   string `gorm:"NOT NULL"`
	EventType   uint32 `gorm:"NOT NULL"`
	EventHash   []byte `gorm:"NOT NULL"`
	CreatedTime int64  `gorm:"NOT NULL"`
}

func (*Vote) TableName() string {
	return "votes"
}

func InitVoteTable(db *gorm.DB) {
	if !db.Migrator().HasTable(&Vote{}) {
		err := db.Migrator().CreateTable(&Vote{})
		if err != nil {
			panic(err)
		}
	}
}
