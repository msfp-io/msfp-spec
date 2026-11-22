# MSFP version 0: notes for implementers

**Not normative.** Nothing here is a rule. The specification names no
product; this file does. It holds what the parts said of particular
media servers and implementations (Overfin, Jellyfin, Plex, Kodi and
others), suggested numbers, and advice on mapping a media server's names
to the vocabularies MSFP uses. An implementation that does otherwise
still conforms, as long as it follows the parts. It was made by
[proposal 17](../../proposals/0017-neutrality.md).

**The principle.** The protocol carries facts in outside vocabularies
(ffmpeg's names, RFC 6381, ITU-T H.273, the DASH role scheme,
MusicBrainz, IANA media types) and open lists, with one rule for values
a receiver doesn't know ([core section 8.5](core.md#85-unknown-values)).
Implementations map their media server's names to these; the protocol
doesn't map for them. A new media server, codec or kind of library needs
no change of the specification.

What Overfin does is said of its version at the time of writing; its
code is the reference for what it does now.

## Contents

1. [Core](#core)
2. [Catalog](#catalog)
3. [Video](#video)
4. [Playback](#playback)
5. [Media](#media)
6. [Music](#music)

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

### Library kinds

A library's `kind` is open
([catalog section 2](catalog.md#2-libraries)): a server sends its media
server's kind, as a token. The examples the part gives (`movies`,
`tvshows`, `music`, `mixed`) and the other kinds version 0 listed before
it was opened (`musicvideos`, `homevideos`, `boxsets`, `books`,
`photos`) are Jellyfin's collection types, which a Jellyfin server can
send as they are. A media server with other names maps them to these
where they mean the same (a library of films to `movies`, of series to
`tvshows`, of music to `music`), and sends its own token otherwise. A
caller that doesn't know a kind shows the library under a generic label
and reads its titles by the kinds of their keys.

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

## Video

Notes for implementers of [video](video.md), moved out of its normative
sections. Nothing here is a rule.

**Filling `copies`** ([video section 3.2](video.md#32-copies)). Overfin,
as an owning server, fills `copies` from its index, which keeps each
version's streams, length and size as its media server reports them. A
media server's container and codec names are mapped by the owning server
to media types and the [registry](../registry.md)'s tokens
([video section 3.5](video.md#35-names)); a producer reading ffprobe
gets the codec and subtitle tokens as they are, from `codec_name`.

**The order of copies** ([video section 3.7](video.md#37-the-order-of-copies)).
A reasonable order, for an owning server, and for a viewer's server
choosing without a better reason: the copy of the greatest height first
(for a picture wider than 16:9, the height of a 16:9 picture as wide: a
3840 × 1600 film counts as 2160), then one with a high dynamic range
before one without (a `colour.transfer` of 16 or 18, or a non-empty
`hdr`), then the one with more `hdr` entries; a copy without `video`
after those with one.

**Building `codecs`** ([video section 3.5](video.md#35-names)). ffprobe
prints no RFC 6381 string: a producer builds it from the stream's
fields, or from the file's own records (`avcC`, `hvcC`, `av1C`, `vpcC`,
`esds`, `ddts`; in Matroska, the same records in `CodecPrivate`), which
are exact.

- H.264: `avc1.PPCCLL` (`avc3` when the file's sample entry is
  `avc3`): `profile_idc`, the constraint flags (zero when unknown) and
  `level_idc`, in hexadecimal. ffprobe's `level` 41 is `29`; High is
  `64`, High 10 `6E`, High 4:2:2 `7A`, High 4:4:4 `F4`, Main `4D`,
  Constrained Baseline `42E0`.
- HEVC: `hvc1` or `hev1`, as the sample entry (ffprobe's
  `codec_tag_string`) says, then `general_profile_idc`, the
  compatibility flags in reverse bit order (only the profile's own bit
  when unknown: Main `2`, Main 10 `4`), the tier and `general_level_idc`
  (`L153` for level 5.1, Main tier), and the constraint bytes (`B0` for
  most progressive streams), required for range extensions (profile 4
  and above), where they carry the chroma format.
- AV1: `av01.P.LLT.DD`: the profile, `seq_level_idx` on two digits, the
  tier (`M` or `H`) and the bit depth on two digits; for a stream that
  isn't 4:2:0, the long form with its mono flag and chroma subsampling.
- VP9: `vp09.PP.LL.DD`, and the long form for profiles 1 and 3 (4:2:2,
  4:4:4). ffprobe gives no VP9 level: a guess from the picture's size and
  frame rate is enough, since consumers ignore it.
- AAC: `mp4a.40.<object type>`: LC `2`, HE-AAC `5`, HE-AAC v2 `29`, LD
  `23`, ELD `39`, xHE-AAC `42`. Media servers often report HE-AAC v2 as
  HE-AAC: the record tells.
- DTS: `dtsc` for core, ES and 96/24, `dtsh` for DTS-HD (High Resolution,
  or Master Audio with a core), `dtsl` for Master Audio without a core,
  `dtse` for Express.

**Colour from ffmpeg's names** ([video section 3.6](video.md#36-colour-and-dynamic-range)).
ffprobe prints H.273's values by name. `color_primaries`: `bt709` 1,
`bt470m` 4, `bt470bg` 5, `smpte170m` 6, `smpte240m` 7, `film` 8, `bt2020`
9, `smpte428` 10, `smpte431` 11, `smpte432` 12, `jedec-p22` 22.
`color_transfer`: `bt709` 1, `gamma22` 4, `gamma28` 5, `smpte170m` 6,
`smpte240m` 7, `linear` 8, `log100` 9, `log316` 10, `iec61966-2-4` 11,
`bt1361e` 12, `iec61966-2-1` 13, `bt2020-10` 14, `bt2020-12` 15,
`smpte2084` 16, `smpte428` 17, `arib-std-b67` 18. `color_space` (the
matrix): `gbr` 0, `bt709` 1, `fcc` 4, `bt470bg` 5, `smpte170m` 6,
`smpte240m` 7, `ycgco` 8, `bt2020nc` 9, `bt2020c` 10, `smpte2085` 11,
`chroma-derived-nc` 12, `chroma-derived-c` 13, `ictcp` 14. `unknown`
(2) isn't sent. `color_range` `tv` is `fullRange` false, `pc` true.
ffprobe reports the VUI's transfer: for broadcast HLG, read the
alternative transfer characteristics SEI too (its side data), which
holds 18 where the VUI holds 14.

**Object audio** ([video section 3.3](video.md#33-streams-and-tracks)).
Where each codec stores it: Dolby Atmos in E-AC-3 is the JOC flag
(`flag_ec3_extension_type_a` in `addbsi`, and the same flag in MP4's
`dec3`); in TrueHD, four substreams and the top bit of `substream_info`
in the major sync. DTS:X in DTS-HD Master Audio is an extension
substream payload with the sync word `0x02000850` (`0xF14000D0` for IMAX
Enhanced). Media servers often show these in a profile string ("Dolby
TrueHD + Dolby Atmos", "DTS-HD MA + DTS:X"). Some players want `ec+3`
rather than `ec-3` for E-AC-3 with Atmos: a viewer's server writes it
from `ec-3` and `objects` `dolby-atmos` for them.

**Track titles** ([video section 3.3](video.md#33-streams-and-tracks)).
Tracks carry no title. An owning server maps what a good title says to
the facts: "Commentary" to `role` `commentary`, "Audio description" to
`description`, "SDH" or "CC" to `hearingImpaired`, "Forced" to `forced`
when the file doesn't flag it. A title that is only the language's name,
or a release group's, says nothing to map.

**A stream's bitrate.** No stream carries its own: the copy's overall
rate (`size` × 8 / `durationMs`) bounds every stream's. On music it is
inflated by cover art.

**Skip markers from other media servers**
([video section 3.4](video.md#34-chapters-and-skip-markers)). Credits
map to `outro`. A marker whose end the owning server doesn't know
(credits that run to the end) can end at the copy's `durationMs`. Some
media servers count a title as watched at the start of its last credits:
a viewer's server finds them as the last `outro`.

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

**Choosing the subtitles** ([video section 5.4](video.md#54-choosing-the-subtitles)).
A reasonable default for a viewer's server: a track the viewer chose
wins, and "off" shows none; otherwise, a regular track in the viewer's
subtitle language when there is one, else none; whatever is chosen, the
forced track in the language of the audio played, when the copy has one.
When the viewer's language matches both a regular track and a forced
one, the regular one is shown.

**Seek-bar previews** ([video section 5.5](video.md#55-seek-bar-previews)).
A media server that keeps one image per interval (BIF files) has them
tiled into sheets by the owning server. A set near 320 pixels wide is a
good choice: some 30 to 300 KB an episode. An owning server that keeps
its re-encoded sheets doesn't encode them again for each peer.

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

**Container and codec names.** ffprobe's `format_name` is the demuxer's
list (`mov,mp4,m4a,3gp,3g2,mj2`), not a muxer name: the owning server
sends the one muxer name that fits the file (`mp4` for an MP4 or .m4a
file). `codec_name` is sent as ffprobe prints it.

**R128 to ReplayGain.** An EBU R128 gain (Opus's `R128_TRACK_GAIN`,
relative to −23 LUFS, in 1/256 dB) converts to ReplayGain 2.0's
reference as the value over 256, plus 5 dB.

**Gapless playback** needs a format whose padding the player knows: the
file as it is (FLAC, Opus, MP3 or AAC with its encoder delay tags),
rather than AAC converted track by track, which usually adds a short
silence at its start.

**Album art.** The Cover Art Archive serves a release's cover at
`/release/<mbid>/front` and a release group's at
`/release-group/<mbid>/front`, in sizes 250, 500 and 1200 pixels. It
states no rate limit (checked 2026-11-17) but may answer `503`.

**Lyrics times.** Jellyfin keeps lyric times as 100-nanosecond ticks:
divide by 10,000 for milliseconds.
