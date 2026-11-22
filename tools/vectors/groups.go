package main

// The groups part (spec/v0/groups.md): events, the state they give, a
// code for a group, and a mail. The state is computed here by the rules of
// groups section 3, written from the specification alone.

import (
	"cmp"
	"crypto/aes"
	"crypto/cipher"
	"crypto/ecdh"
	"crypto/hkdf"
	"crypto/hmac"
	"crypto/hpke"
	"crypto/sha256"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"slices"
	"sort"
	"strings"
	"time"
	"unicode/utf8"
)

// The members of the examples: A the owner (key a), B an admin (key b),
// C a member, N a newcomer, D a server banned later.
var (
	keyGC = newKey(sha([]byte("msfp-spec group member c")))
	keyGN = newKey(sha([]byte("msfp-spec group member n")))
	keyGD = newKey(sha([]byte("msfp-spec group member d")))
	// X25519 keys of the profiles, for the mail.
	xC = mustX25519(sha([]byte("msfp-spec x25519 c")))
	xN = mustX25519(sha([]byte("msfp-spec x25519 n")))
	// The ephemeral key of the example mail (RFC 9180 skE), fixed so that
	// the file is the same at every run.
	xEph = mustX25519(sha([]byte("msfp-spec x25519 ephemeral")))
)

const groupEventLabel = "msfp-group-event-1\n"
const groupMailLabel = "msfp-group-mail-1\n"
const groupDate = 1800086400

func mustX25519(seed []byte) *ecdh.PrivateKey {
	k, err := ecdh.X25519().NewPrivateKey(seed)
	if err != nil {
		panic(err)
	}
	return k
}

// ---- events ----------------------------------------------------------------

type gEvent struct {
	Name      string          `json:"name"`
	Payload   string          `json:"payload"`
	Event     string          `json:"event"`
	Signature string          `json:"signature"`
	ID        string          `json:"id"`
	Valid     bool            `json:"valid"`
	Reason    string          `json:"reason,omitempty"`
	parsed    map[string]any  `json:"-"`
	raw       json.RawMessage `json:"-"`
}

// signEvent makes an event: payload bytes (map keys sorted by Go's
// encoder), its envelope, its signature and ID.
func signEvent(name string, k key, p map[string]any) gEvent {
	b, err := json.Marshal(p)
	if err != nil {
		panic(err)
	}
	return gEvent{Name: name, Payload: string(b), Event: b64.EncodeToString(b),
		Signature: k.sign(groupEventLabel + string(b)), ID: b64.EncodeToString(sha(b))}
}

// gState is a copy's state (groups section 3.5).
type gState struct {
	ID       string            `json:"id"`
	Name     string            `json:"name"`
	Settings map[string]int64  `json:"settings"`
	Owner    string            `json:"owner"`
	Admins   []string          `json:"admins"`
	Members  []string          `json:"members"`
	Banned   []string          `json:"banned"`
	Heir     string            `json:"heir,omitempty"`
	Excluded []string          `json:"excluded"`
	Keys     map[string]string `json:"keys"` // member ID -> current key

	ownerSeq  int64
	lastSeq   map[string]int64            // member -> last kept seq
	payloads  map[string]map[int64]string // member -> seq -> payload, for equivocation
	adminLog  []adminChange
	bans      map[string]string // banned event ID -> target
	joinedAt  map[string]int64
	capRank   map[string]int   // joined event ID -> its rank in its inviter's UTC day
	removedAt map[string]int64 // admin -> lastSeq that counts
	bannedAt  map[string]int64 // banned -> lastSeq that counts
	ever      map[string]bool  // named by a counted founded or joined
}

type adminChange struct {
	ownerSeq int64
	target   string
	on       bool
}

func newState() *gState {
	return &gState{Settings: map[string]int64{"waitDays": 3, "capPerDay": 5}, Keys: map[string]string{},
		lastSeq: map[string]int64{}, payloads: map[string]map[int64]string{}, bans: map[string]string{},
		joinedAt: map[string]int64{}, capRank: map[string]int{}, removedAt: map[string]int64{}, bannedAt: map[string]int64{}, ever: map[string]bool{},
		Admins: []string{}, Members: []string{}, Banned: []string{}, Excluded: []string{}}
}

