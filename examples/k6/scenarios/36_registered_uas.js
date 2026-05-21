/**
 * Scenario 36 — Registered UAS: Register → Wait for Call → Answer → Play → BYE
 * ==============================================================================
 * Simulates a SIP endpoint (phone/softphone) that:
 *   1. Registers its AOR with a SIP proxy/PBX (Digest Auth)
 *   2. Listens on a local port advertised in the REGISTER Contact header
 *   3. Answers every inbound INVITE automatically
 *   4. Streams an audio file back to the caller
 *   5. Hangs up after `CALL_DURATION` or immediately on BYE from caller
 *   6. Unregisters cleanly when the test ends
 *
 * Pair this with a UAC script (e.g. 01_baseline.js) or SIPp targeting the
 * same AOR to drive inbound calls.
 *
 * Usage — single VU (smoke):
 *   SIP_REGISTRAR="sip:pbx.example.com" \
 *   SIP_AOR="sip:alice@pbx.example.com" \
 *   SIP_USERNAME=alice SIP_PASSWORD=secret \
 *   SIP_LISTEN_ADDR="0.0.0.0:5070" \
 *   SIP_AUDIO="./examples/audio/sample.wav" \
 *     ./k6 run scenarios/35_registered_uas.js
 *
 * Usage — parallel (Terminal 1 = this script, Terminal 2 = UAC):
 *   Terminal 1: ./k6 run scenarios/35_registered_uas.js
 *   Terminal 2: SIP_TARGET="sip:alice@<uas-ip>:5070" ./k6 run scenarios/01_baseline.js
 *
 * Environment variables:
 *   SIP_REGISTRAR      SIP registrar URI           (default: sip:192.168.1.100)
 *   SIP_AOR            Address of Record URI        (default: sip:k6-uas@192.168.1.100)
 *   SIP_USERNAME       Digest auth username
 *   SIP_PASSWORD       Digest auth password
 *   SIP_LISTEN_ADDR    Local SIP listen address     (default: 0.0.0.0:5070)
 *   SIP_AUDIO          WAV file to stream           (default: ./examples/audio/sample.wav)
 *   CALL_DURATION      Max per-call duration        (default: 30s, 0 = wait for BYE)
 *   UAS_MAX_CC         Max concurrent answered calls (default: 50)
 *   TEST_DURATION_S    How long to stay registered, in seconds (default: 600)
 */
import sip from 'k6/x/sip';
import { sleep } from 'k6';

const REGISTRAR    = __ENV.SIP_REGISTRAR   || 'sip:192.168.1.100';
const AOR          = __ENV.SIP_AOR         || 'sip:k6-uas@192.168.1.100';
const USERNAME     = __ENV.SIP_USERNAME    || 'k6-uas';
const PASSWORD     = __ENV.SIP_PASSWORD    || '';
const LISTEN_ADDR  = __ENV.SIP_LISTEN_ADDR || '0.0.0.0:5070';
const AUDIO_FILE   = __ENV.SIP_AUDIO       || './examples/audio/sample.wav';
const CALL_DUR     = __ENV.CALL_DURATION   || '30s';
const MAX_CC       = parseInt(__ENV.UAS_MAX_CC   || '50', 10);
const TEST_DUR_S   = parseInt(__ENV.TEST_DURATION_S || '600', 10); // seconds

export const options = {
  scenarios: {
    registered_uas: {
      executor:  'constant-vus',
      vus:       1,   // one VU holds the registration; calls are handled concurrently
      duration:  `${TEST_DUR_S}s`,
    },
  },
  thresholds: {
    sip_register_success: ['count>0'],
  },
};

export default function () {
  // Step 1: Register with the SIP proxy and start listening for inbound calls.
  // Calls are answered automatically in the background; the VU just sleeps.
  const uas = sip.registerAndListen({
    registrar:     REGISTRAR,
    aor:           AOR,
    username:      USERNAME,
    password:      PASSWORD,
    listenAddr:    LISTEN_ADDR,
    audio:         { file: AUDIO_FILE },
    duration:      CALL_DUR,    // per-call max duration; '0' = wait for BYE
    maxConcurrent: MAX_CC,
  });

  // Step 2: Stay registered and answering calls for the test duration.
  sleep(TEST_DUR_S);

  // Step 3: Unregister and shut down cleanly.
  uas.stop();
}
