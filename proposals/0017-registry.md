# Proposal 17: the registry's first tables

The first content of the lists [proposal 17](0017-neutrality.md)
section 12 puts in `spec/registry.md`, beside the key and `ids` schemes
already there. The spec pass copies it. Gathered 2026-11-22 from the
sources in section 13; the research behind it is
[0017-format-names-research.md](0017-format-names-research.md).

How to read it:

- **The token column is normative.** Every other column is an
  informative crosswalk: where the token comes from, and what other
  vocabularies call the same thing. Products (ffmpeg, Jellyfin) appear
  only as sources.
- Tokens are 1 to 32 characters of `a–z 0–9 . _ -` (music types:
  section 11). Codec and subtitle tokens are spelled as ffmpeg spells
  its `codec_name` where it has one.
- **—**: the source has no entry (searched, none found).
- **UNVERIFIED**: a cell not checked against a primary source. Cells not
  marked were checked against section 13's sources.
- **MSFP-coined**: a token with no ffmpeg name, which MSFP makes up.
- **Open**: an entry or a choice still to be decided (section 12 lists
  them).

## 1. Video codecs (`codec` on the video stream)

"MP4RA" lists the sample entries, the first element of `codecs`. "QT"
is MP4RA's QuickTime list; "OTI" the `mp4v`/`mp4a` object type
indication. **`codecs` required** marks the four video codecs for which
proposal 17 section 3.3 requires `codecs`.

| Token | Codec | ffmpeg | MP4RA | Matroska CodecID | IANA type (RTP) | Notes |
|---|---|---|---|---|---|---|
| `h264` | H.264 / AVC | `h264` | `avc1`, `avc2`, `avc3`, `avc4`; on Dolby Vision `dva1`, `dvav` | `V_MPEG4/ISO/AVC` | `video/H264` (RFC 6184) | `codecs` required |
| `hevc` | H.265 / HEVC | `hevc` | `hvc1`, `hev1` (`hvc2`, `hvc3`, `hev2`, `hev3` with extractors); on Dolby Vision `dvh1`, `dvhe` | `V_MPEGH/ISO/HEVC` | `video/H265` (RFC 7798) | `codecs` required |
| `vvc` | H.266 / VVC | `vvc` | `vvc1`, `vvi1` | `V_MPEGI/ISO/VVC` | `video/H266` (RFC 9328) | |
| `av1` | AV1 | `av1` | `av01`; on Dolby Vision `dav1` | `V_AV1` | `video/AV1` | `codecs` required |
| `vp8` | VP8 | `vp8` | `vp08` | `V_VP8` | `video/VP8` (RFC 7741) | |
| `vp9` | VP9 | `vp9` | `vp09` (OTI 0xB1) | `V_VP9` | `video/VP9` (RFC 9628) | `codecs` required |
| `mpeg1video` | MPEG-1 video | `mpeg1video` | `mp4v`, OTI 0x6A | `V_MPEG1` | `video/MPV` (RFC 3555) | |
| `mpeg2video` | MPEG-2 video (H.262) | `mpeg2video` | `mp4v`, OTI 0x60 to 0x65 | `V_MPEG2` | `video/MPV` (RFC 3555) | DVD, Blu-ray, broadcast |
| `mpeg4` | MPEG-4 Part 2 (DivX 4+, Xvid) | `mpeg4` | `mp4v`, OTI 0x20 | `V_MPEG4/ISO/SP`, `/ASP`, `/AP` | `video/MP4V-ES` (RFC 6416) | in AVI: FourCC `XVID`, `DX50`… |
| `msmpeg4v3` | Microsoft MPEG-4 v3 (DivX 3) | `msmpeg4v3` | — | `V_MPEG4/MS/V3` | — | AVI FourCC `DIV3`, `MP43`… |
| `msmpeg4v2` | Microsoft MPEG-4 v2 | `msmpeg4v2` | — | `V_MS/VFW/FOURCC` + `MP42` | — | |
| `wmv1`, `wmv2` | Windows Media Video 7, 8 | same | — | `V_MS/VFW/FOURCC` + `WMV1`, `WMV2` | — | usually in ASF |
| `wmv3` | Windows Media Video 9 (VC-1 Simple, Main) | `wmv3` | — | `V_MS/VFW/FOURCC` + `WMV3` | — | |
| `vc1` | SMPTE VC-1 (Advanced) | `vc1` | `vc-1` (OTI 0xA3) | `V_VC1`; ffmpeg reads VC-1 only as `V_MS/VFW/FOURCC` + `WVC1` | `video/vc1` (RFC 4425) | |
| `h263` | H.263 | `h263` | `s263` (3GPP); QT `h263` | — | `video/H263` (RFC 3555) | |
| `prores` | Apple ProRes 422, 4444 | `prores` | QT `apco`, `apcs`, `apcn`, `apch`, `ap4h`, `ap4x` | `V_PRORES` | — | |
| `prores_raw` | Apple ProRes RAW | `prores_raw` | QT `aprn`, `aprh` | — | — | |
| `mjpeg` | Motion JPEG | `mjpeg` | QT `jpeg`, `mjpa`, `mjpb`; `mp4v` OTI 0x6C | `V_MJPEG` | `video/JPEG` (RFC 3555) | cover art is an attached picture, not a video stream |
| `jpeg2000` | JPEG 2000 | `jpeg2000` | `mjp2` | `V_JPEG2000` | `video/jpeg2000` (RFC 5371) | |
| `theora` | Theora | `theora` | — | `V_THEORA` | — | mostly in Ogg |
| `dvvideo` | DV | `dvvideo` | QT `dvc `, `dvcp`… (from ffmpeg's tables; MP4RA's QT list has only `dvh5`, `dvh6`, `dvhp`) | — | `video/DV` (RFC 6469) | |
| `rv30`, `rv40` | RealVideo 3, 4 | same | — | `V_REAL/RV30`, `V_REAL/RV40` | — | |
| `flv1`, `vp6f` | Sorenson Spark, VP6 (Flash) | same | — | — | — | FLV only |
| `ffv1` | FFV1 | `ffv1` | — | `V_FFV1` | — | |
| `dnxhd` | Avid DNxHD, DNxHR | `dnxhd` | QT `AVdn` (UNVERIFIED) | — | — | |
| `rawvideo` | uncompressed | `rawvideo` | QT `raw `, `2vuy`…; ISO `uncv` | `V_UNCOMPRESSED` | `video/raw` (RFC 4175) | |
| `evc` | MPEG-5 EVC | `evc` | `evc1` | — | — | |
| `apv` | Advanced Professional Video | `apv` | `apv1` | — | — | |
| `avs2`, `avs3`, `cavs` | AVS family | same | `avst`, `avs3`, — | `V_AVS2`, `V_AVS3`, `V_CAVS` | — | |

