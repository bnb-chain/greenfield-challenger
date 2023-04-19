package dao

import (
	"github.com/bnb-chain/greenfield-challenger/db/model"
	"gorm.io/gorm"
)

type VoteDao struct {
	DB *gorm.DB
}

func NewVoteDao(db *gorm.DB) *VoteDao {
	return &VoteDao{
		DB: db,
	}
}

func (d *VoteDao) SaveVote(vote *model.Vote) error {
	err := d.DB.Create(vote).Error
	if err != nil {
		return err
	}
	return nil
}

func (d *VoteDao) SaveVoteAndUpdateEventStatus(vote *model.Vote, challengeId uint64) error {
	return d.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Save(vote).Error; err != nil {
			return err
		}

		if err := tx.Model(&model.Event{}).Where("challenge_id = ?", challengeId).Update("status", model.SelfVoted).Error; err != nil {
			return err
		}
		return nil
	})
}

func (d *VoteDao) GetVotesByEventHash(eventHash string) ([]*model.Vote, error) {
	votes := make([]*model.Vote, 0)
	err := d.DB.
		Where("event_hash = ?", eventHash).
		Find(&votes).Error
	if err != nil && err != gorm.ErrRecordNotFound {
		return nil, err
	}
	return votes, nil
}

func (d *VoteDao) IsVoteExists(eventHash string, pubKey string) (bool, error) {
	exists := false
	if err := d.DB.Raw(
		"SELECT EXISTS(SELECT id FROM votes WHERE event_hash = ? and pub_key = ?)",
		eventHash, pubKey).Scan(&exists).Error; err != nil {
		return false, err
	}
	return exists, nil
}

func (d *VoteDao) DeleteVotesBefore(unixTimestamp int64) error {
	return d.DB.Model(&model.Vote{}).Where("created_time = ?", unixTimestamp).Delete(&model.Vote{}).Error
}
