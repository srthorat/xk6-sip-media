/**
 * Concurrent Call Volume — 50 CC, 200 CC, Max Stress
 * ====================================================
 * Fixed concurrent call (CC) tests at three load levels using
 * `constant-vus` executor. Each VU holds its call open for the
 * full step duration → concurrency == VU count.
 *
 * Steps:
 *  Step 1 — 50 CC  (5 min):  normal operational load
 *  Step 2 — 200 CC (5 min):  peak capacity
 *  Step 3 — 500 CC (3 min):  max stress / saturation point
 *
 * Usage:
 *   SIP_TARGET="sip:ivr@pbx" ./k6 run scenarios/03_concurrent_calls.js
 */
import sip from 'k6/x/sip';
import { check } from 'k6';
import { Trend } from 'k6/metrics';

const TARGET     = __ENV.SIP_TARGET || 'sip:ivr@192.168.1.100';
const AUDIO_FILE = __ENV.SIP_AUDIO  || './examples/audio/sample.wav';

// Per-step MOS tracking so we can see degradation under load
const mosByStep = new Trend('mos_by_step');

export const options = {
  scenarios: {
    // ── Step 1: 50 concurrent calls ────────────────────────────────────────
    cc_50: {
      executor:  'constant-vus',
      vus:       50,
      duration:  '5m',
      env: { CALL_DURATION: '60s', STEP: '50CC' },
    },
    // ── Step 2: 200 concurrent calls ───────────────────────────────────────
    cc_200: {
      executor:  'constant-vus',
      vus:       200,
      duration:  '5m',
      startTime: '5m30s',
      env: { CALL_DURATION: '60s', STEP: '200CC' },
    },
    // ── Step 3: 500 concurrent calls (max stress) ───────────────────────────
    cc_max: {
      executor:  'constant-vus',
      vus:       500,
      duration:  '3m',
      startTime: '11m',
      env: { CALL_DURATION: '30s', STEP: '500CC' },
    },
  },
  thresholds: {
    // Strict at 50 CC; relaxed at peak
    'sip_call_failure':                   ['rate<0.01'],  // <1% overall
    'mos_score{step:50CC}':               ['avg>=4.0'],
    'mos_score{step:200CC}':              ['avg>=3.5'],
    'mos_score{step:500CC}':              ['avg>=3.0'],
    'rtp_jitter_ms{step:50CC}':           ['avg<30'],
    'rtp_jitter_ms{step:200CC}':          ['avg<60'],
    'sip_call_duration{step:50CC}':       ['p(95)<2000'],
    'sip_call_duration{step:200CC}':      ['p(95)<3000'],
  },
};

export default function () {
  const callDuration = __ENV.CALL_DURATION || '30s';
  const step         = __ENV.STEP || 'unknown';

  const result = sip.call({
    target:   TARGET,
    duration: callDuration,
    audio:    { file: AUDIO_FILE },
    localIP:  '0.0.0.0',
  });

  // Tag sample with step for per-step threshold matching
  mosByStep.add(result.mos || 0, { step });

  check(result, {
    [`[${step}] call ok`]:       (r) => r.success,
    [`[${step}] MOS >= 3.0`]:    (r) => r.mos >= 3.0,
    [`[${step}] jitter < 100ms`]:(r) => r.jitter < 100,
  });
}