Not codec tokens: **Dolby Vision** (its sample entries map to the base
codec's token; the layer is the copy's `dv` object) and **encrypted
entries** (`encv`, `enca`: the real codec's token is sent).

## 2. Audio codecs (`codec` on an audio track or a music copy)

**Fixed `codecs`** is the code a producer sends where `codecs` is
optional (proposal 17 section 3.3).

| Token | Codec | ffmpeg | MP4RA | Matroska CodecID | IANA type | Fixed `codecs` / notes |
|---|---|---|---|---|---|---|
| `aac` | AAC (LC, HE, HE v2, xHE, LD, ELD…) | `aac` | `mp4a` OTI 0x40 (MPEG-4), 0x66 to 0x68 (MPEG-2) | `A_AAC`, and the old `A_AAC/MPEG2/*`, `A_AAC/MPEG4/*` | `audio/aac` (ADTS file); `audio/mp4a-latm`, `audio/mpeg4-generic` (RTP) | `codecs` required (`mp4a.40.2`, `.5`, `.29`…) |
| `aac_latm` | AAC in LATM syntax | `aac_latm` | — | — | `audio/MP4A-LATM` (RFC 6416) | broadcast TS |
| `ac3` | Dolby Digital (AC-3) | `ac3` | `ac-3` | `A_AC3` (also `/BSID9`, `/BSID10`) | `audio/ac3` (RFC 4184) | `ac-3` |
| `eac3` | Dolby Digital Plus (E-AC-3) | `eac3` | `ec-3` | `A_EAC3` | `audio/eac3` (RFC 4598) | `ec-3`; Atmos in `objects` (section 5) |
| `ac4` | Dolby AC-4 | `ac4` | `ac-4` | — | — | |
| `truehd` | Dolby TrueHD | `truehd` | `mlpa` | `A_TRUEHD` | `audio/vnd.dolby.mlp` | `mlpa` |
| `mlp` | Meridian Lossless Packing (DVD-Audio) | `mlp` | UNVERIFIED (`mlpa` may cover it) | `A_MLP` | `audio/vnd.dolby.mlp` | |
| `dts` | DTS Coherent Acoustics and every DTS-HD flavour | `dts` (Plex reports `dca`) | `dtsc`, `dtsh`, `dtsl`, `dtse` (layered `dts+`, `dts-`) | `A_DTS`, `A_DTS/LOSSLESS`, `A_DTS/EXPRESS` | `audio/vnd.dts`; `audio/vnd.dts.hd` | `codecs` required; flavours in section 2.1 |
| `dtsuhd` | DTS-UHD (DTS:X profiles 2, 3) | — (ffmpeg has none) | `dtsx` (OTI 0xB2), `dtsy` (0xB3) | — | `audio/vnd.dts.uhd` | MSFP-coined; spelling open (`dtsuhd` or `dts_uhd`) |
| `mp3` | MPEG-1, 2 Layer III | `mp3` | `mp4a` OTI 0x6B (MPEG-1), 0x69 (MPEG-2) | `A_MPEG/L3` | `audio/mpeg` (RFC 3003; file); `audio/MPA` (RTP) | `mp4a.40.34` as Jellyfin sends it, or `mp4a.6B` (UNVERIFIED which players prefer) |
| `mp2` | MPEG-1, 2 Layer II | `mp2` | `mp4a` OTI 0x6B, 0x69 | `A_MPEG/L2` | `audio/mpeg`, `audio/MPA` | DVD, DVB |
| `mp1` | MPEG Layer I | `mp1` | — | `A_MPEG/L1` | `audio/MPA` | rare |
| `opus` | Opus | `opus` | `Opus` (OTI 0xAD) | `A_OPUS` | `audio/opus` (RFC 7587, RTP only) | `Opus` |
| `vorbis` | Vorbis | `vorbis` | — | `A_VORBIS` | `audio/vorbis` (RFC 5215) | |
| `flac` | FLAC | `flac` | `fLaC` | `A_FLAC` | `audio/flac` (RFC 9639) | `fLaC` |
| `alac` | Apple Lossless | `alac` | `alac` | `A_ALAC` | — | `alac` |
| `wmav1`, `wmav2` | Windows Media Audio 1, 2 | same | — | `A_MS/ACM` + WAVE tag 0x0160, 0x0161 | — | usually in ASF |
| `wmapro`, `wmalossless` | WMA 9 Pro, WMA Lossless | same | — | `A_MS/ACM` + 0x0162, 0x0163 | — | |
| `mpegh_3d_audio` | MPEG-H 3D Audio | `mpegh_3d_audio` | `mha1`, `mha2`, `mhm1`, `mhm2` | — | `audio/mhas` | |
| `wavpack` | WavPack | `wavpack` | — | `A_WAVPACK4` | — | music |
| `tta` | True Audio | `tta` | — | `A_TTA1` | — | music |
| `ape` | Monkey's Audio | `ape` | — | — | — | music |
| `dsd_lsbf`, `dsd_msbf`, `dsd_lsbf_planar`, `dsd_msbf_planar`, `dst` | DSD, DST | same | — | — | — | music (DSF, DFF, SACD) |
| `cook`, `ra_288`, `atrac3` | RealAudio | same | — | `A_REAL/COOK`, `A_REAL/28_8`, `A_REAL/ATRC` | — | |
| `amr_nb`, `amr_wb` | AMR | same | UNVERIFIED (`samr`, `sawb`, 3GPP) | — | UNVERIFIED (`audio/AMR`) | phone videos |
| `pcm_…` | PCM | same | section 2.2 | section 2.2 | section 2.2 | |

**IAMF** has no codec token: ffmpeg carries it as a stream group over
Opus, AAC, FLAC or PCM, not as a codec. Open.

### 2.1 DTS flavours

One token, `dts`; the flavour shows in `codecs` only as far as the
sample entry tells it.

| Flavour | ffmpeg profile | MP4RA | Matroska | `audio/vnd.dts.hd` profile |
|---|---|---|---|---|
| DTS (core) | `DTS` | `dtsc` | `A_DTS` | `DTS` (or `audio/vnd.dts`) |
| DTS-ES | `DTS-ES` | `dtsc` (UNVERIFIED) | `A_DTS` | UNVERIFIED |
| DTS 96/24 | `DTS 96/24` | `dtsc` (UNVERIFIED) | `A_DTS` | UNVERIFIED |
| DTS-HD High Resolution | `DTS-HD HRA` | `dtsh` | `A_DTS` | `DTSHR` |
| DTS-HD Master Audio, with core | `DTS-HD MA` | `dtsh` | `A_DTS` | `DTSMA` |
| DTS-HD Master Audio, lossless only | `DTS-HD MA` | `dtsl` | `A_DTS/LOSSLESS` | `DTSMA` |
| DTS Express (LBR) | `DTS Express` | `dtse` | `A_DTS/EXPRESS` | `LBR` |
| DTS-HD MA + DTS:X (and IMAX) | `DTS-HD MA + DTS:X` | `dtsh`, `dtsl` | `A_DTS` | — |

`dtsh` covers both High Resolution (lossy) and Master Audio (lossless),
and `dtsc` covers core, ES and 96/24: with audio `profile` dropped,
nothing in a copy tells them apart. Open (section 12).

### 2.2 PCM: ffmpeg's tokens, many to one

Tokens are ffmpeg's, one per sample format: `pcm_s16le`, `pcm_s24le`,
`pcm_s32le`, `pcm_f32le`, `pcm_f64le`, `pcm_s16be`, `pcm_s24be`,
`pcm_s32be`, `pcm_f32be`, `pcm_f64be`, `pcm_u8`, `pcm_s8`, `pcm_alaw`,
`pcm_mulaw`, and the disc forms `pcm_bluray`, `pcm_dvd`, `pcm_dvda`,
`s302m`. Other sources map to them with the stream's bit depth:

| Matroska ID + BitDepth | Token |
|---|---|
| `A_PCM/INT/LIT` 8 | `pcm_u8` |
| `A_PCM/INT/LIT` 16, 24, 32 | `pcm_s16le`, `pcm_s24le`, `pcm_s32le` |
| `A_PCM/INT/BIG` 8 | `pcm_u8` |
| `A_PCM/INT/BIG` 16, 24, 32 | `pcm_s16be`, `pcm_s24be`, `pcm_s32be` |
| `A_PCM/FLOAT/IEEE` 32, 64 | `pcm_f32le`, `pcm_f64le` |
| `A_PCM/INT/*` 20 | no token: ffmpeg leaves it `pcm_s16le`, which is wrong. Open. |

| Where met | Token |
|---|---|
| Blu-ray LPCM (16, 20, 24 bits, big-endian), M2TS | `pcm_bluray` |
| DVD-Video LPCM, VOB | `pcm_dvd` |
| WAV (WAVE tags 0x0001, 0x0003) | `pcm_s16le`, `pcm_s24le`, `pcm_f32le`… |
| AIFF | `pcm_s16be`… |

MP4RA's PCM entries (ISO `ipcm`, `fpcm`, `twos`; QT `sowt`, `twos`,
`in24`, `in32`, `fl32`, `fl64`, `lpcm`, `alaw`, `ulaw`) map the same
way. IANA has only the RTP types `audio/L8`, `L16`, `L20`, `L24`.

