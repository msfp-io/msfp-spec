# Proposal 17: Neutrality

Status: Adopted (2026-11-22). Amends [proposal 16](0016-version-0-revision.md)
and reverses some of its decisions (listed below). Every choice here is
the owner's, taken on 2026-11-22.

**Frozen** (2026-11-22): no further design review. While it is written
into the spec, a problem is fixed here only when it blocks an
implementation; anything else is listed for a later proposal.

**Written so far.** Only this proposal's first version is in spec/v0:
core 8.5 (one rule for unknown values, without the media-type class);
`implementers.md` (the product text); `spec/registry.md` (schemes only);
catalog 2 and 4 (open library kinds, `tmdb` in `ids`); video 3 to 6
(exact values, `fps` as a fraction, ffmpeg's names, open lists, the wider
`dv`, `sampleAspect`, `rotation`, 16 copies, 64 fonts, themes); playback
2, 5 and 8 (no capabilities, the baseline, remux to `mpegts` or `mp4`,
`audioRenditions` in the answer, opaque playback IDs, limits up to 1,000,
`scanBytes` in `limits`, the choice tables as recommended defaults);
media 1 to 4; music 3 (MusicBrainz types, exact values). Nothing else
below is: format names, colour as numbers, previews on demand, the scan
cap in the scan's answer, the field review's drops, URLs built from the
ticket, 16 music copies, the viewer review's changes. **The whole
proposal, as written here, is to be written into spec/v0, the vectors,
the engines and the conformance suite in one batch**, replacing what the
first version put there where they differ.

Touches: [core](../spec/v0/core.md) (8.4, 8.5),
[catalog](../spec/v0/catalog.md) (2, 3, 3.3, 3.4, 4.4),
[video](../spec/v0/video.md) (3.2 to 3.10, 4.3, 5.1 to 5.6, 6.1, 6.2, 7),
[playback](../spec/v0/playback.md) (2.1 to 2.7, 3, 5, 8),
[media](../spec/v0/media.md) (1, 3, 3.1, 3.3, 3.4, 4, 4.2),
[music](../spec/v0/music.md) (3, 3.1, 3.3, 5.2, 6, 9),
[groups](../spec/v0/groups.md) (5, 8.3, 9),
[implementers.md](../spec/v0/implementers.md), the
[registry](../spec/registry.md) (widened to the lists that describe
media), the vectors and the conformance suite. No new part; signing,
pairing, relations and key rotation are untouched.

The registry's first content is in [0017-registry.md](0017-registry.md);
the research behind the format names in
[0017-format-names-research.md](0017-format-names-research.md).

## Problem

The owner's principle (2026-11-22):

> We are adapting the protocol for everyone. What happens if a new
> player enters the game? We adapt again. It should not be our job to
> adapt. The implementations should do it.

Three rules the owner set later the same day, for every field:

1. **No field that is only safe through owner guidance.** A field whose
   safety rests on "the owner should leave it out" (a copy name that may
   be a file name) shouldn't exist.
2. **No duplicate data.** One fact is said once.
3. **No data the receiver can resolve itself**: from what is already
   sent, from a public database by an ID already sent, or from the
   stream at play time.

**The goal, clarified.** A federated title behaves and feels like a
local one, with every feature a local title has. Its labels need not be
the owner's: the viewer's server builds them in its own words, from
facts.

Proposal 16 was shaped by a survey of six products, and drifted toward
them. A review of spec/v0 after it (161 rows) found about 60 rows to
change, in three shapes:

1. **Closed lists MSFP keeps itself**: codec and container names, HDR
   formats, editions, audio roles and profiles, segment kinds, extra
   types, release types, library kinds. Each new codec is a spec change;
   library kinds and segment kinds are Jellyfin's lists word for word.
2. **Product text in normative sections**: about 20 paragraphs on
   Overfin, Jellyfin, Plex, Kodi and BIF.
3. **Viewer policy fixed by the spec**: copy order, language matching,
   subtitle choice, and capabilities from which the owner chose for a
   device it can't see.

Unknown values were handled three ways: ignored, refused (`400`), or
turned into `other`.

A second review (about 217 fields, against the three rules) found some
30 fields to drop and 8 to reshape.

A third, the viewer review, asked whether a viewer's server can still do
everything a local title allows (Jellyfin, Emby, Kodi and Plex as
viewers; browsers, Media3 and AVPlayer as players). Deciding whether a
copy plays as it is was nearly covered; asking for what a local server
would make when it doesn't (video kept with the audio converted, a known
output picture) was not.

**The rule this proposal applies.** The protocol carries facts in
outside vocabularies (IANA media types, RFC 6381, ITU-T H.273, the DASH
role scheme, MusicBrainz) and, where none exists, in MSFP's registry,
with open lists and one rule for unknown values. It carries no product
text in normative sections, no field that needs guidance to be safe, no
fact twice, and nothing the receiver can work out. Policy is the
viewer's server's, except the safety defaults the owner kept normative.
Implementations map their media server's names to these vocabularies.

## What changes in proposal 16

| Proposal 16 | Was | Now |
|-------------|-----|-----|
| Proposal 15 point 3 | `durationMs` to the second, `bitrateKbps` to a whole 500, `fps` two decimals | exact values (section 3.6); `bitrateKbps` dropped (section 4) |
| 1.3, names | MSFP's list from the capabilities lists, else `other` | the file's media type (section 3.1); codec and subtitle tokens from the registry (section 3.2) |
| 1.1, `colour` | ffmpeg's names | H.273 numbers and `fullRange` (section 3.4) |
| 1.1, `hdr` | `hdr10`, `hdr10plus`, `hlg`, `dv` | enhancement layers only, from the registry; HDR10 and HLG from `colour`; Dolby Vision by the `dv` object alone (section 3.4) |
| 1.1, audio `profile` | MSFP's list (`atmos`, `dtsx`…) | dropped; Atmos and DTS:X in `objects` (section 3.5) |
| 1.1, `profile`, `level`, `codecTag`, `layout`, `fileIndex` | sent | dropped (section 4) |
| 1.4, names and titles | copy `name`, track `title`, extra `name`, owner guidance | dropped (section 4); labels built from facts; `role` on subtitle tracks too (section 3.9) |
| 1.1, `durationMs` of a shared episode | the episode's, else the copy's | always the copy's; the episode's span by `startMs` and `endMs` (section 3.7) |
| 1.1, `parts` | each part's `size` and `durationMs` | a count (section 3.7) |
| 1.1, `added` | sent | dropped (section 4) |
| Proposal 15 point 1, copies | at most 8 | 16, music tracks too (section 3.8) |
| 4 (decision 6), themes | left out | allowed as extras (section 3.9) |
| 5, `releaseType` | MSFP's list | MusicBrainz's `primaryType` and `secondaryTypes` (section 10) |
| 6 (decision 8), capabilities | the viewer may ask a method; capabilities enriched, coarsened | dropped: the viewer's server sends `delivery`; without it, the baseline, its picture defined; a transcode may `copy` one stream and names its `range` (section 8) |
| music queue | every entry its first copy, under the request's `delivery` | an entry may name its own `versionId` and `delivery` (section 10) |
| 7 (decision 4), fonts | 8 a copy, 10 MiB each | 64 a copy, 10 MiB each, 64 MiB in all (section 9.2) |
| 7, previews | fetched for every copy before play, every sheet at once | layout in the catalog; sheets only for copies someone opens, never a library ahead (section 6) |
| 9 (decision 3), scan cap | 32 MiB fixed, Plex text normative | the cap the scan's answer states, 4 MiB to 1 GiB, not in `limits`; Plex text to the notes (section 9.4) |
| 3 and 6, choice | the owner chooses copy and tracks from the viewer's languages | the viewer's server chooses; no language members (section 7) |
| 1.2, answers | URLs, tracks, length, media type | the ticket and what the catalog can't know (section 5); subtitle forms fixed in advance for the formats video 5.4 lists (section 5.3) |
| 8, limits | `streams`, `downloads` at most 50 | at most 1,000 (section 9.1) |

