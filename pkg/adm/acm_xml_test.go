package adm_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/ravisuhag/astro/internal/ndm"
	"github.com/ravisuhag/astro/pkg/adm"
)

// The ACM instantiation from figure G-12 of CCSDS 504.0-B-2, lifted out of the
// combined NDM file it appears in and given the root attributes clause 7.4
// requires of a message standing on its own.
//
// The figure prints REF_FRAME_B as SC_BODY, where the key-value form of the
// same message (figure G-6) says SC_BODY_1. Both are carried as written —
// annex B3's frame names are not checked here — so the difference is left as
// the document has it.
const acmXML = `<?xml version="1.0" encoding="UTF-8"?>
<acm xmlns:xsi="http://www.w3.org/2001/XMLSchema-instance"
     xsi:noNamespaceSchemaLocation="https://sanaregistry.org/r/ndmxml_unqualified/ndmxml-4.0.0-master-4.0.xsd"
     id="CCSDS_ACM_VERS" version="2.0">
  <header>
    <CREATION_DATE>1998-11-06T09:23:57</CREATION_DATE>
    <ORIGINATOR>JAXA</ORIGINATOR>
    <MESSAGE_ID>A7015Z4</MESSAGE_ID>
  </header>
  <body>
    <segment>
      <metadata>
        <OBJECT_NAME>EUROBIRD-4A</OBJECT_NAME>
        <INTERNATIONAL_DESIGNATOR>2000-052A</INTERNATIONAL_DESIGNATOR>
        <TIME_SYSTEM>UTC</TIME_SYSTEM>
        <EPOCH_TZERO>1998-12-18T14:28:15.1172</EPOCH_TZERO>
      </metadata>
      <data>
        <att>
          <REF_FRAME_A>J2000</REF_FRAME_A>
          <REF_FRAME_B>SC_BODY</REF_FRAME_B>
          <NUMBER_STATES>4</NUMBER_STATES>
          <ATT_TYPE>QUATERNION</ATT_TYPE>
          <attLine>0.0 0.73566 -0.50547 0.41390 0.180707</attLine>
          <attLine>0.25 0.73529 -0.50531 0.41375 0.181158</attLine>
          <attLine>0.50 0.73492 -0.50515 0.41441 0.181610</attLine>
        </att>
      </data>
    </segment>
  </body>
</acm>
`

func TestDecodeXMLACM(t *testing.T) {
	m, err := adm.DecodeXMLACM([]byte(acmXML))
	if err != nil {
		t.Fatalf("DecodeXMLACM: %v", err)
	}

	if m.Header.Version != "2.0" || m.Header.MessageID != "A7015Z4" {
		t.Errorf("header = %+v", m.Header)
	}
	if m.ObjectName() != "EUROBIRD-4A" || m.TimeSystem() != "UTC" {
		t.Errorf("metadata = %q / %q", m.ObjectName(), m.TimeSystem())
	}

	if len(m.Attitudes) != 1 {
		t.Fatalf("read %d attitude blocks, want 1", len(m.Attitudes))
	}
	att := m.Attitudes[0]
	if att.AttitudeType() != "QUATERNION" {
		t.Errorf("ATT_TYPE = %q", att.AttitudeType())
	}
	// Clause 7.7.13.3 makes an attLine an xsd:string, so the columns are still
	// the reader's problem: a time tag and four quaternion components.
	if len(att.Rows) != 3 {
		t.Fatalf("read %d rows, want 3", len(att.Rows))
	}
	if len(att.Rows[0].Fields) != 5 {
		t.Errorf("a row holds %d fields, want 5", len(att.Rows[0].Fields))
	}
	if !att.Rows[0].IsRelative() {
		t.Error("the row was read as an absolute time tag")
	}
}

// Clause 7.7.13.4 says an attLine holds one component per ATT_TYPE component
// "plus one for the time tag", and that is right here — unlike the OCM, where
// clause 8.11.18 of CCSDS 502.0-B-3 double-counts the time tag because
// MAN_COMPOSITION already names it.
func TestACMLineWidthMatchesTheClause(t *testing.T) {
	m, err := adm.DecodeXMLACM([]byte(acmXML))
	if err != nil {
		t.Fatalf("DecodeXMLACM: %v", err)
	}
	att := m.Attitudes[0]
	states, ok := att.StateCount()
	if !ok {
		t.Fatal("StateCount failed on a QUATERNION block")
	}
	for i, row := range att.Rows {
		if len(row.Fields) != states+1 {
			t.Errorf("row %d holds %d fields, want %d states plus a time tag",
				i+1, len(row.Fields), states)
		}
	}
}

func TestACMXMLRoundTrip(t *testing.T) {
	first, err := adm.DecodeXMLACM([]byte(acmXML))
	if err != nil {
		t.Fatalf("DecodeXMLACM: %v", err)
	}
	encoded, err := first.EncodeXML()
	if err != nil {
		t.Fatalf("EncodeXML: %v", err)
	}
	second, err := adm.DecodeXMLACM(encoded)
	if err != nil {
		t.Fatalf("DecodeXMLACM on our own output: %v\n%s", err, encoded)
	}

	if len(second.Attitudes) != len(first.Attitudes) {
		t.Fatal("the attitude block count changed on round trip")
	}
	if len(second.Attitudes[0].Rows) != len(first.Attitudes[0].Rows) {
		t.Error("the row count changed on round trip")
	}
	// The ADM names its own master schema, which is not the ODM's.
	if !strings.Contains(string(encoded), ndm.XMLSchemaADM) {
		t.Errorf("the ADM schema location was not written:\n%s", encoded)
	}
}

