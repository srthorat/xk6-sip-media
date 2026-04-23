package rtp

import (
	pionrtp "github.com/pion/rtp"
)

func Stream(sess *Session, payloads [][]byte, pt uint8, tsIncrement uint32, stats *SendStats, stop <-chan struct{}, onDone func()) {
	player := &StreamPlayer{
		sess:        sess,
		payloads:    payloads,
		pt:          pt,
		stats:       stats,
		stop:        stop,
		idx:         0,
		onDone:      onDone,
		tsIncrement: tsIncrement,
	}
	MediaReactor.Add(player)
}

type StreamPlayer struct {
	sess        *Session
	payloads    [][]byte
	pt          uint8
	stats       *SendStats
	stop        <-chan struct{}
	idx         int
	onDone      func() // called once when Tick returns false
	tsIncrement uint32 // RTP timestamp step per frame (160=8kHz, 960=48kHz Opus)
}

// Tick executes a single 20ms frame extraction and UDP write block.
func (p *StreamPlayer) Tick() bool {
	select {
	case <-p.stop:
		if p.onDone != nil {
			p.onDone()
			p.onDone = nil // prevent double-call
		}
		return false // Kill stream
	default:
	}

	if p.idx >= len(p.payloads) {
		if p.onDone != nil {
			p.onDone()
			p.onDone = nil
		}
		return false // Exhausted payloads
	}

	payload := p.payloads[p.idx]
	p.idx++

	seq, ts := p.sess.NextSeqTS(p.tsIncrement)

	pkt := &pionrtp.Packet{
		Header: pionrtp.Header{
			Version:        2,
			PayloadType:    p.pt,
			SequenceNumber: seq,
			Timestamp:      ts,
			SSRC:           p.sess.SSRC,
		},
		Payload: payload,
	}

	raw, err := pkt.Marshal()
	if err != nil {
		return true // Skip bad, but keep stream alive
	}

	if err := p.sess.Send(raw); err == nil {
		p.stats.PacketsSent.Add(1)
		p.stats.OctetsSent.Add(int64(len(payload)))
		p.stats.BytesSent.Add(int64(len(raw)))
	}

	return true // continue streaming
}
