"""What a caller keeps of a music title and its copies (MSFP Media music
2, 3, 3.1 to 3.3, as MSFP's music part carries it), music title keys
of the mb and local schemes, the queue of a play (MSFP Media music 5.2) and lyrics
(MSFP Media music 6), with what an owning server got wrong in a music title, and
the check against vectors/v0/music.json.

The keep_* functions take parsed JSON: from copies.loads (numbers as
written) or from wire.parse_ijson (an answer the suite received). They
return what the caller keeps, as dicts in the vectors' member order,
written with copies.dumps."""

import json
import math
import re

from .copies import OPAQUE, diff, dumps, int_in, loads, media_type, number, plain, real_day, token, valid_tag

MBID = re.compile(r"[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}")
KINDS = ("artist", "album", "track")
# MusicBrainz release-group types as MSFP Media music 3 sends them (proposal 17): open.
TYPE = re.compile(r"[a-z0-9._/-](?:[a-z0-9 ._/-]{0,30}[a-z0-9._/-])?")
CODECS = re.compile(r"[A-Za-z0-9.+-]{1,64}")
# ids on mb titles only (MSFP Media music 3.1, proposal 17): the scheme and the kind it is for.
IDS = {"mbreleasegroup": "album", "mbrecording": "track"}
MAX_COPIES = 16
DELIVERIES = ("file", "remux", "transcode")
# A track's remux: MPEG-TS or fragmented MP4 segments, audio only (MSFP Media playing 2.7).
REMUX_TYPES = ("audio/mp2t", "audio/mp4")
MAX_TRACK_MS = 86400000
_DATE = re.compile(r"([0-9]{4})(?:-([0-9]{2})(?:-([0-9]{2}))?)?")
_TICKET = re.compile(r"[A-Za-z0-9_-]{1,64}")


# ---- Keys (MSFP Media music 2) -------------------------------------------------------------

def parse_music_key(k):
    """(kind, scheme) of a music title key in canonical form (MSFP Media music 2.1,
    2.2); raises ValueError."""
    if not isinstance(k, str):
        raise ValueError("not a string")
    p = k.split("/")
    if len(p) != 3:
        raise ValueError("not three parts joined by /")
    if p[0] not in KINDS:
        raise ValueError("kind %r isn't artist, album or track" % p[0])
    if p[1] == "mb":
        if not MBID.fullmatch(p[2]):
            raise ValueError("an MBID that isn't a lowercase UUID")
    elif p[1] == "local":
        if not OPAQUE.fullmatch(p[2]):
            raise ValueError("an opaque part that isn't 1 to 64 characters of A-Z a-z 0-9 _ -")
    else:
        raise ValueError("scheme %r isn't mb or local" % p[1])
    return p[0], p[1]


def music_key(k):
    """The kind a music title key names, or None."""
    try:
        return parse_music_key(k)[0]
    except ValueError:
        return None


# ---- A track's copies (MSFP Media music 3.3) -----------------------------------------------------

def _round(f, digits):
    """f rounded to digits decimals, to the nearest."""
    s = 10 ** digits
    r = math.floor(abs(f) * s + 0.5) / s
    r = -r if f < 0 else r
    return 0 if r == 0 else (int(r) if r == int(r) else r)


def _keep_gain(o):
    if not isinstance(o, dict):
        return None
    out = {}
    for f, lo, hi, digits in (("trackGain", -64, 64, 2), ("trackPeak", 0, 10, 6), ("albumGain", -64, 64, 2),
                              ("albumPeak", 0, 10, 6)):
        v = number(o.get(f)) if not isinstance(o.get(f), bool) else None
        if v is not None and lo <= v <= hi:
            out[f] = _round(v, digits)
    return out or None


