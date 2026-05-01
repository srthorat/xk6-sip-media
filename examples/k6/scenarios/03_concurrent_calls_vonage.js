/**
 * Concurrent Call Volume — Vonage carrier edition
 * ================================================
 * Three steps with Vonage digest auth:
 *   Step 1 —  3 CC  (2 min): warm-up
 *   Step 2 —  6 CC  (2 min): moderate load
 *   Step 3 — 10 CC  (3 min): peak / stress
 *
 * Usage:
 *   set -a && source .env && set +a
 *   ./k6 run examples/k6/scenarios/03_concurrent_calls_vonage.js
 */
import sip from 'k6/x/sip';
import { check } from 'k6';
import { Trend } from 'k6/metrics';

const USERNAME = __ENV.VONAGE_USERNAME;
const DOMAIN   = __ENV.VONAGE_DOMAIN;
const PASSWORD = __ENV.VONAGE_PASSWORD;
const CALLEE   = `sip:${__ENV.VONAGE_CALLEE}@${DOMAIN}`;
const AOR      = `sip:${USERNAME}@${DOMAIN}`;
const AUDIO    = 'examples/audio/hard.wav';

const mosByStep = new Trend('mos_by_step');

export const options = {
  scenarios: {
    // ── Step 1: 3 concurrent calls ──────────────────────────────────────────
    cc_3: {
      executor: 'constant-vus',
      vus:      3,
      duration: '2m',
      env: { CALL_DURATION: '30s', STEP: '3CC' },
    },
    // ── Step 2: 6 concurrent calls ──────────────────────────────────────────
    cc_6: {
      executor:  'constant-vus',
      vus:       6,
      duration:  '2m',
      startTime: '2m30s',
      env: { CALL_DURATION: '30s', STEP: '6CC' },
    },
    // ── Step 3: 10 concurrent calls (peak) ──────────────────────────────────
    cc_10: {
      executor:  'constant-vus',
      vus:       10,
      duration:  '3m',
      startTime: '5m',
      env: { CALL_DURATION: '30s', STEP: '10CC' },
    },
  },
  thresholds: {
    'sip_call_failure':              ['rate<0.02'],
    'mos_by_step{step:3CC}':         ['avg>=4.0'],
    'mos_by_step{step:6CC}':         ['avg>=4.0'],
    'mos_by_step{step:10CC}':        ['avg>=3.8'],
    'rtp_jitter_ms':                 ['avg<50'],
    'sip_call_duration':             ['p(95)<2500'],
  },
};

export default function () {
  const callDuration = __ENV.CALL_DURATION || '30s';
  const step         = __ENV.STEP || 'unknown';

  const result = sip.call({
    target:   CALLEE,
    aor:      AOR,
    username: USERNAME,
    password: PASSWORD,
    duration: callDuration,
    rtcp:     true,
    audio: {
      file:  AUDIO,
      codec: 'PCMU',
    },
  });

  mosByStep.add(result.mos || 0, { step });

  check(result, {
    [`[${step}] call ok`]:        (r) => r.success,
    [`[${step}] MOS >= 3.5`]:     (r) => (r.mos || 0) >= 3.5,
    [`[${step}] jitter < 100ms`]: (r) => (r.jitter || 0) < 100,
    [`[${step}] loss < 5%`]:      (r) => (r.loss_pct || 0) < 5,
  });
}
