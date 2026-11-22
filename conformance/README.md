# MSFP version 0 conformance suite

A program that plays a peer against a server you run, and checks that
server against [version 0](../spec/v0/README.md): the
[core](../spec/v0/core.md) and the parts
[catalog](../spec/v0/catalog.md), [playback](../spec/v0/playback.md),
[media](../spec/v0/media.md), [video](../spec/v0/video.md) and, when the
server lists it, [music](../spec/v0/music.md). It tests version 0 only. For the part [groups](../spec/v0/groups.md),
with `--group-code`, it checks joining a group of the server's, its
list, `group-sync`, `group-push`, the mail both ways (sealed with HPKE,
in plain Python in `msfpconf/hpke.py`), `group-pair` as the caller, and
leaving; not the server calling `group-pair`, nor the state's replay
beyond what a refusal shows (tools/vectors checks the group vectors).
The test draft's suite is in this repository's git history (up to commit
`8cb79af`).

It pairs with the server from a pairing code, then checks the pairing,
`hello` and the versions and parts it carries, signatures both ways,
the strict forms of core section 2, replay and nonces, the catalog,
playback and media, rotation and revocation of its own key, address
changes, what a server does toward a peer that can't be called, and,
with `--watch-pause`, the server as a viewer of the suite's films.
Every check names the file and section it tests (`core 6.4`,
`media 2.1`), whether that section says *must* or *should*, whether it
needs the server's operator, and why it passed, failed or was skipped.

