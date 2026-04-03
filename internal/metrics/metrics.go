package metrics

import (
	"net/http"

	"github.com/prometheus/client_golang/prometheus"
)

type Metrics struct {
	OperationsTotal       *prometheus.CounterVec
	PostgresQueryDuration *prometheus.HistogramVec
	HTTPRequestsInFlight  prometheus.Gauge
}

func New(reg *prometheus.Registry) *Metrics {
	m := &Metrics{
		OperationsTotal: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "configs_operations_total",
			Help: "Total config operations",
		}, []string{"operation", "status"}),
		PostgresQueryDuration: prometheus.NewHistogramVec(prometheus.HistogramOpts{
			Name:    "configs_postgres_query_duration_seconds",
			Help:    "Duration of PostgreSQL queries",
			Buckets: prometheus.DefBuckets,
		}, []string{"operation"}),
		HTTPRequestsInFlight: prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "configs_http_requests_in_flight",
			Help: "Number of in-flight HTTP requests",
		}),
	}

	reg.MustRegister(m.OperationsTotal, m.PostgresQueryDuration, m.HTTPRequestsInFlight)

	return m
}

func (m *Metrics) InFlightMiddleware() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			m.HTTPRequestsInFlight.Inc()
			defer m.HTTPRequestsInFlight.Dec()
			next.ServeHTTP(w, r)
		})
	}
}