def keep_track_copy(o):
    """MSFP Media music 3.3 for one entry of a track's copies: what the caller keeps
    (a dict in the vectors' order); raises ValueError with why it skips it.
    codecs, required for aac and dts, counts as absent when missing or
    malformed: the copy stays, one the viewer's server doesn't play as it
    is (MSFP Media base 3)."""
    if not isinstance(o, dict):
        raise ValueError("not an object")
    cid = o.get("id")
    if not isinstance(cid, str) or not OPAQUE.fullmatch(cid):
        raise ValueError("id isn't 1 to 64 characters of A-Z a-z 0-9 _ -")
    d = int_in(o.get("durationMs"), 1, MAX_TRACK_MS)
    if d is None:
        raise ValueError("durationMs isn't an integer from 1 to 86,400,000")
    mt, codec = media_type(o.get("mediaType")), token(o.get("codec"))
    if mt is None or codec is None:
        raise ValueError("mediaType isn't a media type or codec isn't a token (MSFP Media base 3)")
    size = int_in(o.get("size"), 1, 999999999999999)
    if size is None:
        raise ValueError("size isn't an integer from 1 to 999,999,999,999,999")
    etag = o.get("etag")
    if not isinstance(etag, str) or not OPAQUE.fullmatch(etag):
        raise ValueError("etag isn't 1 to 64 characters of A-Z a-z 0-9 _ -")
    c = {"id": cid, "durationMs": d, "mediaType": mt, "codec": codec}
    if isinstance(o.get("codecs"), str) and CODECS.fullmatch(o["codecs"]):
        c["codecs"] = o["codecs"]
    c["size"], c["etag"] = size, etag
    for f, lo, hi in (("sampleRate", 8000, 768000), ("bitDepth", 8, 32), ("channels", 1, 64)):
        v = int_in(o.get(f), lo, hi)
        if v is not None:
            c[f] = v
    g = _keep_gain(o.get("replayGain"))
    if g:
        c["replayGain"] = g
    if o.get("lyrics") in ("plain", "synced"):
        c["lyrics"] = o["lyrics"]
    return c


# ---- Titles (MSFP Media music 3, 3.1, 3.2; MSFP Media listing 4.3) -----------------------------------------------

def _date(s):
    """(year, text) of a date in one of MSFP Media music 3's three forms, or None."""
    m = _DATE.fullmatch(s) if isinstance(s, str) else None
    if not m:
        return None
    y = int(m.group(1))
    if not 1000 <= y <= 9999:
        return None
    if m.group(3) is not None:
        if not real_day(s):
            return None
    elif m.group(2) is not None and not 1 <= int(m.group(2)) <= 12:
        return None
    return y


def keep_music_title(o, libraries):
    """MSFP Media listing 3.3 and MSFP Media music 3.2 for one entry of titles: what the caller
    keeps (a dict in the vectors' order); raises ValueError with why it
    skips the title."""
    if not isinstance(o, dict):
        raise ValueError("not an object")
    # A title carries no kind (MSFP Media listing 3): its key names it, and a kind
    # member is unknown, ignored.
    key, lib = o.get("key"), o.get("library")
    if not all(isinstance(x, str) for x in (key, lib)):
        raise ValueError("key or library isn't a string")
    try:
        kind, scheme = parse_music_key(key)
    except ValueError as e:
        raise ValueError("key: %s" % e)
    if lib not in libraries:
        raise ValueError("library %r wasn't asked for" % lib)
    t = {"key": key, "library": lib}
    name = plain(o.get("name"), 256)
    if not name:
        raise ValueError("no name, once cleaned")
    t["name"] = name
    if kind in ("album", "track"):
        arts = []
        for a in o.get("artists") if isinstance(o.get("artists"), list) else []:
            if isinstance(a, str) and a not in arts and len(arts) < 16 and music_key(a) == "artist":
                arts.append(a)
        if arts:
            t["artists"] = arts
        an = plain(o.get("artistNames"), 512)
        if an:
            t["artistNames"] = an
    if kind == "album":
        pt = o.get("primaryType")
        if isinstance(pt, str) and TYPE.fullmatch(pt):
            t["primaryType"] = pt
        st = []
        for x in o.get("secondaryTypes") if isinstance(o.get("secondaryTypes"), list) else []:
            if isinstance(x, str) and len(st) < 8 and TYPE.fullmatch(x) and x not in st:
                st.append(x)
        if st:
            t["secondaryTypes"] = st
        if isinstance(o.get("compilation"), bool):
            t["compilation"] = o["compilation"]
    if kind in ("album", "track"):
        dy, oy = _date(o.get("date")), _date(o.get("originalDate"))
        if dy is not None:
            t["date"] = o["date"]
        if oy is not None and (dy is None or oy <= dy):
            t["originalDate"] = o["originalDate"]
        genres, low = [], set()
        for g in o.get("genres") if isinstance(o.get("genres"), list) else []:
            if not isinstance(g, str) or len(genres) == 8:
                continue
            c = plain(g, 10 ** 6)
            if not c or len(c) > 64 or c.lower() in low:
                continue
            low.add(c.lower())
            genres.append(c)
        if genres:
            t["genres"] = genres
    if kind == "track":
        al = o.get("album")
        if isinstance(al, str) and music_key(al) == "album":
            t["album"] = al
        if scheme == "mb" and not t.get("album", "").startswith("album/mb/"):
            raise ValueError("a track/mb track whose album isn't an album/mb key")
        for f in ("disc", "number"):
            v = int_in(o.get(f), 1, 999)
            if v is not None:
                t[f] = v
        if not isinstance(o.get("copies"), list):
            raise ValueError("copies isn't an array")
        cs = []
        for e in o["copies"]:
            if len(cs) == MAX_COPIES:
                break
            try:
                c = keep_track_copy(e)
            except ValueError:
                continue
            if all(x["id"] != c["id"] for x in cs):
                cs.append(c)
        if not cs:
            raise ValueError("no copy left")
        t["copies"] = cs
    if scheme == "local" and kind in ("artist", "album") and isinstance(o.get("image"), bool):
        t["image"] = o["image"]
    ids = o.get("ids")
    if isinstance(ids, dict) and scheme == "mb":   # none on local titles
        kept = {}
        for s, v in ids.items():
            if IDS.get(s) == kind and isinstance(v, str) and MBID.fullmatch(v):
                kept[s] = v
        if kept:
            t["ids"] = dict(sorted(kept.items()))
    return t


