"""MSFP as MSFP Media's carrier: what MSFP decides of MSFP Media as it
carries it between servers (playback 2, 5, 6, 8; media 2, 4.2), and the
carrier MSFP Media's checks (msfpmedia.checks) run through, built on the
suite's signed calls (msfpmedia/carrier.py is the contract).

MSFP's own rules here: the limits of a libraries answer (playback 5), the
purpose scan, its ticket and scanBytes (playback 8, media 4.2), the
refusal of a capability list (playback 2) and of a fetch by a browser
(media 2), the refusal over a pairing's limits, 409 stream-limit-reached
(playback 6). They are checked against vectors/v0/playback.json and
media.json, MSFP's carrier vectors."""

import json
import os
import time

from msfpmedia.base import dumps, is_int
from msfpmedia.carrier import Carrier
from msfpmedia.copies import dumps as text_of, int_in, loads
from msfpmedia.media import Refused, check_range, serves
from msfpmedia.playback import playback_vectors_ok

from .wire import FED, STREAM

VECTORS = os.path.join(os.path.dirname(os.path.abspath(__file__)), "..", "..", "vectors", "v0")


# ---- scans (playback 8, media 4.2) ---------------------------------------------------------

SCAN_FLOOR = 4 * 1024 * 1024        # scanBytes absent or out of range (playback 5)
SCAN_TOP = 1024 * 1024 * 1024
SCAN_DEFAULT = 32 * 1024 * 1024     # the recommended default for owners
READ_AT_MOST = 64 * 1024 * 1024     # PB22 reads a larger cap only this far


def scan_cap(scan_bytes):
    """The bytes one scan ticket may read in all, for the scanBytes an owner
    sends in limits (playback 5): it, within 4 MiB to 1 GiB, else the floor."""
    if isinstance(scan_bytes, int) and not isinstance(scan_bytes, bool) and SCAN_FLOOR <= scan_bytes <= SCAN_TOP:
        return scan_bytes
    return SCAN_FLOOR


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


def keep_scan(o, out):
    """What a viewer's server keeps of a scan's answer beside play's
    (playback 8): scanBytes, within 4 MiB to 1 GiB, else 4 MiB."""
    n = int_in(o.get("scanBytes"), -2**53 + 1, 2**53 - 1)
    out["scanBytes"] = n if n is not None and SCAN_FLOOR <= n <= SCAN_TOP else SCAN_FLOOR


# MSFP's own purpose (playback 8) and kind of ticket (media 4.2).
PURPOSES = {"scan": {"deliveries": ("file",), "keep": keep_scan}}
TICKETS = {"scan": ("file",)}


# ---- limits (playback 5, music 5.1) ------------------------------------------------

MAX_STREAMS = 1000
LIMITS = (("streams", 0, MAX_STREAMS), ("transcodes", 0, MAX_STREAMS), ("bitrateKbps", 0, 1000000),
          ("downloads", 0, MAX_STREAMS), ("downloadsPerDay", 0, 10000), ("downloadKbps", 0, 1000000),
          ("scansPerDay", 0, 1000000), ("audioStreams", 0, MAX_STREAMS), ("audioTranscodes", 0, MAX_STREAMS))
_ALWAYS = ("downloads", "downloadsPerDay", "downloadKbps", "scansPerDay")
NO_LIMITS = {"streams": None, "downloads": 0, "downloadsPerDay": 0, "downloadKbps": 0, "scansPerDay": 0}


def _int(v):
    """An MSFP integer (core 2.3) of any size within 2^53, or None."""
    return int_in(v, -2**53 + 1, 2**53 - 1)


