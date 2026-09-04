package odm_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/ravisuhag/astro/internal/ndm"
	"github.com/ravisuhag/astro/pkg/odm"
)

// Figure G-20 of CCSDS 502.0-B-3, the OCM in XML, transcribed.
//
// The document wraps long values across lines for the page; they are joined
// here, since a line break inside an element's text is not part of the value.
// CLASSIFICATION is absent from the figure even though the key-value form of
// the same message (figure G-16) carries it, which is the document's own
// inconsistency rather than a rule about the XML form.
const ocmXML = `<?xml version="1.0" encoding="UTF-8"?>
<ocm xmlns:xsi="http://www.w3.org/2001/XMLSchema-instance"
     xsi:noNamespaceSchemaLocation="https://sanaregistry.org/r/ndmxml_unqualified/ndmxml-3.0.0-master-3.0.xsd"
     id="CCSDS_OCM_VERS" version="3.0">
  <header>
    <COMMENT>ODM V.3 Example G-2</COMMENT>
    <COMMENT>OCM example with space object characteristics and perturbations.</COMMENT>
    <CREATION_DATE>2022-11-06T09:23:57</CREATION_DATE>
    <ORIGINATOR>JAPAN AEROSPACE EXPLORATION AGENCY</ORIGINATOR>
    <MESSAGE_ID>OCM 201113719185</MESSAGE_ID>
  </header>
  <body>
    <segment>
      <metadata>
        <OBJECT_NAME>OSPREY 5</OBJECT_NAME>
        <INTERNATIONAL_DESIGNATOR>2022-999A</INTERNATIONAL_DESIGNATOR>
        <ORIGINATOR_POC>R. Rabbit</ORIGINATOR_POC>
        <ORIGINATOR_PHONE>(719)555-1234</ORIGINATOR_PHONE>
        <TECH_POC>Mr. Rodgers</TECH_POC>
        <TECH_PHONE>(719)555-1234</TECH_PHONE>
        <TECH_EMAIL>email@email.XXX</TECH_EMAIL>
        <TIME_SYSTEM>UT1</TIME_SYSTEM>
        <EPOCH_TZERO>2022-12-18T00:00:00.0000</EPOCH_TZERO>
        <TAIMUTC_AT_TZERO units="s">36</TAIMUTC_AT_TZERO>
        <UT1MUTC_AT_TZERO units="s">.357</UT1MUTC_AT_TZERO>
      </metadata>
      <data>
        <traj>
          <COMMENT>GEOCENTRIC, CARTESIAN, EARTH FIXED</COMMENT>
          <COMMENT>THIS IS MY SECOND COMMENT LINE</COMMENT>
          <TRAJ_BASIS>PREDICTED</TRAJ_BASIS>
          <CENTER_NAME>EARTH</CENTER_NAME>
          <TRAJ_REF_FRAME>EFG</TRAJ_REF_FRAME>
          <TRAJ_TYPE>CARTPVA</TRAJ_TYPE>
          <trajLine>2022-12-18T14:28:25.1172 2854.5 -2916.2 -5360.7 5.90 4.86 0.52 0.0037 -0.0038 -0.0070</trajLine>
        </traj>
        <phys>
          <COMMENT>Spacecraft Physical Characteristics</COMMENT>
          <WET_MASS units="kg">100.0</WET_MASS>
          <OEB_Q1>0.03123</OEB_Q1>
          <OEB_Q2>0.78543</OEB_Q2>
          <OEB_Q3>0.39158</OEB_Q3>
          <OEB_QC>0.47832</OEB_QC>
          <OEB_MAX units="m">2.0</OEB_MAX>
          <OEB_INT units="m">1.0</OEB_INT>
          <OEB_MIN units="m">0.5</OEB_MIN>
          <AREA_ALONG_OEB_MAX units="m**2">0.5</AREA_ALONG_OEB_MAX>
          <AREA_ALONG_OEB_INT units="m**2">1.0</AREA_ALONG_OEB_INT>
          <AREA_ALONG_OEB_MIN units="m**2">2.0</AREA_ALONG_OEB_MIN>
        </phys>
        <pert>
          <COMMENT>Perturbations Specification</COMMENT>
          <ATMOSPHERIC_MODEL>NRLMSIS00</ATMOSPHERIC_MODEL>
          <GRAVITY_MODEL>EGM-96: 36D 36O</GRAVITY_MODEL>
          <GM units="km**3/s**2">398600.4415</GM>
          <N_BODY_PERTURBATIONS>MOON, SUN</N_BODY_PERTURBATIONS>
          <FIXED_GEOMAG_KP>12.0</FIXED_GEOMAG_KP>
          <FIXED_F10P7>105.0</FIXED_F10P7>
          <FIXED_F10P7_MEAN>120.0</FIXED_F10P7_MEAN>
        </pert>
        <user>
          <USER_DEFINED parameter="CONSOLE_POC">MAXWELL RAFERTY</USER_DEFINED>
          <USER_DEFINED parameter="EARTH_MODEL">WGS-84</USER_DEFINED>
        </user>
      </data>
    </segment>
  </body>
</ocm>
`

