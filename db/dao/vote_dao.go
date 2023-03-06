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

func (db *VoteDao) SaveVote(vote *model.Vote) error {
	err := db.DB.Create(vote).Error
	if err != nil {
		return err
	}
	return nil
}

func (db *VoteDao) SaveAllVotes(votes []*model.Vote) error {
	err := db.DB.Create(votes).Error
	if err != nil {
		return err
	}
	return nil
}

func (db *VoteDao) GetVotesByChallengeId(challengeId uint64) ([]*model.Vote, error) {
	votes := make([]*model.Vote, 0)
	err := db.DB.
		Where("challenge_id = ?", challengeId).
		Find(&votes).Error
	if err != nil && err != gorm.ErrRecordNotFound {
		return nil, err
	}
	return votes, nil
}

func (db *VoteDao) IsVoteExists(challengeId uint64, pubKey string) (bool, error) {
	var count int64
	err := db.DB.Model(&model.Vote{}).
		Where("challenge_id = ?", challengeId).
		Where("pub_key = ?", pubKey).
		Count(&count).Error
	if err != nil {
		return false, err
	}
	return count > 0, nil
}

func (db *VoteDao) DeleteVoteByChallengeId(challengeId uint64) error {
	err := db.DB.Where("challenge_id = ?", challengeId).Delete(&model.Vote{}).Error
	return err
}