_TITLE_FIELDS = {"key", "library", "name", "artists", "artistNames", "primaryType", "secondaryTypes",
                 "compilation",
                 "date", "originalDate", "genres", "album", "disc", "number", "copies", "image", "ids"}


def music_problems(t, libraries=None):
    """What an owning server got wrong in a music title it sent (MSFP Media music 2,
    3, 3.1 to 3.3): anything a caller would skip, drop, clean or change."""
    key = t.get("key") if isinstance(t, dict) else None
    libs = libraries if libraries is not None else [t.get("library")] if isinstance(t, dict) else []
    try:
        kept = keep_music_title(t, libs)
    except ValueError as e:
        return ["%s: a caller skips it: %s" % (key, e)]
    bad = []
    for c in t.get("copies") if isinstance(t.get("copies"), list) else []:
        try:
            keep_track_copy(c)
        except ValueError as e:
            bad.append("%s: a copy a caller skips: %s" % (key, e))
    sent = dict(t)
    if isinstance(sent.get("copies"), list):
        ids = {c["id"] for c in kept.get("copies", [])}
        sent["copies"] = [c for c in sent["copies"] if isinstance(c, dict) and c.get("id") in ids]
    return bad + diff(sent, kept, str(key), _TITLE_FIELDS)


# ---- The queue (MSFP Media music 5.2) ----------------------------------------------------------------

class QueueRefused(ValueError):
    """A queue the owning server refuses: .status 400, or 409 with .type
    delivery-refused or target-refused (MSFP Media playing 2.2 step 7)."""

    def __init__(self, status, why, typ=None):
        self.type = typ or ("delivery-refused" if status == 409 else "invalid-request")
        ValueError.__init__(self, "%d%s: %s" % (status, " " + self.type if status == 409 else "", why))
        self.status = status


