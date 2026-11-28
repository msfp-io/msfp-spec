package main

import "msfp-media/tools/vectors/rules"

// playbackCase is one call and its outcome, as MSFP Media's cases
// (rules.Case) with scanBytes: a scan's cap (playback section 8).
type playbackCase struct {
	Name      string          `json:"name"`
	Call      string          `json:"call"`
	Limits    *playbackLimits `json:"limits,omitempty"`
	InUse     *playbackUse    `json:"inUse,omitempty"`
	Ranges    []string        `json:"ranges,omitempty"`
	ScanBytes int64           `json:"scanBytes,omitempty"`
	Progress  int64           `json:"progressSeconds,omitempty"`
	Playback  *playbackState  `json:"playback,omitempty"`
	Text      string          `json:"text"`
	Valid     bool            `json:"valid"`
	Status    int             `json:"status,omitempty"`
	Type      string          `json:"type,omitempty"`
	Output    string          `json:"output,omitempty"`
	Reason    string          `json:"reason,omitempty"`
}

type playbackLimitsCase struct {
	Name   string `json:"name"`
	Text   string `json:"text"`
	Valid  bool   `json:"valid"`
	Output string `json:"output,omitempty"`
	Reason string `json:"reason,omitempty"`
}

type v0PlaybackFile struct {
	About      []string             `json:"about"`
	Context    msfpContext          `json:"context"`
	Requests   []playbackCase       `json:"requests"`
	Deliveries []playbackCase       `json:"deliveries"`
	Answers    []playbackAnswerCase `json:"answers"`
	Scans      []playbackCase       `json:"scans"`
	Limits     []playbackLimitsCase `json:"limits"`
}

func playbackRequestCases() []pbIn {
	film := `"title":"` + pbFilmKey + `"`
	play := func(name string, members ...string) pbIn { return pbIn{name: name, text: pbObj(members...)} }
	return []pbIn{
		play("capabilities, a capability list: 400, MSFP takes none (MSFP playback section 2)", film, `"capabilities":{"files":["video/mp4"],"segments":["video/mp4"],"video":[{"codec":"h264","codecs":["avc1.640028"]}],"audio":[{"codec":"aac","codecs":["mp4a.40.2"]}]}`),
		play("capabilities empty: 400, MSFP takes none", film, `"capabilities":{}`),
		play("capabilities of another form: 400, MSFP takes none", film, `"capabilities":{"containers":["mp4"],"videoCodecs":"h264","maxHeight":-1}`),
		play("capabilities null: 400", film, `"capabilities":null`),
		play("capabilities with delivery: 400", film, `"delivery":"file"`, `"capabilities":{"files":["video/mp4"]}`),
		{name: "order: capabilities with streams 0 is 400 (step 1 first)", text: pbObj(film, `"capabilities":{}`), limits: pbLim(func(l *playbackLimits) { l.Streams = 0 })},
		play("scan without delivery: 400", film, `"purpose":"scan"`),
		play("scan with delivery file", film, `"purpose":"scan"`, `"delivery":"file"`),
		play("scan with delivery remux: 400", film, `"purpose":"scan"`, `"delivery":"remux"`, `"mediaType":"video/mp2t"`),
		play("scan with delivery transcode: 400", film, `"purpose":"scan"`, `"delivery":"transcode"`, `"transcode":{"maxHeight":480}`),
	}
}

