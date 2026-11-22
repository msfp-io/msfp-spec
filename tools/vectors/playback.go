package main

// The playback part's data rules (spec/v0/playback.md, with video section
// 5.1's request members and capabilities and media section 1's URLs), as
// a viewer's server and an owning server apply them:
//
//   - playbackCoarsen: what a viewer's server sends of a device's
//     capabilities (playback section 2.1, video section 5.1);
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
	"sort"
	"strconv"
	"strings"
	"unicode"
)

// The coarse lists (playback section 2.1, video section 5.1).
var (
	playbackContainers    = []string{"mp4", "m4v", "mov", "webm", "mkv", "ts", "mp3", "aac", "m4a", "flac", "ogg", "opus", "wav", "avi", "m2ts", "wmv", "flv", "3gp"}
	playbackAudioCodecs   = []string{"aac", "mp3", "opus", "vorbis", "flac", "ac3", "eac3", "alac", "pcm_s16le", "dts", "truehd", "mp2", "wmav2", "pcm_s24le", "pcm_bluray"}
	playbackAudioProfiles = []string{"atmos", "dtsx", "dts-hd-ma", "dts-hd-hra", "dts-es", "he-aac", "lc"}
	playbackVideoCodecs   = []string{"h264", "hevc", "av1", "vp8", "vp9", "mpeg2video", "mpeg4", "vc1", "wmv3", "h263", "theora"}
	playbackHDR           = []string{"hdr10", "hdr10plus", "hlg", "dv"}
	playbackDVProfiles    = []int64{4, 5, 7, 8, 9, 10}
	playbackHeights       = []int64{480, 720, 1080, 1440, 2160, 4320}
	playbackChannelSteps  = []int64{2, 6, 8}
	playbackRateSteps     = []int64{44100, 48000, 96000, 192000}
	playbackDepthSteps    = []int64{16, 24, 32}
)

// Bounds.
const (
	playbackMaxPositionMs = 8640000000 // 100 days
	playbackMaxKbps       = 10000000
	playbackDefaultCap    = 120000
	playbackAtBack        = 30 * 86400
	playbackAtAhead       = 5 * 60
	playbackLateWindow    = 30 * 86400
	playbackDownloadTTL   = 86400
	playbackScanTTL       = 600
	playbackIdleSeconds   = 120
	playbackScanCapBytes  = 33554432
)

