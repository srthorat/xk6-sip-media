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