Kept as proposal 16 decided: "off by default", normative, for `local`
titles, downloads and scans; owner-sent images for `local` titles only,
JPEG or PNG; exact `size`; the music queue of 5; ASS rebuilt by the
engine; the `image` call; lyrics as their own call; chapters (256, names
64); downloads' speed cap and 24-hour ticket; complete VOD HLS; file
delivery, play on first read and restarts on the same `etag`; the `tvdb`
and `local` schemes; `HEAD` answered `405`.

## Proposal

### 1. One rule for unknown values (core 8.5)

One rule, in the core, for every part. It replaces what each part said.

- A **token** is 1 to 32 characters of `a–z 0–9 . _ -`. Fields this
  proposal calls open take tokens, unless they say otherwise.
- A **media type** is an IANA-form media type: lowercase, exactly one
  `/`, no parameters, each half starting with a letter or a digit, at
  most 127 characters (RFC 6838's bound) of `a–z 0–9 . + - _ /`:
  `video/mp4`, `application/ttml+xml`. Media-type fields follow the
  token rules below.
- **In a fact** (what `titles` says, what an answer says): an unknown
  token or media type is **kept** as it is, never turned into another
  value, never dropped. A malformed value (another JSON type, empty, too
  long, other characters) is handled as any malformed member: an
  optional member is dropped alone; a required one drops the entry that
  holds it (a copy, a stream, a track, an extra).
- **In a request**: an unknown value is ignored where the field is a
  hint, and refused with `409` `delivery-refused` where it names what to
  serve (a remux's media type, a transcode's codec). Only a malformed
  value gets `400`.
- A viewer's server that doesn't know a copy's `mediaType`, a stream's
  `codec`, or its `threeD`, or can't parse a `codecs` string, treats the
  copy as one it can't play as it is: it asks for a `remux` or a
  `transcode`, or doesn't offer the copy. An unknown subtitle `format`
  affects that track only. Other unknown values (`edition`, `role`, an
  `hdr` token, `objects`, `extraType`, a segment `kind`) don't stop the
  copy from playing as it is; section 3.9 says how some are shown.
- No list has a value meaning "other". A sender with no value for an
  optional field leaves it out.
- The values a part gives beside an open field are examples. The lists
  are the registry's (section 12) or an outside vocabulary's, which the
  part names. Core 8.5 no longer names ffmpeg as a vocabulary.

### 2. Product text out (implementers.md)

As in the first version: no product is named in normative text.
`spec/v0/implementers.md` takes every Overfin, Jellyfin, Plex, Kodi and
BIF paragraph, the suggested numbers (section 9.5), music tag names, the
mapping of credits to `outro`, finding TMDB numbers from other IDs,
trickplay as BIF files, Plex's scans, and a "Decoding safely" section
(the JPEG DNL rule from catalog 4.4 and other decoder hygiene). From
this version it also takes: the table from ffmpeg's colour names to
H.273 numbers, and the `tv`/`pc` words (section 3.4); how to build
`codecs` from ffprobe and from the file's records (section 3.3); where
each codec stores Atmos and DTS:X (section 3.5); mapping a track's title
to `role`, `hearingImpaired` and `forced` (section 4); the choice
tables, as advice to viewer's servers (section 7); using the overall
rate (`size` × 8 / `durationMs`) as the bound on a stream's bitrate,
since no stream carries its own (inflated by cover art on music); writing
`ec+3` from `ec-3` and `objects` `dolby-atmos` where a player wants it;
Plex as a viewer only through scan access, and a 4 MiB cap missing an
MP4 `moov` or Matroska cues at the end of a big file (section 9.4). Its
sentences that call ffmpeg's names a vocabulary go.

Album art: "from a public source by MusicBrainz ID, such as the Cover
Art Archive" (music 4).

**`HEAD`** (media 2.2, core 8.4): the wire stays: media URLs answer
`GET` only; any other method gets `405` with `Allow: GET`. The Kodi
reason goes.

### 3. Copies: facts in outside vocabularies (video 3.2, 3.3, 3.5; music 3.3)

#### 3.1 The container: `mediaType`

`container` gives way to **`mediaType`** (string, required), the file's
media type (section 1), on video and music copies alike.

- **IANA's types** where IANA has one: `video/mp4`, `video/quicktime`,
  `video/matroska`, `video/mp2t`, `video/ogg`, `video/3gpp`, `audio/mp4`,
  `audio/matroska`, `audio/flac`, `audio/mpeg`, `audio/ogg`… Lowercase
  (`video/MP2T` is sent `video/mp2t`).
- **Where IANA has none**, the registry names the type in use, from
  WHATWG MIME Sniffing or the format's own project: `video/webm`,
  `audio/webm`, `video/avi`, `audio/wave`, `audio/aiff`. Other spellings
  (`video/x-msvideo`, `audio/x-wav`, `video/x-matroska`) are listed as
  aliases: a producer sends the registry's type; a receiver may accept
  an alias as the same.
- **Audio or video.** `audio/<sub>` when the file has no video stream
  other than attached pictures (cover art), else `video/<sub>`. For the
  pairs the registry lists (`mp4`, `matroska`, `webm`, `ogg`, `3gpp`,
  `3gpp2`, `mp2t`), a receiver treats `audio/<sub>` and `video/<sub>` as
  the same container; not for others (`audio/mpeg` is MP3 alone,
  `video/mpeg` MPEG-PS).
- **BDAV.** A transport stream of 192-byte packets (Blu-ray, AVCHD:
  M2TS) is `video/vnd.dlna.mpeg-tts`, IANA's type; one of 188-byte
  packets stays `video/mp2t`.
- **From the file where it can.** A producer reads the type from the
  file's first bytes where it can, else maps its media server's
  container name through the registry. For ISO BMFF, the major brand
  (`ftyp`) chooses the type, by the registry's brand table; an unknown
  brand gives `video/mp4` (or `audio/mp4`). A WebM file whose DocType
  isn't known may be sent as `video/matroska`: less precise, never
  wrong.
- **No `brand` field.** The brand only chooses the media type.
- **The caller** keeps `mediaType` as section 1 says; a copy whose
  `mediaType` is missing or malformed is skipped (video 3.8, music 3.3),
  as one without `container` was.
- **The relayed type** (media 3, 3.3): the viewer's server relays a file
  with the copy's own `mediaType`, whatever its top-level type, and as
  `application/octet-stream` when it has none.

#### 3.2 Codec and subtitle names: registry tokens

Video and audio `codec` (required, as today) and subtitle `format`
(optional, as today) are open tokens **defined by the registry**, not by
a product.

- Tokens are spelled as ffmpeg spells its `codec_name` where ffmpeg has
  a name (`h264`, `hevc`, `av1`, `aac`, `eac3`, `truehd`, `dts`, `opus`,
  `pcm_s24le`, `subrip`, `ass`, `webvtt`, `hdmv_pgs_subtitle`,
  `dvd_subtitle`), so that a producer reading ffprobe maps nothing; one
  reading the file another way maps through the registry's crosswalk
  (MP4RA sample entries, Matroska codec IDs, IANA types). The registry,
  not ffmpeg, is the authority; its ffmpeg column is informative.
- A producer sends the registry's token. For a codec with none yet, it
  sends the token it proposes for registration, and switches once one is
  registered. Unknown tokens are kept meanwhile (section 1).
- Each subtitle format's kind (text or picture) is the registry's. No
  `kind` field is sent: the forms a track is served in (section 5.3) say
  what arrives.
- Captions carried inside the video stream (CEA-608 and 708 in SEI or
  user data) aren't listed as subtitle tracks.
- Dolby Vision and encryption aren't codecs: their sample entries map to
  the base codec's token.
- The names table of video 3.5 goes; the section points to the
  registry. Transcode targets (section 8) take the same tokens.

#### 3.3 `codecs` (RFC 6381)

On the video stream and on each audio track, `codecs` is the stream's
codecs string of RFC 6381 (`avc1.640028`, `hvc1.2.4.L153.B0`,
`av01.0.08M.10`, `mp4a.40.2`, `ec-3`, `dtsh`), 1 to 64 characters of
`A–Z a–z 0–9 . -`. No `+`, no `/`.

- **Required** when `codec` is `h264`, `hevc`, `av1` or `vp9` (video)
  or `aac` or `dts` (audio): there it carries the profile, level, tier or
  flavour players decide on. **Optional** for every other codec. Where
  it is a fixed code the registry gives (`ac-3`, `ec-3`, `fLaC`, `Opus`,
  `mlpa`), it may be left out; where its string carries a profile or
  level (`mpeg4`'s `mp4v.20.<n>`, `vvc`, `ac4`, `mpegh_3d_audio`), it is
  sent when known.
- **The chroma format.** For `hevc` with `general_profile_idc` 4 or
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
  profile, level, tier and chroma are `codecs`. They must agree; when they
  don't, the viewer's server decides by the authoritative field and still
  hands `codecs` to players as it is.
- **Missing or malformed where required**: the stream or track is kept,
  `codecs` counts as absent, and the viewer's server treats the copy as
  one it can't play as it is (section 1).
- **Dolby Vision**, every profile: `codecs` is the base codec's string,
  with the base 4CC of the file's Dolby Vision entry (`dvh1` → `hvc1`,
  `dvhe` → `hev1`, `dav1` → `av01`, `dva1` → `avc1`, `dvav` → `avc3`);
  the `dv` object holds the rest (section 3.4). A consumer rebuilds the
  Dolby Vision string (`dvh1.08.06`) from that 4CC and `dv`. A viewer's
  server that can't decode Dolby Vision treats a copy whose
  `dv.compatibility` is 0 as one it can't play as it is.
- Music track copies follow the same rules (music 3.3).

#### 3.4 Colour and HDR (video 3.3, 3.7, 3.8)

- **`colour`**: `primaries`, `transfer` and `matrix` as **ITU-T H.273
  code points**, integers 0 to 255, each optional, and **`fullRange`**,
  a boolean (H.273's full-range flag), absent when unknown. The value 2
  ("unspecified") isn't sent: the member is left out. A member that isn't
  an integer in range, or a `fullRange` that isn't a boolean, counts as
  absent alone; a `colour` left empty counts as absent.
- **`transfer`** is the preferred transfer: the alternative transfer
  characteristics SEI's value when the stream has one (broadcast HLG
  often signals 14 in the VUI and 18 there), else the VUI's. When the
  bitstream and the container (`colr`, Matroska's colour elements)
  disagree, the bitstream wins.
- **The range follows from `transfer`**: 16 is PQ (HDR10's transfer),
  18 is HLG; 14 and 15 are standard range in BT.2020. A copy with no
  `colour` says nothing of its range.
- **`hdr`**: only enhancement layers and dynamic metadata, as open
  tokens from the registry (`hdr10plus` today), sorted in byte order,
  each once, at most 8. `hdr10`, `hlg` and `dv` aren't sent there.
- **`dv`**: the Dolby Vision configuration record, sent whenever the
  copy has Dolby Vision: `profile` 0 to 127, `level` 0 to 63,
  `compatibility` 0 to 15, and the optional boolean `el` (an enhancement
  layer is present). `bl` and `rpu` go. The caller keeps `dv` with
  `profile`, `level` and `compatibility` in range, as today, without the
  condition on `hdr`.
- Example: `"colour":{"primaries":9,"transfer":16,"matrix":9}`.

#### 3.5 Object audio: `objects`

On an audio track, **`objects`** (string, optional): the object-based
format it carries, one open token from the registry, `dolby-atmos` or
`dts-x` today (names no 4CC uses: `dtsx` is DTS-UHD's sample entry).
`objects` is the fact; a viewer's server picks an Atmos or DTS:X track by
it. The notes say where each codec stores it (E-AC-3's JOC flag, the
TrueHD major sync, DTS-HD MA's extension substream).

#### 3.6 Other stream facts (video 3.3)

- **Exact values**: `durationMs` as known, never rounded.
- **`fps`**: `"<num>/<den>"`, each 1 to 1,000,000, reduced
  (`"24000/1001"`, `"25/1"`): the nominal rate the stream or container
  declares; for a variable rate, always the average. Malformed: absent.
- **`sampleAspect`**: `"<w>:<h>"`, each 1 to 65,535, as the stream or
  container states it; absent means square pixels.
- **`rotation`**: `0`, `90`, `180` or `270`, the degrees clockwise to
  show the picture upright; absent means 0. A display matrix that mirrors
  the picture, or turns it by another angle: left out.
- **`bitDepth`**: 1 to 16 on video. Where `codecs` holds the bit depth
  too (AV1, VP9), the two agree; see "One place per fact" (section 3.3).
- **`refFrames`** (new): integer, optional, on an H.264 video stream
  only: the stream's declared maximum number of reference frames
  (`max_num_ref_frames`), 1 to 16. Out of range, or on another codec:
  absent. Some players refuse H.264 past a number of reference frames,
  and `codecs` doesn't carry it.
- **`threeD`**: an open token from the registry (`sbs`, `tab`, with
  the left eye first; `sbs-rl`, `tab-rl`, with the right eye first;
  `mvc`), absent for 2D. Half or full resolution follows from the
  picture's size.
- **`channels`** stays; `layout` goes (section 4): labels come from the
  count.
- **`original`** (audio): only the file's own flag (Matroska's
  FlagOriginal and the like), never worked out from metadata.

#### 3.7 Length, parts and track IDs (video 3.2, 4.2, 5.2; playback 2.3)

- **`durationMs` is always the copy's**, all its parts: the length of
  what `size` measures.
- **`endMs`** (new): integer, optional, for an episode in a copy holding
  several (`with`): where the episode ends in the copy. With `startMs`
  it gives the episode's span. It must be above `startMs` (0 when
  `startMs` is absent) and at most `durationMs`; otherwise it counts as
  absent.
- **`parts`** becomes an integer: the number of files the copy is made
  of, 2 to 16; absent for one file. Out of range: absent. A copy of
  several parts is never served as `file`, as today.
- **Track IDs**: for a copy of one file, the *n* of `a<n>` and `s<n>` is
  the stream's index in the file, from 0. A subtitle track beside the
  video (`external`) takes a number above the file's streams. For a copy
  of several parts, *n* is the owning server's own, as today. `fileIndex`
  goes, and so does playback 2.3's "reads nothing into *n*".

#### 3.8 Bounds (video 3.2, 3.8; music 3.3)

`copies`: at most **16**, for video titles and music tracks alike.
`audioTracks` at most 32, `subtitleTracks` at most 64, unchanged.

#### 3.9 Open lists (video 3.3, 3.4, 3.9)

Each is an open token from the registry (section 12), unknown values
kept (section 1):

- `edition` (copy);
- audio `role`: the DASH role scheme's values (`urn:mpeg:dash:role:2011`:
  `main`, `alternate`, `supplementary`, `commentary`, `dub`,
  `description`…); an unknown role is never taken for the title's own
  sound;
- subtitle `role` (new, optional): from the same scheme (`subtitle`,
  `commentary`, `description`, `easyreader`…). `forced` stays the only
  forced flag and `hearingImpaired` the only one for subtitles with
  sounds (SDH): `forced-subtitle` and `caption` aren't sent as a `role`
  (the registry notes how they map);
- segment `kind` (`intro`, `outro`, `recap`, `preview`, `commercial`); a
  viewer's server shows a kind it doesn't know as a plain "Skip";
- `extraType`, with `themesong` and `themevideo`: an owning server may
  offer theme songs and videos as extras; the viewer's server decides
  whether and how to present them, and each one it plays is a playback
  like any other. An unknown type is shown as a plain extra. Extras are
  listed in key order (byte order), so a label built from `extraType`
  and a number stays the same between readings.

### 4. Fields dropped

Each by the three rules (Problem): **G** only safe through guidance,
**D** said twice, **R** the receiver can work it out.

| Field | Where | Why |
|-------|-------|-----|
| copy `name` (video and music) | video 3.2, 3.6; music 3.3 | G: media servers fill it from file names. The viewer's server builds a label from height, HDR, `edition`, codec, sample rate. |
| track `title` (audio, subtitle) | video 3.3, 3.6 | G: release groups and language names end up here. What a good title says is in `role`, `hearingImpaired` and `forced`; the owner maps titles to those (notes). |
| extra `name` | video 3.9 | G: often a file name. Shown as `extraType` and a number. |
| title `kind` | catalog 3 | D: the key says it. |
| copy `bitrateKbps` (video and music) | video 3.2; music 3.3 | R: `size` × 8 / `durationMs` (section 3.7). |
| `codecTag` | video 3.3 | D: the first element of `codecs`, or on Dolby Vision the entry rebuilt from that 4CC (section 3.3). |
| video and audio `profile`, video `level` | video 3.3 | D: in `codecs`. |
| audio `layout` | video 3.3 | R: labels come from `channels`; the exact layout stays in the stream. |
| `fileIndex` | video 3.3 | D: the track ID's number (section 3.7). |
| `parts` sizes and lengths | video 3.2, 3.8 | D: nothing reads them; a count stays (section 3.7). |
| `dv` in `hdr`; `dv.bl`, `dv.rpu` | video 3.3 | D: the `dv` object says it; R: `bl` and `rpu` hold in every stream a player meets. |
| `brand`, subtitle `kind` (first amendment) | — | D: the brand chooses `mediaType`; the registry gives each format's kind. |
| copy `added` (video and music) | video 3.2; music 3.3 | a fact about a person. R: "recently added" from the day the viewer's server first saw the copy; the titles of a first reading count as old. |
| progress `paused` | playback 3 | a fact about a person, used by no rule. |
| library `kind` | catalog 2 | R: the titles' keys say what a library holds; a viewer's server types an empty library by its first titles. |
| segment `final` | video 3.4 | R: the last `outro`. |
| `ids` on `tmdb`-keyed films and series | video 4.3 | R: TMDB's lookup by key gives them (`external_ids`). Kept on episodes and on `tvdb`-keyed titles. |
| `ids` on `local` music titles | music 3.1 | no allowed use: a `local` title is never matched through `ids`. |
| `isrc`, `acoustid` on `mb` tracks | music 3.1 | R: from `mbrecording`, MusicBrainz gives the ISRCs and the AcoustID service the AcoustIDs. |
| album `sortName` | music 3 | R: receivers sort by their own rules. |
| track `artists`, `artistNames`, `date`, `originalDate`, `genres` equal to the album's | music 3 | D: sent on a track only when they differ from its album's. |
| lyrics `displayArtist`, `displayTitle` | music 6 | D: the track's `artistNames` and `name`. |
| lyrics `offset` | music 6 | R: the owner adds it to every `start` before sending. |
| lyrics `synced` | music 6 | D: a set is synced when its lines carry `start` (section 10). |
| `name` in `group-pair` and in the mail | groups 5, 8.3 | D: the member's `profile` name. Kept in `joined`, until the newcomer's `profile` arrives. |
| `audioLanguage`, `subtitleLanguage`, `subtitleTrack` (requests) | playback 2.1; video 5.1 | the viewer's choice (section 7). |
| answer `subtitleTrack`, `forcedTrack`, `forcedTracks`, `burnedTrack` | video 5.3 | the viewer's choice; D: `burnSubtitle` names the burned track. |
| answer `audioTracks`, the subtitle tracks' members, `durationMs`, `stream` (`url`, `mimeType`), `fonts`, `trickplay` | playback 2.3; video 5.3 | D: in the catalog, or built from the ticket (section 5). |
| queue entries' `durationMs` and `stream` | music 5.2 | as for the answer. |
| `previews` (boolean); the layout in `previews`' answer | video 3.2, 5.6 | D: `previewLayout` in the catalog (section 6). |
| `limits.scanBytes` | playback 5 | D: the scan's answer states the cap that applies (section 9.4). |
| `url` in `image`'s answer | catalog 4.4 | built from the ticket (section 5). |

The first version's guidance on names (video 3.6) goes with the fields.
The `local` title's `name` (video 3.10) stays: nothing else names the
title. Its "never a file name" guidance goes; its protection is the
switch (`local` titles off by default, shared per pairing and per group).
The must that a viewer's server never sends a `local` description on,
nor looks it up elsewhere, stays.

### 5. Answers say only what the catalog can't (playback 2.3; video 5.3, 5.6, 6.1; music 5.2; catalog 4.4; media 1)

#### 5.1 The playback object

`play`'s answer keeps:

| Field | Type | Content | The viewer's server requires or does |
|-------|------|---------|--------------------------------------|
| `id` | string | the playback ID | `^[A-Za-z0-9_-]{1,64}$` |
| `ticket` | string | the playback's ticket (new; replaces `stream.url`) | `^[A-Za-z0-9_-]{1,64}$` |
| `protocol` | string | `hls` or `file` (out of `stream`, which goes) | anything else: refused; without `delivery`, `hls` |
| `method` | string | `direct`, `remux` or `transcode` | as today |
| `etag` | string | the copy playing | as today |
| `positionMs` | integer | where it starts, in the copy | as today |
| `versionId` | string | the copy playing | as today |
| `audioTrack` | string, optional | the audio track playing | as today |
| `audioRenditions` | boolean, optional | renditions made | as today |
| `maxBitrateKbps` | integer, optional | the cap in effect | as today |
| `scanBytes` | integer | scans only (section 9.4) | section 9.4 |
| `subtitles` | array, optional | video: the subtitle tracks served apart (section 5.3) | section 5.3 |
| `fontCount` | integer, optional | video: the copy's fonts (section 5.3) | 0 to 64; otherwise 0 |

When the answer breaks a "must", the viewer's server sends `stop` and
gives up, as today. When its `etag` differs from the catalog's, the
answer wins for that playback and the title is read again, as today.

#### 5.2 URLs built from the ticket (media 1; video 6.1)

Answers carry **no URLs**. The viewer's server builds every media URL
from the owning server's stream root (media 1), the ticket and a fixed
name, so it never fetches an address a peer wrote.

| What | URL, after `<stream root><ticket>/` |
|------|-----|
| the stream | `master.m3u8` (`hls`) or `file` (`file`) |
| a subtitle track served apart | `subtitles/<id>`, `pictures/<id>` or `ass/<id>` (section 5.3), then `/<part>` or `/whole` as today |
| font *n* | `fonts/<n>`, *n* from 0 to `fontCount` − 1 |
| preview sheet *n* | `trickplay/<n>`, *n* below the sheet count of the copy's `previewLayout` (section 6) |
| queue entry *n* | `queue/<n>/file` or `queue/<n>/master.m3u8`, *n* the entry's place in the request's `queue` |
| a `local` title's image | `image`, under the ticket of `image`'s answer |

The owning server serves exactly these names, and answers `404` past a
count. HLS playlists' URIs are checked as media 1 and 3.1 say today.

#### 5.3 Subtitles and fonts in the answer (video 5.3, 5.4)

- **`subtitles`**: at most 64 entries, one per track of the copy the
  owning server serves apart: `{"id": "s<n>", "form": "text" or
  "picture", "ass": true}`. `form` `text` opens `subtitles/<id>`
  (WebVTT); `picture` opens `pictures/<id>` (PGS); `ass: true`, only with
  `text`, also opens `ass/<id>`. An entry whose `id` isn't a track of the
  copy in the catalog, or whose `form` is neither value, is dropped
  alone; `ass` that isn't a boolean counts as absent. A track of the
  file with no entry reaches a viewer only in the copy played as it is,
  or burned in. An `external` track the owning server can't serve apart
  isn't listed in the catalog.
- **`fontCount`**: the number of fonts the copy's ASS tracks use, 0 to
  64; absent means 0.
- **Forms known before `play`.** Video 5.4's table is a must for the
  formats it lists: `subrip`, `webvtt`, `mov_text`, `ttml` are served
  `text`; `ass` and `ssa`, `text` with `ass: true`;
  `hdmv_pgs_subtitle`, `dvd_subtitle` and `dvb_subtitle`, `picture`.
  Only a track in another format waits for the answer to know its form.

#### 5.4 The other answers

- **`previews`' answer** (video 5.6): `{"ticket", "etag"}`, `etag` being
  the copy's as the owning server holds it now. When it differs from the
  catalog's, the viewer's server reads the title again before fetching
  sheets.
- **`image`'s answer** (catalog 4.4): `{"ticket"}`.
- **Queue entries** (music 5.2): `title`, `versionId`, `method` and
  `protocol`; their URLs as section 5.2 says, under the playback's
  ticket. The request's entries are in section 10.
- A malformed `ticket`: the answer is refused (for `play`, `stop` and
  give up; for `previews` and `image`, no sheets, no image).

### 6. Seek-bar previews (video 3.2, 3.4, 5.5, 5.6, 6.2, 7; media 3.4, 4; music 3.3)

Proposal 16 had every paired server fetch every copy's sheets ahead; a
library's previews can weigh tens of gigabytes.

- **The layout is in the catalog.** A copy's `previews: true` gives way
  to **`previewLayout`**: `tileWidth`, `tileHeight`, `columns`, `rows`,
  `intervalMs` and `tileCount`, with video 5.5's bounds, plus `tileCount`
  at most 1,000 × `columns` × `rows`. No URLs. The sheet count is
  ⌈`tileCount` / (`columns` × `rows`)⌉. A malformed `previewLayout` is
  dropped alone. The `trickplay` object leaves the answers: the layout is
  said once.
- **A change of `previewLayout`** is a change of the title: a changes
  reading sends it again.
- **When sheets may be fetched** (normative). A viewer's server never
  fetches previews for a whole library, nor in bulk for copies nobody
  opened. For a copy a person opens (its page, its chapter list, a
  playback, a scrub), how it fetches the sheets, all at once or one by
  one, is its choice. Chapter images (video 3.4) count as use.
- **Under which ticket.** During a live playback of the copy, the
  playback's ticket opens its sheets, with no `previews` call. Outside
  one, the viewer's server calls `previews` once for the copy and uses
  that ticket for all its viewers until it expires (10 minutes); it calls
  again only after expiry, at most one call per copy at a time. A `429`
  means no previews of that peer until `retryAfter`. Video 5.6's "one
  copy at a time per peer" and "fetch every sheet at once" go.
- **`previews`' `404`**: for a copy without a `previewLayout` in the
  catalog. Video 5.6's purpose sentence ("from the first play") goes.
- **Caching recommended**, keyed by the copy's `etag` plus its
  `previewLayout`: a viewer's server keeps the sheets it fetched, and
  fetches none again while both hold. Owning servers are recommended to
  keep their re-encoded sheets.
- **Each sheet** is checked against the catalog's layout (size from the
  header, then decoding, then re-encoding), as video 6.2 says.
- **Privacy** (video 7): a `previews` call outside a playback tells the
  owner that someone on that server opened that copy, and the sheet
  numbers fetched tell roughly where. Fetching a copy's sheets together
  hides the position.
- Music copies carry no `previewLayout` (music 3.3).

### 7. Choice is the viewer's (playback 2.1, 2.6; video 3.7, 5.1, 5.4)

The viewer's server has every copy's facts and knows its device; the
owner's choosing goes.

- `copies` come in the **owner's order of preference**, best first.
- **`versionId`** names the copy; without it, the first copy plays.
- **`audioTrack`** names the audio track; without it, the file's default
  audio: the track the file marks default, else its first audio track.
- The request names a subtitle only to burn it in
  (`transcode.burnSubtitle`, video 5.1). Which subtitle a viewer sees is
  the viewer's server's choice, among the catalog's tracks and those
  served apart (section 5.3).
- `audioLanguage`, `subtitleLanguage` and `subtitleTrack` leave the
  request; `subtitleTrack`, `forcedTrack`, `forcedTracks` and
  `burnedTrack` leave the answer.
- The answer says what plays (`versionId`, `audioTrack`).
- The recommended order of copies (video 3.7) and the choice tables
  (playback 2.6, video 5.4) leave the protocol for the notes, as advice
  to viewer's servers, the order written in H.273 numbers (`transfer` 16
  or 18). Their vectors go.

