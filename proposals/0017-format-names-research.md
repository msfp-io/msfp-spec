# MSFP format names: research for proposal 17, section 3.2

Research behind [proposal 17](0017-neutrality.md) section 3.2. The owner
decided afterwards (2026-11-23), so this report goes further than what
was adopted in places: the container is the IANA media type with **no**
`brand` field (the owner reads the brand to choose the media type);
`codecs` has no `+`; codec, subtitle and `objects` tokens live in the
registry, spelled as ffmpeg spells them where it has a name; `profile` is
display text; `level` and the audio `layout` are dropped.

2026-11-22. Read-only research: nothing was changed in the spec or the code.
Every claim below was checked against the source it cites, as fetched on 2026-11-22.
Lines marked "not checked" were not.

## 1. Short answer

- No outside registry covers every container, codec and subtitle format a
  media server meets. Each one has gaps (section 5).
- What players actually test is a **media type plus an RFC 6381 `codecs`
  string**. MSE, HTML `canPlayType`, AVPlayer, HLS and DASH all work this way,
  and Media3/ExoPlayer maps the strings to its own MIME types.
- Recommendation:
  1. For the container, use an **IANA media type**, plus the ISO BMFF
     **brand** for the MP4/MOV family.
  2. For each video and audio stream, a **`codecs` string** (recommended,
     not required). This is the fact a player tests.
  3. Keep a short, open **codec family token**. MSFP's registry file defines
     it, with a crosswalk table to MP4RA, Matroska, IANA and ffmpeg. The
     tokens are spelled as ffmpeg spells them where ffmpeg has a name, but
     the registry is the authority, not ffmpeg.
  4. Profile stays display text.

  The one big change from proposal 17 is the container. ffmpeg's muxer name
  can't be read from a file, even by ffprobe.

## 2. The ffmpeg gap, confirmed

- ffmpeg 7.1.5 on a test machine (`ffmpeg -muxers`, `ffprobe -demuxers`):
  - The demuxers are `mov,mp4,m4a,3gp,3g2,mj2` and `matroska,webm`.
  - The muxers are separate names: `mp4`, `mov`, `ipod` (that is M4V/M4A),
    `3gp`, `ismv`, `psp`, `f4v`, `matroska`, `webm`.
  - ffprobe reports only the demuxer list. So "the muxer name" is a guess
    from the extension or the brand.
  - An M4V file's muxer is `ipod`, which nobody would guess.
- ffprobe prints no RFC 6381 string. Its options have none, and its stream
  fields stop at `codec_name`, `codec_tag`, `profile` and `level`. A producer
  using ffprobe has to build the string itself.
