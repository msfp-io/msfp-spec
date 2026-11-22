package main

import (
	"fmt"
	"strings"
)

// The libraries the caller asked for, in every title case, and the
// owning server's stream root in the previews cases.
var (
	videoLibraries  = []string{"v1", "v2"}
	videoStreamRoot = "https://a.example.org/api/v1/stream/"
)

// videoCase is one entry as received and what the caller keeps of it.
type videoCase struct {
	Name   string `json:"name"`
	Text   string `json:"text"`
	Valid  bool   `json:"valid"`
	Output string `json:"output,omitempty"`
	Reason string `json:"reason,omitempty"`
}

// videoKeyCase is one title key and the kind it names.
type videoKeyCase struct {
	Name   string `json:"name"`
	Text   string `json:"text"`
	Valid  bool   `json:"valid"`
	Kind   string `json:"kind,omitempty"`
	Reason string `json:"reason,omitempty"`
}

// videoPreviewsCase is a previews request or answer.
type videoPreviewsCase struct {
	Name   string `json:"name"`
	Side   string `json:"side"`
	Text   string `json:"text"`
	Valid  bool   `json:"valid"`
	Output string `json:"output,omitempty"`
	Reason string `json:"reason,omitempty"`
}

type videoFile struct {
	About       []string            `json:"about"`
	Libraries   []string            `json:"libraries"`
	StreamRoot  string              `json:"streamRoot"`
	Titles      []videoCase         `json:"titles"`
	Copies      []videoCase         `json:"copies"`
	Extras      []videoCase         `json:"extras"`
	LocalTitles []videoCase         `json:"localTitles"`
	LocalKeys   []videoKeyCase      `json:"localKeys"`
	TVDBKeys    []videoKeyCase      `json:"tvdbKeys"`
	Previews    []videoPreviewsCase `json:"previews"`
}

type videoInput struct{ name, text string }

func videoReason(err error) (bool, string) {
	if err == nil {
		return true, ""
	}
	return false, err.Error()
}

func videoTitleSet(in []videoInput) []videoCase {
	var out []videoCase
	for _, c := range in {
		got, err := keepVideoTitle([]byte(c.text), videoLibraries)
		ok, r := videoReason(err)
		out = append(out, videoCase{c.name, c.text, ok, string(got), r})
	}
	return out
}

func videoKeySet(in []videoInput) []videoKeyCase {
	var out []videoKeyCase
	for _, c := range in {
		kind, err := checkVideoKey(c.text)
		ok, r := videoReason(err)
		out = append(out, videoKeyCase{c.name, c.text, ok, kind, r})
	}
	return out
}

// videoCore is a copy's required core with the given audio tracks.
func videoCore(audio string) string {
	return `{"id":"v1","durationMs":3600000,"container":"mkv","size":1000000,"etag":"t1","audioTracks":[` + audio + `]`
}

// videoCp is a copy with the required core and members (",..." or "").
func videoCp(members string) string { return videoCore("") + members + `}` }

// videoVid is a copy whose video has codec, width, height and members.
func videoVid(members string) string {
	return videoCp(`,"video":{"codec":"hevc","width":1920,"height":1080` + members + `}`)
}

func videoQuoted(n int, f func(i int) string) string {
	var l []string
	for i := 0; i < n; i++ {
		l = append(l, f(i))
	}
	return strings.Join(l, ",")
}

// A copy every title case uses.
var videoC = `{"id":"v1","durationMs":8160000,"container":"mkv","size":1000,"etag":"t1","audioTracks":[{"id":"a1","codec":"aac","language":"en"}]}`