def keep_limits(o):
    """playback 5: what a caller keeps of limits (downloads, its daily cap
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


# ---- the carrier MSFP Media's checks run through --------------------------------------------

class MsfpCarrier(Carrier):
    """MSFP Media's checks through the suite's signed calls: the suite
    (checks.Suite) is a paired peer of the server; every call is signed,
    every answer must be signed by the server (core 6.6)."""

    limit_refusal = (409, "stream-limit-reached")      # playback 6
    refused_headers = (("an Origin header", "Origin", "https://example.org"),      # media 2: a browser's
                       ("Sec-Fetch-Site", "Sec-Fetch-Site", "same-origin"),
                       ("Sec-Fetch-Mode", "Sec-Fetch-Mode", "cors"))
    purposes = PURPOSES

    def __init__(self, suite):
        self.s = suite
        self._plays = []

    @property
    def root(self):
        """The server's stream root (MSFP Media fetching 1)."""
        c = self.s.client
        return self.s.target + c.base[:-len(FED)] + STREAM

    def call(self, name, body):
        return self.s.client.call(name, body)

    def play(self, body):
        """A play, paced below the suggested play limit (core 8.3: 30 a
        minute, bursts of 20), and waiting out a signed retryAfter once."""
        now = time.time()
        self._plays = [t for t in self._plays if t > now - 60]
        if len(self._plays) >= 15:
            time.sleep(2.2)
        self._plays.append(time.time())
        a = self.s.client.call("play", body)
        if a.status == 429 and a.signed:
            ra = a.json.get("retryAfter") if isinstance(a.json, dict) else None
            wait = ra if is_int(ra) and 0 < ra <= 120 else 10
            self.s.r.note("play answered 429 rate-limited: waiting %d s and trying again once" % wait)
            time.sleep(wait)
            a = self.s.client.call("play", body)
        return a

    def get(self, url, headers=(), method="GET", limit=80 * 1024 * 1024):
        return self.s.client.get(url, headers, method, limit)

    def get_timed(self, url, headers=(), seconds=12.0, limit=80 * 1024 * 1024):
        return self.s.client.get_timed(url, headers, seconds, limit)

    def expect(self, a, status, typ=None):
        return self.s.signed(a, status, typ)

    def seen(self, a):
        self.s.answers.append(a)

    def carried(self, a):
        return a.signed

    def limits(self):
        lim = self.s.limits
        return keep_limits(lim) if isinstance(lim, dict) else None

    def answer_problems(self, j, purpose):
        if purpose != "scan":
            return []
        sb = j.get("scanBytes")
        if not (is_int(sb) and 4194304 <= sb <= 1073741824):
            return ["scanBytes %s: a scan's answer must state it, 4,194,304 to 1,073,741,824 (MSFP Media playing 2.3; "
                    "playback 8; a caller counts on 4 MiB)" % (dumps(sb).decode() if "scanBytes" in j else "missing")]
        return []

    def play_cases(self):
        v = json.load(open(os.path.join(VECTORS, "playback.json")))
        return [c for g in ("requests", "deliveries", "scans") for c in v[g]]

    def checks(self, point, **context):
        s = self.s
        if point == "libraries":
            s.limits_checks()
        elif point == "titles":
            s.local_switch(context["libraries"])
        elif point == "limits":
            s.race_checks(context["title"])
        elif point == "playing":
            s.scan_checks(context["title"])


# ---- MSFP's carrier vectors ----------------------------------------------------------------

def carrier_vectors_ok(vec=VECTORS):
    """MSFP's rules against vectors/v0/playback.json (a scan's answer, the
    limits) and media.json (the scan ticket's names and reads): a list of
    mismatches."""
    bad = playback_vectors_ok(os.path.join(vec, "playback.json"), PURPOSES)
    v = json.load(open(os.path.join(vec, "playback.json")))
    for c in v["limits"]:
        try:
            o = loads(c["text"])
        except ValueError:
            o = None
        k = keep_limits(o)
        if (k is not None) != c["valid"] or (k is not None and text_of(k) != c["output"]):
            bad.append("playback limits %s: %s" % (c["name"], k))
    media = json.load(open(os.path.join(vec, "media.json")))
    for c in media["names"]:
        if (serves(c["ticket"], c["text"], TICKETS) is None) != c["valid"]:
            bad.append("media names %s: not %s" % (c["name"], c["valid"]))
    for c in media["scans"]:
        got = scan_answer(c["sent"], c.get("range"), c["size"], c.get("scanBytes"))
        if got != (c["status"], c["contentRange"]):
            bad.append("media scans %s: %s, not %s" % (c["name"], got, (c["status"], c["contentRange"])))
    return bad
