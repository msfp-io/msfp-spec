package main

import (
	"strconv"
	"strings"
)

// playbackCase is one call and its outcome: a play, progress or stop the
// owning server judges.
type playbackCase struct {
	Name      string          `json:"name"`
	Call      string          `json:"call"`
	Limits    *playbackLimits `json:"limits,omitempty"`
	InUse     *playbackUse    `json:"inUse,omitempty"`
	Ranges    []string        `json:"ranges,omitempty"`
	ScanBytes int64           `json:"scanBytes,omitempty"`
	Playback  *playbackState  `json:"playback,omitempty"`
	Text      string          `json:"text"`
	Valid     bool            `json:"valid"`
	Status    int             `json:"status,omitempty"`
	Type      string          `json:"type,omitempty"`
	Output    string          `json:"output,omitempty"`
	Reason    string          `json:"reason,omitempty"`
}

type playbackAnswerCase struct {
	Name   string        `json:"name"`
	Asked  playbackAsked `json:"asked"`
	Text   string        `json:"text"`
	Valid  bool          `json:"valid"`
	Output string        `json:"output,omitempty"`
	Reason string        `json:"reason,omitempty"`
}

type playbackRestartCase struct {
	Name    string          `json:"name"`
	Reading playbackReading `json:"reading"`
	Request string          `json:"request"`
	Status  int             `json:"status"`
	Type    string          `json:"type,omitempty"`
	Answer  string          `json:"answer,omitempty"`
	Valid   bool            `json:"valid"`
	Output  string          `json:"output"`
	Reason  string          `json:"reason,omitempty"`
}

type playbackLimitsCase struct {
	Name   string `json:"name"`
	Text   string `json:"text"`
	Valid  bool   `json:"valid"`
	Output string `json:"output,omitempty"`
	Reason string `json:"reason,omitempty"`
}

type playbackQueueCase struct {
	Name    string `json:"name"`
	Request string `json:"request"`
	Entry   string `json:"entry"`
	Valid   bool   `json:"valid"`
	Status  int    `json:"status,omitempty"`
	Type    string `json:"type,omitempty"`
	Output  string `json:"output,omitempty"`
	Reason  string `json:"reason,omitempty"`
}

type v0PlaybackFile struct {
	About        []string              `json:"about"`
	Context      playbackContext       `json:"context"`
	Requests     []playbackCase        `json:"requests"`
	Answers      []playbackAnswerCase  `json:"answers"`
	Deliveries   []playbackCase        `json:"deliveries"`
	Baseline     []playbackCase        `json:"baseline"`
	QueueEntries []playbackQueueCase   `json:"queueEntries"`
	Restarts     []playbackRestartCase `json:"restarts"`
	Reports      []playbackCase        `json:"reports"`
	Downloads    []playbackCase        `json:"downloads"`
	Scans        []playbackCase        `json:"scans"`
	Limits       []playbackLimitsCase  `json:"limits"`
}

const (
	pbNow       = 1795132800
	pbPID       = "play_abcdefghijklmnopqrstuvwxyz"
	pbRoot      = "https://a.example.org/api/v1/stream/"
	pbFilmKey   = "movie/tmdb/949"
	pbTestKey   = "movie/tmdb/550"
	pbThemeKey  = "movie/tmdb/348/x/Th1aQ2"
	pbTrackKeyA = "track/mb/0c000000-0000-4000-8000-000000000003"
	pbTrackKeyB = "track/mb/0c000000-0000-4000-8000-000000000004"
	pbTrackKeyC = "track/local/Tl5Kp2"
)

// pbSize is the size of a copy of durationMs whose overall rate is kbps.
func pbSize(kbps, durationMs int64) int64 { return kbps * durationMs / 8 }

func playbackContextOf() playbackContext {
	const film = 8160000 // 2 h 16 min
	sdr := &playbackColour{Primaries: 1, Transfer: 1, Matrix: 1}
	avc := func(codecs string) *playbackVideoInfo {
		return &playbackVideoInfo{Codec: "h264", Codecs: codecs, Height: 1080, BitDepth: 8, Colour: sdr}
	}
	lc := func(id string, def bool) playbackTrackInfo {
		return playbackTrackInfo{ID: id, Codec: "aac", Codecs: "mp4a.40.2", Channels: 2, SampleRate: 48000, Default: def}
	}
	test := func(id string, v *playbackVideoInfo, a playbackTrackInfo) playbackCopy {
		return playbackCopy{ID: id, Parts: 1, Etag: "e" + id, Size: pbSize(5000, film), DurationMs: film, Video: v, Audio: []playbackTrackInfo{a}}
	}
	return playbackContext{
		Copies: map[string][]playbackCopy{
			pbFilmKey: {
				{ID: "Hd8kQ2mZ", Parts: 1, Etag: "9c1e4f70", Size: pbSize(8000, film), DurationMs: film,
					Video:     &playbackVideoInfo{Codec: "hevc", Codecs: "hvc1.2.4.L153.B0", Height: 2160, BitDepth: 10, Colour: &playbackColour{Primaries: 9, Transfer: 16, Matrix: 9}},
					Audio:     []playbackTrackInfo{{ID: "a1", Codec: "eac3", Codecs: "ec-3", Channels: 6, SampleRate: 48000, Default: true}, lc("a2", false), lc("a3", false)},
					Subtitles: []string{"s4", "s5"}},
				{ID: "Uk4nR7vT", Parts: 2, Etag: "2b7d00aa", Size: pbSize(40000, film), DurationMs: film, Video: avc("avc1.640028"),
					Audio: []playbackTrackInfo{lc("a1", false), {ID: "a2", Codec: "dts", Codecs: "dtsh", Channels: 6, SampleRate: 48000, Default: true}, {ID: "a3", Codec: "pcm_s24le", Channels: 2, SampleRate: 48000}}, Subtitles: []string{"s3"}},
				{ID: "Lw3pX9cJ", Parts: 1, Etag: "51f0c3d2", Size: pbSize(3000, film), DurationMs: film, Video: avc("avc1.640028"),
					Audio: []playbackTrackInfo{lc("a1", false), {ID: "a2", Codec: "aac", Codecs: "mp4a.40.5", Channels: 2, SampleRate: 48000}, {ID: "a3", Codec: "aac", Codecs: "mp4a.40.2", Channels: 6, SampleRate: 48000}}},
			},
			pbTestKey: {
				test("L41", avc("avc1.640029"), lc("a1", true)),
				test("L42", avc("avc1.64002A"), lc("a1", true)),
				test("Main", avc("avc1.4D401F"), lc("a1", true)),
				test("Avc3", avc("avc3.640028"), lc("a1", true)),
				test("Hi10", &playbackVideoInfo{Codec: "h264", Codecs: "avc1.6E0028", Height: 1080, BitDepth: 10, Colour: sdr}, lc("a1", true)),
				test("Intl", &playbackVideoInfo{Codec: "h264", Codecs: "avc1.640028", Height: 1080, BitDepth: 8, Interlaced: true, Colour: sdr}, lc("a1", true)),
				test("NoCol", &playbackVideoInfo{Codec: "h264", Codecs: "avc1.640028", Height: 1080, BitDepth: 8}, lc("a1", true)),
				test("P709", &playbackVideoInfo{Codec: "h264", Codecs: "avc1.640028", Height: 1080, BitDepth: 8, Colour: &playbackColour{Primaries: 1, Transfer: 1}}, lc("a1", true)),
				test("NoCodecs", &playbackVideoInfo{Codec: "h264", Height: 1080, BitDepth: 8, Colour: sdr}, lc("a1", true)),
				test("Hz96", avc("avc1.640028"), playbackTrackInfo{ID: "a1", Codec: "aac", Codecs: "mp4a.40.2", Channels: 2, SampleRate: 96000}),
				test("Mono", avc("avc1.640028"), playbackTrackInfo{ID: "a1", Codec: "aac", Codecs: "mp4a.40.2", Channels: 1, SampleRate: 44100}),
				test("Ac3", avc("avc1.640028"), playbackTrackInfo{ID: "a1", Codec: "ac3", Channels: 2, SampleRate: 48000}),
				test("Av1", &playbackVideoInfo{Codec: "av1", Codecs: "av01.0.08M.10", Height: 2160, BitDepth: 10, Colour: &playbackColour{Primaries: 9, Transfer: 16, Matrix: 9}}, playbackTrackInfo{ID: "a1", Codec: "opus", Channels: 2, SampleRate: 48000}),
			},
			"series/tmdb/1399/s1/e1":  {{ID: "Ep1aB2cD", Parts: 1, Etag: "e1e1e1e1", Size: pbSize(5000, 3600000), DurationMs: 3600000, Video: avc("avc1.640028"), Audio: []playbackTrackInfo{lc("a1", true)}}},
			"movie/tmdb/348/x/Tr7aQ1": {{ID: "Xt0aZ9", Parts: 1, Etag: "77aa", Size: pbSize(2000, 120000), DurationMs: 120000, Video: avc("avc1.64001F"), Audio: []playbackTrackInfo{lc("a1", false)}}},
			pbThemeKey:                {{ID: "Th0", Parts: 1, Etag: "7e7e", Size: pbSize(256, 90000), DurationMs: 90000, Audio: []playbackTrackInfo{{ID: "a0", Codec: "aac", Codecs: "mp4a.40.2", Channels: 2, SampleRate: 44100}}}},
			pbTrackKeyA:               {{ID: "Vq3kM0aZ7pXc", Parts: 1, Etag: "7f3a9c21", Size: pbSize(936, 240000), DurationMs: 240000, Audio: []playbackTrackInfo{{ID: "a0", Codec: "flac", Codecs: "fLaC", Channels: 2, SampleRate: 44100}}}},
			pbTrackKeyB: {
				{ID: "Tb1", Parts: 1, Etag: "b1b1", Size: pbSize(320, 200000), DurationMs: 200000, Audio: []playbackTrackInfo{{ID: "a0", Codec: "mp3", Channels: 2, SampleRate: 44100}}},
				{ID: "Tb2", Parts: 1, Etag: "b2b2", Size: pbSize(160, 200000), DurationMs: 200000, Audio: []playbackTrackInfo{{ID: "a0", Codec: "opus", Codecs: "Opus", Channels: 2, SampleRate: 48000}}},
			},
			pbTrackKeyC: {{ID: "Lt1", Parts: 1, Etag: "aa11", Size: pbSize(256, 180000), DurationMs: 180000, Audio: []playbackTrackInfo{{ID: "a0", Codec: "aac", Codecs: "mp4a.40.2", Channels: 2, SampleRate: 44100}}}},
		},
		VideoEncoders: []string{"h264", "hevc", "av1"},
		AudioEncoders: []string{"aac", "ac3", "eac3", "opus"},
		Ranges:        []string{"sdr", "keep"},
		SegmentCodecs: map[string][]string{
			"mp2t": {"h264", "hevc", "aac", "ac3", "eac3", "mp3"},
			"mp4":  {"h264", "hevc", "av1", "vp9", "aac", "ac3", "eac3", "dts", "opus", "flac", "alac", "mp3"},
		},
		ScanBytes:  33554432,
		Limits:     playbackLimits{Streams: 2, Transcodes: 1, Downloads: 1, DownloadsPerDay: 10, DownloadKbps: 20000, ScansPerDay: 1000},
		InUse:      playbackUse{ServerTranscodesFree: 4},
		StreamRoot: pbRoot,
		Now:        pbNow,
	}
}

