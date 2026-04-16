package k6ext

import (
	"go.k6.io/k6/metrics"
)

// SIPMetrics holds all custom k6 metrics exposed by xk6-sip-media.
type SIPMetrics struct {
	// Call
	CallSuccess  *metrics.Metric
	CallFailure  *metrics.Metric
	CallDuration *metrics.Metric

	// RTP
	RTPPacketsSent     *metrics.Metric
	RTPPacketsReceived *metrics.Metric
	RTPPacketsLost     *metrics.Metric
	RTPJitter          *metrics.Metric

	// Quality
	MOSScore *metrics.Metric

	// Transfer
	TransferSuccess  *metrics.Metric
	TransferDuration *metrics.Metric

	// Register
	RegisterSuccess *metrics.Metric

	// Conference
	ConferenceLegs *metrics.Metric

	// Options (Ping)
	OptionsSuccess *metrics.Metric
	OptionsFailure *metrics.Metric
	OptionsRTT     *metrics.Metric
}

// registerMetrics registers all custom metrics with the k6 metrics registry.
func registerMetrics(registry *metrics.Registry) (*SIPMetrics, error) {
	m := &SIPMetrics{}
	var err error

	if m.CallSuccess, err = registry.NewMetric("sip_call_success", metrics.Counter); err != nil {
		return nil, err
	}
	if m.CallFailure, err = registry.NewMetric("sip_call_failure", metrics.Counter); err != nil {
		return nil, err
	}
	if m.CallDuration, err = registry.NewMetric("sip_call_duration", metrics.Trend, metrics.Time); err != nil {
		return nil, err
	}
	if m.RTPPacketsSent, err = registry.NewMetric("rtp_packets_sent", metrics.Counter); err != nil {
		return nil, err
	}
	if m.RTPPacketsReceived, err = registry.NewMetric("rtp_packets_received", metrics.Counter); err != nil {
		return nil, err
	}
	if m.RTPPacketsLost, err = registry.NewMetric("rtp_packets_lost", metrics.Counter); err != nil {
		return nil, err
	}
	if m.RTPJitter, err = registry.NewMetric("rtp_jitter_ms", metrics.Trend); err != nil {
		return nil, err
	}
	if m.MOSScore, err = registry.NewMetric("mos_score", metrics.Trend); err != nil {
		return nil, err
	}
	if m.TransferSuccess, err = registry.NewMetric("sip_transfer_success", metrics.Counter); err != nil {
		return nil, err
	}
	if m.TransferDuration, err = registry.NewMetric("sip_transfer_duration_ms", metrics.Trend); err != nil {
		return nil, err
	}
	if m.RegisterSuccess, err = registry.NewMetric("sip_register_success", metrics.Counter); err != nil {
		return nil, err
	}
	if m.ConferenceLegs, err = registry.NewMetric("sip_conference_legs", metrics.Trend); err != nil {
		return nil, err
	}
	if m.OptionsSuccess, err = registry.NewMetric("sip_options_success", metrics.Counter); err != nil {
		return nil, err
	}
	if m.OptionsFailure, err = registry.NewMetric("sip_options_failure", metrics.Counter); err != nil {
		return nil, err
	}
	if m.OptionsRTT, err = registry.NewMetric("sip_options_rtt_ms", metrics.Trend, metrics.Time); err != nil {
		return nil, err
	}

	return m, nil
}

// makeSample creates a metrics.Sample with the correct k6 v0.59 struct layout.
func makeSample(m *metrics.Metric, tags *metrics.TagSet, value float64) metrics.Sample {
	return metrics.Sample{
		TimeSeries: metrics.TimeSeries{
			Metric: m,
			Tags:   tags,
		},
		Value: value,
	}
}
