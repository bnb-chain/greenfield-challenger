package metrics

import (
	"fmt"
	"github.com/bnb-chain/greenfield-challenger/logging"
	"net/http"
	"time"

	"github.com/bnb-chain/greenfield-challenger/config"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

const (
	// Monitor
	MetricGnfdSavedBlock      = "gnfd_saved_block"
	MetricGnfdSavedBlockCount = "gnfd_saved_block_count"
	MetricGnfdSavedEvent      = "gnfd_saved_event"
	MetricGnfdSavedEventCount = "gnfd_saved_event_count"

	// Verifier
	MetricVerifiedChallenges       = "verified_challenges"
	MetricVerifiedChallengeFailed  = "challenge_failed"
	MetricVerifiedChallengeSuccess = "challenge_success"
	MetricHeartbeatEvents          = "heartbeat_events"
	MetricHashVerifierErr          = "hash_verifier_error_count"
	MetricSpAPIErr                 = "hash_verifier_sp_api_error"
	MetricHashVerifierDuration     = "hash_verifier_duration"

	// Vote Broadcaster
	MetricBroadcastedChallenges = "broadcasted_challenges"
	MetricBroadcasterDuration   = "broadcaster_duration"
	MetricBroadcasterErr        = "broadcaster_error_count"

	// Vote Collector
	MetricsVoteCollectorErr = "vote_collector_error_count"
	MetricsVotesCollected   = "votes_collected"

	// Vote Collator
	MetricCollatedChallenges = "collated_challenges"
	MetricCollatorDuration   = "collator_duration"
	MetricCollatorErr        = "collator_error_count"

	// Tx Submitter
	MetricSubmittedChallenges = "submitted_challenges"
	MetricSubmitterDuration   = "submitter_duration"
	MetricSubmitterErr        = "submitter_error_count"

	// Attest Monitor
	MetricAttestedCount = "attested_count"
)

type MetricService struct {
	MetricsMap map[string]prometheus.Collector
	cfg        *config.Config
}

func NewMetricService(config *config.Config) *MetricService {
	ms := make(map[string]prometheus.Collector, 0)

	// Monitor
	gnfdSavedBlockMetric := prometheus.NewGauge(prometheus.GaugeOpts{
		Name: MetricGnfdSavedBlock,
		Help: "Saved block height for Greenfield in database",
	})
	ms[MetricGnfdSavedBlock] = gnfdSavedBlockMetric
	prometheus.MustRegister(gnfdSavedBlockMetric)

	gnfdSavedBlockCountMetric := prometheus.NewCounter(prometheus.CounterOpts{
		Name: MetricGnfdSavedBlockCount,
		Help: "Saved block count for Greenfield in database",
	})
	ms[MetricGnfdSavedBlockCount] = gnfdSavedBlockCountMetric
	prometheus.MustRegister(gnfdSavedBlockCountMetric)

	gnfdSavedEventMetric := prometheus.NewGauge(prometheus.GaugeOpts{
		Name: MetricGnfdSavedEvent,
		Help: "Saved event challengeId in database",
	})
	ms[MetricGnfdSavedEvent] = gnfdSavedEventMetric
	prometheus.MustRegister(gnfdSavedEventMetric)

	gnfdSavedEventCountMetric := prometheus.NewCounter(prometheus.CounterOpts{
		Name: MetricGnfdSavedEventCount,
		Help: "Saved gnfd event count in database",
	})
	ms[MetricGnfdSavedEventCount] = gnfdSavedEventCountMetric
	prometheus.MustRegister(gnfdSavedEventCountMetric)

	// Hash Verifier
	verifiedChallengesMetric := prometheus.NewCounter(prometheus.CounterOpts{
		Name: MetricVerifiedChallenges,
		Help: "Verified challenge count",
	})
	ms[MetricVerifiedChallenges] = verifiedChallengesMetric
	prometheus.MustRegister(verifiedChallengesMetric)

	hashVerifierDurationMetric := prometheus.NewHistogram(prometheus.HistogramOpts{
		Name: MetricHashVerifierDuration,
		Help: "Duration of the hash verifier process for each challenge ID",
	})
	ms[MetricHashVerifierDuration] = hashVerifierDurationMetric
	prometheus.MustRegister(hashVerifierDurationMetric)

	challengeFailedMetric := prometheus.NewCounter(prometheus.CounterOpts{
		Name: MetricVerifiedChallengeFailed,
		Help: "Failed challenges in database",
	})
	ms[MetricVerifiedChallengeFailed] = challengeFailedMetric
	prometheus.MustRegister(challengeFailedMetric)

	challengeSuccessMetric := prometheus.NewCounter(prometheus.CounterOpts{
		Name: MetricVerifiedChallengeSuccess,
		Help: "Succeeded challenges in database",
	})
	ms[MetricVerifiedChallengeSuccess] = challengeSuccessMetric
	prometheus.MustRegister(challengeSuccessMetric)

	heartbeatEventsMetric := prometheus.NewCounter(prometheus.CounterOpts{
		Name: MetricHeartbeatEvents,
		Help: "Heartbeat challenges",
	})
	ms[MetricHeartbeatEvents] = heartbeatEventsMetric
	prometheus.MustRegister(heartbeatEventsMetric)

	hashVerifierErrCountMetric := prometheus.NewCounter(prometheus.CounterOpts{
		Name: MetricHashVerifierErr,
		Help: "Hash verifier error count",
	})
	ms[MetricHashVerifierErr] = hashVerifierErrCountMetric
	prometheus.MustRegister(hashVerifierErrCountMetric)

	hashVerifierSpApiErrCountMetric := prometheus.NewCounter(prometheus.CounterOpts{
		Name: MetricSpAPIErr,
		Help: "Hash verifier SP API error count",
	})
	ms[MetricSpAPIErr] = hashVerifierSpApiErrCountMetric
	prometheus.MustRegister(hashVerifierSpApiErrCountMetric)

	// Broadcaster
	broadcasterErrCountMetric := prometheus.NewCounter(prometheus.CounterOpts{
		Name: MetricBroadcasterErr,
		Help: "Broadcaster error count",
	})
	ms[MetricBroadcasterErr] = broadcasterErrCountMetric
	prometheus.MustRegister(broadcasterErrCountMetric)

	broadcastedChallengesMetric := prometheus.NewCounter(prometheus.CounterOpts{
		Name: MetricBroadcastedChallenges,
		Help: "Broadcasted challenge count",
	})
	ms[MetricBroadcastedChallenges] = broadcastedChallengesMetric
	prometheus.MustRegister(broadcastedChallengesMetric)

	broadcastedDurationMetric := prometheus.NewHistogram(prometheus.HistogramOpts{
		Name: MetricBroadcasterDuration,
		Help: "Broadcaster duration for 1 challenge",
	})
	ms[MetricBroadcasterDuration] = broadcastedDurationMetric
	prometheus.MustRegister(broadcastedDurationMetric)

	// Vote Collector
	voteCollectorErrCountMetric := prometheus.NewCounter(prometheus.CounterOpts{
		Name: MetricsVoteCollectorErr,
		Help: "Vote Collector error count",
	})
	ms[MetricsVoteCollectorErr] = voteCollectorErrCountMetric
	prometheus.MustRegister(voteCollectorErrCountMetric)

	votesCollectedMetric := prometheus.NewCounter(prometheus.CounterOpts{
		Name: MetricsVotesCollected,
		Help: "Votes collected count",
	})
	ms[MetricsVotesCollected] = votesCollectedMetric
	prometheus.MustRegister(votesCollectedMetric)

	// Collator
	collatorErrCountMetric := prometheus.NewCounter(prometheus.CounterOpts{
		Name: MetricCollatorErr,
		Help: "Collator error count",
	})
	ms[MetricCollatorErr] = collatorErrCountMetric
	prometheus.MustRegister(collatorErrCountMetric)

	collatedChallengesMetric := prometheus.NewCounter(prometheus.CounterOpts{
		Name: MetricCollatedChallenges,
		Help: "Collated challenge count",
	})
	ms[MetricCollatedChallenges] = collatedChallengesMetric
	prometheus.MustRegister(collatedChallengesMetric)

	collatedDurationMetric := prometheus.NewHistogram(prometheus.HistogramOpts{
		Name: MetricCollatorDuration,
		Help: "Collator duration for 1 challenge",
	})
	ms[MetricCollatorDuration] = collatedDurationMetric
	prometheus.MustRegister(collatedDurationMetric)

	// Submitter
	submitterErrCountMetric := prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: MetricSubmitterErr,
			Help: "Submitter error count",
		},
		[]string{"challengeId", "error"},
	)
	ms[MetricSubmitterErr] = submitterErrCountMetric
	prometheus.MustRegister(submitterErrCountMetric)

	submitterChallengesMetric := prometheus.NewCounter(prometheus.CounterOpts{
		Name: MetricSubmittedChallenges,
		Help: "Submitted challenge count",
	})
	ms[MetricSubmittedChallenges] = submitterChallengesMetric
	prometheus.MustRegister(submitterChallengesMetric)

	submitterDurationMetric := prometheus.NewHistogram(prometheus.HistogramOpts{
		Name: MetricSubmitterDuration,
		Help: "Submitter duration for 1 challengeID",
	})
	ms[MetricSubmitterDuration] = submitterDurationMetric
	prometheus.MustRegister(submitterDurationMetric)

	// Attest Monitor
	challengeAttestedCountMetric := prometheus.NewCounter(prometheus.CounterOpts{
		Name: MetricAttestedCount,
		Help: "Attested challenges count",
	})
	ms[MetricAttestedCount] = challengeAttestedCountMetric
	prometheus.MustRegister(challengeAttestedCountMetric)

	return &MetricService{
		MetricsMap: ms,
		cfg:        config,
	}
}