func (s *gState) adminAsOf(m string, k int64) bool {
	on := false
	for _, c := range s.adminLog {
		if c.target == m && c.ownerSeq <= k {
			on = c.on
		}
	}
	return on
}

func (s *gState) isMember(m string) bool { return slices.Contains(s.Members, m) }

func num64(p map[string]any, k string) (int64, bool) {
	f, ok := p[k].(float64)
	if !ok || f != float64(int64(f)) {
		return 0, false
	}
	return int64(f), true
}

func strOf(p map[string]any, k string) string { s, _ := p[k].(string); return s }

// apply keeps an event, or says why not (groups sections 3.2 to 3.5). now
// is the copy's clock.
// gKept is an event that passed the checks a member makes before keeping
// it (groups section 3.2).
type gKept struct {
	e      gEvent
	p      map[string]any
	b      []byte
	typ    string
	signer string
	member string
	seq    int64
	date   int64
	done   bool
}

// check is what a member checks before keeping an event: a well-formed
// payload, its signature under signer, the group's ID, the date. Whether it
// counts is computeGroup's.
func check(e gEvent, group string, now int64) (*gKept, error) {
	b, err := b64.DecodeString(e.Event)
	if err != nil || len(b) > 4096 || checkIJSON(b) != nil {
		return nil, errors.New("not a payload of at most 4,096 bytes of I-JSON")
	}
	var p map[string]any
	if json.Unmarshal(b, &p) != nil {
		return nil, errors.New("not a JSON object")
	}
	k := &gKept{e: e, p: p, b: b, typ: strOf(p, "type"), signer: strOf(p, "signer"), member: strOf(p, "member")}
	var ok, okd bool
	k.seq, ok = num64(p, "seq")
	k.date, okd = num64(p, "date")
	if k.typ == "" || k.signer == "" || k.member == "" || !ok || k.seq < 1 || !okd {
		return nil, errors.New("a common member is missing")
	}
	sigKey, err := b64.DecodeString(k.signer)
	if err != nil || checkKey(sigKey) != nil {
		return nil, errors.New("signer isn't a valid key")
	}
	sig, err := checkB64(e.Signature, 64)
	if err != nil || verifyStrict(sigKey, []byte(groupEventLabel+string(b)), sig) != nil {
		return nil, errors.New("the signature doesn't verify under signer")
	}
	if k.date > now+300 {
		return nil, errors.New("dated more than 5 minutes in the future")
	}
	if k.typ == "founded" {
		if e.ID != group {
			return nil, errors.New("another group's founded event")
		}
	} else if strOf(p, "group") != group {
		return nil, errors.New("another group's event")
	}
	return k, nil
}

