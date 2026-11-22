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