// pbIn is a case before its outcome is worked out.
type pbIn struct {
	name, text string
	limits     *playbackLimits
	inUse      *playbackUse
	ranges     []string
	scanBytes  int64
	playback   *playbackState
	call       string
}

// pbJudge works out each case's outcome with the rules of playback.go.
func pbJudge(ctx playbackContext, in []pbIn) []playbackCase {
	var out []playbackCase
	for _, c := range in {
		pc := playbackCase{Name: c.name, Call: c.call, Limits: c.limits, InUse: c.inUse, Ranges: c.ranges, ScanBytes: c.scanBytes, Playback: c.playback, Text: c.text}
		if pc.Call == "" {
			pc.Call = "play"
		}
		got, err := pbRun(ctx, pc)
		pc.Valid, pc.Reason = reason(err)
		pc.Output = string(got)
		pc.Status, pc.Type = playbackStatus(err)
		out = append(out, pc)
	}
	return out
}

// pbRun applies the rule a case names; verify_rev_playback.go calls it too.
func pbRun(ctx playbackContext, c playbackCase) ([]byte, error) {
	if c.Call == "play" {
		lim, use := ctx.Limits, ctx.InUse
		if c.Limits != nil {
			lim = *c.Limits
		}
		if c.InUse != nil {
			use = *c.InUse
		}
		if c.Ranges != nil {
			ctx.Ranges = c.Ranges
		}
		if c.ScanBytes != 0 {
			ctx.ScanBytes = c.ScanBytes
		}
		return playbackPlay(ctx, lim, use, []byte(c.Text))
	}
	var pb playbackState
	if c.Playback != nil {
		pb = *c.Playback
	}
	return playbackReport(c.Call, ctx.Now, pb, []byte(c.Text))
}

func pbObj(members ...string) string { return "{" + strings.Join(members, ",") + "}" }

func pbLim(f func(*playbackLimits)) *playbackLimits {
	l := playbackContextOf().Limits
	f(&l)
	return &l
}

func pbUse(f func(*playbackUse)) *playbackUse {
	u := playbackContextOf().InUse
	f(&u)
	return &u
}

func playbackRequestCases() []pbIn {
	film := `"title":"` + pbFilmKey + `"`
	play := func(name string, members ...string) pbIn { return pbIn{name: name, text: pbObj(members...)} }
	return []pbIn{
		play("the title alone: the baseline, the first copy and its default audio, no bitrate cap", film),
		play("every member of playback and video", film, `"purpose":"play"`, `"delivery":"transcode"`, `"transcode":{"videoCodec":"h264","maxHeight":1080,"maxBitrateKbps":8000,"audioCodec":"aac","audioChannels":2,"range":"sdr","burnSubtitle":"s4"}`, `"audioRenditions":true`, `"startPositionMs":60000`, `"maxBitrateKbps":20000`, `"versionId":"Hd8kQ2mZ"`, `"audioTrack":"a2"`),
		play("an episode by its numbers", `"title":"series/tmdb/1399/s1/e1"`),
		play("an extra", `"title":"movie/tmdb/348/x/Tr7aQ1"`),
		play("an extra with no video, a theme song: the baseline is audio only", `"title":"`+pbThemeKey+`"`),
		play("a track of the mb scheme: the baseline is audio only", `"title":"`+pbTrackKeyA+`"`),
		play("a track of the local scheme", `"title":"`+pbTrackKeyC+`"`),
		play("unknown members ignored", film, `"Title":"x"`, `"quality":"best"`, `"device":null`),
		play("capabilities are an unknown member: ignored, whatever they hold", film, `"capabilities":{"containers":["mp4"],"videoCodecs":"h264","maxHeight":-1}`),
		play("audioLanguage and subtitleLanguage are unknown members now: ignored, whatever they hold", film, `"audioLanguage":"fr-CA"`, `"subtitleLanguage":7`),
		play("subtitleTrack is an unknown member now: ignored, whatever it holds", film, `"subtitleTrack":"s007"`),
		play("container is an unknown member now: ignored, whatever it holds", film, `"container":"MP4"`),
		play("a key of a kind play takes, not offered: 404", `"title":"movie/tmdb/603"`),
		play("an episode by TMDB's episode ID, not offered: 404", `"title":"series/tmdb/1399/e63056"`),
		play("a local film, not offered: 404", `"title":"movie/local/q3Zp0Lw8"`),
		play("a series: 400", `"title":"series/tmdb/1399"`),
		play("a season: 400", `"title":"series/tmdb/1399/s1"`),
		play("an album: 400", `"title":"album/mb/0b000000-0000-4000-8000-000000000002"`),
		play("an artist: 400", `"title":"artist/local/Ar7"`),
		play("a key with a leading zero: 400", `"title":"series/tmdb/1399/s01/e1"`),
		play("a TMDB ID above 9,999,999,999: 400", `"title":"movie/tmdb/10000000000"`),
		play("an mb key in capitals: 400", `"title":"track/mb/0C000000-0000-4000-8000-000000000003"`),
		play("no title: 400", `"purpose":"play"`),
		play("a title that isn't a string: 400", `"title":949`),
		play("a null title: 400", `"title":null`),
		{name: "a body that isn't an object: 400", text: `["movie/tmdb/949"]`},
		{name: "a member twice: not I-JSON, 400", text: `{"title":"movie/tmdb/949","title":"movie/tmdb/603"}`},
		play("purpose play", film, `"purpose":"play"`),
		play("purpose in capitals: 400", film, `"purpose":"Play"`),
		play("an unknown purpose, a token: 400 (purpose is closed)", film, `"purpose":"stream"`),
		play("a purpose that isn't a string: 400", film, `"purpose":1`),
		play("an unknown delivery, a token: 409 delivery-refused", film, `"delivery":"direct"`),
		play("an unknown delivery with purpose download: 409 delivery-refused", film, `"purpose":"download"`, `"delivery":"dash"`),
		{name: "an unknown delivery with streams all in use: 409 stream-limit-reached (step 5 before step 7)", text: pbObj(film, `"delivery":"direct"`), inUse: pbUse(func(u *playbackUse) { u.Streams = 2 })},
		play("a delivery in capitals, not a token: 400", film, `"delivery":"FILE"`),
		play("a delivery of 33 characters, not a token: 400", film, `"delivery":"`+strings.Repeat("d", 33)+`"`),
		play("an empty delivery, not a token: 400", film, `"delivery":""`),
		play("a delivery that isn't a string: 400", film, `"delivery":["file"]`),
		play("play with delivery file", film, `"purpose":"play"`, `"delivery":"file"`),
		play("play with delivery remux", film, `"delivery":"remux"`, `"mediaType":"video/mp4"`),
		play("play with delivery transcode: every target left out takes the baseline's", film, `"delivery":"transcode"`),
		play("download without delivery: 400", film, `"purpose":"download"`),
		play("download with delivery file", film, `"purpose":"download"`, `"delivery":"file"`),
		play("download with delivery remux: 400", film, `"purpose":"download"`, `"delivery":"remux"`, `"mediaType":"video/mp2t"`),
		play("download with a transcode naming no size: 400", film, `"purpose":"download"`, `"delivery":"transcode"`, `"transcode":{"videoCodec":"h264"}`),
		play("download with a transcode naming maxHeight", film, `"purpose":"download"`, `"delivery":"transcode"`, `"transcode":{"maxHeight":720}`),
		play("scan without delivery: 400", film, `"purpose":"scan"`),
		play("scan with delivery file", film, `"purpose":"scan"`, `"delivery":"file"`),
		play("scan with delivery remux: 400", film, `"purpose":"scan"`, `"delivery":"remux"`, `"mediaType":"video/mp2t"`),
		play("scan with delivery transcode: 400", film, `"purpose":"scan"`, `"delivery":"transcode"`, `"transcode":{"maxHeight":480}`),
		play("audioRenditions true: a request, never refused", film, `"audioRenditions":true`),
		play("audioRenditions false", film, `"audioRenditions":false`),
		play("audioRenditions that isn't a boolean: 400", film, `"audioRenditions":"true"`),
		play("startPositionMs 0", film, `"startPositionMs":0`),
		play("startPositionMs at 100 days", film, `"startPositionMs":8640000000`),
		play("startPositionMs above 100 days: 400", film, `"startPositionMs":8640000001`),
		play("a negative startPositionMs: 400", film, `"startPositionMs":-1`),
		play("startPositionMs with a fraction: 400", film, `"startPositionMs":1.0`),
		play("startPositionMs as a string: 400", film, `"startPositionMs":"0"`),
		play("maxBitrateKbps 0: none, no cap", film, `"maxBitrateKbps":0`),
		play("maxBitrateKbps at 10,000,000", film, `"maxBitrateKbps":10000000`),
		play("maxBitrateKbps above 10,000,000: 400", film, `"maxBitrateKbps":10000001`),
		play("maxBitrateKbps below 1,000, kept as it is", film, `"maxBitrateKbps":500`),
		play("maxBitrateKbps not a whole 1,000, kept as it is", film, `"maxBitrateKbps":25999`),
		{name: "the lowest non-zero cap wins: the pairing's", text: pbObj(film, `"maxBitrateKbps":20000`), limits: pbLim(func(l *playbackLimits) { l.BitrateKbps = 15000 })},
		play("audioTrack names a track of the copy: that track", film, `"audioTrack":"a3"`),
		play("audioTrack a0, not a track of the copy: as if absent, the default audio", film, `"audioTrack":"a0"`),
		play("audioTrack a999", film, `"audioTrack":"a999"`),
		play("audioTrack a1000: 400", film, `"audioTrack":"a1000"`),
		play("audioTrack with a leading zero: 400", film, `"audioTrack":"a01"`),
		play("audioTrack in capitals: 400", film, `"audioTrack":"A1"`),
		play("audioTrack naming a subtitle: 400", film, `"audioTrack":"s4"`),
		play("an empty audioTrack: 400", film, `"audioTrack":""`),
		play("audioTrack that isn't a string: 400", film, `"audioTrack":1`),
		play("order: a malformed audioTrack of a title not offered is 404 (step 3 before step 4)", `"title":"movie/tmdb/603"`, `"audioTrack":"a01"`),
		play("order: a malformed burnSubtitle of a title not offered is 404 (step 3 before step 4)", `"title":"movie/tmdb/603"`, `"delivery":"transcode"`, `"transcode":{"burnSubtitle":"s04"}`),
		{name: "order: a malformed audioTrack with streams 0 is 409 (step 2 before step 4)", text: pbObj(film, `"audioTrack":"a01"`), limits: pbLim(func(l *playbackLimits) { l.Streams = 0 })},
		{name: "order: a malformed burnSubtitle with streams 0 is 409 (step 2 before step 4)", text: pbObj(film, `"delivery":"transcode"`, `"transcode":{"burnSubtitle":4}`), limits: pbLim(func(l *playbackLimits) { l.Streams = 0 })},
		{name: "order: a bad startPositionMs with streams 0 is 400 (step 1 first)", text: pbObj(film, `"startPositionMs":-1`), limits: pbLim(func(l *playbackLimits) { l.Streams = 0 })},
		{name: "order: a delivery that isn't a token with streams 0 is 400 (step 1 first)", text: pbObj(film, `"delivery":"File"`), limits: pbLim(func(l *playbackLimits) { l.Streams = 0 })},
		{name: "order: a malformed mediaType with streams 0 is 400 (step 1 first)", text: pbObj(film, `"delivery":"remux"`, `"mediaType":"MP4"`), limits: pbLim(func(l *playbackLimits) { l.Streams = 0 })},
		{name: "order: an unknown delivery with streams 0 is 409 stream-limit-reached (step 2 before step 7)", text: pbObj(film, `"delivery":"direct"`), limits: pbLim(func(l *playbackLimits) { l.Streams = 0 })},
		{name: "order: a title not offered with streams 0 is 409 (step 2 before step 3)", text: pbObj(`"title":"movie/tmdb/603"`), limits: pbLim(func(l *playbackLimits) { l.Streams = 0 })},
		{name: "streams 0: 409 stream-limit-reached", text: pbObj(film), limits: pbLim(func(l *playbackLimits) { l.Streams = 0 })},
		{name: "streams all in use: 409 stream-limit-reached", text: pbObj(film), inUse: pbUse(func(u *playbackUse) { u.Streams = 2 })},
		{name: "one stream left", text: pbObj(film), inUse: pbUse(func(u *playbackUse) { u.Streams = 1 })},
		{name: "downloads all in use: play is not concerned", text: pbObj(film), inUse: pbUse(func(u *playbackUse) { u.Downloads = 1 })},
		play("versionId names a copy: it plays", film, `"versionId":"Lw3pX9cJ"`),
		play("versionId names no copy: the first copy", film, `"versionId":"zzz"`),
		play("versionId of any form, naming no copy: the first copy", film, `"versionId":"../x"`),
		play("versionId that isn't a string: 400", film, `"versionId":7`),
		play("versionId and audioTrack named", film, `"versionId":"Uk4nR7vT"`, `"audioTrack":"a1"`, `"delivery":"remux"`, `"mediaType":"video/mp2t"`),
		play("a copy whose default audio isn't its first track", film, `"versionId":"Uk4nR7vT"`, `"delivery":"transcode"`),
		play("a copy with no track marked default: its first audio track", film, `"versionId":"Lw3pX9cJ"`),
		play("audioTrack not of the copy named: as if absent, the default audio", film, `"versionId":"Lw3pX9cJ"`, `"audioTrack":"a7"`),
	}
}

