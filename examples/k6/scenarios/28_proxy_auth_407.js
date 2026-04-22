/**
 * Scenario 28 — SIP Proxy 407 Authentication
 * ============================================
 * Validates SIP routing through a Session Border Controller (SBC) or
 * SIP Proxy that requires `407 Proxy Authentication Required` prior to
 * routing the INVITE or REGISTER downstream.
 *
 * This scenario checks if xk6-sip-media correctly consumes the 407 challenge,
 * computes the MD5 digest, constructs the `Proxy-Authorization` header, and
 * resends the transaction successfully.
 *
 * Usage:
 *   SIP_REGISTRAR="sip:proxy.example.com" \
 *   SIP_AOR_PREFIX="sip:user" SIP_DOMAIN="example.com" \
 *   SIP_PASSWORD="correct" \
 *   ./k6 run scenarios/28_proxy_auth_407.js
 */
import sip from 'k6/x/sip';
import { check } from 'k6';
import { Counter, Rate } from 'k6/metrics';

const REGISTRAR  = __ENV.SIP_REGISTRAR  || 'sip:192.168.1.100';
const DOMAIN     = __ENV.SIP_DOMAIN     || '192.168.1.100';
const AOR_PREFIX = __ENV.SIP_AOR_PREFIX || 'sip:user';
const PASSWORD   = __ENV.SIP_PASSWORD   || 'correct-password';

const proxyAuthSuccess = new Counter('sip_proxy_auth_success');
const proxyAuthFail    = new Counter('sip_proxy_auth_failure');

export const options = {
  scenarios: {
    proxy_auth: {
      executor:  'constant-vus',
      vus:       20,
      duration:  '1m',
    },
  },
  thresholds: {
    sip_proxy_auth_success: ['count>0'],
    sip_proxy_auth_failure: ['count==0'], // MUST be 0
  },
};

export default function () {
  const vuID = __VU;
  const aor = `${AOR_PREFIX}${vuID}@${DOMAIN}`;

  let reg;
  try {
    // The SIP Proxy will challenge this with a 407 Proxy Authentication Required
    // Our Go backend automatically ACKs it, calculates the Digest, and sends
    // a new REGISTER with a `Proxy-Authorization` header.
    reg = sip.register({
      registrar: REGISTRAR,
      aor:       aor,
      username:  `user${vuID}`,
      password:  PASSWORD,
    });
  } catch (e) {
    proxyAuthFail.add(1);
    console.error(`407 Proxy Auth challenge failed: ${e}`);
    return;
  }

  // If we reach here, the 407 was correctly cleared and 200 OK received
  check(reg, { '407 Proxy Auth handled': (r) => r !== undefined });
  proxyAuthSuccess.add(1);

  reg.unregister();
}
