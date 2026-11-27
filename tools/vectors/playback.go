package main

// The rules of playing (MSFP Media's playing.md, with its video sections
// 5.1 and 5.3's members and fetching section 1's URLs), as MSFP's
// playback part carries them between servers (spec/v0/playback.md: the
// purpose scan, no capability list, the pairing's limits), as a
// viewer's server (the caller) and an owning server (the source) apply
// them:
//
//   - playbackPlay: an owning server's checks of a play request, in the
//     order of playing section 2.2, against the copies and limits of a
//     context; playbackQueueEntry the same for one entry of a queue;
//   - playbackKeepAnswer: what a viewer's server keeps of a play answer
//     (playing section 2.3, MSFP Media video section 5.3), and the URLs it
//     builds from the ticket (fetching section 1);
//   - playbackReadRefusal: what a viewer's server does with a play
//     refused (playing section 2.5, MSFP Media base section 4, MSFP
//     MSFP Media playing section 2);
//   - playbackRestart: whether a viewer's server may join the bytes of a
//     new playback to a file it was reading (playing section 2.7);
//   - playbackReport: an owning server's answer to progress and stop
//     (playing sections 3 to 6);
//   - playbackKeepLimits: what a caller keeps of limits (MSFP playback
//     section 5).
//
// A section named alone is playing's.

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"slices"
	"strconv"
	"strings"
)

// The language's own lists and steps (playing sections 2.1 and 2.7), with
// MSFP's purpose scan (MSFP playback section 2).
var (
	playbackPurposes         = []string{"play", "download", "scan"}
	playbackDeliveries       = []string{"file", "remux", "transcode"}
	playbackHeights          = []int64{480, 720, 1080, 1440, 2160, 4320}
	playbackChannelSteps     = []int64{2, 6, 8}
	playbackBaselineVideo    = "h264"
	playbackBaselineAudio    = "aac"
	playbackBaselineChannels = int64(2)
)

// Bounds.
const (
	playbackMaxPositionMs = 8640000000 // 100 days
	playbackMaxKbps       = 10000000
	playbackAtBack        = 30 * 86400
	playbackAtAhead       = 5 * 60
	playbackLateWindow    = 30 * 86400
	playbackDownloadTTL   = 86400
	playbackScanTTL       = 600
	playbackProgressLow   = 45 // progressSeconds (playing section 2.3, 3)
	playbackProgressHigh  = 120
	playbackProgressDef   = 60
	playbackMaxStreams    = 1000
	playbackScanFloor     = 4 << 20 // scanBytes absent or out of range
	playbackScanTop       = 1 << 30
)

var (
	playbackIDForm     = regexp.MustCompile(`^[A-Za-z0-9_-]{1,64}$`)
	playbackEtagForm   = regexp.MustCompile(`^[A-Za-z0-9_-]{1,64}$`)
	playbackObjIDForm  = regexp.MustCompile(`^[A-Za-z0-9_-]{1,64}$`)
	playbackAudioForm  = regexp.MustCompile(`^a(0|[1-9][0-9]{0,2})$`)
	playbackSubForm    = regexp.MustCompile(`^s(0|[1-9][0-9]{0,2})$`)
	playbackTicketForm = `[A-Za-z0-9_-]{1,64}`

	// Title keys (MSFP Media video section 4, MSFP Media music section 2).
	pbID      = `([1-9][0-9]{0,9})`
	pbOpaque  = `[A-Za-z0-9_-]{1,64}`
	pbSeason  = `/s(0|[1-9][0-9]{0,3})`
	pbEpisode = `/e(0|[1-9][0-9]{0,4})`
	pbFilm    = `movie/tmdb/` + pbID + `|movie/local/` + pbOpaque
	pbSeries  = `series/tmdb/` + pbID + `|series/tvdb/` + pbID + `|series/local/` + pbOpaque
	pbMBID    = `[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}`

	playbackKinds = []struct {
		kind, part string
		form       *regexp.Regexp
	}{
		{"film", "video", regexp.MustCompile(`^(` + pbFilm + `)$`)},
		{"series", "", regexp.MustCompile(`^(` + pbSeries + `)$`)},
		{"season", "", regexp.MustCompile(`^(` + pbSeries + `)` + pbSeason + `$`)},
		{"episode", "video", regexp.MustCompile(`^((` + pbSeries + `)` + pbSeason + pbEpisode + `|series/tmdb/` + pbID + `/e[1-9][0-9]{0,9}|series/tvdb/` + pbID + `/a(0|[1-9][0-9]{0,4}))$`)},
		{"extra", "video", regexp.MustCompile(`^(` + pbFilm + `|(` + pbSeries + `)(` + pbSeason + `)?)/x/` + pbOpaque + `$`)},
		{"track", "music", regexp.MustCompile(`^track/(mb/` + pbMBID + `|local/` + pbOpaque + `)$`)},
		{"album", "", regexp.MustCompile(`^album/(mb/` + pbMBID + `|local/` + pbOpaque + `)$`)},
		{"artist", "", regexp.MustCompile(`^artist/(mb/` + pbMBID + `|local/` + pbOpaque + `)$`)},
	}
)

// playbackTitleKind names the kind (IDs of at most 10 digits are at most
// 9,999,999,999 by their form) of a title key and the content part
// whose play takes it ("" for a kind play doesn't take).
func playbackTitleKind(key string) (kind, part string, err error) {
	for _, k := range playbackKinds {
		if k.form.MatchString(key) {
			return k.kind, k.part, nil
		}
	}
	return "", "", errors.New("not a canonical title key")
}

// playbackRefusal is an owning server's refusal: a status and a problem type.
type playbackRefusal struct {
	Status int
	Type   string
	Why    string
}

func (r *playbackRefusal) Error() string { return r.Why }

func pbBad(format string, a ...any) error {
	return &playbackRefusal{400, "invalid-request", fmt.Sprintf(format, a...)}
}

func pbRefuse(status int, typ, why string) error { return &playbackRefusal{status, typ, why} }

// playbackStatus gives the status and type of an error of this file (200
// and "" for none).
func playbackStatus(err error) (int, string) {
	var r *playbackRefusal
	if errors.As(err, &r) {
		return r.Status, r.Type
	}
	if err != nil {
		return 400, "invalid-request"
	}
	return 200, ""
}

func playbackDecode(raw []byte) (map[string]any, error) {
	if err := checkIJSON(raw); err != nil {
		return nil, err
	}
	d := json.NewDecoder(bytes.NewReader(raw))
	d.UseNumber()
	var v any
	if err := d.Decode(&v); err != nil {
		return nil, err
	}
	o, ok := v.(map[string]any)
	if !ok {
		return nil, errors.New("not a JSON object")
	}
	return o, nil
}

// playbackInt reads an integer (MSFP Media base section 2.2).
func playbackInt(v any) (int64, bool) {
	n, ok := v.(json.Number)
	if !ok || checkInteger(n.String()) != nil {
		return 0, false
	}
	i, err := strconv.ParseInt(n.String(), 10, 64)
	if err != nil || i > 1<<53-1 || i < -(1<<53-1) {
		return 0, false
	}
	return i, true
}

