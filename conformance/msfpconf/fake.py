"""A minimal MSFP version 0 server, in process, for selftest.py: the suite
runs against it and must pass. It is built from spec/v0/ and from the
vectors (its keys are vectors/v0/keys.json's "a", "a2", "a3"; its
playlists, subtitles and PGS come from vectors/v0/media.json), and does
only what the suite needs. Test code: never a server for real use."""

import hashlib
import hmac
import http.client
import http.server
import json
import os
import re
import socket
import struct
import ssl
import threading
import time
import urllib.parse

from . import ed25519
from .tls import Certificate
from .wire import (CODE_PREFIX, VERSION, FED, H_DATE, H_KEY, H_NONCE, H_SIG, Answer, Key, NotIJSON, answer_message, b64u,
                   check_address, check_destination, check_uncallable_address, clean_name, commitment, decimal,
                   dumps, has_null, host_of, is_int, make_code, make_revocation, make_rotation,
                   parse_ijson, proof, read_pins, read_revocation, read_rotation, request_message,
                   split_hostport, spki_pin, unb64u)
from .copies import check_previews_request, media_type, token, video_key
from .media import scan_cap, serves
from .music import check_lyrics_request, check_queue_request, music_key

HERE = os.path.dirname(os.path.abspath(__file__))
VEC = os.path.join(HERE, "..", "..", "vectors", "v0")
PARTS = {"catalog": [VERSION], "playback": [VERSION], "media": [VERSION], "video": [VERSION], "music": [VERSION],
         "groups": [VERSION]}
NEEDS = {"catalog": (), "media": (), "groups": (), "playback": ("catalog", "media"),
         "video": ("catalog", "playback", "media"), "music": ("catalog", "playback", "media")}
JPEG = (b"\xff\xd8\xff\xe0\x00\x10JFIF\x00\x01\x01\x00\x00\x01\x00\x01\x00\x00"
        b"\xff\xc0\x00\x11\x08\x00\x5a\x00\xa0\x03\x01\x22\x00\x02\x11\x01\x03\x11\x01\xff\xd9")


PICTURES = ("hdmv_pgs_subtitle", "dvd_subtitle", "dvb_subtitle")     # video 5.4's table
TEXTS = ("subrip", "webvtt", "mov_text", "ttml", "ass", "ssa")


def problem(status, typ, title, **extra):
    d = {"type": typ, "title": title, "status": status}
    d.update(extra)
    return dumps(d) + b"\n"


NF = problem(404, "not-found", "Not found")
NS = problem(404, "not-supported", "Not supported")
BAD = problem(400, "invalid-request", "Invalid request")


class Peer:
    def __init__(self, key, address, name):
        self.key, self.address, self.name = key, address, name
        self.state = "ours"
        self.was_paired = False
        self.commitment = None
        self.prev = None                # (key, until)
        self.pending = None
        self.suspended = False
        self.history = [key]
        self.inbound = True
        self.confirmed_inbound = True
        self.pins = []
        self.versions = (None, None)
        self.nonces = set()
        self.sign_key = None            # the server's key toward this peer
        self.push = None                # (old, new, nc): a rotate to send
        self.pull = None                # (old, new, nc): a rotation pulled by hello
        self.revoked = None             # a revoked Key, carried in hello answers
        self.calls = []                 # (time, bucket)
        self.local = True               # its local titles shared with this peer (catalog 5)
        self.offer_seq = 0              # the seq of the last change of what it is offered
        self.downloads = []             # when its downloads started (playback 7)
        self.scans = []                 # when its scans started (playback 8)


