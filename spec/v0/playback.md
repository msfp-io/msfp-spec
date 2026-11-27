# MSFP version 0: the playback part, version 0

**Draft.**

The `playback` part carries
[MSFP Media version 0's playing](https://github.com/msfp-io/msfp-media/blob/main/spec/v0/playing.md)
between paired servers: `play`, `progress` and `stop`, the deliveries,
a playback's states and endings, and downloads are as playing says. A
viewer's server starts, keeps and stops a playback of a title a peer
holds: to watch it, to download it, or to let a media server read the
start of its file to list it. This part adds what is the federation's:
the limits a server sets per pairing and per group (section 5), what
the owning server enforces (section 6), the limits on downloads
(section 7), scan access (section 8), and what a peer never learns.

It needs the [core](core.md), [catalog](catalog.md) (it plays title
keys from `titles`) and [media](media.md) (the viewer's server builds
media URLs from its answers' tickets). If either is off between two
servers, `playback` is off too ([core section 10.3](core.md#103-choosing)).
The content parts in use are those on between the two servers
([core section 10](core.md#10-versions-and-parts)). Its calls follow
[core section 8](core.md#8-common-rules-for-every-call): signed `POST`s
under the peer's base path, answered only to peers in state `paired`. A
server that doesn't speak `playback` with the caller answers its calls
with a signed `404` `not-supported`. A peer that can't be called
([core section 7.5](core.md#75-a-peer-that-cant-be-called)) is only ever
the viewer's server: nobody calls `play`, `progress` or `stop` on it, and
nothing of it is offered for playing.

The words "must", "must not" and "should" carry their usual meaning in
specifications.

## Contents

1. [Terms](#1-terms)
2. [play](#2-play)
3. [progress](#3-progress)
4. [stop](#4-stop)
5. [Limits](#5-limits)
6. [What the owning server enforces](#6-what-the-owning-server-enforces)
   1. [Limits at once](#62-limits-at-once)
   2. [How a playback ends](#63-how-a-playback-ends)
7. [Downloads](#7-downloads)
8. [Scan access](#8-scan-access)
9. [Security considerations](#9-security-considerations)
10. [Open questions](#10-open-questions)

## 1. Terms

Copy, playback, ticket, delivery, purpose and viewer session are as in
[playing section 1](https://github.com/msfp-io/msfp-media/blob/main/spec/v0/playing.md#1-terms).
Between servers:

- **Owning server**: the server that holds a title's copy and streams
  it: playing's source.
- **Viewer's server**: the server whose signed-in person watches a title
  the owning server holds: playing's caller. The person's browser only
  ever talks to the viewer's server.
- **Scan**: a playback of purpose `scan`, which MSFP adds to playing's
  purposes (section 8).

## 2. play

`play`, under the peer's base path, is
[playing section 2](https://github.com/msfp-io/msfp-media/blob/main/spec/v0/playing.md#2-play).
MSFP adds:

- **The purpose `scan`** (section 8), with its row of the table of
  purpose and delivery: `scan` takes `delivery` `file` only. A `purpose`
  not of `play`, `download` and `scan` is refused (`400`).
- **No capability list.** The ask is always explicit: `delivery`, or
  none for the baseline. A `play` with `capabilities` is refused (`400`),
  so that no device is ever described to a peer
  ([playing section 2.8](https://github.com/msfp-io/msfp-media/blob/main/spec/v0/playing.md#28-the-capability-list)).
- **The carrier's limits** of playing section 2.2 are this part's
  (section 5). At step 2, a limit of 0 for the purpose, `streams` for
  `play`, `downloads` for `download`, `scansPerDay` for `scan`, is
  refused with `409` `stream-limit-reached`. At step 5, so are the
  purpose's limits used: for `play`, the pairing's streams all in use;
  for `download`, its downloads all in use, or `downloadsPerDay`
  reached; for `scan`, `scansPerDay` reached. At step 6, the carrier's
  cap is the pairing's `bitrateKbps`; a download's speed is capped by
  `downloadKbps` (section 7), and a scan's reads by section 8. At step 3,
  the libraries the caller may see are those offered to that peer
  ([catalog section 5](catalog.md#5-what-is-shared)). At step 8, the
  limits are section 5's, counted as section 6 says.
- **`scanBytes` in the answer**, scans only: the bytes the scan's ticket
  may read in all (section 8). With `purpose` `scan`, 4,194,304 to
  1,073,741,824; missing, not an integer or out of range, the viewer's
  server counts on 4,194,304 (4 MiB); with another purpose, it is
  ignored.
- **Refusals** ([playing section 2.5](https://github.com/msfp-io/msfp-media/blob/main/spec/v0/playing.md#25-refusals)),
  besides the language's: `409` `stream-limit-reached`, the carrier's
  refusal for its limits, which the viewer's server tells the viewer as
  busy now; `429` `rate-limited`
  ([core section 8.3](core.md#83-limits-and-overload)); and `404`
  `not-supported` when `playback` is off. A refusal MSFP vouches for is
  a signed one: the viewer's server reads the `type` of a signed `409`
  as playing says; an unsigned refusal, like a signed `429` or `503` and
  no answer at all, means busy now, as
  [core section 8.3](core.md#83-limits-and-overload) says.

## 3. progress

`progress`, under the peer's base path, is
[playing section 3](https://github.com/msfp-io/msfp-media/blob/main/spec/v0/playing.md#3-progress).
A `progress` of a playback of another pairing is refused as playing
says of another caller's (`410` `playback-stopped`). A `progress` of a
scan (section 8) is answered `200` and changes nothing.

## 4. stop

`stop`, under the peer's base path, is
[playing section 4](https://github.com/msfp-io/msfp-media/blob/main/spec/v0/playing.md#4-stop).
A playback ID of another pairing changes nothing.

## 5. Limits

The owning server sends, in its `libraries` answer
([catalog section 2](catalog.md#2-libraries)), what the caller may use of
it. It sends `limits` only when `playback` is on between the two.

| Field | Type | Range | Content |
|-------|------|-------|---------|
| `streams` | integer | 0 to 1,000 | playbacks of purpose `play` at the same time |
| `transcodes` | integer | 0 to `streams` | of those, how many may be transcoded |
| `bitrateKbps` | integer | 0 to 1,000,000 | a bitrate cap in kbit/s; 0 means none |
| `downloads` | integer | 0 to 1,000 | downloads at the same time (section 7); 0: no downloads |
| `downloadsPerDay` | integer | 0 to 10,000 | downloads started in any 24 hours; 0 means no daily cap besides `downloads` |
| `downloadKbps` | integer | 0 to 1,000,000 | the speed cap of each download, kbit/s (section 7); 0 means none |
| `scansPerDay` | integer | 0 to 1,000,000 | scans started in any 24 hours (section 8); 0: no scan access |

Absent, `downloads`, `downloadsPerDay`, `downloadKbps` and `scansPerDay`
are 0: no downloads, no scan access. The bytes one scan may read are not
a limit here: each scan's answer states them (`scanBytes`, section 2). A
content part may add members:
[music section 5.1](music.md#51-limits) adds `audioStreams` and
`audioTranscodes`, in the same bounds as `streams` and `transcodes`.

The caller uses the limits only to avoid offering its people what the
owning server would refuse: it shows Download (section 7) only for a
peer whose `downloads` is above 0, asks for scans (section 8) only
of a peer whose `scansPerDay` is above 0, and lets its media server read
no more than the scan answer's `scanBytes` in one scan. The owning server enforces them
itself (section 6). A caller ignores limits out of range. A server that doesn't
enforce limits sends none.

What the caller keeps of `limits`:

- a member that isn't an integer makes the caller ignore the whole
  `limits` object, as
  [listing section 2.2](https://github.com/msfp-io/msfp-media/blob/main/spec/v0/listing.md#22-what-the-caller-keeps)
  says of members of the wrong type;
- a member out of its range is ignored alone, the others kept;
- `transcodes` is bounded by `streams` only when `streams` is kept:
  with `streams` ignored or absent, `transcodes` is checked against 0 to
  1,000 alone. `audioTranscodes` and `audioStreams` likewise.

Test cases: `limits` in
[vectors/v0/playback.json](../../vectors/v0/playback.json).

**Shared limits.** Limits may be shared with other peers: a group's pool
is shared by all its members ([groups section 9](groups.md#9-sharing-and-limits)),
downloads and scans as streams. A peer linked through a group is sent the most generous of its relations'
limits, and may be refused (`409` `stream-limit-reached`) before reaching
them, its pool being in use by others. A playback counts against exactly
one pool: the first with room, the individual relation's own limits
first. A day's counts (`downloadsPerDay`, `scansPerDay`) are kept in the
pool the playback counted against.

## 6. What the owning server enforces

The owning server decides alone. The viewer's server can't widen
anything.

- **What is played.** `play` finds copies only in libraries offered to
  that peer ([catalog section 5](catalog.md#5-what-is-shared)).
- **Stream caps.** Per pairing: at most `streams` playbacks of purpose
  `play` at once (0 refuses every such `play`), and at most `transcodes`
  of them transcoded. Downloads count against `downloads` and
  `downloadsPerDay`, never `streams` (section 7); scans against
  `scansPerDay` only (section 8). A playback and the one that
  `replaces` it count as one, against every cap here and in its pool
  (section 5): when the new `play` is checked, the old one is counted
  as ended
  ([playing section 2.3](https://github.com/msfp-io/msfp-media/blob/main/spec/v0/playing.md#23-the-answer)).
  Across the server: its transcode
  budget
  ([playing section 2.6](https://github.com/msfp-io/msfp-media/blob/main/spec/v0/playing.md#26-choosing-the-copy-and-the-tracks)),
  less the transcodes it keeps for its own people; a download's
  transcode takes one from that budget, but none of the pairing's
  `transcodes`. A server whose media server has no such budget makes
  one, an operator setting counting its own people's transcodes and its
  peers' together. The bitrate is the lowest cap of playing section 2.2,
  step 6, with the pairing's `bitrateKbps` among them.
- **Off by default.** Downloads and scan access are off until the
  owning server's operator allows them, per pairing and per group:
  `downloads` 0 and `scansPerDay` 0 (section 5). So are `local` titles,
  not shared until the operator shares them
  ([catalog section 5](catalog.md#5-what-is-shared)). The other limits
  are the operator's choice.
- **Media.** Only through the ticket, only from a client that doesn't
  look like a browser, only while the playback is live
  ([media section 4](media.md#4-how-long-a-ticket-works)). A download's
  reads at `downloadKbps` at most (section 7); a scan's reads within its
  cap (section 8).
- **What a peer never learns:**
  - nothing about the viewer: no account, profile, name, email, address,
    IP, browser or device. Nothing in `play`, `progress` or `stop` names a
    person.
  - not the viewer's device: no capability list (section 2), only the
    delivery asked, and transcode targets in steps.
  - of the owning server: never files, paths, or its media server's item
    IDs, not even inside version IDs.

  What a peer does learn: the title, the purpose, the copy and audio
  track asked, the delivery, the start position, the transcode targets
  and the bitrate cap, and progress over time (with `at`, the times of
  offline viewing too), for one unnamed viewer session of the other
  server.

A playback's states, and the endings that aren't the federation's, are
[playing section 5](https://github.com/msfp-io/msfp-media/blob/main/spec/v0/playing.md#5-a-playbacks-states-and-endings).

### 6.2 Limits at once

The owning server must enforce the stream, transcode, download and scan
caps atomically: checking that a stream (and, for a transcode, a
transcode; for a download, a download and the day's count; for a scan,
the day's count) is free and counting the new playback is one step,
which no other `play` can come between. So two `play`s that arrive
together can never both take the last stream. A `play` that loses this race is refused as if it
had come later (`409` or `503`, playing section 2.6). A server may check the
caps earlier too, to refuse before doing any work, but the check that
counts is the atomic one. With shared limits (section 5), choosing the
pool and counting the playback in it is that same one step.

### 6.3 How a playback ends

Besides the endings of
[playing section 5](https://github.com/msfp-io/msfp-media/blob/main/spec/v0/playing.md#5-a-playbacks-states-and-endings),
a live playback ends, and its ticket stops working at once, when:

- the pairing ends, the caller is no longer `paired`, or its key is
  suspended ([core section 12.6](core.md#126-revocation)): MSFP's
  withdrawal of the caller's access;
- the library holding the copy is no longer offered to that peer
  ([catalog section 5](catalog.md#5-what-is-shared));
- it is a scan, 10 minutes after its `play` was answered (section 8).

A download or a scan stays counted in the day's count it took. Expiry
cleans up an abandoned scan as it does a download: never much longer
than its lifetime.

## 7. Downloads

Downloads are
[playing section 6](https://github.com/msfp-io/msfp-media/blob/main/spec/v0/playing.md#6-downloads).
Between servers they are the owning server's choice, off by default
(`downloads` 0, section 5).

- **Limits.** A download counts against `downloads` (at once) and
  `downloadsPerDay` (started in any 24 hours), never against `streams`
  or `transcodes`; a transcode takes one from the server's budget
  (section 6). Over `downloads` or `downloadsPerDay`: `409`
  `stream-limit-reached`.
- **The speed cap** is `downloadKbps`, paced as
  [fetching section 4.3](https://github.com/msfp-io/msfp-media/blob/main/spec/v0/fetching.md#43-downloads)
  says.
- **Download shown only where allowed.** A viewer's server shows Download
  only for titles of a peer whose `limits` allow downloads (`downloads`
  above 0).

Test cases: `downloads` in
[vectors/v0/playback.json](../../vectors/v0/playback.json).

## 8. Scan access

Some media servers, without plugins, can only show real files: they list
a file by reading part of it. Scan access lets a viewer's server serve
them those reads, outside any playback of a viewer. It is off by default
(`scansPerDay` 0, section 5); the owning server's operator allows it per
pairing and per group, with a number of scans a day.

- **The request.** A `play` with `purpose` `scan` and `delivery` `file`;
  any other delivery of the list is refused (`400`). One scan is of one
  copy. A copy of several parts is never delivered as `file`, so it is
  never scanned.
- **No stream used.** A scan counts against `scansPerDay` only (started
  in any 24 hours), never against `streams`, `transcodes` or `downloads`,
  and never converts anything. Over it: `409` `stream-limit-reached`.
  Scan `play`s count against the `play` limit of
  [core section 8.3](core.md#83-limits-and-overload) like any `play`.
- **The cap.** A scan's answer states `scanBytes` (section 2): the
  bytes its ticket may read in all (one scan, one copy), 4,194,304
  (4 MiB) to 1,073,741,824 (1 GiB). It is required with `purpose`
  `scan`; a caller that finds it missing or out of range counts on
  4 MiB. Only the owning server knows which pool counted the scan, so
  the cap is that pool's ([groups section 9](groups.md#9-sharing-and-limits)).
  The owning server serves the scan's reads up to the answer's
  `scanBytes`, counting every byte it sends under that ticket, a byte
  read twice counted twice; `scansPerDay` bounds the total. A read that
  would go past the cap is cut at the cap (a `206` ending there); any
  later read is refused (`416`), as
  [media section 4.2](media.md#42-scans) says.
- **Rate-limited.** The owning server serves a scan's reads at most 2 at
  once (more: `503`), and may pace them as it paces downloads.
- **The ticket** lasts 10 minutes from the answer, and the idle limit
  holds ([playing section 5](https://github.com/msfp-io/msfp-media/blob/main/spec/v0/playing.md#5-a-playbacks-states-and-endings)),
  counted on media requests alone. The viewer's server sends `stop` when
  its media server is done reading. It sends no `progress` for a scan.
- **What reads are for.** A viewer's server uses `scan` only for reads
  to list a file, and a `play` with `purpose` `play` for reads to watch
  it, like any other playback; how it tells them apart is its own. No
  whole-file analysis is made of a peer's copies. Scan access is the one
  case where a viewer's server lets its media server read a peer's file
  to learn what it is; everywhere else, media info comes from the
  catalog only.

Test cases: `scans` in
[vectors/v0/playback.json](../../vectors/v0/playback.json).

## 9. Security considerations

Playing's
[security considerations](https://github.com/msfp-io/msfp-media/blob/main/spec/v0/playing.md#7-security-considerations)
hold. Between servers:

- **The ticket** is carried by `play`'s answer. That is why `play` needs
  a *trusted* or *named* certificate
  ([core section 3.2](core.md#32-https-certificates)). A viewer's server
  never passes a download's ticket to a device.
- **The viewer's privacy.** A viewer's server never passes a peer's URLs
  to a browser, never sends anything that names a person, and says
  nothing of devices but the delivery it asks: never a capability list
  (section 2).
- **Downloads leave the owning server**, which can't take back what was
  read: that is why downloads are off by default.
- **Scans are bounded,** by size (the scan answer's `scanBytes`, at most
  1 GiB), time and count (section 8), so that scan access can't be used to copy a file.

## 10. Open questions

The section numbers in the list below are those this part had before
proposal 19 moved the language to MSFP Media: sections 2, 3 and 4 are
now [playing](https://github.com/msfp-io/msfp-media/blob/main/spec/v0/playing.md)'s
sections 2, 3 and 4, sections 6.1 and 6.3 are playing section 5
(MSFP's own endings stay in section 6.3), and section 7 is playing
section 6 (the limits on downloads stay in section 7).

No open point of the proposals of 2026-11-16 concerns this part alone.
The core's open questions ([core section 18](core.md#18-open-questions))
apply. Choices this draft made where the proposals were silent, all
confirmed by the owner on 2026-11-17 (they may still change as
implementations show):

1. **`versionId` in the request** (section 2.1). Proposal 2 says a
   viewer's server passes version IDs back in a later `play` of the same
   title, but the test draft's request has no member for it. This draft adds
   the optional `versionId`.

The second choice of that list, `startPositionMs` with several episodes
in one copy, moved to [video](video.md#8-open-questions).

Choices of the review round of 2026-11-16, confirmed on 2026-11-17:

2. **An `audioTrack` the copy doesn't have** is read as absent
   (section 2.6), not refused with `400`. (`subtitleTrack` left the
   request with proposal 17.)
3. ~~**No member to forbid transcoding**~~ Superseded by proposal 16:
   `delivery` `file` or `remux` forbids a transcode (section 2.6).
4. **The three states** of section 6.1, where started and active differ
   only in use. A server that needs no difference may keep one live state.
5. **Ending at once** when a pairing ends or a library is no longer
   offered (section 6.3): required as "the next media request fails";
   whether the transcode must also stop at the moment of the change is
   left to implementations.

**Choices this draft made, confirmed by the owner on 2026-11-17** (when the video rules were moved
out of this part on 2026-11-17):

6. ~~**The track object stays here**~~ Superseded by proposal 17: the
   answer carries no tracks; the catalog describes them.
7. **Section 2.4 keeps its number** for what the content parts add, so
   that sections 2.5 and 2.6 keep theirs.
8. **Members of another content part** in an answer are ignored
   (section 2.4); music already says so for a track's subtitles, skip
   markers and previews.
9. ~~**`subtitleTrack` is still named here**~~ Superseded by proposal 17:
   the viewer's server chooses; the choice tables left the protocol.

**Decided by the owner on 2026-11-18** (platform neutrality,
[core open questions 43 and 44](core.md#18-open-questions)):

10. **A copy may be several files** (section 1), played as one; the
    default audio is the file's, never a person's saved choice
    (section 2.6).

**Proposal 16** (adopted 2026-11-22): delivery, restarts, one playback
per viewer session, capabilities and transcode targets, downloads and
scan access. Choices this draft made where it was silent, to be
confirmed:

11. **`etag` in the answer** (section 2.3), required, so that a restart
    and "the answer wins" can compare it with the catalog's.
12. **`mediaType`** (`container` before proposal 17) names the remux's
    segment format, and **`audioRenditions`** asks for every audio track
    in HLS (section 2.1).
13. ~~**Steps of the new capabilities**~~ Superseded by proposal 17: no
    capabilities.
14. **A download's size** is named by `transcode.maxHeight` or
    `maxBitrateKbps` (section 7), not by a size in bytes.
15. **`at`** (section 3): Unix seconds, from 30 days before to 5 minutes
    after the owning server's clock; a download's late reports accepted
    for 30 days after its `play`.
16. **The speed cap** is per download, over any 10 seconds, paced, never
    refused (section 7).
17. **Scans** (section 8): `scansPerDay` up to 1,000,000, 0 meaning off;
    tickets of 10 minutes; 2 reads at once; ~~32 MiB per scan ticket~~
    `scanBytes` since proposal 17.
18. **One refusal type for every limit** (`stream-limit-reached`), for
    downloads and scans as for streams; `delivery-refused` is new.

**Proposal 17** (adopted 2026-11-22, [neutrality](../../proposals/0017-neutrality.md)):
no capabilities, the baseline without `delivery` and its picture,
remux to an `mp2t` or `mp4` media type, `copy` and `range` in a
transcode, `audioRenditions` in the answer, the copy and tracks chosen
by the viewer's server, answers without URLs (the ticket), opaque
playback IDs, no default bitrate ceiling, limits up to 1,000, no
`paused`, and `scanBytes` in the scan's answer. Choices this draft made
where it was silent, to be confirmed:

19. **`purpose` stays closed**: one not of its list is `400`; an unknown
    `delivery` token is `409` `delivery-refused`, like a remux's media
    type or a codec, since it names what to serve (section 2.1).
20. **The baseline is HLS** (section 2.7): without `delivery`, an answer
    with `protocol` `file` is refused, and the baseline's audio has at
    most 2 channels, as a transcode with `audioChannels` left out.
21. **The answer's `audioRenditions`**: absent means `false`; `true`
    with `protocol` `file` is dropped (section 2.3).
22. ~~**The title's own sound**~~ Superseded: the recommended defaults
    left the protocol for the notes.
23. ~~**An owning server without `scanBytes`**~~ Superseded: a scan's
    answer states its `scanBytes`; a caller counts on 4 MiB when it is
    missing (section 8).

Settled while msfp-go was written (2026-11-22), as the text reads:

24. **`burnSubtitle`'s form** is checked at step 4, with the track IDs,
    not at step 1 (section 2.2).
25. **Empty strings**: an empty `audioTrack` is malformed (`400`, step 4); an empty `title` in `progress` or `stop`
    is a title that is neither the request's nor of its queue (`400`).
26. ~~**`paused` on a `stop`**~~ Superseded: `paused` left `progress`
    with proposal 17, and is an unknown member everywhere.
27. ~~**An answer track's `channels`**~~ Superseded: the answer carries
    no tracks.

Proposal 17, written in on 2026-11-22: choices where its text gave no
form, to be confirmed:

28. **`versionId` in the answer** is required, as proposal 17's table
    gives it without "optional"; one of another form refuses the answer
    (section 2.3).
29. **A scan answer's `scanBytes`** that isn't an integer counts as
    missing: 4 MiB (section 2.3).
30. **A `range` that is a token not of its list** gets `409`
    `delivery-refused`, as an unknown `delivery` does, since it names
    what to serve (section 2.7).
31. **A transcode of a track with audio `copy`** still counts as a
    transcode: the rule that frees one is for video `copy` (section 2.6).

**Proposal 19** (adopted 2026-11-26, [MSFP Media](../../proposals/0019-msfp-media.md)):
the language of this part moved to MSFP Media's playing; this part keeps
the limits, what the owning server enforces, the limits on downloads and
scan access.

Since proposal 19, a target the owning server doesn't make (a remux's
media type, a transcode codec, `copy`, `range` or `burnSubtitle`) is
refused with `409` `target-refused`, not `delivery-refused`
([MSFP Media's base, section 4](https://github.com/msfp-io/msfp-media/blob/main/spec/v0/base.md#4-refusals));
items 18, 19 and 30 above read so. Choices of the writing:

32. **`capabilities` is refused** between servers (`400`), not ignored
    as an unknown member, so that a device is never described to a peer
    (section 2).
33. **A scan's idle limit** is counted on media requests alone, since
    a scan has no `progress` (section 8).
