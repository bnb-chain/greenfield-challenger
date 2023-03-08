package verifier

import (
	"bytes"
	"github.com/bnb-chain/greenfield-challenger/db/dao"
	"github.com/bnb-chain/greenfield-challenger/db/model"
	"github.com/bnb-chain/greenfield-common/go/hash"
	"testing"

	"github.com/stretchr/testify/suite"
)

type eventSuite struct {
	suite.Suite
	dao      *dao.EventDao
	verifier *Verifier
	db       *dao.Database
}

func TestEventSuite(t *testing.T) {
	suite.Run(t, new(eventSuite))
}

func (s *eventSuite) SetupSuite() {
	dbName := "challenger"
	db, err := dao.RunDB(dbName)
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

	s.dao = dao.NewEventDao(s.db.DB)
}

func (s *eventSuite) TearDownTest() {
	err := s.db.ClearDB()
	s.Require().NoError(err)
}

func (s *eventSuite) TestHashing() {
	// Create event, Add update event by challengeId
	//event := model.Event{
	//	ChallengeId: 0,
	//	Status:      model.Unprocessed,
	//}

	hashesStr := []string{"test1", "test2", "test3", "test4", "test5", "test6", "test7"}
	checksums := make([][]byte, 7)
	for i, v := range hashesStr {
		checksums[i] = hash.CalcSHA256([]byte(v))
	}
	rootHash := bytes.Join(checksums, []byte(""))
	rootHash = []byte(hash.CalcSHA256Hex(rootHash))

	// Valid testcase
	validStr := []byte("test1")
	println(checksums[0])
	validRootHash, err := s.verifier.computeRootHash(0, validStr, checksums)
	s.Require().NoError(err)
	s.Require().Equal(validRootHash, rootHash)

	//s.verifier.compareHashAndUpdate(event.ChallengeId, validRootHash, rootHash)
	//updatedValidEvent, err := s.dao.GetEventByChallengeId(event.ChallengeId)
	//s.Require().NoError(err)
	//s.Require().Equal(updatedValidEvent.Status, model.VerifiedValidChallenge)

	// Invalid testcase
	invalidStr := []byte("invalid")
	invalidRootHash, err := s.verifier.computeRootHash(0, invalidStr, checksums)
	s.Require().NoError(err)
	s.Require().NotEqual(validRootHash, invalidRootHash)

	//s.verifier.compareHashAndUpdate(event.ChallengeId, invalidRootHash, rootHash)
	//updatedInvalidEvent, err := s.dao.GetEventByChallengeId(event.ChallengeId)
	//s.Require().NoError(err)
	//s.Require().Equal(updatedInvalidEvent.Status, model.VerifiedInvalidChallenge)
}
