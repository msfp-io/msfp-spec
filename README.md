<img src="logo.png" alt="" width="64" align="left">

# MSFP: Media Server Federation Protocol

MSFP is a protocol between media servers. Two servers pair, then each
reads the other's catalog and streams the titles it wants from it. Every
call is an HTTPS `POST` with a JSON body, and every request and answer is
signed with the server's Ed25519 key. Titles are named by public IDs
(TMDB for films and series, MusicBrainz for music), so two servers agree
on what a title is without sharing files or accounts. Servers pair by
hand, from a one-line code, or through a group.

**Status: version 0, a draft.** Anything may change between drafts;
version 1 will be the first stable version.

## Start here

Read [spec/v0/README.md](spec/v0/README.md): it lists the core and the
parts, and has a glossary. Then read the core.

## This repository

| Folder | What it holds |
|--------|---------------|
| [spec/](spec/v0/README.md) | the specification: version 0, and earlier drafts as history |
| [proposals/](proposals/README.md) | changes to the specification, adopted or under discussion |
| [vectors/](vectors/README.md) | signed sample requests and answers, valid and invalid, for an implementation's tests |
| [conformance/](conformance/README.md) | a test suite that pairs with your server as a peer and checks it against version 0 (Python 3, standard library only) |
| [api/](api/README.md) | the application-side API of the MSFP libraries (informative, not part of the protocol) |
| [tools/](tools/vectors/) | the generator of the test vectors |

## Implementations

Libraries, which do the protocol for a media server:

- [msfp-go](https://github.com/msfp-io/msfp-go), in Go;
- [msfp-dotnet](https://github.com/msfp-io/msfp-dotnet), in C#.

Servers:

- [Overfin](https://github.com/Overfin/server), on msfp-go;
- [jellyfin-plugin-msfp](https://github.com/C0PT3R/jellyfin-plugin-msfp),
  a Jellyfin plugin on msfp-dotnet.

## Questions and changes

Questions and gaps in the specification are welcome as issues. A change
to the protocol starts as a proposal in [proposals/](proposals/README.md).

## License

- The specification and this README: [CC BY 4.0](LICENSE). Copy, translate
  or adapt them, with credit. Implementing the protocol needs no permission
  and carries no condition.
- The test vectors (`vectors/`): [CC0 1.0](vectors/LICENSE), no conditions.
  Copy them into any test suite.
- The conformance suite (`conformance/`): [MIT](conformance/LICENSE), and
  its bundled Ed25519 reference code the IETF Trust's Simplified BSD
  License.
