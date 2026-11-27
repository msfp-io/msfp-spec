"""The checks of key rotation and revocation (core 9.3, 9.4, 12), address
changes and refused destinations (core 13, 3.3), servers that can't be
called (core 7.5), the server as a reader of the suite's catalog, rate
limits (core 8.3) and ending (core 7.4)."""

import json
import os
import socket
import time

from .report import PASS
from .wire import (Client, Key, b64u, commitment, dumps, fingerprint, host_of, is_int, make_rotation,
                   make_revocation, make_uncallable_address, read_revocation, read_rotation, split_hostport)


def _x():
    from .checks import Fail, Skip
    return Fail, Skip


class KeyChecks:

    # ---- the server reads this suite's catalog (MSFP Media listing 2, 3) ---------------------------

    def reader_checks(self):
        Fail, Skip = _x()
        s = self.peer
        ok, why = self.can_ask_operator()
        title = "the server reads this suite's libraries and titles, well signed, following next"
        if not ok:
            self.skip("Q1", "MSFP Media listing 2, 3; core 6.4", title, why, operator=True)
            return
        if "catalog" not in self.parts_on():
            self.skip("Q1", "MSFP Media listing 2, 3; core 6.4", title, "catalog is off", operator=True)
            return
        t0 = time.time()

        def done():
            return any(r["call"] == "titles" and b'"after":"movie/tmdb/603"' in r["body"] for r in s.since(t0 - 3600))
        self.ask_operator("want", "on this peer ('MSFP conformance suite'), want its library 'Conformance films' "
                                  "(3 films on 2 pages)", done)
        time.sleep(1)

        def reads():
            lib = s.since(0, call="libraries")
            ts = s.since(0, call="titles")
            if not lib and not ts:
                raise Skip("the server didn't call this suite")
            bad = sorted({f for x in lib + ts for f in x["faults"]})
            if bad:
                raise Fail("\n".join(bad))
            if not ts:
                raise Fail("the server read libraries but asked for no titles")
            return "%d libraries and %d titles calls, all well signed and formed" % (len(lib), len(ts))
        self.check("Q1", "MSFP Media listing 2, 3; core 6.4", "the server's calls to this suite are well signed and well formed", reads,
                   operator=True)

        def bodies():
            bad = []
            ts = s.since(0, call="titles")
            if not ts:
                raise Skip("no titles call")
            for x in ts:
                j = x["json"]
                if not isinstance(j, dict):
                    continue
                if not isinstance(j.get("libraries"), list) or any(not isinstance(i, str) for i in j["libraries"]):
                    bad.append("libraries isn't an array of strings: %r" % x["body"][:100])
                elif set(j["libraries"]) - {"lib_conformance"}:
                    bad.append("asks for libraries this suite doesn't offer: %r" % j["libraries"])
                if "after" in j and (not isinstance(j["after"], str) or len(j["after"].encode()) > 512):
                    bad.append("after isn't a string of at most 512 bytes")
                if "since" in j and not is_int(j["since"]):
                    bad.append("since isn't an integer: %r" % j["since"])
            if bad:
                raise Fail("\n".join(bad))
            return "well formed"
        self.check("Q2", "MSFP Media listing 3", "the server's titles requests: libraries offered, after a string, since an integer", bodies,
                   operator=True)

        def follows():
            if not done():
                raise Fail("the server didn't ask for the page after next = movie/tmdb/603")
            return "followed next (the suite sends next on a short page on purpose: a caller follows any next above its after)"
        self.check("Q3", "MSFP Media listing 3.1", "the server follows next to the following page", follows, operator=True)

        def ahead():
            calls = s.since(0, call="previews")
            sheets = [r for r in s.since(0, call="media") if "/trickplay/" in r["path"]]
            if calls or sheets:
                raise Fail("reading the catalog, with nobody opening a copy: %d previews call(s) and %d sheet fetch(es) "
                           "for this suite's copies (video 5.5: never previews for a whole library, nor in bulk for "
                           "copies nobody opened)" % (len(calls), len(sheets)))
            return "no previews call and no sheet fetched while reading the catalog (its copies carry a previewLayout)"
        self.check("Q4", "video 5.5", "the server fetches no previews ahead: none while it reads the catalog", ahead,
                   operator=True)
        self.viewer_checks()

    # ---- the server as a viewer of this suite's films (MSFP Media playing 2 to 4, 5) -------------

    def viewer_checks(self):
        """With --watch-pause: the operator plays one of this suite's films
        on the server, pauses at once and stops after that many seconds.
        This suite plays the owning server (a two-second clip for each
        film) and ends a playback it hears nothing of for 2 minutes."""
        Fail, Skip = _x()
        s = self.peer
        ids = ("VW1", "VW2", "VW3", "VW4", "VW5")
        sections = ("MSFP Media playing 2 to 4; playback 2 to 4; core 6.4", "MSFP Media playing 3, 5", "MSFP Media playing 3",
                    "MSFP Media playing 4", "MSFP Media playing 2.3, 3")
        titles = ("the server's play, progress and stop for this suite's film are well signed and well formed",
                  "while its viewer pauses, the server keeps the playback: never its idle limit (twice the answer's "
                  "progressSeconds) without progress or media",
                  "the pause is reported at once, with paused true (a progress soon after the play, the viewer pausing "
                  "at once)",
                  "the server sends stop when its viewer stops",
                  "progress at the answer's interval: never more often than every 45 seconds, but when the viewer pauses "
                  "or resumes")
        levels = ("MUST", "SHOULD", "SHOULD", "SHOULD", "SHOULD")

        def skip_all(why):
            for cid, sec, title, level in zip(ids, sections, titles, levels):
                self.skip(cid, sec, title, why, level=level, operator=True)
        if not self.a.watch_pause:
            skip_all("run with --watch-pause SECONDS to have the operator play one of this suite's films and pause it")
            return
        if "playback" not in self.parts_on():
            skip_all("playback isn't on between the server and this suite")
            return
        t0 = time.time()

        def mine():
            with s.lock:
                return [pb for pb in s.playbacks.values() if pb["opened"] >= t0]

        def done():
            # Any stop ends the step: a malformed one too (VW1 and VW4 tell).
            return any(pb["stopped"] is not None and not pb.get("replaced") for pb in mine()) or bool(s.since(t0, call="stop"))
        self.ask_operator("watch", "on the server, play one of this peer's films (Fight Club, The Matrix or Heat), "
                                   "pause it at once, keep it paused, and stop it after %d seconds" % self.a.watch_pause,
                          done, self.a.watch_pause + 120)
        pbs = mine()
        if not pbs:
            skip_all("the server played none of this suite's films")
            return
        pb = next((x for x in pbs if x["stopped"] is not None and not x.get("replaced")), pbs[-1])
        end = pb["stopped"] or time.time()

        def well():
            calls = [r for r in s.since(t0 - 1) if r["call"] in ("play", "progress", "stop")]
            bad = sorted({f for r in calls for f in r["faults"]})
            if bad:
                raise Fail("\n".join(bad))
            n = {c: sum(1 for r in calls if r["call"] == c) for c in ("play", "progress", "stop")}
            return "%(play)d play, %(progress)d progress and %(stop)d stop request(s), well signed and formed" % n + \
                "; " + pb["title"]
        self.check("VW1", sections[0], titles[0], well, operator=True)

        def kept():
            with s.lock:
                heard = sorted([pb["opened"]] + [t for t, _, _ in pb["progress"]] + pb["media"])
            times = [t for t in heard if t <= end] + [end]
            gaps = [(b - a, a) for a, b in zip(times, times[1:])]
            gap, since = max(gaps) if gaps else (0, pb["opened"])
            if gap > s.EXPIRY:
                raise Fail("%d s without progress or media from %d s after the play (a playback ends after %d s, its "
                           "idle limit: MSFP Media playing 3, 5)" % (gap, since - pb["opened"], s.EXPIRY))
            return "%d progress reports and %d media requests over %d s; the longest gap %d s" % (
                len(pb["progress"]), len(pb["media"]), end - pb["opened"], gap)
        self.check("VW2", sections[1], titles[1], kept, level="SHOULD", operator=True)

        def paused():
            first = min([t for t, _, _ in pb["progress"]] or [None]) if pb["progress"] else None
            if first is None or first - pb["opened"] > 60:
                raise Fail("no progress within 60 s of the play, its viewer pausing at once (MSFP Media playing 3: at "
                           "once when the viewer pauses; %d progress reports in all)" % len(pb["progress"]))
            told = [t for t, _, p in pb["progress"] if p]
            if not told or told[0] - pb["opened"] > 60:
                raise Fail("no progress with paused true within 60 s of the play, its viewer pausing at once (MSFP Media "
                           "playing 3: paused true when the viewer has paused; %d progress reports, %d with paused true)"
                           % (len(pb["progress"]), len(told)))
            return "a progress %d s after the play, paused true %d s after it" % (first - pb["opened"], told[0] - pb["opened"])
        self.check("VW3", sections[2], titles[2], paused, level="SHOULD", operator=True)

        def stopped():
            if pb["stopped"] is None:
                raise Fail("no stop within %d s" % (self.a.watch_pause + 120))
            return "stop %d s after the play" % (pb["stopped"] - pb["opened"])
        self.check("VW4", sections[3], titles[3], stopped, level="SHOULD", operator=True)

        def spaced():
            reps = sorted((t, p) for t, _, p in pb["progress"])
            close = [(b[0] - a[0], a[0] - pb["opened"]) for a, b in zip(reps, reps[1:])
                     if b[0] - a[0] < 45 - 2 and a[1] == b[1]]
            if close:
                gap, since = close[0]
                raise Fail("%d progress reports less than 45 s apart, the viewer neither pausing nor resuming between "
                           "them (the first: %d s apart, %d s after the play; MSFP Media playing 3: about every "
                           "progressSeconds, %d here, never more often than every 45 seconds)" % (
                               len(close) + 1, gap, since, s.PROGRESS))
            return "%d progress report(s), none closer than 45 s but at a pause or a resume" % len(reps)
        self.check("VW5", sections[4], titles[4], spaced, level="SHOULD", operator=True)

    # ---- rate limits (core 8.3) ----------------------------------------------------------

    def load_checks(self):
        Fail, Skip = _x()
        if not self.a.load:
            self.skip("R1", "core 8.3", "past a peer's limit: a signed 429 rate-limited (or 503) with retryAfter",
                      "sends a burst of calls: run with --load")
            return
        c = self.client
        body = dumps(self.hello_body("paired"))
        n = self.a.load_count
        self.r.note("signing %d requests for the load check (pure Python: this takes a while)" % n)
        reqs = [c.headers(c.base + "hello", body) for _ in range(n)]

        def flood():
            t0 = time.time()
            res = c.burst("hello", body, reqs)
            took = max(time.time() - t0, 0.001)
            ok = [a for a in res if not isinstance(a, Exception) and a.status == 200]
            limited = [a for a in res if not isinstance(a, Exception) and a.status in (429, 503) and a.signed]
            errors = [a for a in res if isinstance(a, Exception)]
            other = [a for a in res if not isinstance(a, Exception) and a not in ok and a not in limited]
            summary = "%d calls in %.1f s (%.0f a second): %d passed, %d signed 429/503, %d other (%s), %d connection errors" % (
                len(res), took, len(res) / took, len(ok), len(limited), len(other), sorted({a.status for a in other}), len(errors))
            for a in limited:
                j = a.json if isinstance(a.json, dict) else {}
                ra = j.get("retryAfter")
                if ra is not None and not (is_int(ra) and 1 <= ra <= 3600):
                    raise Fail("retryAfter %r isn't an integer from 1 to 3,600; %s" % (ra, summary))
                if (a.status, j.get("type")) not in ((429, "rate-limited"), (503, "server-busy")):
                    raise Fail("a %d of type %r (core 8.2: 429 rate-limited, 503 server-busy); %s" % (a.status, j.get("type"), summary))
                hdr = a.header("Retry-After")
                if ra is not None and hdr is not None and hdr.strip() != str(ra):
                    raise Fail("Retry-After %r differs from the body's retryAfter %r (core 8.3: the same value)" % (hdr, ra))
            if errors and not limited:
                raise Fail("connections dropped instead of answers (core 8.3: never a hang or a dropped connection): " + summary)
            if not limited and not other:
                if len(ok) >= 1000 and len(res) / took >= 300:
                    raise Fail("no limit at all: " + summary)
                raise Skip("no limit reached, which this burst can't tell from no limit: %s. Raise --load-count, or run "
                           "nearer the server" % summary)
            if not limited:
                return (PASS, summary + " (only unsigned refusals: a limit by network address, allowed)")
            self.retry_after = max((a.json.get("retryAfter") or 0) for a in limited)
            return summary
        self.check("R1", "core 8.3", "past a peer's limit: a signed 429 rate-limited (or 503 server-busy) with retryAfter", flood)

        def recovers():
            if not hasattr(self, "retry_after"):
                raise Skip("no signed 429 or 503 in the burst")
            wait = self.retry_after or 5
            time.sleep(min(wait, 60) + 1)
            self.signed(self.hello("paired"), 200)
            return "answered again after %d s" % wait
        self.check("R2", "core 8.3", "after retryAfter, calls pass again", recovers)

    # ---- a fresh pairing as a server that can't be called (core 7.5, 9.1) -------------------

    def uncallable_pairing(self):
        Fail, Skip = _x()
        title = "pair as a server that can't be called (inbound false), accepted, read by its scheduled hellos"
        if not getattr(self, "code2", None):
            self.skip("UP1", "core 7.5, 9.1", title, "needs --code2 (a second code of the same server)")
            return
        k2 = Key()
        addr = make_uncallable_address()
        c2 = Client(k2, self.target, server_keys=[self.code2[0]], connect=self.a.connect, base=self.client.base)
        c2.pins = list(self.client.pins)
        self.r.note("second pairing: this suite, as a server that can't be called at %s, key fingerprint %s"
                    % (addr, fingerprint(k2.pub)))

        def body(state, **over):
            b = dict(self.hello_body(state), address=addr, inbound=False, knownKey=b64u(self.code2[0]))
            b.pop("certificates", None)
            b.update(over)
            return b

        def pair():
            import hashlib
            from .wire import proof
            secret = self.code2[1]
            b = {"address": addr, "name": "MSFP conformance suite (can't be called)",
                 "secretHash": b64u(hashlib.sha256(secret).digest()), "proof": b64u(proof(secret, k2.pub, addr)),
                 "nextCommitment": Key().commitment, "inbound": False}
            a = c2.call("pair", b)
            self.signed(a, 200)
            return "200 signed: the code stayed live after the 409 of P11 (core 9.1)"
        if not self.check("UP1", "core 7.5, 9.1", "pair with inbound false and an address https://msfp-<26>.invalid: 200", pair):
            return
        ok, why = self.can_ask_operator()
        if not ok:
            self.skip("UP2", "core 7.1, 7.5", "accepted, the scheduled hello gets paired", why, operator=True)
            return
        t0 = time.time()
        got = {}

        def accepted():
            a = c2.call("hello", body("waiting"))
            if a.signed and a.status == 200 and isinstance(a.json, dict) and a.json.get("state") == "paired":
                got["a"] = a
                return True
            return False
        self.ask_operator("accept2", "accept the second pairing, of 'MSFP conformance suite (can't be called)', fingerprint %s"
                          % fingerprint(k2.pub), accepted)

        def told():
            if "a" not in got:
                raise Skip("not accepted in time")
            reached = [r for r in self.peer.since(t0) if r.get("key") == self.code2[0] and r["call"] == "hello"
                       and r["json"] and r["json"].get("state") == "paired" and r.get("host") == host_of(addr)]
            if reached:
                raise Fail("a hello signed for %s reached this suite: impossible unless the server connected elsewhere" % addr)
            return "the scheduled hello learnt paired; nothing was sent to the address that can't be called"
        self.check("UP2", "core 7.1, 7.5", "accepted: the server sends no hello, the scheduled hello gets paired", told, operator=True)

        def reads():
            if "a" not in got:
                raise Skip("not accepted")
            self.signed(c2.call("libraries", {}), 200)
            return "libraries: 200 (a server that can't be called makes its own calls)"
        self.check("UP3", "core 7.5", "the server that can't be called reads libraries", reads, operator=True)

        def end():
            a = self.signed(c2.call("hello", {"state": "ended"}), 200)
            if a.json.get("state") != "ended":
                raise Fail("state %r" % a.json.get("state"))
            again = c2.call("hello", body("paired"))
            if again.status == 200 and again.signed:
                raise Fail("still answered after ended")
            return "ended, then deleted (%s)" % again.short()
        self.check("UP4", "core 7.4, 7.5", "the server that can't be called ends the pairing with hello ended", end)

    # ---- the suite's own key: rotation and revocation (core 9.3, 9.4, 12) ---------------------

    def rotate(self, old, new, nc=None, date=None, sign=None, keys=None):
        body = make_rotation(old, new, nc, date)
        return self.client.call("rotate", body, key=sign or old, keys=keys)

    def key_checks(self):
        Fail, Skip = _x()
        c = self.client
        n1, n2, n3, n4 = self.next_keys[0], self.next_keys[1], self.next_keys[2], self.next_keys[3]
        o = self.key

        def hello_under(k):
            return c.call("hello", self.hello_body("paired"), key=k)

        def mismatch():
            r = Key()
            a = self.rotate(o, r, Key().commitment)
            self.signed(a, 409, "rotation-refused")
            self.signed(hello_under(o), 200)
            return "409 rotation-refused; the old key still answered"
        self.check("K1", "core 12.3, 9.3", "rotate to a key that doesn't match the commitment: 409 rotation-refused, the key stays",
                   mismatch)

        def bad_rotates():
            r = Key()
            now = int(time.time())
            good = make_rotation(o, r, Key().commitment)
            st_date = make_rotation(o, r, None, now - 400)
            same = make_rotation(o, o, None)
            cases = [("an empty body", {}), ("no newKey", {k: v for k, v in good.items() if k != "newKey"}),
                     ("no date", {k: v for k, v in good.items() if k != "date"}),
                     ("newSignature by another key", dict(good, newSignature=make_rotation(o, Key(), good["nextCommitment"], good["date"])["newSignature"])),
                     ("oldSignature by another key", dict(good, oldSignature=make_rotation(Key(), r, good["nextCommitment"], good["date"])["oldSignature"])),
                     ("dated 400 s ago", st_date), ("newKey the old key", same),
                     ("newKey of small order", dict(good, newKey="JuiVj8KyJ7BFw_SJ8u-Y8NXfrAXTxjM5sTgCiG1T_AU")),
                     ("nextCommitment not 32 bytes", dict(good, nextCommitment="abc")),
                     ("date a string", dict(good, date=str(good["date"]))), ("nextCommitment null", dict(good, nextCommitment=None))]
            bad = []
            for name, b in cases:
                a = c.call("rotate", dumps(b))
                self.answers.append(a)
                if not (a.status == 400 and a.signed and a.problem_type() == "invalid-request"):
                    bad.append("%s: %s" % (name, a.short()))
            if bad:
                raise Fail("expected a signed 400 invalid-request for:\n" + "\n".join(bad))
            return "%d bad rotations refused with a signed 400" % len(cases)
        self.check("K2", "core 9.3, 12.2, 12.3", "bad rotate bodies (missing members, bad signatures, an old date, the same key) "
                   "get a signed 400", bad_rotates)

        def bad_revokes():
            now = int(time.time())
            cases = [("a signature by another key", make_revocation(Key())),
                     ("dated 400 s ago", make_revocation(o, now - 400)),
                     ("no signature", {"date": now}), ("date a string", dict(make_revocation(o), date=str(now))),
                     ("a signature over another date", dict(make_revocation(o, now - 1), date=now))]
            bad = []
            for name, b in cases:
                a = c.call("revoke", dumps(b))
                self.answers.append(a)
                if not (a.status == 400 and a.signed and a.problem_type() == "invalid-request"):
                    bad.append("%s: %s" % (name, a.short()))
            self.signed(hello_under(o), 200)
            if bad:
                raise Fail("expected a signed 400 invalid-request for:\n" + "\n".join(bad))
            return "%d bad revocations refused with a signed 400; the key still answers" % len(cases)
        self.check("K3", "core 9.4, 12.6", "bad revoke bodies get a signed 400, and suspend nothing", bad_revokes)

        def revoke():
            a = self.signed(c.call("revoke", make_revocation(o)), 200)
            if a.json != {"state": "suspended"}:
                raise Fail("revoke answered %r; core 9.4: {\"state\":\"suspended\"}" % a.body[:80])
            return "200 {\"state\":\"suspended\"}"
        t_rev = time.time()
        if not self.check("K4", "core 9.4, 12.6", "revoke the suite's key: 200 suspended", revoke):
            return

        def suspended():
            bad = []
            for name, call, body in (("hello", "hello", self.hello_body("paired")), ("libraries", "libraries", {}),
                                     ("titles", "titles", {"libraries": []}), ("play", "play", {"title": "movie/tmdb/949"})):
                a = c.call(call, body)
                self.answers.append(a)
                if not (a.status == 404 and a.signed):
                    bad.append("%s: %s (expected the signed 404 of a forgotten key)" % (name, a.short()))
            a = self.signed(c.call("revoke", make_revocation(o)), 200)
            if a.json != {"state": "suspended"}:
                bad.append("revoke again: %r" % a.body[:80])
            if bad:
                raise Fail("\n".join(bad))
            calls = [r for r in self.peer.since(t_rev + 2) if r["method"] == "POST"]
            return "hello, libraries, titles, play: signed 404; revoke again: 200 suspended%s" % (
                "; note: the server called this suite %d time(s) while it was suspended (core 12.6: its calls stop)" % len(calls)
                if calls else "")
        self.check("K5", "core 12.6, 6.4", "while suspended: only rotate and revoke are answered; others get the signed 404",
                   suspended)

        def suspended_mismatch():
            self.signed(self.rotate(o, Key(), Key().commitment), 409, "rotation-refused")
            return "409 rotation-refused"
        self.check("K6", "core 12.3", "while suspended, a rotation to a key that doesn't match: 409 rotation-refused",
                   suspended_mismatch)

        def lift():
            a = self.signed(self.rotate(o, n1, n2.commitment), 200)
            if a.json != {"state": "rotated"}:
                raise Fail("answered %r; core 9.3: {\"state\":\"rotated\"}" % a.body[:80])
            self.set_key(n1)
            self.signed(hello_under(n1), 200)
            self.signed(c.call("libraries", {}, key=n1), 200)
            return "200 rotated; the new key's hello and libraries answered: the suspension is lifted"
        if not self.check("K7", "core 12.3, 12.6", "a rotation to the committed key lifts the suspension and rotates", lift):
            return

        def dash():
            a = self.signed(self.rotate(n1, n2, None), 200)
            if a.json != {"state": "rotated"}:
                raise Fail("answered %r" % a.body[:80])
            self.set_key(n2)
            self.signed(hello_under(n2), 200)
            return "200 rotated (the statement's commitment is '-')"
        if not self.check("K8", "core 12.2, 12.3", "a rotation to the committed key, with no commitment after it: 200 rotated", dash):
            return

        def previous():
            self.signed(hello_under(n1), 200)
            a = self.signed(self.rotate(n1, n2, None), 200)
            if a.json != {"state": "rotated"}:
                raise Fail("the repeated rotate answered %r; core 12.3: 200 rotated again, nothing changes" % a.body[:80])
            return "the previous key still heard; the same rotate again: 200 rotated"
        self.check("K9", "core 6.5, 12.3, 12.4", "the previous key is heard for 10 minutes; a repeated rotate is 200 rotated", previous)

        def pending():
            a = self.signed(self.rotate(n2, n3, n4.commitment), 202)
            if a.json != {"state": "pending"}:
                raise Fail("answered %r; core 9.3: {\"state\":\"pending\"}" % a.body[:80])
            a = self.signed(self.rotate(n2, n3, n4.commitment), 202)
            self.signed(hello_under(n2), 200)
            b = hello_under(n3)
            if b.status == 200:
                raise Fail("the pending key is already heard")
            return "202 pending twice; the current key still answers, the pending one gets %s" % b.short()
        if not self.check("K10", "core 12.3 (case 4)", "with no commitment, a rotation is 202 pending until the operator confirms",
                          pending):
            return

        def confirm():
            ok, why = self.can_ask_operator()
            if not ok:
                raise Skip(why + "; the rotation stays pending, the suite keeps its key")
            got = {}

            def done():
                a = self.rotate(n2, n3, n4.commitment)
                if a.signed and a.status == 200:
                    got["a"] = a
                    return True
                return False
            self.ask_operator("confirm", "confirm the pending key rotation of peer 'MSFP conformance suite' to the key with "
                              "fingerprint %s" % fingerprint(n3.pub), done)
            if "a" not in got:
                raise Skip("not confirmed in time; the suite keeps its key")
            self.set_key(n3)
            self.signed(hello_under(n3), 200)
            return "confirmed: the next rotate is 200 rotated, and the new key answers"
        self.check("K11", "core 12.3 (case 4)", "once the operator confirms, the next rotate is 200 rotated", confirm, operator=True)

    # ---- address changes and refused destinations (core 13, 3.3) -------------------------------

    def announce(self, addr):
        self.address = addr
        self.peer.known_hosts.add(host_of(addr))
        return self.hello("paired")

    def address_checks(self):
        Fail, Skip = _x()
        a1 = self.address
        a2 = self.new_address()
        a3 = derive_unserved(a1)
        listen2 = self.a.new_listen or derive_listen(self.a.listen, 1)
        try:
            self.peer.listen("new", a2, listen2, self.cert.context() if self.cert else None)
        except OSError as e:
            self.skip("AD1", "core 13.2", "the server confirms a new address by a hello there before using it",
                      "can't listen on %s for %s: %s (see --new-address, --new-listen)" % (listen2, a2, e))
            return
        self.peer.hosts.add(host_of(a2))
        hold = socket.socket(socket.AF_INET6 if ":" in split_hostport(derive_listen(self.a.listen, 2), 0)[0] else socket.AF_INET)
        try:
            hold.bind(split_hostport(derive_listen(self.a.listen, 2), 0))
        except OSError:
            pass
        self.r.note("address changes: this suite moves to %s (served on %s) and back, then names %s, which nothing serves"
                    % (a2, listen2, a3))

        def confirm_at(label, addr, t, timeout=30):
            return self.peer.wait_for(lambda r: r["t"] >= t and r["listener"] == label and r["method"] == "POST", timeout)

        def move():
            t = time.time()
            self.signed(self.announce(a2), 200)
            got = confirm_at("new", a2, t)
            if not got:
                raise Fail("no call reached %s within 30 s (core 13.2 step 2: a hello there, signed for its host)" % a2)
            first = got[0]
            if first["call"] != "hello":
                raise Fail("the first call at the new address was %s, not the confirming hello" % first["call"])
            if first.get("host") != host_of(a2):
                raise Fail("the confirming hello was signed for %r, not %s" % (first.get("host"), host_of(a2)))
            if first["faults"]:
                raise Fail("; ".join(first["faults"]))
            disc = [r for r in self.peer.since(t, "new") if r["call"] == "discovery"]
            return "a hello at %s, signed for its host%s" % (a2, ", after GET /.well-known/msfp" if disc else
                                                            " (no discovery document fetched first: core 13.2 step 2 says it is)")
        moved = self.check("AD1", "core 13.2", "a new address the suite serves: the server confirms it by a hello there", move)

        def back():
            if not moved:
                raise Skip("the move to %s wasn't confirmed" % a2)
            t = time.time()
            self.signed(self.announce(a1), 200)
            got = confirm_at("main", a1, t)
            if not got or got[0]["call"] != "hello" or got[0].get("host") != host_of(a1):
                raise Fail("no confirming hello at %s within 30 s: either %s was never stored, or the move back isn't checked" % (a1, a2))
            return "a hello at %s: %s had been stored, and the move back is confirmed too" % (a1, a2)
        self.check("AD2", "core 13.2", "moving back: confirmed again (which shows the first move was stored)", back)

        def unserved():
            t = time.time()
            self.signed(self.announce(a3), 200)
            time.sleep(10)
            self.address = a1
            t2 = time.time()
            self.signed(self.hello("paired"), 200)
            got = confirm_at("main", a1, t2, 15)
            if got:
                raise Fail("after naming %s (unserved) and then %s again, the server confirmed %s: it had moved to %s "
                           "without a confirming hello (core 13.2 steps 3-4)" % (a3, a1, a1, a3))
            return "naming %s again started no confirmation: the server kept it while %s failed" % (a1, a3)
        self.check("AD3", "core 13.2", "an address nothing serves: the old address stays", unserved)

        def loopback():
            h, port = split_hostport(host_of(a1), 443)
            if h in ("127.0.0.1", "::1", "localhost"):
                raise Skip("this suite's own address is loopback")
            scheme = a1.split("://")[0]
            lp = split_hostport(self.a.listen, 8000)[1]
            hosts = ["%s://127.0.0.1:%d" % (scheme, lp), "%s://[::1]:%d" % (scheme, lp), "%s://localhost:%d" % (scheme, lp)]
            t = time.time()
            for x in hosts:
                self.signed(self.announce(x), 200)
                time.sleep(4)
            self.address = a1
            self.signed(self.hello("paired"), 200)
            time.sleep(4)
            reached = [r for r in self.peer.since(t) if r.get("host") in {host_of(x) for x in hosts}]
            if reached:
                raise Fail("the server connected to %s for this peer (core 3.3: loopback is refused by default; an "
                           "operator may allow it)" % reached[0]["host"])
            return (PASS, "no call reached a loopback address (this can only fail when the suite runs on the server's own "
                          "machine or network namespace)")
        self.check("X1", "core 3.3", "addresses that point at loopback (127.0.0.1, [::1], localhost) aren't connected to",
                   loopback)
        self.note_unclear("core 3.3", "refused destinations can be seen from outside only when the suite shares the server's "
                          "machine (loopback); link-local, metadata, multicast and the NAT64 and 6to4 forms can't be")
        # Core 13.2 step 3: that hello also drops the pending move's
        # retries; an hour apart, they can't be watched from here.
        self.address = a1

    # ---- the server's own key, pushed (core 9.3, 12.4) ----------------------------------------

    def server_key_checks(self):
        Fail, Skip = _x()
        ok, why = self.can_ask_operator()
        title = "the server's operator rotates its key: a valid rotate, signed with the old key, matching its commitment"
        if not ok:
            self.skip("O1", "core 9.3, 12.3, 12.4", title, why, operator=True)
            return
        old = self.server_keys[0]
        t0 = time.time()

        def done():
            ev = [e for e in self.peer.events if e[0] >= t0]
            if any(e[1] == "rotated" for e in ev):
                return True
            if any(e[1] in ("rotate-refused", "rotate-bad") for e in ev):
                return True
            self.hello("paired")
            return False
        self.ask_operator("rotate", "rotate this server's own key (to its next key): it sends rotate to this peer", done)

        def rotated():
            ev = [e for e in self.peer.events if e[0] >= t0]
            if not ev:
                raise Skip("no rotate reached this suite")
            bad = [e for e in ev if e[1] == "rotate-bad"]
            if bad:
                raise Fail("its rotate would be refused with 400: %s" % bad[0][2]["why"])
            ref = [e for e in ev if e[1] == "rotate-refused"]
            if ref:
                raise Fail("its new key %s %s (core 12.3 case 3: the receiver refuses it and keeps the old key)"
                           % (b64u(ref[0][2]["new"]), ref[0][2]["why"]))
            r = [e for e in ev if e[1] == "rotated"][0][2]
            if not r["matched"]:
                return (PASS, "rotated after a 202 pending (the pairing had no commitment from the server)")
            return "rotated to %s, which matched the commitment from pair; next commitment %s" % (b64u(r["new"]), r["commitment"])
        if not self.check("O1", "core 9.3, 12.3, 12.4", title, rotated, operator=True):
            return

        def signs_new():
            new = self.server_keys[0]
            a = self.hello("paired")
            late = ""
            if not a.signed:
                # Core 12.4's window: a request sent right after answering
                # rotated may reach the server before it took the answer in,
                # and get one signed with the old key. Asked again a moment
                # later, it must sign with the new key only.
                time.sleep(2)
                a = self.hello("paired")
                if a.signed:
                    late = "; the first answer, right after this suite answered rotated, wasn't under the new key yet"
                    self.note_unclear("core 12.4", "a request the receiver sends right after answering rotated can "
                                      "reach the rotating server before it takes that answer in: signed with the old "
                                      "key, the answer fails the receiver's check (against the new key only). A "
                                      "window of one round trip; the suite asks again 2 s later")
            if not a.signed:
                b = self.client.call("hello", self.hello_body("paired"), keys=[old])
                if b.signed:
                    raise Fail("after this suite answered rotated, the server still signs with its old key (core 12.4: "
                               "toward a peer that answered rotated, it signs with the new key only)")
                raise Fail("the hello answer verifies under neither key: %s (%s; under the old key: %s)"
                           % (a.why_unsigned, a.short(), b.short()))
            if a.signer != new:
                raise Fail("signed with another key")
            reqs = [r for r in self.peer.since(t0) if r.get("key") == old and r["t"] > self.peer.events[-1][0] + 1]
            return "its answers verify under the new key %s%s%s" % (b64u(new), "; but %d request(s) were still signed with the "
                                                                   "old key afterwards" % len(reqs) if reqs else "", late)
        self.check("O2", "core 12.4", "after the rotation, the server signs toward this suite with its new key only", signs_new,
                   operator=True)

    # ---- a server that can't be called (core 7.5, 5.2, 12.4, 12.6, 13.2) ---------------------------

    def uncallable_checks(self):
        Fail, Skip = _x()
        c = self.client
        a1 = self.address
        x = make_uncallable_address()

        def refused(cases):
            bad = []
            for name, b in cases:
                a = c.call("hello", dumps(b))
                self.answers.append(a)
                if not (a.status == 404 and a.signed):
                    bad.append("%s: %s" % (name, a.short()))
            if bad:
                raise Fail("expected a signed 404 (a bad body) for:\n" + "\n".join(bad))
            return "%d refused with a signed 404" % len(cases)

        def callable_form():
            base = self.hello_body("paired")
            nocert = {k: v for k, v in base.items() if k != "certificates"}
            return refused([("inbound false with this suite's real address", dict(nocert, inbound=False)),
                            ("inbound false and no address (the stored one is real)",
                             {k: v for k, v in nocert.items() if k != "address"} | {"inbound": False})])
        self.check("U1", "core 5.2, 7.5, 9.2", "hello with inbound false and an address not of the form msfp-<26>.invalid: "
                   "a signed 404", callable_form)

        def bad_forms():
            vec = os.path.join(os.path.dirname(__file__), "..", "..", "vectors", "v0", "strict.json")
            forms = [v.get("text", "") for v in json.load(open(vec))["uncallableAddresses"] if not v["valid"]]
            base = {k: v for k, v in self.hello_body("paired").items() if k != "certificates"}
            return refused([("inbound false with %s" % f, dict(base, address=f, inbound=False)) for f in forms] +
                           [("inbound \"false\"", dict(base, address=x, inbound="false")),
                            ("inbound 0", dict(base, address=x, inbound=0))])
        self.check("U2", "core 5.2, 7.5", "inbound false with the invalid forms of vectors/v0/strict.json, or inbound not a "
                   "boolean: a signed 404", bad_forms)

        def switch():
            t = time.time()
            self.inbound = False
            self.uncallable_address = x
            self.address = x
            a = self.signed(self.hello("paired"), 200)
            if a.json.get("state") != "paired":
                raise Fail("state %r" % a.json.get("state"))
            self.t_uncallable = t
            return "200 paired: the server stores %s at once (core 13.2: no confirming call)" % x
        if not self.check("U3", "core 7.5, 13.2", "hello with inbound false and https://msfp-<26>.invalid: 200, stored at once", switch):
            self.inbound, self.address = True, a1
            return

        def one_way():
            base = self.hello_body("paired")
            out = refused([("knownKey not a string", dict(base, knownKey=7)), ("rotation not a string", dict(base, rotation=5))])
            for name, b in (("rotation of another value", dict(base, rotation="conformance")),
                            ("knownKey of neither of the server's keys", dict(base, knownKey=Key().text))):
                self.signed(c.call("hello", dumps(b)), 200)
            self.signed(self.hello("paired"), 200)
            return out + "; a rotation string of another value and a stranger's knownKey are ignored (200)"
        self.check("U4", "core 9.2, 12.4", "from a server that can't be called, knownKey and rotation are type-checked, "
                   "other values ignored", one_way)

        def still_calls():
            self.signed(c.call("libraries", {}), 200)
            if self.libraries:
                self.signed(c.call("titles", {"libraries": [self.libraries[0]["id"]]}), 200)
            return "libraries and titles answered"
        self.check("U5", "core 7.5", "a server that can't be called still makes its own calls", still_calls)

        time.sleep(max(0, self.a.uncallable_watch - (time.time() - self.t_uncallable)))

        def never_called():
            reqs = self.peer.since(self.t_uncallable + 1)
            tls = [e for e in self.peer.tls_errors if e[0] > self.t_uncallable + 1]
            if reqs or tls:
                raise Fail("%d request(s) and %d TLS connection(s) reached this suite after it said it can't be called: %s"
                           % (len(reqs), len(tls), (reqs[0]["call"] if reqs else tls[0][2])))
            return "nothing reached any of the suite's addresses in %d s" % (time.time() - self.t_uncallable)
        self.check("U6", "core 7.5", "toward a peer that can't be called, the server opens no connection", never_called)

        def back():
            t = time.time()
            self.inbound = True
            self.address = a1
            self.signed(self.hello("paired"), 200)
            got = self.peer.wait_for(lambda r: r["t"] >= t and r["method"] == "POST", 30)
            if not got:
                raise Fail("no call reached %s within 30 s (core 7.5: a hello there confirms the address first)" % a1)
            if got[0]["call"] != "hello":
                raise Fail("the first call was %s, not the confirming hello" % got[0]["call"])
            if got[0]["faults"]:
                raise Fail("; ".join(got[0]["faults"]))
            return "a hello at %s confirmed it before any other call" % a1
        self.check("U7", "core 7.5, 13.2", "inbound back to true: the server confirms the address by a hello before calling",
                   back)

        ok, why = self.can_ask_operator()
        if not ok:
            for cid, t in (("O3", "a pulled rotation"), ("O4", "a pulled revocation")):
                self.skip(cid, "core 12.4, 12.6, 7.5", "the server's operator rotates (or revokes) its key: %s reaches a "
                          "server that can't be called" % t, why, operator=True)
            return
        self.inbound, self.address = False, x
        self.signed(self.hello("paired"), 200)
        self.pulled_checks()

    def pulled_checks(self):
        Fail, Skip = _x()
        c = self.client
        got = {}

        def poll(key):
            def done():
                a = self.hello("paired")
                if a.signed and isinstance(a.json, dict) and (isinstance(a.json.get(key), dict)):
                    got[key] = a
                    return True
                time.sleep(3)
                return False
            return done

        self.ask_operator("rotate2", "rotate this server's own key again: this suite now can't be called, so the rotation "
                          "travels in hello answers", poll("rotation"))

        def pulled_rotation():
            if "rotation" not in got:
                raise Skip("no hello answer carried a rotation in time")
            a = got["rotation"]
            old = a.signer
            new, nc = read_rotation(old, a.json["rotation"])
            want = self.peer.server_commitment
            if want is not None and commitment(new) != want:
                b = self.client.call("hello", dumps(self.hello_body("paired", rotation="refused")))
                raise Fail("the new key %s doesn't match the commitment %s (core 12.3 case 3); this suite answered "
                           "rotation refused (%s)" % (b64u(new), want, b.short()))
            d1 = a.json["rotation"]["date"]
            time.sleep(1.1)
            p = self.client.call("hello", dumps(self.hello_body("paired", rotation="pending")))
            if not p.signed or p.signer != old:
                raise Fail("with knownKey still the old key, the answer isn't signed with it: %s" % p.why_unsigned)
            if not isinstance(p.json.get("rotation"), dict) or p.json["rotation"].get("date", 0) < d1:
                raise Fail("the next answer to a peer still on the old key doesn't carry a newly dated rotation (core 12.4 step 1)")
            self.peer.server_prev = (old, time.time() + 600)
            self.server_keys[0] = new
            self.peer.server_commitment = nc
            n = self.hello("paired")
            if not n.signed or n.signer != new:
                raise Fail("after knownKey names the new key, the answer isn't signed with it (core 12.4 step 4): %s" % n.why_unsigned)
            if "rotation" in n.json:
                raise Fail("the answer to a peer on the new key still carries a rotation")
            return "a valid rotation to %s%s; 'pending' kept the old key's signature and a newly dated statement; knownKey " \
                   "the new key: answers signed with it" % (b64u(new), " (it matched the commitment)" if want else "")
        self.check("O3", "core 12.4, 12.3, 7.5", "a rotation pulled by a server that can't be called: in hello answers, "
                   "valid, followed by knownKey", pulled_rotation, operator=True)

        self.ask_operator("revoke", "revoke this server's key (and, if it offers it, rotate to its next key in the same "
                          "action: core 12.7 case 1)", poll("revocation"))

        def pulled_revocation():
            if "revocation" not in got:
                raise Skip("no hello answer carried a revocation in time")
            a = got["revocation"]
            k = a.signer
            read_revocation(k, a.json["revocation"])
            out = "a valid revocation of %s, in an answer signed with it: this suite suspends the server" % b64u(k)
            rot = a.json.get("rotation")
            if isinstance(rot, dict):
                new, nc = read_rotation(k, rot)
                if self.peer.server_commitment and commitment(new) != self.peer.server_commitment:
                    raise Fail("the rotation that came with the revocation doesn't match the commitment")
                self.peer.server_prev = (k, time.time() + 600)
                self.server_keys[0] = new
                self.peer.server_commitment = nc
                n = self.hello("paired")
                if not n.signed or n.signer != new:
                    raise Fail("after the revocation and rotation, knownKey the new key: the answer isn't signed with it")
                if "revocation" in n.json:
                    raise Fail("an answer signed with the new key carries the revocation (core 12.6: only answers signed "
                               "with the revoked key carry it)")
                out += "; with it a rotation to %s, which lifts the suspension (core 12.7 case 1)" % b64u(new)
            else:
                self.peer.suspended = True
            return out
        self.check("O4", "core 12.6, 12.7, 7.5", "a revocation pulled by a server that can't be called: in hello answers "
                   "signed with the revoked key, valid", pulled_revocation, operator=True)

    # ---- ending (core 7.4) ---------------------------------------------------------------------

    def end_checks(self):
        Fail, Skip = _x()
        if not self.a.end:
            self.skip("E1", "core 7.4, 9.2", "hello ended: 200 ended; then the server has forgotten this suite",
                      "ends the pairing: run with --end (otherwise remove the peer on the server afterwards)")
            return
        c = self.client

        def end():
            a = self.signed(c.call("hello", {"state": "ended"}), 200)
            if a.json.get("state") != "ended":
                raise Fail("state %r; expected ended" % a.json.get("state"))
            extra = sorted(set(a.json) - {"state"})
            if extra:
                return (PASS, "ended (it also sent %s; core 9.2: it leaves everything else out)" % extra)
            return "ended, nothing else"
        if not self.check("E1", "core 7.4, 9.2", "hello ended: 200 signed, state ended", end):
            return

        def gone():
            a = c.call("hello", self.hello_body("waiting"))
            if a.status == 200:
                raise Fail("the server still answers this suite: %s" % a.short())
            return self.unsigned_404(a)
        self.check("E2", "core 7.4, 6.4", "after ended, the server has deleted this suite: the unsigned 404", gone)


def derive_listen(listen, delta):
    h, port = split_hostport(listen, 8000)
    return "%s:%d" % ("[%s]" % h if ":" in h else h, port + delta)


def derive_unserved(address):
    from .checks import derive_address
    return derive_address(address, 2)
