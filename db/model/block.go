package model

import "gorm.io/gorm"

type Block struct {
	Id        int64
	Height    uint64 `gorm:"NOT NULL"`
	BlockTime int64  `gorm:"NOT NULL"`
}

func (*Block) TableName() string {
	return "block"
}

func InitBlockTable(db *gorm.DB) {
	if !db.Migrator().HasTable(&Block{}) {
		err := db.Migrator().CreateTable(&Block{})
		if err != nil {
			panic(err)
		}
	}
}
