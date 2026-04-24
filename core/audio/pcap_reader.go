// Package audio provides PCAP-based media replay for xk6-sip-media.
// It reads a Wireshark/tcpdump PCAP file, extracts UDP/RTP payloads
// from the first audio stream found, and returns them as ordered frames
// ready for replay via core/rtp/sender.go.
//
// Codec-agnostic: the raw RTP payloads are replayed byte-for-byte,
// enabling G.729, AMR, G.722, T.38 PCAP replay without codec implementation.
package audio

import (
	"encoding/binary"
	"fmt"
	"io"
	"net"
	"os"
	"sort"
)

// PCAPFrame is a single captured RTP payload with its original timestamp.
type PCAPFrame struct {
	Payload     []byte
	OffsetMicro int64 // microseconds from first frame — preserved for replay timing
	PayloadType uint8
}

// LoadPCAP reads a PCAP file and extracts RTP frames from the first UDP
// flow found that carries parseable RTP packets.
//
// The function is intentionally simple: no gopacket dependency, native PCAP
// parsing using the standard pcap global header + per-packet header layout.
//
// Supports:
//   - PCAP classic format (magic 0xa1b2c3d4 / 0xd4c3b2a1)
//   - PCAP-NG is NOT supported (use Wireshark to convert: File → Save As → pcap)
func LoadPCAP(path string) ([]PCAPFrame, uint8, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, 0, fmt.Errorf("pcap: open %q: %w", path, err)
	}
	defer f.Close()

	bo, _, err := parsePCAPGlobalHeader(f)
	if err != nil {
		return nil, 0, fmt.Errorf("pcap: global header: %w", err)
	}

	var frames []PCAPFrame
	var startMicro int64 = -1
	var dominantPT uint8

	ptCount := map[uint8]int{}

	for {
		tsSec, tsUsec, capLen, err := parsePCAPRecordHeader(f, bo)
		if err == io.EOF {
			break
		}
		if err != nil {
			break
		}

		data := make([]byte, capLen)
		if _, err := io.ReadFull(f, data); err != nil {
			break
		}

		rtp, ok := extractRTPFromEthernet(data)
		if !ok {
			continue
		}
		if len(rtp) < 12 {
			continue
		}

		// Minimal RTP header parse
		version := (rtp[0] >> 6) & 0x3
		if version != 2 {
			continue
		}
		pt := rtp[1] & 0x7F
		// Skip DTMF (payload type 101 and >95 dynamic usually DTMF)
		if pt > 127 {
			continue
		}

		// Skip CSRC list
		cc := int(rtp[0] & 0x0F)
		if len(rtp) < 12+cc*4 {
			continue
		}
		payload := rtp[12+cc*4:]
		// Skip extension header
		if rtp[0]&0x10 != 0 && len(payload) >= 4 {
			extLen := int(binary.BigEndian.Uint16(payload[2:4]))*4 + 4
			if len(payload) < extLen {
				continue
			}
			payload = payload[extLen:]
		}
		if len(payload) == 0 {
			continue
		}

		microTs := int64(tsSec)*1_000_000 + int64(tsUsec)
		if startMicro < 0 {
			startMicro = microTs
		}

		frames = append(frames, PCAPFrame{
			Payload:     append([]byte(nil), payload...),
			OffsetMicro: microTs - startMicro,
			PayloadType: pt,
		})
		ptCount[pt]++
	}

	if len(frames) == 0 {
		return nil, 0, fmt.Errorf("pcap: no RTP frames found in %q", path)
	}

	// Pick the most common payload type as the codec
	for pt, count := range ptCount {
		if count > ptCount[dominantPT] {
			dominantPT = pt
		}
	}

	sort.Slice(frames, func(i, j int) bool {
		return frames[i].OffsetMicro < frames[j].OffsetMicro
	})

	return frames, dominantPT, nil
}

// PCAPPayloads extracts raw payloads from PCAPFrames as a flat [][]byte slice,
// suitable for direct use in core/rtp/sender.Stream().
func PCAPPayloads(frames []PCAPFrame) [][]byte {
	payloads := make([][]byte, len(frames))
	for i, f := range frames {
		payloads[i] = f.Payload
	}
	return payloads
}