### 8. No capabilities; one baseline (playback 2.1, 2.2, 2.7; video 5.1; music 5)

As in the first version, with media types and registry tokens:

- The `capabilities` object goes, with every capability list, the
  `max…` members and the coarsening.
- The viewer's server asks with **`delivery`**: `file`, `remux` or
  `transcode`.
- **Without `delivery`**, the owning server serves the baseline every
  viewer's server must play, in complete HLS. For a copy with video (a
  film, an episode, an extra): H.264 at most High profile and level 4.1,
  8-bit 4:2:0, progressive, SDR in BT.709 (H.273 primaries, transfer and
  matrix 1); AAC-LC, at most 2 channels, at most 48 kHz. For a copy with
  no `video` (a track, or an extra such as a theme song): the same AAC,
  audio only. It may leave a stream unchanged only when it is already
  within that.
- **Remux** (playback 2.7): the request member `container` becomes
  **`mediaType`**, required with `delivery` `remux` only: a media type
  (section 1) of subtype `mp2t` (MPEG-TS segments) or `mp4` (fragmented
  MP4 segments), the two HLS segment formats, as `video/…` or, for a
  copy with no `video`, `audio/…`. A malformed value gets `400`; a
  well-formed one the owner doesn't make, `409` `delivery-refused`. A
  remux is served as complete HLS. Playback 2.1, 2.2 and 2.7 and core
  8.5's example say "a remux's media type".
