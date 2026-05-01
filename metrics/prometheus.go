// Package metrics provides optional Prometheus metrics export.
// Enable by setting PROMETHEUS_ENABLED=1 before starting k6.
//
// Metrics are available at http://localhost:2112/metrics
// (or PROMETHEUS_PORT env var).
package metrics

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"strconv"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

var (
	once sync.Once

	MOSScore = promauto.NewHistogram(prometheus.HistogramOpts{
		Name:    "sip_mos_score",
		Help:    "MOS score (E-model) per SIP call",
		Buckets: []float64{1.0, 2.0, 3.0, 3.5, 4.0, 4.3, 5.0},
	})

	PacketLoss = promauto.NewHistogram(prometheus.HistogramOpts{
		Name:    "sip_rtp_packet_loss_percent",
		Help:    "RTP packet loss percentage per call",
		Buckets: prometheus.LinearBuckets(0, 5, 10),
	})

	JitterMs = promauto.NewHistogram(prometheus.HistogramOpts{
		Name:    "sip_rtp_jitter_ms",
		Help:    "RTP jitter (ms) per call",
		Buckets: prometheus.LinearBuckets(0, 5, 20),
	})

	CallsTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "sip_calls_total",
		Help: "Total SIP calls by outcome (success/failure)",
	}, []string{"outcome"})

	ActiveCalls = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "sip_active_calls",
		Help: "Number of currently active SIP calls",
	})
)

// Init starts the Prometheus HTTP server if PROMETHEUS_ENABLED=1.
// Safe to call multiple times — server is only started once.
func Init() {
	if os.Getenv("PROMETHEUS_ENABLED") != "1" {
		return
	}
	once.Do(func() {
		port := os.Getenv("PROMETHEUS_PORT")
		if port == "" {
			port = "2112"
		}
		// Validate port is a numeric value in the valid TCP port range.
		portNum, err := strconv.Atoi(port)
		if err != nil || portNum < 1 || portNum > 65535 {
			log.Print("[prometheus] PROMETHEUS_PORT is not a valid port number (1-65535), using 2112")
			portNum = 2112
		}
		addr := fmt.Sprintf(":%d", portNum)
		mux := http.NewServeMux()
		mux.Handle("/metrics", promhttp.Handler())
		srv := &http.Server{
			Addr:         addr,
			Handler:      mux,
			ReadTimeout:  5 * time.Second,
			WriteTimeout: 10 * time.Second,
			IdleTimeout:  60 * time.Second,
		}
		go func() {
			if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
				log.Printf("[prometheus] server error: %v", err)
			}
		}()
		log.Printf("[prometheus] metrics at http://localhost:%d/metrics", portNum)
	})
}
