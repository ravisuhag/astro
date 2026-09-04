package cdm_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/ravisuhag/astro/pkg/cdm"
)

// The two forms must agree. Clause 3.1.1 of the CDM and section 4 offer both,
// and a partner may send either, so the values a reader gets must not depend
// on which arrived.
func TestXMLAgreesWithKVN(t *testing.T) {
	fromKVN, err := cdm.Decode([]byte(clause362))
	if err != nil {
		t.Fatalf("Decode: %v", err)
	}

	encoded, err := fromKVN.EncodeXML()
	if err != nil {
		t.Fatalf("EncodeXML: %v", err)
	}
	fromXML, err := cdm.DecodeXML(encoded)
	if err != nil {
		t.Fatalf("DecodeXML: %v\n%s", err, encoded)
	}

	if fromXML.Header.Version != fromKVN.Header.Version ||
		fromXML.Header.MessageID != fromKVN.Header.MessageID ||
		fromXML.Header.Originator != fromKVN.Header.Originator {
		t.Errorf("header differs:\n\t%+v\n\t%+v", fromKVN.Header, fromXML.Header)
	}
	if !fromXML.Header.CreationDate.Equal(fromKVN.Header.CreationDate) {
		t.Errorf("creation date differs: %v and %v",
			fromKVN.Header.CreationDate, fromXML.Header.CreationDate)
	}

	a, _ := fromKVN.TCA()
	b, _ := fromXML.TCA()
	if !a.Equal(b) {
		t.Errorf("TCA differs: %v and %v", a, b)
	}

	missA, _ := fromKVN.MissDistance()
	missB, _ := fromXML.MissDistance()
	if missA != missB {
		t.Errorf("miss distance differs: %v and %v", missA, missB)
	}

	for i := range fromKVN.Objects {
		x, y := fromKVN.Objects[i], fromXML.Objects[i]
		if x.Name() != y.Name() || x.Designator() != y.Designator() {
			t.Errorf("object %d identity differs: %q/%q and %q/%q",
				i+1, x.Name(), x.Designator(), y.Name(), y.Designator())
		}
		// The asymmetry an operator acts on must survive the conversion.
		xMoves, _ := x.Maneuverable()
		yMoves, _ := y.Maneuverable()
		if xMoves != yMoves {
			t.Errorf("object %d maneuverability differs", i+1)
		}
		if x.CovarianceOrder() != y.CovarianceOrder() {
			t.Errorf("object %d covariance order differs: %d and %d",
				i+1, x.CovarianceOrder(), y.CovarianceOrder())
		}
		if x.Covariance() != y.Covariance() {
			t.Errorf("object %d covariance differs", i+1)
		}
		if pa, va, _ := x.StateVector(); true {
			pb, vb, _ := y.StateVector()
			if pa != pb || va != vb {
				t.Errorf("object %d state vector differs", i+1)
			}
		}
	}
}

func TestXMLRoundTrip(t *testing.T) {
	first, err := cdm.Decode([]byte(clause362))
	if err != nil {
		t.Fatalf("Decode: %v", err)
	}
	encoded, err := first.EncodeXML()
	if err != nil {
		t.Fatalf("EncodeXML: %v", err)
	}
	second, err := cdm.DecodeXML(encoded)
	if err != nil {
		t.Fatalf("DecodeXML: %v", err)
	}
	again, err := second.EncodeXML()
	if err != nil {
		t.Fatalf("EncodeXML on a decoded message: %v", err)
	}

	if string(encoded) != string(again) {
		t.Errorf("the XML form is not stable across a round trip")
	}

	// And back to the key-value form.
	kvn, err := second.Encode()
	if err != nil {
		t.Fatalf("Encode: %v", err)
	}
	if _, err := cdm.Decode(kvn); err != nil {
		t.Fatalf("the key-value form written from XML does not read back: %v", err)
	}
}

// The XML form writes units as an attribute where the key-value form writes
// them inside the value. A conversion in either direction has to move them,
// and losing one turns a distance in metres into a bare number.
func TestXMLUnitsBecomeAttributes(t *testing.T) {
	m, err := cdm.Decode([]byte(clause362))
	if err != nil {
		t.Fatalf("Decode: %v", err)
	}
	encoded, err := m.EncodeXML()
	if err != nil {
		t.Fatalf("EncodeXML: %v", err)
	}

	if !strings.Contains(string(encoded), `<MISS_DISTANCE units="m">715</MISS_DISTANCE>`) {
		t.Errorf("the units did not become an attribute:\n%s", encoded)
	}
	if strings.Contains(string(encoded), "[m]") {
		t.Errorf("a bracketed unit was written into the element text:\n%s", encoded)
	}

	// And back again.
	back, err := cdm.DecodeXML(encoded)
	if err != nil {
		t.Fatalf("DecodeXML: %v", err)
	}
	kvn, err := back.Encode()
	if err != nil {
		t.Fatalf("Encode: %v", err)
	}
	if !strings.Contains(string(kvn), "MISS_DISTANCE = 715 [m]") {
		t.Errorf("the units did not return to the value:\n%s", kvn)
	}
}

