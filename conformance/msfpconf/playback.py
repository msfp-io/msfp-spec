"""What a viewer's server keeps of a play answer (spec/v0/playback.md
section 2.3, as revised by proposal 16), of a libraries answer's limits
(playback section 5), and what it does on a restart (playback section
2.7), verified against vectors/v0/playback.json (answers, limits,
restarts). The members video and music add to an answer are theirs
(copies.py, music.py)."""

import json
import re

from .copies import OPAQUE, boolean, dumps, int_in, loads, token, valid_tag

PLAY_ID = re.compile(r"[A-Za-z0-9_-]{1,64}")   # opaque (proposal 17), unguessable
TRACK_ID = re.compile(r"[A-Za-z0-9_-]{1,64}")
MAX_POSITION = 8640000000
METHODS = ("direct", "remux", "transcode")
HLS_TYPE = "application/vnd.apple.mpegurl"
# delivery asked -> the method the answer must give (playback 2.7)
DELIVERY_METHOD = {"file": "direct", "remux": "remux", "transcode": "transcode"}


class Refused(ValueError):
    """The viewer's server sends stop and gives up (playback 2.3)."""


def stream_ticket(url, root, name):
    """The ticket of <root><ticket>/<name> (media 1), or None."""
    if not isinstance(url, str) or not url.startswith(root):
        return None
    rest = url[len(root):].split("/")
    if len(rest) != 2 or rest[1] != name or not OPAQUE.fullmatch(rest[0]):
        return None
    return rest[0]


def keep_stream(s, root, method=None):
    """Media 1's checks of an answer's stream: the kept stream; raises Refused."""
    if not isinstance(s, dict):
        raise Refused("stream missing or not an object")
    proto = s.get("protocol")
    if proto not in ("hls", "file"):
        raise Refused("stream.protocol %s isn't hls or file" % _show(proto))
    name = "master.m3u8" if proto == "hls" else "file"
    if stream_ticket(s.get("url"), root, name) is None:
        raise Refused("stream.url isn't <stream root><ticket>/%s" % name)
    if proto == "file" and method in ("remux", "transcode"):
        raise Refused("a remux or a transcode served as file")
    out = {"url": s["url"], "protocol": proto}
    if "mimeType" in s:
        mt = s["mimeType"]
        if not isinstance(mt, str):
            raise Refused("stream.mimeType isn't a string")
        if proto == "hls" and mt != HLS_TYPE:
            raise Refused("an hls stream.mimeType other than %s" % HLS_TYPE)
        out["mimeType"] = mt
    return out


def _show(v):
    return json.dumps(v) if isinstance(v, str) else repr(v)


def _keep_track(t):
    """A track of playback 2.3: None when dropped (its ID out of form)."""
    if not isinstance(t, dict):
        return None
    tid = t.get("id")
    if not isinstance(tid, str) or not (tid == "off" or TRACK_ID.fullmatch(tid)):
        return None
    out = {"id": tid}
    if valid_tag(t.get("language")):
        out["language"] = t["language"]
    ch = int_in(t.get("channels"), 1, 64)
    if ch is not None:
        out["channels"] = ch
    if token(t.get("role")):
        out["role"] = t["role"]
    for f in ("default", "forced", "hearingImpaired"):
        if boolean(t.get(f)) is not None:
            out[f] = t[f]
    return out


