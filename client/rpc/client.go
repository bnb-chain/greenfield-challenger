package rpc

import (
	"context"
	"encoding/hex"
	"time"

	"github.com/bnb-chain/greenfield-challenger/logging"
	gnfdtypes "github.com/bnb-chain/greenfield/sdk/types"

	"github.com/avast/retry-go/v4"
	"github.com/bnb-chain/greenfield-challenger/common"
	"github.com/bnb-chain/greenfield-challenger/config"
	"github.com/bnb-chain/greenfield-challenger/keys"
	"github.com/bnb-chain/greenfield-go-sdk/client/chain"
	"github.com/bnb-chain/greenfield-go-sdk/client/sp"
	"github.com/tendermint/tendermint/libs/sync"
	rpcclient "github.com/tendermint/tendermint/rpc/client"
	rpchttp "github.com/tendermint/tendermint/rpc/client/http"
	coretypes "github.com/tendermint/tendermint/rpc/core/types"
	libclient "github.com/tendermint/tendermint/rpc/jsonrpc/client"
	tmtypes "github.com/tendermint/tendermint/types"
	"google.golang.org/grpc"
)

type ChallengerClient struct {
	ChainClient *chain.GreenfieldClient
	SpClient    *sp.SPClient
	RpcClient   rpcclient.Client
	keyManager  keys.KeyManager
	validators  []*tmtypes.Validator // used to cache validatorss
	config      *config.Config
	mutex       sync.RWMutex
}

func grpcConn(addr string) *grpc.ClientConn {
	conn, err := grpc.Dial(
		addr,
		grpc.WithInsecure(),
	)
	if err != nil {
		panic(err)
	}
	return conn
}

func NewRpcClient(addr string) *rpchttp.HTTP {
	httpClient, err := libclient.DefaultHTTPClient(addr)
	if err != nil {
		panic(err)
	}
	rpcClient, err := rpchttp.NewWithClient(addr, "/websocket", httpClient)
	if err != nil {
		panic(err)
	}
	return rpcClient
}

func NewGreenfieldChallengerClient(grpcAddr, rpcAddr, chainId, endpoint string, opt sp.Option, km keys.KeyManager, cfg *config.Config) *ChallengerClient {
	chainClient := chain.NewGreenfieldClient(grpcAddr, chainId)
	spClient, err := sp.NewSpClient(endpoint, opt)
	if err != nil {
		panic("sp client cannot be initiated")
	}
	rpcClient := NewRpcClient(rpcAddr)
	if err != nil {
		panic(err)
	}

	return &ChallengerClient{
		ChainClient: &chainClient,
		SpClient:    spClient,
		RpcClient:   rpcClient,
		keyManager:  km,
	}
}

func (c *ChallengerClient) GetKeyManager() (keys.KeyManager, error) {
	if c.keyManager == nil {
		return nil, gnfdtypes.KeyManagerNotInitError
	}
	return c.keyManager, nil
}

func (c *ChallengerClient) GetBlockResultAtHeight(height int64) (*coretypes.ResultBlockResults, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	blockResults, err := c.RpcClient.BlockResults(ctx, &height)
	if err != nil {
		return nil, err
	}
	return blockResults, nil
}

func (c *ChallengerClient) GetBlockAtHeight(height int64) (*tmtypes.Block, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	block, err := c.RpcClient.Block(ctx, &height)
	if err != nil {
		return nil, err
	}
	return block.Block, nil
}

func (c *ChallengerClient) GetLatestBlockHeightWithRetry() (latestHeight uint64, err error) {
	return c.getLatestBlockHeightWithRetry(c.RpcClient)
}

func (c *ChallengerClient) getLatestBlockHeightWithRetry(client rpcclient.Client) (latestHeight uint64, err error) {
	return latestHeight, retry.Do(func() error {
		latestHeightQueryCtx, cancelLatestHeightQueryCtx := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancelLatestHeightQueryCtx()
		var err error
		latestHeight, err = c.GetLatestBlockHeight(latestHeightQueryCtx, client)
		return err
	}, common.RtyAttem,
		common.RtyDelay,
		common.RtyErr,
		retry.OnRetry(func(n uint, err error) {
			logging.Logger.Infof("failed to query latest height, attempt: %d times, max_attempts: %d", n+1, common.RtyAttemNum)
		}))
}

func (c *ChallengerClient) GetLatestBlockHeight(ctx context.Context, client rpcclient.Client) (uint64, error) {
	status, err := client.Status(ctx)
	if err != nil {
		return 0, err
	}
	return uint64(status.SyncInfo.LatestBlockHeight), nil
}

func (c *ChallengerClient) QueryValidators() ([]*tmtypes.Validator, error) {
	validators, err := c.RpcClient.Validators(context.Background(), nil, nil, nil)
	c.validators = validators.Validators
	if err != nil {
		return nil, err
	}
	return validators.Validators, nil
}

func (c *ChallengerClient) QueryValidatorsAtHeight(height uint64) ([]*tmtypes.Validator, error) {
	atHeight := int64(height)
	validators, err := c.RpcClient.Validators(context.Background(), &atHeight, nil, nil)
	if err != nil {
		return nil, err
	}
	return validators.Validators, nil
}

func (c *ChallengerClient) GetCachedLatestValidators() ([]*tmtypes.Validator, error) {
	if len(c.validators) != 0 {
		return c.validators, nil
	}
	validators, err := c.QueryValidators()
	if err != nil {
		return nil, err
	}
	return validators, nil
}

func (c *ChallengerClient) QueryValidatorsBlsPublicKey() ([]string, error) {
	validators, err := c.QueryValidators()
	if err != nil {
		return nil, err
	}
	var pubKeys []string
	for _, v := range validators {
		pubKeys = append(pubKeys, hex.EncodeToString(v.RelayerBlsKey))
	}
	return pubKeys, nil
}

func (c *ChallengerClient) GetCachedValidatorsBlsPublicKey() ([]string, error) {
	var validators []*tmtypes.Validator
	var err error
	if len(c.validators) != 0 {
		validators = c.validators
	} else {
		validators, err = c.QueryValidators()
		if err != nil {
			return nil, err
		}
	}

	var pubKeys []string
	for _, v := range validators {
		pubKeys = append(pubKeys, hex.EncodeToString(v.RelayerBlsKey))
	}
	return pubKeys, nil
}
