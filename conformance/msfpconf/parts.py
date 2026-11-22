"""The parts' checks: catalog (libraries, titles, extras, local titles and
their image), playback (play, progress, stop, limits, the choice of copy
and tracks), as revised by proposal 16. Deliveries, downloads, scans,
previews and music are in revision.py; the media in streams.py."""

import concurrent.futures
import json
import os
import re
import time

from .content import KINDS, LIBRARY_ID, title_problems
from .copies import video_key
from .media import Refused as MediaRefused, image_answer, image_kind
from .music import music_key
from .playback import Refused, keep_limits, keep_play_answer, stream_ticket
from .report import PASS
from .revision import RevisionChecks
from .streams import StreamChecks
from .wire import STREAM, dumps, is_int

BROAD_CAPS = {"containers": ["aac", "flac", "m4a", "m4v", "mkv", "mov", "mp3", "mp4", "ogg", "opus", "ts", "wav", "webm"],
              "audioCodecs": ["aac", "ac3", "alac", "eac3", "flac", "mp3", "opus", "pcm_s16le", "vorbis"],
              "videoCodecs": ["av1", "h264", "hevc", "vp8", "vp9"], "hdr": ["hdr10", "hlg"],
              "maxAudioChannels": 8, "maxHeight": 2160}
PAGE_BYTES = 3 * 1024 * 1024
TEXT_FORMATS = ("subrip", "webvtt", "mov_text", "ttml", "ass", "ssa")
PICTURE_FORMATS = ("pgs", "vobsub", "dvb")
VECTORS = os.path.join(os.path.dirname(os.path.abspath(__file__)), "..", "..", "vectors", "v0")
LIMIT_FIELDS = ("streams", "transcodes", "bitrateKbps", "downloads", "downloadsPerDay", "downloadKbps", "scansPerDay",
                "audioStreams", "audioTranscodes")
MAX = 2**53 - 1


def _x():
    from .checks import Fail, Skip
    return Fail, Skip


def scheme_of(key):
    """A title key's scheme (tmdb, tvdb, mb, local), or None."""
    p = key.split("/") if isinstance(key, str) else []
    return p[1] if len(p) > 2 else None


