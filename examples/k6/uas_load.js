/**
 * uas_load.js — 100-VU Registered UAS load test
 *
 * Each VU picks one row from uas_users.csv, registers on a unique port,
 * and answers every inbound call by playing an audio file.
 *
 * Environment variables:
 *   SIP_REGISTRAR    SIP registrar URI          (default: sip:sip.example.com)
 *   SIP_LISTEN_BASE  Base UDP port for UAS VUs  (default: 5100 → VU 1 = 5101)
 *   SIP_AUDIO        Path to audio file         (default: ./examples/audio/sample.wav)
 *   SIP_CODEC        RTP codec name             (default: PCMU)
 *   CALL_DURATION    Per-call time cap          (default: 30s; use 0s to wait for BYE)
 *   MAX_CONCURRENT   Max simultaneous calls/VU  (default: 5)
 *   TEST_DURATION_S  Total test duration (sec)  (default: 300)
 *
 * Run:
 *   SIP_REGISTRAR=sip:pbx.example.com \
 *   SIP_AUDIO=./examples/audio/sample.wav \
 *     ./k6 run examples/k6/uas_load.js
 */

import sip from 'k6/x/sip';
import { sleep } from 'k6';

const REGISTRAR    = __ENV.SIP_REGISTRAR    || 'sip:sip.example.com';
const LISTEN_BASE  = parseInt(__ENV.SIP_LISTEN_BASE || '5100', 10);
const AUDIO        = __ENV.SIP_AUDIO        || './examples/audio/sample.wav';
const CODEC        = __ENV.SIP_CODEC        || 'PCMU';
const CALL_DUR     = __ENV.CALL_DURATION    || '30s';
const MAX_CC       = parseInt(__ENV.MAX_CONCURRENT   || '5', 10);
const TEST_DUR_S   = parseInt(__ENV.TEST_DURATION_S  || '300', 10);

// Each VU gets its own row (VU 1 → row 1, VU 2 → row 2 …)
const pool = sip.loadCSV('./examples/csv/uas_users.csv');

export const options = {
  scenarios: {
    uas: {
      executor: 'constant-vus',
      vus:      100,
      duration: `${TEST_DUR_S}s`,
    },
  },
  thresholds: {
    sip_register_success: ['count>=100'],
  },
};

export default function () {
  const creds = pool.pick(__VU);

  const uas = sip.registerAndListen({
    registrar:     REGISTRAR,
    aor:           `sip:${creds.username}@${creds.domain}`,
    username:      creds.username,
    password:      creds.password,
    listenAddr:    `0.0.0.0:${LISTEN_BASE + __VU}`, // port 5101–5200
    audio:         { file: AUDIO, codec: CODEC },
    duration:      CALL_DUR,
    maxConcurrent: MAX_CC,
  });

  // Stay registered for the test duration minus 10 s to allow clean shutdown
  sleep(Math.max(TEST_DUR_S - 10, 1));

  // Send REGISTER Expires:0 to unregister before the VU exits
  uas.stop();
}
