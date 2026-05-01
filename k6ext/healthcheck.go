package k6ext

import (
	"time"

	sipcall "github.com/srthorat/xk6-sip-media/sip"
)

// K6HealthChecker is the JavaScript handle returned by sip.startHealthCheck().
// All methods are safe to call from multiple k6 VUs concurrently.
type K6HealthChecker struct {
	hc *sipcall.HealthChecker
}

// StartHealthCheck launches a background SIP OPTIONS health-check loop and
// returns a handle exposing isHealthy(), stats(), and stop().
//
// Usage in a k6 script (typically in setup()):
//
//	const hc = sip.startHealthCheck({
//	  target:      'sip:pbx.example.com',
//	  interval:    '5s',
//	  timeout:     '2s',
//	  maxFailures: 3,
//	});
//
//	// … run load test …
//
//	if (!hc.isHealthy()) { console.error('SBC became unhealthy!'); }
//	hc.stop();
func (m *SIPModule) StartHealthCheck(opts map[string]interface{}) *K6HealthChecker {
	cfg := sipcall.HealthCheckConfig{}

	if v, ok := opts["target"].(string); ok {
		cfg.Target = v
	}
	if cfg.Target == "" {
		panic("sip.startHealthCheck: 'target' is required")
	}
	if v, ok := opts["localIP"].(string); ok {
		cfg.LocalIP = v
	}
	if v, ok := opts["transport"].(string); ok && v != "" {
		cfg.Transport = v
	}
	if v, ok := opts["interval"].(string); ok {
		if d, err := time.ParseDuration(v); err == nil {
			cfg.Interval = d
		}
	}
	if v, ok := opts["timeout"].(string); ok {
		if d, err := time.ParseDuration(v); err == nil {
			cfg.Timeout = d
		}
	}
	if v := toInt(opts["maxFailures"]); v > 0 {
		cfg.MaxFailures = v
	}

	return &K6HealthChecker{hc: sipcall.StartHealthCheck(cfg)}
}

// IsHealthy returns false when the target has failed MaxFailures consecutive pings.
func (h *K6HealthChecker) IsHealthy() bool {
	return h.hc.IsHealthy()
}

// Stats returns a JS object with health-check counters.
//
//	{
//	  checks:      <total pings sent>,
//	  successes:   <pings with 200 OK>,
//	  failures:    <pings that timed out / errored>,
//	  consecFails: <current consecutive failure streak>,
//	  avgRttMs:    <rolling average RTT of successful pings (ms)>,
//	  lastRttMs:   <RTT of the most recent successful ping (ms)>,
//	  healthy:     <boolean>,
//	}
func (h *K6HealthChecker) Stats() map[string]interface{} {
	s := h.hc.Stats()
	return map[string]interface{}{
		"checks":      s.Checks,
		"successes":   s.Successes,
		"failures":    s.Failures,
		"consecFails": s.ConsecFails,
		"avgRttMs":    s.AvgRTTms,
		"lastRttMs":   s.LastRTTms,
		"healthy":     s.Healthy,
	}
}

// Stop cancels the background ping loop and blocks until it exits.
// Call this in teardown() or at the end of setup().
func (h *K6HealthChecker) Stop() {
	h.hc.Stop()
}
