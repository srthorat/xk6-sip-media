package rtp

import (
	"bytes"
	"crypto/rand"
	"testing"
	"time"
)

// generateSRTPKey returns 30 random bytes (16-byte key + 14-byte salt).
func generateSRTPKey(t *testing.T) []byte {
	t.Helper()
	key := make([]byte, 30)
	_, err := rand.Read(key)
	if err != nil {
		t.Fatal(err)
	}
	return key
}

func TestParseSRTPConfig_Valid(t *testing.T) {
	// Known 30-byte key: all zeros, base64-encoded
	zeros := make([]byte, 30)
	import64 := encodeBase64(zeros)

	cfg, err := ParseSRTPConfig("inline:" + import64)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if len(cfg.MasterKey) != 16 {
		t.Errorf("key length: want 16, got %d", len(cfg.MasterKey))
	}
	if len(cfg.MasterSalt) != 14 {
		t.Errorf("salt length: want 14, got %d", len(cfg.MasterSalt))
	}
}

func TestParseSRTPConfig_TooShort(t *testing.T) {
	_, err := ParseSRTPConfig("inline:aGVsbG8=") // "hello" = 5 bytes, not 30
	if err == nil {
		t.Fatal("expected error for short key")
	}
}

func TestParseSRTPConfig_MissingPrefix(t *testing.T) {
	_, err := ParseSRTPConfig("base64only")
	if err == nil {
		t.Fatal("expected error for missing inline: prefix")
	}
}

func TestSRTPEncryptDecrypt_RoundTrip(t *testing.T) {
	keyBytes := generateSRTPKey(t)
	cfg := &SRTPConfig{
		MasterKey:  keyBytes[:16],
		MasterSalt: keyBytes[16:],
		Profile:    "AES_CM_128_HMAC_SHA1_80",
	}

	const ssrc = 0xDEADBEEF

	sender, err := NewSRTPSenderSession(cfg, ssrc)
	if err != nil {
		t.Fatal(err)
	}
	receiver, err := NewSRTPReceiverSession(cfg, ssrc)
	if err != nil {
		t.Fatal(err)
	}

	// Build a minimal RTP packet manually
	rtp := buildMinimalRTP(ssrc, 42, 160*5, 0, []byte("hello world pcmu"))

	encrypted, err := sender.EncryptPacket(rtp)
	if err != nil {
		t.Fatalf("encrypt: %v", err)
	}

	if len(encrypted) != len(rtp)+10 {
		t.Errorf("encrypted length: want %d, got %d", len(rtp)+10, len(encrypted))
	}

	decrypted, err := receiver.DecryptPacket(encrypted)
	if err != nil {
		t.Fatalf("decrypt: %v", err)
	}

	if string(decrypted[12:]) != "hello world pcmu" {
		t.Errorf("decrypted payload mismatch: %q", decrypted[12:])
	}
}

func TestSRTPDecrypt_TamperedPacket(t *testing.T) {
	keyBytes := generateSRTPKey(t)
	cfg := &SRTPConfig{
		MasterKey:  keyBytes[:16],
		MasterSalt: keyBytes[16:],
		Profile:    "AES_CM_128_HMAC_SHA1_80",
	}

	sender, _ := NewSRTPSenderSession(cfg, 1)
	receiver, _ := NewSRTPReceiverSession(cfg, 1)

	rtp := buildMinimalRTP(1, 1, 160, 0, []byte("test"))
	encrypted, _ := sender.EncryptPacket(rtp)

	// Flip a bit in the payload
	encrypted[15] ^= 0xFF

	_, err := receiver.DecryptPacket(encrypted)
	if err == nil {
		t.Fatal("expected auth failure for tampered packet")
	}
}

func TestSRTPConfig_InvalidKey(t *testing.T) {
	cfg := &SRTPConfig{
		MasterKey:  make([]byte, 8), // wrong: 8 bytes instead of 16
		MasterSalt: make([]byte, 14),
	}
	_, err := NewSRTPSenderSession(cfg, 1)
	if err == nil {
		t.Fatal("expected error for invalid key size")
	}
}

