#!/usr/bin/env python3
"""The suite's self-test. Needs no server.

1. The suite's readers against the vectors: vectors/v0/*.json (keys,
   code, exchanges, rotation, revocation, strict, groups; and MSFP's
   carrier vectors, playback.json and media.json), the cases of
   vectors/history/test-draft-strict.json that core 0 section 17 says
   hold for core 0 (keys, signatures, base64url, dates, integers, JSON),
   and MSFP Media's readers against msfp-media's vectors, as
   msfp-media's conformance/selftest.py checks them (with listing.json's
   titleRequests, the source's reading of a titles request that the
   fake answers with).
2. The whole suite against an in-process fake version 0 server
   (msfpconf/fake.py, built from the specification and the vectors),
   over HTTPS both ways, with the self-test playing the server's
   operator. Every check must pass.

3. The same run against the fake with deliberate faults (FAULTS below,
   then CHOICE in a pass of its own): the checks aimed at each fault must
   fail, so the suite is seen to catch what it checks.

    python3 conformance/selftest.py [-v] [--vectors-only] [--quick] [--media DIR]

--quick skips part 3. --media names msfp-media's checkout (default:
../msfp-media, beside this repository). Exit 0 when every part passes."""

import io
import json
import os
import re
import socket
import sys
import time

HERE = os.path.dirname(os.path.abspath(__file__))
sys.path.insert(0, HERE)
if "--media" in sys.argv[:-1]:
    os.environ["MSFP_MEDIA"] = sys.argv[sys.argv.index("--media") + 1]

from msfpconf import ed25519  # noqa: E402
from msfpconf.carrier import carrier_vectors_ok  # noqa: E402
from msfpconf.wire import (Answer, Key, NotIJSON, answer_message, b64u, check_address, check_base,  # noqa: E402
                           check_destination, check_uncallable_address, commitment, decimal,
                           fingerprint, is_int, make_rotation, parse_code, parse_ijson, proof, read_revocation,
                           parse_code_group, read_rotation, request_message, unb64u)
from msfpmedia.vectorcheck import vectors_ok as media_vectors_ok  # noqa: E402

VEC = os.path.join(HERE, "..", "vectors")