- **Transcode targets** (`videoCodec`, `audioCodec`, `maxHeight`,
  `maxBitrateKbps`, `audioChannels`, `burnSubtitle`, `range`): the
  codecs take registry tokens or `copy`; an unknown or unsupported codec
  gets `409` `delivery-refused`. A target left out takes the baseline's
  value: `h264`, `aac`, the copy's own height, no cap, at most 2
  channels, `sdr`. The
  steps stay (`maxHeight` 480 to 4320, `maxBitrateKbps` in 1,000s,
  `audioChannels` 2, 6 or 8).
- **`copy`** (new value of `videoCodec` and `audioCodec`, a closed
  mechanic like `delivery`, not a registry token): that stream is served
  unchanged. `409` `delivery-refused` when the segment format can't carry
  it. A copied stream takes none of the targets that would change it
  (`maxHeight` on video, `audioChannels` on audio); `burnSubtitle` with
  video `copy` is refused, `409` `delivery-refused`. A transcode whose
  video is `copy` counts as a transcode (playback 2.6, 6) only when its
  audio is converted.
- **`range`** (new target, closed): `sdr` (the default: tone-mapped to
  SDR, 8-bit, BT.709) or `keep` (the copy's transfer and primaries kept,
  10-bit; HDR10+ kept where the codec carries it). Dolby Vision is
  dropped unless the video is `copy`; with video `copy`, `range` is
  ignored. An owning server that can't make the range asked: `409`
  `delivery-refused`.
