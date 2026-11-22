package main

// verifyAll reads the vector files back and re-checks every value from
// the files' own fields: keys from seeds, codes from their parts,
// messages rebuilt from their fields, every signature under the strict
// rules, every verdict against this program's checks.

import (
	"crypto/ed25519"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"reflect"
	"slices"
	"strconv"
)

type obj = map[string]any

func str(o obj, k string) string { s, _ := o[k].(string); return s }
func num(o obj, k string) int64  { f, _ := o[k].(float64); return int64(f) }

func verifyAll(read func(string) []byte) []error {
	var errs []error
	bad := func(format string, a ...any) { errs = append(errs, fmt.Errorf(format, a...)) }
	load := func(rel string) obj {
		var o obj
		b := read(rel)
		if err := checkIJSON(b); err != nil {
			bad("%s: %v", rel, err)
		}
		if err := json.Unmarshal(b, &o); err != nil {
			bad("%s: %v", rel, err)
		}
		return o
	}

	// Keys: public key and fingerprint from the seed, and the key valid.
	pubs := map[string][]byte{}
	checkKeys := func(file string, keys obj, withCommitment bool) {
		for name, v := range keys {
			k := v.(obj)
			seed, err := hex.DecodeString(str(k, "seed"))
			if err != nil || len(seed) != 32 {
				bad("%s: key %s: bad seed", file, name)
				continue
			}
			pub := ed25519.NewKeyFromSeed(seed).Public().(ed25519.PublicKey)
			if b64.EncodeToString(pub) != str(k, "public") {
				bad("%s: key %s: public key isn't the seed's", file, name)
			}
			if err := checkKey(pub); err != nil {
				bad("%s: key %s: %v", file, name, err)
			}
			if fingerprint(pub) != str(k, "fingerprint") {
				bad("%s: key %s: wrong fingerprint", file, name)
			}
			if withCommitment && commitment(pub) != str(k, "commitment") {
				bad("%s: key %s: wrong commitment", file, name)
			}
			pubs[name] = pub
		}
	}

	checkCode := func(file string, c core, allowHTTP bool, cd, pf obj) {
		got, err := readCode(str(cd, "text"), c.prefix, allowHTTP)
		if err != nil {
			bad("%s: code: %v", file, err)
			return
		}
		secret, _ := hex.DecodeString(str(cd, "secret"))
		if b64.EncodeToString(got.key) != b64.EncodeToString(pubs[str(cd, "key")]) ||
			hex.EncodeToString(got.secret) != str(cd, "secret") ||
			got.expires != num(cd, "expires") || got.address != str(cd, "address") {
			bad("%s: code: its parts differ from the fields", file)
		}
		if b64.EncodeToString(sha(secret)) != str(cd, "secretHash") {
			bad("%s: wrong secretHash", file)
		}
		if err := checkAddress(str(pf, "address"), allowHTTP); err != nil {
			bad("%s: proof address: %v", file, err)
		}
		if c.proof(secret, pubs[str(pf, "redeemer")], str(pf, "address")) != str(pf, "value") {
			bad("%s: wrong proof", file)
		}
	}

	checkSig := func(where, signer, msg, sig string) {
		s, err := checkB64(sig, 64)
		if err != nil {
			bad("%s: signature: %v", where, err)
			return
		}
		if err := verifyStrict(pubs[signer], []byte(msg), s); err != nil {
			bad("%s: signature: %v", where, err)
		}
	}

	checkExchange := func(file string, c core, e obj) {
		where := file + ": " + str(e, "name")
		nonce, err := hex.DecodeString(str(e, "nonce"))
		if err != nil || len(nonce) != 16 || b64.EncodeToString(nonce) != str(e, "nonceText") {
			bad("%s: nonce and nonceText differ", where)
		}
		if err := checkDate(str(e, "date")); err != nil {
			bad("%s: date: %v", where, err)
		}
		for _, k := range []string{"body", "answerBody"} {
			if err := checkIJSON([]byte(str(e, k))); err != nil {
				bad("%s: %s: %v", where, k, err)
			}
		}
		msg := c.requestMessage(str(e, "method"), str(e, "host"), str(e, "target"), str(e, "date"), str(e, "nonceText"), str(e, "body"))
		if msg != str(e, "message") {
			bad("%s: message isn't rebuilt from the fields", where)
		}
		checkSig(where+" request", str(e, "signer"), msg, str(e, "signature"))
		am := c.answerMessage(str(e, "nonceText"), int(num(e, "status")), str(e, "answerContentType"), str(e, "answerBody"))
		if am != str(e, "answerMessage") {
			bad("%s: answerMessage isn't rebuilt from the fields", where)
		}
		checkSig(where+" answer", str(e, "answerer"), am, str(e, "answerSignature"))
	}

	// vectors/v0.json
	v0 := load("history/test-draft.json")
	checkKeys("history/test-draft.json", v0["keys"].(obj), false)
	checkCode("history/test-draft.json", draftCore, true, v0["code"].(obj), v0["proof"].(obj))
	for _, e := range v0["exchanges"].([]any) {
		checkExchange("history/test-draft.json", draftCore, e.(obj))
	}

	// vectors/v0-strict.json and vectors/v0/strict.json
	verdicts := func(file, list string, f func(obj) error) {
		o := load(file)
		for _, v := range o[list].([]any) {
			x := v.(obj)
			err := f(x)
			if (err == nil) != (x["valid"] == true) {
				bad("%s: %s %q%q: the file says valid=%v, the check says %v", file, list, str(x, "name"), str(x, "text"), x["valid"], err)
			}
		}
	}
	text := func(x obj) []byte {
		if h := str(x, "hex"); h != "" {
			b, _ := hex.DecodeString(h)
			return b
		}
		return []byte(str(x, "text"))
	}
	verdicts("history/test-draft-strict.json", "keys", func(x obj) error {
		b, err := b64.DecodeString(str(x, "text"))
		if err != nil {
			return err
		}
		return checkKey(b)
	})
	verdicts("history/test-draft-strict.json", "signatures", func(x obj) error {
		pub, _ := b64.DecodeString(str(x, "public"))
		sig, _ := b64.DecodeString(str(x, "signature"))
		return verifyStrict(pub, []byte(str(x, "message")), sig)
	})
	verdicts("history/test-draft-strict.json", "base64url", func(x obj) error {
		n := 32
		if len(str(x, "name")) >= 7 && str(x, "name")[:7] == "a nonce" {
			n = 16
		}
		_, err := checkB64(str(x, "text"), n)
		return err
	})
	verdicts("history/test-draft-strict.json", "dates", func(x obj) error { return checkDate(str(x, "text")) })
	verdicts("history/test-draft-strict.json", "integers", func(x obj) error { return checkInteger(str(x, "text")) })
	verdicts("history/test-draft-strict.json", "json", func(x obj) error { return checkIJSON(text(x)) })
	verdicts("history/test-draft-strict.json", "addresses", func(x obj) error { return checkAddress(str(x, "text"), true) })
	verdicts("history/test-draft-strict.json", "codes", func(x obj) error { _, err := readCode(str(x, "text"), draftCore.prefix, true); return err })
	verdicts("v0/strict.json", "addresses", func(x obj) error { return checkAddress(str(x, "text"), false) })
	verdicts("v0/strict.json", "codes", func(x obj) error { _, err := readCode(str(x, "text"), core0.prefix, false); return err })
	verdicts("v0/strict.json", "mediaNames", func(x obj) error { return checkMediaName(str(x, "text")) })
	verdicts("v0/strict.json", "bases", func(x obj) error { return checkBase(str(x, "text")) })
	verdicts("v0/strict.json", "destinations", func(x obj) error { return checkDestination(str(x, "text")) })
	verdicts("v0/strict.json", "uncallableAddresses", func(x obj) error { return checkUncallableAddress(str(x, "text")) })
	for _, v := range load("v0/strict.json")["helloVersions"].([]any) {
		x := v.(obj)
		got, err := keptVersions([]byte(str(x, "text")))
		if (err == nil) != (x["valid"] == true) || got != str(x, "output") {
			bad("v0/strict.json: helloVersions %q: the file says valid=%v, output %q; the rules give %v, %q", str(x, "name"), x["valid"], str(x, "output"), err, got)
		}
	}
	for _, v := range load("v0/strict.json")["languageCodes"].([]any) {
		x := v.(obj)
		got, err := sentTag(str(x, "text"))
		if (err == nil) != (x["valid"] == true) || got != str(x, "output") {
			bad("v0/strict.json: languageCodes %q: the file says valid=%v, output %q; the rules give %v, %q", str(x, "text"), x["valid"], str(x, "output"), err, got)
		}
	}
	verdicts("v0/media.json", "hls", func(x obj) error { return checkPlaylist([]byte(str(x, "text"))) })
	verdicts("v0/media.json", "pgs", func(x obj) error { return checkPGS(text(x)) })
	for _, v := range load("v0/media.json")["webvtt"].([]any) {
		x := v.(obj)
		got, err := rebuildVTT(text(x))
		if (err == nil) != (x["valid"] == true) || string(got) != str(x, "output") {
			bad("v0/media.json: webvtt %q: the file says valid=%v, output %q; the rules give %v, %q", str(x, "name"), x["valid"], str(x, "output"), err, got)
		}
	}

	mf := load("v0/music.json")
	var libs []string
	for _, l := range mf["libraries"].([]any) {
		libs = append(libs, l.(string))
	}
	for _, v := range mf["keys"].([]any) {
		x := v.(obj)
		kind, err := checkMusicKey(str(x, "text"))
		if (err == nil) != (x["valid"] == true) || kind != str(x, "kind") {
			bad("v0/music.json: key %q: the file says valid=%v, kind %q; the rules give %v, %q", str(x, "text"), x["valid"], str(x, "kind"), err, kind)
		}
	}
	for _, v := range mf["titles"].([]any) {
		x := v.(obj)
		got, err := keepMusicTitle([]byte(str(x, "text")), libs)
		if (err == nil) != (x["valid"] == true) || string(got) != str(x, "output") {
			bad("v0/music.json: title %q: the file says valid=%v, output %q; the rules give %v, %q", str(x, "name"), x["valid"], str(x, "output"), err, got)
		}
		if err == nil {
			if e := checkIJSON(got); e != nil {
				bad("v0/music.json: title %q: output: %v", str(x, "name"), e)
			}
		}
	}

	videoFile := load("v0/video.json")
	var vlibs []string
	for _, l := range videoFile["libraries"].([]any) {
		vlibs = append(vlibs, l.(string))
	}
	for _, v := range videoFile["titles"].([]any) {
		x := v.(obj)
		got, err := keepVideoTitle([]byte(str(x, "text")), vlibs)
		if (err == nil) != (x["valid"] == true) || string(got) != str(x, "output") {
			bad("v0/video.json: title %q: the file says valid=%v, output %q; the rules give %v, %q", str(x, "name"), x["valid"], str(x, "output"), err, got)
		}
		if err == nil {
			if e := checkIJSON(got); e != nil {
				bad("v0/video.json: title %q: output: %v", str(x, "name"), e)
			}
		}
	}

	// vectors/v0/
	keys := load("v0/keys.json")
	checkKeys("v0/keys.json", keys["keys"].(obj), true)
	for name, v := range keys["certificates"].(obj) {
		c := v.(obj)
		der, _ := hex.DecodeString(str(c, "spki"))
		if b64.EncodeToString(sha(der)) != str(c, "pin") {
			bad("v0/keys.json: certificate %s: wrong pin", name)
		}
		seed, _ := hex.DecodeString(str(c, "seed"))
		if hex.EncodeToString(spki(newKey(seed))) != str(c, "spki") {
			bad("v0/keys.json: certificate %s: spki isn't the seed's", name)
		}
	}
	cf := load("v0/code.json")
	checkCode("v0/code.json", core0, false, cf["code"].(obj), cf["proof"].(obj))
	for _, e := range load("v0/exchanges.json")["exchanges"].([]any) {
		checkExchange("v0/exchanges.json", core0, e.(obj))
		checkPrivate("v0/exchanges.json: "+str(e.(obj), "name"), e.(obj), pubs, bad)
	}

	rf := load("v0/rotation.json")
	r := rf["rotation"].(obj)
	old, nu := pubs[str(r, "from")], pubs[str(r, "to")]
	if b64.EncodeToString(old) != str(r, "oldKey") || b64.EncodeToString(nu) != str(r, "newKey") ||
		commitment(pubs[str(r, "next")]) != str(r, "nextCommitment") {
		bad("v0/rotation.json: keys or commitment differ from keys.json")
	}
	st := rotationStatement(old, nu, str(r, "nextCommitment"), num(r, "date"))
	if st != str(r, "statement") {
		bad("v0/rotation.json: statement isn't rebuilt from the fields")
	}
	checkSig("v0/rotation.json old", str(r, "from"), st, str(r, "oldSignature"))
	checkSig("v0/rotation.json new", str(r, "to"), st, str(r, "newSignature"))
	re := rf["exchange"].(obj)
	checkExchange("v0/rotation.json", core0, re)
	var rb obj
	_ = json.Unmarshal([]byte(str(re, "body")), &rb)
	if str(rb, "newKey") != str(r, "newKey") || str(rb, "nextCommitment") != str(r, "nextCommitment") ||
		num(rb, "date") != num(r, "date") || str(rb, "oldSignature") != str(r, "oldSignature") ||
		str(rb, "newSignature") != str(r, "newSignature") || str(re, "signer") != str(r, "from") {
		bad("v0/rotation.json: the rotate request doesn't carry the statement")
	}

	vf := load("v0/revocation.json")
	v := vf["revocation"].(obj)
	vs := revocationStatement(pubs[str(v, "key")], num(v, "date"))
	if vs != str(v, "statement") || b64.EncodeToString(pubs[str(v, "key")]) != str(v, "public") {
		bad("v0/revocation.json: statement isn't rebuilt from the fields")
	}
	checkSig("v0/revocation.json", str(v, "key"), vs, str(v, "signature"))
	ve := vf["exchange"].(obj)
	checkExchange("v0/revocation.json", core0, ve)
	var vb obj
	_ = json.Unmarshal([]byte(str(ve, "body")), &vb)
	if strconv.FormatInt(num(vb, "date"), 10) != strconv.FormatInt(num(v, "date"), 10) ||
		str(vb, "signature") != str(v, "signature") || str(ve, "signer") != str(v, "key") {
		bad("v0/revocation.json: the revoke request doesn't carry the statement")
	}

	// vectors/v0/groups.json: each event judged again from its envelope,
	// the state compared, the code read, the mail checked and opened with
	// Go's crypto/hpke.
	gf := load("v0/groups.json")
	var gevents []gEvent
	for _, v := range gf["events"].([]any) {
		e := v.(obj)
		ge := gEvent{Name: str(e, "name"), Event: str(e, "event"), Signature: str(e, "signature"), ID: str(e, "id"), Valid: e["valid"] == true}
		pb, _ := b64.DecodeString(ge.Event)
		if string(pb) != str(e, "payload") || b64.EncodeToString(sha(pb)) != ge.ID {
			bad("v0/groups.json: %s: payload or id isn't the event's", ge.Name)
		}
		gevents = append(gevents, ge)
	}
	// Judged as a set, in the file's order and reversed: the same.
	judged, gstate := judgeGroupEvents(gevents, str(gf, "group"))
	reversed := slices.Clone(gevents)
	slices.Reverse(reversed)
	if _, again := judgeGroupEvents(reversed, str(gf, "group")); !reflect.DeepEqual(again, gstate) {
		bad("v0/groups.json: the state depends on the events' order")
	}
	for i, e := range judged {
		if e.Valid != gevents[i].Valid {
			bad("v0/groups.json: %s: valid is %v, the rules say %v (%s)", e.Name, gevents[i].Valid, e.Valid, e.Reason)
		}
	}
	want, _ := json.Marshal(gf["state"])
	got, _ := json.Marshal(gstate)
	var w, g2 any
	_ = json.Unmarshal(want, &w)
	_ = json.Unmarshal(got, &g2)
	if wb, _ := json.Marshal(w); string(wb) != func() string { b, _ := json.Marshal(g2); return string(b) }() {
		bad("v0/groups.json: state isn't what the events give")
	}
	gcode := gf["code"].(obj)
	gc, err := readCode(str(gcode, "text"), core0.prefix, false)
	if err != nil {
		bad("v0/groups.json: code: %v", err)
	} else if b64.EncodeToString(gc.group) != str(gf, "group") || gc.groupName != str(gf, "groupName") ||
		gc.address != str(gcode, "address") || b64.EncodeToString(gc.key) != keyB.text() {
		bad("v0/groups.json: code: its group, name, address or key differ")
	}
	if jp := gf["joinProof"].(obj); str(jp, "redeemer") != "d" || str(jp, "value") != joinProof(mustHex(secretS), keyGD.pub) {
		bad("v0/groups.json: joinProof isn't group-join's proof of the code by d")
	}
	m := gf["mail"].(obj)
	mb, _ := b64.DecodeString(str(m, "mail"))
	if string(mb) != str(m, "payload") {
		bad("v0/groups.json: mail isn't its payload")
	}
	pubs["group n"] = keyGN.pub
	checkSig("v0/groups.json mail", "group n", groupMailLabel+string(mb), str(m, "signature"))
	var mp obj
	_ = json.Unmarshal(mb, &mp)
	sealed, _ := b64.DecodeString(str(mp, "sealed"))
	priv, _ := hex.DecodeString(str(m, "recipientX25519Private"))
	info, _ := hex.DecodeString(str(m, "info"))
	aad, _ := hex.DecodeString(str(m, "aad"))
	gid, _ := b64.DecodeString(str(mp, "group"))
	mid, _ := b64.DecodeString(str(mp, "id"))
	from, _ := b64.DecodeString(str(mp, "from"))
	to, _ := b64.DecodeString(str(mp, "to"))
	if hex.EncodeToString(mailInfo(gid, mid)) != str(m, "info") || hex.EncodeToString(append(from, to...)) != str(m, "aad") {
		bad("v0/groups.json: mail info or aad aren't the payload's")
	}
	if opened, err := openMail(mustX25519(priv), sealed, info, aad); err != nil || string(opened) != str(m, "opened") {
		bad("v0/groups.json: the mail doesn't open to opened with crypto/hpke: %v", err)
	}
	// The revision of version 0 (proposal 16): each part checks its own
	// vectors in its own file (verify_rev_<part>.go).
	errs = append(errs, verifyRevCatalog(read)...)
	errs = append(errs, verifyRevVideo(read)...)
	errs = append(errs, verifyRevMusic(read)...)
	errs = append(errs, verifyRevMedia(read)...)
	errs = append(errs, verifyRevPlayback(read)...)
	return errs
}