func videoCopyCases() []videoCase {
	long := strings.Repeat("x", 64)
	example := `{"id":"v_2HqXo3","name":"4K HDR","durationMs":8160000,"container":"mkv","size":48213884928,"etag":"k9Tq2w","bitrateKbps":47000,"video":{"codec":"hevc","width":3840,"height":1600,"bitDepth":10,"hdr":["hdr10","dv"],"dv":{"profile":7,"level":6,"compatibility":6},"fps":23.98},"audioTracks":[{"id":"a1","language":"en","channels":8,"codec":"truehd","profile":"atmos","default":true,"original":true}],"subtitleTracks":[{"id":"s2","language":"fr","format":"pgs"}],"chapters":[{"startMs":0},{"startMs":312000,"name":"The Desert"}],"previews":true}`
	in := []videoInput{
		{"the example of section 3.2", example},
		{"the required core only, no audio track", videoCp("")},
		{"every member", `{"id":"v2","name":"Director's cut","edition":"directors","durationMs":7200000,"startMs":0,"container":"mp4","size":3000,"etag":"t2","parts":[{"size":1000,"durationMs":3600000},{"size":2000,"durationMs":3600000}],"bitrateKbps":8000,"video":{"codec":"h264","codecTag":"avc1","profile":"high","level":4.1,"width":1920,"height":1080,"bitDepth":8,"fps":24,"interlaced":false,"colour":{"primaries":"bt709","transfer":"bt709","matrix":"bt709","range":"tv"}},"audioTracks":[{"id":"a0","language":"fr","channels":6,"role":"commentary","default":false,"forced":false,"fileIndex":1,"codec":"ac3","layout":"5.1(side)","sampleRate":48000,"bitDepth":16,"title":"Commentary","original":false}],"subtitleTracks":[{"id":"s0","language":"en","default":true,"forced":false,"hearingImpaired":true,"format":"subrip","external":true,"title":"SDH"}],"chapters":[{"startMs":0,"name":"Opening"}],"segments":[{"kind":"intro","startMs":1000,"endMs":60000},{"kind":"outro","startMs":7000000,"endMs":7200000,"final":true}],"previews":false,"added":"2024-02-29","threeD":"sbs"}`},
		{"unknown members of a copy, a stream and a track ignored", `{"id":"v1","durationMs":3600000,"container":"mkv","size":1000000,"etag":"t1","path":"/media/a.mkv","video":{"codec":"hevc","width":1920,"height":1080,"label":"x"},"audioTracks":[{"id":"a1","codec":"aac","label":"English"}]}`},

		// The required core.
		{"id: 64 characters", strings.Replace(videoCp(""), `"v1"`, `"`+long+`"`, 1)},
		{"id: _ and -", strings.Replace(videoCp(""), `"v1"`, `"_-"`, 1)},
		{"id: 65 characters: skipped", strings.Replace(videoCp(""), `"v1"`, `"`+long+`x"`, 1)},
		{"id: empty: skipped", strings.Replace(videoCp(""), `"v1"`, `""`, 1)},
		{"id: a dot: skipped", strings.Replace(videoCp(""), `"v1"`, `"v.1"`, 1)},
		{"id: not a string: skipped", strings.Replace(videoCp(""), `"v1"`, `1`, 1)},
		{"id missing: skipped", `{"durationMs":3600000,"container":"mkv","size":1000000,"etag":"t1","audioTracks":[]}`},
		{"durationMs: 0", strings.Replace(videoCp(""), `3600000`, `0`, 1)},
		{"durationMs: 8,640,000,000 (100 days)", strings.Replace(videoCp(""), `3600000`, `8640000000`, 1)},
		{"durationMs: 8,640,000,001: skipped", strings.Replace(videoCp(""), `3600000`, `8640000001`, 1)},
		{"durationMs: negative: skipped", strings.Replace(videoCp(""), `3600000`, `-1`, 1)},
		{"durationMs: 3600000.0 isn't an integer: skipped", strings.Replace(videoCp(""), `3600000`, `3600000.0`, 1)},
		{"durationMs: a string: skipped", strings.Replace(videoCp(""), `3600000`, `"3600000"`, 1)},
		{"durationMs missing: skipped", `{"id":"v1","container":"mkv","size":1000000,"etag":"t1","audioTracks":[]}`},
		{"container: one known for conversion only", strings.Replace(videoCp(""), `"mkv"`, `"avi"`, 1)},
		{"container: unknown, kept as other", strings.Replace(videoCp(""), `"mkv"`, `"matroska"`, 1)},
		{"container: another case is unknown, kept as other", strings.Replace(videoCp(""), `"mkv"`, `"MKV"`, 1)},
		{"container: not a string: skipped", strings.Replace(videoCp(""), `"mkv"`, `7`, 1)},
		{"container missing: skipped", `{"id":"v1","durationMs":3600000,"size":1000000,"etag":"t1","audioTracks":[]}`},
		{"size: 1", strings.Replace(videoCp(""), `1000000`, `1`, 1)},
		{"size: 999,999,999,999,999", strings.Replace(videoCp(""), `1000000`, `999999999999999`, 1)},
		{"size: 0: skipped", strings.Replace(videoCp(""), `1000000`, `0`, 1)},
		{"size: 1,000,000,000,000,000: skipped", strings.Replace(videoCp(""), `1000000`, `1000000000000000`, 1)},
		{"size: in exponent form: skipped", strings.Replace(videoCp(""), `1000000`, `1e6`, 1)},
		{"size missing: skipped", `{"id":"v1","durationMs":3600000,"container":"mkv","etag":"t1","audioTracks":[]}`},
		{"etag: 64 characters", strings.Replace(videoCp(""), `"t1"`, `"`+long+`"`, 1)},
		{"etag: 65 characters: skipped", strings.Replace(videoCp(""), `"t1"`, `"`+long+`x"`, 1)},
		{"etag: empty: skipped", strings.Replace(videoCp(""), `"t1"`, `""`, 1)},
		{"etag: a character outside the form: skipped", strings.Replace(videoCp(""), `"t1"`, `"t+1"`, 1)},
		{"etag missing: skipped", `{"id":"v1","durationMs":3600000,"container":"mkv","size":1000000,"audioTracks":[]}`},
		{"audioTracks missing: skipped", `{"id":"v1","durationMs":3600000,"container":"mkv","size":1000000,"etag":"t1"}`},
		{"audioTracks not an array: skipped", `{"id":"v1","durationMs":3600000,"container":"mkv","size":1000000,"etag":"t1","audioTracks":{"a1":"aac"}}`},
		{"audioTracks null: skipped", `{"id":"v1","durationMs":3600000,"container":"mkv","size":1000000,"etag":"t1","audioTracks":null}`},
		{"not an object: skipped", `["v1"]`},

		// Optional members of a copy.
		{"name: made plain text", videoCp(`,"name":"  4K\tHDR\u200b "`)},
		{"name: 32 characters kept", videoCp(`,"name":"` + strings.Repeat("é", 32) + `"`)},
		{"name: cut to 32 characters", videoCp(`,"name":"` + strings.Repeat("ab", 16) + `cd"`)},
		{"name: empty once cleaned: absent", videoCp(`,"name":" \u200b\t"`)},
		{"name: not a string: absent", videoCp(`,"name":4`)},
		{"edition: each of the list", videoCp(`,"edition":"imax"`)},
		{"edition: unknown, kept as other", videoCp(`,"edition":"fan"`)},
		{"edition: not a string: absent", videoCp(`,"edition":true`)},
		{"startMs: equal to durationMs", videoCp(`,"startMs":3600000`)},
		{"startMs: above durationMs: absent", videoCp(`,"startMs":3600001`)},
		{"startMs: negative: absent", videoCp(`,"startMs":-1`)},
		{"bitrateKbps: 500", videoCp(`,"bitrateKbps":500`)},
		{"bitrateKbps: rounded down to a whole 500", videoCp(`,"bitrateKbps":47999`)},
		{"bitrateKbps: 10,000,000", videoCp(`,"bitrateKbps":10000000`)},
		{"bitrateKbps: 499: absent", videoCp(`,"bitrateKbps":499`)},
		{"bitrateKbps: 10,000,001: absent", videoCp(`,"bitrateKbps":10000001`)},
		{"bitrateKbps: not an integer: absent", videoCp(`,"bitrateKbps":1500.5`)},
		{"added: a valid date", videoCp(`,"added":"2024-02-29"`)},
		{"added: a day that doesn't exist: absent", videoCp(`,"added":"2023-02-29"`)},
		{"added: a month 13: absent", videoCp(`,"added":"2024-13-01"`)},
		{"added: without leading zeros: absent", videoCp(`,"added":"2024-2-1"`)},
		{"added: with a time: absent", videoCp(`,"added":"2024-02-01T00:00:00Z"`)},
		{"added: not a string: absent", videoCp(`,"added":20240201`)},
		{"threeD: tab", videoCp(`,"threeD":"tab"`)},
		{"threeD: mvc", videoCp(`,"threeD":"mvc"`)},
		{"threeD: unknown: absent", videoCp(`,"threeD":"3d"`)},
		{"previews: true", videoCp(`,"previews":true`)},
		{"previews: not a boolean: absent", videoCp(`,"previews":"true"`)},

		// parts.
		{"parts: 16 parts", videoCp(`,"parts":[` + videoQuoted(16, func(i int) string { return `{"size":62500,"durationMs":225000}` }) + `]`)},
		{"parts: 17 parts: absent", videoCp(`,"parts":[` + videoQuoted(17, func(i int) string {
			if i == 16 {
				return `{"size":58816,"durationMs":0}`
			}
			return `{"size":58824,"durationMs":225000}`
		}) + `]`)},
		{"parts: sizes that don't add up to size: absent", videoCp(`,"parts":[{"size":500000,"durationMs":1800000},{"size":499999,"durationMs":1800000}]`)},
		{"parts: a part of size 0: absent", videoCp(`,"parts":[{"size":1000000,"durationMs":1800000},{"size":0,"durationMs":1800000}]`)},
		{"parts: a durationMs out of range: absent", videoCp(`,"parts":[{"size":1000000,"durationMs":8640000001}]`)},
		{"parts: a durationMs missing: absent", videoCp(`,"parts":[{"size":1000000}]`)},
		{"parts: a part that isn't an object: absent", videoCp(`,"parts":[{"size":1000000,"durationMs":0},7]`)},
		{"parts: empty: absent", videoCp(`,"parts":[]`)},
		{"parts: not an array: absent", videoCp(`,"parts":{"size":1000000,"durationMs":0}`)},
		{"parts: other members ignored", videoCp(`,"parts":[{"size":1000000,"durationMs":3600000,"file":"a.mkv"}]`)},

		// video.
		{"video: codec, width and height only", videoVid("")},
		{"video: width and height 1", videoCp(`,"video":{"codec":"av1","width":1,"height":1}`)},
		{"video: width and height 100,000", videoCp(`,"video":{"codec":"av1","width":100000,"height":100000}`)},
		{"video: a width of 0: absent", videoCp(`,"video":{"codec":"av1","width":0,"height":1080}`)},
		{"video: a height of 100,001: absent", videoCp(`,"video":{"codec":"av1","width":1920,"height":100001}`)},
		{"video: a width that isn't an integer: absent", videoCp(`,"video":{"codec":"av1","width":1920.0,"height":1080}`)},
		{"video: no height: absent", videoCp(`,"video":{"codec":"av1","width":1920}`)},
		{"video: no codec: absent", videoCp(`,"video":{"width":1920,"height":1080}`)},
		{"video: a codec that isn't a string: absent", videoCp(`,"video":{"codec":27,"width":1920,"height":1080}`)},
		{"video: an unknown codec, kept as other", videoCp(`,"video":{"codec":"prores","width":1920,"height":1080}`)},
		{"video: a codec known for conversion only", videoCp(`,"video":{"codec":"mpeg2video","width":720,"height":576,"interlaced":true}`)},
		{"video: not an object: absent", videoCp(`,"video":"1080p"`)},
		{"video: codecTag of 8 characters", videoVid(`,"codecTag":"dvhe1234"`)},
		{"video: codecTag of 9 characters: absent alone", videoVid(`,"codecTag":"dvhe12345"`)},
		{"video: codecTag with a dash: absent alone", videoVid(`,"codecTag":"hv-1"`)},
		{"video: profile of 32 characters", videoVid(`,"profile":"` + strings.Repeat("m", 32) + `"`)},
		{"video: profile of 33 characters: absent alone", videoVid(`,"profile":"` + strings.Repeat("m", 33) + `"`)},
		{"video: level 0", videoVid(`,"level":0`)},
		{"video: level 1,000", videoVid(`,"level":1000`)},
		{"video: level 5.1, a number", videoVid(`,"level":5.1`)},
		{"video: level above 1,000: absent alone", videoVid(`,"level":1000.5`)},
		{"video: level negative: absent alone", videoVid(`,"level":-1`)},
		{"video: level as a string: absent alone", videoVid(`,"level":"5.1"`)},
		{"video: bitDepth 12", videoVid(`,"bitDepth":12`)},
		{"video: bitDepth 9: absent alone", videoVid(`,"bitDepth":9`)},
		{"video: bitDepth 16: absent alone", videoVid(`,"bitDepth":16`)},
		{"video: hdr kept in the order hdr10, hdr10plus, hlg, dv", videoVid(`,"hdr":["dv","hlg","hdr10plus","hdr10"]`)},
		{"video: hdr repeats, unknown formats and non-strings skipped", videoVid(`,"hdr":["hdr10","HDR10","hdr10+","hdr10",7,"hdr10plus"]`)},
		{"video: an empty hdr is standard range", videoVid(`,"hdr":[]`)},
		{"video: hdr of only unknown formats is standard range", videoVid(`,"hdr":["sl-hdr1"]`)},
		{"video: hdr not an array: absent alone", videoVid(`,"hdr":"hdr10"`)},
		{"video: dv kept with dv in hdr, at its largest values", videoVid(`,"hdr":["dv"],"dv":{"profile":10,"level":13,"compatibility":15}`)},
		{"video: dv at its smallest values", videoVid(`,"hdr":["hdr10","dv"],"dv":{"profile":0,"level":0,"compatibility":0}`)},
		{"video: dv without dv in hdr: absent alone", videoVid(`,"hdr":["hdr10"],"dv":{"profile":8,"level":6,"compatibility":1}`)},
		{"video: dv with a profile of 11: absent alone", videoVid(`,"hdr":["dv"],"dv":{"profile":11,"level":6,"compatibility":1}`)},
		{"video: dv with a level of 14: absent alone", videoVid(`,"hdr":["dv"],"dv":{"profile":8,"level":14,"compatibility":1}`)},
		{"video: dv with a compatibility of 16: absent alone", videoVid(`,"hdr":["dv"],"dv":{"profile":8,"level":6,"compatibility":16}`)},
		{"video: dv without its compatibility: absent alone", videoVid(`,"hdr":["dv"],"dv":{"profile":5,"level":6}`)},
		{"video: fps rounded to two decimals", videoVid(`,"fps":23.976`)},
		{"video: fps 1,000", videoVid(`,"fps":1000`)},
		{"video: fps that rounds to 1,000", videoVid(`,"fps":1000.004`)},
		{"video: fps above 1,000: absent alone", videoVid(`,"fps":1000.01`)},
		{"video: fps 0: absent alone", videoVid(`,"fps":0`)},
		{"video: fps that rounds to 0: absent alone", videoVid(`,"fps":0.004`)},
		{"video: fps 0.01", videoVid(`,"fps":0.01`)},
		{"video: fps as a string: absent alone", videoVid(`,"fps":"24"`)},
		{"video: interlaced not a boolean: absent alone", videoVid(`,"interlaced":1`)},
		{"video: colour, every member", videoVid(`,"colour":{"primaries":"bt2020","transfer":"smpte2084","matrix":"bt2020nc","range":"pc"}`)},
		{"video: colour members out of form absent alone", videoVid(`,"colour":{"primaries":"BT2020","transfer":"` + strings.Repeat("a", 33) + `","matrix":"bt2020nc","range":7}`)},
		{"video: colour member of 32 characters", videoVid(`,"colour":{"transfer":"` + strings.Repeat("a", 32) + `"}`)},
		{"video: colour with no member left: absent alone", videoVid(`,"colour":{"primaries":"","range":"TV"}`)},
		{"video: colour not an object: absent alone", videoVid(`,"colour":"bt709"`)},

		// audioTracks.
		{"audio: a0 and a999", videoCore(`{"id":"a0","codec":"aac"},{"id":"a999","codec":"opus"}`) + `}`},
		{"audio: a1000, a01, 1 and a non-object skipped", videoCore(`{"id":"a1000","codec":"aac"},{"id":"a01","codec":"aac"},{"id":"1","codec":"aac"},"a2",{"id":"a2","codec":"aac"}`) + `}`},
		{"audio: a repeated ID skipped", videoCore(`{"id":"a1","codec":"aac","language":"en"},{"id":"a1","codec":"ac3","language":"fr"}`) + `}`},
		{"audio: a track without a codec, or with one that isn't a string, skipped", videoCore(`{"id":"a1"},{"id":"a2","codec":null},{"id":"a3","codec":"flac"}`) + `}`},
		{"audio: an unknown codec, kept as other", videoCore(`{"id":"a1","codec":"dts-hd"}`) + `}`},
		{"audio: a codec known for conversion only", videoCore(`{"id":"a1","codec":"truehd"}`) + `}`},
		{"audio: the first 32 tracks kept", videoCore(videoQuoted(33, func(i int) string { return fmt.Sprintf(`{"id":"a%d","codec":"aac"}`, i) })) + `}`},
		{"audio: language of 12 characters", videoCore(`{"id":"a1","codec":"aac","language":"zh-Hant-TW1"},{"id":"a2","codec":"aac","language":"abcdefghijkl"}`) + `}`},
		{"audio: language of 13 characters or out of form: absent alone", videoCore(`{"id":"a1","codec":"aac","language":"abcdefghijklm"},{"id":"a2","codec":"aac","language":"fr_CA"},{"id":"a3","codec":"aac","language":""},{"id":"a4","codec":"aac","language":7}`) + `}`},
		{"audio: language of one character", videoCore(`{"id":"a1","codec":"aac","language":"e"}`) + `}`},
		{"audio: channels 1 and 64", videoCore(`{"id":"a1","codec":"aac","channels":1},{"id":"a2","codec":"aac","channels":64}`) + `}`},
		{"audio: channels 0 and 65: absent alone", videoCore(`{"id":"a1","codec":"aac","channels":0},{"id":"a2","codec":"aac","channels":65}`) + `}`},
		{"audio: role description; an unknown role absent alone", videoCore(`{"id":"a1","codec":"aac","role":"description"},{"id":"a2","codec":"aac","role":"director"}`) + `}`},
		{"audio: flags that aren't booleans: absent alone", videoCore(`{"id":"a1","codec":"aac","default":"true","forced":1,"original":null}`) + `}`},
		{"audio: fileIndex 0 and 999; 1,000 absent alone", videoCore(`{"id":"a1","codec":"aac","fileIndex":0},{"id":"a2","codec":"aac","fileIndex":999},{"id":"a3","codec":"aac","fileIndex":1000}`) + `}`},
		{"audio: profiles of the list; others absent alone", videoCore(`{"id":"a1","codec":"dts","profile":"dts-hd-ma"},{"id":"a2","codec":"aac","profile":"he-aac"},{"id":"a3","codec":"dts","profile":"dts-x"},{"id":"a4","codec":"eac3","profile":"Atmos"}`) + `}`},
		{"audio: layout of 32 characters; 33 or a space absent alone", videoCore(`{"id":"a1","codec":"aac","layout":"`+strings.Repeat("7", 32)+`"},{"id":"a2","codec":"aac","layout":"`+strings.Repeat("7", 33)+`"},{"id":"a3","codec":"aac","layout":"5.1 side"}`) + `}`},
		{"audio: sampleRate 8,000 and 768,000; 7,999 and 768,001 absent alone", videoCore(`{"id":"a1","codec":"pcm_s24le","sampleRate":8000},{"id":"a2","codec":"pcm_s24le","sampleRate":768000},{"id":"a3","codec":"aac","sampleRate":7999},{"id":"a4","codec":"aac","sampleRate":768001}`) + `}`},
		{"audio: bitDepth 8 and 32; 7 and 33 absent alone", videoCore(`{"id":"a1","codec":"flac","bitDepth":8},{"id":"a2","codec":"flac","bitDepth":32},{"id":"a3","codec":"flac","bitDepth":7},{"id":"a4","codec":"flac","bitDepth":33}`) + `}`},
		{"audio: title made plain text and cut to 32 characters; empty absent", videoCore(`{"id":"a1","codec":"aac","title":" Signs\n& Songs "},{"id":"a2","codec":"aac","title":"`+strings.Repeat("c", 33)+`"},{"id":"a3","codec":"aac","title":"\u0007"}`) + `}`},

		// subtitleTracks.
		{"subtitles: s0 and s999, every format of the list", videoCp(`,"subtitleTracks":[{"id":"s0","format":"subrip"},{"id":"s1","format":"ass"},{"id":"s2","format":"ssa"},{"id":"s3","format":"webvtt"},{"id":"s4","format":"mov_text"},{"id":"s5","format":"ttml"},{"id":"s6","format":"pgs"},{"id":"s7","format":"vobsub"},{"id":"s8","format":"dvb"},{"id":"s999","format":"other"}]`)},
		{"subtitles: s1000, s01, a repeat and a non-object skipped", videoCp(`,"subtitleTracks":[{"id":"s1000","format":"pgs"},{"id":"s01","format":"pgs"},{"id":"s1","format":"pgs"},{"id":"s1","format":"ass"},["s2"]]`)},
		{"subtitles: an unknown, missing or non-string format kept as other", videoCp(`,"subtitleTracks":[{"id":"s1","format":"srt"},{"id":"s2"},{"id":"s3","format":4}]`)},
		{"subtitles: members out of range or of the wrong type absent alone", videoCp(`,"subtitleTracks":[{"id":"s1","format":"pgs","language":"fr_FR","default":"yes","forced":true,"hearingImpaired":0,"external":false,"fileIndex":1000,"title":"  "}]`)},
		{"subtitles: the first 64 tracks kept", videoCp(`,"subtitleTracks":[` + videoQuoted(65, func(i int) string { return fmt.Sprintf(`{"id":"s%d","format":"subrip"}`, i) }) + `]`)},
		{"subtitles: not an array: absent", videoCp(`,"subtitleTracks":{"id":"s1","format":"pgs"}`)},

		// chapters.
		{"chapters: from 0 to durationMs", videoCp(`,"chapters":[{"startMs":0,"name":"Opening"},{"startMs":3600000,"name":"End"}]`)},
		{"chapters: a repeated or earlier start dropped", videoCp(`,"chapters":[{"startMs":1000},{"startMs":1000,"name":"again"},{"startMs":500},{"startMs":2000}]`)},
		{"chapters: a start out of range dropped", videoCp(`,"chapters":[{"startMs":-1},{"startMs":0},{"startMs":3600001},{"startMs":1.5}]`)},
		{"chapters: names made plain text and cut to 64 characters; empty absent", videoCp(`,"chapters":[{"startMs":0,"name":"` + strings.Repeat("n", 65) + `"},{"startMs":1,"name":"\u200b"},{"startMs":2,"name":3}]`)},
		{"chapters: the first 256 kept", videoCp(`,"chapters":[` + videoQuoted(257, func(i int) string { return fmt.Sprintf(`{"startMs":%d}`, i*1000) }) + `]`)},
		{"chapters: an entry that isn't an object dropped", videoCp(`,"chapters":[0,{"startMs":0}]`)},
		{"chapters: not an array: absent", videoCp(`,"chapters":{"startMs":0}`)},

		// segments.
		{"segments: every kind, end at durationMs", videoCp(`,"segments":[{"kind":"recap","startMs":0,"endMs":30000},{"kind":"intro","startMs":30000,"endMs":90000},{"kind":"commercial","startMs":1800000,"endMs":1860000},{"kind":"outro","startMs":3400000,"endMs":3600000,"final":true},{"kind":"preview","startMs":3500000,"endMs":3600000}]`)},
		{"segments: unknown kinds and times out of range or order dropped", videoCp(`,"segments":[{"kind":"credits","startMs":0,"endMs":1000},{"kind":"intro","startMs":1000,"endMs":1000},{"kind":"intro","startMs":2000,"endMs":1000},{"kind":"intro","startMs":-1,"endMs":1000},{"kind":"outro","startMs":0,"endMs":3600001},{"kind":"outro","startMs":0},{"kind":"intro","startMs":0,"endMs":1000,"final":"yes"}]`)},
		{"segments: the first 32 kept", videoCp(`,"segments":[` + videoQuoted(33, func(i int) string {
			return fmt.Sprintf(`{"kind":"commercial","startMs":%d,"endMs":%d}`, i*1000, i*1000+500)
		}) + `]`)},
		{"segments: not an array: absent", videoCp(`,"segments":"intro"`)},
	}
	var out []videoCase
	for _, c := range in {
		got, err := keepVideoCopy([]byte(c.text))
		ok, r := videoReason(err)
		out = append(out, videoCase{c.name, c.text, ok, string(got), r})
	}
	return out
}

