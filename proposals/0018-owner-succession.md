# Proposal 18: Owner succession

Status: To be reviewed (2026-11-22). Nothing here is decided.

Builds on [proposal 12](0012-groups.md) (groups) and amends it: it
reverses its alternative 6 ("an officer acting while the owner is down").
The idea of a successor claiming the group with backing from the other
members, and of fencing the returning owner, comes from jellymesh's
design specification (washburnello/jellymesh, `docs/design-spec.md`,
section 8, "Succession and fencing"), read on 2026-11-22. Its mechanism
doesn't carry over as it is (see Alternatives considered); this is an
adaptation to the groups part.

Touches: [groups](../spec/v0/groups.md) sections 1, 3.1, 3.3, 3.5
("A vanished owner"), 10 and 11; its vectors; the conformance suite. The
core, the calls of the groups part and the other parts are untouched.

## Problem

A group has one owner, and only the owner names and removes admins,
lifts bans, changes the settings, gives the group away and dissolves it
(groups section 1). Handing over is already covered: the owner names an
**heir** (`heir`), and **stepping down** (`owner`) goes to the heir, or
else the oldest admin, or else the oldest member (groups section 3.3).

But every handover needs the owner's signature. When the owner's server
is gone for good (the machine died with its key, its operator stopped
running it, the operator died), groups section 3.5 says:

> **A vanished owner** (its key lost, its server gone, without giving the
> group away) can't be replaced: nobody can tell for every member that it
> is gone. Its powers freeze: no admin named or removed, no ban lifted, no
> setting changed.

So a group outlives its owner only half-way. A hacked admin can't be
removed any more, and a wrong ban can't be lifted. The heir that the
owner named can't take over, which is exactly the case an heir is for.
The members' only way out is to make a new group and invite everyone
again.

Proposal 12 dropped an officer that acts while the owner is down, because
"'down' can't be decided for every member without an authority". That
holds as long as being down is about reachability. Members link only
with members they accepted (groups section 4.1), and blocking is private
(section 7), so "I can't reach the owner" is a statement no other member
can check. This proposal makes the owner's absence something every copy
of the list can see instead.

## Proposal

### 1. The owner's server shows it is there

The owner's server signs a **`present`** event when it has signed no
event in the group for 7 days. It has no members of its own, and it isn't
an owner event: it carries no `ownerSeq`, like `profile`. The owner's
server signs it by itself; its operator does nothing.

The cost is one event a week per group, about 300 bytes. An owner that
is active signs nothing extra.

The owner's **last sign of life** is the owner's latest event in the
list, whatever its type: the highest `seq` of the owner's member ID that
the copy kept, and that event's `date`. The owner is **silent** when its
last sign of life is more than 15 days old (the **window**).

### 2. The successors

The successors are the same as for stepping down, in the same order:

1. the heir, if the owner named one and it is still a member;
2. then the admins, oldest first (named at the lowest `ownerSeq`);
3. then the other members, oldest first (the lowest `joined` `date`,
   then the lowest event ID).

Banned members, members that left and the owner itself are skipped. The
successor at place *n* (from 1) may claim once the owner has been silent
for *n* windows: the heir after 15 days, the next one after 30, and so
on. If the first successors are gone too, the group still finds an
owner, but later ones wait longer.

The order is computed from the state, so every copy agrees on it.

### 3. Backing

Every member's server watches the owner's last sign of life in its own
copy. Once the owner has been silent for a whole window, and its server
has completed a `group-sync` within the last 24 hours (so the silence it
sees isn't its own staleness), it signs a **`backed`** event:

| Member | Type | Content |
|--------|------|---------|
| `owner` | string | the silent owner's member ID |
| `last` | string | the ID of the owner's last sign of life in this copy |
| `lastSeq` | integer | that event's `seq` |

The backing says: "at my `date`, the owner's latest event I had was
`last`". It is signed automatically, like `present`, and passed on like
any event. A server signs at most one `backed` per `last`; if a newer
sign of life arrives later, that backing no longer supports any claim
(section 4).

**Who counts.** Only **owner-endorsed members** count as backers: the
members whose `joined` was signed by the owner of the time, the
founder, and the admins in force. Members invited by an admin don't count. Without
this, a hacked admin could invite servers of its own, up to its daily
cap (5 a day by default, 75 over a window), and gather a majority of
fakes. Only the owner invites the members that count, or names the
admins that invite, so the vote stays among servers the owner chose.

### 4. The claim

The successor whose turn has come signs a **`claimed`** event. It is an
owner event: it takes an `ownerSeq`, the one after the highest it kept.

| Member | Type | Content |
|--------|------|---------|
| `owner` | string | the silent owner's member ID |
| `last` | string | the ID of the owner's last sign of life in the claimant's copy |
| `lastSeq` | integer | that event's `seq` |
| `backing` | array of string | the IDs of the `backed` events it relies on, at most 64 |
| `ownerSeq` | integer | as for any owner event |

A `claimed` is ready (groups section 3.5, step 1) when the owner's events
up to `lastSeq` and every event in `backing` have been replayed. It
counts when, at its turn:

1. its `owner` is the owner at its `ownerSeq`;
2. its signer is the successor whose turn has come: the claim's `date`
   is at least *n* windows after the `date` of `last`, *n* being the
   signer's place;
3. a strict majority of the owner-endorsed members, other than the
   silent owner, supports it: each `backed` in `backing` counts when its
   signer is an owner-endorsed member, its `owner` is the claim's, its
   `lastSeq` is at most the claim's `lastSeq`, and its own `date` is at
   least one window after the `date` of its `last`. The claimant counts
   as one voice if it is owner-endorsed. A member counts once, however
   many of its backings the claim names, and the silent owner never.

When it counts, the signer is the owner from that `ownerSeq` on, as if
the silent owner had signed `owner` toward it, and the claim **cuts**
the silent owner (section 3.5, step 3): its owner events after
`lastSeq` don't count. Its other events (`profile`, `present`, `left`,
`rotated`, `revoked`) still do: it stays a member.

