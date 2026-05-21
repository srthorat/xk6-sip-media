/**
 * uac_load.js — 100-VU UAC load test (calls paired UAS endpoints)
 *
 * Each VU picks one row from uac_users.csv, registers as a UAC, then
 * places a call to the matching UAS endpoint (the 'callee' CSV column).
 *
 * Environment variables:
 *   SIP_REGISTRAR  SIP registrar URI   (default: sip:sip.example.com)
 *   SIP_AUDIO      Path to audio file  (default: ./examples/audio/sample.wav)
 *   SIP_CODEC      RTP codec name      (default: PCMU)
 *   CALL_DURATION  Call duration       (default: 30s)
 *
 * Run (after uas_load.js has started and all UAS VUs are registered):
 *   SIP_REGISTRAR=sip:pbx.example.com \
 *   SIP_AUDIO=./examples/audio/sample.wav \
 *     ./k6 run examples/k6/uac_load.js
 */

import sip from 'k6/x/sip';
import { check } from 'k6';

const REGISTRAR = __ENV.SIP_REGISTRAR || 'sip:sip.example.com';
const AUDIO     = __ENV.SIP_AUDIO     || './examples/audio/sample.wav';
const CODEC     = __ENV.SIP_CODEC     || 'PCMU';
const CALL_DUR  = __ENV.CALL_DURATION || '30s';

// Each VU gets its own row (VU 1 → row 1 → calls uas001, VU 2 → uas002 …)
const pool = sip.loadCSV('./examples/csv/uac_users.csv');

export const options = {
  scenarios: {
    uac: {
      executor:    'per-vu-iterations',
      vus:         100,
      iterations:  1,
      maxDuration: '5m',
    },
  },
  thresholds: {
    sip_register_success: ['count>=100'],
    sip_call_success:     ['count>=95'],   // allow ≤5% failure
    mos_score:            ['avg>=3.5'],
  },
};

export default function () {
  const creds = pool.pick(__VU);

  // Register the UAC endpoint
  sip.register({
    registrar: REGISTRAR,
    aor:       `sip:${creds.username}@${creds.domain}`,
    username:  creds.username,
    password:  creds.password,
    expires:   120,
  });

  // Call the paired UAS endpoint ('callee' column = uas001, uas002 …)
  const result = sip.call({
    target:   `sip:${creds.callee}@${creds.domain}`,
    aor:      `sip:${creds.username}@${creds.domain}`,
    username: creds.username,
    password: creds.password,
    duration: CALL_DUR,
    audio:    { file: AUDIO, codec: CODEC },
    rtcp:     true,
  });

  check(result, {
    'call succeeded': (r) => r.success === true,
    'MOS >= 3.5':     (r) => r.mos >= 3.5,
    'loss < 5%':      (r) => (r.lost / Math.max(r.sent, 1)) < 0.05,
  });

  if (!result.success) {
    console.error(`[VU${__VU}] ${creds.username} → ${creds.callee}: ${result.error}`);
  } else {
    console.log(
      `[VU${__VU}] ${creds.username} → ${creds.callee}` +
      ` | MOS=${result.mos.toFixed(2)} sent=${result.sent} lost=${result.lost}`
    );
  }
}