def queue_asked(o):
    """A play request's title and queue as the owning server reads them:
    the entries asked, each (title, versionId or None, the delivery it
    takes: its own, else the request's); raises QueueRefused (400 for a
    malformed one first, then 409 delivery-refused for a delivery, or
    target-refused for a remux's media type, it doesn't make). The request's own delivery
    members and an entry's transcode targets are playback's."""
    if not isinstance(o, dict):
        raise QueueRefused(400, "not an object")
    title = o.get("title")
    if music_key(title) != "track":
        raise QueueRefused(400, "title isn't a track key")
    own = o.get("delivery") if isinstance(o.get("delivery"), str) else ""
    if "queue" not in o:
        return []
    q = o["queue"]
    if not isinstance(q, list):
        raise QueueRefused(400, "queue isn't an array")
    if len(q) > 5:
        raise QueueRefused(400, "a queue of %d entries, more than 5" % len(q))
    asked, seen, refused = [], set(), None
    for i, e in enumerate(q):
        if not isinstance(e, dict):
            raise QueueRefused(400, "queue entry %d isn't an object" % i)
        x = e.get("title")
        if not isinstance(x, str):
            raise QueueRefused(400, "queue entry %d: title isn't a string" % i)
        if music_key(x) != "track":
            raise QueueRefused(400, "queue entry %d: title isn't a track key in canonical form" % i)
        if x == title:
            raise QueueRefused(400, "queue entry %d is title itself" % i)
        if x in seen:
            raise QueueRefused(400, "queue entry %d is a repeat" % i)
        seen.add(x)
        vid = None
        if "versionId" in e:
            vid = e["versionId"]
            if not isinstance(vid, str) or not OPAQUE.fullmatch(vid):
                raise QueueRefused(400, "queue entry %d: versionId isn't a version ID" % i)
        d, mine = own, ""
        if "delivery" in e:
            d = mine = e["delivery"]
            if token(d) is None:
                raise QueueRefused(400, "queue entry %d: delivery isn't a token (MSFP Media base 3)" % i)
            if d not in DELIVERIES and refused is None:
                refused = QueueRefused(409, "queue entry %d: a delivery the owning server doesn't make" % i)
        if "mediaType" in e:
            mt = e["mediaType"]
            if mine != "remux":
                raise QueueRefused(400, "queue entry %d: mediaType without its own delivery remux" % i)
            if media_type(mt) is None:
                raise QueueRefused(400, "queue entry %d: mediaType isn't a media type (MSFP Media base 3)" % i)
            if mt not in REMUX_TYPES and refused is None:
                refused = QueueRefused(409, "queue entry %d: a remux into %s, not audio/mp2t or audio/mp4" % (i, mt),
                                       "target-refused")
        elif mine == "remux":
            raise QueueRefused(400, "queue entry %d: delivery remux without mediaType" % i)
        if "transcode" in e:
            if mine != "transcode":
                raise QueueRefused(400, "queue entry %d: a transcode object without its own delivery transcode" % i)
            if not isinstance(e["transcode"], dict):
                raise QueueRefused(400, "queue entry %d: transcode isn't an object" % i)
        asked.append((x, vid, d))
    if refused is not None:
        raise refused
    return asked


def check_queue_request(o):
    """None when the owning server accepts a play request's title and
    queue, else why it refuses them (starting 400, or 409 delivery-refused or
    target-refused)."""
    try:
        queue_asked(o)
    except QueueRefused as e:
        return str(e)
    return None


def keep_queue(request, answer):
    """MSFP Media music 5.2: the queue the viewer's server keeps of a play answer (a
    list, possibly empty, of title, versionId, method, protocol), the
    request being the play it sent. Entries carry no URL: queue_urls
    builds them from the ticket."""
    try:
        asked = queue_asked(request)
    except QueueRefused as e:
        raise ValueError("the request: %s" % e)
    if not isinstance(answer, dict):
        raise ValueError("the answer isn't an object")
    out, nxt = [], 0
    for e in answer.get("queue") if isinstance(answer.get("queue"), list) else []:
        if len(out) == 5:
            break
        if not isinstance(e, dict):
            continue
        title = e.get("title")
        at = next((i for i in range(nxt, len(asked)) if asked[i][0] == title), -1)
        if at < 0:
            continue
        vid, method, proto = e.get("versionId"), e.get("method"), e.get("protocol")
        _, want_vid, d = asked[at]
        want = DELIVERY_METHOD.get(d)
        if not isinstance(vid, str) or not OPAQUE.fullmatch(vid) or (want_vid and vid != want_vid) \
                or method not in ("direct", "remux", "transcode") or (want and method != want) \
                or proto != ("file" if d == "file" else "hls"):
            continue
        out.append({"title": title, "versionId": vid, "method": method, "protocol": proto, "place": at})
        nxt = at + 1
    return out


DELIVERY_METHOD = {"file": "direct", "remux": "remux", "transcode": "transcode"}


def queue_urls(root, ticket, kept):
    """The URL of each kept queue entry (MSFP Media fetching 1): <stream root><ticket>/
    queue/<n>/file or /master.m3u8, n its place in the request's queue."""
    return ["%s%s/queue/%d/%s" % (root, ticket, e["place"], "file" if e["protocol"] == "file" else "master.m3u8")
            for e in kept]


# ---- Lyrics (MSFP Media music 6) ---------------------------------------------------------------------