// playbackEncode writes a value as compact JSON, without HTML escaping.
func playbackEncode(v any) []byte {
	var buf bytes.Buffer
	e := json.NewEncoder(&buf)
	e.SetEscapeHTML(false)
	if err := e.Encode(v); err != nil {
		panic(err)
	}
	return bytes.TrimSuffix(buf.Bytes(), []byte("\n"))
}

// The language's own steps of a transcode (playing section 2.7): copy
// and the range are the language's, not registry tokens.
const playbackCopyStep = "copy"

var playbackRanges = []string{"sdr", "keep"}

// playbackTargets is a transcode object as read (playing section 2.7,
// MSFP Media video section 5.1); the output gives the baseline's values for targets
// left out (maxHeight left out: the copy's own height), and drops those
// a copied stream, or a copy with no video, doesn't take.
type playbackTargets struct {
	VideoCodec     string `json:"videoCodec,omitempty"`
	MaxHeight      int64  `json:"maxHeight,omitempty"`
	MaxBitrateKbps int64  `json:"maxBitrateKbps,omitempty"`
	AudioCodec     string `json:"audioCodec,omitempty"`
	AudioChannels  int64  `json:"audioChannels,omitempty"`
	Range          string `json:"range,omitempty"`
	BurnSubtitle   string `json:"burnSubtitle,omitempty"`
	burn           any    // burnSubtitle as received, checked at step 4
	hasBurn        bool
}

// playbackReadTargets reads a transcode object at step 1: codecs and the
// range must be tokens (an unknown one is refused at step 7, 409
// target-refused),
// numbers of their steps. burnSubtitle is checked at step 4.
func playbackReadTargets(v any) (playbackTargets, error) {
	var t playbackTargets
	o, ok := v.(map[string]any)
	if !ok {
		return t, pbBad("transcode isn't an object")
	}
	str := func(name string, dst *string) error {
		raw, present := o[name]
		if !present {
			return nil
		}
		s, ok := raw.(string)
		if !ok || !openToken(s) {
			return pbBad("transcode.%s isn't a token (MSFP Media base section 3)", name)
		}
		*dst = s
		return nil
	}
	for _, f := range []struct {
		name string
		dst  *string
	}{{"videoCodec", &t.VideoCodec}, {"audioCodec", &t.AudioCodec}, {"range", &t.Range}} {
		if err := str(f.name, f.dst); err != nil {
			return t, err
		}
	}
	num := func(name string, ok func(int64) bool, dst *int64) error {
		raw, present := o[name]
		if !present {
			return nil
		}
		n, isInt := playbackInt(raw)
		if !isInt || !ok(n) {
			return pbBad("transcode.%s isn't one of its values", name)
		}
		*dst = n
		return nil
	}
	if err := num("maxHeight", func(n int64) bool { return slices.Contains(playbackHeights, n) }, &t.MaxHeight); err != nil {
		return t, err
	}
	if err := num("maxBitrateKbps", func(n int64) bool { return n >= 1000 && n <= playbackMaxKbps && n%1000 == 0 }, &t.MaxBitrateKbps); err != nil {
		return t, err
	}
	if err := num("audioChannels", func(n int64) bool { return slices.Contains(playbackChannelSteps, n) }, &t.AudioChannels); err != nil {
		return t, err
	}
	t.burn, t.hasBurn = o["burnSubtitle"]
	return t, nil
}

// The owning server's context: its copies, the encoders it has, the
// pairing's limits, what is in use.

// playbackColour is a stream's colour as H.273 code points (MSFP Media
// MSFP Media video section 3.3); 0 for a member left out.
type playbackColour struct {
	Primaries int64 `json:"primaries,omitempty"`
	Transfer  int64 `json:"transfer,omitempty"`
	Matrix    int64 `json:"matrix,omitempty"`
}

type playbackVideoInfo struct {
	Codec      string          `json:"codec"`
	Codecs     string          `json:"codecs,omitempty"`
	Height     int64           `json:"height"`
	BitDepth   int64           `json:"bitDepth,omitempty"`
	Interlaced bool            `json:"interlaced,omitempty"`
	Colour     *playbackColour `json:"colour,omitempty"`
}

type playbackTrackInfo struct {
	ID         string `json:"id"`
	Codec      string `json:"codec"`
	Codecs     string `json:"codecs,omitempty"`
	Channels   int64  `json:"channels,omitempty"`
	SampleRate int64  `json:"sampleRate,omitempty"`
	Default    bool   `json:"default,omitempty"`
}

type playbackCopy struct {
	ID         string              `json:"id"`
	Parts      int64               `json:"parts"`
	Etag       string              `json:"etag"`
	Size       int64               `json:"size"`
	DurationMs int64               `json:"durationMs"`
	Video      *playbackVideoInfo  `json:"video,omitempty"`
	Audio      []playbackTrackInfo `json:"audio"`
	Subtitles  []string            `json:"subtitles,omitempty"`
}

type playbackLimits struct {
	Streams         int64 `json:"streams"`
	Transcodes      int64 `json:"transcodes"`
	BitrateKbps     int64 `json:"bitrateKbps"`
	Downloads       int64 `json:"downloads"`
	DownloadsPerDay int64 `json:"downloadsPerDay"`
	DownloadKbps    int64 `json:"downloadKbps"`
	ScansPerDay     int64 `json:"scansPerDay"`
}

type playbackUse struct {
	Streams              int64          `json:"streams"`
	Transcodes           int64          `json:"transcodes"`
	Downloads            int64          `json:"downloads"`
	DownloadsToday       int64          `json:"downloadsToday"`
	ScansToday           int64          `json:"scansToday"`
	ServerTranscodesFree int64          `json:"serverTranscodesFree"`
	Live                 []playbackLive `json:"live,omitempty"`
}

type playbackContext struct {
	Copies        map[string][]playbackCopy `json:"copies"`
	VideoEncoders []string                  `json:"videoEncoders"`
	AudioEncoders []string                  `json:"audioEncoders"`
	Ranges        []string                  `json:"ranges"`
	SegmentCodecs map[string][]string       `json:"segmentCodecs"`
	OwnCapKbps    int64                     `json:"ownCapKbps"`
	ScanBytes     int64                     `json:"scanBytes"`
	Progress      int64                     `json:"progressSeconds"`
	Gone          []string                  `json:"gone"`
	Unreachable   []string                  `json:"unreachable"`
	Limits        playbackLimits            `json:"limits"`
	InUse         playbackUse               `json:"inUse"`
	StreamRoot    string                    `json:"streamRoot"`
	Now           int64                     `json:"now"`
}

// playbackBaselineOf is what the baseline serves of a stream: copy when
// it is already within the baseline, else the baseline's codec.
type playbackBaselineOf struct {
	Video  string `json:"video,omitempty"`
	Audio  string `json:"audio"`
	Method string `json:"method"`
}

