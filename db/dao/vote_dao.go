package dao

import (
	"github.com/bnb-chain/gnfd-challenger/db/model"
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

// TODO: Check which methods are required
func (db *VoteDao) GetVoteById(id int64) (*model.Vote, error) {
	var vote model.Vote
	err := db.DB.First(&vote, id).Error
	if err != nil {
		return nil, err
	}
	return &vote, nil
}

func (db *VoteDao) GetVotesByChallengeId(challengeId uint64) (*model.Vote, error) {
	var vote model.Vote
	err := db.DB.Where("challenge_id = ?", challengeId).Find(&vote).Error
	if err != nil {
		return nil, err
	}
	return &vote, nil
}

func (db *VoteDao) IsVoteExist(challengeId uint64) (bool, error) {
	var count int64
	err := db.DB.Model(&model.Vote{}).Where("challenge_id = ?", challengeId).Count(&count).Error
	if err != nil {
		return false, err
	}
	return count > 0, nil
}

func (db *VoteDao) UpdateVote(vote *model.Vote) error {
	err := db.DB.Save(vote).Error
	return err
}

func (db *VoteDao) UpdateVoteByChallengeId(challengeId uint64, updateFields map[string]interface{}) error {
	err := db.DB.Model(model.Vote{}).Where("challenge_id = ?", challengeId).Updates(updateFields).Error
	return err
}

func (db *VoteDao) DeleteVote(vote model.Vote) error {
	err := db.DB.Delete(vote).Error
	return err
}

func (db *VoteDao) DeleteVoteByChallengeId(challengeId uint64) error {
	err := db.DB.Where("challenge_id = ?", challengeId).Delete(&model.Vote{}).Error
	return err
}
