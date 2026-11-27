package main

import (
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"strings"
	"unicode/utf8"
)

// vttCase is a WebVTT or ASS part and what rebuilding it gives.
type vttCase struct {
	Name   string `json:"name"`
	Text   string `json:"text,omitempty"`
	Hex    string `json:"hex,omitempty"`
	Valid  bool   `json:"valid"`
	Output string `json:"output,omitempty"`
	Reason string `json:"reason,omitempty"`
}

// mediaFontCase is a font's bytes and what the checks say.
type mediaFontCase struct {
	Name   string `json:"name"`
	Hex    string `json:"hex"`
	Valid  bool   `json:"valid"`
	Type   string `json:"type,omitempty"`
	Reason string `json:"reason,omitempty"`
}

// mediaRangeCase is a Range header against a file's size.
type mediaRangeCase struct {
	Name   string `json:"name"`
	Text   string `json:"text"`
	Size   int64  `json:"size"`
	Valid  bool   `json:"valid"`
	Start  *int64 `json:"start,omitempty"`
	End    *int64 `json:"end,omitempty"`
	Reason string `json:"reason,omitempty"`
}

// mediaAnswerCase is an answer for the file as the viewer's server checks it.
type mediaAnswerCase struct {
	Name          string `json:"name"`
	Range         string `json:"range,omitempty"`
	Size          int64  `json:"size"`
	ETag          string `json:"etag"`
	Status        int    `json:"status"`
	ETagHeader    string `json:"etagHeader,omitempty"`
	ContentRange  string `json:"contentRange,omitempty"`
	ContentLength string `json:"contentLength,omitempty"`
	Valid         bool   `json:"valid"`
	Reason        string `json:"reason,omitempty"`
}

// mediaScanCase is a fetch under a scan ticket and the owning server's answer.
type mediaScanCase struct {
	Name         string `json:"name"`
	ScanBytes    int64  `json:"scanBytes,omitempty"`
	Sent         int64  `json:"sent"`
	Range        string `json:"range,omitempty"`
	Size         int64  `json:"size"`
	Status       int    `json:"status"`
	ContentRange string `json:"contentRange"`
}

// mediaNameCase is a name under a ticket and whether the owning server
// serves it.
type mediaNameCase struct {
	Name   string      `json:"name"`
	Ticket mediaTicket `json:"ticket"`
	Text   string      `json:"text"`
	Valid  bool        `json:"valid"`
	Reason string      `json:"reason,omitempty"`
}

// mediaRelayCase is what the viewer's server relays the file or a segment
// as.
type mediaRelayCase struct {
	Name   string `json:"name"`
	What   string `json:"what"`
	Value  string `json:"value"`
	Output string `json:"output"`
}

type v1MediaFile struct {
	About       []string          `json:"about"`
	Names       []mediaNameCase   `json:"names"`
	RelayTypes  []mediaRelayCase  `json:"relayTypes"`
	WebVTT      []vttCase         `json:"webvtt"`
	ASS         []vttCase         `json:"ass"`
	PGS         []verdict         `json:"pgs"`
	Fonts       []mediaFontCase   `json:"fonts"`
	HLS         []verdict         `json:"hls"`
	Ranges      []mediaRangeCase  `json:"ranges"`
	FileAnswers []mediaAnswerCase `json:"fileAnswers"`
	Scans       []mediaScanCase   `json:"scans"`
}

const cue = "00:00:01.000 --> 00:00:02.000\n"

func vttCases() []vttCase {
	in := []struct{ name, text string }{
		{"a plain cue", "WEBVTT\n\n" + cue + "Hello\n"},
		{"byte order mark, CRLF, a cue identifier and cue settings", "\uFEFFWEBVTT\r\n\r\n1\r\n00:01.000 --> 00:02.500 align:start line:0\r\nHi\r\n"},
		{"hours of 1 to 3 digits", "WEBVTT\n\n1:00:00.000 --> 123:00:00.000\nlong\n"},
		// Tags.
		{"italic, bold and colours kept; other tags become a space", "WEBVTT\n\n" + cue + "<b>Bold</b> <i>it</i> <u>u</u> <c.yellow>c</c> <c.white.bg_black>w</c> <v Roger>voice</v> <lang en>x</lang> <ruby>kan<rt>ji</rt></ruby>\n"},
		{"every text colour and every bg_ colour", "WEBVTT\n\n" + cue + "<c.white>a</c><c.lime>b</c><c.cyan>c</c><c.red>d</c><c.yellow>e</c><c.magenta>f</c><c.blue>g</c><c.black>h</c>\n<c.bg_white>a</c><c.bg_lime>b</c><c.bg_cyan>c</c><c.bg_red>d</c><c.bg_yellow>e</c><c.bg_magenta>f</c><c.bg_blue>g</c><c.bg_black>h</c>\n"},
		{"a background then a text colour", "WEBVTT\n\n" + cue + "<c.bg_black.yellow>x</c>\n"},
		{"two text colours, three classes, an unknown class, <c> alone: spaces", "WEBVTT\n\n" + cue + "a<c.red.blue>b</c>c<c.red.bg_black.lime>d</c>e<c.orange>f</c>g<c>h</c>i<c.>j\n"},
		{"tags with attributes or another case: spaces", "WEBVTT\n\n" + cue + "<i class=x>a</i> <B>b</B> <i >c</i>\n"},
		{"an end tag with no start tag is removed", "WEBVTT\n\n" + cue + "a</i>b</c>c\n"},
		{"an end tag that isn't the innermost is removed; tags closed at the end", "WEBVTT\n\n" + cue + "<i><b>a</i>b\n"},
		{"tags balanced across a cue's lines", "WEBVTT\n\n" + cue + "<i>one\ntwo</i> three\n"},
		{"8 open tags kept", "WEBVTT\n\n" + cue + strings.Repeat("<i>", 8) + "x" + strings.Repeat("</i>", 8) + "\n"},
		{"a 9th open tag removed with its end tag", "WEBVTT\n\n" + cue + strings.Repeat("<b>", 8) + "<i>x</i>y" + strings.Repeat("</b>", 8) + "\n"},
		{"32 tags on a line kept", "WEBVTT\n\n" + cue + strings.Repeat("<i>a</i>", 16) + "\n"},
		{"a 33rd tag on a line removed", "WEBVTT\n\n" + cue + strings.Repeat("<i>a</i>", 16) + "<b>b</b>\n"},
		{"a line of tags alone is dropped with its tags", "WEBVTT\n\n" + cue + "<i></i>\nkept\n"},
		{"spaces collapsed across tags, written where the run starts", "WEBVTT\n\n" + cue + "  a <i> b </i>  c  \n"},
		{"spaces at the end removed, tags after them or not", "WEBVTT\n\n" + cue + "a <i> </i> \nb <b>\n"},
		{"an unknown tag", "WEBVTT\n\n" + cue + "<script>alert(1)</script>Text\n"},
		{"a tag without its >", "WEBVTT\n\n" + cue + "before <b class=x and the rest\n"},
		{"character references decoded, then tags removed again", "WEBVTT\n\n" + cue + "Tom &amp; Jerry &lt;b&gt;x&lt;/b&gt; &#x3c;i&#62; &eacute;\n"},
		{"a decoded tag without its > removes the rest of its text", "WEBVTT\n\n" + cue + "a &lt;b c<i>d</i>\n"},
		{"a lone < stays, escaped", "WEBVTT\n\n" + cue + "a < b, <3 and a > c\n"},
		{"STYLE, NOTE and REGION blocks dropped", "WEBVTT\n\nSTYLE\n::cue { color: red; background: url(https://example.org/x.png) }\n\nNOTE a note\n\nREGION\nid:r1\n\n" + cue + "x\n"},
		{"a timestamp tag stays as text", "WEBVTT\n\n" + cue + "<00:00:01.500>karaoke\n"},
		{"a URL stays as text", "WEBVTT\n\n" + cue + "see https://example.org/x\n"},
		{"direction controls removed", "WEBVTT\n\n" + cue + "\u202Eabc\u2066d\u200F\n"},
		{"white space and controls collapsed", "WEBVTT\n\n" + cue + "  a\tb\u00A0c\u0007d  \n"},
		// Settings.
		{"every setting kept, written in order", "WEBVTT\n\n00:00:01.000 --> 00:00:02.000 align:end size:50% position:10%,line-left line:-1,end\nx\n"},
		{"line at -1,000 and 1,000; percentages at 0 and 100", "WEBVTT\n\n00:00:01.000 --> 00:00:02.000 line:-1000 position:0% size:100%\nx\n\n00:00:03.000 --> 00:00:04.000 line:1000,start position:100.000%,center size:0.000%\ny\n"},
		{"line at -1,001 and 1,001 dropped", "WEBVTT\n\n00:00:01.000 --> 00:00:02.000 line:-1001\nx\n\n00:00:03.000 --> 00:00:04.000 line:1001\ny\n"},
		{"a percentage over 100, of 4 digits, of 4 decimals, without digits: dropped", "WEBVTT\n\n00:00:01.000 --> 00:00:02.000 size:100.001% position:0100% line:50.0001%\nx\n\n00:00:03.000 --> 00:00:04.000 size:.5% position:5.% line:%\ny\n"},
		{"a line percentage with a position alignment, and other bad values: dropped", "WEBVTT\n\n00:00:01.000 --> 00:00:02.000 line:50%,line-left position:50%,start align:middle size:50\nx\n"},
		{"only the first setting of a name is kept", "WEBVTT\n\n00:00:01.000 --> 00:00:02.000 align:start align:end\nx\n"},
		{"a bad first setting of a name: the second is dropped too", "WEBVTT\n\n00:00:01.000 --> 00:00:02.000 align:middle align:end\nx\n"},
		{"region, vertical and unknown settings dropped", "WEBVTT\n\n00:00:01.000 --> 00:00:02.000 region:r1 vertical:rl foo:bar line\tsize:10%\fposition:20%\nx\n"},
		{"a setting in another case is dropped", "WEBVTT\n\n00:00:01.000 --> 00:00:02.000 Align:start LINE:0\nx\n"},
		// Blocks.
		{"a malformed timing: the block dropped", "WEBVTT\n\n00:00:01 --> 00:00:02\nno milliseconds\n\n" + cue + "kept\n"},
		{"a timing without spaces around -->: the block dropped", "WEBVTT\n\n00:00:01.000-->00:00:02.000\nx\n"},
		{"4 digits of hours: the block dropped", "WEBVTT\n\n1000:00:00.000 --> 1000:00:01.000\nx\n"},
		{"a cue left empty: dropped", "WEBVTT\n\n" + cue + "<b></b>\n\n" + cue + "kept\n"},
		{"a line shaped like a timing, inside a cue, is text", "WEBVTT\n\n" + cue + "one\n00:00:03.000 --> 00:00:04.000\n"},
		{"a cue right after the header, without an empty line", "WEBVTT\n" + cue + "x\n"},
		{"a line of spaces doesn't end a block", "WEBVTT\n\n" + cue + "a\n   \nb\n"},
		{"out-of-range timings are not checked", "WEBVTT\n\n00:99.999 --> 00:01.000\nx\n"},
		// Length.
		{"a line of 500 bytes kept whole", "WEBVTT\n\n" + cue + strings.Repeat("a", 500) + "\n"},
		{"a line cut to 500 bytes, not splitting a character", "WEBVTT\n\n" + cue + strings.Repeat("a", 499) + "é" + "b\n"},
		{"tags don't count in the 500 bytes; tags past the cut removed", "WEBVTT\n\n" + cue + "<i>" + strings.Repeat("a", 500) + "</i>b<b>c</b>\n"},
		{"a cut at a space: the space removed", "WEBVTT\n\n" + cue + strings.Repeat("a", 499) + " bc\n"},
		{"escaping comes after the cut", "WEBVTT\n\n" + cue + strings.Repeat("&amp;", 500) + "\n"},
		// The part.
		{"no cue left: valid", "WEBVTT\n\nNOTE only a note\n"},
		{"the header is a prefix", "WEBVTT - a title\n\n" + cue + "x\n"},
		{"no WEBVTT header", cue + "x\n"},
		{"empty", ""},
	}
	var out []vttCase
	for _, c := range in {
		out = append(out, vttCaseOf(c.name, []byte(c.text), rebuildVTT))
	}
	out = append(out, vttCaseOf("invalid UTF-8 removed", []byte("WEBVTT\n\n"+cue+"ab\xffcd\xc3\n"), rebuildVTT))
	return out
}

