package sip

import (
	"context"
	"fmt"
	"math/rand"
	"net"
	"os"
	"path/filepath"
	"time"

	sipmsg "github.com/emiago/sipgo/sip"

	"xk6-sip-media/core/audio"
	"xk6-sip-media/core/codec"
	corertp "xk6-sip-media/core/rtp"
)

// Dial establishes a SIP call and returns a live *CallHandle immediately
// after the 200 OK + ACK exchange. The call remains up until Hangup() is
// called or the remote sends BYE.
//
// Supported audio modes (cfg.AudioMode):
//
//	""       stream WAV/MP3 file (default)
//	"echo"   reflect received RTP back
//	"pcap"   replay PCAP file byte-for-byte (codec-agnostic)
//	"silent" send comfort noise frames
//
// SRTP: set cfg.SRTP=true to use encrypted media (RTP/SAVP).
// RTCP: set cfg.RTCP=true to start RTCP SR/RR on port rtpPort+1.
// EarlyMedia: set cfg.EarlyMedia=true to stream audio during 183.
func Dial(cfg CallConfig) (*CallHandle, error) {
	// 1. Resolve local IP
	localIP := resolveLocalIPAuto(cfg.LocalIP, cfg.IPv6)

	// 2. Choose local RTP port
	rtpPort := cfg.RTPPort
	if rtpPort == 0 {
		rtpPort = 20000 + rand.Intn(20000)
	}

	// 3. Select codec
	var cod codec.Codec
	if cfg.AudioMode == "pcap" {
		cod = codec.New("PCMU") // SDP placeholder; actual PT comes from PCAP
	} else {
		codecName := cfg.Codec
		if codecName == "" {
			codecName = "PCMU"
		}
		cod = codec.New(codecName)
		if cod == nil {
			return nil, fmt.Errorf("dial: unknown codec %q (supported: PCMU, PCMA, G722)", codecName)
		}
	}

	// 4. Load media
	var payloads [][]byte
	var pcapPayloadType uint8

	switch cfg.AudioMode {
	case "pcap":
		if cfg.PCAPFile == "" {
			return nil, fmt.Errorf("dial: AudioMode=pcap requires PCAPFile")
		}
		frames, pt, err := audio.LoadPCAP(cfg.PCAPFile)
		if err != nil {
			return nil, fmt.Errorf("dial: load PCAP %q: %w", cfg.PCAPFile, err)
		}
		payloads = audio.PCAPPayloads(frames)
		pcapPayloadType = pt

	case "echo", "silent":
		// goroutine handles these

	default:
		if cfg.AudioFile != "" {
			p, err := audio.LoadAudioForCodec(cfg.AudioFile, cod)
			if err != nil {
				return nil, fmt.Errorf("dial: load audio: %w", err)
			}
			payloads = p
		}
	}

	// 5. Build SIP client
	transport := cfg.Transport
	if transport == "" {
		transport = TransportUDP
	}
	sipClient, err := NewClientWithTransport(localIP, transport, cfg.TLSConfig)
	if err != nil {
		return nil, fmt.Errorf("dial: SIP client (%s): %w", transport, err)
	}

	// 6. Build SDP offer (plain or SRTP)
	dir := cfg.Direction
	if dir == "" {
		dir = DirSendRecv
	}
	sdpPT := cod.PayloadType()
	if cfg.AudioMode == "pcap" && pcapPayloadType > 0 {
		sdpPT = pcapPayloadType
	}

	var sdpOffer string
	var localSRTPCfg *corertp.SRTPConfig

	if cfg.SRTP {
		sdpOffer, localSRTPCfg, err = BuildSDPWithSRTP(localIP, rtpPort, sdpPT, dir)
		if err != nil {
			sipClient.Close()
			return nil, fmt.Errorf("dial: build SRTP SDP: %w", err)
		}
	} else {
		sdpOffer = BuildSDPWithDirection(localIP, rtpPort, sdpPT, dir)
	}

	// 7. Parse + enrich target URI
	var toURI sipmsg.Uri
	if err := sipmsg.ParseUri(cfg.Target, &toURI); err != nil {
		sipClient.Close()
		return nil, fmt.Errorf("dial: parse target URI %q: %w", cfg.Target, err)
	}
	if cfg.SIPPort > 0 {
		toURI.Port = cfg.SIPPort
	} else if transport == TransportTLS && toURI.Port == 0 {
		toURI.Port = 5061
	}
	if transport != TransportUDP {
		if toURI.UriParams == nil {
			toURI.UriParams = sipmsg.NewParams()
		}
		toURI.UriParams.Add("transport", transport)
		if transport == TransportTLS {
			toURI.Scheme = "sips"
		}
	}

	// 8. Build extra SIP headers
	ctHdr := sipmsg.NewHeader("Content-Type", "application/sdp")
	extraHeaders := []sipmsg.Header{ctHdr}
	for k, v := range cfg.CustomHeaders {
		extraHeaders = append(extraHeaders, sipmsg.NewHeader(k, v))
	}

	var ctx context.Context
	var cancel context.CancelFunc

	// 8.5 CancelAfter logic
	if cfg.CancelAfter > 0 {
		ctx, cancel = context.WithTimeout(context.Background(), cfg.CancelAfter)
	} else {
		ctx, cancel = context.WithTimeout(context.Background(), 30*time.Second)
	}
	defer cancel()

	// 9. INVITE → [183 early media] → 200 OK → ACK
	var inviteResult *INVITEResult
	var earlyMedia *EarlyMedia

	if cfg.EarlyMedia {
		inviteResult, earlyMedia, err = SendINVITEWithEarlyMedia(ctx, sipClient.cache, toURI, sdpOffer, extraHeaders...)
	} else {
		inviteResult, err = SendINVITE(ctx, sipClient.cache, toURI, sdpOffer, extraHeaders...)
	}
	if err != nil {
		if cfg.CancelAfter > 0 && (ctx.Err() != nil) {
			result := corertp.CallResult{}
			doneCh := make(chan struct{})
			close(doneCh) // Already unblocked

			sipClient.Close()
			return &CallHandle{
				cfg:       cfg,
				sipClient: sipClient,
				done:      doneCh,
				active:    false,
				result:    result,
			}, nil
		}

		sipClient.Close()
		return nil, fmt.Errorf("dial: INVITE → %s: %w", cfg.Target, err)
	}

	// 10. Parse remote SRTP key from 200 OK answer (if SRTP)
	var remoteSRTPCfg *corertp.SRTPConfig
	if cfg.SRTP && inviteResult.Dialog.InviteResponse != nil {
		inlineKey := ParseSDPCrypto(string(inviteResult.Dialog.InviteResponse.Body()))
		if inlineKey != "" {
			remoteSRTPCfg, err = corertp.ParseSRTPConfig(inlineKey)
			if err != nil {
				_ = inviteResult.Dialog.Bye(ctx)
				sipClient.Close()
				return nil, fmt.Errorf("dial: parse remote SRTP key: %w", err)
			}
		}
	}

	// 11. Bind local UDP RTP socket
	bindIP := "0.0.0.0"
	if cfg.IPv6 {
		bindIP = "::"
	}
	localAddr := &net.UDPAddr{IP: net.ParseIP(bindIP), Port: rtpPort}
	conn, err := net.ListenUDP("udp", localAddr)
	if err != nil {
		byeCtx, byeCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer byeCancel()
		_ = inviteResult.Dialog.Bye(byeCtx)
		sipClient.Close()
		return nil, fmt.Errorf("dial: bind RTP port %d: %w", rtpPort, err)
	}

	// Use early media remote address if available, fall back to 200 OK
	remoteIP := inviteResult.RemoteIP
	remotePort := inviteResult.RemotePort
	if earlyMedia != nil && earlyMedia.RemoteIP != "" {
		remoteIP = earlyMedia.RemoteIP
		remotePort = earlyMedia.RemotePort
	}

	remoteAddr := &net.UDPAddr{
		IP:   net.ParseIP(remoteIP),
		Port: remotePort,
	}

	// 12. Create RTP session
	ssrc := rand.Uint32()
	sess := corertp.NewSession(conn, remoteAddr, ssrc)

	// 13. Create SRTP sessions (if enabled)
	var srtpSender *corertp.SRTPSession
	var srtpReceiver *corertp.SRTPSession

	if cfg.SRTP && localSRTPCfg != nil {
		srtpSender, err = corertp.NewSRTPSenderSession(localSRTPCfg, ssrc)
		if err != nil {
			_ = conn.Close()
			return nil, fmt.Errorf("dial: SRTP sender: %w", err)
		}
		if remoteSRTPCfg != nil {
			srtpReceiver, err = corertp.NewSRTPReceiverSession(remoteSRTPCfg, ssrc)
			if err != nil {
				_ = conn.Close()
				return nil, fmt.Errorf("dial: SRTP receiver: %w", err)
			}
		}
	}

	// 14. Media recorder
	var recorder *corertp.AudioRecorder
	var recPath string
	if cfg.EnablePESQ {
		recPath = filepath.Join(os.TempDir(), fmt.Sprintf("xk6-sip-%d.raw", rtpPort))
		recorder, _ = corertp.NewRecorder(recPath)
	} else {
		recorder, _ = corertp.NewRecorder("")
	}

	// 15. Build the handle
	h := &CallHandle{
		cfg:          cfg,
		localIP:      localIP,
		rtpPort:      rtpPort,
		cod:          cod,
		sipClient:    sipClient,
		dialog:       inviteResult.Dialog,
		conn:         conn,
		sess:         sess,
		srtpSender:   srtpSender,
		srtpReceiver: srtpReceiver,
		sendStats:    &corertp.SendStats{},
		recvStats:    &corertp.RTPStats{},
		recorder:     recorder,
		recPath:      recPath,
		stop:         make(chan struct{}),
		done:         make(chan struct{}),
		active:       true,
	}

	// 16. RTP goroutines
	h.wg.Add(1)
	go func() {
		defer h.wg.Done()
		if srtpReceiver != nil {
			corertp.ReceiveSRTP(conn, srtpReceiver, h.recvStats, recorder, h.stop)
		} else {
			corertp.Receive(conn, h.recvStats, recorder, h.stop)
		}
	}()

	switch cfg.AudioMode {
	case "echo":
		h.wg.Add(1)
		go func() {
			defer h.wg.Done()
			corertp.Echo(conn, remoteAddr, h.recvStats, h.stop)
		}()

	case "silent":
		silentFrame := make([]byte, 160)
		silentPayloads := loopPayloads([][]byte{silentFrame}, cfg.Duration)
		h.wg.Add(1)
		go func() {
			defer h.wg.Done()
			if srtpSender != nil {
				corertp.StreamSRTP(sess, srtpSender, silentPayloads, cod.PayloadType(), h.sendStats, h.stop)
			} else {
				corertp.Stream(sess, silentPayloads, cod.PayloadType(), h.sendStats, h.stop)
			}
		}()

	default:
		loopedPayloads := loopPayloads(payloads, cfg.Duration)
		sendPT := cod.PayloadType()
		if cfg.AudioMode == "pcap" && pcapPayloadType > 0 {
			sendPT = pcapPayloadType
		}
		h.wg.Add(1)
		go func() {
			defer h.wg.Done()
			if srtpSender != nil {
				corertp.StreamSRTP(sess, srtpSender, loopedPayloads, sendPT, h.sendStats, h.stop)
			} else {
				corertp.Stream(sess, loopedPayloads, sendPT, h.sendStats, h.stop)
			}
		}()
	}

	// 17. RTCP goroutine (port rtpPort+1)
	if cfg.RTCP {
		rtcpLocal := &net.UDPAddr{IP: net.ParseIP(bindIP), Port: rtpPort + 1}
		rtcpRemote := &net.UDPAddr{IP: net.ParseIP(remoteIP), Port: remotePort + 1}
		rtcpSess, err := corertp.NewRTCPSession(rtcpLocal, rtcpRemote, ssrc, h.recvStats, h.sendStats)
		if err == nil {
			h.wg.Add(1)
			go func() {
				defer h.wg.Done()
				rtcpSess.Run(h.stop)
				rtcpSess.Close()
			}()
		}
	}

	// 18. Finalization goroutine
	h.startFinalize()

	// 19. Apply UDP retransmit config
	if cfg.RetransmitConfig != nil {
		applyRetransmitConfig(sipClient, cfg.RetransmitConfig)
	}

	// 20. DTMF sequence
	if len(cfg.DTMFSequence) > 0 {
		go func() {
			time.Sleep(2 * time.Second)
			for i, digit := range cfg.DTMFSequence {
				if i > 0 {
					time.Sleep(2 * time.Second)
				}
				h.SendDTMF(digit)
			}
		}()
	}

	// 21. Auto-hangup
	if cfg.Duration > 0 {
		go func() {
			select {
			case <-time.After(cfg.Duration):
				_ = h.Hangup()
			case <-h.stop:
			}
		}()
	}

	return h, nil
}

// resolveLocalIPAuto picks the best local IP.
func resolveLocalIPAuto(localIP string, ipv6 bool) string {
	if localIP != "" && localIP != "0.0.0.0" && localIP != "::" {
		return localIP
	}
	if ipv6 {
		if ip := audio.ResolveLocalIPv6(); ip != "" {
			return ip
		}
	}
	ip, _ := localOutboundIP()
	return ip
}
