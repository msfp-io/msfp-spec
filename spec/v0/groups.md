# MSFP version 0: the groups part, version 0

**Draft.**

The `groups` part lets servers whose operators chose each other federate
together without pairing every pair by hand. A **group** is a list of
signed events that every member keeps and passes on: no central
authority, no register, nothing that depends on one server staying up.
Joining a group starts like any pairing, from a code its maker accepts
(core section 7.1); the newcomer then asks every member to pair, each
member answers for itself, and the pairings that follow carry everything
a pairing made with a code does. Nothing about a group is visible from outside: a
server that isn't a member gets the plain `404` of
[core section 6.4](core.md#64-checking-a-request).

It needs the [core](core.md) only. Its calls follow
[core section 8](core.md#8-common-rules-for-every-call): signed `POST`s
under the peer's base path. Offers and limits per group change how a
server computes what it offers a peer and what it lets a peer use; the
calls of [catalog](catalog.md) and [playback](playback.md) stay the same
(section 9). It was adopted from
[proposal 12](../../proposals/0012-groups.md).

There is no global discoverability in MSFP: a server that wants to be
known widely publishes itself on the internet, as anyone can. A wider
federation grows through groups: servers in several groups bridge them
without mixing them.

The words "must", "must not" and "should" carry their usual meaning in
specifications. In the examples, A is the owner of the group "Family", B
an admin, C and D members, and N a server joining.

## Contents

1. [Terms](#1-terms)
2. [Codes for a group](#2-codes-for-a-group)
3. [Events](#3-events)
4. [Joining](#4-joining)
5. [Requests and answers: the mail](#5-requests-and-answers-the-mail)
6. [Passing the list on](#6-passing-the-list-on)
7. [Leaving, blocking, banning, dissolving](#7-leaving-blocking-banning-dissolving)
8. [The calls](#8-the-calls)
9. [Sharing and limits](#9-sharing-and-limits)
10. [What members see](#10-what-members-see)
11. [Security considerations](#11-security-considerations)
12. [Open questions](#12-open-questions)

## 1. Terms

- **Group**: a set of servers, its **members**, with a name, an owner,
  admins and settings. A server may be in any number of groups; a group
  never learns another's members.
- **Owner**: the one member that names and removes admins, lifts bans,
  sets the group's name and settings, dissolves the group, names its
  heir, gives the group away or steps down; it also invites and bans, as
  admins do. Exactly one at a time. It can't be banned. The server that
  founds a group is its first owner; "creator" is not a role.
- **Admin**: a member that may invite and ban members. Any number.
- **Member**: any server in the group, the owner and admins included.
  Members other than the owner and admins don't invite: an owner that
  wants everyone to invite makes everyone admin.
- **The list**: the group's events (section 3). Each member keeps its own
  copy and passes events on (section 6). The group's **state** (members,
  roles, settings, bans) is a function of the events: two copies holding
  the same events compute the same state.
- **Link**: a pairing between two servers (core section 7), whatever made
  it. Two servers are linked once, however many groups they share. Each
  server keeps, per link, its **relations** (core section 7.7): the
  individual relation, and each group the two servers share and are
  linked through. A link lasts while one of them holds.
- **Mail**: a message from one member to another, carried by the members
  between them and readable by its recipient only (section 5).

## 2. Codes for a group

A group's invitation is a pairing code with the group attached
([core section 5.1](core.md#51-format)): the same code, made and used the
same way as a plain code. An admin or the owner makes it,
with a number of uses (1 by default) and an expiry (24 hours by default,
at most 30 days). A server that uses it calls `pair` on the code's maker
(core section 9.1), and the maker's operator accepts it after the
fingerprint check (core section 7.1, step 6): using a code is never
enough on its own. Its panel says what the code leads to ("join the group
Family, invited by B").

**After the maker's operator accepts** a server that used a group code,
and the two are `paired`, the maker signs a `joined` event for it
(section 3.3), if the maker still holds the right to invite and its daily
cap allows. Otherwise the link has no relation yet: the maker asks the
server for the individual relation (core section 7.7), and its panel says
the server couldn't be added to the group. A group code
made by an admin stops working when the admin loses its role or is
banned; one made by any member, when the group is dissolved or its maker
leaves it.

The newcomer is then paired with its inviter, a link whose one relation
is the group (neither side wants the individual relation, core section
7.7), and receives the list through it
(section 6). Until it is accepted, it is nothing to the group.

**A server already paired with the code's maker** doesn't call `pair`
(it would get `409`): it calls `group-join` on the maker, through their
pairing (section 8.4), with the code's secret hash and a proof. The code's
use counts as for `pair`. The maker's operator accepts it as any server
that used its code, the fingerprint there to check, or refuses it; then,
as above, the maker signs `joined`, and the list follows through the
pairing, which gains the group as a relation. A refusal sends nothing: the
caller's operator gives up on its side, and a caller still waiting drops
the group 30 days after calling (a code works 30 days at most).

**A member back in.** A member with no link through the group to the
code's maker (section 6, "No way in") may use the group's code too: by
`pair` when the two aren't paired, by `group-join` when they are paired
some other way. Nobody signs `joined`: the server is a member already.
Once the maker's operator accepts, their link gains the group as a
source, and the list follows through it, as for a newcomer. The member
adds the group to the link when it is made (`pair`), or when the maker's
first `group-push` arrives (`group-join`). A member that can't be called
gets no push: it uses `pair`, ending the other pairing first. Through a
link that already has the group, the code is refused as for any member.

## 3. Events

### 3.1 The envelope

An event travels as a JSON object:

| Member | Type | Content |
|--------|------|---------|
| `event` | string | base64url of the event's **payload**: a JSON object, I-JSON ([core section 2.3](core.md#23-json)), at most 4,096 bytes |
| `signature` | string | base64url of the Ed25519 signature ([core section 2.1](core.md#21-ed25519)) of the bytes `msfp-group-event-1\n` followed by the payload's bytes |

The payload is signed as bytes and never rebuilt: every member keeps and
passes on the exact bytes. An event's **ID** is SHA-256 of its payload's
bytes. A **group's ID** is the ID of its `founded` event. Both are written
base64url (43 characters).

Every payload has:

| Member | Type | Content |
|--------|------|---------|
| `group` | string | the group's ID; absent in `founded` only |
| `type` | string | one of section 3.3 |
| `signer` | string | the key that signed, base64url of 32 bytes |
| `member` | string | the signer's member ID (section 3.4) |
| `seq` | integer | the signer's own count of its events in this group, from 1, without gaps |
| `date` | integer | Unix seconds when it was signed |

**Owner events** (the types of section 3.3 signed by the owner) also
carry `ownerSeq`: the group's count of owner events, from 1 (`founded`),
whoever the owner is. **Admin events** (`joined` and `banned` signed by
an admin) also carry `asOf`: the highest `ownerSeq` the admin kept when
it signed.

### 3.2 Keeping an event

A member keeps an event, and passes it on, when:

1. the envelope and the payload are well formed, and its signature
   verifies under `signer` by core section 2.1's rules;
2. its `group` is the group's ID (for `founded`, its ID is);
3. its `date` is at most 5 minutes in the future of the member's clock.

It keeps it whether it counts or not: whether an event counts is the
state's to say (section 3.5), from all the events kept, whatever order
they came in. Copies that kept the same events have the same state. An
event that the replay finds not signed with `member`'s key in force at
its `seq` (its member ID until its first `rotated` event, then each
rotation's new key from the `seq` of that `rotated` event on, section
3.4) is dropped. One that never becomes ready (section 3.5) is dropped
after 30 days.

**Equivocation.** Two different payloads, each signed with the member's
key in force, from one member with the same `seq` prove misbehaviour:
neither counts, nor does any later event of that member, which is out of
the group as if banned (a `joined` naming it doesn't count). A member
passes both on, so that every copy sees the proof, and its panel says
so.

### 3.3 The types

| Type | Signed by | Members | Rules |
|------|-----------|---------|-------|
| `founded` | the founder, who becomes owner | `name`, `settings`, `nonce` (16 random bytes, base64url), `ownerSeq` 1 | the first event of the list; its ID is the group's |
| `profile` | the member itself | `name`, `encryptionKey` (an X25519 public key, base64url of 32 bytes), `nextCommitment` ([core section 12.5](core.md#125-the-next-key-commitment)) | its latest `profile` is in force |
| `joined` | the owner or an admin: the inviter | `newMember` (the newcomer's key), `name`, `asOf` for an admin | the inviter holds the right (section 3.5); within its daily cap; `newMember` isn't banned, nor a member already |
| `admin` | the owner | `target`, `on` (boolean), `lastSeq` when `on` is false (the last of the target's events that counts), `ownerSeq` | `target` is a member |
| `heir` | the owner | `target` (absent: no heir), `ownerSeq` | `target` is a member |
| `owner` | the owner | `target`, `ownerSeq` | `target` is a member; it is the owner from the next `ownerSeq` |
| `settings` | the owner | `name`, `settings`, `ownerSeq` | |
| `banned` | the owner or an admin | `target`, `lastSeq` (the last of the target's events that counts), `reason` (optional, at most 280 code points), `asOf` for an admin | the target isn't the owner; for an admin, section 3.5 |
| `unbanned` | the owner | `ban` (the `banned` event's ID), `ownerSeq` | |
| `left` | the member itself | | not the owner: the owner gives the group away first |
| `dissolved` | the owner | `keepPairings` (boolean), `ownerSeq` | the last event: nothing after it is kept |
| `rotated` | the member, with its new key | `statement`: an object with `oldKey`, `newKey`, `nextCommitment` (optional), `date`, `oldSignature` and `newSignature`, as in `rotate` ([core section 9.3](core.md#93-rotate)) | the statement verifies as core section 12.2 says; `oldKey` is the member's current key; when its `profile` holds a commitment, SHA-256 of `newKey` matches it |
| `revoked` | the member | `statement`: an object with `key`, `date` and `signature`, as in `revoke` ([core section 9.4](core.md#94-revoke)); `lastSeq` | the statement verifies as core section 12.6 says, and its `key` is one of the member's keys (its member ID, or a key a kept `rotated` moved it to); the member's events signed with the revoked key after `lastSeq` don't count (those under its next key, from its `rotated` on, do); with two, the lower `lastSeq` holds |

`name` is cleaned as core section 9.1 cleans names. `settings` is an
object: `capPerDay` (integer, 1 to 100; 5 when absent). `waitDays`
(integer, 0 to 30), which earlier drafts sent, is still read for its
type and range, and changes nothing. Unknown members are ignored (core
section 2.3); an unknown `type` is kept and passed on, and changes
nothing in the state.

**Stepping down** is an `owner` event toward the successor, which the
stepping-down owner's server names itself: its heir, if it named one and
the heir is still a member; otherwise the oldest admin (named at the
lowest `ownerSeq`); otherwise the oldest member (the lowest `joined`
`date`, then the lowest event ID). The successor is in the event, so no
copy has to compute it.

### 3.4 Member IDs and keys

A member is known by the key it joined with, its **member ID**: the
`newMember` of its `joined` event, or, for the founder, the `signer` of
`founded`. Its current key follows its `rotated` events. Events name
members by member ID (`member`, `target`, `newMember`), never by a later
key. Two members pair with their current keys, as
[core section 12](core.md#12-key-rotation-and-revocation) says.

### 3.5 Rights

The state is computed from the kept events, replayed in one order that
doesn't depend on how they arrived:

1. **Ready.** An event is ready when its member's events of lower `seq`
   have been replayed; for an owner event, the owner event before its
   `ownerSeq`; for an admin event, the owner event its `asOf` names; its
   member was named by a replayed `founded` or `joined`; and, for a
   `banned` or an `admin` with `on` false, the target's events up to its
   `lastSeq` have been replayed.
2. **Counting first.** At each step, the ready event with the lowest ID
   whose rules hold (section 3.3 and those below) is replayed, and counts.
   When no ready event's rules hold, the ready one with the lowest ID is
   replayed without counting. What can never count is refused as soon as
   it is ready, whatever else could count: an event of an excluded member
   or of a dissolved group, one a cut takes away, an owner event whose
   `ownerSeq` was taken, the other payload of an equivocation. A replayed
   event takes its `seq` either way, and an owner event its `ownerSeq`.
   An event never ready doesn't count.
3. **Cuts.** A ban, the removal of an admin and a revocation take away
   events of their target after their `lastSeq` whenever they are
   replayed: the replay starts again with the cuts its counted events made
   applied from the start, until they no longer change.

By these rules, two owner events with the same `ownerSeq`: the first
replayed counts, the other doesn't. The rules:

- **The owner** at `ownerSeq` k is the founder, changed by each `owner`
  event below k. An owner event whose signer isn't the owner at its
  `ownerSeq` doesn't count.
- **An admin, as of `ownerSeq` k**, is a member that an `admin` event
  with `on` true and an `ownerSeq` at most k named, and that no later
  `admin` event at most k removed.
- **An admin event counts** when its signer was an admin as of its
  `asOf`, and, if the owner later removed it, when its `seq` is at most
  the removal's `lastSeq`. An `asOf` higher than every kept owner event is
  held until that owner event arrives.
- **A ban by an admin** counts only when its target is neither an admin
  as of its `asOf` nor an admin in the current state: the owner removes
  an admin's role before that admin can be banned. The owner's bans count
  always, except on itself.
- **Conflicts** are settled by the replay's order: an event counts when
  its rules hold at its turn. An `unbanned` lifts the ban it names,
  whatever their dates.
- **The daily cap**: an inviter's `joined` events are counted per UTC day
  of their `date`; beyond the `capPerDay` in force, the extra ones (latest
  `date`, then highest ID) don't count.
- **A ban** takes a member out: its events after the ban's `lastSeq`
  don't count, it is no longer a member, and a `joined` naming it doesn't
  count until an `unbanned` lifts the ban.
- **A member that left** is no longer a member; its events after its
  `left` don't count. It may come back through a new `joined`.
- **A vanished owner** (its key lost, its server gone, without giving the
  group away) can't be replaced: nobody can tell for every member that it
  is gone. Its powers freeze: no admin named or removed, no ban lifted, no
  setting changed. Admins keep inviting and banning, and pairings go on.

### 3.6 What each member keeps

The events, as received, with their signatures, and the state computed
from them. Events of members that left or were banned stay in the list:
they explain the state, and the list is how later members check it. A
server that left a group, or whose group was dissolved, keeps nothing of
it.

## 4. Joining

1. **A code.** B, an admin of "Family", makes a code for the group
   (section 2), and sends it to N's operator.
2. **Pairing with the inviter.** N's operator pastes it; N calls `pair`
   on B (core section 9.1). B's operator accepts after the fingerprint
   check (core section 7.1, step 6): B and N are `paired`.
3. **Joined.** B signs `joined` for N and passes it on (section 6).
4. **The list.** N calls `group-sync` on B (section 8.1) and keeps the
   list.
5. **Profile.** N signs its `profile`, with a new X25519 key pair kept
   for this group, and passes it to B.
6. **Requests.** N sends a **request** (section 5) to every member it
   isn't linked with, one each, encrypted for that member, through the
   group. A member that joins later sends N its own.
7. **Answers.** Each member answers for itself (section 4.1). A member
   that accepts makes the link (section 4.2). A member that refuses sends
   nothing.

### 4.1 Answering

Each member chooses, per group, whether it **automatically accepts new
members**. Off (the default), each request waits in its panel for its
operator. On, its server accepts a request as soon as it arrives. The
panel shows the choice as one checkbox, "Automatically accept new
members", in the member's settings for that group.

Accepting by hand is possible at any time. A **refusal** is final until
the refusing member changes its mind: it lifts it, or sends a request
itself. It is private: nothing is sent, the requester sees no answer, and
the others never know. A request from a server the receiver blocked
(section 7) is dropped without a trace. A request from a member already
linked with the receiver adds the group to the link's relations, without
asking.

### 4.2 Making the link

When C accepts N's request:

- if N can be called (its request says so), C calls `group-pair` on N's
  address, from N's request;
- if N can't be called ([core section 7.5](core.md#75-a-peer-that-cant-be-called)),
  C sends N an **answer** (section 5) carrying C's address, and N calls
  `group-pair` on C;
- if neither can be called, they can't be linked, as with codes.

`group-pair` (section 8.3) proves that its caller is a member of the
group, and answers a request or an answer the receiver sent. Both servers
have consented: one by sending the request, the other by accepting it.
Both store each other as `paired` at once (core section 7.3), with the
group as the link's source, and the caller sends `hello` (core section
9.2) for versions and pins, as after an operator's acceptance. A link
that already exists isn't made twice: the group is added to its relations.

## 5. Requests and answers: the mail

A mail carries what pairing needs and the list mustn't show: an address.
With it, a member's address reaches only the members it accepted.

| Member | Type | Content |
|--------|------|---------|
| `mail` | string | base64url of the payload, a JSON object of at most 2,048 bytes |
| `signature` | string | the sender's Ed25519 signature of `msfp-group-mail-1\n` followed by the payload's bytes |

The payload:

| Member | Type | Content |
|--------|------|---------|
| `group` | string | the group's ID |
| `id` | string | 16 random bytes, base64url |
| `kind` | string | `request` or `answer` |
| `from` | string | the sender's member ID |
| `to` | string | the recipient's member ID |
| `answers` | string | for `answer`: the `id` of the request it answers |
| `expires` | integer | Unix seconds, at most 30 days after it was made |
| `sealed` | string | base64url of the HPKE encapsulated key (32 bytes) followed by the ciphertext |

Once opened, `sealed` holds a JSON object: `address` (core section 5.2),
`name`, `nextCommitment`, `inbound` (core section 7.5) and
`certificates`, the pins of the certificates the sender's address serves
(core section 3.2; optional, as in `hello`).

**Encryption**: HPKE (RFC 9180), mode base, with the KEM
DHKEM(X25519, HKDF-SHA256) (`0x0020`), the KDF HKDF-SHA256 (`0x0001`) and
the AEAD AES-128-GCM (`0x0001`), to the recipient's `encryptionKey` (its
latest `profile`). The `info` is the bytes `msfp-group-mail-1` followed by
the group's ID and the mail's `id`, both decoded. The associated data is
`from` and `to`, decoded, in that order. One message is sealed per
context.

A member keeps a mail when its signature verifies under `from`'s current
key, `from` and `to` are members, and it hasn't expired. It passes it on
until it expires, keeps at most 1,000 mails per group (dropping the
soonest to expire first), and opens only those to itself. A requester
whose request expired unanswered may send it again.

## 6. Passing the list on

Events and mail travel along the links inside the group: each member
exchanges them with the members it is paired with through that group.

- **`group-push`** (section 8.2), at once, to every paired member of the
  group, when a member signs an event or a mail, or keeps one it didn't
  have, except toward the member it came from.
- **`group-sync`** (section 8.1), every 10 minutes, give or take a fifth,
  with one paired member of the group chosen at random; at once after a
  link is made through the group. A member that can't be called is never
  pushed to (core section 7.5): it calls `group-sync` every 5 minutes.
- **The newcomer** is paired with its inviter from the start (section 4,
  step 2): every member has at least one way in.
- **A ban or a dissolution** is passed on before anything else is done
  with it (section 7).

**No way in.** A member whose links through a group have all ended has no
way in: nothing it signs or sends leaves it, and nothing from the others
reaches it, while the group has other members. An individual relation
with a member doesn't change that: it isn't a link through the group.

- Before ending a link that is the server's last one through a group
  with other members, the panel names that group and offers to leave it
  instead (section 7). Without that offer, the group stops reaching the
  server without warning.
- While a member has no way in, its panel says so for that group, and
  sending a mail is refused there with the reason, never kept as sent.
- The way back is a new code for the group from a member: a member back
  in (section 2).

## 7. Leaving, blocking, banning, dissolving

- **Leaving**: the member signs `left` and passes it on. Every link
  through the group loses it as a relation, on both sides, and a link
  with no relation left ends (core section 7.7). Before it leaves, its
  panel lists the links only this group holds, nothing ticked, and asks
  the ticked servers for the individual relation first: those links stay
  until their operators answer. The owner gives the group away before
  leaving.
- **Blocking** is private and covers everything: a server that blocks
  another ends its link with it, whatever its relations, and keeps it
  away from every group: its keys are blocked (core section 7.4). Nothing goes to the group: a
  count others could check would name who blocked, and one they couldn't
  check could be faked. Blocking a member offers to block the members it
  invited too, and theirs, nothing ticked, one click for all.
- **Banning**: a `banned` event (section 3.3). Every member removes the
  group from the banned server's link relations; a link with no other
  relation ends. The banned server keeps the ban like any event, through
  any member still paired with it: it removes the group, ends the links
  that had no other relation, and its panel says it was banned, with the
  reason if one was given. If that misses it, the members' `ended`
  give the same result.
- **Dissolving**: a `dissolved` event, the owner's only. Each member
  passes it on first, then removes the group. With `keepPairings` false,
  links whose only relation was the group end; with true, each waits for
  its operator on each side, to keep it (asking for the individual
  relation, core section 7.7) or end it. A member offline at the time learns
  it from a link's `ended`, or from its next `group-sync`.

## 8. The calls

Each call is signed and answered as core sections 6 and 8 say. A call
about a group the receiver isn't in, or from a caller the receiver
doesn't hear for that group, gets the `404` of core section 6.4.

**Who is heard** (core section 6.5): on `group-sync` and `group-push`, a
peer in state `paired` that is a member of the group in the receiver's
copy, or that the receiver called `group-join` on for the group and is
waiting for; on `group-join`, a peer in state `paired`; on `group-pair`, a member of the group in the receiver's copy, not
banned, not blocked, holding a request or answer the receiver can match
(section 8.3). Requests from members heard on `group-pair` only go in a
nonce memory of their own (for example at most 1,000 entries), and count
against limits of their own (core section 8.3).

### 8.1 group-sync

Request:

| Field | Type | Required | Content |
|-------|------|----------|---------|
| `group` | string | yes | the group's ID |
| `have` | object | yes | per member ID, the highest `seq` the caller keeps of that member |
| `mail` | array of string | no | the `id`s of the mails the caller keeps, at most 1,000 |

Answer `200`:

| Field | Type | Content |
|-------|------|---------|
| `events` | array | envelopes the caller lacks, in `seq` order per member, at most 500 |
| `mail` | array | mails the caller lacks |
| `have` | object | the receiver's own, as in the request |
| `more` | boolean | `true`: there is more; call again |

The caller then pushes what the receiver lacks (`group-push`).

### 8.2 group-push

Request: `group` (string), `events` (array, at most 500), `mail` (array,
at most 100). Answer `200`, signed, with `{}`. The receiver keeps what
sections 3.2 and 5 allow, and ignores the rest.

### 8.3 group-pair

Request:

| Field | Type | Required | Content |
|-------|------|----------|---------|
| `group` | string | yes | the group's ID |
| `answers` | string | yes | the `id` of the request the caller accepted, or of the answer the caller received for its own request |
| `address` | string | yes | the caller's address (core section 5.2) |
| `name` | string | no | the caller's name |
| `nextCommitment` | string | yes | as in `pair` (core section 9.1) |
| `inbound` | boolean | no | as in `pair` |

The caller checks the receiver's certificate as *trusted*, or *named* by
the pins of the mail it answers or received ([core section 3.2](core.md#32-https-certificates)),
never *any*: `group-pair` carries the caller's address, which the mail
keeps from everyone else.

The receiver checks that the caller's key is the current key of a member
of the group, and that `answers` names a request it sent to that member,
or an answer that member sent for one of its requests, not used before.
Then it stores the caller as `paired` (or adds the group to an existing
link's relations), and answers `200`, signed, with `name`,
`nextCommitment` and `inbound`. Refusals: the `404` above; `409` when
that request or answer was already used.

### 8.4 group-join

A server already paired with a group code's maker uses the code (section
2). Request:

| Field | Type | Required | Content |
|-------|------|----------|---------|
| `secretHash` | string | yes | as in `pair` (core section 9.1) |
| `proof` | string | yes | the join proof, below |

The proof is HMAC-SHA256, keyed with the code's 32-byte secret, over the
ASCII label `msfp-group-join-1\n` (it ends with a line feed) followed by
the caller's 32 raw public-key bytes, sent base64url (43 characters). The
receiver compares it in constant time.

The receiver checks, in this order: a live code with that hash, with a
group attached; the proof; the receiver still holds the right to invite
to that group (section 3.5). Then it spends one use of the code, as `pair`
does, and keeps the caller waiting for its operator. Answer `200`,
signed, with `{}`.

Refusals: `404` (signed: the caller is a peer) when a check fails or the
part is off with the caller; `409` when the caller is already a member of
the group linked with the receiver through it, or a member that can't be
called, banned from it, or already waiting to join it (the code's use
isn't spent). A member linked some other way is heard: a member back in
(section 2).

The caller keeps the group as one it is joining, through the receiver,
and hears the receiver's `group-push` for it (section 8). Accepted, the
receiver signs `joined` and pushes the list (section 6), as for a
newcomer that used `pair`.

## 9. Sharing and limits

- **Offers to a group.** A member chooses, per group, the libraries it
  offers that group, as if the group were one server: every member it is
  linked with through that group is offered them. A link is offered what
  each of its relations offers (their union). On the wire nothing changes:
  `libraries` answers what the caller is offered
  ([catalog section 2](catalog.md#2-libraries)).
- **`local` titles for a group.** A member chooses, per group, whether it
  shares its `local` titles with that group: off by default, as for a
  pairing. A link shares them when one of its relations allows it and
  offers the library that holds them
  ([catalog section 5](catalog.md#5-what-is-shared)).
- **Limits for a group as a whole.** A member sets, per group, a pool it
  lends that group, shared by all the group's other members, with every
  member of `limits` ([playback section 5](playback.md#5-limits)):
  - counts, for the whole group together: `streams` and `transcodes`,
    `downloads` at once, and the day's `downloadsPerDay` and
    `scansPerDay` (and the members a content part adds, such as music's
    `audioStreams` and `audioTranscodes`);
  - caps, for each playback counted in the pool: `bitrateKbps`, and
    `downloadKbps` for each download.

  Downloads and scans are off for a group (0) unless the member allows
  them. A playback counts against the **first pool with room** for its
  purpose: the individual relation's own limits first, then its groups'
  pools; it is refused only when all are full, and counts against exactly
  one, its day's count included. The `limits` member of `libraries` gives
  the most generous source's limits; since a pool is shared, a peer may
  be refused (`409` `stream-limit-reached`) before reaching them. A
  server's own users are never refused for a peer.

## 10. What members see

Each member's name, fingerprint and role, whether they are linked, who
invited it and when it joined. Whether this server has a way in (section 6). What a server offers stays between linked
servers; addresses too.

## 11. Security considerations

- **Trust passes through the inviter.** Members that pair through a group
  compare no fingerprints: each trusts that the inviter checked the
  newcomer's, and that the owner chose the admins. A group is as careful
  as its least careful admin.
- **A hacked member** sees names and keys, never an address it wasn't
  given, and gets no link without a yes.
- **A hacked admin** can invite (each newcomer still asks every member,
  and only members that accept new members automatically say yes without
  their operator) and ban members, until the owner lifts the bans and
  removes it. Its invitations are capped per day. A member that accepts
  new members automatically trusts the group's admins entirely.
- **A hacked owner** can do anything the owner does, dissolving the group
  included. The members' way out: block it, leave, make a new group.
- **Forging rights** needs the right key: every member checks every event
  against its own copy. Hiding events from a copy delays it, and the
  numbering shows the gap.
- **Equivocation** is detected by any member that sees both events, and
  excludes the signer.
- **Mail** is readable by its recipient only; the members carrying it see
  who writes to whom, and when.
- **Outside the group** nothing changes: no call of this part is answered
  to a stranger.
- **Encryption** is new to MSFP; it goes to the external review that
  [core section 18](core.md#18-open-questions), item 3, asks for before
  version 1.

Test vectors: [vectors/v0/groups.json](../../vectors/v0/groups.json)
(events valid and not, the state they give, a code with a group, a mail
sealed with HPKE); the codes' cases are in
[vectors/v0/strict.json](../../vectors/v0/strict.json).

## 12. Open questions

1. **Large groups.** The part was thought out for friends. With hundreds
   of members: the list's size over years (a snapshot to start from,
   signed by the owner?), one mail per member for each newcomer, and
   whether pools and the cap still make sense.
2. **Sizes.** 4,096-byte events, 2,048-byte mail, 1,000 mails, 500 events
   a call: first guesses, to check against the first implementation.
3. **Catching up after a long absence.** A member offline past a mail's
   30 days misses it; this draft lets the requester send an expired
   request again (section 5). Whether that is enough is open.
