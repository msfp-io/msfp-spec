package main

import (
	"encoding/json"
	"fmt"
	"strings"
)

// verifyRevMedia checks the media vectors (what MSFP adds to MSFP Media's
// fetching as it carries it: the scan ticket), read back from the file:
// each value on its own.
func verifyRevMedia(read func(string) []byte) []error {
	const file = "v0/media.json"
	var errs []error
	bad := func(format string, a ...any) { errs = append(errs, fmt.Errorf(file+": "+format, a...)) }
	var f obj
	if err := json.Unmarshal(read(file), &f); err != nil {
		return []error{fmt.Errorf("%s: %v", file, err)}
	}
	if a, ok := f["about"].([]any); !ok || len(a) == 0 {
		bad("no about")
	}
	// The arrays media section 4.2 names: names under a scan ticket, and
	// the scan's answers.
	list := func(name string) []obj {
		a, ok := f[name].([]any)
		if !ok || len(a) == 0 {
			bad("no %s array", name)
			return nil
		}
		var out []obj
		names := map[string]bool{}
		for _, v := range a {
			x, ok := v.(obj)
			if !ok {
				bad("%s: a case that isn't an object", name)
				continue
			}
			n := str(x, "name")
			if n == "" || names[n] {
				bad("%s: a case without a name of its own: %q", name, n)
			}
			names[n] = true
			valid, ok := x["valid"].(bool)
			if name != "scans" {
				if !ok {
					bad("%s %q: no valid", name, n)
				}
				if valid == (str(x, "reason") != "") {
					bad("%s %q: valid and reason disagree", name, n)
				}
			}
			out = append(out, x)
		}
		return out
	}
	for _, x := range list("names") {
		var t mediaTicket
		b, _ := json.Marshal(x["ticket"])
		if err := json.Unmarshal(b, &t); err != nil || t.Kind == "" {
			bad("names %q: no ticket", str(x, "name"))
		}
		err := mediaServes(t, str(x, "text"))
		if (err == nil) != (x["valid"] == true) || !sameReason(err, x) {
			bad("names %q: the file says valid=%v; the rules give %v", str(x, "name"), x["valid"], err)
		}
		// A name served is a bare name or one of section 1's paths, never
		// a part that is empty, a dot, or holds a %, a query or a fragment.
		if err == nil {
			for _, part := range strings.Split(str(x, "text"), "/") {
				if checkMediaName(part) != nil {
					bad("names %q: a part %q that isn't a bare name", str(x, "name"), part)
				}
			}
		}
	}
	for _, x := range list("scans") {
		scanCap := scanCapOf(num(x, "scanBytes"))
		status, cr := scanAnswer(scanCap, num(x, "sent"), str(x, "range"), num(x, "size"))
		if status != int(num(x, "status")) || cr != str(x, "contentRange") {
			bad("scans %q: the file says %d %q; the rules give %d %q", str(x, "name"), num(x, "status"), str(x, "contentRange"), status, cr)
		}
		// A 206 never takes the ticket past the cap.
		if status == 206 {
			var s, e, t int64
			if _, err := fmt.Sscanf(cr, "bytes %d-%d/%d", &s, &e, &t); err != nil || num(x, "sent")+e-s+1 > scanCap || e >= t {
				bad("scans %q: a 206 past the cap or the end", str(x, "name"))
			}
		}
	}
	return errs
}