// playbackStarted is what the owning server starts for a play it accepts.
type playbackStarted struct {
	Title           string              `json:"title"`
	Purpose         string              `json:"purpose"`
	Delivery        string              `json:"delivery,omitempty"`
	Baseline        *playbackBaselineOf `json:"baseline,omitempty"`
	MediaType       string              `json:"mediaType,omitempty"`
	Transcode       *playbackTargets    `json:"transcode,omitempty"`
	AudioRenditions bool                `json:"audioRenditions,omitempty"`
	VersionID       string              `json:"versionId"`
	AudioTrack      string              `json:"audioTrack"`
	StartPositionMs int64               `json:"startPositionMs"`
	BitrateCapKbps  int64               `json:"bitrateCapKbps,omitempty"`
	SpeedCapKbps    int64               `json:"speedCapKbps,omitempty"`
	Counts          string              `json:"counts"`
	Transcodes      string              `json:"transcodes,omitempty"`
	TicketSeconds   int64               `json:"ticketSeconds,omitempty"`
	ProgressSeconds int64               `json:"progressSeconds,omitempty"`
	IdleSeconds     int64               `json:"idleSeconds,omitempty"`
	ScanBytes       int64               `json:"scanBytes,omitempty"`
	Replaces        string              `json:"replaces,omitempty"`
}

// playbackLive is one of the caller's live playbacks, which the case's
// inUse counts: its ID, its purpose, and whether it holds a transcode
// (of the pairing's and of the server's budget; a download's, of the
// budget only). A play that replaces it is counted as if it had ended
// (playing sections 2.2 step 8 and 2.3; MSFP playback section 6).
type playbackLive struct {
	ID        string `json:"id"`
	Purpose   string `json:"purpose"`
	Transcode bool   `json:"transcode,omitempty"`
}

// playbackPickCopy is the copy versionId names, else the first
// (section 2.6).
func playbackPickCopy(copies []playbackCopy, versionID string) playbackCopy {
	for _, c := range copies {
		if c.ID == versionID {
			return c
		}
	}
	return copies[0]
}

// playbackPickAudio is the track audioTrack names, else the file's
// default audio: the track it marks default, else its first.
func playbackPickAudio(c playbackCopy, track string) (playbackTrackInfo, bool) {
	for _, a := range c.Audio {
		if a.ID == track {
			return a, true
		}
	}
	for _, a := range c.Audio {
		if a.Default {
			return a, true
		}
	}
	if len(c.Audio) > 0 {
		return c.Audio[0], true
	}
	return playbackTrackInfo{}, false
}

// playbackAVCWithin: an RFC 6381 avc1 or avc3 string of a profile up to
// High (Baseline 66, Main 77, High 100) and a level up to 4.1.
func playbackAVCWithin(codecs string) bool {
	fourcc, hex, ok := strings.Cut(codecs, ".")
	if !ok || fourcc != "avc1" && fourcc != "avc3" || len(hex) != 6 {
		return false
	}
	b, err := strconv.ParseUint(hex, 16, 32)
	if err != nil {
		return false
	}
	profile, level := b>>16, b&0xff
	return (profile == 66 || profile == 77 || profile == 100) && level >= 10 && level <= 41
}

// playbackVideoWithin: the baseline's picture (section 2.7): H.264 up to
// High and level 4.1, 8-bit 4:2:0, progressive, SDR in BT.709 (H.273
// primaries, transfer and matrix 1). A fact the copy doesn't state isn't
// known to be within.
func playbackVideoWithin(v playbackVideoInfo) bool {
	c := v.Colour
	return v.Codec == "h264" && playbackAVCWithin(v.Codecs) && v.BitDepth == 8 && !v.Interlaced &&
		c != nil && c.Primaries == 1 && c.Transfer == 1 && c.Matrix == 1
}

// playbackAudioWithin: AAC-LC, at most 2 channels, at most 48 kHz.
func playbackAudioWithin(a playbackTrackInfo) bool {
	return a.Codec == "aac" && a.Codecs == "mp4a.40.2" && a.Channels >= 1 && a.Channels <= 2 && a.SampleRate >= 1 && a.SampleRate <= 48000
}

// playbackCarried: whether a segment format (mp2t or mp4) carries the
// codecs of the copied streams.
func playbackCarried(ctx playbackContext, sub string, codecs ...string) bool {
	for _, c := range codecs {
		if !slices.Contains(ctx.SegmentCodecs[sub], c) {
			return false
		}
	}
	return true
}

// playbackAllowed is section 2.7's table of purpose and a delivery of the
// list ("" for none).
func playbackAllowed(purpose, delivery string, t *playbackTargets) bool {
	switch purpose {
	case "play":
		return true
	case "download":
		return delivery == "file" || delivery == "transcode" && t != nil && (t.MaxHeight != 0 || t.MaxBitrateKbps != 0)
	case "scan":
		return delivery == "file"
	}
	return false
}

// playbackRequestMembers are the members of a play request playing and
// video define (but capabilities, which MSFP refuses whatever it holds);
// any other is unknown, and ignored whatever it holds.
var playbackRequestMembers = []string{"title", "purpose", "delivery", "mediaType", "transcode", "audioRenditions", "startPositionMs", "maxBitrateKbps", "versionId", "audioTrack", "replaces"}

// playbackPlay applies playing section 2.2, steps 1 to 9, to a play
// request, as MSFP carries it.
func playbackPlay(ctx playbackContext, lim playbackLimits, use playbackUse, raw []byte) ([]byte, error) {
	o, err := playbackDecode(raw)
	if err != nil {
		return nil, pbBad("%v", err)
	}
	s, err := playbackStart(ctx, lim, use, o)
	if err != nil {
		return nil, err
	}
	return playbackEncode(s), nil
}

