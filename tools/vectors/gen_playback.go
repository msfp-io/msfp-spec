package main

import (
	"strconv"
	"strings"
)

// playbackCase is one call and its outcome: a play, progress or stop the
// owning server judges, or (by "viewer") the capabilities a viewer's
// server coarsens.
type playbackCase struct {
	Name     string          `json:"name"`
	Call     string          `json:"call"`
	By       string          `json:"by,omitempty"`
	Limits   *playbackLimits `json:"limits,omitempty"`
	InUse    *playbackUse    `json:"inUse,omitempty"`
	Playback *playbackState  `json:"playback,omitempty"`
	Text     string          `json:"text"`
	Valid    bool            `json:"valid"`
	Status   int             `json:"status,omitempty"`
	Type     string          `json:"type,omitempty"`
	Output   string          `json:"output,omitempty"`
	Reason   string          `json:"reason,omitempty"`
}

type playbackAnswerCase struct {
	Name        string `json:"name"`
	Delivery    string `json:"delivery,omitempty"`
	CatalogEtag string `json:"catalogEtag"`
	Text        string `json:"text"`
	Valid       bool   `json:"valid"`
	Output      string `json:"output,omitempty"`
	Reason      string `json:"reason,omitempty"`
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

type v0PlaybackFile struct {
	About      []string              `json:"about"`
	Context    playbackContext       `json:"context"`
	Requests   []playbackCase        `json:"requests"`
	Answers    []playbackAnswerCase  `json:"answers"`
	Deliveries []playbackCase        `json:"deliveries"`
	Restarts   []playbackRestartCase `json:"restarts"`
	Reports    []playbackCase        `json:"reports"`
	Downloads  []playbackCase        `json:"downloads"`
	Scans      []playbackCase        `json:"scans"`
	Limits     []playbackLimitsCase  `json:"limits"`
}

const (
	pbNow       = 1795132800
	pbPID       = "play_abcdefghijklmnopqrstuvwxyz"
	pbRoot      = "https://a.example.org/api/v1/stream/"
	pbFilmKey   = "movie/tmdb/949"
	pbTrackKeyA = "track/mb/0c000000-0000-4000-8000-000000000003"
	pbTrackKeyB = "track/mb/0c000000-0000-4000-8000-000000000004"
	pbTrackKeyC = "track/local/Tl5Kp2"
)

func playbackContextOf() playbackContext {
	return playbackContext{
		Copies: map[string][]playbackCopy{
			pbFilmKey: {
				{ID: "Hd8kQ2mZ", Parts: 1, Etag: "9c1e4f70", BitrateKbps: 8000, Audio: []playbackTrackInfo{{ID: "a1", Language: "en", Default: true}, {ID: "a2", Language: "fr-CA"}, {ID: "a3", Language: "en", Role: "commentary"}}, Subtitles: []string{"s4", "s5"}},
				{ID: "Uk4nR7vT", Parts: 2, Etag: "2b7d00aa", BitrateKbps: 40000, Audio: []playbackTrackInfo{{ID: "a1", Language: "de", Default: true}, {ID: "a2", Language: "fr"}}, Subtitles: []string{"s3"}},
				{ID: "Lw3pX9cJ", Parts: 1, Etag: "51f0c3d2", BitrateKbps: 3000, Audio: []playbackTrackInfo{{ID: "a1", Language: "ja"}, {ID: "a2", Language: "en", Role: "description"}}},
			},
			"series/tmdb/1399/s1/e1":  {{ID: "Ep1aB2cD", Parts: 1, Etag: "e1e1e1e1", BitrateKbps: 5000, Audio: []playbackTrackInfo{{ID: "a1", Language: "en", Default: true}}}},
			"movie/tmdb/348/x/Tr7aQ1": {{ID: "Xt0aZ9", Parts: 1, Etag: "77aa", BitrateKbps: 2000, Audio: []playbackTrackInfo{{ID: "a1", Language: "en"}}}},
			pbTrackKeyA:               {{ID: "Vq3kM0aZ7pXc", Parts: 1, Etag: "7f3a9c21", BitrateKbps: 936, Audio: []playbackTrackInfo{{ID: "a0", Language: "en"}}}},
			pbTrackKeyC:               {{ID: "Lt1", Parts: 1, Etag: "aa11", BitrateKbps: 320, Audio: []playbackTrackInfo{{ID: "a0"}}}},
		},
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
	playback   *playbackState
	call, by   string
}

// pbJudge works out each case's outcome with the rules of playback.go.
func pbJudge(ctx playbackContext, in []pbIn) []playbackCase {
	var out []playbackCase
	for _, c := range in {
		pc := playbackCase{Name: c.name, Call: c.call, By: c.by, Limits: c.limits, InUse: c.inUse, Playback: c.playback, Text: c.text}
		if pc.Call == "" {
			pc.Call = "play"
		}
		got, err := pbRun(ctx, pc)
		pc.Valid, pc.Reason = reason(err)
		pc.Output = string(got)
		if c.by != "viewer" {
			pc.Status, pc.Type = playbackStatus(err)
		}
		out = append(out, pc)
	}
	return out
}

// pbRun applies the rule a case names; verify_rev_playback.go calls it too.
func pbRun(ctx playbackContext, c playbackCase) ([]byte, error) {
	if c.By == "viewer" {
		return playbackCoarsen([]byte(c.Text))
	}
	if c.Call == "play" {
		lim, use := ctx.Limits, ctx.InUse
		if c.Limits != nil {
			lim = *c.Limits
		}
		if c.InUse != nil {
			use = *c.InUse
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

func pbCaps(members ...string) string { return `"capabilities":` + pbObj(members...) }

func playbackRequestCases() []pbIn {
	film := `"title":"` + pbFilmKey + `"`
	play := func(name string, members ...string) pbIn { return pbIn{name: name, text: pbObj(members...)} }
	viewer := func(name string, members ...string) pbIn {
		return pbIn{name: name, text: pbObj(append([]string{film}, members...)...), by: "viewer"}
	}
	return []pbIn{
		play("the title alone: the first copy, its default audio, the cap of 120,000", film),
		play("every member of playback and video", film, `"purpose":"play"`, `"delivery":"transcode"`, `"transcode":{"videoCodec":"h264","maxHeight":1080,"maxBitrateKbps":8000,"audioCodec":"aac","audioChannels":2,"burnSubtitle":"s4"}`, `"audioRenditions":true`, pbCaps(`"containers":["mp4","ts"]`, `"videoCodecs":["h264"]`, `"audioCodecs":["aac"]`, `"maxHeight":1080`, `"maxAudioChannels":6`), `"startPositionMs":60000`, `"maxBitrateKbps":20000`, `"audioLanguage":"fr-CA"`, `"audioTrack":"a2"`, `"versionId":"Hd8kQ2mZ"`, `"subtitleLanguage":"fr"`, `"subtitleTrack":"s4"`),
		play("an episode by its numbers", `"title":"series/tmdb/1399/s1/e1"`),
		play("an extra", `"title":"movie/tmdb/348/x/Tr7aQ1"`),
		play("a track of the mb scheme", `"title":"`+pbTrackKeyA+`"`),
		play("a track of the local scheme", `"title":"`+pbTrackKeyC+`"`),
		play("unknown members ignored", film, `"Title":"x"`, `"quality":"best"`, `"device":null`),
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
		play("an unknown purpose: 400", film, `"purpose":"stream"`),
		play("a purpose that isn't a string: 400", film, `"purpose":1`),
		play("an unknown delivery: 400", film, `"delivery":"direct"`),
		play("a delivery in capitals: 400", film, `"delivery":"FILE"`),
		play("play with delivery file", film, `"purpose":"play"`, `"delivery":"file"`),
		play("play with delivery remux", film, `"delivery":"remux"`, `"container":"mp4"`),
		play("play with delivery transcode", film, `"delivery":"transcode"`),
		play("download without delivery: 400", film, `"purpose":"download"`),
		play("download with delivery file", film, `"purpose":"download"`, `"delivery":"file"`),
		play("download with delivery remux: 400", film, `"purpose":"download"`, `"delivery":"remux"`, `"container":"mkv"`),
		play("download with a transcode naming no size: 400", film, `"purpose":"download"`, `"delivery":"transcode"`, `"transcode":{"videoCodec":"h264"}`),
		play("download with a transcode naming maxHeight", film, `"purpose":"download"`, `"delivery":"transcode"`, `"transcode":{"maxHeight":720}`),
		play("scan without delivery: 400", film, `"purpose":"scan"`),
		play("scan with delivery file", film, `"purpose":"scan"`, `"delivery":"file"`),
		play("scan with delivery remux: 400", film, `"purpose":"scan"`, `"delivery":"remux"`, `"container":"mkv"`),
		play("scan with delivery transcode: 400", film, `"purpose":"scan"`, `"delivery":"transcode"`, `"transcode":{"maxHeight":480}`),
		play("audioRenditions false", film, `"audioRenditions":false`),
		play("audioRenditions that isn't a boolean: 400", film, `"audioRenditions":"true"`),
		play("startPositionMs 0", film, `"startPositionMs":0`),
		play("startPositionMs at 100 days", film, `"startPositionMs":8640000000`),
		play("startPositionMs above 100 days: 400", film, `"startPositionMs":8640000001`),
		play("a negative startPositionMs: 400", film, `"startPositionMs":-1`),
		play("startPositionMs with a fraction: 400", film, `"startPositionMs":1.0`),
		play("startPositionMs as a string: 400", film, `"startPositionMs":"0"`),
		play("maxBitrateKbps 0: none, the cap of 120,000", film, `"maxBitrateKbps":0`),
		play("maxBitrateKbps at 10,000,000", film, `"maxBitrateKbps":10000000`),
		play("maxBitrateKbps above 10,000,000: 400", film, `"maxBitrateKbps":10000001`),
		play("maxBitrateKbps below 1,000, kept as it is", film, `"maxBitrateKbps":500`),
		play("the lowest non-zero cap wins: capabilities", film, `"maxBitrateKbps":20000`, pbCaps(`"maxBitrateKbps":15000`)),
		play("audioLanguage of 12 characters, a valid tag", film, `"audioLanguage":"abcdefgh-ijk"`),
		play("audioLanguage fre, not the form a server sends: 400", film, `"audioLanguage":"fre"`),
		play("audioLanguage fr-ca, not the form a server sends: 400", film, `"audioLanguage":"fr-ca"`),
		play("audioLanguage of one character: 400", film, `"audioLanguage":"e"`),
		play("subtitleLanguage und: 400", film, `"subtitleLanguage":"und"`),
		play("audioLanguage of 13 characters: 400", film, `"audioLanguage":"abcdefgh-ijkl"`),
		play("audioLanguage with an underscore: 400", film, `"audioLanguage":"fr_CA"`),
		play("an empty audioLanguage: 400", film, `"audioLanguage":""`),
		play("subtitleLanguage of 13 characters: 400", film, `"subtitleLanguage":"abcdefghijklm"`),
		play("audioTrack a0, not a track of the copy: the default audio", film, `"audioTrack":"a0"`),
		play("audioTrack a999", film, `"audioTrack":"a999"`),
		play("audioTrack a1000: 400", film, `"audioTrack":"a1000"`),
		play("audioTrack with a leading zero: 400", film, `"audioTrack":"a01"`),
		play("audioTrack in capitals: 400", film, `"audioTrack":"A1"`),
		play("audioTrack naming a subtitle: 400", film, `"audioTrack":"s4"`),
		play("audioTrack that isn't a string: 400", film, `"audioTrack":1`),
		play("subtitleTrack off", film, `"subtitleTrack":"off"`),
		play("subtitleTrack s999", film, `"subtitleTrack":"s999"`),
		play("subtitleTrack with leading zeros: 400", film, `"subtitleTrack":"s007"`),
		play("subtitleTrack Off: 400", film, `"subtitleTrack":"Off"`),
		play("order: a malformed audioTrack of a title not offered is 404 (step 3 before step 4)", `"title":"movie/tmdb/603"`, `"audioTrack":"a01"`),
		{name: "order: a malformed audioTrack with streams 0 is 409 (step 2 before step 4)", text: pbObj(film, `"audioTrack":"a01"`), limits: pbLim(func(l *playbackLimits) { l.Streams = 0 })},
		{name: "order: a bad startPositionMs with streams 0 is 400 (step 1 first)", text: pbObj(film, `"startPositionMs":-1`), limits: pbLim(func(l *playbackLimits) { l.Streams = 0 })},
		{name: "order: a title not offered with streams 0 is 409 (step 2 before step 3)", text: pbObj(`"title":"movie/tmdb/603"`), limits: pbLim(func(l *playbackLimits) { l.Streams = 0 })},
		{name: "streams 0: 409 stream-limit-reached", text: pbObj(film), limits: pbLim(func(l *playbackLimits) { l.Streams = 0 })},
		{name: "streams all in use: 409 stream-limit-reached", text: pbObj(film), inUse: pbUse(func(u *playbackUse) { u.Streams = 2 })},
		{name: "one stream left", text: pbObj(film), inUse: pbUse(func(u *playbackUse) { u.Streams = 1 })},
		{name: "downloads all in use: play is not concerned", text: pbObj(film), inUse: pbUse(func(u *playbackUse) { u.Downloads = 1 })},
		play("versionId names a copy", film, `"versionId":"Lw3pX9cJ"`),
		play("versionId names no copy: the first", film, `"versionId":"zzz"`),
		play("versionId of any form, naming no copy: the first", film, `"versionId":"../x"`),
		play("versionId that isn't a string: 400", film, `"versionId":7`),
		play("versionId wins over audioLanguage", film, `"versionId":"Lw3pX9cJ"`, `"audioLanguage":"fr"`),
		play("audioLanguage: the same tag first, in any copy (fr: the second)", film, `"audioLanguage":"fr"`),
		play("audioLanguage in another case than a server sends: 400", film, `"audioLanguage":"FR-ca"`),
		play("audioLanguage: then the same base language (fr-FR: fr-CA of the first)", film, `"audioLanguage":"fr-FR"`),
		play("audioLanguage: the same base language (de-AT: de of the second)", film, `"audioLanguage":"de-AT"`),
		play("audioLanguage no copy has: the first copy, its default audio", film, `"audioLanguage":"es"`),
		play("audioLanguage en: the title's own sound, never the commentary", film, `"audioLanguage":"en"`),
		play("a description is never chosen by language: the first audio", film, `"versionId":"Lw3pX9cJ"`, `"audioLanguage":"en"`),
		play("audioTrack names a commentary: that track", film, `"audioTrack":"a3"`),
		play("audioTrack not of the copy: as if absent, by audioLanguage", film, `"audioTrack":"a7"`, `"audioLanguage":"fr-CA"`),
		play("audioTrack wins over audioLanguage", film, `"audioTrack":"a1"`, `"audioLanguage":"fr-CA"`),
		play("capabilities: unknown entries (in another case too) ignored, repeats once, sorted", film, pbCaps(`"containers":["ts","mp4","MP4","mp4","dash"]`, `"videoCodecs":["H264","hevc","h265","av1"]`, `"audioCodecs":["opus","aac","dts-hd"]`, `"audioProfiles":["lc","atmos","Atmos"]`)),
		play("capabilities: an empty array or one of unknown entries says nothing", film, pbCaps(`"containers":[]`, `"audioCodecs":["xyz"]`, `"subtitles":["vtt"]`)),
		play("capabilities: values not coarsened are read as they are", film, pbCaps(`"maxAudioChannels":7`, `"maxAudioSampleRate":44000`, `"maxAudioBitDepth":20`, `"maxHeight":1081`, `"maxBitrateKbps":1500`)),
		play("capabilities: 0 means not said", film, pbCaps(`"maxAudioChannels":0`, `"maxAudioSampleRate":0`, `"maxAudioBitDepth":0`, `"maxHeight":0`, `"maxBitrateKbps":0`)),
		play("capabilities: every number at its top", film, pbCaps(`"maxAudioChannels":64`, `"maxAudioSampleRate":768000`, `"maxAudioBitDepth":64`, `"maxHeight":100000`, `"maxBitrateKbps":10000000`)),
		play("capabilities: maxAudioChannels above 64: 400", film, pbCaps(`"maxAudioChannels":65`)),
		play("capabilities: maxAudioSampleRate above 768,000: 400", film, pbCaps(`"maxAudioSampleRate":768001`)),
		play("capabilities: maxAudioBitDepth above 64: 400", film, pbCaps(`"maxAudioBitDepth":65`)),
		play("capabilities: maxHeight above 100,000: 400", film, pbCaps(`"maxHeight":100001`)),
		play("capabilities: maxBitrateKbps above 10,000,000: 400", film, pbCaps(`"maxBitrateKbps":10000001`)),
		play("capabilities: a negative maxAudioChannels: 400", film, pbCaps(`"maxAudioChannels":-1`)),
		play("capabilities: maxAudioSampleRate with a fraction: 400", film, pbCaps(`"maxAudioSampleRate":48000.0`)),
		play("capabilities: an entry that isn't a string: 400", film, pbCaps(`"audioCodecs":["aac",1]`)),
		play("capabilities: a null entry: 400", film, pbCaps(`"containers":["mp4",null]`)),
		play("capabilities: a list that isn't an array: 400", film, pbCaps(`"containers":"mp4"`)),
		play("capabilities that aren't an object: 400", film, `"capabilities":["mp4"]`),
		play("capabilities null: 400", film, `"capabilities":null`),
		play("capabilities: unknown members ignored", film, pbCaps(`"maxWidth":1920`, `"codecs":7`)),
		play("capabilities: dvProfiles listed, increasing, unknown ignored", film, pbCaps(`"hdr":["dv","hdr10"]`, `"dvProfiles":[8,5,6,5]`)),
		play("capabilities: dv without dvProfiles counts as no Dolby Vision", film, pbCaps(`"hdr":["dv","hlg"]`)),
		play("capabilities: dv with only unknown dvProfiles counts as no Dolby Vision", film, pbCaps(`"hdr":["dv"]`, `"dvProfiles":[1,2]`)),
		play("capabilities: a dvProfiles entry that isn't an integer: 400", film, pbCaps(`"hdr":["dv"]`, `"dvProfiles":[5,"8"]`)),
		play("capabilities: a dvProfiles entry with a fraction: 400", film, pbCaps(`"dvProfiles":[5.0]`)),
		viewer("coarsening: lowercase, listed values only, each once, sorted", pbCaps(`"containers":["MP4","ts","mp4","dash"]`, `"videoCodecs":["HEVC","h264","h265"]`, `"audioCodecs":["EAC3","aac","aac"]`, `"audioProfiles":["Atmos","lc"]`, `"hdr":["HDR10","DV","hdr10plus"]`, `"dvProfiles":[8,5,5,6,"7"]`)),
		viewer("coarsening: steps at their values", pbCaps(`"maxHeight":1080`, `"maxAudioChannels":6`, `"maxAudioSampleRate":48000`, `"maxAudioBitDepth":24`, `"maxBitrateKbps":1000`)),
		viewer("coarsening: just below a step, the step below", pbCaps(`"maxHeight":1079`, `"maxAudioChannels":5`, `"maxAudioSampleRate":47999`, `"maxAudioBitDepth":23`, `"maxBitrateKbps":1999`)),
		viewer("coarsening: below the first step, the first", pbCaps(`"maxHeight":360`, `"maxAudioChannels":1`, `"maxAudioSampleRate":22050`, `"maxAudioBitDepth":8`)),
		viewer("coarsening: above the last step, the last", pbCaps(`"maxHeight":8640`, `"maxAudioChannels":24`, `"maxAudioSampleRate":384000`, `"maxAudioBitDepth":64`, `"maxBitrateKbps":10000000`)),
		viewer("coarsening: between steps", pbCaps(`"maxHeight":1600`, `"maxAudioChannels":7`, `"maxAudioSampleRate":88200`, `"maxAudioBitDepth":20`, `"maxBitrateKbps":25999`)),
		viewer("coarsening: 0 is not said", pbCaps(`"maxHeight":0`, `"maxAudioChannels":0`, `"maxAudioSampleRate":0`, `"maxAudioBitDepth":0`, `"maxBitrateKbps":0`)),
		viewer("coarsening: capabilities.maxBitrateKbps below 1,000 left out", pbCaps(`"maxBitrateKbps":999`, `"containers":["mp4"]`)),
		viewer("coarsening: the request's maxBitrateKbps from 1,000 up, rounded down", `"maxBitrateKbps":25999`),
		viewer("coarsening: the request's maxBitrateKbps at 1,000", `"maxBitrateKbps":1000`),
		viewer("coarsening: the request's maxBitrateKbps below 1,000, kept as it is", `"maxBitrateKbps":999`),
		viewer("coarsening: the request's maxBitrateKbps 0, kept", `"maxBitrateKbps":0`),
	}
}

func playbackDeliveryCases() []pbIn {
	film := `"title":"` + pbFilmKey + `"`
	parts := `"versionId":"Uk4nR7vT"`
	play := func(name string, members ...string) pbIn {
		return pbIn{name: name, text: pbObj(append([]string{film}, members...)...)}
	}
	tc := func(name, targets string, extra ...string) pbIn {
		return play(name, append([]string{`"delivery":"transcode"`, `"transcode":` + targets}, extra...)...)
	}
	return []pbIn{
		play("file of a copy of one file", `"delivery":"file"`),
		play("file of a copy of several parts: 409 delivery-refused", `"delivery":"file"`, parts),
		play("file of a copy of several parts, for a download: 409 delivery-refused", `"purpose":"download"`, `"delivery":"file"`, parts),
		play("file of a copy of several parts, for a scan: 409 delivery-refused", `"purpose":"scan"`, `"delivery":"file"`, parts),
		play("remux of a copy of several parts", `"delivery":"remux"`, `"container":"ts"`, parts),
		play("transcode of a copy of several parts", `"delivery":"transcode"`, parts),
		play("no delivery, a copy of several parts: the owning server decides (HLS)", parts),
		play("file at a cap equal to the copy's bitrate", `"delivery":"file"`, `"maxBitrateKbps":8000`),
		play("file at a cap below the copy's bitrate: 409 delivery-refused", `"delivery":"file"`, `"maxBitrateKbps":7999`),
		play("remux above the cap: 409 delivery-refused", `"delivery":"remux"`, `"container":"mkv"`, parts, `"maxBitrateKbps":20000`),
		play("file above the capabilities' cap: 409 delivery-refused", `"delivery":"file"`, pbCaps(`"maxBitrateKbps":7000`)),
		{name: "file above the pairing's bitrateKbps: 409 delivery-refused", text: pbObj(film, `"delivery":"file"`), limits: pbLim(func(l *playbackLimits) { l.BitrateKbps = 5000 })},
		{name: "the pairing's bitrateKbps is a cap: the lowest", text: pbObj(film, `"maxBitrateKbps":20000`), limits: pbLim(func(l *playbackLimits) { l.BitrateKbps = 9000 })},
		play("a transcode above the cap is not refused for bandwidth", `"delivery":"transcode"`, `"maxBitrateKbps":2000`),
		play("no delivery above the cap: the owning server decides", `"maxBitrateKbps":2000`),
		play("remux without container: 400", `"delivery":"remux"`),
		play("container with delivery file: 400", `"delivery":"file"`, `"container":"mp4"`),
		play("container without delivery: 400", `"container":"mp4"`),
		play("container in capitals: 400", `"delivery":"remux"`, `"container":"MKV"`),
		play("container other: 400", `"delivery":"remux"`, `"container":"other"`),
		play("container avi (known for conversion)", `"delivery":"remux"`, `"container":"avi"`),
		play("container that isn't a string: 400", `"delivery":"remux"`, `"container":["mkv"]`),
		tc("transcode, every target", `{"videoCodec":"hevc","maxHeight":2160,"maxBitrateKbps":30000,"audioCodec":"eac3","audioChannels":6,"burnSubtitle":"s5"}`),
		tc("transcode, no targets: each the owning server's choice", `{}`),
		tc("transcode, unknown members ignored", `{"maxHeight":720,"preset":"fast"}`),
		tc("transcode videoCodec outside the list: 400", `{"videoCodec":"h265"}`),
		tc("transcode videoCodec in capitals: 400", `{"videoCodec":"H264"}`),
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
		tc("transcode audioCodec outside the list: 400", `{"audioCodec":"AAC"}`),
		tc("transcode audioChannels 2", `{"audioChannels":2}`),
		tc("transcode audioChannels 8", `{"audioChannels":8}`),
		tc("transcode audioChannels not 2, 6 or 8: 400", `{"audioChannels":5}`),
		tc("transcode audioChannels as a string: 400", `{"audioChannels":"6"}`),
		tc("burnSubtitle a track of the copy", `{"burnSubtitle":"s4"}`),
		tc("burnSubtitle not a track of the copy: 409 delivery-refused", `{"burnSubtitle":"s9"}`),
		tc("burnSubtitle with a leading zero: 400", `{"burnSubtitle":"s04"}`),
		tc("burnSubtitle off: 400", `{"burnSubtitle":"off"}`),
		tc("burnSubtitle a number: 400", `{"burnSubtitle":4}`),
		tc("a transcode that isn't an object: 400", `"h264"`),
		play("a transcode object with delivery file: 400", `"delivery":"file"`, `"transcode":{}`),
		play("a transcode object without delivery: 400", `"transcode":{"maxHeight":720}`),
		{name: "transcode with the pairing's transcodes all used: 503 server-busy", text: pbObj(film, `"delivery":"transcode"`), inUse: pbUse(func(u *playbackUse) { u.Transcodes = 1 })},
		{name: "transcode with transcodes 0: 503 server-busy", text: pbObj(film, `"delivery":"transcode"`), limits: pbLim(func(l *playbackLimits) { l.Transcodes = 0 })},
		{name: "transcode with the server's budget used: 503 server-busy", text: pbObj(film, `"delivery":"transcode"`), inUse: pbUse(func(u *playbackUse) { u.ServerTranscodesFree = 0 })},
		{name: "remux with the transcodes all used: no transcode needed", text: pbObj(film, `"delivery":"remux"`, `"container":"mkv"`), inUse: pbUse(func(u *playbackUse) { u.Transcodes = 1; u.ServerTranscodesFree = 0 })},
		{name: "file with the transcodes all used: no transcode needed", text: pbObj(film, `"delivery":"file"`), inUse: pbUse(func(u *playbackUse) { u.Transcodes = 1; u.ServerTranscodesFree = 0 })},
		{name: "streams all in use before delivery: 409 stream-limit-reached", text: pbObj(film, `"delivery":"file"`, parts), inUse: pbUse(func(u *playbackUse) { u.Streams = 2 })},
		{name: "a track, file", text: pbObj(`"title":"`+pbTrackKeyA+`"`, `"delivery":"file"`)},
		{name: "a track, transcode: video targets read, ignored for a title without a picture", text: pbObj(`"title":"`+pbTrackKeyA+`"`, `"delivery":"transcode"`, `"transcode":{"videoCodec":"h264","maxHeight":720,"audioCodec":"aac"}`)},
	}
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
		{name: "remux: 400", text: dl(`"delivery":"remux"`, `"container":"mp4"`)},
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
		{name: "file: scansPerDay, a 10-minute ticket, the idle rule, 32 MiB", text: file},
		{name: "a track", text: pbObj(`"title":"`+pbTrackKeyA+`"`, `"purpose":"scan"`, `"delivery":"file"`)},
		{name: "no delivery: 400", text: sc()},
		{name: "remux: 400", text: sc(`"delivery":"remux"`, `"container":"mkv"`)},
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
		p("progress: position and paused, at absent means now", active, idm, `"positionMs":600000`, `"paused":true`),
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
		p("paused that isn't a boolean: 400", active, idm, `"paused":"yes"`),
		p("no id: 400", active, `"positionMs":0`),
		p("an id of another form: 400", active, `"id":"play_ABCDEFGHIJKLMNOPQRSTUVWXYZ"`),
		p("an id one character short: 400", active, `"id":"play_abcdefghijklmnopqrstuvwxy"`),
		p("an id with 1, outside base32: 400", active, `"id":"play_abcdefghijklmnopqrstuvwxy1"`),
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
	hls := `"stream":{"url":"` + pbRoot + `Tk3vQ9xY_-a/master.m3u8","protocol":"hls","mimeType":"application/vnd.apple.mpegurl"}`
	file := `"stream":{"url":"` + pbRoot + `Tk3vQ9xY_-a/file","protocol":"file"}`
	etag := `"etag":"9c1e4f70"`
	pos := `"positionMs":0`
	tr := func(extra ...string) string {
		return pbObj(append([]string{id, `"method":"transcode"`, hls, etag, pos}, extra...)...)
	}
	url := func(u string) string {
		return pbObj(id, `"method":"transcode"`, `"stream":{"url":"`+u+`","protocol":"hls"}`, etag, pos)
	}
	long := strings.Repeat("A", 64)
	type in struct{ name, delivery, text string }
	list := []in{
		{"every member of playback, cleaned", "", pbObj(id, `"method":"transcode"`, hls, etag, `"positionMs":60000`, `"durationMs":8160000`,
			`"versions":[{"id":"Hd8kQ2mZ","name":"1080p"},{"id":"Uk4nR7vT","name":"4K HDR"}]`, `"versionId":"Hd8kQ2mZ"`,
			`"audioTracks":[{"id":"a1","language":"en","channels":6,"default":true},{"id":"a2","language":"fr-CA","channels":2},{"id":"a3","language":"en","channels":2,"role":"commentary"}]`,
			`"audioTrack":"a1"`, `"maxBitrateKbps":20000`, `"info":{"reasons":["codec"],"container":"ts","bitrateKbps":8000}`)},
		{"the required members alone", "", tr()},
		{"delivery file: method direct, the file", "file", pbObj(id, `"method":"direct"`, file, etag, pos)},
		{"delivery remux: method remux, HLS", "remux", pbObj(id, `"method":"remux"`, hls, etag, pos)},
		{"no delivery: direct as HLS", "", pbObj(id, `"method":"direct"`, hls, etag, pos)},
		{"no delivery: direct as the file", "", pbObj(id, `"method":"direct"`, file, etag, pos)},
		{"delivery file, method remux: refused", "file", pbObj(id, `"method":"remux"`, hls, etag, pos)},
		{"delivery file, method direct as HLS: refused", "file", pbObj(id, `"method":"direct"`, hls, etag, pos)},
		{"delivery transcode, method direct: refused", "transcode", pbObj(id, `"method":"direct"`, file, etag, pos)},
		{"a transcode as the file: refused", "", pbObj(id, `"method":"transcode"`, file, etag, pos)},
		{"method in capitals: refused", "", pbObj(id, `"method":"Direct"`, file, etag, pos)},
		{"an unknown method: refused", "", pbObj(id, `"method":"copy"`, hls, etag, pos)},
		{"no method: refused", "", pbObj(id, hls, etag, pos)},
		{"an id in capitals: refused", "", pbObj(`"id":"play_ABCDEFGHIJKLMNOPQRSTUVWXYZ"`, `"method":"transcode"`, hls, etag, pos)},
		{"an id of 27 characters after play_: refused", "", pbObj(`"id":"`+pbPID+`a"`, `"method":"transcode"`, hls, etag, pos)},
		{"no id: refused", "", pbObj(`"method":"transcode"`, hls, etag, pos)},
		{"stream under another root: refused", "", url("https://b.example.org/api/v1/stream/Tk3vQ9xY/master.m3u8")},
		{"stream over http: refused", "", url("http://a.example.org/api/v1/stream/Tk3vQ9xY/master.m3u8")},
		{"stream with a ticket of 64 characters", "", url(pbRoot + long + "/master.m3u8")},
		{"stream with a ticket of 65 characters: refused", "", url(pbRoot + long + "A/master.m3u8")},
		{"stream with a dot in the ticket: refused", "", url(pbRoot + "Tk3.vQ9/master.m3u8")},
		{"stream with another name: refused", "", url(pbRoot + "Tk3vQ9xY/index.m3u8")},
		{"stream with a part more: refused", "", url(pbRoot + "Tk3vQ9xY/x/master.m3u8")},
		{"stream with a query: refused", "", url(pbRoot + "Tk3vQ9xY/master.m3u8?x=1")},
		{"stream with a percent: refused", "", url(pbRoot + "Tk3vQ9xY%2F/master.m3u8")},
		{"stream protocol hls with the name file: refused", "", url(pbRoot + "Tk3vQ9xY/file")},
		{"stream protocol dash: refused", "", pbObj(id, `"method":"transcode"`, `"stream":{"url":"`+pbRoot+`Tk3vQ9xY/master.m3u8","protocol":"dash"}`, etag, pos)},
		{"stream mimeType other than HLS's: refused", "", pbObj(id, `"method":"transcode"`, `"stream":{"url":"`+pbRoot+`Tk3vQ9xY/master.m3u8","protocol":"hls","mimeType":"video/mp2t"}`, etag, pos)},
		{"stream that isn't an object: refused", "", pbObj(id, `"method":"transcode"`, `"stream":"`+pbRoot+`Tk3vQ9xY/master.m3u8"`, etag, pos)},
		{"no etag: refused", "", pbObj(id, `"method":"transcode"`, hls, pos)},
		{"an empty etag: refused", "", pbObj(id, `"method":"transcode"`, hls, `"etag":""`, pos)},
		{"an etag of 64 characters", "", pbObj(id, `"method":"transcode"`, hls, `"etag":"`+strings.Repeat("e", 64)+`"`, pos)},
		{"an etag of 65 characters: refused", "", pbObj(id, `"method":"transcode"`, hls, `"etag":"`+strings.Repeat("e", 65)+`"`, pos)},
		{"an etag with a dot: refused", "", pbObj(id, `"method":"transcode"`, hls, `"etag":"9c1e.4f70"`, pos)},
		{"an etag in quotes, as in a header: refused", "", pbObj(id, `"method":"transcode"`, hls, `"etag":"\"9c1e4f70\""`, pos)},
		{"an etag that isn't a string: refused", "", pbObj(id, `"method":"transcode"`, hls, `"etag":91`, pos)},
		{"another etag than the catalog's: kept, the answer wins, the title read again", "", pbObj(id, `"method":"transcode"`, hls, `"etag":"0d0d0d0d"`, pos)},
		{"no positionMs: refused", "", pbObj(id, `"method":"transcode"`, hls, etag)},
		{"positionMs at 100 days", "", pbObj(id, `"method":"transcode"`, hls, etag, `"positionMs":8640000000`)},
		{"positionMs above 100 days: refused", "", pbObj(id, `"method":"transcode"`, hls, etag, `"positionMs":8640000001`)},
		{"a negative positionMs: refused", "", pbObj(id, `"method":"transcode"`, hls, etag, `"positionMs":-1`)},
		{"positionMs as a string: refused", "", pbObj(id, `"method":"transcode"`, hls, etag, `"positionMs":"0"`)},
		{"durationMs at 100 days", "", tr(`"durationMs":8640000000`)},
		{"durationMs above 100 days: set to 0", "", tr(`"durationMs":8640000001`)},
		{"a negative durationMs: set to 0", "", tr(`"durationMs":-5`)},
		{"durationMs with a fraction: refused", "", tr(`"durationMs":2.5`)},
		{"durationMs null: refused", "", tr(`"durationMs":null`)},
		{"versions: bad IDs and entries dropped, names made plain and cut to 32", "", tr(`"versions":[{"id":"ok_1","name":"  Director's\tcut​ "},{"id":"a b","name":"x"},7,{"name":"no id"},{"id":"` + long + `","name":"` + strings.Repeat("n", 40) + `"},{"id":"` + long + `A"},{"id":"v2","name":5}]`)},
		{"versions that isn't an array: refused", "", tr(`"versions":{"id":"v1"}`)},
		{"versionId of another form: cleared", "", tr(`"versionId":"../v1"`)},
		{"versionId that isn't a string: refused", "", tr(`"versionId":1`)},
		{"audioTracks: bad IDs dropped, off kept, unknown members and bad values dropped", "", tr(`"audioTracks":[{"id":"a 1"},{"id":"off"},{"id":"a1","label":"English","codec":"aac","role":"director","channels":"6","default":"true","forced":false,"hearingImpaired":true},{"id":"a2","language":"not_a_tag"},"a3",{"id":1}]`)},
		{"track languages kept only as valid tags", "", tr(`"audioTracks":[{"id":"a1","language":"fr-CA"},{"id":"a2","language":"fre"},{"id":"a3","language":"fr-ca"},{"id":"a4","language":"e"},{"id":"a5","language":"zh-Hant-TW"}]`)},
		{"audioTracks that isn't an array: refused", "", tr(`"audioTracks":"a1"`)},
		{"audioTrack of another form: cleared", "", tr(`"audioTrack":"a 1"`)},
		{"maxBitrateKbps above 10,000,000: dropped", "", tr(`"maxBitrateKbps":10000001`)},
		{"maxBitrateKbps that isn't an integer: refused", "", tr(`"maxBitrateKbps":"20000"`)},
		{"title and next dropped whatever they hold", "", tr(`"title":"movie/tmdb/603"`, `"next":{"title":"movie/tmdb/604"}`)},
		{"an answer that isn't an object: refused", "", `[]`},
	}
	var out []playbackAnswerCase
	for _, c := range list {
		got, err := playbackKeepAnswer([]byte(c.text), c.delivery, "9c1e4f70", pbRoot)
		ok, r := reason(err)
		out = append(out, playbackAnswerCase{Name: c.name, Delivery: c.delivery, CatalogEtag: "9c1e4f70", Text: c.text, Valid: ok, Output: string(got), Reason: r})
	}
	return out
}

func playbackRestartCases() []playbackRestartCase {
	reading := playbackReading{VersionID: "Hd8kQ2mZ", Etag: "9c1e4f70", Purpose: "play", Delivery: "file", Offset: 104857600}
	req := `{"title":"` + pbFilmKey + `","versionId":"Hd8kQ2mZ","delivery":"file"}`
	ans := func(etag, method, stream string) string {
		return pbObj(`"id":"play_bbbbbbbbbbbbbbbbbbbbbbbbbb"`, `"method":"`+method+`"`, `"stream":{"url":"`+pbRoot+`Nw7tK2/`+stream+`","protocol":"`+map[string]string{"file": "file", "master.m3u8": "hls"}[stream]+`"}`, `"etag":"`+etag+`"`, `"positionMs":0`)
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
		{"the new answer refused (method remux): stopped, the read failed", reading, req, 200, "", ans("9c1e4f70", "remux", "master.m3u8")},
		{"the new answer refused (a bad etag): stopped, the read failed", reading, req, 200, "", ans("9c1e.4f70", "direct", "file")},
		{"a play of another copy: not a restart", reading, `{"title":"` + pbFilmKey + `","versionId":"Lw3pX9cJ","delivery":"file"}`, 200, "", ok},
		{"a play without versionId: not a restart", reading, `{"title":"` + pbFilmKey + `","delivery":"file"}`, 200, "", ok},
		{"a play of another purpose: not a restart", reading, `{"title":"` + pbFilmKey + `","purpose":"download","versionId":"Hd8kQ2mZ","delivery":"file"}`, 200, "", ok},
		{"a play of another delivery: not a restart", reading, `{"title":"` + pbFilmKey + `","versionId":"Hd8kQ2mZ","delivery":"remux","container":"mkv"}`, 200, "", ans("9c1e4f70", "remux", "master.m3u8")},
		{"a reading of a remux: never restarted", remux, `{"title":"` + pbFilmKey + `","versionId":"Hd8kQ2mZ","delivery":"remux","container":"mkv"}`, 200, "", ans("9c1e4f70", "remux", "master.m3u8")},
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
		{"every member at its top", `{"streams":50,"transcodes":50,"bitrateKbps":1000000,"downloads":50,"downloadsPerDay":10000,"downloadKbps":1000000,"scansPerDay":1000000}`},
		{"every member above its top: each ignored alone", `{"streams":51,"transcodes":51,"bitrateKbps":1000001,"downloads":51,"downloadsPerDay":10001,"downloadKbps":1000001,"scansPerDay":1000001}`},
		{"negative members: each ignored alone", `{"streams":-1,"downloads":-1,"scansPerDay":-1,"transcodes":0}`},
		{"transcodes above streams: ignored", `{"streams":2,"transcodes":3}`},
		{"transcodes equal to streams", `{"streams":3,"transcodes":3}`},
		{"downloads 0 with a daily cap: no downloads", `{"downloads":0,"downloadsPerDay":10,"downloadKbps":20000}`},
		{"scansPerDay 1: scan access", `{"scansPerDay":1}`},
		{"music's audioStreams and audioTranscodes", `{"streams":2,"transcodes":1,"audioStreams":4,"audioTranscodes":2}`},
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
			"MSFP playback part version 0 (spec/v0/playback.md, with video section 5.1's request members and capabilities, music section 5.2's title in progress and stop, media section 1's stream URLs), as revised by proposal 16. Generated by tools/vectors; no implementation has produced them yet.",
			"context: the owning server's copies of each title it offers the caller (parts above 1: a copy of several files), its own bitrate cap (ownCapKbps, 0: none), the pairing's limits and what is in use, its stream root, and now (Unix seconds). A case's limits or inUse replaces the context's for that case.",
			"requests, deliveries, downloads, scans, reports: call play, progress or stop; text is the request body as received. valid true: status 200 and output; valid false: status and type, the refusal (playback sections 2.2, 2.5, 3, 4). For play, output is what the owning server starts: the request as read (capabilities with unknown entries ignored, each once, sorted; 0 left out; dv only with dvProfiles), the copy (versionId) and audio (audioTrack) of section 2.6, the start position, the bitrate cap of step 6 (play only), the speed cap (downloadKbps, downloads only), what it counts against, which transcodes it takes, the ticket's lifetime and idle rule, a scan's read cap. Members in that order, absent ones left out. play doesn't model the choice of method without delivery, nor music's queue (in music.json).",
			"requests with by \"viewer\": text is what the device says, output what the viewer's server sends once coarsened (playback section 2.1, video section 5.1); always valid.",
			"progress and stop cases carry playback: the owning server's playback the id names (state started, active or ended; unknown: another pairing's or never given), its purpose, title and queue, when its play was answered (playedAt), the at of the latest report it has (lastAt). output: effect (recorded, recorded and ended, ended, ignored, unchanged), the state after, and what is recorded.",
			"answers: text is a play answer, delivery what the request asked, catalogEtag the copy's etag in the catalog. valid false: the viewer's server sends stop and gives up. valid true: output is what it keeps of playback's own members, in the order id, method, stream, etag, positionMs, durationMs, versions, versionId, audioTracks, audioTrack, maxBitrateKbps, then readTitleAgain when the etag isn't the catalog's. info, title and next are not kept; video's members (subtitleTracks and the rest) are video's.",
			"restarts: a viewer's server reading a playback's file (reading: its copy, etag, purpose, delivery and the byte it was at) sends request, a new play; status (and type) is the answer's, answer its body when 200. valid true: it goes on reading at offset; false: output says whether it stops the new playback, fails the read and reads the title again from titles (playback section 2.7).",
			"limits: text is the limits object of a libraries answer; valid false: ignored whole; output is what the caller keeps, the new members 0 when absent, and whether it offers downloads and asks for scans (playback section 5).",
		},
		Context:    ctx,
		Requests:   pbJudge(ctx, playbackRequestCases()),
		Answers:    playbackAnswerCases(),
		Deliveries: pbJudge(ctx, playbackDeliveryCases()),
		Restarts:   playbackRestartCases(),
		Reports:    pbJudge(ctx, playbackReportCases()),
		Downloads:  pbJudge(ctx, playbackDownloadCases()),
		Scans:      pbJudge(ctx, playbackScanCases()),
		Limits:     playbackLimitsCases(),
	}
}

// genV0Playback makes vectors/v0/playback.json (spec/v0/playback.md).
func genV0Playback() []byte { return encode(playbackFile()) }
