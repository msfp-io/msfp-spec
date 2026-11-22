package main

// The media rules of version 0, as revised by proposals 16 and 17: the
// names an owning server serves, HLS playlists, the relayed media type,
// ranges and the file's answers, the scan cap of the scan answer's
// scanBytes (spec/v0/media.md sections 1, 2.1, 3, 3.1, 3.2 and 4.2),
// WebVTT, ASS, PGS and fonts (spec/v0/video.md sections 6.3 to 6.6), and
// the destinations of spec/v0/core.md section 3.3. Written from the
// specification's text, to check vectors/v0/media.json and the
// destinations of vectors/v0/strict.json.

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"html"
	"math"
	"net/netip"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"
)

// --- Destinations (core section 3.3) ---

var refusedRanges = []struct {
	prefix netip.Prefix
	why    string
}{
	{netip.MustParsePrefix("127.0.0.0/8"), "loopback"},
	{netip.MustParsePrefix("::1/128"), "loopback"},
	{netip.MustParsePrefix("169.254.0.0/16"), "link-local"},
	{netip.MustParsePrefix("fe80::/10"), "link-local"},
	{netip.MustParsePrefix("0.0.0.0/8"), "unspecified or this network"},
	{netip.MustParsePrefix("::/128"), "unspecified"},
	{netip.MustParsePrefix("224.0.0.0/4"), "multicast"},
	{netip.MustParsePrefix("ff00::/8"), "multicast"},
	{netip.MustParsePrefix("240.0.0.0/4"), "reserved or broadcast"},
}

var (
	nat64     = netip.MustParsePrefix("64:ff9b::/96")
	sixToFour = netip.MustParsePrefix("2002::/16")
)

// checkDestination says whether a server connects to ip for a peer by
// default: nil when it does.
func checkDestination(s string) error {
	ip, err := netip.ParseAddr(s)
	if err != nil || ip.Zone() != "" {
		return errors.New("not an IP address")
	}
	ip = ip.Unmap() // an IPv4-mapped address is judged as IPv4
	// NAT64's well-known prefix (RFC 6052: the last 32 bits) and 6to4
	// (RFC 3056: bits 16 to 47) are judged as the IPv4 address they carry.
	if b := ip.As16(); ip.Is6() && nat64.Contains(ip) {
		ip = netip.AddrFrom4([4]byte(b[12:16]))
	} else if ip.Is6() && sixToFour.Contains(ip) {
		ip = netip.AddrFrom4([4]byte(b[2:6]))
	}
	for _, r := range refusedRanges {
		if r.prefix.Contains(ip) {
			return fmt.Errorf("refused: %s", r.why)
		}
	}
	return nil
}

// --- Small helpers ---

func mediaWhite(r rune) bool { return unicode.Is(unicode.White_Space, r) }

func mediaTrim(s string) string { return strings.TrimFunc(s, mediaWhite) }

// mediaFold compares without ASCII case only.
func mediaFold(a, b string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range len(a) {
		x, y := a[i], b[i]
		if x >= 'A' && x <= 'Z' {
			x += 'a' - 'A'
		}
		if y >= 'A' && y <= 'Z' {
			y += 'a' - 'A'
		}
		if x != y {
			return false
		}
	}
	return true
}

var mediaIntForm = regexp.MustCompile(`^-?[0-9]+$`)

// mediaInt reads an integer (an optional -, digits) within lo to hi.
func mediaInt(s string, lo, hi int64) bool {
	if !mediaIntForm.MatchString(s) {
		return false
	}
	v, err := strconv.ParseInt(s, 10, 64)
	return err == nil && v >= lo && v <= hi
}

// --- HLS (media section 3.1) ---

const (
	maxPlaylist        = 2 << 20
	maxPlaylistURIs    = 50_000
	maxRenditions      = 32
	maxTargetDuration  = 60
	playbackAudioCount = 40 // the hls cases' playback has audio tracks a1 to a40
)

var playlistURI = regexp.MustCompile(`(?i)URI=`)

// playbackAudio is the hls cases' playback: its audio track IDs.
func playbackAudio(id string) bool {
	if !strings.HasPrefix(id, "a") || len(id) < 2 || id[1] == '0' || !mediaIntForm.MatchString(id[1:]) {
		return false
	}
	n, err := strconv.Atoi(id[1:])
	return err == nil && n >= 1 && n <= playbackAudioCount
}

var (
	extinfForm   = regexp.MustCompile(`^#EXTINF:([0-9]+(?:\.[0-9]+)?)(?:,.*)?$`)
	targetForm   = regexp.MustCompile(`^#EXT-X-TARGETDURATION:([0-9]+)$`)
	hlsAttribute = regexp.MustCompile(`^([A-Z0-9-]+)=("[^"\r\n]*"|[^",]*)(?:,|$)`)
)

// hlsAttributes reads an attribute list (RFC 8216 section 4.2).
func hlsAttributes(s string) (map[string]string, error) {
	out := map[string]string{}
	for s != "" {
		m := hlsAttribute.FindStringSubmatch(s)
		if m == nil {
			return nil, errors.New("a malformed attribute list")
		}
		if _, ok := out[m[1]]; ok {
			return nil, errors.New("an attribute twice")
		}
		out[m[1]] = m[2]
		s = s[len(m[0]):]
	}
	return out, nil
}

// checkPlaylist applies the viewer's server's checks, in order, for the
// hls cases' playback (audio tracks a1 to a40).
func checkPlaylist(b []byte) error {
	// 1.
	if len(b) > maxPlaylist {
		return errors.New("step 1: over 2 MiB")
	}
	// 2.
	lines := strings.Split(string(b), "\n")
	for i, l := range lines {
		lines[i] = strings.TrimSuffix(l, "\r")
	}
	// 3.
	if lines[0] != "#EXTM3U" {
		return errors.New("step 3: the first line isn't #EXTM3U")
	}
	lines = lines[1:]
	// 4.
	uris := 0
	lastURI := -1
	for i, l := range lines {
		if l == "" || strings.HasPrefix(l, "#") {
			continue
		}
		if err := checkMediaName(l); err != nil {
			return fmt.Errorf("step 4: a URI line: %v", err)
		}
		uris++
		lastURI = i
	}
	// 5.
	for _, l := range lines {
		if !strings.HasPrefix(l, "#") {
			continue
		}
		for _, m := range playlistURI.FindAllStringIndex(l, -1) {
			rest := l[m[1]:]
			if !strings.HasPrefix(rest, `"`) {
				return errors.New("step 5: an unquoted URI attribute")
			}
			end := strings.IndexByte(rest[1:], '"')
			if end < 0 {
				return errors.New("step 5: a URI attribute without its closing quote")
			}
			if err := checkMediaName(rest[1 : 1+end]); err != nil {
				return fmt.Errorf("step 5: a URI attribute: %v", err)
			}
			uris++
		}
	}
	// 6.
	for _, l := range lines {
		if l == "#EXT-X-DEFINE" || strings.HasPrefix(l, "#EXT-X-DEFINE:") {
			return errors.New("step 6: EXT-X-DEFINE")
		}
	}
	// 7.
	for _, l := range lines {
		if strings.HasPrefix(l, "#EXT-X-KEY:") || strings.HasPrefix(l, "#EXT-X-SESSION-KEY:") {
			return errors.New("step 7: EXT-X-KEY or EXT-X-SESSION-KEY")
		}
	}
	// 8.
	if uris > maxPlaylistURIs {
		return errors.New("step 8: over 50,000 URIs")
	}
	// 9.
	isMedia, isMaster := false, false
	for _, l := range lines {
		if l == "#EXTINF" || strings.HasPrefix(l, "#EXTINF:") {
			isMedia = true
		}
		if l == "#EXT-X-STREAM-INF" || strings.HasPrefix(l, "#EXT-X-STREAM-INF:") {
			isMaster = true
		}
	}
	if !isMedia && !isMaster {
		return errors.New("step 9: neither EXT-X-STREAM-INF nor EXTINF")
	}
	if isMedia {
		if err := checkMediaPlaylist(lines, lastURI); err != nil {
			return fmt.Errorf("step 9: %v", err)
		}
	}
	// 10.
	if isMaster {
		renditions := 0
		for _, l := range lines {
			if l != "#EXT-X-MEDIA" && !strings.HasPrefix(l, "#EXT-X-MEDIA:") {
				continue
			}
			if renditions++; renditions > maxRenditions {
				return errors.New("step 10: over 32 EXT-X-MEDIA")
			}
			a, err := hlsAttributes(strings.TrimPrefix(strings.TrimPrefix(l, "#EXT-X-MEDIA"), ":"))
			if err != nil {
				return fmt.Errorf("step 10: EXT-X-MEDIA: %v", err)
			}
			if a["TYPE"] != "AUDIO" {
				return errors.New("step 10: an EXT-X-MEDIA without TYPE=AUDIO")
			}
			name := a["NAME"]
			if len(name) < 2 || name[0] != '"' || !playbackAudio(name[1:len(name)-1]) {
				return errors.New("step 10: an EXT-X-MEDIA whose NAME isn't an audio track ID of the playback")
			}
		}
	}
	return nil
}

