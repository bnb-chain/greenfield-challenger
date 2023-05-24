package executor

import (
	"time"
)

const (
	UpdateCachedValidatorsInterval = 1 * time.Minute
	QueryHeartbeatIntervalInterval = 120 * time.Minute // blockchain challenge heartbeat interval only changed by governance

	VotePoolBroadcastMethodName   = "broadcast_vote"
	VotePoolBroadcastParameterKey = "vote"

	VotePoolQueryMethodName         = "query_vote"
	VotePoolQueryParameterEventType = "event_type"
	VotePoolQueryParameterEventHash = "event_hash"
)
