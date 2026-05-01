package sip

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// HealthCheckConfig configures a background SIP OPTIONS health-check loop.
type HealthCheckConfig struct {
	// Target is the SIP URI to ping, e.g. "sip:pbx.example.com".
	Target string

	// Interval between consecutive OPTIONS pings. Default: 5s.
	Interval time.Duration

	// Timeout for each individual OPTIONS request. Default: 2s.
	Timeout time.Duration

	// LocalIP overrides the auto-detected outbound IP.
	LocalIP string

	// Transport selects the SIP signaling transport ("udp", "tcp", "tls").
	Transport string

	// MaxFailures is the number of consecutive failures before IsHealthy()
	// returns false. Default: 3.
	MaxFailures int
}

// HealthCheckStats is a snapshot of the health-check loop state.
type HealthCheckStats struct {
	Checks      int64
	Successes   int64
	Failures    int64
	ConsecFails int
	AvgRTTms    float64
	LastRTTms   int64
	Healthy     bool
}

// HealthChecker runs a background SIP OPTIONS ping loop.
// Create one via StartHealthCheck(); call Stop() to tear it down.
type HealthChecker struct {
	cfg       HealthCheckConfig
	mu        sync.RWMutex
	stats     HealthCheckStats
	cancel    context.CancelFunc
	done      chan struct{}
	sipClient *Client // reused across all pings to avoid socket churn
}

// StartHealthCheck launches a background goroutine that sends SIP OPTIONS
// to cfg.Target at cfg.Interval and tracks health state.
func StartHealthCheck(cfg HealthCheckConfig) *HealthChecker {
	if cfg.Interval == 0 {
		cfg.Interval = 5 * time.Second
	}
	if cfg.Timeout == 0 {
		cfg.Timeout = 2 * time.Second
	}
	if cfg.MaxFailures == 0 {
		cfg.MaxFailures = 3
	}

	ctx, cancel := context.WithCancel(context.Background())

	transport := cfg.Transport
	if transport == "" {
		transport = TransportUDP
	}
	localIP := resolveLocalIPAuto(cfg.LocalIP, false)
	cl, _ := NewClientWithTransport(localIP, transport, nil) // nil on err → ping() marks failure

	hc := &HealthChecker{
		cfg:       cfg,
		cancel:    cancel,
		done:      make(chan struct{}),
		stats:     HealthCheckStats{Healthy: true},
		sipClient: cl,
	}
	go hc.run(ctx)
	return hc
}

// run is the background ping loop. It fires once immediately, then on every tick.
func (hc *HealthChecker) run(ctx context.Context) {
	defer close(hc.done)

	hc.ping(ctx) // immediate first check

	ticker := time.NewTicker(hc.cfg.Interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			hc.ping(ctx)
		}
	}
}

// ping sends one OPTIONS request using the shared client and updates stats.
// ctx is the health-checker's parent context; cancelling it aborts in-flight pings immediately.
func (hc *HealthChecker) ping(ctx context.Context) {
	var res *OptionsResult
	var err error
	if hc.sipClient != nil {
		res, err = sendOptionsWithClient(ctx, hc.sipClient, hc.cfg.Target, hc.cfg.Timeout)
	} else {
		err = fmt.Errorf("options: no client (creation failed at startup)")
	}

	hc.mu.Lock()
	defer hc.mu.Unlock()

	hc.stats.Checks++

	if err != nil || res == nil {
		hc.stats.Failures++
		hc.stats.ConsecFails++
		if hc.stats.ConsecFails >= hc.cfg.MaxFailures {
			hc.stats.Healthy = false
		}
		return
	}

	// Successful ping — reset consecutive-failure counter.
	hc.stats.Successes++
	hc.stats.ConsecFails = 0
	hc.stats.Healthy = true
	hc.stats.LastRTTms = res.RTT.Milliseconds()

	// Rolling average over successful pings only.
	hc.stats.AvgRTTms = (hc.stats.AvgRTTms*float64(hc.stats.Successes-1) +
		float64(res.RTT.Milliseconds())) / float64(hc.stats.Successes)
}

// IsHealthy returns false when ConsecFails >= MaxFailures.
func (hc *HealthChecker) IsHealthy() bool {
	hc.mu.RLock()
	defer hc.mu.RUnlock()
	return hc.stats.Healthy
}

// Stats returns a point-in-time snapshot of health-check counters.
func (hc *HealthChecker) Stats() HealthCheckStats {
	hc.mu.RLock()
	defer hc.mu.RUnlock()
	return hc.stats
}

// Stop cancels the background loop, waits for it to exit, then closes the shared client.
func (hc *HealthChecker) Stop() {
	hc.cancel()
	<-hc.done
	if hc.sipClient != nil {
		hc.sipClient.Close()
	}
}
