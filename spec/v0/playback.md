# MSFP version 0: the playback part, version 0

**Draft.**

The `playback` part lets a viewer's server start, keep and stop a
playback of a title a peer holds, within limits the owning server sets:
to watch it, to download it, or to let a media server read the start of
its file to list it. Which titles it plays, and what their playbacks
carry besides, is said by the content parts,
[video](video.md#5-playing-a-film-or-an-episode) and
[music](music.md#5-playing-a-track) (section 2.4). It needs the
[core](core.md), [catalog](catalog.md) (it plays title keys from
`titles`) and [media](media.md) (its answers point at media URLs). If either is off between two servers, `playback` is off
too ([core section 10.3](core.md#103-choosing)). Its calls follow
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
  each copy before any playback: its version ID, length, container,
  exact size, `etag` and tracks (the video part's copies). A copy's
  `etag` changes whenever its bytes change.
- **Playback**: one stream the owning server runs for the viewer's
  server. It has a playback ID and a ticket.
- **Ticket**: an opaque string in the stream URLs. It is the only
  credential for fetching the media ([media](media.md)).
- **Delivery**: what the viewer's server asks to be served: the file as
  it is, a remux, or a transcode (section 2.7). Absent, the owning server
  decides.
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
| `delivery` | string | no | `file`, `remux` or `transcode` (section 2.7); absent: the owning server decides from the capabilities |
| `container` | string | with `delivery` `remux` only, and then required | the container to remux into, one value of `containers` (below) |
| `transcode` | transcode object | no; with `delivery` `transcode` only | the targets of the transcode (section 2.7) |
| `audioRenditions` | boolean | no | `true`: when served as HLS, every audio track of the copy as a rendition ([media](media.md), several audio tracks in HLS), so that switching tracks needs no new playback |
| `capabilities` | capabilities object | no | what the viewer's device plays, coarsened (below) |
| `startPositionMs` | integer | no | where to start, in milliseconds; 0 to 8,640,000,000 (100 days) |
| `maxBitrateKbps` | integer | no | the viewer's bitrate cap, kbit/s; 0 to 10,000,000; 0 means none |
| `audioLanguage` | string | no | the viewer's audio language, a tag; at most 12 characters of `A–Z a–z 0–9 -` |
| `audioTrack` | string | no | an audio track ID from a former answer or from the copy in the catalog, `a` and a number from 0 to 999 without leading zeros (`a01` is invalid) |
| `versionId` | string | no | a version ID: a copy's `id` in the catalog, or one from a former answer of the same title (section 2.3) |

The content parts may add members ([video section 5.1](video.md#51-the-request));
[music](music.md#5-playing-a-track) adds the next tracks of a queue, at
most 5, which the owning server prepares under the same playback, and
which count once against `streams`.

Capabilities (all fields optional; unknown values are ignored; the
content parts may add more, as [video](video.md#51-the-request) does for
video codecs, HDR and Dolby Vision, height and subtitles):

| Field | Type | Values |
|-------|------|--------|
| `containers` | array of string | `mp4`, `m4v`, `mov`, `webm`, `mkv`, `ts`, `mp3`, `aac`, `m4a`, `flac`, `ogg`, `opus`, `wav`, `avi`, `m2ts`, `mpegts`, `wmv`, `flv`, `3gp` |
| `audioCodecs` | array of string | `aac`, `mp3`, `opus`, `vorbis`, `flac`, `ac3`, `eac3`, `alac`, `pcm_s16le`, `dts`, `truehd`, `mp2`, `wmav2`, `pcm_s24le`, `pcm_bluray` |
| `audioProfiles` | array of string | `atmos`, `dtsx`, `dts-hd-ma`, `dts-hd-hra`, `dts-es`, `he-aac`, `lc` |
| `maxAudioChannels` | integer | 0 to 64; 0 means not said |
| `maxAudioSampleRate` | integer | hertz, 0 to 768,000; 0 means not said |
| `maxAudioBitDepth` | integer | bits, 0 to 64; 0 means not said |
| `maxBitrateKbps` | integer | 0 to 10,000,000; 0 means not said |

These lists, with the content parts' own, are the names a copy's
`container` and codecs take in the catalog (the names list of the video
part's copies); a copy's name outside them is `other`, which the owning
server converts. An audio profile listed means the device plays that
profile of its codec (`atmos` in `eac3` or `truehd`, `dts-hd-ma` in
`dts`); a profile not listed means the device plays only the codec's
core, when the codec has one.

**Coarsening.** So that the owning server can't tell one device from
another, the viewer's server must coarsen what it sends: only the values
listed above (and by the content parts), lowercase, each once, sorted;
the content parts' own steps ([video](video.md#51-the-request):
`maxHeight`); `maxAudioChannels` stepped down to 2, 6 or 8 (below 2
becomes 2); `maxAudioSampleRate` stepped down to 44,100, 48,000, 96,000
or 192,000 (below 44,100 becomes 44,100); `maxAudioBitDepth` stepped
down to 16, 24 or 32 (below 16 becomes 16); `capabilities.maxBitrateKbps`
rounded down to a whole 1,000, and below 1,000 left out (0); the
request's `maxBitrateKbps` rounded down to a whole 1,000 from 1,000 up,
and kept as it is below.

**Arrays in capabilities.** The owning server reads each array of
`capabilities` this way:

- An array whose entries aren't all strings, or a member that isn't an
  array where the table says one, refuses the request (`400`).
- An entry not in the table, in another case included, is ignored.
  Duplicates count once. Order doesn't matter.
- An empty array, or one left with nothing once unknown entries are
  ignored, means the same as the member left out: nothing said.
- What the owning server serves when nothing is said is up to the
  content part of the title (section 2.4): H.264 and AAC in HLS for a
  film or an episode ([video section 5.1](video.md#51-the-request)), AAC
  in HLS, audio only, for a track ([music section 5](music.md#5-playing-a-track)).

The capabilities matter only when `delivery` is absent, or for the
targets a `transcode` object leaves out: otherwise the owning server
serves what `delivery` names, whatever they say (section 2.7).

Test cases: `requests` in
[vectors/v0/playback.json](../../vectors/v0/playback.json) (members,
combinations of `purpose` and `delivery`, coarsening).

**Overfin.** Since 2026-11-16, Overfin compares each entry a peer sends
exactly, so `H264` is an unknown entry (before, it lowercased them).
Its own apps' API, which is not MSFP, still lowercases.

### 2.2 What the owning server does

1. It refuses the request (`400`) when `title` isn't the key of a kind
   `play` takes in a content part on between the two servers
   (section 2.4), or a number or tag is out of range: a negative
   number, or one above its range, among them `capabilities`'
   `maxAudioChannels`, `maxAudioSampleRate`, `maxAudioBitDepth` and
   video's `maxHeight`. It refuses (`400`) too a `purpose`, `delivery` or
   `container` not of its list, a `transcode` object that breaks
   section 2.7, and a combination of `purpose` and `delivery` that
   section 2.7 doesn't allow.
2. It refuses (`409` `stream-limit-reached`) when the pairing's limit for
   the purpose is 0: `streams` for `play`, `downloads` for `download`,
   `scansPerDay` for `scan` (section 5).
3. It looks for its copies of that exact key in the libraries it offers
   the caller. None: `404`.
4. It picks the copy `versionId` names, if it is one of them; else the
   copy whose audio has `audioLanguage` (same tag first, then same base
   language); else the first. Then it refuses (`400`) a malformed
   `audioTrack` or `subtitleTrack` ([video section 5.1](video.md#51-the-request)).
   A track ID's number has no leading zeros, like other numbers of MSFP:
   `a01` and `s007` are malformed.
5. It refuses (`409` `stream-limit-reached`) when the purpose's limits
   are used: for `play`, the pairing's streams all in use; for
   `download`, its downloads all in use, or `downloadsPerDay` reached;
   for `scan`, `scansPerDay` reached.
6. The bitrate cap is the lowest non-zero of: `maxBitrateKbps`,
   `capabilities.maxBitrateKbps`, `transcode.maxBitrateKbps`, the owning
   server's own cap, and the pairing's `bitrateKbps`. With none, 120,000
   kbit/s. It applies to `play` only; a download's speed is capped by
   `downloadKbps` (section 7), and a scan's reads by section 8.
7. With `delivery`, it does what is asked, or refuses (`409`
   `delivery-refused`) for reasons of its own: bandwidth (the copy's
   bitrate above the cap of step 6, for `file` or `remux`), its policy,
   or a delivery it can't make of that copy. It never serves another
   delivery than the one asked. Without `delivery`, it decides from the
   capabilities (section 2.6).
8. It starts the playback within the pairing's limits and the server's
   transcode budget (section 6). Over them: `409` `stream-limit-reached`,
   or `503` `server-busy`. When it has no way to play the copy at all:
   `409` `not-playable`.
9. It answers `200`, signed, with the playback.

A copy that holds several episodes starts where
[video section 5.2](video.md#52-several-episodes-in-one-copy) says.

### 2.3 The answer

Answer `200`: the playback object. The viewer's server checks and cleans
every field, as listed.

| Field | Type | Content | What the viewer's server requires or does |
|-------|------|---------|-------------------------------------------|
| `id` | string | the playback ID | must match `^play_[a-z2-7]{26}$` |
| `method` | string | `direct`, `remux` or `transcode` | anything else: refused; with `delivery`, must be the one asked (`direct` for `file`) |
| `stream` | object | `url`, `protocol`, `mimeType` (optional) | [media section 1](media.md#1-the-urls) |
| `etag` | string | the `etag` of the copy playing, as the catalog gives it | must be 1 to 64 characters of `A–Z a–z 0–9 - _` |
| `positionMs` | integer | where the playback starts, in the copy | |
| `durationMs` | integer, optional | the copy's length | outside 0 to 100 days: set to 0 |
| `versions` | array, optional | `{id, name}` of the title's versions | IDs as below; names as a copy's `name` (the video part's copies): made plain text, at most 32 characters |
| `versionId` | string, optional | the version playing | IDs as below |
| `audioTracks` | array of track, optional | the audio tracks | IDs as below |
| `audioTrack` | string, optional | the one playing, `a<n>` | |
| `maxBitrateKbps` | integer, optional | the cap in effect | |
| `info` | object, optional | how it's served: `reasons`, `container`, `bitrateKbps`, `video`, `audio` | text made plain, short |

The content parts may add members: [video section 5.3](video.md#53-the-answer)
adds subtitles, forced subtitles and seek-bar previews.

**The answer wins.** When the answer and the catalog disagree about the
copy playing (another `etag`, other tracks, another length: the copy
changed since the catalog was read), the viewer's server goes by the
answer for this playback, and reads the title again from `titles`.

Track:

| Field | Type | Content |
|-------|------|---------|
| `id` | string | `a<n>` for audio, `s<n>` for subtitles ([video section 5.3](video.md#53-the-answer)); *n* has no leading zeros |
| `language` | string, optional | a language tag |
| `channels` | integer, optional | audio channels |
| `role` | string, optional | `commentary` or `description` |
| `default`, `forced`, `hearingImpaired` | boolean, optional | flags |

A track carries no label: a viewer's server names languages in its own
people's words, from `language`. The content parts may add members to a
track: the video part's copies describe a track in full (codec, profile,
title and so on), with the same IDs, and a track of an answer may carry
those members, cleaned as the catalog's are.

**IDs.** IDs of tracks and versions must match `^[A-Za-z0-9_-]{1,64}$`
(or be `off`). The viewer's server drops tracks with another ID, and
clears other IDs. It drops the playback object's members `title` and
`next` whatever they hold.

**Version IDs** (`versionId`, `versions[].id`) are the `id`s of the
title's copies in the catalog. They follow the rule of library
IDs ([catalog section 2.1](catalog.md#21-library-ids)): opaque, chosen
by the owning server, not derived from its media server's IDs without a
key. They need to be stable only for the life of a title's copies, since
a viewer's server passes them back only in a later `play` of the same
title. Track IDs (`a<n>`, `s<n>`) are positions, not identifiers, and are
not concerned. *n* is the owning server's own number for the track in
that copy, the same at every `play` of it and in the catalog: its media
server's stream index, for example, so that a film whose video is stream
0 has its first audio track at `a1`. A viewer's server reads nothing into
*n*: it only sends back an ID a former answer or the catalog gave it.

**Roles are best effort.** `role` is absent when the owning server
doesn't know it; it may guess from a track's title. A viewer's server
treats an absent role as the title's own sound, so an unflagged
commentary may be chosen by language (section 2.6).

When the answer breaks a "must" above, the viewer's server sends `stop`
for that `id` and gives up.

To change the audio track, the version or the quality, the viewer's
server stops the playback and starts a new one with `play`. It waits up
to 5 seconds for the `stop` to be answered first, so the old playback
doesn't count against the limit. Switching the audio track needs no new
playback when the copy is delivered as `file` (it holds every track) or
served with `audioRenditions`.

Test cases: `answers` in
[vectors/v0/playback.json](../../vectors/v0/playback.json).

### 2.4 What the content parts add

Each content part says which of its kinds `play` takes, what the owning
server serves when the capabilities say nothing, and which members it
adds to the request and the answer:

| Content part | `play` takes | Nothing said | Adds |
|--------------|--------------|--------------|------|
| `video` | films, episodes and extras | H.264 and AAC in HLS | subtitles and forced subtitles, seek-bar previews, video capabilities, a subtitle burned in as a transcode target ([video section 5](video.md#5-playing-a-film-or-an-episode)) |
| `music` | tracks | AAC in HLS, audio only | the next tracks of a queue; audio limits in `limits` ([music section 5](music.md#5-playing-a-track)) |

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
| 409 | `delivery-refused` | the owning server won't serve the delivery asked (section 2.2, step 7) | nothing yet: the viewer's server asks again with another delivery, or without `delivery`; when no delivery it can use is left, not playable (for a download: download not available) |
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

The owning server chooses in this order: the copy, then the audio, then
the subtitles ([video section 5.4](video.md#54-choosing-the-subtitles)),
then how to serve it. A track ID (`a<n>`, `s<n>`) names a
track of the copy chosen. Section 2.2 already refused a malformed ID
(`400`). "The default audio" is the track the copy's file marks as default, else
its first audio track; never a choice a person of the owning server
saved. Language matching takes the same tag first
(`fr-CA` for `fr-CA`), then the same base language (`fr` for `fr-CA`),
and for audio only the title's own sound, never a commentary or a
description.

**The copy.**

| `versionId` | `audioLanguage` | Copy chosen |
|-------------|-----------------|-------------|
| one of the title's copies | any | that copy |
| absent, or no copy has it | a copy's audio matches | the first copy whose audio matches |
| absent, or no copy has it | absent, or no copy matches | the first copy |
| no copies offered to the peer | any | refused: `404` |

**The audio.**

| `audioTrack` | `audioLanguage` | Audio played |
|--------------|-----------------|--------------|
| a track of the copy | any (ignored) | that track |
| not a track of the copy | any | as if `audioTrack` were absent |
| absent | a track matches | that track |
| absent | absent, or no track matches | the default audio |

With `delivery` `file`, the copy comes with all its tracks, and the
viewer's player selects the audio itself; the answer's `audioTrack` is
the one chosen as above, which the player starts with.

**How it's served.** With `delivery`, as asked (section 2.7). Without
it, `method` follows from the copy, the audio track and the
capabilities, the cheapest first: `direct`, then `remux`, then
`transcode`.

| What the copy and track need | Limits (section 6) | Result |
|------------------------------|--------------------|--------|
| `direct` or `remux` | a stream free | `200` |
| `transcode` | a stream free, a transcode free for the pairing and in the server's budget | `200` |
| `transcode` | the pairing's `transcodes` all used or 0, or the server's budget used | `503` `server-busy` |
| anything | the pairing's `streams` all used, or 0 | `409` `stream-limit-reached` |
| no way to play the copy | any | `409` `not-playable` |

A `play` with `delivery` `file` or `remux` never transcodes: it is how a
viewer's server forbids a transcode. One with `delivery` `transcode`
needs a transcode free, as the second and third rows say.

The owning server doesn't fall back to another copy, track or quality to
avoid a transcode, and doesn't refuse for lack of a language. The answer
always says what plays (`versionId`, `audioTrack`, `subtitleTrack`), and
the viewer's server shows that, not what it asked for.

**Overfin.** Since 2026-11-16, an `audioTrack` or `subtitleTrack` that
isn't a track of the copy counts as absent, as above (before, the audio
track was passed to its media server as is, and the subtitle track
echoed). The rest is as above.

### 2.7 Delivery

**The deliveries.**

| `delivery` | What is served | `method` in the answer | How it is fetched |
|------------|----------------|------------------------|-------------------|
| `file` | the copy as it is, all its tracks | `direct` | by byte ranges ([media](media.md), files by byte ranges) |
| `remux` | the copy's streams, unchanged, in the `container` the request names | `remux` | as [media](media.md) says for a remux (complete HLS) |
| `transcode` | the copy converted to the targets of `transcode` | `transcode` | as [media](media.md) says for a conversion (complete HLS) |
| absent | what the owning server decides from the capabilities (section 2.6) | any | as the answer's `stream` says |

A copy of several files is delivered as `file` only as
[media](media.md) allows for a copy in parts; an owning server that
can't refuses with `delivery-refused`. The size the viewer's server
presents for a `file` is the copy's exact `size` in the catalog (the
video part's copies).

**Purpose and delivery.** Combinations not in this table are refused
(`400`):

| `purpose` | `delivery` allowed |
|-----------|--------------------|
| `play` (or absent) | `file`, `remux`, `transcode`, absent |
| `download` | `file`; `transcode` with `transcode.maxHeight` or `transcode.maxBitrateKbps` (section 7) |
| `scan` | `file` only (section 8) |

**Transcode targets.** The `transcode` object, with `delivery`
`transcode` only (with any other `delivery`, or none, it is refused,
`400`). All members are optional; one left out is the owning server's
choice, from the capabilities as when `delivery` is absent. Values come
from the same coarse lists as the capabilities; any other value is
refused (`400`).

| Field | Type | Values | The owning server serves |
|-------|------|--------|--------------------------|
| `videoCodec` | string | one value of video's `videoCodecs` ([video section 5.1](video.md#51-the-request)) | the picture in that codec; ignored for a title without a picture (a track) |
| `maxHeight` | integer | 480, 720, 1080, 1440, 2160 or 4320 | a picture at most that tall, never scaled up; ignored for a title without a picture |
| `maxBitrateKbps` | integer | 1,000 to 10,000,000, a whole 1,000 | at most that overall bitrate (with the cap of section 2.2, step 6, the lower wins) |
| `audioCodec` | string | one value of `audioCodecs` | the audio in that codec |
| `audioChannels` | integer | 2, 6 or 8 | at most that many channels, never upmixed |

The video part may add targets ([video](video.md#51-the-request): one
subtitle track burned into the picture). The owning server converts to
exactly the targets given, or refuses with `delivery-refused`; it never
serves something close.

**A playback may start on the first read.** A viewer's server may present
a peer's copy to its media server and clients as a plain file, at an
address of its own, before anyone plays it, with the length, container,
tracks and size the catalog gives. It calls `play` when its media server
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

Test cases: `deliveries` and `restarts` in
[vectors/v0/playback.json](../../vectors/v0/playback.json).

## 3. progress

`progress`, under the peer's base path. The viewer's server tells the
owning server that a playback is still watched, and where.

Request:

| Field | Type | Required | Content |
|-------|------|----------|---------|
| `id` | string | yes | the playback ID from `play`, `^play_[a-z2-7]{26}$` |
| `positionMs` | integer | no | where the viewer is; 0 to 8,640,000,000 |
| `paused` | boolean | no | whether the viewer paused |
| `at` | integer | no | when the viewer was there, Unix seconds; at most 30 days before the owning server's clock and at most 5 minutes after it; absent means now |

Answer `200`: `{}`.

Refusals: `400` for a malformed body, an `at` out of its bounds included;
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
changes nothing.

Test cases: `reports` in
[vectors/v0/playback.json](../../vectors/v0/playback.json) (`at` bounds).

## 4. stop

`stop`, under the peer's base path. The viewer's server tells the owning
server a playback is over.

Request:

| Field | Type | Required | Content |
|-------|------|----------|---------|
| `id` | string | yes | the playback ID, `^play_[a-z2-7]{26}$` |
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
| `streams` | integer | 0 to 50 | playbacks of purpose `play` at the same time |
| `transcodes` | integer | 0 to `streams` | of those, how many may be transcoded |
| `bitrateKbps` | integer | 0 to 1,000,000 | a bitrate cap in kbit/s; 0 means none |
| `downloads` | integer | 0 to 50 | downloads at the same time (section 7); 0: no downloads |
| `downloadsPerDay` | integer | 0 to 10,000 | downloads started in any 24 hours; 0 means no daily cap besides `downloads` |
| `downloadKbps` | integer | 0 to 1,000,000 | the speed cap of each download, kbit/s (section 7); 0 means none |
| `scansPerDay` | integer | 0 to 1,000,000 | scans started in any 24 hours (section 8); 0: no scan access |

Absent, `downloads`, `downloadsPerDay`, `downloadKbps` and `scansPerDay`
are 0: no downloads, no scan access. A content part may add members:
[music section 5.1](music.md#51-limits) adds `audioStreams` and
`audioTranscodes`.

The caller uses the limits only to avoid offering its people what the
owning server would refuse: it shows Download (section 7) only for a
peer whose `downloads` is above 0, and asks for scans (section 8) only
of a peer whose `scansPerDay` is above 0. The owning server enforces them
itself (section 6). A caller ignores limits out of range. A server that doesn't
enforce limits sends none.

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
  `transcodes`. A server whose media server has no such budget
  (Jellyfin starts as many transcodes as it is asked) makes one, an
  operator setting counting its own people's transcodes and its peers'
  together. A remux found to re-encode the video counts as a
  transcode from then on, when the owning server can tell (its media
  server names the encoder of the job, for example); one that can't asks
  for a remux only when its media server copies the video. The bitrate is the lowest cap of section 2.2,
  step 6.
- **Defaults.** A reasonable start for a new pairing is 2 streams, 1
  transcode, no bitrate cap, no downloads and no scan access; the
  operator changes them, per pairing and per group. An operator who
  allows downloads may start from 1 at once, 10 a day and 20,000 kbit/s;
  one who allows scans, from 1,000 a day.
- **Media.** Only through the ticket, only from a client that doesn't
  look like a browser, only while the playback is live
  ([media section 4](media.md#4-how-long-a-ticket-works)). A download's
  reads at `downloadKbps` at most (section 7); a scan's reads within its
  cap (section 8).
- **What a peer never learns:**
  - nothing about the viewer: no account, profile, name, email, address,
    IP, browser or device. Nothing in `play`, `progress` or `stop` names a
    person.
  - not the viewer's exact device: capabilities, transcode targets and
    bitrates are coarsened (sections 2.1 and 2.7).
  - of the owning server: never files, paths, or its media server's item
    IDs, not even inside version IDs (section 2.3).

  What a peer does learn: the title, the purpose, the delivery, the start
  position, the language tags wanted, the coarsened capabilities and
  transcode targets, and progress over time (with `at`, the times of
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
  has one (Overfin: 24 hours); for a download, its 24 hours stand
  instead;
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

**Overfin.** Overfin's check and count are one step under one lock
(atomic), and it checks the pairing's streams once more before asking its
media server, to refuse early. It looks for expired playbacks every 30
seconds. A pairing that ends (by either side), a peer found no longer
paired, or a change of what it offers a peer ends that peer's playbacks
at the moment of the change, open media responses included; each media
request checks again. After a playback ends, it remembers why for an
hour; `progress` gets `410` either way.

## 7. Downloads

A download lets a viewer keep a copy on a device and watch it offline.
Downloads are the owning server's choice, off by default
(`downloads` 0, section 5).

- **The request.** A `play` with `purpose` `download` and `delivery`
  `file` (the copy as it is) or `transcode` to a size the request names:
  a `transcode` object with `maxHeight`, `maxBitrateKbps` or both, and
  its other targets as section 2.7 says. Other deliveries are refused
  (`400`). `startPositionMs` is ignored: a download is served from its
  beginning.
- **Limits.** It counts against `downloads` (at once) and
  `downloadsPerDay` (started in any 24 hours), never against `streams`
  or `transcodes`; a transcode takes one from the server's budget
  (section 6). Over them: `409` `stream-limit-reached`.
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
  bounds of [media](media.md) to the status line and headers still hold.
- **Late reports.** `progress` and `stop` carry `at` (sections 3 and 4)
  for what the viewer watched offline. For a download, the owning server
  accepts them for 30 days after its `play`, even once the playback has
  ended: it answers `200` and records the position, and the playback
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
a file by reading its start (Plex, through a file system, reads 1 to
4 MB of a file to list it). Scan access lets a viewer's server serve them
those reads, outside any playback of a viewer. It is off by default
(`scansPerDay` 0, section 5); the owning server's operator allows it per
pairing and per group, with a number of scans a day.

- **The request.** A `play` with `purpose` `scan` and `delivery` `file`;
  any other delivery is refused (`400`). One scan is of one copy.
- **No stream used.** A scan counts against `scansPerDay` only (started
  in any 24 hours), never against `streams`, `transcodes` or `downloads`,
  and never converts anything. Over it: `409` `stream-limit-reached`.
  Scan `play`s count against the `play` limit of
  [core section 8.3](core.md#83-limits-and-overload) like any `play`.
- **The cap.** The owning server serves a scan's reads up to 32 MiB
  (33,554,432 bytes) of its copy in all, counting every byte it sends
  under that ticket, a byte read twice counted twice. A read that would
  go past the cap is served up to it and then ends; any later read is
  refused (`416`).
- **Rate-limited.** The owning server serves a scan's reads at most 2 at
  once (more: `503`), and may pace them as it paces downloads.
- **The ticket** lasts 10 minutes from the answer, and the 2-minute idle
  rule holds. The viewer's server sends `stop` when its media server is
  done reading. It sends no `progress` for a scan.
- **What a media server reading this way needs.** A viewer's server uses
  `scan` only for its media server's reads to list a file, and a `play`
  with `purpose` `play` for reads to watch it, like any other playback;
  how it tells them apart is its own. A media server reading a peer's
  copies this way must not make whole-file analyses of them: for Plex,
  "extensive media analysis" must be off for such a library (tested: it
  reads every file whole, and when a read is refused, overwrites the
  file's length with what it read). Scan access is the one case where a
  viewer's server lets its media server read a peer's file to learn what
  it is; everywhere else, media info comes from the catalog only.

Test cases: `scans` in
[vectors/v0/playback.json](../../vectors/v0/playback.json).

## 9. Security considerations

- **The ticket** is the only credential for the media, and is carried by
  `play`'s answer. That is why `play` needs a *trusted* or *named*
  certificate ([core section 3.2](core.md#32-https-certificates)). A
  download's ticket lives 24 hours: the viewer's server keeps it as it
  keeps its own credentials, and never passes it to a device.
- **Hostile answers.** Treat every field of a `play` answer as untrusted:
  check IDs, URLs against the owning server's stream root, numbers
  against their bounds, and clean text.
- **The viewer's privacy.** A viewer's server never passes a peer's URLs
  to a browser, never sends anything that names a person, and coarsens
  what it says about devices (sections 2.1 and 2.7).
- **Downloads leave the owning server.** A downloaded copy stays on a
  viewer's device; the owning server can stop a download, not take back
  what was read. That is why downloads are off by default.
- **Late reports can't be checked.** `at` is the viewer's server's word;
  the owning server bounds it (section 3) and trusts nothing else to it.
- **Scans are bounded,** by size, time and count (section 8), so that
  scan access can't be used to copy a file.

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

2. **An `audioTrack` or `subtitleTrack` the copy doesn't have** is read
   as absent (section 2.6), not refused with `400`.
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

6. **The track object stays here** (section 2.3), with its `forced` and
   `hearingImpaired` flags and the `s<n>` IDs; only the subtitle members
   `pictures` and `url` moved to [video](video.md#53-the-answer).
7. **Section 2.4 keeps its number** for what the content parts add, so
   that sections 2.5 and 2.6 keep theirs.
8. **Members of another content part** in an answer are ignored
   (section 2.4); music already says so for a track's subtitles, skip
   markers and previews.
9. **`subtitleTrack` is still named here** (sections 2.2 and 2.6): the
   order of the choice and the tables of the copy and the audio stay
   here; the subtitles' table moved to video.

**Decided by the owner on 2026-11-18** (platform neutrality,
[core open questions 43 and 44](core.md#18-open-questions)):

10. **A copy may be several files** (section 1), played as one; the
    default audio is the file's, never a person's saved choice
    (section 2.6); `role` is best effort (section 2.3).

**Proposal 16** (adopted 2026-11-22): delivery, restarts, one playback
per viewer session, capabilities and transcode targets, downloads and
scan access. Choices this draft made where it was silent, to be
confirmed:

11. **`etag` in the answer** (section 2.3), required, so that a restart
    and "the answer wins" can compare it with the catalog's.
12. **`container`** names the remux's container, and **`audioRenditions`**
    asks for every audio track in HLS (section 2.1).
13. **Steps of the new capabilities** (section 2.1): sample rates 44,100,
    48,000, 96,000 and 192,000; bit depths 16, 24 and 32.
14. **A download's size** is named by `transcode.maxHeight` or
    `maxBitrateKbps` (section 7), not by a size in bytes.
15. **`at`** (section 3): Unix seconds, from 30 days before to 5 minutes
    after the owning server's clock; a download's late reports accepted
    for 30 days after its `play`.
16. **The speed cap** is per download, over any 10 seconds, paced, never
    refused (section 7).
17. **Scans** (section 8): `scansPerDay` up to 1,000,000, 0 meaning off;
    tickets of 10 minutes; 2 reads at once; 32 MB read as 32 MiB.
18. **One refusal type for every limit** (`stream-limit-reached`), for
    downloads and scans as for streams; `delivery-refused` is new.
