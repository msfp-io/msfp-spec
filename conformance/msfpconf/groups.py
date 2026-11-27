"""Checks of the groups part (groups 2, 3, 8): the suite, already paired
with the server, joins a group of the server's through group-join with a
--group-code; the server's operator accepts; the server's list, its
group-sync and group-push, and the suite leaving."""

import hashlib
import hmac
import json
import os
import time

from . import ed25519
from .wire import b64u, dumps, parse_code_group, unb64u

EVENT_LABEL = b"msfp-group-event-1\n"
MAIL_LABEL = b"msfp-group-mail-1\n"
JOIN_LABEL = b"msfp-group-join-1\n"
MAX_PAYLOAD = 4096


class GroupFault(ValueError):
    pass


def join_proof(secret, pub):
    """Groups 8.4: HMAC-SHA256 keyed with the code's secret over the label and the caller's key."""
    return b64u(hmac.new(secret, JOIN_LABEL + pub, hashlib.sha256).digest())


def read_envelope(env, group=None):
    """An event envelope (groups 3.1, 3.2) as (id, payload dict, payload bytes);
    raises GroupFault with why it wouldn't be kept."""
    if not isinstance(env, dict) or not isinstance(env.get("event"), str) or not isinstance(env.get("signature"), str):
        raise GroupFault("an envelope without event and signature strings: %r" % (str(env)[:80],))
    raw = unb64u(env["event"])
    sig = unb64u(env["signature"], 64)
    if raw is None or sig is None:
        raise GroupFault("event or signature isn't base64url (signature of 64 bytes)")
    if len(raw) > MAX_PAYLOAD:
        raise GroupFault("a payload of %d bytes, over 4,096" % len(raw))
    try:
        p = json.loads(raw)
    except ValueError as e:
        raise GroupFault("a payload that isn't JSON: %s" % e)
    if not isinstance(p, dict):
        raise GroupFault("a payload that isn't an object")
    for f, t in (("type", str), ("signer", str), ("member", str), ("seq", int), ("date", int)):
        if not isinstance(p.get(f), t) or isinstance(p.get(f), bool):
            raise GroupFault("a %s payload without %s as %s" % (p.get("type"), f, t.__name__))
    if p["seq"] < 1:
        raise GroupFault("seq %d (from 1)" % p["seq"])
    signer = unb64u(p["signer"], 32)
    if signer is None or unb64u(p["member"], 32) is None:
        raise GroupFault("signer or member isn't base64url of 32 bytes")
    if not ed25519.verify_strict(signer, EVENT_LABEL + raw, sig):
        raise GroupFault("the %s event's signature doesn't verify under its signer (groups 3.1)" % p["type"])
    eid = b64u(hashlib.sha256(raw).digest())
    if p["type"] == "founded":
        if "group" in p:
            raise GroupFault("founded carries group (groups 3.1: absent in founded only)")
        if group is not None and eid != group:
            raise GroupFault("a founded whose ID %s isn't the group's %s" % (eid, group))
    elif group is not None and p.get("group") != group:
        raise GroupFault("a %s event of group %r, not %s" % (p["type"], p.get("group"), group))
    return eid, p, raw