// checkMediaPlaylist is step 9 for a media playlist.
func checkMediaPlaylist(lines []string, lastURI int) error {
	vod, ends, endAt := 0, 0, -1
	target := -1
	for i, l := range lines {
		switch {
		case l == "#EXT-X-PLAYLIST-TYPE:VOD":
			vod++
		case l == "#EXT-X-PLAYLIST-TYPE" || strings.HasPrefix(l, "#EXT-X-PLAYLIST-TYPE:"):
			return errors.New("a playlist type other than VOD: a growing playlist")
		case l == "#EXT-X-ENDLIST":
			ends++
			endAt = i
		case l == "#EXT-X-TARGETDURATION" || strings.HasPrefix(l, "#EXT-X-TARGETDURATION:"):
			m := targetForm.FindStringSubmatch(l)
			if m == nil || !mediaInt(m[1], 1, maxTargetDuration) {
				return errors.New("a target duration that isn't an integer from 1 to 60")
			}
			if target < 0 {
				target, _ = strconv.Atoi(m[1])
			}
		}
	}
	if vod != 1 {
		return errors.New("not exactly one EXT-X-PLAYLIST-TYPE:VOD")
	}
	if ends != 1 {
		return errors.New("not exactly one EXT-X-ENDLIST: a growing playlist")
	}
	if endAt < lastURI {
		return errors.New("EXT-X-ENDLIST before the last URI line")
	}
	if target < 0 {
		return errors.New("no EXT-X-TARGETDURATION")
	}
	for _, l := range lines {
		if l != "#EXTINF" && !strings.HasPrefix(l, "#EXTINF:") {
			continue
		}
		m := extinfForm.FindStringSubmatch(l)
		if m == nil {
			return errors.New("an EXTINF duration that isn't a decimal")
		}
		d, err := strconv.ParseFloat(m[1], 64)
		if err != nil || d <= 0 {
			return errors.New("an EXTINF duration not above 0")
		}
		if math.Round(d) > float64(target) {
			return errors.New("an EXTINF duration that rounds above the target duration")
		}
	}
	return nil
}

// --- Ranges and the file (media sections 2.1, 3.2 and 4.2) ---

var rangeForm = regexp.MustCompile(`^bytes=(?:([0-9]{1,15})-([0-9]{1,15})?|-([0-9]{1,15}))$`)

// mediaRangeOf is a well-formed Range: its form and numbers.
type mediaRangeOf struct {
	form        int // 1: first-last, 2: first-, 3: -count
	first, last int64
	count       int64
}

var errRangeMalformed = errors.New("malformed: 416, the file not read")

func parseRange(h string) (mediaRangeOf, error) {
	m := rangeForm.FindStringSubmatch(h)
	if m == nil {
		return mediaRangeOf{}, errRangeMalformed
	}
	n := func(s string) int64 { v, _ := strconv.ParseInt(s, 10, 64); return v }
	switch {
	case m[3] != "":
		r := mediaRangeOf{form: 3, count: n(m[3])}
		if r.count < 1 {
			return r, errRangeMalformed
		}
		return r, nil
	case m[2] != "":
		r := mediaRangeOf{form: 1, first: n(m[1]), last: n(m[2])}
		if r.first > r.last {
			return r, errRangeMalformed
		}
		return r, nil
	default:
		return mediaRangeOf{form: 2, first: n(m[1])}, nil
	}
}

// checkRange gives the bytes a Range asks of a file of size bytes: start
// and end, both included.
func checkRange(h string, size int64) (start, end int64, err error) {
	r, err := parseRange(h)
	if err != nil {
		return 0, 0, err
	}
	switch r.form {
	case 3:
		if size == 0 {
			return 0, 0, errors.New("not satisfiable: 416, the file has no byte")
		}
		return max(0, size-r.count), size - 1, nil
	default:
		if r.first >= size {
			return 0, 0, errors.New("not satisfiable: 416, first at or past the end")
		}
		end := size - 1
		if r.form == 1 && r.last < end {
			end = r.last
		}
		return r.first, end, nil
	}
}

// mediaAnswer is an answer of the owning server for the file, as the
// viewer's server receives it; empty strings are absent headers.
type mediaAnswer struct {
	Range         string // the Range the viewer's server sent
	Size          int64  // the copy's size
	ETag          string // the copy's etag
	Status        int
	ETagHeader    string
	ContentRange  string
	ContentLength string
}

var contentRangeForm = regexp.MustCompile(`^bytes ([0-9]+)-([0-9]+)/([0-9]+|\*)$`)

// checkFileAnswer is what the viewer's server checks on an answer for
// the file: nil when it relays it.
func checkFileAnswer(a mediaAnswer) error {
	var r mediaRangeOf
	satisfiable := false
	if a.Range != "" {
		var err error
		if r, err = parseRange(a.Range); err != nil {
			return errors.New("a malformed Range is never sent")
		}
		_, _, err = checkRange(a.Range, a.Size)
		satisfiable = err == nil
	}
	switch a.Status {
	case 200:
		if a.Range != "" {
			if satisfiable {
				return errors.New("a 200 to a satisfiable Range")
			}
			return errors.New("a 200 to a Range")
		}
	case 206:
		if a.Range == "" {
			return errors.New("a 206 without a Range")
		}
	case 416:
		return errors.New("416: a failed fetch (passed to the viewer as a 416 of its own when the viewer asked that range)")
	default:
		return fmt.Errorf("status %d: a failed fetch", a.Status)
	}
	if a.ETagHeader != `"`+a.ETag+`"` {
		return errors.New("an ETag other than the copy's etag, strong: the copy changed")
	}
	if a.Status == 200 {
		if a.ContentLength != strconv.FormatInt(a.Size, 10) {
			return errors.New("a 200 whose Content-Length isn't the copy's size")
		}
		return nil
	}
	m := contentRangeForm.FindStringSubmatch(a.ContentRange)
	if m == nil {
		return errors.New("a Content-Range not of the form bytes <start>-<end>/<total>")
	}
	if m[3] == "*" {
		return errors.New("a Content-Range total of * for the file")
	}
	start, e1 := strconv.ParseInt(m[1], 10, 64)
	end, e2 := strconv.ParseInt(m[2], 10, 64)
	total, e3 := strconv.ParseInt(m[3], 10, 64)
	if e1 != nil || e2 != nil || e3 != nil {
		return errors.New("a Content-Range number too large")
	}
	if start > end || end >= total {
		return errors.New("a Content-Range without start <= end < total")
	}
	if total != a.Size {
		return errors.New("a Content-Range total other than the copy's size: the copy changed")
	}
	want := r.first
	if r.form == 3 {
		want = max(0, total-r.count)
	}
	if start != want {
		return errors.New("a Content-Range start other than the range's")
	}
	if r.form == 1 && end > r.last {
		return errors.New("a Content-Range end past the range's last")
	}
	if a.ContentLength != "" && a.ContentLength != strconv.FormatInt(end-start+1, 10) {
		return errors.New("a Content-Length other than end - start + 1")
	}
	return nil
}

