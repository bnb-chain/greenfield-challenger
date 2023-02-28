package model

import "gorm.io/gorm"

type Tx struct {
	Id        int64
	Chain     string
	Height    uint64 `gorm:"NOT NULL"`
	BlockTime int64  `gorm:"NOT NULL"`
}

func (*Tx) TableName() string {
	return "tx"
}

func InitTxTable(db *gorm.DB) {
	if !db.Migrator().HasTable(&Tx{}) {
		err := db.Migrator().CreateTable(&Tx{})
		if err != nil {
			panic(err)
		}
	}
}