// computeGroup is the state the kept events give (groups section 3.5),
// whatever their order: replayed by lowest ID among the ready ones, again
// from the start while bans and removals of admins cut events that counted.
// It gives each event's verdict: counted, refused, pending or forged.
func computeGroup(kept []*gKept) (gState, map[string]string, map[string]string) {
	slices.SortFunc(kept, func(a, b *gKept) int { return strings.Compare(a.e.ID, b.e.ID) })
	rank := map[string]int{}
	days := map[string][]*gKept{}
	for _, k := range kept {
		if k.typ == "joined" {
			day := fmt.Sprintf("%s %s", k.member, time.Unix(k.date, 0).UTC().Format("2006-01-02"))
			days[day] = append(days[day], k)
		}
	}
	for _, l := range days {
		slices.SortStableFunc(l, func(a, b *gKept) int { return cmp.Compare(a.date, b.date) })
		for i, k := range l {
			rank[k.e.ID] = i
		}
	}
	banCut, removeCut := map[string]int64{}, map[string]int64{}
	var s *gState
	var verdict, reason map[string]string
	for range 8 {
		s = newState()
		s.capRank = rank
		maps.Copy(s.bannedAt, banCut)
		maps.Copy(s.removedAt, removeCut)
		verdict, reason = map[string]string{}, map[string]string{}
		nextBan, nextRemove := map[string]int64{}, map[string]int64{}
		for _, k := range kept {
			k.done = false
		}
		for final := false; ; {
			progressed := false
			for _, k := range kept {
				if k.done {
					continue
				}
				v, why := s.step(k, kept, final)
				if v == "pending" || v == "failing" {
					continue
				}
				k.done, progressed, final = true, true, false
				verdict[k.e.ID], reason[k.e.ID] = v, why
				if v == "counted" {
					ls, _ := num64(k.p, "lastSeq")
					switch on, _ := k.p["on"].(bool); {
					case k.typ == "banned":
						if old, ok := nextBan[strOf(k.p, "target")]; !ok || ls < old {
							nextBan[strOf(k.p, "target")] = ls
						}
					case k.typ == "admin" && !on:
						if old, ok := nextRemove[strOf(k.p, "target")]; !ok || ls < old {
							nextRemove[strOf(k.p, "target")] = ls
						}
					}
				}
				break
			}
			if !progressed {
				if final {
					break
				}
				final = true // none can count: the lowest ready one is refused
			}
		}
		for _, k := range kept {
			if !k.done {
				verdict[k.e.ID], reason[k.e.ID] = "pending", "waiting for events it needs"
			}
		}
		if maps.Equal(nextBan, banCut) && maps.Equal(nextRemove, removeCut) {
			break
		}
		banCut, removeCut = nextBan, nextRemove
	}
	return s.out(), verdict, reason
}

