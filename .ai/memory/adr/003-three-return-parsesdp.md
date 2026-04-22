# ADR-003: 3-Return ParseSDP

**Date:** 2026-04-17
**Status:** Accepted

## Context
Dynamic payload type negotiation needed for modern PBX compatibility.

## Decision
`ParseSDP()` returns `(ip, port, ptMap)` instead of `(ip, port)`.

## Consequences
- Eliminates hardcoded PT assumptions
- All callers must destructure 3 values
- `ptMap` maps `uint8 → string` (PT number → codec name)