func playbackStart(ctx playbackContext, lim playbackLimits, use playbackUse, o map[string]any) (playbackStarted, error) {
	var s playbackStarted
	// Step 1: forms, types, ranges, combinations. MSFP takes no capability
	// list (MSFP playback section 2; playing section 2.8): refused,
	// whatever it holds.
	if _, present := o["capabilities"]; present {
		return s, pbBad("capabilities: MSFP takes no capability list")
	}
	for _, name := range playbackRequestMembers {
		if v, present := o[name]; present && v == nil {
			return s, pbBad("%s is null", name)
		}
	}
	text := func(name string) (string, bool, error) {
		raw, present := o[name]
		if !present {
			return "", false, nil
		}
		s, ok := raw.(string)
		if !ok {
			return "", true, pbBad("%s isn't a string", name)
		}
		return s, true, nil
	}
	title, present, err := text("title")
	if err != nil {
		return s, err
	}
	if !present {
		return s, pbBad("no title")
	}
	kind, part, err := playbackTitleKind(title)
	if err != nil {
		return s, pbBad("title: %v", err)
	}
	if part == "" {
		return s, pbBad("title: play doesn't take a %s", kind)
	}
	s = playbackStarted{Title: title, Purpose: "play"}
	if p, present, err := text("purpose"); err != nil {
		return s, err
	} else if present {
		if !slices.Contains(playbackPurposes, p) {
			return s, pbBad("purpose %q isn't play, download or scan", p)
		}
		s.Purpose = p
	}
	unknownDelivery := false
	if d, present, err := text("delivery"); err != nil {
		return s, err
	} else if present {
		if !openToken(d) {
			return s, pbBad("delivery %q isn't a token (MSFP Media base section 3)", d)
		}
		unknownDelivery = !slices.Contains(playbackDeliveries, d)
		s.Delivery = d
	}
	if m, present, err := text("mediaType"); err != nil {
		return s, err
	} else if present {
		if s.Delivery != "remux" {
			return s, pbBad("mediaType without delivery remux")
		}
		if !openMediaType(m) {
			return s, pbBad("mediaType %q isn't a media type (MSFP Media base section 3)", m)
		}
		s.MediaType = m
	} else if s.Delivery == "remux" {
		return s, pbBad("delivery remux without mediaType")
	}
	if v, present := o["transcode"]; present {
		if s.Delivery != "transcode" {
			return s, pbBad("a transcode object without delivery transcode")
		}
		t, err := playbackReadTargets(v)
		if err != nil {
			return s, err
		}
		s.Transcode = &t
	} else if s.Delivery == "transcode" {
		s.Transcode = &playbackTargets{}
	}
	if v, present := o["audioRenditions"]; present {
		b, ok := v.(bool)
		if !ok {
			return s, pbBad("audioRenditions isn't a boolean")
		}
		s.AudioRenditions = b
	}
	intIn := func(name string, max int64) (int64, error) {
		v, present := o[name]
		if !present {
			return 0, nil
		}
		n, ok := playbackInt(v)
		if !ok {
			return 0, pbBad("%s isn't an integer", name)
		}
		if n < 0 || n > max {
			return 0, pbBad("%s out of 0 to %d", name, max)
		}
		return n, nil
	}
	if s.StartPositionMs, err = intIn("startPositionMs", playbackMaxPositionMs); err != nil {
		return s, err
	}
	reqCap, err := intIn("maxBitrateKbps", playbackMaxKbps)
	if err != nil {
		return s, err
	}
	versionID, _, err := text("versionId")
	if err != nil {
		return s, err
	}
	audioTrack, hasAudio, err := text("audioTrack")
	if err != nil {
		return s, err
	}
	replaces, hasReplaces, err := text("replaces")
	if err != nil {
		return s, err
	}
	if hasReplaces && !playbackIDForm.MatchString(replaces) {
		return s, pbBad("replaces isn't of a playback ID's form")
	}
	// An unknown delivery is answered at step 7, whatever the purpose.
	if !unknownDelivery && !playbackAllowed(s.Purpose, s.Delivery, s.Transcode) {
		d := s.Delivery
		if d == "" {
			d = "none"
		}
		return s, pbBad("purpose %s with delivery %s isn't allowed", s.Purpose, d)
	}
	// Step 2: the purpose's limit is 0.
	switch {
	case s.Purpose == "play" && lim.Streams == 0:
		return s, pbRefuse(409, "stream-limit-reached", "streams is 0")
	case s.Purpose == "download" && lim.Downloads == 0:
		return s, pbRefuse(409, "stream-limit-reached", "downloads is 0")
	case s.Purpose == "scan" && lim.ScansPerDay == 0:
		return s, pbRefuse(409, "stream-limit-reached", "scansPerDay is 0")
	}
	// Step 3: copies of that exact key; gone when it held the title for
	// this caller and no longer does.
	copies := ctx.Copies[title]
	if len(copies) == 0 {
		if slices.Contains(ctx.Gone, title) {
			return s, pbRefuse(404, "gone", "the title was offered to this caller, and is no longer held")
		}
		return s, pbRefuse(404, "not-found", "no copy of that key is offered")
	}
	// Step 4: the copy, then malformed track IDs and burnSubtitle.
	c := playbackPickCopy(copies, versionID)
	if hasAudio && !playbackAudioForm.MatchString(audioTrack) {
		return s, pbBad("audioTrack isn't a and a number from 0 to 999")
	}
	if t := s.Transcode; t != nil && t.hasBurn {
		b, ok := t.burn.(string)
		if !ok || !playbackSubForm.MatchString(b) {
			return s, pbBad("transcode.burnSubtitle isn't s and a number from 0 to 999")
		}
		t.BurnSubtitle = b
	}
	s.VersionID = c.ID
	audio, _ := playbackPickAudio(c, audioTrack)
	s.AudioTrack = audio.ID
	// A replaces naming a live playback of this caller: the limits are
	// checked as if it had ended (steps 5 and 8); one naming none is
	// ignored.
	if hasReplaces {
		for _, old := range use.Live {
			if old.ID != replaces {
				continue
			}
			s.Replaces = old.ID
			switch old.Purpose {
			case "play":
				use.Streams--
				if old.Transcode {
					use.Transcodes--
				}
			case "download":
				use.Downloads-- // the day's count stays
			}
			if old.Transcode {
				use.ServerTranscodesFree++
			}
		}
	}
	// The interval the answer states, and the idle limit, twice it.
	progress := ctx.Progress
	if progress < playbackProgressLow || progress > playbackProgressHigh {
		progress = playbackProgressDef
	}
	// Step 5: the purpose's limits in use.
	switch s.Purpose {
	case "play":
		if use.Streams >= lim.Streams {
			return s, pbRefuse(409, "stream-limit-reached", "the pairing's streams are all in use")
		}
		s.Counts, s.ProgressSeconds, s.IdleSeconds = "streams", progress, 2*progress
	case "download":
		if use.Downloads >= lim.Downloads {
			return s, pbRefuse(409, "stream-limit-reached", "the pairing's downloads are all in use")
		}
		if lim.DownloadsPerDay > 0 && use.DownloadsToday >= lim.DownloadsPerDay {
			return s, pbRefuse(409, "stream-limit-reached", "downloadsPerDay reached")
		}
		s.Counts, s.TicketSeconds, s.SpeedCapKbps = "downloads", playbackDownloadTTL, lim.DownloadKbps
		s.StartPositionMs = 0 // served from its beginning
	case "scan":
		if use.ScansToday >= lim.ScansPerDay {
			return s, pbRefuse(409, "stream-limit-reached", "scansPerDay reached")
		}
		s.Counts, s.TicketSeconds, s.ProgressSeconds, s.IdleSeconds, s.ScanBytes = "scansPerDay", playbackScanTTL, progress, 2*progress, ctx.ScanBytes
	}
	// Step 6: the bitrate cap, for play only; with none, no cap.
	if s.Purpose == "play" {
		caps := []int64{reqCap, ctx.OwnCapKbps, lim.BitrateKbps}
		if s.Transcode != nil {
			caps = append(caps, s.Transcode.MaxBitrateKbps)
		}
		for _, v := range caps {
			if v > 0 && (s.BitrateCapKbps == 0 || v < s.BitrateCapKbps) {
				s.BitrateCapKbps = v
			}
		}
	}
	// Step 7: the delivery asked, or a refusal: delivery-refused for the
	// delivery as such, target-refused for a target it doesn't make.
	hasVideo := c.Video != nil
	refuse := func(why string) (playbackStarted, error) {
		return s, pbRefuse(409, "delivery-refused", why)
	}
	target := func(why string) (playbackStarted, error) {
		return s, pbRefuse(409, "target-refused", why)
	}
	if unknownDelivery {
		return refuse("a delivery the owning server doesn't make")
	}
	if s.Delivery == "remux" {
		top, sub, _ := strings.Cut(s.MediaType, "/")
		want := "video"
		if !hasVideo {
			want = "audio"
		}
		if sub != "mp2t" && sub != "mp4" {
			return target("a remux to a media type of another subtype than mp2t or mp4")
		}
		if top != want {
			return target("a remux to " + top + "/" + sub + " of a copy that calls for " + want + "/")
		}
		streams := []string{audio.Codec}
		if hasVideo {
			streams = append(streams, c.Video.Codec)
		}
		if !playbackCarried(ctx, sub, streams...) {
			return target("a remux to a segment format that can't carry the copy's streams")
		}
	}
	videoCopy, audioCopy := false, false
	if t := s.Transcode; t != nil {
		if hasVideo && t.VideoCodec == playbackCopyStep {
			videoCopy = true
		} else if hasVideo && t.VideoCodec != "" && !slices.Contains(ctx.VideoEncoders, t.VideoCodec) {
			return target("a video codec the owning server doesn't encode")
		}
		if t.AudioCodec == playbackCopyStep {
			audioCopy = true
		} else if t.AudioCodec != "" && !slices.Contains(ctx.AudioEncoders, t.AudioCodec) {
			return target("an audio codec the owning server doesn't encode")
		}
		var copied []string
		if videoCopy {
			copied = append(copied, c.Video.Codec)
		}
		if audioCopy {
			copied = append(copied, audio.Codec)
		}
		if len(copied) > 0 && !playbackCarried(ctx, "mp2t", copied...) && !playbackCarried(ctx, "mp4", copied...) {
			return target("a copied stream no segment format can carry")
		}
		if videoCopy && t.BurnSubtitle != "" {
			return target("burnSubtitle with video copy")
		}
		if hasVideo && !videoCopy && t.Range != "" {
			if !slices.Contains(playbackRanges, t.Range) {
				return target("a range other than sdr or keep")
			}
			if !slices.Contains(ctx.Ranges, t.Range) {
				return target("a range the owning server can't make")
			}
		}
	}
	if s.Delivery == "file" && c.Parts > 1 {
		return refuse("a copy of several parts is never delivered as file")
	}
	// The copy's overall rate, size × 8 / durationMs kbit/s, above the cap.
	if (s.Delivery == "file" || s.Delivery == "remux") && s.Purpose == "play" && s.BitrateCapKbps > 0 && c.Size*8 > s.BitrateCapKbps*c.DurationMs {
		return refuse("the copy's overall rate is above the cap")
	}
	if s.Transcode != nil && s.Transcode.BurnSubtitle != "" && !slices.Contains(c.Subtitles, s.Transcode.BurnSubtitle) {
		return target("burnSubtitle isn't a subtitle track of the copy")
	}
	// Targets left out take the baseline's values (maxHeight: the copy's
	// own height, left out); a copied stream, and a copy with no video,
	// take none of the targets that would change them.
	needsTranscode := false
	if t := s.Transcode; t != nil {
		switch {
		case !hasVideo:
			t.VideoCodec, t.MaxHeight, t.Range = "", 0, "" // ignored for a copy with no video
		case videoCopy:
			t.MaxHeight, t.Range = 0, "" // a copied picture takes none of them
		default:
			if t.VideoCodec == "" {
				t.VideoCodec = playbackBaselineVideo
			}
			if t.Range == "" {
				t.Range = "sdr"
			}
		}
		if audioCopy {
			t.AudioChannels = 0
		} else {
			if t.AudioCodec == "" {
				t.AudioCodec = playbackBaselineAudio
			}
			if t.AudioChannels == 0 {
				t.AudioChannels = playbackBaselineChannels
			}
		}
		// A transcode whose video is copy counts as one only when its
		// audio is converted.
		needsTranscode = !(videoCopy && audioCopy)
	}
	// Without delivery, the baseline: a stream already within it is left
	// unchanged.
	if s.Delivery == "" {
		b := &playbackBaselineOf{Audio: playbackBaselineAudio, Method: "remux"}
		if playbackAudioWithin(audio) {
			b.Audio = playbackCopyStep
		}
		if hasVideo {
			b.Video = playbackBaselineVideo
			if playbackVideoWithin(*c.Video) {
				b.Video = playbackCopyStep
			}
		}
		if b.Audio != playbackCopyStep || hasVideo && b.Video != playbackCopyStep {
			b.Method, needsTranscode = "transcode", true
		}
		s.Baseline = b
	}
	// Step 8: the media reached, then transcodes.
	if slices.Contains(ctx.Unreachable, title) {
		return s, pbRefuse(503, "unreachable", "the copy's media can't be reached now")
	}
	if needsTranscode {
		if s.Purpose == "play" && use.Transcodes >= lim.Transcodes {
			return s, pbRefuse(503, "server-busy", "the pairing's transcodes are all in use, or 0")
		}
		if use.ServerTranscodesFree <= 0 {
			return s, pbRefuse(503, "server-busy", "the server's transcode budget is used")
		}
		s.Transcodes = "pairing and server"
		if s.Purpose == "download" {
			s.Transcodes = "server"
		}
	}
	return s, nil
}

