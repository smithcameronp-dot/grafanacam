package ratelimit

import (
	"github.com/prometheus/client_golang/prometheus"
)

// Metrics exposes rate limiting counters.
type Metrics struct {
	RequestsTotal  *prometheus.CounterVec
	ExceededTotal  *prometheus.CounterVec
}

// NewMetrics registers and returns rate limit metrics.
func NewMetrics(registerer prometheus.Registerer) *Metrics {
	m := &Metrics{
		RequestsTotal: prometheus.NewCounterVec(prometheus.CounterOpts{
			Namespace: "grafana",
			Name:      "http_rate_limit_requests_total",
			Help:      "Total number of HTTP API requests evaluated by the rate limiter.",
		}, []string{"rule", "allowed"}),
		ExceededTotal: prometheus.NewCounterVec(prometheus.CounterOpts{
			Namespace: "grafana",
			Name:      "http_rate_limit_exceeded_total",
			Help:      "Total number of HTTP API requests rejected by the rate limiter.",
		}, []string{"rule"}),
	}

	registerer.MustRegister(m.RequestsTotal, m.ExceededTotal)
	return m
}

func (m *Metrics) Observe(rule string, allowed bool) {
	allowedLabel := "true"
	if !allowed {
		allowedLabel = "false"
		m.ExceededTotal.WithLabelValues(rule).Inc()
	}
	m.RequestsTotal.WithLabelValues(rule, allowedLabel).Inc()
}
