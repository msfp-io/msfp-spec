# MSFP version 0: the music part, version 0

**Draft.**

The `music` part shares artists, albums and tracks, as music servers
hold them: albums by edition, tracks by their place on an album. It is a
content part: it says what the shared parts carry for music: the title
kinds, keys and descriptions in [catalog](catalog.md), and what
[playback](playback.md)'s `play` takes and answers for a track. It has
one call of its own, `lyrics` (section 6), and uses the catalog's `image`
call ([catalog section 4.4](catalog.md#44-the-image-of-a-local-title-image))
for the images of `local` artists and albums (section 4). It needs the
[core](core.md), `catalog`, `playback` and `media`; if any of them is
off between two servers, `music` is off too
([core section 10](core.md#10-versions-and-parts)). A server lists
`music` in `hello` only when it shares or plays music. It comes from
[proposal 11](../../proposals/0011-music.md), revised by
[proposal 16](../../proposals/0016-version-0-revision.md) and
[proposal 17](../../proposals/0017-neutrality.md). Notes for implementers,
not normative, are in [implementers](implementers.md#music).

The words "must", "must not" and "should" carry their usual meaning in
specifications.

## Contents

1. [Terms](#1-terms)
2. [Title keys](#2-title-keys)
   - [2.1 The `mb` scheme](#21-the-mb-scheme)
   - [2.2 The `local` scheme](#22-the-local-scheme)
3. [What a title carries](#3-what-a-title-carries)
   - [3.1 Other identifiers](#31-other-identifiers)
   - [3.2 What the caller keeps](#32-what-the-caller-keeps)
   - [3.3 A track's copies](#33-a-tracks-copies)
4. [Pictures](#4-pictures)
5. [Playing a track](#5-playing-a-track)
   - [5.1 Limits](#51-limits)
   - [5.2 The queue](#52-the-queue)
6. [Lyrics](#6-lyrics)
7. [Libraries](#7-libraries)
8. [Security considerations](#8-security-considerations)
9. [Open questions](#9-open-questions)

## 1. Terms

- **MBID**: a MusicBrainz identifier, the lowercase UUID MusicBrainz
  gives an artist, a release group, a release, a recording or a release
  track.
- **Release**: one edition of an album (a remaster, a country's
  pressing, a deluxe edition), with its own track list.
- **Release group**: every release of one album.
- **Release track**: one place on a release: disc *d*, track *n*.
- **Recording**: one performance, wherever it appears: the same
  recording may be a release track of several releases (the album, a
  compilation, a deluxe edition).
- **Music title**: an artist, an album or a track.
- **Album**: a release.
- **Track**: a release track.
- **Schemes**: `music` 0 has two: `mb` (MusicBrainz) and `local` (the
  owning server's own, for untagged music).

A track key takes the release track's MBID, and `ids`' `mbrecording`
the recording's (sections 2.1 and 3.1). Tags and media servers name the
two in different ways; [implementers](implementers.md#music) lists some.

## 2. Title keys

Three kinds of title, under two schemes:

| Kind | `mb` key | `local` key |
|------|----------|-------------|
| artist | `artist/mb/<artist mbid>` | `artist/local/<opaque>` |
| album | `album/mb/<release mbid>` | `album/local/<opaque>` |
| track | `track/mb/<release track mbid>` | `track/local/<opaque>` |

A title's `kind` in `titles` is `artist`, `album` or `track`: the kind
its key names. `play` takes only tracks, of either scheme.

An owning server keys a title under `mb` whenever it has its MBID, under
`local` otherwise. Each scheme gives one form for each title; a key in
another form is a different string, and the receiver skips it
([catalog section 4](catalog.md#4-title-keys)).

Test cases: `keys` in
[vectors/v0/music.json](../../vectors/v0/music.json).

### 2.1 The `mb` scheme

- **An album is a release**, not a release group: two editions are two
  albums, each with its own track list, as music servers show them. The
  release group goes in `ids` (section 3.1), so that a viewer's server
  can group a release group's editions, and tell that a peer's album is
  an edition of one it has.
- **A track is a release track**: its place on one release. A recording
  on its album and on a compilation is two tracks, one on each album,
  so neither album's track list has a hole. The recording goes in `ids`
  (section 3.1), so that a viewer's server can tell that it already has
  the song.
- `<mbid>` is 36 characters,
  `[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}`,
  lowercase. Any other form is a different string, and the receiver
  skips it.
- A `track/mb` track is on an `album/mb` album: the release its release
  track belongs to (section 3.2).
- MusicBrainz sometimes merges two entities into one; files tagged
  before the merge keep the old MBID. Two servers may then hold one
  release under two keys. They are two titles: keys are compared byte
  for byte ([catalog section 4.1](catalog.md#41-same-title-same-key)).

### 2.2 The `local` scheme

For music no public database knows, or that isn't tagged: shop
downloads, rips without a tagger, self-released or home recordings.

- `<opaque>` is 1 to 64 characters of `A–Z a–z 0–9 _ -`, chosen by the
  owning server under the rule of library IDs
  ([catalog section 2.1](catalog.md#21-library-ids)): not derived from
  its media server's IDs, a file, a path or a name without a key. It is
  stable as long as the owning server holds the title.
- A `local` title never matches another server's, not even one with the
  same opaque part: a viewer's server keeps it under the peer it came
  from. `isrc` and `acoustid` in `ids` (section 3.1), where known, let a
  viewer's server find the same song elsewhere.
- **The owning server chooses** whether to share `local` titles at all,
  off by default, per pairing and per group, as
  [catalog section 5](catalog.md#5-what-is-shared) says.
- A `local` title carries the same description as an `mb` one
  (section 3), and may carry `image`, a picture of its own (section 4).
- Titles of both schemes mix freely: a `local` track may be on an `mb`
  album (a file with a release MBID but no release track MBID), and an
  `mb` album may credit a `local` artist (one without an artist MBID).

## 3. What a title carries

Video titles carry keys and copies only: the viewer's server looks names
and pictures up in TMDB. Music can't work that way. At one request a
second, a library of 20,000 tracks would take more than five hours of
MusicBrainz lookups on every peer that reads it, and MusicBrainz would
be asked the same thing by every server of a federation. So **music
titles carry their own description**, from the owning server, beside the
members of [catalog section 3](catalog.md#3-titles):

| Field | Kinds | Type | Content |
|-------|-------|------|---------|
| `name` | all | string | the title's name, text, at most 256 characters; required |
| `sortName` | album | string | the name to sort it by (`"Album, The"`), at most 256 characters |
| `artists` | album, track | array of string | the artist keys credited, in order, at most 16 |
| `artistNames` | album, track | string | the credited names as written (`"Simon & Garfunkel"`), at most 512 characters |
| `primaryType` | album | string | the release group's MusicBrainz primary type (`album`, `single`, `ep`, `broadcast`, `other`…), open |
| `secondaryTypes` | album | array of string | its MusicBrainz secondary types (`compilation`, `soundtrack`, `live`, `remix`, `dj-mix`, `audio drama`, `mixtape/street`…), open, at most 8 |
| `compilation` | album | boolean | `true`: an album of various artists |
| `date` | album, track | string | the date of this edition: `YYYY`, `YYYY-MM` or `YYYY-MM-DD` |
| `originalDate` | album, track | string | the date of the album's (or the track's) first edition, in the same form |
| `genres` | album, track | array of string | at most 8, from the files' tags, each at most 64 characters |
| `album` | track | string | the album key it is on; required for a `track/mb` track |
| `disc`, `number` | track | integer | its place on the album: 1 to 999 each |
| `copies` | track | array of copy | its copies, in the owner's order of preference, at most 8; required (section 3.3) |
| `image` | `local` artists and albums | boolean | `true`: the owning server has a picture of it (section 4) |
| `ids` | all | object | other identifiers (section 3.1) |

- **`primaryType`** and **`secondaryTypes`** are the release group's
  types as MusicBrainz names them, lowercased (`Audio drama` is sent
  `audio drama`): each 1 to 32 characters of `a–z 0–9 space . _ - /`,
  with no space at either end. The lists are MusicBrainz's and open:
  a value the viewer's server doesn't know is kept as it is
  ([core section 8.5](core.md#85-unknown-values)), and the viewer's
  server maps the types to its own.
- **`compilation`** is the tag of that name: albums of various artists,
  which music servers file apart. It doesn't follow from
  `secondaryTypes`: a soundtrack may be one or not.
- **`date`** and **`originalDate`**: as precise as the owning server
  knows them. Without `originalDate`, a viewer's server sorts and shows
  by `date`.
- A track may carry its own dates and genres (a compilation's tracks
  have them each). Without them, its album's are its own.
- An owning server lists, in the same library, the album and the artists
  its tracks name. A viewer's server shows a track whose album or artist
  it hasn't got with the names the track carries.

These are untrusted text, cleaned as
[core section 9.1](core.md#91-pair) cleans a server's name (with the
lengths above instead of 80), and never more than a label: two titles are
the same when their keys are equal, whatever their names
([catalog section 4.1](catalog.md#41-same-title-same-key)). A viewer's
server may replace them with its own data for keys it knows.

A change of any of these members, a copy's included, is a change of the
title: a changes reading sends it again
([catalog section 3.2](catalog.md#32-whole-list-or-changes)).

Example: an album, and a track on it:

```json
{"key":"album/mb/0b000000-0000-4000-8000-000000000002","kind":"album","library":"m1","name":"Bridge over Troubled Water","artists":["artist/mb/0a000000-0000-4000-8000-000000000001"],"artistNames":"Simon & Garfunkel","primaryType":"album","date":"2011-03-08","originalDate":"1970-01-26","genres":["Folk rock"],"ids":{"mbreleasegroup":"0d000000-0000-4000-8000-000000000005"}}
{"key":"track/mb/0c000000-0000-4000-8000-000000000003","kind":"track","library":"m1","name":"The Boxer","artists":["artist/mb/0a000000-0000-4000-8000-000000000001"],"artistNames":"Simon & Garfunkel","album":"album/mb/0b000000-0000-4000-8000-000000000002","disc":1,"number":2,"copies":[{"id":"Vq3kM0aZ7pXc","durationMs":308000,"container":"flac","codec":"flac","size":36214455,"etag":"7f3a9c21","bitrateKbps":936,"sampleRate":44100,"bitDepth":16,"channels":2,"replayGain":{"trackGain":-4.12,"trackPeak":0.988525,"albumGain":-5.3,"albumPeak":1.0},"lyrics":"synced"}],"ids":{"mbrecording":"0e000000-0000-4000-8000-000000000006","isrc":"USSM17000123"}}
```

(The MBIDs of this example and of the test vectors are made up.)

### 3.1 Other identifiers

The schemes and their forms are the
[registry's](../registry.md#ids-schemes); those of music are:

| Scheme | Kinds | Form |
|--------|-------|------|
| `mbreleasegroup` | album | the MBID of the release group the release belongs to |
| `mbrecording` | track | the MBID of the recording the release track holds |
| `isrc` | track | an ISRC, 12 characters `[A-Z]{2}[A-Z0-9]{3}[0-9]{7}` |
| `acoustid` | track | an AcoustID track ID: a lowercase UUID, in the form of an MBID |

MBIDs and AcoustIDs here have the form of section 2.1. A title of either
scheme may carry them: a `local` album may know its release group, a
`local` track its ISRC. As in [video](video.md)'s other identifiers,
`ids` never replaces the key.

### 3.2 What the caller keeps

Besides the rules of [catalog section 3.3](catalog.md#33-what-the-caller-keeps-of-each-title),
title by title:

- the key's scheme is one the caller knows (`mb` or `local` in
  `music` 0), and a `local` key's opaque part has the form of
  section 2.2;
- `name`: cleaned as above. A title without a `name`, or whose `name`
  is empty once cleaned, is skipped.
- `sortName`, `artistNames`: cleaned as above; empty once cleaned counts
  as absent.
- `artists`: each entry a valid artist key (section 2), each once; a bad
  or repeated entry is skipped; at most 16 kept, the first. Here and in
  every "at most *N*, the first" of this part, only kept entries count:
  a skipped entry doesn't use a place.
- `primaryType`: a string of the form of section 3, kept as it is,
  known or not; any other counts as absent. The caller doesn't lowercase:
  `Album` counts as absent.
- `secondaryTypes`: each entry of the same form, each once; a bad or
  repeated entry is skipped alone; at most 8 kept, the first; empty once
  skipped counts as absent.
- `date`, `originalDate` (on albums and tracks): in one of the three
  forms, with a year from 1000 to 9999, a month from `01` to `12` and a
  day that exists in that month (29 February in leap years only); any
  other counts as absent. An `originalDate` whose year is after `date`'s
  counts as absent.
- `genres` (on albums and tracks): each entry cleaned as above, 1 to 64
  characters once cleaned; an empty, over-long or repeated entry
  (compared without case) is skipped alone; at most 8 kept, the first.
- `album`: a valid album key; any other counts as absent. A `track/mb`
  track whose `album` isn't an `album/mb` key is skipped.
- `disc`, `number`: integers from 1 to 999; any other counts as absent.
- `copies`: kept as section 3.3 says. A track with no copy left is
  skipped.
- `image`: kept only on a `local` artist or album; on any other title it
  is ignored.
- `year`: the description of [catalog section 4.3](catalog.md#43-titles-no-public-source-knows-the-description)
  gives one, but music titles carry their dates in `date` and
  `originalDate`: a `year` on a music title, of either scheme, is
  ignored.
- `ids`: values of known schemes in their form, for the kinds the table
  of section 3.1 gives; others are skipped.
- A member the table of section 3 gives for other kinds than the
  title's (an `album` on an album, `primaryType` on a track, `genres` on
  an artist) is ignored, as is a member it doesn't give (`releaseType`).

Members of the wrong JSON type ([core section 2.3](core.md#23-json)):
each counts as absent (a `copies` that isn't an array: the track is
skipped); an entry of `artists`, `secondaryTypes` or `genres` that
isn't a string is skipped alone, as is a value of `ids` that isn't a
string.

Test cases: `titles` in
[vectors/v0/music.json](../../vectors/v0/music.json), each with what the
caller keeps.

### 3.3 A track's copies

A track's `copies` follow the common rules of video's copies
([video section 3.2](video.md#32-copies)): one entry per copy, in the
owner's order of preference, at most 8; the `id` `play` takes as
`versionId`; an exact `size`; an `etag` that changes whenever the copy's
bytes change; a `name` of plain text. Values are exact, as the owning
server knows them: `durationMs` and `bitrateKbps` aren't rounded.
`container` and `codec` are ffmpeg's names, open (core section 8.5). A
track's copy is one audio stream, so it carries these:

| Field | Type | Required | Content |
|-------|------|----------|---------|
| `id` | string | yes | the version ID, as for video |
| `name` | string | no | a quality label ("Hi-Res", "Vinyl rip"), as for video |
| `durationMs` | integer | yes | its length in milliseconds, exact, 1 to 86,400,000 |
| `container` | string | yes | the ffmpeg muxer name of the file's format, the name `ffmpeg -f` takes (`flac`, `mp3`, `mp4`, `ogg`, `wav`, `matroska`…), a token |
| `codec` | string | yes | its audio codec, ffmpeg's `codec_name` (`flac`, `mp3`, `aac`, `alac`, `opus`, `vorbis`, `pcm_s16le`…), a token |
| `codecs` | string | no | its RFC 6381 codecs string (`mp4a.40.2`, `fLaC`, `opus`, `ec-3`), at most 64 characters of `A–Z a–z 0–9 . -` |
| `size` | integer | yes | its exact size in bytes, as for video |
| `etag` | string | yes | its identity, as for video |
| `added` | string | no | the day the owner added it, as for video |
| `bitrateKbps` | integer | no | its overall bitrate in kbit/s, exact, 1 to 100,000 |
| `sampleRate` | integer | no | in Hz, 8,000 to 768,000 |
| `bitDepth` | integer | no | 8 to 32; absent for lossy codecs |
| `channels` | integer | no | 1 to 64 |
| `replayGain` | object | no | its loudness values (below) |
| `lyrics` | string | no | `plain` or `synced`: the owning server has lyrics for it (section 6), synced ones among them for `synced` |

The members [video section 3.2](video.md#32-copies) gives for video only (`edition`,
`startMs`, `parts`, `video`, `audioTracks`, `subtitleTracks`,
`chapters`, `segments`, `previews`, `threeD`) are not sent for a track,
and the caller ignores them.

**ReplayGain.** `replayGain` holds up to four numbers, each optional,
from the files' tags, to the ReplayGain 2.0 reference (−18 LUFS):

| Field | Type | Range | Content |
|-------|------|-------|---------|
| `trackGain` | number | −64 to 64 | the gain to apply to this track, in dB |
| `trackPeak` | number | 0 to 10 | its highest sample, as a linear amplitude (1 is full scale; above 1 for lossy codecs) |
| `albumGain` | number | −64 to 64 | the gain to apply to the whole album, in dB |
| `albumPeak` | number | 0 to 10 | the album's highest sample, linear |

- An owning server whose files hold other loudness values (EBU R128)
  converts them to this reference.
- It serves the audio at its own level, the gain not applied, whatever
  the delivery: the viewer's player applies it, as for local tracks. A
  gain stays right after a conversion; a peak may move slightly.

**What the caller keeps** of `copies`, copy by copy:

- the common rules of [video section 3.8](video.md#38-what-the-caller-keeps-of-copies): a copy missing a required
  member, or whose `id`, `size` or `etag` breaks its form, is skipped
  alone; at most 8 kept, the first; a repeated `id` skipped;
- `durationMs` from 1 to 86,400,000, else the copy is skipped;
- `container`, `codec`: a token
  ([core section 8.5](core.md#85-unknown-values)), kept as it is, known
  or not, never turned into `other`; a value that isn't a token is
  dropped, and the copy, missing a required member, skipped;
- `codecs`: a string of the form above; any other counts as absent;
- `name` and `added` as for video (a `name` cut to 64 characters);
  `bitrateKbps` an integer in the range above, kept exact; any other
  counts as absent;
- `sampleRate`, `bitDepth`, `channels`: integers in their ranges; any
  other counts as absent. A `bitDepth` on a lossy codec, which the owning
  server shouldn't send, is kept all the same: the caller doesn't judge
  codecs;
- `replayGain`: each member a number in its range, the range checked on
  the number as sent, before rounding; then gains rounded to two
  decimals and peaks to six, to the nearest (an exact tie may go either
  way: an owning server that wants one value sends it already rounded);
  any other counts as absent, and a `replayGain` left empty, or that
  isn't an object, counts as absent;
- `lyrics`: `plain` or `synced` exactly; any other counts as absent.

Test cases: tracks with copies in `titles`, in
[vectors/v0/music.json](../../vectors/v0/music.json).

## 4. Pictures

**Album art** comes from a public source by MusicBrainz ID, such as the
Cover Art Archive, fetched by the viewer's server itself, as it fetches
TMDB's pictures for films: by the album key's release, or by its release
group (`ids`' `mbreleasegroup`) when the edition has no cover of its
own. A viewer's server caches each cover. The owning server sends no
picture of an `mb` album: one that no source the viewer's server uses
has art for shows none.

**Artist images** are the viewer's server's choice: a public source that
knows artists by MBID (TheAudioDB, fanart.tv…), fetched by the viewer's
server itself and cached. Nothing in the protocol carries them.

**A `local` artist or album** has no public picture. When its `image` is
`true`, the viewer's server asks for it with the catalog's `image` call,
fetches, checks and re-encodes it (JPEG or PNG, at most 1 MiB) as
[catalog section 4.4](catalog.md#44-the-image-of-a-local-title-image)
says, and shows it as the owner's. The owning server sends pictures of
`local` titles only.

A track, of either scheme, shows its album's art.

## 5. Playing a track

- `play` ([playback](playback.md#2-play)) takes a track key, of either
  scheme. Everything else stays: copies and `versionId`, `delivery`
  (the file as it is, a remux, a conversion), limits, ticket,
  `progress`, `stop`, restarts on an unchanged `etag`, downloads and
  scan access where the owner allows them.
- **Audio only.** The answer has no subtitle tracks, no forced tracks,
  no fonts and no previews, and a track's copies no chapters or skip
  markers (section 3.3); a viewer's server drops any it gets for a
  track. `audioTracks` lists the stream's one audio track.
- **The viewer's server chooses.** It has every copy's facts and knows
  its player: it asks for what it wants with `delivery`
  ([playback section 2.7](playback.md#27-delivery)). With
  `delivery: file` the owning server serves the copy as it is, by byte
  ranges: FLAC to a player that plays FLAC is never converted.
- **Without `delivery`** the owning server serves playback's baseline
  for a track: AAC in HLS, audio only, which every viewer's server must
  play. It may serve a copy as it is when the copy already meets it.
- **Gapless** playback needs the next track's bytes before the current
  one ends (section 5.2).
- A viewer's server that plays an album without a queue calls `play` for
  each track as it comes, and `stop` for the one before. Skipping fast
  then reaches the `play` limit of
  [core section 8.3](core.md#83-limits-and-overload) (suggested: 30 a
  minute), and is refused with `429`; a queue (section 5.2) avoids that.

### 5.1 Limits

An audio playback counts as a stream, like a video one. The owning
server may count audio separately; it says so with two members of
`limits` ([playback](playback.md#5-limits)):

| Field | Type | Range | Content |
|-------|------|-------|---------|
| `audioStreams` | integer | 0 to 1,000 | audio playbacks at the same time |
| `audioTranscodes` | integer | 0 to `audioStreams` | of those, how many may be transcoded |

Absent: audio counts in `streams` and `transcodes`.

### 5.2 The queue

A `play` of a track may name the tracks that follow it
([playback section 2.1](playback.md#21-the-request)). The owning server prepares them under the same
playback, so the viewer's server can go from one to the next with no
gap and no new `play`. Request member:

| Field | Type | Required | Content |
|-------|------|----------|---------|
| `queue` | array of string | no | the track keys to play after `title`, in order, at most 5 |

- The owning server refuses the request (`400`) when `queue` isn't an
  array of strings, holds more than 5 entries, or holds an entry that
  isn't a track key in canonical form (section 2), `title` itself or a
  repeat.
- A track of the queue the owning server doesn't hold in the libraries
  it offers the caller, or can't play, is left out of the answer; the
  request isn't refused for it.
- Each entry plays its first copy, the owner's preferred, under the
  request's `delivery`, or as the baseline without one (section 5).
- **Counting.** The playback, queue included, counts once against
  `streams` (and `audioStreams`), and once against `transcodes` (and
  `audioTranscodes`) when any of its tracks is converted. It stays live
  while any of its tracks is read
  ([media section 4](media.md#4-how-long-a-ticket-works));
  `stop` ends it whole.
- **Prepared** means that the owning server has each entry's first bytes
  ready before the one before it ends: a converted entry starts
  converting while the one before plays.

Answer member, beside playback's own:

| Field | Type | Content | What the viewer's server requires or does |
|-------|------|---------|-------------------------------------------|
| `queue` | array of entry | the queued tracks prepared, in the request's order | an entry that fails a check below is dropped alone; at most 5 kept |

Entry:

| Field | Type | Content | What the viewer's server requires or does |
|-------|------|---------|-------------------------------------------|
| `title` | string | its track key | must be one of the request's `queue`, each once, in the request's order |
| `versionId` | string | the copy playing | the form of version IDs ([playback](playback.md#23-the-answer)) |
| `method` | string | `direct`, `remux` or `transcode` | as the answer's own |
| `stream` | object | `url`, `protocol`, `mimeType` (optional), under the playback's ticket | checked as [media](media.md#1-the-urls) says, with the entry's own URL (below) |
| `durationMs` | integer, optional | the copy's length | outside 0 to 86,400,000: set to 0; not an integer: absent |

- **An entry's URL.** Each queued track has its own stream URL, under
  the playback's ticket (the ticket of the answer's own `stream.url`):
  `<stream root><ticket>/queue/<n>/file` for `protocol` `file`, or
  `<stream root><ticket>/queue/<n>/master.m3u8` for `hls`, with the HLS
  files it names under `<stream root><ticket>/queue/<n>/`
  ([media section 1](media.md#1-the-urls)). `<n>` is the entry's place in
  the request's `queue`, 0 to 4, in decimal without leading zeros: a
  track the owning server leaves out keeps its place empty, and moves no
  other entry. An entry whose URL isn't exactly that form, for its
  protocol and its place, or names another ticket, is dropped. An answer
  whose own `stream.url` names no ticket has every entry dropped.
- **An entry's `mimeType`**, when sent, is a string, else the entry is
  dropped. For `hls` it must be `application/vnd.apple.mpegurl`, else
  the entry is dropped. For `file` it isn't checked: the viewer's server
  sets the type of what it relays itself
  ([media section 3](media.md#3-what-is-fetched)).

- **The track playing.** `progress` and `stop` of a playback with a
  queue may carry `title`: the key of the track playing, the request's
  `title` or one of the answer's `queue` entries; `positionMs` is then a
  position in that track. Absent, it is the request's `title`. Any other
  `title`, or one that isn't a string, is refused (`400`). A playback
  without a queue (a film's too) takes only its own `title` there. Test
  cases: `reports` in
  [vectors/v0/playback.json](../../vectors/v0/playback.json), those
  named `music:` and those with a `title`.
- **Past the queue.** Before the last entry ends, the viewer's server
  may send a new `play` for the track after it, with its own `queue`,
  and stops the old playback once the new one's first track plays. The
  two count as two playbacks in between; when the limits leave no room
  for that, it stops the old playback first, and the listener may hear
  a gap there.
- A skip within the queue reads the next entry's stream; a skip out of
  it is a new `play`, as above.

Test cases: `queue` in
[vectors/v0/music.json](../../vectors/v0/music.json): requests the
owning server refuses or accepts, and answers with what the viewer's
server keeps.

## 6. Lyrics

A copy whose `lyrics` is `plain` or `synced` (section 3.3) has lyrics
the viewer's server may fetch with `lyrics`, under the peer's base path,
signed as every call is
([core section 8](core.md#8-common-rules-for-every-call)); it needs no
ticket and is no playback. The request names a track and a copy, and
is answered with lyrics shaped as OpenSubsonic's `structuredLyrics`, so
that music servers built on that API pass them on as they are, with
times in milliseconds.

Request:

| Field | Type | Required | Content |
|-------|------|----------|---------|
| `title` | string | yes | a track key, as the owning server sent it in `titles` |
| `versionId` | string | yes | the copy's `id` |

Answer `200`, at most 1 MiB of JSON:

| Field | Type | Content |
|-------|------|---------|
| `structuredLyrics` | array of lyrics | at most 8: a set per language, kind and timing |

Lyrics:

| Field | Type | Required | Content |
|-------|------|----------|---------|
| `lang` | string | no | the language, a tag ([catalog section 3](catalog.md#3-titles)); absent when unknown |
| `kind` | string | no | OpenSubsonic's `main`, `translation` or `pronunciation`, open; absent means `main` |
| `synced` | boolean | yes | `true`: each line has a `start` |
| `offset` | integer | no | milliseconds to add to every `start`, −600,000 to 600,000; absent means 0 |
| `displayArtist`, `displayTitle` | string | no | the names the lyrics file gives, at most 512 and 256 characters |
| `line` | array of line | yes | at most 2,000 |

Line:

| Field | Type | Content |
|-------|------|---------|
| `start` | integer | synced lyrics only: when the line starts, in milliseconds from the track's start, 0 to 86,400,000 |
| `value` | string | the line, plain text, at most 512 characters; may be empty (a pause, a break between verses) |

An owning server converts what its media server holds (LRC, TTML, SRT,
or times in other units) to this shape, in milliseconds; word timing is
not sent.

**What the viewer's server keeps:**

- the answer read whole, refused past 1 MiB (an answer of exactly
  1,048,576 bytes is read; the vectors leave this bound out, for the
  size of the file); a `structuredLyrics` that isn't an array fails the
  fetch;
- per set: `synced` a boolean, else the set is skipped; `lang` a valid
  tag ([catalog section 3](catalog.md#3-titles): exactly the form a
  server sends, so `fre` and `fr-ca` count as absent), else absent; `kind` a token
  (core section 8.5), kept as it is, known or not, else `main`; a
  viewer's server never shows a set of a kind it doesn't know as the
  main lyrics; `offset` an
  integer in range, else 0; `displayArtist`, `displayTitle` cleaned as
  section 3's names, empty counting as absent;
- per line: `value` a string, cleaned as section 3's names but kept when
  empty, cut to 512 characters; for synced lyrics, `start` an integer in
  range, a line without one or whose `start` is before the line before
  skipped; for plain lyrics, `start` ignored;
- at most 2,000 lines kept per set, the first, and 8 sets, the first,
  counting kept lines and sets only; a
  set with no line left is skipped, and an answer with no set left is a
  failed fetch;
- lines are text, never markup.

Refusals: `400` `invalid-request` for a malformed request; `404`
`not-found` for a track or a copy it doesn't offer the caller, or a copy
with no lyrics; `404` `not-supported` when `music` is off; `429`
`rate-limited` when asked too often
([core section 8.3](core.md#83-limits-and-overload)). A viewer's server
asks for a copy's lyrics when a viewer wants them, and keeps them while
the copy's `etag` holds.

Test cases: `lyrics` in
[vectors/v0/music.json](../../vectors/v0/music.json), each with what
the viewer's server keeps.

## 7. Libraries

A `music` library holds artists, albums and tracks. A `mixed` library
may hold them beside films and series. A receiver that speaks `music`
reads them from `titles` ([catalog section 3](catalog.md#3-titles)), in
the same paging and the same changes.

## 8. Security considerations

- **Descriptions and lyrics are untrusted text** from another server:
  clean them, show them as text, never as markup, and never let a name
  decide that two titles are the same.
- **MusicBrainz and picture sources.** A viewer's server fetches covers and artist images itself, never through
  a URL a peer sent, and caches them, so a peer can't make it fetch
  anything else, nor learn when a viewer looks. It names itself in its
  User-Agent and keeps to MusicBrainz's rate of one request a second
  when it asks MusicBrainz anything.
- **Pictures of `local` titles are images from another server**: a
  viewer's server checks and re-encodes them as
  [catalog section 4.4](catalog.md#44-the-image-of-a-local-title-image)
  says, so that only pixels reach the viewer.
- **ReplayGain from a peer** sets a volume: a viewer's player applies
  a gain within the ranges of section 3.3 only, and should not amplify
  past full scale (it lowers the gain to fit the peak).
- **The queue** lets one `play` prepare up to 6 tracks: an owning server
  bounds what it converts ahead under its transcode budget
  ([playback](playback.md#6-what-the-owning-server-enforces)), and may
  prepare fewer entries than asked.
- **A large library.** A compromised peer can send many titles; readers
  cap what they keep ([catalog section 6](catalog.md#6-security-considerations)).

## 9. Open questions

Left for later versions of this part:

1. **Album art from the owning server** for `mb` albums, when public
   sources have none, through `media` as for `local` titles.
2. **Sort names** for artists and tracks ("Beatles, The"): proposal 16
   gives `sortName` to albums only.
3. **Credits and roles**: one name per artist key
   (`[{name, key, joinPhrase}]`, MusicBrainz's artist credit), and
   composers, conductors and performers. Some media servers keep one
   artist MBID per track ([implementers](implementers.md#music)).
4. **A queue past 5 tracks without a gap** when the limits leave room for
   one playback only (section 5.2).
5. **Music videos**: a track's video, or a film? Left out.
6. **Watched state** for music is play counts and ratings, not
   "watched": nothing in this part shares it, as nothing shares it for
   video either.

Settled by proposal 16: untagged libraries (the `local` scheme), lyrics
(section 6), gapless playback (section 5.2), and descriptions for video
(only `local` titles carry one).

**Choices this draft made, confirmed by the owner on 2026-11-17** (when
proposal 11 was written into this part), still standing:

7. **A title without a name is skipped** (section 3.2): a viewer's
   server has nothing else to show for a key it doesn't know.
8. **What the caller keeps** (section 3.2): a value out of its range or
   form counts as absent rather than skipping the title; `artists`
   entries are kept once each, the first 16; members of the wrong JSON
   type count as absent, as for video titles.
9. **MBIDs in `ids`** take the lowercase form of keys (section 2.1).
10. **Limits**: `audioTranscodes` ranges from 0 to `audioStreams`, as
    `transcodes` to `streams`; absent, audio counts in `transcodes` too,
    not only in `streams`.

**Choices made when proposal 16 was written into this part** (chosen by
the writer, not yet confirmed by the owner; proposal 16 gives no bound
for them):

11. **Names**: music titles of both schemes keep the 256 characters of
    `name`, not the 128 of the `local` description: a `local` music
    title carries music's description, which is already longer.
12. **Dates**: `YYYY`, `YYYY-MM` or `YYYY-MM-DD`, years 1000 to 9999,
    real days only; an `originalDate` after `date` by year is dropped.
    `year` and `durationMs` leave the track: dates go to the album,
    lengths to the copies. (Proposal 17 gives tracks their own dates and
    genres again, optional.)
13. ~~**`releaseType`**~~ Replaced by proposal 17 with MusicBrainz's own
    `primaryType` and `secondaryTypes` (section 3).
14. **Genres**: 1 to 64 characters each, repeats compared without case.
15. **`album` required for `track/mb` tracks**, and an `mb` track's
    album is an `mb` album; a `local` track may have none.
16. **Copies of a track**: `durationMs` 1 to 86,400,000;
    `bitrateKbps` 1 to 100,000 (exact since proposal 17); `sampleRate` 8,000 to
    768,000 Hz; `bitDepth` 8 to 32; `channels` 1 to 64; a track with no
    copy left is skipped.
17. **ReplayGain**: ReplayGain 2.0's reference; gains in dB from −64
    to 64, two decimals; peaks linear from 0 to 10, six decimals; other
    loudness values converted; the owning server never applies the gain.
18. **Lyrics**: a signed call of its own, `lyrics`, with no ticket (the
    proposal says only "a call"); the request names a track and a copy; at most 1 MiB,
    8 sets, 2,000 lines of 512 characters; `offset` within ±10 minutes;
    `kind` from OpenSubsonic's three; empty lines kept; no word timing.
19. **The queue**: the member is `queue` in request and answer; a
    malformed queue refuses the `play`, a track that can't be played is
    left out; entries play their first copy; one transcode counted when
    any entry is converted; past the queue, a new `play` before the
    last entry ends.
20. **`image`** on `local` artists and albums only; tracks show their
    album's. It is fetched with the catalog's own `image` call
    ([catalog section 4.4](catalog.md#44-the-image-of-a-local-title-image)).
21. **`acoustid`** in the lowercase UUID form of MBIDs, on tracks.
22. **The track playing in a queue**: `progress` and `stop` carry
    `title`, the queue entry playing (section 5.2).

**Choices made when proposal 17 was written into this part** (chosen by
the writer, not yet confirmed by the owner):

23. **Types**: no space at either end of a `primaryType` or a
    secondary type; the caller doesn't lowercase (a type in another case
    counts as absent); a repeated secondary type is skipped.
24. **Copies of a track** stay at most 8: proposal 17 raises video's
    copies to 16 and names no change for music.
25. **`bitrateKbps`** from 1, not 8: the lower bound came from the
    rounding to a whole 8, which goes.
26. **`codecs`** on a track's copy, as on video's audio tracks.
27. **A `container` or `codec` that isn't a token** skips the copy: the
    member is dropped, as core section 8.5 says, and both are required.
28. **A copy's `name`** follows video's: 64 characters.
29. **Lyrics `kind`** is open as well: an unknown token is kept, not
    taken for `main`.
