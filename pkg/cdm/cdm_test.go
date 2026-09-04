package cdm_test

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/ravisuhag/astro/pkg/cdm"
)

// Clause 3.6.2 of CCSDS 508.0-B-1: a CDM carrying only the obligatory
// keywords, transcribed.
//
// The Blue Book renders its minus signs as U+2212 and this is written with
// ASCII hyphen-minus, which clause 6.3.1 requires: the document allows only
// printable ASCII, so the typography in the PDF is a rendering artefact rather
// than what a real file holds.
const clause362 = `CCSDS_CDM_VERS                          = 1.0
CREATION_DATE                           = 2010-03-12T22:31:12.000
ORIGINATOR                              = JSPOC
MESSAGE_ID                              = 201113719185
TCA                                     = 2010-03-13T22:37:52.618
MISS_DISTANCE                           = 715                                     [m]
OBJECT                                  = OBJECT1
OBJECT_DESIGNATOR                       = 12345
CATALOG_NAME                            = SATCAT
OBJECT_NAME                             = SATELLITE A
INTERNATIONAL_DESIGNATOR                = 1997-030E
EPHEMERIS_NAME                          = EPHEMERIS SATELLITE A
COVARIANCE_METHOD                       = CALCULATED
MANEUVERABLE                            = YES
REF_FRAME                               = EME2000
X                                       = 2570.097065                             [km]
Y                                       = 2244.654904                             [km]
Z                                       = 6281.497978                             [km]
X_DOT                                   = 4.418769571                             [km/s]
Y_DOT                                   = 4.833547743                             [km/s]
Z_DOT                                   = -3.526774282                            [km/s]
CR_R                                    = 4.142E+01                               [m**2]
CT_R                                    = -8.579E+00                              [m**2]
CT_T                                    = 2.533E+03                               [m**2]
CN_R                                    = -2.313E+01                              [m**2]
CN_T                                    = 1.336E+01                               [m**2]
CN_N                                    = 7.098E+01                               [m**2]
CRDOT_R                                 = 2.520E-03                               [m**2/s]
CRDOT_T                                 = -5.476E+00                              [m**2/s]
CRDOT_N                                 = 8.626E-04                               [m**2/s]
CRDOT_RDOT                              = 5.744E-03                               [m**2/s**2]
CTDOT_R                                 = -1.006E-02                              [m**2/s]
CTDOT_T                                 = 4.041E-03                               [m**2/s]
CTDOT_N                                 = -1.359E-03                              [m**2/s]
CTDOT_RDOT                              = -1.502E-05                              [m**2/s**2]
CTDOT_TDOT                              = 1.049E-05                               [m**2/s**2]
CNDOT_R                                 = 1.053E-03                               [m**2/s]
CNDOT_T                                 = -3.412E-03                              [m**2/s]
CNDOT_N                                 = 1.213E-02                               [m**2/s]
CNDOT_RDOT                              = -3.004E-06                              [m**2/s**2]
CNDOT_TDOT                              = -1.091E-06                              [m**2/s**2]
CNDOT_NDOT                              = 5.529E-05                               [m**2/s**2]
OBJECT                                  = OBJECT2
OBJECT_DESIGNATOR                       = 30337
CATALOG_NAME                            = SATCAT
OBJECT_NAME                             = FENGYUN 1C DEB
INTERNATIONAL_DESIGNATOR                = 1999-025AA
EPHEMERIS_NAME                          = NONE
COVARIANCE_METHOD                       = CALCULATED
MANEUVERABLE                            = NO
REF_FRAME                               = EME2000
X                                       = 2569.540800                             [km]
Y                                       = 2245.093614                             [km]
Z                                       = 6281.599946                             [km]
X_DOT                                   = -2.888612500                            [km/s]
Y_DOT                                   = -6.007247516                            [km/s]
Z_DOT                                   = 3.328770172                             [km/s]
CR_R                                    = 1.337E+03                               [m**2]
CT_R                                    = -4.806E+04                              [m**2]
CT_T                                    = 2.492E+06                               [m**2]
CN_R                                    = -3.298E+01                              [m**2]
CN_T                                    = -7.5888E+02                             [m**2]
CN_N                                    = 7.105E+01                               [m**2]
CRDOT_R                                 = 2.591E-03                               [m**2/s]
CRDOT_T                                 = -4.152E-02                              [m**2/s]
CRDOT_N                                 = -1.784E-06                              [m**2/s]
CRDOT_RDOT                              = 6.886E-05                               [m**2/s**2]
CTDOT_R                                 = -1.016E-02                              [m**2/s]
CTDOT_T                                 = -1.506E-04                              [m**2/s]
CTDOT_N                                 = 1.637E-03                               [m**2/s]
CTDOT_RDOT                              = -2.987E-06                              [m**2/s**2]
CTDOT_TDOT                              = 1.059E-05                               [m**2/s**2]
CNDOT_R                                 = 4.400E-03                               [m**2/s]
CNDOT_T                                 = 8.482E-03                               [m**2/s]
CNDOT_N                                 = 8.633E-05                               [m**2/s]
CNDOT_RDOT                              = -1.903E-06                              [m**2/s**2]
CNDOT_TDOT                              = -4.594E-06                              [m**2/s**2]
CNDOT_NDOT                              = 5.178E-05                               [m**2/s**2]
`

