package main

import (
	"fmt"
	"strings"
)

// The libraries the caller asked for, in every music title case.
var musicLibraries = []string{"m1", "m2"}

// The owning server's stream root in the queue cases.
const musicStreamRoot = "https://a.example.org/api/v1/stream/"

// Made-up MBIDs for the cases (those of music section 3's example).
const (
	mbArtist  = "0a000000-0000-4000-8000-000000000001"
	mbArtist2 = "0a000000-0000-4000-8000-000000000004"
	mbAlbum   = "0b000000-0000-4000-8000-000000000002"
	mbAlbum2  = "0b000000-0000-4000-8000-000000000007"
	mbTrack   = "0c000000-0000-4000-8000-000000000003"
	mbTrack2  = "0c000000-0000-4000-8000-000000000008"
	mbTrack3  = "0c000000-0000-4000-8000-000000000009"
	mbTrack4  = "0c000000-0000-4000-8000-00000000000a"
	mbTrack5  = "0c000000-0000-4000-8000-00000000000b"
	mbTrack6  = "0c000000-0000-4000-8000-00000000000c"
	mbGroup   = "0d000000-0000-4000-8000-000000000005"
	mbRec     = "0e000000-0000-4000-8000-000000000006"
	acoustID  = "0f000000-0000-4000-8000-00000000000d"
)

// keyCase is a music title key, its verdict, and the kind it names.
type keyCase struct {
	Name   string `json:"name"`
	Text   string `json:"text"`
	Valid  bool   `json:"valid"`
	Kind   string `json:"kind,omitempty"`
	Reason string `json:"reason,omitempty"`
}

// titleCase is one entry of titles and what the caller keeps of it.
type titleCase struct {
	Name   string `json:"name"`
	Text   string `json:"text"`
	Valid  bool   `json:"valid"`
	Output string `json:"output,omitempty"`
	Reason string `json:"reason,omitempty"`
}

// musicCallCase is a request the owning server accepts (valid) or
// refuses, or an answer and what the viewer's server keeps of it.
type musicCallCase struct {
	Name    string `json:"name"`
	Kind    string `json:"kind"`
	Request string `json:"request,omitempty"`
	Text    string `json:"text"`
	Valid   bool   `json:"valid"`
	Output  string `json:"output,omitempty"`
	Reason  string `json:"reason,omitempty"`
}

type v1MusicFile struct {
	About       []string        `json:"about"`
	Libraries   []string        `json:"libraries"`
	StreamRoot  string          `json:"streamRoot"`
	Keys        []keyCase       `json:"keys"`
	Titles      []titleCase     `json:"titles"`
	LocalTitles []titleCase     `json:"localTitles"`
	Queue       []musicCallCase `json:"queue"`
	Lyrics      []musicCallCase `json:"lyrics"`
}

type musicIn struct{ name, text string }

func musicKeyCases() []keyCase {
	in := []musicIn{
		{"an artist", "artist/mb/" + mbArtist},
		{"an album (a release)", "album/mb/" + mbAlbum},
		{"a track (a release track)", "track/mb/" + mbTrack},
		{"another track", "track/mb/9f3c1e2a-7b4d-4c8e-a1f0-2d6b8e4c7a95"},
		{"a local artist", "artist/local/Xr4_k-9"},
		{"a local album", "album/local/a"},
		{"a local track, an opaque part of 64 characters", "track/local/" + strings.Repeat("Ab0_-", 12) + "Ab0_"},
		{"a local track whose opaque part looks like an MBID", "track/local/" + mbTrack},
		{"a local opaque part of 65 characters", "track/local/" + strings.Repeat("Ab0_-", 13)},
		{"an empty local opaque part", "track/local/"},
		{"a dot in a local opaque part", "track/local/a.b"},
		{"a space in a local opaque part", "track/local/a b"},
		{"a non-ASCII local opaque part", "track/local/é"},
		{"a part after a local opaque part", "track/local/abc/x"},
		{"uppercase hex", "track/mb/0C000000-0000-4000-8000-000000000003"},
		{"without hyphens", "track/mb/0c000000000040008000000000000003"},
		{"in braces", "track/mb/{0c000000-0000-4000-8000-000000000003}"},
		{"as a URN", "track/mb/urn:uuid:0c000000-0000-4000-8000-000000000003"},
		{"one character short", "track/mb/0c000000-0000-4000-8000-00000000003"},
		{"one character long", "track/mb/0c000000-0000-4000-8000-0000000000003"},
		{"a letter that isn't hex", "track/mb/0g000000-0000-4000-8000-000000000003"},
		{"hyphens in the wrong places", "track/mb/0c0000000-000-4000-8000-000000000003"},
		{"an empty identifier", "track/mb/"},
		{"a release isn't a kind: albums are", "release/mb/" + mbAlbum},
		{"a release group isn't a kind", "release-group/mb/" + mbGroup},
		{"a recording isn't a kind: tracks are", "recording/mb/" + mbRec},
		{"song isn't a kind", "song/mb/" + mbTrack},
		{"a kind in another case", "Track/mb/" + mbTrack},
		{"another scheme", "track/musicbrainz/" + mbTrack},
		{"the scheme in another case", "track/MB/" + mbTrack},
		{"the local scheme in another case", "track/Local/abc"},
		{"a video scheme", "album/tmdb/949"},
		{"a part after the identifier", "track/mb/" + mbTrack + "/s1"},
		{"a trailing slash", "track/mb/" + mbTrack + "/"},
		{"a space before", " track/mb/" + mbTrack},
		{"a video key", "movie/tmdb/949"},
	}
	var out []keyCase
	for _, c := range in {
		k, err := checkMusicKey(c.text)
		ok, r := reason(err)
		out = append(out, keyCase{Name: c.name, Text: c.text, Valid: ok, Kind: k, Reason: r})
	}
	return out
}

func musicKeep(in []musicIn) []titleCase {
	var out []titleCase
	for _, c := range in {
		got, err := keepMusicTitle([]byte(c.text), musicLibraries)
		ok, r := reason(err)
		out = append(out, titleCase{Name: c.name, Text: c.text, Valid: ok, Output: string(got), Reason: r})
	}
	return out
}

// Pieces of the title cases.
const (
	mAlbumHead  = `"key":"album/mb/` + mbAlbum + `","kind":"album","library":"m1"`
	mArtistHead = `"key":"artist/mb/` + mbArtist + `","kind":"artist","library":"m2"`
	mTrackHead  = `"key":"track/mb/` + mbTrack + `","kind":"track","library":"m1","album":"album/mb/` + mbAlbum + `"`
	mCopy       = `{"id":"c1","durationMs":308000,"container":"flac","codec":"flac","size":36214455,"etag":"7f3a9c21"}`
)

func ak(id string) string { return `"artist/mb/` + id + `"` }

// mTrack is a track with one good copy and the members given.
func mTrack(members string) string {
	if members != "" {
		members = "," + members
	}
	return `{` + mTrackHead + `,"name":"x"` + members + `,"copies":[` + mCopy + `]}`
}

// mCopies is a track with the copies given.
func mCopies(copies ...string) string {
	return `{` + mTrackHead + `,"name":"x","copies":[` + strings.Join(copies, ",") + `]}`
}

