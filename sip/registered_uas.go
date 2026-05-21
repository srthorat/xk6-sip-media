// Package sip — RegisteredUAS: a SIP UA that registers with a proxy and
// answers inbound calls.  Unlike the plain UAS Server (server.go) which
// binds a raw port and expects direct INVITE delivery, RegisteredUAS first
// sends REGISTER so that the proxy/PBX can route calls to it by AOR.
//
// Flow per VU:
//  1. Create UA (one per RegisteredUAS instance)
//  2. Register with proxy — Contact includes the listen port
//  3. Start server.ListenAndServe on the same UA
//  4. handleInvite: answer, play audio, wait for BYE or duration timeout
//  5. Stop: unregister, cancel context, close UA
package sip

import (
	"context"
	"fmt"
	"math/rand"
	"net"
	"strconv"
	"sync"
	"time"

	"github.com/emiago/sipgo"
	sipmsg "github.com/emiago/sipgo/sip"

	"github.com/srthorat/xk6-sip-media/core/audio"
	"github.com/srthorat/xk6-sip-media/core/codec"
	corertp "github.com/srthorat/xk6-sip-media/core/rtp"
)

// RegisteredUASConfig configures a SIP UA that registers and handles inbound calls.
type RegisteredUASConfig struct {
	// ── Registration ──────────────────────────────────────────────────────

	// Registrar is the SIP registrar URI, e.g. "sip:pbx.example.com".
	Registrar string
	// AOR is the Address of Record, e.g. "sip:alice@pbx.example.com".
	AOR string
	// Username and Password for Digest authentication (RFC 2617).
	Username string
	Password string
	// Expires is the registration lifetime in seconds (default 3600).
	// Auto-refresh fires at Expires/2.
	Expires int

	// ── Transport / Listen ────────────────────────────────────────────────

	// ListenAddr is the address the UAS listens on, e.g. "0.0.0.0:5060".
	// The port is advertised in the REGISTER Contact header so the proxy
	// can route inbound calls here.
	ListenAddr string
	// Transport selects "udp" (default), "tcp", or "tls".
	Transport string
	// LocalIP is the IP advertised in Contact and SDP.
	// Auto-detected when empty.
	LocalIP string
	// TLSConfig holds TLS parameters for Transport == "tls".
	TLSConfig *TLSConfig

	// ── Media ─────────────────────────────────────────────────────────────

	// AudioFile is streamed to the caller when a call is answered.
	AudioFile string
	// Codec for answered calls (default "PCMU").
	Codec string
	// CallDuration caps each answered call; 0 = hang up only on BYE.
	CallDuration time.Duration
	// MaxConcurrent limits simultaneously active legs; 0 = unlimited.
	MaxConcurrent int
	// EchoMode reflects incoming RTP back (no AudioFile needed).
	EchoMode bool
}

// callStop guards a stop channel with sync.Once so concurrent closers
// (BYE handler vs duration timeout) never panic on double-close.
type callStop struct {
	once sync.Once
	ch   chan struct{}
}

func newCallStop() *callStop { return &callStop{ch: make(chan struct{})} }
func (s *callStop) signal()  { s.once.Do(func() { close(s.ch) }) }

// RegisteredUAS is a SIP UA that has registered and is ready to accept calls.
type RegisteredUAS struct {
	cfg    RegisteredUASConfig
	ua     *sipgo.UserAgent
	client *sipgo.Client
	server *sipgo.Server

	ctx    context.Context
	cancel context.CancelFunc

	mu      sync.Mutex
	active  int
	stopMap map[string]*callStop // call-ID → once-guarded stop signal
	results []corertp.CallResult
}

