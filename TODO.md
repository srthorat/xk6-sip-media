# TODO & Roadmap

## Pre-Release Validation
- [ ] **Infrastructure Baseline:** Run a massive load test against a real PBX/SBC (Asterisk, FreeSWITCH, Cisco CUCM, or Twilio) using `04_ramp_spike.js` or `03_concurrent_calls.js`.
- [ ] **Performance Validation:** Verify the target infrastructure can handle 1,000+ concurrent calls with SRTP enabled without crashing.
- [ ] **Quality Telemetry Check:** Verify that Grafana properly displays E-model MOS and RTCP jitter under heavy load.

## Publication
- [ ] **k6 Registry Submission:** Open a PR against `grafana/k6-docs` to list on the official [k6 Extensions Directory](https://k6.io/docs/extensions/).

## Future Roadmap
- [ ] **WebSocket Transport (WSS) & WebRTC:** SIP over WebSockets (RFC 7118) for browser-driven contact centre infrastructure testing.

