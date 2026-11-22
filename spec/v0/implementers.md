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
