package verifier

import (
	"bytes"
	"context"
	"encoding/hex"
	"github.com/bnb-chain/greenfield-challenger/db/dao"
	"github.com/bnb-chain/greenfield-challenger/db/model"
	"github.com/bnb-chain/greenfield-challenger/keys"
	"github.com/bnb-chain/greenfield-challenger/logging"
	"github.com/bnb-chain/greenfield-common/go/hash"
	"github.com/bnb-chain/greenfield-go-sdk/client/sp"
	"github.com/evmos/ethermint/crypto/ethsecp256k1"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/suite"
)

type eventSuite struct {
	suite.Suite
	dao *dao.EventDao
	db  *dao.Database
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
	event := model.Event{
		ChallengeId:       1,
		ObjectId:          1,
		SegmentIndex:      1,
		SpOperatorAddress: "test",
		RedundancyIndex:   1,
		ChallengerAddress: "test",
		Height:            1,
		AttestStatus:      model.Unprocessed,
		HeartbeatStatus:   model.Unprocessed,
	}

	err := s.dao.SaveEvent(&event)
	s.Require().NoError(err, "db: error creating event")

	lowestUnprocessedEvent, err := s.dao.GetUnprocessedEventWithLowestChallengeId()
	s.Require().NoError(err, "db: error retrieving event")
	s.Require().Equal(event, lowestUnprocessedEvent, "db: retrieved event != created event")

	// Query API for ObjectInfo
	challengeInfo := sp.ChallengeInfo{
		ObjectId:        "xxx",
		PieceIndex:      int(event.SegmentIndex),
		RedundancyIndex: int(event.RedundancyIndex),
	}

	mux := http.NewServeMux()
	server := httptest.NewServer(mux)
	privKey, err := ethsecp256k1.GenerateKey()
	keyManager, err := keys.NewPrivateKeyManager(hex.EncodeToString(privKey.Bytes()))
	spClient, err := sp.NewSpClient(server.URL[len("http://"):], sp.WithKeyManager(keyManager), sp.WithSecure(false))

	authInfo := sp.NewAuthInfo(false, "")
	challengeRes, err := spClient.ChallengeSP(context.Background(), challengeInfo, authInfo)

	logging.Logger.Infof(challengeRes.IntegrityHash)

	// TODO: Remove after API is working
	hashesStr := []string{"test1", "test2", "test3", "test4", "test5", "test6", "test7"}
	var hashesByte [][]byte
	for i, v := range hashesStr {
		hashesByte[i] = hash.CalcSHA256([]byte(v))
	}
	originalChecksums := bytes.Join(hashesByte, []byte(""))
	originalRootHash := []byte(hash.CalcSHA256Hex(originalChecksums))

	// Valid testcase
	validStr := "test1"
	hashesByte[0] = hash.CalcSHA256([]byte(validStr))
	validChecksums := bytes.Join(hashesByte, []byte(""))
	validRootHash := []byte(hash.CalcSHA256Hex(validChecksums))
	s.Require().Equal(originalRootHash, validRootHash)

	if bytes.Equal(originalRootHash, validRootHash) {
		s.dao.UpdateEventStatusByChallengeId(lowestUnprocessedEvent.ChallengeId, model.VerifiedValidChallenge)
		s.dao.GetEventByChallengeId(lowestUnprocessedEvent.ChallengeId)
	}

	s.Require().Equal(lowestUnprocessedEvent.AttestStatus, model.VerifiedValidChallenge)

	// Invalid testcase
	invalidStr := "testinvalid"
	hashesByte[0] = hash.CalcSHA256([]byte(invalidStr))
	invalidChecksums := bytes.Join(hashesByte, []byte(""))
	invalidRootHash := []byte(hash.CalcSHA256Hex(invalidChecksums))
	s.Require().NotEqual(originalRootHash, invalidRootHash)

	if !bytes.Equal(originalRootHash, invalidRootHash) {
		s.dao.UpdateEventStatusByChallengeId(lowestUnprocessedEvent.ChallengeId, model.VerifiedInvalidChallenge)
		s.dao.GetEventByChallengeId(lowestUnprocessedEvent.ChallengeId)
	}

	s.Require().Equal(lowestUnprocessedEvent.AttestStatus, model.VerifiedInvalidChallenge)

	//storageObj := types.QueryHeadObjectResponse{
	//	ObjectInfo: &types.ObjectInfo{
	//		Owner:                "test",
	//		BucketName:           "test",
	//		ObjectName:           "test",
	//		Id:                   math.NewUint(1),
	//		PayloadSize:          10,
	//		IsPublic:             true,
	//		ContentType:          "test",
	//		CreateAt:             0,
	//		ObjectStatus:         types.ObjectStatus(1),
	//		RedundancyType:       types.RedundancyType(1),
	//		SourceType:           types.SourceType(1),
	//		Checksums:            [][]byte{},
	//		SecondarySpAddresses: []string{},
	//		//LockedBalance: &cosmostypes.NewInt(1),
	//	},
	//}
	//
	//newChecksums := storageObj.ObjectInfo.GetChecksums() // 0-6
	//bytePieceData, err := ioutil.ReadAll(challengeRes.PieceData)
	//s.Require().NoError(err)
	//
	//hashPieceData := hash.CalcSHA256(bytePieceData)
	//newChecksums[lowestUnprocessedEvent.SegmentIndex] = hashPieceData
	//total := bytes.Join(newChecksums, []byte(""))
	//rootHash := []byte(hash.CalcSHA256Hex(total))

}
