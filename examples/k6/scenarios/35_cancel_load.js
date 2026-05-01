/**
 * Scenario 35 — CANCEL Mid-Ring Load Test
 * ========================================
 * Stress-tests a SIP target's CANCEL handling under concurrent load.
 * Each VU sends an INVITE, waits CANCEL_AFTER seconds during the ringing
 * phase, then fires CANCEL. Validates the SBC/PBX correctly handles
 * 487 Request Terminated without leaking dialogs or crashing under load.
 *
 * Key metrics:
 *   sip_call_success — every CANCEL must be handled gracefully (no error)
 *   sip_call_failure — any unexpected errors
 *   sip_call_duration — should be close to CANCEL_AFTER (not full call)
 *
 * Environment variables:
 *   SIP_TARGET     SIP URI to call             (default: sip:192.168.1.100)
 *   SIP_USERNAME   Auth username               (optional)
 *   SIP_PASSWORD   Auth password               (optional)
 *   VUS            Number of concurrent VUs    (default: 20)
 *   DURATION       Test duration               (default: 1m)
 *   CANCEL_AFTER   Delay before CANCEL (s)     (default: 2s)
 *
 * Usage:
 *   SIP_TARGET="sip:pbx.example.com" \
 *   VUS=50 DURATION=2m CANCEL_AFTER=3s \
 *   ./k6 run examples/k6/scenarios/35_cancel_load.js
 */
import sip from 'k6/x/sip';
import { check, sleep } from 'k6';

const TARGET       = __ENV.SIP_TARGET    || 'sip:192.168.1.100';
const USERNAME     = __ENV.SIP_USERNAME  || '';
const PASSWORD     = __ENV.SIP_PASSWORD  || '';
const VUS          = parseInt(__ENV.VUS          || '20');
const DURATION     = __ENV.DURATION              || '1m';
const CANCEL_AFTER = __ENV.CANCEL_AFTER          || '2s';

export const options = {
  vus:      VUS,
  duration: DURATION,

  thresholds: {
    // Every CANCEL must be handled without an error
    'checks{check:cancel handled gracefully}': ['rate==1'],
    // No unexpected failures
    sip_call_failure: ['count==0'],
    // Call duration should be short (cancelled before full duration)
    // Allow up to cancelAfter + 5s for signaling overhead
    sip_call_duration: ['p(95)<7000'],
  },
};

export default function () {
  const result = sip.call({
    target:      TARGET,
    cancelAfter: CANCEL_AFTER, // CANCEL fires after this delay if still ringing
    duration:    '5s',         // cap if answered before CANCEL fires
    ...(USERNAME && { username: USERNAME }),
    ...(PASSWORD && { password: PASSWORD }),
  });

  // A graceful CANCEL returns success=true (or no error field).
  // If the target answers before CANCEL fires, the call completes normally —
  // that is also acceptable.
  check(result, {
    'cancel handled gracefully': (r) => r !== undefined && !r.error,
  });

  // Small pause between iterations so VUs don't hammer the target continuously
  sleep(0.5);
}