class FakeServer:
    def __init__(self, resolve, allow_http=False, faults=()):
        self.faults = set(faults)       # deliberate faults, for the self-test's second pass
        keys = json.load(open(os.path.join(VEC, "keys.json")))["keys"]
        self.media = json.load(open(os.path.join(VEC, "media.json")))
        self.key = Key(bytes.fromhex(keys["a"]["seed"]))
        self.next = [Key(bytes.fromhex(keys["a2"]["seed"])), Key(bytes.fromhex(keys["a3"]["seed"]))] + [Key() for _ in range(4)]
        self.resolve = resolve          # names the operator lets this server reach, -> IP
        self.allow_http = allow_http
        self.lock = threading.RLock()
        self.codes = {}                 # secret hash -> (secret, expiry)
        self.peers = {}                 # key bytes -> Peer
        self.forgotten = set()
        self.started = time.time() - 2
        self.seen = set()               # (key, nonce) accepted
        self.inflight = {}              # claimed key -> requests in flight (core 8.3 item 3)
        self.playbacks = {}             # id -> dict
        self.tickets = {}               # ticket -> id
        self.seq = 1
        self.log = []                   # (seq, key, "add"/"gone")
        sock = socket.socket()
        sock.bind(("127.0.0.1", 0))
        self.port = sock.getsockname()[1]
        sock.close()
        self.address = "%s://server.example.org:%d" % ("http" if allow_http else "https", self.port)
        self.host = host_of(self.address)
        self.cert = None if allow_http else Certificate(["server.example.org"])
        self.libraries = [{"id": "Lq3vB9xYpW2kT7mN4rS6aC", "name": "Films"},
                          {"id": "Ze8hJ1uQ5dF0gK3lP7oV2w", "name": "Séries"},
                          {"id": "Mu5c0R4nH8sT2vX6yB9eQa", "name": "Music"}]
        L0, L1, L2 = (x["id"] for x in self.libraries)
        FILE = 4 * 1024 * 1024 + 123

        def copy(cid, height, etag, audio=("en",), subs=(), previews=False, parts=None, duration=7200000, video=True):
            """A copy as video 3.2 to 3.8 say: H.264 High 4.0, 8-bit SDR in
            BT.709, AAC-LC (the default track 5.1, the others stereo), its
            track IDs its streams' indexes in the file (video 3.7)."""
            c = {"id": cid, "durationMs": duration + 417, "mediaType": "video/mp4" if video else "audio/mp4",
                 "size": FILE, "etag": etag}
            if parts:
                c["parts"] = 2
            if video:
                c["video"] = {"codec": "h264", "codecs": "avc1.640028", "width": height * 16 // 9, "height": height,
                              "bitDepth": 8, "colour": {"primaries": 1, "transfer": 1, "matrix": 1}, "fps": "24000/1001",
                              "refFrames": 4}
            first = 1 if video else 0
            c["audioTracks"] = [dict({"id": "a%d" % (first + i), "language": lang, "channels": 6 if i == 0 else 2,
                                      "codec": "aac", "codecs": "mp4a.40.2", "sampleRate": 48000},
                                     **({"default": True} if i == 0 else {}))
                                for i, lang in enumerate(audio)]
            if subs:
                n = first + len(audio)
                c["subtitleTracks"] = [dict({"id": "s%d" % (n + i), "language": lang, "format": fmt},
                                            **({"role": "subtitle"} if i == 0 else {}))
                                       for i, (lang, fmt) in enumerate(subs)]
            if previews:
                c["previewLayout"] = dict(self.TRICKPLAY)
            if "nocodecs" in self.faults and cid == "v_Pz4nW7":
                del c["video"]["codecs"]
            return c
        film = [copy("v_Kd8sQ2", 1080, "f550a", ("en", "fr"), (("en", "subrip"), ("fr", "hdmv_pgs_subtitle"), ("en", "ass")),
                     True),
                copy("v_Pz4nW7", 720, "f550b", ("en", "fr")),
                copy("v_Mp3aR9", 480, "f550c", parts=True)]
        self.titles = {
            "movie/tmdb/550": {"kind": "movie", "library": L0,
                               **({"audio": ["en", "fr"]} if "summary" in self.faults else {}), "copies": film},
            "movie/tmdb/550/x/Tr7aQ1": {"kind": "extra", "library": L0, "extraType": "trailer",
                                        "copies": [copy("x_1", 1080, "x550", duration=120000)]},
            "movie/tmdb/550/x/Th9sW2": {"kind": "extra", "library": L0, "extraType": "themesong",
                                        "copies": [copy("x_2", 0, "x550t", duration=60000, video=False)]},
            "movie/tmdb/603": {"kind": "movie", "library": L0, "copies": [
                dict({k: v for k, v in copy("v_Mx1", 1080, "f603").items() if k != "mediaType"}, container="matroska")
                if "container" in self.faults else copy("v_Mx1", 1080, "f603")]},
            "movie/local/q3Zp0Lw8": {"kind": "movie", "library": L0, "name": "Summer 2019", "year": 2019, "image": True,
                                     "local": True, "copies": [copy("h_1", 720, "loc1", duration=600000)]},
            "series/tmdb/1399": {"kind": "series", "library": L1},
            "series/tmdb/1399/s1": {"kind": "season", "library": L1},
            "series/tmdb/1399/e63056": {"kind": "episode", "library": L1, "place": {"season": 1, "episode": 1},
                                        "ids": {"tvdb": "3254641"},
                                        "with": ["series/tmdb/1399/e63057"] if "with" not in self.faults else ["series/tmdb/1400/e1"],
                                        "copies": [dict(copy("e_1", 1080, "ep12", duration=3600000), endMs=1800000)]},
            "series/tmdb/1399/e63057": {"kind": "episode", "library": L1, "place": {"season": 1, "episode": 2},
                                        "with": ["series/tmdb/1399/e63056"],
                                        "copies": [dict(copy("e_1", 1080, "ep12", duration=3600000), startMs=1800000)]},
            "album/mb/0b000000-0000-4000-8000-000000000002": {"kind": "album", "library": L2, "name": "Bridge over Troubled Water",
                                                             "artists": ["artist/mb/0a000000-0000-4000-8000-000000000001"],
                                                             "artistNames": "Simon & Garfunkel", "primaryType": "album",
                                                             "secondaryTypes": ["compilation"],
                                                             "ids": {"mbreleasegroup": "0b000000-0000-4000-8000-0000000000ff"}},
            "artist/mb/0a000000-0000-4000-8000-000000000001": {"kind": "artist", "library": L2, "name": "Simon & Garfunkel"},
        }
        for n, (name, lyr) in enumerate((("Bridge over Troubled Water", None), ("The Boxer", "synced"))):
            c = {"id": "t_%d" % n, "durationMs": 308000, "mediaType": "audio/flac", "codec": "flac", "size": 2000000 + n,
                 "etag": "tr%d" % n, "sampleRate": 44100, "bitDepth": 16, "channels": 2}
            if lyr:
                c["lyrics"] = lyr
            self.titles["track/mb/0c000000-0000-4000-8000-00000000000%d" % (n + 3)] = {
                "kind": "track", "library": L2, "name": name, "album": "album/mb/0b000000-0000-4000-8000-000000000002",
                "disc": 1, "number": n + 1, "copies": [c]}
        self.changes = {k: 1 for k in self.titles}
        self.limits = {"streams": 2, "transcodes": 1, "bitrateKbps": 0, "downloads": 1, "downloadsPerDay": 50,
                       "downloadKbps": 2000, "scansPerDay": 100}
        self.scan_bytes = 4 * 1024 * 1024 + 64      # the pool's scan cap, stated in each scan's answer (playback 8)
        self.peer_rate = (200.0, 400)   # calls a second, burst
        self.group = None               # the one group this server founded (make_group_code)
        self.httpd = self._serve()

    # ---- codes and operator actions ---------------------------------------------

    def make_code(self):
        secret = os.urandom(32)
        expiry = int(time.time()) + 86400
        with self.lock:
            self.codes[hashlib.sha256(secret).digest()] = (secret, expiry)
        return make_code(self.key.pub, secret, expiry, self.address)

    # ---- the group (groups 2, 3, 8): one, founded by this server -----------------------

    def make_group_code(self, name="Fake group"):
        """Founds the group (once) and gives a code for it."""
        from .groups import EVENT_LABEL
        with self.lock:
            if self.group is None:
                p = {"type": "founded", "signer": self.key.text, "member": self.key.text, "seq": 1,
                     "date": int(time.time()), "name": name, "settings": {"waitDays": 3, "capPerDay": 5},
                     "nonce": b64u(os.urandom(16)), "ownerSeq": 1}
                raw = dumps(p)
                gid = b64u(hashlib.sha256(raw).digest())
                self.group = {"id": gid, "name": name, "events": {gid: (p, {"event": b64u(raw), "signature": b64u(
                    self.key.sign(EVENT_LABEL + raw))})}, "members": {self.key.text}, "joining": {}, "seq": 1,
                    "codes": {}, "ownerSeq": 1, "admins": set(), "x": os.urandom(32), "profiles": {}, "mail": {},
                    "answers": {}, "answered": set()}
                from . import hpke
                self._group_sign("profile", name="Fake server", encryptionKey=b64u(hpke.x25519_public(self.group["x"])),
                                 nextCommitment=self.next[0].commitment)
            g = self.group
            secret = os.urandom(32)
            expiry = int(time.time()) + 86400
            h = hashlib.sha256(secret).digest()
            self.codes[h] = (secret, expiry)
            g["codes"][h] = 2           # uses (groups 2)
            nb = name.encode()
            return CODE_PREFIX + b64u(self.key.pub + secret + struct.pack(">Q", expiry) + b"\x01" + unb64u(g["id"])
                                      + bytes([len(nb)]) + nb + self.address.encode())

    def _group_sign(self, typ, **extra):
        from .groups import EVENT_LABEL
        g = self.group
        g["seq"] += 1
        p = {"group": g["id"], "type": typ, "signer": self.key.text, "member": self.key.text, "seq": g["seq"],
             "date": int(time.time())}
        p.update(extra)
        raw = dumps(p)
        eid = b64u(hashlib.sha256(raw).digest())
        g["events"][eid] = (p, {"event": b64u(raw), "signature": b64u(self.key.sign(EVENT_LABEL + raw))})

    def op_join_accept(self):
        with self.lock:
            g = self.group
            waiting = [] if g is None else list(g["joining"].items())
            for key, peer in waiting:
                del g["joining"][key]
                self._group_sign("joined", newMember=b64u(key), name=peer.name or "a member")
                g["members"].add(b64u(key))
            envs = [e for _, e in g["events"].values()] if g else []
        for key, peer in waiting:
            self.call(peer, "group-push", {"group": g["id"], "events": envs, "mail": []})

    def _paired_members(self):
        return [p for p in self.peers.values() if p.state == "paired" and b64u(p.key) in self.group["members"]]

    def op_admin(self):
        with self.lock:
            g = self.group
            names = [m for m in g["members"] if m != self.key.text and m not in g["admins"]]
            for m in names:
                g["ownerSeq"] += 1
                self._group_sign("admin", target=m, on=True, ownerSeq=g["ownerSeq"])
                g["admins"].add(m)
            envs = [e for _, e in g["events"].values()]
            peers = self._paired_members()
        for p in peers:
            self.call(p, "group-push", {"group": g["id"], "events": envs, "mail": []})

    def op_request_accept(self):
        """Accepts the requests to this server: to one that can't be called, a
        sealed answer with this server's address (groups 4.2, 5)."""
        from . import hpke
        from .groups import MAIL_LABEL
        with self.lock:
            g = self.group
            out = []
            for i, (mp, _) in list(g["mail"].items()):
                if mp.get("kind") != "request" or mp.get("to") != self.key.text or i in g["answered"]:
                    continue
                g["answered"].add(i)
                try:
                    inside = json.loads(hpke.open_(g["x"], b"msfp-group-mail-1" + unb64u(g["id"]) + unb64u(i),
                                                   unb64u(mp["from"]) + self.key.pub, unb64u(mp["sealed"])))
                except (ValueError, TypeError, KeyError):
                    continue
                to_x = unb64u(g["profiles"].get(mp["from"]) or "", 32)
                if inside.get("inbound") is not False or to_x is None:
                    continue
                aid = os.urandom(16)
                body = {"address": self.address, "nextCommitment": self.next[0].commitment, "inbound": True}
                if "groupname" in self.faults:
                    body["name"] = "Fake server"
                if self.cert:
                    body["certificates"] = [self.cert.pin]
                sealed = hpke.seal(to_x, b"msfp-group-mail-1" + unb64u(g["id"]) + aid,
                                   self.key.pub + unb64u(mp["from"]), dumps(body))
                ap = {"group": g["id"], "id": b64u(aid), "kind": "answer", "from": self.key.text, "to": mp["from"],
                      "answers": i, "expires": int(time.time()) + 86400, "sealed": b64u(sealed)}
                raw = dumps(ap)
                env = {"mail": b64u(raw), "signature": b64u(self.key.sign(MAIL_LABEL + raw))}
                g["mail"][ap["id"]] = (ap, env)
                g["answers"][ap["id"]] = [mp["from"], False]
                out.append(env)
            peers = self._paired_members()
        for p in peers:
            self.call(p, "group-push", {"group": g["id"], "events": [], "mail": out})

    def group_pair(self, key, j):
        """Groups 8.3, from a member this server isn't paired with yet."""
        g = self.group
        if not isinstance(j, dict) or has_null(j) or g is None or j.get("group") != g["id"]:
            return None, b"", None
        a = g["answers"].get(j.get("answers"))
        if a is None or a[0] != b64u(key) or not isinstance(j.get("address"), str) \
                or unb64u(j.get("nextCommitment") or "", 32) is None:
            return None, b"", None
        if a[1]:
            return 409, b"", self.key
        a[1] = True
        p = Peer(key, j["address"], "a member")
        p.state, p.was_paired = "paired", True
        p.commitment = j["nextCommitment"]
        p.inbound = j.get("inbound") is not False
        p.confirmed_inbound = p.inbound
        self.peers[key] = p
        out = {"nextCommitment": self.next[0].commitment, "inbound": True}
        if "groupname" in self.faults:
            out["name"] = "Fake server"
        return 200, dumps(out), self.key

    def _have(self):
        have = {}
        for p, _ in self.group["events"].values():
            have[p["member"]] = max(have.get(p["member"], 0), p["seq"])
        return have

    def group_call(self, call, key, peer, j, sk):
        """group-join, group-sync, group-push (groups 8)."""
        from .groups import GroupFault, join_proof, read_envelope
        g = self.group
        if not isinstance(j, dict) or has_null(j):
            return None, b"", None
        if call == "group-join":
            h = unb64u(j.get("secretHash") or "", 32)
            if g is None or h is None or h not in g["codes"] or h not in self.codes \
                    or self.codes[h][1] < time.time():
                return 404, NF, sk
            if not hmac.compare_digest(j.get("proof") or "", join_proof(self.codes[h][0], key)):
                return 404, NF, sk
            if b64u(key) in g["members"] or key in g["joining"]:
                return 409, b"", sk
            g["codes"][h] -= 1
            if not g["codes"][h]:
                del self.codes[h], g["codes"][h]
            g["joining"][key] = peer
            return 200, b"{}", sk
        # Heard on group-sync and group-push: a member, in this copy (groups 8).
        if g is None or j.get("group") != g["id"] or b64u(key) not in g["members"]:
            return None, b"", None
        if call == "group-sync":
            have = j.get("have") if isinstance(j.get("have"), dict) else {}
            out = sorted((((p["member"], p["seq"]), e) for p, e in g["events"].values()
                          if p["seq"] > (have.get(p["member"]) if is_int(have.get(p["member"])) else 0)),
                         key=lambda x: x[0])     # an equivocation has two of one (member, seq)
            known = set(j.get("mail") or []) if isinstance(j.get("mail"), list) else set()
            mail = [m for i, (_, m) in g["mail"].items() if i not in known]
            return 200, dumps({"events": [e for _, e in out], "mail": mail, "have": self._have(), "more": False}), sk
        for env in (j.get("events") or [])[:500]:
            try:
                eid, p, _ = read_envelope(env, g["id"])
            except GroupFault:
                if "groupforged" not in self.faults:
                    continue
                eid, p = b64u(os.urandom(32)), {}
                try:
                    p = json.loads(unb64u(env["event"]))
                except (ValueError, TypeError, KeyError):
                    continue
            if p.get("date", 0) > time.time() + 300 or p.get("signer") != p.get("member"):
                continue
            g["events"][eid] = (p, env)
            if p.get("type") == "left":
                g["members"].discard(p["member"])
            if p.get("type") == "joined" and p["member"] in g["admins"] and isinstance(p.get("newMember"), str):
                g["members"].add(p["newMember"])
            if p.get("type") == "profile" and isinstance(p.get("encryptionKey"), str):
                g["profiles"][p["member"]] = p["encryptionKey"]
        from .groups import MAIL_LABEL
        for m in (j.get("mail") or [])[:100]:
            try:
                raw = unb64u(m["mail"])
                mp = json.loads(raw)
                ok = ed25519.verify_strict(unb64u(mp["from"], 32), MAIL_LABEL + raw, unb64u(m["signature"], 64) or b"") \
                    or "mailforged" in self.faults
            except (TypeError, ValueError, KeyError):
                continue
            if ok and mp["from"] in g["members"] and mp.get("to") in g["members"] and mp.get("expires", 0) > time.time():
                g["mail"][mp["id"]] = (mp, m)
        return 200, b"{}", sk

    def operator(self, action):
        action = action.replace("-", "_")
        getattr(self, "op_" + action.replace("2", "_again") if action.endswith("2") else "op_" + action)()

    def op_accept(self, want_inbound=True):
        with self.lock:
            ps = [p for p in self.peers.values() if p.state == "ours" and p.inbound == want_inbound]
        for p in ps:
            with self.lock:
                p.state, p.was_paired = "paired", True
            if p.inbound:
                self.call(p, "hello", self.hello_body(p, "paired"))

    def op_accept_again(self):
        self.op_accept(want_inbound=False)

    def op_want(self):
        for p in list(self.peers.values()):
            if p.state != "paired" or not p.inbound:
                continue
            a = self.call(p, "libraries", {})
            libs = [x["id"] for x in a.json.get("libraries", [])] if a and a.status == 200 else []
            after = ""
            for _ in range(10):
                a = self.call(p, "titles", {"libraries": libs, "after": after})
                if not a or a.status != 200:
                    break
                if "prefetch" in self.faults:
                    # Proposal 16's way, gone: every copy's previews ahead.
                    for t in a.json.get("titles") or []:
                        for c in t.get("copies") or []:
                            if isinstance(c, dict) and c.get("previewLayout"):
                                self.call(p, "previews", {"title": t["key"], "versionId": c["id"]})
                nxt = a.json.get("next")
                if not isinstance(nxt, str) or not nxt or nxt <= after:
                    break
                after = nxt

    watch_seconds = 30

    def op_watch(self):
        """A viewer of the suite's film Heat: play, paused at once, reported
        paused every 10 seconds, stopped after watch_seconds."""
        for p in list(self.peers.values()):
            if p.state != "paired" or not p.inbound:
                continue
            # The suite's copy is H.264 and AAC in MP4, which this viewer
            # plays as it is: it asks for the file (playback 2.7).
            a = self.call(p, "play", {"title": "movie/tmdb/949", "delivery": "file"})
            if not a or a.status != 200 or not isinstance(a.json.get("id"), str):
                return
            pid = a.json["id"]
            if "quietpause" in self.faults:
                # A viewer's server that says nothing while its viewer pauses.
                time.sleep(self.watch_seconds)
            else:
                self.call(p, "progress", {"id": pid, "positionMs": 0})
                end = time.time() + self.watch_seconds
                while time.time() < end:
                    time.sleep(max(0, min(10, end - time.time())))
                    self.call(p, "progress", {"id": pid, "positionMs": 0})
            self.call(p, "stop", {"id": pid, "positionMs": -1 if "badstop" in self.faults else 0})
            return

    def op_confirm(self):
        with self.lock:
            for p in self.peers.values():
                if p.pending:
                    p.pending = ("confirmed", p.pending[1] if isinstance(p.pending, tuple) else p.pending)

    def op_change(self):
        with self.lock:
            self.seq += 1
            self.titles["movie/tmdb/680"] = {"kind": "movie", "library": self.libraries[0]["id"], "copies": [
                {"id": "v_680", "durationMs": 8340000, "mediaType": "video/matroska", "size": 1000000, "etag": "f680",
                 "audioTracks": [{"id": "a0", "language": "en", "codec": "aac", "codecs": "mp4a.40.2"}]}]}
            self.changes["movie/tmdb/680"] = self.seq
            self.titles.pop("movie/tmdb/603", None)
            self.changes["movie/tmdb/603"] = self.seq

    def _local(self, on):
        with self.lock:
            self.seq += 1
            for p in self.peers.values():
                p.local, p.offer_seq = on, self.seq

    def op_local_off(self):
        self._local(False)

    def op_local_on(self):
        self._local(True)

    def _rotate_own(self):
        old, new = self.key, self.next.pop(0)
        nc = self.next[0].commitment
        self.key = new
        return old, new, nc

    def op_rotate(self):
        with self.lock:
            old, new, nc = self._rotate_own()
            todo = []
            for p in self.peers.values():
                p.sign_key = old
                if p.inbound:
                    p.push = (old, new, nc)
                    todo.append(p)
                else:
                    p.pull = (old, new, nc)
        for p in todo:
            self.push_rotation(p)

    def op_rotate_again(self):
        self.op_rotate()

    def op_revoke(self):
        """Revokes the current key and rotates to the next in one action (core 12.7 case 1)."""
        with self.lock:
            k = self.key
            old, new, nc = self._rotate_own()
            for p in self.peers.values():
                p.sign_key = old
                if p.inbound:
                    p.push = (old, new, nc)
                else:
                    p.revoked = k
                    p.pull = (old, new, nc)

    def push_rotation(self, p):
        if not p.push:
            return
        old, new, nc = p.push
        a = self.call(p, "rotate", make_rotation(old, new, nc), key=old)
        if a and a.status == 200 and a.signed:
            with self.lock:
                p.push = None
                if "oldkey" not in self.faults:
                    p.sign_key = None

    # ---- the HTTP server -----------------------------------------------------------------

    def _serve(self):
        srv = self
        ctx = self.cert.context() if self.cert else None

        class Handler(http.server.BaseHTTPRequestHandler):
            protocol_version = "HTTP/1.1"
            timeout = 30

            def log_message(self, *a):
                pass

            def do_POST(self):
                srv.handle_post(self)

            def do_GET(self):
                srv.handle_get(self)

            def do_HEAD(self):
                srv.handle_get(self, head=True)

            def do_PUT(self):
                srv.handle_get(self)

            def do_DELETE(self):
                srv.handle_get(self)

        class Server(http.server.ThreadingHTTPServer):
            daemon_threads = True
            allow_reuse_address = True

            def finish_request(self, request, client_address):
                request.settimeout(30)
                if ctx is not None:
                    try:
                        request = ctx.wrap_socket(request, server_side=True)
                    except (ssl.SSLError, OSError):
                        return
                self.RequestHandlerClass(request, client_address, self)

        httpd = Server(("127.0.0.1", self.port), Handler)
        threading.Thread(target=httpd.serve_forever, daemon=True).start()
        return httpd

    def close(self):
        self.httpd.shutdown()
        self.httpd.server_close()

    def send(self, h, status, body, nonce=None, key=None, ctype=None, extra=()):
        try:
            h.send_response(status)
            mt = ctype or ("application/json" if body and status in (200, 202) else "application/problem+json" if body else "-")
            if mt != "-":
                h.send_header("Content-Type", mt)
            if nonce and key:
                h.send_header(H_SIG, b64u(key.sign(answer_message(nonce, status, mt.split(";")[0], body))))
            for k, v in extra:
                h.send_header(k, v)
            h.send_header("Content-Length", str(len(body)))
            h.end_headers()
            if h.command != "HEAD":
                h.wfile.write(body)
        except OSError:
            pass

    def handle_post(self, h):
        n = int(h.headers.get("Content-Length") or 0)
        body = h.rfile.read(n) if n else b""
        path = h.path
        if path.startswith("/api/v1/stream/"):
            return self.handle_get(h)
        if not path.startswith(FED) or "/" in path[len(FED):]:
            return self.send(h, 404, b"")
        call = path[len(FED):]
        claimed = (h.headers.get("MSFP-Key") or "").strip()
        with self.lock:
            self.inflight[claimed] = self.inflight.get(claimed, 0) + 1
        try:
            self._post(h, call, body, claimed)
        finally:
            with self.lock:
                self.inflight[claimed] -= 1

    def _post(self, h, call, body, claimed):
        with self.lock:
            out = self.check(h, call, body)
            busy = self.inflight.get(claimed, 0) > 8
        if out is None:
            return self.send(h, 404, b"")
        key, nonce, peer, signed404 = out
        if signed404:
            return self.send(h, 404, NF, nonce, self.sign_key(peer))
        if peer is not None and busy:
            return self.send(h, 429, problem(429, "rate-limited", "Too many requests", retryAfter=1), nonce,
                             self.sign_key(peer), extra=[("Retry-After", "1")])
        if peer is not None and not self.rate_ok(peer):
            return self.send(h, 429, problem(429, "rate-limited", "Too many requests", retryAfter=1), nonce,
                             self.sign_key(peer), extra=[("Retry-After", "1")])
        try:
            j = parse_ijson(body)
        except NotIJSON:
            j = None
        with self.lock:
            status, out_body, sign = self.route(call, key, peer, j, h)
        if status is None:
            return self.send(h, 404, b"")
        self.send(h, status, out_body, nonce if sign else None, sign)
        if peer is not None and peer.push and call != "rotate":
            threading.Thread(target=self.push_rotation, args=(peer,), daemon=True).start()

    def sign_key(self, peer):
        if peer is not None and peer.sign_key is not None:
            return peer.sign_key
        return self.key

    def rate_ok(self, peer):
        rate, burst = self.peer_rate
        now = time.time()
        with self.lock:
            peer.calls = [t for t in peer.calls if t > now - burst / rate]
            if len(peer.calls) >= burst:
                return False
            peer.calls.append(now)
        return True

    def check(self, h, call, body):
        """Core 6.4. None: the unsigned 404. Else (key, nonce, peer, signed404)."""
        hs = [k.lower() for k in h.headers.keys()]
        vals = {}
        for name in ("msfp-key", "msfp-date", "msfp-nonce", "msfp-signature"):
            if hs.count(name) != 1:
                return None
            v = h.headers.get(name).strip(" \t")
            if "," in v or re.search(r"\s", v):
                return None
            vals[name] = v
        if "?" in h.path or len(body) > 65536:
            return None
        ct = h.headers.get_all("Content-Type") or []
        if len(ct) != 1 or ct[0].split(";")[0].strip().lower() != "application/json":
            return None
        sfs = h.headers.get("Sec-Fetch-Site")
        if sfs is not None and sfs.strip() not in ("same-origin", "none"):
            return None
        if (h.headers.get("Content-Encoding") or "identity").strip().lower() != "identity":
            return None
        key = unb64u(vals["msfp-key"], 32)
        d = decimal(vals["msfp-date"])
        if key is None or d is None or d == 0 or abs(time.time() - d) > 300 or d < int(self.started):
            return None
        nonce, sig = vals["msfp-nonce"], unb64u(vals["msfp-signature"], 64)
        if unb64u(nonce, 16) is None or sig is None or not ed25519.valid_point(key):
            return None
        if not ed25519.verify_strict(key, request_message("POST", self.host, h.path, vals["msfp-date"], nonce, body), sig):
            return None
        if (key, nonce) in self.seen and "nonce" not in self.faults:
            return None
        peer = self.peer_of(key)
        if peer is None:
            if call == "group-pair" and self.group and b64u(key) in self.group["members"]:
                self.seen.add((key, nonce))
                return key, nonce, None, False
            if call == "pair" and any(e > time.time() for _, e in self.codes.values()):
                self.seen.add((key, nonce))
                return key, nonce, None, False
            if hashlib.sha256(key).digest() in self.forgotten:
                return key, nonce, None, True
            return None
        self.seen.add((key, nonce))
        if peer.suspended and call not in ("rotate", "revoke"):
            return key, nonce, peer, True
        return key, nonce, peer, False

    def peer_of(self, key):
        p = self.peers.get(key)
        if p:
            return p
        for p in self.peers.values():
            if p.prev and p.prev[0] == key and p.prev[1] > time.time():
                return p
        return None

    def parts_on(self, peer):
        core, parts = peer.versions
        if not isinstance(core, list) or VERSION not in [x for x in core[:8] if is_int(x)]:
            return set()
        theirs = parts if isinstance(parts, dict) else {}
        on = set()
        for p in ("catalog", "media", "playback", "video", "music", "groups"):
            v = theirs.get(p)
            if isinstance(v, list) and VERSION in [x for x in v[:8] if is_int(x)] and all(n in on for n in NEEDS[p]):
                on.add(p)
        return on

    # ---- the calls ----------------------------------------------------------------------------

    def route(self, call, key, peer, j, h):
        sk = self.sign_key(peer)
        if call == "pair":
            return self.pair(key, peer, j)
        if call == "group-pair":
            return self.group_pair(key, j)
        if peer is None:
            return None, b"", None
        if call == "hello":
            return self.hello(key, peer, j)
        if call == "rotate":
            return self.rotate(key, peer, j)
        if call == "revoke":
            if not isinstance(j, dict):
                return 400, BAD, sk
            try:
                read_revocation(key, j)
            except ValueError:
                return 400, BAD, sk
            peer.suspended = True
            return 200, dumps({"state": "suspended"}), sk
        if peer.state != "paired":
            return 404, NF, sk
        on = self.parts_on(peer)
        if call in ("group-join", "group-sync", "group-push"):
            if "groups" not in on:
                return (404, NS, sk) if call == "group-join" else (None, b"", None)
            return self.group_call(call, key, peer, j, sk)
        if call in ("libraries", "titles"):
            if "catalog" not in on:
                return 404, NS, sk
            if not isinstance(j, dict) or has_null(j):
                return 404, NF, sk
            if call == "libraries":
                out = {"name": "Fake server", "libraries": self.libraries}
                if "playback" in on:
                    out["limits"] = self.limits
                return 200, dumps(out), sk
            return self.titles_call(peer, j, on, sk)
        if call in ("play", "progress", "stop", "previews", "lyrics"):
            if "playback" not in on:
                return 404, NS, sk
            if call in ("previews", "lyrics") and ("video" if call == "previews" else "music") not in on:
                return 404, NS, sk
            if not isinstance(j, dict) or has_null(j):
                return 400, BAD, sk
            return getattr(self, call)(peer, j, on, sk)
        if call == "image":
            if "catalog" not in on:
                return 404, NS, sk
            return self.image(peer, j, on, sk)
        return None, b"", None

    def pair(self, key, peer, j):
        if not isinstance(j, dict) or has_null(j):
            return (404, NF, self.sign_key(peer)) if peer else (None, b"", None)
        addr, name, sh, pf = j.get("address"), j.get("name", ""), j.get("secretHash"), j.get("proof")
        nc, inbound = j.get("nextCommitment"), j.get("inbound", True)
        refuse = (404, NF, self.sign_key(peer)) if peer else (None, b"", None)
        if not isinstance(addr, str) or check_address(addr, self.allow_http) or not isinstance(name, str):
            return refuse
        if unb64u(sh, 32) is None or unb64u(pf, 32) is None or ("nextCommitment" in j and unb64u(nc, 32) is None):
            return refuse
        if not isinstance(inbound, bool) or (inbound is False and check_uncallable_address(addr)):
            return refuse
        code = self.codes.get(unb64u(sh, 32))
        if not code or code[1] <= time.time():
            return refuse
        if not hmac.compare_digest(proof(code[0], key, addr), unb64u(pf, 32)):
            return refuse
        if peer is not None:
            return 409, b"", self.sign_key(peer)
        del self.codes[unb64u(sh, 32)]
        self.forgotten.discard(hashlib.sha256(key).digest())
        p = Peer(key, addr, clean_name(name) or host_of(addr))
        p.commitment, p.inbound = nc, inbound
        self.peers[key] = p
        return 200, dumps({"name": "Fake server", "nextCommitment": self.next[0].commitment}), self.key

    def hello_body(self, peer, state):
        if state == "ended":
            return {"state": "ended"}
        b = {"state": state, "core": [VERSION], "parts": PARTS}
        if self.cert:
            b["certificates"] = [self.cert.pin]
        b["address"] = self.address
        return b

    def hello(self, key, peer, j):
        sk = self.sign_key(peer)
        if not isinstance(j, dict) or not isinstance(j.get("state"), str):
            return 404, NF, sk
        if "null" in self.faults:
            j = {k: v for k, v in j.items() if v is not None}
        types = {"core": list, "parts": dict, "certificates": list, "address": str, "inbound": bool, "nextCommitment": str}
        for f, t in types.items():
            if f in j and not isinstance(j[f], t):
                return 404, NF, sk
        if isinstance(j.get("parts"), dict):
            for n in PARTS:
                if n in j["parts"] and not isinstance(j["parts"][n], list):
                    return 404, NF, sk
        uncallable = j.get("inbound") is False
        if uncallable:
            for f in ("knownKey", "rotation"):
                if f in j and not isinstance(j[f], str):
                    return 404, NF, sk
            if check_uncallable_address(j.get("address", peer.address)):
                return 404, NF, sk
        st = j["state"]
        if st == "ended":
            self.peers.pop(peer.key, None)
            return 200, dumps({"state": "ended"}), sk
        if st == "paired" and (peer.state == "theirs" or (peer.state == "gone" and peer.was_paired)):
            peer.state = "paired"
        peer.versions = (j.get("core"), j.get("parts"))
        peer.pins = read_pins(j.get("certificates"))
        was = peer.inbound
        peer.inbound = j.get("inbound", True)
        addr = j.get("address")
        if not peer.inbound:
            if isinstance(addr, str):
                peer.address = addr
            peer.confirmed_inbound = False
        elif (isinstance(addr, str) and addr != peer.address) or (not was):
            threading.Thread(target=self.confirm, args=(peer, addr if isinstance(addr, str) else peer.address), daemon=True).start()
        out = self.hello_body(peer, "paired" if peer.state == "paired" else "waiting")
        if uncallable:
            known = unb64u(j.get("knownKey"), 32)
            if peer.pull and known == peer.pull[1].pub:
                peer.pull, peer.sign_key, peer.revoked = None, None, None
                sk = self.key
            elif peer.pull:
                old, new, nc = peer.pull
                out["rotation"] = make_rotation(old, new, nc)
                sk = old
            if peer.revoked is not None and sk is peer.revoked:
                out["revocation"] = make_revocation(peer.revoked)
        return 200, dumps(out), sk

    def confirm(self, peer, addr):
        """Core 13.2 step 2 (and core 7.5, inbound back to true): a hello at the new address."""
        if check_address(addr, self.allow_http):
            return
        time.sleep(0.3)
        old = peer.address
        peer.address = addr
        try:
            self.get(peer, "/.well-known/msfp")
            a = self.call(peer, "hello", self.hello_body(peer, "paired" if peer.state == "paired" else "waiting"), confirming=True)
        except OSError:
            a = None
        with self.lock:
            if a is not None and a.signed and a.status == 200:
                peer.confirmed_inbound = True
                if addr != old:
                    peer.pins = read_pins(a.json.get("certificates")) if isinstance(a.json, dict) else []
            else:
                peer.address = old

    def rotate(self, key, peer, j):
        sk = self.sign_key(peer)
        if not isinstance(j, dict):
            return 400, BAD, sk
        if key != peer.key:
            if peer.prev and key == peer.prev[0] and j.get("newKey") == b64u(peer.key):
                try:
                    read_rotation(key, j)
                    return 200, dumps({"state": "rotated"}), sk
                except ValueError:
                    return 400, BAD, sk
            return 409, problem(409, "rotation-refused", "Rotation refused"), sk
        try:
            new, nc = read_rotation(key, j)
        except ValueError:
            return 400, BAD, sk
        if new in self.peers or hashlib.sha256(new).digest() in self.forgotten:
            return 409, problem(409, "rotation-refused", "Rotation refused"), sk
        match = peer.commitment is not None and commitment(new) == peer.commitment
        if not match and (peer.suspended or peer.commitment is not None):
            return 409, problem(409, "rotation-refused", "Rotation refused"), sk
        if not match:
            if peer.pending == ("confirmed", new):
                pass
            else:
                if not (isinstance(peer.pending, bytes) and peer.pending == new):
                    peer.pending = new
                return 202, dumps({"state": "pending"}), sk
        del self.peers[peer.key]
        peer.prev = (peer.key, time.time() + 600)
        peer.key, peer.commitment, peer.pending, peer.suspended = new, nc, None, False
        peer.history.append(new)
        self.peers[new] = peer
        return 200, dumps({"state": "rotated"}), sk

    def titles_call(self, peer, j, on, sk):
        libs, after, since = j.get("libraries"), j.get("after", ""), j.get("since", 0)
        if not isinstance(libs, list) or any(not isinstance(x, str) for x in libs):
            return 404, NF, sk
        if not isinstance(after, str) or len(after.encode()) > 512 or not is_int(since):
            return 404, NF, sk
        offered = {x["id"] for x in self.libraries}

        def shown(k, t):
            part = "video" if video_key(k) else "music"
            return part in on and t["library"] in libs and t["library"] in offered and (peer.local or not t.get("local"))

        if (0 < since <= self.seq and since >= peer.offer_seq) or ("since" in self.faults and since > self.seq):
            keys = sorted((k for k, s in self.changes.items() if s > since), key=str.encode)
            titles = [self._shown(dict(self.titles[k], key=k)) for k in keys if k in self.titles and shown(k, self.titles[k])]
            gone = [k for k in keys if k not in self.titles]
            return 200, dumps({"titles": titles, "gone": gone, "changes": True, "seq": self.seq}), sk
        keys = sorted((k for k, t in self.titles.items() if shown(k, t) and k.encode() > after.encode()), key=str.encode)
        page = keys[:5000]
        out = {"titles": [self._shown(dict(self.titles[k], key=k)) for k in page], "seq": self.seq}
        if len(page) == 5000 and len(keys) > 5000:
            out["next"] = page[-1]
        return 200, dumps(out), sk

    @staticmethod
    def _shown(t):
        """A title as titles sends it: no kind (catalog 3: the key names it)."""
        return dict([("key", t["key"]), ("library", t["library"])] +
                    [(k, v) for k, v in t.items() if k not in ("key", "kind", "library", "local")])

    # ---- playback (playback 2-8, video 5, music 5) ----------------------------------------------

    def _copies(self, key):
        t = self.titles.get(key)
        return t.get("copies", []) if t else []

    def play_refusal(self, j, on):
        """Playback 2.2 step 1, 2.7, video 5.1, music 5.2: None, or why it is
        a 400. Core 8.5: a value that isn't a token (or a media type) is
        malformed; an unknown well-formed one is a 409 later (step 7)."""
        t = j.get("title")
        if not isinstance(t, str):
            return "title"
        vk, mk = video_key(t), music_key(t)
        if not ((vk and vk.kind in ("movie", "episode", "extra") and "video" in on) or (mk == "track" and "music" in on)):
            return "a key play doesn't take"
        purpose, delivery = j.get("purpose", "play"), j.get("delivery")
        if purpose not in ("play", "download", "scan") or ("delivery" in j and token(delivery) is None):
            return "purpose or delivery"
        if "tokens" in self.faults and "delivery" in j and delivery not in ("file", "remux", "transcode"):
            return "an unknown delivery refused as malformed"
        known = delivery in ("file", "remux", "transcode")
        if "mediaType" in j and (delivery != "remux" or media_type(j["mediaType"]) is None):
            return "mediaType"
        if delivery == "remux" and "mediaType" not in j:
            return "mediaType"
        tr = j.get("transcode")
        if "transcode" in j:
            if delivery != "transcode" or not isinstance(tr, dict):
                return "transcode"
            for f, ok in (("videoCodec", lambda v: token(v) is not None), ("audioCodec", lambda v: token(v) is not None),
                          ("range", lambda v: token(v) is not None),
                          ("maxHeight", lambda v: is_int(v) and v in (480, 720, 1080, 1440, 2160, 4320)),
                          ("maxBitrateKbps", lambda v: is_int(v) and 1000 <= v <= 10000000 and v % 1000 == 0),
                          ("audioChannels", lambda v: is_int(v) and v in (2, 6, 8)),
                          ("burnSubtitle", lambda v: isinstance(v, str))):
                if f in tr and not ok(tr[f]):
                    return "transcode." + f
        if purpose == "download" and (known or delivery is None) and not (
                delivery == "file" or (delivery == "transcode" and isinstance(tr, dict)
                                       and ("maxHeight" in tr or "maxBitrateKbps" in tr))):
            return "download's delivery"
        if purpose == "scan" and (known or delivery is None) and delivery != "file":
            return "scan's delivery"
        if "audioRenditions" in j and not isinstance(j["audioRenditions"], bool):
            return "audioRenditions"
        for f, hi in (("startPositionMs", 8640000000), ("maxBitrateKbps", 10000000)):
            if f in j and not (is_int(j[f]) and 0 <= j[f] <= hi):
                return f
        if "versionId" in j and not isinstance(j["versionId"], str):
            return "versionId"
        if "queue" in j and check_queue_request(j) and check_queue_request(j).startswith("400"):
            return "queue"
        return None

    @staticmethod
    def _within(cp, track):
        """Playback 2.7: the copy's video and the audio track playing are
        within the baseline (H.264 up to High 4.1, 8-bit, progressive,
        H.273 1/1/1; AAC-LC, at most 2 channels and 48 kHz)."""
        v = cp.get("video")
        if v:
            m = re.fullmatch(r"avc[13]\.([0-9A-Fa-f]{2})[0-9A-Fa-f]{2}([0-9A-Fa-f]{2})", v.get("codecs") or "")
            col = v.get("colour") or {}
            if v.get("codec") != "h264" or not m or int(m.group(1), 16) not in (66, 77, 100) or \
                    not 10 <= int(m.group(2), 16) <= 41 or v.get("bitDepth") != 8 or v.get("interlaced") or \
                    (col.get("primaries"), col.get("transfer"), col.get("matrix")) != (1, 1, 1):
                return False
        return track is None or (track.get("codec") == "aac" and track.get("codecs") == "mp4a.40.2"
                                 and 1 <= track.get("channels", 0) <= 2 and 1 <= track.get("sampleRate", 0) <= 48000)

    @staticmethod
    def _default_audio(cp, asked):
        """Playback 2.6: the track named, else the file's default audio (the
        track marked default, else the first)."""
        tracks = cp.get("audioTracks") or []
        return next((a for a in tracks if a["id"] == asked), None) or next(
            (a for a in tracks if a.get("default")), tracks[0] if tracks else None)

    def play(self, peer, j, on, sk):
        why = self.play_refusal(j, on)
        if why:
            return 400, BAD, sk
        t, purpose, delivery = j["title"], j.get("purpose", "play"), j.get("delivery")
        lim = self.limits
        busy = problem(409, "stream-limit-reached", "Too many streams on this account")
        if {"play": lim["streams"], "download": lim["downloads"], "scan": lim["scansPerDay"]}[purpose] == 0:
            return 409, busy, sk
        copies = self._copies(t)
        if t not in self.titles or not copies or (self.titles[t]["library"] not in {x["id"] for x in self.libraries}):
            return 404, NF, sk
        if self.titles[t].get("local") and not peer.local:
            return 404, NF, sk
        cp = next((c for c in copies if c["id"] == j.get("versionId")), None)
        cp = cp or (copies[-1] if "lastcopy" in self.faults else copies[0])
        tr = j.get("transcode") or {}
        if "audioTrack" in j and not (isinstance(j["audioTrack"], str) and re.fullmatch(r"a(0|[1-9][0-9]{0,2})", j["audioTrack"])):
            return 400, BAD, sk
        if "burnSubtitle" in tr and not re.fullmatch(r"s(0|[1-9][0-9]{0,2})", tr["burnSubtitle"]):
            return 400, BAD, sk
        now = time.time()
        mine = [p for p in self.playbacks.values() if p["peer"] is peer and p["live"]]
        if purpose == "play" and len([p for p in mine if p["purpose"] == "play"]) >= lim["streams"] and "limits" not in self.faults:
            return 409, busy, sk
        if purpose == "download" and (len([p for p in mine if p["purpose"] == "download"]) >= lim["downloads"] or
                                      len([x for x in peer.downloads if x > now - 86400]) >= lim["downloadsPerDay"]):
            return 409, busy, sk
        if purpose == "scan" and len([x for x in peer.scans if x > now - 86400]) >= lim["scansPerDay"]:
            return 409, busy, sk
        refused = problem(409, "delivery-refused", "Delivery refused")
        if delivery == "file" and cp.get("parts", 1) > 1:
            return 409, refused, sk
        subs = cp.get("subtitleTracks") or []
        if "burnSubtitle" in tr and tr["burnSubtitle"] not in [x["id"] for x in subs]:
            return 409, refused, sk
        # Step 7: a delivery, media type, codec, copy or range it doesn't
        # make (core 8.5, playback 2.7).
        if delivery is not None and delivery not in ("file", "remux", "transcode"):
            return 409, refused, sk
        top = "video" if cp.get("video") else "audio"
        if delivery == "remux" and j["mediaType"] not in (top + "/mp2t", top + "/mp4") and "remux" not in self.faults:
            return 409, refused, sk
        if tr.get("videoCodec", "h264") not in ("h264", "hevc", "copy") or \
                tr.get("audioCodec", "aac") not in ("aac", "ac3", "eac3", "copy"):
            return 409, refused, sk
        if tr.get("range", "sdr") not in ("sdr", "keep") and "rangetoken" not in self.faults:
            return 409, refused, sk
        if tr.get("range") not in (None, "sdr", "keep") and "rangetoken" in self.faults:
            return 400, BAD, sk
        if tr.get("videoCodec") == "copy" and "burnSubtitle" in tr and "burncopy" not in self.faults:
            return 409, refused, sk
        audio = self._default_audio(cp, j.get("audioTrack"))
        if "defaultaudio" in self.faults and "audioTrack" not in j and len(cp.get("audioTracks") or []) > 1:
            audio = cp["audioTracks"][-1]
        if delivery:
            method = {"file": "direct", "remux": "remux", "transcode": "transcode"}[delivery]
            if delivery == "transcode" and tr.get("videoCodec") == "copy" and tr.get("audioCodec") == "copy":
                method = "transcode"
        else:
            # The baseline (playback 2.7): left unchanged only when within it.
            method = "remux" if self._within(cp, audio) or "baselinecopy" in self.faults else "transcode"
            if "baseline" in self.faults and "startPositionMs" in j and cp.get("parts", 1) < 2:
                method = "direct"
        takes = method == "transcode" and not (tr.get("videoCodec") == "copy" and tr.get("audioCodec") == "copy")
        if purpose == "play" and takes and \
                sum(1 for p in mine if p.get("takes") and p["purpose"] == "play") >= lim["transcodes"]:
            return 503, problem(503, "server-busy", "The server is busy; try again shortly"), sk
        pid = "play_" + "".join("abcdefghijklmnopqrstuvwxyz234567"[b % 32] for b in os.urandom(26))
        ticket = b64u(os.urandom(24))
        proto = "file" if method == "direct" else "hls"
        out = {"id": pid, "ticket": ticket, "protocol": proto, "method": method,
               "etag": cp["etag"] + ("x" if "etag" in self.faults and purpose == "download" else ""),
               "positionMs": j.get("startPositionMs", 0) if purpose == "play" else 0, "versionId": cp["id"]}
        if "urls" in self.faults:
            out["stream"] = {"url": self.address + "/api/v1/stream/" + ticket + "/" + (
                "file" if proto == "file" else "master.m3u8"), "protocol": proto}
        if audio:
            out["audioTrack"] = audio["id"]
        # Audio renditions when asked and served as HLS (playback 2.3).
        out["audioRenditions"] = proto == "hls" and j.get("audioRenditions") is True
        made = out["audioRenditions"] and "renditions" not in self.faults
        if purpose == "scan":
            out["scanBytes"] = self.scan_bytes
        served, fonts = [], 0
        if video_key(t) and delivery != "file":
            # Video 5.4's table, a must for the formats it lists.
            for x in subs:
                if x["format"] in PICTURES:
                    served.append({"id": x["id"], "form": "picture"})
                elif x["format"] in ("ass", "ssa"):
                    served.append({"id": x["id"], "form": "text", "ass": True})
                elif x["format"] in TEXTS:
                    served.append({"id": x["id"], "form": "text"})
            if served:
                out["subtitles"] = served
            if any(x["format"] in ("ass", "ssa") for x in subs):
                fonts = out["fontCount"] = 1
        queue = []
        for n, e in enumerate(j.get("queue") or []):
            qc = self._copies(e["title"])
            if not qc:
                queue.append(None)
                continue
            qd = e.get("delivery", delivery)
            qm = {"file": "direct", "remux": "remux", "transcode": "transcode"}.get(qd, "remux")
            qcp = next((c for c in qc if c["id"] == e.get("versionId")), qc[0])
            queue.append({"title": e["title"], "versionId": qcp["id"], "method": qm,
                          "protocol": "file" if qm == "direct" else "hls"})
        if j.get("queue") is not None:
            out["queue"] = [e for e in queue if e]
        layout = cp.get("previewLayout")
        self.playbacks[pid] = {"peer": peer, "live": True, "method": method, "takes": takes, "ticket": ticket,
                               "purpose": purpose, "copy": cp, "title": t, "queue": [e["title"] for e in queue if e],
                               "queueProto": [e["protocol"] if e else "" for e in queue], "proto": proto,
                               "renditions": made, "sent": 0, "played": now, "kind": "playback",
                               "subtitles": served, "fontCount": fonts, "scanBytes": out.get("scanBytes"),
                               "sheetCount": -(-layout["tileCount"] // (layout["columns"] * layout["rows"])) if layout else 0}
        if purpose == "download":
            peer.downloads.append(now)
        if purpose == "scan":
            peer.scans.append(now)
        self.tickets[ticket] = pid
        return 200, dumps(out), sk

    def _report_refusal(self, j, call):
        pid = j.get("id")
        if not isinstance(pid, str) or not re.fullmatch(r"[A-Za-z0-9_-]{1,64}", pid):
            return True
        if "positionMs" in j and not (is_int(j["positionMs"]) and 0 <= j["positionMs"] <= 8640000000):
            return True
        now = time.time()
        if "at" in j and not (is_int(j["at"]) and now - 30 * 86400 <= j["at"] <= now + 300):
            return True
        if "title" in j:
            p = self.playbacks.get(pid)
            if not isinstance(j["title"], str) or (p and j["title"] != p["title"] and j["title"] not in p["queue"]):
                return True
        return False

    def progress(self, peer, j, on, sk):
        if self._report_refusal(j, "progress"):
            return 400, BAD, sk
        p = self.playbacks.get(j["id"])
        if p and p["peer"] is peer and not p["live"] and p["purpose"] == "download" and "at" in j \
                and time.time() <= p["played"] + 30 * 86400:
            return 200, b"{}", sk
        if (not p or not p["live"] or p["peer"] is not peer) and "progress" not in self.faults:
            return 410, problem(410, "playback-stopped", "Playback was stopped"), sk
        return 200, b"{}", sk

    def stop(self, peer, j, on, sk):
        if self._report_refusal(j, "stop"):
            return 400, BAD, sk
        p = self.playbacks.get(j["id"])
        if p and p["peer"] is peer:
            p["live"] = False
        return 200, b"{}", sk

    TRICKPLAY = {"tileWidth": 160, "tileHeight": 90, "columns": 10, "rows": 10, "intervalMs": 10000, "tileCount": 50}

    def _ticket(self, peer, kind, **extra):
        ticket = b64u(os.urandom(24))
        pid = "t_" + ticket
        self.playbacks[pid] = dict({"peer": peer, "live": True, "kind": kind, "purpose": kind, "until": time.time() + 600,
                                    "ticket": ticket, "method": None}, **extra)
        self.tickets[ticket] = pid
        return ticket

    def previews(self, peer, j, on, sk):
        if "video" not in on:
            return 404, NS, sk
        if check_previews_request(j):
            return 400, BAD, sk
        cp = next((c for c in self._copies(j["title"]) if c["id"] == j["versionId"]), None)
        if cp is None or not cp.get("previewLayout"):
            return 404, NF, sk
        lay = cp["previewLayout"]
        ticket = self._ticket(peer, "previews", sheetCount=-(-lay["tileCount"] // (lay["columns"] * lay["rows"])))
        return 200, dumps({"ticket": ticket, "etag": cp["etag"]}), sk

    def image(self, peer, j, on, sk):
        if not ({"video", "music"} & on):
            return 404, NS, sk
        if not isinstance(j, dict) or not isinstance(j.get("title"), str):
            return 400, BAD, sk
        t = self.titles.get(j["title"])
        if not t or not t.get("local") or not peer.local or t.get("image") is not True:
            return 404, NF, sk
        return 200, dumps({"ticket": self._ticket(peer, "image")}), sk

    def lyrics(self, peer, j, on, sk):
        if "music" not in on:
            return 404, NS, sk
        if check_lyrics_request(j):
            return 400, BAD, sk
        cp = next((c for c in self._copies(j["title"]) if c["id"] == j["versionId"]), None)
        if cp is None or cp.get("lyrics") not in ("plain", "synced"):
            return 404, NF, sk
        return 200, dumps({"structuredLyrics": [{"lang": "en", "synced": True, "line": [
            {"start": 0, "value": "I am just a poor boy"}, {"start": 4000, "value": ""}]}]}), sk

    # ---- media (media 2, 3, 4; video 6) ----------------------------------------------------------

    def _master(self, p):
        lines = ["#EXTM3U", "#EXT-X-VERSION:7", "#EXT-X-INDEPENDENT-SEGMENTS"]
        audio = p["copy"]["audioTracks"] if p.get("renditions") else []
        for i, a in enumerate(audio):
            lines.append('#EXT-X-MEDIA:TYPE=AUDIO,GROUP-ID="audio",NAME="%s",%sDEFAULT=%s,AUTOSELECT=YES,%sURI="%s.m3u8"' % (
                a["id"], 'LANGUAGE="%s",' % a["language"] if a.get("language") else "", "YES" if i == 0 else "NO",
                'CHANNELS="%d",' % a["channels"] if a.get("channels") else "", a["id"]))
        lines.append('#EXT-X-STREAM-INF:BANDWIDTH=5000000,CODECS="avc1.640028,mp4a.40.2",RESOLUTION=1920x1080%s' % (
            ',AUDIO="audio"' if audio else ""))
        lines.append("v0.m3u8")
        return ("\n".join(lines) + "\n").encode()

    def handle_get(self, h, head=False):
        u = urllib.parse.urlsplit(h.path)
        if not h.path.startswith("/api/v1/stream/"):
            return self.send(h, 404, b"")
        if h.command != "GET":
            if h.command == "HEAD" and "head" in self.faults:
                return self.send(h, 404, b"")
            return self.send(h, 405, b"", extra=[("Allow", "GET")])
        if ((u.query or "?" in h.path) and "query" not in self.faults) or any(h.headers.get(x) for x in ("Origin", "Sec-Fetch-Site", "Sec-Fetch-Mode")):
            return self.send(h, 404, b"")
        parts = u.path[len("/api/v1/stream/"):].split("/")
        with self.lock:
            pid = self.tickets.get(parts[0])
            p = self.playbacks.get(pid) if pid else None
            live = p is not None and p["live"] and (p["kind"] == "playback" or p["until"] > time.time())
        if not live:
            return self.send(h, 404, b"")
        name = "/".join(parts[1:])
        kind, proto = p["kind"], p.get("proto")
        if kind == "playback" and p["purpose"] in ("scan", "download"):
            kind = p["purpose"]
        names = ["v0.m3u8", "init.mp4", "seg0.mp4", "seg-1.mp4"] + ["a%d.m3u8" % i for i in range(41)]
        ticket = {"kind": kind, "protocol": proto, "subtitles": p.get("subtitles", []), "fontCount": p.get("fontCount", 0),
                  "sheetCount": p.get("sheetCount", 0), "queue": p.get("queueProto", []), "names": names}
        why = serves(ticket, name)
        if why and "pastcount" in self.faults and name == "fonts/%d" % ticket["fontCount"] and ticket["fontCount"]:
            why = None      # a built name served past its count (video 6.1: 404)
        if why:
            return self.send(h, 404, b"")
        q = re.fullmatch(r"queue/([0-9]+)/(.+)", name)
        if q:
            name, proto = q.group(2), p["queueProto"][int(q.group(1))]
        vec = {c["name"]: c for c in self.media["hls"]}
        body, ctype, ranged = None, "application/octet-stream", False
        m = re.fullmatch(r"(subtitles|ass|pictures)/(s[0-9]+)/(0|[1-9][0-9]{0,3}|whole)", name)
        if kind in ("image", "previews") or name.startswith("trickplay/"):
            body, ctype = JPEG, "image/jpeg"
        elif name == "master.m3u8":
            body, ctype = self._master(p), "application/vnd.apple.mpegurl"
        elif name.endswith(".m3u8"):
            body, ctype = vec["a media playlist"]["text"].encode(), "application/vnd.apple.mpegurl"
            if "growing" in self.faults:
                body = body.replace(b"#EXT-X-ENDLIST\n", b"")
        elif name in ("init.mp4", "seg0.mp4", "seg-1.mp4"):
            body, ctype, ranged = (bytes(range(256)) * 40)[:4000], "video/mp4", True
        elif name == "file":
            return self._file(h, p)
        elif m and m.group(1) == "subtitles":
            body, ctype = self.media["webvtt"][0]["output"].encode(), "text/vtt; charset=utf-8"
            if "vtt" in self.faults:
                body = self.media["webvtt"][1]["text"].encode()
        elif m and m.group(1) == "ass":
            body, ctype = next(c for c in self.media["ass"] if c["valid"])["output"].encode(), "text/x-ass; charset=utf-8"
        elif m and m.group(1) == "pictures":
            body = bytes.fromhex(self.media["pgs"][0]["hex"])
        elif name.startswith("fonts/"):
            body, ctype = bytes.fromhex(self.media["fonts"][0]["hex"]), "font/ttf"
        if body is None:
            return self.send(h, 404, b"")
        rng = h.headers.get_all("Range") or []
        if ranged and rng:
            got = self._range(rng, len(body))
            if got is None:
                return self.send(h, 416, b"")
            start, end = got
            return self.send(h, 206, body[start:end + 1], ctype=ctype,
                             extra=[("Content-Range", "bytes %d-%d/%d" % (start, end, len(body)))])
        self.send(h, 200, body, ctype=ctype)

    def _range(self, rng, total):
        """Media 2.1: (start, end) of one well-formed, satisfiable Range, or None (416)."""
        if len(rng) != 1:
            return None
        r0 = rng[0].split(",")[0] if "range" in self.faults else rng[0]
        m = re.fullmatch(r"bytes=([0-9]{1,15})-([0-9]{1,15})?|bytes=-([0-9]{1,15})", r0)
        if not m:
            return None
        if m.group(3) is not None:
            cnt = int(m.group(3))
            if cnt < 1 or total == 0:
                return None
            return max(total - cnt, 0), total - 1
        start = int(m.group(1))
        end = int(m.group(2)) if m.group(2) is not None else total - 1
        if (m.group(2) is not None and start > end) or start >= total:
            return None
        return start, min(end, total - 1)

    def _file(self, h, p):
        """Media 3.2: the copy as it is, by ranges, with its ETag; a scan's
        cap (media 4.2) and a download's pace (media 4.1)."""
        cp = p["copy"]
        size, data = cp["size"], self.file_bytes(cp)
        common = [("Accept-Ranges", "bytes"), ("ETag", '"%s"' % cp["etag"])]
        rng = h.headers.get_all("Range") or []
        unsat = [("Content-Range", "bytes */%d" % size)]
        if p["purpose"] == "scan" and not rng:
            return self.send(h, 416, b"", extra=unsat)
        if not rng:
            start, end, status = 0, size - 1, 200
        else:
            got = self._range(rng, size)
            if got is None:
                return self.send(h, 416, b"", extra=unsat)
            (start, end), status = got, 206
        if p["purpose"] == "scan" and "scancap" not in self.faults:
            with self.lock:
                # Its announced scanBytes (media 4.2); the fault scanfloor
                # cuts at the floor instead, as if nothing were announced.
                cap = scan_cap(None if "scanfloor" in self.faults else p.get("scanBytes"))
                left = cap - p["sent"]
                if left <= 0:
                    return self.send(h, 416, b"", extra=unsat)
                end = min(end, start + left - 1)
                p["sent"] += end - start + 1
        extra = common + ([("Content-Range", "bytes %d-%d/%d" % (start, end, size))] if status == 206 else [])
        kbps = self.limits["downloadKbps"] if p["purpose"] == "download" else 0
        if not kbps:
            return self.send(h, status, data[start:end + 1], ctype="video/mp4", extra=extra)
        try:
            h.send_response(status)
            h.send_header("Content-Type", "video/mp4")
            for k, v in extra:
                h.send_header(k, v)
            h.send_header("Content-Length", str(end - start + 1))
            h.end_headers()
            rate, t0, sent = kbps * 125, time.time(), 0
            for i in range(start, end + 1, 32768):
                chunk = data[i:min(i + 32768, end + 1)]
                h.wfile.write(chunk)
                sent += len(chunk)
                wait = t0 + sent / rate - time.time()
                if wait > 0:
                    time.sleep(wait)
        except OSError:
            pass

    _files = {}

    def file_bytes(self, cp):
        key = (cp["etag"], cp["size"])
        if key not in self._files:
            self._files[key] = (bytes(range(251)) * (cp["size"] // 251 + 1))[:cp["size"]]
        return self._files[key]

    # ---- calls to a peer ----------------------------------------------------------------------

    def _connect(self, peer, checked):
        scheme, hp = peer.address.split("://", 1)
        host, port = split_hostport(hp, 443 if scheme == "https" else 80)
        name = host.strip("[]")
        ip = self.resolve.get(name)
        if ip is None:
            try:
                ip = socket.getaddrinfo(name, port)[0][4][0]
            except OSError:
                raise OSError("can't resolve %s" % name)
            if check_destination(ip) and "loopback" not in self.faults:
                raise OSError("refused destination %s (core 3.3)" % ip)
        sock = socket.create_connection((ip, port), 10)
        if scheme == "http":
            c = http.client.HTTPConnection(ip, port, timeout=10)
            c.sock = sock
            return c
        ctx = ssl._create_unverified_context()
        s = ctx.wrap_socket(sock, server_hostname=name if not re.fullmatch(r"[0-9.:]+", name) else None)
        if checked and spki_pin(s.getpeercert(binary_form=True)) not in peer.pins:
            s.close()
            raise OSError("the peer's certificate is neither trusted nor named")
        c = http.client.HTTPSConnection(ip, port, timeout=10, context=ctx)
        c.sock = s
        return c

    def get(self, peer, path):
        c = self._connect(peer, False)
        try:
            c.putrequest("GET", path, skip_host=True, skip_accept_encoding=True)
            c.putheader("Host", host_of(peer.address))
            c.endheaders()
            r = c.getresponse()
            r.read()
        finally:
            c.close()

    def call(self, peer, name, body, key=None, confirming=False):
        """A signed call to a peer (never to one that can't be called)."""
        if not peer.inbound and not confirming:
            return None
        key = key or self.sign_key(peer)
        raw = dumps(body)
        path = FED + name
        date, nonce = str(int(time.time())), b64u(os.urandom(16))
        host = host_of(peer.address)
        hs = [("Content-Type", "application/json"), (H_KEY, key.text), (H_DATE, date), (H_NONCE, nonce),
              (H_SIG, b64u(key.sign(request_message("POST", host, path, date, nonce, raw))))]
        try:
            c = self._connect(peer, name not in ("pair", "hello"))
        except OSError:
            return None
        try:
            c.putrequest("POST", path, skip_host=True, skip_accept_encoding=True)
            c.putheader("Host", host)
            for k, v in hs:
                c.putheader(k, v)
            c.putheader("Content-Length", str(len(raw)))
            c.endheaders(raw)
            r = c.getresponse()
            data = r.read()
            a = Answer(r.status, r.getheaders(), data, nonce, [peer.key])
        except (OSError, http.client.HTTPException):
            return None
        finally:
            c.close()
        if a.signed and name == "hello" and a.status == 200 and isinstance(a.json, dict):
            with self.lock:
                if a.json.get("state") == "paired" and peer.state == "theirs":
                    peer.state = "paired"
                peer.versions = (a.json.get("core"), a.json.get("parts"))
                peer.pins = read_pins(a.json.get("certificates"))
        return a

