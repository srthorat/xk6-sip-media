# Example Test Trace

This file tracks end-to-end execution status for the example scripts under `examples/`.

## Legend

- `pass` — executed successfully in the current environment
- `fail` — executed and failed
- `not tested` — not executed in the current environment yet

## Current Validation Basis

- Environment: local custom `k6` binary in this workspace
- Date: 2026-04-18 to 2026-04-19
- Known working external target: Vonage edge using the current credentials configured in the Vonage smoke scripts
- Audio used in validated Vonage scripts: `examples/audio/hard.wav`

## Suggested Validation Order

| Step | Coverage                  | Script / Path                      | Status | Notes                                                                                                          |
|------|---------------------------|------------------------------------|--------|----------------------------------------------------------------------------------------------------------------|
| 1    | Register only             | `examples/k6/register_only.js`     | pass   | Validated in `SIP_SMOKE=1` mode against the current Vonage registrar; register and unregister both passed on 2026-04-19 |
| 2    | Register and call         | `examples/k6/register_call.js`     | pass   | Validated in `SIP_SMOKE=1` mode against the current Vonage register, authenticated call, and unregister flow on 2026-04-19 |
| 3    | Direct call + 407/auth path | `examples/k6/vonage_direct_call.js` | pass   | Validated without prior REGISTER against the current Vonage INVITE proxy-auth path on 2026-04-19             |
| 4    | IVR flow                  | `examples/k6/vonage_ivr_flow.js`   | pass   | Current known-good IVR path against Vonage                                                                     |

## Top-Level k6 Examples

| Script                             | Status     | Notes |
|------------------------------------|------------|-------|
| `examples/k6/call.js`              | not tested | Generic unauthenticated call example; current Vonage validation used dedicated auth-aware scripts instead |
| `examples/k6/ivr_flow.js`          | not tested | Generic unauthenticated IVR flow (`1`, `3`) with `ivr_ok` check; current Vonage validation used `vonage_ivr_flow.js` instead |
| `examples/k6/register_only.js`     | pass       | Validated in `SIP_SMOKE=1` mode against Vonage: register and unregister both passed on 2026-04-19 |
| `examples/k6/register_call.js`     | pass       | Validated in `SIP_SMOKE=1` mode against Vonage: register, authenticated call to 443361, and unregister all passed on 2026-04-19 |
| `examples/k6/vonage_direct_call.js`| pass       | Direct call to 443361 without REGISTER; INVITE proxy-auth path validated on 2026-04-19 |
| `examples/k6/vonage_ivr_flow.js`   | pass       | Current edited variant dials 443362, waits 3 s, sends DTMF 2, waits 20 s, then BYE; rerun completed successfully on 2026-04-19 |
| `examples/k6/vonage_single_call.js`| pass       | 1 call, 20s, `hard.wav`, RTP TX/RX checks, packet loss check, MOS > 3.8; registers first in `setup()` |
| `examples/k6/vonage_two_call.js`   | pass       | 2 concurrent calls, 20s each, `hard.wav`, RTP TX/RX checks, packet loss check, MOS > 3.8 |
| `examples/k6/vonage_ten_call.js`   | pass       | 10 concurrent calls, 20s each, RTP TX/RX checks, packet loss check, MOS > 3.8 |

## Advanced Scenarios

| Script                                    | Status     | Notes                                                      |
|-------------------------------------------|------------|------------------------------------------------------------|
| `examples/k6/conference.js`               | not tested | Conference-capable SIP target and room/bridge URI required |
| `examples/k6/transfer_blind.js`           | not tested | Requires transfer support on target platform               |
| `examples/k6/transfer_attended.js`        | not tested | Requires transfer support and multiple call legs           |
| `examples/k6/scenarios/23_conference.js`  | not tested | Conference bridge support required                         |
| `examples/k6/scenarios/21_blind_transfer.js` | not tested | Requires transfer support                               |
| `examples/k6/scenarios/22_attended_transfer.js` | not tested | Requires attended transfer and Replaces support      |

