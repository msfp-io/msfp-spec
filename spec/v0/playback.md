# MSFP version 0: the playback part, version 0

**Draft.**

The `playback` part lets a viewer's server start, keep and stop a
playback of a title a peer holds, within limits the owning server sets.
Which titles it plays, and what their playbacks carry besides, is said by
the content parts, [video](video.md#5-playing-a-film-or-an-episode) and
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
7. [Security considerations](#7-security-considerations)
8. [Open questions](#8-open-questions)

## 1. Terms

- **Owning server**: the server that holds a title's copy and streams it.
- **Viewer's server**: the server whose signed-in person watches a title
  the owning server holds. The person's browser only ever talks to the
  viewer's server.
- **Copy**: one version of a title on the owning server: one file, or
  several files played one after the other as one (parts). A title may
  have several copies (versions). Positions, `durationMs` and the
  content parts' markers are in the whole copy.
- **Playback**: one stream the owning server runs for the viewer's
  server. It has a playback ID and a ticket.
- **Ticket**: an opaque string in the stream URLs. It is the only
  credential for fetching the media ([media](media.md)).

## 2. play

`play`, under the peer's base path. The viewer's server asks the owning
server to start a playback of a title for one viewer.

### 2.1 The request

| Field | Type | Required | Content |
|-------|------|----------|---------|
| `title` | string | yes | the key of a title `play` takes (section 2.4), as the owning server sent it in `titles` |
| `capabilities` | capabilities object | no | what the viewer's device plays, coarsened (below) |
| `startPositionMs` | integer | no | where to start, in milliseconds; 0 to 8,640,000,000 (100 days) |
| `maxBitrateKbps` | integer | no | the viewer's bitrate cap, kbit/s; 0 to 10,000,000; 0 means none |
| `audioLanguage` | string | no | the viewer's audio language, a tag; at most 12 characters of `A–Z a–z 0–9 -` |
| `audioTrack` | string | no | an audio track ID from a former answer, `a` and a number from 0 to 999 without leading zeros (`a01` is invalid) |
| `versionId` | string | no | a version ID from a former answer of the same title (section 2.3) |

The content parts may add members ([video section 5.1](video.md#51-the-request)).

Capabilities (all fields optional; unknown values are ignored; the
content parts may add more, as [video](video.md#51-the-request) does for
video codecs, HDR, height and subtitles):

| Field | Type | Values |
|-------|------|--------|
| `containers` | array of string | `mp4`, `m4v`, `mov`, `webm`, `mkv`, `ts`, `mp3`, `aac`, `m4a`, `flac`, `ogg`, `opus`, `wav` |
| `audioCodecs` | array of string | `aac`, `mp3`, `opus`, `vorbis`, `flac`, `ac3`, `eac3`, `alac`, `pcm_s16le` |
| `maxAudioChannels` | integer | 0 to 64; 0 means not said |
| `maxBitrateKbps` | integer | 0 to 10,000,000; 0 means not said |

**Coarsening.** So that the owning server can't tell one device from
another, the viewer's server must coarsen what it sends: only the values
listed above (and by the content parts), lowercase, each once, sorted;
the content parts' own steps ([video](video.md#51-the-request):
`maxHeight`); `maxAudioChannels` stepped down to 2, 6 or 8 (below 2
becomes 2); `capabilities.maxBitrateKbps` rounded down to a whole 1,000, and below
1,000 left out (0); the request's `maxBitrateKbps` rounded down to a whole
1,000 from 1,000 up, and kept as it is below.

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

**Overfin.** Since 2026-11-16, Overfin compares each entry a peer sends
exactly, so `H264` is an unknown entry (before, it lowercased them).
Its own apps' API, which is not MSFP, still lowercases.

### 2.2 What the owning server does

1. It refuses the request (`400`) when `title` isn't the key of a kind
   `play` takes in a content part on between the two servers
   (section 2.4), or a number or tag is out of range: a negative
   number, or one above its range, among them `capabilities`'
   `maxAudioChannels` and video's `maxHeight`.
2. It refuses (`409` `stream-limit-reached`) when the pairing's `streams`
   limit is 0.
3. It looks for its copies of that exact key in the libraries it offers
   the caller. None: `404`.
4. It picks the copy `versionId` names, if it is one of them; else the
   copy whose audio has `audioLanguage` (same tag first, then same base
   language); else the first. Then it refuses (`400`) a malformed
   `audioTrack` or `subtitleTrack` ([video section 5.1](video.md#51-the-request)).
   A track ID's number has no leading zeros, like other numbers of MSFP:
   `a01` and `s007` are malformed.
5. It refuses (`409` `stream-limit-reached`) when the pairing's streams
   are all in use.
6. The bitrate cap is the lowest non-zero of: `maxBitrateKbps`,
   `capabilities.maxBitrateKbps`, the owning server's own cap, and the
   pairing's `bitrateKbps`. With none, 120,000 kbit/s.
7. It starts the playback within the pairing's limits and the server's
   transcode budget (section 6). Over them: `409` `stream-limit-reached`,
   or `503` `server-busy`. When it has no way to play the copy: `409`
   `not-playable`.
8. It answers `200`, signed, with the playback.

A copy that holds several episodes starts where
[video section 5.2](video.md#52-several-episodes-in-one-copy) says.

### 2.3 The answer

Answer `200`: the playback object. The viewer's server checks and cleans
every field, as listed.

| Field | Type | Content | What the viewer's server requires or does |
|-------|------|---------|-------------------------------------------|
| `id` | string | the playback ID | must match `^play_[a-z2-7]{26}$` |
| `method` | string | `direct`, `remux` or `transcode` | anything else: refused |
| `stream` | object | `url`, `protocol`, `mimeType` (optional) | [media section 1](media.md#1-the-urls) |
| `positionMs` | integer | where the playback starts, in the copy | |
| `durationMs` | integer, optional | the copy's length | outside 0 to 100 days: set to 0 |
| `versions` | array, optional | `{id, name}` of the title's versions | IDs as below; names made plain text, 64 characters |
| `versionId` | string, optional | the version playing | IDs as below |
| `audioTracks` | array of track, optional | the audio tracks | IDs as below |
| `audioTrack` | string, optional | the one playing, `a<n>` | |
| `maxBitrateKbps` | integer, optional | the cap in effect | |
| `info` | object, optional | how it's served: `reasons`, `container`, `bitrateKbps`, `video`, `audio` | text made plain, short |

The content parts may add members: [video section 5.3](video.md#53-the-answer)
adds subtitles, forced subtitles, skip markers and seek-bar previews.

Track:

| Field | Type | Content |
|-------|------|---------|
| `id` | string | `a<n>` for audio, `s<n>` for subtitles ([video section 5.3](video.md#53-the-answer)); *n* has no leading zeros |
| `label` | string | the language's name in itself |
| `language` | string, optional | a language tag |
| `channels` | integer, optional | audio channels |
| `role` | string, optional | `commentary` or `description` |
| `default`, `forced`, `hearingImpaired` | boolean, optional | flags |

**IDs.** IDs of tracks and versions must match `^[A-Za-z0-9_-]{1,64}$`
(or be `off`). The viewer's server drops tracks with another ID or an
empty label, and clears other IDs. It drops `title` and `next` whatever
they hold.

**Version IDs** (`versionId`, `versions[].id`) follow the rule of library
IDs ([catalog section 2.1](catalog.md#21-library-ids)): opaque, chosen
by the owning server, not derived from its media server's IDs without a
key. They need to be stable only for the life of a title's copies, since
a viewer's server passes them back only in a later `play` of the same
title. Track IDs (`a<n>`, `s<n>`) are positions, not identifiers, and are
not concerned. *n* is the owning server's own number for the track in
that copy, the same at every `play` of it: its media server's stream
index, for example, so that a film whose video is stream 0 has its
first audio track at `a1`. A viewer's server reads nothing into *n*: it
only sends back an ID a former answer gave it.

**Roles are best effort.** `role` is absent when the owning server
doesn't know it; it may guess from a track's title. A viewer's server
treats an absent role as the title's own sound, so an unflagged
commentary may be chosen by language (section 2.6).

When the answer breaks a "must" above, the viewer's server sends `stop`
for that `id` and gives up.

To change the audio track, the version or the quality, the viewer's
server stops the playback and starts a new one with `play`. It waits up
to 5 seconds for the `stop` to be answered first, so the old playback
doesn't count against the limit.

### 2.4 What the content parts add

Each content part says which of its kinds `play` takes, what the owning
server serves when the capabilities say nothing, and which members it
adds to the request and the answer:

| Content part | `play` takes | Nothing said | Adds |
|--------------|--------------|--------------|------|
| `video` | films and episodes | H.264 and AAC in HLS | subtitles and forced subtitles, skip markers, seek-bar previews, video capabilities ([video section 5](video.md#5-playing-a-film-or-an-episode)) |
| `music` | tracks | AAC in HLS, audio only | nothing; audio limits in `limits` ([music section 5](music.md#5-playing-a-track)) |

A viewer's server ignores the members a content part adds when the title
played isn't of that part.

### 2.5 Refusals

`400` `invalid-request`, `404` `not-found`, `409`, `503`, `500` as above,
`429` `rate-limited` ([core section 8.3](core.md#83-limits-and-overload)),
and `404` `not-supported` when `playback` is off. A `409` is one of two
refusals, told apart by the problem `type` of its signed body:

| Status | `type` | Meaning | The viewer's server tells the viewer |
|--------|--------|---------|--------------------------------------|
| 409 | `stream-limit-reached` | the pairing's streams are all in use, or its limit is 0 | busy now |
| 409 | `not-playable` | the owning server has no way to play its copy | not playable |
| 503 | `server-busy` | the server's transcodes are all in use | busy now |

The owning server must send `not-playable` with `409` only for a title it
can't play, and `stream-limit-reached` for its limits. A viewer's server
must read the `type` of a signed `409`: `not-playable` means not
playable; any other `type`, or a body without one, means busy now. A body
that isn't JSON counts as a body without a `type`. A signed `429` or
`503`, an unsigned refusal, and no answer at all mean busy now, as
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

**How it's served.** `method` follows from the copy, the audio track and
the capabilities, the cheapest first: `direct`, then `remux`, then
`transcode`. There is no request member to forbid transcoding.

| What the copy and track need | Limits (section 6) | Result |
|------------------------------|--------------------|--------|
| `direct` or `remux` | a stream free | `200` |
| `transcode` | a stream free, a transcode free for the pairing and in the server's budget | `200` |
| `transcode` | the pairing's `transcodes` all used or 0, or the server's budget used | `503` `server-busy` |
| anything | the pairing's `streams` all used, or 0 | `409` `stream-limit-reached` |
| no way to play the copy | any | `409` `not-playable` |

The owning server doesn't fall back to another copy, track or quality to
avoid a transcode, and doesn't refuse for lack of a language. The answer
always says what plays (`versionId`, `audioTrack`, `subtitleTrack`), and
the viewer's server shows that, not what it asked for.

**Overfin.** Since 2026-11-16, an `audioTrack` or `subtitleTrack` that
isn't a track of the copy counts as absent, as above (before, the audio
track was passed to its media server as is, and the subtitle track
echoed). The rest is as above.

## 3. progress

`progress`, under the peer's base path. The viewer's server tells the
owning server that a playback is still watched, and where.

Request:

| Field | Type | Required | Content |
|-------|------|----------|---------|
| `id` | string | yes | the playback ID from `play`, `^play_[a-z2-7]{26}$` |
| `positionMs` | integer | no | where the viewer is; 0 to 8,640,000,000 |
| `paused` | boolean | no | whether the viewer paused |

Answer `200`: `{}`.

Refusals: `400` for a malformed body; `410` `playback-stopped` when the
playback isn't live, or isn't this pairing's.

The viewer's server should send `progress` while the viewer watches,
paused too: no more often than every 45 seconds, but often enough that 2
minutes never pass without one (below; once a minute does), and at once
when the viewer pauses or resumes. The
owning server ends a playback that has neither a progress report nor a
media request for 2 minutes ([media section 4](media.md#4-how-long-a-ticket-works)).
An open media response held by a paused player counts as use.

## 4. stop

`stop`, under the peer's base path. The viewer's server tells the owning
server a playback is over.

Request:

| Field | Type | Required | Content |
|-------|------|----------|---------|
| `id` | string | yes | the playback ID, `^play_[a-z2-7]{26}$` |
| `positionMs` | integer | no | where the viewer stopped; 0 to 8,640,000,000 |

Answer `200`: `{}`, whether or not the playback was live. A playback ID
of another pairing changes nothing.

Refusals: `400` for a malformed body, a position out of range included.

## 5. Limits

The owning server sends, in its `libraries` answer
([catalog section 2](catalog.md#2-libraries)), what the caller may use of
it at once. It sends `limits` only when `playback` is on between the two.

| Field | Type | Range | Content |
|-------|------|-------|---------|
| `streams` | integer | 0 to 50 | playbacks at the same time |
| `transcodes` | integer | 0 to `streams` | of those, how many may be transcoded |
| `bitrateKbps` | integer | 0 to 1,000,000 | a bitrate cap in kbit/s; 0 means none |

A content part may add members: [music section 5.1](music.md#51-limits)
adds `audioStreams` and `audioTranscodes`.

The caller uses the limits only to avoid offering its people what the
owning server would refuse. The owning server enforces them itself
(section 6). A caller ignores limits out of range. A server that doesn't
enforce limits sends none.

**Shared limits.** Limits may be shared with other peers: a group's pool
is shared by all its members ([groups section 9](groups.md#9-sharing-and-limits)).
A peer linked through a group is sent the most generous of its relations'
limits, and may be refused (`409` `stream-limit-reached`) before reaching
them, its pool being in use by others. A playback counts against exactly
one pool: the first with room, the individual relation's own limits
first.

## 6. What the owning server enforces

The owning server decides alone. The viewer's server can't widen
anything.

- **What is played.** `play` finds copies only in libraries offered to
  that peer ([catalog section 5](catalog.md#5-what-is-shared)).
- **Stream caps.** Per pairing: at most `streams` playbacks at once
  (0 refuses every `play`), and at most `transcodes` of them transcoded.
  Across the server: its transcode budget, less the transcodes it keeps
  for its own people. A server whose media server has no such budget
  (Jellyfin starts as many transcodes as it is asked) makes one, an
  operator setting counting its own people's transcodes and its peers'
  together. A remux found to re-encode the video counts as a
  transcode from then on, when the owning server can tell (its media
  server names the encoder of the job, for example); one that can't asks
  for a remux only when its media server copies the video. The bitrate is the lowest cap of section 2.2,
  step 6.
- **Defaults.** A reasonable start for a new pairing is 2 streams, 1
  transcode and no bitrate cap; the operator changes them.
- **Media.** Only through the ticket, only from a client that doesn't
  look like a browser, only while the playback is live
  ([media section 4](media.md#4-how-long-a-ticket-works)).
- **What a peer never learns:**
  - nothing about the viewer: no account, profile, name, email, address,
    IP, browser or device. Nothing in `play`, `progress` or `stop` names a
    person.
  - not the viewer's exact device: capabilities and bitrates are
    coarsened (section 2.1).
  - of the owning server: never files, paths, or its media server's item
    IDs, not even inside version IDs (section 2.3).

  What a peer does learn: the title, the start position, the language
  tags wanted, the coarsened capabilities, and progress over time, for
  one unnamed viewer of the other server.

### 6.1 A playback's states

A playback exists only on the owning server, from a `200` to `play`. It
is in one of three states:

| State | How it gets there | `progress` | `stop` | Media with its ticket |
|-------|-------------------|------------|--------|-----------------------|
| **started** | `play` answered `200` | `200`; the playback becomes active | `200`; the playback ends | served; the playback becomes active |
| **active** | a `progress` or a media request since it started | `200` | `200`; the playback ends | served |
| **ended** | `stop`, expiry, or one of the endings of section 6.3 | `410` `playback-stopped` | `200`, nothing changes | `404` |

- **started** and **active** are both *live*: the ticket works, and the
  playback counts against the limits. They differ only in that an active
  playback has been used since it started; a server may treat them as one.
- **ended** is final. A playback never comes back to life; the viewer's
  server starts a new one with `play`, with a new ID and a new ticket.
- A playback ID of another pairing, or one the owning server never gave,
  is treated as ended: `410` on `progress`, `200` on `stop`, and nothing
  changes.

**Duplicate and late calls.**

- A second `stop` of the same playback is answered `200`, and changes
  nothing. A viewer's server may send `stop` again when it got no answer.
- A `progress` that arrives after the playback ended, however it ended,
  gets `410` `playback-stopped`, and the playback stays ended. The viewer's
  server stops its player.
- Calls of one playback may cross on the way: a `progress` sent before a
  `stop` may arrive after it. The owning server goes by its own state,
  never by the order the viewer's server sent things in.

### 6.2 Limits at once

The owning server must enforce the stream and transcode caps
atomically: checking that a stream (and, for a transcode, a transcode) is
free and counting the new playback is one step, which no other `play`
can come between. So two `play`s that arrive together can never both
take the last stream. A `play` that loses this race is refused as if it
had come later (`409` or `503`, section 2.6). A server may check the
caps earlier too, to refuse before doing any work, but the check that
counts is the atomic one. With shared limits (section 5), choosing the
pool and counting the playback in it is that same one step.

### 6.3 How a playback ends

A live playback ends, and its ticket stops working at once, when:

- the viewer's server sends `stop`;
- it has had neither a `progress` nor a media request for 2 minutes
  (expiry; a media response still open counts as use);
- it has lasted longer than the owning server's longest playback, if it
  has one (Overfin: 24 hours);
- the pairing ends, the caller is no longer `paired`, or its key is
  suspended ([core section 12.6](core.md#126-revocation));
- the library holding the copy is no longer offered to that peer;
- it is a remux found to re-encode the video, and no transcode is left
  for it, in the pairing or in the server's budget (section 6.2);
- the owning server's operator stops it.

When it ends, the owning server stops the work it started for it (the
transcode, open media responses) and frees its stream. An abandoned
playback (the viewer's server crashed, or lost its network) is cleaned up
by expiry: the owning server checks for expired playbacks at least once a
minute, so that one never holds a stream much longer than 2 minutes after
its last use.

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

## 7. Security considerations

- **The ticket** is the only credential for the media, and is carried by
  `play`'s answer. That is why `play` needs a *trusted* or *named*
  certificate ([core section 3.2](core.md#32-https-certificates)).
- **Hostile answers.** Treat every field of a `play` answer as untrusted:
  check IDs, URLs against the owning server's stream root, numbers
  against their bounds, and clean text.
- **The viewer's privacy.** A viewer's server never passes a peer's URLs
  to a browser, never sends anything that names a person, and coarsens
  what it says about devices (section 2.1).

## 8. Open questions

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
3. **No member to forbid transcoding** (section 2.6): a viewer's server
   that doesn't want one has no way to say so; the limits decide.
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
