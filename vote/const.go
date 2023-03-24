package vote

import "time"

const (
	ValidatorsCapacity = 256

	RetryInterval         = 1 * time.Second
	QueryVotepoolMaxRetry = 5
	CollectVotesInterval  = 2 * time.Second
)
