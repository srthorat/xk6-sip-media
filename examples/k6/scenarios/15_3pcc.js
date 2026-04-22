/**
 * Scenario 15 — 3PCC (Third Party Call Control) Load Test
 * =========================================================
 * Dials two parties simultaneously from a single k6 VU, simulating
 * click-to-dial and call-recording server scenarios.
 *
 * Usage:
 *   SIP_PARTY_A="sip:alice@pbx" SIP_PARTY_B="sip:bob@pbx" \
 *     ./k6 run scenarios/15_3pcc.js
 *
 * Scenario flow:
 *   1. VU dials Party A (INVITE 1)
 *   2. VU dials Party B (INVITE 2) — both legs established
 *   3. A ←── RTP media ──→ k6 ←── RTP media ──→ B (controller in path)
 *   4. After duration, both legs hung up
 */
import sip from 'k6/x/sip';
import { check } from 'k6';

const PARTY_A  = __ENV.SIP_PARTY_A || 'sip:alice@192.168.1.100';
const PARTY_B  = __ENV.SIP_PARTY_B || 'sip:bob@192.168.1.100';
const AUDIO    = __ENV.SIP_AUDIO   || './examples/audio/sample.wav';

export const options = {
  scenarios: {
    threepcc_load: {
      executor:  'constant-vus',
      vus:       20,
      duration:  '5m',
    },
  },
  thresholds: {
    sip_call_success: ['count>0'],
    sip_call_failure: ['rate<0.02'],
  },
};

export default function () {
  const session = sip.dial3pcc({
    partyA:   PARTY_A,
    partyB:   PARTY_B,
    audioA:   AUDIO,
    audioB:   AUDIO,
    duration: '30s',
  });

  if (!session) {
    return;
  }

  const resultsA = session.legA.result();
  const resultsB = session.legB.result();

  check(resultsA, { '3PCC leg A ok': (r) => r && r.success });
  check(resultsB, { '3PCC leg B ok': (r) => r && r.success });

  session.hangupAll();
}
