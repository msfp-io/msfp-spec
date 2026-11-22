package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"unicode/utf8"
)

// The key forms of video section 4, written a second time as whole
// regular expressions, apart from parseVideoKey.
var videoKeyForms = func() map[string]*regexp.Regexp {
	o, s, e, id := `[A-Za-z0-9_-]{1,64}`, `(0|[1-9][0-9]{0,3})`, `(0|[1-9][0-9]{0,4})`, `[1-9][0-9]{0,9}`
	film := `movie/(tmdb/` + id + `|local/` + o + `)`
	series := `series/(tmdb/` + id + `|tvdb/` + id + `|local/` + o + `)`
	return map[string]*regexp.Regexp{
		"movie":   regexp.MustCompile(`^` + film + `$`),
		"series":  regexp.MustCompile(`^` + series + `$`),
		"season":  regexp.MustCompile(`^` + series + `/s` + s + `$`),
		"episode": regexp.MustCompile(`^(` + series + `/s` + s + `/e` + e + `|series/tmdb/` + id + `/e` + id + `|series/tvdb/` + id + `/a` + e + `)$`),
		"extra":   regexp.MustCompile(`^(` + film + `|` + series + `(/s` + s + `)?)/x/` + o + `$`),
	}
}()

// videoKindOfForm is the kind whose form key matches, or "".
func videoKindOfForm(key string) string {
	found := ""
	for kind, re := range videoKeyForms {
		if re.MatchString(key) {
			if found != "" {
				return "two kinds"
			}
			found = kind
		}
	}
	return found
}

// testCasePointers lists the arrays a spec file says are in
// vectors/v0/video.json ("Test cases: `a` and `b` in [vectors/v0/video.json]").
func videoTestCasePointers(spec []byte) []string {
	var out []string
	for _, para := range strings.Split(string(spec), "\n\n") {
		i := strings.Index(para, "Test cases")
		if i < 0 || !strings.Contains(para[i:], "vectors/v0/video.json") {
			continue
		}
		head := para[i:strings.Index(para, "vectors/v0/video.json")]
		if j := strings.Index(head, " in"); j >= 0 {
			head = head[:j]
		}
		for _, m := range regexp.MustCompile("`([A-Za-z]+)`").FindAllStringSubmatch(head, -1) {
			out = append(out, m[1])
		}
	}
	return out
}