// A message crossing between the forms must say the same thing. The sensor
// sub-blocks are the part worth checking: they are nested delimiters in the
// key-value form and nested elements in XML, and clause 7.7.14 shows them only
// by example.
func TestACMCrossesBetweenForms(t *testing.T) {
	fromKVN, err := adm.DecodeACM([]byte(acmManeuver))
	if err != nil {
		t.Fatalf("DecodeACM: %v", err)
	}
	asXML, err := fromKVN.EncodeXML()
	if err != nil {
		t.Fatalf("EncodeXML: %v", err)
	}
	if !strings.Contains(string(asXML), "<sensorData>") {
		t.Errorf("the sensor blocks were not written:\n%s", asXML)
	}

	back, err := adm.DecodeXMLACM(asXML)
	if err != nil {
		t.Fatalf("DecodeXMLACM: %v\n%s", err, asXML)
	}
	if back.AttitudeDetermination == nil || len(back.AttitudeDetermination.Sensors) != 4 {
		t.Fatalf("the sensor blocks did not survive: %+v", back.AttitudeDetermination)
	}
	if n, _ := back.AttitudeDetermination.Sensors[2].Get("SENSOR_NUMBER"); n != "5" {
		t.Errorf("the third sensor is number %q, want 5", n)
	}
	if strings.Join(back.Attitudes[0].Rows[0].Fields, " ") !=
		strings.Join(fromKVN.Attitudes[0].Rows[0].Fields, " ") {
		t.Error("an attitude row changed crossing the forms")
	}

	// And back again, to the key-value form the XML came from.
	asKVN, err := back.Encode()
	if err != nil {
		t.Fatalf("Encode: %v", err)
	}
	if _, err := adm.DecodeACM(asKVN); err != nil {
		t.Fatalf("the key-value form written from XML does not read back: %v\n%s", err, asKVN)
	}
}

// The units of table 7-6 are an attribute in XML and a bracketed suffix in the
// key-value form, and a message crossing between them keeps its numbers.
func TestACMXMLUnitsAreAnAttribute(t *testing.T) {
	fromKVN, err := adm.DecodeACM([]byte(acmPhysical))
	if err != nil {
		t.Fatalf("DecodeACM: %v", err)
	}
	asXML, err := fromKVN.EncodeXML()
	if err != nil {
		t.Fatalf("EncodeXML: %v", err)
	}
	if !strings.Contains(string(asXML), `<WET_MASS units="kg">1916</WET_MASS>`) {
		t.Errorf("the units were not written as an attribute:\n%s", asXML)
	}

	back, err := adm.DecodeXMLACM(asXML)
	if err != nil {
		t.Fatalf("DecodeXMLACM: %v", err)
	}
	// A three-component vector keeps its components and its units.
	if got := back.Physical.GetOr("CP", ""); got != "0.04 -0.78 -0.023 [m]" {
		t.Errorf("CP = %q after a crossing", got)
	}
}

func TestDecodeXMLACMRejects(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  error
	}{
		{
			name:  "another message type",
			input: strings.NewReplacer("<acm", "<apm", "</acm>", "</apm>").Replace(acmXML),
			want:  ndm.ErrWrongMessageType,
		},
		{
			name:  "a block table 7-7 does not name",
			input: strings.NewReplacer("<att>", "<attitude>", "</att>", "</attitude>").Replace(acmXML),
			want:  ndm.ErrMalformedXML,
		},
		{
			name: "blocks out of table 5-1's order",
			input: strings.Replace(acmXML, "<att>",
				"<phys><WET_MASS>100.0</WET_MASS></phys>\n<att>", 1),
			want: adm.ErrSectionsOutOfOrder,
		},
		{
			name:  "a keyword that belongs to no table",
			input: strings.Replace(acmXML, "<REF_FRAME_A>J2000</REF_FRAME_A>", "<REF_FRAME_C>J2000</REF_FRAME_C>", 1),
			want:  adm.ErrUnknownKeyword,
		},
		{
			// Clause 5.3.9.6 puts a sensor block inside AD_START and AD_STOP
			// and nowhere else, so one in an attitude block is refused. Found
			// by FuzzDecodeXMLACM: the key-value form of such a message is
			// one this package will not read back.
			name:  "a sensorData element outside the attitude determination block",
			input: strings.Replace(acmXML, "</att>", "<sensorData><SENSOR_NUMBER>1</SENSOR_NUMBER></sensorData></att>", 1),
			want:  adm.ErrUnexpectedDelimiter,
		},
		{
			name:  "an attLine that does not match NUMBER_STATES",
			input: strings.Replace(acmXML, "<NUMBER_STATES>4</NUMBER_STATES>", "<NUMBER_STATES>9</NUMBER_STATES>", 1),
			want:  adm.ErrStateCountMismatch,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := adm.DecodeXMLACM([]byte(tt.input)); !errors.Is(err, tt.want) {
				t.Errorf("DecodeXMLACM = %v, want %v", err, tt.want)
			}
		})
	}
}
