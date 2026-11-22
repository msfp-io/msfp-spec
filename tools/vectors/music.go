package main

// The music part's rules (spec/v0/music.md, revised by proposals 16
// and 17):
// title keys under the mb and local schemes (section 2), what a caller
// keeps of a music title and of a track's copies (sections 3.2 and 3.3),
// the queue of a play (section 5.2) and the lyrics call (section 6).
// Names are cleaned as core section 9.1 cleans a server's name.

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"
)

var (
	mbidForm        = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`)
	isrcForm        = regexp.MustCompile(`^[A-Z]{2}[A-Z0-9]{3}[0-9]{7}$`)
	musicOpaqueForm = regexp.MustCompile(`^[A-Za-z0-9_-]{1,64}$`) // catalog section 4.2
	musicIDForm     = regexp.MustCompile(`^[A-Za-z0-9_-]{1,64}$`) // version IDs, etags, tickets
	musicDateForm   = regexp.MustCompile(`^([0-9]{4})(?:-([0-9]{2})(?:-([0-9]{2}))?)?$`)
	musicDayForm    = regexp.MustCompile(`^[0-9]{4}-[0-9]{2}-[0-9]{2}$`)
	musicMediaName  = regexp.MustCompile(`^[A-Za-z0-9._-]{1,64}$`)
	musicTypeForm   = regexp.MustCompile(`^[a-z0-9._/-](?:[a-z0-9 ._/-]{0,30}[a-z0-9._/-])?$`) // music section 3
	musicCodecsForm = regexp.MustCompile(`^[A-Za-z0-9.-]{1,64}$`)                              // RFC 6381, video section 3.3

	musicMethods = []string{"direct", "remux", "transcode"}
)

// musicType: a MusicBrainz release-group type as music section 3 sends
// it: lowercased, 1 to 32 characters of a-z 0-9 space . _ - /, no space
// at either end.
func musicType(s string) bool { return musicTypeForm.MatchString(s) }

// checkMBID: 36 characters, lowercase hex in groups of 8-4-4-4-12.
func checkMBID(s string) error {
	if len(s) != 36 {
		return fmt.Errorf("an MBID of %d characters, not 36", len(s))
	}
	if !mbidForm.MatchString(s) {
		return errors.New("not a lowercase MBID (8-4-4-4-12 lowercase hex)")
	}
	return nil
}

// musicParseKey returns the kind and the scheme a music title key names
// (music section 2): <kind>/mb/<mbid> or <kind>/local/<opaque>.
func musicParseKey(key string) (kind, scheme string, err error) {
	p := strings.Split(key, "/")
	if len(p) != 3 {
		return "", "", fmt.Errorf("%d parts joined by /, not 3", len(p))
	}
	switch p[0] {
	case "artist", "album", "track":
	default:
		return "", "", fmt.Errorf("kind %q is not a music kind", p[0])
	}
	switch p[1] {
	case "mb":
		if err := checkMBID(p[2]); err != nil {
			return "", "", err
		}
	case "local":
		if !musicOpaqueForm.MatchString(p[2]) {
			return "", "", errors.New("an opaque part that isn't 1 to 64 characters of A-Z a-z 0-9 _ -")
		}
	default:
		return "", "", fmt.Errorf("scheme %q is neither mb nor local", p[1])
	}
	return p[0], p[1], nil
}

// checkMusicKey returns the kind a music title key names.
func checkMusicKey(key string) (string, error) {
	kind, _, err := musicParseKey(key)
	return kind, err
}

// musicKeyOf: s is a valid music key of that kind, of either scheme.
func musicKeyOf(s, kind string) bool {
	k, _, err := musicParseKey(s)
	return err == nil && k == kind
}

// cleanName is core section 9.1's cleaning, with max code points
// instead of 80.
func cleanName(s string, max int) string {
	var b strings.Builder
	space := false
	for _, r := range s { // 1. runs of White_Space become one space
		if unicode.Is(unicode.White_Space, r) {
			if !space {
				b.WriteByte(' ')
			}
			space = true
			continue
		}
		space = false
		b.WriteRune(r)
	}
	t := strings.Map(func(r rune) rune { // 2. Cc and Cf removed
		if unicode.Is(unicode.Cc, r) || unicode.Is(unicode.Cf, r) {
			return -1
		}
		return r
	}, b.String())
	t = strings.Trim(t, " ") // 3. both ends trimmed
	if r := []rune(t); len(r) > max {
		t = string(r[:max]) // 4. cut, then trimmed again
	}
	return strings.Trim(t, " ")
}

func kindIn(kind string, kinds ...string) bool {
	for _, k := range kinds {
		if k == kind {
			return true
		}
	}
	return false
}

// integerIn reads an MSFP integer (core section 2.3) in [lo, hi]; any
// other value counts as absent.
func integerIn(v any, lo, hi int64) *int64 {
	n, ok := v.(json.Number)
	if !ok || checkInteger(n.String()) != nil {
		return nil
	}
	i, err := strconv.ParseInt(n.String(), 10, 64)
	if err != nil || i < lo || i > hi {
		return nil
	}
	return &i
}

// musicDecode reads one JSON value, numbers kept as written.
func musicDecode(raw []byte) (any, error) {
	d := json.NewDecoder(bytes.NewReader(raw))
	d.UseNumber()
	var v any
	if err := d.Decode(&v); err != nil {
		return nil, err
	}
	if d.More() {
		return nil, errors.New("text after the value")
	}
	return v, nil
}

// musicEncode writes what is kept: no HTML escaping, no final line feed.
func musicEncode(v any) []byte {
	var buf bytes.Buffer
	e := json.NewEncoder(&buf)
	e.SetEscapeHTML(false)
	if err := e.Encode(v); err != nil {
		panic(err)
	}
	return bytes.TrimSuffix(buf.Bytes(), []byte("\n"))
}

// musicDate: YYYY, YYYY-MM or YYYY-MM-DD, year 1000 to 9999, a month
// 01 to 12, a day that exists (music section 3.2). It returns the year.
func musicDate(s string) (int, bool) {
	m := musicDateForm.FindStringSubmatch(s)
	if m == nil {
		return 0, false
	}
	y, _ := strconv.Atoi(m[1])
	if y < 1000 {
		return 0, false
	}
	if m[2] != "" {
		mo, _ := strconv.Atoi(m[2])
		if mo < 1 || mo > 12 {
			return 0, false
		}
		if m[3] != "" {
			d, _ := strconv.Atoi(m[3])
			last := time.Date(y, time.Month(mo)+1, 0, 0, 0, 0, 0, time.UTC).Day()
			if d < 1 || d > last {
				return 0, false
			}
		}
	}
	return y, true
}

// musicDay: a valid date YYYY-MM-DD (a copy's added, as for video).
func musicDay(s string) bool {
	if !musicDayForm.MatchString(s) {
		return false
	}
	_, err := time.Parse("2006-01-02", s)
	return err == nil
}

// musicTitle is what a caller keeps of a music title, in the order the
// vectors write it.
type musicTitle struct {
	Key            string            `json:"key"`
	Kind           string            `json:"kind"`
	Library        string            `json:"library"`
	Name           string            `json:"name"`
	SortName       string            `json:"sortName,omitempty"`
	Artists        []string          `json:"artists,omitempty"`
	ArtistNames    string            `json:"artistNames,omitempty"`
	PrimaryType    string            `json:"primaryType,omitempty"`
	SecondaryTypes []string          `json:"secondaryTypes,omitempty"`
	Compilation    *bool             `json:"compilation,omitempty"`
	Date           string            `json:"date,omitempty"`
	OriginalDate   string            `json:"originalDate,omitempty"`
	Genres         []string          `json:"genres,omitempty"`
	Album          string            `json:"album,omitempty"`
	Disc           *int64            `json:"disc,omitempty"`
	Number         *int64            `json:"number,omitempty"`
	Copies         []musicCopy       `json:"copies,omitempty"`
	Image          *bool             `json:"image,omitempty"`
	IDs            map[string]string `json:"ids,omitempty"`
}

// musicCopy is what a caller keeps of a track's copy (music section 3.3).
type musicCopy struct {
	ID          string           `json:"id"`
	Name        string           `json:"name,omitempty"`
	DurationMs  int64            `json:"durationMs"`
	Container   string           `json:"container"`
	Codec       string           `json:"codec"`
	Codecs      string           `json:"codecs,omitempty"`
	Size        int64            `json:"size"`
	Etag        string           `json:"etag"`
	Added       string           `json:"added,omitempty"`
	BitrateKbps *int64           `json:"bitrateKbps,omitempty"`
	SampleRate  *int64           `json:"sampleRate,omitempty"`
	BitDepth    *int64           `json:"bitDepth,omitempty"`
	Channels    *int64           `json:"channels,omitempty"`
	ReplayGain  *musicReplayGain `json:"replayGain,omitempty"`
	Lyrics      string           `json:"lyrics,omitempty"`
}

type musicReplayGain struct {
	TrackGain *float64 `json:"trackGain,omitempty"`
	TrackPeak *float64 `json:"trackPeak,omitempty"`
	AlbumGain *float64 `json:"albumGain,omitempty"`
	AlbumPeak *float64 `json:"albumPeak,omitempty"`
}

// musicIDKinds: the kind each scheme of ids is for (music section 3.1).
var musicIDKinds = map[string]string{
	"mbreleasegroup": "album",
	"mbrecording":    "track",
	"isrc":           "track",
	"acoustid":       "track",
}

// keepMusicTitle applies catalog section 3.3 and music section 3.2 to one
// entry of titles, given the libraries the caller asked for. It returns
// what the caller keeps, written as JSON, or why the title is skipped.
func keepMusicTitle(raw []byte, libraries []string) ([]byte, error) {
	v, err := musicDecode(raw)
	if err != nil {
		return nil, err
	}
	o, ok := v.(map[string]any)
	if !ok {
		return nil, errors.New("not an object")
	}
	text := func(k string) (string, bool) { s, ok := o[k].(string); return s, ok }
	// A title carries no kind (catalog section 3): the key names it, and
	// a kind member is unknown, ignored.
	key, ok1 := text("key")
	lib, ok3 := text("library")
	if !ok1 || !ok3 {
		return nil, errors.New("key or library isn't a string")
	}
	kind, scheme, err := musicParseKey(key)
	if err != nil {
		return nil, fmt.Errorf("key: %v", err)
	}
	if !kindIn(lib, libraries...) {
		return nil, fmt.Errorf("library %q wasn't asked for", lib)
	}
	t := musicTitle{Key: key, Kind: kind, Library: lib}
	name, _ := text("name")
	if t.Name = cleanName(name, 256); t.Name == "" {
		return nil, errors.New("no name, once cleaned")
	}
	if kind == "album" || kind == "track" {
		if a, ok := o["artists"].([]any); ok {
			seen := map[string]bool{}
			for _, e := range a {
				s, ok := e.(string)
				if !ok || seen[s] || len(t.Artists) == 16 || !musicKeyOf(s, "artist") {
					continue
				}
				seen[s] = true
				t.Artists = append(t.Artists, s)
			}
		}
		an, _ := text("artistNames")
		t.ArtistNames = cleanName(an, 512)
	}
	if kind == "album" {
		sn, _ := text("sortName")
		t.SortName = cleanName(sn, 256)
		if pt, ok := text("primaryType"); ok && musicType(pt) {
			t.PrimaryType = pt
		}
		if st, ok := o["secondaryTypes"].([]any); ok {
			for _, e := range st {
				s, ok := e.(string)
				if !ok || len(t.SecondaryTypes) == 8 || !musicType(s) || kindIn(s, t.SecondaryTypes...) {
					continue
				}
				t.SecondaryTypes = append(t.SecondaryTypes, s)
			}
		}
		if c, ok := o["compilation"].(bool); ok {
			t.Compilation = &c
		}
	}
	if kind == "album" || kind == "track" {
		dateYear := 0
		if d, ok := text("date"); ok {
			if y, ok := musicDate(d); ok {
				t.Date, dateYear = d, y
			}
		}
		if d, ok := text("originalDate"); ok {
			if y, ok := musicDate(d); ok && (dateYear == 0 || y <= dateYear) {
				t.OriginalDate = d
			}
		}
		if g, ok := o["genres"].([]any); ok {
			for _, e := range g {
				s, ok := e.(string)
				if !ok || len(t.Genres) == 8 {
					continue
				}
				c := cleanName(s, math.MaxInt)
				if n := utf8.RuneCountInString(c); n < 1 || n > 64 {
					continue
				}
				repeat := false
				for _, k := range t.Genres {
					repeat = repeat || strings.EqualFold(k, c)
				}
				if !repeat {
					t.Genres = append(t.Genres, c)
				}
			}
		}
	}
	if kind == "track" {
		if a, ok := text("album"); ok && musicKeyOf(a, "album") {
			t.Album = a
		}
		if scheme == "mb" && !strings.HasPrefix(t.Album, "album/mb/") {
			return nil, errors.New("a track/mb track whose album isn't an album/mb key")
		}
		t.Disc = integerIn(o["disc"], 1, 999)
		t.Number = integerIn(o["number"], 1, 999)
		c, ok := o["copies"].([]any)
		if !ok {
			return nil, errors.New("copies isn't an array")
		}
		if t.Copies = keepMusicCopies(c); len(t.Copies) == 0 {
			return nil, errors.New("no copy left")
		}
	}
	if scheme == "local" && kind != "track" {
		if b, ok := o["image"].(bool); ok {
			t.Image = &b
		}
	}
	if ids, ok := o["ids"].(map[string]any); ok {
		for s, val := range ids {
			v, ok := val.(string)
			if !ok || musicIDKinds[s] != kind {
				continue
			}
			if (s == "isrc" && isrcForm.MatchString(v)) || (s != "isrc" && checkMBID(v) == nil) {
				if t.IDs == nil {
					t.IDs = map[string]string{}
				}
				t.IDs[s] = v
			}
		}
	}
	return musicEncode(t), nil
}

// keepMusicCopies is music section 3.3's "what the caller keeps", copy
// by copy: the first 8 kept, a repeat of a kept copy's id skipped.
func keepMusicCopies(list []any) []musicCopy {
	var out []musicCopy
	seen := map[string]bool{}
	for _, e := range list {
		if len(out) == 8 {
			break
		}
		o, ok := e.(map[string]any)
		if !ok {
			continue
		}
		id, _ := o["id"].(string)
		container, okC := o["container"].(string)
		codec, okK := o["codec"].(string)
		etag, _ := o["etag"].(string)
		dur := integerIn(o["durationMs"], 1, 86_400_000)
		size := integerIn(o["size"], 1, 999_999_999_999_999)
		// container and codec are open (core section 8.5): an unknown
		// token is kept; a value that isn't a token is dropped, and the
		// copy, missing a required member, skipped.
		if !musicIDForm.MatchString(id) || seen[id] || dur == nil || !okC || !okK || !openToken(container) || !openToken(codec) || size == nil || !musicIDForm.MatchString(etag) {
			continue
		}
		seen[id] = true
		c := musicCopy{ID: id, DurationMs: *dur, Container: container, Codec: codec, Size: *size, Etag: etag}
		if cs, ok := o["codecs"].(string); ok && musicCodecsForm.MatchString(cs) {
			c.Codecs = cs
		}
		if n, ok := o["name"].(string); ok {
			c.Name = cleanName(n, 64)
		}
		if a, ok := o["added"].(string); ok && musicDay(a) {
			c.Added = a
		}
		c.BitrateKbps = integerIn(o["bitrateKbps"], 1, 100_000) // exact
		c.SampleRate = integerIn(o["sampleRate"], 8_000, 768_000)
		c.BitDepth = integerIn(o["bitDepth"], 8, 32)
		c.Channels = integerIn(o["channels"], 1, 64)
		if rg, ok := o["replayGain"].(map[string]any); ok {
			g := musicReplayGain{
				TrackGain: musicNumberIn(rg["trackGain"], -64, 64, 2),
				TrackPeak: musicNumberIn(rg["trackPeak"], 0, 10, 6),
				AlbumGain: musicNumberIn(rg["albumGain"], -64, 64, 2),
				AlbumPeak: musicNumberIn(rg["albumPeak"], 0, 10, 6),
			}
			if g != (musicReplayGain{}) {
				c.ReplayGain = &g
			}
		}
		if l, ok := o["lyrics"].(string); ok && (l == "plain" || l == "synced") {
			c.Lyrics = l
		}
		out = append(out, c)
	}
	return out
}

// musicNumberIn reads a JSON number in [lo, hi], rounded to places
// decimals (to the nearest; the vectors hold no exact ties); any other
// value counts as absent.
func musicNumberIn(v any, lo, hi float64, places int) *float64 {
	n, ok := v.(json.Number)
	if !ok {
		return nil
	}
	f, err := strconv.ParseFloat(n.String(), 64)
	if err != nil || f < lo || f > hi {
		return nil
	}
	r, _ := strconv.ParseFloat(strconv.FormatFloat(f, 'f', places, 64), 64)
	if r == 0 {
		r = 0 // never -0
	}
	return &r
}

// --- The queue (music section 5.2) ---

// musicCheckQueueRequest is what the owning server checks of a play
// request's title and queue: nil means it accepts them.
func musicCheckQueueRequest(raw []byte) error {
	v, err := musicDecode(raw)
	if err != nil {
		return err
	}
	o, ok := v.(map[string]any)
	if !ok {
		return errors.New("400: not an object")
	}
	title, ok := o["title"].(string)
	if !ok || !musicKeyOf(title, "track") {
		return errors.New("400: title isn't a track key")
	}
	q, present := o["queue"]
	if !present {
		return nil
	}
	list, ok := q.([]any)
	if !ok {
		return errors.New("400: queue isn't an array")
	}
	if len(list) > 5 {
		return fmt.Errorf("400: a queue of %d entries, more than 5", len(list))
	}
	seen := map[string]bool{}
	for i, e := range list {
		s, ok := e.(string)
		switch {
		case !ok:
			return fmt.Errorf("400: queue entry %d isn't a string", i)
		case !musicKeyOf(s, "track"):
			return fmt.Errorf("400: queue entry %d isn't a track key in canonical form", i)
		case s == title:
			return fmt.Errorf("400: queue entry %d is title itself", i)
		case seen[s]:
			return fmt.Errorf("400: queue entry %d is a repeat", i)
		}
		seen[s] = true
	}
	return nil
}

type musicQueueEntry struct {
	Title      string           `json:"title"`
	VersionID  string           `json:"versionId"`
	Method     string           `json:"method"`
	Stream     musicQueueStream `json:"stream"`
	DurationMs *int64           `json:"durationMs,omitempty"`
}

type musicQueueStream struct {
	URL      string `json:"url"`
	Protocol string `json:"protocol"`
	MimeType string `json:"mimeType,omitempty"`
}

// musicStreamCheck checks an entry's stream as media section 1 says:
// <stream root><ticket>/queue/<n>/<name>, under the playback's ticket, n
// the entry's place in the request's queue (0 to 4, decimal, no leading
// zeros), hls as master.m3u8 (its mimeType, when sent,
// application/vnd.apple.mpegurl) or file as file; a remux or a transcode
// served as hls, a delivery file as file.
func musicStreamCheck(s musicQueueStream, mimeSet bool, root, ticket string, n int, method, delivery string) error {
	if !strings.HasPrefix(s.URL, root) {
		return errors.New("the url isn't under the stream root")
	}
	if strings.ContainsAny(s.URL, "%?#") {
		return errors.New("a %, a query or a fragment in the url")
	}
	p := strings.Split(strings.TrimPrefix(s.URL, root), "/")
	if len(p) != 4 || p[1] != "queue" || !musicMediaName.MatchString(p[3]) || p[3] == "." || p[3] == ".." {
		return errors.New("the url isn't <stream root><ticket>/queue/<n>/<name>")
	}
	if p[0] != ticket {
		return errors.New("the url names another ticket than the playback's")
	}
	if p[2] != strconv.Itoa(n) {
		return fmt.Errorf("the url's <n> isn't %d, the entry's place in the request's queue", n)
	}
	switch s.Protocol {
	case "hls":
		if p[3] != "master.m3u8" {
			return errors.New("an hls stream not named master.m3u8")
		}
		if mimeSet && s.MimeType != "application/vnd.apple.mpegurl" {
			return errors.New("an hls stream of another mimeType")
		}
		if delivery == "file" {
			return errors.New("delivery file served as hls")
		}
	case "file":
		if p[3] != "file" {
			return errors.New("a file stream not named file")
		}
		if method != "direct" {
			return errors.New("a remux or a transcode served as file")
		}
	default:
		return fmt.Errorf("protocol %q is neither hls nor file", s.Protocol)
	}
	return nil
}

// musicQueueAsked is the request's queue, once the request is accepted.
func musicQueueAsked(request []byte) ([]string, string, error) {
	if err := musicCheckQueueRequest(request); err != nil {
		return nil, "", fmt.Errorf("the request: %v", err)
	}
	v, _ := musicDecode(request)
	o := v.(map[string]any)
	var asked []string
	if q, ok := o["queue"].([]any); ok {
		for _, e := range q {
			asked = append(asked, e.(string))
		}
	}
	delivery, _ := o["delivery"].(string)
	return asked, delivery, nil
}

// musicPlaybackTicket is the ticket of a play answer's own stream URL,
// <stream root><ticket>/<name>, or "" when it has none of that form.
func musicPlaybackTicket(a map[string]any, root string) string {
	so, _ := a["stream"].(map[string]any)
	url, _ := so["url"].(string)
	rest, ok := strings.CutPrefix(url, root)
	tk, name, cut := strings.Cut(rest, "/")
	if !ok || !cut || !musicIDForm.MatchString(tk) || (name != "file" && name != "master.m3u8") {
		return ""
	}
	return tk
}

// musicKeepQueueAnswer is what the viewer's server keeps of a play
// answer's queue (an array, possibly empty), given its request and the
// owning server's stream root.
func musicKeepQueueAnswer(request, answer []byte, root string) ([]byte, error) {
	asked, delivery, err := musicQueueAsked(request)
	if err != nil {
		return nil, err
	}
	av, err := musicDecode(answer)
	if err != nil {
		return nil, err
	}
	a, ok := av.(map[string]any)
	if !ok {
		return nil, errors.New("the answer isn't an object")
	}
	want := map[string]string{"file": "direct", "remux": "remux", "transcode": "transcode"}[delivery]
	ticket := musicPlaybackTicket(a, root) // "" when the answer's own stream names none: every entry dropped
	kept := []musicQueueEntry{}
	list, _ := a["queue"].([]any) // not an array: absent
	next := 0                     // the first place of the request's queue an entry may take
	for _, e := range list {
		if len(kept) == 5 {
			break
		}
		o, ok := e.(map[string]any)
		if !ok {
			continue
		}
		title, _ := o["title"].(string)
		at := -1
		for i := next; i < len(asked) && at < 0; i++ {
			if asked[i] == title {
				at = i
			}
		}
		vid, _ := o["versionId"].(string)
		method, _ := o["method"].(string)
		so, okS := o["stream"].(map[string]any)
		if at < 0 || !musicIDForm.MatchString(vid) || !kindIn(method, musicMethods...) || (want != "" && method != want) || !okS {
			continue
		}
		url, ok1 := so["url"].(string)
		proto, ok2 := so["protocol"].(string)
		mime, okM := so["mimeType"].(string)
		_, mimeSet := so["mimeType"]
		if !ok1 || !ok2 || (mimeSet && !okM) {
			continue
		}
		s := musicQueueStream{URL: url, Protocol: proto, MimeType: mime}
		if ticket == "" || musicStreamCheck(s, mimeSet, root, ticket, at, method, delivery) != nil {
			continue
		}
		k := musicQueueEntry{Title: title, VersionID: vid, Method: method, Stream: s}
		if d, ok := o["durationMs"].(json.Number); ok && checkInteger(d.String()) == nil {
			if k.DurationMs = integerIn(d, 0, 86_400_000); k.DurationMs == nil {
				zero := int64(0)
				k.DurationMs = &zero
			}
		}
		kept = append(kept, k)
		next = at + 1
	}
	return musicEncode(kept), nil
}

// --- Lyrics (music section 6) ---

// musicCheckLyricsRequest: nil means the owning server reads the
// request; otherwise it answers 400 invalid-request.
func musicCheckLyricsRequest(raw []byte) error {
	v, err := musicDecode(raw)
	if err != nil {
		return err
	}
	o, ok := v.(map[string]any)
	if !ok {
		return errors.New("400: not an object")
	}
	title, ok := o["title"].(string)
	if !ok || !musicKeyOf(title, "track") {
		return errors.New("400: title isn't a track key")
	}
	vid, ok := o["versionId"].(string)
	if !ok || !musicIDForm.MatchString(vid) {
		return errors.New("400: versionId isn't a version ID")
	}
	return nil
}

type musicLyrics struct {
	StructuredLyrics []musicLyricsSet `json:"structuredLyrics"`
}

type musicLyricsSet struct {
	Lang          string           `json:"lang,omitempty"`
	Kind          string           `json:"kind"`
	Synced        bool             `json:"synced"`
	Offset        int64            `json:"offset"`
	DisplayArtist string           `json:"displayArtist,omitempty"`
	DisplayTitle  string           `json:"displayTitle,omitempty"`
	Line          []musicLyricLine `json:"line"`
}

type musicLyricLine struct {
	Start *int64 `json:"start,omitempty"`
	Value string `json:"value"`
}

// musicKeepLyrics is what the viewer's server keeps of a lyrics answer,
// or why the fetch fails.
func musicKeepLyrics(raw []byte) ([]byte, error) {
	if len(raw) > 1<<20 {
		return nil, errors.New("an answer past 1 MiB")
	}
	v, err := musicDecode(raw)
	if err != nil {
		return nil, err
	}
	o, ok := v.(map[string]any)
	if !ok {
		return nil, errors.New("the answer isn't an object")
	}
	sets, ok := o["structuredLyrics"].([]any)
	if !ok {
		return nil, errors.New("structuredLyrics isn't an array")
	}
	out := musicLyrics{StructuredLyrics: []musicLyricsSet{}}
	for _, e := range sets {
		if len(out.StructuredLyrics) == 8 {
			break
		}
		so, ok := e.(map[string]any)
		if !ok {
			continue
		}
		synced, ok := so["synced"].(bool)
		if !ok {
			continue
		}
		s := musicLyricsSet{Kind: "main", Synced: synced}
		if l, ok := so["lang"].(string); ok && validTag(l) {
			s.Lang = l
		}
		if k, ok := so["kind"].(string); ok && openToken(k) { // open (core section 8.5)
			s.Kind = k
		}
		if off := integerIn(so["offset"], -600_000, 600_000); off != nil {
			s.Offset = *off
		}
		da, _ := so["displayArtist"].(string)
		s.DisplayArtist = cleanName(da, 512)
		dt, _ := so["displayTitle"].(string)
		s.DisplayTitle = cleanName(dt, 256)
		lines, _ := so["line"].([]any)
		var last int64
		for _, le := range lines {
			if len(s.Line) == 2000 {
				break
			}
			lo, ok := le.(map[string]any)
			if !ok {
				continue
			}
			val, ok := lo["value"].(string)
			if !ok {
				continue
			}
			l := musicLyricLine{Value: cleanName(val, 512)}
			if synced {
				st := integerIn(lo["start"], 0, 86_400_000)
				if st == nil || *st < last {
					continue
				}
				last, l.Start = *st, st
			}
			s.Line = append(s.Line, l)
		}
		if len(s.Line) == 0 {
			continue
		}
		out.StructuredLyrics = append(out.StructuredLyrics, s)
	}
	if len(out.StructuredLyrics) == 0 {
		return nil, errors.New("no set left")
	}
	return musicEncode(out), nil
}