// mCopyWith is the good copy with members replaced or added: each
// "name":value; a value of "-" removes the member.
func mCopyWith(id string, members ...string) string {
	base := []string{`"id":"` + id + `"`, `"durationMs":308000`, `"container":"flac"`, `"codec":"flac"`, `"size":36214455`, `"etag":"7f3a9c21"`}
	for _, m := range members {
		name := m[:strings.Index(m, ":")]
		val := m[len(name)+1:]
		found := false
		for i, b := range base {
			if strings.HasPrefix(b, name+":") {
				found = true
				if val == "-" {
					base = append(base[:i], base[i+1:]...)
				} else {
					base[i] = m
				}
				break
			}
		}
		if !found && val != "-" {
			base = append(base, m)
		}
	}
	return "{" + strings.Join(base, ",") + "}"
}

func mAlbum(members string) string {
	return `{` + mAlbumHead + `,"name":"x",` + members + `}`
}

func musicTitleCases() []titleCase {
	var many []string
	for i := 0; i < 18; i++ {
		many = append(many, ak(fmt.Sprintf("0a000000-0000-4000-8000-0000000001%02d", i)))
	}
	many = append(many[:2], append([]string{ak(mbArtist), `"album/mb/` + mbAlbum + `"`, `"ARTIST"`, `7`, ak(mbArtist), `"artist/local/L1"`}, many[2:]...)...)
	var nine []string
	for i := 1; i <= 9; i++ {
		nine = append(nine, mCopyWith(fmt.Sprintf("c%d", i)))
	}
	in := []musicIn{
		// Section 3's example, as it is.
		{"music section 3's example album", `{"key":"album/mb/0b000000-0000-4000-8000-000000000002","kind":"album","library":"m1","name":"Bridge over Troubled Water","artists":["artist/mb/0a000000-0000-4000-8000-000000000001"],"artistNames":"Simon & Garfunkel","primaryType":"album","date":"2011-03-08","originalDate":"1970-01-26","genres":["Folk rock"],"ids":{"mbreleasegroup":"0d000000-0000-4000-8000-000000000005"}}`},
		{"music section 3's example track", `{"key":"track/mb/0c000000-0000-4000-8000-000000000003","kind":"track","library":"m1","name":"The Boxer","artists":["artist/mb/0a000000-0000-4000-8000-000000000001"],"artistNames":"Simon & Garfunkel","album":"album/mb/0b000000-0000-4000-8000-000000000002","disc":1,"number":2,"copies":[{"id":"Vq3kM0aZ7pXc","durationMs":308000,"container":"flac","codec":"flac","size":36214455,"etag":"7f3a9c21","bitrateKbps":936,"sampleRate":44100,"bitDepth":16,"channels":2,"replayGain":{"trackGain":-4.12,"trackPeak":0.988525,"albumGain":-5.3,"albumPeak":1.0},"lyrics":"synced"}],"ids":{"mbrecording":"0e000000-0000-4000-8000-000000000006","isrc":"USSM17000123"}}`},
		{"an artist", `{` + mArtistHead + `,"name":"Simon & Garfunkel"}`},
		{"an album with every member", mAlbum(`"sortName":"Bridge over Troubled Water, The","artists":[` + ak(mbArtist) + `,` + ak(mbArtist2) + `],"artistNames":"A & B","primaryType":"album","secondaryTypes":["compilation","live"],"compilation":true,"date":"2011","originalDate":"1970-01","genres":["Folk rock","Pop"],"ids":{"mbreleasegroup":"` + mbGroup + `"}`)},

		// Names.
		{"names cleaned: white space runs, controls and format characters", `{` + mTrackHead + `,"name":"  The\t\tBoxer\n\u0007(live)‮ ","artistNames":" Simon &​Garfunkel ","copies":[` + mCopy + `]}`},
		{"a format character between spaces leaves two", `{` + mArtistHead + `,"name":"a ​ b"}`},
		{"a name cut to 256 code points, then trimmed", `{` + mArtistHead + `,"name":"` + strings.Repeat("é", 255) + ` b"}`},
		{"a name cut to 256 code points", `{` + mArtistHead + `,"name":"` + strings.Repeat("é", 255) + `ab"}`},
		{"artistNames cut to 512 code points", mTrack(`"artistNames":"` + strings.Repeat("a", 511) + `bc"`)},
		{"sortName cut to 256 code points", mAlbum(`"sortName":"` + strings.Repeat("s", 255) + `tu"`)},
		{"sortName and artistNames empty once cleaned: absent", mAlbum(`"sortName":" ​ ","artistNames":"\t"`)},
		{"sortName and artistNames that aren't strings: absent", mAlbum(`"sortName":["x"],"artistNames":7`)},
		{"no name: skipped", `{` + mArtistHead + `}`},
		{"a name empty once cleaned: skipped", `{` + mArtistHead + `,"name":" ​\t "}`},
		{"a name that isn't a string: skipped", `{` + mArtistHead + `,"name":7}`},
		{"a null name: skipped", `{` + mArtistHead + `,"name":null}`},

		// Catalog section 3.3.
		{"a kind member, not the key's: no title kind (catalog section 3), ignored", `{"key":"artist/mb/` + mbArtist + `","kind":"album","library":"m1","name":"x"}`},
		{"a library not asked for: skipped", `{"key":"artist/mb/` + mbArtist + `","kind":"artist","library":"m3","name":"x"}`},
		{"a key in uppercase: skipped", `{"key":"artist/mb/0A000000-0000-4000-8000-000000000001","kind":"artist","library":"m1","name":"x"}`},
		{"an unknown scheme: skipped", `{"key":"artist/discogs/123","kind":"artist","library":"m1","name":"x"}`},
		{"a video key: skipped", `{"key":"movie/tmdb/949","kind":"movie","library":"m1","name":"x"}`},
		{"a key that isn't a string: skipped", `{"key":7,"kind":"artist","library":"m1","name":"x"}`},
		{"a library that isn't a string: skipped", `{"key":"artist/mb/` + mbArtist + `","kind":"artist","library":["m1"],"name":"x"}`},
		{"not an object: skipped", `"artist/mb/` + mbArtist + `"`},

		// artists.
		{"artists: bad, repeated and non-string entries skipped, local keys kept, the first 16 kept", mTrack(`"artists":[` + strings.Join(many, ",") + `]`)},
		{"artists that isn't an array: absent", mTrack(`"artists":` + ak(mbArtist))},
		{"artists empty once bad entries are skipped: absent", mAlbum(`"artists":["artist/mb/x",7]`)},

		// primaryType, secondaryTypes and compilation.
		{"primaryType single, secondaryTypes soundtrack and remix", mAlbum(`"primaryType":"single","secondaryTypes":["soundtrack","remix"]`)},
		{"MusicBrainz's other primary types kept: broadcast", mAlbum(`"primaryType":"broadcast"`)},
		{"primaryType other is a MusicBrainz type, kept", mAlbum(`"primaryType":"other"`)},
		{"an unknown primaryType kept as it is", mAlbum(`"primaryType":"box-set"`)},
		{"types with a space and a slash kept", mAlbum(`"primaryType":"ep","secondaryTypes":["audio drama","mixtape/street","dj-mix","spokenword"]`)},
		{"a type of 32 characters kept, of 33 absent", mAlbum(`"primaryType":"` + strings.Repeat("a", 32) + `","secondaryTypes":["` + strings.Repeat("b", 33) + `","live"]`)},
		{"a primaryType in another case: absent, never lowercased by the caller", mAlbum(`"primaryType":"Album"`)},
		{"an empty primaryType: absent", mAlbum(`"primaryType":""`)},
		{"a primaryType with a space at an end: absent", mAlbum(`"primaryType":" album"`)},
		{"a primaryType of a space only: absent", mAlbum(`"primaryType":" "`)},
		{"a primaryType with other characters: absent", mAlbum(`"primaryType":"album+live"`)},
		{"a primaryType that isn't a string: absent", mAlbum(`"primaryType":1`)},
		{"secondaryTypes: bad, repeated and non-string entries skipped alone", mAlbum(`"secondaryTypes":["live","Live","live",7,"","remix "," remix","remix"]`)},
		{"secondaryTypes: at most 8 kept, the first, skipped ones not counted", mAlbum(`"secondaryTypes":["a","B","b","c","d","e","f","g","h","i"]`)},
		{"secondaryTypes empty once bad entries are skipped: absent", mAlbum(`"secondaryTypes":["Live",7]`)},
		{"secondaryTypes that isn't an array: absent", mAlbum(`"secondaryTypes":"live"`)},
		{"releaseType isn't a member any more: ignored", mAlbum(`"releaseType":"album"`)},
		{"compilation false is kept", mAlbum(`"compilation":false`)},
		{"compilation that isn't a boolean: absent", mAlbum(`"compilation":"true"`)},

		// Dates.
		{"a date of a year and a month", mAlbum(`"date":"1970-01"`)},
		{"dates at the years' bounds", mAlbum(`"date":"9999-12-31","originalDate":"1000"`)},
		{"a year before 1000: absent", mAlbum(`"date":"0999"`)},
		{"a year of 5 digits: absent", mAlbum(`"date":"10000"`)},
		{"month 00 and month 13: absent", mAlbum(`"date":"2011-00","originalDate":"1970-13"`)},
		{"29 February in a leap year", mAlbum(`"date":"2024-02-29","originalDate":"2000-02-29"`)},
		{"29 February outside leap years: absent", mAlbum(`"date":"2023-02-29","originalDate":"1900-02-29"`)},
		{"31 April and day 00: absent", mAlbum(`"date":"2024-04-31","originalDate":"1970-01-00"`)},
		{"dates out of form: absent", mAlbum(`"date":"2024-1-05","originalDate":"19700126"`)},
		{"a date with a time: absent", mAlbum(`"date":"2011-03-08T00:00:00Z"`)},
		{"a date that isn't a string: absent", mAlbum(`"date":2011,"originalDate":"1970"`)},
		{"an originalDate whose year is after date's: absent", mAlbum(`"date":"1970","originalDate":"1971-01-01"`)},
		{"an originalDate later in date's year is kept (years compared)", mAlbum(`"date":"1970-01-01","originalDate":"1970-12-31"`)},
		{"an originalDate without a date is kept", mAlbum(`"originalDate":"1970"`)},
		{"an originalDate beside a date that counts as absent is kept", mAlbum(`"date":"1969-02-30","originalDate":"1970"`)},

		// Genres.
		{"genres: cleaned; empty, over 64, repeated (without case) and non-string entries skipped alone", mAlbum(`"genres":[" Folk\t rock ","","​","` + strings.Repeat("g", 64) + `","` + strings.Repeat("h", 65) + `","folk ROCK",7,"Pop"]`)},
		{"genres: at most 8 kept, the first", mAlbum(`"genres":["a","b","c","d","A","e","f","g","h","i"]`)},
		{"genres: 64 code points once cleaned is kept", mAlbum(`"genres":["  ` + strings.Repeat("é", 64) + `  "]`)},
		{"genres that isn't an array: absent", mAlbum(`"genres":"Pop"`)},

		// album, disc, number.
		{"a track/mb track without album: skipped", `{"key":"track/mb/` + mbTrack + `","kind":"track","library":"m1","name":"x","copies":[` + mCopy + `]}`},
		{"a track/mb track on a local album: skipped", `{"key":"track/mb/` + mbTrack + `","kind":"track","library":"m1","name":"x","album":"album/local/A1","copies":[` + mCopy + `]}`},
		{"a track/mb track whose album isn't an album key: skipped", `{"key":"track/mb/` + mbTrack + `","kind":"track","library":"m1","name":"x","album":"artist/mb/` + mbArtist + `","copies":[` + mCopy + `]}`},
		{"a track/mb track whose album isn't a string: skipped", `{"key":"track/mb/` + mbTrack + `","kind":"track","library":"m1","name":"x","album":7,"copies":[` + mCopy + `]}`},
		{"disc and number at their bounds", mTrack(`"disc":999,"number":1`)},
		{"disc and number at their other bounds", mTrack(`"disc":1,"number":999`)},
		{"disc and number out of range: absent", mTrack(`"disc":0,"number":1000`)},
		{"disc and number that aren't MSFP integers: absent", mTrack(`"disc":1.0,"number":"2"`)},
		{"disc -0: absent", mTrack(`"disc":-0,"number":1e0`)},

		// Members of other kinds.
		{"an album ignores a track's members", mAlbum(`"album":"album/mb/` + mbAlbum2 + `","disc":1,"number":1,"copies":[` + mCopy + `],"image":true,"ids":{"mbrecording":"` + mbRec + `","isrc":"USSM17000123","acoustid":"` + acoustID + `"}`)},
		{"a track ignores an album's members", mTrack(`"sortName":"y","primaryType":"album","secondaryTypes":["live"],"compilation":true,"image":true,"ids":{"mbreleasegroup":"` + mbGroup + `"}`)},
		{"a track's own dates and genres kept", mTrack(`"date":"1999-05","originalDate":"1968","genres":["Soul"," Funk ","soul"]`)},
		{"a track's dates checked as an album's", mTrack(`"date":"2023-02-29","originalDate":"1970-13"`)},
		{"a track's originalDate after its date: absent", mTrack(`"date":"1970","originalDate":"1971"`)},
		{"a track's genres: at most 8 kept, the first", mTrack(`"genres":["a","b","c","d","e","f","g","h","i"]`)},
		{"a track's dates and genres of the wrong type: absent", mTrack(`"date":1999,"originalDate":["1968"],"genres":"Soul"`)},
		{"an artist carries no artists, artistNames, album members, dates, genres or image", `{` + mArtistHead + `,"name":"x","artists":[` + ak(mbArtist2) + `],"artistNames":"y","sortName":"z","primaryType":"album","genres":["Pop"],"date":"1970","image":true,"copies":[` + mCopy + `],"ids":{"mbreleasegroup":"` + mbGroup + `"}}`},

		// ids.
		{"ids of a track: mbrecording, isrc, acoustid", mTrack(`"ids":{"mbrecording":"` + mbRec + `","isrc":"GBAYE6300001","acoustid":"` + acoustID + `"}`)},
		{"ids out of form, of other kinds, of unknown or old schemes, and non-strings skipped", mTrack(`"ids":{"mbrecording":"0E000000-0000-4000-8000-000000000006","isrc":"ussm17000123","acoustid":"` + acoustID + `x","mbreleasegroup":"` + mbGroup + `","mbrelease":"` + mbAlbum + `","mbtrack":"` + mbTrack + `","discogs":"123"}`)},
		{"ids: an ISRC of 11 characters skipped alone", mTrack(`"ids":{"isrc":"USSM1700012","mbrecording":"` + mbRec + `"}`)},
		{"ids: an ISRC whose registrant code has digits", mTrack(`"ids":{"isrc":"US1M27000123"}`)},
		{"ids: an ISRC with a letter in its last 7 characters skipped", mTrack(`"ids":{"isrc":"USSM1A000123"}`)},
		{"ids: values that aren't strings skipped alone", mTrack(`"ids":{"isrc":7,"mbrecording":"` + mbRec + `","acoustid":null}`)},
		{"ids that isn't an object: absent", mTrack(`"ids":["` + mbRec + `"]`)},

		// Copies: the track.
		{"a track without copies: skipped", `{` + mTrackHead + `,"name":"x"}`},
		{"copies that isn't an array: skipped", `{` + mTrackHead + `,"name":"x","copies":` + mCopy + `}`},
		{"empty copies: skipped", mCopies()},
		{"no copy left: skipped", mCopies(mCopyWith("c1", `"durationMs":0`), `"c2"`)},

		// Copies: required members.
		{"a copy without id skipped alone", mCopies(mCopyWith("c1", `"id":-`), mCopyWith("c2"))},
		{"a copy without durationMs skipped alone", mCopies(mCopyWith("c1", `"durationMs":-`), mCopyWith("c2"))},
		{"a copy without container skipped alone", mCopies(mCopyWith("c1", `"container":-`), mCopyWith("c2"))},
		{"a copy without codec skipped alone", mCopies(mCopyWith("c1", `"codec":-`), mCopyWith("c2"))},
		{"a copy without size skipped alone", mCopies(mCopyWith("c1", `"size":-`), mCopyWith("c2"))},
		{"a copy without etag skipped alone", mCopies(mCopyWith("c1", `"etag":-`), mCopyWith("c2"))},
		{"a copy that isn't an object skipped alone", mCopies(`"c1"`, `[]`, mCopyWith("c2"))},
		{"ids of 64 characters, 65, with a space, and a repeat", mCopies(mCopyWith(strings.Repeat("i", 64)), mCopyWith(strings.Repeat("j", 65)), mCopyWith("a b"), mCopyWith(strings.Repeat("i", 64), `"etag":"other"`), mCopyWith("c2"))},
		{"durationMs at its bounds", mCopies(mCopyWith("c1", `"durationMs":1`), mCopyWith("c2", `"durationMs":86400000`))},
		{"durationMs out of range or not an integer: the copy skipped", mCopies(mCopyWith("c1", `"durationMs":0`), mCopyWith("c2", `"durationMs":86400001`), mCopyWith("c3", `"durationMs":1.5`), mCopyWith("c4", `"durationMs":"308000"`), mCopyWith("c5"))},
		{"size at its bounds", mCopies(mCopyWith("c1", `"size":1`), mCopyWith("c2", `"size":999999999999999`))},
		{"size out of range: the copy skipped", mCopies(mCopyWith("c1", `"size":0`), mCopyWith("c2", `"size":1000000000000000`), mCopyWith("c3", `"size":1e3`), mCopyWith("c4"))},
		{"etags of 64 characters, 65, empty, with a dot", mCopies(mCopyWith("c1", `"etag":"`+strings.Repeat("e", 64)+`"`), mCopyWith("c2", `"etag":"`+strings.Repeat("e", 65)+`"`), mCopyWith("c3", `"etag":""`), mCopyWith("c4", `"etag":"7f.3"`), mCopyWith("c5", `"etag":7`))},
		{"ffmpeg names of containers and codecs kept", mCopies(mCopyWith("c1", `"container":"mp3"`, `"codec":"mp3"`), mCopyWith("c2", `"container":"mp4"`, `"codec":"alac"`), mCopyWith("c3", `"container":"ogg"`, `"codec":"vorbis"`), mCopyWith("c4", `"container":"wav"`, `"codec":"pcm_s24le"`), mCopyWith("c5", `"container":"matroska"`, `"codec":"opus"`))},
		{"container and codec tokens kept as they are, known or not, never turned into other", mCopies(mCopyWith("c1", `"container":"ipod"`, `"codec":"wmav2"`), mCopyWith("c2", `"container":"mka"`, `"codec":"h264"`), mCopyWith("c3", `"container":"x.new_1-2"`, `"codec":"iamf"`), mCopyWith("c4", `"container":"other"`, `"codec":"other"`))},
		{"a container and a codec of 32 characters kept", mCopies(mCopyWith("c1", `"container":"`+strings.Repeat("c", 32)+`"`, `"codec":"`+strings.Repeat("k", 32)+`"`))},
		{"a container or codec that isn't a token: the copy skipped", mCopies(mCopyWith("c1", `"container":"FLAC"`), mCopyWith("c2", `"codec":"pcm s16le"`), mCopyWith("c3", `"container":""`), mCopyWith("c4", `"codec":"`+strings.Repeat("k", 33)+`"`), mCopyWith("c5", `"container":"m4a,mp4"`), mCopyWith("c6"))},
		{"a container or codec that isn't a string: the copy skipped", mCopies(mCopyWith("c1", `"container":7`), mCopyWith("c2", `"codec":["flac"]`), mCopyWith("c3"))},
		{"codecs, RFC 6381 strings, kept", mCopies(mCopyWith("c1", `"codecs":"fLaC"`), mCopyWith("c2", `"container":"mp4"`, `"codec":"aac"`, `"codecs":"mp4a.40.2"`), mCopyWith("c3", `"container":"mp4"`, `"codec":"eac3"`, `"codecs":"ec-3"`))},
		{"codecs of 64 characters kept, of 65 absent", mCopies(mCopyWith("c1", `"codecs":"`+strings.Repeat("a", 64)+`"`), mCopyWith("c2", `"codecs":"`+strings.Repeat("a", 65)+`"`))},
		{"codecs empty, with a comma, a space or a plus, or not a string: absent", mCopies(mCopyWith("c1", `"codecs":""`), mCopyWith("c2", `"codecs":"mp4a.40.2,avc1"`), mCopyWith("c3", `"codecs":"mp4a 40"`), mCopyWith("c4", `"codecs":"a+b"`), mCopyWith("c5", `"codecs":7`))},
		{"at most 8 copies kept, the first", mCopies(append([]string{`7`}, nine...)...)},
		{"at most 8 copies kept, skipped ones not counted", mCopies(append([]string{mCopyWith("bad", `"etag":-`)}, nine[:8]...)...)},

		// Copies: optional members.
		{"a copy's name cleaned and cut to 64 code points", mCopies(mCopyWith("c1", `"name":" Hi-Res\t\t`+strings.Repeat("x", 62)+`"`), mCopyWith("c2", `"name":"Vinyl rip"`))},
		{"a copy's name empty once cleaned or not a string: absent", mCopies(mCopyWith("c1", `"name":" ​ "`), mCopyWith("c2", `"name":7`))},
		{"added: a valid date kept", mCopies(mCopyWith("c1", `"added":"2024-02-29"`))},
		{"added out of form or not a real day: absent", mCopies(mCopyWith("c1", `"added":"2023-02-29"`), mCopyWith("c2", `"added":"2024-2-01"`), mCopyWith("c3", `"added":"2024-02"`), mCopyWith("c4", `"added":20240201`))},
		{"bitrateKbps kept exact, never rounded", mCopies(mCopyWith("c1", `"bitrateKbps":941`), mCopyWith("c2", `"bitrateKbps":15`), mCopyWith("c3", `"bitrateKbps":321`))},
		{"bitrateKbps at its bounds", mCopies(mCopyWith("c1", `"bitrateKbps":1`), mCopyWith("c2", `"bitrateKbps":100000`))},
		{"bitrateKbps out of range or not an integer: absent", mCopies(mCopyWith("c1", `"bitrateKbps":0`), mCopyWith("c2", `"bitrateKbps":100001`), mCopyWith("c3", `"bitrateKbps":-8`), mCopyWith("c4", `"bitrateKbps":320.0`))},
		{"durationMs kept exact, to the millisecond", mCopies(mCopyWith("c1", `"durationMs":308417`), mCopyWith("c2", `"durationMs":1001`))},
		{"sampleRate, bitDepth and channels at their lower bounds", mCopies(mCopyWith("c1", `"sampleRate":8000`, `"bitDepth":8`, `"channels":1`))},
		{"sampleRate, bitDepth and channels at their upper bounds", mCopies(mCopyWith("c1", `"sampleRate":768000`, `"bitDepth":32`, `"channels":64`))},
		{"sampleRate, bitDepth and channels below their ranges: absent", mCopies(mCopyWith("c1", `"sampleRate":7999`, `"bitDepth":7`, `"channels":0`))},
		{"sampleRate, bitDepth and channels above their ranges: absent", mCopies(mCopyWith("c1", `"sampleRate":768001`, `"bitDepth":33`, `"channels":65`))},
		{"sampleRate, bitDepth and channels that aren't integers: absent", mCopies(mCopyWith("c1", `"sampleRate":44100.0`, `"bitDepth":"16"`, `"channels":true`))},
		{"bitDepth on a lossy codec is kept: the caller checks only its range", mCopies(mCopyWith("c1", `"codec":"mp3"`, `"container":"mp3"`, `"bitDepth":16`))},
		{"replayGain at its bounds", mCopies(mCopyWith("c1", `"replayGain":{"trackGain":-64,"trackPeak":0,"albumGain":64,"albumPeak":10}`))},
		{"replayGain rounded: gains to two decimals, peaks to six", mCopies(mCopyWith("c1", `"replayGain":{"trackGain":-4.126,"trackPeak":0.9885254,"albumGain":5.304,"albumPeak":1.0000006}`))},
		{"replayGain in exponent form, and a gain rounding to zero", mCopies(mCopyWith("c1", `"replayGain":{"trackGain":-0.001,"trackPeak":9.5e-1,"albumGain":1E1,"albumPeak":1e-7}`))},
		{"replayGain members out of range: absent alone", mCopies(mCopyWith("c1", `"replayGain":{"trackGain":-64.01,"trackPeak":-0.000001,"albumGain":64.001,"albumPeak":1.5}`))},
		{"replayGain peaks above 10: absent", mCopies(mCopyWith("c1", `"replayGain":{"trackPeak":10.0000001,"albumGain":-1}`))},
		{"replayGain members that aren't numbers: absent", mCopies(mCopyWith("c1", `"replayGain":{"trackGain":"-4.12","trackPeak":true,"albumGain":[1],"albumPeak":1e999}`))},
		{"a replayGain left empty: absent", mCopies(mCopyWith("c1", `"replayGain":{"trackGain":99}`), mCopyWith("c2", `"replayGain":{}`))},
		{"a replayGain that isn't an object: absent", mCopies(mCopyWith("c1", `"replayGain":[-4.12]`))},
		{"lyrics plain and synced", mCopies(mCopyWith("c1", `"lyrics":"plain"`), mCopyWith("c2", `"lyrics":"synced"`))},
		{"lyrics of another value or type: absent", mCopies(mCopyWith("c1", `"lyrics":"Synced"`), mCopyWith("c2", `"lyrics":"none"`), mCopyWith("c3", `"lyrics":true`))},
		{"video's members of a copy ignored on a track", mCopies(mCopyWith("c1", `"edition":"remastered"`, `"startMs":0`, `"parts":[{"size":36214455,"durationMs":308000}]`, `"video":{"codec":"h264","width":640,"height":480}`, `"audioTracks":[{"id":"a1","codec":"flac"}]`, `"subtitleTracks":[{"id":"s1","format":"subrip"}]`, `"chapters":[{"startMs":0}]`, `"segments":[]`, `"previews":true`, `"threeD":"sbs"`))},
	}
	return musicKeep(in)
}