def keep_play_answer(o, root, delivery=None, catalog_etag=None):
    """Playback 2.3: what the viewer's server keeps of playback's own
    members of a play answer (a dict in the vectors' order, with
    readTitleAgain when the etag isn't the catalog's); raises Refused
    when it sends stop and gives up."""
    if not isinstance(o, dict):
        raise Refused("not a JSON object")
    if "id" not in o:
        raise Refused("no id")
    if not isinstance(o["id"], str) or not PLAY_ID.fullmatch(o["id"]):
        raise Refused("id doesn't match ^[A-Za-z0-9_-]{1,64}$")
    if "method" not in o:
        raise Refused("no method")
    m = o["method"]
    if m not in METHODS:
        raise Refused("method %s" % _show(m))
    if delivery in DELIVERY_METHOD and m != DELIVERY_METHOD[delivery]:
        raise Refused("method %s for delivery %s" % (m, delivery))
    stream = keep_stream(o.get("stream"), root, m)
    if delivery == "file" and stream["protocol"] != "file":
        raise Refused("protocol %s for delivery file" % stream["protocol"])
    if delivery in ("remux", "transcode") and stream["protocol"] != "hls":
        raise Refused("protocol %s for delivery %s" % (stream["protocol"], delivery))
    if not delivery and stream["protocol"] != "hls":
        raise Refused("without delivery, the baseline: hls only")
    if "etag" not in o:
        raise Refused("no etag")
    if not isinstance(o["etag"], str):
        raise Refused("etag isn't a string")
    if not OPAQUE.fullmatch(o["etag"]):
        raise Refused("etag isn't 1 to 64 characters of A-Z a-z 0-9 - _")
    if "positionMs" not in o:
        raise Refused("no positionMs")
    if int_in(o["positionMs"], -2**53, 2**53) is None:
        raise Refused("positionMs isn't an integer")
    pos = int_in(o["positionMs"], 0, MAX_POSITION)
    if pos is None:
        raise Refused("positionMs out of 0 to 100 days")
    out = {"id": o["id"], "method": m, "stream": stream, "etag": o["etag"], "positionMs": pos}
    if "durationMs" in o:
        if int_in(o["durationMs"], -2**53, 2**53) is None:
            raise Refused("durationMs isn't an integer")
        d = int_in(o["durationMs"], 0, MAX_POSITION)
        out["durationMs"] = 0 if d is None else d
    if "versionId" in o:
        if not isinstance(o["versionId"], str):
            raise Refused("versionId isn't a string")
        if TRACK_ID.fullmatch(o["versionId"]):
            out["versionId"] = o["versionId"]
    if "audioTracks" in o:
        if not isinstance(o["audioTracks"], list):
            raise Refused("audioTracks isn't an array")
        out["audioTracks"] = [x for x in map(_keep_track, o["audioTracks"]) if x is not None]
    if "audioTrack" in o:
        if not isinstance(o["audioTrack"], str):
            raise Refused("audioTrack isn't a string")
        if o["audioTrack"] == "off" or TRACK_ID.fullmatch(o["audioTrack"]):
            out["audioTrack"] = o["audioTrack"]
    if "audioRenditions" in o:
        if not isinstance(o["audioRenditions"], bool):
            raise Refused("audioRenditions isn't a boolean")
        if o["audioRenditions"] and stream["protocol"] == "hls":
            out["audioRenditions"] = True
    if "maxBitrateKbps" in o:
        if int_in(o["maxBitrateKbps"], -2**53, 2**53) is None:
            raise Refused("maxBitrateKbps isn't an integer")
        b = int_in(o["maxBitrateKbps"], 0, 10000000)
        if b is not None:
            out["maxBitrateKbps"] = b
    if catalog_etag is not None and o["etag"] != catalog_etag:
        out["readTitleAgain"] = True
    return out


# ---- limits (playback 5, music 5.1) ------------------------------------------------

MAX_STREAMS = 1000
SCAN_FLOOR, SCAN_TOP = 4 * 1024 * 1024, 1024 * 1024 * 1024
LIMITS = (("streams", 0, MAX_STREAMS), ("transcodes", 0, MAX_STREAMS), ("bitrateKbps", 0, 1000000),
          ("downloads", 0, MAX_STREAMS), ("downloadsPerDay", 0, 10000), ("downloadKbps", 0, 1000000),
          ("scansPerDay", 0, 1000000), ("scanBytes", SCAN_FLOOR, SCAN_TOP), ("audioStreams", 0, MAX_STREAMS),
          ("audioTranscodes", 0, MAX_STREAMS))
_ALWAYS = ("downloads", "downloadsPerDay", "downloadKbps", "scansPerDay", "scanBytes")


def keep_limits(o):
    """Playback 5: what a caller keeps of limits (downloads, its daily cap
    and speed, scansPerDay 0 when absent, scanBytes its floor, 4 MiB, when
    absent or out of range; then offersDownloads and offersScans), or None
    when it ignores the whole object."""
    if not isinstance(o, dict):
        return None
    for f, _, _ in LIMITS:
        if f in o and int_in(o[f], -2**53, 2**53) is None:
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
            out[f] = kept.get(f, SCAN_FLOOR if f == "scanBytes" else 0)
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
    same = (reading.get("delivery") == "file" and isinstance(request, dict)
            and request.get("versionId") == reading.get("versionId")
            and request.get("purpose", "play") == reading.get("purpose", "play")
            and request.get("delivery") == reading.get("delivery"))
    if not same or status != 200:
        return fail
    try:
        kept = keep_play_answer(answer, root, delivery=reading["delivery"])
    except Refused:
        return {"join": False, "stopNew": True, "failRead": True, "readTitles": True}
    if kept["etag"] != reading.get("etag"):
        return {"join": False, "stopNew": True, "failRead": True, "readTitles": True}
    return {"join": True, "offset": reading["offset"]}


def playback_vectors_ok(path):
    """Checks keep_play_answer, keep_limits and restart against
    vectors/v0/playback.json: a list of mismatches."""
    v = json.load(open(path))
    root = v["context"]["streamRoot"]
    bad = []
    for c in v["answers"]:
        try:
            out, ok = dumps(keep_play_answer(loads(c["text"]), root, c.get("delivery"), c.get("catalogEtag"))), True
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
