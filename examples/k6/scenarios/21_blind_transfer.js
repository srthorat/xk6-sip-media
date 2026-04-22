/**
 * Scenario 21 — Blind Transfer Load
 * ====================================
 * Dials an agent, then performs a SIP REFER blind transfer to a second
 * destination. Tests REFER processing and how quickly the SBC/PBX
 * completes the transfer under concurrent load.
 *
 * SIP flow per VU:
 *   INVITE → 200 OK → ACK → (media 3s) → REFER (blind) → 202 Accepted
 *   → wait for BYE or timeout → done
 *
 * Usage:
 *   SIP_TARGET="sip:agent@pbx" \
 *   SIP_TRANSFER_TO="sip:supervisor@pbx" \
 *   ./k6 run scenarios/21_blind_transfer.js
 */
import sip from 'k6/x/sip';
import { check, sleep } from 'k6';
import { Counter, Trend } from 'k6/metrics';

const TARGET      = __ENV.SIP_TARGET      || 'sip:agent@192.168.1.100';
const TRANSFER_TO = __ENV.SIP_TRANSFER_TO || 'sip:supervisor@192.168.1.100';
const AUDIO       = __ENV.SIP_AUDIO       || './examples/audio/sample.wav';

const transferOK   = new Counter('blind_transfer_success');
const transferFail = new Counter('blind_transfer_failure');
const transferTime = new Trend('blind_transfer_time_ms');

export const options = {
  scenarios: {
    blind_transfer: {
      executor:  'constant-vus',
      vus:       30,
      duration:  '5m',
    },
  },
  thresholds: {
    blind_transfer_success: ['count>0'],
    blind_transfer_time_ms: ['avg<2000'],   // REFER must complete within 2s
    sip_call_failure:       ['rate<0.05'],
  },
};

export default function () {
  const call = sip.dial({
    target:   TARGET,
    audio:    { file: AUDIO },
    duration: '60s',
  });

  if (!call) { transferFail.add(1); return; }

  // Talk for 3 seconds before transfer
  sleep(3);

  const start = Date.now();
  const err = call.blindTransfer(TRANSFER_TO);
  const elapsed = Date.now() - start;

  const ok = check({ err, elapsed }, {
    'REFER accepted (202)':    (v) => v.err === null || v.err === undefined,
    'transfer within 2000ms':  (v) => v.elapsed < 2000,
  });

  if (ok) {
    transferOK.add(1);
    transferTime.add(elapsed);
  } else {
    transferFail.add(1);
  }

  // Wait a moment for the transfer to complete, then clean up
  sleep(2);
  call.hangup();
}
