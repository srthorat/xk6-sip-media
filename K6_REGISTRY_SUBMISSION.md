# k6 Extension Registry Submission

Extensions are listed via **`grafana/k6-extension-registry`** (not `grafana/k6-docs`).
The registry drives the auto-generated extension catalogue at https://registry.k6.io and the
Grafana docs page at https://grafana.com/docs/k6/latest/extensions/explore.

---

## 1. Pre-flight checklist

Before opening the PR, verify:

- [x] Repository is public: https://github.com/srthorat/xk6-sip-media
- [x] Go module path is the full GitHub URL: `github.com/srthorat/xk6-sip-media`
- [x] At least one tagged release exists (e.g. `v0.1.0`) — the registry auto-detects versions via GitHub API
- [x] README has build instructions and usage examples
- [x] Tested against the latest k6 version (v1.7.1)
- [x] CGO requirement documented (`cgo: true` in registry entry)

---

## 2. Fork and add your entry

```bash
# 1. Fork grafana/k6-extension-registry on GitHub, then clone your fork
git clone https://github.com/<your-fork>/k6-extension-registry.git
cd k6-extension-registry

# 2. Add the entry to the END of registry.yaml
```

Append this block to `registry.yaml`:

```yaml
- module: github.com/srthorat/xk6-sip-media
  description: >-
    High-performance SIP + RTP media engine for VoIP load testing.
    Sharded CPU-parallel reactor (up to 100k concurrent streams),
    Opus/G.722/G.711 codecs, SRTP, adaptive jitter buffer with PLC,
    E-model MOS scoring, CSV credential pool, and SIP OPTIONS health-check loop.
  imports:
    - k6/x/sip
  cgo: true
  tier: community
```

---

## 3. Open the Pull Request

Target: `grafana/k6-extension-registry` → `main`

**PR title:**
```
Register xk6-sip-media — SIP/RTP VoIP load testing extension
```

**PR body:**
```markdown
## Extension

**Module:** `github.com/srthorat/xk6-sip-media`
**Repository:** https://github.com/srthorat/xk6-sip-media
**Tier:** community

## Description

Production-grade SIP + RTP load testing natively in k6 JavaScript.

Key features:
- CPU-sharded Go Reactor (nginx-style) — scales beyond "1 goroutine per call"
- Opus 48kHz (CGO), G.722, G.711 µ-law/A-law codecs
- SRTP encrypted media (AES-128-CM)
- Adaptive jitter buffer with Packet Loss Concealment
- E-model MOS scoring and RTCP statistics
- `sip.loadCSV()` — SIPp-compatible credential pool (sequential / round-robin / random)
- `sip.startHealthCheck()` — background SIP OPTIONS ping loop

## Checklist

- [x] Module path matches the GitHub repository URL
- [x] `cgo: true` set (Opus codec requires CGO)
- [x] README has build and usage instructions
- [x] Tested with k6 v1.7.1
- [x] Repository is public
```

---

## 4. After merge

Once the PR is merged:
1. The registry at https://registry.k6.io updates automatically.
2. A workflow dispatch in `grafana/k6-docs` regenerates the community extensions page.
3. The extension appears at https://grafana.com/docs/k6/latest/extensions/explore/.

---

## 5. Tag a release first

The registry auto-detects versions via the GitHub API. Create a release tag before or shortly after the PR merges:

```bash
git tag v0.1.0
git push origin v0.1.0
```

Then create a GitHub Release from that tag so the registry picks up a stable version.

