package verifier

import (
	"bytes"
	"context"
	"github.com/bnb-chain/greenfield-challenger/common"
	"github.com/bnb-chain/greenfield-challenger/executor"
	"io/ioutil"
	"time"

	"github.com/bnb-chain/greenfield-challenger/config"
	"github.com/bnb-chain/greenfield-challenger/db/dao"
	"github.com/bnb-chain/greenfield-challenger/db/model"
	"github.com/bnb-chain/greenfield-challenger/keys"
	"github.com/bnb-chain/greenfield-challenger/logging"
	"github.com/bnb-chain/greenfield-challenger/vote"
	"github.com/bnb-chain/greenfield-common/go/hash"
	"github.com/bnb-chain/greenfield-go-sdk/client/sp"
	storagetypes "github.com/bnb-chain/greenfield/x/storage/types"
)

type GreenfieldHashVerifier struct {
	votePoolExecutor *vote.VotePoolExecutor
	daoManager       *dao.DaoManager
	config           *config.Config
	signer           *vote.VoteSigner
	executor         *executor.Executor
	blsPublicKey     []byte
}

func NewGreenfieldHashVerifier(cfg *config.Config, dao *dao.DaoManager, signer *vote.VoteSigner, executor *executor.Executor,
	votePoolExecutor *vote.VotePoolExecutor,
) *GreenfieldHashVerifier {
	return &GreenfieldHashVerifier{
		config:           cfg,
		daoManager:       dao,
		signer:           signer,
		executor:         executor,
		votePoolExecutor: votePoolExecutor,
		blsPublicKey:     keys.GetBlsPubKeyFromPrivKeyStr(cfg.VotePoolConfig.BlsPrivateKey),
	}
}

func (p *GreenfieldHashVerifier) VerifyHash() {
	for {
		err := p.verifyHash()
		if err != nil {
			time.Sleep(common.RetryInterval)
		}
	}
}

func (p *GreenfieldHashVerifier) verifyHash() error {
	// Read unprocessed event from db with lowest challengeId
	lowestUnprocessedEvent, err := p.daoManager.EventDao.GetEarliestAttestEvent(model.Unprocessed)
	if err != nil {
		logging.Logger.Infof("No unprocessed events remaining.")
		return nil
	}

	// Call StorageProvider API to get piece hashes
	challengeInfo := sp.ChallengeInfo{
		ObjectId:        string(lowestUnprocessedEvent.ObjectId),
		PieceIndex:      int(lowestUnprocessedEvent.SegmentIndex),
		RedundancyIndex: int(lowestUnprocessedEvent.RedundancyIndex),
	}
	// TODO: What to use for authinfo?
	authInfo := sp.NewAuthInfo(false, "")
	client, err := p.executor.GetGnfdClient()
	spClient := p.executor.GetSPClient()
	if err != nil {
		return err
	}
	challengeRes, err := spClient.ChallengeSP(context.Background(), challengeInfo, authInfo)
	if err != nil {
		return err
	}

	// Call blockchain for storage obj
	// TODO: Will be changed to use ObjectID instead so will have to wait
	headObjQueryReq := &storagetypes.QueryHeadObjectRequest{
		//BucketName:,
		//ObjectName:,
	}
	storageObj, err := client.StorageQueryClient.HeadObject(context.Background(), headObjQueryReq)
	if err != nil {
		return err
	}
	// Hash pieceData -> Replace pieceData hash in checksums -> Validate against original checksum stored on-chain
	// RootHash = dataHash + piecesHash
	newChecksums := storageObj.ObjectInfo.GetChecksums() // 0-6
	bytePieceData, err := ioutil.ReadAll(challengeRes.PieceData)
	if err != nil {
		return err
	}
	hashPieceData := hash.CalcSHA256(bytePieceData)
	newChecksums[lowestUnprocessedEvent.SegmentIndex] = hashPieceData
	total := bytes.Join(newChecksums, []byte(""))
	rootHash := []byte(hash.CalcSHA256Hex(total))

	if bytes.Equal(rootHash, storageObj.ObjectInfo.Checksums[lowestUnprocessedEvent.RedundancyIndex+1]) {
		p.daoManager.EventDao.UpdateEventStatusByChallengeId(lowestUnprocessedEvent.ChallengeId, model.VerifiedValidChallenge)
		return nil
	}

	p.daoManager.EventDao.UpdateEventStatusByChallengeId(lowestUnprocessedEvent.ChallengeId, model.VerifiedInvalidChallenge)
	return nil
}