// NewRegisteredUAS creates the UA, registers with the proxy, and starts
// listening for inbound INVITEs.  Calls are handled concurrently in the
// background.  Call Stop() to unregister and shut down.
func NewRegisteredUAS(cfg RegisteredUASConfig) (*RegisteredUAS, error) {
	if cfg.Codec == "" {
		cfg.Codec = "PCMU"
	}
	if cfg.Expires == 0 {
		cfg.Expires = 3600
	}
	if cfg.LocalIP == "" {
		cfg.LocalIP = resolveLocalIP("")
	}
	if cfg.ListenAddr == "" {
		cfg.ListenAddr = "0.0.0.0:5060"
	}

	// ── Build UA ──────────────────────────────────────────────────────────
	uaOpts := []sipgo.UserAgentOption{
		sipgo.WithUserAgent("xk6-sip-media-reguas/1.0"),
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
		return nil, fmt.Errorf("reguas: create UA: %w", err)
	}

	// ── Server (inbound INVITE handler) ───────────────────────────────────
	srv, err := sipgo.NewServer(ua)
	if err != nil {
		_ = ua.Close()
		return nil, fmt.Errorf("reguas: create server: %w", err)
	}

	// ── Client (outbound REGISTER) ────────────────────────────────────────
	client, err := sipgo.NewClient(ua, sipgo.WithClientHostname(cfg.LocalIP))
	if err != nil {
		_ = ua.Close()
		return nil, fmt.Errorf("reguas: create client: %w", err)
	}

	ctx, cancel := context.WithCancel(context.Background())

	r := &RegisteredUAS{
		cfg:     cfg,
		ua:      ua,
		client:  client,
		server:  srv,
		ctx:     ctx,
		cancel:  cancel,
		stopMap: make(map[string]*callStop),
	}

	srv.OnInvite(r.handleInvite)
	srv.OnBye(r.handleBye)
	srv.OnOptions(handleOptions)

	// Start listening and capture early bind errors.
	listenErrCh := make(chan error, 1)
	go func() {
		transport := cfg.Transport
		if transport == "" {
			transport = TransportUDP
		}
		if err := srv.ListenAndServe(ctx, transport, cfg.ListenAddr); err != nil && ctx.Err() == nil {
			listenErrCh <- err
		}
		close(listenErrCh)
	}()

	// Allow transport to bind before sending REGISTER.
	time.Sleep(150 * time.Millisecond)

	// Fail early if the transport failed to bind (e.g. address already in use).
	select {
	case err := <-listenErrCh:
		if err != nil {
			cancel()
			_ = ua.Close()
			return nil, fmt.Errorf("reguas: listen on %s: %w", cfg.ListenAddr, err)
		}
	default:
	}

	// ── Register ──────────────────────────────────────────────────────────
	if err := r.sendRegister(cfg.Expires); err != nil {
		cancel()
		_ = ua.Close()
		return nil, fmt.Errorf("reguas: initial REGISTER: %w", err)
	}

	// Auto-refresh at half-expiry
	go r.autoRefresh()

	return r, nil
}

// Results returns collected call-quality results (thread-safe).
func (r *RegisteredUAS) Results() []corertp.CallResult {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([]corertp.CallResult, len(r.results))
	copy(out, r.results)
	return out
}

// Unregister sends REGISTER with Expires:0 to remove the binding.
func (r *RegisteredUAS) Unregister() error {
	return r.sendRegister(0)
}

// Stop unregisters and shuts down the UA (cancel context + close UA).
func (r *RegisteredUAS) Stop() {
	_ = r.Unregister()
	r.cancel()
	_ = r.ua.Close()
}

// ── Internal helpers ────────────────────────────────────────────────────────

// sendRegister sends a REGISTER request with the given Expires value.
// expires == 0 means un-register.
func (r *RegisteredUAS) sendRegister(expires int) error {
	var registrarURI sipmsg.Uri
	if err := sipmsg.ParseUri(r.cfg.Registrar, &registrarURI); err != nil {
		return fmt.Errorf("reguas: parse registrar %q: %w", r.cfg.Registrar, err)
	}

	var aorURI sipmsg.Uri
	if err := sipmsg.ParseUri(r.cfg.AOR, &aorURI); err != nil {
		return fmt.Errorf("reguas: parse AOR %q: %w", r.cfg.AOR, err)
	}

	req := sipmsg.NewRequest(sipmsg.REGISTER, registrarURI)

	to := sipmsg.ToHeader{
		Address: sipmsg.Uri{
			Scheme:    aorURI.Scheme,
			User:      aorURI.User,
			Host:      aorURI.Host,
			Port:      aorURI.Port,
			UriParams: sipmsg.NewParams(),
			Headers:   sipmsg.NewParams(),
		},
		Params: sipmsg.NewParams(),
	}
	from := sipmsg.FromHeader{
		Address: sipmsg.Uri{
			Scheme:    aorURI.Scheme,
			User:      aorURI.User,
			Host:      aorURI.Host,
			Port:      aorURI.Port,
			UriParams: sipmsg.NewParams(),
			Headers:   sipmsg.NewParams(),
		},
		Params: sipmsg.NewParams(),
	}
	from.Params.Add("tag", sipmsg.GenerateTagN(16))
	req.AppendHeader(&from)
	req.AppendHeader(&to)

	// Contact includes the listen port so the proxy routes calls here.
	listenPort := listenPortFromAddr(r.cfg.ListenAddr)
	contactUser := r.cfg.Username
	if contactUser == "" {
		contactUser = aorURI.User
	}
	contact := sipmsg.ContactHeader{
		Address: sipmsg.Uri{
			User:      contactUser,
			Host:      r.cfg.LocalIP,
			Port:      listenPort,
			UriParams: sipmsg.NewParams(),
		},
	}
	req.AppendHeader(&contact)

	exp := sipmsg.ExpiresHeader(expires)
	req.AppendHeader(&exp)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	resp, err := r.client.Do(ctx, req, sipgo.ClientRequestRegisterBuild)
	if err != nil {
		return fmt.Errorf("reguas: REGISTER send: %w", err)
	}

	if resp.StatusCode == 401 || resp.StatusCode == 407 {
		resp, err = r.client.DoDigestAuth(ctx, req, resp, sipgo.DigestAuth{
			Username: r.cfg.Username,
			Password: r.cfg.Password,
		})
		if err != nil {
			return fmt.Errorf("reguas: REGISTER digest auth: %w", err)
		}
	}

	if resp.StatusCode != 200 {
		return fmt.Errorf("reguas: REGISTER %d %s", resp.StatusCode, resp.Reason)
	}
	return nil
}

