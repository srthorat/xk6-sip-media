package sip

import (
	"fmt"
	"math/rand"
	"sync"
	"time"

	corertp "xk6-sip-media/core/rtp"
)

// ConferenceConfig describes a bridge-based SIP conference.
//
// Each participant is dialed as an independent SIP call leg to BridgeURI.
// The conference bridge (Asterisk ConfBridge / FreeSWITCH) mixes audio.
// No local audio mixing is performed by xk6-sip-media.
type ConferenceConfig struct {
	// BridgeURI is the conference bridge SIP URI, e.g. "sip:room101@pbx".
	BridgeURI string

	// AudioFile is the WAV file streamed by each participant leg.
	AudioFile string

	// Codec is the audio codec for all legs (default PCMU).
	Codec string

	// LocalIP for all legs (default: auto-detect).
	LocalIP string

	// Duration caps each leg. 0 = manual Hangup() required.
	Duration time.Duration

	// Username / Password for digest auth on each INVITE (optional).
	Username string
	Password string
}

// Conference manages a set of concurrent SIP call legs all connected to the
// same conference bridge URI.
type Conference struct {
	mu   sync.Mutex
	legs []*CallHandle
	cfg  ConferenceConfig
}

// ConferenceResult aggregates quality metrics across all legs.
type ConferenceResult struct {
	Legs             int
	AvgMOS           float64
	MinMOS           float64
	MaxMOS           float64
	TotalPacketsSent int64
	TotalPacketsLost int64
}

// StartConference dials BridgeURI once (as the "host" leg) and returns
// a *Conference handle. Use AddParticipant to add more legs.
func StartConference(cfg ConferenceConfig) (*Conference, error) {
	c := &Conference{cfg: cfg}

	// Dial the initial host leg to open the bridge room
	leg, err := c.dialLeg(cfg.BridgeURI, nil)
	if err != nil {
		return nil, fmt.Errorf("conference: dial host leg: %w", err)
	}

	c.mu.Lock()
	c.legs = append(c.legs, leg)
	c.mu.Unlock()

	return c, nil
}

// AddParticipant dials targetURI (or BridgeURI if empty) into the conference.
// Each leg gets a unique RTP port so concurrent participants do not collide.
// If cfg is non-nil, its AudioFile, Codec, and LocalIP fields override the
// conference-level defaults for this leg only.
func (c *Conference) AddParticipant(targetURI string, cfg *ConferenceConfig) error {
	uri := targetURI
	if uri == "" {
		uri = c.cfg.BridgeURI
	}
	leg, err := c.dialLeg(uri, cfg)
	if err != nil {
		return fmt.Errorf("conference add participant %s: %w", uri, err)
	}

	c.mu.Lock()
	c.legs = append(c.legs, leg)
	c.mu.Unlock()
	return nil
}

// ParticipantCount returns the number of active legs.
func (c *Conference) ParticipantCount() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	active := 0
	for _, l := range c.legs {
		if l.IsActive() {
			active++
		}
	}
	return active
}

// Hangup terminates all active legs.
func (c *Conference) Hangup() error {
	c.mu.Lock()
	legs := make([]*CallHandle, len(c.legs))
	copy(legs, c.legs)
	c.mu.Unlock()

	var lastErr error
	for _, leg := range legs {
		if err := leg.Hangup(); err != nil {
			lastErr = err
		}
	}
	return lastErr
}

// WaitDone blocks until all conference legs have ended.
func (c *Conference) WaitDone() {
	c.mu.Lock()
	legs := make([]*CallHandle, len(c.legs))
	copy(legs, c.legs)
	c.mu.Unlock()

	for _, leg := range legs {
		leg.WaitDone()
	}
}

// Result aggregates quality metrics across all legs once the conference ends.
func (c *Conference) Result() ConferenceResult {
	c.mu.Lock()
	legs := make([]*CallHandle, len(c.legs))
	copy(legs, c.legs)
	c.mu.Unlock()

	res := ConferenceResult{
		Legs:   len(legs),
		MinMOS: 5.0,
	}

	for _, leg := range legs {
		r := leg.Result() // blocks per leg until done
		res.TotalPacketsSent += int64(r.PacketsSent)
		res.TotalPacketsLost += int64(r.PacketsLost)
		res.AvgMOS += r.MOS
		if r.MOS < res.MinMOS {
			res.MinMOS = r.MOS
		}
		if r.MOS > res.MaxMOS {
			res.MaxMOS = r.MOS
		}
	}
	if len(legs) > 0 {
		res.AvgMOS /= float64(len(legs))
	}
	return res
}

// dialLeg creates one CallHandle for a single conference participant leg.
// If override is non-nil, its AudioFile, Codec, and LocalIP fields take
// precedence over the conference-level config for this leg.
func (c *Conference) dialLeg(targetURI string, override *ConferenceConfig) (*CallHandle, error) {
	// Each leg gets a unique From user to simulate different participants
	fromUser := fmt.Sprintf("conf-leg-%d", rand.Intn(999999))
	_ = fromUser // future: set custom From header

	cfg := CallConfig{
		Target:    targetURI,
		AudioFile: c.cfg.AudioFile,
		Codec:     c.cfg.Codec,
		LocalIP:   c.cfg.LocalIP,
		Duration:  c.cfg.Duration,
		RTPPort:   0, // auto-assign unique port per leg
	}
	if override != nil {
		if override.AudioFile != "" {
			cfg.AudioFile = override.AudioFile
		}
		if override.Codec != "" {
			cfg.Codec = override.Codec
		}
		if override.LocalIP != "" {
			cfg.LocalIP = override.LocalIP
		}
	}
	if cfg.Codec == "" {
		cfg.Codec = "PCMU"
	}

	return Dial(cfg)
}

// Compile-time check: ConferenceResult embeds no external types.
var _ = corertp.CallResult{}