// verifyRevVideo checks the video vectors of the revision of version 0
// (proposal 16), read back from the files: each value on its own.
func verifyRevVideo(read func(string) []byte) []error {
	var errs []error
	bad := func(format string, a ...any) { errs = append(errs, fmt.Errorf("v0/video.json: "+format, a...)) }
	b := read("v0/video.json")
	if err := checkIJSON(b); err != nil {
		bad("%v", err)
	}
	var f map[string]json.RawMessage
	if err := json.Unmarshal(b, &f); err != nil {
		bad("%v", err)
		return errs
	}
	var libs []string
	var root string
	_ = json.Unmarshal(f["libraries"], &libs)
	_ = json.Unmarshal(f["streamRoot"], &root)
	if len(libs) == 0 || root == "" || !strings.HasPrefix(root, "https://") || !strings.HasSuffix(root, "/api/v1/stream/") {
		bad("libraries or streamRoot missing or out of form")
	}

	// 1. Every array the specification points to is here, has a valid and
	// an invalid case, and unique names.
	type vcase struct {
		Name, Text, Output, Reason, Kind, Side string
		Valid                                  bool
	}
	arrays := map[string][]vcase{}
	for name, raw := range f {
		var list []vcase
		if json.Unmarshal(raw, &list) == nil && len(raw) > 0 && raw[0] == '[' && name != "libraries" && name != "about" {
			arrays[name] = list
		}
	}
	pointed := append(videoTestCasePointers(read("../spec/v0/video.md")), videoTestCasePointers(read("../spec/v0/catalog.md"))...)
	if len(pointed) < 7 {
		bad("the specification points to %d arrays (%v), fewer than expected", len(pointed), pointed)
	}
	for _, p := range pointed {
		if _, ok := arrays[p]; !ok {
			bad("the specification points to %q, which isn't here", p)
		}
	}
	for name, list := range arrays {
		seen := map[string]bool{}
		valid, invalid := 0, 0
		for _, c := range list {
			if c.Name == "" || seen[c.Side+c.Name] {
				bad("%s: a missing or repeated name %q", name, c.Name)
			}
			seen[c.Side+c.Name] = true
			if c.Valid {
				valid++
				if c.Reason != "" {
					bad("%s %q: valid with a reason", name, c.Name)
				}
			} else {
				invalid++
				if c.Reason == "" || c.Output != "" {
					bad("%s %q: invalid without a reason, or with an output", name, c.Name)
				}
			}
		}
		if valid == 0 || invalid == 0 {
			bad("%s: %d valid and %d invalid cases; each array needs both", name, valid, invalid)
		}
	}

	// 2. Every output follows from its text by the rules, and keeping
	// what was kept changes nothing.
	same := func(where string, c vcase, got []byte, err error) {
		ok, r := videoReason(err)
		if ok != c.Valid || string(got) != c.Output || r != c.Reason {
			bad("%s %q: the file says valid=%v, output %q, reason %q; the rules give %v, %q, %q", where, c.Name, c.Valid, c.Output, c.Reason, ok, got, r)
		}
		if ok && got != nil {
			if e := checkIJSON(got); e != nil {
				bad("%s %q: output: %v", where, c.Name, e)
			}
		}
	}
	for _, arr := range []string{"titles", "extras", "localTitles"} {
		for _, c := range arrays[arr] {
			got, err := keepVideoTitle([]byte(c.Text), libs)
			same(arr, c, got, err)
			if !c.Valid {
				continue
			}
			again, err := keepVideoTitle([]byte(c.Output), libs)
			if err != nil || !bytes.Equal(again, []byte(c.Output)) {
				bad("%s %q: keeping the output again gives %q, %v", arr, c.Name, again, err)
			}
			for _, e := range videoCheckKeptTitle([]byte(c.Output), libs) {
				bad("%s %q: %v", arr, c.Name, e)
			}
		}
	}
	for _, c := range arrays["copies"] {
		got, err := keepVideoCopy([]byte(c.Text))
		same("copies", c, got, err)
		if !c.Valid {
			continue
		}
		if again, err := keepVideoCopy([]byte(c.Output)); err != nil || !bytes.Equal(again, []byte(c.Output)) {
			bad("copies %q: keeping the output again gives %q, %v", c.Name, again, err)
		}
		var cp map[string]any
		_ = json.Unmarshal([]byte(c.Output), &cp)
		for _, e := range videoCheckKeptCopy(cp) {
			bad("copies %q: %v", c.Name, e)
		}
	}
	for _, arr := range []string{"localKeys", "tvdbKeys"} {
		scheme := map[string]string{"localKeys": "/local/", "tvdbKeys": "/tvdb/"}[arr]
		for _, c := range arrays[arr] {
			kind, err := checkVideoKey(c.Text)
			ok, r := videoReason(err)
			if ok != c.Valid || kind != c.Kind || r != c.Reason {
				bad("%s %q: the file says valid=%v, kind %q; the rules give %v, %q (%s)", arr, c.Name, c.Valid, c.Kind, ok, kind, r)
			}
			if form := videoKindOfForm(c.Text); form != c.Kind {
				bad("%s %q: the key's form names %q, the file says %q", arr, c.Name, form, c.Kind)
			}
			if c.Valid && !strings.Contains(c.Text, scheme) {
				bad("%s %q: not a key of the scheme of its array", arr, c.Name)
			}
		}
	}
	for _, c := range arrays["previews"] {
		switch c.Side {
		case "request":
			same("previews request", c, nil, checkVideoPreviewsRequest([]byte(c.Text)))
		case "answer":
			got, err := keepVideoPreviewsAnswer([]byte(c.Text), root)
			same("previews answer", c, got, err)
			if !c.Valid {
				continue
			}
			if again, err := keepVideoPreviewsAnswer([]byte(c.Output), root); err != nil || !bytes.Equal(again, []byte(c.Output)) {
				bad("previews answer %q: keeping the output again gives %q, %v", c.Name, again, err)
			}
			var a struct{ Trickplay map[string]any }
			_ = json.Unmarshal([]byte(c.Output), &a)
			if t := a.Trickplay; t != nil {
				n := func(k string) int64 { x, _ := t[k].(float64); return int64(x) }
				sheets, _ := t["sheets"].([]any)
				per := n("columns") * n("rows")
				if per == 0 || int64(len(sheets)) != (n("tileCount")+per-1)/per || len(sheets) > 1000 ||
					n("columns")*n("tileWidth") > 4096 || n("rows")*n("tileHeight") > 4096 {
					bad("previews answer %q: kept previews out of bounds", c.Name)
				}
				for i, s := range sheets {
					if u, _ := s.(string); !strings.HasPrefix(u, root) || !strings.HasSuffix(u, fmt.Sprintf("/trickplay/%d", i)) {
						bad("previews answer %q: kept sheet %d isn't under the stream root at its index", c.Name, i)
					}
				}
			}
		default:
			bad("previews %q: side %q", c.Name, c.Side)
		}
	}
	return errs
}

