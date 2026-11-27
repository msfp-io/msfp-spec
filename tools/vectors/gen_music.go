package main

import (
	"fmt"
	"strings"
)

// The libraries the caller asked for, in every music title case.
var musicLibraries = []string{"m1", "m2"}

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
	mAlbumHead  = `"key":"album/mb/` + mbAlbum + `","library":"m1"`
	mArtistHead = `"key":"artist/mb/` + mbArtist + `","library":"m2"`
	mTrackHead  = `"key":"track/mb/` + mbTrack + `","library":"m1","album":"album/mb/` + mbAlbum + `"`
	mCopy       = `{"id":"c1","durationMs":308000,"mediaType":"audio/flac","codec":"flac","size":36214455,"etag":"7f3a9c21"}`
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
	base := []string{`"id":"` + id + `"`, `"durationMs":308000`, `"mediaType":"audio/flac"`, `"codec":"flac"`, `"size":36214455`, `"etag":"7f3a9c21"`}
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
	var seventeen []string
	for i := 1; i <= 17; i++ {
		seventeen = append(seventeen, mCopyWith(fmt.Sprintf("c%d", i)))
	}
	in := []musicIn{
		// Section 3's example, as it is.
		{"music section 3's example album", `{"key":"album/mb/0b000000-0000-4000-8000-000000000002","library":"m1","name":"Bridge over Troubled Water","artists":["artist/mb/0a000000-0000-4000-8000-000000000001"],"artistNames":"Simon & Garfunkel","primaryType":"album","date":"2011-03-08","originalDate":"1970-01-26","genres":["Folk rock"],"ids":{"mbreleasegroup":"0d000000-0000-4000-8000-000000000005"}}`},
		{"music section 3's example track", `{"key":"track/mb/0c000000-0000-4000-8000-000000000003","library":"m1","name":"The Boxer","album":"album/mb/0b000000-0000-4000-8000-000000000002","disc":1,"number":2,"copies":[{"id":"Vq3kM0aZ7pXc","durationMs":308417,"mediaType":"audio/flac","codec":"flac","size":36214455,"etag":"7f3a9c21","sampleRate":44100,"bitDepth":16,"channels":2,"replayGain":{"trackGain":-4.12,"trackPeak":0.988525,"albumGain":-5.3,"albumPeak":1.0},"lyrics":"synced"}],"ids":{"mbrecording":"0e000000-0000-4000-8000-000000000006"}}`},
		{"an artist", `{` + mArtistHead + `,"name":"Simon & Garfunkel"}`},
		{"an album with every member", mAlbum(`"artists":[` + ak(mbArtist) + `,` + ak(mbArtist2) + `],"artistNames":"A & B","primaryType":"album","secondaryTypes":["compilation","live"],"compilation":true,"date":"2011","originalDate":"1970-01","genres":["Folk rock","Pop"],"ids":{"mbreleasegroup":"` + mbGroup + `"}`)},

		// Names.
		{"names cleaned: white space runs, controls and format characters", `{` + mTrackHead + `,"name":"  The\t\tBoxer\n\u0007(live)‮ ","artistNames":" Simon &​Garfunkel ","copies":[` + mCopy + `]}`},
		{"a format character between spaces leaves two", `{` + mArtistHead + `,"name":"a ​ b"}`},
		{"a name cut to 256 code points, then trimmed", `{` + mArtistHead + `,"name":"` + strings.Repeat("é", 255) + ` b"}`},
		{"a name cut to 256 code points", `{` + mArtistHead + `,"name":"` + strings.Repeat("é", 255) + `ab"}`},
		{"artistNames cut to 512 code points", mTrack(`"artistNames":"` + strings.Repeat("a", 511) + `bc"`)},
		{"sortName isn't a member any more: ignored", mAlbum(`"sortName":"Bridge over Troubled Water, The"`)},
		{"artistNames empty once cleaned: absent", mAlbum(`"artistNames":" ​\t"`)},
		{"artistNames that isn't a string: absent", mAlbum(`"artistNames":7`)},
		{"no name: skipped", `{` + mArtistHead + `}`},
		{"a name empty once cleaned: skipped", `{` + mArtistHead + `,"name":" ​\t "}`},
		{"a name that isn't a string: skipped", `{` + mArtistHead + `,"name":7}`},
		{"a null name: skipped", `{` + mArtistHead + `,"name":null}`},

		// Catalog section 3.3.
		{"a kind member, not the key's: no title kind (MSFP Media listing section 3), ignored", `{"key":"artist/mb/` + mbArtist + `","kind":"album","library":"m1","name":"x"}`},
		{"a library not asked for: skipped", `{"key":"artist/mb/` + mbArtist + `","library":"m3","name":"x"}`},
		{"a key in uppercase: skipped", `{"key":"artist/mb/0A000000-0000-4000-8000-000000000001","library":"m1","name":"x"}`},
		{"an unknown scheme: skipped", `{"key":"artist/discogs/123","library":"m1","name":"x"}`},
		{"a video key: skipped", `{"key":"movie/tmdb/949","library":"m1","name":"x"}`},
		{"a key that isn't a string: skipped", `{"key":7,"library":"m1","name":"x"}`},
		{"a library that isn't a string: skipped", `{"key":"artist/mb/` + mbArtist + `","library":["m1"],"name":"x"}`},
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
		{"MusicBrainz's secondary types with a space and a slash: field recording, audio drama, mixtape/street", mAlbum(`"primaryType":"album","secondaryTypes":["field recording","audio drama","mixtape/street","audiobook"]`)},
		{"a type with a slash at an end, and one with two spaces inside, kept", mAlbum(`"primaryType":"a/","secondaryTypes":["/b","c  d"]`)},
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
		{"a track/mb track without album: skipped", `{"key":"track/mb/` + mbTrack + `","library":"m1","name":"x","copies":[` + mCopy + `]}`},
		{"a track/mb track on a local album: skipped", `{"key":"track/mb/` + mbTrack + `","library":"m1","name":"x","album":"album/local/A1","copies":[` + mCopy + `]}`},
		{"a track/mb track whose album isn't an album key: skipped", `{"key":"track/mb/` + mbTrack + `","library":"m1","name":"x","album":"artist/mb/` + mbArtist + `","copies":[` + mCopy + `]}`},
		{"a track/mb track whose album isn't a string: skipped", `{"key":"track/mb/` + mbTrack + `","library":"m1","name":"x","album":7,"copies":[` + mCopy + `]}`},
		{"disc and number at their bounds", mTrack(`"disc":999,"number":1`)},
		{"disc and number at their other bounds", mTrack(`"disc":1,"number":999`)},
		{"disc and number out of range: absent", mTrack(`"disc":0,"number":1000`)},
		{"disc and number that aren't MSFP integers: absent", mTrack(`"disc":1.0,"number":"2"`)},
		{"disc -0: absent", mTrack(`"disc":-0,"number":1e0`)},

		// Members of other kinds.
		{"an album ignores a track's members", mAlbum(`"album":"album/mb/` + mbAlbum2 + `","disc":1,"number":1,"copies":[` + mCopy + `],"image":true,"ids":{"mbrecording":"` + mbRec + `","isrc":"USSM17000123","acoustid":"` + acoustID + `"}`)},
		{"a track ignores an album's members", mTrack(`"sortName":"y","primaryType":"album","secondaryTypes":["live"],"compilation":true,"image":true,"ids":{"mbreleasegroup":"` + mbGroup + `"}`)},
		{"a track without artists, artistNames, dates or genres: its album's, nothing filled in", mTrack(``)},
		{"a compilation's track with its own artists and names", mTrack(`"artists":[` + ak(mbArtist2) + `],"artistNames":"B"`)},
		{"a track's members equal to its album's kept as sent: the caller doesn't compare", mTrack(`"artists":[` + ak(mbArtist) + `],"artistNames":"Simon & Garfunkel","date":"2011-03-08","genres":["Folk rock"]`)},
		{"a track's own dates and genres kept", mTrack(`"date":"1999-05","originalDate":"1968","genres":["Soul"," Funk ","soul"]`)},
		{"a track's dates checked as an album's", mTrack(`"date":"2023-02-29","originalDate":"1970-13"`)},
		{"a track's originalDate after its date: absent", mTrack(`"date":"1970","originalDate":"1971"`)},
		{"a track's genres: at most 8 kept, the first", mTrack(`"genres":["a","b","c","d","e","f","g","h","i"]`)},
		{"a track's dates and genres of the wrong type: absent", mTrack(`"date":1999,"originalDate":["1968"],"genres":"Soul"`)},
		{"an artist carries no artists, artistNames, album members, dates, genres or image", `{` + mArtistHead + `,"name":"x","artists":[` + ak(mbArtist2) + `],"artistNames":"y","sortName":"z","primaryType":"album","genres":["Pop"],"date":"1970","image":true,"copies":[` + mCopy + `],"ids":{"mbreleasegroup":"` + mbGroup + `"}}`},

		// ids.
		{"ids of an mb track: mbrecording", mTrack(`"ids":{"mbrecording":"` + mbRec + `"}`)},
		{"ids of an mb album: mbreleasegroup", mAlbum(`"ids":{"mbreleasegroup":"` + mbGroup + `"}`)},
		{"ids: isrc and acoustid aren't schemes any more, skipped", mTrack(`"ids":{"mbrecording":"` + mbRec + `","isrc":"USSM17000123","acoustid":"` + acoustID + `"}`)},
		{"ids: isrc alone, skipped: ids absent", mTrack(`"ids":{"isrc":"GBAYE6300001"}`)},
		{"ids out of form, of other kinds, of unknown or old schemes, and non-strings skipped", mTrack(`"ids":{"mbrecording":"0E000000-0000-4000-8000-000000000006","mbreleasegroup":"` + mbGroup + `","mbrelease":"` + mbAlbum + `","mbtrack":"` + mbTrack + `","discogs":"123","tmdb":"949"}`)},
		{"ids: an mbrecording on an album skipped", mAlbum(`"ids":{"mbrecording":"` + mbRec + `","mbreleasegroup":"` + mbGroup + `"}`)},
		{"ids on an mb artist: skipped, its key is its MBID", `{` + mArtistHead + `,"name":"x","ids":{"mbreleasegroup":"` + mbGroup + `","mbrecording":"` + mbRec + `"}}`},
		{"ids: values that aren't strings skipped alone", mTrack(`"ids":{"mbreleasegroup":7,"mbrecording":"` + mbRec + `","acoustid":null}`)},
		{"ids: an mbrecording that isn't a string: ids absent", mTrack(`"ids":{"mbrecording":7}`)},
		{"ids that isn't an object: absent", mTrack(`"ids":["` + mbRec + `"]`)},

		// Copies: the track.
		{"a track without copies: skipped", `{` + mTrackHead + `,"name":"x"}`},
		{"copies that isn't an array: skipped", `{` + mTrackHead + `,"name":"x","copies":` + mCopy + `}`},
		{"empty copies: skipped", mCopies()},
		{"no copy left: skipped", mCopies(mCopyWith("c1", `"durationMs":0`), `"c2"`)},

		// Copies: required members.
		{"a copy without id skipped alone", mCopies(mCopyWith("c1", `"id":-`), mCopyWith("c2"))},
		{"a copy without durationMs skipped alone", mCopies(mCopyWith("c1", `"durationMs":-`), mCopyWith("c2"))},
		{"a copy without mediaType skipped alone", mCopies(mCopyWith("c1", `"mediaType":-`), mCopyWith("c2"))},
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
		{"media types and codec tokens kept", mCopies(mCopyWith("c1", `"mediaType":"audio/mpeg"`, `"codec":"mp3"`), mCopyWith("c2", `"mediaType":"audio/mp4"`, `"codec":"alac"`), mCopyWith("c3", `"mediaType":"audio/ogg"`, `"codec":"vorbis"`), mCopyWith("c4", `"mediaType":"audio/wave"`, `"codec":"pcm_s24le"`), mCopyWith("c5", `"mediaType":"audio/matroska"`, `"codec":"opus"`, `"codecs":"Opus"`), mCopyWith("c6", `"mediaType":"audio/aiff"`, `"codec":"pcm_s16be"`))},
		{"media types and codecs kept as they are, known or not, never turned into another value", mCopies(mCopyWith("c1", `"mediaType":"audio/x-wav"`, `"codec":"wmav2"`), mCopyWith("c2", `"mediaType":"video/mp4"`, `"codec":"h264"`), mCopyWith("c3", `"mediaType":"audio/vnd.dolby.mlp+x_1"`, `"codec":"iamf"`), mCopyWith("c4", `"mediaType":"application/octet-stream"`, `"codec":"other"`), mCopyWith("c5", `"mediaType":"1/2"`))},
		{"a media type of 127 characters kept, of 128 the copy skipped", mCopies(mCopyWith("c1", `"mediaType":"audio/`+strings.Repeat("m", 121)+`"`), mCopyWith("c2", `"mediaType":"audio/`+strings.Repeat("m", 122)+`"`), mCopyWith("c3"))},
		{"a codec of 32 characters kept", mCopies(mCopyWith("c1", `"codec":"`+strings.Repeat("k", 32)+`"`))},
		{"a mediaType that isn't a media type: the copy skipped", mCopies(mCopyWith("c1", `"mediaType":"audio/FLAC"`), mCopyWith("c2", `"mediaType":"flac"`), mCopyWith("c3", `"mediaType":"audio/mp4; codecs=mp4a.40.2"`), mCopyWith("c4", `"mediaType":"audio/mp4/x"`), mCopyWith("c5", `"mediaType":"audio/"`), mCopyWith("c6", `"mediaType":"/flac"`), mCopyWith("c7", `"mediaType":"audio/.flac"`), mCopyWith("c8", `"mediaType":"+audio/flac"`), mCopyWith("c9", `"mediaType":"audio/fl ac"`), mCopyWith("c10", `"mediaType":""`), mCopyWith("c11"))},
		{"a mediaType that isn't a string, or missing: the copy skipped", mCopies(mCopyWith("c1", `"mediaType":7`), mCopyWith("c2", `"mediaType":["audio/flac"]`), mCopyWith("c3", `"mediaType":-`), mCopyWith("c4"))},
		{"container isn't a member any more: a copy with container and no mediaType skipped", mCopies(mCopyWith("c1", `"mediaType":-`, `"container":"flac"`), mCopyWith("c2", `"container":"flac"`))},
		{"a codec that isn't a token: the copy skipped", mCopies(mCopyWith("c1", `"codec":"FLAC"`), mCopyWith("c2", `"codec":"pcm s16le"`), mCopyWith("c3", `"codec":""`), mCopyWith("c4", `"codec":"`+strings.Repeat("k", 33)+`"`), mCopyWith("c5", `"codec":"flac,alac"`), mCopyWith("c6"))},
		{"a codec that isn't a string: the copy skipped", mCopies(mCopyWith("c1", `"codec":7`), mCopyWith("c2", `"codec":["flac"]`), mCopyWith("c3"))},
		{"codecs required for aac and dts: sent, kept", mCopies(mCopyWith("c1", `"mediaType":"audio/mp4"`, `"codec":"aac"`, `"codecs":"mp4a.40.2"`), mCopyWith("c2", `"mediaType":"audio/mp4"`, `"codec":"aac"`, `"codecs":"mp4a.40.5"`), mCopyWith("c3", `"mediaType":"audio/matroska"`, `"codec":"dts"`, `"codecs":"dtsh"`))},
		{"codecs missing on aac or dts: the copy kept without codecs, not played as it is", mCopies(mCopyWith("c1", `"mediaType":"audio/mp4"`, `"codec":"aac"`), mCopyWith("c2", `"mediaType":"audio/wave"`, `"codec":"dts"`))},
		{"codecs malformed on aac: counts as absent, the copy kept", mCopies(mCopyWith("c1", `"mediaType":"audio/mp4"`, `"codec":"aac"`, `"codecs":"mp4a.40.2,mp4a.40.5"`), mCopyWith("c2", `"mediaType":"audio/mp4"`, `"codec":"aac"`, `"codecs":7`))},
		{"codecs optional for other codecs: absent on flac, mp3 and opus", mCopies(mCopyWith("c1"), mCopyWith("c2", `"mediaType":"audio/mpeg"`, `"codec":"mp3"`), mCopyWith("c3", `"mediaType":"audio/ogg"`, `"codec":"opus"`))},
		{"codecs, RFC 6381 strings, kept as sent, case and all", mCopies(mCopyWith("c1", `"codecs":"fLaC"`), mCopyWith("c2", `"codecs":"flac"`), mCopyWith("c3", `"mediaType":"audio/mp4"`, `"codec":"eac3"`, `"codecs":"ec-3"`))},
		{"codecs of 64 characters kept, of 65 absent", mCopies(mCopyWith("c1", `"codecs":"`+strings.Repeat("a", 64)+`"`), mCopyWith("c2", `"codecs":"`+strings.Repeat("a", 65)+`"`))},
		{"codecs empty, with a comma, a space or a slash, or not a string: absent", mCopies(mCopyWith("c1", `"codecs":""`), mCopyWith("c2", `"codecs":"mp4a.40.2,avc1"`), mCopyWith("c3", `"codecs":"mp4a 40"`), mCopyWith("c4", `"codecs":"a/b"`), mCopyWith("c5", `"codecs":7`))},
		{"at most 16 copies kept, the first", mCopies(append([]string{`7`}, seventeen...)...)},
		{"at most 16 copies kept, skipped ones not counted", mCopies(append([]string{mCopyWith("bad", `"etag":-`)}, seventeen[:16]...)...)},
		{"16 copies, all kept", mCopies(seventeen[:16]...)},

		// Copies: optional members.
		{"name, added and bitrateKbps aren't members any more: ignored", mCopies(mCopyWith("c1", `"name":"Hi-Res"`, `"added":"2024-02-29"`, `"bitrateKbps":941`), mCopyWith("c2", `"name":7`, `"bitrateKbps":0`))},
		{"durationMs kept exact, to the millisecond", mCopies(mCopyWith("c1", `"durationMs":308417`), mCopyWith("c2", `"durationMs":1001`))},
		{"sampleRate, bitDepth and channels at their lower bounds", mCopies(mCopyWith("c1", `"sampleRate":8000`, `"bitDepth":8`, `"channels":1`))},
		{"sampleRate, bitDepth and channels at their upper bounds", mCopies(mCopyWith("c1", `"sampleRate":768000`, `"bitDepth":32`, `"channels":64`))},
		{"sampleRate, bitDepth and channels below their ranges: absent", mCopies(mCopyWith("c1", `"sampleRate":7999`, `"bitDepth":7`, `"channels":0`))},
		{"sampleRate, bitDepth and channels above their ranges: absent", mCopies(mCopyWith("c1", `"sampleRate":768001`, `"bitDepth":33`, `"channels":65`))},
		{"sampleRate, bitDepth and channels that aren't integers: absent", mCopies(mCopyWith("c1", `"sampleRate":44100.0`, `"bitDepth":"16"`, `"channels":true`))},
		{"bitDepth on a lossy codec is kept: the caller checks only its range", mCopies(mCopyWith("c1", `"codec":"mp3"`, `"mediaType":"audio/mpeg"`, `"bitDepth":16`))},
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
		{"video's members of a copy ignored on a track", mCopies(mCopyWith("c1", `"edition":"remastered"`, `"startMs":0`, `"parts":2`, `"video":{"codec":"h264","width":640,"height":480}`, `"audioTracks":[{"id":"a1","codec":"flac"}]`, `"subtitleTracks":[{"id":"s1","format":"subrip"}]`, `"endMs":1000`, `"chapters":[{"startMs":0}]`, `"segments":[]`, `"previewLayout":{"tileWidth":320,"tileHeight":180,"columns":10,"rows":10,"intervalMs":10000,"tileCount":31}`, `"previews":true`, `"threeD":"sbs"`))},
	}
	return musicKeep(in)
}

