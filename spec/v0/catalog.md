# MSFP version 0: the catalog part, version 0

**Draft.**

The `catalog` part says which libraries a server offers a peer, which
titles they hold, and the rules every title key follows. It needs only
the [core](core.md). The content parts, [video](video.md) and
[music](music.md), define the kinds of title, their keys and what each
title carries. Its calls follow the core's rules for every call
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

- **Library**: a collection of titles on a server, with an ID and a
  name.
- **Offer**: the libraries a server lets one peer see. Each server sets
  its own offers, per peer, or per group: a peer is then offered what
  each of its link's relations offers ([groups section 9](groups.md#9-sharing-and-limits)).
- **Want**: the libraries of a peer's offer that this server reads. Only
  titles in a library one side offers and the other wants are shared.
- **Owning server**: the server that holds a title's copy.
- **Copy**: one version of a title as the owning server holds it (a
  file, or several files played in order), described in the title's
  `copies` by its content part.
- **Title key**: the text that names a title (section 4). Its form is
  defined by a content part.
- **Scheme**: the source of the identifier in a title key, or of an
  identifier in `ids`. The [registry](../registry.md) lists them; each
  content part defines the key forms of its own: `tmdb` (The Movie
  Database) and `tvdb` (TheTVDB) for [video](video.md#4-title-keys), `mb`
  (MusicBrainz) for [music](music.md#2-title-keys), and `local`, the
  owning server's own, for both (section 4.3).
- **Content part**: a part that names one kind of content's titles and
  says how they play: `video` and `music` in version 0
  ([core section 10.1](core.md#101-the-parts)).

## 2. Libraries

`libraries`, under the peer's base path. A paired peer asks which
libraries the receiver offers it.

Request: `{}`. No fields.

Answer `200`:

| Field | Type | Content |
|-------|------|---------|
| `name` | string, optional | the receiver's name now, for the caller's menu |
| `libraries` | array of library | the libraries offered to the caller, possibly empty |
| `limits` | limits object, optional | sent only when `playback` is on between the two servers: what the caller may use of the receiver at once ([playback section 5](playback.md#5-limits)) |

Library:

| Field | Type | Content |
|-------|------|---------|
| `id` | string | the library's ID: 1 to 64 characters of `A–Z a–z 0–9 _ -` |
| `name` | string | its name, text |

A library has no kind: the keys of its titles say what it holds
(section 3). A viewer's server types an empty library by its first
titles.

### 2.1 Library IDs

A library ID is chosen by the owning server and means nothing outside
it. It must not be, contain, or be computable by a peer from, any
identifier of the media server, a file, a path or a name. A keyed MAC of
such an identifier, under a secret the server keeps, satisfies this; a
plain hash doesn't.

A library ID is stable: the same library keeps the same ID across
restarts, renames, upgrades and, where the server can tell it is the same
library, a change of media server. An ID is never reused for another
library.

The ID may be the same toward every peer, or differ per peer. A peer must
not compare library IDs across peers.

Recommended practice, not required: a random value (at least 96 bits,
base64url), stored with the library when the server first offers it, and
kept with the server's other federation state. Unlike a MAC of the media
server's ID, a stored value survives a change of media server: the
operator (or an import) maps the old library to the new one.

**Every identifier an owning server sends** follows the first paragraph
of this section: library IDs, version IDs
([playback section 2.3](playback.md#23-the-answer)), playback IDs and
tickets ([playback](playback.md#2-play), [media](media.md#1-the-urls)).
None may be, contain, or be computable from, an identifier of the media
server, a file, a path or a name. Track IDs (`a<n>`, `s<n>`) are
positions, not identifiers: they have only the forms `a<n>` and `s<n>`
([playback section 2.1](playback.md#21-the-request)), and no other rule
for identifiers applies to them.

### 2.2 What the caller keeps

- at most 200 libraries;
- entries whose `id` breaks the form above are skipped; of two libraries
  with the same `id`, the first;
- names cleaned as in [core section 9.1](core.md#91-pair) (`?` when
  empty). Names are text, never markup: a receiver treats them as text
  wherever it shows them, and may clean further;
- members of the wrong JSON type ([core section 2.3](core.md#23-json)): a
  `libraries` that isn't an array, or a top-level `name` that isn't a
  string, fails the answer (the caller treats the peer as unreachable and
  keeps what it had); an entry of `libraries` that isn't an object, or
  whose `id` isn't a string, is skipped; a library's `name` that isn't a
  string counts as absent (`?`); a `limits` that isn't
  an object, or whose members aren't integers, is ignored, as are limits
  out of range.

Refusals: signed `404` when the caller isn't `paired`, or when the body
isn't a JSON object; signed `404` `not-supported` when `catalog` is off;
`500`.

## 3. Titles

`titles`, under the peer's base path. A paired peer reads which titles
the libraries it wants hold, a page at a time. It can ask only for what
changed since its last reading.

Request:

| Field | Type | Required | Content |
|-------|------|----------|---------|
| `libraries` | array of string | yes | library IDs; those not offered to the caller are ignored |
| `after` | string | no | the page starts after this title key; `""` for the first page; at most 512 bytes |
| `since` | integer | no | the `seq` of a former reading, to get only what changed; 0 or absent for the whole list; a negative one is answered like 0 |

Answer `200`:

| Field | Type | Content |
|-------|------|---------|
| `titles` | array of title | the page; absent counts as empty |
| `next` | string, optional | where the next page starts (pass it as `after`); absent after the last page |
| `changes` | boolean, optional | `true`: the answer holds only changes since `since` |
| `gone` | array of string, optional | with `changes`: title keys no longer held |
| `seq` | integer, optional | the receiver's catalog change number now; ask `since` it next time |

Title:

| Field | Type | Content |
|-------|------|---------|
| `key` | string | the title key (section 4); the kind of title is the one it names |
| `library` | string | one of the requested libraries that holds it: when several do, the one whose ID comes first in byte order |

A title carries, besides, the members its content part defines for the
kind its key names: [video section 3](video.md#3-titles) for films, series, seasons,
episodes and extras,
[music section 3](music.md#3-what-a-title-carries) for artists, albums
and tracks.

**Language tags.** A tag uses the shortest code BCP 47 allows: the
ISO 639-1 code where one exists (`en`, `fr`), else ISO 639-2/T or 639-3
(`yue`), with a region when known (`fr-CA`), in the case conventions of
[core section 2.5](core.md#25-case). `und`, `mul`, `zxx` and `mis` are not
sent. An ISO 639-2 code, /B (`fre`, `ger`, `chi`) or /T (`fra`, `deu`,
`zho`), of a language that has an ISO 639-1 code is sent as that code
(`fr`, `de`, `zh`), a region kept (`fre-CA` is sent `fr-CA`). Test cases: `languageCodes` in
[vectors/v0/strict.json](../../vectors/v0/strict.json).

**A valid tag** is one already in the form a server sends: the tag these
rules give for it is itself, unchanged (`fr`, `fr-CA`, `yue`,
`zh-Hant-TW`, `es-419`). Anything else is not a valid tag: `fre`
(not yet converted), `fr-ca` or `FR` (another case), `e` (a tag is 2
to 12 characters, its language 2 to 8 letters), `und`, `fr_CA`. Wherever a
title or a track carries a language tag (video's tracks, music's lyrics),
the receiver checks it is a valid tag, as its part says, and never
converts it: an invalid one counts as absent in what it keeps, and
refuses a request. Valid tags are compared byte for byte, or without
case where a part says so; both give the same answer on valid tags.

### 3.1 Paging and ordering

- Titles come in ascending byte order of `key`. A page holds the titles
  with keys strictly greater than `after`.
- A page is bounded twice: it holds at most 5,000 titles, and its answer
  body is at most 3 MiB (3,145,728 bytes) of JSON. The owning server ends
  the page early when the next title would pass either bound. In a
  changes answer, `titles` and `gone` together hold at most 5,000 keys,
  taken in one byte order, within the same 3 MiB.
- A title that alone would pass 3 MiB is not sent. (The bounds of the
  content parts keep a title far below it.)
- In a whole-list answer, `next` is the last key of the page, and is set
  whenever titles remain after it, whether the page is full or was ended
  early. It is absent after the last title.
- In a changes answer, `next` is set when there may be more changes after
  the page. A changes page may hold fewer than 5,000 titles and still
  have a `next`.
- A page need not be full. Ending early is allowed, not required: a
  whole-list page may hold fewer titles than either bound allows and
  still have a `next`.
- In a changes answer, `next` may be any string. The caller checks
  nothing of it but the stop rule below.
- The caller stops when `next` is absent, empty, or not greater than its
  `after`. A short page with a `next` is no end: the caller asks on.
- The owning server never sends a `next` over 512 bytes, the bound of
  `after`. A caller that gets one passes it back as it is; the request is
  refused (section 3.3), and the reading fails (section 3.2).
- A caller accepts any answer within the core's bound for an answer
  ([core section 3](core.md#3-transport)); the 3 MiB bound is the owning
  server's, and leaves room for the rest of the answer.

Test cases: `titlePages` in
[vectors/v0/strict.json](../../vectors/v0/strict.json).

### 3.2 Whole list or changes

- The receiver answers with changes only when `since` is above 0, it
  still keeps its change log back to `since`, `since` isn't above its
  current `seq`, and what it offers the caller hasn't changed since
  `since`. Otherwise it answers with the whole list, whatever `since` was.
  A change of what it offers the caller (section 5), and a change of
  which parts are on between the two servers
  ([core section 10](core.md#10-versions-and-parts)), count as such
  changes: they force a whole reading. A receiver that keeps a change log
  gives each one a number there, so that a `since` from before it gets
  the whole list.
- A caller asks with `since` only when `libraries` is exactly the set of
  the reading that gave that `seq`; otherwise it asks for the whole list.
- The first page decides which kind of reading it is, and its `seq` is
  the one the caller keeps. Every later page of the same reading must
  have the same `changes` value. The caller treats a mix as a failure.
- After a whole-list reading, the caller drops every title of that peer
  it didn't see in the reading's pages. After a changes reading, it keeps
  `titles` (new or changed) and drops `gone`.
- A reading counts only once its last page is read. A whole-list reading
  that fails halfway (a failed answer, no answer, its time used up)
  changes nothing: the caller keeps the list of its last complete reading
  and drops the pages it read.
- A title has changed when anything sent for it changes: its `library`, its description (section 4.3), or any member its content
  part adds (video's `copies`, `place`, `with` and `ids`, music's
  descriptions and copies). A change of its copies is a change of the
  title (section 3.4). A changes reading sends it again, whole.
- A receiver that sends no `seq` gets asked for the whole list every
  time.
- `seq` is an opaque positive counter. Only the receiver gives it a
  meaning.
- A receiver should keep its change log at least 30 days. A caller should
  read every paired peer's titles (never those of a peer that can't be
  called) at most every 30 minutes, and at once
  when what it wants of one changes; it should give a reading 5 minutes.

### 3.3 What the caller keeps of each title

Anything else is skipped, title by title:

- the key parses as a title key of a content part on between the two
  servers, is in canonical form (section 4), and its kind and scheme are
  ones that part defines;
- `library` is one of the libraries it asked for;
- the members the content part defines are kept as that part says
  ([video section 3.1](video.md#31-what-the-caller-keeps),
  [music section 3.2](music.md#32-what-the-caller-keeps)); a bad member
  is skipped, not the whole title, unless the part says otherwise;
- `gone` keys over 128 bytes are skipped.

Members of the wrong JSON type ([core section 2.3](core.md#23-json)):

- `titles` or `gone` that isn't an array, `next` that isn't a string,
  `changes` that isn't a boolean, or `seq` that isn't an integer fails
  the answer: the caller treats the peer as unreachable and keeps what it
  had. Skipping one of them could end a reading early or mix the kinds of
  reading, and the caller would then drop titles the peer still holds.
- An entry of `titles` that isn't an object, or whose `key` or `library`
  isn't a string, is skipped.
- A member a content part defines, of the wrong type, is read as that
  part says.
- An entry of `gone` that isn't a string is skipped.

Refusals: signed `404` when the caller isn't `paired`, or the body isn't
valid JSON, or `libraries` is missing or isn't an array of strings, or
`after` is over 512 bytes, or `since` isn't an integer;
signed `404` `not-supported` when `catalog` is off; `500`.

### 3.4 Copies

A film, an episode, an extra (section 3.5) or a music track describes its
copies in `titles`: their length, media type, streams and tracks, as its
content part defines them ([video section 3.2](video.md#32-copies) for
films, episodes and extras, [music section 3.3](music.md#33-a-tracks-copies)
for tracks). A viewer's server
builds its library from them before anyone plays.

- **Media info comes from the catalog only.** A viewer's server must not
  read a peer's file, nor start a playback, to learn what a copy is:
  every read would start a playback on the owning server. What its media
  server would read from a local file, it takes from the copy in
  `titles`.
- **A change of a title's copies** (a copy added, removed, replaced, or
  its `etag` changed) is a change of the title: a changes reading sends
  the title again, whole (section 3.2).
- **`play`'s answer and the catalog disagreeing** (the copy changed
  since the last reading): the answer wins for that playback
  ([playback section 2.3](playback.md#23-the-answer)), and the viewer's server reads the peer's
  titles again, with `since`, as soon as the spacing of section 3.2
  allows.

### 3.5 Extras

Trailers, featurettes and the like are titles of their own in `titles`,
of kind `extra`, each keyed under the title it belongs to (a film, a
series or a season): `movie/tmdb/348/x/<opaque>`.
[Video sections 3.9](video.md#39-extras) and [4.6](video.md#46-extras-keys)
define them, their types and their keys.

- An extra is listed, ordered and paged like any title (section 3.1): its
  key's byte order puts it after the title it is under, and not
  necessarily on the same page.
- An owning server sends an extra only while it sends the title it is
  under, in the same `library`. When that title goes, its extras go too,
  each in `gone`.
- A viewer's server shows an extra only under its title. An extra whose
  title it doesn't hold from that peer, at the end of a reading, it keeps
  hidden until the title comes.

## 4. Title keys

Titles are named by identifiers from a public source, or, for titles no
public source knows, by an opaque identifier the owning server chooses
(section 4.3): never by files, paths or the media server's item IDs. A
title key is ASCII text: a name for the kind, the scheme, and the
identifier, joined by `/`, followed by what the content part adds (a
season's or an episode's place in its series, an extra's opaque part).
The content parts define the kinds and the key forms; the
[registry](../registry.md) lists the schemes, which kinds each keys, and
where its key forms are defined:

| Content part | Kinds | Schemes, in order of preference | Example | Defined in |
|--------------|-------|---------------------------------|---------|------------|
| `video` | `movie`, `series`, `season`, `episode`, `extra` | `tmdb`, `tvdb`, `local` | `movie/tmdb/949`, `series/tvdb/81189/a12` | [video section 4](video.md#4-title-keys) |
| `music` | `artist`, `album`, `track` | `mb`, `local` | `album/mb/<mbid>` | [music section 2](music.md#2-title-keys) |

- **A new scheme is a registry entry**, not a new version of a part. A
  receiver skips the keys of a scheme it doesn't know, and ignores the
  `ids` of a scheme it doesn't know.
- **`local`** is the owning server's own identifier, for titles no public
  source knows. It never names the same title on two servers
  (section 4.1), and is shared only when the owner allows it
  (section 5).
- **A server should key a title under the first scheme of its part's
  list that can name it**: under TMDB whenever it can, under TVDB
  otherwise, under `local` only when no public source names it. Servers
  that do so pick the same key for the same title, where one exists.
  Where two servers still key one title under two schemes, `ids` join
  them (section 4.1).

- Keys are canonical: each content part gives one form for each title. A
  key in another form is a different string, and the receiver skips it.
- A receiver skips keys of a kind or a scheme it doesn't know, and keys
  of a content part that is off between the two servers (section 3.3).
- A title no scheme of its content part can name is not sent; nor is a
  `local` title while the owner doesn't share them (section 5).
- Each content part says which of its kinds `play` takes
  ([playback section 2.4](playback.md#24-what-the-content-parts-add)).

### 4.1 Same title, same key

Two titles are the same when their keys are equal, byte for byte. A
viewer's server uses this to tell that a peer's title is one it already
has, or to carry watched state across.

- **Across schemes**, a viewer's server may also match a title through
  its `ids`: other identifiers of the same title, by scheme, in the forms
  the [registry](../registry.md#ids-schemes) gives. One title may have
  two keys on two servers, `tmdb` on one and `tvdb` on the other; `ids`
  join them. A series keyed `series/tvdb/<n>` with `ids.tmdb` *m* and
  one keyed `series/tmdb/<m>` are the same show. `tmdb` is in `ids` so
  that a title keyed under another scheme can name its TMDB ID once TMDB
  has it. They are still two titles, each played from its own key.
- **Which titles carry `ids`.** On video, only episodes and titles keyed
  under `tvdb`; not films and series keyed under `tmdb`, whose other
  identifiers TMDB's lookup by key gives, nor `local` titles. On music,
  only `mb` titles: `mbreleasegroup` on albums and `mbrecording` on
  tracks. The content parts give the details
  ([video section 4.3](video.md#43-other-identifiers),
  [music section 3.1](music.md#31-other-identifiers)), the
  [registry](../registry.md#ids-schemes) the forms.
- **A `local` key never matches** another server's title, even an equal
  key: it is equal only to the same key from the same owning server. A
  viewer's server never matches a `local` title through `ids` either,
  and never compares `local` keys across peers.

### 4.2 Opaque parts of a key

Where a key holds an opaque part (the identifier of a `local` title,
an extra's part after `x/`), it has the form of a library ID: 1 to 64
characters of `A–Z a–z 0–9 _ -`, compared byte for byte. It follows the
rule of section 2.1: chosen by the owning server, it must not be,
contain, or be computable by a peer from, any identifier of the media
server, a file, a path or a name (a keyed MAC under a secret the server
keeps, or a stored random value, satisfies this). It is stable, so that
what a viewer's server keeps of the title follows it, and never reused
for another title. It may differ per peer.

Test cases: `opaqueKeyParts` in
[vectors/v0/strict.json](../../vectors/v0/strict.json).

### 4.3 Titles no public source knows: the description

A title no public source names has nothing a viewer's server could look
up. So a `local` title carries a small description, sent by the owning
server, beside the members of section 3:

| Field | Type | Content |
|-------|------|---------|
| `name` | string | its name, text, at most 128 characters |
| `year` | integer, optional | its year, 1800 to 9999 |
| `image` | boolean, optional | `true`: the owning server has one image of the title to give |

- `name` is required, except for the kinds a content part names by
  their place alone (video's seasons and numbered episodes, say); the
  content parts say which of their kinds carry it. A `local` title
  without one, where one is required, or whose `name` is empty once
  cleaned, is skipped.
- `name` is untrusted text, cleaned as [core section 9.1](core.md#91-pair)
  cleans a server's name (with 128 instead of 80), never markup. What
  protects it is the switch of section 5: `local` titles are off by
  default, shared per pairing and per group.
- `image` is asked for with the `image` call (section 4.4), and fetched
  under that answer's short ticket, no playback, as
  [media section 3.4](media.md#34-subtitles-fonts-sheets-and-images)
  says.
- A viewer's server shows the description as the owning server's, never
  as data from a public source, and never sends it on to another peer.
- `year` that isn't an integer in range, or `image` that isn't a
  boolean, counts as absent.

The description is defined here because both content parts use it. A
content part may give it to other titles of its own, with its own bounds
(music describes every title, and gives an artist with no public picture
an image this way). Outside what this section and the content parts
give, metadata beyond keys stays out of `titles`.

Test cases: `localTitles` in
[vectors/v0/video.json](../../vectors/v0/video.json) and
[vectors/v0/music.json](../../vectors/v0/music.json).

### 4.4 The image of a `local` title: `image`

`image`, under the peer's base path, signed as every call is. The
viewer's server asks for the one image of a `local` title whose
description says `image: true` (section 4.3): a film or a series of
[video](video.md), an artist or an album of [music](music.md). A server
answers `image` while a content part with `local` titles is on between
the two, for the titles of the parts that are on.

Request:

| Field | Type | Required | Content |
|-------|------|----------|---------|
| `title` | string | yes | the key of a `local` title, as the owning server sent it in `titles` |

Answer `200`:

| Field | Type | Content | What the viewer's server requires or does |
|-------|------|---------|-------------------------------------------|
| `ticket` | string | a ticket of the answer's own, made as every ticket is ([media section 1](media.md#1-the-urls)) | `^[A-Za-z0-9_-]{1,64}$`; otherwise the answer is refused, and there is no image |

- The answer carries no URL. The viewer's server builds it from the
  owning server's stream root and the ticket:
  `<stream root><ticket>/image` ([media section 1](media.md#1-the-urls)),
  so that it never fetches an address a peer wrote. The ticket works for
  10 minutes from the answer, for that URL only, and is no playback: it
  doesn't count against `streams` and gets no `progress` or `stop`.
- The owning server answers `404` `not-supported` when no content part
  with `local` titles is on between the two, checked before the body;
  `400` when the body isn't I-JSON, isn't an object, or its `title` is
  missing or isn't a string; `404` for any other `title`: one it doesn't
  offer the caller, one that isn't `local`, one of a part that is off,
  one of a kind without an image (only films, series, artists and albums
  have one), or one without an image; and may refuse with `429`
  `rate-limited` when asked too often
  ([core section 8.3](core.md#83-limits-and-overload)).
- The image is a JPEG or a PNG of at most 1 MiB (1,048,576 bytes; an
  image of exactly that size is accepted), 1 to 4,096 pixels wide and
  tall, as its header says. The viewer's server reads it
  whole, refuses it past 1 MiB, checks from its header, before decoding,
  that it is a JPEG or a PNG (by its content, whatever its
  `Content-Type`) within those sizes, and re-encodes it in the same
  format, so that only its pixels reach its people. It keeps the image,
  marked as the owner's, with the title, and asks again only when the
  title changes.
- A viewer's server asks for one image at a time per peer, and waits
  after a `429` as core section 8.3 says.

Test cases: `localImages` in
[vectors/v0/strict.json](../../vectors/v0/strict.json).

## 5. What is shared

The owning server decides alone. The viewer's server can't widen
anything.

- Only titles in libraries it offers that peer, and only those the peer
  asked for. `titles` ignores other library IDs.
- Only titles of the content parts on between the two servers
  ([core section 10](core.md#10-versions-and-parts)). A receiver skips
  the others anyway (section 3.3).
- A server offers only its own libraries, never what it got from another
  peer.
- **`local` titles** (section 4.3) are shared only when the owner allows
  it: off by default, set per pairing and per group, beside the offers.
  An owning server must not share them until its owner allows it.
  A link shares them when one of its relations allows it and offers the
  library that holds them ([groups section 9](groups.md#9-sharing-and-limits)).
  While it is off, the owning server leaves its `local` titles out of
  `titles`, as if it couldn't name them. Turning it on or off is a change
  of what it offers the caller: it forces a whole reading (section 3.2).
- A title its content part can't name under a public scheme, and that
  isn't shared as `local`, is not sent.
- Of the owning server, a peer never learns files, paths or its media
  server's item IDs, not even inside library IDs (section 2.1), version
  IDs ([playback section 2.3](playback.md#23-the-answer)), or the opaque
  parts of keys (section 4.2). Copies, tracks and extras carry no names
  ([video section 3](video.md#3-titles)).

## 6. Security considerations

- **A compromised peer** can make the other store a long title list, and
  look those titles up at the outside services the content parts name
  (TMDB, TheTVDB, MusicBrainz and the public sources of pictures).
  Readers cap what they keep (section 3.3) and should cap the total per
  peer.
- **Hostile answers.** Treat every name and key as untrusted input: clean
  names, parse keys strictly, never show text as markup.
- **Descriptions and images** of `local` titles are the owning server's
  words and pictures, reaching the viewer's people: a viewer's server
  cleans the name, checks an image's type and size before decoding it,
  decodes it with bounds (the [implementers' notes](implementers.md#decoding-safely)
  give advice), and serves its people a copy of its own, never
  the owning server's URL.
- **Copies** describe the owning server's files closely (sizes, tracks,
  lengths). They are what a viewer would learn by playing. They carry
  no names, so no file name or release group reaches a peer through
  them.
- **Library IDs** give a peer nothing about the owning server's layout
  (section 2.1). The same holds of every ID the parts define.

## 7. Open questions

Settled by the owner on 2026-11-17; each says how.

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
   [playback](playback.md#23-the-answer), the specification could state
   once that every identifier an owning server sends is opaque, and list
   them.
   **Settled:** yes, section 2.1 states it once and lists them.

The open points of proposal 5 (TMDB's numbering, IMDb episode IDs, the
cap of 16 for `with`) moved to [video](video.md#8-open-questions) with the
rules they concern.

**Choices this draft made, confirmed by the owner on 2026-11-17** (when the video rules were moved
out of this part on 2026-11-17):

4. **Language tags stay defined here** (section 3), since `play`'s
   `audioLanguage` uses them whatever the content part; `video` uses them
   in its copies' tracks.
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
13. **Schemes in a registry** ([spec/registry.md](../registry.md)): a
    new scheme is an entry there. `tmdb` joins `ids`, and `ids` join one
    title keyed under two schemes (section 4.1). The order of schemes is
    a "should".
14. **No product text here.** What this part said of media servers, and
    the JPEG DNL rule, moved to the
    [implementers' notes](implementers.md#catalog). The image's bounds
    are unchanged: a JPEG whose header gives a height of 0 is still
    refused, being under 1 pixel tall.

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
