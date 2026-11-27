"""The checks of core 0 (spec/v0/core.md): the code, discovery, strangers,
pairing, waiting, hello and versions, JSON, signatures, nonces, answers.
The parts' checks are in parts.py; keys, addresses, servers that can't
be called and the reader checks in keys.py. Each check names the file
and section it tests, and whether that section says must or should."""

import hashlib
import os
import select
import sys
import time

from . import clip
from .groups import GroupChecks
from .keys import KeyChecks
from .parts import PartChecks
from .peer import PeerServer
from .report import FAIL, PASS, SKIP, Result
from .tls import Certificate, FileCertificate
from .wire import (VERSION, H_DATE, H_KEY, H_NONCE, H_SIG, CertificateRefused, Client, Key, b64u, check_address,
                   check_base, dumps, fingerprint, has_null, host_of, is_int, parse_code, parse_ijson, proof,
                   read_pins, request_message, split_hostport, unb64u, NotIJSON)

PARTS = ("catalog", "playback", "media", "video", "music")
NEEDS = {"catalog": (), "media": (), "playback": ("catalog", "media"),
         "video": ("catalog", "playback", "media"), "music": ("catalog", "playback", "media")}

# Core 2.1's forged signature: the identity as key, R the identity, S 0.
IDENTITY_KEY = "AQAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA"
IDENTITY_SIG = "AQAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA"
SMALL_ORDER_KEY = "JuiVj8KyJ7BFw_SJ8u-Y8NXfrAXTxjM5sTgCiG1T_AU"
MIXED_ORDER_KEY = "sVS9YhiK4oPdGH_v6MMfXidu3wg7TP8RydXP-FbHREA"
NON_CANONICAL_KEY = "7v_______________________________________38"


class Skip(Exception):
    pass


class Fail(Exception):
    pass


def derive_address(address, delta):
    """The same host, another port: for the address the suite moves to."""
    scheme, hp = address.split("://", 1)
    h, port = split_hostport(hp, 443 if scheme == "https" else 80)
    new = port + delta if port not in (443, 80) else 8000 + delta
    host = "[%s]" % h if ":" in h else h
    return "%s://%s:%d" % (scheme, host, new)