// autoRefresh re-registers at Expires/2 intervals until ctx is done.
// The interval is clamped so it never exceeds the registration lifetime.
func (r *RegisteredUAS) autoRefresh() {
	halfSec := r.cfg.Expires / 2
	if halfSec < 5 {
		halfSec = 5
	}
	capSec := r.cfg.Expires - 5
	if capSec < 1 {
		capSec = 1
	}
	if halfSec > capSec {
		halfSec = capSec
	}
	interval := time.Duration(halfSec) * time.Second
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-r.ctx.Done():
			return
		case <-ticker.C:
			_ = r.sendRegister(r.cfg.Expires)
		}
	}
}

// listenPortFromAddr parses the port from a "host:port" address string.
// Returns 5060 if parsing fails.
func listenPortFromAddr(addr string) int {
	_, portStr, err := net.SplitHostPort(addr)
	if err != nil {
		return 5060
	}
	port, err := strconv.Atoi(portStr)
	if err != nil || port <= 0 {
		return 5060
	}
	return port
}

// ── INVITE handler ──────────────────────────────────────────────────────────

func (r *RegisteredUAS) handleInvite(req *sipmsg.Request, tx sipmsg.ServerTransaction) {
	// Concurrency cap
	if r.cfg.MaxConcurrent > 0 {
		r.mu.Lock()
		if r.active >= r.cfg.MaxConcurrent {
			r.mu.Unlock()
			_ = tx.Respond(sipmsg.NewResponseFromRequest(req, 486, "Busy Here", nil))
			return
		}
		r.active++
		r.mu.Unlock()
		defer func() {
			r.mu.Lock()
			r.active--
			r.mu.Unlock()
		}()
	}

	cod, err := codec.New(r.cfg.Codec)
	if err != nil {
		_ = tx.Respond(sipmsg.NewResponseFromRequest(req, 415, "Unsupported Media Type", nil))
		return
	}
	defer func() { _ = cod.Close() }()

	// Parse caller's SDP offer
	remoteIP, remotePort, _ := ParseSDP(string(req.Body()))
	if remoteIP == "" || remotePort == 0 {
		_ = tx.Respond(sipmsg.NewResponseFromRequest(req, 400, "Bad SDP", nil))
		return
	}

	_ = tx.Respond(sipmsg.NewResponseFromRequest(req, 100, "Trying", nil))

	// Bind the RTP socket first (port 0 → OS picks an available port).
	// We must know the actual port before building the SDP answer.
	conn, err := net.ListenUDP("udp", &net.UDPAddr{IP: net.ParseIP("0.0.0.0"), Port: 0})
	if err != nil {
		_ = tx.Respond(sipmsg.NewResponseFromRequest(req, 500, "RTP Bind Failed", nil))
		return
	}
	defer conn.Close()
	rtpPort := conn.LocalAddr().(*net.UDPAddr).Port

	sdpAnswer := BuildSDP(r.cfg.LocalIP, rtpPort, cod.PayloadType())
	resp200 := sipmsg.NewResponseFromRequest(req, 200, "OK", []byte(sdpAnswer))
	resp200.AppendHeader(sipmsg.NewHeader("Content-Type", "application/sdp"))
	// Derive contact user consistently: username if set, else AOR user.
	var aorForContact sipmsg.Uri
	_ = sipmsg.ParseUri(r.cfg.AOR, &aorForContact)
	contactUser200 := r.cfg.Username
	if contactUser200 == "" {
		contactUser200 = aorForContact.User
	}
	resp200.AppendHeader(&sipmsg.ContactHeader{
		Address: sipmsg.Uri{
			User: contactUser200,
			Host: r.cfg.LocalIP,
			Port: listenPortFromAddr(r.cfg.ListenAddr),
		},
	})
	// Reject malformed requests with no Call-ID to prevent empty-key collisions.
	callID := callIDValue(req)
	if callID == "" {
		_ = tx.Respond(sipmsg.NewResponseFromRequest(req, 400, "Missing Call-ID", nil))
		return
	}

	// Register stop signal BEFORE sending 200 OK so a BYE that races in
	// immediately after ACK is never missed.
	cs := newCallStop()
	r.mu.Lock()
	r.stopMap[callID] = cs
	r.mu.Unlock()
	defer func() {
		r.mu.Lock()
		delete(r.stopMap, callID)
		r.mu.Unlock()
	}()

	if err := tx.Respond(resp200); err != nil {
		return
	}

	remoteAddr := &net.UDPAddr{IP: net.ParseIP(remoteIP), Port: remotePort}

	recvStats := &corertp.RTPStats{}
	sendStats := &corertp.SendStats{}
	sess := corertp.NewSession(conn, remoteAddr, rand.Uint32())
	recorder, _ := corertp.NewRecorder("")

	var wg sync.WaitGroup

	// Rule #4: wg.Add before goroutine
	wg.Add(1)
	go func() {
		defer wg.Done()
		if r.cfg.EchoMode {
			corertp.Echo(conn, remoteAddr, recvStats, cs.ch)
		} else {
			corertp.Receive(conn, recvStats, recorder, plcPayloadSize(cod.Name()), cs.ch, nil)
		}
	}()

	// Rule #1: timed media via MediaReactor (Stream registers a Tickable)
	if r.cfg.AudioFile != "" && !r.cfg.EchoMode {
		payloads, err := audio.LoadAudioForCodec(r.cfg.AudioFile, cod)
		if err == nil {
			// Rule #5: tsIncrement = SampleRate/1000 * 20 — never hardcode 160
			tsIncrement := uint32(cod.SampleRate() / 1000 * 20)
			looped := loopPayloads(payloads, r.cfg.CallDuration)
			corertp.Stream(sess, looped, cod.PayloadType(), tsIncrement, sendStats, cs.ch, nil)
		}
	}

	// Wait for BYE signal, duration cap, or global shutdown
	if r.cfg.CallDuration > 0 {
		select {
		case <-cs.ch:
		case <-time.After(r.cfg.CallDuration):
		case <-r.ctx.Done():
		}
	} else {
		select {
		case <-cs.ch:
		case <-r.ctx.Done():
		}
	}

	// Signal receiver to stop (idempotent via sync.Once), then wait.
	cs.signal()
	wg.Wait()

	snap := recvStats.Snapshot()
	mos := corertp.CalculateMOS(snap.PacketLossPct, snap.Jitter)

	r.mu.Lock()
	r.results = append(r.results, corertp.CallResult{
		PacketsSent:     int(sendStats.PacketsSent.Load()),
		PacketsReceived: snap.PacketsReceived,
		PacketsLost:     snap.PacketsLost,
		Jitter:          snap.Jitter,
		MOS:             mos,
	})
	r.mu.Unlock()
}

// handleBye signals the active call (matched by Call-ID) to stop RTP.
func (r *RegisteredUAS) handleBye(req *sipmsg.Request, tx sipmsg.ServerTransaction) {
	_ = tx.Respond(sipmsg.NewResponseFromRequest(req, 200, "OK", nil))

	callID := callIDValue(req)
	r.mu.Lock()
	cs, ok := r.stopMap[callID]
	r.mu.Unlock()
	if ok {
		cs.signal() // idempotent — safe if duration timeout already fired
	}
}

// callIDValue extracts the raw Call-ID value from a SIP request.
func callIDValue(req *sipmsg.Request) string {
	if cid := req.CallID(); cid != nil {
		return cid.Value()
	}
	return ""
}