func vttCaseOf(name string, in []byte, rebuild func([]byte) ([]byte, error)) vttCase {
	c := vttCase{Name: name}
	if utf8.Valid(in) {
		c.Text = string(in)
	} else {
		c.Hex = hex.EncodeToString(in)
	}
	got, err := rebuild(in)
	c.Valid, c.Reason = reason(err)
	c.Output = string(got)
	return c
}

// assScript is a script with the given styles and events, under the
// standard formats.
func assScript(info, styles, events string) string {
	return "[Script Info]\nScriptType: v4.00+\n" + info + "\n[V4+ Styles]\nFormat: Name, Fontname, Fontsize, PrimaryColour, SecondaryColour, OutlineColour, BackColour, Bold, Italic, Underline, StrikeOut, ScaleX, ScaleY, Spacing, Angle, BorderStyle, Outline, Shadow, Alignment, MarginL, MarginR, MarginV, Encoding\n" +
		styles + "\n[Events]\nFormat: Layer, Start, End, Style, Name, MarginL, MarginR, MarginV, Effect, Text\n" + events
}

const assStyleLine = "Style: Main,Arial,48,&H00FFFFFF,&H000000FF,&H00000000,&H64000000,-1,0,0,0,100,100,0,0,1,2,1,2,20,20,30,1\n"

// assLine is an event of style Main with the given text.
func assLine(text string) string {
	return "Dialogue: 0,0:00:01.00,0:00:02.00,Main,,0,0,0,," + text + "\n"
}

// assOne is a script with style Main and one event.
func assOne(text string) string { return assScript("", assStyleLine, assLine(text)) }