func musicLocalTitleCases() []titleCase {
	in := []musicIn{
		{"a local artist with an image", `{"key":"artist/local/Ar7","library":"m1","name":"The Garage Band","image":true}`},
		{"a local album without an image (false kept)", `{"key":"album/local/Al_1","library":"m1","name":"Demos","artists":["artist/local/Ar7"],"artistNames":"The Garage Band","primaryType":"album","secondaryTypes":["demo"],"date":"2019","genres":["Rock"],"image":false}`},
		{"a local album's ids ignored: a local title carries none", `{"key":"album/local/Al_2","library":"m1","name":"Bootleg","ids":{"mbreleasegroup":"` + mbGroup + `"}}`},
		{"an image that isn't a boolean: absent", `{"key":"artist/local/Ar8","library":"m1","name":"x","image":"true"}`},
		{"a local track carries no image", `{"key":"track/local/T-1","library":"m1","name":"Song","image":true,"copies":[` + mCopy + `]}`},
		{"an mb artist's image ignored", `{` + mArtistHead + `,"name":"x","image":true}`},
		{"an mb album's image ignored", mAlbum(`"image":true`)},
		{"a local track without an album; its ids ignored", `{"key":"track/local/T-2","library":"m1","name":"Home recording","artists":["artist/local/Ar7"],"disc":1,"number":3,"copies":[` + mCopy + `],"ids":{"isrc":"FRZ039900001","acoustid":"` + acoustID + `","mbrecording":"` + mbRec + `"}}`},
		{"a local track on an mb album", `{"key":"track/local/T-3","library":"m1","name":"Bonus","album":"album/mb/` + mbAlbum + `","copies":[` + mCopy + `]}`},
		{"a local track on a local album", `{"key":"track/local/T-4","library":"m1","name":"Take 2","album":"album/local/Al_1","copies":[` + mCopy + `]}`},
		{"a local track whose album isn't an album key: album absent", `{"key":"track/local/T-5","library":"m1","name":"x","album":"track/local/T-4","copies":[` + mCopy + `]}`},
		{"an mb album crediting a local artist", mAlbum(`"artists":["artist/local/Ar7",` + ak(mbArtist) + `]`)},
		{"a local name keeps music's 256 code points, not the description's 128", `{"key":"artist/local/Ar9","library":"m1","name":"` + strings.Repeat("n", 200) + `"}`},
		{"a local name cut to 256 code points", `{"key":"artist/local/Ar9","library":"m1","name":"` + strings.Repeat("n", 257) + `"}`},
		{"year isn't a music member: ignored", `{"key":"album/local/Al_3","library":"m1","name":"x","year":2019}`},
		{"an opaque part of 64 characters", `{"key":"artist/local/` + strings.Repeat("Z", 64) + `","library":"m1","name":"x"}`},
		{"an opaque part of 65 characters: skipped", `{"key":"artist/local/` + strings.Repeat("Z", 65) + `","library":"m1","name":"x"}`},
		{"an opaque part with a dot: skipped", `{"key":"album/local/a.b","library":"m1","name":"x"}`},
		{"an empty opaque part: skipped", `{"key":"album/local/","library":"m1","name":"x"}`},
		{"a local title without a name: skipped", `{"key":"artist/local/Ar7","library":"m1","image":true}`},
		{"a local track without copies: skipped", `{"key":"track/local/T-6","library":"m1","name":"x"}`},
		{"a local key with a kind member not its own: ignored", `{"key":"album/local/Al_1","kind":"artist","library":"m1","name":"x"}`},
	}
	return musicKeep(in)
}

