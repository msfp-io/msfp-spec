"""What a viewer's server keeps of a play answer (spec/v0/playback.md
section 2.3, as revised by proposals 16 and 17) and the URLs it builds
from the answer's ticket (media 1), of a libraries answer's limits
(playback 5), and what it does on a restart (playback 2.7), verified
against vectors/v0/playback.json (answers, limits, restarts). The
requests, deliveries, baseline, queueEntries, downloads, scans and
reports are an owning server's decisions, which the fake makes (fake.py)
and the live checks test."""

import json
import re

from .copies import OPAQUE, dumps, int_in, loads

PLAY_ID = re.compile(r"[A-Za-z0-9_-]{1,64}")   # opaque (proposal 17), unguessable
AUDIO_ID = re.compile(r"a(0|[1-9][0-9]{0,2})")
MAX_POSITION = 8640000000
MAX_KBPS = 10000000
METHODS = ("direct", "remux", "transcode")
HLS_TYPE = "application/vnd.apple.mpegurl"
# delivery asked -> the method the answer must give (playback 2.7)
DELIVERY_METHOD = {"file": "direct", "remux": "remux", "transcode": "transcode"}
SCAN_FLOOR, SCAN_TOP = 4 * 1024 * 1024, 1024 * 1024 * 1024
STREAM_NAME = {"hls": "master.m3u8", "file": "file"}


class Refused(ValueError):
    """The viewer's server sends stop and gives up (playback 2.3)."""


def _show(v):
    return json.dumps(v) if isinstance(v, str) else repr(v)


def _int(v):
    """An MSFP integer (core 2.3) of any size within 2^53, or None."""
    return int_in(v, -2**53 + 1, 2**53 - 1)


def keep_play_answer(o, root, delivery=None, catalog_etag=None, purpose=None, part="video", catalog_subtitles=()):
    """Playback 2.3 and video 5.3: what the viewer's server keeps of a play
    answer, and urls, the URLs it builds from the stream root and the
    ticket (the stream; each subtitle track's subtitles/, ass/ or
    pictures/ base; each font), a dict in the vectors' order, with
    readTitleAgain when the etag isn't the catalog's; raises Refused when
    it sends stop and gives up. Answers carry no URL (proposal 17): any
    member it doesn't define, a url among them, is ignored."""
    if not isinstance(o, dict):
        raise Refused("not a JSON object")

    def text(name):
        if name not in o:
            raise Refused("no %s" % name)
        if not isinstance(o[name], str):
            raise Refused("%s isn't a string" % name)
        return o[name]

    def formed(name):
        v = text(name)
        if not OPAQUE.fullmatch(v):
            raise Refused("%s doesn't match ^[A-Za-z0-9_-]{1,64}$" % name)
        return v
    out = {"id": formed("id"), "ticket": formed("ticket")}
    proto = text("protocol")
    if proto not in ("hls", "file"):
        raise Refused("protocol %s isn't hls or file" % _show(proto))
    want = {None: "hls", "": "hls", "file": "file", "remux": "hls", "transcode": "hls"}.get(delivery, "hls")
    if proto != want:
        raise Refused("protocol %s for delivery %s: %s only" % (proto, _show(delivery or ""), want))
    out["protocol"] = proto
    m = text("method")
    if m not in METHODS:
        raise Refused("method %s" % _show(m))
    if delivery in DELIVERY_METHOD and m != DELIVERY_METHOD[delivery]:
        raise Refused("method %s for delivery %s" % (m, delivery))
    out["method"] = m
    et = text("etag")
    if not OPAQUE.fullmatch(et):
        raise Refused("etag isn't 1 to 64 characters of A-Z a-z 0-9 - _")
    out["etag"] = et
    if "positionMs" not in o:
        raise Refused("no positionMs")
    pos = _int(o["positionMs"])
    if pos is None:
        raise Refused("positionMs isn't an integer")
    if not 0 <= pos <= MAX_POSITION:
        raise Refused("positionMs out of 0 to 100 days")
    out["positionMs"] = pos
    out["versionId"] = formed("versionId")
    if "audioTrack" in o:
        if not isinstance(o["audioTrack"], str):
            raise Refused("audioTrack isn't a string")
        if AUDIO_ID.fullmatch(o["audioTrack"]):
            out["audioTrack"] = o["audioTrack"]
    if "audioRenditions" in o:
        if not isinstance(o["audioRenditions"], bool):
            raise Refused("audioRenditions isn't a boolean")
        if o["audioRenditions"] and proto == "hls":
            out["audioRenditions"] = True
    if "maxBitrateKbps" in o:
        b = _int(o["maxBitrateKbps"])
        if b is None:
            raise Refused("maxBitrateKbps isn't an integer")
        if 0 <= b <= MAX_KBPS:
            out["maxBitrateKbps"] = b
    if purpose == "scan":
        n = _int(o.get("scanBytes"))
        out["scanBytes"] = n if n is not None and SCAN_FLOOR <= n <= SCAN_TOP else SCAN_FLOOR
    base = root + out["ticket"] + "/"
    urls = [base + STREAM_NAME[proto]]
    if part == "video":
        subs = []
        for e in o.get("subtitles") if isinstance(o.get("subtitles"), list) else []:
            if not isinstance(e, dict):
                continue
            sid, form = e.get("id"), e.get("form")
            if not isinstance(sid, str) or sid not in catalog_subtitles or any(x["id"] == sid for x in subs) \
                    or form not in ("text", "picture"):
                continue
            x = {"id": sid, "form": form}
            if e.get("ass") is True and form == "text":
                x["ass"] = True
            subs.append(x)
        if subs:
            out["subtitles"] = subs
        n = _int(o.get("fontCount"))
        if n is not None and 0 < n <= 64:
            out["fontCount"] = n
        for x in subs:
            if x["form"] == "picture":
                urls.append(base + "pictures/" + x["id"])
                continue
            urls.append(base + "subtitles/" + x["id"])
            if x.get("ass"):
                urls.append(base + "ass/" + x["id"])
        urls += [base + "fonts/%d" % i for i in range(out.get("fontCount", 0))]
    out["urls"] = urls
    if catalog_etag and et != catalog_etag:
        out["readTitleAgain"] = True
    return out