func assCases() []vttCase {
	full := "\uFEFF[Script Info]\r\n; a comment\r\n!: another\r\nTitle: a title\r\nScriptType: v4.00+\r\nPlayResX: 1920\r\nplayresy: 1080\r\nWrapStyle: 0\r\nScaledBorderAndShadow: yes\r\nYCbCr Matrix: TV.709\r\n\r\n" +
		"[V4+ Styles]\r\nFormat: Name, Fontname, Fontsize, PrimaryColour, SecondaryColour, OutlineColour, BackColour, Bold, Italic, Underline, StrikeOut, ScaleX, ScaleY, Spacing, Angle, BorderStyle, Outline, Shadow, Alignment, MarginL, MarginR, MarginV, Encoding\r\n" +
		"Style: Default,Arial,48,&H00FFFFFF,&H000000FF,&H00000000,&H64000000,-1,0,0,0,100,100,0,0,1,2,1,2,20,20,30,1\r\n\r\n" +
		"[Events]\r\nFormat: Layer, Start, End, Style, Name, MarginL, MarginR, MarginV, Effect, Text\r\n" +
		"Comment: 0,0:00:00.00,0:00:01.00,Default,,0,0,0,,a comment\r\n" +
		"Dialogue: 0,0:00:01.00,0:00:02.50,Default,Roger,0000,0000,0000,karaoke,{\\i1}Hello{\\i0}, world\\Nsecond line\r\n"
	manyStyles := func(n int) string {
		var b strings.Builder
		b.WriteString("[Script Info]\n[V4+ Styles]\nFormat: Name\n")
		for i := range n {
			fmt.Fprintf(&b, "Style: s%d\n", i)
		}
		return b.String()
	}
	in := []struct{ name, text string }{
		{"a script with every section: header, comments, Title and ScriptType dropped", full},
		{"a minimal script: the Default style written", "[Script Info]\n"},
		{"no [Script Info] first", "[V4+ Styles]\n[Script Info]\n"},
		{"[script info] in another case first: refused", "[script info]\n"},
		{"empty", ""},
		{"unknown sections dropped: fonts and pictures", "[Script Info]\nPlayResX: 640\n[Fonts]\nfontname: a.ttf\nM4!%-\n[Graphics]\nfilename: b.png\n[Aegisub Project Garbage]\nVideo File: x.mkv\n[events]\nFormat: Layer, Start, End, Style, Name, MarginL, MarginR, MarginV, Effect, Text\nDialogue: 0,0:00:01.00,0:00:02.00,Default,,0,0,0,,x\n"},
		// Script information.
		{"script information at its bounds", "[Script Info]\nPlayResX: 1\nPlayResY: 16384\nLayoutResX: 1\nLayoutResY: 16384\nWrapStyle: 3\nScaledBorderAndShadow: no\nCollisions: Reverse\nKerning: yes\nYCbCr Matrix: PC.240M\n"},
		{"script information out of bounds dropped", "[Script Info]\nPlayResX: 0\nPlayResY: 16385\nLayoutResX: -1\nLayoutResY: 1.5\nWrapStyle: 4\nScaledBorderAndShadow: Yes\nCollisions: normal\nKerning: 1\nYCbCr Matrix: TV.2020\n"},
		{"only the first of a name, valid or not", "[Script Info]\nPlayResX: 640\nPlayResX: 1280\nPlayResY: abc\nPlayResY: 720\n"},
		{"a line without : and a name with a space before : dropped", "[Script Info]\nPlayResX 640\nPlayResY : 480\n"},
		// Styles.
		{"a style at its bounds", assScript("", "Style: Max,@Font,1000,&HFFFFFFFF,4294967295,&H0,&Hab&,1000,-1,-1,-1,1000,1000,-1000,-360,4,100,100,9,16384,16384,16384,255\nStyle: Min,F,0,0,0,0,0,-1,0,0,0,0,0,1000,360,3,0,0,1,0,0,0,0\n", "")},
		{"a style past its bounds: defaults", assScript("", "Style: Bad,"+strings.Repeat("f", 65)+",1000.5,&H123456789,4294967296,&HXY,red,1001,5,x,-2,1000.0001,-1,1001,361,2,100.5,-0.5,0,16385,-1,1.5,256\n", "")},
		{"a style named 64 bytes kept, 65 dropped", assScript("", "Style: "+strings.Repeat("n", 64)+",Arial,20,0,0,0,0,0,0,0,0,100,100,0,0,1,2,2,2,10,10,10,1\nStyle: "+strings.Repeat("m", 65)+",Arial,20,0,0,0,0,0,0,0,0,100,100,0,0,1,2,2,2,10,10,10,1\n", "")},
		{"a style with no name dropped; two of one name, the first kept", assScript("", "Style: ,Arial,20,0,0,0,0,0,0,0,0,100,100,0,0,1,2,2,2,10,10,10,1\nStyle: A,One,20,0,0,0,0,0,0,0,0,100,100,0,0,1,2,2,2,10,10,10,1\nStyle: A,Two,20,0,0,0,0,0,0,0,0,100,100,0,0,1,2,2,2,10,10,10,1\n", "")},
		{"a style with too few or too many fields dropped", assScript("", "Style: A,Arial,20\nStyle: B,Arial,20,0,0,0,0,0,0,0,0,100,100,0,0,1,2,2,2,10,10,10,1,extra\n", "")},
		{"numbers: 7 digits and 3 decimals; colours of 1 to 8 digits", assScript("", "Style: N,Arial,0000020.125,&H1,&H12345678&,&Hff,16777215,0,0,0,0,100,100,0.5,-0.25,1,1.5,0,2,10,10,10,1\nStyle: O,Arial,00000020,&H,H123,&H123456789,-1,0,0,0,0,100,100,1.2345,0,1,2,2,2,10,10,10,1\n", "")},
		{"a Format of a few fields, others unknown: defaults", "[Script Info]\n[V4+ Styles]\nFormat: Fontsize, Name, Foo\nStyle: 30, Spaced , x\n"},
		{"text made plain", "[Script Info]\n[V4+ Styles]\nFormat: Name, Fontname\nStyle: A\u0007B,Font\u0001Name\n"},
		{"an SSA script: [V4 Styles], TertiaryColour, AlphaLevel, alignments, Marked", "[Script Info]\nScriptType: v4.00\n[V4 Styles]\nFormat: Name, Fontname, Fontsize, PrimaryColour, SecondaryColour, TertiaryColour, BackColour, Bold, Italic, BorderStyle, Outline, Shadow, Alignment, MarginL, MarginR, MarginV, AlphaLevel, Encoding\nStyle: S1,Arial,20,16777215,65535,255,0,-1,0,1,2,2,1,10,10,10,0,0\nStyle: S5,Arial,20,16777215,65535,255,0,-1,0,1,2,2,5,10,10,10,0,0\nStyle: S7,Arial,20,16777215,65535,255,0,-1,0,1,2,2,7,10,10,10,0,0\nStyle: S9,Arial,20,16777215,65535,255,0,-1,0,1,2,2,9,10,10,10,0,0\nStyle: S11,Arial,20,16777215,65535,255,0,-1,0,1,2,2,11,10,10,10,0,0\nStyle: S4,Arial,20,16777215,65535,255,0,-1,0,1,2,2,4,10,10,10,0,0\n[Events]\nFormat: Marked, Start, End, Style, Name, MarginL, MarginR, MarginV, Effect, Text\nDialogue: Marked=1,0:00:01.00,0:00:02.00,S5,,0,0,0,,{\\a6}x\n"},
		{"SSA default formats, no Format lines", "[Script Info]\n[V4 Styles]\nStyle: S,Arial,20,16777215,65535,255,0,-1,0,1,2,2,10,10,10,10,0,0\n[Events]\nDialogue: Marked=0,0:00:01.00,0:00:02.00,S,,0,0,0,,x\n"},
		{"1,024 styles kept, a 1,025th dropped", manyStyles(1025)},
		// Events.
		{"event fields: layer, times, margins at their bounds", assScript("", assStyleLine, "Dialogue: 1000,999:59:59.99,0:00:00.00,Main,Speaker,16384,0,16384,Banner;10,x\nDialogue: 1001,0:00:01.00,0:00:02.00,Nope,,16385,-1,a,,y\n")},
		{"bad times drop the event", assScript("", assStyleLine, "Dialogue: 0,1000:00:00.00,0:00:02.00,Main,,0,0,0,,a\nDialogue: 0,0:00:01.000,0:00:02.00,Main,,0,0,0,,b\nDialogue: 0,0:0:01.00,0:00:02.00,Main,,0,0,0,,c\nDialogue: 0,0:00:01.00,0:00:02.00,Main,,0,0,0,,kept\n")},
		{"only Dialogue lines kept", assScript("", assStyleLine, "Comment: 0,0:00:01.00,0:00:02.00,Main,,0,0,0,,a\nPicture: 0,0:00:01.00,0:00:02.00,Main,,0,0,0,,b.png\nSound: 0,0:00:01.00,0:00:02.00,Main,,0,0,0,,c.wav\nMovie: 0,0:00:01.00,0:00:02.00,Main,,0,0,0,,d.avi\nCommand: 0,0:00:01.00,0:00:02.00,Main,,0,0,0,,e\ndialogue: 0,0:00:01.00,0:00:02.00,Main,,0,0,0,,f\n")},
		{"commas in the text; too few fields drops the event", assScript("", assStyleLine, "Dialogue: 0,0:00:01.00,0:00:02.00,Main,,0,0,0,,a, b, c\nDialogue: 0,0:00:01.00,0:00:02.00,Main\n")},
		{"a Format of its own order", "[Script Info]\n[Events]\nFormat: Start, End, Text\nDialogue: 0:00:01.00,0:00:02.00,x, y\n"},
		{"an event left empty dropped", assOne("{\\comment only}")},
		{"text kept as it is outside braces", assOne("a \\N b \\n c \\h d } e \\x")},
		{"controls removed, a tab becomes a space, direction controls removed", assOne("a\tb\u0007c\u202Ed\uFFFDe")},
		{"a block without its } removes the rest", assOne("a{\\b1 b")},
		{"comments in a block dropped; an empty block dropped", assOne("{a comment\\b1}x{}y{no tag}z")},
		{"unknown tags and bad arguments dropped", assOne("{\\xyz\\b1001\\i2\\fs-1\\bord101\\an0\\q4\\k-1\\fe256\\shad101\\b1}x")},
		// Tags.
		{"tags at their bounds kept", assOne("{\\b0\\b1000\\i1\\u0\\s1\\bord100\\xbord0\\ybord0.5\\be100\\blur100\\shad-100\\xshad100\\yshad0\\fs1000\\fscx0\\fscy1000\\fsp-1000\\frx-3600\\fry3600\\frz0\\fr3600\\fax-100\\fay100\\fe255\\an9\\q3\\k100000\\K0\\kf0\\ko100000\\pbo-100000}x")},
		{"tags just past their bounds dropped", assOne("{\\b-1\\bord100.001\\shad-100.5\\fs1000.1\\fsp1001\\frz3601\\fax-101\\fe-1\\k100001\\pbo100001\\b1}x")},
		{"the longest tag name matches", assOne("{\\bord2\\blur1\\be1\\fscx50\\fsp2\\frz10\\fad(100,200)\\fade(255,0,255,0,100,200,300)\\iclip(0,0,10,10)\\xbord1}x")},
		{"colours and alpha normalised", assOne("{\\c&Hff&\\1c&H123456\\2c&HABCDEF&\\3c&H0&\\4c&Ha\\alpha&H8&\\1a&Hff\\2a&H0&\\3a&H7f\\4a&H10&}x")},
		{"colours and alpha out of form dropped", assOne("{\\c&H1234567&\\c&HXYZ&\\cFF\\alpha&H100&\\1a&H\\c&H1&}x")},
		{"\\fn of 64 bytes kept, 65 dropped; made plain", assOne("{\\fn" + strings.Repeat("f", 64) + "}a{\\fn" + strings.Repeat("g", 65) + "}b{\\fnA,B\u0001C}c{\\fn}d")},
		{"\\r with nothing, a kept style, an unknown style", assOne("{\\r}a{\\rMain}b{\\rNope}c")},
		{"SSA's \\a converted to \\an", assOne("{\\a1}a{\\a5}b{\\a7}c{\\a9}d{\\a11}e{\\a4}f{\\a8}g")},
		{"\\pos, \\org and \\move at their bounds, white space removed", assOne("{\\pos(-100000, 100000)\\org( 0.5 ,-0.125 )\\move(1,2,3,4)\\move(1,2,3,4,0,3600000)}x")},
		{"\\pos, \\org and \\move out of bounds or form dropped", assOne("{\\pos(100001,0)\\pos(1)\\pos(1,2,3)\\pos 1,2\\org(a,b)\\move(1,2,3)\\move(1,2,3,4,5)\\move(1,2,3,4,0,3600001)\\pos(1,2)x\\b1}x")},
		{"\\fad and \\fade at and past their bounds", assOne("{\\fad(0,3600000)\\fade(0,255,0,0,1,2,3600000)}a{\\fad(-1,0)\\fad(0,3600001)\\fade(256,0,0,0,0,0,0)\\fade(0,0,0,0,0,0)\\b1}b")},
		{"\\clip and \\iclip as rectangles and drawings", assOne("{\\clip(0,0,1920,1080)\\iclip(-100000,0,100000,1)\\clip(m 0 0 l 100 0 100 100)\\iclip(16,m 0 0 l 1 1)}x")},
		{"\\clip out of bounds or form dropped", assOne("{\\clip(0,0,0)\\clip(0,0,0,100001)\\clip(17,m 0 0)\\clip(0,m 0 0)\\clip(junk)\\clip(1,2,3,4,5)\\b1}x")},
		{"drawings: commands and numbers kept, anything else dropped", assOne("{\\p1}m 0 0 l 100.5 0 junk 100 100 b 1 2 3 4 5 6 s n p c 100001 x 1e5{\\p0}text")},
		{"\\p at 16 kept, 17 dropped", assOne("{\\p16}m 0 0{\\p0}a{\\p17}b")},
		{"\\t with every form of arguments", assOne("{\\t(\\fs20)\\t(0.5,\\bord2)\\t(0,1000,\\frz90\\c&HFF&)\\t(0,3600000,100,\\alpha&H80&\\clip(0,0,10,10))}x")},
		{"\\t with bad arguments or excluded tags dropped", assOne("{\\t(0,3600001,\\fs20)\\t(100.5,\\fs20)\\t(0,1,2,3,\\fs20)\\t(\\pos(1,2)\\an5\\k10\\fnA\\r\\p1\\pbo1\\t(\\fs1))\\t(0,1000)\\t(0,1000\\fs20)\\b1}x")},
		{"\\t keeps its checked tags only", assOne("{\\t(\\fs20\\pos(1,2)\\xyz\\b1)}x")},
		{"16 \\t an event kept, a 17th dropped", assOne("{" + strings.Repeat("\\t(\\fs1)", 16) + "}a{\\t(\\fs2)\\b1}b")},
		{"256 blocks kept, a 257th dropped", assOne(strings.Repeat("{\\b1}a", 256) + "{\\b0}b{\\i1}c")},
		{"1,024 tags kept; a block past them dropped", assOne(strings.Repeat("{"+strings.Repeat("\\b1", 256)+"}a", 4) + "{\\i1}b")},
		{"a block that would pass 1,024 tags dropped, and every block after it", assOne("{" + strings.Repeat("\\b1", 1020) + "}a{" + strings.Repeat("\\i1", 5) + "}b{\\u1}c")},
		{"10,000 numbers a drawing kept; past them dropped", assOne("{\\p1}m" + strings.Repeat(" 1", 10001) + " l 2{\\p0}")},
		{"an event of 65,536 bytes kept", assOne(strings.Repeat("a", 65536))},
		{"an event of 65,537 bytes dropped", assOne(strings.Repeat("a", 65537))},
		{"a style not kept: Default", assOne("{\\rMain}x") + "Dialogue: 0,0:00:01.00,0:00:02.00,Other,,0,0,0,,y\n"},
		{"no event left: valid", assScript("", assStyleLine, "")},
	}
	var out []vttCase
	for _, c := range in {
		out = append(out, vttCaseOf(c.name, []byte(c.text), rebuildASS))
	}
	out = append(out, vttCaseOf("invalid UTF-8 removed from text and style names", []byte(assScript("", "Style: A\xffB,Arial,20,0,0,0,0,0,0,0,0,100,100,0,0,1,2,2,2,10,10,10,1\n", "Dialogue: 0,0:00:01.00,0:00:02.00,AB,,0,0,0,,x\xc3y\n")), rebuildASS))
	return out
}

