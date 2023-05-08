package executor

import (
	"context"
	"encoding/hex"
	"encoding/json"
	_ "encoding/json"
	"fmt"
	"strconv"
	"sync"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	sdkmath "cosmossdk.io/math"
	"github.com/bnb-chain/greenfield-challenger/common"
	"github.com/bnb-chain/greenfield-challenger/config"
	"github.com/bnb-chain/greenfield-challenger/logging"
	gnfdClient "github.com/bnb-chain/greenfield-go-sdk/client"
	"github.com/bnb-chain/greenfield-go-sdk/types"
	tm "github.com/bnb-chain/greenfield/sdk/client"
	types2 "github.com/bnb-chain/greenfield/sdk/types"
	challangetypes "github.com/bnb-chain/greenfield/x/challenge/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	ethcommon "github.com/ethereum/go-ethereum/common"
	"github.com/prysmaticlabs/prysm/crypto/bls/blst"
	"github.com/spf13/viper"
	tmrpcclient "github.com/tendermint/tendermint/rpc/client"
	coretypes "github.com/tendermint/tendermint/rpc/core/types"
	ctypes "github.com/tendermint/tendermint/rpc/core/types"
	tmjsonrpcclient "github.com/tendermint/tendermint/rpc/jsonrpc/client"
	tmtypes "github.com/tendermint/tendermint/types"
	"github.com/tendermint/tendermint/votepool"
)

type Executor struct {
	gnfdClients          []gnfdClient.Client
	tmRpcClients         []tmrpcclient.Client
	tmJsonRpcClients     []*tmjsonrpcclient.Client
	config               *config.Config
	address              string
	mtx                  sync.RWMutex
	validators           []*tmtypes.Validator // used to cache validators
	heartbeatInterval    uint64               // used to save challenge heartbeat interval
	attestedChallengeIds []uint64             // used to save the last attested challenge id
	height               uint64
	BlsPrivKey           []byte
	BlsPubKey            []byte
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

	km, err := types.NewAccountFromPrivateKey("challenger", privKey)
	if err != nil {
		logging.Logger.Errorf("executor failed to initiate with a key manager, err=%+v", err.Error())
		panic(err)
	}

	gnfdClients := make([]gnfdClient.Client, len(cfg.GreenfieldConfig.GRPCAddrs))
	for _, addr := range cfg.GreenfieldConfig.GRPCAddrs {
		client, err := gnfdClient.New(
			cfg.GreenfieldConfig.ChainIdString,
			addr,
			gnfdClient.Option{DefaultAccount: km, GrpcDialOption: grpc.WithTransportCredentials(insecure.NewCredentials())},
		)
		if err != nil {
			logging.Logger.Errorf("executor failed to initiate with greenfield clients, err=%s", err.Error())
		}
		gnfdClients = append(gnfdClients, client)
	}

	tmRPCClients := make([]tmrpcclient.Client, len(cfg.GreenfieldConfig.RPCAddrs))
	tmJsonRPCClients := make([]*tmjsonrpcclient.Client, len(cfg.GreenfieldConfig.RPCAddrs))
	for _, addr := range cfg.GreenfieldConfig.RPCAddrs {
		RPCClient := NewTendermintRPCClient(addr)
		JsonRPCClient, err := NewTendermintJsonRPCClient(addr)
		if err != nil {
			logging.Logger.Errorf("executor failed to initiate with tendermint json rpc clients, err=%s", err.Error())
		}
		tmRPCClients = append(tmRPCClients, RPCClient.TmClient)
		tmJsonRPCClients = append(tmJsonRPCClients, JsonRPCClient)
	}

	return &Executor{
		gnfdClients:      gnfdClients,
		tmRpcClients:     tmRPCClients,
		tmJsonRpcClients: tmJsonRPCClients,
		address:          km.GetAddress().String(),
		config:           cfg,
		mtx:              sync.RWMutex{},
		BlsPrivKey:       blsPrivKeyBytes,
		BlsPubKey:        blsPubKey,
	}
}

func NewTendermintRPCClient(provider string) *tm.TendermintClient {
	rpcClient := tm.NewTendermintClient(provider)
	return &rpcClient
}