// step replays one event: "pending" when it isn't ready yet.
func (s *gState) step(k *gKept, all []*gKept, final bool) (string, string) {
	p, typ, signer, member, seq, date := k.p, k.typ, k.signer, k.member, k.seq, k.date
	keep := func() {
		s.lastSeq[member] = seq
		if s.payloads[member] == nil {
			s.payloads[member] = map[int64]string{}
		}
		s.payloads[member][seq] = string(k.b)
	}
	refuse := func(why string) (string, string) {
		keep()
		return "refused", why
	}
	// What may hold later: the event waits while others can count, and is
	// refused only when none can.
	fail := func(why string) (string, string) {
		if final {
			return refuse(why)
		}
		return "failing", why
	}
	if typ == "founded" {
		if s.ID != "" {
			return "refused", "a second founded event"
		}
		if seq != 1 || member != signer {
			return "forged", "founded: seq 1, signed by the founder's own key"
		}
		s.ID, s.Owner, s.ownerSeq = k.e.ID, member, 1
		s.Name = cleanName(strOf(p, "name"), 80)
		s.applySettings(p)
		s.Members = append(s.Members, member)
		s.ever[member] = true
		s.Keys[member] = signer
		keep()
		return "counted", ""
	}
	if s.ID == "" || seq > s.lastSeq[member]+1 {
		return "pending", ""
	}
	// These vectors don't rotate keys: a member's key is its member ID.
	if signer != member {
		return "forged", "signer isn't the member's key in force"
	}
	if seq <= s.lastSeq[member] {
		return "refused", "equivocation: another event with this seq"
	}
	if !s.ever[member] {
		return "pending", "" // its joined hasn't counted yet
	}
	for _, o := range all {
		if o != k && o.member == member && o.seq == seq && o.signer == member {
			if !slices.Contains(s.Excluded, member) {
				s.Excluded = append(s.Excluded, member)
				s.Members = slices.DeleteFunc(s.Members, func(x string) bool { return x == member })
				s.Admins = slices.DeleteFunc(s.Admins, func(x string) bool { return x == member })
			}
			return refuse("equivocation: another event with this seq: the signer is excluded")
		}
	}
	if slices.Contains(s.Excluded, member) {
		return refuse("its signer was excluded for equivocation")
	}
	if !s.isMember(member) && typ != "left" {
		return fail("its signer isn't a member")
	}
	if r, ok := s.removedAt[member]; ok && seq > r && (typ == "joined" || typ == "banned") {
		return refuse("signed after the removal of its signer's role")
	}
	if r, ok := s.bannedAt[member]; ok && seq > r {
		return refuse("signed after its signer was banned")
	}
	owner := member == s.Owner
	var oSeq int64
	if owner && typ != "profile" && typ != "rotated" && typ != "revoked" && typ != "left" {
		o, _ := num64(p, "ownerSeq")
		if o > s.ownerSeq+1 {
			return "pending", ""
		}
		if o != s.ownerSeq+1 {
			return refuse("an owner event whose ownerSeq doesn't follow the last one")
		}
		oSeq = o
	}
	asOf, hasAsOf := num64(p, "asOf")
	if !owner && hasAsOf && asOf > s.ownerSeq {
		return "pending", ""
	}
	target := strOf(p, "target")
	// A ban and the removal of an admin wait for their target's events up
	// to their lastSeq.
	if on, _ := p["on"].(bool); typ == "banned" || (typ == "admin" && !on) {
		if ls, ok := num64(p, "lastSeq"); ok && s.lastSeq[target] < ls {
			return "pending", ""
		}
	}
	admin := !owner && hasAsOf && asOf <= s.ownerSeq && s.adminAsOf(member, asOf)
	switch typ {
	case "profile":
	case "joined":
		if !owner && !admin {
			return fail("joined by a member without the right to invite")
		}
		nm := strOf(p, "newMember")
		if slices.Contains(s.Banned, nm) || slices.Contains(s.Excluded, nm) {
			return fail("the newcomer is banned, or excluded")
		}
		if s.isMember(nm) {
			return fail("the newcomer is a member already")
		}
		if int64(s.capRank[k.e.ID]) >= s.Settings["capPerDay"] {
			return fail("over the inviter's daily cap")
		}
		s.Members = append(s.Members, nm)
		s.ever[nm] = true
		s.Keys[nm] = nm
		s.joinedAt[nm] = date
	case "admin":
		if !owner {
			return fail("admin events are the owner's")
		}
		if !s.isMember(target) {
			return fail("the target isn't a member")
		}
		on, _ := p["on"].(bool)
		s.adminLog = append(s.adminLog, adminChange{oSeq, target, on})
		if on {
			s.Admins = append(s.Admins, target)
			delete(s.removedAt, target)
		} else {
			ls, _ := num64(p, "lastSeq")
			s.removedAt[target] = ls
			s.Admins = slices.DeleteFunc(s.Admins, func(x string) bool { return x == target })
		}
	case "heir":
		if !owner {
			return fail("heir events are the owner's")
		}
		s.Heir = target
	case "owner":
		if !owner {
			return fail("owner events are the owner's")
		}
		if !s.isMember(target) {
			return fail("the target isn't a member")
		}
		s.Owner = target
	case "settings":
		if !owner {
			return fail("settings are the owner's")
		}
		s.Name = cleanName(strOf(p, "name"), 80)
		s.applySettings(p)
	case "banned":
		if !owner && !admin {
			return fail("banned by a member without the right to ban")
		}
		if target == s.Owner {
			return fail("the owner can't be banned")
		}
		if !owner && (s.adminAsOf(target, asOf) || slices.Contains(s.Admins, target)) {
			return fail("an admin can't ban an admin: the owner removes its role first")
		}
		if !s.isMember(target) {
			return fail("the target isn't a member")
		}
		if r := strOf(p, "reason"); utf8.RuneCountInString(r) > 280 {
			return fail("a reason over 280 code points")
		}
		ls, _ := num64(p, "lastSeq")
		if old, ok := s.bannedAt[target]; !ok || ls < old {
			s.bannedAt[target] = ls
		}
		s.bans[k.e.ID] = target
		s.Banned = append(s.Banned, target)
		s.Members = slices.DeleteFunc(s.Members, func(x string) bool { return x == target })
		s.Admins = slices.DeleteFunc(s.Admins, func(x string) bool { return x == target })
	case "unbanned":
		if !owner {
			return fail("unbanned is the owner's")
		}
		t, ok := s.bans[strOf(p, "ban")]
		if !ok {
			return fail("no such ban")
		}
		s.Banned = slices.DeleteFunc(s.Banned, func(x string) bool { return x == t })
	case "left":
		if owner {
			return fail("the owner gives the group away before leaving")
		}
		s.Members = slices.DeleteFunc(s.Members, func(x string) bool { return x == member })
		s.Admins = slices.DeleteFunc(s.Admins, func(x string) bool { return x == member })
	default:
		return fail("a type these vectors don't use")
	}
	if oSeq > 0 {
		s.ownerSeq = oSeq
	}
	keep()
	return "counted", ""
}

