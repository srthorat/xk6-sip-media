import sip from 'k6/x/sip';
import { check, sleep } from 'k6';

/**
 * xk6-sip-media — IVR Flow Test
 * ------------------------------
 * Simulates a full IVR navigation flow:
 *   1. Connect to IVR
 *   2. Wait for greeting
 *   3. Press 1 (sales department)
 *   4. Press 3 (new inquiry)
 *   5. Stay connected for 30s
 *   6. Hang up
 *
 * Includes MOS threshold enforcement and per-call console reporting.
 */

export const options = {
  scenarios: {
    ivr_load: {
      executor:          'ramping-vus',
      startVUs:          0,
      stages: [
        { duration: '30s', target: 50  },  // ramp up
        { duration: '2m',  target: 200 },  // sustained load
        { duration: '30s', target: 0   },  // ramp down
      ],
      gracefulRampDown: '10s',
    },
  },

  thresholds: {
    sip_call_success:        ['count>0'],
    sip_call_failure:        ['count<50'],
    mos_score:               ['avg>=3.5', 'p(95)>=3.0'],
    rtp_jitter_ms:           ['avg<40', 'p(95)<100'],
    rtp_packets_lost:        ['count<1000'],
    sip_call_duration:       ['avg<35000', 'p(95)<40000'],
  },
};

const IVR_TARGET = __ENV.SIP_TARGET || 'sip:ivr@192.168.1.100';
const WAV_FILE   = __ENV.SIP_WAV    || './examples/audio/sample.wav';
const USERNAME   = __ENV.SIP_USERNAME || '';
const PASSWORD   = __ENV.SIP_PASSWORD || '';
const AOR        = __ENV.SIP_AOR      || '';

export default function () {
  const result = sip.call({
    target:   IVR_TARGET,
    duration: '30s',
    ...(USERNAME && { username: USERNAME }),
    ...(PASSWORD && { password: PASSWORD }),
    ...(AOR      && { aor: AOR }),

    audio: {
      file:  WAV_FILE,
      codec: 'PCMU',
    },

    // IVR navigation sequence
    // digits are sent with auto 2s delay between each
    dtmf: ['1', '3'],

    // enable MOS via E-model (always on)
    // enable PESQ only if pesq binary present and env var set
    pesq: __ENV.ENABLE_PESQ === '1',
  });

  const success = check(result, {
    'call connected':      (r) => r.success === true,
    'MOS >= 3.5':          (r) => r.mos >= 3.5,
    'packet loss < 5%':    (r) => (r.lost / Math.max(r.sent, 1)) < 0.05,
    'sent RTP packets':    (r) => r.sent > 0,
  });

  if (!success) {
    console.warn(`[VU${__VU}] degraded call: MOS=${result.mos} lost=${result.lost}/${result.sent}`);
  }

  // Short pause between calls to avoid hammering the SIP stack
  sleep(1);
}
