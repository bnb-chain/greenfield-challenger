package monitor

import (
	"strconv"
	"strings"
	"time"

	sdkmath "cosmossdk.io/math"
	"github.com/bnb-chain/greenfield-challenger/common"
	"github.com/bnb-chain/greenfield-challenger/db/dao"
	"github.com/bnb-chain/greenfield-challenger/db/model"
	"github.com/bnb-chain/greenfield-challenger/executor"
	"github.com/bnb-chain/greenfield-challenger/logging"
	challengetypes "github.com/bnb-chain/greenfield/x/challenge/types"
	abci "github.com/tendermint/tendermint/abci/types"
	ctypes "github.com/tendermint/tendermint/rpc/core/types"
	tmtypes "github.com/tendermint/tendermint/types"
	"gorm.io/gorm"
)

type Monitor struct {
	executor   *executor.Executor
	daoManager *dao.DaoManager
}

func NewMonitor(executor *executor.Executor, daoManager *dao.DaoManager) *Monitor {
	return &Monitor{
		executor:   executor,
		daoManager: daoManager,
	}
}

func (m Monitor) parseEvents(blockRes *ctypes.ResultBlockResults) []*challengetypes.EventStartChallenge {
	events := make([]*challengetypes.EventStartChallenge, 0)
	for _, tx := range blockRes.TxsResults {
		for _, event := range tx.Events {
			e := m.parseEvent(event)
			if e != nil {
				events = append(events, e)
			}
		}
	}

	for _, event := range blockRes.EndBlockEvents {
		e := m.parseEvent(event)
		if e != nil {
			events = append(events, e)
		}
	}
	return events
}

func (m Monitor) parseEvent(event abci.Event) *challengetypes.EventStartChallenge {
	if event.Type == "bnbchain.greenfield.challenge.EventStartChallenge" {
		challengeIdStr, objectIdStr, redundancyIndexStr, segmentIndexStr, spOpAddress, challengerAddress := "", "", "", "", "", ""
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
				spOpAddress = strings.Trim(string(attr.Value), `"`)
			}
		}
		challengeId, err := strconv.ParseInt(challengeIdStr, 10, 64)
		if err != nil {
			panic(err)
		}
		objectId := sdkmath.NewUintFromString(objectIdStr)
		redundancyIndex, err := strconv.ParseInt(redundancyIndexStr, 10, 32)
		if err != nil {
			panic(err)
		}
		segmentIndex, err := strconv.ParseInt(segmentIndexStr, 10, 32)
		if err != nil {
			panic(err)
		}
		return &challengetypes.EventStartChallenge{
			ChallengeId:       uint64(challengeId),
			ObjectId:          objectId,
			SegmentIndex:      uint32(segmentIndex),
			SpOperatorAddress: spOpAddress,
			RedundancyIndex:   int32(redundancyIndex),
			ChallengerAddress: challengerAddress,
		}
	}
	return nil
}

func (m *Monitor) StartLoop() {
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
		logging.Logger.Errorf("encounter error when monitor cross-chain events at blockHeight=%d, err=%s", nextHeight, err.Error())
		return err
	}
	return nil
}

func (m *Monitor) getLatestPolledBlock() (*model.Block, error) {
	block, err := m.daoManager.GetLatestBlock()
	if err != nil {
		return nil, err
	}
	return block, nil
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
	events := m.parseEvents(blockResults)
	b := &model.Block{
		Height:    uint64(block.Height),
		BlockTime: block.Time.Unix(),
	}
	return m.daoManager.SaveBlockAndEvents(b, EntitiesToDtos(uint64(block.Height), events))
}

func (m *Monitor) calNextHeight() (uint64, error) {
	latestPolledBlock, err := m.getLatestPolledBlock()
	if err != nil && err != gorm.ErrRecordNotFound {
		logging.Logger.Errorf("failed to get latest block from db, error: %s", err.Error())
		return 0, err
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
