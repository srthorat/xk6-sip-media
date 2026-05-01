/**
 * Scenario 34 — Multi-User Ramp: CSV credential pool + ramping VUs
 * =================================================================
 * Same CSV credential pool as scenario 33, but uses a ramping-vus executor
 * instead of constant-vus.  Ramps from 0 → target VUs, holds for a soak
 * period, then ramps back down.
 *
 * Environment variables:
 *   CREDS_CSV      path to CSV credential file  (default: examples/csv/vonage_users.csv)
 *   CALL_DURATION  duration of each call        (default: 20s)
 *   RAMP_TARGET    peak concurrent VUs          (default: 50)
 *   RAMP_UP        ramp-up duration             (default: 2m)
 *   HOLD           hold at peak duration        (default: 3m)
 *   RAMP_DOWN      ramp-down duration           (default: 1m)
 *   AUDIO          path to WAV file             (default: examples/audio/simplest-short.wav)
 *   CODEC          codec name                   (default: PCMU)
 *
 * Usage:
 *   CREDS_CSV=examples/csv/vonage_users.csv \
 *   CALL_DURATION=20s RAMP_TARGET=50 \
 *   ./k6 run examples/k6/scenarios/34_multi_user_csv_ramp.js
 */
import sip from 'k6/x/sip';
import { check } from 'k6';
import { Trend } from 'k6/metrics';

const CSV_PATH  = __ENV.CREDS_CSV      || 'examples/csv/vonage_users.csv';
const CALL_DUR  = __ENV.CALL_DURATION  || '20s';
const PEAK_VUS  = parseInt(__ENV.RAMP_TARGET || '50');
const RAMP_UP   = __ENV.RAMP_UP        || '2m';
const HOLD      = __ENV.HOLD           || '3m';
const RAMP_DOWN = __ENV.RAMP_DOWN      || '1m';
const AUDIO     = __ENV.AUDIO          || 'examples/audio/simplest-short.wav';
const CODEC     = __ENV.CODEC          || 'PCMU';

const pool = sip.loadCSV(CSV_PATH);

console.log(`[init] ${pool.len()} credential rows | peak ${PEAK_VUS} VUs`);
console.log(`[init] ramp ${RAMP_UP} → hold ${HOLD} → down ${RAMP_DOWN}`);

export const options = {
  scenarios: {
    ramp_to_peak: {
      executor: 'ramping-vus',
      startVUs: 0,
      stages: [
        { duration: RAMP_UP,   target: PEAK_VUS },
        { duration: HOLD,      target: PEAK_VUS },
        { duration: RAMP_DOWN, target: 0        },
      ],
      gracefulRampDown: '30s',
    },
  },
  thresholds: {
    'sip_call_failure': ['rate<0.05'],   // <5% failures allowed during ramp
    'mos_score':        ['avg>=3.5'],
    'rtp_jitter_ms':    ['avg<100'],
  },
};

const mosTrend = new Trend('mos_by_user', true);

export default function () {
  const creds  = pool.pick(__VU);
  const domain = creds.domain;
  const aor    = creds.aor || `sip:${creds.username}@${domain}`;
  const callee = creds.callee || __ENV.CALLEE;

  if (!callee) {
    console.error('[VU] No callee in CSV and CALLEE env var not set');
    return;
  }

  const target = callee.startsWith('sip:') ? callee : `sip:${callee}@${domain}`;

  const result = sip.call({
    target,
    aor,
    username: creds.username,
    password: creds.password,
    duration: CALL_DUR,
    rtcp:     true,
    audio: {
      file:  AUDIO,
      codec: CODEC,
    },
  });

  check(result, {
    'call succeeded':   (r) => r.success === true,
    'MOS >= 3.5':       (r) => (r.mos || 0) >= 3.5,
    'packet loss < 5%': (r) => (r.lost / Math.max(r.sent, 1)) < 0.05,
  });

  mosTrend.add(result.mos || 0, { user: creds.username });

  if (!result.success) {
    console.warn(`[VU${__VU}] call failed: ${result.error}`);
  }
}
