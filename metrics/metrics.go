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
	// speed, errors
	// challengeId, starttime, endtime
	// error inc
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

	challengeFailedMetric := prometheus.NewGauge(prometheus.GaugeOpts{
		Name: MetricChallengeFailed,
		Help: "Failed challenges in database",
	})
	ms[MetricChallengeFailed] = challengeFailedMetric
	prometheus.MustRegister(challengeFailedMetric)

	challengeSuccessMetric := prometheus.NewGauge(prometheus.GaugeOpts{
		Name: MetricChallengeSuccess,
		Help: "Succeeded challenges in database",
	})
	ms[MetricChallengeSuccess] = challengeSuccessMetric
	prometheus.MustRegister(challengeSuccessMetric)

	challengeAttestedMetric := prometheus.NewGauge(prometheus.GaugeOpts{
		Name: MetricChallengeAttested,
		Help: "Saved block height for Greenfield in database",
	})
	ms[MetricChallengeAttested] = challengeAttestedMetric
	prometheus.MustRegister(challengeAttestedMetric)

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
func (m *MetricService) IncVerifiedChallenges() {
	m.MetricsMap[MetricVerifiedChallenges].(prometheus.Counter).Inc()
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

func (m *MetricService) IncChallengeAttested() {
	m.MetricsMap[MetricChallengeAttested].(prometheus.Counter).Inc()
}

// Broadcaster
func (m *MetricService) SetBroadcasterDuration(duration time.Duration) {
	m.MetricsMap[MetricBroadcasterDuration].(prometheus.Histogram).Observe(duration.Seconds())
}

// Collator
func (m *MetricService) SetCollatorDuration(duration time.Duration) {
	m.MetricsMap[MetricCollatorDuration].(prometheus.Histogram).Observe(duration.Seconds())
}

// Submitter
func (m *MetricService) SetSubmitterDuration(duration time.Duration) {
	m.MetricsMap[MetricSubmitterDuration].(prometheus.Histogram).Observe(duration.Seconds())
}
