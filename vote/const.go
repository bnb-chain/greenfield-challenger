package vote

import "time"

const (
	ValidatorsCapacity = 256

	RetryInterval         = 1 * time.Second
	QueryVotepoolMaxRetry = 5
)
