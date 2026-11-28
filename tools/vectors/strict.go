package main

// The strict rules of MSFP: the test draft's "Canonical forms" section (the
// corrections of 2026-11-16) and core version 0's section 2. Each check
// returns nil for a valid value, or an error that says why it isn't.

import (
	"bytes"
	"crypto/ed25519"
	"encoding/base32"
	"encoding/base64"
	"errors"
	"fmt"
	"net/netip"
	"strings"
	"unicode"
	"unicode/utf8"

	"filippo.io/edwards25519"
)

// orderL is L, the order of the prime-order subgroup, little-endian.
var orderL = []byte{
	0xed, 0xd3, 0xf5, 0x5c, 0x1a, 0x63, 0x12, 0x58,
	0xd6, 0x9c, 0xf7, 0xa2, 0xde, 0xf9, 0xde, 0x14,
	0, 0, 0, 0, 0, 0, 0, 0,
	0, 0, 0, 0, 0, 0, 0, 0x10,
}

// lMinusOne is L - 1 as a scalar. Scalars are taken mod L, so L itself
// can't be one; [L]P is computed as [L - 1]P + P.
var lMinusOne = func() *edwards25519.Scalar {
	b := bytes.Clone(orderL)
	b[0]--
	s, err := edwards25519.NewScalar().SetCanonicalBytes(b)
	if err != nil {
		panic(err)
	}
	return s
}()

// timesL returns [L]P.
func timesL(p *edwards25519.Point) *edwards25519.Point {
	q := new(edwards25519.Point).ScalarMult(lMinusOne, p)
	return q.Add(q, p)
}

// checkPoint applies the point rules to a key or to R: the canonical
// encoding of a point on the curve, in the prime-order subgroup.
func checkPoint(b []byte) error {
	if len(b) != 32 {
		return fmt.Errorf("%d bytes, not 32", len(b))
	}
	p, err := new(edwards25519.Point).SetBytes(b)
	if err != nil {
		return errors.New("not a point on the curve")
	}
	if !bytes.Equal(p.Bytes(), b) {
		return errors.New("not the canonical encoding of its point")
	}
	// The 8 points of small order, the identity among them, are in the
	// torsion subgroup: [8]P is the identity.
	if new(edwards25519.Point).MultByCofactor(p).Equal(edwards25519.NewIdentityPoint()) == 1 {
		return errors.New("a point of small order")
	}
	if timesL(p).Equal(edwards25519.NewIdentityPoint()) != 1 {
		return errors.New("not in the prime-order subgroup")
	}
	return nil
}

// checkKey applies the rules for a public key.
func checkKey(pub []byte) error { return checkPoint(pub) }

// verifyStrict applies the rules for a signature under a valid key.
func verifyStrict(pub, msg, sig []byte) error {
	if err := checkKey(pub); err != nil {
		return fmt.Errorf("key: %w", err)
	}
	if len(sig) != 64 {
		return fmt.Errorf("signature of %d bytes, not 64", len(sig))
	}
	if err := checkPoint(sig[:32]); err != nil {
		return fmt.Errorf("R: %w", err)
	}
	if _, err := edwards25519.NewScalar().SetCanonicalBytes(sig[32:]); err != nil {
		return errors.New("S is not below L")
	}
	// With A and R in the prime-order subgroup, the cofactorless check
	// of crypto/ed25519 accepts exactly what the cofactored one would.
	if !ed25519.Verify(pub, msg, sig) {
		return errors.New("the equation doesn't hold")
	}
	return nil
}

var b64 = base64.RawURLEncoding

// checkB64 applies the base64url rules, and the fixed length n of the
// decoded value when n > 0.
func checkB64(text string, n int) ([]byte, error) {
	for _, c := range text {
		if !(c >= 'A' && c <= 'Z' || c >= 'a' && c <= 'z' || c >= '0' && c <= '9' || c == '-' || c == '_') {
			return nil, fmt.Errorf("character %q outside the alphabet", c)
		}
	}
	// The alphabet check above leaves no CR or LF for the decoder to
	// skip; the lenient decoder then accepts nonzero unused bits, which
	// the re-encoding below catches.
	b, err := b64.DecodeString(text)
	if err != nil {
		return nil, errors.New("not base64url: a bad length")
	}
	if b64.EncodeToString(b) != text {
		return nil, errors.New("unused bits are not zero")
	}
	if n > 0 && len(b) != n {
		return nil, fmt.Errorf("%d bytes, not %d", len(b), n)
	}
	return b, nil
}

// checkDate applies the rules for the date header: a positive decimal.
func checkDate(text string) error {
	n, err := checkDecimal(text)
	if err != nil {
		return err
	}
	if n == 0 {
		return errors.New("not positive")
	}
	return nil
}