class Suite(PartChecks, KeyChecks, GroupChecks):
    def __init__(self, args, report, operator_hook=None):
        self.a = args
        self.r = report
        self.allow_http = args.plain_http
        self.operator_hook = operator_hook      # the self-test plays the operator through this
        self.code = None
        self.target = args.target
        self.key = Key.load_or_make(args.key) if args.key else Key()
        self.next_keys = [Key() for _ in range(5)]   # the suite's next keys, in order (core 12.5)
        self.old_keys = []
        self.server_keys = []                   # the server's current key, first (shared with the client)
        self.server_versions = ({}, {})         # core, parts of the server's latest signed hello
        self.server_name = None
        self.inbound = True
        self.address = args.address
        self.uncallable_address = None
        self.client = None
        self.peer = None
        self.cert = None
        self.paired = False
        self.libraries = []
        self.limits = None
        self.whole = None
        self.seq = None
        self.answers = []                       # every answer that had to be signed
        self.unclear = []
        self.defaults = []

    # ---- running a check ---------------------------------------------------------

    def skipped(self, cid):
        for x in (y.strip() for y in getattr(self.a, "skip", "").split(",")):
            if x and (cid == x or (cid.startswith(x) and not cid[len(x):len(x) + 1].isalpha())):
                return True
        return False

    def check(self, cid, section, title, fn, level="MUST", operator=False):
        if self.skipped(cid):
            self.r.add(Result(cid, section, title, level, SKIP, "skipped by --skip", operator))
            return False
        try:
            out = fn()
            status, detail = (PASS, out) if isinstance(out, str) or out is None else out
        except Skip as e:
            status, detail = SKIP, str(e)
        except Fail as e:
            status, detail = FAIL, str(e)
        except CertificateRefused as e:
            status, detail = FAIL, "certificate (core 3.2): %s" % e
        except (OSError, ValueError) as e:
            status, detail = FAIL, "%s: %s" % (type(e).__name__, e)
        self.r.add(Result(cid, section, title, level, status, detail or "", operator))
        return status == PASS

    def skip(self, cid, section, title, why, level="MUST", operator=False):
        self.r.add(Result(cid, section, title, level, SKIP, why, operator))

    def can_ask_operator(self):
        """Whether the checks that need the server's operator run."""
        if self.a.no_operator:
            return False, "--no-operator"
        if self.operator_hook:
            return True, ""
        if self.a.operator or sys.stdin.isatty():
            return True, ""
        return False, "not run interactively (stdin isn't a terminal); pass --operator to run the operator steps anyway"

    def ask_operator(self, action, text, done, timeout=None, cid=None):
        """Prints what the operator must do on the server, then waits until
        done() is true (polled every 2 s), the time is up, or Enter is
        pressed. Returns whether done() became true."""
        cid = cid or {"want": "Q1", "accept2": "UP2", "confirm": "K11", "rotate": "O1", "rotate2": "O3", "revoke": "O4",
                      "change": "T9", "watch": "VW1"}.get(action)
        if cid and self.skipped(cid):
            return False
        timeout = timeout or self.a.operator_timeout
        if self.operator_hook:
            self.operator_hook(action)
        else:
            self.r.note("OPERATOR, on the server under test: %s (waiting up to %d s; press Enter to skip)" % (text, timeout))
        end = time.time() + timeout
        while time.time() < end:
            if done():
                return True
            if not self.operator_hook and sys.stdin.isatty():
                r, _, _ = select.select([sys.stdin], [], [], 2)
                if r:
                    sys.stdin.readline()
                    return done()
            else:
                time.sleep(0.5 if self.operator_hook else 2)
        return done()

    @staticmethod
    def unsigned_404(a, also=()):
        if a.signed:
            raise Fail("a signed answer %s; expected the unsigned 404 of a failed check (core 6.4)" % a.short())
        if a.status != 404 and a.status not in also:
            raise Fail("got %s; expected the unsigned 404 (core 6.4)" % a.short())
        return "unsigned %d" % a.status

    def signed(self, a, status, typ=None):
        """The answer is signed by the server's key, with this status (and
        problem type), and well formed (core 6.6, 2.3, 8.2)."""
        self.answers.append(a)
        if a.status != status:
            raise Fail("got %s; expected a signed %d%s" % (a.short(), status, " " + typ if typ else ""))
        if not a.signed:
            raise Fail("the %d isn't signed by the server's key (core 6.6): %s; %s" % (status, a.why_unsigned, a.short()))
        if status in (200, 202):
            if a.json_error:
                raise Fail("the %d isn't I-JSON (core 2.3): %s" % (status, a.json_error))
            if not isinstance(a.json, dict):
                raise Fail("the %d isn't a JSON object" % status)
            if has_null(a.json):
                raise Fail("the answer sends null (core 2.3: no MSFP member is null)")
        elif typ is not None and a.problem_type() != typ:
            raise Fail("a signed %d of type %r; expected %r (core 8.2)" % (status, a.problem_type(), typ))
        return a

    def note_default(self, section, text):
        """A recommended default the server doesn't follow (proposal 17:
        copy, track and subtitle choice): a note, never a failure."""
        if (section, text) not in self.defaults and len(self.defaults) < 50:
            self.defaults.append((section, text))

    def note_unclear(self, section, text):
        if (section, text) not in self.unclear:
            self.unclear.append((section, text))

    # ---- the suite's side --------------------------------------------------------

    def set_key(self, k):
        self.old_keys.append(self.key)
        self.key = k
        if self.client:
            self.client.key = k

    def suite_parts(self):
        parts = {p: [VERSION] for p in PARTS}
        if self.group_on():
            parts["groups"] = [VERSION]
        return parts

    def hello_body(self, state="paired", answer=False, parts=None, **over):
        """The suite's hello (core 9.2): its versions, its pins, its address."""
        b = {"state": state, "core": [VERSION], "parts": self.suite_parts() if parts is None else parts}
        if self.cert and self.inbound:
            b["certificates"] = [self.cert.pin]
        b["address"] = self.address or self.uncallable_address
        if not self.inbound:
            b["inbound"] = False
            if not answer and self.server_keys:
                b["knownKey"] = b64u(self.server_keys[0])
        b.update(over)
        return {k: v for k, v in b.items() if v is not None}

    def learn(self, j):
        """What the suite keeps of the server's latest signed hello,
        request or answer (core 3.2, 10.3)."""
        if not isinstance(j, dict):
            return
        self.server_versions = (j.get("core"), j.get("parts"))
        if self.client:
            self.client.pins = read_pins(j.get("certificates"))

    def learn_hello(self, j, rec):
        """A hello request from the server; returns why it is refused, or None."""
        for f, t in (("core", list), ("parts", dict), ("certificates", list), ("address", str), ("inbound", bool),
                     ("nextCommitment", str)):
            if f in j and not isinstance(j[f], t):
                rec["faults"].append("its hello's %s isn't a%s %s (core 9.2)" % (f, "n" if t is list else "", t.__name__))
        self.learn(j)
        return None

    def on_server_rotated(self, old, new):
        self.r.note("the server rotated its key toward this suite: %s -> %s" % (b64u(old), b64u(new)))

    def parts_on(self):
        """The parts on between the suite and the server (core 10.3)."""
        core, parts = self.server_versions
        if not isinstance(core, list) or VERSION not in [x for x in core[:8] if is_int(x)]:
            return set()
        theirs = parts if isinstance(parts, dict) else {}
        on = set()
        for p in ("catalog", "media", "playback", "video", "music"):
            v = theirs.get(p)
            if isinstance(v, list) and VERSION in [x for x in v[:8] if is_int(x)] and all(n in on for n in NEEDS[p]):
                on.add(p)
        return on

    def hello(self, state="paired", key=None, keys=None, **over):
        a = self.client.call("hello", self.hello_body(state, **over), key=key, keys=keys)
        if a.signed and a.status == 200:
            self.learn(a.json)
        return a

    # ---- the run -----------------------------------------------------------------------

    def run(self):
        a = self.a
        if a.code:
            if not self.check("C1", "core 5.1", "the pairing code reads as core 5.1 says", self.c_code):
                return
        if not self.target:
            self.r.note("no target: give --target, or a --code that names one")
            return
        if self.target.startswith("http://") and not self.allow_http:
            self.r.note("the target's address is http://: core 0 is https only; pass --plain-http for an implementation's test switch")
            return
        if self.allow_http:
            self.r.note("--plain-http: plain http, a test switch OUTSIDE the protocol (core 3.1); the HTTPS checks are skipped")
        self.r.note("target %s; this suite's key %s, fingerprint %s" % (self.target, self.key.text, fingerprint(self.key.pub)))
        self.client = Client(self.key, self.target, server_keys=self.server_keys, connect=a.connect)
        self.server_keys.extend([self.code[0]] if self.code else [])
        try:
            self.discovery_checks()
            Client(Key(), self.target, connect=a.connect, timeout=10, base=self.client.base,
                   any_certificate=True).call("hello", {"state": "waiting"})
        except (OSError, CertificateRefused) as e:
            self.r.add(Result("S0", "core 3", "the server answers at its address", "MUST", FAIL, "%s: %s" % (type(e).__name__, e)))
            return
        self.stranger()
        if not a.code:
            self.r.note("no --code: the pairing and paired checks are skipped")
            return
        if not self.address:
            self.r.note("no --address: the suite can't pair (the server must reach it); see --help")
            return
        why = check_address(self.address, self.allow_http)
        if why:
            self.r.note("--address %s breaks core 5.2 (%s): the server should refuse it" % (self.address, why))
        self.start_peer()
        try:
            self.client.on_cert_refused = lambda: self.hello()
            if not self.pairing():
                return
            self.waiting()
            if not self.wait_for_accept():
                return
            self.paired = True
            self.paired_checks()
        finally:
            self.peer.close()

    def start_peer(self):
        a = self.a
        if not self.allow_http:
            if a.tls_cert:
                self.cert = FileCertificate(a.tls_cert, a.tls_key)
            else:
                hosts = [split_hostport(host_of(x), 443)[0] for x in (self.address, self.new_address())]
                self.cert = Certificate(sorted(set(hosts)))
            self.r.note("this suite serves HTTPS with a self-signed certificate, pin %s (named in its hellos, core 3.2)" % self.cert.pin)
        self.peer = PeerServer(self)
        ctx = self.cert.context() if self.cert else None
        self.peer.listen("main", self.address, a.listen, ctx)
        self.peer.hosts.add(host_of(self.address))
        self.peer.offer = [{"id": "lib_conformance", "name": "Conformance films"}]
        # The clip (clip.py): H.264 Constrained Baseline 1.2 and AAC-LC mono
        # at 22,050 Hz in MP4, track IDs its stream indexes (MSFP Media video 3.7), a
        # preview layout of one sheet so that a server reading the catalog
        # is seen not to fetch previews ahead (MSFP Media video 5.5).
        clip_copy = [{"id": "c_clip", "durationMs": clip.DURATION_MS, "mediaType": "video/mp4", "size": len(clip.MP4),
                      "etag": clip.ETAG, "video": {"codec": "h264", "codecs": "avc1.42C00C", "width": 160, "height": 90,
                                                   "bitDepth": 8, "colour": {"primaries": 1, "transfer": 1, "matrix": 1},
                                                   "fps": "10/1"},
                      "audioTracks": [{"id": "a1", "language": "en", "channels": 1, "default": True, "codec": "aac",
                                       "codecs": "mp4a.40.2", "sampleRate": 22050}],
                      "previewLayout": {"tileWidth": 160, "tileHeight": 90, "columns": 1, "rows": 1, "intervalMs": 2000,
                                        "tileCount": 1}}]
        self.peer.pages = {
            "": {"titles": [{"key": "movie/tmdb/550", "library": "lib_conformance", "copies": clip_copy},
                            {"key": "movie/tmdb/603", "library": "lib_conformance", "copies": clip_copy}],
                 "next": "movie/tmdb/603", "seq": 1},
            "movie/tmdb/603": {"titles": [{"key": "movie/tmdb/949", "library": "lib_conformance",
                                           "copies": clip_copy}], "seq": 1},
        }

    def new_address(self):
        return self.a.new_address or derive_address(self.address, 1)

    def paired_checks(self):
        self.hello_checks()
        self.version_checks()
        self.json_checks()
        self.signing_checks()
        self.nonce_checks()
        self.catalog_checks()
        self.playback_checks()
        self.reader_checks()
        self.load_checks()
        self.group_checks()
        self.uncallable_pairing()
        self.key_checks()
        self.address_checks()
        self.server_key_checks()
        self.uncallable_checks()
        self.end_checks()
        self.answer_checks()

    # ---- the code (core 5) ---------------------------------------------------------------

    def c_code(self):
        key, secret, expiry, address = parse_code(self.a.code, self.allow_http)
        self.code = (key, secret, expiry, address)
        if self.target and self.target != address:
            self.r.note("--target %s differs from the code's address %s: the code's is used" % (self.target, address))
        self.target = address
        left = expiry - time.time()
        if left <= 0:
            raise Fail("the code expired at %d" % expiry)
        if left > 24 * 3600 + 60:
            raise Fail("expires in %.1f hours; core 5.1 says 24 hours after it was made" % (left / 3600))
        return "key %s (fingerprint %s), address %s, expires in %.1f hours" % (b64u(key), fingerprint(key), address, left / 3600)

    # ---- discovery (core 11) -----------------------------------------------------------------

    def discovery_checks(self):
        c = self.client

        def missing():
            cc = Client(Key(), self.target, connect=self.a.connect, any_certificate=True)
            conn = cc._conn(False)[0]
            conn.request("GET", "/.well-known/msfp-conformance-no-such-document")
            r = conn.getresponse()
            other = (r.status, r.read())
            conn.close()
            return other[0]

        def doc():
            status, hdrs, body = c.discovery()
            if status == 404:
                other = missing()
                if other != 404:
                    return (PASS, "no document (404); a missing path answers %d, so the two can be told apart (core 11 "
                                  "should: like any path that doesn't exist)" % other)
                return "no document: 404, like a path that doesn't exist (off by default)"
            if status != 200:
                # A server that serves none answers like any path that
                # doesn't exist (core 11), whatever that answer is.
                other = missing()
                if other == status:
                    return "no document: %d, like a path that doesn't exist (off by default)" % status
                raise Fail("GET /.well-known/msfp answered %d, and a path that doesn't exist %d: a document (200), "
                           "or the answer of a missing path" % (status, other))
            try:
                j = parse_ijson(body)
            except NotIJSON as e:
                raise Fail("the document isn't I-JSON: %s" % e)
            if len(body) > 16 * 1024:
                raise Fail("the document is over 16 KiB (a fetcher reads 16 KiB)")
            if not isinstance(j, dict):
                raise Fail("the document isn't a JSON object")
            if "core" in j and not isinstance(j["core"], list):
                raise Fail("core isn't an array")
            if "parts" in j and not isinstance(j["parts"], dict):
                raise Fail("parts isn't an object")
            if "base" in j:
                why = check_base(j["base"])
                if why:
                    raise Fail("base %r: %s (a document whose base breaks core 11 counts as none)" % (j["base"], why))
                c.base = j["base"]
            return "a document: core %s, base %s (used for every call)" % (j.get("core"), c.base)
        self.check("D1", "core 11", "GET /.well-known/msfp: a valid document, or the 404 of a missing path", doc)

    # ---- strangers (core 6.4) -------------------------------------------------------------------

    def stranger(self):
        st = Key()
        c = Client(st, self.target, connect=self.a.connect, any_certificate=True, base=self.client.base)

        def unknown_key():
            return self.unsigned_404(c.call("hello", self.hello_body("waiting")))
        self.check("S1", "core 6.4", "a stranger's hello gets the unsigned 404", unknown_key)

        def other_calls():
            bodies = {"rotate": {"newKey": Key().text, "date": int(time.time()), "oldSignature": "x", "newSignature": "x"},
                      "revoke": {"date": int(time.time()), "signature": "x"}, "libraries": {}, "titles": {"libraries": []},
                      "play": {"title": "movie/tmdb/949"}, "progress": {"id": "play_" + "a" * 26}, "stop": {"id": "play_" + "a" * 26}}
            for name, b in bodies.items():
                try:
                    self.unsigned_404(c.call(name, b))
                except Fail as e:
                    raise Fail("%s: %s" % (name, e))
            return "rotate, revoke, libraries, titles, play, progress, stop: unsigned 404"
        self.check("S2", "core 6.4, 6.5", "a stranger's other calls get the unsigned 404", other_calls)

        def same_as_missing():
            a = c.call("hello", self.hello_body("waiting"))
            path = c.base + "conformance-no-such-call"
            body = dumps(self.hello_body("waiting"))
            b = c.raw(path, body, c.headers(path, body))
            if a.status != b.status or a.body != b.body:
                raise Fail("a failed check: %d %r; a missing path: %d %r" % (a.status, a.body[:60], b.status, b.body[:60]))
            ct = lambda x: [v for k, v in x.headers if k.lower() == "content-type"]
            if ct(a) != ct(b):
                raise Fail("Content-Type differs: %s and %s" % (ct(a), ct(b)))
            return "same status, body and type"
        self.check("S3", "core 3, 14", "the unsigned 404 can't be told from a path that doesn't exist", same_as_missing, "SHOULD")

        def wrong_method():
            out = []
            for path in (c.base + "hello", c.base + "conformance-no-such-call"):
                conn = c._conn(False)[0]
                conn.request("GET", path)
                r = conn.getresponse()
                r.read()
                conn.close()
                out.append(r.status)
            if out[0] != out[1]:
                raise Fail("GET on hello: %d; GET on a missing path: %d" % tuple(out))
            return "both %d" % out[0]
        self.check("S4", "core 3, 14", "a GET on a federation path answers like a missing path", wrong_method, "SHOULD")

    # ---- pairing (core 9.1, 7.1) ----------------------------------------------------------------

    def pair_body(self, key, address=None, secret=None, code=None, **over):
        _, code_secret, _, _ = code or self.code
        address = address or self.address
        b = {"address": address, "name": "MSFP conformance suite",
             "secretHash": b64u(hashlib.sha256(code_secret).digest()),
             "proof": b64u(proof(secret or code_secret, key.pub, address)),
             "nextCommitment": self.next_keys[0].commitment}
        if not self.inbound:
            b["inbound"] = False
        b.update(over)
        return dumps({k: v for k, v in b.items() if v is not ...})

    def pair_raw(self, body, key, c=None, **kw):
        c = c or self.client
        path = c.base + "pair"
        return c.raw(path, body, c.headers(path, body, key=key, **kw))

    def pairing(self):
        c = self.client
        used = {}

        def refused(a, name):
            if a.status == 200:
                used["by"] = name
                raise Fail("ACCEPTED %s; the code is now used, so the pairing can't go on" % name)
            return self.unsigned_404(a)

        def wrong_proof():
            return refused(self.pair_raw(self.pair_body(self.key, secret=os.urandom(32)), self.key), "a wrong proof")
        self.check("P1", "core 9.1, 6.6", "pair with a wrong proof: the unsigned 404, and the code stays live", wrong_proof)

        def forged_identity():
            fake = Key()
            fake.text = IDENTITY_KEY
            body = self.pair_body(fake)
            hs = [(k, IDENTITY_KEY if k == H_KEY else IDENTITY_SIG if k == H_SIG else v) for k, v in c.headers(c.base + "pair", body)]
            a = c.raw(c.base + "pair", body, hs)
            if a.status == 200:
                raise Fail("ACCEPTED a pair signed by the identity key with R = identity, S = 0: the server skips core 2.1's "
                           "point checks, and anyone can forge its peers' requests")
            return self.unsigned_404(a)
        self.check("P2", "core 2.1, 4.1", "pair under the identity key with the forged signature is refused", forged_identity)

        def bad_keys():
            out = []
            for name, text in (("a point of order 8", SMALL_ORDER_KEY), ("a key with a small-order part", MIXED_ORDER_KEY),
                               ("a non-canonical encoding", NON_CANONICAL_KEY)):
                k = Key()
                body = self.pair_body(k)
                hs = [(x, text if x == H_KEY else v) for x, v in c.headers(c.base + "pair", body, key=k)]
                refused(c.raw(c.base + "pair", body, hs), name)
                out.append(name)
            return "refused: " + ", ".join(out)
        self.check("P3", "core 2.1, 4.1", "pair under keys core 2.1 refuses (small order, not canonical) is refused", bad_keys)

        def stale():
            return refused(self.pair_raw(self.pair_body(self.key), self.key, date=str(int(time.time()) - 400)), "a pair dated 400 s ago")
        self.check("P4", "core 6.4", "pair dated 400 seconds ago is refused", stale)

        def bad_addresses():
            bad = ["HTTPS://a.example.org", "https://a.example.org/", "https://a.example.org:443", "https://192.0.2.010:8000",
                   "https://Upper.example.org", "https://a.example.org:08000", "https://a.example.org?x", "https://a_b.example.org"]
            if not self.allow_http:
                bad.append("http://192.0.2.10:8000")
            for addr in bad:
                k = Key()
                refused(self.pair_raw(self.pair_body(k, address=addr), k), "the address %r" % addr)
            return "refused %d addresses that break core 5.2" % len(bad)
        self.check("P5", "core 5.2, 9.1", "pair naming an address that breaks core 5.2 is refused, the code stays live", bad_addresses)
        if used:
            return False

        def bad_bodies():
            cases = [("inbound false with an address that isn't https://msfp-<26>.invalid", {"inbound": False}),
                     ("inbound not a boolean", {"inbound": "false"}),
                     ("nextCommitment not 32 bytes of base64url", {"nextCommitment": "abc"}),
                     ("secretHash not 32 bytes", {"secretHash": b64u(os.urandom(31))}),
                     ("name null (core 2.3)", {"name": None}),
                     ("name not a string (core 2.3)", {"name": 7})]
            for name, over in cases:
                k = Key()
                body = self.pair_body(k, **over)
                if "name" in over:
                    j = parse_ijson(body)
                    j["name"] = over["name"]
                    body = dumps(j)
                refused(self.pair_raw(body, k), name)
            k = Key()
            refused(self.pair_raw(b"[]", k), "a body that is an array")
            return "refused %d bodies" % (len(cases) + 1)
        self.check("P6", "core 9.1, 7.5, 2.3", "pair bodies with a bad member are refused with the unsigned 404, the code stays live",
                   bad_bodies)
        if used:
            return False

        state = {}

        def pair():
            body = self.pair_body(self.key)
            hs = c.headers(c.base + "pair", body)
            state["req"] = (body, hs)
            a = c.raw(c.base + "pair", body, hs)
            if a.status != 200:
                raise Fail("got %s: check that the code is live and that the server reaches %s" % (a.short(), self.address))
            self.signed(a, 200)
            name = a.json.get("name")
            if not isinstance(name, str):
                raise Fail("no name string in %r" % a.body[:100])
            self.server_name = name
            nc = a.json.get("nextCommitment")
            if nc is not None:
                if not isinstance(nc, str) or unb64u(nc, 32) is None:
                    raise Fail("nextCommitment %r isn't base64url of 32 bytes (core 12.5)" % nc)
                self.peer.server_commitment = nc
            return "paired from this side; the server calls itself %r" % name
        if not self.check("P7", "core 9.1, 7.1", "pair with the code: 200, signed by the code's key, with a name", pair):
            return False

        def commits():
            if self.peer.server_commitment is None:
                raise Fail("the pair answer carries no nextCommitment: the server keeps no next key, and a rotation "
                           "will need both operators (core 12.5, 12.3 case 4)")
            return "nextCommitment %s" % self.peer.server_commitment
        self.check("P8", "core 12.5", "the pair answer carries the server's next key commitment", commits, "SHOULD")

        def replay():
            body, hs = state["req"]
            return self.unsigned_404(c.raw(c.base + "pair", body, hs))
        self.check("P9", "core 6.5", "the same pair request again (its nonce replayed): the unsigned 404", replay)

        def code_used():
            a = self.pair_raw(self.pair_body(self.key), self.key)
            if a.status == 200:
                raise Fail("the code worked twice")
            self.signed(a, 404)
            return "signed 404"
        self.check("P10", "core 9.1, 7.1", "the used code again, from the now known key: a signed 404", code_used)

        def second_code():
            if not self.a.code2:
                raise Skip("give --code2 (a second code of the same server) to check the signed 409 and the uncallable pairing")
            self.code2 = parse_code(self.a.code2, self.allow_http)
            if self.code2[0] != self.code[0]:
                raise Fail("--code2 holds another key than --code")
            a = self.pair_raw(self.pair_body(self.key, code=self.code2), self.key)
            self.answers.append(a)
            if a.status != 409:
                raise Fail("got %s; expected a signed 409 with an empty body" % a.short())
            if not a.signed:
                raise Fail("the 409 isn't signed (core 9.1: it is, since the key is known): %s" % a.why_unsigned)
            if a.body or a.media_type != "-":
                raise Fail("the 409 has a body %r or a Content-Type (%s): core 3 says an empty body and none" % (a.body[:60], a.media_type))
            return "a signed 409, no body, no Content-Type (signed with '-' as its media type)"
        self.check("P11", "core 9.1, 3, 6.3", "a second live code from a known key: the signed 409 with no body", second_code)
        return True

    # ---- waiting for the server's operator (core 7.1, 7.3) -----------------------------------------

    def waiting(self):
        c = self.client

        def hello_waiting():
            a = self.signed(self.hello("waiting"), 200)
            if a.json.get("state") not in ("waiting", "paired"):
                raise Fail("state %r" % a.json.get("state"))
            return "state %s" % a.json["state"]
        self.check("W1", "core 9.2, 7.3", "hello waiting: 200 signed, state waiting", hello_waiting)

        def cant_force():
            a = self.signed(self.hello("paired"), 200)
            if a.json.get("state") == "paired":
                raise Skip("the server's operator had already accepted, so this can't be told")
            again = self.signed(self.hello("waiting"), 200)
            if again.json.get("state") != "waiting":
                raise Fail("a hello paired from a peer in state ours moved it on")
            return "still waiting"
        self.check("W2", "core 7.1, 9.2", "hello paired from a peer waiting for the server's operator changes nothing", cant_force)

        def libs_waiting():
            a = c.call("libraries", {})
            if a.status == 200 and a.signed:
                if self.signed(self.hello("waiting"), 200).json.get("state") == "paired":
                    raise Skip("already accepted")
                raise Fail("libraries answered 200 before the server's operator accepted")
            self.signed(a, 404, "not-found")
            return "signed 404 not-found"
        self.check("W3", "core 7.3; MSFP Media listing 2", "libraries before the server's operator accepts: a signed 404", libs_waiting)

    def wait_for_accept(self):
        timeout = self.a.accept_timeout if self.a.accept_timeout is not None else (60 if self.a.no_operator else 600)
        t0 = time.time()

        def accepted():
            if self.peer.state == "paired":
                return True
            a = self.hello("waiting")
            if a.signed and a.status == 200 and a.json and a.json.get("state") == "paired":
                self.r.note("the server answered paired before this suite heard its hello")
                self.peer.state = "paired"
            return self.peer.state == "paired"

        text = "accept the pairing of peer 'MSFP conformance suite', fingerprint %s" % fingerprint(self.key.pub)
        if self.a.no_operator and not self.operator_hook:
            self.r.note("--no-operator: waiting %d s for the pairing to be accepted without a prompt (a test switch)" % timeout)
            end = time.time() + timeout
            while time.time() < end and not accepted():
                time.sleep(5)
        else:
            self.ask_operator("accept", text, lambda: self.peer.state == "paired" or (time.time() - t0 > 10 and accepted()), timeout)
        if self.peer.state != "paired":
            self.skip("W4", "core 7.1", "the server's operator accepted; the server tells this suite",
                      "not accepted within %d s: the paired checks are skipped" % timeout, operator=True)
            return False

        def told_us():
            hellos = [x for x in self.peer.requests if x["call"] == "hello"]
            if not hellos:
                raise Fail("the server never sent this suite a hello (core 7.1 step 6), or it couldn't reach %s" % self.address)
            bad = [x for x in hellos if x["faults"]]
            if bad:
                raise Fail("; ".join(bad[0]["faults"]))
            j = hellos[0]["json"]
            probs = []
            if j.get("state") != "paired":
                probs.append("state %r" % j.get("state"))
            core = j.get("core")
            if not isinstance(core, list) or VERSION not in core:
                probs.append("core %r doesn't list %d (core 10.2: sent in every hello)" % (core, VERSION))
            if not isinstance(j.get("parts"), dict):
                probs.append("no parts object (core 10.2)")
            if j.get("inbound") is False:
                probs.append("inbound false: the server says it can't be called, yet it called")
            if probs:
                raise Fail("; ".join(probs))
            notes = []
            if "address" not in j:
                notes.append("no address (core 9.2: should be in every hello)")
            if self.cert and not j.get("certificates"):
                notes.append("no certificates (core 3.2: should name its pins)")
            tls = [e for e in self.peer.tls_errors if e[0] >= t0]
            return "%d hello(s), well signed, core %s, parts %s%s%s" % (
                len(hellos), core, sorted(j["parts"]), "; " + "; ".join(notes) if notes else "",
                "; TLS failures seen before: %s" % tls[0][2] if tls else "")
        self.check("W4", "core 7.1, 6.4, 9.2", "on accepting, the server sends this suite a signed hello paired", told_us,
                   operator=True)
        return True

    # ---- hello (core 9.2, 3.2) -----------------------------------------------------------------------

    def hello_checks(self):
        def hello_paired():
            a = self.signed(self.hello("paired"), 200)
            j = a.json
            if j.get("state") != "paired":
                raise Fail("state %r" % j.get("state"))
            probs = []
            core, parts = j.get("core"), j.get("parts")
            if not isinstance(core, list) or len(core) > 8 or any(not is_int(x) or x < 0 for x in core):
                probs.append("core %r isn't an array of at most 8 non-negative integers (core 10.2)" % core)
            elif VERSION not in core:
                probs.append("core %r doesn't list %d (a core %d server lists every version it speaks)" % (core, VERSION, VERSION))
            if not isinstance(parts, dict):
                probs.append("parts isn't an object (core 10.2: sent in every hello)")
            else:
                import re
                for n, v in parts.items():
                    if not re.fullmatch(r"[a-z-]{1,32}", n):
                        probs.append("part name %r isn't lowercase letters and -, at most 32" % n)
                    if not isinstance(v, list) or len(v) > 8 or any(not is_int(x) or x < 0 for x in v):
                        probs.append("parts.%s %r isn't an array of at most 8 non-negative integers" % (n, v))
            if "certificates" in j and not isinstance(j["certificates"], list):
                probs.append("certificates isn't an array")
            elif any(not (isinstance(p, str) and len(p) == 43 and unb64u(p, 32)) for p in j.get("certificates", [])):
                probs.append("a certificates entry isn't 43 characters of base64url (core 3.2)")
            if "address" in j and check_address(j["address"], self.allow_http):
                probs.append("address %r breaks core 5.2" % j["address"])
            if j.get("inbound") is False:
                probs.append("inbound false from the server this suite calls")
            if "inbound" in j and not isinstance(j["inbound"], bool):
                probs.append("inbound isn't a boolean")
            if probs:
                raise Fail("; ".join(probs))
            on = self.parts_on()
            self.r.note("the server lists core %s, parts %s; on between it and this suite: %s"
                        % (core, {k: v for k, v in parts.items()}, ", ".join(p for p in PARTS if p in on) or "none"))
            return "paired; core %s; pins %s" % (core, self.client.pins or "none")
        self.check("H1", "core 9.2, 10.2", "hello paired: 200 signed, paired, its versions and pins well formed", hello_paired)

        def lists_parts():
            on = self.parts_on()
            miss = [p for p in ("catalog", "playback", "media", "video") if p not in on]
            if miss:
                raise Fail("not on between the server and this suite: %s; their checks are skipped (every part is "
                           "optional, but a media server without them shares nothing)" % ", ".join(miss))
            return "catalog, playback, media and video 1 on%s" % (", and music" if "music" in on else "; music not listed (optional)")
        self.check("H2", "core 10.1, 10.3", "the server lists catalog, playback, media and video at version 0", lists_parts, "SHOULD")

        def address():
            a = self.signed(self.hello("paired"), 200)
            if "address" not in a.json:
                raise Fail("the hello answer carries no address")
            if a.json["address"] != self.target:
                raise Fail("the server names %r; this suite knows it at %s (core 13.2 would start a move)" % (a.json["address"], self.target))
            return "names %s" % a.json["address"]
        self.check("H3", "core 9.2, 13.1", "the server names its own address in its hello answers", address, "SHOULD")

        def pins_match():
            if not self.client.https:
                raise Skip("--plain-http: no certificates")
            a = self.hello("paired")
            if a.trusted:
                return "the certificate is publicly trusted (pins optional, core 3.2 says to name them all the same)"
            if a.leaf_pin not in self.client.pins:
                raise Fail("the leaf served (%s) isn't among the pins named (%s)" % (a.leaf_pin, self.client.pins or "none"))
            return "the served leaf %s is named" % a.leaf_pin
        self.check("H4", "core 3.2", "over https, the server names the pin of the leaf certificate it serves", pins_match, "SHOULD")

        def checked_call():
            if not self.client.https:
                raise Skip("--plain-http")
            a = self.client.call("libraries", {})
            return "libraries passed the trusted-or-named check (%s)" % a.status
        self.check("H5", "core 3.2", "over https, the calls that need trusted or named pass", checked_call)

        def no_pins():
            if not self.client.https:
                raise Skip("--plain-http")
            a = self.client.call("hello", self.hello_body("paired", certificates=None))
            self.signed(a, 200)
            b = self.client.call("hello", self.hello_body("paired"))
            self.signed(b, 200)
            self.learn(b.json)
            return "a hello without certificates (removing this suite's pins) and one with them again: both 200"
        self.check("H6", "core 3.2", "a hello without certificates is valid; the next one names the pins again", no_pins)

    # ---- versions and parts (core 10) -----------------------------------------------------------------

    def version_checks(self):
        c = self.client
        base = self.hello_body("paired")

        def with_(**over):
            b = dict(base)
            b.update(over)
            return {k: v for k, v in b.items() if v is not ...}

        def refused():
            cases = [("core not an array", with_(core=1)), ("core an object", with_(core={"1": 1})),
                     ("core null", with_(core=None)), ("parts an array", with_(parts=[])),
                     ("parts a string", with_(parts="catalog")), ("a known part's list not an array",
                                                                   with_(parts=dict(base["parts"], catalog=1))),
                     ("a known part's list null", with_(parts=dict(base["parts"], catalog=None)))]
            bad = []
            for name, b in cases:
                a = c.call("hello", dumps(b))
                if not (a.status == 404 and a.signed):
                    bad.append("%s: %s" % (name, a.short()))
                self.answers.append(a)
            if bad:
                raise Fail("expected a signed 404 (a bad body: nothing stored) for:\n" + "\n".join(bad))
            return "%d refused with a signed 404" % len(cases)
        self.check("V1", "core 10.2, 9.2", "hello with core or parts of the wrong type is a bad body: a signed 404", refused)

        def tolerated():
            cases = [("an unknown part with any value", with_(parts=dict(base["parts"], **{"zz-conformance": 5}))),
                     ("a part name in capitals (an unknown name)", with_(parts=dict(base["parts"], Catalog="x"))),
                     ("a version twice", with_(core=[VERSION, VERSION])),
                     ("entries that aren't non-negative integers", with_(core=[VERSION, "2", -1, 2.5, True])),
                     ("more than 8 entries", with_(core=[0, 1, 2, 3, 4, 5, 6, 7, 8, 9]))]
            bad = []
            for name, b in cases:
                a = c.call("hello", dumps(b))
                if not (a.status == 200 and a.signed):
                    bad.append("%s: %s" % (name, a.short()))
                self.answers.append(a)
            self.signed(self.hello("paired"), 200)
            if bad:
                raise Fail("expected 200 (core 10.2: ignored) for:\n" + "\n".join(bad))
            return "%d accepted" % len(cases)
        self.check("V2", "core 10.2", "unknown parts and odd version entries are ignored, not refused", tolerated)

        def part_off(dropped, calls, also=None):
            def run():
                on = self.parts_on()
                need = {"playback": "playback", "catalog": "catalog", "media": "playback", "video": "video"}[dropped]
                if need not in on:
                    raise Skip("%s isn't on between the server and this suite" % need)
                parts = {k: v for k, v in self.suite_parts().items() if k != dropped}
                self.signed(self.hello("paired", parts=parts), 200)
                try:
                    out = []
                    for name, body in calls:
                        a = c.call(name, body)
                        self.signed(a, 404, "not-supported")
                        out.append(name)
                    extra = also() if also else ""
                finally:
                    self.signed(self.hello("paired"), 200)
                a = self.signed(c.call("libraries", {}), 200)
                return "without %s: %s answered 404 not-supported%s; with it again, libraries %d" % (
                    dropped, ", ".join(out) or "nothing", extra, a.status)
            return run

        play = ("play", {"title": "movie/tmdb/949"})

        def no_limits():
            a = self.signed(c.call("libraries", {}), 200)
            if "limits" in a.json:
                raise Fail("libraries carries limits while playback is off (MSFP Media listing 2, playback 5: only when it is on)")
            return "; libraries carries no limits"

        def no_video_titles():
            a = self.signed(c.call("libraries", {}), 200)
            libs = [x["id"] for x in a.json.get("libraries", []) if isinstance(x, dict) and isinstance(x.get("id"), str)]
            if not libs:
                return "; nothing offered, titles not checked"
            t = self.signed(c.call("titles", {"libraries": libs}), 200)
            from .content import video_key
            vids = [x.get("key") for x in t.json.get("titles", []) if isinstance(x, dict) and video_key(x.get("key"))]
            if vids:
                raise Fail("titles holds video titles while video is off: %s (catalog 5)" % vids[:3])
            return "; titles holds no video title"

        self.check("V3", "core 10.4, playback", "playback off: play gets the signed 404 not-supported, libraries no limits",
                   part_off("playback", [play, ("progress", {"id": "play_" + "a" * 26}), ("stop", {"id": "play_" + "a" * 26})], no_limits))
        self.check("V4", "core 10.4, catalog", "catalog off: libraries and titles get the signed 404 not-supported",
                   part_off("catalog", [("libraries", {}), ("titles", {"libraries": []})]))
        self.check("V5", "core 10.3, 10.4", "media off turns playback off (a dependency): play gets not-supported",
                   part_off("media", [play]))
        self.check("V6", "catalog 5; core 10.3", "video off: titles holds no video title", part_off("video", [], no_video_titles))

        def no_core():
            # A hello without core, then one with an empty core: no common
            # core, so every call but hello gets the signed 404
            # not-supported (core 10.2, 10.3, 10.4); a hello with core 0
            # puts it back.
            out = []
            for label, over in (("no core", {"core": None}), ("core []", {"core": []})):
                self.signed(self.hello("paired", **over), 200)
                try:
                    a = c.call("libraries", {})
                    self.signed(a, 404, "not-supported")
                    out.append(label)
                finally:
                    self.signed(self.hello("paired"), 200)
            a = self.signed(c.call("libraries", {}), 200)
            return "with %s: libraries answered 404 not-supported; with core 0 again, libraries %d" % (" and ".join(out), a.status)

        self.check("V7", "core 10.2, 10.3, 10.4", "no common core: every call but hello gets the signed 404 not-supported", no_core)

    # ---- JSON (core 2.3) --------------------------------------------------------------------------------

    def json_checks(self):
        c = self.client
        base = dumps(self.hello_body("paired"))
        inner = base[1:-1]

        def body(prefix=b"", suffix=b""):
            return b"{" + prefix + inner + suffix + b"}"

        def mod(drop=(), **over):
            d = self.hello_body("paired")
            for k in drop:
                d.pop(k, None)
            d.update(over)
            return dumps(d)

        st = self.hello_body("paired")
        refused = [
            ("no state", mod(drop=("state",))),
            ("state not a string", mod(state=7)),
            ("state null", mod(state=None)),
            ("State in another case only", mod(drop=("state",), State="paired")),
            ("certificates not an array", mod(certificates="x")),
            ("certificates null", mod(certificates=None)),
            ("address null", mod(address=None)),
            ("address not a string", mod(address=7)),
            ("inbound not a boolean", mod(inbound="true")),
            ("inbound null", mod(inbound=None)),
            ("nextCommitment not a string", mod(nextCommitment=5)),
            ("an array, not an object", b"[" + base + b"]"),
            ("not JSON", b"state=paired"),
            ("state twice", body(suffix=b',"state":"ended"')),
            ("state twice, once escaped", body(suffix=b',"\\u0073tate":"ended"')),
            ("a member twice, nested", body(suffix=b',"x":{"a":1,"a":2}')),
            ("a lone high surrogate, escaped", body(suffix=b',"x":"\\ud800"')),
            ("a lone low surrogate, escaped", body(suffix=b',"x":"\\udc00x"')),
            ("a noncharacter", body(suffix=b',"x":"\xef\xbf\xbf"')),
            ("invalid UTF-8", body(suffix=b',"x":"\xff"')),
            ("a byte order mark", b"\xef\xbb\xbf" + base),
        ]
        del st
        accepted = [
            ("white space between tokens", b'{ ' + inner.replace(b'":', b'" : ', 1) + b' }'),
            ("a surrogate pair, escaped", body(suffix=b',"x":"\\ud83c\\udfac"')),
            ("unknown members", body(suffix=b',"x":[1,{"y":true}],"Address":7')),
            ("an unknown state string", mod(state="conformance")),
        ]

        def run_refused():
            bad = []
            for name, b in refused:
                a = c.raw(c.base + "hello", b, c.headers(c.base + "hello", b))
                self.answers.append(a)
                if not (a.status == 404 and a.signed):
                    bad.append("%s: %s" % (name, a.short()))
            self.hello("paired")
            if bad:
                raise Fail("expected a signed 404 for:\n" + "\n".join(bad))
            return "%d bodies refused with a signed 404" % len(refused)
        self.check("J1", "core 2.3, 9.2", "hello bodies that aren't valid (I-JSON, types, null) get a signed 404", run_refused)

        def run_accepted():
            bad = []
            for name, b in accepted:
                a = c.raw(c.base + "hello", b, c.headers(c.base + "hello", b))
                try:
                    self.signed(a, 200)
                except Fail as e:
                    bad.append("%s: %s" % (name, e))
            self.hello("paired")
            if bad:
                raise Fail("\n".join(bad))
            return "%d accepted" % len(accepted)
        self.check("J2", "core 2.3, 9.2", "valid hello bodies (white space, surrogate pairs, unknown members and states) pass", run_accepted)

    # ---- signatures (core 2.2, 2.4, 3, 6) ---------------------------------------------------------------

    def signing_checks(self):
        c = self.client
        body = dumps(self.hello_body("paired"))
        p = c.base + "hello"

        def h(**kw):
            return c.headers(p, body, **kw)

        def mutate(hs, name, value):
            return [(k, value if k == name else v) for k, v in hs]

        def resigned(date):
            nonce = b64u(os.urandom(16))
            msg = request_message("POST", c.host, p, date, nonce, body)
            return [("Content-Type", "application/json"), (H_KEY, self.key.text), (H_DATE, date), (H_NONCE, nonce),
                    (H_SIG, b64u(self.key.sign(msg)))]

        def run():
            now = int(time.time())
            k = self.key.text
            alphabet = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789-_"
            alt = k[:-1] + alphabet[(alphabet.index(k[-1]) + 1) % 64]
            cases = [("dated 330 seconds ago", h(date=str(now - 330))), ("dated 330 seconds ahead", h(date=str(now + 330))),
                     ("a date with a plus sign", resigned("+%d" % now)), ("a date with a leading zero", resigned("0%d" % now)),
                     ("a date with a fraction", resigned("%d.0" % now)), ("a date of 0", h(date="0")),
                     ("a date of 2^53", h(date=str(2**53))), ("a nonce of 15 bytes", h(nonce=b64u(os.urandom(15)))),
                     ("a padded nonce", h(nonce=b64u(os.urandom(16)) + "==")),
                     ("a nonce with nonzero unused bits", h(nonce=b64u(b"\0" * 16)[:-1] + "B")),
                     ("a nonce in the standard alphabet", h(nonce="+" + b64u(os.urandom(16))[1:])),
                     ("signed for another host", h(sign_host="conformance.example.org")),
                     ("signed for another path", h(sign_path=c.base + "libraries")),
                     ("signed with another key", mutate(h(key=Key()), H_KEY, k)),
                     ("a key text with nonzero unused bits", mutate(h(), H_KEY, alt)),
                     ("a signature of 63 bytes", mutate(h(), H_SIG, b64u(os.urandom(63))))]
            bad = []
            for name, hs in cases:
                a = c.raw(p, body, hs)
                if a.status == 200 or a.signed:
                    bad.append("%s: %s" % (name, a.short()))
            if bad:
                raise Fail("expected the unsigned 404 for:\n" + "\n".join(bad))
            return "%d forms refused with the unsigned 404" % len(cases)
        self.check("G1", "core 2.2, 6.2, 6.4", "dates, nonces, keys and signatures outside their one form, or signed for "
                   "another host, path or key, are refused", run)

        def headers():
            base = h()
            sig = base[-1]
            forms = [("the signature header twice", base + [sig]), ("the nonce header twice", base + [base[3]]),
                     ("a comma in the signature", base[:-1] + [(sig[0], sig[1] + "," + sig[1])]),
                     ("a tab inside the date", mutate(base, H_DATE, base[2][1][:3] + "\t" + base[2][1][3:]))]
            bad = []
            for name, hs in forms:
                a = c.raw(p, body, hs)
                if a.status == 200 or a.signed:
                    bad.append("%s: %s" % (name, a.short()))
            if bad:
                raise Fail("expected the unsigned 404 for:\n" + "\n".join(bad))
            return "%d forms refused" % len(forms)
        self.check("G2", "core 2.4, 6.4", "each signature header exactly once, no comma, no white space inside", headers)

        def lenient():
            forms = [("lowercase header names", lambda hs: [(k.lower(), v) for k, v in hs]),
                     ("spaces around a value", lambda hs: mutate(hs, H_DATE, "  " + dict(hs)[H_DATE] + " ")),
                     ("Content-Type with a charset", lambda hs: mutate(hs, "Content-Type", "application/json; charset=utf-8")),
                     ("Sec-Fetch-Site: none", lambda hs: hs + [("Sec-Fetch-Site", "none")]),
                     ("Sec-Fetch-Site: same-origin", lambda hs: hs + [("Sec-Fetch-Site", "same-origin")])]
            bad = []
            for name, f in forms:
                a = c.raw(p, body, f(h()))
                if not (a.status == 200 and a.signed):
                    bad.append("%s: %s" % (name, a.short()))
            if bad:
                raise Fail("\n".join(bad))
            return "%d forms accepted" % len(forms)
        self.check("G3", "core 2.4, 2.5, 3", "header names in any case, white space around values, media type parameters, "
                   "Sec-Fetch-Site none or same-origin: accepted", lenient)

        def query():
            path = p + "?conformance=1"
            a = c.raw(path, body, c.headers(path, body))
            if a.status == 200:
                raise Fail("a hello signed over %s was answered 200" % path)
            return self.unsigned_404(a)
        self.check("G4", "core 6.2, 6.4", "a federation call with a query is refused, even when signed so", query)

        def percent():
            path = c.base + "%68ello"
            a = c.raw(path, body, c.headers(path, body, sign_path=c.base + "hello"))
            if a.status == 200:
                raise Fail("%s signed over the decoded path passed: the server rebuilds <target> from a decoded path" % path)
            return self.unsigned_404(a, also=(400,))
        self.check("G5", "core 6.2", "the target is taken as received: a percent-encoded path signed decoded fails", percent)

        def media_type():
            out = []
            for ct, name in ((None, "no Content-Type"), ("text/plain", "text/plain"), ("application/json-seq", "application/json-seq")):
                hs = [x for x in h() if x[0] != "Content-Type"] + ([("Content-Type", ct)] if ct else [])
                a = c.raw(p, body, hs)
                if a.status == 200 or a.signed:
                    raise Fail("%s: %s" % (name, a.short()))
                self.unsigned_404(a)
                out.append(name)
            return "unsigned 404 for " + ", ".join(out)
        self.check("G6", "core 3", "a request that isn't application/json gets the unsigned 404", media_type)

        def cross_site():
            a = c.raw(p, body, h() + [("Sec-Fetch-Site", "cross-site")])
            if a.status == 200 or a.signed:
                raise Fail(a.short())
            return self.unsigned_404(a)
        self.check("G7", "core 3", "a cross-site Sec-Fetch-Site gets the unsigned 404", cross_site)

        def no_coding():
            a = c.raw(p, body, h() + [("Accept-Encoding", "gzip, deflate, br")])
            if a.coding:
                raise Fail("asked with Accept-Encoding: gzip, the answer came with Content-Encoding %s: federation answers "
                           "are sent uncoded, whatever Accept-Encoding says" % a.coding)
            self.signed(a, 200)
            return "200 signed, no Content-Encoding"
        self.check("G8", "core 3, 6.3", "an answer carries no content coding, even to Accept-Encoding: gzip", no_coding)

        def coded_request():
            a = c.raw(p, body, h() + [("Content-Encoding", "gzip")])
            if a.status == 200:
                raise Fail("a request marked Content-Encoding: gzip (its body not gzip at all) was answered 200: the "
                           "receiver must never decode a coded request, and this one isn't even coded")
            # Refused at once, before any signature check (core 3).
            return "refused at once: " + self.unsigned_404(a)
        self.check("G9", "core 3", "a request with a Content-Encoding gets the unsigned 404 at once", coded_request)

        def too_long():
            big = dumps(self.hello_body("paired", x="a" * 70000))
            a = c.raw(p, big, c.headers(p, big))
            if a.status == 200:
                raise Fail("a 70,000-byte body was answered 200 (core 3: at most 64 KiB)")
            return self.unsigned_404(a)
        self.check("G10", "core 3", "a request body over 64 KiB gets the unsigned 404", too_long)

    # ---- nonces (core 6.5, 6.3) ----------------------------------------------------------------------------

    def nonce_checks(self):
        c = self.client
        p = c.base + "hello"
        body = dumps(self.hello_body("paired"))

        def same_twice():
            hs = c.headers(p, body)
            self.signed(c.raw(p, body, hs), 200)
            return self.unsigned_404(c.raw(p, body, hs))
        self.check("N1", "core 6.5", "a request replayed with its nonce gets the unsigned 404", same_twice)

        def nonce_reused():
            n = b64u(os.urandom(16))
            self.signed(c.raw(p, body, c.headers(p, body, nonce=n)), 200)
            other = dumps(self.hello_body("paired", x=1))
            return self.unsigned_404(c.raw(p, other, c.headers(p, other, nonce=n)))
        self.check("N2", "core 6.5", "a nonce used again in a new, validly signed request gets the unsigned 404", nonce_reused)

        def bound():
            from . import ed25519
            from .wire import answer_message
            a = self.signed(self.hello("paired"), 200)
            sig = unb64u(a.header(H_SIG).strip(), 64)
            if ed25519.verify_strict(a.signer, answer_message(b64u(os.urandom(16)), a.status, a.media_type, a.body), sig):
                raise Fail("the answer's signature verifies for another nonce")
            if ed25519.verify_strict(a.signer, answer_message(a.nonce, a.status, "application/problem+json", a.body), sig):
                raise Fail("the answer's signature verifies for another media type")
            return "the signature covers the request's nonce and the answer's media type (core 6.3's five lines)"
        self.check("N3", "core 6.3", "the answer's signature is bound to the request's nonce and the media type", bound)

    # ---- every answer (core 6.6, 8.2) ------------------------------------------------------------------------

    def answer_checks(self):
        def all_signed():
            bad = [a for a in self.answers if not a.signed]
            if bad:
                raise Fail("%d answer(s) the suite needed signed weren't: %s" % (len(bad), "; ".join(
                    "%s: %s" % (a.path, a.why_unsigned) for a in bad[:5])))
            return "%d answers" % len(self.answers)
        self.check("A1", "core 6.6", "every answer the suite needed signed was signed by the server's key", all_signed)

        def problems():
            bad = []
            for a in self.client.log:
                if not a.signed or a.status in (200, 202):
                    continue
                if a.status == 409 and a.path.endswith("/pair"):
                    continue
                # group-join's and group-pair's 409: no row in core 8.2 (GR7 notes it); no body taken, as pair's.
                if a.status == 409 and a.path.endswith(("/group-join", "/group-pair")) and not a.body:
                    continue
                j = a.json
                if a.media_type != "application/problem+json":
                    bad.append("%s %d: Content-Type %s, not application/problem+json" % (a.path, a.status, a.media_type))
                elif not isinstance(j, dict) or not isinstance(j.get("type"), str) or not isinstance(j.get("title"), str) \
                        or j.get("status") != a.status:
                    bad.append("%s %d: not a problem document with type, title and status %d: %r" % (a.path, a.status, a.status, a.body[:80]))
                elif "retryAfter" in j and not (is_int(j["retryAfter"]) and 1 <= j["retryAfter"] <= 3600):
                    bad.append("%s %d: retryAfter %r" % (a.path, a.status, j["retryAfter"]))
                elif has_null(j):
                    bad.append("%s %d: null in the problem document" % (a.path, a.status))
            if bad:
                raise Fail("\n".join(sorted(set(bad))[:10]))
            n = sum(1 for a in self.client.log if a.signed and a.status not in (200, 202))
            return "%d signed refusals, all problem documents" % n
        self.check("A2", "core 3, 8.2", "every signed refusal is a problem document with type, title and its status", problems)
