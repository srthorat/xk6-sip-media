/**
 * Baseline / Warm-up Suite
 * ========================
 * Stage 1 — Single call sanity (1 VU, 1 iteration): smoke test the target.
 * Stage 2 — Low-rate warm-up (5 VUs, 5 min): establish baseline metrics.
 *
 * Validates:
 *  - Basic SIP INVITE → 200 OK → RTP → BYE flow
 *  - MOS baseline at zero stress
 *  - Jitter and packet-loss floor
 *
 * Usage:
 *   SIP_TARGET="sip:ivr@pbx" ./k6 run scenarios/01_baseline.js
 */
import sip from 'k6/x/sip';
import { check } from 'k6';

const TARGET     = __ENV.SIP_TARGET    || 'sip:ivr@192.168.1.100';
const AUDIO_FILE = __ENV.SIP_AUDIO    || './examples/audio/sample.wav';

export const options = {
  scenarios: {
    // ── Stage 1: single-call sanity ───────────────────────────────────────
    sanity: {
      executor:    'per-vu-iterations',
      vus:         1,
      iterations:  1,
      maxDuration: '30s',
      env: { STAGE: 'sanity' },
    },
    // ── Stage 2: low-rate warm-up ──────────────────────────────────────────
    warmup: {
      executor:    'constant-vus',
      vus:         5,
      duration:    '5m',
      startTime:   '35s',       // run after sanity finishes
      env: { STAGE: 'warmup' },
    },
  },
  thresholds: {
    sip_call_success: ['count>0'],
    sip_call_failure: ['count==0'],   // zero failures in baseline
    mos_score:        ['avg>=4.0', 'min>=3.5'],
    rtp_jitter_ms:    ['avg<30', 'p(95)<80'],
    rtp_packets_lost: ['count==0'],
  },
};

export default function () {
  const result = sip.call({
    target:   TARGET,
    duration: '10s',
    audio:    { file: AUDIO_FILE },
    localIP:  '0.0.0.0',
  });

  check(result, {
    'call succeeded':    (r) => r.success,
    'no packet loss':    (r) => r.lost === 0,
    'MOS excellent':     (r) => r.mos >= 4.0,
    'jitter < 30 ms':    (r) => r.jitter < 30,
  });

  if (!result.success) {
    console.error(`[${__ENV.STAGE}] call failed: ${result.error}`);
  }
}