class PartChecks(StreamChecks, RevisionChecks):

    def need(self, part):
        _, Skip = _x()
        if part not in self.parts_on():
            raise Skip("%s isn't on between the server and this suite (core 10.3)" % part)

    @property
    def root(self):
        """The server's stream root (media 1)."""
        return self.target + self.client.base[:-len("/api/v1/federation/")] + STREAM

    # ---- the catalog's view of a title ---------------------------------------------

    def title_of(self, key):
        return next((t for t in self.whole or [] if t.get("key") == key), None)

    def copies_of(self, key):
        t = self.title_of(key)
        cs = t.get("copies") if isinstance(t, dict) else None
        return [c for c in cs if isinstance(c, dict)] if isinstance(cs, list) else []

    def copy_of(self, key, vid=None):
        """The copy of key the catalog lists as vid (the first without it)."""
        cs = self.copies_of(key)
        if vid is None:
            return cs[0] if cs else None
        return next((c for c in cs if c.get("id") == vid), None)

    # ---- catalog: libraries (catalog 2) -------------------------------------------

    def catalog_checks(self):
        Fail, Skip = _x()
        c = self.client

        def libs():
            self.need("catalog")
            a = self.signed(c.call("libraries", {}), 200)
            j = a.json
            if "name" in j and not isinstance(j["name"], str):
                raise Fail("name isn't a string (a caller fails the answer)")
            ls = j.get("libraries")
            if not isinstance(ls, list):
                raise Fail("libraries isn't an array (a caller fails the answer)")
            ids = []
            for lib in ls:
                if not isinstance(lib, dict):
                    raise Fail("a library that isn't an object: %r" % lib)
                i = lib.get("id")
                if not isinstance(i, str) or not LIBRARY_ID.fullmatch(i):
                    raise Fail("an ID outside 1 to 64 characters of A-Z a-z 0-9 _ -: %r" % i)
                if i in ids:
                    raise Fail("the ID %r twice" % i)
                ids.append(i)
                if not isinstance(lib.get("name"), str):
                    raise Fail("library %s: name isn't a string" % i)
                if lib.get("kind") not in KINDS:
                    raise Fail("library %s: kind %r isn't one of catalog 2's" % (i, lib.get("kind")))
            if len(ls) > 200:
                raise Fail("%d libraries: a caller keeps 200" % len(ls))
            self.libraries = ls
            self.limits = j.get("limits")
            return "%d librar%s: %s" % (len(ls), "y" if len(ls) == 1 else "ies",
                                       ", ".join("%s (%s)" % (x["id"], x["kind"]) for x in ls) or "none offered")
        self.check("L1", "catalog 2", "libraries: 200 signed, IDs and kinds in their forms, no ID twice", libs)

        def limits():
            self.need("catalog")
            lim = self.limits
            if "playback" not in self.parts_on():
                if lim is not None:
                    raise Fail("limits sent while playback is off (catalog 2: only when it is on)")
                return "playback off, no limits"
            if lim is None:
                return (PASS, "no limits (a server that doesn't enforce limits sends none, playback 5)")
            kept = keep_limits(lim)
            if kept is None:
                self.limits = None
                raise Fail("a caller ignores the whole limits object (a member that isn't an integer, or not an "
                           "object): %s" % dumps(lim).decode()[:200])
            dropped = [f for f in LIMIT_FIELDS if f in lim and kept.get(f) != lim[f]]
            if dropped:
                raise Fail("out of range, a caller ignores them: %s (playback 5, music 5.1)" % ", ".join(
                    "%s %r" % (f, lim[f]) for f in dropped))
            return "limits %s; downloads %s, scans %s" % (dumps(lim).decode(), "offered" if kept["offersDownloads"] else "off",
                                                          "offered" if kept["offersScans"] else "off")
        self.check("L2", "playback 5, music 5.1", "libraries' limits: only with playback on, each in its range "
                   "(streams, downloads, scans)", limits)

        def opaque():
            self.need("catalog")
            if not self.libraries:
                raise Skip("nothing offered to this suite")
            looks = [x["id"] for x in self.libraries if re.fullmatch(
                r"[0-9a-fA-F]{32}|[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}", x["id"])]
            if looks:
                raise Fail("IDs shaped like a media server's item IDs (32 hex digits, a GUID): %s; catalog 2.1 says "
                           "they must not be one (a heuristic: a random value may look so)" % looks)
            return "none looks like a media server's ID"
        self.check("L3", "catalog 2.1", "library IDs don't look like the media server's own IDs", opaque, "SHOULD")

        def stable():
            self.need("catalog")
            a = self.signed(c.call("libraries", {}), 200)
            again = [x.get("id") for x in a.json.get("libraries", []) if isinstance(x, dict)]
            if again != [x["id"] for x in self.libraries]:
                raise Fail("two readings a moment apart give other IDs: %s, then %s" % ([x["id"] for x in self.libraries], again))
            return "the same IDs twice"
        self.check("L4", "catalog 2.1", "library IDs are stable between two readings", stable)

        def bad_body():
            self.need("catalog")
            out = []
            for name, body in (("an array", b"[]"), ("not JSON", b"libraries"), ("a member twice", b'{"a":1,"a":2}'),
                               ("a lone surrogate", b'{"x":"\\udc00"}')):
                a = c.raw(c.base + "libraries", body, c.headers(c.base + "libraries", body))
                self.signed(a, 404)
                out.append(name)
            return "signed 404 for " + ", ".join(out)
        self.check("L5", "catalog 2, core 2.3", "a libraries body that isn't an I-JSON object gets a signed 404", bad_body)
        self.titles_checks()

    # ---- catalog: titles (catalog 3, video 3-4, music 2-3) ------------------------------

    def read_titles(self, libs, since=None, after="", limit_pages=400):
        """A reading: its pages, each with "_bytes", the size of its body."""
        Fail, _ = _x()
        pages = []
        while len(pages) < limit_pages:
            req = {"libraries": libs, "after": after}
            if since is not None:
                req["since"] = since
            a = self.signed(self.client.call("titles", req), 200)
            j = a.json
            for f, t in (("titles", list), ("gone", list), ("next", str), ("changes", bool)):
                if f in j and not isinstance(j[f], t):
                    raise Fail("%s isn't a%s %s: a caller fails the whole answer (catalog 3.3)" % (f, "n" if t is list else "", t.__name__))
            if "seq" in j and not is_int(j["seq"]):
                raise Fail("seq isn't an integer: a caller fails the answer (catalog 3.3)")
            j = dict(j, _bytes=len(a.body))
            pages.append(j)
            nxt = j.get("next")
            if not isinstance(nxt, str) or nxt == "" or nxt <= after:
                break
            after = nxt
        if len({bool(p.get("changes")) for p in pages}) > 1:
            raise Fail("a reading mixes changes and whole-list pages (catalog 3.2)")
        return pages

    def validate_page(self, j, libs, after, whole):
        bad = []
        titles = j.get("titles") or []
        keys = []
        on = self.parts_on()
        for t in titles:
            _, probs = title_problems(t, libs, on)
            bad += probs
            if isinstance(t, dict) and isinstance(t.get("key"), str):
                keys.append(t["key"])
        bk = [k.encode() for k in keys]
        if bk != sorted(bk) or len(set(bk)) != len(bk):
            bad.append("keys not in strictly ascending byte order (catalog 3.1)")
        if any(k.encode() <= after.encode() for k in keys):
            bad.append("keys not after %r" % after)
        gone = j.get("gone") or []
        if len(titles) + len(gone) > 5000:
            bad.append("more than 5,000 keys in a page (catalog 3.1)")
        if j.get("_bytes", 0) > PAGE_BYTES:
            bad.append("a page of %d bytes, over 3 MiB of JSON (catalog 3.1)" % j["_bytes"])
        if whole and j.get("next") and keys and j["next"] != keys[-1]:
            bad.append("next isn't the page's last key (catalog 3.1)")
        if isinstance(j.get("next"), str) and len(j["next"].encode()) > 512:
            bad.append("a next over 512 bytes (catalog 3.1)")
        if "seq" in j and not j["seq"] > 0:
            bad.append("seq %r isn't positive" % j["seq"])
        if not whole and any(not isinstance(g, str) or len(g.encode()) > 128 for g in gone):
            bad.append("a gone entry that isn't a key of at most 128 bytes")
        return bad

    def titles_checks(self):
        Fail, Skip = _x()
        c = self.client
        libs = sorted(x["id"] for x in self.libraries)

        def need():
            self.need("catalog")
            if not libs:
                raise Skip("the server offers this suite no library: offer one on the server, then run again")

        def whole():
            need()
            pages = self.read_titles(libs)
            bad, after = [], ""
            for j in pages:
                if j.get("changes") is True:
                    bad.append("a reading without since answered with changes")
                bad += self.validate_page(j, libs, after, True)
                ks = [t["key"] for t in (j.get("titles") or []) if isinstance(t, dict) and isinstance(t.get("key"), str)]
                if ks:
                    after = ks[-1]
            self.pages = pages
            self.whole = [t for j in pages for t in (j.get("titles") or []) if isinstance(t, dict)]
            self.seq = pages[0].get("seq")
            if bad:
                raise Fail("\n".join(sorted(set(bad))[:25]))
            kinds = {}
            for t in self.whole:
                kinds[t.get("kind")] = kinds.get(t.get("kind"), 0) + 1
            ncopies = sum(len(t.get("copies") or []) for t in self.whole)
            return "%d titles in %d page(s) (%s), %d copies, seq %s" % (
                len(self.whole), len(pages), ", ".join("%d %s" % (v, k) for k, v in sorted(kinds.items())), ncopies, self.seq)
        self.check("T1", "catalog 3, 3.1, 3.4, 4; video 3, 3.1 to 3.10, 4; music 2, 3", "titles, the whole list: keys, "
                   "kinds, libraries, copies in their forms and order, extras, local descriptions, paging", whole)

        def paging():
            need()
            if not self.whole or len(self.whole) < 2:
                raise Skip("fewer than 2 titles offered")
            after = self.whole[0]["key"]
            got = [t["key"] for j in self.read_titles(libs, after=after) for t in (j.get("titles") or [])]
            want = [t["key"] for t in self.whole[1:]]
            if got != want:
                raise Fail("after %r gave %d titles; the whole list has %d after it" % (after, len(got), len(want)))
            return "after %r gives the rest of the list" % after
        self.check("T2", "catalog 3.1", "a page starts strictly after `after`", paging)

        def changes():
            need()
            if not self.seq:
                raise Skip("the server sends no seq: it is asked for the whole list every time (allowed)")
            pages = self.read_titles(libs, since=self.seq)
            j = pages[0]
            if j.get("changes") is not True:
                return (PASS, "the whole list (allowed: catalog 3.2 lets a receiver without the log answer so)")
            bad = sum((self.validate_page(p, libs, "", False) for p in pages), [])
            if bad:
                raise Fail("\n".join(bad))
            return "changes: %d new or changed, %d gone, seq %s" % (len(j.get("titles") or []), len(j.get("gone") or []), j.get("seq"))
        self.check("T3", "catalog 3.2", "titles since the current seq: a changes answer (or the whole list)", changes)

        def above():
            need()
            if not self.seq:
                raise Skip("no seq")
            if self.read_titles(libs, since=self.seq + 1000000)[0].get("changes") is True:
                raise Fail("since above the current seq was answered with changes; catalog 3.2: the whole list")
            return "the whole list"
        self.check("T4", "catalog 3.2", "since above the current seq: the whole list", above)

        def negative():
            need()
            if self.read_titles(libs, since=-1)[0].get("changes") is True:
                raise Fail("a negative since was answered with changes")
            return "the whole list"
        self.check("T5", "catalog 3", "a negative since is answered like 0", negative)

        def bad_requests():
            need()
            L = dumps(libs)
            forms = [("since as a string", dumps({"libraries": libs, "since": "5"})),
                     ("since 1.0", b'{"libraries":%s,"since":1.0}' % L), ("since 1e1", b'{"libraries":%s,"since":1e1}' % L),
                     ("since 2^53", b'{"libraries":%s,"since":9007199254740992}' % L),
                     ("since -0", b'{"libraries":%s,"since":-0}' % L), ("since null", b'{"libraries":%s,"since":null}' % L),
                     ("after over 512 bytes", dumps({"libraries": libs, "after": "a" * 513})),
                     ("after not a string", dumps({"libraries": libs, "after": 5})),
                     ("after null", b'{"libraries":%s,"after":null}' % L),
                     ("a library ID that isn't a string", dumps({"libraries": libs + [7]})),
                     ("libraries missing", b"{}"), ("libraries null", b'{"libraries":null}'),
                     ("libraries a string", dumps({"libraries": libs[0]}))]
            bad = []
            for name, body in forms:
                a = c.raw(c.base + "titles", body, c.headers(c.base + "titles", body))
                self.answers.append(a)
                if not (a.status == 404 and a.signed):
                    bad.append("%s: %s" % (name, a.short()))
            if bad:
                raise Fail("expected a signed 404 for:\n" + "\n".join(bad))
            return "%d bad requests refused with a signed 404" % len(forms)
        self.check("T6", "catalog 3, core 2.3", "bad titles requests (since, after, libraries; null) get a signed 404", bad_requests)

        def unknown_library():
            need()
            j = self.read_titles(["lib_conformance_not_offered"])[0]
            if j.get("titles"):
                raise Fail("titles for a library that isn't offered: %r" % j["titles"][:2])
            return "ignored"
        self.check("T7", "catalog 3, 5", "a library ID that isn't offered is ignored", unknown_library)

        def first_library():
            need()
            if len(libs) < 2:
                raise Skip("one library offered")
            holders = {}
            for lib in libs:
                for p in self.read_titles([lib]):
                    for t in p.get("titles") or []:
                        if isinstance(t, dict):
                            holders.setdefault(t.get("key"), []).append(lib)
            shared = {k: v for k, v in holders.items() if len(v) > 1}
            if not shared:
                return (PASS, "no title is in two libraries; nothing to tell")
            got = {t["key"]: t.get("library") for t in self.whole}
            bad = ["%s: library %r, the first by bytes of %s is %r" % (k, got.get(k), v, min(v, key=str.encode))
                   for k, v in shared.items() if got.get(k) != min(v, key=str.encode)]
            if bad:
                raise Fail("\n".join(bad[:10]))
            return "%d titles in several libraries name the first by bytes" % len(shared)
        self.check("T8", "catalog 3", "a title in several asked libraries names the first in byte order", first_library)

        def watch():
            need()
            ok, why = self.can_ask_operator()
            if not ok:
                raise Skip(why)
            if not self.a.watch_changes:
                raise Skip("run with --watch-changes to have the operator add and remove a title")
            if not self.seq:
                raise Skip("the server sends no seq")
            old = {t["key"] for t in self.whole}
            seen = {}

            def changed():
                pages = self.read_titles(libs, since=self.seq)
                if pages[0].get("changes") is True and any((p.get("titles") or p.get("gone")) for p in pages):
                    seen["pages"] = pages
                    return True
                return False
            self.ask_operator("change", "add one title to an offered library and remove one", changed, self.a.watch_changes)
            if "pages" not in seen:
                raise Skip("no change seen")
            pages = seen["pages"]
            new = [t["key"] for p in pages for t in (p.get("titles") or [])]
            gone = [k for p in pages for k in (p.get("gone") or [])]
            after = [t for p in self.read_titles(libs) for t in (p.get("titles") or []) if isinstance(t, dict)]
            now = {t["key"] for t in after}
            # The titles the checks after this one take (play among them):
            # the list as it is now, not as it was before the change.
            self.whole = after
            expect = (old - set(gone)) | set(new)
            if now != expect:
                raise Fail("the changes don't add up to the new whole list: differ on %s" % sorted(expect ^ now)[:10])
            return "%s new or changed, %s gone, and they add up to the whole list" % (new, gone)
        self.check("T9", "catalog 3.2", "after a change on the server: titles and gone add up to the new whole list", watch,
                   operator=True)

        def bounds():
            need()
            if not getattr(self, "pages", None):
                raise Skip("no whole reading")
            sizes = [p["_bytes"] for p in self.pages]
            bad = ["page %d: %d titles, %d bytes" % (i, len(p.get("titles") or []), p["_bytes"]) for i, p in enumerate(self.pages)
                   if len(p.get("titles") or []) > 5000 or p["_bytes"] > PAGE_BYTES]
            keys = [t["key"] for t in self.whole if isinstance(t.get("key"), str)]
            if keys and not self.pages[-1].get("next"):
                rest = self.read_titles(libs, after=keys[-1])
                more = [t.get("key") for p in rest for t in (p.get("titles") or []) if isinstance(t, dict)]
                if more:
                    bad.append("the last page had no next, yet %d title(s) remain after %r: %s (catalog 3.1: next "
                               "is set whenever titles remain)" % (len(more), keys[-1], more[:3]))
            if bad:
                raise Fail("\n".join(bad))
            return "%d page(s) of at most 5,000 titles and 3 MiB (the largest %d bytes); nothing after the last" % (
                len(sizes), max(sizes))
        self.check("T10", "catalog 3.1", "pages within 5,000 titles and 3 MiB of JSON; next whenever titles remain", bounds)

        def extras():
            need()
            xs = [t for t in self.whole or [] if t.get("kind") == "extra"]
            if not xs:
                raise Skip("no extra offered (a film's or a series' trailer or featurette, say)")
            by = {t["key"]: t for t in self.whole}
            bad = []
            for x in xs:
                vk = video_key(x["key"])
                parent = vk.parent if vk else None
                p = by.get(parent)
                if p is None:
                    bad.append("%s: its title %s isn't in the list (catalog 3.5: an extra only with its title)" % (x["key"], parent))
                elif p.get("library") != x.get("library"):
                    bad.append("%s: library %r, its title's is %r (catalog 3.5: in the same library)" % (
                        x["key"], x.get("library"), p.get("library")))
            if bad:
                raise Fail("\n".join(bad[:10]))
            return "%d extra(s), each with its title, in its library" % len(xs)
        self.check("T11", "catalog 3.5, video 3.9, 4.6", "extras only with their title, in the same library", extras)
        self.image_checks()
        self.local_switch(libs)

    # ---- the image of a local title (catalog 4.4, media 3.4) ----------------------------

    def local_titles(self):
        return [t for t in self.whole or [] if scheme_of(t.get("key")) == "local"]

    def image_checks(self):
        Fail, Skip = _x()
        c = self.client
        st = {}

        def on():
            self.need("catalog")
            if not ({"video", "music"} & self.parts_on()):
                raise Skip("neither video nor music is on: no local titles, no image call")

        def image():
            on()
            with_image = [t for t in self.local_titles() if t.get("image") is True and t.get("kind") in ("movie", "series",
                                                                                                         "artist", "album")]
            if not with_image:
                raise Skip("no local title with an image offered (share local titles with this peer, one with an image)")
            t = with_image[0]
            a = self.signed(c.call("image", {"title": t["key"]}), 200)
            try:
                ticket = image_answer(self.root, a.json)
            except MediaRefused as e:
                raise Fail("the answer's url: %s (catalog 4.4: <stream root><ticket>/image)" % e)
            if len(ticket) < 22:
                raise Fail("a ticket of %d characters can't hold 128 random bits (media 1)" % len(ticket))
            status, hdrs, body = c.get(a.json["url"], limit=1024 * 1024 + 1)
            if status != 200:
                raise Fail("the image: %d" % status)
            try:
                fmt, w, h = image_kind(body)
            except MediaRefused as e:
                raise Fail("the image of %s: %s (catalog 4.4)" % (t["key"], e))
            ct = dict((k.lower(), v) for k, v in hdrs).get("content-type", "")
            if ct.split(";")[0].strip().lower() != "image/" + fmt:
                raise Fail("a %s served as %r" % (fmt, ct))
            st["url"] = a.json["url"]
            return "%s: a %dx%d %s of %d bytes, under a ticket of its own" % (t["key"], w, h, fmt.upper(), len(body))
        self.check("I1", "catalog 4.4, media 3.4", "image: 200 signed, <stream root><ticket>/image, a JPEG or PNG within "
                   "1 MiB and 4,096 pixels", image)

        def scope():
            if "url" not in st:
                raise Skip("no image fetched")
            base = st["url"][:-len("image")]
            bad = []
            for name in ("file", "master.m3u8", "trickplay/0", "subtitles/s0/0"):
                status, _, _ = c.get(base + name)
                if status != 404:
                    bad.append("%s under the image ticket: %d, not 404 (media 4: it opens the image only)" % (name, status))
            bad += self.head_405([st["url"]])
            status, _, _ = c.get(st["url"] + "?x=1")
            if status != 404:
                bad.append("the image with a query: %d" % status)
            if bad:
                raise Fail("\n".join(bad))
            return "the image ticket opens nothing else (404); HEAD 405 with Allow: GET; a query 404"
        self.check("I2", "media 4, 2.2, catalog 4.4", "an image ticket opens only its image; HEAD 405", scope)

        def refusals():
            on()
            bad = []
            for name, body, want in (("an array", b"[]", 400), ("no title", b"{}", 400),
                                     ("a title that isn't a string", b'{"title":7}', 400),
                                     ("a key not offered", dumps({"title": "movie/local/conformanceNoSuchTitle"}), 404),
                                     ("a key in no canonical form", dumps({"title": "movie/tmdb/0949"}), 404)):
                a = c.call("image", body)
                self.answers.append(a)
                if not (a.status == want and a.signed):
                    bad.append("%s: %s, not a signed %d" % (name, a.short(), want))
            public = next((t for t in self.whole or [] if scheme_of(t.get("key")) in ("tmdb", "tvdb", "mb")), None)
            if public:
                a = c.call("image", {"title": public["key"]})
                self.answers.append(a)
                if not (a.status == 404 and a.signed):
                    bad.append("%s, not a local title: %s, not a signed 404" % (public["key"], a.short()))
            others = [t for t in self.local_titles() if t.get("image") is not True or t.get("kind") not in (
                "movie", "series", "artist", "album")]
            if others:
                a = c.call("image", {"title": others[0]["key"]})
                self.answers.append(a)
                if not (a.status == 404 and a.signed):
                    bad.append("%s (%s, image %r): %s, not a signed 404" % (others[0]["key"], others[0].get("kind"),
                                                                          others[0].get("image"), a.short()))
            if bad:
                raise Fail("\n".join(bad))
            return "400 for bodies that aren't {title}; 404 for a key not offered%s%s" % (
                ", a public title" if public else "", ", a local title without an image" if others else "")
        self.check("I3", "catalog 4.4", "image refusals: a signed 400 for a malformed body, 404 for a title without "
                   "an image to give", refusals)

    def local_switch(self, libs):
        """T12 (with --watch-local): the operator turns the sharing of local
        titles off, then on again; titles follow, with a whole reading."""
        Fail, Skip = _x()

        def run():
            self.need("catalog")
            ok, why = self.can_ask_operator()
            if not ok:
                raise Skip(why)
            if not self.a.watch_local:
                raise Skip("run with --watch-local SECONDS to have the operator turn the sharing of local titles off and on")
            locals_ = {t["key"] for t in self.local_titles()}
            if not locals_:
                raise Skip("no local title offered: share local titles with this peer first")
            seen = {}

            def keys():
                return {t.get("key") for p in self.read_titles(libs) for t in (p.get("titles") or []) if isinstance(t, dict)}

            def gone():
                if not (keys() & locals_):
                    seen["off"] = True
                    return True
                return False
            seq0 = self.read_titles(libs)[0].get("seq")
            self.ask_operator("local-off", "stop sharing local titles with this peer", gone, self.a.watch_local)
            if "off" not in seen:
                raise Skip("the local titles stayed in the list")
            bad = []
            if seq0 and self.read_titles(libs, since=seq0)[0].get("changes") is True:
                bad.append("since the seq before the switch: a changes answer; catalog 5: turning it off forces a "
                           "whole reading")
            seq1 = self.read_titles(libs)[0].get("seq")
            self.ask_operator("local-on", "share local titles with this peer again", lambda: locals_ <= keys(),
                              self.a.watch_local)
            if not locals_ <= keys():
                bad.append("the local titles didn't come back once shared again")
            elif seq1 and self.read_titles(libs, since=seq1)[0].get("changes") is True:
                bad.append("since the seq before the switch back: a changes answer; catalog 5: a whole reading")
            pages = self.read_titles(libs)
            self.whole = [t for j in pages for t in (j.get("titles") or []) if isinstance(t, dict)]
            self.seq = pages[0].get("seq")
            if bad:
                raise Fail("\n".join(bad))
            return "%d local title(s) left the list when sharing them stopped, and came back, each time with a whole " \
                   "reading" % len(locals_)
        self.check("T12", "catalog 5, video 4.5", "local titles follow the owner's switch, which forces a whole reading",
                   run, operator=True)

    # ---- playback (playback 2-6, video 5, music 5) -------------------------------------

    def play(self, body):
        """A play, paced below the suggested play limit (core 8.3: 30 a
        minute, bursts of 20), and waiting out a signed retryAfter once."""
        now = time.time()
        self._plays = [t for t in getattr(self, "_plays", []) if t > now - 60]
        if len(self._plays) >= 15:
            time.sleep(2.2)
        self._plays.append(time.time())
        a = self.client.call("play", body)
        if a.status == 429 and a.signed:
            ra = a.json.get("retryAfter") if isinstance(a.json, dict) else None
            wait = ra if is_int(ra) and 0 < ra <= 120 else 10
            self.r.note("play answered 429 rate-limited: waiting %d s and trying again once" % wait)
            time.sleep(wait)
            a = self.client.call("play", body)
        return a

    def stop(self, pb):
        if isinstance(pb, dict) and isinstance(pb.get("id"), str):
            try:
                self.client.call("stop", {"id": pb["id"]})
            except OSError:
                pass

    def playable(self):
        """A title play takes, with a copy: a film, an episode or an extra
        (video 5), else a track (music 5)."""
        _, Skip = _x()
        on = self.parts_on()
        if "playback" not in on:
            raise Skip("playback isn't on between the server and this suite")
        if not self.whole:
            raise Skip("no titles read (offer a library with a film or an episode)")
        def score(t):
            """The title that exercises most: a film before an episode, an
            extra and a track; then a copy of one file, with subtitles,
            previews, several audio tracks; a public key before a local one."""
            cp = self.copies_of(t["key"])[0]
            return (("movie", "episode", "extra", "track").index(t["kind"]), len(cp.get("parts") or []) > 1,
                    -len(cp.get("subtitleTracks") or []), cp.get("previews") is not True,
                    -len(cp.get("audioTracks") or []), scheme_of(t["key"]) == "local")
        found = [t for t in self.whole if t.get("kind") in ("movie", "episode", "extra", "track")
                 and ("music" if t["kind"] == "track" else "video") in on
                 and (video_key(t["key"]) or music_key(t["key"])) and self.copies_of(t["key"])]
        if not found:
            raise Skip("no film, episode, extra or track with copies among the titles offered")
        return min(found, key=score)["key"]

    def check_playback(self, j, title, delivery=None):
        """Playback 2.3, media 1, video 5.3 and 5.5, music 5: the problems
        of a play answer, and its ticket."""
        bad = []
        vid = j.get("versionId") if isinstance(j, dict) and isinstance(j.get("versionId"), str) else None
        cat = self.copy_of(title, vid)
        try:
            kept = keep_play_answer(j, self.root, delivery, cat.get("etag") if cat else None)
        except Refused as e:
            return ["refused by the viewer's server (it sends stop and gives up): %s" % e], None
        proto = kept["stream"]["protocol"]
        ticket = stream_ticket(kept["stream"]["url"], self.root, "master.m3u8" if proto == "hls" else "file")
        if len(ticket) < 22:
            bad.append("a ticket of %d characters can't hold 128 random bits (media 1)" % len(ticket))
        for f in ("versions", "versionId", "audioTracks", "audioTrack", "maxBitrateKbps", "durationMs"):
            if f in j and kept.get(f) != j[f] and not (f == "audioTracks" and isinstance(j[f], list)):
                bad.append("%s %s: a viewer's server cleans it to %s (playback 2.3)" % (
                    f, dumps(j[f]).decode()[:120], dumps(kept[f]).decode()[:120] if f in kept else "nothing"))
        for t in j.get("audioTracks") or []:
            if isinstance(t, dict) and "label" in t:
                bad.append("audio track %s carries label (playback 2.3: tracks carry no label)" % t.get("id"))
                break
        if "info" in j and not isinstance(j["info"], dict):
            bad.append("info isn't an object")
        if cat is None:
            bad.append("versionId %r is none of the title's copies in the catalog (playback 2.3: version IDs are the "
                       "copies' ids)" % j.get("versionId"))
        else:
            if kept.get("readTitleAgain"):
                bad.append("etag %r; the catalog's copy %s has %r, and nothing changed on the server (playback 2.3, "
                           "media 3.2: the copy's etag, as the catalog gives it)" % (j["etag"], cat.get("id"), cat.get("etag")))
            if "versionId" not in j and len(self.copies_of(title)) > 1:
                bad.append("no versionId, and the title has %d copies: which plays?" % len(self.copies_of(title)))
            ids = {t.get("id") for t in cat.get("audioTracks") or [] if isinstance(t, dict)} if video_key(title) else set()
            for t in kept.get("audioTracks") if ids else []:
                if t["id"] != "off" and t["id"] not in ids:
                    bad.append("audio track %s isn't one of the copy's in the catalog %s (video 3.2: the same IDs)"
                               % (t["id"], sorted(ids)))
            if kept.get("audioTrack") and ids and kept["audioTrack"] not in ids:
                bad.append("audioTrack %s isn't one of the copy's in the catalog" % kept["audioTrack"])
            vk = video_key(title)
            if "durationMs" in kept and is_int(cat.get("durationMs")) and kept["durationMs"] and not (vk and vk.kind == "episode") \
                    and abs(kept["durationMs"] - cat["durationMs"]) > 1000:
                bad.append("durationMs %d; the catalog's copy %d" % (kept["durationMs"], cat["durationMs"]))
        root = self.root
        if video_key(title):
            subs = self.check_sub_tracks(j, "subtitleTracks", root, ticket, cat, bad)
            self.check_sub_tracks(j, "forcedTracks", root, ticket, cat, bad)
            if "subtitleTrack" in j and j["subtitleTrack"] not in [t.get("id") for t in subs]:
                bad.append("subtitleTrack %r isn't one of subtitleTracks" % j["subtitleTrack"])
            if "subtitleStretchSeconds" in j and j["subtitleStretchSeconds"] != 120:
                bad.append("subtitleStretchSeconds %r, not 120 (video 5.3)" % j["subtitleStretchSeconds"])
            if "segments" in j:
                bad.append("segments in play's answer: skip markers come with the copy, in the catalog (video 5.3)")
            if "fonts" in j:
                fs = j["fonts"]
                if not isinstance(fs, list) or len(fs) > 8 or any(u != "%s%s/fonts/%d" % (root, ticket, i) for i, u in enumerate(fs)):
                    bad.append("fonts %s isn't at most 8 URLs <stream root><ticket>/fonts/<n> (video 5.3, 6.1)" % str(fs)[:120])
            if "burnedTrack" in j and not re.fullmatch(r"s(0|[1-9][0-9]{0,2})", str(j["burnedTrack"])):
                bad.append("burnedTrack %r isn't s<n>" % j["burnedTrack"])
            tp = j.get("trickplay")
            if tp is not None:
                bad += self.check_trickplay(tp, root, ticket)
        else:
            for f in ("subtitleTracks", "forcedTracks", "segments", "trickplay", "fonts"):
                if j.get(f):
                    bad.append("%s in the answer for a track (music 5: audio only; a viewer's server drops it)" % f)
        return bad, ticket

    @staticmethod
    def check_sub_tracks(j, name, root, ticket, cat, bad):
        out = []
        v = j.get(name)
        if v is None:
            return out
        if not isinstance(v, list):
            bad.append("%s isn't an array" % name)
            return out
        if len(v) > 64:
            bad.append("%s holds %d tracks; a viewer's server keeps 64" % (name, len(v)))
        formats = {t.get("id"): t.get("format") for t in (cat or {}).get("subtitleTracks") or [] if isinstance(t, dict)}
        for t in v:
            if not isinstance(t, dict):
                bad.append("a %s entry isn't an object" % name)
                continue
            tid = t.get("id")
            if not isinstance(tid, str) or not re.fullmatch(r"s(0|[1-9][0-9]{0,2})", tid):
                bad.append("%s: ID %r isn't s<n> without leading zeros" % (name, tid))
                continue
            if "label" in t:
                bad.append("%s %s carries label (video 5.3: no label)" % (name, tid))
            if "pictures" in t:
                bad.append("%s %s carries pictures (video 5.3: format says it)" % (name, tid))
            for f in ("default", "forced", "hearingImpaired", "external"):
                if f in t and not isinstance(t[f], bool):
                    bad.append("%s %s: %s isn't a boolean" % (name, tid, f))
            if cat is not None and tid not in formats:
                bad.append("%s %s isn't one of the copy's subtitle tracks in the catalog (video 5.4: the same IDs)"
                           % (name, tid))
            fmt = t.get("format", formats.get(tid))
            if "url" in t:
                kind = "pictures" if fmt in PICTURE_FORMATS else "subtitles"
                if fmt == "other":
                    bad.append("%s %s: a url for a track of format other (video 5.4: neither URL)" % (name, tid))
                elif t["url"] != "%s%s/%s/%s" % (root, ticket, kind, tid):
                    bad.append("%s %s: url %r isn't <stream root><ticket>/%s/%s (video 6.1)" % (name, tid, t["url"], kind, tid))
            if "assUrl" in t:
                if fmt not in ("ass", "ssa"):
                    bad.append("%s %s: assUrl for a track of format %s (video 5.4: ass and ssa only)" % (name, tid, fmt))
                elif t["assUrl"] != "%s%s/ass/%s" % (root, ticket, tid):
                    bad.append("%s %s: assUrl %r isn't <stream root><ticket>/ass/%s (video 6.1)" % (name, tid, t["assUrl"], tid))
            out.append(t)
        return out

    @staticmethod
    def check_trickplay(tp, root, ticket):
        bad = []
        if not isinstance(tp, dict):
            return ["trickplay isn't an object"]
        bounds = {"tileWidth": (1, 2048), "tileHeight": (1, 2048), "columns": (1, 100), "rows": (1, 100),
                  "intervalMs": (100, 3600000), "tileCount": (1, MAX)}
        for f, (lo, hi) in bounds.items():
            v = tp.get(f)
            if not is_int(v):
                bad.append("trickplay.%s %r isn't an integer (a viewer's server refuses the whole answer)" % (f, v))
            elif not lo <= v <= hi:
                bad.append("trickplay.%s %r outside %d to %d" % (f, v, lo, hi))
        if bad:
            return bad
        if tp["columns"] * tp["tileWidth"] > 4096 or tp["rows"] * tp["tileHeight"] > 4096:
            bad.append("a full sheet wider or taller than 4,096 pixels")
        sheets = tp.get("sheets")
        per = tp["columns"] * tp["rows"]
        n = -(-tp["tileCount"] // per)
        if not isinstance(sheets, list):
            return bad + ["trickplay.sheets isn't an array"]
        if len(sheets) != n or n > 1000:
            bad.append("trickplay.sheets has %d entries; ceil(tileCount / (columns x rows)) is %d (at most 1,000)" % (len(sheets), n))
        for i, u in enumerate(sheets):
            if u != "%s%s/trickplay/%d" % (root, ticket, i):
                bad.append("sheets[%d] is %r, not <stream root><ticket>/trickplay/%d (video 6.1)" % (i, u, i))
                break
        return bad

    @staticmethod
    def bad_plays(k):
        """The malformed play requests of vectors/v0/playback.json (those
        answered 400), for title k instead of the vectors' film."""
        v = json.load(open(os.path.join(VECTORS, "playback.json")))
        out = []
        for g in ("requests", "deliveries", "downloads", "scans"):
            for c in v[g]:
                if c.get("call") != "play" or c["valid"] or c["status"] != 400:
                    continue
                text = c["text"]
                if not video_key(k) and re.search(r"subtitle|burnSubtitle|videoCodec|maxHeight|dvProfiles", text):
                    continue
                out.append((c["name"], text.replace('"movie/tmdb/949"', json.dumps(k)).encode()))
        return out

    def playback_checks(self):
        Fail, Skip = _x()
        c = self.client
        st = {}

        def title():
            if "key" not in st:
                st["key"] = self.playable()
            return st["key"]

        def bad_bodies():
            k = title()
            forms = self.bad_plays(k)
            bad = []
            for name, b in forms:
                a = self.play(b)
                self.answers.append(a)
                if a.status == 200:
                    self.stop(a.json)
                if not (a.status == 400 and a.signed and a.problem_type() == "invalid-request"):
                    bad.append("%s: %s" % (name, a.short()))
            if bad:
                raise Fail("expected a signed 400 invalid-request for:\n" + "\n".join(bad))
            return "%d malformed plays (the 400s of vectors/v0/playback.json, for %s) refused with a signed 400" % (len(forms), k)
        self.check("PB1", "playback 2.1, 2.2, 2.7, 7, 8; video 5.1", "play with a malformed member, delivery or purpose: "
                   "the signed 400 invalid-request", bad_bodies)

        def not_held():
            title()
            k = "movie/tmdb/9999999999" if "video" in self.parts_on() else "track/mb/00000000-0000-4000-8000-000000000000"
            a = self.play({"title": k})
            if a.status == 200:
                self.stop(a.json)
            self.signed(a, 404, "not-found")
            return "signed 404 not-found for %s" % k
        self.check("PB2", "playback 2.2", "play of a key the server doesn't offer: a signed 404", not_held)

        def plain():
            k = title()
            a = self.play({"title": k})
            if a.status != 200:
                raise Fail("play %s: %s (offer a playable copy; limits may be 0)" % (k, a.short()))
            self.signed(a, 200)
            bad, ticket = self.check_playback(a.json, k)
            st["pb"], st["ticket"] = a.json, ticket
            if bad:
                raise Fail("\n".join(bad))
            j = a.json
            return "%s: %s over %s, etag %s as the catalog's, %d audio, %d subtitle tracks%s" % (
                j["id"], j["method"], j["stream"]["protocol"], j.get("etag"), len(j.get("audioTracks") or []),
                len(j.get("subtitleTracks") or []), ", previews" if j.get("trickplay") else "")
        if not self.check("PB3", "playback 2.3, media 1, video 3.2, 5.3, 5.5", "play: 200 signed, the playback object in "
                          "its forms, URLs under the stream root, its etag and tracks the catalog's", plain):
            self.stop(st.get("pb"))
            return

        def nothing_said():
            j = st["pb"]
            if j["method"] == "direct" or j["stream"]["protocol"] != "hls":
                raise Fail("with no capabilities and no delivery, %s over %s: nothing said means no direct play, "
                           "H.264 and AAC (or AAC alone for a track) in HLS" % (j["method"], j["stream"]["protocol"]))
            return "%s over hls" % j["method"]
        self.check("PB4", "video 5.1, music 5", "play with nothing said: not direct, HLS", nothing_said)

        self.media_checks(st["pb"], st["key"], "M")

        def progress():
            j = st["pb"]
            now = int(time.time())
            for b in ({"id": j["id"], "positionMs": 1000}, {"id": j["id"], "positionMs": 2000, "paused": True}, {"id": j["id"]},
                      {"id": j["id"], "positionMs": 2500, "at": now - 60}):
                a = self.signed(c.call("progress", b), 200)
                if a.json != {}:
                    raise Fail("progress answered %r, not {}" % a.body[:80])
            bad = []
            for name, b in (("an id of another form", {"id": "play_x"}), ("no id", {}),
                            ("positionMs -1", {"id": j["id"], "positionMs": -1}),
                            ("positionMs over 100 days", {"id": j["id"], "positionMs": 8640000001}),
                            ("paused not a boolean", {"id": j["id"], "paused": "yes"}),
                            ("positionMs null", {"id": j["id"], "positionMs": None}),
                            ("at over 30 days ago", {"id": j["id"], "at": now - 30 * 86400 - 600}),
                            ("at over 5 minutes ahead", {"id": j["id"], "at": now + 900}),
                            ("at in milliseconds", {"id": j["id"], "at": now * 1000}),
                            ("at as a string", {"id": j["id"], "at": str(now)}),
                            ("at null", {"id": j["id"], "at": None}),
                            ("another title than the playback's", {"id": j["id"], "title": "movie/tmdb/9999999998"})):
                a = c.call("progress", b)
                self.answers.append(a)
                if not (a.status == 400 and a.signed):
                    bad.append("%s: %s" % (name, a.short()))
            a = c.call("progress", {"id": "play_" + "a" * 26})
            try:
                self.signed(a, 410, "playback-stopped")
            except Fail as e:
                bad.append("a playback ID never given: %s" % e)
            if bad:
                raise Fail("\n".join(bad))
            return "200 {} four times (one with at a minute ago); 400 for malformed bodies and at out of its bounds; " \
                   "410 playback-stopped for an ID never given"
        self.check("PB5", "playback 3, 6.1, music 5.2", "progress: 200 {}, 400 for a bad body or at, 410 for a playback "
                   "that isn't live", progress)

        def stop():
            j = st["pb"]
            a = self.signed(c.call("stop", {"id": j["id"], "positionMs": 3000, "at": int(time.time()) - 5}), 200)
            if a.json != {}:
                raise Fail("stop answered %r" % a.body[:80])
            self.signed(c.call("stop", {"id": j["id"]}), 200)
            self.signed(c.call("stop", {"id": "play_" + "b" * 26}), 200)
            bad = []
            a = c.call("progress", {"id": j["id"]})
            try:
                self.signed(a, 410, "playback-stopped")
            except Fail as e:
                bad.append("progress after stop: %s" % e)
            for name, b in (("positionMs -1", {"id": j["id"], "positionMs": -1}), ("no id", {}),
                            ("at over 5 minutes ahead", {"id": j["id"], "at": int(time.time()) + 900})):
                a = c.call("stop", b)
                self.answers.append(a)
                if not (a.status == 400 and a.signed):
                    bad.append("stop with %s: %s" % (name, a.short()))
            status, _, _ = c.get(j["stream"]["url"])
            if status != 404:
                bad.append("the ticket still works after stop: %d for %s (media 4: it stops at once)" % (status, "the stream"))
            if bad:
                raise Fail("\n".join(bad))
            return "200 {} (with at, again, and for an ID never given); 400 for at out of its bounds; then progress 410 " \
                   "and the media 404"
        self.check("PB6", "playback 4, 6.1, 6.3, media 4", "stop: 200 {} even when repeated; the playback ends at once", stop)
        self.stop(st["pb"])
        for later in getattr(self, "_subs_later", []):
            self.check(*later)
        self._subs_later = []

        self.track_checks(st["key"], st["pb"])
        self.limit_checks(st["key"])
        self.revision_checks(st["key"])

    def track_checks(self, k, pb):
        Fail, Skip = _x()

        def run(body):
            a = self.play(dict(body, title=k))
            self.signed(a, 200)
            self.stop(a.json)
            return a.json

        cat = self.copy_of(k, pb.get("versionId")) or {}
        audio = [t for t in pb.get("audioTracks") or cat.get("audioTracks") or [] if isinstance(t, dict)]
        subs = [t for t in pb.get("subtitleTracks") or cat.get("subtitleTracks") or [] if isinstance(t, dict)]

        def audio_rules():
            if not audio:
                raise Skip("neither the answer nor the catalog's copy lists audio tracks")
            out = []
            j = run({"audioTrack": "a999"})
            if j.get("audioTrack") != pb.get("audioTrack"):
                raise Fail("audioTrack a999 (not of the copy) gave %r; absent gave %r: it must count as absent"
                           % (j.get("audioTrack"), pb.get("audioTrack")))
            out.append("a999 counts as absent")
            others = [t for t in audio if t.get("id") != pb.get("audioTrack")]
            if others:
                t = others[0]
                j = run({"audioTrack": t["id"]})
                if j.get("audioTrack") != t["id"]:
                    raise Fail("audioTrack %s gave %r" % (t["id"], j.get("audioTrack")))
                out.append("%s chosen" % t["id"])
                default = next((x for x in audio if x.get("id") == pb.get("audioTrack")), {})
                lang = next((x for x in others if x.get("language") and x.get("language") != default.get("language")
                             and not x.get("role")), None)
                if lang:
                    j = run({"audioLanguage": lang["language"]})
                    got = next((x for x in audio if x.get("id") == j.get("audioTrack")), {})
                    if (got.get("language") or "").lower().split("-")[0] != lang["language"].lower().split("-")[0]:
                        raise Fail("audioLanguage %s gave %r (%s)" % (lang["language"], j.get("audioTrack"), got.get("language")))
                    if got.get("role"):
                        raise Fail("audioLanguage %s chose a %s track" % (lang["language"], got["role"]))
                    out.append("audioLanguage %s gives a track in it" % lang["language"])
            else:
                out.append("one audio track only")
            return "; ".join(out)
        self.check("PB7", "playback 2.6", "the audio: a track not of the copy counts as absent; a track named; a language", audio_rules)

        def subtitle_rules():
            if not video_key(k):
                raise Skip("not a film, an episode or an extra")
            out = []
            j = run({"subtitleTrack": "off"})
            if "subtitleTrack" in j:
                raise Fail("subtitleTrack off gave subtitleTrack %r: none shown" % j["subtitleTrack"])
            out.append("off shows none")
            j = run({"subtitleTrack": "s999"})
            if j.get("subtitleTrack") != pb.get("subtitleTrack"):
                raise Fail("subtitleTrack s999 gave %r; absent gave %r" % (j.get("subtitleTrack"), pb.get("subtitleTrack")))
            out.append("s999 counts as absent")
            if subs:
                t = subs[-1]
                j = run({"subtitleTrack": t["id"]})
                if j.get("subtitleTrack") != t["id"]:
                    raise Fail("subtitleTrack %s gave %r" % (t["id"], j.get("subtitleTrack")))
                out.append("%s chosen" % t["id"])
                lt = next((x for x in subs if x.get("language") and not x.get("forced")), None)
                if lt:
                    j = run({"subtitleLanguage": lt["language"]})
                    got = next((x for x in subs if x.get("id") == j.get("subtitleTrack")), {})
                    if (got.get("language") or "").lower().split("-")[0] != lt["language"].lower().split("-")[0]:
                        raise Fail("subtitleLanguage %s gave %r" % (lt["language"], j.get("subtitleTrack")))
                    out.append("subtitleLanguage %s gives a track in it" % lt["language"])
            if j.get("method") == "transcode" and pb.get("method") != "transcode":
                raise Fail("choosing subtitles made the playback transcode (video 5.4: burned in only when asked)")
            return "; ".join(out)
        self.check("PB8", "video 5.4", "the subtitles: off, a track not of the copy, a track named, a language", subtitle_rules)

        def version_rules():
            out = []
            j = run({"versionId": "conformance-no-such-version"})
            if j.get("versionId") != pb.get("versionId"):
                raise Fail("an unknown versionId gave %r; absent gave %r (playback 2.6: as absent)" % (j.get("versionId"), pb.get("versionId")))
            out.append("an unknown versionId counts as absent")
            vs = [x["id"] for x in self.copies_of(k) if x.get("id") != pb.get("versionId")]
            if vs:
                j = run({"versionId": vs[0]})
                if j.get("versionId") != vs[0]:
                    raise Fail("versionId %s (a copy in the catalog) gave %r" % (vs[0], j.get("versionId")))
                out.append("%s chosen" % vs[0])
            else:
                out.append("one copy only")
            first = self.copies_of(k)[0].get("id") if self.copies_of(k) else None
            if pb.get("versionId", first) != first:
                raise Fail("without versionId, %r played; the catalog's first copy is %r (video 3.7: the first is the "
                           "one play chooses)" % (pb.get("versionId"), first))
            return "; ".join(out) + "; without versionId, the catalog's first copy"
        self.check("PB9", "playback 2.6, video 3.7", "the copy: an unknown versionId counts as absent; a copy named; "
                   "the first copy by default", version_rules)

    def limit_checks(self, k):
        Fail, Skip = _x()
        lim = self.limits if isinstance(self.limits, dict) else None

        def over():
            if lim is None or not is_int(lim.get("streams")):
                raise Skip("the server sends no limits")
            n = lim["streams"]
            if n > 4:
                raise Skip("streams is %d: the suite starts at most 4 playbacks" % n)
            live = []
            try:
                for i in range(n):
                    a = self.play({"title": k, "capabilities": BROAD_CAPS})
                    self.signed(a, 200)
                    live.append(a.json)
                a = self.play({"title": k, "capabilities": BROAD_CAPS})
                if a.status == 200:
                    live.append(a.json)
                    raise Fail("play number %d of a pairing whose streams is %d was answered 200" % (n + 1, n))
                if a.status == 503:
                    self.signed(a, 503, "server-busy")
                    return (PASS, "the %d+1th play: 503 server-busy (it needed a transcode)" % n)
                self.signed(a, 409, "stream-limit-reached")
                return "streams %d: the next play gets 409 stream-limit-reached" % n
            finally:
                for j in live:
                    self.stop(j)
        self.check("PB10", "playback 2.2, 2.5, 6", "past the pairing's streams: 409 stream-limit-reached", over)

        def race():
            if lim is None or not is_int(lim.get("streams")) or not 1 <= lim["streams"] <= 4:
                raise Skip("needs limits with streams 1 to 4")
            n = lim["streams"]
            live = []
            try:
                for _ in range(n - 1):
                    a = self.play({"title": k, "capabilities": BROAD_CAPS})
                    self.signed(a, 200)
                    live.append(a.json)
                with concurrent.futures.ThreadPoolExecutor(2) as ex:
                    res = list(ex.map(lambda _: self.client.call("play", {"title": k, "capabilities": BROAD_CAPS}), range(2)))
                ok = [a for a in res if a.status == 200]
                live += [a.json for a in ok]
                if len(ok) != 1:
                    raise Fail("two plays for the last stream: %s (playback 6.2: exactly one wins)" % [a.short() for a in res])
                return "two plays at once for the last stream: one 200, one %d" % [a for a in res if a.status != 200][0].status
            finally:
                for j in live:
                    self.stop(j)
        self.check("PB11", "playback 6.2", "two plays at once for the last stream: exactly one starts", race)