def check_lyrics_request(o):
    """A lyrics request as the owning server reads it: None, or why it
    answers 400 invalid-request."""
    if not isinstance(o, dict):
        return "not an object"
    if music_key(o.get("title")) != "track":
        return "title isn't a track key"
    vid = o.get("versionId")
    if not isinstance(vid, str) or not OPAQUE.fullmatch(vid):
        return "versionId isn't a version ID"
    return None


def keep_lyrics(answer):
    """MSFP Media music 6: what the viewer's server keeps of a lyrics answer (bytes,
    text or parsed JSON): {"structuredLyrics": [sets]}, each set in the
    vectors' order (lang, kind, line); raises ValueError when the fetch
    fails. A set is synced when every line left carries start, plain when
    none does, skipped when they mix."""
    if isinstance(answer, (bytes, bytearray)):
        if len(answer) > 1024 * 1024:
            raise ValueError("over 1 MiB")
        answer = loads(answer)
    elif isinstance(answer, str):
        answer = loads(answer)
    if not isinstance(answer, dict) or not isinstance(answer.get("structuredLyrics"), list):
        raise ValueError("structuredLyrics isn't an array")
    out = []
    for s in answer["structuredLyrics"]:
        if len(out) == 8:
            break
        if not isinstance(s, dict):
            continue
        k = {}
        if valid_tag(s.get("lang")):
            k["lang"] = s["lang"]
        k["kind"] = s["kind"] if token(s.get("kind")) else "main"
        left = [ln for ln in (s.get("line") if isinstance(s.get("line"), list) else [])
                if isinstance(ln, dict) and isinstance(ln.get("value"), str)]
        starts = sum(1 for ln in left if "start" in ln)
        if starts not in (0, len(left)):
            continue
        synced = starts > 0
        lines, last = [], 0
        for ln in left:
            if len(lines) == 2000:
                break
            e = {}
            if synced:
                st = int_in(ln.get("start"), 0, MAX_TRACK_MS)
                if st is None or st < last:
                    continue
                last = st
                e["start"] = st
            e["value"] = plain(ln["value"], 512)
            lines.append(e)
        if not lines:
            continue
        k["line"] = lines
        out.append(k)
    if not out:
        raise ValueError("no set left")
    return {"structuredLyrics": out}


# ---- The vectors ---------------------------------------------------------------------------

def music_vectors_ok(path):
    """Checks the readers above against vectors/v0/music.json: a list of
    mismatches."""
    v = json.load(open(path))
    bad = []
    for c in v["keys"]:
        kind = music_key(c["text"])
        if (kind is not None) != c["valid"] or (kind and kind != c.get("kind")):
            bad.append("music key %s: %s" % (c["name"], kind))
    for g in ("titles", "localTitles"):
        for c in v[g]:
            try:
                out, ok = dumps(keep_music_title(loads(c["text"]), v["libraries"])), True
            except ValueError as e:
                out, ok = str(e), False
            if ok != c["valid"] or (ok and out != c["output"]):
                bad.append("music %s %s: %s, not %s" % (g, c["name"], out, c.get("output", c.get("reason"))))
    for c in v["queue"]:
        if c["kind"] == "request":
            try:
                o = loads(c["text"])
            except ValueError:
                o = None
            why = check_queue_request(o)
            ok = why is None
            if ok != c["valid"] or (not ok and not why.startswith(c.get("reason", "")[:3])):
                bad.append("music queue request %s: %s" % (c["name"], check_queue_request(o)))
        else:
            try:
                kept = keep_queue(loads(c["request"]), loads(c["text"]))
                out, ok = dumps([{f: e[f] for f in ("title", "versionId", "method", "protocol")} for e in kept]), True
            except ValueError as e:
                out, ok = str(e), False
            if ok != c["valid"] or (ok and out != c["output"]):
                bad.append("music queue answer %s: %s, not %s" % (c["name"], out, c.get("output")))
    for c in v["lyrics"]:
        if c["kind"] == "request":
            try:
                o = loads(c["text"])
            except ValueError:
                o = None
            ok = check_lyrics_request(o) is None
            if ok != c["valid"]:
                bad.append("music lyrics request %s: %s" % (c["name"], check_lyrics_request(o)))
        else:
            try:
                out, ok = dumps(keep_lyrics(c["text"].encode("utf-8", "surrogatepass"))), True
            except ValueError as e:
                out, ok = str(e), False
            if ok != c["valid"] or (ok and out != c["output"]):
                bad.append("music lyrics answer %s: %s, not %s" % (c["name"], out, c.get("output", c.get("reason"))))
    return bad
