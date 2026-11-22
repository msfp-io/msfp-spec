"""What a caller keeps of a video title and its copies (spec/v0/video.md
sections 3.1 to 3.10, as revised by proposals 16 and 17), video title
keys of the tmdb, tvdb and local schemes and extras' keys (video 4), what
a viewer's server reads from a codecs string (video 3.5), play's video
members and the URLs built from its ticket (video 5.3, 6.1), the previews
answer (video 5.6), language tags as catalog 3 sends them, and the JSON
helpers the vectors' outputs need.

The keep_* functions take a title or a copy as parsed JSON: from
loads() here (numbers kept as written, as the vectors are read) or from
wire.parse_ijson (an answer the suite received). They return what the
caller keeps, as dicts in the vectors' member order, written with dumps()."""

import json
import math
import re
from decimal import Decimal

from .wire import MAX_INT, NegativeZero, clean_name


# ---- JSON with numbers as written ------------------------------------------------

class Num:
    """A JSON number as written (what Go's json.Number keeps)."""

    def __init__(self, text):
        self.text = text

    def __repr__(self):
        return "Num(%s)" % self.text


def _fix(v):
    if isinstance(v, str):
        # A lone surrogate (an escape) reads as U+FFFD, as Go reads it.
        return "".join("\ufffd" if 0xD800 <= ord(c) <= 0xDFFF else c for c in v)
    if isinstance(v, list):
        return [_fix(x) for x in v]
    if isinstance(v, dict):
        return {_fix(k): _fix(x) for k, x in v.items()}
    return v


def loads(text):
    """One JSON value, numbers as Num; raises ValueError."""
    if isinstance(text, (bytes, bytearray)):
        text = text.decode("utf-8", "replace")
    return _fix(json.loads(text, parse_int=Num, parse_float=Num, parse_constant=lambda c: (_ for _ in ()).throw(
        ValueError("not JSON: " + c))))


def num_text(f):
    """A number in its shortest decimal form, without an exponent (1.0 is 1)."""
    if isinstance(f, int):
        return str(f)
    if f == 0:
        return "0"
    if f == int(f) and abs(f) < 1e21:
        return str(int(f))
    return format(Decimal(repr(f)).normalize(), "f")


def dumps(v):
    """Compact JSON as the vectors write it (Go's encoder without HTML
    escaping): non-ASCII as it is, numbers in their shortest form."""
    if v is None:
        return "null"
    if v is True:
        return "true"
    if v is False:
        return "false"
    if isinstance(v, (int, float)):
        return num_text(v)
    if isinstance(v, str):
        return json.dumps(v, ensure_ascii=False).replace("\u2028", "\\u2028").replace("\u2029", "\\u2029")
    if isinstance(v, list):
        return "[" + ",".join(dumps(x) for x in v) + "]"
    if isinstance(v, dict):
        return "{" + ",".join(dumps(k) + ":" + dumps(x) for k, x in v.items()) + "}"
    raise TypeError(type(v))


_INTEGER = re.compile(r"-?(0|[1-9][0-9]*)")


def int_in(v, lo, hi):
    """v as an MSFP integer (core 2.3) in [lo, hi], or None."""
    if isinstance(v, Num):
        t = v.text
        if not _INTEGER.fullmatch(t) or t == "-0":
            return None
        i = int(t)
    elif isinstance(v, int) and not isinstance(v, (bool, NegativeZero)):
        i = v
    else:
        return None
    if abs(i) > MAX_INT or not lo <= i <= hi:
        return None
    return i


def number(v):
    """v as a float when it is a JSON number of any form, or None."""
    if isinstance(v, Num):
        return float(v.text)
    if isinstance(v, (int, float)) and not isinstance(v, bool):
        return float(v)
    return None


def boolean(v):
    return v if isinstance(v, bool) else None


def plain(v, n):
    """Core 9.1's cleaning, with n code points: "" when nothing is left
    or v isn't a string."""
    return clean_name(v, n) if isinstance(v, str) else ""


# ---- Language tags (catalog 3) -------------------------------------------------------

ISO639 = dict(x.split(":") for x in (
    "aar:aa abk:ab afr:af aka:ak alb:sq amh:am ara:ar arg:an arm:hy asm:as ava:av ave:ae aym:ay aze:az bak:ba bam:bm "
    "baq:eu bel:be ben:bn bih:bh bis:bi bod:bo bos:bs bre:br bul:bg bur:my cat:ca ces:cs cha:ch che:ce chi:zh chu:cu "
    "chv:cv cor:kw cos:co cre:cr cym:cy cze:cs dan:da deu:de div:dv dut:nl dzo:dz ell:el eng:en epo:eo est:et eus:eu "
    "ewe:ee fao:fo fas:fa fij:fj fin:fi fra:fr fre:fr fry:fy ful:ff geo:ka ger:de gla:gd gle:ga glg:gl glv:gv gre:el "
    "grn:gn guj:gu hat:ht hau:ha heb:he her:hz hin:hi hmo:ho hrv:hr hun:hu hye:hy ibo:ig ice:is ido:io iii:ii iku:iu "
    "ile:ie ina:ia ind:id ipk:ik isl:is ita:it jav:jv jpn:ja kal:kl kan:kn kas:ks kat:ka kau:kr kaz:kk khm:km kik:ki "
    "kin:rw kir:ky kom:kv kon:kg kor:ko kua:kj kur:ku lao:lo lat:la lav:lv lim:li lin:ln lit:lt ltz:lb lub:lu lug:lg "
    "mac:mk mah:mh mal:ml mao:mi mar:mr may:ms mkd:mk mlg:mg mlt:mt mon:mn mri:mi msa:ms mya:my nau:na nav:nv nbl:nr "
    "nde:nd ndo:ng nep:ne nld:nl nno:nn nob:nb nor:no nya:ny oci:oc oji:oj ori:or orm:om oss:os pan:pa per:fa pli:pi "
    "pol:pl por:pt pus:ps que:qu roh:rm ron:ro rum:ro run:rn rus:ru sag:sg san:sa sin:si slk:sk slo:sk slv:sl sme:se "
    "smo:sm sna:sn snd:sd som:so sot:st spa:es sqi:sq srd:sc srp:sr ssw:ss sun:su swa:sw swe:sv tah:ty tam:ta tat:tt "
    "tel:te tgk:tg tgl:tl tha:th tib:bo tir:ti ton:to tsn:tn tso:ts tuk:tk tur:tr twi:tw uig:ug ukr:uk urd:ur uzb:uz "
    "ven:ve vie:vi vol:vo wel:cy wln:wa wol:wo xho:xh yid:yi yor:yo zha:za zho:zh zul:zu").split())