// Queue entries (playing section 2.1, MSFP Media music section 5.2): an entry may
// name its own copy and delivery.

// playbackQueueEntryMembers are what an entry may carry of a request.
var playbackQueueEntryMembers = []string{"title", "versionId", "delivery", "mediaType", "transcode"}

// playbackQueueEntry is what the owning server starts for one entry of a
// request's queue: the entry's title and versionId (without one, its
// first copy), its own delivery with its own mediaType or transcode, or
// without one the request's. The rest (purpose, the caps, audio
// renditions) is the request's; the entry starts at 0, on its default
// audio. A refusal is the entry's alone; what music does with it is
// music's.
func playbackQueueEntry(ctx playbackContext, lim playbackLimits, use playbackUse, request, entry []byte) ([]byte, error) {
	r, err := playbackDecode(request)
	if err != nil {
		return nil, pbBad("the request: %v", err)
	}
	e, err := playbackDecode(entry)
	if err != nil {
		return nil, pbBad("the entry: %v", err)
	}
	o := map[string]any{}
	for _, k := range []string{"purpose", "audioRenditions", "maxBitrateKbps"} {
		if v, ok := r[k]; ok {
			o[k] = v
		}
	}
	if _, own := e["delivery"]; !own {
		for _, k := range []string{"delivery", "mediaType", "transcode"} {
			if v, ok := r[k]; ok {
				o[k] = v
			}
		}
	}
	for _, k := range playbackQueueEntryMembers {
		if v, ok := e[k]; ok {
			o[k] = v
		}
	}
	if _, ok := e["title"]; !ok {
		return nil, pbBad("an entry without title")
	}
	s, err := playbackStart(ctx, lim, use, o)
	if err != nil {
		return nil, err
	}
	s.Counts, s.ProgressSeconds, s.IdleSeconds, s.Replaces = "", 0, 0, "" // the playback counts once, under the request
	return playbackEncode(s), nil
}