# The second pass: faults put in the fake on purpose (msfpconf/fake.py),
# and the checks that must then fail. Other checks may fail too, from the
# same faults; only these are required to.
FAULTS = {
    "nonce": ["N1", "N2"],          # replays accepted (core 6.5)
    "null": ["J1"],                 # null members in hello accepted (core 2.3)
    "since": ["T4"],                # changes for a since above seq (MSFP Media listing 3.2)
    "with": ["T1"],                 # an episode's with naming another series (MSFP Media video 4.2)
    "limits": ["PB10"],             # no stream limit (playback 6)
    "progress": ["PB5"],            # progress of a playback that isn't live answered 200 (MSFP Media playing 5)
    "query": ["M4"],                # media served with a query string (MSFP Media fetching 2)
    "range": ["F2"],                # several ranges served as the first (MSFP Media fetching 2.1)
    "vtt": ["M6"],                  # a WebVTT part served as it is, not rebuilt (MSFP Media video 6.3)
    "loopback": ["X1"],             # connects to a loopback address for a peer (core 3.3)
    "oldkey": ["O2"],               # still signs with the old key after the peer answered rotated (core 12.4)
    "quietpause": ["VW2", "VW3"],   # no progress while its viewer pauses (MSFP Media playing 3, 5; expiry 20 s in that run)
    "badstop": ["VW1"],             # a stop with a negative position (MSFP Media playing 4)
    "groupforged": ["GR13"],        # a group event whose signature doesn't verify kept (groups 3.2)
    "mailforged": ["GM3"],          # a mail whose signature doesn't verify kept (groups 5)
    "summary": ["T1"],              # a film with the title summary video 0 removed (MSFP Media video 3.2)
    "etag": ["PB18"],               # a download's answer whose etag isn't the catalog's (MSFP Media playing 2.3)
    "head": ["M4", "F4"],           # HEAD on media answered 404, not 405 (MSFP Media fetching 2.2)
    "growing": ["M2"],              # a media playlist without EXT-X-ENDLIST (MSFP Media fetching 3.1)
    "scancap": ["PB22"],            # a scan's reads past the answer's scanBytes served (media 4.2)
    "scanfloor": ["PB22"],          # scans cut at the 4 MiB floor, not at the scanBytes their answer states (playback 8)
    "tokens": ["PB24"],             # an unknown delivery token refused with 400, not 409 (MSFP Media base 3)
    "remux": ["PB24"],              # a remux to video/matroska served, not refused (MSFP Media playing 2.7: mp2t or mp4)
    "rangetoken": ["PB24"],         # an unknown range token refused with 400, not 409 (MSFP Media playing 2.7)
    "baseline": ["PB4"],            # the file served without delivery (MSFP Media playing 2.7: the baseline is HLS)
    "baselinecopy": ["PB4"],        # without delivery, a copy outside the baseline left unchanged (MSFP Media playing 2.7)
    "renditions": ["PB17"],         # audioRenditions true in the answer, none in the playlist (MSFP Media playing 2.3)
    "container": ["T1"],            # a copy with container matroska, no mediaType (MSFP Media video 3.2: skipped by a caller)
    "nocodecs": ["T1"],             # an H.264 stream without codecs (MSFP Media video 3.5: required for the six)
    "urls": ["PB3"],                # a URL in play's answer (MSFP Media playing 2.3: answers carry no URLs)
    "pastcount": ["M10"],           # fonts/<fontCount> served (MSFP Media video 6.1: 404 past a count)
    "burncopy": ["PB8"],            # burnSubtitle with video copy served, not refused (MSFP Media playing 2.7)
    "prefetch": ["Q4"],             # as a viewer, every copy's previews fetched while reading a catalog (video 5.5)
    "groupname": ["GM5", "GM6"],    # a name in the mail and in group-pair's answer (groups 5, 8.3)
    "targettype": ["PB24", "PB8"],  # a target it doesn't make refused with delivery-refused, not target-refused (MSFP Media playing 2.2 step 7)
    "capsignored": ["PB1"],         # a capability list ignored, not refused with 400 (playback 2)
    "pausedtype": ["PB5"],          # a paused that isn't a boolean accepted (MSFP Media playing 3)
    "replaces": ["PB26"],           # replaces ignored: the old playback stays, both counted (MSFP Media playing 2.3; playback 6)
    "progressrange": ["PB25"],      # progressSeconds 30 in play's answer (MSFP Media playing 2.3: 45 to 120)
    "nogone": ["PB27"],             # a title no longer held answered not-found, not gone (MSFP Media playing 2.2 step 3)
    "nolibraries": ["T13"],         # titles without libraries refused (MSFP Media listing 3: absent, every library)
    "kind": ["T14"],                # kind ignored (MSFP Media listing 3.6)
    "under": ["T15"],               # under ignored (MSFP Media listing 3.6)
}
# Faults in a pass of their own: in the choice of copy and audio (MSFP
# Media playing 2.6), since playing another copy than the one the checks
# expect would hide the faults of the first pass aimed at that copy's
# tracks and fonts; and in the viewer's reports, which quietpause would
# hide.
CHOICE = {
    "chatty": ["VW5"],              # as a viewer, progress every 10 s while paused (MSFP Media playing 3: not more often than 45 s)
    "nopaused": ["VW3"],            # as a viewer, the pause reported without paused true (MSFP Media playing 3)
    "lastcopy": ["PB9"],            # the last copy plays without versionId (MSFP Media playing 2.6: the first)
    "defaultaudio": ["PB7"],        # without audioTrack, not the file's default audio (MSFP Media playing 2.6)
}


