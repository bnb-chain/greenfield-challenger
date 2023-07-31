package common

import (
	"time"

	"github.com/avast/retry-go/v4"
)

var (
	RtyAttemNum              = uint(2)
	RtyAttem                 = retry.Attempts(RtyAttemNum)
	RtyDelay                 = retry.Delay(time.Millisecond * 500)
	RtyErr                   = retry.LastErrorOnly(true)
	RetryInterval            = 1 * time.Second
	CacheClearIterations     = 100
	CacheSize                = 600
	MaxSubmitAttempts        = 5
	MaxCheckAttestedAttempts = 20
)