// mediaPlaylist is a complete media playlist around segment lines.
func mediaPlaylist(body string) string {
	return "#EXTM3U\n#EXT-X-VERSION:7\n#EXT-X-TARGETDURATION:6\n#EXT-X-PLAYLIST-TYPE:VOD\n" + body + "#EXT-X-ENDLIST\n"
}

// masterPlaylist is a master playlist with the given renditions.
func masterPlaylist(media string) string {
	return "#EXTM3U\n#EXT-X-VERSION:7\n" + media + "#EXT-X-STREAM-INF:BANDWIDTH=5000000,CODECS=\"avc1.640028,mp4a.40.2\",RESOLUTION=1920x1080,AUDIO=\"audio\"\nv0.m3u8\n"
}

func rendition(id string) string {
	return "#EXT-X-MEDIA:TYPE=AUDIO,GROUP-ID=\"audio\",NAME=\"" + id + "\",DEFAULT=NO,AUTOSELECT=YES,URI=\"" + id + ".m3u8\"\n"
}

func renditions(n int) string {
	var b strings.Builder
	for i := 1; i <= n; i++ {
		b.WriteString(rendition(fmt.Sprintf("a%d", i)))
	}
	return b.String()
}

func hlsVerdicts() []verdict {
	cases := []struct{ name, text string }{
		{"a master playlist", "#EXTM3U\n#EXT-X-VERSION:7\n#EXT-X-INDEPENDENT-SEGMENTS\n#EXT-X-STREAM-INF:BANDWIDTH=5000000,CODECS=\"avc1.640028,mp4a.40.2\",RESOLUTION=1920x1080\nv0.m3u8\n"},
		{"a media playlist", "#EXTM3U\n#EXT-X-VERSION:7\n#EXT-X-TARGETDURATION:6\n#EXT-X-MEDIA-SEQUENCE:0\n#EXT-X-PLAYLIST-TYPE:VOD\n#EXT-X-MAP:URI=\"init.mp4\"\n#EXTINF:6.000000, title\nseg0.mp4\n#EXT-X-DISCONTINUITY\n#EXTINF:6.000000,\nseg-1.mp4\n#EXT-X-ENDLIST\n"},
		{"CRLF line ends", "#EXTM3U\r\n#EXT-X-TARGETDURATION:6\r\n#EXT-X-PLAYLIST-TYPE:VOD\r\n#EXTINF:6,\r\nseg0.ts\r\n#EXT-X-ENDLIST\r\n"},
		{"an unknown tag without a URI, and a comment: may pass", mediaPlaylist("#EXT-X-FOO:BAR=1\n# a comment\n#EXTINF:6,\nseg0.ts\n")},
		// Steps 3 to 8.
		{"no #EXTM3U", "#EXTINF:6,\nseg0.ts\n"},
		{"a first line that only starts with #EXTM3U", "#EXTM3UX\n#EXTINF:6,\nseg0.ts\n"},
		{"#EXTM3U after a blank line", "\n#EXTM3U\n"},
		{"an absolute URI line", mediaPlaylist("#EXTINF:6,\nhttps://example.org/seg0.ts\n")},
		{"a path in a URI line", mediaPlaylist("#EXTINF:6,\nhls1/main/0.ts\n")},
		{"a dot segment as a URI line", mediaPlaylist("#EXTINF:6,\n..\n")},
		{"a query in a URI line", mediaPlaylist("#EXTINF:6,\nseg0.ts?x=1\n")},
		{"percent-encoding in a URI line", mediaPlaylist("#EXTINF:6,\nseg%2e.ts\n")},
		{"a space after a URI line", mediaPlaylist("#EXTINF:6,\nseg0.ts \n")},
		{"an unquoted URI attribute", mediaPlaylist("#EXT-X-MAP:URI=init.mp4\n#EXTINF:6,\nseg0.ts\n")},
		{"an empty URI attribute", mediaPlaylist("#EXT-X-MAP:URI=\"\"\n#EXTINF:6,\nseg0.ts\n")},
		{"a URI attribute without its closing quote", mediaPlaylist("#EXT-X-MAP:URI=\"init.mp4\n#EXTINF:6,\nseg0.ts\n")},
		{"an absolute URI in EXT-X-MEDIA", masterPlaylist("#EXT-X-MEDIA:TYPE=AUDIO,GROUP-ID=\"audio\",NAME=\"a1\",URI=\"https://example.org/a.m3u8\"\n")},
		{"an absolute URI in an I-frame playlist", masterPlaylist("#EXT-X-I-FRAME-STREAM-INF:BANDWIDTH=1,URI=\"https://example.org/i.m3u8\"\n")},
		{"uri= in lower case, in a comment", mediaPlaylist("# uri=\"a/b\"\n#EXTINF:6,\nseg0.ts\n")},
		{"URI= inside another attribute's quoted string", masterPlaylist("#EXT-X-SESSION-DATA:DATA-ID=\"x\",VALUE=\"URI=a/b\"\n")},
		{"EXT-X-DEFINE", mediaPlaylist("#EXT-X-DEFINE:NAME=\"h\",VALUE=\"example.org\"\n#EXTINF:6,\nseg0.ts\n")},
		{"EXT-X-KEY with a bare name", mediaPlaylist("#EXT-X-KEY:METHOD=AES-128,URI=\"key\"\n#EXTINF:6,\nseg0.ts\n")},
		{"EXT-X-KEY with METHOD=NONE", mediaPlaylist("#EXT-X-KEY:METHOD=NONE\n#EXTINF:6,\nseg0.ts\n")},
		{"EXT-X-SESSION-KEY", masterPlaylist("#EXT-X-SESSION-KEY:METHOD=SAMPLE-AES,URI=\"k\"\n")},
		{"a step 4 failure is reported before a step 6 one", "#EXTM3U\n#EXT-X-DEFINE:NAME=\"h\"\nseg/0.ts\n"},
		// Step 9.
		{"neither EXT-X-STREAM-INF nor EXTINF", "#EXTM3U\n#EXT-X-VERSION:7\n"},
		{"only #EXTM3U", "#EXTM3U\n"},
		{"a media playlist without EXT-X-ENDLIST: growing", "#EXTM3U\n#EXT-X-TARGETDURATION:6\n#EXT-X-PLAYLIST-TYPE:VOD\n#EXTINF:6,\nseg0.ts\n"},
		{"a media playlist of type EVENT: growing", "#EXTM3U\n#EXT-X-TARGETDURATION:6\n#EXT-X-PLAYLIST-TYPE:EVENT\n#EXTINF:6,\nseg0.ts\n#EXT-X-ENDLIST\n"},
		{"a media playlist without EXT-X-PLAYLIST-TYPE", "#EXTM3U\n#EXT-X-TARGETDURATION:6\n#EXTINF:6,\nseg0.ts\n#EXT-X-ENDLIST\n"},
		{"two EXT-X-PLAYLIST-TYPE:VOD", mediaPlaylist("#EXT-X-PLAYLIST-TYPE:VOD\n#EXTINF:6,\nseg0.ts\n")},
		{"two EXT-X-ENDLIST", mediaPlaylist("#EXTINF:6,\nseg0.ts\n#EXT-X-ENDLIST\n")},
		{"EXT-X-ENDLIST before the last URI line", "#EXTM3U\n#EXT-X-TARGETDURATION:6\n#EXT-X-PLAYLIST-TYPE:VOD\n#EXTINF:6,\nseg0.ts\n#EXT-X-ENDLIST\n#EXTINF:6,\nseg1.ts\n"},
		{"no EXT-X-TARGETDURATION", "#EXTM3U\n#EXT-X-PLAYLIST-TYPE:VOD\n#EXTINF:6,\nseg0.ts\n#EXT-X-ENDLIST\n"},
		{"a target duration of 1 and 60", "#EXTM3U\n#EXT-X-TARGETDURATION:1\n#EXT-X-PLAYLIST-TYPE:VOD\n#EXTINF:1.499,\nseg0.ts\n#EXT-X-ENDLIST\n#EXT-X-TARGETDURATION:60\n"},
		{"a target duration of 0", "#EXTM3U\n#EXT-X-TARGETDURATION:0\n#EXT-X-PLAYLIST-TYPE:VOD\n#EXTINF:0.4,\nseg0.ts\n#EXT-X-ENDLIST\n"},
		{"a target duration of 61", "#EXTM3U\n#EXT-X-TARGETDURATION:61\n#EXT-X-PLAYLIST-TYPE:VOD\n#EXTINF:6,\nseg0.ts\n#EXT-X-ENDLIST\n"},
		{"a target duration that isn't an integer", "#EXTM3U\n#EXT-X-TARGETDURATION:6.0\n#EXT-X-PLAYLIST-TYPE:VOD\n#EXTINF:6,\nseg0.ts\n#EXT-X-ENDLIST\n"},
		{"EXTINF that rounds to the target duration", mediaPlaylist("#EXTINF:6.499,\nseg0.ts\n#EXTINF:0.001,\nseg1.ts\n")},
		{"EXTINF that rounds above the target duration", mediaPlaylist("#EXTINF:6.5,\nseg0.ts\n")},
		{"EXTINF of 0", mediaPlaylist("#EXTINF:0,\nseg0.ts\n")},
		{"EXTINF of 0.000", mediaPlaylist("#EXTINF:0.000,\nseg0.ts\n")},
		{"EXTINF negative", mediaPlaylist("#EXTINF:-1,\nseg0.ts\n")},
		{"EXTINF that isn't a decimal", mediaPlaylist("#EXTINF:6s,\nseg0.ts\n")},
		{"EXTINF without a comma", mediaPlaylist("#EXTINF:6\nseg0.ts\n")},
		// Step 10.
		{"a master playlist with an audio rendition", masterPlaylist(rendition("a1"))},
		{"a PQ stream: VIDEO-RANGE", "#EXTM3U\n#EXT-X-VERSION:7\n#EXT-X-STREAM-INF:BANDWIDTH=20000000,CODECS=\"hvc1.2.4.L153.B0,ec-3\",RESOLUTION=3840x2160,VIDEO-RANGE=PQ\nv0.m3u8\n"},
		{"Dolby Vision kept: SUPPLEMENTAL-CODECS", "#EXTM3U\n#EXT-X-VERSION:10\n#EXT-X-STREAM-INF:BANDWIDTH=20000000,CODECS=\"hvc1.2.4.L153.B0,mp4a.40.2\",SUPPLEMENTAL-CODECS=\"dvh1.08.06/db1p\",RESOLUTION=3840x2160,VIDEO-RANGE=PQ\nv0.m3u8\n"},
		{"an Atmos rendition: CHANNELS 16/JOC", masterPlaylist("#EXT-X-MEDIA:TYPE=AUDIO,GROUP-ID=\"audio\",NAME=\"a1\",LANGUAGE=\"en\",DEFAULT=YES,AUTOSELECT=YES,CHANNELS=\"16/JOC\",URI=\"a1.m3u8\"\n")},
		{"an audio-only stream: no RESOLUTION", "#EXTM3U\n#EXT-X-VERSION:7\n#EXT-X-STREAM-INF:BANDWIDTH=192000,CODECS=\"mp4a.40.2\"\nv0.m3u8\n"},
		{"32 audio renditions", masterPlaylist(renditions(32))},
		{"33 audio renditions", masterPlaylist(renditions(33))},
		{"a subtitles rendition", masterPlaylist("#EXT-X-MEDIA:TYPE=SUBTITLES,GROUP-ID=\"subs\",NAME=\"s0\",URI=\"s0.m3u8\"\n")},
		{"a rendition named by a title, not a track ID", masterPlaylist("#EXT-X-MEDIA:TYPE=AUDIO,GROUP-ID=\"audio\",NAME=\"English\",URI=\"a1.m3u8\"\n")},
		{"a rendition of a track the playback hasn't", masterPlaylist(rendition("a41"))},
		{"a rendition named a01", masterPlaylist(rendition("a01"))},
		{"a rendition with an unquoted NAME", masterPlaylist("#EXT-X-MEDIA:TYPE=AUDIO,GROUP-ID=\"audio\",NAME=a1,URI=\"a1.m3u8\"\n")},
		{"a rendition without NAME", masterPlaylist("#EXT-X-MEDIA:TYPE=AUDIO,GROUP-ID=\"audio\",URI=\"a1.m3u8\"\n")},
		{"a rendition with a malformed attribute list", masterPlaylist("#EXT-X-MEDIA:TYPE=AUDIO,,NAME=\"a1\"\n")},
	}
	var out []verdict
	for _, c := range cases {
		ok, r := reason(checkPlaylist([]byte(c.text)))
		out = append(out, verdict{Name: c.name, Text: c.text, Valid: ok, Reason: r})
	}
	return out
}