func TestDeriveIV_Deterministic(t *testing.T) {
	salt := make([]byte, 14)
	iv1 := deriveIV(salt, 0x1234, 42)
	iv2 := deriveIV(salt, 0x1234, 42)
	for i := range iv1 {
		if iv1[i] != iv2[i] {
			t.Errorf("IV not deterministic at byte %d", i)
		}
	}
}

func TestNTPConversion(t *testing.T) {
	now := time.Now()
	sec, frac := toNTP(now)
	if sec < 3900000000 { // sanity: 2023-era NTP timestamp
		t.Errorf("NTP seconds seems wrong: %d", sec)
	}
	_ = frac
}

// ── helpers ─────────────────────────────────────────────────────────────────

func buildMinimalRTP(ssrc uint32, seq uint16, ts uint32, pt uint8, payload []byte) []byte {
	pkt := make([]byte, 12+len(payload))
	pkt[0] = 0x80 // V=2
	pkt[1] = pt
	pkt[2] = byte(seq >> 8)
	pkt[3] = byte(seq)
	pkt[4] = byte(ts >> 24)
	pkt[5] = byte(ts >> 16)
	pkt[6] = byte(ts >> 8)
	pkt[7] = byte(ts)
	pkt[8] = byte(ssrc >> 24)
	pkt[9] = byte(ssrc >> 16)
	pkt[10] = byte(ssrc >> 8)
	pkt[11] = byte(ssrc)
	copy(pkt[12:], payload)
	return pkt
}

func encodeBase64(data []byte) string {
	const table = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789+/"
	n := len(data)
	out := make([]byte, ((n+2)/3)*4)
	j := 0
	for i := 0; i < n; i += 3 {
		b0 := data[i]
		b1 := byte(0)
		b2 := byte(0)
		if i+1 < n {
			b1 = data[i+1]
		}
		if i+2 < n {
			b2 = data[i+2]
		}
		out[j] = table[b0>>2]
		out[j+1] = table[((b0&3)<<4)|b1>>4]
		out[j+2] = table[((b1&0xF)<<2)|b2>>6]
		out[j+3] = table[b2&0x3F]
		j += 4
	}
	if n%3 == 1 {
		out[j-1] = '='
		out[j-2] = '='
	} else if n%3 == 2 {
		out[j-1] = '='
	}
	return string(out)
}

// ── Key-caching tests ─────────────────────────────────────────────────────────

// TestSRTPSession_KeysCachedAtInit verifies that encKey, saltKey, authorKey, and
// encBlock are populated immediately after NewSRTPSenderSession (not nil/zero).
func TestSRTPSession_KeysCachedAtInit(t *testing.T) {
	keyBytes := generateSRTPKey(t)
	cfg := &SRTPConfig{
		MasterKey:  keyBytes[:16],
		MasterSalt: keyBytes[16:],
		Profile:    "AES_CM_128_HMAC_SHA1_80",
	}

	sess, err := NewSRTPSenderSession(cfg, 0xDEADBEEF)
	if err != nil {
		t.Fatalf("NewSRTPSenderSession: %v", err)
	}

	if len(sess.encKey) == 0 {
		t.Error("encKey not cached: len=0")
	}
	if len(sess.saltKey) == 0 {
		t.Error("saltKey not cached: len=0")
	}
	if len(sess.authorKey) == 0 {
		t.Error("authorKey not cached: len=0")
	}
	if sess.encBlock == nil {
		t.Error("encBlock not cached: nil")
	}
}

