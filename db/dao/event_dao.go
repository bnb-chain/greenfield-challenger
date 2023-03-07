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

func (d *EventDao) GetLatestEventByStatus(status model.EventStatus) (*model.Event, error) {
	e := model.Event{}
	err := d.DB.Where("status = ?", status).Order("challenge_id desc").First(&e).Error
	if err != nil {
		return nil, err
	}
	return &e, nil
}

func (d *EventDao) GetEarliestEventByStatus(status model.EventStatus, limit int) ([]*model.Event, error) {
	events := []*model.Event{}
	err := d.DB.Where("status = ?", status).
		Order("challenge_id asc").
		Limit(limit).
		Find(&events).Error
	if err != nil {
		return nil, err
	}
	return events, nil
}

func (d *EventDao) GetEarliestEventsByStatusAndAfter(status model.EventStatus, limit int, minChallengeId uint64) ([]*model.Event, error) {
	events := []*model.Event{}
	err := d.DB.Where("status = ?", status).
		Where("challenge_id >= ?", minChallengeId).
		Order("challenge_id asc").
		Limit(limit).
		Find(&events).Error
	if err != nil {
		return nil, err
	}
	return events, nil
}

func (db *EventDao) GetEventByChallengeId(challengeId uint64) (*model.Event, error) {
	var event model.Event
	err := db.DB.Where("challenge_id = ?", challengeId).Take(&event).Error
	if err != nil {
		return nil, err
	}
	return &event, nil
}

func (db *EventDao) UpdateEventStatusByChallengeId(challengeId uint64, status model.EventStatus) error {
	return db.DB.Model(&model.Event{}).
		Where("challenge_id = ?", challengeId).
		Update("status", status).
		Error
}

func (db *EventDao) UpdateEventStatusVerifyResultByChallengeId(challengeId uint64, status model.EventStatus, result model.VerifyResult) error {
	return db.DB.Model(&model.Event{}).
		Where("challenge_id = ?", challengeId).
		Updates(model.Event{Status: status, VerifyResult: result}).
		Error
}

func (db *EventDao) IsEventExistsBetween(objectId, spOperatorAddress string, lowChallengeId, highChallengeId uint64) (bool, error) {
	var count int64
	err := db.DB.Model(&model.Event{}).
		Where("object_id = ?", objectId).
		Where("sp_operator_address = ?", spOperatorAddress).
		Where("challenge_id between ? and ?", lowChallengeId, highChallengeId).
		Count(&count).Error
	if err != nil {
		return false, err
	}
	return count > 0, nil
}