def vectors():
    bad = []

    def expect(group, name, want, got):
        if want != got:
            bad.append("%s / %s: expected %s, got %s" % (group, name, want, got))

    load = lambda *p: json.load(open(os.path.join(VEC, *p)))
    v0 = load("history", "test-draft-strict.json")
    for c in v0["keys"]:
        k = unb64u(c["text"], 32)
        expect("v0-strict keys", c["name"], c["valid"], k is not None and ed25519.valid_point(k))
    for c in v0["signatures"]:
        expect("v0-strict signatures", c["name"], c["valid"],
               ed25519.verify_strict(unb64u(c["public"], 32), c["message"].encode(), unb64u(c["signature"], 64) or b""))
    for c in v0["base64url"]:
        size = 16 if "nonce" in c["name"] else 32 if "key" in c["name"] else None
        expect("v0-strict base64url", c["name"], c["valid"], unb64u(c["text"], size) is not None)
    for c in v0["dates"]:
        expect("v0-strict dates", c["text"], c["valid"], decimal(c["text"]) not in (None, 0))
    for c in v0["integers"]:
        try:
            ok = is_int(parse_ijson(('{"n":%s}' % c["text"]).encode())["n"])
        except NotIJSON:
            ok = False
        expect("v0-strict integers", c["text"], c["valid"], ok)
    for c in v0["json"]:
        raw = bytes.fromhex(c["hex"]) if "hex" in c else c["text"].encode("utf-8", "surrogatepass")
        try:
            parse_ijson(raw)
            ok = True
        except NotIJSON:
            ok = False
        expect("v0-strict json", c["name"], c["valid"], ok)

    st = load("v0", "strict.json")
    for c in st["addresses"]:
        expect("addresses", c.get("text", ""), c["valid"], check_address(c.get("text", "")) is None)
    for c in st["codes"]:
        try:
            parse_code(c.get("text", ""))
            ok = True
        except ValueError:
            ok = False
        expect("codes", c["name"], c["valid"], ok)
    for c in st["bases"]:
        expect("bases", c.get("text", ""), c["valid"], check_base(c.get("text", "")) is None)
    for c in st["destinations"]:
        expect("destinations", c.get("text", ""), c["valid"], check_destination(c.get("text", "")) is None)
    for c in st["uncallableAddresses"]:
        expect("uncallableAddresses", c.get("text", ""), c["valid"], check_uncallable_address(c.get("text", "")) is None)


    keys = load("v0", "keys.json")
    K = {}
    for name, k in keys["keys"].items():
        K[name] = Key(bytes.fromhex(k["seed"]))
        expect("keys", name + " public", k["public"], K[name].text)
        expect("keys", name + " fingerprint", k["fingerprint"], fingerprint(K[name].pub))
        expect("keys", name + " commitment", k["commitment"], commitment(K[name].pub))
    import hashlib
    for name, c in keys["certificates"].items():
        expect("certificates", name, c["pin"], b64u(hashlib.sha256(bytes.fromhex(c["spki"])).digest()))

    code = load("v0", "code.json")
    key, secret, expiry, address = parse_code(code["code"]["text"])
    expect("code", "key", K[code["code"]["key"]].pub, key)
    expect("code", "secret", code["code"]["secret"], secret.hex())
    expect("code", "expiry", code["code"]["expires"], expiry)
    expect("code", "address", code["code"]["address"], address)
    expect("code", "secretHash", code["code"]["secretHash"], b64u(hashlib.sha256(secret).digest()))
    pr = code["proof"]
    expect("code", "proof", pr["value"], b64u(proof(secret, K[pr["redeemer"]].pub, pr["address"])))

    ex = load("v0", "exchanges.json")["exchanges"] + [load("v0", "rotation.json")["exchange"], load("v0", "revocation.json")["exchange"]]
    for e in ex:
        n = e["name"]
        signer, answerer = K[e["signer"]], K[e["answerer"]]
        nonce = b64u(bytes.fromhex(e["nonce"]))
        expect("exchanges", n + " nonce", e["nonceText"], nonce)
        msg = request_message(e["method"], e["host"], e["target"], e["date"], nonce, e["body"].encode())
        expect("exchanges", n + " message", e["message"], msg.decode())
        expect("exchanges", n + " signature", e["signature"], b64u(signer.sign(msg)))
        am = answer_message(nonce, e["status"], e["answerContentType"], e["answerBody"].encode())
        expect("exchanges", n + " answer message", e["answerMessage"], am.decode())
        a = Answer(e["status"], [("Content-Type", e["answerContentType"]), ("MSFP-Signature", e["answerSignature"])],
                   e["answerBody"].encode(), nonce, [answerer.pub])
        expect("exchanges", n + " answer verifies", True, a.signed)
        body = json.loads(e["answerBody"]) if e["answerContentType"] == "application/json" else {}
        if "rotation" in body:
            new, nc = read_rotation(answerer.pub, body["rotation"], now=body["rotation"]["date"])
            expect("exchanges", n + " pulled rotation", K["a2"].pub, new)
        if "revocation" in body:
            read_revocation(answerer.pub, body["revocation"], now=body["revocation"]["date"])

    rot = load("v0", "rotation.json")["rotation"]
    mine = make_rotation(K["a"], K["a2"], rot["nextCommitment"], rot["date"])
    expect("rotation", "oldSignature", rot["oldSignature"], mine["oldSignature"])
    expect("rotation", "newSignature", rot["newSignature"], mine["newSignature"])
    new, nc = read_rotation(K["a"].pub, mine, now=rot["date"])
    expect("rotation", "reads", (K["a2"].pub, rot["nextCommitment"]), (new, nc))
    rev = load("v0", "revocation.json")["revocation"]
    read_revocation(K["a"].pub, {"date": rev["date"], "signature": rev["signature"]}, now=rev["date"])

    bad += carrier_vectors_ok(os.path.join(VEC, "v0"))
    bad += media_vectors_ok()

    # Groups: envelopes kept or not (section 3.2), the code's group, the
    # join proof, and the mail opened with HPKE (section 5).
    from msfpconf import hpke
    from msfpconf.groups import GroupFault, join_proof, read_envelope
    gv = load("v0", "groups.json")
    gid = gv["events"][0]["id"]
    for c in gv["events"]:
        try:
            eid, p, _ = read_envelope({"event": c["event"], "signature": c["signature"]}, gid)
            kept = eid == c["id"] and p["date"] <= gv["now"] + 300
        except GroupFault:
            kept = False
        expect("groups events", c["name"], not c.get("reason", "").startswith("not kept"), kept)
    _, secret, _, _, group, name = parse_code_group(gv["code"]["text"])
    expect("groups code", "group", gid, b64u(group or b""))
    expect("groups joinProof", "value", gv["joinProof"]["value"],
           join_proof(secret, unb64u(gv["keys"][gv["joinProof"]["redeemer"]], 32)))
    m = gv["mail"]
    sealed = unb64u(json.loads(m["payload"])["sealed"])
    try:
        opened = hpke.open_(bytes.fromhex(m["recipientX25519Private"]), bytes.fromhex(m["info"]), bytes.fromhex(m["aad"]),
                            sealed).decode()
    except ValueError as e:
        opened = "not opened: %s" % e
    expect("groups mail", m["name"], m["opened"], opened)
    return bad


