package adm_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/ravisuhag/astro/pkg/adm"
)

// Both forms of the same message must give the same values.

func TestAPMFormsAgree(t *testing.T) {
	for name, input := range map[string]string{"figure G-1": figureG1, "figure G-3": figureG3} {
		t.Run(name, func(t *testing.T) {
			fromKVN, err := adm.DecodeAPM([]byte(input))
			if err != nil {
				t.Fatalf("DecodeAPM: %v", err)
			}
			encoded, err := fromKVN.EncodeXML()
			if err != nil {
				t.Fatalf("EncodeXML: %v", err)
			}
			fromXML, err := adm.DecodeXMLAPM(encoded)
			if err != nil {
				t.Fatalf("DecodeXMLAPM: %v\n%s", err, encoded)
			}

			if !fromXML.Epoch.Equal(fromKVN.Epoch) {
				t.Errorf("epoch differs")
			}
			if (fromXML.Quaternion == nil) != (fromKVN.Quaternion == nil) ||
				(fromXML.Euler == nil) != (fromKVN.Euler == nil) ||
				(fromXML.Inertia == nil) != (fromKVN.Inertia == nil) {
				t.Fatal("a block differs in presence")
			}
			if q := fromKVN.Quaternion; q != nil {
				if fromXML.Quaternion.Quaternion != q.Quaternion {
					t.Errorf("quaternion differs:\n\t%+v\n\t%+v", q.Quaternion, fromXML.Quaternion.Quaternion)
				}
				if fromXML.Quaternion.FrameA != q.FrameA || fromXML.Quaternion.FrameB != q.FrameB {
					t.Errorf("frames differ")
				}
			}
			if e := fromKVN.Euler; e != nil {
				if fromXML.Euler.EulerAngles != e.EulerAngles {
					t.Errorf("Euler angles differ:\n\t%+v\n\t%+v", e.EulerAngles, fromXML.Euler.EulerAngles)
				}
			}
			if in := fromKVN.Inertia; in != nil && fromXML.Inertia.Inertia != in.Inertia {
				t.Errorf("inertia differs")
			}
			if len(fromXML.Maneuvers) != len(fromKVN.Maneuvers) {
				t.Fatalf("maneuver count differs")
			}
			for i := range fromKVN.Maneuvers {
				a, b := fromKVN.Maneuvers[i], fromXML.Maneuvers[i]
				if a.Duration != b.Duration || a.TorqueX != b.TorqueX || a.RefFrame != b.RefFrame {
					t.Errorf("maneuver %d differs:\n\t%+v\n\t%+v", i, a, b)
				}
			}
		})
	}
}

// Clause 7.5.11 puts the four quaternion components in their own element
// inside the state, with the derivatives as a sibling rather than as four more
// components. The key-value form has no such nesting.
func TestAPMXMLNestsTheQuaternion(t *testing.T) {
	m, err := adm.DecodeAPM([]byte(figureG1))
	if err != nil {
		t.Fatalf("DecodeAPM: %v", err)
	}
	encoded, err := m.EncodeXML()
	if err != nil {
		t.Fatalf("EncodeXML: %v", err)
	}
	source := string(encoded)

	for _, want := range []string{"<quaternionState>", "<quaternion>", "<Q1>", "<QC>"} {
		if !strings.Contains(source, want) {
			t.Errorf("%s is missing:\n%s", want, source)
		}
	}
	// This message has no derivatives, so that sibling must not appear.
	if strings.Contains(source, "<quaternionDot>") {
		t.Errorf("an empty quaternionDot was written")
	}
	// And the ADM names its own schema, not the ODM's.
	if !strings.Contains(source, "ndmxml-4.0.0-master-4.0.xsd") {
		t.Errorf("the ADM schema location is wrong:\n%s", source)
	}
}

// The AEM's XML gives each attitude type its own inner element (table 7-5), so
// the choice the key-value form expresses as a line width becomes a choice of
// element name.
func TestAEMFormsAgree(t *testing.T) {
	fromKVN, err := adm.DecodeAEM([]byte(figureG4))
	if err != nil {
		t.Fatalf("DecodeAEM: %v", err)
	}
	encoded, err := fromKVN.EncodeXML()
	if err != nil {
		t.Fatalf("EncodeXML: %v", err)
	}
	fromXML, err := adm.DecodeXMLAEM(encoded)
	if err != nil {
		t.Fatalf("DecodeXMLAEM: %v\n%s", err, encoded)
	}

	if len(fromXML.Blocks) != len(fromKVN.Blocks) || fromXML.Records() != fromKVN.Records() {
		t.Fatalf("shape differs: %d/%d and %d/%d",
			len(fromKVN.Blocks), fromKVN.Records(), len(fromXML.Blocks), fromXML.Records())
	}
	for i := range fromKVN.Blocks {
		a, b := fromKVN.Blocks[i], fromXML.Blocks[i]
		if a.Metadata.Type != b.Metadata.Type {
			t.Errorf("block %d attitude type differs: %q and %q", i, a.Metadata.Type, b.Metadata.Type)
		}
		if a.Metadata.FrameA != b.Metadata.FrameA || a.Metadata.FrameB != b.Metadata.FrameB {
			t.Errorf("block %d frames differ", i)
		}
		for j := range a.Lines {
			if !a.Lines[j].Epoch.Equal(b.Lines[j].Epoch) {
				t.Errorf("block %d line %d epoch differs", i, j)
			}
			for k := range a.Lines[j].Values {
				if a.Lines[j].Values[k] != b.Lines[j].Values[k] {
					t.Errorf("block %d line %d value %d differs: %v and %v",
						i, j, k, a.Lines[j].Values[k], b.Lines[j].Values[k])
				}
			}
		}
	}
}

