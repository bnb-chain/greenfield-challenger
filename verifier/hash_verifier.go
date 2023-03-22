package verifier

import (
	"bytes"
	"context"
	"encoding/hex"
	"io"
	"strings"
	"sync"
	"time"

	"github.com/avast/retry-go/v4"
	"github.com/bnb-chain/greenfield-challenger/common"
	"github.com/bnb-chain/greenfield-challenger/config"
	"github.com/bnb-chain/greenfield-challenger/db/dao"
	"github.com/bnb-chain/greenfield-challenger/db/model"
	"github.com/bnb-chain/greenfield-challenger/executor"
	"github.com/bnb-chain/greenfield-challenger/logging"
	"github.com/bnb-chain/greenfield-common/go/hash"
	"github.com/bnb-chain/greenfield-go-sdk/client/sp"
)

type Verifier struct {
	daoManager            *dao.DaoManager
	config                *config.Config
	executor              *executor.Executor
	deduplicationInterval uint64
}

func NewHashVerifier(cfg *config.Config, dao *dao.DaoManager, executor *executor.Executor,
	deduplicationInterval uint64,
) *Verifier {
	return &Verifier{
		config:                cfg,
		daoManager:            dao,
		executor:              executor,
		deduplicationInterval: deduplicationInterval,
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
	events, err := v.daoManager.EventDao.GetEarliestEventsByStatus(model.Unprocessed, 10)
	if err != nil {
		logging.Logger.Errorf("verifier failed to retrieve the earliest events from db to begin verification, err=%s", err.Error())
		return err
	}
	if len(events) == 0 {
		time.Sleep(common.RetryInterval)
		return nil
	}

	var wg sync.WaitGroup
	var firstErr error
	for _, event := range events {
		wg.Add(1)
		go func(event *model.Event) {
			defer wg.Done()
			err := v.verifyForSingleEvent(event)
			if err != nil && firstErr == nil {
				firstErr = err
			}
		}(event)
	}
	wg.Wait()

	return firstErr
}

func (v *Verifier) verifyForSingleEvent(event *model.Event) error {
	if err := v.preCheck(event); err != nil {
		return err
	}

	// Call blockchain for object info to get original hash
	checksums, err := v.executor.GetObjectInfoChecksums(event.ObjectId)
	if err != nil {
		if strings.Contains(err.Error(), "No such object") {
			v.daoManager.EventDao.UpdateEventStatusVerifyResultByChallengeId(event.ChallengeId, model.Skipped, model.Unknown)
		}
		return err
	}
	chainRootHash := checksums[event.RedundancyIndex+1]

	// Call StorageProvider API to get piece hashes of the event
	spEndpoint, err := v.executor.GetStorageProviderEndpoint(event.SpOperatorAddress)
	if err != nil {
		logging.Logger.Errorf("verifier failed to get piece hashes from StorageProvider for event %d, err=%s", event.ChallengeId, err.Error())
		return err
	}

	challengeRes := &sp.ChallengeResult{}
	err = retry.Do(func() error {
		challengeRes, err = v.executor.GetChallengeResultFromSp(spEndpoint, event.ObjectId,
			int(event.SegmentIndex), int(event.RedundancyIndex))
		if err != nil {
			return err
		}
		return err
	}, retry.Context(context.Background()), common.RtyAttem, common.RtyDelay, common.RtyErr)
	if err != nil {
		// after testing, we can make sure it is not client errors, treat it as sp side error
		logging.Logger.Errorf("failed to call storage api for challenge %d, err=%s", event.ChallengeId, err.Error())
		return v.compareHashAndUpdate(event.ChallengeId, chainRootHash, []byte{})
	}

	pieceData, err := io.ReadAll(challengeRes.PieceData)
	piecesHash := challengeRes.PiecesHash
	if err != nil {
		logging.Logger.Errorf("verifier failed to read piece data for event %d, err=%s", event.ChallengeId, err.Error())
		return err
	}
	spChecksums := make([][]byte, 0)
	for _, h := range piecesHash {
		checksum, err := hex.DecodeString(h)
		if err != nil {
			panic(err)
		}
		spChecksums = append(spChecksums, checksum)
	}
	spRootHash := v.computeRootHash(event.SegmentIndex, pieceData, spChecksums)
	// Update database after comparing
	err = v.compareHashAndUpdate(event.ChallengeId, chainRootHash, spRootHash)
	if err != nil {
		logging.Logger.Errorf("failed to update event status, challenge id: %d, err: %s",
			event.ChallengeId, err)
	}
	return err
}

func (v *Verifier) preCheck(event *model.Event) error {
	// event will be skipped if
	// 1) the challenge with bigger id has been attested
	attestedId, err := v.executor.QueryLatestAttestedChallengeId()
	if err != nil {
		return err
	}
	if attestedId >= event.ChallengeId {
		logging.Logger.Infof("verifier skips the challenge %d, attested id=%d", event.ChallengeId, attestedId)
		return v.daoManager.UpdateEventStatusByChallengeId(event.ChallengeId, model.Skipped)
	}

	// event is duplicated if
	// 1) no challenger field and
	// 2) the event is not for heartbeat
	// 3) the event with same storage provider and object id has been processed recently and
	heartbeatInterval, err := v.executor.QueryChallengeHeartbeatInterval()
	if err != nil {
		return err
	}
	if heartbeatInterval == 0 {
		panic("heartbeat interval should not zero, potential bug")
	}
	if event.ChallengerAddress == "" && event.ChallengeId%heartbeatInterval != 0 && event.ChallengeId > v.deduplicationInterval {
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

	return nil
}

func (v *Verifier) computeRootHash(segmentIndex uint32, pieceData []byte, checksums [][]byte) []byte {
	// Hash the piece that is challenged, replace original checksum, recompute new root hash
	dataHash := hash.GenerateChecksum(pieceData)
	checksums[segmentIndex] = dataHash
	total := bytes.Join(checksums, []byte(""))
	rootHash := hash.GenerateChecksum(total)
	return rootHash
}

func (v *Verifier) compareHashAndUpdate(challengeId uint64, chainRootHash []byte, spRootHash []byte) error {
	if bytes.Equal(chainRootHash, spRootHash) {
		return v.daoManager.EventDao.UpdateEventStatusVerifyResultByChallengeId(challengeId, model.Verified, model.HashMatched)
	}
	return v.daoManager.EventDao.UpdateEventStatusVerifyResultByChallengeId(challengeId, model.Verified, model.HashMismatched)
}
