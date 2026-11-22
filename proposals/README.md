# Change proposals for MSFP

Proposals 1 to 11 change [the test draft](../spec/history/test-draft.md)
of November 2026, which they call "version 0", written while it was the
only version. Those adopted made the version they call "version 1",
renumbered [version 0](../spec/v0/README.md) by proposal 13 (2026-11-20):
a draft until it is stable. They are kept as written, in that old
numbering. Each one stands alone and is decided on its own. Each has the
same sections: Problem (with evidence), Proposal, Alternatives
considered, Cost (specification, Overfin, the plugin), Compatibility,
Open points. Since 2026-11-17, that version doesn't work with the test
draft: what the Compatibility sections say about moving test-draft
pairings, or `http` ones, no longer applies.

The evidence comes from the specification, from the second, independent
implementation (the Jellyfin plugin's SPEC-GAPS.md and DESIGN-NOTES.md),
and from reading Overfin's federation code. Where a proposal says
something was run or checked, it was, on 2026-11-16; where it was only
read or assumed, it says so.

| # | Proposal | Status |
|---|----------|--------|
| 1 | [Strict canonical rules](0001-strict-canonical-rules.md): one Ed25519 rule set (canonical points in the prime-order subgroup), I-JSON, one text per value, refuse the rest | Adopted, R check required |
| 2 | [Opaque library IDs](0002-opaque-library-ids.md): library and version IDs chosen by the owning server, never the media server's | Adopted, choose again |
| 3 | [Key rotation and revocation](0003-key-rotation-and-revocation.md): a rotation signed by the old and new keys, a committed next key, revocation | Adopted whole |
| 4 | [A core plus separately versioned parts](0004-core-and-parts.md): core, `catalog`, `playback`, `media`, announced in `hello`; MSFP names; optional `/.well-known/msfp` | Adopted, MSFP names |
| 5 | [Title identity beyond TMDB, and episode order](0005-title-identity-and-episode-order.md): TMDB's numbering in keys, `with` for double episodes, `e0`, `ids` for IMDb and TVDB | Adopted |
| 6 | [Request signatures vs RFC 9421](0006-request-signatures-vs-rfc9421.md): a comparison; recommends keeping MSFP's scheme | Adopted: own scheme, plus Content-Type |
| 7 | [HTTPS only](0007-https-only.md): drop plain `http`, now that pins allow self-signed certificates | Adopted |
| 8 | [Changing a server's address](0008-address-changes.md): a signed, confirmed address change in `hello` | Adopted |
| 9 | [Private servers](0009-private-servers.md): a server that can't be called says so in `hello` and pulls rotation and revocation (option 1); a reverse connection, the optional part `transport` (option 2) | Option 1 adopted, its open points settled 2026-11-17 (address `msfp-<26>.invalid` required), and written into spec/v1, now spec/v0 (core sections 5.2, 7.5, 9, 12.4, 12.6, 13.2 and others; choices to confirm in core 18, items 29 to 35); option 2 set aside |
| 10 | [The mesh](0010-mesh.md): servers learn that others exist (signed, expiring announcements: key, name, address or "can't be called", versions), never catalogs; knowing is not trusting; spread through pairings (option a), open gossip (b) or a DHT (c); asks that end in an ordinary pairing; optional vouching; the optional part `mesh` | Replaced by proposal 12 (2026-11-20): global discoverability won't be added |
| 11 | [Music](0011-music.md): the part `music`; artists, albums (release groups) and tracks (recordings) by MusicBrainz ID; titles carry their own names; art from the Cover Art Archive; one audio playback per track | Adopted; every type optional (`video`, `music`); open points still open |
| 12 | [Groups](0012-groups.md): servers whose operators chose each other, as a list of signed events every member keeps; an owner, admins, members; one kind of code; requests encrypted for each member; offers and limits per group; the part `groups` | Adopted 2026-11-20, written into spec/v0 (groups.md, core, catalog, playback) |
| 13 | [Version 0](0013-version-0.md): the protocol is a draft until it's stable: every version becomes 0, spec/history/test-draft.md moves to history | Adopted and done 2026-11-20 |
| 14 | [Relations](0014-relations.md): the pairing becomes the low-level link; operators manage relations (an individual one, chosen by both, and one per group); a link lasts while one relation does; `individual` in `hello`; keys forgotten only on blocking | Adopted 2026-11-21, written into spec/v0 (core 7.7) |
| 15 | [Copies in the catalog](0015-copies-in-the-catalog.md): a film's or an episode's copies in `titles` (length, container, picture, tracks, version IDs), so a media server builds its library before anyone plays; pages bounded by size too | Absorbed into 16 (2026-11-22) |
| 16 | [The revision of version 0](0016-version-0-revision.md): one batch from an audit, a survey of six platforms and a test phase: copies described in the catalog (required, with streams, tracks, chapters, markers, editions, size and etag), copies served as files (delivery, lazy playback, restarts, complete HLS), extras, `tvdb` and `local` keys, music keyed by edition and release track, the viewer choosing the method, styled subtitles and fonts, previews before play, downloads with a speed cap, a scan access | Adopted 2026-11-22, written into spec/v0; absorbs 15 |
| 17 | [Neutrality](0017-neutrality.md): the protocol doesn't adapt to products; outside vocabularies (ffmpeg names, RFC 6381, H.273, DASH roles, MusicBrainz types), open lists with one rule for unknown values, exact values, no capabilities (the viewer sends `delivery`; else the H.264/AAC HLS baseline), choice as defaults, product text to implementer notes, schemes in a registry, owner-announced scan cap, 16 copies, 64 fonts, themes allowed | Adopted 2026-11-22, written into spec/v0; amends 16 |
| 18 | [Owner succession](0018-owner-succession.md): the owner's server signs `present` weekly; after 15 days of silence the successor (heir, oldest admin, oldest member) claims the group with backing from a majority of owner-endorsed members; the returning owner is cut back to a member | To be reviewed |
| – | [Clarifications](clarifications.md): 34 small fixes that need only a "yes" | Accepted, all 34 |

Some proposals lean on others; each says how it reads without them:

- 3 uses 1's Ed25519 rules for the statements, and 4 to learn which peers
  support rotation.
- 6's recommendation assumes 1, and 4's core version 1 for its small
  changes.
- 7's migration of existing pairings uses 8.
- 9's option 2 uses option 1's `inbound` member, codes and address rule,
  and 4's parts; its pulled rotation and revocation use 3.
- 10 uses 4's parts and 1's Ed25519 rules for its statements, carries 3's
  rotation and revocation statements, applies 8's address confirmation to
  addresses learnt from the mesh, and 9's `inbound` member when 9 is
  adopted.
- 12 replaces 10; it uses the core's pairing, 3's rotation for members'
  keys, and 9's `inbound` member. It shipped with 13.
- 14 builds on 12: a group is one of a link's relations.
- 18 builds on 12: it replaces the vanished owner's freeze with a claim.
