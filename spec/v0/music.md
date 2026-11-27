# MSFP version 0: the music part, version 0

**Draft.**

The `music` part carries
[MSFP Media version 0's music](https://github.com/msfp-io/msfp-media/blob/main/spec/v0/music.md)
between paired servers: artists, albums and tracks, their keys, their
descriptions and copies, how a track plays, and `lyrics`. What music
titles are, and how they play, is there; this part adds only what MSFP
decides around it: when `music` is on, which `local` titles a peer
sees, how audio playbacks count against a server's limits, and how
`lyrics` reaches a peer. Its section numbers are those of MSFP Media's
music.

It needs the [core](core.md), `catalog`, `playback` and `media`; if any
of them is off between two servers, `music` is off too
([core section 10](core.md#10-versions-and-parts)). A server lists
`music` in `hello` only when it shares or plays music. It comes from
[proposal 11](../../proposals/0011-music.md), revised by
[proposal 16](../../proposals/0016-version-0-revision.md) and
[proposal 17](../../proposals/0017-neutrality.md); its language moved
into MSFP Media by [proposal 19](../../proposals/0019-msfp-media.md).

The words "must", "must not" and "should" carry their usual meaning in
specifications.

## Contents

1. [Terms](#1-terms)
2. [Title keys](#2-title-keys)
3. [What a title carries](#3-what-a-title-carries)
4. [Pictures](#4-pictures)
5. [Playing a track](#5-playing-a-track)
   - [5.1 Limits](#51-limits)
   - [5.2 The queue](#52-the-queue)
6. [Lyrics](#6-lyrics)
7. [Libraries](#7-libraries)
8. [Security considerations](#8-security-considerations)
9. [Open questions](#9-open-questions)

## 1. Terms

As in [MSFP Media's music, section 1](https://github.com/msfp-io/msfp-media/blob/main/spec/v0/music.md#1-terms).
Where MSFP Media says the **source**, MSFP means the owning server;
where it says the **caller**, the viewer's server.

## 2. Title keys

The `mb` and `local` schemes are those of
[MSFP Media's music, section 2](https://github.com/msfp-io/msfp-media/blob/main/spec/v0/music.md#2-title-keys).
MSFP adds one rule: **the owning server chooses** whether to share
`local` titles at all, off by default, per pairing and per group, as
[catalog section 5](catalog.md#5-what-is-shared) says.

## 3. What a title carries

As in [MSFP Media's music, section 3](https://github.com/msfp-io/msfp-media/blob/main/spec/v0/music.md#3-what-a-title-carries).

## 4. Pictures

As in [MSFP Media's music, section 4](https://github.com/msfp-io/msfp-media/blob/main/spec/v0/music.md#4-pictures).

## 5. Playing a track

A track plays as
[MSFP Media's music, section 5](https://github.com/msfp-io/msfp-media/blob/main/spec/v0/music.md#5-playing-a-track)
says, through [playback](playback.md), under its limits, with downloads
and scan access where the owner allows them. Between servers the ask is
always explicit: a viewer's server names the delivery it wants and never
sends a capability list ([playback](playback.md)).

A viewer's server that plays an album without a queue calls `play` for
each track as it comes, and `stop` for the one before. Skipping fast
then reaches the `play` limit of
[core section 8.3](core.md#83-limits-and-overload), and is refused with
`429`; a queue (section 5.2) avoids that.

### 5.1 Limits

An audio playback counts as a stream, like a video one. The owning
server may count audio separately; it says so with two members of
`limits` ([playback section 5](playback.md#5-limits)):

| Field | Type | Range | Content |
|-------|------|-------|---------|
| `audioStreams` | integer | 0 to 1,000 | audio playbacks at the same time |
| `audioTranscodes` | integer | 0 to `audioStreams` | of those, how many may be transcoded |

Absent: audio counts in `streams` and `transcodes`.

### 5.2 The queue

The queue is that of
[MSFP Media's music, section 5.2](https://github.com/msfp-io/msfp-media/blob/main/spec/v0/music.md#52-the-queue).
The playback, queue included, counts once against `streams` (and
`audioStreams`), and once against `transcodes` (and `audioTranscodes`)
when any of its tracks is converted.

## 6. Lyrics

`lyrics` is the call of
[MSFP Media's music, section 6](https://github.com/msfp-io/msfp-media/blob/main/spec/v0/music.md#6-lyrics).
Between servers it is made under the peer's base path, signed as every
call is ([core section 8](core.md#8-common-rules-for-every-call)). Beside
the refusals MSFP Media lists, the owning server answers `404`
`not-supported` when `music` is off, and `429` `rate-limited` when asked
too often ([core section 8.3](core.md#83-limits-and-overload)).

## 7. Libraries

As in [MSFP Media's music, section 7](https://github.com/msfp-io/msfp-media/blob/main/spec/v0/music.md#7-libraries),
within the libraries one side offers and the other wants
([catalog section 5](catalog.md#5-what-is-shared)).

## 8. Security considerations

Those of
[MSFP Media's music, section 8](https://github.com/msfp-io/msfp-media/blob/main/spec/v0/music.md#8-security-considerations),
and one of MSFP's: a queue lets one `play` prepare up to 6 tracks, so an
owning server bounds what it converts ahead under its transcode budget
([playback section 6](playback.md#6-what-the-owning-server-enforces)).

## 9. Open questions

The open questions on music are
[MSFP Media's](https://github.com/msfp-io/msfp-media/blob/main/spec/v0/music.md#9-open-questions),
but one, which goes with the limits of section 5.1:

- **Limits**: `audioTranscodes` ranges from 0 to `audioStreams`, as
  `transcodes` to `streams`; absent, audio counts in `transcodes` too,
  not only in `streams`.