class GroupChecks:
    """Mixed into Suite."""

    def group_on(self):
        return bool(getattr(self.a, "group_code", None))

    def sign_event(self, payload, key=None):
        raw = dumps(payload)
        key = key or self.key
        return {"event": b64u(raw), "signature": b64u(key.sign(EVENT_LABEL + raw))}, b64u(hashlib.sha256(raw).digest())

    def group_call(self, name, body, key=None):
        return self.client.call(name, body, key=key)

    def not_heard(self, a):
        """Groups 8's 404 to a caller not heard for a group: the plain 404 of
        core 6.4, or, since the caller is a paired peer, core 6.6's signed
        not-found? The spec reads both ways; the suite takes either."""
        from .checks import Fail
        if a.status != 404:
            raise Fail("got %s; expected a 404" % a.short())
        if a.signed:
            self.answers.append(a)
            if a.problem_type() != "not-found":
                raise Fail("a signed 404 of type %r, not not-found" % a.problem_type())
        if not getattr(self, "_noted_heard", False):
            self._noted_heard = True
            self.note_unclear("groups 8, core 6.4, 6.6", "a paired peer not heard for a group: groups 8 says the 404 "
                              "of core 6.4 (plain), core 6.6 signs every refusal to a key heard as a peer's. This "
                              "server sends it %s; the suite takes either" % ("signed" if a.signed else "plain"))
        return "%s 404" % ("signed" if a.signed else "plain")

    def server_have(self, gid):
        from .checks import Fail
        a = self.group_call("group-sync", {"group": gid, "have": {}})
        self.signed(a, 200)
        h = a.json.get("have")
        if not isinstance(h, dict):
            raise Fail("group-sync's answer has no have object (groups 8.1)")
        return h, a

    def group_checks(self):
        if not self.group_on():
            self.skip("GR1", "groups 2", "the group checks", "give --group-code (a code of a group the server's operator made)")
            return
        from .checks import Fail, Skip
        c = self.client
        st = {}

        def code():
            key, secret, expiry, address, group, name = parse_code_group(self.a.group_code, self.allow_http)
            if group is None:
                raise Fail("--group-code carries no group (core 5.1 flag): it's a code for a pairing by hand")
            if key != self.code[0]:
                raise Fail("--group-code holds another key than --code: it must be the same server's")
            st.update(secret=secret, group=b64u(group), name=name)
            return "group %s, named %r" % (b64u(group), name)
        if not self.check("GR1", "groups 2; core 5.1", "the group code reads, with the group's ID and name", code):
            return
        gid, secret = st["group"], st["secret"]
        hashed = b64u(hashlib.sha256(secret).digest())

        def not_member_sync():
            return self.not_heard(self.group_call("group-sync", {"group": gid, "have": {}}))
        self.check("GR2", "groups 8; core 6.4, 6.6", "group-sync from a paired server that isn't a member: a 404",
                   not_member_sync)

        def unknown_group():
            return self.not_heard(self.group_call("group-sync", {"group": b64u(os.urandom(32)), "have": {}}))
        self.check("GR3", "groups 8; core 6.4, 6.6", "group-sync about a group the server isn't in: a 404", unknown_group)

        def wrong_proof():
            a = self.group_call("group-join", {"secretHash": hashed, "proof": join_proof(os.urandom(32), self.key.pub)})
            self.signed(a, 404)
            return "signed 404"
        self.check("GR4", "groups 8.4", "group-join with a wrong proof: a signed 404", wrong_proof)

        def unknown_code():
            other = os.urandom(32)
            a = self.group_call("group-join", {"secretHash": b64u(hashlib.sha256(other).digest()),
                                               "proof": join_proof(other, self.key.pub)})
            self.signed(a, 404)
            return "signed 404"
        self.check("GR5", "groups 8.4", "group-join with no live code of that hash: a signed 404", unknown_code)

        def join():
            a = self.group_call("group-join", {"secretHash": hashed, "proof": join_proof(secret, self.key.pub)})
            self.signed(a, 200)
            if a.json != {}:
                raise Fail("the 200 isn't {} (groups 8.4): %r" % a.body[:80])
            return "signed 200 {}"
        if not self.check("GR6", "groups 8.4", "group-join with the code: a signed 200 {}, the suite waiting", join):
            return

        def again():
            # Not through self.signed: A2 would want a problem document,
            # and core 8.2 names no type for this 409 (see the note).
            a = self.group_call("group-join", {"secretHash": hashed, "proof": join_proof(secret, self.key.pub)})
            if a.status == 404 and a.signed:
                raise Skip("a signed 404: the code is spent (it had one use), which groups 8.4 checks first; make "
                           "the group code with 2 uses or more for this check")
            if a.status != 409 or not a.signed:
                raise Fail("got %s; expected a signed 409" % a.short())
            if a.body:
                self.note_unclear("core 8.2, groups 8.4", "the 409s of group-join and group-pair are in no row of core "
                                  "8.2's table: no body, as pair's 409, or a problem document of some type? This server "
                                  "sends %r; the suite takes either" % a.body[:60])
                return "signed 409 with a body (%s)" % a.media_type
            self.note_unclear("core 8.2, groups 8.4", "the 409s of group-join and group-pair are in no row of core 8.2's "
                              "table: no body, as pair's 409, or a problem document of some type? This server sends no "
                              "body; the suite takes either")
            return "signed 409, no body"
        self.check("GR7", "groups 8.4", "group-join again while waiting: a signed 409", again)

        # The operator accepts; the server signs joined and pushes the list.
        t0 = time.time()
        me = self.key.text

        def pushed():
            return [r for r in self.peer.since(t0, call="group-push") if isinstance(r.get("json"), dict)
                    and r["json"].get("group") == gid]

        def has_joined():
            for r in pushed():
                for env in r["json"].get("events") or []:
                    try:
                        _, p, _ = read_envelope(env, gid)
                    except GroupFault:
                        continue
                    if p["type"] == "joined" and p.get("newMember") == me:
                        return True
            return False
        self.ask_operator("join-accept", "accept the group join of 'MSFP conformance suite', fingerprint %s, in the group %r"
                          % (self.key.fingerprint, st["name"]), has_joined, cid="GR8")

        def the_list():
            rs = pushed()
            if not rs:
                raise Fail("no group-push for the group came within %d s of asking the operator (groups 2, 6: the maker "
                           "signs joined and pushes the list)" % self.a.operator_timeout)
            bad = [f for r in rs for f in r["faults"]]
            if bad:
                raise Fail("; ".join(bad[:2]))
            events = {}
            for r in rs:
                for env in r["json"].get("events") or []:
                    try:
                        eid, p, raw = read_envelope(env, gid)
                    except GroupFault as e:
                        raise Fail(str(e))
                    events[eid] = p
            founded = [p for e, p in events.items() if p["type"] == "founded"]
            joined = [p for p in events.values() if p["type"] == "joined" and p.get("newMember") == me]
            if not founded:
                raise Fail("the list pushed has no founded event (the newcomer needs the whole list, groups 6)")
            if not joined:
                raise Fail("the list pushed has no joined naming this suite's key %s (groups 2)" % me)
            j = joined[0]
            if j["signer"] not in [b64u(k) for k in self.server_keys]:
                raise Fail("joined is signed by %s, not the server's key (it accepted the join)" % j["signer"])
            st["events"] = events
            return "%d event(s): %s" % (len(events), ", ".join(sorted({p["type"] for p in events.values()})))
        if not self.check("GR8", "groups 2, 3.1, 3.2, 6", "accepted, the server signs joined for the suite and pushes the "
                          "list: every envelope well formed and well signed, founded's ID the group's", the_list,
                          operator=True):
            return

        def sync():
            h, a = self.server_have(gid)
            j = a.json
            if not isinstance(j.get("events"), list) or not isinstance(j.get("mail", []), list) \
                    or not isinstance(j.get("more"), bool):
                raise Fail("group-sync's answer lacks events (array), mail (array) or more (boolean): %r" % a.body[:120])
            got = {}
            for env in j["events"]:
                try:
                    eid, p, _ = read_envelope(env, gid)
                except GroupFault as e:
                    raise Fail(str(e))
                got[eid] = p
            missing = [e for e in st["events"] if e not in got]
            if missing and not j["more"]:
                raise Fail("group-sync with an empty have lacks %d event(s) it pushed, and more is false" % len(missing))
            for p in got.values():
                if not isinstance(h.get(p["member"]), int) or h[p["member"]] < p["seq"]:
                    raise Fail("have names %r for %s, below the seq %d of an event it sent" % (h.get(p["member"]),
                                                                                              p["member"], p["seq"]))
            # Per member, seq order (groups 8.1).
            seqs = {}
            for env in j["events"]:
                _, p, _ = read_envelope(env, gid)
                if seqs.get(p["member"], 0) > p["seq"]:
                    raise Fail("events of %s out of seq order" % p["member"])
                seqs[p["member"]] = p["seq"]
            st["have"] = h
            st["all"] = got
            return "%d event(s), have %s" % (len(got), json.dumps(h, sort_keys=True))
        if not self.check("GR9", "groups 8.1", "group-sync with an empty have: a signed 200 with every event, in seq "
                          "order per member, and the server's have", sync):
            return

        def sync_full():
            a = self.group_call("group-sync", {"group": gid, "have": st["have"]})
            self.signed(a, 200)
            if a.json.get("events"):
                raise Fail("%d event(s) though the suite's have names everything" % len(a.json["events"]))
            return "no events"
        self.check("GR10", "groups 8.1", "group-sync with the server's own have: no events (only what the caller lacks)",
                   sync_full, "SHOULD")

        now = int(time.time())
        seq = [0]

        def mine(typ, **extra):
            seq[0] += 1
            p = {"group": gid, "type": typ, "signer": me, "member": me, "seq": seq[0], "date": now}
            p.update(extra)
            return p

        def push(*envs, kept=True):
            # Only what is meant to be kept is served on the server's
            # group-sync: a probe meant to be refused, served later, could be
            # kept then (the one dated ahead, once its date is near) and,
            # with its seq taken since, make an equivocation.
            if kept:
                self.peer.group_events.extend(envs)
            a = self.group_call("group-push", {"group": gid, "events": list(envs)})
            self.signed(a, 200)
            if a.json != {}:
                raise Fail("group-push's 200 isn't {} (groups 8.2): %r" % a.body[:80])
            return a

        def kept_seq():
            h, _ = self.server_have(gid)
            return h.get(me, 0)

        def profile():
            env, _ = self.sign_event(mine("profile", name="MSFP conformance suite", encryptionKey=b64u(os.urandom(32)),
                                          nextCommitment=self.next_keys[0].commitment))
            push(env)
            n = kept_seq()
            if n != 1:
                raise Fail("the server's have names %r for the suite after its profile (seq 1) was pushed" % n)
            return "kept: have %s = 1" % me[:8]
        if not self.check("GR11", "groups 3.2, 8.2", "the suite's profile, pushed: a signed 200 {}, and kept", profile):
            return

        def unknown_type():
            env, _ = self.sign_event(mine("x-conformance", note="an unknown type"))
            push(env)
            n = kept_seq()
            if n != 2:
                raise Fail("an unknown type well signed wasn't kept: have %r, expected 2 (groups 3.3: kept and passed on)" % n)
            return "kept"
        self.check("GR12", "groups 3.3", "an event of an unknown type, well signed: kept", unknown_type)

        def refused(label, payload, env=None):
            if env is None:
                env, _ = self.sign_event(payload)
            before = kept_seq()
            push(env, kept=False)
            after = kept_seq()
            seq[0] = before   # the next probe reuses the seq
            if after != before:
                raise Fail("%s was kept: have went from %d to %d" % (label, before, after))
            return "not kept"

        def forged():
            env, _ = self.sign_event(mine("x-conformance"))
            sig = bytearray(unb64u(env["signature"]))
            sig[0] ^= 1
            return refused("a forged signature", None, {"event": env["event"], "signature": b64u(bytes(sig))})
        self.check("GR13", "groups 3.2", "an event whose signature doesn't verify: not kept", forged)

        def future():
            return refused("an event dated 10 minutes ahead", mine("x-conformance", date=now + 600))
        self.check("GR14", "groups 3.2", "an event dated 10 minutes in the future: not kept", future)

        def other_group():
            return refused("an event of another group", mine("x-conformance", group=b64u(os.urandom(32))))
        self.check("GR15", "groups 3.2", "an event of another group, in this group's push: not kept", other_group)

        def too_big():
            return refused("a payload over 4,096 bytes", mine("x-conformance", note="x" * 4200))
        self.check("GR16", "groups 3.1", "an event whose payload is over 4,096 bytes: not kept", too_big)

        self.mail_checks(gid, st, seq, mine, push, kept_seq)

        def leave():
            seq[0] = kept_seq()
            env, _ = self.sign_event(mine("left"))
            push(env)
            deadline = time.time() + 10
            while time.time() < deadline:
                a = self.group_call("group-sync", {"group": gid, "have": {}})
                if a.status == 404 and (not a.signed or a.problem_type() == "not-found"):
                    return "left; group-sync then gets the %s 404" % ("signed" if a.signed else "plain")
                time.sleep(1)
            raise Fail("after the suite's left, group-sync still gets %s: a member that left isn't heard (groups 3.5, 8)"
                       % a.short())
        self.check("GR17", "groups 3.5, 7, 8", "the suite leaves: kept, and group-sync is then refused with a 404", leave)

    # ---- the mail (groups 4, 5, 8.3): a second identity of the suite's asks to pair ----------

    def mail_checks(self, gid, st, seq, mine, push, kept_seq):
        """Named admin by the server's operator, the suite signs joined for a
        second identity of its own, N, that can't be called; N signs its
        profile and sends the server a sealed request; the server's operator
        accepts; the server sends N a sealed answer with its address (N can't
        be called), and N calls group-pair on it."""
        from . import hpke
        from .checks import Fail, Skip
        from .wire import Client, Key, make_uncallable_address, read_pins
        me = self.key.text
        server = None
        for p in st["all"].values():
            if p["type"] == "founded":
                server = p["member"]
        profiles = [p for p in st["all"].values() if p["type"] == "profile" and p["member"] == server]
        if not profiles or unb64u(profiles[-1].get("encryptionKey") or "", 32) is None:
            self.skip("GM1", "groups 4, 5", "the mail checks", "the server's list has no profile with an encryptionKey "
                      "(groups 4 step 5: each member signs one)")
            return
        server_x = unb64u(max(profiles, key=lambda p: p["seq"])["encryptionKey"], 32)
        t0 = time.time()

        def admin_event():
            try:
                a = self.group_call("group-sync", {"group": gid, "have": {}})
            except OSError:
                return None
            if a.status != 200 or not a.signed:
                return None
            for env in a.json.get("events") or []:
                try:
                    _, p, _ = read_envelope(env, gid)
                except GroupFault:
                    continue
                if p["type"] == "admin" and p.get("target") == me and p.get("on") is True:
                    return p
            return None
        self.ask_operator("admin", "name 'MSFP conformance suite' (fingerprint %s) an admin of the group %r"
                          % (self.key.fingerprint, st["name"]), lambda: admin_event() is not None, cid="GM1")
        n = Key()
        nx = os.urandom(32)
        n_next = Key()
        n_address = make_uncallable_address()

        def named_admin():
            p = admin_event()
            if p is None:
                raise Fail("no admin event naming the suite came within %d s" % self.a.operator_timeout)
            if not isinstance(p.get("ownerSeq"), int) or p["member"] != server:
                raise Fail("the admin event isn't an owner event (the owner's, with ownerSeq): %r" % p)
            a = self.group_call("group-sync", {"group": gid, "have": {}})
            self.signed(a, 200)
            owner_seqs = []
            for env in a.json.get("events") or []:
                try:
                    _, x, _ = read_envelope(env, gid)
                except GroupFault:
                    continue        # not kept, as a member wouldn't
                if isinstance(x.get("ownerSeq"), int) and x["member"] == server:
                    owner_seqs.append(x["ownerSeq"])
            st["asOf"] = max(owner_seqs)
            # The suite signs joined for N, N its profile (groups 2, 4 step 5).
            seq[0] = kept_seq()
            j_env, _ = self.sign_event(mine("joined", newMember=n.text, name="MSFP conformance suite, second", asOf=st["asOf"]))
            pr = {"group": gid, "type": "profile", "signer": n.text, "member": n.text, "seq": 1, "date": int(time.time()),
                  "name": "MSFP conformance suite, second", "encryptionKey": b64u(hpke.x25519_public(nx)),
                  "nextCommitment": n_next.commitment}
            p_env, _ = self.sign_event(pr, key=n)
            push(j_env, p_env)
            h, _ = self.server_have(gid)
            if h.get(n.text) != 1:
                raise Fail("after the suite's joined for a second identity and that one's profile, have names %r for it"
                           % h.get(n.text))
            return "admin as of ownerSeq %d; joined for N and N's profile kept" % st["asOf"]
        if not self.check("GM1", "groups 2, 3.3, 3.5", "named admin by the operator, the suite's joined for a second "
                          "identity N, and N's profile: kept", named_admin, operator=True):
            return

        # N's request, sealed to the server's encryptionKey (groups 5).
        rid = os.urandom(16)
        body = {"address": n_address, "nextCommitment": n_next.commitment, "inbound": False}
        info = b"msfp-group-mail-1" + unb64u(gid, 32) + rid
        aad = n.pub + unb64u(server, 32)
        payload = {"group": gid, "id": b64u(rid), "kind": "request", "from": n.text, "to": server,
                   "expires": int(time.time()) + 86400,
                   "sealed": b64u(hpke.seal(server_x, info, aad, dumps(body)))}
        raw = dumps(payload)
        req = {"mail": b64u(raw), "signature": b64u(n.sign(MAIL_LABEL + raw))}

        def request():
            a = self.group_call("group-push", {"group": gid, "events": [], "mail": [req]})
            self.signed(a, 200)
            s2 = self.group_call("group-sync", {"group": gid, "have": {}, "mail": []})
            self.signed(s2, 200)
            ids = []
            for m in s2.json.get("mail") or []:
                try:
                    ids.append(json.loads(unb64u(m["mail"]))["id"])
                except (TypeError, ValueError, KeyError):
                    pass
            if b64u(rid) not in ids:
                raise Fail("the request isn't among the mails group-sync lists: not kept (groups 5: kept and passed on "
                           "until it expires)")
            return "kept: listed by group-sync"
        if not self.check("GM2", "groups 5, 8.1, 8.2", "N's request, sealed to the server's encryptionKey and pushed "
                          "through the suite: kept", request):
            return

        def not_kept(label, mp, signer, flip=False):
            raw3 = dumps(mp)
            sig = bytearray(signer.sign(MAIL_LABEL + raw3))
            if flip:
                sig[0] ^= 1
            env = {"mail": b64u(raw3), "signature": b64u(bytes(sig))}
            self.signed(self.group_call("group-push", {"group": gid, "events": [], "mail": [env]}), 200)
            s3 = self.group_call("group-sync", {"group": gid, "have": {}, "mail": []})
            self.signed(s3, 200)
            for m in s3.json.get("mail") or []:
                try:
                    if json.loads(unb64u(m["mail"]))["id"] == mp["id"]:
                        raise Fail("%s was kept: group-sync lists it" % label)
                except (TypeError, ValueError, KeyError):
                    pass
            return "not kept"

        def other(**over):
            mp = dict(payload, id=b64u(os.urandom(16)))
            mp.update(over)
            return mp

        self.check("GM3", "groups 5", "a mail whose signature doesn't verify: not kept",
                   lambda: not_kept("a mail with a forged signature", other(), n, flip=True))
        self.check("GM4", "groups 5", "a mail already expired: not kept",
                   lambda: not_kept("an expired mail", other(expires=int(time.time()) - 60), n))

        def answer():
            out = []
            for r in self.peer.since(t0, call="group-push"):
                for m in ((r.get("json") or {}).get("mail") or []) if isinstance(r.get("json"), dict) else []:
                    out.append(m)
            try:
                a = self.group_call("group-sync", {"group": gid, "have": {}, "mail": []})
            except OSError:
                a = None
            if a is not None and a.status == 200 and a.signed:
                out += a.json.get("mail") or []
            for m in out:
                try:
                    p = json.loads(unb64u(m["mail"]))
                except (TypeError, ValueError, KeyError):
                    continue
                if p.get("kind") == "answer" and p.get("to") == n.text and p.get("answers") == b64u(rid):
                    return m, p
            return None
        self.ask_operator("request-accept", "accept the request of 'MSFP conformance suite, second' in the group %r"
                          % st["name"], lambda: answer() is not None, cid="GM5")

        def answered():
            got = answer()
            if got is None:
                raise Fail("no answer to N's request came within %d s (groups 4.2: N can't be called, so the server "
                           "answers with its address)" % self.a.operator_timeout)
            m, p = got
            raw2 = unb64u(m["mail"])
            sig = unb64u(m.get("signature") or "", 64)
            if sig is None or not ed25519.verify_strict(self.server_keys[0], MAIL_LABEL + raw2, sig):
                raise Fail("the answer's signature doesn't verify under the server's key (groups 5)")
            if p.get("from") != server or p.get("group") != gid or not isinstance(p.get("expires"), int):
                raise Fail("the answer's from, group or expires is wrong: %r" % p)
            try:
                opened = hpke.open_(nx, b"msfp-group-mail-1" + unb64u(gid, 32) + unb64u(p["id"], 16),
                                    unb64u(server, 32) + n.pub, unb64u(p["sealed"]))
                inside = json.loads(opened)
            except (ValueError, TypeError, KeyError) as e:
                raise Fail("the answer doesn't open with N's key (HPKE base, X25519, HKDF-SHA256, AES-128-GCM; info "
                           "and aad of groups 5): %s" % e)
            if not isinstance(inside.get("address"), str) or unb64u(inside.get("nextCommitment") or "", 32) is None:
                raise Fail("the sealed answer lacks address or nextCommitment: %r" % inside)
            st["answer"] = (p, inside)
            if "name" in inside:
                raise Fail("the sealed answer carries name %r (groups 5, proposal 17: the mail carries no name; a "
                           "member's is its profile's)" % (inside["name"],))
            return "opened: address %s, %d pin(s)" % (inside["address"], len(inside.get("certificates") or []))
        if not self.check("GM5", "groups 4.2, 5", "accepted, the server answers N with a mail signed by it, sealed to "
                          "N's encryptionKey, carrying its address and no name", answered, operator=True) \
                and "answer" not in st:
            return

        def group_pair():
            p, inside = st["answer"]
            c = Client(n, inside["address"], server_keys=self.server_keys[:1], connect=self.a.connect)
            c.pins = read_pins(inside.get("certificates"))
            st["n_client"] = c
            body = {"group": gid, "answers": p["id"], "address": n_address, "nextCommitment": n_next.commitment,
                    "inbound": False}
            st["gp_body"] = body
            a = c.call("group-pair", body)
            self.signed(a, 200)
            if unb64u(a.json.get("nextCommitment") or "", 32) is None:
                raise Fail("the 200 lacks nextCommitment (groups 8.3): %r" % a.body[:120])
            if "name" in a.json:
                raise Fail("the 200 carries name %r (groups 8.3, proposal 17: neither side sends a name; a member's "
                           "is its profile's)" % (a.json["name"],))
            return "paired, nextCommitment and no name"
        if not self.check("GM6", "groups 8.3", "N calls group-pair with the answer: a signed 200 with nextCommitment "
                          "and no name; N and the server paired", group_pair):
            return

        def used():
            a = st["n_client"].call("group-pair", st["gp_body"])
            if a.status != 409 or not a.signed:
                raise Fail("got %s; expected a signed 409 (the answer was used)" % a.short())
            return "signed 409"
        self.check("GM7", "groups 8.3", "group-pair with the same answer again: a signed 409", used)

        def n_hello():
            a = st["n_client"].call("hello", {"state": "paired", "core": [0], "parts": self.suite_parts(),
                                              "address": n_address, "inbound": False,
                                              "knownKey": b64u(self.server_keys[0])})
            self.signed(a, 200)
            if a.json.get("state") != "paired":
                raise Fail("the server answers N's hello with state %r, not paired" % a.json.get("state"))
            e = st["n_client"].call("hello", {"state": "ended"})
            self.signed(e, 200)
            return "paired; then ended by N"
        self.check("GM8", "groups 4.2; core 9.2, 7.4", "N's hello: the server answers paired; N then ends the pairing",
                   n_hello)