// The XML form nests blocks the key-value form leaves flat, so a conversion
// has to know which keyword belongs to which block.
func TestXMLNestsBlocks(t *testing.T) {
	m, err := cdm.Decode([]byte(clause362))
	if err != nil {
		t.Fatalf("Decode: %v", err)
	}
	encoded, err := m.EncodeXML()
	if err != nil {
		t.Fatalf("EncodeXML: %v", err)
	}
	source := string(encoded)

	for _, block := range []string{"<stateVector>", "<covarianceMatrix>"} {
		if !strings.Contains(source, block) {
			t.Errorf("the %s block was not written:\n%s", block, source)
		}
	}
	// This message has no OD or additional parameters, so those blocks must
	// not appear at all.
	for _, block := range []string{"<odParameters>", "<additionalParameters>"} {
		if strings.Contains(source, block) {
			t.Errorf("the empty %s block was written", block)
		}
	}
	// And a metadata keyword must not have been swept into a data block.
	if !strings.Contains(source, "<MANEUVERABLE>YES</MANEUVERABLE>") {
		t.Errorf("MANEUVERABLE is not where it should be:\n%s", source)
	}
}

func TestDecodeXMLRejects(t *testing.T) {
	valid, err := cdm.Decode([]byte(clause362))
	if err != nil {
		t.Fatalf("Decode: %v", err)
	}
	encoded, err := valid.EncodeXML()
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
			// In the key-value form OBJECT is the section boundary, so a wrong
			// value cannot survive. Here the segments separate the objects and
			// OBJECT is an ordinary element, so it can disagree with the
			// segment it sits in — which would flip which object an operator
			// thinks can manoeuvre.
			name:  "the two objects in the wrong order",
			input: strings.Replace(source, "<OBJECT>OBJECT1</OBJECT>", "<OBJECT>OBJECT2</OBJECT>", 1),
			want:  cdm.ErrObjectRepeated,
		},
		{
			name:  "an OBJECT value that is neither",
			input: strings.Replace(source, "<OBJECT>OBJECT2</OBJECT>", "<OBJECT>OBJECT3</OBJECT>", 1),
			want:  cdm.ErrObjectValue,
		},
		{
			name:  "a relative keyword inside an object segment",
			input: strings.Replace(source, "<MANEUVERABLE>YES</MANEUVERABLE>", "<MISS_DISTANCE>10</MISS_DISTANCE>", 1),
			want:  cdm.ErrUnknownKeyword,
		},
		{
			name:  "an object keyword in the relative section",
			input: strings.Replace(source, "<relativeMetadataData>", "<relativeMetadataData><X>1.0</X>", 1),
			want:  cdm.ErrObjectOutOfOrder,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := cdm.DecodeXML([]byte(tt.input)); !errors.Is(err, tt.want) {
				t.Errorf("DecodeXML = %v, want %v", err, tt.want)
			}
		})
	}
}

// A CDM in XML must have exactly two segments (clause 3.4.2 of the XML
// standard), not one and not three.
func TestDecodeXMLNeedsTwoSegments(t *testing.T) {
	const oneSegment = `<?xml version="1.0" encoding="UTF-8"?>
<cdm id="CCSDS_CDM_VERS" version="1.0">
  <header>
    <CREATION_DATE>2010-03-12T22:31:12.000</CREATION_DATE>
    <ORIGINATOR>JSPOC</ORIGINATOR>
    <MESSAGE_ID>1</MESSAGE_ID>
  </header>
  <body>
    <relativeMetadataData>
      <TCA>2010-03-13T22:37:52.618</TCA>
      <MISS_DISTANCE>715</MISS_DISTANCE>
    </relativeMetadataData>
    <segment>
      <metadata>
        <OBJECT>OBJECT1</OBJECT>
      </metadata>
    </segment>
  </body>
</cdm>
`
	if _, err := cdm.DecodeXML([]byte(oneSegment)); !errors.Is(err, cdm.ErrMissingObject) {
		t.Errorf("DecodeXML with one segment = %v, want ErrMissingObject", err)
	}
}
