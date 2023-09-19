package verifier

import (
	"bytes"
	"context"
	"encoding/hex"
	lru "github.com/hashicorp/golang-lru"
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
	"github.com/bnb-chain/greenfield-challenger/metrics"
	"github.com/bnb-chain/greenfield-common/go/hash"
	"github.com/bnb-chain/greenfield-go-sdk/types"
)

type Verifier struct {
	config                *config.Config
	executor              *executor.Executor
	deduplicationInterval uint64
	processedEvent        *lru.Cache
	mtx                   sync.RWMutex
	dataProvider          DataProvider
	metricService         *metrics.MetricService
	wg                    sync.WaitGroup
	eventChannel          chan *model.Event
}

func NewHashVerifier(cfg *config.Config, executor *executor.Executor, dataProvider DataProvider, metricService *metrics.MetricService,
) *Verifier {
	deduplicationInterval, err := executor.QueryChallengeSlashCoolingOffPeriod()
	if err != nil {
		logging.Logger.Errorf("verifier failed to query slash cooling off period, err=%+v", err)
	}

	lruCache, err := lru.New(1000)
	if err != nil {
		logging.Logger.Errorf("verifier failed to create cache, err=%+v", err)
	}

	return &Verifier{
		config:                cfg,
		executor:              executor,
		deduplicationInterval: deduplicationInterval,
		processedEvent:        lruCache,
		mtx:                   sync.RWMutex{},
		dataProvider:          dataProvider,
		metricService:         metricService,
		eventChannel:          make(chan *model.Event, 100),
	}
}

func (v *Verifier) Start() {
	go v.startFetcher()
	go v.startWorkers()
}

func (v *Verifier) startFetcher() {
	ticker := time.NewTicker(10000)
	for range ticker.C {
		currentHeight := v.executor.GetCachedBlockHeight()
		events, err := v.dataProvider.FetchEventsForVerification(currentHeight)
		if err != nil {
			logging.Logger.Errorf("error fetching events for verification, err=%+v", err.Error())
			v.metricService.IncDBErr(0, err)
			continue
		}

		for _, event := range events {
			if !v.isProcessed(event) {
				v.eventChannel <- event
				v.addCache(event.ChallengeId)
			}
		}
	}
}

func (v *Verifier) startWorkers() {
	for event := range v.eventChannel {
		err := v.verifyForSingleEvent(event)
		if err != nil {
			continue
		}
	}
}

func (v *Verifier) isProcessed(event *model.Event) bool {
	if !v.isCached(event.ChallengeId) || event.Status == model.Unprocessed || event.VerifyResult == model.Unknown {
		return false
	}
	return true
}

func (v *Verifier) isCached(challengeId uint64) bool {
	v.mtx.Lock()
	isCached := v.processedEvent.Contains(challengeId)
	v.mtx.Unlock()
	return isCached
}

func (v *Verifier) addCache(challengeId uint64) {
	v.mtx.Lock()
	v.processedEvent.Add(challengeId, true)
	v.mtx.Unlock()
}

func (v *Verifier) removeCache(challengeId uint64) {
	v.mtx.Lock()
	v.processedEvent.Remove(challengeId)
	v.mtx.Unlock()
}

func (v *Verifier) verifyForSingleEvent(event *model.Event) error {
	startTime := time.Now()
	logging.Logger.Infof("verifier started for challengeId: %d %s", event.ChallengeId, time.Now().Format("15:04:05.000000"))

	currentHeight := v.executor.GetCachedBlockHeight()
	if err := v.preCheck(event, currentHeight); err != nil {
		return err
	}

	// Call blockchain for sp endpoint
	endpoint, err := v.fetchSpEndpoint(event)
	if err != nil {
		return err
	}

	// Call blockchain for object info to get original hash
	chainRootHash, err := v.fetchChainRootHash(event)
	if err != nil {
		return err
	}

	// Call sp for challenge result
	spRootHash, err := v.fetchSpRootHash(event, endpoint)
	if err != nil {
		return err
	}

	// Update database after comparing
	err = v.compareHashAndUpdate(event.ChallengeId, chainRootHash, spRootHash)
	if err != nil {
		v.eventChannel <- event
		return err
	}
	// Log duration
	elaspedTime := time.Since(startTime)
	v.metricService.SetHashVerifierDuration(elaspedTime)
	logging.Logger.Infof("verifier completed time for challengeId: %d %s", event.ChallengeId, time.Now().Format("15:04:05.000000"))
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
			v.metricService.IncDBErr(event.ChallengeId, err)
			v.metricService.IncHashVerifierErr(err)
			logging.Logger.Errorf("verifier failed to retrieve information for event %d, err=%+v", event.ChallengeId, err.Error())
			return err
		}
		if found {
			err = v.dataProvider.UpdateEventStatus(event.ChallengeId, model.Duplicated)
			if err != nil {
				v.metricService.IncDBErr(event.ChallengeId, err)
				v.metricService.IncHashVerifierErr(err)
				return err
			}
			return err
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
		// TODO: Revert this if debugging
		err := v.dataProvider.UpdateEventStatusVerifyResult(challengeId, model.Verified, model.HashMatched)
		if err != nil {
			logging.Logger.Errorf("failed to update event status, challenge id: %d, err: %s", challengeId, err)
			v.metricService.IncDBErr(challengeId, err)
			v.metricService.IncHashVerifierErr(err)
			return err
		}
		// update metrics if no err
		v.metricService.IncVerifiedChallenges()
		v.metricService.IncChallengeFailed()
		return err
	}
	err := v.dataProvider.UpdateEventStatusVerifyResult(challengeId, model.Verified, model.HashMismatched)
	if err != nil {
		logging.Logger.Errorf("failed to update event status, challenge id: %d, err: %s", challengeId, err)
		v.metricService.IncDBErr(challengeId, err)
		v.metricService.IncHashVerifierErr(err)
		return err
	}
	// update metrics if no err
	v.metricService.IncVerifiedChallenges()
	v.metricService.IncChallengeSuccess()
	return err
}