# ---- limits (playback 5, music 5.1) ------------------------------------------------

MAX_STREAMS = 1000
LIMITS = (("streams", 0, MAX_STREAMS), ("transcodes", 0, MAX_STREAMS), ("bitrateKbps", 0, 1000000),
          ("downloads", 0, MAX_STREAMS), ("downloadsPerDay", 0, 10000), ("downloadKbps", 0, 1000000),
          ("scansPerDay", 0, 1000000), ("audioStreams", 0, MAX_STREAMS), ("audioTranscodes", 0, MAX_STREAMS))
_ALWAYS = ("downloads", "downloadsPerDay", "downloadKbps", "scansPerDay")


def keep_limits(o):
    """Playback 5: what a caller keeps of limits (downloads, its daily cap
    and speed, scansPerDay 0 when absent; then offersDownloads and
    offersScans), or None when it ignores the whole object. scanBytes is
    no member of limits since proposal 17 (a scan's answer states it):
    unknown, ignored whatever it holds."""
    if not isinstance(o, dict):
        return None
    for f, _, _ in LIMITS:
        if f in o and _int(o[f]) is None:
            return None
    kept = {}
    for f, lo, hi in LIMITS:
        if f in o:
            v = int_in(o[f], lo, hi)
            if v is not None:
                kept[f] = v
    for f, top in (("transcodes", "streams"), ("audioTranscodes", "audioStreams")):
        if f in kept and top in kept and kept[f] > kept[top]:
            del kept[f]
    out = {}
    for f, _, _ in LIMITS:
        if f in _ALWAYS:
            out[f] = kept.get(f, 0)
        elif f in kept:
            out[f] = kept[f]
    out["offersDownloads"] = out["downloads"] > 0
    out["offersScans"] = out["scansPerDay"] > 0
    return out


# ---- restarts (playback 2.7) --------------------------------------------------------

def restart(reading, request, status, answer, root):
    """What a viewer's server reading reading's file does with the answer
    to a new play: {"join": True, "offset": n} to go on at the same byte,
    else join false with stopNew, failRead, readTitles as they apply."""
    fail = {"join": False, "failRead": True, "readTitles": True}
    rp = reading.get("purpose") or "play"
    if reading.get("delivery") != "file" or not isinstance(request, dict):
        return fail
    purpose = request.get("purpose") if isinstance(request.get("purpose"), str) and request.get("purpose") else "play"
    if request.get("versionId") != reading.get("versionId") or purpose != rp or request.get("delivery") != "file":
        return fail
    if status != 200:
        return fail
    stop = {"join": False, "stopNew": True, "failRead": True, "readTitles": True}
    try:
        kept = keep_play_answer(answer, root, delivery="file", purpose=rp)
    except Refused:
        return stop
    if kept["etag"] != reading.get("etag"):
        return stop
    return {"join": True, "offset": reading["offset"]}


def playback_vectors_ok(path):
    """Checks keep_play_answer, keep_limits and restart against
    vectors/v0/playback.json: a list of mismatches."""
    v = json.load(open(path))
    root = v["context"]["streamRoot"]
    bad = []
    for c in v["answers"]:
        ask = c.get("asked", {})
        try:
            out, ok = dumps(keep_play_answer(loads(c["text"]), root, ask.get("delivery"), ask.get("catalogEtag"),
                                             ask.get("purpose"), ask.get("part"), ask.get("catalogSubtitles") or ())), True
        except (Refused, ValueError) as e:
            out, ok = str(e), False
        if ok != c["valid"] or (ok and out != c["output"]):
            bad.append("playback answers %s: %s %s" % (c["name"], ok, out))
    for c in v["limits"]:
        try:
            o = loads(c["text"])
        except ValueError:
            o = None
        k = keep_limits(o)
        if (k is not None) != c["valid"] or (k is not None and dumps(k) != c["output"]):
            bad.append("playback limits %s: %s" % (c["name"], k))
    for c in v["restarts"]:
        ans = loads(c["answer"]) if c.get("answer") else None
        got = restart(c["reading"], loads(c["request"]), c["status"], ans, root)
        if got["join"] != c["valid"] or dumps(got) != c["output"]:
            bad.append("playback restarts %s: %s" % (c["name"], dumps(got)))
    return bad
