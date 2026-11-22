package main

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
	"unicode/utf8"
)

// verifyRevMedia checks the media vectors of the revision of version 0
// (proposal 16), read back from the files: each value on its own.
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
	// The arrays the specification names (video section 6.2: webvtt, ass,
	// pgs, fonts) and those of media sections 2.1 to 4.2.
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
	// bytesOf is a case's input: text, or hex when it isn't UTF-8.
	bytesOf := func(where string, x obj, textual bool) []byte {
		_, hasText := x["text"]
		_, hasHex := x["hex"]
		if hasText && hasHex {
			bad("%s: both text and hex", where)
		}
		if !hasHex {
			return []byte(str(x, "text"))
		}
		b, err := hex.DecodeString(str(x, "hex"))
		if err != nil {
			bad("%s: bad hex", where)
		}
		if textual && utf8.Valid(b) {
			bad("%s: hex for UTF-8 input", where)
		}
		return b
	}
	rebuilt := func(name string, rebuild func([]byte) ([]byte, error)) {
		for _, x := range list(name) {
			where := name + " " + fmt.Sprintf("%q", str(x, "name"))
			got, err := rebuild(bytesOf(where, x, true))
			if (err == nil) != (x["valid"] == true) || string(got) != str(x, "output") {
				bad("%s: the file says valid=%v; the rules give %v", where, x["valid"], err)
			}
			if err == nil && !utf8.Valid(got) {
				bad("%s: an output that isn't UTF-8", where)
			}
			if err != nil && str(x, "output") != "" {
				bad("%s: an output for a refused part", where)
			}
		}
	}
	rebuilt("webvtt", rebuildVTT)
	rebuilt("ass", rebuildASS)
	// Rebuilding is stable: a rebuilt part rebuilds to itself.
	for _, name := range []string{"webvtt", "ass"} {
		rebuild := rebuildVTT
		if name == "ass" {
			rebuild = rebuildASS
		}
		for _, x := range list(name) {
			// (A script of 1,024 styles without Default is written with
			// 1,025, and gives the first 1,024 again.)
			if x["valid"] != true || strings.Count(str(x, "output"), "\nStyle: ") > maxASSStyles {
				continue
			}
			again, err := rebuild([]byte(str(x, "output")))
			if err != nil || string(again) != str(x, "output") {
				bad("%s %q: the output doesn't rebuild to itself", name, str(x, "name"))
			}
		}
	}

	for _, x := range list("pgs") {
		if err := checkPGS(bytesOf("pgs", x, false)); (err == nil) != (x["valid"] == true) || !sameReason(err, x) {
			bad("pgs %q: the file says valid=%v; the check says %v", str(x, "name"), x["valid"], err)
		}
	}
	for _, x := range list("fonts") {
		typ, err := checkFont(bytesOf("fonts", x, false))
		if (err == nil) != (x["valid"] == true) || typ != str(x, "type") || !sameReason(err, x) {
			bad("fonts %q: the file says valid=%v, type %q; the check says %v, %q", str(x, "name"), x["valid"], str(x, "type"), err, typ)
		}
	}
	for _, x := range list("hls") {
		if err := checkPlaylist([]byte(str(x, "text"))); (err == nil) != (x["valid"] == true) || !sameReason(err, x) {
			bad("hls %q: the file says valid=%v; the check says %v", str(x, "name"), x["valid"], err)
		}
	}
	for _, x := range list("ranges") {
		start, end, err := checkRange(str(x, "text"), num(x, "size"))
		_, hasStart := x["start"]
		_, hasEnd := x["end"]
		if (err == nil) != (x["valid"] == true) || !sameReason(err, x) || hasStart != (err == nil) || hasEnd != (err == nil) ||
			err == nil && (start != num(x, "start") || end != num(x, "end") || start > end || end >= num(x, "size")) {
			bad("ranges %q: the file disagrees with the rules (%d-%d, %v)", str(x, "name"), start, end, err)
		}
	}
	for _, x := range list("fileAnswers") {
		err := checkFileAnswer(mediaAnswer{Range: str(x, "range"), Size: num(x, "size"), ETag: str(x, "etag"), Status: int(num(x, "status")),
			ETagHeader: str(x, "etagHeader"), ContentRange: str(x, "contentRange"), ContentLength: str(x, "contentLength")})
		if (err == nil) != (x["valid"] == true) || !sameReason(err, x) {
			bad("fileAnswers %q: the file says valid=%v; the check says %v", str(x, "name"), x["valid"], err)
		}
	}
	for _, x := range list("scans") {
		status, cr := scanAnswer(num(x, "sent"), str(x, "range"), num(x, "size"))
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

func sameReason(err error, x obj) bool {
	if err == nil {
		return str(x, "reason") == ""
	}
	return err.Error() == str(x, "reason")
}
