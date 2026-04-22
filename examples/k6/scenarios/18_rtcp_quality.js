/**
 * Scenario 18 — RTCP Quality Reporting
 * ======================================
 * Enables RTCP sender/receiver reports alongside RTP media.
 * RTCP adds SR every 5s (packet count, NTP timestamp) and RR
 * (fraction lost, jitter, LSR/DLSR for RTT measurement).
 *
 * Useful for validating RTCP pass-through on SBCs and media
 * gateways, and for obtaining standard ITU-T G.107 metrics
 * that RTCP reports provide.
 *
 * Usage:
 *   SIP_TARGET="sip:ivr@pbx" ./k6 run scenarios/18_rtcp_quality.js
 */
import sip from 'k6/x/sip';
import { check, sleep } from 'k6';
import { Trend, Counter } from 'k6/metrics';

const TARGET   = __ENV.SIP_TARGET || 'sip:ivr@192.168.1.100';
const AUDIO    = __ENV.SIP_AUDIO  || './examples/audio/sample.wav';

const rtcpMOS    = new Trend('rtcp_mos');
const rtcpJitter = new Trend('rtcp_jitter_ms');
const rtcpLoss   = new Counter('rtcp_packets_lost');

export const options = {
  scenarios: {
    rtcp_quality: {
      executor:  'constant-vus',
      vus:       20,
      duration:  '5m',
    },
  },
  thresholds: {
    rtcp_mos:        ['avg>=3.5'],
    rtcp_jitter_ms:  ['avg<50'],
    sip_call_failure: ['rate<0.01'],
  },
};

export default function () {
  const result = sip.call({
    target:   TARGET,
    duration: '30s',
    rtcp:     true,         // ← enable RTCP SR + RR on port rtpPort+1
    audio:    { file: AUDIO },
  });

  check(result, {
    'call succeeded':  (r) => r && r.success,
    'MOS acceptable':  (r) => r && r.mos >= 3.5,
    'jitter < 50ms':   (r) => r && r.jitter < 50,
  });

  if (result) {
    rtcpMOS.add(result.mos);
    rtcpJitter.add(result.jitter);
    rtcpLoss.add(result.lost);
  }

  sleep(1);
}