// pgsSeg is one PGS segment: header (times zero) and payload.
func pgsSeg(typ byte, payload ...byte) []byte {
	b := make([]byte, 13, 13+len(payload))
	b[0], b[1], b[10] = 'P', 'G', typ
	binary.BigEndian.PutUint16(b[11:], uint16(len(payload)))
	return append(b, payload...)
}

func u16(v int) []byte { return []byte{byte(v >> 8), byte(v)} }

func cat(parts ...[]byte) []byte {
	var b []byte
	for _, p := range parts {
		b = append(b, p...)
	}
	return b
}

// pcs is a presentation of a w by h screen with no composition object.
func pcs(w, h int) []byte {
	return pgsSeg(0x16, cat(u16(w), u16(h), []byte{0x10, 0, 0, 0x80, 0, 0, 0})...)
}

// ods is an object segment: id 0, version 0, flags; first fragments carry
// a length (not checked), the size and the data.
func ods(flags byte, w, h int, data ...byte) []byte {
	if flags&0x80 == 0 {
		return pgsSeg(0x15, cat(u16(0), []byte{0, flags}, data)...)
	}
	return pgsSeg(0x15, cat(u16(0), []byte{0, flags, 0, 0, byte(len(data) + 4)}, u16(w), u16(h), data)...)
}

func pgsVerdicts() []verdict {
	end := pgsSeg(0x80)
	wds := pgsSeg(0x17, cat([]byte{1, 0}, u16(0), u16(0), u16(2), u16(2))...)
	pds := pgsSeg(0x14, 0, 0, 1, 235, 128, 128, 255)
	// 2 by 2: two pixels, end of line, twice.
	square := []byte{1, 1, 0, 0, 1, 1, 0, 0}
	cases := []struct {
		name string
		b    []byte
	}{
		{"a display set: presentation, window, palette, object, end", cat(pcs(1920, 1080), wds, pds, ods(0xC0, 2, 2, square...), end)},
		{"an object in two fragments", cat(pcs(1920, 1080), ods(0x80, 2, 2, 1, 1, 0, 0), ods(0x40, 0, 0, 1, 1, 0, 0), end)},
		{"runs: 2 pixels of color 3, then a 14-bit run of 300 of color 0", cat(pcs(1920, 1080), ods(0xC0, 302, 1, 0, 0x82, 3, 0, 0x41, 0x2C, 0, 0), end)},
		{"a presentation with one cropped object", pgsSeg(0x16, cat(u16(1920), u16(1080), []byte{0x10, 0, 0, 0x80, 0, 0, 1}, []byte{0, 0, 0, 0x80, 0, 0, 0, 0}, make([]byte, 8))...)},
		{"an object left open at the end", cat(pcs(1920, 1080), ods(0x80, 2, 2, 1, 1), end)},
		{"a first fragment with an ID already open starts that object again", cat(ods(0x80, 2, 2, 1, 1, 1), ods(0xC0, 2, 2, square...))},
		{"screen and object sizes of 1 and 4,096", cat(pcs(1, 4096), pcs(4096, 1), ods(0xC0, 4096, 1, 1), ods(0xC0, 1, 4096, 1))},
		{"empty: a stretch with no subtitles, no segments", nil},
		{"another magic", append([]byte("PX"), pgsSeg(0x80)[2:]...)},
		{"a header cut short", pgsSeg(0x80)[:12]},
		{"a payload past the end", pgsSeg(0x14, 0, 0, 1, 2, 3, 4, 5)[:15]},
		{"an unknown segment type", pgsSeg(0x18)},
		{"a screen 0 wide", pcs(0, 1080)},
		{"a screen 4,097 high", pcs(1920, 4097)},
		{"a presentation of 10 bytes", pgsSeg(0x16, cat(u16(1920), u16(1080), make([]byte, 6))...)},
		{"a presentation whose object doesn't fit", pgsSeg(0x16, cat(u16(1920), u16(1080), []byte{0x10, 0, 0, 0x80, 0, 0, 1}, make([]byte, 4))...)},
		{"a cropped object without its crop", pgsSeg(0x16, cat(u16(1920), u16(1080), []byte{0x10, 0, 0, 0x80, 0, 0, 1}, []byte{0, 0, 0, 0x80, 0, 0, 0, 0})...)},
		{"a window segment of the wrong size", pgsSeg(0x17, 1, 0, 0, 0)},
		{"a window segment of no byte", pgsSeg(0x17)},
		{"a palette segment of 3 bytes", pgsSeg(0x14, 0, 0, 1)},
		{"an end segment that isn't empty", pgsSeg(0x80, 0)},
		{"an object segment of 3 bytes", pgsSeg(0x15, 0, 0, 0)},
		{"an object 0 wide", ods(0xC0, 0, 2, square...)},
		{"an object 4,097 high", ods(0xC0, 2, 4097, square...)},
		{"a first fragment of 10 bytes", pgsSeg(0x15, 0, 0, 0, 0xC0, 0, 0, 4, 0, 2, 0)},
		{"a fragment of no open object", ods(0x40, 0, 0, square...)},
		{"a line wider than the object", ods(0xC0, 2, 2, 1, 1, 1, 0, 0)},
		{"more lines than the object's height", ods(0xC0, 2, 2, 1, 0, 0, 1, 0, 0, 1, 0, 0)},
		{"a run cut short", ods(0xC0, 2, 2, 1, 0)},
		{"a 14-bit run cut short", ods(0xC0, 2, 2, 0, 0x41)},
		{"a run longer than the line", ods(0xC0, 2, 2, 0, 3)},
	}
	var out []verdict
	for _, c := range cases {
		ok, r := reason(checkPGS(c.b))
		out = append(out, verdict{Name: c.name, Hex: hex.EncodeToString(c.b), Valid: ok, Reason: r})
	}
	return out
}

