package verifier

import (
	"bytes"
	"context"
	"encoding/hex"
	"io"
	"strings"
	"sync"
	"time"

	"github.com/bnb-chain/greenfield-go-sdk/types"

	"github.com/panjf2000/ants/v2"

	"github.com/avast/retry-go/v4"
	"github.com/bnb-chain/greenfield-challenger/common"
	"github.com/bnb-chain/greenfield-challenger/config"
	"github.com/bnb-chain/greenfield-challenger/db/dao"
	"github.com/bnb-chain/greenfield-challenger/db/model"
	"github.com/bnb-chain/greenfield-challenger/executor"
	"github.com/bnb-chain/greenfield-challenger/logging"
	"github.com/bnb-chain/greenfield-common/go/hash"
)

type Verifier struct {
	daoManager            *dao.DaoManager
	config                *config.Config
	executor              *executor.Executor
	deduplicationInterval uint64
	cachedChallengeIds    map[uint64]bool
	mtx                   sync.RWMutex
}

func NewHashVerifier(cfg *config.Config, dao *dao.DaoManager, executor *executor.Executor,
	deduplicationInterval uint64,
) *Verifier {
	return &Verifier{
		config:                cfg,
		daoManager:            dao,
		executor:              executor,
		deduplicationInterval: deduplicationInterval,
		mtx:                   sync.RWMutex{},
	}
}

func (v *Verifier) VerifyHashLoop() {
	// Event lasts for 300 blocks, 2x for redundancy
	v.cachedChallengeIds = make(map[uint64]bool, common.CacheSize)

	pool, err := ants.NewPool(20)
	if err != nil {
		logging.Logger.Errorf("verifier failed to create ant pool, err=%+v", err.Error())
		return
	}
	defer pool.Release()

	for {
		err := v.verifyHash(pool)
		if err != nil {
			time.Sleep(common.RetryInterval)
			continue
		}
		time.Sleep(VerifyHashLoopInterval)
	}
}

func (v *Verifier) verifyHash(pool *ants.Pool) error {
	// Read unprocessed event from db with lowest challengeId
	currentHeight := v.executor.GetCachedBlockHeight()
	events, err := v.daoManager.EventDao.GetUnexpiredEventsByStatus(currentHeight, model.Unprocessed)
	if err != nil {
		logging.Logger.Errorf("verifier failed to retrieve the earliest events from db to begin verification, err=%+v", err.Error())
		return err
	}
	if len(events) == 0 {
		time.Sleep(common.RetryInterval)
		return nil
	}

	for _, event := range events {
		v.mtx.Lock()
		isCached := v.cachedChallengeIds[event.ChallengeId]
		v.mtx.Unlock()

		if isCached {
			continue
		}

		var firstErr error
		err = pool.Submit(func() {
			firstErr = v.verifyForSingleEvent(event)
		})
		if !isCached {
			v.mtx.Lock()
			v.cachedChallengeIds[event.ChallengeId] = true
			v.mtx.Unlock()
		}
		if firstErr != nil {
			if err.Error() == common.ErrEventExpired.Error() {
				err = v.daoManager.UpdateEventStatusVerifyResultByChallengeId(event.ChallengeId, model.Expired, model.Unknown)
				if err != nil {
					return err
				}
				v.mtx.Lock()
				delete(v.cachedChallengeIds, event.ChallengeId)
				v.mtx.Unlock()
				continue
			}
		}
		if err != nil {
			logging.Logger.Errorf("verifier failed to submit to pool for challenge %d, err=%+v", event.ChallengeId, err.Error())
			continue
		}
	}
	return nil
}

