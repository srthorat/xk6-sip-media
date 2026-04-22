# ADR-001: Sharded MediaReactor

**Date:** 2026-04-17
**Status:** Accepted

## Context
At 100k streams, flushing 100k UDP syscalls in one goroutine within 20ms is impossible.

## Decision
Replace single-goroutine reactor with CPU-sharded parallel reactor.

## Consequences
- `NumCPU()` goroutines × ~12,500 streams each
- Round-robin `Add()` distributes load
- All timed media uses `Tickable` interface
- Never spawn goroutines for RTP — **the #1 rule**
