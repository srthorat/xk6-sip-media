# xk6-sip-media — Project Walkthrough

## What Is This?

`xk6-sip-media` is a k6 extension that enables production-grade SIP + RTP load testing from JavaScript. It goes beyond SIPp in programmability, quality measurement, and feature completeness — while leveraging k6's executor engine, thresholds, and Prometheus integration.

---

## What Was Built (Complete)

### Phase 1 — Core SIP UAC Foundation
- **`sip/client.go`** — sipgo UserAgent, UDP/TCP/TLS client
- **`sip/invite.go`** — `SendINVITE()` with variadic headers
- **`sip/sdp.go`** — SDP offer/answer builder + parser
- **`sip/call.go`** — `CallConfig` struct + `StartCall()` blocking wrapper
- **`sip/dial.go`** — Non-blocking `Dial()` → `CallHandle`
- **`sip/handle.go`** — Goroutine lifecycle, WaitGroup, finalization

### Phase 2 — Media Engine
- **`core/audio/frame.go`** — 160-sample @ 8kHz frame constants
- **`core/audio/wav_reader.go`** — Delegating WAV + MP3 loader
- **`core/audio/loader.go`** — Format-agnostic audio loader (magic byte detect, resample, downmix)
- **`core/audio/codec_loader.go`** — `LoadAudioForCodec()` — codec-aware loading
- **`core/audio/pcap_reader.go`** — Pure-stdlib PCAP parser (no gopacket)
- **`core/audio/silence.go`** — Energy-based silence ratio
- **`core/codec/codec.go`** — Codec interface + registry
- **`core/codec/g711.go`** — PCMU + PCMA encoder/decoder
- **`core/codec/g722.go`** — G.722 wideband subband ADPCM
- **`core/rtp/session.go`** — RTP session (SSRC, timestamp)
- **`core/rtp/sender.go`** — 20ms-paced RTP stream
- **`core/rtp/receiver.go`** — Packet receive + stats
- **`core/rtp/echo.go`** — RTP loopback reflect
- **`core/rtp/dtmf.go`** — RFC 2833 telephone-event
- **`core/rtp/mos.go`** — E-model MOS calculation
- **`core/rtp/recorder.go`** — PCM recording to disk
- **`core/rtp/stats.go`** — `RTPStats`, `SendStats`, `CallResult`

### Phase 3 — Advanced SIP Call Flows
- **`sip/hold.go`** — Hold/Unhold (re-INVITE a=inactive/sendrecv)
- **`sip/transfer.go`** — Blind Transfer (REFER) + Attended Transfer (REFER+Replaces)
- **`sip/register.go`** — REGISTER + auto Digest Auth (401 retry)
- **`sip/conference.go`** — Bridge-based conference multi-leg manager
- **`sip/info.go`** — SIP INFO method: `SendInfo()` + `SendDTMFInfo()`
- **`sip/vars.go`** — Variable extraction: `ResponseHeader()`, `CallID()`, `ToTag()`, etc.
- **`sip/threepcc.go`** — 3PCC: `Dial3PCC()` + `ThirdPartyCall`
- **`sip/server.go`** — UAS mode: `NewServer()`, `ListenAndServe()`
- **`sip/retransmit.go`** — `RetransmitConfig` struct

### Phase 4 — Transport: TCP + TLS
- **`sip/client.go`** — `NewClientWithTransport()`, `TLSConfig` struct
- **`sip/dial.go`** — Transport URI param injection, sips: scheme, port defaults
- **`sip/register.go`** — Transport-aware REGISTER
- **`scripts/gen_test_certs.sh`** — CA + server + client cert generation

### Phase 5 — Quality & Validation
- **`core/quality/pesq.go`** — PESQ MOS-LQO via external binary
- **`core/quality/ivr.go`** — Rule-based IVR prompt validation
- **`core/ai/validator.go`** — Whisper STT transcript validation

### Phase 6 — k6 Binding Layer
- **`k6ext/module.go`** — RootModule + JS registration
- **`k6ext/sip.go`** — All JS entry points: `call()`, `dial()`, `register()`, `conference()`, `dial3pcc()`, `serve()`
- **`k6ext/call_handle.go`** — All 20+ methods on call handles
- **`k6ext/conference.go`** — Conference JS wrapper
- **`k6ext/registration.go`** — Registration JS wrapper
- **`k6ext/metrics.go`** — 12 custom k6 metrics

### Phase 7 — 16 Load Scenarios
| # | File | What It Tests |
|---|---|---|
| 01 | `scenes/01_baseline.js` | Sanity + low-rate warm-up |
| 02 | `scenes/02_cps_limit.js` | CPS enforcement + bypass |
| 03 | `scenes/03_concurrent_calls.js` | 50 / 200 / 500 CC |
| 04 | `scenes/04_ramp_spike.js` | Ramp + spike + burst |
| 05 | `scenes/05_soak.js` | 1h + 4h soak |
| 06 | `scenes/06_long_duration.js` | 10-min + 1-hour calls |
| 07 | `scenes/07_failure_codes.js` | 403 / 486 / 503 + mixed |
| 08 | `scenes/08_auth_security.js` | Auth + bypass detect + REGISTER storm |
| 09 | `scenes/09_inbound_load.js` | Plain RTP + SRTP SDP |
| 10 | `scenes/10_gcs_routing.js` | GCS happy path + carrier failover |
| 11 | `scenes/11_tcp_transport.js` | TCP load 10→300 VU |
| 12 | `scenes/12_tls_transport.js` | TLS skip-verify + mutual TLS |
| 13 | `scenes/13_uas_server.js` | UAS answer inbound calls |
| 14 | `scenes/14_pcap_replay.js` | PCAP codec-agnostic replay |
| 15 | `scenes/15_3pcc.js` | Click-to-dial / 3PCC |
| 16 | `scenes/16_variable_extraction.js` | Header extraction + SIP INFO |

