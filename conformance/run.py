#!/usr/bin/env python3
"""Checks an MSFP version 0 server against spec/v0/, as a peer would.

    python3 conformance/run.py --code '<pairing code from the server>' \\
        --address https://<this machine, as the server reaches it>:8000 --listen 0.0.0.0:8000

See conformance/README.md."""

import argparse
import os
import sys
import time

sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))

from msfpconf import __version__  # noqa: E402
from msfpconf.checks import Suite  # noqa: E402
from msfpconf.report import Report  # noqa: E402


def parser():
    ap = argparse.ArgumentParser(description="MSFP version 0 conformance suite (spec/v0/: core 0, catalog, playback, "
                                             "media, video and music 1)")
    ap.add_argument("--code", help="a pairing code the server's operator made (msfp-pair-0.…); it is used: one code per run")
    ap.add_argument("--group-code", help="a code of a group the server's operator made (groups 2), of the same server as "
                    "--code: the suite joins it through group-join (groups 8.4), its operator accepts, and the group "
                    "checks run")
    ap.add_argument("--code2", help="a second code of the same server: checks the signed 409, then pairs again as a server "
                                    "that can't be called")
    ap.add_argument("--target", help="the server's address, https://host[:port]; taken from --code when given")
    ap.add_argument("--address", help="this suite's own address, as the server reaches it, https://host:port "
                                      "(put in the pair request)")
    ap.add_argument("--listen", default="0.0.0.0:8000", help="where the suite listens for --address (default 0.0.0.0:8000)")
    ap.add_argument("--new-address", help="the address the suite moves to for the address-change checks "
                                          "(default: --address with the port + 1)")
    ap.add_argument("--new-listen", help="where the suite listens for --new-address (default: --listen with the port + 1)")
    ap.add_argument("--connect", help="connect to this host:port instead of the server address's (requests stay signed for "
                                      "the address)")
    ap.add_argument("--key", help="a file holding the suite's Ed25519 seed (made on first use); default: a new key each run")
    ap.add_argument("--tls-cert", help="a PEM certificate for the suite's HTTPS server (default: a self-signed P-256 one, "
                                       "made for the run)")
    ap.add_argument("--tls-key", help="the PEM private key of --tls-cert")
    ap.add_argument("--plain-http", action="store_true",
                    help="OUTSIDE THE PROTOCOL: plain http both ways, for an implementation's test switch (core 3.1)")
    ap.add_argument("--no-operator", action="store_true",
                    help="skip every check that needs the server's operator (rotate or revoke its key, confirm, want "
                         "a library); the pairing must then be accepted by a test switch within --accept-timeout")
    ap.add_argument("--operator", action="store_true", help="run the operator steps even when stdin isn't a terminal")
    ap.add_argument("--operator-timeout", type=int, default=300, help="seconds to wait for each operator step (default 300)")
    ap.add_argument("--accept-timeout", type=int, default=None,
                    help="seconds to wait for the pairing to be accepted (default 600; 60 with --no-operator)")
    ap.add_argument("--watch-changes", type=int, default=0,
                    help="seconds to wait for the operator to add and remove a title (0: skip that check)")
    ap.add_argument("--watch-local", type=int, default=0,
                    help="seconds to wait for the operator to stop sharing local titles with this suite, then share "
                         "them again (T12; 0: skip it)")
    ap.add_argument("--watch-pause", type=int, default=0,
                    help="seconds the operator keeps one of this suite's films paused on the server, then stops it "
                         "(checks the server as a viewer, VW1 to VW4; 0: skip them)")
    ap.add_argument("--uncallable-watch", type=int, default=20,
                    help="seconds the suite watches that nothing connects to it once it says it can't be called (default 20)")
    ap.add_argument("--load", action="store_true", help="also send a burst of calls to check the rate limits (core 8.3)")
    ap.add_argument("--load-count", type=int, default=1000, help="how many calls the burst sends (default 1000)")
    ap.add_argument("--end", action="store_true", help="end the pairing at the end (core 7.4): the server forgets the suite")
    ap.add_argument("--skip", default="", help="check IDs or ID prefixes to skip, comma-separated (for example AD,X1,U)")
    ap.add_argument("--json", help="also write the results to this file")
    ap.add_argument("-v", "--verbose", action="store_true", help="show the whole detail of passing checks too")
    return ap


def main():
    ap = parser()
    a = ap.parse_args()
    if not a.target and not a.code:
        ap.error("give --code (or --target for the stranger checks only)")
    if bool(a.tls_cert) != bool(a.tls_key):
        ap.error("give --tls-cert and --tls-key together")
    report = Report(verbose=a.verbose)
    report.note("MSFP version 0 conformance suite %s, %s" % (__version__, time.strftime("%Y-%m-%d %H:%M:%S %Z")))
    suite = Suite(a, report)
    try:
        suite.run()
    except KeyboardInterrupt:
        report.note("interrupted")
    for section, text in suite.defaults:
        report.note("a recommended default not followed (%s; not a conformance failure): %s" % (section, text))
    for section, text in suite.unclear:
        report.note("unclear in the specification (%s): %s" % (section, text))
    n, _ = report.summary()
    if a.json:
        report.write_json(a.json, {"version": __version__, "target": suite.target, "time": time.time(),
                                   "unclear": [{"section": s, "text": t} for s, t in suite.unclear],
                                   "defaults": [{"section": s, "text": t} for s, t in suite.defaults]})
    return 1 if n["fail"] else 0


if __name__ == "__main__":
    sys.exit(main())
