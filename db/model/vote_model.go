package model

import "gorm.io/gorm"

// TODO: Check if id, other fields required
// TODO: EventType string -> EventType EventType
type Vote struct {
	Id        int64
	PubKey    []byte `gorm:"NOT NULL;index:idx_vote_channel_id_sequence_pub_key"`
	Signature []byte `gorm:"NOT NULL"`
	EventType string `gorm:"NOT NULL"`
	EventHash []byte `gorm:"NOT NULL"`
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