func (m *MetricService) Start() {
	http.Handle("/metrics", promhttp.Handler())
	err := http.ListenAndServe(fmt.Sprintf(":%d", m.cfg.MetricsConfig.Port), nil)
	if err != nil {
		panic(err)
	}
}

// Monitor
func (m *MetricService) SetGnfdSavedBlock(height uint64) {
	m.MetricsMap[MetricGnfdSavedBlock].(prometheus.Gauge).Set(float64(height))
}

func (m *MetricService) IncGnfdSavedBlockCount() {
	m.MetricsMap[MetricGnfdSavedBlockCount].(prometheus.Counter).Inc()
}

func (m *MetricService) SetGnfdSavedEvent(challengeId uint64) {
	m.MetricsMap[MetricGnfdSavedEvent].(prometheus.Gauge).Set(float64(challengeId))
}

func (m *MetricService) IncGnfdSavedEventCount() {
	m.MetricsMap[MetricGnfdSavedEventCount].(prometheus.Counter).Inc()
}

// Hash Verifier
func (m *MetricService) IncVerifiedChallenges() {
	m.MetricsMap[MetricVerifiedChallenges].(prometheus.Counter).Inc()
}

func (m *MetricService) SetHashVerifierDuration(duration time.Duration) {
	m.MetricsMap[MetricHashVerifierDuration].(prometheus.Histogram).Observe(duration.Seconds())
}