// The scan cap is the scan answer's scanBytes (playback section 8): 4
// MiB to 1 GiB, 4 MiB when the answer has none, or one out of range.
const (
	scanFloor = 4 << 20
	scanTop   = 1 << 30
)

// scanCapOf is the cap an owning server enforces for the scanBytes its
// scan answer states (0: none).
func scanCapOf(scanBytes int64) int64 {
	if scanBytes < scanFloor || scanBytes > scanTop {
		return scanFloor
	}
	return scanBytes
}

// scanAnswer is the owning server's answer to a fetch under a scan
// ticket of cap scanCap that has already sent sent bytes: 206 and its
// Content-Range, or 416 with Content-Range: bytes */<size>.
func scanAnswer(scanCap, sent int64, rng string, size int64) (int, string) {
	unsatisfied := fmt.Sprintf("bytes */%d", size)
	if rng == "" || sent >= scanCap {
		return 416, unsatisfied
	}
	start, end, err := checkRange(rng, size)
	if err != nil {
		return 416, unsatisfied
	}
	end = min(end, start+(scanCap-sent)-1)
	return 206, fmt.Sprintf("bytes %d-%d/%d", start, end, size)
}

// --- The names an owning server serves (media section 1) ---

// mediaTicket is what an owning server knows of a ticket: its kind
// (playback, download, scan, previews or image), and for a playback what
// its answer opened: the protocol, the subtitle entries, fontCount, the
// sheet count of the copy's previewLayout, the protocol of each queue
// entry it prepared ("" for one it didn't), and the bare names the
// playlists of its streams give.
type mediaTicket struct {
	Kind       string             `json:"kind"`
	Protocol   string             `json:"protocol,omitempty"`
	Subtitles  []playbackSubtitle `json:"subtitles,omitempty"`
	FontCount  int64              `json:"fontCount,omitempty"`
	SheetCount int64              `json:"sheetCount,omitempty"`
	Queue      []string           `json:"queue,omitempty"`
	Names      []string           `json:"names,omitempty"`
}

var mediaNumberForm = regexp.MustCompile(`^(0|[1-9][0-9]*)$`)

// mediaNumber reads a decimal number without leading zeros, below top.
func mediaNumber(s string, top int64) (int64, bool) {
	if !mediaNumberForm.MatchString(s) || len(s) > 15 {
		return 0, false
	}
	n, err := strconv.ParseInt(s, 10, 64)
	return n, err == nil && n < top
}

// mediaServes says whether the owning server serves name, the URL's path
// after <stream root><ticket>/, under a ticket: nil, or why it answers
// 404. It serves exactly the names section 1 lists, within their counts.
func mediaServes(t mediaTicket, name string) error {
	p := strings.Split(name, "/")
	stream := func(protocol string, p []string) error {
		switch {
		case len(p) != 1:
			return errors.New("not a name of the stream")
		case protocol == "file" && p[0] == "file", protocol == "hls" && p[0] == "master.m3u8":
			return nil
		case protocol == "hls" && checkMediaName(p[0]) == nil && slices.Contains(t.Names, p[0]):
			return nil
		}
		return errors.New("not a name of the stream")
	}
	switch t.Kind {
	case "image":
		if name == "image" {
			return nil
		}
		return errors.New("an image ticket opens image only")
	case "scan":
		if name == "file" {
			return nil
		}
		return errors.New("a scan ticket opens file only")
	case "previews":
		if len(p) == 2 && p[0] == "trickplay" {
			if _, ok := mediaNumber(p[1], t.SheetCount); ok {
				return nil
			}
			return errors.New("a sheet at or past the sheet count")
		}
		return errors.New("a previews ticket opens the sheets only")
	case "playback", "download":
	default:
		return errors.New("no ticket of that kind")
	}
	switch p[0] {
	case "subtitles", "ass", "pictures":
		if len(p) != 3 {
			return errors.New("a subtitle URL is <kind>/<id>/<part> or /whole")
		}
		var e *playbackSubtitle
		for i := range t.Subtitles {
			if t.Subtitles[i].ID == p[1] {
				e = &t.Subtitles[i]
			}
		}
		switch {
		case e == nil:
			return errors.New("a subtitle track the answer didn't list")
		case p[0] == "subtitles" && e.Form != "text", p[0] == "pictures" && e.Form != "picture", p[0] == "ass" && !e.ASS:
			return errors.New("a form the answer didn't open for that track")
		case p[2] == "whole" && p[0] != "pictures":
			return nil
		}
		if _, ok := mediaNumber(p[2], 10000); !ok {
			return errors.New("a part that isn't a number from 0 to 9,999")
		}
		return nil
	case "fonts":
		if len(p) == 2 {
			if _, ok := mediaNumber(p[1], t.FontCount); ok {
				return nil
			}
		}
		return errors.New("a font at or past fontCount")
	case "trickplay":
		if t.Kind != "playback" {
			return errors.New("a download ticket opens no sheets")
		}
		if len(p) == 2 {
			if _, ok := mediaNumber(p[1], t.SheetCount); ok {
				return nil
			}
		}
		return errors.New("a sheet at or past the sheet count")
	case "queue":
		if t.Kind != "playback" {
			return errors.New("a download ticket opens no queue")
		}
		if len(p) < 2 {
			return errors.New("a queue URL is queue/<n>/<name>")
		}
		n, ok := mediaNumber(p[1], int64(len(t.Queue)))
		if !ok || t.Queue[n] == "" {
			return errors.New("a queue entry the owning server didn't prepare")
		}
		return stream(t.Queue[n], p[2:])
	}
	return stream(t.Protocol, p)
}

// --- The relayed media type (media section 3) ---

// mediaRelayType is the Content-Type the viewer's server relays the file
// with, from the copy's mediaType in the catalog ("" for none), or a
// segment with, from the owning server's Content-Type.
func mediaRelayType(what, value string) string {
	const other = "application/octet-stream"
	if what == "file" {
		if playbackMediaType(value) {
			return value
		}
		return other
	}
	mt := mediaType(value)
	top, sub, ok := strings.Cut(mt, "/")
	if !ok || top != "video" && top != "audio" || sub == "" {
		return other
	}
	for _, c := range []byte(sub) {
		if !(c >= 'a' && c <= 'z' || c >= '0' && c <= '9' || c == '.' || c == '+' || c == '-') {
			return other
		}
	}
	return mt
}

// --- WebVTT (video section 6.3) ---

const (
	maxVTT      = 5 << 20
	maxCues     = 50_000
	maxCueLine  = 500
	maxCueTags  = 32
	maxCueDepth = 8
)

var timingLine = regexp.MustCompile(`^((?:[0-9]{1,3}:)?[0-9]{2}:[0-9]{2}\.[0-9]{3})[\t\f ]+-->[\t\f ]+((?:[0-9]{1,3}:)?[0-9]{2}:[0-9]{2}\.[0-9]{3})([\t\f ].*)?$`)

var escapeCue = strings.NewReplacer("&", "&amp;", "<", "&lt;", ">", "&gt;")

var (
	vttPercent  = `[0-9]{1,3}(?:\.[0-9]{1,3})?%`
	vttLine     = regexp.MustCompile(`^(` + vttPercent + `|-?[0-9]+)(?:,(?:start|center|end))?$`)
	vttPosition = regexp.MustCompile(`^(` + vttPercent + `)(?:,(?:line-left|center|line-right))?$`)
	vttSize     = regexp.MustCompile(`^(` + vttPercent + `)$`)
)

func vttPercentOK(s string) bool {
	v, err := strconv.ParseFloat(strings.TrimSuffix(s, "%"), 64)
	return err == nil && v >= 0 && v <= 100
}

