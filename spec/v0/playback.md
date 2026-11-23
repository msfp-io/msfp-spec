# MSFP version 0: the playback part, version 0

**Draft.**

The `playback` part lets a viewer's server start, keep and stop a
playback of a title a peer holds, within limits the owning server sets:
to watch it, to download it, or to let a media server read the start of
its file to list it. Which titles it plays, and what their playbacks
carry besides, is said by the content parts,
[video](video.md#5-playing-a-film-an-episode-or-an-extra) and
[music](music.md#5-playing-a-track) (section 2.4). It needs the
[core](core.md), [catalog](catalog.md) (it plays title keys from
`titles`) and [media](media.md) (the viewer's server builds media URLs
from its answers' tickets). If either is off between two servers,
`playback` is off too ([core section 10.3](core.md#103-choosing)). Its calls follow
[core section 8](core.md#8-common-rules-for-every-call): signed `POST`s
under the peer's base path, answered only to peers in state `paired`. A
server that doesn't speak `playback` with the caller answers its calls
with a signed `404` `not-supported`. A peer that can't be called
([core section 7.5](core.md#75-a-peer-that-cant-be-called)) is only ever
the viewer's server: nobody calls `play`, `progress` or `stop` on it, and
nothing of it is offered for playing.

The words "must", "must not" and "should" carry their usual meaning in
specifications.

## Contents

1. [Terms](#1-terms)
2. [play](#2-play)
3. [progress](#3-progress)
4. [stop](#4-stop)
5. [Limits](#5-limits)
6. [What the owning server enforces](#6-what-the-owning-server-enforces)
7. [Downloads](#7-downloads)
8. [Scan access](#8-scan-access)
9. [Security considerations](#9-security-considerations)
10. [Open questions](#10-open-questions)

## 1. Terms

- **Owning server**: the server that holds a title's copy and streams it.
- **Viewer's server**: the server whose signed-in person watches a title
  the owning server holds. The person's browser only ever talks to the
  viewer's server.
- **Copy**: one version of a title on the owning server: one file, or
  several files played one after the other as one (parts). A title may
  have several copies (versions). Positions, `durationMs` and the
  content parts' markers are in the whole copy. The catalog describes
  each copy before any playback: its version ID, length, media type,
  exact size, `etag` and tracks ([video section 3.2](video.md#32-copies),
  [music section 3.3](music.md#33-a-tracks-copies)). A copy's
  `etag` changes whenever its bytes change.
- **Playback**: one stream the owning server runs for the viewer's
  server. It has a playback ID and a ticket.
- **Ticket**: an opaque string in `play`'s answer, from which the
  viewer's server builds the media URLs. It is the only credential for
  fetching the media ([media section 1](media.md#1-the-urls)).
- **Delivery**: what the viewer's server asks to be served: the file as
  it is, a remux, or a transcode (section 2.7). Absent, the baseline
  every viewer's server plays (section 2.7).
- **Purpose**: why a playback is started: `play` (a viewer watches it),
  `download` (section 7) or `scan` (section 8).
- **Viewer session**: one person of the viewer's server watching on one
  device. A playback serves one viewer session (section 2.7).

## 2. play

`play`, under the peer's base path. The viewer's server asks the owning
server to start a playback of a title for one viewer session.

### 2.1 The request

| Field | Type | Required | Content |
|-------|------|----------|---------|
| `title` | string | yes | the key of a title `play` takes (section 2.4), as the owning server sent it in `titles` |
| `purpose` | string | no | `play`, `download` (section 7) or `scan` (section 8); absent means `play` |
| `delivery` | string | no | `file`, `remux` or `transcode` (section 2.7); absent: the baseline (section 2.7) |
| `mediaType` | string | with `delivery` `remux` only, and then required | the remux's media type: subtype `mp2t` or `mp4` (section 2.7) |
| `transcode` | transcode object | no; with `delivery` `transcode` only | the targets of the transcode (section 2.7) |
| `audioRenditions` | boolean | no | `true`: when served as HLS, every audio track of the copy as a rendition ([media section 3.1](media.md#31-hls-playlists)), so that switching tracks needs no new playback; the answer says whether the owning server made them (section 2.3) |
| `startPositionMs` | integer | no | where to start, in milliseconds; 0 to 8,640,000,000 (100 days) |
| `maxBitrateKbps` | integer | no | the viewer's bitrate cap, kbit/s; 0 to 10,000,000; 0 means none |
| `versionId` | string | no | the copy to play: a copy's `id` in the catalog; without it, the first copy (section 2.6) |
| `audioTrack` | string | no | the audio track to play: a track ID of that copy in the catalog, `a` and a number from 0 to 999 without leading zeros (`a01` is invalid); without it, the file's default audio (section 2.6) |

The request carries no language and names a subtitle only to burn it
in (`transcode.burnSubtitle`, [video section 5.1](video.md#51-the-request)).
The content parts may add members: video the subtitle burned in;
[music](music.md#5-playing-a-track) the next tracks of a queue, at most
5, each with its own `versionId` and `delivery` if it wants, which the
owning server prepares under the same playback, and which count once
against `streams`.

The request says nothing of the viewer's device. The viewer's server has
every copy's facts in the catalog, and knows what its device plays: it
chooses the copy and the audio track, and asks for what it wants with
`delivery`, or leaves `delivery` out for the baseline that every
viewer's server plays (section 2.7).

**Values.** Requests follow the rule for unknown values of
[core section 8.5](core.md#85-unknown-values). `purpose` and `delivery`
name MSFP's own steps: a `purpose` not of its list is refused (`400`); a
`delivery` that is a token but not of its list is refused with `409`
`delivery-refused`, since it names what to serve. So are a remux's media
type, a transcode codec and a `range` that are well-formed but that the
owning server doesn't make (section 2.7). A value that isn't a token, or
a `mediaType` that isn't a media type (core 8.5), is malformed, and
refused (`400`).

Test cases: `requests` in
[vectors/v0/playback.json](../../vectors/v0/playback.json) (members,
combinations of `purpose` and `delivery`, unknown values), and
`queueEntries` (an entry's own copy and delivery).

### 2.2 What the owning server does

1. It refuses the request (`400`) when `title` isn't the key of a kind
   `play` takes in a content part on between the two servers
   (section 2.4), or a number is out of range: a negative number, or one
   above its range. It refuses (`400`) too a `purpose` not of its list,
   a `delivery` that isn't a token, a `mediaType` that isn't a media type
   (section 2.1), a `transcode` object that breaks section 2.7's forms,
   and a combination of `purpose` and `delivery` that section 2.7
   doesn't allow.
2. It refuses (`409` `stream-limit-reached`) when the pairing's limit for
   the purpose is 0: `streams` for `play`, `downloads` for `download`,
   `scansPerDay` for `scan` (section 5).
3. It looks for its copies of that exact key in the libraries it offers
   the caller. None: `404`.
4. It takes the copy `versionId` names, if it is one of them; else the
   first copy (section 2.6). Then it refuses (`400`) a malformed
   `audioTrack`, and a malformed `transcode.burnSubtitle`: `s` and a
   number from 0 to 999 without leading zeros. A track ID's number has
   no leading zeros, like other numbers of MSFP: `a01` and `s007` are
   malformed, and so is an empty string.
5. It refuses (`409` `stream-limit-reached`) when the purpose's limits
   are used: for `play`, the pairing's streams all in use; for
   `download`, its downloads all in use, or `downloadsPerDay` reached;
   for `scan`, `scansPerDay` reached.
6. The bitrate cap is the lowest non-zero of: `maxBitrateKbps`,
   `transcode.maxBitrateKbps`, the owning server's own cap, and the
   pairing's `bitrateKbps`. With none, there is no cap. It applies to
   `play` only; a download's speed is capped by `downloadKbps`
   (section 7), and a scan's reads by section 8.
7. With `delivery`, it does what is asked, or refuses (`409`
   `delivery-refused`): a `delivery`, a remux's media type, a transcode
   codec, a `copy` or a `range` it doesn't make (sections 2.1 and 2.7),
   bandwidth (the copy's overall rate, `size` × 8 / `durationMs`, above
   the cap of step 6, for `file` or `remux`), its policy, or a delivery
   it can't make of that copy. It never serves another delivery than the
   one asked. Without `delivery`, it serves the baseline (section 2.7).
8. It starts the playback within the pairing's limits and the server's
   transcode budget (section 6). Over them: `409` `stream-limit-reached`,
   or `503` `server-busy`. When it has no way to play the copy at all:
   `409` `not-playable`.
9. It answers `200`, signed, with the playback.

A copy that holds several episodes starts where
[video section 5.2](video.md#52-several-episodes-in-one-copy) says.

### 2.3 The answer

Answer `200`: the playback object. It says only what the catalog can't:
the playback, its ticket, and what plays. It carries no URLs: the
viewer's server builds every media URL from the owning server's stream
root, the ticket and a fixed name
([media section 1](media.md#1-the-urls)), so that it never fetches an
address a peer wrote. The viewer's server checks and cleans every field,
as listed.

| Field | Type | Content | What the viewer's server requires or does |
|-------|------|---------|-------------------------------------------|
| `id` | string | the playback ID | must match `^[A-Za-z0-9_-]{1,64}$` |
| `ticket` | string | the playback's ticket | must match `^[A-Za-z0-9_-]{1,64}$` |
| `protocol` | string | `hls` or `file` | anything else: refused; without `delivery`, or with `remux` or `transcode`, must be `hls`; with `file`, `file` |
| `method` | string | `direct`, `remux` or `transcode` | anything else: refused; with `delivery`, must be the one asked (`direct` for `file`) |
| `etag` | string | the `etag` of the copy playing, as the catalog gives it | must be 1 to 64 characters of `A–Z a–z 0–9 - _` |
| `positionMs` | integer | where the playback starts, in the copy | required, an integer from 0 to 8,640,000,000 (100 days) |
| `versionId` | string | the copy playing | required, must match `^[A-Za-z0-9_-]{1,64}$` |
| `audioTrack` | string, optional | the audio track playing, `a<n>` | another form: cleared |
| `audioRenditions` | boolean, optional | `true`: the playlist carries every audio track as a rendition ([media section 3.1](media.md#31-hls-playlists)); `false` or absent: the owning server made none | anything but a boolean: refused; `true` with `protocol` `file`: dropped |
| `maxBitrateKbps` | integer, optional | the cap in effect | not an integer: refused; out of 0 to 10,000,000: dropped |
| `scanBytes` | integer | scans only: the bytes the scan's ticket may read in all (section 8) | with `purpose` `scan`: 4,194,304 to 1,073,741,824; missing, not an integer or out of range: 4,194,304 (4 MiB); with another purpose: ignored |

The content parts may add members: [video section 5.3](video.md#53-the-answer)
adds `subtitles`, the subtitle tracks served apart, and `fontCount`, the
copy's fonts. Nothing else of the copy is in the answer: its tracks,
length and seek-bar previews are the catalog's.

**The playback ID** follows the form of other opaque IDs. The owning
server makes it from at least 128 bits of a cryptographically secure
random source, as it makes tickets ([media section 1](media.md#1-the-urls)),
so that nobody can guess another pairing's playback.

**The answer wins.** When the answer's `etag` differs from the
catalog's (the copy changed since the catalog was read), the viewer's
server goes by the answer for this playback, and reads the title again
from `titles`.

**Audio renditions.** The owning server makes renditions only when the
request asks for them (`audioRenditions`), and may make none: a request
for them is never refused. The answer's `audioRenditions` says which.
With `false`, switching the audio track means a new playback.

**Version IDs** (`versionId`) are the `id`s of the title's copies in
the catalog, which lists every version. They follow the rule of library
IDs ([catalog section 2.1](catalog.md#21-library-ids)): opaque, chosen
by the owning server, not derived from its media server's IDs without a
key. They need to be stable only for the life of a title's copies, since
a viewer's server passes them back only in a later `play` of the same
title. Track IDs (`a<n>`, `s<n>`) are those of the copy in the catalog
([video section 3.3](video.md#33-streams-and-tracks)).

When the answer breaks a "must" above, the viewer's server sends `stop`
for that `id` and gives up.

To change the audio track, the version or the quality, the viewer's
server stops the playback and starts a new one with `play`. It waits up
to 5 seconds for the `stop` to be answered first, so the old playback
doesn't count against the limit. Switching the audio track needs no new
playback when the copy is delivered as `file` (it holds every track) or
served with audio renditions (`audioRenditions` `true` in the answer).

Test cases: `answers` in
[vectors/v0/playback.json](../../vectors/v0/playback.json).

### 2.4 What the content parts add

Each content part says which of its kinds `play` takes, what its
baseline is (what the owning server serves without `delivery`,
section 2.7), and which members it adds to the request and the answer:

| Content part | `play` takes | Baseline | Adds |
|--------------|--------------|----------|------|
| `video` | films, episodes and extras | H.264 video and AAC audio in complete HLS; for a copy with no `video` (an extra such as a theme song), AAC alone (section 2.7) | a subtitle burned in as a transcode target; the subtitle tracks served apart and the copy's fonts in the answer ([video section 5](video.md#5-playing-a-film-an-episode-or-an-extra)) |
| `music` | tracks | AAC in complete HLS, audio only | the next tracks of a queue, each with its own copy and delivery if it wants; audio limits in `limits` ([music section 5](music.md#5-playing-a-track)) |

A viewer's server ignores the members a content part adds when the title
played isn't of that part.

### 2.5 Refusals

`400` `invalid-request`, `404` `not-found`, `409`, `503`, `500` as above,
`429` `rate-limited` ([core section 8.3](core.md#83-limits-and-overload)),
and `404` `not-supported` when `playback` is off. A `409` is one of three
refusals, told apart by the problem `type` of its signed body:

| Status | `type` | Meaning | The viewer's server tells the viewer |
|--------|--------|---------|--------------------------------------|
| 409 | `stream-limit-reached` | the limits of the purpose are all in use or reached, or the limit is 0 (section 2.2, steps 2 and 5) | busy now |
| 409 | `delivery-refused` | the owning server won't serve the delivery asked, or doesn't make the delivery, media type, codec, `copy` or `range` it names (section 2.2, step 7) | nothing yet: the viewer's server asks again with another delivery, or without `delivery`; when no delivery it can use is left, not playable (for a download: download not available) |
| 409 | `not-playable` | the owning server has no way to play its copy | not playable |
| 503 | `server-busy` | the server's transcodes are all in use | busy now |

The owning server must send `not-playable` with `409` only for a title it
can't play at all, `delivery-refused` for a delivery it won't make of a
copy it can play, and `stream-limit-reached` for its limits. A viewer's
server must read the `type` of a signed `409`: `not-playable` means not
playable; `delivery-refused` means asking for something else; any other
`type`, or a body without one, means busy now. A body that isn't JSON
counts as a body without a `type`. A signed `429` or `503`, an unsigned
refusal, and no answer at all mean busy now, as
[core section 8.3](core.md#83-limits-and-overload) says. Any other signed
refusal (`400`, `404`, `500`) means not playable.

### 2.6 Choosing the copy and the tracks

The viewer's server chooses: it has every copy's facts in the catalog
and knows its device. The owning server only plays what it is asked.

- The catalog lists a title's `copies` in the owning server's order of
  preference, best first ([video section 3.7](video.md#37-the-order-of-copies)).
- **The copy.** A `versionId` that names one of the title's copies: that
  copy plays. Without one, or with one that names none of them, the
  first copy plays.
- **The audio.** An `audioTrack` that names an audio track of that copy:
  that track plays. Without one, or with one that isn't a track of the
  copy, the file's default audio plays: the track the file marks
  default, else its first audio track; never a choice a person of the
  owning server saved.
- **The subtitles** are the viewer's server's own choice, among the
  catalog's tracks and those the answer serves apart
  ([video section 5.3](video.md#53-the-answer)). The request names a
  subtitle only to burn it in (`transcode.burnSubtitle`).
- The answer says what plays (`versionId`, `audioTrack`).

With `delivery` `file`, the copy comes with all its tracks, and the
viewer's player selects the audio itself; the answer's `audioTrack` is
the one chosen as above, which the player starts with.

**How it's served.** With `delivery`, as asked (section 2.7). Without
it, the baseline (section 2.7): `method` is `direct` or `remux` when the
copy's streams already are within the baseline, and `transcode`
otherwise. A transcode whose video is `copy` counts as a transcode only
when its audio is converted (section 2.7). The limits then decide:

| What the copy and track need | Limits (section 6) | Result |
|------------------------------|--------------------|--------|
| `direct` or `remux` | a stream free | `200` |
| `transcode` | a stream free, a transcode free for the pairing and in the server's budget | `200` |
| `transcode` | the pairing's `transcodes` all used or 0, or the server's budget used | `503` `server-busy` |
| anything | the pairing's `streams` all used, or 0 | `409` `stream-limit-reached` |
| no way to play the copy | any | `409` `not-playable` |

A `play` with `delivery` `file` or `remux` never transcodes: it is how a
viewer's server forbids a transcode. One with `delivery` `transcode`
needs a transcode free, as the second and third rows say, unless its
video and its audio are both `copy`.

The owning server doesn't fall back to another copy, track or quality to
avoid a transcode.

Test cases: `requests` and `deliveries` in
[vectors/v0/playback.json](../../vectors/v0/playback.json).

### 2.7 Delivery

**The deliveries.**

| `delivery` | What is served | `method` in the answer | How it is fetched |
|------------|----------------|------------------------|-------------------|
| `file` | the copy as it is, all its tracks | `direct` | by byte ranges ([media section 3.2](media.md#32-the-file)) |
| `remux` | the copy's streams, unchanged, in the media type the request names | `remux` | as complete HLS ([media section 3.1](media.md#31-hls-playlists)) |
| `transcode` | the copy converted to the targets of `transcode` | `transcode` | as complete HLS ([media section 3.1](media.md#31-hls-playlists)) |
| absent | the baseline (below) | any | as complete HLS ([media section 3.1](media.md#31-hls-playlists)) |

**The baseline.** Without `delivery`, the owning server serves what
every viewer's server must be able to play, in complete HLS:

- for a copy with video (a film, an episode, an extra): H.264 at most
  High profile and level 4.1, 8-bit 4:2:0, progressive, SDR in BT.709
  (H.273 primaries, transfer and matrix 1); and AAC-LC, at most 2
  channels, at most 48 kHz;
- for a copy with no `video` (a track, or an extra such as a theme
  song): the same AAC, audio only.

It may leave a stream unchanged only when it is already within that; it
never serves the file without `delivery`.

**A copy made of several parts** (section 1) is never delivered as
`file`: a `play` with `delivery` `file` of such a copy, whatever its
`purpose`, is refused with `409` `delivery-refused`; it may be asked for
as `remux` or `transcode`, and without `delivery` it is served as the
baseline ([media section 1](media.md#1-the-urls)). So it is neither
presented as a plain file, nor downloaded as it is, nor scanned. The
size the viewer's server presents for a `file` is the copy's exact
`size` in the catalog ([video section 3.2](video.md#32-copies)).

**Purpose and delivery.** Combinations of `purpose` and a `delivery` of
the list not in this table are refused (`400`); a `delivery` that is a
token not of the list gets `409` `delivery-refused` (section 2.1),
whatever the purpose:

| `purpose` | `delivery` allowed |
|-----------|--------------------|
| `play` (or absent) | `file`, `remux`, `transcode`, absent |
| `download` | `file`; `transcode` with `transcode.maxHeight` or `transcode.maxBitrateKbps` (section 7) |
| `scan` | `file` only (section 8) |

**Remux.** `mediaType`, required with `delivery` `remux` and only
there, is a media type ([core 8.5](core.md#85-unknown-values)) of
subtype `mp2t` (MPEG-TS segments) or `mp4` (fragmented MP4 segments),
the two segment formats of HLS: `video/mp2t` or `video/mp4`, or, for a
copy with no `video`, `audio/mp2t` or `audio/mp4`. A value that isn't a
media type is refused (`400`); a well-formed one the owning server
doesn't make, another subtype or the wrong top-level type among them,
gets `409` `delivery-refused`. A remux is always served as complete HLS.

**Transcode targets.** The `transcode` object, with `delivery`
`transcode` only (with any other `delivery`, or none, it is refused,
`400`). All members are optional; one left out takes the baseline's
value.

| Field | Type | Values | Left out | The owning server serves |
|-------|------|--------|----------|--------------------------|
| `videoCodec` | string | a codec token of the [registry](../registry.md) (`h264`, `hevc`, `av1`, `vp9`), or `copy` | `h264` | the picture in that codec, or unchanged with `copy`; ignored for a copy with no `video` |
| `maxHeight` | integer | 480, 720, 1080, 1440, 2160 or 4320 | the copy's own height | a picture at most that tall, never scaled up; ignored with video `copy` and for a copy with no `video` |
| `maxBitrateKbps` | integer | 1,000 to 10,000,000, a whole 1,000 | no cap but step 6's | at most that overall bitrate (with the cap of section 2.2, step 6, the lower wins) |
| `audioCodec` | string | a codec token of the registry (`aac`, `ac3`, `eac3`, `opus`, `flac`), or `copy` | `aac` | the audio in that codec, or unchanged with `copy` |
| `audioChannels` | integer | 2, 6 or 8 | 2 | at most that many channels, never upmixed; ignored with audio `copy` |
| `range` | string | `sdr` or `keep` | `sdr` | `sdr`: tone-mapped to SDR, 8-bit, BT.709; `keep`: the copy's transfer and primaries kept, 10-bit, HDR10+ kept where the codec carries it; ignored with video `copy` and for a copy with no `video` |

- **Values.** A codec that is a token the owning server doesn't encode,
  or doesn't know, and a `range` that is a token not of its list, are
  refused with `409` `delivery-refused`; a value that isn't a token, or
  a number not of its values, is refused (`400`). The video part adds
  one target ([video](video.md#51-the-request): `burnSubtitle`, one
  subtitle track burned into the picture).
- **`copy`** (a value of `videoCodec` and `audioCodec`, like `delivery`
  a step of MSFP's own, not a registry token): that stream is served
  unchanged. `409` `delivery-refused` when the segment format can't
  carry it. A copied stream takes none of the targets that would change
  it (`maxHeight` on video, `audioChannels` on audio). `burnSubtitle`
  with video `copy` is refused, `409` `delivery-refused`. A transcode
  whose video is `copy` counts as a transcode (sections 2.6 and 6) only
  when its audio is converted.
- **`range`.** Dolby Vision is dropped unless the video is `copy`. An
  owning server that can't make the range asked: `409`
  `delivery-refused`.
- **Exactly as asked.** The owning server converts to exactly the
  targets given, and the baseline's values for those left out, or
  refuses with `delivery-refused`; it never serves something close.
  It has no bitrate ceiling of its own beyond step 6's caps.

**Segments.** In `mp4` segments, of a remux or a transcode alike, HEVC
is written with the `hvc1` sample entry, parameter sets out of band.
Dolby Vision with a compatible base layer (`dv.compatibility` not 0,
such as profile 8) keeps the base's sample entry (`hvc1`, `av01`) and
is announced by the master playlist's `SUPPLEMENTAL-CODECS`
(`dvh1…/db1p`); Dolby Vision with no compatible base (profile 5) is
written `dvh1` (`dav1` on AV1). The master playlist
describes the stream served ([media section 3.1](media.md#31-hls-playlists)).

**A playback may start on the first read.** A viewer's server may present
a peer's copy to its media server and clients as a plain file, at an
address of its own, before anyone plays it, with the length, media
type, tracks and size the catalog gives. It calls `play` when its media server
or a client first reads that address, not before: presenting a file, and
listing it, start nothing on the owning server. Reads of the address
then go to that playback's `file`.

**Restart.** When a playback with `delivery` `file` has ended while its
file was still being read (a long pause: the 2 idle minutes of
section 6.3), a viewer's server may start a new playback of the same copy
(`versionId` its `id`, same `purpose`, same `delivery`) and go on reading
at the same byte, **only if the new answer's `etag` is the `etag` it was
reading**. It sends a new `play`; the owning server treats it as any
other, against the same limits. When the `etag` differs, or the new
`play` is refused, the viewer's server must not join bytes of the two: it
stops the new playback if there is one, fails the read, and reads the
title again from `titles`. A restart is made only for a read that
arrives: a viewer's server never keeps a playback live, or starts one,
ahead of a read.

**One playback per viewer session**, not per title: two people watching
one title, or one person on two devices, are two playbacks, and all the
reads of one viewer session go through its one playback. A viewer's
server must not share one playback, or its ticket, between viewer
sessions.

Test cases: `deliveries`, `baseline` and `restarts` in
[vectors/v0/playback.json](../../vectors/v0/playback.json).

## 3. progress

`progress`, under the peer's base path. The viewer's server tells the
owning server that a playback is still watched, and where.

Request:

| Field | Type | Required | Content |
|-------|------|----------|---------|
| `id` | string | yes | the playback ID from `play`, `^[A-Za-z0-9_-]{1,64}$` |
| `positionMs` | integer | no | where the viewer is; 0 to 8,640,000,000 |
| `at` | integer | no | when the viewer was there, Unix seconds; at most 30 days before the owning server's clock and at most 5 minutes after it; absent means now |

Answer `200`: `{}`.

Refusals: `400` for a malformed body, an `at` out of its bounds and an
empty `title` ([music section 5.2](music.md#52-the-queue)) included;
`410` `playback-stopped` when the playback isn't live, or isn't this
pairing's, except for a download's late reports (section 7).

The viewer's server should send `progress` while the viewer watches,
paused too: no more often than every 45 seconds, but often enough that 2
minutes never pass without one (below; once a minute does), and at once
when the viewer pauses or resumes. The
owning server ends a playback that has neither a progress report nor a
media request for 2 minutes ([media section 4](media.md#4-how-long-a-ticket-works)),
except a download (section 7).
An open media response held by a paused player counts as use.

`at` is for progress made away from the viewer's server (a download
watched offline, section 7), reported when the device syncs, as it would
have been during a playback. The viewer's server sends reports with `at`
in the order of their `at`; the owning server keeps, for a playback, the
report with the latest `at`, and ignores one older than a report it
already has. A `progress` of a scan (section 8) is answered `200` and
changes nothing. The content parts may add members:
[music section 5.2](music.md#52-the-queue) adds the queued track
playing.

Test cases: `reports` in
[vectors/v0/playback.json](../../vectors/v0/playback.json) (`at` bounds).

## 4. stop

`stop`, under the peer's base path. The viewer's server tells the owning
server a playback is over.

Request:

| Field | Type | Required | Content |
|-------|------|----------|---------|
| `id` | string | yes | the playback ID, `^[A-Za-z0-9_-]{1,64}$` |
| `positionMs` | integer | no | where the viewer stopped; 0 to 8,640,000,000 |
| `at` | integer | no | when the viewer stopped, Unix seconds; bounds as in `progress` (section 3); absent means now |

Answer `200`: `{}`, whether or not the playback was live. A playback ID
of another pairing changes nothing.

Refusals: `400` for a malformed body, a position or an `at` out of range
included.

## 5. Limits

The owning server sends, in its `libraries` answer
([catalog section 2](catalog.md#2-libraries)), what the caller may use of
it. It sends `limits` only when `playback` is on between the two.

| Field | Type | Range | Content |
|-------|------|-------|---------|
| `streams` | integer | 0 to 1,000 | playbacks of purpose `play` at the same time |
| `transcodes` | integer | 0 to `streams` | of those, how many may be transcoded |
| `bitrateKbps` | integer | 0 to 1,000,000 | a bitrate cap in kbit/s; 0 means none |
| `downloads` | integer | 0 to 1,000 | downloads at the same time (section 7); 0: no downloads |
| `downloadsPerDay` | integer | 0 to 10,000 | downloads started in any 24 hours; 0 means no daily cap besides `downloads` |
| `downloadKbps` | integer | 0 to 1,000,000 | the speed cap of each download, kbit/s (section 7); 0 means none |
| `scansPerDay` | integer | 0 to 1,000,000 | scans started in any 24 hours (section 8); 0: no scan access |

Absent, `downloads`, `downloadsPerDay`, `downloadKbps` and `scansPerDay`
are 0: no downloads, no scan access. The bytes one scan may read are not
a limit here: each scan's answer states them (`scanBytes`, section 8). A
content part may add members:
[music section 5.1](music.md#51-limits) adds `audioStreams` and
`audioTranscodes`, in the same bounds as `streams` and `transcodes`.

The caller uses the limits only to avoid offering its people what the
owning server would refuse: it shows Download (section 7) only for a
peer whose `downloads` is above 0, asks for scans (section 8) only
of a peer whose `scansPerDay` is above 0, and lets its media server read
no more than the scan answer's `scanBytes` in one scan. The owning server enforces them
itself (section 6). A caller ignores limits out of range. A server that doesn't
enforce limits sends none.

What the caller keeps of `limits`:

- a member that isn't an integer makes the caller ignore the whole
  `limits` object, as [catalog section 2.2](catalog.md#22-what-the-caller-keeps)
  says of members of the wrong type;
- a member out of its range is ignored alone, the others kept;
- `transcodes` is bounded by `streams` only when `streams` is kept:
  with `streams` ignored or absent, `transcodes` is checked against 0 to
  1,000 alone. `audioTranscodes` and `audioStreams` likewise.

Test cases: `limits` in
[vectors/v0/playback.json](../../vectors/v0/playback.json).

**Shared limits.** Limits may be shared with other peers: a group's pool
is shared by all its members ([groups section 9](groups.md#9-sharing-and-limits)),
downloads and scans as streams. A peer linked through a group is sent the most generous of its relations'
limits, and may be refused (`409` `stream-limit-reached`) before reaching
them, its pool being in use by others. A playback counts against exactly
one pool: the first with room, the individual relation's own limits
first. A day's counts (`downloadsPerDay`, `scansPerDay`) are kept in the
pool the playback counted against.

## 6. What the owning server enforces

The owning server decides alone. The viewer's server can't widen
anything.

- **What is played.** `play` finds copies only in libraries offered to
  that peer ([catalog section 5](catalog.md#5-what-is-shared)).
- **Stream caps.** Per pairing: at most `streams` playbacks of purpose
  `play` at once (0 refuses every such `play`), and at most `transcodes`
  of them transcoded. Downloads count against `downloads` and
  `downloadsPerDay`, never `streams` (section 7); scans against
  `scansPerDay` only (section 8). Across the server: its transcode
  budget, less the transcodes it keeps for its own people; a download's
  transcode takes one from that budget too, but none of the pairing's
  `transcodes`. A server whose media server has no such budget makes
  one, an operator setting counting its own people's transcodes and its
  peers' together. A remux found to re-encode the video counts as a
  transcode from then on, when the owning server can tell. The bitrate
  is the lowest cap of section 2.2, step 6.
- **Off by default.** Downloads and scan access are off until the
  owning server's operator allows them, per pairing and per group:
  `downloads` 0 and `scansPerDay` 0 (section 5). So are `local` titles,
  not shared until the operator shares them
  ([video section 4.5](video.md#45-the-local-scheme)). The other limits
  are the operator's choice.
- **Media.** Only through the ticket, only from a client that doesn't
  look like a browser, only while the playback is live
  ([media section 4](media.md#4-how-long-a-ticket-works)). A download's
  reads at `downloadKbps` at most (section 7); a scan's reads within its
  cap (section 8).
- **What a peer never learns:**
  - nothing about the viewer: no account, profile, name, email, address,
    IP, browser or device. Nothing in `play`, `progress` or `stop` names a
    person.
  - not the viewer's device: a request carries no capabilities, only the
    delivery asked, and transcode targets in steps (sections 2.1 and
    2.7).
  - of the owning server: never files, paths, or its media server's item
    IDs, not even inside version IDs (section 2.3).

  What a peer does learn: the title, the purpose, the copy and audio
  track asked, the delivery, the start position, the transcode targets
  and the bitrate cap, and progress over time (with `at`, the times of
  offline viewing too), for one unnamed viewer session of the other
  server.

### 6.1 A playback's states

A playback exists only on the owning server, from a `200` to `play`. It
is in one of three states:

| State | How it gets there | `progress` | `stop` | Media with its ticket |
|-------|-------------------|------------|--------|-----------------------|
| **started** | `play` answered `200` | `200`; the playback becomes active | `200`; the playback ends | served; the playback becomes active |
| **active** | a `progress` or a media request since it started | `200` | `200`; the playback ends | served |
| **ended** | `stop`, expiry, or one of the endings of section 6.3 | `410` `playback-stopped` (a download's late reports: `200`, section 7) | `200`, nothing changes | `404` |

- **started** and **active** are both *live*: the ticket works, and the
  playback counts against the limits of its purpose. They differ only in that an active
  playback has been used since it started; a server may treat them as one.
- **ended** is final. A playback never comes back to life; the viewer's
  server starts a new one with `play`, with a new ID and a new ticket
  (a restart, section 2.7, is such a new playback). A download's late
  reports (section 7) are recorded without making it live.
- A playback ID of another pairing, or one the owning server never gave,
  is treated as ended: `410` on `progress`, `200` on `stop`, and nothing
  changes.

**Duplicate and late calls.**

- A second `stop` of the same playback is answered `200`, and changes
  nothing. A viewer's server may send `stop` again when it got no answer.
- A `progress` that arrives after the playback ended, however it ended,
  gets `410` `playback-stopped`, and the playback stays ended. The viewer's
  server stops its player. The one exception is a download's late report
  with `at` (section 7).
- Calls of one playback may cross on the way: a `progress` sent before a
  `stop` may arrive after it. The owning server goes by its own state,
  never by the order the viewer's server sent things in.

### 6.2 Limits at once

The owning server must enforce the stream, transcode, download and scan
caps atomically: checking that a stream (and, for a transcode, a
transcode; for a download, a download and the day's count; for a scan,
the day's count) is free and counting the new playback is one step,
which no other `play` can come between. So two `play`s that arrive
together can never both take the last stream. A `play` that loses this race is refused as if it
had come later (`409` or `503`, section 2.6). A server may check the
caps earlier too, to refuse before doing any work, but the check that
counts is the atomic one. With shared limits (section 5), choosing the
pool and counting the playback in it is that same one step.

### 6.3 How a playback ends

A live playback ends, and its ticket stops working at once, when:

- the viewer's server sends `stop`;
- it has had neither a `progress` nor a media request for 2 minutes
  (expiry; a media response still open counts as use); never for a
  download (section 7);
- it is a download, 24 hours after its `play` was answered (section 7);
- it is a scan, 10 minutes after its `play` was answered (section 8);
- it has lasted longer than the owning server's longest playback, if it
  has one; for a download, its 24 hours stand instead;
- the pairing ends, the caller is no longer `paired`, or its key is
  suspended ([core section 12.6](core.md#126-revocation));
- the library holding the copy is no longer offered to that peer;
- it is a remux found to re-encode the video, and no transcode is left
  for it, in the pairing or in the server's budget (section 6.2);
- the owning server's operator stops it.

A playback of `file` that ends while its file is still being read may be
followed by a restart (section 2.7).

When it ends, the owning server stops the work it started for it (the
transcode, open media responses) and frees its stream, or its download.
A download or a scan stays counted in the day's count it took. An abandoned
playback (the viewer's server crashed, or lost its network) is cleaned up
by expiry: the owning server checks for expired playbacks at least once a
minute, so that one never holds a stream much longer than 2 minutes after
its last use, nor a download or a scan much longer than its lifetime.

The ticket must stop working at once in every case: the next media
request after the ending gets `404`, even when the ending was noticed
only then.

## 7. Downloads

A download lets a viewer keep a copy on a device and watch it offline.
Downloads are the owning server's choice, off by default
(`downloads` 0, section 5).

- **The request.** A `play` with `purpose` `download` and `delivery`
  `file` (the copy as it is) or `transcode` to a size the request names:
  a `transcode` object with `maxHeight`, `maxBitrateKbps` or both, and
  its other targets as section 2.7 says. Other deliveries of the list
  are refused (`400`). `startPositionMs` is ignored: a download is served from its
  beginning.
- **Limits.** It counts against `downloads` (at once) and
  `downloadsPerDay` (started in any 24 hours), never against `streams`
  or `transcodes`; a transcode takes one from the server's budget
  (section 6). Over `downloads` or `downloadsPerDay`: `409`
  `stream-limit-reached`. A transcode with the server's budget used:
  `503` `server-busy`, as for a `play`.
- **The ticket** lasts 24 hours from the answer, without the 2-minute
  idle rule: the viewer's server may read, stop reading and read again,
  by byte ranges, within those 24 hours. It sends `stop` when the
  download is complete or given up, which frees the download at once. A
  download not complete in 24 hours is restarted as section 2.7 says,
  with a new `play` of the same copy, only on the same `etag`.
- **The speed cap.** The owning server serves each download's reads at
  `downloadKbps` at most, measured as the bytes it sends for that
  playback over any 10 seconds, all its reads together, so that
  downloads never take all its bandwidth. It paces the bytes; it never
  refuses a read for its speed. A viewer's server must not count a slow
  download read as a failed fetch while bytes keep arriving; the time
  bounds of [media section 2.4](media.md#24-bounds) to the status line
  and headers still hold.
- **Late reports.** `progress` and `stop` carry `at` (sections 3 and 4)
  for what the viewer watched offline. For a download, the owning server
  accepts them for 30 days after its `play` (while now is at most the
  time of the `play` plus 30 days, that last second included), even once
  the playback has ended: it answers `200` and records the position, and the playback
  stays ended. A late report without `at`, or after those 30 days, is
  answered as for any ended playback: `410` to `progress`, `200` to
  `stop`.
- **Download shown only where allowed.** A viewer's server shows Download
  only for titles of a peer whose `limits` allow downloads (`downloads`
  above 0).

Test cases: `downloads` in
[vectors/v0/playback.json](../../vectors/v0/playback.json).

## 8. Scan access

Some media servers, without plugins, can only show real files: they list
a file by reading part of it. Scan access lets a viewer's server serve
them those reads, outside any playback of a viewer. It is off by default
(`scansPerDay` 0, section 5); the owning server's operator allows it per
pairing and per group, with a number of scans a day.

- **The request.** A `play` with `purpose` `scan` and `delivery` `file`;
  any other delivery of the list is refused (`400`). One scan is of one
  copy.
- **No stream used.** A scan counts against `scansPerDay` only (started
  in any 24 hours), never against `streams`, `transcodes` or `downloads`,
  and never converts anything. Over it: `409` `stream-limit-reached`.
  Scan `play`s count against the `play` limit of
  [core section 8.3](core.md#83-limits-and-overload) like any `play`.
- **The cap.** A scan's answer states `scanBytes` (section 2.3): the
  bytes its ticket may read in all (one scan, one copy), 4,194,304
  (4 MiB) to 1,073,741,824 (1 GiB). It is required with `purpose`
  `scan`; a caller that finds it missing or out of range counts on
  4 MiB. Only the owning server knows which pool counted the scan, so
  the cap is that pool's ([groups section 9](groups.md#9-sharing-and-limits)).
  The owning server serves the scan's reads up to the answer's
  `scanBytes`, counting every byte it sends under that ticket, a byte
  read twice counted twice; `scansPerDay` bounds the total. A read that
  would go past the cap is cut at the cap (a `206` ending there); any
  later read is refused (`416`), as
  [media section 4.2](media.md#42-scans) says.
- **Rate-limited.** The owning server serves a scan's reads at most 2 at
  once (more: `503`), and may pace them as it paces downloads.
- **The ticket** lasts 10 minutes from the answer, and the 2-minute idle
  rule holds. The viewer's server sends `stop` when its media server is
  done reading. It sends no `progress` for a scan.
- **What reads are for.** A viewer's server uses `scan` only for reads
  to list a file, and a `play` with `purpose` `play` for reads to watch
  it, like any other playback; how it tells them apart is its own. No
  whole-file analysis is made of a peer's copies. Scan access is the one
  case where a viewer's server lets its media server read a peer's file
  to learn what it is; everywhere else, media info comes from the
  catalog only.

Test cases: `scans` in
[vectors/v0/playback.json](../../vectors/v0/playback.json).

## 9. Security considerations

- **The ticket** is the only credential for the media, and is carried by
  `play`'s answer. That is why `play` needs a *trusted* or *named*
  certificate ([core section 3.2](core.md#32-https-certificates)). A
  download's ticket lives 24 hours: the viewer's server keeps it as it
  keeps its own credentials, and never passes it to a device.
- **Hostile answers.** Treat every field of a `play` answer as untrusted:
  check the ticket and IDs against their forms and numbers against their
  bounds. The answer carries no URLs: the viewer's server builds them
  ([media section 1](media.md#1-the-urls)), and never fetches an address
  a peer wrote.
- **The viewer's privacy.** A viewer's server never passes a peer's URLs
  to a browser, never sends anything that names a person, and says
  nothing of devices but the delivery it asks (sections 2.1 and 2.7).
- **Downloads leave the owning server.** A downloaded copy stays on a
  viewer's device; the owning server can stop a download, not take back
  what was read. That is why downloads are off by default.
- **Late reports can't be checked.** `at` is the viewer's server's word;
  the owning server bounds it (section 3) and trusts nothing else to it.
- **Scans are bounded,** by size (the scan answer's `scanBytes`, at most
  1 GiB), time and count (section 8), so that scan access can't be used to copy a file.

## 10. Open questions

No open point of the proposals of 2026-11-16 concerns this part alone.
The core's open questions ([core section 18](core.md#18-open-questions))
apply. Choices this draft made where the proposals were silent, all
confirmed by the owner on 2026-11-17 (they may still change as
implementations show):

1. **`versionId` in the request** (section 2.1). Proposal 2 says a
   viewer's server passes version IDs back in a later `play` of the same
   title, but the test draft's request has no member for it. This draft adds
   the optional `versionId`.

The second choice of that list, `startPositionMs` with several episodes
in one copy, moved to [video](video.md#8-open-questions).

Choices of the review round of 2026-11-16, confirmed on 2026-11-17:

2. **An `audioTrack` the copy doesn't have** is read as absent
   (section 2.6), not refused with `400`. (`subtitleTrack` left the
   request with proposal 17.)
3. ~~**No member to forbid transcoding**~~ Superseded by proposal 16:
   `delivery` `file` or `remux` forbids a transcode (section 2.6).
4. **The three states** of section 6.1, where started and active differ
   only in use. A server that needs no difference may keep one live state.
5. **Ending at once** when a pairing ends or a library is no longer
   offered (section 6.3): required as "the next media request fails";
   whether the transcode must also stop at the moment of the change is
   left to implementations.

**Choices this draft made, confirmed by the owner on 2026-11-17** (when the video rules were moved
out of this part on 2026-11-17):

6. ~~**The track object stays here**~~ Superseded by proposal 17: the
   answer carries no tracks; the catalog describes them.
7. **Section 2.4 keeps its number** for what the content parts add, so
   that sections 2.5 and 2.6 keep theirs.
8. **Members of another content part** in an answer are ignored
   (section 2.4); music already says so for a track's subtitles, skip
   markers and previews.
9. ~~**`subtitleTrack` is still named here**~~ Superseded by proposal 17:
   the viewer's server chooses; the choice tables left the protocol.

**Decided by the owner on 2026-11-18** (platform neutrality,
[core open questions 43 and 44](core.md#18-open-questions)):

10. **A copy may be several files** (section 1), played as one; the
    default audio is the file's, never a person's saved choice
    (section 2.6).

**Proposal 16** (adopted 2026-11-22): delivery, restarts, one playback
per viewer session, capabilities and transcode targets, downloads and
scan access. Choices this draft made where it was silent, to be
confirmed:

11. **`etag` in the answer** (section 2.3), required, so that a restart
    and "the answer wins" can compare it with the catalog's.
12. **`mediaType`** (`container` before proposal 17) names the remux's
    segment format, and **`audioRenditions`** asks for every audio track
    in HLS (section 2.1).
13. ~~**Steps of the new capabilities**~~ Superseded by proposal 17: no
    capabilities.
14. **A download's size** is named by `transcode.maxHeight` or
    `maxBitrateKbps` (section 7), not by a size in bytes.
15. **`at`** (section 3): Unix seconds, from 30 days before to 5 minutes
    after the owning server's clock; a download's late reports accepted
    for 30 days after its `play`.
16. **The speed cap** is per download, over any 10 seconds, paced, never
    refused (section 7).
17. **Scans** (section 8): `scansPerDay` up to 1,000,000, 0 meaning off;
    tickets of 10 minutes; 2 reads at once; ~~32 MiB per scan ticket~~
    `scanBytes` since proposal 17.
18. **One refusal type for every limit** (`stream-limit-reached`), for
    downloads and scans as for streams; `delivery-refused` is new.

**Proposal 17** (adopted 2026-11-22, [neutrality](../../proposals/0017-neutrality.md)):
no capabilities, the baseline without `delivery` and its picture,
remux to an `mp2t` or `mp4` media type, `copy` and `range` in a
transcode, `audioRenditions` in the answer, the copy and tracks chosen
by the viewer's server, answers without URLs (the ticket), opaque
playback IDs, no default bitrate ceiling, limits up to 1,000, no
`paused`, and `scanBytes` in the scan's answer. Choices this draft made
where it was silent, to be confirmed:

19. **`purpose` stays closed**: one not of its list is `400`; an unknown
    `delivery` token is `409` `delivery-refused`, like a remux's media
    type or a codec, since it names what to serve (section 2.1).
20. **The baseline is HLS** (section 2.7): without `delivery`, an answer
    with `protocol` `file` is refused, and the baseline's audio has at
    most 2 channels, as a transcode with `audioChannels` left out.
21. **The answer's `audioRenditions`**: absent means `false`; `true`
    with `protocol` `file` is dropped (section 2.3).
22. ~~**The title's own sound**~~ Superseded: the recommended defaults
    left the protocol for the notes.
23. ~~**An owning server without `scanBytes`**~~ Superseded: a scan's
    answer states its `scanBytes`; a caller counts on 4 MiB when it is
    missing (section 8).

Settled while msfp-go was written (2026-11-22), as the text reads:

24. **`burnSubtitle`'s form** is checked at step 4, with the track IDs,
    not at step 1 (section 2.2).
25. **Empty strings**: an empty `audioTrack` is malformed (`400`, step 4); an empty `title` in `progress` or `stop`
    is a title that is neither the request's nor of its queue (`400`).
26. ~~**`paused` on a `stop`**~~ Superseded: `paused` left `progress`
    with proposal 17, and is an unknown member everywhere.
27. ~~**An answer track's `channels`**~~ Superseded: the answer carries
    no tracks.

Proposal 17, written in on 2026-11-22: choices where its text gave no
form, to be confirmed:

28. **`versionId` in the answer** is required, as proposal 17's table
    gives it without "optional"; one of another form refuses the answer
    (section 2.3).
29. **A scan answer's `scanBytes`** that isn't an integer counts as
    missing: 4 MiB (section 2.3).
30. **A `range` that is a token not of its list** gets `409`
    `delivery-refused`, as an unknown `delivery` does, since it names
    what to serve (section 2.7).
31. **A transcode of a track with audio `copy`** still counts as a
    transcode: the rule that frees one is for video `copy` (section 2.6).
