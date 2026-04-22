/**
 * TLS / SIPS Transport Load Test
 * ================================
 * Runs SIP calls over TLS (SIPS, port 5061) — mandatory for carrier-grade
 * PSTN gateway, SBC, and ITSP connectivity.
 *
 * Scenario A — Skip-verify TLS:   no certificate setup, fast to run
 * Scenario B — Mutual TLS:        presents client cert, verifies server cert
 *                                   (requires cert files, see env vars)
 *
 * Certificate generation (for testing):
 *   ./scripts/gen_test_certs.sh   (created below)
 *
 * Usage (skip-verify, quickest):
 *   SIP_TARGET="sip:ivr@pbx" ./k6 run scenarios/12_tls_transport.js
 *
 * Usage (mutual TLS):
 *   SIP_TARGET="sip:ivr@pbx" \
 *   TLS_CERT=./certs/client.pem \
 *   TLS_KEY=./certs/client.key  \
 *   TLS_CA=./certs/ca.pem       \
 *   TLS_SERVER_NAME=pbx.example.com \
 *   ./k6 run scenarios/12_tls_transport.js
 */
import sip from 'k6/x/sip';
import { check } from 'k6';

const TARGET      = __ENV.SIP_TARGET      || 'sip:ivr@192.168.1.100';
const SIP_PORT    = parseInt(__ENV.SIP_PORT || '5061', 10);
const AUDIO_FILE  = __ENV.SIP_AUDIO       || './examples/audio/sample.wav';

// TLS cert paths (empty = skip-verify mode)
const TLS_CERT        = __ENV.TLS_CERT        || '';
const TLS_KEY         = __ENV.TLS_KEY         || '';
const TLS_CA          = __ENV.TLS_CA          || '';
const TLS_SERVER_NAME = __ENV.TLS_SERVER_NAME || '';

// Build tls options object — only include keys that are set
function buildTLSOpts() {
  const t = { skipVerify: true };    // safe default for load testing
  if (TLS_CERT)        { t.cert = TLS_CERT; t.skipVerify = false; }
  if (TLS_KEY)         { t.key  = TLS_KEY; }
  if (TLS_CA)          { t.ca   = TLS_CA; }
  if (TLS_SERVER_NAME) { t.serverName = TLS_SERVER_NAME; }
  return t;
}

const tlsOpts = buildTLSOpts();

export const options = {
  scenarios: {
    // ── Scenario A: TLS skip-verify (fast smoke) ─────────────────────────
    tls_skip_verify: {
      executor:  'ramping-vus',
      startVUs:  0,
      stages: [
        { duration: '1m',  target: 20  },
        { duration: '3m',  target: 100 },
        { duration: '3m',  target: 100 },
        { duration: '1m',  target: 0   },
      ],
    },
  },
  thresholds: {
    sip_call_success:  ['count>0'],
    sip_call_failure:  ['rate<0.02'],
    mos_score:         ['avg>=3.5'],
    rtp_jitter_ms:     ['avg<50'],
    sip_call_duration: ['p(95)<3000'],  // TLS handshake adds ~50ms
  },
};

export default function () {
  const result = sip.call({
    target:    TARGET,
    transport: 'tls',          // ← TLS signaling over port 5061
    sipPort:   SIP_PORT,
    tls:       tlsOpts,
    duration:  '15s',
    audio:     { file: AUDIO_FILE },
  });

  check(result, {
    'TLS call ok':     (r) => r.success,
    'MOS >= 3.5':      (r) => r.mos >= 3.5,
    'no packet loss':  (r) => r.lost === 0,
  });
}