func videoTitleCases() []videoCase {
	cs := `,"copies":[` + videoC + `]`
	film := `"key":"movie/tmdb/949","kind":"movie","library":"v1"`
	bare := `"key":"series/tmdb/1399/e63056","kind":"episode","library":"v2"` + cs
	episode := bare + `,"place":{"season":1,"episode":1}`
	series := `"key":"series/tmdb/1399","kind":"series","library":"v2"`
	season := `"key":"series/tmdb/1399/s1","kind":"season","library":"v2"`
	numbered := `"key":"series/tmdb/1399/s1/e2","kind":"episode","library":"v2"` + cs
	key := func(k, kind string) string {
		return `{"key":"` + k + `","kind":"` + kind + `","library":"v1"` + cs + `}`
	}
	copyN := func(i int) string { return strings.Replace(videoC, `"v1"`, fmt.Sprintf(`"c%d"`, i), 1) }
	var with []string
	for i := 0; i < 18; i++ {
		with = append(with, fmt.Sprintf(`"series/tmdb/1399/e%d"`, 63060+i))
	}
	in := []videoInput{
		{"a film", `{` + film + cs + `,"ids":{"imdb":"tt0113277"}}`},
		{"an episode keyed by its TMDB ID", `{` + episode + `,"with":["series/tmdb/1399/e63057"],"ids":{"imdb":"tt1480055","tvdb":"3254641"}}`},
		{"a series", `{` + series + `,"ids":{"imdb":"tt0944947","tvdb":"121361"}}`},
		{"a season", `{` + season + `}`},
		{"an episode by its numbers", `{` + numbered + `}`},
		{"a series of the tvdb scheme, with ids", `{"key":"series/tvdb/81189","kind":"series","library":"v2","ids":{"imdb":"tt0903747"}}`},
		{"an episode of the tvdb scheme, by its absolute number", `{"key":"series/tvdb/81189/a27","kind":"episode","library":"v2"` + cs + `,"ids":{"tvdb":"349232"}}`},
		{"the summary of the first version 0 (audio, subtitles, video): ignored", `{` + film + cs + `,"audio":["en"],"subtitles":["fr"],"video":{"height":2160,"hdr":["hdr10"]}}`},

		// Keys (section 4).
		{"a TMDB ID of 10 digits", key("movie/tmdb/9999999999", "movie")},
		{"a TMDB ID of 11 digits: skipped", key("movie/tmdb/10000000000", "movie")},
		{"a key with a leading zero: skipped", key("movie/tmdb/0949", "movie")},
		{"a TMDB ID of 0: skipped", key("movie/tmdb/0", "movie")},
		{"season 0 and episode 0", `{"key":"series/tmdb/1399/s0/e0","kind":"episode","library":"v1"` + cs + `}`},
		{"season 9,999 and episode 99,999", `{"key":"series/tmdb/1399/s9999/e99999","kind":"episode","library":"v1"` + cs + `}`},
		{"season 10,000: skipped", `{"key":"series/tmdb/1399/s10000","kind":"season","library":"v1"}`},
		{"episode 100,000: skipped", key("series/tmdb/1399/s1/e100000", "episode")},
		{"season s01: skipped", `{"key":"series/tmdb/1399/s01","kind":"season","library":"v1"}`},
		{"a TMDB episode ID of 10 digits", `{"key":"series/tmdb/1399/e9999999999","kind":"episode","library":"v1"` + cs + `,"place":{"season":1,"episode":1}}`},
		{"a TMDB episode ID of 11 digits: skipped", `{"key":"series/tmdb/1399/e10000000000","kind":"episode","library":"v1"` + cs + `,"place":{"season":1,"episode":1}}`},
		{"a TMDB episode ID with a leading zero: skipped", `{"key":"series/tmdb/1399/e063056","kind":"episode","library":"v1"` + cs + `,"place":{"season":1,"episode":1}}`},
		{"an absolute number under tmdb: skipped", key("series/tmdb/1399/a27", "episode")},
		{"a film with a season: skipped", key("movie/tmdb/949/s1", "movie")},
		{"a film of the tvdb scheme: skipped", key("movie/tvdb/603", "movie")},
		{"an unknown scheme: skipped", key("movie/imdb/tt0113277", "movie")},
		{"a kind in another case: skipped", key("Movie/tmdb/949", "movie")},
		{"a key ending in /: skipped", key("movie/tmdb/949/", "movie")},
		{"a music key: skipped", `{"key":"track/mb/0c000000-0000-4000-8000-000000000003","kind":"track","library":"v1"}`},
		{"a key that isn't a string: skipped", `{"key":949,"kind":"movie","library":"v1"` + cs + `}`},
		{"a kind that doesn't match the key: skipped", `{"key":"movie/tmdb/949","kind":"series","library":"v1"` + cs + `}`},
		{"a library not asked for: skipped", `{"key":"movie/tmdb/949","kind":"movie","library":"v3"` + cs + `}`},
		{"not an object: skipped", `"movie/tmdb/949"`},

		// copies (sections 3.1 and 3.8).
		{"copies: the first 8 kept", `{` + film + `,"copies":[` + videoQuoted(9, copyN) + `]}`},
		{"copies: skipped copies don't count toward the 8", `{` + film + `,"copies":[7,{"id":"bad"},` + videoQuoted(9, copyN) + `]}`},
		{"copies: a repeated id skipped alone", `{` + film + `,"copies":[` + copyN(1) + `,` + strings.Replace(copyN(1), `"t1"`, `"t2"`, 1) + `,` + copyN(2) + `]}`},
		{"copies: an entry that isn't an object skipped alone", `{` + film + `,"copies":["v0",` + videoC + `]}`},
		{"copies: a film with no copy left: skipped", `{` + film + `,"copies":[{"id":"v1","durationMs":-1,"container":"mkv","size":1,"etag":"t1","audioTracks":[]}]}`},
		{"copies: empty: skipped", `{` + film + `,"copies":[]}`},
		{"copies missing on a film: skipped", `{` + film + `}`},
		{"copies missing on an episode: skipped", `{"key":"series/tmdb/1399/s1/e2","kind":"episode","library":"v2"}`},
		{"copies not an array: skipped", `{` + film + `,"copies":` + videoC + `}`},
		{"copies on a series: ignored", `{` + series + cs + `}`},
		{"copies on a season, even not an array: ignored", `{` + season + `,"copies":"none"}`},

		// place (section 4.1).
		{"place: season 0 and episode 0 (a special, a pilot)", `{` + bare + `,"place":{"season":0,"episode":0}}`},
		{"place: the largest numbers", `{` + bare + `,"place":{"season":9999,"episode":99999}}`},
		{"place: other members ignored", `{` + bare + `,"place":{"season":2,"episode":5,"absolute":17}}`},
		{"place missing on an episode keyed by its TMDB ID: skipped", `{` + bare + `}`},
		{"place that isn't an object: skipped", `{` + bare + `,"place":[1,1]}`},
		{"place without an episode: skipped", `{` + bare + `,"place":{"season":1}}`},
		{"place with a season above 9,999: skipped", `{` + bare + `,"place":{"season":10000,"episode":1}}`},
		{"place with an episode above 99,999: skipped", `{` + bare + `,"place":{"season":1,"episode":100000}}`},
		{"place with a negative number: skipped", `{` + bare + `,"place":{"season":-1,"episode":1}}`},
		{"place with numbers as strings: skipped", `{` + bare + `,"place":{"season":"1","episode":"1"}}`},
		{"place with a number that isn't an MSFP integer: skipped", `{` + bare + `,"place":{"season":1,"episode":1.0}}`},
		{"place of an episode keyed by its numbers: ignored", `{` + numbered + `,"place":{"season":3,"episode":4}}`},
		{"place of a tvdb episode: ignored", `{"key":"series/tvdb/81189/s1/e2","kind":"episode","library":"v2"` + cs + `,"place":{"season":3,"episode":4}}`},
		{"place of a film: ignored", `{` + film + cs + `,"place":{"season":1,"episode":1}}`},

		// with (section 4.2).
		{"with: other series, itself, films, seasons, extras, bad keys, non-strings and repeats skipped, the first 16 kept", `{` + episode + `,"with":["series/tmdb/1400/e63057","series/tmdb/1399/e63056","movie/tmdb/949","series/tmdb/1399/s1","series/tmdb/1399/x/Tr1","series/tmdb/1399/e063057",7,` + strings.Join(with[:2], ",") + `,` + with[0] + `,` + strings.Join(with[2:], ",") + `]}`},
		{"with: episodes of the same series in both forms", `{` + numbered + `,"with":["series/tmdb/1399/s1/e3","series/tmdb/1399/e63058"]}`},
		{"with: a tvdb episode's, the same series only", `{"key":"series/tvdb/81189/a27","kind":"episode","library":"v2"` + cs + `,"with":["series/tvdb/81189/a28","series/tvdb/81190/a28","series/tmdb/81189/s1/e2"]}`},
		{"with: not an array: absent", `{` + episode + `,"with":"series/tmdb/1399/e63057"}`},
		{"with of a film: ignored", `{` + film + cs + `,"with":["series/tmdb/1399/e63057"]}`},

		// ids (section 4.3).
		{"ids: imdb of 7 and tvdb of 10 digits", `{` + film + cs + `,"ids":{"imdb":"tt0000001","tvdb":"9999999999"}}`},
		{"ids: imdb of 10 digits", `{` + film + cs + `,"ids":{"imdb":"tt1234567890"}}`},
		{"ids: values out of form, unknown schemes and non-strings skipped", `{` + film + cs + `,"ids":{"imdb":"tt012345","tvdb":"081189","tmdb":"949","mb":"x"}}`},
		{"ids: imdb of 11 digits and tvdb of 11 digits skipped", `{` + film + cs + `,"ids":{"imdb":"tt12345678901","tvdb":"10000000000"}}`},
		{"ids: one good, one not a string", `{` + film + cs + `,"ids":{"imdb":7,"tvdb":"81189"}}`},
		{"ids: not an object: absent", `{` + film + cs + `,"ids":["tt0113277"]}`},
		{"ids of a season: ignored", `{` + season + `,"ids":{"tvdb":"81189"}}`},

		// Members other titles may carry (sections 3.9 and 3.10).
		{"extraType, name, year and image on a film of the tmdb scheme: ignored", `{` + film + cs + `,"extraType":"trailer","name":"Heat","year":1995,"image":true}`},
	}
	return videoTitleSet(in)
}

