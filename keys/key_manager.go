package keys

import (
	"encoding/hex"
	"fmt"
	"github.com/bnb-chain/greenfield-relayer/config"
	"github.com/cosmos/cosmos-sdk/crypto/hd"
	ctypes "github.com/cosmos/cosmos-sdk/crypto/types"
	"github.com/ethereum/go-ethereum/common"
	"github.com/evmos/ethermint/crypto/ethsecp256k1"
	ethHd "github.com/evmos/ethermint/crypto/hd"
	"github.com/prysmaticlabs/prysm/crypto/bls/blst"

	"strings"

	"github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/go-bip39"
)

const (
	defaultBIP39Passphrase = ""
	FullPath               = "m/44'/60'/0'/0/0"
)

type KeyManager interface {
	GetPrivKey() ctypes.PrivKey
	GetAddr() types.AccAddress
}

type keyManager struct {
	privKey  ctypes.PrivKey
	addr     types.AccAddress
	mnemonic string
}

func NewPrivateKeyManager(priKey string) (KeyManager, error) {
	k := keyManager{}
	err := k.recoveryFromPrivateKey(priKey)
	return &k, err
}

func NewMnemonicKeyManager(mnemonic string) (KeyManager, error) {
	k := keyManager{}
	err := k.recoveryFromMnemonic(mnemonic, FullPath)
	return &k, err
}

//TODO NewKeyStoreKeyManager to be implemented

func (m *keyManager) recoveryFromPrivateKey(privateKey string) error {
	priBytes, err := hex.DecodeString(privateKey)
	if err != nil {
		return err
	}

	if len(priBytes) != 32 {
		return fmt.Errorf("Len of Keybytes is not equal to 32 ")
	}
	var keyBytesArray [32]byte
	copy(keyBytesArray[:], priBytes[:32])
	priKey := ethHd.EthSecp256k1.Generate()(keyBytesArray[:]).(*ethsecp256k1.PrivKey)
	addr := types.AccAddress(priKey.PubKey().Address())
	m.addr = addr
	m.privKey = priKey
	return nil
}

func (m *keyManager) recoveryFromMnemonic(mnemonic, keyPath string) error {
	words := strings.Split(mnemonic, " ")
	if len(words) != 12 && len(words) != 24 {
		return fmt.Errorf("mnemonic length should either be 12 or 24")
	}
	seed, err := bip39.NewSeedWithErrorChecking(mnemonic, defaultBIP39Passphrase)
	if err != nil {
		return err
	}
	// create master key and derive first key:
	masterPriv, ch := hd.ComputeMastersFromSeed(seed)
	derivedPriv, err := hd.DerivePrivateKeyForPath(masterPriv, ch, keyPath)
	if err != nil {
		return err
	}
	priKey := ethHd.EthSecp256k1.Generate()(derivedPriv[:]).(*ethsecp256k1.PrivKey)
	addr := types.AccAddress(priKey.PubKey().Address())
	m.addr = addr
	m.privKey = priKey
	m.mnemonic = mnemonic
	return nil
}

func (m *keyManager) GetPrivKey() ctypes.PrivKey {
	return m.privKey
}

func (m *keyManager) GetAddr() types.AccAddress {
	return m.addr
}

func getInscriptionPrivateKey() *ethsecp256k1.PrivKey {
	var privateKey string
	if cfg.KeyType == config.KeyTypeAWSPrivateKey {
		result, err := config.GetSecret(cfg.AWSSecretName, cfg.AWSRegion)
		if err != nil {
			panic(err)
		}
		type AwsPrivateKey struct {
			PrivateKey string `json:"private_key"`
		}
		var awsPrivateKey AwsPrivateKey
		err = json.Unmarshal([]byte(result), &awsPrivateKey)
		if err != nil {
			panic(err)
		}
		privateKey = awsPrivateKey.PrivateKey
	} else {
		privateKey = cfg.PrivateKey
	}
	privKey, err := HexToEthSecp256k1PrivKey(privateKey)
	if err != nil {
		panic(err)
	}
	return privKey
}

func GetBlsPubKeyFromPrivKeyStr(privKeyStr string) []byte {
	privKey, err := blst.SecretKeyFromBytes(common.Hex2Bytes(privKeyStr))
	if err != nil {
		panic(err)
	}
	return privKey.PublicKey().Marshal()
}

func HexToEthSecp256k1PrivKey(hexString string) (*ethsecp256k1.PrivKey, error) {
	bz, err := hex.DecodeString(hexString)
	if err != nil {
		return nil, err
	}
	return ethHd.EthSecp256k1.Generate()(bz).(*ethsecp256k1.PrivKey), nil
}
