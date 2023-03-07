package dao

import (
	"testing"

	"github.com/bnb-chain/greenfield-challenger/db/model"
	"github.com/ethereum/go-ethereum/common"
	"github.com/stretchr/testify/suite"
)

type voteSuite struct {
	suite.Suite
	dao *VoteDao
	db  *Database
}

func TestVoteSuite(t *testing.T) {
	suite.Run(t, new(voteSuite))
}

func (s *voteSuite) SetupSuite() {
	dbName := "challenger"
	db, err := RunDB(dbName)
	s.Require().NoError(err)
	s.db = db
}

func (s *voteSuite) TearDownSuite() {
	err := s.db.StopDB()
	s.Require().NoError(err)
}

func (s *voteSuite) SetupTest() {
	model.InitVoteTable(s.db.DB)

	s.dao = NewVoteDao(s.db.DB)
}

func (s *voteSuite) TearDownTest() {
	err := s.db.ClearDB()
	s.Require().NoError(err)
}

func (s *voteSuite) createVote() *model.Vote {
	return &model.Vote{
		Id:          0,
		ChallengeId: 100,
		PubKey:      "pubkey",
		EventHash:   common.HexToHash("hash").Bytes(),
	}
}

func (s *voteSuite) TestVoteDao_SaveVote() {
	vote := s.createVote()
	err := s.dao.SaveVote(vote)
	s.Require().NoError(err, "failed to save")
}

func (s *voteSuite) TestVoteDao_GetVotesByChallengeId() {
	vote := s.createVote()
	_ = s.dao.SaveVote(vote)

	result, err := s.dao.GetVotesByChallengeId(vote.ChallengeId)
	s.Require().NoError(err, "failed to query")
	s.Require().True(result[0].ChallengeId == vote.ChallengeId)
}

func (s *voteSuite) TestVoteDao_IsVoteExists() {
	vote := s.createVote()
	_ = s.dao.SaveVote(vote)

	result, err := s.dao.IsVoteExists(vote.ChallengeId, vote.PubKey)
	s.Require().NoError(err, "failed to query")
	s.Require().True(result)

	result, err = s.dao.IsVoteExists(vote.ChallengeId, vote.PubKey+"fake")
	s.Require().NoError(err, "failed to query")
	s.Require().True(!result)
}
