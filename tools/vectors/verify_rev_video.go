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
			got, err := keepVideoPreviewsAnswer([]byte(c.Text))
			same("previews answer", c, got, err)
			if !c.Valid {
				continue
			}
			if again, err := keepVideoPreviewsAnswer([]byte(c.Output)); err != nil || !bytes.Equal(again, []byte(c.Output)) {
				bad("previews answer %q: keeping the output again gives %q, %v", c.Name, again, err)
			}
			var a map[string]any
			_ = json.Unmarshal([]byte(c.Output), &a)
			form := regexp.MustCompile(`^[A-Za-z0-9_-]{1,64}$`)
			if len(a) != 2 || !form.MatchString(str(a, "ticket")) || !form.MatchString(str(a, "etag")) {
				bad("previews answer %q: kept %v, not a ticket and an etag in their forms", c.Name, a)
			}
		default:
			bad("previews %q: side %q", c.Name, c.Side)
		}
	}
	var answerCopy string
	_ = json.Unmarshal(f["answerCopy"], &answerCopy)
	acv, _ := videoDecode([]byte(answerCopy))
	acp, err := keepCopy(acv)
	if err != nil {
		bad("answerCopy: %v", err)
		return errs
	}
	var ac map[string]any
	_ = json.Unmarshal([]byte(answerCopy), &ac)
	sheets := int64(0)
	if l, ok := ac["previewLayout"].(map[string]any); ok {
		n := func(k string) int64 { x, _ := l[k].(float64); return int64(x) }
		sheets = (n("tileCount") + n("columns")*n("rows") - 1) / (n("columns") * n("rows"))
	}
	for _, c := range arrays["answers"] {
		got, err := keepVideoAnswer([]byte(c.Text), acp, root)
		same("answers", c, got, err)
		if !c.Valid {
			continue
		}
		var in, out map[string]any
		_ = json.Unmarshal([]byte(c.Text), &in)
		_ = json.Unmarshal([]byte(c.Output), &out)
		base := root + str(in, "ticket") + "/"
		subs, _ := out["subtitles"].([]any)
		fonts, _ := out["fontCount"].(float64)
		urls, _ := out["urls"].([]any)
		if len(subs) > 64 || fonts < 0 || fonts > 64 {
			bad("answers %q: %d subtitles or %v fonts kept", c.Name, len(subs), fonts)
		}
		want := 0
		for _, e := range subs {
			m, _ := e.(map[string]any)
			want++
			if m["ass"] == true {
				want++
				if str(m, "form") != "text" {
					bad("answers %q: ass kept on a picture track", c.Name)
				}
			}
		}
		if len(urls) != want+int(fonts)+int(sheets) {
			bad("answers %q: %d URLs, not %d", c.Name, len(urls), want+int(fonts)+int(sheets))
		}
		name := regexp.MustCompile(`^(subtitles|pictures|ass)/s(0|[1-9][0-9]{0,2})$|^(fonts|trickplay)/(0|[1-9][0-9]*)$`)
		for _, u := range urls {
			s, _ := u.(string)
			if rest, ok := strings.CutPrefix(s, base); !ok || !name.MatchString(rest) {
				bad("answers %q: URL %q isn't built from the stream root, the ticket and a fixed name", c.Name, s)
			}
		}
	}
	required := map[string]bool{"h264": true, "hevc": true, "av1": true, "vp9": true, "aac": true, "dts": true}
	for _, c := range arrays["codecs"] {
		got, err := readVideoCodecs([]byte(c.Text))
		same("codecs", c, got, err)
		var in, out map[string]any
		_ = json.Unmarshal([]byte(c.Text), &in)
		_ = json.Unmarshal([]byte(c.Output), &out)
		if c.Valid && required[str(in, "codec")] && str(out, "codecs") != str(in, "codecs") {
			bad("codecs %q: a required codecs string not kept as it is", c.Name)
		}
		if !c.Valid && required[str(in, "codec")] && !strings.Contains(c.Reason, "can't play as it is") {
			bad("codecs %q: refused without saying the copy can't play as it is", c.Name)
		}
		if b, ok := in["bitDepth"]; ok && c.Valid && out["bitDepth"] != b {
			bad("codecs %q: bitDepth %v isn't the stream's own %v", c.Name, out["bitDepth"], b)
		}
	}
	for _, c := range arrays["fontSets"] {
		got, err := checkVideoFontSet([]byte(c.Text))
		same("fontSets", c, got, err)
		var sizes []int64
		if json.Unmarshal([]byte(c.Text), &sizes) != nil {
			continue
		}
		var total int64
		within := len(sizes) <= 64
		for _, n := range sizes {
			total += n
			within = within && n >= 1 && n <= 10_485_760
		}
		if within = within && total <= 67_108_864; within != c.Valid {
			bad("fontSets %q: %d fonts, %d bytes in all; the file says valid=%v", c.Name, len(sizes), total, c.Valid)
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
	key := str(t, "key")
	kind := videoKindOfForm(key)
	if kind == "" || kind == "two kinds" {
		bad("kept key %q isn't of the form of one kind", key)
	}
	if _, ok := t["kind"]; ok {
		bad("a kind kept on %q", key)
	}
	if !videoIn(str(t, "library"), libs) {
		bad("kept library %q wasn't asked for", str(t, "library"))
	}
	local := strings.Contains(key, "/local/") && !strings.Contains(key, "/x/")
	copies, hasCopies := t["copies"].([]any)
	switch kind {
	case "movie", "episode", "extra":
		if len(copies) == 0 || len(copies) > 16 {
			bad("%d copies kept, not 1 to 16", len(copies))
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
	if _, ok := t["ids"]; ok && !(kind == "episode" && !strings.Contains(key, "/local/") || kind == "series" && strings.HasPrefix(key, "series/tvdb/")) {
		bad("ids kept on a %s %q", kind, key)
	}
	if x, ok := t["extraType"]; ok != (kind == "extra") || ok && openTokenValue(x) == "" {
		bad("extraType %v on a %s", x, kind)
	}
	name, hasName := t["name"].(string)
	switch {
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
	for _, k := range []string{"name", "container", "bitrateKbps", "added", "previews"} {
		if _, ok := c[k]; ok {
			bad("a dropped member %s kept", k)
		}
	}
	d, _ := n(c, "durationMs")
	if d < 0 || d > videoMaxDuration {
		bad("durationMs %d", d)
	}
	if s, _ := n(c, "size"); s < 1 || s > videoMaxSize {
		bad("size %d", s)
	}
	if !videoMediaType(str(c, "mediaType")) {
		bad("mediaType %q isn't a media type", str(c, "mediaType"))
	}
	for _, k := range []string{"edition", "threeD"} {
		if e, ok := c[k]; ok && openTokenValue(e) == "" {
			bad("%s %v isn't a token", k, e)
		}
	}
	s, hasStart := n(c, "startMs")
	if hasStart && (s < 0 || s > d) {
		bad("startMs %d", s)
	}
	if e, ok := n(c, "endMs"); ok && (e <= s || e > d) {
		bad("endMs %d not above startMs %d and at most durationMs", e, s)
	}
	if p, ok := c["parts"]; ok {
		if x, isNum := p.(float64); !isNum || x < 2 || x > 16 {
			bad("parts %v isn't a count from 2 to 16", p)
		}
	}
	if l, ok := c["previewLayout"].(map[string]any); ok {
		w, _ := n(l, "tileWidth")
		h, _ := n(l, "tileHeight")
		cols, _ := n(l, "columns")
		rows, _ := n(l, "rows")
		iv, _ := n(l, "intervalMs")
		tc, _ := n(l, "tileCount")
		if len(l) != 6 || w < 1 || w > 2048 || h < 1 || h > 2048 || cols < 1 || cols > 100 || rows < 1 || rows > 100 ||
			iv < 100 || iv > 3_600_000 || tc < 1 || tc > 1000*cols*rows || cols*w > 4096 || rows*h > 4096 {
			bad("previewLayout %v out of bounds", l)
		}
	}
	if v, ok := c["video"].(map[string]any); ok {
		for _, k := range []string{"codecTag", "profile", "level"} {
			if _, ok := v[k]; ok {
				bad("a dropped member %s kept on video", k)
			}
		}
		if !openToken(str(v, "codec")) {
			bad("video codec %q isn't a token", str(v, "codec"))
		}
		hdr, _ := v["hdr"].([]any)
		if len(hdr) > 8 {
			bad("over 8 hdr entries")
		}
		last := ""
		for _, h := range hdr {
			x, _ := h.(string)
			if !openToken(x) || x <= last {
				bad("hdr entry %q not a token, repeated or out of byte order", x)
			}
			last = x
		}
		if dv, ok := v["dv"].(map[string]any); ok {
			p, _ := n(dv, "profile")
			l, _ := n(dv, "level")
			cp, _ := n(dv, "compatibility")
			_, bl := dv["bl"]
			_, rpu := dv["rpu"]
			if p < 0 || p > 127 || l < 0 || l > 63 || cp < 0 || cp > 15 || bl || rpu {
				bad("dv %v", dv)
			}
		}
		if col, ok := v["colour"].(map[string]any); ok {
			if len(col) == 0 {
				bad("an empty colour kept")
			}
			for _, k := range []string{"primaries", "transfer", "matrix"} {
				if x, ok := col[k]; ok {
					if f, isNum := x.(float64); !isNum || f < 0 || f > 255 || f == 2 || f != float64(int64(f)) {
						bad("colour %s %v isn't an H.273 code point", k, x)
					}
				}
			}
			if x, ok := col["fullRange"]; ok {
				if _, isBool := x.(bool); !isBool {
					bad("fullRange %v isn't a boolean", x)
				}
			}
		}
		if f, ok := v["fps"]; ok {
			var a, b int64
			if k, _ := fmt.Sscanf(f.(string), "%d/%d", &a, &b); k != 2 || fmt.Sprintf("%d/%d", a, b) != f || a < 1 || b < 1 || a > 1e6 || b > 1e6 || videoGCD(a, b) != 1 {
				bad("fps %q isn't a reduced fraction of 1 to 1,000,000", f)
			}
		}
		if r, ok := n(v, "rotation"); ok && r != 0 && r != 90 && r != 180 && r != 270 {
			bad("rotation %d", r)
		}
		if b, ok := n(v, "bitDepth"); ok && (b < 1 || b > 16) {
			bad("bitDepth %d", b)
		}
		if r, ok := n(v, "refFrames"); ok && (r < 1 || r > 16 || str(v, "codec") != "h264") {
			bad("refFrames %d on %s", r, str(v, "codec"))
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
			if ids[str(m, "id")] {
				bad("track %q repeated", str(m, "id"))
			}
			for _, k := range []string{"title", "profile", "layout", "fileIndex"} {
				if _, ok := m[k]; ok {
					bad("track %q: a dropped member %s kept", str(m, "id"), k)
				}
			}
			for _, k := range []string{"codec", "role", "format", "objects"} {
				if x, ok := m[k]; ok && openTokenValue(x) == "" {
					bad("track %q: %s %v isn't a token", str(m, "id"), k, x)
				}
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
		_, final := m["final"]
		if s < 0 || s >= t || t > d || openTokenValue(m["kind"]) == "" || final {
			bad("segment %v", m)
		}
	}
	return errs
}