func musicLocalTitleCases() []titleCase {
	in := []musicIn{
		{"a local artist with an image", `{"key":"artist/local/Ar7","kind":"artist","library":"m1","name":"The Garage Band","image":true}`},
		{"a local album without an image (false kept)", `{"key":"album/local/Al_1","kind":"album","library":"m1","name":"Demos","artists":["artist/local/Ar7"],"artistNames":"The Garage Band","primaryType":"album","secondaryTypes":["demo"],"date":"2019","genres":["Rock"],"image":false}`},
		{"a local album may carry its release group", `{"key":"album/local/Al_2","kind":"album","library":"m1","name":"Bootleg","ids":{"mbreleasegroup":"` + mbGroup + `"}}`},
		{"an image that isn't a boolean: absent", `{"key":"artist/local/Ar8","kind":"artist","library":"m1","name":"x","image":"true"}`},
		{"a local track carries no image", `{"key":"track/local/T-1","kind":"track","library":"m1","name":"Song","image":true,"copies":[` + mCopy + `]}`},
		{"an mb artist's image ignored", `{` + mArtistHead + `,"name":"x","image":true}`},
		{"an mb album's image ignored", mAlbum(`"image":true`)},
		{"a local track without an album, with an ISRC and an AcoustID", `{"key":"track/local/T-2","kind":"track","library":"m1","name":"Home recording","artists":["artist/local/Ar7"],"disc":1,"number":3,"copies":[` + mCopy + `],"ids":{"isrc":"FRZ039900001","acoustid":"` + acoustID + `","mbrecording":"` + mbRec + `"}}`},
		{"a local track on an mb album", `{"key":"track/local/T-3","kind":"track","library":"m1","name":"Bonus","album":"album/mb/` + mbAlbum + `","copies":[` + mCopy + `]}`},
		{"a local track on a local album", `{"key":"track/local/T-4","kind":"track","library":"m1","name":"Take 2","album":"album/local/Al_1","copies":[` + mCopy + `]}`},
		{"a local track whose album isn't an album key: album absent", `{"key":"track/local/T-5","kind":"track","library":"m1","name":"x","album":"track/local/T-4","copies":[` + mCopy + `]}`},
		{"an mb album crediting a local artist", mAlbum(`"artists":["artist/local/Ar7",` + ak(mbArtist) + `]`)},
		{"a local name keeps music's 256 code points, not the description's 128", `{"key":"artist/local/Ar9","kind":"artist","library":"m1","name":"` + strings.Repeat("n", 200) + `"}`},
		{"a local name cut to 256 code points", `{"key":"artist/local/Ar9","kind":"artist","library":"m1","name":"` + strings.Repeat("n", 257) + `"}`},
		{"year isn't a music member: ignored", `{"key":"album/local/Al_3","kind":"album","library":"m1","name":"x","year":2019}`},
		{"an opaque part of 64 characters", `{"key":"artist/local/` + strings.Repeat("Z", 64) + `","kind":"artist","library":"m1","name":"x"}`},
		{"an opaque part of 65 characters: skipped", `{"key":"artist/local/` + strings.Repeat("Z", 65) + `","kind":"artist","library":"m1","name":"x"}`},
		{"an opaque part with a dot: skipped", `{"key":"album/local/a.b","kind":"album","library":"m1","name":"x"}`},
		{"an empty opaque part: skipped", `{"key":"album/local/","kind":"album","library":"m1","name":"x"}`},
		{"a local title without a name: skipped", `{"key":"artist/local/Ar7","kind":"artist","library":"m1","image":true}`},
		{"a local track without copies: skipped", `{"key":"track/local/T-6","kind":"track","library":"m1","name":"x"}`},
		{"a local key with a kind member not its own: ignored", `{"key":"album/local/Al_1","kind":"artist","library":"m1","name":"x"}`},
	}
	return musicKeep(in)
}

