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

	"github.com/prysmaticlabs/prysm/crypto/bls/blst"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	"github.com/bnb-chain/greenfield-challenger/common"
	"github.com/bnb-chain/greenfield-challenger/config"
	"github.com/bnb-chain/greenfield-challenger/logging"
	sdkclient "github.com/bnb-chain/greenfield-go-sdk/client/chain"
	"github.com/bnb-chain/greenfield-go-sdk/client/sp"
	sdkkeys "github.com/bnb-chain/greenfield-go-sdk/keys"
	challangetypes "github.com/bnb-chain/greenfield/x/challenge/types"
	sptypes "github.com/bnb-chain/greenfield/x/sp/types"
	storagetypes "github.com/bnb-chain/greenfield/x/storage/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	ethcommon "github.com/ethereum/go-ethereum/common"
	"github.com/spf13/viper"
	tmclient "github.com/tendermint/tendermint/rpc/client"
	coretypes "github.com/tendermint/tendermint/rpc/core/types"
	ctypes "github.com/tendermint/tendermint/rpc/core/types"
	tmtypes "github.com/tendermint/tendermint/types"
	"github.com/tendermint/tendermint/votepool"
)

type Executor struct {
	gnfdClients         *sdkclient.GnfdCompositeClients
	spClient            *sp.SPClient
	config              *config.Config
	address             string
	mtx                 sync.RWMutex
	validators          []*tmtypes.Validator // used to cache validators
	heartbeatInterval   uint64               // used to save challenge heartbeat interval
	attestedChallengeId uint64               // used to save the last attested challenge id
	height              uint64
	BlsPrivKey          []byte
	BlsPubKey           []byte
}

func NewExecutor(cfg *config.Config) *Executor {
	privKey := viper.GetString(config.FlagConfigPrivateKey)
	if privKey == "" {
		privKey = getGreenfieldPrivateKey(&cfg.GreenfieldConfig)
	}

	blsPrivKeyStr := viper.GetString(config.FlagConfigBlsPrivateKey)
	if blsPrivKeyStr == "" {
		blsPrivKeyStr = getGreenfieldBlsPrivateKey(&cfg.GreenfieldConfig)
	}

	blsPrivKeyBytes := ethcommon.Hex2Bytes(blsPrivKeyStr)
	blsPrivKey, err := blst.SecretKeyFromBytes(blsPrivKeyBytes)
	if err != nil {
		logging.Logger.Errorf("executor failed to derive bls private key, err=%+v", err.Error())
	}
	blsPubKey := blsPrivKey.PublicKey().Marshal()

	km, err := sdkkeys.NewPrivateKeyManager(privKey)
	if err != nil {
		logging.Logger.Errorf("executor failed to initiate with a key manager, err=%+v", err.Error())
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
		mtx:         sync.RWMutex{},
		BlsPrivKey:  blsPrivKeyBytes,
		BlsPubKey:   blsPubKey,
	}
}

func getGreenfieldPrivateKey(cfg *config.GreenfieldConfig) string {
	if cfg.KeyType == config.KeyTypeAWSPrivateKey {
		result, err := config.GetSecret(cfg.AWSSecretName, cfg.AWSRegion)
		if err != nil {
			logging.Logger.Errorf("executor failed to get aws private key, err=%+v", err.Error())
			panic(err)
		}
		type AwsPrivateKey struct {
			PrivateKey string `json:"private_key"`
		}
		var awsPrivateKey AwsPrivateKey
		err = json.Unmarshal([]byte(result), &awsPrivateKey)
		if err != nil {
			logging.Logger.Errorf("executor failed to unmarshal aws private key, err=%+v", err.Error())
			panic(err)
		}
		return awsPrivateKey.PrivateKey
	}
	return cfg.PrivateKey
}

func getGreenfieldBlsPrivateKey(cfg *config.GreenfieldConfig) string {
	if cfg.KeyType == config.KeyTypeAWSPrivateKey {
		result, err := config.GetSecret(cfg.AWSBlsSecretName, cfg.AWSRegion)
		if err != nil {
			panic(err)
		}
		type AwsPrivateKey struct {
			PrivateKey string `json:"bls_private_key"`
		}
		var awsBlsPrivateKey AwsPrivateKey
		err = json.Unmarshal([]byte(result), &awsBlsPrivateKey)
		if err != nil {
			panic(err)
		}
		return awsBlsPrivateKey.PrivateKey
	}
	return cfg.BlsPrivateKey
}

