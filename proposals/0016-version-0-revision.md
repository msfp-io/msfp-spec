# Proposal 16: The revision of version 0

Status: Adopted (2026-11-22) as a whole, the unmarked points included;
being written into spec/v0.
Absorbs [proposal 15](0015-copies-in-the-catalog.md) (copies in the
catalog) and its settled points. Every choice marked "decided" is the
owner's, taken on 2026-11-21 and 2026-11-22.

Touches: [catalog](../spec/v0/catalog.md) (pages bounded by size; key
schemes), [video](../spec/v0/video.md) (copies, extras, the `local` and
`tvdb` schemes, styled subtitles, previews), [music](../spec/v0/music.md)
(units, descriptions), [playback](../spec/v0/playback.md) (delivery and
method, downloads, scan access, capabilities, restarts),
[media](../spec/v0/media.md) (files by byte ranges, complete HLS, whole
subtitle tracks, fonts), the vectors and the conformance suite. No new
part; `groups` and the core's pairing are untouched.

MSFP is version 0, a draft, with two implementations, both the owner's.
This revision is made in one batch, then applied once everywhere: the
spec, its vectors and conformance checks, msfp-go, msfp-dotnet, Overfin
and the Jellyfin plugin.

## Problem

MSFP's goal for a viewer's server is that a title from a paired server
appears and plays **exactly as a local title does**: same place, same
pages, same menus, same player, on any media server and any of its
clients. Version 0 was shaped by two implementations on Jellyfin, and
falls short of that in three ways.

**The evidence:**

- **An audit of the Jellyfin plugin** (2026-11-21) found 13 visible
  differences between a local title and a federated one in jellyfin-web:
  the duration, the track block, the media details and the chapters
  missing before a first play; seek-bar previews only from the second
  play; Download, "Copy stream URL" and Delete hidden; subtitles turned
  on; the quality menu capped; titles kept in libraries of their own.
- **A survey of six platforms and client groups** (Jellyfin, Emby, Plex,
  Kodi; Swiftfin, Findroid, Streamyfin, Android TV and Infuse; Navidrome,
  OpenSubsonic and Jellyfin's and Plex's music), each from its own
  documentation and source code, found the same pattern everywhere:
  **a federated title works everywhere only if it looks like a local
  file**, with its details known before play.
- **A test phase** (2026-11-21 and 22) checked it:
  - **Plex** (a throwaway server, through a logging file system) lists a
    file from 1 to 4 MiB of reads, but its nightly "extensive media
    analysis" reads every file whole, and corrupts the length when a
    read is refused.
  - **A Jellyfin spike** serving a federated title as a plain file, with
    its details known in advance and the playback started by the first
    read, behaved exactly like a local film: no background reads at
    scans and tasks, identical answers to direct-play clients, seeking,
    and a silent restart after the playback had ended.

The reports and test write-ups are kept with the owner's working notes,
outside this repository.

**The three ways version 0 falls short:**

1. **It doesn't describe copies.** A catalog says which title a server
   holds, not what its files are: no length, no container, no streams,
   no chapters, no versions. A media server builds its library before
   anyone plays; without these it can only fake an item, opened like
   live TV, with the visible differences above.
2. **It serves media only during a playback the owner shapes.** A
   viewer's server can't present a plain file address before play, can't
   ask for the file as it is, can't survive a long pause, can't allow a
   download, and can't let a media server like Plex read a file to list
   it.
3. **It covers less than real libraries hold.** Titles TMDB doesn't
   place (anime numbered by TVDB, home videos), extras (trailers,
   featurettes), styled subtitles (ASS, its fonts, DVD picture
   subtitles), and music as music servers keep it (editions, a song on
   several albums) are left out.

## Proposal

### 1. Copies in the catalog (video section 3)

A film, an episode or an extra (section 4) in `titles` **must** carry
`copies`: its copies, best first, in the order `play` would choose them,
**at most 8**. The summary members `audio`, `subtitles` and `video` are
**removed** (decided: one source of truth); a viewer's server works them
out from `copies` when it needs them.

Copy:

| Field | Type | Required | Content |
|-------|------|----------|---------|
| `id` | string | yes | the version ID `play` takes and gives |
| `name` | string | no | an edition or quality label ("4K HDR", "Director's cut"), **never a file name** (section 1.4) |
| `edition` | string | no | `theatrical`, `directors`, `extended`, `unrated`, `remastered`, `special`, `imax`, `other` |
| `durationMs` | integer | yes | its length, all its parts; for an episode sharing a copy (`with`), the episode's own length when known, the copy's otherwise |
| `startMs` | integer | no | for an episode sharing a copy: where it starts in the copy, when known |
| `container` | string | yes | from the names list (section 1.3) |
| `size` | integer | yes | its exact size in bytes (decided: exact, open point 1) |
| `etag` | string | yes | its identity: changes whenever its bytes change; 1 to 64 characters of `A–Z a–z 0–9 - _` |
| `parts` | array of part | no | for a copy made of several files: each part's `size` and `durationMs`, in order |
| `bitrateKbps` | integer | no | its overall bitrate, rounded down to a whole 500 (below 500: absent) |
| `video` | video stream | no | its picture |
| `audioTracks` | array of audio track | yes (may be empty) | at most 32 |
| `subtitleTracks` | array of subtitle track | no | at most 64 |
| `chapters` | array of chapter | no | at most 256 |
| `segments` | array of segment | no | its skip markers (moved here from `play`'s answer) |
| `previews` | boolean | no | `true`: seek-bar previews to fetch with `previews` (section 7.4) |
| `added` | string | no | the day the owner added it, `YYYY-MM-DD` |
| `threeD` | string | no | `sbs`, `tab`, `mvc`, absent for 2D |

The required core is `id`, `durationMs`, `container`, `size`, `etag` and
`audioTracks` with their languages: every owning server knows these, so
no title says nothing. Everything else is optional, sent when the media
server knows it.

#### 1.1 Streams and tracks

Video stream:

| Field | Type | Content |
|-------|------|---------|
| `codec` | string | from the names list |
| `codecTag` | string, optional | the sample entry or FourCC as the file says it (`avc1`, `hvc1`, `hev1`, `dvh1`, `dvhe`, `av01`), at most 8 characters |
| `profile` | string, optional | as ffmpeg names it (`main`, `main 10`, `high`), at most 32 characters |
| `level` | number, optional | the codec level |
| `width`, `height` | integer | exact pixels |
| `bitDepth` | integer, optional | 8, 10 or 12 |
| `hdr` | array of string, optional | `hdr10`, `hdr10plus`, `hlg`, `dv`; absent for standard range |
| `dv` | object, optional | with `dv` in `hdr`: `{profile, level, compatibility}` (Dolby Vision profile 5, 7, 8…, its level, its base-layer compatibility ID) |
| `fps` | number, optional | two decimals at most |
| `interlaced` | boolean, optional | |
| `colour` | object, optional | `{primaries, transfer, matrix, range}` as ffmpeg names them |

Audio track (the track of `play`'s answer, same `a<n>` IDs):

| Field | Type | Content |
|-------|------|---------|
| `id`, `language`, `channels`, `role`, `default`, `forced` | as today | |
| `fileIndex` | integer, optional | its stream's place in the file (0-based), when the copy is one file |
| `codec` | string | from the names list |
| `profile` | string, optional | `atmos`, `dtsx`, `dts-hd-ma`, `dts-hd-hra`, `dts-es`, `he-aac`, `lc`, or absent |
| `layout` | string, optional | the channel layout as ffmpeg names it (`5.1(side)`, `7.1`) |
| `sampleRate`, `bitDepth` | integer, optional | |
| `title` | string, optional | the track's title, limited (section 1.4) |
| `original` | boolean, optional | the title's original language |

Subtitle track (same `s<n>` IDs):

| Field | Type | Content |
|-------|------|---------|
| `id`, `language`, `default`, `forced`, `hearingImpaired` | as today | |
| `format` | string | its source format: `subrip`, `ass`, `ssa`, `webvtt`, `mov_text`, `ttml`, `pgs`, `vobsub`, `dvb`, `other` |
| `external` | boolean, optional | `true`: a file beside the video, not inside it |
| `fileIndex` | integer, optional | its stream's place in the file, when inside it |
| `title` | string, optional | limited (section 1.4) |

`label` is dropped from tracks: media servers name languages in their
own users' words. `pictures` is replaced by `format`.

Chapter: `{startMs, name}`, name at most 64 characters, in order of
start, a start not after the one before dropped (proposal 15 section 5).

Segment: `{kind, startMs, endMs, final}`: the kinds of today
(`intro`, `outro`, `recap`, `preview`, `commercial`); `final: true` on
the last credits marker (some media servers mark a title watched there).

#### 1.2 Rules

- **Media info comes from the catalog only.** A viewer's server must not
  read a peer's file to learn what it is (each read would start a
  playback on the owner). Tested: with the details known in advance,
  Jellyfin reads nothing outside playback.
- **A change of `copies`** (a copy added, replaced, changed: its `etag`)
  is a change of the title, sent again by a changes reading.
- **`play`'s answer and the catalog disagreeing** (the copy changed since
  the reading): the answer wins, and the viewer's server reads the title
  again.