func musicQueueCases() []musicCallCase {
	t := func(id string) string { return `"track/mb/` + id + `"` }
	req := func(members string) string { return `{"title":` + t(mbTrack) + members + `}` }
	var out []musicCallCase
	for _, c := range []musicIn{
		{"no queue", req(``)},
		{"an empty queue", req(`,"queue":[]`)},
		{"one track", req(`,"queue":[` + t(mbTrack2) + `]`)},
		{"5 tracks, local ones among them", req(`,"queue":[` + t(mbTrack2) + `,"track/local/T-1",` + t(mbTrack3) + `,` + t(mbTrack4) + `,"track/local/T-2"]`)},
		{"a local track's queue", `{"title":"track/local/T-1","queue":[` + t(mbTrack) + `]}`},
		{"6 tracks: refused", req(`,"queue":[` + t(mbTrack2) + `,` + t(mbTrack3) + `,` + t(mbTrack4) + `,` + t(mbTrack5) + `,` + t(mbTrack6) + `,"track/local/T-1"]`)},
		{"an entry that isn't a string: refused", req(`,"queue":[` + t(mbTrack2) + `,7]`)},
		{"an entry in uppercase: refused", req(`,"queue":["track/mb/0C000000-0000-4000-8000-000000000008"]`)},
		{"an album in the queue: refused", req(`,"queue":["album/mb/` + mbAlbum + `"]`)},
		{"an entry with a trailing slash: refused", req(`,"queue":["track/mb/` + mbTrack2 + `/"]`)},
		{"title itself in the queue: refused", req(`,"queue":[` + t(mbTrack2) + `,` + t(mbTrack) + `]`)},
		{"a repeated entry: refused", req(`,"queue":[` + t(mbTrack2) + `,` + t(mbTrack3) + `,` + t(mbTrack2) + `]`)},
		{"a queue that isn't an array: refused", req(`,"queue":` + t(mbTrack2))},
		{"a null queue: refused", req(`,"queue":null`)},
		{"a queue after an album's title: refused", `{"title":"album/mb/` + mbAlbum + `","queue":[` + t(mbTrack2) + `]}`},
	} {
		err := musicCheckQueueRequest([]byte(c.text))
		ok, r := reason(err)
		out = append(out, musicCallCase{Name: c.name, Kind: "request", Text: c.text, Valid: ok, Reason: r})
	}

	three := req(`,"queue":[` + t(mbTrack2) + `,` + t(mbTrack3) + `,` + t(mbTrack4) + `]`)
	// Every entry's stream is under the playback's own ticket, Tk1, at
	// queue/<n>, n its place in the request's queue.
	at := func(ticket, n, name string) string { return musicStreamRoot + ticket + "/queue/" + n + "/" + name }
	hls := func(n string) string {
		return `{"url":"` + at("Tk1", n, "master.m3u8") + `","protocol":"hls","mimeType":"application/vnd.apple.mpegurl"}`
	}
	file := func(n string) string { return `{"url":"` + at("Tk1", n, "file") + `","protocol":"file"}` }
	url := func(u, proto string) string { return `{"url":"` + u + `","protocol":"` + proto + `"}` }
	entry := func(id, method, stream, more string) string {
		return `{"title":` + t(id) + `,"versionId":"v1","method":"` + method + `","stream":` + stream + more + `}`
	}
	e2 := entry(mbTrack2, "transcode", hls("0"), `,"durationMs":201000`)
	e3 := entry(mbTrack3, "direct", file("1"), ``)
	e4 := entry(mbTrack4, "remux", url(at("Tk1", "2", "master.m3u8"), "hls"), `,"durationMs":0`)
	own := `"stream":{"url":"` + musicStreamRoot + `Tk1/file","protocol":"file"}`
	ansOwn := func(stream string, entries ...string) string {
		return `{"id":"play_` + strings.Repeat("a", 26) + `","method":"direct",` + stream + `,"queue":[` + strings.Join(entries, ",") + `]}`
	}
	ans := func(entries ...string) string { return ansOwn(own, entries...) }
	answers := []struct{ name, request, text string }{
		{"three entries prepared, in order", three, ans(e2, e3, e4)},
		{"an entry the owning server left out: the others keep their places", three, ans(e2, e4)},
		{"an empty queue in the answer", three, ans()},
		{"no queue in the answer", three, `{"id":"play_` + strings.Repeat("a", 26) + `",` + own + `}`},
		{"a queue that isn't an array: absent", three, `{` + own + `,"queue":` + e2 + `}`},
		{"an entry that isn't an object dropped alone", three, ans(`"x"`, e2)},
		{"out of the request's order: the later entry dropped", three, ans(e4, e3)},
		{"a repeated entry dropped", three, ans(e2, e2, e3)},
		{"an entry not in the request's queue dropped", three, ans(entry(mbTrack5, "direct", file("3"), ``), e3)},
		{"the request's title as an entry dropped", three, ans(entry(mbTrack, "direct", file("0"), ``), e2)},
		{"a bad versionId dropped", three, ans(strings.Replace(e2, `"v1"`, `"v 1"`, 1), e3)},
		{"a missing versionId dropped", three, ans(strings.Replace(e2, `"versionId":"v1",`, ``, 1), e3)},
		{"an unknown method dropped", three, ans(strings.Replace(e2, `"transcode"`, `"copy"`, 1), e3)},
		{"a missing stream dropped", three, ans(`{"title":`+t(mbTrack2)+`,"versionId":"v1","method":"direct"}`, e3)},
		{"a stream under another root dropped", three, ans(entry(mbTrack2, "direct", url("https://b.example.org/api/v1/stream/Tk1/queue/0/file", "file"), ``), e3)},
		{"a stream over http dropped", three, ans(entry(mbTrack2, "direct", url("http://a.example.org/api/v1/stream/Tk1/queue/0/file", "file"), ``), e3)},
		{"a stream URL with another file shape dropped", three, ans(entry(mbTrack2, "direct", url(at("Tk1", "0", "x/file"), "file"), ``), e3)},
		{"a stream URL with a .. part dropped", three, ans(entry(mbTrack2, "direct", url(musicStreamRoot+"Tk1/queue/../file", "file"), ``), e3)},
		{"a stream URL with a query or a %", three, ans(entry(mbTrack2, "direct", url(at("Tk1", "0", "file?x=1"), "file"), ``), entry(mbTrack3, "direct", url(at("Tk1", "%31", "file"), "file"), ``), e4)},
		{"the playback's own stream URL, without queue/<n>, dropped", three, ans(entry(mbTrack2, "direct", url(musicStreamRoot+"Tk1/file", "file"), ``), e3)},
		{"an entry under another ticket than the playback's dropped", three, ans(entry(mbTrack2, "direct", url(at("Tk2", "0", "file"), "file"), ``), e3)},
		{"an entry at another entry's place dropped", three, ans(entry(mbTrack2, "direct", file("1"), ``), entry(mbTrack3, "direct", file("2"), ``), e4)},
		{"an entry at its place in the answer, not the request's, dropped", three, ans(e2, entry(mbTrack4, "remux", url(at("Tk1", "1", "master.m3u8"), "hls"), ``))},
		{"<n> with a leading zero, or out of 0 to 4, dropped", three, ans(entry(mbTrack2, "direct", file("00"), ``), entry(mbTrack3, "direct", file("01"), ``), entry(mbTrack4, "direct", file("5"), ``))},
		{"<n> missing dropped", three, ans(entry(mbTrack2, "direct", url(musicStreamRoot+"Tk1/queue/file", "file"), ``), e3)},
		{"an answer whose own stream names no ticket: every entry dropped", three, ansOwn(`"stream":{"url":"https://b.example.org/api/v1/stream/Tk1/file","protocol":"file"}`, e2, e3)},
		{"an answer without its own stream: every entry dropped", three, `{"id":"play_` + strings.Repeat("a", 26) + `","method":"direct","queue":[` + e2 + `]}`},
		{"an answer whose own stream is HLS: entries under its ticket kept", three, ansOwn(`"stream":{"url":"`+musicStreamRoot+`Tk1/master.m3u8","protocol":"hls"}`, e2, e3)},
		{"an hls stream named file, and a file stream named master.m3u8", three, ans(entry(mbTrack2, "transcode", url(at("Tk1", "0", "file"), "hls"), ``), entry(mbTrack3, "direct", url(at("Tk1", "1", "master.m3u8"), "file"), ``), e4)},
		{"a transcode served as file dropped", three, ans(entry(mbTrack2, "transcode", file("0"), ``), e3)},
		{"an unknown protocol dropped", three, ans(entry(mbTrack2, "transcode", url(at("Tk1", "0", "master.m3u8"), "dash"), ``), e3)},
		{"an hls stream of another mimeType, and a mimeType that isn't a string", three, ans(entry(mbTrack2, "transcode", `{"url":"`+at("Tk1", "0", "master.m3u8")+`","protocol":"hls","mimeType":"audio/mpeg"}`, ``), entry(mbTrack3, "transcode", `{"url":"`+at("Tk1", "1", "master.m3u8")+`","protocol":"hls","mimeType":7}`, ``), e4)},
		{"a file stream with any mimeType kept", three, ans(entry(mbTrack2, "direct", `{"url":"`+at("Tk1", "0", "file")+`","protocol":"file","mimeType":"audio/flac"}`, ``), e3)},
		{"durationMs at its bounds", three, ans(entry(mbTrack2, "direct", file("0"), `,"durationMs":0`), entry(mbTrack3, "direct", file("1"), `,"durationMs":86400000`))},
		{"durationMs out of range: set to 0", three, ans(entry(mbTrack2, "direct", file("0"), `,"durationMs":86400001`), entry(mbTrack3, "direct", file("1"), `,"durationMs":-1`))},
		{"durationMs that isn't an integer: absent", three, ans(entry(mbTrack2, "direct", file("0"), `,"durationMs":1.5`), entry(mbTrack3, "direct", file("1"), `,"durationMs":"1"`))},
		{"with delivery file: only direct as file kept", req(`,"delivery":"file","queue":[` + t(mbTrack2) + `,` + t(mbTrack3) + `,` + t(mbTrack4) + `]`), ans(entry(mbTrack2, "remux", hls("0"), ``), entry(mbTrack3, "direct", hls("1"), ``), entry(mbTrack4, "direct", file("2"), ``))},
		{"with delivery transcode: a direct entry dropped", req(`,"delivery":"transcode","queue":[` + t(mbTrack2) + `,` + t(mbTrack3) + `]`), ans(entry(mbTrack2, "direct", file("0"), ``), entry(mbTrack3, "transcode", hls("1"), ``))},
		{"5 entries, local ones among them: places 0 to 4", req(`,"queue":[` + t(mbTrack2) + `,"track/local/T-1",` + t(mbTrack3) + `,` + t(mbTrack4) + `,"track/local/T-2"]`), ans(entry(mbTrack2, "transcode", hls("0"), ``), `{"title":"track/local/T-1","versionId":"L1","method":"direct","stream":`+file("1")+`}`, entry(mbTrack3, "direct", file("2"), ``), entry(mbTrack4, "direct", file("3"), ``), `{"title":"track/local/T-2","versionId":"L2","method":"transcode","stream":`+hls("4")+`}`)},
		{"a queue in the answer to a request without one: every entry dropped", req(``), ans(e2)},
	}
	for _, c := range answers {
		got, err := musicKeepQueueAnswer([]byte(c.request), []byte(c.text), musicStreamRoot)
		ok, r := reason(err)
		out = append(out, musicCallCase{Name: c.name, Kind: "answer", Request: c.request, Text: c.text, Valid: ok, Output: string(got), Reason: r})
	}
	return out
}

