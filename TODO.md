# TODO & Pre-Release Checklist

This document tracks final action items before publishing the `xk6-sip-media` extension to the open source community, as well as the future roadmap.

## 1. Pre-Release Testing
- [ ] **Infrastructure Baseline:** Run a massive load test against a real PBX/SBC (e.g., Asterisk, FreeSWITCH, Cisco CUCM, or Twilio) using `04_ramp_spike.js` or `03_concurrent_calls.js`.
- [ ] **Performance Validation:** Verify the target infrastructure can handle 1,000+ Concurrent Calls with SRTP enabled without crashing.
- [ ] **Quality Telemetry Check:** Verify that Grafana properly displays the E-model MOS and RTCP Jitter under heavy load.

## 2. CI/CD Automation
- [ ] **GitHub Actions Setup:** Create `.github/workflows/build.yml` to automatically run `go fmt`, `go test`, and build the `xk6` binary on every push to `main` branch.

## 3. Publication & Open Source
- [ ] **Git Initialization:** Run `git init`, `git add .`, `git commit` to finalize repo state.
- [ ] **GitHub Push:** Push the repository to the primary GitHub account (e.g., `github.com/USER/xk6-sip-media`).
- [ ] **k6 Registry Submission:** Open a Pull Request against `grafana/k6-docs` to get listed on the official [k6 Extensions Directory](https://k6.io/docs/extensions/).

## 4. Future Roadmap (Post-Release Expansion)

### Protocols & Features
- [ ] **WebSocket Transport (WSS) & WebRTC:** Build support for SIP over WebSockets (RFC 7118) to strictly test browser-driven contact center infrastructure.
- [ ] **Opus Codec:** Add native Opus encoding/decoding for Discord/WebRTC-style HD ultra-wideband voice testing.
- [ ] **G.729 Codec:** Add support for the compressed G.729 codec (common in legacy PBXs and international carrier trunks).

### Advanced Media Handling
- [ ] **Adaptive Jitter Buffers:** Refactor the `Receive()` RTP loop to intelligently slow down and buffer packets identically to a Polycom/Cisco hardware phone, rather than passively dumping missing packets.
- [ ] **Dynamic SDP Mapping:** Parse dynamic `a=rtpmap` Payload Types (PT) negotiated by modern PBXs rather than strictly assuming static payload codes like `PT=0` for PCMU.

### Performance & Memory Tuning (The "100k Concurrent" Goal)
- [ ] **Goroutine Heavy-weighting Refactor:** Currently, every active call spins up multiple Goroutines (`Sender`, `Receiver`, `RTCP`). For 5,000 calls, this is fine. To hit 100,000 concurrent encrypted calls on a single node without hitting Go scheduler thread limits, we need to transition from "1 goroutine per call" to a highly multiplexed single-loop event reactor (similar to NGINX/Redis).