// vttSetting says whether a cue setting is kept.
func vttSetting(name, value string) bool {
	switch name {
	case "line":
		m := vttLine.FindStringSubmatch(value)
		if m == nil {
			return false
		}
		if strings.HasSuffix(m[1], "%") {
			return vttPercentOK(m[1])
		}
		return mediaInt(m[1], -1000, 1000)
	case "position":
		m := vttPosition.FindStringSubmatch(value)
		return m != nil && vttPercentOK(m[1])
	case "size":
		m := vttSize.FindStringSubmatch(value)
		return m != nil && vttPercentOK(m[1])
	case "align":
		switch value {
		case "start", "center", "end", "left", "right":
			return true
		}
	}
	return false
}

// vttSettings keeps the cue settings, written in their order.
func vttSettings(s string) string {
	kept := map[string]string{}
	seen := map[string]bool{}
	for _, f := range strings.FieldsFunc(s, func(r rune) bool { return r == ' ' || r == '\t' || r == '\f' }) {
		i := strings.IndexByte(f, ':')
		if i < 0 {
			continue
		}
		name := f[:i]
		if seen[name] {
			continue
		}
		seen[name] = true
		if vttSetting(name, f[i+1:]) {
			kept[name] = f
		}
	}
	var out strings.Builder
	for _, n := range []string{"line", "position", "size", "align"} {
		if f, ok := kept[n]; ok {
			out.WriteString(" " + f)
		}
	}
	return out.String()
}

// rebuildVTT rebuilds a WebVTT part, or refuses it.
func rebuildVTT(in []byte) ([]byte, error) {
	if len(in) > maxVTT {
		return nil, errors.New("over 5 MiB")
	}
	text := strings.ReplaceAll(strings.ReplaceAll(string(in), "\r\n", "\n"), "\r", "\n")
	text = strings.TrimPrefix(text, "\uFEFF")
	if !strings.HasPrefix(text, "WEBVTT") {
		return nil, errors.New("doesn't start with WEBVTT")
	}
	var out bytes.Buffer
	out.WriteString("WEBVTT\n")
	cues := 0
	for _, block := range strings.Split(text, "\n\n") {
		lines := strings.Split(strings.Trim(block, "\n"), "\n")
		at := -1
		var m []string
		for i, l := range lines {
			if m = timingLine.FindStringSubmatch(mediaTrim(l)); m != nil {
				at = i
				break
			}
		}
		if at < 0 {
			continue
		}
		var body [][]vttTok
		for _, l := range lines[at+1:] {
			if toks := cleanCueLine(l); toks != nil {
				body = append(body, toks)
			}
		}
		if len(body) == 0 {
			continue
		}
		if cues++; cues > maxCues {
			return nil, errors.New("over 50,000 cues")
		}
		out.WriteString("\n" + m[1] + " --> " + m[2] + vttSettings(m[3]) + "\n")
		for _, l := range balanceCue(body) {
			out.WriteString(l + "\n")
		}
	}
	return out.Bytes(), nil
}

func directionControl(r rune) bool {
	return r == 0x061C || r == 0x200E || r == 0x200F || r >= 0x202A && r <= 0x202E || r >= 0x2066 && r <= 0x2069
}

// dropInvisible is step 1: invalid UTF-8, U+FFFD, direction controls.
func dropInvisible(s string) string {
	return strings.Map(func(r rune) rune {
		if r == utf8.RuneError || directionControl(r) {
			return -1
		}
		return r
	}, s)
}

// vttTok is a piece of a cue text line: a kept tag, or text.
type vttTok struct {
	tag  bool
	text string
}

// vttTagStart says whether s[i] is a '<' that starts a tag (step 2).
func vttTagStart(s string, i int) bool {
	if s[i] != '<' {
		return false
	}
	next, _ := utf8.DecodeRuneInString(s[i+1:])
	return unicode.IsLetter(next) || next == '/' || next == '!' || next == '?'
}

var vttColours = map[string]bool{"white": true, "lime": true, "cyan": true, "red": true, "yellow": true, "magenta": true, "blue": true, "black": true}

// vttKept says whether a tag is kept (step 3).
func vttKept(t string) bool {
	switch t {
	case "<i>", "</i>", "<b>", "</b>", "</c>":
		return true
	}
	if !strings.HasPrefix(t, "<c.") || !strings.HasSuffix(t, ">") {
		return false
	}
	classes := strings.Split(t[3:len(t)-1], ".")
	colour := func(c string) (bg, ok bool) {
		if strings.HasPrefix(c, "bg_") {
			return true, vttColours[c[3:]]
		}
		return false, vttColours[c]
	}
	switch len(classes) {
	case 1:
		_, ok := colour(classes[0])
		return ok
	case 2:
		bg1, ok1 := colour(classes[0])
		bg2, ok2 := colour(classes[1])
		return ok1 && ok2 && bg1 != bg2
	}
	return false
}

// vttUntag turns what is shaped like a tag into one space (step 4); one
// without its > removes the rest of the text.
func vttUntag(s string) string {
	var b strings.Builder
	for i := 0; i < len(s); i++ {
		if !vttTagStart(s, i) {
			b.WriteByte(s[i])
			continue
		}
		j := strings.IndexByte(s[i:], '>')
		if j < 0 {
			break
		}
		b.WriteByte(' ')
		i += j
	}
	return b.String()
}

// cleanCueLine is the cleaning of a cue text line, steps 1 to 7: its
// kept tags and escaped text, or nil when no text is left.
func cleanCueLine(s string) []vttTok {
	s = dropInvisible(s)
	// 2 and 3: tags and text.
	var pieces []vttTok
	for i := 0; i < len(s); {
		if vttTagStart(s, i) {
			j := strings.IndexByte(s[i:], '>')
			if j < 0 {
				break
			}
			t := s[i : i+j+1]
			if vttKept(t) {
				pieces = append(pieces, vttTok{tag: true, text: t})
			} else {
				pieces = append(pieces, vttTok{text: " "})
			}
			i += j + 1
			continue
		}
		j := i + 1
		for j < len(s) && !vttTagStart(s, j) {
			j++
		}
		pieces = append(pieces, vttTok{text: s[i:j]})
		i = j
	}
	// 4 to 6.
	var out []vttTok
	addText := func(t string) {
		if n := len(out); n > 0 && !out[n-1].tag {
			out[n-1].text += t
		} else {
			out = append(out, vttTok{text: t})
		}
	}
	size, tags := 0, 0
	pending, hasText, lastSpace, cut := false, false, false, false
	for _, p := range pieces {
		if cut {
			break
		}
		if p.tag {
			if tags < maxCueTags {
				// A run of spaces before a tag is written before it.
				if pending && hasText && !lastSpace {
					if size+1 > maxCueLine {
						break
					}
					addText(" ")
					size++
					lastSpace = true
				}
				pending = false
				tags++
				out = append(out, p)
			}
			continue
		}
		t := vttUntag(dropInvisible(html.UnescapeString(p.text)))
		for _, r := range t {
			if r == utf8.RuneError || directionControl(r) {
				continue
			}
			if unicode.IsControl(r) || mediaWhite(r) {
				pending = true
				continue
			}
			if pending && hasText && !lastSpace {
				if size+1 > maxCueLine {
					cut = true
					break
				}
				addText(" ")
				size++
			}
			pending = false
			if size+utf8.RuneLen(r) > maxCueLine {
				cut = true
				break
			}
			addText(string(r))
			size += utf8.RuneLen(r)
			hasText, lastSpace = true, false
		}
	}
	if !hasText {
		return nil
	}
	// Spaces at the end of the text go, tags after them or not.
	for i := len(out) - 1; i >= 0; i-- {
		if !out[i].tag {
			out[i].text = strings.TrimRight(out[i].text, " ")
			if out[i].text == "" {
				out = append(out[:i], out[i+1:]...)
			}
			break
		}
	}
	for i := range out {
		if !out[i].tag {
			out[i].text = escapeCue.Replace(out[i].text)
		}
	}
	return out
}