func playbackDeliveryCases() []pbIn {
	film := `"title":"` + pbFilmKey + `"`
	theme := `"title":"` + pbThemeKey + `"`
	track := `"title":"` + pbTrackKeyA + `"`
	parts := `"versionId":"Uk4nR7vT"`
	sdrCopy := `"versionId":"Lw3pX9cJ"`
	play := func(name string, members ...string) pbIn {
		return pbIn{name: name, text: pbObj(append([]string{film}, members...)...)}
	}
	of := func(name, title string, members ...string) pbIn {
		return pbIn{name: name, text: pbObj(append([]string{title}, members...)...)}
	}
	tc := func(name, targets string, extra ...string) pbIn {
		return play(name, append([]string{`"delivery":"transcode"`, `"transcode":` + targets}, extra...)...)
	}
	allUsed := pbUse(func(u *playbackUse) { u.Transcodes = 1; u.ServerTranscodesFree = 0 })
	return []pbIn{
		play("file of a copy of one file", `"delivery":"file"`),
		play("file of a copy of several parts: 409 delivery-refused", `"delivery":"file"`, parts),
		play("file of a copy of several parts, for a download: 409 delivery-refused", `"purpose":"download"`, `"delivery":"file"`, parts),
		play("file of a copy of several parts, for a scan: 409 delivery-refused", `"purpose":"scan"`, `"delivery":"file"`, parts),
		play("remux of a copy of several parts", `"delivery":"remux"`, `"mediaType":"video/mp2t"`, `"audioTrack":"a1"`, parts),
		play("transcode of a copy of several parts", `"delivery":"transcode"`, parts),
		play("no delivery, a copy of several parts: the baseline", parts),
		play("file at a cap equal to the copy's overall rate (size × 8 / durationMs)", `"delivery":"file"`, `"maxBitrateKbps":8000`),
		play("file at a cap below the copy's overall rate: 409 delivery-refused", `"delivery":"file"`, `"maxBitrateKbps":7999`),
		play("a remux of 40,000 kbit/s with no cap: no ceiling of its own", `"delivery":"remux"`, `"mediaType":"video/mp4"`, parts),
		play("remux above the cap: 409 delivery-refused", `"delivery":"remux"`, `"mediaType":"video/mp4"`, parts, `"maxBitrateKbps":20000`),
		{name: "file above the pairing's bitrateKbps: 409 delivery-refused", text: pbObj(film, `"delivery":"file"`), limits: pbLim(func(l *playbackLimits) { l.BitrateKbps = 5000 })},
		{name: "the pairing's bitrateKbps is a cap: the lowest", text: pbObj(film, `"maxBitrateKbps":20000`), limits: pbLim(func(l *playbackLimits) { l.BitrateKbps = 9000 })},
		play("a transcode above the cap is not refused for bandwidth", `"delivery":"transcode"`, `"maxBitrateKbps":2000`),
		play("no delivery above the cap: the baseline", `"maxBitrateKbps":2000`),
		// Remux media types (section 2.7).
		play("remux without mediaType: 400", `"delivery":"remux"`),
		play("mediaType with delivery file: 400", `"delivery":"file"`, `"mediaType":"video/mp4"`),
		play("mediaType without delivery: 400", `"mediaType":"video/mp4"`),
		play("mediaType with an unknown delivery: 400", `"delivery":"dash"`, `"mediaType":"video/mp4"`),
		play("mediaType video/mp2t (MPEG-TS segments)", `"delivery":"remux"`, `"mediaType":"video/mp2t"`),
		play("mediaType video/mp4 (fragmented MP4 segments)", `"delivery":"remux"`, `"mediaType":"video/mp4"`),
		play("mediaType video/matroska, a media type: 409 delivery-refused", `"delivery":"remux"`, `"mediaType":"video/matroska"`),
		play("mediaType video/vnd.dlna.mpeg-tts, another subtype: 409 delivery-refused", `"delivery":"remux"`, `"mediaType":"video/vnd.dlna.mpeg-tts"`),
		play("mediaType audio/mp4 for a copy with video: 409 delivery-refused", `"delivery":"remux"`, `"mediaType":"audio/mp4"`),
		play("mediaType application/mp4: 409 delivery-refused", `"delivery":"remux"`, `"mediaType":"application/mp4"`),
		play("mediaType mpegts, the first version's token, no media type: 400", `"delivery":"remux"`, `"mediaType":"mpegts"`),
		play("mediaType in capitals: 400", `"delivery":"remux"`, `"mediaType":"video/MP4"`),
		play("mediaType with a parameter: 400", `"delivery":"remux"`, `"mediaType":"video/mp4; codecs=avc1"`),
		play("mediaType with two slashes: 400", `"delivery":"remux"`, `"mediaType":"video/mp4/x"`),
		play("mediaType whose subtype starts with a dot: 400", `"delivery":"remux"`, `"mediaType":"video/.mp4"`),
		play("mediaType of 128 characters: 400", `"delivery":"remux"`, `"mediaType":"video/`+strings.Repeat("x", 122)+`"`),
		play("an empty mediaType: 400", `"delivery":"remux"`, `"mediaType":""`),
		play("mediaType that isn't a string: 400", `"delivery":"remux"`, `"mediaType":["video/mp4"]`),
		play("mediaType video/mp2t of a copy whose audio is DTS: the segment format can't carry it, 409 delivery-refused", `"delivery":"remux"`, `"mediaType":"video/mp2t"`, parts),
		of("a theme song, remux to audio/mp4", theme, `"delivery":"remux"`, `"mediaType":"audio/mp4"`),
		of("a theme song, remux to audio/mp2t", theme, `"delivery":"remux"`, `"mediaType":"audio/mp2t"`),
		of("a theme song, remux to video/mp4: 409 delivery-refused", theme, `"delivery":"remux"`, `"mediaType":"video/mp4"`),
		of("a track of FLAC, remux to audio/mp4", track, `"delivery":"remux"`, `"mediaType":"audio/mp4"`),
		of("a track of FLAC, remux to audio/mp2t: 409 delivery-refused", track, `"delivery":"remux"`, `"mediaType":"audio/mp2t"`),
		// Transcode targets.
		tc("transcode, every target", `{"videoCodec":"hevc","maxHeight":2160,"maxBitrateKbps":30000,"audioCodec":"eac3","audioChannels":6,"range":"keep","burnSubtitle":"s5"}`),
		tc("transcode, no targets: each the baseline's (h264, the copy's height, aac, 2 channels, sdr)", `{}`),
		tc("transcode, only maxHeight: the others the baseline's", `{"maxHeight":720}`),
		tc("transcode, unknown members ignored", `{"maxHeight":720,"preset":"fast"}`),
		tc("transcode videoCodec av1, an encoder the owning server has", `{"videoCodec":"av1"}`),
		tc("transcode videoCodec vvc, a token it doesn't encode: 409 delivery-refused", `{"videoCodec":"vvc"}`),
		tc("transcode videoCodec h265, a token it doesn't know: 409 delivery-refused", `{"videoCodec":"h265"}`),
		tc("transcode videoCodec in capitals, not a token: 400", `{"videoCodec":"H264"}`),
		tc("transcode videoCodec of 33 characters, not a token: 400", `{"videoCodec":"`+strings.Repeat("h", 33)+`"}`),
		tc("transcode videoCodec that isn't a string: 400", `{"videoCodec":264}`),
		tc("transcode audioCodec opus", `{"audioCodec":"opus"}`),
		tc("transcode audioCodec flac, a token it doesn't encode: 409 delivery-refused", `{"audioCodec":"flac"}`),
		tc("transcode audioCodec in capitals, not a token: 400", `{"audioCodec":"AAC"}`),
		tc("transcode maxHeight 480", `{"maxHeight":480}`),
		tc("transcode maxHeight 4320", `{"maxHeight":4320}`),
		tc("transcode maxHeight not of the six: 400", `{"maxHeight":1000}`),
		tc("transcode maxHeight 0: 400", `{"maxHeight":0}`),
		tc("transcode maxBitrateKbps 1,000", `{"maxBitrateKbps":1000}`),
		tc("transcode maxBitrateKbps 10,000,000", `{"maxBitrateKbps":10000000}`),
		tc("transcode maxBitrateKbps below 1,000: 400", `{"maxBitrateKbps":999}`),
		tc("transcode maxBitrateKbps not a whole 1,000: 400", `{"maxBitrateKbps":1500}`),
		tc("transcode maxBitrateKbps above 10,000,000: 400", `{"maxBitrateKbps":10001000}`),
		tc("transcode maxBitrateKbps is a cap: the lowest wins", `{"maxBitrateKbps":4000}`, `"maxBitrateKbps":20000`),
		tc("transcode audioChannels 2", `{"audioChannels":2}`),
		tc("transcode audioChannels 8", `{"audioChannels":8}`),
		tc("transcode audioChannels not 2, 6 or 8: 400", `{"audioChannels":5}`),
		tc("transcode audioChannels as a string: 400", `{"audioChannels":"6"}`),
		tc("burnSubtitle a track of the copy", `{"burnSubtitle":"s4"}`),
		tc("burnSubtitle not a track of the copy: 409 delivery-refused", `{"burnSubtitle":"s9"}`),
		tc("burnSubtitle with a leading zero: 400", `{"burnSubtitle":"s04"}`),
		tc("burnSubtitle off: 400", `{"burnSubtitle":"off"}`),
		tc("burnSubtitle a number: 400", `{"burnSubtitle":4}`),
		tc("burnSubtitle null: 400", `{"burnSubtitle":null}`),
		tc("a transcode that isn't an object: 400", `"h264"`),
		play("a transcode object with delivery file: 400", `"delivery":"file"`, `"transcode":{}`),
		play("a transcode object without delivery: 400", `"transcode":{"maxHeight":720}`),
		// copy (section 2.7).
		tc("videoCodec copy: the picture unchanged, maxHeight and range not taken; the audio converted, a transcode", `{"videoCodec":"copy","maxHeight":720,"range":"sdr","audioCodec":"aac"}`),
		{name: "videoCodec and audioCodec copy: nothing converted, no transcode taken", text: pbObj(film, `"delivery":"transcode"`, `"transcode":{"videoCodec":"copy","audioCodec":"copy","audioChannels":2}`), inUse: allUsed},
		{name: "videoCodec copy, the audio converted, the transcodes all used: 503 server-busy", text: pbObj(film, `"delivery":"transcode"`, `"transcode":{"videoCodec":"copy"}`), inUse: allUsed},
		tc("audioCodec copy: the audio unchanged, audioChannels not taken", `{"audioCodec":"copy","audioChannels":6}`, `"audioTrack":"a1"`),
		tc("videoCodec copy with burnSubtitle: 409 delivery-refused", `{"videoCodec":"copy","burnSubtitle":"s4"}`),
		tc("audioCodec copy of a PCM track no segment format carries: 409 delivery-refused", `{"audioCodec":"copy"}`, parts, `"audioTrack":"a3"`),
		tc("audioCodec copy of a DTS track: mp4 segments carry it", `{"audioCodec":"copy"}`, parts, `"audioTrack":"a2"`),
		of("videoCodec copy of AV1 with Opus copied: mp4 segments carry both", `"title":"`+pbTestKey+`"`, `"versionId":"Av1"`, `"delivery":"transcode"`, `"transcode":{"videoCodec":"copy","audioCodec":"copy"}`),
		tc("videoCodec Copy in capitals: 400", `{"videoCodec":"Copy"}`),
		of("a track, audioCodec copy: counts as a transcode (the rule frees only video copy)", track, `"delivery":"transcode"`, `"transcode":{"audioCodec":"copy"}`),
		// range (section 2.7).
		tc("range keep: the copy's transfer and primaries kept", `{"videoCodec":"hevc","range":"keep"}`),
		tc("range sdr: tone-mapped", `{"range":"sdr"}`),
		tc("range hdr10, a token not of its list: 409 delivery-refused", `{"range":"hdr10"}`),
		tc("range in capitals, not a token: 400", `{"range":"SDR"}`),
		tc("range that isn't a string: 400", `{"range":1}`),
		{name: "range keep from an owning server that can't keep it: 409 delivery-refused", text: pbObj(film, `"delivery":"transcode"`, `"transcode":{"range":"keep"}`), ranges: []string{"sdr"}},
		{name: "range sdr from an owning server that makes sdr only", text: pbObj(film, `"delivery":"transcode"`, `"transcode":{"range":"sdr"}`), ranges: []string{"sdr"}},
		tc("range with video copy: ignored, even one not of its list", `{"videoCodec":"copy","range":"hdr10"}`),
		of("a track, transcode with range keep: ignored for a copy with no video", track, `"delivery":"transcode"`, `"transcode":{"range":"keep","audioCodec":"opus"}`),
		// Limits and order.
		{name: "transcode with the pairing's transcodes all used: 503 server-busy", text: pbObj(film, `"delivery":"transcode"`), inUse: pbUse(func(u *playbackUse) { u.Transcodes = 1 })},
		{name: "transcode with transcodes 0: 503 server-busy", text: pbObj(film, `"delivery":"transcode"`), limits: pbLim(func(l *playbackLimits) { l.Transcodes = 0 })},
		{name: "transcode with the server's budget used: 503 server-busy", text: pbObj(film, `"delivery":"transcode"`), inUse: pbUse(func(u *playbackUse) { u.ServerTranscodesFree = 0 })},
		{name: "a codec it doesn't encode with the transcodes all used: 409 delivery-refused (step 7 before step 8)", text: pbObj(film, `"delivery":"transcode"`, `"transcode":{"videoCodec":"vvc"}`), inUse: pbUse(func(u *playbackUse) { u.Transcodes = 1 })},
		{name: "remux with the transcodes all used: no transcode needed", text: pbObj(film, `"delivery":"remux"`, `"mediaType":"video/mp2t"`), inUse: allUsed},
		{name: "file with the transcodes all used: no transcode needed", text: pbObj(film, `"delivery":"file"`), inUse: allUsed},
		{name: "streams all in use before delivery: 409 stream-limit-reached", text: pbObj(film, `"delivery":"file"`, parts), inUse: pbUse(func(u *playbackUse) { u.Streams = 2 })},
		{name: "a media type it doesn't make with streams all in use: 409 stream-limit-reached (step 5 before step 7)", text: pbObj(film, `"delivery":"remux"`, `"mediaType":"video/matroska"`), inUse: pbUse(func(u *playbackUse) { u.Streams = 2 })},
		of("a track, file", track, `"delivery":"file"`),
		of("a track, transcode: video targets ignored for a copy with no video, an unknown one too", track, `"delivery":"transcode"`, `"transcode":{"videoCodec":"vvc","maxHeight":720,"audioCodec":"aac"}`),
		of("a track, transcode with no targets: aac, 2 channels", track, `"delivery":"transcode"`),
		of("a theme song, transcode: video targets ignored", theme, `"delivery":"transcode"`, `"transcode":{"videoCodec":"hevc","audioCodec":"opus"}`),
		play("a copy within the baseline, transcode named: still a transcode", `"delivery":"transcode"`, sdrCopy),
	}
}

