# Lessons Learned

Patterns, mistakes, and insights from building xk6-sip-media.

## Reactor
- Single-goroutine reactor hits wall at ~50k streams on 8-core — sharding solved it
- `Tick()` that does any I/O silently degrades all streams on that shard

## Codecs
- Opus tsIncrement=960 (not 160) — caught by adding `SampleRate()` to interface
- G.729 CGO leaks memory if you forget `Close()` — always defer it
- Never hardcode PT numbers — PBXes negotiate dynamically via SDP

## SDP
- Real-world SDP from Oasis/Asterisk/FreeSWITCH often has quirky formatting
- Always test `ParseSDP()` against real captures, not just synthetic ones

## Testing
- `NewJitterBuffer()` triggers `MediaReactor.Add()` — breaks isolated tests
- Raw struct init with `make(map[...])` is the safe pattern for unit tests

## AI instructions
- Duplicating knowledge across CLAUDE.md + copilot-instructions + .cursorrules leads to drift
- Single `.ai/` folder eliminates the consistency problem

## Reactor Fan-out (April 2026)
- `for range ticker.C` across N goroutines is NOT a broadcast — one goroutine wins each tick
- Correct pattern: one fan-out goroutine reads master ticker → writes to N per-shard buffered channels
- Use `select { case ch <- struct{}{}: default: }` in fan-out to skip slow shards rather than blocking

## Opus Audio Loading
- `LoadAudioForCodec` must know about Opus (48kHz/960) — the default 8kHz fallback silently breaks it
- Opus tsIncrement = 960, frame = 960 samples, but encoded payload is VBR (NOT 960 bytes)
- PLC for Opus: inject nothing (silenceSize=0); zeros are not valid Opus bitstream

## PLC Silence Sizes (encoded bytes per 20ms)
- G.711/G.722: 160 bytes | G.729: 20 bytes | Opus: 0 (skip)

## Async Recorder Writes
- `AudioRecorder.Write()` with a file path blocks on disk I/O — stalls reactor shards at concurrency
- Fix: background goroutine drains a `chan []byte` (buffered 4096); `Close()` waits for drain

## Smoke Script Hygiene (April 2026)
- Keep one immutable known-good baseline smoke script for single-leg validation
- Keep intermediate scale smoke scripts when they are already known-good, instead of collapsing everything into one max-scale script
- Scale-out smoke tests should live in separate files with filenames that match their actual VU count
- For carrier validation, use a real WAV file and explicit MOS / TX / RX checks in the script

---

## Deep Review — April 2026 (findings 1.1–6.6, all fixed)

### CGO / Codec lifecycle (1.1)
- `h.cod.Close()` was never called → one CGO memory leak per call leg
- Rule: always `defer cod.Close()` (or call in `startFinalize`) for every codec created via `codec.New()`

### RTCP correctness (1.2, 1.3, 1.4, 5.5, 6.1)
- Compact NTP is the **middle 32 bits** of the 64-bit NTP timestamp: `(sec&0xFFFF)<<16 | frac>>16` — NOT a bit-rotation of seconds (1.2)
- Octet count in SR must be taken from `sendStats.OctetsSent` (actual bytes sent) — never `packets * 160` (breaks all non-PCMU codecs) (1.3)
- RR "highest sequence number received" field (RFC 3550 §6.4.1) must use the extended sequence number (`rollover<<16 | seq`), not `PacketsReceived` count (1.4)
- parseSR compact NTP: use `binary.BigEndian.Uint32(data[10:14])` directly — avoid multi-step bit assembly that is easy to get wrong (5.5)
- RR jitter field is in **RTP timestamp units**, not milliseconds: `uint32(jitter_ms × sampleRate / 1000)` — pass `sampleRate` into `RTCPSession` (6.1)

### Data races (2.1, 2.2)
- `RTPStats` was written by the receiver goroutine and read by the RTCP goroutine without synchronisation → data race under `-race`
- Fix: `sync.Mutex` on `RTPStats` + `Snapshot()` method; readers never touch struct fields directly
- `SendStats.PacketsSent` / `OctetsSent` must be `atomic.Int64` — plain int64 written from sender, read from RTCP = race

### Goroutine lifecycle (3.1, 4.1)
- DTMF-sequence goroutine used bare `time.Sleep` and never checked `h.stop` → goroutine leaked for the call's lifetime (3.1)
- Rule: every goroutine that runs for the duration of a call must `select` on `h.stop`
- `aes.NewCipher` + full SRTP key derivation were running on **every RTP packet** → CPU spike at scale (4.1)
- Fix: derive and cache keys once in `initKeys()` at session construction; packet path is then a single AES-CTR/HMAC call

### Jitter buffer (1.5, 5.1, 5.7, 5.8)
- `Tick()` flush loop iterated all 65535 seq slots (O(65535)) even when the map had 1 entry → CPU waste at low load (1.5)
- Fix: `for seq, pkt := range jb.buf` — range directly over the map
- `playoutDelay` field was stored but never consulted in `Tick()` — advertised feature was silently missing (5.1)
- `JitterBuffer.Close()` had no double-close guard → second close panicked on closed channel (5.7)
- Fix: `sync.Once` wrapping the close logic
- Dead `wg sync.WaitGroup` field in `JitterBuffer` — remove unused fields; they confuse readers and waste 12 bytes (5.8)

### SDP negotiation (5.4)
- `ParseSDP` was injecting G.722 and G.729 into the codec map even when the remote SDP didn't advertise them
- Rule: only add a codec to `ptMap` when its `a=rtpmap` line appears in the remote offer

### k6ext / JavaScript boundary (6.3, 6.4)
- Goja (the k6 JS engine) can emit integers as `int64`, `float64`, `int`, or `int32` depending on value and context
- Never use a bare type assertion `opts["key"].(int64)` — add a `toInt(v interface{}) int` helper that handles all four cases and returns 0 for unrecognised types
- Hardcoded magic-number delays (e.g. 2-second DTMF gap) should be `CallConfig` fields with JS-exposed options and a sensible default applied at the call site, not buried in goroutine code

### Error observability (6.5, 6.6)
- Non-timeout UDP receive errors (EMSGSIZE, ECONNREFUSED, etc.) were silently swallowed with `continue` → impossible to diagnose carrier-side path MTU or firewall issues
- Fix: `RTPStats.RecvErrors atomic.Int64`; increment on every non-timeout error; expose in `RTPStatsSnapshot` and `CallResult.RecvErrors`
- `AudioRecorder.Write()` dropped frames silently when `writeCh` was full (slow disk) → no way to know if a recording was truncated
- Fix: `AudioRecorder.DroppedFrames atomic.Int64`; increment in `default:` branch; expose in `CallResult.RecorderDrops`
- General rule: **every silent drop/discard path needs a counter** visible in the call result

### macOS build (5.9)
- `ld: warning: malformed LC_DYSYMTAB` from CGO opus library on macOS — suppressed cleanly with `CGO_LDFLAGS="-Wl,-w"`
- Add this to the Makefile test target; also document in `config.yaml` `test_command`

### Code duplication (6.2, 5.2)
- `resolveLocalIP` (IPv4-only) and `resolveLocalIPAuto` (IPv4+IPv6) were near-identical — the shorter one now delegates to the longer one with `ipv6=false`
- Local `indexByte()` re-implemented `strings.IndexByte` — always check stdlib before writing a helper
