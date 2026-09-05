package ndm

import (
	"encoding/xml"
	"errors"
	"strings"
	"testing"
)

// The XML form of the OPM from figure G-5 of CCSDS 502.0-B-3, transcribed.
// It is the clearest statement of the structure clauses 3.2 to 3.4 describe,
// and of the naming rule: keywords keep their upper-case names as elements,
// and the blocks that wrap them are lower camel case.
const opmXML = `<?xml version="1.0" encoding="UTF-8"?>
<opm xmlns:xsi="http://www.w3.org/2001/XMLSchema-instance"
      xsi:noNamespaceSchemaLocation="https://sanaregistry.org/r/ndmxml_unqualified/ndmxml-3.0.0-master-3.0.xsd"
      id="CCSDS_OPM_VERS" version="3.0">
  <header>
    <COMMENT>THIS IS AN XML VERSION OF THE OPM</COMMENT>
    <CLASSIFICATION>NONE</CLASSIFICATION>
    <CREATION_DATE>2022-11-06T09:23:57</CREATION_DATE>
    <ORIGINATOR>JAXA</ORIGINATOR>
    <MESSAGE_ID>OPM 201113719185</MESSAGE_ID>
  </header>
  <body>
    <segment>
       <metadata>
         <COMMENT>GEOCENTRIC, CARTESIAN, EARTH FIXED</COMMENT>
         <OBJECT_NAME>OSPREY 5</OBJECT_NAME>
         <OBJECT_ID>2022-999A</OBJECT_ID>
         <CENTER_NAME>EARTH</CENTER_NAME>
         <REF_FRAME>ITRF1997</REF_FRAME>
         <TIME_SYSTEM>UTC</TIME_SYSTEM>
       </metadata>
       <data>
         <stateVector>
           <EPOCH>2022-12-18T14:28:15.1172</EPOCH>
           <X>6503.514000</X>
           <Y>1239.647000</Y>
           <Z>-717.490000</Z>
           <X_DOT>-0.873160</X_DOT>
           <Y_DOT>8.740420</Y_DOT>
           <Z_DOT>-4.191076</Z_DOT>
         </stateVector>
         <spacecraftParameters>
           <MASS>3000.000000</MASS>
           <SOLAR_RAD_AREA>18.770000</SOLAR_RAD_AREA>
           <SOLAR_RAD_COEFF>1.000000</SOLAR_RAD_COEFF>
           <DRAG_AREA>18.770000</DRAG_AREA>
           <DRAG_COEFF>2.500000</DRAG_COEFF>
         </spacecraftParameters>
       </data>
    </segment>
  </body>
</opm>
`

func TestDecodeXML(t *testing.T) {
	m, err := DecodeXML([]byte(opmXML), "opm")
	if err != nil {
		t.Fatalf("DecodeXML: %v", err)
	}

	if m.ID != "CCSDS_OPM_VERS" || m.Version != "3.0" {
		t.Errorf("root attributes = %q / %q", m.ID, m.Version)
	}

	if origin, ok := Find(m.Header, "ORIGINATOR"); !ok || origin != "JAXA" {
		t.Errorf("ORIGINATOR = %q, %v", origin, ok)
	}
	if comments := CollectComments(m.Header); len(comments) != 1 {
		t.Errorf("header comments = %q, want 1", comments)
	}

	if len(m.Segments) != 1 {
		t.Fatalf("read %d segments, want 1", len(m.Segments))
	}
	segment := m.Segments[0]

	if name, ok := Find(segment.Metadata, "OBJECT_NAME"); !ok || name != "OSPREY 5" {
		t.Errorf("OBJECT_NAME = %q, %v", name, ok)
	}

	// The blocks are what the key-value form leaves implicit.
	state, ok := FindBlock(segment.Data, "stateVector")
	if !ok {
		t.Fatal("the stateVector block was not found")
	}
	if x, ok := Find(state, "X"); !ok || x != "6503.514000" {
		t.Errorf("X = %q, %v", x, ok)
	}
	if _, ok := FindBlock(segment.Data, "spacecraftParameters"); !ok {
		t.Error("the spacecraftParameters block was not found")
	}
	if _, ok := FindBlock(segment.Data, "covarianceMatrix"); ok {
		t.Error("a covarianceMatrix block was found in a message that has none")
	}
}

