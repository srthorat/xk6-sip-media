# k6-docs Extension Registry Submission

To get your extension officially listed on `https://k6.io/docs/extensions/`, you need to fork the `grafana/k6-docs` repository and submit a Pull Request.

## 1. Where to add your entry
In the `grafana/k6-docs` repository, locate the JSON file that lists extensions. This is typically located at:
`src/data/markdown/docs/01_extensions/02_explore/extensions.json`
*(Note: file paths in their repo change occasionally, so look for the JSON array of extensions).*

## 2. What to insert
Add the following JSON block into the `extensions.json` array (maintaining alphabetical order):

```json
{
  "name": "xk6-sip-media",
  "description": "High-performance SIP and RTP media engine for VoIP load testing. Features sharded CPU-parallel reactors (up to 100k concurrent streams), Opus/G.722/G.711 native codecs, dynamic WebRTC-style SDP payload negotiation, and adaptive jitter buffers with Packet Loss Concealment (PLC).",
  "author": {
    "name": "Your Name/Organization",
    "url": "https://github.com/your-username"
  },
  "url": "https://github.com/your-username/xk6-sip-media",
  "tier": "community",
  "categories": ["Protocol", "Audio", "VoIP", "SIP"]
}
```

## 3. Pull Request Template
When you open the Pull Request on GitHub against `grafana/k6-docs`, copy and paste this into the PR description to get it merged quickly:

```markdown
### Description
Adding `xk6-sip-media` to the community extension registry. 

This extension provides production-grade SIP + RTP load testing natively in JavaScript. It replaces the need for legacy tools like SIPp by bringing multi-leg conferences, attended transfers, PESQ/E-Model MOS scoring, and SRTP directly into k6. 

**Key Technical Details:**
* **Scale:** Uses a custom CPU-sharded Go Reactor (nginx-style) bypassing the standard "1-goroutine-per-call" limit to push upwards of 100,000 concurrent UDP streams.
* **Codecs:** Implements native `tree-sitter` parsed Opt-in G.729 (GPL tag isolated), Opus 48kHz, G.722, and PCMU/A. 
* **Quality:** Built-in adaptive Jitter Buffer with Packet Loss Concealment.

### Checklist
- [x] Extension repository is public
- [x] Repository has a clear README with usage examples
- [x] Tested with the latest k6 version
- [x] JSON format in `extensions.json` is valid and matches the schema
```
