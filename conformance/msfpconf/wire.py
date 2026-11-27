"""MSFP core 0 on the wire (spec/v0/core.md): encodings, I-JSON,
addresses, the pairing code, signed requests and answers, statements,
and an HTTPS client with section 3.2's certificate checks."""

import base64
import hashlib
import hmac
import http.client
import ipaddress
import json
import os
import re
import socket
import ssl
import struct
import time
import unicodedata
import urllib.parse

from . import ed25519

VERSION = 0  # the core and every part this suite speaks (core 10)
REQUEST_LABEL = "msfp-0 request"
ANSWER_LABEL = "msfp-0 response"
PAIRING_LABEL = b"msfp-0 pairing\n"
CODE_PREFIX = "msfp-pair-0."
ROTATION_LABEL = "msfp-key-rotation-1"
REVOCATION_LABEL = "msfp-key-revocation-1"
FED = "/api/v1/federation/"
STREAM = "/api/v1/stream/"
MAX_INT = 2**53 - 1
H_KEY, H_DATE, H_NONCE, H_SIG = "MSFP-Key", "MSFP-Date", "MSFP-Nonce", "MSFP-Signature"

# Unicode's White_Space property (core 5.1, 9.1).
WHITE_SPACE = set("\t\n\x0b\x0c\r \x85\xa0\u1680\u2028\u2029\u202f\u205f\u3000") | {chr(c) for c in range(0x2000, 0x200B)}


# ---- Encodings (core 2) ---------------------------------------------------------

_B64U = re.compile(r"[A-Za-z0-9_-]*")


def b64u(b):
    return base64.urlsafe_b64encode(b).rstrip(b"=").decode("ascii")


def unb64u(text, size=None):
    """Strict base64url (core 2.2): the URL-safe alphabet only, no padding,
    unused bits zero. None when invalid, or not `size` bytes."""
    if not isinstance(text, str) or not _B64U.fullmatch(text) or len(text) % 4 == 1:
        return None
    try:
        b = base64.urlsafe_b64decode(text + "=" * (-len(text) % 4))
    except (ValueError, TypeError):
        return None
    if b64u(b) != text or (size is not None and len(b) != size):
        return None
    return b


def sha256hex(b):
    return hashlib.sha256(b).hexdigest()


def commitment(pub):
    """The next key commitment to a key (core 12.5)."""
    return b64u(hashlib.sha256(pub).digest())


def decimal(text):
    """A decimal integer of core 2.2, or None."""
    if not isinstance(text, str) or not re.fullmatch(r"0|[1-9][0-9]*", text) or int(text) > MAX_INT:
        return None
    return int(text)


class NotIJSON(ValueError):
    pass


class NonInteger(float):
    """A JSON number with a fraction or an exponent."""


class NegativeZero(int):
    """-0, which core 2.3 doesn't allow as an integer."""


def _clean_text(s):
    for ch in s:
        o = ord(ch)
        if 0xD800 <= o <= 0xDFFF or 0xFDD0 <= o <= 0xFDEF or (o & 0xFFFE) == 0xFFFE:
            return False
    return True


def parse_ijson(raw):
    """A body as core 2.3 reads it (I-JSON, RFC 7493). Numbers with a
    fraction or an exponent come back as NonInteger, -0 as NegativeZero;
    is_int() says whether a value is an integer of core 2.3. Raises
    NotIJSON."""
    if raw.startswith(b"\xef\xbb\xbf"):
        raise NotIJSON("a byte order mark")
    try:
        text = raw.decode("utf-8")
    except UnicodeDecodeError:
        raise NotIJSON("invalid UTF-8")

    def pairs(ps):
        seen = set()
        for k, _ in ps:
            if k in seen:
                raise NotIJSON("member %r twice" % k)
            if not _clean_text(k):
                raise NotIJSON("a surrogate or noncharacter in a name")
            seen.add(k)
        return dict(ps)

    def integer(t):
        return NegativeZero(0) if t == "-0" else int(t)

    def constant(t):
        raise NotIJSON("%s isn't JSON" % t)

    try:
        value = json.loads(text, object_pairs_hook=pairs, parse_int=integer,
                           parse_float=NonInteger, parse_constant=constant)
    except NotIJSON:
        raise
    except (ValueError, RecursionError) as e:
        raise NotIJSON("not JSON: %s" % e)

    def walk(v):
        if isinstance(v, str):
            if not _clean_text(v):
                raise NotIJSON("a surrogate or noncharacter in a string")
        elif isinstance(v, dict):
            for x in v.values():
                walk(x)
        elif isinstance(v, list):
            for x in v:
                walk(x)

    walk(value)
    return value