// balanceCue balances the tags over a cue's lines and writes them.
func balanceCue(lines [][]vttTok) []string {
	type open struct {
		name  string
		ghost bool // a start tag removed for depth
	}
	var stack []open
	real := 0
	var out []string
	for _, l := range lines {
		var b strings.Builder
		for _, t := range l {
			if !t.tag {
				b.WriteString(t.text)
				continue
			}
			end := strings.HasPrefix(t.text, "</")
			name := strings.Trim(t.text, "</>")
			if strings.HasPrefix(name, "c.") {
				name = "c"
			}
			if !end {
				if real >= maxCueDepth {
					stack = append(stack, open{name, true})
					continue
				}
				stack = append(stack, open{name, false})
				real++
				b.WriteString(t.text)
				continue
			}
			if n := len(stack); n > 0 && stack[n-1].name == name {
				top := stack[n-1]
				stack = stack[:n-1]
				if !top.ghost {
					real--
					b.WriteString(t.text)
				}
			}
		}
		out = append(out, b.String())
	}
	for i := len(stack) - 1; i >= 0; i-- {
		if !stack[i].ghost {
			out[len(out)-1] += "</" + stack[i].name + ">"
		}
	}
	return out
}

// --- ASS (video section 6.4) ---

const (
	maxASS           = 5 << 20
	maxASSStyles     = 1024
	maxASSEvents     = 50_000
	maxASSEventText  = 65_536
	maxASSBlocks     = 256
	maxASSTags       = 1024
	maxASSTransforms = 16
	maxASSNumbers    = 10_000
)

var assNumberForm = regexp.MustCompile(`^-?[0-9]{1,7}(?:\.[0-9]{1,3})?$`)

func assNumber(s string, lo, hi float64) bool {
	if !assNumberForm.MatchString(s) {
		return false
	}
	v, err := strconv.ParseFloat(s, 64)
	return err == nil && v >= lo && v <= hi
}

func assOneOf(s string, set ...string) bool {
	for _, v := range set {
		if s == v {
			return true
		}
	}
	return false
}

// assPlain makes text plain: invalid UTF-8 and control characters
// removed, commas removed.
func assPlain(s string) string {
	return strings.Map(func(r rune) rune {
		if r == utf8.RuneError || unicode.IsControl(r) || r == ',' {
			return -1
		}
		return r
	}, s)
}

var (
	assHexColour = regexp.MustCompile(`^&H([0-9A-Fa-f]{1,8})&?$`)
	assDecColour = regexp.MustCompile(`^[0-9]+$`)
	assTagColour = regexp.MustCompile(`^&H([0-9A-Fa-f]{1,6})&?$`)
	assTagAlpha  = regexp.MustCompile(`^&H([0-9A-Fa-f]{1,2})&?$`)
	assTime      = regexp.MustCompile(`^[0-9]{1,3}:[0-9]{2}:[0-9]{2}\.[0-9]{2}$`)
)

// assColour reads a style's colour, written &H and 8 uppercase digits.
func assColour(s string) (string, bool) {
	var v uint64
	var err error
	if m := assHexColour.FindStringSubmatch(s); m != nil {
		v, err = strconv.ParseUint(m[1], 16, 32)
	} else if assDecColour.MatchString(s) {
		v, err = strconv.ParseUint(s, 10, 32)
	} else {
		return "", false
	}
	if err != nil {
		return "", false
	}
	return fmt.Sprintf("&H%08X", v), true
}

var assInfoNames = []struct {
	name string
	ok   func(string) bool
}{
	{"PlayResX", func(s string) bool { return mediaInt(s, 1, 16384) }},
	{"PlayResY", func(s string) bool { return mediaInt(s, 1, 16384) }},
	{"LayoutResX", func(s string) bool { return mediaInt(s, 1, 16384) }},
	{"LayoutResY", func(s string) bool { return mediaInt(s, 1, 16384) }},
	{"WrapStyle", func(s string) bool { return assOneOf(s, "0", "1", "2", "3") }},
	{"ScaledBorderAndShadow", func(s string) bool { return assOneOf(s, "yes", "no") }},
	{"Collisions", func(s string) bool { return assOneOf(s, "Normal", "Reverse") }},
	{"Kerning", func(s string) bool { return assOneOf(s, "yes", "no") }},
	{"YCbCr Matrix", func(s string) bool {
		return assOneOf(s, "None", "TV.601", "PC.601", "TV.709", "PC.709", "TV.FCC", "PC.FCC", "TV.240M", "PC.240M")
	}},
}

// assStyleField is a field of a written style: its default, and its check
// (the written value, or false).
type assStyleField struct {
	name, def string
	check     func(v string, ssa bool) (string, bool)
}

func assKeep(ok func(string) bool) func(string, bool) (string, bool) {
	return func(v string, _ bool) (string, bool) { return v, ok(v) }
}

func assPlainField(v string, _ bool) (string, bool) {
	p := assPlain(v)
	return p, len(p) >= 1 && len(p) <= 64
}

func assFlag(v string, _ bool) (string, bool) {
	if !mediaIntForm.MatchString(v) {
		return "", false
	}
	if strings.Trim(strings.TrimPrefix(v, "-"), "0") == "" {
		return "0", true
	}
	return "-1", true
}

func assColourField(v string, _ bool) (string, bool) { return assColour(v) }

// assAlignment reads an alignment: 1 to 9, or SSA's 1 to 3, 5 to 7 and 9
// to 11, converted.
func assAlignment(v string, ssa bool) (string, bool) {
	if !ssa {
		return v, assOneOf(v, "1", "2", "3", "4", "5", "6", "7", "8", "9")
	}
	conv := map[string]string{"1": "1", "2": "2", "3": "3", "5": "7", "6": "8", "7": "9", "9": "4", "10": "5", "11": "6"}
	c, ok := conv[v]
	return c, ok
}

var assStyleFields = []assStyleField{
	{"Name", "", assPlainField},
	{"Fontname", "Arial", assPlainField},
	{"Fontsize", "20", assKeep(func(s string) bool { return assNumber(s, 0, 1000) })},
	{"PrimaryColour", "&H00FFFFFF", assColourField},
	{"SecondaryColour", "&H000000FF", assColourField},
	{"OutlineColour", "&H00000000", assColourField},
	{"BackColour", "&H00000000", assColourField},
	{"Bold", "0", assKeep(func(s string) bool { return mediaInt(s, -1, 1000) })},
	{"Italic", "0", assFlag},
	{"Underline", "0", assFlag},
	{"StrikeOut", "0", assFlag},
	{"ScaleX", "100", assKeep(func(s string) bool { return assNumber(s, 0, 1000) })},
	{"ScaleY", "100", assKeep(func(s string) bool { return assNumber(s, 0, 1000) })},
	{"Spacing", "0", assKeep(func(s string) bool { return assNumber(s, -1000, 1000) })},
	{"Angle", "0", assKeep(func(s string) bool { return assNumber(s, -360, 360) })},
	{"BorderStyle", "1", assKeep(func(s string) bool { return assOneOf(s, "1", "3", "4") })},
	{"Outline", "2", assKeep(func(s string) bool { return assNumber(s, 0, 100) })},
	{"Shadow", "2", assKeep(func(s string) bool { return assNumber(s, 0, 100) })},
	{"Alignment", "2", assAlignment},
	{"MarginL", "10", assKeep(func(s string) bool { return mediaInt(s, 0, 16384) })},
	{"MarginR", "10", assKeep(func(s string) bool { return mediaInt(s, 0, 16384) })},
	{"MarginV", "10", assKeep(func(s string) bool { return mediaInt(s, 0, 16384) })},
	{"Encoding", "1", assKeep(func(s string) bool { return mediaInt(s, 0, 255) })},
}

var (
	assV4PlusFormat = []string{"Name", "Fontname", "Fontsize", "PrimaryColour", "SecondaryColour", "OutlineColour", "BackColour", "Bold", "Italic", "Underline", "StrikeOut", "ScaleX", "ScaleY", "Spacing", "Angle", "BorderStyle", "Outline", "Shadow", "Alignment", "MarginL", "MarginR", "MarginV", "Encoding"}
	assV4Format     = []string{"Name", "Fontname", "Fontsize", "PrimaryColour", "SecondaryColour", "TertiaryColour", "BackColour", "Bold", "Italic", "BorderStyle", "Outline", "Shadow", "Alignment", "MarginL", "MarginR", "MarginV", "AlphaLevel", "Encoding"}
	assEventFormat  = []string{"Layer", "Start", "End", "Style", "Name", "MarginL", "MarginR", "MarginV", "Effect", "Text"}
	assSSAEvents    = []string{"Marked", "Start", "End", "Style", "Name", "MarginL", "MarginR", "MarginV", "Effect", "Text"}
)

