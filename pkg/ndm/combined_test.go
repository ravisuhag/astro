package ndm_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/ravisuhag/astro/pkg/adm"
	"github.com/ravisuhag/astro/pkg/ndm"
	"github.com/ravisuhag/astro/pkg/odm"
)

// Figure G-21 of CCSDS 502.0-B-3, the published combined instantiation,
// transcribed with its first constituent complete.
//
// The document wraps the schema location across two lines for the page;
// clause 4.3.6 requires it to be a single string of non-blank characters with
// no line breaks, so it is joined here. The second <omm> the figure begins is
// left out — it is cut off by the page break, and one complete constituent is
// what the structure needs to show.
//
// What matters here is the attributes. The root carries the namespace and the
// schema location and neither 'id' nor 'version' (clause 4.11.4); the <omm>
// carries 'id' and 'version' and nothing else (clause 4.11.5).
const figureG21 = `<?xml version="1.0" encoding="UTF-8"?>
<ndm xmlns:xsi="http://www.w3.org/2001/XMLSchema-instance"
xsi:noNamespaceSchemaLocation="https://sanaregistry.org/r/ndmxml_unqualified/ndmxml-3.0.0-master-3.0.xsd">
  <omm id="CCSDS_OMM_VERS" version="3.0">
    <header>
      <COMMENT>GENERATED VIA SPACE-TRACK.ORG API</COMMENT>
      <CREATION_DATE>2020-05-16T14:00:01</CREATION_DATE>
      <ORIGINATOR>18 SPCS</ORIGINATOR>
    </header>
    <body>
      <segment>
         <metadata>
           <OBJECT_NAME>STARLINK-1073</OBJECT_NAME>
           <OBJECT_ID>2020-001A</OBJECT_ID>
           <CENTER_NAME>EARTH</CENTER_NAME>
           <REF_FRAME>TEME</REF_FRAME>
           <TIME_SYSTEM>UTC</TIME_SYSTEM>
           <MEAN_ELEMENT_THEORY>SGP4</MEAN_ELEMENT_THEORY>
         </metadata>
         <data>
           <meanElements>
             <EPOCH>2020-05-16T14:00:01</EPOCH>
             <MEAN_MOTION>15.05566242</MEAN_MOTION>
             <ECCENTRICITY>0.0001225</ECCENTRICITY>
             <INCLINATION>52.9981</INCLINATION>
             <RA_OF_ASC_NODE>157.6133</RA_OF_ASC_NODE>
             <ARG_OF_PERICENTER>93.35</ARG_OF_PERICENTER>
             <MEAN_ANOMALY>295.8599</MEAN_ANOMALY>
           </meanElements>
           <tleParameters>
             <EPHEMERIS_TYPE>0</EPHEMERIS_TYPE>
             <CLASSIFICATION_TYPE>U</CLASSIFICATION_TYPE>
             <NORAD_CAT_ID>44914</NORAD_CAT_ID>
             <ELEMENT_SET_NO>999</ELEMENT_SET_NO>
             <REV_AT_EPOCH>176</REV_AT_EPOCH>
             <BSTAR>0.00057678</BSTAR>
             <MEAN_MOTION_DOT>0.00008131</MEAN_MOTION_DOT>
             <MEAN_MOTION_DDOT>0</MEAN_MOTION_DDOT>
           </tleParameters>
           <userDefinedParameters>
             <USER_DEFINED parameter="TLE_LINE0">0 STARLINK-1073</USER_DEFINED>
           </userDefinedParameters>
         </data>
      </segment>
    </body>
  </omm>
</ndm>
`