func (s *gState) applySettings(p map[string]any) {
	st, _ := p["settings"].(map[string]any)
	if v, ok := num64(st, "waitDays"); ok && v >= 0 && v <= 30 {
		s.Settings["waitDays"] = v
	}
	if v, ok := num64(st, "capPerDay"); ok && v >= 1 && v <= 100 {
		s.Settings["capPerDay"] = v
	}
}

func (s *gState) out() gState {
	c := *s
	c.Admins, c.Members, c.Banned, c.Excluded = sorted(s.Admins), sorted(s.Members), sorted(s.Banned), sorted(s.Excluded)
	return c
}

func sorted(l []string) []string {
	c := slices.Clone(l)
	sort.Strings(c)
	if c == nil {
		c = []string{}
	}
	return c
}

// groupEvents is the example list: valid events, then invalid ones, each
// judged on the state the valid ones before it built.
func groupEvents() ([]gEvent, string) {
	a, b, c, n, d := keyA.text(), keyB.text(), keyGC.text(), keyGN.text(), keyGD.text()
	founded := signEvent("A founds Family", keyA, map[string]any{"type": "founded", "signer": a, "member": a, "seq": 1,
		"date": groupDate, "ownerSeq": 1, "name": "Family", "settings": map[string]any{"waitDays": 3, "capPerDay": 5},
		"nonce": b64.EncodeToString(seq(0x10, 16))})
	g := founded.ID
	ev := func(name string, k key, member string, sq int64, extra map[string]any) gEvent {
		p := map[string]any{"group": g, "signer": k.text(), "member": member, "seq": sq, "date": groupDate + 60*sq}
		for kk, v := range extra {
			p[kk] = v
		}
		return signEvent(name, k, p)
	}
	valid := []gEvent{
		founded,
		ev("A invites B", keyA, a, 2, map[string]any{"type": "joined", "newMember": b, "name": "B", "ownerSeq": 2}),
		ev("A makes B an admin", keyA, a, 3, map[string]any{"type": "admin", "target": b, "on": true, "ownerSeq": 3}),
		ev("B's profile", keyB, b, 1, map[string]any{"type": "profile", "name": "B",
			"encryptionKey":  b64.EncodeToString(mustX25519(sha([]byte("msfp-spec x25519 b"))).PublicKey().Bytes()),
			"nextCommitment": commitment(keyB2.pub)}),
		ev("B invites C", keyB, b, 2, map[string]any{"type": "joined", "newMember": c, "name": "C", "asOf": 3}),
		ev("C's profile", keyGC, c, 1, map[string]any{"type": "profile", "name": "C",
			"encryptionKey": b64.EncodeToString(xC.PublicKey().Bytes()), "nextCommitment": commitment(keyGC.pub)}),
		ev("B invites N", keyB, b, 3, map[string]any{"type": "joined", "newMember": n, "name": "N", "asOf": 3}),
		ev("N's profile", keyGN, n, 1, map[string]any{"type": "profile", "name": "N",
			"encryptionKey": b64.EncodeToString(xN.PublicKey().Bytes()), "nextCommitment": commitment(keyGN.pub)}),
		ev("B invites D", keyB, b, 4, map[string]any{"type": "joined", "newMember": d, "name": "D", "asOf": 3}),
		ev("B bans D, with a reason", keyB, b, 5, map[string]any{"type": "banned", "target": d, "lastSeq": 0,
			"reason": "Not who we thought", "asOf": 3}),
		ev("A names C its heir", keyA, a, 4, map[string]any{"type": "heir", "target": c, "ownerSeq": 4}),
		ev("A sets a 7-day wait", keyA, a, 5, map[string]any{"type": "settings", "name": "Family",
			"settings": map[string]any{"waitDays": 7, "capPerDay": 5}, "ownerSeq": 5}),
	}
	bad := []gEvent{
		ev("C, a member, invites", keyGC, c, 2, map[string]any{"type": "joined", "newMember": keyB2.text(), "name": "X", "asOf": 5}),
		ev("B, an admin, bans the owner", keyB, b, 6, map[string]any{"type": "banned", "target": a, "lastSeq": 5, "asOf": 5}),
		ev("B names an admin", keyB, b, 7, map[string]any{"type": "admin", "target": c, "on": true, "asOf": 5}),
		ev("the owner leaves without giving the group away", keyA, a, 6, map[string]any{"type": "left"}),
		ev("an owner event that skips an ownerSeq: pending", keyA, a, 7, map[string]any{"type": "heir", "target": b, "ownerSeq": 7}),
		ev("a seq that skips one: pending", keyGN, n, 3, map[string]any{"type": "profile", "name": "N"}),
		ev("D's profile, signed after its ban's lastSeq: cut, whichever comes first", keyGD, d, 1, map[string]any{"type": "profile", "name": "D"}),
		func() gEvent {
			e := ev("signed by another key than the member's: forged", keyGN, c, 3, map[string]any{"type": "profile", "name": "C"})
			return e
		}(),
		func() gEvent {
			e := ev("a broken signature", keyGC, c, 2, map[string]any{"type": "profile", "name": "C2"})
			s, _ := b64.DecodeString(e.Signature)
			s[0] ^= 1
			e.Signature = b64.EncodeToString(s)
			return e
		}(),
		func() gEvent {
			e := signEvent("another group's event", keyGC, map[string]any{"group": b64.EncodeToString(sha([]byte("x"))),
				"type": "profile", "signer": c, "member": c, "seq": 2, "date": groupDate + 120, "name": "C"})
			return e
		}(),
		ev("dated an hour ahead of the copy's clock", keyGC, c, 2, map[string]any{"type": "profile", "name": "C", "date": groupDate + 4000}),
		ev("N again, with another seq 1: equivocation, both dropped", keyGN, n, 1, map[string]any{"type": "profile", "name": "N, again"}),
	}
	return append(valid, bad...), g
}