It needs Python 3 and its standard library, nothing else (tested with
Python 3.13). Ed25519 is bundled in pure Python (the reference code of
RFC 8032 with core section 2.1's point checks on top), and so is the
ECDSA P-256 code that makes the suite's own self-signed certificate. It
is test code: slow, not constant-time, never for a key that matters.

## Running it

The server must be able to reach the suite: the suite puts its own
address in the `pair` request, and the server calls it there (`hello`
when its operator accepts, `libraries` and `titles` when it reads the
suite's catalog, the confirming `hello` of an address change). Core
section 3.3 has servers refuse loopback, so run the suite on another
machine, or another container, that the server reaches.

1. On the server, make a pairing code (it works once: one code per run).
   A second code, given as `--code2`, adds the checks of a second
   pairing (see below).
2. Run the suite:

   ```sh
   python3 conformance/run.py \
       --code 'msfp-pair-0.…' \
       --address https://192.0.2.40:8000 \
       --listen 0.0.0.0:8000
   ```

   `--address` is where the server reaches the suite; `--listen` is
   where the suite listens. The suite also listens on the next port
   (8001 here) for the address-change checks: open both, or give
   `--new-address` and `--new-listen`, or `--skip AD`.
3. When the suite says so, do what it asks on the server: accept the
   pairing first (check the fingerprint it prints against the server's
   page). The suite waits for each step (`--accept-timeout`, 600 s;
   `--operator-timeout`, 300 s); press Enter to skip a step.
4. On the server, offer this peer at least one library holding a film
   or an episode that plays: the catalog, playback and media checks are
   skipped otherwise.

Without `--code`, only the checks a stranger can make run (S1 to S4,
D1), against `--target`.

### HTTPS and pins

Every call is over HTTPS, both ways, as core section 3 says. The suite
serves HTTPS with a self-signed ECDSA P-256 certificate it makes for the
run (for the host of `--address` and of the new address), and names its
pin in every `hello` it sends or answers. The server must therefore
accept it as core section 3.2 says: any certificate for `pair` and
`hello`, the named pin for everything else. `--tls-cert` and
`--tls-key` give a certificate of your own instead.

As a client, the suite follows section 3.2 too: `pair`, `hello` and the
discovery document take any certificate; every other call, and every
media fetch, needs a certificate that is publicly trusted or whose pin
the server named in its latest signed `hello` (a refused one sends a
`hello` and tries once more, as for a renewal).

### Plain http, for implementations' test switches

`--plain-http` runs everything over plain `http`, both ways: the codes,
the addresses and the calls. This is **outside the protocol**: core
section 3.1 lets an implementation offer plain `http` as a test switch
for its own development, and nothing else. The HTTPS checks are skipped
then (H4 to H6, and the certificate rules everywhere else).

### Checks that need the operator

Some checks need the server's operator to do something on the server.
They are marked `[operator]`:

| Step | Checks |
|------|--------|
| accept the pairing | W4, and everything after it |
| want the suite's library "Conformance films" | Q1 to Q3 |
| add one title to an offered library and remove one (with `--watch-changes SECONDS`) | T9 |
| play one of the suite's films, pause it at once, stop it after the given time (with `--watch-pause SECONDS`) | VW1 to VW4 |
| accept the second pairing (with `--code2`) | UP2, UP3 |
| accept the suite's join of the group (with `--group-code`) | GR8 to GR17, GM |
| name the suite an admin of that group | GM1 to GM8 |
| accept the request of "MSFP conformance suite, second" in that group | GM5 to GM8 |
| confirm the suite's pending key rotation | K11 |
| rotate the server's own key | O1, O2 |
| rotate it again, while the suite can't be called | O3 |
| revoke the server's key (with a rotation in the same action, if it has one) | O4 |

They run when the suite is run from a terminal (or with `--operator`);
not run interactively, they are skipped with that reason. `--no-operator`
skips them all. The pairing must still be accepted: with
`--no-operator`, the suite waits `--accept-timeout` (60 s by default
then) for a test switch that accepts it by itself, and skips everything
after the pairing if nothing does.

With `--watch-pause`, the suite is also the owning server of its three
films: it answers `play` with a playback of a two-second clip (served as
it is, with ranges, under its stream root) and ends a playback it hears
nothing of for 2 minutes, as playback section 6.3 says. The checks VW1
to VW4 look at the server as a viewer: its `play` well signed and
formed, the playback kept while its viewer pauses (no 2 minutes without
`progress` or a media request), the pause reported, and `stop` at the
end. Give a pause longer than 2 minutes (150 s, say) for VW2 to tell.

The steps can also be played by a program, through the server's own
administration API: build the suite in Python with
`Suite(args, report, operator_hook=hook)` (from `msfpconf.checks`, with
`run.parser()` for `args`), where `hook(step)` is called with the step's
name (`accept`, `want`, `change`, `watch`, `accept2`, `join-accept`,
`admin`, `request-accept`, `confirm`, `rotate`, `rotate2`, `revoke`) before the suite starts watching for it, exactly as
an operator would be asked. The suite still checks the outcome itself
and waits up to the step's timeout. `selftest.py` plays the operator of
its own test server this way.

### Other options

| Option | What it does |
|--------|--------------|
| `--code2 CODE` | P11: a second live code from the known key gets the signed `409` with no body; then UP1 to UP4: a second pairing, as a server that can't be called (`inbound: false`, an address `https://msfp-<26>.invalid`), accepted by the operator, ended with `hello` `ended` |
| `--group-code CODE` | GR1 to GR17 and GM1 to GM8: a code of a group the server's operator made, of the same server as `--code`, with 2 uses or more (GR7 needs one left). The suite, already paired, joins through `group-join`; the operator accepts; then the list, `group-sync`, `group-push`, the mail and `group-pair` (GM), and leaving are checked |
| `--load` | R1 and R2: a burst of `--load-count` (1,000) calls over 32 connections, expecting a signed `429` `rate-limited` (or `503` `server-busy`) with `retryAfter`, then calls passing again. A burst that meets no limit is a SKIP unless it was big and fast (1,000 calls at 300 a second or more) |
| `--end` | E1 and E2: ends the pairing with `hello` `ended`. Without it, the pairing stays: remove the peer on the server afterwards |
| `--skip IDS` | skips checks by ID or prefix: `--skip AD,X1,U` |
| `--uncallable-watch SECONDS` | how long the suite watches that nothing connects to it once it says it can't be called (20) |
| `--connect host:port` | connects elsewhere than the server's address says, the requests staying signed for the address |
| `--key FILE` | keeps the suite's key between runs |
| `--json FILE` | the results as JSON |
| `-v` | the whole detail of passing checks too |

The exit status is 1 when a check failed.

## What it checks

The run goes in this order; the counts are the checks in each group.

| Group | Checks | Sections | What is checked |
|-------|--------|----------|-----------------|
| C | 1 | core 5.1 | the code: prefix `msfp-pair-0.`, base64url, the key, the expiry, the address |
| D | 1 | core 11 | the discovery document: none, answered like a missing path (`404`, or whatever such a path gets), or a valid document whose `base` the suite then uses |
| S | 4 | core 6.4, 3, 14 | strangers: the unsigned `404` on every call, like a missing path |
| P | 11 | core 9.1, 2.1, 5.2, 6.5, 12.5 | `pair`: wrong proofs, forged and small-order keys, stale dates, bad addresses, bad members (`inbound`, `null`, wrong types), the pairing itself, the next key commitment, replay, the used code, the signed `409` |
| W | 4 | core 7.1, 7.3 | waiting: `hello` `waiting`, no forcing `paired`, a signed `404` on `libraries`, the server's `hello` on acceptance |
| H | 6 | core 9.2, 10, 3.2 | `hello`: its members, versions and parts (core 0; catalog, playback, media, video), its address, the pin of the certificate served, calls that need *trusted* or *named*, removing and naming pins |
| V | 7 | core 10.2 to 10.4, catalog 5 | versions: bad `core` and `parts` refused, odd entries ignored, a part that is off answered `404` `not-supported` (`playback`, `catalog`, `media` as a dependency), no `limits` and no video titles when their parts are off, no common core (every call but `hello` gets the signed `404` `not-supported`) |
| J | 2 | core 2.3 | I-JSON and types: twice-named members, surrogates, noncharacters, bad UTF-8, a byte order mark, `null`, wrong types refused; white space and unknown members accepted |
| G | 10 | core 2.2, 2.4, 3, 6.2, 6.4 | signatures: dates, nonces, keys and signatures outside their one form, another host, path or key, duplicate headers, a query, percent-encoding, the media type, `Sec-Fetch-Site`, content codings, a body over 64 KiB |
| N | 3 | core 6.5, 6.3 | replay, a nonce used again, the answer bound to the nonce and the media type |
| L | 5 | catalog 2, playback 5, music 5.1 | `libraries`: IDs, kinds, `limits` and their ranges, opaque and stable IDs, bad bodies |
| T | 9 | catalog 3, 4, 5; video 3, 4; music 2, 3 | `titles`: canonical keys, kinds, libraries, language tags, `with`, `ids`, music descriptions, order, paging, `since` and `seq`, bad requests, the first library, changes adding up |
| PB | 11 | playback 2 to 6, video 5, music 5 | `play`: malformed requests (`400`), keys not offered (`404`), the playback object and its URLs, nothing said (HLS, not direct); `progress` and `stop` and the playback's states; the choice of audio, subtitles and copy; the streams limit and the race for the last stream |
| M | 9 | media 1 to 4, video 5.5, 6 | the media: the HLS subset in master and media playlists, segments, ranges (`206` and `416`), refusals (a query, browser headers, an unknown ticket, other methods), WebVTT parts in their rebuilt form, PGS parts, preview sheets |
| Q | 3 | catalog 2, 3; core 6.4 | the server as a reader of the suite's catalog: its calls well signed, its `titles` requests, following `next` |
| VW | 4 | playback 2 to 4, 6.3; core 6.4 | (`--watch-pause`) the server as a viewer of the suite's films: its `play` well signed and formed, the playback kept while paused, the pause reported, `stop` at the end |
| R | 2 | core 8.3 | (`--load`) a signed `429` or `503` with `retryAfter`, then calls passing again |
| GR | 17 | groups 2, 3, 6 to 8 | (`--group-code`) groups: the group code; `group-sync` from a non-member and about an unknown group (a `404`, plain or signed: see the notes); `group-join` with a wrong proof, an unknown code (signed `404`s), the code (`200` `{}`), again (`409`); accepted, the list pushed (`founded`, the suite's `joined`, every envelope well signed); `group-sync` (every event, `seq` order, `have`; nothing more with a full `have`); the suite's `profile` and an unknown type kept; a forged signature, a date 10 minutes ahead, another group's event and a payload over 4,096 bytes not kept; `left`, then a `404` (GR17 runs after GM) |
| GM | 8 | groups 4, 5, 8.3 | (`--group-code`) the mail: named admin, the suite signs `joined` for a second identity N of its own that can't be called, and N its `profile`; N's request, sealed with HPKE to the server's `encryptionKey`, kept; accepted, the server's answer, signed, sealed to N, opened, with its address; N's `group-pair` with it (signed `200`), the same again (`409`), N's `hello`; a mail with a forged signature and an expired one not kept |
| UP | 4 | core 7.5, 9.1, 7.4 | (`--code2`) a pairing as a server that can't be called |
| K | 11 | core 9.3, 9.4, 12 | the suite's own key: a rotation not matching the commitment (`409`), bad `rotate` and `revoke` bodies (`400`), revocation and suspension (only `rotate` and `revoke` answered), a rotation to the committed key lifting it, a statement with `-`, the previous key heard for 10 minutes, a repeated rotation, `202` `pending` without a commitment, the operator's confirmation |
| AD | 3 | core 13.2 | address changes: a new address the suite serves, confirmed by a `hello` there before any other call; moving back; an address nothing serves, the old one staying |
| X | 1 | core 3.3 | refused destinations: loopback addresses announced as the suite's are never connected to |
| O | 4 | core 9.3, 12.3, 12.4, 12.6, 12.7 | the server's own key: a pushed `rotate` signed with the old key and matching its commitment, then signing with the new key only; a rotation and a revocation pulled in `hello` answers by a server that can't be called |
| U | 7 | core 5.2, 7.5, 9.2, 13.2 | the suite as a server that can't be called: `inbound: false` only with an address `https://msfp-<26>.invalid` (the forms of `vectors/v0/strict.json`), stored at once, `knownKey` and `rotation` type-checked, its own calls still answered, no connection to it at all, and `inbound` back to `true` confirmed by a `hello` first |
| E | 2 | core 7.4 | (`--end`) `hello` `ended`, then the unsigned `404` |
| A | 2 | core 6.6, 8.2 | every answer that had to be signed was, and every signed refusal is a problem document |

That is 147 checks in all, with `--code2`, `--group-code`, `--load` and
`--end`, and 151 with `--watch-pause` too. A server that doesn't answer
at its address at all gets one failed S0, and nothing else runs.

Every signed answer is also read as I-JSON and checked for `null`; a
signed `200` or `202` must be `application/json`, and the answer's media
type is signed as core section 6.3's fifth line.

The suite offers the server one library, "Conformance films", with
three films on two pages. Its first page is short and still has `next`,
on purpose, to see that the server follows it; catalog section 3.1 says
a whole-list `next` comes only after a full page of 5,000, so the suite
itself breaks that rule there. It answers `play` with `409`
`not-playable`: it serves no media.

## Its limits

- **Seen from outside.** The suite sees only what reaches it and what
  the server answers. Refused destinations (core 3.3) can fail only for
  loopback, and only when the suite shares the server's machine;
  link-local, metadata, multicast, NAT64 and 6to4 addresses can't be
  seen. "Never connects to a peer that can't be called" is seen as
  nothing reaching the suite's real addresses after the switch.
- **The server's own operations** (its rotations, revocations,
  acceptances, renewals of its certificate) need its operator; the
  suite can't make a server rotate.
- **Timing rules** that take long are not waited for: the 30-day
  overlaps, the 30-day suspension turning `gone`, the hourly retry of a
  failed address confirmation, the 2-minute expiry of an unused
  playback, the 24-hour code.
- **Limits** are each server's numbers (core 8.3). The burst of `--load`
  is pure Python, and may not reach them over HTTPS; the `play` limit
  is not tested (the suite paces its own plays under it).
- **Media** is checked on the first stream, its first media playlist
  and segment, part 0 of one text and one PGS subtitle track, and sheet
  0: the suite doesn't play a whole title. When the first title played
  has no text or no PGS track, M6 or M7 waits for that playback to stop,
  then plays up to 8 other films or episodes listing subtitles for one.
- **Heuristics**: L3 (library IDs that look like a media server's) and
  the shortest-language-code rule (a list of common three-letter codes)
  can't be exact.
- **Not tested**: what a receiver does after a `hello` with no common
  core, beyond V7's signed `404`s; relations and `individual` in `hello`
  (core 7.7), not covered yet; a change of `base` (not announced, core open question 16);
  several certificates; a peer that ends the pairing toward the server
  while it is suspended.

When it ends, the suite prints the points of the specification it found
unclear or couldn't test from outside.

## Its self-test

```sh
python3 conformance/selftest.py           # about 5 minutes
python3 conformance/selftest.py --quick   # without the fault pass
```

It checks the suite's readers against the cases of `vectors/v0/` but
`video.json` (keys, code, exchanges, rotation, revocation, strict, media,
music and groups; and
those of `vectors/history/test-draft-strict.json` that core section 17 says hold for
core 0), then runs the whole suite, over HTTPS both ways, against an
in-process fake version 0 server (`msfpconf/fake.py`, built from the
specification and the vectors, with the self-test as its operator):
every check must pass. Then it runs again with deliberate faults in the
fake (replays accepted, `null` accepted, wrong changes, a bad `with`, no
stream limit, a query served, several ranges served, WebVTT not
rebuilt, `progress` answered for a playback that isn't live, loopback
connected to, the old key kept after a rotation, no `progress` while its
viewer pauses, a `stop` with a negative position, a group event and a
mail with a forged signature kept), and the checks aimed at each
fault must fail. The fake founds a group and makes its code, for the
group checks.

## License

The suite's code is under the [MIT license](LICENSE). The Ed25519
reference code it bundles (`msfpconf/ed25519.py`) is the IETF Trust's,
under the Simplified BSD License, whose notice is at the top of that
file. The specification is under CC BY 4.0 and the vectors under CC0
(see the repository's README).
