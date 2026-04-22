/**
 * REGISTER-only example
 *
 * Each VU:
 *  1. Registers its AOR with the registrar using Digest Auth
 *  2. Optionally unregisters immediately
 *
 * Usage:
 *   SIP_REGISTRAR="sip:pbx.example.com" \
 *   SIP_AOR="sip:alice@pbx.example.com" \
 *   SIP_USERNAME=alice SIP_PASSWORD=secret \
 *     ./k6 run examples/k6/register_only.js
 *
 * Smoke mode:
 *   SIP_SMOKE=1 ./k6 run examples/k6/register_only.js
 */
import sip from 'k6/x/sip';
import { check } from 'k6';

const REGISTRAR = __ENV.SIP_REGISTRAR || 'sip:192.168.1.100';
const AOR = __ENV.SIP_AOR || 'sip:k6-user@192.168.1.100';
const USERNAME = __ENV.SIP_USERNAME || 'k6-user';
const PASSWORD = __ENV.SIP_PASSWORD || '';
const EXPIRES = parseInt(__ENV.SIP_EXPIRES || '120', 10);
const DO_UNREGISTER = __ENV.SIP_UNREGISTER !== '0';
const SMOKE = __ENV.SIP_SMOKE === '1';

export const options = SMOKE
  ? {
      vus: 1,
      iterations: 1,
      thresholds: {
        sip_register_success: ['count>0'],
      },
    }
  : {
      scenarios: {
        register_only: {
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
      },
    };

export default function () {
  const reg = sip.register({
    registrar: REGISTRAR,
    aor: AOR,
    username: USERNAME,
    password: PASSWORD,
    expires: EXPIRES,
  });

  check(reg, {
    'registered ok': (value) => value != null,
  });

  if (DO_UNREGISTER) {
    const unregErr = reg.unregister();
    check(unregErr, {
      'unregistered ok': (value) => value == null,
    });
  }
}