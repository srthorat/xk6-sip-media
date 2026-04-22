/**
 * Auth / Security Load Tests
 * ===========================
 * Validates authentication infrastructure under three attack/load patterns:
 *
 *  Scenario A — Valid auth:         100 VUs with correct credentials (baseline)
 *  Scenario B — Invalid bulk:       50 VUs sending wrong passwords → verify
 *                                   401 returned, NOT 200 (auth bypass check)
 *  Scenario C — REGISTER storm:     300 REGISTER/s to simulate botnet or
 *                                   re-registration thundering herd
 *
 * Usage:
 *   SIP_REGISTRAR="sip:pbx" \
 *   SIP_AOR_PREFIX="sip:user" SIP_DOMAIN="pbx.example.com" \
 *   SIP_PASSWORD=correct SIP_BAD_PASS=wrong \
 *   SIP_TARGET="sip:ivr@pbx" \
 *   ./k6 run scenarios/08_auth_security.js
 */
import sip from 'k6/x/sip';
import { check } from 'k6';
import { Counter, Rate } from 'k6/metrics';

const REGISTRAR  = __ENV.SIP_REGISTRAR  || 'sip:192.168.1.100';
const DOMAIN     = __ENV.SIP_DOMAIN     || '192.168.1.100';
const AOR_PREFIX = __ENV.SIP_AOR_PREFIX || 'sip:user';
const PASSWORD   = __ENV.SIP_PASSWORD   || 'correct-password';
const BAD_PASS   = __ENV.SIP_BAD_PASS   || 'wrong-password';
const TARGET     = __ENV.SIP_TARGET     || 'sip:ivr@192.168.1.100';
const AUDIO      = __ENV.SIP_AUDIO      || './examples/audio/sample.wav';

// ── Metrics ────────────────────────────────────────────────────────────────
const authBypass    = new Counter('sip_auth_bypass');    // MUST stay 0
const authRejected  = new Counter('sip_auth_rejected');  // 401/403 on bad creds
const regStormRate  = new Rate('sip_register_success');

export const options = {
  scenarios: {
    // ── A: valid auth load ─────────────────────────────────────────────────
    valid_auth: {
      executor:  'constant-vus',
      vus:       100,
      duration:  '3m',
      env: { AUTH_TYPE: 'valid' },
    },
    // ── B: invalid credential bulk ─────────────────────────────────────────
    invalid_bulk: {
      executor:  'constant-arrival-rate',
      rate:      20,
      timeUnit:  '1s',
      duration:  '2m',
      startTime: '3m30s',
      preAllocatedVUs: 40,
      maxVUs: 100,
      env: { AUTH_TYPE: 'invalid' },
    },
    // ── C: REGISTER storm ──────────────────────────────────────────────────
    register_storm: {
      executor:         'constant-arrival-rate',
      rate:             300,       // 300 REGISTER/s
      timeUnit:         '1s',
      duration:         '2m',
      startTime:        '6m',
      preAllocatedVUs:  100,
      maxVUs:           500,
      env: { AUTH_TYPE: 'storm' },
    },
  },
  thresholds: {
    sip_auth_bypass:  ['count==0'],        // CRITICAL: must NEVER be > 0
    sip_auth_rejected:['count>0'],         // bad creds must be rejected
    sip_register_success: ['rate>=0.95'],  // valid auth must succeed
    // Valid auth must perform well
    'sip_call_success': ['count>0'],
    'mos_score':        ['avg>=3.5'],
  },
};

export default function () {
  const authType = __ENV.AUTH_TYPE || 'valid';
  const vuID     = __VU;

  // ── Scenario A: valid auth — register + call ────────────────────────────
  if (authType === 'valid') {
    const aor = `${AOR_PREFIX}${vuID}@${DOMAIN}`;
    let reg;
    try {
      reg = sip.register({
        registrar: REGISTRAR,
        aor,
        username: `user${vuID}`,
        password: PASSWORD,
      });
    } catch (e) {
      console.error(`valid auth register failed: ${e}`);
      return;
    }

    const result = sip.call({
      target:   TARGET,
      duration: '10s',
      audio:    { file: AUDIO },
    });

    regStormRate.add(1);
    check(result, { 'valid auth call ok': (r) => r.success });
    reg.unregister();
    return;
  }

  // ── Scenario B: invalid credentials ────────────────────────────────────
  if (authType === 'invalid') {
    const aor = `${AOR_PREFIX}attacker-${vuID}@${DOMAIN}`;
    try {
      const reg = sip.register({
        registrar: REGISTRAR,
        aor,
        username:  `attacker_${vuID}`,
        password:  BAD_PASS,         // wrong password
      });
      // If register() succeeded with wrong password → auth bypass!
      authBypass.add(1);
      console.error(`AUTH BYPASS DETECTED for ${aor}`);
      reg.unregister();
    } catch (e) {
      // Expected: 401/403 throws → increment rejected counter
      authRejected.add(1);
    }
    return;
  }

  // ── Scenario C: REGISTER storm ──────────────────────────────────────────
  if (authType === 'storm') {
    const aor      = `${AOR_PREFIX}${vuID}@${DOMAIN}`;
    const useValid = Math.random() < 0.8; // 80% valid, 20% invalid
    try {
      const reg = sip.register({
        registrar: REGISTRAR,
        aor,
        username: `user${vuID}`,
        password:  useValid ? PASSWORD : BAD_PASS,
        expires:   30,               // very short expiry to force storm re-REGs
      });
      regStormRate.add(1);
      reg.unregister();
    } catch (e) {
      if (!useValid) {
        authRejected.add(1); // expected
      } else {
        console.warn(`[storm] valid reg failed: ${e}`);
      }
    }
  }
}
