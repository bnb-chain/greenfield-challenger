package executor

import (
	"context"
	"encoding/hex"
	"encoding/json"
	_ "encoding/json"
	"fmt"
	"net/url"
	"sync"
	"time"

	sdkmath "cosmossdk.io/math"
	"github.com/bnb-chain/greenfield-go-sdk/client/sp"
	"github.com/tendermint/tendermint/votepool"

	"github.com/bnb-chain/greenfield-challenger/config"
	"github.com/bnb-chain/greenfield-challenger/logging"
	sdkclient "github.com/bnb-chain/greenfield-go-sdk/client/chain"
	sdkkeys "github.com/bnb-chain/greenfield-go-sdk/keys"
	challangetypes "github.com/bnb-chain/greenfield/x/challenge/types"
	sptypes "github.com/bnb-chain/greenfield/x/sp/types"
	storagetypes "github.com/bnb-chain/greenfield/x/storage/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/tendermint/tendermint/rpc/client"
	coretypes "github.com/tendermint/tendermint/rpc/core/types"
	ctypes "github.com/tendermint/tendermint/rpc/core/types"
	tmtypes "github.com/tendermint/tendermint/types"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

type Executor struct {
	gnfdClients *sdkclient.GnfdCompositeClients
	spClient    *sp.SPClient
	config      *config.Config
	address     string

	mtx                 sync.RWMutex
	validators          []*tmtypes.Validator // used to cache validators
	heartbeatInterval   uint64               // used to save challenge heartbeat interval
	attestedChallengeId uint64               // used to save the last attested challenge id
	height              uint64
}

func NewExecutor(cfg *config.Config) *Executor {
	privKey := getGreenfieldPrivateKey(&cfg.GreenfieldConfig)

	km, err := sdkkeys.NewPrivateKeyManager(privKey)
	if err != nil {
		logging.Logger.Errorf("executor failed to initiate with a key manager, err=%s", err.Error())
		panic(err)
	}

	clients := sdkclient.NewGnfdCompositClients(
		cfg.GreenfieldConfig.GRPCAddrs,
		cfg.GreenfieldConfig.RPCAddrs,
		cfg.GreenfieldConfig.ChainIdString,
		sdkclient.WithKeyManager(km),
		sdkclient.WithGrpcDialOption(grpc.WithTransportCredentials(insecure.NewCredentials())),
	)
	spClient, err := sp.NewSpClient("127.0.0.1", sp.WithKeyManager(km))
	if err != nil {
		panic(err)
	}
	return &Executor{
		gnfdClients: clients,
		spClient:    spClient,
		address:     km.GetAddr().String(),
		config:      cfg,
	}
}

func getGreenfieldPrivateKey(cfg *config.GreenfieldConfig) string {
	var privateKey string
	if cfg.KeyType == config.KeyTypeAWSPrivateKey {
		result, err := config.GetSecret(cfg.AWSSecretName, cfg.AWSRegion)
		if err != nil {
			logging.Logger.Errorf("failed to get aws secret from config file, err=%s", err.Error())
			panic(err)
		}
		type AwsPrivateKey struct {
			PrivateKey string `json:"private_key"`
		}
		var awsPrivateKey AwsPrivateKey
		err = json.Unmarshal([]byte(result), &awsPrivateKey)
		if err != nil {
			logging.Logger.Errorf("failed to unmarshal aws private key, err=%s", err.Error())
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
		logging.Logger.Errorf("executor failed to get rpc client, err=%s", err.Error())
		return nil, err
	}
	return client.TendermintClient.RpcClient.TmClient, nil
}

func (e *Executor) getGnfdClient() (*sdkclient.GreenfieldClient, error) {
	client, err := e.gnfdClients.GetClient()
	if err != nil {
		logging.Logger.Errorf("executor failed to get greenfield client, err=%s", err.Error())
		return nil, err
	}
	return client.GreenfieldClient, nil
}

func (e *Executor) GetBlockAndBlockResultAtHeight(height int64) (*tmtypes.Block, *ctypes.ResultBlockResults, error) {
	client, err := e.getRpcClient()
	if err != nil {
		return nil, nil, err
	}
	block, err := client.Block(context.Background(), &height)
	if err != nil {
		logging.Logger.Errorf("executor failed to get block at height %d, err=%s", height, err.Error())
		return nil, nil, err
	}
	blockResults, err := client.BlockResults(context.Background(), &height)
	if err != nil {
		logging.Logger.Errorf("executor failed to get block results at height %d, err=%s", height, err.Error())
		return nil, nil, err
	}
	return block.Block, blockResults, nil
}

func (e *Executor) GetLatestBlockHeight() (latestHeight uint64, err error) {
	client, err := e.gnfdClients.GetClient()
	if err != nil {
		logging.Logger.Errorf("executor failed to get greenfield clients, err=%s", err.Error())
		return 0, err
	}
	latestHeight = uint64(client.Height)
	e.height = latestHeight
	return latestHeight, nil
}

func (e *Executor) GetCachedBlockHeight() (latestHeight uint64) {
	return e.height
}

func (e *Executor) queryLatestValidators() ([]*tmtypes.Validator, error) {
	client, err := e.getRpcClient()
	if err != nil {
		return nil, err
	}
	validators, err := client.Validators(context.Background(), nil, nil, nil)
	if err != nil {
		logging.Logger.Errorf("executor failed to query the latest validators, err=%s", err.Error())
		return nil, err
	}
	return validators.Validators, nil
}

func (e *Executor) QueryCachedLatestValidators() ([]*tmtypes.Validator, error) {
	e.mtx.Lock()
	result := make([]*tmtypes.Validator, len(e.validators))
	if len(e.validators) > 0 {
		for i, p := range e.validators {
			v := *p
			result[i] = &v
		}
	}
	e.mtx.Unlock()

	if len(result) != 0 {
		return result, nil
	}

	validators, err := e.queryLatestValidators()
	if err != nil {
		return nil, err
	}
	return validators, nil
}

func (e *Executor) CacheValidatorsLoop() {
	ticker := time.NewTicker(UpdateCachedValidatorsInterval)
	for range ticker.C {
		validators, err := e.queryLatestValidators()
		if err != nil {
			logging.Logger.Errorf("update latest greenfield validators error, err=%s", err)
			continue
		}
		e.mtx.Lock()
		e.validators = validators
		e.mtx.Unlock()
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

func (e *Executor) SendAttestTx(challengeId uint64, objectId, spOperatorAddress string,
	voteResult challangetypes.VoteResult, challenger string,
	voteAddressSet []uint64, aggregatedSig []byte,
) (string, error) {
	gnfdClient, err := e.getGnfdClient()
	if err != nil {
		return "", err
	}

	acc, err := sdk.AccAddressFromHexUnsafe(e.address)
	if err != nil {
		logging.Logger.Errorf("error converting addr from hex unsafe when sending attest tx, err=%s", err.Error())
		return "", err
	}

	msgAttest := challangetypes.NewMsgAttest(
		acc,
		challengeId,
		sdkmath.NewUintFromString(objectId),
		spOperatorAddress,
		challangetypes.VoteResult(voteResult),
		challenger,
		voteAddressSet,
		aggregatedSig,
	)

	txRes, err := gnfdClient.BroadcastTx(
		[]sdk.Msg{msgAttest},
		nil,
	)
	if err != nil {
		logging.Logger.Errorf("error broadcasting msg attest, err=%s", err.Error())
		return "", err
	}
	if txRes.TxResponse.Code != 0 {
		return "", fmt.Errorf("tx error, code=%d, log=%s", txRes.TxResponse.Code, txRes.TxResponse.RawLog)
	}
	return txRes.TxResponse.TxHash, nil
}

func (e *Executor) queryLatestAttestedChallengeId() (uint64, error) {
	client, err := e.gnfdClients.GetClient()
	if err != nil {
		return 0, err
	}

	res, err := client.ChallengeQueryClient.LatestAttestedChallenge(context.Background(), &challangetypes.QueryLatestAttestedChallengeRequest{})
	if err != nil {
		logging.Logger.Errorf("executor failed to get latest attested challenge, err=%s", err.Error())
		return 0, err
	}

	return res.ChallengeId, nil
}

func (e *Executor) QueryLatestAttestedChallengeId() (uint64, error) {
	challengeId := uint64(0)

	e.mtx.RLock()
	challengeId = e.attestedChallengeId
	e.mtx.RUnlock()

	if challengeId != 0 {
		return challengeId, nil
	}
	challengeId, err := e.queryLatestAttestedChallengeId()
	if err != nil {
		return 0, err
	}
	return challengeId, nil
}

func (e *Executor) UpdateAttestedChallengeIdLoop() {
	ticker := time.NewTicker(QueryAttestedChallengeInterval)
	for range ticker.C {
		challengeId, err := e.queryLatestAttestedChallengeId()
		if err != nil {
			logging.Logger.Errorf("update latest attested challenge error, err=%s", err)
			continue
		}
		e.mtx.Lock()
		e.attestedChallengeId = challengeId
		e.mtx.Unlock()
	}
}

func (e *Executor) queryChallengeHeartbeatInterval() (uint64, error) {
	client, err := e.gnfdClients.GetClient()
	if err != nil {
		return 0, err
	}

	res, err := client.ChallengeQueryClient.Params(context.Background(), &challangetypes.QueryParamsRequest{})
	if err != nil {
		logging.Logger.Errorf("executor failed to get latest heartbeat interval, err=%s", err.Error())
		return 0, err
	}

	return res.Params.HeartbeatInterval, nil
}

func (e *Executor) QueryChallengeHeartbeatInterval() (uint64, error) {
	heartbeatInterval := uint64(0)

	e.mtx.RLock()
	heartbeatInterval = e.heartbeatInterval
	e.mtx.RUnlock()

	if heartbeatInterval != 0 {
		return heartbeatInterval, nil
	}
	heartbeatInterval, err := e.queryChallengeHeartbeatInterval()
	if err != nil {
		return 0, err
	}
	return heartbeatInterval, nil
}

func (e *Executor) UpdateHeartbeatIntervalLoop() {
	ticker := time.NewTicker(QueryHeartbeatIntervalInterval)
	for range ticker.C {
		heartbeatInterval, err := e.queryChallengeHeartbeatInterval()
		if err != nil {
			logging.Logger.Errorf("update latest heartbeat interval error, err=%s", err)
			continue
		}
		e.mtx.Lock()
		e.heartbeatInterval = heartbeatInterval
		e.mtx.Unlock()
	}
}

func (e *Executor) GetStorageProviderEndpoint(address string) (string, error) {
	client, err := e.gnfdClients.GetClient()
	if err != nil {
		return "", err
	}

	res, err := client.SpQueryClient.StorageProvider(context.Background(), &sptypes.QueryStorageProviderRequest{SpAddress: address})
	if err != nil {
		logging.Logger.Errorf("executor failed to query storage provider %s, err=%s", address, err.Error())
		return "", err
	}

	return res.StorageProvider.Endpoint, nil
}

func (e *Executor) GetObjectInfoChecksums(objectId string) ([][]byte, error) {
	client, err := e.gnfdClients.GetClient()
	if err != nil {
		return nil, err
	}

	headObjQueryReq := storagetypes.QueryHeadObjectByIdRequest{ObjectId: objectId}
	res, err := client.StorageQueryClient.HeadObjectById(context.Background(), &headObjQueryReq)
	if err != nil {
		logging.Logger.Errorf("executor failed to query storage client for objectId %s, err=%s", objectId, err.Error())
		return nil, err
	}
	return res.ObjectInfo.Checksums, nil
}

func (e *Executor) GetChallengeResultFromSp(endpoint string, objectId string, segmentIndex, redundancyIndex int) (*sp.ChallengeResult, error) {
	spUrl, err := url.Parse(endpoint)
	if err != nil {
		logging.Logger.Errorf("executor failed to parse sp url %s, err=%s", endpoint, err.Error())
		return nil, err
	}
	e.spClient.SetUrl(spUrl)

	challengeInfo := sp.ChallengeInfo{
		ObjectId:        objectId,
		PieceIndex:      segmentIndex,
		RedundancyIndex: redundancyIndex,
	}
	authInfo := sp.NewAuthInfo(false, "") // TODO: fill auth info when sp api is ready
	challengeRes, err := e.spClient.ChallengeSP(context.Background(), challengeInfo, authInfo)
	if err != nil {
		logging.Logger.Errorf("executor failed to query challenge result info from sp client for objectId %s, err=%s", objectId, err.Error())
		return nil, err
	}
	return &challengeRes, nil
}

func (e *Executor) QueryVotes(eventType votepool.EventType) ([]*votepool.Vote, error) {
	client, err := e.gnfdClients.GetClient()
	if err != nil {
		return nil, err
	}

	queryMap := make(map[string]interface{})
	queryMap[VotePoolQueryParameterEventType] = int(eventType)
	queryMap[VotePoolQueryParameterEventHash] = nil
	var queryVote coretypes.ResultQueryVote
	_, err = client.JsonRpcClient.Call(context.Background(), VotePoolQueryMethodName, queryMap, &queryVote)
	if err != nil {
		logging.Logger.Errorf("executor failed to query votes for event type %s, err=%s", string(eventType), err.Error())
		return nil, err
	}
	return queryVote.Votes, nil
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
		logging.Logger.Errorf("executor failed to broadcast vote to votepool for event hash %s event type %s, err=%s", string(v.EventHash), string(v.EventType), err.Error())
		return err
	}
	return nil
}
