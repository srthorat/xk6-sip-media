package sip

import (
	"context"
	"fmt"
	"net"
	"os"
	"sync"
	"sync/atomic"
	"time"

	"github.com/emiago/sipgo"
	sipmsg "github.com/emiago/sipgo/sip"

	"github.com/srthorat/xk6-sip-media/core/audio"
	"github.com/srthorat/xk6-sip-media/core/codec"
	corertp "github.com/srthorat/xk6-sip-media/core/rtp"
)

// CallHandle represents a live, established SIP call (post-ACK).
// All public methods are goroutine-safe and can be called from k6 scripts.
type CallHandle struct {
	// ── immutable after Dial() ─────────────────────────────────────────────
	cfg       CallConfig
	localIP   string
	rtpPort   int
	cod       codec.Codec
	sipClient *Client

	// ── SIP dialog ─────────────────────────────────────────────────────────
	dialog *sipgo.DialogClientSession

	// ── RTP ────────────────────────────────────────────────────────────────
	conn      *net.UDPConn
	sess      *corertp.Session
	sendStats *corertp.SendStats
	recvStats *corertp.RTPStats
	rtcpSess  *corertp.RTCPSession
	recorder  *corertp.AudioRecorder
	recPath   string

	// ── SRTP (optional) ────────────────────────────────────────────────────
	srtpSender   *corertp.SRTPSession
	srtpReceiver *corertp.SRTPSession

	// ── lifecycle ──────────────────────────────────────────────────────────
	stop chan struct{}  // closed to signal goroutines to stop
	wg   sync.WaitGroup // tracks sender + receiver + RTCP goroutines
	done chan struct{}  // closed after finalize() completes; safe to read Result()

	// ── state ──────────────────────────────────────────────────────────────
	mu       sync.Mutex
	active   bool // false after Hangup() or remote BYE
	onHold   bool
	result   corertp.CallResult
	sdpVer   atomic.Uint64 // RFC 4566 §5.2: session-version increments on each re-INVITE
}

// IsActive returns true if the call is still connected.
func (h *CallHandle) IsActive() bool {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.active
}

// OnHold returns true if the call is currently on hold.
func (h *CallHandle) OnHold() bool {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.onHold
}

// WaitDone blocks until the call ends (BYE sent/received, Hangup called).
func (h *CallHandle) WaitDone() {
	<-h.done
}

// Result returns the final quality metrics.
// Blocks until the call ends if called before Hangup/WaitDone.
func (h *CallHandle) Result() corertp.CallResult {
	<-h.done
	return h.result
}

// SendDTMF sends a single DTMF digit via RFC 2833 while the call is active.
func (h *CallHandle) SendDTMF(digit string) {
	h.mu.Lock()
	active := h.active
	h.mu.Unlock()
	if !active {
		return
	}
	corertp.SendDTMF(h.sess, digit)
}

// Hangup sends BYE and terminates the call.
// It is safe to call multiple times (idempotent).
func (h *CallHandle) Hangup() error {
	h.mu.Lock()
	if !h.active {
		h.mu.Unlock()
		<-h.done // wait for already-started teardown
		return nil
	}
	h.active = false
	h.mu.Unlock()

	// Signal RTP goroutines to stop
	select {
	case <-h.stop:
	default:
		close(h.stop)
	}

	// Send BYE to remote
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_ = h.dialog.Bye(ctx)

	// finalize runs in background; wait for it
	<-h.done
	return nil
}

// startFinalize waits for RTP goroutines, computes metrics, and closes resources.
// Must be called in a goroutine exactly once per CallHandle.
func (h *CallHandle) startFinalize() {
	go func() {
		h.wg.Wait() // wait for sender + receiver to exit

		// Compute quality metrics
		snap := h.recvStats.Snapshot()
		mos := corertp.CalculateMOS(snap.PacketLossPct, snap.Jitter)
		var rtcpStats corertp.RTCPStats
		if h.rtcpSess != nil {
			rtcpStats = h.rtcpSess.Stats()
		}

		var silenceRatio float64
		var recorderDrops int
		if h.recorder != nil {
			silenceRatio = audio.SilenceRatioBytes(h.recorder.Bytes())
			recorderDrops = int(h.recorder.DroppedFrames.Load())
			h.recorder.Close()
			if h.recPath != "" {
				_ = os.Remove(h.recPath)
			}
		}

		h.mu.Lock()
		h.result = corertp.CallResult{
			PacketsSent:        int(h.sendStats.PacketsSent.Load()),
			PacketsReceived:    snap.PacketsReceived,
			PacketsLost:        snap.PacketsLost,
			Jitter:             snap.Jitter,
			PacketLossPct:      snap.PacketLossPct,
			MOS:                mos,
			SilenceRatio:       silenceRatio,
			RTTMs:              rtcpStats.RTTMs,
			RTCPFractionLost:   rtcpStats.FractionLost,
			RTCPCumulativeLost: rtcpStats.CumulativeLost,
			RecvErrors:         snap.RecvErrors,
			RecorderDrops:      recorderDrops,
			BytesSent:          h.sendStats.BytesSent.Load(),
			BytesReceived:      snap.BytesReceived,
		}
		h.mu.Unlock()

		_ = h.cod.Close() // release CGO codec resources (Opus/G.729)
		h.conn.Close()
		h.sipClient.Close()

		close(h.done) // signal Result() + WaitDone() waiters
	}()
}

// dialogID returns a string uniquely identifying this call's SIP dialog.
// Used to build Replaces headers for attended transfer.
func (h *CallHandle) dialogID() (callID, toTag, fromTag string, err error) {
	req := h.dialog.InviteRequest
	resp := h.dialog.InviteResponse
	if req == nil || resp == nil {
		return "", "", "", fmt.Errorf("dialog not established")
	}

	callID = req.CallID().Value()

	if to := resp.To(); to != nil {
		toTag, _ = to.Params.Get("tag")
	}
	if from := req.From(); from != nil {
		fromTag, _ = from.Params.Get("tag")
	}
	if toTag == "" || fromTag == "" {
		return "", "", "", fmt.Errorf("dialog tags missing (call-id=%s)", callID)
	}
	return callID, toTag, fromTag, nil
}

// remoteContact returns the remote Contact URI for use as the request target in
// subsequent in-dialog requests (re-INVITE, REFER).
func (h *CallHandle) remoteContact() sipmsg.Uri {
	if h.dialog.InviteResponse != nil {
		if c := h.dialog.InviteResponse.Contact(); c != nil {
			return c.Address
		}
	}
	return h.dialog.InviteRequest.Recipient
}
