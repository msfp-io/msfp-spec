"""The checks of what proposal 16 added to playback, video and music, as a
peer sees them: deliveries (file, remux, transcode, a copy of several
parts), restarts on the same etag, audio renditions, downloads (limits,
late reports, the speed cap), scan access (limits, the 32 MiB cap, the
ticket), the previews call, and music's tracks, queue and lyrics."""

import time

from .copies import keep_previews_answer, video_key
from .media import SCAN_CAP, check_owner_playlist, scan_answer
from .music import keep_lyrics, keep_queue, music_key
from .playback import keep_limits
from .report import PASS
from .wire import dumps

BROAD_CAPS = {"containers": ["aac", "flac", "m4a", "m4v", "mkv", "mov", "mp3", "mp4", "ogg", "opus", "ts", "wav", "webm"],
              "audioCodecs": ["aac", "ac3", "alac", "eac3", "flac", "mp3", "opus", "pcm_s16le", "vorbis"],
              "videoCodecs": ["av1", "h264", "hevc", "vp8", "vp9"], "hdr": ["hdr10", "hlg"]}
NO_LIMITS = {"streams": None, "downloads": 0, "downloadsPerDay": 0, "downloadKbps": 0, "scansPerDay": 0}


def _x():
    from .checks import Fail, Skip
    return Fail, Skip


def _h(hdrs, name):
    return dict((k.lower(), v) for k, v in hdrs).get(name.lower())


