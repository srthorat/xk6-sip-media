/**
 * Soak / Endurance Tests
 * ======================
 * Detects slow resource leaks (FD, goroutines, memory, dialog state).
 *
 *  Run with --env SOAK_HOURS=1   → 1-hour soak  (50 VUs)
 *  Run with --env SOAK_HOURS=4   → 4-hour soak  (100 VUs)
 *
 * Usage (1-hour):
 *   SIP_TARGET="sip:ivr@pbx" SOAK_HOURS=1 ./k6 run scenarios/05_soak.js
 *
 * Usage (4-hour, requires k6 ≥ 0.43 for long-running support):
 *   SIP_TARGET="sip:ivr@pbx" SOAK_HOURS=4 ./k6 run scenarios/05_soak.js
 */
import sip from 'k6/x/sip';
import { check } from 'k6';
import { Trend } from 'k6/metrics';

const TARGET     = __ENV.SIP_TARGET  || 'sip:ivr@192.168.1.100';
const AUDIO_FILE = __ENV.SIP_AUDIO   || './examples/audio/sample.wav';
const HOURS      = parseFloat(__ENV.SOAK_HOURS || '1');
const DURATION   = `${Math.round(HOURS * 60)}m`;
const VUS        = HOURS <= 1 ? 50 : 100;

// Track MOS every 5 minutes to detect degradation trend
const mosOverTime = new Trend('mos_over_time');

export const options = {
  scenarios: {
    soak: {
      executor:  'constant-vus',
      vus:       VUS,
      duration:  DURATION,
    },
  },
  thresholds: {
    // Soak must be rock-solid — very low thresholds
    sip_call_failure: ['rate<0.001'],    // <0.1% failure
    mos_score:        ['avg>=3.8', 'min>=3.0'],
    rtp_jitter_ms:    ['avg<40', 'p(99)<100'],
    rtp_packets_lost: ['rate<0.005'],   // <0.5% packet loss
    // Setup time must stay stable — no SIP stack degradation
    sip_call_duration:['avg<2000', 'p(99)<4000'],
  },
};

// Bucket start time for MOS trend tagging
const startMs = Date.now();

export default function () {
  const result = sip.call({
    target:   TARGET,
    duration: '20s',
    audio:    { file: AUDIO_FILE },
    localIP:  '0.0.0.0',
  });

  // Tag MOS sample with 5-minute bucket for trend analysis
  const elapsedMin = Math.floor((Date.now() - startMs) / 60000);
  const bucket     = `${Math.floor(elapsedMin / 5) * 5}min`;
  mosOverTime.add(result.mos || 0, { bucket });

  check(result, {
    'no call failure':     (r) => r.success,
    'MOS stable >= 3.8':   (r) => r.mos >= 3.8,
    'no packet loss':      (r) => r.lost === 0,
    'jitter < 40ms':       (r) => r.jitter < 40,
  });

  if (!result.success) {
    console.error(`[soak ${bucket}] FAILURE: ${result.error}`);
  }
}