_TAG_CHARS = re.compile(r"[A-Za-z0-9-]{2,12}")


def sent_tag(code):
    """The tag a server sends for a media server's language code (catalog
    3), or None when none is sent."""
    if not isinstance(code, str) or not _TAG_CHARS.fullmatch(code):
        return None
    out = []
    for i, s in enumerate(code.split("-")):
        if s == "":
            return None
        s = s.lower()
        if i == 0:
            if len(s) > 8 or not re.fullmatch(r"[a-z]+", s):
                return None
            s = ISO639.get(s, s)
            if s in ("und", "mul", "zxx", "mis"):
                return None
        elif (len(s) == 2 and s.isalpha()) or (len(s) == 3 and s.isdigit()):
            s = s.upper()
        elif len(s) == 4 and s.isalpha():
            s = s[0].upper() + s[1:]
        out.append(s)
    return "-".join(out)


def valid_tag(s):
    """A valid tag (catalog 3): one sent_tag leaves as it is."""
    return isinstance(s, str) and sent_tag(s) == s


# ---- Video title keys (video 4) -----------------------------------------------------

_ID = re.compile(r"[1-9][0-9]{0,9}")
OPAQUE = re.compile(r"[A-Za-z0-9_-]{1,64}")
_SEASON = re.compile(r"s(0|[1-9][0-9]{0,3})")
_EPISODE = re.compile(r"e(0|[1-9][0-9]{0,4})")
_ABS = re.compile(r"a(0|[1-9][0-9]{0,4})")
_EP_ID = re.compile(r"e[1-9][0-9]{0,9}")
AUDIO_ID = re.compile(r"a(0|[1-9][0-9]{0,2})")
SUBTITLE_ID = re.compile(r"s(0|[1-9][0-9]{0,2})")
_IMDB = re.compile(r"tt[0-9]{7,10}")


class VideoKey(tuple):
    """What a video title key says: (kind, series), series being the
    film's or series' own key (its first three parts, "series/tmdb/1399"),
    with .scheme, .parent (an extra's title, else None) and .by_id (an
    episode keyed by its TMDB episode ID)."""

    def __new__(cls, kind, series, scheme, parent=None, by_id=False):
        t = tuple.__new__(cls, (kind, series))
        t.scheme, t.parent, t.by_id = scheme, parent, by_id
        return t

    kind = property(lambda self: self[0])
    series = property(lambda self: self[1])


def parse_video_key(key):
    """The VideoKey of a video title key in canonical form; raises ValueError."""
    if not isinstance(key, str):
        raise ValueError("not a string")
    p = key.split("/")
    if len(p) < 3:
        raise ValueError("fewer than three parts joined by /")
    if p[0] not in ("movie", "series"):
        raise ValueError("kind %r is neither movie nor series" % p[0])
    scheme, series = p[1], "/".join(p[:3])
    if scheme == "tvdb" and p[0] == "movie":
        raise ValueError("films have no tvdb keys")
    if scheme not in ("tmdb", "tvdb", "local"):
        raise ValueError("scheme %r isn't tmdb, tvdb or local" % scheme)
    if scheme == "local":
        if not OPAQUE.fullmatch(p[2]):
            raise ValueError("a local identifier outside 1 to 64 characters of A-Z a-z 0-9 _ -")
    elif not _ID.fullmatch(p[2]):
        raise ValueError("a %s ID that isn't 1 to 9,999,999,999 without leading zeros" % scheme)
    rest = p[3:]
    if len(rest) >= 2 and rest[-2] == "x":
        if not OPAQUE.fullmatch(rest[-1]):
            raise ValueError("an extra's part outside 1 to 64 characters of A-Z a-z 0-9 _ -")
        parent = "/".join(p[:-2])
        pk = parse_video_key(parent)
        if pk.kind not in ("movie", "series", "season"):
            raise ValueError("an extra under an %s: only films, series and seasons have extras" % pk.kind)
        return VideoKey("extra", series, scheme, parent)
    if p[0] == "movie":
        if rest:
            raise ValueError("a film key with more parts")
        return VideoKey("movie", series, scheme)
    if not rest:
        return VideoKey("series", series, scheme)
    if len(rest) == 1 and _SEASON.fullmatch(rest[0]):
        return VideoKey("season", series, scheme)
    if len(rest) == 2 and _SEASON.fullmatch(rest[0]) and _EPISODE.fullmatch(rest[1]):
        return VideoKey("episode", series, scheme)
    if len(rest) == 1 and scheme == "tmdb" and _EP_ID.fullmatch(rest[0]):
        return VideoKey("episode", series, scheme, by_id=True)
    if len(rest) == 1 and scheme == "tvdb" and _ABS.fullmatch(rest[0]):
        return VideoKey("episode", series, scheme)
    raise ValueError("not one of the forms of section 4 for its scheme")


