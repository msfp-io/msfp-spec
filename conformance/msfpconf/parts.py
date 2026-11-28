"""MSFP's checks of the parts that carry MSFP Media (spec/v0/catalog.md,
playback.md, media.md): a libraries answer's limits (playback 5), the
owner's switch of local titles (catalog 5), a race for the last stream
(playback 6.2) and the purpose scan (playback 8, media 4.2). MSFP
Media's checks are msfp-media's (msfpmedia.checks); the carrier
(carrier.py) runs these in their place among them."""

import concurrent.futures

from msfpmedia.base import dumps, is_int
from msfpmedia.checks import MediaChecks
from msfpmedia.report import PASS

from .carrier import NO_LIMITS, READ_AT_MOST, SCAN_FLOOR, keep_limits, scan_answer

LIMIT_FIELDS = ("streams", "transcodes", "bitrateKbps", "downloads", "downloadsPerDay", "downloadKbps", "scansPerDay",
                "audioStreams", "audioTranscodes")


def _x():
    from .checks import Fail, Skip
    return Fail, Skip


def _h(hdrs, name):
    return dict((k.lower(), v) for k, v in hdrs).get(name.lower())


class PartChecks(MediaChecks):

    # ---- the libraries' limits (playback 5, music 5.1) ----------------------------------

    def limits_checks(self):
        Fail, Skip = _x()
        self.limits = (self.libraries_answer or {}).get("limits")

        def limits():
            self.need("catalog")
            lim = self.limits
            if "playback" not in self.parts_on():
                if lim is not None:
                    raise Fail("limits sent while playback is off (MSFP Media listing 2: only when it is on)")
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
            if "scanBytes" in lim:
                self.note_default("playback 5", "limits carries scanBytes, which left limits with proposal 17 (a scan's "
                                  "answer states it; ignored by a caller)")
            return "limits %s; downloads %s, scans %s" % (
                dumps(lim).decode(), "offered" if kept["offersDownloads"] else "off",
                "offered" if kept["offersScans"] else "off")
        self.check("L2", "playback 5; music 5.1", "libraries' limits: only with playback on, each in its range "
                   "(streams and downloads to 1,000, scans)", limits)


    # ---- local titles (catalog 5) ---------------------------------------------------------

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
        self.check("T12", "catalog 5; MSFP Media video 4.5", "local titles follow the owner's switch, which forces a whole reading",
                   run, operator=True)

    # ---- the last stream (playback 6.2) -----------------------------------------------------

    def race_checks(self, k):
        Fail, Skip = _x()
        lim = self.limits if isinstance(self.limits, dict) else None

        def race():
            if lim is None or not is_int(lim.get("streams")) or not 1 <= lim["streams"] <= 4:
                raise Skip("needs limits with streams 1 to 4")
            n = lim["streams"]
            live = []
            try:
                for _ in range(n - 1):
                    a = self.play(self.stream_body(k))
                    self.signed(a, 200)
                    live.append(a.json)
                with concurrent.futures.ThreadPoolExecutor(2) as ex:
                    body = self.stream_body(k)
                    res = list(ex.map(lambda _: self.client.call("play", body), range(2)))
                ok = [a for a in res if a.status == 200]
                live += [a.json for a in ok]
                if len(ok) != 1:
                    raise Fail("two plays for the last stream: %s (playback 6.2: exactly one wins)" % [a.short() for a in res])
                return "two plays at once for the last stream: one 200, one %d" % [a for a in res if a.status != 200][0].status
            finally:
                for j in live:
                    self.stop(j)
        self.check("PB11", "playback 6.2", "two plays at once for the last stream: exactly one starts", race)

    # ---- scan access (playback 8, media 4.2) --------------------------------------------

    def scan_checks(self, k):
        Fail, Skip = _x()
        lim = keep_limits(self.limits) if isinstance(self.limits, dict) else dict(NO_LIMITS)
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
        self.check("PB21", "playback 8, 5, 2; MSFP Media playing 2.2", "scans: 409 stream-limit-reached when off; else 200 the file, "
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
        self.check("PB22", "playback 8, 2; media 4.2", "a scan's reads: within the scan answer's scanBytes (absent: "
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
                    bad.append("%s under the scan ticket: %d, not 404 (MSFP Media fetching 4.2: file only)" % (name, s))
            bad += self.head_405([j["url"]])
            a = c.call("progress", {"id": j["id"]})
            self.answers.append(a)
            if a.status != 200 or not a.signed:
                bad.append("progress of a scan: %s, not a signed 200 (MSFP Media playing 3)" % a.short())
            self.signed(c.call("stop", {"id": j["id"]}), 200)
            if bad:
                raise Fail("\n".join(bad))
            return "the scan ticket opens the file only; HEAD 405; progress 200; stop 200"
        self.check("PB23", "media 4; MSFP Media fetching 4.2, 2.2; playback 3, 8", "a scan's ticket opens the file only; progress answered 200",
                   ticket)
        self.stop(st.get("pb"))
