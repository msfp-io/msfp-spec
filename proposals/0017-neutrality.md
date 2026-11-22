# Proposal 17: Neutrality

Status: Adopted (2026-11-22); written into spec/v0 in one batch
with proposal 16's remaining work. Every choice below is the owner's,
taken on 2026-11-22, except those listed in "Points to confirm".

Amends [proposal 16](0016-version-0-revision.md) and reverses some of its
decisions (listed below).

Touches: [core](../spec/v0/core.md) (one rule for unknown values; 8.3,
8.4), [catalog](../spec/v0/catalog.md) (2, 4, 4.4, 5),
[video](../spec/v0/video.md) (3.2 to 3.10, 4.3, 5.1, 5.3, 5.4, 6.6),
[playback](../spec/v0/playback.md) (2.1 to 2.7, 5, 6, 8),
[media](../spec/v0/media.md) (1, 2.2, 4.2), [music](../spec/v0/music.md)
(1, 3, 3.3, 4, 5, 6), a new non-normative notes file, a new registry
file, the vectors and the conformance suite. No new part; signing,
pairing, relations and `groups` are untouched.

## Problem

The owner's principle (2026-11-22):

> We are adapting the protocol for everyone. What happens if a new
> player enters the game? We adapt again. It should not be our job to
> adapt. The implementations should do it.

