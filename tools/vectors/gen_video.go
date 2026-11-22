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
	AnswerCopy  string              `json:"answerCopy"`
	Answers     []videoCase         `json:"answers"`
	Codecs      []videoCase         `json:"codecs"`
	FontSets    []videoCase         `json:"fontSets"`
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
	return `{"id":"v1","durationMs":3600000,"mediaType":"video/matroska","size":1000000,"etag":"t1","audioTracks":[` + audio + `]`
}

// videoCp is a copy with the required core and members (",..." or "").
func videoCp(members string) string { return videoCore("") + members + `}` }

// videoVid is a copy whose video has codec, width, height and members.
func videoVid(members string) string {
	return videoCp(`,"video":{"codec":"hevc","codecs":"hvc1.1.2.L123.B0","width":1920,"height":1080` + members + `}`)
}

func videoQuoted(n int, f func(i int) string) string {
	var l []string
	for i := 0; i < n; i++ {
		l = append(l, f(i))
	}
	return strings.Join(l, ",")
}

// A copy every title case uses.
var videoC = `{"id":"v1","durationMs":8160000,"mediaType":"video/matroska","size":1000,"etag":"t1","audioTracks":[{"id":"a1","codec":"aac","codecs":"mp4a.40.2","language":"en"}]}`

func videoCopyCases() []videoCase {
	long := strings.Repeat("x", 64)
	example := `{"id":"v_2HqXo3","edition":"directors","durationMs":8160043,"mediaType":"video/matroska","size":48213884928,"etag":"k9Tq2w","video":{"codec":"hevc","codecs":"hev1.2.4.L153.B0","width":3840,"height":1600,"bitDepth":10,"colour":{"primaries":9,"transfer":16,"matrix":9,"fullRange":false},"dv":{"profile":7,"level":6,"compatibility":6,"el":true},"fps":"24000/1001"},"audioTracks":[{"id":"a1","language":"en","channels":8,"codec":"truehd","objects":"dolby-atmos","default":true,"original":true}],"subtitleTracks":[{"id":"s2","language":"fr","format":"hdmv_pgs_subtitle"}],"chapters":[{"startMs":0},{"startMs":312000,"name":"The Desert"}],"previewLayout":{"tileWidth":320,"tileHeight":133,"columns":10,"rows":10,"intervalMs":10000,"tileCount":817}}`
	layout := func(w, h, c, r, iv, n int) string {
		return videoCp(fmt.Sprintf(`,"previewLayout":{"tileWidth":%d,"tileHeight":%d,"columns":%d,"rows":%d,"intervalMs":%d,"tileCount":%d}`, w, h, c, r, iv, n))
	}
	mt := func(t string) string { return strings.Replace(videoCp(""), `"video/matroska"`, t, 1) }
	in := []videoInput{
		{"the example of section 3.2", example},
		{"the required core only, no audio track", videoCp("")},
		{"every member", `{"id":"v2","edition":"extended","durationMs":7200001,"startMs":0,"endMs":3600000,"mediaType":"video/mp4","size":3000,"etag":"t2","parts":2,"video":{"codec":"h264","codecs":"avc1.640028","width":1920,"height":1080,"sampleAspect":"1:1","rotation":0,"bitDepth":8,"colour":{"primaries":1,"transfer":1,"matrix":1,"fullRange":false},"hdr":["hdr10plus"],"dv":{"profile":9,"level":4,"compatibility":2,"el":false},"fps":"24/1","refFrames":4,"interlaced":false},"audioTracks":[{"id":"a1","language":"fr","channels":6,"role":"commentary","default":false,"forced":false,"codec":"eac3","codecs":"ec-3","objects":"dolby-atmos","sampleRate":48000,"bitDepth":16,"original":false}],"subtitleTracks":[{"id":"s2","language":"en","role":"subtitle","default":true,"forced":false,"hearingImpaired":true,"format":"subrip","external":true}],"chapters":[{"startMs":0,"name":"Opening"}],"segments":[{"kind":"intro","startMs":1000,"endMs":60000},{"kind":"outro","startMs":7000000,"endMs":7200000}],"previewLayout":{"tileWidth":320,"tileHeight":180,"columns":10,"rows":10,"intervalMs":10000,"tileCount":721},"threeD":"sbs"}`},
		{"unknown members of a copy, a stream and a track ignored", `{"id":"v1","durationMs":3600000,"mediaType":"video/matroska","size":1000000,"etag":"t1","path":"/media/a.mkv","video":{"codec":"hevc","width":1920,"height":1080,"label":"x"},"audioTracks":[{"id":"a1","codec":"aac","label":"English"}]}`},
		{"the members earlier drafts sent: ignored", `{"id":"v1","name":"Movie.2019.2160p.mkv","durationMs":3600000,"container":"matroska","mediaType":"video/matroska","size":1000000,"etag":"t1","bitrateKbps":2222,"added":"2024-02-29","previews":true,"video":{"codec":"hevc","codecTag":"hvc1","profile":"Main 10","level":5.1,"width":1920,"height":1080},"audioTracks":[{"id":"a1","codec":"truehd","profile":"Dolby TrueHD + Dolby Atmos","layout":"7.1","title":"English Atmos","fileIndex":1}],"subtitleTracks":[{"id":"s2","format":"subrip","title":"SDH","fileIndex":2}],"segments":[{"kind":"outro","startMs":0,"endMs":1000,"final":true}]}`},

		// The required core.
		{"id: 64 characters", strings.Replace(videoCp(""), `"v1"`, `"`+long+`"`, 1)},
		{"id: _ and -", strings.Replace(videoCp(""), `"v1"`, `"_-"`, 1)},
		{"id: 65 characters: skipped", strings.Replace(videoCp(""), `"v1"`, `"`+long+`x"`, 1)},
		{"id: empty: skipped", strings.Replace(videoCp(""), `"v1"`, `""`, 1)},
		{"id: a dot: skipped", strings.Replace(videoCp(""), `"v1"`, `"v.1"`, 1)},
		{"id: not a string: skipped", strings.Replace(videoCp(""), `"v1"`, `1`, 1)},
		{"id missing: skipped", `{"durationMs":3600000,"mediaType":"video/matroska","size":1000000,"etag":"t1","audioTracks":[]}`},
		{"durationMs: exact, not rounded to the second", strings.Replace(videoCp(""), `3600000`, `3600999`, 1)},
		{"durationMs: 0", strings.Replace(videoCp(""), `3600000`, `0`, 1)},
		{"durationMs: 8,640,000,000 (100 days)", strings.Replace(videoCp(""), `3600000`, `8640000000`, 1)},
		{"durationMs: 8,640,000,001: skipped", strings.Replace(videoCp(""), `3600000`, `8640000001`, 1)},
		{"durationMs: negative: skipped", strings.Replace(videoCp(""), `3600000`, `-1`, 1)},
		{"durationMs: 3600000.0 isn't an integer: skipped", strings.Replace(videoCp(""), `3600000`, `3600000.0`, 1)},
		{"durationMs: a string: skipped", strings.Replace(videoCp(""), `3600000`, `"3600000"`, 1)},
		{"durationMs missing: skipped", `{"id":"v1","mediaType":"video/matroska","size":1000000,"etag":"t1","audioTracks":[]}`},
		{"mediaType: video/mp4", mt(`"video/mp4"`)},
		{"mediaType: the registry's video/webm", mt(`"video/webm"`)},
		{"mediaType: M2TS, video/vnd.dlna.mpeg-tts", mt(`"video/vnd.dlna.mpeg-tts"`)},
		{"mediaType: audio/mp4 for a copy without video, kept", mt(`"audio/mp4"`)},
		{"mediaType: a type nobody listed, kept as it is", mt(`"video/x-ms-asf"`)},
		{"mediaType: with + and _ and a digit first, kept", mt(`"3d/x+y_z"`)},
		{"mediaType: 127 characters, kept", mt(`"video/` + strings.Repeat("m", 121) + `"`)},
		{"mediaType: 128 characters: skipped", mt(`"video/` + strings.Repeat("m", 122) + `"`)},
		{"mediaType: capitals: skipped", mt(`"video/MP2T"`)},
		{"mediaType: with a parameter: skipped", mt(`"video/mp4; codecs=avc1"`)},
		{"mediaType: no /: skipped", mt(`"matroska"`)},
		{"mediaType: two /: skipped", mt(`"video/mp4/x"`)},
		{"mediaType: an empty half: skipped", mt(`"video/"`)},
		{"mediaType: a half starting with a dot: skipped", mt(`"video/.mp4"`)},
		{"mediaType: a half starting with +: skipped", mt(`"+video/mp4"`)},
		{"mediaType: not a string: skipped", mt(`7`)},
		{"mediaType missing, even with a container: skipped", `{"id":"v1","durationMs":3600000,"container":"matroska","size":1000000,"etag":"t1","audioTracks":[]}`},
		{"size: 1", strings.Replace(videoCp(""), `1000000`, `1`, 1)},
		{"size: 999,999,999,999,999", strings.Replace(videoCp(""), `1000000`, `999999999999999`, 1)},
		{"size: 0: skipped", strings.Replace(videoCp(""), `1000000`, `0`, 1)},
		{"size: 1,000,000,000,000,000: skipped", strings.Replace(videoCp(""), `1000000`, `1000000000000000`, 1)},
		{"size: in exponent form: skipped", strings.Replace(videoCp(""), `1000000`, `1e6`, 1)},
		{"size missing: skipped", `{"id":"v1","durationMs":3600000,"mediaType":"video/matroska","etag":"t1","audioTracks":[]}`},
		{"etag: 64 characters", strings.Replace(videoCp(""), `"t1"`, `"`+long+`"`, 1)},
		{"etag: 65 characters: skipped", strings.Replace(videoCp(""), `"t1"`, `"`+long+`x"`, 1)},
		{"etag: empty: skipped", strings.Replace(videoCp(""), `"t1"`, `""`, 1)},
		{"etag: a character outside the form: skipped", strings.Replace(videoCp(""), `"t1"`, `"t+1"`, 1)},
		{"etag missing: skipped", `{"id":"v1","durationMs":3600000,"mediaType":"video/matroska","size":1000000,"audioTracks":[]}`},
		{"audioTracks missing: skipped", `{"id":"v1","durationMs":3600000,"mediaType":"video/matroska","size":1000000,"etag":"t1"}`},
		{"audioTracks not an array: skipped", `{"id":"v1","durationMs":3600000,"mediaType":"video/matroska","size":1000000,"etag":"t1","audioTracks":{"a1":"aac"}}`},
		{"audioTracks null: skipped", `{"id":"v1","durationMs":3600000,"mediaType":"video/matroska","size":1000000,"etag":"t1","audioTracks":null}`},
		{"not an object: skipped", `["v1"]`},

		// Optional members of a copy.
		{"edition: one of the registry's", videoCp(`,"edition":"imax"`)},
		{"edition: a value not among the examples, kept", videoCp(`,"edition":"fan"`)},
		{"edition: capitals aren't a token: absent", videoCp(`,"edition":"Directors"`)},
		{"edition: not a string: absent", videoCp(`,"edition":true`)},
		{"startMs: equal to durationMs", videoCp(`,"startMs":3600000`)},
		{"startMs: above durationMs: absent", videoCp(`,"startMs":3600001`)},
		{"startMs: negative: absent", videoCp(`,"startMs":-1`)},
		{"endMs: with startMs, the episode's span", videoCp(`,"startMs":1200000,"endMs":2400000`)},
		{"endMs: alone, above 0", videoCp(`,"endMs":1`)},
		{"endMs: at durationMs", videoCp(`,"startMs":1200000,"endMs":3600000`)},
		{"endMs: equal to startMs: absent", videoCp(`,"startMs":1200000,"endMs":1200000`)},
		{"endMs: below startMs: absent", videoCp(`,"startMs":1200000,"endMs":600000`)},
		{"endMs: 0 without startMs: absent", videoCp(`,"endMs":0`)},
		{"endMs: above durationMs: absent", videoCp(`,"endMs":3600001`)},
		{"endMs: with a startMs out of range, above 0 is enough", videoCp(`,"startMs":3600001,"endMs":10`)},
		{"endMs: not an integer: absent", videoCp(`,"endMs":"2400000"`)},
		{"parts: 2", videoCp(`,"parts":2`)},
		{"parts: 16", videoCp(`,"parts":16`)},
		{"parts: 1 (one file is no parts): absent", videoCp(`,"parts":1`)},
		{"parts: 17: absent", videoCp(`,"parts":17`)},
		{"parts: the old array of sizes: absent", videoCp(`,"parts":[{"size":500000,"durationMs":1800000},{"size":500000,"durationMs":1800000}]`)},
		{"parts: 2.0 isn't an integer: absent", videoCp(`,"parts":2.0`)},
		{"threeD: tab-rl", videoCp(`,"threeD":"tab-rl"`)},
		{"threeD: sbs-rl", videoCp(`,"threeD":"sbs-rl"`)},
		{"threeD: mvc", videoCp(`,"threeD":"mvc"`)},
		{"threeD: a token nobody listed, kept as it is", videoCp(`,"threeD":"anaglyph"`)},
		{"threeD: capitals aren't a token: absent", videoCp(`,"threeD":"SBS"`)},
		{"threeD: not a string: absent", videoCp(`,"threeD":true`)},

		// previewLayout (section 5.5).
		{"previewLayout: 250 tiles on 3 sheets", layout(320, 180, 10, 10, 10000, 250)},
		{"previewLayout: one tile", layout(1, 1, 1, 1, 100, 1)},
		{"previewLayout: the largest tiles, 2,048 pixels, 2 by 2", layout(2048, 2048, 2, 2, 3600000, 4)},
		{"previewLayout: a full sheet of 4,096 pixels each way", layout(256, 256, 16, 16, 10000, 256)},
		{"previewLayout: 1,000 sheets of 1 tile", layout(320, 180, 1, 1, 1000, 1000)},
		{"previewLayout: 1,000 × 100 × 100 tiles", layout(40, 40, 100, 100, 100, 10000000)},
		{"previewLayout: 1,000 × columns × rows + 1 tiles: absent", layout(320, 180, 10, 10, 10000, 100001)},
		{"previewLayout: tileWidth 2,049: absent", layout(2049, 180, 1, 1, 10000, 1)},
		{"previewLayout: tileHeight 0: absent", layout(320, 0, 1, 1, 10000, 1)},
		{"previewLayout: columns 101: absent", layout(40, 40, 101, 1, 10000, 101)},
		{"previewLayout: rows 0: absent", layout(40, 40, 1, 0, 10000, 1)},
		{"previewLayout: intervalMs 99: absent", layout(320, 180, 10, 10, 99, 250)},
		{"previewLayout: intervalMs 3,600,001: absent", layout(320, 180, 10, 10, 3600001, 250)},
		{"previewLayout: tileCount 0: absent", layout(320, 180, 10, 10, 10000, 0)},
		{"previewLayout: a sheet wider than 4,096 pixels: absent", layout(241, 180, 17, 1, 10000, 17)},
		{"previewLayout: a sheet taller than 4,096 pixels: absent", layout(320, 410, 1, 10, 10000, 10)},
		{"previewLayout: a member missing: absent", videoCp(`,"previewLayout":{"tileWidth":320,"tileHeight":180,"columns":10,"rows":10,"tileCount":250}`)},
		{"previewLayout: a number as a string: absent", videoCp(`,"previewLayout":{"tileWidth":"320","tileHeight":180,"columns":10,"rows":10,"intervalMs":10000,"tileCount":250}`)},
		{"previewLayout: sheet URLs and other members ignored", videoCp(`,"previewLayout":{"tileWidth":320,"tileHeight":180,"columns":10,"rows":10,"intervalMs":10000,"tileCount":250,"sheets":["https://a.example.org/x"]}`)},
		{"previewLayout: not an object: absent", videoCp(`,"previewLayout":true`)},

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
		{"video: a registry token, vvc, kept", videoCp(`,"video":{"codec":"vvc","width":3840,"height":2160}`)},
		{"video: a token nobody registered yet, kept as it is", videoCp(`,"video":{"codec":"h267","width":3840,"height":2160}`)},
		{"video: mpeg2video, interlaced", videoCp(`,"video":{"codec":"mpeg2video","width":720,"height":576,"interlaced":true}`)},
		{"video: a codec in capitals isn't a token: absent", videoCp(`,"video":{"codec":"HEVC","width":1920,"height":1080}`)},
		{"video: a codec of 33 characters: absent", videoCp(`,"video":{"codec":"` + strings.Repeat("c", 33) + `","width":1920,"height":1080}`)},
		{"video: not an object: absent", videoCp(`,"video":"1080p"`)},
		{"video: h264 without codecs: kept without it", videoCp(`,"video":{"codec":"h264","width":1920,"height":1080}`)},
		{"video: codecs, an RFC 6381 string", videoVid(`,"codecs":"hvc1.2.4.L153.B0"`)},
		{"video: codecs of 64 characters", videoVid(`,"codecs":"` + strings.Repeat("a", 64) + `"`)},
		{"video: codecs of 65 characters: absent alone", videoVid(`,"codecs":"` + strings.Repeat("a", 65) + `"`)},
		{"video: codecs with a comma: absent alone", videoVid(`,"codecs":"hvc1.2.4.L153.B0,mp4a.40.2"`)},
		{"video: codecs with a + (dts+): absent alone", videoVid(`,"codecs":"hvc1+2"`)},
		{"video: codecs empty: absent alone", videoVid(`,"codecs":""`)},
		{"video: bitDepth 16", videoVid(`,"bitDepth":16`)},
		{"video: bitDepth 0: absent alone", videoVid(`,"bitDepth":0`)},
		{"video: bitDepth 17: absent alone", videoVid(`,"bitDepth":17`)},
		{"video: sampleAspect 32:27", videoVid(`,"sampleAspect":"32:27"`)},
		{"video: sampleAspect 65535:1", videoVid(`,"sampleAspect":"65535:1"`)},
		{"video: sampleAspect 65536:1: absent alone", videoVid(`,"sampleAspect":"65536:1"`)},
		{"video: sampleAspect 0:1: absent alone", videoVid(`,"sampleAspect":"0:1"`)},
		{"video: sampleAspect with a slash: absent alone", videoVid(`,"sampleAspect":"32/27"`)},
		{"video: rotation 90", videoVid(`,"rotation":90`)},
		{"video: rotation 270", videoVid(`,"rotation":270`)},
		{"video: rotation 45: absent alone", videoVid(`,"rotation":45`)},
		{"video: rotation -90: absent alone", videoVid(`,"rotation":-90`)},
		{"video: rotation 360: absent alone", videoVid(`,"rotation":360`)},
		{"video: rotation as a string: absent alone", videoVid(`,"rotation":"90"`)},
		{"video: fps 24000/1001", videoVid(`,"fps":"24000/1001"`)},
		{"video: fps 1000000/1", videoVid(`,"fps":"1000000/1"`)},
		{"video: fps 1/1000000", videoVid(`,"fps":"1/1000000"`)},
		{"video: fps not reduced: absent alone", videoVid(`,"fps":"48000/2002"`)},
		{"video: fps 0/0: absent alone", videoVid(`,"fps":"0/0"`)},
		{"video: fps 1000001/1: absent alone", videoVid(`,"fps":"1000001/1"`)},
		{"video: fps with a leading zero: absent alone", videoVid(`,"fps":"024/1"`)},
		{"video: fps as a decimal string: absent alone", videoVid(`,"fps":"23.976"`)},
		{"video: fps as a number: absent alone", videoVid(`,"fps":23.976`)},
		{"video: refFrames 16 on h264", videoCp(`,"video":{"codec":"h264","codecs":"avc1.64001F","width":1280,"height":720,"refFrames":16}`)},
		{"video: refFrames 0: absent alone", videoCp(`,"video":{"codec":"h264","codecs":"avc1.64001F","width":1280,"height":720,"refFrames":0}`)},
		{"video: refFrames 17: absent alone", videoCp(`,"video":{"codec":"h264","codecs":"avc1.64001F","width":1280,"height":720,"refFrames":17}`)},
		{"video: refFrames on hevc: absent alone", videoVid(`,"refFrames":4`)},
		{"video: refFrames as a string: absent alone", videoCp(`,"video":{"codec":"h264","codecs":"avc1.64001F","width":1280,"height":720,"refFrames":"4"}`)},
		{"video: interlaced not a boolean: absent alone", videoVid(`,"interlaced":1`)},

		// colour (section 3.6).
		{"colour: HDR10 told by colour alone, no hdr", videoVid(`,"bitDepth":10,"colour":{"primaries":9,"transfer":16,"matrix":9}`)},
		{"colour: HLG, transfer 18", videoVid(`,"colour":{"transfer":18}`)},
		{"colour: BT.2020 at standard range, transfer 14", videoVid(`,"colour":{"primaries":9,"transfer":14,"matrix":9}`)},
		{"colour: 0 and 255 kept", videoVid(`,"colour":{"primaries":255,"transfer":0,"matrix":0}`)},
		{"colour: fullRange true", videoVid(`,"colour":{"matrix":1,"fullRange":true}`)},
		{"colour: fullRange false alone", videoVid(`,"colour":{"fullRange":false}`)},
		{"colour: 2 (unspecified) counts as absent", videoVid(`,"colour":{"primaries":2,"transfer":1,"matrix":2}`)},
		{"colour: 256 and -1: absent alone", videoVid(`,"colour":{"primaries":256,"transfer":-1,"matrix":1}`)},
		{"colour: ffmpeg's names aren't numbers: absent alone", videoVid(`,"colour":{"primaries":"bt2020","transfer":"smpte2084","matrix":9}`)},
		{"colour: 9.0 isn't an integer: absent alone", videoVid(`,"colour":{"primaries":9.0,"matrix":9}`)},
		{"colour: fullRange not a boolean, and range tv: absent alone", videoVid(`,"colour":{"primaries":1,"fullRange":"true","range":"tv"}`)},
		{"colour: no member left: absent", videoVid(`,"colour":{"primaries":2,"range":"pc"}`)},
		{"colour: not an object: absent", videoVid(`,"colour":"bt709"`)},

		// hdr and dv (section 3.6).
		{"hdr: hdr10plus", videoVid(`,"hdr":["hdr10plus"]`)},
		{"hdr: sorted, each once", videoVid(`,"hdr":["sl-hdr1","hdr10plus","hdr10plus"]`)},
		{"hdr: tokens nobody registered kept", videoVid(`,"hdr":["hdrvivid"]`)},
		{"hdr: entries that aren't tokens skipped alone", videoVid(`,"hdr":["DV","hdr10+",7,"","hdr10plus"]`)},
		{"hdr: the first 8 once sorted kept", videoVid(`,"hdr":["i","h","g","f","e","d","c","b","a"]`)},
		{"hdr: an empty hdr says nothing", videoVid(`,"hdr":[]`)},
		{"hdr: not an array: absent alone", videoVid(`,"hdr":"hdr10plus"`)},
		{"dv: kept without anything in hdr, at its largest values", videoVid(`,"dv":{"profile":127,"level":63,"compatibility":15,"el":false}`)},
		{"dv: profile 5, compatibility 0, without el", videoVid(`,"dv":{"profile":5,"level":6,"compatibility":0}`)},
		{"dv: bl and rpu ignored", videoVid(`,"dv":{"profile":8,"level":6,"compatibility":1,"bl":true,"rpu":true}`)},
		{"dv: el not a boolean: absent alone", videoVid(`,"dv":{"profile":8,"level":6,"compatibility":1,"el":1}`)},
		{"dv: a profile of 128: absent alone", videoVid(`,"dv":{"profile":128,"level":6,"compatibility":1}`)},
		{"dv: a level of 64: absent alone", videoVid(`,"dv":{"profile":8,"level":64,"compatibility":1}`)},
		{"dv: a compatibility of 16: absent alone", videoVid(`,"dv":{"profile":8,"level":6,"compatibility":16}`)},
		{"dv: without its compatibility: absent alone", videoVid(`,"dv":{"profile":5,"level":6}`)},
		{"dv: not an object: absent alone", videoVid(`,"dv":true`)},

		// audioTracks.
		{"audio: a0 and a999", videoCore(`{"id":"a0","codec":"aac","codecs":"mp4a.40.2"},{"id":"a999","codec":"opus"}`) + `}`},
		{"audio: a1000, a01, 1 and a non-object skipped", videoCore(`{"id":"a1000","codec":"ac3"},{"id":"a01","codec":"ac3"},{"id":"1","codec":"ac3"},"a2",{"id":"a2","codec":"ac3"}`) + `}`},
		{"audio: a repeated ID skipped", videoCore(`{"id":"a1","codec":"ac3","language":"en"},{"id":"a1","codec":"eac3","language":"fr"}`) + `}`},
		{"audio: a track without a codec, or with one that isn't a token, skipped", videoCore(`{"id":"a1"},{"id":"a2","codec":null},{"id":"a3","codec":"FLAC"},{"id":"a4","codec":""},{"id":"a5","codec":"flac"}`) + `}`},
		{"audio: tokens nobody registered yet, kept as they are", videoCore(`{"id":"a1","codec":"iamf"},{"id":"a2","codec":"dtsuhd"},{"id":"a3","codec":"pcm_s24le"}`) + `}`},
		{"audio: aac and dts without codecs kept without it", videoCore(`{"id":"a1","codec":"aac"},{"id":"a2","codec":"dts","codecs":"dts+"}`) + `}`},
		{"audio: codecs strings; out of form absent alone", videoCore(`{"id":"a1","codec":"aac","codecs":"mp4a.40.2"},{"id":"a2","codec":"eac3","codecs":"ec-3"},{"id":"a3","codec":"aac","codecs":"mp4a 40 2"}`) + `}`},
		{"audio: the first 32 tracks kept", videoCore(videoQuoted(33, func(i int) string { return fmt.Sprintf(`{"id":"a%d","codec":"ac3"}`, i) })) + `}`},
		{"audio: valid tags kept", videoCore(`{"id":"a1","codec":"ac3","language":"zh-Hant-TW"},{"id":"a2","codec":"ac3","language":"fr-CA"},{"id":"a3","codec":"ac3","language":"yue"},{"id":"a4","codec":"ac3","language":"es-419"}`) + `}`},
		{"audio: tags not in the form a server sends: absent alone", videoCore(`{"id":"a1","codec":"ac3","language":"fre"},{"id":"a2","codec":"ac3","language":"fr-ca"},{"id":"a3","codec":"ac3","language":"FR"},{"id":"a4","codec":"ac3","language":"und"},{"id":"a5","codec":"ac3","language":"zh-Hant-TW1"}`) + `}`},
		{"audio: language of 13 characters or out of form: absent alone", videoCore(`{"id":"a1","codec":"ac3","language":"abcdefghijklm"},{"id":"a2","codec":"ac3","language":"fr_CA"},{"id":"a3","codec":"ac3","language":""},{"id":"a4","codec":"ac3","language":7}`) + `}`},
		{"audio: language of one character: absent alone", videoCore(`{"id":"a1","codec":"ac3","language":"e"}`) + `}`},
		{"audio: channels 1 and 64", videoCore(`{"id":"a1","codec":"ac3","channels":1},{"id":"a2","codec":"ac3","channels":64}`) + `}`},
		{"audio: channels 0 and 65: absent alone", videoCore(`{"id":"a1","codec":"ac3","channels":0},{"id":"a2","codec":"ac3","channels":65}`) + `}`},
		{"audio: roles of the DASH scheme, and one nobody listed, kept", videoCore(`{"id":"a1","codec":"ac3","role":"main"},{"id":"a2","codec":"ac3","role":"dub"},{"id":"a3","codec":"ac3","role":"description"},{"id":"a4","codec":"ac3","role":"karaoke"}`) + `}`},
		{"audio: roles that aren't tokens absent alone", videoCore(`{"id":"a1","codec":"ac3","role":"Commentary"},{"id":"a2","codec":"ac3","role":7}`) + `}`},
		{"audio: flags that aren't booleans: absent alone", videoCore(`{"id":"a1","codec":"ac3","default":"true","forced":1,"original":null}`) + `}`},
		{"audio: objects dolby-atmos and dts-x", videoCore(`{"id":"a1","codec":"truehd","objects":"dolby-atmos"},{"id":"a2","codec":"dts","codecs":"dtsh","objects":"dts-x"}`) + `}`},
		{"audio: objects nobody registered kept; not tokens absent alone", videoCore(`{"id":"a1","codec":"ac4","objects":"ac4-ims"},{"id":"a2","codec":"eac3","objects":"Dolby Atmos"},{"id":"a3","codec":"eac3","objects":["dolby-atmos"]}`) + `}`},
		{"audio: sampleRate 8,000 and 768,000; 7,999 and 768,001 absent alone", videoCore(`{"id":"a1","codec":"pcm_s24le","sampleRate":8000},{"id":"a2","codec":"pcm_s24le","sampleRate":768000},{"id":"a3","codec":"ac3","sampleRate":7999},{"id":"a4","codec":"ac3","sampleRate":768001}`) + `}`},
		{"audio: bitDepth 8 and 32; 7 and 33 absent alone", videoCore(`{"id":"a1","codec":"flac","bitDepth":8},{"id":"a2","codec":"flac","bitDepth":32},{"id":"a3","codec":"flac","bitDepth":7},{"id":"a4","codec":"flac","bitDepth":33}`) + `}`},

		// subtitleTracks.
		{"subtitles: s0 and s999, the registry's tokens", videoCp(`,"subtitleTracks":[{"id":"s0","format":"subrip"},{"id":"s1","format":"ass"},{"id":"s2","format":"ssa"},{"id":"s3","format":"webvtt"},{"id":"s4","format":"mov_text"},{"id":"s5","format":"ttml"},{"id":"s6","format":"hdmv_pgs_subtitle"},{"id":"s7","format":"dvd_subtitle"},{"id":"s8","format":"dvb_subtitle"},{"id":"s9","format":"dvb_teletext"},{"id":"s999","format":"arib_caption"}]`)},
		{"subtitles: s1000, s01, a repeat and a non-object skipped", videoCp(`,"subtitleTracks":[{"id":"s1000","format":"hdmv_pgs_subtitle"},{"id":"s01","format":"hdmv_pgs_subtitle"},{"id":"s1","format":"hdmv_pgs_subtitle"},{"id":"s1","format":"ass"},["s2"]]`)},
		{"subtitles: a format nobody listed kept as it is", videoCp(`,"subtitleTracks":[{"id":"s1","format":"usf"},{"id":"s2","format":"pgs"}]`)},
		{"subtitles: a missing format, or one that isn't a token, absent; the track kept", videoCp(`,"subtitleTracks":[{"id":"s1","format":"SRT"},{"id":"s2"},{"id":"s3","format":4}]`)},
		{"subtitles: roles of the DASH scheme, and one nobody listed, kept", videoCp(`,"subtitleTracks":[{"id":"s1","format":"subrip","role":"subtitle"},{"id":"s2","format":"subrip","role":"commentary"},{"id":"s3","format":"subrip","role":"easyreader"},{"id":"s4","format":"subrip","role":"signs"}]`)},
		{"subtitles: roles that aren't tokens absent alone", videoCp(`,"subtitleTracks":[{"id":"s1","format":"subrip","role":"Forced"},{"id":"s2","format":"subrip","role":1}]`)},
		{"subtitles: members out of range or of the wrong type absent alone", videoCp(`,"subtitleTracks":[{"id":"s1","format":"hdmv_pgs_subtitle","language":"fr_FR","default":"yes","forced":true,"hearingImpaired":0,"external":false}]`)},
		{"subtitles: the first 64 tracks kept", videoCp(`,"subtitleTracks":[` + videoQuoted(65, func(i int) string { return fmt.Sprintf(`{"id":"s%d","format":"subrip"}`, i) }) + `]`)},
		{"subtitles: not an array: absent", videoCp(`,"subtitleTracks":{"id":"s1","format":"subrip"}`)},

		// chapters.
		{"chapters: from 0 to durationMs", videoCp(`,"chapters":[{"startMs":0,"name":"Opening"},{"startMs":3600000,"name":"End"}]`)},
		{"chapters: a repeated or earlier start dropped", videoCp(`,"chapters":[{"startMs":1000},{"startMs":1000,"name":"again"},{"startMs":500},{"startMs":2000}]`)},
		{"chapters: a start out of range dropped", videoCp(`,"chapters":[{"startMs":-1},{"startMs":0},{"startMs":3600001},{"startMs":1.5}]`)},
		{"chapters: names made plain text and cut to 64 characters; empty absent", videoCp(`,"chapters":[{"startMs":0,"name":"` + strings.Repeat("n", 65) + `"},{"startMs":1,"name":"​"},{"startMs":2,"name":3}]`)},
		{"chapters: the first 256 kept", videoCp(`,"chapters":[` + videoQuoted(257, func(i int) string { return fmt.Sprintf(`{"startMs":%d}`, i*1000) }) + `]`)},
		{"chapters: an entry that isn't an object dropped", videoCp(`,"chapters":[0,{"startMs":0}]`)},
		{"chapters: not an array: absent", videoCp(`,"chapters":{"startMs":0}`)},

		// segments.
		{"segments: every kind, end at durationMs", videoCp(`,"segments":[{"kind":"recap","startMs":0,"endMs":30000},{"kind":"intro","startMs":30000,"endMs":90000},{"kind":"commercial","startMs":1800000,"endMs":1860000},{"kind":"outro","startMs":3400000,"endMs":3600000},{"kind":"preview","startMs":3500000,"endMs":3600000}]`)},
		{"segments: a kind nobody listed kept", videoCp(`,"segments":[{"kind":"credits","startMs":0,"endMs":1000},{"kind":"hook","startMs":1000,"endMs":2000}]`)},
		{"segments: kinds that aren't tokens dropped", videoCp(`,"segments":[{"kind":"Intro","startMs":0,"endMs":1000},{"kind":7,"startMs":0,"endMs":1000},{"startMs":0,"endMs":1000},{"kind":"intro","startMs":0,"endMs":1000}]`)},
		{"segments: times out of range or order dropped", videoCp(`,"segments":[{"kind":"intro","startMs":1000,"endMs":1000},{"kind":"intro","startMs":2000,"endMs":1000},{"kind":"intro","startMs":-1,"endMs":1000},{"kind":"outro","startMs":0,"endMs":3600001},{"kind":"outro","startMs":0}]`)},
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
	film := `"key":"movie/tmdb/949","library":"v1"`
	bare := `"key":"series/tmdb/1399/e63056","library":"v2"` + cs
	episode := bare + `,"place":{"season":1,"episode":1}`
	series := `"key":"series/tmdb/1399","library":"v2"`
	season := `"key":"series/tmdb/1399/s1","library":"v2"`
	numbered := `"key":"series/tmdb/1399/s1/e2","library":"v2"` + cs
	key := func(k string) string {
		return `{"key":"` + k + `","library":"v1"` + cs + `}`
	}
	copyN := func(i int) string { return strings.Replace(videoC, `"v1"`, fmt.Sprintf(`"c%d"`, i), 1) }
	var with []string
	for i := 0; i < 18; i++ {
		with = append(with, fmt.Sprintf(`"series/tmdb/1399/e%d"`, 63060+i))
	}
	in := []videoInput{
		{"a film", `{` + film + cs + `}`},
		{"an episode keyed by its TMDB ID", `{` + episode + `,"with":["series/tmdb/1399/e63057"],"ids":{"imdb":"tt1480055","tvdb":"3254641"}}`},
		{"a series", `{` + series + `}`},
		{"a season", `{` + season + `}`},
		{"an episode by its numbers", `{` + numbered + `}`},
		{"a series of the tvdb scheme, with ids", `{"key":"series/tvdb/81189","library":"v2","ids":{"imdb":"tt0903747"}}`},
		{"an episode of the tvdb scheme, by its absolute number", `{"key":"series/tvdb/81189/a27","library":"v2"` + cs + `,"ids":{"tvdb":"349232"}}`},
		{"the summary of the first version 0 (audio, subtitles, video): ignored", `{` + film + cs + `,"audio":["en"],"subtitles":["fr"],"video":{"height":2160,"hdr":["hdr10"]}}`},

		// Keys (section 4).
		{"a TMDB ID of 10 digits", key("movie/tmdb/9999999999")},
		{"a TMDB ID of 11 digits: skipped", key("movie/tmdb/10000000000")},
		{"a key with a leading zero: skipped", key("movie/tmdb/0949")},
		{"a TMDB ID of 0: skipped", key("movie/tmdb/0")},
		{"season 0 and episode 0", `{"key":"series/tmdb/1399/s0/e0","library":"v1"` + cs + `}`},
		{"season 9,999 and episode 99,999", `{"key":"series/tmdb/1399/s9999/e99999","library":"v1"` + cs + `}`},
		{"season 10,000: skipped", `{"key":"series/tmdb/1399/s10000","library":"v1"}`},
		{"episode 100,000: skipped", key("series/tmdb/1399/s1/e100000")},
		{"season s01: skipped", `{"key":"series/tmdb/1399/s01","library":"v1"}`},
		{"a TMDB episode ID of 10 digits", `{"key":"series/tmdb/1399/e9999999999","library":"v1"` + cs + `,"place":{"season":1,"episode":1}}`},
		{"a TMDB episode ID of 11 digits: skipped", `{"key":"series/tmdb/1399/e10000000000","library":"v1"` + cs + `,"place":{"season":1,"episode":1}}`},
		{"a TMDB episode ID with a leading zero: skipped", `{"key":"series/tmdb/1399/e063056","library":"v1"` + cs + `,"place":{"season":1,"episode":1}}`},
		{"an absolute number under tmdb: skipped", key("series/tmdb/1399/a27")},
		{"a film with a season: skipped", key("movie/tmdb/949/s1")},
		{"a film of the tvdb scheme: skipped", key("movie/tvdb/603")},
		{"an unknown scheme: skipped", key("movie/imdb/tt0113277")},
		{"a kind in another case: skipped", key("Movie/tmdb/949")},
		{"a key ending in /: skipped", key("movie/tmdb/949/")},
		{"a music key: skipped", `{"key":"track/mb/0c000000-0000-4000-8000-000000000003","library":"v1"}`},
		{"a key that isn't a string: skipped", `{"key":949,"library":"v1"` + cs + `}`},
		{"a kind member ignored, even one the key doesn't name", `{"key":"movie/tmdb/949","kind":"series","library":"v1"` + cs + `}`},
		{"a library not asked for: skipped", `{"key":"movie/tmdb/949","library":"v3"` + cs + `}`},
		{"not an object: skipped", `"movie/tmdb/949"`},

		// copies (sections 3.1 and 3.8).
		{"copies: 16 copies kept", `{` + film + `,"copies":[` + videoQuoted(16, copyN) + `]}`},
		{"copies: the first 16 kept", `{` + film + `,"copies":[` + videoQuoted(17, copyN) + `]}`},
		{"copies: skipped copies don't count toward the 16", `{` + film + `,"copies":[7,{"id":"bad"},` + videoQuoted(17, copyN) + `]}`},
		{"copies: a repeated id skipped alone", `{` + film + `,"copies":[` + copyN(1) + `,` + strings.Replace(copyN(1), `"t1"`, `"t2"`, 1) + `,` + copyN(2) + `]}`},
		{"copies: an entry that isn't an object skipped alone", `{` + film + `,"copies":["v0",` + videoC + `]}`},
		{"copies: a film with no copy left: skipped", `{` + film + `,"copies":[{"id":"v1","durationMs":-1,"mediaType":"video/matroska","size":1,"etag":"t1","audioTracks":[]}]}`},
		{"copies: empty: skipped", `{` + film + `,"copies":[]}`},
		{"copies missing on a film: skipped", `{` + film + `}`},
		{"copies missing on an episode: skipped", `{"key":"series/tmdb/1399/s1/e2","library":"v2"}`},
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
		{"place of a tvdb episode: ignored", `{"key":"series/tvdb/81189/s1/e2","library":"v2"` + cs + `,"place":{"season":3,"episode":4}}`},
		{"place of a film: ignored", `{` + film + cs + `,"place":{"season":1,"episode":1}}`},

		// with (section 4.2).
		{"with: other series, itself, films, seasons, extras, bad keys, non-strings and repeats skipped, the first 16 kept", `{` + episode + `,"with":["series/tmdb/1400/e63057","series/tmdb/1399/e63056","movie/tmdb/949","series/tmdb/1399/s1","series/tmdb/1399/x/Tr1","series/tmdb/1399/e063057",7,` + strings.Join(with[:2], ",") + `,` + with[0] + `,` + strings.Join(with[2:], ",") + `]}`},
		{"with: episodes of the same series in both forms", `{` + numbered + `,"with":["series/tmdb/1399/s1/e3","series/tmdb/1399/e63058"]}`},
		{"with: a tvdb episode's, the same series only", `{"key":"series/tvdb/81189/a27","library":"v2"` + cs + `,"with":["series/tvdb/81189/a28","series/tvdb/81190/a28","series/tmdb/81189/s1/e2"]}`},
		{"with: not an array: absent", `{` + episode + `,"with":"series/tmdb/1399/e63057"}`},
		{"with of a film: ignored", `{` + film + cs + `,"with":["series/tmdb/1399/e63057"]}`},

		// ids (section 4.3).
		{"ids: imdb of 7 and tvdb of 10 digits on an episode", `{` + numbered + `,"ids":{"imdb":"tt0000001","tvdb":"9999999999"}}`},
		{"ids: imdb of 10 digits", `{` + numbered + `,"ids":{"imdb":"tt1234567890"}}`},
		{"ids: values out of form, unknown schemes and non-strings skipped", `{` + numbered + `,"ids":{"imdb":"tt012345","tvdb":"081189","tmdb":"949","mb":"x"}}`},
		{"ids: imdb of 11 digits and tvdb of 11 digits skipped", `{` + numbered + `,"ids":{"imdb":"tt12345678901","tvdb":"10000000000"}}`},
		{"ids on a film: ignored (TMDB's lookup gives them)", `{` + film + cs + `,"ids":{"imdb":"tt0113277","tvdb":"603"}}`},
		{"ids on a series of the tmdb scheme: ignored", `{` + series + `,"ids":{"imdb":"tt0944947","tvdb":"121361"}}`},
		{"ids: tvdb on a series of the tvdb scheme skipped (episodes only)", `{"key":"series/tvdb/81189","library":"v2","ids":{"tvdb":"81189","imdb":"tt0903747"}}`},
		{"ids: tmdb on a tvdb series, once TMDB has it", `{"key":"series/tvdb/81189","library":"v2","ids":{"tmdb":"1396","imdb":"tt0903747"}}`},
		{"ids: tmdb on a tvdb episode", `{"key":"series/tvdb/81189/s1/e2","library":"v2"` + cs + `,"ids":{"tmdb":"62086"}}`},
		{"ids: tmdb out of form on a tvdb series skipped", `{"key":"series/tvdb/81189","library":"v2","ids":{"tmdb":"01396"}}`},
		{"ids: one good, one not a string", `{` + numbered + `,"ids":{"imdb":7,"tvdb":"81189"}}`},
		{"ids: not an object: absent", `{` + numbered + `,"ids":["tt0113277"]}`},
		{"ids of a season: ignored", `{` + season + `,"ids":{"tvdb":"81189"}}`},

		// Members other titles may carry (sections 3.9 and 3.10).
		{"extraType, name, year and image on a film of the tmdb scheme: ignored", `{` + film + cs + `,"extraType":"trailer","name":"Heat","year":1995,"image":true}`},
	}
	return videoTitleSet(in)
}

func videoExtraCases() []videoCase {
	cs := `,"copies":[` + videoC + `]`
	x := func(k, members string) string {
		return `{"key":"` + k + `","library":"v1"` + cs + members + `}`
	}
	opaque := strings.Repeat("Q", 64)
	in := []videoInput{
		{"a trailer of a film", x("movie/tmdb/348/x/Tr7aQ1", `,"extraType":"trailer"`)},
		{"a featurette of a series", x("series/tmdb/1399/x/Fe_1", `,"extraType":"featurette"`)},
		{"an extra of a season", x("series/tmdb/1399/s1/x/Bt-2", `,"extraType":"behindthescenes"`)},
		{"an extra of a tvdb series", x("series/tvdb/81189/x/In3", `,"extraType":"interview"`)},
		{"an extra of a tvdb season", x("series/tvdb/81189/s0/x/Sc4", `,"extraType":"scene"`)},
		{"an extra of a local film", x("movie/local/q3Zp0Lw8/x/Sh5", `,"extraType":"short"`)},
		{"an extra of a local season", x("series/local/Hk2v9TnA/s1/x/De6", `,"extraType":"deletedscene"`)},
		{"extraType clip", x("movie/tmdb/348/x/Cl7", `,"extraType":"clip"`)},
		{"extra part of 64 characters", x("movie/tmdb/348/x/"+opaque, `,"extraType":"trailer"`)},
		{"extraType themesong", x("movie/tmdb/348/x/Th1", `,"extraType":"themesong"`)},
		{"extraType themevideo of a series", x("series/tmdb/1399/x/Th2", `,"extraType":"themevideo"`)},
		{"extraType MSFP never listed, kept as it is", x("movie/tmdb/348/x/Tr7aQ1", `,"extraType":"bloopers"`)},
		{"extraType in capitals isn't a token: skipped", x("movie/tmdb/348/x/Tr7aQ1", `,"extraType":"Trailer"`)},
		{"extraType missing: skipped", x("movie/tmdb/348/x/Tr7aQ1", ``)},
		{"extraType not a string: skipped", x("movie/tmdb/348/x/Tr7aQ1", `,"extraType":1`)},
		{"name on an extra: ignored (extras carry none)", x("movie/tmdb/348/x/Tr7aQ1", `,"extraType":"trailer","name":"Teaser"`)},
		{"name on an extra, even not a string: ignored", x("movie/tmdb/348/x/Tr7aQ1", `,"extraType":"trailer","name":["Teaser"]`)},
		{"ids, place, with, year and image on an extra: ignored", x("movie/tmdb/348/x/Tr7aQ1", `,"extraType":"trailer","ids":{"imdb":"tt0078748"},"place":{"season":1,"episode":1},"with":["movie/tmdb/348/x/Tr8"],"year":1979,"image":true`)},
		{"an extra without copies: skipped", `{"key":"movie/tmdb/348/x/Tr7aQ1","library":"v1","extraType":"trailer"}`},
		{"an extra whose copies isn't an array: skipped", `{"key":"movie/tmdb/348/x/Tr7aQ1","library":"v1","extraType":"trailer","copies":{}}`},
		{"an extra of an episode: skipped", x("series/tmdb/1399/s1/e1/x/Tr1", `,"extraType":"trailer"`)},
		{"an extra of an episode keyed by its TMDB ID: skipped", x("series/tmdb/1399/e63056/x/Tr1", `,"extraType":"trailer"`)},
		{"an extra of an extra: skipped", x("movie/tmdb/348/x/Tr1/x/Tr2", `,"extraType":"trailer"`)},
		{"an extra part of 65 characters: skipped", x("movie/tmdb/348/x/"+opaque+"Q", `,"extraType":"trailer"`)},
		{"an extra part with a dot: skipped", x("movie/tmdb/348/x/Tr.1", `,"extraType":"trailer"`)},
		{"an empty extra part: skipped", x("movie/tmdb/348/x/", `,"extraType":"trailer"`)},
		{"x without an extra part: skipped", x("movie/tmdb/348/x", `,"extraType":"trailer"`)},
		{"an extra under a bad key: skipped", x("movie/tmdb/0348/x/Tr1", `,"extraType":"trailer"`)},
		{"a kind member ignored: the key names an extra", `{"key":"movie/tmdb/348/x/Tr7aQ1","kind":"movie","library":"v1","extraType":"trailer"` + cs + `}`},
	}
	return videoTitleSet(in)
}

func videoLocalTitleCases() []videoCase {
	cs := `,"copies":[` + videoC + `]`
	film := `"key":"movie/local/q3Zp0Lw8","library":"v1"` + cs
	series := `"key":"series/local/Hk2v9TnA","library":"v2"`
	in := []videoInput{
		{"a local film with its whole description", `{` + film + `,"name":"Summer 2019","year":2019,"image":true}`},
		{"a local series with a name only", `{` + series + `,"name":"Family videos"}`},
		{"a local season: no description", `{"key":"series/local/Hk2v9TnA/s1","library":"v2"}`},
		{"a local episode: no description, copies kept", `{"key":"series/local/Hk2v9TnA/s1/e3","library":"v2"` + cs + `}`},
		{"a description on a local season or episode: ignored", `{"key":"series/local/Hk2v9TnA/s1/e3","library":"v2"` + cs + `,"name":"Day 3","year":2019,"image":true}`},
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
		{"place on a local episode: ignored", `{"key":"series/local/Hk2v9TnA/s1/e3","library":"v2"` + cs + `,"place":{"season":1,"episode":3}}`},
		{"with on a local episode: the same series only", `{"key":"series/local/Hk2v9TnA/s1/e3","library":"v2"` + cs + `,"with":["series/local/Hk2v9TnA/s1/e4","series/local/Other/s1/e4"]}`},
		{"a local film without copies: skipped", `{"key":"movie/local/q3Zp0Lw8","library":"v1","name":"Summer"}`},
		{"a description on a film of the tmdb scheme: ignored", `{"key":"movie/tmdb/949","library":"v1"` + cs + `,"name":"Heat","year":1995,"image":true}`},
		{"a description on a series of the tvdb scheme: ignored", `{"key":"series/tvdb/81189","library":"v2","name":"Breaking Bad","year":2008}`},
		{"a local key out of form: skipped", `{"key":"movie/local/q3Zp.0Lw8","library":"v1"` + cs + `,"name":"Summer"}`},
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
		{"a ticket and the copy's etag", `{"ticket":"Tk7Qw2xLp9mZ","etag":"k9Tq2w"}`},
		{"a ticket and an etag of 64 characters", `{"ticket":"` + strings.Repeat("T", 64) + `","etag":"` + strings.Repeat("e", 64) + `"}`},
		{"unknown members ignored, the old trickplay object too", `{"ticket":"Tk7","etag":"t1","trickplay":{"tileWidth":320}}`},
		{"ticket missing: refused", `{"etag":"t1"}`},
		{"ticket of 65 characters: refused", `{"ticket":"` + strings.Repeat("T", 65) + `","etag":"t1"}`},
		{"ticket with a dot: refused", `{"ticket":"Tk.7","etag":"t1"}`},
		{"ticket as a URL: refused", `{"ticket":"https://a.example.org/api/v1/stream/Tk7/","etag":"t1"}`},
		{"ticket not a string: refused", `{"ticket":7,"etag":"t1"}`},
		{"etag missing: refused", `{"ticket":"Tk7"}`},
		{"etag out of its form: refused", `{"ticket":"Tk7","etag":"t+1"}`},
		{"not an object: refused", `[]`},
	}
	var out []videoPreviewsCase
	for _, c := range requests {
		ok, r := videoReason(checkVideoPreviewsRequest([]byte(c.text)))
		out = append(out, videoPreviewsCase{c.name, "request", c.text, ok, "", r})
	}
	for _, c := range answers {
		got, err := keepVideoPreviewsAnswer([]byte(c.text))
		ok, r := videoReason(err)
		out = append(out, videoPreviewsCase{c.name, "answer", c.text, ok, string(got), r})
	}
	return out
}

// videoAnswerCopy is the copy of the catalog the answers cases are
// for, as the caller keeps it.
var videoAnswerCopy = `{"id":"v1","durationMs":3600000,"mediaType":"video/matroska","size":1000000,"etag":"t1","audioTracks":[{"id":"a1","codec":"ac3"}],"subtitleTracks":[{"id":"s2","format":"subrip"},{"id":"s3","format":"ass"},{"id":"s4","format":"hdmv_pgs_subtitle"},{"id":"s5","format":"dvb_teletext"},{"id":"s6"},{"id":"s7","format":"xsub"}],"previewLayout":{"tileWidth":320,"tileHeight":180,"columns":10,"rows":10,"intervalMs":10000,"tileCount":150}}`

func videoAnswerCases() []videoCase {
	tk := `"ticket":"Tk7Qw2xLp9mZ"`
	sub := func(entries string) string { return `{` + tk + `,"subtitles":[` + entries + `]}` }
	in := []videoInput{
		{"every form, fonts, and the copy's sheets", `{` + tk + `,"subtitles":[{"id":"s2","form":"text"},{"id":"s3","form":"text","ass":true},{"id":"s4","form":"picture"},{"id":"s5","form":"text"},{"id":"s7","form":"picture"}],"fontCount":3}`},
		{"no subtitles, no fonts: the sheets only", `{` + tk + `}`},
		{"a track with no format served as text", sub(`{"id":"s6","form":"text"}`)},
		{"a track of another format may be left out", sub(`{"id":"s2","form":"text"}`)},
		{"fontCount 64", `{` + tk + `,"fontCount":64}`},
		{"fontCount 65: 0", `{` + tk + `,"fontCount":65}`},
		{"fontCount -1: 0", `{` + tk + `,"fontCount":-1}`},
		{"fontCount not an integer: 0", `{` + tk + `,"fontCount":"3"}`},
		{"an entry for a track not in the catalog dropped alone", sub(`{"id":"s9","form":"text"},{"id":"s2","form":"text"}`)},
		{"an entry with a form neither text nor picture dropped alone", sub(`{"id":"s5","form":"bitmap"},{"id":"s7","form":"picture"}`)},
		{"a repeated entry dropped", sub(`{"id":"s2","form":"text"},{"id":"s2","form":"text"}`)},
		{"an entry that isn't an object, or without an id, dropped", sub(`"s2",{"form":"text"},{"id":"s4","form":"picture"}`)},
		{"ass with picture counts as absent", sub(`{"id":"s7","form":"picture","ass":true}`)},
		{"ass not a boolean counts as absent", sub(`{"id":"s5","form":"text","ass":"yes"}`)},
		{"ass false counts as absent", sub(`{"id":"s6","form":"text","ass":false}`)},
		{"a listed format in another form than the table's dropped", sub(`{"id":"s2","form":"picture"},{"id":"s4","form":"text"}`)},
		{"ass on subrip, against the table, dropped", sub(`{"id":"s2","form":"text","ass":true}`)},
		{"an ass track without ass: true, against the table, dropped", sub(`{"id":"s3","form":"text"}`)},
		{"URLs in an entry ignored: they are built", sub(`{"id":"s2","form":"text","url":"https://evil.example.org/x"}`)},
		{"the first 64 entries kept", `{` + tk + `,"subtitles":[` + videoQuoted(66, func(i int) string { return `{"id":"s2","form":"text"}` }) + `]}`},
		{"subtitles not an array: none", `{` + tk + `,"subtitles":{"id":"s2","form":"text"}}`},
		{"ticket missing: refused", `{"subtitles":[{"id":"s2","form":"text"}]}`},
		{"ticket out of its form: refused", `{"ticket":"Tk7/../x"}`},
		{"not an object: refused", `[]`},
	}
	v, _ := videoDecode([]byte(videoAnswerCopy))
	cp, err := keepCopy(v)
	if err != nil {
		panic(err)
	}
	var out []videoCase
	for _, c := range in {
		got, err := keepVideoAnswer([]byte(c.text), cp, videoStreamRoot)
		ok, r := videoReason(err)
		out = append(out, videoCase{c.name, c.text, ok, string(got), r})
	}
	return out
}

func videoCodecsCases() []videoCase {
	in := []videoInput{
		// The six codecs for which codecs is required.
		{"h264: High at level 4.0", `{"codec":"h264","codecs":"avc1.640028"}`},
		{"h264: avc3", `{"codec":"h264","codecs":"avc3.4D401F"}`},
		{"h264: constraint flags ignored (the stream's or zero)", `{"codec":"h264","codecs":"avc1.640828"}`},
		{"h264: lowercase hexadecimal", `{"codec":"h264","codecs":"avc1.64001f"}`},
		{"h264: codecs missing: can't play as it is", `{"codec":"h264"}`},
		{"h264: codecs malformed (counts as absent): can't play as it is", `{"codec":"h264","codecs":"avc1.64 28"}`},
		{"h264: codecs of another codec: can't play as it is", `{"codec":"h264","codecs":"hvc1.1.2.L123.B0"}`},
		{"h264: the 4CC in capitals (compared case-sensitively): can't play as it is", `{"codec":"h264","codecs":"AVC1.640028"}`},
		{"h264: two bytes only: can't play as it is", `{"codec":"h264","codecs":"avc1.6400"}`},
		{"h264 with Dolby Vision: the base avc1 string, Dolby Vision rebuilt as dva1", `{"codec":"h264","codecs":"avc1.640028","dv":{"profile":9,"level":5,"compatibility":2}}`},
		{"hevc: Main 10, tier L, level 5.1", `{"codec":"hevc","codecs":"hvc1.2.4.L153.B0","bitDepth":10}`},
		{"hevc: hev1, tier H", `{"codec":"hevc","codecs":"hev1.1.6.H150.90"}`},
		{"hevc: compatibility flags ignored (only the profile's own bit)", `{"codec":"hevc","codecs":"hvc1.2.4.L153.B0"}`},
		{"hevc: compatibility flags ignored (the stream's)", `{"codec":"hevc","codecs":"hvc1.2.6.L153.B0"}`},
		{"hevc: no constraint bytes on Main", `{"codec":"hevc","codecs":"hvc1.1.2.L93"}`},
		{"hevc: range extensions (profile 4) with constraint bytes", `{"codec":"hevc","codecs":"hvc1.4.10.L120.9D.08"}`},
		{"hevc: profile 4 without constraint bytes: can't play as it is", `{"codec":"hevc","codecs":"hvc1.4.10.L120"}`},
		{"hevc: a tier other than L or H: can't play as it is", `{"codec":"hevc","codecs":"hvc1.2.4.M153.B0"}`},
		{"hevc: a Dolby Vision 4CC (dvh1) instead of the base's: can't play as it is", `{"codec":"hevc","codecs":"dvh1.08.06"}`},
		{"hevc with Dolby Vision profile 8: hvc1 and dv give dvh1.08.06", `{"codec":"hevc","codecs":"hvc1.2.4.L150.B0","bitDepth":10,"dv":{"profile":8,"level":6,"compatibility":1}}`},
		{"hevc with Dolby Vision profile 7: hev1 and dv give dvhe.07.06", `{"codec":"hevc","codecs":"hev1.2.4.L153.B0","dv":{"profile":7,"level":6,"compatibility":6,"el":true}}`},
		{"hevc: codecs missing: can't play as it is", `{"codec":"hevc","codecs":""}`},
		{"av1: Main, level 5.0, 10 bits", `{"codec":"av1","codecs":"av01.0.12M.10"}`},
		{"av1: the long form, 4:2:0; its colour parts ignored", `{"codec":"av1","codecs":"av01.0.08M.10.0.110.09.16.09.0"}`},
		{"av1: profile 1 (4:4:4) with the long form", `{"codec":"av1","codecs":"av01.1.08H.10.0.000.01.01.01.0"}`},
		{"av1: profile 1 without the long form: can't play as it is", `{"codec":"av1","codecs":"av01.1.08H.10"}`},
		{"av1: monochrome", `{"codec":"av1","codecs":"av01.0.04M.08.1.110.01.01.01.0"}`},
		{"av1: bitDepth decides when it differs from codecs", `{"codec":"av1","codecs":"av01.0.08M.08","bitDepth":10}`},
		{"av1 with Dolby Vision: dav1", `{"codec":"av1","codecs":"av01.0.13M.10","dv":{"profile":10,"level":9,"compatibility":1}}`},
		{"av1: a short form cut: can't play as it is", `{"codec":"av1","codecs":"av01.0.08M"}`},
		{"vp9: profile 0, 8 bits", `{"codec":"vp9","codecs":"vp09.00.41.08"}`},
		{"vp9: the level ignored (the stream's or a guess)", `{"codec":"vp9","codecs":"vp09.00.10.08"}`},
		{"vp9: profile 2, 10 bits, the long form", `{"codec":"vp9","codecs":"vp09.02.10.10.01.09.16.09.01"}`},
		{"vp9: profile 1 with the long form, 4:4:4", `{"codec":"vp9","codecs":"vp09.01.20.08.03"}`},
		{"vp9: profile 1 without the long form: can't play as it is", `{"codec":"vp9","codecs":"vp09.01.20.08"}`},
		{"vp9: a chroma subsampling of 04: can't play as it is", `{"codec":"vp9","codecs":"vp09.00.10.08.04"}`},
		{"vp9: codecs missing: can't play as it is", `{"codec":"vp9"}`},
		{"aac: LC", `{"codec":"aac","codecs":"mp4a.40.2"}`},
		{"aac: HE-AAC v2", `{"codec":"aac","codecs":"mp4a.40.29"}`},
		{"aac: MPEG-2 AAC LC", `{"codec":"aac","codecs":"mp4a.67"}`},
		{"aac: codecs missing: can't play as it is", `{"codec":"aac"}`},
		{"aac: MP3's string: can't play as it is", `{"codec":"aac","codecs":"mp4a.6B"}`},
		{"aac: mp4a.40 without an object type: can't play as it is", `{"codec":"aac","codecs":"mp4a.40"}`},
		{"dts: dtsh", `{"codec":"dts","codecs":"dtsh"}`},
		{"dts: dtse", `{"codec":"dts","codecs":"dtse"}`},
		{"dts: dtsx is DTS-UHD's, not dts's: can't play as it is", `{"codec":"dts","codecs":"dtsx"}`},
		{"dts: codecs missing: can't play as it is", `{"codec":"dts"}`},

		// Every other codec: optional.
		{"eac3: the registry's fixed ec-3", `{"codec":"eac3","codecs":"ec-3"}`},
		{"eac3: codecs left out", `{"codec":"eac3"}`},
		{"truehd: mlpa", `{"codec":"truehd","codecs":"mlpa"}`},
		{"vvc: a string passed as it is", `{"codec":"vvc","codecs":"vvc1.1.L51.CQA"}`},
		{"vvc: a string with + is out of form, absent; not required for vvc", `{"codec":"vvc","codecs":"vvc1.1.L51.CQA.O1+3"}`},
		{"mpeg4: mp4v.20.9", `{"codec":"mpeg4","codecs":"mp4v.20.9"}`},
		{"a codec nobody registered, without codecs", `{"codec":"h267"}`},
		{"codec missing: not a stream", `{"codecs":"avc1.640028"}`},
	}
	var out []videoCase
	for _, c := range in {
		got, err := readVideoCodecs([]byte(c.text))
		ok, r := videoReason(err)
		out = append(out, videoCase{c.name, c.text, ok, string(got), r})
	}
	return out
}

func videoFontSetCases() []videoCase {
	const mib = 1 << 20
	sizes := func(n int, f func(i int) int) string {
		return `[` + videoQuoted(n, func(i int) string { return fmt.Sprint(f(i)) }) + `]`
	}
	in := []videoInput{
		{"no font", `[]`},
		{"one font of 10 MiB", sizes(1, func(int) int { return 10 * mib })},
		{"64 fonts of 1 MiB: 64 MiB in all", sizes(64, func(int) int { return mib })},
		{"six fonts of 10 MiB and one of 4 MiB: 64 MiB in all", sizes(7, func(i int) int {
			if i == 6 {
				return 4 * mib
			}
			return 10 * mib
		})},
		{"a font of 1 byte", `[1]`},
		{"65 fonts of 1 byte: refused", sizes(65, func(int) int { return 1 })},
		{"a font of 10 MiB and 1 byte: refused", sizes(1, func(int) int { return 10*mib + 1 })},
		{"seven fonts of 10 MiB: over 64 MiB in all, refused", sizes(7, func(int) int { return 10 * mib })},
		{"64 MiB and 1 byte in all: refused", sizes(7, func(i int) int {
			if i == 6 {
				return 4*mib + 1
			}
			return 10 * mib
		})},
		{"a size of 0: refused", `[1000,0]`},
		{"a size that isn't an integer: refused", `[1000.5]`},
		{"not an array: refused", `{"fonts":1}`},
	}
	var out []videoCase
	for _, c := range in {
		got, err := checkVideoFontSet([]byte(c.text))
		ok, r := videoReason(err)
		out = append(out, videoCase{c.name, c.text, ok, string(got), r})
	}
	return out
}

func genV0Video() []byte {
	return encode(videoFile{
		About: []string{
			"MSFP video part version 0, as revised by proposals 16 and 17 (spec/v0/video.md): what a caller keeps of a video title (section 3.1), of its copies (sections 3.2 to 3.8), of an extra (section 3.9) and of a local title (section 3.10); what a viewer's server reads from a codecs string (section 3.5); title keys of the tvdb and local schemes (sections 4.4 and 4.5); the video members of play's answer and the URLs built from its ticket (sections 5.3 and 6.1); the previews call (section 5.6); the fonts of a copy (section 6.6). Generated by tools/vectors, which re-derives every output from its text; no implementation has produced them yet.",
			"titles, extras, localTitles: text is one entry of a titles answer, as JSON; the caller asked for the libraries listed in libraries. A title has no kind: its key names it, and a kind member is ignored. valid false: the caller skips the title, for the reason given. valid true: output is what it keeps, members in this order: key, library, extraType, name, year, image, place (season, episode), with, ids (its members sorted), copies (each as in copies); absent members left out.",
			"copies: text is one entry of a title's copies. valid false: the caller skips that copy alone. valid true: output is what it keeps, members in the order of the table of section 3.2 (id, edition, durationMs, startMs, endMs, mediaType, size, etag, parts, video, audioTracks, subtitleTracks, chapters, segments, previewLayout, threeD), streams and tracks in the order of the tables of sections 3.3, 3.4 and 5.5; absent members left out, audioTracks always written. The cap of 16 copies, repeated copy IDs and a title left without copies are in titles.",
			"Open fields (the codecs, edition, role, subtitle format, objects, segment kind, extraType, hdr, threeD) keep any token of core section 8.5 (1 to 32 characters of a-z 0-9 . _ -), known or not, and mediaType any media type of core section 8.5 (lowercase, one /, each half starting with a letter or a digit, at most 127 characters of a-z 0-9 . + - _ /); a value that isn't well formed is dropped alone when optional, and a required one (mediaType, a video or audio codec, extraType) takes its copy, video, track or extra with it. colour members are H.273 code points, 0 to 255, 2 counting as absent.",
			"codecs: text is a video stream or an audio track as kept (codec, codecs, bitDepth, dv). valid true: output is what the viewer's server decides by: the parts of codecs it reads (profile, tier, level, chroma, constraints, mp4a, the part after mp4a for AAC), the parts it ignores left out (H.264 constraint flags, HEVC compatibility flags, VP9 level, AV1 colour), bitDepth from the stream's own field when it has one, else from codecs, and dolbyVision, the Dolby Vision string rebuilt from the base 4CC and dv. valid false: codecs missing or malformed where it is required (h264, hevc, av1, vp9, aac, dts), or unreadable for its codec: the viewer's server treats the copy as one it can't play as it is.",
			"answers: text is the video members of play's answer (subtitles, fontCount) with the playback's ticket, for the copy answerCopy (as kept), the owning server's stream root being streamRoot. valid true: output is what the viewer's server keeps, and urls, every URL it may build from the ticket: each subtitle entry's (subtitles/, pictures/, ass/), fonts/0 to fontCount - 1, and the copy's sheets, trickplay/0 to the sheet count of its previewLayout - 1. valid false: it refuses the answer.",
			"fontSets: text is the sizes in bytes of one copy's fonts. valid true: within the bounds of section 6.6, output their count and total; valid false: the first font that breaks a bound, which the owning server doesn't serve and a viewer's server refuses.",
			"localKeys, tvdbKeys: text is a title key. valid true: kind is the kind it names (movie, series, season, episode or extra); valid false: the receiver skips a title with that key.",
			"previews: side request is a previews request as the owning server reads it: valid false means it answers 400 (the 404s depend on what it offers, not on the request). side answer is a previews answer as the viewer's server reads it: valid false means it refuses the answer and fetches no sheet; valid true: output is the ticket and etag it keeps.",
			"Tracks' languages are kept only as valid tags, in the form catalog section 3 sends them (a tag sentTag of strict.json's languageCodes leaves unchanged), never converted; the conversions of catalog section 3 are in strict.json (languageCodes). The WebVTT, ASS, PGS and font cases of video section 6 are in media.json.",
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
		AnswerCopy:  videoAnswerCopy,
		Answers:     videoAnswerCases(),
		Codecs:      videoCodecsCases(),
		FontSets:    videoFontSetCases(),
	})
}
