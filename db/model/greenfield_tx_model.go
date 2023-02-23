package model

import "gorm.io/gorm"

type GreenfieldTx struct {
	Id        int64
	Chain     string
	Height    uint64 `gorm:"NOT NULL"`
	BlockTime int64  `gorm:"NOT NULL"`
}

func (*GreenfieldTx) TableName() string {
	return "greenfield_tx"
}

func InitGreenfieldTxTables(db *gorm.DB) {
	if !db.Migrator().HasTable(&GreenfieldTx{}) {
		err := db.Migrator().CreateTable(&GreenfieldTx{})
		if err != nil {
			panic(err)
		}
	}
}
