# Proposal 19: MSFP Media

Status: Adopted (2026-11-26), written (2026-11-27); vectors split
(2026-11-27); conformance suite split, player mode (2026-11-28). Every choice below is the owner's, taken on
2026-11-25 and 2026-11-26; where this text fills a gap the owner left to
the writing, it says "writing choice". Written into a new spec,
[msfp-media](https://github.com/msfp-io/msfp-media), and into spec/v0.

Touches: every part of spec/v0 except `groups`; the registry moves; the
vectors and the conformance suite moved after the text (below). The
core's pairing, signing, keys and groups are untouched; so is `hello`'s
list of parts.

## Problem

MSFP describes media (titles, keys, copies, how a copy is asked for,
fetched and played) and federates servers (keys, signed calls, pairing,
groups, what each peer may see). The first half isn't about federation.
Overfin's players already speak it to their own server: its player API
uses MSFP's copy object and the `play` exchange (an inventory of its
player API, 2026-11-24). Keeping the two
halves in one spec means a change for players forces a federation
change, and a player has to read a federation spec to play a video.

## Proposal

**MSFP Media** is a language of its own: what a title is, what a copy
is, how a caller lists them, asks to play one, fetches it and reports on
it. It is a separate spec, `msfp-media`, in msfp-io, with its own
version. A **carrier** brings it between two ends and decides who may
use it: MSFP between paired servers; a product's own API between its
players and its server (Overfin). MSFP references the MSFP Media
version it carries, in its text and its discovery document.

Aim, in the owner's words: "so generalistic that a toaster could play a
video from a toilet bowl".

### What MSFP Media holds

Everything below is MSFP version 0's text as it is, unless a line says
otherwise; wording made neutral ("the caller", "the source") where it
said "the peer", "the owning server", "the viewer's server". Where this
summary and spec/v0 differ, spec/v0 wins, except for what is marked
**new**.

- **Base**: text and JSON encodings (strict I-JSON), case, language tags,
  the one rule for unknown values (core 8.5), the error format (problem
  documents), media answers (core 8.4). JSON is defined by the language
  itself, with HTTP compression (**new**: gzip, zstd); CBOR set aside.
- **Level 1, the title**: kind (film, series, season, episode, extra;
  artist, album, track), outside IDs (TMDB first), names per language,
  year. Out: pictures, overview, cast, genres. Title keys as video
  section 4 has them: `tmdb` → `tvdb` (absolute `a<n>` included) →
  `local`; episodes by TMDB episode ID `e<id>`, the numbers form as a
  fallback, TMDB's order; episode 0; several episodes with `with`;
  extras `…/x/<opaque>`. `local` titles are never matched across
  sources and carry name, year and one image (the one exception to
  "pictures out").
- **Level 2, the copy**: what playing needs (container, duration, size,
  video facts, audio and subtitle tracks, outside subtitle files as
  tracks), and the extras announced in the copy: chapters, segments,
  previews, fonts. Copy IDs opaque, stable while the file doesn't change,
  never derivable from the file alone. Codecs: registry tokens and
  RFC 6381 `codecs` strings; **the registry moves into MSFP Media**.
- **Listing**: MSFP's catalog `libraries` and `titles` as they are (key
  order, 5,000 titles or 3 MiB a page, `after`/`next`,
  `since`/`changes`/`gone`/`seq`), plus, **new**: an optional `kind` filter; an
  optional `under` (a parent key: a series' seasons, a season's
  episodes); `libraries` optional (absent: everything the caller may
  see). The libraries call is in the language (**new**: `libraries` optional in `titles`): ID and name, MSFP's ID
  rules (opaque, stable, never reused, never compared across sources).
- **Level 3, playing**: `play`, `progress`, `stop`.
  - The ask has two forms: **explicit** (`delivery`: the file as it is,
    a remux, a transcode to stated targets; none = the baseline) or, **new**, the
    device's **full capability list**, from which the source picks a
    delivery and says which (`method`). The list is never trimmed and
    names no profiles. Optional `maxBitrateKbps`, the player's own
    estimate. No adaptive streaming; no DASH.
  - Deliveries: `file` (byte ranges), `remux` (HLS, mp2t or fMP4),
    `transcode` (HLS, targets); the baseline is H.264 High ≤ 4.1 SDR with
    stereo AAC, in HLS.
  - The answer: the delivery, where to fetch, an opaque ticket. Sign-in
    stays outside the language.
  - Progress: playback ID, position, **new**: playing or paused; then
    `stop` with the last position. **New**: the interval is the source's;
    no report for a while = stopped. **New**: a track or delivery change
    is a new ask naming the playback it replaces. Positions travel; `startPositionMs` too. No
    carrier may forbid positions.
  - Purposes: `play` and `download` (file or sized transcode, 24-hour
    ticket without the idle rule, stopped when complete, late reports
    with `at` accepted for 30 days).
  - Refusals (**new** as a list of the language): not found, gone, not that way, target not possible, bad
    ask, busy (temporary, with a retry time), unreachable, ticket
    refused, no such playback. Each says temporary or final and may name
    what would work instead. **No "not allowed"**: anything outside what
    the caller may see is "not found".