func TestDecodeCombined(t *testing.T) {
	c, err := ndm.DecodeCombined([]byte(figureG21))
	if err != nil {
		t.Fatalf("DecodeCombined: %v", err)
	}

	if len(c.Messages) != 1 {
		t.Fatalf("read %d messages, want 1", len(c.Messages))
	}
	if got := ndm.Kind(c.Messages[0]); got != "omm" {
		t.Errorf("Kind = %q, want omm", got)
	}

	// The constituent is a real OMM, read by pkg/odm's own decoder.
	message, ok := c.Messages[0].(*odm.OMM)
	if !ok {
		t.Fatalf("the constituent is a %T", c.Messages[0])
	}
	if message.Metadata.ObjectName != "STARLINK-1073" {
		t.Errorf("OBJECT_NAME = %q", message.Metadata.ObjectName)
	}
	if !message.Metadata.IsTLEBased() {
		t.Error("the message should be TLE-based, since MEAN_ELEMENT_THEORY is SGP4")
	}
	if message.Data.TLE == nil || message.Data.TLE.NoradCatID != 44914 {
		t.Errorf("TLE block = %+v", message.Data.TLE)
	}

	// The schema location is the root's, which is the only place clause 4.11.5
	// allows one.
	if c.Schema != "ndmxml-3.0.0-master-3.0.xsd" {
		t.Errorf("Schema = %q", c.Schema)
	}
}

// IsCombined tells a combined instantiation from a single message by the root
// element alone, which is what a caller handed a navigation file needs before
// it can choose a decoder.
func TestIsCombined(t *testing.T) {
	if !ndm.IsCombined([]byte(figureG21)) {
		t.Error("the published combined instantiation was not recognised")
	}

	single, err := singleOPM().EncodeXML()
	if err != nil {
		t.Fatalf("EncodeXML: %v", err)
	}
	if ndm.IsCombined(single) {
		t.Error("a single OPM was taken for a combined instantiation")
	}
	if ndm.IsCombined([]byte("not xml at all")) {
		t.Error("unreadable input was taken for a combined instantiation")
	}
}

// Clause 4.11.5 allows a constituent message tag 'id' and 'version' and
// nothing else, and clause 4.11.4 denies the root both. Getting either wrong
// produces a file that is not a combined instantiation, so this checks the
// octets rather than a round trip.
func TestCombinedAttributePlacement(t *testing.T) {
	c := &ndm.Combined{Messages: []ndm.Message{singleOPM()}}

	encoded, err := c.Encode()
	if err != nil {
		t.Fatalf("Encode: %v", err)
	}
	text := string(encoded)

	// Past the XML declaration, which has a version attribute of its own.
	_, afterDeclaration, found := strings.Cut(text, "?>\n")
	if !found {
		t.Fatalf("no XML declaration:\n%s", text)
	}
	root, _, found := strings.Cut(afterDeclaration, ">")
	if !found {
		t.Fatalf("no root element:\n%s", text)
	}
	if !strings.Contains(root, `xmlns:xsi="http://www.w3.org/2001/XMLSchema-instance"`) {
		t.Errorf("the root has no schema instance namespace:\n%s", root)
	}
	if !strings.Contains(root, "noNamespaceSchemaLocation=") {
		t.Errorf("the root has no schema location:\n%s", root)
	}
	if strings.Contains(root, "id=") || strings.Contains(root, "version=") {
		t.Errorf("clause 4.11.4 denies the root an id and a version:\n%s", root)
	}

	// The constituent carries the other two and neither of the root's.
	if !strings.Contains(text, `<opm id="CCSDS_OPM_VERS" version="3.0">`) {
		t.Errorf("the constituent tag is not as clause 4.11.5 requires:\n%s", text)
	}
	if strings.Count(text, "xmlns:xsi") != 1 {
		t.Errorf("the namespace was written more than once:\n%s", text)
	}
	if strings.Count(text, "noNamespaceSchemaLocation") != 1 {
		t.Errorf("the schema location was written more than once:\n%s", text)
	}
	// One document, one XML declaration. Joining whole files would leave one
	// per message.
	if strings.Count(text, "<?xml") != 1 {
		t.Errorf("there is not exactly one XML declaration:\n%s", text)
	}
}

