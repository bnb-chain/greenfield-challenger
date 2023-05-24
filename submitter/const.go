package submitter

import "time"

const (
	CacheSize            = 300
	TimeFormat           = "15:04:05.000000"
	TxSubmitLoopInterval = 5 * time.Second        // query last attested challenge id
	TxSubmitInterval     = 100 * time.Millisecond // query last attested challenge id
)