- **Sample entries in `mp4` segments**: HEVC is written `hvc1`, Dolby
  Vision `dvh1` (`dav1` on AV1), parameter sets out of band, in a remux
  and a transcode alike.
- **The master playlist** (media 3.1): each `#EXT-X-STREAM-INF` carries
  `BANDWIDTH`, `CODECS` and `RESOLUTION`; `VIDEO-RANGE` (`PQ` or `HLG`)
  when the picture served is PQ or HLG; `SUPPLEMENTAL-CODECS` when Dolby
  Vision is kept. An audio rendition's `CHANNELS` may be `"16/JOC"` for
  an E-AC-3 track whose `objects` is `dolby-atmos`. It is the only place
  the served stream is described.
- **Audio renditions**: a request, never refused; the answer's
  `audioRenditions` says whether they were made.
- **No ceiling of its own**: the 120,000 kbit/s default is gone.
- **Dead members gone**: `subtitleStretchSeconds`, `versions` in the
  answer, `info`.
- **Playback ID**: `^[A-Za-z0-9_-]{1,64}$`, from at least 128 random
  bits.

### 9. Limits, scans and defaults (playback 5, 6, 8; media 3.4, 4, 4.2; groups 9; core 8.4)

#### 9.1 Limits

`streams` and `downloads` 0 to **1,000**; `transcodes` 0 to `streams`;
music's `audioStreams` and `audioTranscodes` in the same bounds.
`scanBytes` leaves `limits` (section 9.4).