func videoExtraCases() []videoCase {
	cs := `,"copies":[` + videoC + `]`
	x := func(k, members string) string {
		return `{"key":"` + k + `","kind":"extra","library":"v1"` + cs + members + `}`
	}
	opaque := strings.Repeat("Q", 64)
	in := []videoInput{
		{"a trailer of a film", x("movie/tmdb/348/x/Tr7aQ1", `,"extraType":"trailer","name":"Teaser"`)},
		{"a featurette of a series", x("series/tmdb/1399/x/Fe_1", `,"extraType":"featurette"`)},
		{"an extra of a season", x("series/tmdb/1399/s1/x/Bt-2", `,"extraType":"behindthescenes"`)},
		{"an extra of a tvdb series", x("series/tvdb/81189/x/In3", `,"extraType":"interview"`)},
		{"an extra of a tvdb season", x("series/tvdb/81189/s0/x/Sc4", `,"extraType":"scene"`)},
		{"an extra of a local film", x("movie/local/q3Zp0Lw8/x/Sh5", `,"extraType":"short"`)},
		{"an extra of a local season", x("series/local/Hk2v9TnA/s1/x/De6", `,"extraType":"deletedscene"`)},
		{"extraType clip", x("movie/tmdb/348/x/Cl7", `,"extraType":"clip"`)},
		{"extra part of 64 characters", x("movie/tmdb/348/x/"+opaque, `,"extraType":"trailer"`)},
		{"extraType unknown: clip", x("movie/tmdb/348/x/Tr7aQ1", `,"extraType":"themesong"`)},
		{"extraType in another case: clip", x("movie/tmdb/348/x/Tr7aQ1", `,"extraType":"Trailer"`)},
		{"extraType missing: clip", x("movie/tmdb/348/x/Tr7aQ1", ``)},
		{"extraType not a string: clip", x("movie/tmdb/348/x/Tr7aQ1", `,"extraType":1`)},
		{"name made plain text", x("movie/tmdb/348/x/Tr7aQ1", `,"extraType":"trailer","name":"\tMaking\u00a0of \u200e"`)},
		{"name of 64 characters kept", x("movie/tmdb/348/x/Tr7aQ1", `,"extraType":"trailer","name":"`+strings.Repeat("é", 64)+`"`)},
		{"name cut to 64 characters", x("movie/tmdb/348/x/Tr7aQ1", `,"extraType":"trailer","name":"`+strings.Repeat("n", 65)+`"`)},
		{"name empty once cleaned: absent", x("movie/tmdb/348/x/Tr7aQ1", `,"extraType":"trailer","name":"  "`)},
		{"name not a string: absent", x("movie/tmdb/348/x/Tr7aQ1", `,"extraType":"trailer","name":["Teaser"]`)},
		{"ids, place, with, year and image on an extra: ignored", x("movie/tmdb/348/x/Tr7aQ1", `,"extraType":"trailer","ids":{"imdb":"tt0078748"},"place":{"season":1,"episode":1},"with":["movie/tmdb/348/x/Tr8"],"year":1979,"image":true`)},
		{"an extra without copies: skipped", `{"key":"movie/tmdb/348/x/Tr7aQ1","kind":"extra","library":"v1","extraType":"trailer"}`},
		{"an extra whose copies isn't an array: skipped", `{"key":"movie/tmdb/348/x/Tr7aQ1","kind":"extra","library":"v1","extraType":"trailer","copies":{}}`},
		{"an extra of an episode: skipped", x("series/tmdb/1399/s1/e1/x/Tr1", `,"extraType":"trailer"`)},
		{"an extra of an episode keyed by its TMDB ID: skipped", x("series/tmdb/1399/e63056/x/Tr1", `,"extraType":"trailer"`)},
		{"an extra of an extra: skipped", x("movie/tmdb/348/x/Tr1/x/Tr2", `,"extraType":"trailer"`)},
		{"an extra part of 65 characters: skipped", x("movie/tmdb/348/x/"+opaque+"Q", `,"extraType":"trailer"`)},
		{"an extra part with a dot: skipped", x("movie/tmdb/348/x/Tr.1", `,"extraType":"trailer"`)},
		{"an empty extra part: skipped", x("movie/tmdb/348/x/", `,"extraType":"trailer"`)},
		{"x without an extra part: skipped", x("movie/tmdb/348/x", `,"extraType":"trailer"`)},
		{"an extra under a bad key: skipped", x("movie/tmdb/0348/x/Tr1", `,"extraType":"trailer"`)},
		{"kind trailer: skipped (the kind is extra)", `{"key":"movie/tmdb/348/x/Tr7aQ1","kind":"trailer","library":"v1"` + cs + `}`},
		{"kind movie for an extra's key: skipped", `{"key":"movie/tmdb/348/x/Tr7aQ1","kind":"movie","library":"v1"` + cs + `}`},
	}
	return videoTitleSet(in)
}