def video_key(k):
    """The VideoKey of k ((kind, series) with .scheme, .parent, .by_id),
    or None when it isn't a video title key in canonical form (video 4)."""
    try:
        return parse_video_key(k)
    except ValueError:
        return None


def extra_parent(k):
    """The key of the title an extra is under, or None when k isn't an extra's key."""
    vk = video_key(k)
    return vk.parent if vk and vk.kind == "extra" else None


# ---- Copies (video 3.2 to 3.8) ------------------------------------------------------

# Every list of video 3.2 to 3.9 is open (core 8.5, proposal 17): its
# values are tokens (or media types), kept known or not.
_TOKEN = re.compile(r"[a-z0-9._-]{1,32}")
_MEDIA_TYPE = re.compile(r"[a-z0-9.+_/-]{1,127}")
MAX_DURATION = 8640000000
MAX_SIZE = 999999999999999
_CODECS = re.compile(r"[A-Za-z0-9.+-]{1,64}")
_FPS = re.compile(r"([1-9][0-9]{0,6})/([1-9][0-9]{0,6})")
_ASPECT = re.compile(r"([1-9][0-9]{0,4}):([1-9][0-9]{0,4})")
MAX_COPIES = 16
MAX_HDR = 8
_DATE = re.compile(r"([0-9]{4})-([0-9]{2})-([0-9]{2})")
# The six codecs whose codecs string is required (video 3.5).
REQUIRES_CODECS = ("h264", "hevc", "av1", "vp9", "aac", "dts")


def _form(v, rx):
    return v if isinstance(v, str) and rx.fullmatch(v) else None


def token(v):
    """v when it is a string and a token (core 8.5: 1 to 32 characters of
    a-z 0-9 . _ -), else None: an open field keeps any token, known or not."""
    return v if isinstance(v, str) and _TOKEN.fullmatch(v) else None


def media_type(v):
    """v when it is a media type of core 8.5 (lowercase, exactly one /, no
    parameters, each half starting with a letter or a digit, at most 127
    characters of a-z 0-9 . + - _ /), else None."""
    if not isinstance(v, str) or not _MEDIA_TYPE.fullmatch(v) or v.count("/") != 1:
        return None
    top, sub = v.split("/")
    ok = lambda h: h != "" and (h[0].isdigit() or "a" <= h[0] <= "z")
    return v if ok(top) and ok(sub) else None


def fps(v):
    """A frame rate "<num>/<den>", each 1 to 1,000,000, reduced, else None."""
    m = _FPS.fullmatch(v) if isinstance(v, str) else None
    if not m:
        return None
    a, b = int(m.group(1)), int(m.group(2))
    return v if a <= 1000000 and b <= 1000000 and math.gcd(a, b) == 1 else None


def aspect(v):
    """A sample aspect ratio "<w>:<h>", each 1 to 65,535, else None."""
    m = _ASPECT.fullmatch(v) if isinstance(v, str) else None
    return v if m and int(m.group(1)) <= 65535 and int(m.group(2)) <= 65535 else None


def real_day(s):
    """A valid date YYYY-MM-DD."""
    m = _DATE.fullmatch(s) if isinstance(s, str) else None
    if not m:
        return False
    import datetime
    try:
        datetime.date(int(m.group(1)), int(m.group(2)), int(m.group(3)))
    except ValueError:
        return False
    return True


def _put(d, k, v):
    if v is not None and v != "" and v != []:
        d[k] = v


def _h273(v):
    """An H.273 code point, 0 to 255; 2 ("unspecified") counts as absent."""
    n = int_in(v, 0, 255)
    return None if n == 2 else n


def keep_layout(v):
    """Video 5.5's reading of a previewLayout: None when it is dropped."""
    if not isinstance(v, dict):
        return None
    w, h = int_in(v.get("tileWidth"), 1, 2048), int_in(v.get("tileHeight"), 1, 2048)
    c, r = int_in(v.get("columns"), 1, 100), int_in(v.get("rows"), 1, 100)
    iv = int_in(v.get("intervalMs"), 100, 3600000)
    if None in (w, h, c, r, iv) or c * w > 4096 or r * h > 4096:
        return None
    n = int_in(v.get("tileCount"), 1, 1000 * c * r)
    if n is None:
        return None
    return {"tileWidth": w, "tileHeight": h, "columns": c, "rows": r, "intervalMs": iv, "tileCount": n}


