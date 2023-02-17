package keys

import (
	"encoding/hex"
	"encoding/json"
	"github.com/bnb-chain/greenfield-relayer/config"
	"github.com/ethereum/go-ethereum/common"
	"github.com/prysmaticlabs/prysm/crypto/bls/blst"

	"github.com/evmos/ethermint/crypto/ethsecp256k1"
	ethHd "github.com/evmos/ethermint/crypto/hd"
)

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