func TestDecode(t *testing.T) {
	m, err := cdm.Decode([]byte(clause362))
	if err != nil {
		t.Fatalf("Decode: %v", err)
	}

	if m.Header.Version != "1.0" || m.Header.Originator != "JSPOC" {
		t.Errorf("header = %+v", m.Header)
	}
	// MESSAGE_ID is obligatory here and optional in every other navigation
	// message, because a conjunction warning has to be referable later.
	if m.Header.MessageID != "201113719185" {
		t.Errorf("MessageID = %q", m.Header.MessageID)
	}
	wantCreated := time.Date(2010, 3, 12, 22, 31, 12, 0, time.UTC)
	if !m.Header.CreationDate.Equal(wantCreated) {
		t.Errorf("CreationDate = %v, want %v", m.Header.CreationDate, wantCreated)
	}

	tca, ok := m.TCA()
	if !ok {
		t.Fatal("TCA was not read")
	}
	wantTCA := time.Date(2010, 3, 13, 22, 37, 52, 618000000, time.UTC)
	if !tca.Equal(wantTCA) {
		t.Errorf("TCA = %v, want %v", tca, wantTCA)
	}

	// The miss distance is in metres, and it carries its unit in the example.
	miss, ok := m.MissDistance()
	if !ok || miss != 715 {
		t.Errorf("MissDistance = %v, %v, want 715, true", miss, ok)
	}

	// This message gives no probability, which is legal: table 3-2 makes it
	// optional and only pairs it with its method when present.
	if _, _, ok := m.CollisionProbability(); ok {
		t.Error("a collision probability was read from a message that has none")
	}

	first, second := m.Objects[0], m.Objects[1]
	if first.Name() != "SATELLITE A" || first.Designator() != "12345" {
		t.Errorf("object 1 = %q / %q", first.Name(), first.Designator())
	}
	if second.Name() != "FENGYUN 1C DEB" || second.Designator() != "30337" {
		t.Errorf("object 2 = %q / %q", second.Name(), second.Designator())
	}
	if first.CatalogName() != "SATCAT" {
		t.Errorf("object 1 catalog = %q", first.CatalogName())
	}

	// The asymmetry an operator acts on: one object can move and the other
	// cannot.
	if m, given := first.Maneuverable(); !given || !m {
		t.Errorf("object 1 maneuverable = %v, %v, want true, true", m, given)
	}
	if m, given := second.Maneuverable(); !given || m {
		t.Errorf("object 2 maneuverable = %v, %v, want false, true", m, given)
	}

	position, velocity, ok := first.StateVector()
	if !ok {
		t.Fatal("object 1 state vector was not read")
	}
	if position != [3]float64{2570.097065, 2244.654904, 6281.497978} {
		t.Errorf("object 1 position = %v", position)
	}
	if velocity != [3]float64{4.418769571, 4.833547743, -3.526774282} {
		t.Errorf("object 1 velocity = %v", velocity)
	}
}

// Only the 6x6 block is obligatory. The three optional rows that would take it
// to 9x9 are absent here, and a reader must be able to tell that from a matrix
// whose extra rows happen to be zero.
func TestCovarianceOrder(t *testing.T) {
	m, err := cdm.Decode([]byte(clause362))
	if err != nil {
		t.Fatalf("Decode: %v", err)
	}

	object := m.Objects[0]
	if got := object.CovarianceOrder(); got != 6 {
		t.Errorf("CovarianceOrder = %d, want 6", got)
	}

	c := object.Covariance()
	if c[0][0] != 4.142e+01 {
		t.Errorf("[1,1] = %v, want 4.142E+01", c[0][0])
	}
	if c[5][5] != 5.529e-05 {
		t.Errorf("[6,6] = %v, want 5.529E-05", c[5][5])
	}
	// Symmetric, and the absent rows are zero.
	if c[0][1] != c[1][0] {
		t.Error("matrix is not symmetric")
	}
	for row := 6; row < 9; row++ {
		if c[row][row] != 0 {
			t.Errorf("row %d is non-zero in a 6x6 message", row+1)
		}
	}

	// With a drag row the order rises, and the row is readable.
	withDrag := strings.Replace(clause362,
		"CNDOT_NDOT                              = 5.529E-05                               [m**2/s**2]\n",
		"CNDOT_NDOT                              = 5.529E-05                               [m**2/s**2]\n"+
			"CDRG_R = 1.0 [m**3/kg]\nCDRG_T = 2.0 [m**3/kg]\nCDRG_N = 3.0 [m**3/kg]\n"+
			"CDRG_RDOT = 4.0 [m**3/(kg*s)]\nCDRG_TDOT = 5.0 [m**3/(kg*s)]\nCDRG_NDOT = 6.0 [m**3/(kg*s)]\n"+
			"CDRG_DRG = 7.0 [m**4/kg**2]\n", 1)

	extended, err := cdm.Decode([]byte(withDrag))
	if err != nil {
		t.Fatalf("Decode with a drag row: %v", err)
	}
	if got := extended.Objects[0].CovarianceOrder(); got != 7 {
		t.Errorf("CovarianceOrder with a drag row = %d, want 7", got)
	}
	if got := extended.Objects[0].Covariance()[6][6]; got != 7.0 {
		t.Errorf("[7,7] = %v, want 7", got)
	}
}