func assFormat(v string) []string {
	f := strings.Split(v, ",")
	for i := range f {
		f[i] = mediaTrim(f[i])
	}
	return f
}

// assStyle reads a Style: line under its format: the written style and
// its name, or false.
func assStyle(value string, format []string, ssa bool) (string, string, bool) {
	fields := strings.Split(value, ",")
	if len(fields) != len(format) {
		return "", "", false
	}
	raw := map[string]string{}
	for i, name := range format {
		if ssa && name == "TertiaryColour" {
			name = "OutlineColour"
		}
		if _, ok := raw[name]; !ok {
			raw[name] = mediaTrim(fields[i])
		}
	}
	var out []string
	var styleName string
	for _, f := range assStyleFields {
		v, present := raw[f.name]
		w, ok := "", false
		if present {
			w, ok = f.check(v, ssa)
		}
		if !ok {
			if f.name == "Name" {
				return "", "", false
			}
			w = f.def
		}
		if f.name == "Name" {
			styleName = w
		}
		out = append(out, w)
	}
	return strings.Join(out, ","), styleName, true
}

func assDefaultStyle() string {
	var out []string
	for _, f := range assStyleFields {
		if f.name == "Name" {
			out = append(out, "Default")
		} else {
			out = append(out, f.def)
		}
	}
	return strings.Join(out, ",")
}

// rebuildASS rebuilds an ASS part, or refuses it.
func rebuildASS(in []byte) ([]byte, error) {
	if len(in) > maxASS {
		return nil, errors.New("over 5 MiB")
	}
	text := strings.ReplaceAll(strings.ReplaceAll(string(in), "\r\n", "\n"), "\r", "\n")
	text = strings.TrimPrefix(text, "\uFEFF")
	var lines []string
	for _, l := range strings.Split(text, "\n") {
		l = mediaTrim(l)
		if l == "" || strings.HasPrefix(l, ";") || strings.HasPrefix(l, "!:") {
			continue
		}
		lines = append(lines, l)
	}
	if len(lines) == 0 || lines[0] != "[Script Info]" {
		return nil, errors.New("the first line isn't [Script Info]")
	}
	// The lines of each known section, with the section they are in.
	type line struct {
		section    string // info, v4+, v4, events
		start      bool   // the section's header
		name, text string
	}
	var known []line
	section := ""
	ssa, plus := false, false
	for _, l := range lines {
		if strings.HasPrefix(l, "[") && strings.HasSuffix(l, "]") {
			switch {
			case mediaFold(l, "[Script Info]"):
				section = "info"
			case mediaFold(l, "[V4+ Styles]"):
				section, plus = "v4+", true
			case mediaFold(l, "[V4 Styles]"):
				section, ssa = "v4", true
			case mediaFold(l, "[Events]"):
				section = "events"
			default:
				section = ""
			}
			if section != "" {
				known = append(known, line{section: section, start: true})
			}
			continue
		}
		if section == "" {
			continue
		}
		i := strings.IndexByte(l, ':')
		if i < 0 {
			continue
		}
		known = append(known, line{section: section, name: l[:i], text: mediaTrim(l[i+1:])})
	}
	ssaEvents := ssa && !plus

	// Script information.
	info := map[string]string{}
	seen := map[string]bool{}
	for _, l := range known {
		if l.section != "info" || l.start {
			continue
		}
		for _, n := range assInfoNames {
			if mediaFold(l.name, n.name) && !seen[n.name] {
				seen[n.name] = true
				if n.ok(l.text) {
					info[n.name] = l.text
				}
			}
		}
	}

	// Styles.
	var styles []string
	names := map[string]bool{}
	var format []string
	for _, l := range known {
		if l.section != "v4+" && l.section != "v4" {
			continue
		}
		isSSA := l.section == "v4"
		switch {
		case l.start && isSSA:
			format = assV4Format
		case l.start:
			format = assV4PlusFormat
		case l.name == "Format":
			format = assFormat(l.text)
		case l.name == "Style":
			s, name, ok := assStyle(l.text, format, isSSA)
			if !ok || names[name] || len(styles) >= maxASSStyles {
				continue
			}
			names[name] = true
			styles = append(styles, s)
		}
	}

	// Events.
	var events []string
	for _, l := range known {
		if l.section != "events" {
			continue
		}
		switch {
		case l.start && ssaEvents:
			format = assSSAEvents
		case l.start:
			format = assEventFormat
		case l.name == "Format":
			format = assFormat(l.text)
		case l.name == "Dialogue":
			e, ok := assEvent(l.text, format, names)
			if !ok {
				continue
			}
			if len(events) >= maxASSEvents {
				return nil, errors.New("over 50,000 events")
			}
			events = append(events, e)
		}
	}

	var out strings.Builder
	out.WriteString("[Script Info]\nScriptType: v4.00+\n")
	for _, n := range assInfoNames {
		if v, ok := info[n.name]; ok {
			out.WriteString(n.name + ": " + v + "\n")
		}
	}
	out.WriteString("\n[V4+ Styles]\nFormat: " + strings.Join(assV4PlusFormat, ", ") + "\n")
	if !names["Default"] {
		out.WriteString("Style: " + assDefaultStyle() + "\n")
	}
	for _, s := range styles {
		out.WriteString("Style: " + s + "\n")
	}
	out.WriteString("\n[Events]\nFormat: " + strings.Join(assEventFormat, ", ") + "\n")
	for _, e := range events {
		out.WriteString("Dialogue: " + e + "\n")
	}
	return []byte(out.String()), nil
}

// assEvent reads a Dialogue: line: the written event, or false.
func assEvent(value string, format []string, styles map[string]bool) (string, bool) {
	if len(format) == 0 {
		return "", false
	}
	parts := strings.SplitN(value, ",", len(format))
	if len(parts) != len(format) {
		return "", false
	}
	raw := map[string]string{}
	for i, name := range format[:len(format)-1] {
		if _, ok := raw[name]; !ok {
			raw[name] = mediaTrim(parts[i])
		}
	}
	text := parts[len(parts)-1]
	layer := "0"
	if v, ok := raw["Layer"]; ok && mediaInt(v, 0, 1000) {
		layer = v
	}
	start, end := raw["Start"], raw["End"]
	if !assTime.MatchString(start) || !assTime.MatchString(end) {
		return "", false
	}
	style := "Default"
	if v, ok := raw["Style"]; ok && styles[v] {
		style = v
	}
	margin := func(n string) string {
		if v, ok := raw[n]; ok && mediaInt(v, 0, 16384) {
			return v
		}
		return "0"
	}
	t := assText(text, styles)
	if t == "" || len(t) > maxASSEventText {
		return "", false
	}
	return strings.Join([]string{layer, start, end, style, "", margin("MarginL"), margin("MarginR"), margin("MarginV"), "", t}, ","), true
}

// assText rebuilds an event's text.
func assText(s string, styles map[string]bool) string {
	s = strings.Map(func(r rune) rune {
		switch {
		case r == '\t':
			return ' '
		case r == utf8.RuneError || directionControl(r) || unicode.IsControl(r):
			return -1
		}
		return r
	}, s)
	var out strings.Builder
	drawing, stopped := false, false
	blocks, tags, transforms := 0, 0, 0
	for {
		i := strings.IndexByte(s, '{')
		piece := s
		if i >= 0 {
			piece = s[:i]
		}
		if drawing {
			out.WriteString(assDrawing(piece))
		} else {
			out.WriteString(piece)
		}
		if i < 0 {
			break
		}
		j := strings.IndexByte(s[i+1:], '}')
		if j < 0 {
			break
		}
		inner := s[i+1 : i+1+j]
		s = s[i+2+j:]
		if stopped {
			continue
		}
		t := transforms
		kept, p := assBlock(inner, styles, &t)
		if len(kept) == 0 {
			continue
		}
		if blocks+1 > maxASSBlocks || tags+len(kept) > maxASSTags {
			stopped = true
			continue
		}
		blocks++
		tags += len(kept)
		transforms = t
		out.WriteString("{" + strings.Join(kept, "") + "}")
		if p >= 0 {
			drawing = p > 0
		}
	}
	return out.String()
}