func (e *Executor) getRpcClient() (tmclient.Client, error) {
	client := e.gnfdClients.GetClient()
	return client.TendermintClient.RpcClient.TmClient, nil
}

func (e *Executor) getGnfdClient() (*sdkclient.GreenfieldClient, error) {
	client := e.gnfdClients.GetClient()
	return client.GreenfieldClient, nil
}

func (e *Executor) GetBlockAndBlockResultAtHeight(height int64) (*tmtypes.Block, *ctypes.ResultBlockResults, error) {
	client, err := e.getRpcClient()
	if err != nil {
		return nil, nil, err
	}
	block, err := client.Block(context.Background(), &height)
	if err != nil {
		logging.Logger.Errorf("executor failed to get block at height %d, err=%+v", height, err.Error())
		return nil, nil, err
	}
	blockResults, err := client.BlockResults(context.Background(), &height)
	if err != nil {
		logging.Logger.Errorf("executor failed to get block results at height %d, err=%+v", height, err.Error())
		return nil, nil, err
	}
	return block.Block, blockResults, nil
}

func (e *Executor) GetLatestBlockHeight() (latestHeight uint64, err error) {
	client := e.gnfdClients.GetClient()
	latestHeight = uint64(client.Height)

	e.mtx.Lock()
	e.height = latestHeight
	e.mtx.Unlock()
	return latestHeight, nil
}

func (e *Executor) GetCachedBlockHeight() (latestHeight uint64) {
	e.mtx.Lock()
	cachedHeight := e.height
	e.mtx.Unlock()
	return cachedHeight
}

func (e *Executor) queryLatestValidators() ([]*tmtypes.Validator, error) {
	client, err := e.getRpcClient()
	if err != nil {
		return nil, err
	}
	validators, err := client.Validators(context.Background(), nil, nil, nil)
	if err != nil {
		logging.Logger.Errorf("executor failed to query the latest validators, err=%+v", err.Error())
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
			logging.Logger.Errorf("update latest greenfield validators error, err=%+v", err)
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
		keys = append(keys, hex.EncodeToString(v.BlsKey))
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
		logging.Logger.Errorf("error converting addr from hex unsafe when sending attest tx, err=%+v", err.Error())
		return "", err
	}

	msgAttest := challangetypes.NewMsgAttest(
		acc,
		challengeId,
		sdkmath.NewUintFromString(objectId),
		spOperatorAddress,
		voteResult,
		challenger,
		voteAddressSet,
		aggregatedSig,
	)

	txRes, err := gnfdClient.BroadcastTx(
		[]sdk.Msg{msgAttest},
		nil,
	)
	if err != nil {
		logging.Logger.Errorf("error broadcasting msg attest, err=%+v", err.Error())
		return "", err
	}
	if txRes.TxResponse.Code != 0 {
		return "", fmt.Errorf("tx error, code=%d, log=%s", txRes.TxResponse.Code, txRes.TxResponse.RawLog)
	}
	return txRes.TxResponse.TxHash, nil
}

func (e *Executor) queryLatestAttestedChallengeId() (uint64, error) {
	client := e.gnfdClients.GetClient()

	res, err := client.ChallengeQueryClient.LatestAttestedChallenge(context.Background(), &challangetypes.QueryLatestAttestedChallengeRequest{})
	if err != nil {
		logging.Logger.Errorf("executor failed to get latest attested challenge, err=%+v", err.Error())
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
			logging.Logger.Errorf("update latest attested challenge error, err=%+v", err)
			continue
		}
		e.mtx.Lock()
		e.attestedChallengeId = challengeId
		e.mtx.Unlock()
	}
}

func (e *Executor) queryChallengeHeartbeatInterval() (uint64, error) {
	client := e.gnfdClients.GetClient()

	res, err := client.ChallengeQueryClient.Params(context.Background(), &challangetypes.QueryParamsRequest{})
	if err != nil {
		logging.Logger.Errorf("executor failed to get latest heartbeat interval, err=%+v", err.Error())
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
			logging.Logger.Errorf("update latest heartbeat interval error, err=%+v", err)
			continue
		}
		e.mtx.Lock()
		e.heartbeatInterval = heartbeatInterval
		e.mtx.Unlock()
	}
}

