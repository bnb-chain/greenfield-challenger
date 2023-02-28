package monitor

import (
	"strconv"
	"strings"
	"time"

	"github.com/bnb-chain/greenfield-challenger/common"
	"github.com/bnb-chain/greenfield-challenger/db/dao"
	"github.com/bnb-chain/greenfield-challenger/db/model"
	"github.com/bnb-chain/greenfield-challenger/executor"
	"github.com/bnb-chain/greenfield-challenger/logging"
	challengetypes "github.com/bnb-chain/greenfield/x/challenge/types"
	abci "github.com/tendermint/tendermint/abci/types"
	ctypes "github.com/tendermint/tendermint/rpc/core/types"
	"github.com/tendermint/tendermint/rpc/jsonrpc/client"
	tmtypes "github.com/tendermint/tendermint/types"
	"gorm.io/gorm"
)

type Monitor struct {
	client.Client
	daoManager dao.DaoManager
	executor   executor.Executor
}

func NewMonitor(c client.Client, daoManager dao.DaoManager) *Monitor {
	return &Monitor{
		Client:     c,
		daoManager: daoManager,
	}
}

func (m Monitor) Start() {

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
	if event.Type == "bnbchain.greenfield.sp.Event" {
		challengeIdStr, objectIdStr, redundancyIndexStr, segmentIndexStr, spOpAddress := "", "", "", "", ""
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
			}
		}
		challengeId, _ := strconv.ParseInt(challengeIdStr, 10, 64)
		objectId, _ := strconv.ParseInt(objectIdStr, 10, 64)
		redundancyIndex, _ := strconv.ParseInt(redundancyIndexStr, 10, 32)
		segmentIndex, _ := strconv.ParseInt(segmentIndexStr, 10, 32)
		return &challengetypes.EventStartChallenge{
			ChallengeId:       uint64(challengeId),
			ObjectId:          uint64(objectId),
			SegmentIndex:      uint32(segmentIndex),
			SpOperatorAddress: spOpAddress,
			RedundancyIndex:   int32(redundancyIndex),
		}
	}
	return nil
}

func (l *Monitor) StartLoop() {
	for {
		err := l.poll()
		if err != nil {
			time.Sleep(common.RetryInterval)
			continue
		}
	}
}

func (l *Monitor) poll() error {
	nextHeight, err := l.calNextHeight()
	if err != nil {
		return err
	}
	blockResults, block, err := l.getBlockAndBlockResult(nextHeight)
	if err != nil {
		return err
	}
	if err = l.monitorChallengeEvents(block, blockResults); err != nil {
		logging.Logger.Errorf("encounter error when monitor cross-chain events at blockHeight=%d, err=%s", nextHeight, err.Error())
		return err
	}
	return nil
}

func (l *Monitor) getLatestPolledBlock() (*model.Block, error) {
	block, err := l.daoManager.GetLatestBlock()
	if err != nil {
		return nil, err
	}
	return block, nil
}

func (l *Monitor) getBlockAndBlockResult(height uint64) (*ctypes.ResultBlockResults, *tmtypes.Block, error) {
	logging.Logger.Infof("retrieve greenfield block at height=%d", height)
	blockResults, err := l.executor.GetBlockResultAtHeight(int64(height))
	if err != nil {
		return nil, nil, err
	}
	block, err := l.executor.GetBlockAtHeight(int64(height))
	if err != nil {
		return nil, nil, err
	}
	return blockResults, block, nil
}

func (l *Monitor) monitorChallengeEvents(block *tmtypes.Block, blockResults *ctypes.ResultBlockResults) error {
	events := l.parseEvents(blockResults)

	b := &model.Block{
		Height:    uint64(block.Height),
		BlockTime: block.Time.Unix(),
	}
	return l.daoManager.SaveBlockAndEvents(b, EntitiesToDtos(uint64(block.Height), events))
}

func (l *Monitor) calNextHeight() (uint64, error) {
	latestPolledBlock, err := l.getLatestPolledBlock()
	if err != nil && err != gorm.ErrRecordNotFound {
		logging.Logger.Errorf("failed to get latest block from db, error: %s", err.Error())
		return 0, err
	}
	nextHeight := latestPolledBlock.Height

	if nextHeight == 0 {
		nextHeight = 1
	}

	latestBlockHeight, err := l.executor.GetLatestBlockHeightWithRetry()
	if err != nil {
		logging.Logger.Errorf("failed to get latest block height, error: %s", err.Error())
		return 0, err
	}
	// pauses relayer for a bit since it already caught the newest block
	if int64(nextHeight) == int64(latestBlockHeight) {
		time.Sleep(common.RetryInterval)
		return nextHeight, nil
	}
	return nextHeight, nil
}
