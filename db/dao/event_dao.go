package dao

import (
	"gorm.io/gorm"
)

type GreenlandDao struct {
	DB *gorm.DB
}

func NewGreenlandDao(db *gorm.DB) *GreenlandDao {
	return &GreenlandDao{
		DB: db,
	}
}
