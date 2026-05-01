/**
 * Scenario 33 — Multi-User Load: CSV credential pool
 * ====================================================
 * Mirrors SIPp's -inf CSV injection pattern.
 *
 * Each VU registers using its own SIP credentials loaded from a CSV file.
 * All VUs target the same callee (e.g. a toll-free IVR / contact centre number).
 * The pool distributes credentials round-robin by VU id (SEQUENTIAL mode)
 * so VU 1 → row 0, VU 2 → row 1, etc. — wrapping when VUs > rows.
 *
 * CSV file format (see examples/csv/vonage_users.csv):
 *   SEQUENTIAL
 *   username,password,domain,callee
 *   alice,pass1,sip.example.com,18005551234
 *   bob,pass2,sip.example.com,18005551234
 *
 * Environment variables:
 *   CREDS_CSV   path to the CSV credential file (required)
 *   CALL_DURATION  duration of each call (default: 30s)
 *   VUS         number of concurrent virtual users (default: 10)
 *   DURATION    scenario run duration (default: 5m)
 *
 * Usage (Vonage QA, 10 concurrent users, 5-minute run):
 *   CREDS_CSV=examples/csv/vonage_users.csv \
 *   ./k6 run examples/k6/scenarios/33_multi_user_csv.js
 *
 * Usage (100-user contact-centre simulation):
 *   CREDS_CSV=examples/csv/cc_agents.csv VUS=100 DURATION=10m \
 *   ./k6 run examples/k6/scenarios/33_multi_user_csv.js
 */
import sip from 'k6/x/sip';
import { check } from 'k6';
import { Trend } from 'k6/metrics';

// ── Load credential pool at init time (once, shared across all VUs) ─────────
const CSV_PATH     = __ENV.CREDS_CSV    || 'examples/csv/vonage_users.csv';
const CALL_DUR     = __ENV.CALL_DURATION || '30s';
const VUS          = parseInt(__ENV.VUS   || '10');
const DURATION     = __ENV.DURATION      || '5m';
const AUDIO        = __ENV.AUDIO         || 'examples/audio/hard.wav';
const CODEC        = __ENV.CODEC         || 'PCMU';

// Derive a call-duration threshold: parse CALL_DURATION to ms and add 5 s overhead.
function callDurMs(durStr) {
  const m = durStr.match(/^(\d+)(ms|s|m)?$/i);
  if (!m) return 60000;
  const n = parseInt(m[1]);
  switch ((m[2] || 's').toLowerCase()) {
    case 'ms': return n + 5000;
    case 'm':  return n * 60000 + 5000;
    default:   return n * 1000 + 5000;
  }
}
const CALL_DUR_THRESHOLD = callDurMs(CALL_DUR);

const pool = sip.loadCSV(CSV_PATH);

console.log(`[init] Loaded ${pool.len()} credential rows from ${CSV_PATH}`);
console.log(`[init] Mode: ${VUS} VUs × ${DURATION}, call duration ${CALL_DUR}`);
console.log(`[init] CSV rows cover ${pool.len()} distinct SIP users`);
console.log(`[init] VU-to-row mapping: VU N → row (N-1) % ${pool.len()} (SEQUENTIAL)`);

// ── Options ─────────────────────────────────────────────────────────────────
export const options = {
  scenarios: {
    multi_user: {
      executor:  'constant-vus',
      vus:       VUS,
      duration:  DURATION,
    },
  },
  thresholds: {
    'sip_call_failure':  ['rate<0.02'],   // <2% failures
    'mos_score':         ['avg>=3.8'],
    'rtp_jitter_ms':     ['avg<50'],
    'sip_call_duration': [`p(95)<${CALL_DUR_THRESHOLD}`],  // call duration + 5 s signaling overhead
  },
};

const mosTrend = new Trend('mos_by_user', true);

// ── setup: register all users before VUs start calling ─────────────────────
export function setup() {
  const results = { registered: 0, failed: 0 };
  for (let i = 1; i <= pool.len(); i++) {
    const creds = pool.pick(i);
    const domain    = creds.domain;
    const aor       = creds.aor || `sip:${creds.username}@${domain}`;
    const registrar = `sip:${domain}`;
    try {
      sip.register({
        registrar,
        aor,
        username: creds.username,
        password: creds.password,
        expires:  180,
      });
      results.registered++;
    } catch (e) {
      console.warn(`[setup] Registration failed for ${creds.username}: ${e}`);
      results.failed++;
    }
  }
  console.log(`[setup] Registered ${results.registered}/${pool.len()} users (${results.failed} failed)`);
  return results;
}

// ── default: each VU picks its credential row and makes calls ────────────────
export default function (data) {
  if (data.failed > 0 && data.registered === 0) {
    console.warn('[VU] All registrations failed — skipping');
    return;
  }

  // pick() distributes by VU id — VU 1 → row 0, VU 2 → row 1, …
  const creds  = pool.pick(__VU);
  const domain = creds.domain;
  const aor    = creds.aor    || `sip:${creds.username}@${domain}`;
  const callee = creds.callee || __ENV.CALLEE;

  if (!callee) {
    console.error('[VU] No callee defined — set the callee column in CSV or CALLEE env var');
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

  // Tag MOS with the username so per-user quality is visible in Grafana
  mosTrend.add(result.mos || 0, { user: creds.username });

  check(result, {
    'call succeeded':       (r) => r.success === true,
    'sent RTP packets':     (r) => r.sent > 0,
    'received RTP packets': (r) => r.received > 0,
    'packet loss < 5%':     (r) => (r.loss_pct || 0) < 5,
    'mos > 3.8':            (r) => (r.mos || 0) > 3.8,
  });
}
