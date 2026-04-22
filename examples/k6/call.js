import sip from 'k6/x/sip';
import { check } from 'k6';

/**
 * xk6-sip-media — Basic load test example
 * ----------------------------------------
 * Makes a SIP call to the target server, streams WAV audio as RTP,
 * sends DTMF digits, and records quality metrics.
 *
 * Build first:
 *   xk6 build --with xk6-sip-media=.
 *
 * Run:
 *   ./k6 run examples/k6/call.js
 *
 * Environment variables:
 *   SIP_TARGET   SIP URI to call         (default: sip:ivr@127.0.0.1)
 *   SIP_DURATION Call duration           (default: 20s)
 *   SIP_WAV      Path to WAV audio file  (default: ./examples/audio/sample.wav)
 */

const TARGET   = __ENV.SIP_TARGET   || 'sip:ivr@127.0.0.1';
const DURATION = __ENV.SIP_DURATION || '20s';
const WAV_FILE = __ENV.SIP_WAV      || './examples/audio/sample.wav';

export const options = {
  vus: 10,
  duration: '2m',

  thresholds: {
    // At least 95% of calls must succeed
    sip_call_success: ['count>0'],
    // MOS score must average ≥ 3.5 (Fair or better)
    mos_score: ['avg>=3.5'],
    // Jitter must stay below 50ms
    rtp_jitter_ms: ['avg<50'],
  },
};

export default function () {
  const result = sip.call({
    target:   TARGET,
    duration: DURATION,

    audio: {
      file:  WAV_FILE,
      codec: 'PCMU',
    },

    dtmf: ['1', '2'], // IVR navigation: press 1, then 2
  });

  check(result, {
    'call succeeded':  (r) => r.success === true,
    'MOS acceptable':  (r) => r.mos >= 3.5,
    'IVR responded':   (r) => r.ivr_ok === true,
  });

  if (!result.success) {
    console.error(`Call failed: ${result.error}`);
  } else {
    console.log(
      `Call OK | MOS=${result.mos.toFixed(2)} ` +
      `sent=${result.sent} recv=${result.received} lost=${result.lost} ` +
      `jitter=${result.jitter.toFixed(1)}ms`
    );
  }
}