func TestDecodeXMLRejects(t *testing.T) {
	tests := []struct {
		name  string
		input string
		root  string
		want  error
	}{
		{"empty", "", "opm", ErrMalformedXML},
		{"another message type", opmXML, "oem", ErrWrongMessageType},
		{
			name:  "no id attribute",
			input: strings.Replace(opmXML, `id="CCSDS_OPM_VERS" `, "", 1),
			root:  "opm",
			want:  ErrNoVersionLine,
		},
		{
			name:  "no version attribute",
			input: strings.Replace(opmXML, ` version="3.0"`, "", 1),
			root:  "opm",
			want:  ErrNoVersionLine,
		},
		{
			name:  "something other than a header or a body under the root",
			input: strings.NewReplacer("<body>", "<nobody>", "</body>", "</nobody>").Replace(opmXML),
			root:  "opm",
			want:  ErrUnknownHeaderKeyword,
		},
		{
			name: "a body with no segment",
			input: `<?xml version="1.0"?><opm id="CCSDS_OPM_VERS" version="3.0">` +
				`<header></header><body></body></opm>`,
			root: "opm",
			want: ErrMalformedXML,
		},
		{
			name:  "something other than metadata or data in a segment",
			input: strings.NewReplacer("<metadata>", "<middledata>", "</metadata>", "</middledata>").Replace(opmXML),
			root:  "opm",
			want:  ErrMalformedXML,
		},
		{
			name:  "mixed content, text beside elements",
			input: strings.Replace(opmXML, "<stateVector>", "<stateVector>loose text", 1),
			root:  "opm",
			want:  ErrMalformedXML,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := DecodeXML([]byte(tt.input), tt.root); !errors.Is(err, tt.want) {
				t.Errorf("DecodeXML = %v, want %v", err, tt.want)
			}
		})
	}
}

func TestXMLRoundTrip(t *testing.T) {
	first, err := DecodeXML([]byte(opmXML), "opm")
	if err != nil {
		t.Fatalf("DecodeXML: %v", err)
	}
	encoded, err := first.EncodeXML()
	if err != nil {
		t.Fatalf("EncodeXML: %v", err)
	}
	second, err := DecodeXML(encoded, "opm")
	if err != nil {
		t.Fatalf("DecodeXML on our own output: %v\n%s", err, encoded)
	}

	if second.ID != first.ID || second.Version != first.Version {
		t.Errorf("root attributes changed")
	}
	if len(second.Header) != len(first.Header) {
		t.Errorf("header element count changed: %d then %d", len(first.Header), len(second.Header))
	}
	if len(second.Segments) != len(first.Segments) {
		t.Fatalf("segment count changed: %d then %d", len(first.Segments), len(second.Segments))
	}

	a, b := first.Segments[0], second.Segments[0]
	if len(a.Metadata) != len(b.Metadata) || len(a.Data) != len(b.Data) {
		t.Errorf("segment shape changed")
	}
	for _, name := range []string{"stateVector", "spacecraftParameters"} {
		if _, ok := FindBlock(b.Data, name); !ok {
			t.Errorf("the %s block did not survive", name)
		}
	}

	// The schema a message arrived under is kept, so re-encoding does not
	// silently move it to another standard's schema.
	if second.Schema != first.Schema {
		t.Errorf("schema changed: %q then %q", first.Schema, second.Schema)
	}
	if first.Schema != XMLSchemaODM {
		t.Errorf("schema = %q, want the ODM's", first.Schema)
	}
}

