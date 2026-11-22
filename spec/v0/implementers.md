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
