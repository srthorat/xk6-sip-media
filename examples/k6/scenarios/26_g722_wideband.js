/**
 * Scenario 26 — G.722 Wideband (HD Voice)
 * ==========================================
 * Tests G.722 16kHz wideband codec under load.
 * G.722 delivers 7kHz audio bandwidth (vs 3.4kHz for G.711) and is
 * used by Cisco, Polycom, and most modern IP phones for HD voice calls.
 *
 * SDP offer: m=audio <port> RTP/AVP 9   (PT=9 = G.722)
 * Expected:  SBC/PBX accepts and negotiate G.722 in answer
 *
 * Usage:
 *   SIP_TARGET="sip:hd@pbx" \
 *   SIP_AUDIO=./examples/audio/sample_hd.wav \
 *   ./k6 run scenarios/26_g722_wideband.js
 *
 * Generate HD audio:
 *   cd examples/audio && bash generate_sample.sh
 *   (creates sample_hd.wav at 16kHz mono)
 */
import sip from 'k6/x/sip';
import { check, sleep } from 'k6';
import { Counter, Trend } from 'k6/metrics';

const TARGET = __ENV.SIP_TARGET || 'sip:ivr@192.168.1.100';
const AUDIO  = __ENV.SIP_AUDIO  || './examples/audio/sample_hd.wav';

const g722OK  = new Counter('g722_call_success');
const g722MOS = new Trend('g722_mos');

export const options = {
  scenarios: {
    g722_load: {
      executor:  'constant-vus',
      vus:       30,
      duration:  '5m',
    },
  },
  thresholds: {
    g722_call_success: ['count>0'],
    g722_mos:          ['avg>=3.5'],  // G.722 should score higher than G.711
    sip_call_failure:  ['rate<0.02'],
  },
};

export default function () {
  const result = sip.call({
    target:   TARGET,
    duration: '20s',
    audio: {
      file:  AUDIO,
      codec: 'G722',     // ← G.722 16kHz wideband
    },
  });

  const ok = check(result, {
    'G.722 call succeeded':     (r) => r && r.success,
    'G.722 MOS >= 3.5':         (r) => r && r.mos >= 3.5,
    'G.722 packets delivered':  (r) => r && r.sent > 0,
    'no negotiation failure':   (r) => r && !r.error,
  });

  if (ok) {
    g722OK.add(1);
    g722MOS.add(result.mos);
  }

  sleep(1);
}
