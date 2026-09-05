package xtce_test

import (
	"github.com/ravisuhag/astro/internal/vectors"
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
	entries, err := os.ReadDir(filepath.Join(vectors.Root(), "xtce"))
	if err != nil {
		f.Fatalf("reading the shared XTCE fixtures: %v", err)
	}
	for _, entry := range entries {
		if filepath.Ext(entry.Name()) != ".xml" {
			continue
		}
		data, err := os.ReadFile(filepath.Join(vectors.Root(), "xtce", entry.Name()))
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

// FuzzExtractDynamic throws arbitrary packet bytes at the packet-derived
// layout path.
//
// FuzzLoad covers the database (XML) side of this package; nothing fuzzes the
// packet side, where a RepeatEntry's count and a binary field's width can both
// come straight out of the bytes being parsed rather than the database. The
// fixture below names both from packet fields wide enough (32 bits) to name a
// count or a width in the billions, which is exactly the shape MaxRepeatCount,
// MaxFields and the binary width cap exist to survive. The property is the
// same as FuzzLoad's: never panic, never hang, whatever the packet holds.
func FuzzExtractDynamic(f *testing.F) {
	db, err := xtce.Load(strings.NewReader(`<?xml version="1.0" encoding="UTF-8"?>
<SpaceSystem xmlns="` + xtce.Namespace + `" name="Sat">
  <TelemetryMetaData>
    <ParameterTypeSet>
      <IntegerParameterType name="U32" signed="false">
        <IntegerDataEncoding sizeInBits="32" encoding="unsigned"/>
      </IntegerParameterType>
      <IntegerParameterType name="U8" signed="false">
        <IntegerDataEncoding sizeInBits="8" encoding="unsigned"/>
      </IntegerParameterType>
      <BinaryParameterType name="Blob">
        <BinaryDataEncoding>
          <SizeInBits>
            <DynamicValue><ParameterInstanceRef parameterRef="Length"/></DynamicValue>
          </SizeInBits>
        </BinaryDataEncoding>
      </BinaryParameterType>
    </ParameterTypeSet>
    <ParameterSet>
      <Parameter name="Count" parameterTypeRef="U32"/>
      <Parameter name="Sample" parameterTypeRef="U8"/>
      <Parameter name="Length" parameterTypeRef="U32"/>
      <Parameter name="Data" parameterTypeRef="Blob"/>
    </ParameterSet>
    <ContainerSet>
      <SequenceContainer name="C">
        <EntryList>
          <ParameterRefEntry parameterRef="Count"/>
          <ParameterRefEntry parameterRef="Sample">
            <RepeatEntry>
              <Count><DynamicValue><ParameterInstanceRef parameterRef="Count"/></DynamicValue></Count>
            </RepeatEntry>
          </ParameterRefEntry>
          <ParameterRefEntry parameterRef="Length"/>
          <ParameterRefEntry parameterRef="Data"/>
        </EntryList>
      </SequenceContainer>
    </ContainerSet>
  </TelemetryMetaData>
</SpaceSystem>`))
	if err != nil {
		f.Fatalf("loading the fixture database: %v", err)
	}

	container, err := db.FindContainer("/Sat/C")
	if err != nil {
		f.Fatalf("FindContainer: %v", err)
	}

	f.Add([]byte{})
	f.Add([]byte{0, 0, 0, 0})
	f.Add([]byte{0, 0, 0, 4, 1, 2, 3, 4, 0, 0, 0, 16, 0xAA, 0xBB})
	// A repeat count near 2^32, which the loop over count must refuse
	// without appending a Field per repetition.
	f.Add([]byte{0xFF, 0xFF, 0xFF, 0xFF})
	// Count zero, then a binary width near 2^32 bits.
	f.Add([]byte{0, 0, 0, 0, 0xFF, 0xFF, 0xFF, 0xFF})
	f.Add([]byte{0x7F, 0xFF, 0xFF, 0xFF, 0x7F, 0xFF, 0xFF, 0xFF})

	f.Fuzz(func(t *testing.T, packet []byte) {
		layout, err := container.ResolveLayout(packet)
		if err != nil {
			return
		}
		if layout == nil {
			t.Fatal("ResolveLayout returned no error and no layout")
		}
		_, _ = layout.Extract(packet)
		_, _ = container.ExtractDynamic(packet)
	})
}
