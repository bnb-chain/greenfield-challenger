package model

import (
	"errors"

	"gorm.io/gorm"
)

type Vote struct {
	Id          int64
	VoteOption  VoteOption `gorm:"NOT NULL"`
	ChallengeId uint64     `gorm:"NOT NULL"`
	PubKey      string     `gorm:"NOT NULL"`
	Signature   string     `gorm:"NOT NULL"`
	EventType   uint32     `gorm:"NOT NULL"`
	EventHash   []byte     `gorm:"NOT NULL"`
	CreatedTime int64      `gorm:"NOT NULL"`
	Kind        string     `gorm:"NOT NULL"`
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

type VoteOption uint32

const (
	VoteOptChallengeSucceed VoteOption = 1
	VoteOptChallengeFailed  VoteOption = 2
)

func VoteOptionToStr(opt VoteOption) (string, error) {
	switch opt {
	case 1:
		return "VoteOptChallengeResultSucceed", nil
	case 2:
		return "VoteOptChallengeResultFailed", nil
	default:
		return "", errors.New("invalid event status (0-2)")
	}
}
