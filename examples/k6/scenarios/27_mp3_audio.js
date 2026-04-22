/**
 * Scenario 27 — MP3 Audio Playback
 * ===================================
 * Tests playback of MP3 audio files in SIP calls.
 * xk6 auto-decodes MP3 → PCM16 → resamples to 8kHz → encodes PCMU.
 * No external tools required (pure Go MP3 decoder).
 *
 * Use-cases:
 *   - IVR hold music from MP3 library files
 *   - Pre-recorded announcements from MP3
 *   - Testing audio quality pipeline (lossy→lossless→G.711→RTP)
 *
 * Usage:
 *   SIP_TARGET="sip:ivr@pbx" \
 *   SIP_AUDIO=./examples/audio/hold_music.mp3 \
 *   ./k6 run scenarios/27_mp3_audio.js
 *
 * Generate test MP3:
 *   cd examples/audio && bash generate_sample.sh
 */
import sip from 'k6/x/sip';
import { check, sleep } from 'k6';
import { Counter, Trend } from 'k6/metrics';

const TARGET = __ENV.SIP_TARGET || 'sip:ivr@192.168.1.100';
const AUDIO  = __ENV.SIP_AUDIO  || './examples/audio/hold_music.mp3';

const mp3OK  = new Counter('mp3_call_success');
const mp3MOS = new Trend('mp3_mos');

export const options = {
  scenarios: {
    mp3_load: {
      executor:  'constant-vus',
      vus:       25,
      duration:  '5m',
    },
  },
  thresholds: {
    mp3_call_success: ['count>0'],
    mp3_mos:          ['avg>=3.0'],  // MP3→G.711 conversion loses some quality
    sip_call_failure: ['rate<0.02'],
  },
};

export default function () {
  const result = sip.call({
    target:   TARGET,
    duration: '20s',
    audio: {
      file:  AUDIO,   // MP3 auto-detected by magic bytes (ID3 header)
      codec: 'PCMU',  // target codec (auto-resampled from MP3's 44.1kHz)
    },
  });

  const ok = check(result, {
    'MP3 call succeeded':     (r) => r && r.success,
    'MP3 decoded + sent':     (r) => r && r.sent > 0,
    'MOS acceptable':         (r) => r && r.mos >= 3.0,
  });

  if (ok) {
    mp3OK.add(1);
    mp3MOS.add(result.mos || 0);
  }

  sleep(1);
}
