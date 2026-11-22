package main

// The video part's rules (spec/v0/video.md, as revised by proposals 16
// and 17):
// title keys of the tmdb, tvdb and local schemes and extras' keys
// (section 4), what a caller keeps of a video title (section 3.1), of
// its copies (sections 3.2 to 3.8), of an extra (section 3.9) and of a
// local title's description (section 3.10), what a viewer's server reads
// from a codecs string (section 3.5), the video members of play's answer
// and the URLs built from its ticket (sections 5.3 and 6.1), the previews
// call (section 5.6), and the fonts of a copy (section 6.6).
//
// Every helper here has a video prefix (or is a method of a video type),
// so that this file depends on no other part's file.

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"unicode"
)

var (
	videoIDForm      = regexp.MustCompile(`^[1-9][0-9]{0,9}$`)      // a TMDB or TheTVDB ID, a TMDB episode ID
	videoOpaqueForm  = regexp.MustCompile(`^[A-Za-z0-9_-]{1,64}$`)  // local identifiers, extras, copy IDs, etags, tickets
	videoSeasonForm  = regexp.MustCompile(`^s(0|[1-9][0-9]{0,3})$`) // s<season>
	videoEpisodeForm = regexp.MustCompile(`^e(0|[1-9][0-9]{0,4})$`) // e<episode>
	videoAbsForm     = regexp.MustCompile(`^a(0|[1-9][0-9]{0,4})$`) // a<number>
	videoEpIDForm    = regexp.MustCompile(`^e[1-9][0-9]{0,9}$`)     // e<episode id>
	videoAudioID     = regexp.MustCompile(`^a(0|[1-9][0-9]{0,2})$`) // a<n>, n 0 to 999
	videoSubtitleID  = regexp.MustCompile(`^s(0|[1-9][0-9]{0,2})$`) // s<n>, n 0 to 999
	videoIMDbForm    = regexp.MustCompile(`^tt[0-9]{7,10}$`)        // ids.imdb
	videoCodecsForm  = regexp.MustCompile(`^[A-Za-z0-9.+-]{1,64}$`)  // an RFC 6381 codecs string
	videoFPSForm     = regexp.MustCompile(`^([1-9][0-9]{0,6})/([1-9][0-9]{0,6})$`)
	videoAspectForm  = regexp.MustCompile(`^([1-9][0-9]{0,4}):([1-9][0-9]{0,4})$`)
	videoIntegerForm = regexp.MustCompile(`^-?(0|[1-9][0-9]*)$`)
)

func videoIn(s string, list []string) bool {
	for _, v := range list {
		if v == s {
			return true
		}
	}
	return false
}

// videoKeyInfo is what a video title key says (section 4).
type videoKeyInfo struct {
	Kind   string // movie, series, season, episode or extra
	Scheme string // tmdb, tvdb or local
	Series string // the film's or series' own key: the first three parts
	Parent string // for an extra, the key of the title it belongs to
	ByID   bool   // an episode keyed by its TMDB episode ID
}

// parseVideoKey reads a video title key in its canonical form.
func parseVideoKey(key string) (videoKeyInfo, error) {
	p := strings.Split(key, "/")
	if len(p) < 3 {
		return videoKeyInfo{}, errors.New("fewer than three parts joined by /")
	}
	if p[0] != "movie" && p[0] != "series" {
		return videoKeyInfo{}, fmt.Errorf("kind %q is neither movie nor series", p[0])
	}
	k := videoKeyInfo{Scheme: p[1], Series: strings.Join(p[:3], "/")}
	switch p[1] {
	case "tmdb", "local":
	case "tvdb":
		if p[0] == "movie" {
			return k, errors.New("films have no tvdb keys")
		}
	default:
		return k, fmt.Errorf("scheme %q isn't tmdb, tvdb or local", p[1])
	}
	if p[1] == "local" {
		if !videoOpaqueForm.MatchString(p[2]) {
			return k, errors.New("a local identifier outside 1 to 64 characters of A-Z a-z 0-9 _ -")
		}
	} else if !videoIDForm.MatchString(p[2]) {
		return k, fmt.Errorf("a %s ID that isn't 1 to 9,999,999,999 without leading zeros", p[1])
	}
	rest := p[3:]
	// An extra: the key of a film, a series or a season, then x/<opaque>.
	if n := len(rest); n >= 2 && rest[n-2] == "x" {
		if !videoOpaqueForm.MatchString(rest[n-1]) {
			return k, errors.New("an extra's part outside 1 to 64 characters of A-Z a-z 0-9 _ -")
		}
		parent := strings.Join(p[:len(p)-2], "/")
		pk, err := parseVideoKey(parent)
		if err != nil {
			return k, fmt.Errorf("an extra under a bad key: %v", err)
		}
		if pk.Kind != "movie" && pk.Kind != "series" && pk.Kind != "season" {
			return k, fmt.Errorf("an extra under an %s: only films, series and seasons have extras", pk.Kind)
		}
		k.Kind, k.Parent = "extra", parent
		return k, nil
	}
	if p[0] == "movie" {
		if len(rest) != 0 {
			return k, errors.New("a film key with more parts")
		}
		k.Kind = "movie"
		return k, nil
	}
	switch {
	case len(rest) == 0:
		k.Kind = "series"
	case len(rest) == 1 && videoSeasonForm.MatchString(rest[0]):
		k.Kind = "season"
	case len(rest) == 2 && videoSeasonForm.MatchString(rest[0]) && videoEpisodeForm.MatchString(rest[1]):
		k.Kind = "episode"
	case len(rest) == 1 && p[1] == "tmdb" && videoEpIDForm.MatchString(rest[0]):
		k.Kind, k.ByID = "episode", true
	case len(rest) == 1 && p[1] == "tvdb" && videoAbsForm.MatchString(rest[0]):
		k.Kind = "episode"
	default:
		return k, errors.New("not one of the forms of section 4 for its scheme")
	}
	return k, nil
}