#### 9.2 Fonts (video 6.6; media 3.4)

At most **64** fonts a copy, each at most 10 MiB, **64 MiB in all**.
TrueType and OpenType only; font collections (TTC) later, once their
checks are written.

#### 9.3 Images (catalog 4.4)

Unchanged: `local` titles only, JPEG or PNG, at most 1 MiB. The DNL rule
moves to "Decoding safely". The answer carries the ticket (section 5.4).

#### 9.4 The scan cap, in the scan's answer

- `purpose: scan` stays, with `scansPerDay`.
- **`scanBytes`** (integer) is on the playback object of a scan's answer
  only: the bytes that scan's ticket may read in all. Required with
  `purpose: scan`; 4,194,304 (4 MiB) to 1,073,741,824 (1 GiB). Missing or
  out of range: the caller counts on 4 MiB. On any other purpose:
  ignored.
- Only the owner knows which pool counted the scan, so the cap is the
  pool's: groups 9 lets a pool set its own scan cap, as it sets
  `bitrateKbps`.
- The owning server cuts reads at the answer's `scanBytes` as media 4.2
  says (a `206` ending at the cap, then `416`). Media 4 and 4.2,
  playback 5 and 8, and core 8.4's `416` row say "the scan answer's
  `scanBytes`".
- Normative text says only: `scan` for reads to list a file, `play` for
  reads to watch it, and no whole-file analysis of a peer's copies.
  Plex's sentences and the suggested 32 MiB go to the notes.