**Examples.** With 3 owner-endorsed members besides the owner, a claim
needs 2 voices: the heir and one backing. With 1 (the owner and one other
member), the claimant alone is a majority, and no backing is needed:
there is nobody who could disagree. With 20, it needs 11: the claimant
and 10 backings.

A claim that doesn't count at its turn is replayed without counting, as
any event whose rules don't hold. The successor may claim again later,
with more backing or a newer `last`. Two claims for the same `ownerSeq`
are settled as any two owner events are: the first that counts takes it,
and the other doesn't.

### 5. The owner coming back

The owner's returning server receives the claim like any event (groups
section 6). Its owner events signed since `lastSeq` no longer count, and
it is an ordinary member: its panel says who took over, on what date,
and that its server had signed nothing for so long. The new owner may
hand the group back with an `owner` event; nothing does it automatically.

If the owner was only cut off (its server alive but reaching nobody),
its `present` events didn't reach the members either, so the members saw
what a vanished owner looks like. That is the price of deciding without
an authority, and the reason the window is long.

### 6. In the groups part

- **Section 3.5, "A vanished owner"** is replaced by: "A vanished owner
  is replaced by a claim (section 3.7). While no claim counts, its
  powers freeze: …" (the rest of today's text).
- A new **section 3.7, Succession**, with sections 1 to 5 above.
- **Section 3.3** gains the three types: `present`, `backed` and
  `claimed`.
- **Section 10**: each member's panel shows the owner's last sign of
  life once the owner has been silent for 7 days, and the claim when
  there is one.
- **Section 11** gains: "**A hacked successor** can take the group only
  with a majority of the owner-endorsed members, each of whose copies
  saw the owner silent for a whole window: hiding the owner's events
  from the successor isn't enough. **A hacked owner key** keeps the
  owner present, as it can today: that is what revocation is for (core
  section 12.6)."

## Alternatives considered

1. **No change**: the owner's powers freeze, as today. A group survives
   the loss of its owner only as long as its admins behave, and a wrong
   ban or a hacked admin stays forever.
2. **jellymesh's attestations of unreachability**: each member states
   that it can't reach the owner. Set aside: MSFP members aren't all
   linked to the owner, and blocking is private, so the statement can't
   be checked, and a member that blocked the owner would always be ready
   to attest.
3. **A majority of all members**: simpler, but members invited by a
   hacked admin would count (section 3, "Who counts").
4. **Admins only**: a group without admins (the owner invites everyone)
   would have no voters. Owner-endorsed members cover that group, and
   include the admins.
5. **A pre-signed handover**: the owner regularly signs an `owner` event
   toward its heir, dated in the future, and the heir publishes it if the
   owner stops renewing it. No backing needed, but the heir alone
   decides, a hacked heir takes the group at the first missed renewal, and
   every copy has to order handovers by their dates.
6. **Votes on a new owner**, chosen by members: proposal 12 dropped votes
   (its alternative 5): they need every operator at their panel. Here the
   successor is known in advance and the backing is automatic.
7. **Jellymesh's single sequencer and epochs**: MSFP's list has no
   sequencer (every member numbers its own events), so there is nothing
   to fence but the owner's own events, which the cut already does.

## Cost

- **Specification:** groups 3.3, 3.5, a new 3.7, 10 and 11; vectors (a
  `present` restarting the window, a claim before its time, one
  without a majority, one with backings from admin-invited members, a
  backing that saw a newer sign of life, two claims for one `ownerSeq`,
  the cut on a returning owner's events); conformance checks.
- **msfp-go and msfp-dotnet:** the three types in the state; the weekly
  `present`; the watch on the owner's last sign of life, and the
  automatic `backed`; the claim, signed when the successor's turn comes
  and the backing is there. A small change: no new call, no new message.
- **Overfin and the Jellyfin plugin:** two notices in the group's
  window (the owner has been silent since a date; who took over), and the
  explanation of the heir, which now also takes over when the owner
  vanishes.

## Compatibility

MSFP version 0 is a draft (proposal 13), so this ships in one release of
every implementation. An implementation without it keeps the new types
and passes them on, but counts none of them (groups section 3.3): its
state would keep the old owner while the others' moves on. So all
implementations must update before any group relies on it.

A group whose owner vanished before the update can be claimed at once:
the owner's last sign of life is its latest event, however old.

## Open points

1. **The values:** a 7-day `present`, a 15-day window, 24 hours since the
   last `group-sync` for backing. Fixed values, or a group setting? The
   deprecated `waitDays` setting (groups section 3.3) is free to become the
   window.
2. **Automatic backing**, or the operator's yes in the panel? Automatic
   is what makes it work when operators are away; a yes protects against
   a silence that its operator knows is temporary (the owner moving house).
3. **Who counts:** owner-endorsed members (this proposal), or every
   member that joined before the owner went silent? The second needs a
   way to date a `joined` that its signer can't backdate, which the list
   doesn't have.
4. **At most 64 backings** in a claim keeps it under 4,096 bytes; a
   group with more than 129 owner-endorsed members besides the owner
   couldn't gather a majority. A limit to accept, or a claim in several events?
5. **The names** `present`, `backed` and `claimed`.