def is_int(v):
    """An integer of core 2.3: no fraction or exponent, not -0, within 2^53 - 1."""
    return isinstance(v, int) and not isinstance(v, (bool, NegativeZero)) and abs(v) <= MAX_INT


def has_null(v):
    if v is None:
        return True
    if isinstance(v, dict):
        return any(has_null(x) for x in v.values())
    if isinstance(v, list):
        return any(has_null(x) for x in v)
    return False


def dumps(obj):
    return json.dumps(obj, separators=(",", ":"), ensure_ascii=True).encode("ascii")


def clean_name(s, limit=80):
    """Core 9.1's cleaning of a name."""
    out, space = [], False
    for ch in s:
        if ch in WHITE_SPACE:
            if not space:
                out.append(" ")
            space = True
            continue
        space = False
        out.append(ch)
    t = "".join(ch for ch in "".join(out) if unicodedata.category(ch) not in ("Cc", "Cf")).strip(" ")
    return t[:limit].strip(" ")


# ---- Addresses (core 5.2, 3.3, 11) ------------------------------------------------

def check_host(host):
    """None when a host of core 5.2 (name, IPv4, [IPv6]), else why not."""
    if host.startswith("["):
        if not host.endswith("]") or "%" in host:
            return "not an IPv6 address in brackets"
        try:
            ip = ipaddress.IPv6Address(host[1:-1])
        except ValueError:
            return "not an IPv6 address"
        if ip.compressed != host[1:-1]:
            return "IPv6 not in RFC 5952 form"
        return None
    if not host or len(host) > 253:
        return "an empty host, or a name over 253 characters"
    labels = host.split(".")
    if labels[-1] and labels[-1].isdigit():
        if len(labels) != 4 or any(decimal(x) is None or int(x) > 255 for x in labels):
            return "a numeric host that isn't an IPv4 address in its one form"
        return None
    for x in labels:
        if not re.fullmatch(r"[a-z0-9]([a-z0-9-]{0,61}[a-z0-9])?", x):
            return "a label outside a-z 0-9 -, empty, over 63 characters, or starting or ending with -"
    return None


def check_address(a, allow_http=False):
    """None when an address of core 5.2, else why not. allow_http: the
    test switch of core 3.1, outside the protocol."""
    if not isinstance(a, str) or len(a) > 512 or not a.isascii():
        return "not at most 512 bytes of ASCII"
    m = re.fullmatch(r"(https?)://([^/?#@\\]+)", a)
    if not m:
        return "not https://host[:port] with nothing after"
    scheme, auth = m.groups()
    if scheme == "http" and not allow_http:
        return "http (core 0 is https only)"
    host, port = auth, None
    if auth.startswith("["):
        close = auth.find("]")
        if close < 0:
            return "an unclosed ["
        host, rest = auth[:close + 1], auth[close + 1:]
        if rest:
            if not rest.startswith(":"):
                return "junk after ]"
            port = rest[1:]
    elif ":" in auth:
        host, port = auth.rsplit(":", 1)
    why = check_host(host)
    if why:
        return why
    if port is not None:
        if decimal(port) is None or not 1 <= int(port) <= 65535:
            return "a port not 1 to 65535 without leading zeros"
        if port == ("443" if scheme == "https" else "80"):
            return "the scheme's default port"
    return None


