package dao

import (
	"database/sql"
	"github.com/gnfd-challenger/db"
	"github.com/gnfd-challenger/db/model"
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

func (d *GreenlandDao) GetLatestBlock() (*model.GreenlandBlock, error) {
	block := model.GreenlandBlock{}
	err := d.DB.Model(model.GreenlandBlock{}).Order("height desc").Take(&block).Error
	if err != nil && err != gorm.ErrRecordNotFound {
		return nil, err
	}
	return &block, nil
}

func (d *GreenlandDao) GetTransactionsByStatus(s db.TxStatus) ([]*model.EventStartChallenge, error) {
	txs := make([]*model.EventStartChallenge, 0)
	err := d.DB.Where("status = ? ", s).Find(&txs).Order("tx_time desc").Error
	if err != nil && err != gorm.ErrRecordNotFound {
		return nil, err
	}
	return txs, nil
}

func (d *GreenlandDao) GetTransactionsByStatusAndHeight(status db.TxStatus, height uint64) ([]*model.EventStartChallenge, error) {
	txs := make([]*model.EventStartChallenge, 0)
	err := d.DB.Where("status = ? and height = ?", status, height).Find(&txs).Order("tx_time asc").Error
	if err != nil && err != gorm.ErrRecordNotFound {
		return nil, err
	}
	return txs, nil
}

func (d *GreenlandDao) GetLatestVotedTransactionHeight() (uint64, error) {
	var result uint64
	res := d.DB.Table("event_start_challenge").Select("MAX(height)").Where("status = ?", db.SelfVoted)
	if res.RowsAffected == 0 {
		return 0, nil
	}
	err := res.Row().Scan(&result)
	if err != nil {
		return 0, err
	}
	return result, nil
}

func (d *GreenlandDao) GetLeastSavedTransactionHeight() (uint64, error) {
	var result sql.NullInt64
	res := d.DB.Table("event_start_challenge").Select("MIN(height)").Where("status = ?", db.Saved)
	err := res.Row().Scan(&result)
	if err != nil {
		return 0, err
	}
	return uint64(result.Int64), nil
}

// TODO: Check required
//func (d *GreenlandDao) GetTransactionByChannelIdAndSequenceAndStatus(channelId relayercommon.ChannelId, sequence uint64, status db.TxStatus) (*model.EventStartChallenge, error) {
//	tx := model.EventStartChallenge{}
//	err := d.DB.Where("channel_id = ? and sequence = ? and status = ?", channelId, sequence, status).Find(&tx).Error
//	if err != nil && err != gorm.ErrRecordNotFound {
//		return nil, err
//	}
//	return &tx, nil
//}

// TODO: EventStartChallenge fields
func (d *GreenlandDao) UpdateTransactionStatus(id int64, status db.TxStatus) error {
	err := d.DB.Model(model.EventStartChallenge{}).Where("id = ?", id).Updates(
		model.EventStartChallenge{}).Error
	return err
}

// TODO: EventStartChallenge fields
func (d *GreenlandDao) UpdateTransactionStatusAndClaimTxHash(id int64, status db.TxStatus, claimTxHash string) error {
	return d.DB.Transaction(func(dbTx *gorm.DB) error {
		return dbTx.Model(model.EventStartChallenge{}).Where("id = ?", id).Updates(
			model.EventStartChallenge{}).Error
	})
}

func (d *GreenlandDao) SaveBlockAndBatchTransactions(b *model.GreenlandBlock, txs []*model.EventStartChallenge) error {
	return d.DB.Transaction(func(dbTx *gorm.DB) error {
		err := dbTx.Create(b).Error
		if err != nil {
			return err
		}

		if len(txs) != 0 {
			err := dbTx.Create(txs).Error
			if err != nil {
				return err
			}
		}
		return nil
	})
}
