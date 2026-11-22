"""What a caller keeps of a video title and its copies (spec/v0/video.md
sections 3.1 to 3.10, as revised by proposal 16), video title keys of the
tmdb, tvdb and local schemes and extras' keys (video 4), the previews
answer (video 5.5, 5.6), language tags as catalog 3 sends them, and the
JSON helpers the vectors' outputs need.

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
# values are tokens, kept known or not. threeD is the one closed list left.
THREE_D = ["sbs", "tab", "mvc"]
_TOKEN = re.compile(r"[a-z0-9._-]{1,32}")
MAX_DURATION = 8640000000
MAX_SIZE = 999999999999999
_CODEC_TAG = re.compile(r"[A-Za-z0-9]{1,8}")
_LAYOUT = re.compile(r"[A-Za-z0-9()._-]{1,32}")
_CODECS = re.compile(r"[A-Za-z0-9.-]{1,64}")
_FPS = re.compile(r"([1-9][0-9]{0,6})/([1-9][0-9]{0,6})")
_ASPECT = re.compile(r"([1-9][0-9]{0,4}):([1-9][0-9]{0,4})")
MAX_COPIES = 16
MAX_HDR = 8
_DATE = re.compile(r"([0-9]{4})-([0-9]{2})-([0-9]{2})")


def _form(v, rx):
    return v if isinstance(v, str) and rx.fullmatch(v) else None


def _one_of(v, lst):
    return v if isinstance(v, str) and v in lst else None


def token(v):
    """v when it is a string and a token (core 8.5: 1 to 32 characters of
    a-z 0-9 . _ -), else None: an open field keeps any token, known or not."""
    return v if isinstance(v, str) and _TOKEN.fullmatch(v) else None


def text_fact(v, n):
    """A fact given as plain text (a profile, video 3.3): cleaned, None when
    empty or longer than n code points, never cut."""
    if not isinstance(v, str):
        return None
    t = clean_name(v, n + 1)
    return t if t and len(t) <= n else None


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


def _keep_stream(o):
    if not isinstance(o, dict):
        return None
    codec = token(o.get("codec"))
    w, h = int_in(o.get("width"), 1, 100000), int_in(o.get("height"), 1, 100000)
    if codec is None or w is None or h is None:
        return None
    s = {"codec": codec}
    _put(s, "codecTag", _form(o.get("codecTag"), _CODEC_TAG))
    _put(s, "codecs", _form(o.get("codecs"), _CODECS))
    _put(s, "profile", text_fact(o.get("profile"), 64))
    lv = number(o.get("level"))
    if lv is not None and 0 <= lv <= 1000:
        s["level"] = lv
    s["width"], s["height"] = w, h
    _put(s, "sampleAspect", aspect(o.get("sampleAspect")))
    r = int_in(o.get("rotation"), 0, 270)
    if r is not None and r % 90 == 0:
        s["rotation"] = r
    _put(s, "bitDepth", int_in(o.get("bitDepth"), 1, 16))
    c = o.get("colour")
    if isinstance(c, dict):
        col = {}
        for k in ("primaries", "transfer", "matrix", "range"):
            _put(col, k, token(c.get(k)))
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
    if isinstance(dv, dict) and "dv" in s.get("hdr", []):
        p, lv2, c = int_in(dv.get("profile"), 0, 127), int_in(dv.get("level"), 0, 63), int_in(dv.get("compatibility"), 0, 15)
        if None not in (p, lv2, c):
            s["dv"] = {"profile": p, "level": lv2, "compatibility": c}
            for f in ("el", "bl", "rpu"):
                _put(s["dv"], f, boolean(dv.get(f)))
    _put(s, "fps", fps(o.get("fps")))
    _put(s, "interlaced", boolean(o.get("interlaced")))
    return s


def _keep_parts(v, size):
    if not isinstance(v, list) or not 1 <= len(v) <= 16:
        return None
    out, total = [], 0
    for e in v:
        if not isinstance(e, dict):
            return None
        s, d = int_in(e.get("size"), 1, MAX_SIZE), int_in(e.get("durationMs"), 0, MAX_DURATION)
        if s is None or d is None:
            return None
        total += s
        out.append({"size": s, "durationMs": d})
    return out if total == size else None


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
        _put(t, "fileIndex", int_in(o.get("fileIndex"), 0, 999))
        t["codec"] = codec
        _put(t, "codecs", _form(o.get("codecs"), _CODECS))
        _put(t, "profile", text_fact(o.get("profile"), 64))
        _put(t, "layout", _form(o.get("layout"), _LAYOUT))
        _put(t, "sampleRate", int_in(o.get("sampleRate"), 8000, 768000))
        _put(t, "bitDepth", int_in(o.get("bitDepth"), 8, 32))
        _put(t, "title", plain(o.get("title"), 64))
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
        for f in ("default", "forced", "hearingImpaired"):
            _put(t, f, boolean(o.get(f)))
        _put(t, "format", token(o.get("format")))
        _put(t, "external", boolean(o.get("external")))
        _put(t, "fileIndex", int_in(o.get("fileIndex"), 0, 999))
        _put(t, "title", plain(o.get("title"), 64))
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
        g = {"kind": kind, "startMs": s, "endMs": t}
        _put(g, "final", boolean(o.get("final")))
        out.append(g)
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
    container = token(o.get("container"))
    if container is None:
        raise ValueError("container isn't a token (core 8.5)")
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
    _put(c, "name", plain(o.get("name"), 64))
    _put(c, "edition", token(o.get("edition")))
    c["durationMs"] = d
    _put(c, "startMs", int_in(o.get("startMs"), 0, d))
    c["container"], c["size"], c["etag"] = container, size, etag
    _put(c, "parts", _keep_parts(o.get("parts"), size))
    _put(c, "bitrateKbps", int_in(o.get("bitrateKbps"), 1, 10000000))
    _put(c, "video", _keep_stream(o.get("video")))
    c["audioTracks"] = _keep_audio(audio)
    if isinstance(o.get("subtitleTracks"), list):
        _put(c, "subtitleTracks", _keep_subtitles(o["subtitleTracks"]))
    _put(c, "chapters", _keep_chapters(o.get("chapters"), d))
    _put(c, "segments", _keep_segments(o.get("segments"), d))
    _put(c, "previews", boolean(o.get("previews")))
    if real_day(o.get("added")):
        c["added"] = o["added"]
    _put(c, "threeD", _one_of(o.get("threeD"), THREE_D))
    return c


# ---- Titles (video 3.1, 3.9, 3.10) ----------------------------------------------------

def keep_video_title(o, libraries):
    """Catalog 3.3 and video 3.1, 3.8 to 3.10 for one entry of titles:
    what the caller keeps (a dict in the vectors' order); raises
    ValueError with why it skips the title."""
    if not isinstance(o, dict):
        raise ValueError("not an object")
    key, kind, lib = o.get("key"), o.get("kind"), o.get("library")
    if not all(isinstance(x, str) for x in (key, kind, lib)):
        raise ValueError("key, kind or library isn't a string")
    try:
        k = parse_video_key(key)
    except ValueError as e:
        raise ValueError("key: %s" % e)
    if kind != k.kind:
        raise ValueError("kind %r, but the key names a %s" % (kind, k.kind))
    if lib not in libraries:
        raise ValueError("library %r wasn't asked for" % lib)
    t = {"key": key, "kind": kind, "library": lib}
    if kind == "extra":
        et = token(o.get("extraType"))
        if et is None:
            raise ValueError("an extra whose extraType is missing or isn't a token")
        t["extraType"] = et
        _put(t, "name", plain(o.get("name"), 64))
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
    ids = o.get("ids")
    if isinstance(ids, dict) and k.scheme != "local" and kind in ("movie", "series", "episode"):
        kept = {}
        for scheme, val in ids.items():
            if isinstance(val, str) and ((scheme == "imdb" and _IMDB.fullmatch(val))
                                         or (scheme == "tvdb" and _ID.fullmatch(val))
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


_TRICKPLAY = ["tileWidth", "tileHeight", "columns", "rows", "intervalMs", "tileCount"]


def keep_trickplay(o, root):
    """Video 5.5's reading of a trickplay object: raises ValueError when it
    refuses the whole answer (a field of another JSON type, a sheet URL
    that isn't the one video 6.1 gives); None when it drops the previews;
    else the kept object (with "ticket" not in it: see trickplay_ticket)."""
    if not isinstance(o, dict):
        raise ValueError("trickplay isn't an object")
    n = {}
    for f in _TRICKPLAY:
        if f in o:
            i = int_in(o[f], -MAX_INT, MAX_INT)
            if i is None:
                raise ValueError("%s isn't an integer" % f)
            n[f] = i
    sheets = None
    if "sheets" in o:
        if not isinstance(o["sheets"], list):
            raise ValueError("sheets isn't an array")
        sheets, ticket = [], ""
        for i, s in enumerate(o["sheets"]):
            if not isinstance(s, str):
                raise ValueError("sheets[%d] isn't a string" % i)
            rest = s[len(root):] if s.startswith(root) else None
            tk, _, tail = (rest or "").partition("/")
            if rest is None or "/" not in rest or not OPAQUE.fullmatch(tk) or tail != "trickplay/%d" % i:
                raise ValueError("sheets[%d] isn't <stream root><ticket>/trickplay/%d" % (i, i))
            if ticket and tk != ticket:
                raise ValueError("sheets[%d] names another ticket" % i)
            ticket = tk
            sheets.append(s)

    def within(f, lo, hi):
        return f in n and lo <= n[f] <= hi
    if not (within("tileWidth", 1, 2048) and within("tileHeight", 1, 2048) and within("columns", 1, 100)
            and within("rows", 1, 100) and within("intervalMs", 100, 3600000) and within("tileCount", 1, MAX_INT)) \
            or sheets is None:
        return None
    if n["columns"] * n["tileWidth"] > 4096 or n["rows"] * n["tileHeight"] > 4096:
        return None
    per = n["columns"] * n["rows"]
    want = -(-n["tileCount"] // per)
    if want > 1000 or len(sheets) != want:
        return None
    return dict([(f, n[f]) for f in _TRICKPLAY] + [("sheets", sheets)])


def keep_previews_answer(o, root):
    """Video 5.6: what the viewer's server keeps of a previews answer ({}
    when it drops the previews); raises ValueError when it refuses it."""
    if not isinstance(o, dict):
        raise ValueError("not an object")
    out = {}
    if "trickplay" in o:
        tp = keep_trickplay(o["trickplay"], root)
        if tp is not None:
            out["trickplay"] = tp
    return out


# ---- fonts (video 5.3, 6.6) -----------------------------------------------------------

MAX_FONTS, MAX_FONT, MAX_FONT_SET = 64, 10 * 1024 * 1024, 64 * 1024 * 1024


def keep_fonts(v, root):
    """Video 5.3's reading of play's fonts: the URLs kept; raises
    ValueError when the member is dropped (each entry exactly
    <stream root><ticket>/fonts/<index>, one ticket, at most 64)."""
    if not isinstance(v, list):
        raise ValueError("fonts isn't an array")
    if len(v) > MAX_FONTS:
        raise ValueError("%d fonts, more than 64" % len(v))
    ticket = ""
    for i, u in enumerate(v):
        if not isinstance(u, str):
            raise ValueError("fonts[%d] isn't a string" % i)
        rest = u[len(root):] if u.startswith(root) else None
        tk, _, tail = (rest or "").partition("/")
        if rest is None or "/" not in rest or not OPAQUE.fullmatch(tk) or tail != "fonts/%d" % i:
            raise ValueError("fonts[%d] isn't <stream root><ticket>/fonts/%d" % (i, i))
        if ticket and tk != ticket:
            raise ValueError("fonts[%d] names another ticket" % i)
        ticket = tk
    return list(v)


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

_COPY_FIELDS = {"id", "name", "edition", "durationMs", "startMs", "container", "size", "etag", "parts", "bitrateKbps",
                "video", "audioTracks", "subtitleTracks", "chapters", "segments", "previews", "added", "threeD"}
_FIELDS = {
    "video": {"codec", "codecTag", "codecs", "profile", "level", "width", "height", "sampleAspect", "rotation",
              "bitDepth", "hdr", "dv", "fps", "interlaced", "colour"},
    "audioTracks": {"id", "language", "channels", "role", "default", "forced", "fileIndex", "codec", "codecs", "profile",
                    "layout", "sampleRate", "bitDepth", "title", "original"},
    "subtitleTracks": {"id", "language", "default", "forced", "hearingImpaired", "format", "external", "fileIndex", "title"},
    "chapters": {"startMs", "name"}, "segments": {"kind", "startMs", "endMs", "final"}, "parts": {"size", "durationMs"},
    "dv": {"profile", "level", "compatibility", "el", "bl", "rpu"}, "colour": {"primaries", "transfer", "matrix", "range"},
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


# MSFP's own names before proposal 17 that aren't ffmpeg's (video 3.5).
OLD_NAMES = {"container": {"mkv": "matroska", "ts": "mpegts", "m2ts": "mpegts", "other": None},
             "codec": {"other": None},
             "format": {"pgs": "hdmv_pgs_subtitle", "vobsub": "dvd_subtitle", "dvb": "dvb_subtitle", "other": None}}


def old_names(c, where):
    """Names a copy gives in the closed lists video 0 had before proposal
    17, which ffmpeg doesn't use (video 3.5: ffmpeg's muxer and codec names)."""
    out = []

    def one(what, v, field):
        new = OLD_NAMES[field].get(v) if isinstance(v, str) else None
        if isinstance(v, str) and v in OLD_NAMES[field]:
            out.append("%s: %s %r is MSFP's name before proposal 17, not ffmpeg's%s (video 3.5)" % (
                where, what, v, " (%r)" % new if new else ""))
    one("container", c.get("container"), "container")
    v = c.get("video") if isinstance(c.get("video"), dict) else {}
    one("video codec", v.get("codec"), "codec")
    for t in c.get("audioTracks") if isinstance(c.get("audioTracks"), list) else []:
        if isinstance(t, dict):
            one("audio %s codec" % t.get("id"), t.get("codec"), "codec")
    for t in c.get("subtitleTracks") if isinstance(c.get("subtitleTracks"), list) else []:
        if isinstance(t, dict):
            one("subtitle %s format" % t.get("id"), t.get("format"), "format")
    hdr = v.get("hdr") if isinstance(v.get("hdr"), list) else []
    for x in ("hdr10", "hlg"):
        if x in hdr:
            out.append("%s: hdr lists %s, which colour says (video 3.3: hdr holds only enhancement layers and "
                       "dynamic metadata)" % (where, x))
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
    bad += old_names(c, where)
    for f in ("subtitleTracks", "chapters", "segments", "parts"):
        if f in c and not isinstance(c[f], list):
            bad.append("%s: %s isn't an array" % (where, f))
    if "video" in c and "video" not in kept:
        bad.append("%s: video counts as absent for a caller (codec, width or height out of form)" % where)
    if "parts" in c and "parts" not in kept:
        bad.append("%s: parts count as absent for a caller (at most 16, sizes adding up to size)" % where)
    return bad
