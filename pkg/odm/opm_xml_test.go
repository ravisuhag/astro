package odm_test

import (
	"strings"
	"testing"

	"github.com/ravisuhag/astro/pkg/odm"
)

// Figure G-5 of CCSDS 502.0-B-3: the OPM in XML, transcribed.
const figureG5 = `<?xml version="1.0" encoding="UTF-8"?>
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

func TestDecodeXMLOPM(t *testing.T) {
	m, err := odm.DecodeXMLOPM([]byte(figureG5))
	if err != nil {
		t.Fatalf("DecodeXMLOPM: %v", err)
	}

	if m.Header.Version != "3.0" || m.Header.Originator != "JAXA" {
		t.Errorf("header = %+v", m.Header)
	}
	if m.Header.Classification != "NONE" || m.Header.MessageID != "OPM 201113719185" {
		t.Errorf("header = %+v", m.Header)
	}
	if len(m.Header.Comments) != 1 {
		t.Errorf("header comments = %q, want 1", m.Header.Comments)
	}

	md := m.Metadata
	if md.ObjectName != "OSPREY 5" || md.RefFrame != "ITRF1997" {
		t.Errorf("metadata = %+v", md)
	}
	if len(md.Comments) != 1 {
		t.Errorf("metadata comments = %q, want 1", md.Comments)
	}

	sv := m.Data.StateVector
	if sv.X != 6503.514 || sv.Z != -717.49 || sv.ZDot != -4.191076 {
		t.Errorf("state vector = %+v", sv)
	}
	if s := m.Data.Spacecraft; s == nil || !s.HasMass() || s.Mass != 3000 {
		t.Errorf("spacecraft parameters = %+v", s)
	}
	if m.Data.Keplerian != nil || m.Data.Covariance != nil {
		t.Error("blocks were read from a message that has none")
	}
}

// The two forms must agree. Both are offered by clause 1.1, and a partner may
// send either.
func TestOPMFormsAgree(t *testing.T) {
	fromKVN, err := odm.DecodeOPM([]byte(figureG2))
	if err != nil {
		t.Fatalf("DecodeOPM: %v", err)
	}

	encoded, err := fromKVN.EncodeXML()
	if err != nil {
		t.Fatalf("EncodeXML: %v", err)
	}
	fromXML, err := odm.DecodeXMLOPM(encoded)
	if err != nil {
		t.Fatalf("DecodeXMLOPM: %v\n%s", err, encoded)
	}

	if !fromXML.Data.StateVector.Epoch.Equal(fromKVN.Data.StateVector.Epoch) {
		t.Errorf("epoch differs")
	}
	if fromXML.Data.StateVector.X != fromKVN.Data.StateVector.X {
		t.Errorf("X differs: %v and %v", fromKVN.Data.StateVector.X, fromXML.Data.StateVector.X)
	}
	if (fromXML.Data.Keplerian == nil) != (fromKVN.Data.Keplerian == nil) {
		t.Fatal("the Keplerian block differs in presence")
	}
	if fromKVN.Data.Keplerian != nil {
		a, b := fromKVN.Data.Keplerian, fromXML.Data.Keplerian
		if a.SemiMajorAxis != b.SemiMajorAxis || a.Anomaly != b.Anomaly ||
			a.AnomalyIsMean != b.AnomalyIsMean {
			t.Errorf("Keplerian elements differ:\n\t%+v\n\t%+v", a, b)
		}
	}
	if len(fromXML.Data.Maneuvers) != len(fromKVN.Data.Maneuvers) {
		t.Fatalf("maneuver count differs: %d and %d",
			len(fromKVN.Data.Maneuvers), len(fromXML.Data.Maneuvers))
	}
	for i := range fromKVN.Data.Maneuvers {
		a, b := fromKVN.Data.Maneuvers[i], fromXML.Data.Maneuvers[i]
		if a.DV != b.DV || a.DeltaMass != b.DeltaMass || a.RefFrame != b.RefFrame {
			t.Errorf("maneuver %d differs:\n\t%+v\n\t%+v", i, a, b)
		}
		if !a.EpochIgnition.Equal(b.EpochIgnition) {
			t.Errorf("maneuver %d ignition differs", i)
		}
	}
}

// Each manoeuvre is its own block in XML, where the key-value form repeats the
// keywords instead (clause 8.8.14).
func TestOPMXMLRepeatsManeuverBlocks(t *testing.T) {
	m, err := odm.DecodeOPM([]byte(figureG2))
	if err != nil {
		t.Fatalf("DecodeOPM: %v", err)
	}
	encoded, err := m.EncodeXML()
	if err != nil {
		t.Fatalf("EncodeXML: %v", err)
	}

	if got := strings.Count(string(encoded), "<maneuverParameters>"); got != 2 {
		t.Errorf("wrote %d maneuver blocks, want 2:\n%s", got, encoded)
	}
	// And the units are attributes, not brackets.
	if !strings.Contains(string(encoded), `<X units="km">`) {
		t.Errorf("the units did not become an attribute:\n%s", encoded)
	}
	if strings.Contains(string(encoded), "[km]") {
		t.Errorf("a bracketed unit reached the XML:\n%s", encoded)
	}
}

func TestOPMXMLRoundTrip(t *testing.T) {
	first, err := odm.DecodeXMLOPM([]byte(figureG5))
	if err != nil {
		t.Fatalf("DecodeXMLOPM: %v", err)
	}
	encoded, err := first.EncodeXML()
	if err != nil {
		t.Fatalf("EncodeXML: %v", err)
	}
	second, err := odm.DecodeXMLOPM(encoded)
	if err != nil {
		t.Fatalf("DecodeXMLOPM on our own output: %v\n%s", err, encoded)
	}
	again, err := second.EncodeXML()
	if err != nil {
		t.Fatalf("EncodeXML on a decoded message: %v", err)
	}

	if string(encoded) != string(again) {
		t.Error("the XML form is not stable across a round trip")
	}

	// And across the forms: XML in, key-value out, and back.
	kvn, err := second.Encode()
	if err != nil {
		t.Fatalf("Encode: %v", err)
	}
	third, err := odm.DecodeOPM(kvn)
	if err != nil {
		t.Fatalf("the key-value form written from XML does not read back: %v", err)
	}
	if third.Data.StateVector.X != first.Data.StateVector.X {
		t.Errorf("X changed crossing the forms: %v then %v",
			first.Data.StateVector.X, third.Data.StateVector.X)
	}
}

// A user-defined parameter changes shape between the forms: in the key-value
// form its name is part of the keyword, and in XML it is an attribute.
func TestOPMXMLUserDefinedParameters(t *testing.T) {
	input := figureG1 + "USER_DEFINED_EARTH_MODEL = WGS-84\nUSER_DEFINED_CONSOLE_POC = M RAFERTY\n"

	m, err := odm.DecodeOPM([]byte(input))
	if err != nil {
		t.Fatalf("DecodeOPM: %v", err)
	}
	encoded, err := m.EncodeXML()
	if err != nil {
		t.Fatalf("EncodeXML: %v", err)
	}

	if !strings.Contains(string(encoded), `<USER_DEFINED parameter="EARTH_MODEL">WGS-84</USER_DEFINED>`) {
		t.Errorf("the parameter name did not become an attribute:\n%s", encoded)
	}

	back, err := odm.DecodeXMLOPM(encoded)
	if err != nil {
		t.Fatalf("DecodeXMLOPM: %v", err)
	}
	if len(back.Data.UserDefined) != 2 {
		t.Fatalf("read %d user-defined parameters, want 2", len(back.Data.UserDefined))
	}
	if back.Data.UserDefined[0].Name != "EARTH_MODEL" || back.Data.UserDefined[0].Value != "WGS-84" {
		t.Errorf("user-defined parameter = %+v", back.Data.UserDefined[0])
	}
}
