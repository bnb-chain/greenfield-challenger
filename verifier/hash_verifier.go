package verifier

import (
	"bytes"
	"encoding/hex"
	"github.com/bnb-chain/greenfield-challenger/logging"
	"io"
	"time"

	"github.com/bnb-chain/greenfield-challenger/common"
	"github.com/bnb-chain/greenfield-challenger/config"
	"github.com/bnb-chain/greenfield-challenger/db/dao"
	"github.com/bnb-chain/greenfield-challenger/db/model"
	"github.com/bnb-chain/greenfield-challenger/executor"
	"github.com/bnb-chain/greenfield-common/go/hash"
)

type Verifier struct {
	daoManager            *dao.DaoManager
	config                *config.Config
	executor              *executor.Executor
	deduplicationInterval uint64
	heartbeatInterval     uint64
}

func NewHashVerifier(cfg *config.Config, dao *dao.DaoManager, executor *executor.Executor,
	deduplicationInterval, heartbeatInterval uint64) *Verifier {
	return &Verifier{
		config:                cfg,
		daoManager:            dao,
		executor:              executor,
		deduplicationInterval: deduplicationInterval,
		heartbeatInterval:     heartbeatInterval,
	}
}

func (v *Verifier) VerifyHashLoop() {
	for {
		err := v.verifyHash()
		if err != nil {
			time.Sleep(common.RetryInterval)
		}
	}
}

func (v *Verifier) verifyHash() error {
	// Read unprocessed event from db with lowest challengeId
	events, err := v.daoManager.EventDao.GetEarliestEventByStatus(model.Unprocessed, 10)
	if err != nil {
		logging.Logger.Errorf("verifier failed to retrieve the earliest events from db to begin verification, err=%s", err.Error())
		return err
	}
	if len(events) == 0 {
		time.Sleep(common.RetryInterval)
		return nil
	}

	for _, event := range events {
		err = v.verifyForSingleEvent(event)
		if err != nil {
			return err
		}
	}

	return nil
}

func (v *Verifier) verifyForSingleEvent(event *model.Event) error {
	// skip event if
	// 1) no challenger field and
	// 2) the event is not for heartbeat
	// 3) the event with same storage provider and object id has been processed recently and
	if event.ChallengerAddress == "" && event.ChallengeId%v.heartbeatInterval != 0 {
		found, err := v.daoManager.EventDao.IsEventExistsBetween(event.ObjectId, event.SpOperatorAddress,
			event.ChallengeId-v.deduplicationInterval, event.ChallengeId-1)
		if err != nil {
			logging.Logger.Errorf("verifier failed to retrieve information for event %d, err=%s", event.ChallengeId, err.Error())
			return err
		}
		if found {
			return v.daoManager.UpdateEventStatusByChallengeId(event.ChallengeId, model.Duplicated)
		}
	}

	// Call blockchain for object info to get original hash
	checksums, err := v.executor.GetObjectInfoChecksums(event.ObjectId)
	if err != nil {
		return err
	}
	chainRootHash := checksums[event.RedundancyIndex+1]

	// Call StorageProvider API to get piece hashes of the event
	spEndpoint, err := v.executor.GetStorageProviderEndpoint(event.SpOperatorAddress)
	if err != nil {
		logging.Logger.Errorf("verifier failed to get piece hashes from StorageProvider for event %d, err=%s", event.ChallengeId, err.Error())
		return err
	}
	challengeRes, err := v.executor.GetChallengeResultFromSp(spEndpoint, event.ObjectId,
		int(event.SegmentIndex), int(event.RedundancyIndex))

	pieceData, err := io.ReadAll(challengeRes.PieceData)
	if err != nil {
		logging.Logger.Errorf("verifier failed to read piece data for event %d, err=%s", event.ChallengeId, err.Error())
		return err
	}
	spChecksums := make([][]byte, 0)
	for _, h := range challengeRes.PiecesHash {
		checksum, err := hex.DecodeString(h)
		if err != nil {
			logging.Logger.Errorf("verifier failed to decode piece hash for event %d, err=%s", event.ChallengeId, err.Error())
			return err
		}
		spChecksums = append(checksums, checksum)
	}
	spRootHash := v.computeRootHash(event.SegmentIndex, pieceData, spChecksums)
	// Update database after comparing
	return v.compareHashAndUpdate(event.ChallengeId, chainRootHash, spRootHash)
}

func (v *Verifier) computeRootHash(segmentIndex uint32, pieceData []byte, checksums [][]byte) []byte {
	// Hash the piece that is challenged, replace original checksum, recompute new root hash
	dataHash := hash.CalcSHA256(pieceData)
	checksums[segmentIndex] = dataHash
	total := bytes.Join(checksums, []byte(""))
	rootHash := []byte(hash.CalcSHA256Hex(total))
	return rootHash
}

func (v *Verifier) compareHashAndUpdate(challengeId uint64, newHash []byte, originalHash []byte) error {
	if bytes.Equal(newHash, originalHash) {
		return v.daoManager.EventDao.UpdateEventStatusVerifyResultByChallengeId(challengeId, model.Verified, model.HashMatched)
	}
	return v.daoManager.EventDao.UpdateEventStatusVerifyResultByChallengeId(challengeId, model.Verified, model.HashMismatched)
}