func musicLyricsCases() []musicCallCase {
	var out []musicCallCase
	for _, c := range []musicIn{
		{"a track and a copy", `{"title":"track/mb/` + mbTrack + `","versionId":"Vq3kM0aZ7pXc"}`},
		{"a local track", `{"title":"track/local/T-1","versionId":"c1"}`},
		{"unknown members ignored", `{"title":"track/mb/` + mbTrack + `","versionId":"c1","lang":"fr"}`},
		{"no versionId: 400", `{"title":"track/mb/` + mbTrack + `"}`},
		{"a versionId out of form: 400", `{"title":"track/mb/` + mbTrack + `","versionId":"c/1"}`},
		{"a versionId of 65 characters: 400", `{"title":"track/mb/` + mbTrack + `","versionId":"` + strings.Repeat("v", 65) + `"}`},
		{"a null versionId: 400", `{"title":"track/mb/` + mbTrack + `","versionId":null}`},
		{"no title: 400", `{"versionId":"c1"}`},
		{"an album as title: 400", `{"title":"album/mb/` + mbAlbum + `","versionId":"c1"}`},
		{"a title that isn't a string: 400", `{"title":["track/mb/` + mbTrack + `"],"versionId":"c1"}`},
		{"not an object: 400", `["track/mb/` + mbTrack + `","c1"]`},
	} {
		err := musicCheckLyricsRequest([]byte(c.text))
		ok, r := reason(err)
		out = append(out, musicCallCase{Name: c.name, Kind: "request", Text: c.text, Valid: ok, Reason: r})
	}

	set := func(members string) string { return `{"structuredLyrics":[` + members + `]}` }
	var lines2001, sets9 []string
	for i := 0; i < 2001; i++ {
		lines2001 = append(lines2001, fmt.Sprintf(`{"value":"%d"}`, i))
	}
	for i := 0; i < 9; i++ {
		sets9 = append(sets9, fmt.Sprintf(`{"synced":false,"line":[{"value":"set %d"}]}`, i))
	}
	for _, c := range []musicIn{
		{"plain lyrics", set(`{"synced":false,"line":[{"value":"Hello darkness"},{"value":""},{"value":"my old friend"}]}`)},
		{"synced lyrics with every member", set(`{"lang":"en","kind":"main","synced":true,"offset":-250,"displayArtist":"Simon & Garfunkel","displayTitle":"The Boxer","line":[{"start":0,"value":"I am just a poor boy"},{"start":4210,"value":"Though my story's seldom told"}]}`)},
		{"a translation and a pronunciation beside the main set", set(`{"lang":"en","synced":false,"line":[{"value":"a"}]},{"lang":"fr","kind":"translation","synced":false,"line":[{"value":"b"}]},{"lang":"ja-Latn","kind":"pronunciation","synced":false,"line":[{"value":"c"}]}`)},
		{"lang kept only as a tag is sent", set(`{"lang":"fr-CA","synced":false,"line":[{"value":"a"}]},{"lang":"fre","synced":false,"line":[{"value":"b"}]},{"lang":"fr-ca","synced":false,"line":[{"value":"c"}]},{"lang":"und","synced":false,"line":[{"value":"d"}]},{"lang":7,"synced":false,"line":[{"value":"e"}]}`)},
		{"a kind that isn't a token, or isn't a string: main", set(`{"kind":"Translation","synced":false,"line":[{"value":"a"}]},{"kind":2,"synced":false,"line":[{"value":"b"}]},{"kind":"","synced":false,"line":[{"value":"c"}]}`)},
		{"an unknown kind token kept as it is, never turned into main", set(`{"kind":"romanization","synced":false,"line":[{"value":"a"}]},{"kind":"other","synced":false,"line":[{"value":"b"}]}`)},
		{"offset at its bounds", set(`{"synced":true,"offset":-600000,"line":[{"start":0,"value":"a"}]},{"synced":true,"offset":600000,"line":[{"start":0,"value":"b"}]}`)},
		{"offset out of range or not an integer: 0", set(`{"synced":true,"offset":600001,"line":[{"start":0,"value":"a"}]},{"synced":true,"offset":-600001,"line":[{"start":0,"value":"b"}]},{"synced":true,"offset":1.5,"line":[{"start":0,"value":"c"}]},{"synced":true,"offset":"10","line":[{"start":0,"value":"d"}]}`)},
		{"displayArtist and displayTitle cleaned and cut", set(`{"synced":false,"displayArtist":" A\t\tB ` + strings.Repeat("a", 520) + `","displayTitle":"` + strings.Repeat("t", 257) + `","line":[{"value":"a"}]}`)},
		{"displayArtist and displayTitle empty once cleaned or not strings: absent", set(`{"synced":false,"displayArtist":" ​ ","displayTitle":7,"line":[{"value":"a"}]}`)},
		{"synced that isn't a boolean, or missing: the set skipped", set(`{"synced":"true","line":[{"value":"a"}]},{"line":[{"value":"b"}]},{"synced":false,"line":[{"value":"c"}]}`)},
		{"a set that isn't an object skipped", set(`"x",{"synced":false,"line":[{"value":"c"}]}`)},
		{"line values cleaned, kept when empty, cut to 512 code points", set(`{"synced":false,"line":[{"value":"  two\t\tspaces \u0007"},{"value":" ​ "},{"value":"` + strings.Repeat("é", 511) + `ab"}]}`)},
		{"lines are text: markup kept as it is", set(`{"synced":false,"line":[{"value":"<b>loud</b> &amp; <script>x</script>"}]}`)},
		{"a line whose value isn't a string, or that isn't an object, skipped", set(`{"synced":false,"line":[{"value":7},{"start":5},"x",{"value":"kept"}]}`)},
		{"synced: lines without a start, or out of range, skipped", set(`{"synced":true,"line":[{"value":"no start"},{"start":-1,"value":"negative"},{"start":86400001,"value":"too late"},{"start":1.5,"value":"not an integer"},{"start":0,"value":"zero"},{"start":86400000,"value":"last"}]}`)},
		{"synced: a start before the line before skipped, an equal one kept", set(`{"synced":true,"line":[{"start":1000,"value":"a"},{"start":999,"value":"before"},{"start":1000,"value":"same"},{"start":2000,"value":"b"}]}`)},
		{"plain: start ignored", set(`{"synced":false,"line":[{"start":5000,"value":"a"},{"start":"x","value":"b"}]}`)},
		{"at most 2,000 lines kept, the first", set(`{"synced":false,"line":[` + strings.Join(lines2001, ",") + `]}`)},
		{"at most 8 sets kept, the first", set(strings.Join(sets9, ","))},
		{"a set with no line left skipped", set(`{"synced":true,"line":[{"value":"no start"}]},{"synced":false,"line":[]},{"synced":false},{"synced":false,"line":{"value":"x"}},{"synced":false,"line":[{"value":"kept"}]}`)},
		{"no set left: the fetch fails", set(`{"synced":true,"line":[{"value":"no start"}]}`)},
		{"an empty structuredLyrics: the fetch fails", set(``)},
		{"a structuredLyrics that isn't an array: the fetch fails", `{"structuredLyrics":{"synced":false,"line":[{"value":"a"}]}}`},
		{"no structuredLyrics: the fetch fails", `{}`},
		{"an answer that isn't an object: the fetch fails", `[{"synced":false,"line":[{"value":"a"}]}]`},
	} {
		got, err := musicKeepLyrics([]byte(c.text))
		ok, r := reason(err)
		out = append(out, musicCallCase{Name: c.name, Kind: "answer", Text: c.text, Valid: ok, Output: string(got), Reason: r})
	}
	return out
}