- **Fetching**: MSFP's media part (URLs built from the ticket, ranges,
  complete HLS, the file, subtitles, fonts, sheets, images, bounds).
  Tickets: at least 128 random bits, a bearer, in the address; fixed
  names; addresses built, not received; the kinds and how long each
  works; a ticket ends when the carrier withdraws the caller's access.
- **Subtitles**: fetched apart; text as WebVTT, ASS kept as ASS
  (rebuilt and checked by the source), pictures as PGS; fonts TTF/OTF;
  burned in only by `transcode.burnSubtitle`.
- **Extras**: chapters, segments and previews as in MSFP's video part.
- **Music** as a content part beside video (MusicBrainz or `local`
  keys, tracks with album, disc and number, covers from the Cover Art
  Archive, an audio baseline of AAC in HLS, queues).
- Everything else carries over from MSFP version 0 as it is: refusal
  codes of limits, restarts, queues, the error format.

Out of the language: accounts, sign-in, profiles, categories,
recommendations, ratings, watched state and history, search, sorting,
filters, home rows.

### What MSFP keeps

The core (encodings of keys and signatures, HTTPS and certificates,
pairing, signed calls, `hello`, rotation, revocation, address changes,
versions and parts, the discovery document), `groups`, and, for each
part that carries MSFP Media, the federation rules around it:

- who sees what: offers per pairing and per group, "the listing limited
  to what's shared", whole readings forced by sharing changes;
- `limits` and their counts (streams, downloads, speed caps) and the
  server's name;
- signed requests; between servers the ask is **always explicit**: no
  capability list ever goes to a peer;
- the stream root, its certificates, and who may fetch: the paired
  server only, never a browser;
- **scan access** (a server serving its own media server) stays MSFP's
  only, with its ticket kind;
- "never fetch previews in bulk" stays MSFP's (between a player and its
  own server, the product decides).

The owner's rule kept as a rule of the carriers: a player asking a
transcode asks its own server; if that server can transcode, it asks
the source for the file and transcodes itself; otherwise it asks the
source to transcode.

MSFP's parts keep their names on the wire (`catalog`, `playback`,
`media`, `video`, `music`): each now says what of MSFP Media it carries
and what it adds. `hello` is unchanged.

### Versioning

MSFP's rules: version 0 is a draft, with no compatibility between
drafts; version 1 only on the owner's word. MSFP states the MSFP Media
version it carries; unknown fields and values are ignored by the one
rule for unknown values. The split happens **before MSFP version 1**, at
no set date.

### Conformance

One suite with two modes: peer mode (MSFP, and MSFP Media through it)
and player mode (MSFP Media alone, against a player API, given a ready
ticket or token, no sign-in). The vectors split: keys, copies, strict
JSON, asks, refusals, tickets, subtitles and previews go to msfp-media;
server keys, rotation, revocation, groups and signed exchanges stay.

### Writing choices

- The spec's documents: `README.md`, `base.md`, `listing.md`,
  `playing.md`, `fetching.md`, `video.md`, `music.md`, `notes.md` (not
  normative), and `registry.md` beside the versions. Sections of MSFP's
  catalog, playback and media parts that are the language move into
  listing, playing and fetching; video and music move as they are.
- Ticket kinds: MSFP Media has `playback`, `download`, `previews` and
  `image`; MSFP adds `scan`.
- The capability list's exact shape is new text (the owner decided its
  vocabulary, not its layout).
- First step: the text of both specs. Second, the vectors split: the
  language's cases in msfp-media's vectors, one file per document
  (`base`, `listing`, `playing`, `fetching`, `video`, `music`), made by
  its own generator, whose rules msfp-spec's generator imports;
  msfp-spec keeps the federation's, and the carrier's cases of groups
  that moved (a capability list refused, the scan purpose and its
  ticket). Carrier values became inputs of the cases they decide (the
  refusal over limits, the bound on an answer's size); case names and
  values use the language's words. Subtitle and font cases sit in
  `video.json`, with video's rules, which replaces video's choice 9.
  base 2.1 and 2.2 gained their own cases, and four rules found while
  writing them (owner's, 2026-11-27): a body is an object, nested at
  most 64 levels, its numbers within a double's range, and hex never
  empty. Third, the conformance suite: MSFP Media's checks in
  msfp-media's suite, which a carrier runs through an adapter; MSFP's
  suite runs them through its own (peer mode), and msfp-media's runs
  them against a product's own API (player mode), from a binding file
  the tester writes (its calls, paths, sign-in headers, stream root),
  with the capability list's checks, which only player mode reaches.

## Alternatives considered

- **A `players` part inside MSFP** (the plan of 2026-11-24): set aside
  on 2026-11-25; an Overfin change would still touch the federation
  spec.
- **CBOR**: weighed, set aside; revisit only if parsing on very weak
  devices matters.
- **Named device profiles**: refused; they would need editing over time.

## Cost

The text of both specs; then the vectors and the conformance suite;
then msfp-go and msfp-dotnet (names and documents only, unless the
additions are built); Overfin's player API aligned with the language
(listing additions, the capability ask); the plugin unchanged.

## Compatibility

Version 0 is a draft; no servers run it outside the owner's.

## Open points

None decided open. The capability list's shape is for the owner's
review once written.
