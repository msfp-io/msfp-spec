package main

import (
	"crypto/ed25519"
	"crypto/sha512"
	"encoding/hex"
	"fmt"
	"math/big"
	"strings"

	"filippo.io/edwards25519"
)

// The test keys of the test draft's section 15.1, also used by core 0.
var (
	keyA = newKey(seq(0x00, 32))
	keyB = newKey(seq(0x20, 32))
)

const (
	addrA   = "https://a.example.org"
	addrB0  = "http://192.0.2.10:8096" // server B in version 0
	secretS = "404142434445464748494a4b4c4d4e4f505152535455565758595a5b5c5d5e5f"
	expires = 1800086400
)

// ---- vectors/history/test-draft.json, regenerated to check it, never written ----

type v0Key struct {
	Seed        string `json:"seed"`
	Public      string `json:"public"`
	Fingerprint string `json:"fingerprint"`
}

type v0Code struct {
	Key        string `json:"key"`
	Address    string `json:"address"`
	Secret     string `json:"secret"`
	Expires    int64  `json:"expires"`
	Text       string `json:"text"`
	SecretHash string `json:"secretHash"`
}

type v0Proof struct {
	Redeemer string `json:"redeemer"`
	Address  string `json:"address"`
	Value    string `json:"value"`
}

type v0Exchange struct {
	Name            string `json:"name"`
	Signer          string `json:"signer"`
	Answerer        string `json:"answerer"`
	Method          string `json:"method"`
	Host            string `json:"host"`
	Target          string `json:"target"`
	Date            string `json:"date"`
	Nonce           string `json:"nonce"`
	NonceText       string `json:"nonceText"`
	Body            string `json:"body"`
	Message         string `json:"message"`
	Signature       string `json:"signature"`
	Status          int    `json:"status"`
	AnswerBody      string `json:"answerBody"`
	AnswerMessage   string `json:"answerMessage"`
	AnswerSignature string `json:"answerSignature"`
}

type v0File struct {
	Keys      map[string]v0Key `json:"keys"`
	Code      v0Code           `json:"code"`
	Proof     v0Proof          `json:"proof"`
	Exchanges []v0Exchange     `json:"exchanges"`
}

var testKeys = map[string]key{"a": keyA, "b": keyB}

func v0KeyOf(k key) v0Key {
	return v0Key{hex.EncodeToString(k.seed), k.text(), fingerprint(k.pub)}
}

func v0Exch(name, signer, answerer, host, target, date string, nonce []byte, body string, status int, answerBody string) v0Exchange {
	s, a := testKeys[signer], testKeys[answerer]
	nt := b64.EncodeToString(nonce)
	msg := draftCore.requestMessage("POST", host, target, date, nt, body)
	am := draftCore.answerMessage(nt, status, "", answerBody)
	return v0Exchange{name, signer, answerer, "POST", host, target, date, hex.EncodeToString(nonce), nt,
		body, msg, s.sign(msg), status, answerBody, am, a.sign(am)}
}

func genDraft() []byte {
	secret := mustHex(secretS)
	sh := b64.EncodeToString(sha(secret))
	proof := draftCore.proof(secret, keyB.pub, addrB0)
	pairBody := fmt.Sprintf(`{"address":%q,"name":"Server B","secretHash":%q,"proof":%q}`, addrB0, sh, proof)
	hello := `{"state":"paired"}`
	f := v0File{
		Keys:  map[string]v0Key{"a": v0KeyOf(keyA), "b": v0KeyOf(keyB)},
		Code:  v0Code{"a", addrA, secretS, expires, makeCode(draftCore.prefix, keyA.pub, secret, expires, addrA), sh},
		Proof: v0Proof{"b", addrB0, proof},
		Exchanges: []v0Exchange{
			v0Exch("pair", "b", "a", "a.example.org", "/api/v1/federation/pair", "1800000000", seq(0x60, 16), pairBody, 200, `{"name":"Server A"}`),
			v0Exch("hello", "a", "b", "192.0.2.10:8096", "/api/v1/federation/hello", "1800000060", seq(0x70, 16), hello, 200, hello),
			v0Exch("refused hello", "a", "b", "192.0.2.10:8096", "/api/v1/federation/hello", "1800000120", seq(0x80, 16), hello, 404, notFound),
		},
	}
	return encode(f)
}