Proposal 16 was shaped by a survey of six products, and it drifted
toward them. A review of spec/v0 after proposal 16 (161 rows; kept with
the owner's working notes, outside this repository) found about half the
rows universal, and about 60 that ask for a change, in three shapes:

1. **Closed lists MSFP keeps itself.** Codec and container names, HDR
   formats, editions, audio roles and profiles, segment kinds, extra
   types, release types, library kinds. Each new codec (VVC, ProRes,
   IAMF, AC-4) is a spec change, and is sent as `other` even to a viewer
   that plays it. An HDR format outside the four is sent as standard
   range: a wrong fact. Library kinds are Jellyfin's CollectionType list
   word for word; segment kinds are Jellyfin's MediaSegmentType list.
2. **Product text in normative sections.** About 20 paragraphs on
   Overfin, Jellyfin, Plex, Kodi and BIF; the reason for the `HEAD` rule
   (Kodi); the scan cap and its figures (Plex).
3. **Viewer policy fixed by the spec.** The order of copies, language
   matching, subtitle choice, and capabilities from which the owner
   chooses for the viewer, although the viewer's server has every copy's
   facts and knows its device.

Unknown values are also handled three ways: ignored in capabilities,
refused (`400`) in transcode targets, collapsed to `other` or dropped in
the catalog.

**The rule this proposal applies.** The protocol carries universal facts
in outside vocabularies (ffmpeg names, RFC 6381, ITU-T H.273, the DASH
role scheme, MusicBrainz, IANA media types) and generic mechanisms with
open lists. No product text in normative sections. Policy is a
recommended default, except the safety defaults the owner kept
normative. Implementations map their media server's names to these
vocabularies; the protocol doesn't map for them.

## What changes in proposal 16

Reversed or amended:

| Proposal 16 | Was | Now |
|-------------|-----|-----|
| Proposal 15 point 3, kept 2026-11-21 | `durationMs` to the second, `bitrateKbps` to a whole 500 (music 8), `fps` two decimals | **Dropped**: all exact (section 3.1) |
| Decision 8 (section 6), capabilities | the viewer may ask a method; capabilities stay, enriched, coarsened | **Dropped**: no capabilities; the viewer's server sends `delivery`; without it, the H.264/AAC HLS baseline (section 5) |
| Decision 6 (section 4), themes | theme songs and videos left out | **Allowed** as extras; the viewer's server decides whether and how to present them (section 4.3) |
| Decision 4 (section 7), fonts | at most 8 a copy, 10 MiB each | **64 a copy, 10 MiB each, 64 MiB a copy in all**; TTC later (section 6.2) |
| Proposal 15 point 1 (section 1), copies | at most 8 | **16**; tracks unchanged: 32 audio, 64 subtitles (section 3.4) |
| Decision 3 and open point 2 (section 9), scan cap | 32 MiB fixed; Plex text normative | **Announced by the owner** in `limits`, floor 4 MiB, recommended default 32 MiB; Plex text to the notes (section 6.4) |

Kept as decided: "off by default", normative, for `local` titles,
downloads and scans; owner-sent images for `local` titles only, JPEG or
PNG; exact `size`; the music queue of 5; ASS rebuilt by the engine; the
`image` call; lyrics as their own call (OpenSubsonic's shape); chapters
(256, names 64); downloads' speed cap and 24-hour ticket; complete VOD
HLS; file delivery, play on first read and restarts on the same `etag`;
the `tvdb` and `local` schemes.

Settled by the answers and not carried as items of their own: the
review's P-2 (open capability values: moot, capabilities go), P-7 (scan
cap: section 6.4), P-15 (image types: kept as they are), P-16 (defaults:
kept normative, numbers to guidance, section 6.5).

## Proposal

### 1. One rule for unknown values (core, new section 8.5; review P-4)

One rule, in the core, for every part:

- A **token** is 1 to 32 characters of `a–z 0–9 . _ -`. Fields this
  proposal calls open take tokens unless they say otherwise.
- **In a fact** (the catalog, `play`'s answer): an unknown token is
  **kept** as it is, never turned into `other`, never dropped. A value
  that isn't a token (wrong type, too long, other characters) is dropped
  alone, as a malformed member is today.
- **In a request**: an unknown token is ignored where the field is a
  hint, and answered `409` `delivery-refused` where it names what to
  serve (a remux container, a transcode codec). Only a malformed value
  gets `400`.
- A viewer's server that doesn't know a value of a copy treats the copy
  as one it can't play as it is: it asks for a `remux` or a `transcode`,
  or doesn't offer the copy.

`other` disappears from every list.

### 2. Product text out (new file spec/v0/implementers.md; review P-9)

The normative text names no product. One non-normative file,
`spec/v0/implementers.md`, takes every Overfin, Jellyfin, Plex, Kodi and
BIF paragraph (video 3.2, 3.4, 5.5, 6.3, 6.4, 6.5; media 2.4, 3.1;
playback 2.1, 2.6, 6, 6.3, 8; core 8.3, 8.4; music 1, 3.3, 5, 6), and
this advice:

- credits mapped to `outro`, a marker with no end ending at `durationMs`
  (video 3.4);
- finding TMDB numbers from IMDb or TVDB IDs (video 4.1);
- guessing audio roles from a track's title (playback 2.3);
- Jellyfin's transcodes, and spotting a remux that re-encodes (playback 6);
- suggested numbers, today "Overfin's" (core 8.3, playback 6; see
  section 6.5);
- music tag names per product, R128 to ReplayGain, gapless playback,
  Jellyfin's 100-ns ticks (music 1, 3.3, 5, 6);
- "media servers often give ISO 639-2/B" (catalog 3);
- trickplay as BIF files, sheets near 320 px (video 5.5);
- Plex's scans and "extensive media analysis" (playback 8);
- a section **"Decoding safely"**, which takes the JPEG DNL rule from
  catalog 4.4 and other decoder hygiene.

Album art: "from a public source by MusicBrainz ID, such as the Cover
Art Archive" (music 4), not "from the Cover Art Archive".

**`HEAD`** (media 2.2, core 8.4): the wire stays. Media URLs answer
`GET` only; any other method, `HEAD` included, gets `405` with
`Allow: GET`. The Kodi reason goes.

### 3. Copies: facts, exact and open (video 3; review Q-1, P-1, P-3, P-5, P-8, P-11, P-13)

#### 3.1 Exact values

- `durationMs`: the length in milliseconds as known; no rounding.
- `bitrateKbps`: as known, in kbit/s, for video and music copies alike.
- `fps` becomes a rational: the string `"<num>/<den>"`, as ffmpeg prints
  `avg_frame_rate` (`"24000/1001"`, `"25/1"`), each 1 to 1,000,000,
  reduced. A value not in that form is dropped.

#### 3.2 Names from ffmpeg, plus RFC 6381 (video 3.3, 3.5; music 3.3)

- `container`: the ffmpeg muxer name of the file's format, the name
  `ffmpeg -f` takes (`matroska`, `webm`, `mp4`, `mov`, `mpegts`, `avi`,
  `flac`, `ogg`), open.
- Video and audio `codec`: ffmpeg's `codec_name` (`h264`, `hevc`, `av1`,
  `vvc`, `prores`; `aac`, `eac3`, `truehd`, `dts`, `opus`, `pcm_s24le`),
  open.
- Subtitle `format`: ffmpeg's subtitle `codec_name` (`subrip`, `ass`,
  `webvtt`, `mov_text`, `ttml`, `hdmv_pgs_subtitle`, `dvd_subtitle`,
  `dvb_subtitle`, `dvb_teletext`, `eia_608`), open. So `pgs`, `vobsub`
  and `dvb` become ffmpeg's names. The rule that a track gets no URL for
  what the owner can't serve stays.
