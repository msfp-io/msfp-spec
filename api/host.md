# The host interfaces

**Informative, not part of the protocol.** What the host implements for
the engine. Notation: [README](README.md#notation). Each interface is a
`Config` field ([engine.md section 1](engine.md#1-opening-the-engine)).

| Interface | Required | What it is |
|-----------|----------|------------|
| [Store](#1-store) | yes | the engine's records. Two are shipped |
| [Catalog](#2-catalog) | to share | the host's libraries and titles, with their copies; `Images`, `Lyrics`, `ChangeLog` and `SeqBumper` optional |
| [Playback](#3-playback) | to let peers play | describe what the catalog doesn't say of a copy, start, report and stop a copy's stream |
| [Media](#4-media) | to let peers play | the bytes of a playback |
| [Sealer](#5-sealer) | yes, or the Store's own | seals key seeds and secrets |
| [Clock](#6-clock-dialer-notifier-sheet-encoder) | no | the system's by default; `Timers` optional |
| [Dialer](#6-clock-dialer-notifier-sheet-encoder) | no | the engine's checked one by default |
| [Notifier](#6-clock-dialer-notifier-sheet-encoder) | no | events; without it they are only logged |
| [SheetEncoder](#6-clock-dialer-notifier-sheet-encoder) | no | `JPEGSheets` by default |

Without Catalog, Playback and Media, the host offers nothing and every
`play` is answered `404`. It can still read and play its peers' titles.

The host may be a separate service beside its media server rather than
inside it ([core 1](../spec/v0/core.md#1-overview-and-terms)): it then
relays the media, and holds the narrowest credential of the media server
it can (a dedicated account limited to the offered libraries).

## 1. Store

Spec: [core 7.2](../spec/v0/core.md#72-what-each-side-stores),
[core 7.4](../spec/v0/core.md#74-ending-a-pairing),
[core 13](../spec/v0/core.md#13-changing-a-servers-address),
[listing 2.1](https://github.com/msfp-io/msfp-media/blob/main/spec/v0/listing.md#21-library-ids),
[listing 3.2](https://github.com/msfp-io/msfp-media/blob/main/spec/v0/listing.md#32-whole-list-or-changes).

```
Read(fn: func(Tx) error) error
Update(fn: func(Tx) error) error
```

- `Read` gives a consistent view. `Update` is atomic: everything `fn`
  wrote is kept, or nothing (when `fn` fails, or the commit does).
- Two `Update`s never interleave. One at a time is enough.
- The `Tx` is valid only inside `fn`. Getters return copies: a change
  counts only once put back.
- The engine owns the meaning of every field. The store keeps records
  and gives them back as they were. It may spread them over columns.
- Any error fails the engine's action; toward a peer, a signed `500`.

### 1.1 Tx

Every method returns an error besides what is listed.

| Method | Content |
|--------|---------|
| `Server() -> *ServerRecord` | the one server record; `nil` on the very first start |
| `PutServer(ServerRecord)` | |
| `Peers() -> []PeerRecord` | every peer, any state |
| `Peer(id) -> *PeerRecord`, `PeerByKey(key) -> *PeerRecord` | by current key only |
| `PutPeer(PeerRecord)` | adds or replaces |
| `DeletePeer(id)` | with its catalog, its sent keys, its leftovers and its downloads (`DownloadsTx`); an absent peer is no error |
| `Codes() -> []CodeRecord`, `CodeByHash(hash) -> *CodeRecord`, `PutCode`, `DeleteCode(id)` | pairing codes |
| `Forgotten(keyHash) -> *ForgottenKey`, `Forget(ForgottenKey)`, `Unforget(keyHash)`, `TrimForgotten(before)` | keys whose pairing ended, by SHA-256 of the key |
| `LibraryIDs() -> map[hostID]publicID`, `PutLibraryID(hostID, publicID)` | gone libraries included: a public ID is never reused |
| `Sent(peer, keys) -> []string`, `MarkSent(peer, keys)` | which title keys a peer was ever sent: a changes answer names in `gone` only those |
| `Leftovers()`, `PutLeftover`, `DeleteLeftover` | playbacks this server started on peers, to stop after a restart |
| `PutPeerTitles(peer, reading, titles)` | a peer's titles, added or replaced, marked with the reading; a whole reading's all at its end, in one `Update` |
| `EndReading(peer, reading)` | after a whole reading: drops titles not marked with it |
| `DropPeerTitles(peer, keys)` | a changes reading's `gone` |
| `PeerTitles(peer, after, limit) -> []Title`, `PeerTitle(peer, key) -> Title` | in key order |

The engine only writes peers' titles. The host reads them in its own
storage, as it likes (counts, joins, search).

**`GroupsTx`**, for the part `groups`: a `Tx` implements it to keep the
groups this server is in. Without it, the group actions fail with
`NoGroups` and peers' group calls are answered `404`. The shipped stores
do.

| Method | Content |
|--------|---------|
| `Groups() -> []GroupRecord` | every group |
| `Group(id) -> *GroupRecord` | `nil` for none |
| `PutGroup(GroupRecord)` | adds or replaces |
| `DeleteGroup(id)` | an absent group is no error |

**`DownloadsTx`**, optional: a `Tx` implements it to keep downloads for
their late reports across restarts
([playback 7](../spec/v0/playback.md#7-downloads)), of both sides.
Without it, downloads are kept in memory only, and a restart forgets
them. The shipped stores do.

| Method | Content |
|--------|---------|
| `Downloads() -> []DownloadRecord` | every download kept, of both sides |
| `PutDownload(DownloadRecord)` | adds or replaces one, by its `Viewer` and `ID` |
| `DeleteDownload(viewer, id)` | an absent one is no error |

**`OwnLogTx`**, optional: a `Tx` implements it when the engine keeps the
change log of the host's own catalog (a Catalog without `ChangeLog`).
The shipped stores do.

| Method | Content |
|--------|---------|
| `OwnSnapshot() -> map[key]Title`, `PutOwnSnapshot(map)` | the titles as the last refresh read them |
| `OwnChanges(since, after, limit) -> []OwnChange` | for each key after `after` changed after `since`, its latest change, in key order |
| `AppendOwnChanges([]OwnChange)` | an `OwnChange` is `{Seq, Key, Title, At}`; `Title` nil for gone |
| `TrimOwnChanges(before)` | drops changes noted before then |
| `OwnFloor() -> Int` | the last change number no longer kept (0: none dropped) |
| `OwnSeq() -> Int` | the highest number appended (0: none) |
| `BumpOwnSeq() -> Int` | a new number with no change: after a change of what a peer sees |

### 1.2 Records

The engine fills every field. Go spells them in capitals
(`ServerRecord.NextCommitment`); the JSON names are below.

**`ServerRecord`**, one:

| Field | Content |
|-------|---------|
| `current` | the current key's seed, sealed (`msfp current key`) |
| `next`, `nextCommitment` | the next key's seed, sealed (absent once exported and deleted); SHA-256 of its public key |
| `old`, `oldSince`, `oldRevoked` | the key before, while peers move from it, sealed (`msfp old key`); since when; whether it was revoked first |
| `revoked`, `revokedAt` | a key revoked with no next key, sealed (`msfp revoked key`), kept 30 days for former peers' hellos ([core 12.7](../spec/v0/core.md#127-what-to-do-on-compromise)) |
| `idsSecret` | the secret of opaque IDs, sealed (`msfp ids secret`); never changes |
| `address`, `former`, `movedAt`, `reached` | where this server is now; the address before and when it changed, while a move lasts; whether it reached itself at the new one (until then its hellos name the former) |
| `uncallable`, `uncallableAddress` | the operator's setting; the `https://msfp-<26>.invalid` address made for it, kept |

**`PeerRecord`**:

| Group | Fields |
|-------|--------|
| identity | `id`, `key`, `previousKey`, `previousUntil`, `name`, `created` |
| state | `state` (`ours`, `theirs`, `paired`, `gone`), `wasPaired`, `paired`, `seen`, `suspended` |
| address | `address`, `base` (from its discovery document; empty for the default), `move` (`address`, `since`, `tried`, `failingSince`, `warned`), `ourAddress` (our `host[:port]` its requests were last signed for), `inbound` (`yes`, `no`, `confirming`), `callsFailed` |
| versions | `versions` (its latest signed hello's), `pins` |
| its keys | `commitment`, `commitmentFrom` (`pair`, `rotation`, or empty), `pending` (a rotation waiting for our operator), `rotations` (statements kept), `refused` (`{key, at}`) |
| our keys toward it | `ourKey` (empty for current, `old`, `lost`), `rotationAnswer` (`pending`, `refused`, or empty), `rotateSent`, `commitmentTold` |
| sharing | `offers` (host library IDs), `offersSeq`, `local` (the individual relation shares local titles too), `wants` (`WantedLibrary`), `limits` (ours to it), `theirLimits`, `catalogRead` (`seq`, `wants`) |
| relations ([core 7.7](../spec/v0/core.md#77-relations)) | `groups` (the group relations: the groups the link was made or kept through), `notByHand` (this side doesn't want the individual relation), `theirIndividual` (what the peer last said of it in `hello`; absent: never said, counted as ours), `keepChoice` (our operator is choosing whether to keep the link, after a dissolution with `keepPairings`), `individualRefused` (our operator refused the peer's ask, privately), `groupOffers` (host library IDs its groups offer, together), `groupLocal` (the groups that share local titles with it), `pools` (`{group, limits}`, each group's pool), `joiningGroup` (a group whose code it redeemed, while it joins) |

**`CodeRecord`**: `id`, `secret` (sealed, `msfp code secret`),
`secretHash`, `note`, `created`, `expires`, `uses` (how many more
servers may use it), `group` (the group a group's code invites to).

**`GroupRecord`**, one per group ([groups 3.6](../spec/v0/groups.md#36-what-each-member-keeps)):

| Field | Content |
|-------|---------|
| `id`, `me` | the group's ID; this server's member ID in it |
| `via` | the inviter, until the list arrives from it |
| `events`, `held` | signed events kept (`{event, signature, received}`), counted or not: the state is their replay ([groups 3.5](../spec/v0/groups.md#35-rights)); `held` is no longer written (read as kept events) |
| `mail` | sealed requests and answers passed on, until they expire (`{id, mail, signature, expires}`) |
| `mailKey` | this server's X25519 key for the group's mail, sealed (`msfp group mail key`) |
| `accept`, `offers`, `local`, `pool` | how this server answers requests; host library IDs it offers the group; whether it shares local titles with it; the pool it lends |
| `hidden` | the members' libraries this server doesn't show from the group (`{member, library}`: a member ID, the library's public ID); it shows everything else they offer it |
| `requests` | members' requests waiting for the operator: `{mail, from, name, address, nextCommitment, inbound, pins, received}` |
| `refused`, `sent`, `used` | members whose requests were refused; requests and answers sent (`{id, to, kind, expires}`); mail IDs a `group-pair` used |
| `synced` | the last sync with the members |
| `joining` | when this server called `group-join` to join through `via`, a peer already (zero: it used `pair`) |
| `joins` | `paired` peers that used this server's code for the group, waiting for its operator: `{peer, received}` |

**`ForgottenKey`**: `keyHash`, `at`, `revoked` (the pairing ended with
a revocation without next key: for 30 days its hellos get the
revocation), `blocked` (the operator blocked it, `BlockPeer` or `Block`:
kept away from every group; other ended keys only get the signed
`404`).

**`Leftover`**: `peer`, `playbackId` (the peer's).

**`DownloadRecord`**: `id`, `viewer` (this server's download on a peer,
rather than a peer's here), `peer`, `theirId`, `title`, `queue`,
`handle`, `playedAt`, `lastAt`.

`msfp.MarshalTitle` and `UnmarshalTitle` write and read a `Title` as
JSON (`{"video":{…}}` or `{"music":{…}}`), for a store that keeps titles
as text.

### 1.3 The shipped stores

| Store | For | Notes |
|-------|-----|-------|
| `memory.New()` | tests, runs that needn't outlive the process | seals nothing (`PlainSealer`) |
| `jsonfile.Open(dir, Options{KeyFile})` | small hosts with no database | one process at a time (a lock on the directory); files mode 0600 |

`jsonfile` keeps `state.json` (server, peers, codes, forgotten keys,
library IDs, leftovers, groups, downloads), `peers/<id>.json` (a peer's titles and sent
keys), `own.json` (the change log) and `sealing.key` (the sealing key,
unless `KeyFile` puts it elsewhere). Each file is replaced whole and
atomically. A commit that touches several files is not atomic across
them: catalogs are written first, then `state.json`, so a crash leaves
at worst a catalog without its peer, removed at the next open.

Both stores bring their own Sealer (`SealerProvider`): `PlainSealer`
for memory, a `KeyFileSealer` on `sealing.key` for `jsonfile`.

## 2. Catalog

Spec: [catalog 2](../spec/v0/catalog.md#2-libraries),
[catalog 3](../spec/v0/catalog.md#3-titles),
[video 3](../spec/v0/video.md#3-titles),
[music 3](../spec/v0/music.md#3-what-a-title-carries).

The host's own libraries and titles, by its own library IDs. The engine
maps them to public IDs, filters them by offers, pages them, and checks
every key before it leaves (a key that isn't canonical is dropped, with
a warning in the log).

| Method | Content |
|--------|---------|
| `Libraries() -> []Library` | `{ID, Name}`: a library carries no kind; its titles' keys say what it holds |
| `Titles(libraries, after, limit) -> TitlePage` | titles held in any of these libraries, keys after `after`, in byte order, at most `limit`, each once; `TitlePage` is `{Titles, More}`. A change of anything a title carries, its copies included (a copy added, gone, or its `ETag` changed), is a change of the title |

- A `Title` is a `*VideoTitle` or a `*MusicTitle`
  ([engine.md section 10](engine.md#10-data-objects)). Its `Libraries`
  are the host libraries that hold it. The engine sends one public ID.
- A title its part can't name is keyed under the `local` scheme
  (`movie/local/<opaque>`, `series/local/<opaque>`), with a `Name` (and
  a `Year`, an `Image`) of the host's
  ([listing 4.3](https://github.com/msfp-io/msfp-media/blob/main/spec/v0/listing.md#43-titles-no-public-source-knows-the-description)).
  Extras are titles of their own, of kind `extra`, keyed under their
  film, series or season (`<key>/x/<opaque>`), with an `ExtraType`
  ([video 3.9](https://github.com/msfp-io/msfp-media/blob/main/spec/v0/video.md#39-extras)). Their opaque parts are
  made with `engine.OpaqueKeyPart(id)`
  ([listing 4.2](https://github.com/msfp-io/msfp-media/blob/main/spec/v0/listing.md#42-opaque-parts-of-a-key)). The
  operator shares local titles per pairing and per group (`SetLocal`,
  `SetGroupLocal`), off by default.
- An episode keyed by its TMDB episode ID is looked up at TMDB when the
  media server stores none: its numbers are often not TMDB's
  ([video 4.1](https://github.com/msfp-io/msfp-media/blob/main/spec/v0/video.md#41-episode-order)). The host fills its
  `Place` with TMDB's current season and episode numbers (found by the
  episode's IMDb or TheTVDB ID when the media server's numbers aren't
  TMDB's); an episode without one isn't sent.
- **Copies.** Films, episodes, extras and tracks carry their copies
  (`VideoTitle.Copies`, `wire.Copy`; `MusicTitle.Copies`,
  `wire.MusicCopy`), described as their files are
  ([video 3.2](https://github.com/msfp-io/msfp-media/blob/main/spec/v0/video.md#32-copies),
  [music 3.3](https://github.com/msfp-io/msfp-media/blob/main/spec/v0/music.md#33-a-tracks-copies)), so that a
  viewer's server lists them before anyone plays. Series and seasons
  carry none; a film, an episode or an extra without a copy isn't sent.
  Each copy's ID is the host's handle: it never leaves; the engine sends
  the version ID instead. The host's order is its preference, best
  first: the first copy is the one played when a peer names none
  ([video 3.7](https://github.com/msfp-io/msfp-media/blob/main/spec/v0/video.md#37-the-order-of-copies)). Facts are
  in outside vocabularies or the [registry](../spec/registry.md): the
  file's media type, the registry's codec and subtitle tokens, RFC 6381
  `Codecs` (required for h264, hevc, av1, vp9, aac and dts), H.273
  colour numbers. A copy carries no name and its tracks no title. Track
  IDs are `a<n>` and `s<n>`, n the stream's index in the file. The
  `ETag` changes when and only when the copy's bytes change. The
  previews' layout is on the copy (`PreviewLayout`). The package
  `describe` builds a copy from a file with ffprobe.
- Languages may be the media server's codes: the engine sends them as
  `wire.SentTag` gives them (`fre` and `fra` as `fr`).

**`Images`**, optional, on a Catalog that shares the images of its local
titles ([listing 4.4](https://github.com/msfp-io/msfp-media/blob/main/spec/v0/listing.md#44-the-image-of-a-local-title-image)):
`Image(key) -> Bytes`, one JPEG or PNG of at most 1 MiB, 1 to 4,096
pixels wide and tall; `NotFound` for none. The engine checks it before
it leaves.

**`Lyrics`**, optional, on a Catalog that shares its tracks' lyrics
([music 6](../spec/v0/music.md#6-lyrics)): `Lyrics(peer, copy) ->
[]wire.Lyrics`, at most 8 sets; `NotFound` for none. The engine cleans
them before they leave.

**`ChangeLog`**, optional, for a host whose database notes changes:

| Method | Content |
|--------|---------|
| `Seq() -> Int` | the change number now |
| `Changes(libraries, since, after, limit) -> ChangePage` | what changed after `since`: `{Present, Gone, Last, More}`; `TooOld` when the log no longer goes back that far (the engine then sends the whole list) |

Without it, the engine keeps the log itself in storage (`OwnLogTx`),
reading every title at each refresh. Without either, every reading is a
whole one, and the engine warns at `Open`.

**`SeqBumper`**, optional, on a `ChangeLog`: `BumpSeq() -> Int` takes a
new change number with no title changed. After a change of what a peer
sees (offers, a library removed, parts on), its readings are whole until
the log passes the number noted then. With `BumpSeq` the engine moves
the log at once; without it, readings stay whole until a title changes.

**Telling the engine:** `engine.CatalogChanged()` when titles changed
(a hint), `engine.LibraryRemoved(id)` when a library is gone.

## 3. Playback

Spec: [playback 2](../spec/v0/playback.md#2-play),
[playback 6](../spec/v0/playback.md#6-what-the-owning-server-enforces),
[video 5](../spec/v0/video.md#5-playing-a-film-an-episode-or-an-extra),
[music 5](../spec/v0/music.md#5-playing-a-track).

The owning side. The engine has checked the `play`, found the title's
copies as the catalog sends them, picked the copy and the audio (those
the request names, else the first copy and the file's default audio),
decided what is served (`wire.Serve`, `StartRequest.Served`), and refused
what MSFP refuses of the delivery asked and what the limits refuse
early.

| Method | Content |
|--------|---------|
| `Describe(peer, copy) -> CopyInfo` | what the catalog doesn't say of the copy, resolved within `peer.Libraries` |
| `Start(req: StartRequest) -> Started` | start serving; errors `NotPlayable`, `DeliveryRefused`, `ServerBusy`, `StreamLimitReached`, `NotFound` |
| `Progress(r: Report)` | a peer's `progress` or `stop`, passed on for the host's live view: `{Handle, Title, PositionMs, HasPosition, At, Stop, Late}`; `Late` for a download's late report, after its playback ended |
| `Stop(handle, reason: EndReason)` | the playback ended; once for each `Start` that succeeded |

**`PeerRef`** `{ID, Name, Libraries}`: the peer a call is made for (for
the live view; never a person), and the host's libraries offered to it
then, sorted. `Describe`, `Start` (`StartRequest.Peer`), `Lyrics` and
`Media.Serve` are given it, so that a handle can name the copy alone,
wherever its files are: the host resolves it to a file in one of those
libraries, and answers `NotFound` when the copy is in none.

**`CopyInfo`**: `Fonts` (how many fonts the copy's ASS tracks use, 0 to
64, served as fonts 0 to `Fonts`-1), `NotApart` (the subtitle tracks the
host can't serve apart: they get no entry in the answer's `subtitles`),
`Forms` (by track ID, `text` or `picture`: how the host serves apart a
track of a format [video 5.4](https://github.com/msfp-io/msfp-media/blob/main/spec/v0/video.md#54-choosing-the-subtitles)
doesn't list). Every other track of a format video 5.4 lists is served
apart, in its form.

**`StartRequest`**:

| Field | Content |
|-------|---------|
| `Copy`, `Title` | the copy's handle; the key played (an episode's, in a copy holding several) |
| `AudioOnly` | a music track: serve no video |
| `Purpose` | `play`, `download` or `scan` ([playback 7](../spec/v0/playback.md#7-downloads), [8](../spec/v0/playback.md#8-scan-access)); a scan is always of delivery `file` |
| `Delivery`, `MediaType`, `Transcode` | what the viewer's server asked: `file` (the copy as it is), `remux` (into `MediaType`'s segment format, as `hls`), `transcode` (to `Transcode`'s targets: registry tokens, `copy` for a stream left unchanged, a `Range` of `sdr` or `keep`), or empty for the baseline: H.264 High 4.1 8-bit SDR and AAC-LC stereo in complete HLS of MPEG-TS segments, each stream known to be within it left unchanged while the copy is within `MaxBitrateKbps` (above it, the video converted, or the audio of a copy with no video) ([playing 2.7](https://github.com/msfp-io/msfp-media/blob/main/spec/v0/playing.md#27-delivery)). With a delivery, serve exactly that or refuse with `DeliveryRefused`; never another |
| `Served` | what the engine decided is served of the copy: the method, `copy` or the codec each stream becomes, the segment format of a remux and of the baseline (`mp2t`) |
| `AudioRenditions` | for `hls`, every audio track of the copy as a rendition named by its ID; never refused (`Started.AudioRenditions` says whether it was made) |
| `Parts` | the copy's files, as its description says (1 for one file): several are started as `hls`, never `file` |
| `Peer` | `PeerRef`, above |
| `AudioTrack` | the audio played: the one asked, else the file's default. Subtitles never make a playback transcode, but for `Transcode.BurnSubtitle` |
| `StartMs` | where to start, in the copy; 0 for a download or a scan |
| `MaxBitrateKbps` | the cap in effect for a play; 0 for none. A download's pace is the engine's |
| `Queue` | for a music play, the queued tracks to prepare under the same playback: `QueuedCopy{Place, Title, Copy, Delivery, MediaType, Transcode, Served}` ([music 5.2](../spec/v0/music.md#52-the-queue)) |
| `Admit` | `func(method) -> (Slot, error)`, below |

The host's duties toward what it serves are in
[playing 2.7](https://github.com/msfp-io/msfp-media/blob/main/spec/v0/playing.md#27-delivery) and
[fetching 3.1](https://github.com/msfp-io/msfp-media/blob/main/spec/v0/fetching.md#31-hls-playlists): in fragmented MP4
segments, HEVC written `hvc1`, parameter sets out of band, and Dolby
Vision as playback 2.7 says (the base's sample entry when the base is
compatible, `dvh1` or `dav1` when it isn't); each variant of a master playlist with
`BANDWIDTH`, `CODECS` and, for video, `RESOLUTION`, plus `VIDEO-RANGE`
and `SUPPLEMENTAL-CODECS` where they apply. The engine fills those it
knows when the host leaves them out, and logs what stays missing.
`Config.Encoders` says what the host converts to.

**`Admit`.** The host chooses `direct`, `remux` or `transcode`, then
calls `Admit(method)` **before starting any work**. The engine counts the
pairing's streams and, for `transcode`, its transcodes, or the download,
or the scan, in one step
([playback 6.2](../spec/v0/playback.md#62-limits-at-once)). Over them:
`StreamLimitReached` or `ServerBusy`. With `Limits.Audio`, tracks are
counted apart. The host then checks its own budget, and calls
`Slot.Release()` if that refuses or its start fails.
`Slot.Transcoding() -> Bool` moves a remux that turned into a re-encode
into the transcode count (false: none left; the host stops it).

`Admit` is optional: a host that never calls it is counted after
`Start` returns, and the playback is stopped (`Stop`, `stopped`) when
the limits refuse it.

**`Started`**:

| Field | Content |
|-------|---------|
| `Handle` | the host's for this playback (not the copy's); given to `Progress`, `Stop`, `Media.Serve` |
| `Method` | as admitted |
| `Protocol` | `hls` or `file`; a file is served with the copy's own media type |
| `PositionMs` | where it starts |
| `AudioRenditions` | the playlist carries every audio track as a rendition, as asked |
| `Queue` | the queued tracks prepared (`StartedEntry{Place, Method, Protocol}`), served under the same handle with `MediaPath.Entry` their `Place` + 1 |

The engine then makes the playback ID and the ticket, and answers the
peer: the viewer's server builds every media URL from the ticket.

**Endings** (`EndReason`, [playback 6.3](../spec/v0/playback.md#63-how-a-playback-ends)):
`stopped`, `expired`, `tooLong`, `pairingEnded`, `notOffered`,
`operator`. A playback whose pairing ends while `Start` runs is stopped
at once as `pairingEnded`; one whose library is no longer offered then,
as `notOffered`. The engine cuts open media responses itself. Background
`Progress` and `Stop` calls are bounded to 30 seconds each.

## 4. Media

Spec: [media](../spec/v0/media.md), [video 6](../spec/v0/video.md#6-media).

```
Serve(peer: PeerRef, handle, path: MediaPath, rng: *Range, out: ResponseSink)
    errors: NotFound, RangeNotSatisfiable, ServerBusy
```

- The engine has checked the ticket, `GET` only (`HEAD` and other
  methods get `405` with `Allow: GET`), no query, no browser headers,
  the `Range` form, and the fetches at once. It cuts the response when
  the playback ends.
- `MediaPath` is `{Kind, Name, Track, Part, Whole, Sheet, Font, Entry,
  OfCopy}`, kind one of `master`, `playlist`, `segment`, `file`,
  `subtitles`, `ass`, `pictures`, `font`, `sheet`. `Whole` asks a whole
  text track rather than a stretch; `Entry` is 0 for the playback's own
  stream, n+1 for queue entry n's; `OfCopy` says the handle is a copy's,
  for a sheet under a previews ticket. Names are checked against
  `^[A-Za-z0-9._-]{1,64}$`, never `.` or `..`.
- For `file`, the engine has resolved the range against the copy's
  exact size (`rng` nil for the whole file, else `First` to `Last`, both
  within it), and writes the status and headers itself (`206` or `200`,
  `Content-Range`, `ETag`, `Accept-Ranges`): the host writes the copy's
  bytes from `First`, as they are. For `segment`, `Range{First, Last,
  Suffix}` (`Last` -1 for "to the end"): answer `206` with it, `200`
  with the whole, or `RangeNotSatisfiable`.
- `ResponseSink`: `Start(status, contentType, length, contentRange)`
  once (`length` -1 unknown, `contentRange` empty for none), then write
  the body.
- Send playlists complete (`VOD`, `#EXT-X-ENDLIST`), in media's subset
  ([fetching 3.1](https://github.com/msfp-io/msfp-media/blob/main/spec/v0/fetching.md#31-hls-playlists)), their audio
  renditions named by the copy's track IDs: the engine reads each one
  whole and refuses one outside it, with a warning. Subtitles, ASS,
  pictures and fonts are read whole and rebuilt or checked before they
  leave ([video 6](../spec/v0/video.md#6-media)). Sheets are JPEGs within
  the copy's layout; the engine re-encodes them.

## 5. Sealer

Spec: [core 12.5](../spec/v0/core.md#125-the-next-key-commitment).

```
Seal(plain, context) -> sealed       sync, may fail
Open(sealed, context) -> plain       sync; errors: Sealed
```

- Contexts: `msfp current key`, `msfp next key`, `msfp old key`,
  `msfp revoked key`, `msfp ids secret`, `msfp code secret`,
  `msfp group mail key`. A box never
  opens under another context. A revoked key sealed under `msfp old
  key`, as engines before 2026-11-18 did, is sealed again at `Open`
  (for one release).
- Default: the Store's own (`SealerProvider`). `Open` of the engine
  fails when there is neither.
- Shipped: `KeySealer(key)` (AES-256-GCM, 32-byte key, the context as
  additional data), `KeyFileSealer(path)` (its key in a file, made on
  first use, mode 0600; keep it apart from the data), and `PlainSealer`
  (seals nothing; tests and the memory store only).

## 6. Clock, Dialer, Notifier, sheet encoder

| Interface | Method | Notes |
|-----------|--------|-------|
| `Clock` | `Now() -> Time`, sync | tests set a fake one and call `RunOnce`. A clock that isn't the system's also implements `Timers` (`After(d) -> channel`), so that the engine's waits in a request (a download's pace) follow it |
| `Dialer` | `Dial(ctx, network, "host:port") -> net.Conn` | see below |
| `Notifier` | `Notify(Event)`, sync | in order, one at a time, outside the engine's locks; must return quickly |
| `SheetEncoder` | `Reencode(jpeg, maxWidth, maxHeight) -> Bytes`; errors `NotAnImage` | the viewer's side: the engine has read the sheet (at most 8 MiB) and checked its header; the encoder decodes and encodes again, so only pixels reach the viewer |

**Dialer** ([core 3.3](../spec/v0/core.md#33-connecting-to-peers)). The
default looks the name up once, refuses the destinations of core 3.3
(loopback only with `Config.AllowLoopback`), and connects to a checked
address, with no proxy. A host whose connections go out elsewhere (a
gateway) gives its own, which must make the same check, and wrap
`ErrRefusedDestination` when it refuses. The engine still does TLS,
pins, no redirects and the time bounds on top. The method is `Dial`, so
a plain `*net.Dialer` (whose method is `DialContext`), which checks
nothing, can't be given by mistake.

**Randomness** is `Config.Random` (`crypto/rand` by default): keys,
nonces, codes, tickets and IDs. Sealing boxes and the jitter of waits
use their own sources.