// The four navigation standards name four different master schemas, and the
// numbers do not track the NDM/XML document. Substituting one for another
// produces a file that validates against the wrong schema.
func TestSchemaLocationsDifferPerStandard(t *testing.T) {
	tests := []struct {
		root string
		want string
	}{
		{"opm", XMLSchemaODM},
		{"oem", XMLSchemaODM},
		{"apm", XMLSchemaADM},
		{"aem", XMLSchemaADM},
		{"tdm", XMLSchemaTDM},
		{"cdm", XMLSchemaCDM},
	}
	for _, tt := range tests {
		if got := defaultSchema(tt.root); got != tt.want {
			t.Errorf("defaultSchema(%q) = %q, want %q", tt.root, got, tt.want)
		}
	}

	if XMLSchemaODM == XMLSchemaADM {
		t.Error("the ODM and ADM schemas are the same; the standards give 3.0 and 4.0")
	}
}

// An empty block is written as nothing. A block exists only to group the
// keywords inside it, so one with no keywords says nothing and an empty tag
// would be a claim that the block is present but empty.
func TestEmptyBlocksAreOmitted(t *testing.T) {
	m := &XMLMessage{
		Root: "opm", ID: "CCSDS_OPM_VERS", Version: "3.0", Schema: XMLSchemaODM,
		Header: []Element{Leaf("ORIGINATOR", "JAXA")},
		Segments: []Segment{{
			Metadata: []Element{Leaf("OBJECT_NAME", "A")},
			Data: []Element{
				Block("stateVector", Leaf("X", "1.0")),
				Block("covarianceMatrix"),
			},
		}},
	}

	encoded, err := m.EncodeXML()
	if err != nil {
		t.Fatalf("EncodeXML: %v", err)
	}
	if strings.Contains(string(encoded), "covarianceMatrix") {
		t.Errorf("an empty block was written:\n%s", encoded)
	}
	if !strings.Contains(string(encoded), "<stateVector>") {
		t.Errorf("a block with children was not written:\n%s", encoded)
	}
}

// Clause 4.3.3 gives the schema instance namespace "exactly as shown", and
// notes that https is wrong there because the string names a namespace rather
// than a protocol. It is the kind of thing an editor helpfully corrects.
func TestNamespaceIsHTTPNotHTTPS(t *testing.T) {
	if !strings.HasPrefix(XMLNamespaceInstance, "http://") {
		t.Errorf("XMLNamespaceInstance = %q, want the http form", XMLNamespaceInstance)
	}

	m := &XMLMessage{
		Root: "opm", ID: "CCSDS_OPM_VERS", Version: "3.0", Schema: XMLSchemaODM,
		Header:   []Element{Leaf("ORIGINATOR", "JAXA")},
		Segments: []Segment{{Metadata: []Element{Leaf("OBJECT_NAME", "A")}}},
	}
	encoded, err := m.EncodeXML()
	if err != nil {
		t.Fatalf("EncodeXML: %v", err)
	}
	if !strings.Contains(string(encoded), `xmlns:xsi="http://www.w3.org/2001/XMLSchema-instance"`) {
		t.Errorf("the schema instance namespace is not as clause 4.3.3 requires:\n%s", encoded)
	}
}

// Values that contain XML's reserved characters have to survive.
func TestXMLEscaping(t *testing.T) {
	m := &XMLMessage{
		Root: "opm", ID: "CCSDS_OPM_VERS", Version: "3.0", Schema: XMLSchemaODM,
		Header:   []Element{Leaf("ORIGINATOR", `A & B <C> "D"`)},
		Segments: []Segment{{Metadata: []Element{Leaf("OBJECT_NAME", "A")}}},
	}

	encoded, err := m.EncodeXML()
	if err != nil {
		t.Fatalf("EncodeXML: %v", err)
	}
	back, err := DecodeXML(encoded, "opm")
	if err != nil {
		t.Fatalf("DecodeXML: %v", err)
	}
	if got, _ := Find(back.Header, "ORIGINATOR"); got != `A & B <C> "D"` {
		t.Errorf("ORIGINATOR round-tripped to %q", got)
	}
}

