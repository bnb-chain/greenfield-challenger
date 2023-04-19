package vote

import "time"

const (
	ValidatorsCapacity = 256

	RetryInterval        = 1 * time.Second
	BroadcastInterval    = 10 * time.Second
	CollectVotesInterval = 5 * time.Second
	CollateVotesInterval = 2 * time.Second
	BatchSize            = 20 // to fetch records from database in batch
)
