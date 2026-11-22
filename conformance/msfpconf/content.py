"""Libraries and titles as an owning server sends them:
catalog.md sections 2 to 4, video.md sections 3 and 4, music.md sections
2 and 3 (the readers are in copies.py and music.py)."""

import re

from .copies import MAX_COPIES, copy_problems, dropped_members, keep_video_title, token, video_key
from .music import music_key, music_problems
from .wire import is_int

LIBRARY_ID = re.compile(r"[A-Za-z0-9_-]{1,64}")

# ---- a title as the owning server sends it --------------------------------------------

SUMMARY = ("audio", "subtitles", "video")   # the title summary video 0 removed (video 3.2)


def _height(c):
    """Video 3.7's height of a copy: that of a 16:9 picture as wide, for a
    wider one; -1 without video."""
    v = c.get("video") if isinstance(c, dict) else None
    if not isinstance(v, dict) or not is_int(v.get("width")) or not is_int(v.get("height")) or v["height"] < 1:
        return -1
    return max(v["height"], -(-v["width"] * 9 // 16)) if v["width"] * 9 > v["height"] * 16 else v["height"]


def kind_of(t):
    """The kind a title's key names (catalog 3: a title carries no kind),
    or None."""
    k = t.get("key") if isinstance(t, dict) else t
    vk = video_key(k)
    return vk.kind if vk else music_key(k)


def copies_order_problems(key, cs):
    """Video 3.7: the copies come in the owner's order of preference; best
    first by height, then PQ or HLG (H.273 transfer 16 or 18), is the
    notes' advice (proposal 17), not a rule. A note, never a failure."""
    def rank(c):
        v = c.get("video") if isinstance(c.get("video"), dict) else {}
        col = v.get("colour") if isinstance(v.get("colour"), dict) else {}
        return (_height(c), 1 if col.get("transfer") in (16, 18) else 0)
    rs = [rank(c) for c in cs if isinstance(c, dict)]
    for i in range(1, len(rs)):
        if rs[i] > rs[i - 1]:
            return ["%s: copies not best first by height and range (the notes' advice for video 3.7): copy %d "
                    "(height %d) after a lesser one" % (key, i, rs[i][0])]
    return []


def title_notes(t):
    """Members a title still sends that proposal 17 dropped: unknown to a
    caller, which ignores them (notes, not failures)."""
    if not isinstance(t, dict):
        return []
    k = t.get("key")
    out = ["%s carries kind, dropped by proposal 17 (catalog 3: the key names it)" % k] if "kind" in t else []
    kind = kind_of(t)
    if kind == "extra" and "name" in t:
        out.append("%s: an extra carries name, dropped by proposal 17 (video 3.9)" % k)
    if kind == "album" and "sortName" in t:
        out.append("%s carries sortName, dropped by proposal 17 (music 3)" % k)
    for c in t.get("copies") if isinstance(t.get("copies"), list) else []:
        if isinstance(c, dict):
            out += dropped_members(c, "%s copy %s" % (k, c.get("id")))
    return out


def video_problems(t, libs):
    """What an owning server got wrong in a video title it sent (video 3,
    3.1 to 3.10, 4): what a caller would skip, drop, clean or change."""
    k = t["key"]
    vk = video_key(k)
    kind = vk.kind
    try:
        kept = keep_video_title(t, libs)
    except ValueError as e:
        return ["%s: a caller skips it: %s" % (k, e)]
    bad = []
    for f in SUMMARY:
        if f in t:
            bad.append("%s carries %s, a title summary video 0 removed (video 3.2: its copies say it)" % (k, f))
    if kind in ("movie", "episode", "extra"):
        cs = t.get("copies")
        if len(cs) > MAX_COPIES:
            bad.append("%s: %d copies; at most 16 (video 3.2)" % (k, len(cs)))
        ids = [c.get("id") for c in cs if isinstance(c, dict)]
        if len(set(map(str, ids))) != len(ids):
            bad.append("%s: a copy ID twice" % k)
        for c in cs:
            bad += ["%s %s" % (k, x) for x in copy_problems(c)]
    elif "copies" in t:
        bad.append("%s (%s) carries copies (video 3: films, episodes and extras only)" % (k, kind))
    if kind == "extra":
        if token(t.get("extraType")) is None:
            bad.append("%s: extraType %r isn't a token (video 3.9, core 8.5)" % (k, t.get("extraType")))
    elif vk.scheme == "local" and kind in ("movie", "series"):
        if kept.get("name") != t.get("name"):
            bad.append("%s: name isn't plain text of at most 128 characters (video 3.10)" % k)
        if "year" in t and kept.get("year") != t["year"]:
            bad.append("%s: year %r isn't an integer from 1800 to 9999" % (k, t["year"]))
        if "image" in t and not isinstance(t["image"], bool):
            bad.append("%s: image isn't a boolean" % k)
    else:
        for f in ("extraType", "year", "image") + (("name",) if kind != "extra" else ()):
            if f in t:
                bad.append("%s (%s%s) carries %s (video 3: extras and local films and series only)"
                           % (k, kind, ", " + vk.scheme if vk.scheme != "tmdb" else "", f))
    if "with" in t:
        if kind != "episode":
            bad.append("%s (%s) carries with (video 3: episodes only)" % (k, kind))
        elif not isinstance(t["with"], list) or kept.get("with", []) != t["with"]:
            bad.append("%s: with isn't at most 16 other episodes of the same series, each once" % k)
        elif t["with"] != sorted(t["with"], key=str.encode):
            bad.append("%s: with isn't in byte order (video 4.2)" % k)
    if not vk.by_id and "place" in t:
        bad.append("%s (%s) carries place (video 3: episodes keyed by their TMDB ID only)" % (k, kind))
    if "ids" in t:
        if not (kind == "episode" and vk.scheme != "local") and not (kind == "series" and vk.scheme == "tvdb"):
            bad.append("%s: a %s of %s carries ids (video 4.3: only episodes and titles keyed under tvdb; TMDB's "
                       "lookup by key gives a tmdb title's)" % (k, kind, vk.scheme))
        elif not isinstance(t["ids"], dict):
            bad.append("%s: ids isn't an object" % k)
        else:
            for s, v in t["ids"].items():
                if (s in ("imdb", "tvdb") or s == "tmdb" and vk.scheme == "tvdb") and kept.get("ids", {}).get(s) != v:
                    bad.append("%s: ids.%s %r isn't in its form (video 4.3)" % (k, s, v))
    return bad


def title_problems(t, libs, parts_on):
    """What's wrong with one entry of a titles answer, as the owning server
    sent it (catalog 3, 3.3; video 3, 4; music 2, 3). Returns (kind, problems)."""
    if not isinstance(t, dict):
        return None, ["an entry that isn't an object"]
    k, lib = t.get("key"), t.get("library")
    if not all(isinstance(x, str) for x in (k, lib)):
        return None, ["%r: key or library isn't a string" % (k,)]
    kind = kind_of(t)
    vk, mk = video_key(k), music_key(k)
    bad = []
    if vk:
        if "video" not in parts_on:
            bad.append("%s is a video title, and video is off between the two servers (catalog 5)" % k)
        bad += video_problems(t, libs)
    elif mk:
        if "music" not in parts_on:
            bad.append("%s is a music title, and music is off between the two servers (catalog 5)" % k)
        bad += music_problems(t, libs)
    else:
        bad.append("%r isn't a title key of a content part in canonical form (catalog 4, video 4, music 2)" % k)
    return kind, bad
