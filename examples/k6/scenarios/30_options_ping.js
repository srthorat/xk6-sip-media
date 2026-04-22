/**
 * Scenario 30 — SIP OPTIONS Ping (Healthcheck)
 * =============================================
 * Validates the core routing and processing availability of a SIP Target
 * without negotiating actual calls.
 * 
 * Perfect for simulating "Keep-Alive" heartbeat architectures, where 
 * 100,000 idle endpoints ping the network every 30 seconds to ensure
 * NAT hole-punching and general IP availability.
 *
 * Usage:
 *   SIP_TARGET="sip:192.168.1.100" ./k6 run scenarios/30_options_ping.js
 */
import sip from 'k6/x/sip';
import { check, sleep } from 'k6';

const TARGET = __ENV.SIP_TARGET || 'sip:192.168.1.100';

export const options = {
  // Constant flow of SIP ping healthchecks
  executor: 'constant-arrival-rate',
  rate: 1000, // 1000 pings per second
  timeUnit: '1s',
  duration: '1m',
  preAllocatedVUs: 200,
  maxVUs: 500,

  thresholds: {
    sip_options_success: ['count>0'],
    sip_options_failure: ['count==0'],       // All pings must succeed
    // Make sure our ping response is fast! (Under 50ms)
    sip_options_rtt_ms:  ['p(95)<50', 'avg<10'], 
  },
};

export default function () {
  // Send a lightweight, connectionless OPTIONS ping
  const res = sip.options({
    target: TARGET,
    timeout: '2s', // Tight timeout for healthchecks
  });

  check(res, {
    'ping successful (200 OK)': (r) => r.success === true && r.status === 200,
    'latency is fast': (r) => r.rtt_ms < 50,
  });
}
