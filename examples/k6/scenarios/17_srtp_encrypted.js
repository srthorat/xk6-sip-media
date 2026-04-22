/**
 * Scenario 17 — SRTP Encrypted Media
 * =====================================
 * Tests SIP calls with SRTP (Secure RTP, RFC 3711) encrypted media.
 * xk6 generates a fresh AES-CM-128-HMAC-SHA1-80 key per call, advertises
 * it in SDP a=crypto, and encrypts every RTP packet.
 *
 * SIP signaling:  UDP (plain) — swap transport='tls' for full SIPS+SRTP
 * Media:          SRTP AES-CM-128-HMAC-SHA1-80
 *
 * Usage:
 *   SIP_TARGET="sip:ivr@pbx" SIP_AUDIO=./examples/audio/sample.wav \
 *     ./k6 run scenarios/17_srtp_encrypted.js
 *
 * To test fully-encrypted sessions (SIPS + SRTP):
 *   SIP_TARGET="sips:ivr@pbx" SIP_TRANSPORT=tls \
 *     ./k6 run scenarios/17_srtp_encrypted.js
 */
import sip from 'k6/x/sip';
import { check } from 'k6';
import { Counter, Trend } from 'k6/metrics';

const TARGET    = __ENV.SIP_TARGET    || 'sip:ivr@192.168.1.100';
const AUDIO     = __ENV.SIP_AUDIO     || './examples/audio/sample.wav';
const TRANSPORT = __ENV.SIP_TRANSPORT || 'udp';
const DURATION  = __ENV.CALL_DURATION || '20s';

const srtpSuccess = new Counter('srtp_call_success');
const srtpMOS     = new Trend('srtp_mos_score');

export const options = {
  scenarios: {
    srtp_load: {
      executor:  'ramping-vus',
      startVUs:  1,
      stages: [
        { duration: '30s', target: 10  },
        { duration: '2m',  target: 50  },
        { duration: '30s', target: 0   },
      ],
    },
  },
  thresholds: {
    srtp_call_success: ['count>0'],
    srtp_mos_score:    ['avg>=3.5'],
    sip_call_failure:  ['rate<0.02'],
  },
};

export default function () {
  const result = sip.call({
    target:    TARGET,
    duration:  DURATION,
    transport: TRANSPORT,
    srtp:      true,        // ← enable SRTP encryption
    audio:     { file: AUDIO },
  });

  const ok = check(result, {
    'SRTP call succeeded':     (r) => r && r.success,
    'SRTP MOS >= 3.5':         (r) => r && r.mos >= 3.5,
    'No excessive packet loss': (r) => r && (r.lost / (r.sent || 1)) < 0.02,
  });

  if (ok) {
    srtpSuccess.add(1);
    srtpMOS.add(result.mos);
  }
}
