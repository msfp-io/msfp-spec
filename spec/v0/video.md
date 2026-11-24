# MSFP version 0: the video part, version 0

**Draft.**

The `video` part shares films, series and their extras, named by their
TMDB IDs, by their TheTVDB IDs where TMDB has no match, or by keys of
the owner's own for titles no public database knows. It is a content
part: it says what the shared parts carry for films and series: the title
kinds and keys in [catalog](catalog.md), the copies each title describes,
what [playback](playback.md)'s `play` takes and answers, and the
subtitles, fonts and preview sheets [media](media.md) serves. It has one
call of its own, `previews` (section 5.6). It needs the [core](core.md),
`catalog`, `playback` and `media`; if any of them is off between two
servers, `video` is off too
([core section 10](core.md#10-versions-and-parts)). A server lists
`video` in `hello` only when it shares or plays films and series.

The words "must", "must not" and "should" carry their usual meaning in
specifications.

## Contents

1. [Terms](#1-terms)
2. [Libraries](#2-libraries)
3. [Titles](#3-titles)
   1. [What the caller keeps](#31-what-the-caller-keeps)
   2. [Copies](#32-copies)
   3. [Streams and tracks](#33-streams-and-tracks)
   4. [Chapters and skip markers](#34-chapters-and-skip-markers)
   5. [Names](#35-names)
   6. [Colour and dynamic range](#36-colour-and-dynamic-range)
   7. [The order of copies](#37-the-order-of-copies)
   8. [What the caller keeps of copies](#38-what-the-caller-keeps-of-copies)
   9. [Extras](#39-extras)
   10. [The description of a local title](#310-the-description-of-a-local-title)
4. [Title keys](#4-title-keys)
   1. [Episode order](#41-episode-order)
   2. [Several episodes in one copy](#42-several-episodes-in-one-copy)
   3. [Other identifiers](#43-other-identifiers)
   4. [The tvdb scheme](#44-the-tvdb-scheme)
   5. [The local scheme](#45-the-local-scheme)
   6. [Extras' keys](#46-extras-keys)
5. [Playing a film, an episode or an extra](#5-playing-a-film-an-episode-or-an-extra)
   1. [The request](#51-the-request)
   2. [Several episodes in one copy](#52-several-episodes-in-one-copy)
   3. [The answer](#53-the-answer)
   4. [Choosing the subtitles](#54-choosing-the-subtitles)
   5. [Seek-bar previews](#55-seek-bar-previews)
   6. [previews](#56-previews)
6. [Media](#6-media)
   1. [The URLs](#61-the-urls)
   2. [What is fetched](#62-what-is-fetched)
   3. [WebVTT](#63-webvtt)
   4. [ASS](#64-ass)
   5. [Picture subtitles](#65-picture-subtitles)
   6. [Fonts](#66-fonts)
   7. [The image of a local title](#67-the-image-of-a-local-title)
7. [Security considerations](#7-security-considerations)
8. [Open questions](#8-open-questions)

## 1. Terms

- **Video title**: a film, a series, a season, an episode or an extra.
- **Extra**: a trailer, a featurette or the like, shared under the film,
  series or season it belongs to (section 3.9).
- **Title key**: here, the text that names a video title (section 4).
  The general rules of title keys are in
  [catalog section 4](catalog.md#4-title-keys).
- **Scheme**: the source of the identifier in a title key. `video` 0 has
  three: `tmdb` (The Movie Database), `tvdb` (TheTVDB, for series TMDB
  has no match for) and `local` (the owner's own, for titles no public
  database knows).
- **Copy**: one version of a film, an episode or an extra, made of one
  file or of several parts, as [playback](playback.md#1-terms) says. A
  title in `titles` describes its copies (section 3.2).
- **Local title**: a title keyed under the `local` scheme (section 4.5).

## 2. Libraries

A library may hold films, series with their seasons and episodes, and
the extras of its titles, beside titles of other content parts: the
titles' keys say what it holds. A receiver
that speaks `video` reads them from `titles`
([catalog section 3](catalog.md#3-titles)), in the same paging and the
same changes.

## 3. Titles

A video title in `titles` carries the members of
[catalog section 3](catalog.md#3-titles), and these:

| Field | Type | Content |
|-------|------|---------|
| `copies` | array of copy | films, episodes and extras only, and required for them: its copies, in the owning server's order of preference, at most 16 (section 3.2) |
| `place` | object | episodes keyed by their TMDB episode ID only, and required for them: the episode's place in TMDB's numbering, `{"season": <n>, "episode": <n>}` (section 4.1) |
| `with` | array of string, optional | episodes only: the keys of the other episodes the same copy holds (section 4.2) |
| `ids` | object, optional | episodes of the `tmdb` and `tvdb` schemes, and series of the `tvdb` scheme: other identifiers of the same title, by scheme (section 4.3) |
| `extraType` | string | extras only, and required for them: what the extra is (section 3.9) |
| `name` | string | local films and series only, and required for them: the owner's name for the title (section 3.10) |
| `year` | integer, optional | local films and series only (section 3.10) |
| `image` | boolean, optional | local films and series only: `true` when the owner shares one image of the title (section 3.10) |

A title carries no `kind`: its key names it (section 4). Series and
seasons carry no `copies`. Nothing else about a title (no name, no
picture, no summary) is sent, except for local titles as above: a
viewer's server takes the rest from the public database its key names.

A change of any of these members (a copy added, gone or changed, a new
`ids` value, or a new `place`, say) is a change of the title: a changes
reading sends it again
([catalog section 3.2](catalog.md#32-whole-list-or-changes)).

### 3.1 What the caller keeps

Besides the rules of [catalog section 3.3](catalog.md#33-what-the-caller-keeps-of-each-title):

- the key's scheme is one the caller knows (`tmdb`, `tvdb` and `local`
  in `video` 0), and the key is one of the forms of section 4;
- a film, an episode or an extra keeps its copies as section 3.8 says;
  one left with no copy is skipped;
- at most 16 keys in `with`, each a valid episode key of the same series;
  a bad key is skipped;
- in `ids`, values of known schemes in their form (section 4.3); others
  are skipped. `ids` on a film, on a series of the `tmdb` scheme, on a
  local title, an extra or a season is ignored;
- for an episode keyed by its TMDB episode ID, `place` is an object whose
  `season` is an integer from 0 to 9,999 and whose `episode` is an
  integer from 0 to 99,999 ([core section 2.3](core.md#23-json): `1.0`
  and `"1"` aren't); without such a `place` the caller skips the
  episode. Other members of `place` are ignored. `place` on any other
  title (a film, an episode keyed by its numbers) is ignored;
- an extra keeps `extraType` as section 3.9 says, and a local film or
  series its description as section 3.10 says;
- `copies` on a series or a season, and `extraType`, `name`, `year` or
  `image` on a title that may not carry them, are ignored, as is a
  `kind` member.

Members of the wrong JSON type ([core section 2.3](core.md#23-json)): a
`copies` that isn't an array skips the title (a film, an episode or an
extra); a `with` member that isn't an array, or an `ids` member that
isn't an object, counts as absent; an entry of `with` that isn't a
string is skipped alone, as is a value of `ids` that isn't in its
scheme's form.

Test cases: `titles` in
[vectors/v0/video.json](../../vectors/v0/video.json), each with what the
caller keeps.

### 3.2 Copies

A film, an episode or an extra must carry `copies`: its copies, in the
owning server's order of preference, best first (section 3.7), at most
16. Each copy says what a media server needs to list it as one of its own
files before anyone plays it: its length, its size, its media type, its
picture and its tracks.

Copy:

| Field | Type | Required | Content |
|-------|------|----------|---------|
| `id` | string | yes | the copy's version ID: the `versionId` `play` takes and gives ([playback section 2.3](playback.md#23-the-answer)) |
| `edition` | string | no | open (section 3.5): `theatrical`, `directors`, `extended`, `unrated`, `remastered`, `special`, `imax`… |
| `durationMs` | integer | yes | its length in milliseconds, all its parts: the length of what `size` measures, as known |
| `startMs` | integer | no | for an episode in a copy holding several (`with`): where the episode starts in the copy, when the owning server knows it (as `play` starts there, section 5.2) |
| `endMs` | integer | no | for an episode in a copy holding several: where the episode ends in the copy, when the owning server knows it |
| `mediaType` | string | yes | the file's media type (section 3.5): `video/mp4`, `video/matroska`, `video/mp2t`… |
| `size` | integer | yes | its exact size in bytes, all its parts |
| `etag` | string | yes | its identity: it changes whenever its bytes change; 1 to 64 characters of `A–Z a–z 0–9 - _` |
| `parts` | integer | no | the number of files the copy is made of, 2 to 16; absent for one file |
| `video` | video stream | no | its picture (section 3.3) |
| `audioTracks` | array of audio track | yes | its audio tracks (section 3.3), at most 32; may be empty |
| `subtitleTracks` | array of subtitle track | no | its subtitle tracks (section 3.3), at most 64 |
| `chapters` | array of chapter | no | its chapters (section 3.4), at most 256 |
| `segments` | array of segment | no | its skip markers (section 3.4), at most 32 |
| `previewLayout` | object | no | the layout of its seek-bar previews, when the owning server has them (section 5.5) |
| `threeD` | string | no | open (section 3.5): how a 3D picture is laid out: `sbs` or `tab` (side by side, top and bottom, the left eye first), `sbs-rl` or `tab-rl` (the right eye first), `mvc`; absent for 2D. Half or full resolution follows from the picture's size |

The required core (`id`, `durationMs`, `mediaType`, `size`, `etag`,
`audioTracks` with each track's language when known) is what every
owning server knows of its files, so every copy says at least that. Everything
else is sent when the owning server's media server knows it. Values are
exact, as the owning server knows them: none is rounded. A copy carries
no bitrate: `size` × 8 / `durationMs` is its overall rate. It carries no
name either: a viewer's server builds a label from its facts (its
height, its range, its `edition`, its codecs), in its own words.

Example:

```json
{"id":"v_2HqXo3","edition":"directors","durationMs":8160043,
 "mediaType":"video/matroska","size":48213884928,"etag":"k9Tq2w",
 "video":{"codec":"hevc","codecs":"hev1.2.4.L153.B0","width":3840,"height":1600,
  "bitDepth":10,"colour":{"primaries":9,"transfer":16,"matrix":9,"fullRange":false},
  "dv":{"profile":7,"level":6,"compatibility":6,"el":true},"fps":"24000/1001"},
 "audioTracks":[{"id":"a1","language":"en","channels":8,"codec":"truehd",
  "objects":"dolby-atmos","default":true,"original":true}],
 "subtitleTracks":[{"id":"s2","language":"fr","format":"hdmv_pgs_subtitle"}],
 "chapters":[{"startMs":0},{"startMs":312000,"name":"The Desert"}],
 "previewLayout":{"tileWidth":320,"tileHeight":133,"columns":10,"rows":10,
  "intervalMs":10000,"tileCount":817}}
```

**Rules.**

- **Media info comes from the catalog only.** A viewer's server must not
  read a peer's file to learn what it is: each read starts a playback on
  the owning server. It builds its library from `copies`, and reads a
  file only to play it, or under scan access
  ([playback section 8](playback.md#8-scan-access)), where its media
  server reads it to list it.
- **A change of `copies`** (a copy added, gone or replaced, or changed:
  its `etag`, its `previewLayout`) is a change of the title: a changes
  reading sends it again. An owning server changes a copy's `etag`
  whenever its bytes change, and only then.
- **`play`'s answer and the catalog disagree** (the copy changed since
  the reading): the answer wins, and the viewer's server reads the title
  again in its next changes reading.
- **Track IDs.** For a copy of one file, the *n* of `a<n>` and `s<n>` is
  the stream's index in the file, from 0; a subtitle track beside the
  video (`external`) takes a number above the file's streams. For a copy
  of several parts, *n* is the owning server's own. The IDs are the same
  in `copies` and in `play` ([playback section 2.3](playback.md#23-the-answer)),
  at every `play` of that copy.
- **Several parts.** A copy of several parts is never served as `file`
  ([playback section 2.7](playback.md#27-delivery)).
- **What the summary was.** `video` 0 has no title summary of languages
  or picture: a viewer's server that shows a title's audio and subtitle
  languages, or its best picture, works them out from `copies`.
- **Recently added.** A copy says nothing of when the owner added it: a
  viewer's server that shows what is new counts from the day it first
  saw the copy, and the titles of its first reading of a peer count as
  old.
- **Unknown values** are kept as they are
  ([core section 8.5](core.md#85-unknown-values)). A viewer's server
  that doesn't know a copy's `mediaType`, a stream's `codec` or the
  copy's `threeD`, or can't read a `codecs` string (section 3.5),
  treats the copy as one it can't play as it is: it asks for a `remux`
  or a `transcode`, or doesn't offer the copy. An unknown subtitle
  `format` affects that track only. Other unknown values (`edition`, a
  `role`, an `hdr` token, `objects`, `extraType`, a segment `kind`)
  don't stop the copy from playing as it is.

**Pages.** A title with copies is some 2 to 3 KB of JSON: pages are
bounded by size as well as by count, as
[catalog section 3.1](catalog.md#31-paging-and-ordering) says (at most
5,000 titles and 3 MiB of JSON a page).

### 3.3 Streams and tracks

Video stream:

| Field | Type | Content |
|-------|------|---------|
| `codec` | string | open (section 3.5): the registry's token (`h264`, `hevc`, `av1`, `vp9`, `vvc`, `prores`…) |
| `codecs` | string | the stream's codecs string of RFC 6381 (`avc1.640028`, `hvc1.2.4.L153.B0`, `av01.0.08M.10`), section 3.5: required when `codec` is `h264`, `hevc`, `av1` or `vp9`, optional otherwise |
| `width`, `height` | integer | the picture's size in pixels, exact, each 1 to 100,000 |
| `sampleAspect` | string, optional | the sample (pixel) aspect ratio as the stream or container states it, `"<w>:<h>"` (`"32:27"`), each side 1 to 65,535; absent means square pixels |
| `rotation` | integer, optional | the degrees clockwise to turn the picture to show it upright: `0`, `90`, `180` or `270`; absent means 0. A display matrix that mirrors the picture, or turns it by another angle, is left out |
| `bitDepth` | integer, optional | 1 to 16 |
| `colour` | object, optional | its colour, as ITU-T H.273 code points (section 3.6) |
| `hdr` | array of string, optional | open: its enhancement layers and dynamic metadata (`hdr10plus`), section 3.6; at most 8 |
| `dv` | object, optional | the Dolby Vision configuration record, whenever the copy has Dolby Vision (section 3.6) |
| `fps` | string, optional | its frame rate, `"<num>/<den>"` (`"24000/1001"`, `"25/1"`), each 1 to 1,000,000, the fraction reduced: the nominal rate the stream or container declares; for a variable rate, the average |
| `refFrames` | integer, optional | on an `h264` stream only: its declared maximum number of reference frames (`max_num_ref_frames`), 1 to 16 |
| `interlaced` | boolean, optional | `true` for an interlaced picture |

Some players refuse H.264 past a number of reference frames, and
`codecs` doesn't carry it: hence `refFrames`.

Audio track: a track of the copy, with the `a<n>` ID `play` takes, and
these members:

| Field | Type | Content |
|-------|------|---------|
| `id` | string | `a<n>` (section 3.2) |
| `language` | string, optional | a valid language tag ([catalog section 3](catalog.md#3-titles)); sent whenever the owning server knows it |
| `channels` | integer, optional | 1 to 64; the exact layout stays in the stream, and labels come from the count |
| `role` | string, optional | open: a value of the DASH role scheme (`urn:mpeg:dash:role:2011`): `main`, `alternate`, `supplementary`, `commentary`, `dub`, `description`… |
| `default`, `forced` | boolean, optional | the file's flags |
| `codec` | string | open (section 3.5): the registry's token (`aac`, `ac3`, `eac3`, `truehd`, `dts`, `opus`, `flac`, `pcm_s24le`…) |
| `codecs` | string | its codecs string of RFC 6381 (`mp4a.40.2`, `ec-3`, `dtsh`), section 3.5: required when `codec` is `aac` or `dts`, optional otherwise |
| `objects` | string, optional | open (section 3.5): the object-based format it carries, `dolby-atmos` or `dts-x` |
| `sampleRate` | integer, optional | in hertz, 8,000 to 768,000 |
| `bitDepth` | integer, optional | 8 to 32 |
| `original` | boolean, optional | `true`: the title's original language, by the file's own flag only (Matroska's `FlagOriginal` and the like), never worked out from metadata |

A role a viewer's server doesn't know is never taken for the title's own
sound (`main`). A viewer's server picks an Atmos or DTS:X track by
`objects`.

Subtitle track: a track of the copy, with its `s<n>` ID, and these
members:

| Field | Type | Content |
|-------|------|---------|
| `id` | string | `s<n>` (section 3.2) |
| `language` | string, optional | a valid language tag, as for audio |
| `role` | string, optional | open: a value of the same DASH role scheme: `subtitle`, `commentary`, `description`, `easyreader`… |
| `default`, `forced`, `hearingImpaired` | boolean, optional | the file's flags |
| `format` | string | open (section 3.5): its source format, the registry's token (`subrip`, `ass`, `ssa`, `webvtt`, `mov_text`, `ttml`, `hdmv_pgs_subtitle`, `dvd_subtitle`, `dvb_subtitle`, `dvb_teletext`…); absent when the owning server doesn't know it |
| `external` | boolean, optional | `true`: a file beside the video, not inside it |

`forced` is the only flag for forced subtitles, and `hearingImpaired`
the only one for subtitles with sounds (SDH): `forced-subtitle` and
`caption` aren't sent as a `role` (the registry notes how they map).
Captions carried inside the video stream (CEA-608 and 708, in SEI or
user data) aren't listed as subtitle tracks.

A copy lists all its subtitle tracks, those the owning server can't
serve apart too: a copy played as it is carries them (section 5.4 says
which are served apart, and how). A track beside the video that the
owning server can't serve apart isn't listed.

Tracks carry no `label` and no title: a viewer's server names each
language in its own people's words, from the tag, and each track from
its facts (`role`, the flags, `channels`, `objects`).

### 3.4 Chapters and skip markers

Chapter:

| Field | Type | Content |
|-------|------|---------|
| `startMs` | integer | where the chapter starts, in the copy, from 0 to its `durationMs` |
| `name` | string, optional | its name ("Opening", "Chapter 3"), plain text, at most 64 characters |

An owning server sends the chapters its media server knows of the copy;
none known, it leaves `chapters` out. A copy of several parts lists the
chapters of all its parts, each part's shifted by the length of the parts
before it. The viewer's server shows them as its media server shows a
local file's: marks on the seek bar, a chapter list, previous and next
chapter. Chapter images aren't sent: a viewer's server that shows them
takes them from the copy's seek-bar previews (section 5.5), which counts
as using the copy, or shows none.

Segment (a skip marker):

| Field | Type | Content |
|-------|------|---------|
| `kind` | string | open (section 3.5): `intro`, `outro`, `recap`, `preview`, `commercial`… |
| `startMs`, `endMs` | integer | where it starts and ends, in the copy, from 0 to its `durationMs`; `startMs` below `endMs` (a segment whose `startMs` isn't below its `endMs` is out of order) |

An owning server that knows only some kinds sends those. A viewer's
server shows a kind it doesn't know as a plain "Skip". The last `outro`
is the final credits: a viewer's server may count the title as watched
from its start.

### 3.5 Names

The names a copy uses are those of outside vocabularies (IANA media
types, RFC 6381, ITU-T H.273, the DASH role scheme) or, where none
exists, of the MSFP [registry](../registry.md), not those of a product.
Every list is open: a value a receiver doesn't know is kept as it is
([core section 8.5](core.md#85-unknown-values)). The values this part
gives beside an open field are examples; the lists are the registry's,
or the outside vocabulary's it names. A new codec, format or kind is a
registry entry, not a new version of this part.

**`mediaType`** is the file's media type, of the form core section 8.5
gives:

- **IANA's types** where IANA has one: `video/mp4`, `video/quicktime`,
  `video/matroska`, `video/mp2t`, `video/ogg`, `video/3gpp`,
  `audio/mp4`, `audio/matroska`, `audio/flac`, `audio/mpeg`,
  `audio/ogg`…, lowercase (`video/MP2T` is sent `video/mp2t`).
- **Where IANA has none**, the registry names the type in use:
  `video/webm`, `audio/webm`, `video/avi`, `audio/wave`, `audio/aiff`.
  Other spellings (`video/x-msvideo`, `audio/x-wav`, `video/x-matroska`)
  are the registry's aliases: a producer sends the registry's type; a
  receiver may accept an alias as the same.
- **Audio or video.** `audio/<sub>` when the file has no video stream
  other than attached pictures (cover art), else `video/<sub>`. For the
  pairs the registry lists (`mp4`, `matroska`, `webm`, `ogg`, `3gpp`,
  `3gpp2`, `mp2t`), a receiver treats `audio/<sub>` and `video/<sub>` as
  the same container; not for others (`audio/mpeg` is MP3 alone,
  `video/mpeg` MPEG-PS).
- **BDAV.** A transport stream of 192-byte packets (Blu-ray, AVCHD:
  M2TS) is `video/vnd.dlna.mpeg-tts`; one of 188-byte packets is
  `video/mp2t`.
- **From the file where it can.** A producer reads the type from the
  file's first bytes where it can, else maps its media server's
  container name through the registry. For ISO BMFF, the major brand
  (`ftyp`) chooses the type, by the registry's brand table; an unknown
  brand gives `video/mp4` (or `audio/mp4`). A WebM file whose DocType
  isn't known may be sent as `video/matroska`: less precise, never
  wrong. No brand is sent: it only chooses the media type.
- **The relayed type.** A viewer's server relays a copy's file with the
  copy's own `mediaType`, whatever its top-level type
  ([media section 3](media.md#3-what-is-fetched)).

**Codec and subtitle tokens.** The video and audio `codec` and the
subtitle `format` are the registry's tokens. They are spelled as ffmpeg
spells its `codec_name` where ffmpeg has a name (`h264`, `hevc`, `av1`,
`aac`, `eac3`, `truehd`, `dts`, `opus`, `pcm_s24le`, `subrip`, `ass`,
`webvtt`, `hdmv_pgs_subtitle`, `dvd_subtitle`), so that a producer
reading ffprobe maps nothing; one reading the file another way maps
through the registry's crosswalk. The registry, not ffmpeg, is the
authority.

- A producer sends the registry's token. For a codec with none yet, it
  sends the token it proposes for registration, and switches once one is
  registered.
- Each subtitle format's kind (text or picture) is the registry's: no
  field carries it, and the forms a track is served in (sections 5.3
  and 5.4) say what arrives.
- Dolby Vision and encryption aren't codecs: their sample entries map to
  the base codec's token.
- Transcode targets (section 5.1) take the same tokens.

**`codecs`** is the stream's codecs string of RFC 6381 (`avc1.640028`,
`hvc1.2.4.L153.B0`, `av01.0.08M.10`, `mp4a.40.2`, `ec-3`, `dtsh`), 1 to
64 characters of `A–Z a–z 0–9 . + -` (`+` for VVC's
`vvc1.1.L51.CQA.O1+3`): no `/`, and only the codes registered for the
codec (no `ec+3`: Atmos is `objects`). It is the value
players test a stream against before playing it.

- **Required** when `codec` is `h264`, `hevc`, `av1` or `vp9` (video),
  or `aac` or `dts` (audio): there it carries the profile, level, tier or
  flavour players decide on. **Optional** for every other codec. Where
  it is a fixed code the registry gives (`ac-3`, `ec-3`, `fLaC`, `Opus`,
  `mlpa`), it may be left out; where its string carries a profile or a
  level (`mpeg4`'s `mp4v.20.<n>`, `vvc`, `ac4`, `mpegh_3d_audio`), it is
  sent when known.
- **The chroma format.** For `hevc` with a `general_profile_idc` of 4 or
  more, the constraint bytes are required; for `av1` and `vp9` on a
  stream that isn't 4:2:0, the long form is required. Elsewhere the
  profile tells it.
- **Tolerance.** Profile, level and (AV1, VP9) bit depth must be the
  stream's own, as must the parts the previous point requires. The rest
  may be inferred, and a consumer ignores it: H.264 constraint flags (the
  stream's, or zero); HEVC compatibility flags (the stream's, or only the
  profile's own bit); VP9 level (the stream's, or a guess from size and
  frame rate). HEVC tier `L` means "Main or unknown", `H` High: a
  producer that doesn't know the tier sends `L`. A viewer's server never
  refuses a copy for a mismatch in a part it ignores.
- **Whatever the container.** On a Matroska or MPEG-TS file, `codecs` is
  the string the stream would have in ISO BMFF.
- **Compared case-sensitively**, as RFC 6381 says. A consumer may also
  accept the lowercase form of a 4CC (`opus`, `flac`).
- **`codec` and `codecs` never disagree.** Decisions go by `codecs` when
  it is present.
- **One place per fact.** The `codecs` string is a standard and is sent
  whole, so some of its parts repeat other fields. Each fact has one
  authoritative field: bit depth is `bitDepth`; colour is `colour`;
  profile, level, tier and chroma are `codecs`. They must agree; when
  they don't, the viewer's server decides by the authoritative field and
  still hands `codecs` to players as it is.
- **Missing or malformed where required**: the stream or track is kept,
  `codecs` counts as absent, and the viewer's server treats the copy as
  one it can't play as it is (section 3.2).
- **Dolby Vision**, every profile: `codecs` is the base codec's string,
  with the base 4CC of the file's Dolby Vision entry (`dvh1` → `hvc1`,
  `dvhe` → `hev1`, `dav1` → `av01`, `dva1` → `avc1`, `dvav` → `avc3`);
  the `dv` object holds the rest (section 3.6). A consumer rebuilds the
  Dolby Vision string (`dvh1.08.06`) from that 4CC and `dv`. A viewer's
  server that can't decode Dolby Vision treats a copy whose
  `dv.compatibility` is 0 as one it can't play as it is.

**The other open lists**, each the registry's, with these examples:
`edition` (a copy); the audio and subtitle `role` (the DASH role
scheme's values, to which the registry points); segment `kind` (section
3.4); `extraType` (section 3.9); `objects` (`dolby-atmos`, `dts-x`);
`hdr` (section 3.6); `threeD` (section 3.2).

Test cases: `codecs` in
[vectors/v0/video.json](../../vectors/v0/video.json), what a viewer's
server reads from a stream's `codecs`.

### 3.6 Colour and dynamic range

- **`colour`**: `{"primaries", "transfer", "matrix", "fullRange"}`:
  `primaries`, `transfer` and `matrix` as ITU-T H.273 code points,
  integers from 0 to 255, each optional, and `fullRange`, a boolean
  (H.273's full-range flag), absent when unknown. The value 2
  ("unspecified") isn't sent: the member is left out. Example:
  `"colour":{"primaries":9,"transfer":16,"matrix":9}`.
- **`transfer`** is the preferred transfer: the value of the alternative
  transfer characteristics SEI when the stream has one (broadcast HLG
  often signals 14 in the VUI and 18 there), else the VUI's. When the
  bitstream and the container (`colr`, Matroska's colour elements)
  disagree, the bitstream wins.
- **The range follows from `transfer`**: 16 is PQ (HDR10's transfer), 18
  is HLG; 14 and 15 are standard range in BT.2020. A copy with no
  `colour` says nothing of its range.
- **`hdr`** lists only what sits on top of that range: enhancement layers
  and dynamic metadata, as the registry's tokens (`hdr10plus` today),
  sorted in byte order, each once, at most 8. HDR10 and HLG come from
  `colour`, Dolby Vision from `dv`: `hdr10`, `hlg` and `dv` aren't sent
  in `hdr`.
- **`dv`**: `{"profile": <n>, "level": <n>, "compatibility": <n>, "el":
  <b>}`, from the file's Dolby Vision configuration record, sent whenever
  the copy has Dolby Vision: the profile (0 to 127), the level (0 to 63),
  the base-layer compatibility ID (0 to 15), and whether an enhancement
  layer is present (`el`, optional). A Dolby Vision copy with no
  compatible base (profile 5, compatibility 0) carries the `colour` of
  its signal.

### 3.7 The order of copies

The owning server lists a title's copies in its own order of preference,
best first. Its first copy is the one `play` plays when the request
names none
([playback section 2.6](playback.md#26-choosing-the-copy-and-the-tracks)).

A copy says what it is, not how it reaches a viewer: the viewer's server,
which has every copy's facts and knows its device, chooses the copy and
its tracks, and asks for a conversion when it needs one
([playback section 2.7](playback.md#27-delivery)).

### 3.8 What the caller keeps of copies

Of `copies`, for films, episodes and extras:

- at most 16 copies, the first 16 kept. An entry that isn't an object is
  skipped alone. The caps of this list (16 copies, 32 audio tracks, 64
  subtitle tracks, 256 chapters, 32 segments, 8 `hdr` entries) count kept
  entries only: an entry skipped or dropped doesn't count, and the next
  good one takes its place;
- a copy is skipped alone when its `id` doesn't match
  `^[A-Za-z0-9_-]{1,64}$`, or repeats an earlier copy's; when its
  `durationMs` isn't an integer from 0 to 8,640,000,000 (100 days); when
  `mediaType` is missing or isn't a media type
  ([core section 8.5](core.md#85-unknown-values)); when `size` isn't an
  integer from 1 to 999,999,999,999,999; when `etag` doesn't match
  `^[A-Za-z0-9_-]{1,64}$`; or when `audioTracks` isn't an array;
- **open values** (`mediaType`, the codecs, `edition`, `role`, `format`,
  `objects`, segment `kind`, `hdr` entries, `threeD`) are kept as they
  are when they are well formed (tokens, or for `mediaType` a media
  type), known or not, never turned into another value; one that isn't
  counts as absent, and where the member is required (`mediaType`, a
  `codec`) its copy, its `video` or its track goes with it, as said here;
- `startMs` from 0 to `durationMs`; `endMs` above `startMs` (0 when
  `startMs` is absent) and at most `durationMs`; `parts` an integer from
  2 to 16; `previewLayout` as section 5.5 says: otherwise the member
  counts as absent;
- `video` counts as absent when it isn't an object, or when `codec`
  isn't a token, or `width` or `height` isn't an integer from 1 to
  100,000. Its other members, out of range or of the wrong type, count
  as absent alone: `codecs` out of its form (section 3.5); `sampleAspect`
  not `<w>:<h>` with each side 1 to 65,535 without leading zeros;
  `rotation` not 0, 90, 180 or 270; `bitDepth` not an integer from 1 to
  16; `fps` not `<num>/<den>`, each 1 to 1,000,000 without leading
  zeros, the fraction reduced (`"48000/2002"` and `"0/0"` count as
  absent, as does a number); `refFrames` not an integer from 1 to 16, or
  on a stream whose `codec` isn't `h264`; `interlaced` not a boolean;
- in `colour`, a `primaries`, `transfer` or `matrix` that isn't an
  integer from 0 to 255, or is 2, and a `fullRange` that isn't a boolean,
  count as absent alone; a `colour` left with no member counts as
  absent;
- in `hdr`, an entry that isn't a token is skipped alone, as is a
  repeat; the caller keeps the rest sorted in byte order, the first 8;
- `dv` is kept only with `profile`, `level` and `compatibility` integers
  in range; an `el` that isn't a boolean counts as absent alone;
- in `audioTracks`, at most 32 tracks; a track is skipped alone when its
  `id` isn't `a` and a number from 0 to 999 without leading zeros, or
  repeats an earlier track's, or when `codec` isn't a token. Its other
  members, out of range, of the wrong type, or out of their form, count
  as absent alone; a `language` that isn't a valid tag
  ([catalog section 3](catalog.md#3-titles): `fre`, `fr-ca`, `e`, `und`)
  counts as absent, and is never converted;
- in `subtitleTracks`, at most 64 tracks; a track is skipped alone when
  its `id` isn't `s` and a number from 0 to 999 without leading zeros, or
  repeats an earlier track's. A missing `format`, or one that isn't a
  token, counts as absent; the track stays. Its other members count as
  absent alone, as for audio;
- `codecs` missing where section 3.5 requires it: the stream or track is
  kept without it;
- `chapters` as section 3.4 says: in order, a chapter whose `startMs` is
  out of range or not after the one before (a repeated start) dropped,
  the first 256 kept, names made plain text and cut to 64 characters;
- `segments`: a segment whose `kind` isn't a token, or whose times are
  out of range or not in order (`startMs` not below `endMs`), dropped;
  the first 32 kept;
- other members of a copy, a stream or a track are ignored: among them
  those of earlier drafts (`name`, `container`, `bitrateKbps`, `added`,
  `previews`, a track's `title`, `profile`, `layout` and `fileIndex`,
  `codecTag`, `level`, `dv`'s `bl` and `rpu`, a segment's `final`).

A title left with no copy is skipped (section 3.1).

Test cases: `copies` in
[vectors/v0/video.json](../../vectors/v0/video.json), each with what the
caller keeps.

### 3.9 Extras

Trailers, featurettes and the like are shared under the title they
belong to, keyed under its key (section 4.6). An extra is a title whose
key is an extra's, with this member besides `copies`:

| Field | Type | Content |
|-------|------|---------|
| `extraType` | string | open (section 3.5): what the extra is: `trailer`, `featurette`, `behindthescenes`, `deletedscene`, `interview`, `scene`, `short`, `clip`, `themesong`, `themevideo`… |

- An extra is played with `play` like any title, and counts against the
  pairing's `streams` as any playback does.
- **An extra carries no name**: a viewer's server shows it by its
  `extraType` and a number. Extras come, as every title does, in the byte
  order of their keys
  ([catalog section 3.1](catalog.md#31-paging-and-ordering)), so that
  such a label stays the same between readings.
- **Theme songs and theme videos** (`themesong`, `themevideo`) are
  extras like the others: an owning server may offer them. The viewer's
  server decides whether and how to present them; each one it plays is a
  playback like any other.
- The caller keeps `extraType` when it is a token, known or not; an
  extra whose `extraType` is missing or isn't a token is skipped. A
  viewer's server shows a type it doesn't know as a plain extra. A
  `name` on an extra is ignored.
- A viewer's server should show an extra under its title only: while it
  holds no title of that peer under the key the extra's key starts with
  (section 4.6), it shows the extra nowhere.

Test cases: `extras` in
[vectors/v0/video.json](../../vectors/v0/video.json).

### 3.10 The description of a local title

A local title (section 4.5) is known to no public database, so the owner
sends a small description of it, for films and series only:

| Field | Type | Content |
|-------|------|---------|
| `name` | string | the title's name, at most 128 characters |
| `year` | integer, optional | its year, 1800 to 9,999 |
| `image` | boolean, optional | `true` when the owner shares one image of it, asked for with the catalog's `image` call ([catalog section 4.4](catalog.md#44-the-image-of-a-local-title-image)) |

- The caller makes `name` plain text and cuts it to 128 characters; a
  local film or series without a `name` left is skipped. A `year` out of
  range or not an integer, and an `image` that isn't a boolean, count as
  absent. `image: false` is kept as it is.
- A viewer's server should show the description as the owner's, not a
  public database's: it may say whose server named it. It must never send
  a local description on to another server, and must never look it up
  elsewhere.
- Seasons and episodes of a local series carry no description: their
  numbers name them.
- Metadata beyond keys stays out everywhere else: these members are
  ignored on any other title.

## 4. Title keys

Films and series are named by identifiers from TMDB, TheTVDB or the
owner, never by files, paths or the media server's item IDs. A video
title key has three parts, joined by `/`: the kind (`movie` or
`series`), the scheme, and the identifier, followed for seasons, episodes
and extras by their place under the title:

| Kind | Key | Example |
|------|-----|---------|
| film | `movie/tmdb/<id>` | `movie/tmdb/949` |
| series | `series/tmdb/<id>` | `series/tmdb/1399` |
| season | `series/tmdb/<id>/s<season>` | `series/tmdb/1399/s1` |
| episode, by TMDB episode ID | `series/tmdb/<id>/e<episode id>` | `series/tmdb/1399/e63056` |
| episode, by numbers | `series/tmdb/<id>/s<season>/e<episode>` | `series/tmdb/1399/s1/e2` |
| series, by TheTVDB | `series/tvdb/<id>` | `series/tvdb/81189` |
| season, by TheTVDB | `series/tvdb/<id>/s<season>` | `series/tvdb/81189/s1` |
| episode, by TheTVDB numbers | `series/tvdb/<id>/s<season>/e<episode>` | `series/tvdb/81189/s1/e2` |
| episode, by absolute number | `series/tvdb/<id>/a<number>` | `series/tvdb/81189/a27` |
| film, local | `movie/local/<opaque>` | `movie/local/q3Zp0Lw8` |
| series, local | `series/local/<opaque>` | `series/local/Hk2v9TnA` |
| season, local | `series/local/<opaque>/s<season>` | `series/local/Hk2v9TnA/s1` |
| episode, local | `series/local/<opaque>/s<season>/e<episode>` | `series/local/Hk2v9TnA/s1/e3` |
| extra | `<film, series or season key>/x/<opaque>` | `movie/tmdb/348/x/Tr7aQ1` |

- `<id>` is the TMDB or TheTVDB ID of the film or the series, in decimal,
  without leading zeros: 1 to 9,999,999,999.
- `<season>` is `0` or 1 to 4 digits without a leading zero; season 0
  holds specials.
- `<episode>` and `<number>` are `0` or `[1-9][0-9]{0,4}`. Episode 0 is
  valid wherever the scheme numbers one (some seasons have a pilot or
  prologue as 0).
- `<episode id>` is TMDB's ID of the episode, 1 to 10 digits, above 0,
  without a leading zero.
- `<opaque>` matches `^[A-Za-z0-9_-]{1,64}$` (sections 4.5 and 4.6).
- Keys are canonical: no leading zeros anywhere (`s1`, not `s01`). A key
  in another form is a different string, and the receiver skips it.
- A season is named by its series' key and its number, never by a
  database's season ID.
- `play` takes only films, episodes and extras (section 5).

**Which scheme.** A server should key a series, and with it its seasons,
episodes and extras, under TMDB whenever TMDB has it; under TheTVDB when
TMDB has no match and TheTVDB has; under `local` when neither has it.
Films are keyed under TMDB, or `local`. A receiver skips keys of a
scheme it doesn't know. The schemes are listed in the
[registry](../registry.md), with the forms of their keys and of their
`ids` values: a new scheme is a registry entry, not a new version.

A title of the `tmdb` or `tvdb` scheme the owning server can't place in
its scheme's numbering is left out (section 4.1).

### 4.1 Episode order

For a series keyed under TMDB, TMDB is MSFP's reference for episodes:
their identity and their order are TMDB's. An episode's key names it;
its `place` (section 3) says where TMDB puts it now. Season and episode
numbers are TMDB's: the `season_number` and `episode_number` TMDB gives
the episode in the series' own season list, not in an episode group.

- **An episode TMDB lists** is keyed by its TMDB episode ID
  (`series/tmdb/<id>/e<episode id>`), and carries `place`. The key never
  changes; when TMDB renumbers (a season split or merged, an episode
  moved), the owning server sends the new `place` under the same key: a
  change of the title ([catalog section 3.2](catalog.md#32-whole-list-or-changes)).
  A receiver places the episode by `place`, and moves it when `place`
  changes; what it keeps of the episode (watched state, say) follows the
  key.
- **An episode TMDB doesn't list (yet)**, in a season TMDB numbers, is
  keyed by its numbers (`series/tmdb/<id>/s<season>/e<episode>`); its
  numbers continue TMDB's numbering of that season, and it carries no
  `place`. A receiver matches it only against the same form. Once TMDB
  lists it, the owning server sends it under its episode ID instead: one
  title gone, another new.
- **An owning server whose files are numbered another way** (DVD,
  absolute, a broadcaster's order, TheTVDB's) maps them to TMDB's numbers
  before it sends `place`, within TMDB's limits (section 7). When it
  can't place an episode in TMDB's numbering, it leaves the episode out.

For a series keyed under TheTVDB, section 4.4 gives the numbering; for a
local one, the owner's numbers are its keys (section 4.5).

The order a server shows its people is its own business and never
appears in keys or `place`.

### 4.2 Several episodes in one copy

A copy that holds several episodes is listed once per episode, each under
its own key. Each of those titles carries `with`: the keys of the other
episodes the same copy holds, in byte order, at most 16. `play` of any of
them plays that copy (section 5.2).

A receiver may use `with` to show "episodes 1–2" as one entry, and to
mark all of them watched together. Each copy's `startMs` and `endMs`,
when they are sent, give the episode's span in the copy; its
`durationMs` is always the copy's.

### 4.3 Other identifiers

Episodes keyed under `tmdb` or `tvdb`, and series keyed under `tvdb`,
may carry `ids`: other identifiers of the same title, by scheme. Films
and series keyed under `tmdb` carry none: TMDB's lookup by key gives
them (its `external_ids`). Local titles, seasons and extras carry none
either. The schemes and their forms are the
[registry's](../registry.md#ids-schemes); those of video are:

| Scheme | Titles | Form | Example |
|--------|--------|------|---------|
| `imdb` | episodes; series of the `tvdb` scheme | `tt` and 7 to 10 digits | `tt0903747` |
| `tvdb` | episodes | 1 to 10 digits, no leading zero | `349232` |
| `tmdb` | series and episodes of the `tvdb` scheme | 1 to 10 digits, no leading zero | `1396` |

Example: `"ids":{"imdb":"tt1480055","tvdb":"3254641"}`.

- Each value is a string. Unknown schemes are ignored. A value that
  breaks its form is ignored, as is a scheme on a title the table
  doesn't give it.
- `ids` never replaces the key: two titles are the same when their keys
  are equal ([catalog section 4.1](catalog.md#41-same-title-same-key)).
  `ids` only helps a receiver find a title it holds under another
  identifier.
- A series' `imdb` names it; an episode's names the episode. An
  episode's `tvdb` is TheTVDB's episode ID. A series' `tmdb` is TMDB's
  series ID, sent once TMDB has the series; an episode's, TMDB's episode
  ID.
- **Across schemes.** A series one server keys `series/tvdb/<n>` with
  `ids.tmdb` *m*, and one another server keys `series/tmdb/<m>`, are the
  same show, as are `series/tmdb/<m>` and `series/tvdb/<n>` when TMDB's
  identifiers of *m* give TheTVDB's *n*: a viewer's server may show them
  as one, matching episodes by their own `ids` (`tvdb`, `tmdb` or
  `imdb`). They are still two titles, each played from its own key.

### 4.4 The tvdb scheme

For series TMDB has no match for (anime numbered by TheTVDB, often),
`series/tvdb/<id>`, with TheTVDB's series ID.

- Seasons and episodes are TheTVDB's aired order: the season and episode
  numbers TheTVDB gives in its default order
  (`series/tvdb/<id>/s<season>/e<episode>`).
- **Absolute numbers** (`series/tvdb/<id>/a<number>`) are TheTVDB's
  absolute order, for a series whose episodes the owning server knows
  only by their absolute number. An owning server keys an episode by its
  season and episode numbers whenever it knows them, and by its absolute
  number only otherwise, so that two servers holding the same episode key
  it alike as far as they can.
- Episodes of this scheme carry no `place`: their key is their place.
  Their `ids` may carry TheTVDB's episode ID, by which a viewer's server
  may match two forms of the same episode.
- An owning server that can place a series in TMDB should move it there:
  its titles under `tvdb` gone, new ones under `tmdb`. Until it does, it
  may say the series' TMDB ID in `ids` (section 4.3).

### 4.5 The local scheme

For titles no public database knows (home videos, personal recordings):
`movie/local/<opaque>`, `series/local/<opaque>`, its seasons and its
episodes by the owner's numbers.

- `<opaque>` follows the rule of library IDs
  ([catalog section 2.1](catalog.md#21-library-ids)): chosen by the
  owning server, stable, never made from (or computable from) its media
  server's IDs, a file, a path or a name without a key.
- A local title never matches another server's, even with the same
  `<opaque>`: two servers' local titles are always two titles.
- It carries the description of section 3.10, and no `ids`.
- **The owner chooses** whether to share local titles at all: off by
  default, it is allowed per pairing and per group. An owning server
  sends local titles only to a peer it allows them; a change of that
  choice is a change of what it offers
  ([catalog section 5](catalog.md#5-what-is-shared)).

Test cases: `localKeys` and `tvdbKeys` in
[vectors/v0/video.json](../../vectors/v0/video.json).

### 4.6 Extras' keys

An extra is keyed under the film, the series or the season it belongs
to, of any scheme: `movie/tmdb/348/x/<opaque>`,
`series/tmdb/1399/x/<opaque>`, `series/tmdb/1399/s1/x/<opaque>`.
`<opaque>` follows the rule of library IDs, as in section 4.5. Episodes
have no extras. An extra's key, without `/x/<opaque>`, is the key of the
title it belongs to.

## 5. Playing a film, an episode or an extra

`play` ([playback section 2](playback.md#2-play)) takes the key of a
film, an episode or an extra. Everything [playback](playback.md) says
holds, the delivery a request asks for included
([playback section 2.7](playback.md#27-delivery)); this section adds the
members and rules that only video has.

### 5.1 The request

Video adds no member to the request of
[playback section 2.1](playback.md#21-the-request) but one transcode
target. The request names the copy (`versionId`) and the audio track
(`audioTrack`) as playback says; it names a subtitle only to burn it in.
Which subtitles a viewer sees is the viewer's server's choice
(section 5.4).

**Transcode targets.** The `transcode` object and its targets
(`videoCodec`, `audioCodec`, `maxHeight`, `maxBitrateKbps`,
`audioChannels`, `range`) are
[playback section 2.7](playback.md#27-delivery)'s; the codecs take the
registry's tokens (section 3.5), or `copy`. As they concern the picture:
a target left out takes the baseline's value (`h264`, the copy's own
height, `range` `sdr`); a `videoCodec` of `copy` serves the video
unchanged, takes no `maxHeight`, ignores `range` and keeps Dolby Vision;
any other drops Dolby Vision, and `range` `keep` keeps the copy's
transfer and primaries, in 10 bits. Video adds one target:

| Field | Type | Content |
|-------|------|---------|
| `burnSubtitle` | string | `s<n>`: a subtitle track of the copy to burn into the picture, as a fallback for devices that can't draw it (ASS, say) |

The owning server burns that track in, or refuses with `409`
`delivery-refused` ([playback section 2.5](playback.md#25-refusals)) a
`burnSubtitle` that isn't a track of the copy, one it can't burn, or one
asked with a `videoCodec` of `copy`. A malformed `burnSubtitle` (not `s`
and a number from 0 to 999 without leading zeros) is refused (`400`).

**Without `delivery`**, the owning server serves the baseline every
viewer's server must play, in complete HLS, as
[playback section 2.7](playback.md#27-delivery) says. For a copy with
`video`: H.264 at most High profile and level 4.1, 8-bit 4:2:0,
progressive, SDR in BT.709 (H.273 `primaries`, `transfer`
and `matrix` 1); AAC-LC, at most 2 channels, at most 48 kHz. For a copy
with no `video` (an extra such as a theme song): the same AAC, audio
only. It may leave a stream unchanged only when it is already within
that. A viewer's server that wants anything else (the copy as it is, a
remux, other codecs) asks for it with `delivery`: it has every copy's
facts, and knows its device.

### 5.2 Several episodes in one copy

(Section 4.2.) `play` of any of the episodes a copy holds plays that
copy. Without `startPositionMs` (or with 0), the owning server starts at
the episode's beginning when it knows where that is (the copy's
`startMs`), else at the copy's beginning. A non-zero `startPositionMs`
is a position in the copy, as for any other title. The answer's
`positionMs` says where it started, in the copy, and the viewer's server
must start there. The copy's `endMs`, when it is sent, says where the
episode ends.

Progress is per episode key. The playback is of the episode whose key
`play` named; its `progress` and `stop` give `positionMs` as a position
in the copy, not in that episode, and the viewer's server keeps the
viewer's progress under that episode's key, with that position in the
copy.

### 5.3 The answer

Members added to the playback object of
[playback section 2.3](playback.md#23-the-answer). The viewer's server
checks and cleans every field, as listed.

| Field | Type | Content | What the viewer's server requires or does |
|-------|------|---------|-------------------------------------------|
| `subtitles` | array, optional | the subtitle tracks of the copy the owning server serves apart (below) | at most 64 entries; entries checked as below |
| `fontCount` | integer, optional | the number of fonts the copy's ASS tracks use (section 6.6); absent means 0 | 0 to 64; otherwise 0 |

The answer carries no URLs and repeats nothing the catalog says: the
copy's tracks, chapters and skip markers come with the copy
(sections 3.3 and 3.4), and the viewer's server builds every URL from the
playback's ticket (section 6.1).

A `subtitles` entry is `{"id": "s<n>", "form": "text" or "picture",
"ass": true}`, one per track the owning server serves apart:

- `form` `text`: the track is served as WebVTT, at `subtitles/<id>`;
  `picture`: as PGS, at `pictures/<id>`. `ass: true`, only with `text`,
  says it is also served as ASS, at `ass/<id>` (section 6.1).
- An entry whose `id` isn't a subtitle track of the copy in the
  catalog, or repeats an earlier entry's, or whose `form` is neither
  value, is dropped alone. An `ass` that isn't a boolean, or comes with
  `picture`, counts as absent.
- For a track in one of the formats section 5.4 lists, an entry whose
  `form` or `ass` isn't the table's is dropped alone: the table says how
  it is served.
- A track of the file with no entry reaches a viewer only in the copy
  played as it is, or burned in.

Test cases: `answers` in
[vectors/v0/video.json](../../vectors/v0/video.json), the members above
as the viewer's server reads them, with the URLs it builds.

### 5.4 Choosing the subtitles

Which subtitles a viewer sees is the viewer's server's choice, among the
catalog's tracks (section 3.3) and those served apart (section 5.3): the
owning server chooses none, and the answer names none. Subtitles are
burned into the picture only when the request asks for it
(`burnSubtitle`, section 5.1): they never make a playback transcode
otherwise.

**Forms known before `play`.** The owning server must serve a track of
these formats apart, in these forms, so that a viewer's server knows
them from the catalog:

| `format` | `form`, `ass` | Served as |
|----------|---------------|-----------|
| `subrip`, `webvtt`, `mov_text`, `ttml` | `text` | WebVTT (section 6.3) |
| `ass`, `ssa` | `text`, `ass: true` | WebVTT, its styling flattened (section 6.3), and ASS (section 6.4) |
| `hdmv_pgs_subtitle`, `dvd_subtitle`, `dvb_subtitle` | `picture` | PGS, DVD and DVB subtitles converted by the owning server (section 6.5) |

Only a track in another format, or with none, waits for the answer to
know its form: the owning server serves it apart when it can convert it
(as WebVTT for a text track, as PGS for a picture track), and leaves it
out of `subtitles` otherwise. A track beside the video (`external`)
isn't in the file: a viewer's server fetches it apart, whatever the
delivery.

**A copy played as it is** (`delivery` `file`, [playback section 2.7](playback.md#27-delivery))
carries its own tracks and fonts untouched: the rules of section 6 cover
converted streams and subtitles fetched apart.

### 5.5 Seek-bar previews

Trickplay: thumbnails of the video every `intervalMs`, laid out in a grid
on JPEG images called sheets. Each sheet holds `columns` × `rows` tiles,
each `tileWidth` × `tileHeight` pixels, read left to right, then top to
bottom. Tile *t* (from 0) shows the video at *t* × `intervalMs`, and is
tile *t* mod (`columns` × `rows`) of sheet ⌊*t* / (`columns` × `rows`)⌋.
The last sheet may hold fewer tiles, and may be shorter or narrower than
a full sheet.

A copy's `previewLayout` (section 3.2) gives the layout, in the catalog:

| Field | Type | Content | Bounds |
|-------|------|---------|--------|
| `tileWidth` | integer | a tile's width, pixels | 1 to 2,048 |
| `tileHeight` | integer | a tile's height, pixels | 1 to 2,048 |
| `columns` | integer | tiles across a sheet | 1 to 100 |
| `rows` | integer | tiles down a sheet | 1 to 100 |
| `intervalMs` | integer | milliseconds between two tiles | 100 to 3,600,000 |
| `tileCount` | integer | tiles in all | 1 to 1,000 × `columns` × `rows` |

Besides those bounds, a full sheet is at most 4,096 pixels each way:
`columns` × `tileWidth` ≤ 4,096 and `rows` × `tileHeight` ≤ 4,096. The
copy has ⌈`tileCount` / (`columns` × `rows`)⌉ sheets, at most 1,000. A
`previewLayout` that isn't an object, or with a member missing, of
another JSON type than this table says, or out of bounds, is dropped
alone: the copy has no previews. The layout is said once: no answer
repeats it.

- An owning server whose media server keeps one image per interval
  tiles them into sheets; it may send every *n*-th image, with
  `intervalMs` multiplied by *n*, to stay within the bounds.
- The owning server sends `previewLayout` only when it has previews for
  the copy, within these bounds. It sends one set, not one per width.
- Both sides may leave previews out: an owning server need not send
  `previewLayout`, and a viewer's server that offers no previews
  ignores it.
- **When sheets may be fetched.** A viewer's server never fetches
  previews for a whole library, nor in bulk for copies nobody opened.
  For a copy a person opens (its page, its chapter list, a playback, a
  scrub), how it fetches the sheets, all at once or one by one, is its
  choice. Showing chapter images (section 3.4) counts as opening.
- **Under which ticket.** During a live playback of the copy, the
  playback's ticket opens its sheets, with no `previews` call. Outside
  one, the viewer's server calls `previews` (section 5.6) once for the
  copy, and uses that ticket for all its viewers until it expires; it
  calls again only after it has expired, and makes at most one call per
  copy at a time. A `429` means no previews of that peer until
  `retryAfter`.
- **Caching** is recommended, keyed by the copy's `etag` and its
  `previewLayout`: a viewer's server keeps the sheets it fetched, and
  fetches none again while both hold.
- The viewer's server never hands the owning server's URLs to the viewer:
  it serves each sheet at an address of its own, fetched as section 6.2
  says.

The `previewLayout` cases are among the `copies` of
[vectors/v0/video.json](../../vectors/v0/video.json).

### 5.6 previews

`previews`, under the peer's base path, signed as every call is. The
viewer's server asks for a ticket to fetch a copy's seek-bar previews
outside a playback, for a copy someone opens (section 5.5). (A `local`
title's image comes with the catalog's own `image` call,
[catalog section 4.4](catalog.md#44-the-image-of-a-local-title-image).)

Request:

| Field | Type | Required | Content |
|-------|------|----------|---------|
| `title` | string | yes | the key of a film, an episode or an extra, as the owning server sent it in `titles` |
| `versionId` | string | yes | the copy's `id` |

Answer `200`:

| Field | Type | Content | What the viewer's server requires or does |
|-------|------|---------|-------------------------------------------|
| `ticket` | string | the ticket the copy's sheets are fetched under (section 6.1) | `^[A-Za-z0-9_-]{1,64}$` |
| `etag` | string | the copy's `etag`, as the owning server holds it now | `^[A-Za-z0-9_-]{1,64}$` |

- The ticket is made as every ticket is
  ([media section 1](media.md#1-the-urls)). It works for 10 minutes from
  the answer, for the copy's sheets only, and is no playback: it doesn't
  count against `streams`, starts no conversion, and gets no `progress`
  or `stop`.
- An answer whose `ticket` or `etag` is missing or out of its form is
  refused: no sheets. When `etag` differs from the catalog's, the
  viewer's server reads the title again before it fetches any sheet.
- The owning server answers `404` for a title or a copy it doesn't offer
  the caller, and for a copy without a `previewLayout` in the catalog;
  `400` for a malformed request: a body that isn't an I-JSON object, a
  `title` that isn't the canonical key of a film, an episode or an extra
  (section 4), or a `versionId` that isn't a copy ID's form
  (`^[A-Za-z0-9_-]{1,64}$`); `404` `not-supported` when `video` isn't
  on; and may refuse with `429` `rate-limited` when asked too often
  ([core section 8.3](core.md#83-limits-and-overload)).

Test cases: `previews` in
[vectors/v0/video.json](../../vectors/v0/video.json).

## 6. Media

What [media](media.md) says holds: the forms of the URLs below, how
they are fetched, under which tickets, their sizes and the types the
viewer's server relays them as are all media's
([media sections 1](media.md#1-the-urls), [2.4](media.md#24-bounds) and
[3.4](media.md#34-subtitles-fonts-sheets-and-images)). This section says
which URLs video's files have, and what the bytes must be. A copy served
as it is (`file`) is the media part's alone: its own tracks and fonts
come in it, untouched.

### 6.1 The URLs

Answers carry no URLs. The viewer's server builds every URL of video's
files from the owning server's stream root
([media section 1](media.md#1-the-urls)), a ticket and a fixed name, so
that it never fetches an address a peer wrote:

| What | URL, after `<stream root><ticket>/` | Under the ticket of |
|------|-------------------------------------|---------------------|
| a subtitle track served as WebVTT (`form` `text`) | `subtitles/<id>`, then `/<part>` or `/whole` | the playback |
| a subtitle track served as PGS (`form` `picture`) | `pictures/<id>`, then `/<part>` | the playback |
| a subtitle track served as ASS (`ass: true`) | `ass/<id>`, then `/<part>` or `/whole` | the playback |
| font *n* | `fonts/<n>`, *n* from 0 to `fontCount` − 1 | the playback |
| preview sheet *n* | `trickplay/<n>`, *n* below the copy's sheet count (section 5.5) | a playback of that copy, or `previews`' answer (section 5.6) |

`<id>` is the track's ID; `<part>` is as media section 1 says. `<n>` is
in decimal, without leading zeros: sheet 0 is `trickplay/0`, sheet 1
`trickplay/1`, and so on. The owning server serves exactly these names,
and answers `404` past a count (a font from `fontCount` on, a sheet from
the sheet count on) and for a track not served apart in that form.

### 6.2 What is fetched

Everything here is fetched as
[media section 3.4](media.md#34-subtitles-fonts-sheets-and-images) says:
read whole, with plain `GET`s and no `Range`, within the sizes of
[media section 2.4](media.md#24-bounds). What the answers are:

- **Text subtitles**, in stretches (`<part>`) or whole (`whole`). From
  `subtitles/<id>`, WebVTT in the form of section 6.3; from `ass/<id>`,
  ASS in the form of section 6.4, a stretch holding the script's header
  and the events that show during it.
- **Picture subtitles**, in stretches (`pictures/<id>/<part>`): PGS,
  checked as section 6.5 says. A stretch with no subtitles is a `200`
  with an empty body (no segments), and the viewer's server relays it as
  such.
- **Fonts** (`fonts/<n>`): a TrueType or OpenType file, checked as
  section 6.6 says.
- **Preview sheets** (`trickplay/<n>`, section 5.5): a JPEG no wider
  than `columns` × `tileWidth` and no taller than `rows` × `tileHeight`
  pixels of the catalog's `previewLayout`. The owning server should
  re-encode every sheet before sending it, and is recommended to keep the
  sheets it re-encoded. Under a playback's ticket, the fetches count as
  media requests for
  [media section 4](media.md#4-how-long-a-ticket-works).
- **A local title's image** (`image`): as
  [catalog section 4.4](catalog.md#44-the-image-of-a-local-title-image)
  says.

The viewer's server checks again everything it relays, as
[media section 3](media.md#3-what-is-fetched) says, and besides:

- WebVTT rebuilt as section 6.3 says, ASS as section 6.4 says, PGS
  checked whole as section 6.5 says, fonts as section 6.6 says; whatever
  fails is a failed fetch
  ([media section 2.3](media.md#23-statuses)), answered `404` to the
  viewer;
- a preview sheet is checked against the catalog's layout: its size from
  its header first, before decoding, to be a JPEG (by its content,
  whatever its `Content-Type`) within the size above; any other is
  refused. It is then decoded and re-encoded as JPEG, so that only its
  pixels reach the viewer, and served at the viewer's server's own route,
  to its own people only.

Test cases for sections 6.3 to 6.6 are in
[vectors/v0/media.json](../../vectors/v0/media.json): `webvtt`, `ass`,
`pgs` and `fonts`.

### 6.3 WebVTT

A WebVTT part is rebuilt, not filtered: only cue timings, the cue
settings that place a cue, plain cue text, and the italic, bold and
colour tags survive. The owning server applies these rules to what it
serves, and the viewer's server applies them again to what it relays.
The same rules hold for a whole track, with its own bounds (section 6.2).

**Reading.**

1. A part over 5 MiB (5,242,880 bytes) is refused; a whole track, over
   16 MiB (16,777,216 bytes) ([media section 2.4](media.md#24-bounds)).
2. Each carriage return and line feed pair becomes a line feed, then each
   remaining carriage return becomes a line feed. One byte order mark
   (U+FEFF) at the start is removed.
3. The text must start with `WEBVTT`; otherwise the part is refused.
4. The text is cut into blocks at each pair of line feeds (an empty line).
   A line of spaces is not empty. Line feeds at the start and end of a
   block are removed.
5. In each block, the first line that is a timing line, once white
   space (Unicode's `White_Space`) at both ends is removed, starts a
   cue. The lines before it (the header, a cue identifier) are dropped.
   A block without a timing line is dropped whole: the header, `NOTE`,
   `STYLE`, `REGION`, or anything malformed.
6. A timing line is: a timestamp, one or more spaces, tabs or form
   feeds, `-->`, one or more of them again, a timestamp, then optionally
   one of them and anything. A timestamp is
   `[<hours>:]<minutes>:<seconds>.<milliseconds>`: 1 to 3 digits of
   hours, exactly 2 of minutes, 2 of seconds, 3 of milliseconds. What
   follows the second timestamp is the cue's settings, read as below.
7. Every line after the timing line, up to the end of the block, is a cue
   text line, even one shaped like a timing line. Each is cleaned (below).
   A line with no text left after cleaning (tags alone don't count) is
   dropped, with its tags. A cue with no line left is dropped.
8. More than 50,000 cues left refuses the part; more than 200,000, a
   whole track.

**Cue settings.** The settings are split at spaces, tabs and form feeds.
A setting is kept when it is one of these, exactly, and the first of its
name; every other is dropped. The first of its name is its first
occurrence, valid or not: after an invalid `line:`, a later valid
`line:` is dropped too.

| Setting | Value |
|---------|-------|
| `line:` | a percentage, or an integer from −1,000 to 1,000; then optionally `,start`, `,center` or `,end` |
| `position:` | a percentage; then optionally `,line-left`, `,center` or `,line-right` |
| `size:` | a percentage |
| `align:` | `start`, `center`, `end`, `left` or `right` |

A percentage is 1 to 3 digits, optionally `.` and 1 to 3 digits, then
`%`, from 0 to 100. `region:` and `vertical:` are dropped.

**Cleaning a cue text line.**

1. Remove invalid UTF-8, U+FFFD, and the direction controls U+061C,
   U+200E, U+200F, U+202A to U+202E and U+2066 to U+2069.
2. Cut the line into tags and text. A tag starts at a `<` followed by a
   letter (any script), `/`, `!` or `?`, and ends at the next `>`. A tag
   with no `>` removes the rest of the line. A `<` followed by anything
   else is text (`a < b`, `<3`).
3. A tag is kept when it is exactly one of: `<i>`, `</i>`, `<b>`,
   `</b>`, `</c>`, or `<c.` followed by one or two class names joined by
   `.`, then `>`. A class name is one of the text colours `white`,
   `lime`, `cyan`, `red`, `yellow`, `magenta`, `blue`, `black`, or one of
   them after `bg_` (`<c.yellow>`, `<c.white.bg_black>`); two classes
   are one text colour and one `bg_` colour. Every other tag becomes one
   space.
4. In each piece of text: decode character references as HTML does in
   text: named (`&amp;`, `&nbsp;`…), decimal (`&#60;`) and hexadecimal
   (`&#x3c;`). Then do step 1 again, and turn anything shaped like a tag
   (step 2) into one space, so that `&lt;b&gt;` can't come back as a tag.
5. Turn each control character and each white space character
   (Unicode's `White_Space`) into a space. Collapse runs of spaces into
   one, across tags too: a run broken only by tags becomes one space,
   where the run starts, before the tags. Remove spaces at both ends of
   the line.
6. Cut the line's text to at most 500 bytes, not counting tags, without
   splitting a character, and keep at most 32 tags; tags past the cut are
   removed. Then remove spaces at its end.
7. Escape `&` as `&amp;`, `<` as `&lt;` and `>` as `&gt;` in the text.

**Balancing the tags.** Then, over the cue's lines in order: a start tag
opens; an end tag is kept only when the innermost open tag has its name
(`c` for any `<c.…>`), and closes it; otherwise it is removed, even when
a tag of that name is open further out. A start tag
while 8 are open is removed, with the end tag that would close it. At
the end of the cue, each tag still open is closed, innermost first.

So no other markup survives: not `<u>`, `<v>`, `<lang>`, `<ruby>` or
`<rt>`, nor classes other than the colours, nor annotations (a voice's
name, a language); their text does. There is no CSS, since `STYLE` blocks
are dropped and nothing else can hold any; the colour classes are
WebVTT's own, which players draw without a style sheet. A URL in cue text
stays as text, and nothing is ever fetched for a part. A timestamp tag
(`<00:00:01.000>`) starts with a digit: it isn't a tag under step 2, and
stays as escaped text.

**Writing.** The part is `WEBVTT` and a line feed, then for each cue, in
the order read: a line feed, the two timestamps as they were written,
joined by ` --> `, then the kept settings, each after one space, in the
order `line`, `position`, `size`, `align`, a line feed, and the cleaned
lines, each followed by a line feed. UTF-8, line feeds only, no byte
order mark. A part with no cue left is `WEBVTT` and a line feed: it is
valid.

Not checked: that minutes and seconds are below 60, that a cue ends after
it starts, that cues are in order, and how many lines a cue has. Players
handle those.

**From ASS.** An `ass` or `ssa` track served as WebVTT (`subtitles/<id>`) is converted to
WebVTT first by the owning server: each event of section 6.4 a cue, its
text without override blocks, `\N` and `\n` a new line, `\h` a space,
drawings left out; italics and bold may become `<i>` and `<b>`. Then it
is rebuilt as above.

### 6.4 ASS

An `ass` or `ssa` track served as ASS (`ass/<id>/<part>` or `ass/<id>/whole`, section 6.1) is ASS (Advanced
SubStation Alpha, script type `v4.00+`), rebuilt, not filtered: known
sections, known lines and known override tags only, anything else
dropped, sizes capped. The owning server applies these rules to what it
serves, converting an SSA script (`v4.00`) to ASS on the way and a
script in another encoding to UTF-8 first; the viewer's server applies
them again to what it relays.

A part holds the script's header (its script information and styles)
and the events that show during its stretch: those that end after its
start, 10 seconds early, and start before its end. A whole track holds
every event.

**Reading.**

1. A part over 5 MiB (5,242,880 bytes) is refused; a whole track, over
   16 MiB (16,777,216 bytes) ([media section 2.4](media.md#24-bounds)).
2. Line ends and the byte order mark as WebVTT's step 2 (section 6.3).
   The script is split into lines; white space at both ends of a line is
   removed. Empty lines, and lines starting with `;` or `!:`, are
   dropped.
3. The first line must be `[Script Info]`, exactly, in this case;
   otherwise the part is refused.
4. A line starting with `[` and ending with `]` opens a section. Known,
   compared without ASCII case: `[Script Info]`, `[V4+ Styles]`,
   `[V4 Styles]` and `[Events]`. A script is SSA when it has a
   `[V4 Styles]` section and no `[V4+ Styles]` one: its events then take
   SSA's default format. Every line of any other section
   (`[Fonts]`, `[Graphics]`, `[Aegisub Project Garbage]`…) is dropped:
   embedded fonts and pictures never come in a script.
5. In a known section, a line is a name, `:`, and a value; white space
   around the value is removed. A line of another shape, or of an
   unknown name, is dropped. The line names `Format`, `Style` and
   `Dialogue`, and the field names a `Format:` line gives, are compared exactly,
   case included; only section names and script information names are
   compared without ASCII case.
6. More than 1,024 styles keeps the first 1,024. More than 50,000 events
   left refuses the part; more than 200,000, a whole track.

**Script information.** Kept, each the first of its name, compared
without ASCII case, when its value is valid; every other line is dropped
(`Title`, `Original Script` and the like included). The first of its
name is its first occurrence: when it is invalid, the name is dropped,
and a later valid line of that name with it:

| Name | Value |
|------|-------|
| `PlayResX`, `PlayResY`, `LayoutResX`, `LayoutResY` | an integer from 1 to 16,384 |
| `WrapStyle` | `0`, `1`, `2` or `3` |
| `ScaledBorderAndShadow` | `yes` or `no` |
| `Collisions` | `Normal` or `Reverse` |
| `Kerning` | `yes` or `no` |
| `YCbCr Matrix` | `None`, `TV.601`, `PC.601`, `TV.709`, `PC.709`, `TV.FCC`, `PC.FCC`, `TV.240M` or `PC.240M` |

**Styles.** A `Format:` line names the fields of the `Style:` lines after
it; without one, the fields are those of the section's version (`[V4+
Styles]`, `[V4 Styles]`), in their standard order. A `Style:` line is
split at its commas into as many fields as the format names; another
count drops it. A field of an unknown name is ignored. Each field is
checked; a missing or bad value takes its default:

| Field | Value | Default |
|-------|-------|---------|
| `Name` | text, 1 to 64 bytes | required: none drops the style |
| `Fontname` | text, 1 to 64 bytes; a leading `@` kept | `Arial` |
| `Fontsize` | a number from 0 to 1,000 | `20` |
| `PrimaryColour`, `SecondaryColour`, `OutlineColour`, `BackColour` | a colour | `&H00FFFFFF`, `&H000000FF`, `&H00000000`, `&H00000000` |
| `Bold` | an integer from −1 to 1,000 | `0` |
| `Italic`, `Underline`, `StrikeOut` | `-1` or `0` (any other integer is `-1`) | `0` |
| `ScaleX`, `ScaleY` | a number from 0 to 1,000 | `100` |
| `Spacing` | a number from −1,000 to 1,000 | `0` |
| `Angle` | a number from −360 to 360 | `0` |
| `BorderStyle` | `1`, `3` or `4` | `1` |
| `Outline`, `Shadow` | a number from 0 to 100 | `2` |
| `Alignment` | `1` to `9` | `2` |
| `MarginL`, `MarginR`, `MarginV` | an integer from 0 to 16,384 | `10` |
| `Encoding` | an integer from 0 to 255 | `1` |

- *Text* is made plain: invalid UTF-8 and control characters removed,
  `,` removed. Two styles of the same name (compared exactly): the first
  is kept. More than 1,024 styles: the first 1,024 kept, counting kept
  styles only.
- A *number* is an optional `-`, 1 to 7 digits, then optionally `.` and 1
  to 3 digits. A *colour* is `&H` and 1 to 8 hexadecimal digits,
  optionally followed by `&`, or a decimal integer from 0 to 4,294,967,295
  (SSA writes them so); it is written `&H` and 8 uppercase digits.
- From SSA: `TertiaryColour` is `OutlineColour`; `AlphaLevel` is
  dropped; `Alignment` 1 to 3 stays, 5 to 7 becomes 7 to 9, 9 to 11
  becomes 4 to 6.

**Events.** A `Format:` line names the fields of the event lines, the
last always `Text`; without one, `Layer, Start, End, Style, Name,
MarginL, MarginR, MarginV, Effect, Text` (SSA: `Marked` for `Layer`).
Only `Dialogue:` lines are kept; `Comment:`, `Picture:`, `Sound:`,
`Movie:` and `Command:` are dropped. A line is split at its first commas,
one fewer than the format names, `Text` taking the rest.

- `Layer`: an integer from 0 to 1,000, else `0`; SSA's `Marked` becomes
  `0`.
- `Start`, `End`: `<hours>:<minutes>:<seconds>.<centiseconds>`, 1 to 3
  digits of hours, 2 of minutes, 2 of seconds, 2 of centiseconds; another
  form drops the event.
- `Style`: a kept style's name, else `Default`.
- `Name` (the speaker) and `Effect` are emptied.
- `MarginL`, `MarginR`, `MarginV`: an integer from 0 to 16,384, else `0`.
- `Text` is rebuilt (below). An event whose text is empty after it, or
  over 65,536 bytes, is dropped.

**Rebuilding an event's text.**

1. Remove invalid UTF-8, U+FFFD, the direction controls of section 6.3,
   and control characters (a tab becomes a space).
2. Outside braces, text is kept as it is, `\N`, `\n` and `\h` included.
3. A `{` opens an override block, which ends at the next `}`; a `{` with
   no `}` after it removes the rest of the text. In a block, what comes
   before the first `\` (a comment) is dropped, and the rest is cut at
   each `\` into tags, except inside a `\t(…)`'s parentheses, which stay
   with their `\t`. A tag is kept when its name is in the table below
   (the longest name that matches) and its arguments are valid; every
   other is dropped. A block left with no tag is dropped. A block is
   written `{`, its kept tags, `}`.
4. At most 256 blocks and 1,024 tags an event; past them, blocks are
   dropped. Each kept tag of a block counts once toward the 1,024, a
   `\t` with all its inner tags counting one.

| Tags | Arguments |
|------|-----------|
| `\b` | an integer from 0 to 1,000 |
| `\i`, `\u`, `\s` | `0` or `1` |
| `\bord`, `\xbord`, `\ybord`, `\be`, `\blur` | a number from 0 to 100 |
| `\shad`, `\xshad`, `\yshad` | a number from −100 to 100 |
| `\fn` | a font name: the text up to the next `\` or the block's end, made plain as a style's `Fontname`, 0 to 64 bytes |
| `\fs`, `\fscx`, `\fscy` | a number from 0 to 1,000 |
| `\fsp` | a number from −1,000 to 1,000 |
| `\frx`, `\fry`, `\frz`, `\fr` | a number from −3,600 to 3,600 |
| `\fax`, `\fay` | a number from −100 to 100 |
| `\fe` | an integer from 0 to 255 |
| `\c`, `\1c`, `\2c`, `\3c`, `\4c` | `&H`, 1 to 6 hexadecimal digits, `&` optional; written with 6 and `&` |
| `\alpha`, `\1a`, `\2a`, `\3a`, `\4a` | `&H`, 1 to 2 hexadecimal digits, `&` optional; written with 2 and `&` |
| `\an` | `1` to `9` |
| `\a` | SSA's 1 to 3, 5 to 7, 9 to 11; written as `\an`, converted as a style's `Alignment` |
| `\k`, `\K`, `\kf`, `\ko` | an integer from 0 to 100,000 (centiseconds) |
| `\q` | `0` to `3` |
| `\r` | nothing, or a kept style's name |
| `\pos(x,y)`, `\org(x,y)` | two numbers from −100,000 to 100,000 |
| `\move(x1,y1,x2,y2[,t1,t2])` | four such numbers, then optionally two integers from 0 to 3,600,000 (milliseconds) |
| `\fad(t1,t2)` | two integers from 0 to 3,600,000 |
| `\fade(a1,a2,a3,t1,t2,t3,t4)` | three integers from 0 to 255, then four from 0 to 3,600,000 |
| `\t([t1,t2,][accel,]tags)` | optionally two integers from 0 to 3,600,000, optionally a number from 0 to 100, then tags of this table other than `\t`, `\pos`, `\org`, `\move`, `\fad`, `\fade`, `\an`, `\a`, `\k`, `\K`, `\kf`, `\ko`, `\q`, `\r`, `\fn`, `\p` and `\pbo`, each checked; a `\t` left with no tag is dropped; at most 16 `\t` an event |
| `\clip(x1,y1,x2,y2)`, `\iclip(x1,y1,x2,y2)` | four numbers from −100,000 to 100,000 |
| `\clip([scale,]drawing)`, `\iclip([scale,]drawing)` | optionally an integer from 1 to 16, then a drawing (below); one with an empty drawing, once rebuilt, is dropped |
| `\p` | an integer from 0 to 16; above 0, the text up to the next `\p0` is a drawing (below) |
| `\pbo` | a number from −100,000 to 100,000 |

Arguments in parentheses are separated by commas, with white space around
them removed. A *number* is as in styles.

**Drawings.** A drawing is rebuilt from its commands: `m`, `n`, `l`, `b`,
`s`, `p` and `c`, and numbers from −100,000 to 100,000 in the form of
styles' numbers, separated by single spaces when written. Anything else
in it is dropped. At most 10,000 numbers a drawing; past them, the rest
of the drawing is dropped.

**Writing.** The part is UTF-8, line feeds only, no byte order mark:

```
[Script Info]
ScriptType: v4.00+
<the kept script information, in the order of its table>

[V4+ Styles]
Format: Name, Fontname, Fontsize, PrimaryColour, SecondaryColour, OutlineColour, BackColour, Bold, Italic, Underline, StrikeOut, ScaleX, ScaleY, Spacing, Angle, BorderStyle, Outline, Shadow, Alignment, MarginL, MarginR, MarginV, Encoding
Style: <each kept style, all fields in that order>

[Events]
Format: Layer, Start, End, Style, Name, MarginL, MarginR, MarginV, Effect, Text
Dialogue: <each kept event, in the order read>
```

A style named `Default` is always written, with every default above,
when no kept style has that name (exactly), first of the styles. A part with no event left is valid.

Not checked: that an event ends after it starts, that events are in
order, what a drawing draws, and how tags combine. A renderer must be
robust to those itself.

### 6.5 Picture subtitles

Picture subtitles are served as PGS: a `hdmv_pgs_subtitle` track as it
is, a `dvd_subtitle` or `dvb_subtitle` track (or another picture format
it can read) converted to PGS by the owning server, its pictures
and their times kept. A PGS part is checked whole, then served
unchanged: a player decodes and draws it. Checking means reading every
segment, and decoding every object's run-length data to count it, not to
draw it.

**The track.** A part is at most 64 MiB (67,108,864 bytes), made of
segments one after another, up to its exact end. An empty part, of no
bytes and no segments, is valid: it is the answer for a stretch with no
subtitles, the smallest there is (no display set). A segment is a
13-byte header and a payload:

| Bytes | Content |
|-------|---------|
| 0–1 | `PG` |
| 2–9 | presentation and decoding times (not checked) |
| 10 | the segment's type |
| 11–12 | the payload's size, big-endian |

A header cut short, another magic, or a payload past the end refuses the
part. So does any type but these five:

| Type | Segment | Checks on its payload |
|------|---------|-----------------------|
| `0x16` | presentation (PCS) | at least 11 bytes; the screen's width (bytes 0–1) and height (2–3) are each 1 to 4,096; byte 10 counts composition objects, each 8 bytes, or 16 when its byte 3 has bit `0x80` (cropped); all of them within the payload |
| `0x17` | window (WDS) | at least 1 byte; exactly 1 + 9 × byte 0 bytes |
| `0x14` | palette (PDS) | at least 2 bytes; 2 + 5 × *n* bytes |
| `0x15` | object (ODS) | below |
| `0x80` | end | empty |

**Objects.** An object segment is at least 4 bytes: the object's ID
(bytes 0–1), a version (byte 2, not checked), flags (byte 3).

- Bit `0x80` of the flags marks the object's first fragment. It is at
  least 11 bytes; bytes 7–8 are the width and 9–10 the height, each 1 to
  4,096; bytes 11 on are run-length data. Bytes 4–6 (the data's length)
  are not checked. A part holds at most 20,000 first fragments. A first
  fragment with an ID already open starts that object again.
- Without bit `0x80`, the segment continues an open object of the same ID;
  bytes 4 on are more data. No open object of that ID refuses the part.
- Bit `0x40` marks the last fragment (it may be the first too). The
  object's data, all its fragments together, must then fit its size, and
  the object is closed.

An object left open at the end of the part doesn't refuse it.

**Run-length data fits** when, read from the start:

- a byte other than 0 is one pixel;
- `0x00 0x00` ends a line: the next pixel is at the start of the next
  line; more than *height* line ends refuses it;
- `0x00` then a flag byte *F* is a run: its length is *F* & `0x3F`, or,
  when *F* has bit `0x40`, (*F* & `0x3F`) × 256 plus the next byte; when
  *F* has bit `0x80`, a colour byte follows;
- a run cut short by the end of the data refuses it;
- after each pixel or run, more than *width* pixels on the line refuses
  it.

Not checked: segments' times and order, that each display set ends with
an end segment, IDs across segments, positions against the screen,
palette entries and colours, lines shorter than *width* or fewer than
*height*. A player must be robust to those itself.

**Memory.** Nothing here makes a picture: the check keeps only the open
objects' data, which the part's 64 MiB bounds. A player that draws a
picture needs at most 4,096 × 4,096 pixels for it, and the screen as big.

### 6.6 Fonts

The fonts a copy's ASS tracks use (its file's attached fonts) are served
apart, for ASS fetched apart (`ass/<id>`); a copy played as it is
carries its own.

- An owning server serves at most 64 fonts a copy, and counts them in
  `fontCount` (section 5.3), those the copy's first ASS track uses
  first: each at most 10 MiB, and 64 MiB (67,108,864 bytes) in all. The
  font URLs run from `fonts/0` to `fonts/63`. It serves standard TrueType and OpenType files only: a font
  it holds in another form (a collection, WOFF) it converts or leaves
  out. Font collections (TTC) are left for later (section 8).
- **A viewer's server may refuse fonts entirely**: it then fetches none,
  and its players draw ASS with their own fonts.

**What is checked**, by both sides, before a font is served or relayed:

1. It is at most 10 MiB (10,485,760 bytes), and the copy's fonts the
   viewer's server has relayed in the playback, this one included, are
   at most 64 MiB in all; a font past that is refused.
2. Its first four bytes are `00 01 00 00` or `true` (`74 72 75 65`,
   Apple's older TrueType signature, which font engines read), both
   TrueType, or `OTTO` (OpenType with CFF). Any other (`ttcf`, `wOFF`,
   `wOF2`, `typ1`…) refuses it.
3. Its table count (bytes 4–5, big-endian) is 1 to 64, and its table
   directory (12 bytes, then 16 a table) is within the file.
4. Each table's tag is four bytes from `0x20` to `0x7E`, each tag once,
   and its offset and length lie within the file.
5. It has the tables `cmap`, `head`, `hhea`, `hmtx`, `maxp`, `name` and
   `post`, and either `glyf` and `loca` or `CFF ` or `CFF2`.

A font that fails is refused. A server that has a font sanitizer (one
that rebuilds a font's tables, as browsers do for web fonts) should pass
every font through it after these checks, and serve what it rebuilds.
The viewer's server serves a font at its own route, to its own people
only, `font/ttf` for `00 01 00 00` and `true`, and `font/otf` for
`OTTO`.

Test cases: `fontSets` in
[vectors/v0/video.json](../../vectors/v0/video.json), the bounds on one
copy's fonts; the checks of each font are `fonts` in
[vectors/v0/media.json](../../vectors/v0/media.json).

### 6.7 The image of a local title

A local film or series with `image: true` (section 3.10) has one image,
asked for with the catalog's `image` call and checked as
[catalog section 4.4](catalog.md#44-the-image-of-a-local-title-image)
says.

## 7. Security considerations

- **Public databases.** A title of the `tmdb` or `tvdb` scheme carries a
  key only: a viewer's server looks its names and pictures up at TMDB or
  TheTVDB. A compromised peer can make it look up a long title list;
  readers cap what they keep
  ([catalog section 6](catalog.md#6-security-considerations)).
  An owning server that looks up TMDB episode IDs (section 4.1) does so
  with its own TMDB access, within TMDB's limits, caching what it
  learns.
- **Local titles** carry a name from the owning server, and may have an
  image: text and pixels a peer chose, shown to the viewer's people.
  Names are plain text, bounded, and shown as the owner's; an image is
  checked and re-encoded as a sheet is. Local titles are off by default,
  shared per pairing and per group (section 4.5).
- **Copies describe files.** A copy's exact size and its tracks can tell
  which known release a file is. A peer offered the title can learn as
  much by playing it; an owning server that doesn't want a peer to know
  doesn't offer it the title. No copy, track or extra carries a name or
  a title, which media servers fill from file names and release groups,
  and no copy says when a person added it.
- **No address from a peer.** Answers carry no URLs: the viewer's server
  builds every URL from the stream root, a ticket and a fixed name
  (section 6.1), so a peer can't send it to fetch an address of its
  choosing.
- **No reading to describe.** A viewer's server never reads a peer's
  file to learn what it is (section 3.2): every read is a playback the
  owning server sees and counts.
- **Preview sheets and images are pictures from another server.** Check
  a sheet's byte size first, then its dimensions from its header against
  the layout, and only then decode it. Re-encode it, so nothing but
  pixels (no metadata, no trailing bytes, no second image) reaches a
  browser. The layout's bounds keep the number of fetches, and the
  memory a sheet takes, small: at most 1,000 sheets of 4,096 × 4,096
  pixels. Never give the browser the peer's URLs: they carry the ticket,
  and would let the peer see the viewer. The `previews` ticket lives 10
  minutes and opens one copy's sheets only.
- **Previews tell use.** A `previews` call outside a playback tells the
  owning server that someone on the viewer's server opened that copy,
  and the sheet numbers fetched tell roughly where in it. Fetching a
  copy's sheets together hides the position. Sheets are fetched only for
  copies someone opens, never for a whole library (section 5.5).
- **Hostile subtitles and fonts.** Rebuild WebVTT and ASS, check PGS and
  fonts, before relaying them (sections 6.3 to 6.6).
- **The risk that remains.** Subtitles and fonts from another server
  reach the viewer's devices, and are read there by subtitle renderers
  and font engines with a history of flaws. The rebuilding of WebVTT and
  ASS removes the known attack paths: markup, style sheets, embedded
  files, unknown tags, unbounded sizes. It doesn't remove all: a valid
  script or drawing can still reach a renderer's bug, and a font that
  passes section 6.6 can still reach a font engine's. A font sanitizer
  narrows that; refusing fonts entirely (section 6.6) closes it for
  fonts. A copy played as it is (`file`) is the file the owning server
  holds, its tracks and fonts untouched: its viewer's player meets it as
  it would a file of its own.

## 8. Open questions

Points 1 to 3 were settled by the owner on 2026-11-17, point 5 on
2026-11-19, point 4 by the revision of version 0 on 2026-11-22. Point 17
is open.
Where a proposal of 2026-11-16 left a point open, the text above follows
the proposal's own default.

1. **Series TMDB numbers differently** (proposal 5, open point 1). Series
   that exist in TMDB only as one long season while files use real
   seasons (or the reverse), common for anime: section 4.1 says "map, or
   leave out". Is leaving out acceptable, or should the numbers form be
   allowed with the owning server's numbering, marked as such?
   **Settled:** map, or leave out; to look at again if implementations
   show it leaves out too much (anime). Since the revision of version 0
   ([proposal 16](../../proposals/0016-version-0-revision.md)), series
   TMDB has no match for are keyed under TheTVDB (section 4.4).
2. **IMDb episode IDs** (proposal 5, open point 2). Section 4.3 lets an
   episode carry its own `imdb` ID. Should `ids` stay at film and series
   level instead?
   **Settled:** episodes keep their own `imdb` ID.
3. **The cap of 16 for `with`** (proposal 5, open point 3).
   **Settled:** 16, kept.
4. **Descriptions for video too**
   ([proposal 11](../../proposals/0011-music.md), open point 2;
   [music's open questions](music.md#9-open-questions)). Video titles
   carry keys only; optional names would let a viewer's server list a
   peer's titles without TMDB
   ([core open question 26](core.md#18-open-questions)).
   **Settled** by proposal 16: keys only for titles of a public
   database; a small owner-sent description (name, year, one image) for
   local titles only (section 3.10), and a short name for extras
   (dropped by proposal 17: extras carry none).

5. **Placing an episode keyed by its TMDB ID.** A receiver can't place
   such an episode from its key: neither TMDB nor a media server looks an
   episode up by its TMDB episode ID alone. Found while building the
   Jellyfin plugin's viewing side.
   **Settled:** the owning server sends `place`, TMDB's current season
   and episode numbers, required for those episodes (sections 3, 3.1 and
   4.1); the key stays the episode's identity when TMDB renumbers.

Choices confirmed by the owner on 2026-11-17, moved here from
[playback](playback.md#10-open-questions) and [media](media.md#6-open-questions)
(they may still change as implementations show):

6. **`startPositionMs` with several episodes in one copy** (section 5.2).
   Proposal 5 says where a playback of a later episode starts, but not how
   `startPositionMs` combines with it. This draft reads a non-zero
   `startPositionMs` as a position in the copy.
7. **Timestamp tags in WebVTT** stay as escaped text (section 6.3). Removing them would suit karaoke-style subtitles better;
   karaoke now comes as ASS (section 6.4).
8. **Checks left to the player**: WebVTT timings out of range or order,
   ASS events' times and drawings, and PGS fields beyond sizes (sections
   6.3 to 6.5). An open PGS object at the end of a part isn't refused.

**Choices this draft made, confirmed by the owner on 2026-11-17** (when the video rules were moved
out of `catalog`, `playback` and `media` on 2026-11-17):

9. **The subtitle vectors stay in
   [vectors/v0/media.json](../../vectors/v0/media.json)**, a file named
   for the part that held these rules before, rather than in a new file
   for `video`: WebVTT, PGS, and since the revision ASS and fonts.

**Decided by the owner on 2026-11-18.**

10. **What the picture looks like.** Version 0 first gave films and
    episodes a summary of their best copy (`video`, a stepped height and
    HDR formats) and their languages (`audio`, `subtitles`).
    **Replaced** by the revision of version 0 (proposal 16): `copies`
    describes each copy whole (sections 3.2 to 3.8), and is the one
    source of truth; the summary is gone. Its test cases, with the rest
    of what a caller keeps of a video title (section 3.1), are in a file
    of their own, [vectors/v0/video.json](../../vectors/v0/video.json);
    the subtitle cases stay where point 9 says.
11. **Platform neutrality** (sections 3, 4.1, 5.3, 5.4, 5.5 and 7): every
    subtitle track a copy holds is listed, with URLs for those the owning
    server can serve apart; episodes keyed by their TMDB episode ID;
    credits, markers without an end, BIF previews. With the rest of the
    check of Plex and Emby,
    [core open questions 43 and 44](core.md#18-open-questions).

**Decided by the owner on 2026-11-21 and 2026-11-22**, in the revision
of version 0 ([proposal 16](../../proposals/0016-version-0-revision.md),
absorbing [proposal 15](../../proposals/0015-copies-in-the-catalog.md)):

12. **Copies in the catalog** (sections 3.2 to 3.8): required for films,
    episodes and extras, at most 8, with a required core; the exact size;
    the length to the second and the bitrate to 500 kbit/s; names and
    track titles limited to labels. **Changed** by proposal 17 (point
    16): at most 16, exact values, no bitrate, no names or track titles.
13. **Identity beyond TMDB** (sections 4.4 and 4.5): the `tvdb` scheme,
    absolute numbers included, and the `local` scheme, off by default.
14. **Extras** (sections 3.9 and 4.6), without theme songs and videos.
    **Changed** by proposal 17 (point 16): themes are extras like the
    others, and extras carry no name.
15. **Styled subtitles, fonts and previews** (sections 5.6 and 6): ASS
    rebuilt, at most 8 fonts of 10 MiB, VobSub and DVB converted to PGS,
    WebVTT keeping italics, bold, colours and positions, burning in on
    request, whole text tracks, and `previews` before a first play.
    **Changed** by proposal 17 (point 16): 64 fonts of 10 MiB, 64 MiB in
    all; the preview layout in the catalog, and sheets fetched only for
    copies someone opens.

**Decided by the owner on 2026-11-22**, in
[proposal 17](../../proposals/0017-neutrality.md) (neutrality):

16. **Neutrality: facts in outside vocabularies, open lists, choice to
    the viewer** (sections 3.2 to 3.10, 4.3, 5.1 to 5.6, 6.1, 6.2 and 7):
    `mediaType` for the container; codec and subtitle tokens from the
    [registry](../registry.md); RFC 6381 `codecs`, required for `h264`,
    `hevc`, `av1`, `vp9`, `aac` and `dts`; colour as H.273 numbers with
    `fullRange`; `hdr` for enhancement layers only; `dv` without `bl` and
    `rpu`; `objects`, `refFrames`, `endMs`; `parts` as a count; track IDs
    from the stream index; exact values; open lists by the unknown-value
    rule of [core section 8.5](core.md#85-unknown-values); no names, track
    titles, bitrates, profiles, layouts or `added`; `ids` only where TMDB
    can't give them; answers without URLs; `previewLayout` in the catalog
    and previews fetched only for copies someone opens; the choice of
    copy, tracks and subtitles the viewer's server's, the recommended
    orders moved to [the notes](implementers.md#video); no capabilities;
    the baseline's picture; 16 copies.
17. **Font collections (TTC)** (section 6.6). A copy's fonts may come
    as a collection; `video` 0 serves single TrueType and OpenType files
    only. Collections wait until their checks are written.
