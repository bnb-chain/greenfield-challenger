package dao

import (
	"gorm.io/gorm"
)

type EventDao struct {
	DB *gorm.DB
}

func NewEventDao(db *gorm.DB) *EventDao {
	return &EventDao{
		DB: db,
	}
}

// query local db for challenge id -> query blockchain for challenge
// get_event
// update_event_status
// save_event
