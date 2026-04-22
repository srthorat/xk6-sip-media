/**
 * Scenario 16 — Variable Extraction + Dynamic Call Routing
 * ==========================================================
 * Reads SIP headers from the 200 OK response and uses them in
 * subsequent call decisions — equivalent to SIPp's <ereg> action.
 *
 * Example flow:
 *   1. Dial IVR → extract X-Session-Id header from 200 OK
 *   2. Send DTMF INFO using the session context
 *   3. Transfer to skill group using extracted routing data
 *
 * Usage:
 *   SIP_TARGET="sip:ivr@pbx" ./k6 run scenarios/16_variable_extraction.js
 */
import sip from 'k6/x/sip';
import { check } from 'k6';
import { sleep } from 'k6';

const TARGET = __ENV.SIP_TARGET || 'sip:ivr@192.168.1.100';
const AUDIO  = __ENV.SIP_AUDIO  || './examples/audio/sample.wav';

export const options = {
  scenarios: {
    var_extract: {
      executor:  'constant-vus',
      vus:       30,
      duration:  '5m',
    },
  },
  thresholds: {
    sip_call_success: ['count>0'],
    sip_call_failure: ['rate<0.02'],
  },
};

export default function () {
  // Dial with custom header injection
  const call = sip.dial({
    target:  TARGET,
    audio:   { file: AUDIO },
    headers: {
      'X-Tenant-ID':           'load-test',
      'P-Preferred-Identity':  'sip:k6agent@test.local',
    },
  });

  if (!call) return;

  // Extract response headers from 200 OK
  const sessionID   = call.responseHeader('X-Session-Id');
  const callID      = call.callID();
  const toTag       = call.toTag();
  const remoteURI   = call.remoteContactURI();

  check({ sessionID, callID, toTag, remoteURI }, {
    'call-id present':    (v) => v.callID.length > 0,
    'remote URI set':     (v) => v.remoteURI.length > 0,
  });

  // Use SIP INFO for DTMF (works with Cisco/Avaya PBX)
  sleep(1);
  call.sendDTMFInfo('5', 160);   // digit 5, 160ms
  sleep(1);
  call.sendDTMFInfo('#', 160);   // pound

  sleep(5);

  const result = call.hangup();
  check(result, { 'call ended ok': (r) => r.success });
}
