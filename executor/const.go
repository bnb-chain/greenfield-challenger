package executor

import (
	"time"
)

const (
	FallBehindThreshold            = 5
	SleepSecondForUpdateClient     = 10
	DataSeedDenyServiceThreshold   = 60
	RPCTimeout                     = 3 * time.Second
	UpdateCachedValidatorsInterval = 1 * time.Minute
)
