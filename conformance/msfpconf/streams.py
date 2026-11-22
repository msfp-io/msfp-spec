"""The media checks (spec/v0/media.md, video.md section 6, as revised by
proposals 16 and 17): HLS complete from the first fetch and the same on
every fetch, segments, ranges, refusals and HEAD; the file by byte ranges
(206, Content-Range against the exact size, ETag, Accept-Ranges, 416);
WebVTT, ASS, PGS and fonts within their bounds; preview sheets under the
playback's ticket; the names built from the ticket served exactly, 404
past their counts. pb is a play answer as parts.view keeps it (url and
base built from its ticket)."""

import os
import re
import time

from .copies import sheet_count, video_key
from .media import (Refused, check_206, check_file_answer, check_font, check_owner_playlist, check_pgs, jpeg_size,
                    rebuild_ass, rebuild_vtt)
from .report import PASS
from .wire import b64u

MALFORMED = ["bytes=0-1,5-9", "bytes=-", "bytes=5-1", "items=0-9", "bytes = 0-9", "bytes=0-1234567890123456",
             "bytes=-0", "bytes=a-b"]


TEXT_FORMATS = ("subrip", "webvtt", "mov_text", "ttml", "ass", "ssa")
PICTURE_FORMATS = ("hdmv_pgs_subtitle", "dvd_subtitle", "dvb_subtitle")   # video 5.4's table


def _x():
    from .checks import Fail, Skip
    return Fail, Skip


def _h(hdrs, name):
    return dict((k.lower(), v) for k, v in hdrs).get(name.lower())


def audio_ids(copy):
    return [t.get("id") for t in (copy or {}).get("audioTracks") or [] if isinstance(t, dict)]


