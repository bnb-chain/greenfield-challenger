package vote

import (
	"github.com/bnb-chain/greenfield-challenger/executor"
	"github.com/bnb-chain/greenfield-challenger/logging"
	"time"
)

type HeightTracker struct {
	executor *executor.Executor
}

func NewHeightTracker(executor *executor.Executor) *HeightTracker {
	return &HeightTracker{
		executor: executor,
	}
}

func (h *HeightTracker) GetHeightLoop() {
	for {
		err := h.getCurrentHeight()
		if err != nil {
			logging.Logger.Errorf("broadcaster cannot get current height, err=%s", err.Error())
			time.Sleep(RetryInterval)
		}
	}
}

// Updates the cached height in executor
func (h *HeightTracker) getCurrentHeight() error {
	_, err := h.executor.GetLatestBlockHeight()
	if err != nil {
		return err
	}
	return nil
}
