/**
 * Scenario 29 — CANCEL Mid-Ring (Action on Ring)
 * ===============================================
 * Simulates a massively common real-world occurrence: a caller dialing,
 * hearing ringing, but hanging up before the callee answers.
 * 
 * By injecting `cancelAfter: '2s'`, xk6-sip-media will gracefully send
 * a SIP CANCEL directly into the active INVITE transaction during the 
 * ringing phase, ensuring PBXs/SBCs correctly drop the call without crashing.
 *
 * Usage:
 *   SIP_TARGET="sip:delay_answer@pbx" ./k6 run scenarios/29_cancel_mid_ring.js
 */
import sip from 'k6/x/sip';
import { check } from 'k6';

const TARGET    = __ENV.SIP_TARGET    || 'sip:192.168.1.100';
const USERNAME  = __ENV.SIP_USERNAME  || '';
const PASSWORD  = __ENV.SIP_PASSWORD  || '';
const AOR       = __ENV.SIP_AOR       || '';

export const options = {
  vus: 50,
  duration: '1m',
  thresholds: {
    sip_call_success: ['count>0'],
  },
};

export default function () {
  // Start the call, but explicitly instruct the engine to abort/CANCEL exactly
  // 2 seconds after the INVITE was sent.
  const result = sip.call({
    target:      TARGET,
    cancelAfter: '2s',   // sends CANCEL if still ringing after 2s
    duration:    '3s',   // cap call duration if answered before cancel fires
    ...(USERNAME && { username: USERNAME }),
    ...(PASSWORD && { password: PASSWORD }),
    ...(AOR      && { aor: AOR }),
  });

  // CANCEL returns success=true with empty stats (no media exchanged),
  // or success=true with normal stats if the call was answered before 2s elapsed.
  check(result, {
    'cancel handled gracefully': (r) => r !== undefined && r.error === undefined,
  });
}