// groupNow is the copy's clock when it judges the examples.
const groupNow = groupDate + 600

// judgeGroupEvents judges the events as one set, as every copy that kept
// them all does, whatever their order (groups section 3.5): valid means
// the event counts in the state they give.
func judgeGroupEvents(events []gEvent, group string) ([]gEvent, gState) {
	var kept []*gKept
	why := map[string]string{}
	for _, e := range events {
		k, err := check(e, group, groupNow)
		if err != nil {
			why[e.ID] = "not kept: " + err.Error()
			continue
		}
		kept = append(kept, k)
	}
	state, verdict, reason := computeGroup(kept)
	var out []gEvent
	for _, e := range events {
		e.Valid, e.Reason = verdict[e.ID] == "counted", why[e.ID]
		if v, ok := verdict[e.ID]; ok && v != "counted" {
			e.Reason = v + ": " + reason[e.ID]
		}
		out = append(out, e)
	}
	return out, state
}

// ---- the mail (groups section 5), sealed as RFC 9180 says ------------------

func labeledExtract(suite []byte, salt []byte, label string, ikm []byte) []byte {
	in := append(append(append([]byte("HPKE-v1"), suite...), label...), ikm...)
	prk, err := hkdf.Extract(sha256.New, in, salt)
	if err != nil {
		panic(err)
	}
	return prk
}

