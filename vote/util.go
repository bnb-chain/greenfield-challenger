package vote

import (
	"encoding/hex"

	"github.com/bnb-chain/greenfield-challenger/logging"

	"github.com/ethereum/go-ethereum/common"
	"github.com/pkg/errors"
	"github.com/prysmaticlabs/prysm/crypto/bls"
	"github.com/prysmaticlabs/prysm/crypto/bls/blst"
	tmtypes "github.com/tendermint/tendermint/types"
	"github.com/tendermint/tendermint/votepool"
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
func verifySignature(vote *votepool.Vote, eventHash []byte) error {
	blsPubKey, err := bls.PublicKeyFromBytes(vote.PubKey[:])
	if err != nil {
		return errors.Wrap(err, "convert public key from bytes to bls failed")
	}
	sig, err := bls.SignatureFromBytes(vote.Signature[:])
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