const notFound = `{"type":"not-found","title":"Not found","status":404}` + "\n"

// ---- vectors/v0-strict.json ----

type sigVector struct {
	Name      string `json:"name"`
	Public    string `json:"public"`
	Message   string `json:"message"`
	Signature string `json:"signature"`
	Valid     bool   `json:"valid"`
	Reason    string `json:"reason,omitempty"`
	Comment   string `json:"comment,omitempty"`
}

type strictFile struct {
	About      []string    `json:"about"`
	Keys       []verdict   `json:"keys"`
	Signatures []sigVector `json:"signatures"`
	Base64url  []verdict   `json:"base64url"`
	Dates      []verdict   `json:"dates"`
	Integers   []verdict   `json:"integers"`
	JSON       []verdict   `json:"json"`
	Addresses  []verdict   `json:"addresses"`
	Codes      []verdict   `json:"codes"`
}

func keyVerdict(name string, pub []byte, comment string) verdict {
	ok, r := reason(checkKey(pub))
	return verdict{Name: name, Text: b64.EncodeToString(pub), Valid: ok, Reason: r, Comment: comment}
}

func sigVerdict(name string, pub []byte, msg string, sig []byte, comment string) sigVector {
	ok, r := reason(verifyStrict(pub, []byte(msg), sig))
	return sigVector{name, b64.EncodeToString(pub), msg, b64.EncodeToString(sig), ok, r, comment}
}

// torsionPoint finds a point of order 8, deterministically: it maps
// hashes to points and keeps their small-order part.
func torsionPoint() *edwards25519.Point {
	id := edwards25519.NewIdentityPoint()
	for i := 0; ; i++ {
		h := sha(fmt.Appendf(nil, "msfp-spec torsion %d", i))
		h[31] &= 0x7f
		p, err := new(edwards25519.Point).SetBytes(h)
		if err != nil {
			continue
		}
		t := timesL(p)
		four := new(edwards25519.Point).Add(t, t)
		four.Add(four, four)
		if four.Equal(id) != 1 {
			return t // [4]T isn't the identity: T has order 8
		}
	}
}

// nonCanonicalKey finds a valid point of the prime-order subgroup whose
// y coordinate also has the encoding y + p (only y below 19 do), or, if
// none, returns the encoding of the identity with y = p + 1.
func nonCanonicalKey() ([]byte, string) {
	p := new(big.Int).Sub(new(big.Int).Lsh(big.NewInt(1), 255), big.NewInt(19))
	for y := int64(2); y < 19; y++ {
		for _, sign := range []byte{0, 0x80} {
			v := new(big.Int).Add(p, big.NewInt(y)).FillBytes(make([]byte, 32))
			for i, j := 0, 31; i < j; i, j = i+1, j-1 {
				v[i], v[j] = v[j], v[i]
			}
			v[31] |= sign
			pt, err := new(edwards25519.Point).SetBytes(v)
			if err != nil {
				continue
			}
			if timesL(pt).Equal(edwards25519.NewIdentityPoint()) == 1 {
				return v, fmt.Sprintf("y = p + %d: a point of the subgroup, written with y above p", y)
			}
		}
	}
	v := new(big.Int).Add(p, big.NewInt(1)).FillBytes(make([]byte, 32))
	for i, j := 0, 31; i < j; i, j = i+1, j-1 {
		v[i], v[j] = v[j], v[i]
	}
	return v, "y = p + 1: the identity, written with y above p"
}

func scalarOfSeed(seed []byte) *edwards25519.Scalar {
	h := sha512.Sum512(seed)
	s, err := edwards25519.NewScalar().SetBytesWithClamping(h[:32])
	if err != nil {
		panic(err)
	}
	return s
}

