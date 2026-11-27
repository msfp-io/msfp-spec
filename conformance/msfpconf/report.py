"""Results: each check has an ID, the spec file and section it tests
("core 6.4", "MSFP Media fetching 2.1"), a level (MUST or SHOULD, as the section says),
whether it needs the server's operator, a result, and why."""

import json
import sys

PASS, FAIL, SKIP = "pass", "fail", "skip"


class Result:
    def __init__(self, cid, section, title, level, status, detail="", operator=False):
        self.id, self.section, self.title, self.level = cid, section, title, level
        self.status, self.detail, self.operator = status, detail, operator

    def as_dict(self):
        return {"id": self.id, "section": self.section, "title": self.title, "level": self.level,
                "operator": self.operator, "result": self.status, "detail": self.detail}


class Report:
    def __init__(self, out=sys.stdout, verbose=False):
        self.results = []
        self.notes = []
        self.out = out
        self.verbose = verbose
        self._tty = hasattr(out, "isatty") and out.isatty()

    def add(self, r):
        self.results.append(r)
        mark = {PASS: "PASS", FAIL: "FAIL", SKIP: "SKIP"}[r.status]
        if self._tty:
            mark = {PASS: "\033[32mPASS\033[0m", FAIL: "\033[31mFAIL\033[0m", SKIP: "\033[33mSKIP\033[0m"}[r.status]
        line = "%s  %-5s %-22s %s" % (mark, r.id, r.section, r.title)
        if r.level == "SHOULD":
            line += " (should)"
        if r.operator:
            line += " [operator]"
        if r.detail:
            lines = r.detail.splitlines()
            if r.status == PASS and not self.verbose:
                lines = lines[:1]
            line += "\n" + "\n".join("        " + x for x in lines)
        print(line, file=self.out, flush=True)

    def note(self, text):
        self.notes.append(text)
        print("-- " + text, file=self.out, flush=True)

    def summary(self):
        n = {s: sum(1 for r in self.results if r.status == s) for s in (PASS, FAIL, SKIP)}
        must_fail = sum(1 for r in self.results if r.status == FAIL and r.level == "MUST")
        print("\n%d passed, %d failed (%d of them MUST), %d skipped" % (n[PASS], n[FAIL], must_fail, n[SKIP]),
              file=self.out, flush=True)
        return n, must_fail

    def write_json(self, path, meta):
        with open(path, "w") as f:
            json.dump({"meta": meta, "results": [r.as_dict() for r in self.results], "notes": self.notes}, f, indent=1)