// Every one of the nine attitude types must survive both forms, since each has
// its own element name and its own value list.
func TestAEMXMLEveryAttitudeType(t *testing.T) {
	tests := []struct {
		attitudeType adm.AttitudeType
		element      string
	}{
		{adm.Quaternion4, "quaternionEphemeris"},
		{adm.QuaternionDerivative, "quaternionDerivative"},
		{adm.QuaternionAngVel, "quaternionAngVel"},
		{adm.EulerAngle, "eulerAngle"},
		{adm.EulerAngleDerivative, "eulerAngleDerivative"},
		{adm.EulerAngleAngVel, "eulerAngleAngVel"},
		{adm.SpinType, "spin"},
		{adm.SpinNutation, "spinNutation"},
		{adm.SpinNutationMomentum, "spinNutationMom"},
	}

	for _, tt := range tests {
		t.Run(string(tt.attitudeType), func(t *testing.T) {
			width, _ := tt.attitudeType.Fields()
			values := make([]string, width)
			for i := range values {
				values[i] = "0.5"
			}
			rotSeq := ""
			if tt.attitudeType.IsEuler() {
				rotSeq = "EULER_ROT_SEQ = ZXZ\n"
			}
			input := "CCSDS_AEM_VERS = 2.0\nCREATION_DATE = 2002-11-04T17:22:31\nORIGINATOR = X\n" +
				"META_START\nOBJECT_NAME = A\nOBJECT_ID = 1996-062A\n" +
				"REF_FRAME_A = EME2000\nREF_FRAME_B = SC_BODY_1\nTIME_SYSTEM = UTC\n" +
				"START_TIME = 1996-11-28T21:29:07\nSTOP_TIME = 1996-11-30T01:28:02\n" +
				"ATTITUDE_TYPE = " + string(tt.attitudeType) + "\n" + rotSeq +
				"META_STOP\nDATA_START\n" +
				"1996-11-28T21:29:07 " + strings.Join(values, " ") + "\n" +
				"DATA_STOP\n"

			fromKVN, err := adm.DecodeAEM([]byte(input))
			if err != nil {
				t.Fatalf("DecodeAEM: %v", err)
			}
			encoded, err := fromKVN.EncodeXML()
			if err != nil {
				t.Fatalf("EncodeXML: %v", err)
			}
			if !strings.Contains(string(encoded), "<"+tt.element+">") {
				t.Errorf("the %s element was not written:\n%s", tt.element, encoded)
			}

			fromXML, err := adm.DecodeXMLAEM(encoded)
			if err != nil {
				t.Fatalf("DecodeXMLAEM: %v\n%s", err, encoded)
			}
			if fromXML.Blocks[0].Metadata.Type != tt.attitudeType {
				t.Errorf("attitude type = %q, want %q", fromXML.Blocks[0].Metadata.Type, tt.attitudeType)
			}
			if got := len(fromXML.Blocks[0].Lines[0].Values); got != width {
				t.Errorf("read %d values, want %d", got, width)
			}
		})
	}
}

// The inner element must be the one the segment's attitude type names. A
// disagreement is the XML form of a data line of the wrong width, and it must
// be refused rather than read as something else.
func TestAEMXMLRejectsTheWrongInnerElement(t *testing.T) {
	m, err := adm.DecodeAEM([]byte(figureG4))
	if err != nil {
		t.Fatalf("DecodeAEM: %v", err)
	}
	encoded, err := m.EncodeXML()
	if err != nil {
		t.Fatalf("EncodeXML: %v", err)
	}

	wrong := strings.NewReplacer(
		"<quaternionEphemeris>", "<spinNutation>",
		"</quaternionEphemeris>", "</spinNutation>",
	).Replace(string(encoded))

	if _, err := adm.DecodeXMLAEM([]byte(wrong)); !errors.Is(err, adm.ErrAttitudeLineFields) {
		t.Errorf("DecodeXMLAEM = %v, want ErrAttitudeLineFields", err)
	}
}
