package verifier

import (
	"bytes"
	"testing"

	"github.com/bnb-chain/greenfield-common/go/hash"
	"github.com/stretchr/testify/require"
)

func TestHashing(t *testing.T) {
	verifier := NewHashVerifier(nil, nil, nil, 100, 100)

	hashesStr := []string{"test1", "test2", "test3", "test4", "test5", "test6", "test7"}
	checksums := make([][]byte, 7)
	for i, v := range hashesStr {
		checksums[i] = hash.CalcSHA256([]byte(v))
	}
	rootHash := bytes.Join(checksums, []byte(""))
	rootHash = []byte(hash.CalcSHA256Hex(rootHash))

	// Valid testcase
	validStr := []byte("test1")
	println(checksums[0])
	validRootHash, err := verifier.computeRootHash(0, validStr, checksums)
	require.NoError(t, err)
	require.Equal(t, validRootHash, rootHash)

	//s.verifier.compareHashAndUpdate(event.ChallengeId, validRootHash, rootHash)
	//updatedValidEvent, err := s.dao.GetEventByChallengeId(event.ChallengeId)
	//s.Require().NoError(err)
	//s.Require().Equal(updatedValidEvent.Status, model.VerifiedValidChallenge)

	// Invalid testcase
	invalidStr := []byte("invalid")
	invalidRootHash, err := verifier.computeRootHash(0, invalidStr, checksums)
	require.NoError(t, err)
	require.NotEqual(t, validRootHash, invalidRootHash)

	//s.verifier.compareHashAndUpdate(event.ChallengeId, invalidRootHash, rootHash)
	//updatedInvalidEvent, err := s.dao.GetEventByChallengeId(event.ChallengeId)
	//s.Require().NoError(err)
	//s.Require().Equal(updatedInvalidEvent.Status, model.VerifiedInvalidChallenge)
}
