# MSFP version 0

**Draft.** Version 0 stands alone: a version 0 server speaks version 0
only. Version 0 means draft: anything may change, with no promise of
compatibility between two drafts; version 1 will be the first stable
version. Before it, MSFP had [a test draft](../history/test-draft.md)
([history](#history)).

Version 0 is made of a core and six parts, each with its own version:
three that hold the shared mechanics (`catalog`, `playback`, `media`),
two content parts (`video`, `music`), each optional, that say what the
titles are and how they play, and `groups`, optional too, for servers
that federate as a group. This lets an implementation build the
parts at different times, and lets the parts change at different speeds:
the trust part (keys, signatures, pairing) rarely and carefully, the
media formats more often.

## The documents

| Document | Version | What it holds |
|----------|---------|---------------|
| [core.md](core.md) | core 0 | encodings and canonical forms, HTTPS, keys and fingerprints, the pairing code, signed requests and answers, pairing and its states, pairing through a group, relations, `pair`, `hello`, key rotation and revocation (`rotate`, `revoke`), address changes, versions and parts, the discovery document |
| [catalog.md](catalog.md) | `catalog` 0 | `libraries` and `titles`, pages bounded by size, opaque library IDs, the general rules of title keys and their schemes, copies and extras in `titles`, the description of `local` titles, what is shared |
| [playback.md](playback.md) | `playback` 0 | `play` (purpose, delivery, transcode targets, restarts), `progress`, `stop`, limits, downloads, scan access, what the owning server enforces |
| [media.md](media.md) | `media` 0 | what the stream URLs serve: complete HLS, the file by byte ranges, subtitles, fonts, sheets and images, and their sizes; the kinds of ticket and how long each works |
| [groups.md](groups.md) | `groups` 0 | groups of servers: owner, admins and members, the signed list every member keeps, codes for a group, requests and answers encrypted for each member, pairing through a group, offers and limits per group |
| [video.md](video.md) | `video` 0 | films, series and extras by TMDB, TheTVDB or the owner's own keys: title keys and their identity rules (TMDB's numbering, several episodes in one copy, other identifiers), copies (streams, tracks, chapters, skip markers), the description of `local` titles, `previews`, subtitles (WebVTT, ASS, PGS), fonts, seek-bar previews |
| [music.md](music.md) | `music` 0 | artists, albums (by release) and tracks (by release track) by MusicBrainz or the owner's own keys: descriptions, copies, cover art, `lyrics`, audio-only playback, queues |
| [implementers.md](implementers.md) | not normative | notes for implementers: what the parts would say of particular media servers and implementations, suggested numbers, mapping a media server's names to the registry and the outside vocabularies (H.273, RFC 6381), advice to viewer's servers on choosing copies and tracks, decoding safely |
| [registry.md](../registry.md) | for every version | the MSFP registry: the values of the open fields that describe media (codecs, subtitle formats, container media types, `hdr`, `edition`, `extraType`, segment kinds, roles, `objects`, `threeD`, music types) and the schemes of title keys and of `ids`; a new codec or scheme is an entry there |

Read the core first. Each part says what it needs from the core, and
refers to it by section.

## Glossary

Each document defines its own terms in full; this list is a summary, in
one place, with where each is defined.

| Term | Meaning | Defined in |
|------|---------|------------|
| Server | one installation of an implementation, with one address and one current key | [core 0](core.md#1-overview-and-terms) |
| Address | where a server's API answers: `https://host[:port]`, no path | [core 5.2](core.md#52-addresses) |
| Base path | the path of the federation calls, `/api/v1/federation/` unless the discovery document says otherwise | [core 11](core.md#11-the-discovery-document) |
| Prefix | the part of the base path before `/api/v1/federation/`; empty by default | [core 11](core.md#11-the-discovery-document) |
| Peer | another server this one knows by its public key | [core 7.3](core.md#73-states) |
| Pairing | two servers becoming peers: by hand, from a code, or through a group | [core 7](core.md#7-pairing-step-by-step) |
| Pairing code | one line holding the maker's address, key, a secret, an expiry and, for a group, the group | [core 5](core.md#5-the-pairing-code) |
| Group | servers whose operators chose each other, with an owner, admins and members, as a signed list each member keeps | [groups](groups.md#1-terms) |
| Owner, admin, member | the one member that names admins, sets the group's name and settings and dissolves it; a member that may invite and ban; any server in the group | [groups 1](groups.md#1-terms) |
| The list, event | a group's signed events, which each member keeps and passes on; its state (members, roles, settings, bans) is computed from them | [groups 1](groups.md#1-terms), [groups 3](groups.md#3-events) |
| Member ID | the key a member joined with, by which events name it | [groups 3.4](groups.md#34-member-ids-and-keys) |
| Mail | a message from one member to another, carried by the members between them and readable by its recipient only | [groups 5](groups.md#5-requests-and-answers-the-mail) |
| Link | a pairing between two servers, whatever made it; two servers are linked once | [groups 1](groups.md#1-terms), [core 7.7](core.md#77-relations) |
| Relation | why two servers are linked: the individual relation, or one group relation per group they share and are linked through; a link lasts while one holds | [core 7.7](core.md#77-relations) |
| Individual relation | the relation both operators chose, which each side says it wants with `individual` in `hello` | [core 7.7](core.md#77-relations) |
| Fingerprint | a short form of a public key, which operators compare when pairing | [core 4.2](core.md#42-the-fingerprint) |
| Current key | the key a peer knows a server by now | [core 12](core.md#12-key-rotation-and-revocation) |
| Next key commitment | SHA-256 of a key made for a later rotation | [core 12.5](core.md#125-the-next-key-commitment) |
| A server that can't be called | one that connects out but can't be connected to, and says so with `inbound: false`; its peers never call it | [core 7.5](core.md#75-a-peer-that-cant-be-called) |
| Nonce | 16 random bytes that make each signed request unique | [core 6.5](core.md#65-nonces-replay-and-restarts) |
| Pin | SHA-256 of a certificate's public key, which a server names in `hello` | [core 3.2](core.md#32-https-certificates) |
| *Trusted*, *named* | a certificate a public authority vouches for; one the peer named by its pin | [core 3.2](core.md#32-https-certificates) |
| Part | a separately versioned set of calls and rules: `catalog`, `playback`, `media`, `video`, `music`, `groups` | [core 10](core.md#10-versions-and-parts) |
| Content part | a part that names one kind of content's titles and says how they play: `video`, `music` | [core 10.1](core.md#101-the-parts) |
| Open field, token, media type | a field whose values come from a list that grows; its value, `a–z 0–9 . _ -` or an IANA-form media type, kept when unknown | [core 8.5](core.md#85-unknown-values) |
| Registry | the lists of the open fields that describe media, and the schemes of keys and `ids` | [registry](../registry.md) |
| Problem document | the JSON body of a refusal (RFC 9457), with `type`, `title`, `status` | [core 8.2](core.md#82-refusals) |
| Library | a set of titles a server may offer a peer, by an opaque ID | [catalog 2](catalog.md#2-libraries) |
| Offer, want | what one side shares; what the other takes of it | [catalog 1](catalog.md#1-terms) |
| Title key | the identity of a title: a film, series, season, episode or extra from TMDB's or TheTVDB's IDs; an artist, album or track from MusicBrainz's; or, for titles no public source knows, the owner's own (`local`) | [catalog 4](catalog.md#4-title-keys), [video 4](video.md#4-title-keys), [music 2](music.md#2-title-keys) |
| Owning server | the server that holds a copy and streams it | [playback 1](playback.md#1-terms) |
| Viewer's server | the server whose person watches; the only one the person's browser talks to | [playback 1](playback.md#1-terms) |
| Copy, version | one version of a title, one file or several parts, described in the catalog with its length, size, `etag` and tracks; the ID the owning server gives it | [playback 1](playback.md#1-terms), [video 3.2](video.md#32-copies) |
| Delivery, purpose | what a `play` asks to be served (`file`, `remux`, `transcode`); why it is started (`play`, `download`, `scan`) | [playback 2.7](playback.md#27-delivery) |
| Playback | one stream the owning server runs for the viewer's server, with an ID and a ticket | [playback 6.1](playback.md#61-a-playbacks-states) |
| Ticket | the opaque credential in the media URLs, the only one | [media 1](media.md#1-the-urls) |
| Stream root | `<address><prefix>/api/v1/stream/`, under which a server's media is | [media 1](media.md#1-the-urls) |
| Stretch, part | 120 seconds of a subtitle track, fetched on its own | [media 3.4](media.md#34-subtitles-fonts-sheets-and-images) |
| Sheet | a JPEG of seek-bar preview tiles | [video 5.5](video.md#55-seek-bar-previews) |

## Errors

Every status and problem `type` a federation call can answer, in every
part, is listed with its meaning and what the caller does in
[core section 8.2](core.md#82-refusals). The statuses of media fetches are
in [core section 8.4](core.md#84-media-answers). Limits and overload, and
how a caller retries, are in
[core section 8.3](core.md#83-limits-and-overload).

## Dependencies

- `catalog` needs only the core.
- `media` needs only the core.
- `playback` needs `catalog` (it plays title keys from `titles`) and
  `media` (its answers point at media URLs). The `limits` member of
  `libraries` belongs to `playback`.
- `video` and `music` each need `catalog`, `playback` and `media`.
  Every content part is optional: a server lists the ones it handles,
  and two servers share only titles of the content parts both list.
- `groups` needs only the core. Its offers and limits per group change
  what a server offers a peer and lets it use; the calls of `catalog`
  and `playback` stay the same.

A part whose dependency is off between two servers is off too. For
example, a server that speaks `catalog` but not `media` can list its
libraries, but shares no titles: `video` and `music` are off with
`media`.

## Negotiation in hello

Each server announces, in every `hello` request and answer, the core
versions and the part versions it speaks
([core section 10](core.md#10-versions-and-parts)):

```json
{"state":"paired","core":[0],"parts":{"catalog":[0],"playback":[0],"media":[0],"video":[0],"music":[0]}}
```

- For the core and for each part, two servers use the highest version
  both list. No common version: that part is off between them.
- A `hello` without `core` leaves no common core: the sender doesn't
  speak MSFP 0. Without `parts`, the sender speaks no part.
- Each side keeps what the peer's latest signed `hello` listed.
- A caller makes no call of a part that is off, and shows its people
  nothing that needs it. A receiver answers such a call with a signed
  `404` of type `not-supported`, which doesn't mean "not paired".
- The core's version also shows in every signed message: core 0 uses the
  labels `msfp-0 …` and the headers `MSFP-*`. A later core will have its
  own, so a message signed under one core never verifies under another.

An optional discovery document, `GET /.well-known/msfp`, lets a server
see a peer's versions and base path before pairing. It is unsigned,
proves nothing, and is off by default
([core section 11](core.md#11-the-discovery-document)).

## Versioning

- The core and each part have their own version: a non-negative integer.
  Version 0 of each is the documents above.
- **A new version of a part** is needed when an existing member's meaning
  changes, or when a receiver of the older version would misread a
  message.
- **Adding an optional member** doesn't need a new version, as long as
  its absence keeps the old behaviour. Receivers ignore unknown members
  ([core section 2.3](core.md#23-json)), so this is safe.
- **A change to keys, signatures, the code or pairing** is a new core
  version, with new labels and headers, and a new code prefix, so that a
  server of the older core refuses the new messages instead of misreading
  them.
- A server lists in `hello` every version it speaks, and keeps an older
  one for as long as it has a peer that needs it.
- Corrections that change nothing an honest sender writes may be made to
  an existing version as errata, listed in that version's changes.

## History

The test draft of November 2026
([spec/history/test-draft.md](../history/test-draft.md)) came first. This
version supersedes it, and doesn't work with it: no server of this
version speaks the test draft, and the test draft is not to be
implemented. It stays in this repository as history, with its vectors
([vectors/history/test-draft.json](../../vectors/history/test-draft.json)
and
[vectors/history/test-draft-strict.json](../../vectors/history/test-draft-strict.json)),
whose canonical-form cases hold for this core too.

This version was first numbered 1, and became version 0 on 2026-11-20
([proposal 13](../../proposals/0013-version-0.md)): a draft until it is
stable.

### What this version changes from the test draft

- **Strict rules** (proposal 1): one rule set for Ed25519, base64url,
  integers, JSON (I-JSON), headers and addresses. The test draft received
  the same rules as corrections; this version also refuses `null`.
- **Opaque IDs** (proposal 2): library and version IDs chosen by the
  owning server. Also a correction of the test draft.
- **Key rotation and revocation** (proposal 3), in the core.
- **A core plus parts** (proposal 4), MSFP's own labels and headers, and
  the discovery document.
- **Title identity** (proposal 5), in `video`: TMDB's numbering, `e0`,
  `with` for several episodes in one copy, `ids` for IMDb and TVDB.
- **Every content type optional** (proposal 11): films and series in the
  part `video`, music in the part `music` (artists, albums and tracks by
  MusicBrainz ID, with their own descriptions; cover art from the Cover
  Art Archive; one audio playback per track).
- **The answer's media type is signed** (proposal 6), in the core. MSFP
  keeps its own signing scheme rather than HTTP Message Signatures
  (RFC 9421).
- **HTTPS only** (proposal 7).
- **Address changes** (proposal 8), in the core.
- **Servers that can't be called** (proposal 9, option 1), in the core:
  `inbound: false` in `pair` and `hello`, peers that stop calling, a
  `hello` on a schedule, rotation and revocation pulled in `hello`
  answers, and the address `https://msfp-<26>.invalid`.
- **Groups** (proposal 12, 2026-11-20): the part `groups`, codes for a
  group in the core (section 5.1), pairing through a group (core
  section 7.6), offers and limits per group.
- **Relations** (proposal 14, 2026-11-21), in the core: a link lasts
  while one relation holds, `individual` in `hello` (core sections 7.7
  and 9.2).
- **The revision of version 0** (proposal 16, 2026-11-22, absorbing
  proposal 15): copies in the catalog, pages bounded by size, the `tvdb`
  and `local` schemes, extras ([catalog](catalog.md), [video](video.md)
  sections 3 and 4); delivery, restarts, downloads and scan access
  ([playback](playback.md) sections 2.7, 7 and 8); the file by byte
  ranges, complete HLS, audio renditions ([media](media.md) section 3);
  ASS, fonts, whole text tracks and `previews` (video sections 5.6 and
  6); music by release and release track, with copies, lyrics and a
  queue ([music](music.md)). Servers on version 0 before it and after it
  don't pair.
- **The clarifications** (all 34), as in the test draft's corrections.
- The seek-bar previews already in the test draft, in `video`.
- **The review round of 2026-11-16**: which addresses a server connects
  to ([core section 3.3](core.md#33-connecting-to-peers), also a correction
  of the test draft); required rate limits and overload answers (core section
  8.3); logging (core section 14.1); one error reference (core section
  8.2); exact HLS, WebVTT and PGS rules and bounds on fetches
  ([media](media.md) sections 2 and 3, and [video](video.md) section 6
  for WebVTT and PGS); choosing the copy and tracks, and
  a playback's states ([playback](playback.md) sections 2.6 and 6).
- **The live interop of 2026-11-17**, as in the test draft's corrections: no
  content codings (core sections 3, 6.3 and 6.6); answers' members of
  the wrong JSON type ([catalog](catalog.md) sections 2.2 and 3.3);
  `titles` without `libraries` (catalog section 3); a `hello`'s answer
  names the certificate served now (core section 3.2); limits behind a
  reverse proxy (core section 8.3); NAT64 and 6to4 addresses (core
  section 3.3).

The proposals and their decisions are in
[proposals/](../../proposals/README.md). Points the proposals left open
are listed in each document's open questions.

## Test vectors

[vectors/v0/](../../vectors/v0/) holds keys, a code, signed requests and
answers under the MSFP labels and headers, a rotation statement, a
revocation statement, a `hello` carrying `core`, `parts`,
`certificates` and `address`, and the `hello`s of a server that can't be
called, with a rotation and a revocation pulled in an answer
([core section 17](core.md#17-test-vectors)), the addresses of servers
that can't be called (`uncallableAddresses` in `strict.json`), media
servers' language codes and the tags sent (`languageCodes` in
`strict.json`), file and track names in media URLs (`mediaNames`), the
discovery document's `base` (`bases`), what a receiver keeps of a
`hello`'s versions (`helloVersions`), the
destinations a server
refuses to connect to (`destinations` in `strict.json`), and WebVTT,
HLS and PGS cases with their verdicts (`media.json`, media section 3 and
video section 6), music keys and titles with what a caller keeps
(`music.json`, music sections 2 and 3), and video titles with what a
caller keeps (`video.json`, video section 3), `play`, `progress` and
`stop` requests and answers with deliveries, restarts, downloads and
scans (`playback.json`, playback sections 2 to 8), and a group's events and
the state they give, a code with a group, `group-join`'s proof and a
sealed request mail (`groups.json`, groups sections 2, 3, 5 and 8.4).
They come from the
generator in [tools/vectors/](../../tools/vectors/), not from an
implementation.
