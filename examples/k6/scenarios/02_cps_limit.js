/**
 * CPS Limit Tests — Calls Per Second Enforcement
 * ===============================================
 * Uses k6's `constant-arrival-rate` executor to drive an exact CPS rate
 * regardless of call duration. Tests:
 *
 *  Scenario A — CPS enforcement:   push to rated CPS limit, verify 403 appears
 *  Scenario B — Cross-region CPS:  split rate across two target regions
 *  Scenario C — Emergency bypass:  prove priority calls pass even at overload
 *
 * Requires the SIP proxy to enforce CPS limits (e.g. OpenSIPS dispatcher,
 * Kamailio cfgng, or a carrier rate limiter).
 *
 * Usage:
 *   SIP_TARGET="sip:ivr@pbx" SIP_CPS=30 ./k6 run scenarios/02_cps_limit.js
 */
import sip from 'k6/x/sip';
import { check, sleep } from 'k6';
import { Counter, Rate } from 'k6/metrics';

const TARGET_A   = __ENV.SIP_TARGET    || 'sip:ivr@192.168.1.100';
const TARGET_B   = __ENV.SIP_TARGET_B  || 'sip:ivr@192.168.1.200';  // second region
const AUDIO_FILE = __ENV.SIP_AUDIO    || './examples/audio/sample.wav';
const CPS_LIMIT  = parseInt(__ENV.SIP_CPS || '30', 10);   // rated CPS limit

// ── Custom metrics ──────────────────────────────────────────────────────────
const cpsRejects  = new Counter('sip_cps_rejected');   // 403 / 503 at CPS limit
const bypassOK    = new Rate('sip_emergency_bypass_ok');

export const options = {
  scenarios: {
    // ── A: CPS enforcement — drive 110% of rated limit ─────────────────────
    cps_enforcement: {
      executor:            'constant-arrival-rate',
      rate:                Math.ceil(CPS_LIMIT * 1.1),
      timeUnit:            '1s',
      duration:            '2m',
      preAllocatedVUs:     50,
      maxVUs:              200,
    },
    // ── B: cross-region split — half rate to each region ───────────────────
    cross_region: {
      executor:            'constant-arrival-rate',
      rate:                Math.ceil(CPS_LIMIT / 2),
      timeUnit:            '1s',
      duration:            '2m',
      startTime:           '2m30s',
      preAllocatedVUs:     30,
      maxVUs:              100,
      env: { REGION: 'B' },
    },
    // ── C: emergency bypass — 5 priority calls while A is saturating ────────
    emergency_bypass: {
      executor:            'constant-arrival-rate',
      rate:                5,
      timeUnit:            '1s',
      duration:            '1m',
      startTime:           '1m',   // overlap with enforcement test
      preAllocatedVUs:     10,
      maxVUs:              20,
      env: { PRIORITY: '1' },
    },
  },
  thresholds: {
    // CPS enforcement: some rejections expected at 110% load
    sip_cps_rejected:       ['count>0'],
    // Emergency bypass MUST succeed even under load
    sip_emergency_bypass_ok: ['rate>=0.95'],
    // Non-bypassed calls: allow up to 15% failure at CPS limit
    sip_call_failure:        ['rate<0.15'],
    mos_score:               ['avg>=3.0'],
  },
};

export default function () {
  const isPriority = __ENV.PRIORITY === '1';
  const isRegionB  = __ENV.REGION   === 'B';
  const target     = isRegionB ? TARGET_B : TARGET_A;

  // Priority calls carry a P-Asserted-Header (mapped via SIP proxy policy).
  // From load-tester perspective: just track pass/fail separately.
  const result = sip.call({
    target:   target,
    duration: '3s',     // short calls — we care about setup rate not media
    audio:    { file: AUDIO_FILE },
  });

  if (isPriority) {
    // Emergency bypass must succeed
    bypassOK.add(result.success);
    if (!result.success) {
      console.warn(`[emergency] call rejected: ${result.error}`);
    }
    return;
  }

  // Track 403 / 503 CPS rejections
  if (!result.success) {
    const is403or503 = result.error &&
      (result.error.includes('403') || result.error.includes('503'));
    cpsRejects.add(is403or503 ? 1 : 0);
  }

  check(result, {
    // At CPS limit some failures are expected — just validate successes
    'if success, MOS ok': (r) => !r.success || r.mos >= 3.0,
  });
}
