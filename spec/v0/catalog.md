# MSFP version 0: the catalog part, version 0

**Draft.**

The `catalog` part carries
[MSFP Media version 0's listing](https://github.com/msfp-io/msfp-media/blob/main/spec/v0/listing.md)
between paired servers: its `libraries`, `titles` and `image` calls,
its title keys, copies and extras, as that document defines them. MSFP's
catalog is MSFP Media's listing, limited to what's shared. This part
adds what only the federation knows: which libraries a server offers a
peer and which the peer wants, what is shared (section 5), the server's
name and its limits in the `libraries` answer, the spacing of readings,
and the refusals of a federation call.

It needs only the [core](core.md). The content parts, [video](video.md)
and [music](music.md), carry MSFP Media's video and music. Its calls
follow the core's rules for every call
([core section 8](core.md#8-common-rules-for-every-call)): signed `POST`s
under the peer's base path, answered only to peers in state `paired`,
and only when `catalog` is on between the two servers
([core section 10](core.md#10-versions-and-parts)). A server that doesn't
speak `catalog` with the caller answers its calls with a signed `404`
`not-supported`. A peer that can't be called
([core section 7.5](core.md#75-a-peer-that-cant-be-called)) is never
called: it reads its peers' libraries and titles, and nobody reads its
own.

The words "must", "must not" and "should" carry their usual meaning in
specifications.

## Contents

1. [Terms](#1-terms)
2. [Libraries](#2-libraries)
3. [Titles](#3-titles)
4. [Title keys](#4-title-keys)
5. [What is shared](#5-what-is-shared)
6. [Security considerations](#6-security-considerations)
7. [Open questions](#7-open-questions)

## 1. Terms

Library, copy, title key, scheme and content part are MSFP Media's
([listing section 1](https://github.com/msfp-io/msfp-media/blob/main/spec/v0/listing.md#1-terms)).
Its source is the **owning server**, the server that holds a title's
copy; its caller is the **viewer's server**, the peer that reads it.

- **Offer**: the libraries a server lets one peer see. Each server sets
  its own offers, per peer, or per group: a peer is then offered what
  each of its link's relations offers ([groups section 9](groups.md#9-sharing-and-limits)).
- **Want**: the libraries of a peer's offer that this server reads. Only
  titles in a library one side offers and the other wants are shared.
- **What the caller may see**, in MSFP Media's words, is what section 5
  shares: the libraries offered to that peer, the titles in them of the
  content parts on between the two servers, and `local` titles only
  where the owner allows them.

## 2. Libraries

`libraries`, under the peer's base path, is
[MSFP Media's `libraries`](https://github.com/msfp-io/msfp-media/blob/main/spec/v0/listing.md#2-libraries):
a paired peer asks which libraries the receiver offers it. Library IDs
and what the caller keeps of each library are as
[listing sections 2.1](https://github.com/msfp-io/msfp-media/blob/main/spec/v0/listing.md#21-library-ids)
and [2.2](https://github.com/msfp-io/msfp-media/blob/main/spec/v0/listing.md#22-what-the-caller-keeps)
say.

MSFP adds two members to the answer:

| Field | Type | Content |
|-------|------|---------|
| `name` | string, optional | the receiver's name now, for the caller's menu |
| `limits` | limits object, optional | sent only when `playback` is on between the two servers: what the caller may use of the receiver at once ([playback section 5](playback.md#5-limits)) |

What the caller keeps of them:

- `name` cleaned as in [core section 9.1](core.md#91-pair). Names are
  text, never markup: a receiver treats them as text wherever it shows
  them, and may clean further. A top-level `name` that isn't a string
  fails the answer (the caller treats the peer as unreachable and keeps
  what it had).
- a `limits` that isn't an object, or whose members aren't integers, is
  ignored, as are limits out of range.

Refusals: signed `404` when the caller isn't `paired`; signed `404`
`not-supported` when `catalog` is off; the rest as listing section 2.2
says.

## 3. Titles

`titles`, under the peer's base path, is
[MSFP Media's `titles`](https://github.com/msfp-io/msfp-media/blob/main/spec/v0/listing.md#3-titles):
a paired peer reads which titles the libraries it wants hold, a page at
a time, limited to what is shared with it (section 5). Paging, changes,
what the caller keeps, copies, extras and filters are as
[listing sections 3.1 to 3.6](https://github.com/msfp-io/msfp-media/blob/main/spec/v0/listing.md#31-paging-and-ordering)
say; language tags as
[MSFP Media's base section 2.4](https://github.com/msfp-io/msfp-media/blob/main/spec/v0/base.md#24-language-tags).
MSFP adds:

- **The answer's bound.** A caller accepts any answer within the core's
  bound for an answer, 4 MiB ([core section 3](core.md#3-transport)).
- **Whole readings forced by sharing.** A change of what the owning
  server offers the caller, of its `local` switch (section 5), and of
  which parts are on between the two servers
  ([core section 10](core.md#10-versions-and-parts)) are changes of what
  the caller may see: each forces a whole reading
  ([listing section 3.2](https://github.com/msfp-io/msfp-media/blob/main/spec/v0/listing.md#32-whole-list-or-changes)).
- **Spacing.** A viewer's server should read every paired peer's titles
  (never those of a peer that can't be called) at most every 30
  minutes, and at once when what it wants of one changes; it should give
  a reading 5 minutes. After `play`'s answer disagrees with the catalog,
  it reads again, with `since`, as soon as this spacing allows.
- **Media info comes from the catalog only**, as
  [listing section 3.4](https://github.com/msfp-io/msfp-media/blob/main/spec/v0/listing.md#34-copies)
  says, with one exception: scan access
  ([playback section 8](playback.md#8-scan-access)).

Refusals: signed `404` when the caller isn't `paired`; signed `404`
`not-supported` when `catalog` is off; the rest as listing section 3.3
says.

## 4. Title keys

Title keys are
[MSFP Media's](https://github.com/msfp-io/msfp-media/blob/main/spec/v0/listing.md#4-title-keys),
with their identity rules, opaque parts, the description of `local`
titles and their `image` call
([listing sections 4.1 to 4.4](https://github.com/msfp-io/msfp-media/blob/main/spec/v0/listing.md#41-same-title-same-key)).
A `local` title is sent only when the owner shares them (section 5).

`image`, under the peer's base path, is signed as every call is. For
it, MSFP's refusals are: signed `404` `not-supported` when no content
part with `local` titles is on between the two servers, checked before
the body; and `429` `rate-limited` when asked too often
([core section 8.3](core.md#83-limits-and-overload)), after which the
viewer's server waits as core section 8.3 says.

## 5. What is shared

The owning server decides alone. The viewer's server can't widen
anything.

- Only titles in libraries it offers that peer, and only those the peer
  asked for. `titles` ignores other library IDs.
- Only titles of the content parts on between the two servers
  ([core section 10](core.md#10-versions-and-parts)). A receiver skips
  the others anyway
  ([listing section 3.3](https://github.com/msfp-io/msfp-media/blob/main/spec/v0/listing.md#33-what-the-caller-keeps-of-each-title)).
- A server offers only its own libraries, never what it got from another
  peer.
- **`local` titles**
  ([listing section 4.3](https://github.com/msfp-io/msfp-media/blob/main/spec/v0/listing.md#43-titles-no-public-source-knows-the-description))
  are shared only when the owner allows it: off by default, set per
  pairing and per group, beside the offers. An owning server must not
  share them until its owner allows it. A link shares them when one of
  its relations allows it and offers the library that holds them
  ([groups section 9](groups.md#9-sharing-and-limits)). While it is off,
  the owning server leaves its `local` titles out of `titles`, as if it
  couldn't name them. Turning it on or off is a change of what it offers
  the caller: it forces a whole reading (section 3).
- A title its content part can't name under a public scheme, and that
  isn't shared as `local`, is not sent.
- Of the owning server, a peer never learns files, paths or its media
  server's item IDs, not even inside library IDs, version IDs or the
  opaque parts of keys
  ([listing section 2.1](https://github.com/msfp-io/msfp-media/blob/main/spec/v0/listing.md#21-library-ids)).
  Copies, tracks and extras carry no names
  ([MSFP Media's video section 3](https://github.com/msfp-io/msfp-media/blob/main/spec/v0/video.md#3-titles)).

## 6. Security considerations

[Listing section 5](https://github.com/msfp-io/msfp-media/blob/main/spec/v0/listing.md#5-security-considerations)
holds, a compromised peer being its hostile source. MSFP adds only that what is shared is the owning
server's alone to decide (section 5), checked on every signed call.

## 7. Open questions

Settled by the owner on 2026-11-17; each says how. The section numbers
are those of this part before
[proposal 19](../../proposals/0019-msfp-media.md) moved its language to
MSFP Media's listing; what they name is now in listing.

1. **Paging.** A whole-list reading whose size is a multiple of 5,000 ends
   with one empty page.
   **Settled:** fine as it is; one more small call. Since the revision
   of version 0 ([proposal 16](../../proposals/0016-version-0-revision.md)),
   `next` is set only when titles remain, so no reading ends with an
   empty page.
2. **How often a library ID may change** (proposal 2, open point 2).
   Section 2.1 says "stable" and "never reused"; a cap (for example "only
   when the library is removed") would be hard to check.
   **Settled:** "stable" and "never reused", no cap.
3. **One rule for every identifier** (proposal 2, open point 3). Rather
   than one rule for library IDs here and one for version IDs in
   playback, the specification could state once that every identifier
   an owning server sends is opaque, and list them.
   **Settled:** yes, section 2.1 states it once and lists them.

The open points of proposal 5 (TMDB's numbering, IMDb episode IDs, the
cap of 16 for `with`) moved to [video](video.md#8-open-questions) with the
rules they concern.

**Choices this draft made, confirmed by the owner on 2026-11-17** (when the video rules were moved
out of this part on 2026-11-17):

4. **Language tags stay defined here** (section 3), since `play`'s
   `audioLanguage` uses them whatever the content part; `video` uses them
   in its copies' tracks. Since proposal 19 they are defined in MSFP
   Media's base section 2.4.
5. **With no content part on** between two servers, `libraries` still
   lists the libraries offered, and `titles` holds none of them
   (section 5).

**Choices made when writing the revision of version 0** (proposal 16
gave no bound for them):

6. **The 3 MiB bound is the owning server's** (section 3.1): a caller
   still accepts what the core allows, and a title that alone would pass
   3 MiB is not sent.
7. **The opaque parts of keys** (section 4.2) take the form of library
   IDs: 1 to 64 characters of `A–Z a–z 0–9 _ -`.
8. **The description's `year`** is 1800 to 9999, as video's (music's
   titles carry dates, not `year`); its image bound is 1 MiB, and the
   image is fetched with the catalog's own `image` call (section 4.4),
   shared by video and music.
9. **`local` sharing on a link** follows the union of its relations, as
   offers do (section 5).
10. **Extras** are sent only with their title, in its library, and go
    with it (section 3.5).
11. **After `play` disagrees with the catalog**, the viewer's server
    reads again with `since` as soon as section 3.2's spacing allows
    (section 3.4).

**Neutrality, decided by the owner on 2026-11-22**
([proposal 17](../../proposals/0017-neutrality.md)):

12. **A library's `kind` is open** (section 2): a token, kept when
    unknown; absent when the owning server has none. `""` is no longer
    sent, and counts as absent. Dropped by the whole of proposal 17
    (item 15).
13. **Schemes in a registry**
    ([MSFP Media's registry](https://github.com/msfp-io/msfp-media/blob/main/spec/registry.md)
    since proposal 19): a new scheme is an entry there. `tmdb` joins
    `ids`, and `ids` join one title keyed under two schemes
    (section 4.1). The order of schemes is a "should".
14. **No product text here.** What this part said of media servers, and
    the JPEG DNL rule, moved to the implementers' notes, now
    [MSFP Media's notes](https://github.com/msfp-io/msfp-media/blob/main/spec/v0/notes.md#catalog).
    The image's bounds are unchanged: a JPEG whose header gives a height
    of 0 is still refused, being under 1 pixel tall.

**The whole of proposal 17, as frozen** (2026-11-22):

15. **No library `kind`** (section 2): the titles' keys say what a
    library holds. **No title `kind`** (section 3): the key says it.
    Both, sent, are unknown members, ignored.
16. **`ids` only where they add something** (section 4.1): on video,
    episodes and titles keyed under `tvdb`; on music, `mbreleasegroup`
    and `mbrecording` on `mb` titles; never on `local` titles.
17. **`image`'s answer is `{"ticket"}`** (section 4.4): the viewer's
    server builds the URL from the stream root and the ticket. The
    `local` title's "never a file name" guidance went; the switch of
    section 5 protects it.

**MSFP Media, adopted by the owner on 2026-11-26**
([proposal 19](../../proposals/0019-msfp-media.md)):

18. **The catalog carries MSFP Media's listing.** Libraries, titles,
    title keys, copies, extras, the description of `local` titles and
    `image` moved there, with listing's new filters (`kind`, `under`)
    and `libraries` optional in `titles` (absent: every library offered
    to the caller). This part keeps offers and wants, what is shared,
    the `name` and `limits` of the `libraries` answer, the spacing of
    readings, and the federation's refusals.
