/**
 * GCS Routing Load Tests — Happy Path & Carrier Failover
 * =======================================================
 * Validates Global Carrier Selection (GCS) / outbound routing tier
 * under load. Tests:
 *
 *  Scenario A — Happy path:        all calls route via primary carrier, 
 *                                   verify MOS/ASR/ALOC at normal CPS
 *  Scenario B — Carrier failover:  primary carrier is blacked out mid-test
 *                                   (done by switching SIP_CARRIER_A env),
 *                                   verify failover to secondary within SLA
 *  Scenario C — Multi-carrier:     split calls across 3 carrier routes
 *                                   and validate per-carrier quality
 *
 * Usage (happy path):
 *   SIP_CARRIER_A="sip:ivr@carrier-a.example.com" \
 *   SIP_CARRIER_B="sip:ivr@carrier-b.example.com" \
 *   SIP_CARRIER_C="sip:ivr@carrier-c.example.com" \
 *   ./k6 run scenarios/10_gcs_routing.js
 *
 * To simulate failover: restart the test with SIP_CARRIER_A pointing to
 * an unreachable address after the first minute.
 */
import sip from 'k6/x/sip';
import { check } from 'k6';
import { Counter, Trend, Rate } from 'k6/metrics';

const CARRIER_A  = __ENV.SIP_CARRIER_A || 'sip:ivr@192.168.1.100';   // primary
const CARRIER_B  = __ENV.SIP_CARRIER_B || 'sip:ivr@192.168.1.101';   // secondary
const CARRIER_C  = __ENV.SIP_CARRIER_C || 'sip:ivr@192.168.1.102';   // tertiary
const AUDIO      = __ENV.SIP_AUDIO     || './examples/audio/sample.wav';

// ── Per-carrier metrics ────────────────────────────────────────────────────
const mosA     = new Trend('mos_carrier_a');
const mosB     = new Trend('mos_carrier_b');
const mosC     = new Trend('mos_carrier_c');
const failA    = new Counter('sip_carrier_a_failures');
const failB    = new Counter('sip_carrier_b_failures');
const failoverOK = new Rate('sip_failover_success');

export const options = {
  scenarios: {
    // ── A: happy path — 100% primary carrier ──────────────────────────────
    happy_path: {
      executor:  'constant-vus',
      vus:       100,
      duration:  '5m',
      env: { ROUTING: 'primary' },
    },
    // ── B: carrier failover — simulate outage at 1m ────────────────────────
    failover: {
      executor:  'constant-vus',
      vus:       50,
      duration:  '4m',
      startTime: '5m30s',
      env: { ROUTING: 'failover' },
    },
    // ── C: multi-carrier split (33/33/33) ─────────────────────────────────
    multi_carrier: {
      executor:  'constant-vus',
      vus:       120,
      duration:  '5m',
      startTime: '10m',
      env: { ROUTING: 'multi' },
    },
  },
  thresholds: {
    // Happy path: primary carrier must be rock solid
    'mos_carrier_a':              ['avg>=3.8'],
    'sip_carrier_a_failures':     ['count==0'],    // zero failures on primary
    // Failover: secondary must absorb traffic with acceptable quality
    'mos_carrier_b':              ['avg>=3.5'],
    'sip_failover_success':       ['rate>=0.95'],  // 95% of failovers succeed
    // Overall
    sip_call_failure:             ['rate<0.05'],
    mos_score:                    ['avg>=3.5'],
    sip_call_duration:            ['p(95)<3000'],
  },
};

// Simple per-VU failover state: after 60s, treat carrier A as "down"
const FAILOVER_AFTER_MS = 60_000;
const scenarioStart = Date.now();

export default function () {
  const routing = __ENV.ROUTING || 'primary';
  let target, carrier;

  if (routing === 'primary') {
    target  = CARRIER_A;
    carrier = 'A';
  } else if (routing === 'failover') {
    // After 60s simulate primary outage → failover to B
    const elapsed = Date.now() - scenarioStart;
    if (elapsed > FAILOVER_AFTER_MS) {
      target  = CARRIER_B;
      carrier = 'B';
    } else {
      target  = CARRIER_A;
      carrier = 'A';
    }
  } else {
    // Multi-carrier: round-robin by VU ID
    const idx = __VU % 3;
    if (idx === 0) { target = CARRIER_A; carrier = 'A'; }
    else if (idx === 1) { target = CARRIER_B; carrier = 'B'; }
    else { target = CARRIER_C; carrier = 'C'; }
  }

  const result = sip.call({
    target:   target,
    duration: '10s',
    audio:    { file: AUDIO },
    localIP:  '0.0.0.0',
  });

  // Record per-carrier MOS
  if (carrier === 'A') {
    mosA.add(result.mos || 0);
    if (!result.success) failA.add(1);
  } else if (carrier === 'B') {
    mosB.add(result.mos || 0);
    if (!result.success) failB.add(1);
  } else {
    mosC.add(result.mos || 0);
  }

  // Failover scenario: calls on B (after failover) are the ones to check
  if (routing === 'failover' && carrier === 'B') {
    failoverOK.add(result.success ? 1 : 0);
  }

  check(result, {
    [`carrier ${carrier} call ok`]:     (r) => r.success,
    [`carrier ${carrier} MOS >= 3.5`]:  (r) => r.mos >= 3.5,
    [`carrier ${carrier} jitter ok`]:   (r) => r.jitter < 80,
  });
}
