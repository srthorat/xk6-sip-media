// Package rtp provides SRTP (Secure RTP, RFC 3711) encryption support.
//
// SRTP protects RTP payload confidentiality and integrity using AES-CM-128
// with HMAC-SHA1-80 authentication (the mandatory cipher suite per RFC 3711).
//
// Key derivation follows RFC 3711 §4.3: the master key and salt from the
// SDP crypto attribute are expanded into session keys via the KDF.
//
// SDP a=crypto line format (RFC 4568):
//
//	a=crypto:1 AES_CM_128_HMAC_SHA1_80 inline:<base64-key-salt>
//
// where <base64-key-salt> is 30 bytes = 16 (key) + 14 (salt), base64-encoded.
package rtp

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/hmac"
	"crypto/sha1" //nolint:gosec // SRTP mandates HMAC-SHA1 per RFC 3711
	"encoding/base64"
	"encoding/binary"
	"fmt"
	"net"
	"sync"
	"time"

	pionrtp "github.com/pion/rtp"
)

// SRTPConfig holds keying material parsed from an SDP a=crypto attribute.
type SRTPConfig struct {
	// MasterKey is the 16-byte AES-CM master key.
	MasterKey []byte

	// MasterSalt is the 14-byte master salt.
	MasterSalt []byte

	// Profile identifies the cipher suite.
	// Only "AES_CM_128_HMAC_SHA1_80" is supported.
	Profile string
}

// SRTPSession holds the per-SSRC SRTP cipher state for a single stream.
// It is safe for concurrent use via an internal mutex.
type SRTPSession struct {
	cfg     SRTPConfig
	ssrc    uint32
	mu      sync.Mutex
	index   uint64 // packet index (48-bit: 16-bit rollover count + 16-bit seq)
	lastSeq uint16
	encrypt bool // true = sender; false = receiver
}

// ParseSRTPConfig parses the inline key-salt from an SDP a=crypto value.
//
//	cfg, err := ParseSRTPConfig("inline:WVNfX19zZW1jdGwgKCkgewkyMjA7fQp9CnVubGVz")
func ParseSRTPConfig(inline string) (*SRTPConfig, error) {
	// Strip "inline:" prefix
	const prefix = "inline:"
	if len(inline) < len(prefix) {
		return nil, fmt.Errorf("srtp: invalid inline key (too short)")
	}
	b64 := inline[len(prefix):]

	// Strip any trailing pipe-delimited lifetime/MKI
	if idx := indexByte(b64, '|'); idx >= 0 {
		b64 = b64[:idx]
	}

	raw, err := base64.StdEncoding.DecodeString(b64)
	if err != nil {
		return nil, fmt.Errorf("srtp: decode key: %w", err)
	}
	if len(raw) != 30 {
		return nil, fmt.Errorf("srtp: key+salt must be 30 bytes, got %d", len(raw))
	}

	return &SRTPConfig{
		MasterKey:  raw[:16],
		MasterSalt: raw[16:30],
		Profile:    "AES_CM_128_HMAC_SHA1_80",
	}, nil
}

// NewSRTPSenderSession creates an SRTP session for the outbound (sender) direction.
func NewSRTPSenderSession(cfg *SRTPConfig, ssrc uint32) (*SRTPSession, error) {
	if err := validateConfig(cfg); err != nil {
		return nil, err
	}
	return &SRTPSession{cfg: *cfg, ssrc: ssrc, encrypt: true}, nil
}

// NewSRTPReceiverSession creates an SRTP session for the inbound (receiver) direction.
func NewSRTPReceiverSession(cfg *SRTPConfig, ssrc uint32) (*SRTPSession, error) {
	if err := validateConfig(cfg); err != nil {
		return nil, err
	}
	return &SRTPSession{cfg: *cfg, ssrc: ssrc, encrypt: false}, nil
}

// EncryptPacket authenticates and encrypts a raw RTP packet in-place.
// The returned slice is the protected SRTP packet (header + encrypted payload + auth tag).
func (s *SRTPSession) EncryptPacket(raw []byte) ([]byte, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	index := s.nextIndex(raw)
	return s.protect(raw, index)
}

