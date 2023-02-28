package common

import (
	"time"

	"github.com/avast/retry-go/v4"
)

var (
	RtyAttemNum   = uint(5)
	RtyAttem      = retry.Attempts(RtyAttemNum)
	RtyDelay      = retry.Delay(time.Millisecond * 400)
	RtyErr        = retry.LastErrorOnly(true)
	RetryInterval = 1 * time.Second
)
