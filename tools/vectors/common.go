package main

import (
	"bytes"
	"crypto/ed25519"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base32"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
)

// seq returns n bytes counting up from first: seq(0x00, 32) is 00 01 … 1f.
func seq(first byte, n int) []byte {
	b := make([]byte, n)
	for i := range b {
		b[i] = first + byte(i)
	}
	return b
}

func mustHex(s string) []byte {
	b, err := hex.DecodeString(s)
	if err != nil {
		panic(err)
	}
	return b
}

func sha(b []byte) []byte {
	h := sha256.Sum256(b)
	return h[:]
}

func shaHex(b []byte) string { return hex.EncodeToString(sha(b)) }

// fingerprint is section 4.2 of v0 and core 0.
func fingerprint(pub []byte) string {
	t := base32.StdEncoding.WithPadding(base32.NoPadding).EncodeToString(sha(pub)[:15])
	var g []string
	for i := 0; i < len(t); i += 4 {
		g = append(g, t[i:i+4])
	}
	return strings.Join(g, "-")
}

// commitment is a next key commitment: SHA-256 of the key, base64url.
func commitment(pub []byte) string { return b64.EncodeToString(sha(pub)) }

// key is a test key.
type key struct {
	seed []byte
	priv ed25519.PrivateKey
	pub  []byte
}

func newKey(seed []byte) key {
	p := ed25519.NewKeyFromSeed(seed)
	return key{seed: seed, priv: p, pub: p.Public().(ed25519.PublicKey)}
}

func (k key) sign(msg string) string { return b64.EncodeToString(ed25519.Sign(k.priv, []byte(msg))) }
func (k key) text() string           { return b64.EncodeToString(k.pub) }

// The labels of each core version.
type core struct {
	signsType bool   // the answer message carries the media type (core 0, not the test draft)
	prefix    string // the code's prefix
	request   string
	response  string
	pairing   string
	header    string // the headers' prefix, as in "<header>-Key"
}

var draftCore = core{false, "overfin-pair-1.", "overfin-federation-1 request", "overfin-federation-1 response", "overfin-federation-1 pairing", "Overfin"}
var core0 = core{true, "msfp-pair-0.", "msfp-0 request", "msfp-0 response", "msfp-0 pairing", "MSFP"}

func (c core) requestMessage(method, host, target, date, nonce, body string) string {
	return strings.Join([]string{c.request, method, host, target, date, nonce, shaHex([]byte(body))}, "\n")
}

// answerMessage: the test draft has four lines; core 0 adds the media type of
// the answer's Content-Type ("-" without one) before the body hash.
func (c core) answerMessage(nonce string, status int, contentType, body string) string {
	lines := []string{c.response, nonce, strconv.Itoa(status)}
	if c.signsType {
		lines = append(lines, mediaType(contentType))
	}
	lines = append(lines, shaHex([]byte(body)))
	return strings.Join(lines, "\n")
}

// mediaType is the type/subtype of a Content-Type value, lowercase,
// without parameters or white space; "-" for an absent header.
func mediaType(ct string) string {
	if i := strings.IndexByte(ct, ';'); i >= 0 {
		ct = ct[:i]
	}
	ct = strings.ToLower(strings.TrimSpace(ct))
	if ct == "" {
		return "-"
	}
	return ct
}

func (c core) proof(secret, redeemer []byte, address string) string {
	m := hmac.New(sha256.New, secret)
	m.Write([]byte(c.pairing + "\n"))
	m.Write(redeemer)
	m.Write([]byte("\n"))
	m.Write([]byte(address))
	return b64.EncodeToString(m.Sum(nil))
}

// encode writes a vector file: two-space indents, no HTML escaping, a
// final line feed.
func encode(v any) []byte {
	var buf bytes.Buffer
	e := json.NewEncoder(&buf)
	e.SetEscapeHTML(false)
	e.SetIndent("", "  ")
	if err := e.Encode(v); err != nil {
		panic(err)
	}
	return buf.Bytes()
}

// openToken tells a token (spec/v0/core.md section 8.5): 1 to 32
// characters of a-z 0-9 . _ -. The value of an open field is kept when
// it is a token, known or not, and dropped alone when it isn't. Every
// part's open fields check their values with it.
func openToken(s string) bool {
	if len(s) < 1 || len(s) > 32 {
		return false
	}
	for _, c := range []byte(s) {
		if !(c >= 'a' && c <= 'z' || c >= '0' && c <= '9' || c == '.' || c == '_' || c == '-') {
			return false
		}
	}
	return true
}

// openTokenValue is v when it is a string and a token (openToken),
// else "": an open field keeps any token, known or not.
func openTokenValue(v any) string {
	if s, ok := v.(string); ok && openToken(s) {
		return s
	}
	return ""
}

// openTokenReason is openToken with the reason a value isn't a token.
func openTokenReason(s string) error {
	if len(s) < 1 || len(s) > 32 {
		return fmt.Errorf("%d bytes, not 1 to 32", len(s))
	}
	for _, c := range []byte(s) {
		if !openToken(string(c)) {
			return fmt.Errorf("byte %q outside a-z 0-9 . _ -", c)
		}
	}
	return nil
}
