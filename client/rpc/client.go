package rpc

import (
	"context"
	"github.com/avast/retry-go/v4"
	"github.com/bnb-chain/greenfield-go-sdk/client/chain"
	"github.com/bnb-chain/greenfield-go-sdk/client/sp"
	"github.com/bnb-chain/greenfield-go-sdk/types"
	"github.com/gnfd-challenger/common"
	"github.com/gnfd-challenger/config"
	"github.com/gnfd-challenger/keys"
	"github.com/gnfd-challenger/vote"
	"github.com/tendermint/tendermint/libs/sync"
	rpcclient "github.com/tendermint/tendermint/rpc/client"
	rpchttp "github.com/tendermint/tendermint/rpc/client/http"
	coretypes "github.com/tendermint/tendermint/rpc/core/types"
	libclient "github.com/tendermint/tendermint/rpc/jsonrpc/client"
	tmtypes "github.com/tendermint/tendermint/types"
	"github.com/tendermint/tendermint/votepool"
	"google.golang.org/grpc"
	"time"
)

type GreenfieldChallengerClient struct {
	ChainClient   *chain.GreenfieldClient
	SpClient      *sp.SPClient
	RpcClient     rpcclient.Client
	JsonRpcClient *libclient.Client
	keyManager    keys.KeyManager
	config        *config.Config
	mutex         sync.RWMutex
	clientIdx     int // TODO: Check if required
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

func NewGreenfieldChallengerClient(grpcAddr, rpcAddr, chainId, endpoint string, opt sp.Option, km keys.KeyManager, cfg *config.Config) *GreenfieldChallengerClient {
	chainClient := chain.NewGreenfieldClient(grpcAddr, chainId)
	spClient, err := sp.NewSpClient(endpoint, &opt)
	if err != nil {
		panic("sp client cannot be initiated")
	}
	rpcClient := NewRpcClient(rpcAddr)
	jsonRpcClient, err := libclient.New(cfg.VotePoolConfig.RPCAddr)
	if err != nil {
		panic(err)
	}

	return &GreenfieldChallengerClient{
		ChainClient:   &chainClient,
		SpClient:      spClient,
		RpcClient:     rpcClient,
		JsonRpcClient: jsonRpcClient,
		keyManager:    km,
	}
}

func (c *GreenfieldChallengerClient) GetKeyManager() (keys.KeyManager, error) {
	if c.keyManager == nil {
		return nil, types.KeyManagerNotInitError
	}
	return c.keyManager, nil
}

func (c *GreenfieldChallengerClient) GetBlockResultAtHeight(height int64) (*coretypes.ResultBlockResults, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	blockResults, err := c.RpcClient.BlockResults(ctx, &height)
	if err != nil {
		return nil, err
	}
	return blockResults, nil
}

func (c *GreenfieldChallengerClient) GetBlockAtHeight(height int64) (*tmtypes.Block, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	block, err := c.RpcClient.Block(ctx, &height)
	if err != nil {
		return nil, err
	}
	return block.Block, nil
}

func (c *GreenfieldChallengerClient) GetLatestBlockHeightWithRetry() (latestHeight uint64, err error) {
	return c.getLatestBlockHeightWithRetry(c.RpcClient)
}

func (c *GreenfieldChallengerClient) getLatestBlockHeightWithRetry(client rpcclient.Client) (latestHeight uint64, err error) {
	return latestHeight, retry.Do(func() error {
		latestHeightQueryCtx, cancelLatestHeightQueryCtx := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancelLatestHeightQueryCtx()
		var err error
		latestHeight, err = c.GetLatestBlockHeight(latestHeightQueryCtx, client)
		return err
	}, common.RetryAttempts,
		common.RetryDelay,
		common.RetryErr,
		retry.OnRetry(func(n uint, err error) {
			common.Logger.Infof("failed to query latest height, attempt: %d times, max_attempts: %d", n+1, common.RetryAttemptNum)
		}))
}

func (c *GreenfieldChallengerClient) GetLatestBlockHeight(ctx context.Context, client rpcclient.Client) (uint64, error) {
	status, err := client.Status(ctx)
	if err != nil {
		return 0, err
	}
	return uint64(status.SyncInfo.LatestBlockHeight), nil
}

func (c *GreenfieldChallengerClient) QueryVotes(eventHash []byte, eventType votepool.EventType) ([]*votepool.Vote, error) {
	queryMap := make(map[string]interface{})
	queryMap[vote.VotePoolQueryParameterEventType] = int(eventType)
	queryMap[vote.VotePoolQueryParameterEventHash] = eventHash
	var queryVote coretypes.ResultQueryVote
	_, err := c.JsonRpcClient.Call(context.Background(), vote.VotePoolQueryMethodName, queryMap, &queryVote)
	if err != nil {
		return nil, err
	}
	return queryVote.Votes, nil
}

func (c *GreenfieldChallengerClient) BroadcastVote(v *votepool.Vote) error {
	broadcastMap := make(map[string]interface{})
	broadcastMap[vote.VotePoolBroadcastParameterKey] = *v
	var broadcastVote coretypes.ResultBroadcastVote

	_, err := c.JsonRpcClient.Call(context.Background(), vote.VotePoolBroadcastMethodName, broadcastMap, &broadcastVote)
	if err != nil {
		return err
	}
	return nil
}

// TODO: Check if required
//func (c *GreenfieldChallengerClient) GetValidatorsBlsPublicKey() ([]string, error) {
//	validators, err := c.QueryLatestValidators()
//	if err != nil {
//		return nil, err
//	}
//	var keys []string
//	for _, v := range validators {
//		keys = append(keys, hex.EncodeToString(v.GetRelayerBlsKey()))
//	}
//	return keys, nil
//}
