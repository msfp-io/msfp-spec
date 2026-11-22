"""The media rules, as revised by proposal 16: HLS playlists
(spec/v0/media.md section 3.1), ranges, the file's answers and the scan
cap (media 2.1, 3.2, 4.2), WebVTT, ASS, PGS and fonts (video.md sections
6.3 to 6.6), a local title's image (catalog.md section 4.4), and a JPEG's
size from its header (video 6.2)."""

import html
import math
import re
import unicodedata

from .wire import WHITE_SPACE, check_media_name


class Refused(ValueError):
    pass


_WS = "".join(sorted(WHITE_SPACE))


def _trim(s):
    """White space (Unicode's White_Space) removed at both ends."""
    return s.strip(_WS)


def _fold_eq(a, b):
    """Equal without ASCII case only."""
    low = str.maketrans("ABCDEFGHIJKLMNOPQRSTUVWXYZ", "abcdefghijklmnopqrstuvwxyz")
    return a.translate(low) == b.translate(low)


_INT = re.compile(r"-?[0-9]+")


def _int_in(s, lo, hi):
    """s is an integer (an optional -, digits) within lo to hi."""
    return bool(_INT.fullmatch(s)) and lo <= int(s) <= hi


def _text(raw):
    """Bytes as text: invalid UTF-8 becomes U+FFFD, which the rules drop
    wherever they drop it."""
    return raw.decode("utf-8", "replace") if isinstance(raw, (bytes, bytearray)) else raw


# ---- HLS (media 3.1) -------------------------------------------------------------

MASTER_TAGS = {"#EXTM3U", "#EXT-X-VERSION", "#EXT-X-INDEPENDENT-SEGMENTS", "#EXT-X-STREAM-INF", "#EXT-X-MEDIA"}
MEDIA_TAGS = {"#EXTM3U", "#EXT-X-VERSION", "#EXT-X-TARGETDURATION", "#EXT-X-MEDIA-SEQUENCE", "#EXT-X-PLAYLIST-TYPE",
              "#EXT-X-INDEPENDENT-SEGMENTS", "#EXTINF", "#EXT-X-DISCONTINUITY", "#EXT-X-ENDLIST", "#EXT-X-MAP"}
_URI = re.compile(r"URI=", re.I)
_EXTINF = re.compile(r"#EXTINF:([0-9]+(?:\.[0-9]+)?)(?:,.*)?")
_TARGET = re.compile(r"#EXT-X-TARGETDURATION:([0-9]+)")
_ATTR = re.compile(r'([A-Z0-9-]+)=("[^"\r\n]*"|[^",]*)(?:,|$)')
_AUDIO_ID = re.compile(r"a(0|[1-9][0-9]{0,2})")


def hls_attributes(s):
    """An attribute list (RFC 8216 section 4.2) as a dict, values as
    written (quoted ones with their quotes); raises ValueError."""
    out = {}
    while s:
        m = _ATTR.match(s)
        if not m:
            raise ValueError("a malformed attribute list")
        if m.group(1) in out:
            raise ValueError("an attribute twice")
        out[m.group(1)] = m.group(2)
        s = s[m.end():]
    return out


def _tag_is(line, tag):
    return line == tag or line.startswith(tag + ":")


def _media_playlist(lines, last_uri):
    """Step 9 for a media playlist: None, or why it is refused."""
    vod = ends = 0
    end_at, target = -1, -1
    for i, line in enumerate(lines):
        if line == "#EXT-X-PLAYLIST-TYPE:VOD":
            vod += 1
        elif _tag_is(line, "#EXT-X-PLAYLIST-TYPE"):
            return "a playlist type other than VOD: a growing playlist"
        elif line == "#EXT-X-ENDLIST":
            ends += 1
            end_at = i
        elif _tag_is(line, "#EXT-X-TARGETDURATION"):
            m = _TARGET.fullmatch(line)
            if not m or not _int_in(m.group(1), 1, 60):
                return "a target duration that isn't an integer from 1 to 60"
            if target < 0:
                target = int(m.group(1))
    if vod != 1:
        return "not exactly one EXT-X-PLAYLIST-TYPE:VOD"
    if ends != 1:
        return "not exactly one EXT-X-ENDLIST: a growing playlist"
    if end_at < last_uri:
        return "EXT-X-ENDLIST before the last URI line"
    if target < 0:
        return "no EXT-X-TARGETDURATION"
    for line in lines:
        if not _tag_is(line, "#EXTINF"):
            continue
        m = _EXTINF.fullmatch(line)
        if not m:
            return "an EXTINF duration that isn't a decimal"
        d = float(m.group(1))
        if d <= 0:
            return "an EXTINF duration not above 0"
        # Go's math.Round: halves away from zero.
        if math.floor(d + 0.5) > target:
            return "an EXTINF duration that rounds above the target duration"
    return None


def check_playlist(b, audio_ids=None):
    """The viewer's server's checks of media 3.1, steps 1 to 10, in order.
    audio_ids: the playback's audio track IDs, for step 10 (None: any
    a<n>). Returns (None, uris) when they pass, else (why, [])."""
    if len(b) > 2 * 1024 * 1024:
        return "step 1: over 2 MiB", []
    text = b.decode("utf-8", "replace") if isinstance(b, (bytes, bytearray)) else b
    lines = [x[:-1] if x.endswith("\r") else x for x in text.split("\n")]
    if lines[0] != "#EXTM3U":
        return "step 3: the first line isn't #EXTM3U", []
    lines = lines[1:]
    uris, last_uri = [], -1
    for i, line in enumerate(lines):
        if line == "" or line.startswith("#"):
            continue
        if check_media_name(line):
            return "step 4: a URI line that isn't a bare name: %r" % line[:80], []
        uris.append(line)
        last_uri = i
    for line in lines:
        if not line.startswith("#"):
            continue
        for m in _URI.finditer(line):
            rest = line[m.end():]
            if not rest.startswith('"'):
                return "step 5: an unquoted URI attribute: %r" % line[:80], []
            end = rest.find('"', 1)
            if end < 0:
                return "step 5: a URI attribute without its closing quote", []
            if check_media_name(rest[1:end]):
                return "step 5: a URI attribute that isn't a bare name: %r" % rest[1:end][:80], []
            uris.append(rest[1:end])
    for line in lines:
        if line == "#EXT-X-DEFINE" or line.startswith("#EXT-X-DEFINE:"):
            return "step 6: EXT-X-DEFINE", []
    for line in lines:
        if line.startswith("#EXT-X-KEY:") or line.startswith("#EXT-X-SESSION-KEY:"):
            return "step 7: EXT-X-KEY or EXT-X-SESSION-KEY", []
    if len(uris) > 50000:
        return "step 8: over 50,000 URIs", []
    is_media = any(_tag_is(x, "#EXTINF") for x in lines)
    is_master = any(_tag_is(x, "#EXT-X-STREAM-INF") for x in lines)
    if not is_media and not is_master:
        return "step 9: neither EXT-X-STREAM-INF nor EXTINF", []
    if is_media:
        why = _media_playlist(lines, last_uri)
        if why:
            return "step 9: " + why, []
    if is_master:
        n = 0
        for line in lines:
            if not _tag_is(line, "#EXT-X-MEDIA"):
                continue
            n += 1
            if n > 32:
                return "step 10: over 32 EXT-X-MEDIA", []
            try:
                a = hls_attributes(line[len("#EXT-X-MEDIA:"):] if line.startswith("#EXT-X-MEDIA:") else "")
            except ValueError as e:
                return "step 10: EXT-X-MEDIA: %s" % e, []
            if a.get("TYPE") != "AUDIO":
                return "step 10: an EXT-X-MEDIA without TYPE=AUDIO", []
            name = a.get("NAME", "")
            nid = name[1:-1] if len(name) >= 2 and name[0] == '"' else None
            ok = nid is not None and _AUDIO_ID.fullmatch(nid) and (audio_ids is None or nid in audio_ids)
            if not ok:
                return "step 10: an EXT-X-MEDIA whose NAME isn't an audio track ID of the playback", []
    return None, uris


