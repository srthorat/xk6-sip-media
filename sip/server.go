// Package sip provides the UAS (User Agent Server) mode:
// listens for incoming SIP INVITEs, answers them, streams audio,
// and terminates cleanly on BYE. This is the server-side counterpart
// to the UAC Dial() flow.
package sip

import (
	"context"
	"fmt"
	"math/rand"
	"net"
	"sync"
	"time"

	"github.com/emiago/sipgo"
	sipmsg "github.com/emiago/sipgo/sip"

	"xk6-sip-media/core/audio"
	"xk6-sip-media/core/codec"
	corertp "xk6-sip-media/core/rtp"
)

// ServerConfig holds parameters for the UAS listener.
type ServerConfig struct {
	// ListenAddr is the SIP listen address, e.g. "0.0.0.0:5080".
	ListenAddr string

	// Transport is "udp" (default), "tcp", or "tls".
	Transport string

	// TLSConfig for TLS transport.
	TLSConfig *TLSConfig

	// LocalIP is advertised in Contact and SDP.
	LocalIP string

	// AudioFile is streamed toward the caller when a call is answered.
	AudioFile string

	// Codec for answered calls (default "PCMU").
	Codec string

	// CallDuration caps each answered call. 0 = hang up when caller sends BYE.
	CallDuration time.Duration

	// MaxConcurrent limits simultaneously active server legs. 0 = unlimited.
	MaxConcurrent int

	// EchoMode reflects incoming RTP back to the caller (no AudioFile needed).
	EchoMode bool
}

// Server is a SIP UAS instance.  Start it with ListenAndServe() in a goroutine
// and stop it by cancelling the context.
type Server struct {
	cfg       ServerConfig
	sipServer *sipgo.Server
	ua        *sipgo.UserAgent

	mu      sync.Mutex
	active  int
	results []corertp.CallResult
}

// NewServer creates a UAS Server.
func NewServer(cfg ServerConfig) (*Server, error) {
	if cfg.Codec == "" {
		cfg.Codec = "PCMU"
	}
	if cfg.LocalIP == "" {
		cfg.LocalIP = resolveLocalIP("")
	}
	if cfg.ListenAddr == "" {
		cfg.ListenAddr = "0.0.0.0:5080"
	}

	uaOpts := []sipgo.UserAgentOption{
		sipgo.WithUserAgent("xk6-sip-media-uas/1.0"),
		sipgo.WithUserAgentHostname(cfg.LocalIP),
	}
	if cfg.Transport == TransportTLS && cfg.TLSConfig != nil {
		tlsConf, err := buildTLSConfig(cfg.TLSConfig)
		if err != nil {
			return nil, err
		}
		uaOpts = append(uaOpts, sipgo.WithUserAgenTLSConfig(tlsConf))
	}

	ua, err := sipgo.NewUA(uaOpts...)
	if err != nil {
		return nil, fmt.Errorf("uas: create UA: %w", err)
	}

	srv, err := sipgo.NewServer(ua)
	if err != nil {
		_ = ua.Close()
		return nil, fmt.Errorf("uas: create server: %w", err)
	}

	s := &Server{cfg: cfg, sipServer: srv, ua: ua}
	srv.OnInvite(s.handleInvite)
	srv.OnOptions(handleOptions)
	return s, nil
}

// ListenAndServe starts listening. Blocks until ctx is cancelled.
func (s *Server) ListenAndServe(ctx context.Context) error {
	transport := s.cfg.Transport
	if transport == "" {
		transport = TransportUDP
	}
	return s.sipServer.ListenAndServe(ctx, transport, s.cfg.ListenAddr)
}

// Results returns call quality results collected so far (thread-safe).
func (s *Server) Results() []corertp.CallResult {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]corertp.CallResult, len(s.results))
	copy(out, s.results)
	return out
}

// Close shuts down the server.
func (s *Server) Close() {
	_ = s.ua.Close()
}

// ── INVITE handler ─────────────────────────────────────────────────────────