- **Pages bounded by size** (proposal 15 section 4): a page holds at most
  5,000 titles and 3 MiB of JSON; `next` is set whenever titles remain.

#### 1.3 Names

`container`, video `codec` and audio `codec` take the values of the
capabilities lists (section 6), plus those a viewer's server must know of
to convert them (proposal 15 section 1: `avi`, `m2ts`, `mpegts`, `wmv`,
`flv`, `3gp`; `mpeg2video`, `mpeg4`, `vc1`, `wmv3`, `h263`, `theora`;
`dts`, `truehd`, `mp2`, `wmav2`, `pcm_s24le`, `pcm_bluray`); anything
else is `other`, which a viewer's server reads as "convert it".

#### 1.4 What names and titles may say

A copy's `name` and a track's `title` are where file names and release
groups would leak: media servers fall back to the file name for a version
with no label, and track titles often carry a release group. So:

- `name` is an edition or quality label, at most 32 characters; an
  owning server sends none rather than a file name;
- a track `title` is at most 32 characters, and an owning server drops a
  title that is only the language's name, or carries anything else than
  a description of the track ("Commentary", "SDH", "Signs & Songs",
  "Forced");
- both are made plain text.

### 2. Identity beyond TMDB (catalog section 4, video section 4)

**Decided:** two more key schemes.

- **`tvdb`**, for series and episodes TMDB has no match for: `series/tvdb/<id>`,
  `series/tvdb/<id>/s<n>/e<n>`, `series/tvdb/<id>/a<n>` (absolute
  numbering, for anime). A server keys a title under TMDB whenever it
  can, under TVDB otherwise; a viewer's server matches the same show
  across schemes through `ids`.
- **`local`**, for titles no public database knows (home videos, personal
  recordings, untagged music): `movie/local/<opaque>`,
  `series/local/<opaque>/…`, and the music equivalents (section 5). The
  opaque part follows the rule of library IDs (chosen by the owning
  server, not derived from its media server's IDs without a key). Such a
  title never matches another server's.
  - It carries a **small owner-sent description**: `name` (at most 128
    characters), `year`, and one image (`image`: fetched like a preview
    sheet, under a short ticket, JPEG or PNG, at most 1 MiB), marked as
    the owner's. Metadata beyond keys stays out everywhere else.
  - **The owner chooses** whether to share `local` titles at all (off by
    default), per pairing and per group.

### 3. Delivery: copies as files (playback section 2, media section 1)

**Decided** (tested on Jellyfin): a viewer's server can present a copy
as a plain file before anyone plays it, and read it by byte ranges.

- **`delivery`** in `play`'s request: `file` (the copy as it is, all its
  tracks, read by byte ranges), `remux` (into a container the request
  names), `transcode` (to the targets of section 6), or absent (the owner
  decides from the capabilities, as today). The owning server does what
  is asked or refuses (`409` `delivery-refused`, its own reasons:
  bandwidth, policy); the viewer's server then asks for something else.
- **A playback may start on the first read.** A viewer's server may
  start `play` when its media server or a client first reads the file
  address it presented, not before.
- **Restart.** When a playback has ended while its file was still being
  read (a long pause: the ticket's 2 idle minutes), a viewer's server may
  start a new playback of the same copy and go on reading at the same
  byte, **only if the `etag` is unchanged**. It sends a new `play`; the
  owning server treats it as any other.
- **One playback per viewer session**, not per title: two viewers of one
  title are two playbacks.
- **HLS complete from the first fetch.** An HLS playlist an owning server
  serves (a remux or a conversion) lists every segment from the first
  request, with `#EXT-X-PLAYLIST-TYPE:VOD` and `#EXT-X-ENDLIST`; a growing
  playlist is refused by the viewer's server. (A player treats a growing
  playlist as live: no total time, no seeking.)
- **Several audio tracks in HLS**, as `#EXT-X-MEDIA` renditions, when the
  request asks for them; switching tracks then needs no new playback.