## 3. Subtitle formats (`format` on a subtitle track)

The kind (text or picture) is the registry's fact; no field carries it.
It comes from ffmpeg's codec properties where ffmpeg sets one.

| Token | Kind | Format | ffmpeg | Matroska CodecID | MP4RA | IANA | Notes |
|---|---|---|---|---|---|---|---|
| `subrip` | text | SubRip (SRT) | `subrip` | `S_TEXT/UTF8` | — | — | aliases `srt`, `text` (section 4.4) |
| `ass` | text | Advanced SubStation Alpha | `ass` | `S_TEXT/ASS` (old `S_ASS`) | — | — | |
| `ssa` | text | SubStation Alpha | `ssa` | `S_TEXT/SSA` (old `S_SSA`) | — | — | ffmpeg reports these as `ass`; a receiver treats `ssa` as `ass` |
| `webvtt` | text | WebVTT | `webvtt` | `S_TEXT/WEBVTT`; WebM `D_WEBVTT/SUBTITLES`, `/CAPTIONS`, `/DESCRIPTIONS` | `wvtt` | `text/vtt` | ffmpeg maps neither `S_TEXT/WEBVTT` nor `wvtt` |
| `mov_text` | text | 3GPP timed text (MP4) | `mov_text` | — | `tx3g`; QT `text` | — | |
| `ttml` | text | TTML, IMSC | `ttml` | — | `stpp` | `application/ttml+xml` | |
| `hdmv_pgs_subtitle` | picture | Blu-ray PGS | `hdmv_pgs_subtitle` | `S_HDMV/PGS` | — | — | Jellyfin reports `PGSSUB` (section 4.4) |
| `dvd_subtitle` | picture | DVD VobSub | `dvd_subtitle` | `S_VOBSUB` | — | — | |
| `dvb_subtitle` | picture | DVB subtitles (EN 300 743) | `dvb_subtitle` | `S_DVBSUB` | — | `text/vnd.dvb.subtitle` (UNVERIFIED that it means EN 300 743) | |
| `dvb_teletext` | text (UNVERIFIED: ffmpeg sets no kind; character-coded, but may be rendered as pictures) | DVB teletext | `dvb_teletext` | — | — | — | recorded TV |
| `hdmv_text_subtitle` | text | Blu-ray Text ST | `hdmv_text_subtitle` | `S_HDMV/TEXTST` | — | — | |
| `arib_caption` | text (UNVERIFIED: ffmpeg sets no kind) | ARIB captions | `arib_caption` | `S_ARIBSUB` | — | — | |
| `eia_608` | text | CEA-608 | `eia_608` | — | QT `c608` | — | inside the video stream, not listed (proposal 17 section 3.2); as its own track (MP4 `c608`): open |
| `xsub` | picture | DivX XSUB | `xsub` | — | — | — | AVI |
| `microdvd`, `sami`, `subviewer`, `mpl2`, `realtext`, `jacosub`, `pjs`, `vplayer`, `stl` | text | files beside the video | same | — | — | — | |