def free_ports(n):
    """A base port with n free ports after it, on 127.0.0.1."""
    import random
    for _ in range(200):
        base = random.randint(20000, 60000)
        socks = []
        try:
            for i in range(n):
                s = socket.socket()
                s.bind(("127.0.0.1", base + i))
                socks.append(s)
            return base
        except OSError:
            continue
        finally:
            for s in socks:
                s.close()
    raise OSError("no free ports")


def end_to_end(verbose, faults=()):
    import run
    from msfpconf.checks import Suite
    from msfpconf.fake import FakeServer
    from msfpmedia.report import FAIL, SKIP, Report
    fake = FakeServer({"suite.example.org": "127.0.0.1"}, faults=faults)
    port = free_ports(3)
    args = run.parser().parse_args([
        "--code", fake.make_code(), "--code2", fake.make_code(), "--group-code", fake.make_group_code(),
        "--address", "https://suite.example.org:%d" % port, "--listen", "127.0.0.1:%d" % port,
        "--connect", "127.0.0.1:%d" % fake.port, "--operator", "--operator-timeout", "20", "--accept-timeout", "20",
        "--watch-changes", "20", "--watch-local", "20", "--watch-pause", "30", "--load", "--load-count", "150", "--uncallable-watch", "3", "--end"])
    out = sys.stdout if verbose else io.StringIO()
    report = Report(out=out, verbose=verbose)
    t0 = time.time()
    from msfpconf.peer import PeerServer
    # The suite's 2-minute expiry, shorter than the 30-second watch when
    # the fake says nothing while paused (VW2 must fail then).
    PeerServer.EXPIRY = 20 if "quietpause" in faults else 120
    suite = Suite(args, report, operator_hook=fake.operator)
    try:
        suite.run()
    finally:
        fake.close()
    bad = []
    for r in report.results:
        if r.status == FAIL:
            bad.append("%s %s: FAIL: %s" % (r.id, r.title, r.detail))
        elif r.status == SKIP:
            bad.append("%s %s: SKIP: %s" % (r.id, r.title, r.detail))
    n = len(report.results)
    return bad, n, time.time() - t0, report, suite


