/**
 * Scenario 13 — UAS Mode: Answer Incoming Calls
 * ===============================================
 * Starts an xk6-sip-media UAS server that answers inbound INVITEs and
 * streams a WAV audio file back to the caller.
 *
 * Use this alongside a UAC load test (01–12) or SIPp UAC scenario to
 * simulate a fully-loaded agent endpoint.
 *
 * Usage:
 *   SIP_UAC_TARGET="sip:k6uas@10.0.0.1:5080" ./k6 run scenarios/13_uas_server.js
 *
 * Both the embedded UAS (this script) and an external UAC must be running
 * simultaneously. In CI, use two k6 processes:
 *   Terminal 1: k6 run scenarios/13_uas_server.js   (server)
 *   Terminal 2: k6 run scenarios/01_baseline.js      (client → port 5080)
 */
import sip from 'k6/x/sip';
import { sleep } from 'k6';

const LISTEN_ADDR = __ENV.UAS_ADDR      || '0.0.0.0:5080';
const TRANSPORT   = __ENV.UAS_TRANSPORT || 'udp';
const AUDIO_FILE  = __ENV.SIP_AUDIO     || './examples/audio/sample.wav';
const MAX_CC      = parseInt(__ENV.UAS_MAX_CC || '200', 10);
const CALL_DUR_S  = parseInt(__ENV.UAS_CALL_DURATION || '30', 10);

export const options = {
  scenarios: {
    uas_server: {
      executor:  'constant-vus',
      vus:       1,              // single VU runs the server listener
      duration:  '10m',
    },
  },
  thresholds: {
    sip_call_success: ['count>0'],
  },
};

export default function () {
  const server = sip.serve({
    listenAddr:    LISTEN_ADDR,
    transport:     TRANSPORT,
    audio:         { file: AUDIO_FILE },
    maxConcurrent: MAX_CC,
    callDuration:  `${CALL_DUR_S}s`,
  });

  // Block the VU for the test duration
  sleep(600);

  server.stop();
}
