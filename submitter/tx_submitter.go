package submitter

import (
	"encoding/hex"
	"fmt"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"time"

	"cosmossdk.io/math"

	"github.com/bnb-chain/greenfield-challenger/common"
	"github.com/bnb-chain/greenfield-challenger/config"
	"github.com/bnb-chain/greenfield-challenger/db/dao"
	"github.com/bnb-chain/greenfield-challenger/db/model"
	"github.com/bnb-chain/greenfield-challenger/executor"
	"github.com/bnb-chain/greenfield-challenger/logging"
	"github.com/bnb-chain/greenfield-challenger/vote"
	"github.com/bnb-chain/greenfield/sdk/types"
	challengetypes "github.com/bnb-chain/greenfield/x/challenge/types"
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
		attestPeriodEnd := uint64(0)
		for {
			// blsPubKey of current submitter
			res, err := s.executor.QueryInturnAttestationSubmitter()
			if err != nil {
				logging.Logger.Errorf("tx submitter failed to query inturn attestation submitter, err=%+v", err.Error())
				continue
			}
			attestPeriodEnd = res.SubmitInterval.GetEnd()
			if res.BlsPubKey == hex.EncodeToString(s.executor.BlsPubKey) {
				logging.Logger.Infof("tx submitter is currently inturn for submitting until: %s, current timestamp: %s", time.Unix(int64(attestPeriodEnd), 0).Format("15:04:05.000000"), time.Now().Format("15:04:05.000000"))
				break
			}
			time.Sleep(common.RetryInterval)
			continue
		}

		currentHeight := s.executor.GetCachedBlockHeight()
		events, err := s.FetchEventsForSubmit(currentHeight)
		logging.Logger.Infof("tx submitter fetched %d events for submit", len(events))
		if err != nil {
			logging.Logger.Errorf("tx submitter failed to fetch events for submitting, err=%+v", err.Error())
			continue
		}
		if len(events) == 0 {
			time.Sleep(common.RetryInterval)
			continue
		}

		for _, event := range events {
			err = s.submitForSingleEvent(event, attestPeriodEnd)
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
		time.Sleep(executor.QueryAttestedChallengeInterval)
	}
}

func (s *TxSubmitter) submitForSingleEvent(event *model.Event, attestPeriodEnd uint64) error {
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
	submittedAttempts := 0
	for {
		logging.Logger.Infof("current time: %d, attestPeriodEnd: %d", time.Now().Unix(), attestPeriodEnd)
		if time.Now().Unix() > int64(attestPeriodEnd) {
			return fmt.Errorf("submit interval ended for submitter. failed to submit in time for challengeId: %d, timestamp: %s", event.ChallengeId, time.Now().Format("15:04:05.000000"))
		}
		if submittedAttempts > common.MaxSubmitAttempts {
			return fmt.Errorf("submitter exceeded max submit attempts for challengeId: %d, timestamp: %s", event.ChallengeId, time.Now().Format("15:04:05.000000"))
		}
		nonce, err := s.executor.GetNonce()
		if err != nil {
			logging.Logger.Errorf("submitter failed to get nonce for challengeId: %d, timestamp: %s, err=%+v", event.ChallengeId, time.Now().Format("15:04:05.000000"), err.Error())
			continue
		}
		txOpts := types.TxOption{
			NoSimulate: true,
			GasLimit:   1000,
			FeeAmount:  sdk.NewCoins(sdk.NewCoin("BNB", sdk.NewInt(int64(5000000000000)))),
			Nonce:      nonce,
		}
		attestRes, err := s.executor.AttestChallenge(s.executor.GetAddr(), event.ChallengerAddress, event.SpOperatorAddress, event.ChallengeId, math.NewUintFromString(event.ObjectId), challengetypes.CHALLENGE_SUCCEED, valBitSet.Bytes(), aggregatedSignature, txOpts)
		if err != nil || !attestRes {
			submittedAttempts++
			time.Sleep(100 * time.Millisecond)
			continue
		}
		logging.Logger.Errorf("attestation submitted successfully, challengeId: %d, timestamp: %s", event.ChallengeId, time.Now().Format("15:04:05.000000"))
		err = s.daoManager.UpdateEventStatusByChallengeId(event.ChallengeId, model.Submitted)
		return err
	}
}

func (s *TxSubmitter) preCheck(event *model.Event) error {
	currentHeight := s.executor.GetCachedBlockHeight()
	if event.ExpiredHeight < currentHeight {
		logging.Logger.Infof("submitter for challengeId: %d has expired. expired height: %d, current height: %d, timestamp: %s", event.ChallengeId, event.ExpiredHeight, currentHeight, time.Now().Format("15:04:05.000000"))
		return fmt.Errorf("event %d has expired", event.ChallengeId)
	}
	return nil
}
