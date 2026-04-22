# MemPalace — Local-First AI Memory

> Verbatim conversation storage + semantic search. 96.6% recall, zero API calls.
> Nothing leaves your machine unless you opt in.

## What It Does

MemPalace stores conversation history as verbatim text (no summarization) and retrieves it with semantic search. The index is structured like a palace:

- **Wings** — people and projects (e.g. `xk6-sip-media`)
- **Rooms** — topics (e.g. `codec-integration`, `srtp-encryption`)
- **Drawers** — original content (conversations, decisions, code context)

Includes a temporal knowledge graph (SQLite) with validity windows — add, query, invalidate, timeline.

## Install

```bash
pip install mempalace
mempalace init ~/projects/xk6-sip-media
```

Requirements: Python 3.9+, ~300 MB disk for embedding model. No API key needed.

## Mine This Repo

```bash
# Mine project files into the palace
mempalace mine ~/projects/xk6-sip-media

# Mine conversation exports (if you save chat logs)
mempalace mine ~/chats/ --mode convos

# Search past decisions
mempalace search "why did we use sharded reactor instead of goroutine-per-call"

# Load context for a new session
mempalace wake-up
```

## Platform Integration

### Claude Code (auto-save hooks)

MemPalace provides two Claude Code hooks:
1. **Periodic save** — auto-saves conversation to palace at intervals
2. **Pre-compression save** — saves before context window compression

Setup: [mempalaceofficial.com/guide/hooks](https://mempalaceofficial.com/guide/hooks.html)

### MCP Server (any MCP-compatible tool)

29 MCP tools for palace reads/writes, knowledge-graph ops, cross-wing navigation, drawer management, and agent diaries.

```json
{
  "mcpServers": {
    "mempalace": {
      "command": "mempalace",
      "args": ["mcp"]
    }
  }
}
```

Works with: Claude Code, Claude Desktop, VS Code Copilot (MCP), Cursor, any MCP client.

Full tool list: [mempalaceofficial.com/reference/mcp-tools](https://mempalaceofficial.com/reference/mcp-tools.html)

### Codex

MemPalace includes a Codex plugin. See `.codex-plugin/` in the mempalace repo.

### Agent Diaries

Each specialist agent (from `.ai/agents/`) can get its own wing and diary in the palace. Discoverable at runtime via `mempalace_list_agents`.

## How It Complements Graphify

| | Graphify | MemPalace |
|---|---|---|
| **Stores** | Code structure + relationships | Conversation history + decisions |
| **Retrieves** | Graph traversal (nodes, edges, paths) | Semantic search (verbatim text) |
| **Answers** | "What connects X to Y?" | "Why did we decide X?" |
| **Updates** | On code change (AST, no LLM) | On conversation save |
| **Backend** | NetworkX + JSON | ChromaDB + SQLite |

Use both together:
- **Graphify** for architecture questions → `graphify-out/GRAPH_REPORT.md`
- **MemPalace** for historical context → `mempalace search "..."`

## Useful Commands

```bash
mempalace search "why did we add G.729 codec"
mempalace search "what was the SRTP key rotation discussion"
mempalace search "jitter buffer design decision"
mempalace wake-up                    # load context for new session
mempalace mine .                     # re-mine after changes
```

## Docs

- Getting started: [mempalaceofficial.com/guide/getting-started](https://mempalaceofficial.com/guide/getting-started.html)
- CLI reference: [mempalaceofficial.com/reference/cli](https://mempalaceofficial.com/reference/cli.html)
- MCP tools: [mempalaceofficial.com/reference/mcp-tools](https://mempalaceofficial.com/reference/mcp-tools.html)
- Knowledge graph: [mempalaceofficial.com/concepts/knowledge-graph](https://mempalaceofficial.com/concepts/knowledge-graph.html)
- GitHub: [github.com/MemPalace/mempalace](https://github.com/MemPalace/mempalace)