class RevisionChecks:

    def kept_limits(self):
        return keep_limits(self.limits) if isinstance(self.limits, dict) else dict(NO_LIMITS)

    def one_file_copy(self, k):
        """The first copy of k that is one file (no parts, or one)."""
        return next((c for c in self.copies_of(k) if len(c.get("parts") or []) < 2), None)

    def started(self, body):
        """A play expected to start: (its answer's JSON, or None, and the answer)."""
        a = self.play(body)
        if a.status != 200 or not a.signed:
            return None, a
        self.answers.append(a)
        return a.json, a

    def revision_checks(self, k):
        self.delivery_checks(k)
        self.download_checks(k)
        self.scan_checks(k)
        self.previews_checks()
        self.music_checks()

    # ---- deliveries (playback 2.7, media 1, 3.1, 3.2) -----------------------------------

    def delivery_checks(self, k):
        Fail, Skip = _x()
        st = {}

        def file_():
            cp = self.one_file_copy(k)
            if cp is None:
                raise Skip("every copy of %s is of several parts: none plays as file" % k)
            body = {"title": k, "delivery": "file", "versionId": cp["id"]}
            a = self.play(body)
            if a.status == 409 and a.signed and a.problem_type() == "delivery-refused":
                return (PASS, "409 delivery-refused (allowed: the owning server's choice, playback 2.2 step 7)")
            self.signed(a, 200)
            bad, _ = self.check_playback(a.json, k, "file")
            if a.json.get("versionId", cp["id"]) != cp["id"]:
                bad.append("versionId %s asked, %r played" % (cp["id"], a.json.get("versionId")))
            st["pb"], st["copy"] = a.json, cp
            if bad:
                raise Fail("\n".join(bad))
            return "%s: method direct, the file under %s, etag %s as the catalog's" % (
                a.json["id"], "<stream root><ticket>/file", a.json["etag"])
        self.check("PB12", "playback 2.7, 2.3, media 1", "delivery file: 200 direct, the file, the catalog's etag (or 409 "
                   "delivery-refused)", file_)
        if "pb" in st:
            self.file_checks(st["pb"], st["copy"], "F")

        def restart():
            if "pb" not in st:
                raise Skip("no playback of the file")
            cp, a_pb = st["copy"], st["pb"]
            off = min(cp["size"] // 2, 1024 * 1024)
            rng = [("Range", "bytes=%d-%d" % (off, off + 4095))]
            s1, h1, b1 = self.client.get(a_pb["stream"]["url"], rng)
            self.stop(a_pb)
            time.sleep(0.5)
            j, a = self.started({"title": k, "delivery": "file", "versionId": cp["id"]})
            if j is None:
                raise Fail("the new play of the same copy: %s" % a.short())
            st["pb"] = j
            bad = []
            if j.get("etag") != a_pb.get("etag"):
                bad.append("the new playback's etag %r isn't the first's %r, and the copy didn't change" % (
                    j.get("etag"), a_pb.get("etag")))
            s2, h2, b2 = self.client.get(j["stream"]["url"], rng)
            if (s1, s2) != (206, 206):
                bad.append("bytes %d-%d: %d, then %d under the new ticket" % (off, off + 4095, s1, s2))
            elif b1 != b2:
                bad.append("the same range gives other bytes under the new ticket (media 3.2: stable bytes while the "
                           "etag holds)")
            if _h(h1, "etag") != _h(h2, "etag"):
                bad.append("ETag %r, then %r" % (_h(h1, "etag"), _h(h2, "etag")))
            self.stop(j)
            if bad:
                raise Fail("\n".join(bad))
            return "stopped, played again: the same etag %s, and bytes %d-%d the same under the new ticket" % (
                j["etag"], off, off + 4095)
        self.check("PB16", "playback 2.7, media 3.2, 3.3", "a restart: a new play of the same copy gives the same etag "
                   "and the same bytes at the same offset", restart)
        self.stop(st.get("pb"))

        def several_parts():
            multi = [(t["key"], cp) for t in self.whole or [] for cp in (t.get("copies") or [])
                     if isinstance(cp, dict) and len(cp.get("parts") or []) >= 2]
            if not multi:
                raise Skip("no copy of several parts in the catalog")
            key, cp = multi[0]
            a = self.play({"title": key, "delivery": "file", "versionId": cp["id"]})
            if a.status == 200:
                self.stop(a.json)
            self.signed(a, 409, "delivery-refused")
            return "%s, copy %s of %d parts: 409 delivery-refused" % (key, cp["id"], len(cp["parts"]))
        self.check("PB13", "playback 2.7, media 1", "delivery file of a copy of several parts: 409 delivery-refused",
                   several_parts)

        def hls_of(j):
            """The master and first media playlist of an HLS playback: problems."""
            c = self.client
            s, _, m = c.get(j["stream"]["url"])
            if s != 200:
                return ["master.m3u8: %d" % s]
            bad, uris = check_owner_playlist(m, True)
            if bad or not uris:
                return bad or ["no media playlist named"]
            base = j["stream"]["url"].rsplit("/", 1)[0] + "/"
            s, _, p = c.get(base + uris[0])
            if s != 200:
                return ["%s: %d" % (uris[0], s)]
            return check_owner_playlist(p, False)[0]

        def remux():
            cp = self.copy_of(k)
            a = self.play({"title": k, "delivery": "remux", "container": "mp4", "versionId": cp["id"]})
            if a.status == 409 and a.signed and a.problem_type() == "delivery-refused":
                return (PASS, "409 delivery-refused (allowed: the owning server's choice)")
            self.signed(a, 200)
            try:
                bad, _ = self.check_playback(a.json, k, "remux")
                if not bad and a.json["stream"]["protocol"] == "hls":
                    bad += hls_of(a.json)
            finally:
                self.stop(a.json)
            if bad:
                raise Fail("\n".join(bad))
            return "method remux, as complete HLS"
        self.check("PB14", "playback 2.7, media 3.1", "delivery remux: 200 remux as complete HLS (or 409 delivery-refused)",
                   remux)

        def transcode():
            target = {"audioCodec": "aac", "audioChannels": 2}
            if video_key(k):
                target.update({"videoCodec": "h264", "maxHeight": 480})
            a = self.play({"title": k, "delivery": "transcode", "transcode": target})
            if a.status in (409, 503) and a.signed and a.problem_type() in ("delivery-refused", "server-busy"):
                return (PASS, "%d %s (allowed)" % (a.status, a.problem_type()))
            self.signed(a, 200)
            try:
                bad, _ = self.check_playback(a.json, k, "transcode")
                if not bad:
                    bad += hls_of(a.json)
            finally:
                self.stop(a.json)
            if bad:
                raise Fail("\n".join(bad))
            return "method transcode, as complete HLS, to %s" % dumps(target).decode()
        self.check("PB15", "playback 2.7, media 3.1", "delivery transcode to targets: 200 transcode as complete HLS (or "
                   "409 delivery-refused, 503 server-busy)", transcode)

        def renditions():
            cp = self.copy_of(k)
            ids = [t.get("id") for t in cp.get("audioTracks") or [] if isinstance(t, dict)]
            if not ids:
                raise Skip("the copy has no audio track")
            a = self.play({"title": k, "audioRenditions": True, "versionId": cp["id"]})
            self.signed(a, 200)
            try:
                if a.json["stream"]["protocol"] != "hls":
                    raise Skip("served as the file, which holds every track: no renditions to give")
                s, _, m = self.client.get(a.json["stream"]["url"])
                if s != 200:
                    raise Fail("master.m3u8: %d" % s)
                bad, _ = check_owner_playlist(m, True, ids)
                names = [x.split('NAME="', 1)[1].split('"', 1)[0] for x in m.decode("utf-8", "replace").split("\n")
                         if x.startswith("#EXT-X-MEDIA:") and 'NAME="' in x]
                if sorted(names) != sorted(ids[:32]):
                    bad.append("audioRenditions asked: the master playlist names renditions %s; the copy's audio tracks "
                               "are %s (media 3.1: one #EXT-X-MEDIA per track)" % (names, ids))
            finally:
                self.stop(a.json)
            if bad:
                raise Fail("\n".join(bad))
            return "%d audio rendition(s), named by track ID, every EXT-X-STREAM-INF with AUDIO=\"audio\"" % len(names)
        self.check("PB17", "media 3.1, playback 2.1", "audioRenditions: every audio track of the copy as an #EXT-X-MEDIA "
                   "rendition", renditions)

    # ---- downloads (playback 7, media 4.1) ----------------------------------------------

    def download_checks(self, k):
        Fail, Skip = _x()
        lim = self.kept_limits()
        st = {}

        def body():
            cp = self.one_file_copy(k)
            if cp is None:
                raise Skip("no copy of one file to download as it is")
            return {"title": k, "purpose": "download", "delivery": "file", "versionId": cp["id"]}, cp

        def limits():
            b, cp = body()
            if lim["downloads"] == 0:
                a = self.play(b)
                if a.status == 200:
                    self.stop(a.json)
                self.signed(a, 409, "stream-limit-reached")
                return "downloads 0 (none allowed): 409 stream-limit-reached"
            live, extra = [], []
            try:
                a = self.play(b)
                self.signed(a, 200)
                bad, _ = self.check_playback(a.json, k, "file")
                st["pb"], st["copy"] = a.json, cp
                if bad:
                    raise Fail("\n".join(bad))
                out = ["a download: 200, the file"]
                n = lim.get("streams")
                if isinstance(n, int) and 1 <= n <= 4:
                    for _ in range(n):
                        j, a2 = self.started({"title": k, "capabilities": BROAD_CAPS})
                        if j is None:
                            raise Fail("with a download live, play %d of %d streams: %s (playback 7: a download never "
                                       "counts against streams)" % (len(extra) + 1, n, a2.short()))
                        extra.append(j)
                    out.append("the %d streams still free beside it" % n)
                d = lim["downloads"]
                if d <= 4 and (lim["downloadsPerDay"] == 0 or lim["downloadsPerDay"] >= d + 3):
                    for _ in range(d - 1):
                        a2 = self.play(b)
                        self.signed(a2, 200)
                        live.append(a2.json)
                    a2 = self.play(b)
                    if a2.status == 200:
                        live.append(a2.json)
                        raise Fail("download %d of a pairing whose downloads is %d was answered 200" % (d + 1, d))
                    self.signed(a2, 409, "stream-limit-reached")
                    out.append("download %d: 409 stream-limit-reached" % (d + 1))
                return "; ".join(out)
            finally:
                for j in live + extra:
                    self.stop(j)
        self.check("PB18", "playback 7, 5, 2.2", "downloads: 409 stream-limit-reached when none are allowed; else 200 "
                   "the file, beside the streams, within downloads", limits)

        def speed():
            if "pb" not in st:
                raise Skip("no download started")
            kbps = lim["downloadKbps"]
            if not kbps:
                raise Skip("downloadKbps is 0: no speed cap")
            per10 = kbps * 1250
            if st["copy"]["size"] < per10 * 1.3:
                raise Skip("the copy (%d bytes) is under 13 seconds at the cap of %d kbit/s: too small to tell" % (
                    st["copy"]["size"], kbps))
            status, hdrs, marks = self.client.get_timed(st["pb"]["stream"]["url"], [("Range", "bytes=0-")], seconds=11)
            if status != 206:
                raise Fail("bytes=0- of the download: %d" % status)
            in10 = max(n for t, n in marks if t <= 10.0)
            slack = 256 * 1024
            if in10 > per10 * 1.1 + slack:
                raise Fail("%d bytes in the first 10 seconds; the cap of %d kbit/s allows %d (playback 7: paced over any "
                           "10 seconds)" % (in10, kbps, per10))
            if marks[-1][1] == 0:
                raise Fail("no byte in 11 seconds")
            return "%d bytes in the first 10 seconds, within %d kbit/s (%d bytes)" % (in10, kbps, per10)
        self.check("PB20", "playback 7, media 4.1", "a download's reads paced at downloadKbps", speed)

        def late():
            if "pb" not in st:
                raise Skip("no download started")
            j, c = st["pb"], self.client
            now = int(time.time())
            bad = []
            s, _, _ = c.get(j["stream"]["url"], [("Range", "bytes=0-99")])
            if s != 206:
                bad.append("the download's file, bytes=0-99: %d" % s)
            self.signed(c.call("stop", {"id": j["id"], "positionMs": 0}), 200)
            for name, b, want in (("progress with at", {"id": j["id"], "positionMs": 1000, "at": now - 60}, 200),
                                  ("progress without at", {"id": j["id"], "positionMs": 1000}, 410),
                                  ("stop with at", {"id": j["id"], "positionMs": 2000, "at": now - 30}, 200)):
                a = c.call(name.split()[0], b)
                self.answers.append(a)
                if a.status != want or not a.signed:
                    bad.append("%s after the download's stop: %s, not a signed %d (playback 7: late reports)" % (
                        name, a.short(), want))
            s, _, _ = c.get(j["stream"]["url"], [("Range", "bytes=0-99")])
            if s != 404:
                bad.append("the download's ticket after stop: %d, not 404" % s)
            if bad:
                raise Fail("\n".join(bad))
            return "stopped; then progress with at 200, without at 410, stop with at 200; the ticket 404"
        self.check("PB19", "playback 7, 3, 4", "a download's late reports: progress and stop with at accepted after it "
                   "ended", late)
        self.stop(st.get("pb"))

    # ---- scan access (playback 8, media 4.2) --------------------------------------------

    def scan_checks(self, k):
        Fail, Skip = _x()
        lim = self.kept_limits()
        st = {}

        def limits():
            cp = self.one_file_copy(k)
            if cp is None:
                raise Skip("no copy of one file to scan")
            b = {"title": k, "purpose": "scan", "delivery": "file", "versionId": cp["id"]}
            if lim["scansPerDay"] == 0:
                a = self.play(b)
                if a.status == 200:
                    self.stop(a.json)
                self.signed(a, 409, "stream-limit-reached")
                return "scansPerDay 0 (no scan access): 409 stream-limit-reached"
            extra = []
            try:
                n = lim.get("streams")
                if isinstance(n, int) and 1 <= n <= 4:
                    for _ in range(n):
                        j, a2 = self.started({"title": k, "capabilities": BROAD_CAPS})
                        if j is None:
                            raise Fail("play %d of %d streams: %s" % (len(extra) + 1, n, a2.short()))
                        extra.append(j)
                a = self.play(b)
                self.signed(a, 200)
                bad, _ = self.check_playback(a.json, k, "file")
                st["pb"], st["copy"] = a.json, cp
                if bad:
                    raise Fail("\n".join(bad))
                return "a scan: 200, the file%s" % (", with every stream in use (playback 8: it uses none)" if extra else "")
            finally:
                for j in extra:
                    self.stop(j)
        self.check("PB21", "playback 8, 5, 2.2", "scans: 409 stream-limit-reached when off; else 200 the file, using no "
                   "stream", limits)

        def cap():
            if "pb" not in st:
                raise Skip("no scan started")
            u, size = st["pb"]["stream"]["url"], st["copy"]["size"]
            c = self.client
            bad, sent, n = [], 0, 0
            s, h, _ = c.get(u)
            if s != 416:
                bad.append("a fetch without Range: %d, not 416 (media 4.2)" % s)
            while n < 600:
                n += 1
                rng = "bytes=0-"
                want = scan_answer(sent, rng, size)
                s, h, body = c.get(u, [("Range", rng)], limit=SCAN_CAP + 1)
                got = (s, (_h(h, "content-range") or "").strip())
                if got != want:
                    bad.append("read %d (%d bytes sent before): %d %s; media 4.2: %d %s" % (n, sent, got[0], got[1],
                                                                                         want[0], want[1]))
                    break
                if s == 416:
                    break
                sent += len(body)
            if not bad and sent != SCAN_CAP:
                bad.append("%d bytes served in all, not the cap of %d" % (sent, SCAN_CAP))
            if bad:
                raise Fail("\n".join(bad))
            return "416 without Range; %d reads of bytes=0- served %d bytes, the last cut at the cap, then 416 bytes */%d" % (
                n - 1, sent, size)
        self.check("PB22", "playback 8, media 4.2", "a scan's reads: within 32 MiB in all, cut at the cap, then 416",
                   cap)

        def ticket():
            if "pb" not in st:
                raise Skip("no scan started")
            j, c = st["pb"], self.client
            base = j["stream"]["url"][:-len("file")]
            bad = []
            for name in ("master.m3u8", "subtitles/s0/0", "trickplay/0", "fonts/0"):
                s, _, _ = c.get(base + name)
                if s != 404:
                    bad.append("%s under the scan ticket: %d, not 404 (media 4: file only)" % (name, s))
            bad += self.head_405([j["stream"]["url"]])
            a = c.call("progress", {"id": j["id"]})
            self.answers.append(a)
            if a.status != 200 or not a.signed:
                bad.append("progress of a scan: %s, not a signed 200 (playback 3)" % a.short())
            self.signed(c.call("stop", {"id": j["id"]}), 200)
            if bad:
                raise Fail("\n".join(bad))
            return "the scan ticket opens the file only; HEAD 405; progress 200; stop 200"
        self.check("PB23", "media 4, 2.2, playback 3, 8", "a scan's ticket opens the file only; progress answered 200",
                   ticket)
        self.stop(st.get("pb"))

    # ---- previews (video 5.5, 5.6) -------------------------------------------------------

    def previews_checks(self):
        Fail, Skip = _x()
        c = self.client
        st = {}

        def need():
            self.need("video")
            if "playback" not in self.parts_on():
                raise Skip("playback isn't on")

        def copies_with(want):
            return [(t["key"], cp) for t in self.whole or [] if video_key(t.get("key")) for cp in (t.get("copies") or [])
                    if isinstance(cp, dict) and (cp.get("previews") is True) == want]

        def previews():
            need()
            found = copies_with(True)
            if not found:
                raise Skip("no copy with previews true in the catalog")
            key, cp = found[0]
            a = self.signed(c.call("previews", {"title": key, "versionId": cp["id"]}), 200)
            try:
                kept = keep_previews_answer(a.json, self.root)
            except ValueError as e:
                raise Fail("the viewer's server refuses the whole answer: %s (video 5.5)" % e)
            if "trickplay" not in kept:
                raise Fail("the viewer's server drops the previews (a number out of bounds, or sheets of another count): "
                           "%s" % dumps(a.json).decode()[:200])
            bad = self.check_trickplay(a.json["trickplay"], self.root, kept["trickplay"]["sheets"][0][len(self.root):].split("/")[0])
            if bad:
                raise Fail("\n".join(bad))
            st["tp"] = a.json["trickplay"]
            return "%s, copy %s: %d sheet(s) of %d tiles under a ticket of its own; " % (
                key, cp["id"], len(st["tp"]["sheets"]), st["tp"]["tileCount"]) + self.sheet_check(st["tp"])
        self.check("PV1", "video 5.6, 5.5, media 3.4", "previews: 200 signed, the trickplay object within its bounds, "
                   "sheets under its own ticket", previews)

        def scope():
            if "tp" not in st:
                raise Skip("no previews fetched")
            base = st["tp"]["sheets"][0].rsplit("/", 2)[0] + "/"
            bad = []
            for name in ("file", "master.m3u8", "subtitles/s0/0", "image"):
                s, _, _ = c.get(base + name)
                if s != 404:
                    bad.append("%s under the previews ticket: %d, not 404 (media 4: its sheets only)" % (name, s))
            if bad:
                raise Fail("\n".join(bad))
            return "the previews ticket opens its sheets only"
        self.check("PV2", "media 4, video 5.6", "a previews ticket opens only its sheets", scope)

        def refusals():
            need()
            k = next((t["key"] for t in self.whole or [] if video_key(t.get("key")) and t.get("copies")), None)
            bad = []
            forms = [("an array", b"[]", 400), ("no title", b'{"versionId":"x"}', 400),
                     ("a series, which has no copies", dumps({"title": "series/tmdb/1399", "versionId": "x"}), 400),
                     ("a key in no canonical form", dumps({"title": "movie/tmdb/0949", "versionId": "x"}), 400),
                     ("a film not offered", dumps({"title": "movie/tmdb/9999999999", "versionId": "x"}), 404)]
            if k:
                forms += [("no versionId", dumps({"title": k}), 400),
                          ("a versionId of another form", dumps({"title": k, "versionId": "a b"}), 400),
                          ("a copy it doesn't have", dumps({"title": k, "versionId": "conformance-no-such-copy"}), 404)]
            without = copies_with(False)
            if without:
                forms.append(("a copy without previews", dumps({"title": without[0][0], "versionId": without[0][1]["id"]}), 404))
            for name, b, want in forms:
                a = c.call("previews", b)
                self.answers.append(a)
                if a.status != want or not a.signed:
                    bad.append("%s: %s, not a signed %d" % (name, a.short(), want))
            if bad:
                raise Fail("\n".join(bad))
            return "%d refusals: 400 for malformed requests, 404 for what isn't offered or has no previews" % len(forms)
        self.check("PV3", "video 5.6", "previews refusals: a signed 400 for a malformed request, 404 for a copy without "
                   "previews or not offered", refusals)

    # ---- music (music 5, 5.2, 6) -----------------------------------------------------------

    def music_checks(self):
        Fail, Skip = _x()
        c = self.client
        st = {}

        def tracks():
            self.need("music")
            if "playback" not in self.parts_on():
                raise Skip("playback isn't on")
            ts = [t for t in self.whole or [] if t.get("kind") == "track" and music_key(t.get("key")) == "track"
                  and self.copies_of(t["key"])]
            if not ts:
                raise Skip("no track with copies offered")
            return ts

        def play_track():
            t = tracks()[0]
            a = self.play({"title": t["key"]})
            self.signed(a, 200)
            try:
                bad, _ = self.check_playback(a.json, t["key"])
            finally:
                self.stop(a.json)
            if bad:
                raise Fail("\n".join(bad))
            return "%s: %s over %s, audio only, the catalog's etag" % (t["key"], a.json["method"], a.json["stream"]["protocol"])
        self.check("MU1", "music 5, playback 2.3", "a track's play: 200, audio only, its etag the catalog's", play_track)

        def queue():
            ts = tracks()
            if len(ts) < 2:
                raise Skip("one track offered: no queue to name")
            req = {"title": ts[0]["key"], "queue": [t["key"] for t in ts[1:6]]}
            a = self.play(req)
            self.signed(a, 200)
            try:
                kept = keep_queue(req, a.json, self.root)
                sent = a.json.get("queue") or []
                bad = []
                if len(kept) != len(sent):
                    bad.append("%d queue entries sent, the viewer's server keeps %d (music 5.2): %s" % (
                        len(sent), len(kept), dumps(sent).decode()[:300]))
                for e in kept:
                    u = e["stream"]["url"]
                    s, _, _ = c.get(u, [("Range", "bytes=0-99")] if e["stream"]["protocol"] == "file" else ())
                    if s not in (200, 206):
                        bad.append("queue entry %s: its stream %d" % (e["title"], s))
            finally:
                self.stop(a.json)
            if bad:
                raise Fail("\n".join(bad))
            return "%d of %d queued tracks prepared, each with its stream under the playback's ticket" % (len(kept), len(req["queue"]))
        self.check("MU2", "music 5.2", "a play with a queue: the entries in their forms, their streams served", queue)

        def queue_refusals():
            ts = tracks()
            k = ts[0]["key"]
            other = ts[1]["key"] if len(ts) > 1 else "track/mb/0c000000-0000-4000-8000-00000000000f"
            six = ["track/mb/0c000000-0000-4000-8000-0000000000%02x" % i for i in range(16, 22)]
            bad = []
            for name, b in (("6 entries", {"title": k, "queue": six}), ("an entry twice", {"title": k, "queue": [other, other]}),
                            ("the title itself", {"title": k, "queue": [k]}),
                            ("an album key", {"title": k, "queue": ["album/mb/0b000000-0000-4000-8000-000000000002"]}),
                            ("not an array", {"title": k, "queue": other})):
                a = self.play(b)
                self.answers.append(a)
                if a.status == 200:
                    self.stop(a.json)
                if not (a.status == 400 and a.signed):
                    bad.append("%s: %s" % (name, a.short()))
            if bad:
                raise Fail("expected a signed 400 for:\n" + "\n".join(bad))
            return "5 malformed queues refused with a signed 400"
        self.check("MU3", "music 5.2", "a malformed queue: a signed 400", queue_refusals)

        def lyrics():
            ts = tracks()
            found = [(t["key"], cp) for t in ts for cp in t.get("copies") or [] if isinstance(cp, dict)
                     and cp.get("lyrics") in ("plain", "synced")]
            if not found:
                raise Skip("no track copy with lyrics in the catalog")
            key, cp = found[0]
            a = self.signed(c.call("lyrics", {"title": key, "versionId": cp["id"]}), 200)
            try:
                kept = keep_lyrics(a.body)
            except ValueError as e:
                raise Fail("the viewer's server fails the fetch: %s (music 6)" % e)
            bad = []
            if len(kept["structuredLyrics"]) != len(a.json.get("structuredLyrics") or []):
                bad.append("sets dropped by the viewer's server")
            if cp["lyrics"] == "synced" and not any(s.get("synced") for s in kept["structuredLyrics"]):
                bad.append("the copy says synced; no synced set")
            for name, b, want in (("no versionId", {"title": key}, 400),
                                  ("a copy it doesn't have", {"title": key, "versionId": "conformance-no-such-copy"}, 404)):
                r = c.call("lyrics", b)
                self.answers.append(r)
                if r.status != want or not r.signed:
                    bad.append("%s: %s, not a signed %d" % (name, r.short(), want))
            if bad:
                raise Fail("\n".join(bad))
            return "%s: %d set(s) of lyrics kept whole; 400 without versionId, 404 for another copy" % (
                key, len(kept["structuredLyrics"]))
        self.check("MU4", "music 6", "lyrics: 200 as structuredLyrics, kept whole; refusals", lyrics)
