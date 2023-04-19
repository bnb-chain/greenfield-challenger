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

func (d *EventDao) GetUnexpiredEventsByStatus(currentHeight uint64, status model.EventStatus) ([]*model.Event, error) {
	events := []*model.Event{}
	err := d.DB.Where("expired_height > ?", currentHeight).
		Where("status = ?", status).
		Order("challenge_id asc").
		Find(&events).Error
	if err != nil && err != gorm.ErrRecordNotFound {
		return nil, err
	}
	return events, nil
}

func (d *EventDao) GetUnexpiredEventsByVerifyResult(limit int, currentHeight uint64, verifyResult model.VerifyResult) ([]*model.Event, error) {
	events := []*model.Event{}
	err := d.DB.Where("verify_result = ?", verifyResult).
		Where("expired_height > ?", currentHeight).
		Order("challenge_id asc").
		Limit(limit).
		Find(&events).Error
	if err != nil && err != gorm.ErrRecordNotFound {
		return nil, err
	}
	return events, nil
}

func (d *EventDao) GetEventByChallengeId(challengeId uint64) (*model.Event, error) {
	var event model.Event
	err := d.DB.Where("challenge_id = ?", challengeId).Take(&event).Error
	if err != nil {
		return nil, err
	}
	return &event, nil
}

func (d *EventDao) UpdateEventStatusByChallengeId(challengeId uint64, status model.EventStatus) error {
	return d.DB.Model(&model.Event{}).
		Where("challenge_id = ?", challengeId).
		Update("status", status).
		Error
}

func (d *EventDao) UpdateEventStatusVerifyResultByChallengeId(challengeId uint64, status model.EventStatus, result model.VerifyResult) error {
	return d.DB.Model(&model.Event{}).
		Where("challenge_id = ?", challengeId).
		Updates(model.Event{Status: status, VerifyResult: result}).
		Error
}

func (d *EventDao) IsEventExistsBetween(objectId, spOperatorAddress string, lowChallengeId, highChallengeId uint64) (bool, error) {
	exists := false
	if err := d.DB.Raw(
		"SELECT EXISTS(SELECT id FROM events WHERE object_id = ? and sp_operator_address = ? and challenge_id between ? and ?)",
		objectId, spOperatorAddress, lowChallengeId, highChallengeId).Scan(&exists).Error; err != nil {
		return false, err
	}
	return exists, nil
}

func (d *EventDao) DeleteEventsBefore(unixTimestamp int64) error {
	return d.DB.Model(&model.Event{}).Where("created_time < ?", unixTimestamp).Delete(&model.Event{}).Error
}
