package wiper

import "time"

var (
	DBWipeInterval = 1 * time.Hour
	WipeBefore     = time.Now().Add(-1 * time.Hour).Unix()
)
