# xk6-sip-media

[![k6 extension](https://img.shields.io/badge/k6-extension-blue)](https://k6.io/docs/extensions/)
[![Go 1.25](https://img.shields.io/badge/Go-1.25-blue)](https://go.dev/)
[![License](https://img.shields.io/badge/license-Apache%202.0-green)](LICENSE)
[![Tests](https://img.shields.io/badge/tests-passing-brightgreen)](#)
[![Scenarios](https://img.shields.io/badge/scenarios-29-orange)](#)

> **Production-grade SIP + RTP load testing for [k6](https://k6.io).**  
> The only k6 extension that tests real SIP signaling, live RTP audio, SRTP encryption, voice quality (MOS/PESQ/RTCP), and complex call flows at any scale — surpassing SIPp in programmability, codec support, and quality observability.

---

## What It Does

Write SIP load tests in JavaScript, run them with k6's powerful executor engine. Each VU establishes a real SIP call, streams actual audio (WAV or MP3), encrypts media with SRTP if required, and measures voice quality — exactly as a real phone does.

```javascript
import sip from 'k6/x/sip';

export default function () {
  const result = sip.call({
    target:   'sip:ivr@192.168.1.100',
    duration: '30s',
    audio:    { file: './examples/audio/hold_music.mp3' }, // WAV or MP3
    dtmf:     ['1', '2', '#'],
    srtp:     true,    // AES-CM-128-HMAC-SHA1-80 encryption
    rtcp:     true,    // Standard RTCP SR/RR quality reports
  });
  console.log(`MOS: ${result.mos} | jitter: ${result.jitter}ms | lost: ${result.lost}`);
}
```

---

## Feature Matrix

### SIP Signaling
| Feature | Status | Notes |
|---|---|---|
| INVITE / ACK / BYE | ✅ | Full dialog management |
| REGISTER + Digest Auth (401) | ✅ | Auto-retry on 401 challenge |
| Hold / Unhold (re-INVITE) | ✅ | `a=inactive` / `a=sendrecv` |
| Blind Transfer (REFER) | ✅ | RFC 3515 |
| Attended Transfer (REFER+Replaces) | ✅ | RFC 3891 |
| Conference (bridge model) | ✅ | Multi-leg, aggregated metrics |
| 3PCC — Third Party Call Control | ✅ | RFC 3725 |
| SIP INFO (DTMF relay) | ✅ | `application/dtmf-relay` (Cisco/Avaya) |
| OPTIONS health ping | ✅ | RTT measurement |
| Early media (183 Session Progress) | ✅ | Provisional SDP + RTP before answer |
| CANCEL | ✅ | Via sipgo dialog |

### Transport & Security
| Feature | Status | Notes |
|---|---|---|
| UDP | ✅ | Default |
| TCP | ✅ | Persistent connection |
| TLS / SIPS | ✅ | Mutual TLS, custom CA, SNI |
| SRTP (AES-CM-128-HMAC-SHA1-80) | ✅ | RFC 3711 — full payload encryption |
| RTCP SR / RR | ✅ | RFC 3550 — every 5s, RTT via DLSR |
| IPv4 / IPv6 | ✅ | Auto-detect or explicit |
| SCTP | ❌ | Not supported by sipgo |

### Media
| Feature | Status | Notes |
|---|---|---|
| G.711 PCMU (μ-law) | ✅ | PT=0, 8kHz |
| G.711 PCMA (A-law) | ✅ | PT=8, 8kHz |
| G.722 wideband | ✅ | PT=9, 16kHz HD voice |
| RFC 2833 DTMF | ✅ | `telephone-event` PT=101 |
| SIP INFO DTMF | ✅ | Cisco/Avaya style |
| Echo mode | ✅ | Reflect received RTP back |
| Silent mode | ✅ | Comfort noise frames |
| PCAP replay | ✅ | Codec-agnostic byte-accurate |

### Audio Input
| Format | Status | Notes |
|---|---|---|
| WAV (any rate/channels) | ✅ | Auto-resample + downmix |
| MP3 (any bitrate) | ✅ | Pure Go decoder, no CGO |
| PCAP (.pcap) | ✅ | G.729, AMR, T.38 — codec-agnostic |

### Quality Measurement
| Metric | Status | Notes |
|---|---|---|
| E-model MOS (1.0–5.0) | ✅ | Per-call, RFC 3611 |
| PESQ MOS-LQO | 🚧 | Records audio; scoring binary not wired yet |
| RTCP-based jitter | ✅ | Via RTCP RR |
| RTT (round-trip time) | ✅ | Via RTCP DLSR calculation |
| Packet loss % | ✅ | RTP sequence gap detection |
| Silence ratio | ✅ | Energy-based silence detection |
| IVR validation | 🚧 | Field present; no rule engine wired yet |
| AI transcript (Whisper) | 🚧 | Planned; not yet wired |

---

## Quick Start

### Prerequisites

| Tool | Version | Purpose |
|---|---|---|
| [Go](https://go.dev/dl/) | 1.25+ | Build toolchain |
| [xk6](https://github.com/grafana/xk6) | latest | k6 extension builder |
| GCC / Clang | system | CGO required for Opus codec |
| [ffmpeg](https://ffmpeg.org/) | any | Generate test audio (optional) |

> **macOS:** `brew install go gcc ffmpeg`  
> **Ubuntu/Debian:** `apt install golang gcc ffmpeg`

---

### Option A — Makefile (recommended)

```bash
# Clone the repo
git clone https://github.com/srthorat/xk6-sip-media.git
cd xk6-sip-media

# Build the custom k6 binary (installs xk6 automatically)
make build
# Equivalent to: xk6 build --cgo --with github.com/srthorat/xk6-sip-media=.

# Run unit tests
make test

# Clean build artifacts
make clean
```

### Option B — Manual steps

```bash
# 1. Install xk6
go install go.k6.io/xk6/cmd/xk6@latest

# 2. Build the custom k6 binary
#    --cgo is REQUIRED: the Opus codec uses CGO
cd xk6-sip-media
xk6 build --cgo --with github.com/srthorat/xk6-sip-media=.

# 3. Verify the build
./k6 version
# Expected output:
#   k6 v1.7.1 (go1.25.x, darwin/arm64)
#   Extensions:
#     github.com/srthorat/xk6-sip-media (devel), k6/x/sip [js]

# 4. Run unit tests (all packages)
CGO_ENABLED=1 CGO_LDFLAGS="-Wl,-w" go test -race -count=1 ./...

# 5. Generate test audio (requires ffmpeg)
cd examples/audio && bash generate_sample.sh
# Creates: sample.wav, sample.mp3, hold_music.mp3, sample_hd.wav

# 6. Run a sanity test
./k6 run examples/k6/scenarios/01_baseline.js

# 7. Run against a real SIP server
SIP_TARGET="sip:ivr@192.168.1.100" ./k6 run examples/k6/scenarios/01_baseline.js
```

> **Why `--cgo`?** xk6 defaults to `CGO_ENABLED=0`. This extension links `libopus` via CGO.
> Omitting `--cgo` produces: `ERR compiling: executing go command: exit status 1`.

## Vonage Smoke Scripts

Five ready-made scripts for Vonage SIP testing. All require these env vars:

```bash
export VONAGE_USERNAME=<sip-username>
export VONAGE_DOMAIN=<sip-domain>          # e.g. edge4-va.qa7.vonedge.com
export VONAGE_PASSWORD=<sip-password>
export VONAGE_CALLEE=<extension>           # e.g. 443361
export VONAGE_IVR_CALLEE=<ivr-extension>  # e.g. 443362  (ivr_flow only)
```

Or put them in a `.env` file (see `.env.example`) and load with:
```bash
export $(grep -v '^#' .env | xargs)
```

| Script | VUs | Auth | Duration | Checks |
|---|:---:|---|---|---|
| `vonage_direct_call.js` | 1 | Proxy auth (no REGISTER) | 20 s | success, RTP, loss < 5%, MOS > 3.8 |
| `vonage_single_call.js` | 1 | REGISTER + call | 20 s | success, RTP, loss < 5%, MOS > 3.8 |
| `vonage_two_call.js` | 2 | REGISTER + concurrent calls | 20 s | success, RTP, loss < 5%, MOS > 3.8 |
| `vonage_ten_call.js` | 10 | REGISTER + concurrent calls | 20 s | success, RTP, loss < 5%, MOS > 3.8 |
| `vonage_ivr_flow.js` | 1 | REGISTER + dial + DTMF | ~24 s | success, RTP, loss < 5%, hangup ok |

### How to run

```bash
# Direct call (no REGISTER)
./k6 run examples/k6/vonage_direct_call.js

# Single call with REGISTER
./k6 run examples/k6/vonage_single_call.js

# 2 concurrent calls
./k6 run examples/k6/vonage_two_call.js

# 10 concurrent calls
./k6 run examples/k6/vonage_ten_call.js

# IVR flow: dial → wait 3 s → DTMF 2 → wait 20 s → BYE
./k6 run examples/k6/vonage_ivr_flow.js
```

### Sample results (Vonage QA environment — 2026-04-23)

**vonage_direct_call.js** — 1 VU, direct INVITE (proxy auth), no REGISTER
```
✓ call succeeded
✓ sent RTP packets
✓ received RTP packets
✓ packet loss < 5%
✓ mos > 3.8

mos_score............: avg=4.43
rtp_jitter_ms........: avg=0.46 ms
rtp_packets_sent.....: 1000
rtp_packets_received.: 738
rtp_packets_lost.....: 0
sip_call_duration....: avg=21.64s
```

**vonage_single_call.js** — 1 VU, REGISTER + 20 s call
```
✓ call succeeded        ✓ sent RTP packets
✓ received RTP packets  ✓ packet loss < 5%
✓ mos > 3.8

mos_score............: avg=4.42
rtp_jitter_ms........: avg=1.21 ms
rtp_packets_sent.....: 1000
rtp_packets_received.: 741    lost: 1
loss_pct.............: 0.13%
sip_call_duration....: avg=21.2s
silence_ratio........: 0.0422
```

**vonage_two_call.js** — 2 VUs, concurrent calls
```
checks: 10/10 passed (100%)

mos_score............: avg=4.43  min=4.43  max=4.43
rtp_jitter_ms........: avg=0.60  min=0.59  max=0.62 ms
rtp_packets_sent.....: 1998   received: 1478   lost: 0
sip_call_duration....: avg=21.25s
sip_call_success.....: 2
```

**vonage_ten_call.js** — 10 VUs, all concurrent
```
checks: 50/50 passed (100%)

mos_score............: avg=4.43  min=4.43  max=4.43
rtp_jitter_ms........: avg=0.66  min=0.50  max=1.14 ms
rtp_packets_sent.....: 10000   received: 7157   lost: 0
sip_call_duration....: avg=23.24s  min=22.53s  max=24.01s
sip_call_success.....: 10
```

**vonage_ivr_flow.js** — 1 VU, REGISTER → dial IVR → wait → DTMF 2 → BYE
```
✓ call succeeded        ✓ sent RTP packets
✓ received RTP packets  ✓ packet loss < 5%
✓ hangup ok

rtp_packets_sent.....: 1155   received: 1120   lost: 0
rtp_jitter_ms........: avg=2.18 ms
mos_score............: 4.42
iteration_duration...: avg=24.27s
```

---

## Complete API Reference

### `sip.call(opts)` — Blocking SIP call

```javascript
const result = sip.call({
  // ── Required ──────────────────────────────────────────────────
  target:   'sip:ivr@pbx.example.com',
  duration: '30s',

  // ── Audio ─────────────────────────────────────────────────────
  audio: {
    file:  './examples/audio/sample.wav', // WAV or MP3 (auto-detected)
    codec: 'PCMU',                         // PCMU | PCMA | G722
  },
  audioMode: '',       // '' = file | 'echo' | 'silent' | 'pcap'
  pcapFile:  '',       // path to .pcap when audioMode='pcap'

  // ── DTMF ──────────────────────────────────────────────────────
  dtmf: ['1', '2', '#'],  // RFC 2833 sequence (2s inter-digit delay)

  // ── Security ──────────────────────────────────────────────────
  srtp:      false,    // true = AES-CM-128-HMAC-SHA1-80 media encryption
  rtcp:      false,    // true = RTCP SR+RR on rtpPort+1
  earlyMedia: false,   // true = stream audio during 183 ring phase

  // ── Transport ─────────────────────────────────────────────────
  transport: 'udp',    // 'udp' | 'tcp' | 'tls'
  sipPort:   5060,
  ipv6:      false,
  tls: {               // required when transport='tls'
    cert:       './certs/client.pem',
    key:        './certs/client.key',
    ca:         './certs/ca.pem',
    serverName: 'pbx.example.com',
    skipVerify: false,
  },

  // ── Auth ──────────────────────────────────────────────────────
  username: 'alice',
  password: 'secret',

  // ── Custom headers ────────────────────────────────────────────
  headers: {
    'X-Tenant-ID':          'acme',
    'P-Preferred-Identity': 'sip:alice@acme.com',
  },

  // ── Network ───────────────────────────────────────────────────
  localIP: '0.0.0.0',
  rtpPort: 0,           // random if 0

  // ── Quality ───────────────────────────────────────────────────
  pesq: false,          // enable PESQ scoring (pesq binary in PATH)
});
```

**Return value:**
| Field | Type | Description |
|---|---|---|
| `success` | bool | Call completed without error |
| `sent` | int | RTP packets sent |
| `received` | int | RTP packets received |
| `lost` | int | Estimated packets lost |
| `jitter` | float | Average jitter (ms) |
| `mos` | float | E-model MOS (1.0–5.0) |
| `pesq_mos` | float | PESQ MOS-LQO (0 if disabled) |
| `ivr_ok` | bool | IVR rule-based result |
| `transfer_ok` | bool | Transfer success flag |
| `error` | string | Error message (if success=false) |

---

### `sip.dial(opts)` — Non-blocking (returns call handle)

```javascript
const call = sip.dial({
  target:    'sip:ivr@pbx',
  audio:     { file: './sample.wav' },
  duration:  '60s',
  srtp:      true,
  earlyMedia: true,
});

// ── Mid-call operations ──────────────────────────────────────────
call.hold();
call.unhold();
call.sendDTMF('5');                    // RFC 2833
call.sendDTMFInfo('5', 160);           // SIP INFO (Cisco/Avaya)
call.sendInfo('Signal=5\r\n', 'application/dtmf-relay');
call.blindTransfer('sip:agent@pbx');
call.attendedTransfer(consultCall);    // REFER + Replaces (RFC 3891)

// ── Variable extraction (SIPp ereg parity) ──────────────────────
const token   = call.responseHeader('X-Session-Token');
const callID  = call.callID();
const toTag   = call.toTag();
const contact = call.remoteContactURI();
const sdp     = call.responseBody();

// ── Lifecycle ────────────────────────────────────────────────────
call.hangup();
call.waitDone();
const result = call.result();
```

---

### `sip.register(opts)` — SIP REGISTER

```javascript
const reg = sip.register({
  registrar: 'sip:pbx.example.com',
  aor:       'sip:alice@pbx.example.com',
  username:  'alice',
  password:  'secret',
  expires:   3600,
  transport: 'tls',
  tls:       { skipVerify: true },
});

reg.refresh();
reg.unregister();
```

> **Note:** `sip.dial3pcc()` and `sip.serve()` are implemented in Go (`sip/threepcc.go`, `sip/server.go`) but are not yet exposed as k6 JavaScript APIs.

---

### `sip.loadCSV(path, opts?)` — Multi-user credential pool

Load a CSV file of SIP credentials and distribute them across VUs. Each row becomes a credential object whose fields map directly to `sip.call()` / `sip.register()` options.

```javascript
// CSV format (examples/csv/users.csv):
// SEQUENTIAL          ← or RANDOM
// username,password,domain,callee
// alice,secret1,pbx.example.com,1001
// bob,secret2,pbx.example.com,1001

const pool = sip.loadCSV('examples/csv/users.csv');

export default function () {
  // Sequential: VU 1 → the first data row, VU 2 → the second data row, then wraps after the last row
  const creds = pool.pick(__VU);

  // Or: always advance a shared counter (round-robin across all VUs)
  // const creds = pool.pickRoundRobin();

  // Or: uniformly random row per iteration
  // const creds = pool.pickRandom();

  sip.call({
    target:   `sip:${creds.callee}@${creds.domain}`,
    username: creds.username,
    password: creds.password,
    duration: '20s',
    audio:    { file: './examples/audio/sample.wav' },
  });
}
```

**Pool methods:**
| Method | Description |
|---|---|
| `pool.pick(vuId)` | Row for this VU (sequential or random, based on CSV header) |
| `pool.pickRoundRobin()` | Next row via shared atomic counter (global round-robin) |
| `pool.pickRandom()` | Uniformly random row |
| `pool.len()` | Total number of credential rows |

**CSV file rules:**
- First non-comment line: `SEQUENTIAL` (default) or `RANDOM` — sets default pick mode
- Second line: header row (`username`, `password`, `domain`, `callee`, any extras)
- Separator: comma or semicolon, auto-detected
- Extra columns are passed through as strings on the returned object

---

## k6 Metrics Reference

| Metric | Type | Description |
|---|---|---|
| `sip_call_success` | Counter | Successful completed calls |
| `sip_call_failure` | Counter | Failed calls (any error) |
| `sip_call_duration` | Trend | Wall-clock call duration (ms) |
| `rtp_packets_sent` | Counter | Total RTP packets transmitted |
| `rtp_packets_received` | Counter | Total RTP packets received |
| `rtp_packets_lost` | Counter | Estimated lost packets |
| `rtp_jitter_ms` | Trend | Per-call RTP jitter (ms) |
| `mos_score` | Trend | Per-call E-model MOS (1.0–5.0) |
| `sip_transfer_success` | Counter | Successful REFER operations |
| `sip_register_success` | Counter | Successful REGISTER operations |
| `sip_options_success` | Counter | Successful OPTIONS pings |
| `sip_options_failure` | Counter | Failed OPTIONS pings |
| `sip_options_rtt_ms` | Trend | OPTIONS round-trip time (ms) |
| `rtp_bytes_sent` | Counter | RTP bytes transmitted |
| `rtp_bytes_received` | Counter | RTP bytes received |

### Threshold examples

```javascript
export const options = {
  thresholds: {
    mos_score:          ['avg>=3.5'],
    rtp_jitter_ms:      ['avg<50'],
    rtp_packets_lost:   ['rate<0.01'],
    sip_call_failure:   ['rate<0.01'],
    sip_call_duration:  ['p(95)<2000'],
  },
};
```

---

## Audio Formats

| Input | Auto-converts to |
|---|---|
| WAV (any rate, any channels) | Resample → 8kHz/16kHz, downmix → mono |
| MP3 (any bitrate, any rate) | Decode → resample → downmix |
| PCAP (.pcap) | Byte-accurate RTP replay (G.729, AMR, T.38…) |

```bash
# Generate all test file formats
cd examples/audio && bash generate_sample.sh
# sample.wav       — 8kHz mono   (native telephony, zero processing)
# sample_hd.wav    — 16kHz mono  (G.722 wideband)
# sample_44k.wav   — 44.1kHz stereo (auto-resample test)
# sample.mp3       — 128kbps stereo (auto-decode test)
# hold_music.mp3   — 30s hold music
```

```javascript
// All equivalent — format auto-detected by magic bytes:
sip.call({ audio: { file: './sample.wav' } });
sip.call({ audio: { file: './hold_music.mp3' } });
sip.call({ audio: { file: './sample_44k.wav' } }); // auto-resampled
sip.call({ audio: { file: './sample_hd.wav', codec: 'G722' } });
```

---

## SRTP (Encrypted Media)

```javascript
// Basic SRTP (plain UDP signaling + encrypted RTP)
sip.call({ target: TARGET, srtp: true, audio: { file: AUDIO } });

// Full encryption (TLS signaling + SRTP media = SIPS+SRTP)
sip.call({
  target:    'sips:ivr@pbx',
  transport: 'tls',
  srtp:      true,
  tls:       { skipVerify: true },
  audio:     { file: AUDIO },
});
```

SRTP key exchange is fully automatic:
1. xk6 generates a fresh 30-byte AES-CM master key per call
2. Advertises it in SDP `a=crypto:1 AES_CM_128_HMAC_SHA1_80 inline:<base64>`
3. Parses remote party's key from 200 OK SDP
4. All RTP packets are AES-CM encrypted + HMAC-SHA1-80 authenticated

---

## RTCP Quality Reports

```javascript
sip.call({ target: TARGET, rtcp: true, audio: { file: AUDIO } });
```

RTCP runs on `rtpPort + 1` and sends:
- **Sender Reports (SR)** every 5 seconds with NTP timestamp, packet count, octet count
- **Receiver Reports (RR)** immediately after receiving SR, with fraction lost, cumulative loss, jitter, LSR/DLSR
- **RTT calculation** from DLSR field per RFC 3550 §6.4.1

---

## Early Media (183 Session Progress)

```javascript
sip.call({ target: TARGET, earlyMedia: true, audio: { file: AUDIO } });
```

When the remote sends `183 Session Progress` with SDP, xk6 begins streaming RTP toward the provisional remote address immediately — before the call is answered. Used for IVR queuing announcements, ringback tones, and carrier interconnects.

---

## TLS Setup

```bash
# Generate self-signed CA + server + client certificates
SIP_DOMAIN=pbx.example.com bash scripts/gen_test_certs.sh

# Run with TLS
SIP_TARGET="sips:ivr@pbx"          \
TLS_CERT=./certs/client.pem        \
TLS_KEY=./certs/client.key         \
TLS_CA=./certs/ca.pem              \
./k6 run examples/k6/scenarios/12_tls_transport.js
```

---

## Prometheus + Grafana

```bash
# Start Prometheus exporter
PROMETHEUS_ENABLED=1 PROMETHEUS_PORT=2112 \
  ./k6 run examples/k6/scenarios/03_concurrent_calls.js

# Metrics endpoint: http://localhost:2112/metrics
# Import dashboard: Grafana → Dashboards → Import → grafana/xk6-sip-dashboard.json
```

The dashboard includes:
- ✅ / ❌ Call success/failure counters  
- 🎙️ MOS score gauge with ITU-T color thresholds
- ⏱️ CPS rate, call duration percentiles (p50/p95/p99)
- 📦 RTP packet rate (sent/received/lost)
- REGISTER rate, transfer rate, conference leg count

---

## Scaling

```bash
# OS tuning for 1000+ concurrent calls
sudo sysctl -w net.ipv4.ip_local_port_range="10000 65535"
ulimit -n 200000

# Distributed k6
k6 cloud examples/k6/scenarios/05_soak.js
```

---

## Load Scenarios (29 scripts)

### Batch 1 — Core Load (01–10)
| # | Scenario | Tests |
|---|---|---|
| 01 | Baseline / Warm-up | Single call sanity + low-rate warm-up |
| 02 | CPS Limit | Rate enforcement, cross-region, emergency bypass |
| 03 | Concurrent Calls | 50 / 200 / 500 CC |
| 04 | Ramp / Spike | Gradual ramp + 200% spike + burst |
| 05 | Soak / Endurance | 1-hour + 4-hour soak |
| 06 | Long Duration | 10-min + 1-hour calls |
| 07 | SIP Failure Codes | 403 / 486 / 503 + mixed |
| 08 | Auth / Security | Valid + invalid bulk + REGISTER storm |
| 09 | Inbound Load | Plain RTP + SRTP SDP negotiation |
| 10 | GCS Routing | Happy path + carrier failover |

### Batch 2 — Transport (11–12)
| # | Scenario | Tests |
|---|---|---|
| 11 | TCP Transport | 10→300 CC over persistent TCP |
| 12 | TLS / SIPS | Skip-verify + mutual TLS |

### Batch 3 — Advanced Features (13–16)
| # | Scenario | Tests |
|---|---|---|
| 13 | UAS Server | Answer inbound calls at load |
| 14 | PCAP Replay | Codec-agnostic byte-accurate media replay |
| 15 | 3PCC | Click-to-dial, recording server orchestration |
| 16 | Variable Extraction | Dynamic header routing, SIP INFO relay |

### Batch 4 — New Features (17–30)
| # | Scenario | Tests |
|---|---|---|
| 17 | SRTP Encrypted | AES-CM-128-HMAC-SHA1-80 media encryption |
| 18 | RTCP Quality | SR/RR reports, RTT measurement, jitter from RTCP |
| 19 | Early Media | 183 Session Progress + ring-phase audio |
| 20 | Hold / Unhold | re-INVITE hold/resume under concurrent load |
| 21 | Blind Transfer | REFER load, 202 Accepted rate, transfer time |
| 22 | Attended Transfer | REFER+Replaces (impossible in SIPp) |
| 23 | Conference Load | 3-party bridge, join latency, bridge capacity |
| 24 | DTMF IVR Navigation | RFC 2833 + SIP INFO, multi-level IVR menus |
| 25 | Echo Loopback MOS | Round-trip voice quality via RTP echo |
| 26 | G.722 Wideband | HD voice, SBC codec negotiation |
| 27 | MP3 Audio | MP3 decode→resample→G.711→RTP pipeline |
| 28 | Proxy Auth 407 | SIP Proxy 407 challenge / Digest calculation |
| 29 | CANCEL Mid-Ring | Trigger `CancelAfter` exactly during provisional `180 Ringing` |
| 30 | OPTIONS Ping | High frequency connectionless SIP heartbeat checks |
| 33 | Multi-User CSV | Concurrent calls from CSV credential pool |
| 34 | CSV Ramp | Ramping VUs with CSV credentials |
| 35 | CANCEL Load | CANCEL mid-INVITE stress test — validates 487 handling at scale |

### Base examples (non-numbered)
| File | Description |
|---|---|
| `call.js` | Minimal single-call example |
| `register_only.js` | Register only example |
| `register_call.js` | Register then call |
| `ivr_flow.js` | IVR + AI transcript validation |
| `vonage_direct_call.js` | Vonage direct call without prior REGISTER |
| `vonage_ivr_flow.js` | Vonage IVR flow: dial 443362, send DTMF 1, then BYE |

### Advanced scenarios (non-numbered)
| File | Description |
|---|---|
| `conference.js` | Conference bridge example |
| `transfer_blind.js` | Blind transfer example |
| `transfer_attended.js` | Attended transfer example |

---

## Project Structure

```
xk6-sip-media/
│
├── k6ext/                    # k6 JS binding layer
│   ├── module.go             # RootModule, initialization
│   ├── sip.go                # call(), dial(), register(), conference(), options(), loadCSV()
│   ├── call_handle.go        # All mid-call methods (hold, transfer, DTMF, etc.)
│   ├── healthcheck.go        # startHealthCheck() — background OPTIONS loop
│   ├── conference.go         # Conference JS wrapper
│   ├── registration.go       # Registration JS wrapper
│   └── metrics.go            # Custom k6 metrics
│
├── sip/                      # SIP protocol layer
│   ├── client.go             # UA: UDP / TCP / TLS transport
│   ├── dial.go               # Non-blocking Dial(): SRTP + RTCP + EarlyMedia wired
│   ├── call.go               # CallConfig (SRTP / RTCP / EarlyMedia fields)
│   ├── handle.go             # CallHandle: goroutines, WaitGroup, SRTP sessions
│   ├── invite.go             # SendINVITE (variadic headers)
│   ├── early_media.go        # SendINVITEWithEarlyMedia: 183 interception
│   ├── sdp.go                # SDP offer/answer builder + parser
│   ├── sdp_srtp.go           # BuildSDPWithSRTP, ParseSDPCrypto (RFC 4568)
│   ├── hold.go               # Hold/Unhold (re-INVITE)
│   ├── transfer.go           # Blind + Attended REFER
│   ├── register.go           # REGISTER + Digest Auth
│   ├── conference.go         # Bridge conference manager
│   ├── server.go             # UAS: answer inbound calls
│   ├── info.go               # SIP INFO method
│   ├── vars.go               # Variable extraction from responses
│   ├── healthcheck.go        # Background OPTIONS ping loop
│   ├── threepcc.go           # 3PCC orchestration (Go only; no JS binding yet)
│   ├── server.go             # UAS: answer inbound calls (Go only; no JS binding yet)
│   ├── retransmit.go         # RetransmitConfig struct
│   └── transport_utils.go    # applyRetransmitConfig, sipgo retransmit hook
│
├── core/
│   ├── audio/
│   │   ├── loader.go         # Format-agnostic loader (WAV + MP3, resample, downmix)
│   │   ├── codec_loader.go   # LoadAudioForCodec (codec-aware sample rate selection)
│   │   ├── pcap_reader.go    # Pure-stdlib PCAP parser + IPv6 detect
│   │   ├── frame.go          # FrameSize constants (8kHz/16kHz)
│   │   ├── pcm.go            # IntToInt16 helper
│   │   └── silence.go        # Silence ratio (energy-based)
│   │
│   ├── codec/
│   │   ├── codec.go          # Codec interface + registry
│   │   ├── g711.go           # PCMU + PCMA
│   │   ├── g722.go           # G.722 16kHz wideband
│   │   └── opus.go           # Opus 48kHz (CGO, libopus)
│   │
│   └── rtp/
│       ├── session.go        # RTP session: SSRC, sequence, timestamp
│       ├── sender.go         # Stream(): 20ms-paced RTP transmission
│       ├── receiver.go       # Receive(): packet stats tracking
│       ├── echo.go           # Echo(): RTP loopback reflect
│       ├── srtp.go           # SRTP: AES-CM-128-HMAC-SHA1-80 (RFC 3711)
│       ├── rtcp.go           # RTCP: SR + RR + RTT calculation (RFC 3550)
│       ├── dtmf.go           # RFC 2833 telephone-event
│       ├── mos.go            # E-model MOS calculation
│       ├── jitter.go         # Adaptive jitter buffer
│       ├── reactor.go        # Sharded MediaReactor (NumCPU shards)
│       ├── recorder.go       # PCM recording
│       └── stats.go          # RTPStats, SendStats, CallResult
│
├── metrics/                  # Prometheus exporter
├── grafana/
│   └── xk6-sip-dashboard.json  # Production Grafana dashboard (import directly)
├── scripts/
│   └── gen_test_certs.sh     # TLS cert generator (CA + server + client)
│
└── examples/
    ├── audio/
    │   ├── generate_sample.sh   # Generates WAV + MP3 test files (ffmpeg)
    │   └── README.md
    └── k6/
        ├── call.js               # Minimal call example
        ├── conference.js         # Conference example
        ├── transfer_blind.js     # Blind transfer
        ├── transfer_attended.js  # Attended transfer
        ├── register_call.js      # Register then call
        ├── ivr_flow.js           # IVR + AI validation
        └── scenarios/            # 29 load scenarios (01–35)
```

---

## vs SIPp Comparison

| Capability | SIPp | xk6-sip-media |
|---|---|---|
| UAC (make calls) | ✅ | ✅ |
| UAS (answer calls) | ✅ | ✅ |
| UDP / TCP / TLS | ✅ | ✅ |
| IPv4 / IPv6 | ✅ | ✅ |
| PCAP media replay | ✅ | ✅ |
| SRTP encrypted media | ❌ | ✅ RFC 3711 |
| RTCP SR + RR | ❌ | ✅ RFC 3550 |
| Early media (183) | Partial | ✅ |
| Attended Transfer | **❌** | ✅ REFER+Replaces |
| Conference management | **❌** | ✅ multi-leg |
| 3PCC (RFC 3725) | ✅ XML | 🚧 Go implementation; JS binding pending |
| MOS scoring | **❌** | ✅ E-model per call |
| PESQ scoring | **❌** | 🚧 Planned |
| RTCP RTT measurement | **❌** | ✅ |
| AI transcript validation | **❌** | 🚧 Planned |
| MP3 audio input | **❌** | ✅ |
| G.722 wideband | Via PCAP | ✅ native |
| Grafana dashboard | **❌** | ✅ import-ready JSON |
| Scripting language | XML | **JavaScript** |
| Variable extraction | XML `<ereg>` | JS `.responseHeader()` |
| Prometheus native | File only | **✅ native** |
| CSV credential pool | ❌ | ✅ `sip.loadCSV()` — sequential, round-robin, or random |
| Multi-user concurrent calls | Via `-inf` users file | ✅ CSV pool × k6 VUs, no extra tooling |

---

## Dependencies

| Library | Version | Purpose |
|---|---|---|
| `github.com/emiago/sipgo` | v0.30.0 | SIP stack (UAC + UAS + dialogs) |
| `github.com/pion/rtp` | v1.8.7 | RTP packet marshal/unmarshal |
| `github.com/zaf/g711` | v1.4.0 | G.711 µ-law / A-law codecs |
| `github.com/hajimehoshi/go-mp3` | v0.3.4 | Pure-Go MP3 decoder (no CGO) |
| `github.com/go-audio/wav` | v1.1.0 | WAV file decoder |
| `github.com/prometheus/client_golang` | v1.23.2 | Prometheus metrics |
| `go.k6.io/k6` | v1.7.1 | k6 extension framework |

Most media processing (MP3 decode, PCAP parse, G.722, SRTP, RTCP) is implemented in pure Go. The Opus codec requires CGO and links against `libopus`.

---

## Roadmap

### Pre-Release Validation
- [x] **Infrastructure baseline** — validated with `33_multi_user_csv.js` and `34_multi_user_csv_ramp.js` against Vonage (multi-user CSV pool, concurrent calls, ramp scenarios)
- [ ] **Performance validation** — verify 1,000+ concurrent calls with SRTP enabled without crashing
- [ ] **Quality telemetry check** — verify Grafana displays E-model MOS and RTCP jitter correctly under heavy load

### Publication
- [ ] **k6 Registry submission** — open a PR against `grafana/k6-docs` to list on the official [k6 Extensions Directory](https://k6.io/docs/extensions/)

### Near-term
- [ ] **WebSocket / WSS transport** — SIP over WebSocket (RFC 7118), browser SIP clients
- [ ] **G.729 native codec** — licensed library or arithmetic encoder

### Medium-term
- [ ] **Docker image** — pre-built k6 + extension
- [ ] **k6 cloud distributed guide** — multi-region SIP load generation

---

## License

Apache 2.0 — see [LICENSE](LICENSE)