func videoLocalTitleCases() []videoCase {
	cs := `,"copies":[` + videoC + `]`
	film := `"key":"movie/local/q3Zp0Lw8","kind":"movie","library":"v1"` + cs
	series := `"key":"series/local/Hk2v9TnA","kind":"series","library":"v2"`
	in := []videoInput{
		{"a local film with its whole description", `{` + film + `,"name":"Summer 2019","year":2019,"image":true}`},
		{"a local series with a name only", `{` + series + `,"name":"Family videos"}`},
		{"a local season: no description", `{"key":"series/local/Hk2v9TnA/s1","kind":"season","library":"v2"}`},
		{"a local episode: no description, copies kept", `{"key":"series/local/Hk2v9TnA/s1/e3","kind":"episode","library":"v2"` + cs + `}`},
		{"a description on a local season or episode: ignored", `{"key":"series/local/Hk2v9TnA/s1/e3","kind":"episode","library":"v2"` + cs + `,"name":"Day 3","year":2019,"image":true}`},
		{"name made plain text", `{` + film + `,"name":"  Summer\n\n2019\u202e "}`},
		{"name of 128 characters kept", `{` + film + `,"name":"` + strings.Repeat("é", 128) + `"}`},
		{"name cut to 128 characters", `{` + film + `,"name":"` + strings.Repeat("ab", 64) + `c"}`},
		{"name empty once cleaned: skipped", `{` + film + `,"name":" \u200b "}`},
		{"name missing on a local film: skipped", `{` + film + `,"year":2019}`},
		{"name missing on a local series: skipped", `{` + series + `}`},
		{"name not a string: skipped", `{` + film + `,"name":2019}`},
		{"year 1800 and image false", `{` + film + `,"name":"Old film","year":1800,"image":false}`},
		{"year 9,999", `{` + film + `,"name":"Far future","year":9999}`},
		{"year 1,799: absent", `{` + film + `,"name":"Too old","year":1799}`},
		{"year 10,000: absent", `{` + film + `,"name":"Too far","year":10000}`},
		{"year as a string: absent", `{` + film + `,"name":"Summer","year":"2019"}`},
		{"year that isn't an MSFP integer: absent", `{` + film + `,"name":"Summer","year":2019.0}`},
		{"image that isn't a boolean: absent", `{` + film + `,"name":"Summer","image":"true"}`},
		{"ids on a local title: ignored", `{` + film + `,"name":"Summer","ids":{"imdb":"tt0113277"}}`},
		{"place on a local episode: ignored", `{"key":"series/local/Hk2v9TnA/s1/e3","kind":"episode","library":"v2"` + cs + `,"place":{"season":1,"episode":3}}`},
		{"with on a local episode: the same series only", `{"key":"series/local/Hk2v9TnA/s1/e3","kind":"episode","library":"v2"` + cs + `,"with":["series/local/Hk2v9TnA/s1/e4","series/local/Other/s1/e4"]}`},
		{"a local film without copies: skipped", `{"key":"movie/local/q3Zp0Lw8","kind":"movie","library":"v1","name":"Summer"}`},
		{"a description on a film of the tmdb scheme: ignored", `{"key":"movie/tmdb/949","kind":"movie","library":"v1"` + cs + `,"name":"Heat","year":1995,"image":true}`},
		{"a description on a series of the tvdb scheme: ignored", `{"key":"series/tvdb/81189","kind":"series","library":"v2","name":"Breaking Bad","year":2008}`},
		{"a local key out of form: skipped", `{"key":"movie/local/q3Zp.0Lw8","kind":"movie","library":"v1"` + cs + `,"name":"Summer"}`},
	}
	return videoTitleSet(in)
}

