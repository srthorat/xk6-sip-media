/**
 * Blind Transfer Load Test
 *
 * Each VU:
 *  1. Dials an IVR/queue
 *  2. Waits for answer, sends optional DTMF
 *  3. Blind-transfers the call to an agent queue
 *  4. Waits for BYE (remote ends leg after transfer)
 *  5. Reports MOS + transfer_ok
 *
 * Usage:
 *   SIP_TARGET="sip:ivr@pbx" SIP_TRANSFER="sip:agents@pbx" \
 *     ./k6 run examples/k6/transfer_blind.js
 */
import sip from 'k6/x/sip';
import { check, sleep } from 'k6';

const TARGET   = __ENV.SIP_TARGET   || 'sip:ivr@192.168.1.100';
const TRANSFER = __ENV.SIP_TRANSFER || 'sip:agents@192.168.1.100';

export const options = {
  scenarios: {
    blind_transfer: {
      executor:    'ramping-vus',
      startVUs:    0,
      stages: [
        { duration: '30s', target: 50  },
        { duration: '60s', target: 100 },
        { duration: '30s', target: 0   },
      ],
    },
  },
  thresholds: {
    sip_call_success:    ['count>0'],
    sip_transfer_success: ['count>0'],
    mos_score:           ['avg>=3.5'],
    rtp_jitter_ms:       ['avg<50'],
  },
};

export default function () {
  // Dial into the IVR — returns immediately after ACK
  const call = sip.dial({
    target:  TARGET,
    audio:   { file: './examples/audio/sample.wav' },
    localIP: '0.0.0.0',
  });

  // Let IVR greet the caller
  sleep(3);

  // Navigate IVR if needed
  call.sendDtmf('1');
  sleep(1);

  // Blind transfer to agent queue
  const err = call.blindTransfer(TRANSFER);
  check(err, { 'REFER accepted': (e) => e === null });

  // Wait for the remote to BYE us after transfer completes
  call.waitDone();

  const r = call.result();
  check(r, {
    'call succeeded':    (r) => r.success,
    'transfer ok':       (r) => r.transfer_ok,
    'MOS >= 3.5':        (r) => r.mos >= 3.5,
    'packet loss < 5%':  (r) => r.lost / Math.max(r.sent, 1) < 0.05,
  });
}