// playbackBaselineCases are plays without delivery: what the baseline
// leaves unchanged, and what it converts (section 2.7).
func playbackBaselineCases() []pbIn {
	of := func(name, title, versionID string, extra ...string) pbIn {
		m := []string{`"title":"` + title + `"`}
		if versionID != "" {
			m = append(m, `"versionId":"`+versionID+`"`)
		}
		return pbIn{name: name, text: pbObj(append(m, extra...)...)}
	}
	allUsed := pbUse(func(u *playbackUse) { u.Transcodes = 1; u.ServerTranscodesFree = 0 })
	within := of("H.264 High 4.0, 8-bit, SDR in BT.709, AAC-LC stereo at 48 kHz: left unchanged, a remux", pbFilmKey, "Lw3pX9cJ")
	within.inUse = allUsed
	within.name += ", no transcode taken"
	music := of("a track of AAC-LC stereo at 44.1 kHz: left unchanged, no transcode taken", pbTrackKeyC, "")
	music.inUse = allUsed
	busy := of("HEVC with the transcodes all used: 503 server-busy", pbFilmKey, "")
	busy.inUse = allUsed
	return []pbIn{
		within,
		of("HEVC in PQ, E-AC-3 5.1: both converted, a transcode", pbFilmKey, ""),
		busy,
		of("its audio track a2, HE-AAC: the audio converted", pbFilmKey, "Lw3pX9cJ", `"audioTrack":"a2"`),
		of("its audio track a3, AAC-LC in 5.1: the audio converted", pbFilmKey, "Lw3pX9cJ", `"audioTrack":"a3"`),
		of("level 4.1: within", pbTestKey, "L41"),
		of("level 4.2: the picture converted", pbTestKey, "L42"),
		of("Main profile, level 3.1: within", pbTestKey, "Main"),
		of("avc3, High 4.0: within", pbTestKey, "Avc3"),
		of("High 10: the picture converted", pbTestKey, "Hi10"),
		of("interlaced: the picture converted", pbTestKey, "Intl"),
		of("no colour: not known to be BT.709, the picture converted", pbTestKey, "NoCol"),
		of("no matrix: not known to be BT.709, the picture converted", pbTestKey, "P709"),
		of("no codecs: not known to be within, the picture converted", pbTestKey, "NoCodecs"),
		of("AAC at 96 kHz: the audio converted", pbTestKey, "Hz96"),
		of("AAC-LC mono at 44.1 kHz: within", pbTestKey, "Mono"),
		of("AC-3 stereo: the audio converted", pbTestKey, "Ac3"),
		of("AV1 and Opus: both converted", pbTestKey, "Av1"),
		of("an extra with no video, a theme song of AAC-LC: audio only, left unchanged", pbThemeKey, ""),
		music,
		of("a track of FLAC: converted to AAC, audio only", pbTrackKeyA, ""),
	}
}

