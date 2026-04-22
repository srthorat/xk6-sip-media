/**
 * Scenario 25 — Echo Loopback MOS Test
 * ======================================
 * Uses audioMode='echo' to create an RTP loopback — every packet sent
 * is reflected back. Measures actual round-trip media quality (MOS,
 * jitter) through the SBC/media server under load.
 *
 * This is the closest approximation to a real voice quality test
 * without a reference PESQ file — the remote echoes our audio back
 * and we measure what came back.
 *
 * Usage:
 *   SIP_TARGET="sip:echo@pbx" ./k6 run scenarios/25_echo_loopback.js
 *
 * Note: The SBC/PBX needs to support echo mode (UAS with echo).
 *       Use xk6-sip-media UAS server (scenario 13) as the remote.
 */
import sip from 'k6/x/sip';
import { check, sleep } from 'k6';
import { Trend, Counter } from 'k6/metrics';

const TARGET   = __ENV.SIP_TARGET || 'sip:echo@192.168.1.100';
const AUDIO    = __ENV.SIP_AUDIO  || './examples/audio/sample.wav';

const loopMOS    = new Trend('echo_mos');
const loopJitter = new Trend('echo_jitter_ms');
const loopLoss   = new Trend('echo_loss_ratio');

export const options = {
  scenarios: {
    echo_loopback: {
      executor: 'ramping-vus',
      stages: [
        { duration: '30s', target: 20  },
        { duration: '3m',  target: 100 },
        { duration: '30s', target: 0   },
      ],
    },
  },
  thresholds: {
    echo_mos:       ['avg>=3.5'],
    echo_jitter_ms: ['avg<30'],          // tight jitter for loopback
    sip_call_failure: ['rate<0.01'],
  },
};

export default function () {
  const result = sip.call({
    target:    TARGET,
    duration:  '15s',
    audioMode: 'echo',    // ← reflect received RTP back (no file needed)
  });

  check(result, {
    'echo call succeeded': (r) => r && r.success,
    'packets echoed back': (r) => r && r.received > 0,
    'echo MOS >= 3.5':     (r) => r && r.mos >= 3.5,
    'jitter < 30ms':       (r) => r && r.jitter < 30,
  });

  if (result) {
    loopMOS.add(result.mos);
    loopJitter.add(result.jitter);
    loopLoss.add(result.lost / (result.sent || 1));
  }
}
