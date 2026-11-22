"""The checks of what proposal 16 added to playback, video and music, as a
peer sees them: deliveries (file, remux, transcode, a copy of several
parts), restarts on the same etag, audio renditions, downloads (limits,
late reports, the speed cap), scan access (limits, the cap the owner
announces as scanBytes, the ticket), the previews call, and music's
tracks, queue and lyrics; with proposal 17's: the answer's
audioRenditions, unknown values in requests (core 8.5), a transcode's
copy and range, the scan's cap in its answer, previews on demand (a
ticket and the etag; the layout is the catalog's), queue entries' URLs
built from the ticket."""

import time

from .copies import keep_previews_answer, sheet_count, video_key
from .media import check_owner_playlist, hls_attributes, scan_answer
from .music import keep_lyrics, keep_queue, music_key, queue_urls
from .playback import SCAN_FLOOR, Refused, keep_limits
from .report import PASS
from .wire import dumps

NO_LIMITS = {"streams": None, "downloads": 0, "downloadsPerDay": 0, "downloadKbps": 0, "scansPerDay": 0}
READ_AT_MOST = 64 * 1024 * 1024     # PB22 reads a larger cap only this far


def from_parts_urls(v):
    from .parts import _urls
    return _urls(v)


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
        from .parts import parts_of
        return next((c for c in self.copies_of(k) if parts_of(c) < 2), None)

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
            bad, pb = self.check_playback(a.json, k, "file")
            if pb is None:
                raise Fail("\n".join(bad))
            if pb["versionId"] != cp["id"]:
                bad.append("versionId %s asked, %r played" % (cp["id"], pb["versionId"]))
            st["pb"], st["copy"] = pb, cp
            if bad:
                raise Fail("\n".join(bad))
            return "%s: method direct, the file at <stream root><ticket>/file, etag %s as the catalog's" % (
                pb["id"], pb["etag"])
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
            s1, h1, b1 = self.client.get(a_pb["url"], rng)
            self.stop(a_pb)
            time.sleep(0.5)
            j, a = self.started({"title": k, "delivery": "file", "versionId": cp["id"]})
            if j is None:
                raise Fail("the new play of the same copy: %s" % a.short())
            st["pb"] = j
            try:
                j = self.view(j, k, "file")
            except Refused as e:
                raise Fail("the new answer is refused: %s" % e)
            st["pb"] = j
            bad = []
            if j.get("etag") != a_pb.get("etag"):
                bad.append("the new playback's etag %r isn't the first's %r, and the copy didn't change" % (
                    j.get("etag"), a_pb.get("etag")))
            s2, h2, b2 = self.client.get(j["url"], rng)
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
            from .parts import parts_of
            multi = [(t["key"], cp) for t in self.whole or [] for cp in (t.get("copies") or [])
                     if isinstance(cp, dict) and parts_of(cp) >= 2]
            if not multi:
                raise Skip("no copy of several parts in the catalog")
            key, cp = multi[0]
            a = self.play({"title": key, "delivery": "file", "versionId": cp["id"]})
            if a.status == 200:
                self.stop(a.json)
            self.signed(a, 409, "delivery-refused")
            return "%s, copy %s of %d parts: 409 delivery-refused" % (key, cp["id"], parts_of(cp))
        self.check("PB13", "playback 2.7, media 1", "delivery file of a copy of several parts: 409 delivery-refused",
                   several_parts)

        def hls_of(j, keep=None):
            """The master and first media playlist of an HLS playback (as
            view keeps it): problems; keep, a dict, gets the master's text."""
            c = self.client
            s, _, m = c.get(j["url"])
            if s != 200:
                return ["master.m3u8: %d" % s]
            if keep is not None:
                keep["master"] = m
            bad, uris = check_owner_playlist(m, True)
            if bad or not uris:
                return bad or ["no media playlist named"]
            base = j["base"]
            s, _, p = c.get(base + uris[0])
            if s != 200:
                return ["%s: %d" % (uris[0], s)]
            return check_owner_playlist(p, False)[0]

        def remux():
            cp = self.copy_of(k)
            mt = "%s/mp4" % ("video" if cp.get("video") else "audio")
            a = self.play({"title": k, "delivery": "remux", "mediaType": mt, "versionId": cp["id"]})
            if a.status == 409 and a.signed and a.problem_type() == "delivery-refused":
                return (PASS, "409 delivery-refused (allowed: the owning server's choice)")
            self.signed(a, 200)
            try:
                bad, pb = self.check_playback(a.json, k, "remux")
                if not bad:
                    bad += hls_of(pb)
            finally:
                self.stop(a.json)
            if bad:
                raise Fail("\n".join(bad))
            return "mediaType %s: method remux, as complete HLS" % mt
        self.check("PB14", "playback 2.7, media 3.1", "delivery remux to mp4 segments (mediaType): 200 remux as complete "
                   "HLS (or 409 delivery-refused)", remux)

        def transcode():
            target = {"audioCodec": "aac", "audioChannels": 2}
            if video_key(k):
                target.update({"videoCodec": "h264", "maxHeight": 480})
            a = self.play({"title": k, "delivery": "transcode", "transcode": target})
            if a.status in (409, 503) and a.signed and a.problem_type() in ("delivery-refused", "server-busy"):
                return (PASS, "%d %s (allowed)" % (a.status, a.problem_type()))
            self.signed(a, 200)
            try:
                bad, pb = self.check_playback(a.json, k, "transcode")
                if not bad:
                    bad += hls_of(pb)
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
                said = a.json.get("audioRenditions")
                if "audioRenditions" in a.json and not isinstance(said, bool):
                    raise Fail("audioRenditions %r isn't a boolean (playback 2.3)" % (said,))
                try:
                    pb = self.view(a.json, k)
                except Refused as e:
                    raise Fail("the answer is refused: %s" % e)
                s, _, m = self.client.get(pb["url"])
                if s != 200:
                    raise Fail("master.m3u8: %d" % s)
                names = [x.split('NAME="', 1)[1].split('"', 1)[0] for x in m.decode("utf-8", "replace").split("\n")
                         if x.startswith("#EXT-X-MEDIA:") and "TYPE=AUDIO" in x and 'NAME="' in x]
                if said is True:
                    bad, _ = check_owner_playlist(m, True, ids)
                    if sorted(names) != sorted(ids[:32]):
                        bad.append("audioRenditions true: the master playlist names renditions %s; the copy's audio "
                                   "tracks are %s (media 3.1: one #EXT-X-MEDIA per track)" % (names, ids))
                    what = "%d audio rendition(s), named by track ID, as the answer's audioRenditions true says" % len(names)
                else:
                    bad = []
                    if names:
                        bad.append("the answer's audioRenditions is %s, and the master playlist names renditions %s "
                                   "(playback 2.3: false or absent means none made)" % (
                                       "false" if said is False else "absent", names))
                    if said is None:
                        bad.append("no audioRenditions in the answer to a play asking for them (playback 2.3: the "
                                   "answer says whether it made them)")
                    what = "audioRenditions false: none made (allowed: a request for them is never refused), and the " \
                           "playlist has none"
            finally:
                self.stop(a.json)
            if bad:
                raise Fail("\n".join(bad))
            return what
        self.check("PB17", "playback 2.1, 2.3, media 3.1", "audioRenditions asked: the answer says whether they were made, "
                   "and the playlist matches (every audio track, or none)", renditions)

        def unknown_values():
            """Core 8.5 in play: an unknown token naming what to serve is 409
            delivery-refused; an unknown member is ignored. (Values that
            aren't tokens are PB1's 400s.)"""
            cp = self.copy_of(k)
            top, other = ("video", "audio") if cp.get("video") else ("audio", "video")
            cases = [("delivery dash, an unknown token", {"delivery": "dash"}),
                     ("remux to %s/matroska, no HLS segment format" % top, {"delivery": "remux", "mediaType": "%s/matroska" % top}),
                     ("remux to %s/mp4, the wrong top-level type" % other, {"delivery": "remux", "mediaType": "%s/mp4" % other}),
                     ("transcode audioCodec conformance-codec, a token no encoder has",
                      {"delivery": "transcode", "transcode": {"audioCodec": "conformance-codec"}})]
            if video_key(k):
                cases.append(("transcode videoCodec h265, not the registry's token", {"delivery": "transcode",
                                                                                     "transcode": {"videoCodec": "h265"}}))
                cases.append(("transcode range hdr, a token not of its list", {"delivery": "transcode",
                                                                               "transcode": {"range": "hdr"}}))
            bad, out = [], []
            for name, extra in cases:
                a = self.play(dict(extra, title=k, versionId=cp["id"]))
                self.answers.append(a)
                if a.status == 200:
                    self.stop(a.json)
                if not (a.status == 409 and a.signed and a.problem_type() == "delivery-refused"):
                    bad.append("%s: %s, not a signed 409 delivery-refused" % (name, a.short()))
            out.append("%d unknown tokens refused with 409 delivery-refused" % len(cases))
            a = self.play({"title": k, "versionId": cp["id"], "conformanceUnknownMember": {"x": [1]}})
            self.answers.append(a)
            if a.status == 200:
                self.stop(a.json)
                out.append("an unknown member ignored (200)")
            else:
                bad.append("a play with an unknown member: %s (core 8.5: unknown members are ignored)" % a.short())
            if bad:
                raise Fail("\n".join(bad))
            return "; ".join(out)
        self.check("PB24", "core 8.5, playback 2.1, 2.2, 2.7", "unknown values in play: a token or media type naming what "
                   "to serve refused with 409 delivery-refused, an unknown member ignored", unknown_values)

        def copy_and_range():
            """Playback 2.7: a transcode with video copy serves the video
            unchanged (the master playlist's CODECS names the copy's own);
            burnSubtitle with video copy is refused; range sdr is SDR (no
            VIDEO-RANGE PQ or HLG); range keep is made or refused."""
            if not video_key(k):
                raise Skip("not a film, an episode or an extra")
            cp = self.copy_of(k)
            v = cp.get("video") if isinstance(cp.get("video"), dict) else None
            if v is None:
                raise Skip("the copy has no video")
            bad, out = [], []
            for name, tr in (("video copy, audio aac", {"videoCodec": "copy", "audioCodec": "aac"}),
                             ("range sdr", {"videoCodec": "h264", "range": "sdr", "audioCodec": "aac"}),
                             ("range keep", {"videoCodec": "hevc", "range": "keep", "audioCodec": "aac"})):
                a = self.play({"title": k, "versionId": cp["id"], "delivery": "transcode", "transcode": tr})
                self.answers.append(a)
                if a.status in (409, 503) and a.signed and a.problem_type() in ("delivery-refused", "server-busy"):
                    out.append("%s: %d %s" % (name, a.status, a.problem_type()))
                    continue
                if a.status != 200 or not a.signed:
                    bad.append("%s: %s, not a signed 200, 409 delivery-refused or 503 server-busy" % (name, a.short()))
                    continue
                try:
                    b, pb = self.check_playback(a.json, k, "transcode")
                    keep = {}
                    if not b:
                        b += hls_of(pb, keep)
                    text = keep.get("master", b"").decode("utf-8", "replace")
                    infs = [hls_attributes(x.split(":", 1)[1]) for x in text.split("\n") if x.startswith("#EXT-X-STREAM-INF:")]
                    infs = [x for x in infs if isinstance(x, dict)]
                    if not b and tr["videoCodec"] == "copy" and v.get("codecs"):
                        want = v["codecs"]
                        if want.startswith("hev1."):
                            want = "hvc1." + want[5:]   # hvc1 in mp4 segments (playback 2.7)
                        if not any(want in [c.strip() for c in (x.get("CODECS") or "").strip('"').split(",")] for x in infs):
                            b.append("video copy: the master playlist's CODECS %s don't name the copy's own %s (playback "
                                     "2.7: the stream unchanged; media 3.1: CODECS describes what is served)"
                                     % ([x.get("CODECS") for x in infs], want))
                    if not b and tr.get("range") == "sdr" and any((x.get("VIDEO-RANGE") or "").strip('"') in ("PQ", "HLG")
                                                                  for x in infs):
                        b.append("range sdr: the master playlist says VIDEO-RANGE %s (playback 2.7: tone-mapped to SDR)"
                                 % [x.get("VIDEO-RANGE") for x in infs])
                    bad += ["%s: %s" % (name, x) for x in b]
                    out.append("%s: 200 transcode" % name)
                finally:
                    self.stop(a.json)
            subs = [t.get("id") for t in cp.get("subtitleTracks") or [] if isinstance(t, dict)]
            if subs:
                a = self.play({"title": k, "versionId": cp["id"], "delivery": "transcode",
                               "transcode": {"videoCodec": "copy", "burnSubtitle": subs[0]}})
                self.answers.append(a)
                if a.status == 200:
                    self.stop(a.json)
                if not (a.status == 409 and a.signed and a.problem_type() == "delivery-refused"):
                    bad.append("burnSubtitle %s with video copy: %s, not a signed 409 delivery-refused (playback 2.7, "
                               "video 5.1)" % (subs[0], a.short()))
                else:
                    out.append("burnSubtitle with video copy: 409 delivery-refused")
            if bad:
                raise Fail("\n".join(bad))
            return "; ".join(out)
        self.check("PB8", "playback 2.7, video 5.1, media 3.1", "a transcode's copy and range: video copy served unchanged, "
                   "range sdr SDR, keep made or refused; burnSubtitle with video copy refused", copy_and_range)

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
                bad, pb = self.check_playback(a.json, k, "file")
                st["pb"], st["copy"] = pb or a.json, cp
                if bad:
                    raise Fail("\n".join(bad))
                out = ["a download: 200, the file"]
                n = lim.get("streams")
                if isinstance(n, int) and 1 <= n <= 4:
                    for _ in range(n):
                        j, a2 = self.started(self.stream_body(k))
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
            status, hdrs, marks = self.client.get_timed(st["pb"]["url"], [("Range", "bytes=0-")], seconds=11)
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
            s, _, _ = c.get(j["url"], [("Range", "bytes=0-99")])
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
            s, _, _ = c.get(j["url"], [("Range", "bytes=0-99")])
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
                        j, a2 = self.started(self.stream_body(k))
                        if j is None:
                            raise Fail("play %d of %d streams: %s" % (len(extra) + 1, n, a2.short()))
                        extra.append(j)
                a = self.play(b)
                self.signed(a, 200)
                bad, pb = self.check_playback(a.json, k, "file", "scan")
                if pb is not None:
                    st["pb"], st["copy"] = pb, cp
                if bad:
                    raise Fail("\n".join(bad))
                return "a scan: 200, the file, scanBytes %d%s" % (pb["scanBytes"], ", with every stream in use (playback 8: "
                                                                  "it uses none)" if extra else "")
            finally:
                for j in extra:
                    self.stop(j)
        self.check("PB21", "playback 8, 5, 2.2, 2.3", "scans: 409 stream-limit-reached when off; else 200 the file, "
                   "scanBytes in the answer, using no stream", limits)

        def cap():
            if "pb" not in st:
                raise Skip("no scan started")
            u, size = st["pb"]["url"], st["copy"]["size"]
            cut = st["pb"]["scanBytes"]
            c = self.client
            bad, sent, n = [], 0, 0
            s, h, _ = c.get(u)
            if s != 416:
                bad.append("a fetch without Range: %d, not 416 (media 4.2)" % s)
            while n < 600 and sent < READ_AT_MOST:
                n += 1
                rng = "bytes=0-"
                want = scan_answer(sent, rng, size, cut)
                s, h, body = c.get(u, [("Range", rng)], limit=min(cut, READ_AT_MOST) + 1)
                got = (s, (_h(h, "content-range") or "").strip())
                if got != want:
                    bad.append("read %d (%d bytes sent before): %d %s; media 4.2: %d %s" % (n, sent, got[0], got[1],
                                                                                         want[0], want[1]))
                    break
                if s == 416:
                    break
                sent += len(body)
            if not bad and sent < READ_AT_MOST and sent != cut:
                bad.append("%d bytes served in all, not the cap of %d (the scan answer's scanBytes%s)" % (
                    sent, cut, "" if cut != SCAN_FLOOR else ", or the floor, 4 MiB"))
            if bad:
                raise Fail("\n".join(bad))
            if sent >= READ_AT_MOST:
                return (PASS, "416 without Range; %d reads of bytes=0- served %d bytes within the cap of %d, read no "
                        "further" % (n, sent, cut))
            return "416 without Range; %d reads of bytes=0- served %d bytes, the last cut at the cap of %d (scanBytes), " \
                   "then 416 bytes */%d" % (n - 1, sent, cut, size)
        self.check("PB22", "playback 8, 2.3, media 4.2", "a scan's reads: within the scan answer's scanBytes (absent: "
                   "4 MiB) in all, cut at the cap, then 416", cap)

        def ticket():
            if "pb" not in st:
                raise Skip("no scan started")
            j, c = st["pb"], self.client
            base = j["base"]
            bad = []
            for name in ("master.m3u8", "subtitles/s0/0", "trickplay/0", "fonts/0"):
                s, _, _ = c.get(base + name)
                if s != 404:
                    bad.append("%s under the scan ticket: %d, not 404 (media 4: file only)" % (name, s))
            bad += self.head_405([j["url"]])
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
                    if isinstance(cp, dict) and isinstance(cp.get("previewLayout"), dict) == want]

        def previews():
            need()
            found = copies_with(True)
            if not found:
                raise Skip("no copy with a previewLayout in the catalog")
            key, cp = found[0]
            a = self.signed(c.call("previews", {"title": key, "versionId": cp["id"]}), 200)
            try:
                kept = keep_previews_answer(a.json)
            except ValueError as e:
                raise Fail("the viewer's server refuses the answer, and fetches no sheet: %s (video 5.6)" % e)
            bad = []
            if from_parts_urls(a.json):
                bad.append("the answer carries URLs (video 5.6, media 1: answers carry no URLs)")
            if "trickplay" in a.json:
                self.note_default("video 5.6", "previews' answer still sends trickplay, dropped by proposal 17 (the "
                                  "layout is the catalog's; ignored)")
            if kept["etag"] != cp.get("etag"):
                bad.append("etag %r; the catalog's copy has %r, and nothing changed on the server (video 5.6: the "
                           "copy's etag as the owning server holds it)" % (kept["etag"], cp.get("etag")))
            if len(kept["ticket"]) < 22:
                bad.append("a ticket of %d characters can't hold 128 random bits (media 1)" % len(kept["ticket"]))
            if bad:
                raise Fail("\n".join(bad))
            st["base"], st["layout"] = self.root + kept["ticket"] + "/", cp["previewLayout"]
            return "%s, copy %s: a ticket and the catalog's etag; %d sheet(s) by the catalog's layout; " % (
                key, cp["id"], sheet_count(cp["previewLayout"])) + self.sheet_check(st["base"], st["layout"])
        self.check("PV1", "video 5.5, 5.6, media 3.4", "previews: 200 signed, a ticket and the copy's etag, no URL; the "
                   "sheets of the catalog's previewLayout under that ticket, 404 past the count", previews)

        def scope():
            if "base" not in st:
                raise Skip("no previews fetched")
            base = st["base"]
            bad = []
            for name in ("file", "master.m3u8", "subtitles/s0/0", "fonts/0", "image"):
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
                forms.append(("a copy without previewLayout", dumps({"title": without[0][0],
                                                                     "versionId": without[0][1]["id"]}), 404))
            for name, b, want in forms:
                a = c.call("previews", b)
                self.answers.append(a)
                if a.status != want or not a.signed:
                    bad.append("%s: %s, not a signed %d" % (name, a.short(), want))
            if bad:
                raise Fail("\n".join(bad))
            return "%d refusals: 400 for malformed requests, 404 for what isn't offered or has no previewLayout" % len(forms)
        self.check("PV3", "video 5.6", "previews refusals: a signed 400 for a malformed request, 404 for a copy without "
                   "previewLayout or not offered", refusals)

    # ---- music (music 5, 5.2, 6) -----------------------------------------------------------

    def music_checks(self):
        Fail, Skip = _x()
        c = self.client
        st = {}

        def tracks():
            self.need("music")
            if "playback" not in self.parts_on():
                raise Skip("playback isn't on")
            ts = [t for t in self.whole or [] if music_key(t.get("key")) == "track" and self.copies_of(t["key"])]
            if not ts:
                raise Skip("no track with copies offered")
            return ts

        def play_track():
            t = tracks()[0]
            a = self.play({"title": t["key"]})
            self.signed(a, 200)
            try:
                bad, pb = self.check_playback(a.json, t["key"])
            finally:
                self.stop(a.json)
            if bad:
                raise Fail("\n".join(bad))
            return "%s: %s over %s, audio only, the catalog's etag" % (t["key"], pb["method"], pb["protocol"])
        self.check("MU1", "music 5, playback 2.3", "a track's play: 200, audio only, its etag the catalog's", play_track)

        def queue():
            ts = tracks()
            if len(ts) < 2:
                raise Skip("one track offered: no queue to name")
            q = [t["key"] for t in ts[1:6]]
            first = self.copies_of(q[-1])[0]
            entries = [{"title": x} for x in q[:-1]] + [{"title": q[-1], "versionId": first["id"]}]
            req = {"title": ts[0]["key"], "queue": entries}
            a = self.play(req)
            self.signed(a, 200)
            try:
                bad = []
                try:
                    pb = self.view(a.json, ts[0]["key"])
                    kept = keep_queue(req, a.json)
                except ValueError as e:
                    raise Fail("the answer is refused: %s" % e)
                sent = a.json.get("queue") or []
                if from_parts_urls(sent):
                    bad.append("queue entries carry URLs (music 5.2, media 1: built from the ticket)")
                if len(kept) != len(sent):
                    bad.append("%d queue entries sent, the viewer's server keeps %d (music 5.2): %s" % (
                        len(sent), len(kept), dumps(sent).decode()[:300]))
                if kept and kept[-1]["title"] == q[-1] and kept[-1]["versionId"] != first["id"]:
                    bad.append("the entry naming copy %s played %s" % (first["id"], kept[-1]["versionId"]))
                for e, u in zip(kept, queue_urls(self.root, pb["ticket"], kept)):
                    s, _, _ = c.get(u, [("Range", "bytes=0-99")] if e["protocol"] == "file" else ())
                    if s not in (200, 206):
                        bad.append("queue entry %s: %s %d" % (e["title"], u[len(self.root):], s))
                s, _, _ = c.get(pb["base"] + "queue/%d/master.m3u8" % len(q))
                if s != 404:
                    bad.append("queue/%d, past the request's queue: %d, not 404 (media 1)" % (len(q), s))
            finally:
                self.stop(a.json)
            if bad:
                raise Fail("\n".join(bad))
            return "%d of %d queued tracks prepared (one naming its copy), each at <stream root><ticket>/queue/<n>/; 404 " \
                   "past the queue" % (len(kept), len(q))
        self.check("MU2", "music 5.2, media 1", "a play with a queue of entries (one with its own versionId): the entries "
                   "in their forms, no URL, their streams at the names built from the ticket", queue)

        def queue_refusals():
            ts = tracks()
            k = ts[0]["key"]
            other = ts[1]["key"] if len(ts) > 1 else "track/mb/0c000000-0000-4000-8000-00000000000f"
            six = ["track/mb/0c000000-0000-4000-8000-0000000000%02x" % i for i in range(16, 22)]
            bad = []
            E = lambda x: {"title": x}
            for name, b in (("6 entries", {"title": k, "queue": [E(x) for x in six]}),
                            ("an entry twice", {"title": k, "queue": [E(other), E(other)]}),
                            ("the title itself", {"title": k, "queue": [E(k)]}),
                            ("an album key", {"title": k, "queue": [E("album/mb/0b000000-0000-4000-8000-000000000002")]}),
                            ("an entry that is a string, not an object", {"title": k, "queue": [other]}),
                            ("an entry's mediaType without its own remux", {"title": k, "queue": [dict(E(other), mediaType="audio/mp4")]}),
                            ("not an array", {"title": k, "queue": other})):
                a = self.play(b)
                self.answers.append(a)
                if a.status == 200:
                    self.stop(a.json)
                if not (a.status == 400 and a.signed):
                    bad.append("%s: %s" % (name, a.short()))
            if bad:
                raise Fail("expected a signed 400 for:\n" + "\n".join(bad))
            return "7 malformed queues refused with a signed 400"
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
            if cp["lyrics"] == "synced" and not any(all("start" in ln for ln in s["line"]) for s in kept["structuredLyrics"]):
                bad.append("the copy says synced; no set whose lines all carry start (music 6)")
            gone = sorted({f for x in a.json.get("structuredLyrics") or [] if isinstance(x, dict)
                           for f in ("synced", "offset", "displayArtist", "displayTitle") if f in x})
            if gone:
                self.note_default("music 6", "%s: lyrics still send %s, dropped by proposal 17 (ignored)" % (key, ", ".join(gone)))
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
