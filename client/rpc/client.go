package rpc

import (
	"github.com/bnb-chain/greenfield-go-sdk/client/chain"
	"github.com/bnb-chain/greenfield-go-sdk/client/sp"
	"github.com/bnb-chain/greenfield-go-sdk/types"
	"github.com/gnfd-challenger/keys"
	"google.golang.org/grpc"
)

type GreenfieldChallengerClient struct {
	ChainClient *chain.GreenfieldClient
	SpClient    *sp.SPClient
	keyManager  keys.KeyManager
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

func NewGreenfieldChallengerClient(grpcAddr, chainId, endpoint string, opt sp.Option, km keys.KeyManager) *GreenfieldChallengerClient {
	chainClient := chain.NewGreenfieldClient(grpcAddr, chainId)
	spClient, err := sp.NewSpClient(endpoint, &opt)
	// TODO: Error message
	if err != nil {
		panic("sp client cannot be initiated")
	}

	return &GreenfieldChallengerClient{
		ChainClient: &chainClient,
		SpClient:    spClient,
		keyManager:  km,
	}
}

func (c *GreenfieldChallengerClient) GetKeyManager() (keys.KeyManager, error) {
	if c.keyManager == nil {
		return nil, types.KeyManagerNotInitError
	}
	return c.keyManager, nil
}
