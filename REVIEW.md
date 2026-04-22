# Deep Review — xk6-sip-media

**Date:** 2026-04-22 | **Closed:** 2026-04-23  
**Scope:** Full repository — SE best practices, performance, memory, races, code smells  
**Branch:** `bugfix/deep-review-2026-04-22`  
**Build:** `CGO_ENABLED=1 go build ./...` — clean  
**Tests:** `CGO_ENABLED=1 CGO_LDFLAGS="-Wl,-w" go test -race -count=1 ./...` — **all pass, 0 warnings**

---

## Fixed Findings

| # | Severity | Location | Issue | Fix |
|---|---|---|---|---|
| 1.1 | Critical | `sip/handle.go` | `h.cod.Close()` never called — CGO memory leak per call | Added `_ = h.cod.Close()` in `startFinalize()` |
| 1.2 | Critical | `core/rtp/rtcp.go` `parseRR()` | RTCP RTT compact-NTP formula was bit-rotating seconds, not extracting middle 32 bits | `(nowSec&0xFFFF)<<16 \| nowFrac>>16` |
| 1.3 | Critical | `core/rtp/rtcp.go` `buildSR()` | Octet count hardcoded to `sent*160` (PCMU-only) | Use `sendStats.OctetsSent.Load()` |
| 1.4 | High | `core/rtp/rtcp.go` `buildRR()` | RFC 3550 "highest seq" field used `PacketsReceived` count | Use `RTPStats.HighestSeqExtended` |
| 1.5 | High | `core/rtp/jitter.go` `Tick()` | Flush loop walked all 65535 seq slots — O(65535) on sparse map | Range directly over the map |
| 2.1 | High | `core/rtp/receiver.go` + `rtcp.go` | `RTPStats` written by receiver goroutine, read by RTCP goroutine — data race | Added `sync.Mutex` + `Snapshot()` method |
| 2.2 | High | `core/rtp/sender.go` + `rtcp.go` | `SendStats.PacketsSent` had no synchronization — data race | Changed to `atomic.Int64` |
| 3.1 | Medium | `sip/dial.go` | DTMF-sequence goroutine used `time.Sleep`, ignored `h.stop` — leaked goroutine | `select` on `h.stop` |
| 4.1 | High | `core/rtp/srtp.go` | `aes.NewCipher` + full key derivation ran on every RTP packet | Keys cached in `initKeys()` at session construction |
| 5.1 | Medium | `core/rtp/jitter.go` | `playoutDelay` stored but never used in `Tick()` — advertised feature silently missing | Implemented `firstPktTime`/`delayExpired` gate in `Tick()` |
| 5.2 | Low | `core/rtp/srtp.go` | Local `indexByte()` reimplemented `strings.IndexByte` | Replaced with `strings.IndexByte` |
| 5.3 | Medium | `core/audio/silence.go` | `SilenceRatioBytes` is PCMU-specific but comment was silent about it | Clarified doc comment |
| 5.4 | Medium | `sip/sdp.go` | `ParseSDP` injected G.722 and G.729 even when remote didn't advertise them | Removed those injection blocks |
| 5.5 | Low | `core/rtp/rtcp.go` `parseSR()` | Compact NTP constructed in two confusing steps | `binary.BigEndian.Uint32(data[10:14])` |
| 5.6 | Low | `core/codec/codec.go` | Doc comment omitted G.729 from supported codec list | Updated doc comment |
| 5.7 | Low | `core/rtp/jitter.go` | `JitterBuffer.Close()` had no double-close protection | `sync.Once` guard added |
| 5.8 | Low | `core/rtp/jitter.go` | Dead `wg sync.WaitGroup` field in `JitterBuffer` | Removed |
| 5.9 | Low | macOS build | `ld: warning: malformed LC_DYSYMTAB` from CGO opus library | `CGO_LDFLAGS="-Wl,-w"` in Makefile test target |
| 6.1 | Medium | `core/rtp/rtcp.go` `buildRR()` | Jitter stored as float64 ms but serialised as uint32 without unit conversion | `uint32(jitter_ms × sampleRate / 1000)`; `sampleRate` added to `RTCPSession` |
| 6.2 | Low | `sip/client.go` + `sip/dial.go` | Two near-identical `resolveLocalIP*` helpers | `resolveLocalIP` delegates to `resolveLocalIPAuto` |
| 6.3 | Medium | `k6ext/sip.go` | `opts["expires"].(int64)` fragile for JS integer types | Added `toInt()` helper; applied to `expires`, `rtpPort`, `sipPort` |
| 6.4 | Medium | `sip/call.go` + `sip/dial.go` | Hardcoded 2-second DTMF initial/inter-digit delays | `DTMFInitialDelay`, `DTMFInterDigitGap` fields in `CallConfig`; JS options `dtmfInitialDelay`, `dtmfInterDigitGap` |
| 6.5 | Medium | `core/rtp/receiver.go` | Non-timeout UDP errors silently discarded | `RTPStats.RecvErrors atomic.Int64`; exposed in `RTPStatsSnapshot` and `CallResult.RecvErrors` |
| 6.6 | Low | `core/rtp/recorder.go` | `writeCh` frame drops silent — no diagnostic | `AudioRecorder.DroppedFrames atomic.Int64`; exposed in `CallResult.RecorderDrops` |

---

## New Tests Added

| File | Tests |
|---|---|
| `core/rtp/stats_test.go` | Atomic add, concurrent add, snapshot consistency, rollover (×3), loss with gaps, loss % zero, RecvErrors in snapshot, RecvErrors zero default, RecvErrors concurrent add |
| `core/rtp/rtcp_test.go` | NTP compact formula, RTT formula, SR octet count (×2), RR highest-seq (×2), fraction lost, parseSR state update, jitter units 8 kHz, jitter units 48 kHz, jitter units zero sampleRate (no panic) |
| `core/rtp/jitter_test.go` | Double-close, playout delay hold, playout delay release, zero delay immediate, O(n) flush |
| `core/rtp/srtp_test.go` | Sender/receiver key caching, cached == derived, multi-packet encrypt/decrypt, pipe-suffix stripped |
| `core/rtp/recorder_test.go` | DroppedFrames channel full, DroppedFrames nil channel (no drops), DroppedFrames concurrent write (race check), DroppedFrames buffered (no drops) |
| `sip/sdp_test.go` | G.722 not injected, G.729 not injected, G.722 present when offered, PCMU always present |
| `sip/transport_test.go` | resolveLocalIP empty returns valid IP, resolveLocalIP passthrough, resolveLocalIP matches resolveLocalIPAuto |
| `k6ext/sip_test.go` | toInt int64, toInt float64, toInt int, toInt int32, toInt unknown type, toInt nil, toInt missing map key; parseCfg dtmfInitialDelay, parseCfg dtmfInterDigitGap, parseCfg both fields, parseCfg zero value, parseCfg invalid duration |