var (
	playbackIDForm     = regexp.MustCompile(`^play_[a-z2-7]{26}$`)
	playbackEtagForm   = regexp.MustCompile(`^[A-Za-z0-9_-]{1,64}$`)
	playbackObjIDForm  = regexp.MustCompile(`^[A-Za-z0-9_-]{1,64}$`)
	playbackAudioForm  = regexp.MustCompile(`^a(0|[1-9][0-9]{0,2})$`)
	playbackSubForm    = regexp.MustCompile(`^s(0|[1-9][0-9]{0,2})$`)
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

// stepDown: the largest step at most v; below the first, the first.
func playbackStepDown(v int64, steps []int64) int64 {
	out := steps[0]
	for _, s := range steps {
		if s <= v {
			out = s
		}
	}
	return out
}

// playbackCaps is a capabilities object as the vectors write it: lists
// sorted, each value once, members left out when nothing is said.
type playbackCaps struct {
	Containers         []string `json:"containers,omitempty"`
	VideoCodecs        []string `json:"videoCodecs,omitempty"`
	AudioCodecs        []string `json:"audioCodecs,omitempty"`
	AudioProfiles      []string `json:"audioProfiles,omitempty"`
	HDR                []string `json:"hdr,omitempty"`
	DVProfiles         []int64  `json:"dvProfiles,omitempty"`
	MaxHeight          int64    `json:"maxHeight,omitempty"`
	MaxAudioChannels   int64    `json:"maxAudioChannels,omitempty"`
	MaxAudioSampleRate int64    `json:"maxAudioSampleRate,omitempty"`
	MaxAudioBitDepth   int64    `json:"maxAudioBitDepth,omitempty"`
	MaxBitrateKbps     int64    `json:"maxBitrateKbps,omitempty"`
}

func (c playbackCaps) empty() bool {
	return len(c.Containers)+len(c.VideoCodecs)+len(c.AudioCodecs)+len(c.AudioProfiles)+len(c.HDR)+len(c.DVProfiles) == 0 &&
		c.MaxHeight == 0 && c.MaxAudioChannels == 0 && c.MaxAudioSampleRate == 0 && c.MaxAudioBitDepth == 0 && c.MaxBitrateKbps == 0
}

var playbackStringLists = []struct {
	name string
	list []string
	set  func(*playbackCaps, []string)
}{
	{"containers", playbackContainers, func(c *playbackCaps, v []string) { c.Containers = v }},
	{"videoCodecs", playbackVideoCodecs, func(c *playbackCaps, v []string) { c.VideoCodecs = v }},
	{"audioCodecs", playbackAudioCodecs, func(c *playbackCaps, v []string) { c.AudioCodecs = v }},
	{"audioProfiles", playbackAudioProfiles, func(c *playbackCaps, v []string) { c.AudioProfiles = v }},
	{"hdr", playbackHDR, func(c *playbackCaps, v []string) { c.HDR = v }},
	{"subtitles", nil, func(*playbackCaps, []string) {}}, // no values defined yet
}

var playbackIntCaps = []struct {
	name string
	max  int64
	set  func(*playbackCaps, int64)
}{
	{"maxHeight", 100000, func(c *playbackCaps, v int64) { c.MaxHeight = v }},
	{"maxAudioChannels", 64, func(c *playbackCaps, v int64) { c.MaxAudioChannels = v }},
	{"maxAudioSampleRate", 768000, func(c *playbackCaps, v int64) { c.MaxAudioSampleRate = v }},
	{"maxAudioBitDepth", 64, func(c *playbackCaps, v int64) { c.MaxAudioBitDepth = v }},
	{"maxBitrateKbps", playbackMaxKbps, func(c *playbackCaps, v int64) { c.MaxBitrateKbps = v }},
}

// keepListed keeps the entries of list, each once, sorted.
func playbackKeepListed(entries []string, list []string) []string {
	var out []string
	for _, e := range entries {
		if slices.Contains(list, e) && !slices.Contains(out, e) {
			out = append(out, e)
		}
	}
	sort.Strings(out)
	return out
}

// playbackReadCaps is the owning server's reading of capabilities
// (playback sections 2.1 and 2.2, video section 5.1): a wrong type, a
// non-string (or, in dvProfiles, non-integer) entry or a number out of
// range refuses the request; unknown entries are ignored, in another case
// included; dv counts only with dvProfiles.
func playbackReadCaps(v any) (playbackCaps, error) {
	var c playbackCaps
	o, ok := v.(map[string]any)
	if !ok {
		return c, pbBad("capabilities isn't an object")
	}
	for _, l := range playbackStringLists {
		raw, present := o[l.name]
		if !present {
			continue
		}
		arr, ok := raw.([]any)
		if !ok {
			return c, pbBad("capabilities.%s isn't an array", l.name)
		}
		var entries []string
		for _, e := range arr {
			s, ok := e.(string)
			if !ok {
				return c, pbBad("capabilities.%s has an entry that isn't a string", l.name)
			}
			entries = append(entries, s)
		}
		l.set(&c, playbackKeepListed(entries, l.list))
	}
	if raw, present := o["dvProfiles"]; present {
		arr, ok := raw.([]any)
		if !ok {
			return c, pbBad("capabilities.dvProfiles isn't an array")
		}
		for _, e := range arr {
			n, ok := playbackInt(e)
			if !ok {
				return c, pbBad("capabilities.dvProfiles has an entry that isn't an integer")
			}
			if slices.Contains(playbackDVProfiles, n) && !slices.Contains(c.DVProfiles, n) {
				c.DVProfiles = append(c.DVProfiles, n)
			}
		}
		slices.Sort(c.DVProfiles)
	}
	for _, f := range playbackIntCaps {
		raw, present := o[f.name]
		if !present {
			continue
		}
		n, ok := playbackInt(raw)
		if !ok {
			return c, pbBad("capabilities.%s isn't an integer", f.name)
		}
		if n < 0 || n > f.max {
			return c, pbBad("capabilities.%s out of 0 to %d", f.name, f.max)
		}
		f.set(&c, n)
	}
	if len(c.DVProfiles) == 0 {
		c.HDR = slices.DeleteFunc(c.HDR, func(s string) bool { return s == "dv" })
		if len(c.HDR) == 0 {
			c.HDR = nil
		}
	}
	return c, nil
}

// playbackCoarsen is what a viewer's server sends of its device's
// capabilities and bitrate cap (playback section 2.1, video section 5.1).
// text is {"title", "capabilities", "maxBitrateKbps"} as the device
// describes itself; the result is the same request, coarsened.
func playbackCoarsen(raw []byte) ([]byte, error) {
	o, err := playbackDecode(raw)
	if err != nil {
		return nil, err
	}
	title, _ := o["title"].(string)
	in, _ := o["capabilities"].(map[string]any)
	var c playbackCaps
	for _, l := range playbackStringLists {
		arr, _ := in[l.name].([]any)
		var entries []string
		for _, e := range arr {
			if s, ok := e.(string); ok {
				entries = append(entries, strings.ToLower(s))
			}
		}
		l.set(&c, playbackKeepListed(entries, l.list))
	}
	arr, _ := in["dvProfiles"].([]any)
	for _, e := range arr {
		if n, ok := playbackInt(e); ok && slices.Contains(playbackDVProfiles, n) && !slices.Contains(c.DVProfiles, n) {
			c.DVProfiles = append(c.DVProfiles, n)
		}
	}
	slices.Sort(c.DVProfiles)
	step := func(name string, steps []int64) int64 {
		n, ok := playbackInt(in[name])
		if !ok || n <= 0 {
			return 0 // not said
		}
		return playbackStepDown(n, steps)
	}
	c.MaxHeight = step("maxHeight", playbackHeights)
	c.MaxAudioChannels = step("maxAudioChannels", playbackChannelSteps)
	c.MaxAudioSampleRate = step("maxAudioSampleRate", playbackRateSteps)
	c.MaxAudioBitDepth = step("maxAudioBitDepth", playbackDepthSteps)
	if n, ok := playbackInt(in["maxBitrateKbps"]); ok && n >= 1000 {
		c.MaxBitrateKbps = n / 1000 * 1000
	}
	out := struct {
		Title          string        `json:"title"`
		Capabilities   *playbackCaps `json:"capabilities,omitempty"`
		MaxBitrateKbps *int64        `json:"maxBitrateKbps,omitempty"`
	}{Title: title}
	if !c.empty() {
		out.Capabilities = &c
	}
	if n, ok := playbackInt(o["maxBitrateKbps"]); ok && n >= 0 {
		if n >= 1000 {
			n = n / 1000 * 1000
		}
		out.MaxBitrateKbps = &n
	}
	return playbackEncode(out), nil
}

// playbackTargets is a transcode object as read (playback section 2.7,
// video section 5.1).
type playbackTargets struct {
	VideoCodec     string `json:"videoCodec,omitempty"`
	MaxHeight      int64  `json:"maxHeight,omitempty"`
	MaxBitrateKbps int64  `json:"maxBitrateKbps,omitempty"`
	AudioCodec     string `json:"audioCodec,omitempty"`
	AudioChannels  int64  `json:"audioChannels,omitempty"`
	BurnSubtitle   string `json:"burnSubtitle,omitempty"`
}

func playbackReadTargets(v any) (playbackTargets, error) {
	var t playbackTargets
	o, ok := v.(map[string]any)
	if !ok {
		return t, pbBad("transcode isn't an object")
	}
	str := func(name string, list []string, dst *string) error {
		raw, present := o[name]
		if !present {
			return nil
		}
		s, ok := raw.(string)
		if !ok || !slices.Contains(list, s) {
			return pbBad("transcode.%s isn't one of its list", name)
		}
		*dst = s
		return nil
	}
	if err := str("videoCodec", playbackVideoCodecs, &t.VideoCodec); err != nil {
		return t, err
	}
	if err := str("audioCodec", playbackAudioCodecs, &t.AudioCodec); err != nil {
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
	if raw, present := o["burnSubtitle"]; present {
		s, ok := raw.(string)
		if !ok || !playbackSubForm.MatchString(s) {
			return t, pbBad("transcode.burnSubtitle isn't s and a number from 0 to 999")
		}
		t.BurnSubtitle = s
	}
	return t, nil
}

// The owning server's context: its copies, the pairing's limits, what is
// in use.

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
	Copies     map[string][]playbackCopy `json:"copies"`
	OwnCapKbps int64                     `json:"ownCapKbps"`
	Limits     playbackLimits            `json:"limits"`
	InUse      playbackUse               `json:"inUse"`
	StreamRoot string                    `json:"streamRoot"`
	Now        int64                     `json:"now"`
}

// playbackStarted is what the owning server starts for a play it accepts.
type playbackStarted struct {
	Title           string           `json:"title"`
	Purpose         string           `json:"purpose"`
	Delivery        string           `json:"delivery,omitempty"`
	Container       string           `json:"container,omitempty"`
	Transcode       *playbackTargets `json:"transcode,omitempty"`
	AudioRenditions bool             `json:"audioRenditions,omitempty"`
	Capabilities    *playbackCaps    `json:"capabilities,omitempty"`
	VersionID       string           `json:"versionId"`
	AudioTrack      string           `json:"audioTrack"`
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

// playbackOwnSound: a track of the title's own sound (no role).
func playbackOwnSound(t playbackTrackInfo) bool { return t.Role == "" }

// playbackPickCopy is section 2.6's table of the copy.
func playbackPickCopy(copies []playbackCopy, versionID, lang string) playbackCopy {
	for _, c := range copies {
		if versionID != "" && c.ID == versionID {
			return c
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
						return c
					}
				}
			}
		}
	}
	return copies[0]
}

