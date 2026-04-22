# xk6-sip-media — Copilot Instructions

> Auto-loaded every session. Read `.ai/` files before writing code.

---

## BEFORE ANY CODE CHANGE — read these 3 files:

1. **`.ai/config.yaml`** — build commands, codec table, next scenario number
2. **`.ai/context/coding-conventions.md`** — the 5 unbreakable rules (details below)
3. **`.ai/context/architecture.md`** — package map, data flows

### The 5 Rules (violations = broken code)

| # | Rule | Violation = |
|---|---|---|
| 1 | **Never spawn goroutines for timed media** — use `Tickable` + `MediaReactor` | Deadlock/race at scale |
| 2 | **Codec interface = 6 methods** — `Name`, `PayloadType`, `SampleRate`, `Encode`, `Decode`, `Close` | Won't compile |
| 3 | **`ParseSDP()` returns 3 values** — `(ip, port, ptMap)` — NEVER 2 | Call setup failure |
| 4 | **`wg.Add(1)` BEFORE goroutine** — pass `wg.Done` as callback | Early return, leaked goroutines |
| 5 | **`tsIncrement = cod.SampleRate() / 1000 * 20`** — never hardcode 160 | Broken Opus/future codecs |

### Build & Test

```
CGO_ENABLED=1 go build ./...
CGO_ENABLED=1 go test -v -race ./...
```

---

## Past Decisions (check before re-debating)

Read `.ai/memory/adr/` and `.ai/memory/lessons-learned.md` before proposing architectural changes.

| ADR | Decision |
|---|---|
| `001-sharded-reactor.md` | Why MediaReactor uses NumCPU() shards |
| `002-samplerate-interface.md` | Why SampleRate() was added to Codec |
| `003-three-return-parsesdp.md` | Why ParseSDP returns 3 values |
| `004-unified-ai-folder.md` | Why everything is in `.ai/` |

---

## MemPalace — Cross-Session Memory (MCP connected, 29 tools)

MemPalace is available as a native MCP tool. Use it directly — no terminal needed.

- `mempalace_search` — find past decisions, code context, rationale
- `mempalace_status` — palace overview
- `mempalace_kg_query` — query knowledge graph

---

## AFTER CODE CHANGES — update .ai/ files

**You MUST check and update these files after any structural change:**

| If you changed... | Update this file | How |
|---|---|---|
| Added/removed a package or file | `.ai/context/architecture.md` | Add/remove from package map |
| Changed a public API signature | `.ai/context/coding-conventions.md` | Update rule if affected |
| Made an architectural decision | `.ai/memory/adr/NNN-*.md` | Create new ADR (next number) |
| Added a codec | `.ai/config.yaml` | Add to codec table, update Quick Reference below |
| Learned something surprising | `.ai/memory/lessons-learned.md` | Append the lesson |
| Added a new scenario | `.ai/config.yaml` | Increment `next_scenario` |

**ADR format** (create in `.ai/memory/adr/`):
```markdown
# ADR-NNN: <Title>

**Status:** Accepted
**Date:** <today>

## Context
<What problem we faced>

## Decision
<What we chose and why>

## Consequences
<What changes, tradeoffs>
```

**After updating .ai/ files**, also store the decision in MemPalace:
```
mempalace_add_drawer(wing="xk6_sip_media", room="decisions", content="<what changed and why>")
```

---

## Quick Reference

**Codecs:**

| Name | PT | Rate | tsIncrement | CGO | License |
|---|:---:|---:|---:|:---:|---|
| PCMU | 0 | 8kHz | 160 | No | BSD-3 |
| PCMA | 8 | 8kHz | 160 | No | BSD-3 |
| G722 | 9 | 8kHz | 160 | No | Own |
| OPUS | 111 | 48kHz | 960 | Yes | BSD-3 |
| G729 | 18 | 8kHz | 160 | Yes | **GPLv3** |

**Packages:** `k6ext/` (JS API) → `sip/` (signaling) → `core/rtp/` (media) → `core/codec/` (encode/decode)

**Key files:** `sip/dial.go` (call lifecycle), `core/rtp/reactor.go` (sharded reactor), `core/codec/codec.go` (interface + factory), `sip/sdp.go` (3-return ParseSDP)

**Next scenario:** 33 → `examples/k6/scenarios/33_*.js`
