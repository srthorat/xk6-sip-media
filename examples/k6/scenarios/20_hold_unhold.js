/**
 * Scenario 20 — Hold / Unhold Under Load
 * ========================================
 * Simulates agent-style call handling: answer → hold → unhold → hang up.
 * Verifies that re-INVITE hold/unhold works correctly under concurrent load
 * (each VU holds and resumes once mid-call).
 *
 * SIP flow per VU:
 *   INVITE → 200 OK → ACK → (media 5s) → re-INVITE a=inactive (hold)
 *   → (silence 3s) → re-INVITE a=sendrecv (unhold) → (media 5s) → BYE
 *
 * Usage:
 *   SIP_TARGET="sip:agent@pbx" ./k6 run scenarios/20_hold_unhold.js
 */
import sip from 'k6/x/sip';
import { check, sleep } from 'k6';
import { Counter, Trend } from 'k6/metrics';

const TARGET    = __ENV.SIP_TARGET    || 'sip:agent@192.168.1.100';
const AUDIO     = __ENV.SIP_AUDIO     || './examples/audio/sample.wav';
const USERNAME  = __ENV.SIP_USERNAME  || '';
const PASSWORD  = __ENV.SIP_PASSWORD  || '';
const AOR       = __ENV.SIP_AOR       || '';

const holdSuccess   = new Counter('hold_success');
const unholdSuccess = new Counter('unhold_success');
const holdDuration  = new Trend('hold_duration_ms');

export const options = {
  scenarios: {
    hold_unhold: {
      executor:  'constant-vus',
      vus:       50,
      duration:  '5m',
    },
  },
  thresholds: {
    hold_success:     ['count>0'],
    unhold_success:   ['count>0'],
    sip_call_failure: ['rate<0.02'],
    // mos_score not tracked here — sip.dial() does not auto-emit MOS metrics;
    // use sip.call() or call.result() if MOS is required.
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

  if (!call) return;

  // Active media phase: 5 seconds
  sleep(5);

  // Put call on hold (re-INVITE with a=inactive)
  const holdStart = Date.now();
  const holdErr = call.hold();
  check({ holdErr }, { 'hold sent': (v) => v.holdErr === null || v.holdErr === undefined });
  holdSuccess.add(holdErr ? 0 : 1);

  // Simulate agent consulting another call: 3 seconds of hold
  sleep(3);
  holdDuration.add(Date.now() - holdStart);

  // Resume the call (re-INVITE with a=sendrecv)
  const unholdErr = call.unhold();
  check({ unholdErr }, { 'unhold sent': (v) => v.unholdErr === null || v.unholdErr === undefined });
  unholdSuccess.add(unholdErr ? 0 : 1);

  // Continue talking for 5 more seconds
  sleep(5);

  const result = call.hangup();
  check(result, { 'call ended cleanly': (r) => r === null || r === undefined });
}
