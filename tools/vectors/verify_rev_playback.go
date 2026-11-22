package main

import (
	"encoding/json"
	"fmt"
	"slices"
	"strings"
)

// verifyRevPlayback checks the playback vectors of the revision of version 0
// (proposals 16 and 17), read back from the files: each value on its own.
func verifyRevPlayback(read func(string) []byte) []error {
	const file = "v0/playback.json"
	var errs []error
	bad := func(format string, a ...any) { errs = append(errs, fmt.Errorf(file+": "+format, a...)) }
	b := read(file)
	if err := checkIJSON(b); err != nil {
		return []error{fmt.Errorf("%s: %v", file, err)}
	}
	var f v0PlaybackFile
	if err := json.Unmarshal(b, &f); err != nil {
		return []error{fmt.Errorf("%s: %v", file, err)}
	}
	ctx := f.Context
	if ctx.StreamRoot == "" || ctx.Now == 0 || len(ctx.Copies) == 0 {
		bad("context incomplete")
	}
	for key, copies := range ctx.Copies {
		if _, part, err := playbackTitleKind(key); err != nil || part == "" {
			bad("context: %s isn't a key play takes", key)
		}
		for _, c := range copies {
			if !playbackObjIDForm.MatchString(c.ID) || !playbackEtagForm.MatchString(c.Etag) || c.Parts < 1 {
				bad("context: %s: copy %q out of form", key, c.ID)
			}
		}
	}

	names := func(list string, n []string) {
		if len(n) == 0 {
			bad("%s is empty", list)
		}
		seen := map[string]bool{}
		for _, x := range n {
			if seen[x] {
				bad("%s: %q twice", list, x)
			}
			seen[x] = true
		}
	}
	output := func(where, out string, valid bool) {
		if valid != (out != "") {
			bad("%s: output present iff valid", where)
		}
		if out != "" {
			if err := checkIJSON([]byte(out)); err != nil {
				bad("%s: output: %v", where, err)
			}
		}
	}

	calls := func(list string, cases []playbackCase) {
		var n []string
		for _, c := range cases {
			n = append(n, c.Name)
			where := list + " " + c.Name
			got, err := pbRun(ctx, c)
			valid, _ := reason(err)
			if valid != c.Valid || string(got) != c.Output {
				bad("%s: the file says valid=%v, output %q; the rules give %v, %q", where, c.Valid, c.Output, err, got)
			}
			status, typ := playbackStatus(err)
			if status != c.Status || typ != c.Type || (c.Status == 200) != c.Valid {
				bad("%s: the file says %d %q; the rules give %d %q", where, c.Status, c.Type, status, typ)
			}
			if !slices.Contains([]string{"play", "progress", "stop"}, c.Call) || (c.Call != "play") != (c.Playback != nil) {
				bad("%s: call %q and playback don't go together", where, c.Call)
			}
			output(where, c.Output, c.Valid)
			if c.Call == "play" && c.Valid {
				var s playbackStarted
				_ = json.Unmarshal([]byte(c.Output), &s)
				found := false
				for _, cp := range ctx.Copies[s.Title] {
					found = found || cp.ID == s.VersionID
				}
				if !found {
					bad("%s: versionId %q isn't a copy of %s", where, s.VersionID, s.Title)
				}
				// The required minimum (section 2.6): a named copy and a
				// named track of it win; defaults only otherwise.
				var req map[string]any
				_ = json.Unmarshal([]byte(c.Text), &req)
				if v, _ := req["versionId"].(string); v != "" && v != s.VersionID && slices.ContainsFunc(ctx.Copies[s.Title], func(cp playbackCopy) bool { return cp.ID == v }) {
					bad("%s: the named copy %q doesn't win", where, v)
				}
				// Without delivery, the baseline; never the file.
				if (s.Delivery == "") != (s.Baseline != "") {
					bad("%s: the baseline goes with no delivery", where)
				}
				// No capabilities: the owning server never keeps them.
				if strings.Contains(c.Output, "capabilities") {
					bad("%s: capabilities kept", where)
				}
			}
			if c.Call == "play" && c.Type == "delivery-refused" && c.Status != 409 {
				bad("%s: delivery-refused without 409", where)
			}
		}
		names(list, n)
	}
	calls("requests", f.Requests)
	calls("deliveries", f.Deliveries)
	calls("reports", f.Reports)
	calls("downloads", f.Downloads)
	calls("scans", f.Scans)
	for _, c := range f.Downloads {
		if c.Call == "play" && playbackPurposeOf(c.Text) != "download" {
			bad("downloads %s: not a download", c.Name)
		}
	}
	for _, c := range f.Scans {
		if c.Call == "play" && playbackPurposeOf(c.Text) != "scan" {
			bad("scans %s: not a scan", c.Name)
		}
	}

	var n []string
	for _, c := range f.Answers {
		n = append(n, c.Name)
		got, err := playbackKeepAnswer([]byte(c.Text), c.Delivery, c.CatalogEtag, ctx.StreamRoot)
		valid, _ := reason(err)
		if valid != c.Valid || string(got) != c.Output {
			bad("answers %s: the file says valid=%v, output %q; the rules give %v, %q", c.Name, c.Valid, c.Output, err, got)
		}
		output("answers "+c.Name, c.Output, c.Valid)
	}
	names("answers", n)

	n = nil
	for _, c := range f.Restarts {
		n = append(n, c.Name)
		r := playbackRestart(c.Reading, []byte(c.Request), c.Status, []byte(c.Answer), ctx.StreamRoot)
		if r.Join != c.Valid || string(playbackEncode(r)) != c.Output {
			bad("restarts %s: the file says valid=%v, output %q; the rules give %v, %s", c.Name, c.Valid, c.Output, r.Join, playbackEncode(r))
		}
		if (c.Status == 200) != (c.Answer != "") || (c.Status == 200) == (c.Type != "") {
			bad("restarts %s: an answer goes with 200, a type with a refusal", c.Name)
		}
		if c.Valid && (c.Answer == "" || r.Offset != c.Reading.Offset) {
			bad("restarts %s: a join without an answer, or at another byte", c.Name)
		}
	}
	names("restarts", n)

	n = nil
	for _, c := range f.Limits {
		n = append(n, c.Name)
		got, err := playbackKeepLimits([]byte(c.Text))
		valid, _ := reason(err)
		if valid != c.Valid || string(got) != c.Output {
			bad("limits %s: the file says valid=%v, output %q; the rules give %v, %q", c.Name, c.Valid, c.Output, err, got)
		}
		output("limits "+c.Name, c.Output, c.Valid)
	}
	names("limits", n)
	return errs
}

// playbackPurposeOf is a play request's purpose ("play" when absent).
func playbackPurposeOf(text string) string {
	var o map[string]any
	_ = json.Unmarshal([]byte(text), &o)
	if p, ok := o["purpose"].(string); ok {
		return p
	}
	return "play"
}
