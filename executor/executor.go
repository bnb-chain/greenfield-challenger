package executor

import (
	"context"
	"encoding/hex"
	"encoding/json"
	_ "encoding/json"
	"sync"
	"time"

	sdkmath "cosmossdk.io/math"
	"github.com/bnb-chain/greenfield-challenger/common"
	"github.com/bnb-chain/greenfield-challenger/config"
	"github.com/bnb-chain/greenfield-challenger/logging"
	"github.com/bnb-chain/greenfield-go-sdk/types"
	sdktypes "github.com/bnb-chain/greenfield/sdk/types"
	challengetypes "github.com/bnb-chain/greenfield/x/challenge/types"
	coretypes "github.com/cometbft/cometbft/rpc/core/types"
	ctypes "github.com/cometbft/cometbft/rpc/core/types"
	tmtypes "github.com/cometbft/cometbft/types"
	"github.com/cometbft/cometbft/votepool"
	sdk "github.com/cosmos/cosmos-sdk/types"
	ethcommon "github.com/ethereum/go-ethereum/common"
	"github.com/prysmaticlabs/prysm/crypto/bls/blst"
	"github.com/spf13/viper"
)

type Executor struct {
	clients           GnfdCompositeClients
	config            *config.Config
	address           string
	mtx               sync.RWMutex
	validators        []*tmtypes.Validator // used to cache validators
	heartbeatInterval uint64               // used to save challenge heartbeat interval
	height            uint64
	BlsPrivKey        []byte
	BlsPubKey         []byte
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

	account, err := types.NewAccountFromPrivateKey("challenger", privKey)
	if err != nil {
		logging.Logger.Errorf("executor failed to initiate with a key manager, err=%+v", err.Error())
		panic(err)
	}

	clients := NewGnfdCompositClients(
		cfg.GreenfieldConfig.RPCAddrs,
		cfg.GreenfieldConfig.ChainIdString,
		account,
	)

	return &Executor{
		clients:    clients,
		address:    account.GetAddress().String(),
		config:     cfg,
		mtx:        sync.RWMutex{},
		BlsPrivKey: blsPrivKeyBytes,
		BlsPubKey:  blsPubKey,
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

func (e *Executor) GetBlockAndBlockResultAtHeight(height int64) (*tmtypes.Block, *ctypes.ResultBlockResults, error) {
	block, err := e.clients.GetClient().TmClient.Block(context.Background(), &height)
	if err != nil {
		logging.Logger.Errorf("executor failed to get block at height %d, err=%+v", height, err.Error())
		return nil, nil, err
	}
	blockResults, err := e.clients.GetClient().TmClient.BlockResults(context.Background(), &height)
	if err != nil {
		logging.Logger.Errorf("executor failed to get block results at height %d, err=%+v", height, err.Error())
		return nil, nil, err
	}
	return block.Block, blockResults, nil
}

func (e *Executor) GetLatestBlockHeight() (uint64, error) {
	client := e.clients.GetClient().Client
	res, err := client.GetLatestBlockHeight(context.Background())
	latestHeight := uint64(res)
	if err != nil {
		logging.Logger.Errorf("executor failed to get latest block height, err=%s", err.Error())
	}

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
	client := e.clients.GetClient().TmClient

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

func (e *Executor) QueryInturnAttestationSubmitter() (*challengetypes.QueryInturnAttestationSubmitterResponse, error) {
	client := e.clients.GetClient().Client
	res, err := client.InturnAttestationSubmitter(context.Background(), &challengetypes.QueryInturnAttestationSubmitterRequest{})
	if err != nil {
		logging.Logger.Errorf("executor failed to get inturn attestation submitter, err=%+v", err.Error())
		return nil, err
	}
	return res, nil
}

func (e *Executor) AttestChallenge(submitterAddress, challengerAddress, spOperatorAddress string, challengeId uint64, objectId sdkmath.Uint, voteResult challengetypes.VoteResult, voteValidatorSet []uint64, VoteAggSignature []byte, txOption sdktypes.TxOption) (bool, error) {
	client := e.clients.GetClient().Client
	logging.Logger.Infof("attest challenge params: submitterAddress=%s, challengerAddress=%s, spOperatorAddress=%s, challengeId=%d, objectId=%s, voteResult=%s, voteValidatorSet=%+v, VoteAggSignature=%+v, txOption=%+v", submitterAddress, challengerAddress, spOperatorAddress, challengeId, objectId.String(), voteResult.String(), voteValidatorSet, VoteAggSignature, txOption)
	res, err := client.AttestChallenge(context.Background(), submitterAddress, challengerAddress, spOperatorAddress, challengeId, objectId, voteResult, voteValidatorSet, VoteAggSignature, txOption)
	if err != nil {
		logging.Logger.Infof("challengeId: %d attest failed, code=%d, log=%s, txhash=%s, timestamp: %s, err=%s", challengeId, res.Code, res.RawLog, res.TxHash, time.Now().Format("15:04:05.000000"), err.Error())
		return false, err
	}
	if res.Code != 0 {
		logging.Logger.Infof("challengeId: %d attest failed, code=%d, log=%s, txhash=%s, timestamp: %s", challengeId, res.Code, res.RawLog, res.TxHash, time.Now().Format("15:04:05.000000"))
		return false, nil
	}
	logging.Logger.Infof("challengeId: %d attest succeeded, code=%d, log=%s, txhash=%s, timestamp: %s", challengeId, res.Code, res.RawLog, res.TxHash, time.Now().Format("15:04:05.000000"))
	return true, nil
}

func (e *Executor) QueryLatestAttestedChallengeIds() ([]uint64, error) {
	client := e.clients.GetClient().Client

	res, err := client.LatestAttestedChallenges(context.Background(), &challengetypes.QueryLatestAttestedChallengesRequest{})
	if err != nil {
		logging.Logger.Errorf("executor failed to get latest attested challenge, err=%+v", err.Error())
		return nil, err
	}

	return res, nil
}

func (e *Executor) queryChallengeHeartbeatInterval() (uint64, error) {
	client := e.clients.GetClient().Client
	q := challengetypes.QueryParamsRequest{}
	res, err := client.ChallengeParams(context.Background(), &q)
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
	// TODO: check addr conversion and GetStorageProviderInfo == GetSpEndpoint previously
	client := e.clients.GetClient().Client
	spAddr, err := sdk.AccAddressFromHexUnsafe(address)
	if err != nil {
		logging.Logger.Errorf("error converting addr from hex unsafe when getting sp endpoint, err=%+v", err.Error())
		return "", err
	}
	res, err := client.GetStorageProviderInfo(context.Background(), spAddr)
	logging.Logger.Infof("response res.endpoint %s", res.Endpoint)
	if err != nil {
		logging.Logger.Errorf("executor failed to query storage provider %s, err=%+v", address, err.Error())
		return "", err
	}

	return res.Endpoint, nil
}

func (e *Executor) GetObjectInfoChecksums(objectId string) ([][]byte, error) {
	client := e.clients.GetClient().Client

	res, err := client.HeadObjectByID(context.Background(), objectId)
	if err != nil {
		logging.Logger.Errorf("executor failed to query storage client for objectId %s, err=%+v", objectId, err.Error())
		return nil, err
	}
	return res.GetChecksums(), nil
}

func (e *Executor) GetChallengeResultFromSp(objectId, endpoint string, segmentIndex, redundancyIndex int) (*types.ChallengeResult, error) {
	client := e.clients.GetClient().Client

	challengeInfoOpts := types.GetChallengeInfoOptions{
		Endpoint: endpoint,
	}
	challengeInfo, err := client.GetChallengeInfo(context.Background(), objectId, segmentIndex, redundancyIndex, challengeInfoOpts)
	if err != nil {
		logging.Logger.Errorf("executor failed to query challenge result info from sp client for objectId %s, err=%+v", objectId, err.Error())
		return nil, err
	}

	return &challengeInfo, nil
}

func (e *Executor) QueryVotes(eventType votepool.EventType) ([]*votepool.Vote, error) {
	client := e.clients.GetClient().JsonRpcClient

	queryMap := make(map[string]interface{})
	queryMap[VotePoolQueryParameterEventType] = int(eventType)
	queryMap[VotePoolQueryParameterEventHash] = nil
	var queryVote coretypes.ResultQueryVote
	_, err := client.Call(context.Background(), VotePoolQueryMethodName, queryMap, &queryVote)
	if err != nil {
		logging.Logger.Errorf("executor failed to query votes for event type %s, err=%+v", string(eventType), err.Error())
		return nil, err
	}
	return queryVote.Votes, nil
}

func (e *Executor) BroadcastVote(v *votepool.Vote) error {
	client := e.clients.GetClient().JsonRpcClient
	broadcastMap := make(map[string]interface{})
	broadcastMap[VotePoolBroadcastParameterKey] = *v
	_, err := client.Call(context.Background(), VotePoolBroadcastMethodName, broadcastMap, &ctypes.ResultBroadcastVote{})
	if err != nil {
		logging.Logger.Errorf("executor failed to broadcast vote to votepool for event hash %s event type %s, err=%+v", string(v.EventHash), string(v.EventType), err.Error())
		return err
	}
	return nil
}

func (e *Executor) GetAddr() string {
	return e.address
}

func (e *Executor) GetNonce() (uint64, error) {
	client := e.clients.GetClient().Client
	account, err := client.GetAccount(context.Background(), e.GetAddr())
	if err != nil {
		logging.Logger.Errorf("error getting account, err=%+v", err.Error())
		return 0, err
	}
	nonce := account.GetSequence()
	return nonce, err
}
