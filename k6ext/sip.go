package k6ext

import (
	"fmt"
	"time"

	"go.k6.io/k6/metrics"

	corertp "xk6-sip-media/core/rtp"
	sipcall "xk6-sip-media/sip"
)

// sipMetrics is populated once per VU during the first Call() or Dial() invocation.
var sipMetrics *SIPMetrics

// ensureMetrics lazily registers all custom k6 metrics on first use.
func (m *SIPModule) ensureMetrics() {
	if sipMetrics != nil {
		return
	}
	reg := m.vu.InitEnv().Registry
	var err error
	sipMetrics, err = registerMetrics(reg)
	if err != nil {
		panic(fmt.Sprintf("xk6-sip-media: failed to register metrics: %v", err))
	}
}

// ── sip.call() ──────────────────────────────────────────────────────────────

// Call implements the blocking sip.call() JavaScript API (original behaviour).
func (m *SIPModule) Call(opts map[string]interface{}) map[string]interface{} {
	m.ensureMetrics()

	cfg := parseCfg(opts)
	start := time.Now()
	result, err := sipcall.StartCall(cfg)
	elapsed := time.Since(start)

	return m.emitAndReturn(result, err, elapsed)
}

// ── sip.dial() ──────────────────────────────────────────────────────────────

// Dial establishes a SIP call and returns a live call handle immediately.
// Use the handle for hold/transfer/conference operations.
//
//	const call = sip.dial({ target: "sip:bob@pbx", audio: { file: "sample.wav" } });
//	call.hold();
//	call.blindTransfer("sip:charlie@pbx");
//	call.waitDone();
//	const r = call.result();
func (m *SIPModule) Dial(opts map[string]interface{}) *K6CallHandle {
	m.ensureMetrics()

	cfg := parseCfg(opts)
	handle, err := sipcall.Dial(cfg)
	if err != nil {
		// Return a "dead" handle that immediately returns an error result
		panic(fmt.Sprintf("sip.dial: %v", err))
	}
	return &K6CallHandle{handle: handle}
}

// ── sip.register() ──────────────────────────────────────────────────────────

// Register performs a SIP REGISTER and returns a registration handle.
//
//	const reg = sip.register({
//	    registrar: "sip:pbx.example.com",
//	    aor:       "sip:alice@pbx.example.com",
//	    username:  "alice",
//	    password:  "secret",
//	});
//	// ... make calls ...
//	reg.unregister();
func (m *SIPModule) Register(opts map[string]interface{}) *K6Registration {
	m.ensureMetrics()

	cfg := sipcall.RegisterConfig{}
	if v, ok := opts["registrar"].(string); ok {
		cfg.Registrar = v
	}
	if v, ok := opts["aor"].(string); ok {
		cfg.AOR = v
	}
	if v, ok := opts["username"].(string); ok {
		cfg.Username = v
	}
	if v, ok := opts["password"].(string); ok {
		cfg.Password = v
	}
	if v, ok := opts["expires"].(int64); ok {
		cfg.Expires = int(v)
	}
	if v, ok := opts["localIP"].(string); ok {
		cfg.LocalIP = v
	}
	if v, ok := opts["transport"].(string); ok && v != "" {
		cfg.Transport = v
	}
	if tls, ok := opts["tls"].(map[string]interface{}); ok {
		tlsCfg := &sipcall.TLSConfig{}
		if v, ok := tls["cert"].(string); ok {
			tlsCfg.CertFile = v
		}
		if v, ok := tls["key"].(string); ok {
			tlsCfg.KeyFile = v
		}
		if v, ok := tls["ca"].(string); ok {
			tlsCfg.CAFile = v
		}
		if v, ok := tls["skipVerify"].(bool); ok {
			tlsCfg.InsecureSkipVerify = v
		}
		if v, ok := tls["serverName"].(string); ok {
			tlsCfg.ServerName = v
		}
		cfg.TLSConfig = tlsCfg
	} else if cfg.Transport == sipcall.TransportTLS {
		cfg.TLSConfig = &sipcall.TLSConfig{InsecureSkipVerify: true}
	}

	reg, err := sipcall.Register(cfg)
	if err != nil {
		panic(fmt.Sprintf("sip.register: %v", err))
	}

	now := time.Now()
	tagsAndMeta := m.vu.State().Tags.GetCurrentValues()
	m.vu.State().Samples <- metrics.ConnectedSamples{
		Samples: []metrics.Sample{makeSample(sipMetrics.RegisterSuccess, tagsAndMeta.Tags, 1)},
		Tags:    tagsAndMeta.Tags,
		Time:    now,
	}

	return &K6Registration{reg: reg}
}

// ── sip.options() ───────────────────────────────────────────────────────────

