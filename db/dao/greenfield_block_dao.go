package dao

import (
	"github.com/gnfd-challenger/db/model"
	"gorm.io/gorm"
)

// TODO: Check if dao is required since we're not saving block/tx, just extracting the events
type GreenfieldBlockDao struct {
	DB *gorm.DB
}

func NewGreenfieldBlockDao(db *gorm.DB) *GreenfieldBlockDao {
	return &GreenfieldBlockDao{
		DB: db,
	}
}

func (d *GreenfieldBlockDao) GetLatestGreenfieldBlock() (*model.GreenfieldBlock, error) {
	GreenfieldBlock := model.GreenfieldBlock{}
	err := d.DB.Model(model.GreenfieldBlock{}).Order("Height DESC").Take(&GreenfieldBlock).Error
	if err != nil && err != gorm.ErrRecordNotFound {
		return nil, err
	}
	return &GreenfieldBlock, nil
}