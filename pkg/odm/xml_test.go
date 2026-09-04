package odm_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/ravisuhag/astro/pkg/odm"
)

// Both forms of the same message must give the same values, in both
// directions. Clause 1.1 leaves the choice to the two exchanging parties, so a
// reader does not get to assume which arrives.

func TestOMMFormsAgree(t *testing.T) {
	fromKVN, err := odm.DecodeOMM([]byte(figureG7))
	if err != nil {
		t.Fatalf("DecodeOMM: %v", err)
	}
	encoded, err := fromKVN.EncodeXML()
	if err != nil {
		t.Fatalf("EncodeXML: %v", err)
	}
	fromXML, err := odm.DecodeXMLOMM(encoded)
	if err != nil {
		t.Fatalf("DecodeXMLOMM: %v\n%s", err, encoded)
	}

	a, b := fromKVN.Data.Elements, fromXML.Data.Elements
	if !a.Epoch.Equal(b.Epoch) {
		t.Errorf("epoch differs")
	}
	// The paired keywords are the point: which one arrived changes what the
	// number means, and it has to survive the change of form.
	if a.UsesMeanMotion != b.UsesMeanMotion || a.MeanMotion != b.MeanMotion {
		t.Errorf("orbit size differs:\n\t%+v\n\t%+v", a, b)
	}
	if a.Eccentricity != b.Eccentricity || a.MeanAnomaly != b.MeanAnomaly {
		t.Errorf("elements differ")
	}

	x, y := fromKVN.Data.TLE, fromXML.Data.TLE
	if x == nil || y == nil {
		t.Fatal("the TLE block differs in presence")
	}
	if x.UsesBTerm != y.UsesBTerm || x.UsesAgom != y.UsesAgom {
		t.Errorf("the drag keyword choice did not survive: %+v and %+v", x, y)
	}
	if x.BStar != y.BStar || x.NoradCatID != y.NoradCatID || x.ElementSetNo != y.ElementSetNo {
		t.Errorf("TLE parameters differ:\n\t%+v\n\t%+v", x, y)
	}
	if fromXML.Metadata.MeanElementTheory != fromKVN.Metadata.MeanElementTheory {
		t.Errorf("mean element theory differs")
	}
}

// An SGP4-XP message uses the other half of two paired slots, and XML has to
// carry that choice as faithfully as the key-value form does.
func TestOMMXMLKeepsTheAlternativeKeywords(t *testing.T) {
	input := strings.NewReplacer(
		"MEAN_ELEMENT_THEORY = SGP/SGP4", "MEAN_ELEMENT_THEORY = SGP4-XP",
		"BSTAR             = 0.0001", "BTERM             = 0.0015",
		"MEAN_MOTION_DDOT = 0.0", "AGOM = 0.001",
	).Replace(figureG7)

	m, err := odm.DecodeOMM([]byte(input))
	if err != nil {
		t.Fatalf("DecodeOMM: %v", err)
	}
	encoded, err := m.EncodeXML()
	if err != nil {
		t.Fatalf("EncodeXML: %v", err)
	}

	if !strings.Contains(string(encoded), "<BTERM") || !strings.Contains(string(encoded), "<AGOM") {
		t.Errorf("the SGP4-XP keywords were not written:\n%s", encoded)
	}
	if strings.Contains(string(encoded), "<BSTAR") || strings.Contains(string(encoded), "<MEAN_MOTION_DDOT") {
		t.Errorf("the SGP4 keywords were written instead:\n%s", encoded)
	}

	back, err := odm.DecodeXMLOMM(encoded)
	if err != nil {
		t.Fatalf("DecodeXMLOMM: %v", err)
	}
	if !back.Data.TLE.UsesBTerm || !back.Data.TLE.UsesAgom {
		t.Errorf("the choice did not survive the round trip: %+v", back.Data.TLE)
	}
}

// Giving both halves of a pair is refused in XML as it is in the key-value
// form: a message carrying both has not said which the receiver should believe.
func TestOMMXMLRejectsBothOfAPair(t *testing.T) {
	m, err := odm.DecodeOMM([]byte(figureG7))
	if err != nil {
		t.Fatalf("DecodeOMM: %v", err)
	}
	encoded, err := m.EncodeXML()
	if err != nil {
		t.Fatalf("EncodeXML: %v", err)
	}

	both := strings.Replace(string(encoded),
		"<MEAN_MOTION units=\"rev/day\">1.00273272</MEAN_MOTION>",
		"<MEAN_MOTION units=\"rev/day\">1.00273272</MEAN_MOTION><SEMI_MAJOR_AXIS units=\"km\">42165.0</SEMI_MAJOR_AXIS>", 1)
	if both == string(encoded) {
		t.Fatal("the test input did not change")
	}
	if _, err := odm.DecodeXMLOMM([]byte(both)); !errors.Is(err, odm.ErrBothSizeKeywords) {
		t.Errorf("DecodeXMLOMM = %v, want ErrBothSizeKeywords", err)
	}
}

