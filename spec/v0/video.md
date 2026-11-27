# MSFP version 0: the video part, version 0

**Draft.**

The `video` part carries
[MSFP Media version 0's video](https://github.com/msfp-io/msfp-media/blob/main/spec/v0/video.md)
between paired servers: films, series and their extras, their title
keys, their copies, how they play, and the subtitles, fonts and preview
sheets served for them. Everything MSFP Media's video says holds, under
the same section numbers; this part adds what only federation knows:
which parts it needs, who may see local titles, what counts against a
pairing's limits, when preview sheets may be fetched, and how
`previews` is called between servers.

It needs the [core](core.md), `catalog`, `playback` and `media`; if any
of them is off between two servers, `video` is off too
([core section 10](core.md#10-versions-and-parts)). A server lists
`video` in `hello` only when it shares or plays films and series.

In MSFP Media's words, the source is the owning server and the caller
is the viewer's server.

The words "must", "must not" and "should" carry their usual meaning in
specifications.

## Contents

1. [Terms](#1-terms)
2. [Libraries](#2-libraries)
3. [Titles](#3-titles)
4. [Title keys](#4-title-keys)
5. [Playing a film, an episode or an extra](#5-playing-a-film-an-episode-or-an-extra)
   5. [Seek-bar previews](#55-seek-bar-previews)
   6. [previews](#56-previews)
6. [Media](#6-media)
7. [Security considerations](#7-security-considerations)
8. [Open questions](#8-open-questions)

## 1. Terms

The terms are MSFP Media's
([video section 1](https://github.com/msfp-io/msfp-media/blob/main/spec/v0/video.md#1-terms)).
A **local title** is a title keyed under the `local` scheme
([MSFP Media's video, section 4.5](https://github.com/msfp-io/msfp-media/blob/main/spec/v0/video.md#45-the-local-scheme)).

## 2. Libraries

What a video library holds is as
[MSFP Media's video, section 2](https://github.com/msfp-io/msfp-media/blob/main/spec/v0/video.md#2-libraries)
says. Which libraries a peer sees is the catalog's
([catalog section 5](catalog.md#5-what-is-shared)).

## 3. Titles

A video title, its copies, streams, tracks, chapters, skip markers,
extras and the description of a local title are as
[MSFP Media's video, section 3](https://github.com/msfp-io/msfp-media/blob/main/spec/v0/video.md#3-titles)
says.

## 4. Title keys

Video's title keys are as
[MSFP Media's video, section 4](https://github.com/msfp-io/msfp-media/blob/main/spec/v0/video.md#4-title-keys)
says. Between servers, **the owner chooses** whether to share local
titles at all: off by default, it is allowed per pairing and per group.
An owning server sends local titles only to a peer it allows them; a
change of that choice is a change of what it offers
([catalog section 5](catalog.md#5-what-is-shared)).

## 5. Playing a film, an episode or an extra

Playing a film, an episode or an extra is as
[MSFP Media's video, section 5](https://github.com/msfp-io/msfp-media/blob/main/spec/v0/video.md#5-playing-a-film-an-episode-or-an-extra)
says. Between servers:

- An extra's playback counts against the pairing's `streams` as any
  playback does ([playback section 5](playback.md#5-limits)).
- A `previews` ticket (section 5.6) is no playback: it doesn't count
  against `streams`.

### 5.5 Seek-bar previews

The layout of seek-bar previews, the ticket they are fetched under and
their caching are as
[MSFP Media's video, section 5.5](https://github.com/msfp-io/msfp-media/blob/main/spec/v0/video.md#55-seek-bar-previews)
says. Between servers:

- **When sheets may be fetched.** A viewer's server never fetches
  previews for a whole library, nor in bulk for copies nobody opened.
  For a copy a person opens (its page, its chapter list, a playback, a
  scrub), how it fetches the sheets, all at once or one by one, is its
  choice. Showing chapter images
  ([MSFP Media's video, section 3.4](https://github.com/msfp-io/msfp-media/blob/main/spec/v0/video.md#34-chapters-and-skip-markers))
  counts as opening.
- A `429` `rate-limited` on `previews` (section 5.6) means no previews
  of that peer until `retryAfter`.

### 5.6 previews

`previews` is as
[MSFP Media's video, section 5.6](https://github.com/msfp-io/msfp-media/blob/main/spec/v0/video.md#56-previews)
says. Between servers, it is a signed `POST` under the peer's base path,
as every call is ([core section 8](core.md#8-common-rules-for-every-call)).
The owning server answers it with a signed `404` `not-supported` when
`video` isn't on between the two, and may refuse it with `429`
`rate-limited` when asked too often
([core section 8.3](core.md#83-limits-and-overload)).

## 6. Media

Video's URLs, what is fetched for it, WebVTT, ASS, picture subtitles,
fonts and the image of a local title are as
[MSFP Media's video, section 6](https://github.com/msfp-io/msfp-media/blob/main/spec/v0/video.md#6-media)
says. The stream root, its certificates and who may fetch under it are
the [media part](media.md)'s.

## 7. Security considerations

[MSFP Media's video, section 7](https://github.com/msfp-io/msfp-media/blob/main/spec/v0/video.md#7-security-considerations)
holds. Between servers, besides:

- **Local titles** are text and pixels a peer chose; they are off by
  default, shared per pairing and per group (section 4).
- **Previews tell use.** Sheets are fetched only for copies someone
  opens, never for a whole library (section 5.5).

## 8. Open questions

The open questions and settled choices of video are MSFP Media's
([video section 8](https://github.com/msfp-io/msfp-media/blob/main/spec/v0/video.md#8-open-questions)).
This part has none of its own.