// TestSRTPSession_ReceiverKeysCachedAtInit does the same for the receiver side.
func TestSRTPSession_ReceiverKeysCachedAtInit(t *testing.T) {
	keyBytes := generateSRTPKey(t)
	cfg := &SRTPConfig{
		MasterKey:  keyBytes[:16],
		MasterSalt: keyBytes[16:],
		Profile:    "AES_CM_128_HMAC_SHA1_80",
	}

	sess, err := NewSRTPReceiverSession(cfg, 1)
	if err != nil {
		t.Fatalf("NewSRTPReceiverSession: %v", err)
	}

	if sess.encBlock == nil {
		t.Error("receiver encBlock not cached: nil")
	}
}

// TestSRTPSession_CachedKeysSameAsDerived verifies that the cached keys match
// what deriveKeys(0) would compute — i.e., caching is correct, not fast but wrong.
func TestSRTPSession_CachedKeysSameAsDerived(t *testing.T) {
	keyBytes := generateSRTPKey(t)
	cfg := &SRTPConfig{
		MasterKey:  keyBytes[:16],
		MasterSalt: keyBytes[16:],
		Profile:    "AES_CM_128_HMAC_SHA1_80",
	}

	sess, err := NewSRTPSenderSession(cfg, 0xCAFEBABE)
	if err != nil {
		t.Fatalf("NewSRTPSenderSession: %v", err)
	}

	wantEnc, wantSalt, wantAuth, err := sess.deriveKeys(0)
	if err != nil {
		t.Fatalf("deriveKeys(0): %v", err)
	}

	if !bytes.Equal(sess.encKey, wantEnc) {
		t.Errorf("cached encKey differs from deriveKeys(0)")
	}
	if !bytes.Equal(sess.saltKey, wantSalt) {
		t.Errorf("cached saltKey differs from deriveKeys(0)")
	}
	if !bytes.Equal(sess.authorKey, wantAuth) {
		t.Errorf("cached authorKey differs from deriveKeys(0)")
	}
}

// TestSRTPEncryptDecrypt_CachedKeys verifies that the cached-key path produces
// the same encrypt/decrypt result as the key-derivation path (full round-trip).
// This implicitly tests that protect() + DecryptPacket() both use cached keys correctly.
func TestSRTPEncryptDecrypt_CachedKeys_MultiplePackets(t *testing.T) {
	keyBytes := generateSRTPKey(t)
	cfg := &SRTPConfig{
		MasterKey:  keyBytes[:16],
		MasterSalt: keyBytes[16:],
		Profile:    "AES_CM_128_HMAC_SHA1_80",
	}

	const ssrc = uint32(0x11223344)
	sender, err := NewSRTPSenderSession(cfg, ssrc)
	if err != nil {
		t.Fatal(err)
	}
	receiver, err := NewSRTPReceiverSession(cfg, ssrc)
	if err != nil {
		t.Fatal(err)
	}

	// Encrypt and decrypt 10 packets to verify caching works across multiple packets.
	for i := uint16(0); i < 10; i++ {
		payload := make([]byte, 160)
		payload[0] = byte(i) // distinguish each packet
		raw := buildMinimalRTP(ssrc, i, uint32(i)*160, 0, payload)

		encrypted, err := sender.EncryptPacket(raw)
		if err != nil {
			t.Fatalf("packet %d: encrypt: %v", i, err)
		}

		decrypted, err := receiver.DecryptPacket(encrypted)
		if err != nil {
			t.Fatalf("packet %d: decrypt: %v", i, err)
		}

		// Verify first byte of recovered payload matches
		if decrypted[12] != byte(i) {
			t.Errorf("packet %d: payload mismatch: want %d, got %d", i, i, decrypted[12])
		}
	}
}

// TestParseSRTPConfig_PipeStripped verifies that a crypto-suite suffix after '|' is stripped.
func TestParseSRTPConfig_PipeStripped(t *testing.T) {
	zeros := make([]byte, 30)
	b64 := encodeBase64(zeros)

	// Append a pipe + MKI (should be stripped)
	_, err := ParseSRTPConfig("inline:" + b64 + "|2^20|1:1")
	if err != nil {
		t.Fatalf("pipe-suffixed key should parse fine: %v", err)
	}
}