// DecryptPacket verifies and decrypts an SRTP packet.
func (s *SRTPSession) DecryptPacket(srtp []byte) ([]byte, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if len(srtp) < 12+10 { // RTP header + auth tag
		return nil, fmt.Errorf("srtp: packet too short (%d bytes)", len(srtp))
	}

	// Strip auth tag (last 10 bytes)
	authData := srtp[:len(srtp)-10]
	authTag := srtp[len(srtp)-10:]

	// Parse sequence number from RTP header
	seq := binary.BigEndian.Uint16(srtp[2:4])
	index := s.estimateIndex(seq)

	// Derive session keys
	encKey, _, authKey, err := s.deriveKeys(index)
	if err != nil {
		return nil, err
	}

	// Verify auth tag
	mac := hmac.New(sha1.New, authKey) //nolint:gosec
	mac.Write(authData)
	expected := mac.Sum(nil)[:10]
	if !hmac.Equal(expected, authTag) {
		return nil, fmt.Errorf("srtp: auth tag mismatch")
	}

	// Decrypt payload (skip 12-byte header)
	header := srtp[:12]
	payload := make([]byte, len(srtp)-12-10)
	copy(payload, srtp[12:len(srtp)-10])

	ks := deriveIV(s.cfg.MasterSalt, uint32(s.ssrc), index)
	if err := aesCMCrypt(encKey, ks, payload); err != nil {
		return nil, err
	}

	result := make([]byte, len(header)+len(payload))
	copy(result, header)
	copy(result[len(header):], payload)
	return result, nil
}

// ── Internals ─────────────────────────────────────────────────────────────

func (s *SRTPSession) protect(raw []byte, index uint64) ([]byte, error) {
	encKey, _, authKey, err := s.deriveKeys(index)
	if err != nil {
		return nil, err
	}

	// Encrypt payload (AES-CM, payload starts at byte 12)
	enc := make([]byte, len(raw))
	copy(enc, raw)
	payload := enc[12:]

	ks := deriveIV(s.cfg.MasterSalt, uint32(s.ssrc), index)
	if err := aesCMCrypt(encKey, ks, payload); err != nil {
		return nil, err
	}

	// Append HMAC-SHA1-80 authentication tag
	mac := hmac.New(sha1.New, authKey) //nolint:gosec
	mac.Write(enc)
	tag := mac.Sum(nil)[:10] // truncate to 80 bits
	return append(enc, tag...), nil
}

// nextIndex computes the packet index from the current sequence counter.
func (s *SRTPSession) nextIndex(raw []byte) uint64 {
	var seq uint16
	if len(raw) >= 4 {
		seq = binary.BigEndian.Uint16(raw[2:4])
	}
	return s.estimateIndex(seq)
}

// estimateIndex reconstructs the 48-bit packet index (RFC 3711 §3.3.1).
func (s *SRTPSession) estimateIndex(seq uint16) uint64 {
	roc := s.index >> 16
	v := (roc << 16) | uint64(seq)
	// rollover check: if seq jumped backwards by more than 2^15, increment ROC
	diff := int32(seq) - int32(s.lastSeq)
	if diff < -0x8000 {
		v = ((roc + 1) << 16) | uint64(seq)
	}
	s.lastSeq = seq
	s.index = v
	return v
}

// deriveKeys derives session encryption, salting, and authentication keys
// from the master key and salt using AES-CM (RFC 3711 §4.3).
func (s *SRTPSession) deriveKeys(index uint64) (encKey, saltKey, authKey []byte, err error) {
	const (
		labelEnc  = 0x00
		labelAuth = 0x01
		labelSalt = 0x02
	)
	kdr := uint64(0) // key derivation rate = 0 (derive once)
	r := uint64(0)
	if kdr > 0 {
		r = index / kdr
	}

	encKey, err = deriveKey(s.cfg.MasterKey, s.cfg.MasterSalt, r, labelEnc, 16)
	if err != nil {
		return
	}
	saltKey, err = deriveKey(s.cfg.MasterKey, s.cfg.MasterSalt, r, labelSalt, 14)
	if err != nil {
		return
	}
	authKey, err = deriveKey(s.cfg.MasterKey, s.cfg.MasterSalt, r, labelAuth, 20)
	return
}

// deriveKey generates one session key using AES-CM key derivation (RFC 3711 §4.3.1).
func deriveKey(masterKey, masterSalt []byte, r uint64, label byte, length int) ([]byte, error) {
	// x = label_constant XOR (r << 8) XOR ...
	// Simplified: x = (label << 48) (with r=0)
	x := make([]byte, 16)
	// Apply salt XOR with label
	copy(x, masterSalt)
	x[7] ^= label
	// x[14] ^= byte(r >> 8); x[15] ^= byte(r) — only needed when kdr > 0

	block, err := aes.NewCipher(masterKey)
	if err != nil {
		return nil, fmt.Errorf("srtp: AES key expand: %w", err)
	}

	segments := (length + 15) / 16
	out := make([]byte, segments*16)
	ctr := make([]byte, 16)
	for i := 0; i < segments; i++ {
		binary.BigEndian.PutUint16(ctr[14:], uint16(i))
		iv := make([]byte, 16)
		for j := range iv {
			iv[j] = x[j] ^ ctr[j]
		}
		block.Encrypt(out[i*16:], iv)
	}
	return out[:length], nil
}