_MEDIA_ATTRS = {"TYPE", "GROUP-ID", "NAME", "LANGUAGE", "DEFAULT", "AUTOSELECT", "CHANNELS", "URI"}


def check_owner_playlist(b, master, audio_ids=None):
    """What the owning server sends (media 3.1): the viewer's checks, and
    besides: lines end in a line feed, only the subset's tags, URIs only
    in URI lines, #EXT-X-MAP:URI="<name>" and #EXT-X-MEDIA's URI, a URI
    line only after #EXT-X-STREAM-INF or #EXTINF; a media playlist
    complete (VOD, #EXT-X-ENDLIST its last tag); #EXT-X-MEDIA with exactly
    the attributes media 3.1 lists, one DEFAULT=YES, every
    #EXT-X-STREAM-INF then naming AUDIO="audio". Returns (problems, uris)."""
    why, uris = check_playlist(b, audio_ids)
    if why:
        return [why], []
    text = b.decode("utf-8", "replace")
    bad = []
    if not text.endswith("\n"):
        bad.append("the last line doesn't end in a line feed")
    if "\r" in text:
        bad.append("a carriage return (lines end in a line feed)")
    allowed = MASTER_TAGS if master else MEDIA_TAGS
    want_uri = False
    tags, renditions, defaults, stream_infs = [], 0, 0, []
    for line in text.split("\n"):
        if not line:
            continue
        if line.startswith("#"):
            tag = line.split(":", 1)[0]
            tags.append(line)
            if tag not in allowed:
                bad.append("%s isn't in the %s playlist's subset" % (tag if tag.startswith("#EXT") else "a comment",
                                                                   "master" if master else "media"))
            if tag == "#EXT-X-MEDIA" and master:
                renditions += 1
                try:
                    a = hls_attributes(line[len("#EXT-X-MEDIA:"):])
                except ValueError as e:
                    bad.append("EXT-X-MEDIA: %s" % e)
                    a = {}
                extra = set(a) - _MEDIA_ATTRS
                missing = {"TYPE", "GROUP-ID", "NAME", "DEFAULT", "AUTOSELECT", "URI"} - set(a)
                if extra or missing:
                    bad.append("EXT-X-MEDIA with %s%s (media 3.1: exactly TYPE, GROUP-ID, NAME, LANGUAGE, DEFAULT, "
                               "AUTOSELECT, CHANNELS, URI)" % ("attributes " + ", ".join(sorted(extra)) if extra else "",
                                                               " without " + ", ".join(sorted(missing)) if missing else ""))
                if a.get("GROUP-ID") not in (None, '"audio"'):
                    bad.append('EXT-X-MEDIA GROUP-ID %s, not "audio"' % a.get("GROUP-ID"))
                for f in ("DEFAULT", "AUTOSELECT"):
                    if f in a and a[f] not in ("YES", "NO"):
                        bad.append("EXT-X-MEDIA %s=%s" % (f, a[f]))
                if a.get("DEFAULT") == "YES":
                    defaults += 1
                if "CHANNELS" in a and not re.fullmatch(r'"[1-9][0-9]*"', a["CHANNELS"]):
                    bad.append("EXT-X-MEDIA CHANNELS=%s isn't a quoted number" % a["CHANNELS"])
                if "LANGUAGE" in a and not re.fullmatch(r'"[A-Za-z0-9-]{2,12}"', a["LANGUAGE"]):
                    bad.append("EXT-X-MEDIA LANGUAGE=%s isn't a quoted tag" % a["LANGUAGE"])
            elif tag == "#EXT-X-STREAM-INF":
                stream_infs.append(line)
            if _URI.search(line) and not (re.fullmatch(r'#EXT-X-MAP:URI="[A-Za-z0-9._-]{1,64}"', line)
                                          or (master and tag == "#EXT-X-MEDIA")):
                bad.append("a URI outside #EXT-X-MAP:URI=\"<name>\" and #EXT-X-MEDIA: %r" % line[:80])
            want_uri = tag in ("#EXT-X-STREAM-INF", "#EXTINF")
        else:
            if not want_uri:
                bad.append("a URI line not after #EXT-X-STREAM-INF or #EXTINF: %r" % line[:80])
            want_uri = False
    if master and renditions:
        if defaults != 1:
            bad.append("%d EXT-X-MEDIA with DEFAULT=YES, not exactly one" % defaults)
        for s in stream_infs:
            if 'AUDIO="audio"' not in s:
                bad.append("an EXT-X-STREAM-INF without AUDIO=\"audio\" beside audio renditions")
                break
    if not master:
        if "#EXT-X-PLAYLIST-TYPE:VOD" not in tags:
            bad.append("no #EXT-X-PLAYLIST-TYPE:VOD")
        if not tags or tags[-1] != "#EXT-X-ENDLIST":
            bad.append("#EXT-X-ENDLIST isn't the last tag: the playlist isn't complete")
    return bad, uris


# ---- Ranges, the file and scans (media 2.1, 3.2, 4.2) -------------------------------

_RANGE = re.compile(r"bytes=(?:([0-9]{1,15})-([0-9]{1,15})?|-([0-9]{1,15}))")
_CONTENT_RANGE = re.compile(r"bytes ([0-9]+)-([0-9]+)/([0-9]+|\*)")
SCAN_FLOOR = 4 * 1024 * 1024        # scanBytes absent or out of range (playback 5)
SCAN_TOP = 1024 * 1024 * 1024
SCAN_DEFAULT = 32 * 1024 * 1024     # the recommended default for owners


def scan_cap(scan_bytes):
    """The bytes one scan ticket may read in all, for the scanBytes an owner
    sends in limits (playback 5): it, within 4 MiB to 1 GiB, else the floor."""
    if isinstance(scan_bytes, int) and not isinstance(scan_bytes, bool) and SCAN_FLOOR <= scan_bytes <= SCAN_TOP:
        return scan_bytes
    return SCAN_FLOOR