// assCut cuts the tags of a block (from its first \) at each \, but not
// inside a \t's parentheses.
func assCut(s string) []string {
	var out []string
	for i := 0; i < len(s); {
		j := i + 1
		isT := strings.HasPrefix(s[j:], "t(")
		depth := 0
		for ; j < len(s); j++ {
			c := s[j]
			if c == '\\' && depth == 0 {
				break
			}
			if isT && c == '(' {
				depth++
			} else if isT && c == ')' && depth > 0 {
				depth--
			}
		}
		out = append(out, s[i+1:j])
		i = j
	}
	return out
}

// assBlock rebuilds an override block's inside: its kept tags, and the
// last \p value (-1 without one).
func assBlock(inner string, styles map[string]bool, transforms *int) ([]string, int) {
	k := strings.IndexByte(inner, '\\')
	if k < 0 {
		return nil, -1
	}
	var kept []string
	p := -1
	for _, raw := range assCut(inner[k:]) {
		t, pv, ok := assTag(raw, styles, false, transforms)
		if !ok {
			continue
		}
		if pv >= 0 {
			p = pv
		}
		kept = append(kept, t)
	}
	return kept, p
}

var assTagNames = []string{"b", "i", "u", "s", "bord", "xbord", "ybord", "be", "blur", "shad", "xshad", "yshad",
	"fn", "fs", "fscx", "fscy", "fsp", "frx", "fry", "frz", "fr", "fax", "fay", "fe",
	"c", "1c", "2c", "3c", "4c", "alpha", "1a", "2a", "3a", "4a", "an", "a", "k", "K", "kf", "ko", "q", "r",
	"pos", "org", "move", "fad", "fade", "t", "clip", "iclip", "p", "pbo"}

var assNotInT = map[string]bool{"t": true, "pos": true, "org": true, "move": true, "fad": true, "fade": true, "an": true, "a": true,
	"k": true, "K": true, "kf": true, "ko": true, "q": true, "r": true, "fn": true, "p": true, "pbo": true}

// assParens reads (a,b,…): the arguments, white space around them removed.
func assParens(args string) ([]string, bool) {
	if !strings.HasPrefix(args, "(") || !strings.HasSuffix(args, ")") {
		return nil, false
	}
	parts := strings.Split(args[1:len(args)-1], ",")
	for i := range parts {
		parts[i] = mediaTrim(parts[i])
	}
	return parts, true
}

func assAll(parts []string, ok func(string) bool) bool {
	for _, p := range parts {
		if !ok(p) {
			return false
		}
	}
	return true
}

func assCoord(s string) bool  { return assNumber(s, -100000, 100000) }
func assMillis(s string) bool { return mediaInt(s, 0, 3600000) }