// deriveIV builds the AES-CM IV: 0x00000000 XOR (SSRC << 64) XOR (index << 16) XOR salt
// per RFC 3711 §4.1.1.
func deriveIV(salt []byte, ssrc uint32, index uint64) []byte {
	iv := make([]byte, 16)
	copy(iv, salt)
	// XOR SSRC into bytes 4-7
	iv[4] ^= byte(ssrc >> 24)
	iv[5] ^= byte(ssrc >> 16)
	iv[6] ^= byte(ssrc >> 8)
	iv[7] ^= byte(ssrc)
	// XOR packet index into bytes 8-13
	iv[8] ^= byte(index >> 40)
	iv[9] ^= byte(index >> 32)
	iv[10] ^= byte(index >> 24)
	iv[11] ^= byte(index >> 16)
	iv[12] ^= byte(index >> 8)
	iv[13] ^= byte(index)
	return iv
}

// aesCMCrypt performs AES Counter Mode encryption/decryption in-place.
func aesCMCrypt(key, iv, data []byte) error {
	block, err := aes.NewCipher(key)
	if err != nil {
		return fmt.Errorf("srtp: AES cipher: %w", err)
	}
	stream := cipher.NewCTR(block, iv)
	stream.XORKeyStream(data, data)
	return nil
}

func validateConfig(cfg *SRTPConfig) error {
	if cfg == nil {
		return fmt.Errorf("srtp: config is nil")
	}
	if len(cfg.MasterKey) != 16 {
		return fmt.Errorf("srtp: master key must be 16 bytes, got %d", len(cfg.MasterKey))
	}
	if len(cfg.MasterSalt) != 14 {
		return fmt.Errorf("srtp: master salt must be 14 bytes, got %d", len(cfg.MasterSalt))
	}
	return nil
}

func indexByte(s string, c byte) int {
	for i := 0; i < len(s); i++ {
		if s[i] == c {
			return i
		}
	}
	return -1
}

// ── SRTP-aware Stream ─────────────────────────────────────────────────────

// StreamSRTP sends pre-encoded payloads as SRTP packets (encrypted + authenticated).
// Functionally equivalent to Stream() but wraps each packet with AES-CM-128-HMAC-SHA1-80.
func StreamSRTP(sess *Session, srtp *SRTPSession, payloads [][]byte, pt uint8, stats *SendStats, stop <-chan struct{}) {
	ticker := time.NewTicker(20 * time.Millisecond)
	defer ticker.Stop()

	for _, payload := range payloads {
		select {
		case <-stop:
			return
		case <-ticker.C:
		}

		seq, ts := sess.NextSeqTS(160)

		pkt := &pionrtp.Packet{
			Header: pionrtp.Header{
				Version:        2,
				PayloadType:    pt,
				SequenceNumber: seq,
				Timestamp:      ts,
				SSRC:           sess.SSRC,
			},
			Payload: payload,
		}

		raw, err := pkt.Marshal()
		if err != nil {
			continue
		}

		encrypted, err := srtp.EncryptPacket(raw)
		if err != nil {
			continue
		}

		if err := sess.Send(encrypted); err == nil {
			stats.PacketsSent++
		}
	}
}

// ReceiveSRTP reads SRTP packets, decrypts them, and updates stats.
func ReceiveSRTP(conn *net.UDPConn, srtp *SRTPSession, stats *RTPStats, recorder *AudioRecorder, stop <-chan struct{}) {
	buf := make([]byte, 1500)

	for {
		select {
		case <-stop:
			return
		default:
		}

		_ = conn.SetReadDeadline(time.Now().Add(time.Second))
		n, _, err := conn.ReadFrom(buf)
		if err != nil {
			if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
				continue
			}
			continue
		}

		decrypted, err := srtp.DecryptPacket(buf[:n])
		if err != nil {
			continue // auth failure — drop silently
		}

		var pkt pionrtp.Packet
		if err := pkt.Unmarshal(decrypted); err != nil {
			continue
		}

		arrival := time.Now()
		stats.update(pkt.SequenceNumber, arrival)

		if recorder != nil && len(pkt.Payload) > 0 {
			recorder.Write(pkt.Payload)
		}
	}
}