// The answer (playing section 2.3, MSFP Media video section 5.3), as a
// viewer's server keeps it.

// playbackSubtitle is an entry of the answer's subtitles (MSFP Media video
// section 5.3).
type playbackSubtitle struct {
	ID   string `json:"id"`
	Form string `json:"form"`
	ASS  bool   `json:"ass,omitempty"`
}

type playbackAnswer struct {
	ID              string             `json:"id"`
	Ticket          string             `json:"ticket"`
	Protocol        string             `json:"protocol"`
	Method          string             `json:"method"`
	Etag            string             `json:"etag"`
	PositionMs      int64              `json:"positionMs"`
	VersionID       string             `json:"versionId"`
	AudioTrack      string             `json:"audioTrack,omitempty"`
	AudioRenditions bool               `json:"audioRenditions,omitempty"`
	MaxBitrateKbps  *int64             `json:"maxBitrateKbps,omitempty"`
	ProgressSeconds int64              `json:"progressSeconds"`
	ScanBytes       int64              `json:"scanBytes,omitempty"`
	Subtitles       []playbackSubtitle `json:"subtitles,omitempty"`
	FontCount       int64              `json:"fontCount,omitempty"`
	URLs            []string           `json:"urls"`
	ReadTitleAgain  bool               `json:"readTitleAgain,omitempty"`
}

// playbackAsked is what a viewer's server asked and knows of the copy
// when it reads an answer: the request's purpose and delivery, the
// title's content part, the copy's etag and subtitle tracks in the
// catalog.
type playbackAsked struct {
	Purpose          string   `json:"purpose,omitempty"`
	Delivery         string   `json:"delivery,omitempty"`
	Part             string   `json:"part"`
	CatalogEtag      string   `json:"catalogEtag"`
	CatalogSubtitles []string `json:"catalogSubtitles,omitempty"`
}

// playbackKeepAnswer checks and cleans a play answer, and builds the URLs
// it opens from the stream root and the ticket (fetching section 1). An
// error means the viewer's server stops that playback and gives up.
func playbackKeepAnswer(raw []byte, ask playbackAsked, root string) ([]byte, error) {
	o, err := playbackDecode(raw)
	if err != nil {
		return nil, err
	}
	str := func(name string) (string, error) {
		v, present := o[name]
		if !present {
			return "", fmt.Errorf("no %s", name)
		}
		s, ok := v.(string)
		if !ok {
			return "", fmt.Errorf("%s isn't a string", name)
		}
		return s, nil
	}
	formed := func(name string, form *regexp.Regexp) (string, error) {
		s, err := str(name)
		if err != nil {
			return "", err
		}
		if !form.MatchString(s) {
			return "", fmt.Errorf("%s doesn't match ^[A-Za-z0-9_-]{1,64}$", name)
		}
		return s, nil
	}
	var a playbackAnswer
	if a.ID, err = formed("id", playbackIDForm); err != nil {
		return nil, err
	}
	if a.Ticket, err = formed("ticket", playbackIDForm); err != nil {
		return nil, err
	}
	if a.Protocol, err = str("protocol"); err != nil {
		return nil, err
	}
	if a.Protocol != "hls" && a.Protocol != "file" {
		return nil, fmt.Errorf("protocol %q isn't hls or file", a.Protocol)
	}
	if want := map[string]string{"": "hls", "file": "file", "remux": "hls", "transcode": "hls"}[ask.Delivery]; a.Protocol != want {
		return nil, fmt.Errorf("protocol %s for delivery %q: %s only", a.Protocol, ask.Delivery, want)
	}
	if a.Method, err = str("method"); err != nil {
		return nil, err
	}
	if !slices.Contains([]string{"direct", "remux", "transcode"}, a.Method) {
		return nil, fmt.Errorf("method %q", a.Method)
	}
	if want := map[string]string{"file": "direct", "remux": "remux", "transcode": "transcode"}[ask.Delivery]; want != "" && a.Method != want {
		return nil, fmt.Errorf("method %s for delivery %s", a.Method, ask.Delivery)
	}
	if a.Etag, err = str("etag"); err != nil {
		return nil, err
	}
	if !playbackEtagForm.MatchString(a.Etag) {
		return nil, errors.New("etag isn't 1 to 64 characters of A-Z a-z 0-9 - _")
	}
	pos, present := o["positionMs"]
	if !present {
		return nil, errors.New("no positionMs")
	}
	var ok bool
	if a.PositionMs, ok = playbackInt(pos); !ok {
		return nil, errors.New("positionMs isn't an integer")
	}
	if a.PositionMs < 0 || a.PositionMs > playbackMaxPositionMs {
		return nil, errors.New("positionMs out of 0 to 100 days")
	}
	if a.VersionID, err = formed("versionId", playbackObjIDForm); err != nil {
		return nil, err
	}
	if v, present := o["audioTrack"]; present {
		t, ok := v.(string)
		if !ok {
			return nil, errors.New("audioTrack isn't a string")
		}
		if playbackAudioForm.MatchString(t) {
			a.AudioTrack = t
		}
	}
	if v, present := o["audioRenditions"]; present {
		b, ok := v.(bool)
		if !ok {
			return nil, errors.New("audioRenditions isn't a boolean")
		}
		a.AudioRenditions = b && a.Protocol == "hls"
	}
	if v, present := o["maxBitrateKbps"]; present {
		n, ok := playbackInt(v)
		if !ok {
			return nil, errors.New("maxBitrateKbps isn't an integer")
		}
		if n >= 0 && n <= playbackMaxKbps {
			a.MaxBitrateKbps = &n
		}
	}
	// The interval of progress reports: 45 to 120, else 60 (section 3).
	a.ProgressSeconds = playbackProgressDef
	if n, ok := playbackInt(o["progressSeconds"]); ok && n >= playbackProgressLow && n <= playbackProgressHigh {
		a.ProgressSeconds = n
	}
	if ask.Purpose == "scan" {
		a.ScanBytes = playbackScanFloor
		if n, ok := playbackInt(o["scanBytes"]); ok && n >= playbackScanFloor && n <= playbackScanTop {
			a.ScanBytes = n
		}
	}
	base := root + a.Ticket + "/"
	stream := map[string]string{"hls": "master.m3u8", "file": "file"}[a.Protocol]
	a.URLs = []string{base + stream}
	// Video's members; ignored for a title of another part (section 2.4).
	if ask.Part == "video" {
		if list, ok := o["subtitles"].([]any); ok {
			seen := map[string]bool{}
			for _, e := range list {
				eo, ok := e.(map[string]any)
				if !ok {
					continue
				}
				id, _ := eo["id"].(string)
				form, _ := eo["form"].(string)
				if !slices.Contains(ask.CatalogSubtitles, id) || seen[id] || form != "text" && form != "picture" {
					continue
				}
				seen[id] = true
				sub := playbackSubtitle{ID: id, Form: form}
				if b, ok := eo["ass"].(bool); ok && b && form == "text" {
					sub.ASS = true
				}
				a.Subtitles = append(a.Subtitles, sub)
			}
		}
		if n, ok := playbackInt(o["fontCount"]); ok && n >= 0 && n <= 64 {
			a.FontCount = n
		}
		for _, sub := range a.Subtitles {
			if sub.Form == "picture" {
				a.URLs = append(a.URLs, base+"pictures/"+sub.ID)
				continue
			}
			a.URLs = append(a.URLs, base+"subtitles/"+sub.ID)
			if sub.ASS {
				a.URLs = append(a.URLs, base+"ass/"+sub.ID)
			}
		}
		for n := int64(0); n < a.FontCount; n++ {
			a.URLs = append(a.URLs, base+"fonts/"+strconv.FormatInt(n, 10))
		}
	}
	a.ReadTitleAgain = ask.CatalogEtag != "" && a.Etag != ask.CatalogEtag
	return playbackEncode(a), nil
}