func TestRoundTrip(t *testing.T) {
	first, err := cdm.Decode([]byte(clause362))
	if err != nil {
		t.Fatalf("Decode: %v", err)
	}
	encoded, err := first.Encode()
	if err != nil {
		t.Fatalf("Encode: %v", err)
	}
	second, err := cdm.Decode(encoded)
	if err != nil {
		t.Fatalf("Decode on our own output: %v\n%s", err, encoded)
	}

	// Every keyword must survive, in order: clause 6.3.1.9 fixes the order and
	// a dropped keyword changes what the message says.
	if len(second.Relative.Fields) != len(first.Relative.Fields) {
		t.Errorf("relative field count changed: %d then %d",
			len(first.Relative.Fields), len(second.Relative.Fields))
	}
	for i := range first.Objects {
		a, b := first.Objects[i], second.Objects[i]
		if len(a.Fields) != len(b.Fields) {
			t.Fatalf("object %d field count changed: %d then %d", i+1, len(a.Fields), len(b.Fields))
		}
		for j := range a.Fields {
			if a.Fields[j].Keyword != b.Fields[j].Keyword {
				t.Errorf("object %d field %d keyword changed: %q then %q",
					i+1, j, a.Fields[j].Keyword, b.Fields[j].Keyword)
			}
		}
		if a.CovarianceOrder() != b.CovarianceOrder() {
			t.Errorf("object %d covariance order changed", i+1)
		}
	}
}

func TestDecodeRejects(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  error
	}{
		{
			name:  "no MESSAGE_ID, which this message alone makes obligatory",
			input: strings.Replace(clause362, "MESSAGE_ID                              = 201113719185\n", "", 1),
			want:  nil,
		},
		{
			name:  "no TCA",
			input: strings.Replace(clause362, "TCA                                     = 2010-03-13T22:37:52.618\n", "", 1),
			want:  cdm.ErrMissingKeyword,
		},
		{
			name:  "no MISS_DISTANCE",
			input: strings.Replace(clause362, "MISS_DISTANCE                           = 715                                     [m]\n", "", 1),
			want:  cdm.ErrMissingKeyword,
		},
		{
			name:  "only one object",
			input: strings.Split(clause362, "OBJECT                                  = OBJECT2")[0],
			want:  cdm.ErrMissingObject,
		},
		{
			name:  "an OBJECT value that is neither",
			input: strings.Replace(clause362, "= OBJECT2", "= OBJECT3", 1),
			want:  cdm.ErrObjectValue,
		},
		{
			name:  "the same object twice",
			input: strings.Replace(clause362, "= OBJECT2", "= OBJECT1", 1),
			want:  cdm.ErrObjectRepeated,
		},
		{
			name:  "a state vector before any OBJECT",
			input: strings.Replace(clause362, "OBJECT                                  = OBJECT1\n", "X = 1.0\nOBJECT = OBJECT1\n", 1),
			want:  cdm.ErrObjectOutOfOrder,
		},
		{
			name:  "a relative keyword inside an object section",
			input: strings.Replace(clause362, "MANEUVERABLE                            = YES", "MISS_DISTANCE = 10", 1),
			want:  cdm.ErrUnknownKeyword,
		},
		{
			name:  "a probability with no method",
			input: strings.Replace(clause362, "MISS_DISTANCE ", "COLLISION_PROBABILITY = 1.0E-05\nMISS_DISTANCE ", 1),
			want:  cdm.ErrMissingKeyword,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := cdm.Decode([]byte(tt.input))
			if tt.want == nil {
				if err == nil {
					t.Error("Decode accepted a message with no MESSAGE_ID")
				}
				return
			}
			if !errors.Is(err, tt.want) {
				t.Errorf("Decode = %v, want %v", err, tt.want)
			}
		})
	}
}

// A probability is not comparable between methods, so table 3-2 pairs them.
func TestCollisionProbability(t *testing.T) {
	input := strings.Replace(clause362, "MISS_DISTANCE ",
		"COLLISION_PROBABILITY = 4.835E-05\nCOLLISION_PROBABILITY_METHOD = FOSTER-1992\nMISS_DISTANCE ", 1)

	m, err := cdm.Decode([]byte(input))
	if err != nil {
		t.Fatalf("Decode: %v", err)
	}
	p, method, ok := m.CollisionProbability()
	if !ok {
		t.Fatal("the probability was not read")
	}
	if p != 4.835e-05 || method != "FOSTER-1992" {
		t.Errorf("probability = %v by %q", p, method)
	}
}