def sheet_count(layout):
    """The number of sheets a kept previewLayout gives (video 5.5)."""
    per = layout["columns"] * layout["rows"]
    return -(-layout["tileCount"] // per)


def _keep_stream(o):
    if not isinstance(o, dict):
        return None
    codec = token(o.get("codec"))
    w, h = int_in(o.get("width"), 1, 100000), int_in(o.get("height"), 1, 100000)
    if codec is None or w is None or h is None:
        return None
    s = {"codec": codec}
    _put(s, "codecs", _form(o.get("codecs"), _CODECS))
    s["width"], s["height"] = w, h
    _put(s, "sampleAspect", aspect(o.get("sampleAspect")))
    r = int_in(o.get("rotation"), 0, 270)
    if r is not None and r % 90 == 0:
        s["rotation"] = r
    _put(s, "bitDepth", int_in(o.get("bitDepth"), 1, 16))
    c = o.get("colour")
    if isinstance(c, dict):
        col = {}
        for k in ("primaries", "transfer", "matrix"):
            _put(col, k, _h273(c.get(k)))
        _put(col, "fullRange", boolean(c.get("fullRange")))
        if col:
            s["colour"] = col
    if isinstance(o.get("hdr"), list):
        hdr = []
        for x in o["hdr"]:
            t = token(x)
            if t is not None and t not in hdr:
                hdr.append(t)
        _put(s, "hdr", sorted(hdr)[:MAX_HDR])
    dv = o.get("dv")
    if isinstance(dv, dict):
        p, lv2, c = int_in(dv.get("profile"), 0, 127), int_in(dv.get("level"), 0, 63), int_in(dv.get("compatibility"), 0, 15)
        if None not in (p, lv2, c):
            s["dv"] = {"profile": p, "level": lv2, "compatibility": c}
            _put(s["dv"], "el", boolean(dv.get("el")))
    _put(s, "fps", fps(o.get("fps")))
    if codec == "h264":
        _put(s, "refFrames", int_in(o.get("refFrames"), 1, 16))
    _put(s, "interlaced", boolean(o.get("interlaced")))
    return s


def _keep_audio(lst):
    out, seen = [], set()
    for o in lst:
        if not isinstance(o, dict) or len(out) == 32:
            continue
        tid, codec = _form(o.get("id"), AUDIO_ID), token(o.get("codec"))
        if tid is None or tid in seen or codec is None:
            continue
        seen.add(tid)
        t = {"id": tid}
        _put(t, "language", o["language"] if valid_tag(o.get("language")) else None)
        _put(t, "channels", int_in(o.get("channels"), 1, 64))
        _put(t, "role", token(o.get("role")))
        _put(t, "default", boolean(o.get("default")))
        _put(t, "forced", boolean(o.get("forced")))
        t["codec"] = codec
        _put(t, "codecs", _form(o.get("codecs"), _CODECS))
        _put(t, "objects", token(o.get("objects")))
        _put(t, "sampleRate", int_in(o.get("sampleRate"), 8000, 768000))
        _put(t, "bitDepth", int_in(o.get("bitDepth"), 8, 32))
        _put(t, "original", boolean(o.get("original")))
        out.append(t)
    return out


def _keep_subtitles(lst):
    out, seen = [], set()
    for o in lst:
        if not isinstance(o, dict) or len(out) == 64:
            continue
        tid = _form(o.get("id"), SUBTITLE_ID)
        if tid is None or tid in seen:
            continue
        seen.add(tid)
        t = {"id": tid}
        _put(t, "language", o["language"] if valid_tag(o.get("language")) else None)
        _put(t, "role", token(o.get("role")))
        for f in ("default", "forced", "hearingImpaired"):
            _put(t, f, boolean(o.get(f)))
        _put(t, "format", token(o.get("format")))
        _put(t, "external", boolean(o.get("external")))
        out.append(t)
    return out


def _keep_chapters(v, duration):
    out = []
    for o in v if isinstance(v, list) else []:
        if not isinstance(o, dict) or len(out) == 256:
            continue
        s = int_in(o.get("startMs"), 0, duration)
        if s is None or (out and s <= out[-1]["startMs"]):
            continue
        c = {"startMs": s}
        _put(c, "name", plain(o.get("name"), 64))
        out.append(c)
    return out


def _keep_segments(v, duration):
    out = []
    for o in v if isinstance(v, list) else []:
        if not isinstance(o, dict) or len(out) == 32:
            continue
        kind = token(o.get("kind"))
        s, t = int_in(o.get("startMs"), 0, duration), int_in(o.get("endMs"), 0, duration)
        if kind is None or s is None or t is None or s >= t:
            continue
        out.append({"kind": kind, "startMs": s, "endMs": t})
    return out


def keep_video_copy(o):
    """Video 3.8 for one entry of copies: what the caller keeps (a dict
    in the vectors' order); raises ValueError with why it skips the copy."""
    if not isinstance(o, dict):
        raise ValueError("not an object")
    cid = _form(o.get("id"), OPAQUE)
    if cid is None:
        raise ValueError("id isn't 1 to 64 characters of A-Z a-z 0-9 _ -")
    d = int_in(o.get("durationMs"), 0, MAX_DURATION)
    if d is None:
        raise ValueError("durationMs isn't an integer from 0 to 8,640,000,000")
    mt = media_type(o.get("mediaType"))
    if mt is None:
        raise ValueError("mediaType missing or not a media type (core 8.5)")
    size = int_in(o.get("size"), 1, MAX_SIZE)
    if size is None:
        raise ValueError("size isn't an integer from 1 to 999,999,999,999,999")
    etag = _form(o.get("etag"), OPAQUE)
    if etag is None:
        raise ValueError("etag isn't 1 to 64 characters of A-Z a-z 0-9 _ -")
    audio = o.get("audioTracks")
    if not isinstance(audio, list):
        raise ValueError("audioTracks isn't an array")
    c = {"id": cid}
    _put(c, "edition", token(o.get("edition")))
    c["durationMs"] = d
    _put(c, "startMs", int_in(o.get("startMs"), 0, d))
    _put(c, "endMs", int_in(o.get("endMs"), c.get("startMs", 0) + 1, d))
    c["mediaType"], c["size"], c["etag"] = mt, size, etag
    _put(c, "parts", int_in(o.get("parts"), 2, 16))
    _put(c, "video", _keep_stream(o.get("video")))
    c["audioTracks"] = _keep_audio(audio)
    if isinstance(o.get("subtitleTracks"), list):
        _put(c, "subtitleTracks", _keep_subtitles(o["subtitleTracks"]))
    _put(c, "chapters", _keep_chapters(o.get("chapters"), d))
    _put(c, "segments", _keep_segments(o.get("segments"), d))
    _put(c, "previewLayout", keep_layout(o.get("previewLayout")))
    _put(c, "threeD", token(o.get("threeD")))
    return c


# ---- Titles (video 3.1, 3.9, 3.10) ----------------------------------------------------

def keep_video_title(o, libraries):
    """Catalog 3.3 and video 3.1, 3.8 to 3.10 for one entry of titles:
    what the caller keeps (a dict in the vectors' order); raises
    ValueError with why it skips the title."""
    if not isinstance(o, dict):
        raise ValueError("not an object")
    # A title carries no kind (catalog 3): its key names it, and a kind
    # member is unknown, ignored.
    key, lib = o.get("key"), o.get("library")
    if not all(isinstance(x, str) for x in (key, lib)):
        raise ValueError("key or library isn't a string")
    try:
        k = parse_video_key(key)
    except ValueError as e:
        raise ValueError("key: %s" % e)
    kind = k.kind
    if lib not in libraries:
        raise ValueError("library %r wasn't asked for" % lib)
    t = {"key": key, "library": lib}
    if kind == "extra":
        et = token(o.get("extraType"))
        if et is None:
            raise ValueError("an extra whose extraType is missing or isn't a token")
        t["extraType"] = et
    if k.scheme == "local" and kind in ("movie", "series"):
        name = plain(o.get("name"), 128)
        if not name:
            raise ValueError("a local film or series without a name")
        t["name"] = name
        _put(t, "year", int_in(o.get("year"), 1800, 9999))
        _put(t, "image", boolean(o.get("image")))
    if k.by_id:
        p = o.get("place")
        if not isinstance(p, dict):
            raise ValueError("an episode keyed by its TMDB ID without a place")
        s, e = int_in(p.get("season"), 0, 9999), int_in(p.get("episode"), 0, 99999)
        if s is None or e is None:
            raise ValueError("place: season or episode missing, out of range or not an integer")
        t["place"] = {"season": s, "episode": e}
    if kind == "episode":
        w = []
        for s in o.get("with") if isinstance(o.get("with"), list) else []:
            if not isinstance(s, str) or s == key or len(w) == 16 or s in w:
                continue
            x = video_key(s)
            if x and x.kind == "episode" and x.series == k.series:
                w.append(s)
        _put(t, "with", w)
    # ids (video 4.3): on episodes of tmdb and tvdb and on tvdb series only.
    episode = kind == "episode" and k.scheme != "local"
    tvdb_series = kind == "series" and k.scheme == "tvdb"
    ids = o.get("ids")
    if isinstance(ids, dict) and (episode or tvdb_series):
        kept = {}
        for scheme, val in ids.items():
            if isinstance(val, str) and ((scheme == "imdb" and _IMDB.fullmatch(val))
                                         or (scheme == "tvdb" and episode and _ID.fullmatch(val))
                                         or (scheme == "tmdb" and k.scheme == "tvdb" and _ID.fullmatch(val))):
                kept[scheme] = val
        _put(t, "ids", dict(sorted(kept.items())) or None)
    if kind in ("movie", "episode", "extra"):
        if "copies" in o and not isinstance(o["copies"], list):
            raise ValueError("copies isn't an array")
        copies = []
        for e in o.get("copies") or []:
            if len(copies) == MAX_COPIES:
                break
            try:
                c = keep_video_copy(e)
            except ValueError:
                continue
            if all(x["id"] != c["id"] for x in copies):
                copies.append(c)
        if not copies:
            raise ValueError("no copy left")
        t["copies"] = copies
    return t


# ---- previews (video 5.5, 5.6) ----------------------------------------------------------

def check_previews_request(o):
    """A previews request as the owning server reads it: None, or why it
    answers 400."""
    if not isinstance(o, dict):
        return "not an object"
    title = o.get("title")
    if not isinstance(title, str):
        return "title missing or not a string"
    k = video_key(title)
    if k is None:
        return "title isn't a video title key"
    if k.kind not in ("movie", "episode", "extra"):
        return "title: a %s has no copies" % k.kind
    vid = o.get("versionId")
    if not isinstance(vid, str):
        return "versionId missing or not a string"
    if not OPAQUE.fullmatch(vid):
        return "versionId isn't 1 to 64 characters of A-Z a-z 0-9 _ -"
    return None


def keep_previews_answer(o):
    """Video 5.6: what the viewer's server keeps of a previews answer (its
    ticket and etag); raises ValueError when it refuses it (no sheets)."""
    if not isinstance(o, dict):
        raise ValueError("not an object")
    tk, et = _form(o.get("ticket"), OPAQUE), _form(o.get("etag"), OPAQUE)
    if tk is None:
        raise ValueError("ticket missing or not 1 to 64 characters of A-Z a-z 0-9 _ -")
    if et is None:
        raise ValueError("etag missing or not 1 to 64 characters of A-Z a-z 0-9 _ -")
    return {"ticket": tk, "etag": et}


# ---- codecs (video 3.5) ----------------------------------------------------------------

_AVC = re.compile(r"(avc1|avc3)\.([0-9A-Fa-f]{2})[0-9A-Fa-f]{2}([0-9A-Fa-f]{2})")
_HEVC = re.compile(r"(hvc1|hev1)\.[ABC]?([0-9]{1,2})\.[0-9A-Fa-f]{1,8}\.([LH])([0-9]{1,3})((?:\.[0-9A-Fa-f]{1,2}){0,6})")
_AV1 = re.compile(r"(av01)\.([0-7])\.([0-9]{2})([MH])\.([0-9]{2})(?:\.([01])\.([01][01][0-3])\.[0-9]{2}\.[0-9]{2}\.[0-9]{2}\.[01])?")
_VP9 = re.compile(r"(vp09)\.([0-9]{2})\.[0-9]{2}\.([0-9]{2})(?:\.([0-9]{2})(?:\.[0-9]{2}){0,4})?")
_AAC = re.compile(r"mp4a\.(40\.[1-9][0-9]?|66|67|68)")
_DTS = re.compile(r"dtsc|dtsh|dtsl|dtse")
DV_ENTRY = {"hvc1": "dvh1", "hev1": "dvhe", "av01": "dav1", "avc1": "dva1", "avc3": "dvav"}


def read_codecs(o):
    """Video 3.5: what a viewer's server decides by in a stream or a track
    as kept (codec, codecs, bitDepth, dv): the parts of codecs it reads,
    those it ignores left out, and the Dolby Vision string rebuilt from
    the base 4CC and dv. Raises ValueError when it treats the copy as one
    it can't play as it is (codecs missing or malformed where required,
    or unreadable for its codec)."""
    if not isinstance(o, dict):
        raise ValueError("not an object")
    codec = token(o.get("codec"))
    if codec is None:
        raise ValueError("codec missing or not a token")
    f = {"codec": codec}
    cs = _form(o.get("codecs"), _CODECS)
    if cs is None:
        if codec in REQUIRES_CODECS:
            raise ValueError("codecs missing or malformed, required for %s: can't play as it is" % codec)
        return f
    f["codecs"] = cs
    own = int_in(o.get("bitDepth"), 1, 32)
    for dv in DV_ENTRY.values():
        if cs == dv or cs.startswith(dv + "."):
            raise ValueError("a Dolby Vision 4CC (%s): codecs carries the base codec's; can't play as it is" % dv)
    four = None
    if codec == "h264":
        m = _AVC.fullmatch(cs)
        if not m:
            raise ValueError("codecs isn't avc1 or avc3 and three bytes: can't play as it is")
        four, f["profile"], f["level"] = m.group(1), int(m.group(2), 16), int(m.group(3), 16)
    elif codec == "hevc":
        m = _HEVC.fullmatch(cs)
        if not m:
            raise ValueError("codecs isn't an HEVC string of hvc1 or hev1: can't play as it is")
        four, f["profile"], f["tier"], f["level"] = m.group(1), int(m.group(2)), m.group(3), int(m.group(4))
        cons = m.group(5)[1:] if m.group(5) else ""
        if f["profile"] >= 4 and not cons:
            raise ValueError("an HEVC profile of 4 or more without its constraint bytes: can't play as it is")
        if cons:
            f["constraints"] = cons
    elif codec == "av1":
        m = _AV1.fullmatch(cs)
        if not m:
            raise ValueError("codecs isn't an AV1 string of av01: can't play as it is")
        four, f["profile"], f["level"], f["tier"], f["bitDepth"] = (m.group(1), int(m.group(2)), int(m.group(3)),
                                                                    m.group(4), int(m.group(5)))
        if m.group(6) == "1":
            f["chroma"] = "4:0:0"
        elif m.group(7):
            ch = {"110": "4:2:0", "111": "4:2:0", "112": "4:2:0", "113": "4:2:0", "100": "4:2:2",
                  "000": "4:4:4"}.get(m.group(7))
            if ch:
                f["chroma"] = ch
        elif f["profile"] == 0:
            f["chroma"] = "4:2:0"
        elif f["profile"] == 1:
            raise ValueError("AV1 profile 1 (4:4:4) without the long form: can't play as it is")
    elif codec == "vp9":
        m = _VP9.fullmatch(cs)
        if not m:
            raise ValueError("codecs isn't a VP9 string of vp09: can't play as it is")
        four, f["profile"], f["bitDepth"] = m.group(1), int(m.group(2)), int(m.group(3))
        sub = m.group(4)
        if sub in ("00", "01"):
            f["chroma"] = "4:2:0"
        elif sub == "02":
            f["chroma"] = "4:2:2"
        elif sub == "03":
            f["chroma"] = "4:4:4"
        elif sub is None:
            if f["profile"] in (1, 3):
                raise ValueError("VP9 profile 1 or 3 (not 4:2:0) without the long form: can't play as it is")
            f["chroma"] = "4:2:0"
        else:
            raise ValueError("a VP9 chroma subsampling other than 00 to 03: can't play as it is")
    elif codec == "aac":
        m = _AAC.fullmatch(cs)
        if not m:
            raise ValueError("codecs isn't mp4a.40.<n> or an MPEG-2 AAC mp4a: can't play as it is")
        f["mp4a"] = m.group(1)
    elif codec == "dts":
        if not _DTS.fullmatch(cs):
            raise ValueError("codecs isn't dtsc, dtsh, dtsl or dtse: can't play as it is")
    if own is not None:
        f["bitDepth"] = own
    dv = o.get("dv")
    if isinstance(dv, dict) and four:
        p, lv = int_in(dv.get("profile"), 0, 127), int_in(dv.get("level"), 0, 63)
        if p is not None and lv is not None:
            f["dolbyVision"] = "%s.%02d.%02d" % (DV_ENTRY[four], p, lv)
    order = ("codec", "codecs", "profile", "tier", "level", "bitDepth", "chroma", "constraints", "mp4a", "dolbyVision")
    return {k: f[k] for k in order if k in f}


def codecs_problems(c, where):
    """Video 3.5 in one copy as a caller keeps it: the streams and tracks
    a viewer's server can't read (codecs missing or malformed where it is
    required, or unreadable for its codec)."""
    out = []
    streams = ([("video", c["video"])] if c.get("video") else []) + [
        ("audio %s" % t["id"], t) for t in c.get("audioTracks", [])]
    for what, s in streams:
        try:
            read_codecs(s)
        except ValueError as e:
            out.append("%s: %s: %s (video 3.5)" % (where, what, e))
    return out


# ---- play's video members (video 5.3, 5.4, 6.1) ----------------------------------------------

# Video 5.4's table: the form (and ass) a track of a listed format is served in.
SERVED_FORM = {"subrip": ("text", False), "webvtt": ("text", False), "mov_text": ("text", False),
               "ttml": ("text", False), "ass": ("text", True), "ssa": ("text", True),
               "hdmv_pgs_subtitle": ("picture", False), "dvd_subtitle": ("picture", False),
               "dvb_subtitle": ("picture", False)}


def keep_video_answer(o, cp, root):
    """Video 5.3 and 6.1: the video members of play's answer as the
    viewer's server keeps them for the copy cp (as kept), with urls, every
    URL it may build from the stream root and the ticket; raises
    ValueError when it refuses the answer."""
    if not isinstance(o, dict):
        raise ValueError("not an object")
    tk = _form(o.get("ticket"), OPAQUE)
    if tk is None:
        raise ValueError("ticket missing or not 1 to 64 characters of A-Z a-z 0-9 _ -")
    formats = {s["id"]: s.get("format", "") for s in cp.get("subtitleTracks", [])}
    base = root + tk + "/"
    subs, urls = [], []
    for e in o.get("subtitles") if isinstance(o.get("subtitles"), list) else []:
        if not isinstance(e, dict) or len(subs) == 64:
            continue
        sid, form = e.get("id"), e.get("form")
        if not isinstance(sid, str) or sid not in formats or any(x["id"] == sid for x in subs) \
                or form not in ("text", "picture"):
            continue
        ass = e.get("ass") is True and form == "text"
        want = SERVED_FORM.get(formats[sid])
        if want and (want[0] != form or want[1] != ass):
            continue
        x = {"id": sid, "form": form}
        if ass:
            x["ass"] = True
        subs.append(x)
        urls.append(base + ("subtitles/" if form == "text" else "pictures/") + sid)
        if ass:
            urls.append(base + "ass/" + sid)
    n = int_in(o.get("fontCount"), 0, MAX_FONTS)
    fonts = n or 0
    urls += [base + "fonts/%d" % i for i in range(fonts)]
    if cp.get("previewLayout"):
        urls += [base + "trickplay/%d" % i for i in range(sheet_count(cp["previewLayout"]))]
    out = {}
    if subs:
        out["subtitles"] = subs
    out["fontCount"], out["urls"] = fonts, urls
    return out


# ---- fonts (video 6.6) -----------------------------------------------------------

MAX_FONTS, MAX_FONT, MAX_FONT_SET = 64, 10 * 1024 * 1024, 64 * 1024 * 1024


def check_font_set(sizes):
    """Video 6.6's bounds on one copy's fonts, by their sizes in order:
    (count, bytes); raises ValueError at the first bound broken (at most
    64 fonts, each 1 byte to 10 MiB, 64 MiB in all)."""
    if not isinstance(sizes, list):
        raise ValueError("not an array of sizes")
    total = 0
    for i, e in enumerate(sizes):
        n = int_in(e, 1, 2**53 - 1)
        if n is None:
            raise ValueError("font %d: a size that isn't an integer from 1" % i)
        if i >= MAX_FONTS:
            raise ValueError("font %d: more than 64 fonts" % i)
        if n > MAX_FONT:
            raise ValueError("font %d: over 10 MiB" % i)
        if total + n > MAX_FONT_SET:
            raise ValueError("font %d: the copy's fonts over 64 MiB in all" % i)
        total += n
    return len(sizes), total


# ---- What an owning server got wrong -------------------------------------------------

_COPY_FIELDS = {"id", "edition", "durationMs", "startMs", "endMs", "mediaType", "size", "etag", "parts", "video",
                "audioTracks", "subtitleTracks", "chapters", "segments", "previewLayout", "threeD"}
_FIELDS = {
    "video": {"codec", "codecs", "width", "height", "sampleAspect", "rotation", "bitDepth", "colour", "hdr", "dv", "fps",
              "refFrames", "interlaced"},
    "audioTracks": {"id", "language", "channels", "role", "default", "forced", "codec", "codecs", "objects",
                    "sampleRate", "bitDepth", "original"},
    "subtitleTracks": {"id", "language", "role", "default", "forced", "hearingImpaired", "format", "external"},
    "chapters": {"startMs", "name"}, "segments": {"kind", "startMs", "endMs"},
    "dv": {"profile", "level", "compatibility", "el"}, "colour": {"primaries", "transfer", "matrix", "fullRange"},
    "previewLayout": {"tileWidth", "tileHeight", "columns", "rows", "intervalMs", "tileCount"},
}


def _empty(v):
    return v is None or v == "" or v == [] or v == {}


def _same(a, b):
    na, nb = number(a), number(b)
    if na is not None and nb is not None and not isinstance(a, bool) and not isinstance(b, bool):
        return na == nb
    if isinstance(a, Num) or isinstance(b, Num):
        return False
    return type(a) is type(b) and a == b or (isinstance(a, str) and isinstance(b, str) and a == b)


def diff(sent, kept, where, fields=None):
    """Where what was sent and what a caller keeps differ: a list of
    "<where>.<member>: sent …, kept …" (members a caller ignores, and
    empty ones, aside)."""
    out = []
    if isinstance(sent, dict) and isinstance(kept, dict):
        names = set(kept) | (set(sent) & (fields if fields is not None else set(sent)))
        for k in sorted(names):
            a, b = sent.get(k), kept.get(k)
            if _empty(a) and _empty(b):
                continue
            sub = _FIELDS.get(k)
            if isinstance(a, list) and isinstance(b, list) and all(isinstance(x, dict) for x in a):
                if len(a) != len(b):
                    out.append("%s.%s: %d sent, a caller keeps %d" % (where, k, len(a), len(b)))
                    kb = {x.get("id"): x for x in b if "id" in x}
                    for x in a:
                        if x.get("id") in kb:
                            out += diff(x, kb[x["id"]], "%s.%s[%s]" % (where, k, x.get("id")), sub)
                else:
                    for i, (x, y) in enumerate(zip(a, b)):
                        out += diff(x, y, "%s.%s[%s]" % (where, k, x.get("id", i)), sub)
            elif isinstance(a, dict) and isinstance(b, dict):
                out += diff(a, b, "%s.%s" % (where, k), sub)
            elif isinstance(a, list) and isinstance(b, list):
                if [str(x) for x in a] != [str(x) for x in b]:
                    out.append("%s.%s: sent %s, a caller keeps %s" % (where, k, _show(a), _show(b)))
            elif not _same(a, b):
                out.append("%s.%s: sent %s, a caller keeps %s" % (where, k, _show(a), "nothing" if b is None else _show(b)))
        return out
    if not _same(sent, kept):
        out.append("%s: sent %s, a caller keeps %s" % (where, _show(sent), _show(kept)))
    return out


def _show(v):
    if isinstance(v, Num):
        return v.text
    try:
        s = dumps(v)
    except TypeError:
        s = repr(v)
    return s if len(s) <= 80 else s[:77] + "..."


# Members proposal 17 dropped from copies, streams and tracks (video 3.2,
# 3.3; proposal 17 section 4): an owning server no longer sends them.
DROPPED = {"copy": ("name", "container", "bitrateKbps", "added", "previews"),
           "video": ("codecTag", "profile", "level"),
           "audio": ("profile", "layout", "fileIndex", "title"),
           "subtitle": ("fileIndex", "title", "kind")}


def dropped_members(c, where):
    """Members a copy still sends that proposal 17 dropped, and hdr values
    colour or dv now say (video 3.3: hdr holds only enhancement layers and
    dynamic metadata)."""
    out = []

    def one(what, o, group):
        if isinstance(o, dict):
            for f in DROPPED[group]:
                if f in o:
                    out.append("%s: %s sends %s, dropped by proposal 17" % (where, what, f))
    one("the copy", c, "copy")
    v = c.get("video") if isinstance(c.get("video"), dict) else {}
    one("its video", v, "video")
    for t in c.get("audioTracks") if isinstance(c.get("audioTracks"), list) else []:
        one("audio %s" % (t.get("id") if isinstance(t, dict) else "?"), t, "audio")
    for t in c.get("subtitleTracks") if isinstance(c.get("subtitleTracks"), list) else []:
        one("subtitle %s" % (t.get("id") if isinstance(t, dict) else "?"), t, "subtitle")
    if isinstance(c.get("parts"), list):
        out.append("%s: parts is an array of sizes and lengths, not the count of files (video 3.2)" % where)
    hdr = v.get("hdr") if isinstance(v.get("hdr"), list) else []
    for x in ("hdr10", "hlg", "dv"):
        if x in hdr:
            out.append("%s: hdr lists %s, which colour or dv says (video 3.3)" % (where, x))
    if isinstance(v.get("dv"), dict):
        for f in ("bl", "rpu"):
            if f in v["dv"]:
                out.append("%s: dv sends %s, dropped by proposal 17" % (where, f))
    return out


def copy_problems(c, where="copy"):
    """What an owning server got wrong in one copy it sent (video 3.2 to
    3.8): anything a caller would skip, drop, cut, round or change, and
    the owning server's own rules."""
    try:
        kept = keep_video_copy(c)
    except ValueError as e:
        return ["%s: a caller skips it: %s" % (where, e)]
    where = "%s %s" % (where, kept["id"])
    bad = diff(c, kept, where, _COPY_FIELDS)
    bad += dropped_members(c, where)
    bad += codecs_problems(kept, where)
    for f in ("subtitleTracks", "chapters", "segments"):
        if f in c and not isinstance(c[f], list):
            bad.append("%s: %s isn't an array" % (where, f))
    if "video" in c and "video" not in kept:
        bad.append("%s: video counts as absent for a caller (codec, width or height out of form)" % where)
    if "parts" in c and "parts" not in kept and not isinstance(c["parts"], list):
        bad.append("%s: parts counts as absent for a caller (a count of 2 to 16 files)" % where)
    if "previewLayout" in c and "previewLayout" not in kept:
        bad.append("%s: previewLayout is dropped by a caller (video 5.5's bounds)" % where)
    return bad