def main():
    verbose = "-v" in sys.argv
    bad = vectors()
    if bad:
        print("\n".join(bad))
        print("vectors: %d case(s) wrong" % len(bad))
        return 1
    print("vectors: every case of vectors/v0/, of test-draft-strict.json (as core 0 section 17 says) and of msfp-media's "
          "vectors/v0/ decides as the files say")
    if "--vectors-only" in sys.argv:
        return 0
    bad, n, took, report, suite = end_to_end(verbose)
    groups = {}
    for r in report.results:
        g = re.match(r"[A-Z]+", r.id).group(0)
        groups[g] = groups.get(g, 0) + 1
    print("end to end against the fake server: %d checks in %.0f s (%s)" % (n, took, ", ".join(
        "%s %d" % (k, v) for k, v in sorted(groups.items()))))
    if bad:
        print("\n".join(bad))
        print("%d check(s) didn't pass against the fake server" % len(bad))
        return 1
    print("every check passed against the fake server")
    if "--quick" in sys.argv:
        return 0
    _, n, took, report, _ = end_to_end(verbose, faults=list(FAULTS))
    status = {r.id: r.status for r in report.results}
    missed = ["%s (fault %s): %s" % (cid, f, status.get(cid, "not run")) for f, ids in FAULTS.items() for cid in ids
              if status.get(cid) != "fail"]
    _, _, _, report2, _ = end_to_end(verbose, faults=list(CHOICE))
    status2 = {r.id: r.status for r in report2.results}
    missed += ["%s (fault %s): %s" % (cid, f, status2.get(cid, "not run")) for f, ids in CHOICE.items() for cid in ids
               if status2.get(cid) != "fail"]
    if missed:
        print("the fake server with deliberate faults: these checks should have failed and didn't:\n" + "\n".join(missed))
        return 1
    print("with %d deliberate faults in the fake server (in two passes), the %d checks aimed at them all failed "
          "(%d checks in %.0f s in the first)" % (len(FAULTS) + len(CHOICE), sum(len(v) for v in FAULTS.values())
                                                   + sum(len(v) for v in CHOICE.values()), n, took))
    return 0


if __name__ == "__main__":
    sys.exit(main())