// Options sends a SIP OPTIONS keep-alive or health-check to a target.
//
//	const res = sip.options({ target: "sip:pbx.example.com" });
//	check(res, { 'options ok': (r) => r.success });
func (m *SIPModule) Options(opts map[string]interface{}) map[string]interface{} {
	m.ensureMetrics()

	cfg := sipcall.OptionsConfig{}
	if v, ok := opts["target"].(string); ok {
		cfg.Target = v
	}
	if v, ok := opts["localIP"].(string); ok {
		cfg.LocalIP = v
	}
	if v, ok := opts["transport"].(string); ok && v != "" {
		cfg.Transport = v
	}
	if v, ok := opts["timeout"].(string); ok {
		d, err := time.ParseDuration(v)
		if err == nil {
			cfg.Timeout = d
		}
	}
	// Simplified TLS logic for ping
	if tls, ok := opts["tls"].(map[string]interface{}); ok {
		tlsCfg := &sipcall.TLSConfig{}
		if v, ok := tls["skipVerify"].(bool); ok {
			tlsCfg.InsecureSkipVerify = v
		}
		cfg.TLSConfig = tlsCfg
	} else if cfg.Transport == sipcall.TransportTLS {
		cfg.TLSConfig = &sipcall.TLSConfig{InsecureSkipVerify: true}
	}

	now := time.Now()
	res, err := sipcall.SendOptions(cfg)
	tagsAndMeta := m.vu.State().Tags.GetCurrentValues()

	if err != nil {
		m.vu.State().Samples <- metrics.ConnectedSamples{
			Samples: []metrics.Sample{makeSample(sipMetrics.OptionsFailure, tagsAndMeta.Tags, 1)},
			Tags:    tagsAndMeta.Tags,
			Time:    now,
		}
		return map[string]interface{}{"success": false, "error": err.Error()}
	}

	m.vu.State().Samples <- metrics.ConnectedSamples{
		Samples: []metrics.Sample{
			makeSample(sipMetrics.OptionsSuccess, tagsAndMeta.Tags, 1),
			makeSample(sipMetrics.OptionsRTT, tagsAndMeta.Tags, float64(res.RTT.Milliseconds())),
		},
		Tags: tagsAndMeta.Tags,
		Time: now,
	}

	return map[string]interface{}{
		"success": true,
		"status":  res.StatusCode,
		"rtt_ms":  res.RTT.Milliseconds(),
	}
}

// ── sip.conference() ────────────────────────────────────────────────────────

// Conference creates a bridge-based SIP conference.
//
//	const conf = sip.conference({
//	    bridgeURI: "sip:room101@pbx",
//	    audio:     { file: "./sample.wav" },
//	    duration:  "30s",
//	});
//	conf.addParticipant("sip:bob@pbx", { audio: { file: "./sample.wav" } });
//	conf.waitDone();
//	const r = conf.result();
func (m *SIPModule) Conference(opts map[string]interface{}) *K6Conference {
	m.ensureMetrics()

	cfg := sipcall.ConferenceConfig{}
	if v, ok := opts["bridgeURI"].(string); ok {
		cfg.BridgeURI = v
	}
	if audio, ok := opts["audio"].(map[string]interface{}); ok {
		if f, ok := audio["file"].(string); ok {
			cfg.AudioFile = f
		}
		if c, ok := audio["codec"].(string); ok {
			cfg.Codec = c
		}
	}
	if v, ok := opts["duration"].(string); ok {
		d, err := time.ParseDuration(v)
		if err == nil {
			cfg.Duration = d
		}
	}
	if v, ok := opts["localIP"].(string); ok {
		cfg.LocalIP = v
	}

	conf, err := sipcall.StartConference(cfg)
	if err != nil {
		panic(fmt.Sprintf("sip.conference: %v", err))
	}
	return &K6Conference{conf: conf}
}

// ── helpers ─────────────────────────────────────────────────────────────────

