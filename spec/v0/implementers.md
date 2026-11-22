# MSFP version 0: notes for implementers

**Not normative.** Nothing here is a rule. The specification names no
product; this file does. It holds what the parts said of particular
media servers and implementations (Overfin, Jellyfin, Plex, Kodi and
others), suggested numbers, advice on mapping a media server's names to
the vocabularies MSFP uses, and advice to viewer's servers on choosing
what to play. An implementation that does otherwise still conforms, as
long as it follows the parts. It was made by
[proposal 17](../../proposals/0017-neutrality.md).

**The principle.** The protocol carries facts in outside vocabularies
(IANA media types, RFC 6381, ITU-T H.273, the DASH role scheme,
MusicBrainz) and, where none exists, in the
[MSFP registry](../registry.md), with open lists and one rule for values
a receiver doesn't know ([core section 8.5](core.md#85-unknown-values)).
Implementations map their media server's names to these; the protocol
doesn't map for them. A new media server or codec needs no change of the
specification; a new codec is a registry entry. ffmpeg's names are where
many registry tokens come from, not a vocabulary of the protocol.

What Overfin does is said of its version at the time of writing; its
code is the reference for what it does now.

## Contents

1. [Core](#core)
2. [Catalog](#catalog)
3. [Facts from a media server](#facts-from-a-media-server)
4. [Choosing, on the viewer's server](#choosing-on-the-viewers-server)
5. [Scans](#scans)
6. [Video](#video)
7. [Playback](#playback)
8. [Media](#media)
9. [Music](#music)

## Core

### Suggested numbers

The limits of [core section 8.3](core.md#83-limits-and-overload) are
each implementation's. These are suggestions, Overfin's (the Jellyfin
plugin uses the same call rates and requests at once); a server sets its
own to fit its machine:

| Limit | Suggested default |
|-------|-------------------|
| Request body | 64 KiB (core section 3) |
| Nonces remembered | 20,000 per known peer, 1,000 of strangers (core section 6.5) |
| Requests per network address, before any signature check | 50 a second, in bursts of up to 400 |
| Calls per peer (item 1) | 20 a second, in bursts of up to 200 |
| Calls of all peers together (item 2) | 200 a second, in bursts of up to 2,000 |
| Requests at once (item 3) | 8 per peer and 64 in all, each with a queue twice as long, waited in for 5 seconds at most |
| `play` per peer (item 4) | 30 a minute, in bursts of up to 20 |
| Playbacks per pairing | 2 streams, 1 of them transcoded ([playback section 6](playback.md#6-what-the-owning-server-enforces)) |
| Subtitle and playlist reads at once, on the viewer's server | 4 ([media section 2](media.md#2-requests)) |
| Preview sheet reads at once, on the viewer's server | 4 ([media section 3](media.md#3-what-is-fetched)) |

### Overfin's limits and retries

Overfin's numbers, fixed, with no settings:

| Limit | Overfin |
|-------|---------|
| Per network address, before any signature check | 50 a second, bursts of 400, an IPv6 `/64` counting as one address; past it, the unsigned `404` |
| Per peer | 20 calls a second, bursts of 200 |
| All peers together | 200 calls a second, bursts of 2,000 |
| `play` per peer | 30 a minute, bursts of 20 |
| Requests at once | 8 per peer (a queue of 16), 64 in all (a queue of 128); a queue is waited in for 5 seconds at most |

Past a peer's own limit it answers a signed `429` `rate-limited`, past
all peers' a signed `503` `server-busy`, each with `retryAfter` (the
seconds until a call would pass, at least 1) in the body and the same
`Retry-After` header. Behind its gateway, the gateway also limits
requests by network address, with the same numbers, and answers a
plain-text, unsigned `429` with `Retry-After: 5`. A single Overfin
counts by the address its `identity.trusted_proxies` setting gives
(`X-Forwarded-For` from those proxies only, read from the right). Behind
its gateway, the backend counts by the visitor's address the gateway
passes on, which the gateway takes from the PROXY protocol or
`X-Forwarded-For` of the proxies it is told to trust. Its `play` refusal
for transcodes, a signed `503` `server-busy`, carries no `retryAfter`.

As a caller, Overfin holds every call to a peer (but `pair`) until a
signed `retryAfter` has passed. After no answer, an unsigned refusal, or
a `429` or `503` without `retryAfter`, the calls it makes by itself
(`progress`, catalog reads, a waiting pairing's `hello`) hold off from 2
seconds, doubling, up to an hour, give or take a quarter; calls an
operator or a viewer asks for go when asked. A catalog read told to wait
up to a minute waits and reads on. A `play` held or refused this way
tells the viewer "busy now".

### Overfin's certificates and connections

Overfin reads its own certificate again for a `hello` when its last
reading is more than 2 seconds old
([core section 3.2](core.md#32-https-certificates)), and, while no
reading has ever worked, tries again after 5 seconds rather than a
minute.

Overfin refuses the table of [core section 3.3](core.md#33-connecting-to-peers),
by default, judged as that section says for IPv4-mapped, NAT64
(`64:ff9b::/96`, and `64:ff9b:1::/48` refused outright) and 6to4
addresses, and three cloud metadata addresses outside it
(`100.100.100.200`, `168.63.129.16`, `fd00:ec2::254`), on every
connection to a peer: calls, the media relay, preview sheets, the
`hello` that confirms an address change, and the connection to its own
address when it reads its own certificate. It checks at connection time
only. Its one setting, `federation.allow_loopback`, allows loopback and
nothing else. It offers no setting to refuse private ranges. A backend
behind Overfin's gateway connects through the gateway, which makes the
same check. It uses no proxy from the environment.

### Media answers and logging

As the viewer's server, Overfin answers a peer's `404` to a media fetch
([core section 8.4](core.md#84-media-answers)) with a `404` to the
viewer, for every kind of fetch, and doesn't end the playback itself:
the player stops on its own, and the playback ends by `stop` or, on both
sides, by expiry ([playback section 6.3](playback.md#63-how-a-playback-ends)).

Overfin logs none of the secrets of
[core section 14.1](core.md#141-logging). A failed media fetch has the
URL removed from its error before it is logged, and a crash in a request
logs the route's pattern, not the path.

### A media server under a base path

A media server reached under a path (Jellyfin at `/jellyfin`, say) has a
prefix, and needs the discovery document at its address's root
([core section 11](core.md#11-the-discovery-document)): its reverse proxy
serves it there when the media server serves nothing at the root.

## Catalog

### Library types

A library carries no kind ([catalog section 2](catalog.md#2-libraries)):
the keys of its titles say what it holds. A viewer's server whose media
server wants a type for each library (Jellyfin's collection types
`movies`, `tvshows`, `music`, `mixed`, say) works it out from the
titles: films only, `movies`; series only, `tvshows`; music only,
`music`; several, `mixed`. An empty library is typed by its first
titles.

### Other identifiers

A film or a series keyed under `tmdb` carries no `ids`
([catalog section 4.1](catalog.md#41-same-title-same-key)): a viewer's
server that wants its IMDb or TheTVDB ID asks TMDB for the title's
`external_ids`, once per title, kept. For a music track keyed under
`mb`, MusicBrainz gives the recording's ISRCs from `mbrecording`, and the
AcoustID service its AcoustIDs.

### Language codes

Media servers often give a track's language as the file says it: an ISO
639-2/B code (`fre`, `ger`, `chi`) or a 639-2/T one (`fra`, `deu`,
`zho`). [Catalog section 3](catalog.md#3-titles) has them sent as the
ISO 639-1 code (`fr`, `de`, `zh`); the cases of `languageCodes` in
[vectors/v0/strict.json](../../vectors/v0/strict.json) cover the common
ones.

### Schemes

The schemes are in the [registry](../registry.md). A server that holds a
series under TheTVDB's ID only (anime, often) keys it under `tvdb`, and
names its TMDB ID in `ids.tmdb` once it knows it; a viewer's server that
holds the same show under `tmdb` then joins the two
([catalog section 4.1](catalog.md#41-same-title-same-key)).

### Decoding safely

Images from a peer (the image of a `local` title,
[catalog section 4.4](catalog.md#44-the-image-of-a-local-title-image);
preview sheets and pictures in the content parts) are untrusted input,
handed to image decoders that have a long history of flaws. Advice:

- **Check before decoding.** Read the type and the size from the header
  (a PNG's IHDR, a JPEG's frame header), by the content, whatever the
  `Content-Type`, and refuse what is out of bounds before any decoder
  sees it. The checks of catalog section 4.4 are written so that a
  header alone decides.
- **The JPEG DNL marker.** A JPEG frame header may give a height of 0,
  leaving the height to a DNL marker after the first scan. Few files use
  it, and decoders handle it unevenly: the height isn't known when the
  buffers are sized. The catalog's bounds (1 to 4,096 pixels, from the
  header) refuse such a file; an implementation should not make an
  exception for it.
- **Bound the decoder.** Decode with a cap on pixels and memory, in a
  decoder kept up to date, and, where the platform allows, in a separate
  process.
- **Re-encode.** Serve your people a copy you encoded yourself, in the
  same format, with no metadata carried over (EXIF, comments, colour
  profiles beyond what the picture needs): only the pixels reach them.
- **Never pass the peer's bytes on**, and never show the peer's URL.

## Facts from a media server

How an owning server turns what its media server or ffprobe reports
into the facts the parts ask for. The tokens are the
[registry](../registry.md)'s; its crosswalk gives the names other
sources use.

### Colour: ffmpeg's names to H.273

`colour` carries ITU-T H.273 code points
([video section 3.3](video.md#33-streams-and-tracks)). ffprobe prints
ffmpeg's names for them (`color_primaries`, `color_transfer`,
`color_space`); each name stands for one code point. `unknown`,
`unspecified` and `reserved` are left out, as is any value 2.

| ffmpeg's name | H.273 | Field |
|---------------|-------|-------|
| `bt709` | 1 | primaries, transfer, matrix |
| `bt470m` | 4 | primaries, transfer (also read `gamma22`) |
| `bt470bg` | 5 | primaries, transfer (also read `gamma28`), matrix |
| `smpte170m` | 6 | primaries, transfer, matrix |
| `smpte240m` | 7 | primaries, transfer, matrix |
| `film` | 8 | primaries |
| `bt2020` | 9 | primaries |
| `smpte428` (also read `smpte428_1`) | 10 | primaries |
| `smpte431` | 11 | primaries |
| `smpte432` | 12 | primaries |
| `jedec-p22` (also read `ebu3213`) | 22 | primaries |
| `linear` | 8 | transfer |
| `log100` (also read `log`) | 9 | transfer |
| `log316` (also read `log_sqrt`) | 10 | transfer |
| `iec61966-2-4` | 11 | transfer |
| `bt1361e` | 12 | transfer |
| `iec61966-2-1` (also read `srgb`) | 13 | transfer |
| `bt2020-10` | 14 | transfer |
| `bt2020-12` | 15 | transfer |
| `smpte2084` | 16 | transfer (PQ, HDR10's) |
| `smpte428` | 17 | transfer |
| `arib-std-b67` | 18 | transfer (HLG) |
| `gbr` (also read `rgb`) | 0 | matrix |
| `fcc` | 4 | matrix |
| `ycgco` | 8 | matrix |
| `bt2020nc` | 9 | matrix |
| `bt2020c` | 10 | matrix |
| `smpte2085` | 11 | matrix |
| `chroma-derived-nc` | 12 | matrix |
| `chroma-derived-c` | 13 | matrix |
| `ictcp` | 14 | matrix |
| `ipt-c2` | 15 | matrix |
| `ycgco-re` | 16 | matrix |
| `ycgco-ro` | 17 | matrix |

ffprobe prints the first name; the others are spellings ffmpeg also
reads. From libavutil's `pixfmt.h` and the names `av_color_*_name`
gives; a name not here is mapped by the same file. ffprobe's `color_range` is
`tv` for limited range (`fullRange: false`) and `pc` for full range
(`fullRange: true`); `unknown` leaves `fullRange` out. ffprobe reads
the VUI, and the container's colour where the stream has none; for the
alternative transfer characteristics SEI (broadcast HLG often signals
14 in the VUI and 18 there), an owning server reads the stream's side
data or the SEI itself.

### Building `codecs`

`codecs` is the stream's RFC 6381 string, required for `h264`, `hevc`,
`av1` and `vp9` on video and `aac` and `dts` on audio
([video section 3.3](video.md#33-streams-and-tracks)). Two ways to
build it.

**From the file's records** (exact). In MP4 the sample entry names the
4CC and its box holds the rest: `avcC` (profile, constraint flags and
level, its bytes 1 to 3), `hvcC` (profile space, tier, profile,
compatibility flags, constraint bytes, level), `av1C` (profile, level,
tier, bit depth, monochrome, chroma subsampling), `vpcC` (profile,
level, bit depth, chroma subsampling, colour), `esds` (the object type
and, for AAC, the audio object type of its AudioSpecificConfig). A
Matroska file's CodecPrivate holds the same records for H.264, HEVC and
AV1, and the AudioSpecificConfig for AAC; VP9's CodecPrivate holds its
profile, level, bit depth and chroma subsampling when the muxer wrote
them, else the first frame's header gives them. On Matroska or MPEG-TS
the string is the one the stream would have in ISO BMFF: `avc1`,
`hvc1`, `av01`, `vp09`, `mp4a`. A small parser of these records is
enough; few libraries build RFC 6381 strings.

**From ffprobe** (`-show_streams`), where the records aren't read:

| Codec | String | From ffprobe |
|-------|--------|--------------|
| `h264` | `avc1.PPCCLL`, hex | `profile` to `profile_idc`: Baseline and Constrained Baseline 66 (`42`), Main 77 (`4D`), Extended 88 (`58`), High 100 (`64`), High 10 110 (`6E`), High 4:2:2 122 (`7A`), High 4:4:4 Predictive 244 (`F4`); `CC`, the constraint flags, `00` when unknown (Constrained Baseline: `40`); `level` is `level_idc` (41 is `29`) |
| `hevc` | `hvc1.P.F.TL.C` | `profile` to `general_profile_idc` (Main 1, Main 10 2, Main Still Picture 3, Rext 4); `F`, the compatibility flags, only the profile's own bit when unknown (`2` for Main, `4` for Main 10, `10` for Rext); `T` is `L` when the tier is unknown; `level` is `general_level_idc` (`L153`); the constraint bytes (`B0` on most streams), required from profile 4 on, come only from the records |
| `av1` | `av01.P.LLT.DD` | `profile` (Main 0, High 1, Professional 2), `level` (`seq_level_idx`, two digits), tier `M` when unknown, bit depth from `pix_fmt` (`08`, `10`, `12`); a stream that isn't 4:2:0 takes the long form, with its chroma subsampling (`.0.110` is 4:2:0, `.0.100` 4:2:2, `.0.000` 4:4:4) and its colour |
| `vp9` | `vp09.PP.LL.DD` | `profile` (`00` to `03`); the level, which ffprobe doesn't give, guessed from size and frame rate; bit depth from `pix_fmt`; a stream that isn't 4:2:0 takes the long form, with chroma subsampling `02` (4:2:2) or `03` (4:4:4) |
| `aac` | `mp4a.40.A` | `profile` to the audio object type: LC 2, HE-AAC 5, HE-AACv2 29, LD 23, ELD 39, xHE-AAC 42 |
| `dts` | `dtsc`, `dtsh`, `dtsl`, `dtse` | `profile`: DTS, DTS-ES and DTS 96/24 are `dtsc`; DTS-HD HRA and DTS-HD MA (with a core) `dtsh`; DTS Express `dtse`; a lossless stream with no core (Matroska's `A_DTS/LOSSLESS`) `dtsl` |

`codec_tag_string` tells `avc1` from `avc3` and `hvc1` from `hev1` in
MP4; elsewhere `avc1` and `hvc1` are sent. Media servers that keep less
(Jellyfin keeps a profile name and a level) have known gaps: H.264
High 10, 4:2:2 and 4:4:4, and HE-AAC v2, need the pixel format or the
channel layout to name the right profile. On Dolby Vision, `codecs` is
the base codec's, with the base 4CC of the file's Dolby Vision entry
(the [registry](../registry.md)'s table).

### Atmos and DTS:X: `objects`

Where each codec stores the object layer, for `objects`
([video section 3.3](video.md#33-streams-and-tracks)):

- **E-AC-3** (Dolby Atmos, JOC): `flag_ec3_extension_type_a` in the
  bitstream's `addbsi`; in MP4 the same flag in `dec3`. ffprobe prints
  the profile "Dolby Digital Plus + Dolby Atmos".
- **TrueHD** (Dolby Atmos): 4 substreams and the top bit of
  `substream_info` set in the major sync. ffprobe: "Dolby TrueHD + Dolby
  Atmos".
- **DTS-HD MA** (DTS:X): an XLL payload with the sync word `0x02000850`
  (DTS:X) or `0xF14000D0` (IMAX Enhanced). ffprobe: "DTS-HD MA + DTS:X"
  and "DTS-HD MA + DTS:X IMAX".

A viewer's server whose players want Atmos in E-AC-3 named in the
codecs string (Apple's players read `ec+3`) writes `ec+3` itself from
`codecs` `ec-3` and `objects` `dolby-atmos`: `codecs` never carries a
`+`.

### Track titles to roles and flags

Tracks carry no titles. An owning server reads the flags first
(Matroska's FlagCommentary, FlagVisualImpaired, FlagHearingImpaired,
FlagForced and FlagOriginal; MP4's `kind` boxes), and only then, where
a flag is missing, a track's title:

| A title such as | Sent |
|-----------------|------|
| "Commentary", "Director's commentary" | `role` `commentary` |
| "Audio description", "AD", "Descriptive" | audio `role` `description` |
| "SDH", "CC", "Hearing impaired" | `hearingImpaired` `true` (never a `caption` role) |
| "Forced", "Foreign parts only" | `forced` `true` (never a `forced-subtitle` role) |
| "Easy reader" | subtitle `role` `easyreader` |
| anything else (a release group, a language name, "English") | nothing |

When it can't tell, it leaves `role` out: absent is the title's own
sound.

### A stream's bitrate

No stream carries its bitrate. The copy's overall rate, `size` × 8 /
`durationMs`, in kbit/s, bounds every stream in it: a viewer's server
that needs a bitrate for a stream (a device's cap, a transcode's
target) uses it. On music it runs high when the file holds cover art.

## Choosing, on the viewer's server

The viewer's server chooses the copy and the tracks
([playback section 2.6](playback.md#26-choosing-the-copy-and-the-tracks)).
What follows were once the owning server's recommended defaults; they
are advice for the viewer's server now.

### The order of copies

The owning server lists a title's copies in its order of preference,
best first, and the first plays when `play` names none. A reasonable
order: the copy of the greatest height first (for a picture wider than
16:9, the height of a 16:9 picture as wide: a 3840 × 1600 film counts
as 2160), then one with a high dynamic range before one without
(`colour.transfer` 16 or 18, or a `dv` object), then the one with more
`hdr` entries; between copies equal on all three, the server's own
order. A copy without `video` comes after those with one.

### The copy and the audio

A viewer's server picks, among the copies its device can play as they
are, or the ones it will ask to convert:

- **Languages.** Match the same tag first (`fr-CA` for `fr-CA`), then
  the same base language (`fr` for `fr-CA`), without case; look for the
  same tag in every copy before the base language in any (with `fr-CA`
  asked, a later copy with `fr-CA` wins over an earlier one with `fr`).
  For audio, match only the title's own sound: a track with no `role`,
  or `main` or `dub`; never a commentary, a description or a role it
  doesn't know.

| Wanted | Copy |
|--------|------|
| a language a copy's audio matches | the first copy whose audio matches |
| none, or no copy matches | the first copy |

| Wanted | Audio |
|--------|-------|
| a language a track matches | that track |
| none, or no track matches | the default audio: the track the file marks default, else its first audio track |

It names both in the request (`versionId`, `audioTrack`), and shows
what the answer says plays.

### The subtitles

| Wanted | Subtitles |
|--------|-----------|
| a track the viewer chose | that track, or none for "off" |
| a language a regular track matches | that track (a regular one before a forced one) |
| nothing, or no track matches | none, but the forced track below |

Whatever is shown, the forced track in the language of the audio
played, when the copy has one, is shown too. A track is fetched in the
form the owning server serves it apart
([video section 5.4](video.md#54-choosing-the-subtitles)), or burned in
only when the viewer's device can show none of its forms.

## Scans

- **Plex as a viewer** reaches a peer's copies only through scan access
  ([playback section 8](playback.md#8-scan-access)): it lists files by
  reading them through a file system, then plays them.
- **The cap.** The scan's answer states how many bytes its ticket may
  read (`scanBytes`, 4 MiB to 1 GiB). Media servers read 1 to 4 MiB of a
  file to list it, but a 4 MiB cap misses an MP4's `moov` box or
  Matroska's cues written at the end of a big file: a pool for scans is
  better set to 32 MiB.

## Video

Notes for implementers of [video](video.md), moved out of its normative
sections. Nothing here is a rule.

**Filling `copies`** ([video section 3.2](video.md#32-copies)). Overfin,
as an owning server, fills `copies` from its index, which keeps each
version's streams, length and size as its media server reports them, and
lists a title's versions in the recommended order of
[video section 3.7](video.md#37-the-order-of-copies). A media server
that names containers and codecs otherwise is mapped to ffmpeg's names
([video section 3.5](video.md#35-names)) by the owning server.

**Skip markers from other media servers**
([video section 3.4](video.md#34-chapters-and-skip-markers)). Credits
map to `outro`. A marker whose end the owning server doesn't know
(credits that run to the end) can end at the copy's `durationMs`. Some
media servers count a title as watched at the start of its last credits:
that is the marker to send `final` on.

**Theme songs and theme videos**
([video section 3.9](video.md#39-extras)). Some media servers play them
in the background each time a title's page opens. Played from a peer,
each of those would be a playback on the owning server, counted against
the pairing's `streams`. A viewer's server that offers them may play them
only when a viewer asks, or leave them out; an owning server may keep
them behind a switch of the operator's.

**Finding TMDB's numbers** ([video section 4.1](video.md#41-episode-order)).
An owning server whose files are numbered another way may find an
episode's TMDB numbers from its IMDb or TheTVDB ID: TMDB's lookup by
external ID gives the series, the season number and the episode number.
Once per episode, kept.

**Seek-bar previews** ([video section 5.5](video.md#55-seek-bar-previews)).
A media server that keeps one image per interval (BIF files) has them
tiled into sheets by the owning server. A set near 320 pixels wide is a
good choice.

**Subtitles** ([video sections 6.3 to 6.5](video.md#63-webvtt)). The
WebVTT, ASS and PGS rules are Overfin's, as both owning and viewer's
server, an empty PGS part included; Overfin converts DVD and DVB picture
subtitles to PGS as section 6.5 says.

## Playback

Notes for [playback](playback.md), not normative.

**Audio roles.** An owning server whose media server doesn't flag a
track's role may guess it from the track's title ("Commentary",
"Audio description"), and leaves `role` out when it can't tell
([playback section 2.3](playback.md#23-the-answer)).

**Overfin's requests.** Since 2026-11-16, Overfin compared each entry of
the capabilities a peer sent exactly, so `H264` was an unknown entry
(before, it lowercased them); capabilities went with proposal 17. Its
own apps' API, which is not MSFP, still lowercases. Since 2026-11-16 too,
an `audioTrack` or `subtitleTrack` that isn't a track of the copy counts
as absent (before, the audio track was passed to its media server as is,
and the subtitle track echoed)
([playback section 2.6](playback.md#26-choosing-the-copy-and-the-tracks)).

**Transcode budgets.** Jellyfin starts as many transcodes as it is
asked: an owning server on it makes a budget of its own, an operator
setting counting its own people's transcodes and its peers' together
([playback section 6](playback.md#6-what-the-owning-server-enforces)).

**A remux that re-encodes.** An owning server can tell that a remux
re-encodes the video when its media server names the encoder of the job.
One that can't tell asks its media server for a remux only when the
media server copies the video.

**Suggested numbers.** A reasonable start for a new pairing: 2 streams,
1 transcode, no bitrate cap, no downloads and no scan access; the
operator changes them, per pairing and per group. An operator who allows
downloads may start from 1 at once, 10 a day and 20,000 kbit/s; one who
allows scans, from 1,000 a day and a `scanBytes` of 32 MiB
([playback section 5](playback.md#5-limits)).

**Overfin's playbacks.** Overfin's longest playback is 24 hours
([playback section 6.3](playback.md#63-how-a-playback-ends)). Its check
and count are one step under one lock (atomic), and it checks the
pairing's streams once more before asking its media server, to refuse
early. It looks for expired playbacks every 30 seconds. A pairing that
ends (by either side), a peer found no longer paired, or a change of
what it offers a peer ends that peer's playbacks at the moment of the
change, open media responses included; each media request checks again.
After a playback ends, it remembers why for an hour; `progress` gets
`410` either way.

**Plex and scan access.** Plex, through a file system, reads 1 to 4 MiB
of a file to list it ([playback section 8](playback.md#8-scan-access)).
Its "extensive media analysis" must be off for a library of a peer's
copies: tested, it reads every file whole, and when a read is refused,
overwrites the file's length with what it read.

## Media

Notes for [media](media.md), not normative.

**Overfin's bounds.** Overfin follows
[media sections 2.1 to 2.4](media.md#2-requests) on a peer's ticket. As
the viewer's server, it gives a fetch 60 seconds to its status line and
headers, and 10 seconds each to connect and to finish TLS. It holds at
most 4 playlist and subtitle reads in memory at once across the server,
and 4 sheet reads. It runs 8 fetches at once per playback and 64 per
peer, on both sides, each with a queue as long, waited in for 10
seconds at most, then `503`; a playlist, a subtitle part, a PGS part or a
sheet is read whole within 2 minutes. The same stream route serves its
own people's browsers, on tickets of their own: there it answers `HEAD`
as `GET`, and passes on any single range its media server answers.
These notes describe Overfin before the revision of
[proposal 16](../../proposals/0016-version-0-revision.md): `405` to a
peer's `HEAD`, the file's headers and the new sizes are still to build.

**Overfin's HLS.** As the owning server, Overfin sends exactly the subset
of [media section 3.1](media.md#31-hls-playlists), without `#EXT-X-MEDIA`
so far. It rewrites its media server's HLS: master playlists name
`v<n>.m3u8`, media playlists `init.<ext>` (in `#EXT-X-MAP`) and
`seg<n>.<ext>`, where *n* may be negative and *ext* is `mp4`, `m4s`,
`ts`, `aac` or `mp3`; it keeps a segment's duration in `#EXTINF` but not
its title, drops every other tag and any tag holding `URI=` in any case,
and refuses a playlist of its media server with any other URI. As the
viewer's server, it checks steps 1 to 8, and drops tags outside the
subset, and comments, before serving. It fetches any bare name the
viewer asks for. Steps 9 and 10, and playlists complete from the first
fetch, are new with
[proposal 16](../../proposals/0016-version-0-revision.md) and still to
build.

## Music

Notes for [music](music.md), not normative.

**Tag names.** Tagging tools (Picard, beets) write the recording MBID in
the tag named "MusicBrainz Track Id", and the release track's in
"MusicBrainz Release Track Id": a track key takes the second, `ids`'
`mbrecording` the first. Media servers don't always name their fields
after the tags. Jellyfin names them after MusicBrainz's meaning:
`MusicBrainzTrack` holds the release track, from "MusicBrainz Release
Track Id", and `MusicBrainzRecording` the recording, from "MusicBrainz
Track Id" (checked in its source, 2026-11-21). Navidrome's
`MbzReleaseTrackID` and `MbzRecordingID` read the same way. Jellyfin
keeps only the first MBID of a multi-valued tag, so a Jellyfin-based
owning server gives one artist key per track.

**The `compilation` tag** is iTunes' `cpil` and ID3's `TCMP`.

**MusicBrainz types.** MusicBrainz names them with capitals (`Album`,
`Audio drama`, `Mixtape/Street`); an owning server sends them
lowercased.

**A track's members** (`artists`, `artistNames`, dates, genres): an
owning server compares each with its album's and leaves out those that
are equal. Most tracks of an album then carry none; a compilation's
carry their own artists.

**Media types and codecs.** A track's `mediaType` is read from the
file's first bytes where the owning server can (`fLaC` gives
`audio/flac`, `OggS` `audio/ogg`, an ISO BMFF `ftyp` such as `M4A `
`audio/mp4`), else mapped from its media server's container name
through the registry. ffprobe's `format_name` is a demuxer list
(`mov,mp4,m4a,3gp,3g2,mj2`), not a media type. `codec_name` gives the
codec token as ffprobe prints it. For AAC, ffprobe's `profile` (`LC`,
`HE-AAC`, `HE-AACv2`) gives `codecs`' object type: `mp4a.40.2`,
`mp4a.40.5`, `mp4a.40.29`.

**The overall rate** (`size` × 8 / `durationMs`) of a track counts its
embedded cover art: a FLAC file with a large picture shows a higher
rate than its audio has.

**R128 to ReplayGain.** An EBU R128 gain (Opus's `R128_TRACK_GAIN`,
relative to −23 LUFS, in 1/256 dB) converts to ReplayGain 2.0's
reference as the value over 256, plus 5 dB.

**Gapless playback** needs a format whose padding the player knows: the
file as it is (FLAC, Opus, MP3 or AAC with its encoder delay tags),
rather than AAC converted track by track, which usually adds a short
silence at its start. With a queue, a viewer's server can ask `file`
for the tracks its player plays and a conversion for the others only
(music section 5.2).

**Album art.** A public source by MusicBrainz ID serves it, such as the
Cover Art Archive: a release's cover at `/release/<mbid>/front` and a
release group's at `/release-group/<mbid>/front`, in sizes 250, 500 and
1200 pixels. It states no rate limit (checked 2026-11-17) but may
answer `503`.

**Lyrics.** OpenSubsonic's `structuredLyrics` also has `synced`,
`offset`, `displayArtist` and `displayTitle`, which MSFP doesn't send. An
owning server applies an offset (an LRC `[offset:]` tag, or
OpenSubsonic's `offset`) to every `start` before sending; a viewer's
server built on OpenSubsonic sets `synced` from the lines' `start`, and
the display names from the track. Jellyfin keeps lyric times as
100-nanosecond ticks: divide by 10,000 for milliseconds.
