package submitter

import (
	"cosmossdk.io/math"
	"encoding/hex"
	"fmt"
	"time"

	"github.com/bnb-chain/greenfield-challenger/common"
	"github.com/bnb-chain/greenfield-challenger/config"
	"github.com/bnb-chain/greenfield-challenger/db/model"
	"github.com/bnb-chain/greenfield-challenger/executor"
	"github.com/bnb-chain/greenfield-challenger/logging"
	"github.com/bnb-chain/greenfield-challenger/metrics"
	"github.com/bnb-chain/greenfield-challenger/vote"
	"github.com/bnb-chain/greenfield/sdk/types"
	challengetypes "github.com/bnb-chain/greenfield/x/challenge/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/types/tx"
	lru "github.com/hashicorp/golang-lru"
	"github.com/willf/bitset"
)

type TxSubmitter struct {
	config          *config.Config
	executor        *executor.Executor
	cachedEventHash *lru.Cache
	feeAmount       sdk.Coins
	DataProvider
	metricService *metrics.MetricService
}

func NewTxSubmitter(cfg *config.Config, executor *executor.Executor, submitterDataProvider DataProvider, metricService *metrics.MetricService) *TxSubmitter {
	feeAmount, ok := math.NewIntFromString(cfg.GreenfieldConfig.FeeAmount)
	if !ok {
		logging.Logger.Errorf("error converting fee_amount to math.Int, fee_amount: ", cfg.GreenfieldConfig.FeeAmount)
	}
	// else set default value
	feeCoins := sdk.NewCoins(sdk.NewCoin(cfg.GreenfieldConfig.FeeDenom, feeAmount))

	cacheSize := 1000
	lruCache, _ := lru.New(cacheSize)

	return &TxSubmitter{
		config:          cfg,
		executor:        executor,
		feeAmount:       feeCoins,
		cachedEventHash: lruCache,
		DataProvider:    submitterDataProvider,
		metricService:   metricService,
	}
}

// SubmitTransactionLoop polls for submitter inturn and fetches events for submit.
func (s *TxSubmitter) SubmitTransactionLoop() {
	ticker := time.NewTicker(TxSubmitLoopInterval)
	for range ticker.C {
		// Loop until submitter is inturn to submit
		attestPeriodEnd := s.queryAttestPeriodLoop()
		// Fetch events for submit
		currentHeight := s.executor.GetCachedBlockHeight()
		events, err := s.FetchEventsForSubmit(currentHeight)
		if err != nil {
			s.metricService.IncSubmitterErr()
			logging.Logger.Errorf("tx submitter failed to fetch events for submitting", err)
			continue
		}
		if len(events) == 0 {
			time.Sleep(common.RetryInterval)
			continue
		}
		// Submit events
		for _, event := range events {
			// Submitter no longer in-turn
			if time.Now().Unix() > int64(attestPeriodEnd) {
				break
			}
			err = s.submitForSingleEvent(event, attestPeriodEnd)
			if err != nil {
				logging.Logger.Errorf("tx submitter ran into an error while trying to attest, err=%+v", err.Error())
				continue
			}
			time.Sleep(TxSubmitInterval)
		}
	}
}

// queryAttestPeriodLoop loops until submitter is inturn and return the end time of the current attestation period.
func (s *TxSubmitter) queryAttestPeriodLoop() uint64 {
	for {
		res, err := s.executor.QueryInturnAttestationSubmitter()
		if err != nil {
			continue
		}
		// Submitter is inturn if bls key matches
		if res.BlsPubKey == hex.EncodeToString(s.executor.BlsPubKey) {
			logging.Logger.Infof("tx submitter is currently inturn for submitting until %s", time.Unix(int64(res.SubmitInterval.GetEnd()), 0).Format(TimeFormat))
			return res.SubmitInterval.GetEnd()
		}

		time.Sleep(common.RetryInterval)
	}
}

// submitForSingleEvent fetches required data and submits a single event.
func (s *TxSubmitter) submitForSingleEvent(event *model.Event, attestPeriodEnd uint64) error {
	logging.Logger.Infof("submitter started for challengeId: %d", event.ChallengeId)
	// Check if events expired
	err := s.preCheck(event)
	if err != nil {
		return err
	}
	// Calculate event hash and use it to fetch votes and validator bitset
	aggregatedSignature, valBitSet, err := s.getSignatureAndBitSet(event)
	if err != nil {
		s.metricService.IncSubmitterErr()
		return err
	}
	return s.submitTransactionLoop(event, attestPeriodEnd, aggregatedSignature, valBitSet)
}