// playbackPickAudio is section 2.6's table of the audio.
func playbackPickAudio(c playbackCopy, track, lang string) string {
	for _, a := range c.Audio {
		if a.ID == track {
			return a.ID
		}
	}
	if lang != "" {
		for _, same := range []func(string) bool{
			func(t string) bool { return strings.EqualFold(t, lang) },
			func(t string) bool { return playbackLangBase(t) == playbackLangBase(lang) },
		} {
			for _, a := range c.Audio {
				if playbackOwnSound(a) && a.Language != "" && same(a.Language) {
					return a.ID
				}
			}
		}
	}
	for _, a := range c.Audio {
		if a.Default {
			return a.ID
		}
	}
	if len(c.Audio) > 0 {
		return c.Audio[0].ID
	}
	return ""
}

// playbackAllowed is section 2.7's table of purpose and delivery.
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
	for _, name := range []string{"title", "purpose", "delivery", "container", "transcode", "audioRenditions", "capabilities", "startPositionMs", "maxBitrateKbps", "audioLanguage", "audioTrack", "versionId", "subtitleLanguage", "subtitleTrack"} {
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
		if !slices.Contains([]string{"play", "download", "scan"}, p) {
			return nil, pbBad("purpose %q isn't play, download or scan", p)
		}
		s.Purpose = p
	}
	if d, present, err := text("delivery"); err != nil {
		return nil, err
	} else if present {
		if !slices.Contains([]string{"file", "remux", "transcode"}, d) {
			return nil, pbBad("delivery %q isn't file, remux or transcode", d)
		}
		s.Delivery = d
	}
	if c, present, err := text("container"); err != nil {
		return nil, err
	} else if present {
		if s.Delivery != "remux" {
			return nil, pbBad("container without delivery remux")
		}
		if !slices.Contains(playbackContainers, c) {
			return nil, pbBad("container %q isn't one of containers", c)
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
	}
	if v, present := o["audioRenditions"]; present {
		b, ok := v.(bool)
		if !ok {
			return nil, pbBad("audioRenditions isn't a boolean")
		}
		s.AudioRenditions = b
	}
	if v, present := o["capabilities"]; present {
		c, err := playbackReadCaps(v)
		if err != nil {
			return nil, err
		}
		if !c.empty() {
			s.Capabilities = &c
		}
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
	if !playbackAllowed(s.Purpose, s.Delivery, s.Transcode) {
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
	// Step 4: the copy, then malformed track IDs.
	c := playbackPickCopy(copies, versionID, lang)
	if hasAudio && !playbackAudioForm.MatchString(audioTrack) {
		return nil, pbBad("audioTrack isn't a and a number from 0 to 999")
	}
	if hasSub && subTrack != "off" && !playbackSubForm.MatchString(subTrack) {
		return nil, pbBad("subtitleTrack isn't s and a number from 0 to 999, or off")
	}
	s.VersionID = c.ID
	s.AudioTrack = playbackPickAudio(c, audioTrack, lang)
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
		s.Counts, s.TicketSeconds, s.IdleSeconds, s.ReadCapBytes = "scansPerDay", playbackScanTTL, playbackIdleSeconds, playbackScanCapBytes
	}
	// Step 6: the bitrate cap, for play only.
	if s.Purpose == "play" {
		caps := []int64{reqCap, ctx.OwnCapKbps, lim.BitrateKbps}
		if s.Capabilities != nil {
			caps = append(caps, s.Capabilities.MaxBitrateKbps)
		}
		if s.Transcode != nil {
			caps = append(caps, s.Transcode.MaxBitrateKbps)
		}
		for _, v := range caps {
			if v > 0 && (s.BitrateCapKbps == 0 || v < s.BitrateCapKbps) {
				s.BitrateCapKbps = v
			}
		}
		if s.BitrateCapKbps == 0 {
			s.BitrateCapKbps = playbackDefaultCap
		}
	}
	// Step 7: the delivery asked, or a refusal.
	if s.Delivery == "file" && c.Parts > 1 {
		return nil, pbRefuse(409, "delivery-refused", "a copy of several parts is never delivered as file")
	}
	if (s.Delivery == "file" || s.Delivery == "remux") && s.Purpose == "play" && c.BitrateKbps > s.BitrateCapKbps {
		return nil, pbRefuse(409, "delivery-refused", "the copy's bitrate is above the cap")
	}
	if s.Transcode != nil && s.Transcode.BurnSubtitle != "" && !slices.Contains(c.Subtitles, s.Transcode.BurnSubtitle) {
		return nil, pbRefuse(409, "delivery-refused", "burnSubtitle isn't a subtitle track of the copy")
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

type playbackVersion struct {
	ID   string `json:"id"`
	Name string `json:"name,omitempty"`
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
	ID             string            `json:"id"`
	Method         string            `json:"method"`
	Stream         playbackStream    `json:"stream"`
	Etag           string            `json:"etag"`
	PositionMs     int64             `json:"positionMs"`
	DurationMs     *int64            `json:"durationMs,omitempty"`
	Versions       []playbackVersion `json:"versions,omitempty"`
	VersionID      string            `json:"versionId,omitempty"`
	AudioTracks    []playbackTrack   `json:"audioTracks,omitempty"`
	AudioTrack     string            `json:"audioTrack,omitempty"`
	MaxBitrateKbps *int64            `json:"maxBitrateKbps,omitempty"`
	ReadTitleAgain bool              `json:"readTitleAgain,omitempty"`
}

// playbackCleanName: core section 9.1's cleaning, cut to max code points.
func playbackCleanName(s string, max int) string {
	var b strings.Builder
	space := false
	for _, r := range s {
		if unicode.Is(unicode.White_Space, r) {
			if !space {
				b.WriteRune(' ')
			}
			space = true
			continue
		}
		space = false
		if unicode.Is(unicode.Cc, r) || unicode.Is(unicode.Cf, r) {
			continue
		}
		b.WriteRune(r)
	}
	out := strings.TrimSpace(b.String())
	if r := []rune(out); len(r) > max {
		out = strings.TrimSpace(string(r[:max]))
	}
	return out
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
		return nil, errors.New("id doesn't match ^play_[a-z2-7]{26}$")
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
	if v, present := o["versions"]; present {
		list, ok := v.([]any)
		if !ok {
			return nil, errors.New("versions isn't an array")
		}
		for _, e := range list {
			eo, ok := e.(map[string]any)
			if !ok {
				continue
			}
			id, ok := eo["id"].(string)
			if !ok || !playbackObjIDForm.MatchString(id) {
				continue
			}
			nm, _ := eo["name"].(string)
			a.Versions = append(a.Versions, playbackVersion{ID: id, Name: playbackCleanName(nm, 32)})
		}
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
			if n, ok := playbackInt(eo["channels"]); ok && n >= 0 {
				t.Channels = &n
			}
			if r, ok := eo["role"].(string); ok && (r == "commentary" || r == "description") {
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
	for _, name := range []string{"id", "positionMs", "paused", "at", "title"} {
		if v, present := o[name]; present && v == nil {
			return nil, pbBad("%s is null", name)
		}
	}
	id, ok := o["id"].(string)
	if !ok || !playbackIDForm.MatchString(id) {
		return nil, pbBad("id missing or not ^play_[a-z2-7]{26}$")
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
	}{{"streams", 50}, {"transcodes", 50}, {"bitrateKbps", 1000000}, {"downloads", 50}, {"downloadsPerDay", 10000}, {"downloadKbps", 1000000}, {"scansPerDay", 1000000}, {"audioStreams", 50}, {"audioTranscodes", 50}}
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
		if n >= 0 && n <= k.max {
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
