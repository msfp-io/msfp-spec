# MSFP libraries: the application-side API

**Informative, not part of the protocol.** Nothing here changes what a
server sends or accepts. Two servers interoperate if they follow
[spec/v0](../spec/v0/README.md), whatever library they use.

## Status

| Piece | State |
|-------|-------|
| Go engine | **built**: [msfp-io/msfp-go](https://github.com/msfp-io/msfp-go) (module `github.com/msfp-io/msfp-go`, v0.8.0), package `msfp` at its root, with the toolkit `wire`, `describe` (a media file to a copy's description, with ffprobe), two stores (`store/memory`, `store/jsonfile`) and an example host (`examples/share-folder`) |
| C# engine | **built**: [msfp-io/msfp-dotnet](https://github.com/msfp-io/msfp-dotnet), NuGet packages `Msfp.Protocol` (the toolkit), `Msfp.Engine` (the engine, with `MemoryStore` and `JsonFileStore`) and `Msfp.Describe`, 0.8.0-preview, msfp-go v0.8.0 in .NET; an example host (`examples/ShareFolder`) |
| Overfin on the Go engine | **built**: [overfin.md](overfin.md) |
| The MSFP Jellyfin plugin on the C# engine | **built**: `Msfp.Engine` 0.8.0-preview |

These documents describe the Go engine. The C# engine has the same
methods in its own forms ([Go and C# forms](#go-and-c-forms)).

## What this is

The library is an **engine**: it runs the protocol. The application,
the **host**, plugs in through a few interfaces.

| The engine does | The host keeps |
|-----------------|----------------|
| keys, pairing codes, signing, nonces, limits | the user interface: panel and viewer pages (the engine gives actions, queries and events, never HTML) |
| pairing and its states, `pair`, `hello`, `rotate`, `revoke`; relations ([core 7.7](../spec/v0/core.md#77-relations)) | accounts and people: who may see and play a peer's titles. The engine never sees a person |
| key rotation and revocation, address changes, servers that can't be called, the discovery document | what a title is inside the host: items, files, its database |
| connections to peers (refused destinations, pins, no redirects, no proxy) | how media is made: direct play, remux, transcode, segments, subtitles, sheets |
| `catalog`: offers, wants, library IDs, serving and reading titles | its own stream routes for its own people, and the viewer route that serves a peer's media to a browser |
| `playback` and `media`, both sides: checks, copy and track choice, counts, tickets, expiry, the relay's checks | where data lives: it implements storage |
| `video` and `music`: title keys, what a title carries | |
| `groups`: events, rights, mail, `group-sync`, `group-push`, `group-pair`, `group-join` | |

## The documents

| Document | What it holds |
|----------|---------------|
| this README | status, scope, notation, Go and C# forms, the decisions |
| [host.md](host.md) | the interfaces the host implements |
| [engine.md](engine.md) | what the host calls: opening, settings, handlers, actions, queries, viewers, events, background work, errors |
| [overfin.md](overfin.md) | how Overfin hosts the engine |
| [platforms.md](platforms.md) | whether MSFP fits Plex and Emby besides Jellyfin, and what the spec should change |

## The shape

```
          operator panel, viewer pages (host)
                 |  actions, queries   ^ events
                 v                     |
  +--------------------------- engine ---------------------------+
  |  pairing, keys, addresses, schedules, limits, catalog,      |
  |  playback states, tickets, relay checks, HTTP handlers      |
  +--------------------------------------------------------------+
     |        |          |          |         |        |
   Store   Catalog   Playback     Media     Sealer   Clock, Dialer,
  (host)   (host)     (host)      (host)    (host)   Notifier (host)

  peers --HTTPS--> host's router --> engine's handlers
  engine --Dialer--> peers
```

## Notation

Signatures are written in a neutral form, close to Go:

```
RedeemCode(text) -> PeerView
    errors: CodeInvalid, OwnCode, AlreadyPeer, Unreachable, CodeRefused
```

- Every method takes a `context.Context` first and may fail, unless
  marked **sync**. Lookups that find nothing return `nil`, not an error.
- Errors are named from [engine.md section 9](engine.md#9-errors). Any
  method may also fail with `Internal`, or with the context's error.
- Spec links point at the sections a method serves.

## Conventions

- **Peer IDs** are the engine's: random, stable, never the key (keys
  rotate). Peers never see them.
- **Library IDs.** The host names its libraries by its own IDs. Peers
  see only the opaque public IDs the engine makes
  ([listing 2.1](https://github.com/msfp-io/msfp-media/blob/main/spec/v0/listing.md#21-library-ids)).
- **Copies** are named by an opaque handle from the host. The engine
  stores it and passes it back, never sends it. A peer sees a version ID:
  `ver_` + the first 16 bytes of HMAC-SHA-256(IDs secret, `"version\n" +
  peerID + "\n" + handle`), base64url. That is Overfin's form from before
  the engine, so its peers' saved choices still work.
- **Time** comes from `Config.Clock` only, in UTC.
- **No network inside storage transactions.** The engine never calls a
  peer, or the host's media, inside `Read` or `Update`.
- **Untrusted text.** Every name and label from a peer reaches the host
  cleaned. It is still text, never markup.
- **Tickets** never leave the engine: not logged, not in errors, not
  given to the host.
- **Bodies** the engine sends are JSON with members in field order (the
  order of the specification's tables) and never a `null`.

## Go and C# forms

| Concept | Go | C# |
|---------|------------|-------------|
| Package | `github.com/msfp-io/msfp-go` (package `msfp`); toolkit `…/wire` | package `Msfp.Engine`, namespace `Msfp` (stores in `Msfp.Stores`); toolkit package and namespace `Msfp.Protocol` |
| Engine | `*msfp.Engine`, `msfp.Open(ctx, Config)` | `Msfp.MsfpEngine`, `MsfpEngine.OpenAsync(MsfpConfig, ct)` |
| Host interfaces | `Store`, `Catalog`, `Playback`, `Media`, `Sealer`, `Clock`, `Dialer`, `Notifier`, `SheetEncoder`; optional `Images`, `Lyrics`, `ChangeLog` | `IStore` (its transactions `IStoreTx`), `ICatalog`, … ; `TimeProvider` for the clock |
| Calls | `RedeemCode(ctx, text) (PeerView, error)` | `Task<PeerView> RedeemCodeAsync(string text, CancellationToken ct)` |
| Host callbacks | plain methods taking `ctx` | `ValueTask<T>`, so synchronous data costs nothing |
| Errors | `*msfp.Error{Code, Err}`, matched by `errors.Is(err, msfp.ErrNotFound)` | one `MsfpException` with `MsfpError Code` |
| Events | `Event` interface (`PeerID() string`), events are structs, a type switch | abstract record `MsfpEvent` (`PeerId`), derived records |
| Time | `time.Time` (UTC), `time.Duration` | `DateTimeOffset`, `TimeSpan` |
| Logging | `*slog.Logger` | `ILogger` |
| HTTP | `http.Handler`: `FederationHandler()`, `StreamHandler(fallback)`, `DiscoveryHandler()` | a neutral `MsfpRequest` (method, raw target, headers, body, `ClientAddress`) and `MsfpResponse`, for any router: `HandleFederationAsync`, `HandleStreamAsync` (a `StreamOutcome`; `Fallback` for a ticket the engine doesn't know), `HandleDiscovery`; the host writes the answer unchanged |
| Background | `Run(ctx)` | `RunAsync(ct)` in a `BackgroundService` |

C# raises exceptions because almost every call is async, and a
`Task<Result<T>>` on each would make every caller unwrap twice. Expected
outcomes (a rotation waiting for the operator, a page with more) are
results in both languages, never errors.

## Decisions

All decided by the owner on 2026-11-18 (17 on 2026-11-20, 18 on 2026-11-21).

| # | Question | Decision | Built |
|---|----------|----------|-------|
| 1 | Storage shape | records read and written in transactions (`Read`, `Update`), not named operations | yes |
| 2 | Shipped stores | two: memory (tests) and JSON files (small hosts) | yes |
| 3 | The own catalog's change log | either: the host's (`ChangeLog`), or the engine's, from snapshots in storage (`OwnLogTx`) | yes |
| 4 | Choosing the copy and tracks | the engine, by the spec's tables | yes |
| 5 | Tickets | the engine makes and checks peers' tickets; other tickets go to the host's fallback | yes |
| 6 | Stream counts | the engine counts the pairing's; the host its own budget; one `Admit` step | yes (`Admit` optional) |
| 7 | The host's playlists | checked by the engine before they leave | yes |
| 8 | Re-encoding sheets | a `SheetEncoder` hook; Go's default `JPEGSheets` | yes |
| 9 | C# errors | exceptions | yes |
| 10 | Events | one callback, `Notify`, in order, outside locks | yes |
| 11 | Discovery and base paths | the engine fetches peers' documents and serves its own when on | yes |
| 12 | The next key's text | `msfp-next-key-1.<seed>` only ([core 12.5](../spec/v0/core.md#125-the-next-key-commitment)) | yes |
| 13 | Nonces | in memory only; requests dated before the start refused | yes |
| 14 | Content parts | built in; the host says which it speaks (`Config.Parts`) | yes |
| 15 | Library versions | one number for both libraries | yes (C# with `-preview`) |
| 16 | An admin API | none: actions and queries only | yes |
| 17 | Groups | the engine keeps the events, checks rights, seals the mail and pairs through the group; the store keeps records (`GroupsTx`) | yes |
| 18 | Relations | the engine keeps each link's relations (the individual one, its groups) and ends a link when the last goes ([core 7.7](../spec/v0/core.md#77-relations), proposal 14); the panel manages relations, not links | yes |

## Not built yet

- In Go, a neutral request and response form (C#'s `MsfpRequest`,
  `MsfpResponse`) for routers other than `net/http`.
- `SetPins`: pins are only set at `Open` (`Config.Pins`).
- An event for this server's own pins changing, or failing to be read.