// checkVideoKey returns the kind a video title key names.
func checkVideoKey(key string) (string, error) {
	k, err := parseVideoKey(key)
	if err != nil {
		return "", err
	}
	return k.Kind, nil
}

// videoInt reads an MSFP integer (core section 2.3) in [lo, hi]; any
// other value is nil.
func videoInt(v any, lo, hi int64) *int64 {
	n, ok := v.(json.Number)
	if !ok || !videoIntegerForm.MatchString(n.String()) || n.String() == "-0" {
		return nil
	}
	i, err := strconv.ParseInt(n.String(), 10, 64)
	if err != nil || i > 1<<53-1 || i < -(1<<53-1) || i < lo || i > hi {
		return nil
	}
	return &i
}

func videoBool(v any) *bool {
	if b, ok := v.(bool); ok {
		return &b
	}
	return nil
}

// videoPlain is core section 9.1's cleaning of a name, with max code
// points instead of 80: "" when nothing is left, or v isn't a string.
func videoPlain(v any, max int) string {
	s, ok := v.(string)
	if !ok {
		return ""
	}
	var b strings.Builder
	space := false
	for _, r := range s {
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
	t := strings.Map(func(r rune) rune {
		if unicode.Is(unicode.Cc, r) || unicode.Is(unicode.Cf, r) {
			return -1
		}
		return r
	}, b.String())
	t = strings.Trim(t, " ")
	if r := []rune(t); len(r) > max {
		t = string(r[:max])
	}
	return strings.Trim(t, " ")
}

// videoTag is v when it is a valid tag (catalog section 3), else "".
func videoTag(v any) string {
	if s, ok := v.(string); ok && validTag(s) {
		return s
	}
	return ""
}

// videoFPS is a frame rate in its rational form "<num>/<den>", each 1 to
// 1,000,000 and the fraction reduced, else "".
func videoFPS(v any) string {
	s, ok := v.(string)
	m := videoFPSForm.FindStringSubmatch(s)
	if !ok || m == nil {
		return ""
	}
	n, _ := strconv.ParseInt(m[1], 10, 64)
	d, _ := strconv.ParseInt(m[2], 10, 64)
	if n > 1_000_000 || d > 1_000_000 || videoGCD(n, d) != 1 {
		return ""
	}
	return s
}

func videoGCD(a, b int64) int64 {
	for b != 0 {
		a, b = b, a%b
	}
	return a
}

// videoAspect is a sample aspect ratio "<w>:<h>", each 1 to 65,535, else "".
func videoAspect(v any) string {
	s, ok := v.(string)
	m := videoAspectForm.FindStringSubmatch(s)
	if !ok || m == nil {
		return ""
	}
	w, _ := strconv.ParseInt(m[1], 10, 64)
	h, _ := strconv.ParseInt(m[2], 10, 64)
	if w > 65535 || h > 65535 {
		return ""
	}
	return s
}

// videoForm is v when it is a string matching re, else "".
func videoForm(v any, re *regexp.Regexp) string {
	if s, ok := v.(string); ok && re.MatchString(s) {
		return s
	}
	return ""
}

// What the caller keeps, in the order the vectors write it.

type videoDV struct {
	Profile       int64 `json:"profile"`
	Level         int64 `json:"level"`
	Compatibility int64 `json:"compatibility"`
	EL            *bool `json:"el,omitempty"`
}

type videoColour struct {
	Primaries *int64 `json:"primaries,omitempty"`
	Transfer  *int64 `json:"transfer,omitempty"`
	Matrix    *int64 `json:"matrix,omitempty"`
	FullRange *bool  `json:"fullRange,omitempty"`
}

type videoStream struct {
	Codec        string       `json:"codec"`
	Codecs       string       `json:"codecs,omitempty"`
	Width        int64        `json:"width"`
	Height       int64        `json:"height"`
	SampleAspect string       `json:"sampleAspect,omitempty"`
	Rotation     *int64       `json:"rotation,omitempty"`
	BitDepth     *int64       `json:"bitDepth,omitempty"`
	Colour       *videoColour `json:"colour,omitempty"`
	HDR          []string     `json:"hdr,omitempty"`
	DV           *videoDV     `json:"dv,omitempty"`
	FPS          string       `json:"fps,omitempty"`
	RefFrames    *int64       `json:"refFrames,omitempty"`
	Interlaced   *bool        `json:"interlaced,omitempty"`
}

type videoAudio struct {
	ID         string `json:"id"`
	Language   string `json:"language,omitempty"`
	Channels   *int64 `json:"channels,omitempty"`
	Role       string `json:"role,omitempty"`
	Default    *bool  `json:"default,omitempty"`
	Forced     *bool  `json:"forced,omitempty"`
	Codec      string `json:"codec"`
	Codecs     string `json:"codecs,omitempty"`
	Objects    string `json:"objects,omitempty"`
	SampleRate *int64 `json:"sampleRate,omitempty"`
	BitDepth   *int64 `json:"bitDepth,omitempty"`
	Original   *bool  `json:"original,omitempty"`
}

type videoSubtitle struct {
	ID              string `json:"id"`
	Language        string `json:"language,omitempty"`
	Role            string `json:"role,omitempty"`
	Default         *bool  `json:"default,omitempty"`
	Forced          *bool  `json:"forced,omitempty"`
	HearingImpaired *bool  `json:"hearingImpaired,omitempty"`
	Format          string `json:"format,omitempty"`
	External        *bool  `json:"external,omitempty"`
}

type videoChapter struct {
	StartMs int64  `json:"startMs"`
	Name    string `json:"name,omitempty"`
}

type videoSegment struct {
	Kind    string `json:"kind"`
	StartMs int64  `json:"startMs"`
	EndMs   int64  `json:"endMs"`
}

// videoLayout is a copy's previewLayout (section 5.5).
type videoLayout struct {
	TileWidth  int64 `json:"tileWidth"`
	TileHeight int64 `json:"tileHeight"`
	Columns    int64 `json:"columns"`
	Rows       int64 `json:"rows"`
	IntervalMs int64 `json:"intervalMs"`
	TileCount  int64 `json:"tileCount"`
}

// sheets is the number of sheets the layout gives.
func (l videoLayout) sheets() int64 {
	per := l.Columns * l.Rows
	return (l.TileCount + per - 1) / per
}

type videoCopy struct {
	ID             string          `json:"id"`
	Edition        string          `json:"edition,omitempty"`
	DurationMs     int64           `json:"durationMs"`
	StartMs        *int64          `json:"startMs,omitempty"`
	EndMs          *int64          `json:"endMs,omitempty"`
	MediaType      string          `json:"mediaType"`
	Size           int64           `json:"size"`
	ETag           string          `json:"etag"`
	Parts          *int64          `json:"parts,omitempty"`
	Video          *videoStream    `json:"video,omitempty"`
	AudioTracks    []videoAudio    `json:"audioTracks"`
	SubtitleTracks []videoSubtitle `json:"subtitleTracks,omitempty"`
	Chapters       []videoChapter  `json:"chapters,omitempty"`
	Segments       []videoSegment  `json:"segments,omitempty"`
	PreviewLayout  *videoLayout    `json:"previewLayout,omitempty"`
	ThreeD         string          `json:"threeD,omitempty"`
}

type videoPlace struct {
	Season  int64 `json:"season"`
	Episode int64 `json:"episode"`
}

// videoTitle is a kept title. It has no kind: the key names it.
type videoTitle struct {
	Key       string            `json:"key"`
	Library   string            `json:"library"`
	ExtraType string            `json:"extraType,omitempty"`
	Name      string            `json:"name,omitempty"`
	Year      *int64            `json:"year,omitempty"`
	Image     *bool             `json:"image,omitempty"`
	Place     *videoPlace       `json:"place,omitempty"`
	With      []string          `json:"with,omitempty"`
	IDs       map[string]string `json:"ids,omitempty"`
	Copies    []videoCopy       `json:"copies,omitempty"`
}

// The bounds of section 3.8.
const (
	videoMaxDuration = 8_640_000_000
	videoMaxSize     = 999_999_999_999_999
	videoMaxCopies   = 16
	videoMaxHDR      = 8
	videoMaxFonts    = 64
	videoMaxFont     = 10 << 20 // 10 MiB
	videoMaxFontSet  = 64 << 20 // 64 MiB, a copy's fonts in all
)

// videoH273 is an H.273 code point, 0 to 255, 2 ("unspecified") counting
// as absent (section 3.6).
func videoH273(v any) *int64 {
	if n := videoInt(v, 0, 255); n != nil && *n != 2 {
		return n
	}
	return nil
}

// keepVideoLayout is section 5.5's reading of a previewLayout: nil when it
// is dropped (not an object, a member missing, of another type or out of
// bounds).
func keepVideoLayout(v any) *videoLayout {
	o, ok := v.(map[string]any)
	if !ok {
		return nil
	}
	w, h := videoInt(o["tileWidth"], 1, 2048), videoInt(o["tileHeight"], 1, 2048)
	c, r := videoInt(o["columns"], 1, 100), videoInt(o["rows"], 1, 100)
	iv := videoInt(o["intervalMs"], 100, 3_600_000)
	if w == nil || h == nil || c == nil || r == nil || iv == nil || *c**w > 4096 || *r**h > 4096 {
		return nil
	}
	n := videoInt(o["tileCount"], 1, 1000**c**r)
	if n == nil {
		return nil
	}
	return &videoLayout{*w, *h, *c, *r, *iv, *n}
}

// keepVideoStream is section 3.8's reading of a copy's video: nil when
// it counts as absent.
func keepVideoStream(v any) *videoStream {
	o, ok := v.(map[string]any)
	if !ok {
		return nil
	}
	codec := openTokenValue(o["codec"])
	w, h := videoInt(o["width"], 1, 100_000), videoInt(o["height"], 1, 100_000)
	if codec == "" || w == nil || h == nil {
		return nil
	}
	s := &videoStream{Codec: codec, Width: *w, Height: *h}
	s.Codecs = videoForm(o["codecs"], videoCodecsForm)
	s.SampleAspect = videoAspect(o["sampleAspect"])
	if r := videoInt(o["rotation"], 0, 270); r != nil && *r%90 == 0 {
		s.Rotation = r
	}
	s.BitDepth = videoInt(o["bitDepth"], 1, 16)
	if c, ok := o["colour"].(map[string]any); ok {
		col := videoColour{videoH273(c["primaries"]), videoH273(c["transfer"]), videoH273(c["matrix"]), videoBool(c["fullRange"])}
		if col != (videoColour{}) {
			s.Colour = &col
		}
	}
	if list, ok := o["hdr"].([]any); ok {
		var hdr []string
		for _, e := range list {
			if t := openTokenValue(e); t != "" && !videoIn(t, hdr) {
				hdr = append(hdr, t)
			}
		}
		sort.Strings(hdr)
		if len(hdr) > videoMaxHDR {
			hdr = hdr[:videoMaxHDR]
		}
		s.HDR = hdr
	}
	if dv, ok := o["dv"].(map[string]any); ok {
		p, l, c := videoInt(dv["profile"], 0, 127), videoInt(dv["level"], 0, 63), videoInt(dv["compatibility"], 0, 15)
		if p != nil && l != nil && c != nil {
			s.DV = &videoDV{*p, *l, *c, videoBool(dv["el"])}
		}
	}
	s.FPS = videoFPS(o["fps"])
	if codec == "h264" {
		s.RefFrames = videoInt(o["refFrames"], 1, 16)
	}
	s.Interlaced = videoBool(o["interlaced"])
	return s
}

func keepVideoAudio(list []any) []videoAudio {
	out := []videoAudio{}
	seen := map[string]bool{}
	for _, e := range list {
		o, ok := e.(map[string]any)
		if !ok || len(out) == 32 {
			continue
		}
		id := videoForm(o["id"], videoAudioID)
		codec := openTokenValue(o["codec"])
		if id == "" || seen[id] || codec == "" {
			continue
		}
		seen[id] = true
		out = append(out, videoAudio{
			ID:         id,
			Language:   videoTag(o["language"]),
			Channels:   videoInt(o["channels"], 1, 64),
			Role:       openTokenValue(o["role"]),
			Default:    videoBool(o["default"]),
			Forced:     videoBool(o["forced"]),
			Codec:      codec,
			Codecs:     videoForm(o["codecs"], videoCodecsForm),
			Objects:    openTokenValue(o["objects"]),
			SampleRate: videoInt(o["sampleRate"], 8_000, 768_000),
			BitDepth:   videoInt(o["bitDepth"], 8, 32),
			Original:   videoBool(o["original"]),
		})
	}
	return out
}

func keepVideoSubtitles(list []any) []videoSubtitle {
	var out []videoSubtitle
	seen := map[string]bool{}
	for _, e := range list {
		o, ok := e.(map[string]any)
		if !ok || len(out) == 64 {
			continue
		}
		id := videoForm(o["id"], videoSubtitleID)
		if id == "" || seen[id] {
			continue
		}
		seen[id] = true
		out = append(out, videoSubtitle{
			ID:              id,
			Language:        videoTag(o["language"]),
			Role:            openTokenValue(o["role"]),
			Default:         videoBool(o["default"]),
			Forced:          videoBool(o["forced"]),
			HearingImpaired: videoBool(o["hearingImpaired"]),
			Format:          openTokenValue(o["format"]),
			External:        videoBool(o["external"]),
		})
	}
	return out
}

// videoDecode reads one JSON value, numbers kept as their text.
func videoDecode(raw []byte) (any, error) {
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

// videoEncode writes v as compact JSON, without HTML escaping.
func videoEncode(v any) ([]byte, error) {
	var buf bytes.Buffer
	e := json.NewEncoder(&buf)
	e.SetEscapeHTML(false)
	if err := e.Encode(v); err != nil {
		return nil, err
	}
	return bytes.TrimSuffix(buf.Bytes(), []byte("\n")), nil
}

func keepVideoChapters(v any, duration int64) []videoChapter {
	list, _ := v.([]any)
	var out []videoChapter
	for _, e := range list {
		o, ok := e.(map[string]any)
		if !ok || len(out) == 256 {
			continue
		}
		s := videoInt(o["startMs"], 0, duration)
		if s == nil || len(out) > 0 && *s <= out[len(out)-1].StartMs {
			continue
		}
		out = append(out, videoChapter{*s, videoPlain(o["name"], 64)})
	}
	return out
}

func keepVideoSegments(v any, duration int64) []videoSegment {
	list, _ := v.([]any)
	var out []videoSegment
	for _, e := range list {
		o, ok := e.(map[string]any)
		if !ok || len(out) == 32 {
			continue
		}
		kind := openTokenValue(o["kind"])
		s, t := videoInt(o["startMs"], 0, duration), videoInt(o["endMs"], 0, duration)
		if kind == "" || s == nil || t == nil || *s >= *t {
			continue
		}
		out = append(out, videoSegment{kind, *s, *t})
	}
	return out
}

// keepCopy is section 3.8 for one entry of copies: what the caller
// keeps, or why it skips the copy.
func keepCopy(v any) (*videoCopy, error) {
	o, ok := v.(map[string]any)
	if !ok {
		return nil, errors.New("not an object")
	}
	id := videoForm(o["id"], videoOpaqueForm)
	if id == "" {
		return nil, errors.New("id isn't 1 to 64 characters of A-Z a-z 0-9 _ -")
	}
	d := videoInt(o["durationMs"], 0, videoMaxDuration)
	if d == nil {
		return nil, errors.New("durationMs isn't an integer from 0 to 8,640,000,000")
	}
	mt, _ := o["mediaType"].(string)
	if !openMediaType(mt) {
		return nil, errors.New("mediaType missing or not a media type")
	}
	size := videoInt(o["size"], 1, videoMaxSize)
	if size == nil {
		return nil, errors.New("size isn't an integer from 1 to 999,999,999,999,999")
	}
	etag := videoForm(o["etag"], videoOpaqueForm)
	if etag == "" {
		return nil, errors.New("etag isn't 1 to 64 characters of A-Z a-z 0-9 _ -")
	}
	audio, ok := o["audioTracks"].([]any)
	if !ok {
		return nil, errors.New("audioTracks isn't an array")
	}
	c := &videoCopy{ID: id, DurationMs: *d, MediaType: mt, Size: *size, ETag: etag}
	c.Edition = openTokenValue(o["edition"])
	c.StartMs = videoInt(o["startMs"], 0, *d)
	from := int64(0)
	if c.StartMs != nil {
		from = *c.StartMs
	}
	c.EndMs = videoInt(o["endMs"], from+1, *d)
	c.Parts = videoInt(o["parts"], 2, 16)
	c.Video = keepVideoStream(o["video"])
	c.AudioTracks = keepVideoAudio(audio)
	if subs, ok := o["subtitleTracks"].([]any); ok {
		c.SubtitleTracks = keepVideoSubtitles(subs)
	}
	c.Chapters = keepVideoChapters(o["chapters"], *d)
	c.Segments = keepVideoSegments(o["segments"], *d)
	c.PreviewLayout = keepVideoLayout(o["previewLayout"])
	c.ThreeD = openTokenValue(o["threeD"])
	return c, nil
}

// keepVideoCopy applies section 3.8 to one copy given as JSON text.
func keepVideoCopy(raw []byte) ([]byte, error) {
	v, err := videoDecode(raw)
	if err != nil {
		return nil, err
	}
	c, err := keepCopy(v)
	if err != nil {
		return nil, err
	}
	return videoEncode(c)
}

// keepVideoTitle applies catalog section 3.3 and video sections 3.1,
// 3.8, 3.9 and 3.10 to one entry of titles, given the libraries the
// caller asked for. It returns what the caller keeps, written as JSON,
// or why the title is skipped. A kind member is ignored: the key names
// the kind.
func keepVideoTitle(raw []byte, libraries []string) ([]byte, error) {
	v, err := videoDecode(raw)
	if err != nil {
		return nil, err
	}
	o, ok := v.(map[string]any)
	if !ok {
		return nil, errors.New("not an object")
	}
	// A title carries no kind (catalog section 3): the key names it, and
	// a kind member is unknown, ignored.
	key, ok1 := o["key"].(string)
	lib, ok2 := o["library"].(string)
	if !ok1 || !ok2 {
		return nil, errors.New("key or library isn't a string")
	}
	k, err := parseVideoKey(key)
	if err != nil {
		return nil, fmt.Errorf("key: %v", err)
	}
	kind := k.Kind
	if !videoIn(lib, libraries) {
		return nil, fmt.Errorf("library %q wasn't asked for", lib)
	}
	t := videoTitle{Key: key, Library: lib}

	// Extras (section 3.9): no name.
	if kind == "extra" {
		if t.ExtraType = openTokenValue(o["extraType"]); t.ExtraType == "" {
			return nil, errors.New("an extra whose extraType is missing or isn't a token")
		}
	}
	// The description of a local film or series (section 3.10).
	if k.Scheme == "local" && (kind == "movie" || kind == "series") {
		t.Name = videoPlain(o["name"], 128)
		if t.Name == "" {
			return nil, errors.New("a local film or series without a name")
		}
		t.Year = videoInt(o["year"], 1800, 9999)
		t.Image = videoBool(o["image"])
	}
	// place: episodes keyed by their TMDB episode ID (section 4.1).
	if k.ByID {
		p, ok := o["place"].(map[string]any)
		if !ok {
			return nil, errors.New("an episode keyed by its TMDB ID without a place")
		}
		season, episode := videoInt(p["season"], 0, 9999), videoInt(p["episode"], 0, 99999)
		if season == nil || episode == nil {
			return nil, errors.New("place: season or episode missing, out of range or not an integer")
		}
		t.Place = &videoPlace{Season: *season, Episode: *episode}
	}
	// with: other episodes of the same series (section 4.2).
	if kind == "episode" {
		list, _ := o["with"].([]any)
		for _, e := range list {
			s, ok := e.(string)
			if !ok || s == key || len(t.With) == 16 || videoIn(s, t.With) {
				continue
			}
			if w, err := parseVideoKey(s); err == nil && w.Kind == "episode" && w.Series == k.Series {
				t.With = append(t.With, s)
			}
		}
	}
	// ids (section 4.3): episodes of tmdb and tvdb, series of tvdb.
	episode := kind == "episode" && k.Scheme != "local"
	tvdbSeries := kind == "series" && k.Scheme == "tvdb"
	if ids, ok := o["ids"].(map[string]any); ok && (episode || tvdbSeries) {
		for scheme, val := range ids {
			s, ok := val.(string)
			if !ok || !(scheme == "imdb" && videoIMDbForm.MatchString(s) ||
				scheme == "tvdb" && episode && videoIDForm.MatchString(s) ||
				scheme == "tmdb" && k.Scheme == "tvdb" && videoIDForm.MatchString(s)) {
				continue
			}
			if t.IDs == nil {
				t.IDs = map[string]string{}
			}
			t.IDs[scheme] = s
		}
	}
	// copies (sections 3.2 and 3.8).
	if kind == "movie" || kind == "episode" || kind == "extra" {
		raw, present := o["copies"]
		list, ok := raw.([]any)
		if present && !ok {
			return nil, errors.New("copies isn't an array")
		}
		for _, e := range list {
			if len(t.Copies) == videoMaxCopies {
				break
			}
			c, err := keepCopy(e)
			if err != nil {
				continue
			}
			dup := false
			for _, kept := range t.Copies {
				dup = dup || kept.ID == c.ID
			}
			if !dup {
				t.Copies = append(t.Copies, *c)
			}
		}
		if len(t.Copies) == 0 {
			return nil, errors.New("no copy left")
		}
	}
	return videoEncode(t)
}

// What a viewer's server reads from a codecs string (section 3.5).

// videoCodecsFacts is what a viewer's server decides by: the parts of
// codecs it reads, the parts it ignores left out, bitDepth taken from
// the stream's own field when it has one. dolbyVision is the Dolby
// Vision string rebuilt from the base 4CC and dv.
type videoCodecsFacts struct {
	Codec       string `json:"codec"`
	Codecs      string `json:"codecs,omitempty"`
	Profile     *int64 `json:"profile,omitempty"`
	Tier        string `json:"tier,omitempty"`
	Level       *int64 `json:"level,omitempty"`
	BitDepth    *int64 `json:"bitDepth,omitempty"`
	Chroma      string `json:"chroma,omitempty"`
	Constraints string `json:"constraints,omitempty"`
	MP4A        string `json:"mp4a,omitempty"`
	DolbyVision string `json:"dolbyVision,omitempty"`
}

var (
	videoAVCForm   = regexp.MustCompile(`^(avc1|avc3)\.([0-9A-Fa-f]{2})[0-9A-Fa-f]{2}([0-9A-Fa-f]{2})$`)
	videoHEVCForm  = regexp.MustCompile(`^(hvc1|hev1)\.[ABC]?([0-9]{1,2})\.[0-9A-Fa-f]{1,8}\.([LH])([0-9]{1,3})((?:\.[0-9A-Fa-f]{1,2}){0,6})$`)
	videoAV1Form   = regexp.MustCompile(`^(av01)\.([0-7])\.([0-9]{2})([MH])\.([0-9]{2})(?:\.([01])\.([01][01][0-3])\.[0-9]{2}\.[0-9]{2}\.[0-9]{2}\.[01])?$`)
	videoVP9Form   = regexp.MustCompile(`^(vp09)\.([0-9]{2})\.[0-9]{2}\.([0-9]{2})(?:\.([0-9]{2})(?:\.[0-9]{2}){0,4})?$`)
	videoAACForm   = regexp.MustCompile(`^mp4a\.(40\.[1-9][0-9]?|66|67|68)$`)
	videoDTSForm   = regexp.MustCompile(`^(dtsc|dtsh|dtsl|dtse)$`)
	videoDVEntry   = map[string]string{"hvc1": "dvh1", "hev1": "dvhe", "av01": "dav1", "avc1": "dva1", "avc3": "dvav"}
	videoDVEntries = map[string]bool{"dvh1": true, "dvhe": true, "dav1": true, "dva1": true, "dvav": true}
)

// videoRequiresCodecs: the six codecs for which codecs is required.
var videoRequiresCodecs = []string{"h264", "hevc", "av1", "vp9", "aac", "dts"}

func videoNum(s string, base int) *int64 {
	n, err := strconv.ParseInt(s, base, 64)
	if err != nil {
		return nil
	}
	return &n
}

// readVideoCodecs is section 3.5's reading of a stream's or a track's
// codecs, given the stream as kept (codec, codecs, bitDepth, dv): the
// facts the viewer's server decides by, or why it treats the copy as one
// it can't play as it is.
func readVideoCodecs(raw []byte) ([]byte, error) {
	v, err := videoDecode(raw)
	if err != nil {
		return nil, err
	}
	o, ok := v.(map[string]any)
	if !ok {
		return nil, errors.New("not an object")
	}
	f := videoCodecsFacts{Codec: openTokenValue(o["codec"])}
	if f.Codec == "" {
		return nil, errors.New("codec missing or not a token")
	}
	f.Codecs = videoForm(o["codecs"], videoCodecsForm)
	required := videoIn(f.Codec, videoRequiresCodecs)
	if f.Codecs == "" {
		if required {
			return nil, fmt.Errorf("codecs missing or malformed, required for %s: can't play as it is", f.Codec)
		}
		return videoEncode(f)
	}
	own := videoInt(o["bitDepth"], 1, 32) // the authoritative bit depth
	for dv := range videoDVEntries {
		if strings.HasPrefix(f.Codecs, dv+".") || f.Codecs == dv {
			return nil, fmt.Errorf("a Dolby Vision 4CC (%s): codecs carries the base codec's; can't play as it is", dv)
		}
	}
	var fourCC string
	switch f.Codec {
	case "h264":
		m := videoAVCForm.FindStringSubmatch(f.Codecs)
		if m == nil {
			return nil, errors.New("codecs isn't avc1 or avc3 and three bytes: can't play as it is")
		}
		fourCC, f.Profile, f.Level = m[1], videoNum(m[2], 16), videoNum(m[3], 16)
	case "hevc":
		m := videoHEVCForm.FindStringSubmatch(f.Codecs)
		if m == nil {
			return nil, errors.New("codecs isn't an HEVC string of hvc1 or hev1: can't play as it is")
		}
		fourCC, f.Profile, f.Tier, f.Level = m[1], videoNum(m[2], 10), m[3], videoNum(m[4], 10)
		f.Constraints = strings.TrimPrefix(m[5], ".")
		if *f.Profile >= 4 && f.Constraints == "" {
			return nil, errors.New("an HEVC profile of 4 or more without its constraint bytes: can't play as it is")
		}
	case "av1":
		m := videoAV1Form.FindStringSubmatch(f.Codecs)
		if m == nil {
			return nil, errors.New("codecs isn't an AV1 string of av01: can't play as it is")
		}
		fourCC, f.Profile, f.Level, f.Tier, f.BitDepth = m[1], videoNum(m[2], 10), videoNum(m[3], 10), m[4], videoNum(m[5], 10)
		switch {
		case m[6] == "1":
			f.Chroma = "4:0:0"
		case m[7] != "":
			f.Chroma = map[string]string{"110": "4:2:0", "111": "4:2:0", "112": "4:2:0", "113": "4:2:0", "100": "4:2:2", "000": "4:4:4"}[m[7]]
		case *f.Profile == 0:
			f.Chroma = "4:2:0"
		case *f.Profile == 1:
			return nil, errors.New("AV1 profile 1 (4:4:4) without the long form: can't play as it is")
		}
	case "vp9":
		m := videoVP9Form.FindStringSubmatch(f.Codecs)
		if m == nil {
			return nil, errors.New("codecs isn't a VP9 string of vp09: can't play as it is")
		}
		fourCC, f.Profile, f.BitDepth = m[1], videoNum(m[2], 10), videoNum(m[3], 10)
		switch m[4] {
		case "00", "01":
			f.Chroma = "4:2:0"
		case "02":
			f.Chroma = "4:2:2"
		case "03":
			f.Chroma = "4:4:4"
		case "":
			if *f.Profile == 1 || *f.Profile == 3 {
				return nil, errors.New("VP9 profile 1 or 3 (not 4:2:0) without the long form: can't play as it is")
			}
			f.Chroma = "4:2:0"
		default:
			return nil, errors.New("a VP9 chroma subsampling other than 00 to 03: can't play as it is")
		}
	case "aac":
		m := videoAACForm.FindStringSubmatch(f.Codecs)
		if m == nil {
			return nil, errors.New("codecs isn't mp4a.40.<n> or an MPEG-2 AAC mp4a: can't play as it is")
		}
		f.MP4A = m[1]
	case "dts":
		if !videoDTSForm.MatchString(f.Codecs) {
			return nil, errors.New("codecs isn't dtsc, dtsh, dtsl or dtse: can't play as it is")
		}
	}
	if own != nil {
		f.BitDepth = own
	}
	if dv, ok := o["dv"].(map[string]any); ok && fourCC != "" {
		p, l := videoInt(dv["profile"], 0, 127), videoInt(dv["level"], 0, 63)
		if p != nil && l != nil {
			f.DolbyVision = fmt.Sprintf("%s.%02d.%02d", videoDVEntry[fourCC], *p, *l)
		}
	}
	return videoEncode(f)
}

// The video members of play's answer (sections 5.3 and 6.1).

// videoServedForm is section 5.4's table: the form and ass a track of a
// listed format is served in.
var videoServedForm = map[string][2]string{
	"subrip": {"text", ""}, "webvtt": {"text", ""}, "mov_text": {"text", ""}, "ttml": {"text", ""},
	"ass": {"text", "ass"}, "ssa": {"text", "ass"},
	"hdmv_pgs_subtitle": {"picture", ""}, "dvd_subtitle": {"picture", ""}, "dvb_subtitle": {"picture", ""},
}

type videoServed struct {
	ID   string `json:"id"`
	Form string `json:"form"`
	ASS  *bool  `json:"ass,omitempty"`
}

type videoAnswer struct {
	Subtitles []videoServed `json:"subtitles,omitempty"`
	FontCount int64         `json:"fontCount"`
	URLs      []string      `json:"urls"`
}

// keepVideoAnswer reads the video members of play's answer for a copy
// of the catalog, and builds the URLs of its files from the stream root
// and the answer's ticket: an error refuses the answer (a ticket out of
// its form).
func keepVideoAnswer(raw []byte, cp *videoCopy, root string) ([]byte, error) {
	v, err := videoDecode(raw)
	if err != nil {
		return nil, err
	}
	o, ok := v.(map[string]any)
	if !ok {
		return nil, errors.New("not an object")
	}
	ticket := videoForm(o["ticket"], videoOpaqueForm)
	if ticket == "" {
		return nil, errors.New("ticket missing or not 1 to 64 characters of A-Z a-z 0-9 _ -")
	}
	formats := map[string]string{}
	for _, s := range cp.SubtitleTracks {
		formats[s.ID] = s.Format
	}
	a := videoAnswer{URLs: []string{}}
	base := root + ticket + "/"
	list, _ := o["subtitles"].([]any)
	for _, e := range list {
		m, ok := e.(map[string]any)
		if !ok || len(a.Subtitles) == 64 {
			continue
		}
		id, _ := m["id"].(string)
		form, _ := m["form"].(string)
		format, inCopy := formats[id]
		dup := false
		for _, s := range a.Subtitles {
			dup = dup || s.ID == id
		}
		if !inCopy || dup || form != "text" && form != "picture" {
			continue
		}
		ass := videoBool(m["ass"])
		if form != "text" || ass != nil && !*ass {
			ass = nil
		}
		if want, listed := videoServedForm[format]; listed && (want[0] != form || (want[1] == "ass") != (ass != nil)) {
			continue
		}
		a.Subtitles = append(a.Subtitles, videoServed{id, form, ass})
		if form == "text" {
			a.URLs = append(a.URLs, base+"subtitles/"+id)
		} else {
			a.URLs = append(a.URLs, base+"pictures/"+id)
		}
		if ass != nil {
			a.URLs = append(a.URLs, base+"ass/"+id)
		}
	}
	if n := videoInt(o["fontCount"], 0, videoMaxFonts); n != nil {
		a.FontCount = *n
	}
	for i := int64(0); i < a.FontCount; i++ {
		a.URLs = append(a.URLs, base+"fonts/"+strconv.FormatInt(i, 10))
	}
	if l := cp.PreviewLayout; l != nil {
		for i := int64(0); i < l.sheets(); i++ {
			a.URLs = append(a.URLs, base+"trickplay/"+strconv.FormatInt(i, 10))
		}
	}
	return videoEncode(a)
}

// The previews call (section 5.6).

// checkVideoPreviewsRequest: a JSON object whose title is the key of a
// film, an episode or an extra and whose versionId has a copy ID's form;
// otherwise the owning server answers 400.
func checkVideoPreviewsRequest(raw []byte) error {
	v, err := videoDecode(raw)
	if err != nil {
		return err
	}
	o, ok := v.(map[string]any)
	if !ok {
		return errors.New("not an object")
	}
	title, ok := o["title"].(string)
	if !ok {
		return errors.New("title missing or not a string")
	}
	kind, err := checkVideoKey(title)
	if err != nil {
		return fmt.Errorf("title: %v", err)
	}
	if kind != "movie" && kind != "episode" && kind != "extra" {
		return fmt.Errorf("title: a %s has no copies", kind)
	}
	id, ok := o["versionId"].(string)
	if !ok {
		return errors.New("versionId missing or not a string")
	}
	if !videoOpaqueForm.MatchString(id) {
		return errors.New("versionId isn't 1 to 64 characters of A-Z a-z 0-9 _ -")
	}
	return nil
}

type videoPreviewsAnswer struct {
	Ticket string `json:"ticket"`
	ETag   string `json:"etag"`
}

// keepVideoPreviewsAnswer applies section 5.6 to a previews answer: an
// error refuses it (no sheets); otherwise the ticket and etag kept.
func keepVideoPreviewsAnswer(raw []byte) ([]byte, error) {
	v, err := videoDecode(raw)
	if err != nil {
		return nil, err
	}
	o, ok := v.(map[string]any)
	if !ok {
		return nil, errors.New("not an object")
	}
	a := videoPreviewsAnswer{videoForm(o["ticket"], videoOpaqueForm), videoForm(o["etag"], videoOpaqueForm)}
	if a.Ticket == "" {
		return nil, errors.New("ticket missing or not 1 to 64 characters of A-Z a-z 0-9 _ -")
	}
	if a.ETag == "" {
		return nil, errors.New("etag missing or not 1 to 64 characters of A-Z a-z 0-9 _ -")
	}
	return videoEncode(a)
}

// Fonts (section 6.6).

// videoFontSet is what checkVideoFontSet finds in a set of fonts.
type videoFontSet struct {
	Fonts int   `json:"fonts"`
	Bytes int64 `json:"bytes"`
}

// checkVideoFontSet is section 6.6's bounds on one copy's fonts, given
// their sizes in bytes in the order of fonts: at most 64 fonts, each 1 to
// 10 MiB, 64 MiB in all. It returns their count and total, or the first
// bound broken; the owning server lists a set only within them, and a
// viewer's server refuses the font that breaks one.
func checkVideoFontSet(raw []byte) ([]byte, error) {
	v, err := videoDecode(raw)
	if err != nil {
		return nil, err
	}
	list, ok := v.([]any)
	if !ok {
		return nil, errors.New("not an array of sizes")
	}
	var set videoFontSet
	for i, e := range list {
		n := videoInt(e, 1, 1<<53-1)
		switch {
		case n == nil:
			return nil, fmt.Errorf("font %d: a size that isn't an integer from 1", i)
		case i >= videoMaxFonts:
			return nil, fmt.Errorf("font %d: more than 64 fonts", i)
		case *n > videoMaxFont:
			return nil, fmt.Errorf("font %d: over 10 MiB (10,485,760 bytes)", i)
		case set.Bytes+*n > videoMaxFontSet:
			return nil, fmt.Errorf("font %d: the copy's fonts over 64 MiB (67,108,864 bytes) in all", i)
		}
		set.Fonts++
		set.Bytes += *n
	}
	return videoEncode(set)
}