def parse_range(h):
    """A Range header (media 2.1): (form, first, last, count), form 1 for
    first-last, 2 for first-, 3 for -count; raises Refused when malformed."""
    m = _RANGE.fullmatch(h)
    if not m:
        raise Refused("malformed: 416, the file not read")
    if m.group(3) is not None:
        if int(m.group(3)) < 1:
            raise Refused("malformed: 416, the file not read")
        return 3, 0, 0, int(m.group(3))
    if m.group(2) is not None:
        if int(m.group(1)) > int(m.group(2)):
            raise Refused("malformed: 416, the file not read")
        return 1, int(m.group(1)), int(m.group(2)), 0
    return 2, int(m.group(1)), 0, 0


def check_range(h, size):
    """The bytes a Range asks of a file of size bytes: (start, end), both
    included; raises Refused (416) when malformed or not satisfiable."""
    form, first, last, count = parse_range(h)
    if form == 3:
        if size == 0:
            raise Refused("not satisfiable: 416, the file has no byte")
        return max(0, size - count), size - 1
    if first >= size:
        raise Refused("not satisfiable: 416, first at or past the end")
    end = size - 1
    if form == 1 and last < end:
        end = last
    return first, end


def check_file_answer(rng, size, etag, status, etag_header=None, content_range=None, content_length=None):
    """The viewer's server's check of an answer for the file (media 2.1,
    2.3, 3.2): None when it relays it, else why. rng is the Range sent
    (None or "" for none); headers None when absent."""
    satisfiable = False
    if rng:
        try:
            form, first, last, count = parse_range(rng)
        except Refused:
            return "a malformed Range is never sent"
        try:
            check_range(rng, size)
            satisfiable = True
        except Refused:
            pass
    if status == 200:
        if rng:
            return "a 200 to a satisfiable Range" if satisfiable else "a 200 to a Range"
    elif status == 206:
        if not rng:
            return "a 206 without a Range"
    elif status == 416:
        return "416: a failed fetch (passed to the viewer as a 416 of its own when the viewer asked that range)"
    else:
        return "status %d: a failed fetch" % status
    if etag_header != '"%s"' % etag:
        return "an ETag other than the copy's etag, strong: the copy changed"
    if status == 200:
        if content_length != str(size):
            return "a 200 whose Content-Length isn't the copy's size"
        return None
    m = _CONTENT_RANGE.fullmatch(content_range or "")
    if not m:
        return "a Content-Range not of the form bytes <start>-<end>/<total>"
    if m.group(3) == "*":
        return "a Content-Range total of * for the file"
    start, end, total = int(m.group(1)), int(m.group(2)), int(m.group(3))
    if max(start, end, total) > 2**63 - 1:
        return "a Content-Range number too large"
    if start > end or end >= total:
        return "a Content-Range without start <= end < total"
    if total != size:
        return "a Content-Range total other than the copy's size: the copy changed"
    want = max(0, total - count) if form == 3 else first
    if start != want:
        return "a Content-Range start other than the range's"
    if form == 1 and end > last:
        return "a Content-Range end past the range's last"
    if content_length is not None and content_length != str(end - start + 1):
        return "a Content-Length other than end - start + 1"
    return None


def scan_answer(sent, rng, size, scan_bytes=None):
    """The owning server's answer to a fetch under a scan ticket that has
    already sent sent bytes (media 4.2), for the scanBytes it announced:
    (206, its Content-Range) cut at the cap, or (416, "bytes */<size>")."""
    cap = scan_cap(scan_bytes)
    unsatisfied = "bytes */%d" % size
    if not rng or sent >= cap:
        return 416, unsatisfied
    try:
        start, end = check_range(rng, size)
    except Refused:
        return 416, unsatisfied
    end = min(end, start + (cap - sent) - 1)
    return 206, "bytes %d-%d/%d" % (start, end, size)


# ---- Checking a 206 (media 2.1) --------------------------------------------------

def check_206(range_form, headers, body, first=None, last=None, count=None):
    """Checks a 206 against the range asked for: None when it answers it."""
    h = {k.lower(): v for k, v in headers}
    cr = h.get("content-range", "")
    m = re.fullmatch(r"bytes ([0-9]+)-([0-9]+)/([0-9]+|\*)", cr.strip())
    if not m:
        return "Content-Range %r isn't bytes <start>-<end>/<total>" % cr
    start, end = int(m.group(1)), int(m.group(2))
    total = None if m.group(3) == "*" else int(m.group(3))
    if start > end or (total is not None and end >= total):
        return "Content-Range %r: not start <= end < total" % cr
    if count is not None:
        if total is not None and start != max(total - count, 0):
            return "Content-Range %r for bytes=-%d: start isn't total - count" % (cr, count)
    elif start != first:
        return "Content-Range %r: start isn't %d" % (cr, first)
    if last is not None and end > last:
        return "Content-Range %r: end past %d" % (cr, last)
    cl = h.get("content-length")
    if cl is not None and cl.strip() != str(end - start + 1):
        return "Content-Length %s isn't end - start + 1 = %d" % (cl, end - start + 1)
    if len(body) != end - start + 1:
        return "%d bytes in the body, the range has %d" % (len(body), end - start + 1)
    return None


# ---- WebVTT (video 6.3) ------------------------------------------------------------

_TIMING = re.compile(r"((?:[0-9]{1,3}:)?[0-9]{2}:[0-9]{2}\.[0-9]{3})[\t\f ]+-->[\t\f ]+"
                     r"((?:[0-9]{1,3}:)?[0-9]{2}:[0-9]{2}\.[0-9]{3})([\t\f ].*)?")
_DIRECTION = {0x061C, 0x200E, 0x200F} | set(range(0x202A, 0x202F)) | set(range(0x2066, 0x206A))
_PCT = r"[0-9]{1,3}(?:\.[0-9]{1,3})?%"
_VTT_LINE = re.compile(r"(" + _PCT + r"|-?[0-9]+)(?:,(?:start|center|end))?")
_VTT_POSITION = re.compile(r"(" + _PCT + r")(?:,(?:line-left|center|line-right))?")
_VTT_SIZE = re.compile(r"(" + _PCT + r")")
_COLOURS = {"white", "lime", "cyan", "red", "yellow", "magenta", "blue", "black"}


def _pct_ok(s):
    return 0 <= float(s[:-1]) <= 100


def _vtt_setting(name, value):
    if name == "line":
        m = _VTT_LINE.fullmatch(value)
        if not m:
            return False
        return _pct_ok(m.group(1)) if m.group(1).endswith("%") else _int_in(m.group(1), -1000, 1000)
    if name in ("position", "size"):
        m = (_VTT_POSITION if name == "position" else _VTT_SIZE).fullmatch(value)
        return bool(m) and _pct_ok(m.group(1))
    if name == "align":
        return value in ("start", "center", "end", "left", "right")
    return False