// mediaFontTables are the tables of a TrueType font with outlines in glyf.
var mediaFontTables = []string{"cmap", "glyf", "head", "hhea", "hmtx", "loca", "maxp", "name", "post"}

// mediaFont is a tiny font header: magic, table directory, then 4 zero
// bytes per table; edit changes the bytes after.
func mediaFont(magic string, tags []string, edit func([]byte) []byte) []byte {
	n := len(tags)
	b := make([]byte, 12+16*n+4*n)
	copy(b, magic)
	binary.BigEndian.PutUint16(b[4:], uint16(n))
	for i, t := range tags {
		r := b[12+16*i:]
		copy(r, t)
		binary.BigEndian.PutUint32(r[8:], uint32(12+16*n+4*i))
		binary.BigEndian.PutUint32(r[12:], 4)
	}
	if edit != nil {
		b = edit(b)
	}
	return b
}

func mediaReplace(tags []string, old, new string) []string {
	var out []string
	for _, t := range tags {
		if t == old {
			if new != "" {
				out = append(out, new)
			}
		} else {
			out = append(out, t)
		}
	}
	return out
}

func fontCases() []mediaFontCase {
	ttf := mediaFontTables
	cff := append(mediaReplace(mediaReplace(ttf, "glyf", "CFF "), "loca", ""), "OS/2")
	cff2 := mediaReplace(cff, "CFF ", "CFF2")
	many := func(n int) []string {
		t := append([]string(nil), ttf...)
		for i := len(t); i < n; i++ {
			t = append(t, fmt.Sprintf("x%03d", i))
		}
		return t
	}
	// record sets a field of table i's directory record.
	record := func(i, at int, v uint32) func([]byte) []byte {
		return func(b []byte) []byte {
			binary.BigEndian.PutUint32(b[12+16*i+at:], v)
			return b
		}
	}
	cases := []struct {
		name string
		b    []byte
	}{
		{"TrueType: 00 01 00 00, glyf and loca", mediaFont("\x00\x01\x00\x00", ttf, nil)},
		{"OpenType with CFF: OTTO", mediaFont("OTTO", cff, nil)},
		{"OpenType with CFF2", mediaFont("OTTO", cff2, nil)},
		{"TrueType magic with CFF outlines", mediaFont("\x00\x01\x00\x00", cff, nil)},
		{"OTTO with glyf and loca", mediaFont("OTTO", ttf, nil)},
		{"64 tables", mediaFont("\x00\x01\x00\x00", many(64), nil)},
		{"a table ending at the file's end", mediaFont("\x00\x01\x00\x00", ttf, record(8, 12, 4))},
		{"a table of no byte at the file's end", mediaFont("\x00\x01\x00\x00", ttf, func(b []byte) []byte { return record(8, 12, 0)(record(8, 8, uint32(len(b)))(b)) })},
		{"tags of spaces and ~", mediaFont("\x00\x01\x00\x00", append(append([]string(nil), ttf...), "    ", "~~~~"), nil)},
		{"empty", nil},
		{"3 bytes", []byte{0, 1, 0}},
		{"a font collection: ttcf", mediaFont("ttcf", ttf, nil)},
		{"WOFF", mediaFont("wOFF", ttf, nil)},
		{"WOFF2", mediaFont("wOF2", ttf, nil)},
		{"Apple's older TrueType signature: true, glyf and loca", mediaFont("true", ttf, nil)},
		{"true in another case", mediaFont("TRUE", ttf, nil)},
		{"typ1", mediaFont("typ1", ttf, nil)},
		{"OTTO in another case", mediaFont("otto", cff, nil)},
		{"no table count", []byte("OTTO\x00")},
		{"0 tables", mediaFont("\x00\x01\x00\x00", nil, nil)},
		{"65 tables", mediaFont("\x00\x01\x00\x00", many(65), nil)},
		{"a table directory past the end", mediaFont("\x00\x01\x00\x00", ttf, func(b []byte) []byte { return b[:12+16*8+8] })},
		{"a table one byte past the end", mediaFont("\x00\x01\x00\x00", ttf, record(8, 12, 5))},
		{"a table offset past the end", mediaFont("\x00\x01\x00\x00", ttf, func(b []byte) []byte { return record(0, 8, uint32(len(b)+1))(record(0, 12, 0)(b)) })},
		{"an offset and length that wrap 32 bits", mediaFont("\x00\x01\x00\x00", ttf, func(b []byte) []byte { return record(0, 8, 0xFFFFFFFF)(record(0, 12, 2)(b)) })},
		{"a tag byte of 0x1F", mediaFont("\x00\x01\x00\x00", append(append([]string(nil), ttf...), "ab\x1fc"), nil)},
		{"a tag byte of 0x7F", mediaFont("\x00\x01\x00\x00", append(append([]string(nil), ttf...), "ab\x7fc"), nil)},
		{"a tag twice", mediaFont("\x00\x01\x00\x00", append(append([]string(nil), ttf...), "cmap"), nil)},
		{"no cmap", mediaFont("\x00\x01\x00\x00", mediaReplace(ttf, "cmap", ""), nil)},
		{"no post", mediaFont("\x00\x01\x00\x00", mediaReplace(ttf, "post", ""), nil)},
		{"glyf without loca", mediaFont("\x00\x01\x00\x00", mediaReplace(ttf, "loca", ""), nil)},
		{"no outlines", mediaFont("OTTO", mediaReplace(cff, "CFF ", ""), nil)},
		{"cff in lower case", mediaFont("OTTO", mediaReplace(cff, "CFF ", "cff "), nil)},
	}
	var out []mediaFontCase
	for _, c := range cases {
		typ, err := checkFont(c.b)
		ok, r := reason(err)
		out = append(out, mediaFontCase{Name: c.name, Hex: hex.EncodeToString(c.b), Valid: ok, Type: typ, Reason: r})
	}
	return out
}

func rangeCases() []mediaRangeCase {
	cases := []struct {
		name, text string
		size       int64
	}{
		{"first to last", "bytes=0-99", 1000},
		{"first to last, both included: one byte", "bytes=5-5", 1000},
		{"from first to the end", "bytes=100-", 1000},
		{"the last count bytes", "bytes=-100", 1000},
		{"a last past the end means the end", "bytes=900-5000", 1000},
		{"a count above the length means the whole", "bytes=-5000", 1000},
		{"the last byte", "bytes=999-", 1000},
		{"leading zeros", "bytes=0010-0019", 1000},
		{"15 digits", "bytes=0-999999999999999", 1000},
		{"first at the end: not satisfiable", "bytes=1000-", 1000},
		{"first past the end: not satisfiable", "bytes=2000-3000", 1000},
		{"a suffix of an empty file: not satisfiable", "bytes=-1", 0},
		{"16 digits", "bytes=0-1000000000000000", 1000},
		{"first above last", "bytes=10-9", 1000},
		{"a count of 0", "bytes=-0", 1000},
		{"bytes=-", "bytes=-", 1000},
		{"several ranges", "bytes=0-1,5-9", 1000},
		{"a space", "bytes= 0-99", 1000},
		{"another unit", "items=0-99", 1000},
		{"the unit in another case", "Bytes=0-99", 1000},
		{"a negative first", "bytes=-1-5", 1000},
		{"no unit", "0-99", 1000},
		{"a plus sign", "bytes=+0-99", 1000},
	}
	var out []mediaRangeCase
	for _, c := range cases {
		start, end, err := checkRange(c.text, c.size)
		ok, r := reason(err)
		rc := mediaRangeCase{Name: c.name, Text: c.text, Size: c.size, Valid: ok, Reason: r}
		if ok {
			rc.Start, rc.End = &start, &end
		}
		out = append(out, rc)
	}
	return out
}

