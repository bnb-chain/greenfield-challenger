package verifier

import (
	"bytes"
	"context"
	"time"

	"github.com/bnb-chain/greenfield-challenger/common"
	"github.com/bnb-chain/greenfield-challenger/executor"

	"github.com/bnb-chain/greenfield-challenger/config"
	"github.com/bnb-chain/greenfield-challenger/db/dao"
	"github.com/bnb-chain/greenfield-challenger/db/model"
	"github.com/bnb-chain/greenfield-common/go/hash"
	"github.com/bnb-chain/greenfield-go-sdk/client/sp"
)

type Verifier struct {
	daoManager            *dao.DaoManager
	config                *config.Config
	executor              *executor.Executor
	deduplicationInterval uint64
	heartbeatInterval     uint64
}

func NewGreenfieldHashVerifier(cfg *config.Config, dao *dao.DaoManager, executor *executor.Executor,
	deduplicationInterval, heartbeatInterval uint64) *Verifier {
	return &Verifier{
		config:                cfg,
		daoManager:            dao,
		executor:              executor,
		deduplicationInterval: deduplicationInterval,
		heartbeatInterval:     heartbeatInterval,
	}
}

func (p *Verifier) VerifyHashLoop() {
	for {
		err := p.verifyHash()
		if err != nil {
			time.Sleep(common.RetryInterval)
		}
	}
}

func (p *Verifier) verifyHash() error {
	// Read unprocessed event from db with lowest challengeId
	events, err := p.daoManager.EventDao.GetEarliestEventByStatus(model.Unprocessed, 10)
	if err != nil {
		return err
	}
	if len(events) == 0 {
		time.Sleep(common.RetryInterval)
		return nil
	}

	for _, event := range events {
		err = p.verifyForSingleEvent(event)
		if err != nil {
			return err
		}
	}

	return nil
}

func (p *Verifier) verifyForSingleEvent(event *model.Event) error {
	// skip event if
	// 1) no challenger field and
	// 2) the event is not for heartbeat
	// 3) the event with same storage provider and object id has been processed recently and
	if event.ChallengerAddress == "" && event.ChallengeId%p.heartbeatInterval != 0 {
		found, err := p.daoManager.EventDao.IsEventExistsBetween(event.ObjectId, event.SpOperatorAddress,
			event.ChallengeId-p.deduplicationInterval, event.ChallengeId-1)
		if err != nil {
			return err
		}
		if found {
			return p.daoManager.UpdateEventStatusByChallengeId(event.ChallengeId, model.Duplicated)
		}
	}

	// Call StorageProvider API to get piece hashes of the event
	//challengeRes, err := p.getStorageObj(event)
	//if err != nil {
	//	return err
	//}
	//
	//// Call blockchain for storageObj to get original hash
	//headObjQueryReq := &storagetypes.QueryHeadObjectRequest{ // TODO: Will be changed to use ObjectID instead so will have to wait
	//	//BucketName:,
	//	//ObjectName:,
	//}
	//storageObj, err := client.StorageQueryClient.HeadObject(context.Background(), headObjQueryReq)
	//if err != nil {
	//	return err
	//}
	//
	//checksum := storageObj.ObjectInfo.GetChecksums()
	//segmentIndex := event.SegmentIndex
	//pieceData, err := io.ReadAll(challengeRes.PieceData)
	//if err != nil {
	//	return err
	//}
	//
	//rootHash, err := p.computeRootHash(segmentIndex, pieceData, checksum)
	//if err != nil {
	//	return err
	//}
	//
	//originalHash := storageObj.ObjectInfo.Checksums[event.RedundancyIndex+1]
	//p.compareHashAndUpdate(event.ChallengeId, rootHash, originalHash)

	return nil
}

func (p *Verifier) getStorageObj(event *model.Event) (*sp.ChallengeResult, error) {
	spClient := p.executor.GetSPClient()
	challengeInfo := sp.ChallengeInfo{
		ObjectId:        event.ObjectId,
		PieceIndex:      int(event.SegmentIndex),
		RedundancyIndex: int(event.RedundancyIndex),
	}
	authInfo := sp.NewAuthInfo(false, "") // TODO: What to use for authinfo?
	challengeRes, err := spClient.ChallengeSP(context.Background(), challengeInfo, authInfo)
	if err != nil {
		return nil, err
	}
	return &challengeRes, nil
}

func (p *Verifier) computeRootHash(segmentIndex uint32, pieceData []byte, checksum [][]byte) ([]byte, error) {
	// Checksum contains 7 piece hashes
	// Hash the piece that is challenged, replace in original checksum, recompute new roothash
	hashPieceData := hash.CalcSHA256(pieceData)
	checksum[segmentIndex] = hashPieceData
	total := bytes.Join(checksum, []byte(""))
	rootHash := []byte(hash.CalcSHA256Hex(total))
	return rootHash, nil
}

func (p *Verifier) compareHashAndUpdate(challengeId uint64, newHash []byte, originalHash []byte) error {
	if bytes.Equal(newHash, originalHash) {
		return p.daoManager.EventDao.UpdateEventStatusVerifyResultByChallengeId(challengeId, model.Verified, model.HashMatched)
	}
	return p.daoManager.EventDao.UpdateEventStatusVerifyResultByChallengeId(challengeId, model.Verified, model.HashMismatched)
}