func videoLocalKeyCases() []videoInput {
	o64 := strings.Repeat("A", 64)
	return []videoInput{
		{"a local film", "movie/local/q3Zp0Lw8"},
		{"a local series", "series/local/Hk2v9TnA"},
		{"a local season", "series/local/Hk2v9TnA/s1"},
		{"a local episode", "series/local/Hk2v9TnA/s1/e3"},
		{"season 0, episode 0", "series/local/Hk2v9TnA/s0/e0"},
		{"season 9,999, episode 99,999", "series/local/Hk2v9TnA/s9999/e99999"},
		{"an identifier of one character", "movie/local/_"},
		{"an identifier of 64 characters", "movie/local/" + o64},
		{"an identifier of digits with leading zeros (opaque, not a number)", "movie/local/007"},
		{"an extra of a local film", "movie/local/q3Zp0Lw8/x/Tr1"},
		{"an extra of a local series", "series/local/Hk2v9TnA/x/Tr1"},
		{"an extra of a local season", "series/local/Hk2v9TnA/s2/x/Tr1"},
		{"an identifier of 65 characters", "movie/local/" + o64 + "A"},
		{"an empty identifier", "movie/local/"},
		{"an identifier with a dot", "movie/local/q3Zp.0Lw8"},
		{"an identifier with a space", "movie/local/q3Zp 0Lw8"},
		{"an identifier with a non-ASCII letter", "movie/local/q3Zé"},
		{"season s01", "series/local/Hk2v9TnA/s01"},
		{"season 10,000", "series/local/Hk2v9TnA/s10000"},
		{"episode 100,000", "series/local/Hk2v9TnA/s1/e100000"},
		{"an episode without a season", "series/local/Hk2v9TnA/e3"},
		{"an absolute number", "series/local/Hk2v9TnA/a3"},
		{"a film with a season", "movie/local/q3Zp0Lw8/s1"},
		{"an extra of a local episode", "series/local/Hk2v9TnA/s1/e3/x/Tr1"},
		{"the scheme in another case", "movie/LOCAL/q3Zp0Lw8"},
		{"a trailing /", "series/local/Hk2v9TnA/"},
	}
}

