package metrics

import (
	"fmt"
	"net/http"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"

	"github.com/bnb-chain/greenfield-challenger/config"
)

const (
	// Monitor
	MetricGnfdSavedBlock = "gnfd_saved_block"
	MetricGnfdSavedEvent = "gnfd_saved_event"
	// Verifier
	MetricVerifiedChallenges   = "verified_challenges"
	MetricHashVerifierDuration = "hash_verifier_duration"
	MetricChallengeFailed      = "challenge_failed"
	MetricChallengeSuccess     = "challenge_success"
	MetricHashVerifierErr      = "hash_verifier_error_count"
	// Vote Broadcaster
	MetricBroadcastedChallenges = "broadcasted_challenges"
	MetricBroadcasterDuration   = "broadcaster_duration"
	MetricBroadcasterErr        = "broadcaster_error_count"
	// Vote Collator
	MetricCollatedChallenges = "collated_challenges"
	MetricCollatorDuration   = "collator_duration"
	MetricCollatorErr        = "collator_error_count"
	// Tx Submitter
	MetricSubmittedChallenges = "submitted_challenges"
	MetricSubmitterDuration   = "submitter_duration"
	MetricSubmitterErr        = "submitter_error_count"
	// Attest Monitor
	MetricChallengeAttested = "challenge_attested"
	MetricAttestedCount     = "attested_count"
)

type MetricService struct {
	MetricsMap map[string]prometheus.Metric
	cfg        *config.Config
}

