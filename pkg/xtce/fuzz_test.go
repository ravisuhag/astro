package xtce_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ravisuhag/astro/pkg/xtce"
)

// FuzzLoad throws arbitrary bytes at the loader.
//
// Load is this package's only untrusted-input entry point (everything else
// works on an already-parsed tree) so it is the only thing worth fuzzing. The
// properties are that it never panics and never hangs, which is what the size
// cap and the depth pre-scan exist to guarantee. Validate and Humanize run too
// whenever Load succeeds, because a document that parses can still steer them
// somewhere odd.
func FuzzLoad(f *testing.F) {
	entries, err := os.ReadDir("testdata")
	if err != nil {
		f.Fatalf("reading testdata: %v", err)
	}
	for _, entry := range entries {
		if filepath.Ext(entry.Name()) != ".xml" {
			continue
		}
		data, err := os.ReadFile(filepath.Join("testdata", entry.Name()))
		if err != nil {
			f.Fatalf("reading %s: %v", entry.Name(), err)
		}
		f.Add(data)
		// A truncated fixture, so the corpus starts with something that ends
		// mid-element.
		if len(data) > 100 {
			f.Add(data[:len(data)/2])
		}
	}

	f.Add([]byte{})
	f.Add([]byte(`<SpaceSystem xmlns="` + xtce.Namespace + `" name="X"/>`))
	f.Add([]byte(strings.Repeat("<a>", 50)))

	f.Fuzz(func(t *testing.T, data []byte) {
		db, err := xtce.Load(strings.NewReader(string(data)))
		if err != nil {
			return
		}
		if db == nil {
			t.Fatal("Load returned no error and no database")
		}

		// A loaded database must be safe to work with, however strange its
		// contents.
		_ = db.Validate()
		_ = db.Humanize()
		_ = db.QualifiedName()

		db.Walk(func(system *xtce.SpaceSystem) bool {
			_ = system.Parameters()
			_ = system.ParameterTypes()
			_ = system.Containers()
			_ = system.MetaCommands()
			for _, param := range system.Parameters() {
				_, _ = system.ResolveParameterType(param.ParameterTypeRef)
			}
			for _, container := range system.Containers() {
				for _, entry := range container.EntryList.Entries {
					_, _ = system.ResolveParameter(entry.Ref)
					_, _ = system.ResolveContainer(entry.Ref)
				}
			}
			return true
		})
	})
}
