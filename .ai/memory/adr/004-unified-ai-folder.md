# ADR-004: Unified .ai/ Folder

**Date:** 2026-04-17
**Status:** Accepted

## Context
Same knowledge duplicated across CLAUDE.md, copilot-instructions.md, .claude/skills/, .agent/rules/.

## Decision
Single `.ai/` folder as the only source of truth. No external pointer files.

## Consequences
- Edit `.ai/` to update knowledge — it's the only place
- Adapters in `.ai/adapters/` explain how each model consumes `.ai/`
- No CLAUDE.md, AGENTS.md, or copilot-instructions.md at root