- **HEAD** on a media URL is answered `405`, never `404` (Kodi reads a
  `404` on HEAD as a missing file).

### 4. Extras (video section 3)

**Decided:** trailers, featurettes and the like are shared under their
title, without theme songs and videos.

- Keyed under their title: `movie/tmdb/348/x/<opaque>`,
  `series/tmdb/1399/x/<opaque>`, `series/tmdb/1399/s1/x/<opaque>`.
- `kind` `extra`, with `extraType`: `trailer`, `featurette`,
  `behindthescenes`, `deletedscene`, `interview`, `scene`, `short`,
  `clip`.
- A short owner-sent `name` (at most 64 characters).
- `copies` like any title; played with `play` like any title, counting
  against `streams`.
- Theme songs and theme videos are left out: a media server plays them
  in the background at each page view, which would start a playback on
  the owning server each time.

### 5. Music (music sections 3 and 4)

**Decided:** music is keyed as music servers hold it.

- **Albums by edition** (MusicBrainz release), **tracks by their place on
  it** (MusicBrainz release track); the release group and the recording
  go in `ids`, so a viewer's server can group a release's editions and
  spot the same song on several albums. A song on an album and on a
  compilation is listed twice, once per album, with no hole in either.
- Added to an album: `releaseType` (`album`, `single`, `ep`,
  `compilation`, `soundtrack`, `live`, `remix`, `other`), `compilation`,
  `originalDate` and `date`, `genres` (at most 8, from tags, plain text),
  `sortName`.
- Added to a track copy (its `copies`, as for video): `codec`,
  `sampleRate`, `bitDepth`, `channels`, `bitrateKbps`, `size`, `etag`,
  `replayGain` (`{trackGain, trackPeak, albumGain, albumPeak}`), and
  lyrics availability (`lyrics: plain | synced`), fetched with a call
  shaped as OpenSubsonic's `structuredLyrics`.
- **Artist images**: from a public source by MusicBrainz ID, the viewer's
  server's choice (TheAudioDB, fanart.tv…); nothing in the protocol.
  Artists without one fall under the `local` description (section 2).
- **Gapless and quick skipping:** a `play` may name the next tracks of a
  queue (at most 5); the owning server prepares them under the same
  playback, which then counts once against `streams`.
- **Untagged libraries:** the `local` scheme, with ISRC and AcoustID in
  `ids` where known.

### 6. Capabilities and method (playback section 2.1, video section 5.1)

**Decided:** the viewer's server may say what it wants.

- **The capabilities lists grow** to the names list (section 1.3), plus
  `hdr10plus` and `dv` (with the Dolby Vision profiles a device plays), audio
  profiles (`atmos`, `dtsx`, `dts-hd-ma`…), `maxAudioSampleRate`,
  `maxAudioBitDepth`. Coarsened as today: listed values only, sorted.
- **`transcode` targets**, with `delivery: transcode`: `videoCodec`,
  `maxHeight`, `maxBitrateKbps`, `audioCodec`, `audioChannels`, from the
  same coarse lists. The owning server converts to exactly that, or
  refuses.
- Without `delivery`, the owning server decides from the capabilities, as
  today.

### 7. Subtitles, fonts and previews (video sections 5.4 to 6.4)

**Decided** for styled subtitles: (a). A file played as it is carries its
own tracks and fonts untouched; these rules cover converted streams and
subtitles fetched apart.

- **ASS and SSA** are served as ASS, rebuilt by the engine as WebVTT is
  today: known sections and known styling tags only, anything else
  dropped, sizes capped.
- **Fonts** a copy's ASS tracks use are served as standard TrueType or
  OpenType files only, at most 8 per copy and 10 MiB each; a viewer's
  server may refuse fonts entirely.
- **VobSub and DVB** are served as pictures, as PGS is today.
- **Plain WebVTT** keeps italics, bold, colours and positions.
- **Burned in**: a `transcode` request may ask the owner to burn one
  subtitle track into the picture, as a fallback for devices that can't
  draw ASS.
- **Whole tracks:** a text track may be fetched whole, not only in
  2-minute stretches, during a playback.
- **Previews before a first play** (proposal 15 section 6 (a), decided):
  a call, `previews`, names a title and a copy, and answers the
  `trickplay` object with sheet URLs under a ticket of its own, which
  lives 10 minutes and is no playback; the owning server may refuse it
  as too frequent (`429`).
