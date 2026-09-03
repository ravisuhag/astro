package ocsc_test

import (
	"testing"

	"github.com/ravisuhag/astro/internal/vectors"
	"github.com/ravisuhag/astro/pkg/ocsc"
)

// The wire vectors for this package live in vectors/ocsc/. Most of this
// package works on bit strings — bit-level lengths, termination digits,
// sequence indicators — which the octet-string vector format cannot
// express, so those stay as Go tests. The sync marker is portable.

func TestASMVector(t *testing.T) {
	vectors.RunFile(t, "ocsc/asm.json", vectors.Impl{
		EncodeFn: func(f, _ vectors.Fields) ([]byte, error) {
			return ocsc.DefaultASM(), nil
		},
	})
}

// TestASMMatchesTMSyncMarker states the fact that both packages carry the
// same marker, so a drift in either is caught here rather than by a
// receiver. The value itself is pinned in vectors/ocsc/asm.json and
// vectors/tmsc/cadu.json independently.
func TestASMMatchesTMSyncMarker(t *testing.T) {
	optical, err := vectors.Load("ocsc/asm.json")
	if err != nil {
		t.Fatalf("loading optical vectors: %v", err)
	}
	tm, err := vectors.Load("tmsc/cadu.json")
	if err != nil {
		t.Fatalf("loading TM vectors: %v", err)
	}

	var opticalWant, tmWant string
	for _, v := range optical.Encode {
		if v.Name == "attached-sync-marker" {
			opticalWant = v.Want
		}
	}
	for _, v := range tm.Encode {
		if v.Name == "attached-sync-marker" {
			tmWant = v.Want
		}
	}
	if opticalWant == "" || tmWant == "" {
		t.Fatal("a sync marker vector is missing from one of the two files")
	}
	if opticalWant != tmWant {
		t.Errorf("optical ASM %s and TM ASM %s have diverged; CCSDS 142.0-B-1 clause 3.3.2 and 131.0-B-5 clause 9 specify the same marker",
			opticalWant, tmWant)
	}
}
