# Example Test Trace

This file tracks end-to-end execution status for the example scripts under `examples/`.

## Legend

- `pass` — executed successfully in the current environment
- `fail` — executed and failed
- `not tested` — not executed in the current environment yet

## Current Validation Basis

- Environment: local custom `k6` binary in this workspace
- Date: 2026-04-18 to 2026-05-01
- Known working external target: Vonage edge using the current credentials configured in the Vonage smoke scripts
- Audio used in validated Vonage scripts: `examples/audio/hard.wav`, `examples/audio/simplest-short.wav`

## Suggested Validation Order

| Step | Coverage                  | Script / Path                      | Status | Notes                                                                                                          |
|------|---------------------------|------------------------------------|--------|----------------------------------------------------------------------------------------------------------------|
| 1    | Register only             | `examples/k6/register_only.js`     | pass   | Validated in `SIP_SMOKE=1` mode against the current Vonage registrar; register and unregister both passed on 2026-04-19 |
| 2    | Register and call         | `examples/k6/register_call.js`     | pass   | Validated in `SIP_SMOKE=1` mode against the current Vonage register, authenticated call, and unregister flow on 2026-04-19 |
| 3    | Direct call + 407/auth path | `examples/k6/vonage_direct_call.js` | pass   | Validated without prior REGISTER against the current Vonage INVITE proxy-auth path on 2026-04-19             |
| 4    | IVR flow                  | `examples/k6/vonage_ivr_flow.js`   | pass   | Current known-good IVR path against Vonage                                                                     |

## Top-Level k6 Examples

| Script                                         | Status     | Notes                                                                                       |
|-----------------------------------------------|------------|---------------------------------------------------------------------------------------------|
| `examples/k6/call.js`                         | pass       | 1 VU, 20 s, MOS 4.43, 0 loss; SIP_TARGET/USERNAME/PASSWORD env vars                         |
| `examples/k6/ivr_flow.js`                     | pass       | 1 VU, 30 s, MOS 4.43, 0 loss, DTMF 1+3; all thresholds passed                               |
| `examples/k6/register_only.js`                | pass       | Register and unregister passed on 2026-04-19                                                |
| `examples/k6/register_call.js`                | pass       | Register, call 443361, unregister passed on 2026-04-19                                      |
| `examples/k6/vonage_direct_call.js`           | pass       | Direct call without REGISTER; proxy-auth 2026-04-19                                         |
| `examples/k6/vonage_ivr_flow.js`              | pass       | Dials 443362, DTMF 2, BYE; passed on 2026-04-19                                             |
| `examples/k6/vonage_single_call.js`           | pass       | 1 call, 20 s, MOS > 3.8; registers in `setup()`                                             |
| `examples/k6/vonage_two_call.js`              | pass       | 2 concurrent calls, 20 s, MOS > 3.8                                                         |
| `examples/k6/vonage_ten_call.js`              | pass       | 10 concurrent calls, 20 s, MOS > 3.8                                                        |

## Advanced Scenarios

> Conference, transfer-initiation, and 3PCC scenarios removed — out of scope.

## Scenario Scripts

