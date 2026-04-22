/**
 * Vonage direct-call smoke test
 *
 * default() → call VR extension 443361 directly without prior REGISTER,
 *              handle INVITE proxy auth, run 20 s using hard.wav,
 *              then log RTP/RTCP stats
 *
 * Run:
 *   ./k6 run examples/k6/vonage_direct_call.js
 */
import sip from 'k6/x/sip';
import { check } from 'k6';

const USERNAME = __ENV.VONAGE_USERNAME;
const DOMAIN = __ENV.VONAGE_DOMAIN;
const AOR = `sip:${USERNAME}@${DOMAIN}`;
const PASSWORD = __ENV.VONAGE_PASSWORD;
const CALLEE = `sip:${__ENV.VONAGE_CALLEE}@${DOMAIN}`;
const DURATION = '20s';
const AUDIO = 'examples/audio/hard.wav';

export const options = {
  vus: 1,
  iterations: 1,
};

export default function () {
  console.log(`[VU] Direct calling ${CALLEE} for ${DURATION} without REGISTER ...`);
  const result = sip.call({
    target: CALLEE,
    aor: AOR,
    duration: DURATION,
    username: USERNAME,
    password: PASSWORD,
    rtcp: true,
    audio: {
      file: AUDIO,
      codec: 'PCMU',
    },
  });

  check(result, {
    'call succeeded': (r) => r.success === true,
    'sent RTP packets': (r) => r.sent > 0,
    'received RTP packets': (r) => r.received > 0,
    'packet loss < 5%': (r) => (r.lost / Math.max(r.sent + r.received, 1)) < 0.05,
    'mos > 3.8': (r) => r.mos != null && r.mos > 3.8,
  });

  console.log([
    '--- Direct Call Stats ---------------------',
    `  success  : ${result.success}`,
    `  error    : ${result.error || 'none'}`,
    `  sent     : ${result.sent} pkts`,
    `  received : ${result.received} pkts`,
    `  lost     : ${result.lost} pkts`,
    `  loss_pct : ${result.loss_pct != null ? result.loss_pct.toFixed(2) : 'n/a'} %`,
    `  jitter   : ${result.jitter != null ? result.jitter.toFixed(2) : 'n/a'} ms`,
    `  MOS      : ${result.mos != null ? result.mos.toFixed(2) : 'n/a'}`,
    `  rtt_ms   : ${result.rtt_ms != null ? result.rtt_ms.toFixed(2) : 'n/a'} ms`,
    `  rtcp_fraction_lost : ${result.rtcp_fraction_lost != null ? result.rtcp_fraction_lost : 'n/a'}`,
    `  rtcp_cumulative_lost : ${result.rtcp_cumulative_lost != null ? result.rtcp_cumulative_lost : 'n/a'}`,
    '-------------------------------------------',
  ].join('\n'));
}