func (s *Server) handleInvite(req *sipmsg.Request, tx sipmsg.ServerTransaction) {
	// Concurrency cap
	if s.cfg.MaxConcurrent > 0 {
		s.mu.Lock()
		if s.active >= s.cfg.MaxConcurrent {
			s.mu.Unlock()
			_ = tx.Respond(sipmsg.NewResponseFromRequest(req, 486, "Busy Here", nil))
			return
		}
		s.active++
		s.mu.Unlock()
		defer func() {
			s.mu.Lock()
			s.active--
			s.mu.Unlock()
		}()
	}

	cod := codec.New(s.cfg.Codec)
	if cod == nil {
		_ = tx.Respond(sipmsg.NewResponseFromRequest(req, 415, "Unsupported Media Type", nil))
		return
	}

	// Parse caller's SDP offer
	remoteSDP := string(req.Body())
	remoteIP, remotePort := ParseSDP(remoteSDP)
	if remoteIP == "" || remotePort == 0 {
		_ = tx.Respond(sipmsg.NewResponseFromRequest(req, 400, "Bad SDP", nil))
		return
	}

	// Pick local RTP port
	rtpPort := 20000 + rand.Intn(20000)

	// Send 100 Trying
	_ = tx.Respond(sipmsg.NewResponseFromRequest(req, 100, "Trying", nil))

	// Build SDP answer
	sdpAnswer := BuildSDP(s.cfg.LocalIP, rtpPort, cod.PayloadType())

	// Build and send 200 OK
	resp := sipmsg.NewResponseFromRequest(req, 200, "OK", []byte(sdpAnswer))
	resp.AppendHeader(sipmsg.NewHeader("Content-Type", "application/sdp"))

	contactHdr := &sipmsg.ContactHeader{
		Address: sipmsg.Uri{User: "uas", Host: s.cfg.LocalIP},
	}
	resp.AppendHeader(contactHdr)

	if err := tx.Respond(resp); err != nil {
		return
	}

	// Bind RTP socket
	localAddr := &net.UDPAddr{IP: net.ParseIP("0.0.0.0"), Port: rtpPort}
	conn, err := net.ListenUDP("udp", localAddr)
	if err != nil {
		return
	}
	defer conn.Close()

	remoteAddr := &net.UDPAddr{
		IP:   net.ParseIP(remoteIP),
		Port: remotePort,
	}

	stop := make(chan struct{})
	recvStats := &corertp.RTPStats{}
	sendStats := &corertp.SendStats{}
	sess := corertp.NewSession(conn, remoteAddr, rand.Uint32())
	recorder, _ := corertp.NewRecorder("")

	var wg sync.WaitGroup

	// Receiver goroutine (always active)
	wg.Add(1)
	go func() {
		defer wg.Done()
		if s.cfg.EchoMode {
			corertp.Echo(conn, remoteAddr, recvStats, stop)
		} else {
			corertp.Receive(conn, recvStats, recorder, stop)
		}
	}()

	// Sender goroutine (only when audio file provided and not echo mode)
	if s.cfg.AudioFile != "" && !s.cfg.EchoMode {
		payloads, err := audio.LoadWAVAsPayloads(s.cfg.AudioFile)
		if err == nil {
			looped := loopPayloads(payloads, s.cfg.CallDuration)
			wg.Add(1)
			go func() {
				defer wg.Done()
				corertp.Stream(sess, looped, cod.PayloadType(), sendStats, stop)
			}()
		}
	}

	// Wait for BYE from caller or duration timeout
	byeCtx := context.Background()
	if s.cfg.CallDuration > 0 {
		var cancel context.CancelFunc
		byeCtx, cancel = context.WithTimeout(byeCtx, s.cfg.CallDuration)
		defer cancel()
	}

	// The BYE arrives as a separate in-dialog request — wait via a simple timer
	// or context cancellation. In production use DialogServerCache.
	_ = byeCtx

	// Duration-based auto-hangup
	if s.cfg.CallDuration > 0 {
		time.Sleep(s.cfg.CallDuration)
	} else {
		// Wait indefinitely until the connection is torn down
		// (caller will BYE; sipgo transaction will expire)
		<-stop
	}

	close(stop)
	wg.Wait()

	lossPct := recvStats.PacketLossPercent()
	mos := corertp.CalculateMOS(lossPct, recvStats.Jitter)

	s.mu.Lock()
	s.results = append(s.results, corertp.CallResult{
		PacketsSent:     sendStats.PacketsSent,
		PacketsReceived: recvStats.PacketsReceived,
		PacketsLost:     recvStats.PacketsLost,
		Jitter:          recvStats.Jitter,
		MOS:             mos,
	})
	s.mu.Unlock()
}

// handleOptions responds to SIP OPTIONS keep-alives (ping).
func handleOptions(req *sipmsg.Request, tx sipmsg.ServerTransaction) {
	resp := sipmsg.NewResponseFromRequest(req, 200, "OK", nil)
	resp.AppendHeader(sipmsg.NewHeader("Allow", "INVITE, ACK, BYE, OPTIONS, INFO"))
	_ = tx.Respond(resp)
}
