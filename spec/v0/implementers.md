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
