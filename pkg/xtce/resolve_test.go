package xtce_test

import (
	"errors"
	"testing"

	"github.com/ravisuhag/astro/pkg/xtce"
)

func TestQualifiedNames(t *testing.T) {
	db := load(t, "nested.xml")

	tests := []struct {
		system *xtce.SpaceSystem
		want   string
	}{
		{db, "/Spacecraft"},
		{db.SubSystems[0], "/Spacecraft/Power"},
		{db.SubSystems[1], "/Spacecraft/Thermal"},
	}
	for _, test := range tests {
		if got := test.system.QualifiedName(); got != test.want {
			t.Errorf("QualifiedName() = %q, want %q", got, test.want)
		}
	}
}

func TestFindByQualifiedName(t *testing.T) {
	db := load(t, "nested.xml")

	param, err := db.FindParameter("/Spacecraft/Power/BusVoltage")
	if err != nil {
		t.Fatalf("FindParameter() = %v", err)
	}
	if param.Name != "BusVoltage" {
		t.Errorf("found %q, want BusVoltage", param.Name)
	}

	container, err := db.FindContainer("/Spacecraft/Common")
	if err != nil {
		t.Fatalf("FindContainer() = %v", err)
	}
	if container.Name != "Common" {
		t.Errorf("found %q, want Common", container.Name)
	}

	paramType, err := db.FindParameterType("/Spacecraft/Power/Volts_t")
	if err != nil {
		t.Fatalf("FindParameterType() = %v", err)
	}
	if paramType.TypeName() != "Volts_t" {
		t.Errorf("found %q, want Volts_t", paramType.TypeName())
	}

	system, err := db.FindSpaceSystem("/Spacecraft/Thermal")
	if err != nil {
		t.Fatalf("FindSpaceSystem() = %v", err)
	}
	if system.Name != "Thermal" {
		t.Errorf("found %q, want Thermal", system.Name)
	}
}

func TestFindMisses(t *testing.T) {
	db := load(t, "nested.xml")

	tests := []struct {
		name string
		find func() error
	}{
		{"no such parameter", func() error {
			_, err := db.FindParameter("/Spacecraft/Power/NoSuchThing")
			return err
		}},
		{"no such system", func() error {
			_, err := db.FindParameter("/Spacecraft/Nowhere/BusVoltage")
			return err
		}},
		{"no such container", func() error {
			_, err := db.FindContainer("/Spacecraft/Missing")
			return err
		}},
		{"wrong root", func() error {
			_, err := db.FindSpaceSystem("/Elsewhere/Power")
			return err
		}},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if err := test.find(); !errors.Is(err, xtce.ErrNotFound) {
				t.Errorf("err = %v, want ErrNotFound", err)
			}
		})
	}
}

// TestFindRejectsUnqualifiedNames checks that Find is strict where Resolve is
// permissive: Find takes a full path, Resolve takes a reference.
func TestFindRejectsUnqualifiedNames(t *testing.T) {
	db := load(t, "nested.xml")

	if _, err := db.FindParameter("BusVoltage"); !errors.Is(err, xtce.ErrInvalidReference) {
		t.Errorf("FindParameter(bare name) = %v, want ErrInvalidReference", err)
	}
}

// TestResolveReferenceForms walks the three shapes a NameReference can take,
// each resolved from the SpaceSystem it would have been written in.
func TestResolveReferenceForms(t *testing.T) {
	db := load(t, "nested.xml")
	power := db.SubSystems[0]
	thermal := db.SubSystems[1]

	tests := []struct {
		name string
		from *xtce.SpaceSystem
		ref  string
		want string
	}{
		{"bare name in its own system", power, "BusVoltage", "BusVoltage"},
		{"absolute from the root", thermal, "/Spacecraft/Power/BusVoltage", "BusVoltage"},
		{"relative going up then down", thermal, "../Power/BusVoltage", "BusVoltage"},
		{"explicit here", power, "./BusVoltage", "BusVoltage"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			param, err := test.from.ResolveParameter(test.ref)
			if err != nil {
				t.Fatalf("ResolveParameter(%q) = %v", test.ref, err)
			}
			if param.Name != test.want {
				t.Errorf("resolved to %q, want %q", param.Name, test.want)
			}
		})
	}
}

