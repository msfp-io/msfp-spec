# MSFP version 0: the media part, version 0

**Draft.**

The `media` part carries
[MSFP Media version 0's fetching](https://github.com/msfp-io/msfp-media/blob/main/spec/v0/fetching.md):
what the stream URLs serve, how they are fetched (HLS, and the file
itself read by byte ranges), their bounds, and the tickets that open
them. In MSFP, the source is the owning server and the caller is the
viewer's server. This part adds what fetching leaves to the carrier:
the stream root and how it follows a move, the certificates a fetch
needs, who may fetch (the paired server only, never a browser), when a
peer's access ends, and a ticket kind of its own, `scan`. It needs only
the [core](core.md); [playback](playback.md) gives the tickets the URLs
are built from, and says when a download, a scan or a restart may
happen. Media is fetched with plain `GET` requests that carry the
ticket only: they are not signed.

The words "must", "must not" and "should" carry their usual meaning in
specifications.

## Contents

1. [The URLs](#1-the-urls)
2. [Requests](#2-requests)
3. [What is fetched](#3-what-is-fetched)
4. [How long a ticket works](#4-how-long-a-ticket-works)
   1. [Downloads](#41-downloads)
   2. [Scans](#42-scans)
5. [Security considerations](#5-security-considerations)
6. [Open questions](#6-open-questions)

## 1. The URLs

Media URLs, their names, how the viewer's server builds them (built,
never received) and the form of a ticket are fetching's
([fetching section 1](https://github.com/msfp-io/msfp-media/blob/main/spec/v0/fetching.md#1-the-urls),
[section 4.1](https://github.com/msfp-io/msfp-media/blob/main/spec/v0/fetching.md#41-the-tickets-form)).
MSFP adds:

- **Certificates.** Every fetch needs a *trusted* or *named* certificate
  ([core section 3.2](core.md#32-https-certificates)), never just any:
  the ticket and the media have only TLS to protect them.
- **The stream root** of a server is `<address><prefix>/api/v1/stream/`,
  where `<address>` is its address exactly as its peer stores it, and
  `<prefix>` is the part of its base path before `/api/v1/federation/`
  ([core section 11](core.md#11-the-discovery-document)); without a
  discovery document, `<prefix>` is empty. Example:
  `https://a.example.org/api/v1/stream/`. While the owning server is
  moving, it uses the address whose host the `play` request was signed
  for ([core section 13.1](core.md#131-the-server-that-moves)).
- **A peer that can't be called**
  ([core section 7.5](core.md#75-a-peer-that-cant-be-called)) is never
  the owning server: nothing is fetched from its stream root, which is
  under an address that doesn't resolve. It fetches its peers' media as
  any viewer's server does.

## 2. Requests

Requests are fetching's
([fetching section 2](https://github.com/msfp-io/msfp-media/blob/main/spec/v0/fetching.md#2-requests):
ranges, methods, statuses, bounds). Between servers:

- **Never a browser.** The viewer's server sends no `Origin`,
  `Sec-Fetch-Site` or `Sec-Fetch-Mode` header. The owning server answers
  `404` to a peer's ticket when any of them is present, because only a
  browser sends them, and a browser must never use a peer's ticket. The
  viewer's server sends no signature headers and no cookies either.
- **The connection** is checked as
  [core section 3.3](core.md#33-connecting-to-peers) says, like every
  connection to a peer.
- **Renewals.** A fetch refused for its certificate makes the viewer's
  server send a `hello` and try again once, as
  [core section 3.2](core.md#32-https-certificates) says ("Renewals").
  It is the one retry of a failed fetch that MSFP allows
  ([fetching section 2.3](https://github.com/msfp-io/msfp-media/blob/main/spec/v0/fetching.md#23-statuses)).
- **Bounds per peer.** The bounds on fetches at once
  ([fetching section 2.4](https://github.com/msfp-io/msfp-media/blob/main/spec/v0/fetching.md#24-bounds))
  hold for each peer.

## 3. What is fetched

What is fetched, and what the viewer's server checks before it relays
it, are fetching's
([fetching section 3](https://github.com/msfp-io/msfp-media/blob/main/spec/v0/fetching.md#3-what-is-fetched):
HLS playlists, the file, the file as a plain file, subtitles, fonts,
sheets and images). MSFP adds nothing to them.

## 4. How long a ticket works

The ticket kinds of the language, what each opens and how long each
works are fetching's
([fetching section 4.2](https://github.com/msfp-io/msfp-media/blob/main/spec/v0/fetching.md#42-kinds-and-how-long-each-works)).
MSFP adds one kind:

| Kind | Given by | Opens | Works until |
|------|----------|-------|-------------|
| scan | `play` with `purpose: scan` and `delivery: file` ([playback section 8](playback.md#8-scan-access)) | `file` only, by ranges, within the scan answer's `scanBytes` (section 4.2) | its playback ends, its idle limit included; at most 10 minutes after the answer |

The carrier's endings of a playback, and so of its ticket
([fetching section 4.2](https://github.com/msfp-io/msfp-media/blob/main/spec/v0/fetching.md#42-kinds-and-how-long-each-works):
access withdrawn, library no longer seen), are, in MSFP:

- the pairing ends, the caller is no longer `paired`, or its key is
  suspended ([core section 12.6](core.md#126-revocation));
- the library holding the copy is no longer offered to that peer.

A ticket works only for the pairing it was given to, and never for a
browser (section 2). A playback started before a peer's address changed
keeps its URLs until it ends
([core section 13.2](core.md#132-a-peer-that-learns-of-a-move)).

### 4.1 Downloads

A download's limits are in `limits`
([playback section 5](playback.md#5-limits): `downloads`,
`downloadsPerDay`, `downloadKbps`). `downloadKbps` is the speed cap by
which the owning server paces a download ticket's bytes, as
[fetching section 4.3](https://github.com/msfp-io/msfp-media/blob/main/spec/v0/fetching.md#43-downloads)
says.

### 4.2 Scans

A scan lets a media server that reads files to list them read a capped
part of a copy, outside a watched playback
([playback section 8](playback.md#8-scan-access)). At the media level:

- Every fetch under a scan ticket is a fetch of `file` with a `Range`.
  A fetch without one is answered `416`.
- **The cap.** The owning server counts the body bytes it sends under
  the ticket, a byte sent twice counted twice. Together, they never pass
  the scan answer's `scanBytes`
  ([playback section 8](playback.md#8-scan-access)), 4 MiB (4,194,304
  bytes) to 1 GiB (1,073,741,824 bytes), per scan ticket (a scan is of
  one copy); an answer without it, or with one out of range, gives the
  floor, 4 MiB, the value the caller counts on. The pairing's
  `scansPerDay` bounds the total. A range that would pass the cap is
  served as a `206` cut short at the cap (its *end* earlier than asked,
  as [fetching section 2.1](https://github.com/msfp-io/msfp-media/blob/main/spec/v0/fetching.md#21-ranges)
  allows); a range that starts once the cap is spent is answered `416`.
- **Pace.** The owning server serves at most 2 of a scan's fetches at
  once, and answers `503` past that
  ([playback section 8](playback.md#8-scan-access)); it may pace them as
  it does a download's.
- The viewer's server passes a `416` past the cap to its reader as a
  failed read.

Test cases: `scans` in [vectors/v0/media.json](../../vectors/v0/media.json).

## 5. Security considerations

Fetching's considerations hold
([fetching section 5](https://github.com/msfp-io/msfp-media/blob/main/spec/v0/fetching.md#5-security-considerations):
media protected by TLS only, hostile media, what remains, the file as
it is). Between servers:

- **Certificates.** TLS protects the media only with a *trusted* or
  *named* certificate (section 1); any other is refused.
- **Never a browser.** A peer's ticket opens nothing for a browser
  (section 2), so a viewer's server can't hand one on by mistake.
- **Proxies in front of the stream root.** A server whose stream root
  sits behind a reverse proxy or a load balancer applies
  [fetching section 4.1](https://github.com/msfp-io/msfp-media/blob/main/spec/v0/fetching.md#41-the-tickets-form)
  to its access logs too: a peer's tickets, `scan` tickets included,
  never stay in them.
- **Scans** read a peer's copy outside a watched playback. Their cap
  (section 4.2) is the owning server's, enforced on every byte it sends,
  never left to the viewer's server.

## 6. Open questions

Whether media should be signed is
[fetching's open question](https://github.com/msfp-io/msfp-media/blob/main/spec/v0/fetching.md#6-open-questions).

The choices below were made while this part held the fetching rules
that are now MSFP Media's; their sections are fetching's unless MSFP is
named. They are kept here as their record.

Choices of the review round of 2026-11-16, all confirmed by the owner on
2026-11-17 (they may still change as implementations show):

2. **The HLS subset** (fetching section 3.1) was what Overfin sends.
   Audio renditions (`#EXT-X-MEDIA`) came with
   [proposal 16](../../proposals/0016-version-0-revision.md); I-frame
   playlists and encryption are left for a later version.
3. **Unknown tags** in a relayed playlist: dropped or passed on, at the
   viewer's server's choice (fetching section 3.1), since URIs are
   checked in every tag either way.
4. **`Content-Range` checks** (fetching section 2.1) are required of the
   viewer's server; Overfin makes them since 2026-11-16.

The choices on timestamp tags in WebVTT and on checks left to the player
moved to video's open questions with the WebVTT and PGS rules.

**Choices this draft made, confirmed by the owner on 2026-11-17** (when the video rules were moved
out of this part on 2026-11-17):

5. **Bounds stay with fetching**: the sizes of subtitle parts, picture
   parts, fonts, sheets and images (fetching section 2.4), and the rule
   that they are fetched without `Range` (fetching section 2.1), are
   listed with the other bounds, though only the content parts use
   them.
6. **URL checks stay with fetching** (fetching section 1): the `<id>`
   name that video's URLs add follows the same pattern as `<name>`.

**Choices made when proposal 16 was written in** (2026-11-22, where the
proposal gives no number or form; they may change as implementations
show):

7. **URL forms**: `subtitles/<id>/whole`, `ass/<id>/<part>` and
   `ass/<id>/whole`, `fonts/<n>` (0 to 7; below `fontCount` since
   proposal 17) and `image` (fetching section 1).
   The forms of every URL a content part adds are fetching's; video says
   only what their bytes are.
8. **Sizes**: a whole text track 16 MiB; a font 10 MiB, an image 1 MiB
   and the scan cap 32 MiB, reading the proposal's MB as MiB
   (fetching section 2.4, and MSFP's section 4.2). Since proposal 17,
   the scan cap is the scan answer's `scanBytes`, 4 MiB to 1 GiB.
9. **Tickets**: a `local` title's image comes under an `image` ticket,
   of 10 minutes; a scan ticket follows a playback's idle limit, and
   lasts 10 minutes at most (fetching section 4.2, and MSFP's
   section 4).
10. **The file**: `ETag: "<etag>"` and `Accept-Ranges: bytes` on every
    answer, a `206` required for a satisfiable range, never `*` as its
    total; no conditional headers (fetching sections 2, 2.1 and 3.2).
11. **HLS**: target duration 1 to 60 seconds; renditions named by track
    ID, in one group `audio`, at most 32; a segment's headers within 30
    seconds (fetching sections 2.4 and 3.1).
12. **Fonts** as `font/ttf` or `font/otf`, told by their first bytes; no
    font collections. **ASS** fetched as ASS in stretches or whole, at
    URLs of its own, relayed as `text/x-ass` (fetching section 3.4).
13. **Pacing** of downloads over windows of 10 seconds (fetching
    section 4.3); a scan read past the cap cut short rather than
    refused; a scan read without `Range` refused (MSFP's section 4.2).
14. **`405`** for `HEAD` comes with `Allow: GET`, and answers every
    other method but `GET` too (fetching section 2.2).

**Proposal 17** (adopted 2026-11-22): URLs built from the ticket, the
owning server serving exactly their names, the copy's `mediaType` for
the relayed file, the master playlist's attributes and `hvc1` in `mp4`
segments, the scan cap in the scan's answer, 64 fonts a copy, the `HEAD`
rule kept without a product's reason, and the answer's
`audioRenditions` deciding the renditions. Settled while msfp-go was
written (2026-11-22):

15. **A segment's `206` with `*` as its total** is a failed fetch when it
    answers a `bytes=-<count>` range: its *start* can't be checked
    (fetching section 2.1).

Proposal 17, written in on 2026-11-22: choices where its text gave no
form, to be confirmed:

16. **`RESOLUTION`** is required only for a stream with video: an
    audio-only stream has no picture to describe (fetching section 3.1).
17. **No new check for the master playlist's attributes**: the viewer's
    server's steps (fetching section 3.1) don't refuse a playlist that
    lacks them; the conformance suite checks owning servers.
18. **Segments keep their own type rule** (fetching section 3): only the
    file is relayed with the copy's `mediaType`.

**Proposal 19** (adopted 2026-11-26): the fetching rules moved to MSFP
Media's fetching; this part keeps the stream root, the certificates,
who may fetch, the carrier's endings of a ticket, and scans.
