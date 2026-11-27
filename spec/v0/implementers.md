# MSFP version 0: notes for implementers

**Not normative.** Nothing here is a rule. The specification names no
product; this file does. It holds what the parts said of particular
media servers and implementations (Overfin, Jellyfin, Plex and others)
about MSFP's own rules: suggested numbers, limits and retries,
certificates and connections, scans, transcode budgets and pairings. An
implementation that does otherwise still conforms, as long as it follows
the parts. It was made by
[proposal 17](../../proposals/0017-neutrality.md).

The notes on the media language (library types, identifiers, language
codes, decoding safely, facts from a media server, choosing copies and
tracks, video, playback, media and music) moved with
[proposal 19](../../proposals/0019-msfp-media.md) to
[MSFP Media's notes for implementers](https://github.com/msfp-io/msfp-media/blob/main/spec/v0/notes.md).

What Overfin does is said of its version at the time of writing; its
code is the reference for what it does now.

## Contents

1. [Core](#core)
2. [Scans](#scans)
3. [Playback](#playback)

## Core

### Suggested numbers

The limits of [core section 8.3](core.md#83-limits-and-overload) are
each implementation's. These are suggestions, Overfin's (the Jellyfin
plugin uses the same call rates and requests at once); a server sets its
own to fit its machine:

| Limit | Suggested default |
|-------|-------------------|
| Request body | 64 KiB (core section 3) |
| Nonces remembered | 20,000 per known peer, 1,000 of strangers (core section 6.5) |
| Requests per network address, before any signature check | 50 a second, in bursts of up to 400 |
| Calls per peer (item 1) | 20 a second, in bursts of up to 200 |
| Calls of all peers together (item 2) | 200 a second, in bursts of up to 2,000 |
| Requests at once (item 3) | 8 per peer and 64 in all, each with a queue twice as long, waited in for 5 seconds at most |
| `play` per peer (item 4) | 30 a minute, in bursts of up to 20 |
| Playbacks per pairing | 2 streams, 1 of them transcoded ([playback section 6](playback.md#6-what-the-owning-server-enforces)) |
| Subtitle and playlist reads at once, on the viewer's server | 4 ([MSFP Media's fetching, section 2.4](https://github.com/msfp-io/msfp-media/blob/main/spec/v0/fetching.md#24-bounds)) |
| Preview sheet reads at once, on the viewer's server | 4 ([MSFP Media's fetching, section 2.4](https://github.com/msfp-io/msfp-media/blob/main/spec/v0/fetching.md#24-bounds)) |

### Overfin's limits and retries

Overfin's numbers, fixed, with no settings:

| Limit | Overfin |
|-------|---------|
| Per network address, before any signature check | 50 a second, bursts of 400, an IPv6 `/64` counting as one address; past it, the unsigned `404` |
| Per peer | 20 calls a second, bursts of 200 |
| All peers together | 200 calls a second, bursts of 2,000 |
| `play` per peer | 30 a minute, bursts of 20 |
| Requests at once | 8 per peer (a queue of 16), 64 in all (a queue of 128); a queue is waited in for 5 seconds at most |

Past a peer's own limit it answers a signed `429` `rate-limited`, past
all peers' a signed `503` `server-busy`, each with `retryAfter` (the
seconds until a call would pass, at least 1) in the body and the same
`Retry-After` header. Behind its gateway, the gateway also limits
requests by network address, with the same numbers, and answers a
plain-text, unsigned `429` with `Retry-After: 5`. A single Overfin
counts by the address its `identity.trusted_proxies` setting gives
(`X-Forwarded-For` from those proxies only, read from the right). Behind
its gateway, the backend counts by the visitor's address the gateway
passes on, which the gateway takes from the PROXY protocol or
`X-Forwarded-For` of the proxies it is told to trust. Its `play` refusal
for transcodes, a signed `503` `server-busy`, carries no `retryAfter`.

As a caller, Overfin holds every call to a peer (but `pair`) until a
signed `retryAfter` has passed. After no answer, an unsigned refusal, or
a `429` or `503` without `retryAfter`, the calls it makes by itself
(`progress`, catalog reads, a waiting pairing's `hello`) hold off from 2
seconds, doubling, up to an hour, give or take a quarter; calls an
operator or a viewer asks for go when asked. A catalog read told to wait
up to a minute waits and reads on. A `play` held or refused this way
tells the viewer "busy now".

### Overfin's certificates and connections

Overfin reads its own certificate again for a `hello` when its last
reading is more than 2 seconds old
([core section 3.2](core.md#32-https-certificates)), and, while no
reading has ever worked, tries again after 5 seconds rather than a
minute.

Overfin refuses the table of [core section 3.3](core.md#33-connecting-to-peers),
by default, judged as that section says for IPv4-mapped, NAT64
(`64:ff9b::/96`, and `64:ff9b:1::/48` refused outright) and 6to4
addresses, and three cloud metadata addresses outside it
(`100.100.100.200`, `168.63.129.16`, `fd00:ec2::254`), on every
connection to a peer: calls, the media relay, preview sheets, the
`hello` that confirms an address change, and the connection to its own
address when it reads its own certificate. It checks at connection time
only. Its one setting, `federation.allow_loopback`, allows loopback and
nothing else. It offers no setting to refuse private ranges. A backend
behind Overfin's gateway connects through the gateway, which makes the
same check. It uses no proxy from the environment.

### Media answers and logging

As the viewer's server, Overfin answers a peer's `404` to a media fetch
([core section 8.4](core.md#84-media-answers)) with a `404` to the
viewer, for every kind of fetch, and doesn't end the playback itself:
the player stops on its own, and the playback ends by `stop` or, on both
sides, by expiry
([MSFP Media's playing, section 5](https://github.com/msfp-io/msfp-media/blob/main/spec/v0/playing.md#5-a-playbacks-states-and-endings)).

Overfin logs none of the secrets of
[core section 14.1](core.md#141-logging). A failed media fetch has the
URL removed from its error before it is logged, and a crash in a request
logs the route's pattern, not the path.

### A media server under a base path

A media server reached under a path (Jellyfin at `/jellyfin`, say) has a
prefix, and needs the discovery document at its address's root
([core section 11](core.md#11-the-discovery-document)): its reverse proxy
serves it there when the media server serves nothing at the root.


## Scans

- **Plex as a viewer** reaches a peer's copies only through scan access
  ([playback section 8](playback.md#8-scan-access)): it lists files by
  reading them through a file system, then plays them.
- **The cap.** The scan's answer states how many bytes its ticket may
  read (`scanBytes`, 4 MiB to 1 GiB). Media servers read 1 to 4 MiB of a
  file to list it, but a 4 MiB cap misses an MP4's `moov` box or
  Matroska's cues written at the end of a big file: a pool for scans is
  better set to 32 MiB.
- **Plex's "extensive media analysis"** must be off for a library of a
  peer's copies: tested, it reads every file whole, and when a read is
  refused, overwrites the file's length with what it read.

## Playback

Notes for [playback](playback.md), not normative. The notes on choosing
a delivery and on Overfin's requests are
[MSFP Media's](https://github.com/msfp-io/msfp-media/blob/main/spec/v0/notes.md#playback).

**Transcode budgets.** Jellyfin starts as many transcodes as it is
asked: an owning server on it makes a budget of its own, an operator
setting counting its own people's transcodes and its peers' together
([playback section 6](playback.md#6-what-the-owning-server-enforces)).

**Suggested numbers.** A reasonable start for a new pairing: 2 streams,
1 transcode, no bitrate cap, no downloads and no scan access; the
operator changes them, per pairing and per group. An operator who allows
downloads may start from 1 at once, 10 a day and 20,000 kbit/s; one who
allows scans, from 1,000 a day and a scan cap of 32 MiB (33,554,432
bytes), which its scan answers state as `scanBytes`
([playback section 8](playback.md#8-scan-access)).

**Overfin's playbacks.** Overfin's check and count are one step under
one lock (atomic, [playback section 6.2](playback.md#62-limits-at-once)),
and it checks the pairing's streams once more before asking its media
server, to refuse early. A pairing that ends (by either side), a peer
found no longer paired, or a change of what it offers a peer ends that
peer's playbacks at the moment of the change, open media responses
included; each media request checks again
([playback section 6.3](playback.md#63-how-a-playback-ends)).