#### 9.5 Defaults

- Normative, unchanged: `local` titles, downloads and scans are **off by
  default**, allowed by the owner per pairing and per group.
- To the notes, as suggestions: 2 streams and 1 transcode; downloads 1
  at once, 10 a day, 20,000 kbit/s; scans 1,000 a day, 32 MiB each; the
  call rates of core 8.3; Overfin's 24 hours in playback 6.3.

### 10. Music (music 3, 3.1, 3.3, 5.2, 6)

- **Types**: `releaseType` goes; an album carries `primaryType` (string)
  and `secondaryTypes` (array, at most 8), MusicBrainz's release-group
  types as MusicBrainz names them, lowercased, open, each 1 to 32
  characters of `a–z 0–9 space . _ - /` (`audio drama`,
  `mixtape/street`). The registry lists them with this class.
  `compilation` (the tag) stays.
- **Track members** `artists`, `artistNames`, `date`, `originalDate`,
  `genres`: sent on a track only when they differ from its album's; an
  absent one is the album's.
- **`sortName`** goes.
- **Copies**: at most 16; `mediaType`, `codec` and `codecs` as
  section 3; `durationMs` exact; no `name`, `added` or `bitrateKbps`.
- **`ids`**: none on `local` titles; on `mb` titles, `mbreleasegroup`
  (albums) and `mbrecording` (tracks) only.
- **Queue entries**: in the request, each entry is `{"title",
  "versionId", "delivery"}`, `versionId` and `delivery` optional; an
  entry's `delivery` comes with its own `mediaType` or `transcode`
  targets as the request's would. Absent, an entry takes the request's
  (and without `versionId`, its first copy). The answer's entries as
  section 5.4.
- **Audiobooks**: `secondaryTypes` has `audiobook`, but a music copy
  carries no chapters, which an m4b has. Out of scope; music's open
  questions (music 9) note it.
- **Lyrics**: `offset`, `synced`, `displayArtist` and `displayTitle` go.
  A set whose every line carries `start` is synced; one where no line
  does is plain; a set mixing them is skipped. The owner applies any
  offset before sending.

### 11. Identity (catalog 4; video 4.3; music 3.1)

- As in the first version: the key schemes (`tmdb`, `tvdb`, `mb`,
  `local`) and the `ids` schemes are registry entries; unknown key
  schemes are skipped, unknown `ids` schemes ignored; `tmdb` is in
  `ids` for titles keyed under another scheme; one title may have two
  keys on two servers, joined by `ids`; "TMDB when it can, TVDB
  otherwise" stays a "should".
- New: on video, `ids` are sent only on episodes and on titles keyed
  under `tvdb`; not on films and series keyed under `tmdb`, nor on
  `local` titles. On music: as section 10.

### 12. The registry (spec/registry.md)

- **Scope: the lists that describe media**: codecs, subtitle formats,
  container media types IANA lacks (with aliases) and the brand table,
  `hdr`, `edition`, `extraType`, segment `kind`, audio and subtitle
  `role` (a pointer to the DASH scheme), `objects`, `threeD`, music
  types, and the key and `ids` schemes. Protocol mechanics (`method`,
  `protocol`, `delivery`, `purpose`, the subtitle `form`, `copy`,
  `range`, title kinds, group event and message kinds) stay closed, in
  their parts.
- **The token column is normative; the crosswalk is informative.** MP4RA,
  Matroska, IANA and ffmpeg names say where each token comes from, and
  name products only as sources. Borrowed lists say their origin
  (Jellyfin's ExtraType and MediaSegmentType, the DASH scheme,
  MusicBrainz).
- **A new entry** is added by a proposal giving the token, its field, its
  equivalents and its origin; a token is never reused for another
  meaning. A new codec is a registry entry, not a new version of a part.
  A fuller procedure may come later.
- The parts point to the registry and give examples only.
- The file is retitled "MSFP registry". Its first content is
  [0017-registry.md](0017-registry.md); the spec pass copies it.

### 13. New names

`mediaType` (copies, remux), `codecs` required for six codecs,
`fullRange`, `objects`, `refFrames`, `endMs`, `parts` as a count,
`previewLayout`, `ticket`, `protocol` at the answer's top level,
`subtitles` with `form` and `ass`, `fontCount`, `scanBytes` on scan
answers, `etag` in `previews`' answer, `copy` as a value of `videoCodec`
and `audioCodec`, the `range` target, subtitle `role`, `versionId` and
`delivery` on queue entries; in the registry, `sbs-rl`, `tab-rl` and
`video/vnd.dlna.mpeg-tts`. Kept from the first version:
`sampleAspect`, `rotation`, `el`, `primaryType`, `secondaryTypes`,
`themesong`, `themevideo`, `audioRenditions`.

## What it doesn't change

The core's signing, pairing, relations and key rotation; groups, but for
the names and the pool's scan cap; tickets and their lifetimes; byte
ranges and the file delivery; play on first read and restarts on the
same `etag`; complete VOD HLS and its tag subset; one playback per viewer
session; the WebVTT, ASS, PGS and font rebuilds; image re-encoding;
exact `size` and `etag`; `with` and `startMs`; chapters; the music queue
of 5; lyrics as their own call; downloads (24-hour ticket, speed cap,
late reports); the `previews` call's request; the tiled sheets; the
`tvdb` and `local` schemes and the `local` description; catalog paging.

## Alternatives considered

1. **Keep MSFP's lists and grow them per release.** Each new codec or
   product would cost a spec change and a release of every
   implementation: what the principle rejects.
2. **Open the capability values only.** Keeps the owner choosing for a
   device it can't see, and the coarsening.
