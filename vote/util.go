package vote

import (
	sdkmath "cosmossdk.io/math"
	"encoding/binary"
	"encoding/hex"
	challengetypes "github.com/bnb-chain/greenfield/x/challenge/types"
	sdk "github.com/cosmos/cosmos-sdk/types"

	"github.com/bnb-chain/greenfield-challenger/logging"

	"github.com/ethereum/go-ethereum/common"
	"github.com/pkg/errors"
	"github.com/prysmaticlabs/prysm/crypto/bls"
	"github.com/prysmaticlabs/prysm/crypto/bls/blst"
	tmtypes "github.com/tendermint/tendermint/types"
	"github.com/willf/bitset"

	"github.com/bnb-chain/greenfield-challenger/db/model"
)

// getBlsPubKeyFromPrivKeyStr gets public key from bls private key
func getBlsPubKeyFromPrivKeyStr(privKeyStr string) []byte {
	privKey, err := blst.SecretKeyFromBytes(common.Hex2Bytes(privKeyStr))
	if err != nil {
		panic(err)
	}
	return privKey.PublicKey().Marshal()
}

// verifySignature verifies vote signature
func verifySignature(vote *model.Vote, eventHash []byte) error {
	blsPubKey, err := bls.PublicKeyFromBytes(vote.PubKey[:])
	if err != nil {
		return errors.Wrap(err, "convert public key from bytes to bls failed")
	}
	sig, err := bls.SignatureFromBytes([]byte(vote.Signature))
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
		voteAddrSet[string(v.PubKey)] = struct{}{}
		signatures = append(signatures, common.Hex2Bytes(v.Signature))
	}

	for idx, valInfo := range validators {
		if _, ok := voteAddrSet[hex.EncodeToString(valInfo.RelayerBlsKey[:])]; ok {
			valBitSet.Set(uint(idx))
		}
	}

	sigs, err := bls.MultipleSignaturesFromBytes(signatures)
	if err != nil {
		logging.Logger.Errorf("signature aggregator failed to generate multiple signatures from bytes, err=%s", err.Error())
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

	bs := make([]byte, 0)
	bs = append(bs, challengeIdBz...)
	bs = append(bs, objectIdBz...)
	bs = append(bs, resultBz...)
	bs = append(bs, []byte(event.SpOperatorAddress)...)
	bs = append(bs, []byte(event.ChallengerAddress)...)
	hash := sdk.Keccak256Hash(bs)
	return hash[:]
}
