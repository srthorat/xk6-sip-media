/**
 * Scenario 14 — PCAP Media Replay (Codec-Agnostic)
 * ==================================================
 * Replays RTP payloads captured in a .pcap file byte-for-byte.
 * Works with ANY codec present in the capture: G.729, AMR, G.722, T.38, etc.
 *
 * How to create a PCAP file:
 *   1. Make a real call  2. Capture with:
 *      tcpdump -i eth0 -w call.pcap udp portrange 16000-32000
 *   3. Use Wireshark to trim to the exact RTP stream
 *
 * Usage:
 *   SIP_TARGET="sip:ivr@pbx" PCAP_FILE=./captures/g729-ivr.pcap \
 *     ./k6 run scenarios/14_pcap_replay.js
 */
import sip from 'k6/x/sip';
import { check } from 'k6';

const TARGET    = __ENV.SIP_TARGET  || 'sip:ivr@192.168.1.100';
const PCAP_FILE = __ENV.PCAP_FILE   || './examples/audio/sample.pcap';

export const options = {
  scenarios: {
    pcap_replay: {
      executor:  'constant-vus',
      vus:       50,
      duration:  '3m',
    },
  },
  thresholds: {
    sip_call_success: ['count>0'],
    sip_call_failure: ['rate<0.02'],
    rtp_jitter_ms:    ['avg<30'],
  },
};

export default function () {
  const result = sip.call({
    target:    TARGET,
    audioMode: 'pcap',         // ← PCAP replay mode
    pcapFile:  PCAP_FILE,
    duration:  '20s',
  });

  check(result, {
    'PCAP call ok': (r) => r.success,
    'MOS >= 3.5':   (r) => r.mos >= 3.5,
  });
}