// getEventHash gets the event hash from the cache or calculates it if not present.
func (s *TxSubmitter) getEventHash(event *model.Event) []byte {
	eventHash, found := s.cachedEventHash.Get(event.ChallengeId)
	if found {
		return eventHash.([]byte)
	}
	calculatedEventHash := vote.CalculateEventHash(event, s.config.GreenfieldConfig.ChainIdString)
	s.cachedEventHash.Add(event.ChallengeId, calculatedEventHash)
	return calculatedEventHash
}

func (s *TxSubmitter) getSignatureAndBitSet(event *model.Event) ([]byte, *bitset.BitSet, error) {
	eventHash := s.getEventHash(event)
	votes, err := s.FetchVotesForAggregation(hex.EncodeToString(eventHash))
	if err != nil {
		logging.Logger.Errorf("submitter failed to get votes for event with challengeId", event.ChallengeId, err)
		return nil, nil, err
	}
	validators, err := s.executor.QueryCachedLatestValidators()
	if err != nil {
		logging.Logger.Errorf("submitter failed to query validators for event with challenge id", event.ChallengeId, err)
		return nil, nil, err
	}
	aggregatedSignature, valBitSet, err := vote.AggregateSignatureAndValidatorBitSet(votes, validators)
	if err != nil {
		logging.Logger.Errorf("submitter failed to aggregate signature for event with challenge id", event.ChallengeId, err)
		return nil, nil, err
	}
	return aggregatedSignature, valBitSet, nil
}

// submitTransaction creates and submits the transaction.
func (s *TxSubmitter) submitTransactionLoop(event *model.Event, attestPeriodEnd uint64, aggregatedSignature []byte, valBitSet *bitset.BitSet) error {
	startTime := time.Now()
	submittedAttempts := 0
	for {
		if time.Now().Unix() > int64(attestPeriodEnd) {
			return fmt.Errorf("submit interval ended for submitter. failed to submit in time for challengeId: %d", event.ChallengeId)
		}

		if submittedAttempts > common.MaxSubmitAttempts {
			return fmt.Errorf("submitter exceeded max submit attempts for challengeId: %d", event.ChallengeId)
		}

		voteResult := challengetypes.CHALLENGE_FAILED
		if event.VerifyResult == model.HashMismatched {
			voteResult = challengetypes.CHALLENGE_SUCCEED
		}
		// Create transaction options
		nonce, err := s.executor.GetNonce()
		if err != nil {
			logging.Logger.Errorf("submitter failed to get nonce for challengeId", event.ChallengeId, err)
			continue
		}
		mode := tx.BroadcastMode_BROADCAST_MODE_SYNC
		txOpts := types.TxOption{
			GasLimit:  s.config.GreenfieldConfig.GasLimit,
			FeeAmount: s.feeAmount,
			Nonce:     nonce,
			Mode:      &mode,
		}
		// Submit transaction
		attestRes, err := s.executor.AttestChallenge(s.executor.GetAddr(), event.ChallengerAddress, event.SpOperatorAddress, event.ChallengeId, math.NewUintFromString(event.ObjectId), voteResult, valBitSet.Bytes(), aggregatedSignature, txOpts)
		if err != nil || !attestRes {
			if err != nil {
				logging.Logger.Errorf("submitter failed for challengeId: %d, attempts: %d, err=%+v", event.ChallengeId, submittedAttempts, err.Error())
			}
			s.metricService.IncSubmitterErr()
			submittedAttempts++
			time.Sleep(TxSubmitInterval)
			continue
		}
		// Update event status to include in Attest Monitor
		err = s.DataProvider.UpdateEventStatus(event.ChallengeId, model.Submitted)
		if err != nil {
			s.metricService.IncSubmitterErr()
			logging.Logger.Errorf("submitter succeeded in attesting but failed to update database, err=%+v", err.Error())
			continue
		}

		elaspedTime := time.Since(startTime)
		s.metricService.SetSubmitterDuration(elaspedTime)
		s.metricService.IncSubmittedChallenges()
		logging.Logger.Infof("submitter metrics increased for challengeId %d, elasped time %+v", event.ChallengeId, elaspedTime)
		return err
	}
}

// preCheck checks if the event has expired.
func (s *TxSubmitter) preCheck(event *model.Event) error {
	currentHeight := s.executor.GetCachedBlockHeight()
	if event.ExpiredHeight < currentHeight {
		logging.Logger.Infof("submitter for challengeId has expired. expired height, current height", event.ChallengeId, event.ExpiredHeight, currentHeight)
		return common.ErrEventExpired
	}
	return nil
}
