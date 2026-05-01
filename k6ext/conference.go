package k6ext

import (
	sipcall "github.com/srthorat/xk6-sip-media/sip"
)

// K6Conference wraps a *sip.Conference for the k6 JavaScript runtime.
type K6Conference struct {
	conf *sipcall.Conference
}

// AddParticipant dials targetURI (or the bridge URI if empty) as a new
// conference leg with optional per-participant audio config.
//
//	conf.addParticipant("sip:bob@pbx", { audio: { file: "./sample.wav" } })
func (c *K6Conference) AddParticipant(targetURI string, opts map[string]interface{}) error {
	var partCfg *sipcall.ConferenceConfig
	if opts != nil {
		partCfg = &sipcall.ConferenceConfig{}
		if audio, ok := opts["audio"].(map[string]interface{}); ok {
			if f, ok := audio["file"].(string); ok {
				partCfg.AudioFile = f
			}
			if codec, ok := audio["codec"].(string); ok {
				partCfg.Codec = codec
			}
		}
		if v, ok := opts["localIP"].(string); ok {
			partCfg.LocalIP = v
		}
	}
	return c.conf.AddParticipant(targetURI, partCfg)
}

// ParticipantCount returns the current number of active legs.
func (c *K6Conference) ParticipantCount() int {
	return c.conf.ParticipantCount()
}

// Hangup terminates all active conference legs.
func (c *K6Conference) Hangup() error {
	return c.conf.Hangup()
}

// WaitDone blocks until all conference legs have ended.
func (c *K6Conference) WaitDone() {
	c.conf.WaitDone()
}

// Result returns aggregated quality metrics across all legs.
func (c *K6Conference) Result() map[string]interface{} {
	r := c.conf.Result()
	return map[string]interface{}{
		"legs":         r.Legs,
		"avg_mos":      r.AvgMOS,
		"min_mos":      r.MinMOS,
		"max_mos":      r.MaxMOS,
		"packets_sent": r.TotalPacketsSent,
		"packets_lost": r.TotalPacketsLost,
	}
}