// parseCfg extracts a CallConfig from a JavaScript options map.
func parseCfg(opts map[string]interface{}) sipcall.CallConfig {
	cfg := sipcall.CallConfig{
		LocalIP: "0.0.0.0",
		Codec:   "PCMU",
	}
	if v, ok := opts["target"].(string); ok {
		cfg.Target = v
	}
	if v, ok := opts["duration"].(string); ok {
		if d, err := time.ParseDuration(v); err == nil {
			cfg.Duration = d
		}
	}
	if audio, ok := opts["audio"].(map[string]interface{}); ok {
		if f, ok := audio["file"].(string); ok {
			cfg.AudioFile = f
		}
		if c, ok := audio["codec"].(string); ok {
			cfg.Codec = c
		}
	}
	if dtmf, ok := opts["dtmf"].([]interface{}); ok {
		for _, d := range dtmf {
			if s, ok := d.(string); ok {
				cfg.DTMFSequence = append(cfg.DTMFSequence, s)
			}
		}
	}
	if v, ok := opts["localIP"].(string); ok && v != "" {
		cfg.LocalIP = v
	}
	if v, ok := opts["rtpPort"].(int64); ok {
		cfg.RTPPort = int(v)
	}
	if v, ok := opts["pesq"].(bool); ok {
		cfg.EnablePESQ = v
	}
	if v, ok := opts["username"].(string); ok {
		cfg.Username = v
	}
	if v, ok := opts["password"].(string); ok {
		cfg.Password = v
	}

	// ── Transport ─────────────────────────────────────────────────────────
	if v, ok := opts["transport"].(string); ok && v != "" {
		cfg.Transport = v
	}
	if v, ok := opts["sipPort"].(int64); ok && v > 0 {
		cfg.SIPPort = int(v)
	}

	// ── TLS options ───────────────────────────────────────────────────────
	if tls, ok := opts["tls"].(map[string]interface{}); ok {
		tlsCfg := &sipcall.TLSConfig{}
		if v, ok := tls["cert"].(string); ok {
			tlsCfg.CertFile = v
		}
		if v, ok := tls["key"].(string); ok {
			tlsCfg.KeyFile = v
		}
		if v, ok := tls["ca"].(string); ok {
			tlsCfg.CAFile = v
		}
		if v, ok := tls["skipVerify"].(bool); ok {
			tlsCfg.InsecureSkipVerify = v
		}
		if v, ok := tls["serverName"].(string); ok {
			tlsCfg.ServerName = v
		}
		cfg.TLSConfig = tlsCfg
	} else if cfg.Transport == sipcall.TransportTLS {
		// TLS without explicit config → skip-verify (sensible load-test default)
		cfg.TLSConfig = &sipcall.TLSConfig{InsecureSkipVerify: true}
	}

	// ── Media options ─────────────────────────────────────────────────────
	if v, ok := opts["audioMode"].(string); ok {
		cfg.AudioMode = v
	}
	if v, ok := opts["pcapFile"].(string); ok {
		cfg.PCAPFile = v
	}
	if v, ok := opts["ipv6"].(bool); ok {
		cfg.IPv6 = v
	}

	// ── Security / Quality extensions ────────────────────────────────────
	if v, ok := opts["srtp"].(bool); ok {
		cfg.SRTP = v
	}
	if v, ok := opts["rtcp"].(bool); ok {
		cfg.RTCP = v
	}
	if v, ok := opts["earlyMedia"].(bool); ok {
		cfg.EarlyMedia = v
	}
	if v, ok := opts["cancelAfter"].(string); ok {
		if d, err := time.ParseDuration(v); err == nil {
			cfg.CancelAfter = d
		}
	}

	// ── Custom SIP headers ────────────────────────────────────────────────
	if headers, ok := opts["headers"].(map[string]interface{}); ok {
		cfg.CustomHeaders = make(map[string]string, len(headers))
		for k, v := range headers {
			if s, ok := v.(string); ok {
				cfg.CustomHeaders[k] = s
			}
		}
	}

	return cfg
}

// emitAndReturn emits k6 metrics and returns a JS-compatible result map.
func (m *SIPModule) emitAndReturn(
	result corertp.CallResult,
	err error,
	elapsed time.Duration,
) map[string]interface{} {
	now := time.Now()
	tagsAndMeta := m.vu.State().Tags.GetCurrentValues()
	tagSet := tagsAndMeta.Tags

	if err != nil {
		m.vu.State().Samples <- metrics.ConnectedSamples{
			Samples: []metrics.Sample{makeSample(sipMetrics.CallFailure, tagSet, 1)},
			Tags:    tagSet,
			Time:    now,
		}
		return map[string]interface{}{"success": false, "error": err.Error()}
	}

	samples := []metrics.Sample{
		makeSample(sipMetrics.CallSuccess, tagSet, 1),
		makeSample(sipMetrics.CallDuration, tagSet, float64(elapsed.Milliseconds())),
		makeSample(sipMetrics.RTPPacketsSent, tagSet, float64(result.PacketsSent)),
		makeSample(sipMetrics.RTPPacketsReceived, tagSet, float64(result.PacketsReceived)),
		makeSample(sipMetrics.RTPPacketsLost, tagSet, float64(result.PacketsLost)),
		makeSample(sipMetrics.RTPJitter, tagSet, result.Jitter),
		makeSample(sipMetrics.MOSScore, tagSet, result.MOS),
	}
	if result.TransferOK {
		samples = append(samples, makeSample(sipMetrics.TransferSuccess, tagSet, 1))
	}

	m.vu.State().Samples <- metrics.ConnectedSamples{
		Samples: samples,
		Tags:    tagSet,
		Time:    now,
	}

	return map[string]interface{}{
		"success":     true,
		"sent":        result.PacketsSent,
		"received":    result.PacketsReceived,
		"lost":        result.PacketsLost,
		"jitter":      result.Jitter,
		"mos":         result.MOS,
		"pesq_mos":    result.PESQScore,
		"ivr_ok":      result.IVRValid,
		"transfer_ok": result.TransferOK,
	}
}