- New, optional, on the video stream and each audio track: `codecs`, the
  stream's RFC 6381 codecs string (`avc1.640028`, `hvc1.2.4.L153.B0`,
  `dvh1.08.06`, `mp4a.40.2`, `ec-3`), at most 64 characters of
  `A–Z a–z 0–9 . -`. It is what MSE, ExoPlayer and AVPlayer test against.
- The names table (video 3.5) and its two columns are dropped.
- Video and audio `profile`: as ffmpeg prints the stream's profile
  (`Main 10`, `High`, `LC`, `HE-AAC`, `DTS-HD MA`,
  `Dolby TrueHD + Dolby Atmos`), plain text, at most 64 characters, open;
  not a token. MSFP's audio profile list (`atmos`, `dtsx`…) is dropped.
- `codecTag` and `level` stay.

#### 3.3 HDR from the signal (video 3.3)

- `colour` (`primaries`, `transfer`, `matrix`, `range`, ITU-T H.273 names
  as ffmpeg prints them) is the base fact. HDR10 and HLG follow from
  `transfer` (`smpte2084`, `arib-std-b67`); a copy with no `colour` says
  nothing about its range.
- `hdr` keeps only enhancement layers and dynamic metadata, as open
  tokens: `hdr10plus`, `dv`, later ones. `hdr10` and `hlg` leave it. The
  fixed order goes; the array is sorted, each token once.
- `dv` widens to the Dolby configuration record: `profile` 0 to 127,
  `level` 0 to 63, `compatibility` 0 to 15, and new booleans `el`, `bl`,
  `rpu` (enhancement layer, base layer, RPU present).

#### 3.4 Other bounds and facts (video 3.2, 3.3)

- `copies`: at most **16**. `audioTracks` at most 32, `subtitleTracks` at
  most 64, unchanged.
- Video `bitDepth`: 1 to 16 (not only 8, 10, 12).
- New, optional, on the video stream: `sampleAspect`, the sample (pixel)
  aspect ratio as ffmpeg prints it (`"32:27"`), each side 1 to 65,535;
  absent means square pixels. `rotation`: the degrees clockwise the
  picture must be turned to show upright, `0`, `90`, `180` or `270`;
  absent means 0.

#### 3.5 Open small lists (video 3, catalog 2; review P-5)

Each becomes open, unknown values kept (section 1):

- library `kind` (catalog 2): today's values become examples; unknown
  kinds are kept, not turned into `""`;
- `edition` (video 3.2): today's values become examples;
- audio `role` (video 3.3): the values of the DASH role scheme
  (`urn:mpeg:dash:role:2011`: `main`, `alternate`, `supplementary`,
  `commentary`, `dub`, `description`…); an unknown role is never taken
  for the title's own sound;
- segment `kind` (video 3.4): unknown kinds kept; a viewer's server
  shows a generic "Skip" for a kind it doesn't know. `final` stays, its
  reason worded without a product;
- `extraType` (video 3.9): unknown types kept, not turned into `clip`,
  and shown generically.

#### 3.6 What names and titles may say (video 3.6; review P-8)

- Normative: a copy's `name` and a track's `title` are plain text, at
  most **64** characters (not 32). That is all a caller checks.
- Guidance for owning servers, as their own privacy choice: send no file
  name as `name`; drop a track title that is only the language's name or
  carries a release group. The engines' filters become hooks, with
  today's code as a default (engine work E-2, below).
- Where extras and `local` descriptions are shown (video 3.9, 3.10)
  becomes "should". One rule stays a must: a viewer's server never sends
  a `local` description on to another server and never looks it up
  elsewhere.

### 4. Choice is the viewer's (playback 2.2, 2.6; video 3.7, 5.4; review P-6)

#### 4.1 The normative minimum

