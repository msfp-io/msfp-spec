# MSFP registry

The registry moved to MSFP Media with
[proposal 19](../proposals/0019-msfp-media.md):
[MSFP Media's registry](https://github.com/msfp-io/msfp-media/blob/main/spec/registry.md)
lists the values of the open fields that describe media (codecs,
subtitle formats, container media types, `hdr`, `edition`,
`extraType`, segment kinds, roles, `objects`, `threeD`, music types)
and the schemes of title keys and of `ids`, for every version of MSFP
Media, and so for every version of MSFP that carries it. A new codec or
scheme is an entry there.

MSFP has no entries of its own. What it adds to the language, the
purpose `scan` and its ticket kind, is in
[playback section 8](v0/playback.md#8-scan-access) and
[media section 4.2](v0/media.md#42-scans).

The registry as it stood before the move, made by
[proposal 17](../proposals/0017-neutrality.md), is in this repository's
history.