func TestDecodeXMLOCM(t *testing.T) {
	m, err := odm.DecodeXMLOCM([]byte(ocmXML))
	if err != nil {
		t.Fatalf("DecodeXMLOCM: %v", err)
	}

	if m.Header.Version != "3.0" || m.Header.MessageID != "OCM 201113719185" {
		t.Errorf("header = %+v", m.Header)
	}
	if len(m.Header.Comments) != 2 {
		t.Errorf("header comments = %q, want 2", m.Header.Comments)
	}
	if m.ObjectName() != "OSPREY 5" || m.TimeSystem() != "UT1" {
		t.Errorf("metadata = %q / %q", m.ObjectName(), m.TimeSystem())
	}

	// Clause 8.10.10 puts units in an attribute. Reading them back onto the
	// value is what keeps the two forms saying the same thing.
	if got := m.Metadata.GetOr("TAIMUTC_AT_TZERO", ""); got != "36 [s]" {
		t.Errorf("TAIMUTC_AT_TZERO = %q, want the units back on the value", got)
	}

	if len(m.Trajectories) != 1 {
		t.Fatalf("read %d trajectory blocks, want 1", len(m.Trajectories))
	}
	traj := m.Trajectories[0]
	if traj.TrajType() != "CARTPVA" || traj.RefFrame() != "EFG" {
		t.Errorf("trajectory = %q in %q", traj.TrajType(), traj.RefFrame())
	}
	if len(traj.Comments) != 2 {
		t.Errorf("trajectory comments = %q, want 2", traj.Comments)
	}

	// Clause 8.11.15 makes a trajLine an xsd:string, so the columns are still
	// the reader's problem: ten fields, a time tag and nine numbers.
	if len(traj.Rows) != 1 {
		t.Fatalf("read %d rows, want 1", len(traj.Rows))
	}
	if len(traj.Rows[0].Fields) != 10 {
		t.Errorf("the row holds %d fields, want 10", len(traj.Rows[0].Fields))
	}
	if traj.Rows[0].IsRelative() {
		t.Error("the row was read as a relative time tag")
	}

	if m.Physical == nil || len(m.Physical.Fields) != 11 {
		t.Errorf("physical section = %+v", m.Physical)
	}
	if m.Perturbations == nil || len(m.Perturbations.Fields) != 7 {
		t.Errorf("perturbations section = %+v", m.Perturbations)
	}
	if len(m.UserDefined) != 2 || m.UserDefined[1].Name != "EARTH_MODEL" {
		t.Errorf("user-defined parameters = %+v", m.UserDefined)
	}
}

