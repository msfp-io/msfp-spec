# MSFP: Media Server Federation Protocol, version 0

> **Version 0 was a test draft. It is superseded by
> [version 1](../v0/README.md), and is not to be implemented.** Version 1
> doesn't work with it. This document stays as history; it was corrected
> on 2026-11-16 and 2026-11-17 ([section 17](#17-changes)).

This document specifies MSFP, the server-to-server protocol paired media
servers speak; Overfin is its first implementation. It is written for
developers who build their own implementation, for example a Jellyfin
plugin that federates, without reading Overfin's code. It describes the
protocol Overfin speaks. Where Overfin is loose or specific to itself, the
text says so in **Version 0 notes**. The corrections of 2026-11-16 made
some rules stricter than Overfin's code was then; Overfin follows them
since, and the notes say what it did before.
Questions without an answer yet are collected under
[Open questions](#16-open-questions).

Version 0 covers manual pairing only: two operators pair their servers by
hand, and each trusts only the servers it paired with. Larger federations
are planned, not specified.

The operator's side (pairing in the panel, limits, what people see) is in
Overfin's [federation guide](https://github.com/Overfin/server/blob/main/docs/federation.md).
Overfin's security rules are its
[SECURITY.md](https://github.com/Overfin/server/blob/main/SECURITY.md)'s 94
to 100. This document doesn't repeat them; it says how they show on the wire.

The words "must", "must not" and "should" carry their usual meaning in
specifications. Every example uses `example.org` names and RFC 5737
addresses.

## Contents

1. [Overview and terms](#1-overview-and-terms)
2. [Encodings](#2-encodings), and [canonical forms](#21-canonical-forms)
3. [Transport](#3-transport)
4. [Identity and keys](#4-identity-and-keys)
5. [The pairing code](#5-the-pairing-code)
6. [Signing requests and answers](#6-signing-requests-and-answers)
7. [Pairing, step by step](#7-pairing-step-by-step)
8. [Common rules for every call](#8-common-rules-for-every-call)
9. [The calls](#9-the-calls)
10. [Fetching the media](#10-fetching-the-media)
11. [How titles are identified](#11-how-titles-are-identified)
12. [What the owning server enforces](#12-what-the-owning-server-enforces)
13. [Versioning](#13-versioning)
14. [Security considerations](#14-security-considerations)
15. [Test vectors](#15-test-vectors)
16. [Open questions](#16-open-questions)
17. [Changes](#17-changes)

## 1. Overview and terms

- **Server**: one Overfin installation, or another implementation of this
  protocol. It has one site address and one Ed25519 key pair.
- **Address**: where a server's API answers, `http(s)://host[:port]`, with
  no path (section 5.2).
- **Peer**: another server this server knows by its public key. A peer is
  in one of the states of section 7.3.
- **Pairing**: the act, done by two operators by hand, that makes two
  servers peers. Nothing is shared before it, and there is no discovery.
- **Pairing code**: one line of text an operator sends another operator.
  It holds the maker's address, public key, a one-time secret and an
  expiry time (section 5).
- **Library**: a collection of titles on a server, with an ID, a name and
  a kind.
- **Offer**: the libraries a server lets one peer see. Each server sets
  its own offers, per peer.
- **Want**: the libraries of a peer's offer that this server reads. Only
  titles in a library one side offers and the other wants are shared.
- **Owning server**: the server that holds a title's file and streams it.
- **Viewer's server**: the server whose signed-in person watches a title
  the owning server holds. The person's browser only ever talks to the
  viewer's server.
- **Title key**: the text that names a film, series, season or episode by
  its TMDB ID (section 11).
- **Playback**: one stream the owning server runs for the viewer's server.
  It has a playback ID and a ticket.
- **Ticket**: an opaque string in the stream URL. It is the only
  credential for fetching the media.

Every call is an HTTP `POST` with a JSON body, signed by the caller. Every
answer is signed by the server answering. The media itself is fetched with
plain `GET` requests that carry the ticket only (section 10).

## 2. Encodings

- **base64url**: the URL-safe alphabet of RFC 4648 section 5, **without
  padding**. Keys, nonces, signatures, the secret hash and the pairing
  proof use it.
- **hex**: lowercase hexadecimal. The body hash inside a signed message
  uses it.
- **JSON**: I-JSON (RFC 7493), a restricted form of RFC 8259 (section
  2.1). Field names are case-sensitive. Fields marked "optional" may be
  left out.
- **Times** are Unix seconds, written in decimal ASCII.
- Text in signed messages is ASCII, and lines are separated by a single
  line feed (`\n`, byte 0x0A). There is never a trailing line feed.

**`null`.** A sender must not send `null` as the value of any member: it
leaves an empty optional member out, or sends `[]` or `""` where the field
says so. A receiver treats a member whose value is `null` as absent,
because Overfin, before 2026-11-16, sent `null` for empty arrays. (Core
version 1 refuses `null` from a peer that speaks version 1; see
[spec/v1/core.md](../v0/core.md#2-encodings-and-canonical-forms).)

**Version 0 notes.** Since 2026-11-16, Overfin reads base64url,
`Overfin-Date`, the signature headers, public keys and signatures as
section 2.1 says (the `base64url`, `dates`, `codes`, `keys` and
`signatures` cases of [vectors/v0-strict.json](../../vectors/history/test-draft-strict.json)).
It checks each key once and keeps the result; a stored peer whose key
fails is shown unusable in its panel. It reads federation bodies, requests
and answers alike, as I-JSON with exact member names (the `json` and
`integers` cases), and reads `null` as absent. Since then it sends no
`null`.

### 2.1 Canonical forms

Every value below has exactly one valid text. A sender must write that
text. A receiver must refuse any other, the same way it refuses a bad
signature or a bad body: an unsigned `404` when the value is checked
before the signature checks out (section 6.4); after it, a signed refusal
with the status that call gives a bad body (section 8.2).

**Ed25519.** A **public key** is valid only when all of these hold:

1. it is 32 bytes;
2. it is the canonical encoding of a point: the 255-bit y is below
   p = 2^255 − 19, the point is on the curve, and the sign bit is 0 when
   x = 0;
3. the point is in the prime-order subgroup and is not of small order:
   [L]A is the identity and A is not, where
   L = 2^252 + 27742317777372353535851937790883648493. This excludes the
   8 points of small order (the identity among them), and every point
   with a small-order part.

A **signature** (R ‖ S, 64 bytes) is valid for a message M under a valid
key A only when:

1. R satisfies conditions 2 and 3 above;
2. S, read as a little-endian integer, is below L;
3. [S]B = R + [k]A, where k = SHA-512(R ‖ A ‖ M) mod L and B is the base
   point.

With A and R both in the prime-order subgroup, the cofactored and the
cofactorless equations of RFC 8032 accept exactly the same signatures. So
any RFC 8032 library may do step 3, once the point checks are done. Every
honest signature passes: R = [r]B and A = [a]B. The point checks are
needed because common libraries differ: some accept the identity as a
public key, and under it the signature (R = identity, S = 0) verifies
every message.

A receiver checks a key once, when it first learns it (in a code, or in
`Overfin-Key` on `pair`), and may keep the result. It checks R on every
signature.

**Text encodings.**

- **base64url**: only the characters `A–Z a–z 0–9 - _`; no padding, no
  white space; the unused bits of the last character are zero. A simple
  test: re-encoding the decoded bytes gives the same text. Lengths are
  fixed where the value is: 43 characters for 32 bytes, 22 for 16, 86 for
  64.
- **hex**: lowercase `0–9 a–f` only. A SHA-256 is 64 characters.
- **Decimal integers** in headers and in signed messages: `0` or
  `[1-9][0-9]*`, no sign, no white space, at most 9,007,199,254,740,991
  (2^53 − 1). `Overfin-Date` is a positive integer in this form.
- **Status** in an answer message: the three digits of the status code.

**JSON.**

- Bodies are I-JSON (RFC 7493): UTF-8 without a byte order mark; no
  surrogate code points (escaped or not, so no lone surrogates) and no
  noncharacters; no object with the same member name twice, names being
  compared after their escapes are decoded.
- Member names match exactly, case included. A name in another case is
  an unknown member.
- Unknown members are ignored. This is not leniency: it is how later
  versions add fields (section 13).
- Integers: a JSON number with no fraction, no exponent, not `-0`, from
  −(2^53 − 1) to 2^53 − 1. Every integer MSFP sends fits.
- String values are compared and signed after JSON decoding. Since
  addresses, keys and IDs are ASCII, a sender has no reason to escape any
  of them.
- Insignificant white space between JSON tokens is allowed. Bodies are
  signed as bytes, never rebuilt, so their layout doesn't matter. Senders
  should send none.

**Headers.**

- Each of the four signature headers must appear exactly once in a
  request, and `Overfin-Signature` exactly once in an answer. Two
  instances, or one value holding a comma, are refused.
- A header value is taken after HTTP removes leading and trailing
  optional white space. Any other white space inside makes it invalid.
- `Content-Type` is compared without its parameters:
  `application/json; charset=utf-8` is `application/json`.

**Case.** Header names are case-insensitive, as in HTTP. Everything else
is case-sensitive and has one case:

- scheme and host of an address: lowercase (section 5.2);
- hex: lowercase;
- enumerated values (`paired`, `movies`, `direct`, `hls`…): exactly as
  listed;
- title keys: exactly as section 11 writes them;
- language tags: the case conventions of BCP 47 (RFC 5646 section
  2.1.1): language lowercase, script title case, region uppercase
  (`fr-CA`, `zh-Hant-TW`). Receivers compare tags case-insensitively, so
  `fr-ca` and `fr-CA` count as one tag, but senders write the
  conventional form.

Addresses (section 5.2) and pairing codes (section 5.1) have their own
rules. [vectors/v0-strict.json](../../vectors/history/test-draft-strict.json) holds cases
for all of these, valid and not (section 15).

## 3. Transport

- Scheme: `https`, or plain `http`. Both are allowed. Over `http` every
  request and answer is still signed, so neither server can be
  impersonated, but titles and tickets travel in the clear (rule 99).
- Paths: all calls are under `/api/v1/federation/` on the peer's address.
  The media is under `/api/v1/stream/` (section 10).

  | Call | Method and path | Who calls whom |
  |------|-----------------|----------------|
  | pair | `POST /api/v1/federation/pair` | the server redeeming a code calls the code's maker |
  | hello | `POST /api/v1/federation/hello` | either peer |
  | libraries | `POST /api/v1/federation/libraries` | either paired peer |
  | titles | `POST /api/v1/federation/titles` | either paired peer |
  | play | `POST /api/v1/federation/play` | the viewer's server calls the owning server |
  | progress | `POST /api/v1/federation/progress` | the viewer's server calls the owning server |
  | stop | `POST /api/v1/federation/stop` | the viewer's server calls the owning server |

  There are no other federation calls in version 0.
- Request headers: `Content-Type: application/json` is required (compared
  without parameters, section 2.1). The four signature headers of section
  6 are required.
- Requests must not carry `Sec-Fetch-Site` with a value other than
  `same-origin` or `none` (browsers' cross-site marker). A
  server-to-server client sends no `Sec-Fetch-*` headers at all.
- A request without `Content-Type: application/json`, or with a cross-site
  `Sec-Fetch-Site`, is refused with the unsigned `404` of a failed check
  (section 6.4).
- Answer bodies: `application/json` for `200`. Signed refusals are RFC 9457
  problem documents, `application/problem+json`, with `type`, `title` and
  `status` members (section 8.2). One refusal has an empty body (`409` on
  pair).
- Content codings: none. A federation request and its answer carry no
  `Content-Encoding` other than `identity`, and the signatures of section
  6 cover the body bytes as sent. A receiver sends its answers uncoded,
  whatever the request's `Accept-Encoding` says: a host that compresses
  answers by itself must leave the federation paths alone. A caller
  should send no `Accept-Encoding` (or `identity` only), and refuses an
  answer that carries a content coding like a badly signed one (section
  6.6), without decoding it. A receiver never decodes a coded request
  either: its signature is checked over the bytes received, so it fails
  as a bad signature or a bad body; a receiver may also refuse it at once
  with the unsigned `404` of a failed check. Transfer codings (`chunked`)
  are HTTP's framing, not a content coding: the body is the bytes they
  carry.
- The body of an unsigned refusal is not specified. A receiver should
  send whatever its host sends for a path that doesn't exist, so that the
  two can't be told apart. Where the host can't do that, it sends the
  problem document of section 8.2.
- Redirects: a caller must not follow them. A request is signed for one
  host only. Overfin's client treats a `3xx` like any other non-`200`
  status (section 6.6).
- Sizes: Overfin refuses request bodies over 64 KiB (65,536 bytes) with an
  unsigned `404`. Overfin reads at most 4 MiB (4,194,304 bytes) of an
  answer; a longer answer fails its signature check.
- Timeouts: Overfin's client gives each call 15 seconds in all.

A server behind a reverse proxy must keep the path, query and body
byte for byte: the signature covers them.

**Version 0 notes.** Overfin answers a `POST` without
`Content-Type: application/json` with `415`, and a cross-site
`Sec-Fetch-Site` with `403`, from its web stack, before federation code
runs. Other implementations answer both with the unsigned `404` above. A
wrong method on a federation path gets an unsigned error from Overfin.
Overfin never content-codes a federation answer. As a caller, as of
2026-11-17, it sends `Accept-Encoding: gzip` (its HTTP client's default)
and unpacks a gzipped answer before checking its signature, so it
accepts an answer this section refuses.

### 3.1 HTTPS certificates

Identity rests on the key, never on the certificate (section 4). But the
media fetch of section 10 is not signed: its ticket and bytes have only
TLS to protect them. So a server's key vouches for its certificate, and a
peer over HTTPS may use a certificate no public authority issued
(self-signed, or from a private authority).

**The pin.** A certificate is named by its pin: the SHA-256 of its
`SubjectPublicKeyInfo` (the DER bytes of the public key with its
algorithm, as in the certificate), sent base64url without padding
(43 characters). This is the same value as HPKP's `pin-sha256`, in
base64url instead of base64. A server names its pins in the
`certificates` field of `hello` (section 9.2). It should name the pin of
every certificate its address may serve now: the leaf certificate's, not
an authority's.

**What a client accepts.** For a connection to a peer's address over
`https`, the client checks the certificate chain the peer's TLS server
presents, in one of three ways:

| Check | Passes when |
|-------|-------------|
| *trusted* | the chain is valid for the address's host against the client's usual trusted roots, as for any web site (names, dates, usages) |
| *named* | the pin of the first (leaf) certificate is one the peer named in its latest signed `hello`; names, dates and the rest of the chain are not checked |
| *any* | any certificate at all |

Only the leaf counts for *named*: the TLS handshake proves the server
holds the leaf's private key, and no other's. A pin of an authority in the
chain must not make the chain pass.

Which check each request needs:

| Request | Accepted |
|---------|----------|
| `pair`, `hello` | *trusted*, *named* or *any* |
| every other federation call (`libraries`, `titles`, `play`, `progress`, `stop`) | *trusted* or *named* |
| the media, `GET /api/v1/stream/...` (section 10) | *trusted* or *named* |

`pair` and `hello` may take any certificate because their answers are
checked by key (section 6.6) and neither way carries a secret: the
pairing proof is bound to the caller's key and address (section 9.1).
They are how pins are learnt, before any is known and after a renewal.
Every other request needs a *trusted* or *named* certificate, `play` above
all, since its answer carries the ticket.

**Keeping pins.** The client stores, with the peer, the pins of the
latest signed `hello` it got from it, request or answer, latest by
arrival. Each one replaces the last one's: a `hello` without
`certificates`, or with an empty list, removes them. A receiver ignores
entries that aren't 43 characters of base64url decoding to 32 bytes, and
keeps the first 8 valid entries, each pin once, in the order sent. Pins
only add to what *trusted* accepts; they never refuse a trusted
certificate.

- A `certificates` member that isn't an array: in a request, the body is
  refused like a bad body (section 9.2), and the caller's pins stay as
  they were; in a signed answer, it counts as no pins.
- A signed `404` to a `hello` leaves the stored pins as they were.
- When a peer's pins change, the client closes its connections to that
  peer that were accepted only by a pin the peer no longer names.

**Renewals.** A server whose certificate is renewed names the new pin in
its next `hello`. A client that is refused a certificate on a call that
needs *trusted* or *named* should send a `hello` to that peer, and, if the
pins it gets differ from those it had (compared as sets), try the call
once more. Overfin does so for every call and for media fetches. A client
sends at most one such `hello` to a peer at a time; calls and fetches
that wait on it share its result.

A server's answer to a `hello` names the pins of the certificates its
address serves when it answers, not those of an earlier reading. A
server that learns its pins by reading its own certificate (below) reads
it again for each `hello` it answers, unless its last reading is at most
a few seconds old. A peer sends a `hello` when a call was refused the
certificate: an answer that named the pin from before a renewal would
make that peer's one retry fail, and nothing would try again before its
next scheduled call.

**Knowing one's own certificate.** A server behind a reverse proxy doesn't
hold its certificate. Overfin connects to its own address, as a peer
would, with SNI set to the address's host name (none for an IP address,
as RFC 6066 says), reads the leaf certificate served there, and names its
pin. It reads it again when it sends a `hello` a minute or more after
the last reading, and when it answers one 2 seconds or more after it
(since 2026-11-17; before, a minute too). A failed reading keeps the last
good one (none if there was never one); the next attempt waits a minute,
or 5 seconds while no reading has ever worked. When
the operator sets pins in Overfin's configuration (`site.certificates`),
Overfin names those and reads nothing. Over `http`, it names none.
Another implementation may learn its pins any way it likes, as long as
they are of the certificates its address really serves.

A server that serves several certificates (RSA and ECDSA, several
machines, a renewal rolling out) names all their pins. One reading sees
only one of them, so such a server should take its pins from its
configuration.

A server whose certificate is publicly trusted needs no pins; Overfin
names its pin all the same.

An implementation that does none of this names no pins, ignores the
field, and is reached as before: over `https` with a trusted certificate,
or over `http`.

### 3.2 Connecting to peers

Every address a server connects to for federation comes from a peer: the
address in a code (section 5), in a `pair` request (section 9.1), and the
media URLs under a peer's address (section 10.1). A hostile peer could
name an address that points back into the server's own machine or
network, so each connection made for a peer is checked first.

- **Refused by default:** loopback (`127.0.0.0/8`, `::1`); link-local
  (`169.254.0.0/16`, `fe80::/10`), where a cloud's metadata service
  answers at `169.254.169.254`; unspecified (`0.0.0.0/8`, `::`);
  multicast (`224.0.0.0/4`, `ff00::/8`); reserved and broadcast
  (`240.0.0.0/4`, with `255.255.255.255`). An IPv4-mapped IPv6 address
  (`::ffff:0:0/96`) is judged as the IPv4 address it carries.
- **IPv4 inside IPv6.** A NAT64 address of the well-known prefix
  `64:ff9b::/96` (RFC 6052; the IPv4 address is its last 32 bits) and a
  6to4 address, `2002::/16` (RFC 3056; the IPv4 address is bits 16 to
  47), are judged as the IPv4 address they carry, by every rule here,
  operator policy included. Where a NAT64 gateway or a 6to4 relay is on
  the path, `64:ff9b::a9fe:a9fe` reaches `169.254.169.254`. A server whose
  network uses another NAT64 prefix (a network-specific one, or RFC 8215's
  `64:ff9b:1::/48`) should judge addresses under it the same way.
- **Allowed by default:** private ranges (`10.0.0.0/8`, `172.16.0.0/12`,
  `192.168.0.0/16`, `100.64.0.0/10`, `fc00::/7`), so that two servers on
  one network can pair.
- **Operator policy.** An operator may be allowed to allow loopback, for
  a peer on the same machine, and to refuse more ranges. The other
  refused ranges stay refused whatever the setting.
- **Names.** A name is looked up once per connection. Every address of
  the answer must pass, or the name is refused. The connection goes to an
  address that was checked, never to a second lookup's answer. Each new
  connection looks up and checks again.
- **Proxies.** Where connections go through a proxy or a gateway, the
  check happens where the connection is made. No proxy from the
  environment is used.
- **Redirects** are never followed (section 3).
- A refused address fails like an unreachable peer: nothing is sent.

Core version 1 says the same, in more detail
([core section 3.3](../v0/core.md#33-connecting-to-peers)).

**Version 0 notes.** Overfin refuses these ranges on every connection to
a peer, and when it reads its own certificate at its own address
(section 3.1); behind a gateway, the gateway checks the same way. Since
2026-11-17 it judges NAT64 (`64:ff9b::/96`, and `64:ff9b:1::/48` by its
last 32 bits) and 6to4 addresses as above, and also refuses three cloud
metadata addresses outside these ranges: `100.100.100.200`,
`168.63.129.16` and `fd00:ec2::254`.
Its setting `federation.allow_loopback` allows loopback and nothing else.
It checks at connection time only. Before 2026-11-16, a single Overfin
(without a gateway) made no such check.

## 4. Identity and keys

### 4.1 The key

Each server has one Ed25519 key pair (RFC 8032, pure Ed25519, not
Ed25519ph). It is made once, on first start, and kept. A server is known
to its peers by its public key alone, never by its name, address or
certificate. Overfin stores the 32-byte seed, sealed.

A public key must satisfy the Ed25519 rules of section 2.1. A server
refuses a key that doesn't, wherever it meets it: in a code, or in
`Overfin-Key` on `pair`.

The public key travels as the 32 raw bytes, base64url without padding: 43
characters. Example (the test key "a" of section 15):

```
A6EHv_POEL4dcN0Y50vAmWfk1jCbpQ1fHdyGZBJVMbg
```

Signatures are the 64 raw bytes, base64url without padding: 86
characters.

There is no other server ID on the wire. Overfin gives each peer a local
ID (`peer_…`) for its own panel; peers never see it.

### 4.2 The fingerprint

Operators compare fingerprints, by phone or another channel they trust,
before they accept a pairing. The fingerprint is:

1. SHA-256 of the 32 raw public-key bytes;
2. the first 15 bytes of that hash;
3. encoded in standard base32 (RFC 4648 section 6, alphabet `A–Z2–7`):
   exactly 24 characters, no padding;
4. split into six groups of four, joined by `-`.

Example, key "a": `KZDV-VJ2U-MNDU-YAUF-35O3-6K6K`.

The fingerprint never travels between servers. Each panel computes it.

## 5. The pairing code

### 5.1 Format

A code is the ASCII prefix `overfin-pair-1.` followed by base64url
(without padding) of these bytes, in order:

| Bytes | Content |
|-------|---------|
| 32 | the maker's public key |
| 32 | the secret: 32 random bytes |
| 8 | when the code expires: Unix seconds, unsigned, big-endian |
| 1 or more | the maker's address, as bytes (section 5.2) |

The maker sets the expiry to 24 hours after it made the code, cut to the
whole second. The code works once.

A reader must:

- remove every character with the Unicode `White_Space` property,
  anywhere in the text (codes get wrapped in emails). A valid code is
  ASCII, so this can't change one;
- refuse text without the exact prefix, case included;
- refuse a remainder that isn't base64url as section 2.1 says, or is over
  779 characters (584 bytes: 72 plus a 512-byte address);
- refuse a decoded value of 72 bytes or fewer (the address can't be
  empty);
- refuse a key that breaks section 2.1's Ed25519 rules;
- refuse an expiry above 2^53 − 1;
- refuse address bytes that aren't ASCII or break section 5.2.

The reader doesn't check whether the code has expired. The maker does,
when the code is redeemed, and refuses it then.

### 5.2 Addresses

An address, in a code, in the pair request, and wherever a server writes
its own, has this form and no other:

```
address = scheme "://" host [ ":" port ]
scheme  = "https" / "http"
port    = a decimal integer from 1 to 65535, without leading zeros,
          and not the scheme's default (443 for https, 80 for http)
host    = name / ipv4 / "[" ipv6 "]"
name    = labels of 1 to 63 characters of a-z 0-9 -, not starting or
          ending with -, joined by "."; at most 253 characters in all
ipv4    = four decimal numbers 0 to 255 without leading zeros,
          joined by "."
ipv6    = an IPv6 address in the text form of RFC 5952 (lowercase,
          longest run of zeros compressed); no zone identifier
```

- A host whose last label is all digits is read as an IPv4 address, and
  must be one: `192.0.2.010` and `192.0.2` are refused, not read as
  names.
- The whole address is at most 512 bytes of ASCII.
- There is no path, not even `/`; no `?`, `#` or `@`. A server that has a
  trailing slash in its own configuration removes it before it writes its
  address anywhere.

Examples: `https://a.example.org`, `http://192.0.2.10:8096`. A server
hosted under a path can't federate in version 0.

**Version 0 notes.** Overfin's check follows this section since
2026-11-16 (the addresses of
[vectors/v0-strict.json](../../vectors/history/test-draft-strict.json)). Pairings it stored
before keep their address as stored.

**Stored pairings.** An implementation that adopts these rules after it
has pairings keeps a pairing whose stored address breaks this section,
and normalizes the address once (scheme and host in lowercase, trailing
`/` and empty query removed). A stored key that breaks section 2.1 is
marked unusable and shown to the operator; no honest key does.

### 5.3 Example

The code made by key "a", at `https://a.example.org`, with the secret
`404142…5f` (section 15), expiring at 1800086400:

```
overfin-pair-1.A6EHv_POEL4dcN0Y50vAmWfk1jCbpQ1fHdyGZBJVMbhAQUJDREVGR0hJSktMTU5PUFFSU1RVVldYWVpbXF1eXwAAAABrSyOAaHR0cHM6Ly9hLmV4YW1wbGUub3Jn
```

## 6. Signing requests and answers

### 6.1 Headers

Every request carries four headers:

| Header | Value |
|--------|-------|
| `Overfin-Key` | the sender's public key, base64url (43 characters) |
| `Overfin-Date` | when the request was signed, Unix seconds, decimal |
| `Overfin-Nonce` | 16 random bytes, base64url (22 characters), never used twice |
| `Overfin-Signature` | the sender's Ed25519 signature of the request message, base64url |

Every signed answer carries one header:

| Header | Value |
|--------|-------|
| `Overfin-Signature` | the answering server's Ed25519 signature of the answer message, base64url |

### 6.2 The request message

The sender signs these bytes: seven lines, joined by `\n`, with no
trailing `\n`.

```
overfin-federation-1 request
<method>
<host>
<target>
<date>
<nonce>
<body hash>
```

- Line 1 is the literal label `overfin-federation-1 request`.
- `<method>`: the HTTP method, in capitals: `POST` for every call.
- `<host>`: the host part of the **receiver's** address (section 5.2),
  with its port if it has one, in lowercase. Example: `a.example.org`, or
  `192.0.2.10:8096`. No scheme.
- `<target>`: the request target, in origin form: the path as sent on the
  wire (percent-encoded as sent), plus `?` and the query when there is
  one. Federation calls have no query, so this is the path, for example
  `/api/v1/federation/pair`. The receiver takes `<target>` from the
  request target as it received it, percent-encoding untouched, not from
  a decoded path. A federation call with a query is refused like a bad
  signature.
- `<date>`: exactly the text of the `Overfin-Date` header.
- `<nonce>`: exactly the text of the `Overfin-Nonce` header.
- `<body hash>`: SHA-256 of the exact body bytes sent, in lowercase hex
  (64 characters). The bytes carry no content coding (section 3). An
  empty body hashes to `e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855`.

The receiver rebuilds this message from **its own** address's host, not
from the `Host` header. A request signed for one server therefore fails
on every other, even when it is sent there.

### 6.3 The answer message

The answering server signs these bytes: four lines, joined by `\n`, with
no trailing `\n`.

```
overfin-federation-1 response
<nonce>
<status>
<body hash>
```

- Line 1 is the literal label `overfin-federation-1 response`.
- `<nonce>`: the text of the request's `Overfin-Nonce` header. This binds
  the answer to its request: it can't be replayed for another one.
- `<status>`: the HTTP status code sent, in decimal (`200`, `404`…).
- `<body hash>`: SHA-256 of the exact answer body bytes as sent,
  lowercase hex. They carry no content coding (section 3).

Headers are not signed, nor is the `Content-Type`.

### 6.4 Checking a request

The receiver checks, in this order, and refuses at the first failure:

1. Each of the four signature headers appears exactly once, the request
   has no query (sections 2.1 and 6.2), and `Overfin-Key` is base64url of
   exactly 32 bytes (section 2.1).
2. `Overfin-Date` is a positive decimal integer in the form of section
   2.1.
3. The date is within 5 minutes of the receiver's clock: the difference
   between the receiver's current time and the date, in either direction,
   is at most 300 seconds. The receiver's time is not rounded.
4. The date is not earlier than the moment the receiver's nonce memory
   began (section 6.5). The date counts as the start of its second: a
   request dated the second the process started is refused, unless the
   process started exactly on that second.
5. `Overfin-Nonce` is base64url of exactly 16 bytes.
6. `Overfin-Signature` is base64url of exactly 64 bytes.
7. The key is valid (section 2.1; a known peer's key was checked when it
   was learnt), and the signature is valid under it, by section 2.1's
   rules (R checked), over the request message of section 6.2.
8. The key is one the receiver hears (section 6.5).
9. The pair (key, nonce text) was never accepted before, and the nonce
   memory has room.

Then the receiver remembers (key, nonce text) and handles the call.

Every failure gets the same answer: an unsigned `404`, the same as a
path that doesn't exist (section 3). The sender learns nothing about why.
There is one exception, in step 8: a key whose pairing this receiver
ended (section 7.4) gets a **signed** `404`, so that its server can trust
it and mark the pairing gone.

### 6.5 Nonces, replay and restarts

- **Who is heard.** After the signature checks out, the receiver asks
  whether it hears the key. It hears a key it knows as a peer, in any
  state. On `/api/v1/federation/pair` only, it also hears every key while
  at least one of its pairing codes is live (made and neither used nor
  expired), forgotten keys included; otherwise a new code could never
  restart an ended pairing. On every other call, and on `pair` without a
  live code, a forgotten key gets the signed `404` of section 6.4. Any
  other key is refused before its nonce is stored. So strangers can't fill
  the memory with nonces they made up.
- **The memory.** The receiver keeps every accepted (key, nonce) pair for
  at least 10 minutes (twice the 5-minute window), from the moment the
  request was accepted. Older requests fail step 3 anyway. Overfin keeps two memories: one for known peers (at most
  100,000 entries) and one for strangers heard during pairing (at most
  1,000). A nonce in either one is refused. When a memory is full, after
  old entries are cleared, requests that would go in it are refused until
  room comes back. Overfin clears old entries at most once a minute, or
  when a memory is full.
- **Restarts.** Overfin keeps nonces in memory only. A request signed
  before the process started may have been seen before the restart, so
  step 4 refuses every request dated before the process started. There is
  no allowance for the other server's clock. If the sender's clock runs
  behind by *n* seconds, its requests are refused for about *n* seconds
  after each restart of the receiver. Keep both clocks set by the
  network.
- **Retrying after a restart.** A sender whose call to a peer is refused
  with an unsigned `404` should retry it once, a second later, with a new
  nonce (signed again). A receiver that just restarted then accepts it.
  Overfin's client doesn't retry yet.
- **A sender** must make a fresh random nonce for every request, from a
  cryptographic random source, and must never reuse one, even for a retry.
  A retry is a new request, signed again.

An implementation may keep nonces on disk instead. It must then still
refuse any request whose nonce it can't prove it hasn't seen.

### 6.6 Signing answers and checking them

The answering server signs:

- every `200` answer;
- every refusal (any other status) to a request that passed section 6.4
  from a key it knows as a peer, in any state;
- the `404` to a key whose pairing it ended.

It doesn't sign refusals to strangers: requests that failed section 6.4,
and requests from strangers on `pair` during a live code. Those stay plain
`404`s.

The caller checks every answer against the public key it knows the
answering server by. For `pair`, that is the key in the code.

- A `200` answer counts only when its signature verifies. Otherwise the
  caller treats the server as unreachable. Then it decodes the JSON body.
- An answer with a `Content-Encoding` other than `identity` counts as
  badly signed, whatever its signature (section 3).
- Any other status counts only when its signature verifies. An unsigned
  or badly signed refusal may come from someone between the two servers;
  the caller treats it as "unreachable" and changes no state.
- The one exception is `pair`. The code's maker doesn't know the caller
  yet, so its refusals are unsigned. The caller accepts a refusal on
  `pair`, signed or not. Faking one only spoils that one pairing.

## 7. Pairing, step by step

Server A's operator makes the code. Server B's operator pastes it.

### 7.1 The steps

1. **A makes a code.** A draws a 32-byte secret, stores it with its
   expiry (24 hours), and shows the code (section 5). A stores
   SHA-256(secret) to find the code later. A's operator sends the code to
   B's operator by any means.
2. **B reads the code.** B parses it (section 5.1). B refuses it when the
   key is B's own, or a key B already knows as a peer, in any state. To
   pair again with a peer that is `gone`, B's operator removes it first
   (section 7.4).
3. **B calls pair.** B sends a signed `POST /api/v1/federation/pair` to the
   code's address (section 9.1). The body names B's address and name,
   carries SHA-256(secret), and a proof that B knows the secret. The
   secret itself never travels.
4. **A redeems the code.** A finds a live code by the secret hash, and
   checks the proof against the stored secret, the request's
   `Overfin-Key` and the `address` field. A refuses the request when the
   key is already a peer of A's (`409`). Otherwise A, in one transaction,
   deletes the code, takes B's key off its forgotten list if it is there
   (section 7.4), and stores B: B's public key, B's address and B's
   cleaned name, in state `ours` (waiting for A's operator). A answers
   `200`, signed, with A's name.
5. **B stores A.** B checks the answer's signature against the key in the
   code. Only then B stores A: the code's key, the code's address, and A's
   cleaned name, in state `theirs` (waiting for A's operator). B takes A's
   key off its own forgotten list if it is there: redeeming a code is
   consent to pair again.
6. **A's operator accepts.** A's operator compares B's fingerprint with
   B's operator, then accepts. A moves B to `paired`, and sends B a signed
   `hello` with state `paired` (section 9.2). B, which was waiting, moves A
   to `paired`.
7. **If B missed it.** If B couldn't be reached at step 6, B learns at its
   next `hello` to A: A answers `paired`. Overfin asks every 20 seconds at
   most while the panel shows a peer in state `theirs`. Any call that A
   refuses with a signed `404` also makes B ask (section 9.2).

The operator of B gave consent at step 2, by pasting the code. A's
operator gives it at step 6. B can't make A accept: a `hello` saying
`paired` from a peer in state `ours` changes nothing.

### 7.2 What each side stores

For each peer: its public key (the identity), its address (where to call
it), its name (for the panel), its state, when it was last heard from
(any request from the peer that passed section 6.4, and any answer from
it whose signature verified), the
libraries offered to it, the libraries wanted from it, the limits it may
use here, and the limits it said we may use there. A stores B's address as
B gave it in the pair request. B stores A's address as the code gave it.

### 7.3 States

| State | Meaning | `hello` view sent |
|-------|---------|-------------------|
| `ours` | the peer used our code; our operator hasn't accepted | `waiting` |
| `theirs` | we used the peer's code; its operator hasn't accepted | `waiting` |
| `paired` | both sides accepted | `paired` |
| `gone` | the peer answered, signed, that it doesn't know us | `waiting` |

`libraries`, `titles`, `play`, `progress` and `stop` answer only peers in
state `paired`. Others get a signed `404`. `hello` answers peers in every
state. Overfin remembers whether a `gone` peer was paired before. A
`paired` hello brings it back to `paired` only then.

### 7.4 Ending a pairing

Either operator ends a pairing at once, or refuses a waiting one.
Removing a peer, in any state (`gone` included), is an ending:

1. The ending server deletes the peer, and records the key's SHA-256 as
   "forgotten".
2. Then it sends the peer a signed `hello` with state `ended`. If the peer
   can't be reached, nothing more is done.
3. The peer that receives `ended` deletes the ender, in every state,
   waiting ones included, and answers `200` with state `ended`, signed. It
   doesn't record the ender as forgotten: the ender has deleted it too, so
   nothing more passes between them.

From then on, the ender answers that key's valid requests with a signed
`404` (except `pair` while a code is live, section 6.5). The other
server, on a signed `404` to `hello`, marks the ender `gone`. A pairing
that ended starts again only with a new code.

Entries of the forgotten list may be removed after a year. A removed
entry only turns that key's signed `404` into an unsigned one.

## 8. Common rules for every call

### 8.1 Requests

- Method `POST`, `Content-Type: application/json`, the four headers of
  section 6.1.
- The body is a JSON object, at most 64 KiB. Calls without fields send
  `{}`.
- The caller must not follow redirects.

### 8.2 Refusals

A refusal is a problem document. Overfin writes it with a trailing line
feed. The `404` refusal is, byte for byte (53 bytes, then the line feed):

```
{"type":"not-found","title":"Not found","status":404}
```

followed by `\n`. A receiver must not depend on these exact bytes: the
signature covers whatever was sent, and only the status matters to the
protocol, except on `play`'s `409` (section 9.5), where the `type`
tells two refusals apart. The `type` values used on federation calls:

| Status | `type` | `title` |
|--------|--------|---------|
| 400 | `invalid-request` | `Invalid request` |
| 404 | `not-found` | `Not found` |
| 409 | `stream-limit-reached` | `Too many streams on this account` |
| 409 | `not-playable` | `This item can't be played` |
| 410 | `playback-stopped` | `Playback was stopped` |
| 429 | `rate-limited` | `Too many requests` (section 8.3) |
| 500 | `internal-error` | `Internal error` |
| 503 | `server-busy` | `The server is busy; try again shortly` |

`404` means, on every call: "I don't hear you", "I don't know you", "not
paired yet", "there's no such code", or "there's no such title for you".
Signed or not tells them apart (section 6.6).

**Version 0 notes.** Overfin rejects some bad bodies on `pair`, `hello`
and `titles` with `404`, and on `play`, `progress` and `stop` with `400`.
The problem `title` texts are Overfin's English and may change.

### 8.3 Limits and overload

A server must protect itself from a peer that calls too much, and must
stay predictable when it is full. It must limit federation calls per
peer, and before a signature checks out per network address; limit them
all together; limit concurrent requests, per peer and in all; limit how
often a peer calls `play`, besides the pairing's `streams`; keep queues
bounded, refusing at once when one is full; and answer, never hang, when
a limit is reached. The numbers are each implementation's.

- To a known peer, after its signature checks out: a signed `429`
  `rate-limited` when that peer is over its own limit, a signed `503`
  `server-busy` when the server is full. Either may carry `retryAfter`
  (an integer, 1 to 3,600 seconds) in its body; a `Retry-After` header
  with the same value is not signed, and only the body's counts.
- Before a signature checks out: the unsigned `404` of a failed check, or
  a `429` from the server's host. For a known peer, an unsigned answer is
  no answer (section 6.6).
- A caller never retries before `retryAfter`. Without one, it backs off,
  doubling each wait from a few seconds up to an hour, with some
  randomness. A `play` refused this way isn't retried by itself: the
  viewer is told "busy now".

**Implementation note: behind a reverse proxy.** A server behind a
reverse proxy sees every request arrive from the proxy's address, so a
limit by network address would put every peer and stranger in one
bucket. Such a server counts by the client address the proxy forwards
(`X-Forwarded-For`, `Forwarded`, or the PROXY protocol), and only for a
request that came from a proxy its operator names as trusted. It takes
the address that trusted proxy added: read from the right, the first
address that isn't one of its trusted proxies, never one written further
left. It never trusts a forwarded address from any other source, since
anyone can write those headers. An IPv6 address is best counted by its
`/64`. The limits after a signature checks out are per key and need none
of this.

Core version 1 says the same, with suggested numbers
([core section 8.3](../v0/core.md#83-limits-and-overload)).

**Version 0 notes.** Overfin's numbers are in
[core section 8.3](../v0/core.md#83-limits-and-overload) ("Overfin"). Behind
its gateway, the gateway also limits requests by network address (50 a
second, bursts of 400) with an unsigned, plain-text `429` and
`Retry-After: 5`. A single Overfin counts by the address its
`identity.trusted_proxies` setting gives (`X-Forwarded-For` from those
proxies only, read from the right). Behind its gateway, the backend
counts by the visitor's address the gateway passes on, which the gateway
takes from the PROXY protocol or `X-Forwarded-For` of the proxies it is
told to trust. Since 2026-11-17 both limits count an IPv6 `/64` as one
address.

## 9. The calls

Types: *string*, *integer* (a JSON number in the form of section 2.1:
no fraction, no exponent, not `-0`, within ±(2^53 − 1)), *boolean*,
*array*, *object*. "Required" means the receiver refuses the request
without it, or with an invalid value. A member of another JSON type than
its field says is an invalid value.

That is for requests. In the answers of `libraries` and `titles`
(sections 9.3 and 9.4), a member of another JSON type than its field
says is handled this way:

- inside a list of the answer (libraries, titles, language tags, `gone`
  keys), the entry it is in is skipped alone, and the rest of the answer
  is kept; each section says which members skip their entry and which
  count as absent;
- at the top of the answer, it fails the whole answer: the caller treats
  the peer as unreachable (section 6.6) and keeps what it had. The one
  exception is `limits` in section 9.3.

A body that isn't an I-JSON object fails the answer too. `hello`'s
`certificates` (section 3.1) and `play`'s answer (section 9.5) have rules
of their own.

### 9.1 pair

`POST /api/v1/federation/pair`. The server redeeming a code calls the
code's maker. Heard from strangers only while the maker has a live code.

Request:

| Field | Type | Required | Content |
|-------|------|----------|---------|
| `address` | string | yes | the caller's own address (section 5.2) |
| `name` | string | no | the caller's name, for the maker's panel |
| `secretHash` | string | yes | base64url of SHA-256(secret): 32 bytes |
| `proof` | string | yes | the pairing proof, below |

The proof is HMAC-SHA256, keyed with the 32-byte secret, over these bytes
in order:

1. the ASCII label `overfin-federation-1 pairing\n` (it ends with a line
   feed);
2. the caller's 32 raw public-key bytes, the same key as in
   `Overfin-Key`;
3. a line feed, `\n`;
4. the `address` field's value, after JSON decoding, as UTF-8 bytes (an
   address is ASCII, and a sender has no reason to escape any of it).

It is sent base64url without padding (43 characters). The maker compares
it in constant time. The proof binds the secret to the caller's key and
address: someone who sees it go by can't reuse it with a key of their own.

Answer `200`:

| Field | Type | Content |
|-------|------|---------|
| `name` | string | the maker's name |

Refusals:

- `404`, unsigned (signed when the key is already a peer): the key isn't
  heard, there's no live code with that hash, the code expired, the proof
  is wrong, the body isn't valid JSON, the address is invalid, or the hash
  isn't 32 bytes.
- `409`: the caller's key is already a peer of the maker. The body is
  empty. It is signed, since the key is known.
- `500`: the maker failed to store the result.

The maker checks in the order of section 7.1, step 4: the code and the
proof first, then whether the key is known. A known key gets `409` only
with a valid code and proof; otherwise it gets a signed `404`. A `409`
leaves the code live.

Names, on both sides, are cleaned before they are stored, in this order:

1. every run of characters with the Unicode `White_Space` property
   becomes one space;
2. other control and format characters (Unicode Cc and Cf) are removed;
3. both ends are trimmed;
4. the result is cut to 80 code points, and trimmed again.

An empty name becomes the host of the peer's address, with its port, as
in the request message (section 6.2).

**Version 0 notes.** Overfin before 2026-11-17 removed control
characters before it joined white space, so a tab between two words
joined them; it cleans in the order above since then.

### 9.2 hello

`POST /api/v1/federation/hello`. Either peer, in any state. It tells the
other where the pairing stands, and asks where it stands there.

Request:

| Field | Type | Required | Content |
|-------|------|----------|---------|
| `state` | string | yes | the caller's view: `paired`, `waiting` or `ended` |
| `certificates` | array of string | no | the pins of the certificates the caller's address serves (section 3.1) |

Answer `200`:

| Field | Type | Content |
|-------|------|---------|
| `state` | string | the receiver's view: `paired` or `waiting`; `ended` when the request said `ended` |
| `certificates` | array of string, optional | the pins of the certificates the receiver's address serves (section 3.1) |

Overfin leaves `certificates` out when it names none (an `http` address,
or no reading yet), and when it sends or answers `ended`.

What the receiver does:

- `ended`: it deletes the caller (section 7.4), and answers `ended`.
- `paired`: if the caller is in state `theirs`, it becomes `paired`. If
  the caller is `gone` and was paired before, it becomes `paired`.
  Otherwise nothing changes.
- `waiting`, or any other string: nothing changes; the answer is as for
  `waiting`.
- A body without `state`, or with a `state` that isn't a string, is
  refused like a body that isn't JSON (signed `404`).

Then, unless the request said `ended`, it keeps the request's
`certificates` as the caller's pins (section 3.1), notes the time it heard
from the caller, and answers with its own view (section 7.3).

What the caller does with the answer:

- `200`: it keeps the answer's `certificates` as the receiver's pins.
- `200` with `paired`: as the receiver would do with `paired` above.
- signed `404`: the peer doesn't know the caller. The caller marks it
  `gone`.

Refusals: `404` (unsigned to unknown keys, signed to forgotten ones); a
signed `404` for a body that isn't JSON, lacks a string `state`, or has a
`certificates` member that isn't an array; `500`.

When a paired peer gets a signed `404` on `libraries` or `titles`, Overfin
sends a `hello` at once. The peer may have ended the pairing, or not have
heard yet that the pairing was accepted.

### 9.3 libraries

`POST /api/v1/federation/libraries`. A paired peer asks which libraries
the receiver offers it.

Request: `{}`. No fields.

Answer `200`:

| Field | Type | Content |
|-------|------|---------|
| `name` | string, optional | the receiver's name now, for the caller's menu |
| `libraries` | array of library | the libraries offered to the caller, possibly empty |
| `limits` | limits object, optional | what the caller may use of the receiver at once |

Library:

| Field | Type | Content |
|-------|------|---------|
| `id` | string | the library's ID: 1 to 64 characters of `A–Z a–z 0–9 _ -` |
| `name` | string | its name, plain text |
| `kind` | string | one of `movies`, `tvshows`, `music`, `musicvideos`, `homevideos`, `boxsets`, `books`, `photos`, `mixed`, or `""` |

A library ID is chosen by the owning server and means nothing outside
it. It must not be, contain, or be computable by a peer from, any
identifier of the media server, a file, a path or a name. A keyed MAC of
such an identifier, under a secret the server keeps, satisfies this; a
plain hash doesn't.

A library ID is stable: the same library keeps the same ID across
restarts, renames, upgrades and, where the server can tell it is the
same library, a change of media server. An ID is never reused for
another library. The ID may be the same toward every peer, or differ per
peer. A peer must not compare library IDs across peers.

Recommended practice, not required: a random value (at least 96 bits,
base64url), stored with the library when the server first offers it, and
kept with the server's other federation state. Unlike a MAC of the media
server's ID, a stored value survives a change of media server: the
operator (or an import) maps the old library to the new one.

`mixed` is for a library that holds several kinds; `""` is for kinds
outside the list. Names are text, never markup.

Limits:

| Field | Type | Range | Content |
|-------|------|-------|---------|
| `streams` | integer | 0 to 50 | playbacks at the same time |
| `transcodes` | integer | 0 to `streams` | of those, how many may be transcoded |
| `bitrateKbps` | integer | 0 to 1,000,000 | a bitrate cap in kbit/s; 0 means none |

The caller uses the limits only to avoid offering its people what the
receiver would refuse. The receiver enforces them itself (section 12). A
caller ignores limits out of range.

What the caller keeps: at most 200 libraries; entries whose `id` breaks
the form above are skipped; of two libraries with the same `id`, the
first; names cleaned as in section 9.1 (`?` when empty), and treated as
text wherever they are shown (a caller may clean further); an unknown
`kind` becomes `""`.

Members of the wrong JSON type (section 9): a `libraries` that isn't an
array, or a top-level `name` that isn't a string, fails the answer. An
entry of `libraries` that isn't an object, or whose `id` isn't a string,
is skipped. A library's `name` or `kind` that isn't a string counts as
absent (`?`, `""`). A `limits` that isn't an object, or whose members
aren't integers, is ignored, as are limits out of range.

Refusals: signed `404` when the caller isn't `paired`, or when the body
isn't a JSON object; `500`.

**Changing to opaque IDs.** An owning server whose library IDs broke the
rule above (Overfin's, below) changes them once. Its peers' stored wants
then name IDs that are no longer offered, and `titles` ignores them. There
is no mapping from old IDs to new ones: each peer's operator chooses the
libraries it wants again. The owning server treats the change as a change
of what it offers, so each peer's next `titles` reading is a whole one
(section 9.4). A caller should tell its operator when a library it wants
is no longer in a peer's offer.

**Version 0 notes.** Until 2026-11-16, Overfin's library IDs were its
media server's own library IDs, passed through, which broke the rule
above. Since then they follow the recommended practice: `lib_` and 128
random bits in base64url (26 characters), stored when a library is first
offered, and kept when the media server gives the library another ID (a
rename). Overfin changed them once, as above: every pairing's next
`titles` reading is a whole one. As a caller, it marks a wanted library
that is no longer in the peer's offer, drops its titles at the next
reading, and its panel asks the operator to choose again.

### 9.4 titles

`POST /api/v1/federation/titles`. A paired peer reads which titles the
libraries it wants hold, a page at a time. It can ask only for what
changed since its last reading.

Request:

| Field | Type | Required | Content |
|-------|------|----------|---------|
| `libraries` | array of string | yes | library IDs; those not offered to the caller are ignored |
| `after` | string | no | the page starts after this title key; `""` for the first page; at most 512 bytes |
| `since` | integer | no | the `seq` of a former reading, to get only what changed; 0 or absent for the whole list; a negative one is answered like 0 |

Answer `200`:

| Field | Type | Content |
|-------|------|---------|
| `titles` | array of title | the page; `null` or absent counts as empty |
| `next` | string, optional | where the next page starts (pass it as `after`); absent after the last page |
| `changes` | boolean, optional | `true`: the answer holds only changes since `since` |
| `gone` | array of string, optional | with `changes`: title keys no longer held |
| `seq` | integer, optional | the receiver's catalog change number now; ask `since` it next time |

Title:

| Field | Type | Content |
|-------|------|---------|
| `key` | string | the title key (section 11) |
| `kind` | string | `movie`, `series`, `season` or `episode` |
| `library` | string | one of the requested libraries that holds it: when several do, the one whose ID comes first in byte order |
| `audio` | array of string, optional | films and episodes only: its copies' audio languages, as language tags (`fr-CA`) |
| `subtitles` | array of string, optional | films and episodes only: its copies' subtitle languages, as tags |

Series and seasons carry no `audio` or `subtitles`; films and episodes
do. A language tag uses the shortest code BCP 47 allows: the ISO 639-1
code where one exists (`en`, `fr`), else ISO 639-2/T or 639-3 (`yue`),
with a region when known (`fr-CA`), written in the case of section 2.1.
`und`, `mul`, `zxx` and `mis` are not sent.

Paging and ordering:

- Titles come in ascending byte order of `key`. A page holds the titles
  with keys strictly greater than `after`.
- A page holds at most 5,000 titles. In a changes answer, `titles` and
  `gone` together hold at most 5,000 keys, taken in one byte order.
- In a whole-list answer, `next` is the last key of the page, and is set
  only when the page is full (5,000 titles).
- In a changes answer, `next` is set when there may be more changes after
  the page. A changes page may hold fewer than 5,000 titles and still
  have a `next`.
- The caller stops when `next` is absent, empty, or not greater than its
  `after`.

Whole list or changes:

- The receiver answers with changes only when `since` is above 0, it
  still keeps its change log back to `since`, `since` isn't above its
  current `seq`, and what it offers the caller hasn't changed since
  `since`. Otherwise it answers with the whole list, whatever `since` was.
- A caller asks with `since` only when `libraries` is exactly the set of
  the reading that gave that `seq`; otherwise it asks for the whole list.
- The first page decides which kind of reading it is, and its `seq` is
  the one the caller keeps. Every later page of the same reading must have
  the same `changes` value. The caller treats a mix as a failure.
- After a whole-list reading, the caller drops every title of that peer
  it didn't see in the reading's pages. After a changes reading, it keeps
  `titles` (new or changed) and drops `gone`.
- A receiver that sends no `seq` gets asked for the whole list every
  time.
- `seq` is an opaque positive counter. Only the receiver gives it a
  meaning.

What the caller keeps of each title (anything else is skipped):

- the key parses as a title key, is in canonical form (section 11), and
  its series or film is identified by TMDB;
- `kind` matches the kind the key names;
- `library` is one of the libraries it asked for;
- at most 12 language tags in each list, each 2 to 12 characters of
  `A–Z a–z 0–9 -`, each once (compared without case); a bad tag is
  skipped, not the whole title;
- `gone` keys over 128 bytes are skipped.

Members of the wrong JSON type (section 9):

- `titles` (other than `null`, which counts as empty) or `gone` that
  isn't an array, `next` that isn't a string, `changes` that isn't a
  boolean, or `seq` that isn't an integer fails the answer. Skipping
  one of them could end a reading early or mix the kinds of reading, and
  the caller would then drop titles the peer still holds.
- An entry of `titles` that isn't an object, or whose `key`, `kind` or
  `library` isn't a string, is skipped.
- An `audio` or `subtitles` member that isn't an array counts as absent;
  a tag in one that isn't a string is skipped alone.
- An entry of `gone` that isn't a string is skipped.

Overfin reads every paired peer's titles every 30 minutes, and at once
when what it wants of one changes. It gives a reading 5 minutes. It keeps
its own change log 30 days. A reader that last read before that gets the
whole list.

Refusals: signed `404` when the caller isn't `paired`, or the body isn't
valid JSON, or `libraries` is missing or isn't an array of strings, or
`after` is over 512 bytes, or `since` isn't an integer; `500`.

**Version 0 notes.** Until 2026-11-17, Overfin answered a request without
`libraries` with an empty list, and, as a caller, failed a whole answer
for one member of the wrong type inside a title (a tag that isn't a
string) or a library (a name that isn't a string).

### 9.5 play

`POST /api/v1/federation/play`. The viewer's server asks the owning
server to start a playback of a film or an episode for one viewer.

Request:

| Field | Type | Required | Content |
|-------|------|----------|---------|
| `title` | string | yes | the title key of a film or an episode, as the owning server sent it in `titles` |
| `capabilities` | capabilities object | no | what the viewer's device plays, coarsened (below) |
| `startPositionMs` | integer | no | where to start, in milliseconds; 0 to 8,640,000,000 (100 days) |
| `maxBitrateKbps` | integer | no | the viewer's bitrate cap, kbit/s; 0 to 10,000,000; 0 means none |
| `audioLanguage` | string | no | the viewer's audio language, a tag; at most 12 characters of `A–Z a–z 0–9 -` |
| `subtitleLanguage` | string | no | the viewer's subtitle language, same form |
| `audioTrack` | string | no | an audio track ID from a former answer, `a` and 1 to 3 digits |
| `subtitleTrack` | string | no | a subtitle track ID, `s` and 1 to 3 digits, or `off` |

Capabilities (all fields optional; unknown values are ignored):

| Field | Type | Values Overfin understands |
|-------|------|---------------------------|
| `containers` | array of string | `mp4`, `m4v`, `mov`, `webm`, `mkv`, `ts`, `mp3`, `aac`, `m4a`, `flac`, `ogg`, `opus`, `wav` |
| `videoCodecs` | array of string | `h264`, `hevc`, `av1`, `vp8`, `vp9` |
| `audioCodecs` | array of string | `aac`, `mp3`, `opus`, `vorbis`, `flac`, `ac3`, `eac3`, `alac`, `pcm_s16le` |
| `hdr` | array of string | `hdr10`, `hlg` |
| `maxHeight` | integer | pixels; 0 means not said |
| `maxAudioChannels` | integer | 0 means not said |
| `maxBitrateKbps` | integer | 0 to 10,000,000; 0 means not said |
| `subtitles` | array of string | none yet; send `[]` or leave it out (Overfin's viewer side leaves it out; before 2026-11-16 it sent `null`, section 2) |

How Overfin's viewer side coarsens the request, so that it can't tell one
device from another (rule 97): only known values, lowercase, each once,
sorted; `maxHeight` stepped down to 480, 720, 1080, 1440, 2160 or 4320
(below 480 becomes 480); `maxAudioChannels` stepped down to 2, 6 or 8
(below 2 becomes 2); `capabilities.maxBitrateKbps` rounded down to a
whole 1,000, and below 1,000 left out (0); the request's `maxBitrateKbps`
rounded down to a whole 1,000 from 1,000 up, and kept as it is below.
Another implementation should do the same.

What the owning server does:

1. It refuses the request (`400`) when `title` isn't a film's or an
   episode's key, or a number or tag is out of range.
2. It refuses (`409` `stream-limit-reached`) when the pairing's `streams`
   limit is 0.
3. It looks for its copies of that exact key in the libraries it offers
   the caller. None: `404`.
4. It picks the copy whose audio has `audioLanguage` (same tag first, then
   same base language), or else the first. Then it refuses (`400`) a
   malformed `audioTrack` or `subtitleTrack`.
5. It refuses (`409` `stream-limit-reached`) when the pairing's streams are
   all in use.
6. The bitrate cap is the lowest non-zero of: `maxBitrateKbps`,
   `capabilities.maxBitrateKbps`, the owning server's own cap, and the
   pairing's `bitrateKbps`. With none, 120,000 kbit/s.
7. It starts the playback within the pairing's limits and the server's
   transcode budget (section 12). Over them: `409` `stream-limit-reached`,
   or `503` `server-busy`. When its media server offers no way to play
   the copy: `409` `not-playable`.
8. It answers `200`, signed, with the playback.

Answer `200`: the playback object. Overfin's owning side sends these
fields. Overfin's viewer side checks and cleans every one, as listed.

| Field | Type | Content | What the viewer's server requires or does |
|-------|------|---------|-------------------------------------------|
| `id` | string | the playback ID | must match `^play_[a-z2-7]{26}$` |
| `method` | string | `direct`, `remux` or `transcode` | anything else: refused |
| `stream` | object | `url`, `protocol`, `mimeType` (optional) | section 10 |
| `positionMs` | integer | the start position | |
| `durationMs` | integer, optional | the copy's length | outside 0 to 100 days: set to 0 |
| `versions` | array, optional | `{id, name}` of the title's versions | IDs as below; names made plain text, 64 characters |
| `versionId` | string, optional | the version playing, `ver_…` | IDs as below |
| `audioTracks` | array of track, optional | the audio tracks | IDs as below |
| `audioTrack` | string, optional | the one playing, `a<n>` | |
| `subtitleTracks` | array of track, optional | the subtitle tracks | `url` checked (section 10) |
| `subtitleTrack` | string, optional | the one to show, `s<n>` | |
| `forcedTracks` | array of track, optional | forced subtitles | `url` checked |
| `forcedTrack` | string, optional | the forced one matching the audio | |
| `subtitleStretchSeconds` | integer, optional | 120 when there are subtitles | replaced by its own value, 120 |
| `maxBitrateKbps` | integer, optional | the cap in effect | |
| `segments` | array, optional | skip markers `{kind, startMs, endMs}` | kinds `intro`, `outro`, `recap`, `preview`, `commercial`; within 100 days; at most 32 kept |
| `trickplay` | trickplay object, optional | the seek-bar previews of the copy playing (below) | sheet URLs checked (section 10); out of bounds: dropped |
| `info` | object, optional | how it's served: `reasons`, `container`, `bitrateKbps`, `video`, `audio` | text made plain, short |

Track:

| Field | Type | Content |
|-------|------|---------|
| `id` | string | `a<n>` for audio, `s<n>` for subtitles |
| `label` | string | the language's name in itself |
| `language` | string, optional | a language tag |
| `channels` | integer, optional | audio channels |
| `role` | string, optional | `commentary` or `description` |
| `default`, `forced`, `hearingImpaired` | boolean, optional | flags |
| `pictures` | boolean, optional | the subtitles are pictures (PGS) |
| `url` | string, optional | subtitles only: where to fetch them (section 10) |

Trickplay (seek-bar previews): thumbnails of the video every
`intervalMs`, laid out in a grid on JPEG images called sheets. Each
sheet holds `columns` × `rows` tiles, each `tileWidth` × `tileHeight`
pixels, read left to right, then top to bottom. Tile *t* (from 0) shows
the video at *t* × `intervalMs`, and is tile *t* mod (`columns` ×
`rows`) of sheet ⌊*t* / (`columns` × `rows`)⌋. The last sheet may hold
fewer tiles, and may be shorter or narrower than a full sheet.

| Field | Type | Content | Bounds |
|-------|------|---------|--------|
| `tileWidth` | integer | a tile's width, pixels | 1 to 2,048 |
| `tileHeight` | integer | a tile's height, pixels | 1 to 2,048 |
| `columns` | integer | tiles across a sheet | 1 to 100 |
| `rows` | integer | tiles down a sheet | 1 to 100 |
| `intervalMs` | integer | milliseconds between two tiles | 100 to 3,600,000 |
| `tileCount` | integer | tiles in all | 1 or more |
| `sheets` | array of string | sheet *n*'s URL at index *n* (section 10.1) | exactly ⌈`tileCount` / (`columns` × `rows`)⌉ entries, at most 1,000 |

Besides those bounds, a full sheet is at most 4,096 pixels each way:
`columns` × `tileWidth` ≤ 4,096 and `rows` × `tileHeight` ≤ 4,096.

- The owning server sends `trickplay` only when it has previews for the
  copy playing, within these bounds. It sends one set, not one per
  width: Overfin sends the set nearest 320 pixels wide. Without previews,
  it leaves the field out (a receiver treats `null` as absent, section 2).
- The viewer's server drops `trickplay`, and plays on without previews,
  when a number is missing (read as 0) or out of bounds, or when `sheets`
  has another number of entries. It refuses the whole answer (it sends
  `stop` and gives up, as below) when a sheet URL isn't the one section
  10.1 gives for its index. A field of another JSON type (a string for a
  number, a fraction) makes the answer one Overfin's viewer side can't
  read, so it refuses the playback, as for any other field.
- Both sides may leave previews out: an owning server need not send
  `trickplay`, and a viewer's server that offers no previews ignores it.
  An answer without it (from an Overfin older than this section, say)
  just means a playback without previews.
- The viewer's server never hands the owning server's URLs to the
  viewer: it serves each sheet at an address of its own, fetched as
  section 10.3 says.

IDs of tracks and versions must match `^[A-Za-z0-9_-]{1,64}$` (or be
`off`). Version IDs (`versionId`, `versions[].id`) follow the rule of
library IDs (section 9.3): opaque, chosen by the owning server, not
derived from its media server's IDs without a key. They need to be stable
only for the life of a title's copies, since a viewer's server passes
them back only in a later `play` of the same title. Track IDs (`a<n>`,
`s<n>`) are positions, not identifiers, and are not concerned. The viewer's server drops tracks with another ID or an empty
label, and clears other IDs. It drops `title` and `next` whatever they
hold. When the answer breaks a "must" above, the viewer's
server sends `stop` for that `id` and gives up.

To change the audio track or the quality, the viewer's server stops the
playback and starts a new one with `play`. It waits up to 5 seconds for
the `stop` to be answered first, so the old playback doesn't count
against the limit.

Refusals: `400`, `404`, `409`, `503`, `500` as above. A `409` is one of
two refusals, told apart by the problem `type` of its signed body (section
8.2):

| Status | `type` | Meaning | Overfin's viewer side tells the viewer |
|--------|--------|---------|------------------------------------------|
| 409 | `stream-limit-reached` | the pairing's streams are all in use, or its limit is 0 | busy now |
| 409 | `not-playable` | the owning server has no way to play its copy (the media server offers none) | not playable |
| 503 | `server-busy` | the server's transcodes are all in use | busy now |

The owning server must send `not-playable` with `409` only for a title it
can't play, and `stream-limit-reached` for its limits. A viewer's server
must read the `type` of a signed `409`: `not-playable` means not playable;
any other `type`, or a body without one, means busy now. A body that
isn't JSON counts as a body without a `type`. Any other refusal (`400`,
`404`, `500`, an unsigned one) means not playable.

**Version 0 notes.** Until 2026-11-16, Overfin's owning side sent `ver_`
followed by its media server's media source ID as version IDs, which
broke the rule above. Since then it sends `ver_` and a keyed MAC of that
ID, per peer (128 bits, base64url): stable, and opaque to the peer.
Version 0's `play` takes no version, so nothing maps them back. A
well-formed `audioTrack` or `subtitleTrack` that isn't a track of the
copy chosen counts as absent, as
[playback section 2.6](../v0/playback.md#26-choosing-the-copy-and-the-tracks)
says; before 2026-11-16, Overfin passed the audio track to its media
server unchecked.

### 9.6 progress

`POST /api/v1/federation/progress`. The viewer's server tells the owning
server that a playback is still watched, and where.

Request:

| Field | Type | Required | Content |
|-------|------|----------|---------|
| `id` | string | yes | the playback ID from `play`, `^play_[a-z2-7]{26}$` |
| `positionMs` | integer | no | where the viewer is; 0 to 8,640,000,000 |
| `paused` | boolean | no | whether the viewer paused |

Answer `200`: `{}`.

Refusals: `400` for a malformed body; `410` `playback-stopped` when the
playback isn't live, or isn't this pairing's.

Overfin's viewer side sends `progress` at most every 45 seconds while the
viewer watches, and at once when the viewer pauses or resumes. The owning
server ends a playback that has neither a progress report nor a media
request for 2 minutes (section 10.4). An open media response held by a
paused player counts as use.

### 9.7 stop

`POST /api/v1/federation/stop`. The viewer's server tells the owning
server a playback is over.

Request:

| Field | Type | Required | Content |
|-------|------|----------|---------|
| `id` | string | yes | the playback ID, `^play_[a-z2-7]{26}$` |
| `positionMs` | integer | no | where the viewer stopped; 0 to 8,640,000,000 |

Answer `200`: `{}`, whether or not the playback was live. A playback ID of
another pairing changes nothing.

Refusals: `400` for a malformed body, a position out of range included.

## 10. Fetching the media

Over `https`, every media fetch needs a *trusted* or *named* certificate
(section 3.1), never just any: the ticket and the media have only TLS to
protect them.

### 10.1 The URLs

The stream URL in a `play` answer is absolute, on the owning server's
address, exactly as that address appears in its code or pair request:

```
<address>/api/v1/stream/<ticket>/<name>
```

- `<ticket>` matches `^[A-Za-z0-9_-]{1,64}$`. It is the only credential:
  a bearer capability. The owning server makes each ticket from at least
  128 bits of a cryptographically secure random source, never from a
  counter, a time or any other value a peer could predict. A ticket is
  never reused, for another playback or after its own has ended. The
  owning server compares tickets in a way whose time doesn't depend on
  how much of a ticket matches (a lookup by a hash of the ticket is
  enough), and neither side writes tickets into logs, diagnostics or
  error messages.
- `stream.protocol` is `hls`, with `<name>` `master.m3u8` and
  `stream.mimeType` `application/vnd.apple.mpegurl`; or `file`, with
  `<name>` `file` (the file as it is, served with byte ranges).
- Subtitle and forced tracks carry a `url` of the form
  `<address>/api/v1/stream/<ticket>/subtitles/<track>` for text, or
  `.../pictures/<track>` for PGS, where `<track>` is the track ID.
- `trickplay.sheets[n]` is exactly
  `<address>/api/v1/stream/<ticket>/trickplay/<n>`, with the stream's
  own `<ticket>` and `<n>` the entry's index in decimal, without leading
  zeros: `sheets[0]` ends in `/trickplay/0`, `sheets[1]` in
  `/trickplay/1`, and so on.

Overfin's viewer side refuses any URL that doesn't start with the owning
server's address followed by `/api/v1/stream/`, or that names another
ticket, or another file shape. `<name>` and `<track>` match
`^[A-Za-z0-9._-]{1,64}$` and are never `.` or `..`, which a URL resolves
as folders, not files. The check applies to the URL's path split at `/`,
compared exactly as sent, nothing decoded or resolved first.

### 10.2 Requests

The viewer's server fetches with plain `GET`:

- No signature headers. No cookies. No query string: the owning server
  answers `404` to any query.
- No `Origin`, `Sec-Fetch-Site` or `Sec-Fetch-Mode` header. The owning
  server answers `404` to a peer's ticket when any of them is present,
  because only a browser sends them, and a browser must never use a
  peer's ticket (rule 46).
- `Range: bytes=<first>-<last>` is allowed on media (the file and
  segments), with at most 15 digits on each side. The answer is `200` or
  `206`, with `Content-Length`, `Content-Range` and `Accept-Ranges` as
  usual.
- Redirects must not be followed.

### 10.3 What is fetched

- **HLS.** Fetch `master.m3u8`. Every URI in it, and in the playlists it
  names, is a bare file name of the same stream (it matches
  `^[A-Za-z0-9._-]{1,64}$`, and is not `.` or `..`), relative to
  `<address>/api/v1/stream/<ticket>/`. Fetch each the same way. Overfin's
  viewer side refuses a playlist that doesn't start with `#EXTM3U`, or that
  names anything else, including an unquoted `URI=` attribute. Playlists
  are at most 2 MiB.
- **File.** Fetch `file`, with ranges.
- **Text subtitles.** Fetch `<track url>/<part>`, where `<part>` is a
  decimal integer from 0 to 9,999 without leading zeros. Part *n* covers
  the video from *n* × 120 to (*n* + 1) × 120 seconds, and starts 10
  seconds early, so a line already on screen comes along; drop the
  doubles. The answer is WebVTT, at most 5 MiB.
- **PGS subtitles.** Fetch `<track url>/<part>` the same way. The answer
  is the stretch's PGS data, `application/octet-stream`.
- **Preview sheets.** Fetch `trickplay.sheets[n]`, the same plain `GET`
  with the playback's ticket, no `Range`. The owning server answers
  `404` for a sheet number at or past the sheet count, one with leading
  zeros, a query string, or a ticket not live (section 10.4). The answer
  is `200`, `Content-Type: image/jpeg`, a JPEG of at most 8 MiB, no wider
  than `columns` × `tileWidth` and no taller than `rows` × `tileHeight`
  pixels. Overfin's owning side re-encodes every sheet before sending
  it. A viewer's server may fetch the sheets when the playback starts or
  when the viewer needs them; the fetches count as media requests for
  section 10.4.

Overfin's viewer side checks again everything it relays: WebVTT
sanitized, PGS checked whole, playlists as above (rules 14 and 17). It
reads a preview sheet whole, refusing it past 8 MiB, then checks from its
header, before decoding it, that it is a JPEG (by its content, whatever
its `Content-Type`) within the size above; it refuses any other. It
re-encodes the rest as JPEG, so that only their pixels reach the viewer
(rule 16), and serves them at its own playback's route, to the viewer
playing it only.

### 10.4 How long a ticket works

A ticket works while its playback is live. The playback ends, and the
ticket with it, when:

- the viewer's server sends `stop`;
- neither `progress` nor a media request came for 2 minutes;
- the pairing ends, or the caller is no longer `paired`;
- the library holding the copy is no longer offered to that peer.

A ticket works only for the pairing it was given to, and never for a
browser.

## 11. How titles are identified

Titles are named by TMDB IDs, never by files, paths or the media server's
item IDs (rule 96). A title key is one of:

| Kind | Key | Example |
|------|-----|---------|
| film | `movie/tmdb/<id>` | `movie/tmdb/949` |
| series | `series/tmdb/<id>` | `series/tmdb/1399` |
| season | `series/tmdb/<id>/s<season>` | `series/tmdb/1399/s1` |
| episode, by TMDB episode ID | `series/tmdb/<id>/e<episode id>` | `series/tmdb/1399/e63056` |
| episode, by numbers | `series/tmdb/<id>/s<season>/e<episode>` | `series/tmdb/1399/s1/e2` |

- `<id>` is the TMDB ID of the film or the series, in decimal, without
  leading zeros: 1 to 9,999,999,999.
- `<season>` is 1 to 4 digits; season 0 holds specials. `<episode>` is 1
  to 5 digits. `<episode id>` is TMDB's ID of the episode, 1 to 10 digits,
  above 0.
- Keys are canonical: no leading zeros anywhere (`s1`, not `s01`). A key
  in another form is a different string, and the receiver skips it.
- An episode TMDB gives an ID for uses the `e<episode id>` form. One TMDB
  gives no ID for uses the numbers.
- A season is named by its series' ID and its number, never by TMDB's
  season ID.
- Season and episode numbers follow the owning server's order of the
  series.

The `kind` field of a title is `movie`, `series`, `season` or `episode`,
the kind its key names. `play` takes only films and episodes.

**Version 0 notes.** Overfin's keys also have `imdb` and `tvdb` sources,
and `item/<32 hex digits>` for files nobody identified. They are not part
of the protocol: an owning server must not send them, and a receiver
skips them.

## 12. What the owning server enforces

The owning server decides alone. The viewer's server can't widen anything.

- **What is shared.** Only titles in libraries it offers that peer, and
  only those the peer asked for. `titles` ignores other library IDs.
  `play` finds copies only in offered libraries. A server offers only its
  own libraries, never what it got from another peer (rule 98).
- **Stream caps.** Per pairing: at most `streams` playbacks at once
  (0 refuses every `play`), and at most `transcodes` of them transcoded.
  Across the server: its transcode budget, less the transcodes it keeps
  for its own people. A remux found to re-encode the video counts as a
  transcode from then on. The bitrate is the lowest cap of section 9.5,
  step 6.
- **Defaults.** Overfin starts each pairing at 2 streams, 1 transcode and
  no bitrate cap. The operator changes them.
- **Media.** Only through the ticket, only from a client that doesn't
  look like a browser, only while the playback is live (section 10.4).
- **What a peer never learns** (rule 97):
  - nothing about the viewer: no account, profile, name, email, address,
    IP, browser or device. Nothing in `play`, `progress` or `stop` names a
    person.
  - not the viewer's exact device: capabilities and bitrates are
    coarsened (section 9.5).
  - not the viewer's server's internals beyond its gateway: Overfin's
    backend reaches peers through its gateway, so a peer sees the
    gateway's address only (rule 132).
  - of the owning server: never files, paths, or its media server's item
    IDs, not even inside library or version IDs (sections 9.3 and 9.5).

  What a peer does learn: the title, the start position, the language
  tags wanted, the coarsened capabilities, and progress over time, for
  one unnamed viewer of the other server.

## 13. Versioning

This document defines **version 0** of the protocol. Version 0 has no
version field. A server speaks version 0 when it does what this document
says.

The strings `overfin-federation-1` and `overfin-pair-1.` contain a `1`.
They are fixed literals of version 0, chosen before this document. They
are not the protocol's version number.

The corrections of 2026-11-16 (section 17) didn't make a new version:
they change nothing an honest version 0 sender writes.

Version 1 is announced in `hello`, through optional members a version 0
server ignores, and uses new labels, headers and a new code prefix, so
that a version 0 server refuses its messages instead of misreading them.
A version 0 peer, which sends none of those members, is spoken to in
version 0. How the two coexist is in
[spec/v1/core.md](../v0/core.md); the
versioning policy is in [spec/v1/README.md](../v0/README.md#versioning).
(Earlier drafts of this section recommended a single `protocol`
integer; version 1 replaced it with a core version plus a version per
part.)

## 14. Security considerations

- **Trust is the key, nothing else.** A server is its public key. A name,
  address or certificate never identifies it. The pairing code carries
  the maker's key; the operators' fingerprint check catches a code
  changed on the way. Skipping that check leaves only the secrecy of the
  channel the code travelled on.
- **The code is a secret.** Whoever holds a live code can pair with its
  maker, until the maker's operator refuses them. It works once, for 24
  hours. The secret never travels: only its hash and an HMAC bound to the
  redeemer's key and address.
- **Consent on both sides.** The redeemer's operator consents by pasting
  the code. The maker's operator accepts after the fingerprint check.
  Neither server can move the other to `paired`.
- **Replay.** The nonce, the 5-minute window and the restart rule
  (section 6.5) stop replays. A request names its receiver's host, so it
  can't be sent to another server. Answers name the request's nonce, so an
  answer can't be swapped for another.
- **Plain HTTP.** Over `http`, an observer reads titles, names and
  tickets, and can fetch media with a ticket it read while the playback
  lives. An on-path attacker can also change media bytes: media is not
  signed (only playlists' addresses are checked). It can't forge any call
  or answer. Use HTTPS where possible. The scheme is part of the address
  in the code and the pair request, so an attacker can't move a pairing
  from `https` to `http`.
- **Certificates (section 3.1).** An attacker on the path who presents a
  certificate of its own sees `pair` and `hello` only: it can't change
  them, since both ways are signed, nor have its certificate named, since
  pins come only in signed `hello`s and a `hello` can't be replayed
  (section 6.5). Every other call, and the media, refuses its
  certificate. Removing pins needs the peer's key, and only makes calls
  fail: pins never refuse a trusted certificate. What stays open: a
  certificate a public authority wrongly issued passes, as for any site;
  and a server that reads its own certificate at its own address (as
  Overfin does) names whatever it is served there, so someone who can
  redirect that connection (the server's DNS, say) can have their
  certificate named. Pins set by the operator avoid that. A peer's key,
  if stolen, can name any certificate; it can already do more.
- **Unsigned refusals.** Strangers get unsigned refusals. An attacker on
  the path can fake those, which spoils one pairing attempt. A known
  peer's refusals are signed, and unsigned ones change nothing.
- **Probing.** A server should answer unknown keys with the same `404` as
  a missing path (section 3), so nobody learns that federation is on
  without a code. Unsigned refusals don't tell why.
- **Split-horizon names.** A server that is served another certificate
  inside its network than outside, and reads its own certificate (section
  3.1), names the inside one, and its peers are refused on every call
  that needs *trusted* or *named*. Nothing is opened to an attacker, but
  calls fail; such a server sets its pins by hand.
- **A compromised peer** can't reach the other's people or data beyond
  what was shared (rule 100). It can still make the other store a long
  title list, and fetch those titles' details from TMDB. Readers should
  cap what they keep.
- **Hostile answers.** Treat everything a peer sends as untrusted input:
  clean text, check URLs against the peer's address, check playlists and
  subtitles before relaying (rules 14, 15, 17).
- **Preview sheets are images from another server**, the only ones a
  peer sends. Check a sheet's byte size first, then its dimensions from
  its header against the layout (section 10.3), and only then decode
  it. Re-encode it,
  so nothing but pixels (no metadata, no trailing bytes, no second image)
  reaches a browser. The layout's bounds keep the number of fetches, and
  the memory a sheet takes, small: at most 1,000 sheets of 4,096 × 4,096
  pixels. A viewer's server should also bound how many sheets it reads
  at once (Overfin: 4). Never give the browser the peer's sheet URLs:
  they carry the ticket, and would let the peer see the viewer
  (rule 97).
- **Clocks.** Keep clocks set by the network. A clock off by more than 5
  minutes stops all calls. A clock behind delays calls after the other's
  restarts.
- **Peers' addresses** point where the peer chooses: section 3.2 keeps
  connections out of the server's own machine and its cloud's metadata
  service.
- **Logging.** Never write into logs, diagnostics or error messages a
  ticket or a URL that holds one, a pairing code or its secret, the
  secret's hash or the proof, or a private key or its seed. HTTP clients'
  error messages often quote the URL: remove it first. Public keys,
  fingerprints, addresses, playback IDs, nonces and statuses may be
  logged ([core section 14.1](../v0/core.md#141-logging)).

## 15. Test vectors

These values come from Overfin's own code. They are in
[vectors/v0.json](../../vectors/history/test-draft.json), for an implementation's tests to
check against. Overfin's test `internal/federation/vectors_test.go`
recomputes them from its copy of that file, so its code can't drift from
them. The corrections of 2026-11-16 left every value in that file valid
and unchanged.

The cases of section 2.1 and section 5, valid and invalid (an identity
key, a non-canonical key, a torsion R, S + L, base64url with nonzero
unused bits, duplicate JSON members, addresses and codes with their
verdicts), are in [vectors/v0-strict.json](../../vectors/history/test-draft-strict.json).
That file comes from the generator in
[tools/vectors/](../../tools/vectors/), not from Overfin's code, which
doesn't follow section 2.1 yet. The generator also checks every value of
v0.json.

### 15.1 Keys

Seeds are the 32-byte Ed25519 seeds of RFC 8032, in hex.

| | Key "a" (server A, `https://a.example.org`) | Key "b" (server B, `http://192.0.2.10:8096`) |
|---|---|---|
| seed | `000102030405060708090a0b0c0d0e0f101112131415161718191a1b1c1d1e1f` | `202122232425262728292a2b2c2d2e2f303132333435363738393a3b3c3d3e3f` |
| public key | `A6EHv_POEL4dcN0Y50vAmWfk1jCbpQ1fHdyGZBJVMbg` | `Kay64UG8yvCyLhqU000LxzYeUm0L_hLIl5S8kyKWbdc` |
| fingerprint | `KZDV-VJ2U-MNDU-YAUF-35O3-6K6K` | `ET3O-22WL-7YIA-TQBQ-27FF-M7BT` |

### 15.2 Pairing code, secret hash and proof

Made by key "a" at `https://a.example.org`, with the secret
`404142434445464748494a4b4c4d4e4f505152535455565758595a5b5c5d5e5f` (hex),
expiring at `1800086400`:

```
overfin-pair-1.A6EHv_POEL4dcN0Y50vAmWfk1jCbpQ1fHdyGZBJVMbhAQUJDREVGR0hJSktMTU5PUFFSU1RVVldYWVpbXF1eXwAAAABrSyOAaHR0cHM6Ly9hLmV4YW1wbGUub3Jn
```

- `secretHash`, base64url of SHA-256(secret):
  `yipP5yf6rs8W7NEwqG4IhcVUDAU3U0BEUHHAZXVV_UI`
- `proof`, for redeemer key "b" and address `http://192.0.2.10:8096`:
  `obsMG8Ji37jKVh42gDYkxlPaeZeGHJXxhbsJwKaVw8Q`

### 15.3 pair: B calls A

B signs at `1800000000`, with the nonce bytes
`606162636465666768696a6b6c6d6e6f` (hex).

Request line and headers:

```
POST /api/v1/federation/pair HTTP/1.1
Host: a.example.org
Content-Type: application/json
Overfin-Key: Kay64UG8yvCyLhqU000LxzYeUm0L_hLIl5S8kyKWbdc
Overfin-Date: 1800000000
Overfin-Nonce: YGFiY2RlZmdoaWprbG1ubw
Overfin-Signature: MOVQn_vjzkLlCIpsffd98F7tAJAwRWBuHRQamLuBouLXSCDSzBv85pBG0pr1iNrdOhsrrDLWLLKrlmnqo8bwCQ
```

Body (exact bytes, no trailing line feed):

```
{"address":"http://192.0.2.10:8096","name":"Server B","secretHash":"yipP5yf6rs8W7NEwqG4IhcVUDAU3U0BEUHHAZXVV_UI","proof":"obsMG8Ji37jKVh42gDYkxlPaeZeGHJXxhbsJwKaVw8Q"}
```

Signed message (seven lines, no trailing line feed):

```
overfin-federation-1 request
POST
a.example.org
/api/v1/federation/pair
1800000000
YGFiY2RlZmdoaWprbG1ubw
5a9a1ecab0e914eb75751489b2ac3e94d13cae33d8e67adc3297d7b6bb12aba1
```

A answers `200` with the body `{"name":"Server A"}`. The signed message:

```
overfin-federation-1 response
YGFiY2RlZmdoaWprbG1ubw
200
2afbea032b5472391befb6b5dd9298e4a521b3d7f2728511f3b4509b49b5f845
```

and the header, signed with key "a":

```
Overfin-Signature: OPY68iHZpjVV7CLoEmcfJqgmkOlgzEXL_JS6B2DSN-5IbmyHuMHFa4g2276TGzXQisyniu4zP6DNRtS9hX_IBQ
```

### 15.4 hello: A tells B it accepted

A signs at `1800000060`, with the nonce bytes
`707172737475767778797a7b7c7d7e7f`. The receiver's host is
`192.0.2.10:8096`, port included.

```
POST /api/v1/federation/hello HTTP/1.1
Host: 192.0.2.10:8096
Content-Type: application/json
Overfin-Key: A6EHv_POEL4dcN0Y50vAmWfk1jCbpQ1fHdyGZBJVMbg
Overfin-Date: 1800000060
Overfin-Nonce: cHFyc3R1dnd4eXp7fH1-fw
Overfin-Signature: T-ix6qVaVORxm_Gfuf2YqF2Z33N-hNWFCEGJwiI-3y02xQ8nz7aFCh8dHpAp6kelFp0peFuhR8HNwtSo2DROBQ
```

Body: `{"state":"paired"}`. Signed message:

```
overfin-federation-1 request
POST
192.0.2.10:8096
/api/v1/federation/hello
1800000060
cHFyc3R1dnd4eXp7fH1-fw
9ccf770945b4c39d01e3dc5a2916894d1f74675d3d0631fcc3a3bb995b77cd44
```

B answers `200` with the body `{"state":"paired"}`. Signed message:

```
overfin-federation-1 response
cHFyc3R1dnd4eXp7fH1-fw
200
9ccf770945b4c39d01e3dc5a2916894d1f74675d3d0631fcc3a3bb995b77cd44
```

```
Overfin-Signature: F5KfP2arebDbd13KZQfJjKlAfWi3jBudGpaYMEKzeMdlRpA0e8GgzQcY6v_Efe3tIf4s6x2OahAOoNUYVcJUDQ
```

### 15.5 A signed refusal

Later, B has ended the pairing. A sends the same body at `1800000120`,
with the nonce bytes `808182838485868788898a8b8c8d8e8f`:

```
Overfin-Key: A6EHv_POEL4dcN0Y50vAmWfk1jCbpQ1fHdyGZBJVMbg
Overfin-Date: 1800000120
Overfin-Nonce: gIGCg4SFhoeIiYqLjI2Ojw
Overfin-Signature: 0jmZaPz1bAMd0r0AXGTFVq_xaIPYZm8pyH-LSIS8-5NQBsz9uVjDCrUFCYWCM-Eo6i3AkS1NaVP9KEkvZ2wWCw
```

Signed message:

```
overfin-federation-1 request
POST
192.0.2.10:8096
/api/v1/federation/hello
1800000120
gIGCg4SFhoeIiYqLjI2Ojw
9ccf770945b4c39d01e3dc5a2916894d1f74675d3d0631fcc3a3bb995b77cd44
```

B answers `404`, `application/problem+json`, with the body of section
8.2, its trailing line feed included:

```
{"type":"not-found","title":"Not found","status":404}
```

Signed message, and the signature with key "b":

```
overfin-federation-1 response
gIGCg4SFhoeIiYqLjI2Ojw
404
539cd89e39038897c94385bb427be55a490434cd5f352e677e8a3e4d29669bb5
```

```
Overfin-Signature: rdxo9iPBY4ZEuW9UPOANk6hRC-YsP8q-OnJ6_pGP2LX9RVSvcXhQUpuOa4-kF6Zu8N0SjBX1b2sGRmDBY7RYDg
```

## 16. Open questions

These are places where version 0 is loose, or where a rule may not be
the right one for every implementation. None is decided. Where a
proposal of 2026-11-16 left a point open, the text above follows the
proposal's own default, and the point is listed here.

1. **Paging.** A whole-list reading whose size is a multiple of 5,000 ends
   with one empty page.
2. **Media isn't signed.** Only the ticket and TLS protect what
   `/api/v1/stream/` serves. Over plain HTTP, media can be changed on the
   way. (Version 1 allows HTTPS only.)
3. **Answer headers aren't signed**, `Content-Type` included. (Core
   version 1 signs the answer's media type.)
4. **Episode numbering.** Seasons and episodes follow the owning server's
   order. Two servers that number a series differently disagree on
   `s<n>/e<m>` keys. The `e<episode id>` form avoids it where TMDB gives
   an ID. (Version 1's `catalog` part uses TMDB's numbering.)
5. **Rate limits.** Closed by the review round of 2026-11-16: section 8.3
   requires them.
6. **Point checks in other languages** (proposal 1, open point 2).
   PyNaCl's `crypto_core_ed25519_is_valid_point` and `@noble/curves`
   (`isTorsionFree`) are believed to cover section 2.1's point checks.
   This was not verified.
7. **A problem type for non-canonical input** (proposal 1, open point 3).
   Should a refusal for a non-canonical form, after the signature checked
   out, carry a specific problem `type` (`invalid-request`) to help
   implementers debug? Section 2.1 follows the proposal's text: the
   status that call gives a bad body, with that call's usual `type`.
8. **How often a library ID may change** (proposal 2, open point 2).
   Section 9.3 says "stable" and "never reused"; a cap (for example "only
   when the library is removed") would be hard to check.
9. **One rule for every identifier** (proposal 2, open point 3). Rather
   than a rule for library IDs (section 9.3) and one for version IDs
   (section 9.5), section 12 could state once that every identifier an
   owning server sends is opaque, and list them.

## 17. Changes

**2026-11-16: corrections (errata) to version 0.** They don't make a new
version: an honest version 0 sender already writes what they require, and
every value of [vectors/v0.json](../../vectors/history/test-draft.json) stays valid. The
proposals and clarifications are in [proposals/](../../proposals/README.md).

| Correction | What changed | Sections |
|------------|--------------|----------|
| Proposal 1, with the check on R required | One rule set for Ed25519 keys and signatures (canonical points of the prime-order subgroup); strict base64url, hex and decimals; I-JSON bodies; one instance of each signature header; one case per value; a grammar for addresses; the code reader's rules; `null` no longer sent; integers within 2^53 − 1 | 2, 2.1 (new), 3, 4.1, 5.1, 5.2, 6.4, 9, 9.1, 9.5; open question 7 closed |
| Proposal 2, migration by choosing again (no `formerId`) | Library and version IDs opaque, chosen by the owning server; which library a title names; operators choose their libraries again after the change | 9.3, 9.4, 9.5, 12; open question 2 closed |
| Clarification 1 | The target is taken as received; a query is refused | 6.2 |
| Clarification 2 | A nonce's 10 minutes start when the request is accepted | 6.5 |
| Clarification 3 | Forgotten keys are heard on `pair` while a code is live; a successful `pair` clears them | 6.5, 7.1 |
| Clarification 4 | Retry once after an unsigned `404` | 6.5; open question 6 closed |
| Clarification 5 | `415` and `403` moved to a version 0 note | 3 |
| Clarification 6 | The body of an unsigned `404` | 3, 6.4, 14 |
| Clarification 7 | A `409` body that isn't JSON | 9.5 |
| Clarification 8 | Removing a `gone` peer is an ending; redeeming a code clears the forgotten list | 7.1, 7.4 |
| Clarification 9 | Forgotten entries may go after a year | 7.4 |
| Clarification 10 | What the receiver of `ended` deletes and keeps | 7.4 |
| Clarification 11 | What "last heard from" counts | 7.2 |
| Clarification 12 | The order of `pair`'s checks | 9.1 |
| Clarification 13 | How names are cleaned, step by step | 9.1 |
| Clarification 14 | Library names are text | 9.3 |
| Clarification 15 | A `hello` without a string `state` | 9.2; open question 10 closed |
| Clarification 16 | `stop`'s position capped at 100 days | 9.7; open question 1 closed |
| Clarification 17 | Duplicate library IDs, `mixed`, a bad `libraries` body | 9.3 |
| Clarification 18 | `since` only for the same set of libraries | 9.4 |
| Clarification 19 | `gone` counts toward the 5,000 | 9.4 |
| Clarification 20 | A `since` that isn't an integer, or is negative | 9.4 |
| Clarification 21 | The form of language tags | 9.4 |
| Clarification 22 | No languages on series and seasons | 9.4 |
| Clarification 23 | Seasons by number, never by TMDB's season ID | 11 |
| Clarifications 24 to 33 | Several certificates, SNI, a failed reading, a `certificates` member that isn't an array, which 8 pins, the latest `hello`, pins compared as sets, connections under removed pins, pins after a signed `404`, one `hello` at a time | 3.1 |
| Clarification 34 | Split-horizon names | 14 |
| Review, 2026-11-16 | Media names are never `.` or `..`, checked on the path's parts as sent; tickets carry at least 128 random bits, are never reused, and never logged | 10.1, 10.3 |
| Review round, 2026-11-16 | Connecting to peers: loopback, link-local (cloud metadata), unspecified, multicast and reserved addresses refused by default, private ranges allowed, an operator policy within bounds; one lookup per connection, every address checked, the checked one dialed; IPv4-mapped addresses judged as IPv4 | 3.2 (new), 14 |
| Review round, 2026-11-16 | Limits and overload required: per peer, global, concurrent requests, `play`, bounded queues; signed `429` `rate-limited` or `503` `server-busy` with `retryAfter`; how a caller backs off | 8.2, 8.3 (new); open question 5 closed |
| Review round, 2026-11-16 | What is never logged | 14 |

Test cases for proposal 1's rules are in
[vectors/v0-strict.json](../../vectors/history/test-draft-strict.json) (section 15).

**2026-11-17: corrections from the first live interop**, between Overfin
and an independent implementation (a Jellyfin plugin built from this
text). Like those of 2026-11-16, they make no new version, and every
value of [vectors/v0.json](../../vectors/history/test-draft.json) stays valid.

| Correction | What changed | Sections |
|------------|--------------|----------|
| Content codings | Federation requests and answers carry no `Content-Encoding`; signatures cover the bytes as sent; a receiver answers uncoded whatever `Accept-Encoding` says; a caller refuses a coded answer like a badly signed one | 3, 6.2, 6.3, 6.6 |
| Wrong JSON types in answers | In `libraries` and `titles` answers, a member of the wrong type inside a list skips its entry (or counts as absent, as each section says); at the top of the answer it fails the answer, `limits` excepted | 9, 9.3, 9.4 |
| `libraries` in a `titles` request | Its refusals name a missing `libraries`, or one that isn't an array of strings, as the table's "required" already said | 9.4 |
| Renewals right after a `hello` | A `hello`'s answer names the certificates served when it answers: a server that reads its own certificate reads it again for each `hello` it answers, unless its last reading is a few seconds old | 3.1 |
| Limits behind a reverse proxy | An implementation note: count by the client address a trusted proxy forwards, never by a forwarded address from any other source; an IPv6 `/64` as one | 8.3 |
| NAT64 and 6to4 | `64:ff9b::/96` and `2002::/16` addresses are judged as the IPv4 address they carry; another NAT64 prefix in use should be too | 3.2 |

Destination cases for NAT64 and 6to4 are in
[vectors/v1/strict.json](../../vectors/v0/strict.json) (`destinations`).