- `copies` come in the **owner's order of preference**.
- A `versionId`, an `audioTrack` and a `subtitleTrack` given in `play`
  win.
- Otherwise the owning server chooses, and the answer says what plays
  (its `versionId` and tracks, as today).

#### 4.2 Recommended defaults

The best-first formula (greatest height normalised to 16:9, then HDR,
then more `hdr` formats), the copy chosen by `audioLanguage` (same tag,
then base language), the audio choice table and the subtitle table
(regular before forced, `forcedTrack` in the audio's language) stay in
the spec as recommended defaults, marked so. Their vectors stay, as
tests of the defaults, not of conformance.

#### 4.3 Themes (video 3.9; review Q-4)

`extraType` gains `themesong` and `themevideo`. An owning server may
offer them like any extra. The viewer's server decides whether and how
to present them; each one it plays is a playback like any other. The
spec's reason for leaving them out goes. Advice on media servers that
play themes in the background at each page view goes to the notes.

### 5. No capabilities; one baseline (playback 2.1, 2.7; video 5.1; music 5; review Q-2, P-10, P-12, P-13)

- The `capabilities` object goes from `play`'s request, with every
  capability list (`containers`, `audioCodecs`, `audioProfiles`,
  `videoCodecs`, `hdr`, `dvProfiles`, `subtitles`), the `max…` members,
  and the coarsening steps that existed for them. The owner learns less
  of the viewer's device.
- The viewer's server, which has every copy's facts, asks for what it
  wants with `delivery`: `file`, `remux` or `transcode`.
- **Without `delivery`**: the owning server serves the baseline every
  viewer's server must play: H.264 video and AAC audio in complete HLS
  for a film, an episode or an extra; AAC in HLS, audio only, for a
  track. It may play a copy as it is when the copy already meets it.
- **Transcode targets** (`videoCodec`, `audioCodec`, `maxHeight`,
  `maxBitrateKbps`, `audioChannels`, `burnSubtitle`) take ffmpeg names
  for codecs; an unknown or unsupported codec gets `409`
  `delivery-refused` (section 1). A target left out takes the baseline's
  value: `h264`, `aac`, the copy's own height, no cap, at most 2 channels.
  The targets' steps (`maxHeight`, `maxBitrateKbps` in 1,000s,
  `audioChannels` 2, 6 or 8) stay.
- **Remux** (playback 2.7): the `container` is `mpegts` or `mp4`
  (fragmented MP4), the two HLS segment formats. Any other container
  gets `409` `delivery-refused`. A remux is always served as complete
  HLS.
- **Audio renditions**: `audioRenditions` stays a request; there is no
  refusal. `play`'s answer gains `audioRenditions` (boolean): `true` when
  the playlist carries every audio track as a rendition, `false` when the
  owner made none. With `false`, switching tracks means a new playback.
- **No ceiling of its own** (playback 2.2): with no cap from the
  request, the owner or the pairing, there is none. The 120,000 kbit/s
  default goes.
- **Dead members dropped**: `subtitleStretchSeconds` (video 5.3),
  `versions` in `play`'s answer (the catalog's `copies` are the source),
  `info` (no defined contents), and the sentence on dropping `title` and
  `next` (playback 2.3). Unknown members are ignored anyway.
- **Playback ID** (playback 2.3, 3, 4): the common opaque-ID form,
  `^[A-Za-z0-9_-]{1,64}$`, not `^play_[a-z2-7]{26}$`. It stays
  unguessable (128 bits at least), as tickets are.

### 6. Limits and the owner's policy (playback 5, 6, 7, 8; media 1, 4.2; video 6.6)

#### 6.1 Limits (playback 5; review P-13)

`streams` and `downloads` range 0 to **1,000** (not 50), so a group pool
lent to many members fits. `transcodes` stays 0 to `streams`; music's
`audioStreams` and `audioTranscodes` follow the same bound.

#### 6.2 Fonts (video 6.6, media 1; review Q-5)

A copy's ASS tracks may use at most **64** fonts, each at most 10 MiB,
**64 MiB in all** a copy. The font URLs run from 0 to 63. TrueType and
OpenType only; font collections (TTC) are left for later, once their
checks are written (an open question in video 8).

#### 6.3 Images (catalog 4.4; review Q-9)

Unchanged: the `image` call serves `local` titles only, JPEG or PNG, at
most 1 MiB. The DNL rule moves to "Decoding safely" (section 2).

