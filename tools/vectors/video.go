package main

// The video part's rules (spec/v0/video.md, as revised by proposal 16):
// title keys of the tmdb, tvdb and local schemes and extras' keys
// (section 4), what a caller keeps of a video title (section 3.1), of
// its copies (sections 3.2 to 3.8), of an extra (section 3.9) and of a
// local title's description (section 3.10), and the data checks of the
// previews call (sections 5.5 and 5.6).
//
// Every helper here has a video prefix (or is a method of a video type),
// so that this file depends on no other part's file.

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
)

var (
	videoIDForm      = regexp.MustCompile(`^[1-9][0-9]{0,9}$`)        // a TMDB or TheTVDB ID, a TMDB episode ID
	videoOpaqueForm  = regexp.MustCompile(`^[A-Za-z0-9_-]{1,64}$`)    // local identifiers, extras, copy IDs, etags, tickets
	videoSeasonForm  = regexp.MustCompile(`^s(0|[1-9][0-9]{0,3})$`)   // s<season>
	videoEpisodeForm = regexp.MustCompile(`^e(0|[1-9][0-9]{0,4})$`)   // e<episode>
	videoAbsForm     = regexp.MustCompile(`^a(0|[1-9][0-9]{0,4})$`)   // a<number>
	videoEpIDForm    = regexp.MustCompile(`^e[1-9][0-9]{0,9}$`)       // e<episode id>
	videoAudioID     = regexp.MustCompile(`^a(0|[1-9][0-9]{0,2})$`)   // a<n>, n 0 to 999
	videoSubtitleID  = regexp.MustCompile(`^s(0|[1-9][0-9]{0,2})$`)   // s<n>, n 0 to 999
	videoTagForm     = regexp.MustCompile(`^[A-Za-z0-9-]{1,12}$`)     // a track's language
	videoIMDbForm    = regexp.MustCompile(`^tt[0-9]{7,10}$`)          // ids.imdb
	videoCodecTag    = regexp.MustCompile(`^[A-Za-z0-9]{1,8}$`)       // video codecTag
	videoLayoutForm  = regexp.MustCompile(`^[A-Za-z0-9()._-]{1,32}$`) // audio layout
	videoColourForm  = regexp.MustCompile(`^[a-z0-9_-]{1,32}$`)       // each colour member
	videoIntegerForm = regexp.MustCompile(`^-?(0|[1-9][0-9]*)$`)
	videoDateForm    = regexp.MustCompile(`^[0-9]{4}-[0-9]{2}-[0-9]{2}$`)
)

