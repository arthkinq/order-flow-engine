// Package metrics manages Prometheus metrics registration and HTTP metrics exporter.
package metrics

import (
	"errors"
	"log"
	"net/http"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

var (
	// OrdersCreatedTotal tracks total number of orders created via gRPC API.
	OrdersCreatedTotal = prometheus.NewCounter(
		prometheus.CounterOpts{
			Name: "orders_created_total",
			Help: "Total number of orders created via API",
		},
	)

	// OrdersProcessedTotal tracks total number of orders processed by workers tagged by status.
	OrdersProcessedTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "orders_processed_total",
			Help: "Total number of orders processed by workers tagged by status",
		},
		[]string{"status"}, // "completed" | "failed"
	)

	// OrderProcessingDuration tracks processing latency in seconds.
	OrderProcessingDuration = prometheus.NewHistogram(
		prometheus.HistogramOpts{
			Name:    "order_processing_duration_seconds",
			Help:    "Duration of order processing pipeline in seconds",
			Buckets: prometheus.DefBuckets,
		},
	)
)

func init() {
	prometheus.MustRegister(OrdersCreatedTotal)
	prometheus.MustRegister(OrdersProcessedTotal)
	prometheus.MustRegister(OrderProcessingDuration)
}

// StartServer starts an HTTP server in background exposing /metrics endpoint.
func StartServer(addr string) {
	mux := http.NewServeMux()
	mux.Handle("/metrics", promhttp.Handler())

	go func() {
		log.Printf("Prometheus metrics server listening on http://%s/metrics", addr)
		if err := http.ListenAndServe(addr, mux); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Printf("Metrics server error: %v", err)
		}
	}()
}