func TestOCMXMLRoundTrip(t *testing.T) {
	first, err := odm.DecodeXMLOCM([]byte(ocmXML))
	if err != nil {
		t.Fatalf("DecodeXMLOCM: %v", err)
	}
	encoded, err := first.EncodeXML()
	if err != nil {
		t.Fatalf("EncodeXML: %v", err)
	}
	second, err := odm.DecodeXMLOCM(encoded)
	if err != nil {
		t.Fatalf("DecodeXMLOCM on our own output: %v\n%s", err, encoded)
	}

	if len(second.Trajectories) != len(first.Trajectories) ||
		len(second.UserDefined) != len(first.UserDefined) {
		t.Fatal("the section shape changed on round trip")
	}
	if second.Physical == nil || len(second.Physical.Fields) != len(first.Physical.Fields) {
		t.Error("the physical section changed on round trip")
	}
	if got := second.Metadata.GetOr("TAIMUTC_AT_TZERO", ""); got != "36 [s]" {
		t.Errorf("TAIMUTC_AT_TZERO = %q after a round trip", got)
	}

	// The units must go back into the attribute rather than staying bracketed
	// in the element's text, which is the mistake the CDM's XML form made
	// first time round.
	if !strings.Contains(string(encoded), `<WET_MASS units="kg">100.0</WET_MASS>`) {
		t.Errorf("the units were not written as an attribute:\n%s", encoded)
	}
}

// A message crossing between the forms must say the same thing. The one
// difference allowed is white space inside a value, which clauses 7.4.5 to
// 7.4.7 make insignificant: '36      [s]' in KVN becomes '36 [s]' after a
// crossing, because the units are held apart from the number in XML.
func TestOCMCrossesBetweenForms(t *testing.T) {
	fromKVN, err := odm.DecodeOCM([]byte(ocmCharacteristics))
	if err != nil {
		t.Fatalf("DecodeOCM: %v", err)
	}
	asXML, err := fromKVN.EncodeXML()
	if err != nil {
		t.Fatalf("EncodeXML: %v", err)
	}
	back, err := odm.DecodeXMLOCM(asXML)
	if err != nil {
		t.Fatalf("DecodeXMLOCM: %v\n%s", err, asXML)
	}

	if back.ObjectName() != fromKVN.ObjectName() || back.TimeSystem() != fromKVN.TimeSystem() {
		t.Error("the metadata changed crossing the forms")
	}
	if len(back.Trajectories) != len(fromKVN.Trajectories) {
		t.Fatal("the trajectory count changed crossing the forms")
	}
	if strings.Join(back.Trajectories[0].Rows[0].Fields, " ") !=
		strings.Join(fromKVN.Trajectories[0].Rows[0].Fields, " ") {
		t.Error("a trajectory row changed crossing the forms")
	}
	if len(back.UserDefined) != len(fromKVN.UserDefined) {
		t.Error("the user-defined parameters changed crossing the forms")
	}

	// And back again, to the key-value form the XML came from.
	asKVN, err := back.Encode()
	if err != nil {
		t.Fatalf("Encode: %v", err)
	}
	if _, err := odm.DecodeOCM(asKVN); err != nil {
		t.Fatalf("the key-value form written from XML does not read back: %v\n%s", err, asKVN)
	}
}

func TestDecodeXMLOCMRejects(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  error
	}{
		{
			name:  "another message type",
			input: strings.NewReplacer("<ocm", "<opm", "</ocm>", "</opm>").Replace(ocmXML),
			want:  ndm.ErrWrongMessageType,
		},
		{
			name:  "a block table 8-9 does not name",
			input: strings.NewReplacer("<phys>", "<physical>", "</phys>", "</physical>").Replace(ocmXML),
			want:  ndm.ErrMalformedXML,
		},
		{
			// The block order carries what table 6-1 fixes for the delimiters.
			name: "blocks out of table 6-1's order",
			input: strings.Replace(ocmXML, "<traj>",
				"<pert><GM>398600.4415</GM></pert>\n<traj>", 1),
			want: odm.ErrSectionsOutOfOrder,
		},
		{
			name:  "two perturbations blocks",
			input: strings.Replace(ocmXML, "</pert>", "</pert><pert><GM>398600.4415</GM></pert>", 1),
			want:  odm.ErrDuplicateSection,
		},
		{
			name:  "a keyword that belongs to no table",
			input: strings.Replace(ocmXML, "<CENTER_NAME>EARTH</CENTER_NAME>", "<CENTRE_NAME>EARTH</CENTRE_NAME>", 1),
			want:  odm.ErrUnknownKeyword,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := odm.DecodeXMLOCM([]byte(tt.input)); !errors.Is(err, tt.want) {
				t.Errorf("DecodeXMLOCM = %v, want %v", err, tt.want)
			}
		})
	}
}