func (v *Verifier) verifyForSingleEvent(event *model.Event) error {
	logging.Logger.Infof("verifier started for challengeId: %d %s", event.ChallengeId, time.Now().Format("15:04:05.000000"))
	currentHeight := v.executor.GetCachedBlockHeight()
	if err := v.preCheck(event, currentHeight); err != nil {
		return err
	}

	endpoint, err := v.executor.GetStorageProviderEndpoint(event.SpOperatorAddress)
	logging.Logger.Infof("sp endpoint: %s, objectId: %s", endpoint, event.ObjectId)

	// Call blockchain for object info to get original hash
	checksums, err := v.executor.GetObjectInfoChecksums(event.ObjectId)
	if err != nil {
		if strings.Contains(err.Error(), "No such object") {
			logging.Logger.Errorf("No such object error for challengeId: %d", event.ChallengeId)
			err := v.daoManager.EventDao.UpdateEventStatusByChallengeId(event.ChallengeId, model.Expired)
			if err != nil {
				return err
			}
		}
		return err
	}
	chainRootHash := checksums[event.RedundancyIndex+1]
	logging.Logger.Infof("chainRootHash: %s for challengeId: %d", string(chainRootHash), event.ChallengeId)

	// Call sp for challenge result
	challengeRes := &types.ChallengeResult{}
	err = retry.Do(func() error {
		challengeRes, err = v.executor.GetChallengeResultFromSp(event.ObjectId,
			int(event.SegmentIndex), int(event.RedundancyIndex))
		if err != nil {
			logging.Logger.Errorf("error getting challenge result from sp for challengeId: %d, objectId: %s", event.ChallengeId, event.ObjectId)
			err := v.daoManager.EventDao.UpdateEventStatusVerifyResultByChallengeId(event.ChallengeId, model.Verified, model.HashMismatched)
			return err
		}
		return err
	}, retry.Context(context.Background()), common.RtyAttem, common.RtyDelay, common.RtyErr)
	if err != nil {
		// after testing, we can make sure it is not client errors, treat it as sp side error
		logging.Logger.Errorf("failed to call storage api for challenge %d, err=%+v", event.ChallengeId, err.Error())
		return v.compareHashAndUpdate(event.ChallengeId, chainRootHash, []byte{})
	}

	pieceData, err := io.ReadAll(challengeRes.PieceData)
	piecesHash := challengeRes.PiecesHash
	if err != nil {
		logging.Logger.Errorf("verifier failed to read piece data for event %d, err=%+v", event.ChallengeId, err.Error())
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
	// TODO: remove after debugging
	originalSpRootHash := bytes.Join(checksums, []byte(""))
	logging.Logger.Infof("SpRootHash before replacing: %s for challengeId: %d", string(originalSpRootHash[:]), event.ChallengeId)
	spRootHash := v.computeRootHash(event.SegmentIndex, pieceData, spChecksums)
	logging.Logger.Infof("SpRootHash after replacing: %s for challengeId: %d", string(spRootHash[:]), event.ChallengeId)
	// Update database after comparing
	err = v.compareHashAndUpdate(event.ChallengeId, chainRootHash, spRootHash)
	logging.Logger.Infof("verifier completed time for challengeId: %d %s", event.ChallengeId, time.Now().Format("15:04:05.000000"))
	if err != nil {
		logging.Logger.Errorf("failed to update event status, challenge id: %d, err: %s",
			event.ChallengeId, err)
		return err
	}
	return nil
}

func (v *Verifier) preCheck(event *model.Event, currentHeight uint64) error {
	if event.ExpiredHeight < currentHeight {
		logging.Logger.Infof("verifier for challengeId: %d has expired. expired height: %d, current height: %d, timestamp: %s", event.ChallengeId, event.ExpiredHeight, currentHeight, time.Now().Format("15:04:05.000000"))
		return common.ErrEventExpired
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
	// TODO: Comment this if debugging
	if event.ChallengerAddress == "" && event.ChallengeId%heartbeatInterval != 0 && event.ChallengeId > v.deduplicationInterval {
		found, err := v.daoManager.EventDao.IsEventExistsBetween(event.ObjectId, event.SpOperatorAddress,
			event.ChallengeId-v.deduplicationInterval, event.ChallengeId-1)
		if err != nil {
			logging.Logger.Errorf("verifier failed to retrieve information for event %d, err=%+v", event.ChallengeId, err.Error())
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
	// TODO: Revert this if debugging
	if bytes.Equal(chainRootHash, spRootHash) {
		// return v.daoManager.EventDao.UpdateEventStatusVerifyResultByChallengeId(challengeId, model.Verified, model.HashMismatched)
		return v.daoManager.EventDao.UpdateEventStatusVerifyResultByChallengeId(challengeId, model.Verified, model.HashMatched)
	}
	return v.daoManager.EventDao.UpdateEventStatusVerifyResultByChallengeId(challengeId, model.Verified, model.HashMismatched)
}
