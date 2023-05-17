package vote

import (
	"encoding/binary"
	"encoding/hex"

	sdkmath "cosmossdk.io/math"

	"github.com/bnb-chain/greenfield-challenger/db/model"
	"github.com/bnb-chain/greenfield-challenger/logging"
	challengetypes "github.com/bnb-chain/greenfield/x/challenge/types"
	tmtypes "github.com/cometbft/cometbft/types"
	"github.com/cometbft/cometbft/votepool"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/ethereum/go-ethereum/common"
	"github.com/pkg/errors"
	"github.com/prysmaticlabs/prysm/crypto/bls"
	"github.com/willf/bitset"
)

// verifySignature verifies vote signature
func verifySignature(vote *votepool.Vote, eventHash []byte) error {
	blsPubKey, err := bls.PublicKeyFromBytes(vote.PubKey)
	if err != nil {
		return errors.Wrap(err, "convert public key from bytes to bls failed")
	}
	sig, err := bls.SignatureFromBytes(vote.Signature)
	if err != nil {
		return errors.Wrap(err, "invalid signature")
	}
	if !sig.Verify(blsPubKey, eventHash[:]) {
		return errors.New("verify bls signature failed.")
	}
	return nil
}

// AggregateSignatureAndValidatorBitSet aggregates signature from multiple votes, and marks the bitset of validators who contribute votes
func AggregateSignatureAndValidatorBitSet(votes []*model.Vote, validators []*tmtypes.Validator) ([]byte, *bitset.BitSet, error) {
	signatures := make([][]byte, 0, len(votes))
	voteAddrSet := make(map[string]struct{}, len(votes))
	valBitSet := bitset.New(ValidatorsCapacity)
	for _, v := range votes {
		voteAddrSet[v.PubKey] = struct{}{}
		signatures = append(signatures, common.Hex2Bytes(v.Signature))
	}

	for idx, valInfo := range validators {
		if _, ok := voteAddrSet[hex.EncodeToString(valInfo.BlsKey[:])]; ok {
			valBitSet.Set(uint(idx))
		}
	}

	sigs, err := bls.MultipleSignaturesFromBytes(signatures)
	if err != nil {
		logging.Logger.Errorf("signature aggregator failed to generate multiple signatures from bytes, err=%+v", err.Error())
		return nil, valBitSet, err
	}
	return bls.AggregateSignatures(sigs).Marshal(), valBitSet, nil
}

func CalculateEventHash(event *model.Event) []byte {
	challengeIdBz := make([]byte, 8)
	binary.BigEndian.PutUint64(challengeIdBz, event.ChallengeId)
	objectIdBz := sdkmath.NewUintFromString(event.ObjectId).Bytes()
	resultBz := make([]byte, 8)
	if event.VerifyResult == model.HashMismatched {
		binary.BigEndian.PutUint64(resultBz, uint64(challengetypes.CHALLENGE_SUCCEED))
	} else if event.VerifyResult == model.HashMatched {
		binary.BigEndian.PutUint64(resultBz, uint64(challengetypes.CHALLENGE_FAILED))
	} else {
		panic("cannot convert vote option")
	}

	spOperatorBz := sdk.MustAccAddressFromHex(event.SpOperatorAddress).Bytes()
	challengerBz := make([]byte, 0)
	if event.ChallengerAddress != "" {
		challengerBz = sdk.MustAccAddressFromHex(event.ChallengerAddress).Bytes()
	}

	bs := make([]byte, 0)
	bs = append(bs, challengeIdBz...)
	bs = append(bs, objectIdBz...)
	bs = append(bs, resultBz...)
	bs = append(bs, spOperatorBz...)
	bs = append(bs, challengerBz...)
	hash := sdk.Keccak256Hash(bs)
	return hash[:]
}