func (m *MetricService) IncChallengeFailed() {
	m.MetricsMap[MetricVerifiedChallengeFailed].(prometheus.Counter).Inc()
}

func (m *MetricService) IncChallengeSuccess() {
	m.MetricsMap[MetricVerifiedChallengeSuccess].(prometheus.Counter).Inc()
}

func (m *MetricService) IncHeartbeatEvents() {
	m.MetricsMap[MetricHeartbeatEvents].(prometheus.Counter).Inc()
}

func (m *MetricService) IncHashVerifierErr(err error) {
	if err != nil {
		logging.Logger.Errorf("verifier error count increased, %s", err.Error())
	}
	m.MetricsMap[MetricHashVerifierErr].(prometheus.Counter).Inc()
}

func (m *MetricService) IncHashVerifierSpApiErr(err error) {
	if err != nil {
		logging.Logger.Errorf("verifier sp api error count increased, %s", err.Error())
	}
	m.MetricsMap[MetricSpAPIErr].(prometheus.Counter).Inc()
}

// Broadcaster
func (m *MetricService) IncBroadcastedChallenges() {
	m.MetricsMap[MetricBroadcastedChallenges].(prometheus.Counter).Inc()
}

func (m *MetricService) SetBroadcasterDuration(duration time.Duration) {
	m.MetricsMap[MetricBroadcasterDuration].(prometheus.Histogram).Observe(duration.Seconds())
}

func (m *MetricService) IncBroadcasterErr(err error) {
	logging.Logger.Errorf("broadcaster error count increased, %s", err.Error())
	m.MetricsMap[MetricBroadcasterErr].(prometheus.Counter).Inc()
}

// Vote Collector
func (m *MetricService) IncVoteCollectorErr(err error) {
	if err != nil {
		logging.Logger.Errorf("vote collector error count increased, %s", err.Error())
	}
	m.MetricsMap[MetricsVoteCollectorErr].(prometheus.Counter).Inc()
}

func (m *MetricService) IncVotesCollected() {
	m.MetricsMap[MetricsVotesCollected].(prometheus.Counter).Inc()
}

// Collator
func (m *MetricService) IncCollatedChallenges() {
	m.MetricsMap[MetricCollatedChallenges].(prometheus.Counter).Inc()
}

func (m *MetricService) SetCollatorDuration(duration time.Duration) {
	m.MetricsMap[MetricCollatorDuration].(prometheus.Histogram).Observe(duration.Seconds())
}

func (m *MetricService) IncCollatorErr(err error) {
	if err != nil {
		logging.Logger.Errorf("collator error count increased, %s", err.Error())
	}
	m.MetricsMap[MetricCollatorErr].(prometheus.Counter).Inc()
}

// Submitter
func (m *MetricService) IncSubmittedChallenges() {
	m.MetricsMap[MetricSubmittedChallenges].(prometheus.Counter).Inc()
}

func (m *MetricService) SetSubmitterDuration(duration time.Duration) {
	m.MetricsMap[MetricSubmitterDuration].(prometheus.Histogram).Observe(duration.Seconds())
}

func (m *MetricService) IncSubmitterErr(challengeId string, err error) {
	if err != nil {
		logging.Logger.Errorf("submitter error count increased, %s", err.Error())
		metric, ok := m.MetricsMap[MetricSubmitterErr].(prometheus.CounterVec)
		if ok {
			metric.WithLabelValues(challengeId, err.Error()).Inc()
		}
	}
}

// Attest Monitor
func (m *MetricService) IncAttestedChallenges() {
	m.MetricsMap[MetricAttestedCount].(prometheus.Counter).Inc()
}
