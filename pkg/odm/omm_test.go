package odm_test

import (
	"errors"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/ravisuhag/astro/pkg/odm"
)

// Figure G-7 of CCSDS 502.0-B-3: a TLE-based OMM without a covariance matrix.
// Its creation date is in the ordinal form, which clause 7.5.10 allows
// alongside the calendar one in the same file.
const figureG7 = `CCSDS_OMM_VERS = 3.0
CREATION_DATE = 2020-065T16:00:00
ORIGINATOR     = NOAA
MESSAGE_ID     = OMM 202013719185

OBJECT_NAME    = GOES 9
OBJECT_ID      = 1995-025A
CENTER_NAME    = EARTH
REF_FRAME      = TEME
TIME_SYSTEM    = UTC
MEAN_ELEMENT_THEORY = SGP/SGP4

EPOCH             = 2020-064T10:34:41.4264
MEAN_MOTION       = 1.00273272
ECCENTRICITY      = 0.0005013
INCLINATION       =   3.0539
RA_OF_ASC_NODE    = 81.7939
ARG_OF_PERICENTER = 249.2363
MEAN_ANOMALY      = 150.1602
GM                = 398600.8
EPHEMERIS_TYPE    = 0
CLASSIFICATION_TYPE = U
NORAD_CAT_ID      = 23581
ELEMENT_SET_NO    = 0925
REV_AT_EPOCH      = 4316
BSTAR             = 0.0001
MEAN_MOTION_DOT   = -0.00000113
MEAN_MOTION_DDOT = 0.0
`

func TestDecodeOMM(t *testing.T) {
	m, err := odm.DecodeOMM([]byte(figureG7))
	if err != nil {
		t.Fatalf("DecodeOMM: %v", err)
	}

	if m.Header.Originator != "NOAA" || m.Header.MessageID != "OMM 202013719185" {
		t.Errorf("header = %+v", m.Header)
	}
	// 2020 day 65 is 2020-03-05: a leap year, so the ordinal form has to know
	// February had 29 days.
	wantCreated := time.Date(2020, 3, 5, 16, 0, 0, 0, time.UTC)
	if !m.Header.CreationDate.Equal(wantCreated) {
		t.Errorf("CreationDate = %v, want %v", m.Header.CreationDate, wantCreated)
	}

	md := m.Metadata
	if md.ObjectName != "GOES 9" || md.ObjectID != "1995-025A" {
		t.Errorf("object = %q / %q", md.ObjectName, md.ObjectID)
	}
	if md.MeanElementTheory != "SGP/SGP4" {
		t.Errorf("MeanElementTheory = %q", md.MeanElementTheory)
	}
	if !md.IsTLEBased() {
		t.Error("IsTLEBased is false for SGP/SGP4")
	}

	e := m.Data.Elements
	// Clause 4.2.4.6 requires MEAN_MOTION rather than SEMI_MAJOR_AXIS on a
	// TLE-based message, and the two are alternatives everywhere.
	if !e.UsesMeanMotion {
		t.Fatal("UsesMeanMotion is false; the message gives MEAN_MOTION")
	}
	if e.MeanMotion != 1.00273272 || e.SemiMajorAxis != 0 {
		t.Errorf("size = motion %v, axis %v", e.MeanMotion, e.SemiMajorAxis)
	}
	if e.Eccentricity != 0.0005013 || e.Inclination != 3.0539 || e.MeanAnomaly != 150.1602 {
		t.Errorf("elements = %+v", e)
	}
	wantEpoch := time.Date(2020, 3, 4, 10, 34, 41, 426400000, time.UTC)
	if !e.Epoch.Equal(wantEpoch) {
		t.Errorf("Epoch = %v, want %v", e.Epoch, wantEpoch)
	}

	tle := m.Data.TLE
	if tle == nil {
		t.Fatal("no TLE parameters")
	}
	if tle.NoradCatID != 23581 || tle.RevAtEpoch != 4316 {
		t.Errorf("TLE = %+v", tle)
	}
	// Leading zeroes are legal in an integer (clause 7.5.4), so 0925 is 925.
	if tle.ElementSetNo != 925 {
		t.Errorf("ElementSetNo = %d, want 925", tle.ElementSetNo)
	}
	if tle.ClassificationType != "U" {
		t.Errorf("ClassificationType = %q, want U", tle.ClassificationType)
	}
	if tle.UsesBTerm || tle.BStar != 0.0001 {
		t.Errorf("drag term = BSTAR %v, BTERM used %v", tle.BStar, tle.UsesBTerm)
	}
	if tle.UsesAgom {
		t.Error("UsesAgom is true; the message gives MEAN_MOTION_DDOT")
	}

	if m.Data.Covariance != nil {
		t.Error("a covariance block was read from a message that has none")
	}
}

