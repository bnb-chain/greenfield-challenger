package model

import "gorm.io/gorm"

// TODO: Check if required, remove from init func
type GreenlandBlock struct {
	Id        int64
	Chain     string
	Height    uint64 `gorm:"NOT NULL;index:idx_greenland_block_height"`
	BlockTime int64  `gorm:"NOT NULL"`
}

func (*GreenlandBlock) TableName() string {
	return "greenland_block"
}

func InitGreenlandTables(db *gorm.DB) {
	if !db.Migrator().HasTable(&GreenlandBlock{}) {
		err := db.Migrator().CreateTable(&GreenlandBlock{})
		if err != nil {
			panic(err)
		}
	}
}