### Test Coverage
| Package | Tests | Status |
|---|---|---|
| `core/audio` | 18 tests | ✅ All pass |
| `core/rtp` | 6 tests (MOS) | ✅ All pass |
| `sip` | 13 tests (SDP + transport) | ✅ All pass |

---

## Verified: xk6 Surpasses SIPp On

| Area | SIPp | xk6-sip-media |
|---|---|---|
| Scripting | XML only | JavaScript |
| Attended Transfer | ❌ | ✅ REFER+Replaces |
| Conference management | ❌ | ✅ multi-leg |
| MOS scoring | ❌ | ✅ E-model per call |
| PESQ scoring | ❌ | ✅ |
| AI/Whisper validation | ❌ | ✅ |
| Prometheus native | ❌ file only | ✅ |
| MP3 audio input | ❌ | ✅ |
| Variable extraction | XML `<ereg>` | JS `.responseHeader()` |
| Conditional branching | XML `<if>` | JS `if/else/switch` |
| PCAP replay | ✅ | ✅ (pure Go, no gopacket) |
| kGrafana dashboard | ❌ | 🚧 In progress |

---

## What Remains

### P1 — High Value, Short Effort

| Item | Effort | Notes |
|---|---|---|
| **SRTP media encryption** (RFC 3711) | 1 week | SDP negotiation already done; need AES-CM payload encrypt |
| **RTCP** sender/receiver reports | 3 days | Standard quality reporting, required by PESQ |
| **Early media** (183 + ringback) | 2 days | Play audio during ring phase |
| **UDP retransmit** actual tuning | 1 day | `RetransmitConfig` struct done; sipgo hook needed |
| **Grafana dashboard JSON** | 1 day | Template for all 12 custom metrics |

### P2 — Medium Value

| Item | Effort | Notes |
|---|---|---|
| **WebSocket / WSS transport** | 3 days | RFC 7118, needed for browser SIP clients |
| **Docker image** | 1 day | Pre-built k6 binary with extension |
| **SIP OPTIONS** health loop | 1 day | Active keep-alive for SBC monitoring |
| **CANCEL** mid-INVITE | 1 day | Test SBC CANCEL handling at load |
| **G.729** native codec | 1 week | Licensed library or PCAP workaround |

### P3 — Future Extension

| Item | Effort | Notes |
|---|---|---|
| **xk6-webrtc** | 4+ weeks | Separate extension for ICE + DTLS-SRTP + WebRTC |
| **SCTP transport** | — | No Go/sipgo support; skip |
| **T.38 fax** | — | Use PCAP replay |
| **H.264 video RTP** | — | Use PCAP replay or xk6-webrtc |

---

## Architecture Diagram

```
k6 VU goroutine
│
├─ sip.call() / sip.dial()
│   └─ sip/dial.go → Dial()
│       ├─ audio/loader.go → LoadAudioForCodec()   [WAV/MP3 → PCM16 → encoded]
│       ├─ sip/client.go → NewClientWithTransport() [UDP/TCP/TLS]
│       ├─ sip/sdp.go → BuildSDPWithDirection()
│       ├─ sip/invite.go → SendINVITE()             [INVITE → 200 → ACK]
│       └─ CallHandle (goroutines)
│           ├─ rtp/receiver.go → Receive()          [stats tracking]
│           └─ rtp/sender.go → Stream()             [20ms RTP frames]
│
├─ Mid-call (via CallHandle)
│   ├─ sip/hold.go → Hold() / Unhold()
│   ├─ sip/transfer.go → BlindTransfer() / AttendedTransfer()
│   ├─ sip/info.go → SendInfo() / SendDTMFInfo()
│   └─ rtp/dtmf.go → SendDTMF()
│
├─ Finalize (Hangup / BYE)
│   ├─ rtp/mos.go → CalculateMOS()
│   ├─ quality/pesq.go → RunPESQ()
│   └─ k6ext/metrics.go → emit metrics
│
└─ k6 metrics
    ├─ sip_call_success / failure / duration
    ├─ rtp_packets_sent / received / lost
    ├─ rtp_jitter_ms / mos_score
    └─ Prometheus exporter (port 2112)
```

---

## Key Design Decisions

1. **Non-blocking `Dial()` API** — Returns a `CallHandle` immediately after ACK. Enables `Hold()`, `Transfer()`, `SendDTMF()` without blocking the VU.

2. **Goroutine-per-call isolation** — Each `CallHandle` owns exactly 2 goroutines (receiver + sender) tracked by a `sync.WaitGroup`. No shared state between VUs.

3. **Codec-aware audio loading** — Audio files are loaded and encoded at the right sample rate for the codec (8kHz for G.711, 16kHz for G.722). WAV and MP3 are interchangeable.

4. **Pure-stdlib PCAP parser** — No `gopacket` dependency. Parses Ethernet/IP/UDP headers natively, keeping the binary small and cross-platform.

5. **Transport as URI param** — `sip:...;transport=tcp` injected into the target URI before `cache.Invite()`. This is the sipgo-idiomatic way to select transport.

6. **Magic byte format detection** — Audio format is detected by file header bytes, not extension. `hold_music.mp3` works even if renamed to `.wav`.
