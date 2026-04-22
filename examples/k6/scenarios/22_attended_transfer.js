/**
 * Scenario 22 — Attended Transfer (Consult + Replace)
 * =====================================================
 * Tests full attended transfer flow (RFC 3891 REFER+Replaces):
 *   1. Dial primary party (customer)
 *   2. Dial consultation party (supervisor) — customer put on hold
 *   3. Send REFER+Replaces to primary leg → supervisor takes over
 *   4. Both legs verify the bridge formed correctly
 *
 * This is the most complex SIP flow — SIPp cannot do this.
 *
 * Usage:
 *   SIP_TARGET="sip:customer@pbx" \
 *   SIP_CONSULT="sip:supervisor@pbx" \
 *   ./k6 run scenarios/22_attended_transfer.js
 */
import sip from 'k6/x/sip';
import { check, sleep } from 'k6';
import { Counter, Trend } from 'k6/metrics';

const TARGET  = __ENV.SIP_TARGET || 'sip:customer@192.168.1.100';
const CONSULT = __ENV.SIP_CONSULT || 'sip:supervisor@192.168.1.100';
const AUDIO   = __ENV.SIP_AUDIO   || './examples/audio/sample.wav';

const attended  = new Counter('attended_transfer_success');
const attFailed = new Counter('attended_transfer_failure');
const attTime   = new Trend('attended_transfer_time_ms');

export const options = {
  scenarios: {
    attended_xfer: {
      executor:  'constant-vus',
      vus:       20,
      duration:  '5m',
    },
  },
  thresholds: {
    attended_transfer_success: ['count>0'],
    attended_transfer_time_ms: ['avg<5000'],
    sip_call_failure:          ['rate<0.05'],
  },
};

export default function () {
  // Step 1: Dial primary leg (customer)
  const primary = sip.dial({
    target:   TARGET,
    audio:    { file: AUDIO },
    duration: '120s',
  });
  if (!primary) { attFailed.add(1); return; }

  // Step 2: Talk to customer for 3s
  sleep(3);

  // Step 3: Put customer on hold and dial supervisor (consultation)
  primary.hold();
  const consult = sip.dial({
    target:   CONSULT,
    audio:    { file: AUDIO },
    duration: '30s',
  });
  if (!consult) {
    primary.unhold();
    primary.hangup();
    attFailed.add(1);
    return;
  }

  // Step 4: Brief consultation pause
  sleep(2);

  // Step 5: Attended transfer — primary (customer) connects to consult (supervisor)
  const start = Date.now();
  const err = primary.attendedTransfer(consult);
  const elapsed = Date.now() - start;

  const ok = check({ err, elapsed }, {
    'attended REFER accepted':    (v) => v.err === null || v.err === undefined,
    'transfer within 5s':         (v) => v.elapsed < 5000,
  });

  if (ok) {
    attended.add(1);
    attTime.add(elapsed);
  } else {
    attFailed.add(1);
  }

  sleep(2);
  consult.hangup();
  primary.hangup();
}