// nestedTags returns n opened and closed <a> tags, well formed so the reader
// does not need to guess at an implied close.
func nestedTags(n int) string {
	return strings.Repeat("<a>", n) + strings.Repeat("</a>", n)
}

// readElement recurses once per level of nesting, so without a cap a file of
// repeated open tags would drive it past the goroutine stack limit. This
// checks the cap lands exactly where maxXMLDepth says it should: the element
// at maxXMLDepth is still read, and the one past it is refused.
func TestReadElementDepthLimit(t *testing.T) {
	// maxXMLDepth+1 opens put the outermost element at depth 0 and the
	// innermost at depth maxXMLDepth, which the limit allows.
	d := xml.NewDecoder(strings.NewReader(nestedTags(maxXMLDepth + 1)))
	start, err := nextStart(d)
	if err != nil {
		t.Fatalf("nextStart: %v", err)
	}
	if _, err := readElement(d, start, 0); err != nil {
		t.Errorf("at the depth limit: readElement = %v, want nil", err)
	}

	// One level deeper puts the innermost element at maxXMLDepth+1.
	d = xml.NewDecoder(strings.NewReader(nestedTags(maxXMLDepth + 2)))
	start, err = nextStart(d)
	if err != nil {
		t.Fatalf("nextStart: %v", err)
	}
	if _, err := readElement(d, start, 0); !errors.Is(err, ErrMalformedXML) {
		t.Errorf("one past the depth limit: readElement = %v, want ErrMalformedXML", err)
	}
}

// A file of far more than maxXMLDepth nested elements is what an attacker
// sends, not what a navigation message ever does; DecodeXML must refuse it
// rather than recurse until the goroutine stack runs out.
func TestDecodeXMLRejectsDeepNesting(t *testing.T) {
	input := `<?xml version="1.0"?><opm id="CCSDS_OPM_VERS" version="3.0">` +
		`<header></header><body><segment><metadata></metadata><data>` +
		nestedTags(maxXMLDepth+10) + `</data></segment></body></opm>`

	if _, err := DecodeXML([]byte(input), "opm"); !errors.Is(err, ErrMalformedXML) {
		t.Errorf("DecodeXML = %v, want an error wrapping ErrMalformedXML", err)
	}
}

// A conforming message nests about six levels deep (ndm, message, segment,
// data, block, element), far under the limit. This guards against an
// off-by-one that would reject real input.
func TestDecodeXMLStillAcceptsAConformingMessage(t *testing.T) {
	if _, err := DecodeXML([]byte(opmXML), "opm"); err != nil {
		t.Errorf("DecodeXML: %v", err)
	}
}

// The combined instantiation reads its constituents through decodeMessage and
// readChildren too, so the same depth limit has to apply there.
func TestDecodeCombinedXMLRejectsDeepNesting(t *testing.T) {
	input := `<?xml version="1.0"?>` +
		`<ndm xmlns:xsi="http://www.w3.org/2001/XMLSchema-instance"` +
		` xsi:noNamespaceSchemaLocation="https://sanaregistry.org/r/ndmxml_unqualified/ndmxml-3.0.0-master-3.0.xsd">` +
		`<opm id="CCSDS_OPM_VERS" version="3.0"><header></header><body><segment>` +
		`<metadata></metadata><data>` + nestedTags(maxXMLDepth+10) + `</data>` +
		`</segment></body></opm></ndm>`

	if _, err := DecodeCombinedXML([]byte(input)); !errors.Is(err, ErrMalformedXML) {
		t.Errorf("DecodeCombinedXML = %v, want an error wrapping ErrMalformedXML", err)
	}
}