def _vtt_settings(s):
    """The kept cue settings, written in their order, each after a space."""
    kept, seen = {}, set()
    for f in re.split(r"[ \t\f]+", s or ""):
        i = f.find(":")
        if i < 0:
            continue
        name = f[:i]
        if name in seen:
            continue
        seen.add(name)
        if _vtt_setting(name, f[i + 1:]):
            kept[name] = f
    return "".join(" " + kept[n] for n in ("line", "position", "size", "align") if n in kept)


def _drop_invisible(s):
    return "".join(ch for ch in s if ch != "�" and ord(ch) not in _DIRECTION)


def _tag_start(s, i):
    return s[i] == "<" and i + 1 < len(s) and (s[i + 1].isalpha() or s[i + 1] in "/!?")


def _vtt_kept(t):
    if t in ("<i>", "</i>", "<b>", "</b>", "</c>"):
        return True
    if not (t.startswith("<c.") and t.endswith(">")):
        return False
    classes = t[3:-1].split(".")

    def colour(c):
        return (True, c[3:] in _COLOURS) if c.startswith("bg_") else (False, c in _COLOURS)
    if len(classes) == 1:
        return colour(classes[0])[1]
    if len(classes) == 2:
        (b1, o1), (b2, o2) = colour(classes[0]), colour(classes[1])
        return o1 and o2 and b1 != b2
    return False


def _untag(s):
    out, i = [], 0
    while i < len(s):
        if not _tag_start(s, i):
            out.append(s[i])
            i += 1
            continue
        j = s.find(">", i)
        if j < 0:
            break
        out.append(" ")
        i = j + 1
    return "".join(out)


def _white(ch):
    return ch in WHITE_SPACE


def _clean_cue_line(s):
    """Steps 1 to 7 of cleaning a cue text line: a list of (is_tag, text),
    the text escaped, or None when no text is left."""
    s = _drop_invisible(s)
    pieces, i = [], 0
    while i < len(s):
        if _tag_start(s, i):
            j = s.find(">", i)
            if j < 0:
                break
            t = s[i:j + 1]
            pieces.append((True, t) if _vtt_kept(t) else (False, " "))
            i = j + 1
            continue
        j = i + 1
        while j < len(s) and not _tag_start(s, j):
            j += 1
        pieces.append((False, s[i:j]))
        i = j
    out = []

    def add_text(t):
        if out and not out[-1][0]:
            out[-1] = (False, out[-1][1] + t)
        else:
            out.append((False, t))
    size = tags = 0
    pending = has_text = last_space = cut = False
    for is_tag, text in pieces:
        if cut:
            break
        if is_tag:
            if tags < 32:
                if pending and has_text and not last_space:
                    if size + 1 > 500:
                        break
                    add_text(" ")
                    size += 1
                    last_space = True
                pending = False
                tags += 1
                out.append((True, text))
            continue
        t = _untag(_drop_invisible(html.unescape(text)))
        for ch in t:
            if ch == "�" or ord(ch) in _DIRECTION:
                continue
            if unicodedata.category(ch) == "Cc" or _white(ch):
                pending = True
                continue
            n = len(ch.encode("utf-8", "surrogatepass"))
            if pending and has_text and not last_space:
                if size + 1 > 500:
                    cut = True
                    break
                add_text(" ")
                size += 1
            pending = False
            if size + n > 500:
                cut = True
                break
            add_text(ch)
            size += n
            has_text, last_space = True, False
    if not has_text:
        return None
    for i in range(len(out) - 1, -1, -1):
        if not out[i][0]:
            t = out[i][1].rstrip(" ")
            if t:
                out[i] = (False, t)
            else:
                del out[i]
            break
    return [(tg, x if tg else x.replace("&", "&amp;").replace("<", "&lt;").replace(">", "&gt;")) for tg, x in out]


def _balance(lines):
    """Balances the tags over a cue's lines, and writes them."""
    stack, real, out = [], 0, []
    for line in lines:
        b = []
        for is_tag, t in line:
            if not is_tag:
                b.append(t)
                continue
            end = t.startswith("</")
            name = t.strip("</>")
            if name.startswith("c."):
                name = "c"
            if not end:
                if real >= 8:
                    stack.append((name, True))
                    continue
                stack.append((name, False))
                real += 1
                b.append(t)
                continue
            if stack and stack[-1][0] == name:
                _, ghost = stack.pop()
                if not ghost:
                    real -= 1
                    b.append(t)
        out.append("".join(b))
    for name, ghost in reversed(stack):
        if not ghost:
            out[-1] += "</%s>" % name
    return out


def rebuild_vtt(raw, whole=False):
    """Rebuilds a WebVTT part (or a whole track) as video 6.3 says: the
    bytes, or raises Refused."""
    if len(raw) > (16 if whole else 5) * 1024 * 1024:
        raise Refused("over %d MiB" % (16 if whole else 5))
    text = _text(raw).replace("\r\n", "\n").replace("\r", "\n")
    if text.startswith("﻿"):
        text = text[1:]
    if not text.startswith("WEBVTT"):
        raise Refused("doesn't start with WEBVTT")
    out, cues = ["WEBVTT\n"], 0
    for block in text.split("\n\n"):
        lines = block.strip("\n").split("\n")
        at, m = -1, None
        for i, line in enumerate(lines):
            m = _TIMING.fullmatch(_trim(line))
            if m:
                at = i
                break
        if at < 0:
            continue
        body = [x for x in (_clean_cue_line(line) for line in lines[at + 1:]) if x is not None]
        if not body:
            continue
        cues += 1
        if cues > (200000 if whole else 50000):
            raise Refused("over %s cues" % ("200,000" if whole else "50,000"))
        out.append("\n%s --> %s%s\n" % (m.group(1), m.group(2), _vtt_settings(m.group(3))))
        out += [x + "\n" for x in _balance(body)]
    return "".join(out).encode("utf-8")


# ---- ASS (video 6.4) -------------------------------------------------------------

_ASS_NUMBER = re.compile(r"-?[0-9]{1,7}(?:\.[0-9]{1,3})?")
_ASS_HEX_COLOUR = re.compile(r"&H([0-9A-Fa-f]{1,8})&?")
_ASS_TAG_COLOUR = re.compile(r"&H([0-9A-Fa-f]{1,6})&?")
_ASS_TAG_ALPHA = re.compile(r"&H([0-9A-Fa-f]{1,2})&?")
_ASS_TIME = re.compile(r"[0-9]{1,3}:[0-9]{2}:[0-9]{2}\.[0-9]{2}")


def _ass_number(s, lo, hi):
    return bool(_ASS_NUMBER.fullmatch(s)) and lo <= float(s) <= hi


def _ass_plain(s):
    """Text made plain: U+FFFD (invalid UTF-8), control characters and
    commas removed."""
    return "".join(ch for ch in s if ch != "�" and ch != "," and unicodedata.category(ch) != "Cc")


