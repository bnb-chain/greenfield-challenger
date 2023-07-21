package metrics

import (
	"fmt"
	"net/http"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"

	"github.com/bnb-chain/greenfield-challenger/config"
)

const (
	// Monitor
	MetricGnfdSavedBlock = "gnfd_saved_block"
	MetricGnfdSavedEvent = "gnfd_saved_event"
	// Verifier
	MetricVerifiedChallenges = "verified_challenges"
	MetricChallengeFailed    = "challenge_failed"
	MetricChallengeSuccess   = "challenge_success"
	// Attest Monitor
	MetricChallengeAttested = "challenge_attested"
	// speed, errors
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

func (m *MetricService) SetGnfdSavedBlock(height uint64) {
	m.MetricsMap[MetricGnfdSavedBlock].(prometheus.Gauge).Set(float64(height))
}

func (m *MetricService) SetGnfdSavedEvent(challengeId uint64) {
	m.MetricsMap[MetricGnfdSavedEvent].(prometheus.Gauge).Set(float64(challengeId))
}

func (m *MetricService) IncVerifiedChallenges() {
	m.MetricsMap[MetricVerifiedChallenges].(prometheus.Counter).Inc()
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