- The spec states the remaining risk: subtitles and fonts from another
  server reach the viewer's devices; the engine's rebuilding removes the
  known attack paths, not all.

### 8. Downloads (playback sections 2 and 5)

**Decided:** downloads the owner allows, off by default, with a speed
cap.

- `limits` gains `downloads` (at once, 0 by default: none), `downloadsPerDay`
  and `downloadKbps`, a speed cap the owning server enforces on every
  download's reads, so that downloads never take all its bandwidth.
- A download is a `play` with `purpose: download`, with `delivery: file`
  or `transcode` to a size the request names. Its ticket lasts 24 hours
  without the 2-minute idle rule; it counts against `downloads`, not
  `streams`.
- `progress` and `stop` accept a past time (`at`), so that a viewer's
  progress while watching offline reaches the owner when the device
  syncs, as it would have during a playback.
- A viewer's server shows Download only for titles whose owner allows it.

### 9. Scan access, for media servers that read files to list them (playback section 2)

**Decided** (tested on Plex): a capped read access, outside playback,
for media servers without plugins that can only show real files (Plex,
through a file system).

- A `play` with `purpose: scan` and `delivery: file`: no stream used, no
  conversion, reads capped at **32 MiB per copy** and rate-limited by the
  owner; past the cap, reads are refused (`416`).
- Off by default; the owner allows it per pairing and per group, with a
  limit of scans a day.
- The spec says what a media server reading this way needs: Plex's
  "extensive media analysis" must be off for such a library (tested: it
  reads every file whole, and when refused, overwrites the length); a
  real playback reads under a `play` like any other.

## Alternatives considered

1. **One change at a time,** each through the whole chain (spec, engines,
   Overfin, the plugin). Rejected by the owner: each round costs a full
   pass and leaves the spec in between states.
2. **Remembering what the first play gave,** in the viewer's server, in
   place of copies in the catalog. Leaves the first play of every title
   different, and every implementation to rebuild it (proposal 15).
3. **Keeping the owner deciding alone** how a copy plays, with a richer
   device description only. Leaves clients that need a real file
   (Findroid, Infuse, Android TV's new player) without one.
4. **Leaving Plex aside** and adding scan access later. Would be another
   revision; decided against.
5. **Plain WebVTT only** for subtitles: anime signs and karaoke stay
   flattened. **Burning subtitles in** only: no switching them off.
6. **Placements per track** for music, keeping recordings as keys: every
   reader rebuilds albums.

## Cost

- **Specification:** catalog, video, music, playback, media rewritten in
  the places above; the vectors (copies kept and cleaned, names and
  titles limited, new keys, `delivery` requests, ASS rebuilding, HLS
  completeness); the conformance suite (a catalog's copies matching what
  `play` serves; a file read by byte ranges; a restart on the same
  `etag`; refusals).
- **msfp-go and msfp-dotnet:** the copy model and its cleaning, the host
  catalog describing copies (most of it from what `Playback.Describe`
  already gives), `delivery` and `purpose`, downloads and scan tickets
  and their limits, the speed cap, ASS and font rebuilding, `previews`,
  `local` and `tvdb` keys, music's new keys. Both engines' tests.
- **Overfin:** as an owning server, fill copies from its index (its index
  keeps per-item stream summaries today: to extend to every version and
  track, and episode lengths); downloads and scan switches in its panel;
  extras and `local` titles from its library. As a viewer, read copies
  for its title pages.
- **The Jellyfin plugin:** as an owning server, the same; as a viewer, the
  plain-file source the spike tested, one playback per viewer session,
  complete items, extras, downloads; its multi-file HLS made complete.

## Compatibility

None needed: version 0 is a draft, and both implementations are the
owner's. Every implementation moves to the revised version 0 at once, in
one release of each. Servers on the old version 0 and the new one don't
pair with each other.

## Open points

1. ~~**The exact size**~~ Decided 2026-11-22: **exact `size` in the
   catalog.** A file address needs it before play, and anyone who plays
   the copy gets the exact file anyway. `bitrateKbps` stays rounded.
2. ~~**Scan access's cap**~~ Decided 2026-11-22: **32 MiB per copy**, kept.
3. ~~**Music queue size**~~ Decided 2026-11-22: **5 next tracks**, kept.

## Recommendation

Adopt sections 1 to 9 (open points settled on 2026-11-22), then write the
revision into spec/v0 in one pass, with its vectors and conformance
checks, before any implementation starts.
