package submitter

import (
	"encoding/hex"
	"fmt"
	"time"

	"github.com/bnb-chain/greenfield-challenger/db/dao"

	"github.com/bnb-chain/greenfield-challenger/common"
	"github.com/bnb-chain/greenfield-challenger/config"
	"github.com/bnb-chain/greenfield-challenger/db/model"
	"github.com/bnb-chain/greenfield-challenger/executor"
	"github.com/bnb-chain/greenfield-challenger/logging"
	"github.com/bnb-chain/greenfield-challenger/vote"
)

type TxSubmitter struct {
	config          *config.Config
	daoManager      *dao.DaoManager
	executor        *executor.Executor
	cachedEventHash map[uint64][]byte
	DataProvider
}

func NewTxSubmitter(cfg *config.Config, executor *executor.Executor, daoManager *dao.DaoManager,
	submitterKind DataProvider,
) *TxSubmitter {
	return &TxSubmitter{
		config:       cfg,
		daoManager:   daoManager,
		executor:     executor,
		DataProvider: submitterKind,
	}
}

func (s *TxSubmitter) SubmitTransactionLoop() {
	// Event lasts for 300 blocks, 2x for redundancy
	s.cachedEventHash = make(map[uint64][]byte, common.CacheSize)
	submitLoopCount := 0
	for {
		currentHeight := s.executor.GetCachedBlockHeight()
		events, err := s.FetchEventsForSubmit(currentHeight)
		logging.Logger.Infof("tx submitter fetched %d events for submit", len(events))
		if err != nil {
			logging.Logger.Errorf("tx submitter failed to fetch events for submitting, err=%+v", err.Error())
			continue
		}
		if len(events) == 0 {
			logging.Logger.Infof("tx submitter fetched 0 events for submit, retrying", len(events))
			time.Sleep(common.RetryInterval)
			continue
		}

		for _, event := range events {
			err = s.submitForSingleEvent(event)
			if err != nil {
				logging.Logger.Errorf("tx submitter err=%+v, timestamp: %s", err.Error(), time.Now().Format("15:04:05.000000"))
				continue
			}
			time.Sleep(50 * time.Millisecond)
		}
		// clear event hash every N attempts
		submitLoopCount++
		if submitLoopCount == common.CacheClearIterations {
			submitLoopCount = 0
			s.cachedEventHash = make(map[uint64][]byte, common.CacheSize)
		}
		time.Sleep(common.RetryInterval)
	}
}

func (s *TxSubmitter) submitForSingleEvent(event *model.Event) error {
	logging.Logger.Infof("submitter process started for event with challengeId: %d, timestamp: %s, err=%+v", event.ChallengeId, time.Now().Format("15:04:05.000000"))
	if err := s.preCheck(event); err != nil {
		return err
	}

	// Get votes result for s tx, which are already validated and qualified to aggregate sig
	eventHash := s.cachedEventHash[event.ChallengeId]
	if eventHash == nil {
		eventHash = vote.CalculateEventHash(event)
		s.cachedEventHash[event.ChallengeId] = eventHash
	}
	votes, err := s.FetchVotesForAggregation(hex.EncodeToString(eventHash))
	if err != nil {
		logging.Logger.Errorf("submitter failed to get votes for event with challengeId: %d, timestamp: %s, err=%+v", event.ChallengeId, time.Now().Format("15:04:05.000000"), err.Error())
		return err
	}
	validators, err := s.executor.QueryCachedLatestValidators()
	if err != nil {
		logging.Logger.Errorf("submitter failed to query validators for event with challenge id %d, timestamp: %s, err=%+v", event.ChallengeId, time.Now().Format("15:04:05.000000"), err.Error())
		return err
	}
	aggregatedSignature, valBitSet, err := vote.AggregateSignatureAndValidatorBitSet(votes, validators)
	if err != nil {
		logging.Logger.Errorf("submitter failed to aggregate signature for event with challenge id %d, timestamp: %s, err=%+v", event.ChallengeId, time.Now().Format("15:04:05.000000"), err.Error())
		return err
	}

	// submit transaction
	txHash, errTx := s.SubmitTx(event, valBitSet, aggregatedSignature)
	if errTx != nil {
		logging.Logger.Errorf("failed to submit tx for challengeId: %d,  err: %s", event.ChallengeId, errTx.Error())
	} else {
		logging.Logger.Infof("submitter tx submitted for challengeId: %d, txHash: %s ,timestamp: %s", event.ChallengeId, txHash, time.Now().Format("15:04:05.000000"))
	}
	time.Sleep(100 * time.Millisecond)

	// check if attested
	err = s.checkSubmitStatus(event.ChallengeId)
	if err != nil {
		return err
	}
	err = s.daoManager.UpdateEventStatusByChallengeId(event.ChallengeId, model.Submitted)
	if err != nil {
		return err
	}
	logging.Logger.Infof("attestation completed for challengeId: %d, timestamp: %s", event.ChallengeId, time.Now().Format("15:04:05.000000"))
	return nil
}

func (s *TxSubmitter) preCheck(event *model.Event) error {
	currentHeight := s.executor.GetCachedBlockHeight()
	if event.ExpiredHeight < currentHeight {
		logging.Logger.Infof("submitter for challengeId: %d has expired. expired height: %d, current height: %d, timestamp: %s", event.ChallengeId, event.ExpiredHeight, currentHeight, time.Now().Format("15:04:05.000000"))
		return fmt.Errorf("event %d has expired", event.ChallengeId)
	}
	return nil
}

func (s *TxSubmitter) checkSubmitStatus(challengeId uint64) error {
	attestedChallengeId, err := s.executor.QueryLatestAttestedChallengeId()
	logging.Logger.Infof("latest attested challengeId: %d", attestedChallengeId)
	if err != nil {
		return err
	}
	if challengeId == attestedChallengeId {
		logging.Logger.Infof("attestation verified for challengeId: %d, current attested challenge: %d, timestamp: %s", challengeId, attestedChallengeId, time.Now().Format("15:04:05.000000"))
		return nil
	}
	return fmt.Errorf("checking submit status for challengeId: %d, not attested on blockchain yet, timestamp: %s", challengeId, time.Now().Format("15:04:05.000000"))
}
