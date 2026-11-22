# Clarifications

Status: All 34 accepted (2026-11-16).

Small fixes to [spec/history/test-draft.md](../spec/history/test-draft.md) that need no design decision
beyond "yes". Most come from the plugin's SPEC-GAPS.md (cited by its
headings) and state what the plugin assumed; where Overfin does
something else, the item says so, from reading Overfin's code. Gaps that
need a real decision are in the numbered proposals instead:

- encodings, Ed25519, addresses, the code, headers, `Content-Type`
  parameters, the pairing proof's bytes: proposal 1;
- library IDs, and which library a title names: proposal 2;
- episode order, double episodes, `e0`: proposal 5.

Seek-bar previews (SPEC-GAPS.md, "9.5: no way to share seek-bar
previews") are left out: `trickplay` in `play` answers was added to
the specification separately (now [video section 5.5](../spec/v0/video.md#55-seek-bar-previews)).

## Signing and replay (section 6)

1. **The target the receiver rebuilds** (6.2; gap "6.2: the target the
   receiver rebuilds"). Add: "The receiver takes `<target>` from the
   request target as it received it, percent-encoding untouched, not from
   a decoded path. A federation call with a query is refused like a bad
   signature."

2. **When a nonce's 10 minutes start** (6.5; gap "6.5: when a nonce's 10
   minutes start"). Add: "from the moment the request was accepted".

3. **A forgotten key on `pair` while a code is live** (6.4, 6.5; gap
   "6.4 and 6.5: a forgotten key on pair while a code is live"). Add:
   "On `pair`, while a code is live, every key is heard, forgotten ones
   included; otherwise a new code could never restart an ended pairing.
   A successful `pair` takes the key off the forgotten list. On every
   other call, and on `pair` without a live code, a forgotten key gets
   the signed `404`."

4. **Restarts** (6.5; open question 6). Change "Overfin's client doesn't
   retry" into a rule: "A sender whose call to a peer is refused with
   an unsigned `404` should retry it once, a second later, with a new
   nonce. A receiver that just restarted then accepts it." Overfin's
   client would need this retry.

## Transport and refusals (sections 3 and 8)

5. **`415` and `403`** (3; gap "3: status codes for a missing
   Content-Type and for browsers"). Move both to a version 0 note: "Other
   implementations answer these with the unsigned `404` of a failed
   check. Overfin answers `415` and `403`, from its web stack."

6. **The body of an unsigned `404`** (3, 6.4, 14; gap "3 and 14: the same
   404 as a path that doesn't exist can't hold on Jellyfin"). Add: "The
   body of an unsigned refusal is not specified; a receiver should send
   whatever its host sends for a path that doesn't exist, so the two
   can't be told apart. Where the host can't, it sends the problem
   document of section 8.2." Section 14's "Probing" then says "should".

7. **A `409` body without a `type`** (9.5; gap "9.5: not-playable and
   stream-limit-reached"). Add: "A body that isn't JSON counts as a body
   without a `type`."

## Pairing and states (section 7)

8. **A code from a peer that is `gone`** (7.1 step 2; gap "7.1 step 2: a
   code from a peer that is gone"). Add: "Removing a peer, in any state,
   is an ending (section 7.4). To pair again with a `gone` peer, the
   operator removes it first. Redeeming a code takes its key off the
   forgotten list."

9. **The forgotten list** (7.4; gap "7.4: the forgotten list has no
   end"). Add: "Entries may be removed after a year. A removed entry only
   turns that key's signed `404` into an unsigned one."

10. **The receiver of `ended`** (7.4 step 3; gap "7.4 step 3: what the
    receiver of ended keeps"). Add: "It deletes the ender in every state,
    waiting ones included, and doesn't record it as forgotten: the ender
    has deleted it too, so nothing more passes between them."

11. **"Last heard from"** (7.2; gap "7.2: what last heard from counts").
    Add: "any request from the peer that passed section 6.4, and any
    answer from it whose signature verified".

## The calls (section 9)

12. **Order of `pair`'s checks** (9.1; gap "9.1: the order of the 404
    and 409 checks"). Add: "The maker checks in the order of section
    7.1, step 4. A known key gets `409` only with a valid code and proof;
    otherwise a signed `404`. A `409` leaves the code live."

13. **Cleaning names** (9.1; gap "9.1: cleaning names"). Replace the
    sentence with: "Every run of characters with the Unicode
    `White_Space` property becomes one space; then other Cc and Cf
    characters are removed; both ends are trimmed; the result is cut to
    80 code points, and trimmed again. An empty name becomes the host of
    the peer's address, with its port, as in the request message."
    Overfin's `cleanName` removes control characters first, so a tab
    between two words joins them; it would follow this order (two
    lines).

14. **Library names are plain text** (9.3; gap "9.3: names are made
    plain text"). Add: "Names are text, never markup. A receiver cleans
    them as in section 9.1, and treats them as text wherever it shows
    them; it may clean further." Overfin's receiver also strips HTML tags
    and decodes entities (`textclean.Plain`); that remains allowed.

15. **`hello` without a `state`** (9.2; gap "9.2: a hello without a
    state"; open question 10). Add: "A body without `state`, or with a
    `state` that isn't a string, is refused like a body that isn't JSON.
    An unknown string is answered like `waiting`."

16. **`stop`'s position** (9.7; open question 1). Cap `positionMs` at
    8,640,000,000, as in `play` and `progress`. Overfin's `stop` checks
    only `< 0` today.

17. **Libraries: duplicates, `mixed`, a bad body** (9.3; gap "9.3:
    duplicate IDs, the mixed kind, and a bad body"). Add: "A caller keeps
    the first of two libraries with the same `id`. `mixed` is for a
    library holding several kinds; `""` is for kinds outside the list. A
    `libraries` body that isn't a JSON object is refused with a signed
    `404`."

18. **A changes reading after the caller's wants changed** (9.4; gap
    "9.4: a changes reading after the caller's wants changed"). Add, for
    callers: "Ask with `since` only when `libraries` is exactly the set
    of the reading that gave that `seq`; otherwise ask for the whole
    list."

19. **`gone` counts toward the 5,000** (9.4; gap "9.4: does gone count
    toward the 5,000?"). Add: "In a changes answer, `titles` and `gone`
    together hold at most 5,000 keys, taken in one byte order." Overfin
    already does this (`ChangedTitles` pages over both).

20. **A bad `since`** (9.4; gap "9.4: what a bad since is"). Add: "A
    `since` that isn't an integer is refused like a bad body (signed
    `404`). A negative one is answered with the whole list, like 0."

21. **Language tags** (9.4; gap "9.4: language tags"). Add: "A tag uses
    the shortest code BCP 47 allows: the ISO 639-1 code where one exists
    (`en`, `fr`), else ISO 639-2/T or 639-3 (`yue`), with a region when
    known (`fr-CA`). `und`, `mul`, `zxx` and `mis` are not sent. A
    receiver skips a bad tag, not the whole title." (Case: proposal 1,
    C5.)

22. **Languages of series and seasons** (9.4; gap "9.4: languages of
    series and seasons"). Add: "Series and seasons carry no `audio` or
    `subtitles`; films and episodes do."

23. **TMDB season IDs** (11; gap "11: TMDB season IDs aren't used").
    Add: "A season is named by its series' ID and its number, never by
    TMDB's season ID."

## HTTPS certificates (section 3.1)

24. **Several certificates at one address** (gap "3.1: one pin read,
    several certificates served"). Add: "A server that serves several
    certificates (RSA and ECDSA, several machines, a renewal rolling out)
    names all their pins; one reading sees only one, so such a server
    should take its pins from its configuration."

25. **SNI when reading one's own certificate** (gap "3.1: what to send as
    SNI"). Add: "with SNI set to the address's host name; with none for
    an IP address (RFC 6066)".

26. **A failed reading** (gap "3.1: a failed reading of one's own
    certificate"). Add: "A failed reading keeps the last good one (none
    if there was never one); the next attempt waits a minute."

27. **`certificates` that isn't an array** (gap "3.1 and 9.2: a
    certificates field that isn't a list"). Add: "In a request, the body
    is refused like a bad body, and the caller's pins are unchanged. In a
    signed answer, it counts as no pins."

28. **Which 8 pins** (gap "3.1: which 8, and duplicates"). Add: "the
    first 8 valid entries, each pin once, in the order sent".

29. **The latest `hello`** (gap "3.1: the latest signed hello, when two
    cross"). Add: "latest by arrival".

30. **Pins that "differ"** (gap "3.1: when pins differ"). Add: "compared
    as sets".

31. **Connections opened under removed pins** (gap "3.1: connections
    opened under pins that were since removed"). Add: "When a peer's pins
    change, the client closes connections to it that were accepted only
    by a pin it no longer names."

32. **Pins of a `gone` peer** (gap "3.1: a signed 404 to hello and
    pins"). Add: "A signed `404` leaves the stored pins as they were."

33. **One `hello` per peer for refused media** (gap "10: one hello per
    refused media fetch"). Add: "A client sends at most one such `hello`
    to a peer at a time; fetches that wait on it share its result."

34. **Split-horizon names** (14; gap "14: reading one's own certificate
    trusts the path to oneself"). Add to section 14: "A server that is
    served another certificate inside its network than outside names the
    inside one, and its peers are refused on every call. Nothing is
    opened to an attacker, but calls fail; set the pins by hand."