#### 6.4 Scan access (playback 8, media 4.2, core 8.4; review Q-3)

- `purpose: scan` stays as it is, with its `scansPerDay`.
- `limits` gains **`scanBytes`**: the bytes one scan ticket may read in
  all, 4,194,304 (4 MiB) to 1,073,741,824 (1 GiB). Absent, or out of
  range, a caller counts on the floor, 4 MiB. The recommended default
  for owners is 32 MiB.
- The owning server cuts and refuses reads past its `scanBytes` as it
  does today past 32 MiB (`206` ending at the cap, then `416`).
- The normative text says only what any media server reading files to
  list them needs: `scan` for reads to list a file, `play` for reads to
  watch it, and no whole-file analysis of a peer's copies. Every Plex
  sentence (1 to 4 MiB, "extensive media analysis") goes to the notes.

#### 6.5 Defaults (playback 5, 6; core 8.3; groups 9; review Q-7)

- Normative, unchanged: `local` titles, downloads and scans are **off by
  default** (`downloads` 0, `scansPerDay` 0, `local` not shared), and
  the owner allows them per pairing and per group.
- To the notes, as suggestions: 2 streams and 1 transcode; downloads 1
  at once, 10 a day, 20,000 kbit/s; scans 1,000 a day; the call rates of
  core 8.3; Overfin's 24 hours in playback 6.3.

### 7. Music (music 3, 3.3; review Q-8, P-11)

- `releaseType` goes. An album carries `primaryType` (string) and
  `secondaryTypes` (array, at most 8), MusicBrainz's release-group types
  as MusicBrainz names them, lowercased (`album`, `single`, `ep`,
  `broadcast`, `other`; `compilation`, `soundtrack`, `live`, `remix`,
  `dj-mix`, `audio drama`, `mixtape/street`…), open, each 1 to 32
  characters of `a–z 0–9 space . _ - /`. The viewer's server maps them
  to its own types. `compilation` (the tag) stays.
- A track may carry its own `date`, `originalDate` and `genres` (at most
  8, as on albums), optional: compilations have them per track.
- A track copy's `container` and `codec` take ffmpeg names (section 3.2);
  `bitrateKbps` and `durationMs` exact (section 3.1).

### 8. Identity schemes in a registry (catalog 4, video 4.3, music 3.1; review P-14)

- A new file, `spec/registry.md`, lists the key schemes (`tmdb`, `tvdb`,
  `mb`, `local`: which kinds each keys, where its key forms are defined)
  and the `ids` schemes (`imdb`, `tvdb`, `tmdb`, `mbreleasegroup`,
  `mbrecording`, `isrc`, `acoustid`), with each value's form. A new
  scheme is a registry entry, not a new protocol version. The parts keep
  the key forms of the schemes they define.
- Unknown key schemes are skipped, unknown `ids` schemes ignored, as
  today.
- `tmdb` joins `ids`, so a `tvdb`-keyed series can say its TMDB ID once
  TMDB has it.
- One title may have two keys on two servers (`tmdb` on one, `tvdb` on
  the other); `ids` join them. The order "TMDB when it can, TVDB
  otherwise" stays as a "should".

## What it doesn't change

The core's signing, pairing, relations and key rotation; groups; tickets
and their lifetimes; byte ranges and the file delivery; play on first
read and restarts on the same `etag`; complete VOD HLS and its tag
subset; one playback per viewer session; the WebVTT, ASS, PGS and font
rebuilds; image re-encoding; exact `size` and `etag`; `parts`, `with`
and `startMs`; chapters; the music queue of 5; lyrics; downloads (24-hour
ticket, speed cap, late reports); the `previews` call; the `tvdb` and
`local` schemes and the `local` description; catalog paging.

## Alternatives considered

1. **Keep MSFP's lists and grow them per release.** Each new codec or
   product would cost a spec change and a release of every
   implementation: the cost the principle rejects.
2. **Open the capability values only** (review P-2). Keeps the owner
   choosing for a device it can't see, and the coarsening. Rejected with
   decision 8.
3. **IANA media types for containers.** Fewer values, but no names for
   many containers (`matroska` has one, `mpegts` has `video/mp2t`, many
   have none). ffmpeg's names cover every file a media server reads.

## Costs