// playbackDeliveryCases are MSFP's delivery cases: a scan's file (MSFP
// playback section 8).
func playbackDeliveryCases() []pbIn {
	film := `"title":"` + pbFilmKey + `"`
	parts := `"versionId":"Uk4nR7vT"`
	play := func(name string, members ...string) pbIn {
		return pbIn{name: name, text: pbObj(append([]string{film}, members...)...)}
	}
	return []pbIn{
		play("file of a copy of several parts, for a scan: 409 delivery-refused", `"purpose":"scan"`, `"delivery":"file"`, parts),
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
		{name: "scansPerDay 0: 409 stream-limit-reached", text: file, limits: pbLim(func(l *playbackLimits) { l.Own = rules.SetCount(l.Own, "scansPerDay", 0) })},
		{name: "scansPerDay reached: 409 stream-limit-reached", text: file, inUse: pbUse(func(u *playbackUse) { u.Own = rules.SetCount(u.Own, "scansToday", 1000) })},
		{name: "one scan left today", text: file, inUse: pbUse(func(u *playbackUse) { u.Own = rules.SetCount(u.Own, "scansToday", 999) })},
		{name: "scansPerDay at 1,000,000", text: file, limits: pbLim(func(l *playbackLimits) { l.Own = rules.SetCount(l.Own, "scansPerDay", 1000000) }), inUse: pbUse(func(u *playbackUse) { u.Own = rules.SetCount(u.Own, "scansToday", 999999) })},
		{name: "streams all in use: a scan doesn't count against streams", text: file, inUse: pbUse(func(u *playbackUse) { u.Streams = 2; u.Downloads = 1 })},
		{name: "streams and downloads 0: a scan counts against scansPerDay only", text: file, limits: pbLim(func(l *playbackLimits) { l.Streams = 0; l.Transcodes = 0; l.Downloads = 0 })},
		{name: "no bitrate cap of play: a file above maxBitrateKbps", text: sc(`"delivery":"file"`, `"maxBitrateKbps":1000`)},
		{name: "progress of a live scan: 200, nothing changes", call: "progress", playback: live, text: pbObj(`"id":"`+pbPID+`"`, `"positionMs":0`)},
		{name: "stop of a live scan: ends it", call: "stop", playback: live, text: id},
		{name: "progress of a scan past its 10 minutes: 410 playback-stopped", call: "progress", playback: &playbackState{State: "ended", Purpose: "scan", Title: pbFilmKey, PlayedAt: pbNow - 601}, text: id},
	}
}