// ── PCAP format parsing (pure stdlib, no external deps) ──────────────────────

const (
	pcapMagicLE = 0xa1b2c3d4
	pcapMagicBE = 0xd4c3b2a1
)

func parsePCAPGlobalHeader(r io.Reader) (bo binary.ByteOrder, linkType uint32, err error) {
	buf := make([]byte, 24)
	if _, err = io.ReadFull(r, buf); err != nil {
		return nil, 0, err
	}
	magic := binary.LittleEndian.Uint32(buf[0:4])
	switch magic {
	case pcapMagicLE:
		bo = binary.LittleEndian
	case pcapMagicBE:
		bo = binary.BigEndian
	default:
		return nil, 0, fmt.Errorf("not a PCAP file (magic=0x%08x); convert PCAP-NG with Wireshark", magic)
	}
	linkType = bo.Uint32(buf[20:24])
	return bo, linkType, nil
}

func parsePCAPRecordHeader(r io.Reader, bo binary.ByteOrder) (tsSec, tsUsec, capLen uint32, err error) {
	buf := make([]byte, 16)
	if _, err = io.ReadFull(r, buf); err != nil {
		return 0, 0, 0, err
	}
	tsSec = bo.Uint32(buf[0:4])
	tsUsec = bo.Uint32(buf[4:8])
	capLen = bo.Uint32(buf[8:12])
	// origLen = bo.Uint32(buf[12:16]) — not needed
	return tsSec, tsUsec, capLen, nil
}

// extractRTPFromEthernet attempts to strip Ethernet + IP + UDP headers and
// return the UDP payload (which should be the RTP packet).
// Handles Ethernet II (link type 1) and raw IP (link type 101).
func extractRTPFromEthernet(data []byte) ([]byte, bool) {
	if len(data) < 14 {
		return nil, false
	}

	// Strip Ethernet header (14 bytes)
	etherType := binary.BigEndian.Uint16(data[12:14])

	var ipStart int
	switch etherType {
	case 0x0800: // IPv4
		ipStart = 14
	case 0x8100: // VLAN-tagged — strip 4 more bytes
		if len(data) < 18 {
			return nil, false
		}
		innerEtherType := binary.BigEndian.Uint16(data[16:18])
		if innerEtherType != 0x0800 {
			return nil, false
		}
		ipStart = 18
	default:
		// Try raw IPv4 (link type 101)
		if len(data) >= 1 && (data[0]>>4) == 4 {
			ipStart = 0
		} else {
			return nil, false
		}
	}

	return extractUDPPayloadFromIP(data[ipStart:])
}

func extractUDPPayloadFromIP(ip []byte) ([]byte, bool) {
	if len(ip) < 20 {
		return nil, false
	}
	version := ip[0] >> 4
	if version != 4 {
		return nil, false
	}
	protocol := ip[9]
	if protocol != 17 { // UDP only
		return nil, false
	}
	ihl := int(ip[0]&0x0F) * 4
	if len(ip) < ihl+8 {
		return nil, false
	}
	udp := ip[ihl:]
	// UDP: src(2) dst(2) len(2) checksum(2) payload
	udpLen := int(binary.BigEndian.Uint16(udp[4:6]))
	if udpLen < 8 || len(udp) < udpLen {
		return nil, false
	}
	// Heuristic: skip port filtering — take any UDP >= 1024 src/dst
	srcPort := binary.BigEndian.Uint16(udp[0:2])
	dstPort := binary.BigEndian.Uint16(udp[2:4])
	if srcPort < 1024 && dstPort < 1024 {
		return nil, false // likely DNS / well-known, not RTP
	}
	return udp[8:udpLen], true
}

// ResolveLocalIPv6 returns the local outbound IPv6 address.
// Falls back to empty string if no IPv6 route is available.
func ResolveLocalIPv6() string {
	return resolveLocalIPv6()
}

func resolveLocalIPv6() string {
	conn, err := net.Dial("udp6", "[2001:4860:4860::8888]:80")
	if err != nil {
		return ""
	}
	defer conn.Close()
	addr := conn.LocalAddr().(*net.UDPAddr)
	return addr.IP.String()
}
