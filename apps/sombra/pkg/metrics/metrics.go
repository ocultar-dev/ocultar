// Package metrics registers Sombra's Prometheus counters. apps/proxy has
// had request/latency/fail-closed/SSRF instrumentation since its first
// version (see services/refinery/pkg/proxy/metrics.go); Sombra had none,
// so fail-closed events and rehydration failures here were invisible to
// the same dashboards/alerts that monitor the proxy.
package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	RequestsTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "ocultar_sombra_requests_total",
		Help: "Total number of requests processed by the Sombra gateway.",
	}, []string{"endpoint", "status"})

	RequestLatency = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "ocultar_sombra_request_duration_seconds",
		Help:    "Latency of requests processed by the Sombra gateway.",
		Buckets: prometheus.DefBuckets,
	}, []string{"endpoint"})

	FailClosedTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "ocultar_sombra_fail_closed_total",
		Help: "Fail-closed security events by reason.",
	}, []string{"reason"})

	RehydrationFailuresTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "ocultar_sombra_rehydration_failures_total",
		Help: "Total number of response rehydration failures, by endpoint.",
	}, []string{"endpoint"})
)
