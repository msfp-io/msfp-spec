# MSFP on Plex and Emby

**Informative, not part of the protocol.** This note asks whether MSFP
version 0 could be built on Plex Media Server and on Emby Server, not
only on Jellyfin, and where Jellyfin's ways may have slipped into the
specification. It changes nothing a server sends or accepts by itself.

**Adopted.** The owner accepted every point of
[section 5](#5-what-the-spec-should-change-or-state) on 2026-11-18, the
two changes and the nine clarifications, with the wording proposed there;
they are in [spec/v0](../spec/v0/README.md)
([core open questions 43 and 44](../spec/v0/core.md#18-open-questions)).
The rest of this note is as written before.

Written 2026-11-18, against the drafts of [spec/v0](../spec/v0/README.md)
and [host.md](host.md) of that day. Sources are linked in each section.
Every claim marked **(unconfirmed)** comes from no source I could read,
or only from forum posts; it needs a test on a real server. Claims marked
**(community)** come from community documents or forum posts, not the
vendor's.

The words: **maps** (the platform gives it as MSFP needs it), **work**
(it can be done, with the work said), **doesn't fit** (the platform can't
give it, and the specification would need a change).

## Contents

1. [Overview](#1-overview)
2. [Plex Media Server](#2-plex-media-server)
3. [Emby Server](#3-emby-server)
4. [Jellyfin, for comparison](#4-jellyfin-for-comparison)
5. [What the spec should change or state](#5-what-the-spec-should-change-or-state)
6. [Claims not confirmed](#6-claims-not-confirmed)

## 1. Overview

| Area | Jellyfin | Plex | Emby |
|------|----------|------|------|
| TMDB IDs of films and series | maps | maps (new agents); work (legacy agents) | maps |
| TMDB IDs of episodes | maps; work for a file's second episode | maps (community; gaps reported) | work (unconfirmed) |
| Episode numbers in TMDB's order | work (the files' numbers) | maps under the default order; work under TVDB orders | work (TVDB orders by default) |
| Several episodes in one file | work | work | work |
| Specials (season 0) | maps | maps | maps |
| `imdb` and `tvdb` in `ids` | maps | maps | maps |
| Libraries and their kinds | maps (the spec's list is Jellyfin's) | work (four types to map) | maps |
| Versions (several copies) | maps | maps | maps |
| A copy of several files (parts) | **doesn't fit** `file` | **doesn't fit** `file` | **doesn't fit** `file` |
| Track languages and flags | work (language codes) | maps | work (language codes) |
| Track roles (commentary, description) | work (guessed) | work (description flagged, commentary guessed) | work (guessed) |
| Playing a chosen audio track | maps | work (a per-user setting) | maps |
| Direct, remux or transcode | maps | work | maps |
| Picture height and HDR kinds | maps | work | maps |
| HLS from the transcoder, in MSFP's subset | work (rewriting) | work (rewriting) | work (rewriting) |
| The file with byte ranges | maps | maps (ranges unconfirmed) | maps (ranges unconfirmed) |
| Stopping a transcode, session life | maps | work | maps |
| Text subtitles as WebVTT, by stretch | maps (slow on MKV) | work | maps |
| PGS as raw data | work (cut by stretch) | **doesn't fit** through the API; work with file access | work (unconfirmed) |
| Skip markers | maps (the spec's kinds are Jellyfin's) | work | work |
| Seek-bar preview sheets | maps (the spec's layout is Jellyfin's) | work (BIF to sheets) | work (BIF to sheets) |
| MusicBrainz IDs | maps | work (unclear which IDs) | work (recording ID unconfirmed) |
| Album art | maps (the viewer's server asks the Cover Art Archive) | maps | maps |
| Inside the server | maps (a plugin) | **doesn't fit**: a separate service | work (a plugin) |
| Credentials of a separate service | an API key (admin) | a plex.tv account token | an API key (admin) |

The short answer: **both platforms can carry MSFP 0** as an owning
server, Emby nearly as well as Jellyfin, Plex only through a separate
service (a sidecar) that relays the media. Two rules of the
specification don't hold on any of the three and should change
(section 5.1): copies made of several files, and the rule that every PGS
track is listed. The rest is work for implementations, which the
specification should state in a few places (section 5.2).

## 2. Plex Media Server

**Sources.** Plex published official API documentation in September
2025: [developer.plex.tv/pms](https://developer.plex.tv/pms/), an OpenAPI
document, version 0.2.3 (for PMS 1.43.4 and later) when read on
2026-11-18 ([announcement](https://forums.plex.tv/t/public-api-docs-for-pms-are-here/930957)).
It lists the endpoints and part of the fields; many stream fields are
not in it. For those I used
[python-plexapi](https://github.com/pkkid/python-plexapi) (`plexapi/media.py`,
`video.py`, maintained, last change 2026-11-09) and the community OpenAPI
document [LukeHagar/plex-api-spec](https://github.com/LukeHagar/plex-api-spec)
(changed 2026-11-18); both are **(community)**.

### 2.1 Identity

- **Films and series.** With Plex's current agents, an item's own `guid`
  is `plex://…`, and its other identifiers come as `Guid` entries,
  `tmdb://<id>`, `imdb://tt…`, `tvdb://<id>` (official document, "Guid"
  table: "Internally supported providers include imdb, tmdb, tvdb").
  `includeGuids=1` adds them to list answers, so a library can be read
  without one call per item
  ([forum, implemented 2021](https://forums.plex.tv/t/implemented-add-includeguids-url-parameter-to-return-external-guids/735620),
  **(community)**). Libraries still on the legacy agents
  (`com.plexapp.agents.thetvdb`, `…themoviedb`, `…imdb`) carry only the
  agent's own ID: work, a TMDB lookup (`/find` by IMDb or TVDB ID).
- **Episodes.** Episodes carry `Guid` entries too, with their own
  `tmdb://` ID (the TMDB episode ID, so the `e<episode id>` key form
  maps), but some episodes were reported without one or with two `tvdb://`
  entries ([same forum thread](https://forums.plex.tv/t/implemented-add-includeguids-url-parameter-to-return-external-guids/735620),
  **(community)**). An episode without one is looked up at TMDB, or left
  out, as [video section 4.1](https://github.com/msfp-io/msfp-media/blob/main/spec/v0/video.md#41-episode-order) says.
- **Numbering.** Plex's TV agent offers four episode orders per library or
  per show: TMDB (aired), TheTVDB (aired), TheTVDB (DVD), TheTVDB
  (absolute) (`showOrdering` in python-plexapi's `video.py`:
  `tmdbAiring`, `aired`, `dvd`, `absolute`). Forum posts say TMDB (aired)
  is the default ([forum](https://forums.plex.tv/t/episode-ordering-naming-incorrect/876719),
  **(community)**). Under it, `parentIndex` and `index` are TMDB's numbers,
  as MSFP's keys need. Under a TVDB order they aren't; the TMDB episode
  ID still names the episode, so keys stay right when every episode
  carries it.
- **Several episodes in one file** (`s02e18-e19`): Plex lists each episode
  as its own item, all on the same media part, and "playing any of the
  represented episodes will play the full file"
  ([naming guide](https://support.plex.tv/articles/naming-and-organizing-your-tv-show-files/)).
  Work: the implementation groups episodes by part to fill `with`. Plex
  doesn't say where the second episode starts: the copy's start
  ([video section 5.2](https://github.com/msfp-io/msfp-media/blob/main/spec/v0/video.md#52-several-episodes-in-one-copy)
  allows it).
- **Specials** are season 0 ("Season 00" or "Specials" folder, same
  guide): maps.

### 2.2 Libraries and copies

- **Sections** (`GET /library/sections/all`) have a `type`: `movie`,
  `show`, `artist` (music), `photo`. Work: map them to `movies`,
  `tvshows`, `music`, `photos`. Plex has no mixed section; "Other videos"
  sections are `movie` sections whose items mostly have no TMDB ID, so
  send nothing.
- **Versions.** An item has several `Media` (versions), each with one or
  more `Part` (files): maps, except a `Media` of several parts (stacked
  `cd1`, `cd2` files), which is one copy of several files. The
  transcoder joins parts (`partIndex` -1: "the server should join parts
  together in a transcode", official document), so such a copy plays
  over HLS, never as `file`. See section 5.1, point 1.
- **IDs.** `ratingKey` is "an opaque string" (official document); part
  and stream IDs are numbers. The implementation keeps its own public IDs
  anyway ([listing section 2.1](https://github.com/msfp-io/msfp-media/blob/main/spec/v0/listing.md#21-library-ids)).
- **Changes.** Items carry `addedAt` and `updatedAt`, but a removed item
  just disappears: the engine's own change log (`OwnLogTx`,
  [host.md section 1.1](host.md#11-tx)) fits.

### 2.3 Tracks and capabilities

- **Streams** (`streamType` 1 video, 2 audio, 3 subtitles) carry `codec`,
  `language`, `languageCode` (three letters), `languageTag` (two letters,
  `fr`), `default`, `selected`, `title`, `displayTitle`, `channels`,
  `bitDepth`, `width`, `height` (official document, `stream` schema);
  and, per python-plexapi, `forced`, `hearingImpaired` on subtitles and
  `visualImpaired` ("visually impaired (AD) audio") on audio
  **(community)**. So `description` maps from `visualImpaired`; there is
  no commentary flag: guessed from the title.
- **The audio track.** The transcoder's start call has no audio stream
  parameter; the track is the user's selection on the part,
  `PUT /library/parts/{partId}?audioStreamID=…&subtitleStreamID=…`
  ("Set which streams are selected by this user", official document).
  Work: it is a stored per-user setting, so the implementation must use
  its own Plex account (not the operator's), set the track before each
  start, and accept that two peers' playbacks of one part at once share
  that setting **(unconfirmed: whether a selection made between two
  starts changes a running transcode)**.
- **The decision.** `GET /{video|music}/:/transcode/universal/decision`
  answers per stream whether it plays directly, is copied or is
  transcoded, from `directPlay`, `directStream`, `directStreamAudio`,
  `videoResolution`, `maxVideoBitrate`… and a client profile
  (`X-Plex-Client-Profile-Name`, `X-Plex-Client-Profile-Extra`, official
  document). Work: build a profile from MSFP's coarsened capabilities.
  `direct` is the part's own URL; `directStream` with HLS is `remux`.
- **Height** is the stream's `height` and `width`: maps (the 16:9 rule
  of [the order of copies in the notes](https://github.com/msfp-io/msfp-media/blob/main/spec/v0/notes.md#the-order-of-copies) is computed).
- **HDR.** Plex gives `colorTrc` (`smpte2084` for PQ, `arib-std-b67` for
  HLG) and, for Dolby Vision, `DOVIPresent`, `DOVIProfile`,
  `DOVIBLCompatID` (python-plexapi **(community)**). Work: PQ is `hdr10`,
  HLG `hlg`, `DOVIPresent` `dv`, its base layer from `DOVIBLCompatID`
  (1 HDR10, 4 HLG, 2 standard range, per Dolby's numbering). HDR10+ isn't
  told apart, which MSFP doesn't need.

### 2.4 Media

- **HLS.** `GET /video/:/transcode/universal/start.m3u8` with
  `protocol=hls` (official document; `http` and `dash` too) returns a
  playlist naming `session/<id>/base/index.m3u8`, whose segments are
  `00000.ts` and so on, or `.m4s` with a header file in recent versions
  ([forum logs](https://forums.plex.tv/t/client-development-universal-transcode-api-help/735388),
  **(community)**). Work: rewrite every playlist to bare names
  ([fetching section 3.1](https://github.com/msfp-io/msfp-media/blob/main/spec/v0/fetching.md#31-hls-playlists)), keep a map
  from each name to Plex's path, and add the Plex token to every fetch.
  TS and fMP4 segments are both in MSFP's subset.
- **The file.** `GET /library/parts/{partId}/{changestamp}/{filename}`
  ("Get a media part for streaming or download", official document):
  maps, **(unconfirmed: byte ranges)**, which every player relies on.
- **Session life.** A client reports `POST /:/timeline` "every 10 seconds
  on a LAN" with `state` (`playing`, `paused`, `stopped`) (official
  document). Work: the implementation reports for each playback while
  the peer does, and `stopped` at the end. Stopping the transcode itself:
  `GET /video/:/transcode/universal/stop?session=…` **(community,
  unconfirmed)**; `POST /status/sessions/terminate` ends a session "kicking
  off the user" (official document; **(unconfirmed)** whether it needs
  Plex Pass).
- **Parallel playbacks.** Plex tells sessions apart by
  `X-Plex-Client-Identifier` and `X-Plex-Session-Identifier`; the
  implementation sends its own per playback (official document headers).
  All peers' playbacks show in Plex as that one account.

### 2.5 Subtitles

- **External** subtitle files are streams with a `key`, fetched at
  `GET /library/streams/{streamId}.{ext}`, with `format` to convert text
  ones (official document). **Embedded** ones have no `key` ("If the stream
  is independently streamable, the key", official document). Plex has
  `GET /{type}/:/transcode/universal/subtitles` ("Only transcode subtitle
  streams") and `subtitles=sidecar|segmented` on the start call
  **(unconfirmed: whether either gives a whole WebVTT track, or WebVTT by
  time range)**. No endpoint takes a time range. Work: fetch the track
  whole (or extract it from the file) and cut the 120-second parts of
  [video section 6.2](https://github.com/msfp-io/msfp-media/blob/main/spec/v0/video.md#62-what-is-fetched) itself.
- **PGS.** Plex's transcoder burns picture subtitles in (`subtitles=burn`);
  no endpoint gives an embedded PGS track's raw data. **Doesn't fit**
  through the API. With read access to the files, the implementation can
  extract `.sup` itself (ffmpeg), as it would for text. Without it, it
  can't serve them, and the specification today still makes it list them
  (section 5.1, point 2).

### 2.6 Skip markers and previews

- **Markers** of kinds `intro`, `credits`, `commercial` (also `bookmark`,
  `resume`), with `startTimeOffset` and `endTimeOffset` in ms (official
  document, marker schema and examples). Work: `intro` to `intro`,
  `credits` to `outro`, `commercial` to `commercial`; MSFP's `recap` and
  `preview` never come. Detecting them needs Plex Pass on the server
  owner's account
  ([credits detection](https://support.plex.tv/articles/credits-detection/)).
  **(Unconfirmed:** the query parameter that adds markers to a metadata
  answer, `includeMarkers=1` in community code.)
- **Previews.** Plex makes BIF files ("Start BIF generation", official
  document), read as `GET /library/parts/{partId}/indexes/sd?interval=…`
  (the index) and `GET /library/parts/{partId}/indexes/sd/{offset}` (one
  image at an offset in ms). A BIF holds one JPEG per interval, not tiled
  sheets. Work: decode the images, tile them into sheets within
  [video section 5.5](../spec/v0/video.md#55-seek-bar-previews)'s bounds
  (at most 1,000 sheets, 4,096 pixels each way), and re-encode. One image
  per sheet would break the 1,000 cap for a long film at a short
  interval.

### 2.7 Music

- Tracks, albums and artists matched by Plex's music agent carry
  `mbid://` identifiers, and `includeGuids=1` adds them to lists
  ([forum, 2025](https://forums.plex.tv/t/include-mbids-in-more-api-responses/925119),
  **(community)**). **(Unconfirmed:** which MusicBrainz entity each is: a
  track's could be the recording or the release track, an album's the
  release or the release group; posts disagree.) Work at best, a check
  on a real library first.
- Album art is fetched by the viewer's server from the Cover Art Archive
  ([music section 4](../spec/v0/music.md#4-pictures)): only the release
  group MBID matters.
- Audio-only HLS: `/music/:/transcode/universal/start.m3u8` maps.

### 2.8 Architecture

- **No plugins.** Plex announced the end of plugins on 2018-09-25
  ([TechCrunch, 2018-09-26](https://techcrunch.com/2018/09/26/plex-kills-off-support-for-cloud-sync-plugins-and-bookmarking-features/));
  Plex's support pages say plugins can no longer be installed from Plex
  Web and that support "will be slowly removed"
  ([support](https://support.plex.tv/articles/201053748-overview/)).
  They were Python 2 bundles for "channels", never a way to add an
  arbitrary HTTPS API. Plex's newer extension point, "custom metadata
  providers" (PMS 1.43, official document), is an HTTP service that Plex
  *calls* for film and series metadata; it can't answer peers. So an MSFP
  implementation for Plex is **a separate service**: it holds the MSFP
  keys and address, answers peers, and relays every byte of media from
  Plex to the viewer's server.
- **Credentials.** The service needs a Plex token (`X-Plex-Token`, official
  document). A token is a plex.tv account's, valid on every server that
  account reaches, not a server-scoped key. Best: a dedicated Plex account
  (a managed user of the owner's Plex Home, or a friend account) given
  only the offered libraries, so a leaked token reaches little and the
  per-user stream selections (section 2.3) touch nobody's own.
  **(Unconfirmed:** that a managed user's token can use the transcoder
  and part URLs the same way.)
- **What it means for MSFP.** The service has its own address and port
  (Plex keeps 32400 and its `plex.direct` certificates), or sits behind
  the operator's reverse proxy with a discovery `base`
  ([core section 11](../spec/v0/core.md#11-the-discovery-document)). It
  should run beside Plex: every byte crosses it. It knows Plex's own
  sessions from `GET /status/sessions`, which it needs to keep the
  transcodes reserved for the server's own people
  ([playback section 6](../spec/v0/playback.md#6-what-the-owning-server-enforces)).
  Hardware transcoding needs Plex Pass; remote playback of personal video
  needs Plex Pass or a Remote Watch Pass since 2025-04-29
  ([Plex blog](https://plex.tv/blog/important-2025-plex-updates/)). A
  service on Plex's own network is a local client **(unconfirmed: that
  Plex never counts it as remote)**. Whether relaying Plex streams to
  another server fits Plex's terms of service was not checked.

## 3. Emby Server

**Sources.** Emby's developer site,
[dev.emby.media](https://dev.emby.media/): the REST reference (generated
from the server's own OpenAPI document; pages say "Copyright 2022", so
some may be older than Emby 4.8/4.9) and the plugin API reference;
Emby's [Learn](https://emby.media/learn/) and support articles; the Emby
forum for the plugin route change. Emby has been closed source since
version 3.6 (2018) **(unconfirmed: the exact version)**; its plugin SDK is
public.

### 3.1 Identity

- **Films and series** carry `ProviderIds` with `Tmdb`, `Imdb`, `Tvdb`
  ([MetadataProviders](https://dev.emby.media/reference/pluginapi/MediaBrowser.Model.Entities.MetadataProviders.html));
  `GET /Items` filters by `HasTmdbId` and `AnyProviderIdEquals`
  ([getItems](https://dev.emby.media/reference/RestAPI/ItemsService/getItems.html)):
  maps.
- **Episodes** carry `ProviderIds` too. **(Unconfirmed:** that Emby stores
  an episode's own TMDB ID; with TheTVDB first among a library's
  providers, it may store only `Tvdb`.) Work: a TMDB lookup by the
  episode's TVDB ID, or by the series' TMDB ID and numbers when they are
  TMDB's.
- **Numbering.** Emby's display orders are aired (the default), DVD and
  absolute, all from TheTVDB; with TMDB, only aired; TMDB's episode groups
  aren't supported ([Learn: episode order](https://emby.media/learn/articles/libraries-collections/television/episode-order/Library-Episode-Order.html),
  [forum](https://emby.media/community/topic/120771-missing-episodes-episode-ordering),
  **(community)**). As in Jellyfin, `ParentIndexNumber` and `IndexNumber`
  come from the files. Work: they are TVDB's aired numbers more often than
  TMDB's; the implementation must map them, preferably through the TMDB
  episode ID.
- **Several episodes in one file**: `IndexNumberEnd`, as in Jellyfin
  **(unconfirmed for Emby)**. Work, as section 4.
- **Specials**: season 0: maps.

### 3.2 Libraries and copies

- Library kinds are the same words as Jellyfin's (`movies`, `tvshows`,
  `music`, `musicvideos`, `homevideos`, `boxsets`, `photos`, `mixed`…),
  Jellyfin being a fork of Emby: maps.
- **Versions** are `MediaSources` of one item (an item's files grouped by
  folder or by name), each with its `MediaStreams`
  ([Learn: editions](https://emby.media/learn/articles/libraries-collections/movies/editions-versions/Library-Movie-Editions.html)):
  maps. Stacked parts: as Plex and Jellyfin (section 5.1, point 1)
  **(unconfirmed: how Emby serves a stacked item)**.
- **Changes**: `MinDateLastSaved` on `GET /Items`; removals not listed:
  the engine's own log fits.

### 3.3 Tracks and capabilities

- **Streams** ([MediaStream](https://dev.emby.media/reference/pluginapi/MediaBrowser.Model.Entities.MediaStream.html))
  carry `Language` (from the file's tag, so often three-letter codes like
  `fre`), `IsDefault`, `IsForced`, `IsHearingImpaired`, `Title`, `Comment`,
  `Codec`, `Channels`, `Width`, `Height`, `IsExternal`. No commentary or
  description flag: work, guessed from `Title`/`Comment`. Language codes:
  work, to MSFP's shortest tags ([catalog section 3](../spec/v0/catalog.md#3-titles)).
- **The audio track**: `AudioStreamIndex` on the stream URLs
  ([video streaming](https://betadev.emby.media/doc/restapi/Video-Streaming.html)):
  maps.
- **The decision**: `POST /Items/{Id}/PlaybackInfo` with a device profile
  answers per media source whether it can play directly, be streamed
  directly (remux) or be transcoded, with a transcoding URL (Emby's
  MediaSource API; **(unconfirmed)** in today's reference, but this is
  the Jellyfin model, which kept Emby's). Maps, with a profile built from
  MSFP's capabilities.
- **HDR**: `ExtendedVideoType` is `Hdr10`, `Hdr10Plus`, `HyperLogGamma` or
  `DolbyVision`; `ExtendedVideoSubType` names the Dolby Vision profile and
  its base layer (`DoviProfile81` "HDR10 compatible", `DoviProfile84` "HLG
  compatible", `DoviProfile82` "SDR"…)
  ([types](https://dev.emby.media/reference/pluginapi/MediaBrowser.Model.Entities.ExtendedVideoTypes.html),
  [sub-types](https://dev.emby.media/reference/pluginapi/MediaBrowser.Model.Entities.ExtendedVideoSubTypes.html)):
  maps directly onto `video.hdr`. Height: maps.

### 3.4 Media

- **HLS**: `GET /Videos/{Id}/master.m3u8`, `main.m3u8`, segments under
  `/Videos/{Id}/hls1/{PlaylistId}/{SegmentId}.{SegmentContainer}`
  ([DynamicHlsService](https://dev.emby.media/reference/RestAPI/DynamicHlsService.html)).
  Work: rewrite to bare names (the playlists carry paths and query
  strings **(unconfirmed for Emby; true of Jellyfin)**).
- **The file**: `GET /Videos/{Id}/stream?static=true`
  ([VideoService](https://dev.emby.media/reference/RestAPI/VideoService.html),
  "To direct stream a video file, simply use the static=true parameter"):
  maps, **(unconfirmed: byte ranges in the docs)**.
- **Stopping**: `DELETE /Videos/ActiveEncodings`
  ([HlsSegmentService](https://dev.emby.media/reference/RestAPI/HlsSegmentService.html)),
  with the play session's ID: maps. Session reports
  (`/Sessions/Playing`, `/Progress`, `/Stopped`) as Jellyfin's.

### 3.5 Subtitles

- **Text**: `GET /Videos/{Id}/{MediaSourceId}/Subtitles/{Index}/{StartPositionTicks}/Stream.{Format}`
  with `EndPositionTicks` and `CopyTimestamps`
  ([the call](https://dev.emby.media/reference/RestAPI/SubtitleService/getVideosByIdByMediasourceidSubtitlesByIndexByStartpositionticksStreamByFormat.html)):
  WebVTT by time range, which is what MSFP's 120-second parts need: maps
  (the part is still rebuilt as [video section 6.3](https://github.com/msfp-io/msfp-media/blob/main/spec/v0/video.md#63-webvtt) says).
- **PGS**: **(unconfirmed)** whether the same call returns an embedded PGS
  track's raw data (`Stream.sup`), as Jellyfin's does. If not, work with
  the file: a plugin knows each stream's file (`Path`) and can run ffmpeg.

### 3.6 Skip markers and previews

- **Markers** are chapters with a `MarkerType`: `IntroStart`, `IntroEnd`,
  `CreditsStart`, besides plain `Chapter`
  ([MarkerType](https://dev.emby.media/reference/pluginapi/MediaBrowser.Model.Entities.MarkerType.html),
  [ChapterInfo](https://dev.emby.media/reference/pluginapi/MediaBrowser.Model.Entities.ChapterInfo.html)).
  Work: an intro from its start and end; an `outro` from `CreditsStart`
  to the copy's end, since credits have no end marker. Intro detection
  needs Emby Premiere and at least two episodes in a season
  ([Intro Skip](https://emby.media/support/articles/Intro-Skip.html)).
- **Previews**: "Emby uses BIF resources for video previews"
  ([Learn: seek previews](https://emby.media/learn/articles/libraries-collections/scanning/analysis-previews/Library-Seek-Previews.html)),
  served at `GET /Videos/{Id}/index.bif` and described at
  `GET /Items/{Id}/ThumbnailSet`
  ([BifService](https://dev.emby.media/reference/RestAPI/BifService.html)).
  Work: BIF to sheets, as Plex (section 2.6).

### 3.7 Music

- `ProviderIds` has `MusicBrainzArtist`, `MusicBrainzAlbumArtist`,
  `MusicBrainzAlbum` (the release), `MusicBrainzReleaseGroup` and
  `MusicBrainzTrack`, no `MusicBrainzRecording`
  ([MetadataProviders](https://dev.emby.media/reference/pluginapi/MediaBrowser.Model.Entities.MetadataProviders.html)).
  **(Unconfirmed:** what `MusicBrainzTrack` holds. Picard writes the
  *recording* MBID in the tag it calls "MusicBrainz Track Id", and the
  release track's in "MusicBrainz Release Track Id"; if Emby reads the
  first, `MusicBrainzTrack` is the recording MSFP needs.) Album and
  artist: maps.
- Audio-only HLS: `/Audio/{Id}/master.m3u8` maps.

### 3.8 Architecture

- **Plugins** are .NET assemblies loaded into the server. A plugin adds
  HTTP routes with a request class carrying `[Route("/…", "GET")]` and a
  service implementing `IService`; it can return objects, strings, bytes
  or streams, and files through `IHttpResultFactory`
  ([Creating API endpoints](https://dev.emby.media/doc/plugins/dev/Creating-Api-Endpoints.html)).
  Since 4.8.0.37 (2023-05-29), plugin routes require Emby authentication
  unless marked `[Unauthenticated]`, which MSFP's routes need (peers sign
  with their own keys); a 4.7 build and a 4.8 build can't be the same
  ([Emby team, forum](https://emby.media/community/topic/119028-48037-plugin-routes-now-defaulting-to-authenticated)).
  In-process, a plugin reaches the library, the files' paths, the server's
  ffmpeg and outbound HTTP like any .NET code.
- **(Unconfirmed**, to test before choosing a plugin:**)** that a route can
  be `/api/v1/federation/…` and `/.well-known/msfp` (Emby answers its own
  API at `/emby/…` and, it seems, at the root); that a service can read
  the raw body bytes, needed for signatures
  ([core section 3](../spec/v0/core.md#3-transport)); that Emby doesn't
  compress the answers of plugin routes by itself; that its own `404`
  for an unknown path can be reproduced. A plugin inside Emby's HTTPS
  serves under Emby's certificate (a `.pfx` the operator gives).
- **Or a separate service**, as for Plex, with an Emby API key (made in
  the dashboard; admin rights over the whole server) or a dedicated user's
  token limited to the offered libraries.

## 4. Jellyfin, for comparison

What the two Jellyfin implementations met (Overfin as a separate service,
[overfin.md](overfin.md); the MSFP Jellyfin plugin, `jellyfin-plugin-msfp`, in
progress then and built since, its notes `SPEC-GAPS.md`), and Jellyfin's API ([api.jellyfin.org](https://api.jellyfin.org/)):

- Several parts of the specification are Jellyfin's shapes: the library
  kinds ([catalog section 2](../spec/v0/catalog.md#2-libraries)) are
  Jellyfin's `CollectionType` values; the skip marker kinds `intro`,
  `outro`, `recap`, `preview`, `commercial`
  ([video section 5.3](https://github.com/msfp-io/msfp-media/blob/main/spec/v0/video.md#53-the-answer)) are Jellyfin's
  media segment types; the preview layout (tile size, columns, rows,
  interval, count) is Jellyfin's trickplay manifest. None is wrong
  elsewhere; each needs a mapping on Plex and Emby, said above.
- Episode numbers are the files' numbers, whatever display order is
  chosen; they are TMDB's when the files follow TMDB's aired order.
  Episodes keep one TMDB ID each, so a file's second episode needs a TMDB
  lookup (decided for the plugin then; built since: the plugin maps each
file's episode numbers to a TMDB episode ID and place, and keys each
episode of a file by it).
- Text subtitles come by time range, but Jellyfin reads a whole MKV to
  extract one (16.8 s against 0.45 s with a seek, measured for Overfin's
  companion plugin). PGS comes out as `.sup`, whole; cutting it into
  MSFP's stretches is the implementation's.
- Jellyfin has no commentary flag either **(unconfirmed)**.

## 5. What the spec should change or state

All adopted by the owner on 2026-11-18, as proposed (point 8 with a
region kept and 639-2/T codes too, and test cases, `languageCodes` in MSFP Media's
[vectors/v0/base.json](https://github.com/msfp-io/msfp-media/blob/main/vectors/v0/base.json); point 3's TMDB
lookups in [video section 7](../spec/v0/video.md#7-security-considerations);
point 11 in [core section 1](../spec/v0/core.md#1-overview-and-terms)).

### 5.1 Must change to be platform-neutral

1. **A copy may be several files.** [playback section 1](../spec/v0/playback.md#1-terms)
   defines a copy as "one file of a title". Plex (`Media` with several
   `Part`s), Emby and Jellyfin (stacked `cd1`/`cd2` files) all have
   copies made of several files, played as one. Proposed: "**Copy**: one
   version of a title on the owning server: one file, or several files
   played one after the other as one (parts)." And in
   [media section 1](../spec/v0/media.md#1-the-urls), after `file`: "The
   `file` protocol is only for a copy of one file; a copy of several parts
   is served as `hls`." Positions, `durationMs` and `segments` are then
   in the whole copy, which nothing else needs to say.
2. **List only the subtitles that can be served.** [video section 5.4](https://github.com/msfp-io/msfp-media/blob/main/spec/v0/video.md#54-choosing-the-subtitles)
   says "Every text and PGS subtitle track is listed". A Plex
   implementation without file access can't serve PGS raw, and subtitles
   are never burned in, so it would list tracks whose URLs always fail.
   Proposed: "Every text and PGS subtitle track the owning server can
   serve as section 6.2 says is listed, whichever is shown; a track it
   can't serve that way, and picture formats other than PGS, are not
   listed." The title's `subtitles` languages
   ([video section 3](../spec/v0/video.md#3-titles)) should follow the
   same rule: "the languages of the subtitle tracks `play` would list".

### 5.2 Worth stating

3. **Numbers aren't TMDB's by default** ([video section 4.1](https://github.com/msfp-io/msfp-media/blob/main/spec/v0/video.md#41-episode-order)).
   Add: "A media server's season and episode numbers are often not
   TMDB's (Emby and Jellyfin keep the files' numbers, often TheTVDB's;
   Plex follows the order chosen for the show). An owning server should
   key each episode by its TMDB episode ID, and look it up at TMDB when
   its media server doesn't store one; it uses its stored numbers only
   when it knows they are TMDB's." Add to the security or the
   implementation notes that such lookups need the owning server's own
   TMDB access, kept within TMDB's limits.
4. **Skip markers from other platforms** ([video section 5.3](https://github.com/msfp-io/msfp-media/blob/main/spec/v0/video.md#53-the-answer)).
   Add: "Credits are `outro`. A marker whose end the owning server doesn't
   know (credits that run to the end) ends at the copy's `durationMs`. An
   owning server that knows only some kinds sends those."
5. **Previews from BIF** ([video section 5.5](../spec/v0/video.md#55-seek-bar-previews)).
   Add: "An owning server whose media server keeps one image per interval
   (BIF, as Plex and Emby do) tiles them into sheets; it may send every
   *n*-th image, with `intervalMs` multiplied by *n*, to stay within the
   bounds."
6. **The default audio is the file's, not a person's**
   ([playing section 2.6](https://github.com/msfp-io/msfp-media/blob/main/spec/v0/playing.md#26-choosing-the-copy-and-the-tracks)).
   Plex and Emby keep a per-user chosen track. Add: "The default audio is
   the track the copy's file marks as default, never a choice a person of
   the owning server saved."
7. **Roles are best effort** ([video section 3.3](https://github.com/msfp-io/msfp-media/blob/main/spec/v0/video.md#33-streams-and-tracks)).
   Media servers rarely flag commentary. Add: "`role` is absent when the
   owning server doesn't know it; it may guess from a track's title. A
   viewer's server treats an absent role as the title's own sound." This
   also says what the audio language rule ("never a commentary or a
   description") does with an unflagged commentary: it may be chosen.
8. **Language codes** ([catalog section 3](../spec/v0/catalog.md#3-titles)).
   Add one sentence: "Media servers often give the file's ISO 639-2/B code
   (`fre`, `ger`, `chi`); it is sent as the ISO 639-1 code (`fr`, `de`,
   `zh`)." A test vector or two would help.
9. **Music IDs by tag name** ([music section 1](../spec/v0/music.md#1-terms)).
   Add: "Tagging tools write the recording MBID in the tag named
   'MusicBrainz Track Id', and the release track's in 'MusicBrainz Release
   Track Id': `track/mb/…` takes the first, `mbtrack` the second." Media
   servers name their fields after the tags, so the confusion is likely.
10. **The audio track in `file`** ([media section 1](../spec/v0/media.md#1-the-urls)).
    Add: "With `file`, the copy comes whole, with all its tracks; the
    viewer's player selects `audioTrack` itself."
11. **A separate service** (a new short note in the core's overview, or in
    [host.md](host.md)): "An implementation may run beside its media
    server rather than inside it, with its own address or behind the same
    reverse proxy with a discovery `base`. It then relays the media, and
    holds a credential of the media server; it should hold the narrowest
    one the media server allows (a dedicated account limited to the
    offered libraries)." This is the only way on Plex.

### 5.3 Fine as is

- Title keys by TMDB, with the `e<episode id>` form; `ids` with `imdb`
  and `tvdb`; season 0; `with` for several episodes in one copy. All three
  platforms give what they need, with the work above.
- Opaque library, version and playback IDs: every platform has internal
  IDs to hide, and the rule hides them.
- Library kinds as labels, with `""` for others: Plex's four types map.
- Milliseconds everywhere (Jellyfin and Emby count ticks, Plex ms and
  seconds): neutral.
- `video` height and `hdr`: every platform reports enough; Emby's
  sub-types match MSFP's base-layer rule exactly.
- The HLS subset: every platform's playlists need rewriting, and the
  subset allows TS and fMP4 segments, which all three produce.
- Coarsened capabilities and "nothing said: H.264 and AAC in HLS": all
  three can serve that.
- Track IDs as positions: each owning server numbers its own copy's
  tracks.
- Subtitle parts of 120 seconds: Emby and Jellyfin have time-range calls;
  Plex implementations cut them themselves.
- Preview sheets in JPEG within bounds, re-encoded: fits BIF sources once
  tiled (point 5).
- Album art from the Cover Art Archive by the viewer's server: needs only
  the release group MBID, which Emby and Jellyfin store.

## 6. Claims not confirmed

Each needs a test on a real server, or a vendor source:

**Plex**

- Byte ranges on `/library/parts/{partId}/{changestamp}/{filename}`.
- Episode `Guid` `tmdb://` values are TMDB episode IDs, on every episode
  (forum posts only; gaps reported).
- TMDB (aired) is the default episode order (forum posts only).
- Which MusicBrainz entity Plex's `mbid://` IDs name, per kind.
- The per-user stream selection: whether changing it between two starts
  affects a running transcode, and whether the start call has another
  way to pick the audio track.
- Whether `/transcode/universal/subtitles` or `subtitles=sidecar|segmented`
  gives a WebVTT track of an embedded text subtitle, whole or by range.
- That no API gives raw PGS (no endpoint found; Plex burns them in).
- The marker query parameter (`includeMarkers=1`, community code) and the
  stop call `/video/:/transcode/universal/stop?session=` (community).
- Whether terminating a session needs Plex Pass.
- That a managed or friend account's token can use the transcoder and
  part URLs; that a service on Plex's network is always counted local.
- The fields `forced`, `hearingImpaired`, `visualImpaired`, `DOVI*` come
  from python-plexapi, not the official document.
- Whether relaying Plex streams to another server fits Plex's terms.

**Emby**

- That episodes store their own TMDB ID.
- `IndexNumberEnd` for several episodes in one file; how a stacked item is
  served.
- That the subtitle call gives raw PGS (`Stream.sup`).
- Byte ranges on `static=true`; the shape of Emby's HLS playlists (paths,
  query strings).
- What `MusicBrainzTrack` holds (recording or release track).
- The `PlaybackInfo` decision flow, taken as Jellyfin's (its fork).
- Plugin routes at `/api/v1/federation/…` and `/.well-known/msfp`; the
  raw body in a service; no automatic compression; reproducing Emby's
  `404`.
- The version from which Emby's source was closed.

**Jellyfin**

- That Jellyfin has no commentary flag on audio streams (not checked
  against 12.2).
</content>
</invoke>
