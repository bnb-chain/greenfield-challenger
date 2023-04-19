package dao

import (
	"testing"
	"time"

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

func (s *eventSuite) createEvents() (*model.Block, *model.Event, *model.Event, *model.Event) {
	block := &model.Block{
		Height:    100,
		BlockTime: 1000,
	}
	event1 := &model.Event{
		ChallengeId:       1,
		ObjectId:          "1",
		SegmentIndex:      1,
		SpOperatorAddress: "sp1",
		RedundancyIndex:   0,
		Height:            100,
		Status:            model.Unprocessed,
		VerifyResult:      model.Unknown,
		CreatedTime:       time.Now().Unix(),
	}
	event10 := &model.Event{
		ChallengeId:       10,
		ObjectId:          "1",
		SegmentIndex:      1,
		SpOperatorAddress: "sp1",
		RedundancyIndex:   0,
		Height:            100,
		Status:            model.Unprocessed,
		VerifyResult:      model.Unknown,
		CreatedTime:       time.Now().Unix(),
	}
	event100 := &model.Event{
		ChallengeId:       100,
		ObjectId:          "1",
		SegmentIndex:      1,
		SpOperatorAddress: "sp1",
		RedundancyIndex:   0,
		Height:            100,
		Status:            model.Verified,
		VerifyResult:      model.HashMatched,
		CreatedTime:       time.Now().Unix(),
	}

	return block, event1, event10, event100
}

func (s *eventSuite) TestEventDao_SaveBlockAndEvents() {
	block, event1, event2, event3 := s.createEvents()
	events := []*model.Event{event1, event2, event3}
	err := s.dao.SaveBlockAndEvents(block, events)
	s.Require().NoError(err, "failed to create")
}

func (s *eventSuite) TestEventDao_GetEarliestEventByStatus() {
	block, event1, event2, event3 := s.createEvents()
	events := []*model.Event{event1, event2, event3}
	_ = s.dao.SaveBlockAndEvents(block, events)

	result, err := s.dao.GetUnexpiredEventsByStatus(0, model.Unprocessed)
	s.Require().NoError(err, "failed to query")
	s.Require().True(len(result) == 2)
	s.Require().True(result[0].ChallengeId == 1)

	result, err = s.dao.GetUnexpiredEventsByStatus(0, model.Verified)
	s.Require().NoError(err, "failed to query")
	s.Require().True(len(result) == 1)
	s.Require().True(result[0].ChallengeId == 1000)
}

func (s *eventSuite) TestEventDao_GetEarliestEventsByStatusAndAfter() {
	block, event1, event2, event3 := s.createEvents()
	events := []*model.Event{event1, event2, event3}
	_ = s.dao.SaveBlockAndEvents(block, events)

	result, err := s.dao.GetUnexpiredEventsByStatus(0, model.Unprocessed)
	s.Require().NoError(err, "failed to query")
	s.Require().True(len(result) == 1)
	s.Require().True(result[0].ChallengeId == 10)
}

func (s *eventSuite) TestEventDao_GetEventByChallengeId() {
	block, event1, event2, event3 := s.createEvents()
	events := []*model.Event{event1, event2, event3}
	_ = s.dao.SaveBlockAndEvents(block, events)

	result, err := s.dao.GetEventByChallengeId(event2.ChallengeId)
	s.Require().NoError(err, "failed to query")
	s.Require().True(result.ChallengeId == event2.ChallengeId)
}

func (s *eventSuite) TestEventDao_GetLatestEventByStatus() {
	block, event1, event2, event3 := s.createEvents()
	events := []*model.Event{event1, event2, event3}
	_ = s.dao.SaveBlockAndEvents(block, events)

	result, err := s.dao.GetLatestEventByStatus(model.Unprocessed)
	s.Require().NoError(err, "failed to query")
	s.Require().True(result.ChallengeId == 10)
}

func (s *eventSuite) TestEventDao_IsEventExistsBetween() {
	block, event1, event2, event3 := s.createEvents()
	events := []*model.Event{event1, event2, event3}
	_ = s.dao.SaveBlockAndEvents(block, events)

	result, err := s.dao.IsEventExistsBetween(event2.ObjectId, event2.SpOperatorAddress,
		event2.ChallengeId-1, event2.ChallengeId+1)
	s.Require().NoError(err, "failed to query")
	s.Require().True(result)

	result, err = s.dao.IsEventExistsBetween(event2.ObjectId, event2.SpOperatorAddress+"fake",
		event2.ChallengeId-1, event2.ChallengeId+1)
	s.Require().NoError(err, "failed to query")
	s.Require().True(!result)
}

func (s *eventSuite) TestEventDao_UpdateEventStatusByChallengeId() {
	block, event1, event2, event3 := s.createEvents()
	events := []*model.Event{event1, event2, event3}
	_ = s.dao.SaveBlockAndEvents(block, events)

	err := s.dao.UpdateEventStatusByChallengeId(event2.ChallengeId, model.Submitted)
	s.Require().NoError(err, "failed to update")

	result, _ := s.dao.GetEventByChallengeId(event2.ChallengeId)
	s.Require().True(result.Status == model.Submitted)
}

func (s *eventSuite) TestEventDao_UpdateEventStatusVerifyResultByChallengeId() {
	block, event1, event2, event3 := s.createEvents()
	events := []*model.Event{event1, event2, event3}
	_ = s.dao.SaveBlockAndEvents(block, events)

	err := s.dao.UpdateEventStatusVerifyResultByChallengeId(event2.ChallengeId, model.Verified, model.HashMatched)
	s.Require().NoError(err, "failed to update")

	result, _ := s.dao.GetEventByChallengeId(event2.ChallengeId)
	s.Require().True(result.Status == model.Verified)
	s.Require().True(result.VerifyResult == model.HashMatched)
}
