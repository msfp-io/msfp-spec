# MSFP version 0: the media part, version 0

**Draft.**

The `media` part says what the stream URLs serve, and how a viewer's
server fetches them: HLS, and the file itself read by byte ranges. It
says which tickets open them: a playback's, a download's, a scan's, and
the short tickets of a `previews` call, for preview sheets, and of an
`image` call, for the image of a `local` title. The content parts add their own URLs, whose forms,
fetches and sizes are all here: [video](video.md#6-media) adds text and
picture subtitles, fonts, seek-bar preview sheets and the image; what
their bytes must be is video's to say. It needs only the
[core](core.md); [playback](playback.md) gives the tickets the URLs are
built from, and says when a download, a scan or a restart may happen. Media is fetched with plain
`GET` requests that carry the ticket only: they are not signed.

The words "must", "must not" and "should" carry their usual meaning in
specifications.

## Contents

1. [The URLs](#1-the-urls)
2. [Requests](#2-requests)
   1. [Ranges](#21-ranges)
   2. [Methods](#22-methods)
   3. [Statuses](#23-statuses)
   4. [Bounds](#24-bounds)
3. [What is fetched](#3-what-is-fetched)
   1. [HLS playlists](#31-hls-playlists)
   2. [The file](#32-the-file)
   3. [The file as a plain file](#33-the-file-as-a-plain-file)
   4. [Subtitles, fonts, sheets and images](#34-subtitles-fonts-sheets-and-images)
4. [How long a ticket works](#4-how-long-a-ticket-works)
   1. [Downloads](#41-downloads)
   2. [Scans](#42-scans)
5. [Security considerations](#5-security-considerations)
6. [Open questions](#6-open-questions)

## 1. The URLs

Media is fetched over `https` only, and every fetch needs a *trusted* or
*named* certificate ([core section 3.2](core.md#32-https-certificates)),
never just any: the ticket and the media have only TLS to protect them.

A peer that can't be called
([core section 7.5](core.md#75-a-peer-that-cant-be-called)) is never the
owning server: nothing is fetched from its stream root, which is under an
address that doesn't resolve. It fetches its peers' media as any viewer's
server does.

The **stream root** of a server is `<address><prefix>/api/v1/stream/`,
where `<address>` is its address exactly as its peer stores it, and
`<prefix>` is the part of its base path before `/api/v1/federation/`
([core section 11](core.md#11-the-discovery-document)); without a
discovery document, `<prefix>` is empty. Example:
`https://a.example.org/api/v1/stream/`. While the owning server is
moving, it uses the address whose host the `play` request was signed
for ([core section 13.1](core.md#131-the-server-that-moves)).

Every media URL is absolute, under the owning server's stream root:

```
<stream root><ticket>/<name>
```

**Built, never received.** Answers carry no URLs. The viewer's server
builds every media URL itself from the owning server's stream root, a
ticket an answer gave and one of the fixed names below, so that it never
fetches an address a peer wrote.

- `<ticket>` matches `^[A-Za-z0-9_-]{1,64}$`. It is the only credential:
  a bearer capability. The owning server makes each ticket from at least
  128 bits of a cryptographically secure random source, never from a
  counter, a time or any other value a peer could predict. A ticket is
  never reused, for another playback or after its own has ended. The
  owning server compares tickets in a way whose time doesn't depend on
  how much of a ticket matches (a lookup by a hash of the ticket is
  enough), and neither side writes tickets into logs, diagnostics or
  error messages. Each ticket is of one kind (section 4), and opens only
  the URLs of its kind.
- The stream of a playback is `master.m3u8` when the answer's `protocol`
  is `hls`, and `file` when it is `file` (the copy as it is, all its
  tracks, read by byte ranges: section 3.2). `file` serves only
  playback's `file` delivery; a `remux`, a `transcode` and the baseline
  served without `delivery`
  ([playback section 2.7](playback.md#27-delivery)) are served as `hls`.
  The `file` protocol is only for a copy of one file: a `delivery` `file`
  of a copy made of several parts
  ([playback section 1](playback.md#1-terms)) is refused with `409`
  `delivery-refused`. With `file`, the viewer's player selects
  `audioTrack` itself.
- The content parts add names of their own: [video](video.md#6-media)
  subtitle, picture subtitle, font and preview sheet URLs, and the image
  of a `local` title; [music](music.md#52-the-queue) a stream for each
  entry of a queue, fetched as the main stream is (section 3).

| What | URL, after `<stream root><ticket>/` | Opened when |
|------|-------------------------------------|-------------|
| the stream | `master.m3u8` (`hls`) or `file` (`file`) | always, under a playback's ticket |
| a text track as WebVTT | `subtitles/<id>/<part>`, `subtitles/<id>/whole` | the answer's `subtitles` has an entry `<id>` of `form` `text` |
| an ASS or SSA track as ASS | `ass/<id>/<part>`, `ass/<id>/whole` | that entry also has `ass` `true` |
| a picture track as PGS | `pictures/<id>/<part>` | the answer's `subtitles` has an entry `<id>` of `form` `picture` |
| font *n* | `fonts/<n>` | *n* from 0 to the answer's `fontCount` − 1 |
| preview sheet *n* | `trickplay/<n>` | *n* below the sheet count of the copy's `previewLayout` in the catalog ([video section 5.5](video.md#55-seek-bar-previews)) |
| queue entry *n*'s stream | `queue/<n>/file` (`file`) or `queue/<n>/master.m3u8` (`hls`); the files its playlists name under `queue/<n>/` | *n* the entry's place in the request's `queue`, 0 to 4, for an entry the answer kept |
| a `local` title's image | `image` | under the ticket of `image`'s answer |

`<id>` is the track's ID (`s<n>`), `<part>` a number from 0 to 9,999.
Numbers in these URLs are decimal, without leading zeros. **The owning
server serves exactly these names**, under a ticket of the kind that
opens them (section 4), and answers `404` to any other, and to a number
past its count: a font at or past `fontCount`, a sheet at or past the
sheet count, a subtitle track with no entry of that form, a queue entry
it didn't prepare.

The names a playlist gives (section 3) are bare names: they match
`^[A-Za-z0-9._-]{1,64}$` and are never `.` or `..`, which a URL resolves
as folders, not files. The viewer's server checks a ticket and a track
ID against their forms before it builds a URL with them, and the URLs
it builds, split at `/` into their parts, are compared exactly as built:
no part is decoded, normalized or resolved, and a URL with an empty
part, a `.` or `..` part, a `%`, a query or a fragment is never fetched.
Test cases: `mediaNames` in
[vectors/v0/strict.json](../../vectors/v0/strict.json), and `names` in
[vectors/v0/media.json](../../vectors/v0/media.json) (the names an
owning server serves for a playback).

## 2. Requests

The viewer's server fetches with plain `GET`:

- No signature headers. No cookies. No query string: the owning server
  answers `404` to any query.
- No `Origin`, `Sec-Fetch-Site` or `Sec-Fetch-Mode` header. The owning
  server answers `404` to a peer's ticket when any of them is present,
  because only a browser sends them, and a browser must never use a
  peer's ticket.
- `Range` only on the file and on segments (section 2.1).
- No conditional headers (`If-Range`, `If-Match`, `If-None-Match`,
  `If-Modified-Since`, `If-Unmodified-Since`): the viewer's server checks
  the `ETag` of the answer itself (section 3.2). The owning server
  ignores them.
- Redirects must not be followed. The connection is checked as
  [core section 3.3](core.md#33-connecting-to-peers) says, like every
  connection to a peer.

A fetch refused for its certificate makes the viewer's server send a
`hello` and try again once, as
[core section 3.2](core.md#32-https-certificates) says ("Renewals").

### 2.1 Ranges

A request carries at most one `Range` header, in one of three forms, and
no other:

| Form | Meaning |
|------|---------|
| `bytes=<first>-<last>` | bytes *first* to *last*, both included; *first* ≤ *last* |
| `bytes=<first>-` | from *first* to the end |
| `bytes=-<count>` | the last *count* bytes; *count* ≥ 1 |

- Each number is 1 to 15 decimal digits. Leading zeros are accepted. No
  spaces, no other unit.
- **Several ranges** (`bytes=0-1,5-9`) are refused, so ranges can't
  overlap. `bytes=-` is refused.
- The owning server answers a malformed `Range` with `416`, without
  reading the file. It answers a well-formed one that can't be satisfied
  with `416` too: one whose *first* is at or past the end. A *last* past
  the end means the end; a *count* above the length means the whole.
  On an empty file (size 0) no range is satisfiable, `bytes=-<count>`
  included.
- A `416` for the file carries `Content-Range: bytes */<size>`, with the
  copy's exact `size` (section 3.2).
- `Range` is meant for the file and for segments. On a playlist, a
  subtitle part, a picture part, a font, a sheet or an image, the
  viewer's server sends none, and the owning server ignores one and sends
  the whole.
- The viewer's server sends the viewer's `Range` on only when it is
  well-formed; otherwise it answers the viewer `416` itself, and fetches
  nothing.

**Checking a `206`.** The viewer's server relays a `206` only when it
answers the range asked for:

- `Content-Range` is `bytes <start>-<end>/<total>`, three decimal numbers
  (or `*` for *total*, for a segment only, when the owning server doesn't
  know it, and never for a `bytes=-<count>` range, whose *start* can't
  be checked without *total*), with *start* ≤ *end* < *total*;
- for the file, *total* is the copy's exact `size` (section 3.2), never
  `*`;
- *start* is *first* for the first two forms, and *total* − *count* (or
  0) for the third; *end* is at most *last* for the first form;
- `Content-Length`, when present, is *end* − *start* + 1.

On a segment, a `200` to a request with `Range` is the whole segment,
and is accepted. On the file, the owning server must answer a
satisfiable `Range` with `206`, and a `200` to one is a failed fetch; so
is a `200` to an unsatisfiable one, which must be answered `416`. A
`206` to a request without `Range`, or a `206` that fails these checks,
is a failed fetch. A server never trusts the body to stop at
`Content-Length`: it stops reading there.

Test cases: `ranges` (a `Range` header against a size: well-formed,
satisfiable, the bytes it names) and `fileAnswers` (the owning server's
answers for the file, as the viewer's server checks them, sections 2.1,
2.3 and 3.2) in [vectors/v0/media.json](../../vectors/v0/media.json).

### 2.2 Methods

Media URLs answer `GET` only. The viewer's server sends `GET` only,
never `HEAD`. The owning server answers any other method on a media URL,
`HEAD` included, with `405` and `Allow: GET`.

A viewer's server that presents a copy to its own clients as a plain
file answers their `HEAD` itself, from the catalog (section 3.3),
without fetching anything.

### 2.3 Statuses

[Core section 8.4](core.md#84-media-answers) lists what each status
means. The viewer's server accepts `200` and, under section 2.1, `206`;
anything else is a failed fetch, with one exception: a `416` to a range
the viewer asked for may be passed to the viewer as a `416` of its own,
without the owning server's body or headers. It never passes on the
owning server's body or headers with a failure, and it doesn't retry a
failed fetch by itself, except once after a renewed certificate
(section 2), and except by a restart under a new ticket (section 3.3).

### 2.4 Bounds

Both sides bound what one fetch, one ticket and one peer may hold. The
sizes below are the protocol's; the other numbers are each
implementation's; the bounds are required.

- **Time to the answer.** The viewer's server gives a fetch a bounded
  time to its status line and headers, and to connect and finish TLS.
  The owning server
  should send a segment's status line and headers within 30 seconds,
  even while the segment is still being made.
- **Length of a fetch.** A fetch of the file or a segment never outlives
  its ticket: when the ticket stops working (section 4), every fetch
  under it is cut. A playlist, a subtitle part, a picture part, a font, a
  sheet or an image is read whole within a bounded time.
- **Size.** Anything longer than this is refused:

| What | At most |
|------|---------|
| a playlist | 2 MiB (2,097,152 bytes) |
| a stretch of a text track (WebVTT or ASS) | 5 MiB (5,242,880 bytes) |
| a whole text track (WebVTT or ASS) | 16 MiB (16,777,216 bytes) |
| a stretch of a picture track (PGS) | 64 MiB (67,108,864 bytes) |
| a font | 10 MiB (10,485,760 bytes) |
| a preview sheet | 8 MiB (8,388,608 bytes) |
| an image | 1 MiB (1,048,576 bytes) |

  The file and segments are as long as `Content-Length` says, and no
  longer; nothing of the file lies past the copy's `size`.
- **Fetches at once.** The viewer's server bounds how many fetches it
  runs at once for one ticket and for one peer, and the owning server
  how many it serves at once for one ticket and for one peer. Over the
  bound, a new fetch waits in a bounded queue, or is refused with `503`.
  Reads that are held whole in memory (playlists, subtitles, picture
  parts, fonts, sheets, images) have a bound of their own across the
  server.

## 3. What is fetched

- **HLS.** Fetch `master.m3u8`, then the playlists it names, then their
  files: every URI in them is a bare file name of the same stream (it
  matches `^[A-Za-z0-9._-]{1,64}$`, and is not `.` or `..`), relative to
  the folder of the `master.m3u8`: `<stream root><ticket>/`, or
  `<stream root><ticket>/queue/<n>/` for a queue entry's stream. Fetch
  each the same way. Section 3.1 says what a playlist may hold.
- **File.** Fetch `file` (or a queue entry's `queue/<n>/file`), with
  ranges (section 3.2).
- **What a content part adds.** Fetched as section 3.4 says, with the
  same plain `GET`s and the ticket of the URL; what the bytes must be is
  the content part's to say ([video section 6](video.md#6-media): the
  subtitle formats, fonts, sheets, the image).

The owning server serves only what these rules allow. The viewer's server
checks again everything it relays, whatever the owning server says it
did:

- playlists as section 3.1 says, the file as section 3.2 says, and what a
  content part adds as section 3.4 and that part say; whatever fails is
  a failed fetch (section 2.3), answered `404` to the viewer.

The viewer's server sets the `Content-Type` of what it relays itself:
`application/vnd.apple.mpegurl` for playlists, and for what a content
part adds the type section 3.4 gives. The file it relays with the copy's
own `mediaType` from the catalog, whatever its top-level type, and as
`application/octet-stream` when it has none. For segments, it keeps a
`video/*` or `audio/*` type made only of letters, digits and `.+-`,
which it sends lowercased, and sends `application/octet-stream` instead
of any other. Test cases: `relayTypes` in
[vectors/v0/media.json](../../vectors/v0/media.json).

Test cases for sections 3.1 to 3.4 are in
[vectors/v0/media.json](../../vectors/v0/media.json), beside video's
subtitle cases. The largest sizes and counts (a 2 MiB playlist, 50,000
URIs, cues or events, a 10 MiB font) aren't exercised there, for the
size of the file.

### 3.1 HLS playlists

Media 0 uses a small subset of HLS (RFC 8216): complete playlists only,
audio renditions only, no encryption and no variables.

**Complete from the first fetch.** Every media playlist the owning
server serves, for a remux or a conversion, lists every segment of the
copy from the first fetch, with `#EXT-X-PLAYLIST-TYPE:VOD` and, as its
last tag, `#EXT-X-ENDLIST`. Every fetch of the same playlist under one
ticket gives the same playlist, so a viewer's server may fetch it once
and keep it. A segment may be made only when it is fetched; its answer
then waits, within the time to the answer of section 2.4. A playlist
that grows (one without `#EXT-X-ENDLIST`, or of type `EVENT`) is refused
by the viewer's server: a player reads it as live, with no total time
and no seeking.

**Several audio tracks.** When the `play` request asks for every audio
track (`audioRenditions`, [playback section 2.1](playback.md#21-the-request)),
the owning server may make them renditions. When it does, the answer
says so (`audioRenditions` `true`,
[playback section 2.3](playback.md#23-the-answer)), the master playlist
names them as `#EXT-X-MEDIA` audio renditions, one per track, and every
`#EXT-X-STREAM-INF` names their group with `AUDIO="audio"`. Switching
tracks then needs no new playback. Otherwise (`false` or absent) the
master playlist has no `#EXT-X-MEDIA`, the audio comes in the stream, and
another track is another `play`.

**What the owning server sends.** Lines end in a line feed. The first line
is exactly `#EXTM3U`. A playlist holds only these tags, and the URI lines
after `#EXT-X-STREAM-INF` and `#EXTINF`:

| Playlist | Tags allowed |
|----------|--------------|
| `master.m3u8` | `#EXTM3U`, `#EXT-X-VERSION`, `#EXT-X-INDEPENDENT-SEGMENTS`, `#EXT-X-STREAM-INF`, `#EXT-X-MEDIA` |
| a media playlist it names | `#EXTM3U`, `#EXT-X-VERSION`, `#EXT-X-TARGETDURATION`, `#EXT-X-MEDIA-SEQUENCE`, `#EXT-X-PLAYLIST-TYPE`, `#EXT-X-INDEPENDENT-SEGMENTS`, `#EXTINF`, `#EXT-X-DISCONTINUITY`, `#EXT-X-ENDLIST`, `#EXT-X-MAP` |

- **The stream served is described in the master playlist**, and only
  there: every `#EXT-X-STREAM-INF` carries `BANDWIDTH`, `CODECS` (the
  RFC 6381 strings of what is served) and, for a stream with video,
  `RESOLUTION`; `VIDEO-RANGE` (`PQ` or `HLG`) when the picture served is
  PQ or HLG; and `SUPPLEMENTAL-CODECS` when Dolby Vision is kept.
- **Segments.** In `mp4` segments, HEVC is written with the `hvc1`
  sample entry, parameter sets out of band, in a remux and a transcode
  alike. Dolby Vision with a compatible base layer (profile 8) keeps the
  base's sample entry and is announced by `SUPPLEMENTAL-CODECS`; Dolby
  Vision with no compatible base (profile 5) is written `dvh1` (`dav1`
  on AV1)
  ([playback section 2.7](playback.md#27-delivery)).
- URIs are only in URI lines, in `#EXT-X-MAP`'s `URI` attribute, written
  `#EXT-X-MAP:URI="<name>"`, and in `#EXT-X-MEDIA`'s `URI` attribute.
  Each is a bare name (section 1).
- `#EXT-X-MEDIA` has exactly these attributes: `TYPE=AUDIO`,
  `GROUP-ID="audio"`, `NAME="<track ID>"` (the track's `a<n>`, as the
  catalog and `play` give it, never its title), `LANGUAGE="<tag>"` when
  the track has a language, `DEFAULT=YES` or `NO`, `AUTOSELECT=YES` or
  `NO`, `CHANNELS="<n>"` when known (`CHANNELS="16/JOC"` for an E-AC-3
  track whose `objects` is `dolby-atmos`), and `URI="<name>"`. At most 32 of
  them, one with `DEFAULT=YES`. The viewer's server maps each rendition
  to its track by `NAME`, and names it to its viewer in its own words.
- Every media playlist has `#EXT-X-TARGETDURATION`, an integer from 1 to
  60, and `#EXT-X-PLAYLIST-TYPE:VOD`. Every `#EXTINF` duration is a
  decimal above 0 that, rounded to the nearest integer, is at most the
  target duration (RFC 8216 section 4.3.3.1).
- Not in media 0, so never sent: `#EXT-X-MEDIA` of another type than
  `AUDIO` (subtitles are fetched apart, section 3.4),
  `#EXT-X-I-FRAME-STREAM-INF` (seek-bar previews are sheets),
  `#EXT-X-KEY` and `#EXT-X-SESSION-KEY` (TLS protects the media),
  `#EXT-X-DEFINE` (no variable substitution), `#EXT-X-SESSION-DATA`,
  `#EXT-X-BYTERANGE`, and the low-latency tags (`#EXT-X-PART`,
  `#EXT-X-PRELOAD-HINT`, `#EXT-X-RENDITION-REPORT`,
  `#EXT-X-SERVER-CONTROL`, `#EXT-X-SKIP`).
- A playlist holds at most 50,000 URIs.

**What the viewer's server checks.** In this order; the first failure
refuses the whole playlist:

1. It is at most 2 MiB (2,097,152 bytes).
2. It is split into lines at each line feed; one carriage return at the
   end of a line is removed.
3. The first line is exactly `#EXTM3U`.
4. Empty lines are skipped. A line that starts with `#` is a tag or a
   comment. Any other line is a URI line, and must be a bare name, with
   nothing before or after it.
5. In a tag line, every `URI=`, in any case and anywhere in the line
   (inside another attribute's quoted string too), is followed by `"`, a
   bare name, and `"`. So an unquoted URI, an empty one, and one holding
   `/`, `?`, `#`, `%`, a scheme or a host are refused. HLS quoted strings
   have no escapes, so nothing else can end the string.
6. No line is `#EXT-X-DEFINE` or starts with `#EXT-X-DEFINE:`.
7. No line starts with `#EXT-X-KEY:` or `#EXT-X-SESSION-KEY:`.
8. It holds at most 50,000 URIs, counting URI lines and `URI=` values.
9. A media playlist (one with an `#EXTINF` line) has exactly one
   `#EXT-X-PLAYLIST-TYPE:VOD` line, exactly one `#EXT-X-ENDLIST` line,
   after its last URI line, at least one `#EXT-X-TARGETDURATION` line,
   and `#EXT-X-TARGETDURATION` and `#EXTINF` values within the bounds
   above; each `#EXTINF` is checked against the first
   `#EXT-X-TARGETDURATION`. A playlist with neither
   `#EXT-X-STREAM-INF` nor `#EXTINF` is refused.
10. In a master playlist, every `#EXT-X-MEDIA` line has `TYPE=AUDIO` and
    a `NAME`, quoted (`NAME="a1"`), that is an audio track ID of the
    playback; at most 32 of them.

Then:

- Tags outside the subset, and comments, should be dropped before the
  playlist is served to the viewer. They may be passed on unchanged
  instead: step 5 holds for them too.
- The viewer's server serves the playlist without rewriting a URI: each
  bare name resolves under the viewer's server's own route for the
  playback, which fetches the same name from the owning server.
- It may refuse to fetch a name that none of the playback's playlists
  named.

### 3.2 The file

`file` serves the copy as it is: its exact bytes, all its tracks, never
remuxed or converted.

- **Its size and identity are the catalog's.** The copy's `size` and
  `etag` are in the catalog ([video section 3.2](video.md#32-copies)),
  before anyone plays it. `size` is the file's exact length in bytes;
  `etag` changes whenever its bytes change.
- **Stable bytes.** While a copy's `etag` holds, every read of it, under
  any ticket of any playback, gives the same bytes at the same offsets.
  When its bytes change, the owning server gives it a new `etag`, and
  the change reaches the catalog as a change of the title
  ([catalog section 3.2](catalog.md#32-whole-list-or-changes)).
- **Headers.** Every `200` or `206` for the file carries
  `Accept-Ranges: bytes` and `ETag: "<etag>"`, a strong entity tag
  holding the copy's `etag`. A `206` carries `Content-Range` with the
  exact `size` as *total* (section 2.1).
- **What the viewer's server checks.** On every answer for the file:
  `ETag` is the `etag` it knows for the copy (the catalog's, or the
  `play` answer's when they disagree, as
  [playback section 2.3](playback.md#23-the-answer) says), and `Content-Range`'s *total* (or, for a `200`,
  `Content-Length`, which a `200` must carry) is its `size`. Either one different means the copy
  changed: a failed fetch, and the viewer's server reads the title
  again. It never joins bytes of two `etag`s in one read.

### 3.3 The file as a plain file

A viewer's server may present a peer's copy to its media server and its
clients as a plain file at an address of its own, before anyone plays it
([playback section 2.7](playback.md#27-delivery): the `file` delivery,
and a playback started by the first read). What it may rely on:

- **The length**: the copy's `size` from the catalog. It answers its
  clients' `HEAD` with it, and their ranges against it, with `206` and
  `Content-Range: bytes <start>-<end>/<size>`, or `416` with
  `Content-Range: bytes */<size>` past the end, the `416` without
  fetching.
- **The identity**: the copy's `etag`. It may give it to its clients as
  its own `ETag`, and keep what it read (a header, an index) for as long
  as the `etag` holds.
- **Stable bytes** (section 3.2): a read at an offset gives the same
  bytes in any playback of the copy while the `etag` holds.
- **Byte ranges**: each read of its clients becomes a fetch of `file`
  with a range under the ticket of the playback; it may fetch a larger
  range than asked (read ahead), within its bounds (section 2.4), never
  past the end.
- **A restart.** When the playback ends while the file is still being
  read (a long pause), it may start a new playback of the same copy and
  go on reading at the same byte, under the new ticket, only if the
  `etag` is unchanged ([playback section 2.7](playback.md#27-delivery):
  restarts). The first
  answer under the new ticket is checked as section 3.2 says: a
  different `ETag` or length ends the read, as a failed fetch.

It must not read the file to learn what it holds: the details come from
the catalog only ([video section 3.2](video.md#32-copies)), since each read
would start a playback on the owning server. Its media type is the
copy's `mediaType`, as section 3 says for the file.

### 3.4 Subtitles, fonts, sheets and images

Each is fetched only at a name the answer opened (section 1), read
whole, without `Range`, within its size (section 2.4); then
checked as the content part says, by its content, whatever its
`Content-Type`; then served at an address of the viewer's server's own,
to the viewer playing it only. A viewer's server never hands the owning
server's URLs to the viewer.

| What | URL (section 1) | Tickets | Answer | Relayed as |
|------|-----------------|---------|--------|------------|
| a text track as WebVTT, a stretch or whole | `subtitles/<id>/<part>`, `subtitles/<id>/whole` | playback, download | WebVTT, rebuilt ([video section 6.3](video.md#63-webvtt)) | `text/vtt; charset=utf-8` |
| an ASS or SSA track as ASS, a stretch or whole | `ass/<id>/<part>`, `ass/<id>/whole` | playback, download | ASS, rebuilt ([video section 6.4](video.md#64-ass)) | `text/x-ass; charset=utf-8` |
| a stretch of a picture track as PGS | `pictures/<id>/<part>` | playback, download | PGS (a VobSub or DVB track converted by the owning server), checked ([video section 6.5](video.md#65-picture-subtitles)) | `application/octet-stream` |
| a font | `fonts/<n>` | playback, download | a TrueType or OpenType file ([video section 6.6](video.md#66-fonts)) | `font/ttf` or `font/otf` |
| a preview sheet | `trickplay/<n>` | playback, previews | a JPEG ([video section 5.5](video.md#55-seek-bar-previews)) | `image/jpeg` |
| the image of a `local` title | `image` | image | a JPEG or a PNG ([catalog section 4.4](catalog.md#44-the-image-of-a-local-title-image)) | `image/jpeg` or `image/png` |

- **Stretches** cover 120 seconds of the copy each, part *n* from
  *n* × 120 seconds to (*n* + 1) × 120 seconds, starting 10 seconds
  early, so that a line already on screen comes along; the viewer's
  server drops the doubles. A stretch with nothing in it is a `200` with
  an empty body for a picture track, and a WebVTT part with no cue, or
  an ASS part with no event, for a text track.
- **Whole text tracks.** A text track may be fetched whole, not only in
  stretches, under a playback's or a download's ticket. An ASS or SSA
  track may be fetched as WebVTT, its styling flattened, or as ASS, in
  stretches or whole. Picture tracks come in stretches only.
- **Fonts.** The fonts a copy's ASS tracks use, `fontCount` of them in
  the answer ([video section 6.6](video.md#66-fonts)), at most 64 per
  copy, each at most 10 MiB, 64 MiB in all. The
  viewer's server tells a font by its first four bytes: `00 01 00 00` or
  `true` is TrueType (`font/ttf`), `OTTO` is OpenType (`font/otf`); any
  other is refused, a font collection (`ttcf`) included. A viewer's
  server may refuse fonts entirely, and never fetch them.
- **Sheets** are fetched under the playback's ticket, or under the
  ticket of a `previews` call with a `versionId`
  ([video section 5.6](video.md#56-previews)), which opens that copy's
  sheets only. Their count is the catalog's: ⌈`tileCount` / (`columns` ×
  `rows`)⌉ of the copy's `previewLayout`. The owning server answers
  `404` for a sheet number at or past it.
- **Images.** The image of a `local` title
  ([video section 3.10](video.md#310-the-description-of-a-local-title),
  [music section 4](music.md#4-pictures)) is fetched under the ticket of
  an `image` call
  ([catalog section 4.4](catalog.md#44-the-image-of-a-local-title-image)),
  which opens that image only. The viewer's server tells JPEG from PNG by
  content (`FF D8 FF`, or the 8-byte PNG signature), and checks and
  re-encodes it as catalog section 4.4 says.

## 4. How long a ticket works

Every ticket is of one kind, given by one call, and opens only the URLs
its kind opens:

| Kind | Given by | Opens | Works until |
|------|----------|-------|-------------|
| playback | `play`, to watch ([playback](playback.md#2-play)) | the stream (`hls` or `file`), the names section 1 opens for it (subtitles, fonts, sheets, queue entries) | its playback ends (below) |
| download | `play` with `purpose: download` ([playback section 7](playback.md#7-downloads)) | the stream, its subtitles and fonts | its playback ends, without the 2-minute rule; at most 24 hours after the answer (section 4.1) |
| scan | `play` with `purpose: scan` and `delivery: file` ([playback section 8](playback.md#8-scan-access)) | `file` only, by ranges, within the scan answer's `scanBytes` (section 4.2) | its playback ends (below); at most 10 minutes after the answer |
| previews | a `previews` call ([video section 5.6](video.md#56-previews)) | the sheets of the copy it names, by its `previewLayout` | 10 minutes after the answer |
| image | an `image` call ([catalog section 4.4](catalog.md#44-the-image-of-a-local-title-image)) | the `image` of the title it names | 10 minutes after the answer |

A playback's ticket works while its playback is live. The playback ends,
and the ticket with it, when:

- the viewer's server sends `stop` ([playback](playback.md#4-stop));
- neither `progress` nor a media request came for 2 minutes (not for a
  download);
- the pairing ends, the caller is no longer `paired`, or its key is
  suspended ([core section 12.6](core.md#126-revocation));
- the library holding the copy is no longer offered to that peer;
- or for any other ending
  [playback section 6.3](playback.md#63-how-a-playback-ends) lists.

A previews ticket is no playback: it counts against no
limit, gets no `progress` or `stop`, and fetches under it don't make it
last longer. The third and fourth endings above end it too.

Under a ticket that has stopped working, or for a URL its kind doesn't
open, the owning server answers `404`. A ticket works only for the
pairing it was given to, and never for a browser. A playback started
before a peer's address changed keeps its URLs until it ends
([core section 13.2](core.md#132-a-peer-that-learns-of-a-move)).

### 4.1 Downloads

A download's limits are in `limits`
([playback section 5](playback.md#5-limits): `downloads`,
`downloadsPerDay`, `downloadKbps`). At the media level:

- **Pacing.** The owning server sends a download ticket's bytes no
  faster than `downloadKbps`, measured over every window of 10 seconds,
  across all the ticket's fetches together. It slows the body; it never
  refuses a read for its speed.
- The viewer's server must not take a slow body for a failure: only the
  time to the answer (section 2.4) and the ticket's end bound a fetch.
- Ranges and checks are those of any `file` or `hls` stream.

### 4.2 Scans

A scan lets a media server that reads files to list them read a capped
part of a copy, outside a watched playback
([playback section 8](playback.md#8-scan-access)). At the media level:

- Every fetch under a scan ticket is a fetch of `file` with a `Range`.
  A fetch without one is answered `416`.
- **The cap.** The owning server counts the body bytes it sends under
  the ticket, a byte sent twice counted twice. Together, they never pass
  the scan answer's `scanBytes`
  ([playback section 8](playback.md#8-scan-access)), 4 MiB (4,194,304
  bytes) to 1 GiB (1,073,741,824 bytes), per scan ticket (a scan is of
  one copy); an answer without it, or with one out of range, gives the
  floor, 4 MiB, the value the caller counts on. The pairing's
  `scansPerDay` bounds the total. A range that would pass the cap is served as a `206` cut short
  at the cap (its *end* earlier than asked, as section 2.1 allows); a
  range that starts once the cap is spent is answered `416`.
- **Pace.** The owning server serves at most 2 of a scan's fetches at
  once, and answers `503` past that
  ([playback section 8](playback.md#8-scan-access)); it may pace them as
  it does a download's.
- The viewer's server passes a `416` past the cap to its reader as a
  failed read.

Test cases: `scans` in [vectors/v0/media.json](../../vectors/v0/media.json).

## 5. Security considerations

- **Media is protected by TLS only.** The ticket and the bytes aren't
  signed. HTTPS is required, with a *trusted* or *named* certificate, so
  an observer can't read a ticket, and an attacker on the path can't
  change the media without breaking TLS.
- **Hostile media.** Check playlists, and what the content parts add
  ([video section 7](video.md#7-security-considerations): subtitles,
  fonts, preview sheets), before relaying them; never let a playlist name
  anything outside the stream.
- **What remains.** Subtitles and fonts from another server reach the
  viewer's devices. Rebuilding them removes the known ways to attack a
  player or a font renderer, not all of them. A viewer's server that
  doesn't want that risk refuses fonts (section 3.4) and fetches text
  tracks in stretches, as WebVTT.
- **The file as it is.** A `file` delivery sends the copy's own bytes,
  with all its tracks and metadata, to the viewer's devices. Only its
  media details come from the catalog; nothing the file holds is
  trusted by the viewer's server.
- **Scans and downloads** read a peer's copy outside a watched playback.
  Their cap (section 4.2) and pacing (section 4.1) are the owning
  server's, enforced on every byte it sends, never left to the viewer's
  server.

## 6. Open questions

1. **Media isn't signed.** Only TLS protects what the stream root serves.
   With HTTPS required, that is true in every case; signing media (per
   segment, or signed playlists with hashes) would cost far more.

Choices of the review round of 2026-11-16, all confirmed by the owner on
2026-11-17 (they may still change as implementations show):

2. **The HLS subset** (section 3.1) was what Overfin sends. Audio
   renditions (`#EXT-X-MEDIA`) came with
   [proposal 16](../../proposals/0016-version-0-revision.md); I-frame
   playlists and encryption are left for a later version of `media`.
3. **Unknown tags** in a relayed playlist: dropped or passed on, at the
   viewer's server's choice (section 3.1), since URIs are checked in
   every tag either way.
4. **`Content-Range` checks** (section 2.1) are required of the viewer's
   server; Overfin makes them since 2026-11-16.

The choices on timestamp tags in WebVTT and on checks left to the player
moved to [video](video.md#8-open-questions) with the WebVTT and PGS rules.

**Choices this draft made, confirmed by the owner on 2026-11-17** (when the video rules were moved
out of this part on 2026-11-17):

5. **Bounds stay here**: the sizes of subtitle parts, picture parts,
   fonts, sheets and images (section 2.4), and the rule that they are
   fetched without `Range` (section 2.1), are listed with the other
   bounds, though only the content parts use them.
6. **URL checks stay here** (section 1): the `<id>` name that video's
   URLs add follows the same pattern as `<name>`.

**Choices made when proposal 16 was written in** (2026-11-22, where the
proposal gives no number or form; they may change as implementations
show):

7. **URL forms**: `subtitles/<id>/whole`, `ass/<id>/<part>` and
   `ass/<id>/whole`, `fonts/<n>` (0 to 7; below `fontCount` since
   proposal 17) and `image` (section 1).
   The forms of every URL a content part adds are here; video says only
   what their bytes are.
8. **Sizes**: a whole text track 16 MiB; a font 10 MiB, an image 1 MiB
   and the scan cap 32 MiB, reading the proposal's MB as MiB
   (sections 2.4 and 4.2). Since proposal 17, the scan cap is the scan
   answer's `scanBytes`, 4 MiB to 1 GiB.
9. **Tickets**: a `local` title's image comes under an `image` ticket,
   of 10 minutes; a scan ticket follows a playback's 2-minute rule, and
   lasts 10 minutes at most (section 4).
10. **The file**: `ETag: "<etag>"` and `Accept-Ranges: bytes` on every
    answer, a `206` required for a satisfiable range, never `*` as its
    total; no conditional headers (sections 2, 2.1 and 3.2).
11. **HLS**: target duration 1 to 60 seconds; renditions named by track
    ID, in one group `audio`, at most 32; a segment's headers within 30
    seconds (sections 2.4 and 3.1).
12. **Fonts** as `font/ttf` or `font/otf`, told by their first bytes; no
    font collections. **ASS** fetched as ASS in stretches or whole, at
    URLs of its own, relayed as `text/x-ass` (section 3.4).
13. **Pacing** of downloads over windows of 10 seconds; a scan read past
    the cap cut short rather than refused; a scan read without `Range`
    refused (sections 4.1 and 4.2).
14. **`405`** for `HEAD` comes with `Allow: GET`, and answers every
    other method but `GET` too (section 2.2).

**Proposal 17** (adopted 2026-11-22): URLs built from the ticket, the
owning server serving exactly their names, the copy's `mediaType` for
the relayed file, the master playlist's attributes and `hvc1` in `mp4`
segments, the scan cap in the scan's answer, 64 fonts a copy, the `HEAD`
rule kept without a product's reason, and the answer's
`audioRenditions` deciding the renditions. Settled while msfp-go was
written (2026-11-22):

15. **A segment's `206` with `*` as its total** is a failed fetch when it
    answers a `bytes=-<count>` range: its *start* can't be checked
    (section 2.1).

Proposal 17, written in on 2026-11-22: choices where its text gave no
form, to be confirmed:

16. **`RESOLUTION`** is required only for a stream with video: an
    audio-only stream has no picture to describe (section 3.1).
17. **No new check for the master playlist's attributes**: the viewer's
    server's steps (section 3.1) don't refuse a playlist that lacks them;
    the conformance suite checks owning servers.
18. **Segments keep their own type rule** (section 3): only the file is
    relayed with the copy's `mediaType`.