// checkAddress applies the address rules. Core version 0 allows https
// only; version 0 allows http too.
func checkAddress(addr string, allowHTTP bool) error {
	if len(addr) > 512 {
		return errors.New("over 512 bytes")
	}
	for i := 0; i < len(addr); i++ {
		if addr[i] < 0x21 || addr[i] > 0x7e {
			return errors.New("not printable ASCII")
		}
	}
	var rest string
	defPort := ""
	switch {
	case strings.HasPrefix(addr, "https://"):
		rest, defPort = addr[len("https://"):], "443"
	case strings.HasPrefix(addr, "http://") && allowHTTP:
		rest, defPort = addr[len("http://"):], "80"
	case strings.HasPrefix(addr, "http://"):
		return errors.New("scheme http is not allowed")
	default:
		return errors.New("scheme is not https or http, in lowercase")
	}
	if strings.ContainsAny(rest, "/?#@") {
		return errors.New("a path, query, fragment or user info")
	}
	host, port := rest, ""
	if strings.HasPrefix(rest, "[") {
		end := strings.IndexByte(rest, ']')
		if end < 0 {
			return errors.New("an unclosed IPv6 bracket")
		}
		host = rest[:end+1]
		after := rest[end+1:]
		if after != "" {
			if after[0] != ':' {
				return errors.New("text after the IPv6 address")
			}
			port = after[1:]
			if port == "" {
				return errors.New("an empty port")
			}
		}
		ip, err := netip.ParseAddr(host[1 : len(host)-1])
		if err != nil || !ip.Is6() || ip.Zone() != "" {
			return errors.New("not an IPv6 address without zone")
		}
		if ip.String() != host[1:len(host)-1] {
			return errors.New("IPv6 not in the text form of RFC 5952")
		}
	} else {
		if i := strings.LastIndexByte(rest, ':'); i >= 0 {
			host, port = rest[:i], rest[i+1:]
			if port == "" {
				return errors.New("an empty port")
			}
		}
		if err := checkHost(host); err != nil {
			return err
		}
	}
	if port != "" {
		n, err := checkDecimal(port)
		if err != nil || n < 1 || n > 65535 {
			return errors.New("port not 1 to 65535 without leading zeros")
		}
		if port == defPort {
			return errors.New("the scheme's default port")
		}
	}
	return nil
}

// checkHost checks a name or an IPv4 address. A host whose last label
// is all digits is read as an IPv4 address, and must be one.
func checkHost(host string) error {
	if host == "" {
		return errors.New("no host")
	}
	if len(host) > 253 {
		return errors.New("a name over 253 characters")
	}
	labels := strings.Split(host, ".")
	last := labels[len(labels)-1]
	if last != "" && strings.Trim(last, "0123456789") == "" {
		if len(labels) != 4 {
			return errors.New("a numeric host that isn't four numbers")
		}
		for _, l := range labels {
			n, err := checkDecimal(l)
			if err != nil || n > 255 {
				return errors.New("IPv4 numbers not 0 to 255 without leading zeros")
			}
		}
		return nil
	}
	for _, l := range labels {
		if l == "" || len(l) > 63 {
			return errors.New("a label empty or over 63 characters")
		}
		if l[0] == '-' || l[len(l)-1] == '-' {
			return errors.New("a label starting or ending with -")
		}
		for i := 0; i < len(l); i++ {
			c := l[i]
			if !(c >= 'a' && c <= 'z' || c >= '0' && c <= '9' || c == '-') {
				return errors.New("a name with characters outside a-z 0-9 -")
			}
		}
	}
	return nil
}

// code is a pairing code's content.
type code struct {
	key       []byte
	secret    []byte
	expires   int64
	group     []byte // core 0: the group's ID, when the code has one
	groupName string
	address   string
}

// readCode applies the reader's rules for a pairing code.
func readCode(text, prefix string, allowHTTP bool) (code, error) {
	var sb strings.Builder
	for _, r := range text {
		if !unicode.Is(unicode.White_Space, r) {
			sb.WriteRune(r)
		}
	}
	t := sb.String()
	if !strings.HasPrefix(t, prefix) {
		return code{}, errors.New("not the exact prefix")
	}
	rest := t[len(prefix):]
	// Core 0's codes carry a flag byte, and a group with 1 (core section
	// 5.1); the test draft's don't.
	withFlag := prefix == core0.prefix
	most := 779
	if withFlag {
		most = 1164
	}
	if len(rest) > most {
		return code{}, fmt.Errorf("over %d characters", most)
	}
	b, err := checkB64(rest, 0)
	if err != nil {
		return code{}, err
	}
	if len(b) <= 72 {
		return code{}, errors.New("72 bytes or fewer")
	}
	c := code{key: b[:32], secret: b[32:64]}
	at := 72
	if withFlag {
		switch b[72] {
		case 0:
			at = 73
		case 1:
			if len(b) < 73+33 {
				return code{}, errors.New("too short for the group its flag announces")
			}
			c.group = b[73:105]
			n := int(b[105])
			if n == 0 {
				return code{}, errors.New("a group name of 0 bytes")
			}
			if len(b) < 106+n {
				return code{}, errors.New("too short for the group name's length")
			}
			name := b[106 : 106+n]
			if !utf8.Valid(name) {
				return code{}, errors.New("a group name that isn't UTF-8")
			}
			c.groupName = cleanName(string(name), 1<<20)
			if utf8.RuneCountInString(c.groupName) > 80 {
				return code{}, errors.New("a group name over 80 code points once cleaned")
			}
			at = 106 + n
		default:
			return code{}, errors.New("a flag byte other than 0 and 1")
		}
	}
	if len(b) <= at {
		return code{}, errors.New("no address")
	}
	c.address = string(b[at:])
	var exp uint64
	for _, x := range b[64:72] {
		exp = exp<<8 | uint64(x)
	}
	if exp > maxInt {
		return code{}, errors.New("an expiry above 2^53 - 1")
	}
	c.expires = int64(exp)
	if err := checkKey(c.key); err != nil {
		return code{}, fmt.Errorf("key: %w", err)
	}
	if err := checkAddress(c.address, allowHTTP); err != nil {
		return code{}, fmt.Errorf("address: %w", err)
	}
	return c, nil
}

