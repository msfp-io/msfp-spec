<img src="logo.png" alt="" width="64" align="left">

# MSFP: Media Server Federation Protocol

MSFP lets media servers share their libraries with each other: one
server's people browse and play titles another server holds, through
their own server.

Version 0 covers manual pairing only: two operators pair their servers
by hand, and each trusts only the servers it paired with. Larger
federations are planned, not specified.

## Version 0: a draft

[spec/v0/](spec/v0/README.md) is a core plus five parts, each with its
own version, agreed per peer in `hello`:

- [core](spec/v0/core.md): the strict rules, HTTPS only, MSFP's own labels
  and headers, signed answers that cover their media type, key rotation
  and revocation, address changes, versions and parts, and an optional
  discovery document (`/.well-known/msfp`, off by default);
- [catalog](spec/v0/catalog.md): libraries and titles, opaque library
  IDs, the general rules of title keys;
- [playback](spec/v0/playback.md): starting, keeping and stopping a
  playback, limits;
- [media](spec/v0/media.md): what the stream URLs serve;
- two content parts, each optional:
  [video](spec/v0/video.md), films and series by TMDB (TMDB's
  numbering, several episodes in one copy, IMDb and TVDB identifiers,
  subtitles, skip markers, seek-bar previews), and
  [music](spec/v0/music.md), artists, albums and tracks by MusicBrainz.

It comes from proposals 1 to 8 and the clarifications in
[proposals/](proposals/README.md), all adopted on 2026-11-16, proposal 9
(option 1, servers that can't be called), and proposal 11 (music, every
content type optional), adopted on 2026-11-17.

## The test draft

[spec/history/test-draft.md](spec/history/test-draft.md) was MSFP's
test draft of November 2026. Version 0 supersedes it and doesn't work
with it. The test draft is not to be implemented; it stays here as
history, with its corrections of 2026-11-16 and 2026-11-17
([section 17](spec/history/test-draft.md#17-changes)).

## Test vectors

[vectors/](vectors/README.md): signed sample requests and answers, and
valid and invalid cases, for an implementation's tests.

- [vectors/history/test-draft.json](vectors/history/test-draft.json), the
  test draft's vectors from Overfin's code, stays as history and never
  changes.
- [vectors/history/test-draft-strict.json](vectors/history/test-draft-strict.json) and
  [vectors/v0/](vectors/v0/) come from the generator in
  [tools/vectors/](tools/vectors/), which also checks every file. No
  implementation produced them. test-draft-strict.json is history too, but
  its canonical-form cases hold for core 0.

## Conformance suite

[conformance/](conformance/README.md): a program that pairs with a server
you run, as a peer, and checks it against version 0, over HTTPS both
ways: pairing, `hello`, versions and parts, signatures, the strict
rules, replay, the catalog, playback and media, rotation and revocation,
address changes, servers that can't be called, and on request rate
limits and ending. Python 3, standard library only, with a self-test
against a fake server. Its code is under the MIT license. The test
draft's suite is in git history.

## Implementations

- [Overfin](https://github.com/Overfin/server), the first one.
- [jellyfin-plugin-msfp](https://github.com/C0PT3R/jellyfin-plugin-msfp),
  a Jellyfin plugin in C#.

Questions and gaps in the specification are welcome as issues.

## License

- The specification and this README: [CC BY 4.0](LICENSE). Copy, translate
  or adapt them, with credit. Implementing the protocol needs no permission
  and carries no condition.
- The test vectors (`vectors/`): [CC0 1.0](vectors/LICENSE), no conditions.
  Copy them into any test suite.
- The conformance suite (`conformance/`): [MIT](conformance/LICENSE), and
  its bundled Ed25519 reference code the IETF Trust's Simplified BSD
  License.
