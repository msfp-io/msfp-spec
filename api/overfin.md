# How Overfin hosts the engine

**Informative, not part of the protocol.** Overfin is the first host of
the Go engine. Its own federation code (`internal/federation`,
`internal/msfp`) is gone: package `msfp` runs the protocol, and Overfin
plugs in as [host.md](host.md) says.

## 1. What is the engine's, what is Overfin's

| Concern | Engine (`msfp`) | Overfin |
|---------|-----------------|---------|
| pairing, keys, addresses, limits, signed calls | all of it | the panel's pages, calling the actions |
| relations and groups | the relations of each link, groups' events, rights and mail | the panel shows relations, not links (`internal/admin/federation.go`: `AskIndividual`, `EndIndividual`, `RefuseIndividual`, `BlockPeer`) and groups (`internal/admin/federation_groups.go`) |
| storage | the records' meaning | `store.MSFP()`: the records in its SQLite database (`internal/store/msfp.go`), groups included (`GroupsTx`) |
| its own catalog | paging, offers, public IDs, checks | `store.MSFPCatalog` (`internal/store/msfp_catalog.go`): libraries, titles, copies, with `ChangeLog` and `SeqBumper` on its `title_changes` log |
| peers' playbacks here | checks, copy and tracks, the pairing's counts, tickets, expiry | `api.PeerHost` (`internal/api/peerhost.go`) as Playback and Media: Jellyfin plays the copy, within the server's transcodes less those kept for its own people; a remux Jellyfin turns out to re-encode, found at the first progress, goes into the pairing's transcodes (`Slot.Transcoding`), and is stopped when none is left; a copy of several files (Jellyfin's stacked parts, `PartCount`) is `not-playable`, since Jellyfin streams its parts apart |
| its viewers on peers | `Play`, `Progress`, `Stop`, `Fetch` | accounts, profiles, its browser route (`internal/api/peerplay.go`, `stream.go`) |
| connections out | TLS, pins, no redirects | `internal/outbound` as the Dialer: direct, or through the gateway on the backend, addresses checked either way |
| sealing | the contexts | its `seal.Box` (the store's `Sealer`) |
| events | `Notify` | each one reloads the federation page's lists; logged; a server waiting to pair or to join a group, a member's request and an ask for the individual relation (`PeerWaiting`, `GroupJoinWaiting`, `GroupRequestWaiting`, `RelationsChanged`) email the admins, once each, unless the notice settings keep requests quiet; a reading's new titles wake its TMDB fetcher; a playback ended on a peer stops the viewer's player |

The wiring is `overfin/federation.go`.

## 2. How it is set up

- **One engine for the run**, opened at the first build that has an
  address: the site's origin, `https://host[:port]` alone. A server
  whose record says it can't be called opens without one.
- **Settings reloads** never open a new engine: a new site address is a
  move (`SetAddress`), and `SetDiscovery` follows the configuration.
- **Parts**: `catalog`, `playback`, `media`, `video`, `groups`. Overfin never
  shares nor plays music; a peer's music titles are not stored.
- **Pins**: the site's `certificates` setting when given; otherwise the
  engine reads them.
- **Name**: the branding's name when on, else the site's host.
- **Routes**: each of the thirteen calls registered as its own public route
  (no catch-all), the client address from Overfin's trusted proxies
  (`WithClient`). The stream routes put `StreamHandler` in front of
  Overfin's own, which is the fallback. `/.well-known/msfp` is the
  engine's only while discovery is on; otherwise it is the site's path
  like any other.
- **Playback ends from Overfin's side**: its idle cleanup calls
  `StopPlayback(id, expired)`, the operator's stop `operator`.

## 3. The migration

Migrations 129 and 130 move an existing server onto the engine, keeping
its identity and every pairing: nothing to pair again; 132 follows the
engine's seal contexts; 134 adds groups.

- **129** adds `msfp_server` (the server record, JSON), a `record`
  column on `peers` (each `PeerRecord`, JSON) and `ids` on `peer_titles`.
  In the same transaction, Go code (`convertFederation`,
  `internal/store/msfp_convert.go`) fills them from what Overfin kept:
  its keys, address, move and uncallable settings, peers, rotations,
  wants and codes.
- **Sealing.** Every seed and secret was sealed with the server's box
  under Overfin's own contexts (`federation key`, a code's ID…). Each is
  opened and **sealed again** with the same box under the engine's
  (`msfp current key`…). A server that never federated has nothing to
  convert; the engine makes its keys at the first start.
- **IDs kept.** Peer IDs stay Overfin's (opaque to the engine). Public
  library IDs stay. Version IDs keep Overfin's form
  ([README](README.md#conventions)), so peers' saved choices still work.
- **130** drops what no longer reads it: the old columns of `peers`
  (rebuilt with `id`, `public_key`, `name`, `state`, `paired_at`,
  `suspended_at`, `record`), `peer_rotations`, the `federation-*` meta
  values, and the offer rows of the change log (`item/…`): an offer
  change is now a number of the engine's (`BumpSeq`).
- **132** seals the revoked key of a key replaced again (`resealRevoked`),
  under `msfp revoked key` rather than `msfp old key`, which 129 and
  the engine before 2026-11-18 used.
- **134** moves to MSFP version 0 and its groups part (msfp-go v0.4.0):
  pairing codes gain `uses` and `group_id` (codes made before go: they
  can't be read under version 0), and `msfp_groups` keeps each
  `GroupRecord` as JSON.

The columns beside `record` (name, state, times, key) and the tables
`peer_offers`, `peer_wants`, `peer_titles` stay for Overfin's pages,
which join peers' titles with its own in SQL. The store copies them from
the record at each write.

## 4. What stayed Overfin's

- One registry of live streams (`playback.Tickets`) for both its people
  and peers' playbacks: a peer counts as an account
  (`playback.PeerPrefix` + peer ID), so the live view shows both.
- Its next key is exported as `msfp-next-key-1.` only; the `overfin-`
  form is no longer accepted.
- Overfin's own Jellyfin plugin, its companion, speaks no MSFP. The MSFP
  Jellyfin plugin (`jellyfin-plugin-msfp`) is another host, apart from
  Overfin, on the C# engine (`Msfp.Engine` 0.7.1-preview).