def make_uncallable_address(raw=None):
    """https://msfp-<26>.invalid (core 5.2): 16 bytes in lowercase base32."""
    raw = raw if raw is not None else os.urandom(16)
    return "https://msfp-" + base64.b32encode(raw).decode().rstrip("=").lower() + ".invalid"


def check_uncallable_address(a):
    why = check_address(a)
    if why:
        return why
    m = re.fullmatch(r"https://msfp-([a-z2-7]*)\.invalid", a)
    if not m:
        return "not of the form https://msfp-<26>.invalid"
    mid = m.group(1)
    if len(mid) != 26:
        return "%d characters between msfp- and .invalid, not 26" % len(mid)
    raw = base64.b32decode(mid.upper() + "======")
    if make_uncallable_address(raw) != a:
        return "the unused bits aren't zero"
    return None


def check_base(s):
    """None when a discovery document's base of core 11, else why not."""
    if not isinstance(s, str) or len(s) > 256 or not s.isascii():
        return "not at most 256 bytes of ASCII"
    if not s.startswith("/") or not s.endswith(FED):
        return "not an absolute path ending in /api/v1/federation/"
    for seg in s[1:-1].split("/"):
        if seg in ("", ".", ".."):
            return "an empty, . or .. segment"
        if not re.fullmatch(r"[A-Za-z0-9._~-]+", seg):
            return "a character outside A-Z a-z 0-9 . _ ~ -"
    return None


def check_media_name(s):
    """A file or track name of MSFP Media fetching 1, or None when valid."""
    if not isinstance(s, str) or s in (".", "..") or not re.fullmatch(r"[A-Za-z0-9._-]{1,64}", s):
        return "not 1 to 64 characters of A-Z a-z 0-9 . _ - (or . or ..)"
    return None


_REFUSED = [ipaddress.ip_network(n) for n in ("127.0.0.0/8", "::1/128", "169.254.0.0/16", "fe80::/10", "0.0.0.0/8",
                                              "::/128", "224.0.0.0/4", "ff00::/8", "240.0.0.0/4")]


def check_destination(text):
    """None when core 3.3 lets a server connect to this IP address by
    default, else why not."""
    try:
        ip = ipaddress.ip_address(text)
    except ValueError:
        return "not an IP address"
    if ip.version == 6:
        if ip.ipv4_mapped:
            ip = ip.ipv4_mapped
        elif ip in ipaddress.ip_network("64:ff9b::/96"):
            ip = ipaddress.IPv4Address(ip.packed[12:])
        elif ip in ipaddress.ip_network("2002::/16"):
            ip = ipaddress.IPv4Address(ip.packed[2:6])
    for n in _REFUSED:
        if ip.version == n.version and ip in n:
            return "refused: in %s" % n
    return None


def host_of(address):
    """The <host> line of a request message (core 6.2)."""
    return address.split("://", 1)[1]


def split_hostport(hostport, default):
    if hostport.startswith("["):
        close = hostport.index("]")
        h, rest = hostport[1:close], hostport[close + 1:]
        return h, int(rest[1:]) if rest else default
    if ":" in hostport:
        h, port = hostport.rsplit(":", 1)
        return h, int(port)
    return hostport, default


def is_ip(h):
    try:
        ipaddress.ip_address(h)
        return True
    except ValueError:
        return False


# ---- The pairing code (core 5.1, 9.1) ------------------------------------------

def parse_code(text, allow_http=False):
    """(key, secret, expiry, address), or raises ValueError, as core 5.1 reads it.
    A group the code carries is checked too; parse_code_group returns it."""
    return parse_code_group(text, allow_http)[:4]