## Scenario Scripts

| Script                                       | Status     | Notes                                                  |
|----------------------------------------------|------------|--------------------------------------------------------|
| `examples/k6/scenarios/01_baseline.js`       | not tested | Likely requires configured SIP target                  |
| `examples/k6/scenarios/02_cps_limit.js`      | not tested | Load scenario; target-specific                         |
| `examples/k6/scenarios/03_concurrent_calls.js` | not tested | Load scenario; target-specific                       |
| `examples/k6/scenarios/04_ramp_spike.js`     | not tested | Load scenario; target-specific                         |
| `examples/k6/scenarios/05_soak.js`           | not tested | Long-running scenario                                  |
| `examples/k6/scenarios/06_long_duration.js`  | not tested | Long-duration call scenario                            |
| `examples/k6/scenarios/07_failure_codes.js`  | not tested | Requires target that returns expected SIP failures     |
| `examples/k6/scenarios/08_auth_security.js`  | not tested | Auth/security scenario depends on target configuration |
| `examples/k6/scenarios/09_inbound_load.js`   | not tested | Requires inbound SIP reachability/server mode          |
| `examples/k6/scenarios/10_gcs_routing.js`    | not tested | Routing-specific scenario                              |
| `examples/k6/scenarios/11_tcp_transport.js`  | not tested | Requires TCP SIP target                                |
| `examples/k6/scenarios/12_tls_transport.js`  | not tested | Requires TLS SIP target/certs                          |
| `examples/k6/scenarios/13_uas_server.js`     | not tested | Requires inbound/server-side test setup                |
| `examples/k6/scenarios/14_pcap_replay.js`    | not tested | Requires compatible PCAP input and target              |
| `examples/k6/scenarios/15_3pcc.js`           | not tested | Requires multi-leg 3PCC-capable setup                  |
| `examples/k6/scenarios/16_variable_extraction.js` | not tested | Target-specific SIP header/body behavior         |
| `examples/k6/scenarios/17_srtp_encrypted.js` | not tested | Requires SRTP-compatible target                        |
| `examples/k6/scenarios/18_rtcp_quality.js`   | not tested | Requires RTCP-capable target for meaningful validation |
| `examples/k6/scenarios/19_early_media.js`    | not tested | Requires 183 early media support                       |
| `examples/k6/scenarios/20_hold_unhold.js`    | not tested | Requires re-INVITE hold/unhold support                 |
| `examples/k6/scenarios/24_dtmf_ivr.js`       | not tested | Requires IVR with known DTMF flow                      |
| `examples/k6/scenarios/25_echo_loopback.js`  | not tested | Requires echo endpoint or compatible loopback target   |
| `examples/k6/scenarios/26_g722_wideband.js`  | not tested | Requires G.722-capable target                          |
| `examples/k6/scenarios/27_mp3_audio.js`      | not tested | Requires target reachable for MP3-backed call          |
| `examples/k6/scenarios/28_proxy_auth_407.js` | not tested | Requires proxy-auth challenge path                     |
| `examples/k6/scenarios/29_cancel_mid_ring.js` | not tested | Requires ringing target / CANCEL path                |
| `examples/k6/scenarios/30_options_ping.js`   | not tested | Requires reachable SIP OPTIONS target                  |
| `examples/k6/scenarios/31_opus_webrtc.js`    | not tested | Requires Opus/WebRTC-compatible target                 |
| `examples/k6/scenarios/32_g729_carrier.js`   | not tested | Requires G.729-capable carrier target                  |

## Notes

- No example is currently marked `fail` because only the Vonage smoke scripts were executed to completion after fixes.
- If a script is run and fails, update this file with `fail` and a short reason.
- Keep `vonage_single_call.js` as the baseline known-good example and record scaled runs separately.
- Conference and transfer examples are tracked as advanced scenarios because they are not variants of the validated Vonage independent-call load scripts.
- If the goal is multiple independent outbound calls, use the Vonage single/two/ten-call scripts instead of the advanced scenarios.