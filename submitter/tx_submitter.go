package submitter

import (
	"fmt"
	"github.com/bnb-chain/greenfield-challenger/alert"
	"time"

	"github.com/bnb-chain/greenfield-challenger/common"
	"github.com/bnb-chain/greenfield-challenger/config"
	"github.com/bnb-chain/greenfield-challenger/db/model"
	"github.com/bnb-chain/greenfield-challenger/executor"
	"github.com/bnb-chain/greenfield-challenger/logging"
	"github.com/bnb-chain/greenfield-challenger/vote"
)

type TxSubmitter struct {
	config   *config.Config
	executor *executor.Executor
	DataProvider
}

func NewTxSubmitter(cfg *config.Config, executor *executor.Executor,
	submitterKind DataProvider) *TxSubmitter {
	return &TxSubmitter{
		config:       cfg,
		executor:     executor,
		DataProvider: submitterKind,
	}
}

func (s *TxSubmitter) SubmitTransactionLoop() {
	for {
		err := s.process()
		if err != nil {
			logging.Logger.Errorf("encounter error when relaying tx, err=%s ", err.Error())
			time.Sleep(common.RetryInterval)
		}
	}
}

func (s *TxSubmitter) process() error {
	events, err := s.FetchEventsForSubmit()
	if err != nil {
		return err
	}
	if len(events) == 0 {
		time.Sleep(common.RetryInterval)
		return nil
	}

	for _, event := range events {
		err = s.submitForSingleEvent(event)
		if err != nil {
			return err
		}
	}

	return nil
}

func (s *TxSubmitter) submitForSingleEvent(event *model.Event) error {
	// Get votes result for s tx, which are already validated and qualified to aggregate sig
	votes, err := s.FetchVotesForAggregation(event.ChallengeId)
	if err != nil {
		logging.Logger.Errorf("failed to get votes for event with challenge id %d", event.ChallengeId)
		return err
	}
	validators, err := s.executor.QueryCachedLatestValidators()
	if err != nil {
		return err
	}
	aggregatedSignature, valBitSet, err := vote.AggregateSignatureAndValidatorBitSet(votes, validators)
	if err != nil {
		return err
	}

	//relayerBlsPubKeys, err := s.executor.GetValidatorsBlsPublicKey()
	//if err != nil {
	//	return err
	//}
	//
	//relayerPubKey := util.BlsPubKeyFromPrivKeyStr(s.votePoolExecutor.GetBlsPrivateKey())
	//relayerIdx := util.IndexOf(hex.EncodeToString(relayerPubKey), relayerBlsPubKeys)
	//firstInturnRelayerIdx := int(event.Height) % len(relayerBlsPubKeys)
	//txRelayStartTime := tx.TxTime + s.config.RelayConfig.GreenfieldToBSCRelayingDelayTime
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
	//	curRelayerRelayingStartTime = txRelayStartTime + s.config.RelayConfig.FirstInTurnRelayerRelayingWindow + int64(indexDiff-1)*s.config.RelayConfig.InTurnRelayerRelayingWindow
	//}
	//logging.Logger.Infof("current relayer starts relaying from %d", curRelayerRelayingStartTime)

	// submit transaction
	attested := make(chan struct{})
	errC := make(chan error)
	go s.checkSubmitStatus(attested, errC, event.ChallengeId)

	ticker := time.NewTicker(common.RetryInterval)
	defer ticker.Stop()
	triedTimes := 0
	for {
		select {
		case err = <-errC:
			return err
		case <-attested:
			if err = s.UpdateEventStatus(event.ChallengeId, model.Submitted); err != nil {
				return err
			}
			return nil
		case <-ticker.C:
			triedTimes++
			if triedTimes > SubmitTxMaxRetry {
				alert.SendTelegramMessage(s.config.AlertConfig.Identity, s.config.AlertConfig.TelegramChatId, s.config.AlertConfig.TelegramBotId, fmt.Sprintf("failed to submit tx for challenge after retry, id: %d", event.ChallengeId))
				logging.Logger.Infof("failed to submit tx for challenge after retry, id: %d", event.ChallengeId)
				return s.UpdateEventStatus(event.ChallengeId, model.SubmitFailed)
			}
			logging.Logger.Infof("submit tx for challenge, id: %d", event.ChallengeId)
			txHash, errTx := s.SubmitTx(event, valBitSet, aggregatedSignature)
			if errTx != nil {
				logging.Logger.Errorf("failed to submitted tx,  err: %s", errTx.Error())
			} else {
				logging.Logger.Infof("tx submitted, hash: %s", txHash)
			}
		}
	}
}

func (s TxSubmitter) checkSubmitStatus(attested chan struct{}, errC chan error, challengeId uint64) {
	ticker := time.NewTicker(common.RetryInterval)
	defer ticker.Stop()
	for range ticker.C {
		attestedChallengeId, err := s.executor.QueryLatestAttestedChallenge()
		if err != nil {
			errC <- err
		}
		if challengeId <= attestedChallengeId {
			logging.Logger.Infof("challenge %d has already been attested ", challengeId)
			attested <- struct{}{}
		}
	}
}