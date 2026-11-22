# MSFP version 0: the core, version 0

**Draft.** Version 0 is a draft: anything may change until version 1, the
first stable one. It doesn't work with the test draft of November 2026
([history](README.md#history)). How the parts fit
together is in [README.md](README.md).

The core is what every MSFP server speaks: encodings, transport, keys, the
pairing code, signed requests and answers, pairing and its states, key
rotation and revocation, address changes, and how versions are agreed.
The parts build on it: [catalog](catalog.md) (libraries and titles),
[playback](playback.md) (starting and stopping a playback) and
[media](media.md) (what the stream URLs serve) hold the shared
mechanics; the content parts, [video](video.md) (films and series by
TMDB) and [music](music.md) (artists, albums and tracks by MusicBrainz),
say what the titles are and how they play. Every content part is
optional.

The words "must", "must not" and "should" carry their usual meaning in
specifications. Every example uses `example.org` names and RFC 5737
addresses. The test vectors are in [vectors/v0/](../../vectors/v0/)
(section 17).

## Contents

1. [Overview and terms](#1-overview-and-terms)
2. [Encodings and canonical forms](#2-encodings-and-canonical-forms)
3. [Transport](#3-transport)
4. [Identity and keys](#4-identity-and-keys)
5. [The pairing code](#5-the-pairing-code)
6. [Signing requests and answers](#6-signing-requests-and-answers)
7. [Pairing, step by step](#7-pairing-step-by-step)
8. [Common rules for every call](#8-common-rules-for-every-call)
9. [The core's calls](#9-the-cores-calls)
10. [Versions and parts](#10-versions-and-parts)
11. [The discovery document](#11-the-discovery-document)
12. [Key rotation and revocation](#12-key-rotation-and-revocation)
13. [Changing a server's address](#13-changing-a-servers-address)
14. [Security considerations](#14-security-considerations)
15. [Versioning](#15-versioning)
16. [Coexisting with the test draft](#16-coexisting-with-the-test-draft) (removed)
17. [Test vectors](#17-test-vectors)
18. [Open questions](#18-open-questions)

## 1. Overview and terms

MSFP lets media servers share their libraries: one server's people browse
and play titles another server holds, through their own server. Servers
pair by hand, two operators at a time (section 7), or through a group of
servers whose operators chose each other ([groups](groups.md)). Each
trusts only the servers it paired with, by hand or through a group.

- **Server**: one installation of an implementation of this protocol. It
  has one address and one current Ed25519 key pair.
- **Address**: where a server's API answers, `https://host[:port]`, with
  no path (section 5.2).
- **Base path**: the path under the address where the federation calls
  are; `/api/v1/federation/` unless the server's discovery document says
  otherwise (section 11).
- **Peer**: another server this server knows by its public key. A peer is
  in one of the states of section 7.3.
- **Pairing**: the act, done by two operators by hand, that makes two
  servers peers. Nothing is shared before it.
- **Pairing code**: one line of text an operator sends another operator.
  It holds the maker's address, public key, a one-time secret and an
  expiry time (section 5).
- **Core**: this document. **Part**: a separately versioned set of calls
  and rules built on the core: `catalog`, `playback`, `media`, `video`,
  `music`, `groups` (section 10). **Content part**: a part that names one kind of
  content's titles and says how they play: `video` and `music`.
- **Current key**: the key a peer knows a server by now. It may change by
  a rotation (section 12).
- **Next key commitment**: SHA-256 of a public key a server has made but
  not used yet, which its peers store and accept a rotation to
  (section 12).
- **A server that can't be called**: one that can connect out but can't
  be connected to, and says so with `inbound: false` (section 7.5). It
  calls its peers; they never call it.
- **Owning server**, **viewer's server**, **library**, **offer**, **want**,
  **title key**, **playback**, **ticket**: defined by the parts that use
  them ([catalog](catalog.md#1-terms), [playback](playback.md#1-terms)).

An implementation may run beside its media server rather than inside it,
with its own address, or behind the same reverse proxy with a discovery
`base` (section 11). It then relays the media, and holds a credential of
the media server; it should hold the narrowest one the media server
allows (a dedicated account limited to the offered libraries).

Every call is an HTTP `POST` with a JSON body, signed by the caller. Every
answer to a known key is signed by the server answering. The media itself
is fetched with plain `GET` requests that carry a ticket only
([media](media.md)).

## 2. Encodings and canonical forms

- **base64url**: the URL-safe alphabet of RFC 4648 section 5, **without
  padding**. Keys, nonces, signatures, hashes, commitments, pins and the
  pairing proof use it.
- **hex**: lowercase hexadecimal. The body hash inside a signed message
  uses it.
- **JSON**: I-JSON (RFC 7493), as below. Fields marked "optional" may be
  left out.
- **Times** are Unix seconds, written in decimal ASCII.
- Text in signed messages and statements is ASCII, and lines are
  separated by a single line feed (`\n`, byte 0x0A). There is never a
  trailing line feed.

Every value below has exactly one valid text. A sender must write that
text. A receiver must refuse any other, the same way it refuses a bad
signature or a bad body: an unsigned `404` when the value is checked
before the signature checks out (section 6.4); after it, a signed refusal
with the status that call gives a bad body (section 8.2).

### 2.1 Ed25519

Keys and signatures are pure Ed25519 (RFC 8032, not Ed25519ph), with the
following rules, which RFC 8032 leaves open.

A **public key** is valid only when all of these hold:

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
cofactorless equations accept exactly the same signatures. So any RFC 8032
library may do step 3, once the point checks are done. Every honest
signature passes: R = [r]B and A = [a]B. The checks are needed because
libraries differ: some accept the identity as a key, and under it the
signature (R = identity, S = 0) verifies every message; some accept a
torsion component in R, which matters once a signed statement is checked
by more than one server (section 12).

A receiver checks a key once, when it first learns it (in a code, in
`MSFP-Key` on `pair`, in a rotation), and may keep the result. It checks R
on every signature, statements included.

One way to make the point checks: decode the point, re-encode it and
compare (canonical), multiply it by the cofactor 8 and compare with the
identity (small order), compute [L − 1]P + P and compare with the
identity (subgroup).

### 2.2 Text encodings

- **base64url**: only the characters `A–Z a–z 0–9 - _`; no padding, no
  white space; the unused bits of the last character are zero. A simple
  test: re-encoding the decoded bytes gives the same text. Lengths are
  fixed where the value is: 43 characters for 32 bytes, 22 for 16, 86 for
  64.
- **hex**: lowercase `0–9 a–f` only. A SHA-256 is 64 characters.
- **Decimal integers** in headers, signed messages and statements: `0` or
  `[1-9][0-9]*`, no sign, no white space, at most 9,007,199,254,740,991
  (2^53 − 1). `MSFP-Date` is a positive integer in this form.
- **Status** in an answer message: the three digits of the status code.

### 2.3 JSON

- Bodies are I-JSON (RFC 7493): UTF-8 without a byte order mark; no
  surrogate code points (escaped or not, so no lone surrogates) and no
  noncharacters; no object with the same member name twice, names being
  compared after their escapes are decoded.
- Member names match exactly, case included. A name in another case is
  an unknown member.
- Unknown members are ignored. This is not leniency: it is how later
  versions add fields (section 15).
- Integers: a JSON number with no fraction, no exponent, not `-0`, from
  −(2^53 − 1) to 2^53 − 1. Any other number (`2.5`, `1.0`, `1e3`) is
  valid I-JSON, and is invalid only where an integer is expected. Where
  a list ignores entries that aren't integers, it ignores such a number
  too: `"core":[0,2.5]` lists core 0 (section 10.2).
- `null` is not a valid value for any MSFP member. A sender leaves an
  empty optional member out, or sends `[]` or `""` where the field says
  so. A receiver refuses `null` in a known member of a request as an
  invalid value; in an answer, `null` is a member of the wrong type
  (below). In an unknown member, or as the value of an unknown part
  (section 10.2), `null` is ignored like any other value.
- A member of another JSON type than its field says is an invalid value.
  That is for requests, and for known members only: unknown ones are
  ignored, whatever their value. In an answer, a member of the wrong type
  inside a list skips its entry, or counts as absent, and one at the top
  of the answer fails the answer, as each call's section says
  ([catalog](catalog.md) sections 2.2 and 3.3, and the content parts'
  titles: [video section 3.1](video.md#31-what-the-caller-keeps),
  [music section 3.2](music.md#32-what-the-caller-keeps)).
- String values are compared and signed after JSON decoding. Since
  addresses, keys and IDs are ASCII, a sender has no reason to escape any
  of them.
- Insignificant white space between JSON tokens is allowed. Bodies are
  signed as bytes, never rebuilt, so their layout doesn't matter. Senders
  should send none.

### 2.4 Headers

- Each of the four signature headers (section 6.1) must appear exactly
  once in a request, and `MSFP-Signature` exactly once in an answer. Two
  instances, or one value holding a comma, are refused.
- A header value is taken after HTTP removes leading and trailing
  optional white space. Any other white space inside makes it invalid.
- `Content-Type` is compared without its parameters:
  `application/json; charset=utf-8` is `application/json`. Only the type
  and subtype are checked; parameters aren't read, whatever they say.

### 2.5 Case

Header names are case-insensitive, as in HTTP. Everything else is
case-sensitive and has one case:

- scheme and host of an address: lowercase;
- hex: lowercase;
- enumerated values (`paired`, `movies`, `direct`, `hls`…): exactly as
  listed;
- part names: lowercase, as listed;
- title keys: exactly as the content parts write them
  ([video](video.md#4-title-keys), [music](music.md#2-title-keys);
  [catalog](catalog.md#4-title-keys) gives the general rules);
- language tags: the case conventions of BCP 47 (RFC 5646 section
  2.1.1): language lowercase, script title case, region uppercase
  (`fr-CA`, `zh-Hant-TW`). Receivers compare tags case-insensitively, so
  `fr-ca` and `fr-CA` count as one tag, but senders write the
  conventional form.

Cases for all of these, valid and not, are in
[vectors/history/test-draft-strict.json](../../vectors/history/test-draft-strict.json) and
[vectors/v0/strict.json](../../vectors/v0/strict.json) (section 17).

## 3. Transport

- **HTTPS only.** Every federation call and every media fetch uses
  `https`, with the certificate checks of section 3.2. There is no plain
  `http` in version 0. A server without a certificate from a public
  authority makes a self-signed one, or uses a private authority's, and
  names its pin (section 3.2).
- **Paths.** The federation calls are under the peer's base path:
  `/api/v1/federation/` on the peer's address, unless its discovery
  document gives another (section 11). The `v1` in that path is a fixed
  literal, not the protocol's version.

  | Call | Path under the base | Who calls whom | Defined in |
  |------|---------------------|----------------|------------|
  | pair | `pair` | the server redeeming a code calls the code's maker | section 9.1 |
  | hello | `hello` | either peer | section 9.2 |
  | rotate | `rotate` | a server changing its key calls each peer | section 9.3 |
  | revoke | `revoke` | a server revoking its key calls each peer | section 9.4 |
  | libraries | `libraries` | either paired peer | [catalog](catalog.md#2-libraries) |
  | titles | `titles` | either paired peer | [catalog](catalog.md#3-titles) |
  | play | `play` | the viewer's server calls the owning server | [playback](playback.md#2-play) |
  | progress | `progress` | the viewer's server calls the owning server | [playback](playback.md#3-progress) |
  | stop | `stop` | the viewer's server calls the owning server | [playback](playback.md#4-stop) |

  The media is under the stream root, `<address><prefix>/api/v1/stream/`
  ([media](media.md#1-the-urls)).

  A server never calls a peer that can't be called (section 7.5), and
  never fetches its media: such a peer only makes calls, and is never
  the owning server.
- **Request headers.** `Content-Type: application/json` and the four
  signature headers of section 6.1 are required.
- **No browser headers.** Requests must not carry `Sec-Fetch-Site` with a
  value other than `same-origin` or `none` (browsers' cross-site marker).
  A server-to-server client sends no `Sec-Fetch-*` headers at all.
- A request without `Content-Type: application/json`, with a
  cross-site `Sec-Fetch-Site`, or with a content coding (below), is
  refused with the unsigned `404` of a failed check (section 6.4).
- **Answer bodies.** `application/json` for `200` and `202`. Signed
  refusals are RFC 9457 problem documents, `application/problem+json`,
  with `type`, `title` and `status` members (section 8.2). One refusal has
  an empty body and no `Content-Type` (`409` on `pair`). The answer's
  media type is signed (section 6.3).
- **Unsigned refusals.** Their body is not specified. A receiver should
  send whatever its host sends for a path that doesn't exist, so that the
  two can't be told apart. Where the host can't do that, it sends the
  problem document of section 8.2.
- **Redirects.** A caller must not follow them. A request is signed for
  one host only. A `3xx` counts like any other status that isn't `200`
  or `202`.
- **Sizes.** A request body is at most 64 KiB (65,536 bytes); a receiver
  refuses a longer one with an unsigned `404`. A caller reads at most
  4 MiB (4,194,304 bytes) of an answer; a longer answer fails its
  signature check.
- **Timeouts.** A caller should give each call 15 seconds in all.
- **Content codings: none.** A federation request and its answer carry
  no `Content-Encoding` other than `identity`, and the signatures of
  section 6 cover the body bytes as sent. A receiver sends its answers
  uncoded, whatever the request's `Accept-Encoding` says: a host that
  compresses answers by itself must leave the federation paths alone. A
  caller should send no `Accept-Encoding` (or `identity` only), and
  refuses an answer that carries a content coding like a badly signed one
  (section 6.6), without decoding it. A receiver never decodes a coded
  request either: it refuses a request whose `Content-Encoding` names
  any coding other than `identity` at once, with the unsigned `404` of a
  failed check, whether or not the body is actually coded. Transfer codings
  (`chunked`) are HTTP's framing, not a content coding: the body is the
  bytes they carry.
- **Proxies.** A server behind a reverse proxy must keep the path, query
  and body byte for byte: the signature covers them.

### 3.1 Local testing

An implementation may offer plain `http` as a test switch, for its own
development. Such a switch is outside the protocol: a server must not
make or accept `http` pairings with it on in normal use.

### 3.2 HTTPS certificates

Identity rests on the key, never on the certificate (section 4). But a
media fetch is not signed: its ticket and bytes have only TLS to protect
them. So a server's key vouches for its certificate, and a peer may use a
certificate no public authority issued.

**The pin.** A certificate is named by its pin: the SHA-256 of its
`SubjectPublicKeyInfo` (the DER bytes of the public key with its
algorithm, as in the certificate), sent base64url (43 characters). This
is the same value as HPKP's `pin-sha256`, in base64url instead of base64.
A server names its pins in the `certificates` field of `hello`
(section 9.2). It should name the pin of every certificate its address
may serve now: the leaf certificate's, not an authority's. During a move
(section 13.1), it names the pins of both addresses, the old and the
new, until the move ends.

**What a client accepts.** For a connection to a peer's address, the
client checks the certificate chain the peer's TLS server presents, in
one of three ways:

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
| `pair`, `hello`, and `GET /.well-known/msfp` (section 11) | *trusted*, *named* or *any* |
| every other federation call (`rotate`, `revoke`, and the parts' calls, `group-pair` included: its pins come in the group's mail) | *trusted* or *named* |
| the media (the stream root) | *trusted* or *named* |

`pair` and `hello` may take any certificate because their answers are
checked by key (section 6.6) and neither way carries a secret: the
pairing proof is bound to the caller's key and address (section 9.1). They
are how pins are learnt, before any is known and after a renewal. The
discovery document proves nothing anyway. Every other request needs a
*trusted* or *named* certificate, `play` above all, since its answer
carries the ticket.

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
- When a peer's stored address changes (section 13), the pins learnt
  from the old address are dropped and learnt again from the new one's
  `hello`.

**Renewals.** A server whose certificate is renewed names the new pin in
its `hello`s before it switches to the renewed certificate. A client that
is refused a certificate on a call that needs *trusted* or *named* should
send a `hello` to that peer, and, if the pins it gets differ from those it
had (compared as sets), try the call once more. It does so for every call
and for media fetches. A client sends at most one such `hello` to a peer
at a time; calls and fetches that wait on it share its result.

A server's answer to a `hello` names the pins of the certificates its
address serves when it answers, not those of an earlier reading. A peer
sends a `hello` when a call was refused the certificate: an answer that
named the pin from before a renewal would make that peer's one retry
fail, and nothing would try again before its next scheduled call.

**Knowing one's own certificate.** A server behind a reverse proxy
doesn't hold its certificate. It may connect to its own address, as a
peer would, with SNI set to the address's host name (none for an IP
address, as RFC 6066 says), read the leaf certificate served there, and
name its pin. It reads it again for each `hello` it answers, unless its
last reading is at most a few seconds old, and when
it sends a `hello` a minute or more after the last reading. A failed
reading keeps the last good one (none if there was never one), and the
next attempt waits a minute. A server whose operator sets its pins in its configuration
names those and reads nothing. A server may learn its pins any other
way, as long as they are of the certificates its address really serves.

A server that serves several certificates (RSA and ECDSA, several
machines, a renewal rolling out) names all their pins. One reading sees
only one of them, so such a server should take its pins from its
configuration.

A server whose certificate is publicly trusted needs no pins, but should
name them all the same: its peers then keep working if it moves to a
self-signed certificate.

**Self-signed certificates.** Recommended, not required: a key for the
certificate of type Ed25519 or ECDSA P-256, kept apart from the federation
key; the host of the server's address as its name (subject alternative
name); a lifetime of a year or less, renewed by the implementation itself
before it ends; the new pin named in `hello` before the switch to a
renewed certificate.

### 3.3 Connecting to peers

Every address a server connects to for federation comes from a peer, or
from an operator who copied it from a peer: the address in a code
(section 5), in a `pair` request (section 9.1), in a `hello` that
announces a move (section 13.2), a discovery document's address
(section 11), and the media URLs under a peer's stream root
([media](media.md#1-the-urls)). A hostile peer could name an address that
points back into the server's own machine or network. So each connection
made for a peer is checked first.

**Refused destinations.** A server must refuse, by default, to connect to
an IP address in any of these ranges:

| Range | What it is |
|-------|------------|
| `127.0.0.0/8`, `::1` | loopback: the server's own machine |
| `169.254.0.0/16`, `fe80::/10` | link-local; a cloud's metadata service answers at `169.254.169.254` |
| `0.0.0.0/8`, `::` | unspecified, and IPv4's "this network" |
| `224.0.0.0/4`, `ff00::/8` | multicast |
| `240.0.0.0/4`, with `255.255.255.255` | reserved, and broadcast |

An IPv4-mapped IPv6 address (`::ffff:0:0/96`, for example
`::ffff:127.0.0.1`) is judged as the IPv4 address it carries.

So are a NAT64 address of the well-known prefix `64:ff9b::/96` (RFC
6052; the IPv4 address is its last 32 bits) and a 6to4 address,
`2002::/16` (RFC 3056; the IPv4 address is bits 16 to 47), by every rule
of this section, operator policy included. Where a NAT64 gateway or a
6to4 relay is on the path, `64:ff9b::a9fe:a9fe` reaches
`169.254.169.254`. A server whose network uses another NAT64 prefix (a
network-specific one, or RFC 8215's `64:ff9b:1::/48`) should judge
addresses under it the same way.

**Private ranges are allowed by default.** `10.0.0.0/8`, `172.16.0.0/12`,
`192.168.0.0/16`, `100.64.0.0/10` and `fc00::/7` stay reachable: two
servers on the same home network pair this way.

**Operator policy.** A server may let its operator change this, with
these bounds:

- The operator may allow loopback, for a peer on the same machine. Every
  other range of the table stays refused, whatever the setting.
- The operator may refuse more: private ranges (a server that pairs only
  over the internet), or any range the server's network needs kept out.
  Some clouds also serve metadata at an address in a private range
  (`fd00:ec2::254`, for example); a server there should refuse it.
- The default, with no setting, is the table above.

**Names.** For a host that is a name, a server:

1. looks the name up once for the connection;
2. checks every address the answer holds, and refuses the name if any of
   them is refused, so that a name can't pair a LAN address with the
   metadata service;
3. connects only to an address it checked, without looking the name up
   again. A second lookup could give another answer (DNS rebinding).

Each new connection repeats the lookup and the check. A connection kept
open and reused was checked when it was made. TLS still uses the name:
SNI and the certificate check are as section 3.2 says, whatever address
was dialed.

**Through a proxy or a gateway.** A server whose connections go through a
proxy or a gateway of its own makes sure the check happens where the
connection is made, by the same rules. A server must not use a proxy that
would connect to a refused address for it, such as a proxy named by its
environment.

**Redirects** are never followed (section 3), so a peer can't send a
connection elsewhere after the check.

**What a refused address means.** A refused connection fails like an
unreachable peer, and nothing is sent: on a code, the redeeming server
tells its operator that the address can't be reached; on an address
change, the old address stays (section 13.2, step 3); on a media fetch,
the fetch fails ([media section 2](media.md#2-requests)). A server may
also refuse an address when it reads it (a code, a `pair` request), but
must check again at connection time, since a name's answer can change.

## 4. Identity and keys

### 4.1 The key

Each server has one current Ed25519 key pair (section 2.1). A server is
known to its peers by its public key alone, never by its name, address or
certificate. The key is made on first start, and changes only by a
rotation (section 12). There is one exception: a server whose key was
stolen with no next key safe makes a new key, and every pairing ends
(section 12.7, case 2).

A public key must satisfy section 2.1. A server refuses a key that
doesn't, wherever it meets it: in a code, in `MSFP-Key` on `pair`, or as
the new key of a rotation.

The public key travels as the 32 raw bytes, base64url: 43 characters.
Example (the test key "a" of section 17):

```
A6EHv_POEL4dcN0Y50vAmWfk1jCbpQ1fHdyGZBJVMbg
```

Signatures are the 64 raw bytes, base64url: 86 characters.

There is no other server ID on the wire. An implementation may give each
peer a local ID for its own panel; peers never see it.

### 4.2 The fingerprint

Operators compare fingerprints, by phone or another channel they trust,
before they accept a pairing or confirm a rotation. The fingerprint is:

1. SHA-256 of the 32 raw public-key bytes;
2. the first 15 bytes of that hash;
3. encoded in standard base32 (RFC 4648 section 6, alphabet `A–Z2–7`):
   exactly 24 characters, no padding;
4. split into six groups of four, joined by `-`.

Example, key "a": `KZDV-VJ2U-MNDU-YAUF-35O3-6K6K`.

Steps 1 and 2 give the first 15 bytes of the key's next key commitment
(section 12.5), which is the same SHA-256. So the fingerprint of a
server's next key is made from its commitment alone, and can be shown
while the key's seed is offline.

The fingerprint never travels between servers. Each panel computes it.

## 5. The pairing code

### 5.1 Format

A core 0 code is the ASCII prefix `msfp-pair-0.` followed by base64url of
these bytes, in order:

| Bytes | Content |
|-------|---------|
| 32 | the maker's public key |
| 32 | the secret: 32 random bytes |
| 8 | when the code expires: Unix seconds, unsigned, big-endian |
| 1 | `0`: no group follows; `1`: a group follows |
| 32 | with `1` only: the group's ID ([groups section 3.1](groups.md#31-the-envelope)) |
| 1 | with `1` only: the group name's length in bytes, 1 to 255 |
| that many | with `1` only: the group's name, UTF-8, cleaned as section 9.1 cleans names; a name of 80 code points over 255 bytes is cut, one whole code point at a time from its end, until it fits |
| 1 or more | the maker's address, as ASCII bytes (section 5.2) |

The maker sets the expiry, cut to the whole second: 24 hours after it
made the code by default, at most 30 days. It also sets how many servers
may use the code: once by default. The code doesn't carry that number:
the maker counts the uses, and refuses a `pair` once they are spent, as
for an expired code (section 9.1). A code with a group invites to that
group ([groups section 2](groups.md#2-codes-for-a-group)).

A reader must:

- remove every character with the Unicode `White_Space` property,
  anywhere in the text (codes get wrapped in emails). A valid code is
  ASCII, so this can't change one;
- refuse text without the exact prefix, case included;
- refuse a remainder that isn't base64url (section 2.2), or is over
  1,164 characters (873 bytes: 73, a group's 288 at most, and a 512-byte
  address);
- refuse a flag byte other than `0` and `1`; with `1`, a name length of
  0, or a name that isn't valid UTF-8 or is over 80 code points once
  cleaned;
- refuse a decoded value with no address bytes left (the address can't be
  empty);
- refuse a key that breaks section 2.1;
- refuse an expiry above 2^53 − 1;
- refuse address bytes that aren't ASCII or break section 5.2.

The reader doesn't check whether the code has expired. The maker does,
when the code is redeemed, and refuses it then.

A server that can't be called makes no codes (section 7.5).

### 5.2 Addresses

An address, in a code, in the pair request, in `hello`, and wherever a
server writes its own, has this form and no other:

```
address = "https://" host [ ":" port ]
port    = a decimal integer from 1 to 65535, without leading zeros,
          and not 443
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
  address anywhere. A server hosted under a path gives its base path in
  its discovery document (section 11); its address stays without a path.

Examples: `https://a.example.org`, `https://192.0.2.10:8096`.

**The address of a server that can't be called.** A server that sends
`inbound: false` (section 7.5) must have an address of this form:

```
https://msfp-<26 characters>.invalid
```

The 26 characters are 16 random bytes in lowercase base32 (RFC 4648
section 6, `a-z2-7`, no padding); the 2 unused bits of the last
character are zero. The implementation makes the address once and keeps
it; the operator types none. Example (from
[vectors/v0/strict.json](../../vectors/v0/strict.json),
`uncallableAddresses`): `https://msfp-4dq6fy7e4xtop2hj5lv6z3po54.invalid`.

- It is an address like any other: the pairing proof binds it
  (section 9.1), and requests to the server are signed for its host
  (section 6.2). But nothing ever connects to it. `.invalid` never
  resolves (RFC 6761 section 6.4), so a peer that ignores `inbound`
  connects nowhere, and section 3.3 never comes into play. Never a LAN
  address: a peer would send its calls to whatever answers there on
  **its** network.
- A peer refuses `inbound: false` with any other address, as a bad body
  (sections 9.1 and 9.2). The address checked is the message's `address`
  member, or, in a `hello` without one, the address the peer stores.
  A `hello` answer carrying `inbound: false` with any other address
  fails, as a badly signed answer does (section 6.6), and changes
  nothing.
- A server that becomes reachable announces its real address by an
  address change (section 13), in a `hello` without `inbound: false`.

### 5.3 Example

The code made by key "a", at `https://a.example.org`, with the secret
`404142…5f` (section 17), expiring at 1800086400:

```
msfp-pair-0.A6EHv_POEL4dcN0Y50vAmWfk1jCbpQ1fHdyGZBJVMbhAQUJDREVGR0hJSktMTU5PUFFSU1RVVldYWVpbXF1eXwAAAABrSyOAaHR0cHM6Ly9hLmV4YW1wbGUub3Jn
```

## 6. Signing requests and answers

### 6.1 Headers

Every request carries four headers:

| Header | Value |
|--------|-------|
| `MSFP-Key` | the sender's public key, base64url (43 characters) |
| `MSFP-Date` | when the request was signed, Unix seconds, decimal |
| `MSFP-Nonce` | 16 random bytes, base64url (22 characters), never used twice |
| `MSFP-Signature` | the sender's Ed25519 signature of the request message, base64url (86 characters) |

Every signed answer carries one:

| Header | Value |
|--------|-------|
| `MSFP-Signature` | the answering server's Ed25519 signature of the answer message, base64url |

### 6.2 The request message

The sender signs these bytes: seven lines, joined by `\n`, with no
trailing `\n`.

```
msfp-0 request
<method>
<host>
<target>
<date>
<nonce>
<body hash>
```

- Line 1 is the literal label `msfp-0 request`.
- `<method>`: the HTTP method, in capitals: `POST` for every call.
- `<host>`: the host part of the **receiver's** address (section 5.2),
  with its port if it has one. Example: `a.example.org`, or
  `192.0.2.10:8096`. No scheme.
- `<target>`: the request target, in origin form: the path as sent on the
  wire (percent-encoded as sent), the base path included. Federation calls
  have no query, so this is the path, for example
  `/api/v1/federation/pair`.
- `<date>`: exactly the text of the `MSFP-Date` header.
- `<nonce>`: exactly the text of the `MSFP-Nonce` header.
- `<body hash>`: SHA-256 of the exact body bytes sent, in hex (64
  characters), which carry no content coding (section 3). An empty body
  hashes to `e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855`.

The receiver rebuilds this message from one of **its own** current hosts,
never from the `Host` header. A server has one current host, or two for a
while after its address changed (section 13); it tries each. A request
signed for one server therefore fails on every other, even when it is
sent there. The receiver takes `<target>` from the request target as it
received it, percent-encoding untouched, not from a decoded path. A
federation call with a query is refused like a bad signature.

### 6.3 The answer message

The answering server signs these bytes: five lines, joined by `\n`, with
no trailing `\n`.

```
msfp-0 response
<nonce>
<status>
<media type>
<body hash>
```

- Line 1 is the literal label `msfp-0 response`.
- `<nonce>`: the text of the request's `MSFP-Nonce` header. This binds
  the answer to its request: it can't be replayed for another one.
- `<status>`: the HTTP status code sent, three digits (`200`, `404`…).
- `<media type>`: the media type of the answer's `Content-Type` header:
  type and subtype, in lowercase, without parameters or white space
  (`application/json; charset=utf-8` gives `application/json`). An answer
  without `Content-Type` has `-` here.
- `<body hash>`: SHA-256 of the exact answer body bytes as sent, hex.
  They carry no content coding (section 3).

The caller builds `<media type>` from the `Content-Type` header it
received. An answer carries one `Content-Type` header or none: one sent
more than once, or whose value doesn't read as `type/subtype` (with
optional parameters), fails like a badly signed answer; an empty one
counts as none (`-`). So two implementations never read different media
types from the same answer. So a changed `Content-Type` makes the signature fail, and the
caller can rely on the media type it reads. Other headers are not signed.

A caller refuses a signed `200` or `202` whose media type isn't
`application/json`, like a badly signed one.

### 6.4 Checking a request

The receiver checks, in this order, and refuses at the first failure:

1. Each of the four signature headers appears exactly once, the request
   has no query, and `MSFP-Key` is base64url of exactly 32 bytes.
2. `MSFP-Date` is a positive decimal integer in the form of section 2.2.
3. The date is within 5 minutes of the receiver's clock: the difference
   between the receiver's current time and the date, in either direction,
   is at most 300 seconds. The receiver's time is not rounded.
4. The date is not earlier than the moment the receiver's nonce memory
   began (section 6.5). The date counts as the start of its second: a
   request dated the second the process started is refused, unless the
   process started exactly on that second.
5. `MSFP-Nonce` is base64url of exactly 16 bytes.
6. `MSFP-Signature` is base64url of exactly 64 bytes.
7. The key is valid (section 2.1; a known key was checked when it was
   learnt), and the signature is valid under it, by section 2.1's rules,
   over the request message of section 6.2 rebuilt for one of the
   receiver's current hosts.
8. The key is one the receiver hears (section 6.5).
9. The pair (key, nonce text) was never accepted before, and the nonce
   memory has room.

Then the receiver remembers (key, nonce text) and handles the call.

Every failure gets the same answer: an unsigned `404`, the same as a path
that doesn't exist (section 3). The sender learns nothing about why. There
is one exception, in step 8: a key whose pairing this receiver ended
(section 7.4), or a suspended key on a call other than `rotate` and
`revoke` (section 12.6), gets a **signed** `404`, so that its server can
trust it. A request signed with the receiver's own key (the address check
of section 13.1, step 2) gets a `404` signed with that same key, so that
only the server itself can produce that answer.

### 6.5 Nonces, replay and restarts

- **Who is heard.** After the signature checks out, the receiver asks
  whether it hears the key. It hears:
  - the current key of a peer, in any state;
  - a peer's previous key, for 10 minutes after a rotation away from it
    took effect (section 12.4), until that key is revoked: from then on
    it isn't heard at all (section 9.4);
  - a suspended peer's current key, on `rotate` and `revoke` only; on any
    other call it gets the signed `404` of section 6.4 (section 12.6);
  - on `pair` only, every key while at least one of its pairing codes is
    live (made, with uses left, and not expired), forgotten keys
    included; otherwise a new code could never restart an ended pairing;
  - on the calls of the part `groups`, the members of its groups, as
    [groups section 8](groups.md#8-the-calls) says: on `group-pair`,
    members it isn't paired with yet, with their own nonce memory.

  On every other call, and on `pair` without a live code, a forgotten key
  gets the signed `404` of section 6.4. Any other key is refused before
  its nonce is stored. So strangers can't fill the memory with nonces
  they made up.
- **The memory.** The receiver keeps every accepted (key, nonce) pair for
  at least 10 minutes (twice the 5-minute window), from the moment the
  request was accepted. Older requests fail step 3 anyway. A receiver
  should keep a memory per known peer (for example at most 20,000
  entries each, above the 12,200 calls the suggested limits of section
  8.3 let one peer make in 10 minutes), so that one peer can't fill
  another's, and one for strangers heard during pairing (for example at
  most 1,000). A nonce in either one is refused. When a memory
  is full, after old entries are cleared, requests that would go in it are
  refused until room comes back.
- **Restarts.** A receiver that keeps nonces in memory only can't know
  what it saw before it restarted. So step 4 refuses every request dated
  before the process started. There is no allowance for the other
  server's clock. If the sender's clock runs behind by *n* seconds, its
  requests are refused for about *n* seconds after each restart of the
  receiver. Keep both clocks set by the network.
- **Retrying.** A sender whose call to a peer is refused with an unsigned
  `404` should retry it once, a second later, with a new nonce (signed
  again). A receiver that just restarted then accepts it.
- **A sender** must make a fresh random nonce for every request, from a
  cryptographic random source, and must never reuse one, even for a
  retry. A retry is a new request, signed again.

An implementation may keep nonces on disk instead. It must then still
refuse any request whose nonce it can't prove it hasn't seen.

### 6.6 Signing answers and checking them

The answering server signs, with the key it uses toward that peer
(normally its current key; during its own rotation, see section 12.4):

- every `200` and `202` answer;
- every refusal (any other status) to a request that passed section 6.4
  from a key it hears as a peer's;
- the `404` to a forgotten or suspended key.

It doesn't sign refusals to strangers: requests that failed section 6.4,
and requests from strangers on `pair` during a live code. Those stay
plain `404`s.

The caller checks every answer against the public key it knows the
answering server by: the peer's current key (section 12). For `pair`, that
is the key in the code.

- A `200` or `202` answer counts only when its signature verifies.
  Otherwise the caller treats the server as unreachable. Then it decodes
  the JSON body.
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
   (section 7.4). B may fetch A's discovery document (section 11) to find
   A's base path.
3. **B calls pair.** B sends a signed `pair` to the code's address, under
   A's base path (section 9.1). The body names B's address and name,
   carries SHA-256(secret), a proof that B knows the secret, and B's next
   key commitment. The secret itself never travels.
4. **A redeems the code.** A finds a live code by the secret hash, and
   checks the proof against the stored secret, the request's `MSFP-Key`
   and the `address` field. A refuses the request when the key is already
   a peer of A's (`409`). Otherwise A, in one transaction, deletes the
   code, takes B's key off its forgotten list if it is there (section
   7.4), and stores B: B's public key, B's address, B's cleaned name and
   B's commitment, in state `ours` (waiting for A's operator). A answers
   `200`, signed, with A's name and A's next key commitment.
5. **B stores A.** B checks the answer's signature against the key in the
   code. Only then B stores A: the code's key, the code's address, A's
   cleaned name and A's commitment, in state `theirs` (waiting for A's
   operator). B takes A's key off its own forgotten list if it is there:
   redeeming a code is consent to pair again.
6. **A's operator accepts.** A's operator compares B's fingerprint with
   B's operator, then accepts. A moves B to `paired`, and sends B a signed
   `hello` with state `paired`, its versions and its pins (section 9.2).
   B, which was waiting, moves A to `paired`, and answers with its own
   versions and pins. Both now know which parts they share (section 10).
   When B can't be called, A sends no `hello`: B learns at step 7
   (section 7.5).
7. **If B missed it.** If B couldn't be reached at step 6, B learns at its
   next `hello` to A: A answers `paired`. B should ask every 20 seconds at
   most while its panel shows a peer in state `theirs`; a B that can't be
   called must, on the schedule of section 7.5. Any call that A
   refuses with a signed `404` (other than `not-supported`) also makes B
   ask (section 9.2).

The operator of B gave consent at step 2, by pasting the code. A's
operator gives it at step 6. B can't make A accept: a `hello` saying
`paired` from a peer in state `ours` changes nothing.

A pairing made from a core 0 code starts in core 0: `pair` is signed
under section 6.

### 7.2 What each side stores

For each peer:

- its current key (the identity), its previous key and until when it is
  still heard (section 12.4), its next key commitment (from `pair` or a
  rotation; none after a rotation statement with `-`), a pending rotation,
  the history of its rotation statements, and whether it is suspended;
- its address (where to call it), its base path (section 11), and, while
  it is moving, its former address (section 13);
- its name (for the panel), its state, when it was last heard from (any
  request from the peer that passed section 6.4, and any answer from it
  whose signature verified);
- the core and part versions of its latest signed `hello` (section 10);
- its pins (section 3.2);
- whether it can be called: its `inbound` (section 7.5);
- what the parts keep: offers, wants and catalogs
  ([catalog](catalog.md)), limits ([playback](playback.md)), and the
  link's relations: whether each side wants the individual relation, and
  each group the link was made or kept through (section 7.7).

A stores B's address as B gave it in the pair request. B stores A's
address as the code gave it.

### 7.3 States

| State | Meaning | `hello` view sent |
|-------|---------|-------------------|
| `ours` | the peer used our code; our operator hasn't accepted | `waiting` |
| `theirs` | we used the peer's code; its operator hasn't accepted | `waiting` |
| `paired` | both sides accepted | `paired` |
| `gone` | the peer answered, signed, that it doesn't know us | `waiting` |

The parts' calls answer only peers in state `paired`; others get a signed
`404`. `hello` answers peers in every state. A server remembers whether a
`gone` peer was paired before. A `paired` hello brings it back to
`paired` only then.

Besides its state, a peer may be **suspended**: its key was revoked
(section 12.6). A suspended peer's calls are refused, except `rotate` and
`revoke`, and no call is made to it, but the scheduled `hello` of a
server that can't be called (section 7.5).

### 7.4 Ending a pairing

Either operator ends a pairing at once, or refuses a waiting one. Once
paired, an operator usually ends a relation instead, and the link ends
with its last relation (section 7.7).
Removing a peer, in any state (`gone` and suspended included), is an
ending:

1. The ending server deletes the peer, and records as "forgotten" the
   SHA-256 of every key in the peer's history: its current key, its
   previous key, every key of its rotation statements, and the key of a
   pending rotation (section 12.3, case 4). It keeps none of them.
2. Then it sends the peer a signed `hello` with state `ended`. If the peer
   can't be reached, nothing more is done. To a peer that can't be called
   it sends none (section 7.5).
3. The peer that receives `ended` deletes the ender, in every state,
   waiting ones included, and answers `200` with state `ended`, signed. It
   doesn't record the ender as forgotten: the ender has deleted it too, so
   nothing more passes between them.

From then on, the ender answers that key's valid requests with a signed
`404` (except `pair` while a code is live, section 6.5). The other server,
on a signed `404` to `hello`, marks the ender `gone`. A pairing that ended
starts again with a new code, or through a group both servers are in
([groups section 4.2](groups.md#42-making-the-link)): either takes the
keys off the forgotten list (section 7.1, step 5).

**Blocking** is an ending that also marks the keys it records as
blocked. The forgotten list then does one more thing for those keys: a
blocked server is kept away from every group (groups section 7), its
requests dropped and its `group-pair` refused, until a code from this
server takes it off the list. An ending that isn't a block keeps the
server away from nothing: its keys are recorded only so that its calls
get the signed `404` above.

Entries of the forgotten list may be removed after a year. A removed
entry only turns that key's signed `404` into an unsigned one.

### 7.5 A peer that can't be called

A server that can connect out but can't be connected to (no public
address, no port forwarded, no gateway; at home behind a router or a
carrier-grade NAT, say) says so, its peers stop calling it, and it asks,
on a schedule, for what it would otherwise be told. It can then watch
what its peers share, with their rotations and revocations reaching it.
It shares nothing. A server whose public part is a gateway it runs itself
is reachable at the gateway, and is not concerned.

**The `inbound` member.** `pair` requests (section 9.1) and `hello`
requests and answers (section 9.2) carry an optional boolean `inbound`.
`false`: the sender can't be called. Absent or `true`: it can.

- A receiver keeps, per peer, the value of the latest signed message that
  carried one, request or answer, latest by arrival, as for pins
  (section 3.2). A `hello` without `inbound` sets it back to `true`. In
  `pair`, the maker stores it with the new peer.
- A member that isn't a boolean is a bad body (section 9.2), as for the
  other members. So is `inbound: false` with an address not of the form
  of section 5.2.
- A server sends `inbound: false` in every `pair` and `hello` it sends
  (but those that say `ended`), and in every `hello` answer it gives,
  while its operator says it can't be called. A server can't tell by
  itself that it can't be reached, so this is the operator's setting.
- A `hello` answer may carry `callsFailed: true`: "my last calls to you
  couldn't connect". The receiver's panel shows it, with a suggestion to
  turn on "this server can't be called". Nothing changes by itself, so a
  lying peer can only put a message on a panel. "Couldn't connect" is
  any failure to make the connection (no route, refused, timed out, a
  name that doesn't resolve, TLS failing), except two: a certificate
  refused by section 3.2, and a destination refused by the server's own
  rules (section 3.3). The answering server stops sending it once a call
  of its own to the caller succeeds again; `pair` doesn't count.

**What a peer does toward a server that can't be called.** Toward a peer
whose `inbound` is `false`, a server opens no connection at all. Each
call it would make:

| Call | Normally | Toward a peer that can't be called |
|------|----------|-------------------------------------|
| `hello` on acceptance (section 7.1, step 6) | sent | not sent; the peer's next `hello` gets `paired` |
| `hello` to check, after a signed `404` (section 9.2), to recompute versions (section 10.4), after a refused certificate (section 3.2) | sent | not sent |
| `hello` with `ended` (section 7.4, step 2) | sent | not sent; the peer's next `hello` gets the signed `404` of a forgotten key, and the peer marks it `gone` (section 9.2) |
| `rotate` (sections 9.3, 12.4) | sent at each contact (at most once a minute), at least daily | not sent; the statement travels in `hello` answers (section 12.4) |
| `revoke` (sections 9.4, 12.6) | sent to every peer it can reach | not sent; the statement travels in `hello` answers (section 12.6) |
| `hello` confirming an address change (section 13.2, step 2) | sent to the new address | not sent; the address is stored at once (section 13.2) |
| the discovery document (section 11) | fetched after storing a redeemer's address | not fetched |
| `libraries`, `titles` | called | not called; the peer's catalog isn't shown |
| `play`, `progress`, `stop`, media | called, fetched | not called; nothing of the peer is offered for playing |

Its panel says what that means: "this server can't be called: it can
watch what you share with it, and can share nothing with you".

When a peer's `inbound` changes from `false` to `true`, the server
confirms the peer's address as section 13.2, step 2 does (a `hello`
there, answered under the peer's key) before it makes any other call to
it: the stored address, or the new one when the same `hello` names one.
Only a `200` answer confirms it; a signed `404` there doesn't. Until that
`hello` succeeds, it goes on as if `inbound` were `false`, and retries as
section 13.2, step 3 says (at most once an hour). An answer that carries
`inbound: false` puts the peer back as one that can't be called, and the
confirmation stops.
Once it succeeds, the server reads the peer's catalog at once
([catalog section 3.2](catalog.md#32-whole-list-or-changes)): what it
dropped while the peer couldn't be called comes back without waiting
for the next reading.

**What the server that can't be called does.**

1. **No codes.** It makes no pairing code: its panel tells its operator to
   ask the other operator for one. It can redeem codes. Two servers that
   both can't be called can't pair.
2. **`hello` on a schedule**, to every peer, suspended ones included,
   whether or not its panel is open:
   - every 20 seconds while the peer is `theirs`, for 24 hours after the
     `pair`, then every 5 minutes (section 7.1, step 7 is a "must" for
     it);
   - every 5 minutes while it is `ours` (waiting for its own operator);
   - every 5 minutes while it is `paired`;
   - once an hour while it is `gone`, so that a pairing the peer restores
     comes back (section 9.2);
   - and at once on the occasions where it would send one anyway.

   Each wait varies by a fifth either way. These `hello`s count against
   the peer's limits like any other call, and a signed `retryAfter` is
   obeyed (section 8.3). A suspended peer gets the scheduled `hello`
   only, so that a rotation can still reach it (section 12.6).
3. **Its address** is of the form of section 5.2, `https://msfp-<26
   characters>.invalid`, made by the implementation and kept.
4. **Pins.** It names none (nothing is served at its address). It keeps
   learning its peers' pins from their answers.
5. **`knownKey`.** Its `hello` requests carry `knownKey`, the key it
   checks the peer's answers against now, so that a rotation of the
   peer's key reaches it (section 12.4).

**Turning it off.** When its operator says it can be called again, the
server first checks its real address as section 13.1, step 2 does (a
request of its own there, recognised as its own). Until that address is
reached, it goes on as above: it keeps sending `inbound: false` with its
address of section 5.2's form, and keeps its scheduled `hello`s. Then it
announces the real address (section 13.2).

What stays as it is: its own calls (`pair`, `hello`, `libraries`,
`titles`, `play`, `progress`, `stop`, the media) and everything the peer
checks about them; its ending a pairing (it sends `ended`, which it can);
and the peer ending its playbacks at once when the pairing ends
([playback section 6.3](playback.md#63-how-a-playback-ends)): its player
then gets `404` on the next fetch, and treats the playback as ended
(section 8.4).

### 7.6 Pairing through a group

Two members of a group pair without a code: one asks, through the group,
and the other accepts, by hand or by its own rule; then `group-pair`
makes them `paired` at once, and `hello` follows as in section 7.1, step
6 ([groups section 4](groups.md#4-joining)). A pairing made so is a
pairing like any other: everything in this core holds for it. A request
refused through a group sends nothing: no pairing existed, so there is
no `ended` (section 7.4).

### 7.7 Relations

An operator doesn't manage the link itself, but its **relations**: why
the two servers are linked. There are two kinds:

- the **individual relation**, chosen by both operators;
- a **group relation**, one for each group the two servers share and are
  linked through ([groups section 1](groups.md#1-terms)).

A link lasts while at least one relation holds.

**The individual relation.** Each side says, in every `hello`, whether
it wants it: `individual` (section 9.2). The relation holds while both
sides' latest `hello` say `true`. A side never heard with `individual`
(an implementation from before it) counts as wanting what this side
wants.

- **Pairing with a plain code** (section 7.1): both sides want it from
  the start. **A group's code** gives the group relation only: neither
  side wants it ([groups section 2](groups.md#2-codes-for-a-group)).
- **Asking**: an operator sets its own `individual` to `true`, and the
  server sends `hello` at once. The other side's panel shows the ask, and
  its operator accepts it (its own `true`) or refuses. A refusal is
  private, as in groups section 4.1: nothing is sent, and the asker's
  panel keeps showing its ask until it withdraws it. Refusing an ask on a
  link with no other relation ends the link.
- **Ending**: an operator sets its own `individual` to `false`, and the
  server sends `hello` at once. A side that sees the other's `individual`
  go from `true` to `false` while the relation held sets its own to
  `false` too: the relation has ended for both.
- A server sends `hello` at once when its own `individual` changes. A
  server that can't be called (section 7.5) carries it in its scheduled
  `hello`.

**When the last relation goes**, each side ends the link (section 7.4),
unless either side wants the individual relation (its `individual` is
`true`), or its operator is still choosing whether to keep the link (a
dissolution, groups section 7). An ask that waits keeps the link until
it is answered.

**What the panel shows** is relations, in words the operator knows: the
servers it chose one by one, each group with its members, and for each
an "End", "Leave" or "Block". The link, its keys and the forgotten list
stay out of sight, except for the fingerprint checks of section 7.1 and
of a new individual relation between group members, which the panel
offers and doesn't require.

## 8. Common rules for every call

### 8.1 Requests

- Method `POST`, `Content-Type: application/json`, the four headers of
  section 6.1, under the peer's base path.
- The body is a JSON object, at most 64 KiB. Calls without fields send
  `{}`.
- The caller must not follow redirects.

Types, in every part: *string*, *integer* (section 2.3), *boolean*,
*array*, *object*. "Required" means the receiver refuses the request
without it, or with an invalid value.

### 8.2 Refusals

A signed refusal is a problem document. The `404` refusal is, byte for
byte (53 bytes, then a line feed):

```
{"type":"not-found","title":"Not found","status":404}
```

followed by `\n`. A receiver must not depend on these exact bytes: the
signature covers whatever was sent, and only the status matters to the
protocol, except where the `type` tells refusals apart: `not-supported`
on `404` (section 10.4), and the `409`s of `rotate` and
[playback](playback.md#2-play). The `type` values of version 0:

This is the reference for every status a federation call can answer, in
every part of version 0. Media fetches are not federation calls; their
statuses are in section 8.4.

| Status | `type` | `title` | Calls | Meaning | What the caller does |
|--------|--------|---------|-------|---------|----------------------|
| 200 | (none) | | every call | done; the body is the answer | uses it |
| 202 | (none) | | `rotate` | accepted, waiting for the receiver's operator (section 9.3) | waits |
| 400 | `invalid-request` | `Invalid request` | `rotate`, `revoke`, `play`, `progress`, `stop`, `previews`, `lyrics`, `image` | the body is malformed, or a value is out of range, or isn't a token where one is due (section 8.5) | gives up; a correct caller never gets it |
| 404 | `not-found` | `Not found` | every call | not heard, not known, not paired yet, no such code, or no such title for this peer; signed or not tells them apart (section 6.6) | as section 6.6 says |
| 404 | `not-supported` | `Not supported` | a call of a part that is off; every call but `hello` when no core is common | the receiver doesn't speak that part, or no core, with the caller (sections 10.3, 10.4) | makes no more calls of that part (or none but `hello`) until a newer `hello` |
| 409 | (no body) | | `pair` | the caller's key is already a peer of the maker (section 9.1) | tells its operator |
| 409 | `rotation-refused` | `Rotation refused` | `rotate` | the new key isn't accepted (section 9.3) | as section 12.4 says |
| 409 | `stream-limit-reached` | `Too many streams on this account` | `play` | the limits of the `play`'s purpose are all in use or reached, or the limit is 0: streams for watching, downloads at once or in a day for a download, scans in a day for a scan ([playback section 2.2](playback.md#22-what-the-owning-server-does)) | tells the viewer "busy now" |
| 409 | `delivery-refused` | `This delivery is refused` | `play` | the owning server won't serve the `delivery` asked, for reasons of its own (bandwidth, policy), or can't make it of that copy (`file` of a copy of several parts, a subtitle it can't burn), or the request names what to serve with a value it doesn't know (section 8.5) | asks again with another delivery, or without `delivery`; when no delivery it can use is left, tells the viewer "not playable" (for a download, "download not available") ([playback section 2.5](playback.md#25-refusals)) |
| 409 | `not-playable` | `This item can't be played` | `play` | the owning server has no way to play its copy | tells the viewer "not playable" |
| 410 | `playback-stopped` | `Playback was stopped` | `progress` | the playback isn't live, or isn't this pairing's | stops the viewer's player |
| 429 | `rate-limited` | `Too many requests` | every call | this peer is over its own limit (section 8.3) | waits, as section 8.3 says |
| 500 | `internal-error` | `Internal error` | every call | the receiver failed | tries again later, as for no answer |
| 503 | `server-busy` | `The server is busy; try again shortly` | `play`; every call when the server is full (section 8.3) | the server's transcodes, or its capacity, are all in use | waits, as section 8.3 says; on `play`, tells the viewer "busy now" |

A problem document may carry `retryAfter`, an integer from 1 to 3,600:
seconds before the caller should try again (section 8.3). A receiver that
doesn't know a `type` reads the status alone, except for `409` on `play`
([playback section 2.5](playback.md#25-refusals)).

The `title` texts are examples; any text will do.

`404` `not-found` means, on every call: "I don't hear you", "I don't know
you", "not paired yet", "there's no such code", or "there's no such title
for you". Signed or not tells them apart (section 6.6). A bad body on
`pair`, `hello`, `libraries` and `titles` gets a signed `404` too.

### 8.3 Limits and overload

A server must protect itself from a peer that calls too much, by fault or
on purpose, and must stay predictable when it is full. It must have all
of these:

1. **A limit per peer** on federation calls: how many calls one peer's
   key may make in a period. Calls before a signature checks out (a
   stranger, a `pair` attempt) are limited by network address instead.
2. **A global limit** on federation calls, all peers together.
3. **A limit on concurrent requests**, for one peer and in all.
4. **A limit on `play`**, besides the pairing's `streams`: how often one
   peer may call `play`, since each call can start work on the media.
5. **Bounded queues.** A request waits for a slot only in a queue of
   bounded length, for a bounded time. When the queue is full, or the time
   is up, the request is refused at once.
6. **Predictable overload.** A limit reached gives one of the answers
   below. Never a hang, a dropped connection, or an answer that gets
   slower without bound.

The numbers are each implementation's: a server sets its own to fit its
machine. Suggested numbers are in the
[implementers' notes](implementers.md#core).

**The answers.**

- To a known peer, once its signature checks out: a signed `429`
  `rate-limited` when that peer is over its own limit (items 1, 3 and 4);
  a signed `503` `server-busy` when the server as a whole is full
  (items 2 and 3, a full queue, or its transcodes on `play`).
- Either may carry `retryAfter` in its body (section 8.2), and the same
  value in a `Retry-After` header. The caller trusts only the body's: the
  header isn't signed.
- Before a signature checks out (a limit by network address): the
  unsigned `404` of a failed check, or a `429` from the server's host.
  For a known peer, an unsigned answer is no answer (section 6.6).

**What a caller does.**

- It never retries before `retryAfter` has passed.
- Without one, and after an unsigned refusal or no answer, it backs off:
  each retry waits about twice as long as the one before, from a few
  seconds up to an hour at most, with some randomness so that peers don't
  retry together.
- `hello`, `libraries` and `titles` wait for their next turn on their
  usual schedule, if that comes later.
- A `play` refused this way isn't retried by itself: the viewer is told
  "busy now" and may try again.
- A `progress` refused this way waits for its next turn. A `stop` may be
  retried a few times; the owning server ends an unused playback within
  2 minutes anyway ([media section 4](media.md#4-how-long-a-ticket-works)).

**A server that can't be called** (section 7.5) sends its `hello`s on a
schedule of its own: every 20 seconds while a peer is `theirs` (for 24
hours after the `pair`, then every 5 minutes), every 5 minutes while it is
`ours` or `paired`, once an hour while it is `gone`, each wait give or take a fifth.
They are calls like any other. They count against the peer's limits; a
signed `retryAfter` holds them back; after no answer or an unsigned
refusal they back off as above, or wait for their next turn on the
schedule, whichever is later. One call every 20 seconds is far below any per-peer limit
of the table above.

**Implementation note: behind a reverse proxy.** A server behind a
reverse proxy sees every request arrive from the proxy's address, so a
limit by network address (item 1) would put every peer and stranger in
one bucket. Such a server counts by the client address the proxy
forwards (`X-Forwarded-For`, `Forwarded`, or the PROXY protocol), and
only for a request that came from a proxy its operator names as trusted.
It takes the address that trusted proxy added: read from the right, the
first address that isn't one of its trusted proxies, never one written
further left. It never trusts a forwarded address from any other source,
since anyone can write those headers. An IPv6 address is best counted by
its `/64`. The limits after a signature checks out are per key and need
none of this.

### 8.4 Media answers

Media fetches ([media](media.md)) are plain `GET`s, not signed, and their
answers aren't signed either. Their statuses:

| Status | Meaning | What the viewer's server does |
|--------|---------|-------------------------------|
| 200 | the whole resource | relays it, after the checks of media section 3 |
| 206 | the range asked for | checks `Content-Range` (media section 2), relays it |
| 404 | no such file, a query string, browser headers, a ticket that isn't live, or a URL its ticket's kind doesn't open | for a playlist, the file or a segment: treats the playback as ended; for a subtitle part, a font, a sheet or an image: goes without it |
| 405 | a `HEAD`, or any other method but `GET`, with `Allow: GET`; never `404` (media section 2.2) | not expected: the viewer's server sends `GET` only; it answers its own clients' `HEAD` itself |
| 416 | the `Range` header is malformed or out of the file; under a scan ticket, a fetch without `Range`, or one that starts once the bytes the scan may read are spent (the scan answer's `scanBytes`, [playback section 8](playback.md#8-scan-access); media section 4.2) | answers `416` to the viewer; for a scan, a failed read of its media server |
| 503 | the owning server is busy | answers `503` to the viewer, who may try again |
| anything else | not expected | as a failed fetch: the viewer's server answers `404` to the viewer |

A body that comes with a refusal is not read beyond a few kilobytes, and
never shown.

### 8.5 Unknown values

Many fields take a value from a list that grows: a codec, a container's
media type, an edition. The fields a part calls **open** follow one
rule, in every part, so that a new value needs no new version and no
change of the specification.

- A **token** is 1 to 32 characters of `a–z 0–9 . _ -`. An open field
  takes a token, unless its part says otherwise (a few take a class of
  their own, with a bound of their own).
- A **media type** is an IANA-form media type: lowercase, exactly one
  `/`, no parameters, each half starting with a letter or a digit, at
  most 127 characters (the bound of RFC 6838) of `a–z 0–9 . + - _ /`:
  `video/mp4`, `application/ttml+xml`. Fields that take a media type
  follow the rules below, as tokens do.
- **In a fact** (what `titles` says of a title, what an answer says): a
  token or a media type the receiver doesn't know is **kept** as it is.
  It is never turned into another value, and never dropped. A malformed
  value (of another JSON type, empty, too long, with other characters)
  is handled as any malformed member is: an optional member is dropped
  alone, and the entry that holds it stays; a required member is
  dropped with the entry that holds it (a copy, a stream, a track, an
  extra), as when it is missing.
- **In a request**: a value the receiver doesn't know is ignored where
  the field is a hint, and refused with a signed `409`
  `delivery-refused` where it names what to serve (a remux's media
  type, a transcode's codec). Only a malformed value gets `400`.
- A viewer's server that doesn't know a copy's `mediaType`, a stream's
  `codec` or its `threeD`, or can't parse a `codecs` string, treats the
  copy as one it can't play as it is: it asks for a `remux` or a
  `transcode` ([playback section 2.7](playback.md#27-delivery)), or
  doesn't offer the copy. An unknown subtitle `format` affects that
  track only. Other unknown values (`edition`, `role`, an `hdr` token,
  `objects`, `extraType`, a segment `kind`) don't stop the copy from
  playing as it is; the content parts say how some are shown.
- No list has a value meaning "other". A sender with no value for an
  optional field leaves it out.
- The values a part gives beside an open field are examples, not the
  list. The lists are the [registry](../registry.md)'s, or an outside
  vocabulary's (IANA media types, RFC 6381, ITU-T H.273, the DASH role
  scheme, MusicBrainz), which the part names.

Test cases: `openTokens` and `openMediaTypes` in
[vectors/v0/strict.json](../../vectors/v0/strict.json).

## 9. The core's calls

### 9.1 pair

`pair`, under the maker's base path. The server redeeming a code calls
the code's maker. Heard from strangers only while the maker has a live
code.

Request:

| Field | Type | Required | Content |
|-------|------|----------|---------|
| `address` | string | yes | the caller's own address (section 5.2) |
| `name` | string | no | the caller's name, for the maker's panel |
| `secretHash` | string | yes | base64url of SHA-256(secret): 32 bytes |
| `proof` | string | yes | the pairing proof, below |
| `nextCommitment` | string | yes | the caller's next key commitment (section 12.5): base64url of 32 bytes |
| `inbound` | boolean | no | `false`: the caller can't be called (section 7.5); absent or `true`: it can |

The proof is HMAC-SHA256, keyed with the 32-byte secret, over these bytes
in order:

1. the ASCII label `msfp-0 pairing\n` (it ends with a line feed);
2. the caller's 32 raw public-key bytes, the same key as in `MSFP-Key`;
3. a line feed, `\n`;
4. the `address` field's value, after JSON decoding, as bytes (an
   address is ASCII).

It is sent base64url (43 characters). The maker compares it in constant
time. The proof binds the secret to the caller's key and address: someone
who sees it go by can't reuse it with a key of their own.

Answer `200`:

| Field | Type | Content |
|-------|------|---------|
| `name` | string | the maker's name |
| `nextCommitment` | string | the maker's next key commitment (section 12.5): base64url of 32 bytes; required |

Both sides commit. The redeemer refuses an answer without
`nextCommitment`, or with one that isn't 32 bytes of base64url, as a
badly signed one (section 6.6): it stores nothing, and tells its
operator that the pairing failed.

Refusals:

- `404`, unsigned (signed when the key is already a peer): the key isn't
  heard, there's no live code with that hash, the code expired or its
  uses are spent, the proof
  is wrong, the body isn't valid JSON, the address is invalid, the hash
  isn't 32 bytes, `nextCommitment` is missing or isn't 32 bytes of
  base64url, `inbound`
  isn't a boolean, or `inbound` is `false` and the address isn't of the
  form of section 5.2 for a server that can't be called.
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

### 9.2 hello

`hello`, under the peer's base path. Either peer, in any state. It tells
the other where the pairing stands, which versions the sender speaks,
its pins and its address, and asks the same of the other.

Request and answer `200` carry the same members:

| Field | Type | Required in the request | Content |
|-------|------|-------------------------|---------|
| `state` | string | yes | the sender's view: `paired`, `waiting` or `ended` (in an answer: `ended` only when the request said `ended`) |
| `core` | array of integer | no | the core versions the sender speaks, at most 8 (section 10) |
| `parts` | object | no | for each part the sender speaks, the array of its versions, at most 8 each (section 10) |
| `certificates` | array of string | no | the pins of the certificates the sender's address serves (section 3.2) |
| `address` | string | no | the sender's address now (section 13) |
| `nextCommitment` | string | no | the sender's next key commitment, base64url of 32 bytes; it only repeats the one the receiver stores, and never changes it (section 12.5) |
| `inbound` | boolean | no | `false`: the sender can't be called (section 7.5); absent or `true`: it can |
| `individual` | boolean | no | whether the sender wants the individual relation (section 7.7); absent: what the sender last said |

A request and an answer may also carry members that go one way only:

| Field | Type | In | Content |
|-------|------|----|---------|
| `knownKey` | string | requests from a server that can't be called | the key the sender checks the receiver's answers against now (section 12.4) |
| `rotation` | string | requests from a server that can't be called | `pending` or `refused`: the sender hasn't moved to the receiver's new key (section 12.4) |
| `rotation` | object | answers to a server that can't be called | the receiver's rotation statement, with the members of the `rotate` request: `newKey`, `nextCommitment` (optional), `date`, `oldSignature`, `newSignature` (sections 9.3, 12.4) |
| `revocation` | object | answers to a server that can't be called | the receiver's revocation statement, with the members of the `revoke` request: `date`, `signature` (sections 9.4, 12.6) |
| `callsFailed` | boolean | answers | `true`: the answering server's last calls to the caller couldn't connect (section 7.5) |

A server that speaks version 0 sends `core` and `parts` in every `hello`,
request and answer (section 10.2). It should send `address` in every
`hello`. It leaves `certificates` out when it names none, and leaves out
everything but `state` when it sends or answers `ended`.

What the receiver does:

- `ended`: it deletes the caller (section 7.4), and answers `ended`.
- `paired`: if the caller is in state `theirs`, it becomes `paired`. If
  the caller is `gone` and was paired before, it becomes `paired`, unless
  it is suspended (section 12.6). Otherwise nothing changes.
- `waiting`, or any other string: nothing changes; the answer is as for
  `waiting`.
- A body without `state`, with a `state` that isn't a string, or with a
  member of the wrong type, is refused like a body that isn't JSON
  (signed `404`), and the caller's stored versions, pins and address stay
  as they were. So is a body with `inbound: false` whose address (its
  `address`, or the stored one when it has none) isn't of the form of
  section 5.2 for a server that can't be called, and one whose
  `nextCommitment` isn't 32 bytes of base64url, as in `pair`.

Then, unless the request said `ended`, the receiver:

- keeps the request's `core` and `parts` as the caller's versions
  (section 10.3);
- keeps the request's `certificates` as the caller's pins (section 3.2);
- keeps the request's `inbound` as the caller's (section 7.5), and its
  `individual`, when present (section 7.7);
- if `address` differs from the caller's stored address, starts the
  check of section 13.2, or, for a caller that can't be called, stores
  it at once (section 13.2); if it is the stored address again, drops a
  pending move (section 13.2, step 3);
- ignores `nextCommitment`, once its form is checked (section 12.5);
- if the caller can't be called, reads `knownKey` and `rotation` before
  it answers (section 12.4); from any other caller it ignores them;
- notes the time it heard from the caller;
- answers with its own view (section 7.3), versions, pins and address,
  and, to a caller that can't be called, its pending `rotation` and
  `revocation` (sections 12.4, 12.6).

What the caller does with the answer:

- `200`: it keeps the answer's versions, pins, address, `inbound` and
  `individual`, as
  the receiver does above, and ignores its `nextCommitment`. Any
  server applies a valid `revocation` found in an answer signed with the
  key it revokes (sections 12.6, 12.7), and keeps nothing else of that
  answer; a server that can't be called then applies the answer's
  `rotation` (section 12.4). A `callsFailed` that is `true` is shown on the panel, and nothing else
  (section 7.5); any other value counts as absent. A `rotation` or
  `revocation` that isn't an object counts as absent.

The one-way members are ignored where the table doesn't put them:
`knownKey` in an answer, `revocation` and `callsFailed` in a request. A
request's `rotation` other than `pending` or `refused` is ignored too. The
main table's members are type-checked in every request; `knownKey` and
`rotation` only in requests from a server that can't be called, where
one of the wrong type is refused like any other member, and from any
other caller they are ignored, whatever their value.
- `200` with `paired`: as the receiver would do with `paired` above.
- signed `404`: the peer doesn't know the caller, or has suspended its
  key. The caller marks it `gone`.
- An answer that fails (one that doesn't count by section 6.6, a body
  that isn't a JSON object, or an answer section 5.2 fails) changes
  nothing: the caller keeps nothing of it, and the peer's stored
  versions, pins, address and `inbound` stay as they were, as for a
  refused request.

Refusals: `404` (unsigned to unknown keys, signed to forgotten or
suspended ones, signed with its own key to the server's own key, section
6.4); a signed `404` for a bad body; `500`.

When a paired peer gets a signed `404` `not-found` on a part's call, it
sends a `hello` at once. The peer may have ended the pairing, or not have
heard yet that the pairing was accepted. A `404` `not-supported` doesn't
call for a `hello` (section 10.4).

Example, from [vectors/v0/exchanges.json](../../vectors/v0/exchanges.json):

```json
{"state":"paired","core":[0],"parts":{"catalog":[0],"playback":[0],"media":[0],"video":[0]},"certificates":["JN-hf1COiASAAUsiYeHXMNVvNnxYR2dKYnBT2Zx_6Fo"],"address":"https://a.example.org"}
```

### 9.3 rotate

`rotate`, under the peer's base path. A server changing its key from O to
N sends it to every peer, in any state, signed with O (section 6).
That includes the peers that suspended this server (its key revoked
there: a rotation is how that suspension lifts, section 12.6), and never
the peers this server suspended. Section 12 says when, and what each
side does.

Request:

| Field | Type | Required | Content |
|-------|------|----------|---------|
| `newKey` | string | yes | N, base64url |
| `nextCommitment` | string | no | the commitment to the key after N, base64url of 32 bytes |
| `date` | integer | yes | the statement's date, Unix seconds |
| `oldSignature` | string | yes | O's signature of the rotation statement (section 12.2) |
| `newSignature` | string | yes | N's signature of the rotation statement |

Answers:

| Status | Body | Meaning |
|--------|------|---------|
| `200` | `{"state":"rotated"}` | N is the sender's current key here |
| `202` | `{"state":"pending"}` | the receiver's operator must confirm N (section 12.3, case 4) |
| `409` `rotation-refused` | problem document | N doesn't match the stored commitment, N is another peer's key or a forgotten one, N is one of the sender's own former keys, the request is signed with the sender's previous key and N isn't its current key, or the sender is suspended and N doesn't match |
| `400` `invalid-request` | problem document | a bad body, a signature that doesn't verify, or a date more than 5 minutes from the receiver's clock |

Every answer is signed with the receiver's own key.

### 9.4 revoke

`revoke`, under the peer's base path. A server revoking its key K sends it
to every peer it can reach, signed with K (section 6). Section 12.6 says
what follows.

Request:

| Field | Type | Required | Content |
|-------|------|----------|---------|
| `date` | integer | yes | the statement's date, Unix seconds |
| `signature` | string | yes | K's signature of the revocation statement (section 12.6) |

Answers: `200` `{"state":"suspended"}`, also when the key was already
suspended; `400` `invalid-request` for a bad body, a signature that
doesn't verify, or a date more than 5 minutes from the receiver's clock.
Every answer is signed with the receiver's own key.

K may be the sender's previous key, in the 10 minutes it is still heard
after a rotation (section 12.4). The answer is then `200`
`{"state":"suspended"}` too, and means that K is suspended: from then
on K is not heard at all, for any call. The sender itself is not
suspended, since its current key is the new one.

## 10. Versions and parts

### 10.1 The parts

| Name | What it holds | Calls | Needs |
|------|---------------|-------|-------|
| core | encodings, transport, keys, the code, signatures, pairing, states, rotation, addresses, versions | `pair`, `hello`, `rotate`, `revoke` | — |
| `catalog` | libraries, titles, the general rules of title keys, the description and image of `local` titles | `libraries`, `titles`, `image` | the core |
| `playback` | starting, keeping and stopping a playback; limits | `play`, `progress`, `stop` | `catalog` and `media` |
| `media` | what the stream URLs serve: HLS, the file by byte ranges, subtitles, fonts, sheets and images; the kinds of ticket | plain `GET`s under the stream root | the core |
| `video` | films, series and extras by TMDB, TheTVDB or the owner's own keys: title keys, episode order, copies, subtitles (WebVTT, ASS, PGS), fonts, skip markers, previews | `previews`; it adds to `titles`, `play` and the media | `catalog`, `playback` and `media` |
| `music` | artists, albums and tracks by MusicBrainz or the owner's own keys: keys, descriptions, copies, cover art, lyrics, audio-only playback, queues | `lyrics`; it adds to `titles`, `play`, `progress`, `stop` and `limits` | `catalog`, `playback` and `media` |
| `groups` | groups of servers: events, mail, pairing through a group | `group-sync`, `group-push`, `group-pair`, `group-join` | the core |

The core and each part have their own version, an integer. Version 0 of
each is this document and [catalog.md](catalog.md),
[playback.md](playback.md), [media.md](media.md), [video.md](video.md),
[music.md](music.md) and [groups.md](groups.md).

Rotation and revocation (`rotate`, `revoke`, section 12) are part of
the core, not a part: a server without them doesn't speak core 0. Every
pairing must be able to cut off a stolen key.

`playback` plays title keys from `titles` and points at media URLs, so it
needs both other parts. The `limits` member of `libraries` belongs to
`playback`: a server sends it only to a peer it speaks `playback` with.

The content parts say what titles are, which of them `play` takes and
what media they add, so each needs all three. Every content part is
optional: a server lists in `hello` the ones it handles, and two servers share only titles of the
content parts both list
([catalog section 5](catalog.md#5-what-is-shared)). A receiver skips
keys of a content part it doesn't speak
([catalog section 3.3](catalog.md#33-what-the-caller-keeps-of-each-title)).

### 10.2 Announcing versions

Versions travel in `hello`, request and answer, in `core` and `parts`
(section 9.2):

```json
{"state":"paired","core":[0],"parts":{"catalog":[0],"playback":[0],"media":[0,1],"video":[0],"music":[0]}}
```

- **Absent means none.** A `hello` without `core` leaves no common core:
  the sender doesn't speak MSFP 0. Without `parts`, the sender speaks no
  part: only the core is on.
- **Part names** are lowercase ASCII letters and `-`, at most 32
  characters. Unknown names are ignored, whatever their value (`null`
  included), so that a later version may announce a part in another
  shape.
- **Lists.** At most 8 versions each, each a non-negative integer. A
  receiver ignores entries beyond 8 and entries that aren't such
  integers. An empty list means the sender doesn't speak that part.
- **Duplicates** in a list count once. A `core` or a known part's list
  that isn't an array, or a `parts` that isn't an object, is a bad body
  (section 8.2): the whole `hello` is refused, nothing is stored. An
  empty `core` list leaves no common core.
- A server lists every version it speaks, not only the highest. A server
  that speaks cores 0 and 1 lists `[0,1]`.

### 10.3 Choosing

- **Choice.** For the core and for each part, the version used between
  two servers is the highest one both list. No common version: that part
  is off between them.
- **Dependencies.** A part whose dependency is off is off too.
- **The core.** No common core version: the servers can't talk; the
  panel says so. Addresses are `https` only (section 3). To a peer with
  which it has no common core, a server answers every call but `hello`
  with a signed `404` `not-supported` (section 10.4), and makes no call
  to it but `hello`. A `pair` from a key it already knows gets its usual
  `409` (section 9.1): `pair` is answered before versions are known.
- **Memory.** Each side stores, per peer, what the peer's latest signed
  `hello` (request or answer, latest by arrival) listed, and recomputes
  the choices. Before any `hello`, it assumes no part is on, and the
  core of the code the pairing started with (section 7.1). The
  parts are used only by `paired` peers, and a pairing becomes `paired`
  through a `hello` exchange, so the versions are known by then.

### 10.4 A part that is off

- The caller must not make that part's calls to the peer, and must not
  show its people anything that needs them. For example, it doesn't offer
  a peer's titles for playing when `playback` is off.
- A receiver answers a call of a part it doesn't speak with the caller
  with a signed `404` problem of type `not-supported`; so it answers
  every call but `hello` from a peer with no common core (section 10.3).
- A caller must not take that `404` as "gone" or "not paired". It
  recomputes the choices from a fresh `hello`, at most once an hour per
  peer.

### 10.5 Which core a message uses

A message's core shows in its labels and headers: core 0 uses
`msfp-0 …`, `MSFP-*` and the code prefix `msfp-pair-0.` (section 15). A
server answers in the core of the request.

## 11. The discovery document

A server may serve, at `GET /.well-known/msfp` on its address (RFC 8615),
an `application/json` document:

```json
{"core":[0],"parts":{"catalog":[0],"playback":[0],"media":[0],"video":[0]},"base":"/srv/media/api/v1/federation/"}
```

| Field | Type | Content |
|-------|------|---------|
| `core` | array of integer, optional | as in `hello` |
| `parts` | object, optional | as in `hello` |
| `base` | string, optional | where the federation calls are, as an absolute path; absent: `/api/v1/federation/` |

- **Off by default.** A server serves the document only when its operator
  switches it on: a server that serves it tells anyone that it federates,
  which section 14 ("Probing") otherwise avoids. A server that serves
  none answers that path like any path that doesn't exist, and its peers
  use the default base path, `/api/v1/federation/`.
- **`base`** is at most 256 bytes, ASCII, an absolute path made of
  `/`-separated segments of `A–Z a–z 0–9 . _ ~ -`, and ends in
  `/api/v1/federation/`. No segment is empty, `.` or `..`, and there is no
  percent-encoding at all, so the path means the same before and after
  any normalization. Test cases: `bases` in
  [vectors/v0/strict.json](../../vectors/v0/strict.json). The part
  before that ending is the server's **prefix** (`/srv/media` in the
  example; empty by default). The media's stream root is
  `<address><prefix>/api/v1/stream/` ([media](media.md#1-the-urls)).
  A document whose `base` breaks this counts as no document.
- **A server under a base path** (a media server reached at
  `/media`, say) has a prefix, and its peers learn it only from this
  document, at the address's root: an address has no path (section 5.2).
  Such a server switches the document on, and has it served at the root
  by whatever serves the root (its reverse proxy, when the media server
  serves nothing there). Without it, its peers call the default base and
  get nothing.
- **It proves nothing.** The document is not signed. It lets a server
  find a peer's API and see, before pairing, whether they have a version
  in common. The versions used are always those of the signed `hello`
  (section 10.3).
- **Fetching it.** A server fetches a peer's document when it learns the
  peer's address: before calling `pair` on a code's address, after
  storing a redeemer's address, and after an address change (section 13);
  never for a peer that can't be called (section 7.5).
  The fetch is a plain `GET` with no signature headers, any certificate
  accepted (section 3.2), no redirects followed, and a body of at most
  16 KiB. A `404`, or any answer that isn't a valid document, means the
  default base. So does an answer whose media type (section 2.4) isn't
  `application/json`: it is not a valid document. The server stores the
  base with the peer.
- **Fetching it again.** After 3 calls in a row to the peer that got an
  answer not signed by the peer (an unsigned `404`, say), the server
  fetches the document again, at most once an hour per peer. A fetch that
  gets no answer at all keeps the stored base.
- With discovery, a server hosted under a path can federate: its address
  stays `https://host[:port]` (section 5.2), and `base` gives the path.
  Section 6.2's `<target>` is the full path as sent.

`msfp` is to be registered in IANA's Well-Known URIs registry when the
specification reaches a stable version.

## 12. Key rotation and revocation

A server's rotation and revocation statements also travel in the lists
of the groups it is in ([groups section 3.3](groups.md#33-the-types),
`rotated` and `revoked`), so that members it isn't paired with follow its
key.

### 12.1 Why

A server is its key. Without rotation, moving to a new machine, restoring
from a backup made by another tool, or following a key-lifetime policy
would mean every peer pairs again by hand; and a stolen key would have no
answer but each peer's operator ending the pairing. This section lets a
key change, and lets a server tell its peers that a key is compromised.

### 12.2 The rotation statement

A server that rotates from key O to key N makes this statement: five
lines joined by `\n`, no trailing `\n`, ASCII.

```
msfp-key-rotation-1
<O, base64url>
<N, base64url>
<the commitment to the key after N, base64url, or "-">
<date, Unix seconds, decimal>
```

It signs the statement with O (`oldSignature`) and with N
(`newSignature`), under section 2.1's rules. The second signature proves
the server holds N, so nobody can rotate a server to a key someone else
holds. N must be a valid key (section 2.1), and not O.

### 12.3 Receiving a rotation

The receiver of a `rotate` (section 9.3), signed with O:

1. rebuilds the statement from the request (O from `MSFP-Key`, `-` when
   `nextCommitment` is absent), and checks both signatures, N under
   section 2.1 too. It checks that the date is within 5 minutes of its
   clock. It refuses (`409` `rotation-refused`) when N is a key it knows
   as another peer, or has forgotten, or is one of this peer's former
   keys (its previous key, or any key of its rotation statements): a
   retired key never becomes current again.
2. **If it holds a commitment for that peer, and SHA-256(N) matches
   it**, it makes N the current key at once, and answers `200` `rotated`.
3. **If it holds a commitment and N doesn't match**, it refuses the
   rotation (`409` `rotation-refused`), keeps O, and shows its operator a
   warning: someone holding O tried to move the pairing to another key.
4. **If it holds no commitment** (after a rotation statement with `-`:
   `pair` always carries one, section 9.1), it keeps O, marks the
   rotation to N pending, and asks its operator to confirm N's
   fingerprint with the other operator, as in pairing (section 7.1,
   step 6). It answers `202` `pending`. Once its operator confirms, N
   becomes the current key, and it sends the rotating server a `hello`
   at once. That call makes the rotating server send `rotate` again
   (section 12.4), which is answered `200` `rotated`. The `hello`'s own
   answer may still be signed with O, and then doesn't count
   (section 6.6). A server that can't be called confirms by its next
   `hello` with `knownKey` N, as section 12.4 says.
5. When N becomes current, it stores the statement's commitment as the
   new commitment (or none, for `-`), keeps the statement and its two
   signatures in the peer's history, and keeps O as the previous key
   (section 12.4).

A peer that is suspended (section 12.6) is handled by case 2 only: a
rotation to a key matching the stored commitment lifts the suspension and
rotates; any other is refused (`409` `rotation-refused`). After a
compromise, only the committed key, or a new pairing, can restore trust.

A `rotate` to N after N became current, still signed with O within the
10 minutes of section 12.4, is answered `200` `rotated` again and changes
nothing, so a server can retry safely. A `rotate` signed with O to any
other key, then, is refused (`409` `rotation-refused`): the peer's
current key is N. A pending rotation stays pending through repeated
`rotate`s to the same N; a `rotate` to another N replaces it.

A rotation changes nothing else: address, name, state, offers, wants,
limits, pins and catalogs stay.

A server that can't be called receives a rotation in a `hello` answer
instead of a `rotate` (section 12.4), and handles it by these same steps.

### 12.4 What each side does during the change

**The rotating server** keeps O until every peer has rotated, or until
30 days have passed, whichever comes first:

- toward a peer that hasn't answered `rotated`, it signs requests and
  answers with O, and sends `rotate` again, with a newly dated statement,
  at each contact and at least daily (to a peer that can't be called, see
  below). A contact is any exchange with that peer, a request either way
  or an answer, and gives at most one `rotate` a minute. When a peer whose
  rotation is pending (it answered `202`) calls it, it sends `rotate` at
  once, at most every 10 seconds: that is how a confirmation reaches it
  (section 12.3, case 4);
- it sends no `rotate` to a peer it suspended itself (section 12.6);
- toward a peer that answered `rotated`, it signs with N only;
- when a request signed with O gets an unsigned `404` from a peer it sent
  `rotate` to (the answer to a `rotate` may have been lost, and O no
  longer heard there), it sends that peer a `hello` signed with N; a
  signed answer means the peer has rotated;
- after 30 days, it deletes O. A peer still on O must pair again; the
  rotating server's panel says which, and why.

Two key changes in a row: the first is finished before the second
starts. Starting a rotation from N while O is still kept deletes O at
once, and the peers still on O are lost, as after the 30 days: they must
pair again.

Toward a peer that doesn't speak core 0 (section 10.3), no rotation is
possible: the rotating server keeps O for that peer only, for as long as
the pairing lasts, or until its operator pairs again. Holding O longer is
safe if O isn't compromised.

**The receiving server**, after N became current, still hears requests
signed with O for 10 minutes (requests already on the way), and checks
answers from that peer against N only. A revoked O isn't heard in those
10 minutes (sections 9.4 and 12.6).

**Toward a peer that can't be called** (section 7.5), the rotation is
pulled. When Q rotates from O to N and P can't be called:

1. Q's `hello` answers to P carry `rotation`, an object with the members
   of the `rotate` request (section 9.3): `newKey`, `nextCommitment`
   (optional), `date`, `oldSignature`, `newSignature`. Each answer carries
   a newly dated statement, as repeated `rotate`s do. The answer itself
   is signed with O.
2. P handles it as section 12.3 handles a `rotate` signed with O: the key
   the answer verified under is O. The outcome is what P would have
   answered: rotated, pending, or refused. A `rotation` that `rotate`
   would refuse with `400` (a bad member, a signature that doesn't
   verify, a date more than 5 minutes from P's clock) changes nothing.
3. P's `hello` requests carry `knownKey`: the key P checks Q's answers
   against, now. After P makes N current, its next request to Q is a
   `hello` with `knownKey` N, sent at once. If the rotation is pending or
   refused, P's `hello`s carry `knownKey` O and `rotation` with the string
   `pending` or `refused`. P sends `refused` as long as it still shows
   its operator the refused key (section 12.3, case 3).
4. Q reads `knownKey` before it answers. N: Q treats P as having answered
   `rotated`, and signs toward P with N only, that answer included. O:
   nothing changes, and Q's panel shows `pending` or `refused` when P sent
   it.
5. P checks each answer against the key it knew Q by when it **sent** the
   request. So answers to calls already on their way when it made N
   current, still signed with O, verify.
6. Q's 30-day limit is unchanged.

`knownKey` and `rotation` are ignored from a peer whose `inbound` isn't
`false`, and `knownKey` is ignored when it is neither of Q's keys.

### 12.5 The next key commitment

- A server keeps a next key ready: a key pair made in advance, not used
  for anything yet. Its commitment is SHA-256 of its 32 public-key bytes,
  base64url (43 characters).
- `pair` requests and answers must carry `nextCommitment`
  (section 9.1): both sides commit. Each side stores the other's. A
  `pair` request without it is a bad body; an answer without it fails.
- A commitment is set only by `pair` or by a rotation statement. `hello`
  can't change one. Otherwise a thief holding the current key could
  replace the commitment, then rotate. There is no trust on first use
  from `hello`.
- A `hello` may carry `nextCommitment`, and then only repeats the
  commitment the receiver stores. A sender may send it in every `hello`.
  The receiver checks its form (32 bytes of base64url; any other is a bad
  body, as in `pair`), and otherwise ignores it, whether it matches the
  stored one, differs from it, or the receiver stores none (after a
  rotation statement with `-`).
- A server keeps the seed of its next key apart from the current one. It
  should let its operator export it (as text to keep offline) and then
  delete it from the server. With the seed offline, a thief who copies the
  server's storage can't rotate. Next to the export, the panel should show
  the fingerprint of the committed key (section 4.2), so the operator can
  check that the exported key is the committed one. That fingerprint is
  made from the first 15 bytes of the commitment (section 4.2, steps 3
  and 4), so the panel shows it while the seed is offline too.
- The exported text is `msfp-next-key-1.` followed by the seed's 32
  bytes in base64url (section 2.2), so a key exported by one
  implementation can be used by another. A reader removes white space,
  refuses any other prefix, and checks that the seed's public key matches
  the stored commitment before it uses it.
- After a rotation, the server makes a new next key, whose commitment is
  the one in the statement.

### 12.6 Revocation

A server revoking its key K makes this statement: three lines joined by
`\n`, no trailing `\n`, ASCII.

```
msfp-key-revocation-1
<K, base64url>
<date, Unix seconds, decimal>
```

It signs the statement with K, and sends it in `revoke` (section 9.4),
signed with K, to every peer it can reach. Anyone holding the key can
send it, and gains nothing by it: a thief could already misuse the
pairing, and revoking only stops it.

**Toward a peer that can't be called** (section 7.5), the revocation is
pulled. The revoking server's `hello` answers to such a peer carry, for
30 days, `revocation`: `date` and `signature`, as in `revoke`, the
statement newly dated in each answer. Only answers signed with K carry
it: once the peer knows the server by another key (section 12.4), it no
longer needs it. The peer checks it as below, with K the key the answer
verified under, and suspends the server. The delay is at most one
`hello` period (5 minutes, give or take a fifth), where a `revoke` takes
one call.

What the receiver does, after checking the statement's signature and its
date (within 5 minutes):

- It marks the peer suspended. It answers no call from that key except
  `rotate` and `revoke`: `hello` and every other call get the signed
  `404` of a forgotten key. A revoked key is never heard again for any
  other call: when a rotation lifts the suspension, the revoked key gets
  no 10 minutes as a previous key (section 12.4). Its own calls to that
  peer stop, and its people are shown nothing that needs them. A server
  that can't be called keeps only its scheduled `hello` (section 7.5),
  so that a rotation can still reach it: of those answers it reads only
  `rotation`, and accepts only one that matches the stored commitment
  (section 12.3, case 2). When the suspended server revoked and rotated
  in one action (section 12.7, case 1), one answer carries both: it
  applies `revocation`, then `rotation`.
- It shows its operator a warning.
- A `rotate` from the suspended key to a key matching the stored
  commitment lifts the suspension and rotates (section 12.3).
- Its operator can end the pairing (section 7.4) at any time.
- After 30 days suspended without a valid rotation, the peer becomes
  `gone`, as if it had answered that it doesn't know us, and the
  operator is told. 30 days is the rotating server's own overlap
  (section 12.4). It stays suspended: a `paired` `hello` doesn't bring it
  back (section 9.2), since otherwise a thief holding the revoked key
  could revive the pairing.
- A revocation of a peer's previous key, in the 10 minutes it is still
  heard (section 12.4), doesn't suspend the peer: that key stops being
  heard at once, and the peer stays known by its current key
  (section 9.4).

### 12.7 What to do on compromise

1. If the next key's seed is safe (offline): the operator revokes the
   current key and rotates to the next key, in one action. Peers that
   hold the commitment follow at once.
2. If it isn't, or no commitment exists: the operator revokes the key,
   then pairs again with each peer, with new codes and fingerprint calls.
   Revoking first stops the thief at once at every peer the server can
   reach. This is the one exception to section 4.1: the server makes a
   new key, not by a rotation, and every pairing ends. It deletes its
   peers and records their keys as forgotten (section 7.4, step 1); it
   sends no `ended`, which its peers would refuse from a suspended key.
   For 30 days, it keeps the revoked key K for one use: a `hello` from a
   peer whose pairing ended in this replacement (not one its operator
   ended before), signed for its address, gets a `200` answer signed with
   K that carries only the revocation,
   `{"revocation":{"date":…,"signature":…}}`, newly dated in each
   answer, instead of the signed `404` of a forgotten
   key. So a peer that can't be called, and any peer `revoke` didn't
   reach, learns of it and suspends the pairing. After the 30 days, such
   a peer's calls get the plain unsigned `404` of an unknown key
   (section 6.4), not one signed with the new key, which it can't know;
   its usual rules for that answer then apply (the retry of section 6.5,
   and `gone` by section 9.2).
3. In both cases, the operator makes a new next key and stores it
   offline.

## 13. Changing a server's address

A peer's address is where to call it, and its host is in every request
signed for it (section 6.2). This section lets an address change (a new
name, a new port) without pairing again.

### 13.1 The server that moves

A server whose address changes from A1 to A2:

1. accepts requests whose signed host is A1's host or A2's host, for
   30 days, or until every peer has moved (it knows a peer has moved when
   that peer's requests are signed for A2's host). Every peer counts but
   those that are `gone`; a peer that never calls counts as not moved
   until the 30 days end. The 30 days run from when A2 is reached
   (step 2): if it never is, A1's host stays accepted;
2. announces A2 only once A2 answers: it checks that by a request of its
   own to A2 whose answer it can recognise as its own (A2 must reach
   this server, not another one: the `404` signed with its own key of
   section 6.4), retrying until it does; until then it
   names A1. Then it
   sends `hello` with `address` A2 to every peer, at once and then at
   each contact; a peer that can't be called (section 7.5) learns it by
   step 3, and confirms it by a `hello` at A2, which it can make;
3. names A2 in its `hello` answers too, including those given on A1, so a
   peer that calls first also learns;
4. names, in its `hello`s, the pins of the certificates of both A1 and
   A2 (section 3.2), until the move ends;
5. builds the stream URLs of a `play` answer
   ([media section 1](media.md#1-the-urls)) on the address whose host
   that `play` request was signed for, A1 or A2;
6. after the 30 days, accepts A2's host only.

A move to or from an address of the `.invalid` form of section 5.2
keeps no former address: the moving server accepts the new host only,
and its peers keep no former address either (section 13.2).

Accepting two hosts for a while doesn't weaken section 6.2's goal: a
request signed for one server still fails at every other server, since no
other server has A1 or A2 as its own.

Its panel should tell its operator, when the configured address changes,
that peers will be told and how many have moved, and warn when a peer
hasn't moved after some days. Toward a peer that doesn't speak core 0,
the moving server may keep accepting A1's host longer, at its operator's
choice, or the operators pair again.

### 13.2 A peer that learns of a move

What a server does with a signed `hello`, request or answer, whose
`address` differs from the one it stores for that peer:

1. It checks the address against section 5.2. An address that breaks it
   is ignored, and the stored one stays.
2. It fetches the peer's discovery document at the new address
   (section 11), then calls `hello` there, signed for the new host. If
   the answer is a `200` that verifies under the peer's current key, it
   stores the new address and base; a signed `404` there confirms
   nothing. This proves the key holder actually answers there.
   If that answer names yet another address, the confirmed one is stored
   all the same, and the third waits for the peer's next `hello`.
3. If that `hello` fails, it keeps the old address and tries again
   later, at most once an hour, and shows its operator a warning after a
   day of failures. A signed `hello` from the peer that names the stored
   address again drops the pending move, and its retries.
4. It never moves to an address that is only in an unsigned message, or
   in an answer it couldn't verify.

When the stored address changes, the pins learnt from the old address
are dropped; the confirming `hello`'s answer gives the new ones. Playbacks
in progress keep their old URLs until they end.

**A peer that can't be called moves.** When the peer's `inbound` is
`false` (section 7.5), the server stores the new address from the peer's
signed `hello` at once, without the confirming call of step 2: that call
proves the key holder answers at the address, which matters only for an
address that is called. It drops nothing, since it holds no pins of the
peer's, and keeps no former address. If the peer later sends
`inbound: true`, the confirmation of section 7.5 happens then. A server that turns
"can't be called" on moves this way to its address of section 5.2's
form; one that becomes reachable announces its real address in a `hello`
without `inbound: false`, once it has reached that address itself
(section 7.5, "Turning it off"), and is confirmed by step 2. A move to or from
an address of that form keeps no former address (section 13.1).

## 14. Security considerations

- **Trust is the key, nothing else.** A server is its public key. A name,
  address or certificate never identifies it. The pairing code carries
  the maker's key; the operators' fingerprint check catches a code
  changed on the way. Skipping that check leaves only the secrecy of the
  channel the code travelled on.
- **Strict Ed25519.** The point checks of section 2.1 close the identity
  key, under which one signature verifies every message, and make every
  implementation agree on every signature. That matters for statements
  that more than one server checks (section 12).
- **The code is a secret.** Whoever holds a live code can ask to pair
  with its maker, until the maker's operator refuses them. It works for
  the number of uses and the time its maker set (once, for 24 hours, by
  default). The secret never travels: only its hash and an HMAC bound to the
  redeemer's key and address.
- **Consent on both sides.** The redeemer's operator consents by pasting
  the code. The maker's operator accepts after the fingerprint check.
  Neither server can move the other to `paired`.
- **Through a group**, the inviter's and the owner's signatures stand in
  for the fingerprint check between members that pair through it, and
  consent is a request and its acceptance
  ([groups section 11](groups.md#11-security-considerations)).
- **Replay.** The nonce, the 5-minute window and the restart rule
  (section 6.5) stop replays. A request names its receiver's host, so it
  can't be sent to another server. Answers name the request's nonce, so an
  answer can't be swapped for another. Statements carry a date checked
  within 5 minutes, and travel inside signed requests.
- **HTTPS only.** Titles, names and tickets never travel in the clear,
  and media can't be changed on the way without breaking TLS. Media is
  still not signed: TLS is all that protects it (section 18).
- **Certificates (section 3.2).** An attacker on the path who presents a
  certificate of its own sees `pair`, `hello` and the discovery document
  only: it can't change the first two, since both ways are signed, nor
  have its certificate named, since pins come only in signed `hello`s and
  a `hello` can't be replayed. Every other call, and the media, refuses
  its certificate. Removing pins needs the peer's key, and only makes
  calls fail: pins never refuse a trusted certificate. What stays open: a
  certificate a public authority wrongly issued passes, as for any site;
  and a server that reads its own certificate at its own address names
  whatever it is served there, so someone who can redirect that
  connection (the server's DNS, say) can have their certificate named.
  Pins set by the operator avoid that.
- **Split-horizon names.** A server that is served another certificate
  inside its network than outside, and reads its own certificate, names
  the inside one, and its peers are refused on every call that needs
  *trusted* or *named*. Nothing is opened to an attacker, but calls fail;
  such a server sets its pins by hand.
- **A stolen key.** Its holder is the server toward every peer, until
  the real server revokes it (section 12.6). With a commitment whose seed
  is offline, the real server rotates and its peers follow; the thief
  can't rotate them elsewhere, since a rotation must match the
  commitment. Without one, a rotation needs both operators, as pairing
  does. A thief can't replace a commitment through `hello`. It can name
  any certificate, and announce an address, but a peer moves only after
  the new address answers under the key (section 13.2), which the thief
  holds: so a stolen key can redirect a peer, until it is revoked.
- **Discovery.** A served discovery document tells anyone that the server
  federates; it is off by default. Its content is unsigned and only finds
  the API: versions come from signed `hello`s, and a wrong `base` only
  makes calls fail.
- **Unsigned refusals.** Strangers get unsigned refusals. An attacker on
  the path can fake those, which spoils one pairing attempt. A known
  peer's refusals are signed, and unsigned ones change nothing.
- **Probing.** A server should answer unknown keys with the same `404` as
  a missing path (section 3), so nobody learns that federation is on
  without a code or a discovery document.
- **A compromised peer** can't reach the other's people or data beyond
  what was shared. The parts say what it can still do.
- **Hostile answers.** Treat everything a peer sends as untrusted input.
  The parts say what to check.
- **Clocks.** Keep clocks set by the network. A clock off by more than 5
  minutes stops all calls, rotations and revocations. A clock behind
  delays calls after the other's restarts.
- **Peers' addresses** point where the peer chooses. Section 3.3 keeps
  connections out of the server's own machine and its cloud's metadata
  service.
- **A server that can't be called** (section 7.5). Its peers open no
  connection to it, and its address can't resolve (section 5.2), so a
  peer that ignores `inbound` connects nowhere, and never to a LAN
  address on its own network. `inbound` is its operator's setting: a
  peer's `callsFailed` only puts a message on a panel. A `knownKey` can
  only pick between the peer's own two keys, and only the server that
  can't be called can send it (the request is signed by it), so it gives
  nothing to anyone else. A revocation reaches it within one `hello`
  period instead of one call. Its scheduled `hello`s show each peer its
  network address and that it is up, every 5 minutes, as any call it
  makes does.
- **Overload.** Section 8.3 limits a peer that calls too much.

### 14.1 Logging

Logs, diagnostics, error messages and crash reports are read by more
people, and kept longer, than the secrets they could hold. A server must
never write any of these into them, whole or in part:

- a ticket, or a URL that holds one, such as a media URL
  ([media section 1](media.md#1-the-urls)). HTTP clients' error messages
  often quote the URL: remove it first;
- a pairing code, its secret, the secret's hash, or the pairing proof
  (section 5);
- a private key or its seed, a key kept for a later rotation, or anything
  derived from them but the public key, the fingerprint and the next key
  commitment (sections 4 and 12);
- a request's or an answer's body as a whole, since it may hold any of
  these. Fields known to be harmless (a peer's name, a title key, a status,
  a problem `type`) may be logged.

A server may log public keys, fingerprints, addresses, playback IDs,
nonces, statuses and problem types. A playback ID isn't a credential: the
ticket is.

## 15. Versioning

This document is core version 0. The policy for new versions of the core
and of the parts, and what needs one, is in
[README.md](README.md#versioning). In short: a change to keys,
signatures, the code or pairing is a new core version, with new labels; a
new optional member needs no new version as long as its absence keeps
the old behaviour.

The strings `msfp-0` and `msfp-pair-0.` contain a `0`: they are core 0's
labels. The statements' labels `msfp-key-rotation-1` and
`msfp-key-revocation-1` are versioned on their own: a change to a
statement's form changes its label.

## 16. Coexisting with the test draft

Removed on 2026-11-17, with every rule that existed only for it:
version 0 doesn't work with the test draft ([history](README.md#history)).

## 17. Test vectors

The values below are in [vectors/v0/](../../vectors/v0/), for an
implementation's tests to check against. They come from the generator in
[tools/vectors/](../../tools/vectors/), not from an implementation. The
generator also checks them.

| File | What it holds |
|------|---------------|
| `keys.json` | the keys "a" and "b" (the same as in the test draft's vectors), server A's next keys "a2" and "a3", server B's "b2", with their fingerprints and commitments; two certificate keys with their pins |
| `code.json` | the code of section 5.3, its secret hash, and the pairing proof of redeemer "b" at `https://192.0.2.10:8096` |
| `exchanges.json` | `pair` (B calls A, with commitments), `hello` (A to B, with `core`, `parts`, `certificates` and `address`; B answers without `playback`), a refused `hello` (signed `404`), and a `play` refused with `not-supported`; then B can't be called (section 7.5): its `hello` with `inbound: false`, its new address and `knownKey`, A's answer carrying `rotation` and `revocation` objects (A revokes "a" and rotates to "a2" in one action, sections 12.4, 12.6 and 12.7), and B's next `hello` with `knownKey` "a2", answered under "a2" |
| `rotation.json` | A rotates from "a" to "a2", committing to "a3": the statement, both signatures, and the `rotate` exchange |
| `revocation.json` | A revokes "a": the statement, its signature, and the `revoke` exchange |
| `strict.json` | addresses and codes with their verdicts under core 0 (`https` only) |
| `strict.json`, `destinations` | IP addresses a server connects to for a peer by default, or refuses (section 3.3) |
| `strict.json`, `uncallableAddresses` | addresses of a server that can't be called, `https://msfp-<26>.invalid`, with their verdicts (section 5.2) |
| `strict.json`, `languageCodes` | media servers' language codes and the tag sent for each, or none ([catalog section 3](catalog.md#3-titles)) |
| `strict.json`, `openTokens` | values of open fields, kept as tokens or dropped as malformed (section 8.5) |
| `strict.json`, `openMediaTypes` | values of media-type fields, kept as media types or dropped as malformed (section 8.5) |
| `strict.json`, `helloVersions` | `hello` bodies and the versions a receiver keeps of them, or a bad body: `"core":[0,2.5]` lists core 0, and an unknown member or part of `null` is ignored (sections 2.3 and 10.2) |
| `media.json` | WebVTT parts and what rebuilding them gives, HLS playlists and PGS parts, with their verdicts ([media section 3](media.md#3-what-is-fetched), [video section 6](video.md#6-media)) |
| `music.json` | music title keys and music titles, with their verdicts and what the caller keeps of each title ([music sections 2 and 3](music.md#2-title-keys)) |
| `video.json` | video titles, with their verdicts and what the caller keeps of each, `video` included ([video section 3](video.md#3-titles)) |
| `groups.json` | a group's events, valid and not, with their reasons and the state a copy computes; a code with a group; a request mail sealed with HPKE, with the recipient's key to open it ([groups](groups.md)) |

The cases of [vectors/history/test-draft-strict.json](../../vectors/history/test-draft-strict.json)
(keys, signatures, base64url, dates, integers, JSON) hold for core 0 too.
Its `integers` cases apply where an integer is expected (section 2.3):
`1.0` is refused there, but is valid I-JSON, as its `json` cases show,
and a version list ignores it.

Seeds are the 32-byte Ed25519 seeds of RFC 8032, in hex:

| | Key "a" (server A, `https://a.example.org`) | Key "b" (server B, `https://192.0.2.10:8096`) |
|---|---|---|
| seed | `000102030405060708090a0b0c0d0e0f101112131415161718191a1b1c1d1e1f` | `202122232425262728292a2b2c2d2e2f303132333435363738393a3b3c3d3e3f` |
| public key | `A6EHv_POEL4dcN0Y50vAmWfk1jCbpQ1fHdyGZBJVMbg` | `Kay64UG8yvCyLhqU000LxzYeUm0L_hLIl5S8kyKWbdc` |
| fingerprint | `KZDV-VJ2U-MNDU-YAUF-35O3-6K6K` | `ET3O-22WL-7YIA-TQBQ-27FF-M7BT` |

The code of section 5.3 has the secret
`404142434445464748494a4b4c4d4e4f505152535455565758595a5b5c5d5e5f`. Its
`secretHash` is `yipP5yf6rs8W7NEwqG4IhcVUDAU3U0BEUHHAZXVV_UI`, and the
proof of redeemer "b" at `https://192.0.2.10:8096` is
`KdOcK6rkzzfX8MiXMaFn_9ofUWN96vJ1SciOG9Fjv7M`.

### 17.1 hello: A to B

A signs at `1800000060`, with the nonce bytes
`707172737475767778797a7b7c7d7e7f`. The receiver's host is
`192.0.2.10:8096`.

```
POST /api/v1/federation/hello HTTP/1.1
Host: 192.0.2.10:8096
Content-Type: application/json
MSFP-Key: A6EHv_POEL4dcN0Y50vAmWfk1jCbpQ1fHdyGZBJVMbg
MSFP-Date: 1800000060
MSFP-Nonce: cHFyc3R1dnd4eXp7fH1-fw
MSFP-Signature: 7L_kGyNoMvsmRc3ysOpYEHI4nmqblOIgxjfais5B78FiYXkAcSzhkSFtXiRt9Eg9Zl32mGAqlqPWj9sjioeoDg
```

Body (exact bytes, no trailing line feed):

```
{"state":"paired","core":[0],"parts":{"catalog":[0],"playback":[0],"media":[0],"video":[0]},"certificates":["JN-hf1COiASAAUsiYeHXMNVvNnxYR2dKYnBT2Zx_6Fo"],"address":"https://a.example.org"}
```

Signed message:

```
msfp-0 request
POST
192.0.2.10:8096
/api/v1/federation/hello
1800000060
cHFyc3R1dnd4eXp7fH1-fw
335db782ff11842ca0b4873401edfe2063791c8a9c4bb20a1a6b3325a965c714
```

B answers `200`, `Content-Type: application/json`, with the body:

```
{"state":"paired","core":[0],"parts":{"catalog":[0]},"certificates":["E7UUIjYbkigLNKPjmSGixyA-pslmiFKckCEX_A6IZlM"]}
```

B speaks `catalog` but not `playback` or `media`: between A and B,
`catalog` 0 is on, `playback` and `media` are off, and so is `video`,
which needs them and which B doesn't list. B can list its libraries to
A, and shares no titles. The signed message:

```
msfp-0 response
cHFyc3R1dnd4eXp7fH1-fw
200
application/json
7149e8f2222ae23fb93c91403e6003582967dcb11ded3b902a2c50fdd4011faa
```

and the header, signed with key "b":

```
MSFP-Signature: 8ty8w7X8aFGZylOq14hZcE-PTHuuJ6gsnt_hlAznawV22CX7ETRVekJQB4YG5YSOy_FrH_zZRdLybNIf-98GBg
```

### 17.2 A rotation statement

A rotates from "a" to "a2", with the commitment to "a3"
(`oYESsLe0Il_zBSfg9896HjdC9jKwO2X0VCcDw6VlTck`), dated `1800100000`:

```
msfp-key-rotation-1
A6EHv_POEL4dcN0Y50vAmWfk1jCbpQ1fHdyGZBJVMbg
zRSzf5VulTGU_3-3Oz2B3MVh1hp1OAlLfD4aZD7l86o
oYESsLe0Il_zBSfg9896HjdC9jKwO2X0VCcDw6VlTck
1800100000
```

- `oldSignature` (key "a"):
  `kQSI82v99bIhytamL0mL8akMYD6yqncLjjHzXPOowG_iJhMaXTEGjTZRy0gJflMXERdpJag-9YouRkQBJ68RDg`
- `newSignature` (key "a2"):
  `QpQyWP0uZ4fUS5cQvYal3GolQJkdMR1n3syq3I8IgoQDE1b5-9ImP3kye4qVJP-AJRovQPE9hl4hwSOTQct_AA`

The commitment to "a2", which A sent in its `pair` answer, is
`UZyuk-k60BHXhTVYlEBe2PqBYsblStoTO7ttu5Mt9Xs`: SHA-256 of "a2"'s public
key matches it, so B rotates at once.

### 17.3 A revocation statement

A revokes "a", dated `1800200000`:

```
msfp-key-revocation-1
A6EHv_POEL4dcN0Y50vAmWfk1jCbpQ1fHdyGZBJVMbg
1800200000
```

Its signature with key "a":
`WVNTbO_TC06833zcHZksGCPv-Hwe9ilbWlI8oqyh5grn5W0OHKiDvIo-3S9CLj6-A6VheBQuguJTzmSpFTPoDQ`

## 18. Open questions

Where a proposal of 2026-11-16 left a point open, the text above
follows the proposal's own default, and the point is listed here, with
its source in [proposals/](../../proposals/README.md). Points 1 to 16
were settled by the owner on 2026-11-17; each says how.

**Rules and checks**

1. **Point checks in other languages** (proposal 1, open point 2).
   PyNaCl's `crypto_core_ed25519_is_valid_point` and `@noble/curves`
   (`isTorsionFree`) are believed to cover section 2.1's point checks.
   Not verified.
   **Settled:** a check to make, not a choice: run these libraries
   against the vectors, and write the result here.
2. **A problem type for non-canonical input** (proposal 1, open point 3).
   Should a refusal for a non-canonical form, after the signature checked
   out, carry a specific `type` (`invalid-request`) to help implementers
   debug? Section 2 gives the status that call gives a bad body, with its
   usual `type`.
   **Settled:** no specific `type`; the call's usual status and `type`.
3. **An external review** of the signing scheme (proposal 6, open point
   2). It would settle the "custom crypto protocol" concern more directly
   than switching to HTTP Message Signatures (RFC 9421), which proposal 6
   compared and set aside.
   **Settled:** yes, before version 1 is called stable.
4. **Rate limits.** Closed on 2026-11-16: section 8.3 requires them,
   with the answers and the retry rules.

**Rotation and revocation**

5. **The 30-day overlap** (proposal 3, open point 1): long enough for a
   peer that is off for a holiday, long enough also to keep a retired key
   around. A shorter default with an operator setting is possible.
   Section 13's overlap is the same question (proposal 8, open point 1);
   both could share one setting.
   **Settled:** 30 days, fixed, no setting.
6. **A rotation from an unknown key** (proposal 3, open point 2). Should
   a rotation statement also be accepted from N, carrying O's signature,
   so that a server that lost O after rotating can still catch up a peer?
   This needs care with section 6.5's "who is heard" rule. Not allowed in
   this draft.
   **Settled:** not allowed; a server that lost O pairs again.
7. **Suspension forever?** (proposal 3, open point 3). Should a suspended
   peer become `gone` after some time without a valid rotation? In this
   draft it stays suspended until its operator ends the pairing.
   **Settled:** no: after 30 days without a valid rotation, the peer becomes `gone` (section 12.6).
8. **Proving the exported key** (proposal 3, open point 4). Section 12.5
   shows the committed key's fingerprint next to the export; probably
   enough.
   **Settled:** the fingerprint is enough.

**Versions and parts**

9. **`media` as its own part** (proposal 4, open point 2), or a
   sub-version of `playback`. Separate in this draft, because the media
   formats are where most change is expected.
   **Settled:** its own part.
10. **Versions in `pair`** (proposal 4, open point 3). Should `pair`
    requests and answers carry `core` and `parts` too, so that a new
    pairing knows them before the first `hello`? Only `hello` carries them
    in this draft.
    **Settled:** no; only `hello` carries them.
11. **The 8-entry caps** on version lists (proposal 4, open point 4) are
    arbitrary limits against abuse.
    **Settled:** kept.

**HTTPS and addresses**

12. **The transition for `http` pairings** (proposal 7, open point 1):
    its length, and whether the cut-off is a version or a date.
    **Settled:** one release (section 16.5).
    **Moot since the test draft was dropped (2026-11-17)**: there are no
    `http` pairings to move.
13. **Making a self-signed certificate** (proposal 7, open point 2).
    Should the specification say more (key type, names, lifetime) than
    section 3.2's recommendation?
    **Settled:** section 3.2 now names the address's host as the
    certificate's name, and renewal by the implementation.
14. **Local testing** (proposal 7, open point 3). Section 3.1 leaves plain
    `http` to an implementation's test switch, outside the protocol.
    **Settled:** outside the protocol, as said.
15. **Several addresses** (proposal 8, open point 2), for example a public
    name and a LAN address. Not allowed: one address per server keeps the
    signed host simple.
    **Settled:** one address.
16. **A change of base path** (proposal 8, open point 3). Section 13
    announces addresses only. A peer learns a new base by fetching the
    discovery document again when calls fail (section 11); whether a
    change of `base` should be announced like an address is open.
    **Settled:** not announced; the document is fetched again when calls fail.

**Choices this draft made where the proposals were silent** (all confirmed by
the owner on 2026-11-17; they may still change as implementations show)

17. The answer message's media type is a fifth line, before the body
    hash, written as type/subtype in lowercase without parameters, and
    `-` when there is no `Content-Type` (section 6.3). Confirmed by the
    owner on 2026-11-17.
18. `revoke` is answered `200` `{"state":"suspended"}`; `rotate`'s and
    `revoke`'s bad bodies get `400` `invalid-request` (sections 9.3, 9.4). Confirmed by the
    owner on 2026-11-17.
19. A suspended peer is a flag beside the state, not a fifth state; it may
    still send `revoke` (section 12.6). Confirmed by the owner on 2026-11-17.
20. `base` must end in `/api/v1/federation/`, and the media's stream root
    follows the same prefix (section 11). Confirmed by the owner on 2026-11-17.
21. A rotating server whose request under O gets an unsigned `404` tries a
    `hello` under N (section 12.4); repeated `rotate`s carry newly dated
    statements. Confirmed by the owner on 2026-11-17.
22. **Moot since the test draft was dropped (2026-11-17)**: this core is
    the only core, and addresses are `https` only (section 3). The
    choice was: this core is used only when both servers' addresses are
    `https`; until then a pairing stays in the test draft's core, and the
    `address` member of `hello` is read in the test draft's core between
    servers that list this one, so that `http` pairings can move (sections 10.3 and 16.5). Confirmed by the
    owner on 2026-11-17.
23. Section 3.3 refuses the ranges listed, by default, and lets an
    operator allow loopback only. (Whether NAT64's `64:ff9b::/96` and
    6to4's `2002::/16` are judged by the IPv4 address they carry was open
    here; closed on 2026-11-17: they are, as in the test draft's section 3.2.) Confirmed by the
    owner on 2026-11-17.
24. Section 8.3's refusals: `429` `rate-limited` for one peer over its
    limit, `503` `server-busy` for a full server, and `retryAfter` in the
    signed body rather than in the unsigned header alone. Confirmed by the owner on
    2026-11-17.
25. Section 8.3 suggests a number for every limit it requires, the
    values Overfin uses. Confirmed by the owner on
    2026-11-17 (before, the per-peer, global, concurrent and `play`
    limits had none).

**Outside services** (raised by the owner on 2026-11-17)

26. **Depending on TMDB and MusicBrainz.** Title keys use their
    identifiers, which is safe: a key is a string, and two servers compare
    keys without asking anyone. Calling the service is the weak spot: a
    video title carries a key only, so a viewer's server needs TMDB (a
    company, proprietary data, a key required, terms that can change) to
    show its name and pictures. MusicBrainz is a nonprofit with CC0 data
    that can be mirrored. Three possible answers, none decided:
    1. A statement here: no outside service is needed to speak MSFP;
       schemes are identifier namespaces.
    2. Optional descriptions for video titles (name, year), as music
       titles carry ([proposal 11](../../proposals/0011-music.md), open
       point 2, now settled in
       [video open question 4](video.md#8-open-questions) and
       [music's open questions](music.md#9-open-questions)), so that
       a viewer's server can list a peer's titles without TMDB.
       Proposal 16 took this answer for `local` titles only (a name, a
       year, one image); titles of a public database still carry keys
       only.
    3. Wikidata (CC0, a nonprofit) as a later neutral scheme: it links
       TMDB, IMDb, TVDB and MusicBrainz identifiers, a way out if TMDB
       ever became a problem.

**Choices this draft made, confirmed by the owner on 2026-11-17** (when the content types were
made optional parts on 2026-11-17, [proposal 11](../../proposals/0011-music.md))

27. **Moot since the test draft was dropped (2026-11-17).** The choice was:
    `video` and `music` have no test-draft version, and are off when
    `catalog`, `playback` or `media` is at the test draft's version
    (section 10.1). Between servers whose parts are at the test draft's
    version, films and series are shared as the test draft says, without
    `video` (section 16.6).
28. The examples of sections 9.2, 11 and 17.1 list `video`; the example
    of section 10.2 lists `music` too. B's answer in section 17.1 is
    unchanged: it lists `catalog` only, so B shares no titles with A.

**Choices this draft made, confirmed by the owner on 2026-11-17 (proposal 9)** (when
option 1 of [proposal 9](../../proposals/0009-private-servers.md) was
written in)

29. A server that can't be called sends its scheduled `hello` to
    suspended peers too, and reads only `rotation` from their answers
    (sections 7.5 and 12.6). The proposal says both "to every peer that
    isn't suspended" (its 1.3) and "except P's scheduled `hello`, kept so
    that a rotation can still reach it" (its 1.5); this draft follows 1.5.
30. The 26 characters of the address of section 5.2 are an exact base32
    encoding: the 2 unused bits of the last character are zero, so each
    address has one text, as section 2 asks of every value.
31. The address checked against `inbound: false` is the message's
    `address`, or, in a `hello` without one, the stored address
    (sections 5.2 and 9.2).
32. A server that can't be called sends `knownKey` in every `hello`
    request, not only during a peer's rotation (section 7.5).
33. A pulled `revocation` of K is carried only in answers signed with K;
    once the peer knows the server by its new key, it is no longer sent
    (section 12.6).
34. The pulled members' wrong values: a `rotation` that `rotate` would
    refuse with `400` changes nothing; a `rotation` or `revocation` that
    isn't an object, and a `callsFailed` that isn't `true`, count as
    absent in an answer; a request's `rotation` other than `pending` or
    `refused` is ignored; each one-way member is ignored in the other
    direction (sections 9.2 and 12.4).
35. When a peer's `inbound` turns `true` in a `hello` that also names a
    new address, the confirming `hello` goes to the new address, and
    confirms both (sections 7.5 and 13.2).
36. A `hello` answer with `inbound: false` and an address of another form
    fails, as a badly signed answer does (section 5.2).

**Decided by the owner on 2026-11-18.** None of these was an open
question above; they settle points the text left unclear or got wrong.

37. Keys: `nextCommitment` is required in `pair`, both ways, and there
    is no trust on first use from `hello` (sections 9.1, 9.2 and 12.5);
    the next key's fingerprint comes from its commitment (sections 4.2
    and 12.5); who gets `rotate` (section 9.3); a revoked previous key,
    a rotation signed with the previous key, a rotation back to a former
    key (sections 9.3, 9.4, 12.3, 12.6); confirming a pending key, "each
    contact", two rotations in a row (sections 12.3 and 12.4); a
    suspended peer that becomes `gone` (section 12.6); a stolen key with
    no next key (sections 4.1 and 12.7); ending forgets a pending key
    (section 7.4).
38. Addresses: pins and stream URLs during a move, when to announce, a
    third address, an invalid announced address, which peers count, a
    move dropped (sections 3.2, 13.1 and 13.2,
    [media section 1](media.md#1-the-urls)); `null` in unknown members,
    `Content-Type` parameters, no discovery document (sections 2.3, 2.4,
    10.2 and 11).
39. Servers that can't be called: the 30-day answers of a revoked key
    (section 12.7), the schedule in state `ours`, a move to or from
    `.invalid`, the catalog read again, "couldn't connect",
    `rotation: "refused"` (sections 7.5, 8.3, 12.4 and 13); a failed `hello` answer
    keeps nothing (section 9.2).
40. The rest: no common core (sections 8.2, 10.3, 10.4); content codings
    (section 3); integers and other numbers (section 2.3, with
    `helloVersions` in section 17); and, in the parts, refusals of `play`
    ([playback section 2.5](playback.md#25-refusals)), track IDs
    ([playback section 2.1](playback.md#21-the-request)), changes of a
    title ([catalog section 3.2](catalog.md#32-whole-list-or-changes)),
    the relayed media type ([media section 3](media.md#3-what-is-fetched)),
    empty PGS parts, progress in a copy of several episodes, and `ids`
    ([video sections 4.3, 5.2, 6.2 and 6.5](video.md#65-picture-subtitles)).

**Also decided by the owner on 2026-11-18**, points the text left
unclear while the engine's API was written down:

41. Keys and addresses: after the 30 days of a key replaced
    (section 12.7, case 2), a former peer gets the plain unsigned `404`;
    a request signed with the server's own key, its address check, gets
    a `404` signed with that key (sections 6.4 and 13.1); the 30 days of
    a move run from when the new address is reached, and a peer that
    never calls counts as not moved until they end (section 13.1);
    `callsFailed` clears at the next successful call, `pair` not counting
    (section 7.5); a peer's discovery document is fetched again after 3
    calls in a row answered but not signed by the peer, at most once an
    hour, a fetch with no answer keeping the stored base, and a wrong
    media type is no document (section 11); a server turning "can't be
    called" off stays as it was until it reaches its real address
    (sections 7.5 and 13.2); only a `200` `hello` at the new address
    confirms a move, an answer with `inbound: false` during confirmation
    puts the peer back as one that can't be called, and confirmation
    retries at most once an hour (sections 7.5 and 13.2). Point 16's
    "fetched again when calls fail" now has a rule.
42. Catalog and playback: a change of offers, or of the parts on, forces
    a whole reading, numbered in the change log
    ([catalog section 3.2](catalog.md#32-whole-list-or-changes)); a whole
    reading that fails halfway keeps the last complete list, its pages
    dropped (the same section); track IDs have only the forms `a<n>` and
    `s<n>` ([catalog section 2.1](catalog.md#21-library-ids)); a
    negative or out-of-range `maxAudioChannels` or `maxHeight` gets
    `400` ([playback section 2.2](playback.md#22-what-the-owning-server-does),
    [video section 5.1](video.md#51-the-request)); a `subtitleLanguage`
    matching a regular and a forced track chooses the regular one
    ([video section 5.4](video.md#54-choosing-the-subtitles)); a music
    `durationMs` of 0 counts as absent
    ([music section 3.2](music.md#32-what-the-caller-keeps)); the
    viewer's server may pass a peer's `416` for a range the viewer asked
    for ([media section 2.3](media.md#23-statuses)).

**Platform neutrality, decided by the owner on 2026-11-18**, from the
check of how Plex and Emby map onto MSFP
([api/platforms.md](../../api/platforms.md#5-what-the-spec-should-change-or-state)):

43. Two changes. A copy may be several files played one after the other
    as one (parts); `file` is only for a copy of one file, and a copy of
    several parts is served as `hls`
    ([playback section 1](playback.md#1-terms),
    [media section 1](media.md#1-the-urls)). Only the subtitle tracks the
    owning server can serve are listed, and a title's `subtitles` follow
    the same rule ([video sections 3 and 5.4](video.md#54-choosing-the-subtitles)).
    Revised by proposal 16: a `delivery` `file` of a copy of several
    parts is refused (`409` `delivery-refused`); a copy lists all its
    subtitle tracks, giving URLs to those the owning server can serve
    apart; the title summary `subtitles` is gone
    ([video section 3.3](video.md#33-streams-and-tracks)).
44. Nine points stated: episodes keyed by their TMDB episode ID, looked
    up with the owning server's own TMDB access, a media server's numbers
    not being TMDB's by default ([video sections 4.1 and 7](video.md#41-episode-order));
    credits are `outro`, a marker with no known end ends at `durationMs`
    ([video section 5.3](video.md#53-the-answer)); previews kept one image
    per interval are tiled into sheets, every *n*-th image allowed
    ([video section 5.5](video.md#55-seek-bar-previews)); the default
    audio is the file's, never a person's saved choice, and `role` is best
    effort, absent meaning the title's own sound
    ([playback sections 2.3 and 2.6](playback.md#26-choosing-the-copy-and-the-tracks));
    ISO 639-2 codes sent as ISO 639-1
    ([catalog section 3](catalog.md#3-titles), vectors `languageCodes`);
    the tag "MusicBrainz Track Id" holds the recording MBID
    ([music section 1](music.md#1-terms)); with `file`, the viewer's
    player selects `audioTrack` itself
    ([media section 1](media.md#1-the-urls)); an implementation may run
    as a separate service, holding the narrowest credential (section 1).
45. Six points stated, from the Jellyfin plugin, decided by the owner on
    2026-11-20: a track's position is the owning server's own stable
    number for it in the copy
    ([playback section 2.3](playback.md#23-the-answer)); a server whose
    media server has no transcode budget makes one, and a remux found to
    re-encode counts as a transcode when the owning server can tell
    (playback section 6.2), ending when none is left (playback section
    6.3); a server under a base path has its discovery document served at
    its address's root (section 11); a viewer's server sends `progress`
    no more often than every 45 seconds but never lets 2 minutes pass
    without one, paused too ([playback section 3](playback.md#3-progress)).

**Neutrality, decided by the owner on 2026-11-22**
([proposal 17](../../proposals/0017-neutrality.md)):

46. One rule for unknown values, in every part (section 8.5): open
    fields take tokens, an unknown token is kept in a fact and refused
    with `409` `delivery-refused` where it names what to serve, and no
    list has `other`. The normative text names no product: what it said
    of Overfin, Jellyfin, Plex and Kodi, and its suggested numbers, moved
    to the non-normative [implementers' notes](implementers.md). The
    schemes of title keys and of `ids` are listed in a
    [registry](../registry.md); a new scheme is an entry there, not a
    new version.
47. The whole of proposal 17, as frozen: media types join tokens as a
    class of open value (section 8.5), kept in a fact and refused with
    `409` `delivery-refused` where they name what to serve; a viewer's
    server that doesn't know a copy's `mediaType`, `codec` or `threeD`,
    or can't parse its `codecs`, doesn't play it as it is; the lists are
    the [registry](../registry.md)'s or an outside vocabulary's, and
    ffmpeg is no longer one. A scan's read cap is the `scanBytes` of the
    scan's answer (section 8.4).