// playbackAnswerCases are MSFP's answer cases: scanBytes, the member
// MSFP adds to a scan's answer (MSFP playback section 8).
func playbackAnswerCases() []playbackAnswerCase {
	id := `"id":"` + pbPID + `"`
	tk := `"ticket":"Tk3vQ9xY_-a"`
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
	// ans is an answer of the given members, the others the defaults.
	ans := func(method, protocol string, extra ...string) string {
		return pbObj(append([]string{id, tk, protocol, `"method":"` + method + `"`, etag, pos, ver}, extra...)...)
	}
	list := []struct {
		name  string
		asked playbackAsked
		text  string
	}{
		{"a scan's scanBytes, 32 MiB: kept", scan, ans("direct", file, `"scanBytes":33554432`)},
		{"a scan's scanBytes at 1 GiB", scan, ans("direct", file, `"scanBytes":1073741824`)},
		{"a scan's scanBytes at its floor, 4 MiB", scan, ans("direct", file, `"scanBytes":4194304`)},
		{"a scan without scanBytes: 4 MiB", scan, ans("direct", file)},
		{"a scan's scanBytes below 4 MiB: 4 MiB", scan, ans("direct", file, `"scanBytes":4194303`)},
		{"a scan's scanBytes above 1 GiB: 4 MiB", scan, ans("direct", file, `"scanBytes":1073741825`)},
		{"a scan's scanBytes as a string: 4 MiB", scan, ans("direct", file, `"scanBytes":"33554432"`)},
		{"scanBytes in the answer of a play: ignored", delivery("file"), ans("direct", file, `"scanBytes":33554432`)},
	}
	var out []playbackAnswerCase
	for _, c := range list {
		got, err := playbackKeepAnswer([]byte(c.text), c.asked, pbRoot, msfpCarrier(0))
		ok, r := reason(err)
		out = append(out, playbackAnswerCase{Name: c.name, Asked: c.asked, Text: c.text, Valid: ok, Output: string(got), Reason: r})
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
			"MSFP's playback part (spec/v0/playback.md): what MSFP decides of MSFP Media's playing as it carries it between servers, its carrier vectors. The purpose scan (playback section 8), the refusal of a capability list (playback section 2), the scanBytes of a scan's answer, and the limits of a libraries answer (playback section 5). The rest of playing's cases are MSFP Media's, in https://github.com/msfp-io/msfp-media/blob/main/vectors/v0/playing.json (MSFP's refusal over a pairing's limits, 409 stream-limit-reached, is its limitRefusal). The owning server is playing's source, the viewer's server its caller. Generated by tools/vectors; no implementation has produced them yet.",
			"context: the owning server's copies of each title it offers the caller (parts above 1: a copy of several files; size and durationMs give the overall rate, size × 8 / durationMs kbit/s; the video stream, absent for a copy with no video, and the audio tracks with the facts the baseline goes by), the codecs it encodes (videoEncoders, audioEncoders: a transcode codec outside them is refused with 409 target-refused), the ranges it makes (another: 409 target-refused), the codecs each segment format carries (segmentCodecs: a remux, or a copied stream, outside them is refused with 409 target-refused), its own bitrate cap (ownCapKbps, 0: none), the pool's scan cap it states as scanBytes, the interval of progress its answers state (progressSeconds, 45 to 120; another value: 60), the titles it offered this caller and no longer holds (gone: 404 gone), the titles whose media it can't reach now (unreachable: 503 unreachable, at step 8), the pairing's limits and what is in use, its stream root, and now (Unix seconds). A case's limits, inUse, ranges, scanBytes or progressSeconds replaces the context's for that case.",
			"requests, deliveries, scans: call play, progress or stop; text is the request body as received. valid true: status 200 and output; valid false: status and type, the refusal, as in MSFP Media's playing.json. capabilities is refused with 400 whatever it holds: MSFP takes no capability list (playback section 2; MSFP Media playing section 2.8). A scan takes delivery file and nothing else (playback section 8), counts against scansPerDay, and its ticket works 10 minutes. Over the pairing's limits, or with a limit of 0: 409 stream-limit-reached.",
			"For play, output is what the owning server starts, members as in MSFP Media's playing.json, and for a scan the scanBytes its answer states, before the playback it replaces.",
			"progress and stop cases carry playback: the owning server's playback the id names (state started, active or ended; unknown: another pairing's or never given), its purpose, title and queue, when its play was answered (playedAt), the at of the latest report it has (lastAt). output: effect (recorded, recorded and ended, ended, ignored, unchanged), the state after, and what is recorded (paused when a progress says true).",
			"answers: text is a play answer; asked is what the viewer's server asked (purpose, delivery) and knows, as in MSFP Media's playing.json; output is what it keeps, with scanBytes after progressSeconds for a scan alone: the answer's when an integer from 4 MiB to 1 GiB, else 4 MiB, 4,194,304 bytes (playback section 8). In the answer of a play, scanBytes is unknown, not kept.",
			"limits: text is the limits object of a libraries answer; valid false: ignored whole; output is what the caller keeps, the new members 0 when absent, and whether it offers downloads and asks for scans (MSFP playback section 5). scanBytes is no member of limits since proposal 17.",
		},
		Context:    ctx,
		Requests:   pbJudge(ctx, playbackRequestCases()),
		Deliveries: pbJudge(ctx, playbackDeliveryCases()),
		Answers:    playbackAnswerCases(),
		Scans:      pbJudge(ctx, playbackScanCases()),
		Limits:     playbackLimitsCases(),
	}
}

// genV0Playback makes vectors/v0/playback.json (MSFP Media's playing, as
// spec/v0/playback.md carries it).
func genV0Playback() []byte { return encode(playbackFile()) }