// checkPrivate checks the members of a server that can't be called
// (spec/v0/core.md sections 5.2, 7.5, 12.4 and 12.6) in an exchange: an
// inbound false carries an address of the uncallable form, knownKey is a
// test key, and the rotation and revocation an answer carries check out
// under the key that signed the answer.
func checkPrivate(where string, e obj, pubs map[string][]byte, bad func(string, ...any)) {
	var req, ans obj
	_ = json.Unmarshal([]byte(str(e, "body")), &req)
	_ = json.Unmarshal([]byte(str(e, "answerBody")), &ans)
	for _, m := range []obj{req, ans} {
		if v, ok := m["inbound"]; ok {
			if v != false {
				bad("%s: inbound isn't false", where)
			} else if err := checkUncallableAddress(str(m, "address")); err != nil {
				bad("%s: inbound false with address %q: %v", where, str(m, "address"), err)
			}
		}
	}
	if k, ok := req["knownKey"]; ok {
		found := false
		for _, p := range pubs {
			if b64.EncodeToString(p) == k {
				found = true
			}
		}
		if !found {
			bad("%s: knownKey isn't a test key", where)
		}
		if k != b64.EncodeToString(pubs[str(e, "answerer")]) {
			bad("%s: the answer isn't signed with the knownKey", where)
		}
	}
	old := pubs[str(e, "answerer")]
	if r, ok := ans["rotation"].(obj); ok {
		nu, err := checkB64(str(r, "newKey"), 32)
		if err == nil {
			err = checkKey(nu)
		}
		if err != nil {
			bad("%s: rotation newKey: %v", where, err)
			return
		}
		st := rotationStatement(old, nu, str(r, "nextCommitment"), num(r, "date"))
		for _, s := range []struct {
			pub []byte
			sig string
		}{{old, str(r, "oldSignature")}, {nu, str(r, "newSignature")}} {
			sig, err := checkB64(s.sig, 64)
			if err == nil {
				err = verifyStrict(s.pub, []byte(st), sig)
			}
			if err != nil {
				bad("%s: rotation signature: %v", where, err)
			}
		}
		if strconv.FormatInt(num(r, "date"), 10) != str(e, "date") {
			bad("%s: rotation not dated as the exchange", where)
		}
	}
	if v, ok := ans["revocation"].(obj); ok {
		sig, err := checkB64(str(v, "signature"), 64)
		if err == nil {
			err = verifyStrict(old, []byte(revocationStatement(old, num(v, "date"))), sig)
		}
		if err != nil {
			bad("%s: revocation signature: %v", where, err)
		}
	}
}
