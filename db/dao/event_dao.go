package dao

import (
	"github.com/bnb-chain/greenfield-challenger/db/model"
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

func (d *EventDao) SaveBlockAndEvents(b *model.Block, events []*model.Event) error {
	return d.DB.Transaction(func(dbTx *gorm.DB) error {
		err := dbTx.Create(b).Error
		if err != nil {
			return err
		}

		if len(events) != 0 {
			err := dbTx.Create(events).Error
			if err != nil {
				return err
			}
		}
		return nil
	})
}

func (db *EventDao) SaveEvent(event *model.Event) error {
	err := db.DB.Create(event).Error
	if err != nil {
		return err
	}
	return nil
}

func (db *EventDao) SaveAllEvents(events []*model.Event) error {
	err := db.DB.Create(events).Error
	if err != nil {
		return err
	}
	return nil
}

// TODO: Check which methods are required
func (db *EventDao) GetEventById(id int64) (*model.Event, error) {
	var event model.Event
	err := db.DB.First(&event, id).Error
	if err != nil {
		return nil, err
	}
	return &event, nil
}

func (db *EventDao) GetEventsByChallengeId(challengeId uint64) (*model.Event, error) {
	var event model.Event
	err := db.DB.Where("challenge_id = ?", challengeId).Find(&event).Error
	if err != nil {
		return nil, err
	}
	return &event, nil
}

func (db *EventDao) GetUnprocessedEventWithLowestHeight() (*model.Event, error) {
	var event model.Event
	err := db.DB.Where("status = ?", model.Unprocessed).Order("height ASC").Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, err
	}
	return &event, nil
}

func (db *EventDao) GetUnprocessedEventWithLowestChallengeId() (*model.Event, error) {
	var event model.Event
	err := db.DB.Where("status = ?", model.Unprocessed).Order("challenge_id ASC").Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, err
	}
	return &event, nil
}

func (db *EventDao) GetAllEventsFromHeightWithStatus(height uint64, status model.EventStatus) ([]*model.Event, error) {
	var events []*model.Event
	err := db.DB.Where("status = ? AND height >= ?", status, height).Find(&events).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, err
	}
	return events, nil
}

func (db *EventDao) GetEventByLowestChallengeId() (*model.Event, error) {
	var challenge model.Event
	err := db.DB.Order("challenge_id ASC").First(&challenge).Error
	if err != nil {
		return nil, err
	}
	return &challenge, nil
}

func (db *EventDao) GetEventByHighestChallengeId() (*model.Event, error) {
	var challenge model.Event
	err := db.DB.Order("challenge_id DESC").First(&challenge).Error
	if err != nil {
		return nil, err
	}
	return &challenge, nil
}

func (db *EventDao) IsEventExist(challengeId uint64) (bool, error) {
	var count int64
	err := db.DB.Model(&model.Event{}).Where("challenge_id = ?", challengeId).Count(&count).Error
	if err != nil {
		return false, err
	}
	return count > 0, nil
}

func (db *EventDao) UpdateEvent(event *model.Event) error {
	err := db.DB.Save(event).Error
	return err
}

func (db *EventDao) UpdateEventStatusByChallengeId(challengeId uint64, status model.EventStatus) error {
	var event model.Event
	err := db.DB.Model(&model.Event{}).Where("challenge_id = ?", challengeId).First(&event).Error
	if err != nil {
		return err
	}

	event.Status = status
	err = db.DB.Save(&event).Error
	if err != nil {
		return err
	}

	return nil
}

func (db *EventDao) DeleteEvent(event model.Event) error {
	err := db.DB.Delete(event).Error
	return err
}

func (db *EventDao) DeleteEventByChallengeId(challengeId uint64) error {
	err := db.DB.Where("challenge_id = ?", challengeId).Delete(&model.Event{}).Error
	return err
}

// query local db for challenge id -> query blockchain for challenge
// get_event
// update_event_status
// save_event