def parse_code_group(text, allow_http=False):
    """(key, secret, expiry, address, group or None, group name or None)."""
    clean = "".join(ch for ch in text if ch not in WHITE_SPACE)
    if not clean.startswith(CODE_PREFIX):
        raise ValueError("not the exact prefix msfp-pair-0.")
    rest = clean[len(CODE_PREFIX):]
    if len(rest) > 1164:
        raise ValueError("over 1,164 characters")
    b = unb64u(rest)
    if b is None:
        raise ValueError("not base64url in its one form")
    if len(b) <= 72:
        raise ValueError("72 bytes or fewer")
    key, secret, expiry = b[:32], b[32:64], struct.unpack(">Q", b[64:72])[0]
    group = name = None
    if b[72] == 0:
        at = 73
    elif b[72] == 1:
        if len(b) < 73 + 33:
            raise ValueError("too short for the group its flag announces")
        group, n = b[73:105], b[105]
        if n == 0:
            raise ValueError("a group name of 0 bytes")
        if len(b) < 106 + n:
            raise ValueError("too short for the group name's length")
        try:
            name = clean_name(b[106:106 + n].decode("utf-8"), 10**6)
        except UnicodeDecodeError:
            raise ValueError("a group name that isn't UTF-8")
        if len(name) > 80:
            raise ValueError("a group name over 80 code points once cleaned")
        at = 106 + n
    else:
        raise ValueError("a flag byte other than 0 and 1")
    addr = b[at:]
    if not addr:
        raise ValueError("no address")
    if not ed25519.valid_point(key):
        raise ValueError("the key breaks core 2.1")
    if expiry > MAX_INT:
        raise ValueError("an expiry above 2^53 - 1")
    try:
        address = addr.decode("ascii")
    except UnicodeDecodeError:
        raise ValueError("an address that isn't ASCII")
    why = check_address(address, allow_http)
    if why:
        raise ValueError("address: " + why)
    return key, secret, expiry, address, group, name


def make_code(key_pub, secret, expiry, address):
    return CODE_PREFIX + b64u(key_pub + secret + struct.pack(">Q", expiry) + b"\x00" + address.encode())


def proof(secret, caller_key, address):
    return hmac.new(secret, PAIRING_LABEL + caller_key + b"\n" + address.encode(), hashlib.sha256).digest()


# ---- Keys, messages and statements (core 4, 6, 12) ------------------------------

class Key:
    def __init__(self, seed=None):
        self.seed = seed if seed is not None else os.urandom(32)
        self.pub = ed25519.public_key(self.seed)
        self.text = b64u(self.pub)

    @classmethod
    def load_or_make(cls, path):
        if path and os.path.exists(path):
            with open(path, "rb") as f:
                return cls(f.read())
        k = cls()
        if path:
            fd = os.open(path, os.O_WRONLY | os.O_CREAT | os.O_EXCL, 0o600)
            with os.fdopen(fd, "wb") as f:
                f.write(k.seed)
        return k

    def sign(self, m):
        return ed25519.sign(self.seed, m)

    @property
    def commitment(self):
        return commitment(self.pub)

    @property
    def fingerprint(self):
        return fingerprint(self.pub)


def fingerprint(pub):
    t = base64.b32encode(hashlib.sha256(pub).digest()[:15]).decode()
    return "-".join(t[i:i + 4] for i in range(0, 24, 4))


def request_message(method, host, target, date, nonce, body):
    return "\n".join([REQUEST_LABEL, method, host, target, date, nonce, sha256hex(body)]).encode("ascii")


def answer_message(nonce, status, media_type, body):
    return "\n".join([ANSWER_LABEL, nonce, str(status), media_type, sha256hex(body)]).encode("ascii")


_MT = re.compile(r"([A-Za-z0-9!#$%&'*+.^_`|~-]+)/([A-Za-z0-9!#$%&'*+.^_`|~-]+)[ \t]*(;.*)?")


def media_type_of(headers):
    """The <media type> line of core 6.3 from an answer's headers: '-'
    without Content-Type (or with an empty one), None when it is sent twice
    or doesn't read as type/subtype (the answer fails like a badly signed one)."""
    vals = [v for k, v in headers if k.lower() == "content-type"]
    if len(vals) > 1:
        return None
    if not vals or not vals[0].strip():
        return "-"
    m = _MT.fullmatch(vals[0].strip())
    if not m:
        return None
    return ("%s/%s" % (m.group(1), m.group(2))).lower()