func playbackQueueEntryCases() []playbackQueueCase {
	req := func(members ...string) string {
		return pbObj(append([]string{`"title":"` + pbTrackKeyA + `"`, `"queue":[{"title":"` + pbTrackKeyB + `"},{"title":"` + pbTrackKeyC + `"}]`}, members...)...)
	}
	entry := func(members ...string) string {
		return pbObj(append([]string{`"title":"` + pbTrackKeyB + `"`}, members...)...)
	}
	opus := req(`"delivery":"transcode"`, `"transcode":{"audioCodec":"opus"}`)
	list := []struct{ name, request, entry string }{
		{"an entry with its title alone: the request's delivery, its first copy", opus, entry()},
		{"an entry naming its copy", opus, entry(`"versionId":"Tb2"`)},
		{"an entry naming no copy of its own: its first", opus, entry(`"versionId":"Zz9"`)},
		{"an entry with its own delivery file", opus, entry(`"delivery":"file"`)},
		{"an entry with its own remux to audio/mp4", opus, entry(`"versionId":"Tb2"`, `"delivery":"remux"`, `"mediaType":"audio/mp4"`)},
		{"an entry with its own transcode and targets", req(`"delivery":"file"`), entry(`"delivery":"transcode"`, `"transcode":{"audioCodec":"aac","audioChannels":2}`)},
		{"an entry with its own delivery takes none of the request's targets", opus, entry(`"delivery":"transcode"`)},
		{"an entry with its own delivery: the request's mediaType isn't taken", req(`"delivery":"remux"`, `"mediaType":"audio/mp4"`), entry(`"delivery":"transcode"`)},
		{"an entry without delivery, the request's remux: its mediaType too", req(`"delivery":"remux"`, `"mediaType":"audio/mp4"`), entry()},
		{"an entry without delivery, the request without: the baseline", req(), entry()},
		{"an entry's remux without mediaType: 400", opus, entry(`"delivery":"remux"`)},
		{"an entry's mediaType without its own delivery: 400 (the request's transcode taken)", opus, entry(`"mediaType":"audio/mp4"`)},
		{"an entry's unknown delivery: 409 delivery-refused", opus, entry(`"delivery":"dash"`)},
		{"an entry's remux to audio/mp2t of Opus: 409 delivery-refused", opus, entry(`"versionId":"Tb2"`, `"delivery":"remux"`, `"mediaType":"audio/mp2t"`)},
		{"an entry's versionId that isn't a string: 400", opus, entry(`"versionId":2`)},
		{"an entry's delivery that isn't a token: 400", opus, entry(`"delivery":"File"`)},
		{"an entry's audioTrack, not an entry member: ignored", opus, entry(`"audioTrack":"a01"`)},
		{"an entry without title: 400", opus, `{"versionId":"Tb2"}`},
		{"an entry that isn't an object: 400", opus, `"` + pbTrackKeyB + `"`},
		{"an entry of the request's purpose, a download: file", req(`"purpose":"download"`, `"delivery":"file"`), entry()},
		{"an entry whose own delivery its purpose doesn't allow: 400", req(`"purpose":"download"`, `"delivery":"file"`), entry(`"delivery":"remux"`, `"mediaType":"audio/mp4"`)},
	}
	ctx := playbackContextOf()
	var out []playbackQueueCase
	for _, c := range list {
		got, err := playbackQueueEntry(ctx, ctx.Limits, ctx.InUse, []byte(c.request), []byte(c.entry))
		ok, r := reason(err)
		st, typ := playbackStatus(err)
		out = append(out, playbackQueueCase{Name: c.name, Request: c.request, Entry: c.entry, Valid: ok, Status: st, Type: typ, Output: string(got), Reason: r})
	}
	return out
}

func playbackDownloadCases() []pbIn {
	film := `"title":"` + pbFilmKey + `"`
	dl := func(members ...string) string {
		return pbObj(append([]string{film, `"purpose":"download"`}, members...)...)
	}
	file := dl(`"delivery":"file"`)
	ended := func(playedAt int64) *playbackState {
		return &playbackState{State: "ended", Purpose: "download", Title: pbFilmKey, PlayedAt: playedAt}
	}
	at := func(t int64, extra ...string) string {
		return pbObj(append([]string{`"id":"` + pbPID + `"`, `"positionMs":1800000`, `"at":` + strconv.FormatInt(t, 10)}, extra...)...)
	}
	noAt := pbObj(`"id":"`+pbPID+`"`, `"positionMs":1800000`)
	return []pbIn{
		{name: "file: speed cap downloadKbps, 24-hour ticket, no idle rule", text: file},
		{name: "startPositionMs ignored: served from the beginning", text: dl(`"delivery":"file"`, `"startPositionMs":600000`)},
		{name: "transcode to maxHeight: the server's budget, not the pairing's transcodes", text: dl(`"delivery":"transcode"`, `"transcode":{"maxHeight":720}`)},
		{name: "transcode to maxBitrateKbps", text: dl(`"delivery":"transcode"`, `"transcode":{"maxBitrateKbps":4000}`)},
		{name: "transcode to both, with other targets", text: dl(`"delivery":"transcode"`, `"transcode":{"maxHeight":1080,"maxBitrateKbps":6000,"audioCodec":"aac","audioChannels":2}`)},
		{name: "transcode without a size: 400", text: dl(`"delivery":"transcode"`, `"transcode":{}`)},
		{name: "transcode without a transcode object: 400", text: dl(`"delivery":"transcode"`)},
		{name: "transcode naming only an audio target: 400", text: dl(`"delivery":"transcode"`, `"transcode":{"audioCodec":"aac"}`)},
		{name: "no delivery: 400", text: dl()},
		{name: "remux: 400", text: dl(`"delivery":"remux"`, `"mediaType":"video/mp4"`)},
		{name: "a copy of several parts as file: 409 delivery-refused", text: dl(`"delivery":"file"`, `"versionId":"Uk4nR7vT"`)},
		{name: "a copy of several parts, transcoded", text: dl(`"delivery":"transcode"`, `"versionId":"Uk4nR7vT"`, `"transcode":{"maxHeight":1080}`)},
		{name: "no bitrate cap of play: a file above maxBitrateKbps", text: dl(`"delivery":"file"`, `"maxBitrateKbps":1000`)},
		{name: "downloads 0: 409 stream-limit-reached", text: file, limits: pbLim(func(l *playbackLimits) { l.Downloads = 0 })},
		{name: "downloads 0, bad audioTrack: 409 (step 2 before step 4)", text: dl(`"delivery":"file"`, `"audioTrack":"a01"`), limits: pbLim(func(l *playbackLimits) { l.Downloads = 0 })},
		{name: "downloads all in use: 409 stream-limit-reached", text: file, inUse: pbUse(func(u *playbackUse) { u.Downloads = 1 })},
		{name: "downloadsPerDay reached: 409 stream-limit-reached", text: file, inUse: pbUse(func(u *playbackUse) { u.DownloadsToday = 10 })},
		{name: "one download left today", text: file, inUse: pbUse(func(u *playbackUse) { u.DownloadsToday = 9 })},
		{name: "downloadsPerDay 0: no daily cap besides downloads", text: file, limits: pbLim(func(l *playbackLimits) { l.DownloadsPerDay = 0 }), inUse: pbUse(func(u *playbackUse) { u.DownloadsToday = 500 })},
		{name: "downloadKbps 0: no speed cap", text: file, limits: pbLim(func(l *playbackLimits) { l.DownloadKbps = 0 })},
		{name: "streams all in use: a download doesn't count against streams", text: file, inUse: pbUse(func(u *playbackUse) { u.Streams = 2 })},
		{name: "streams 0: a download doesn't count against streams", text: file, limits: pbLim(func(l *playbackLimits) { l.Streams = 0; l.Transcodes = 0 })},
		{name: "transcode with the pairing's transcodes all used: not concerned", text: dl(`"delivery":"transcode"`, `"transcode":{"maxHeight":720}`), inUse: pbUse(func(u *playbackUse) { u.Transcodes = 1 })},
		{name: "transcode with the server's budget used: 503 server-busy", text: dl(`"delivery":"transcode"`, `"transcode":{"maxHeight":720}`), inUse: pbUse(func(u *playbackUse) { u.ServerTranscodesFree = 0 })},
		{name: "late progress with at, 10 days after the play: recorded, stays ended", call: "progress", playback: ended(pbNow - 10*86400), text: at(pbNow - 9*86400)},
		{name: "late stop with at: recorded, stays ended", call: "stop", playback: ended(pbNow - 10*86400), text: at(pbNow - 86400)},
		{name: "late progress without at: 410 playback-stopped", call: "progress", playback: ended(pbNow - 10*86400), text: noAt},
		{name: "late stop without at: 200, nothing changes", call: "stop", playback: ended(pbNow - 10*86400), text: noAt},
		{name: "late progress exactly 30 days after the play: recorded", call: "progress", playback: ended(pbNow - 30*86400), text: at(pbNow - 29*86400)},
		{name: "late progress 30 days and 1 second after the play: 410 playback-stopped", call: "progress", playback: ended(pbNow - 30*86400 - 1), text: at(pbNow - 29*86400)},
		{name: "late stop 30 days and 1 second after the play: 200, nothing changes", call: "stop", playback: ended(pbNow - 30*86400 - 1), text: at(pbNow - 29*86400)},
		{name: "late progress older than a report already had: ignored", call: "progress", playback: &playbackState{State: "ended", Purpose: "download", Title: pbFilmKey, PlayedAt: pbNow - 10*86400, LastAt: pbNow - 5*86400}, text: at(pbNow - 6*86400)},
		{name: "late progress with an at out of bounds: 400", call: "progress", playback: ended(pbNow - 10*86400), text: at(pbNow - 30*86400 - 1)},
		{name: "progress of a live download, with at", call: "progress", playback: &playbackState{State: "active", Purpose: "download", Title: pbFilmKey, PlayedAt: pbNow - 3600}, text: at(pbNow - 600)},
		{name: "stop of a live download: frees it", call: "stop", playback: &playbackState{State: "active", Purpose: "download", Title: pbFilmKey, PlayedAt: pbNow - 3600}, text: noAt},
		{name: "a late report of a play (not a download): 410 playback-stopped", call: "progress", playback: &playbackState{State: "ended", Purpose: "play", Title: pbFilmKey, PlayedAt: pbNow - 86400}, text: at(pbNow - 3600)},
	}
}