// The OEM is where the two forms diverge most: positional lines become named
// elements, one block per record, and the covariance loses its delimiters.
func TestOEMFormsAgree(t *testing.T) {
	for name, input := range map[string]string{
		"figure G-11": figureG11,
		"figure G-12": figureG12,
		"figure G-13": figureG13Covariance,
	} {
		t.Run(name, func(t *testing.T) {
			fromKVN, err := odm.DecodeOEM([]byte(input))
			if err != nil {
				t.Fatalf("DecodeOEM: %v", err)
			}
			encoded, err := fromKVN.EncodeXML()
			if err != nil {
				t.Fatalf("EncodeXML: %v", err)
			}
			fromXML, err := odm.DecodeXMLOEM(encoded)
			if err != nil {
				t.Fatalf("DecodeXMLOEM: %v\n%s", err, encoded)
			}

			if len(fromXML.Blocks) != len(fromKVN.Blocks) {
				t.Fatalf("block count differs: %d and %d", len(fromKVN.Blocks), len(fromXML.Blocks))
			}
			if fromXML.Records() != fromKVN.Records() {
				t.Errorf("record count differs: %d and %d", fromKVN.Records(), fromXML.Records())
			}

			for i := range fromKVN.Blocks {
				a, b := fromKVN.Blocks[i], fromXML.Blocks[i]
				if a.Metadata.ObjectName != b.Metadata.ObjectName ||
					a.Metadata.RefFrame != b.Metadata.RefFrame ||
					a.Metadata.TimeSystem != b.Metadata.TimeSystem ||
					a.Metadata.Interpolation != b.Metadata.Interpolation ||
					a.Metadata.InterpolationDegree != b.Metadata.InterpolationDegree {
					t.Errorf("block %d metadata differs:\n\t%+v\n\t%+v", i, a.Metadata, b.Metadata)
				}
				if !a.Metadata.StartTime.Equal(b.Metadata.StartTime) ||
					!a.Metadata.StopTime.Equal(b.Metadata.StopTime) {
					t.Errorf("block %d span differs", i)
				}
				for j := range a.Lines {
					if a.Lines[j] != b.Lines[j] {
						t.Errorf("block %d line %d differs:\n\t%+v\n\t%+v", i, j, a.Lines[j], b.Lines[j])
					}
				}
				if len(a.Covariances) != len(b.Covariances) {
					t.Fatalf("block %d covariance count differs", i)
				}
				for j := range a.Covariances {
					if a.Covariances[j].Matrix != b.Covariances[j].Matrix {
						t.Errorf("block %d covariance %d differs", i, j)
					}
					if !a.Covariances[j].Epoch.Equal(b.Covariances[j].Epoch) {
						t.Errorf("block %d covariance %d epoch differs", i, j)
					}
				}
			}
		})
	}
}

// One <stateVector> block per record, and the covariance delimiters gone.
func TestOEMXMLShape(t *testing.T) {
	m, err := odm.DecodeOEM([]byte(figureG13Covariance))
	if err != nil {
		t.Fatalf("DecodeOEM: %v", err)
	}
	encoded, err := m.EncodeXML()
	if err != nil {
		t.Fatalf("EncodeXML: %v", err)
	}
	source := string(encoded)

	if got := strings.Count(source, "<stateVector>"); got != m.Records() {
		t.Errorf("wrote %d stateVector blocks for %d records", got, m.Records())
	}
	if got := strings.Count(source, "<covarianceMatrix>"); got != 2 {
		t.Errorf("wrote %d covariance blocks, want 2", got)
	}
	// The delimiters are the blocks now.
	for _, keyword := range []string{"COVARIANCE_START", "COVARIANCE_STOP", "META_START", "META_STOP"} {
		if strings.Contains(source, keyword) {
			t.Errorf("the key-value delimiter %s reached the XML", keyword)
		}
	}
	// And the covariance uses the OPM's named keywords rather than rows.
	if !strings.Contains(source, "<CX_X") || !strings.Contains(source, "<CZ_DOT_Z_DOT") {
		t.Errorf("the covariance was not written with named keywords:\n%s", source)
	}
}

func TestOEMXMLMultipleSegments(t *testing.T) {
	m, err := odm.DecodeOEM([]byte(figureG11))
	if err != nil {
		t.Fatalf("DecodeOEM: %v", err)
	}
	encoded, err := m.EncodeXML()
	if err != nil {
		t.Fatalf("EncodeXML: %v", err)
	}

	// Two metadata groups become two segments, which is what keeps clause
	// 5.2.4.6's fence intact: a consumer must not interpolate across it.
	if got := strings.Count(string(encoded), "<segment>"); got != 2 {
		t.Errorf("wrote %d segments, want 2:\n%s", got, encoded)
	}

	back, err := odm.DecodeXMLOEM(encoded)
	if err != nil {
		t.Fatalf("DecodeXMLOEM: %v", err)
	}
	if len(back.Blocks) != 2 {
		t.Errorf("read %d blocks, want 2", len(back.Blocks))
	}
}
