# Coding Conventions — The 5 Design Patterns

## 1. Media Reactor — The #1 Rule

**Never spawn goroutines for timed media.** All senders implement `Tickable` + register with `MediaReactor`.

```go
// ✅ CORRECT
corertp.MediaReactor.Add(&MyPlayer{})

// ❌ NEVER
go func() { ticker := time.NewTicker(20*time.Millisecond); for range ticker.C { sendPacket() } }()
```

Reactor: `NumCPU()` shards × round-robin `Add()`.

## 2. Tickable Interface

```go
type Tickable interface {
    Tick() bool  // true = keep alive, false = remove
}
```

Called every 20ms. **Must be non-blocking** — no sleep, no I/O.

## 3. Codec Interface — All 6 Methods

```go
type Codec interface {
    Name()        string
    PayloadType() uint8
    SampleRate()  int
    Encode([]int16) []byte
    Decode([]byte) []int16
    Close()       error
}
```

## 4. WaitGroup Lifecycle

```go
h.wg.Add(1)
corertp.Stream(sess, payloads, pt, tsIncrement, sendStats, stop, h.wg.Done)
```

## 5. Dynamic SDP Payload Type

`ParseSDP()` returns 3 values: `(ip, port, ptMap map[uint8]string)`

```go
for pt, name := range inviteResult.PtMap {
    if name == cod.Name() { sendPT = pt; break }
}
```

## 6. codec.New() Returns (Codec, error)

```go
// ✅ CORRECT — always handle the error
cod, err := codec.New(cfg.Codec)
if err != nil {
    return nil, fmt.Errorf("codec %q: %w", cfg.Codec, err)
}

// ❌ NEVER — ignores init failures (libopus missing → crash)
cod := codec.New(cfg.Codec)
```

## 7. Jitter Buffer silenceSize

`NewJitterBuffer(recorder, delay, silenceSize)` — silenceSize is encoded bytes per 20ms:
- G.711/G.722: `160`
- G.729: `20`
- Opus: `0` (skip PLC — VBR, zeros are not valid encoded silence)

## 8. Smoke Script Stability

- Keep the known-good single-call smoke test in `examples/k6/vonage_single_call.js`
- Keep a separate medium-concurrency smoke test in `examples/k6/vonage_two_call.js`
- Put scaled variants in separate files and name them for their real concurrency, e.g. `vonage_ten_call.js`
- Do not repurpose the baseline smoke file for concurrency experiments

## Anti-Patterns

- ❌ `time.Sleep()` inside `Tick()`
- ❌ `go func() { ticker := ... }()` for audio
- ❌ Hardcoded PT or tsIncrement
- ❌ 2-return `ParseSDP()`
- ❌ Direct `MediaReactor.shards` access
- ❌ `log.Fatal` / `panic` in library code (use `error` returns)
- ❌ Direct codec struct instantiation — use `codec.New("NAME")`
- ❌ Blocking I/O inside `Tick()` — stalls the entire shard
- ❌ Sharing one `time.Ticker.C` across multiple goroutines — use fan-out channels
