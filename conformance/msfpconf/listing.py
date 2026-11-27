"""The titles request as an owning server reads it (MSFP Media listing 3,
3.3 and 3.6, as MSFP's catalog carries it): the libraries it lists
(libraries optional: absent, every library the caller may see), the
filters kind and under, and its refusals. The fake answers titles with
it, the live checks T13 to T15 derive from it what a server must list,
and the self-test checks it against vectors/v0/strict.json
(titleRequests)."""

import re

from .copies import token, video_key
from .music import music_key

KINDS = {"video": ("movie", "series", "season", "episode", "extra"), "music": ("artist", "album", "track")}


class Malformed(ValueError):
    """A titles request refused with 404 not-found (MSFP Media listing 3.3)."""


def kind_of(key):
    """(kind, content part) of a title key, or (None, None)."""
    vk = video_key(key)
    if vk:
        return vk.kind, "video"
    mk = music_key(key)
    if mk:
        return mk, "music"
    return None, None


def under(u, t):
    """Whether title t (a dict with key, and season, album, artists as
    under goes by) is directly under the key u (MSFP Media listing 3.6's
    table). u is the exact string, never checked against the key forms."""
    key = t["key"]
    uk, upart = kind_of(u)
    kind, _ = kind_of(key)
    if upart == "video":
        vk = video_key(key)
        if kind == "extra":
            return vk.parent == u
        rest = key[len(u) + 1:] if key.startswith(u + "/") else None
        if uk == "series":
            return rest is not None and "/" not in rest and (kind == "season" or kind == "episode" and rest.startswith("a"))
        if uk == "season" and kind == "episode":
            if rest is not None and "/" not in rest:
                return True
            m = re.fullmatch(re.escape(vk.series) + r"/s(0|[1-9][0-9]*)", u)
            return bool(vk.by_id and m and t.get("season") == int(m.group(1)))
        return False
    if uk == "album":
        return kind == "track" and t.get("album") == u
    if uk == "artist":
        return kind == "album" and u in (t.get("artists") or [])
    return False


def list_titles(titles, visible, parts, j):
    """What the owning server lists for a titles request j (parsed JSON):
    (titles as (key, library) in byte order of key, may_be_changes).
    titles are dicts with key, libraries, and season, album, artists;
    visible the libraries the caller may see; parts the content parts in
    use. Raises Malformed for a request refused with 404."""
    if not isinstance(j, dict):
        raise Malformed("not a JSON object")
    requested = list(visible)
    if "libraries" in j:
        libs = j["libraries"]
        if not isinstance(libs, list) or any(not isinstance(x, str) for x in libs):
            raise Malformed("libraries isn't an array of strings")
        requested = [x for x in libs if x in visible]
    for f in ("kind", "under", "after"):
        if f in j and not isinstance(j[f], str):
            raise Malformed("%s isn't a string" % f)
    kind, u, after = j.get("kind"), j.get("under"), j.get("after", "")
    if kind is not None and token(kind) is None:
        raise Malformed("kind isn't a token")
    for f, v in (("under", u), ("after", after)):
        if v is not None and len(v.encode()) > 512:
            raise Malformed("%s over 512 bytes" % f)
    from .copies import int_in
    since = int_in(j.get("since", 0), -2**53, 2**53)
    if since is None:
        raise Malformed("since isn't an integer")
    out = []
    for t in titles:
        k, part = kind_of(t["key"])
        if part not in parts or t["key"].encode() <= after.encode():
            continue
        libs = sorted((x for x in t["libraries"] if x in requested), key=str.encode)
        if not libs or (kind is not None and k != kind) or (u is not None and not under(u, t)):
            continue
        out.append((t["key"], libs[0]))
    out.sort(key=lambda x: x[0].encode())
    return out, since > 0 and u is None


def vectors_ok(path):
    """list_titles against strict.json's titleRequests: a list of mismatches."""
    import json
    from .copies import dumps, loads
    from .wire import NotIJSON, parse_ijson
    v = json.load(open(path))
    offer = v["titleOffer"]
    bad = []
    for c in v["titleRequests"]:
        try:
            parse_ijson(c["body"].encode())
            listed, changes = list_titles(offer["titles"], offer["visible"], c["parts"], loads(c["body"]))
            got = dumps({"titles": [{"key": k, "library": lib} for k, lib in listed], "mayBeChanges": changes})
        except (Malformed, NotIJSON, ValueError):
            got = None
        if got != (c.get("output") if c["valid"] else None):
            bad.append("titleRequests %s: %s, not %s" % (c["name"], got, c.get("output", "404")))
    return bad
