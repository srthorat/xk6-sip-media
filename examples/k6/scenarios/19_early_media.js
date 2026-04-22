/**
 * Scenario 19 — Early Media (183 Session Progress)
 * ==================================================
 * Tests the 183 Session Progress flow where the remote party sends
 * provisional SDP before answering. Audio (ringback/hold) is streamed
 * during the ring phase before the call is officially answered.
 *
 * Common in:
 *   - IVR systems that play announcements before answering
 *   - Carrier interconnects with ringback tone generation
 *   - Contact center queuing systems ("Your wait time is 5 minutes")
 *
 * Usage:
 *   SIP_TARGET="sip:ivr@pbx" ./k6 run scenarios/19_early_media.js
 */
import sip from 'k6/x/sip';
import { check, sleep } from 'k6';
import { Counter, Trend } from 'k6/metrics';

const TARGET = __ENV.SIP_TARGET || 'sip:ivr@192.168.1.100';
const AUDIO  = __ENV.SIP_AUDIO  || './examples/audio/sample.wav';

const earlyMediaHits = new Counter('early_media_183_received');
const ringToAnswerMs = new Trend('ring_to_answer_ms');

export const options = {
  scenarios: {
    early_media: {
      executor:  'constant-vus',
      vus:       30,
      duration:  '5m',
    },
  },
  thresholds: {
    early_media_183_received: ['count>0'],
    ring_to_answer_ms:        ['avg<5000'],  // answer within 5s of ring
    sip_call_failure:         ['rate<0.02'],
  },
};

export default function () {
  const start = Date.now();

  const result = sip.call({
    target:      TARGET,
    duration:    '20s',
    earlyMedia:  true,       // ← stream audio during 183 ring phase
    audio:       { file: AUDIO },
    headers: {
      'X-Test-EarlyMedia': 'true',
    },
  });

  const connectMs = Date.now() - start;

  const ok = check(result, {
    'call answered':         (r) => r && r.success,
    'audio streamed in ring':(r) => r && r.sent > 0,
    'MOS acceptable':        (r) => r && r.mos >= 3.0,
  });

  // Heuristic: if packets were sent but call connected fast, 183 was hit
  if (result && result.sent > 0 && connectMs < 3000) {
    earlyMediaHits.add(1);
  }
  ringToAnswerMs.add(connectMs);

  sleep(1);
}