Matroska's `S_TEXT/USF`, `S_KATE` and `S_IMAGE/BMP` have no ffmpeg name;
no token yet.

## 4. Containers: `mediaType`

### 4.1 Audio or video

`audio/<sub>` when the file has no video stream other than attached
pictures (cover art), else `video/<sub>`. Each registration says the
same: RFC 4337 (`audio/mp4`), RFC 3839 and 4393 (3GPP), RFC 9559 section
27.18 (`audio/matroska`), RFC 5334 (`audio/ogg`), the WebM guidelines
(`audio/webm`). Sent lowercase.

### 4.2 Registered by IANA

| File | `mediaType` | Audio only | Source |
|---|---|---|---|
| MP4 family (`.mp4`, `.m4v`) | `video/mp4` | `audio/mp4` (`.m4a`, `.m4b`) | RFC 4337, RFC 6381 |
| QuickTime (`.mov`) | `video/quicktime` | stays `video/quicktime` (nothing else registered) | IANA |
| Matroska (`.mkv`) | `video/matroska` (`video/matroska-3d` for stereoscopic) | `audio/matroska` (`.mka`) | RFC 9559 section 27.18 |
| MPEG-TS, M2TS (`.ts`, `.m2ts`) | `video/mp2t` | `audio/mp2t` (not in IANA; in the W3C MSE byte-stream registry) | RFC 3555 |
| MPEG-PS (`.mpg`, `.vob`) | `video/mpeg` | — (`audio/mpeg` is MP3 or MP2 alone, not PS) | RFC 2045, 2046 (UNVERIFIED that `.vob` is commonly served so) |
| Ogg (`.ogv`, `.ogg`, `.oga`, `.opus`) | `video/ogg` | `audio/ogg` | RFC 5334, RFC 7845 |
| 3GPP (`.3gp`) | `video/3gpp` | `audio/3gpp` | RFC 3839 |
| 3GPP2 (`.3g2`) | `video/3gpp2` | `audio/3gpp2` | RFC 4393 |
| Motion JPEG 2000 (`.mj2`) | `video/mj2` | — | RFC 3745 |
| MP3 (`.mp3`) | — | `audio/mpeg` | RFC 3003 |
| AAC ADTS (`.aac`) | — | `audio/aac` | IANA |
| AC-3, E-AC-3 alone | — | `audio/ac3`, `audio/eac3` | RFC 4184, RFC 4598 |
| DTS alone (`.dts`, `.dtshd`) | — | `audio/vnd.dts`, `audio/vnd.dts.hd` | IANA |
| TrueHD, MLP alone | — | `audio/vnd.dolby.mlp` | IANA |
| FLAC (`.flac`) | — | `audio/flac` | RFC 9639 |
| MPEG-H MHAS | — | `audio/mhas` | IANA |

