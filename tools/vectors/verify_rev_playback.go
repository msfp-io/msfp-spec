package main

import (
	"encoding/json"
	"fmt"
	"regexp"
	"slices"
	"strings"
)

// verifyRevPlayback checks the playback vectors (MSFP Media's playing, as
// MSFP's playback part carries it), read back from the files: each value
// on its own.
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
				// Section 2.6: a named copy plays, else the first.
				var req map[string]any
				_ = json.Unmarshal([]byte(c.Text), &req)
				copies := ctx.Copies[s.Title]
				v, _ := req["versionId"].(string)
				named := slices.ContainsFunc(copies, func(cp playbackCopy) bool { return cp.ID == v })
				if named && v != s.VersionID || !named && s.VersionID != copies[0].ID {
					bad("%s: copy %q, not the named one or the first", where, s.VersionID)
				}
				// Without delivery, the baseline; never the file.
				if (s.Delivery == "") != (s.Baseline != nil) {
					bad("%s: the baseline goes with no delivery", where)
				}
				// A stream left unchanged by the baseline is within it, and
				// a remux takes no transcode.
				if b := s.Baseline; b != nil {
					cp := playbackPickCopy(copies, v)
					a, _ := playbackPickAudio(cp, s.AudioTrack)
					if (b.Audio == "copy") != playbackAudioWithin(a) || (cp.Video != nil) != (b.Video != "") || cp.Video != nil && (b.Video == "copy") != playbackVideoWithin(*cp.Video) {
						bad("%s: the baseline leaves unchanged what isn't within it, or converts what is", where)
					}
					if (b.Method == "remux") != (s.Transcodes == "") {
						bad("%s: a baseline remux takes a transcode, or a conversion none", where)
					}
				}
				// A transcode copying both streams takes no transcode.
				if t := s.Transcode; t != nil && t.VideoCodec == "copy" && t.AudioCodec == "copy" && s.Transcodes != "" {
					bad("%s: video and audio copy take a transcode", where)
				}
				// No capabilities: the owning server never keeps them.
				if strings.Contains(c.Output, "capabilities") {
					bad("%s: capabilities kept", where)
				}
			}
			if c.Call == "play" && (c.Type == "delivery-refused" || c.Type == "target-refused") && c.Status != 409 {
				bad("%s: %s without 409", where, c.Type)
			}
			// A name that says the refusal says the right one.
			for _, t := range []string{"delivery-refused", "target-refused", "stream-limit-reached", "server-busy", "unreachable", "gone"} {
				if strings.Contains(c.Name, ": 409 "+t) || strings.Contains(c.Name, ": 404 "+t) || strings.Contains(c.Name, ": 503 "+t) {
					if c.Type != t {
						bad("%s: the name says %s, the case %q", where, t, c.Type)
					}
				}
			}
			// capabilities never starts a playback between servers.
			if c.Call == "play" && strings.Contains(c.Text, `"capabilities"`) && c.Status != 400 {
				bad("%s: capabilities not refused with 400", where)
			}
			// replaces: kept only when it names a live playback of the
			// caller, and then the old one ends with the 200.
			if c.Call == "play" && c.Valid {
				var s playbackStarted
				_ = json.Unmarshal([]byte(c.Output), &s)
				var req map[string]any
				_ = json.Unmarshal([]byte(c.Text), &req)
				r, _ := req["replaces"].(string)
				live := c.InUse != nil && slices.ContainsFunc(c.InUse.Live, func(l playbackLive) bool { return l.ID == r })
				if s.Replaces != "" && (s.Replaces != r || !live) || s.Replaces == "" && live {
					bad("%s: replaces %q kept, the request's %q", where, s.Replaces, r)
				}
				if s.IdleSeconds != 0 && (s.IdleSeconds != 2*s.ProgressSeconds || s.ProgressSeconds < 45 || s.ProgressSeconds > 120) {
					bad("%s: idleSeconds %d, progressSeconds %d", where, s.IdleSeconds, s.ProgressSeconds)
				}
			}
		}
		names(list, n)
	}
	calls("requests", f.Requests)
	calls("replaces", f.Replaces)
	for _, c := range f.Replaces {
		if c.Call != "play" || !strings.Contains(c.Text, `"replaces"`) {
			bad("replaces %s: not a play with replaces", c.Name)
		}
	}
	calls("deliveries", f.Deliveries)
	calls("baseline", f.Baseline)
	for _, c := range f.Baseline {
		if c.Call != "play" || strings.Contains(c.Text, `"delivery"`) {
			bad("baseline %s: not a play without delivery", c.Name)
		}
	}
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
		got, err := playbackKeepAnswer([]byte(c.Text), c.Asked, ctx.StreamRoot)
		valid, _ := reason(err)
		if valid != c.Valid || string(got) != c.Output {
			bad("answers %s: the file says valid=%v, output %q; the rules give %v, %q", c.Name, c.Valid, c.Output, err, got)
		}
		output("answers "+c.Name, c.Output, c.Valid)
		// Every URL kept is built: the stream root, the ticket, a fixed name.
		if c.Valid {
			var a playbackAnswer
			_ = json.Unmarshal([]byte(c.Output), &a)
			for _, u := range a.URLs {
				rest, ok := strings.CutPrefix(u, ctx.StreamRoot+a.Ticket+"/")
				if !ok || !playbackBuiltName.MatchString(rest) {
					bad("answers %s: %q isn't a name built from the ticket", c.Name, u)
				}
			}
			if len(a.URLs) == 0 || a.FontCount > 64 || len(a.Subtitles) > 64 {
				bad("answers %s: no stream URL, or counts past their bounds", c.Name)
			}
		}
	}
	names("answers", n)

	n = nil
	for _, c := range f.QueueEntries {
		n = append(n, c.Name)
		got, err := playbackQueueEntry(ctx, ctx.Limits, ctx.InUse, []byte(c.Request), []byte(c.Entry))
		valid, _ := reason(err)
		status, typ := playbackStatus(err)
		if valid != c.Valid || string(got) != c.Output || status != c.Status || typ != c.Type {
			bad("queueEntries %s: the file says valid=%v, output %q; the rules give %v, %q", c.Name, c.Valid, c.Output, err, got)
		}
		output("queueEntries "+c.Name, c.Output, c.Valid)
	}
	names("queueEntries", n)

	n = nil
	for _, c := range f.Refusals {
		n = append(n, c.Name)
		got := playbackReadRefusal(c.Status, c.Signed, []byte(c.Body), c.Purpose)
		if string(playbackEncode(got)) != c.Output {
			bad("refusals %s: the file says %s; the rules give %s", c.Name, c.Output, playbackEncode(got))
		}
		if !slices.Contains([]string{"asks again", "not playable", "busy now"}, got.Tells) || len(got.Instead) > 4 ||
			len(got.Instead) > 0 && got.Tells != "asks again" || got.DropTitle && c.Status != 404 {
			bad("refusals %s: %s out of the rules' forms", c.Name, c.Output)
		}
	}
	names("refusals", n)

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

// playbackBuiltName is a name fetching section 1 lets a viewer's server
// build after <stream root><ticket>/ from a play answer.
var playbackBuiltName = regexp.MustCompile(`^(master\.m3u8|file|(subtitles|ass|pictures)/s(0|[1-9][0-9]{0,2})|fonts/(0|[1-9][0-9]?))$`)

// playbackPurposeOf is a play request's purpose ("play" when absent).
func playbackPurposeOf(text string) string {
	var o map[string]any
	_ = json.Unmarshal([]byte(text), &o)
	if p, ok := o["purpose"].(string); ok {
		return p
	}
	return "play"
}
