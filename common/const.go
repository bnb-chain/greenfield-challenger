package common

import (
	"github.com/avast/retry-go/v4"
	"time"
)

var (
	RetryAttemptNum = uint(5)
	RetryAttempts   = retry.Attempts(RetryAttemptNum)
	RetryDelay      = retry.Delay(time.Millisecond * 400)
	RetryErr        = retry.LastErrorOnly(true)
)