func genV0Music() []byte {
	return encode(v1MusicFile{
		About: []string{
			"MSFP music part version 0, as revised by proposals 16 and 17 (spec/v0/music.md). Generated by tools/vectors; no implementation has produced them yet. The MBIDs are made up.",
			"keys (music section 2): valid says whether the text is a music title key, under the mb or the local scheme; kind is the kind it names.",
			"titles (music sections 3.2 and 3.3) and localTitles (catalog section 4.3, music section 2.2): text is one entry of a titles answer, as JSON; the caller asked for the libraries listed in libraries. valid false: the caller skips the title. valid true: output is what it keeps, members in this order: key, kind, library, name, sortName, artists, artistNames, primaryType, secondaryTypes, compilation, date, originalDate, genres, album, disc, number, copies, image, ids (its members sorted); absent members left out.",
			"A kept copy's members, in this order: id, name, durationMs, container, codec, codecs, size, etag, added, bitrateKbps, sampleRate, bitDepth, channels, replayGain (trackGain, trackPeak, albumGain, albumPeak), lyrics. ReplayGain values are written as the shortest decimal of the rounded number (1.0 is written 1); no case holds an exact tie of rounding.",
			"queue (music section 5.2): kind request: text is a play request, valid says whether the owning server accepts its title and queue (otherwise 400). kind answer: request is the play request, text the play answer from the owning server whose stream root is streamRoot; output is the queue the viewer's server keeps (an array, possibly empty), entries' members in this order: title, versionId, method, stream (url, protocol, mimeType), durationMs. Each entry's stream URL is <stream root><ticket>/queue/<n>/file or <stream root><ticket>/queue/<n>/master.m3u8, the ticket that of the answer's own stream URL, n the entry's place in the request's queue (0 to 4, decimal, no leading zeros). Only the queue member of the answer is judged here; playback's own members are playback.json's.",
			"lyrics (music section 6): kind request: text is a lyrics request, valid says whether the owning server reads it (otherwise 400 invalid-request). kind answer: text is a lyrics answer; valid false: the fetch fails; valid true: output is what the viewer's server keeps, each set's members in this order: lang, kind, synced, offset, displayArtist, displayTitle, line (start, value); kind and offset always written.",
			"Open values (spec/v0/core.md section 8.5): a copy's container and codec are tokens (1 to 32 characters of a-z 0-9 . _ -), ffmpeg's names or any other, kept as they are; one that isn't a token is dropped, and the copy, missing a required member, skipped. primaryType and each of secondaryTypes are MusicBrainz release-group types, lowercased, 1 to 32 characters of a-z 0-9 space . _ - /, no space at either end, kept as they are; any other value counts as absent (an entry of secondaryTypes skipped alone). codecs is an RFC 6381 string of 1 to 64 characters of A-Z a-z 0-9 . -; any other counts as absent. durationMs and bitrateKbps are kept exact.",
			"Names are cleaned as spec/v0/core.md section 9.1 cleans a server's name, with 256 code points for name and sortName, 512 for artistNames, 64 for genres (a longer genre skipped, not cut), 64 for a copy's name, 512 and 256 for displayArtist and displayTitle, 512 for a lyrics line.",
		},
		Libraries:   musicLibraries,
		StreamRoot:  musicStreamRoot,
		Keys:        musicKeyCases(),
		Titles:      musicTitleCases(),
		LocalTitles: musicLocalTitleCases(),
		Queue:       musicQueueCases(),
		Lyrics:      musicLyricsCases(),
	})
}