func (v *Verifier) fetchSpEndpoint(event *model.Event) (string, error) {
	endpoint, err := v.executor.GetStorageProviderEndpoint(event.SpOperatorAddress)
	if err != nil {
		logging.Logger.Errorf("verifier failed to get sp endpoint for challengeId: %s, objectId: %s, err=%+v", err.Error(), event.ChallengeId, event.ObjectId)
		v.metricService.IncGnfdChainErr(event.ChallengeId, err)
		return "", err
	}
	logging.Logger.Infof("challengeId: %d, sp endpoint: %s, objectId: %s, segmentIndex: %d, redundancyIndex: %d", event.ChallengeId, endpoint, event.ObjectId, event.SegmentIndex, event.RedundancyIndex)
	return endpoint, err
}

func (v *Verifier) fetchChainRootHash(event *model.Event) ([]byte, error) {
	checksums, err := v.executor.GetObjectInfoChecksums(event.ObjectId)
	if err != nil {
		if strings.Contains(err.Error(), "No such object") {
			logging.Logger.Errorf("No such object error for challengeId: %d", event.ChallengeId)
		}
		v.metricService.IncGnfdChainErr(event.ChallengeId, err)
		return nil, err
	}
	chainRootHash := checksums[event.RedundancyIndex+1]
	logging.Logger.Infof("chainRootHash: %s for challengeId: %d", hex.EncodeToString(chainRootHash), event.ChallengeId)
	return chainRootHash, err
}

func (v *Verifier) fetchSpRootHash(event *model.Event, spEndpoint string) ([]byte, error) {
	challengeRes := &types.ChallengeResult{}
	var challengeResErr error
	_ = retry.Do(func() error {
		challengeRes, challengeResErr = v.executor.GetChallengeResultFromSp(event.ObjectId, spEndpoint, int(event.SegmentIndex), int(event.RedundancyIndex))
		if challengeResErr != nil {
			// TODO: Create error code list for SP side
			logging.Logger.Errorf("error getting challenge result from sp for challengeId: %d, objectId: %s, err=%s", event.ChallengeId, event.ObjectId, challengeResErr.Error())
		}
		return challengeResErr
	}, retry.Context(context.Background()), common.RtyAttem, common.RtyDelay, common.RtyErr)
	if challengeResErr != nil {
		v.metricService.IncHashVerifierSpApiErr(event.ChallengeId, challengeResErr)
		err := v.dataProvider.UpdateEventStatusVerifyResult(event.ChallengeId, model.Verified, model.HashMismatched)
		if err != nil {
			v.metricService.IncHashVerifierErr(err)
			v.metricService.IncDBErr(event.ChallengeId, err)
			logging.Logger.Errorf("error updating event status for challengeId: %d", event.ChallengeId)
		}
		v.metricService.IncVerifiedChallenges()
		v.metricService.IncChallengeSuccess()
		return nil, err
	}

	pieceData, err := io.ReadAll(challengeRes.PieceData)
	piecesHash := challengeRes.PiecesHash
	if err != nil {
		logging.Logger.Errorf("verifier failed to read piece data for event %d, err=%+v", event.ChallengeId, err.Error())
		return nil, err
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
	return spRootHash, err
}