// makeCode writes a pairing code.
func makeCode(prefix string, key, secret []byte, expires uint64, address string) string {
	b := codeHead(key, secret, expires)
	if prefix == core0.prefix {
		b = append(b, 0)
	}
	b = append(b, address...)
	return prefix + b64.EncodeToString(b)
}

// makeGroupCode writes a core 0 code with a group (core section 5.1).
func makeGroupCode(prefix string, key, secret []byte, expires uint64, group []byte, name, address string) string {
	b := append(codeHead(key, secret, expires), 1)
	b = append(append(b, group...), byte(len(name)))
	b = append(append(b, name...), address...)
	return prefix + b64.EncodeToString(b)
}

// rawCode writes a core 0 code from its bytes after the expiry, for cases
// a writer wouldn't make.
func rawCode(prefix string, key, secret []byte, expires uint64, tail []byte) string {
	return prefix + b64.EncodeToString(append(codeHead(key, secret, expires), tail...))
}

func codeHead(key, secret []byte, expires uint64) []byte {
	b := append(bytes.Clone(key), secret...)
	for i := 7; i >= 0; i-- {
		b = append(b, byte(expires>>(8*i)))
	}
	return b
}

// checkBase applies the rule for the discovery document's base
// (spec/v0/core.md, the discovery document): at most 256 bytes, an
// absolute path of segments of A-Z a-z 0-9 . _ ~ -, no empty segment
// before the end, no "." or ".." segment, ending in /api/v1/federation/.
func checkBase(s string) error {
	const end = "/api/v1/federation/"
	if len(s) > 256 {
		return errors.New("over 256 bytes")
	}
	if !strings.HasSuffix(s, end) {
		return errors.New("doesn't end in /api/v1/federation/")
	}
	segs := strings.Split(strings.TrimSuffix(strings.TrimPrefix(s, "/"), "/"), "/")
	if !strings.HasPrefix(s, "/") {
		return errors.New("not an absolute path")
	}
	for _, seg := range segs {
		switch seg {
		case "":
			return errors.New("an empty segment")
		case ".", "..":
			return errors.New(`a "." or ".." segment`)
		}
		for _, c := range []byte(seg) {
			if !(c >= 'A' && c <= 'Z' || c >= 'a' && c <= 'z' || c >= '0' && c <= '9' || c == '.' || c == '_' || c == '~' || c == '-') {
				return errors.New("a character outside A-Z a-z 0-9 . _ ~ - (no percent-encoding)")
			}
		}
	}
	return nil
}

// b32 is lowercase base32 (RFC 4648 section 6, a-z2-7), no padding.
var b32 = base32.NewEncoding("abcdefghijklmnopqrstuvwxyz234567").WithPadding(base32.NoPadding)

// uncallableAddress is the address a server that can't be called must
// have (spec/v0/core.md section 5.2): https://msfp-<26>.invalid, the 26
// characters being 16 bytes in lowercase base32.
func uncallableAddress(b []byte) string { return "https://msfp-" + b32.EncodeToString(b) + ".invalid" }

// checkUncallableAddress applies that rule: an address of section 5.2,
// of that form, whose 26 characters are exactly the encoding of 16
// bytes (the 2 unused bits of the last character are zero).
func checkUncallableAddress(addr string) error {
	if err := checkAddress(addr, false); err != nil {
		return err
	}
	const pre, suf = "https://msfp-", ".invalid"
	if !strings.HasPrefix(addr, pre) || !strings.HasSuffix(addr, suf) {
		return errors.New("not of the form https://msfp-<26>.invalid")
	}
	mid := addr[len(pre) : len(addr)-len(suf)]
	if len(mid) != 26 {
		return fmt.Errorf("%d characters between msfp- and .invalid, not 26", len(mid))
	}
	for _, c := range []byte(mid) {
		if !(c >= 'a' && c <= 'z' || c >= '2' && c <= '7') {
			return fmt.Errorf("character %q outside a-z 2-7", c)
		}
	}
	b, err := b32.DecodeString(mid)
	if err != nil || len(b) != 16 || b32.EncodeToString(b) != mid {
		return errors.New("unused bits are not zero")
	}
	return nil
}

// ---- The catalog part of the revision of version 0 (proposal 16) ----
