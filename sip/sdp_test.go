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
