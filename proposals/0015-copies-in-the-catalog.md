# Proposal 15: Copies in the catalog

Status: Proposed (2026-11-21), under review: open points 1 to 5 and 7
settled the same day, 6 open. On hold since 2026-11-21: it becomes part
of one revision of the protocol, prepared from a survey of what other
platforms need (Jellyfin, Emby, Plex, Kodi, Infuse, Swiftfin,
Navidrome/Subsonic), applied everywhere at once.
Touches: [video section 3](../spec/v0/video.md#3-titles) (a new member,
`copies`, with chapters), [catalog section 3.1](../spec/v0/catalog.md#31-paging-and-ordering)
(pages bounded by size too), [video section 5.5](../spec/v0/video.md#55-seek-bar-previews)
(previews before a first play: a new call, `previews`, if section 6 (a) is
chosen), the test vectors and the conformance suite. No new part.

A film or an episode in `titles` would describe its copies the way a
media server needs them to build a library: each copy's length,
container, picture and tracks. Today a viewer's server learns those
only by playing the title.

## Problem

A video title in `titles` says which title it is (its key, its place, its
other IDs) and, of its copies, only a summary: the audio and subtitle
languages and the best copy's height and HDR
([video section 3](../spec/v0/video.md#3-titles)). Everything else a
media server keeps about a file arrives with `play`
([playback section 2.3](../spec/v0/playback.md#23-the-answer)): the
length (`durationMs`), the versions, the audio and subtitle tracks, and
how the stream is served (`info`: container, codecs).

A media server builds its library before anyone plays anything. The
evidence, from the second implementation (the Jellyfin plugin's
TECHNICAL.md and DESIGN-NOTES.md, and its code, checked 2026-11-21):

- **No length.** A peer's film shows no running time until it was played
  once. Jellyfin marks an item with no length as watched whenever its
  playback stops, at any position: the plugin's 0.5.1 had to fix it by
  keeping the length the first `play` gave (`KeepRuntimeAsync`).
- **No codecs, no container.** Jellyfin decides between playing a file
  as it is and converting it from the item's streams, when a client asks
  how to play it, before anything is opened. A peer's title has no
  streams then, so the plugin makes it a source that must be *opened*
  first, as live TV is: opening calls `play`, then the plugin has
  Jellyfin's ffmpeg read the stream through its relay to learn what it
  is. That one choice brings most of the plugin's workarounds:
  - Jellyfin for Kodi treats every source that must be opened as a live
    channel: no total time, no seeking, one audio track;
  - Jellyfin's own reading of such a source keeps one audio track, so
    the plugin reads it itself;
  - changing the audio track while playing needs an MVC filter that
    reads a Jellyfin request type by its members' names, to recheck
    with every Jellyfin release;
  - seek-bar previews show only because the saved source is marked as
    not remote.
- **Versions unknown.** A title held in several copies (1080p and 4K, a
  director's cut) is one item with one source until played; the versions
  arrive in `play`'s answer, too late for the media server's version
  picker.
- **Tracks unknown.** A track can't be chosen before Play by its ID: the
  plugin passes the chosen track's language instead, from the catalog's
  `audio` list (TECHNICAL.md, "Switching the audio").

Any other media server keeping a peer's titles in its own library meets
the same gap. Overfin as a viewer shows a peer's title from its catalog
too, and can't show its length.

`play` is not a way around it: each `play` starts a playback on the
owning server, counts against the pairing's `streams`, and may start a
transcode there ([playback section 5](../spec/v0/playback.md#5-limits)).
Playing every title once to learn it is out of the question.

## Proposal

### 1. `copies`

A film or an episode in `titles` may carry `copies`, its copies, best
first, in the order `play` would choose them
([video section 3.2](../spec/v0/video.md#32-the-picture), "The best
copy"). An owning server should send it.

| Field | Type | Content |
|-------|------|---------|
| `copies` | array of copy, optional | films and episodes only: the copies, best first; at most 8 |

Copy:

| Field | Type | Content |
|-------|------|---------|
| `id` | string | the copy's version ID: the same `versionId` `play` takes and gives ([playback section 2.3](../spec/v0/playback.md#23-the-answer)); with one copy, may be absent |
| `name` | string, optional | its name, as `versions[].name` gives it ("4K HDR", "Director's cut") |
| `durationMs` | integer | its length, all its parts; for an episode in a copy holding several (`with`), the episode's own length when the owning server knows it, the copy's otherwise |
| `startMs` | integer, optional | for an episode in a copy holding several: where the episode starts in the copy, when the owning server knows it (as `play` starts there, [video section 5.2](../spec/v0/video.md#52-several-episodes-in-one-copy)); absent otherwise |
| `container` | string | its container, from the list below |
| `bitrateKbps` | integer, optional | its overall bitrate, rounded (section 3) |
| `video` | stream, optional | its picture (below); absent for a copy whose picture isn't known |
| `audioTracks` | array of track, optional | its audio tracks, at most 32 |
| `subtitleTracks` | array of track, optional | the subtitle tracks `play` would list for it ([video section 5.4](../spec/v0/video.md#54-choosing-the-subtitles)), at most 64 |
| `chapters` | array of chapter, optional | its chapters, in order of their start (section 5), at most 256 |
| `previews` | boolean, optional | `true`: the owning server has seek-bar previews of it, to fetch with `previews` (section 6 (a)) |

Video stream:

| Field | Type | Content |
|-------|------|---------|
| `codec` | string | from the list below |
| `profile` | string, optional | the codec's profile, as ffmpeg names it (`main`, `main 10`, `high`), at most 32 characters |
| `width`, `height` | integer | in pixels, exact (not stepped: [video section 3.2](../spec/v0/video.md#32-the-picture) steps only the title's `video`) |
| `bitDepth` | integer, optional | 8, 10 or 12 |
| `hdr` | array of string, optional | as in [video section 3.2](../spec/v0/video.md#32-the-picture); absent for standard range |
| `fps` | number, optional | frames per second, two decimals at most |

A track is the track of `play`'s answer
([playback section 2.3](../spec/v0/playback.md#23-the-answer) and
[video section 5.3](../spec/v0/video.md#53-the-answer)), with the same
IDs (`a<n>`, `s<n>`, the owning server's own numbers, the same at every
`play` of that copy), and, for audio, one more member:

| Field | Type | Content |
|-------|------|---------|
| `codec` | string, optional | audio tracks: from the list below |

Subtitle tracks carry no codec: text tracks are served as WebVTT and
picture tracks as PGS ([video section 6.2](../spec/v0/video.md#62-what-is-fetched)),
which `pictures` already says.

**Names.** `container`, the video `codec` and the audio `codec` take
the values `play`'s capabilities list
([playback section 2.1](../spec/v0/playback.md#21-the-request),
[video section 5.1](../spec/v0/video.md#51-the-request)), plus these,
which a viewer's device may not play but a viewer's server must know of
to convert them: containers `avi`, `m2ts`, `mpegts`, `wmv`, `flv`,
`3gp`; video codecs `mpeg2video`, `mpeg4`, `vc1`, `wmv3`, `h263`,
`theora`; audio codecs `dts`, `truehd`, `mp2`, `wmav2`, `pcm_s24le`,
`pcm_bluray`, `dts-hd`. Anything else is sent as `other`. A viewer's
server reads `other` as "convert it".

**The summary stays.** `audio`, `subtitles` and `video` stay as they are,
and an owning server keeps sending them, worked out from its copies as
today. A server that doesn't send `copies` is still understood; a
viewer's server that ignores `copies` loses nothing.

**A change of `copies`** (a copy added, replaced or gone, a track
added) is a change of the title: a changes reading sends it again
([catalog section 3.2](../spec/v0/catalog.md#32-whole-list-or-changes)),
as for `video` today.

**`play` still decides.** `copies` describes the copies; the owning
server still chooses what to serve from `play`'s capabilities
([playback section 2.6](../spec/v0/playback.md#26-choosing-the-copy-and-the-tracks)),
and the answer still lists the tracks. When the answer and the catalog
disagree (the copy changed since the reading), the answer wins, and the
viewer's server should read the title again.

### 2. What the viewer's server keeps

Besides [video section 3.1](../spec/v0/video.md#31-what-the-caller-keeps):

- at most 8 copies, the first 8; a copy without a valid `durationMs`
  (an integer from 0 to 100 days in milliseconds) or without
  `container` is skipped alone;
- `id` and track IDs follow the rules of
  [playback section 2.3](../spec/v0/playback.md#23-the-answer); two
  copies with the same `id`: the second is skipped; a track with a bad
  ID or an empty label is skipped alone, as in `play`'s answer;
- an unknown `container` or `codec` is kept as `other`;
- `width` and `height` from 1 to 100,000, `bitDepth` 8, 10 or 12,
  `fps` above 0 and at most 1,000, `bitrateKbps` from 0 to 10,000,000:
  out of range, the member counts as absent;
- names and labels made plain text, as everywhere.

### 3. Rounding, so a copy can't be told by its numbers

A copy's exact size and bitrate could pick out the very file it came
from (a known release). The owning server rounds:

- `bitrateKbps` down to a whole 500 (below 500: absent);
- `durationMs` down to a whole second;
- `fps` to two decimals.

Codecs, sizes, track lists and languages aren't rounded: a media server
needs them as they are to decide how to play the file, and the first
`play` gives most of them anyway.

### 4. Pages bounded by size

A title with two copies and a dozen tracks each is some 2 to 3 KB of
JSON; 5,000 of them would pass the 4 MiB an answer may hold
([core section 3](../spec/v0/core.md#3-transport)). So
[catalog section 3.1](../spec/v0/catalog.md#31-paging-and-ordering)
changes:

- a page holds at most 5,000 titles **and at most 3 MiB of JSON**; the
  owning server ends the page early when the next title would pass it;
- in a whole-list answer, `next` is set **whenever titles remain**, not
  only on a full page;
- the caller already stops when `next` is absent: nothing changes on its
  side.

### 5. Chapters

A copy's chapters, as its file or its media server names them:

| Field | Type | Content |
|-------|------|---------|
| `startMs` | integer | where the chapter starts, in the copy, from 0 to its `durationMs` |
| `name` | string, optional | its name ("Opening", "Chapter 3"), plain text, at most 64 characters |

The viewer's server keeps them in order, drops a chapter whose `startMs`
is out of range or not after the one before (a repeated start is dropped),
keeps the first 256, and shows them as its media server shows a local
file's: marks on the seek bar, a chapter list, previous and next
chapter. Chapter images aren't sent: a viewer's server that shows them
takes them from the copy's seek-bar previews (section 6), or shows none.

An owning server sends the chapters its media server knows of the copy
(for Jellyfin, those it read from the file or a plugin gave it); none
known, it leaves `chapters` out. A copy of several parts lists the
chapters of all its parts, each part's shifted by the length of the
parts before it.

### 6. Seek-bar previews before a first play (decision)

Today a copy's previews come only with a playback
([video section 5.5](../spec/v0/video.md#55-seek-bar-previews)): a viewer's
server gets them the first time someone plays the title, and its viewers
see them from the second. Two ways:

- **(a) A call for them, `previews`.** Its request names a title and a
  version (`key`, `versionId`); its answer is the `trickplay` object of
  video section 5.5, with sheet URLs under a ticket of its own that lives
  10 minutes and is no playback: it doesn't count against `streams`,
  starts no conversion, and gets no `progress` or `stop`. A viewer's
  server asks it once per copy, after reading the title, and keeps the
  sheets (some 30 to 300 KB per episode at 320 pixels). The owning
  server answers `404` for a copy without previews, and may refuse with
  `429` when asked too often (core section 8.3). `copies` says which
  copies have previews: a `previews` member, `true`, on each.
- **(b) No change.** Previews from the second time a title is opened, as
  today.

Recommended: (a). A viewer's first look at a title is where a missing
seek-bar preview shows most, and the cost to the owning server is small
and spread out (a few requests per title, once).

## Alternatives considered

1. **The viewer's server remembers what the first `play` gave.** No
   protocol change: the plugin could keep each stand-in's streams,
   codecs and length after its first playback, and make later plays
   plain. But the first play of every title stays the live-TV kind, the
   versions stay unknown until then, and a replaced copy goes stale
   until played again. It is a stopgap for a gap in the protocol, and
   every other implementation would have to rebuild it.
2. **A new call, `describe`, for one title's copies.** Asked when a
   viewer opens a title's page, before `play`. It doesn't count
   against `streams`. But a media server builds its library ahead of
   time, not on a page view: it would call `describe` for every title,
   one call each, tens of thousands of signed calls instead of a few
   pages. The catalog is where a library is built from.
3. **A summary only, `durationMs` beside `video`.** Fixes the length,
   not the rest (codecs, versions, tracks), and so not the live-TV
   workaround.
4. **Codec names as each media server gives them.** Simpler for owning
   servers, but every viewer's server would have to know Jellyfin's,
   Plex's and Emby's names. MSFP already has its own lists for
   capabilities; this extends them.

## Cost

**Specification.** Video section 3 (the member, its cleaning, the
names), catalog section 3.1 (pages by size), new test vectors in
`vectors/v0/video.json` (copies kept, skipped, cleaned; `other`), and a
conformance check that a title's copies match what `play` then answers
(the same IDs, the same tracks).

**msfp-go and msfp-dotnet.** A `Copies` member on the video title, the
host filling it (the engine rounds and cleans what it sends, and
cleans what it reads), and pages ended by size. The hosts' `Catalog`
gains what it needs to describe copies, either in `Titles` or by
reusing `Playback.Describe`, which already gives tracks and length.
Both engines' tests.

**Overfin, as an owning server.** Its index reads each file's streams
from Jellyfin already (languages, height, HDR, length for films); it
needs the container, codecs, bitrate and every track, for every version
of a title. Whether its index holds one row per version or per item is
to check before the work is sized. Episodes' length isn't kept today.

**Overfin, as a viewer.** Optional: show a peer's title's length and
versions on its page.

**The plugin.** Owning side: easy, Jellyfin's media sources give
everything. Viewing side: the stand-ins get real media sources, one per
copy, with their streams and length from the catalog. That opens the
change this proposal is for: plain remote sources instead of sources
that must be opened, which would end the Kodi live-TV behaviour, the
plugin's own reading of the stream, the audio-switch filter and the
"not remote" previews trick. That change is the plugin's, to try in a
spike first (see open point 4).

## Compatibility

`copies` is optional and new: an older reader ignores it, an older
owner doesn't send it, and everything works as today. The size bound on
pages is new for owning servers only; callers already follow `next`.
MSFP is version 0, a draft: no version number changes.

## Open points

1. ~~**8 copies, 32 audio and 64 subtitle tracks**: enough?~~ Decided
   2026-11-21: kept.
2. ~~**Episodes in a copy holding several**: the copy's whole length, or
   the episode's share?~~ Decided 2026-11-21: the episode's share
   (`startMs` and its own `durationMs`) when known, the copy's length
   otherwise.
3. ~~**Rounding**: enough, or drop `bitrateKbps`?~~ Decided 2026-11-21:
   kept as written (bitrate down to 500 kbit/s, length to the second).
4. ~~**The plugin's spike**~~ Not a protocol point: step 5 of the
   plugin's plan (the playback hand-over), after this proposal.
5. ~~**`audio`, `subtitles` and `video`**: keep them or drop them?~~
   Agreed 2026-11-21: one source of truth. The summary goes, and `copies`
   becomes required for films and episodes, with a small required core
   (version ID, length, track languages); the details stay optional.
   MSFP is a draft and both implementations are ours: now is the time.
6. **Chapters** (section 5): 256 and 64 characters enough?
7. ~~**Previews before a first play** (section 6): (a) or (b)?~~ Decided
   2026-11-21: (a), the call `previews`.

## Recommendation

Adopt sections 1 to 5, and section 6 (a). The catalog is where a media server builds its
library from, and a library needs each file's length, container,
picture and tracks before anyone plays it. It should have been there
from the start; version 0 is the time to add it.