3. **ffmpeg's names for everything** (this proposal's first version).
   They cover what media servers read, but tie the protocol to a
   product, and ffprobe gives no muxer name (its demuxers are lists:
   `matroska,webm`). No single outside vocabulary covers codecs and
   subtitles either: RFC 6381 misses ASS, PGS and VobSub, Matroska IDs
   miss AC-4 and Dolby Vision, IANA misses most audio and subtitle
   formats. Hence media types for containers, and registry tokens,
   spelled as ffmpeg spells them, for codecs.
4. **Names and titles with owner guidance** (the first version's
   section 3.6). Keeps the owner's labels, but their safety rests on
   each owner leaving file names out: rejected by the first of the three
   rules.
5. **Fetching every preview sheet ahead** (proposal 16). Previews from
   the first play, but every paired server downloads every library's
   previews whole.
6. **`codecs` optional everywhere.** Without it, a viewer's server can't
   tell a stream's profile and level, and plays what its device can't
   decode.

## Costs

- **Spec and vectors.** Text in the sections under Touches; the
  registry's tables; implementers.md's new notes. Vectors: `mediaType`
  instead of `container`; registry tokens; `codecs` required for six
  codecs; H.273 numbers and `fullRange`; `objects`, `refFrames`,
  `endMs`, `parts` as a count; the dropped fields gone; answers with
  `ticket`, `subtitles` and `fontCount`; `previewLayout`; `scanBytes` in
  scan answers; remux media types; the choice tables' vectors removed;
  `copy` and `range` in transcode requests, subtitle `role`, queue
  entries with their own copy and delivery, the Dolby Vision `codecs`
  and the required HEVC constraint bytes and long forms. High: the
  copies' cases touch many vectors.
- **Conformance suite** (on main). Checks that unknown tokens and media
  types are kept in facts and get `409` in delivery requests; that a
  remux names an `mp2t` or `mp4` media type; that `codecs` is required
  for the six; that answers carry no URLs, and that the owner serves the
  built names and `404`s past the counts; that scans stop at the
  answer's `scanBytes`; that `audioRenditions` matches the playlist;
  that a playback's ticket opens its copy's sheets; that the master
  playlist carries its required attributes and `mp4` segments `hvc1`;
  that a `copy` stream is unchanged and a `range` `sdr` transcode is SDR;
  that video 5.4's formats come in their forms. The choice checks go.
  Med.
- **msfp-go** (branch `revision`). On the first version's work: ffmpeg
  names out, `mediaType` and the registry's tables in; `codecs` checks;
  colour as numbers; the dropped fields; URLs built from the ticket (the
  URL checks become URL building); previews on demand (the sheet
  prefetch goes; a lazy sheet fetch with a cache keyed by `etag` and
  layout); the scan cap from the answer; the choice code out of the
  engine; `copy`, `range` and per-entry queue deliveries in requests;
  the master playlist's attributes checked. The copy-name and
  track-title filters go with the fields. Med to high.
- **msfp-dotnet** (branch `revision`). The same as msfp-go. No
  ffprobe-names helper; a small record parser if it helps hosts build
  `codecs`, since no maintained .NET library builds RFC 6381 strings.
  Med to high.
- **Overfin.** As an owning server: `mediaType` and registry tokens from
  its media server's names; a `codecs` builder in Go for the six codecs,
  its index keeping the level, codec tag, pixel format, reference frames
  and Dolby Vision fields its media server reports (about 150 lines);
  H.273 numbers; `previewLayout`; the scan cap per pool; the baseline's
  picture, a stream copied, tone mapping for `range` `sdr`, `hvc1` in
  `mp4` segments and the playlist attributes in its transcoder. As a
  viewer's server: always choose the copy and send `delivery`; build
  labels from facts; "recently added" from first sight; keep unknown
  values. Most of the rest is proposal 16's migration. Med.
- **Jellyfin plugin.** As an owning server: a `codecs` helper on
  Jellyfin's stream data, correcting its known gaps (H.264 High 10,
  4:2:2 and 4:4:4; HE-AAC v2) and using `hev1` when the codec tag says so
  (about 150 lines); its `NameOrOther` mapping removed;
  `audioRenditions: false`. As a viewer's server: parse `codecs` into
  Jellyfin's profile, level and codec tag so that its checks run (about
  100 lines); choose `delivery` from Jellyfin's device profile; map open
  values to Jellyfin's (unknown segment kinds to a plain skip); themes as
  Jellyfin theme songs and videos behind a setting; previews fetched for
  opened copies only; Jellyfin's DirectStream as a transcode with video
  `copy`, its HDR and Dolby Vision fallbacks through `range`; subtitle
  delivery methods from video 5.4's table before play; subtitle `role`
  into track labels. Med; most of the rest is proposal 16's migration.

The engine work the reviews list beyond this proposal (a `RemoteFile`
helper, .NET's headers and image encoder, peer-catalog helpers, API
cleanup) is not protocol; it goes in the same engine release. How an
engine answers `HEAD` on routes it didn't issue is an engine matter, not
decided here.

## Compatibility

None needed: version 0 is a draft and both implementations are the
owner's. This proposal and proposal 16 reach every implementation in one
release.

## History

- **First version** (2026-11-22): the owner's answers to the neutrality
  review (Q-1 to Q-9, P-1 to P-16): ffmpeg names, open lists, exact
  values, no capabilities, product text out, an owner-announced scan
  cap, 16 copies, 64 fonts, themes. Written into spec/v0.
- **First amendment** (2026-11-22): media types and the registry instead
  of ffmpeg's names, H.273 numbers, `objects`, a subtitle `kind`,
  previews fetched only when used, the scan cap in the scan's answer, 16
  music copies. Not written into spec/v0.
- **Reviews** (2026-11-22): an independent review of the amendment (35
  findings) and a second read (12), merged; the owner's answers B1 to
  B11 (no `+`, previews never for a whole library, `fullRange`, the
  alternative transfer, no `brand`, no subtitle `kind`, `dolby-atmos`
  and `dts-x`, no `level` or `layout`, every list of media in the
  registry). Then a field review against the three rules (answers on
  themes 1 to 11), research on `codecs` (required for six codecs,
  `refFrames` added) and the registry's first tables. This text replaces
  both earlier versions.
- **Viewer review** (2026-11-22): can a viewer's server still do
  everything a local title allows (25 findings: 2 blockers, 11 should
  fix, 12 minor). The owner's answers: `copy` for `videoCodec` and
  `audioCodec`; the baseline's picture defined and the `range` target;
  `role` on subtitle tracks, `forced` the only forced flag; queue
  entries with their own `versionId` and `delivery`; video 5.4's table a
  must. Its technical fixes applied: the Dolby Vision `codecs`, the
  chroma format and the ignored parts of `codecs`, audio-only extras,
  `hvc1` in `mp4` segments, the master playlist's attributes, the
  audio-or-video rule for the listed pairs, the narrower unknown-value
  rule, M2TS's media type, and the minor ones.
