# xk6-sip-media Audio Files

This directory contains test audio files for use in k6 SIP load tests.

## Generate all test files

```bash
# Requires ffmpeg (brew install ffmpeg / apt install ffmpeg)
bash generate_sample.sh
```

## What's generated

| File | Format | Rate | Channels | Use |
|---|---|---|---|---|
| `sample.wav` | WAV PCM | 8 kHz | Mono | Native telephony, zero processing |
| `sample_hd.wav` | WAV PCM | 16 kHz | Mono | G.722 wideband HD voice |
| `sample_44k.wav` | WAV PCM | 44.1 kHz | Stereo | Auto-resample + downmix test |
| `sample.mp3` | MP3 128kbps | 44.1 kHz | Stereo | MP3 auto-decode test |
| `hold_music.mp3` | MP3 128kbps | 44.1 kHz | Stereo | Realistic hold music scenario |

## Format auto-detection

xk6-sip-media **detects format by magic bytes**, not file extension:

- `RIFF....WAVE` header → WAV decoder
- `ID3` tag or `0xFF 0xFB` sync word → MP3 decoder
- `.pcap` global header → PCAP RTP replay

## Supported conversions (all automatic)

```
Input WAV (any rate, any channels)
  → resample to 8kHz or 16kHz (linear interpolation)
  → downmix to mono (channel average)
  → encode with selected codec (PCMU / PCMA / G.722)

Input MP3 (any bitrate, any rate, any channels)
  → decode to PCM16 (pure Go, no CGO)
  → resample + downmix
  → encode with selected codec

Input PCAP
  → extract UDP/RTP payloads byte-for-byte
  → replay at original timestamps
  → no codec decoding required (G.729, AMR, T.38 work transparently)
```

## Usage in k6

```javascript
import sip from 'k6/x/sip';

// WAV — native 8kHz (zero processing)
sip.call({ audio: { file: './examples/audio/sample.wav' }, ... });

// MP3 — auto-decoded and resampled
sip.call({ audio: { file: './examples/audio/hold_music.mp3' }, ... });

// Any WAV — auto-resampled to 8kHz mono
sip.call({ audio: { file: './examples/audio/sample_44k.wav' }, ... });

// G.722 wideband (16kHz)
sip.call({ audio: { file: './examples/audio/sample_hd.wav', codec: 'G722' }, ... });

// PCAP replay (any codec, byte-accurate)
sip.call({ audioMode: 'pcap', pcapFile: './captures/g729-call.pcap', ... });

// Echo mode (no file needed — reflects packets back)
sip.call({ audioMode: 'echo', target: TARGET });
```

## Capturing real call audio for PCAP replay

```bash
# Capture on the load generator during a real call
sudo tcpdump -i eth0 -w real_call.pcap udp portrange 16000-32000

# Or capture on the SBC/PBX (all call legs)
sudo tcpdump -i any -w all_calls.pcap udp

# Trim to a single RTP stream with Wireshark:
# Telephony → RTP → RTP Streams → select stream → Save As → .pcap
```
