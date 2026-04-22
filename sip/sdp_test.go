package sip_test

import (
	"strings"
	"testing"

	sipcall "xk6-sip-media/sip"
)

func TestParseSDP_Standard(t *testing.T) {
	sdp := "v=0\r\n" +
		"o=- 0 0 IN IP4 192.168.1.10\r\n" +
		"s=test\r\n" +
		"c=IN IP4 192.168.1.10\r\n" +
		"t=0 0\r\n" +
		"m=audio 5004 RTP/AVP 0\r\n" +
		"a=rtpmap:0 PCMU/8000\r\n"

	ip, port, ptMap := sipcall.ParseSDP(sdp)
	if ip != "192.168.1.10" {
		t.Errorf("expected IP 192.168.1.10, got %q", ip)
	}
	if port != 5004 {
		t.Errorf("expected port 5004, got %d", port)
	}
	if ptMap[0] != "PCMU" {
		t.Errorf("expected ptMap[0]=PCMU, got %q", ptMap[0])
	}
}

func TestParseSDP_LFOnly(t *testing.T) {
	sdp := "v=0\nc=IN IP4 10.0.0.1\nm=audio 6000 RTP/AVP 0\n"
	ip, port, _ := sipcall.ParseSDP(sdp)
	if ip != "10.0.0.1" {
		t.Errorf("expected IP 10.0.0.1, got %q", ip)
	}
	if port != 6000 {
		t.Errorf("expected port 6000, got %d", port)
	}
}

func TestParseSDP_Empty(t *testing.T) {
	ip, port, _ := sipcall.ParseSDP("")
	if ip != "" || port != 0 {
		t.Errorf("empty SDP should return empty IP and 0 port, got %q %d", ip, port)
	}
}

func TestBuildSDP_ContainsRequiredFields(t *testing.T) {
	sdp := sipcall.BuildSDP("127.0.0.1", 40000, 0)
	checks := []string{
		"v=0",
		"c=IN IP4 127.0.0.1",
		"m=audio 40000",
		"PCMU/8000",
		"a=ptime:20",
	}
	for _, s := range checks {
		if !strings.Contains(sdp, s) {
			t.Errorf("BuildSDP output missing %q\n\nFull SDP:\n%s", s, sdp)
		}
	}
}

func TestBuildSDP_PCMA(t *testing.T) {
	sdp := sipcall.BuildSDP("10.0.0.1", 5000, 8)
	if !strings.Contains(sdp, "PCMA/8000") {
		t.Errorf("expected PCMA/8000 in SDP for PT=8, got:\n%s", sdp)
	}
}

func TestParseSDP_DynamicPtMap(t *testing.T) {
	// Simulate a modern PBX answering with Opus on dynamic PT=111
	sdp := "v=0\r\n" +
		"o=- 0 0 IN IP4 10.0.0.2\r\n" +
		"s=test\r\n" +
		"c=IN IP4 10.0.0.2\r\n" +
		"t=0 0\r\n" +
		"m=audio 6000 RTP/AVP 111 0\r\n" +
		"a=rtpmap:111 opus/48000/2\r\n" +
		"a=rtpmap:0 PCMU/8000\r\n"

	ip, port, ptMap := sipcall.ParseSDP(sdp)
	if ip != "10.0.0.2" {
		t.Errorf("expected IP 10.0.0.2, got %q", ip)
	}
	if port != 6000 {
		t.Errorf("expected port 6000, got %d", port)
	}
	if ptMap[111] != "OPUS" {
		t.Errorf("expected ptMap[111]=OPUS, got %q", ptMap[111])
	}
	if ptMap[0] != "PCMU" {
		t.Errorf("expected ptMap[0]=PCMU, got %q", ptMap[0])
	}
}

// TestParseSDP_NoStaticG722Injection verifies that PT 9 (G722) is NOT injected
// when the remote SDP does not advertise it. Injecting G722 unconditionally causes
// codec mismatches with peers that don't support it.
func TestParseSDP_NoStaticG722Injection(t *testing.T) {
	// SDP with only PCMU — G722 is NOT offered by the remote.
	sdp := "v=0\r\n" +
		"o=- 0 0 IN IP4 192.168.1.1\r\n" +
		"s=test\r\n" +
		"c=IN IP4 192.168.1.1\r\n" +
		"t=0 0\r\n" +
		"m=audio 5000 RTP/AVP 0\r\n" +
		"a=rtpmap:0 PCMU/8000\r\n"

	_, _, ptMap := sipcall.ParseSDP(sdp)

	if _, ok := ptMap[9]; ok {
		t.Errorf("ParseSDP should NOT inject PT 9 (G722) when not offered by remote; got ptMap[9]=%q", ptMap[9])
	}
}

// TestParseSDP_NoStaticG729Injection verifies that PT 18 (G729) is NOT injected
// when the remote SDP does not advertise it.
func TestParseSDP_NoStaticG729Injection(t *testing.T) {
	sdp := "v=0\r\n" +
		"o=- 0 0 IN IP4 192.168.1.1\r\n" +
		"s=test\r\n" +
		"c=IN IP4 192.168.1.1\r\n" +
		"t=0 0\r\n" +
		"m=audio 5000 RTP/AVP 0\r\n" +
		"a=rtpmap:0 PCMU/8000\r\n"

	_, _, ptMap := sipcall.ParseSDP(sdp)

	if _, ok := ptMap[18]; ok {
		t.Errorf("ParseSDP should NOT inject PT 18 (G729) when not offered by remote; got ptMap[18]=%q", ptMap[18])
	}
}

// TestParseSDP_G722PresentWhenOffered verifies that G722 IS present in ptMap when
// the remote actually offers it in the SDP.
func TestParseSDP_G722PresentWhenOffered(t *testing.T) {
	sdp := "v=0\r\n" +
		"o=- 0 0 IN IP4 10.0.0.1\r\n" +
		"s=test\r\n" +
		"c=IN IP4 10.0.0.1\r\n" +
		"t=0 0\r\n" +
		"m=audio 5006 RTP/AVP 0 9\r\n" +
		"a=rtpmap:0 PCMU/8000\r\n" +
		"a=rtpmap:9 G722/8000\r\n"

	_, _, ptMap := sipcall.ParseSDP(sdp)

	if ptMap[9] != "G722" {
		t.Errorf("G722 offered in SDP but not in ptMap: %v", ptMap)
	}
}

// TestParseSDP_PCMUAlwaysPresent verifies that PCMU (PT 0) is always in the map
// even if the remote SDP forgot to include it (RFC 3551 requires it).
func TestParseSDP_PCMUAlwaysPresent(t *testing.T) {
	// SDP with only PCMA — PCMU must still be injected as a safe default.
	sdp := "v=0\r\n" +
		"o=- 0 0 IN IP4 10.0.0.1\r\n" +
		"s=test\r\n" +
		"c=IN IP4 10.0.0.1\r\n" +
		"t=0 0\r\n" +
		"m=audio 5000 RTP/AVP 8\r\n" +
		"a=rtpmap:8 PCMA/8000\r\n"

	_, _, ptMap := sipcall.ParseSDP(sdp)

	if ptMap[0] != "PCMU" {
		t.Errorf("PCMU (PT 0) should always be injected as RFC 3551 default; ptMap=%v", ptMap)
	}
}
