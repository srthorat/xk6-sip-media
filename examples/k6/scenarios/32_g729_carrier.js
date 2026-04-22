/**
 * Scenario 32 — G.729 Carrier-Grade Compressed Audio
 * ====================================================
 * Tests the G.729 codec — the most heavily deployed legacy codec for
 * international carrier trunks, traditional SBCs, and on-premise PBXs
 * (Cisco CUCM, Avaya, Genesys, older Asterisk deployments).
 *
 * Why G.729 matters for load testing:
 *   - 8:1 compression vs G.711 — 8Kbps vs 64Kbps per channel
 *   - An SBC handling 100k G.729 calls uses ~12x less bandwidth than G.711
 *   - The compressed bitstream stresses transcoding engines and DSP capacity
 *   - Many SBCs have a hard per-channel G.729 license cap — this finds it
 *
 * Codec details:
 *   - Algorithm: CELP (Code-Excited Linear Prediction)
 *   - Bitrate: 8Kbps
 *   - Packet size: 10 bytes per 10ms frame (vs 160 bytes for G.711)
 *   - Static RTP payload type: PT=18
 *   - Clock rate: 8kHz (same as G.711 — timestamp step is 160/20ms)
 *
 * IMPORTANT — Licensing notice:
 *   The G.729 algorithm patents expired January 1, 2017 (royalty-free).
 *   The bcg729 library used for encoding is licensed under GPLv3.
 *   Build this scenario only if your project is GPLv3-compatible, or
 *   if you have purchased a bcg729 commercial license from Belledonne.
 *   See: https://www.linphone.org/technical-corner/bcg729
 *
 *   Build with: CGO_ENABLED=1 xk6 build --with github.com/USER/xk6-sip-media=.
 *
 * Usage:
 *   SIP_TARGET="sip:carrier@sbc.example.com" \
 *   SIP_AUDIO=./examples/audio/sample.wav \
 *   ./k6 run scenarios/32_g729_carrier.js
 *
 * What to watch for in Grafana:
 *   - 'sip_mos' dropping below 3.5 → transcoding overload on the SBC
 *   - 'sip_packets_lost' spikes → DSP resource exhaustion
 *   - Frequent 503/486 → G.729 license pool exhausted on target SBC
 */
import sip from 'k6/x/sip';
import { check, sleep } from 'k6';
import { Counter, Trend, Rate } from 'k6/metrics';

const TARGET   = __ENV.SIP_TARGET || 'sip:ivr@192.168.1.100';
const AUDIO    = __ENV.SIP_AUDIO  || './examples/audio/sample.wav';
const VUS      = parseInt(__ENV.VUS || '50', 10);
const DURATION = __ENV.DURATION   || '5m';

// ── Custom Metrics ──────────────────────────────────────────────────────────
const g729CallSuccess  = new Counter('g729_call_success');
const g729MOS          = new Trend('g729_mos');
const g729PacketLoss   = new Trend('g729_packet_loss_pct');
const g729LicenseHit   = new Rate('g729_license_rejection');

export const options = {
  scenarios: {
    g729_carrier_load: {
      executor: 'constant-vus',
      vus:      VUS,
      duration: DURATION,
    },
  },
  thresholds: {
    // G.729 CELP compression introduces some quality loss vs PCM
    g729_mos:             ['avg>=3.5'],
    g729_packet_loss_pct: ['p(95)<3'],    // slightly higher tolerance for carrier trunks
    g729_license_rejection: ['rate<0.01'],
    sip_call_failure:     ['rate<0.02'],
  },
};

export default function () {
  const result = sip.call({
    target:   TARGET,
    duration: '20s',
    audio: {
      file:  AUDIO,
      codec: 'G729',   // ← G.729 8Kbps. Static PT=18.
    },
  });

  // Detect G.729 license pool exhaustion (SBC returns 503 when DSP channels full)
  const licenseRejected =
    result && result.error && result.error.includes('503');
  g729LicenseHit.add(licenseRejected ? 1 : 0);

  const ok = check(result, {
    'call succeeded':             (r) => r && r.success,
    'G.729 MOS >= 3.5':          (r) => r && r.mos >= 3.5,
    'packet loss < 3%':          (r) => r && r.packetLossPct < 3,
    'packets sent > 0':          (r) => r && r.sent > 0,
    'no license rejection (503)': () => !licenseRejected,
  });

  if (result && result.success) {
    g729CallSuccess.add(1);
    g729MOS.add(result.mos);
    g729PacketLoss.add(result.packetLossPct || 0);
  }

  sleep(1);
}
