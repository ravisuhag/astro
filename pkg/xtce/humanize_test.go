package xtce_test

import (
	"strings"
	"testing"
)

func TestHumanizeCCSDSHeader(t *testing.T) {
	out := load(t, "ccsds-header.xml").Humanize()

	for _, want := range []string{
		"SpaceSystem /CCSDS",
		"7 parameters, 7 types, 1 containers, 0 commands",
		"Version -> Version_t (integer, 3 bits)",
		"APID -> APID_t (integer, 11 bits)",
		"PacketType -> PacketType_t (enumerated, 1 bits)",
		"SecondaryHeaderFlag -> SecondaryHeaderFlag_t (boolean, 1 bits)",
		"PrimaryHeader (abstract), 7 entries",
		"ParameterRefEntry PacketLength",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("output is missing %q\n---\n%s", want, out)
		}
	}
}

// TestHumanizeShowsWireWidthNotValueWidth: BusVoltage is a 32-bit float in
// software and twelve bits on the downlink, and the listing must show the
// twelve, because that is what a reader needs to decode a packet.
func TestHumanizeShowsWireWidthNotValueWidth(t *testing.T) {
	out := load(t, "nested.xml").Humanize()

	if !strings.Contains(out, "BusVoltage -> Volts_t (float, 12 bits), polynomial calibrator") {
		t.Errorf("output does not show the encoded width and calibrator\n---\n%s", out)
	}
	if !strings.Contains(out, "BusCurrent -> Amps_t (float, 12 bits), spline calibrator") {
		t.Errorf("output does not show the spline calibrator\n---\n%s", out)
	}
}

func TestHumanizeIndentsTheTree(t *testing.T) {
	out := load(t, "nested.xml").Humanize()
	lines := strings.Split(out, "\n")

	var rootLine, childLine string
	for _, line := range lines {
		switch {
		case strings.HasSuffix(line, "SpaceSystem /Spacecraft"):
			rootLine = line
		case strings.HasSuffix(line, "SpaceSystem /Spacecraft/Power"):
			childLine = line
		}
	}

	if rootLine == "" || childLine == "" {
		t.Fatalf("both systems should appear\n---\n%s", out)
	}
	if strings.HasPrefix(rootLine, " ") {
		t.Errorf("the root is indented: %q", rootLine)
	}
	if !strings.HasPrefix(childLine, "  ") {
		t.Errorf("the child is not indented: %q", childLine)
	}
}

// TestHumanizeMarksUnresolvedTypes: the listing is a debugging tool, so it
// must say when a parameter's type is missing rather than skipping it.
func TestHumanizeMarksUnresolvedTypes(t *testing.T) {
	out := load(t, "invalid-unresolved-type.xml").Humanize()

	if !strings.Contains(out, "Reading -> NoSuchType_t (unresolved)") {
		t.Errorf("output does not flag the dangling type\n---\n%s", out)
	}
}

func TestHumanizeShowsInheritance(t *testing.T) {
	out := load(t, "nested.xml").Humanize()

	if !strings.Contains(out, "ThermalPacket extends /Spacecraft/Common, 2 entries") {
		t.Errorf("output does not show container inheritance\n---\n%s", out)
	}
	if !strings.Contains(out, "ContainerRefEntry ../Common") {
		t.Errorf("output does not show the container entry\n---\n%s", out)
	}
}