// Refusals of play, as a viewer's server reads them (section 2.5, MSFP
// Media base section 4, MSFP playback section 2).

// playbackRefusalRead is what a viewer's server does with a play
// refused: what it tells the viewer ("asks again" with another delivery
// or other targets, "not playable", "busy now"), the entries of instead
// it keeps, the retryAfter it keeps, and whether it drops the title and
// reads the owning server's titles again (gone).
type playbackRefusalRead struct {
	Tells      string           `json:"tells"`
	Instead    []map[string]any `json:"instead,omitempty"`
	RetryAfter int64            `json:"retryAfter,omitempty"`
	DropTitle  bool             `json:"dropTitle,omitempty"`
}

// playbackInsteadMax is how many entries of instead a caller reads.
const playbackInsteadMax = 4

// playbackReadRefusal reads a play refused with status (signed: whether
// MSFP vouches for it, a signed answer) and body, for a request of
// purpose. An unsigned refusal, a 429 and a 503 mean busy now; a 409 is
// told by its type; any other refusal of the language means not
// playable, gone besides dropping the title.
func playbackReadRefusal(status int, signed bool, body []byte, purpose string) playbackRefusalRead {
	busy := playbackRefusalRead{Tells: "busy now"}
	if !signed {
		return busy
	}
	o, _ := playbackDecode(body) // a body that isn't JSON counts as one without a type
	typ, _ := o["type"].(string)
	retry := int64(0)
	if n, ok := playbackInt(o["retryAfter"]); ok && n >= 1 && n <= 3600 {
		retry = n
	}
	switch status {
	case 429, 503:
		busy.RetryAfter = retry
		return busy
	case 409:
		switch typ {
		case "delivery-refused", "target-refused":
			return playbackRefusalRead{Tells: "asks again", Instead: playbackKeepInstead(o["instead"], purpose)}
		case "not-playable":
			return playbackRefusalRead{Tells: "not playable"}
		}
		busy.RetryAfter = retry
		return busy
	}
	return playbackRefusalRead{Tells: "not playable", DropTitle: status == 404 && typ == "gone"}
}

// playbackKeepInstead keeps the entries of instead a caller may ask for:
// of the first 4, each object whose delivery, mediaType and transcode
// keep the forms of a play request of that purpose (section 2.2, step 1;
// burnSubtitle's form of step 4), with only those members. Not an
// array: none.
func playbackKeepInstead(v any, purpose string) []map[string]any {
	list, ok := v.([]any)
	if !ok {
		return nil
	}
	if purpose == "" {
		purpose = "play"
	}
	var out []map[string]any
	for i, e := range list {
		if i >= playbackInsteadMax {
			break
		}
		eo, ok := e.(map[string]any)
		if !ok {
			continue
		}
		d, ok := eo["delivery"].(string)
		if !ok || !openToken(d) {
			continue
		}
		keep := map[string]any{"delivery": d}
		m, hasM := eo["mediaType"]
		if hasM != (d == "remux") {
			continue
		}
		if hasM {
			if ms, ok := m.(string); !ok || !openMediaType(ms) {
				continue
			}
			keep["mediaType"] = m
		}
		var targets *playbackTargets
		if t, hasT := eo["transcode"]; hasT {
			if d != "transcode" {
				continue
			}
			read, err := playbackReadTargets(t)
			if err != nil {
				continue
			}
			if read.hasBurn {
				if b, ok := read.burn.(string); !ok || !playbackSubForm.MatchString(b) {
					continue
				}
			}
			targets = &read
			keep["transcode"] = t
		} else if d == "transcode" {
			targets = &playbackTargets{}
		}
		if slices.Contains(playbackDeliveries, d) && !playbackAllowed(purpose, d, targets) {
			continue
		}
		out = append(out, keep)
	}
	return out
}

// Restarts (playing section 2.7).

type playbackReading struct {
	VersionID string `json:"versionId"`
	Etag      string `json:"etag"`
	Purpose   string `json:"purpose"`
	Delivery  string `json:"delivery"`
	Offset    int64  `json:"offset"`
}

type playbackRestartResult struct {
	Join       bool   `json:"join"`
	Offset     int64  `json:"offset,omitempty"`
	StopNew    bool   `json:"stopNew,omitempty"`
	FailRead   bool   `json:"failRead,omitempty"`
	ReadTitles bool   `json:"readTitles,omitempty"`
	Why        string `json:"-"`
}

// playbackRestart: what a viewer's server does with a new play of a file
// it was reading, given that play's status and answer.
func playbackRestart(r playbackReading, request []byte, status int, answer []byte, root string) playbackRestartResult {
	fail := playbackRestartResult{FailRead: true, ReadTitles: true}
	if r.Delivery != "file" {
		fail.Why = "only a playback of delivery file is restarted"
		return fail
	}
	o, err := playbackDecode(request)
	if err != nil {
		fail.Why = "the new play isn't a JSON object"
		return fail
	}
	purpose, _ := o["purpose"].(string)
	if purpose == "" {
		purpose = "play"
	}
	rp := r.Purpose
	if rp == "" {
		rp = "play"
	}
	if v, _ := o["versionId"].(string); v != r.VersionID || purpose != rp || o["delivery"] != "file" {
		fail.Why = "the new play isn't of the same copy, purpose and delivery"
		return fail
	}
	if status != 200 {
		fail.Why = "the new play was refused"
		return fail
	}
	kept, err := playbackKeepAnswer(answer, playbackAsked{Purpose: rp, Delivery: "file"}, root)
	fail.StopNew = true
	if err != nil {
		fail.Why = "the new answer is refused: " + err.Error()
		return fail
	}
	var a playbackAnswer
	_ = json.Unmarshal(kept, &a)
	if a.Etag != r.Etag {
		fail.Why = "the new etag isn't the one being read"
		return fail
	}
	return playbackRestartResult{Join: true, Offset: r.Offset}
}

// Progress and stop (playing sections 3 to 6).

// playbackState is the owning server's playback a report names.
type playbackState struct {
	State    string   `json:"state"` // started, active, ended; unknown: another pairing's, or never given
	Purpose  string   `json:"purpose,omitempty"`
	Title    string   `json:"title,omitempty"`
	Queue    []string `json:"queue,omitempty"`
	PlayedAt int64    `json:"playedAt,omitempty"`
	LastAt   int64    `json:"lastAt,omitempty"`
}

