package main

// The titles request as the owning server reads it (MSFP Media listing
// sections 3, 3.3 and 3.6, as MSFP's catalog carries it): which
// libraries it lists, the filters kind and under, and its refusals.
// Paging and the changes log are listing section 3.1's and 3.2's
// (titlePages); here a request is answered with every title it lists
// after after, and whether it may be a changes answer at all.

import (
	"encoding/json"
	"errors"
	"slices"
	"strconv"
	"strings"
)

// listingTitle is a title the owning server holds, in the libraries it
// is in, with what under goes by: an episode's place.season (an episode
// keyed by its TMDB episode ID), a track's album, an album's artists.
type listingTitle struct {
	Key       string   `json:"key"`
	Libraries []string `json:"libraries"`
	Season    *int64   `json:"season,omitempty"`
	Album     string   `json:"album,omitempty"`
	Artists   []string `json:"artists,omitempty"`
}

// listingOffer is what the owning server holds and what the caller may
// see of it: the libraries the caller may see (visible), and its titles,
// some in libraries the caller may not see.
type listingOffer struct {
	Visible []string       `json:"visible"`
	Titles  []listingTitle `json:"titles"`
}

// listingListed is one title of the answer: its key and library.
type listingListed struct {
	Key     string `json:"key"`
	Library string `json:"library"`
}

// listingAnswer is what the owning server lists for a request: the
// titles, in byte order of key, and whether the answer may be a changes
// answer (since above 0, no under), else a whole list whatever its log.
type listingAnswer struct {
	Titles       []listingListed `json:"titles"`
	MayBeChanges bool            `json:"mayBeChanges"`
}

// listingKinds: the kinds of each content part (listing section 3.6).
var listingKinds = map[string][]string{
	"video": {"movie", "series", "season", "episode", "extra"},
	"music": {"artist", "album", "track"},
}

// listingKindOf is the kind a key names and its content part, or "".
func listingKindOf(key string) (kind, part string) {
	if k, err := parseVideoKey(key); err == nil {
		return k.Kind, "video"
	}
	if k, err := checkMusicKey(key); err == nil {
		return k, "music"
	}
	return "", ""
}

// listingUnder: whether t is directly under the title key u (listing
// section 3.6's table). u is compared as it is, not checked against the
// key forms.
func listingUnder(u string, t listingTitle) bool {
	uk, upart := listingKindOf(u)
	kind, _ := listingKindOf(t.Key)
	if upart == "video" {
		vk, _ := parseVideoKey(t.Key)
		if kind == "extra" {
			return vk.Parent == u
		}
		switch uk {
		case "series":
			rest, ok := strings.CutPrefix(t.Key, u+"/")
			if !ok || strings.Contains(rest, "/") {
				return false
			}
			return kind == "season" || kind == "episode" && strings.HasPrefix(rest, "a")
		case "season":
			if kind != "episode" {
				return false
			}
			if rest, ok := strings.CutPrefix(t.Key, u+"/"); ok && !strings.Contains(rest, "/") {
				return true
			}
			// An episode keyed by its TMDB episode ID, by its place.
			series, season, _ := strings.Cut(strings.TrimPrefix(u, vk.Series), "/s")
			n, err := strconv.ParseInt(season, 10, 64)
			return vk.ByID && series == "" && strings.HasPrefix(u, vk.Series+"/s") && err == nil && t.Season != nil && *t.Season == n
		}
		return false
	}
	switch uk {
	case "album":
		return kind == "track" && t.Album == u
	case "artist":
		return kind == "album" && slices.Contains(t.Artists, u)
	}
	return false
}

// listingRequest answers a titles request (body) of a caller, the
// content parts parts in use between the two: 200 with what it lists,
// or 404 not-found for a request listing section 3.3 refuses.
func listingRequest(offer listingOffer, parts []string, body []byte) (listingAnswer, error) {
	var a listingAnswer
	if checkIJSON(body) != nil {
		return a, errors.New("not valid JSON")
	}
	var o map[string]json.RawMessage
	if json.Unmarshal(body, &o) != nil || o == nil {
		return a, errors.New("not a JSON object")
	}
	str := func(name string) (string, bool, error) {
		raw, ok := o[name]
		if !ok {
			return "", false, nil
		}
		var s string
		if len(raw) == 0 || raw[0] != '"' || json.Unmarshal(raw, &s) != nil {
			return "", true, errors.New(name + " isn't a string")
		}
		return s, true, nil
	}
	requested := offer.Visible
	if raw, ok := o["libraries"]; ok {
		var libs []string
		if len(raw) == 0 || raw[0] != '[' || json.Unmarshal(raw, &libs) != nil {
			return a, errors.New("libraries isn't an array of strings")
		}
		requested = nil
		for _, l := range libs {
			if slices.Contains(offer.Visible, l) {
				requested = append(requested, l) // those it may not see are ignored
			}
		}
	}
	kind, hasKind, err := str("kind")
	if err != nil {
		return a, err
	}
	if hasKind && !openToken(kind) {
		return a, errors.New("kind isn't a token")
	}
	under, hasUnder, err := str("under")
	if err != nil {
		return a, err
	}
	if len(under) > 512 {
		return a, errors.New("under over 512 bytes")
	}
	after, _, err := str("after")
	if err != nil {
		return a, err
	}
	if len(after) > 512 {
		return a, errors.New("after over 512 bytes")
	}
	since := int64(0)
	if raw, ok := o["since"]; ok {
		t := string(raw)
		if checkInteger(t) != nil {
			return a, errors.New("since isn't an integer")
		}
		if n, err := strconv.ParseInt(t, 10, 64); err == nil {
			since = n
		} else if !strings.HasPrefix(t, "-") {
			since = 1 << 62
		}
	}
	a.MayBeChanges = since > 0 && !hasUnder
	a.Titles = []listingListed{}
	for _, t := range offer.Titles {
		k, part := listingKindOf(t.Key)
		if !slices.Contains(parts, part) || t.Key <= after {
			continue
		}
		lib := ""
		for _, l := range t.Libraries {
			if slices.Contains(requested, l) && (lib == "" || l < lib) {
				lib = l
			}
		}
		if lib == "" || hasKind && k != kind || hasUnder && !listingUnder(under, t) {
			continue
		}
		a.Titles = append(a.Titles, listingListed{t.Key, lib})
	}
	slices.SortFunc(a.Titles, func(x, y listingListed) int { return strings.Compare(x.Key, y.Key) })
	return a, nil
}