// The names lists of section 3.5 (both columns), and the other lists of
// sections 3.2, 3.3, 3.4 and 3.9.
var (
	videoContainers  = []string{"mp4", "m4v", "mov", "webm", "mkv", "ts", "mp3", "aac", "m4a", "flac", "ogg", "opus", "wav", "avi", "m2ts", "wmv", "flv", "3gp"}
	videoCodecs      = []string{"h264", "hevc", "av1", "vp8", "vp9", "mpeg2video", "mpeg4", "vc1", "wmv3", "h263", "theora"}
	videoAudioCodecs = []string{"aac", "mp3", "opus", "vorbis", "flac", "ac3", "eac3", "alac", "pcm_s16le", "dts", "truehd", "mp2", "wmav2", "pcm_s24le", "pcm_bluray"}
	videoEditions    = []string{"theatrical", "directors", "extended", "unrated", "remastered", "special", "imax", "other"}
	videoHDR         = []string{"hdr10", "hdr10plus", "hlg", "dv"}
	videoThreeD      = []string{"sbs", "tab", "mvc"}
	videoRoles       = []string{"commentary", "description"}
	videoAudioProfs  = []string{"atmos", "dtsx", "dts-hd-ma", "dts-hd-hra", "dts-es", "he-aac", "lc"}
	videoSubFormats  = []string{"subrip", "ass", "ssa", "webvtt", "mov_text", "ttml", "pgs", "vobsub", "dvb", "other"}
	videoSegKinds    = []string{"intro", "outro", "recap", "preview", "commercial"}
	videoExtraTypes  = []string{"trailer", "featurette", "behindthescenes", "deletedscene", "interview", "scene", "short", "clip"}
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

// videoForm is v when it is a string matching re, else "".
func videoForm(v any, re *regexp.Regexp) string {
	if s, ok := v.(string); ok && re.MatchString(s) {
		return s
	}
	return ""
}

// videoOneOf is v when it is a string of list, else "".
func videoOneOf(v any, list []string) string {
	if s, ok := v.(string); ok && videoIn(s, list) {
		return s
	}
	return ""
}

// videoNamed: a string of list kept, any other string "other".
func videoNamed(s string, list []string) string {
	if videoIn(s, list) {
		return s
	}
	return "other"
}

// What the caller keeps, in the order the vectors write it.

type videoPart struct {
	Size       int64 `json:"size"`
	DurationMs int64 `json:"durationMs"`
}

type videoDV struct {
	Profile       int64 `json:"profile"`
	Level         int64 `json:"level"`
	Compatibility int64 `json:"compatibility"`
}

type videoColour struct {
	Primaries string `json:"primaries,omitempty"`
	Transfer  string `json:"transfer,omitempty"`
	Matrix    string `json:"matrix,omitempty"`
	Range     string `json:"range,omitempty"`
}

type videoStream struct {
	Codec      string       `json:"codec"`
	CodecTag   string       `json:"codecTag,omitempty"`
	Profile    string       `json:"profile,omitempty"`
	Level      json.Number  `json:"level,omitempty"`
	Width      int64        `json:"width"`
	Height     int64        `json:"height"`
	BitDepth   *int64       `json:"bitDepth,omitempty"`
	HDR        []string     `json:"hdr,omitempty"`
	DV         *videoDV     `json:"dv,omitempty"`
	FPS        json.Number  `json:"fps,omitempty"`
	Interlaced *bool        `json:"interlaced,omitempty"`
	Colour     *videoColour `json:"colour,omitempty"`
}

type videoAudio struct {
	ID         string `json:"id"`
	Language   string `json:"language,omitempty"`
	Channels   *int64 `json:"channels,omitempty"`
	Role       string `json:"role,omitempty"`
	Default    *bool  `json:"default,omitempty"`
	Forced     *bool  `json:"forced,omitempty"`
	FileIndex  *int64 `json:"fileIndex,omitempty"`
	Codec      string `json:"codec"`
	Profile    string `json:"profile,omitempty"`
	Layout     string `json:"layout,omitempty"`
	SampleRate *int64 `json:"sampleRate,omitempty"`
	BitDepth   *int64 `json:"bitDepth,omitempty"`
	Title      string `json:"title,omitempty"`
	Original   *bool  `json:"original,omitempty"`
}

type videoSubtitle struct {
	ID              string `json:"id"`
	Language        string `json:"language,omitempty"`
	Default         *bool  `json:"default,omitempty"`
	Forced          *bool  `json:"forced,omitempty"`
	HearingImpaired *bool  `json:"hearingImpaired,omitempty"`
	Format          string `json:"format"`
	External        *bool  `json:"external,omitempty"`
	FileIndex       *int64 `json:"fileIndex,omitempty"`
	Title           string `json:"title,omitempty"`
}

type videoChapter struct {
	StartMs int64  `json:"startMs"`
	Name    string `json:"name,omitempty"`
}

type videoSegment struct {
	Kind    string `json:"kind"`
	StartMs int64  `json:"startMs"`
	EndMs   int64  `json:"endMs"`
	Final   *bool  `json:"final,omitempty"`
}

type videoCopy struct {
	ID             string          `json:"id"`
	Name           string          `json:"name,omitempty"`
	Edition        string          `json:"edition,omitempty"`
	DurationMs     int64           `json:"durationMs"`
	StartMs        *int64          `json:"startMs,omitempty"`
	Container      string          `json:"container"`
	Size           int64           `json:"size"`
	ETag           string          `json:"etag"`
	Parts          []videoPart     `json:"parts,omitempty"`
	BitrateKbps    *int64          `json:"bitrateKbps,omitempty"`
	Video          *videoStream    `json:"video,omitempty"`
	AudioTracks    []videoAudio    `json:"audioTracks"`
	SubtitleTracks []videoSubtitle `json:"subtitleTracks,omitempty"`
	Chapters       []videoChapter  `json:"chapters,omitempty"`
	Segments       []videoSegment  `json:"segments,omitempty"`
	Previews       *bool           `json:"previews,omitempty"`
	Added          string          `json:"added,omitempty"`
	ThreeD         string          `json:"threeD,omitempty"`
}

type videoPlace struct {
	Season  int64 `json:"season"`
	Episode int64 `json:"episode"`
}

type videoTitle struct {
	Key       string            `json:"key"`
	Kind      string            `json:"kind"`
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
	videoMaxCopies   = 8
)

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

// videoNumber reads a JSON number of any form as a float.
func videoNumber(v any) (float64, bool) {
	n, ok := v.(json.Number)
	if !ok {
		return 0, false
	}
	f, err := strconv.ParseFloat(n.String(), 64)
	return f, err == nil
}

func videoNumText(f float64) json.Number {
	return json.Number(strconv.FormatFloat(f, 'f', -1, 64))
}

// keepVideoStream is section 3.8's reading of a copy's video: nil when
// it counts as absent.
func keepVideoStream(v any) *videoStream {
	o, ok := v.(map[string]any)
	if !ok {
		return nil
	}
	codec, ok := o["codec"].(string)
	w, h := videoInt(o["width"], 1, 100_000), videoInt(o["height"], 1, 100_000)
	if !ok || w == nil || h == nil {
		return nil
	}
	s := &videoStream{Codec: videoNamed(codec, videoCodecs), Width: *w, Height: *h}
	s.CodecTag = videoForm(o["codecTag"], videoCodecTag)
	if p, ok := o["profile"].(string); ok && p != "" && len([]rune(p)) <= 32 {
		s.Profile = p
	}
	if f, ok := videoNumber(o["level"]); ok && f >= 0 && f <= 1000 {
		s.Level = videoNumText(f)
	}
	if b := videoInt(o["bitDepth"], 8, 12); b != nil && *b%2 == 0 {
		s.BitDepth = b
	}
	if list, ok := o["hdr"].([]any); ok {
		have := map[string]bool{}
		for _, e := range list {
			if f, ok := e.(string); ok {
				have[f] = true
			}
		}
		for _, f := range videoHDR {
			if have[f] {
				s.HDR = append(s.HDR, f)
			}
		}
	}
	if dv, ok := o["dv"].(map[string]any); ok && videoIn("dv", s.HDR) {
		p, l, c := videoInt(dv["profile"], 0, 10), videoInt(dv["level"], 0, 13), videoInt(dv["compatibility"], 0, 15)
		if p != nil && l != nil && c != nil {
			s.DV = &videoDV{*p, *l, *c}
		}
	}
	if f, ok := videoNumber(o["fps"]); ok {
		if r := math.Round(f*100) / 100; r > 0 && r <= 1000 {
			s.FPS = videoNumText(r)
		}
	}
	s.Interlaced = videoBool(o["interlaced"])
	if c, ok := o["colour"].(map[string]any); ok {
		col := videoColour{
			Primaries: videoForm(c["primaries"], videoColourForm),
			Transfer:  videoForm(c["transfer"], videoColourForm),
			Matrix:    videoForm(c["matrix"], videoColourForm),
			Range:     videoForm(c["range"], videoColourForm),
		}
		if col != (videoColour{}) {
			s.Colour = &col
		}
	}
	return s
}

// keepVideoParts: nil unless every part checks out and the sizes add up.
func keepVideoParts(v any, size int64) []videoPart {
	list, ok := v.([]any)
	if !ok || len(list) == 0 || len(list) > 16 {
		return nil
	}
	var out []videoPart
	var sum int64
	for _, e := range list {
		o, ok := e.(map[string]any)
		if !ok {
			return nil
		}
		s, d := videoInt(o["size"], 1, videoMaxSize), videoInt(o["durationMs"], 0, videoMaxDuration)
		if s == nil || d == nil {
			return nil
		}
		sum += *s
		out = append(out, videoPart{*s, *d})
	}
	if sum != size {
		return nil
	}
	return out
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
		codec, ok := o["codec"].(string)
		if id == "" || seen[id] || !ok {
			continue
		}
		seen[id] = true
		out = append(out, videoAudio{
			ID:         id,
			Language:   videoForm(o["language"], videoTagForm),
			Channels:   videoInt(o["channels"], 1, 64),
			Role:       videoOneOf(o["role"], videoRoles),
			Default:    videoBool(o["default"]),
			Forced:     videoBool(o["forced"]),
			FileIndex:  videoInt(o["fileIndex"], 0, 999),
			Codec:      videoNamed(codec, videoAudioCodecs),
			Profile:    videoOneOf(o["profile"], videoAudioProfs),
			Layout:     videoForm(o["layout"], videoLayoutForm),
			SampleRate: videoInt(o["sampleRate"], 8_000, 768_000),
			BitDepth:   videoInt(o["bitDepth"], 8, 32),
			Title:      videoPlain(o["title"], 32),
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
		format, _ := o["format"].(string)
		out = append(out, videoSubtitle{
			ID:              id,
			Language:        videoForm(o["language"], videoTagForm),
			Default:         videoBool(o["default"]),
			Forced:          videoBool(o["forced"]),
			HearingImpaired: videoBool(o["hearingImpaired"]),
			Format:          videoNamed(format, videoSubFormats),
			External:        videoBool(o["external"]),
			FileIndex:       videoInt(o["fileIndex"], 0, 999),
			Title:           videoPlain(o["title"], 32),
		})
	}
	return out
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
		kind := videoOneOf(o["kind"], videoSegKinds)
		s, t := videoInt(o["startMs"], 0, duration), videoInt(o["endMs"], 0, duration)
		if kind == "" || s == nil || t == nil || *s >= *t {
			continue
		}
		out = append(out, videoSegment{kind, *s, *t, videoBool(o["final"])})
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
	container, ok := o["container"].(string)
	if !ok {
		return nil, errors.New("container isn't a string")
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
	c := &videoCopy{ID: id, DurationMs: *d, Container: videoNamed(container, videoContainers), Size: *size, ETag: etag}
	c.Name = videoPlain(o["name"], 32)
	if e, ok := o["edition"].(string); ok {
		c.Edition = videoNamed(e, videoEditions)
	}
	c.StartMs = videoInt(o["startMs"], 0, *d)
	c.Parts = keepVideoParts(o["parts"], *size)
	if b := videoInt(o["bitrateKbps"], 500, 10_000_000); b != nil {
		r := *b / 500 * 500
		c.BitrateKbps = &r
	}
	c.Video = keepVideoStream(o["video"])
	c.AudioTracks = keepVideoAudio(audio)
	if subs, ok := o["subtitleTracks"].([]any); ok {
		c.SubtitleTracks = keepVideoSubtitles(subs)
	}
	c.Chapters = keepVideoChapters(o["chapters"], *d)
	c.Segments = keepVideoSegments(o["segments"], *d)
	c.Previews = videoBool(o["previews"])
	if s, ok := o["added"].(string); ok && videoDateForm.MatchString(s) {
		if _, err := time.Parse("2006-01-02", s); err == nil {
			c.Added = s
		}
	}
	c.ThreeD = videoOneOf(o["threeD"], videoThreeD)
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
// or why the title is skipped.
func keepVideoTitle(raw []byte, libraries []string) ([]byte, error) {
	v, err := videoDecode(raw)
	if err != nil {
		return nil, err
	}
	o, ok := v.(map[string]any)
	if !ok {
		return nil, errors.New("not an object")
	}
	key, ok1 := o["key"].(string)
	kind, ok2 := o["kind"].(string)
	lib, ok3 := o["library"].(string)
	if !ok1 || !ok2 || !ok3 {
		return nil, errors.New("key, kind or library isn't a string")
	}
	k, err := parseVideoKey(key)
	if err != nil {
		return nil, fmt.Errorf("key: %v", err)
	}
	if kind != k.Kind {
		return nil, fmt.Errorf("kind %q, but the key names a %s", kind, k.Kind)
	}
	if !videoIn(lib, libraries) {
		return nil, fmt.Errorf("library %q wasn't asked for", lib)
	}
	t := videoTitle{Key: key, Kind: kind, Library: lib}

	// Extras (section 3.9).
	if kind == "extra" {
		t.ExtraType = "clip"
		if s, ok := o["extraType"].(string); ok && videoIn(s, videoExtraTypes) {
			t.ExtraType = s
		}
		t.Name = videoPlain(o["name"], 64)
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
	// ids (section 4.3).
	if ids, ok := o["ids"].(map[string]any); ok && k.Scheme != "local" && (kind == "movie" || kind == "series" || kind == "episode") {
		for scheme, val := range ids {
			s, ok := val.(string)
			if !ok || !(scheme == "imdb" && videoIMDbForm.MatchString(s) || scheme == "tvdb" && videoIDForm.MatchString(s)) {
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

// videoTrickplay is a kept trickplay object (section 5.5).
type videoTrickplay struct {
	TileWidth  int64    `json:"tileWidth"`
	TileHeight int64    `json:"tileHeight"`
	Columns    int64    `json:"columns"`
	Rows       int64    `json:"rows"`
	IntervalMs int64    `json:"intervalMs"`
	TileCount  int64    `json:"tileCount"`
	Sheets     []string `json:"sheets"`
}

type videoPreviewsAnswer struct {
	Trickplay *videoTrickplay `json:"trickplay,omitempty"`
}

var videoTrickplayFields = []string{"tileWidth", "tileHeight", "columns", "rows", "intervalMs", "tileCount"}

// keepVideoTrickplay is section 5.5's reading of a trickplay object,
// given the owning server's stream root: an error refuses the whole
// answer (a field of another JSON type, a sheet URL that isn't the one
// section 6.1 gives); nil, nil drops the previews (a number missing or
// out of bounds, another number of sheets).
func keepVideoTrickplay(v any, root string) (*videoTrickplay, error) {
	o, ok := v.(map[string]any)
	if !ok {
		return nil, errors.New("trickplay isn't an object")
	}
	n := map[string]int64{}
	for _, f := range videoTrickplayFields {
		raw, present := o[f]
		if !present {
			continue
		}
		i := videoInt(raw, math.MinInt64, math.MaxInt64)
		if i == nil {
			return nil, fmt.Errorf("%s isn't an integer", f)
		}
		n[f] = *i
	}
	var sheets []string
	if raw, present := o["sheets"]; present {
		list, ok := raw.([]any)
		if !ok {
			return nil, errors.New("sheets isn't an array")
		}
		ticket := ""
		for i, e := range list {
			s, ok := e.(string)
			if !ok {
				return nil, fmt.Errorf("sheets[%d] isn't a string", i)
			}
			rest, ok := strings.CutPrefix(s, root)
			tk, tail, cut := strings.Cut(rest, "/")
			if !ok || !cut || !videoOpaqueForm.MatchString(tk) || tail != "trickplay/"+strconv.Itoa(i) {
				return nil, fmt.Errorf("sheets[%d] isn't <stream root><ticket>/trickplay/%d", i, i)
			}
			if ticket != "" && tk != ticket {
				return nil, fmt.Errorf("sheets[%d] names another ticket", i)
			}
			ticket = tk
			sheets = append(sheets, s)
		}
	}
	in := func(f string, lo, hi int64) bool { x, ok := n[f]; return ok && x >= lo && x <= hi }
	if !in("tileWidth", 1, 2048) || !in("tileHeight", 1, 2048) || !in("columns", 1, 100) || !in("rows", 1, 100) ||
		!in("intervalMs", 100, 3_600_000) || !in("tileCount", 1, math.MaxInt64) || sheets == nil {
		return nil, nil
	}
	if n["columns"]*n["tileWidth"] > 4096 || n["rows"]*n["tileHeight"] > 4096 {
		return nil, nil
	}
	per := n["columns"] * n["rows"]
	if want := (n["tileCount"] + per - 1) / per; want > 1000 || int64(len(sheets)) != want {
		return nil, nil
	}
	return &videoTrickplay{n["tileWidth"], n["tileHeight"], n["columns"], n["rows"], n["intervalMs"], n["tileCount"], sheets}, nil
}

// keepVideoPreviewsAnswer applies section 5.6 to a previews answer: an
// error refuses it; otherwise what the viewer's server keeps, {} when
// it drops the previews.
func keepVideoPreviewsAnswer(raw []byte, root string) ([]byte, error) {
	v, err := videoDecode(raw)
	if err != nil {
		return nil, err
	}
	o, ok := v.(map[string]any)
	if !ok {
		return nil, errors.New("not an object")
	}
	var a videoPreviewsAnswer
	if tp, present := o["trickplay"]; present {
		if a.Trickplay, err = keepVideoTrickplay(tp, root); err != nil {
			return nil, err
		}
	}
	return videoEncode(a)
}
