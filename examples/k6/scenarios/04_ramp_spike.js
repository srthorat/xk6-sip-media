/**
 * Ramp-up / Spike Tests
 * =====================
 * Three profiles in one run:
 *
 *  A — Gradual ramp:   0 → 200 VUs over 10 min, hold 5 min, ramp down
 *  B — 200% spike:     baseline 50 VUs → instant spike to 150 VUs → back
 *  C — Burst:          20 rapid bursts of 30 VUs, each 10s on / 5s off
 *
 * Usage:
 *   SIP_TARGET="sip:ivr@pbx" ./k6 run scenarios/04_ramp_spike.js
 */
import sip from 'k6/x/sip';
import { check, sleep } from 'k6';

const TARGET     = __ENV.SIP_TARGET || 'sip:ivr@192.168.1.100';
const AUDIO_FILE = __ENV.SIP_AUDIO  || './examples/audio/sample.wav';

export const options = {
  scenarios: {
    // ── A: gradual ramp ────────────────────────────────────────────────────
    gradual_ramp: {
      executor: 'ramping-vus',
      startVUs: 0,
      stages: [
        { duration: '2m',  target: 50  },
        { duration: '4m',  target: 100 },
        { duration: '4m',  target: 200 },
        { duration: '5m',  target: 200 },   // hold at peak
        { duration: '2m',  target: 50  },
        { duration: '1m',  target: 0   },
      ],
      gracefulRampDown: '30s',
    },

    // ── B: 200% spike ──────────────────────────────────────────────────────
    spike_200pct: {
      executor:  'ramping-vus',
      startVUs:  50,
      startTime: '19m',
      stages: [
        { duration: '30s', target: 50  },   // baseline 50 VUs
        { duration: '10s', target: 150 },   // instant spike to 150 (200%)
        { duration: '3m',  target: 150 },   // sustain spike
        { duration: '10s', target: 50  },   // recover
        { duration: '1m',  target: 50  },   // observe recovery
        { duration: '30s', target: 0   },
      ],
      gracefulRampDown: '30s',
    },

    // ── C: burst pattern ───────────────────────────────────────────────────
    burst: {
      executor:   'ramping-arrival-rate',
      startTime:  '26m',
      startRate:  0,
      timeUnit:   '1s',
      stages: [
        // 20 burst cycles: 10s burst at 30 CPS, 5s quiet
        { duration: '10s', target: 30 }, { duration: '5s', target: 0 },
        { duration: '10s', target: 30 }, { duration: '5s', target: 0 },
        { duration: '10s', target: 30 }, { duration: '5s', target: 0 },
        { duration: '10s', target: 30 }, { duration: '5s', target: 0 },
        { duration: '10s', target: 30 }, { duration: '5s', target: 0 },
        { duration: '10s', target: 30 }, { duration: '5s', target: 0 },
        { duration: '10s', target: 30 }, { duration: '5s', target: 0 },
        { duration: '10s', target: 30 }, { duration: '5s', target: 0 },
        { duration: '10s', target: 30 }, { duration: '5s', target: 0 },
        { duration: '10s', target: 30 }, { duration: '5s', target: 0 },
      ],
      preAllocatedVUs: 50,
      maxVUs:          200,
    },
  },
  thresholds: {
    sip_call_failure:  ['rate<0.05'],   // <5% failure across all spikes
    mos_score:         ['avg>=3.0'],
    rtp_jitter_ms:     ['p(95)<150'],   // allow higher jitter under spike
    sip_call_duration: ['p(99)<5000'],  // INVITE→200 must complete in 5s
  },
};

export default function () {
  const result = sip.call({
    target:   TARGET,
    duration: '10s',
    audio:    { file: AUDIO_FILE },
    localIP:  '0.0.0.0',
  });

  check(result, {
    'call ok':      (r) => r.success,
    'MOS >= 3.0':   (r) => r.mos >= 3.0,
  });
}
