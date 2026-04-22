# Architecture — xk6-sip-media

## Package Map

```
k6ext/          ← JavaScript API surface (sip.call, sip.dial, sip.register, etc.)
sip/            ← SIP UAC orchestration: Dial, Invite, EarlyMedia, CANCEL, REGISTER
  ├── dial.go      Main call lifecycle (INVITE→ACK→RTP→BYE)
  ├── sdp.go       SDP offer/answer builder + dynamic a=rtpmap parser
  ├── invite.go    INVITE state machine
  ├── handle.go    CallHandle — live call operations
  ├── server.go    UAS mode — inbound INVITE handler
  └── cancel.go    Mid-ring CANCEL + CancelAfter config
core/
  ├── rtp/
  │   ├── reactor.go    ★ SHARDED MEDIA REACTOR — NumCPU() workers
  │   ├── jitter.go     ★ Adaptive Jitter Buffer (PLC silence)
  │   ├── sender.go     StreamPlayer — Tickable RTP sender
  │   ├── receiver.go   UDP receive → JitterBuffer → AudioRecorder
  │   ├── srtp.go       SRTP encrypt/decrypt + StreamSRTPPlayer
  │   ├── session.go    RTP session state (SSRC, seq, timestamp)
  │   ├── dtmf.go       RFC 2833 telephone-event DTMF
  │   ├── echo.go       Echo mode — reflect RTP back
  │   ├── rtcp.go       RTCP sender/receiver reports
  │   └── stats.go      RTPStats, SendStats, CallResult
  ├── codec/
  │   ├── codec.go      Codec interface + factory
  │   ├── g711.go       PCMU (PT=0) + PCMA (PT=8)
  │   ├── g722.go       G.722 wideband (PT=9)
  │   ├── opus.go       Opus 48kHz (PT=111) — CGO
  │   └── g729.go       G.729 8kHz (PT=18) — CGO
  ├── audio/            WAV/MP3 loader, PCM resampler, silence detector
  └── quality/          PESQ/MOS scoring
metrics/        ← Prometheus exporter
grafana/        ← Dashboard JSON
examples/k6/    ← Hand-authored smoke scripts (`vonage_single_call.js`, `vonage_ten_call.js`)
examples/k6/scenarios/  ← 32 k6 test scripts
```

## Key Data Flows

1. **Call flow:** k6 VU → `sip.call()` → `Dial()` → `SendINVITE()` → SDP negotiation → RTP stream → BYE → MOS
2. **Media flow:** Audio file → `LoadAudioForCodec()` → Encode → `StreamPlayer.Tick()` (reactor) → UDP send
3. **Receive flow:** UDP read (goroutine) → `JitterBuffer.Push()` → `JitterBuffer.Tick()` (reactor) → decode → record

## Smoke Entry Points

- `examples/k6/register_only.js` — generic REGISTER-only example with optional unregister and smoke mode
- `examples/k6/vonage_direct_call.js` — direct Vonage call without prior REGISTER; relies on INVITE proxy auth
- `examples/k6/vonage_single_call.js` — baseline 1-call Vonage smoke test
- `examples/k6/vonage_ivr_flow.js` — Vonage IVR flow: dial 443362, wait 3 s, press 1, wait 2 s, send BYE
- `examples/k6/vonage_two_call.js` — 2 concurrent Vonage calls, 1 iteration per VU
- `examples/k6/vonage_ten_call.js` — 10 concurrent Vonage calls, 1 iteration per VU