func playbackScanCases() []pbIn {
	film := `"title":"` + pbFilmKey + `"`
	sc := func(members ...string) string {
		return pbObj(append([]string{film, `"purpose":"scan"`}, members...)...)
	}
	file := sc(`"delivery":"file"`)
	live := &playbackState{State: "active", Purpose: "scan", Title: pbFilmKey, PlayedAt: pbNow - 60}
	id := pbObj(`"id":"` + pbPID + `"`)
	return []pbIn{
		{name: "file: scansPerDay, a 10-minute ticket, the idle rule, the pool's scan cap stated as scanBytes", text: file},
		{name: "a pool whose scan cap is 1 GiB: scanBytes 1 GiB", text: file, scanBytes: 1 << 30},
		{name: "a pool whose scan cap is 4 MiB: scanBytes 4 MiB", text: file, scanBytes: 4 << 20},
		{name: "an unknown delivery: 409 delivery-refused", text: sc(`"delivery":"blocks"`)},
		{name: "a track", text: pbObj(`"title":"`+pbTrackKeyA+`"`, `"purpose":"scan"`, `"delivery":"file"`)},
		{name: "no delivery: 400", text: sc()},
		{name: "remux: 400", text: sc(`"delivery":"remux"`, `"mediaType":"video/mp2t"`)},
		{name: "transcode: 400", text: sc(`"delivery":"transcode"`, `"transcode":{"maxHeight":720}`)},
		{name: "a copy of several parts: 409 delivery-refused", text: sc(`"delivery":"file"`, `"versionId":"Uk4nR7vT"`)},
		{name: "scansPerDay 0: 409 stream-limit-reached", text: file, limits: pbLim(func(l *playbackLimits) { l.ScansPerDay = 0 })},
		{name: "scansPerDay reached: 409 stream-limit-reached", text: file, inUse: pbUse(func(u *playbackUse) { u.ScansToday = 1000 })},
		{name: "one scan left today", text: file, inUse: pbUse(func(u *playbackUse) { u.ScansToday = 999 })},
		{name: "scansPerDay at 1,000,000", text: file, limits: pbLim(func(l *playbackLimits) { l.ScansPerDay = 1000000 }), inUse: pbUse(func(u *playbackUse) { u.ScansToday = 999999 })},
		{name: "streams all in use: a scan doesn't count against streams", text: file, inUse: pbUse(func(u *playbackUse) { u.Streams = 2; u.Downloads = 1 })},
		{name: "streams and downloads 0: a scan counts against scansPerDay only", text: file, limits: pbLim(func(l *playbackLimits) { l.Streams = 0; l.Transcodes = 0; l.Downloads = 0 })},
		{name: "no bitrate cap of play: a file above maxBitrateKbps", text: sc(`"delivery":"file"`, `"maxBitrateKbps":1000`)},
		{name: "progress of a live scan: 200, nothing changes", call: "progress", playback: live, text: pbObj(`"id":"`+pbPID+`"`, `"positionMs":0`)},
		{name: "stop of a live scan: ends it", call: "stop", playback: live, text: id},
		{name: "progress of a scan past its 10 minutes: 410 playback-stopped", call: "progress", playback: &playbackState{State: "ended", Purpose: "scan", Title: pbFilmKey, PlayedAt: pbNow - 601}, text: id},
	}
}

func playbackReportCases() []pbIn {
	idm := `"id":"` + pbPID + `"`
	active := &playbackState{State: "active", Purpose: "play", Title: pbFilmKey, PlayedAt: pbNow - 3600, LastAt: pbNow - 60}
	fresh := &playbackState{State: "active", Purpose: "play", Title: pbFilmKey, PlayedAt: pbNow - 40*86400}
	started := &playbackState{State: "started", Purpose: "play", Title: pbFilmKey, PlayedAt: pbNow - 10}
	ended := &playbackState{State: "ended", Purpose: "play", Title: pbFilmKey, PlayedAt: pbNow - 3600}
	unknown := &playbackState{State: "unknown"}
	queued := &playbackState{State: "active", Purpose: "play", Title: pbTrackKeyA, Queue: []string{pbTrackKeyB, pbTrackKeyC}, PlayedAt: pbNow - 300}
	p := func(name string, pb *playbackState, members ...string) pbIn {
		return pbIn{name: name, call: "progress", playback: pb, text: pbObj(members...)}
	}
	s := func(name string, pb *playbackState, members ...string) pbIn {
		return pbIn{name: name, call: "stop", playback: pb, text: pbObj(members...)}
	}
	t := func(v int64) string { return `"at":` + strconv.FormatInt(v, 10) }
	return []pbIn{
		p("progress: the position, at absent means now", active, idm, `"positionMs":600000`),
		p("paused, gone with proposal 17: an unknown member, ignored", active, idm, `"positionMs":600000`, `"paused":true`),
		p("paused of another type: an unknown member, ignored", active, idm, `"paused":"yes"`),
		p("paused null: an unknown member, ignored", active, idm, `"paused":null`),
		p("progress of a started playback: it becomes active", started, idm, `"positionMs":0`),
		p("progress with the id alone", active, idm),
		p("at 30 days before now", fresh, idm, `"positionMs":1000`, t(pbNow-30*86400)),
		p("at 30 days and 1 second before now: 400", active, idm, t(pbNow-30*86400-1)),
		p("at 5 minutes after now", fresh, idm, t(pbNow+300)),
		p("at 5 minutes and 1 second after now: 400", active, idm, t(pbNow+301)),
		p("at older than the report it has: ignored", active, idm, `"positionMs":1000`, t(pbNow-61)),
		p("at equal to the report it has: recorded", active, idm, `"positionMs":1000`, t(pbNow-60)),
		p("at as a string: 400", active, idm, `"at":"1795132800"`),
		p("at with a fraction: 400", active, idm, `"at":1795132800.5`),
		p("at in milliseconds: out of bounds, 400", active, idm, t(pbNow*1000)),
		p("at null: 400", active, idm, `"at":null`),
		p("positionMs at 100 days", active, idm, `"positionMs":8640000000`),
		p("positionMs above 100 days: 400", active, idm, `"positionMs":8640000001`),
		p("a negative positionMs: 400", active, idm, `"positionMs":-1`),
		p("no id: 400", active, `"positionMs":0`),
		p("an id of capitals and digits, the opaque form", active, `"id":"Pb7Q-x_9"`),
		p("an id of 64 characters", active, `"id":"`+strings.Repeat("P", 64)+`"`),
		p("an id of 65 characters: 400", active, `"id":"`+strings.Repeat("P", 65)+`"`),
		p("an empty id: 400", active, `"id":""`),
		p("an id with a dot: 400", active, `"id":"play.1"`),
		p("an id that isn't a string: 400", active, `"id":7`),
		{name: "a body that isn't an object: 400", call: "progress", playback: active, text: `"` + pbPID + `"`},
		p("progress of an ended playback: 410 playback-stopped", ended, idm, `"positionMs":0`),
		p("progress of an ended playback, with at: 410 (not a download)", ended, idm, t(pbNow-60)),
		p("progress of another pairing's or an unknown playback: 410 playback-stopped", unknown, idm),
		p("a malformed progress of an unknown playback: 400 first", unknown, idm, `"positionMs":-1`),
		p("unknown members ignored", active, idm, `"positionMs":5000`, `"volume":0.5`),
		p("music: title of the queue's entry playing", queued, idm, `"title":"`+pbTrackKeyB+`"`, `"positionMs":12000`),
		p("music: title of the request", queued, idm, `"title":"`+pbTrackKeyA+`"`, `"positionMs":12000`),
		p("music: no title, the request's", queued, idm, `"positionMs":12000`),
		p("music: a title neither the request's nor of the queue: 400", queued, idm, `"title":"track/local/Other1"`),
		p("music: a title that isn't a string: 400", queued, idm, `"title":7`),
		p("music: an empty title, neither the request's nor of the queue: 400", queued, idm, `"title":""`),
		p("a film with its own title", active, idm, `"title":"`+pbFilmKey+`"`),
		p("a film with another title: 400", active, idm, `"title":"movie/tmdb/603"`),
		s("stop: position recorded, the playback ends", active, idm, `"positionMs":600000`),
		s("stop of a started playback", started, idm),
		s("stop with at 30 days before now", fresh, idm, t(pbNow-30*86400)),
		s("stop with at out of bounds: 400", active, idm, t(pbNow+301)),
		s("stop with a positionMs above 100 days: 400", active, idm, `"positionMs":8640000001`),
		s("stop with at older than the report it has: ends, position kept", active, idm, `"positionMs":1`, t(pbNow-61)),
		s("a second stop: 200, nothing changes", ended, idm),
		s("stop of another pairing's or an unknown playback: 200, nothing changes", unknown, idm),
		s("stop without id: 400", active, `"positionMs":0`),
		s("stop with paused: an unknown member, ignored", active, idm, `"paused":true`),
		s("stop with paused null: an unknown member, ignored", active, idm, `"paused":null`),
		s("music: stop with the title of the queue's last entry", queued, idm, `"title":"`+pbTrackKeyC+`"`, `"positionMs":30000`),
		s("music: stop with the request's title", queued, idm, `"title":"`+pbTrackKeyA+`"`, `"positionMs":30000`),
		s("music: stop without title, the request's", queued, idm, `"positionMs":30000`),
		s("music: stop with a title neither the request's nor of the queue: 400", queued, idm, `"title":"track/local/Other1"`),
		s("music: stop with a title that isn't a string: 400", queued, idm, `"title":null`),
		s("stop of a film with another title: 400", active, idm, `"title":"movie/tmdb/603"`),
	}
}