class StreamChecks:

    def head_405(self, urls):
        """Media 2.2: HEAD on a media URL is 405 with Allow: GET, never 404."""
        bad = []
        for u in urls:
            status, hdrs, _ = self.client.get(u, method="HEAD")
            allow = _h(hdrs, "allow") or ""
            if status != 405 or "GET" not in [x.strip() for x in allow.split(",")]:
                bad.append("HEAD %s: %d%s, not 405 with Allow: GET (media 2.2)" % (
                    u.rsplit("/", 2)[-2] + "/" + u.rsplit("/", 1)[-1], status,
                    " with Allow %r" % allow if status == 405 else ""))
        return bad

    # ---- a playback's media (media 1-4, video 6) ----------------------------------

    def media_checks(self, pb, title, prefix):
        Fail, Skip = _x()
        c = self.client
        url, base = pb["url"], pb["base"]
        copy = self.copy_of(title, pb.get("versionId"))
        st = {}

        def fetch(u, headers=(), method="GET"):
            return c.get(u, headers, method)

        def master():
            if pb["protocol"] != "hls":
                raise Skip("a file stream (the F checks read it)")
            status, hdrs, body = fetch(url)
            if status != 200:
                raise Fail("master.m3u8: %d" % status)
            bad, uris = check_owner_playlist(body, True, audio_ids(copy) or None)
            if bad:
                raise Fail("\n".join(bad))
            variants = [u for u in uris if u.endswith(".m3u8")] or uris
            if not variants:
                raise Fail("the master playlist names no media playlist")
            st["variant"], st["master"] = variants[0], body
            return "%d bytes, %d media playlist(s), the subset of media 3.1 only" % (len(body), len(variants))
        self.check(prefix + "1", "media 3.1", "master.m3u8: 200, only the master subset, bare names", master)

        def media_playlist():
            if "variant" not in st:
                raise Skip("no master playlist read")
            status, _, body = fetch(base + st["variant"])
            if status != 200:
                raise Fail("%s: %d" % (st["variant"], status))
            bad, uris = check_owner_playlist(body, False)
            if bad:
                raise Fail("\n".join(bad))
            segs = [u for u in uris if not re.search(r'#EXT-X-MAP:URI="%s"' % re.escape(u), body.decode("utf-8", "replace"))]
            if not segs:
                raise Fail("the media playlist names no segment")
            time.sleep(1)
            again = [fetch(base + st["variant"])[2], fetch(url)[2]]
            if again[0] != body:
                raise Fail("%s fetched again, a second later, isn't the same playlist (media 3.1: every fetch under one "
                           "ticket gives the same, complete from the first)" % st["variant"])
            if again[1] != st["master"]:
                raise Fail("master.m3u8 fetched again isn't the same (media 3.1)")
            st["segment"], st["segments"] = segs[0], uris
            st["playlist"] = base + st["variant"]
            return "%s: complete (VOD, EXT-X-ENDLIST last), %d URIs, the same on a second fetch" % (st["variant"], len(uris))
        self.check(prefix + "2", "media 3.1", "the media playlist: 200, complete from the first fetch and the same on "
                   "every fetch, the media subset, bare names", media_playlist)

        def segment():
            if pb["protocol"] == "file":
                st["ranged"] = url
                return "a file stream: ranges checked below"
            if "segment" not in st:
                raise Skip("no segment named")
            u = base + st["segment"]
            for _ in range(30):
                status, hdrs, body = fetch(u)
                if status != 503:
                    break
                time.sleep(2)
            if status != 200:
                raise Fail("%s: %d" % (st["segment"], status))
            st["ranged"], st["size"] = u, len(body)
            return "%s: %d bytes" % (st["segment"], len(body))
        self.check(prefix + "3", "media 3", "the first segment of the media playlist: 200", segment)

        def refusals():
            if pb["protocol"] == "hls" and "variant" not in st:
                raise Skip("no playlist read")
            bad = []
            for name, u, hs, method in (("a query string", url + "?x=1", (), "GET"),
                                        ("an Origin header", url, [("Origin", "https://example.org")], "GET"),
                                        ("Sec-Fetch-Site", url, [("Sec-Fetch-Site", "same-origin")], "GET"),
                                        ("Sec-Fetch-Mode", url, [("Sec-Fetch-Mode", "cors")], "GET"),
                                        ("a ticket never given", url.replace("/" + base.rsplit("/", 2)[1] + "/", "/" + b64u(os.urandom(24)) + "/"), (), "GET"),
                                        ("a name that isn't served", base + "conformance-no-such-file", (), "GET")):
                status, _, _ = fetch(u, hs, method)
                if status != 404:
                    bad.append("%s: %d" % (name, status))
            for m in ("POST", "PUT", "DELETE"):
                status, _, _ = fetch(url, (), m)
                if status != 405:
                    bad.append("%s: %d (media 2.2: 405)" % (m, status))
            heads = [url] + [x for x in (st.get("playlist"), st.get("ranged") if st.get("ranged") != url else None) if x]
            bad += self.head_405(heads)
            if bad:
                raise Fail("\n".join(bad))
            return "404 for a query, browser headers, an unknown ticket and name; POST, PUT, DELETE 405; HEAD 405 with " \
                   "Allow: GET on %d URL(s)" % len(heads)
        self.check(prefix + "4", "media 1, 2, 2.2", "a query, browser headers, a ticket never given: 404; other methods "
                   "and HEAD: 405", refusals)

        def range_on_playlist():
            if pb["protocol"] != "hls":
                raise Skip("a file stream")
            status, hdrs, body = fetch(url, [("Range", "bytes=0-9")])
            if status != 200:
                raise Fail("a Range on master.m3u8 gave %d; media 2.1: the owning server ignores it and sends the whole" % status)
            return "200, the whole playlist"
        self.check(prefix + "5", "media 2.1", "a Range on a playlist is ignored: 200 and the whole", range_on_playlist)
        self.range_checks(prefix + "9", st, copy if pb["protocol"] == "file" else None)

        if not video_key(title):
            return
        self.subtitle_checks(pb, title, prefix)

        def sheets():
            tp = (copy or {}).get("previewLayout")
            if not isinstance(tp, dict):
                raise Skip("the copy playing has no previewLayout in the catalog")
            return self.sheet_check(base, tp) + " (the playback's ticket opens its copy's sheets, with no previews call)"
        self.check(prefix + "8", "video 5.5, 6.1, 6.2, media 3.4", "a preview sheet under the playback's ticket: 200 "
                   "image/jpeg within the catalog's layout; 404 past the count", sheets)

        def names():
            bad, out = [], []
            n = pb.get("fontCount", 0)
            tries = [("fonts/%d" % n, "a font at fontCount")]
            layout = (copy or {}).get("previewLayout")
            if isinstance(layout, dict):
                tries.append(("trickplay/%d" % sheet_count(layout), "the sheet at the sheet count"))
            else:
                tries.append(("trickplay/0", "a sheet of a copy without previewLayout"))
            served = {x["id"]: x for x in pb.get("subtitles", [])}
            ids = [t.get("id") for t in (copy or {}).get("subtitleTracks") or [] if isinstance(t, dict)]
            for sid in ids:
                x = served.get(sid)
                if x is None:
                    tries.append(("subtitles/%s/0" % sid, "track %s, not served apart" % sid))
                elif x["form"] == "text":
                    tries.append(("pictures/%s/0" % sid, "track %s as pictures, served as text" % sid))
                    if not x.get("ass"):
                        tries.append(("ass/%s/0" % sid, "track %s as ASS, without ass" % sid))
                else:
                    tries.append(("subtitles/%s/0" % sid, "track %s as text, served as pictures" % sid))
            tries.append(("subtitles/s999/0", "a track the copy doesn't have"))
            tries.append(("queue/0/master.m3u8", "a queue entry not asked"))
            for name, what in tries:
                status, _, _ = c.get(base + name)
                if status != 404:
                    bad.append("%s (%s): %d, not 404" % (name, what, status))
                out.append(name)
            if bad:
                raise Fail("\n".join(bad) + "\n(video 6.1, media 1: the owning server serves exactly the names built "
                           "from the answer and the catalog, and 404 past a count)")
            return "404 for %d names past the answer's counts: %s" % (len(out), ", ".join(out))
        self.check(prefix + "10", "video 6.1, media 1", "the names built from the ticket: 404 past fontCount, the sheet "
                   "count, and for a track not served apart in that form", names)

    def sheet_check(self, base, tp):
        """Sheet 0 of the layout tp under base (<stream root><ticket>/)."""
        Fail, _ = _x()
        u = base + "trickplay/0"
        status, hdrs, body = self.client.get(u, limit=8 * 1024 * 1024 + 1)
        if status != 200:
            raise Fail("sheet 0: %d" % status)
        ct = _h(hdrs, "content-type") or ""
        size = jpeg_size(body)
        bad = []
        if ct.split(";")[0].strip().lower() != "image/jpeg":
            bad.append("Content-Type %r" % ct)
        if len(body) > 8 * 1024 * 1024:
            bad.append("%d bytes, over 8 MiB" % len(body))
        if size is None:
            bad.append("not a JPEG by its header")
        elif size[0] > tp["columns"] * tp["tileWidth"] or size[1] > tp["rows"] * tp["tileHeight"]:
            bad.append("%dx%d, bigger than a full sheet" % size)
        n = sheet_count(tp)
        root = u.rsplit("/", 1)[0]
        for name, x in (("the sheet count", "%s/%d" % (root, n)), ("a leading zero", "%s/00" % root), ("a query", u + "?x=1")):
            s2, _, _ = self.client.get(x)
            if s2 != 404:
                bad.append("%s: %d, not 404" % (name, s2))
        bad += self.head_405([u])
        if bad:
            raise Fail("\n".join(bad))
        return "sheet 0: %dx%d JPEG, %d bytes; 404 past the count, with a leading zero, with a query; HEAD 405" % (
            size[0], size[1], len(body))

    # ---- subtitles and fonts (video 5.3, 6.1 to 6.6) --------------------------------------

    def subtitle_checks(self, pb, title, prefix):
        Fail, Skip = _x()
        c = self.client

        def tracks_of(j):
            """The subtitle tracks served apart, each with its URLs built from
            the ticket (video 6.1)."""
            out = []
            for x in j.get("subtitles", []):
                t = dict(x, url=j["base"] + ("subtitles/" if x["form"] == "text" else "pictures/") + x["id"])
                if x.get("ass"):
                    t["assUrl"] = j["base"] + "ass/" + x["id"]
                out.append(t)
            return out

        def has(t, kind):
            if kind == "ass":
                return bool(t.get("assUrl"))
            return t["form"] == ("picture" if kind == "pictures" else "text")
        subs = tracks_of(pb)

        def find(kind):
            """A track of that kind with its URL: the first play's, else one
            of another title whose copy lists one in the catalog, played for
            it (up to 8). Returns (track, the other playback, what was played)."""
            t = next((x for x in subs if has(x, kind)), None)
            if t or prefix != "M":
                return t, None, title
            formats = {"ass": ("ass", "ssa"), "pictures": PICTURE_FORMATS, "subtitles": TEXT_FORMATS}[kind]
            tried = []
            for x in self.whole or []:
                if len(tried) >= 8 or x.get("key") == title or not video_key(x.get("key")) or \
                        video_key(x["key"]).kind not in ("movie", "episode", "extra"):
                    continue
                cp = next((cc for cc in x.get("copies") or [] if isinstance(cc, dict) and any(
                    isinstance(s, dict) and s.get("format") in formats for s in cc.get("subtitleTracks") or [])), None)
                if cp is None:
                    continue
                a = self.play({"title": x["key"], "versionId": cp.get("id")})
                if a.status != 200 or not isinstance(a.json, dict):
                    tried.append("%s: %s" % (x["key"], a.short()))
                    continue
                try:
                    other = self.view(a.json, x["key"])
                except ValueError as e:
                    tried.append("%s: its answer refused: %s" % (x["key"], e))
                    self.stop(a.json)
                    continue
                t = next((s for s in tracks_of(other) if has(s, kind)), None)
                if t:
                    return t, other, x["key"]
                tried.append("%s: no track served as %s in its answer" % (x["key"], kind))
                self.stop(a.json)
            return None, None, "; ".join(tried) or "no other title's copy lists such a track"

        def later(cid, section, name, fn, kind):
            # Without a track of its kind in the first play, a check waits
            # for that playback to stop (PB6): another play may need the
            # stream or the transcode it holds (a server-busy 503 otherwise).
            if prefix == "M" and not any(has(x, kind) for x in subs):
                self._subs_later.append((cid, section, name, fn))
            else:
                self.check(cid, section, name, fn)
        self._subs_later = []

        def with_track(kind, what, fn):
            def run():
                t, other, key = find(kind)
                if not t:
                    raise Skip("no %s, in the first play or in up to 8 other titles listing one (%s)" % (what, key))
                try:
                    return fn(t) + ("" if other is None else " (of %s, played for it)" % key)
                finally:
                    self.stop(other)
            return run

        def text_part(t):
            out = []
            for part, whole in (("0", False), ("whole", True)):
                status, hdrs, body = c.get(t["url"] + "/" + part, limit=17 * 1024 * 1024)
                if status != 200:
                    raise Fail("%s/%s: %d" % (t["id"], part, status))
                try:
                    again = rebuild_vtt(body, whole)
                except Refused as e:
                    raise Fail("%s of %s is refused by video 6.3: %s" % (part, t["id"], e))
                if again != body:
                    raise Fail("%s of %s isn't in the rebuilt form video 6.3 writes (the owning server applies the rules "
                               "to what it serves): %r..." % (part, t["id"], body[:120]))
                ct = _h(hdrs, "content-type") or ""
                if not ct.lower().replace(" ", "").startswith("text/vtt"):
                    raise Fail("%s/%s: Content-Type %r, not text/vtt; charset=utf-8" % (t["id"], part, ct))
                out.append("%s %d bytes" % (part, len(body)))
            bad = self.head_405([t["url"] + "/0", t["url"] + "/whole"])
            if bad:
                raise Fail("\n".join(bad))
            return "%s: part %s, in their rebuilt form; HEAD 405" % (t["id"], " and ".join(out))
        later(prefix + "6", "video 6.2, 6.3, media 3.4", "a text subtitle track, part 0 and whole: 200 text/vtt, already in "
              "its rebuilt form", with_track("subtitles", "text track with a url", text_part), "subtitles")

        def pgs_part(t):
            status, hdrs, body = c.get(t["url"] + "/0", limit=64 * 1024 * 1024 + 1)
            if status != 200:
                raise Fail("%s/0: %d" % (t["id"], status))
            try:
                check_pgs(body)
            except Refused as e:
                raise Fail("part 0 of %s fails video 6.5: %s" % (t["id"], e))
            s2, _, _ = c.get(t["url"] + "/whole")
            if s2 != 404:
                raise Fail("%s/whole: %d (media 3.4: picture tracks come in stretches only)" % (t["id"], s2))
            return "%s part 0: %d bytes of valid PGS; no whole picture track" % (t["id"], len(body))
        later(prefix + "7", "video 6.2, 6.5, media 3.4", "a picture subtitle part: 200, every segment and object checked",
              with_track("pictures", "picture track with a url", pgs_part), "pictures")

        def ass_part(t):
            out = []
            for part, whole in (("0", False), ("whole", True)):
                status, hdrs, body = c.get(t["assUrl"] + "/" + part, limit=17 * 1024 * 1024)
                if status != 200:
                    raise Fail("%s ASS %s: %d" % (t["id"], part, status))
                try:
                    again = rebuild_ass(body, whole)
                except Refused as e:
                    raise Fail("ASS %s of %s is refused by video 6.4: %s" % (part, t["id"], e))
                if again != body:
                    raise Fail("ASS %s of %s isn't in the rebuilt form video 6.4 writes: %r..." % (part, t["id"], body[:120]))
                ct = _h(hdrs, "content-type") or ""
                if not ct.lower().replace(" ", "").startswith("text/x-ass"):
                    raise Fail("ASS %s/%s: Content-Type %r, not text/x-ass; charset=utf-8" % (t["id"], part, ct))
                out.append("%s %d bytes" % (part, len(body)))
            return "%s as ASS: part %s, in their rebuilt form" % (t["id"], " and ".join(out))
        later(prefix + "11", "video 6.4, media 3.4", "an ASS track, part 0 and whole: 200 text/x-ass, already in its "
              "rebuilt form", with_track("ass", "ASS or SSA track with an assUrl", ass_part), "ass")

        def fonts():
            fs = [pb["base"] + "fonts/%d" % i for i in range(pb.get("fontCount", 0))]
            if not fs:
                raise Skip("fontCount 0 in the answer (a copy whose ASS tracks use fonts gives their count)")
            out, bad = [], []
            total = 0
            for i, u in enumerate(fs[:64]):
                status, hdrs, body = c.get(u, limit=10 * 1024 * 1024 + 1)
                if status != 200:
                    bad.append("font %d: %d" % (i, status))
                    continue
                try:
                    typ = check_font(body)
                except Refused as e:
                    bad.append("font %d: %s (video 6.6)" % (i, e))
                    continue
                ct = (_h(hdrs, "content-type") or "").split(";")[0].strip().lower()
                if ct != typ:
                    bad.append("font %d: a %s served as %r" % (i, typ, ct))
                out.append("%s %d bytes" % (typ, len(body)))
                total += len(body)
            if total > 64 * 1024 * 1024:
                bad.append("the copy's fonts hold %d bytes in all, over 64 MiB (video 6.6)" % total)
            s2, _, _ = c.get(pb["base"] + "fonts/%d" % len(fs))
            if s2 != 404:
                bad.append("fonts/%d, at fontCount: %d, not 404 (video 6.1)" % (len(fs), s2))
            if bad:
                raise Fail("\n".join(bad))
            return "%d font(s): %s" % (len(out), ", ".join(out))
        self.check(prefix + "12", "video 6.1, 6.6, media 3.4, 2.4", "fonts 0 to fontCount - 1: TrueType or OpenType, each "
                   "within 10 MiB, 64 MiB in all; 404 at fontCount",
                   fonts)

    # ---- ranges, on a segment or the file (media 2.1, 3.2) ---------------------------------

    def range_checks(self, cid, st, copy=None):
        """Media 2.1 on a segment, or on the file (copy: its catalog entry,
        whose size and etag every answer must give, media 3.2)."""
        Fail, Skip = _x()

        def ranges():
            u = st.get("ranged")
            if not u:
                raise Skip("no file or segment fetched")
            c = self.client
            bad = []
            size, etag = (copy.get("size"), copy.get("etag")) if copy else (None, None)

            def file_ok(rng, status, hdrs, body):
                if copy is None:
                    return
                why = check_file_answer(rng, size, etag, status, _h(hdrs, "etag"), _h(hdrs, "content-range"),
                                        _h(hdrs, "content-length"))
                if why:
                    bad.append("%s: %s (media 3.2)" % (rng, why))
                if (_h(hdrs, "accept-ranges") or "").strip() != "bytes":
                    bad.append("%s: Accept-Ranges %r, not bytes (media 3.2)" % (rng, _h(hdrs, "accept-ranges")))
            status, hdrs, body = c.get(u, [("Range", "bytes=0-99")])
            if status == 200 and copy is None:
                return (PASS, "a 200 with the whole to bytes=0-99 (allowed for a segment: media 2.1); ranges not served")
            if status != 206:
                raise Fail("bytes=0-99: %d%s" % (status, " (media 2.1: the file answers a satisfiable Range with 206)"
                                                 if copy else ""))
            why = check_206("", hdrs, body, first=0, last=99)
            if why:
                bad.append("bytes=0-99: " + why)
            file_ok("bytes=0-99", status, hdrs, body)
            total = re.search(r"/([0-9]+)", _h(hdrs, "content-range") or "")
            total = int(total.group(1)) if total else size
            forms = [("bytes=-10", {"count": 10}), ("bytes=5-", {"first": 5}), ("bytes=000-009", {"first": 0, "last": 9})]
            if total:
                forms.append(("bytes=0-%d" % (total + 1000), {"first": 0, "last": total + 1000}))
            for form, kw in forms:
                s2, h2, b2 = c.get(u, [("Range", form)])
                if s2 == 206:
                    why = check_206(form, h2, b2, **kw)
                    if why:
                        bad.append("%s: %s" % (form, why))
                    file_ok(form, s2, h2, b2)
                elif s2 != 200 or copy is not None:
                    bad.append("%s: %d" % (form, s2))
            malformed = list(MALFORMED)
            if total:
                malformed.append("bytes=%d-" % total)
                malformed.append("bytes=%d-" % (total + 10))
            for form in malformed:
                s2, h2, _ = c.get(u, [("Range", form)])
                if s2 != 416:
                    bad.append("%s: %d, not 416" % (form, s2))
                elif copy is not None and (_h(h2, "content-range") or "").strip() != "bytes */%d" % size:
                    bad.append("%s: 416 with Content-Range %r, not bytes */%d (media 2.1)" % (form, _h(h2, "content-range"), size))
            if bad:
                raise Fail("\n".join(bad))
            return "206s that answer bytes=0-99, -10, 5-, 000-009, past the end%s; 416 for %d malformed or unsatisfiable " \
                   "ranges%s" % (" with Accept-Ranges, the ETag and the exact size" if copy else "", len(malformed),
                                 ", with bytes */<size>" if copy else "")
        self.check(cid, "media 2.1%s" % (", 3.2" if copy else ""), "ranges: the three forms give a matching 206; "
                   "malformed or past the end give 416", ranges)

    def file_checks(self, pb, copy, prefix="F"):
        """The file of a delivery file (media 3.2): F1 the whole, F2 ranges,
        F3 stable bytes, F4 HEAD and conditional headers."""
        Fail, Skip = _x()
        c = self.client
        u = pb["url"]
        size, etag = copy.get("size"), copy.get("etag")
        st = {"ranged": u}

        def whole():
            if size > 64 * 1024 * 1024:
                raise Skip("the copy is %d bytes: the suite reads the file whole only up to 64 MiB" % size)
            status, hdrs, body = c.get(u, limit=size + 1)
            why = check_file_answer(None, size, etag, status, _h(hdrs, "etag"), None, _h(hdrs, "content-length"))
            bad = [why] if why else []
            if status == 200 and len(body) != size:
                bad.append("%d bytes in the body; the catalog's size is %d" % (len(body), size))
            if (_h(hdrs, "accept-ranges") or "").strip() != "bytes":
                bad.append("Accept-Ranges %r, not bytes" % _h(hdrs, "accept-ranges"))
            if bad:
                raise Fail("\n".join(bad))
            st["body"] = body
            return "200 without Range: %d bytes, the catalog's exact size; ETag \"%s\"; Accept-Ranges: bytes" % (size, etag)
        self.check(prefix + "1", "media 3.2, video 3.2", "the file without Range: 200, the catalog's size, ETag and "
                   "Accept-Ranges", whole)
        self.range_checks(prefix + "2", st, copy)

        def stable():
            a = c.get(u, [("Range", "bytes=0-199")])
            b = c.get(u, [("Range", "bytes=100-299")])
            if a[0] != 206 or b[0] != 206:
                raise Fail("two ranges: %d, %d" % (a[0], b[0]))
            bad = []
            if a[2][100:200] != b[2][:100]:
                bad.append("bytes 100 to 199 differ between bytes=0-199 and bytes=100-299 (media 3.2: stable bytes)")
            if "body" in st and st["body"][:200] != a[2]:
                bad.append("bytes=0-199 aren't the whole file's first 200 bytes")
            t = c.get(u, [("Range", "bytes=-16")])
            if "body" in st and t[0] == 206 and t[2] != st["body"][-16:]:
                bad.append("bytes=-16 aren't the whole file's last 16 bytes")
            if bad:
                raise Fail("\n".join(bad))
            return "overlapping ranges, the head and the tail give the same bytes"
        self.check(prefix + "3", "media 3.2", "stable bytes: overlapping ranges agree", stable)

        def methods():
            bad = self.head_405([u])
            s, h, _ = c.get(u, [("Range", "bytes=0-9"), ("If-None-Match", '"%s"' % etag)])
            if s != 206:
                bad.append("bytes=0-9 with If-None-Match the etag: %d, not 206 (media 2: conditional headers ignored)" % s)
            s, h, _ = c.get(u, [("Range", "bytes=0-9"), ("If-Range", '"conformance-another-etag"')])
            if s != 206:
                bad.append("bytes=0-9 with If-Range another etag: %d, not 206 (media 2: ignored)" % s)
            for m in ("POST", "PUT"):
                s, _, _ = c.get(u, method=m)
                if s != 405:
                    bad.append("%s: %d, not 405" % (m, s))
            if bad:
                raise Fail("\n".join(bad))
            return "HEAD 405 with Allow: GET; If-None-Match and If-Range ignored; POST and PUT 405"
        self.check(prefix + "4", "media 2, 2.2", "the file: HEAD and other methods 405; conditional headers ignored", methods)
