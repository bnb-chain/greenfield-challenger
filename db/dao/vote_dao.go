package dao

import (
	"github.com/gnfd-challenger/db/model"
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

func (d *VoteDao) GetVotesByChannelIdAndSequence(channelId uint8, sequence uint64) ([]*model.Vote, error) {
	votes := make([]*model.Vote, 0)
	err := d.DB.Where("channel_id = ? and sequence = ?", channelId, sequence).Find(&votes).Error
	if err != nil && err != gorm.ErrRecordNotFound {
		return nil, err
	}
	return votes, nil
}

func (d *VoteDao) GetVoteByChannelIdAndSequenceAndPubKey(channelId uint8, sequence uint64, pubKey string) (*model.Vote, error) {
	vote := model.Vote{}
	err := d.DB.Model(model.Vote{}).Where("channel_id = ? and sequence = ? and pub_key = ?", channelId, sequence, pubKey).Take(&vote).Error
	if err != nil && err != gorm.ErrRecordNotFound {
		return nil, err
	}
	return &vote, nil
}

func (d *VoteDao) IsVoteExist(channelId uint8, sequence uint64, pubKey string) (bool, error) {
	exists := false
	if err := d.DB.Raw(
		"SELECT EXISTS(SELECT id FROM vote WHERE channel_id = ? and sequence = ? and pub_key = ?)",
		channelId, sequence, pubKey).Scan(&exists).Error; err != nil {
		return false, err
	}
	return exists, nil
}

func (d *VoteDao) SaveVote(vote *model.Vote) error {
	return d.DB.Transaction(func(dbTx *gorm.DB) error {
		return dbTx.Create(vote).Error
	})
}

func (d *VoteDao) SaveBatchVotes(votes []*model.Vote) error {
	return d.DB.Transaction(func(dbTx *gorm.DB) error {
		return dbTx.Create(votes).Error
	})
}