// TestBareNameSearchesAncestors is the rule that makes a shared type library
// work: a name defined once near the root is usable from anywhere below.
func TestBareNameSearchesAncestors(t *testing.T) {
	db := load(t, "nested.xml")
	power := db.SubSystems[0]

	// PacketID lives in the root, not in Power.
	param, err := power.ResolveParameter("PacketID")
	if err != nil {
		t.Fatalf("ResolveParameter(PacketID) from Power = %v", err)
	}
	if param.Name != "PacketID" {
		t.Errorf("resolved to %q, want PacketID", param.Name)
	}

	// Counter_t likewise.
	if _, err := power.ResolveParameterType("Counter_t"); err != nil {
		t.Errorf("ResolveParameterType(Counter_t) from Power = %v", err)
	}
}

// TestPathReferenceDoesNotSearchAncestors is the other half of the rule. A
// bare name searches upwards; a path says exactly where to look, so a miss is
// a miss.
func TestPathReferenceDoesNotSearchAncestors(t *testing.T) {
	db := load(t, "nested.xml")
	power := db.SubSystems[0]

	// PacketID is in the root, so ./PacketID from Power must not find it.
	if _, err := power.ResolveParameter("./PacketID"); !errors.Is(err, xtce.ErrUnresolvedReference) {
		t.Errorf("ResolveParameter(./PacketID) from Power = %v, want it to miss", err)
	}
}

func TestResolveContainerAcrossSystems(t *testing.T) {
	db := load(t, "nested.xml")
	power := db.SubSystems[0]

	// The PowerPacket container's first entry points up a level.
	container, err := power.ResolveContainer("../Common")
	if err != nil {
		t.Fatalf("ResolveContainer(../Common) = %v", err)
	}
	if container.Name != "Common" {
		t.Errorf("resolved to %q, want Common", container.Name)
	}
}

func TestResolveRejectsMalformedReferences(t *testing.T) {
	db := load(t, "nested.xml")

	tests := []string{
		"",             // nothing
		"/",            // a slash and no name
		"Power//Volts", // an empty segment
		"Power/..",     // ending in a path segment
		"Bad:Name",     // a character the pattern forbids
		"Bad.Name",     // a dot inside a segment, which only "." and ".." may have
		"Power/A.B/X",  // same, mid-path
		"../../TooFar", // above the root
	}

	for _, ref := range tests {
		t.Run(ref, func(t *testing.T) {
			_, err := db.ResolveParameter(ref)
			if err == nil {
				t.Fatalf("ResolveParameter(%q) succeeded", ref)
			}
			if !errors.Is(err, xtce.ErrInvalidReference) && !errors.Is(err, xtce.ErrUnresolvedReference) {
				t.Errorf("err = %v, want an invalid or unresolved reference", err)
			}
		})
	}
}

// TestParameterNamespaceSpansBothMetadataSides matches the schema's
// parameterNameKey, whose selector covers TelemetryMetaData/ParameterSet and
// CommandMetaData/ParameterSet together.
func TestParameterNamespaceSpansBothMetadataSides(t *testing.T) {
	db := load(t, "invalid-duplicate-name.xml")

	// Two parameters called Reading, one on each side. Both are returned.
	if got := len(db.Parameters()); got != 2 {
		t.Errorf("Parameters() returned %d, want 2 across both sides", got)
	}
}

func TestParameterTypeSetAll(t *testing.T) {
	db := load(t, "ccsds-header.xml")

	types := db.ParameterTypes()
	if len(types) != 7 {
		t.Fatalf("%d parameter types, want 7", len(types))
	}

	// All() orders by kind, not by document order: integers first, then the
	// enumerations, then the boolean.
	kinds := make([]string, 0, len(types))
	for _, paramType := range types {
		kinds = append(kinds, paramType.TypeKind())
	}
	want := []string{"integer", "integer", "integer", "integer", "enumerated", "enumerated", "boolean"}
	for i := range want {
		if kinds[i] != want[i] {
			t.Errorf("type %d is %q, want %q (full order: %v)", i, kinds[i], want[i], kinds)
			break
		}
	}
}