func (e *Executor) GetHeightLoop() {
	ticker := time.NewTicker(common.RetryInterval)
	for range ticker.C {
		height, err := e.GetLatestBlockHeight()
		if err != nil {
			logging.Logger.Errorf("error trying to get current height, err=%+v", err.Error())
		}
		logging.Logger.Infof("current height=%d", height)
	}
}

func (e *Executor) GetStorageProviderEndpoint(address string) (string, error) {
	client := e.gnfdClients.GetClient()

	res, err := client.SpQueryClient.StorageProvider(context.Background(), &sptypes.QueryStorageProviderRequest{SpAddress: address})
	logging.Logger.Infof("response %s", res)
	logging.Logger.Infof("response sp endpoint %s", res.StorageProvider.Endpoint)
	logging.Logger.Infof("response get sp endpoint %s", res.GetStorageProvider().Endpoint)
	if err != nil {
		logging.Logger.Errorf("executor failed to query storage provider %s, err=%+v", address, err.Error())
		return "", err
	}

	return res.StorageProvider.Endpoint, nil
}

func (e *Executor) GetObjectInfoChecksums(objectId string) ([][]byte, error) {
	client := e.gnfdClients.GetClient()

	headObjQueryReq := storagetypes.QueryHeadObjectByIdRequest{ObjectId: objectId}
	res, err := client.StorageQueryClient.HeadObjectById(context.Background(), &headObjQueryReq)
	if err != nil {
		logging.Logger.Errorf("executor failed to query storage client for objectId %s, err=%+v", objectId, err.Error())
		return nil, err
	}
	return res.ObjectInfo.Checksums, nil
}

func (e *Executor) GetChallengeResultFromSp(endpoint string, objectId string, segmentIndex, redundancyIndex int) (*sp.ChallengeResult, error) {
	spUrl, err := url.Parse(endpoint)
	if err != nil {
		logging.Logger.Errorf("executor failed to parse sp url %s, err=%+v", endpoint, err.Error())
		return nil, err
	}
	e.spClient.SetUrl(spUrl)

	challengeInfo := sp.ChallengeInfo{
		ObjectId:        objectId,
		PieceIndex:      segmentIndex,
		RedundancyIndex: redundancyIndex,
	}
	authInfo := sp.NewAuthInfo(false, "") // TODO: fill auth info when sp api is ready, prove this request is from validator
	challengeRes, err := e.spClient.ChallengeSP(context.Background(), challengeInfo, authInfo)
	if err != nil {
		logging.Logger.Errorf("executor failed to query challenge result info from sp client for objectId %s, err=%+v", objectId, err.Error())
		return nil, err
	}
	return &challengeRes, nil
}

func (e *Executor) QueryVotes(eventType votepool.EventType) ([]*votepool.Vote, error) {
	client := e.gnfdClients.GetClient()

	queryMap := make(map[string]interface{})
	queryMap[VotePoolQueryParameterEventType] = int(eventType)
	queryMap[VotePoolQueryParameterEventHash] = nil
	var queryVote coretypes.ResultQueryVote
	_, err := client.JsonRpcClient.Call(context.Background(), VotePoolQueryMethodName, queryMap, &queryVote)
	if err != nil {
		logging.Logger.Errorf("executor failed to query votes for event type %s, err=%+v", string(eventType), err.Error())
		return nil, err
	}
	return queryVote.Votes, nil
}

func (e *Executor) BroadcastVote(v *votepool.Vote) error {
	client := e.gnfdClients.GetClient()
	broadcastMap := make(map[string]interface{})
	broadcastMap[VotePoolBroadcastParameterKey] = *v
	_, err := client.JsonRpcClient.Call(context.Background(), VotePoolBroadcastMethodName, broadcastMap, &ctypes.ResultBroadcastVote{})
	if err != nil {
		logging.Logger.Errorf("executor failed to broadcast vote to votepool for event hash %s event type %s, err=%+v", string(v.EventHash), string(v.EventType), err.Error())
		return err
	}
	return nil
}