func NewTendermintJsonRPCClient(provider string) (*tmjsonrpcclient.Client, error) {
	rpcClient, err := tmjsonrpcclient.New(provider)
	if err != nil {
		logging.Logger.Errorf("executor failed to initiate with tendermint json rpc client, err=%s", err.Error())
		return nil, err
	}
	return rpcClient, nil
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
	block, err := e.tmRpcClients[0].Block(context.Background(), &height)
	if err != nil {
		logging.Logger.Errorf("executor failed to get block at height %d, err=%+v", height, err.Error())
		return nil, nil, err
	}
	blockResults, err := e.tmRpcClients[0].BlockResults(context.Background(), &height)
	if err != nil {
		logging.Logger.Errorf("executor failed to get block results at height %d, err=%+v", height, err.Error())
		return nil, nil, err
	}
	return block.Block, blockResults, nil
}

func (e *Executor) GetLatestBlockHeight() (uint64, error) {
	client := e.GetGnfdClient()
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
	client := e.GetTmRpcClient()

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
	client := e.GetGnfdClient()

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
	// TODO: Is txOpt correct?
	txRes, err := client.BroadcastTx(
		context.Background(),
		[]sdk.Msg{msgAttest},
		types2.TxOption{},
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

func (e *Executor) QueryInturnAttestationSubmitter() (string, error) {
	client := e.GetGnfdClient()
	res, err := client.InturnAttestationSubmitter(context.Background(), &challangetypes.QueryInturnAttestationSubmitterRequest{})
	if err != nil {
		logging.Logger.Errorf("executor failed to get inturn attestation submitter, err=%+v", err.Error())
		return "", err
	}
	return res.BlsPubKey, nil
}

func (e *Executor) AttestChallenge(submitterAddress, challengerAddress, spOperatorAddress string, challengeId uint64, objectId sdkmath.Uint, voteResult challangetypes.VoteResult, voteValidatorSet []uint64, VoteAggSignature []byte, txOption types2.TxOption) (bool, error) {
	client := e.GetGnfdClient()
	res, err := client.AttestChallenge(context.Background(), submitterAddress, challengerAddress, spOperatorAddress, challengeId, objectId, voteResult, voteValidatorSet, VoteAggSignature, txOption)
	if err != nil {
		logging.Logger.Errorf("executor failed to attest challenge, err=%+v", err.Error())
		return false, err
	}
	if res.Code != 0 {
		logging.Logger.Errorf("executor failed to attest challenge, code=%d, log=%s", strconv.Itoa(int(res.Code)), res.Logs.String())
		return false, err
	}
	logging.Logger.Infof("executor attest challenge success, challengeId=%d", challengeId)
	return true, nil
}

func (e *Executor) queryLatestAttestedChallengeIds() ([]uint64, error) {
	client := e.GetGnfdClient()

	res, err := client.LatestAttestedChallenges(context.Background(), &challangetypes.QueryLatestAttestedChallengesRequest{})
	if err != nil {
		logging.Logger.Errorf("executor failed to get latest attested challenge, err=%+v", err.Error())
		return nil, err
	}

	return res, nil
}

func (e *Executor) QueryLatestAttestedChallengeIds() ([]uint64, error) {
	// TODO: check this
	e.mtx.RLock()
	challengeIds := e.attestedChallengeIds
	e.mtx.RUnlock()

	if len(challengeIds) != 0 {
		return challengeIds, nil
	}
	challengeIds, err := e.queryLatestAttestedChallengeIds()
	if err != nil {
		return nil, err
	}
	return challengeIds, nil
}

func (e *Executor) UpdateAttestedChallengeIdLoop() {
	ticker := time.NewTicker(QueryAttestedChallengeInterval)
	for range ticker.C {
		challengeIds, err := e.queryLatestAttestedChallengeIds()
		if err != nil {
			logging.Logger.Errorf("update latest attested challenge error, err=%+v", err)
			continue
		}
		e.mtx.Lock()
		e.attestedChallengeIds = challengeIds
		e.mtx.Unlock()
	}
}

func (e *Executor) queryChallengeHeartbeatInterval() (uint64, error) {
	client := e.GetGnfdClient()
	q := challangetypes.QueryParamsRequest{}
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
	client := e.GetGnfdClient()
	spAddr, err := sdk.AccAddressFromHexUnsafe(address)
	if err != nil {
		logging.Logger.Errorf("error converting addr from hex unsafe when getting sp endpoint, err=%+v", err.Error())
		return "", err
	}
	res, err := client.GetStorageProviderInfo(context.Background(), spAddr)
	logging.Logger.Infof("response %s", res)
	logging.Logger.Infof("response res.endpoint %s", res.Endpoint)
	logging.Logger.Infof("response ree.getendpoint() %s", res.GetEndpoint())
	if err != nil {
		logging.Logger.Errorf("executor failed to query storage provider %s, err=%+v", address, err.Error())
		return "", err
	}

	return res.Endpoint, nil
}

func (e *Executor) GetObjectInfoChecksums(objectId string) ([][]byte, error) {
	client := e.GetGnfdClient()

	res, err := client.HeadObjectByID(context.Background(), objectId)
	if err != nil {
		logging.Logger.Errorf("executor failed to query storage client for objectId %s, err=%+v", objectId, err.Error())
		return nil, err
	}
	return res.Checksums, nil
}

func (e *Executor) GetChallengeResultFromSp(objectId string, segmentIndex, redundancyIndex int) (*types.ChallengeResult, error) {
	client := e.GetGnfdClient()

	challengeInfoRequest := types.ChallengeInfo{
		ObjectId:        objectId,
		PieceIndex:      segmentIndex,
		RedundancyIndex: redundancyIndex,
	}
	challengeInfo, err := client.GetChallengeInfo(context.Background(), challengeInfoRequest)
	if err != nil {
		logging.Logger.Errorf("executor failed to query challenge info from gnfd client for objectId %s, err=%+v", objectId, err.Error())
		return nil, err
	}

	if err != nil {
		logging.Logger.Errorf("executor failed to query challenge result info from sp client for objectId %s, err=%+v", objectId, err.Error())
		return nil, err
	}
	return &challengeInfo, nil
}

func (e *Executor) QueryVotes(eventType votepool.EventType) ([]*votepool.Vote, error) {
	client := e.GetTmJsonRpcClient()

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
	client := e.GetTmJsonRpcClient()
	broadcastMap := make(map[string]interface{})
	broadcastMap[VotePoolBroadcastParameterKey] = *v
	_, err := client.Call(context.Background(), VotePoolBroadcastMethodName, broadcastMap, &ctypes.ResultBroadcastVote{})
	if err != nil {
		logging.Logger.Errorf("executor failed to broadcast vote to votepool for event hash %s event type %s, err=%+v", string(v.EventHash), string(v.EventType), err.Error())
		return err
	}
	return nil
}

// TODO: implement this
func (e *Executor) GetGnfdClient() gnfdClient.Client {
	return e.gnfdClients[0]
	//wg := new(sync.WaitGroup)
	//wg.Add(len(e.gnfdClients))
	//clientCh := make(chan *gnfdClient.Client)
	//waitCh := make(chan struct{})
	//go func() {
	//	for _, c := range e.gnfdClients {
	//		go getClientBlockHeight(clientCh, wg, &c)
	//	}
	//	wg.Wait()
	//	close(waitCh)
	//}()
	//var maxHeight int64
	//maxHeightclient := e.GetGnfdClient()
	//for {
	//	select {
	//	case c := <-clientCh:
	//		if c.Height > maxHeight {
	//			maxHeight = c.Height
	//			maxHeightClient = c
	//		}
	//	case <-waitCh:
	//		return maxHeightClient
	//	}
	//}
}

//	func getClientBlockHeight(clientChan chan *gnfdClient.Client, wg *sync.WaitGroup, client *gnfdClient.Client) {
//		defer wg.Done()
//		status, err := tmclient.Client.Status(context.Background())
//		if err != nil {
//			return
//		}
//		client.Height = status.SyncInfo.LatestBlockHeight
//		clientChan <- client
//	}
//
// TODO: implement this
func (e *Executor) GetTmRpcClient() tmrpcclient.Client {
	return e.tmRpcClients[0]
}

// TODO: implement this
func (e *Executor) GetTmJsonRpcClient() *tmjsonrpcclient.Client {
	return e.tmJsonRpcClients[0]
}

func (e *Executor) GetAddr() string {
	return e.address
}