func hramScalar(r, a []byte, msg string) *edwards25519.Scalar {
	h := sha512.New()
	h.Write(r)
	h.Write(a)
	h.Write([]byte(msg))
	k, err := edwards25519.NewScalar().SetUniformBytes(h.Sum(nil))
	if err != nil {
		panic(err)
	}
	return k
}

// addL returns S + L, little-endian, 32 bytes.
func addL(s []byte) []byte {
	out := make([]byte, 32)
	carry := 0
	for i := range 32 {
		v := int(s[i]) + int(orderL[i]) + carry
		out[i] = byte(v)
		carry = v >> 8
	}
	return out
}

func genDraftStrict() []byte {
	v0 := genV0Message()
	id := edwards25519.NewIdentityPoint().Bytes()
	tp := torsionPoint()
	mixed := new(edwards25519.Point).Add(mustPoint(keyA.pub), tp).Bytes()
	nc, ncComment := nonCanonicalKey()

	// The signatures.
	honest := ed25519.Sign(keyA.priv, []byte(v0))
	identitySig := append(edwards25519.NewIdentityPoint().Bytes(), make([]byte, 32)...)
	sPlusL := append(honest[:32:32], addL(honest[32:])...)
	r, _ := edwards25519.NewScalar().SetUniformBytes(sha512Of("msfp-spec torsion nonce"))
	rPoint := new(edwards25519.Point).ScalarBaseMult(r)
	rPoint.Add(rPoint, tp)
	rb := rPoint.Bytes()
	k := hramScalar(rb, keyA.pub, v0)
	s := edwards25519.NewScalar().MultiplyAdd(k, scalarOfSeed(keyA.seed), r)
	torsionSig := append(rb, s.Bytes()...)

	f := strictFile{
		About: []string{
			"Negative and positive cases for the canonical forms of the test draft of November 2026 (spec/history/test-draft.md section 2.1, corrections of 2026-11-16). They hold for core version 0 too, except that core 0 allows https addresses only (see vectors/v0/strict.json).",
			"Generated by tools/vectors. \"valid\" is what a receiver must decide. \"reason\" is the generator's own wording, for people; an implementation needn't give the same.",
			"Some invalid cases pass common libraries: Go's crypto/ed25519 and OpenSSL accept the identity key's signature and the non-canonical key. A receiver must make the checks of section 2.1 itself.",
			"integers: the text of a JSON number where an integer is expected. A number that isn't one (1.0, 1e3) is still valid I-JSON (see json), and is invalid only there; a version list ignores it (core 0 sections 2.3 and 10.2, vectors/v0/strict.json helloVersions).",
		},
		Keys: []verdict{
			keyVerdict("key a", keyA.pub, "an honest key"),
			keyVerdict("the identity", id, "small order; with the signature below, it verifies every message under a cofactorless check"),
			keyVerdict("a point of order 8", tp.Bytes(), ""),
			keyVerdict("key a plus a point of order 8", mixed, "canonical and on the curve, outside the subgroup"),
			keyVerdict("non-canonical", nc, ncComment),
		},
		Signatures: []sigVector{
			sigVerdict("honest", keyA.pub, v0, honest, "key a's signature of the message of v0.json's \"hello\" exchange"),
			sigVerdict("identity key, R = identity, S = 0", id, v0, identitySig, "accepted by Go's crypto/ed25519 for any message"),
			sigVerdict("torsion R", keyA.pub, v0, torsionSig, "R = [r]B + T, T of order 8, S = r + k·a: accepted by a cofactored check only"),
			sigVerdict("S + L", keyA.pub, v0, sPlusL, "the honest signature with L added to S"),
		},
		Base64url: b64Verdicts(),
		Dates:     dateVerdicts(),
		Integers:  intVerdicts(),
		JSON:      jsonVerdicts(),
		Addresses: addressVerdicts(true),
		Codes:     codeVerdicts(draftCore, true),
	}
	return encode(f)
}

func sha512Of(s string) []byte { h := sha512.Sum512([]byte(s)); return h[:] }

func mustPoint(b []byte) *edwards25519.Point {
	p, err := new(edwards25519.Point).SetBytes(b)
	if err != nil {
		panic(err)
	}
	return p
}

