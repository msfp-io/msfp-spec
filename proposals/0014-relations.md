# Proposal 14: Relations

Status: Adopted (2026-11-21), with the owner's choices on the open points
(see the end): the name "individual", keys forgotten only on blocking,
fingerprints offered. Written into spec/v0 on 2026-11-21 (core 7.2,
7.4, 7.7 and 9.2; groups 1, 2, 4, 6, 7, 9; playback 5; catalog). From the owner's
model of 2026-11-21: "the direct pairing itself is a low-level unit. It
doesn't necessarily have to be controllable by the user directly.
Individual and group pairings are higher-level relations, which the user
creates and manages. The pairing exists as long as there is a relation
between both peers."
Touches: [core](../spec/v0/core.md) sections 7 and 9.2 (`hello`), 17.1
(its vector); [groups](../spec/v0/groups.md) sections 1, 2, 4, 6, 7, 9
and 10; [playback section 5](../spec/v0/playback.md#5-limits); the
conformance suite.

## Problem

MSFP already has the idea underneath. A **link** (a pairing, core section
7) is kept as long as something holds it, and each server keeps, per
link, its **sources**: a pairing by hand, and each group the two servers
pair through (groups section 1). But the sources are an internal detail.
What the operator sees and acts on is the link itself, and that gives
four problems.

1. **Ending is all or nothing.** "End the pairing" ends the link whatever
   holds it (core section 7.4), and records the peer's keys as
   forgotten. On 2026-11-21 the owner ended zecopter's pairing with its
   Jellyfin server. The pairing had been made through a two-server group,
   "patate", so the group lost its only link. Neither server could reach
   the other through it any more, and their requests to pair again were
   kept as "sent" but never arrived. That needed two new rules the same
   day: "No way in" (groups section 6) and "A member back in" (groups
   section 2). Both are repairs for an action that should not have cut
   the group in the first place.
2. **Joining a group creates a pairing by hand.** The newcomer's link to
   its inviter has two sources, "the pairing by hand and the group"
   (groups section 2). So the inviter appears in both panels (Overfin's
   and the Jellyfin plugin's) next to the servers the operator paired with
   themselves, and it stays after the newcomer leaves the group. The
   owner found it "bizarre": the operator made a group relation, not an
   individual one.
3. **Relations nobody chose.** When a member leaves without ticking its
   links (groups section 7), each one held only by the group "becomes a
   pairing by hand". The other side does the same when it receives
   `left`: msfp-go (`groups.go`, after a member's events) turns that link
   into one by hand and copies the group's offers and pool into it. Both
   operators now have an individual pairing neither of them asked for.
4. **Sources are per side and never told.** Each server keeps its own
   sources, and nothing on the wire carries them. One side can count a
   pairing by hand the other doesn't, and neither panel can show the
   relation as both sides see it.

## Proposal

### Terms

- **Link**: unchanged, the pairing of core section 7: keys, addresses,
  states, rotation, revocation, calls. It is the low-level unit, and the
  panels don't present it as something to manage.
- **Relation**: why two servers are linked. There are two kinds:
  - the **individual relation**, chosen by both operators;
  - a **group relation**, one per group the two servers share and are
    linked through.
- A link exists while the two servers have **at least one relation**.
  When the last one goes, the link ends.
- Relations are **mutual**: both servers hold the same set. An individual
  relation exists only when both sides want it. A group relation exists
  when both are members and their link was made or kept through that
  group, as groups section 1 already defines it.

The groups part's "sources" become relations, and "a pairing by hand"
becomes "the individual relation", everywhere.

### What each side says: `individual` in `hello`

The signed `hello` body (core section 9.2) gains one member,
`individual`, a boolean: whether this side wants the individual
relation. The individual relation holds when both sides' latest `hello`
say `true`. A server sends `hello`
at once when its own `individual` changes, as it does for a new address
(core section 13).

The table below shows how `individual` behaves in each case.

| Situation | Our `individual` | Theirs | Relation | Panel |
|---|---|---|---|---|
| Paired with a code, both accepted | `true` | `true` | individual | Servers: paired |
| We ask a group member for an individual relation | `true` | `false` | asked | Servers: "asked" |
| A group member asks us | `false` | `true` | none yet | Servers: "asks you", Accept or Refuse |
| Either side ends it | `false` | either | none | gone from Servers |

A server that can't be called (core section 7.5) carries `individual` in
the `hello` it sends on schedule, as it carries everything else.

No new call is needed. `hello` already brings both sides to the same state
(core section 9.2), and an asked relation that is never accepted costs
nothing.

### What changes in each flow

1. **Pairing with a code (core section 7.1).** Both sides start with
   `individual: true`. The code is the consent, as today. A plain code gives
   an individual relation and nothing else.
2. **A group's code (groups sections 2 and 4).** The newcomer's link to
   its inviter has the group relation only, with `individual: false` on both
   sides. The inviter no longer appears among the newcomer's individual
   relations, and leaving the group ends that link like any other group
   link.
3. **Asking for an individual relation.** Between two servers already linked
   through a group, either operator asks, and the other accepts or
   refuses in its panel. No code is needed: the keys are already trusted
   through the group. The panel offers a fingerprint comparison then, and
   doesn't require it, because a group link never had one (groups section
   11, "Trust passes through the inviter").
4. **Ending the individual relation.** The server sends `hello` with
   `individual: false`. If a group relation remains, the link stays. If
   none remains, the link ends as core section 7.4 says, without
   keeping the server away (see "Forgetting" below).
5. **Leaving a group (groups section 7).** The group relation goes, on
   both sides, as today. A link with no relation left ends, and nothing is
   turned into an individual relation any more. The leave dialog lists the
   links that would end and offers to **ask** those servers for an individual
   relation first. The links that are asked stay until the answer: an
   accepted one becomes individual, a refused one ends.
6. **Banning and dissolving.** Same rule: the group relation goes, and a
   link with no relation left ends. `keepPairings` on `dissolved`
   (groups section 7) no longer creates individual relations: with it,
   each link held only by the group asks for an individual relation on
   both sides, as in item 5, and stays until both answer. One that both
   accept becomes individual, and one that either refuses ends.
7. **Blocking** stays a link-level action: it ends the link whatever its
   relations, and it is the only action that keeps the server away from
   every group.

### Forgetting

Only **blocking** keeps a server away. Every ending still records the
peer's keys on the forgotten list, as core section 7.4 says, so that the
peer's later calls get a signed `404` and it learns the pairing ended.
But only a block marks them **blocked**, and only blocked keys are kept
away from groups: their requests are dropped and their `group-pair`
refused. Any other ended key comes off the list as soon as the two
servers are linked again, with a code or through a group they share.
Panels never show the list: the operator sees "End" and "Block".

### What belongs to a relation and what to the link

- **Each relation has:** what this server offers the peer, and the
  limits it grants. The individual relation keeps its own offers and limits,
  as the pairing by hand does today. A group relation takes the group's
  offers and pool. The rules of groups section 9 stay as they are, with
  "pairing by hand" read as "individual relation": the link is offered the
  union of its relations' offers, and the first pool with room counts.
- **The link has:** the keys, the fingerprint, rotation and revocation,
  the address, what this server reads and shows of the peer's titles
  (its wants), and blocking.

### How the panels show it

These rules are for the panels, not the wire.

- **Servers** lists the individual relations, the asked ones, and the
  requests waiting for an answer.
- **Each group** lists its members, each with its link state (linked,
  asked, not linked) and the same window a server row opens: what you
  offer, what you show of it, its key, and Block.
- **A server in several relations** appears in each place, with the
  others named on its row, for example "also in Family".
- **Ending acts on one relation:** "End our individual relation", "Leave
  Family". The link itself has only Block.
- **"No way in"** (groups section 6) stays for the cases that remain, but
  ending an individual relation no longer causes it.

## Alternatives considered

- **Change the panels only.** Hiding sources from the panels doesn't
  make ending act on one relation: "End" would still end the link (item
  1), and leaving would still create individual pairings nobody chose (item
  3).
- **A separate `relation` call** instead of a member of `hello`: more
  explicit, but `hello` already carries each side's state, already
  brings both sides to the same state, already reaches servers that
  can't be called, and is already signed. A separate call would repeat
  all of that.
- **Keep sources per side, and let "End" remove only our own source.**
  Then each server ends up with a different picture: one side sees no
  individual relation while the other keeps offering through one. That is
  item 4, made worse.

## Cost

- **Specification:** core 7 and 9.2, the `hello` vector of section 17.1,
  groups 1, 2, 4, 7, 9 and 10, playback 5, and the conformance suite's
  `hello` checks. "Pairing by hand" is renamed throughout.
- **msfp-go and msfp-dotnet:**
  - keep each side's `individual` flag, and send and read it in `hello`;
  - add three operations: ask for an individual relation, accept or refuse
    one, and end one;
  - end a link when its last relation goes;
  - no longer turn group links into individual ones;
  - change the group code's newcomer link to the group relation only.
  - A migration: a link with a pairing by hand today gets `individual: true`
    on that side, and learns the other side's flag at the next `hello`.
- **Overfin and the plugin:** both panels rebuilt around relations, as
  above (Servers, each group, the server window, the leave dialog), and
  the same migration through their engines.

It is a medium change, mostly in the engines and the panels. The groups
mechanics (events, mail, `group-pair`) don't change.

## Compatibility

MSFP version 0 is a draft (proposal 13), so this ships in one release of
every implementation, as proposal 12 did. During the update, a `hello`
without `individual` is read as "unchanged": the peer's last known flag, and
for a peer never heard with it, the same as ours. So a link already paired
by hand on both sides stays individual when only one side has updated.
Links that only one side counted as by hand, such as the ones created
when a member left (item 3), show as "asked" on that side after the
update, and its operator chooses to keep or end them.

## Decisions (2026-11-21)

1. **Forgetting keys:** only blocking keeps a server away. Written into
   the spec as: every ending still records the keys, for the signed
   `404`, but only blocked keys are kept away from groups, and an ended
   key comes off the list when the two servers are linked again.
2. **`keepPairings` on `dissolved`:** each link held only by the group
   asks for an individual relation, both sides still accepting. This was
   the proposal's lean, and the owner made no other choice.
3. **Fingerprints on a new individual relation** between group members:
   offered, not required.
4. **The name:** "individual", on the wire (`individual` in `hello`) and
   in the panels.
