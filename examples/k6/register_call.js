/**
 * REGISTER + Call Load Test
 *
 * Each VU:
 *  1. Registers its AOR with the registrar (with Digest Auth)
 *  2. Makes a SIP call
 *  3. Unregisters
 *
 * Usage:
 *   SIP_REGISTRAR="sip:pbx.example.com" \
 *   SIP_AOR="sip:alice@pbx.example.com" \
 *   SIP_USERNAME=alice SIP_PASSWORD=secret \
 *   SIP_TARGET="sip:bob@pbx.example.com" \
 *     ./k6 run examples/k6/register_call.js
 */
import sip from 'k6/x/sip';
import { check } from 'k6';

const REGISTRAR = __ENV.SIP_REGISTRAR || 'sip:192.168.1.100';
const AOR       = __ENV.SIP_AOR       || 'sip:k6-user@192.168.1.100';
const USERNAME  = __ENV.SIP_USERNAME  || 'k6-user';
const PASSWORD  = __ENV.SIP_PASSWORD  || '';
const TARGET    = __ENV.SIP_TARGET    || 'sip:ivr@192.168.1.100';
const DURATION  = __ENV.SIP_DURATION  || '10s';
const WAV_FILE  = __ENV.SIP_WAV       || './examples/audio/sample.wav';
const SMOKE     = __ENV.SIP_SMOKE === '1';

export const options = SMOKE
  ? {
      vus: 1,
      iterations: 1,
      thresholds: {
        sip_register_success: ['count>0'],
        sip_call_success: ['count>0'],
      },
    }
  : {
      scenarios: {
        register_call: {
          executor: 'ramping-vus',
          startVUs: 0,
          stages: [
            { duration: '30s', target: 50 },
            { duration: '60s', target: 100 },
            { duration: '30s', target: 0 },
          ],
        },
      },
      thresholds: {
        sip_register_success: ['count>0'],
        sip_call_success: ['count>0'],
        mos_score: ['avg>=3.5'],
      },
    };

export default function () {
  // Register
  const reg = sip.register({
    registrar: REGISTRAR,
    aor:       AOR,
    username:  USERNAME,
    password:  PASSWORD,
    expires:   120, // short expiry for load tests
  });

  // Make a call while registered
  const result = sip.call({
    target: TARGET,
    aor: AOR,
    username: USERNAME,
    password: PASSWORD,
    duration: DURATION,
    rtcp: true,
    audio: {
      file: WAV_FILE,
      codec: 'PCMU',
    },
  });

  check(result, {
    'call succeeded': (r) => r.success,
    'MOS >= 3.5':     (r) => r.mos >= 3.5,
  });

  // Unregister
  const unregErr = reg.unregister();
  check(unregErr, { 'unregistered ok': (e) => e == null });
}