func videoTVDBKeyCases() []videoInput {
	return []videoInput{
		{"a series", "series/tvdb/81189"},
		{"a season", "series/tvdb/81189/s1"},
		{"season 0 (specials)", "series/tvdb/81189/s0"},
		{"an episode by its numbers", "series/tvdb/81189/s1/e2"},
		{"episode 0", "series/tvdb/81189/s1/e0"},
		{"season 9,999, episode 99,999", "series/tvdb/81189/s9999/e99999"},
		{"an episode by its absolute number", "series/tvdb/81189/a27"},
		{"absolute number 0", "series/tvdb/81189/a0"},
		{"absolute number 99,999", "series/tvdb/81189/a99999"},
		{"a TheTVDB ID of 1", "series/tvdb/1"},
		{"a TheTVDB ID of 10 digits", "series/tvdb/9999999999"},
		{"an extra of a series", "series/tvdb/81189/x/Tr1"},
		{"an extra of a season", "series/tvdb/81189/s1/x/Tr1"},
		{"absolute number 100,000", "series/tvdb/81189/a100000"},
		{"absolute number with a leading zero", "series/tvdb/81189/a027"},
		{"an absolute number under a season", "series/tvdb/81189/s1/a2"},
		{"an empty absolute number", "series/tvdb/81189/a"},
		{"a TheTVDB ID of 0", "series/tvdb/0"},
		{"a TheTVDB ID with a leading zero", "series/tvdb/081189"},
		{"a TheTVDB ID of 11 digits", "series/tvdb/10000000000"},
		{"a TheTVDB ID that isn't a number", "series/tvdb/breaking-bad"},
		{"season s01", "series/tvdb/81189/s01"},
		{"season 10,000", "series/tvdb/81189/s10000"},
		{"episode 100,000", "series/tvdb/81189/s1/e100000"},
		{"an episode keyed by an episode ID (tmdb only)", "series/tvdb/81189/e349232"},
		{"a film of the tvdb scheme", "movie/tvdb/603"},
		{"an extra of an episode", "series/tvdb/81189/a27/x/Tr1"},
		{"the scheme in another case", "series/TVDB/81189"},
	}
}

