/**
 * TCP Transport Load Test
 * =======================
 * Runs standard SIP call load over TCP transport (no encryption).
 *
 * TCP is preferred over UDP when:
 *  - SIP messages exceed 1300 bytes (INVITE with large SDP / headers)
 *  - The carrier / SBC requires persistent connections
 *  - Network path has high jitter (UDP retransmits become costly)
 *
 * Usage:
 *   SIP_TARGET="sip:ivr@pbx" ./k6 run scenarios/11_tcp_transport.js
 */
import sip from 'k6/x/sip';
import { check } from 'k6';

const TARGET     = __ENV.SIP_TARGET || 'sip:ivr@192.168.1.100';
const AUDIO_FILE = __ENV.SIP_AUDIO  || './examples/audio/sample.wav';

export const options = {
  scenarios: {
    tcp_warmup: {
      executor:  'constant-vus',
      vus:       10,
      duration:  '2m',
      env: { STEP: 'warmup' },
    },
    tcp_load: {
      executor:  'constant-vus',
      vus:       100,
      duration:  '5m',
      startTime: '2m30s',
      env: { STEP: 'load' },
    },
    tcp_peak: {
      executor:  'constant-vus',
      vus:       300,
      duration:  '3m',
      startTime: '8m',
      env: { STEP: 'peak' },
    },
  },
  thresholds: {
    sip_call_success:  ['count>0'],
    sip_call_failure:  ['rate<0.01'],
    mos_score:         ['avg>=3.8'],
    rtp_jitter_ms:     ['avg<30'],
    sip_call_duration: ['p(95)<2000'],
  },
};

export default function () {
  const result = sip.call({
    target:    TARGET,
    transport: 'tcp',          // ← TCP signaling, plain RTP media
    duration:  '15s',
    audio:     { file: AUDIO_FILE },
  });

  check(result, {
    'TCP call ok':    (r) => r.success,
    'MOS >= 3.8':     (r) => r.mos >= 3.8,
    'loss < 1%':      (r) => r.lost / Math.max(r.sent, 1) < 0.01,
  });
}