// Clause 4.11.7 allows any combination of constituent message types, so a file
// may mix the standards. This is the case a per-standard reader cannot handle.
func TestCombinedMixesStandards(t *testing.T) {
	c := &ndm.Combined{
		Comments: []string{"An orbit and the attitude that depends on it"},
		Messages: []ndm.Message{singleOPM(), singleAPM()},
	}

	encoded, err := c.Encode()
	if err != nil {
		t.Fatalf("Encode: %v", err)
	}
	back, err := ndm.DecodeCombined(encoded)
	if err != nil {
		t.Fatalf("DecodeCombined: %v\n%s", err, encoded)
	}

	if len(back.Messages) != 2 {
		t.Fatalf("read %d messages, want 2", len(back.Messages))
	}
	if ndm.Kind(back.Messages[0]) != "opm" || ndm.Kind(back.Messages[1]) != "apm" {
		t.Errorf("kinds = %q, %q", ndm.Kind(back.Messages[0]), ndm.Kind(back.Messages[1]))
	}
	if len(back.Comments) != 1 {
		t.Errorf("comments = %q, want 1", back.Comments)
	}

	// The order is the file's, and clause 4.11.6 gives it no other meaning.
	if _, ok := back.Messages[0].(*odm.OPM); !ok {
		t.Errorf("the first message is a %T", back.Messages[0])
	}
	if _, ok := back.Messages[1].(*adm.APM); !ok {
		t.Errorf("the second message is a %T", back.Messages[1])
	}

	// A file of mixed standards can name only one master schema, and the first
	// message's is what an unset one falls back to.
	if back.Schema != "ndmxml-3.0.0-master-3.0.xsd" {
		t.Errorf("Schema = %q, want the ODM's, from the first message", back.Schema)
	}
}

func TestCombinedRoundTrip(t *testing.T) {
	first, err := ndm.DecodeCombined([]byte(figureG21))
	if err != nil {
		t.Fatalf("DecodeCombined: %v", err)
	}
	encoded, err := first.Encode()
	if err != nil {
		t.Fatalf("Encode: %v", err)
	}
	second, err := ndm.DecodeCombined(encoded)
	if err != nil {
		t.Fatalf("DecodeCombined on our own output: %v\n%s", err, encoded)
	}

	if len(second.Messages) != len(first.Messages) {
		t.Fatalf("message count changed: %d then %d", len(first.Messages), len(second.Messages))
	}
	if second.Schema != first.Schema {
		t.Errorf("schema changed: %q then %q", first.Schema, second.Schema)
	}

	again, err := second.Encode()
	if err != nil {
		t.Fatalf("Encode: %v", err)
	}
	if string(again) != string(encoded) {
		t.Errorf("the second encoding differs:\n%s\n---\n%s", encoded, again)
	}
}

// A constituent is held to exactly the rules it would face on its own. There
// is one decoder per message type, not one for single files and another for
// combined ones.
func TestConstituentIsFullyValidated(t *testing.T) {
	// Clause 4.2.4.9 of CCSDS 502.0-B-3 allows TEME only for a TLE-based OMM.
	// The published file is TLE-based; changing the theory alone makes it a
	// message pkg/odm refuses, and it must be refused here too.
	broken := strings.Replace(figureG21,
		"<MEAN_ELEMENT_THEORY>SGP4</MEAN_ELEMENT_THEORY>",
		"<MEAN_ELEMENT_THEORY>DSST</MEAN_ELEMENT_THEORY>", 1)

	if _, err := ndm.DecodeCombined([]byte(broken)); !errors.Is(err, odm.ErrTEMEWithoutTLE) {
		t.Errorf("DecodeCombined = %v, want %v", err, odm.ErrTEMEWithoutTLE)
	}
}

func TestDecodeCombinedRejects(t *testing.T) {
	tests := []struct {
		name  string
		input string
	}{
		{
			name:  "a single message rather than a combined instantiation",
			input: strings.NewReplacer("<ndm", "<opm", "</ndm>", "</opm>").Replace(figureG21),
		},
		{
			name:  "empty",
			input: "",
		},
		{
			name:  "a root that never closes",
			input: strings.Replace(figureG21, "</ndm>", "", 1),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := ndm.DecodeCombined([]byte(tt.input)); err == nil {
				t.Error("DecodeCombined accepted it")
			}
		})
	}
}

// Clause 4.11.4 says neither 'id' nor 'version' is associated with the <ndm>
// tag. A file carrying one on the root was written against the single-message
// rules, so nothing about its constituents can be trusted to follow 4.11.5.
func TestCombinedRootRejectsIDAndVersion(t *testing.T) {
	for _, attr := range []string{` id="CCSDS_NDM_VERS"`, ` version="3.0"`} {
		withAttr := strings.Replace(figureG21, "-master-3.0.xsd\">", "-master-3.0.xsd\""+attr+">", 1)
		if _, err := ndm.DecodeCombined([]byte(withAttr)); err == nil {
			t.Errorf("a root carrying%s was accepted", attr)
		}
	}
}

