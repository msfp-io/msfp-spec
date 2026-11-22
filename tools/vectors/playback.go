package main

// The playback part's data rules (spec/v0/playback.md, with video section
// 5.1's request members and media section 1's URLs), as a viewer's server
// and an owning server apply them:
//
//   - playbackPlay: an owning server's checks of a play request, in the
//     order of section 2.2, against the copies and limits of a context;
//   - playbackKeepAnswer: what a viewer's server keeps of a play answer
//     (section 2.3);
//   - playbackRestart: whether a viewer's server may join the bytes of a
//     new playback to a file it was reading (section 2.7);
//   - playbackReport: an owning server's answer to progress and stop
//     (sections 3, 4, 6.1 and 7);
//   - playbackKeepLimits: what a caller keeps of limits (section 5).

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

// The protocol's own lists and steps (playback sections 2.1 and 2.7).
var (
	playbackPurposes         = []string{"play", "download", "scan"}
	playbackDeliveries       = []string{"file", "remux", "transcode"}
	playbackRemuxContainers  = []string{"mpegts", "mp4"}
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
	playbackIdleSeconds   = 120
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
	playbackTokenForm  = regexp.MustCompile(`^[a-z0-9._-]{1,32}$`)
	playbackTicketForm = `[A-Za-z0-9_-]{1,64}`

	// Title keys (video section 4, music section 2).
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

// playbackToken: a token of core section 8.5, 1 to 32 characters of
// a-z 0-9 . _ -. (Core's own helper is openToken; the two are one rule.)
func playbackToken(s string) bool { return playbackTokenForm.MatchString(s) }

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

// playbackInt reads an MSFP integer (core section 2.3).
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

// playbackTargets is a transcode object as read (playback section 2.7,
// video section 5.1); the output gives the baseline's values for targets
// left out (maxHeight left out: the copy's own height).
type playbackTargets struct {
	VideoCodec     string `json:"videoCodec,omitempty"`
	MaxHeight      int64  `json:"maxHeight,omitempty"`
	MaxBitrateKbps int64  `json:"maxBitrateKbps,omitempty"`
	AudioCodec     string `json:"audioCodec,omitempty"`
	AudioChannels  int64  `json:"audioChannels,omitempty"`
	BurnSubtitle   string `json:"burnSubtitle,omitempty"`
	burn           any    // burnSubtitle as received, checked at step 4
	hasBurn        bool
}

// playbackReadTargets reads a transcode object at step 1: codecs must be
// tokens (an unknown token is refused at step 7, 409), numbers of their
// steps. burnSubtitle is checked at step 4.
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
		if !ok || !playbackToken(s) {
			return pbBad("transcode.%s isn't a token (core section 8.5)", name)
		}
		*dst = s
		return nil
	}
	if err := str("videoCodec", &t.VideoCodec); err != nil {
		return t, err
	}
	if err := str("audioCodec", &t.AudioCodec); err != nil {
		return t, err
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

type playbackTrackInfo struct {
	ID       string `json:"id"`
	Language string `json:"language,omitempty"`
	Role     string `json:"role,omitempty"`
	Default  bool   `json:"default,omitempty"`
}

type playbackCopy struct {
	ID          string              `json:"id"`
	Parts       int64               `json:"parts"`
	Etag        string              `json:"etag"`
	BitrateKbps int64               `json:"bitrateKbps"`
	Audio       []playbackTrackInfo `json:"audio"`
	Subtitles   []string            `json:"subtitles,omitempty"`
}

type playbackLimits struct {
	Streams         int64 `json:"streams"`
	Transcodes      int64 `json:"transcodes"`
	BitrateKbps     int64 `json:"bitrateKbps"`
	Downloads       int64 `json:"downloads"`
	DownloadsPerDay int64 `json:"downloadsPerDay"`
	DownloadKbps    int64 `json:"downloadKbps"`
	ScansPerDay     int64 `json:"scansPerDay"`
	ScanBytes       int64 `json:"scanBytes,omitempty"`
}

type playbackUse struct {
	Streams              int64 `json:"streams"`
	Transcodes           int64 `json:"transcodes"`
	Downloads            int64 `json:"downloads"`
	DownloadsToday       int64 `json:"downloadsToday"`
	ScansToday           int64 `json:"scansToday"`
	ServerTranscodesFree int64 `json:"serverTranscodesFree"`
}

type playbackContext struct {
	Copies        map[string][]playbackCopy `json:"copies"`
	VideoEncoders []string                  `json:"videoEncoders"`
	AudioEncoders []string                  `json:"audioEncoders"`
	OwnCapKbps    int64                     `json:"ownCapKbps"`
	Limits        playbackLimits            `json:"limits"`
	InUse         playbackUse               `json:"inUse"`
	StreamRoot    string                    `json:"streamRoot"`
	Now           int64                     `json:"now"`
}

// playbackStarted is what the owning server starts for a play it accepts.
type playbackStarted struct {
	Title           string           `json:"title"`
	Purpose         string           `json:"purpose"`
	Delivery        string           `json:"delivery,omitempty"`
	Baseline        string           `json:"baseline,omitempty"`
	Container       string           `json:"container,omitempty"`
	Transcode       *playbackTargets `json:"transcode,omitempty"`
	AudioRenditions bool             `json:"audioRenditions,omitempty"`
	VersionID       string           `json:"versionId"`
	AudioTrack      string           `json:"audioTrack"`
	Defaults        bool             `json:"defaults,omitempty"`
	StartPositionMs int64            `json:"startPositionMs"`
	BitrateCapKbps  int64            `json:"bitrateCapKbps,omitempty"`
	SpeedCapKbps    int64            `json:"speedCapKbps,omitempty"`
	Counts          string           `json:"counts"`
	Transcodes      string           `json:"transcodes,omitempty"`
	TicketSeconds   int64            `json:"ticketSeconds,omitempty"`
	IdleSeconds     int64            `json:"idleSeconds,omitempty"`
	ReadCapBytes    int64            `json:"readCapBytes,omitempty"`
}

// playbackLangBase is a tag's base language: what comes before its first -.
func playbackLangBase(tag string) string {
	base, _, _ := strings.Cut(tag, "-")
	return strings.ToLower(base)
}

// playbackOwnSound: a track of the title's own sound (section 2.6's
// recommended defaults): no role, main or dub; never an unknown role.
func playbackOwnSound(t playbackTrackInfo) bool {
	return t.Role == "" || t.Role == "main" || t.Role == "dub"
}

// playbackPickCopy is the copy a request names, else section 2.6's
// recommended default; named says whether the request named it.
func playbackPickCopy(copies []playbackCopy, versionID, lang string) (c playbackCopy, named bool) {
	for _, c := range copies {
		if versionID != "" && c.ID == versionID {
			return c, true
		}
	}
	if lang != "" {
		for _, same := range []func(string) bool{
			func(t string) bool { return strings.EqualFold(t, lang) },
			func(t string) bool { return playbackLangBase(t) == playbackLangBase(lang) },
		} {
			for _, c := range copies {
				for _, a := range c.Audio {
					if playbackOwnSound(a) && a.Language != "" && same(a.Language) {
						return c, false
					}
				}
			}
		}
	}
	return copies[0], false
}

// playbackPickAudio is the track a request names, else section 2.6's
// recommended default; named says whether the request named it.
func playbackPickAudio(c playbackCopy, track, lang string) (id string, named bool) {
	for _, a := range c.Audio {
		if a.ID == track {
			return a.ID, true
		}
	}
	if lang != "" {
		for _, same := range []func(string) bool{
			func(t string) bool { return strings.EqualFold(t, lang) },
			func(t string) bool { return playbackLangBase(t) == playbackLangBase(lang) },
		} {
			for _, a := range c.Audio {
				if playbackOwnSound(a) && a.Language != "" && same(a.Language) {
					return a.ID, false
				}
			}
		}
	}
	for _, a := range c.Audio {
		if a.Default {
			return a.ID, false
		}
	}
	if len(c.Audio) > 0 {
		return c.Audio[0].ID, false
	}
	return "", false
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

// playbackPlay applies section 2.2, steps 1 to 9, to a play request.
func playbackPlay(ctx playbackContext, lim playbackLimits, use playbackUse, raw []byte) ([]byte, error) {
	o, err := playbackDecode(raw)
	if err != nil {
		return nil, pbBad("%v", err)
	}
	// Step 1: forms, types, ranges, combinations.
	for _, name := range []string{"title", "purpose", "delivery", "container", "transcode", "audioRenditions", "startPositionMs", "maxBitrateKbps", "audioLanguage", "audioTrack", "versionId", "subtitleLanguage", "subtitleTrack"} {
		if v, present := o[name]; present && v == nil {
			return nil, pbBad("%s is null", name)
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
		return nil, err
	}
	if !present {
		return nil, pbBad("no title")
	}
	kind, part, err := playbackTitleKind(title)
	if err != nil {
		return nil, pbBad("title: %v", err)
	}
	if part == "" {
		return nil, pbBad("title: play doesn't take a %s", kind)
	}
	s := playbackStarted{Title: title, Purpose: "play"}
	if p, present, err := text("purpose"); err != nil {
		return nil, err
	} else if present {
		if !slices.Contains(playbackPurposes, p) {
			return nil, pbBad("purpose %q isn't play, download or scan", p)
		}
		s.Purpose = p
	}
	unknownDelivery := false
	if d, present, err := text("delivery"); err != nil {
		return nil, err
	} else if present {
		if !playbackToken(d) {
			return nil, pbBad("delivery %q isn't a token (core section 8.5)", d)
		}
		unknownDelivery = !slices.Contains(playbackDeliveries, d)
		s.Delivery = d
	}
	if c, present, err := text("container"); err != nil {
		return nil, err
	} else if present {
		if s.Delivery != "remux" {
			return nil, pbBad("container without delivery remux")
		}
		if !playbackToken(c) {
			return nil, pbBad("container %q isn't a token (core section 8.5)", c)
		}
		s.Container = c
	} else if s.Delivery == "remux" {
		return nil, pbBad("delivery remux without container")
	}
	if v, present := o["transcode"]; present {
		if s.Delivery != "transcode" {
			return nil, pbBad("a transcode object without delivery transcode")
		}
		t, err := playbackReadTargets(v)
		if err != nil {
			return nil, err
		}
		s.Transcode = &t
	} else if s.Delivery == "transcode" {
		s.Transcode = &playbackTargets{}
	}
	if v, present := o["audioRenditions"]; present {
		b, ok := v.(bool)
		if !ok {
			return nil, pbBad("audioRenditions isn't a boolean")
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
		return nil, err
	}
	reqCap, err := intIn("maxBitrateKbps", playbackMaxKbps)
	if err != nil {
		return nil, err
	}
	var lang string
	for _, name := range []string{"audioLanguage", "subtitleLanguage"} {
		v, present, err := text(name)
		if err != nil {
			return nil, err
		}
		if present && !validTag(v) {
			return nil, pbBad("%s isn't a valid tag (catalog section 3)", name)
		}
		if name == "audioLanguage" {
			lang = v
		}
	}
	versionID, _, err := text("versionId")
	if err != nil {
		return nil, err
	}
	audioTrack, hasAudio, err := text("audioTrack")
	if err != nil {
		return nil, err
	}
	subTrack, hasSub, err := text("subtitleTrack")
	if err != nil {
		return nil, err
	}
	// An unknown delivery is answered at step 7, whatever the purpose.
	if !unknownDelivery && !playbackAllowed(s.Purpose, s.Delivery, s.Transcode) {
		d := s.Delivery
		if d == "" {
			d = "none"
		}
		return nil, pbBad("purpose %s with delivery %s isn't allowed", s.Purpose, d)
	}
	// Step 2: the purpose's limit is 0.
	switch {
	case s.Purpose == "play" && lim.Streams == 0:
		return nil, pbRefuse(409, "stream-limit-reached", "streams is 0")
	case s.Purpose == "download" && lim.Downloads == 0:
		return nil, pbRefuse(409, "stream-limit-reached", "downloads is 0")
	case s.Purpose == "scan" && lim.ScansPerDay == 0:
		return nil, pbRefuse(409, "stream-limit-reached", "scansPerDay is 0")
	}
	// Step 3: copies of that exact key.
	copies := ctx.Copies[title]
	if len(copies) == 0 {
		return nil, pbRefuse(404, "not-found", "no copy of that key is offered")
	}
	// Step 4: the copy, then malformed track IDs and burnSubtitle.
	c, copyNamed := playbackPickCopy(copies, versionID, lang)
	if hasAudio && !playbackAudioForm.MatchString(audioTrack) {
		return nil, pbBad("audioTrack isn't a and a number from 0 to 999")
	}
	if hasSub && subTrack != "off" && !playbackSubForm.MatchString(subTrack) {
		return nil, pbBad("subtitleTrack isn't s and a number from 0 to 999, or off")
	}
	if t := s.Transcode; t != nil && t.hasBurn {
		b, ok := t.burn.(string)
		if !ok || !playbackSubForm.MatchString(b) {
			return nil, pbBad("transcode.burnSubtitle isn't s and a number from 0 to 999")
		}
		t.BurnSubtitle = b
	}
	s.VersionID = c.ID
	var audioNamed bool
	s.AudioTrack, audioNamed = playbackPickAudio(c, audioTrack, lang)
	s.Defaults = !copyNamed || !audioNamed
	// Step 5: the purpose's limits in use.
	switch s.Purpose {
	case "play":
		if use.Streams >= lim.Streams {
			return nil, pbRefuse(409, "stream-limit-reached", "the pairing's streams are all in use")
		}
		s.Counts, s.IdleSeconds = "streams", playbackIdleSeconds
	case "download":
		if use.Downloads >= lim.Downloads {
			return nil, pbRefuse(409, "stream-limit-reached", "the pairing's downloads are all in use")
		}
		if lim.DownloadsPerDay > 0 && use.DownloadsToday >= lim.DownloadsPerDay {
			return nil, pbRefuse(409, "stream-limit-reached", "downloadsPerDay reached")
		}
		s.Counts, s.TicketSeconds, s.SpeedCapKbps = "downloads", playbackDownloadTTL, lim.DownloadKbps
		s.StartPositionMs = 0 // served from its beginning
	case "scan":
		if use.ScansToday >= lim.ScansPerDay {
			return nil, pbRefuse(409, "stream-limit-reached", "scansPerDay reached")
		}
		cap := lim.ScanBytes
		if cap < playbackScanFloor || cap > playbackScanTop {
			cap = playbackScanFloor // none sent: the floor its callers count on
		}
		s.Counts, s.TicketSeconds, s.IdleSeconds, s.ReadCapBytes = "scansPerDay", playbackScanTTL, playbackIdleSeconds, cap
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
	// Step 7: the delivery asked, or a refusal.
	if unknownDelivery {
		return nil, pbRefuse(409, "delivery-refused", "a delivery the owning server doesn't make")
	}
	if s.Delivery == "remux" && !slices.Contains(playbackRemuxContainers, s.Container) {
		return nil, pbRefuse(409, "delivery-refused", "a remux container other than mpegts or mp4")
	}
	if t := s.Transcode; t != nil {
		if t.VideoCodec != "" && part == "video" && !slices.Contains(ctx.VideoEncoders, t.VideoCodec) {
			return nil, pbRefuse(409, "delivery-refused", "a video codec the owning server doesn't encode")
		}
		if t.AudioCodec != "" && !slices.Contains(ctx.AudioEncoders, t.AudioCodec) {
			return nil, pbRefuse(409, "delivery-refused", "an audio codec the owning server doesn't encode")
		}
	}
	if s.Delivery == "file" && c.Parts > 1 {
		return nil, pbRefuse(409, "delivery-refused", "a copy of several parts is never delivered as file")
	}
	if (s.Delivery == "file" || s.Delivery == "remux") && s.Purpose == "play" && s.BitrateCapKbps > 0 && c.BitrateKbps > s.BitrateCapKbps {
		return nil, pbRefuse(409, "delivery-refused", "the copy's bitrate is above the cap")
	}
	if s.Transcode != nil && s.Transcode.BurnSubtitle != "" && !slices.Contains(c.Subtitles, s.Transcode.BurnSubtitle) {
		return nil, pbRefuse(409, "delivery-refused", "burnSubtitle isn't a subtitle track of the copy")
	}
	// Targets left out take the baseline's values (maxHeight: the copy's
	// own height, left out); without delivery, the baseline itself.
	if t := s.Transcode; t != nil {
		if part == "video" {
			if t.VideoCodec == "" {
				t.VideoCodec = playbackBaselineVideo
			}
		} else {
			t.VideoCodec, t.MaxHeight = "", 0 // ignored for a title without a picture
		}
		if t.AudioCodec == "" {
			t.AudioCodec = playbackBaselineAudio
		}
		if t.AudioChannels == 0 {
			t.AudioChannels = playbackBaselineChannels
		}
	}
	if s.Delivery == "" {
		s.Baseline = "h264 aac hls"
		if part == "music" {
			s.Baseline = "aac hls"
		}
	}
	// Step 8: transcodes.
	if s.Delivery == "transcode" {
		if s.Purpose == "play" && use.Transcodes >= lim.Transcodes {
			return nil, pbRefuse(503, "server-busy", "the pairing's transcodes are all in use, or 0")
		}
		if use.ServerTranscodesFree <= 0 {
			return nil, pbRefuse(503, "server-busy", "the server's transcode budget is used")
		}
		s.Transcodes = "pairing and server"
		if s.Purpose == "download" {
			s.Transcodes = "server"
		}
	}
	return playbackEncode(s), nil
}

// The answer (section 2.3), as a viewer's server keeps it.

type playbackStream struct {
	URL      string `json:"url"`
	Protocol string `json:"protocol"`
	MimeType string `json:"mimeType,omitempty"`
}

type playbackTrack struct {
	ID              string `json:"id"`
	Language        string `json:"language,omitempty"`
	Channels        *int64 `json:"channels,omitempty"`
	Role            string `json:"role,omitempty"`
	Default         *bool  `json:"default,omitempty"`
	Forced          *bool  `json:"forced,omitempty"`
	HearingImpaired *bool  `json:"hearingImpaired,omitempty"`
}

type playbackAnswer struct {
	ID              string          `json:"id"`
	Method          string          `json:"method"`
	Stream          playbackStream  `json:"stream"`
	Etag            string          `json:"etag"`
	PositionMs      int64           `json:"positionMs"`
	DurationMs      *int64          `json:"durationMs,omitempty"`
	VersionID       string          `json:"versionId,omitempty"`
	AudioTracks     []playbackTrack `json:"audioTracks,omitempty"`
	AudioTrack      string          `json:"audioTrack,omitempty"`
	AudioRenditions bool            `json:"audioRenditions,omitempty"`
	MaxBitrateKbps  *int64          `json:"maxBitrateKbps,omitempty"`
	ReadTitleAgain  bool            `json:"readTitleAgain,omitempty"`
}

func playbackID(s string) bool { return s == "off" || playbackObjIDForm.MatchString(s) }

// playbackKeepAnswer checks and cleans a play answer: delivery is what
// the request asked ("" for none), catalogEtag the copy's etag in the
// catalog, root the owning server's stream root. An error means the
// viewer's server stops that playback and gives up.
func playbackKeepAnswer(raw []byte, delivery, catalogEtag, root string) ([]byte, error) {
	o, err := playbackDecode(raw)
	if err != nil {
		return nil, err
	}
	str := func(name string, required bool) (string, error) {
		v, present := o[name]
		if !present {
			if required {
				return "", fmt.Errorf("no %s", name)
			}
			return "", nil
		}
		s, ok := v.(string)
		if !ok {
			return "", fmt.Errorf("%s isn't a string", name)
		}
		return s, nil
	}
	var a playbackAnswer
	if a.ID, err = str("id", true); err != nil {
		return nil, err
	}
	if !playbackIDForm.MatchString(a.ID) {
		return nil, errors.New("id doesn't match ^[A-Za-z0-9_-]{1,64}$")
	}
	if a.Method, err = str("method", true); err != nil {
		return nil, err
	}
	if !slices.Contains([]string{"direct", "remux", "transcode"}, a.Method) {
		return nil, fmt.Errorf("method %q", a.Method)
	}
	want := map[string]string{"file": "direct", "remux": "remux", "transcode": "transcode"}[delivery]
	if want != "" && a.Method != want {
		return nil, fmt.Errorf("method %s for delivery %s", a.Method, delivery)
	}
	so, ok := o["stream"].(map[string]any)
	if !ok {
		return nil, errors.New("stream missing or not an object")
	}
	a.Stream.URL, _ = so["url"].(string)
	a.Stream.Protocol, _ = so["protocol"].(string)
	mt, hasMT := so["mimeType"]
	if hasMT {
		if a.Stream.MimeType, ok = mt.(string); !ok {
			return nil, errors.New("stream.mimeType isn't a string")
		}
	}
	name := map[string]string{"hls": "master.m3u8", "file": "file"}[a.Stream.Protocol]
	if name == "" {
		return nil, fmt.Errorf("stream.protocol %q isn't hls or file", a.Stream.Protocol)
	}
	if !regexp.MustCompile(`^` + regexp.QuoteMeta(root) + playbackTicketForm + `/` + regexp.QuoteMeta(name) + `$`).MatchString(a.Stream.URL) {
		return nil, errors.New("stream.url isn't <stream root><ticket>/" + name)
	}
	if a.Stream.Protocol == "hls" && hasMT && a.Stream.MimeType != "application/vnd.apple.mpegurl" {
		return nil, errors.New("an hls stream.mimeType other than application/vnd.apple.mpegurl")
	}
	if delivery == "" && a.Stream.Protocol != "hls" {
		return nil, errors.New("without delivery, the baseline: hls only")
	}
	if (a.Method == "direct") != (a.Stream.Protocol == "file") && delivery != "" {
		return nil, fmt.Errorf("protocol %s for delivery %s", a.Stream.Protocol, delivery)
	}
	if a.Method != "direct" && a.Stream.Protocol == "file" {
		return nil, errors.New("a remux or a transcode served as file")
	}
	if a.Etag, err = str("etag", true); err != nil {
		return nil, err
	}
	if !playbackEtagForm.MatchString(a.Etag) {
		return nil, errors.New("etag isn't 1 to 64 characters of A-Z a-z 0-9 - _")
	}
	pos, present := o["positionMs"]
	if !present {
		return nil, errors.New("no positionMs")
	}
	if a.PositionMs, ok = playbackInt(pos); !ok {
		return nil, errors.New("positionMs isn't an integer")
	}
	if a.PositionMs < 0 || a.PositionMs > playbackMaxPositionMs {
		return nil, errors.New("positionMs out of 0 to 100 days")
	}
	if v, present := o["durationMs"]; present {
		n, ok := playbackInt(v)
		if !ok {
			return nil, errors.New("durationMs isn't an integer")
		}
		if n < 0 || n > playbackMaxPositionMs {
			n = 0
		}
		a.DurationMs = &n
	}
	if a.VersionID, err = str("versionId", false); err != nil {
		return nil, err
	}
	if !playbackObjIDForm.MatchString(a.VersionID) {
		a.VersionID = ""
	}
	if v, present := o["audioTracks"]; present {
		list, ok := v.([]any)
		if !ok {
			return nil, errors.New("audioTracks isn't an array")
		}
		for _, e := range list {
			eo, ok := e.(map[string]any)
			if !ok {
				continue
			}
			id, ok := eo["id"].(string)
			if !ok || !playbackID(id) {
				continue
			}
			t := playbackTrack{ID: id}
			if l, ok := eo["language"].(string); ok && validTag(l) {
				t.Language = l
			}
			if n, ok := playbackInt(eo["channels"]); ok && n >= 1 && n <= 64 {
				t.Channels = &n
			}
			if r, ok := eo["role"].(string); ok && playbackToken(r) {
				t.Role = r
			}
			for _, f := range []struct {
				name string
				dst  **bool
			}{{"default", &t.Default}, {"forced", &t.Forced}, {"hearingImpaired", &t.HearingImpaired}} {
				if b, ok := eo[f.name].(bool); ok {
					*f.dst = &b
				}
			}
			a.AudioTracks = append(a.AudioTracks, t)
		}
	}
	if a.AudioTrack, err = str("audioTrack", false); err != nil {
		return nil, err
	}
	if a.AudioTrack != "" && !playbackID(a.AudioTrack) {
		a.AudioTrack = ""
	}
	if v, present := o["audioRenditions"]; present {
		b, ok := v.(bool)
		if !ok {
			return nil, errors.New("audioRenditions isn't a boolean")
		}
		a.AudioRenditions = b && a.Stream.Protocol == "hls"
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
	a.ReadTitleAgain = catalogEtag != "" && a.Etag != catalogEtag
	return playbackEncode(a), nil
}

// Restarts (section 2.7).

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
	kept, err := playbackKeepAnswer(answer, "file", "", root)
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

// Progress and stop (sections 3, 4, 6.1 and 7).

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
}

// playbackReport answers a progress or stop (call) at time now.
func playbackReport(call string, now int64, pb playbackState, raw []byte) ([]byte, error) {
	o, err := playbackDecode(raw)
	if err != nil {
		return nil, pbBad("%v", err)
	}
	members := []string{"id", "positionMs", "at", "title"}
	if call == "progress" {
		members = append(members, "paused") // stop has no paused: ignored there
	}
	for _, name := range members {
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
	if v, present := o["paused"]; present && call == "progress" {
		if _, ok := v.(bool); !ok {
			return nil, pbBad("paused isn't a boolean")
		}
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
	rec := playbackReportResult{Title: title, PositionMs: pos, At: at}
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

// Limits (section 5, music section 5.1), as a caller keeps them.

type playbackKeptLimits struct {
	Streams         *int64 `json:"streams,omitempty"`
	Transcodes      *int64 `json:"transcodes,omitempty"`
	BitrateKbps     *int64 `json:"bitrateKbps,omitempty"`
	Downloads       int64  `json:"downloads"`
	DownloadsPerDay int64  `json:"downloadsPerDay"`
	DownloadKbps    int64  `json:"downloadKbps"`
	ScansPerDay     int64  `json:"scansPerDay"`
	ScanBytes       int64  `json:"scanBytes"`
	AudioStreams    *int64 `json:"audioStreams,omitempty"`
	AudioTranscodes *int64 `json:"audioTranscodes,omitempty"`
	OffersDownloads bool   `json:"offersDownloads"`
	OffersScans     bool   `json:"offersScans"`
}

// playbackKeepLimits reads the libraries answer's limits: a limits that
// isn't an object, or one of whose known members isn't an integer, is
// ignored whole (an error); a member out of range is ignored alone.
func playbackKeepLimits(raw []byte) ([]byte, error) {
	o, err := playbackDecode(raw)
	if err != nil {
		return nil, err
	}
	known := []struct {
		name string
		max  int64
		min  int64
	}{{"streams", playbackMaxStreams, 0}, {"transcodes", playbackMaxStreams, 0}, {"bitrateKbps", 1000000, 0}, {"downloads", playbackMaxStreams, 0}, {"downloadsPerDay", 10000, 0}, {"downloadKbps", 1000000, 0}, {"scansPerDay", 1000000, 0}, {"scanBytes", playbackScanTop, playbackScanFloor}, {"audioStreams", playbackMaxStreams, 0}, {"audioTranscodes", playbackMaxStreams, 0}}
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
		ScanBytes: playbackScanFloor,
	}
	if n, ok := got["scanBytes"]; ok {
		k.ScanBytes = n
	}
	k.OffersDownloads, k.OffersScans = k.Downloads > 0, k.ScansPerDay > 0
	return playbackEncode(k), nil
}
