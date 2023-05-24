package submitter

import (
	"encoding/hex"
	"fmt"
	"github.com/willf/bitset"
	"time"

	"cosmossdk.io/math"
	"github.com/bnb-chain/greenfield-challenger/common"
	"github.com/bnb-chain/greenfield-challenger/config"
	"github.com/bnb-chain/greenfield-challenger/db/model"
	"github.com/bnb-chain/greenfield-challenger/executor"
	"github.com/bnb-chain/greenfield-challenger/logging"
	"github.com/bnb-chain/greenfield-challenger/vote"
	"github.com/bnb-chain/greenfield/sdk/types"
	challengetypes "github.com/bnb-chain/greenfield/x/challenge/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
)

type TxSubmitter struct {
	config          *config.Config
	executor        *executor.Executor
	cachedEventHash map[uint64][]byte
	feeAmount       sdk.Coins
	DataProvider
}

func NewTxSubmitter(cfg *config.Config, executor *executor.Executor, submitterDataProvider DataProvider) *TxSubmitter {
	feeAmount, ok := math.NewIntFromString(cfg.GreenfieldConfig.FeeAmount)
	if !ok {
		logging.Logger.Errorf("error converting fee_amount to math.Int, fee_amount: ", cfg.GreenfieldConfig.FeeAmount)
	}
	feeCoins := sdk.NewCoins(sdk.NewCoin(cfg.GreenfieldConfig.FeeDenom, feeAmount))

	return &TxSubmitter{
		config:          cfg,
		executor:        executor,
		feeAmount:       feeCoins,
		cachedEventHash: make(map[uint64][]byte, CacheSize),
		DataProvider:    submitterDataProvider,
	}
}

// SubmitTransactionLoop polls for submitter inturn and fetches events for submit.
func (s *TxSubmitter) SubmitTransactionLoop() {
	submitLoopCount := 0

	for {
		// Loop until submitter is inturn to submit
		attestPeriodEnd := s.queryAttestPeriodLoop()
		// Fetch events for submit
		currentHeight := s.executor.GetCachedBlockHeight()
		events, err := s.FetchEventsForSubmit(currentHeight)
		if err != nil {
			logging.Logger.Errorf("tx submitter failed to fetch events for submitting", err)
			continue
		}
		if len(events) == 0 {
			time.Sleep(common.RetryInterval)
			continue
		}
		// Submit events
		for _, event := range events {
			err = s.submitForSingleEvent(event, attestPeriodEnd)
			if err != nil {
				logging.Logger.Errorf("tx submitter err", err)
				continue
			}
			time.Sleep(50 * time.Millisecond)
		}
		// Clear cached event hash
		submitLoopCount++
		if submitLoopCount == common.CacheClearIterations {
			submitLoopCount = 0
			s.clearCachedEventHash()
		}

		time.Sleep(executor.QueryAttestedChallengeInterval)
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
			logging.Logger.Infof("tx submitter is currently inturn for submitting until", time.Unix(int64(res.SubmitInterval.GetEnd()), 0).Format(TimeFormat))
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
		if err.Error() == common.ErrEventExpired.Error() {
			err = s.DataProvider.UpdateEventStatus(event.ChallengeId, model.Expired)
		}
		return err
	}
	// Calculate event hash and use it to fetch votes and validator bitset
	aggregatedSignature, valBitSet, err := s.getSignatureAndBitSet(event)
	if err != nil {
		return err
	}
	return s.submitTransaction(event, attestPeriodEnd, aggregatedSignature, valBitSet)
}

// getEventHash gets the event hash from the cache or calculates it if not present.
func (s *TxSubmitter) getEventHash(event *model.Event) []byte {
	eventHash := s.cachedEventHash[event.ChallengeId]
	if eventHash == nil {
		eventHash = vote.CalculateEventHash(event)
		s.cachedEventHash[event.ChallengeId] = eventHash
	}
	return eventHash
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
func (s *TxSubmitter) submitTransaction(event *model.Event, attestPeriodEnd uint64, aggregatedSignature []byte, valBitSet *bitset.BitSet) error {
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
		txOpts := types.TxOption{
			NoSimulate: s.config.GreenfieldConfig.NoSimulate,
			GasLimit:   s.config.GreenfieldConfig.GasLimit,
			FeeAmount:  s.feeAmount,
			Nonce:      nonce,
		}
		// Submit transaction
		attestRes, err := s.executor.AttestChallenge(s.executor.GetAddr(), event.ChallengerAddress, event.SpOperatorAddress, event.ChallengeId, math.NewUintFromString(event.ObjectId), voteResult, valBitSet.Bytes(), aggregatedSignature, txOpts)
		if err != nil || !attestRes {
			submittedAttempts++
			time.Sleep(100 * time.Millisecond)
			continue
		}
		// Update event status to include in Attest Monitor
		err = s.DataProvider.UpdateEventStatus(event.ChallengeId, model.Submitted)
		return err
	}
}

// clearCachedEventHash clears the cached event hash.
func (s *TxSubmitter) clearCachedEventHash() {
	for key := range s.cachedEventHash {
		delete(s.cachedEventHash, key)
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