// Clause 4.11.6 draws the constituents from table 3-1, which lists the
// Re-entry Data Message of CCSDS 508.1 as well. That standard has no package
// here, so such a file is refused outright rather than half-read.
func TestUnknownConstituentIsRefused(t *testing.T) {
	withRDM := strings.NewReplacer("<omm ", "<rdm ", "</omm>", "</rdm>",
		"CCSDS_OMM_VERS", "CCSDS_RDM_VERS").Replace(figureG21)

	if _, err := ndm.DecodeCombined([]byte(withRDM)); !errors.Is(err, ndm.ErrUnknownMessageType) {
		t.Errorf("DecodeCombined = %v, want %v", err, ndm.ErrUnknownMessageType)
	}
}

// Clause 4.11.8 asks for at least one constituent as a 'should' rather than a
// 'shall', so an empty file is odd but not malformed. Refusing it would refuse
// a file the standard permits.
func TestEmptyCombinedIsAllowed(t *testing.T) {
	const empty = `<?xml version="1.0" encoding="UTF-8"?>
<ndm xmlns:xsi="http://www.w3.org/2001/XMLSchema-instance"
     xsi:noNamespaceSchemaLocation="https://sanaregistry.org/r/ndmxml_unqualified/ndmxml-3.0.0-master-3.0.xsd">
  <COMMENT>Nothing to report</COMMENT>
</ndm>
`
	c, err := ndm.DecodeCombined([]byte(empty))
	if err != nil {
		t.Fatalf("DecodeCombined: %v", err)
	}
	if len(c.Messages) != 0 {
		t.Errorf("read %d messages, want none", len(c.Messages))
	}
	if len(c.Comments) != 1 {
		t.Errorf("comments = %q, want 1", c.Comments)
	}

	// Encoding it back needs a schema, and the file gave one.
	if _, err := c.Encode(); err != nil {
		t.Errorf("Encode: %v", err)
	}
}

func TestCombinedHumanize(t *testing.T) {
	c := &ndm.Combined{
		Comments: []string{"Two standards in one file"},
		Messages: []ndm.Message{singleOPM(), singleAPM()},
	}

	text := c.Humanize()
	for _, want := range []string{
		"2 message(s)",
		"Two standards in one file",
		"[1] OPM",
		"[2] APM",
		"Orbit Parameter Message",
		"Attitude Parameter Message",
	} {
		if !strings.Contains(text, want) {
			t.Errorf("Humanize is missing %q:\n%s", want, text)
		}
	}
}

// singleOPM builds a message from the key-value form, so this package's tests
// do not re-transcribe examples another package already checks.
func singleOPM() *odm.OPM {
	const text = `CCSDS_OPM_VERS = 3.0
CREATION_DATE = 2022-11-06T09:23:57
ORIGINATOR = JAXA
OBJECT_NAME = OSPREY 5
OBJECT_ID = 1998-999A
CENTER_NAME = EARTH
REF_FRAME = ITRF2000
TIME_SYSTEM = UTC
EPOCH = 2022-12-18T14:28:15.1172
X = 6503.514000
Y = 1239.647000
Z = -717.490000
X_DOT = -0.873160
Y_DOT = 8.740420
Z_DOT = -4.191076
`
	m, err := odm.DecodeOPM([]byte(text))
	if err != nil {
		panic(err)
	}
	return m
}

// singleAPM does the same for an attitude message.
func singleAPM() *adm.APM {
	const text = `CCSDS_APM_VERS = 2.0
CREATION_DATE = 2007-11-10T15:23:57
ORIGINATOR = CNES
OBJECT_NAME = EUTELSAT W4
OBJECT_ID = 2000-028A
TIME_SYSTEM = UTC
EPOCH = 2006-06-03T00:00:00
QUAT_START
REF_FRAME_A = EME2000
REF_FRAME_B = SC_BODY_1
Q1 = 0.00005
Q2 = 0.87543
Q3 = 0.40949
QC = 0.25752
QUAT_STOP
`
	m, err := adm.DecodeAPM([]byte(text))
	if err != nil {
		panic(err)
	}
	return m
}