func NewMetricService(config *config.Config) *MetricService {
	ms := make(map[string]prometheus.Metric, 0)

	// Monitor
	gnfdSavedBlockMetric := prometheus.NewGauge(prometheus.GaugeOpts{
		Name: MetricGnfdSavedBlock,
		Help: "Saved block height for Greenfield in database",
	})
	ms[MetricGnfdSavedBlock] = gnfdSavedBlockMetric
	prometheus.MustRegister(gnfdSavedBlockMetric)

	gnfdSavedEventMetric := prometheus.NewGauge(prometheus.GaugeOpts{
		Name: MetricGnfdSavedEvent,
		Help: "Saved event challengeId in database",
	})
	ms[MetricGnfdSavedEvent] = gnfdSavedEventMetric
	prometheus.MustRegister(gnfdSavedEventMetric)

	// Hash Verifier
	verifiedChallengesMetric := prometheus.NewGauge(prometheus.GaugeOpts{
		Name: MetricVerifiedChallenges,
		Help: "Verified challenges in database",
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
		Name: MetricChallengeFailed,
		Help: "Failed challenges in database",
	})
	ms[MetricChallengeFailed] = challengeFailedMetric
	prometheus.MustRegister(challengeFailedMetric)

	challengeSuccessMetric := prometheus.NewCounter(prometheus.CounterOpts{
		Name: MetricChallengeSuccess,
		Help: "Succeeded challenges in database",
	})
	ms[MetricChallengeSuccess] = challengeSuccessMetric
	prometheus.MustRegister(challengeSuccessMetric)

	hashVerifierErrCountMetric := prometheus.NewCounter(prometheus.CounterOpts{
		Name: MetricHashVerifierErr,
		Help: "Hash verifier error count",
	})
	ms[MetricHashVerifierErr] = hashVerifierErrCountMetric
	prometheus.MustRegister(hashVerifierErrCountMetric)

	// Broadcaster
	broadcasterErrCountMetric := prometheus.NewCounter(prometheus.CounterOpts{
		Name: MetricBroadcasterErr,
		Help: "Succeeded challenges in database",
	})
	ms[MetricBroadcasterErr] = broadcasterErrCountMetric
	prometheus.MustRegister(broadcasterErrCountMetric)

	broadcastedChallengesMetric := prometheus.NewGauge(prometheus.GaugeOpts{
		Name: MetricBroadcastedChallenges,
		Help: "Broadcasted challengeID",
	})
	ms[MetricBroadcastedChallenges] = broadcastedChallengesMetric
	prometheus.MustRegister(broadcastedChallengesMetric)

	broadcastedDurationMetric := prometheus.NewHistogram(prometheus.HistogramOpts{
		Name: MetricBroadcasterDuration,
		Help: "Broadcaster duration for 1 challenge",
	})
	ms[MetricBroadcasterDuration] = broadcastedDurationMetric
	prometheus.MustRegister(broadcastedDurationMetric)

	// Collator
	collatorErrCountMetric := prometheus.NewCounter(prometheus.CounterOpts{
		Name: MetricCollatorErr,
		Help: "Collator error count",
	})
	ms[MetricCollatorErr] = collatorErrCountMetric
	prometheus.MustRegister(collatorErrCountMetric)

	collatedChallengesMetric := prometheus.NewGauge(prometheus.GaugeOpts{
		Name: MetricCollatedChallenges,
		Help: "Collated challengeID",
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
	submitterErrCountMetric := prometheus.NewCounter(prometheus.CounterOpts{
		Name: MetricSubmitterErr,
		Help: "Submitter error count",
	})
	ms[MetricSubmitterErr] = submitterErrCountMetric
	prometheus.MustRegister(submitterErrCountMetric)

	submitterChallengesMetric := prometheus.NewGauge(prometheus.GaugeOpts{
		Name: MetricSubmittedChallenges,
		Help: "Submitted challengeID",
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
	challengeAttestedMetric := prometheus.NewGauge(prometheus.GaugeOpts{
		Name: MetricChallengeAttested,
		Help: "Attested challengeID",
	})
	ms[MetricChallengeAttested] = challengeAttestedMetric
	prometheus.MustRegister(challengeAttestedMetric)

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

func (m *MetricService) SetGnfdSavedEvent(challengeId uint64) {
	m.MetricsMap[MetricGnfdSavedEvent].(prometheus.Gauge).Set(float64(challengeId))
}

// Hash Verifier
func (m *MetricService) SetVerifiedChallenges(challengeId uint64) {
	m.MetricsMap[MetricVerifiedChallenges].(prometheus.Gauge).Set(float64(challengeId))
}

func (m *MetricService) SetHashVerifierDuration(duration time.Duration) {
	m.MetricsMap[MetricHashVerifierDuration].(prometheus.Histogram).Observe(duration.Seconds())
}

func (m *MetricService) IncChallengeFailed() {
	m.MetricsMap[MetricChallengeFailed].(prometheus.Counter).Inc()
}

func (m *MetricService) IncChallengeSuccess() {
	m.MetricsMap[MetricChallengeSuccess].(prometheus.Counter).Inc()
}

func (m *MetricService) IncHashVerifierErr() {
	m.MetricsMap[MetricHashVerifierErr].(prometheus.Counter).Inc()
}

// Broadcaster
func (m *MetricService) SetBroadcastedChallenges(challengeId uint64) {
	m.MetricsMap[MetricBroadcastedChallenges].(prometheus.Gauge).Set(float64(challengeId))
}

func (m *MetricService) SetBroadcasterDuration(duration time.Duration) {
	m.MetricsMap[MetricBroadcasterDuration].(prometheus.Histogram).Observe(duration.Seconds())
}

func (m *MetricService) IncBroadcasterErr() {
	m.MetricsMap[MetricBroadcasterErr].(prometheus.Counter).Inc()
}

// Collator
func (m *MetricService) SetCollatorChallenges(challengeId uint64) {
	m.MetricsMap[MetricCollatedChallenges].(prometheus.Gauge).Set(float64(challengeId))
}

func (m *MetricService) SetCollatorDuration(duration time.Duration) {
	m.MetricsMap[MetricCollatorDuration].(prometheus.Histogram).Observe(duration.Seconds())
}

func (m *MetricService) IncCollatorErr() {
	m.MetricsMap[MetricCollatorErr].(prometheus.Counter).Inc()
}

// Submitter
func (m *MetricService) SetSubmitterChallenges(challengeId uint64) {
	m.MetricsMap[MetricSubmittedChallenges].(prometheus.Gauge).Set(float64(challengeId))
}

func (m *MetricService) SetSubmitterDuration(duration time.Duration) {
	m.MetricsMap[MetricSubmitterDuration].(prometheus.Histogram).Observe(duration.Seconds())
}

func (m *MetricService) IncSubmitterErr() {
	m.MetricsMap[MetricSubmitterErr].(prometheus.Counter).Inc()
}

// Attest Monitor
func (m *MetricService) SetAttestedChallenges(challengeId uint64) {
	m.MetricsMap[MetricChallengeAttested].(prometheus.Gauge).Set(float64(challengeId))
}

func (m *MetricService) IncChallengeAttested() {
	m.MetricsMap[MetricAttestedCount].(prometheus.Counter).Inc()
}