def rotation_statement(old, new, next_commitment, date):
    return "\n".join([ROTATION_LABEL, old, new, next_commitment or "-", str(date)]).encode("ascii")


def revocation_statement(key, date):
    return "\n".join([REVOCATION_LABEL, key, str(date)]).encode("ascii")


def make_rotation(old, new, next_commitment=None, date=None):
    """The members of a rotate request (core 9.3); old and new are Keys."""
    date = int(time.time()) if date is None else date
    st = rotation_statement(old.text, new.text, next_commitment, date)
    r = {"newKey": new.text}
    if next_commitment:
        r["nextCommitment"] = next_commitment
    r.update({"date": date, "oldSignature": b64u(old.sign(st)), "newSignature": b64u(new.sign(st))})
    return r


def make_revocation(key, date=None):
    date = int(time.time()) if date is None else date
    return {"date": date, "signature": b64u(key.sign(revocation_statement(key.text, date)))}


def read_rotation(old_pub, r, now=None):
    """Checks a rotation's members (a rotate body, or a pulled `rotation`)
    as core 12.3 step 1 does. Returns (new key bytes, next commitment or
    None), or raises ValueError with why `rotate` would answer 400."""
    now = time.time() if now is None else now
    if not isinstance(r, dict):
        raise ValueError("not an object")
    if has_null(r):
        raise ValueError("a null member (core 2.3)")
    nk, nc, d, so, sn = (r.get(x) for x in ("newKey", "nextCommitment", "date", "oldSignature", "newSignature"))
    new = unb64u(nk, 32) if isinstance(nk, str) else None
    if new is None or not ed25519.valid_point(new):
        raise ValueError("newKey isn't a valid key (core 2.1)")
    if new == old_pub:
        raise ValueError("newKey is the old key (core 12.2)")
    if "nextCommitment" in r and (not isinstance(nc, str) or unb64u(nc, 32) is None):
        raise ValueError("nextCommitment isn't base64url of 32 bytes")
    if not is_int(d) or d <= 0:
        raise ValueError("date isn't a positive integer")
    if abs(now - d) > 300:
        raise ValueError("the date is %+d s from this clock (core 12.3: within 5 minutes)" % (d - now))
    st = rotation_statement(b64u(old_pub), nk, nc, d)
    so_b, sn_b = unb64u(so, 64), unb64u(sn, 64)
    if so_b is None or not ed25519.verify_strict(old_pub, st, so_b):
        raise ValueError("oldSignature doesn't verify under the old key")
    if sn_b is None or not ed25519.verify_strict(new, st, sn_b):
        raise ValueError("newSignature doesn't verify under the new key")
    return new, nc


def read_revocation(key_pub, r, now=None):
    """Checks a revocation's members (core 12.6), or raises ValueError."""
    now = time.time() if now is None else now
    if not isinstance(r, dict):
        raise ValueError("not an object")
    d, s = r.get("date"), r.get("signature")
    if not is_int(d) or d <= 0:
        raise ValueError("date isn't a positive integer")
    if abs(now - d) > 300:
        raise ValueError("the date is %+d s from this clock (core 12.6: within 5 minutes)" % (d - now))
    sb = unb64u(s, 64)
    if sb is None or not ed25519.verify_strict(key_pub, revocation_statement(b64u(key_pub), d), sb):
        raise ValueError("the signature doesn't verify under the revoked key")


# ---- Certificates (core 3.2) --------------------------------------------------------

def _der(b, i):
    n, j = b[i + 1], i + 2
    if n & 0x80:
        k = n & 0x7F
        n = int.from_bytes(b[j:j + k], "big")
        j += k
    return j, j + n


def spki_of(cert_der):
    """The SubjectPublicKeyInfo of a DER certificate."""
    c = _der(cert_der, 0)[0]
    i = _der(cert_der, c)[0]
    if cert_der[i] == 0xA0:
        i = _der(cert_der, i)[1]
    for _ in range(5):              # serial, signature, issuer, validity, subject
        i = _der(cert_der, i)[1]
    return cert_der[i:_der(cert_der, i)[1]]