- ffmpeg's codec names are defined only in libavcodec's `codec_desc.c`. The
  DTS descriptor is named `dts` there
  ([codec_desc.c](https://github.com/FFmpeg/FFmpeg/blob/master/libavcodec/codec_desc.c)),
  but Plex reports `dca`, the decoder's older name (section 6). Products
  that "use ffmpeg names" already disagree with each other.

## 3. Containers

**IANA registry** (CSV files fetched 2026-11-22:
[video](https://www.iana.org/assignments/media-types/video.csv),
[audio](https://www.iana.org/assignments/media-types/audio.csv),
[text](https://www.iana.org/assignments/media-types/text.csv),
[application](https://www.iana.org/assignments/media-types/application.csv)):

| Format | Media type | Registered? |
|---|---|---|
| MP4 | `video/mp4`, `audio/mp4` | yes ([RFC 4337](https://www.rfc-editor.org/rfc/rfc4337), [RFC 6381](https://www.rfc-editor.org/rfc/rfc6381)) |
| QuickTime | `video/quicktime` | yes (RFC 6381; Paul Lindner) |
| Matroska | `video/matroska`, `audio/matroska`, `video/matroska-3d` | yes ([RFC 9559](https://www.rfc-editor.org/rfc/rfc9559) section 27.18; "Optional parameters: N/A", so no `codecs` parameter is defined for it) |
| WebM | `video/webm`, `audio/webm` | **not in IANA**. Defined by the WebM project and the W3C MSE registry (below) |
| MPEG-TS | `video/MP2T` | yes ([RFC 3555](https://www.rfc-editor.org/rfc/rfc3555)); media types are case-insensitive, so lowercase `video/mp2t` is the same type |
| AVI | none (`video/vnd.avi` is not in the registry; `video/x-msvideo` is de facto only) | **no** |
| Ogg | `video/ogg`, `audio/ogg` | yes (RFC 5334, RFC 7845) |
| 3GPP, MJ2, MPEG-PS | `video/3gpp`, `video/3gpp2`, `video/mj2`, `video/mpeg` | yes |
| HLS / DASH manifests | `application/vnd.apple.mpegurl`, `application/dash+xml` | yes |

**Telling files apart inside a family:**

- **ISO BMFF brands.**
  [RFC 6381 section 4](https://www.rfc-editor.org/rfc/rfc6381#section-4)
  defines a `profiles` parameter that carries MP4RA-registered brands. A
  file's major brand is the first 4 bytes after `ftyp`: `isom`, `mp42`,
  `qt  `, `M4V `, `3gp4` and so on. Brands are listed at
  [mp4ra.org](https://mp4ra.org/registered-types/brands). ffprobe shows the
  major brand as the `major_brand` format tag.
- **Matroska DocType.** RFC 9559 section 4.3 says Matroska's DocType is
  `matroska`. The [WebM guidelines](https://www.webmproject.org/docs/container/)
  say a WebM file's DocType "SHOULD be `webm`". WebM is "a subset of the
  Matroska multimedia container format", with only VP8/VP9 video and
  Vorbis/Opus audio in its first release. So calling a WebM file
  `video/matroska` is less precise, but never wrong.

  ffprobe doesn't show the DocType. Jellyfin guesses it instead: it keeps
  `webm` only when every stream is VP8/VP9/AV1 or Opus/Vorbis
  ([ProbeResultNormalizer.cs](https://github.com/jellyfin/jellyfin/blob/master/MediaBrowser.MediaEncoding/Probing/ProbeResultNormalizer.cs),
  `NormalizeFormat`).

**What players decide on:**

- **MSE.** `isTypeSupported(type)` takes a MIME type string. It returns
  false for an "unsupported media type or subtype", an unsupported codec, or
  an unsupported combination
  ([Media Source Extensions](https://www.w3.org/TR/media-source-2/)). The
  [byte stream format registry](https://www.w3.org/TR/mse-byte-stream-format-registry/)
  lists only `audio/webm`/`video/webm`, `audio/mp4`/`video/mp4`,
  `audio/mp2t`/`video/mp2t`, and `audio/mpeg`/`audio/aac`. **Matroska is not
  in it.**
- **HTML.** `canPlayType(type)` takes a "MIME type, in some cases with a
  codecs parameter" ([HTML standard](https://html.spec.whatwg.org/multipage/media.html)).
  jellyfin-web asks it things like `video/mp4; codecs="hvc1.1.L120"`
  ([browserDeviceProfile.js](https://github.com/jellyfin/jellyfin-web/blob/master/src/scripts/browserDeviceProfile.js)).
- **AVPlayer.** `AVURLAsset.isPlayableExtendedMIMEType(_:)` takes "an
  extended MIME type string such as `video/3gpp2; codecs="mp4v.20.9,
  mp4a.E1"`"
  ([Apple](https://developer.apple.com/documentation/avfoundation/avurlasset/isplayableextendedmimetype(_:))).
- **Media3/ExoPlayer.** Uses MIME types internally: `video/mp4`,
  `video/webm`, and the unregistered `video/x-matroska`. It maps RFC 6381
  strings to codec MIME types in `getMediaMimeType`
  ([MimeTypes.java](https://github.com/androidx/media/blob/release/libraries/common/src/main/java/androidx/media3/common/MimeTypes.java)).
- **HLS.** `CODECS` holds identifiers from "the ISO Base Media File Format
  Name Space defined by [RFC6381]". The new `SUPPLEMENTAL-CODECS` gives
  enhancement layers as `<codec>/<brand>`, for example `dvh1.08.07/db4h`.
  `VIDEO-RANGE` is `SDR`, `HLG` or `PQ`. Segments may only be in the formats
  of section 3.1 (MPEG-TS, fMP4, packed audio, WebVTT…)
  ([draft-pantos-hls-rfc8216bis-22](https://www.ietf.org/archive/id/draft-pantos-hls-rfc8216bis-22.txt),
  sections 3.1 and 4.4.6.2; IANA lists -22 as the reference for
  `application/vnd.apple.mpegurl`).
- **DASH.** `@mimeType` plus `@codecs` in RFC 6381 form. Not checked here:
  ISO/IEC 23009-1 is paywalled. The VP9 binding says its string is required
  by "DASH and other applications".

## 4. Codecs

**RFC 6381 itself.** For ISO BMFF and QuickTime, the first element is "a
sample description entry four-character code as registered by the MP4
Registration Authority", and "Values are case sensitive"
([RFC 6381 section 3.3](https://www.rfc-editor.org/rfc/rfc6381#section-3.3)).
The RFC spells out the details only for `avc1` (profile, constraint flags,
level, in hex) and `mp4a`/`mp4v` (OTI). Other codecs define their strings in
their own ISO BMFF binding.

**Coverage.** Sample entries come from the
[MP4RA codecs list](https://mp4ra.org/registered-types/codecs), checked
2026-11-22. Matroska IDs come from the
[Matroska codec mappings](https://www.matroska.org/technical/codec_specs.html).

| Codec | RFC 6381 / MP4RA | String rules | Matroska | IANA codec type |
|---|---|---|---|---|
| H.264 | `avc1`, `avc3` | RFC 6381 | `V_MPEG4/ISO/AVC` | `video/H264` |
| HEVC | `hvc1`, `hev1` | ISO/IEC 14496-15 Annex E (paywalled) | `V_MPEGH/ISO/HEVC` | `video/H265` |
| VVC | `vvc1`, `vvi1` | 14496-15 | `V_MPEGI/ISO/VVC` | — |
| AV1 | `av01` | [AV1-ISOBMFF](https://aomediacodec.github.io/av1-isobmff/): `av01.P.LLT.DD[.M.CCC.cp.tc.mc.F]` | `V_AV1` | `video/AV1` |
| VP9 | `vp09` | [VP codec ISO BMFF](https://www.webmproject.org/vp9/mp4/): `vp09.PP.LL.DD[…]` | `V_VP9` | `video/VP9` |
| Dolby Vision | `dvh1`, `dvhe`, `dva1`, `dvav` | Dolby spec; HLS example `dvh1.08.07` | no codec ID: `dvcC`/`dvvC`/`dvwC` block addition mappings ([Matroska](https://www.matroska.org/technical/block_additional_mappings.html)) | — |
| ProRes | `apcn`, `apch`… (QuickTime section only) | — | `V_PRORES` | — |
| AAC | `mp4a.40.x` | RFC 6381 | `A_AAC` (+ old variants) | `audio/aac` |
| AC-3 / E-AC-3 | `ac-3`, `ec-3` | the 4CC alone | `A_AC3`, `A_EAC3` | `audio/ac3`, `audio/eac3` |
| AC-4 | `ac-4` | ETSI | — | — |
| Opus | `Opus` (capital O) | Opus ISO BMFF | `A_OPUS` | `audio/opus` |
| FLAC | `fLaC` | FLAC ISO BMFF | `A_FLAC` | `audio/flac` |
| DTS / DTS-HD | `dtsc` (core), `dtsh` (core + extension), `dtsl` (lossless only), `dtse` (LBR/Express) | DTS-HD | `A_DTS` (covers DTS-HD MA), `A_DTS/LOSSLESS`, `A_DTS/EXPRESS` | `audio/vnd.dts`, `audio/vnd.dts.hd` |
| DTS-UHD | `dtsx` (profile 2), `dtsy` | DTS-UHD | — | `audio/vnd.dts.uhd` |
| TrueHD (MLP) | `mlpa` | Dolby MLP | `A_TRUEHD`, `A_MLP` | `audio/vnd.dolby.mlp` |
| MPEG-H | `mha1`… | — | — | `audio/mhas` |
| PCM | `ipcm` (+ QuickTime 4CCs) | — | `A_PCM/INT/LIT`… | `audio/L16`, `audio/L24` |

**Where no registry exists:**

- **DTS:X.** It rides on DTS-HD MA, so no 4CC names it. MP4RA's `dtsx` is
  DTS-UHD profile 2, a different codec. Media3 kept a constant named
  `AUDIO_DTS_X` but deprecated it in favour of `AUDIO_DTS_UHD_P2`.
  - ffmpeg shows DTS:X only through `profile`: Kodi's
    `AV_PROFILE_DTS_HD_MA_X` turns into its own `dtshd_ma_x`
    ([Kodi StreamUtils.cpp](https://github.com/xbmc/xbmc/blob/master/xbmc/utils/StreamUtils.cpp)).
- **Atmos.**
  - TrueHD + Atmos has no identifier of its own.
  - E-AC-3 JOC is `ec+3` in Media3 (`CODEC_E_AC3_JOC`), which is not an
    MP4RA entry.
  - HLS signals object audio in `CHANNELS` instead (its "Audio Coding
    Identifiers" parameter, draft section 4.4.6.1).
- **Others use one of three things:** their own MIME-like types (Media3
  `audio/true-hd`, `video/dolby-vision`), their own tokens (Kodi
  `truehd_atmos`, Plex `dca-ma`), or a profile text.

**Case and spelling drift in practice.** GStreamer's RFC 6381 converter
writes `opus` and `flac` in lowercase, and `hev1` for HEVC
([codec-utils.c](https://github.com/GStreamer/gstreamer/blob/main/subprojects/gst-plugins-base/gst-libs/gst/pbutils/codec-utils.c)).
MP4RA registers `Opus` and `fLaC`, and Media3 matches the lowercase forms.
Consumers should compare 4CCs as written, but be lenient.

## 5. Subtitles

- **IANA:** only `text/vtt`, `application/ttml+xml` and
  `text/vnd.dvb.subtitle`. There is nothing for SubRip, SSA/ASS, PGS or
  VobSub.
- **MP4RA:** `wvtt` (WebVTT), `stpp` (TTML), `tx3g` (3GPP timed text),
  `c608` (CEA-608, QuickTime section). Nothing for PGS, ASS or VobSub.
- **Matroska** has the widest coverage: `S_TEXT/UTF8`, `S_TEXT/ASS`,
  `S_TEXT/SSA`, `S_TEXT/WEBVTT`, `S_TEXT/USF`, `S_HDMV/PGS`,
  `S_HDMV/TEXTST`, `S_VOBSUB`, `S_DVBSUB`, `S_ARIBSUB`, `S_KATE`,
  `S_IMAGE/BMP`.
- **ffmpeg:** `subrip`, `ass`, `webvtt`, `mov_text`, `hdmv_pgs_subtitle`,
  `dvd_subtitle`, `dvb_subtitle`…
- **Media3** makes up unregistered types: `application/x-subrip`,
  `text/x-ssa`, `application/pgs`, `application/vobsub`,
  `application/dvbsubs`.

So for subtitles there is no outside vocabulary that both covers the field
and is unique.

## 6. What servers and libraries use

| Product | Container | Codec | Derived from ffmpeg? |
|---|---|---|---|
| Jellyfin | ffprobe `format_name`, rewritten: `matroska`→`mkv`, `mpegts`→`ts`, `webm` dropped unless the streams fit. The MP4 list stays a list ([ProbeResultNormalizer.cs](https://github.com/jellyfin/jellyfin/blob/master/MediaBrowser.MediaEncoding/Probing/ProbeResultNormalizer.cs)) | ffprobe `codec_name`, plus some of its own (`PGSSUB` for .sup) | yes. jellyfin-web turns these into MIME types with `codecs` for `canPlayType` |
| Emby | not checked; Jellyfin forked from it and shares the ffprobe base | — | likely yes (not checked) |
| Plex | as read by the unofficial `plex-api` crate: `mkv`, `mp4`, `m4v`, `mpegts`, `avi`… | `h264`, `hevc`, `mpeg2video`, `msmpeg4v3`, `dca`, `dca-ma`, `truehd`, `eac3`; subtitles `srt`, `subrip`, `ass`, `pgs`, `dvd_subtitle`, `mov_text`. Streams also carry a `codecID` ([plex-api source](https://docs.rs/crate/plex-api/latest/source/src/media_container/server/library/mod.rs)) | mostly ffmpeg, with its own changes. Plex's official docs don't list the values |
| Kodi | — | ffmpeg's codec ID turned into its own names (`dtshd_ma`, `dtshd_ma_x`, `truehd_atmos`, `eac3_ddp_atmos`…) ([StreamUtils.cpp](https://github.com/xbmc/xbmc/blob/master/xbmc/utils/StreamUtils.cpp), [DVDDemuxFFmpeg.cpp](https://github.com/xbmc/xbmc/blob/master/xbmc/cores/VideoPlayer/DVDDemuxers/DVDDemuxFFmpeg.cpp)) | yes, then extended |
| MediaInfo | its own `Format`: `Matroska`, `WebM`, `MPEG-4` (MP4 and MOV together; `Format_Profile` separates them) | its own `Format` (`AVC`, `HEVC`, `E-AC-3`, `DTS`, `TrueHD`, `Opus`). `CodecID` is the container's own ID (`V_MPEG4/ISO/AVC`, `avc1`, `hvc1`). Its database also lists an internet media type per format ([MediaInfoLib DataBase](https://github.com/MediaArea/MediaInfoLib/tree/master/Source/Resource/Text/DataBase)) | **no**: an independent parser |
| GStreamer | caps: its own MIME-like types (`video/x-h264`, `audio/x-ac3`…) | — | **no**. It converts caps to RFC 6381 strings and back (`gst_codec_utils_caps_get_mime_codec`, codec-utils.c) |
| Media3 | MIME types (above) | RFC 6381 in, MIME types out | no |
| DLNA | `protocolInfo` = MIME type + a `DLNA.ORG_PN` profile name | — | no. Not checked in detail: the DLNA guidelines are not public |

The pattern:

- **Servers** (Jellyfin, Plex, Kodi, likely Emby) use ffmpeg-derived names,
  each with its own changes.
- **Players and parsers that don't use ffmpeg** (browsers, AVPlayer,
  Media3, GStreamer, HLS/DASH) meet on **media type + RFC 6381**.
- **MediaInfo** keeps the container's native `CodecID`, which is what a
  parser reads from the file anyway.

## 7. Options and their cost

Producers:

- **Go and .NET engines:** they take the facts from their host, and only
  their ShareFolder examples run ffprobe.
- **Jellyfin plugin:** Jellyfin's MediaSource and MediaStream values.
- **Overfin:** the same values, through Jellyfin's API.

**A. Proposal 17 as written** (ffmpeg muxer + codec_name + profile, optional
`codecs`).

- Spec: none beyond proposal 17.
- Producers:
  - The muxer name can't be read from a file, so it's guessed from the
    extension.
  - A producer without ffmpeg must copy ffmpeg's tables.
  - The tie to one project stays.

**B. Container as a media type + brand; codec family token from MSFP's
registry, spelled as ffmpeg spells it; `codecs` recommended** (my
recommendation, details in section 8).

- Spec:
  - Section 3.2 rewritten.
  - A second value class added in core 8.5 (media type).
  - A crosswalk table in the registry file proposal 17 already adds.
  - Vectors updated.
- Engines: one new validator and a mapping table for the examples. Small.
- Jellyfin plugin, Overfin:
  - Codec names: zero cost, since they already are the ffmpeg spellings.
  - Container: map Jellyfin's `mkv`/`webm`/`ts`/MP4-list to a media type
    with a small table.
  - The MP4-versus-MOV split needs the major brand. That is 12 bytes of
    `ftyp` read from the file. Not checked: whether Jellyfin exposes
    `major_brand`.
  - `codecs` needs bytes Jellyfin doesn't keep. AVC needs the constraint
    flags; HEVC needs the tier and constraint bytes. So it is either
    approximated from profile + level, or built by reading `avcC`/`hvcC`.
    This is the biggest producer cost, which is why it should stay
    recommended, not required.
- A producer without ffmpeg (Go, .NET) reads `ftyp`, the EBML DocType and
  the codec config records. These are well-documented, small parsers.

**C. RFC 6381 4CC as the only codec name** (drop the family token).

- Spec: small.
- But it is not one name per codec: `avc1`/`avc3`, `hvc1`/`hev1` and
  `dvh1`/`dvhe` describe how a codec is stored in MP4, not which codec it
  is.
- It is case-sensitive (`Opus`, `fLaC`), so it clashes with core 8.5.
- It has nothing for PGS, ASS, VobSub or DTS:X, and Matroska-only codecs
  (`V_PRORES` aside) have no MP4 entry.

**D. Matroska codec IDs.**

- Wide coverage, subtitles included, under a published RFC 9559 registry.
- But the IDs contain `/` and uppercase, are Matroska-centred (MP4 files
  would be named by Matroska), and have no Dolby Vision ID.

**E. IANA codec media types** (`video/H264`, `audio/opus`…). They are
RTP-oriented and have gaps (no ProRes, AC-4, PGS, ASS or VobSub). No player
tests against them for files. Not worth it.

## 8. Recommendation

1. **`container`** becomes `mediaType`, the IANA media type of the file,
   lowercase, with no parameters: `video/mp4`, `video/quicktime`,
   `video/matroska`, `video/webm`, `video/mp2t`, `video/ogg`, `audio/mp4`,
   `audio/flac`…
   - Where IANA has nothing, use the de facto type that MSFP's registry
     names: `video/webm`, `video/x-msvideo` for AVI.
   - Add an optional `brand` for the ISO BMFF family: the `ftyp` major
     brand, 4 characters, as in RFC 6381 `profiles`.
   - A WebM file may be sent as `video/matroska` when the DocType is
     unknown. That is less precise, but true.
   - This is what MSE, HTML, AVPlayer and Media3 test, and any
     implementation can read it from the file's first bytes.
2. **`codecs`** (RFC 6381), on the video stream and each audio track:
   RECOMMENDED, the primary playability fact.
   - Its character set needs `+` for `ec+3`, and it should take a
     supplemental part for Dolby Vision 8.x as HLS does
     (`hvc1…` + `dvh1.08.07/db4h`), or let the existing `dv` record cover
     that.
3. **`codec`** (required, open token) stays, as the plain family name.
   - The registry file defines it and lists, for each token, the MP4RA
     4CCs, the Matroska ID, any IANA type, and the ffmpeg `codec_name`.
   - The tokens match ffmpeg's names where ffmpeg has one (`h264`, `hevc`,
     `eac3`, `truehd`, `dts`), so ffprobe-based producers have nothing to
     change.
   - The spec says "the codec, as the registry names it", not "as ffmpeg
     names it".
   - A codec ffmpeg doesn't know gets a token in the registry, and
     unknown tokens are kept meanwhile. This removes the dependence without
     costing anything today.
4. **Subtitle `format`**: the same model, an open token from the registry,
   spelled as ffmpeg spells it.
   - The crosswalk lists the Matroska `S_` ID, the MP4 4CC
     (`wvtt`/`stpp`/`tx3g`) and the IANA type where there is one.
   - An optional `mediaType` only where one is registered (`text/vtt`,
     `application/ttml+xml`).
5. **`profile`** stays display text "as the producer's tool prints it". It
   is not for decisions: ffmpeg and MediaInfo print different texts.
   - Video profile and level are in `codecs` anyway.
   - Object audio (Atmos, DTS:X) has no outside vocabulary. Whether to add
     an open token for it is a separate decision for the owner. It is
     flagged here, not decided.
6. **ffmpeg names as an optional extra field: no.**
   - With tokens spelled as ffmpeg spells them, the extra field would
     duplicate them.
   - Where they differ (Kodi, Plex), a third name helps nobody.
   - The crosswalk table does that job once, in the registry.

## 9. Conflicts with proposal 17

- **Core 8.5's token rule** (1 to 32 characters of `a–z 0–9 . _ -`):
  - Media types need `/` and `+`, and up to 127 characters under
    [RFC 6838](https://www.rfc-editor.org/rfc/rfc6838).
  - Suggestion: a second class, "media type". Lowercase, exactly one `/`,
    characters `a–z 0–9 . + - _`, at most 64, no parameters. Unknown values
    are kept, as for tokens.
- **Proposal 17's `codecs` class** (`A–Z a–z 0–9 . -`, at most 64):
  - It needs `+` (`ec+3`) and, if supplemental codecs are carried, `/`.
  - Comparison is case-sensitive under RFC 6381, but GStreamer and Media3
    show lowercase variants in use. Say so in the notes.
- **The `container` targets in `delivery` requests** (remux/transcode,
  `msfp-go` remote.go) use ffmpeg names. They would move to the media type
  too, and the engines' `Encoders` config comment ("as ffmpeg names them")
  changes with them.
- **The "closed lists" objection in proposal 17's problem statement.**
  Point 3 above is a registry with open values. Adding a row there is not a
  spec change, so it stays inside proposal 17's rule.
- **Section 3.3's `colour` "H.273 names as ffmpeg prints them".** This has
  the same issue in small: H.273 defines numbers, ffmpeg the names. Sending
  the H.273 numbers, or names from the registry, would be consistent with
  the rest. Not researched further.
