package main

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
)

// verifyRevCatalog checks the catalog vectors of the revision of version 0
// (proposal 16), read back from the files: each value on its own.
func verifyRevCatalog(read func(string) []byte) []error {
	var errs []error
	bad := func(format string, a ...any) { errs = append(errs, fmt.Errorf("v0/strict.json: "+format, a...)) }

	raw := read("v0/strict.json")
	var f struct {
		Pages  []json.RawMessage `json:"titlePages"`
		Opaque []verdict         `json:"opaqueKeyParts"`
		Images []json.RawMessage `json:"localImages"`
		Tokens []verdict         `json:"openTokens"`
		Types  []verdict         `json:"openMediaTypes"`
		Kinds  json.RawMessage   `json:"libraryKinds"`
	}
	if err := json.Unmarshal(raw, &f); err != nil {
		bad("%v", err)
		return errs
	}
	if len(f.Pages) == 0 || len(f.Opaque) == 0 || len(f.Images) == 0 || len(f.Tokens) == 0 || len(f.Types) == 0 {
		bad("titlePages, opaqueKeyParts, localImages, openTokens or openMediaTypes missing or empty")
	}
	if f.Kinds != nil {
		bad("libraryKinds: libraries carry no kind (catalog section 2)")
	}

	// Each case holds exactly the members its kind defines.
	members := func(where string, b json.RawMessage, required, optional []string) {
		var m map[string]json.RawMessage
		if json.Unmarshal(b, &m) != nil {
			bad("%s: not an object", where)
			return
		}
		for _, k := range required {
			if _, ok := m[k]; !ok {
				bad("%s: no %s", where, k)
			}
		}
		for k := range m {
			if !strings.Contains(" "+strings.Join(append(required, optional...), " ")+" ", " "+k+" ") {
				bad("%s: unknown member %s", where, k)
			}
		}
	}

	names := map[string]bool{}
	seen := func(list, name string) {
		if name == "" || names[list+"\n"+name] {
			bad("%s: a case named %q empty or twice", list, name)
		}
		names[list+"\n"+name] = true
	}

	// titlePages: valid and caller derived again from the inputs.
	var fail, stop, on, valid, invalid int
	for _, b := range f.Pages {
		var c catalogPageCase
		if err := json.Unmarshal(b, &c); err != nil {
			bad("titlePages: %v", err)
			continue
		}
		seen("titlePages", c.Name)
		req := []string{"name", "changes", "titles", "gone", "bytes", "after", "valid", "caller"}
		if !c.Changes {
			req = append(req, "remain")
		}
		opt := []string{"lastKey", "next", "reason"}
		if c.Changes {
			opt = append(opt, "remain")
		}
		members("titlePages "+c.Name, b, req, opt)
		ok, r := reason(catalogCheckPage(c.catalogPage))
		if ok != c.Valid || r != c.Reason {
			bad("titlePages %q: the file says valid=%v %q, the rules give %v %q", c.Name, c.Valid, c.Reason, ok, r)
		}
		if got := catalogCallerStep(c.catalogPage); got != c.Caller {
			bad("titlePages %q: the file says the caller does %q, the rules give %q", c.Name, c.Caller, got)
		}
		switch c.Caller {
		case "fail":
			fail++
		case "stop":
			stop++
		case "ask on":
			on++
		}
		if c.Valid {
			valid++
		} else {
			invalid++
		}
	}
	if fail == 0 || stop == 0 || on == 0 || valid == 0 || invalid == 0 {
		bad("titlePages: not every outcome has a case")
	}

	// opaqueKeyParts: each verdict again; the bounds 1, 64 and 65 there.
	lens := map[int]bool{}
	texts := map[string]bool{}
	for _, v := range f.Opaque {
		if texts[v.Text] {
			bad("opaqueKeyParts %q: twice", v.Text)
		}
		texts[v.Text] = true
		ok, r := reason(catalogCheckOpaque(v.Text))
		if ok != v.Valid || r != v.Reason {
			bad("opaqueKeyParts %q: the file says valid=%v %q, the rules give %v %q", v.Text, v.Valid, v.Reason, ok, r)
		}
		lens[len(v.Text)] = true
	}
	if !lens[0] || !lens[1] || !lens[64] || !lens[65] {
		bad("opaqueKeyParts: no case of 0, 1, 64 or 65 characters")
	}

	// localImages: each side derived again.
	sides := map[string]int{}
	for _, b := range f.Images {
		var c catalogImageCase
		if err := json.Unmarshal(b, &c); err != nil {
			bad("localImages: %v", err)
			continue
		}
		seen("localImages "+c.Side, c.Name)
		sides[c.Side]++
		where := "localImages " + c.Side + " " + c.Name
		switch c.Side {
		case "request":
			members(where, b, []string{"name", "side", "offered", "body", "status"}, []string{"parts", "type"})
			st, ty := catalogImageRequest(c.Parts, c.Offered, []byte(c.Body))
			if st != c.Status || ty != c.Type {
				bad("localImages request %q: the file says %d %q, the rules give %d %q", c.Name, c.Status, c.Type, st, ty)
			}
			if st == 200 {
				if ty != "" {
					bad("localImages request %q: a 200 with a type", c.Name)
				}
			} else if ty == "" {
				bad("localImages request %q: a refusal without a type", c.Name)
			}
		case "answer":
			members(where, b, []string{"name", "side", "streamRoot", "body", "valid"}, []string{"ticket", "url", "reason"})
			if !catalogStreamRoot(c.StreamRoot) {
				bad("localImages answer %q: streamRoot %q isn't a stream root", c.Name, c.StreamRoot)
			}
			t, err := catalogImageAnswer([]byte(c.Body))
			ok, r := reason(err)
			if c.Valid == nil || ok != *c.Valid || r != c.Reason || t != c.Ticket {
				bad("localImages answer %q: the file and the rules differ (rules: %v %q %q)", c.Name, ok, t, r)
			}
			// The URL is built, never read: the stream root, the ticket
			// and the fixed name, and only for a kept answer.
			if want := map[bool]string{true: c.StreamRoot + t + "/image"}[ok]; c.URL != want {
				bad("localImages answer %q: url %q, the rules build %q", c.Name, c.URL, want)
			}
		case "image":
			members(where, b, []string{"name", "side", "valid"}, []string{"hex", "size", "format", "reason"})
			head, err := hex.DecodeString(c.Hex)
			if err != nil || checkHex(c.Hex) != nil {
				bad("localImages image %q: hex isn't lowercase hex", c.Name)
				continue
			}
			fm, err := catalogCheckImage(head, c.Size)
			ok, r := reason(err)
			if c.Valid == nil || ok != *c.Valid || r != c.Reason || fm != c.Format {
				bad("localImages image %q: the file and the rules differ (rules: %v %q %q)", c.Name, ok, fm, r)
			}
		default:
			bad("localImages %q: side %q", c.Name, c.Side)
		}
	}
	for _, s := range []string{"request", "answer", "image"} {
		if sides[s] == 0 {
			bad("localImages: no %s case", s)
		}
	}
	// openTokens: each verdict again, by the helper and by a pattern of
	// its own; the bounds 0, 1, 32 and 33 there, kept and dropped both.
	tokenRE := regexp.MustCompile(`^[a-z0-9._-]{1,32}$`)
	tokenLens := map[int]bool{}
	tokenTexts := map[string]bool{}
	var kept, dropped int
	for _, v := range f.Tokens {
		if tokenTexts[v.Text] {
			bad("openTokens %q: twice", v.Text)
		}
		tokenTexts[v.Text] = true
		ok, r := reason(openTokenReason(v.Text))
		if ok != v.Valid || r != v.Reason || openToken(v.Text) != v.Valid || tokenRE.MatchString(v.Text) != v.Valid {
			bad("openTokens %q: the file says valid=%v %q, the rules give %v %q", v.Text, v.Valid, v.Reason, ok, r)
		}
		tokenLens[len(v.Text)] = true
		if v.Valid {
			kept++
		} else {
			dropped++
		}
	}
	if !tokenLens[0] || !tokenLens[1] || !tokenLens[32] || !tokenLens[33] || kept == 0 || dropped == 0 {
		bad("openTokens: no case of 0, 1, 32 or 33 characters, or none kept or dropped")
	}

	// openMediaTypes: each verdict again, by the helper and by a pattern
	// of its own; the bounds 127 and 128 there, kept and dropped both.
	typeRE := regexp.MustCompile(`^[a-z0-9][a-z0-9.+_-]*/[a-z0-9][a-z0-9.+_-]*$`)
	typeLens := map[int]bool{}
	typeTexts := map[string]bool{}
	kept, dropped = 0, 0
	for _, v := range f.Types {
		if typeTexts[v.Text] {
			bad("openMediaTypes %q: twice", v.Text)
		}
		typeTexts[v.Text] = true
		ok, r := reason(openMediaTypeReason(v.Text))
		re := typeRE.MatchString(v.Text) && len(v.Text) <= 127
		if ok != v.Valid || r != v.Reason || openMediaType(v.Text) != v.Valid || re != v.Valid {
			bad("openMediaTypes %q: the file says valid=%v %q, the rules give %v %q", v.Text, v.Valid, v.Reason, ok, r)
		}
		typeLens[len(v.Text)] = true
		if v.Valid {
			kept++
		} else {
			dropped++
		}
	}
	if !typeLens[0] || !typeLens[127] || !typeLens[128] || kept == 0 || dropped == 0 {
		bad("openMediaTypes: no case of 0, 127 or 128 characters, or none kept or dropped")
	}
	return errs
}

// catalogStreamRoot tells a stream root (spec/v0/media.md section 1):
// an https address, a prefix of the base path's form, /api/v1/stream/.
func catalogStreamRoot(root string) bool {
	const pre, end = "https://", "/api/v1/stream/"
	if !strings.HasPrefix(root, pre) || !strings.HasSuffix(root, end) {
		return false
	}
	i := strings.IndexByte(root[len(pre):], '/') + len(pre)
	if checkAddress(root[:i], false) != nil {
		return false
	}
	return checkBase(strings.TrimSuffix(root[i:], end)+"/api/v1/federation/") == nil
}