def spki_pin(cert_der):
    return b64u(hashlib.sha256(spki_of(cert_der)).digest())


def read_pins(v):
    """What a receiver keeps of `certificates` (core 3.2): the first 8
    valid entries, each once. A member that isn't an array counts as none."""
    pins = []
    if isinstance(v, list):
        for p in v:
            if isinstance(p, str) and len(p) == 43 and unb64u(p, 32) is not None and p not in pins and len(pins) < 8:
                pins.append(p)
    return pins


class CertificateRefused(Exception):
    pass


# ---- Calls to the server under test ---------------------------------------------------

def content_coding(headers):
    codings = [v.strip().lower() for k, v in headers if k.lower() == "content-encoding"]
    coded = [x for v in codings for x in (s.strip() for s in v.split(",")) if x and x != "identity"]
    return ", ".join(coded)


class Answer:
    """An answer, checked as core 6.6 says against `keys`."""

    def __init__(self, status, headers, body, nonce, keys, leaf_pin=None, trusted=None, path=""):
        self.status, self.headers, self.body, self.nonce, self.path = status, headers, body, nonce, path
        self.leaf_pin, self.trusted = leaf_pin, trusted
        sigs = [v for k, v in headers if k.lower() == "msfp-signature"]
        self.sig_header_count = len(sigs)
        self.media_type = media_type_of(headers)
        self.coding = content_coding(headers)
        self.signer = None              # the key (bytes) whose signature verified
        self.why_unsigned = ""
        if len(sigs) != 1:
            self.why_unsigned = "MSFP-Signature sent %d times" % len(sigs)
        elif self.media_type is None:
            self.why_unsigned = "Content-Type sent twice, or not type/subtype (core 6.3)"
        elif self.coding:
            self.why_unsigned = "Content-Encoding %s (core 3: counts as badly signed)" % self.coding
        else:
            sig = unb64u(sigs[0].strip(), 64)
            if sig is None:
                self.why_unsigned = "MSFP-Signature isn't base64url of 64 bytes"
            else:
                msg = answer_message(nonce, status, self.media_type, body)
                for k in keys:
                    if k is not None and ed25519.verify_strict(k, msg, sig):
                        self.signer = k
                        break
                else:
                    self.why_unsigned = "the signature doesn't verify under the key expected"
        if self.signer is not None and status in (200, 202) and self.media_type != "application/json":
            self.signer = None
            self.why_unsigned = "a signed %d whose media type is %s, not application/json (core 6.3)" % (status, self.media_type)
        self._json, self._json_error = None, None

    @property
    def signed(self):
        return self.signer is not None

    @property
    def json(self):
        if self._json is None and self._json_error is None:
            try:
                self._json = parse_ijson(self.body)
            except NotIJSON as e:
                self._json_error = str(e)
        return self._json

    @property
    def json_error(self):
        _ = self.json
        return self._json_error

    def problem_type(self):
        j = self.json
        return j.get("type") if isinstance(j, dict) else None

    def header(self, name):
        vals = [v for k, v in self.headers if k.lower() == name.lower()]
        return vals[0] if vals else None

    def short(self):
        t = self.problem_type() if self.status not in (200, 202) else None
        return "%s%s%s %r" % (self.status, " signed" if self.signed else " unsigned", " " + t if t else "", self.body[:120])


