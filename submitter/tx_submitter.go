package submitter

import (
	"time"

	"github.com/bnb-chain/greenfield-challenger/common"
	"github.com/bnb-chain/greenfield-challenger/config"
	"github.com/bnb-chain/greenfield-challenger/db/model"
	"github.com/bnb-chain/greenfield-challenger/executor"
	"github.com/bnb-chain/greenfield-challenger/logging"
	"github.com/bnb-chain/greenfield-challenger/vote"
)

type TxSubmitter struct {
	config           *config.Config
	executor         *executor.Executor
	votePoolExecutor *vote.VotePoolExecutor
	SubmitterKind
}

func NewTxSubmitter(cfg *config.Config, executor *executor.Executor,
	votePoolExecutor *vote.VotePoolExecutor, submitterKind SubmitterKind) *TxSubmitter {
	return &TxSubmitter{
		config:           cfg,
		executor:         executor,
		votePoolExecutor: votePoolExecutor,
		SubmitterKind:    submitterKind,
	}
}

func (a *TxSubmitter) SubmitTransactionLoop() {
	for {
		err := a.process()
		if err != nil {
			logging.Logger.Errorf("encounter error when relaying tx, err=%s ", err.Error())
			time.Sleep(common.RetryInterval)
		}
	}
}

func (a *TxSubmitter) process() error {
	event, err := a.FetchEventForSubmit()
	if err != nil {
		return err
	}
	if (*event == model.Event{}) {
		return nil
	}

	// Get votes result for a tx, which are already validated and qualified to aggregate sig
	votes, err := a.FetchVotesForAggregation(event.ChallengeId)
	if err != nil {
		logging.Logger.Errorf("failed to get votes for event with challenge id %d", event.ChallengeId)
		return err
	}
	validators, err := a.executor.QueryCachedLatestValidators()
	if err != nil {
		return err
	}
	aggregatedSignature, valBitSet, err := vote.AggregateSignatureAndValidatorBitSet(votes, validators)
	if err != nil {
		return err
	}

	//relayerBlsPubKeys, err := a.executor.GetValidatorsBlsPublicKey()
	//if err != nil {
	//	return err
	//}
	//
	//relayerPubKey := util.BlsPubKeyFromPrivKeyStr(a.votePoolExecutor.GetBlsPrivateKey())
	//relayerIdx := util.IndexOf(hex.EncodeToString(relayerPubKey), relayerBlsPubKeys)
	//firstInturnRelayerIdx := int(event.Height) % len(relayerBlsPubKeys)
	//txRelayStartTime := tx.TxTime + a.config.RelayConfig.GreenfieldToBSCRelayingDelayTime
	//logging.Logger.Infof("tx will be relayed starting at %d", txRelayStartTime)
	//
	//var indexDiff int
	//if relayerIdx >= firstInturnRelayerIdx {
	//	indexDiff = relayerIdx - firstInturnRelayerIdx
	//} else {
	//	indexDiff = len(relayerBlsPubKeys) - (firstInturnRelayerIdx - relayerIdx)
	//}
	//curRelayerRelayingStartTime := int64(0)
	//if indexDiff == 0 {
	//	curRelayerRelayingStartTime = txRelayStartTime
	//} else {
	//	curRelayerRelayingStartTime = txRelayStartTime + a.config.RelayConfig.FirstInTurnRelayerRelayingWindow + int64(indexDiff-1)*a.config.RelayConfig.InTurnRelayerRelayingWindow
	//}
	//logging.Logger.Infof("current relayer starts relaying from %d", curRelayerRelayingStartTime)

	// submit transaction
	triedTimes := 0
	for {
		triedTimes++
		// skip current tx if reach the max retry.
		if triedTimes > SubmitTxMaxRetry {
			// TODO mark the status to event to ?
			return nil
		}
		_, err := a.SubmitTx(event, valBitSet, aggregatedSignature)
		if err == nil {
			break
		}
	}

	return a.UpdateEventStatus(event.ChallengeId, model.Submitted)
}
