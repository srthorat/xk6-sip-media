import sip from 'k6/x/sip';
import { check, sleep } from 'k6';

/**
 * xk6-sip-media — Registered UAS (inbound call endpoint)
 * -------------------------------------------------------
 * Simulates a SIP phone / softphone that:
 *   1. Registers its AOR with a SIP proxy or PBX (Digest Auth)
 *   2. Listens on a local port advertised in the REGISTER Contact header
 *   3. Answers every inbound INVITE automatically (200 OK + SDP)
 *   4. Streams an audio file back to the caller as RTP
 *   5. Hangs up after CALL_DURATION, or immediately when the caller sends BYE
 *   6. Unregisters cleanly when the test ends
 *
 * Build first:
 *   xk6 build --with xk6-sip-media=.
 *
 * ── Smoke test (single registration, wait 30 s) ──────────────────────────
 *   SIP_REGISTRAR="sip:pbx.example.com" \
 *   SIP_AOR="sip:alice@pbx.example.com" \
 *   SIP_USERNAME=alice SIP_PASSWORD=secret \
 *   SIP_SMOKE=1 \
 *     ./k6 run examples/k6/registered_uas.js
 *
 * ── Load test (50 VUs, each registered & answering) ──────────────────────
 *   SIP_REGISTRAR="sip:pbx.example.com" \
 *   SIP_AOR_PREFIX="sip:uas" SIP_AOR_DOMAIN="pbx.example.com" \
 *   SIP_USERNAME_PREFIX=uas SIP_PASSWORD=secret \
 *     ./k6 run examples/k6/registered_uas.js
 *
 * ── Paired with a UAC script (two terminals) ─────────────────────────────
 *   Terminal 1 (this script — UAS):
 *     SIP_REGISTRAR=sip:pbx.example.com SIP_AOR=sip:alice@pbx.example.com \
 *     SIP_USERNAME=alice SIP_PASSWORD=secret \
 *       ./k6 run examples/k6/registered_uas.js
 *
 *   Terminal 2 (UAC — calls alice):
 *     SIP_TARGET=sip:alice@pbx.example.com \
 *       ./k6 run examples/k6/call.js
 *
 * Environment variables:
 *   SIP_REGISTRAR      SIP registrar URI          (default: sip:192.168.1.100)
 *   SIP_AOR            Full AOR for single-user   (default: sip:uas@192.168.1.100)
 *   SIP_AOR_PREFIX     AOR user prefix for multi-VU mode (e.g. "uas" → uas1, uas2…)
 *   SIP_AOR_DOMAIN     Domain for multi-VU AOR construction
 *   SIP_USERNAME       Digest auth username (single-user mode)
 *   SIP_USERNAME_PREFIX Digest auth username prefix (multi-VU mode)
 *   SIP_PASSWORD       Digest auth password
 *   SIP_LISTEN_BASE    Base SIP listen port per VU (default: 5070; VU N uses 5070+N)
 *   SIP_AUDIO          WAV file to stream to caller (default: ./examples/audio/sample.wav)
 *   CALL_DURATION      Per-call max duration        (default: 30s; "0s" = wait for BYE)
 *   UAS_MAX_CC         Max concurrent calls per VU  (default: 10)
 *   SIP_SMOKE          Set to "1" for a quick smoke run (1 VU, 30 s)
 */

// ── Configuration ─────────────────────────────────────────────────────────────

const REGISTRAR      = __ENV.SIP_REGISTRAR       || 'sip:192.168.1.100';
const AOR_SINGLE     = __ENV.SIP_AOR             || 'sip:uas@192.168.1.100';
const AOR_PREFIX     = __ENV.SIP_AOR_PREFIX      || '';
const AOR_DOMAIN     = __ENV.SIP_AOR_DOMAIN      || '192.168.1.100';
const USERNAME_SINGLE = __ENV.SIP_USERNAME       || 'uas';
const USERNAME_PREFIX = __ENV.SIP_USERNAME_PREFIX || '';
const PASSWORD       = __ENV.SIP_PASSWORD        || '';
const LISTEN_BASE    = parseInt(__ENV.SIP_LISTEN_BASE  || '5070', 10);
const AUDIO_FILE     = __ENV.SIP_AUDIO           || './examples/audio/sample.wav';
const CALL_DUR       = __ENV.CALL_DURATION        || '30s';
const MAX_CC         = parseInt(__ENV.UAS_MAX_CC  || '10', 10);
const SMOKE          = __ENV.SIP_SMOKE === '1';

// ── k6 options ────────────────────────────────────────────────────────────────

export const options = SMOKE
  ? {
      // Quick smoke: one VU stays registered for 30 s, takes one call
      vus:        1,
      duration:   '30s',
      thresholds: {
        sip_register_success: ['count>0'],
      },
    }
  : {
      scenarios: {
        registered_uas: {
          executor:  'constant-vus',
          vus:       50,      // 50 simultaneous registered endpoints
          duration:  '5m',
        },
      },
      thresholds: {
        // Every VU must successfully register
        sip_register_success: ['count>0'],
      },
    };

// ── Helper: derive per-VU identity ────────────────────────────────────────────

/**
 * Returns { aor, username, listenAddr } for the current VU.
 *
 * Single-user mode  (SIP_AOR set):        every VU shares the same AOR.
 * Multi-user mode   (SIP_AOR_PREFIX set): VU N uses "<prefix>N@<domain>".
 */
function identity() {
  if (AOR_PREFIX) {
    const user = `${AOR_PREFIX}${__VU}`;
    return {
      aor:        `sip:${user}@${AOR_DOMAIN}`,
      username:   USERNAME_PREFIX ? `${USERNAME_PREFIX}${__VU}` : user,
      listenAddr: `0.0.0.0:${LISTEN_BASE + __VU}`,
    };
  }
  return {
    aor:        AOR_SINGLE,
    username:   USERNAME_SINGLE,
    listenAddr: `0.0.0.0:${LISTEN_BASE + __VU}`,
  };
}

// ── Main VU function ──────────────────────────────────────────────────────────

export default function () {
  const { aor, username, listenAddr } = identity();

  // Step 1 — Register with the SIP proxy and start answering inbound calls.
  //
  // registerAndListen() is non-blocking: it sends REGISTER, starts a SIP
  // server on listenAddr, and returns immediately. Inbound INVITEs are
  // answered automatically in the background for as long as the handle is live.
  const uas = sip.registerAndListen({
    registrar:     REGISTRAR,
    aor:           aor,
    username:      username,
    password:      PASSWORD,
    listenAddr:    listenAddr,

    // Audio streamed to each caller
    audio: { file: AUDIO_FILE, codec: 'PCMU' },

    // Per-call duration cap. Set to '0s' to wait for BYE from caller.
    duration:      CALL_DUR,

    // Limit concurrent answered calls per VU (protects RTP port budget)
    maxConcurrent: MAX_CC,
  });

  check(uas, {
    'registered successfully': (u) => u !== null,
  });

  // Step 2 — Stay registered and answering calls for the test duration.
  const listenSecs = SMOKE ? 25 : 290; // leave 5 s for clean shutdown
  sleep(listenSecs);

  // Step 3 — Unregister and shut down (sends REGISTER Expires:0).
  uas.stop();

  console.log(`[VU${__VU}] ${aor} unregistered`);
}
