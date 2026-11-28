package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
)

// mediaRepo is a checkout of msfp-media (-media).
var mediaRepo string

// mediaVectors is where MSFP's text finds MSFP Media's vector files.
const mediaVectors = "https://github.com/msfp-io/msfp-media/blob/main/vectors/"

// vectorLink is a link of the text to a vector file, MSFP's
// ([..](../../vectors/v0/<file>.json)) or MSFP Media's
// ([..](https://github.com/msfp-io/msfp-media/blob/main/vectors/v0/<file>.json)).
var vectorLink = regexp.MustCompile(`\]\(((?:\.\./)*vectors/|` + regexp.QuoteMeta(mediaVectors) + `)((?:v0|history)/[a-z-]+\.json)\)`)

// caseFiles are the files whose groups the text must each name: the ones
// that hold MSFP's carrier vectors of MSFP Media and the strict rules.
var caseFiles = []string{"v0/strict.json", "v0/playback.json", "v0/media.json"}

// verifyLinks checks the text against the files: every link to one of
// MSFP's vector files resolves from the document that holds it, every
// link to one of MSFP Media's names a file of a checkout of msfp-media;
// the groups a paragraph names before such a link (`a` and `b` in
// [vectors/v0/x.json]) are groups of that file; and every group of
// caseFiles is named so at least once.
func verifyLinks(read func(string) []byte, dir string) []error {
	var errs []error
	bad := func(format string, a ...any) { errs = append(errs, fmt.Errorf(format, a...)) }

	groupsOf := func(b []byte) map[string]bool {
		var f map[string]json.RawMessage
		if json.Unmarshal(b, &f) != nil {
			return nil
		}
		g := map[string]bool{}
		for k, v := range f {
			var list []json.RawMessage
			isCases := json.Unmarshal(v, &list) == nil && len(list) > 0 && list[0][0] == '{'
			if k != "about" && len(v) > 0 && (v[0] == '[' || v[0] == '{') {
				g[k] = isCases
			}
		}
		return g
	}
	own := map[string]map[string]bool{}
	files, _ := filepath.Glob(filepath.Join(dir, "*", "*.json"))
	for _, f := range files {
		rel, _ := filepath.Rel(dir, f)
		own[filepath.ToSlash(rel)] = groupsOf(read(filepath.ToSlash(rel)))
	}
	media := map[string]map[string]bool{}
	mediaFile := func(file string) map[string]bool {
		if g, ok := media[file]; ok {
			return g
		}
		b, err := os.ReadFile(filepath.Join(mediaRepo, "vectors", file))
		if err != nil {
			media[file] = nil
			return nil
		}
		media[file] = groupsOf(b)
		return media[file]
	}
	named := map[string]map[string]bool{}

	root := filepath.Join(dir, "..")
	var docs []string
	for _, pattern := range []string{"spec/*/*.md", "api/*.md", "conformance/*.md", "*.md"} {
		m, _ := filepath.Glob(filepath.Join(root, pattern))
		docs = append(docs, m...)
	}
	for _, doc := range docs {
		rel, _ := filepath.Rel(root, doc)
		rel = filepath.ToSlash(rel)
		b, err := os.ReadFile(doc)
		if err != nil {
			bad("%v", err)
			continue
		}
		for _, para := range paragraphs(string(b)) {
			from, carried := 0, ""
			links := vectorLink.FindAllStringSubmatchIndex(para, -1)
			for i, m := range links {
				base, file := para[m[2]:m[3]], para[m[4]:m[5]]
				var g map[string]bool
				where := file
				if base == mediaVectors {
					where = "MSFP Media's " + file
					if g = mediaFile(file); g == nil {
						bad("%s: links to MSFP Media's %s, not in %s (-media)", rel, file, mediaRepo)
					}
				} else {
					if got := path.Join(path.Dir(rel), base+file); got != path.Join("vectors", file) {
						bad("%s: %s%s resolves to %s", rel, base, file, got)
					}
					if g = own[file]; g == nil {
						bad("%s: links to %s, which isn't here", rel, file)
					}
					if named[file] == nil {
						named[file] = map[string]bool{}
					}
				}
				start := max(strings.LastIndex(para[:m[0]], "["), from)
				window := sentenceEnd(para[from:start])
				// "`a` in [x.json] and [y.json]": the names go on.
				if strings.TrimSpace(window) == "and" || strings.TrimSpace(window) == "," {
					window = carried
				}
				window += " " + tail(para, links, i)
				carried = window
				strict := strings.Contains(window, "Test cases") || strings.Contains(window, "test cases")
				for _, n := range groupName.FindAllStringSubmatch(window, -1) {
					if _, ok := g[n[1]]; ok {
						if base != mediaVectors {
							named[file][n[1]] = true
						}
					} else if g != nil && strict && looksLikeGroup(n[1]) {
						bad("%s: names `%s` in %s, which has no such group", rel, n[1], where)
					}
				}
				from = m[1]
			}
		}
	}
	for _, f := range caseFiles {
		var missing []string
		for g, cases := range own[f] {
			if cases && !named[f][g] {
				missing = append(missing, g)
			}
		}
		slices.Sort(missing)
		if len(missing) > 0 {
			bad("%s: the text names no link to its groups %v", f, missing)
		}
	}
	return errs
}
