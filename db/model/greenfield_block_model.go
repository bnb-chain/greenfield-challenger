package model

import "gorm.io/gorm"

type GreenfieldBlock struct {
	Id                  int64
	Chain               string
	Height              uint64 `gorm:"NOT NULL"`
	GreenfieldBlockTime int64  `gorm:"NOT NULL"`
}

func (*GreenfieldBlock) TableName() string {
	return "GreenfieldBlock"
}

func InitGreenfieldBlockTables(db *gorm.DB) {
	if !db.Migrator().HasTable(&GreenfieldBlock{}) {
		err := db.Migrator().CreateTable(&GreenfieldBlock{})
		if err != nil {
			panic(err)
		}
	}
}
