package model

import "gorm.io/gorm"

type Block struct {
	Id          int64  `gorm:"NOT NULL"`
	Height      uint64 `gorm:"NOT NULL;uniqueIndex:idx_height"`
	BlockTime   int64  `gorm:"NOT NULL"`
	CreatedTime int64  `gorm:"NOT NULL"`
}

func (*Block) TableName() string {
	return "blocks"
}

func InitBlockTable(db *gorm.DB) {
	if !db.Migrator().HasTable(&Block{}) {
		err := db.Migrator().CreateTable(&Block{})
		if err != nil {
			panic(err)
		}
	}
}