// assTag checks one tag (without its \): the written tag, the \p value
// (-1 for another tag), or false.
func assTag(raw string, styles map[string]bool, inT bool, transforms *int) (string, int, bool) {
	name := ""
	for _, n := range assTagNames {
		if strings.HasPrefix(raw, n) && len(n) > len(name) {
			name = n
		}
	}
	if name == "" || inT && assNotInT[name] {
		return "", -1, false
	}
	args := raw[len(name):]
	keep := func(ok bool) (string, int, bool) { return `\` + raw, -1, ok }
	num := func(lo, hi float64) (string, int, bool) { return keep(assNumber(args, lo, hi)) }
	switch name {
	case "b":
		return keep(mediaInt(args, 0, 1000))
	case "i", "u", "s":
		return keep(args == "0" || args == "1")
	case "bord", "xbord", "ybord", "be", "blur":
		return num(0, 100)
	case "shad", "xshad", "yshad", "fax", "fay":
		return num(-100, 100)
	case "fn":
		f := assPlain(args)
		return `\fn` + f, -1, len(f) <= 64
	case "fs", "fscx", "fscy":
		return num(0, 1000)
	case "fsp":
		return num(-1000, 1000)
	case "frx", "fry", "frz", "fr":
		return num(-3600, 3600)
	case "fe":
		return keep(mediaInt(args, 0, 255))
	case "c", "1c", "2c", "3c", "4c":
		m := assTagColour.FindStringSubmatch(args)
		if m == nil {
			return "", -1, false
		}
		v, _ := strconv.ParseUint(m[1], 16, 32)
		return fmt.Sprintf(`\%s&H%06X&`, name, v), -1, true
	case "alpha", "1a", "2a", "3a", "4a":
		m := assTagAlpha.FindStringSubmatch(args)
		if m == nil {
			return "", -1, false
		}
		v, _ := strconv.ParseUint(m[1], 16, 32)
		return fmt.Sprintf(`\%s&H%02X&`, name, v), -1, true
	case "an":
		return keep(assOneOf(args, "1", "2", "3", "4", "5", "6", "7", "8", "9"))
	case "a":
		a, ok := assAlignment(args, true)
		return `\an` + a, -1, ok
	case "k", "K", "kf", "ko":
		return keep(mediaInt(args, 0, 100000))
	case "q":
		return keep(assOneOf(args, "0", "1", "2", "3"))
	case "r":
		return keep(args == "" || styles[args])
	case "p":
		if !mediaInt(args, 0, 16) {
			return "", -1, false
		}
		v, _ := strconv.Atoi(args)
		return `\p` + args, v, true
	case "pbo":
		return num(-100000, 100000)
	case "pos", "org":
		a, ok := assParens(args)
		if !ok || len(a) != 2 || !assAll(a, assCoord) {
			return "", -1, false
		}
		return `\` + name + "(" + strings.Join(a, ",") + ")", -1, true
	case "move":
		a, ok := assParens(args)
		if !ok || len(a) != 4 && len(a) != 6 || !assAll(a[:4], assCoord) || !assAll(a[4:], assMillis) {
			return "", -1, false
		}
		return `\move(` + strings.Join(a, ",") + ")", -1, true
	case "fad":
		a, ok := assParens(args)
		if !ok || len(a) != 2 || !assAll(a, assMillis) {
			return "", -1, false
		}
		return `\fad(` + strings.Join(a, ",") + ")", -1, true
	case "fade":
		a, ok := assParens(args)
		if !ok || len(a) != 7 || !assAll(a[:3], func(s string) bool { return mediaInt(s, 0, 255) }) || !assAll(a[3:], assMillis) {
			return "", -1, false
		}
		return `\fade(` + strings.Join(a, ",") + ")", -1, true
	case "clip", "iclip":
		a, ok := assParens(args)
		if !ok {
			return "", -1, false
		}
		switch len(a) {
		case 4:
			if !assAll(a, assCoord) {
				return "", -1, false
			}
			return `\` + name + "(" + strings.Join(a, ",") + ")", -1, true
		case 2:
			if !mediaInt(a[0], 1, 16) {
				return "", -1, false
			}
			d := assDrawing(a[1])
			return `\` + name + "(" + a[0] + "," + d + ")", -1, d != ""
		case 1:
			d := assDrawing(a[0])
			return `\` + name + "(" + d + ")", -1, d != ""
		}
		return "", -1, false
	case "t":
		return assTransform(args, styles, transforms)
	}
	return "", -1, false
}

// assTransform checks \t's arguments.
func assTransform(args string, styles map[string]bool, transforms *int) (string, int, bool) {
	if *transforms >= maxASSTransforms || !strings.HasPrefix(args, "(") || !strings.HasSuffix(args, ")") {
		return "", -1, false
	}
	body := args[1 : len(args)-1]
	k := strings.IndexByte(body, '\\')
	if k < 0 {
		return "", -1, false
	}
	nums := strings.Split(body[:k], ",")
	for i := range nums {
		nums[i] = mediaTrim(nums[i])
	}
	if nums[len(nums)-1] != "" {
		return "", -1, false
	}
	nums = nums[:len(nums)-1]
	switch len(nums) {
	case 0:
	case 1:
		if !assNumber(nums[0], 0, 100) {
			return "", -1, false
		}
	case 2:
		if !assAll(nums, assMillis) {
			return "", -1, false
		}
	case 3:
		if !assAll(nums[:2], assMillis) || !assNumber(nums[2], 0, 100) {
			return "", -1, false
		}
	default:
		return "", -1, false
	}
	var kept []string
	for _, raw := range assCut(mediaTrim(body[k:])) {
		if t, _, ok := assTag(raw, styles, true, transforms); ok {
			kept = append(kept, t)
		}
	}
	if len(kept) == 0 {
		return "", -1, false
	}
	*transforms++
	pre := strings.Join(nums, ",")
	if pre != "" {
		pre += ","
	}
	return `\t(` + pre + strings.Join(kept, "") + ")", -1, true
}

// assDrawing rebuilds a drawing: its commands and numbers, single spaces
// between them.
func assDrawing(s string) string {
	var out []string
	numbers := 0
	for _, f := range strings.FieldsFunc(s, mediaWhite) {
		switch {
		case assOneOf(f, "m", "n", "l", "b", "s", "p", "c"):
			out = append(out, f)
		case assCoord(f):
			if numbers++; numbers > maxASSNumbers {
				return strings.Join(out, " ")
			}
			out = append(out, f)
		}
	}
	return strings.Join(out, " ")
}

// --- PGS (video section 6.5) ---

const (
	maxPGS        = 64 << 20
	maxPGSSide    = 4096
	maxPGSObjects = 20_000
)

// checkPGS checks a PGS part whole.
func checkPGS(raw []byte) error {
	if len(raw) > maxPGS { // an empty part is valid: a stretch with no subtitles
		return errors.New("over 64 MiB")
	}
	side := func(v uint16) bool { return v >= 1 && v <= maxPGSSide }
	type object struct {
		w, h int
		data []byte
	}
	open := map[uint16]*object{}
	firsts := 0
	for pos := 0; pos < len(raw); {
		if len(raw)-pos < 13 {
			return errors.New("a segment header cut short")
		}
		if raw[pos] != 'P' || raw[pos+1] != 'G' {
			return errors.New("a segment without its PG magic")
		}
		typ := raw[pos+10]
		size := int(binary.BigEndian.Uint16(raw[pos+11:]))
		if pos+13+size > len(raw) {
			return errors.New("a payload past the end")
		}
		seg := raw[pos+13 : pos+13+size]
		pos += 13 + size
		switch typ {
		case 0x16: // presentation
			if len(seg) < 11 {
				return errors.New("a presentation under 11 bytes")
			}
			if !side(binary.BigEndian.Uint16(seg)) || !side(binary.BigEndian.Uint16(seg[2:])) {
				return errors.New("a screen size outside 1 to 4,096")
			}
			o := 11
			for range int(seg[10]) {
				if o+8 > len(seg) {
					return errors.New("composition objects past the payload")
				}
				n := 8
				if seg[o+3]&0x80 != 0 {
					n = 16
				}
				o += n
			}
			if o > len(seg) {
				return errors.New("composition objects past the payload")
			}
		case 0x17: // window
			if len(seg) < 1 || len(seg) != 1+9*int(seg[0]) {
				return errors.New("a window segment of the wrong size")
			}
		case 0x14: // palette
			if len(seg) < 2 || (len(seg)-2)%5 != 0 {
				return errors.New("a palette segment of the wrong size")
			}
		case 0x15: // object
			if len(seg) < 4 {
				return errors.New("an object segment under 4 bytes")
			}
			id, flags := binary.BigEndian.Uint16(seg), seg[3]
			if flags&0x80 != 0 {
				if len(seg) < 11 {
					return errors.New("a first fragment under 11 bytes")
				}
				w, h := binary.BigEndian.Uint16(seg[7:]), binary.BigEndian.Uint16(seg[9:])
				if !side(w) || !side(h) {
					return errors.New("an object size outside 1 to 4,096")
				}
				if firsts++; firsts > maxPGSObjects {
					return errors.New("over 20,000 objects")
				}
				open[id] = &object{w: int(w), h: int(h), data: append([]byte(nil), seg[11:]...)}
			} else {
				o := open[id]
				if o == nil {
					return errors.New("a fragment of no open object")
				}
				o.data = append(o.data, seg[4:]...)
			}
			if flags&0x40 != 0 {
				o := open[id]
				if err := runsFit(o.data, o.w, o.h); err != nil {
					return err
				}
				delete(open, id)
			}
		case 0x80: // end
			if len(seg) != 0 {
				return errors.New("an end segment that isn't empty")
			}
		default:
			return fmt.Errorf("an unknown segment type 0x%02x", typ)
		}
	}
	return nil
}

// runsFit says whether run-length data fits w by h pixels.
func runsFit(d []byte, w, h int) error {
	short := errors.New("a run cut short")
	x, y := 0, 0
	for i := 0; i < len(d); {
		b := d[i]
		i++
		if b != 0 {
			x++
		} else {
			if i >= len(d) {
				return short
			}
			f := d[i]
			i++
			if f == 0 {
				x, y = 0, y+1
				if y > h {
					return errors.New("more lines than the object's height")
				}
				continue
			}
			n := int(f & 0x3f)
			if f&0x40 != 0 {
				if i >= len(d) {
					return short
				}
				n = n<<8 | int(d[i])
				i++
			}
			if f&0x80 != 0 {
				if i >= len(d) {
					return short
				}
				i++
			}
			x += n
		}
		if x > w {
			return errors.New("a line wider than the object")
		}
	}
	return nil
}

// --- Fonts (video section 6.6) ---

const (
	maxFont       = 10 << 20
	maxFontTables = 64
)

// checkFont checks a font: its media type, or why it is refused.
func checkFont(b []byte) (string, error) {
	if len(b) > maxFont {
		return "", errors.New("step 1: over 10 MiB")
	}
	if len(b) < 4 {
		return "", errors.New("step 2: under 4 bytes")
	}
	var typ string
	switch string(b[:4]) {
	case "\x00\x01\x00\x00", "true": // true: Apple's older TrueType signature
		typ = "font/ttf"
	case "OTTO":
		typ = "font/otf"
	default:
		return "", errors.New("step 2: first four bytes neither 00 01 00 00, true nor OTTO")
	}
	if len(b) < 6 {
		return "", errors.New("step 3: no table count")
	}
	n := int(binary.BigEndian.Uint16(b[4:]))
	if n < 1 || n > maxFontTables {
		return "", errors.New("step 3: a table count outside 1 to 64")
	}
	if 12+16*n > len(b) {
		return "", errors.New("step 3: the table directory past the end")
	}
	tags := map[string]bool{}
	for i := range n {
		r := b[12+16*i:]
		tag := string(r[:4])
		for _, c := range r[:4] {
			if c < 0x20 || c > 0x7e {
				return "", errors.New("step 4: a tag byte outside 0x20 to 0x7E")
			}
		}
		if tags[tag] {
			return "", errors.New("step 4: a tag twice")
		}
		tags[tag] = true
		off, length := uint64(binary.BigEndian.Uint32(r[8:])), uint64(binary.BigEndian.Uint32(r[12:]))
		if off+length > uint64(len(b)) {
			return "", errors.New("step 4: a table past the end")
		}
	}
	for _, t := range []string{"cmap", "head", "hhea", "hmtx", "maxp", "name", "post"} {
		if !tags[t] {
			return "", fmt.Errorf("step 5: no %s table", t)
		}
	}
	if !(tags["glyf"] && tags["loca"]) && !tags["CFF "] && !tags["CFF2"] {
		return "", errors.New("step 5: neither glyf and loca, nor CFF or CFF2")
	}
	return typ, nil
}
