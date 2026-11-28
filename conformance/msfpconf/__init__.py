"""The MSFP version 0 conformance suite: plays a peer against a server
under test and checks it against spec/v0/ (core 0 and the parts) and
MSFP Media version 0, which the parts carry.

MSFP Media's checks are msfp-media's (its conformance/msfpmedia
package); this suite runs them through its MSFP carrier
(msfpconf/carrier.py). It finds the package in a checkout of msfp-media
beside this repository (../msfp-media), or in the one MSFP_MEDIA names
(run.py and selftest.py set it from --media)."""

import os
import sys

__version__ = "1.0.0"

HERE = os.path.dirname(os.path.abspath(__file__))
MEDIA = os.path.abspath(os.environ.get("MSFP_MEDIA") or os.path.join(HERE, "..", "..", "..", "msfp-media"))


def _media_package():
    conf = os.path.join(MEDIA, "conformance")
    if not os.path.isfile(os.path.join(conf, "msfpmedia", "__init__.py")):
        raise SystemExit("MSFP Media's checks aren't in %s: check out msfp-media beside msfp-spec (../msfp-media), "
                         "or name its checkout with --media (or MSFP_MEDIA)" % os.path.join(conf, "msfpmedia"))
    if conf not in sys.path:
        sys.path.insert(0, conf)


_media_package()
