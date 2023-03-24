package vote

import "time"

const (
	ValidatorsCapacity = 256

	RetryInterval         = 1 * time.Second
	QueryVotepoolMaxRetry = 5
	CollectVotesInterval  = 10 * time.Second
)
