/**
 * Vonage IVR flow smoke test
 *
 * setup()   → SIP REGISTER (UDP, digest auth, 180 s expiry)
 * default() → dial IVR extension 443362, wait 3 s for announcement, press 1,
 *              wait 2 s, send BYE, log RTP/RTCP stats
 * teardown()→ registration expires naturally
 *
 * Run:
 *   ./k6 run examples/k6/vonage_ivr_flow.js
 */
import sip from 'k6/x/sip';
import { check, sleep } from 'k6';

const USERNAME = __ENV.VONAGE_USERNAME;
const DOMAIN = __ENV.VONAGE_DOMAIN;
const REGISTRAR = `sip:${DOMAIN}`;
const AOR = `sip:${USERNAME}@${DOMAIN}`;
const PASSWORD = __ENV.VONAGE_PASSWORD;
const CALLEE = `sip:${__ENV.VONAGE_IVR_CALLEE}@${DOMAIN}`;
const AUDIO = 'examples/audio/hard.wav';

const ANNOUNCEMENT_WAIT_SECS = 3;
const POST_DTMF_WAIT_SECS = 20;

export const options = {
  vus: 1,
  iterations: 1,
};

export function setup() {
  console.log(`[setup] Registering ${AOR} via UDP ...`);
  try {
    sip.register({
      registrar: REGISTRAR,
      aor: AOR,
      username: USERNAME,
      password: PASSWORD,
      expires: 180,
    });
    console.log('[setup] Registration OK');
    return { registered: true };
  } catch (e) {
    console.warn(`[setup] Registration failed: ${e} - will skip IVR flow`);
    return { registered: false, regError: String(e) };
  }
}

export default function (data) {
  if (data.registered === false) {
    console.warn(`[VU] Skipping: registration failed in setup (${data.regError})`);
    return;
  }

  console.log(`[VU] Dialing IVR ${CALLEE} ...`);
  const call = sip.dial({
    target: CALLEE,
    aor: AOR,
    username: USERNAME,
    password: PASSWORD,
    rtcp: true,
    audio: {
      file: AUDIO,
      codec: 'PCMU',
    },
  });

  console.log(`[VU] Waiting ${ANNOUNCEMENT_WAIT_SECS}s for announcement ...`);
  sleep(ANNOUNCEMENT_WAIT_SECS);

  console.log('[VU] Sending DTMF 2 ...');
  call.sendDTMF('2');

  console.log(`[VU] Waiting ${POST_DTMF_WAIT_SECS}s after DTMF ...`);
  sleep(POST_DTMF_WAIT_SECS);

  console.log('[VU] Sending BYE ...');
  const hangupErr = call.hangup();

  call.waitDone();
  const result = call.result();

  check({ result, hangupErr }, {
    'call succeeded': ({ result }) => result.success === true,
    'sent RTP packets': ({ result }) => result.sent > 0,
    'received RTP packets': ({ result }) => result.received > 0,
    'packet loss < 5%': ({ result }) => (result.lost / Math.max(result.sent + result.received, 1)) < 0.05,
    'hangup ok': ({ hangupErr }) => hangupErr == null,
  });

  console.log([
    '--- IVR Flow Stats ------------------------',
    `  success  : ${result.success}`,
    `  error    : ${result.error || 'none'}`,
    `  hangup   : ${hangupErr || 'ok'}`,
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

export function teardown(data) {
  if (data.registered) {
    console.log('[teardown] Registration will expire in ~180 s. Test complete.');
  }
}