type playbackReportResult struct {
	Effect     string `json:"effect"`
	State      string `json:"state"`
	Title      string `json:"title,omitempty"`
	PositionMs *int64 `json:"positionMs,omitempty"`
	At         int64  `json:"at,omitempty"`
	Paused     bool   `json:"paused,omitempty"`
}

// playbackReport answers a progress or stop (call) at time now.
func playbackReport(call string, now int64, pb playbackState, raw []byte) ([]byte, error) {
	o, err := playbackDecode(raw)
	if err != nil {
		return nil, pbBad("%v", err)
	}
	for _, name := range []string{"id", "positionMs", "at", "title"} {
		if v, present := o[name]; present && v == nil {
			return nil, pbBad("%s is null", name)
		}
	}
	id, ok := o["id"].(string)
	if !ok || !playbackIDForm.MatchString(id) {
		return nil, pbBad("id missing or not ^[A-Za-z0-9_-]{1,64}$")
	}
	var pos *int64
	if v, present := o["positionMs"]; present {
		n, ok := playbackInt(v)
		if !ok || n < 0 || n > playbackMaxPositionMs {
			return nil, pbBad("positionMs isn't an integer from 0 to 8,640,000,000")
		}
		pos = &n
	}
	at, hasAt := now, false
	if v, present := o["at"]; present {
		n, ok := playbackInt(v)
		if !ok {
			return nil, pbBad("at isn't an integer")
		}
		if n < now-playbackAtBack || n > now+playbackAtAhead {
			return nil, pbBad("at out of 30 days before to 5 minutes after now")
		}
		at, hasAt = n, true
	}
	// paused, a member of progress only (section 3); in stop, unknown.
	paused := false
	if v, present := o["paused"]; present && call == "progress" {
		b, ok := v.(bool)
		if !ok {
			return nil, pbBad("paused isn't a boolean")
		}
		paused = b
	}
	title, hasTitle := "", false
	if v, present := o["title"]; present {
		if title, ok = v.(string); !ok {
			return nil, pbBad("title isn't a string")
		}
		hasTitle = true
	}
	unchanged := func() ([]byte, error) {
		return playbackEncode(playbackReportResult{Effect: "unchanged", State: pb.State}), nil
	}
	if pb.State != "started" && pb.State != "active" && pb.State != "ended" {
		if call == "progress" {
			return nil, pbRefuse(410, "playback-stopped", "not a playback of this pairing")
		}
		return unchanged()
	}
	if hasTitle && title != pb.Title && !slices.Contains(pb.Queue, title) {
		return nil, pbBad("title is neither the playback's title nor one of its queue")
	}
	if !hasTitle {
		title = pb.Title
	}
	rec := playbackReportResult{Title: title, PositionMs: pos, At: at, Paused: paused}
	live := pb.State != "ended"
	if !live {
		late := pb.Purpose == "download" && hasAt && now <= pb.PlayedAt+playbackLateWindow
		if !late {
			if call == "progress" {
				return nil, pbRefuse(410, "playback-stopped", "the playback has ended")
			}
			return unchanged()
		}
		rec.State = "ended"
		rec.Effect = "recorded"
		if pb.LastAt != 0 && at < pb.LastAt {
			return playbackEncode(playbackReportResult{Effect: "ignored", State: "ended"}), nil
		}
		return playbackEncode(rec), nil
	}
	if pb.Purpose == "scan" && call == "progress" {
		return unchanged()
	}
	older := pb.LastAt != 0 && at < pb.LastAt
	if call == "stop" {
		if older || pb.Purpose == "scan" {
			return playbackEncode(playbackReportResult{Effect: "ended", State: "ended"}), nil
		}
		rec.Effect, rec.State = "recorded and ended", "ended"
		return playbackEncode(rec), nil
	}
	if older {
		return playbackEncode(playbackReportResult{Effect: "ignored", State: "active"}), nil
	}
	rec.Effect, rec.State = "recorded", "active"
	return playbackEncode(rec), nil
}

// Limits (MSFP playback section 5, MSFP music section 5.1), as a caller
// keeps them.

type playbackKeptLimits struct {
	Streams         *int64 `json:"streams,omitempty"`
	Transcodes      *int64 `json:"transcodes,omitempty"`
	BitrateKbps     *int64 `json:"bitrateKbps,omitempty"`
	Downloads       int64  `json:"downloads"`
	DownloadsPerDay int64  `json:"downloadsPerDay"`
	DownloadKbps    int64  `json:"downloadKbps"`
	ScansPerDay     int64  `json:"scansPerDay"`
	AudioStreams    *int64 `json:"audioStreams,omitempty"`
	AudioTranscodes *int64 `json:"audioTranscodes,omitempty"`
	OffersDownloads bool   `json:"offersDownloads"`
	OffersScans     bool   `json:"offersScans"`
}

// playbackKeepLimits reads the libraries answer's limits: a limits that
// isn't an object, or one of whose known members isn't an integer, is
// ignored whole (an error); a member out of range is ignored alone.
// scanBytes is no member of limits now (a scan's answer states it):
// unknown, it is ignored whatever it holds.
func playbackKeepLimits(raw []byte) ([]byte, error) {
	o, err := playbackDecode(raw)
	if err != nil {
		return nil, err
	}
	known := []struct {
		name string
		max  int64
		min  int64
	}{{"streams", playbackMaxStreams, 0}, {"transcodes", playbackMaxStreams, 0}, {"bitrateKbps", 1000000, 0}, {"downloads", playbackMaxStreams, 0}, {"downloadsPerDay", 10000, 0}, {"downloadKbps", 1000000, 0}, {"scansPerDay", 1000000, 0}, {"audioStreams", playbackMaxStreams, 0}, {"audioTranscodes", playbackMaxStreams, 0}}
	got := map[string]int64{}
	for _, k := range known {
		v, present := o[k.name]
		if !present {
			continue
		}
		n, ok := playbackInt(v)
		if !ok {
			return nil, fmt.Errorf("%s isn't an integer: limits ignored", k.name)
		}
		if n >= k.min && n <= k.max {
			got[k.name] = n
		}
	}
	// transcodes 0 to streams, audioTranscodes 0 to audioStreams.
	for _, pair := range [][2]string{{"transcodes", "streams"}, {"audioTranscodes", "audioStreams"}} {
		if n, ok := got[pair[0]]; ok {
			if top, ok := got[pair[1]]; ok && n > top {
				delete(got, pair[0])
			}
		}
	}
	ptr := func(name string) *int64 {
		if n, ok := got[name]; ok {
			return &n
		}
		return nil
	}
	k := playbackKeptLimits{
		Streams: ptr("streams"), Transcodes: ptr("transcodes"), BitrateKbps: ptr("bitrateKbps"),
		Downloads: got["downloads"], DownloadsPerDay: got["downloadsPerDay"], DownloadKbps: got["downloadKbps"], ScansPerDay: got["scansPerDay"],
		AudioStreams: ptr("audioStreams"), AudioTranscodes: ptr("audioTranscodes"),
	}
	k.OffersDownloads, k.OffersScans = k.Downloads > 0, k.ScansPerDay > 0
	return playbackEncode(k), nil
}
