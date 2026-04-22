/**
 * Scenario 31 — Opus Ultra-Wideband (WebRTC / Discord-style HD Audio)
 * =====================================================================
 * Tests the Opus codec at 48kHz stereo, the standard for WebRTC, Discord,
 * Microsoft Teams, Zoom, and all modern SIP contact centers that support
 * browser-based WebRTC gateways.
 *
 * Key differences from legacy codecs:
 *   - Dynamic payload type (PT=111 by default, negotiated via a=rtpmap)
 *   - RTP timestamp increments by 960 per 20ms frame (48kHz clock rate)
 *   - Audio quality up to 20kHz — fundamentally better than G.711/G.722
 *   - The engine dynamically picks up the PT from the 200 OK SDP answer
 *
 * SDP offer sent:
 *   m=audio <port> RTP/AVP 111
 *   a=rtpmap:111 opus/48000/2
 *   a=ptime:20
 *
 * Expected: SBC/PBX negotiates Opus in the 200 OK answer, confirming the
 * dynamic SDP payload negotiation (Phase 1) is working correctly.
 *
 * Use this scenario to:
 *   1. Validate Opus codec negotiation against a modern SBC (Kamailio, FreeSWITCH)
 *   2. Load-test WebRTC-to-SIP gateways (Janus, Asterisk WebRTC)
 *   3. Benchmark MOS scores on HD audio versus G.711 under the same load
 *
 * Usage:
 *   SIP_TARGET="sip:webrtc@pbx" \
 *   SIP_AUDIO=./examples/audio/sample_48k.wav \
 *   ./k6 run scenarios/31_opus_webrtc.js
 *
 * Generate 48kHz audio:
 *   ffmpeg -f lavfi -i "sine=frequency=440:duration=30" \
 *     -ar 48000 -ac 1 examples/audio/sample_48k.wav
 */
import sip from 'k6/x/sip';
import { check, sleep } from 'k6';
import { Counter, Trend, Rate } from 'k6/metrics';

const TARGET   = __ENV.SIP_TARGET || 'sip:ivr@192.168.1.100';
const AUDIO    = __ENV.SIP_AUDIO  || './examples/audio/sample_48k.wav';
const VUS      = parseInt(__ENV.VUS || '20', 10);
const DURATION = __ENV.DURATION   || '5m';

// ── Custom Metrics ──────────────────────────────────────────────────────────
const opusCallSuccess  = new Counter('opus_call_success');
const opusMOS          = new Trend('opus_mos');
const opusPacketLoss   = new Trend('opus_packet_loss_pct');
const opusNegotiationOK = new Rate('opus_negotiation_ok');

export const options = {
  scenarios: {
    opus_load: {
      executor: 'constant-vus',
      vus:      VUS,
      duration: DURATION,
    },
  },
  thresholds: {
    // Opus should achieve high MOS; its adaptive bitrate handles loss well
    opus_mos:              ['avg>=3.8'],
    opus_packet_loss_pct:  ['p(95)<2'],    // <2% loss at p95
    opus_negotiation_ok:   ['rate>=0.99'], // ≥99% of calls must negotiate Opus
    sip_call_failure:      ['rate<0.01'],
  },
};

export default function () {
  const result = sip.call({
    target:   TARGET,
    duration: '20s',
    audio: {
      file:  AUDIO,
      codec: 'OPUS',   // ← Opus 48kHz. Engine auto-negotiates dynamic PT from SDP.
    },
  });

  const negotiatedOpus = check(result, {
    'call succeeded':              (r) => r && r.success,
    'Opus PT negotiated (no err)': (r) => r && !r.error,
    'MOS >= 3.8 (HD quality)':    (r) => r && r.mos >= 3.8,
    'packet loss < 2%':           (r) => r && r.packetLossPct < 2,
    'packets sent > 0':           (r) => r && r.sent > 0,
  });

  opusNegotiationOK.add(negotiatedOpus ? 1 : 0);

  if (result && result.success) {
    opusCallSuccess.add(1);
    opusMOS.add(result.mos);
    opusPacketLoss.add(result.packetLossPct || 0);
  }

  sleep(1);
}