// videoCheckKeptTitle checks a kept title against the bounds of
// sections 3.1, 3.9, 3.10 and 4, read from the JSON.
func videoCheckKeptTitle(out []byte, libs []string) []error {
	var errs []error
	bad := func(format string, a ...any) { errs = append(errs, fmt.Errorf(format, a...)) }
	var t map[string]any
	_ = json.Unmarshal(out, &t)
	key, kind := str(t, "key"), str(t, "kind")
	if videoKindOfForm(key) != kind {
		bad("kept key %q isn't of the form of kind %q", key, kind)
	}
	if !videoIn(str(t, "library"), libs) {
		bad("kept library %q wasn't asked for", str(t, "library"))
	}
	local := strings.Contains(key, "/local/") && !strings.Contains(key, "/x/")
	copies, hasCopies := t["copies"].([]any)
	switch kind {
	case "movie", "episode", "extra":
		if len(copies) == 0 || len(copies) > 8 {
			bad("%d copies kept, not 1 to 8", len(copies))
		}
		ids := map[string]bool{}
		for _, c := range copies {
			m, _ := c.(map[string]any)
			if ids[str(m, "id")] {
				bad("copy %q kept twice", str(m, "id"))
			}
			ids[str(m, "id")] = true
			errs = append(errs, videoCheckKeptCopy(m)...)
		}
	default:
		if hasCopies {
			bad("copies kept on a %s", kind)
		}
	}
	_, place := t["place"]
	if place != regexp.MustCompile(`^series/tmdb/[0-9]+/e[0-9]+$`).MatchString(key) {
		bad("place kept: %v, on key %q", place, key)
	}
	if w, ok := t["with"].([]any); ok {
		if kind != "episode" || len(w) > 16 {
			bad("with kept on a %s, or more than 16", kind)
		}
		series := strings.Join(strings.SplitN(key, "/", 4)[:3], "/")
		for _, e := range w {
			s, _ := e.(string)
			if s == key || videoKindOfForm(s) != "episode" || !strings.HasPrefix(s, series+"/") {
				bad("with keeps %q", s)
			}
		}
	}
	if _, ok := t["ids"]; ok && (strings.Contains(key, "/local/") || kind == "season" || kind == "extra") {
		bad("ids kept on a %s %q", kind, key)
	}
	if x, ok := t["extraType"]; ok != (kind == "extra") || ok && !videoIn(x.(string), videoExtraTypes) {
		bad("extraType %v on a %s", x, kind)
	}
	name, hasName := t["name"].(string)
	switch {
	case kind == "extra":
		if hasName && (name == "" || utf8.RuneCountInString(name) > 64) {
			bad("an extra's name of %d characters", utf8.RuneCountInString(name))
		}
	case local && (kind == "movie" || kind == "series"):
		if !hasName || name == "" || utf8.RuneCountInString(name) > 128 {
			bad("a local title's name missing or over 128 characters")
		}
	default:
		if hasName {
			bad("a name kept on a %s %q", kind, key)
		}
	}
	if y, ok := t["year"].(float64); ok && (y < 1800 || y > 9999 || !local) {
		bad("year %v kept", y)
	}
	if _, ok := t["image"]; ok && !(local && (kind == "movie" || kind == "series")) {
		bad("image kept on a %s %q", kind, key)
	}
	return errs
}

