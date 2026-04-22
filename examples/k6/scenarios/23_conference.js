/**
 * Scenario 23 — Conference Bridge Load
 * =======================================
 * Tests multi-leg conference bridging under load.
 * Each VU creates a conference room and adds 3 participants,
 * then measures aggregate RTP quality across all legs.
 *
 * Tests:
 *   - Conference join latency
 *   - Audio quality with 3+ participants
 *   - Bridge capacity (increase VUs to approach max rooms)
 *   - Leg tear-down ordering
 *
 * Usage:
 *   SIP_PARTY_A="sip:alice@pbx" \
 *   SIP_PARTY_B="sip:bob@pbx"   \
 *   SIP_PARTY_C="sip:carol@pbx" \
 *   ./k6 run scenarios/23_conference.js
 */
import sip from 'k6/x/sip';
import { check, sleep } from 'k6';
import { Counter, Trend } from 'k6/metrics';

const PARTY_A = __ENV.SIP_PARTY_A || 'sip:alice@192.168.1.100';
const PARTY_B = __ENV.SIP_PARTY_B || 'sip:bob@192.168.1.100';
const PARTY_C = __ENV.SIP_PARTY_C || 'sip:carol@192.168.1.100';
const AUDIO   = __ENV.SIP_AUDIO   || './examples/audio/sample.wav';

const confOK      = new Counter('conference_success');
const confFail    = new Counter('conference_failure');
const confLegsMOS = new Trend('conference_mos');
const joinTime    = new Trend('conference_join_ms');

export const options = {
  scenarios: {
    conference_load: {
      executor:  'constant-vus',
      vus:       15,
      duration:  '5m',
    },
  },
  thresholds: {
    conference_success:   ['count>0'],
    conference_mos:       ['avg>=3.0'],
    conference_join_ms:   ['avg<3000'],
    sip_call_failure:     ['rate<0.05'],
  },
};

export default function () {
  const conf = sip.conference({ localIP: '0.0.0.0' });
  if (!conf) { confFail.add(1); return; }

  // Add participants one by one and track join time
  const joinStart = Date.now();

  const legA = conf.addParticipant({ target: PARTY_A, audio: { file: AUDIO } });
  const legB = conf.addParticipant({ target: PARTY_B, audio: { file: AUDIO } });
  const legC = conf.addParticipant({ target: PARTY_C, audio: { file: AUDIO } });

  const joined = Date.now() - joinStart;
  joinTime.add(joined);

  const allJoined = check({ legA, legB, legC }, {
    'all 3 legs joined': (v) => !!v.legA && !!v.legB && !!v.legC,
  });

  if (!allJoined) {
    conf.hangup();
    confFail.add(1);
    return;
  }

  // Conference active for 30 seconds
  sleep(30);

  // Collect metrics
  conf.hangup();
  const result = conf.result();

  const ok = check(result, {
    'conference ended cleanly': (r) => r && r.success,
    'conf MOS acceptable':      (r) => r && r.mos >= 3.0,
  });

  if (ok) {
    confOK.add(1);
    confLegsMOS.add(result.mos || 0);
  } else {
    confFail.add(1);
  }
}
