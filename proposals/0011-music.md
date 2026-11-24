# Proposal 11: Music

Status: Adopted (2026-11-17), open points still open. Decided: identity (section 1),
descriptions (section 2), pictures (section 4), playing a track
(section 5) and the library kind (section 6), adopted 2026-11-17. Open
points 1 to 7 left for later versions of the part.

**Decided 2026-11-17, wider than music:** every content type is
optional. Version 1 is restructured before anyone implements it: the
shared mechanics stay in `catalog`, `playback` and `media`, and the
content types become two optional parts, **`video`** (films and series by
TMDB: keys, episode order, subtitles, PGS, skip markers, previews) and
**`music`** (this proposal). A server lists the ones it handles in
`hello`. Named by what identifies the titles, not "audio": audiobooks
and podcasts would be parts of their own. Written into spec/v1 on
2026-11-17: [video.md](../spec/v0/video.md) and
[music.md](../spec/v0/music.md); this proposal's open points are
[music.md's open questions](../spec/v0/music.md#9-open-questions).
Touches: [catalog](../spec/v0/catalog.md) sections 1, 3 and 4,
[playback](../spec/v0/playback.md) sections 2 and 6, and
[media](../spec/v0/media.md) section 3; adds a part, `music`
([core section 10](../spec/v0/core.md#10-versions-and-parts)).

This proposal gives options where the owner has a choice. Its
recommendation is at the end, marked as such.

## Problem

MSFP calls itself a *media server* federation protocol, but version 1
shares films and series only. A server can say that it has a music
library, and nothing more:

- **Libraries.** A library's `kind` may be `music` (catalog section 2),
  so a peer learns that the library exists. That is a label taken from
  Jellyfin's library types; nothing else in the specification uses it.
- **Titles.** A title is a film, a series, a season or an episode, and
  its key is a TMDB key (catalog section 4). TMDB has no music. A track
  can't be named, so it can't be listed: "A title TMDB doesn't know is
  not sent."
- **Playback.** `play` takes "the title key of a film or an episode"
  (playback section 2.1). Its capabilities already list audio containers
  and codecs (`mp3`, `flac`, `opus`, `alac`…), and its answer can carry no
  video, but nothing says how an audio-only playback works: the default
  when nothing is said is "H.264 and AAC in HLS".

Media servers that hold music are common: Jellyfin, Plex, Emby,
Navidrome, Funkwhale, Ampache. Without music, MSFP is a video federation
protocol, and the owner wants it to be more than that (2026-11-17).

What exists today, checked on 2026-11-17:

- **Jellyfin** keeps six MusicBrainz identifiers on items
  (`MediaBrowser.Model/Entities/MetadataProvider.cs`):
  `MusicBrainzArtist`, `MusicBrainzAlbumArtist`, `MusicBrainzAlbum` (a
  release), `MusicBrainzReleaseGroup`, `MusicBrainzTrack` and
  `MusicBrainzRecording`. Not checked: which of the last two Jellyfin
  fills from which file tag. Picard writes the recording's ID under the
  tag name "MusicBrainz Track Id", so the names may not mean what they
  say.
- **MusicBrainz** asks clients for about one request a second, with a
  User-Agent that names the application
  ([rate limiting](https://musicbrainz.org/doc/MusicBrainz_API/Rate_Limiting)).
  Its data is CC0 (core data) and can be mirrored.
- **Overfin** indexes no music (`internal/api/libindex.go`: "kinds it
  doesn't index (music, books…)"), and its site has no music player.
- **The plugin** passes the `music` kind through and sends no music
  title, as the specification says.
- Not measured: how many tracks in real libraries carry MusicBrainz
  identifiers. Libraries tagged with Picard or beets have them on nearly
  every track; libraries ripped or downloaded without tagging often have
  none.

## Proposal

A new part, **`music`**, version 1. It depends on `catalog`, `playback`
and `media`. A server lists it in `hello` only when it shares or plays
music. Between two servers that both speak it, the three parts it
depends on take the additions below; otherwise nothing changes for them,
and a receiver keeps skipping music keys, as it skips any key it doesn't
know today.

### 1. What is shared

Three kinds of title, named by **MusicBrainz identifiers** (MBIDs, the
lowercase UUIDs MusicBrainz gives):

| Kind | Key | MBID of |
|------|-----|---------|
| artist | `artist/mb/<mbid>` | the artist |
| album | `album/mb/<mbid>` | the **release group** |
| track | `track/mb/<mbid>` | the **recording** |

- An album is a release group, not a release: two servers holding
  different editions of the same album (a remaster, a country's
  pressing) have the same album. The edition a copy is goes in `ids`
  (section 3).
- A track is a recording: the same performance on an album and on a
  compilation is one track, so a viewer's server knows that it already
  has it. A live version or a remix is another recording, so another
  track.
- `<mbid>` is 36 characters, `[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}`,
  lowercase. Any other form is a different string, and the receiver
  skips it.
- A track without a recording MBID is not sent, as a film without a
  TMDB ID isn't. An album or an artist without one is not sent either;
  its tracks may be.
- `play` takes only tracks.

### 2. What a title carries

Video titles carry keys only: the viewer's server looks names and
pictures up in TMDB. Music can't work that way. At one request a second,
a library of 20,000 tracks would take more than five hours of
MusicBrainz lookups on every peer that reads it, and MusicBrainz would
be asked the same thing by every server of a federation. So **music
titles carry their own description**, from the owning server:

| Field | Kinds | Content |
|-------|-------|---------|
| `name` | all | the title's name, text, at most 256 characters |
| `artists` | album, track | the artist keys credited, in order, at most 16 |
| `artistNames` | album, track | the credited names as written (`"Simon & Garfunkel"`), at most 512 characters |
| `album` | track | the album key it is on, when the owning server knows it |
| `disc`, `number` | track | its place on the album: 1 to 999 each |
| `year` | album, track | the year of the edition held, 1000 to 9999 |
| `durationMs` | track | its length, 0 to 86,400,000 |
| `ids` | all | other identifiers (section 3) |

These are untrusted text, cleaned as core section 9.1 cleans a server's
name (with the lengths above instead of 80), and never more than a label: two titles are the same when
their keys are equal, whatever their names. A viewer's server may
replace them with its own data for keys it knows.

### 3. Other identifiers

| Scheme | Kinds | Form |
|--------|-------|------|
| `mbrelease` | album, track | the release MBID of the edition the copy comes from |
| `mbtrack` | track | the MBID of the track on that release |
| `isrc` | track | an ISRC, 12 characters `[A-Z]{2}[A-Z0-9]{3}[0-9]{7}` |

As in catalog section 4.4, `ids` never replaces the key.

### 4. Pictures

Album art comes from the **Cover Art Archive**, fetched by the viewer's
server itself, as it fetches TMDB's pictures for films: by the album key's
release group (`/release-group/<mbid>/front`), or by the release in
`mbrelease` when the edition's own cover is wanted. Sizes 250, 500 and
1200 pixels exist. The Archive states no rate limit (checked 2026-11-17)
but may answer `503`; a viewer's server caches each cover. The owning server sends no picture. An album
whose release has no art there shows none.

### 5. Playing a track

- `play` takes a track key. Everything else stays: copies, versions,
  limits, ticket, `progress`, `stop`.
- **Audio only.** The answer has no subtitle tracks, no forced tracks,
  no skip markers and no previews; a viewer's server drops any it gets
  for a track. `audioTracks` lists the copy's one audio track (or more,
  rarely).
- **Nothing said** about `containers` or `audioCodecs`, for a track: the
  owning server serves AAC in HLS, audio only, which every viewer's
  server must play. `direct` serves the file as it is, with ranges, when
  the capabilities allow it: FLAC to a player that plays FLAC is never
  transcoded.
- **One playback per track.** A viewer's server that plays an album
  calls `play` for each track as it comes, and `stop` for the one
  before. The `play` limit of core section 8.3 (suggested: 30 a minute)
  is enough for that; skipping through tracks faster is refused with
  `429`, and the viewer's server waits.
- **Streams.** An audio playback counts as a stream, like a video one.
  The owning server may count audio separately; it says so with two new
  members of `limits`, `audioStreams` and `audioTranscodes`, same ranges
  as `streams` and `transcodes`. Absent: audio counts in `streams`.

### 6. The library kind

A `music` library holds artists, albums and tracks. A `mixed` library
may hold them beside films and series. A receiver that speaks `music`
reads them from `titles`, in the same paging and the same `changes`.

## Alternatives considered

### Identity

- **Release instead of release group, track instead of recording.**
  Exact to the edition, but two servers with different pressings of an
  album would share nothing, and a song on an album and on a compilation
  would be two titles. Kept as `ids`.
- **A key built from names** (`"artist — album — title"`, normalized).
  Works for untagged libraries, but two servers rarely write names the
  same way, and "the same key, byte for byte" (catalog section 4.2)
  becomes a guess. Normalizing rules would grow without end (articles,
  "feat.", accents, scripts).
- **AcoustID** (a fingerprint of the sound). Finds the recording from
  the audio itself, but an implementation must compute fingerprints of
  every file, and the AcoustID service needs a key. Better left to the
  owning server's own tagging, before MSFP.
- **ISRC as the key.** Assigned to recordings by labels, but missing on
  much music (old, independent), and several ISRCs may name one
  recording. Kept as `ids`.

### Metadata

- **Keys only, as for video.** Consistent, but slow and heavy on
  MusicBrainz (section 2), and a federation would multiply the load.
- **A MusicBrainz mirror required on each server.** Fast, but a mirror
  is a database of tens of gigabytes: far from "simple setup".

### Where it lives

- **In `catalog` 1, `playback` 1 and `media` 1 themselves**, not a part
  of its own. Version 1 is still a draft that nobody implements, so this
  breaks nothing. One fewer name, but music would no longer be optional:
  a server that handles no music (a video-only server, or a first
  version of an implementation) would have to play tracks to claim
  `playback` 1, or claim it and refuse them, and an owning server
  couldn't tell which peers to send music to. Music would also change
  only with new versions of the video parts.

### Wider: one change for every kind

Books, audiobooks, photos and home videos are other kinds a media server
holds. They have no shared identifier the way films (TMDB) and music
(MusicBrainz) do: audiobooks have ISBNs and ASINs, photos and home videos
nothing. This proposal is music only; each other kind would be a part of
its own, when an identifier exists for it.

## Cost

- **Specification:** a new file, `spec/v0/music.md`, with the part's
  rules, and a line in each of the three parts it extends; vectors for
  music keys and title objects.
- **Overfin:** large. It indexes no music today: an index of artists,
  albums and tracks from Jellyfin's items and their MusicBrainz IDs, the
  `titles` additions, `play` of a track, and a music player on the site
  to use it. The federation side is the small part; the site's music
  pages and player are most of the work.
- **The plugin:** medium. Jellyfin already holds the MBIDs; the plugin
  maps them to keys (after checking which of `MusicBrainzTrack` and
  `MusicBrainzRecording` holds the recording), sends the descriptions,
  and serves audio playbacks through Jellyfin's audio streaming.

## Compatibility

Nothing changes between servers that don't both list `music`. A server
that doesn't speak it already skips `artist/`, `album/` and `track/`
keys (catalog section 3.3: a key that doesn't parse as a title key, or of
a scheme the caller doesn't know, is skipped), so an owning server may even
send them to every peer; the proposal still says to send them only to
peers that list the part, to keep readings small.

## Open points

1. **Untagged libraries.** A library without MBIDs shares nothing. Is
   that acceptable for version 1, with a later scheme for untagged
   music, or should a fallback be in from the start?
2. **Descriptions for video too.** If music titles carry names, should
   films and series be allowed to (as an option), for servers that don't
   want to depend on TMDB?
3. **Album art from the owning server** as a fallback, through `media`,
   when the Cover Art Archive has none. It would need the same checks as
   preview sheets (re-encoded, size bounds).
4. **Lyrics**, synced or not: a later version of the part.
5. **Gapless playback** across tracks: one playback per track leaves a
   gap unless the viewer's server preloads the next one. Should `play`
   allow a second playback to start while the first ends, counted as one
   stream?
6. **Music videos** (Jellyfin's `musicvideos` kind): a track's video, or
   a film? Left out.
7. **Watched state** for music is play counts and ratings, not
   "watched": nothing in this proposal shares it, as nothing shares it
   for video either.

## Recommendation

> **Adopt the part `music` as proposed: release groups, recordings and
> artists by MBID; descriptions carried by the titles; art from the Cover
> Art Archive; one audio playback per track.** Leave open points 1 to 7
> for later versions of the part.
>
> Reasons: MusicBrainz is to music what TMDB is to films, open (CC0) and
> already in Jellyfin, Navidrome and most taggers; release groups and
> recordings are the levels at which two servers' "same album" and "same
> song" agree; carrying descriptions is the only way a federation reads a
> large music library without overloading MusicBrainz; a part of its own
> keeps video-only servers unaffected.
