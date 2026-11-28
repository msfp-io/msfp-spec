package main

// The media rules of MSFP Media's fetching, as MSFP's media part carries
// them: the names an owning server serves, HLS playlists, the relayed
// media type, ranges and the file's answers (MSFP Media fetching sections
// 1, 2.1, 3, 3.1 and 3.2), the scan cap of the scan answer's scanBytes
// (MSFP media section 4.2),
// WebVTT, ASS, PGS and fonts (MSFP Media video sections 6.3 to 6.6), and
// the destinations of spec/v0/core.md section 3.3. Written from the
// specification's text, to check vectors/v0/media.json and the
// destinations of vectors/v0/strict.json.

import (
	"errors"
	"fmt"
	"net/netip"
)

// --- Destinations (core section 3.3) ---

var refusedRanges = []struct {
	prefix netip.Prefix
	why    string
}{
	{netip.MustParsePrefix("127.0.0.0/8"), "loopback"},
	{netip.MustParsePrefix("::1/128"), "loopback"},
	{netip.MustParsePrefix("169.254.0.0/16"), "link-local"},
	{netip.MustParsePrefix("fe80::/10"), "link-local"},
	{netip.MustParsePrefix("0.0.0.0/8"), "unspecified or this network"},
	{netip.MustParsePrefix("::/128"), "unspecified"},
	{netip.MustParsePrefix("224.0.0.0/4"), "multicast"},
	{netip.MustParsePrefix("ff00::/8"), "multicast"},
	{netip.MustParsePrefix("240.0.0.0/4"), "reserved or broadcast"},
}

var (
	nat64     = netip.MustParsePrefix("64:ff9b::/96")
	sixToFour = netip.MustParsePrefix("2002::/16")
)

// checkDestination says whether a server connects to ip for a peer by
// default: nil when it does.
func checkDestination(s string) error {
	ip, err := netip.ParseAddr(s)
	if err != nil || ip.Zone() != "" {
		return errors.New("not an IP address")
	}
	ip = ip.Unmap() // an IPv4-mapped address is judged as IPv4
	// NAT64's well-known prefix (RFC 6052: the last 32 bits) and 6to4
	// (RFC 3056: bits 16 to 47) are judged as the IPv4 address they carry.
	if b := ip.As16(); ip.Is6() && nat64.Contains(ip) {
		ip = netip.AddrFrom4([4]byte(b[12:16]))
	} else if ip.Is6() && sixToFour.Contains(ip) {
		ip = netip.AddrFrom4([4]byte(b[2:6]))
	}
	for _, r := range refusedRanges {
		if r.prefix.Contains(ip) {
			return fmt.Errorf("refused: %s", r.why)
		}
	}
	return nil
}

// --- Small helpers ---

// --- HLS (fetching section 3.1) ---

// --- Ranges and the file (fetching sections 2.1 and 3.2; MSFP media
// section 4.2 for scans) ---

// The scan cap is the scan answer's scanBytes (MSFP playback section 8): 4
// MiB to 1 GiB, 4 MiB when the answer has none, or one out of range.
const (
	scanFloor = 4 << 20
	scanTop   = 1 << 30
)

// scanCapOf is the cap an owning server enforces for the scanBytes its
// scan answer states (0: none).
func scanCapOf(scanBytes int64) int64 {
	if scanBytes < scanFloor || scanBytes > scanTop {
		return scanFloor
	}
	return scanBytes
}

// scanAnswer is the owning server's answer to a fetch under a scan
// ticket of cap scanCap that has already sent sent bytes: 206 and its
// Content-Range, or 416 with Content-Range: bytes */<size>.
func scanAnswer(scanCap, sent int64, rng string, size int64) (int, string) {
	unsatisfied := fmt.Sprintf("bytes */%d", size)
	if rng == "" || sent >= scanCap {
		return 416, unsatisfied
	}
	start, end, err := checkRange(rng, size)
	if err != nil {
		return 416, unsatisfied
	}
	end = min(end, start+(scanCap-sent)-1)
	return 206, fmt.Sprintf("bytes %d-%d/%d", start, end, size)
}

// --- The names an owning server serves (fetching section 1) ---

// --- The relayed media type (fetching section 3) ---

// --- WebVTT (MSFP Media video section 6.3) ---

// --- ASS (MSFP Media video section 6.4) ---

// --- PGS (MSFP Media video section 6.5) ---

// --- Fonts (MSFP Media video section 6.6) ---
