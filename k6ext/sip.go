package k6ext

import (
	"fmt"
	"time"

	"go.k6.io/k6/js/common"
	"go.k6.io/k6/metrics"

	corertp "xk6-sip-media/core/rtp"
	sipcall "xk6-sip-media/sip"
)

// ── sip.call() ──────────────────────────────────────────────────────────────

// Call implements the blocking sip.call() JavaScript API (original behaviour).
func (m *SIPModule) Call(opts map[string]interface{}) map[string]interface{} {
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
	cfg := parseCfg(opts)
	handle, err := sipcall.Dial(cfg)
	if err != nil {
		common.Throw(m.vu.Runtime(), fmt.Errorf("sip.dial: %w", err))
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
	if v := toInt(opts["expires"]); v > 0 {
		cfg.Expires = v
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
		common.Throw(m.vu.Runtime(), fmt.Errorf("sip.register: %w", err))
	}

	now := time.Now()
	tagsAndMeta := m.vu.State().Tags.GetCurrentValues()
	m.vu.State().Samples <- metrics.ConnectedSamples{
		Samples: []metrics.Sample{makeSample(m.metrics.RegisterSuccess, tagsAndMeta.Tags, 1)},
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
			Samples: []metrics.Sample{makeSample(m.metrics.OptionsFailure, tagsAndMeta.Tags, 1)},
			Tags:    tagsAndMeta.Tags,
			Time:    now,
		}
		return map[string]interface{}{"success": false, "error": err.Error()}
	}

	m.vu.State().Samples <- metrics.ConnectedSamples{
		Samples: []metrics.Sample{
			makeSample(m.metrics.OptionsSuccess, tagsAndMeta.Tags, 1),
			makeSample(m.metrics.OptionsRTT, tagsAndMeta.Tags, float64(res.RTT.Milliseconds())),
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
		common.Throw(m.vu.Runtime(), fmt.Errorf("sip.conference: %w", err))
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
	if v, ok := opts["aor"].(string); ok {
		cfg.AOR = v
	}
	if v, ok := opts["displayName"].(string); ok {
		cfg.DisplayName = v
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
	if v, ok := opts["dtmfInitialDelay"].(string); ok {
		if d, err := time.ParseDuration(v); err == nil {
			cfg.DTMFInitialDelay = d
		}
	}
	if v, ok := opts["dtmfInterDigitGap"].(string); ok {
		if d, err := time.ParseDuration(v); err == nil {
			cfg.DTMFInterDigitGap = d
		}
	}
	if v, ok := opts["localIP"].(string); ok && v != "" {
		cfg.LocalIP = v
	}
	if v := toInt(opts["rtpPort"]); v > 0 {
		cfg.RTPPort = v
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
	if v := toInt(opts["sipPort"]); v > 0 {
		cfg.SIPPort = v
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
			Samples: []metrics.Sample{makeSample(m.metrics.CallFailure, tagSet, 1)},
			Tags:    tagSet,
			Time:    now,
		}
		return map[string]interface{}{"success": false, "error": err.Error()}
	}

	samples := []metrics.Sample{
		makeSample(m.metrics.CallSuccess, tagSet, 1),
		makeSample(m.metrics.CallDuration, tagSet, float64(elapsed.Milliseconds())),
		makeSample(m.metrics.RTPPacketsSent, tagSet, float64(result.PacketsSent)),
		makeSample(m.metrics.RTPPacketsReceived, tagSet, float64(result.PacketsReceived)),
		makeSample(m.metrics.RTPPacketsLost, tagSet, float64(result.PacketsLost)),
		makeSample(m.metrics.RTPJitter, tagSet, result.Jitter),
		makeSample(m.metrics.MOSScore, tagSet, result.MOS),
	}
	if result.TransferOK {
		samples = append(samples, makeSample(m.metrics.TransferSuccess, tagSet, 1))
	}

	m.vu.State().Samples <- metrics.ConnectedSamples{
		Samples: samples,
		Tags:    tagSet,
		Time:    now,
	}

	return map[string]interface{}{
		"success":              true,
		"sent":                 result.PacketsSent,
		"received":             result.PacketsReceived,
		"lost":                 result.PacketsLost,
		"loss_pct":             result.PacketLossPct,
		"jitter":               result.Jitter,
		"mos":                  result.MOS,
		"silence_ratio":        result.SilenceRatio,
		"rtt_ms":               result.RTTMs,
		"rtcp_fraction_lost":   result.RTCPFractionLost,
		"rtcp_cumulative_lost": result.RTCPCumulativeLost,
		"pesq_mos":             result.PESQScore,
		"ivr_ok":               result.IVRValid,
		"transfer_ok":          result.TransferOK,
		"recv_errors":          result.RecvErrors,
		"recorder_drops":       result.RecorderDrops,
	}
}

// toInt converts a JS number value to int regardless of whether Goja emitted
// it as int64, float64, int, or int32. Returns 0 if the type is unrecognised.
func toInt(v interface{}) int {
	switch x := v.(type) {
	case int64:
		return int(x)
	case float64:
		return int(x)
	case int:
		return x
	case int32:
		return int(x)
	}
	return 0
}
