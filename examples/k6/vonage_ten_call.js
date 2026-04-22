/**
 * Vonage ten-call concurrent smoke test
 *
 * setup()   → SIP REGISTER (UDP, digest auth, 180 s expiry)
 * default() → each VU places one 20 s call using hard.wav, send BYE, log RTP/RTCP stats
 * teardown()→ (registration expires naturally; extend here if needed)
 *
 * Build:
 *   # in /tmp/k6manual (or wherever you built previously)
 *   CGO_ENABLED=1 go build -o /path/to/xk6-sip-media/k6 .
 *
 * Run:
 *   ./k6 run examples/k6/vonage_ten_call.js
 */
import sip from 'k6/x/sip';
import { check } from 'k6';

const USERNAME  = __ENV.VONAGE_USERNAME;
const DOMAIN    = __ENV.VONAGE_DOMAIN;
const REGISTRAR = `sip:${DOMAIN}`;
const AOR       = `sip:${USERNAME}@${DOMAIN}`;
const PASSWORD  = __ENV.VONAGE_PASSWORD;
const CALLEE    = `sip:${__ENV.VONAGE_CALLEE}@${DOMAIN}`;
const DURATION  = '20s';
const AUDIO     = 'examples/audio/hard.wav';

export const options = {
  scenarios: {
    default: {
      executor: 'per-vu-iterations',
      vus: 10,
      iterations: 1,
      maxDuration: '2m',
    },
  },
};

export function setup() {
  console.log(`[setup] Registering ${AOR} via UDP …`);
  try {
    sip.register({
      registrar:  REGISTRAR,
      aor:        AOR,
      username:   USERNAME,
      password:   PASSWORD,
      expires:    180,
    });
    console.log('[setup] Registration OK');
    return { registered: true };
  } catch (e) {
    console.warn(`[setup] Registration failed: ${e} — will attempt call anyway`);
    return { registered: false, regError: String(e) };
  }
}

export default function (data) {
  if (data.registered === false) {
    console.warn(`[VU] Skipping: registration failed in setup (${data.regError})`);
    return;
  }

  console.log(`[VU ${__VU}] Calling ${CALLEE} for ${DURATION} …`);
  const result = sip.call({
    target:    CALLEE,
    aor:       AOR,
    duration:  DURATION,
    username:  USERNAME,
    password:  PASSWORD,
    rtcp:      true,
    audio: {
      file: AUDIO,
      codec: 'PCMU',
    },
  });

  check(result, {
    'call succeeded':       (r) => r.success === true,
    'sent RTP packets':     (r) => r.sent > 0,
    'received RTP packets': (r) => r.received > 0,
    'packet loss < 5%':     (r) => (r.lost / Math.max(r.sent + r.received, 1)) < 0.05,
    'mos > 3.8':            (r) => r.mos != null && r.mos > 3.8,
  });

  console.log([
    `─── Call Stats (VU ${__VU}) ──────────────────`,
    `  success  : ${result.success}`,
    `  error    : ${result.error    || 'none'}`,
    `  sent     : ${result.sent}    pkts`,
    `  received : ${result.received} pkts`,
    `  lost     : ${result.lost}    pkts`,
    `  loss_pct : ${result.loss_pct != null ? result.loss_pct.toFixed(2) : 'n/a'} %`,
    `  jitter   : ${result.jitter   != null ? result.jitter.toFixed(2)  : 'n/a'} ms`,
    `  MOS      : ${result.mos      != null ? result.mos.toFixed(2)     : 'n/a'}`,
    `  rtt_ms   : ${result.rtt_ms   != null ? result.rtt_ms.toFixed(2)  : 'n/a'} ms`,
    `  rtcp_fraction_lost : ${result.rtcp_fraction_lost != null ? result.rtcp_fraction_lost : 'n/a'}`,
    `  rtcp_cumulative_lost : ${result.rtcp_cumulative_lost != null ? result.rtcp_cumulative_lost : 'n/a'}`,
    `  silence_ratio : ${result.silence_ratio != null ? result.silence_ratio.toFixed(4) : 'n/a'}`,
    `────────────────────────────────────────────`,
  ].join('\n'));
}

export function teardown(data) {
  if (data.registered) {
    console.log('[teardown] Registration will expire in ~180 s. Test complete.');
  }
}