def _ass_colour(s):
    m = _ASS_HEX_COLOUR.fullmatch(s)
    if m:
        v = int(m.group(1), 16)
    elif re.fullmatch(r"[0-9]+", s):
        v = int(s)
        if v > 0xFFFFFFFF:
            return None
    else:
        return None
    return "&H%08X" % v


_ASS_INFO = [
    ("PlayResX", lambda s: _int_in(s, 1, 16384)), ("PlayResY", lambda s: _int_in(s, 1, 16384)),
    ("LayoutResX", lambda s: _int_in(s, 1, 16384)), ("LayoutResY", lambda s: _int_in(s, 1, 16384)),
    ("WrapStyle", lambda s: s in ("0", "1", "2", "3")), ("ScaledBorderAndShadow", lambda s: s in ("yes", "no")),
    ("Collisions", lambda s: s in ("Normal", "Reverse")), ("Kerning", lambda s: s in ("yes", "no")),
    ("YCbCr Matrix", lambda s: s in ("None", "TV.601", "PC.601", "TV.709", "PC.709", "TV.FCC", "PC.FCC", "TV.240M",
                                     "PC.240M")),
]


def _keep(ok):
    return lambda v, ssa: (v if ok(v) else None)


def _plain_field(v, ssa):
    p = _ass_plain(v)
    return p if 1 <= len(p.encode("utf-8")) <= 64 else None


def _flag(v, ssa):
    if not _INT.fullmatch(v):
        return None
    return "0" if v.lstrip("-").strip("0") == "" else "-1"


_SSA_ALIGN = {"1": "1", "2": "2", "3": "3", "5": "7", "6": "8", "7": "9", "9": "4", "10": "5", "11": "6"}


def _alignment(v, ssa):
    if not ssa:
        return v if v in ("1", "2", "3", "4", "5", "6", "7", "8", "9") else None
    return _SSA_ALIGN.get(v)


_STYLE_FIELDS = [
    ("Name", "", _plain_field), ("Fontname", "Arial", _plain_field),
    ("Fontsize", "20", _keep(lambda s: _ass_number(s, 0, 1000))),
    ("PrimaryColour", "&H00FFFFFF", lambda v, ssa: _ass_colour(v)),
    ("SecondaryColour", "&H000000FF", lambda v, ssa: _ass_colour(v)),
    ("OutlineColour", "&H00000000", lambda v, ssa: _ass_colour(v)),
    ("BackColour", "&H00000000", lambda v, ssa: _ass_colour(v)),
    ("Bold", "0", _keep(lambda s: _int_in(s, -1, 1000))),
    ("Italic", "0", _flag), ("Underline", "0", _flag), ("StrikeOut", "0", _flag),
    ("ScaleX", "100", _keep(lambda s: _ass_number(s, 0, 1000))), ("ScaleY", "100", _keep(lambda s: _ass_number(s, 0, 1000))),
    ("Spacing", "0", _keep(lambda s: _ass_number(s, -1000, 1000))), ("Angle", "0", _keep(lambda s: _ass_number(s, -360, 360))),
    ("BorderStyle", "1", _keep(lambda s: s in ("1", "3", "4"))),
    ("Outline", "2", _keep(lambda s: _ass_number(s, 0, 100))), ("Shadow", "2", _keep(lambda s: _ass_number(s, 0, 100))),
    ("Alignment", "2", _alignment),
    ("MarginL", "10", _keep(lambda s: _int_in(s, 0, 16384))), ("MarginR", "10", _keep(lambda s: _int_in(s, 0, 16384))),
    ("MarginV", "10", _keep(lambda s: _int_in(s, 0, 16384))), ("Encoding", "1", _keep(lambda s: _int_in(s, 0, 255))),
]
_V4PLUS = ["Name", "Fontname", "Fontsize", "PrimaryColour", "SecondaryColour", "OutlineColour", "BackColour", "Bold",
           "Italic", "Underline", "StrikeOut", "ScaleX", "ScaleY", "Spacing", "Angle", "BorderStyle", "Outline", "Shadow",
           "Alignment", "MarginL", "MarginR", "MarginV", "Encoding"]
_V4 = ["Name", "Fontname", "Fontsize", "PrimaryColour", "SecondaryColour", "TertiaryColour", "BackColour", "Bold", "Italic",
       "BorderStyle", "Outline", "Shadow", "Alignment", "MarginL", "MarginR", "MarginV", "AlphaLevel", "Encoding"]
_EVENTS = ["Layer", "Start", "End", "Style", "Name", "MarginL", "MarginR", "MarginV", "Effect", "Text"]
_SSA_EVENTS = ["Marked", "Start", "End", "Style", "Name", "MarginL", "MarginR", "MarginV", "Effect", "Text"]


def _ass_format(v):
    return [_trim(f) for f in v.split(",")]


def _ass_style(value, fmt, ssa):
    fields = value.split(",")
    if len(fields) != len(fmt):
        return None
    raw = {}
    for i, name in enumerate(fmt):
        if ssa and name == "TertiaryColour":
            name = "OutlineColour"
        raw.setdefault(name, _trim(fields[i]))
    out, style_name = [], None
    for name, default, check in _STYLE_FIELDS:
        w = check(raw[name], ssa) if name in raw else None
        if w is None:
            if name == "Name":
                return None
            w = default
        if name == "Name":
            style_name = w
        out.append(w)
    return ",".join(out), style_name


def _default_style():
    return ",".join("Default" if n == "Name" else d for n, d, _ in _STYLE_FIELDS)