- **Spec and vectors.** Text in the sections listed under Touches; the
  new `implementers.md` and `registry.md`. Vectors: the names table's
  cases replaced by ffmpeg names and unknown values kept; the
  capabilities and coarsening cases removed; rounding cases made exact;
  `hdr`, `dv`, `fps`, `sampleAspect`, `rotation`, 16 copies, 64 fonts,
  `scanBytes`, `audioRenditions` in the answer, new limits ranges,
  opaque playback IDs, `primaryType`/`secondaryTypes`; the choice tables
  marked as defaults. Med to high: the names table touches many vectors.
- **Conformance suite** (branch `conformance-wip`). Drop the capability
  and coarsening checks; make the copy, language and subtitle choice
  checks informative; add checks that unknown tokens are kept in facts
  and refused with `409` in delivery requests, that a remux names
  `mpegts` or `mp4`, that the answer's `audioRenditions` matches the
  playlist, and that scans stop at the announced `scanBytes`. Med.
- **msfp-go** (branch `revision`). Rework on the finished branch: drop
  the capability model and coarsening; drop the names mapping and the
  `other` collapse; exact values; the token rule; open lists; `hdr`,
  `dv`, new stream fields; 16 copies, 64 fonts and 64 MiB; `scanBytes`;
  `audioRenditions` in the answer; opaque playback IDs; music types.
  The copy-name and track-title filters become hooks (review E-2); the
  `HEAD` rule only for the engine's own tickets (E-3); the choice code
  kept as exported defaults. Med to high.
- **msfp-dotnet** (branch `revision`). The same as msfp-go, plus no
  ffprobe-names helper to write (review G2 no longer needed). Med to high.
- **Overfin.** As an owning server: send its media server's names as
  ffmpeg names, exact values, the new stream facts, up to 16 copies;
  set `scanBytes`; offer themes as extras behind an operator switch; the
  answer's `audioRenditions` (`false` until it builds renditions). As a
  viewer's server: always choose the copy and send `delivery`; keep
  unknown values; read the open lists. Most of the rest is proposal 16's
  migration (review A-1, A-2, A-10). Med.
- **Jellyfin plugin.** As an owning server: the same, its `NameOrOther`
  mapping removed, `audioRenditions: false` (Jellyfin makes no HLS audio
  renditions). As a viewer's server: choose `delivery` from Jellyfin's
  device profile, replacing the "Everything" capabilities; map open
  values to Jellyfin's own (unknown segment kinds to a plain skip); theme
  extras as Jellyfin theme songs and videos, behind a setting. Med; most
  of the rest is proposal 16's migration (review A-1 to A-4).

The engine work the review lists beyond this proposal (a `RemoteFile`
helper, .NET's headers and image encoder, peer-catalog helpers, API
cleanup) is not protocol; it goes in the same engine release.

## Compatibility

None needed: version 0 is a draft and both implementations are the
owner's. This proposal and proposal 16 reach every implementation in one
release.

## Points to confirm

Details the owner's answers leave open, with the choice made here:

1. **`fps`**: a rational string `"24000/1001"`, as ffmpeg prints it
   (not 3 decimals, not two integers).
2. **`container` names**: the ffmpeg muxer name (`ffmpeg -f`), which
   tells `mp4` from `mov`; not ffprobe's comma list of demuxer names.
3. **Remux containers** written as ffmpeg names: `mpegts` and `mp4`.
4. **Upper bound of `streams` and `downloads`**: 1,000 (to confirm).
5. **`scanBytes`**: the field's name, its range up to 1 GiB, and 4 MiB
   when absent.
6. **Audio `profile`**: ffmpeg's printed profile (plain text, 64), with
   no separate `objects` flag for Atmos and DTS:X.
7. **Transcode targets left out** take the baseline's values (`h264`,
   `aac`, at most 2 channels); the targets' steps are kept.
8. **New field names**: `codecs`, `sampleAspect`, `rotation`, `el`,
   `bl`, `rpu`, `primaryType`, `secondaryTypes`, `themesong`,
   `themevideo`, and `audioRenditions` in the answer.
9. **MusicBrainz types** lowercased, with spaces and `/` allowed.
10. **File names and places**: `spec/v0/implementers.md` for the notes,
    `spec/registry.md` for the schemes, the unknown-values rule as core
    section 8.5.