func playbackAnswerCases() []playbackAnswerCase {
	id := `"id":"` + pbPID + `"`
	tk := `"ticket":"Tk3vQ9xY_-a"`
	hls := `"protocol":"hls"`
	file := `"protocol":"file"`
	etag := `"etag":"9c1e4f70"`
	pos := `"positionMs":0`
	ver := `"versionId":"Hd8kQ2mZ"`
	video := playbackAsked{Part: "video", CatalogEtag: "9c1e4f70", CatalogSubtitles: []string{"s4", "s5", "s6", "s7"}}
	with := func(f func(*playbackAsked)) playbackAsked {
		a := video
		f(&a)
		return a
	}
	delivery := func(d string) playbackAsked { return with(func(a *playbackAsked) { a.Delivery = d }) }
	scan := with(func(a *playbackAsked) { a.Purpose, a.Delivery = "scan", "file" })
	music := playbackAsked{Part: "music", CatalogEtag: "9c1e4f70"}
	// ans is an answer of the given members, the others the defaults.
	ans := func(method, protocol string, extra ...string) string {
		return pbObj(append([]string{id, tk, protocol, `"method":"` + method + `"`, etag, pos, ver}, extra...)...)
	}
	tr := func(extra ...string) string { return ans("transcode", hls, extra...) }
	without := func(drop string) string {
		var m []string
		for _, x := range []string{id, tk, hls, `"method":"transcode"`, etag, pos, ver} {
			if !strings.HasPrefix(x, `"`+drop+`"`) {
				m = append(m, x)
			}
		}
		return pbObj(m...)
	}
	ticket := func(t string) string {
		return pbObj(id, `"ticket":`+t, hls, `"method":"transcode"`, etag, pos, ver)
	}
	long := strings.Repeat("A", 64)
	type in struct {
		name  string
		asked playbackAsked
		text  string
	}
	list := []in{
		{"every member of playback and video, cleaned; the URLs built from the ticket", video, tr(`"audioTrack":"a1"`, `"audioRenditions":true`, `"maxBitrateKbps":20000`,
			`"subtitles":[{"id":"s4","form":"text","ass":true},{"id":"s5","form":"text"},{"id":"s6","form":"picture"}]`, `"fontCount":3`)},
		{"the required members alone: the stream's URL only", video, tr()},
		{"members dropped by proposal 17 are unknown: ignored", video, tr(`"stream":{"url":"https://b.example.org/x/master.m3u8","protocol":"hls","mimeType":"application/vnd.apple.mpegurl"}`, `"durationMs":8160000`,
			`"audioTracks":[{"id":"a1"}]`, `"subtitleTracks":[{"id":"s4"}]`, `"subtitleTrack":"s4"`, `"forcedTracks":["s5"]`, `"burnedTrack":"s4"`, `"fonts":["https://b.example.org/f"]`, `"trickplay":{"columns":10}`)},
		{"versions and info, dropped members, are unknown: ignored", video, tr(`"versions":[{"id":"Hd8kQ2mZ","name":"1080p"}]`, `"info":{"reasons":["codec"]}`)},
		{"a stream URL instead of a ticket: refused, no ticket", video, without("ticket")[:len(without("ticket"))-1] + `,"stream":{"url":"` + pbRoot + `Tk3vQ9xY/master.m3u8","protocol":"hls"}}`},
		{"a ticket of 64 characters", video, ticket(`"` + long + `"`)},
		{"a ticket of 65 characters: refused", video, ticket(`"` + long + `A"`)},
		{"a ticket with a dot: refused", video, ticket(`"Tk3.vQ9"`)},
		{"a ticket with a slash: refused", video, ticket(`"Tk3/vQ9"`)},
		{"a ticket with a percent: refused", video, ticket(`"Tk3%2F"`)},
		{"an empty ticket: refused", video, ticket(`""`)},
		{"a ticket that isn't a string: refused", video, ticket(`7`)},
		{"no protocol: refused", video, without("protocol")},
		{"protocol dash: refused", video, ans("transcode", `"protocol":"dash"`)},
		{"protocol in capitals: refused", video, ans("transcode", `"protocol":"HLS"`)},
		{"delivery file: method direct, the file", delivery("file"), ans("direct", file)},
		{"delivery remux: method remux, HLS", delivery("remux"), ans("remux", hls)},
		{"no delivery: direct as HLS", video, ans("direct", hls)},
		{"no delivery: the file is refused, the baseline is HLS", video, ans("direct", file)},
		{"no delivery: remux as HLS", video, ans("remux", hls)},
		{"delivery file, method remux: refused", delivery("file"), ans("remux", hls)},
		{"delivery file, method direct as HLS: refused", delivery("file"), ans("direct", hls)},
		{"delivery remux served as the file: refused", delivery("remux"), ans("remux", file)},
		{"delivery file, the file, method remux: refused", delivery("file"), ans("remux", file)},
		{"delivery remux, HLS, method transcode: refused", delivery("remux"), ans("transcode", hls)},
		{"delivery transcode, method direct: refused", delivery("transcode"), ans("direct", file)},
		{"method in capitals: refused", video, ans("Direct", hls)},
		{"an unknown method, copy among them: refused", video, ans("copy", hls)},
		{"no method: refused", video, without("method")},
		{"an id in capitals and digits, the opaque form", video, pbObj(`"id":"Pb7Q-x_9"`, tk, hls, `"method":"transcode"`, etag, pos, ver)},
		{"an id of 64 characters", video, pbObj(`"id":"`+long+`"`, tk, hls, `"method":"transcode"`, etag, pos, ver)},
		{"an id of 65 characters: refused", video, pbObj(`"id":"`+long+`A"`, tk, hls, `"method":"transcode"`, etag, pos, ver)},
		{"an id with a dot: refused", video, pbObj(`"id":"play.1"`, tk, hls, `"method":"transcode"`, etag, pos, ver)},
		{"no id: refused", video, without("id")},
		{"no etag: refused", video, without("etag")},
		{"an empty etag: refused", video, pbObj(id, tk, hls, `"method":"transcode"`, `"etag":""`, pos, ver)},
		{"an etag of 65 characters: refused", video, pbObj(id, tk, hls, `"method":"transcode"`, `"etag":"`+strings.Repeat("e", 65)+`"`, pos, ver)},
		{"an etag in quotes, as in a header: refused", video, pbObj(id, tk, hls, `"method":"transcode"`, `"etag":"\"9c1e4f70\""`, pos, ver)},
		{"another etag than the catalog's: kept, the answer wins, the title read again", video, pbObj(id, tk, hls, `"method":"transcode"`, `"etag":"0d0d0d0d"`, pos, ver)},
		{"no positionMs: refused", video, without("positionMs")},
		{"positionMs at 100 days", video, pbObj(id, tk, hls, `"method":"transcode"`, etag, `"positionMs":8640000000`, ver)},
		{"positionMs above 100 days: refused", video, pbObj(id, tk, hls, `"method":"transcode"`, etag, `"positionMs":8640000001`, ver)},
		{"positionMs as a string: refused", video, pbObj(id, tk, hls, `"method":"transcode"`, etag, `"positionMs":"0"`, ver)},
		{"no versionId: refused, it is required", video, without("versionId")},
		{"versionId of another form: refused", video, pbObj(id, tk, hls, `"method":"transcode"`, etag, pos, `"versionId":"../v1"`)},
		{"versionId that isn't a string: refused", video, pbObj(id, tk, hls, `"method":"transcode"`, etag, pos, `"versionId":1`)},
		{"audioTrack of another form: cleared", video, tr(`"audioTrack":"a 1"`)},
		{"audioTrack with a leading zero: cleared", video, tr(`"audioTrack":"a01"`)},
		{"audioTrack that isn't a string: refused", video, tr(`"audioTrack":1`)},
		{"audioRenditions true with HLS: kept", video, tr(`"audioRenditions":true`)},
		{"audioRenditions false: none made, nothing kept", video, tr(`"audioRenditions":false`)},
		{"audioRenditions true with the file: dropped", delivery("file"), ans("direct", file, `"audioRenditions":true`)},
		{"audioRenditions that isn't a boolean: refused", video, tr(`"audioRenditions":"true"`)},
		{"maxBitrateKbps above 10,000,000: dropped", video, tr(`"maxBitrateKbps":10000001`)},
		{"maxBitrateKbps that isn't an integer: refused", video, tr(`"maxBitrateKbps":"20000"`)},
		// scanBytes (section 2.3).
		{"a scan's scanBytes, 32 MiB: kept", scan, ans("direct", file, `"scanBytes":33554432`)},
		{"a scan's scanBytes at 1 GiB", scan, ans("direct", file, `"scanBytes":1073741824`)},
		{"a scan's scanBytes at its floor, 4 MiB", scan, ans("direct", file, `"scanBytes":4194304`)},
		{"a scan without scanBytes: 4 MiB", scan, ans("direct", file)},
		{"a scan's scanBytes below 4 MiB: 4 MiB", scan, ans("direct", file, `"scanBytes":4194303`)},
		{"a scan's scanBytes above 1 GiB: 4 MiB", scan, ans("direct", file, `"scanBytes":1073741825`)},
		{"a scan's scanBytes as a string: 4 MiB", scan, ans("direct", file, `"scanBytes":"33554432"`)},
		{"scanBytes in the answer of a play: ignored", delivery("file"), ans("direct", file, `"scanBytes":33554432`)},
		// subtitles and fontCount (video section 5.3).
		{"subtitles: an entry not a track of the copy in the catalog, dropped alone", video, tr(`"subtitles":[{"id":"s9","form":"text"},{"id":"s4","form":"text"}]`)},
		{"subtitles: a form neither text nor picture, dropped alone", video, tr(`"subtitles":[{"id":"s4","form":"ass"},{"id":"s5","form":"Text"},{"id":"s6","form":"picture"}]`)},
		{"subtitles: ass with picture, dropped; ass that isn't a boolean, absent", video, tr(`"subtitles":[{"id":"s6","form":"picture","ass":true},{"id":"s4","form":"text","ass":"true"},{"id":"s5","form":"text","ass":false}]`)},
		{"subtitles: an entry twice, the second dropped", video, tr(`"subtitles":[{"id":"s4","form":"text"},{"id":"s4","form":"picture","ass":true}]`)},
		{"subtitles: entries of another form, dropped alone", video, tr(`"subtitles":["s4",{"id":4,"form":"text"},{"form":"text"},{"id":"s04","form":"text"},null,{"id":"s5","form":"text","url":"https://b.example.org/s5"}]`)},
		{"subtitles that isn't an array: absent", video, tr(`"subtitles":{"id":"s4","form":"text"}`)},
		{"subtitles empty", video, tr(`"subtitles":[]`)},
		{"fontCount 0", video, tr(`"fontCount":0`)},
		{"fontCount 64: fonts 0 to 63", video, tr(`"fontCount":64`)},
		{"fontCount 65: 0", video, tr(`"fontCount":65`)},
		{"a negative fontCount: 0", video, tr(`"fontCount":-1`)},
		{"fontCount as a string: 0", video, tr(`"fontCount":"3"`)},
		{"fontCount with a fraction: 0", video, tr(`"fontCount":2.5`)},
		{"a track's answer: video's subtitles and fontCount ignored", music, tr(`"subtitles":[{"id":"s4","form":"text"}]`, `"fontCount":2`)},
		{"unknown members, title and next among them, ignored", video, tr(`"title":"movie/tmdb/603"`, `"next":{"title":"movie/tmdb/604"}`)},
		{"an answer that isn't an object: refused", video, `[]`},
	}
	var out []playbackAnswerCase
	for _, c := range list {
		got, err := playbackKeepAnswer([]byte(c.text), c.asked, pbRoot)
		ok, r := reason(err)
		out = append(out, playbackAnswerCase{Name: c.name, Asked: c.asked, Text: c.text, Valid: ok, Output: string(got), Reason: r})
	}
	return out
}

