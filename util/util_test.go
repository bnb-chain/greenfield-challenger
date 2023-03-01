package util

import (
	"encoding/hex"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/willf/bitset"
)

func TestGetBlsPubKeyFromPrivKeyStr(t *testing.T) {
	privKeyStr := "1afd9371ebe27dc75face6fb3602fc6d8b93bbd885d81bfcdac7ec2db8246f6f"
	expectedPubKeyStr := "8ec21505e290d7c15f789c7b4c522179bb7d70171319bfe2d6b2aae2461a1279566782907593cc526a5f2611c0721d60"
	pubKeyBts := BlsPubKeyFromPrivKeyStr(privKeyStr)
	require.Equal(t, expectedPubKeyStr, hex.EncodeToString(pubKeyBts))
}

func TestQuotedStrToInt(t *testing.T) {
	num, err := QuotedStrToIntWithBitSize("\"666666\"", 64)
	require.NoError(t, err)
	require.Equal(t, uint64(666666), num)
}

func TestBitSetToBigInt(t *testing.T) {
	valBitSet := bitset.New(256)
	valBitSet.Set(0)
	valBitSet.Set(1)
	valBitSet.Set(2)
	valBitSet.Set(9)
	valBitSet.Set(64)
	valBitSet.Set(128)
	valBitSet.Set(255)
	bigint := BitSetToBigInt(valBitSet)

	require.EqualValues(t, 1, bigint.Bit(0))
	require.EqualValues(t, 1, bigint.Bit(1))
	require.EqualValues(t, 1, bigint.Bit(2))
	require.EqualValues(t, 1, bigint.Bit(9))
	require.EqualValues(t, 1, bigint.Bit(64))
	require.EqualValues(t, 1, bigint.Bit(128))
	require.EqualValues(t, 1, bigint.Bit(255))
	require.NotEqual(t, 1, bigint.Bit(3))
}
