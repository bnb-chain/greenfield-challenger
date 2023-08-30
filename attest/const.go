package attest

import "time"

const (
	QueryAttestedChallengeInterval = 10 * time.Second // query last attested challenge id
	MaxQueryCount                  = 3000
)
