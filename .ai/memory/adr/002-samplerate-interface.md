# ADR-002: SampleRate() on Codec Interface

**Date:** 2026-04-17
**Status:** Accepted

## Context
Adding Opus (48kHz) broke the hardcoded `tsIncrement=160` assumption.

## Decision
Add `SampleRate() int` to Codec interface, remove hardcoded tsIncrement.

## Consequences
- `tsIncrement = cod.SampleRate()/1000*20` — works for any future codec
- All 5 codecs implement `SampleRate()`
- Opus correctly gets `tsIncrement=960`