func musicQueueCases() []musicCallCase {
	t := func(id string) string { return `"track/mb/` + id + `"` }
	e := func(id, more string) string { return `{"title":` + t(id) + more + `}` }
	req := func(members string) string { return `{"title":` + t(mbTrack) + members + `}` }
	var out []musicCallCase
	for _, c := range []musicIn{
		{"no queue", req(``)},
		{"an empty queue", req(`,"queue":[]`)},
		{"one track", req(`,"queue":[` + e(mbTrack2, ``) + `]`)},
		{"5 tracks, local ones among them", req(`,"queue":[` + e(mbTrack2, ``) + `,{"title":"track/local/T-1"},` + e(mbTrack3, ``) + `,` + e(mbTrack4, ``) + `,{"title":"track/local/T-2"}]`)},
		{"a local track's queue", `{"title":"track/local/T-1","queue":[` + e(mbTrack, ``) + `]}`},
		{"entries with their own versionId", req(`,"queue":[` + e(mbTrack2, `,"versionId":"Vq3kM0aZ7pXc"`) + `,` + e(mbTrack3, `,"versionId":"`+strings.Repeat("v", 64)+`"`) + `]`)},
		{"entries with their own delivery: file, remux into audio/mp4 and audio/mp2t, transcode", req(`,"delivery":"file","queue":[` + e(mbTrack2, `,"delivery":"file"`) + `,` + e(mbTrack3, `,"delivery":"remux","mediaType":"audio/mp4"`) + `,` + e(mbTrack4, `,"delivery":"remux","mediaType":"audio/mp2t"`) + `,` + e(mbTrack5, `,"delivery":"transcode","transcode":{"audioCodec":"aac"}`) + `]`)},
		{"an entry's delivery transcode without targets", req(`,"queue":[` + e(mbTrack2, `,"delivery":"transcode"`) + `]`)},
		{"an entry with versionId and delivery together", req(`,"queue":[` + e(mbTrack2, `,"versionId":"c2","delivery":"remux","mediaType":"audio/mp4"`) + `]`)},
		{"unknown members of an entry ignored", req(`,"queue":[` + e(mbTrack2, `,"positionMs":5,"stream":{}`) + `]`)},
		{"entries without delivery under a request's remux: the request's mediaType", req(`,"delivery":"remux","mediaType":"audio/mp4","queue":[` + e(mbTrack2, ``) + `]`)},
		{"6 tracks: refused", req(`,"queue":[` + e(mbTrack2, ``) + `,` + e(mbTrack3, ``) + `,` + e(mbTrack4, ``) + `,` + e(mbTrack5, ``) + `,` + e(mbTrack6, ``) + `,{"title":"track/local/T-1"}]`)},
		{"an entry that is a bare key, as before proposal 17: refused", req(`,"queue":[` + t(mbTrack2) + `]`)},
		{"an entry that isn't an object: refused", req(`,"queue":[` + e(mbTrack2, ``) + `,7]`)},
		{"an entry without title: refused", req(`,"queue":[{"versionId":"c1"}]`)},
		{"an entry whose title isn't a string: refused", req(`,"queue":[{"title":7}]`)},
		{"an entry in uppercase: refused", req(`,"queue":[{"title":"track/mb/0C000000-0000-4000-8000-000000000008"}]`)},
		{"an album in the queue: refused", req(`,"queue":[{"title":"album/mb/` + mbAlbum + `"}]`)},
		{"an entry with a trailing slash: refused", req(`,"queue":[{"title":"track/mb/` + mbTrack2 + `/"}]`)},
		{"title itself in the queue: refused", req(`,"queue":[` + e(mbTrack2, ``) + `,` + e(mbTrack, ``) + `]`)},
		{"a repeated entry: refused, even with another versionId", req(`,"queue":[` + e(mbTrack2, `,"versionId":"c1"`) + `,` + e(mbTrack3, ``) + `,` + e(mbTrack2, `,"versionId":"c2"`) + `]`)},
		{"a versionId out of form: refused", req(`,"queue":[` + e(mbTrack2, `,"versionId":"c 1"`) + `]`)},
		{"a versionId of 65 characters: refused", req(`,"queue":[` + e(mbTrack2, `,"versionId":"`+strings.Repeat("v", 65)+`"`) + `]`)},
		{"a versionId that isn't a string: refused", req(`,"queue":[` + e(mbTrack2, `,"versionId":7`) + `]`)},
		{"a delivery that isn't a token: refused", req(`,"queue":[` + e(mbTrack2, `,"delivery":"File"`) + `]`)},
		{"a delivery that isn't a string: refused", req(`,"queue":[` + e(mbTrack2, `,"delivery":null`) + `]`)},
		{"an unknown delivery token: 409 delivery-refused", req(`,"queue":[` + e(mbTrack2, `,"delivery":"dash"`) + `]`)},
		{"a remux without mediaType: refused", req(`,"queue":[` + e(mbTrack2, `,"delivery":"remux"`) + `]`)},
		{"a remux with the old container member: refused, mediaType missing", req(`,"queue":[` + e(mbTrack2, `,"delivery":"remux","container":"mp4"`) + `]`)},
		{"a mediaType without the entry's own delivery remux: refused", req(`,"delivery":"remux","mediaType":"audio/mp4","queue":[` + e(mbTrack2, `,"mediaType":"audio/mp4"`) + `]`)},
		{"a mediaType beside delivery file: refused", req(`,"queue":[` + e(mbTrack2, `,"delivery":"file","mediaType":"audio/mp4"`) + `]`)},
		{"a malformed mediaType: refused", req(`,"queue":[` + e(mbTrack2, `,"delivery":"remux","mediaType":"audio/MP4"`) + `]`)},
		{"a mediaType with parameters: refused", req(`,"queue":[` + e(mbTrack2, `,"delivery":"remux","mediaType":"audio/mp4;codecs=flac"`) + `]`)},
		{"a mediaType that isn't a string: refused", req(`,"queue":[` + e(mbTrack2, `,"delivery":"remux","mediaType":["audio/mp4"]`) + `]`)},
		{"a remux into video/mp4 for a track: 409 target-refused", req(`,"queue":[` + e(mbTrack2, `,"delivery":"remux","mediaType":"video/mp4"`) + `]`)},
		{"a remux into another audio type: 409 target-refused", req(`,"queue":[` + e(mbTrack2, `,"delivery":"remux","mediaType":"audio/ogg"`) + `]`)},
		{"a malformed entry wins over a refused delivery: 400", req(`,"queue":[` + e(mbTrack2, `,"delivery":"dash"`) + `,` + e(mbTrack3, `,"versionId":"c/1"`) + `]`)},
		{"a transcode object without the entry's own delivery transcode: refused", req(`,"delivery":"transcode","queue":[` + e(mbTrack2, `,"transcode":{"audioCodec":"opus"}`) + `]`)},
		{"a transcode that isn't an object: refused", req(`,"queue":[` + e(mbTrack2, `,"delivery":"transcode","transcode":"aac"`) + `]`)},
		{"a queue that isn't an array: refused", req(`,"queue":` + e(mbTrack2, ``))},
		{"a null queue: refused", req(`,"queue":null`)},
		{"a queue after an album's title: refused", `{"title":"album/mb/` + mbAlbum + `","queue":[` + e(mbTrack2, ``) + `]}`},
	} {
		err := musicCheckQueueRequest([]byte(c.text))
		ok, r := reason(err)
		out = append(out, musicCallCase{Name: c.name, Kind: "request", Text: c.text, Valid: ok, Reason: r})
	}

	three := req(`,"queue":[` + e(mbTrack2, ``) + `,` + e(mbTrack3, ``) + `,` + e(mbTrack4, ``) + `]`)
	entry := func(id, vid, method, proto, more string) string {
		return `{"title":` + t(id) + `,"versionId":"` + vid + `","method":"` + method + `","protocol":"` + proto + `"` + more + `}`
	}
	e2 := entry(mbTrack2, "v1", "transcode", "hls", ``)
	e3 := entry(mbTrack3, "v1", "direct", "hls", ``)
	e4 := entry(mbTrack4, "v1", "remux", "hls", ``)
	head := `"id":"play_` + strings.Repeat("a", 26) + `","ticket":"Tk1","protocol":"hls","method":"transcode"`
	ans := func(entries ...string) string { return `{` + head + `,"queue":[` + strings.Join(entries, ",") + `]}` }
	mixed := req(`,"delivery":"transcode","queue":[` + e(mbTrack2, `,"delivery":"file"`) + `,` + e(mbTrack3, `,"versionId":"c2","delivery":"remux","mediaType":"audio/mp4"`) + `,` + e(mbTrack4, ``) + `]`)
	answers := []struct{ name, request, text string }{
		{"three entries prepared, in order, without delivery: all hls", three, ans(e2, e3, e4)},
		{"an entry the owning server left out: the others keep their places", three, ans(e2, e4)},
		{"an empty queue in the answer", three, ans()},
		{"no queue in the answer", three, `{` + head + `}`},
		{"a queue that isn't an array: absent", three, `{` + head + `,"queue":` + e2 + `}`},
		{"an entry that isn't an object dropped alone", three, ans(`"x"`, e2)},
		{"out of the request's order: the later entry dropped", three, ans(e4, e3)},
		{"a repeated entry dropped", three, ans(e2, e2, e3)},
		{"an entry not in the request's queue dropped", three, ans(entry(mbTrack5, "v1", "direct", "hls", ``), e3)},
		{"the request's title as an entry dropped", three, ans(entry(mbTrack, "v1", "direct", "hls", ``), e2)},
		{"a bad versionId dropped", three, ans(strings.Replace(e2, `"v1"`, `"v 1"`, 1), e3)},
		{"a missing versionId dropped", three, ans(strings.Replace(e2, `"versionId":"v1",`, ``, 1), e3)},
		{"an unknown method dropped", three, ans(strings.Replace(e2, `"transcode"`, `"copy"`, 1), e3)},
		{"a missing method dropped", three, ans(`{"title":`+t(mbTrack2)+`,"versionId":"v1","protocol":"hls"}`, e3)},
		{"a missing protocol dropped", three, ans(`{"title":`+t(mbTrack2)+`,"versionId":"v1","method":"direct"}`, e3)},
		{"an unknown protocol dropped", three, ans(entry(mbTrack2, "v1", "transcode", "dash", ``), e3)},
		{"without delivery, a file entry dropped: the baseline is HLS", three, ans(entry(mbTrack2, "v1", "direct", "file", ``), e3)},
		{"an entry's stream, url and durationMs, as before proposal 17: ignored", three, ans(entry(mbTrack2, "v1", "transcode", "hls", `,"stream":{"url":"https://b.example.org/x","protocol":"file"},"durationMs":201000`), e3)},
		{"the request's delivery file: only direct as file kept", req(`,"delivery":"file","queue":[` + e(mbTrack2, ``) + `,` + e(mbTrack3, ``) + `,` + e(mbTrack4, ``) + `]`), ans(entry(mbTrack2, "v1", "remux", "hls", ``), entry(mbTrack3, "v1", "direct", "hls", ``), entry(mbTrack4, "v1", "direct", "file", ``))},
		{"the request's delivery transcode: a direct entry dropped", req(`,"delivery":"transcode","queue":[` + e(mbTrack2, ``) + `,` + e(mbTrack3, ``) + `]`), ans(entry(mbTrack2, "v1", "direct", "hls", ``), entry(mbTrack3, "v1", "transcode", "hls", ``))},
		{"each entry checked against its own delivery, else the request's", mixed, ans(entry(mbTrack2, "v1", "direct", "file", ``), entry(mbTrack3, "c2", "remux", "hls", ``), entry(mbTrack4, "v1", "transcode", "hls", ``))},
		{"an entry served by the request's delivery instead of its own dropped", mixed, ans(entry(mbTrack2, "v1", "transcode", "hls", ``), entry(mbTrack3, "c2", "transcode", "hls", ``), entry(mbTrack4, "v1", "remux", "hls", ``))},
		{"an entry's own delivery file served as hls dropped", mixed, ans(entry(mbTrack2, "v1", "direct", "hls", ``), entry(mbTrack3, "c2", "remux", "file", ``))},
		{"an entry playing another copy than its versionId dropped", mixed, ans(entry(mbTrack3, "c1", "remux", "hls", ``), entry(mbTrack4, "c9", "transcode", "hls", ``))},
		{"5 entries, local ones among them", req(`,"queue":[` + e(mbTrack2, ``) + `,{"title":"track/local/T-1"},` + e(mbTrack3, ``) + `,` + e(mbTrack4, ``) + `,{"title":"track/local/T-2"}]`), ans(e2, `{"title":"track/local/T-1","versionId":"L1","method":"direct","protocol":"hls"}`, e3, e4, `{"title":"track/local/T-2","versionId":"L2","method":"transcode","protocol":"hls"}`)},
		{"a queue in the answer to a request without one: every entry dropped", req(``), ans(e2)},
	}
	for _, c := range answers {
		got, err := musicKeepQueueAnswer([]byte(c.request), []byte(c.text))
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
		sets9 = append(sets9, fmt.Sprintf(`{"line":[{"value":"set %d"}]}`, i))
	}
	for _, c := range []musicIn{
		{"plain lyrics: no line carries start", set(`{"line":[{"value":"Hello darkness"},{"value":""},{"value":"my old friend"}]}`)},
		{"synced lyrics: every line carries start", set(`{"lang":"en","kind":"main","line":[{"start":0,"value":"I am just a poor boy"},{"start":4210,"value":"Though my story's seldom told"}]}`)},
		{"a set mixing lines with and without start: skipped", set(`{"line":[{"start":0,"value":"a"},{"value":"b"}]},{"line":[{"value":"kept"}]}`)},
		{"a set mixing them, the line with start first or last: skipped either way", set(`{"line":[{"value":"a"},{"start":10,"value":"b"}]},{"line":[{"start":10,"value":"a"},{"start":20,"value":"b"},{"value":"c"}]},{"line":[{"start":0,"value":"kept"}]}`)},
		{"synced, offset, displayArtist and displayTitle, as before proposal 17: ignored", set(`{"synced":true,"offset":-250,"displayArtist":"Simon & Garfunkel","displayTitle":"The Boxer","line":[{"start":0,"value":"a"},{"start":10,"value":"b"}]}`)},
		{"synced true on lines without start: plain", set(`{"synced":true,"line":[{"value":"a"}]}`)},
		{"synced false on lines with start: synced", set(`{"synced":false,"line":[{"start":5,"value":"a"}]}`)},
		{"a translation and a pronunciation beside the main set", set(`{"lang":"en","line":[{"value":"a"}]},{"lang":"fr","kind":"translation","line":[{"value":"b"}]},{"lang":"ja-Latn","kind":"pronunciation","line":[{"value":"c"}]}`)},
		{"lang kept only as a tag is sent", set(`{"lang":"fr-CA","line":[{"value":"a"}]},{"lang":"fre","line":[{"value":"b"}]},{"lang":"fr-ca","line":[{"value":"c"}]},{"lang":"und","line":[{"value":"d"}]},{"lang":7,"line":[{"value":"e"}]}`)},
		{"a kind that isn't a token, or isn't a string: main", set(`{"kind":"Translation","line":[{"value":"a"}]},{"kind":2,"line":[{"value":"b"}]},{"kind":"","line":[{"value":"c"}]}`)},
		{"an unknown kind token kept as it is, never turned into main", set(`{"kind":"romanization","line":[{"value":"a"}]},{"kind":"other","line":[{"value":"b"}]}`)},
		{"a set that isn't an object skipped", set(`"x",{"line":[{"value":"c"}]}`)},
		{"line values cleaned, kept when empty, cut to 512 code points", set(`{"line":[{"value":"  two\t\tspaces \u0007"},{"value":" ​ "},{"value":"` + strings.Repeat("é", 511) + `ab"}]}`)},
		{"lines are text: markup kept as it is", set(`{"line":[{"value":"<b>loud</b> &amp; <script>x</script>"}]}`)},
		{"a line whose value isn't a string, or that isn't an object, skipped before the set is judged", set(`{"line":[{"value":7},{"start":5},"x",{"value":"kept"}]},{"line":[{"start":0,"value":"a"},{"value":7},{"start":10,"value":"b"}]}`)},
		{"synced: a start out of range or not an integer skips the line", set(`{"line":[{"start":-1,"value":"negative"},{"start":86400001,"value":"too late"},{"start":1.5,"value":"not an integer"},{"start":null,"value":"null"},{"start":"5","value":"a string"},{"start":0,"value":"zero"},{"start":86400000,"value":"last"}]}`)},
		{"synced: a start before the line before skipped, an equal one kept", set(`{"line":[{"start":1000,"value":"a"},{"start":999,"value":"before"},{"start":1000,"value":"same"},{"start":2000,"value":"b"}]}`)},
		{"at most 2,000 lines kept, the first", set(`{"line":[` + strings.Join(lines2001, ",") + `]}`)},
		{"at most 8 sets kept, the first", set(strings.Join(sets9, ","))},
		{"a set with no line left skipped", set(`{"line":[{"start":-1,"value":"bad start"}]},{"line":[]},{},{"line":{"value":"x"}},{"line":[{"value":"kept"}]}`)},
		{"no set left: the fetch fails", set(`{"line":[{"start":0,"value":"a"},{"value":"b"}]}`)},
		{"an empty structuredLyrics: the fetch fails", set(``)},
		{"a structuredLyrics that isn't an array: the fetch fails", `{"structuredLyrics":{"line":[{"value":"a"}]}}`},
		{"no structuredLyrics: the fetch fails", `{}`},
		{"an answer that isn't an object: the fetch fails", `[{"line":[{"value":"a"}]}]`},
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
			"MSFP Media version 0's music (https://github.com/msfp-io/msfp-media/blob/main/spec/v0/music.md), as MSFP's music part carries it (spec/v0/music.md); sections named alone are MSFP Media music's. Generated by tools/vectors; no implementation has produced them yet. The MBIDs are made up.",
			"keys (music section 2): valid says whether the text is a music title key, under the mb or the local scheme; kind is the kind it names.",
			"titles (music sections 3.2 and 3.3) and localTitles (MSFP Media listing section 4.3, music section 2.2): text is one entry of a titles answer, as JSON; the caller asked for the libraries listed in libraries. A title has no kind: its key names it, and a kind member is ignored. valid false: the caller skips the title. valid true: output is what it keeps, members in this order: key, library, name, artists, artistNames, primaryType, secondaryTypes, compilation, date, originalDate, genres, album, disc, number, copies, image, ids (its members sorted); absent members left out. A track's artists, artistNames, dates and genres are kept as sent: an absent one is its album's, which the caller doesn't fill in. ids are kept on mb albums (mbreleasegroup) and mb tracks (mbrecording) only.",
			"A kept copy's members, in this order: id, durationMs, mediaType, codec, codecs, size, etag, sampleRate, bitDepth, channels, replayGain (trackGain, trackPeak, albumGain, albumPeak), lyrics; at most 16 copies. ReplayGain values are written as the shortest decimal of the rounded number (1.0 is written 1); no case holds an exact tie of rounding. A copy of codec aac or dts kept without codecs is one the viewer's server doesn't play as it is (MSFP Media base section 3).",
			"queue (music section 5.2): kind request: text is a play request, valid says whether the owning server accepts its title and queue; reason starts with 400, or with 409 delivery-refused for a well-formed delivery it doesn't make, or 409 target-refused for a media type it doesn't make (MSFP Media playing section 2.2, step 7). Each queue entry is an object: title, optional versionId and delivery, with mediaType (remux) or transcode as the request's; the request's own delivery members and an entry's transcode targets are judged by playback.json's rules (MSFP Media playing), not here. kind answer: request is the play request, text the play answer; output is the queue the viewer's server keeps (an array, possibly empty), entries' members in this order: title, versionId, method, protocol. An entry takes its own delivery, else the request's; with none, protocol is hls. Entries carry no URL: the viewer's server builds <stream root><ticket>/queue/<n>/file or <stream root><ticket>/queue/<n>/master.m3u8 from the answer's ticket and the entry's place n in the request's queue. Only the queue member of the answer is judged here; playback's own members are playback.json's.",
			"lyrics (music section 6): kind request: text is a lyrics request, valid says whether the owning server reads it (otherwise 400 invalid-request). kind answer: text is a lyrics answer; valid false: the fetch fails; valid true: output is what the viewer's server keeps, each set's members in this order: lang, kind, line (start, value); kind always written. A set is synced when every line left carries start, plain when none does, skipped when they mix.",
			"Open values (MSFP Media base section 3): a copy's mediaType is a media type (lowercase, one /, no parameters, each half starting with a letter or a digit, at most 127 characters of a-z 0-9 . + - _ /) and its codec a token (1 to 32 characters of a-z 0-9 . _ -), kept as they are, known or not; a malformed one is dropped, and the copy, missing a required member, skipped. primaryType and each of secondaryTypes are MusicBrainz release-group types, lowercased, 1 to 32 characters of a-z 0-9 space . _ - /, no space at either end, kept as they are; any other value counts as absent (an entry of secondaryTypes skipped alone). codecs is an RFC 6381 string of 1 to 64 characters of A-Z a-z 0-9 . -; any other counts as absent. durationMs is kept exact.",
			"Names are cleaned as MSFP's core section 9.1 cleans a server's name (spec/v0/core.md), with 256 code points for name, 512 for artistNames, 64 for genres (a longer genre skipped, not cut), 512 for a lyrics line.",
		},
		Libraries:   musicLibraries,
		Keys:        musicKeyCases(),
		Titles:      musicTitleCases(),
		LocalTitles: musicLocalTitleCases(),
		Queue:       musicQueueCases(),
		Lyrics:      musicLyricsCases(),
	})
}