def rebuild_ass(raw, whole=False):
    """Rebuilds an ASS or SSA part (or a whole track) as video 6.4 says:
    the bytes, or raises Refused."""
    if len(raw) > (16 if whole else 5) * 1024 * 1024:
        raise Refused("over %d MiB" % (16 if whole else 5))
    text = _text(raw).replace("\r\n", "\n").replace("\r", "\n")
    if text.startswith("﻿"):
        text = text[1:]
    lines = []
    for line in text.split("\n"):
        line = _trim(line)
        if line == "" or line.startswith(";") or line.startswith("!:"):
            continue
        lines.append(line)
    if not lines or lines[0] != "[Script Info]":
        raise Refused("the first line isn't [Script Info]")
    known, section, ssa, plus = [], "", False, False
    for line in lines:
        if line.startswith("[") and line.endswith("]"):
            if _fold_eq(line, "[Script Info]"):
                section = "info"
            elif _fold_eq(line, "[V4+ Styles]"):
                section, plus = "v4+", True
            elif _fold_eq(line, "[V4 Styles]"):
                section, ssa = "v4", True
            elif _fold_eq(line, "[Events]"):
                section = "events"
            else:
                section = ""
            if section:
                known.append((section, True, "", ""))
            continue
        if not section:
            continue
        i = line.find(":")
        if i < 0:
            continue
        known.append((section, False, line[:i], _trim(line[i + 1:])))
    ssa_events = ssa and not plus

    info, seen = {}, set()
    for sec, start, name, value in known:
        if sec != "info" or start:
            continue
        for n, ok in _ASS_INFO:
            if _fold_eq(name, n) and n not in seen:
                seen.add(n)
                if ok(value):
                    info[n] = value

    styles, names, fmt = [], set(), []
    for sec, start, name, value in known:
        if sec not in ("v4+", "v4"):
            continue
        is_ssa = sec == "v4"
        if start:
            fmt = _V4 if is_ssa else _V4PLUS
        elif name == "Format":
            fmt = _ass_format(value)
        elif name == "Style":
            s = _ass_style(value, fmt, is_ssa)
            if s is None or s[1] in names or len(styles) >= 1024:
                continue
            names.add(s[1])
            styles.append(s[0])

    events = []
    for sec, start, name, value in known:
        if sec != "events":
            continue
        if start:
            fmt = _SSA_EVENTS if ssa_events else _EVENTS
        elif name == "Format":
            fmt = _ass_format(value)
        elif name == "Dialogue":
            e = _ass_event(value, fmt, names)
            if e is None:
                continue
            if len(events) >= (200000 if whole else 50000):
                raise Refused("over %s events" % ("200,000" if whole else "50,000"))
            events.append(e)

    out = ["[Script Info]\nScriptType: v4.00+\n"]
    out += ["%s: %s\n" % (n, info[n]) for n, _ in _ASS_INFO if n in info]
    out.append("\n[V4+ Styles]\nFormat: " + ", ".join(_V4PLUS) + "\n")
    if "Default" not in names:
        out.append("Style: " + _default_style() + "\n")
    out += ["Style: " + s + "\n" for s in styles]
    out.append("\n[Events]\nFormat: " + ", ".join(_EVENTS) + "\n")
    out += ["Dialogue: " + e + "\n" for e in events]
    return "".join(out).encode("utf-8")


def _ass_event(value, fmt, styles):
    if not fmt:
        return None
    parts = value.split(",", len(fmt) - 1)
    if len(parts) != len(fmt):
        return None
    raw = {}
    for i, name in enumerate(fmt[:-1]):
        raw.setdefault(name, _trim(parts[i]))
    text = parts[-1]
    layer = raw["Layer"] if "Layer" in raw and _int_in(raw["Layer"], 0, 1000) else "0"
    start, end = raw.get("Start", ""), raw.get("End", "")
    if not _ASS_TIME.fullmatch(start) or not _ASS_TIME.fullmatch(end):
        return None
    style = raw["Style"] if raw.get("Style") in styles else "Default"

    def margin(n):
        return raw[n] if n in raw and _int_in(raw[n], 0, 16384) else "0"
    t = _ass_text(text, styles)
    if t == "" or len(t.encode("utf-8")) > 65536:
        return None
    return ",".join([layer, start, end, style, "", margin("MarginL"), margin("MarginR"), margin("MarginV"), "", t])


def _ass_text(s, styles):
    s = "".join(" " if ch == "\t" else ch for ch in s
                if ch == "\t" or not (ch == "�" or ord(ch) in _DIRECTION or unicodedata.category(ch) == "Cc"))
    out, drawing, stopped = [], False, False
    blocks = tags = 0
    transforms = [0]
    while True:
        i = s.find("{")
        piece = s if i < 0 else s[:i]
        out.append(_ass_drawing(piece) if drawing else piece)
        if i < 0:
            break
        j = s.find("}", i + 1)
        if j < 0:
            break
        inner = s[i + 1:j]
        s = s[j + 1:]
        if stopped:
            continue
        t = [transforms[0]]
        kept, p = _ass_block(inner, styles, t)
        if not kept:
            continue
        if blocks + 1 > 256 or tags + len(kept) > 1024:
            stopped = True
            continue
        blocks += 1
        tags += len(kept)
        transforms = t
        out.append("{" + "".join(kept) + "}")
        if p >= 0:
            drawing = p > 0
    return "".join(out)


def _ass_cut(s):
    """The tags of a block (from its first \\), cut at each \\ but not inside
    a \\t's parentheses."""
    out, i = [], 0
    while i < len(s):
        j = i + 1
        is_t = s.startswith("t(", j)
        depth = 0
        while j < len(s):
            c = s[j]
            if c == "\\" and depth == 0:
                break
            if is_t and c == "(":
                depth += 1
            elif is_t and c == ")" and depth > 0:
                depth -= 1
            j += 1
        out.append(s[i + 1:j])
        i = j
    return out


def _ass_block(inner, styles, transforms):
    k = inner.find("\\")
    if k < 0:
        return [], -1
    kept, p = [], -1
    for raw in _ass_cut(inner[k:]):
        r = _ass_tag(raw, styles, False, transforms)
        if r is None:
            continue
        t, pv = r
        if pv >= 0:
            p = pv
        kept.append(t)
    return kept, p


_ASS_TAG_NAMES = ["b", "i", "u", "s", "bord", "xbord", "ybord", "be", "blur", "shad", "xshad", "yshad",
                  "fn", "fs", "fscx", "fscy", "fsp", "frx", "fry", "frz", "fr", "fax", "fay", "fe",
                  "c", "1c", "2c", "3c", "4c", "alpha", "1a", "2a", "3a", "4a", "an", "a", "k", "K", "kf", "ko", "q", "r",
                  "pos", "org", "move", "fad", "fade", "t", "clip", "iclip", "p", "pbo"]
_NOT_IN_T = {"t", "pos", "org", "move", "fad", "fade", "an", "a", "k", "K", "kf", "ko", "q", "r", "fn", "p", "pbo"}


def _parens(args):
    if not (args.startswith("(") and args.endswith(")")) or len(args) < 2:
        return None
    return [_trim(x) for x in args[1:-1].split(",")]


def _coord(s):
    return _ass_number(s, -100000, 100000)


def _millis(s):
    return _int_in(s, 0, 3600000)


