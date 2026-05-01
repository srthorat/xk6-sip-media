package sip

import (
	"context"
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
	cfg    HealthCheckConfig
	mu     sync.RWMutex
	stats  HealthCheckStats
	cancel context.CancelFunc
	done   chan struct{}
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
	hc := &HealthChecker{
		cfg:    cfg,
		cancel: cancel,
		done:   make(chan struct{}),
		stats:  HealthCheckStats{Healthy: true},
	}
	go hc.run(ctx)
	return hc
}

// run is the background ping loop. It fires once immediately, then on every tick.
func (hc *HealthChecker) run(ctx context.Context) {
	defer close(hc.done)

	hc.ping() // immediate first check

	ticker := time.NewTicker(hc.cfg.Interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			hc.ping()
		}
	}
}

// ping sends one OPTIONS request and updates stats.
func (hc *HealthChecker) ping() {
	res, err := SendOptions(OptionsConfig{
		Target:    hc.cfg.Target,
		LocalIP:   hc.cfg.LocalIP,
		Transport: hc.cfg.Transport,
		Timeout:   hc.cfg.Timeout,
	})

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

// Stop cancels the background loop and waits for it to exit.
func (hc *HealthChecker) Stop() {
	hc.cancel()
	<-hc.done
}