func labeledExpand(suite []byte, prk []byte, label string, info []byte, n int) []byte {
	l := binary.BigEndian.AppendUint16(nil, uint16(n))
	in := append(append(append(append(l, "HPKE-v1"...), suite...), label...), info...)
	out, err := hkdf.Expand(sha256.New, prk, string(in), n)
	if err != nil {
		panic(err)
	}
	return out
}

// seal is HPKE's base mode, DHKEM(X25519, HKDF-SHA256), HKDF-SHA256,
// AES-128-GCM, one message, with the ephemeral key given: it returns enc
// and the ciphertext.
func seal(eph *ecdh.PrivateKey, pkR *ecdh.PublicKey, info, aad, pt []byte) ([]byte, []byte) {
	dh, err := eph.ECDH(pkR)
	if err != nil {
		panic(err)
	}
	enc := eph.PublicKey().Bytes()
	kemSuite := binary.BigEndian.AppendUint16([]byte("KEM"), 0x0020)
	kemContext := append(slices.Clone(enc), pkR.Bytes()...)
	shared := labeledExpand(kemSuite, labeledExtract(kemSuite, nil, "eae_prk", dh), "shared_secret", kemContext, 32)
	suite := []byte("HPKE")
	for _, id := range []uint16{0x0020, 0x0001, 0x0001} {
		suite = binary.BigEndian.AppendUint16(suite, id)
	}
	ctx := append([]byte{0}, labeledExtract(suite, nil, "psk_id_hash", nil)...)
	ctx = append(ctx, labeledExtract(suite, nil, "info_hash", info)...)
	secret := labeledExtract(suite, shared, "secret", nil)
	k := labeledExpand(suite, secret, "key", ctx, 16)
	nonce := labeledExpand(suite, secret, "base_nonce", ctx, 12)
	blk, _ := aes.NewCipher(k)
	gcm, _ := cipher.NewGCM(blk)
	return enc, gcm.Seal(nil, nonce, pt, aad)
}

// openMail opens a sealed mail with Go's own crypto/hpke: the cross-check of
// seal above.
func openMail(priv *ecdh.PrivateKey, sealed, info, aad []byte) ([]byte, error) {
	if len(sealed) < 32 {
		return nil, errors.New("shorter than an encapsulated key")
	}
	kem := hpke.DHKEM(ecdh.X25519())
	sk, err := kem.NewPrivateKey(priv.Bytes())
	if err != nil {
		return nil, err
	}
	r, err := hpke.NewRecipient(sealed[:32], sk, hpke.HKDFSHA256(), hpke.AES128GCM(), info)
	if err != nil {
		return nil, err
	}
	return r.Open(aad, sealed[32:])
}

type gMail struct {
	Name            string `json:"name"`
	Payload         string `json:"payload"`
	Mail            string `json:"mail"`
	Signature       string `json:"signature"`
	RecipientX25519 string `json:"recipientX25519Private"`
	Info            string `json:"info"`
	AAD             string `json:"aad"`
	Opened          string `json:"opened"`
}

func mailInfo(group, id []byte) []byte {
	return append(append([]byte("msfp-group-mail-1"), group...), id...)
}

func groupMail(group string) gMail {
	g, _ := b64.DecodeString(group)
	id := seq(0x30, 16)
	from, to := keyGN.pub, keyGC.pub
	opened := `{"address":"https://n.example.org","inbound":true,"nextCommitment":"` + commitment(keyGN.pub) + `"}`
	info := mailInfo(g, id)
	aad := append(slices.Clone(from), to...)
	enc, ct := seal(xEph, xC.PublicKey(), info, aad, []byte(opened))
	p := map[string]any{"group": group, "id": b64.EncodeToString(id), "kind": "request", "from": keyGN.text(),
		"to": keyGC.text(), "expires": groupDate + 30*86400, "sealed": b64.EncodeToString(append(enc, ct...))}
	b, _ := json.Marshal(p)
	return gMail{Name: "N's request to C", Payload: string(b), Mail: b64.EncodeToString(b),
		Signature: keyGN.sign(groupMailLabel + string(b)), RecipientX25519: fmt.Sprintf("%x", xC.Bytes()),
		Info: fmt.Sprintf("%x", info), AAD: fmt.Sprintf("%x", aad), Opened: opened}
}

