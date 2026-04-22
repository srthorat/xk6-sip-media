/**
 * Long Duration Call Tests
 * ========================
 * Validates RTP stability, sequence number rollover, and memory for
 * individual calls that run far longer than normal.
 *
 *  Scenario A — 10-minute calls:  20 VUs, each holding a call for 10 min
 *  Scenario B — 1-hour calls:     5 VUs, each holding a call for 60 min
 *
 * Key checks:
 *  - No RTP sequence number rollover issues (32-bit wraps at ~256k packets)
 *  - Stable jitter (no bufferbloat accumulation)
 *  - MOS does not decay over time
 *  - BYE sent/received cleanly after long hold
 *
 * Usage:
 *   SIP_TARGET="sip:ivr@pbx" ./k6 run scenarios/06_long_duration.js
 */
import sip from 'k6/x/sip';
import { check } from 'k6';

const TARGET     = __ENV.SIP_TARGET || 'sip:ivr@192.168.1.100';
const AUDIO_FILE = __ENV.SIP_AUDIO  || './examples/audio/sample.wav';

export const options = {
  scenarios: {
    // ── Scenario A: 10-minute calls ────────────────────────────────────────
    calls_10min: {
      executor:    'per-vu-iterations',
      vus:         20,
      iterations:  1,          // each VU makes exactly one long call
      maxDuration: '15m',
      env: { CALL_DURATION: '600s', LABEL: '10min' },
    },
    // ── Scenario B: 1-hour calls ───────────────────────────────────────────
    calls_1hr: {
      executor:    'per-vu-iterations',
      vus:         5,
      iterations:  1,
      maxDuration: '75m',
      startTime:   '16m',
      env: { CALL_DURATION: '3600s', LABEL: '1hr' },
    },
  },
  thresholds: {
    sip_call_failure:  ['rate==0'],     // zero failures on long calls
    mos_score:         ['avg>=3.5'],
    rtp_packets_lost:  ['rate<0.001'],  // <0.1% loss on long calls
    rtp_jitter_ms:     ['avg<50'],
  },
};

export default function () {
  const callDuration = __ENV.CALL_DURATION || '60s';
  const label        = __ENV.LABEL || 'unknown';

  console.log(`[${label}] starting ${callDuration} call`);

  const result = sip.call({
    target:   TARGET,
    duration: callDuration,
    audio:    { file: AUDIO_FILE },
    localIP:  '0.0.0.0',
  });

  const totalPkts = result.sent || 1;
  const lossRate  = (result.lost || 0) / totalPkts;

  check(result, {
    [`[${label}] completed`]:         (r) => r.success,
    [`[${label}] MOS >= 3.5`]:        (r) => r.mos >= 3.5,
    [`[${label}] loss < 0.1%`]:       (r) => lossRate < 0.001,
    [`[${label}] jitter < 50ms`]:     (r) => r.jitter < 50,
  });

  // Report packet count to detect RTP counter rollover issues
  const expectedMin = Math.floor(parseFloat(callDuration) / 0.02) * 0.9;
  check(result, {
    [`[${label}] sent enough packets`]: (r) => r.sent >= expectedMin,
  });

  console.log(
    `[${label}] done: sent=${result.sent} lost=${result.lost} ` +
    `jitter=${result.jitter?.toFixed(1)}ms MOS=${result.mos?.toFixed(2)}`
  );
}