// Figure G-8: the same message with the covariance matrix, which the OMM
// carries as 21 named keywords rather than as the positional rows an OEM uses.
func TestDecodeOMMWithCovariance(t *testing.T) {
	input := strings.TrimSuffix(figureG7, "") + `
COV_REF_FRAME = TEME
CX_X = 3.331349476038534e-04
CY_X = 4.618927349220216e-04
CY_Y = 6.782421679971363e-04
CZ_X = -3.070007847730449e-04
CZ_Y = -4.221234189514228e-04
CZ_Z = 3.231931992380369e-04
CX_DOT_X = -3.349365033922630e-07
CX_DOT_Y = -4.686084221046758e-07
CX_DOT_Z = 2.484949578400095e-07
CX_DOT_X_DOT = 4.296022805587290e-10
CY_DOT_X = -2.211832501084875e-07
CY_DOT_Y = -2.864186892102733e-07
CY_DOT_Z = 1.798098699846038e-07
CY_DOT_X_DOT = 2.608899201686016e-10
CY_DOT_Y_DOT = 1.767514756338532e-10
CZ_DOT_X = -3.041346050686871e-07
CZ_DOT_Y = -4.989496988610662e-07
CZ_DOT_Z = 3.540310904497689e-07
CZ_DOT_X_DOT = 1.869263192954590e-10
CZ_DOT_Y_DOT = 1.008862586240695e-10
CZ_DOT_Z_DOT = 6.224444338635500e-10
`

	m, err := odm.DecodeOMM([]byte(input))
	if err != nil {
		t.Fatalf("DecodeOMM: %v", err)
	}

	c := m.Data.Covariance
	if c == nil {
		t.Fatal("no covariance block")
	}
	if c.RefFrame != "TEME" {
		t.Errorf("COV_REF_FRAME = %q, want TEME", c.RefFrame)
	}
	if c.Matrix[0][0] != 3.331349476038534e-04 {
		t.Errorf("[1,1] = %v", c.Matrix[0][0])
	}
	if c.Matrix[5][5] != 6.224444338635500e-10 {
		t.Errorf("[6,6] = %v", c.Matrix[5][5])
	}
	if c.Matrix[0][1] != c.Matrix[1][0] {
		t.Error("matrix is not symmetric")
	}
}

func TestOMMRoundTrip(t *testing.T) {
	first, err := odm.DecodeOMM([]byte(figureG7))
	if err != nil {
		t.Fatalf("DecodeOMM: %v", err)
	}
	encoded, err := first.Encode()
	if err != nil {
		t.Fatalf("Encode: %v", err)
	}
	second, err := odm.DecodeOMM(encoded)
	if err != nil {
		t.Fatalf("DecodeOMM on our own output: %v\n%s", err, encoded)
	}

	// Comments are dropped from the comparison: a round trip normalises where
	// they attach, and what has to survive is the numbers.
	a, b := first.Data.Elements, second.Data.Elements
	a.Comments, b.Comments = nil, nil
	if !reflect.DeepEqual(a, b) {
		t.Errorf("elements changed:\n\t%+v\n\t%+v", a, b)
	}

	x, y := *first.Data.TLE, *second.Data.TLE
	x.Comments, y.Comments = nil, nil
	if !reflect.DeepEqual(x, y) {
		t.Errorf("TLE parameters changed:\n\t%+v\n\t%+v", x, y)
	}
}