// ---- the file ----------------------------------------------------------------

type groupsFile struct {
	About  []string          `json:"about"`
	Keys   map[string]string `json:"keys"`
	Now    int64             `json:"now"`
	Events []gEvent          `json:"events"`
	State  gState            `json:"state"`
	Code   v0Code            `json:"code"`
	Group  string            `json:"group"`
	Name   string            `json:"groupName"`
	Mail   gMail             `json:"mail"`
	Join   gJoinProof        `json:"joinProof"`
}

// gJoinProof is group-join's proof (groups section 8.4) of the code above,
// by a server already paired with its maker.
type gJoinProof struct {
	Redeemer string `json:"redeemer"`
	Value    string `json:"value"`
}

// groupJoinLabel is group-join's proof label.
const groupJoinLabel = "msfp-group-join-1\n"

func joinProof(secret, redeemer []byte) string {
	m := hmac.New(sha256.New, secret)
	m.Write([]byte(groupJoinLabel))
	m.Write(redeemer)
	return b64.EncodeToString(m.Sum(nil))
}

func genV0Groups() []byte {
	events, g := groupEvents()
	judged, state := judgeGroupEvents(events, g)
	secret := mustHex(secretS)
	gid, _ := b64.DecodeString(g)
	return encode(groupsFile{
		About: []string{
			"The groups part (spec/v0/groups.md). Generated by tools/vectors; no implementation has produced them yet.",
			"keys: the members' Ed25519 keys by name (a and b are keys.json's; seeds are SHA-256 of an ASCII text, see tools/vectors/groups.go). Member IDs are these keys, base64url.",
			"events: one set, with now as the copy's clock. A copy keeps those that pass section 3.2's checks, and the state is what they give in section 3.5's replay, whatever order they arrived in. valid means the event counts in that state; reason says why another doesn't (not kept, refused, pending or forged). The equivocation drops both of N's profiles and excludes N; D's profile, after its ban's lastSeq, never counts, whichever arrives first. payload is the event's bytes as text; event is their base64url; id is base64url of SHA-256 of the payload.",
			"state: what a copy computes once it kept the valid events and met the invalid ones (groups section 3.5): member IDs, sorted.",
			"code: a pairing code for the group (core section 5.1, groups section 2), made by B, with the group's ID and name.",
			"joinProof: group-join's proof of that code (groups section 8.4) by D, already paired with B: HMAC-SHA256 keyed with the code's secret over msfp-group-join-1 and a line feed, then D's 32 raw public-key bytes.",
			"mail: N's request to C (groups section 5), sealed with HPKE base mode, DHKEM(X25519, HKDF-SHA256), HKDF-SHA256, AES-128-GCM, to C's encryption key; recipientX25519Private opens it. Opened, it holds address, nextCommitment and inbound (certificates optional), no name: a member's name is its profile's. info and aad are hex. The generator seals with a fixed ephemeral key, and opens with Go's crypto/hpke.",
		},
		Keys: map[string]string{"a": keyA.text(), "b": keyB.text(), "c": keyGC.text(), "n": keyGN.text(), "d": keyGD.text()},
		Now:  groupNow, Events: judged, State: state, Group: g, Name: "Family",
		Code: v0Code{"b", "https://b.example.org", secretS, expires,
			makeGroupCode(core0.prefix, keyB.pub, secret, expires, gid, "Family", "https://b.example.org"), b64.EncodeToString(sha(secret))},
		Mail: groupMail(g),
		Join: gJoinProof{"d", joinProof(secret, keyGD.pub)},
	})
}