// genV0Message is the request message of v0.json's "hello" exchange.
func genV0Message() string {
	return draftCore.requestMessage("POST", "192.0.2.10:8096", "/api/v1/federation/hello", "1800000060",
		b64.EncodeToString(seq(0x70, 16)), `{"state":"paired"}`)
}

func b64Verdicts() []verdict {
	one := func(name, text string, n int) verdict {
		_, err := checkB64(text, n)
		ok, r := reason(err)
		return verdict{Name: name, Text: text, Valid: ok, Reason: r}
	}
	a := keyA.text()
	last := a[len(a)-1]
	other := strings.IndexByte("ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789-_", last) + 1
	alt := a[:len(a)-1] + string("ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789-_"[other])
	nonce := b64.EncodeToString(seq(0x70, 16))
	nalt := nonce[:len(nonce)-1] + "_"
	return []verdict{
		one("key a, 32 bytes", a, 32),
		one("key a, a nonzero unused bit in the last character", alt, 32),
		one("key a, padded", a+"=", 32),
		one("key a, a line feed inside", a[:20]+"\n"+a[20:], 32),
		one("key a, standard base64 alphabet", strings.NewReplacer("-", "+", "_", "/").Replace(a), 32),
		one("a nonce, 16 bytes", nonce, 16),
		one("a nonce, nonzero unused bits", nalt, 16),
		one("a nonce, 15 bytes", b64.EncodeToString(seq(0x70, 15)), 16),
	}
}

func dateVerdicts() []verdict {
	var out []verdict
	for _, t := range []string{"1800000000", "+1800000000", "01800000000", "-5", "0", "1800000000.0", "9007199254740991", "9007199254740992", "18 00000000"} {
		ok, r := reason(checkDate(t))
		out = append(out, verdict{Text: t, Valid: ok, Reason: r})
	}
	return out
}

func intVerdicts() []verdict {
	var out []verdict
	for _, t := range []string{"0", "120", "-1", "-0", "1.0", "1e3", "01", "9007199254740991", "9007199254740992", "-9007199254740991", "-9007199254740992"} {
		ok, r := reason(checkInteger(t))
		out = append(out, verdict{Text: t, Valid: ok, Reason: r})
	}
	return out
}

func jsonVerdicts() []verdict {
	one := func(name, body string) verdict {
		ok, r := reason(checkIJSON([]byte(body)))
		v := verdict{Name: name, Valid: ok, Reason: r}
		if isPlain(body) {
			v.Text = body
		} else {
			v.Hex = hex.EncodeToString([]byte(body))
		}
		return v
	}
	return []verdict{
		one("a hello body", `{"state":"paired"}`),
		one("white space between tokens", "{ \"state\" : \"paired\" }"),
		one("the same member twice", `{"state":"paired","state":"ended"}`),
		one("the same member twice, once escaped", `{"state":"paired","st\u0061te":"ended"}`),
		one("a lone high surrogate, escaped", `{"name":"\ud800"}`),
		one("a lone low surrogate, escaped", `{"name":"\udc00x"}`),
		one("a surrogate pair, escaped", `{"name":"\ud83c\udfac"}`),
		one("a noncharacter, escaped", `{"name":"\uffff"}`),
		one("invalid UTF-8", "{\"name\":\"\xff\"}"),
		one("a byte order mark", "\xef\xbb\xbf{}"),
		one("a name in another case: valid I-JSON, an unknown member", `{"State":"ended"}`),
		one("a number with a fraction: valid I-JSON, not an integer", `{"core":[1,2.5]}`),
	}
}

// isPlain: printable ASCII, so the body reads well as text in the file.
func isPlain(s string) bool {
	for i := 0; i < len(s); i++ {
		if s[i] < 0x20 || s[i] > 0x7e {
			return false
		}
	}
	return true
}

