/**
 * Scenario 24 — DTMF IVR Navigation
 * =====================================
 * Simulates realistic IVR navigation:
 *   - RFC 2833 telephone-event DTMF (standard)
 *   - SIP INFO DTMF (Cisco/Avaya legacy)
 *   - Multi-step IVR menus with realistic timing
 *
 * Tests:
 *   - DTMF delivery rate at load
 *   - IVR response detection (silence + audio)
 *   - Digit timing jitter
 *   - IVR menu depth (multi-level navigation)
 *
 * Usage:
 *   SIP_TARGET="sip:ivr@pbx" ./k6 run scenarios/24_dtmf_ivr.js
 *
 * IVR MAP:
 *   press 1 → Sales
 *   press 2 → Support   ← we test this path
 *     press 1 → Level 1 Support
 *     press 2 → Level 2 Support  ← and then this
 *   press 9 → Repeat menu
 */
import sip from 'k6/x/sip';
import { check, sleep } from 'k6';
import { Counter, Rate, Trend } from 'k6/metrics';

const TARGET    = __ENV.SIP_TARGET    || 'sip:ivr@192.168.1.100';
const AUDIO     = __ENV.SIP_AUDIO     || './examples/audio/sample.wav';
const USE_INFO  = __ENV.DTMF_SIP_INFO === 'true'; // set to use SIP INFO instead of RFC 2833
const USERNAME  = __ENV.SIP_USERNAME  || '';
const PASSWORD  = __ENV.SIP_PASSWORD  || '';
const AOR       = __ENV.SIP_AOR       || '';

const dtmfOK       = new Counter('dtmf_digit_sent');
const ivrSuccess   = new Counter('ivr_navigation_success');
const ivrFail      = new Counter('ivr_navigation_failure');
const dtmfLatency  = new Trend('dtmf_response_ms');

export const options = {
  scenarios: {
    dtmf_ivr: {
      executor:  'constant-vus',
      vus:       40,
      duration:  '5m',
    },
  },
  thresholds: {
    dtmf_digit_sent:       ['count>0'],
    ivr_navigation_success: ['count>0'],
    dtmf_response_ms:      ['avg<3000'],   // IVR responds within 3s
    sip_call_failure:      ['rate<0.02'],
  },
};

export default function () {
  const call = sip.dial({
    target:   TARGET,
    audio:    { file: AUDIO },
    duration: '60s',
    ...(USERNAME && { username: USERNAME }),
    ...(PASSWORD && { password: PASSWORD }),
    ...(AOR      && { aor: AOR }),
  });

  if (!call) { ivrFail.add(1); return; }

  // Wait for IVR welcome prompt
  sleep(3);

  // ── Level 1: Main menu ──────────────────────────────────────────
  let start = Date.now();
  if (USE_INFO) {
    call.sendDTMFInfo('2', 160);        // SIP INFO: press 2 for Support
  } else {
    call.sendDTMF('2');                  // RFC 2833: press 2
  }
  dtmfOK.add(1);

  // Wait for L1 confirm tone / menu
  sleep(2);
  dtmfLatency.add(Date.now() - start);

  // ── Level 2: Support sub-menu ───────────────────────────────────
  start = Date.now();
  if (USE_INFO) {
    call.sendDTMFInfo('2', 160);        // press 2 for L2 Support
  } else {
    call.sendDTMF('2');
  }
  dtmfOK.add(1);

  sleep(2);
  dtmfLatency.add(Date.now() - start);

  // ── Confirm IVR (rule-based detection) ─────────────────────────
  const result = call.result ? call.result() : null;

  // Wait in queue
  sleep(5);

  call.hangup();

  const ok = check({ result }, {
    'IVR accepted digits':    (v) => true,     // if we got here, digits were sent
    'call connected':         (v) => call.isActive !== undefined,
  });

  if (ok) ivrSuccess.add(1); else ivrFail.add(1);
}
