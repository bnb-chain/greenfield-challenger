package executor

import (
	"context"
	"encoding/hex"
	"encoding/json"
	_ "encoding/json"
	"fmt"
	"github.com/bnb-chain/greenfield-go-sdk/client/sp"
	"time"

	"github.com/bnb-chain/greenfield-challenger/config"
	"github.com/bnb-chain/greenfield-challenger/logging"
	sdkclient "github.com/bnb-chain/greenfield-go-sdk/client/chain"
	sdkkeys "github.com/bnb-chain/greenfield-go-sdk/keys"
	challangetypes "github.com/bnb-chain/greenfield/x/challenge/types"
	"github.com/cosmos/cosmos-sdk/codec"
	sdk "github.com/cosmos/cosmos-sdk/types"
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"
	"github.com/tendermint/tendermint/rpc/client"
	ctypes "github.com/tendermint/tendermint/rpc/core/types"
	tmtypes "github.com/tendermint/tendermint/types"
	"github.com/tendermint/tendermint/votepool"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

type Executor struct {
	gnfdClients *sdkclient.GnfdCompositeClients
	spClient    *sp.SPClient
	config      *config.Config
	address     string
	validators  []*tmtypes.Validator // used to cache validators
	cdc         *codec.ProtoCodec
}

func NewExecutor(cfg *config.Config) *Executor {
	privKey := getGreenfieldPrivateKey(&cfg.GreenfieldConfig)

	km, err := sdkkeys.NewPrivateKeyManager(privKey)
	if err != nil {
		panic(err)
	}

	clients := sdkclient.NewGnfdCompositClients(
		cfg.GreenfieldConfig.GRPCAddrs,
		cfg.GreenfieldConfig.RPCAddrs,
		cfg.GreenfieldConfig.ChainIdString,
		sdkclient.WithKeyManager(km),
		sdkclient.WithGrpcDialOption(grpc.WithTransportCredentials(insecure.NewCredentials())),
	)
	spClient, err := sp.NewSpClient("", sp.WithKeyManager(km))
	return &Executor{
		gnfdClients: clients,
		spClient:    spClient,
		address:     km.GetAddr().String(),
		config:      cfg,
		cdc:         Cdc(),
	}
}

func getGreenfieldPrivateKey(cfg *config.GreenfieldConfig) string {
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
	return privateKey
}

func (e *Executor) getRpcClient() (client.Client, error) {
	client, err := e.gnfdClients.GetClient()
	if err != nil {
		return nil, err
	}
	return client.TendermintClient.RpcClient.TmClient, nil
}

func (e *Executor) getGnfdClient() (*sdkclient.GreenfieldClient, error) {
	client, err := e.gnfdClients.GetClient()
	if err != nil {
		return nil, err
	}
	return client.GreenfieldClient, nil
}

func (e *Executor) GetGnfdClient() (*sdkclient.GreenfieldClient, error) {
	client, err := e.gnfdClients.GetClient()
	if err != nil {
		return nil, err
	}
	return client.GreenfieldClient, nil
}

func (e *Executor) GetSPClient() *sp.SPClient {
	client := e.spClient
	return client
}

func (e *Executor) GetBlockAndBlockResultAtHeight(height int64) (*tmtypes.Block, *ctypes.ResultBlockResults, error) {
	client, err := e.getRpcClient()
	if err != nil {
		return nil, nil, err
	}
	block, err := client.Block(context.Background(), &height)
	if err != nil {
		return nil, nil, err
	}
	blockResults, err := client.BlockResults(context.Background(), &height)
	if err != nil {
		return nil, nil, err
	}
	return block.Block, blockResults, nil
}

func (e *Executor) GetLatestBlockHeight() (latestHeight uint64, err error) {
	client, err := e.gnfdClients.GetClient()
	if err != nil {
		return 0, err
	}
	return uint64(client.Height), nil
}

func (e *Executor) QueryTendermintLightBlock(height int64) ([]byte, error) {
	client, err := e.getRpcClient()
	if err != nil {
		return nil, err
	}
	validators, err := client.Validators(context.Background(), &height, nil, nil)
	if err != nil {
		return nil, err
	}
	commit, err := client.Commit(context.Background(), &height)
	if err != nil {
		return nil, err
	}
	validatorSet := tmtypes.NewValidatorSet(validators.Validators)
	if err != nil {
		return nil, err
	}
	lightBlock := tmtypes.LightBlock{
		SignedHeader: &commit.SignedHeader,
		ValidatorSet: validatorSet,
	}
	protoBlock, err := lightBlock.ToProto()
	if err != nil {
		return nil, err
	}
	return protoBlock.Marshal()
}

func (e *Executor) queryLatestValidators() ([]*tmtypes.Validator, error) {
	client, err := e.getRpcClient()
	if err != nil {
		return nil, err
	}
	validators, err := client.Validators(context.Background(), nil, nil, nil)
	if err != nil {
		return nil, err
	}
	return validators.Validators, nil
}

func (e *Executor) QueryValidatorsAtHeight(height uint64) ([]*tmtypes.Validator, error) {
	client, err := e.getRpcClient()
	if err != nil {
		return nil, err
	}
	h := int64(height)
	validators, err := client.Validators(context.Background(), &h, nil, nil)
	if err != nil {
		return nil, err
	}
	return validators.Validators, nil
}

func (e *Executor) QueryCachedLatestValidators() ([]*tmtypes.Validator, error) {
	if len(e.validators) != 0 {
		return e.validators, nil
	}
	validators, err := e.queryLatestValidators()
	if err != nil {
		return nil, err
	}
	return validators, nil
}

func (e *Executor) UpdateCachedLatestValidatorsLoop() {
	ticker := time.NewTicker(UpdateCachedValidatorsInterval)
	for range ticker.C {
		validators, err := e.queryLatestValidators()
		if err != nil {
			logging.Logger.Errorf("update latest greenfield validators error, err=%s", err)
			continue
		}
		e.validators = validators
	}
}

func (e *Executor) GetValidatorsBlsPublicKey() ([]string, error) {
	validators, err := e.QueryCachedLatestValidators()
	if err != nil {
		return nil, err
	}
	var keys []string
	for _, v := range validators {
		keys = append(keys, hex.EncodeToString(v.RelayerBlsKey))
	}
	return keys, nil
}

func (e *Executor) GetAccount(address string) (authtypes.AccountI, error) {
	gnfdClient, err := e.getGnfdClient()
	if err != nil {
		return nil, err
	}
	authRes, err := gnfdClient.Account(context.Background(), &authtypes.QueryAccountRequest{Address: address})
	if err != nil {
		return nil, err
	}
	var account authtypes.AccountI
	if err := e.cdc.InterfaceRegistry().UnpackAny(authRes.Account, &account); err != nil {
		return nil, err
	}
	return account, nil
}

func (e *Executor) SendHeartbeatTx(challengeId uint64, voteAddressSet []uint64, aggregatedSig []byte) (string, error) {
	gnfdClient, err := e.getGnfdClient()
	if err != nil {
		return "", err
	}

	acc, err := sdk.AccAddressFromHexUnsafe(e.address)
	if err != nil {
		return "", err
	}

	//transfer := banktypes.NewMsgSend(acc, acc, sdk.NewCoins(sdk.NewInt64Coin("BNB", 100)))

	msgHeartbeat := challangetypes.NewMsgHeartbeat(
		acc,
		challengeId,
		voteAddressSet,
		aggregatedSig,
	)

	txRes, err := gnfdClient.BroadcastTx(
		[]sdk.Msg{msgHeartbeat},
		nil,
	)
	if err != nil {
		return "", err
	}
	if txRes.TxResponse.Code != 0 {
		return "", fmt.Errorf("tx error, code=%d, log=%s", txRes.TxResponse.Code, txRes.TxResponse.RawLog)
	}
	return txRes.TxResponse.TxHash, nil
}

func (e *Executor) BroadcastVote(v *votepool.Vote) error {
	client, err := e.gnfdClients.GetClient()
	if err != nil {
		return err
	}
	broadcastMap := make(map[string]interface{})
	broadcastMap[VotePoolBroadcastParameterKey] = *v
	_, err = client.JsonRpcClient.Call(context.Background(), VotePoolBroadcastMethodName, broadcastMap, &ctypes.ResultBroadcastVote{})
	if err != nil {
		return err
	}
	return nil
}
