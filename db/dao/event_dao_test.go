package dao

import (
	"testing"

	"github.com/bnb-chain/greenfield-challenger/db/model"
	"github.com/stretchr/testify/suite"
)

type eventSuite struct {
	suite.Suite
	dao *EventDao
	db  *Database
}

func TestEventSuite(t *testing.T) {
	suite.Run(t, new(eventSuite))
}

func (s *eventSuite) SetupSuite() {
	dbName := "challenger"
	db, err := RunDB(dbName)
	s.Require().NoError(err)
	s.db = db
}

func (s *eventSuite) TearDownSuite() {
	err := s.db.StopDB()
	s.Require().NoError(err)
}

func (s *eventSuite) SetupTest() {
	model.InitBlockTable(s.db.DB)
	model.InitEventTable(s.db.DB)

	s.dao = NewEventDao(s.db.DB)
}

func (s *eventSuite) TearDownTest() {
	err := s.db.ClearDB()
	s.Require().NoError(err)
}

func (s *eventSuite) TestSaveBlockAndEvents() {
	block := &model.Block{
		Height:    100,
		BlockTime: 1000,
	}
	events := make([]*model.Event, 0)
	events = append(events, &model.Event{
		ChallengeId:       1,
		ObjectId:          "1",
		SegmentIndex:      1,
		SpOperatorAddress: "sp1",
		RedundancyIndex:   0,
		Height:            100,
		Status:            0,
	})

	err := s.dao.SaveBlockAndEvents(block, events)
	s.Require().NoError(err, "failed to create")
}
