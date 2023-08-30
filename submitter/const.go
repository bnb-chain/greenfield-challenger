package submitter

import "time"

const (
	TimeFormat           = "15:04:05.00"
	TxSubmitLoopInterval = 5 * time.Second        // query last attested challenge id
	TxSubmitInterval     = 100 * time.Millisecond // query last attested challenge id
)