func addressVerdicts(allowHTTP bool) []verdict {
	list := []string{
		"https://a.example.org",
		"https://a.example.org:8443",
		"http://192.0.2.10:8096",
		"https://192.0.2.10",
		"https://198.51.100.7:8920",
		"HTTPS://a.example.org",
		"https://A.example.org",
		"https://a.example.org/",
		"https://a.example.org/media",
		"https://a.example.org?",
		"https://a.example.org#",
		"https://user@a.example.org",
		"https://a.example.org:443",
		"http://192.0.2.10:80",
		"https://a.example.org:0",
		"https://a.example.org:99999",
		"https://a.example.org:0443",
		"https://a.example.org:",
		"https://192.0.2.010",
		"https://192.0.2",
		"https://-a.example.org",
		"https://a-.example.org",
		"https://a..example.org",
		"https://a_b.example.org",
		"https://" + strings.Repeat("a", 64) + ".example.org",
		"ftp://a.example.org",
		"https://",
	}
	var out []verdict
	for _, a := range list {
		ok, r := reason(checkAddress(a, allowHTTP))
		out = append(out, verdict{Text: a, Valid: ok, Reason: r})
	}
	return out
}

func codeVerdicts(c core, allowHTTP bool) []verdict {
	secret := mustHex(secretS)
	good := makeCode(c.prefix, keyA.pub, secret, expires, addrA)
	one := func(name, text string) verdict {
		_, err := readCode(text, c.prefix, allowHTTP)
		ok, r := reason(err)
		return verdict{Name: name, Text: text, Valid: ok, Reason: r}
	}
	wrapped := good[:40] + "\r\n  " + good[40:90] + "\n\t" + good[90:] + " "
	out := []verdict{
		one(exampleName(c.prefix), good),
		one("wrapped, with white space", wrapped),
		one("the prefix in capitals", strings.ToUpper(c.prefix[:1])+good[1:]),
		one("an address with a trailing slash", makeCode(c.prefix, keyA.pub, secret, expires, addrA+"/")),
		one("an expiry of 2^53", makeCode(c.prefix, keyA.pub, secret, 1<<53, addrA)),
		one("the identity as key", makeCode(c.prefix, edwards25519.NewIdentityPoint().Bytes(), secret, expires, addrA)),
		one("no address", makeCode(c.prefix, keyA.pub, secret, expires, "")),
		one("an http address", makeCode(c.prefix, keyA.pub, secret, expires, "http://192.0.2.10:8096")),
	}
	if c.signsType {
		out = append(out, one("a test draft code", makeCode(draftCore.prefix, keyA.pub, secret, expires, addrA)))
		// Codes with a group (core section 5.1, groups section 2).
		_, g := groupEvents()
		gid, _ := b64.DecodeString(g)
		long := strings.Repeat("é", 81)
		out = append(out,
			one("a code with a group", makeGroupCode(c.prefix, keyA.pub, secret, expires, gid, "Family", addrA)),
			one("a group name of 80 code points", makeGroupCode(c.prefix, keyA.pub, secret, expires, gid, long[:160], addrA)),
			one("a group name of 81 code points", makeGroupCode(c.prefix, keyA.pub, secret, expires, gid, long, addrA)),
			one("a flag byte of 2", rawCode(c.prefix, keyA.pub, secret, expires, append([]byte{2}, addrA...))),
			one("a group name of 0 bytes", rawCode(c.prefix, keyA.pub, secret, expires, append(append(append([]byte{1}, gid...), 0), addrA...))),
			one("a group name that isn't UTF-8", rawCode(c.prefix, keyA.pub, secret, expires, append(append(append([]byte{1}, gid...), 2, 0xff, 0xfe), addrA...))),
			one("a flag of 1 with the group cut short", rawCode(c.prefix, keyA.pub, secret, expires, append([]byte{1}, gid[:20]...))),
			one("a flag of 0 and no address", rawCode(c.prefix, keyA.pub, secret, expires, []byte{0})),
		)
	}
	return out
}

// exampleName names the code example by where each version's text shows
// it: the test draft's section 15.2, core 0's section 5.3.
func exampleName(prefix string) string {
	if strings.HasPrefix(prefix, "msfp-pair-0") {
		return "the code of core section 5.3"
	}
	return "the code of section 15.2"
}
