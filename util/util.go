package util

import (
	"encoding/binary"
	"math/big"
	"strconv"

	"github.com/ethereum/go-ethereum/common"
	"github.com/prysmaticlabs/prysm/crypto/bls/blst"
	"github.com/willf/bitset"
)

func IndexOf(element string, data []string) int {
	for i, v := range data {
		if element == v {
			return i
		}
	}
	return -1
}

func BlsPubKeyFromPrivKeyStr(privKeyStr string) []byte {
	privKey, err := blst.SecretKeyFromBytes(common.Hex2Bytes(privKeyStr))
	if err != nil {
		panic(err)
	}
	return privKey.PublicKey().Marshal()
}

// QuotedStrToIntWithBitSize convert a QuoteStr ""6""  to int 6
func QuotedStrToIntWithBitSize(str string, bitSize int) (uint64, error) {
	s, err := strconv.Unquote(str)
	if err != nil {
		return 0, err
	}
	num, err := strconv.ParseInt(s, 10, bitSize)
	if err != nil {
		return 0, err
	}
	return uint64(num), nil
}

func BitSetToBigInt(set *bitset.BitSet) *big.Int {
	bts := make([]byte, 0)
	for i := len(set.Bytes()) - 1; i >= 0; i-- {
		bytes := Uint64ToBytes(set.Bytes()[i])
		bts = append(bts, bytes...)
	}
	return new(big.Int).SetBytes(bts)
}

func Uint16ToBytes(num uint16) []byte {
	bt := make([]byte, 2)
	binary.BigEndian.PutUint16(bt, num)
	return bt
}

func Uint64ToBytes(num uint64) []byte {
	bt := make([]byte, 8)
	binary.BigEndian.PutUint64(bt, num)
	return bt
}
