/**
 * Attended (Consultative) Transfer Load Test
 *
 * Each VU simulates a warm-transfer scenario:
 *  1. Call A (primary leg) → IVR / customer
 *  2. Put A on hold
 *  3. Call B (consultant leg) → agent
 *  4. Attended transfer: tell A to replace itself with B (REFER+Replaces)
 *  5. A gets BYE; B stays connected to the transferred party
 *  6. Hang up B after the consultation period
 *
 * Usage:
 *   SIP_LEG_A="sip:customer@pbx" SIP_LEG_B="sip:agent@pbx" \
 *     ./k6 run examples/k6/transfer_attended.js
 */
import sip from 'k6/x/sip';
import { check, sleep } from 'k6';

const LEG_A = __ENV.SIP_LEG_A || 'sip:customer@192.168.1.100';
const LEG_B = __ENV.SIP_LEG_B || 'sip:agent@192.168.1.100';

export const options = {
  scenarios: {
    attended_transfer: {
      executor:    'ramping-vus',
      startVUs:    0,
      stages: [
        { duration: '30s', target: 20 },
        { duration: '60s', target: 50 },
        { duration: '30s', target: 0  },
      ],
    },
  },
  thresholds: {
    sip_call_success:     ['count>0'],
    sip_transfer_success: ['count>0'],
    mos_score:            ['avg>=3.5'],
  },
};

export default function () {
  // ── Leg A: primary call (customer side) ──────────────────────────────────
  const legA = sip.dial({
    target:  LEG_A,
    audio:   { file: './examples/audio/sample.wav' },
    localIP: '0.0.0.0',
  });
  sleep(3); // let IVR/customer speak

  // Put customer on hold while we consult the agent
  legA.hold();

  // ── Leg B: consultant call (agent side) ───────────────────────────────────
  const legB = sip.dial({
    target:  LEG_B,
    audio:   { file: './examples/audio/sample.wav' },
    localIP: '0.0.0.0',
  });
  sleep(2); // brief consultation

  // ── Attended transfer ─────────────────────────────────────────────────────
  // Sends REFER to legA with Refer-To: <legB?Replaces=<dialog-id>>
  // legA will send INVITE(Replaces) to legB's remote party and then BYE us
  const err = legA.attendedTransfer(legB);
  check(err, { 'REFER accepted': (e) => e === null });

  // Wait for legA BYE (remote disconnects us after successful transfer)
  legA.waitDone();

  // legB is now bridged to the transferred party — hang it up after a moment
  sleep(5);
  legB.hangup();
  legB.waitDone();

  // ── Assertions ────────────────────────────────────────────────────────────
  const ra = legA.result();
  const rb = legB.result();

  check(ra, {
    'legA transfer ok':  (r) => r.transfer_ok,
    'legA MOS >= 3.0':   (r) => r.mos >= 3.0,
  });
  check(rb, {
    'legB call ended':   (r) => r.success,
    'legB MOS >= 3.0':   (r) => r.mos >= 3.0,
  });
}