class Client:
    """Signed calls to one server, checked as core 3.2 and 6.6 say.

    connect: where to connect instead of the address's host (host:port),
    the requests staying signed for the address. server_keys: the keys an
    answer may verify under (normally the server's current key)."""

    def __init__(self, key, address, server_keys=(), connect=None, timeout=20, base=FED, any_certificate=False):
        self.key, self.address = key, address
        self.server_keys = server_keys if isinstance(server_keys, list) else list(server_keys)   # shared, not copied
        self.base = base
        self.connect_to = connect
        self.timeout = timeout
        self.any_certificate = any_certificate
        self.pins = []                  # of the server's latest signed hello
        self.untrusted = set()          # hosts whose certificate failed the trusted check
        self.log = []                   # every Answer
        self.on_cert_refused = None     # sends a hello after a refused certificate (core 3.2, renewals)

    @property
    def host(self):
        return host_of(self.address)

    @property
    def https(self):
        return self.address.startswith("https://")

    def _conn(self, checked, timeout=None):
        timeout = timeout or self.timeout
        target = self.connect_to or self.host
        h, port = split_hostport(target, 443 if self.https else 80)
        if not self.https:
            return http.client.HTTPConnection(h, port, timeout=timeout), None, None
        name = split_hostport(self.host, 443)[0]
        if self.host not in self.untrusted:
            ctx = ssl.create_default_context()
            raw = socket.create_connection((h, port), timeout)
            try:
                sock = ctx.wrap_socket(raw, server_hostname=name)
                c = http.client.HTTPSConnection(h, port, timeout=timeout, context=ctx)
                c.sock = sock
                return c, spki_pin(sock.getpeercert(binary_form=True)), True
            except (ssl.SSLCertVerificationError, ssl.CertificateError):
                self.untrusted.add(self.host)
                raw.close()
        ctx = ssl._create_unverified_context()
        sock = ctx.wrap_socket(socket.create_connection((h, port), timeout),
                               server_hostname=None if is_ip(name) else name)
        pin = spki_pin(sock.getpeercert(binary_form=True))
        if checked and not self.any_certificate and pin not in self.pins:
            sock.close()
            raise CertificateRefused("the leaf certificate %s is neither trusted nor named in the server's latest "
                                     "signed hello (named: %s)" % (pin, ", ".join(self.pins) or "none"))
        c = http.client.HTTPSConnection(h, port, timeout=timeout, context=ctx)
        c.sock = sock
        return c, pin, False

    def _checked_conn(self, checked, timeout=None):
        try:
            return self._conn(checked, timeout)
        except CertificateRefused:
            if not (checked and self.on_cert_refused):
                raise
            before = set(self.pins)
            self.on_cert_refused()
            if set(self.pins) == before:
                raise
            return self._conn(checked, timeout)

    def raw(self, path, body, headers, method="POST", checked=None, keys=None):
        """Sends exactly these headers (pairs) and body; returns an Answer."""
        name = path[len(self.base):] if path.startswith(self.base) else ""
        if checked is None:
            checked = name not in ("pair", "hello")
        c, pin, trusted = self._checked_conn(checked)
        try:
            c.putrequest(method, path, skip_host=True, skip_accept_encoding=True)
            c.putheader("Host", self.host)
            for k, v in headers:
                c.putheader(k, v)
            c.putheader("Content-Length", str(len(body)))
            c.endheaders(body)
            r = c.getresponse()
            data = r.read(4 * 1024 * 1024 + 1)
            hdrs = r.getheaders()
        finally:
            c.close()
        nonce = next((v.strip() for k, v in headers if k.lower() == "msfp-nonce"), "")
        a = Answer(r.status, hdrs, data, nonce, self.server_keys if keys is None else keys, pin, trusted, path)
        if len(data) > 4 * 1024 * 1024:
            a.signer, a.why_unsigned = None, "an answer over 4 MiB (core 3)"
        self.log.append(a)
        return a

    def headers(self, path, body, date=None, nonce=None, key=None, sign_host=None, sign_path=None, method="POST"):
        key = key or self.key
        date = str(int(time.time())) if date is None else date
        nonce = b64u(os.urandom(16)) if nonce is None else nonce
        msg = request_message(method, sign_host or self.host, sign_path or path, date, nonce, body)
        return [("Content-Type", "application/json"), (H_KEY, key.text), (H_DATE, date),
                (H_NONCE, nonce), (H_SIG, b64u(key.sign(msg)))]

    def call(self, name, body, key=None, keys=None, **kw):
        if isinstance(body, (dict, list)):
            body = dumps(body)
        path = self.base + name
        return self.raw(path, body, self.headers(path, body, key=key, **kw), keys=keys)

    def get(self, url, headers=(), method="GET", limit=80 * 1024 * 1024):
        """A media fetch (MSFP Media fetching 2): a plain GET, unsigned, needing a trusted
        or named certificate. Returns (status, headers, body)."""
        u = urllib.parse.urlsplit(url)
        if "%s://%s" % (u.scheme, u.netloc) != self.address:
            raise ValueError("%s isn't under the server's address %s" % (url, self.address))
        target = url[len(self.address):]
        c, _, _ = self._checked_conn(True, 90)
        try:
            c.putrequest(method, target, skip_host=True, skip_accept_encoding=True)
            c.putheader("Host", self.host)
            for k, v in headers:
                c.putheader(k, v)
            c.endheaders()
            r = c.getresponse()
            data = b"" if method == "HEAD" else r.read(limit)
            return r.status, r.getheaders(), data
        finally:
            c.close()

    def get_timed(self, url, headers=(), seconds=12.0, limit=80 * 1024 * 1024):
        """A media fetch read for at most `seconds`: (status, headers,
        [(time since the request, bytes read so far), ...]), the body
        dropped; for measuring a paced download (MSFP Media playing 6)."""
        u = urllib.parse.urlsplit(url)
        if "%s://%s" % (u.scheme, u.netloc) != self.address:
            raise ValueError("%s isn't under the server's address %s" % (url, self.address))
        c, _, _ = self._checked_conn(True, 90)
        try:
            t0 = time.time()
            c.putrequest("GET", url[len(self.address):], skip_host=True, skip_accept_encoding=True)
            c.putheader("Host", self.host)
            for k, v in headers:
                c.putheader(k, v)
            c.endheaders()
            r = c.getresponse()
            marks, n = [(time.time() - t0, 0)], 0
            while n < limit and time.time() - t0 < seconds:
                b = r.read1(65536)
                if not b:
                    break
                n += len(b)
                marks.append((time.time() - t0, n))
            return r.status, r.getheaders(), marks
        finally:
            c.close()

    def discovery(self):
        """GET /.well-known/msfp, any certificate (core 11): (status, headers, body)."""
        c, _, _ = self._conn(False)
        try:
            c.putrequest("GET", "/.well-known/msfp", skip_host=True, skip_accept_encoding=True)
            c.putheader("Host", self.host)
            c.endheaders()
            r = c.getresponse()
            return r.status, r.getheaders(), r.read(16 * 1024 + 1)
        finally:
            c.close()

    def burst(self, name, body, header_sets, connections=32):
        """Sends many prepared requests over a few kept-alive connections:
        a list of Answers, or exceptions, in order."""
        import concurrent.futures
        path = self.base + name
        chunks = [list(range(i, len(header_sets), connections)) for i in range(connections)]
        out = [None] * len(header_sets)

        def worker(idx):
            conn = None
            for i in idx:
                try:
                    if conn is None:
                        conn = self._conn(True)[0]
                    conn.putrequest("POST", path, skip_host=True, skip_accept_encoding=True)
                    conn.putheader("Host", self.host)
                    for k, v in header_sets[i]:
                        conn.putheader(k, v)
                    conn.putheader("Content-Length", str(len(body)))
                    conn.endheaders(body)
                    r = conn.getresponse()
                    data = r.read()
                    out[i] = Answer(r.status, r.getheaders(), data, dict(header_sets[i])[H_NONCE], self.server_keys)
                    if (r.getheader("Connection") or "").lower() == "close":
                        conn.close()
                        conn = None
                except (OSError, http.client.HTTPException, CertificateRefused) as e:
                    out[i] = e if isinstance(e, OSError) else OSError(str(e))
                    if conn is not None:
                        conn.close()
                    conn = None
            if conn is not None:
                conn.close()

        with concurrent.futures.ThreadPoolExecutor(connections) as ex:
            list(ex.map(worker, chunks))
        return out
