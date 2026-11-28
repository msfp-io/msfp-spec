// Command vectors makes and checks MSFP's test vectors.
//
//	go run .          checks every vector file against the specification's rules
//	go run . -write   writes vectors/history/test-draft-strict.json and vectors/v0/*.json, then checks
//
// The check also reads the specification's text: every link it gives to
// a vector file resolves, and every case group it names is in that file.
// MSFP's text links to MSFP Media's vectors too: -media names a checkout
// of msfp-media (by default beside this repository) to check those.
//
// vectors/history/test-draft.json is never written: it came from Overfin's code, and
// implementations keep copies of it. The check regenerates it and
// insists on the same bytes.
package main

import (
	"bytes"
	"flag"
	"fmt"
	"os"
	"path/filepath"
)

func main() {
	write := flag.Bool("write", false, "write the generated vector files before checking")
	dir := flag.String("dir", "../../vectors", "the vectors folder")
	media := flag.String("media", "../../../msfp-media", "a checkout of msfp-io/msfp-media, beside this one: the vectors MSFP's text links to")
	flag.Parse()
	mediaRepo = *media

	generated := []struct {
		path string
		make func() []byte
	}{
		{"history/test-draft-strict.json", genDraftStrict},
		{"v0/keys.json", genV0Keys},
		{"v0/code.json", genV0Code},
		{"v0/exchanges.json", genV0Exchanges},
		{"v0/rotation.json", genV0Rotation},
		{"v0/revocation.json", genV0Revocation},
		{"v0/strict.json", genV0Strict},
		{"v0/media.json", genV0Media},
		{"v0/groups.json", genV0Groups},
		{"v0/playback.json", genV0Playback},
	}

	failed := false
	fail := func(format string, a ...any) {
		fmt.Fprintf(os.Stderr, "FAIL "+format+"\n", a...)
		failed = true
	}

	if *write {
		for _, g := range generated {
			p := filepath.Join(*dir, g.path)
			if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
				fail("%v", err)
				continue
			}
			if err := os.WriteFile(p, g.make(), 0o644); err != nil {
				fail("%v", err)
			}
		}
	}

	// 1. Every file is what this program makes, byte for byte.
	read := func(rel string) []byte {
		b, err := os.ReadFile(filepath.Join(*dir, rel))
		if err != nil {
			fail("%v", err)
		}
		return b
	}
	if !bytes.Equal(read("history/test-draft.json"), genDraft()) {
		fail("history/test-draft.json: not what the test draft's rules give")
	}
	for _, g := range generated {
		if !bytes.Equal(read(g.path), g.make()) {
			fail("%s: differs from what this program makes (run with -write?)", g.path)
		}
	}

	// 2. Every value in the files checks out on its own, read back from
	// the files rather than from the generator's variables.
	for _, err := range verifyAll(read) {
		fail("%v", err)
	}

	// 3. The text's links to the files, MSFP's and MSFP Media's, resolve,
	// and the groups it names are in them.
	for _, err := range verifyLinks(read, *dir) {
		fail("%v", err)
	}

	if failed {
		os.Exit(1)
	}
	fmt.Println("ok: every vector checks out")
}
