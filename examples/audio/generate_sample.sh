#!/usr/bin/env bash
# generate_sample.sh — Generate test audio files for xk6-sip-media
#
# Requirements: ffmpeg (brew install ffmpeg / apt-get install ffmpeg)
#
# Generates multiple test audio formats:
#   sample.wav     — 8kHz mono 16-bit PCM   (G.711 PCMU/PCMA, direct load)
#   sample_hd.wav  — 16kHz mono 16-bit PCM  (G.722 wideband, direct load)
#   sample_44k.wav — 44.1kHz stereo 16-bit  (auto-resampled + downmixed)
#   sample.mp3     — 128kbps stereo MP3      (auto-decoded + resampled)
#   hold_music.mp3 — 30s music-like tone     (use as hold MOH file)
#
# xk6-sip-media detects format by magic bytes and automatically:
#   - resamples any sample rate → 8kHz (G.711) or 16kHz (G.722)
#   - downmixes stereo → mono
#   No pre-processing required.

set -euo pipefail

DURATION="${DURATION:-30}"

if ! command -v ffmpeg &>/dev/null; then
  echo "Error: ffmpeg not found. Install with: brew install ffmpeg"
  exit 1
fi

echo "==> Generating 8kHz WAV (PCMU/PCMA ready)"
ffmpeg -y -f lavfi -i "sine=frequency=1000:duration=${DURATION}" \
  -ar 8000 -ac 1 -acodec pcm_s16le sample.wav

echo "==> Generating 16kHz WAV (G.722 wideband ready)"
ffmpeg -y -f lavfi -i "sine=frequency=440:duration=${DURATION}" \
  -ar 16000 -ac 1 -acodec pcm_s16le sample_hd.wav

echo "==> Generating 44.1kHz stereo WAV (auto-resample test)"
ffmpeg -y -f lavfi -i "sine=frequency=800:duration=${DURATION}" \
  -ar 44100 -ac 2 -acodec pcm_s16le sample_44k.wav

echo "==> Generating 128kbps MP3 (auto-decode test)"
ffmpeg -y -f lavfi -i "sine=frequency=600:duration=${DURATION}" \
  -ar 44100 -ac 2 -b:a 128k sample.mp3

echo "==> Generating hold music MP3 (multi-tone)"
ffmpeg -y \
  -f lavfi -i "sine=frequency=330:duration=${DURATION}[s1]" \
  -f lavfi -i "sine=frequency=440:duration=${DURATION}[s2]" \
  -filter_complex "[0][1]amix=inputs=2:duration=first" \
  -ar 44100 -ac 2 -b:a 128k hold_music.mp3 2>/dev/null || \
  ffmpeg -y -f lavfi -i "sine=frequency=330:duration=${DURATION}" \
    -ar 44100 -ac 2 -b:a 128k hold_music.mp3

echo ""
echo "==> Done! Files generated:"
for f in sample.wav sample_hd.wav sample_44k.wav sample.mp3 hold_music.mp3; do
  if [ -f "$f" ]; then
    SIZE=$(stat -f%z "$f" 2>/dev/null || stat -c%s "$f")
    echo "    $f  ($SIZE bytes)"
  fi
done

echo ""
echo "==> Usage in k6 scripts:"
echo "    // WAV (8kHz, used as-is)"
echo "    sip.call({ audio: { file: './examples/audio/sample.wav' }, ... })"
echo ""
echo "    // MP3 (any sample rate — auto-decoded and resampled)"
echo "    sip.call({ audio: { file: './examples/audio/hold_music.mp3' }, ... })"
echo ""
echo "    // 44kHz stereo WAV (auto-resampled to 8kHz mono)"
echo "    sip.call({ audio: { file: './examples/audio/sample_44k.wav' }, ... })"
echo ""
echo "    // G.722 wideband"
echo "    sip.call({ audio: { file: './examples/audio/sample_hd.wav', codec: 'G722' }, ... })"