| Script                              | Status     | Notes                                         |
|-------------------------------------|------------|-----------------------------------------------|
| `examples/k6/scenarios/01_baseline.js` | pass       | 1 VU, 10 s, MOS 4.42, 0 loss, jitter 4.8 ms; all thresholds green      |
| `examples/k6/scenarios/02_cps_limit.js` | not tested | Load scenario; target-specific                                 |
| `examples/k6/scenarios/03_concurrent_calls.js` | not tested | Load scenario; target-specific                                 |
| `examples/k6/scenarios/04_ramp_spike.js` | not tested | Load scenario; target-specific                                 |
| `examples/k6/scenarios/05_soak.js`       | not tested | Long-running scenario                                          |
| `examples/k6/scenarios/06_long_duration.js` | not tested | Long-duration call scenario                                    |
| `examples/k6/scenarios/07_failure_codes.js` | not tested | Requires target that returns expected SIP failures             |
| `examples/k6/scenarios/08_auth_security.js` | not tested | Auth/security scenario depends on target configuration         |
| `examples/k6/scenarios/09_inbound_load.js` | not tested | Requires inbound SIP reachability/server mode                  |
| `examples/k6/scenarios/10_gcs_routing.js` | not tested | Routing-specific scenario                                      |
| `examples/k6/scenarios/11_tcp_transport.js` | not tested | Requires TCP SIP target                                        |
| `examples/k6/scenarios/12_tls_transport.js` | not tested | Requires TLS SIP target/certs                                  |
| `examples/k6/scenarios/13_uas_server.js` | not tested | Requires inbound/server-side test setup                        |
| `examples/k6/scenarios/14_pcap_replay.js` | not tested | Requires compatible PCAP input and target                      |
| `examples/k6/scenarios/16_variable_extraction.js` | not tested | Target-specific SIP header/body behavior                       |
| `examples/k6/scenarios/17_srtp_encrypted.js` | not tested | Requires SRTP-compatible target                                |
| `examples/k6/scenarios/18_rtcp_quality.js` | not tested | Requires RTCP-capable target for meaningful validation         |
| `examples/k6/scenarios/19_early_media.js` | pass       | 1 VU, 20 s, MOS ok, earlyMedia=true; Vonage answers 200 OK    |
| `examples/k6/scenarios/20_hold_unhold.js` | pass       | 1 VU; hold+unhold re-INVITEs accepted by Vonage, BYE clean    |
| `examples/k6/scenarios/24_dtmf_ivr.js`    | pass       | 1 VU, 2 DTMF digits, avg resp 2.1 s; thresholds green         |
| `examples/k6/scenarios/25_echo_loopback.js` | not tested | Requires echo endpoint or compatible loopback target           |
| `examples/k6/scenarios/26_g722_wideband.js` | not tested | Requires G.722-capable target                                 |
| `examples/k6/scenarios/27_mp3_audio.js`   | not tested | Requires target reachable for MP3-backed call                  |
| `examples/k6/scenarios/28_proxy_auth_407.js` | pass       | 1 VU; REGISTER 407 challenge handled, 200 OK received         |
| `examples/k6/scenarios/29_cancel_mid_ring.js` | pass       | 3 iterations; Vonage answers before 2 s, call runs 3 s then BYE |
| `examples/k6/scenarios/30_options_ping.js` | not tested | Vonage carrier SBC does not respond to OPTIONS; use self-hosted |
| `examples/k6/scenarios/31_opus_webrtc.js` | not tested | Requires Opus/WebRTC-compatible target                        |
| `examples/k6/scenarios/32_g729_carrier.js` | not tested | Requires G.729-capable carrier target                         |
| `examples/k6/scenarios/33_multi_user_csv.js` | pass       | 5 VUs × 30 s, 15 iters, 100% checks, MOS 4.43, 0 loss         |
| `examples/k6/scenarios/34_multi_user_csv_ramp.js` | pass    | 5 VU smoke: 21 iters, 100% checks, MOS 4.43, 0 loss           |
| `examples/k6/scenarios/35_cancel_load.js`  | not tested | New — CANCEL mid-INVITE load; requires slow-answer SIP target  |

## Notes

- No example is currently marked `fail` because only the Vonage smoke scripts were executed to completion after fixes.
- If a script is run and fails, update this file with `fail` and a short reason.
- Keep `vonage_single_call.js` as the baseline known-good example and record scaled runs separately.
- For multiple independent outbound calls, use `vonage_single_call.js` / `vonage_ten_call.js` or `33_multi_user_csv.js` (CSV credential pool) or `34_multi_user_csv_ramp.js` (ramping VUs).
- Testable against Vonage QA today (no special infra): `01_baseline`, `19_early_media`, `20_hold_unhold`, `24_dtmf_ivr`, `28_proxy_auth_407`, `29_cancel_mid_ring`, `33_multi_user_csv`, `34_multi_user_csv_ramp`, `call.js`, `ivr_flow.js`.
- Scenarios requiring special infra: TCP/TLS targets (11, 12), SRTP target (17), RTCP target (18), inbound/UAS reachability (09, 13), echo endpoint (25), codec-specific targets (26 G.722, 27 MP3, 31 Opus, 32 G.729), multi-carrier routing (10), PCAP replay (14), long-running time budget (05, 06).
- Suggested test order for Vonage QA: `01_baseline → 24_dtmf_ivr → 20_hold_unhold → 29_cancel_mid_ring → 28_proxy_auth_407 → 19_early_media → 33_multi_user_csv → 34_multi_user_csv_ramp`.
- `30_options_ping` is N/A for carrier SBCs (no response to unsolicited OPTIONS from unregistered IPs); valid against self-hosted SBC/PBX.
- At 50 VUs with 5 CSV credentials, each credential handles 10 concurrent calls; some `407 / context deadline / Timer_B` failures are expected at the edge of Vonage QA capacity — not extension bugs.