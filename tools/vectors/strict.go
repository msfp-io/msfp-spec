package main

// The strict rules of MSFP: the test draft's "Canonical forms" section (the
// corrections of 2026-11-16) and core version 0's section 2. Each check
// returns nil for a valid value, or an error that says why it isn't.

import (
	"bytes"
	"crypto/ed25519"
	"encoding/base32"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"hash/crc32"
	"net/netip"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"

	"filippo.io/edwards25519"
)

// maxInt is the largest integer MSFP carries: 2^53 - 1.
const maxInt = 1<<53 - 1

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

// checkHex applies the hex rules.
func checkHex(text string) error {
	for _, c := range text {
		if !(c >= '0' && c <= '9' || c >= 'a' && c <= 'f') {
			return fmt.Errorf("character %q is not lowercase hex", c)
		}
	}
	return nil
}

// checkDecimal applies the rules for a decimal integer in a header or
// a signed message.
func checkDecimal(text string) (int64, error) {
	if text == "" {
		return 0, errors.New("empty")
	}
	for _, c := range text {
		if c < '0' || c > '9' {
			return 0, fmt.Errorf("character %q is not a digit", c)
		}
	}
	if len(text) > 1 && text[0] == '0' {
		return 0, errors.New("a leading zero")
	}
	n, err := strconv.ParseInt(text, 10, 64)
	if err != nil || n > maxInt {
		return 0, errors.New("above 2^53 - 1")
	}
	return n, nil
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

// checkInteger applies the rules for a JSON integer, given the number's
// text as it appears in the body.
func checkInteger(text string) error {
	t := strings.TrimPrefix(text, "-")
	if t == "" {
		return errors.New("empty")
	}
	if strings.ContainsAny(t, ".eE") {
		return errors.New("a fraction or an exponent")
	}
	if text == "-0" {
		return errors.New("-0")
	}
	if _, err := checkDecimal(t); err != nil {
		return err
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

// checkMediaName applies the rule for a file or track name in a media URL
// (spec/v0/media.md section 1): 1 to 64 characters of A-Z a-z 0-9 . _ -,
// and never "." or "..", which a URL resolves as folders.
func checkMediaName(s string) error {
	if s == "." || s == ".." {
		return errors.New(`"." and ".." are not names`)
	}
	if len(s) < 1 || len(s) > 64 {
		return errors.New("not 1 to 64 characters")
	}
	for _, c := range []byte(s) {
		if !(c >= 'A' && c <= 'Z' || c >= 'a' && c <= 'z' || c >= '0' && c <= '9' || c == '.' || c == '_' || c == '-') {
			return errors.New("a character outside A-Z a-z 0-9 . _ -")
		}
	}
	return nil
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

// catalogPageTitles and catalogPageBytes are a page's two bounds
// (spec/v0/catalog.md section 3.1); catalogAnswerBytes is the core's
// bound on any answer a caller reads (spec/v0/core.md section 3).
const (
	catalogPageTitles  = 5000
	catalogPageBytes   = 3 << 20
	catalogAnswerBytes = 4 << 20
)

// catalogPage is one answer of titles, as the vectors give it: what the
// page holds, and, for a whole-list answer, whether titles remain after
// it on the owning server.
type catalogPage struct {
	Changes bool    `json:"changes"`
	Titles  int     `json:"titles"`
	Gone    int     `json:"gone"`
	Bytes   int     `json:"bytes"`
	Remain  *bool   `json:"remain,omitempty"`
	After   string  `json:"after"`
	LastKey string  `json:"lastKey,omitempty"`
	Next    *string `json:"next,omitempty"`
}

// catalogPageCase is a case of titlePages: a page, whether the owning
// server may send it, and what the caller does with it.
type catalogPageCase struct {
	Name string `json:"name"`
	catalogPage
	Valid  bool   `json:"valid"`
	Reason string `json:"reason,omitempty"`
	Caller string `json:"caller"`
}

// catalogImageCase is a case of localImages, of one of three sides:
// "request" (the owning server's answer to an image request: status and
// type), "answer" (the viewer's server's check of the answer's url:
// valid and the ticket), "image" (its checks of the image's bytes: valid
// and the format).
type catalogImageCase struct {
	Name       string           `json:"name"`
	Side       string           `json:"side"`
	Parts      []string         `json:"parts,omitempty"`
	Offered    []catalogOffered `json:"offered,omitempty"`
	Body       string           `json:"body,omitempty"`
	StreamRoot string           `json:"streamRoot,omitempty"`
	Hex        string           `json:"hex,omitempty"`
	Size       int              `json:"size,omitempty"`
	Status     int              `json:"status,omitempty"`
	Type       string           `json:"type,omitempty"`
	Valid      *bool            `json:"valid,omitempty"`
	Ticket     string           `json:"ticket,omitempty"`
	Format     string           `json:"format,omitempty"`
	Reason     string           `json:"reason,omitempty"`
}

// catalogCheckPage applies the owning server's rules for a page
// (catalog section 3.1): at most 5,000 titles (titles and gone together
// in a changes answer), at most 3 MiB of JSON, and in a whole-list
// answer a next that is the page's last key exactly when titles remain.
func catalogCheckPage(p catalogPage) error {
	if p.Titles < 0 || p.Gone < 0 || p.Bytes < 0 {
		return errors.New("a negative count")
	}
	if !p.Changes && p.Gone != 0 {
		return errors.New("gone in a whole-list answer")
	}
	if p.Titles+p.Gone > catalogPageTitles {
		if p.Changes {
			return errors.New("titles and gone together over 5,000 keys")
		}
		return errors.New("over 5,000 titles")
	}
	if p.Bytes > catalogPageBytes {
		return errors.New("an answer body over 3 MiB (3,145,728 bytes)")
	}
	if p.Changes {
		// next is set when there may be more changes: the owning
		// server's call, whatever the page's size.
		return nil
	}
	if p.Remain == nil {
		return errors.New("a whole-list case without remain")
	}
	if p.Titles > 0 && p.LastKey <= p.After {
		return errors.New("the page's last key isn't after after")
	}
	switch {
	case *p.Remain && p.Titles == 0:
		return errors.New("an empty page while titles remain")
	case *p.Remain && p.Next == nil:
		return errors.New("next absent while titles remain")
	case *p.Remain && *p.Next != p.LastKey:
		return errors.New("next isn't the page's last key")
	case !*p.Remain && p.Next != nil:
		return errors.New("next set after the last title")
	}
	return nil
}

// catalogCallerStep is what a caller does with a page it got (catalog
// section 3.1): "fail" for an answer past the core's 4 MiB, else "stop"
// when next is absent, empty, or not greater than its after in byte
// order, else "ask on". The caller accepts what the owning server
// shouldn't have sent, within the core's bound.
func catalogCallerStep(p catalogPage) string {
	switch {
	case p.Bytes > catalogAnswerBytes:
		return "fail"
	case p.Next == nil || *p.Next == "" || *p.Next <= p.After:
		return "stop"
	}
	return "ask on"
}

// catalogCheckOpaque applies the rule for the opaque parts of keys
// (catalog section 4.2): the form of a library ID, 1 to 64 characters
// of A-Z a-z 0-9 _ -.
func catalogCheckOpaque(s string) error {
	if len(s) < 1 || len(s) > 64 {
		return fmt.Errorf("%d bytes, not 1 to 64", len(s))
	}
	for _, c := range []byte(s) {
		if !catalogTicketByte(c) {
			return fmt.Errorf("byte %q outside A-Z a-z 0-9 _ -", c)
		}
	}
	return nil
}

// catalogTicketByte: A-Z a-z 0-9 _ -, the bytes of library IDs, the
// opaque parts of keys and tickets.
func catalogTicketByte(c byte) bool {
	return c >= 'A' && c <= 'Z' || c >= 'a' && c <= 'z' || c >= '0' && c <= '9' || c == '_' || c == '-'
}

// catalogDesc is what a caller keeps of a local title's description
// (catalog section 4.3).
type catalogDesc struct {
	Name  string `json:"name,omitempty"`
	Year  *int64 `json:"year,omitempty"`
	Image bool   `json:"image,omitempty"`
}

// catalogDescription reads a local title's description from the title's
// JSON object (catalog section 4.3). nameRequired is the content part's
// say for the title's kind. name is cleaned as core section 9.1 cleans a
// server's name, with 128 code points instead of 80; a name that isn't a
// string counts as absent (core section 2.3). year that isn't an
// integer from 1800 to 9999, and image that isn't a boolean, count as
// absent. An error means the title is skipped. The vectors of the
// description are the content parts' (localTitles in video.json and
// music.json); this is the catalog's rule they share.
func catalogDescription(title []byte, nameRequired bool) (catalogDesc, error) {
	var m map[string]json.RawMessage
	if err := json.Unmarshal(title, &m); err != nil || m == nil {
		return catalogDesc{}, errors.New("not a JSON object")
	}
	var d catalogDesc
	hasName := false
	if raw, ok := m["name"]; ok && len(raw) > 0 && raw[0] == '"' {
		var name string
		if json.Unmarshal(raw, &name) == nil {
			hasName = true
			d.Name = cleanName(name, 128)
		}
	}
	if nameRequired {
		if !hasName {
			return catalogDesc{}, errors.New("no name, where one is required")
		}
		if d.Name == "" {
			return catalogDesc{}, errors.New("a name empty once cleaned")
		}
	}
	if raw, ok := m["year"]; ok {
		t := string(raw)
		if checkInteger(t) == nil {
			if n, err := strconv.ParseInt(t, 10, 64); err == nil && n >= 1800 && n <= 9999 {
				d.Year = &n
			}
		}
	}
	if raw, ok := m["image"]; ok && string(raw) == "true" {
		d.Image = true
	}
	return d, nil
}

// catalogOffered is a title the owning server offers the caller, with
// whether its description says image true.
type catalogOffered struct {
	Key   string `json:"key"`
	Image bool   `json:"image"`
}

// catalogImageKinds: the kinds that have an image call, and the content
// part each belongs to (catalog section 4.4).
var catalogImageKinds = map[string]string{"movie": "video", "series": "video", "artist": "music", "album": "music"}

// catalogImageRequest is the owning server's answer to an image request
// (catalog section 4.4): its status and problem type. parts are the
// content parts on between the two servers.
func catalogImageRequest(parts []string, offered []catalogOffered, body []byte) (int, string) {
	on := map[string]bool{}
	for _, p := range parts {
		on[p] = true
	}
	if !on["video"] && !on["music"] {
		return 404, "not-supported"
	}
	if checkIJSON(body) != nil {
		return 400, "invalid-request"
	}
	var m map[string]json.RawMessage
	if json.Unmarshal(body, &m) != nil || m == nil {
		return 400, "invalid-request"
	}
	raw, ok := m["title"]
	var title string
	if !ok || len(raw) == 0 || raw[0] != '"' || json.Unmarshal(raw, &title) != nil {
		return 400, "invalid-request"
	}
	for _, o := range offered {
		if o.Key != title {
			continue
		}
		p := strings.Split(title, "/")
		if len(p) != 3 || p[1] != "local" || !on[catalogImageKinds[p[0]]] || !o.Image {
			return 404, "not-found"
		}
		return 200, ""
	}
	return 404, "not-found"
}

// catalogImageAnswer applies the viewer's server's check of an image
// answer (catalog section 4.4): url is exactly <stream root><ticket>/image,
// the ticket as media section 1 gives it. It returns the ticket, or an
// error when the answer is dropped.
func catalogImageAnswer(root string, body []byte) (string, error) {
	if checkIJSON(body) != nil {
		return "", errors.New("not I-JSON")
	}
	var m map[string]json.RawMessage
	if json.Unmarshal(body, &m) != nil || m == nil {
		return "", errors.New("not a JSON object")
	}
	raw, ok := m["url"]
	var u string
	if !ok || len(raw) == 0 || raw[0] != '"' || json.Unmarshal(raw, &u) != nil {
		return "", errors.New("url absent or not a string")
	}
	if !strings.HasPrefix(u, root) {
		return "", errors.New("url not under the owning server's stream root")
	}
	rest := u[len(root):]
	if !strings.HasSuffix(rest, "/image") {
		return "", errors.New("url not ending in /image")
	}
	t := strings.TrimSuffix(rest, "/image")
	if len(t) < 1 || len(t) > 64 {
		return "", errors.New("a ticket not 1 to 64 characters")
	}
	for _, c := range []byte(t) {
		if !catalogTicketByte(c) {
			return "", fmt.Errorf("a ticket with byte %q outside A-Z a-z 0-9 _ -", c)
		}
	}
	return t, nil
}

// catalogImageBytes is the bound on a local title's image, and
// catalogImageSide the bound on its width and height (catalog section 4.4).
const (
	catalogImageBytes = 1 << 20
	catalogImageSide  = 4096
)

var catalogPNGSignature = []byte{0x89, 'P', 'N', 'G', 0x0d, 0x0a, 0x1a, 0x0a}

// catalogCheckImage applies the viewer's server's checks of an image
// before decoding it (catalog section 4.4, media section 3.4): its whole
// length within 1 MiB, a JPEG (FF D8 FF) or a PNG (the 8-byte signature)
// by its content, and a width and a height from its header, 1 to 4,096.
// head is the image's first bytes, size its whole length. It returns
// "jpeg" or "png".
func catalogCheckImage(head []byte, size int) (string, error) {
	if len(head) > size {
		return "", errors.New("more bytes than its size")
	}
	if size > catalogImageBytes {
		return "", errors.New("over 1 MiB (1,048,576 bytes)")
	}
	var format string
	var w, h int
	switch {
	case bytes.HasPrefix(head, catalogPNGSignature):
		format = "png"
		ih := head[8:]
		if len(ih) < 16 || !bytes.Equal(ih[:8], []byte{0, 0, 0, 13, 'I', 'H', 'D', 'R'}) {
			return "", errors.New("a PNG whose first chunk isn't IHDR")
		}
		w = int(ih[8])<<24 | int(ih[9])<<16 | int(ih[10])<<8 | int(ih[11])
		h = int(ih[12])<<24 | int(ih[13])<<16 | int(ih[14])<<8 | int(ih[15])
	case bytes.HasPrefix(head, []byte{0xff, 0xd8, 0xff}):
		format = "jpeg"
		var err error
		if w, h, err = catalogJPEGSize(head); err != nil {
			return "", err
		}
	default:
		return "", errors.New("neither a JPEG nor a PNG by its content")
	}
	if w < 1 || h < 1 {
		return "", errors.New("a width or a height of 0")
	}
	if w > catalogImageSide || h > catalogImageSide {
		return "", errors.New("wider or taller than 4,096 pixels")
	}
	return format, nil
}

// catalogPNG makes the head of a PNG for the vectors: its signature and
// IHDR, of width w and height h, 8-bit RGBA.
func catalogPNG(w, h uint32) []byte {
	ih := []byte{'I', 'H', 'D', 'R', byte(w >> 24), byte(w >> 16), byte(w >> 8), byte(w), byte(h >> 24), byte(h >> 16), byte(h >> 8), byte(h), 8, 6, 0, 0, 0}
	c := crc32.ChecksumIEEE(ih)
	b := append(bytes.Clone(catalogPNGSignature), 0, 0, 0, 13)
	b = append(b, ih...)
	return append(b, byte(c>>24), byte(c>>16), byte(c>>8), byte(c))
}

// catalogJPEG makes the head of a JPEG for the vectors: SOI, a JFIF
// APP0, a DHT when dht, and a frame header of the marker sof, width w
// and height h, 3 components.
func catalogJPEG(sof byte, w, h int, dht bool) []byte {
	b := []byte{0xff, 0xd8, 0xff, 0xe0, 0, 16, 'J', 'F', 'I', 'F', 0, 1, 1, 0, 0, 1, 0, 1, 0, 0}
	if dht {
		b = append(b, 0xff, 0xc4, 0, 20, 0)
		b = append(b, make([]byte, 17)...)
	}
	return append(b, 0xff, sof, 0, 17, 8, byte(h>>8), byte(h), byte(w>>8), byte(w), 3, 1, 0x22, 0, 2, 0x11, 1, 3, 0x11, 1)
}

// catalogJPEGSize reads a JPEG's markers up to its frame header (SOF0 to
// SOF15, but DHT, JPG and DAC), and returns its width and height.
func catalogJPEGSize(b []byte) (int, int, error) {
	i := 2
	for {
		if i >= len(b) || b[i] != 0xff {
			return 0, 0, errors.New("no marker where one is due")
		}
		for i < len(b) && b[i] == 0xff {
			i++ // the marker's FF, and fill bytes before it
		}
		if i >= len(b) {
			return 0, 0, errors.New("no frame header in its header")
		}
		m := b[i]
		i++
		switch {
		case m == 0x01 || m >= 0xd0 && m <= 0xd7:
			continue // markers without a length
		case m == 0xd9 || m == 0xda:
			return 0, 0, errors.New("no frame header before the image data")
		}
		if i+2 > len(b) {
			return 0, 0, errors.New("no frame header in its header")
		}
		n := int(b[i])<<8 | int(b[i+1])
		if n < 2 {
			return 0, 0, errors.New("a segment length under 2")
		}
		if m >= 0xc0 && m <= 0xcf && m != 0xc4 && m != 0xc8 && m != 0xcc {
			if n < 8 || i+7 > len(b) {
				return 0, 0, errors.New("a frame header cut short")
			}
			h := int(b[i+3])<<8 | int(b[i+4])
			w := int(b[i+5])<<8 | int(b[i+6])
			return w, h, nil
		}
		i += n
	}
}

// catalogLibraryKind is what a caller keeps of a library's kind
// (catalog section 2.2): the kind when it is a token, known or not
// (core section 8.5); nil when it is absent, not a string, or not a
// token. library is the library's JSON object.
func catalogLibraryKind(library []byte) (*string, error) {
	var m map[string]json.RawMessage
	if err := json.Unmarshal(library, &m); err != nil || m == nil {
		return nil, errors.New("not a JSON object")
	}
	raw, ok := m["kind"]
	if !ok || len(raw) == 0 || raw[0] != '"' {
		return nil, nil
	}
	var k string
	if json.Unmarshal(raw, &k) != nil || !openToken(k) {
		return nil, nil
	}
	return &k, nil
}
