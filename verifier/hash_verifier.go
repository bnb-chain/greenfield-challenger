package verifier

import (
	"bytes"
	"context"
	"encoding/hex"
	"golang.org/x/sync/semaphore"
	"io"
	"strings"
	"sync"
	"time"

	"github.com/avast/retry-go/v4"
	"github.com/bnb-chain/greenfield-challenger/common"
	"github.com/bnb-chain/greenfield-challenger/config"
	"github.com/bnb-chain/greenfield-challenger/db/model"
	"github.com/bnb-chain/greenfield-challenger/executor"
	"github.com/bnb-chain/greenfield-challenger/logging"
	"github.com/bnb-chain/greenfield-common/go/hash"
	"github.com/bnb-chain/greenfield-go-sdk/types"
	"github.com/panjf2000/ants/v2"
)

type Verifier struct {
	config                *config.Config
	executor              *executor.Executor
	deduplicationInterval uint64
	cachedChallengeIds    map[uint64]bool
	mtx                   sync.RWMutex
	dataProvider          DataProvider
	limiterSemaphore      *semaphore.Weighted
}

func NewHashVerifier(cfg *config.Config, executor *executor.Executor,
	deduplicationInterval uint64, dataProvider DataProvider,
) *Verifier {
	limiterSemaphore := semaphore.NewWeighted(20)
	return &Verifier{
		config:                cfg,
		executor:              executor,
		deduplicationInterval: deduplicationInterval,
		mtx:                   sync.RWMutex{},
		dataProvider:          dataProvider,
		limiterSemaphore:      limiterSemaphore,
	}
}

func (v *Verifier) VerifyHashLoop() {
	// Event lasts for 300 blocks, 2x for redundancy
	v.cachedChallengeIds = make(map[uint64]bool, common.CacheSize)

	pool, err := ants.NewPool(30)
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
	events, err := v.dataProvider.FetchEventsForVerification(currentHeight)

	// TODO: Remove after debugging
	fetchedEvents := []uint64{}
	for _, v := range events {
		fetchedEvents = append(fetchedEvents, v.ChallengeId)
	}
	logging.Logger.Infof("verifier fetched these events for verification: %+v", fetchedEvents)

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
			logging.Logger.Infof("challengeId: %d is cached", event.ChallengeId)
			continue
		}

		logging.Logger.Infof("challengeId: %d is not cached", event.ChallengeId)

		if err = v.limiterSemaphore.Acquire(context.Background(), 1); err != nil {
			logging.Logger.Errorf("failed to acquire semaphore: %v", err)
			continue
		}
		go func(event *model.Event) {
			defer v.limiterSemaphore.Release(1)
			// Call verifyForSingleEvent inside the goroutine
			err = v.verifyForSingleEvent(event)
		}(event)

		if err != nil {
			if err.Error() == common.ErrEventExpired.Error() {
				v.mtx.Lock()
				delete(v.cachedChallengeIds, event.ChallengeId)
				v.mtx.Unlock()
				continue
			}
			logging.Logger.Errorf("verifier failed to verify challengeId: %d, err=%+v", event.ChallengeId, err.Error())
		}

		if !isCached {
			v.mtx.Lock()
			v.cachedChallengeIds[event.ChallengeId] = true
			v.mtx.Unlock()
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
	if err != nil {
		logging.Logger.Errorf("verifier failed to get sp endpoint for challengeId: %s, objectId: %s, err=%+v", err.Error(), event.ChallengeId, event.ObjectId)
		return err
	}
	logging.Logger.Infof("challengeId: %d, sp endpoint: %s, objectId: %s, segmentIndex: %d, redundancyIndex: %d", event.ChallengeId, endpoint, event.ObjectId, event.SegmentIndex, event.RedundancyIndex)

	// Call blockchain for object info to get original hash
	checksums, err := v.executor.GetObjectInfoChecksums(event.ObjectId)
	if err != nil {
		if strings.Contains(err.Error(), "No such object") {
			logging.Logger.Errorf("No such object error for challengeId: %d", event.ChallengeId)
		}
		return err
	}
	chainRootHash := checksums[event.RedundancyIndex+1]
	logging.Logger.Infof("chainRootHash: %s for challengeId: %d", hex.EncodeToString(chainRootHash), event.ChallengeId)

	// Call sp for challenge result
	challengeRes := &types.ChallengeResult{}
	var challengeResErr error
	err = retry.Do(func() error {
		challengeRes, challengeResErr = v.executor.GetChallengeResultFromSp(event.ObjectId, endpoint, int(event.SegmentIndex), int(event.RedundancyIndex))
		if challengeResErr != nil {
			// TODO: Create error code list for SP side
			logging.Logger.Errorf("error getting challenge result from sp for challengeId: %d, objectId: %s, err=%s", event.ChallengeId, event.ObjectId, err.Error())
		}
		return challengeResErr
	}, retry.Context(context.Background()), common.RtyAttem, common.RtyDelay, common.RtyErr)
	if challengeResErr != nil {
		err = v.dataProvider.UpdateEventStatusVerifyResult(event.ChallengeId, model.Verified, model.HashMismatched)
		if err != nil {
			logging.Logger.Errorf("error updating event status for challengeId: %d", event.ChallengeId)
		}
		return err
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
	originalSpRootHash := hash.GenerateChecksum(bytes.Join(spChecksums, []byte("")))
	logging.Logger.Infof("SpRootHash before replacing: %s for challengeId: %d", hex.EncodeToString(originalSpRootHash), event.ChallengeId)
	spRootHash := v.computeRootHash(event.SegmentIndex, pieceData, spChecksums)
	logging.Logger.Infof("SpRootHash after replacing: %s for challengeId: %d", hex.EncodeToString(spRootHash), event.ChallengeId)
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
		found, err := v.dataProvider.IsEventExistsBetween(event.ObjectId, event.SpOperatorAddress,
			event.ChallengeId-v.deduplicationInterval, event.ChallengeId-1)
		if err != nil {
			logging.Logger.Errorf("verifier failed to retrieve information for event %d, err=%+v", event.ChallengeId, err.Error())
			return err
		}
		if found {
			return v.dataProvider.UpdateEventStatus(event.ChallengeId, model.Duplicated)
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
		//return v.dataProvider.UpdateEventStatusVerifyResult(challengeId, model.Verified, model.HashMismatched)
		return v.dataProvider.UpdateEventStatusVerifyResult(challengeId, model.Verified, model.HashMatched)
	}
	return v.dataProvider.UpdateEventStatusVerifyResult(challengeId, model.Verified, model.HashMismatched)
}
