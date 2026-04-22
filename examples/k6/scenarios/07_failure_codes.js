/**
 * SIP Failure Code Tests at Load
 * ================================
 * Validates SIP proxy / PBX error handling under simultaneous load.
 * Intentionally triggers:
 *
 *  Scenario A — 403 CPS:    drive past rate limit, verify 403 Forbidden  
 *  Scenario B — 486 Busy:   target a destination known to return Busy Here
 *  Scenario C — 503 Overload: target overloaded proxy, verify graceful drain
 *  Scenario D — Mixed codes: random routing across A/B/C destinations
 *
 * NOTE: destinations producing 486 / 503 must be configured in your PBX.
 *   Asterisk: CONGESTION → 486; SIP/Trunk max-channels → 503
 *   FreeSWITCH: respond_503_on_busy=true in profile
 *
 * Usage:
 *   SIP_NORMAL="sip:ivr@pbx"    \
 *   SIP_BUSY="sip:busy@pbx"     \
 *   SIP_OVERLOAD="sip:ov@pbx"   \
 *   ./k6 run scenarios/07_failure_codes.js
 */
import sip from 'k6/x/sip';
import { check } from 'k6';
import { Counter, Rate } from 'k6/metrics';

const NORMAL    = __ENV.SIP_NORMAL   || 'sip:ivr@192.168.1.100';
const BUSY      = __ENV.SIP_BUSY     || 'sip:busy@192.168.1.100';
const OVERLOAD  = __ENV.SIP_OVERLOAD || 'sip:overload@192.168.1.100';
const AUDIO     = __ENV.SIP_AUDIO    || './examples/audio/sample.wav';

// ── Custom counters per error class ────────────────────────────────────────
const err403 = new Counter('sip_error_403');
const err486 = new Counter('sip_error_486');
const err503 = new Counter('sip_error_503');
const errOth = new Counter('sip_error_other');
const successRate = new Rate('sip_success_rate');

function classifyError(errStr) {
  if (!errStr) return;
  if (errStr.includes('403')) { err403.add(1); return; }
  if (errStr.includes('486')) { err486.add(1); return; }
  if (errStr.includes('503')) { err503.add(1); return; }
  errOth.add(1);
}

export const options = {
  scenarios: {
    // ── A: 403 via CPS limit — drive 2× CPS limit ──────────────────────────
    drive_403: {
      executor:         'constant-arrival-rate',
      rate:             60,               // 2× typical 30-CPS limit
      timeUnit:         '1s',
      duration:         '2m',
      preAllocatedVUs:  80,
      maxVUs:           200,
      env: { TARGET_TYPE: 'normal' },
    },
    // ── B: 486 Busy Here ────────────────────────────────────────────────────
    drive_486: {
      executor:         'constant-vus',
      vus:              30,
      duration:         '2m',
      startTime:        '2m30s',
      env: { TARGET_TYPE: 'busy' },
    },
    // ── C: 503 Service Unavailable ──────────────────────────────────────────
    drive_503: {
      executor:         'constant-arrival-rate',
      rate:             20,
      timeUnit:         '1s',
      duration:         '2m',
      startTime:        '5m',
      preAllocatedVUs:  40,
      maxVUs:           100,
      env: { TARGET_TYPE: 'overload' },
    },
    // ── D: mixed destinations ───────────────────────────────────────────────
    mixed_codes: {
      executor:         'constant-vus',
      vus:              50,
      duration:         '3m',
      startTime:        '7m30s',
      env: { TARGET_TYPE: 'mixed' },
    },
  },
  thresholds: {
    // We EXPECT failures in this test — validate they are the right kind
    sip_error_403: ['count>0'],     // must see 403s during CPS push
    sip_error_486: ['count>0'],     // must see 486s from busy target
    sip_error_503: ['count>0'],     // must see 503s from overload target
    // Successful calls must still have good quality
    'mos_score':           ['avg>=3.5'],
    'sip_call_duration':   ['p(95)<3000'],
  },
};

export default function () {
  const type = __ENV.TARGET_TYPE || 'normal';
  let target;
  if (type === 'busy')     target = BUSY;
  else if (type === 'overload') target = OVERLOAD;
  else if (type === 'mixed') {
    const r = Math.random();
    target = r < 0.6 ? NORMAL : r < 0.8 ? BUSY : OVERLOAD;
  } else {
    target = NORMAL;
  }

  const result = sip.call({
    target:   target,
    duration: '5s',
    audio:    { file: AUDIO },
  });

  successRate.add(result.success ? 1 : 0);

  if (!result.success) {
    classifyError(result.error);
  }

  check(result, {
    'error classified': (r) => r.success || !!r.error,
    'if ok, MOS >= 3.5': (r) => !r.success || r.mos >= 3.5,
  });
}
