# MSFP registry of schemes

This file lists the schemes of title keys and of `ids`, for every
version of MSFP from [version 0](v0/README.md) on. It is normative for
the forms it gives. It was made by
[proposal 17](../proposals/0017-neutrality.md).

A scheme names the source of an identifier: a public database, or the
owning server itself. Title keys name a title under one scheme
([catalog section 4](v0/catalog.md#4-title-keys)); `ids` carry the same
title's identifiers under other schemes
([catalog section 4.1](v0/catalog.md#41-same-title-same-key)).

## Rules

- **A new scheme is an entry here**, not a new version of a part. It is
  added by a proposal ([proposals](../proposals/README.md)) that gives
  its name, its source, the kinds it names, and its forms.
- **A scheme's name** is 1 to 32 characters of `a–z 0–9`. A name is
  never reused for another source.
- **A receiver skips** the title keys of a scheme it doesn't know, and
  ignores the `ids` of a scheme it doesn't know, as
  [catalog section 4](v0/catalog.md#4-title-keys) says. A sender may
  send a scheme a receiver doesn't know yet: nothing breaks.
- **The key forms** of a scheme are defined by the content part whose
  titles it keys, in the section the table gives. This file says only
  which part, and which kinds.

## Key schemes

| Scheme | Source | Kinds it keys | Key forms defined in |
|--------|--------|---------------|----------------------|
| `tmdb` | The Movie Database | `movie`, `series`, `season`, `episode`; `extra` under them | [video section 4](v0/video.md#4-title-keys) |
| `tvdb` | TheTVDB | `series`, `season`, `episode` (absolute numbers included); `extra` under them | [video section 4.4](v0/video.md#44-the-tvdb-scheme) |
| `mb` | MusicBrainz | `artist`, `album` (a release), `track` (a release track) | [music section 2.1](v0/music.md#21-the-mb-scheme) |
| `local` | the owning server, for titles no public source knows | every kind of `video` and `music` | [video section 4.5](v0/video.md#45-the-local-scheme), [music section 2.2](v0/music.md#22-the-local-scheme); the opaque part: [catalog section 4.2](v0/catalog.md#42-opaque-parts-of-a-key) |

The order in which a server should pick a scheme for a title is its
content part's: [catalog section 4](v0/catalog.md#4-title-keys) gives
it.

## `ids` schemes

Each value is a JSON string. A value that breaks its form is ignored, as
is a scheme sent on a kind the table doesn't give it. `ids` never
replaces the key. A `local` title of `video` carries no `ids`; one of
`music` may ([music section 3.1](v0/music.md#31-other-identifiers)).

| Scheme | Kinds | Form | Example | What it names |
|--------|-------|------|---------|---------------|
| `imdb` | `movie`, `series`, `episode` | `tt` and 7 to 10 digits | `tt0133093` | the IMDb title: the film, the series, or the episode |
| `tvdb` | `series`, `episode` | 1 to 10 digits, no leading zero | `81189` | TheTVDB's series ID; for an episode, TheTVDB's episode ID |
| `tmdb` | `series`, `episode` keyed under another scheme than `tmdb` | 1 to 10 digits, no leading zero (1 to 9,999,999,999) | `1399` | TMDB's series ID; for an episode, TMDB's episode ID |
| `mbreleasegroup` | `album` | an MBID: `[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}` | `5b11f4ce-a62d-471e-81fc-a69a8278c7da` | the release group the release belongs to |
| `mbrecording` | `track` | an MBID | `b1a9c0e9-d987-4042-ae91-78d6a3267d69` | the recording the release track holds |
| `isrc` | `track` | 12 characters, `[A-Z]{2}[A-Z0-9]{3}[0-9]{7}` | `USRC17607839` | the recording's ISRC |
| `acoustid` | `track` | a lowercase UUID, in the form of an MBID | `9ff43b6a-4f16-427c-93c2-92307ca505e0` | the AcoustID track |

- **`tmdb` in `ids`** lets a title keyed under another scheme (a series
  under `tvdb`, say) name its TMDB ID once TMDB has it. A title keyed
  under `tmdb` sends no `ids.tmdb`: its key says it.
- **Matching.** Two titles of the same kind with the same value under
  the same `ids` scheme, or one whose key's scheme and identifier equal
  the other's `ids` value under that scheme, are the same title shown
  twice: a viewer's server may show them as one. They are still two
  titles, each played from its own key
  ([catalog section 4.1](v0/catalog.md#41-same-title-same-key)).
