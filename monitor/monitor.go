package monitor

import (
	"strconv"
	"strings"
	"time"

	sdkmath "cosmossdk.io/math"
	"github.com/bnb-chain/greenfield-challenger/common"
	"github.com/bnb-chain/greenfield-challenger/db/model"
	"github.com/bnb-chain/greenfield-challenger/executor"
	"github.com/bnb-chain/greenfield-challenger/logging"
	challengetypes "github.com/bnb-chain/greenfield/x/challenge/types"
	abci "github.com/cometbft/cometbft/abci/types"
	ctypes "github.com/cometbft/cometbft/rpc/core/types"
	tmtypes "github.com/cometbft/cometbft/types"
	"gorm.io/gorm"
)

type Monitor struct {
	executor     *executor.Executor
	dataProvider DataProvider
}

func NewMonitor(executor *executor.Executor, dataProvider DataProvider) *Monitor {
	return &Monitor{
		executor:     executor,
		dataProvider: dataProvider,
	}
}

func (m Monitor) parseEvents(blockRes *ctypes.ResultBlockResults) ([]*challengetypes.EventStartChallenge, error) {
	events := make([]*challengetypes.EventStartChallenge, 0)
	for _, tx := range blockRes.TxsResults {
		for _, event := range tx.Events {
			e, err := m.parseEvent(event)
			if err != nil {
				return nil, err
			}
			if e != nil {
				events = append(events, e)
			}
		}
	}

	for _, event := range blockRes.EndBlockEvents {
		e, err := m.parseEvent(event)
		if err != nil {
			return nil, err
		}
		if e != nil {
			events = append(events, e)
		}
	}
	return events, nil
}

func (m Monitor) parseEvent(event abci.Event) (*challengetypes.EventStartChallenge, error) {
	if event.Type == "greenfield.challenge.EventStartChallenge" {
		challengeIdStr, objectIdStr, redundancyIndexStr, segmentIndexStr, spOpAddress, challengerAddress, expiredHeightStr := "", "", "", "", "", "", ""
		for _, attr := range event.Attributes {
			if string(attr.Key) == "challenge_id" {
				challengeIdStr = strings.Trim(string(attr.Value), `"`)
			} else if string(attr.Key) == "object_id" {
				objectIdStr = strings.Trim(string(attr.Value), `"`)
			} else if string(attr.Key) == "redundancy_index" {
				redundancyIndexStr = strings.Trim(string(attr.Value), `"`)
			} else if string(attr.Key) == "segment_index" {
				segmentIndexStr = strings.Trim(string(attr.Value), `"`)
			} else if string(attr.Key) == "sp_operator_address" {
				spOpAddress = strings.Trim(string(attr.Value), `"`)
			} else if string(attr.Key) == "challenger_address" {
				challengerAddress = strings.Trim(string(attr.Value), `"`)
			} else if string(attr.Key) == "expired_height" {
				expiredHeightStr = strings.Trim(string(attr.Value), `"`)
			}
		}
		challengeId, err := strconv.ParseInt(challengeIdStr, 10, 64)
		if err != nil {
			return nil, err
		}
		objectId := sdkmath.NewUintFromString(objectIdStr)
		redundancyIndex, err := strconv.ParseInt(redundancyIndexStr, 10, 32)
		if err != nil {
			return nil, err
		}
		segmentIndex, err := strconv.ParseInt(segmentIndexStr, 10, 32)
		if err != nil {
			return nil, err
		}
		expiredHeight, err := strconv.ParseInt(expiredHeightStr, 10, 64)
		if err != nil {
			return nil, err
		}
		return &challengetypes.EventStartChallenge{
			ChallengeId:       uint64(challengeId),
			ObjectId:          objectId,
			SegmentIndex:      uint32(segmentIndex),
			SpOperatorAddress: spOpAddress,
			RedundancyIndex:   int32(redundancyIndex),
			ChallengerAddress: challengerAddress,
			ExpiredHeight:     uint64(expiredHeight),
		}, nil
	}
	return nil, nil
}

func (m *Monitor) ListenEventLoop() {
	for {
		err := m.poll()
		if err != nil {
			time.Sleep(common.RetryInterval)
			continue
		}
	}
}

func (m *Monitor) poll() error {
	nextHeight, err := m.calNextHeight()
	if err != nil {
		return err
	}
	blockResults, block, err := m.getBlockAndBlockResult(nextHeight)
	if err != nil {
		return err
	}
	if err = m.monitorChallengeEvents(block, blockResults); err != nil {
		logging.Logger.Errorf("encounter error when monitor challenge events at blockHeight=%d, err=%+v", nextHeight, err.Error())
		return err
	}
	return nil
}

func (m *Monitor) getBlockAndBlockResult(height uint64) (*ctypes.ResultBlockResults, *tmtypes.Block, error) {
	logging.Logger.Infof("retrieve greenfield block at height=%d", height)
	block, blockResults, err := m.executor.GetBlockAndBlockResultAtHeight(int64(height))
	if err != nil {
		return nil, nil, err
	}
	return blockResults, block, nil
}

func (m *Monitor) monitorChallengeEvents(block *tmtypes.Block, blockResults *ctypes.ResultBlockResults) error {
	parsedEvents, err := m.parseEvents(blockResults)
	if err != nil {
		return err
	}
	b := &model.Block{
		Height:      uint64(block.Height),
		BlockTime:   block.Time.Unix(),
		CreatedTime: time.Now().Unix(),
	}
	events := EntitiesToDtos(uint64(block.Height), parsedEvents)
	err = m.dataProvider.SaveBlockAndEvents(b, events)
	for _, event := range events {
		logging.Logger.Debugf("monitor event saved for challengeId: %d %s", event.ChallengeId, time.Now().Format("15:04:05.000000"))
	}
	if err != nil {
		return err
	}
	return nil
}

func (m *Monitor) calNextHeight() (uint64, error) {
	latestPolledBlock, err := m.dataProvider.GetLatestBlock()
	if err != nil && err != gorm.ErrRecordNotFound {
		logging.Logger.Errorf("failed to get latest block from db, error: %s", err.Error())
		latestHeight, err := m.executor.GetLatestBlockHeight()
		if err != nil {
			logging.Logger.Errorf("failed to get latest block height, error: %s", err.Error())
			return 0, err
		}
		return latestHeight, err
	}
	if latestPolledBlock.Height == 0 { // a fresh database
		latestHeight, err := m.executor.GetLatestBlockHeight()
		if err != nil {
			logging.Logger.Errorf("failed to get latest block height, error: %s", err.Error())
			return m.executor.GetCachedBlockHeight(), err
		}
		return latestHeight, nil
	}
	nextHeight := latestPolledBlock.Height + 1

	latestBlockHeight, err := m.executor.GetLatestBlockHeight()
	if err != nil {
		logging.Logger.Errorf("failed to get latest block height, error: %s", err.Error())
		return 0, err
	}
	// pauses challenger for a bit since it already caught the newest block
	if int64(nextHeight) == int64(latestBlockHeight) {
		time.Sleep(common.RetryInterval)
		return nextHeight, nil
	}
	return nextHeight, nil
}
