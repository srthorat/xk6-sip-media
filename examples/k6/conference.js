/**
 * Conference Bridge Load Test
 *
 * Each VU dials into a conference bridge URI with N participant legs.
 * The bridge (Asterisk ConfBridge / FreeSWITCH) mixes audio.
 * xk6-sip-media manages N independent SIP/RTP legs per VU.
 *
 * Usage:
 *   SIP_BRIDGE="sip:conf-room-101@pbx" SIP_PARTICIPANTS=3 \
 *     ./k6 run examples/k6/conference.js
 */
import sip from 'k6/x/sip';
import { check, sleep } from 'k6';

const BRIDGE       = __ENV.SIP_BRIDGE        || 'sip:conf101@192.168.1.100';
const PARTICIPANTS = parseInt(__ENV.SIP_PARTICIPANTS || '3', 10);
const DURATION     = __ENV.SIP_DURATION      || '30s';

export const options = {
  scenarios: {
    conference_load: {
      executor:    'ramping-vus',
      startVUs:    0,
      stages: [
        { duration: '30s', target: 10 },  // 10 VUs × 3 legs = 30 conference legs
        { duration: '60s', target: 30 },  // 30 VUs × 3 legs = 90 conference legs
        { duration: '30s', target: 0  },
      ],
    },
  },
  thresholds: {
    sip_call_success:    ['count>0'],
    sip_conference_legs: ['avg>=1'],
    mos_score:           ['avg>=3.5'],
    rtp_jitter_ms:       ['avg<80'],  // bridges can add extra buffering
  },
};

export default function () {
  // Start conference with host leg (dial 1)
  const conf = sip.conference({
    bridgeURI: BRIDGE,
    audio:     { file: './examples/audio/sample.wav' },
    duration:  DURATION,
  });

  // Add remaining participants (dials 2…N)
  for (let i = 1; i < PARTICIPANTS; i++) {
    const err = conf.addParticipant('', { // empty = use same bridgeURI
      audio: { file: './examples/audio/sample.wav' },
    });
    if (err !== null) {
      console.warn(`participant ${i + 1} failed to join: ${err}`);
    }
  }

  check(conf, {
    'all participants joined': (c) => c.participantCount() === PARTICIPANTS,
  });

  // Conference runs until duration expires (set on each leg)
  conf.waitDone();

  const r = conf.result();
  check(r, {
    'conference completed':   (r) => r.legs > 0,
    'avg MOS >= 3.5':         (r) => r.avg_mos >= 3.5,
    'packet loss < 5%':       (r) => r.packets_lost / Math.max(r.packets_sent, 1) < 0.05,
  });

  console.log(
    `conf done: legs=${r.legs} avg_mos=${r.avg_mos.toFixed(2)} ` +
    `min_mos=${r.min_mos.toFixed(2)} lost=${r.packets_lost}/${r.packets_sent}`,
  );
}
