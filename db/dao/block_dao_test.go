package dao

import (
	"testing"

	"github.com/bnb-chain/greenfield-challenger/db/model"
	"github.com/stretchr/testify/suite"
)

type blockSuite struct {
	suite.Suite
	dao *BlockDao
	db  *Database
}

func TestBlockSuite(t *testing.T) {
	suite.Run(t, new(blockSuite))
}

func (s *blockSuite) SetupSuite() {
	dbName := "challenger"
	db, err := RunDB(dbName)
	s.Require().NoError(err)
	s.db = db
}

func (s *blockSuite) TearDownSuite() {
	err := s.db.StopDB()
	s.Require().NoError(err)
}

func (s *blockSuite) SetupTest() {
	model.InitBlockTable(s.db.DB)

	s.dao = NewBlockDao(s.db.DB)
}

func (s *blockSuite) TearDownTest() {
	err := s.db.ClearDB()
	s.Require().NoError(err)
}

func (s *blockSuite) TestGetLatestBlock() {
	block1 := &model.Block{
		Height:    100,
		BlockTime: 1000,
	}

	block2 := &model.Block{
		Height:    200,
		BlockTime: 1000,
	}

	result := s.dao.DB.Create(block1)
	s.Require().NoError(result.Error, "failed to create")

	result = s.dao.DB.Create(block2)
	s.Require().NoError(result.Error, "failed to create")

	latest, err := s.dao.GetLatestBlock()
	s.Require().NoError(err, "failed to query")
	s.Require().True(latest.Height == uint64(200))
}
