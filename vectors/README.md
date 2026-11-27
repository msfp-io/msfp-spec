# MSFP test vectors

Values for an implementation's tests to check against. They are under
[CC0 1.0](LICENSE): copy them into any test suite.

| File | Version | Where it comes from |
|------|---------|---------------------|
| [history/test-draft.json](history/test-draft.json) | the test draft | History: the test draft of November 2026, not to be implemented. From Overfin's own code. Keys, a pairing code, the pairing proof, and three signed exchanges ([its section 15](../spec/history/test-draft.md#15-test-vectors)). It never changes. |
| [history/test-draft-strict.json](history/test-draft-strict.json) | the test draft | History, as test-draft.json. The generator below. Valid and invalid cases for the canonical forms of [the test draft's section 2.1](../spec/history/test-draft.md#21-canonical-forms) and the code and address rules of its section 5: keys (the identity, a point of order 8, a key with a small-order part, a non-canonical encoding), signatures (an identity-key forgery, a torsion R, S + L), base64url, dates, integers, JSON bodies, addresses and codes, each with its verdict. They hold for core 0 too, except that core 0 refuses `http`. |
| [v0/](v0/) | core 0 and its parts | The generator below. Keys and commitments, a `msfp-pair-0.` code with an `https` address, signed requests and answers under the MSFP labels and headers (with the answer's media type), a rotation statement, a revocation statement, a `hello` carrying `core`, `parts`, `certificates` and `address`, the `hello`s of a server that can't be called (`inbound: false`, `knownKey`, and an answer carrying `rotation` and `revocation`), address and code verdicts under core 0, file and track names in media URLs (`mediaNames`), the discovery document's `base` (`bases`), what a receiver keeps of a `hello`'s versions (`helloVersions`), the addresses of servers that can't be called (`msfp-<26>.invalid`), the destinations a server refuses to connect to, media servers' language codes and the tags sent for them, `titles` requests with their filters (`titleRequests`), `play`, `progress` and `stop` requests and answers with deliveries, replacements, refusals as a caller reads them, restarts, downloads and scans (`playback.json`), WebVTT, HLS and PGS cases, music title keys and titles with what a caller keeps of each, video titles with what a caller keeps of each, and a group's events and the state they give, a code with a group, `group-join`'s proof and a sealed request mail ([groups](../spec/v0/groups.md); [core section 17](../spec/v0/core.md#17-test-vectors)). |

**No implementation produced the files of version 0, nor test-draft-strict.json.**
The media and destination cases were checked against Overfin's own
sanitizer and checks on 2026-11-16: they agree, except for the HLS cases
that [fetching section 3.1](https://github.com/msfp-io/msfp-media/blob/main/spec/v0/fetching.md#31-hls-playlists) lists in
its Overfin note.
These files come from the
generator alone: an independent program written from the specification. When an
implementation disagrees with them, check the specification first; either
may be wrong, and a report is welcome.

In every file, `valid` is what a receiver must decide. `reason` is the
generator's own wording, for people; an implementation needn't give the
same reason, only the same verdict. Bodies and messages are the exact
bytes, as JSON strings; a body that isn't printable ASCII is given as
`hex` instead.

## The generator

[tools/vectors/](../tools/vectors/) is a small Go program, with the
standard library and `filippo.io/edwards25519` (for the point checks of
Ed25519) as its only dependency.

```
cd tools/vectors
go run .          # checks every file in vectors/
go run . -write   # writes history/test-draft-strict.json and v0/*.json, then checks
go test ./...     # checks what the files say about common libraries
```

Most of what the files check is
[MSFP Media](https://github.com/msfp-io/msfp-media)'s language, as MSFP
carries it; its text points to the arrays here. The check reads that
text from a checkout of msfp-media beside this repository
(`../msfp-media`), or from the folder `-media` names (its `spec/`).

The check:

- regenerates every file it writes and compares the bytes;
- regenerates history/test-draft.json from the test draft's rules and compares the bytes,
  without ever writing it;
- reads every file back and checks each value on its own: public keys,
  fingerprints and commitments from their seeds, codes from their parts,
  every signed message rebuilt from its fields, every signature under the
  strict rules, every pin from its key, and every verdict against the
  generator's checks.

Its tests confirm the comments in test-draft-strict.json about common libraries
(Go's `crypto/ed25519` accepts the identity-key forgery and refuses the
torsion R; a cofactored check accepts the torsion R), and check IPv6
addresses, which the vectors leave out.