`audio/opus` is Opus's RTP type, not a file's: `.opus` files are
`audio/ogg`. `application/mxf` (RFC 4539) is kept as any media type is.

### 4.3 Not in IANA: named here

| File | `mediaType` | Aliases a receiver may accept | From | Status |
|---|---|---|---|---|
| WebM (`.webm`) | `video/webm`, `audio/webm` | — | WebM container guidelines, W3C MSE registry | settled |
| AVI (`.avi`) | `video/avi` | `video/x-msvideo` (ffmpeg's muxer), `video/vnd.avi` (RFC 2361, never registered) | WHATWG MIME Sniffing | settled 2026-11-22 |
| WAV (`.wav`) | `audio/wave` | `audio/wav`, `audio/x-wav` (ffmpeg), `audio/vnd.wave` (RFC 2361, never registered) | WHATWG MIME Sniffing | settled 2026-11-22 |
| AIFF (`.aif`, `.aiff`) | `audio/aiff` | `audio/x-aiff` (UNVERIFIED) | WHATWG MIME Sniffing, ffmpeg's muxer | proposed |
| FLV (`.flv`) | `video/x-flv` | — | ffmpeg's muxer | proposed, UNVERIFIED elsewhere |
| Matroska, old spellings | `video/matroska`, `audio/matroska` | `video/x-matroska`, `audio/x-matroska` (deprecated in RFC 9559) | RFC 9559 | settled |
| ASF, WMV, WMA (`.asf`, `.wmv`, `.wma`) | open: IANA registers only `application/vnd.ms-asf`, which breaks the audio-or-video rule; ffmpeg declares `video/x-ms-asf`; `video/x-ms-wmv`, `audio/x-ms-wma` are common (UNVERIFIED) | | | open |
| RealMedia (`.rm`, `.rmvb`) | open (`application/vnd.rn-realmedia` is common, UNVERIFIED) | | | open, low priority |
| WavPack, Monkey's Audio, TTA, DSF, DFF, raw AC-4 | none found: MSFP-coined if needed (`audio/x-wavpack`, `audio/x-ape`, `audio/x-dsf`: UNVERIFIED) | | | open |

A producer sends the `mediaType` column; the aliases are for receivers.

### 4.4 Producer-side aliases for codec and format tokens

Names products report for a registered token. A producer that meets them
sends the token; a receiver may accept them as the token.

| Reported | Token | By |
|---|---|---|
| `dca` | `dts` | Plex (ffmpeg's old decoder name) |
| `PGSSUB` | `hdmv_pgs_subtitle` | Jellyfin, for `.sup` files |
| `srt`, `text` | `subrip` | ffmpeg (older ID; raw UTF-8 from `S_TEXT/ASCII`) |
| `ssa` reported as `ass` | `ass` | ffmpeg's Matroska and `.ssa` demuxers |

### 4.5 ISO BMFF brand to `mediaType` (producers only)

No brand is sent. A producer reads the major brand (`ftyp`'s first four
bytes) to choose `mediaType`. A major brand not in the table: the
compatible brands, in order. An unknown brand: `video/mp4`. The
audio-only rule (section 4.1) then turns `video/*` into `audio/*`,
except for `qt  `. MP4RA writes a space as `$20`.

| Brands | `mediaType` | Source |
|---|---|---|
| `qt  ` | `video/quicktime` (audio only too) | MP4RA "QuickTime" |
| `isom`, `iso2` to `iso9`, `isoa` to `isod` | `video/mp4`, `audio/mp4` | MP4RA "ISO" |
| `mp41`, `mp42` | `video/mp4`, `audio/mp4` | MP4RA "MP4v2" |
| `avc1`, `av01` | `video/mp4` | MP4RA |
| `dash`, `cmfc`, `cmf2`, `cmff`, `cmfl`, `cmfs`, `msdh`, `msix` | `video/mp4`, `audio/mp4` | MP4RA (DASH, CMAF) |
| `M4V ` | `video/mp4` | MP4RA "iTunes" |
| `M4A `, `M4B `, `M4P ` | `audio/mp4` | MP4RA "iTunes" |
| `MSNV` | `video/mp4` | MP4RA (IEC 62592) |
| `XAVC`, `niko`, `caqv`, `ROSS` and other camera brands | `video/mp4` | MP4RA; the unknown-brand default |
| `dby1` | `video/mp4`, `audio/mp4` | MP4RA |
| `iamf` | `audio/mp4` (UNVERIFIED that the IAMF spec says so) | MP4RA |
| `3gp4` to `3gp9`, `3gg6`, `3gg9`, `3ge6`, `3ge7`, `3ge9`, `3gr6`, `3gr9`, `3gs6`, `3gs9`, `3gh9`, `3gm9`, `3gmA`, `3gt8`, `3gt9`, `3gf9`, `3gtv`, `3gvr` | `video/3gpp`, `audio/3gpp` | MP4RA "3GPP"; RFC 3839 |
| `3g2a` (ffmpeg also writes `3g2b`, not in MP4RA) | `video/3gpp2`, `audio/3gpp2` | MP4RA "3GPP2"; RFC 4393 |
| `mjp2`, `mj2s` | `video/mj2` | MP4RA "MJ2"; RFC 3745 |
| `f4v `, `isml` (ffmpeg's muxers; not in MP4RA) | `video/mp4` (the unknown-brand default) | ffmpeg |
| `heic`, `mif1`, `avif`, `avis` | not media copies (HEIF images) | |

## 5. `objects` (audio track)

MSFP-coined: no outside vocabulary names these, and `dtsx` is DTS-UHD's
sample entry.

| Token | What | Where a parser finds it |
|---|---|---|
| `dolby-atmos` | Dolby Atmos | **E-AC-3 (JOC)**: `flag_ec3_extension_type_a` in `addbsi`; in MP4 the same flag in `dec3`; in Matroska `A_EAC3`'s CodecPrivate may carry that box. **TrueHD**: 4 substreams and the MSB of `substream_info` set in the major sync. **AC-4**: UNVERIFIED. |
| `dts-x` | DTS:X | **DTS-HD MA**: an XLL payload with sync word `0x02000850` (DTS:X) or `0xF14000D0` (IMAX Enhanced). **DTS-UHD**: whether `dts-x` applies to every stream is open (UNVERIFIED). |

Not tokens: MPEG-H 3D Audio (objects are native; a token would repeat
`codec`), Auro-3D (channel-based), IAMF (channel- and scene-based,
UNVERIFIED), IMAX Enhanced (`dts-x` covers it).

## 6. `hdr` (video stream)

| Token | What | Origin, equivalents |
|---|---|---|
| `hdr10plus` | HDR10+ (SMPTE ST 2094-40) | name from Jellyfin's `VideoRangeType.HDR10Plus`, lowercased; ITU-T T.35 in HEVC and AV1 SEI/OBU; Matroska `BlockAdditionMapping` (ITU T.35, BlockAddIDValue 4) |

HDR10 and HLG come from `colour.transfer` (16, 18); Dolby Vision from
the `dv` object. Candidates, not registered: `hdrvivid` (CUVA 005.1),
`smpte2094-50`; SL-HDR (ETSI TS 103 433) UNVERIFIED.

## 7. `edition` (copy)

`theatrical`, `directors`, `extended`, `unrated`, `remastered`,
`special`, `imax`. **Origin: MSFP's own** (proposal 16). No outside list
exists; the same words are those Radarr recognises in file names.
Candidates, not registered: `uncut`, `ultimate`, `collectors`,
`anniversary`, `open-matte`, `restored`, `fan-edit`.

## 8. `extraType` (extra)

`trailer`, `featurette`, `behindthescenes`, `deletedscene`, `interview`,
`scene`, `short`, `clip`, `themesong`, `themevideo`. **Origin:
Jellyfin's `ExtraType`**, lowercased. Jellyfin's `sample` is left out
(open); `unknown` is never sent.

## 9. Segment `kind` and audio `role`

- Segment `kind`: `intro`, `outro`, `recap`, `preview`, `commercial`.
  **Origin: Jellyfin's `MediaSegmentType`**, lowercased (`unknown` not
  sent).
- Audio `role`: **the DASH role scheme** (`urn:mpeg:dash:role:2011`,
  ISO/IEC 23009-1); the registry points to it rather than copying it.
  The values that concern audio: `main`, `alternate`, `supplementary`,
  `commentary`, `dub`, `description`, `enhanced-audio-intelligibility`,
  `emergency`, `karaoke` (the last two UNVERIFIED as audio roles: the
  standard is paywalled; the list is from Shaka Packager and ExoPlayer).

## 10. `threeD` (copy)

| Token | What | Equivalents |
|---|---|---|
| `sbs` | side by side | Jellyfin `HalfSideBySide`, `FullSideBySide`; Matroska `StereoMode` 1, 11 (UNVERIFIED) |
| `tab` | top and bottom | Jellyfin `HalfTopAndBottom`, `FullTopAndBottom`; Matroska `StereoMode` 2, 3 (UNVERIFIED) |
| `mvc` | H.264 MVC (Blu-ray 3D) | Jellyfin `MVC` |

**Origin: MSFP's own**, close to Jellyfin's `Video3DFormat`. Half or
full resolution follows from the picture's size.

## 11. Music types (album)

The values are MusicBrainz's release-group types
(musicbrainz.org/doc/Release_Group/Type), lowercased. These two fields
take their own class, 1 to 32 characters of `a–z 0–9 space . _ - /`,
with no space at either end (music 3).

| Field | Values |
|---|---|
| `primaryType` | `album`, `single`, `ep`, `broadcast`, `other` |
| `secondaryTypes` | `compilation`, `soundtrack`, `spokenword`, `interview`, `audiobook`, `audio drama`, `live`, `remix`, `dj-mix`, `mixtape/street`, `demo`, `field recording` |

(`other` here is MusicBrainz's own type, not MSFP's "other".)

## 12. Open entries

Not decided; the spec pass leaves them out or marks them:

1. **DTS flavours**: with audio `profile` dropped, `dtsh` can't tell
   High Resolution from Master Audio, nor `dtsc` core from ES or 96/24
   (section 2.1).
2. **DTS-UHD**'s token spelling, and whether `dts-x` applies to it.
3. **20-bit Matroska PCM** (section 2.2).
4. **IAMF**: no token; a layer like Dolby Vision.
5. **ASF, WMV, WMA** and **RealMedia** media types; types for WavPack,
   Monkey's Audio, TTA, DSF, DFF.
6. **CEA-608 as its own track** (MP4 `c608`): listed or not.
7. **Jellyfin's `sample`** extra type.

## 13. Sources (fetched 2026-11-22)

- **ffmpeg**, master `329157f585`: libavcodec `codec_desc.c`,
  `codec_id.h`, `profiles.c`, `defs.h`, `mlpdec.c`, `ac3_parser.c`,
  `ac3dec.c`, `dca_xll.c`, `dca_core.c`, `dca_syncwords.h`; libavformat
  `matroska.c`, `matroskadec.c`, `isom.c`, `isom_tags.c`, `mov.c`,
  `movenc.c`, `mpegts.c`, `riff.c`, `assdec.c`, and the muxers'
  `mime_type`s; libavutil `frame.h`, `side_data.c`.
- **MP4RA** (`mp4ra/mp4ra.github.io` `d35e02ae5d`): `sample-entries.csv`,
  `sample-entries-qt.csv`, `oti.csv`, `brands.csv`.
- **Matroska** (`ietf-wg-cellar/matroska-specification` `c3479fb572`):
  the codec specs and block additional mappings; RFC 9559.
- **IANA** media types (video, audio, text, application); templates of
  `audio/vnd.dolby.mlp` and `audio/vnd.dts.hd`.
- **RFCs** 4337, 3839, 4393, 5334, 2361 (informational), 6381, 6838.
- **W3C and WHATWG**: MSE Byte Stream Format Registry; WebM container
  guidelines; MIME Sniffing (`audio/aiff`, `audio/wave`, `video/avi`).
- **DASH roles**: Shaka Packager `adaptation_set.cc`, ExoPlayer
  `DashManifestParser.java`. ISO/IEC 23009-1 not read (paywalled).
- **Jellyfin** master: `ExtraType.cs`, `MediaSegmentType.cs`,
  `VideoRangeType.cs`, `Video3DFormat.cs`; its HLS codecs-string helper
  for the fixed codes.
- **MusicBrainz**: Release Group types page. **Radarr**: `Parser.cs`
  (`EditionRegex`).
- **Not read**: ISO/IEC 14496-15 and 23009-1, ETSI TS 102 366 Annex F
  (only through ffmpeg's code), ETSI TS 102 114, the DTS-UHD, TrueHD and
  AC-4 specs, the IAMF spec. Cells resting on them are UNVERIFIED.
