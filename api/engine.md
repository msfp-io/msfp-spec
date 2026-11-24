# The engine API

**Informative, not part of the protocol.** What the host calls.
Notation: [README](README.md#notation). The host's side:
[host.md](host.md).

## Contents

1. [Opening the engine](#1-opening-the-engine)
2. [Settings](#2-settings)
3. [HTTP handlers](#3-http-handlers)
4. [The operator's actions](#4-the-operators-actions)
5. [Queries](#5-queries)
6. [Viewers](#6-viewers)
7. [Events](#7-events)
8. [Background work](#8-background-work)
9. [Errors](#9-errors)
10. [Data objects](#10-data-objects)
11. [An example](#11-an-example)

## 1. Opening the engine

```
Open(cfg: Config) -> *Engine
    errors: InvalidAddress, PlainHTTP, Sealed
```

- On the very first start, makes this server's key, its next key and
  its IDs secret, and stores them sealed.
- An address other than at the last start is a move (as `SetAddress`).
- `InvalidAddress` also for a bad `Prefix`, or an address of the
  `.invalid` form (the engine makes those). `Sealed`: the Sealer doesn't
  open what is stored.
- One engine per server, for the life of the process, safe from many
  goroutines.

**`Config`.** Only `Address` and `Store` are required, and `Address` may
be empty when the stored record says the server can't be called.

| Field | Default | Content |
|-------|---------|---------|
| `Address` | required | `https://host[:port]`, no path ([core 5.2](../spec/v0/core.md#52-addresses)). Kept while the server can't be called, for when it can again |
| `Store` | required | [host.md section 1](host.md#1-store) |
| `Name` | the host of the address | `func() string`, read at each `pair` and `hello` |
| `Sealer` | the Store's own | [host.md section 5](host.md#5-sealer) |
| `Prefix` | `""` | where the routes are mounted, before `/api/v1/federation/` and `/api/v1/stream/`; segments of `A-Z a-z 0-9 . _ ~ -`. A server with one needs `Discovery`, or peers can't find it ([core 11](../spec/v0/core.md#11-the-discovery-document)) |
| `Discovery` | off | serve `/.well-known/msfp` |
| `Pins` | nil: the engine reads them | this server's certificate pins ([core 3.2](../spec/v0/core.md#32-https-certificates)). nil: read by connecting to its own address (and the former one during a move). Give them all behind a proxy with several certificates |
| `AllowLoopback` | off | let the default Dialer reach peers on this machine |
| `PlainHTTPForTests` | off | accept and make `http` pairings ([core 3.1](../spec/v0/core.md#31-local-testing)). **Process-wide**: it turns on `wire.AllowPlainHTTPForTests`. Logged as a warning at each start |
| `Limits` | the spec's numbers | `CallLimits` ([core 8.3](../spec/v0/core.md#83-limits-and-overload)): per address 50/s burst 400; per peer 20/s burst 200; all peers 200/s burst 2,000; `play` per peer 0.5/s burst 20; at once 8 per peer, 64 in all, queue wait 5 s. A zero field takes its default |
| `NewPairing` | 2 streams, 1 transcode, no cap | the limits a new pairing starts with |
| `Clock`, `Random`, `Dialer`, `Notifier`, `SheetEncoder` | the system's, the engine's | [host.md section 6](host.md#6-clock-dialer-notifier-sheet-encoder) |
| `Logger` | `slog.Default()` | nothing secret is logged |
| `ClientAddress` | `WithClient`'s, else the connection's | `func(*http.Request) netip.Addr`: the caller's address as the host's trusted proxies say. The engine never reads `X-Forwarded-For` |
| `NotFound` | MSFP's `404` problem | the host's own answer for a path that doesn't exist, so the engine's unsigned refusals look the same ([core 3](../spec/v0/core.md#3-transport)) |
| `Parts` | nil: all six | `catalog`, `playback`, `media`, `video`, `music`, `groups`. A part without those it needs is left out (`video` and `music` need the other three; `playback` needs `catalog` and `media`; `groups` needs none, but its store must keep groups: [host.md section 1.1](host.md#11-tx)). An empty list: the core alone |
| `Catalog`, `Playback`, `Media` | absent | absent: nothing offered, every `play` answered `404` |
| `MaxBitrateKbps` | 0 (none) | this server's cap on what peers play here |
| `Encoders` | the baseline's: `h264` and `aac`, `sdr`, `wire.DefaultSegments` | what `Playback` makes ([playback 2.7](../spec/v0/playback.md#27-delivery)): the codecs it encodes (registry tokens), the ranges it makes (`sdr`, `keep`), and the codecs each HLS segment format (`mp2t`, `mp4`) carries. A peer's transcode to another codec or range, and a remux or a copied stream a segment format can't carry, are refused `409` `delivery-refused` |
| `LongestPlayback` | 24 hours | a peer's playback longer than this ends |
| `SyncEvery` | 30 minutes | how often peers' catalogs are read |

## 2. Settings

The engine lives as long as the process. Settings change through it,
never by opening a new one.

| Method | Content | Spec |
|--------|---------|------|
| `SetAddress(address)` | a move: the new address is tried at once; once this server reaches itself there, peers are told, and the former address is accepted 30 days or until every peer moved. While the server can't be called, only kept. Errors `InvalidAddress`, `PlainHTTP` | [core 13.1](../spec/v0/core.md#131-the-server-that-moves) |
| `SetUncallable(on)` | on: the address becomes the `.invalid` one, made at the first time and kept; live codes withdrawn; every peer sent a hello, then on core 7.5's schedule. Off: a move back to the address, announced once reached. Off without any address: `InvalidAddress` (`SetAddress` first) | [core 7.5](../spec/v0/core.md#75-a-peer-that-cant-be-called) |
| `SetDiscovery(on)` | **sync** | [core 11](../spec/v0/core.md#11-the-discovery-document) |

## 3. HTTP handlers

Spec: [core 6](../spec/v0/core.md#6-signing-requests-and-answers),
[core 8](../spec/v0/core.md#8-common-rules-for-every-call),
[media 2](../spec/v0/media.md#2-requests).

| Handler | Mount at | Serves |
|---------|----------|--------|
| `FederationHandler()` | `<Prefix>/api/v1/federation/` | `pair`, `hello`, `rotate`, `revoke`, `libraries`, `titles`, `image`, `play`, `progress`, `stop`, `previews`, `lyrics`, `group-sync`, `group-push`, `group-pair`, `group-join`; anything else is `Config.NotFound` |
| `StreamHandler(fallback)` | `<Prefix>/api/v1/stream/` | peers' tickets. A `GET` with a ticket the engine doesn't know (the host's own people's, or one ended) goes to `fallback`; nil: `Config.NotFound`. `HEAD` and other methods get `405` on every media URL form, whatever the ticket |
| `DiscoveryHandler()` | `/.well-known/msfp` at the root, whatever the prefix | `GET` and `HEAD` while on; otherwise `Config.NotFound` |

The handlers do everything: limits by address before the signature, the
signature, nonces, limits per peer and in all, the body cap, the call,
a signed answer or refusal, crashes as signed `500`s.

**Around them, the host must:**

- mount them on the raw path, never behind `http.StripPrefix`: the
  signature covers the path as sent;
- give the client address (`msfp.WithClient(r, addr)` or
  `Config.ClientAddress`);
- send the answers unchanged: no compression, no content coding;
- put no authentication in front: the caller is a server, proven by its
  signature.

## 4. The operator's actions

Each goes at once: a call to a peer isn't held by the back-off of
background calls, only by a peer's signed `retryAfter`.

**Pairing.** Spec: [core 5](../spec/v0/core.md#5-the-pairing-code),
[core 7](../spec/v0/core.md#7-pairing-step-by-step).

| Method | Result | Errors |
|--------|--------|--------|
| `MakeCode(note)` | `(text, CodeView)`: the text, shown once; works once, for 24 hours | `NoCodes` (this server can't be called) |
| `MakeCodeWith(CodeOptions)` | as `MakeCode`, with `Note`, `Uses` (1 to 100; 0: 1), `Lifetime` (a minute to 30 days; 0: 24 hours) and `Group` (a group's ID: the code invites to it, [groups 2](../spec/v0/groups.md#2-codes-for-a-group)) | `NoCodes`, `CodeOptions`; for a group, `NotFound`, `NoRight` (not its owner nor an admin), `NoGroups` |
| `Codes()` | live codes, soonest to expire first, without their text | |
| `CodeText(id)` | a live code's text again, to copy or send once more, built with this server's address and key now | `NotFound` (no such live code), `NoCodes` |
| `DeleteCode(id)` | | `NotFound` |
| `ReadCode(text)` | `CodePreview`, to show before redeeming | `CodeInvalid`, `CodeExpired`, `OwnCode`, `AlreadyPeer` (not for a group's code from a `paired` peer), `PlainHTTP` |
| `RedeemCode(text)` | `PeerView` in state `theirs`: fetches the discovery document, calls `pair`. A group's code: this server joins the group once the inviter accepts, then gets the list and asks every member ([groups 4](../spec/v0/groups.md#4-joining)). A group's code from a server already `paired`: `group-join` through the pairing instead of `pair`, and the peer's `PeerView` is returned, the group `Joining` until its operator accepts ([groups 8.4](../spec/v0/groups.md#84-group-join)) | those of `ReadCode`, `RefusedDestination`, `Unreachable`, `CodeRefused` |
| `Accept(peer)` | `PeerView`, `paired`; the peer told at once, or at its next hello | `NotFound`, `NotWaiting` |
| `Check(peer)` | `PeerView`: a `hello` now, its answer applied | `NotFound`, `Uncallable`, `Suspended`, `KeyLost`, `Unreachable`, `Unknown` (marked `gone`) |
| `End(peer)` | ends a link whatever its relations, or refuses a waiting one, in any state: deleted, its keys recorded for the signed `404` (not blocked: it may be linked again through a group), then told. Panels end relations instead (`EndIndividual`, `LeaveGroup`) and keep `End` for refusing a waiting server | `NotFound` |
| `BlockPeer(peer)` | as `End`, and its keys recorded blocked: it is kept away from every group ([core 7.4](../spec/v0/core.md#74-ending-a-pairing)). `Block` does it for a group's member | `NotFound` |
| `PanelOpen()` | **sync**: the panel lists peers now; each peer in `theirs` gets a `hello`, at most every 20 seconds, in the background | |

**Relations.** Spec: [core 7.7](../spec/v0/core.md#77-relations). A
link lasts while one relation holds it: the individual relation (both
sides want it; a plain code gives it, a group's code doesn't) or a group.
Each change is sent in a `hello` at once and told with `RelationsChanged`.

| Method | Result | Errors |
|--------|--------|--------|
| `AskIndividual(peer)` | `PeerView`: asks a peer linked through a group for the individual relation, or accepts its ask; also keeps a link a dissolution left to the operator (`KeepChoice`) | `NotFound`, `NotPaired` |
| `EndIndividual(peer)` | ends the individual relation, or withdraws this server's ask: the link stays while a group holds it, and ends otherwise, without keeping the peer away | `NotFound` |
| `RefuseIndividual(peer)` | refuses a peer's ask, privately (nothing sent), or ends a link a dissolution left to the operator; a link nothing else holds ends | `NotFound` |

**Sharing.** Spec: [catalog 2](../spec/v0/catalog.md#2-libraries),
[playback 5](../spec/v0/playback.md#5-limits).

| Method | Result | Errors |
|--------|--------|--------|
| `HostLibraries()` | `[]Library`: the host's libraries, as its Catalog gives them, to offer to a server or a group (also before a group exists) | |
| `SetOffers(peer, hostLibraries)` | what the individual relation offers, counted only while it holds; unknown IDs left out; the peer's next reading is whole; playbacks from libraries no longer offered end | `NotFound`, `NotPaired` |
| `TheirLibraries(peer)` | `[]WantedLibrary`: a `libraries` call now, kept | `NotFound`, `NotPaired`, `Uncallable`, `Suspended`, `NotSupported`, `Unreachable`, `BackingOff` |
| `SetWants(peer, publicIDs)` | what is read of the peer individually, counted only while the individual relation holds (for a server known only through groups, `SetGroupShows` counts); IDs it doesn't offer left out (an unseen one asks `TheirLibraries` first); a reading follows | `NotFound`, `NotPaired`, those of `TheirLibraries` |
| `SetLocal(peer, on)` | whether the individual relation shares the host's local titles too, in the libraries offered ([catalog 5](../spec/v0/catalog.md#5-what-is-shared)); off by default; the peer's next reading is whole | `NotFound`, `NotPaired` |
| `SetLimits(peer, Limits)` | from its next `play` on | `NotFound`, `InvalidLimits` |
| `ReadNow(peer)` | a reading now, waited for: titles kept | `NotFound`, `NotPaired`, `Uncallable`, `Suspended`, `NotSupported`, `Unreachable`, `BackingOff`, the peer's refusals |

**Keys.** Spec: [core 12](../spec/v0/core.md#12-key-rotation-and-revocation).

| Method | Result | Errors |
|--------|--------|--------|
| `NextKey()` | `(fingerprint, onServer)`: from the commitment ([core 4.2](../spec/v0/core.md#42-the-fingerprint)) | |
| `ExportNextKey()` | `(text, fingerprint)`; text `msfp-next-key-1.<seed>` | `NoNextKey` |
| `DeleteNextKey()` | the seed leaves the server, the commitment stays; `Rotate` and `KeyStolen` then need the text | `NoNextKey` |
| `Rotate(nextKey)` | to the next key (`""`: the seed here; or the exported text). Live codes withdrawn. Every callable peer sent `rotate`; returns when each answered or failed. A change still under way is finished first | `NoNextKey`, `NotNextKey`, `RotationUnderWay` (another key action is running now) |
| `KeyStolen(nextKey, withoutNext)` | `rotated`: revokes the key (every callable peer sent `revoke`), then rotates to the next key. With `withoutNext`: a new key, not a rotation; every pairing ends without `ended`; for 30 days former peers' hellos get the revocation ([core 12.7](../spec/v0/core.md#127-what-to-do-on-compromise)). Live codes withdrawn | `NoNextKey` (nothing revoked), `NotNextKey`, `RotationUnderWay` |
| `ConfirmRotation(peer)` | `PeerView`: the peer's pending key becomes its key, after the operators compared fingerprints; a hello follows | `NotFound`, `Suspended`, `NoPending`, `KeyTaken` |
| `DismissRefused(peer)` | clears a refused rotation's warning | `NotFound` |

`withoutNext` is never implied: without a next key and without it,
`KeyStolen` fails with `NoNextKey`.

**Groups.** Spec: [groups](../spec/v0/groups.md). Every action is signed as an event,
kept, and passed on to the members at once and at each sync (every 30
seconds, in `Run`). `id` is a group's ID, `member` a member ID (both
base64url, as the views give them). Each one may also fail with
`NoGroups` (the store keeps no groups) and `NotFound` (no such group).

| Method | Result | Errors |
|--------|--------|--------|
| `CreateGroup(name, GroupSettings)` | `GroupView`: a new group, this server its owner. `GroupSettings{WaitDays, CapPerDay}`, all zero: 3 and 5 joins a day; `WaitDays` is still sent and checked (0 to 30) but changes nothing ([groups 3.3](../spec/v0/groups.md#33-the-types), [4.1](../spec/v0/groups.md#41-answering)). Invite with `MakeCodeWith` | `CodeOptions` (an empty name; days or invitations out of range) |
| `SetGroupSettings(id, name, settings)` | the name (`""`: kept) and settings | `NoRight` (the owner only), `CodeOptions` (days 0 to 30, invitations 1 to 100 a day) |
| `SetAdmin(id, member, on)` | names an admin, or removes one | `NoRight` (the owner only), `NotMember` |
| `SetHeir(id, member)` | who takes the group when the owner steps down; `""`: none (the oldest admin, else the oldest member) | `NoRight`, `NotMember` |
| `GiveGroup(id, member)` | the group goes to `member`; `""`: to the successor | `NoRight` (also when nobody is left to take it), `NotMember` |
| `Ban(id, member, reason)` | the member is out, and told; its links through this group alone end on every member. `reason` optional | `NoRight` (an admin bans members only), `NotMember` |
| `Unban(id, member)` | it may be invited again | `NoRight` (the owner only) |
| `LeaveGroup(id, ask)` | this server leaves (or, while `Joining` through `group-join`, gives up); every link through it loses the group as a relation, and one with no relation left ends, but the peers in `ask`, which are asked for the individual relation first and stay until their operators answer ([groups 7](../spec/v0/groups.md#7-leaving-blocking-banning-dissolving), [core 7.7](../spec/v0/core.md#77-relations)) | |
| `DissolveGroup(id, keepPairings)` | every member removes the group; the links it alone held end, or, with `keepPairings`, wait for each side's operator to keep (`AskIndividual`) or end them (`RefuseIndividual`): `PeerView.KeepChoice` | `NoRight` (the owner only) |
| `Block(id, member, invitees)` | ends the link with `member`, whatever its sources, and forgets its key, so its requests are dropped in every group; with `invitees`, those it invited too, and theirs | `NotMember` |
| `SetGroupAnswer(id, accept)` | `true`: this server accepts each request itself, as soon as it arrives; `false` (the default): the operator answers each ([groups 4.1](../spec/v0/groups.md#41-answering)) | |
| `AcceptRequest(id, mail)` | `mail` from `GroupView.Requests`: the two pair through the group (`group-pair`, or an answer mailed when the requester calls instead), no fingerprint to compare | `NotFound` (no such request), the call's |
| `AcceptJoin(id, peer)` | a `paired` peer that used this server's code for the group (`GroupView.Joins`): `joined` signed, the list sent, the link gains the group | `NotFound` (no such peer waiting), `NotPaired`, `NoRight` |
| `RefuseJoin(id, peer)` | nothing is sent; the peer gives up on its side, or after 30 days | `NotFound` |
| `RefuseRequest(id, mail)` | nothing is sent; the member's requests are dropped until `LiftRefusal`. An unknown `mail`: nothing done | |
| `LiftRefusal(id, member)` | | |
| `RequestMember(id, member)` | sends this server's request to one member (a newcomer sends it to all by itself). Skipped, without error: a member already linked, or one whose request from us is still live | `NoWayIn` (this server has no pairing through the group, [groups 6](../spec/v0/groups.md#6-passing-the-list-on)), the sending's error |
| `SetGroupOffers(id, hostLibraries)` | what every link through the group is offered, added to what each link offers by hand; playbacks from libraries no longer offered end ([groups 9](../spec/v0/groups.md#9-sharing-and-limits)) | |
| `SetGroupLocal(id, on)` | whether this server shares its local titles with the group, in the libraries it offers it; off by default; a whole reading on every link through it | |
| `GroupOffers(id)` | `[]Library`, the host's, those offered to the group marked `Offered` (as `Offers` for a peer) | |
| `SetGroupShows(id, []HiddenLibrary)` | what the group shows here, one choice for the whole group: everything its members offer this server, but these (`{Member, Library}`: a member ID and its library's public ID); each member linked through it is read again soon | |
| `GroupShows(id)` | `GroupShowsView{Members}`: each member (`GroupMemberShows{Member, Name, Peer, Libraries}`, `Peer` empty when not linked) with its libraries offered here (`ShownLibrary{ID, Name, Shown}`) | |
| `GroupsCutBy(peer)` | `[]GroupRef{ID, Name}`: the groups that ending this pairing would leave with no way in, for the panel to say before it ends ([groups 6](../spec/v0/groups.md#6-passing-the-list-on)) | |
| `SetGroupPool(id, Limits)` | the pool this server lends the group: streams, transcodes and a bitrate cap shared by all its members; a playback counts against the first pool with room (the link's own limits first) | `InvalidLimits` |

**Catalog and playbacks here.**

| Method | Content | Errors |
|--------|---------|--------|
| `CatalogChanged()` | **sync**, a hint: titles changed | |
| `LibraryRemoved(hostID)` | offered to nobody from now; playbacks from it end; its public ID kept | |
| `OpaqueKeyPart(id)` | **sync**: an opaque part of a title key for one of the host's titles, a local title's or an extra's ([catalog 4.2](../spec/v0/catalog.md#42-opaque-parts-of-a-key)), made from the host's own ID of it under the IDs secret: 22 characters, stable, never computable by a peer | |
| `StopPlayback(id, why)` | **sync**: the host ends a peer's playback here: `operator` (also for `""`), `expired`, `tooLong`, `notOffered`. Take `id` and the host's handle from `Playbacks()` | `NotFound`, `InvalidReason` (`stopped` and `pairingEnded` are the engine's) |

## 5. Queries

They read storage and never call a peer.

| Method | Result |
|--------|--------|
| `Server()` | `ServerView` |
| `Peers()` | `[]PeerView`, every state, by name |
| `Peer(id)` | `PeerView`; `NotFound` |
| `Offers(peer)` | `[]Library`, those offered marked `Offered`; `NotFound` |
| `Wants(peer)` | `[]WantedLibrary` as last read; `NotFound` |
| `Playbacks()` | **sync**: `[]PlaybackView`, peers' playbacks here, oldest first |
| `Groups()` | `[]GroupView`, the groups this server is in; `NoGroups` |
| `Group(id)` | `GroupView`; `NotFound`, `NoGroups` |

**`ServerView`**:

| Field | Content |
|-------|---------|
| `Fingerprint` | the current key's |
| `Address`, `Named` | where it is now (the `.invalid` one while it can't be called); what its hellos name (the former while the new one isn't reached) |
| `Former`, `MovedAt`, `Waiting`, `PeersMoved`, `PeersToMove` | a move under way: from where, when, whether the new address doesn't answer yet (peers not told), how many of the peers not `gone` moved |
| `Uncallable`, `Discovery` | the settings |
| `NextKey`, `NextOnServer` | the committed next key's fingerprint, and whether its seed is here |
| `RotationSince`, `NotRotated` | a change of key under way: since when, and how many peers still know the old key |
| `RevokedAt` | a key revoked without next key (30 days of revocation answers); zero: none |
| `Core`, `Parts` | the versions spoken |

**`PeerView`**:

| Field | Content |
|-------|---------|
| `ID`, `Name`, `Address`, `Fingerprint` | the current key's fingerprint, for the operators to compare |
| `State` | `ours`, `theirs`, `paired`, `gone` |
| `Created`, `Paired`, `Seen`, `Suspended` | times; `Suspended` when it revoked its key (zero: it didn't) |
| `Core`, `Parts` | the versions on with it; `Core` -1: none in common |
| `Inbound`, `CallsFailed` | `yes`, `no`, `confirming`; its last hello answer said our calls fail (suggest "this server can't be called") |
| `NotMoved`, `MoveLate`, `Moving` | it still calls our former address; 3 days after we announced; its own new address being checked (`MoveView{Address, Since, FailingSince, Warn}`, `Warn` after a day) |
| `CommitmentFrom` | `pair`, `rotation`, or empty (its next rotation waits for the operator) |
| `Pending`, `Refused` | `KeyNotice{Fingerprint, At}`: a key waiting for `ConfirmRotation`; a refused rotation |
| `OurKey`, `RotationAnswer` | what it knows us by (`""` current, `old`, `lost`: pair again); its last answer to our `rotate` |
| `Limits`, `TheirLimits` | ours to it; its to us (nil until said) |
| `Offers`, `Wants`, `WantsGone` | counts; `Offers` counts only those by hand |
| `Local` | the individual relation shares local titles too (`SetLocal`) |
| `Reading` | how the last reading of its titles went: `LastReading{At, Failure, Refusal}`, `Failure` one of a closed set of codes (`unreachable`, `timeout`, `certificate-refused`, `refused-unsigned`, `refused`, `not-paired`, `suspended`, `uncallable`, `no-common-version`, `catalog-off`, `older-draft`, `answer-not-understood`, `internal`; empty when it read), `Refusal` a signed refusal's problem type; nil before any |
| `Groups` | its group relations: the groups the link was made or kept through |
| `Individual`, `IndividualAsked`, `IndividualAsks` | the individual relation holds (both sides want it); this side asks and the peer hasn't accepted; the peer asks, waiting for `AskIndividual` or `RefuseIndividual` ([core 7.7](../spec/v0/core.md#77-relations)) |
| `KeepChoice` | a dissolution left the link to this side's operator: keep (`AskIndividual`) or end (`RefuseIndividual`) |
| `ByHand` | `Individual`, under its former name |
| `JoiningGroup` | it used one of our codes for this group and waits for our operator (state `ours`): a request to join that group |

Not to be confused: `ServerView.NotRotated` counts peers still on our
**old key**; `PeerView.NotMoved` is a peer still on our **former
address**.

**`GroupView`**:

| Field | Content |
|-------|---------|
| `ID`, `Name`, `Settings` | |
| `Role`, `Me` | this server's role (`owner`, `admin`, `member`) and member ID |
| `Owner`, `Heir` | member IDs; `Heir` empty for none |
| `Members` | `GroupMemberView{ID, Name, Fingerprint, Role, InvitedBy, Joined, Peer, Refused, Asked}`; `Peer` is the link with it (empty: none); `Refused`: this server refused its requests (`LiftRefusal`); `Asked`: this server's request to it waits |
| `Banned` | `GroupBanView{ID, Name, Fingerprint, Reason}`, the name and fingerprint as it was a member |
| `Requests` | `GroupRequestView{Mail, From, Name, Fingerprint, Received, Waited}`, waiting for this server's operator; `Waited`: the requester is a member of the group |
| `Accept`, `Offers`, `Local`, `Pool` | this server's answer setting, host library IDs offered, whether it shares local titles with the group, pool lent |
| `Dissolved` | the owner dissolved it (kept until left) |
| `Joining` | the list hasn't come from the inviter yet |
| `Joins` | `GroupJoinView{Peer, Name, Fingerprint, Received}`: `paired` peers that used this server's code for the group, waiting for `AcceptJoin` or `RefuseJoin` |
| `NoWayIn` | this server has no pairing through the group, and the group has other members: nothing reaches them from here, nor here from them ([groups 6](../spec/v0/groups.md#6-passing-the-list-on)) |

**`PlaybackView`**: `ID`, `Peer`, `PeerName`, `Title`, `Method`,
`Handle` (the host's), `Since`, `State` (`started`, or `active` once
used).

## 6. Viewers

This server's people play a peer's title. The host decides who may and
serves its own browser route; the engine talks to the peer and checks
everything it relays.

### 6.1 Playing

Spec: [playback 2](../spec/v0/playback.md#2-play),
[playback 3](../spec/v0/playback.md#3-progress),
[video 5](../spec/v0/video.md#5-playing-a-film-an-episode-or-an-extra).

```
Play(peer, req: PlayRequest) -> RemotePlayback
    errors: NotFound, NotPaired, Uncallable, Suspended, KeyLost,
            NotSupported, RefusedDestination, CertificateRefused,
            StreamLimitReached, ServerBusy, BackingOff, NotPlayable,
            DeliveryRefused, InvalidRequest
Progress(id, positionMs, paused)          sync
PlayingTrack(id, title)                   sync; errors: NotFound, InvalidRequest
Stop(id, positionMs)                      errors: NotFound
Restart(old: RemotePlayback) -> RemotePlayback
    errors: those of Play, CopyChanged, InvalidRequest
ReportOffline(id, OfflineReport)          errors: NotFound, InvalidRequest, PlaybackStopped, the call's
```

**`PlayRequest`**:

| Field | Content |
|-------|---------|
| `Title` | the key, as the peer listed it |
| `Purpose` | `play` (also `""`), `download` (of delivery `file` or a sized transcode, when the peer's limits allow downloads) or `scan` (of delivery `file`, when they allow scans: for the host's media server to list a file, never to watch it) ([playback 7](../spec/v0/playback.md#7-downloads), [8](../spec/v0/playback.md#8-scan-access)) |
| `Delivery`, `MediaType`, `Transcode` | `file`, `remux` (into `MediaType`: `video/mp2t` or `video/mp4`, `audio/…` for a copy with no video), `transcode` (to `Transcode`'s targets: registry tokens or `copy`, the range `sdr` or `keep`), or `""` for the baseline every viewer's server plays ([playback 2.7](../spec/v0/playback.md#27-delivery)). The host chooses from the copy's facts and its device: a copy whose media type, codec or `threeD` it doesn't know, or whose `codecs` it can't read (`wire.PlayableAsIs`, `wire.ReadCodecs`), it can't play as it is. On `DeliveryRefused`, ask again with another delivery, or none |
| `AudioRenditions` | for `hls`, every audio track as a rendition, so that switching tracks needs no new playback; the peer may make none |
| `StartMs`, `MaxBitrateKbps` | where to start; the viewer's cap (0: none) |
| `VersionID`, `AudioTrack` | the copy (without it, the peer's first) and its audio (without it, the file's default), from the title's copies kept. Subtitles are the host's choice, among the copy's tracks and those served apart; one is named only to burn it in (`Transcode.BurnSubtitle`) |
| `Queue` | for a track, at most 5 tracks to play after it, each with its own copy and delivery if it names them (`QueueEntry`, [music 5.2](../spec/v0/music.md#52-the-queue)) |
| `Replaces` | a playback of the same viewer to stop first; its `stop` waited for up to 5 seconds |

- The engine refuses before calling when the part is off, the peer
  can't be used, or its last limits say no stream.
- **Busy now**: `StreamLimitReached`, `ServerBusy` (a `503`, or no
  answer, or an unsigned one), `BackingOff` (a `429`, or calls
  unanswered lately). **Not playable**: `NotPlayable` and the rest
  ([playback 2.5](../spec/v0/playback.md#25-refusals)). What lasts
  (`KeyLost`, `Suspended`, `Uncallable`, `NotSupported`, a destination
  or certificate refused) comes as itself, never as `ServerBusy`.
- An answer that breaks a "must": the engine sends `stop`, then fails
  with `NotPlayable`.
- **`RemotePlayback`**: `ID` (the engine's, not the peer's), `Peer`,
  `Title`, `Purpose`, `Delivery`, `Method`, `Protocol`, `MimeType` (for
  a file, the copy's media type from the catalog), `Etag`, `Size` (when
  the catalog's copy has that etag; -1 otherwise), `CopyChanged` (the
  answer's etag isn't the catalog's: the title is read again),
  `PositionMs`, `DurationMs` (the copy's, from the catalog),
  `MaxBitrateKbps`, `ScanBytes` (a scan's cap), `VersionID`,
  `AudioTrack`, `AudioRenditions`, `Subtitles` (the tracks served apart,
  `wire.ServedSubtitle`, fetched in stretches of `wire.StretchSeconds` or
  whole), `Fonts`, `Sheets` (how many preview sheets the ticket opens),
  `Queue` (`RemoteEntry{Place, Title, VersionID, Method, Protocol,
  MimeType, DurationMs}`, fetched with `MediaPath.Entry` their `Place` +
  1). The copy's tracks are in the catalog. No URL: the host asks for
  each piece with `Fetch`, and the engine builds every URL from the
  peer's stream root and the ticket.
- Kept as a leftover until stopped, so a restart still stops it.

**`Progress`** returns at once. The peer is told at the first report,
at once on a pause or resume, otherwise at most every 45 seconds while
the host reports (`Run` sends the latest when no report falls on the
time). While a viewer is on a playback, paused too, call it at least
every minute: the peer ends a playback it hears nothing of for 2
minutes. A scan sends none. A `410`, the pairing ending, or the peer
suspended or uncallable ends the playback: `RemotePlaybackEnded`, then
fetches fail.

**`PlayingTrack`** names the track of a playback's queue that plays now;
`progress` and `stop` carry it, positions then in that track.

**`Stop`** sends `stop` (again a few times without an answer) and
forgets the playback. `positionMs` -1: not known.

**`Restart`** starts a new playback of the copy a playback of delivery
`file` was reading, once it ended (a long pause), so that the host reads
on at the same byte: the same title, copy, purpose and delivery, only
if the new answer's etag is the one being read; otherwise
`CopyChanged`, and the title must be read again.

**`ReportOffline`** sends a download's late report
(`OfflineReport{PositionMs, At, Title, Stop}`) when the viewer's device
syncs, for 30 days after its play
([playback 7](../spec/v0/playback.md#7-downloads)), in the order of
their `At`.

### 6.2 Relaying media

Spec: [media 2](../spec/v0/media.md#2-requests),
[media 3](../spec/v0/media.md#3-what-is-fetched),
[video 6](../spec/v0/video.md#6-media).

```
Fetch(id, path: MediaPath, rng: String, out: ResponseSink)
    errors: NotFound, RangeNotSatisfiable, ServerBusy
```

- `path`: the same `MediaPath` as on the owning side
  ([host.md section 4](host.md#4-media)). `msfp.ParseMediaPath(rest)`
  reads it from what follows the host's own ID in its route URL. A
  playlist's bare names then resolve under the route, so playlists pass
  unrewritten.
- `rng`: the viewer's `Range` header as received (`""`: none), sent on
  for the file and segments once well-formed. Malformed:
  `RangeNotSatisfiable`, nothing fetched.
- The engine fetches with the ticket over a connection that needs a
  *trusted* or *named* certificate (refused: a `hello`, then once more),
  and checks status, `Content-Range`, `Content-Length`, sizes, time,
  playlists, WebVTT (rebuilt), PGS (checked whole), sheets
  (re-encoded). It sets `Content-Type`.
- Any failure is `NotFound`, and nothing is written: the peer's body and
  headers never pass. The peer's `503` and the bounds of fetches at once
  are `ServerBusy`; its `416`, `RangeNotSatisfiable`.
- A fetch is cut when its playback ends.
- `msfp.NewHTTPSink(ctx, w)` is a `ResponseSink` on an
  `http.ResponseWriter`: only `Content-Type`, `Content-Length`,
  `Content-Range`, `Accept-Ranges`, `ETag`, no content coding.
  `msfp.ParseRange(h)` reads a `Range` header by media 2.1.

### 6.3 Seeing what peers hold

The engine reads each wanted peer's titles on its schedule and stores
them ([host.md section 1.1](host.md#11-tx)). The host reads them there.
Before offering a peer's title:

```
Playable(peer, kind) -> Bool          errors: NotFound, the store's
```

`true` when the peer is paired, not suspended, can be called, and
`playback`, `media` and the kind's part (`video`, or `music` for
`artist`, `album`, `track`) are on with it. It reads storage: a store
that fails gives its error, never a plain `false`.

### 6.4 Previews, lyrics and images

These ask a peer for what a title kept here says it has. Each may fail
with `NotFound` (no such title or copy kept, none with it, or the peer's
refusal), `NotPaired`, `Uncallable`, `Suspended`, `NotSupported`,
`Unreachable` (an answer refused), `RateLimited` and `BackingOff`.

| Method | Content | Spec |
|--------|---------|------|
| `PreviewSheet(peer, key, versionID, n) -> Bytes` | seek-bar preview sheet `n` of a copy, by the catalog's layout, re-encoded. Only for a copy someone opened, never for a whole library. During a live playback of the copy its ticket opens the sheets; otherwise one `previews` call per copy, its ticket shared by every viewer until it expires. Sheets are kept in memory (64 MiB) while the copy's etag and layout hold. Also `CopyChanged`, `ServerBusy` | [video 5.5](../spec/v0/video.md#55-seek-bar-previews), [5.6](../spec/v0/video.md#56-previews) |
| `TrackLyrics(peer, key, versionID) -> []wire.Lyrics` | a track's copy's lyrics, cleaned; the host keeps them while the copy's etag holds | [music 6](../spec/v0/music.md#6-lyrics) |
| `TitleImage(peer, key) -> (Bytes, format)` | a local title's image, checked and re-encoded (`wire.ImageJPEG` or `wire.ImagePNG`); the host keeps it, shown as the owner's, and asks again only when the title changes. One at a time per peer | [catalog 4.4](../spec/v0/catalog.md#44-the-image-of-a-local-title-image) |

## 7. Events

`Notify(Event)` ([host.md section 6](host.md#6-clock-dialer-notifier-sheet-encoder)).
Each event has `PeerID()` (`""` for an event of this server, and for
group events: a group's members aren't peers until linked). Every peer
event also carries `Name`. Links made through a group tell `PeerPaired`
and `PeerEnded` as any other. The state an event speaks of is in the
queries too, so a host that missed one loses nothing.

| Event | Fields | When | Spec |
|-------|--------|------|------|
| `PeerWaiting` | `Fingerprint` | a peer redeemed our code: the operator compares, then `Accept` or `End` | [core 7.1](../spec/v0/core.md#71-the-steps) |
| `PeerPaired` | | a pairing became `paired`, either side | core 7.1 |
| `PeerGone` | | a peer said, signed, it doesn't know us; or 30 days suspended without a valid rotation | [core 7.3](../spec/v0/core.md#73-states) |
| `RelationsChanged` | | why this server and a peer are linked changed: the individual relation asked, accepted, refused or ended, by either side | [core 7.7](../spec/v0/core.md#77-relations) |
| `PeerEnded` | `ByThem` | a pairing ended, by us (`End`) or by them | [core 7.4](../spec/v0/core.md#74-ending-a-pairing) |
| `VersionsChanged` | | a peer's versions changed | [core 10](../spec/v0/core.md#10-versions-and-parts) |
| `PeerSuspended` | | a peer revoked its key: warn, hide what needs it from viewers | [core 12.6](../spec/v0/core.md#126-revocation) |
| `PeerResumed` | | a suspended peer rotated to its committed key; `PeerRotated` comes too | core 12.6 |
| `RotationPending` | `Fingerprint` | a peer moved to a key it hadn't committed to: `ConfirmRotation` or `End` | [core 12.3](../spec/v0/core.md#123-receiving-a-rotation) |
| `RotationRefused` | `Fingerprint` | a peer's rotation didn't match its commitment; a warning | core 12.3 |
| `PeerRotated` | `Fingerprint` | a peer's new key is current | core 12.3 |
| `RotationAnswered` | `State` | a peer answered our change of key: `rotated`, `pending`, `refused`; also when a hello under the new key shows it moved | [core 12.4](../spec/v0/core.md#124-what-each-side-does-during-the-change) |
| `OurKeyLost` | | our change of key ended and this peer never moved: pair again | core 12.4 |
| `RotationEnded` | `Lost` | our change of key is over, the old key deleted (server event) | core 12.4 |
| `AddressAnnounced` | `Address` | we reached ourselves at our new address and told peers (server event) | [core 13.1](../spec/v0/core.md#131-the-server-that-moves) |
| `NotMovedLate` | | a peer still calls our former address 3 days after the announcement | core 13.1 |
| `MoveEnded` | `NotMoved` | our move is over: all moved (0), or 30 days (server event) | core 13.1 |
| `PeerMoved` | `From`, `To` | a peer's new address was confirmed (or read from an uncallable peer's hello) | [core 13.2](../spec/v0/core.md#132-a-peer-that-learns-of-a-move) |
| `PeerMoveFailing` | `Address`, `Since` | a peer's new address has failed for a day; once per move | core 13.2 |
| `InboundChanged` | `Inbound` | a peer can't be called (`no`), can again (`confirming`: its address checked first), or its address answered (`yes`) | [core 7.5](../spec/v0/core.md#75-a-peer-that-cant-be-called) |
| `CallsFailed` | | a peer's hello answer said our calls fail: suggest "this server can't be called" | core 7.5 |
| `CatalogReadEnded` | `Titles`, `Changes`, `Err`, `Failure`, `Refusal` | a reading ended: titles kept (new or changed, for changes), or why it failed (what was kept stays: a whole reading is kept only once read to its end); `Failure` and `Refusal` as in `PeerView.Reading` | [catalog 3.2](../spec/v0/catalog.md#32-whole-list-or-changes) |
| `WantsGone` | `Libraries` | wanted libraries no longer offered; their titles go at the next reading | [catalog 2.2](../spec/v0/catalog.md#22-what-the-caller-keeps) |
| `PlaybackStarted` | `Playback`, `Title`, `Method`, `Purpose` | a peer's playback here started | [playback 6.1](../spec/v0/playback.md#61-a-playbacks-states) |
| `PlaybackEnded` | `Playback`, `Title`, `Reason` | it ended, and why | [playback 6.3](../spec/v0/playback.md#63-how-a-playback-ends) |
| `RemotePlaybackEnded` | `Playback`, `Reason` | our playback on a peer ended without `Stop`: `410` (`expired`), the pairing ended, the peer suspended or uncallable (`pairingEnded`) | playback 6.3 |
| `GroupRequestWaiting` | `Group`, `GroupName`, `Mail`, `Member`, `Name`, `Fingerprint` | a member's request waits for the operator: `AcceptRequest` or `RefuseRequest` with `Mail`. Not for one the server accepts itself | [groups 4.1](../spec/v0/groups.md#41-answering) |
| `GroupJoinWaiting` | `Group`, `GroupName`, `Peer`, `Name`, `Fingerprint` | a `paired` peer used this server's code for a group: `AcceptJoin` or `RefuseJoin`. Its `PeerID()` is the peer | [groups 8.4](../spec/v0/groups.md#84-group-join) |
| `GroupJoined` | `Group`, `GroupName` | the list arrived and names this server; its requests are sent | [groups 4](../spec/v0/groups.md#4-joining) |
| `GroupChanged` | `Group`, `GroupName`, `Types` | members' events changed a group; `Types`: their types, once each (`joined`, `admin`, `owner`, `banned`…). Not for this server's own actions | [groups 3.3](../spec/v0/groups.md#33-the-types) |
| `GroupLeft` | `Group`, `GroupName`, `Why`, `Reason` | this server is out and removed the group: `left`, `dissolved` (by its owner, maybe this server), `banned` with the ban's reason | [groups 7](../spec/v0/groups.md#7-leaving-blocking-banning-dissolving) |

## 8. Background work

```
Run()        until the context ends; returns its error
RunOnce()    everything due now, the hourly work included, waited for
```

| Every second | Every hour |
|--------------|------------|
| our new address tried until it answers, then announced | expired codes deleted; forgotten keys over a year dropped |
| the scheduled hellos of a server that can't be called | keys: our change of key ended after 30 days, `rotate` sent again daily, a revoked key's 30 days, peers suspended 30 days made `gone` |
| peers' catalogs read when due: a minute after start, then every `SyncEvery`, at once when wants change | addresses: our move ended, late peers told again, peers' new addresses tried again |
| groups synced with their members, every 30 seconds; with `SetGroupAnswer(true)`, requests still waiting accepted | `group-join`s unanswered for 30 days dropped, on both sides |
| peers' playbacks here expired: no `progress` and no fetch for 2 minutes (looked at every 30 seconds), or over `LongestPlayback` | leftovers stopped (once; never a playback started since) |
| `progress` due to peers for our viewers | the engine's own change log refreshed |

- The hour is `Config.Clock`'s (a clock set back runs the hourly work
  again); the ticks of a second are real time.
- `RunOnce` is for tests with a fake clock, and for hosts that schedule
  work themselves (every second for a server that can't be called,
  every few minutes otherwise). Call one or the other, never both.
- Without either, the engine answers peers and the operator, but
  nothing happens with time.

## 9. Errors

One vocabulary for every library. Go: `msfp.ErrNotFound` and so on,
each a `*msfp.Error{Code, Err}`; `Err` holds detail for the logs (never
a secret, never sent). Match with `errors.Is`.

**A peer's refusals**, for calls the engine makes
([core 8.2](../spec/v0/core.md#82-refusals)):

| Code | From | Means |
|------|------|-------|
| `NotFound` | a signed `404` `not-found` | no such title, or not paired yet (a `hello` follows) |
| `Unknown` | a signed `404` to `hello` | it doesn't know us: marked `gone` |
| `NotSupported` | a signed `404` `not-supported` | the part is off with it |
| `InvalidRequest` | `400` | a bug on our side |
| `AlreadyPeer` | `409` to `pair` | we already are its peer |
| `RotationRefused` | `409` `rotation-refused` | |
| `StreamLimitReached` | `409`, another type than `not-playable` and `delivery-refused` | busy now |
| `NotPlayable` | `409` `not-playable`, any other refusal of `play` | |
| `DeliveryRefused` | `409` `delivery-refused` | the owning server won't serve that delivery of the copy: ask again with another, or none |
| `PlaybackStopped` | `410` | the playback is over |
| `RateLimited` | `429` | calls held until `retryAfter` |
| `ServerBusy` | `503` | busy now |
| `PeerFailed` | `500` | |
| `Unreachable` | no answer, unsigned or badly signed, a refused certificate; an unsigned `404` only once tried again a second later ([core 6.5](../spec/v0/core.md#65-nonces-replay-and-restarts)) | |

**The engine's own:**

| Code | Means |
|------|-------|
| `BackingOff` | the peer asked to wait, or didn't answer lately |
| `Uncallable`, `Suspended`, `KeyLost` | the peer can't be called; revoked its key; never moved to our new key |
| `NotPaired`, `NotWaiting` | not in the state the action needs |
| `CodeInvalid`, `CodeExpired`, `OwnCode`, `CodeRefused`, `NoCodes` | pairing codes |
| `NoNextKey`, `NotNextKey`, `NoPending`, `KeyTaken`, `RotationUnderWay` | keys |
| `InvalidAddress`, `PlainHTTP`, `RefusedDestination`, `CertificateRefused`, `MoveFailed` | addresses and connections |
| `InvalidLimits` | limits out of range ([section 10](#10-data-objects)) |
| `CopyChanged` | a restart's or previews' copy isn't the one kept any more (another etag): read the title again |
| `CodeOptions` | a code's uses or lifetime out of range; a group's name empty or too long |
| `NoGroups`, `NoRight`, `NotMember` | the store keeps no groups; this server's role doesn't allow it; not a member (or the group is gone) |
| `NoWayIn` | this server has no pairing through the group, so a mail it sends can't leave |
| `AlreadyMember` | a group's code from a peer: this server is already in the group, banned from it, or waiting to join it |
| `InvalidReason` | `StopPlayback` with an ending that isn't the host's |
| `RangeNotSatisfiable` | a malformed `Range`, or out of the file |
| `Sealed` | a sealed value didn't open |
| `TooOld` | a `ChangeLog` no longer goes back that far |
| `NotAnImage` | a sheet that isn't a JPEG within its size |
| `Internal` | anything else; logged, never shown to a peer |

**The host's errors**, and what the peer gets: `NotPlayable` `409`
`not-playable`; `DeliveryRefused` `409` `delivery-refused`; `StreamLimitReached` `409` `stream-limit-reached`;
`ServerBusy` `503`; `NotFound` `404`; from Media, `RangeNotSatisfiable`
`416`; anything else a signed `500`, logged.

## 10. Data objects

Those not given with the methods above.

**`Title`**: a `*VideoTitle` or a `*MusicTitle`; `Common()` gives
`TitleCommon{Key, Kind, Libraries}` (the host's IDs from Catalog; for a
peer's title, the one public ID it was listed in).

| Type | Fields |
|------|--------|
| `VideoTitle` | `Copies` (`wire.Copy`; of a peer's title, each ID is its version ID), `Place` (`Season`, `Episode`: TMDB's current numbers; required for an episode keyed by its TMDB episode ID, which isn't sent without it; a peer's is always set for those), `With` (episode keys), `IDs` (`imdb`, `tvdb`…; sent only on episodes and on titles keyed under `tvdb`), `ExtraType` (an extra's), `Name`, `Year`, `Image` (a local film's or series' description). `BestCopy`, `AudioLanguages` and `SubtitleLanguages` work out from the copies what a title list shows |
| `MusicTitle` | `Name`, `Artists`, `ArtistNames`, `PrimaryType`, `SecondaryTypes`, `Compilation`, `Date`, `OriginalDate`, `Genres`, `Album`, `Disc`, `Number`, `Copies` (`wire.MusicCopy`), `Image` (a local artist's or album's), `IDs`. A track's artists, names, dates and genres are sent only when they differ from its album's: of a peer's, an absent one is its album's |

**`Limits`**: `Streams` (0 to 1,000), `Transcodes` (0 to streams),
`BitrateKbps` (0: none, up to 1,000,000), `Downloads` (at once, 0 to
1,000; 0: none), `DownloadsPerDay` (0 to 10,000; 0: no cap besides
`Downloads`), `DownloadKbps` (each download's speed cap; 0: none),
`ScansPerDay` (0: no scan access), `ScanBytes` (the pool's own scan cap,
4 MiB to 1 GiB; 0: 4 MiB; stated in each scan's answer, never in a
limits object), `Audio`, `AudioStreams`, `AudioTranscodes`. With
`Audio`, tracks count apart, within the last two; without, in
`Streams` and `Transcodes` ([music 5.1](../spec/v0/music.md#51-limits)).
`Check()` gives `InvalidLimits`.

**`Library`** `{ID, Name, Offered}`. **`WantedLibrary`** `{ID`
(the peer's public ID)`, Name, Wanted, Gone}`. A library carries no
kind: its titles' keys say what it holds. **`CodeView`** `{ID, Note, Created, Expires, Uses, Group}` (`Uses`: how many more servers may use it).
**`CodePreview`** `{Address, Fingerprint, Expires, Group, GroupName}` (the group a group's code invites to: show its name before redeeming).

**The toolkit** (package `wire`, `github.com/msfp-io/msfp-go/wire`) has the protocol's pieces: keys, codes,
signatures, strict JSON, pins, statements, and helpers a host uses:
`ParseKey` (a title key, refusing all but the canonical form),
`CleanText`, `WebVTTPart`, `Origin`, `CheckAddress`, `SentTag` and
`SentTags` (a media server's language codes as MSFP sends them).

## 11. An example

```go
store, err := jsonfile.Open("data/msfp")
if err != nil { return err }
eng, err := msfp.Open(ctx, msfp.Config{
    Address:  "https://media.example.org",
    Store:    store,          // seals with its own key file
    Name:     branding.Name,
    Catalog:  myCatalog,
    Playback: myPlayback,
    Media:    myMedia,
    Notifier: panelEvents,
})
if err != nil { return err }
mux.Handle("/api/v1/federation/", eng.FederationHandler())
mux.Handle("/api/v1/stream/", eng.StreamHandler(myOwnStreams))
go eng.Run(ctx)

// The panel's "Use a code" button:
peer, err := eng.RedeemCode(ctx, pasted)
switch {
case errors.Is(err, msfp.ErrCodeExpired):
    show("This code has expired; ask for a new one.")
case err != nil:
    show(explain(err))
default:
    show("Waiting for " + peer.Name + "'s operator. Fingerprint: " + peer.Fingerprint)
}
```

A peer's playback here, step by step:

1. Its `play` reaches `FederationHandler`: signature, pairing, part,
   body and limits checked.
2. The engine finds the title's copies in `Catalog.Titles` (within the
   offered libraries), picks the copy and the audio the request names
   (else the first copy and the file's default), decides what is served,
   and asks `Playback.Describe`.
3. `Playback.Start`: no delivery was asked and the copy is HEVC, so
   the host serves the baseline by `transcode`, calls
   `Admit("transcode")`, checks its own budget, starts, returns
   `Started`.
4. The engine makes the ID and the ticket, answers, tells
   `PlaybackStarted`.
5. The peer fetches `…/stream/<ticket>/master.m3u8`: `StreamHandler`
   checks it, calls `Media.Serve`, checks the playlist, sends it.
6. No `progress` and no fetch for 2 minutes: `Run` ends it, cuts its
   responses, calls `Playback.Stop(handle, expired)`, tells
   `PlaybackEnded`.