def _ass_tag(raw, styles, in_t, transforms):
    """One tag (without its \\): (the written tag, the \\p value or -1), or None."""
    name = ""
    for n in _ASS_TAG_NAMES:
        if raw.startswith(n) and len(n) > len(name):
            name = n
    if not name or (in_t and name in _NOT_IN_T):
        return None
    args = raw[len(name):]

    def keep(ok):
        return ("\\" + raw, -1) if ok else None

    def num(lo, hi):
        return keep(_ass_number(args, lo, hi))
    if name == "b":
        return keep(_int_in(args, 0, 1000))
    if name in ("i", "u", "s"):
        return keep(args in ("0", "1"))
    if name in ("bord", "xbord", "ybord", "be", "blur"):
        return num(0, 100)
    if name in ("shad", "xshad", "yshad", "fax", "fay"):
        return num(-100, 100)
    if name == "fn":
        f = _ass_plain(args)
        return ("\\fn" + f, -1) if len(f.encode("utf-8")) <= 64 else None
    if name in ("fs", "fscx", "fscy"):
        return num(0, 1000)
    if name == "fsp":
        return num(-1000, 1000)
    if name in ("frx", "fry", "frz", "fr"):
        return num(-3600, 3600)
    if name == "fe":
        return keep(_int_in(args, 0, 255))
    if name in ("c", "1c", "2c", "3c", "4c"):
        m = _ASS_TAG_COLOUR.fullmatch(args)
        return ("\\%s&H%06X&" % (name, int(m.group(1), 16)), -1) if m else None
    if name in ("alpha", "1a", "2a", "3a", "4a"):
        m = _ASS_TAG_ALPHA.fullmatch(args)
        return ("\\%s&H%02X&" % (name, int(m.group(1), 16)), -1) if m else None
    if name == "an":
        return keep(args in ("1", "2", "3", "4", "5", "6", "7", "8", "9"))
    if name == "a":
        a = _alignment(args, True)
        return ("\\an" + a, -1) if a else None
    if name in ("k", "K", "kf", "ko"):
        return keep(_int_in(args, 0, 100000))
    if name == "q":
        return keep(args in ("0", "1", "2", "3"))
    if name == "r":
        return keep(args == "" or args in styles)
    if name == "p":
        return ("\\p" + args, int(args)) if _int_in(args, 0, 16) else None
    if name == "pbo":
        return num(-100000, 100000)
    if name in ("pos", "org"):
        a = _parens(args)
        if a is None or len(a) != 2 or not all(_coord(x) for x in a):
            return None
        return "\\%s(%s)" % (name, ",".join(a)), -1
    if name == "move":
        a = _parens(args)
        if a is None or len(a) not in (4, 6) or not all(_coord(x) for x in a[:4]) or not all(_millis(x) for x in a[4:]):
            return None
        return "\\move(%s)" % ",".join(a), -1
    if name == "fad":
        a = _parens(args)
        if a is None or len(a) != 2 or not all(_millis(x) for x in a):
            return None
        return "\\fad(%s)" % ",".join(a), -1
    if name == "fade":
        a = _parens(args)
        if a is None or len(a) != 7 or not all(_int_in(x, 0, 255) for x in a[:3]) or not all(_millis(x) for x in a[3:]):
            return None
        return "\\fade(%s)" % ",".join(a), -1
    if name in ("clip", "iclip"):
        a = _parens(args)
        if a is None:
            return None
        if len(a) == 4:
            return ("\\%s(%s)" % (name, ",".join(a)), -1) if all(_coord(x) for x in a) else None
        if len(a) == 2:
            if not _int_in(a[0], 1, 16):
                return None
            d = _ass_drawing(a[1])
            return ("\\%s(%s,%s)" % (name, a[0], d), -1) if d else None
        if len(a) == 1:
            d = _ass_drawing(a[0])
            return ("\\%s(%s)" % (name, d), -1) if d else None
        return None
    if name == "t":
        return _ass_transform(args, styles, transforms)
    return None


def _ass_transform(args, styles, transforms):
    if transforms[0] >= 16 or not (args.startswith("(") and args.endswith(")")) or len(args) < 2:
        return None
    body = args[1:-1]
    k = body.find("\\")
    if k < 0:
        return None
    nums = [_trim(x) for x in body[:k].split(",")]
    if nums[-1] != "":
        return None
    nums = nums[:-1]
    if len(nums) == 1:
        if not _ass_number(nums[0], 0, 100):
            return None
    elif len(nums) == 2:
        if not all(_millis(x) for x in nums):
            return None
    elif len(nums) == 3:
        if not all(_millis(x) for x in nums[:2]) or not _ass_number(nums[2], 0, 100):
            return None
    elif len(nums) > 3:
        return None
    kept = []
    for raw in _ass_cut(_trim(body[k:])):
        r = _ass_tag(raw, styles, True, transforms)
        if r is not None:
            kept.append(r[0])
    if not kept:
        return None
    transforms[0] += 1
    pre = ",".join(nums)
    if pre:
        pre += ","
    return "\\t(%s%s)" % (pre, "".join(kept)), -1


def _ass_drawing(s):
    out, numbers = [], 0
    for f in re.split("[%s]+" % re.escape(_WS), s):
        if not f:
            continue
        if f in ("m", "n", "l", "b", "s", "p", "c"):
            out.append(f)
        elif _coord(f):
            numbers += 1
            if numbers > 10000:
                return " ".join(out)
            out.append(f)
    return " ".join(out)


# ---- Fonts (video 6.6) -------------------------------------------------------------

def check_font(b):
    """Checks a font as video 6.6 says: its media type, "font/ttf" or
    "font/otf"; raises Refused."""
    if len(b) > 10 * 1024 * 1024:
        raise Refused("step 1: over 10 MiB")
    if len(b) < 4:
        raise Refused("step 2: under 4 bytes")
    if b[:4] in (b"\x00\x01\x00\x00", b"true"):
        typ = "font/ttf"
    elif b[:4] == b"OTTO":
        typ = "font/otf"
    else:
        raise Refused("step 2: first four bytes neither 00 01 00 00, true nor OTTO")
    if len(b) < 6:
        raise Refused("step 3: no table count")
    n = int.from_bytes(b[4:6], "big")
    if not 1 <= n <= 64:
        raise Refused("step 3: a table count outside 1 to 64")
    if 12 + 16 * n > len(b):
        raise Refused("step 3: the table directory past the end")
    tags = set()
    for i in range(n):
        r = b[12 + 16 * i:12 + 16 * i + 16]
        tag = bytes(r[:4])
        if any(c < 0x20 or c > 0x7E for c in tag):
            raise Refused("step 4: a tag byte outside 0x20 to 0x7E")
        if tag in tags:
            raise Refused("step 4: a tag twice")
        tags.add(tag)
        if int.from_bytes(r[8:12], "big") + int.from_bytes(r[12:16], "big") > len(b):
            raise Refused("step 4: a table past the end")
    for t in (b"cmap", b"head", b"hhea", b"hmtx", b"maxp", b"name", b"post"):
        if t not in tags:
            raise Refused("step 5: no %s table" % t.decode())
    if not ({b"glyf", b"loca"} <= tags) and b"CFF " not in tags and b"CFF2" not in tags:
        raise Refused("step 5: neither glyf and loca, nor CFF or CFF2")
    return typ


# ---- A local title's image (catalog 4.4) ---------------------------------------------

_PNG = b"\x89PNG\r\n\x1a\n"


def _jpeg_frame(b):
    i = 2
    while True:
        if i >= len(b) or b[i] != 0xFF:
            raise Refused("no marker where one is due")
        while i < len(b) and b[i] == 0xFF:
            i += 1
        if i >= len(b):
            raise Refused("no frame header in its header")
        m = b[i]
        i += 1
        if m == 0x01 or 0xD0 <= m <= 0xD7:
            continue
        if m in (0xD9, 0xDA):
            raise Refused("no frame header before the image data")
        if i + 2 > len(b):
            raise Refused("no frame header in its header")
        n = b[i] << 8 | b[i + 1]
        if n < 2:
            raise Refused("a segment length under 2")
        if 0xC0 <= m <= 0xCF and m not in (0xC4, 0xC8, 0xCC):
            if n < 8 or i + 7 > len(b):
                raise Refused("a frame header cut short")
            return b[i + 5] << 8 | b[i + 6], b[i + 3] << 8 | b[i + 4]
        i += n