func playbackRestartCases() []playbackRestartCase {
	reading := playbackReading{VersionID: "Hd8kQ2mZ", Etag: "9c1e4f70", Purpose: "play", Delivery: "file", Offset: 104857600}
	req := `{"title":"` + pbFilmKey + `","versionId":"Hd8kQ2mZ","delivery":"file"}`
	ans := func(etag, method, protocol string) string {
		return pbObj(`"id":"play_bbbbbbbbbbbbbbbbbbbbbbbbbb"`, `"ticket":"Nw7tK2"`, `"protocol":"`+protocol+`"`, `"method":"`+method+`"`, `"etag":"`+etag+`"`, `"positionMs":0`, `"versionId":"Hd8kQ2mZ"`)
	}
	ok := ans("9c1e4f70", "direct", "file")
	dl := reading
	dl.Purpose = "download"
	remux := reading
	remux.Delivery = "remux"
	type in struct {
		name    string
		reading playbackReading
		request string
		status  int
		typ     string
		answer  string
	}
	list := []in{
		{"the same etag: the read goes on at the same byte", reading, req, 200, "", ok},
		{"the same etag, purpose play named", reading, `{"title":"` + pbFilmKey + `","purpose":"play","versionId":"Hd8kQ2mZ","delivery":"file"}`, 200, "", ok},
		{"another etag: the new playback stopped, the read failed, the title read again", reading, req, 200, "", ans("0d0d0d0d", "direct", "file")},
		{"the etag in another case: another etag", reading, req, 200, "", ans("9C1E4F70", "direct", "file")},
		{"the new play refused, stream-limit-reached: the read failed", reading, req, 409, "stream-limit-reached", ""},
		{"the new play refused, delivery-refused: the read failed", reading, req, 409, "delivery-refused", ""},
		{"no answer to the new play (busy): the read failed", reading, req, 503, "server-busy", ""},
		{"the new answer refused (method remux): stopped, the read failed", reading, req, 200, "", ans("9c1e4f70", "remux", "hls")},
		{"the new answer refused (a bad etag): stopped, the read failed", reading, req, 200, "", ans("9c1e.4f70", "direct", "file")},
		{"the new answer refused (no ticket): stopped, the read failed", reading, req, 200, "", strings.Replace(ok, `"ticket":"Nw7tK2",`, "", 1)},
		{"a play of another copy: not a restart", reading, `{"title":"` + pbFilmKey + `","versionId":"Lw3pX9cJ","delivery":"file"}`, 200, "", ok},
		{"a play without versionId: not a restart", reading, `{"title":"` + pbFilmKey + `","delivery":"file"}`, 200, "", ok},
		{"a play of another purpose: not a restart", reading, `{"title":"` + pbFilmKey + `","purpose":"download","versionId":"Hd8kQ2mZ","delivery":"file"}`, 200, "", ok},
		{"a play of another delivery: not a restart", reading, `{"title":"` + pbFilmKey + `","versionId":"Hd8kQ2mZ","delivery":"remux","mediaType":"video/mp2t"}`, 200, "", ans("9c1e4f70", "remux", "hls")},
		{"a reading of a remux: never restarted", remux, `{"title":"` + pbFilmKey + `","versionId":"Hd8kQ2mZ","delivery":"remux","mediaType":"video/mp2t"}`, 200, "", ans("9c1e4f70", "remux", "hls")},
		{"a download past its 24 hours, the same etag: goes on", dl, `{"title":"` + pbFilmKey + `","purpose":"download","versionId":"Hd8kQ2mZ","delivery":"file"}`, 200, "", ok},
		{"a download past its 24 hours, another etag: failed", dl, `{"title":"` + pbFilmKey + `","purpose":"download","versionId":"Hd8kQ2mZ","delivery":"file"}`, 200, "", ans("0d0d0d0d", "direct", "file")},
	}
	var out []playbackRestartCase
	for _, c := range list {
		r := playbackRestart(c.reading, []byte(c.request), c.status, []byte(c.answer), pbRoot)
		out = append(out, playbackRestartCase{Name: c.name, Reading: c.reading, Request: c.request, Status: c.status, Type: c.typ, Answer: c.answer, Valid: r.Join, Output: string(playbackEncode(r)), Reason: r.Why})
	}
	return out
}

func playbackLimitsCases() []playbackLimitsCase {
	list := []struct{ name, text string }{
		{"every member", `{"streams":2,"transcodes":1,"bitrateKbps":0,"downloads":1,"downloadsPerDay":10,"downloadKbps":20000,"scansPerDay":1000}`},
		{"the new members absent: 0, no downloads, no scan access", `{"streams":2,"transcodes":1,"bitrateKbps":20000}`},
		{"an empty object", `{}`},
		{"every member at its top", `{"streams":1000,"transcodes":1000,"bitrateKbps":1000000,"downloads":1000,"downloadsPerDay":10000,"downloadKbps":1000000,"scansPerDay":1000000}`},
		{"every member above its top: each ignored alone", `{"streams":1001,"transcodes":1001,"bitrateKbps":1000001,"downloads":1001,"downloadsPerDay":10001,"downloadKbps":1000001,"scansPerDay":1000001}`},
		{"a pool of 51 streams and downloads, above the old top", `{"streams":51,"transcodes":51,"downloads":51}`},
		{"scanBytes, no member of limits since proposal 17: unknown, ignored", `{"scansPerDay":10,"scanBytes":33554432}`},
		{"scanBytes as a string: unknown, ignored whatever it holds", `{"scansPerDay":10,"scanBytes":"33554432"}`},
		{"negative members: each ignored alone", `{"streams":-1,"downloads":-1,"scansPerDay":-1,"transcodes":0}`},
		{"transcodes above streams: ignored", `{"streams":2,"transcodes":3}`},
		{"transcodes equal to streams", `{"streams":3,"transcodes":3}`},
		{"downloads 0 with a daily cap: no downloads", `{"downloads":0,"downloadsPerDay":10,"downloadKbps":20000}`},
		{"scansPerDay 1: scan access", `{"scansPerDay":1}`},
		{"music's audioStreams and audioTranscodes", `{"streams":2,"transcodes":1,"audioStreams":4,"audioTranscodes":2}`},
		{"audioStreams at 1,000, audioTranscodes above it: ignored", `{"audioStreams":1000,"audioTranscodes":1001}`},
		{"audioTranscodes above audioStreams: ignored", `{"audioStreams":1,"audioTranscodes":2}`},
		{"unknown members ignored", `{"streams":2,"Downloads":5,"uploads":"x"}`},
		{"a member as a string: the limits ignored", `{"streams":2,"downloads":"1"}`},
		{"a member with a fraction: the limits ignored", `{"streams":2,"scansPerDay":1000.0}`},
		{"a null member: the limits ignored", `{"streams":2,"downloadKbps":null}`},
		{"limits that aren't an object: ignored", `[2,1]`},
	}
	var out []playbackLimitsCase
	for _, c := range list {
		got, err := playbackKeepLimits([]byte(c.text))
		ok, r := reason(err)
		out = append(out, playbackLimitsCase{Name: c.name, Text: c.text, Valid: ok, Output: string(got), Reason: r})
	}
	return out
}

func playbackFile() v0PlaybackFile {
	ctx := playbackContextOf()
	return v0PlaybackFile{
		About: []string{
			"MSFP playback part version 0 (spec/v0/playback.md, with video sections 5.1 and 5.3's members, music section 5.2's queue entries and title in progress and stop, media section 1's URLs), as revised by proposals 16 and 17. Generated by tools/vectors; no implementation has produced them yet.",
			"context: the owning server's copies of each title it offers the caller (parts above 1: a copy of several files; size and durationMs give the overall rate, size × 8 / durationMs kbit/s; the video stream, absent for a copy with no video, and the audio tracks with the facts the baseline goes by), the codecs it encodes (videoEncoders, audioEncoders: a transcode codec outside them is refused with 409 delivery-refused), the ranges it makes, the codecs each segment format carries (segmentCodecs: a remux, or a copied stream, outside them is refused with 409 delivery-refused), its own bitrate cap (ownCapKbps, 0: none), the pool's scan cap it states as scanBytes, the pairing's limits and what is in use, its stream root, and now (Unix seconds). A case's limits, inUse, ranges or scanBytes replaces the context's for that case.",
			"requests, deliveries, baseline, downloads, scans, reports: call play, progress or stop; text is the request body as received. valid true: status 200 and output; valid false: status and type, the refusal (playback sections 2.2, 2.5, 3, 4). Unknown values follow core section 8.5: a well-formed value not of a list is 409 delivery-refused where it names what to serve (delivery, a remux's mediaType, a transcode codec, range), a malformed one 400. Members proposal 17 dropped (audioLanguage, subtitleLanguage, subtitleTrack, container, paused) are unknown members, ignored whatever they hold.",
			"For play, output is what the owning server starts: the request as read; baseline, without delivery, what it serves of each stream (copy: left unchanged, within the baseline of playback section 2.7; else the baseline's codec; no video member for a copy with no video) and the method; a transcode's targets with the baseline's values for those left out (maxHeight left out: the copy's own height), without the targets a copied stream or a copy with no video doesn't take; the copy (versionId: the one named, else the first) and audio (audioTrack: the one named, else the file's default audio); the start position, the bitrate cap of step 6 (play only, absent: none), the speed cap (downloadKbps, downloads only), what it counts against, which transcodes it takes (absent: none; video and audio copy take none), the ticket's lifetime and idle rule, and for a scan the scanBytes its answer states. Members in that order, absent ones left out.",
			"queueEntries: request is a play request, entry one entry of its queue; output is what the owning server starts for that entry, as for play: the entry's title and versionId, its own delivery with its own mediaType or transcode, or without one the request's; purpose, the caps and audioRenditions are the request's. It counts once, under the request (no counts). What music does with a refused entry is music's (music.json).",
			"progress and stop cases carry playback: the owning server's playback the id names (state started, active or ended; unknown: another pairing's or never given), its purpose, title and queue, when its play was answered (playedAt), the at of the latest report it has (lastAt). output: effect (recorded, recorded and ended, ended, ignored, unchanged), the state after, and what is recorded.",
			"answers: text is a play answer; asked is what the viewer's server asked (purpose, delivery) and knows (the title's content part, the copy's etag and subtitle track IDs in the catalog). valid false: the viewer's server sends stop and gives up. valid true: output is what it keeps, in the order id, ticket, protocol, method, etag, positionMs, versionId, audioTrack, audioRenditions (only when true, with hls), maxBitrateKbps, scanBytes (scans only), subtitles and fontCount (video only), then urls, the URLs it builds from the stream root and the ticket (the stream; each subtitle track's subtitles/, ass/ or pictures/ base, to which it adds /<part> or /whole; each font), then readTitleAgain when the etag isn't the catalog's. Unknown members, the ones proposal 17 dropped among them, are not kept.",
			"restarts: a viewer's server reading a playback's file (reading: its copy, etag, purpose, delivery and the byte it was at) sends request, a new play; status (and type) is the answer's, answer its body when 200. valid true: it goes on reading at offset; false: output says whether it stops the new playback, fails the read and reads the title again from titles (playback section 2.7).",
			"limits: text is the limits object of a libraries answer; valid false: ignored whole; output is what the caller keeps, the new members 0 when absent, and whether it offers downloads and asks for scans (playback section 5). scanBytes is no member of limits since proposal 17.",
		},
		Context:      ctx,
		Requests:     pbJudge(ctx, playbackRequestCases()),
		Answers:      playbackAnswerCases(),
		Deliveries:   pbJudge(ctx, playbackDeliveryCases()),
		Baseline:     pbJudge(ctx, playbackBaselineCases()),
		QueueEntries: playbackQueueEntryCases(),
		Restarts:     playbackRestartCases(),
		Reports:      pbJudge(ctx, playbackReportCases()),
		Downloads:    pbJudge(ctx, playbackDownloadCases()),
		Scans:        pbJudge(ctx, playbackScanCases()),
		Limits:       playbackLimitsCases(),
	}
}

// genV0Playback makes vectors/v0/playback.json (spec/v0/playback.md).
func genV0Playback() []byte { return encode(playbackFile()) }