func fileAnswerCases() []mediaAnswerCase {
	const size, etag, tag = 1000, "v1-3f9a", `"v1-3f9a"`
	cases := []mediaAnswerCase{
		{Name: "a 200 without Range", Status: 200, ETagHeader: tag, ContentLength: "1000"},
		{Name: "a 206 to first-last", Range: "bytes=0-99", Status: 206, ETagHeader: tag, ContentRange: "bytes 0-99/1000", ContentLength: "100"},
		{Name: "a 206 without Content-Length", Range: "bytes=0-99", Status: 206, ETagHeader: tag, ContentRange: "bytes 0-99/1000"},
		{Name: "a 206 ending before last (cut short)", Range: "bytes=0-99", Status: 206, ETagHeader: tag, ContentRange: "bytes 0-49/1000", ContentLength: "50"},
		{Name: "a 206 to first-", Range: "bytes=100-", Status: 206, ETagHeader: tag, ContentRange: "bytes 100-999/1000", ContentLength: "900"},
		{Name: "a 206 to -count", Range: "bytes=-100", Status: 206, ETagHeader: tag, ContentRange: "bytes 900-999/1000", ContentLength: "100"},
		{Name: "a 206 to a count above the length", Range: "bytes=-5000", Status: 206, ETagHeader: tag, ContentRange: "bytes 0-999/1000", ContentLength: "1000"},
		{Name: "a 206 to a last past the end", Range: "bytes=900-5000", Status: 206, ETagHeader: tag, ContentRange: "bytes 900-999/1000"},
		{Name: "a 200 to a satisfiable Range", Range: "bytes=0-99", Status: 200, ETagHeader: tag, ContentLength: "1000"},
		{Name: "a 200 to a Range that can't be satisfied", Range: "bytes=1000-", Status: 200, ETagHeader: tag, ContentLength: "1000"},
		{Name: "a 206 without Range", Status: 206, ETagHeader: tag, ContentRange: "bytes 0-999/1000", ContentLength: "1000"},
		{Name: "a 416 to a Range that can't be satisfied", Range: "bytes=1000-", Status: 416, ContentRange: "bytes */1000"},
		{Name: "a 404", Status: 404},
		{Name: "a 304", Status: 304, ETagHeader: tag},
		{Name: "no ETag", Status: 200, ContentLength: "1000"},
		{Name: "a weak ETag", Status: 200, ETagHeader: "W/" + tag, ContentLength: "1000"},
		{Name: "an unquoted ETag", Status: 200, ETagHeader: etag, ContentLength: "1000"},
		{Name: "another etag: the copy changed", Range: "bytes=0-99", Status: 206, ETagHeader: `"v2-0000"`, ContentRange: "bytes 0-99/1000"},
		{Name: "a 200 of another length: the copy changed", Status: 200, ETagHeader: tag, ContentLength: "999"},
		{Name: "a 200 without Content-Length", Status: 200, ETagHeader: tag},
		{Name: "a total other than the size: the copy changed", Range: "bytes=0-99", Status: 206, ETagHeader: tag, ContentRange: "bytes 0-99/1001"},
		{Name: "a total of *", Range: "bytes=0-99", Status: 206, ETagHeader: tag, ContentRange: "bytes 0-99/*"},
		{Name: "no Content-Range", Range: "bytes=0-99", Status: 206, ETagHeader: tag},
		{Name: "a Content-Range of another form", Range: "bytes=0-99", Status: 206, ETagHeader: tag, ContentRange: "bytes=0-99/1000"},
		{Name: "a Content-Range with start above end", Range: "bytes=10-99", Status: 206, ETagHeader: tag, ContentRange: "bytes 10-9/1000"},
		{Name: "a Content-Range with end at total", Range: "bytes=0-", Status: 206, ETagHeader: tag, ContentRange: "bytes 0-1000/1000"},
		{Name: "a start other than first", Range: "bytes=100-199", Status: 206, ETagHeader: tag, ContentRange: "bytes 0-199/1000"},
		{Name: "a start other than total - count", Range: "bytes=-100", Status: 206, ETagHeader: tag, ContentRange: "bytes 800-999/1000"},
		{Name: "an end past last", Range: "bytes=0-99", Status: 206, ETagHeader: tag, ContentRange: "bytes 0-100/1000"},
		{Name: "a Content-Length other than end - start + 1", Range: "bytes=0-99", Status: 206, ETagHeader: tag, ContentRange: "bytes 0-99/1000", ContentLength: "1000"},
		{Name: "a malformed Range is never sent", Range: "bytes=0-1,5-9", Status: 206, ETagHeader: tag, ContentRange: "bytes 0-1/1000"},
	}
	for i := range cases {
		c := &cases[i]
		c.Size, c.ETag = size, etag
		c.Valid, c.Reason = reason(checkFileAnswer(mediaAnswer{Range: c.Range, Size: c.Size, ETag: c.ETag, Status: c.Status,
			ETagHeader: c.ETagHeader, ContentRange: c.ContentRange, ContentLength: c.ContentLength}))
	}
	return cases
}

func nameCases() []mediaNameCase {
	pb := mediaTicket{Kind: "playback", Protocol: "hls",
		Subtitles: []playbackSubtitle{{ID: "s4", Form: "text", ASS: true}, {ID: "s5", Form: "text"}, {ID: "s6", Form: "picture"}},
		FontCount: 3, SheetCount: 12, Queue: []string{"hls", "", "file"}, Names: []string{"v0.m3u8", "init.mp4", "seg0.mp4"}}
	file := pb
	file.Protocol = "file"
	dl := file
	dl.Kind = "download"
	scan := mediaTicket{Kind: "scan", Protocol: "file"}
	prev := mediaTicket{Kind: "previews", SheetCount: 12}
	img := mediaTicket{Kind: "image"}
	cases := []struct {
		name string
		t    mediaTicket
		text string
	}{
		{"hls: the master playlist", pb, "master.m3u8"},
		{"hls: a name its playlists give", pb, "seg0.mp4"},
		{"hls: a name no playlist gave", pb, "seg1.mp4"},
		{"hls: file, not its protocol", pb, "file"},
		{"file: the file", file, "file"},
		{"file: master.m3u8, not its protocol", file, "master.m3u8"},
		{"a text track's stretch", pb, "subtitles/s4/0"},
		{"a text track's last stretch", pb, "subtitles/s5/9999"},
		{"a text track's stretch 10,000", pb, "subtitles/s5/10000"},
		{"a text track whole", pb, "subtitles/s5/whole"},
		{"a stretch with a leading zero", pb, "subtitles/s4/01"},
		{"a text track as ASS, ass true", pb, "ass/s4/whole"},
		{"a text track as ASS, ass absent", pb, "ass/s5/0"},
		{"a picture track's stretch", pb, "pictures/s6/3"},
		{"a picture track whole: pictures come in stretches only", pb, "pictures/s6/whole"},
		{"a picture track as text", pb, "subtitles/s6/0"},
		{"a text track as pictures", pb, "pictures/s4/0"},
		{"a track the answer didn't list", pb, "subtitles/s7/0"},
		{"a track URL without its part", pb, "subtitles/s4"},
		{"font 0", pb, "fonts/0"},
		{"the last font, fontCount - 1", pb, "fonts/2"},
		{"a font at fontCount", pb, "fonts/3"},
		{"a font with a leading zero", pb, "fonts/02"},
		{"a sheet under the playback's ticket", pb, "trickplay/0"},
		{"the last sheet", pb, "trickplay/11"},
		{"a sheet at the sheet count", pb, "trickplay/12"},
		{"queue entry 0, hls", pb, "queue/0/master.m3u8"},
		{"queue entry 0, a name its playlists give", pb, "queue/0/seg0.mp4"},
		{"queue entry 0 as file: not its protocol", pb, "queue/0/file"},
		{"queue entry 1, not prepared", pb, "queue/1/master.m3u8"},
		{"queue entry 2, file", pb, "queue/2/file"},
		{"queue entry 3, past the queue", pb, "queue/3/file"},
		{"queue entry 00", pb, "queue/00/master.m3u8"},
		{"a name of no list", pb, "index.m3u8"},
		{"a path under a name", pb, "fonts/0/x"},
		{"a download's file", dl, "file"},
		{"a download's subtitles", dl, "subtitles/s4/whole"},
		{"a download's font", dl, "fonts/1"},
		{"a download's sheet: not opened", dl, "trickplay/0"},
		{"a download's queue: not opened", dl, "queue/2/file"},
		{"a scan's file", scan, "file"},
		{"a scan's subtitles: not opened", scan, "subtitles/s4/0"},
		{"a previews ticket's sheet", prev, "trickplay/11"},
		{"a previews ticket's sheet at the count", prev, "trickplay/12"},
		{"a previews ticket's stream: not opened", prev, "master.m3u8"},
		{"an image ticket's image", img, "image"},
		{"an image ticket's file: not opened", img, "file"},
		{"a playback ticket's image: not opened", pb, "image"},
	}
	var out []mediaNameCase
	for _, c := range cases {
		ok, r := reason(mediaServes(c.t, c.text))
		out = append(out, mediaNameCase{Name: c.name, Ticket: c.t, Text: c.text, Valid: ok, Reason: r})
	}
	return out
}