// Clause 4.2.4.6 fixes four things about a TLE-based OMM. Any of them wrong
// produces a message an SGP4 propagator accepts and mispropagates, because the
// propagator assumes all four.
func TestOMMTLEConventions(t *testing.T) {
	tests := []struct {
		name string
		from string
		to   string
		want error
	}{
		{"a center other than EARTH", "CENTER_NAME    = EARTH", "CENTER_NAME    = MARS", odm.ErrTLEConventions},
		{"a time system other than UTC", "TIME_SYSTEM    = UTC", "TIME_SYSTEM    = TAI", odm.ErrTLEConventions},
		{
			name: "a semi-major axis instead of mean motion",
			from: "MEAN_MOTION       = 1.00273272",
			to:   "SEMI_MAJOR_AXIS   = 42165.0",
			want: odm.ErrTLEConventions,
		},
		{
			// Clause 4.2.4.9: TEME is allowed for a TLE-based OMM "and in no
			// other circumstances", because no international convention pins
			// it down.
			name: "TEME on a message that is not TLE-based",
			from: "MEAN_ELEMENT_THEORY = SGP/SGP4",
			to:   "MEAN_ELEMENT_THEORY = DSST",
			want: odm.ErrTEMEWithoutTLE,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			input := strings.Replace(figureG7, tt.from, tt.to, 1)
			if input == figureG7 {
				t.Fatalf("the test input did not change; %q not found", tt.from)
			}
			if _, err := odm.DecodeOMM([]byte(input)); !errors.Is(err, tt.want) {
				t.Errorf("DecodeOMM = %v, want %v", err, tt.want)
			}
		})
	}
}

// Three pairs of keywords share one slot in table 4-3, and which name applies
// is decided by MEAN_ELEMENT_THEORY. A message giving both has not said which
// it means.
func TestOMMAlternativeKeywords(t *testing.T) {
	tests := []struct {
		name  string
		extra string
		want  error
	}{
		{"both size keywords", "SEMI_MAJOR_AXIS = 42165.0\n", odm.ErrBothSizeKeywords},
		{"both drag terms", "BTERM = 0.02\n", odm.ErrBothDragKeywords},
		{"both second-derivative terms", "AGOM = 0.01\n", odm.ErrBothDragKeywords},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := odm.DecodeOMM([]byte(figureG7 + tt.extra)); !errors.Is(err, tt.want) {
				t.Errorf("DecodeOMM = %v, want %v", err, tt.want)
			}
		})
	}
}

// An SGP4-XP message uses the other half of two of those pairs: BTERM for the
// ballistic coefficient and AGOM for solar radiation, both in m**2/kg.
func TestOMMSGP4XPUsesTheOtherKeywords(t *testing.T) {
	input := strings.NewReplacer(
		"MEAN_ELEMENT_THEORY = SGP/SGP4", "MEAN_ELEMENT_THEORY = SGP4-XP",
		"BSTAR             = 0.0001", "BTERM             = 0.0015",
		"MEAN_MOTION_DDOT = 0.0", "AGOM = 0.001",
	).Replace(figureG7)

	m, err := odm.DecodeOMM([]byte(input))
	if err != nil {
		t.Fatalf("DecodeOMM: %v", err)
	}
	tle := m.Data.TLE
	if !tle.UsesBTerm || tle.BTerm != 0.0015 {
		t.Errorf("BTERM = %v, used %v", tle.BTerm, tle.UsesBTerm)
	}
	if !tle.UsesAgom || tle.Agom != 0.001 {
		t.Errorf("AGOM = %v, used %v", tle.Agom, tle.UsesAgom)
	}

	// And they must survive a round trip under their own names.
	encoded, err := m.Encode()
	if err != nil {
		t.Fatalf("Encode: %v", err)
	}
	if !strings.Contains(string(encoded), "BTERM") || !strings.Contains(string(encoded), "AGOM") {
		t.Errorf("the SGP4-XP keywords were not written back:\n%s", encoded)
	}
}

func TestDecodeOMMRejects(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  error
	}{
		{
			name:  "neither size keyword",
			input: strings.Replace(figureG7, "MEAN_MOTION       = 1.00273272\n", "", 1),
			want:  odm.ErrSizeKeywordMissing,
		},
		{
			name:  "no mean element theory",
			input: strings.Replace(figureG7, "MEAN_ELEMENT_THEORY = SGP/SGP4\n", "", 1),
			want:  odm.ErrMissingKeyword,
		},
		{
			name:  "a missing mandatory element",
			input: strings.Replace(figureG7, "ECCENTRICITY      = 0.0005013\n", "", 1),
			want:  odm.ErrMissingKeyword,
		},
		{
			// Clause 4.2.4.8 says manoeuvres are not accommodated in an OMM,
			// so the OPM's manoeuvre keywords are simply unknown here.
			name:  "a maneuver keyword",
			input: figureG7 + "MAN_EPOCH_IGNITION = 2020-064T11:00:00\n",
			want:  odm.ErrUnknownKeyword,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := odm.DecodeOMM([]byte(tt.input)); !errors.Is(err, tt.want) {
				t.Errorf("DecodeOMM = %v, want %v", err, tt.want)
			}
		})
	}
}
