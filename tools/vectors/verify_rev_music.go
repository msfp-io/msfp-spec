package main

import (
	"encoding/json"
	"fmt"
)

// verifyRevMusic checks the music vectors of the revision of version 0
// (proposals 16 and 17), read back from the files: each case's output re-derived
// from its text by music.go's rules, and the file's shape.
func verifyRevMusic(read func(string) []byte) []error {
	const file = "v0/music.json"
	var errs []error
	bad := func(format string, a ...any) { errs = append(errs, fmt.Errorf(file+": "+format, a...)) }
	var f obj
	if err := json.Unmarshal(read(file), &f); err != nil {
		return []error{fmt.Errorf("%s: %v", file, err)}
	}
	for _, k := range []string{"about", "libraries", "keys", "titles", "localTitles", "queue", "lyrics"} {
		if _, ok := f[k]; !ok {
			bad("no %s", k)
		}
	}
	var libs []string
	if l, ok := f["libraries"].([]any); ok {
		for _, e := range l {
			s, _ := e.(string)
			libs = append(libs, s)
		}
	}
	if _, ok := f["streamRoot"]; ok {
		bad("a streamRoot: queue entries carry no URL since proposal 17")
	}

	// Each array: cases with unique names, some valid and some not, the
	// output present exactly when valid (an answer's output, a request's
	// never), every text and output I-JSON.
	cases := func(name string, withOutput func(x obj) bool) []obj {
		list, ok := f[name].([]any)
		if !ok || len(list) == 0 {
			bad("%s: not a non-empty array", name)
			return nil
		}
		var out []obj
		seen := map[string]bool{}
		valid, invalid := 0, 0
		for i, v := range list {
			x, ok := v.(obj)
			if !ok {
				bad("%s[%d]: not an object", name, i)
				continue
			}
			n := str(x, "name")
			if n == "" || seen[n] {
				bad("%s[%d]: a name empty or repeated: %q", name, i, n)
			}
			seen[n] = true
			if x["valid"] == true {
				valid++
				if (str(x, "reason") != "") || (withOutput(x) != (str(x, "output") != "")) {
					bad("%s %q: valid, but its reason or output don't fit", name, n)
				}
			} else {
				invalid++
				if str(x, "reason") == "" || str(x, "output") != "" {
					bad("%s %q: invalid, but without a reason or with an output", name, n)
				}
			}
			if name != "keys" {
				if err := checkIJSON([]byte(str(x, "text"))); err != nil {
					bad("%s %q: text: %v", name, n, err)
				}
				if o := str(x, "output"); o != "" {
					if err := checkIJSON([]byte(o)); err != nil {
						bad("%s %q: output: %v", name, n, err)
					}
				}
			}
			out = append(out, x)
		}
		if valid == 0 || invalid == 0 {
			bad("%s: %d valid and %d invalid cases: each array needs both", name, valid, invalid)
		}
		return out
	}
	always := func(obj) bool { return true }
	answersOnly := func(x obj) bool { return str(x, "kind") == "answer" }

	for _, x := range cases("keys", func(obj) bool { return false }) {
		kind, err := checkMusicKey(str(x, "text"))
		if (err == nil) != (x["valid"] == true) || kind != str(x, "kind") {
			bad("key %q: the file says valid=%v, kind %q; the rules give %v, %q", str(x, "text"), x["valid"], str(x, "kind"), err, kind)
		}
	}

	for _, arr := range []string{"titles", "localTitles"} {
		for _, x := range cases(arr, always) {
			got, err := keepMusicTitle([]byte(str(x, "text")), libs)
			if (err == nil) != (x["valid"] == true) || string(got) != str(x, "output") {
				bad("%s %q: the file says valid=%v, output %q; the rules give %v, %q", arr, str(x, "name"), x["valid"], str(x, "output"), err, got)
				continue
			}
			if err != nil {
				continue
			}
			// What is kept is itself a title the caller keeps. (Not always
			// unchanged: core section 9.1's cleaning, applied again, may
			// join two spaces a format character left.)
			if _, err := keepMusicTitle(got, libs); err != nil {
				bad("%s %q: the output, kept again, is skipped: %v", arr, str(x, "name"), err)
			}
			var t musicTitle
			if err := json.Unmarshal(got, &t); err != nil {
				bad("%s %q: output: %v", arr, str(x, "name"), err)
				continue
			}
			kind, scheme, _ := musicParseKey(t.Key)
			switch {
			case len(t.Artists) > 16, len(t.Genres) > 8, len(t.SecondaryTypes) > 8, len(t.Copies) > 16:
				bad("%s %q: more artists, genres, secondary types or copies than allowed", arr, str(x, "name"))
			case t.PrimaryType != "" && !musicType(t.PrimaryType):
				bad("%s %q: a primaryType kept out of its form", arr, str(x, "name"))
			case kind == "track" && len(t.Copies) == 0:
				bad("%s %q: a track kept without a copy", arr, str(x, "name"))
			case t.Image != nil && (scheme != "local" || kind == "track"):
				bad("%s %q: image kept on a title that isn't a local artist or album", arr, str(x, "name"))
			case t.IDs != nil && (scheme != "mb" || kind == "artist"):
				bad("%s %q: ids kept on a title that isn't an mb album or track", arr, str(x, "name"))
			}
			for s := range t.IDs {
				if musicIDKinds[s] != kind {
					bad("%s %q: an ids scheme %q kept on a %s", arr, str(x, "name"), s, kind)
				}
			}
			for _, st := range t.SecondaryTypes {
				if !musicType(st) {
					bad("%s %q: a secondary type %q kept out of its form", arr, str(x, "name"), st)
				}
			}
			for _, c := range t.Copies {
				if !musicMediaType(c.MediaType) || !openToken(c.Codec) {
					bad("%s %q: a copy's mediaType or codec kept out of its form", arr, str(x, "name"))
				}
			}
		}
	}

	for _, x := range cases("queue", answersOnly) {
		var got []byte
		var err error
		switch str(x, "kind") {
		case "request":
			err = musicCheckQueueRequest([]byte(str(x, "text")))
		case "answer":
			got, err = musicKeepQueueAnswer([]byte(str(x, "request")), []byte(str(x, "text")))
			if err == nil {
				var kept []musicQueueEntry
				if e := json.Unmarshal(got, &kept); e != nil || len(kept) > 5 {
					bad("queue %q: output isn't an array of at most 5 entries", str(x, "name"))
				}
				for _, k := range kept {
					if k.Protocol != "hls" && k.Protocol != "file" {
						bad("queue %q: an entry kept with protocol %q", str(x, "name"), k.Protocol)
					}
				}
			}
		default:
			bad("queue %q: kind %q", str(x, "name"), str(x, "kind"))
		}
		if (err == nil) != (x["valid"] == true) || string(got) != str(x, "output") {
			bad("queue %q: the file says valid=%v, output %q; the rules give %v, %q", str(x, "name"), x["valid"], str(x, "output"), err, got)
		}
	}

	for _, x := range cases("lyrics", answersOnly) {
		var got []byte
		var err error
		switch str(x, "kind") {
		case "request":
			err = musicCheckLyricsRequest([]byte(str(x, "text")))
		case "answer":
			got, err = musicKeepLyrics([]byte(str(x, "text")))
			if err == nil {
				if again, e := musicKeepLyrics(got); e != nil || string(again) != string(got) {
					bad("lyrics %q: the output, kept again, fails or changes: %v", str(x, "name"), e)
				}
			}
		default:
			bad("lyrics %q: kind %q", str(x, "name"), str(x, "kind"))
		}
		if (err == nil) != (x["valid"] == true) || string(got) != str(x, "output") {
			bad("lyrics %q: the file says valid=%v, output %q; the rules give %v, %q", str(x, "name"), x["valid"], str(x, "output"), err, got)
		}
	}
	return errs
}