func relayCases() []mediaRelayCase {
	cases := []mediaRelayCase{
		{Name: "the file of video/matroska", What: "file", Value: "video/matroska"},
		{Name: "the file of audio/flac, a video title's copy too: the copy's own type", What: "file", Value: "audio/flac"},
		{Name: "the file of video/vnd.dlna.mpeg-tts (M2TS)", What: "file", Value: "video/vnd.dlna.mpeg-tts"},
		{Name: "the file of a type of another top level, kept", What: "file", Value: "application/mp4"},
		{Name: "the file with no mediaType", What: "file", Value: ""},
		{Name: "the file with a malformed mediaType: as none", What: "file", Value: "video/MP4"},
		{Name: "a segment of video/mp2t", What: "segment", Value: "video/mp2t"},
		{Name: "a segment of video/MP2T: lowercased", What: "segment", Value: "video/MP2T"},
		{Name: "a segment with a parameter: dropped", What: "segment", Value: "video/mp4; codecs=\"avc1.640028\""},
		{Name: "a segment of audio/aac", What: "segment", Value: "audio/aac"},
		{Name: "a segment of text/html", What: "segment", Value: "text/html"},
		{Name: "a segment of application/octet-stream", What: "segment", Value: "application/octet-stream"},
		{Name: "a segment with an underscore", What: "segment", Value: "video/x_y"},
		{Name: "a segment without a type", What: "segment", Value: ""},
	}
	for i := range cases {
		cases[i].Output = mediaRelayType(cases[i].What, cases[i].Value)
	}
	return cases
}

func scanCases() []mediaScanCase {
	const big, huge = 1 << 30, 4 << 30
	const rec = 32 << 20 // the suggested scan cap
	cases := []mediaScanCase{
		{Name: "a first read", ScanBytes: rec, Sent: 0, Range: "bytes=0-65535", Size: big},
		{Name: "a read of exactly the cap", ScanBytes: rec, Sent: 0, Range: "bytes=0-33554431", Size: big},
		{Name: "a read past the cap: cut short at the cap", ScanBytes: rec, Sent: 0, Range: "bytes=0-33554432", Size: big},
		{Name: "a read to the end, past the cap: cut short", ScanBytes: rec, Sent: 0, Range: "bytes=100-", Size: big},
		{Name: "the last count bytes, past the cap: cut short", ScanBytes: rec, Sent: 0, Range: "bytes=-40000000", Size: big},
		{Name: "a read with one byte left", ScanBytes: rec, Sent: rec - 1, Range: "bytes=500-999", Size: big},
		{Name: "a read once the cap is spent", ScanBytes: rec, Sent: rec, Range: "bytes=0-0", Size: big},
		{Name: "a byte sent twice counts twice", ScanBytes: rec, Sent: rec - 10, Range: "bytes=0-99", Size: big},
		{Name: "a read of a small file", ScanBytes: rec, Sent: 0, Range: "bytes=0-", Size: 1000},
		{Name: "a read without Range", ScanBytes: rec, Sent: 0, Size: big},
		{Name: "a malformed Range", ScanBytes: rec, Sent: 0, Range: "bytes=0-1,5-9", Size: big},
		{Name: "a Range past the end", ScanBytes: rec, Sent: 0, Range: "bytes=1000-", Size: 1000},
		{Name: "an answer without scanBytes: the floor, 4 MiB, cuts the read", Sent: 0, Range: "bytes=0-33554431", Size: big},
		{Name: "an answer without scanBytes: past 4 MiB, refused", Sent: scanFloor, Range: "bytes=0-0", Size: big},
		{Name: "scanBytes at its floor, 4 MiB", ScanBytes: scanFloor, Sent: 0, Range: "bytes=0-", Size: big},
		{Name: "scanBytes below its floor: the floor", ScanBytes: scanFloor - 1, Sent: 0, Range: "bytes=0-", Size: big},
		{Name: "scanBytes at 1 GiB: a read of the whole cap", ScanBytes: scanTop, Sent: 0, Range: "bytes=0-", Size: huge},
		{Name: "scanBytes at 1 GiB, one byte left", ScanBytes: scanTop, Sent: scanTop - 1, Range: "bytes=0-99", Size: huge},
		{Name: "scanBytes above 1 GiB: the floor", ScanBytes: scanTop + 1, Sent: 0, Range: "bytes=0-", Size: huge},
	}
	for i := range cases {
		c := &cases[i]
		c.Status, c.ContentRange = scanAnswer(scanCapOf(c.ScanBytes), c.Sent, c.Range, c.Size)
	}
	return cases
}

func genV0Media() []byte {
	return encode(v1MediaFile{
		About: []string{
			"What a server does with media it serves or relays: MSFP Media version 0's fetching (https://github.com/msfp-io/msfp-media/blob/main/spec/v0/fetching.md) sections 1, 2.1, 2.3, 3, 3.1, 3.2 and 4.2, and its video sections 6.3 to 6.6, as MSFP's media part carries them (spec/v0/media.md, which adds the scan ticket, section 4.2). The owning server is fetching's source, the viewer's server its caller. Generated by tools/vectors from the specification's rules.",
			"names: a name, the URL's path after <stream root><ticket>/, under a ticket the owning server knows (its kind; for a playback or a download what its answer opened: protocol, subtitles entries, fontCount, the sheet count of the copy's previewLayout, the protocol of each queue entry it prepared, empty for one it didn't, and the bare names its playlists give). valid: served; else 404 (MSFP Media base section 4 names it ticket-refused, for logs; the caller reads the status only), and reason says why (fetching sections 1 and 4.2).",
			"relayTypes: what the viewer's server relays as Content-Type (output): the file (what: file) with the copy's mediaType from the catalog (value; empty: none), application/octet-stream when it has none; a segment (what: segment) from the owning server's Content-Type (value) (fetching section 3).",
			"webvtt: a part (text, or hex when it isn't UTF-8), whether it is refused, and the rebuilt part (output) when it isn't (MSFP Media video section 6.3). The 5 MiB part and 16 MiB track sizes and the 50,000 and 200,000 cue counts are not exercised here.",
			"ass: the same for ASS (MSFP Media video section 6.4). The 5 MiB and 16 MiB sizes and the 50,000 and 200,000 event counts are not exercised here.",
			"pgs: PGS parts, in hex, as MSFP Media video section 6.5 checks them; times are zero. An empty part (no hex) is valid. The 64 MiB size and 20,000 objects are not exercised here.",
			"fonts: tiny font headers, in hex (a table directory and 4 zero bytes per table, not real fonts), as MSFP Media video section 6.6 checks them; type is the media type a valid font is relayed as. The 10 MiB size is not exercised here.",
			"hls: playlists as the viewer's server checks them (fetching section 3.1, steps 1 to 10); reason names the step that fails first. The master playlist's attributes (BANDWIDTH, CODECS, RESOLUTION, VIDEO-RANGE, SUPPLEMENTAL-CODECS, CHANNELS) are the owning server's to send; no step refuses a playlist without them. The playback of every case has the audio tracks a1 to a40. valid means the checks pass; tags outside the subset may still be dropped before serving. The 2 MiB size and 50,000 URIs are not exercised here.",
			"ranges: a Range header against a file of size bytes (fetching section 2.1): start and end, both included, when it is well-formed and satisfiable; reason says malformed or not satisfiable, both answered 416.",
			"fileAnswers: an answer of the owning server for the file (status and headers; an absent member is an absent header), for the Range the viewer's server sent (none when absent) and the copy's size and etag from the catalog, as the viewer's server checks it (fetching sections 2.1, 2.3 and 3.2): valid when it relays it.",
			"scans: the owning server's answer to a fetch under a scan ticket, MSFP's own, that has already sent sent body bytes (MSFP media section 4.2), for the scanBytes its scan answer states (MSFP playback section 8; absent: none, and the cap is the floor, 4,194,304 bytes, the value the caller counts on; out of 4 MiB to 1 GiB: the floor too): status 206 with its Content-Range, cut short at the cap, or 416 with Content-Range: bytes */<size>.",
		},
		Names:       nameCases(),
		RelayTypes:  relayCases(),
		WebVTT:      vttCases(),
		ASS:         assCases(),
		PGS:         pgsVerdicts(),
		Fonts:       fontCases(),
		HLS:         hlsVerdicts(),
		Ranges:      rangeCases(),
		FileAnswers: fileAnswerCases(),
		Scans:       scanCases(),
	})
}

// destinationVerdicts are IP addresses a server connects to for a peer by
// default (valid) or refuses (spec/v0/core.md section 3.3).
func destinationVerdicts() []verdict {
	var out []verdict
	for _, a := range []string{"192.0.2.10", "198.51.100.7", "203.0.113.200", "2001:db8::1", "10.1.2.3", "172.16.0.1",
		"192.168.1.5", "100.64.0.1", "fd00::5", "::ffff:192.0.2.10",
		"127.0.0.1", "127.1.2.3", "::1", "::ffff:127.0.0.1", "169.254.169.254", "::ffff:169.254.169.254", "fe80::1",
		"0.0.0.0", "0.1.2.3", "::", "224.0.0.1", "239.255.255.250", "ff02::1", "240.0.0.1", "255.255.255.255", "fe80::1%eth0",
		// NAT64 (64:ff9b::/96) and 6to4 (2002::/16), judged as the IPv4 address they carry.
		"64:ff9b::c000:20a", "64:ff9b::a01:203", "64:ff9b::a9fe:a9fe", "64:ff9b::7f00:1", "64:ff9b::e000:1",
		"2002:c000:20a::1", "2002:a9fe:a9fe::1", "2002:7f00:1::", "2002::1"} {
		ok, r := reason(checkDestination(a))
		out = append(out, verdict{Text: a, Valid: ok, Reason: r})
	}
	return out
}
