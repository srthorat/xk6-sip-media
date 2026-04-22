package rtp

import (
	"sync"
	"time"

	pionrtp "github.com/pion/rtp"
)

// JitterBuffer implements a dynamic de-jittering priority queue designed to mirror
// the behavior of enterprise hardware phones (Polycom/Cisco). It repairs out-of-order
// UDP datagrams and synthesizes silence for irrecoverable dropped packets.
type JitterBuffer struct {
	mu           sync.Mutex
	packets      map[uint16]*pionrtp.Packet
	nextSeq      uint16
	started      bool
	playoutDelay time.Duration
	recorder     *AudioRecorder
	// silenceSize is the byte length of one encoded 20ms payload for this codec,
	// used to synthesize PLC silence frames. 0 means skip PLC writes (e.g. Opus).
	silenceSize int

	// Playout-delay state: hold packets until playoutDelay has elapsed.
	firstPktTime time.Time
	delayExpired bool

	closeOnce sync.Once
	stop      chan struct{}
}

// NewJitterBuffer initializes an adaptive buffer.
// delay dictates how long to hold the first packet before starting continuous playout.
// silenceSize is the byte length of one encoded 20ms payload for the codec in use,
// used to synthesize PLC silence frames on packet loss (0 = skip PLC writes).
func NewJitterBuffer(recorder *AudioRecorder, delay time.Duration, silenceSize int) *JitterBuffer {
	jb := &JitterBuffer{
		packets:      make(map[uint16]*pionrtp.Packet),
		playoutDelay: delay,
		recorder:     recorder,
		silenceSize:  silenceSize,
		stop:         make(chan struct{}),
	}

	if recorder != nil {
		MediaReactor.Add(jb)
	}

	return jb
}

// Push inserts an RTP packet into the buffer.
func (jb *JitterBuffer) Push(pkt *pionrtp.Packet) {
	jb.mu.Lock()
	defer jb.mu.Unlock()

	// Initialize tracking on the very first packet
	if !jb.started {
		jb.nextSeq = pkt.SequenceNumber
		jb.started = true
		jb.firstPktTime = time.Now()
		jb.delayExpired = jb.playoutDelay == 0
	}

	// Drop only packets we've already played — they are truly late.
	// Use signed-delta logic to handle the uint16 sequence number wraparound:
	// If delta is negative (with wrap), the packet is behind the playout head.
	if jb.started {
		delta := int32(pkt.SequenceNumber) - int32(jb.nextSeq)
		if delta < -256 || (delta < 0 && delta > -32768) {
			// Already played — discard
			return
		}
	}

	// Store uniquely in the map
	jb.packets[pkt.SequenceNumber] = pkt
}

// Tick is called globally by the MediaReactor every 20ms to execute the playout cycle.
func (jb *JitterBuffer) Tick() bool {
	jb.mu.Lock()
	defer jb.mu.Unlock()

	select {
	case <-jb.stop:
		// Flush remaining packets without iterating up to 65535 gaps.
		for _, pkt := range jb.packets {
			jb.recorder.Write(pkt.Payload)
		}
		jb.packets = nil
		return false // Signal removal
	default:
	}

	if !jb.started {
		return true // keep trying
	}

	// Honour playout delay: hold packets until the configured duration has elapsed.
	if !jb.delayExpired {
		if time.Since(jb.firstPktTime) < jb.playoutDelay {
			return true
		}
		jb.delayExpired = true
	}

	// Check if the exact expected sequential packet has arrived
	if pkt, ok := jb.packets[jb.nextSeq]; ok {
		jb.recorder.Write(pkt.Payload)
		delete(jb.packets, jb.nextSeq)
	} else if jb.silenceSize > 0 {
		// Packet is missing/dropped — inject codec-appropriate PLC silence.
		// silenceSize == 0 means skip (e.g. Opus VBR where zeros are invalid).
		silenceFrame := make([]byte, jb.silenceSize)
		jb.recorder.Write(silenceFrame)
	}

	jb.nextSeq++
	return true
}

// Close gracefully terminates the JitterBuffer and flushes remaining frames.
// Safe to call multiple times — only the first call has effect.
func (jb *JitterBuffer) Close() {
	jb.closeOnce.Do(func() { close(jb.stop) })
}
