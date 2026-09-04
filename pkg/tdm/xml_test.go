package tdm_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/ravisuhag/astro/pkg/tdm"
)

// Both forms must give the same values, and above all the same units — which
// come from the segment in either form, never from the record.
func TestFormsAgree(t *testing.T) {
	fromKVN, err := tdm.Decode([]byte(figureE19))
	if err != nil {
		t.Fatalf("Decode: %v", err)
	}
	encoded, err := fromKVN.EncodeXML()
	if err != nil {
		t.Fatalf("EncodeXML: %v", err)
	}
	fromXML, err := tdm.DecodeXML(encoded)
	if err != nil {
		t.Fatalf("DecodeXML: %v\n%s", err, encoded)
	}

	if fromXML.Header.Originator != fromKVN.Header.Originator {
		t.Errorf("originator differs")
	}
	if !fromXML.Header.CreationDate.Equal(fromKVN.Header.CreationDate) {
		t.Errorf("creation date differs")
	}
	if len(fromXML.Segments) != len(fromKVN.Segments) ||
		fromXML.Observations() != fromKVN.Observations() {
		t.Fatalf("shape differs: %d/%d and %d/%d",
			len(fromKVN.Segments), fromKVN.Observations(),
			len(fromXML.Segments), fromXML.Observations())
	}

	for i := range fromKVN.Segments {
		a, b := fromKVN.Segments[i], fromXML.Segments[i]

		// The range units are the whole point of this message's metadata.
		if a.Metadata.RangeUnits() != b.Metadata.RangeUnits() {
			t.Errorf("segment %d range units differ: %q and %q",
				i, a.Metadata.RangeUnits(), b.Metadata.RangeUnits())
		}
		modulusA, okA := a.Metadata.RangeModulus()
		modulusB, okB := b.Metadata.RangeModulus()
		if okA != okB || modulusA != modulusB {
			t.Errorf("segment %d range modulus differs", i)
		}
		if len(a.Metadata.Fields) != len(b.Metadata.Fields) {
			t.Errorf("segment %d metadata field count differs: %d and %d",
				i, len(a.Metadata.Fields), len(b.Metadata.Fields))
		}
		for _, keyword := range []string{"TRANSMIT_DELAY_1", "CORRECTION_RANGE", "PATH"} {
			x, _ := a.Metadata.Get(keyword)
			y, _ := b.Metadata.Get(keyword)
			if x != y {
				t.Errorf("segment %d %s differs: %q and %q", i, keyword, x, y)
			}
		}

		for j := range a.Observations {
			if a.Observations[j] != b.Observations[j] {
				t.Errorf("segment %d observation %d differs:\n\t%+v\n\t%+v",
					i, j, a.Observations[j], b.Observations[j])
			}
		}
	}
}

// One record becomes one <observation> carrying its epoch and its measurement.
func TestXMLShape(t *testing.T) {
	m, err := tdm.Decode([]byte(figureE19))
	if err != nil {
		t.Fatalf("Decode: %v", err)
	}
	encoded, err := m.EncodeXML()
	if err != nil {
		t.Fatalf("EncodeXML: %v", err)
	}
	source := string(encoded)

	if got := strings.Count(source, "<observation>"); got != m.Observations() {
		t.Errorf("wrote %d observation blocks for %d records", got, m.Observations())
	}
	// The TDM names its own schema, which is issue 2.0 rather than the ODM's.
	if !strings.Contains(source, "ndmxml-2.0.0-master-2.0.xsd") {
		t.Errorf("the TDM schema location is wrong:\n%s", source)
	}
	// Its worked example declares the NDM namespace, so it is written.
	if !strings.Contains(source, `xmlns:ndm="urn:ccsds:schema:ndmxml"`) {
		t.Errorf("the NDM namespace was not declared:\n%s", source)
	}
	// The metadata's units move to attributes like anything else.
	if !strings.Contains(source, "<RANGE_UNITS>RU</RANGE_UNITS>") {
		t.Errorf("RANGE_UNITS is not where it should be:\n%s", source)
	}
}

// Clause 3.4.3 pairs a timetag with one observable. An observation carrying
// two measurements has no timetag for the second, so it is refused.
func TestXMLRejectsTwoMeasurementsInOneObservation(t *testing.T) {
	m, err := tdm.Decode([]byte(figureE19))
	if err != nil {
		t.Fatalf("Decode: %v", err)
	}
	encoded, err := m.EncodeXML()
	if err != nil {
		t.Fatalf("EncodeXML: %v", err)
	}

	doubled := strings.Replace(string(encoded),
		"<RANGE>65249.6771931631</RANGE>",
		"<RANGE>65249.6771931631</RANGE><PR_N0>30.0</PR_N0>", 1)
	if doubled == string(encoded) {
		t.Fatal("the test input did not change")
	}
	if _, err := tdm.DecodeXML([]byte(doubled)); !errors.Is(err, tdm.ErrMalformedRecord) {
		t.Errorf("DecodeXML = %v, want ErrMalformedRecord", err)
	}
}

func TestXMLRejects(t *testing.T) {
	m, err := tdm.Decode([]byte(figureE19))
	if err != nil {
		t.Fatalf("Decode: %v", err)
	}
	encoded, err := m.EncodeXML()
	if err != nil {
		t.Fatalf("EncodeXML: %v", err)
	}
	source := string(encoded)

	tests := []struct {
		name  string
		input string
		want  error
	}{
		{
			name:  "no TIME_SYSTEM",
			input: strings.Replace(source, "<TIME_SYSTEM>UTC</TIME_SYSTEM>", "", 1),
			want:  tdm.ErrMissingTimeSystem,
		},
		{
			name: "no participant",
			input: strings.NewReplacer(
				"<PARTICIPANT_1>DSS-14</PARTICIPANT_1>", "",
				"<PARTICIPANT_2>CAS</PARTICIPANT_2>", "",
			).Replace(source),
			want: tdm.ErrMissingParticipant,
		},
		{
			name:  "a metadata keyword no table lists",
			input: strings.Replace(source, "<MODE>SEQUENTIAL</MODE>", "<NOPE>1</NOPE>", 1),
			want:  tdm.ErrUnknownKeyword,
		},
		{
			name:  "a data keyword no table lists",
			input: strings.Replace(source, "<PR_N0>30.2351</PR_N0>", "<NOPE>1.0</NOPE>", 1),
			want:  tdm.ErrUnknownKeyword,
		},
		{
			name:  "an observation with no measurement",
			input: strings.Replace(source, "<RANGE>65249.6771931631</RANGE>", "", 1),
			want:  tdm.ErrMalformedRecord,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := tdm.DecodeXML([]byte(tt.input)); !errors.Is(err, tt.want) {
				t.Errorf("DecodeXML = %v, want %v", err, tt.want)
			}
		})
	}
}
