package attest

import "time"

const (
	QueryAttestedChallengeInterval = 5 * time.Second                                       // query last attested challenge id
	MaxQueryCount                  = int((1 * time.Hour) / QueryAttestedChallengeInterval) // query last attested challenge id limit
)
