"""The suite's own side of the pairing: an HTTPS server (or plain HTTP
under --plain-http) that answers the server under test as a core 0 peer
would, checks each of its requests as core 6.4 says, and records them."""

import http.server
import os
import re
import socket
import ssl
import threading
import time
import urllib.parse

from msfpmedia import clip
from msfpmedia.copies import media_type

from . import ed25519
from .wire import (FED, STREAM, H_SIG, MAX_INT, NotIJSON, answer_message, b64u, commitment, decimal, dumps, has_null,
                   host_of, parse_ijson, read_revocation, read_rotation, request_message, split_hostport,
                   unb64u)

NOT_FOUND = b'{"type":"not-found","title":"Not found","status":404}\n'
MAX_POSITION = 8640000000       # MSFP Media playing 3 and 4: 100 days


def problem(status, typ, title, **extra):
    d = {"type": typ, "title": title, "status": status}
    d.update(extra)
    return dumps(d) + b"\n"


class PeerServer:
    """Plays the suite's side toward the server under test.

    Shared state, read and changed by the checks under `lock`:
    key (the suite's current key), server_keys (keys the server's requests
    may be signed with: current first, then previous ones with until when),
    hosts (signed hosts the suite accepts as its own), state, suspended
    (the server's key revoked toward the suite), and what the suite offers."""

    def __init__(self, suite):
        self.suite = suite
        self.lock = threading.RLock()
        self.listeners = {}             # label -> (httpd, address)
        self.requests = []              # every request, see _record
        self.tls_errors = []            # (time, label, error)
        self.events = []                # (time, what, detail): rotations, revocations from the server
        self.nonces = set()
        self.state = "theirs"
        self.ended = False
        self.suspended = False
        self.hosts = set()              # signed hosts accepted as the suite's
        self.known_hosts = set()        # every host the suite ever named (to tell what a request was signed for)
        self.offer = []
        self.pages = {}
        self.server_commitment = None   # from the pair answer, then rotations
        self.pending_rotation = None    # a new key of the server waiting for this suite's "operator"
        self.auto_confirm = True
        self.server_prev = None         # (key bytes, until)
        self.playbacks = {}             # the server playing this suite's films: id -> see _play
        self.group_events = []          # envelopes this suite signed, for the server's group-sync
        self.tickets = {}               # ticket -> playback id

    # ---- listeners --------------------------------------------------------------

    def listen(self, label, address, listen, context=None):
        """Starts a listener for `address` on `listen` (host:port)."""
        h, port = split_hostport(listen, 8000)
        srv = self

        class Handler(http.server.BaseHTTPRequestHandler):
            protocol_version = "HTTP/1.1"
            timeout = 30

            def log_message(self, *a):
                pass

            def do_POST(self):
                srv._handle(self, label)

            def do_GET(self):
                srv._get(self, label)

            def do_HEAD(self):
                srv._get(self, label)

            def do_PUT(self):
                srv._get(self, label)

        class Server(http.server.ThreadingHTTPServer):
            daemon_threads = True
            allow_reuse_address = True
            address_family = socket.AF_INET6 if ":" in h else socket.AF_INET

            def finish_request(self, request, client_address):
                request.settimeout(30)
                if context is not None:
                    try:
                        request = context.wrap_socket(request, server_side=True)
                    except (ssl.SSLError, OSError) as e:
                        with srv.lock:
                            srv.tls_errors.append((time.time(), label, "%s: %s" % (type(e).__name__, e)))
                        return
                self.RequestHandlerClass(request, client_address, self)

        httpd = Server((h, port), Handler)
        threading.Thread(target=httpd.serve_forever, daemon=True).start()
        self.listeners[label] = (httpd, address)
        self.known_hosts.add(host_of(address))
        return httpd.server_address[1]

    def close(self):
        for httpd, _ in self.listeners.values():
            httpd.shutdown()
            httpd.server_close()

    # ---- keys -------------------------------------------------------------------

    def server_heard(self):
        """The keys the server's requests may be signed with now."""
        keys = list(self.suite.server_keys[:1])
        if self.server_prev and self.server_prev[1] > time.time():
            keys.append(self.server_prev[0])
        return keys

    # ---- requests ---------------------------------------------------------------

    def _check(self, h, body):
        """Core 6.4, and core 3's request rules, for a request from the
        server under test: (faults, notes, values)."""
        faults, notes, vals = [], [], {}
        names = [k.lower() for k in h.headers.keys()]
        for n in ("msfp-key", "msfp-date", "msfp-nonce", "msfp-signature"):
            c = names.count(n)
            if c != 1:
                faults.append("%s sent %d times (core 2.4)" % (n, c))
            else:
                v = h.headers.get(n).strip(" \t")
                if "," in v or re.search(r"\s", v):
                    faults.append("%s holds a comma or white space (core 2.4)" % n)
                vals[n] = v
        if "?" in h.path:
            faults.append("a query in the target (core 6.2)")
        ct = h.headers.get_all("Content-Type") or []
        if len(ct) != 1 or ct[0].split(";")[0].strip().lower() != "application/json":
            faults.append("Content-Type %r, not application/json (core 3)" % ct)
        if any(k.lower().startswith("sec-fetch-") for k in h.headers.keys()):
            faults.append("Sec-Fetch-* headers (core 3: a server-to-server client sends none)")
        if (h.headers.get("Content-Encoding") or "identity").strip().lower() != "identity":
            faults.append("a Content-Encoding on the request (core 3)")
        ae = (h.headers.get("Accept-Encoding") or "identity").strip().lower()
        if ae not in ("identity", ""):
            notes.append("Accept-Encoding %r (core 3: a caller should send none, or identity)" % ae)
        if len(body) > 65536:
            faults.append("a body of %d bytes, over 64 KiB (core 3)" % len(body))
        if len(vals) == 4:
            key = unb64u(vals["msfp-key"], 32)
            vals["key"] = key
            if key is None:
                faults.append("MSFP-Key isn't base64url of 32 bytes (core 2.2)")
            elif key not in self.server_heard():
                faults.append("MSFP-Key %s isn't the server's key as this suite knows it" % vals["msfp-key"])
            d = decimal(vals["msfp-date"])
            if d is None or d == 0:
                faults.append("MSFP-Date %r isn't a positive integer of core 2.2" % vals["msfp-date"])
            elif abs(time.time() - d) > 300:
                faults.append("MSFP-Date %s is %+d s from this clock (core 6.4: 5 minutes)" % (d, d - time.time()))
            if unb64u(vals["msfp-nonce"], 16) is None:
                faults.append("MSFP-Nonce isn't base64url of 16 bytes (core 6.1)")
            elif vals["msfp-nonce"] in self.nonces:
                faults.append("a nonce used twice (core 6.5)")
            sig = unb64u(vals["msfp-signature"], 64)
            if sig is None:
                faults.append("MSFP-Signature isn't base64url of 64 bytes (core 6.1)")
            elif key is not None:
                for host in sorted(self.known_hosts):
                    msg = request_message(h.command, host, h.path, vals["msfp-date"], vals["msfp-nonce"], body)
                    if ed25519.verify_strict(key, msg, sig):
                        vals["host"] = host
                        break
                else:
                    faults.append("the signature doesn't verify over any host this suite has had (%s) (core 6.2)"
                                  % ", ".join(sorted(self.known_hosts)))
                if vals.get("host") and vals["host"] not in self.hosts:
                    faults.append("signed for %s, not a host this suite serves now (core 6.2)" % vals["host"])
        vals["refuse"] = bool(faults)
        try:
            j = parse_ijson(body)
            vals["json"] = j
            if not isinstance(j, dict):
                faults.append("the body isn't a JSON object (core 8.1)")
            elif has_null(j):
                faults.append("the body sends null (core 2.3: no MSFP member is null)")
        except NotIJSON as e:
            faults.append("the body isn't I-JSON: %s (core 2.3)" % e)
        return faults, notes, vals

    def _record(self, **kw):
        kw.setdefault("t", time.time())
        self.requests.append(kw)
        return kw

    def _get(self, h, label):
        if h.path.startswith(STREAM) and h.command in ("GET", "HEAD"):
            self._media(h, label)
            return
        hdrs = {k.lower(): v for k, v in h.headers.items()}
        with self.lock:
            self._record(listener=label, method=h.command, path=h.path, call="discovery" if h.path == "/.well-known/msfp" else None,
                         faults=[x for x in ("msfp-key", "msfp-signature", "msfp-date", "msfp-nonce") if x in hdrs],
                         notes=[], body=b"", host=None, json=None, status=404)
        self._send(h, 404, b"", None, None)

    def _handle(self, h, label):
        n = int(h.headers.get("Content-Length") or 0)
        body = h.rfile.read(min(n, 1024 * 1024)) if n else b""
        path = h.path
        call = path[len(FED):] if path.startswith(FED) and "/" not in path[len(FED):].split("?")[0] else None
        if call:
            call = call.split("?")[0]
        with self.lock:
            faults, notes, vals = self._check(h, body)
            nonce = vals.get("msfp-nonce")
            if nonce and unb64u(nonce, 16) is not None:
                self.nonces.add(nonce)
            rec = self._record(listener=label, method="POST", path=path, call=call, faults=faults, notes=notes, body=body,
                               host=vals.get("host"), json=vals.get("json"), key=vals.get("key"), status=None,
                               uncallable=not self.suite.inbound)
            j = vals.get("json") if isinstance(vals.get("json"), dict) else None
            known = call in ("pair", "hello", "rotate", "revoke", "libraries", "titles", "play", "progress", "stop",
                             "previews") \
                or (call in ("group-push", "group-sync") and self.suite.group_on())
            if not known or vals["refuse"] or call == "pair":
                rec["status"] = 404
                self._send(h, 404, b"", None, None)
                return
            status, out = self._answer(call, j, vals.get("key"), rec)
            rec["status"] = status
            rec["answer"] = out
            self._send(h, status, out, nonce, self.suite.key)

    def _answer(self, call, j, key, rec):
        s = self.suite
        if self.ended:
            return 404, NOT_FOUND
        if j is None:
            return (400, problem(400, "invalid-request", "Invalid request")) if call in ("rotate", "revoke", "play", "progress",
                                                                                         "stop", "previews") \
                else (404, NOT_FOUND)
        if call == "revoke":
            try:
                read_revocation(key, j)
            except ValueError as e:
                rec["faults"].append("its revoke would be refused: %s (core 12.6)" % e)
                return 400, problem(400, "invalid-request", "Invalid request")
            self.suspended = True
            self.events.append((time.time(), "revoke", {"key": key}))
            return 200, dumps({"state": "suspended"})
        if call == "rotate":
            return self._rotate(j, key, rec)
        if self.suspended:
            return 404, NOT_FOUND
        if call == "hello":
            st = j.get("state")
            if not isinstance(st, str):
                return 404, NOT_FOUND
            if st == "ended":
                self.ended = True
                return 200, dumps({"state": "ended"})
            s.learn_hello(j, rec)
            if st == "paired" and self.state == "theirs":
                self.state = "paired"
            return 200, dumps(s.hello_body("paired" if self.state == "paired" else "waiting", answer=True))
        if self.state != "paired":
            return 404, NOT_FOUND
        if call == "libraries":
            return 200, dumps({"name": "MSFP conformance suite", "libraries": self.offer})
        if call == "titles":
            libs = j.get("libraries")
            after = j.get("after", "")
            if not isinstance(libs, list) or any(not isinstance(x, str) for x in libs) or not isinstance(after, str):
                return 404, NOT_FOUND
            page = self.pages.get(after)
            if page is None:
                page = {"titles": [], "seq": 1}
            offered = {x["id"] for x in self.offer}
            page = dict(page, titles=[t for t in page.get("titles", []) if t["library"] in libs and t["library"] in offered])
            return 200, dumps(page)
        if call == "group-push":
            return 200, b"{}"
        if call == "group-sync":
            return self._group_sync(j)
        if call == "play":
            return self._play(j, rec)
        if call == "previews":
            return self._previews(j, rec)
        if call == "progress":
            return self._progress(j, rec)
        if call == "stop":
            if self._malformed("stop", j, rec):
                return 400, problem(400, "invalid-request", "Invalid request")
            pb = self.playbacks.get(j.get("id"))
            if pb is not None and pb["stopped"] is None:
                pb["stopped"] = time.time()
            return 200, b"{}"
        return 404, NOT_FOUND

    def _group_sync(self, j):
        """Groups 8.1: the events this suite signed that the caller lacks."""
        import base64
        import json as _json
        have = j.get("have") if isinstance(j.get("have"), dict) else {}
        out = []
        for env in self.group_events:
            p = _json.loads(base64.urlsafe_b64decode(env["event"] + "=" * (-len(env["event"]) % 4)))
            if p.get("group") == j.get("group") and p.get("seq", 0) > have.get(p.get("member"), 0):
                out.append(env)
        mine = {}
        for env in self.group_events:
            p = _json.loads(base64.urlsafe_b64decode(env["event"] + "=" * (-len(env["event"]) % 4)))
            if p.get("group") == j.get("group"):
                mine[p["member"]] = max(mine.get(p["member"], 0), p["seq"])
        return 200, dumps({"events": out, "mail": [], "have": mine, "more": False})

    # ---- this suite as the owning server: the server under test plays its films ----------

    PROGRESS = 60   # the interval its play answers state (MSFP Media playing 2.3, 3)
    EXPIRY = 120    # MSFP Media playing 5: the idle limit, twice the interval

    def _play(self, j, rec):
        """MSFP Media playing 2: a film of this suite's catalog (the same short clip
        for each), with a ticket under the stream root. delivery file: the
        clip as it is; no delivery (the baseline, MSFP Media playing 2.7: its streams
        already are H.264 and AAC) or a remux to mpegts: complete HLS of the
        clip in MPEG-TS; any other delivery, media type or codec: 409
        delivery-refused for another delivery, target-refused for another
        media type (MSFP Media playing 2.2 step 7). The answer carries the
        ticket, no URL, and progressSeconds (MSFP Media playing 2.3); the
        ticket opens the clip and its one preview sheet. A play that
        replaces a live playback of the server ends that one."""
        bad = self._play_faults(j)
        for b in bad:
            rec["faults"].append("its play would be refused with 400: %s (MSFP Media playing 2.1, 2.2, 2.7; MSFP Media base 3)" % b)
        if bad:
            return 400, problem(400, "invalid-request", "Invalid request")
        keys = {t["key"] for page in self.pages.values() for t in page.get("titles", [])}
        if j["title"] not in keys:
            return 404, NOT_FOUND
        delivery = j.get("delivery")
        if delivery == "file":
            proto = "file"
        elif delivery is None or (delivery == "remux" and j.get("mediaType") == "video/mp2t"):
            proto = "hls"
        elif delivery == "remux":
            return 409, problem(409, "target-refused", "This target is refused")
        else:
            return 409, problem(409, "delivery-refused", "This delivery is refused")
        old = self.playbacks.get(j.get("replaces")) if isinstance(j.get("replaces"), str) else None
        if old is not None and old["stopped"] is None and self.live(old):
            old["stopped"] = time.time()    # replaced: it ends with this 200 (MSFP Media playing 2.3, 5)
            old["replaced"] = True
        for f in ("container", "audioLanguage", "subtitleLanguage", "subtitleTrack"):
            if f in j:
                rec["notes"].append("its play sends %s, which proposal 17 removed (an unknown member, ignored)" % f)
        alphabet = "abcdefghijklmnopqrstuvwxyz234567"
        pid = "play_" + "".join(alphabet[b % 32] for b in os.urandom(26))
        ticket = b64u(os.urandom(24))
        now = time.time()
        self.playbacks[pid] = {"title": j["title"], "ticket": ticket, "opened": now, "progress": [], "media": [],
                               "stopped": None, "expired": None, "proto": proto, "delivery": delivery}
        self.tickets[ticket] = pid
        return 200, dumps({"id": pid, "ticket": ticket, "protocol": proto, "method": "direct" if proto == "file" else "remux",
                           "etag": clip.ETAG, "positionMs": 0, "versionId": "c_clip", "audioTrack": "a1",
                           "audioRenditions": False, "progressSeconds": self.PROGRESS})

    def _previews(self, j, rec):
        """MSFP Media video 5.6: a ticket for the clip's one sheet, for 10 minutes."""
        keys = {t["key"] for page in self.pages.values() for t in page.get("titles", [])}
        if not isinstance(j.get("title"), str) or not isinstance(j.get("versionId"), str):
            return 400, problem(400, "invalid-request", "Invalid request")
        if j["title"] not in keys or j["versionId"] != "c_clip":
            return 404, NOT_FOUND
        ticket = b64u(os.urandom(24))
        pid = "previews_" + ticket
        self.playbacks[pid] = {"title": j["title"], "ticket": ticket, "opened": time.time(), "progress": [], "media": [],
                               "stopped": None, "expired": None, "proto": None, "delivery": None, "previews": True}
        self.tickets[ticket] = pid
        return 200, dumps({"ticket": ticket, "etag": clip.ETAG})

    @staticmethod
    def _play_faults(j):
        """What in a play request the owning server refuses with 400
        (MSFP Media playing 2.1, 2.2 step 1, 2.7; MSFP Media base 3: a value that isn't a
        token is malformed, an unknown token isn't)."""
        tok = re.compile(r"[a-z0-9._-]{1,32}")
        bad = []
        if "capabilities" in j:
            bad.append("capabilities, a capability list: MSFP refuses it (playback 2)")
        if "replaces" in j and not (isinstance(j["replaces"], str) and re.fullmatch(r"[A-Za-z0-9_-]{1,64}", j["replaces"])):
            bad.append("replaces %r isn't of a playback ID's form" % (j["replaces"],))
        if not isinstance(j.get("title"), str):
            bad.append("title missing or not a string")
        purpose = j.get("purpose", "play")
        if purpose not in ("play", "download", "scan"):
            bad.append("purpose %r isn't play, download or scan" % purpose)
        d = j.get("delivery")
        if "delivery" in j and not (isinstance(d, str) and tok.fullmatch(d)):
            bad.append("delivery %r isn't a token" % (d,))
        if "mediaType" in j:
            c = j["mediaType"]
            if d != "remux":
                bad.append("mediaType without delivery remux")
            elif media_type(c) is None:
                bad.append("mediaType %r isn't a media type (MSFP Media base 3)" % (c,))
        elif d == "remux":
            bad.append("delivery remux without mediaType")
        if "transcode" in j:
            t = j["transcode"]
            if d != "transcode":
                bad.append("a transcode object without delivery transcode")
            elif not isinstance(t, dict):
                bad.append("transcode isn't an object")
            else:
                for f in ("videoCodec", "audioCodec", "range"):
                    if f in t and not (isinstance(t[f], str) and tok.fullmatch(t[f])):
                        bad.append("transcode.%s %r isn't a token" % (f, t[f]))
        if purpose == "download" and d not in ("file", "transcode"):
            bad.append("download with delivery %r" % (d,))
        if purpose == "scan" and d != "file":
            bad.append("scan with delivery %r" % (d,))
        for f in ("startPositionMs", "maxBitrateKbps"):
            v = j.get(f)
            if f in j and not (isinstance(v, int) and not isinstance(v, bool) and 0 <= v <= (MAX_POSITION if f[0] == "s" else 10000000)):
                bad.append("%s %r out of its range" % (f, v))
        if "audioRenditions" in j and not isinstance(j["audioRenditions"], bool):
            bad.append("audioRenditions isn't a boolean")
        return bad

    def live(self, pb, now=None):
        """Whether a playback is live: not stopped, and heard of (progress
        or media) within its idle limit (EXPIRY, twice its progressSeconds);
        marks it expired otherwise."""
        now = now or time.time()
        if pb["stopped"] is not None or pb["expired"] is not None:
            return False
        last = max([pb["opened"]] + [t for t, _, _ in pb["progress"]] + pb["media"])
        if now - last > self.EXPIRY:
            pb["expired"] = last + self.EXPIRY
            return False
        return True

    @staticmethod
    def _malformed(call, j, rec):
        """MSFP Media playing 3 and 4: a progress or stop body this suite refuses with
        400, noted as the server's fault."""
        bad = []
        if not isinstance(j.get("id"), str) or not re.fullmatch(r"[A-Za-z0-9_-]{1,64}", j["id"]):
            bad.append("id %r isn't 1 to 64 characters of A-Z a-z 0-9 _ - (MSFP Media playing 2.3)" % j.get("id"))
        pos = j.get("positionMs")
        if "positionMs" in j and not (isinstance(pos, int) and not isinstance(pos, bool) and 0 <= pos <= MAX_POSITION):
            bad.append("positionMs %r isn't an integer from 0 to 8,640,000,000" % pos)
        if call == "progress" and "paused" in j and not isinstance(j["paused"], bool):
            bad.append("paused %r isn't a boolean (MSFP Media playing 3)" % (j["paused"],))
        at, now = j.get("at"), time.time()
        if "at" in j and not (isinstance(at, int) and not isinstance(at, bool) and now - 30 * 86400 <= at <= now + 300):
            bad.append("at %r isn't Unix seconds from 30 days ago to 5 minutes ahead" % at)
        for b in bad:
            rec["faults"].append("its %s would be refused with 400: %s (playback %s)" % (call, b, "3" if call == "progress" else "4"))
        return bool(bad)

    def _progress(self, j, rec):
        if self._malformed("progress", j, rec):
            return 400, problem(400, "invalid-request", "Invalid request")
        pb = self.playbacks.get(j.get("id"))
        if pb is None or not self.live(pb):
            return 410, problem(410, "playback-stopped", "Playback was stopped")
        pos = j.get("positionMs")
        pb["progress"].append((time.time(), pos if isinstance(pos, int) else None, j.get("paused") is True))
        return 200, b"{}"

    def _media(self, h, label):
        """MSFP Media fetching 1 and 2.1: the clip at <stream root><ticket>/file, with
        ranges, for a live playback's ticket."""
        rest = h.path[len(STREAM):].split("?")[0].split("/")
        rest = [rest[0], "/".join(rest[1:])] if len(rest) >= 2 else rest
        with self.lock:
            pid = self.tickets.get(rest[0]) if len(rest) == 2 else None
            pb = self.playbacks.get(pid) if pid else None
            if pb is not None and pb.get("previews"):
                names = ("trickplay/0",)
                ok = rest[1] in names and time.time() < pb["opened"] + 600
            else:
                names = (("file",) if pb is None or pb.get("proto", "file") == "file" else
                         ("master.m3u8", "media.m3u8", "0.ts")) + ("trickplay/0",)
                ok = pb is not None and rest[1] in names and self.live(pb)
            if ok:
                pb["media"].append(time.time())
            self._record(listener=label, method=h.command, path=h.path, call="media", faults=[], notes=[], body=b"",
                         host=None, json=None, status=200 if ok else 404)
        data, ctype = clip.MP4, "video/mp4"
        if ok and rest[1] == "trickplay/0":
            data, ctype = clip.SHEET, "image/jpeg"
        elif ok and rest[1] == "master.m3u8":
            data, ctype = ('#EXTM3U\n#EXT-X-VERSION:3\n#EXT-X-INDEPENDENT-SEGMENTS\n'
                           '#EXT-X-STREAM-INF:BANDWIDTH=200000,CODECS="avc1.42c00c,mp4a.40.2",RESOLUTION=160x90\n'
                           'media.m3u8\n').encode(), "application/vnd.apple.mpegurl"
        elif ok and rest[1] == "media.m3u8":
            data, ctype = ("#EXTM3U\n#EXT-X-VERSION:3\n#EXT-X-TARGETDURATION:2\n#EXT-X-MEDIA-SEQUENCE:0\n"
                           "#EXT-X-PLAYLIST-TYPE:VOD\n#EXTINF:2.000,\n0.ts\n#EXT-X-ENDLIST\n").encode(), \
                "application/vnd.apple.mpegurl"
        elif ok and rest[1] == "0.ts":
            data, ctype = clip.TS, "video/mp2t"
        try:
            if h.command != "GET":
                h.send_response(405)
                h.send_header("Allow", "GET")
                h.send_header("Content-Length", "0")
                h.end_headers()
                return
            if not ok:
                h.send_response(404)
                h.send_header("Content-Length", "0")
                h.end_headers()
                return
            start, end, status = 0, len(data) - 1, 200
            m = re.fullmatch(r"bytes=(\d*)-(\d*)", (h.headers.get("Range") or "").strip())
            if m and (m.group(1) or m.group(2)):
                if m.group(1):
                    start = int(m.group(1))
                    end = min(int(m.group(2)), len(data) - 1) if m.group(2) else len(data) - 1
                else:
                    start = max(len(data) - int(m.group(2)), 0)
                if start >= len(data) or start > end:
                    h.send_response(416)
                    h.send_header("Content-Range", "bytes */%d" % len(data))
                    h.send_header("Content-Length", "0")
                    h.end_headers()
                    return
                status = 206
            h.send_response(status)
            h.send_header("Content-Type", ctype)
            h.send_header("Accept-Ranges", "bytes")
            if rest[1] == "file":
                h.send_header("ETag", '"%s"' % clip.ETAG)
            if status == 206:
                h.send_header("Content-Range", "bytes %d-%d/%d" % (start, end, len(data)))
            h.send_header("Content-Length", str(end - start + 1))
            h.end_headers()
            if h.command == "GET":
                h.wfile.write(data[start:end + 1])
        except OSError:
            pass

    def _rotate(self, j, key, rec):
        """Core 12.3, as the receiver of the server's rotate."""
        s = self.suite
        if key != s.server_keys[0]:
            # Signed with the previous key, after the rotation took effect.
            if self.server_prev and key == self.server_prev[0] and j.get("newKey") == b64u(s.server_keys[0]):
                return 200, dumps({"state": "rotated"})
            return 409, problem(409, "rotation-refused", "Rotation refused")
        try:
            new, nc = read_rotation(key, j)
        except ValueError as e:
            rec["faults"].append("its rotate would be refused with 400: %s (core 9.3, 12.3)" % e)
            self.events.append((time.time(), "rotate-bad", {"why": str(e)}))
            return 400, problem(400, "invalid-request", "Invalid request")
        matches = self.server_commitment is not None and commitment(new) == self.server_commitment
        if self.suspended and not matches:
            self.events.append((time.time(), "rotate-refused", {"new": new, "why": "suspended, no match"}))
            return 409, problem(409, "rotation-refused", "Rotation refused")
        if self.server_commitment is not None and not matches:
            self.events.append((time.time(), "rotate-refused", {"new": new, "why": "doesn't match the commitment"}))
            return 409, problem(409, "rotation-refused", "Rotation refused")
        if not matches:
            if self.pending_rotation != new:
                self.pending_rotation = new
                self.events.append((time.time(), "rotate-pending", {"new": new}))
                return 202, dumps({"state": "pending"})
            if not self.auto_confirm:
                return 202, dumps({"state": "pending"})
        self.pending_rotation = None
        self.server_prev = (key, time.time() + 600)
        s.server_keys[0] = new
        self.server_commitment = nc
        self.suspended = False
        self.events.append((time.time(), "rotated", {"old": key, "new": new, "commitment": nc, "matched": matches}))
        s.on_server_rotated(key, new)
        return 200, dumps({"state": "rotated"})

    def _send(self, h, status, body, nonce, key):
        try:
            h.send_response(status)
            mt = "-"
            if body and status in (200, 202):
                mt = "application/json"
            elif body:
                mt = "application/problem+json"
            if mt != "-":
                h.send_header("Content-Type", mt)
            if nonce is not None and key is not None:
                h.send_header(H_SIG, b64u(key.sign(answer_message(nonce, status, mt, body))))
            h.send_header("Content-Length", str(len(body)))
            h.end_headers()
            h.wfile.write(body)
        except OSError:
            pass

    # ---- what the checks read ----------------------------------------------------------

    def since(self, t, listener=None, call=None):
        with self.lock:
            return [r for r in self.requests if r["t"] >= t and (listener is None or r["listener"] == listener)
                    and (call is None or r["call"] == call)]

    def wait_for(self, pred, timeout, step=0.5):
        end = time.time() + timeout
        while time.time() < end:
            with self.lock:
                hit = [r for r in self.requests if pred(r)]
            if hit:
                return hit
            time.sleep(step)
        return []


def is_int_str(v):
    return isinstance(v, str) and decimal(v) is not None and int(v) <= MAX_INT


def path_name(url):
    return urllib.parse.urlsplit(url).path