// videoCheckKeptCopy checks a kept copy against the bounds of section 3.8,
// read from the JSON.
func videoCheckKeptCopy(c map[string]any) []error {
	var errs []error
	bad := func(format string, a ...any) {
		errs = append(errs, fmt.Errorf("copy %q: "+format, append([]any{str(c, "id")}, a...)...))
	}
	n := func(m map[string]any, k string) (int64, bool) { x, ok := m[k].(float64); return int64(x), ok }
	d, _ := n(c, "durationMs")
	if d < 0 || d > videoMaxDuration {
		bad("durationMs %d", d)
	}
	if s, _ := n(c, "size"); s < 1 || s > videoMaxSize {
		bad("size %d", s)
	}
	if !videoIn(str(c, "container"), append(videoContainers, "other")) {
		bad("container %q", str(c, "container"))
	}
	if utf8.RuneCountInString(str(c, "name")) > 32 {
		bad("name over 32 characters")
	}
	if b, ok := n(c, "bitrateKbps"); ok && (b%500 != 0 || b < 500 || b > 10_000_000) {
		bad("bitrateKbps %d", b)
	}
	if s, ok := n(c, "startMs"); ok && (s < 0 || s > d) {
		bad("startMs %d", s)
	}
	if v, ok := c["video"].(map[string]any); ok {
		if !videoIn(str(v, "codec"), append(videoCodecs, "other")) {
			bad("video codec %q", str(v, "codec"))
		}
		hdr, _ := v["hdr"].([]any)
		last := -1
		for _, h := range hdr {
			i := strings.Index(" hdr10 hdr10plus hlg dv ", " "+h.(string)+" ")
			if i <= last {
				bad("hdr out of the order of section 3.3")
			}
			last = i
		}
		if _, ok := v["dv"]; ok && !strings.Contains(fmt.Sprint(hdr), "dv") {
			bad("dv kept without dv in hdr")
		}
	}
	audio, _ := c["audioTracks"].([]any)
	if _, ok := c["audioTracks"]; !ok || len(audio) > 32 {
		bad("audioTracks missing or over 32")
	}
	subs, _ := c["subtitleTracks"].([]any)
	if len(subs) > 64 {
		bad("over 64 subtitle tracks")
	}
	for _, list := range [][]any{audio, subs} {
		ids := map[string]bool{}
		for _, e := range list {
			m, _ := e.(map[string]any)
			if ids[str(m, "id")] || utf8.RuneCountInString(str(m, "title")) > 32 {
				bad("track %q repeated, or its title over 32 characters", str(m, "id"))
			}
			ids[str(m, "id")] = true
		}
	}
	chapters, _ := c["chapters"].([]any)
	prev := int64(-1)
	for _, e := range chapters {
		s, _ := n(e.(map[string]any), "startMs")
		if s <= prev || s > d || utf8.RuneCountInString(str(e.(map[string]any), "name")) > 64 {
			bad("chapter at %d out of order or range, or its name too long", s)
		}
		prev = s
	}
	segs, _ := c["segments"].([]any)
	if len(chapters) > 256 || len(segs) > 32 {
		bad("over 256 chapters or 32 segments")
	}
	for _, e := range segs {
		m := e.(map[string]any)
		s, _ := n(m, "startMs")
		t, _ := n(m, "endMs")
		if s < 0 || s >= t || t > d || !videoIn(str(m, "kind"), videoSegKinds) {
			bad("segment %v", m)
		}
	}
	return errs
}