func videoPreviewCases() []videoPreviewsCase {
	root := videoStreamRoot
	tk := "Tk7Qw2xLp9mZ"
	sheets := func(n int) string {
		return `[` + videoQuoted(n, func(i int) string { return fmt.Sprintf(`"%s%s/trickplay/%d"`, root, tk, i) }) + `]`
	}
	tp := func(w, h, c, r, iv, count int, sh string) string {
		return fmt.Sprintf(`{"trickplay":{"tileWidth":%d,"tileHeight":%d,"columns":%d,"rows":%d,"intervalMs":%d,"tileCount":%d,"sheets":%s}}`, w, h, c, r, iv, count, sh)
	}
	good := tp(320, 180, 10, 10, 10000, 250, sheets(3))
	requests := []videoInput{
		{"a film's copy", `{"title":"movie/tmdb/949","versionId":"v_2HqXo3"}`},
		{"an episode's copy", `{"title":"series/tmdb/1399/e63056","versionId":"v1"}`},
		{"an extra's copy", `{"title":"movie/tmdb/348/x/Tr7aQ1","versionId":"v1"}`},
		{"a tvdb episode's copy", `{"title":"series/tvdb/81189/a27","versionId":"v1"}`},
		{"a versionId of 64 characters", `{"title":"movie/local/q3Zp0Lw8","versionId":"` + strings.Repeat("v", 64) + `"}`},
		{"unknown members ignored", `{"title":"movie/tmdb/949","versionId":"v1","etag":"t1"}`},
		{"a series has no copies: 400", `{"title":"series/tmdb/1399","versionId":"v1"}`},
		{"a season has no copies: 400", `{"title":"series/tmdb/1399/s1","versionId":"v1"}`},
		{"a title that isn't a video key: 400", `{"title":"album/mb/0c000000-0000-4000-8000-000000000003","versionId":"v1"}`},
		{"a title out of canonical form: 400", `{"title":"movie/tmdb/0949","versionId":"v1"}`},
		{"title missing: 400", `{"versionId":"v1"}`},
		{"title not a string: 400", `{"title":949,"versionId":"v1"}`},
		{"versionId missing: 400", `{"title":"movie/tmdb/949"}`},
		{"versionId of 65 characters: 400", `{"title":"movie/tmdb/949","versionId":"` + strings.Repeat("v", 65) + `"}`},
		{"versionId empty: 400", `{"title":"movie/tmdb/949","versionId":""}`},
		{"versionId with a dot: 400", `{"title":"movie/tmdb/949","versionId":"v.1"}`},
		{"versionId not a string: 400", `{"title":"movie/tmdb/949","versionId":1}`},
		{"not an object: 400", `["movie/tmdb/949","v1"]`},
	}
	answers := []videoInput{
		{"previews of 250 tiles on 3 sheets", good},
		{"one tile on one sheet", tp(1, 1, 1, 1, 100, 1, sheets(1))},
		{"the largest tiles: 2,048 pixels, 2 by 2", tp(2048, 2048, 2, 2, 3600000, 4, sheets(1))},
		{"a full sheet of 4,096 pixels each way", tp(256, 256, 16, 16, 10000, 256, sheets(1))},
		{"100 columns and 100 rows", tp(40, 40, 100, 100, 1000, 10000, sheets(1))},
		{"1,000 sheets", tp(320, 180, 1, 1, 1000, 1000, sheets(1000))},
		{"unknown members ignored", strings.Replace(good, `"tileCount"`, `"width":320,"tileCount"`, 1)},
		{"no trickplay: no previews", `{}`},
		{"tileWidth 2,049: previews dropped", tp(2049, 180, 1, 1, 10000, 1, sheets(1))},
		{"tileHeight 0: previews dropped", tp(320, 0, 1, 1, 10000, 1, sheets(1))},
		{"columns 101: previews dropped", tp(40, 40, 101, 1, 10000, 101, sheets(1))},
		{"rows 0: previews dropped", tp(40, 40, 1, 0, 10000, 1, sheets(1))},
		{"intervalMs 99: previews dropped", tp(320, 180, 10, 10, 99, 250, sheets(3))},
		{"intervalMs 3,600,001: previews dropped", tp(320, 180, 10, 10, 3600001, 250, sheets(3))},
		{"tileCount 0: previews dropped", tp(320, 180, 10, 10, 10000, 0, sheets(1))},
		{"a sheet wider than 4,096 pixels: previews dropped", tp(241, 180, 17, 1, 10000, 17, sheets(1))},
		{"a sheet taller than 4,096 pixels: previews dropped", tp(320, 410, 1, 10, 10000, 10, sheets(1))},
		{"a sheet too few: previews dropped", tp(320, 180, 10, 10, 10000, 250, sheets(2))},
		{"a sheet too many: previews dropped", tp(320, 180, 10, 10, 10000, 250, sheets(4))},
		{"1,001 sheets: previews dropped", tp(320, 180, 1, 1, 1000, 1001, sheets(1001))},
		{"a number missing: previews dropped", `{"trickplay":{"tileWidth":320,"tileHeight":180,"columns":10,"rows":10,"tileCount":250,"sheets":` + sheets(3) + `}}`},
		{"sheets missing: previews dropped", `{"trickplay":{"tileWidth":320,"tileHeight":180,"columns":10,"rows":10,"intervalMs":10000,"tileCount":250}}`},
		{"sheets out of order: refused", tp(320, 180, 10, 10, 10000, 250, `["`+root+tk+`/trickplay/1","`+root+tk+`/trickplay/0","`+root+tk+`/trickplay/2"]`)},
		{"a sheet under another stream root: refused", tp(320, 180, 10, 10, 10000, 250, `["`+root+tk+`/trickplay/0","https://b.example.org/api/v1/stream/`+tk+`/trickplay/1","`+root+tk+`/trickplay/2"]`)},
		{"a sheet over http: refused", tp(320, 180, 10, 10, 10000, 1, `["http://a.example.org/api/v1/stream/`+tk+`/trickplay/0"]`)},
		{"a sheet under another ticket: refused", tp(320, 180, 10, 10, 10000, 250, `["`+root+tk+`/trickplay/0","`+root+`Other1/trickplay/1","`+root+tk+`/trickplay/2"]`)},
		{"a ticket with a dot: refused", tp(320, 180, 10, 10, 10000, 1, `["`+root+`Tk.7/trickplay/0"]`)},
		{"an index with a leading zero: refused", tp(320, 180, 10, 10, 10000, 1, `["`+root+tk+`/trickplay/00"]`)},
		{"a sheet URL with a query: refused", tp(320, 180, 10, 10, 10000, 1, `["`+root+tk+`/trickplay/0?w=320"]`)},
		{"another file shape: refused", tp(320, 180, 10, 10, 10000, 1, `["`+root+tk+`/fonts/0"]`)},
		{"a sheet that isn't a string: refused", tp(320, 180, 10, 10, 10000, 1, `[0]`)},
		{"sheets not an array: refused", tp(320, 180, 10, 10, 10000, 1, `"`+root+tk+`/trickplay/0"`)},
		{"a number as a string: refused", strings.Replace(good, `"tileWidth":320`, `"tileWidth":"320"`, 1)},
		{"a number that isn't an integer: refused", strings.Replace(good, `"tileCount":250`, `"tileCount":250.0`, 1)},
		{"a null number: refused", strings.Replace(good, `"rows":10`, `"rows":null`, 1)},
		{"out of bounds and a bad sheet URL: refused", tp(4000, 180, 10, 10, 10000, 250, `["`+root+tk+`/trickplay/1"]`)},
		{"trickplay not an object: refused", `{"trickplay":[320,180]}`},
		{"not an object: refused", `[]`},
	}
	var out []videoPreviewsCase
	for _, c := range requests {
		ok, r := videoReason(checkVideoPreviewsRequest([]byte(c.text)))
		out = append(out, videoPreviewsCase{c.name, "request", c.text, ok, "", r})
	}
	for _, c := range answers {
		got, err := keepVideoPreviewsAnswer([]byte(c.text), root)
		ok, r := videoReason(err)
		out = append(out, videoPreviewsCase{c.name, "answer", c.text, ok, string(got), r})
	}
	return out
}

func genV0Video() []byte {
	return encode(videoFile{
		About: []string{
			"MSFP video part version 0, as revised by proposal 16 (spec/v0/video.md): what a caller keeps of a video title (section 3.1), of its copies (sections 3.2 to 3.8), of an extra (section 3.9) and of a local title (section 3.10); title keys of the tvdb and local schemes (sections 4.4 and 4.5); the previews call (sections 5.5 and 5.6). Generated by tools/vectors, which re-derives every output from its text; no implementation has produced them yet.",
			"titles, extras, localTitles: text is one entry of a titles answer, as JSON; the caller asked for the libraries listed in libraries. valid false: the caller skips the title, for the reason given. valid true: output is what it keeps, members in this order: key, kind, library, extraType, name, year, image, place (season, episode), with, ids (its members sorted), copies (each as in copies); absent members left out.",
			"copies: text is one entry of a title's copies. valid false: the caller skips that copy alone. valid true: output is what it keeps, members in the order of the table of section 3.2 (id, name, edition, durationMs, startMs, container, size, etag, parts, bitrateKbps, video, audioTracks, subtitleTracks, chapters, segments, previews, added, threeD), streams and tracks in the order of the tables of sections 3.3 and 3.4; absent members left out, audioTracks always written. Numbers that aren't integers (level, fps) are written in their shortest decimal form. The cap of 8 copies, repeated copy IDs and a title left without copies are in titles.",
			"localKeys, tvdbKeys: text is a title key. valid true: kind is the kind it names (movie, series, season, episode or extra); valid false: the receiver skips a title with that key.",
			"previews: side request is a previews request as the owning server reads it: valid false means it answers 400 (the 404s depend on what it offers, not on the request). side answer is a previews answer as the viewer's server reads it, the owning server's stream root being streamRoot: valid false means it refuses the whole answer; valid true: output is what it keeps, {} when it drops the previews and plays on without them.",
			"Tracks' languages are checked as section 3.8 says (at most 12 characters of A-Z a-z 0-9 -), not converted; the conversions of catalog section 3 are in strict.json (languageCodes). The WebVTT, ASS, PGS and font cases of video section 6 are in media.json.",
		},
		Libraries:   videoLibraries,
		StreamRoot:  videoStreamRoot,
		Titles:      videoTitleCases(),
		Copies:      videoCopyCases(),
		Extras:      videoExtraCases(),
		LocalTitles: videoLocalTitleCases(),
		LocalKeys:   videoKeySet(videoLocalKeyCases()),
		TVDBKeys:    videoKeySet(videoTVDBKeyCases()),
		Previews:    videoPreviewCases(),
	})
}
