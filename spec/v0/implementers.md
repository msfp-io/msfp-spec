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

(filled by its part)

## Media

(filled by its part)

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
