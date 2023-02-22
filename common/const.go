package common

import (
	"github.com/avast/retry-go/v4"
	"time"
)

var (
	RtyAttemNum = uint(5)
	RtyAttem    = retry.Attempts(RtyAttemNum)
	RtyDelay    = retry.Delay(time.Millisecond * 400)
	RtyErr      = retry.LastErrorOnly(true)
)