def image_kind(b, size=None):
    """The checks of a local title's image before decoding it (catalog
    4.4, media 3.4): ("jpeg" or "png", width, height); raises Refused. b
    is the image (or its first bytes, with size its whole length)."""
    if size is None:
        size = len(b)
    if len(b) > size:
        raise Refused("more bytes than its size")
    if size > 1024 * 1024:
        raise Refused("over 1 MiB (1,048,576 bytes)")
    if b.startswith(_PNG):
        ih = b[8:]
        if len(ih) < 16 or ih[:8] != b"\x00\x00\x00\x0dIHDR":
            raise Refused("a PNG whose first chunk isn't IHDR")
        fmt, w, h = "png", int.from_bytes(ih[8:12], "big"), int.from_bytes(ih[12:16], "big")
    elif b.startswith(b"\xff\xd8\xff"):
        fmt = "jpeg"
        w, h = _jpeg_frame(b)
    else:
        raise Refused("neither a JPEG nor a PNG by its content")
    if w < 1 or h < 1:
        raise Refused("a width or a height of 0")
    if w > 4096 or h > 4096:
        raise Refused("wider or taller than 4,096 pixels")
    return fmt, w, h


def image_answer(root, body):
    """The viewer's server's check of an image answer (catalog 4.4): the
    ticket of its url, exactly <stream root><ticket>/image; raises
    Refused (the answer dropped). body is the parsed answer."""
    if not isinstance(body, dict):
        raise Refused("not a JSON object")
    u = body.get("url")
    if not isinstance(u, str):
        raise Refused("url absent or not a string")
    if not u.startswith(root):
        raise Refused("url not under the owning server's stream root")
    rest = u[len(root):]
    if not rest.endswith("/image"):
        raise Refused("url not ending in /image")
    t = rest[:-len("/image")]
    if not 1 <= len(t) <= 64:
        raise Refused("a ticket not 1 to 64 characters")
    for c in t:
        if not re.fullmatch(r"[A-Za-z0-9_-]", c):
            raise Refused("a ticket with byte %r outside A-Z a-z 0-9 _ -" % c)
    return t


# ---- PGS (video 6.5) ------------------------------------------------------------------

def check_pgs(raw):
    """Checks a PGS part whole; raises Refused."""
    if len(raw) > 64 * 1024 * 1024:
        raise Refused("over 64 MiB")

    def side(v):
        return 1 <= v <= 4096

    opened, firsts, pos = {}, 0, 0
    while pos < len(raw):
        if len(raw) - pos < 13:
            raise Refused("a segment header cut short")
        if raw[pos:pos + 2] != b"PG":
            raise Refused("a segment without its PG magic")
        typ, size = raw[pos + 10], int.from_bytes(raw[pos + 11:pos + 13], "big")
        if pos + 13 + size > len(raw):
            raise Refused("a payload past the end")
        seg = raw[pos + 13:pos + 13 + size]
        pos += 13 + size
        if typ == 0x16:
            if len(seg) < 11:
                raise Refused("a presentation under 11 bytes")
            if not side(int.from_bytes(seg[0:2], "big")) or not side(int.from_bytes(seg[2:4], "big")):
                raise Refused("a screen size outside 1 to 4,096")
            o = 11
            for _ in range(seg[10]):
                if o + 8 > len(seg):
                    raise Refused("composition objects past the payload")
                o += 16 if seg[o + 3] & 0x80 else 8
            if o > len(seg):
                raise Refused("composition objects past the payload")
        elif typ == 0x17:
            if len(seg) < 1 or len(seg) != 1 + 9 * seg[0]:
                raise Refused("a window segment of the wrong size")
        elif typ == 0x14:
            if len(seg) < 2 or (len(seg) - 2) % 5:
                raise Refused("a palette segment of the wrong size")
        elif typ == 0x15:
            if len(seg) < 4:
                raise Refused("an object segment under 4 bytes")
            oid, flags = int.from_bytes(seg[0:2], "big"), seg[3]
            if flags & 0x80:
                if len(seg) < 11:
                    raise Refused("a first fragment under 11 bytes")
                w, h = int.from_bytes(seg[7:9], "big"), int.from_bytes(seg[9:11], "big")
                if not side(w) or not side(h):
                    raise Refused("an object size outside 1 to 4,096")
                firsts += 1
                if firsts > 20000:
                    raise Refused("over 20,000 objects")
                opened[oid] = [w, h, bytearray(seg[11:])]
            else:
                if oid not in opened:
                    raise Refused("a fragment of no open object")
                opened[oid][2] += seg[4:]
            if flags & 0x40:
                w, h, data = opened.pop(oid)
                _runs_fit(bytes(data), w, h)
        elif typ == 0x80:
            if seg:
                raise Refused("an end segment that isn't empty")
        else:
            raise Refused("an unknown segment type 0x%02x" % typ)


def _runs_fit(d, w, h):
    x = y = i = 0
    while i < len(d):
        b = d[i]
        i += 1
        if b:
            x += 1
        else:
            if i >= len(d):
                raise Refused("a run cut short")
            f = d[i]
            i += 1
            if f == 0:
                x, y = 0, y + 1
                if y > h:
                    raise Refused("more lines than the object's height")
                continue
            n = f & 0x3F
            if f & 0x40:
                if i >= len(d):
                    raise Refused("a run cut short")
                n = n << 8 | d[i]
                i += 1
            if f & 0x80:
                if i >= len(d):
                    raise Refused("a run cut short")
                i += 1
            x += n
        if x > w:
            raise Refused("a line wider than the object")


# ---- JPEG (video 6.2) -------------------------------------------------------------------

def jpeg_size(b):
    """(width, height) from a JPEG's header (its first SOF marker), or None
    when it isn't a JPEG."""
    if b[:3] != b"\xff\xd8\xff":
        return None
    i = 2
    while i + 4 <= len(b):
        if b[i] != 0xFF:
            return None
        m = b[i + 1]
        if m == 0xFF:
            i += 1
            continue
        if m in (0xD8, 0x01) or 0xD0 <= m <= 0xD7:
            i += 2
            continue
        n = int.from_bytes(b[i + 2:i + 4], "big")
        if m in (0xC0, 0xC1, 0xC2, 0xC3, 0xC5, 0xC6, 0xC7, 0xC9, 0xCA, 0xCB, 0xCD, 0xCE, 0xCF):
            if i + 9 > len(b):
                return None
            return int.from_bytes(b[i + 7:i + 9], "big"), int.from_bytes(b[i + 5:i + 7], "big")
        if m == 0xDA:
            return None
        i += 2 + n
    return None
