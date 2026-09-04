package odm_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/ravisuhag/astro/internal/ndm"
	"github.com/ravisuhag/astro/pkg/odm"
)

// Figure G-15 of CCSDS 502.0-B-3, the smallest OCM the standard prints: a
// header with only what table 6-2 makes mandatory, a two-keyword metadata
// section, and one trajectory block.
//
// The editorial '< intervening data records omitted here >' line is left out;
// it is not part of the message.
const ocmSimple = `CCSDS_OCM_VERS = 3.0
CREATION_DATE = 2022-11-06T09:23:57
ORIGINATOR = JAPAN AEROSPACE EXPLORATION AGENCY
META_START
TIME_SYSTEM = UTC
EPOCH_TZERO = 2022-12-18T14:28:15.1172
META_STOP
TRAJ_START
CENTER_NAME = EARTH
TRAJ_REF_FRAME=ITRF2000
   0.0 2854.5 -2916.2 -5360.7 5.90 4.86 0.52 0.0037 -0.0038 -0.0070
 120.0 5478.6    434.3 -3862.5 2.50 5.87 4.29 0.0072 0.0006 -0.0051
 240.0 4146.0 -1655.8 -5038.3 4.80 5.58 2.16 0.0054 -0.0022 -0.0066
86400.0 -1553.4 -4848.7 -4406.5 6.73 1.01 -3.53 -0.002 -0.0063 -0.0058
TRAJ_STOP
`

// Figure G-16, an OCM with physical characteristics, perturbations and
// user-defined parameters.
const ocmCharacteristics = `CCSDS_OCM_VERS = 3.0
COMMENT This OCM reflects the latest conditions post-maneuver A67Z
COMMENT This example shows the specification of multiple comment lines
CLASSIFICATION        = SBU
CREATION_DATE         = 2022-11-06T09:23:57
ORIGINATOR            = JAPAN AEROSPACE EXPLORATION AGENCY
MESSAGE_ID            = OCM 201113719185

META_START
OBJECT_NAME           = OSPREY 5
INTERNATIONAL_DESIGNATOR = 2022-999A

ORIGINATOR_POC        = R. Rabbit
ORIGINATOR_POSITION   = Flight Dynamics Mission Design Lead
ORIGINATOR_PHONE      = (719)555-1234
ORIGINATOR_ADDRESS    = 5040 Spaceflight Ave., Cocoa Beach FL USA 12345

TECH_POC              = Mr. Rodgers
TECH_PHONE            = (719)555-1234
TECH_EMAIL            = email@email.XXX

TIME_SYSTEM           = UT1
EPOCH_TZERO           = 2022-12-18T00:00:00.0000

TAIMUTC_AT_TZERO      = 36      [s]
UT1MUTC_AT_TZERO      = .357    [s]
META_STOP

TRAJ_START
COMMENT           GEOCENTRIC, CARTESIAN, EARTH FIXED
COMMENT           THIS IS MY SECOND COMMENT LINE
CENTER_NAME            = EARTH
TRAJ_REF_FRAME         = EFG
TRAJ_TYPE              = CARTPV
TRAJ_UNITS             = [km, km, km, km/s, km/s, km/s]
2022-12-18T14:28:25.1172 2854.533 -2916.187 -5360.774 5.688 4.652 0.520
TRAJ_STOP

PHYS_START
COMMENT S/C Physical Chars, w/mandatory OEB_PARENT_FRAME defaulting to RSW_ROTATING:
WET_MASS              = 100.0   [kg]
OEB_Q1                = 0.03123
OEB_Q2                = 0.78543
OEB_Q3                = 0.39158
OEB_QC                = 0.47832
OEB_MAX               = 2.0     [m]
OEB_INT               = 1.0     [m]
OEB_MIN               = 0.5     [m]
AREA_ALONG_OEB_MAX    = 0.5     [m**2]
AREA_ALONG_OEB_INT    = 1.0     [m**2]
AREA_ALONG_OEB_MIN    = 2.0     [m**2]
PHYS_STOP

PERT_START
COMMENT Perturbations Specification:
ATMOSPHERIC_MODEL     = NRLMSIS00
GRAVITY_MODEL         = EGM-96: 36D 36O
GM                    = 398600.4415          [km**3/s**2]
N_BODY_PERTURBATIONS = MOON, SUN
FIXED_GEOMAG_KP       = 12.0
FIXED_F10P7           = 105.0
FIXED_F10P7_MEAN      = 120.0
PERT_STOP

USER_START
USER_DEFINED_CONSOLE_POC = MAXWELL RAFERTY
USER_DEFINED_EARTH_MODEL = WGS-84
USER_STOP
`

// Figure G-19, two covariance time histories in different element sets. One
// block is on relative time tags and the other on absolute ones, which
// clause 6.2.2.5 allows because the rule binds a block rather than a message.
//
// The '<cont.>' demarcation the figure uses is display only — the note under
// the figure says so outright — so each covariance line is one line here.
const ocmCovariances = `CCSDS_OCM_VERS      = 3.0
CREATION_DATE       = 2022-11-06T09:23:57
ORIGINATOR          = JAPAN AEROSPACE EXPLORATION AGENCY
META_START
OBJECT_NAME     = OSPREY 5
INTERNATIONAL_DESIGNATOR = 2022-999A
TIME_SYSTEM     = UTC
EPOCH_TZERO     = 2022-12-18T14:28:15.1172
META_STOP
TRAJ_START
COMMENT           GEOCENTRIC, CARTESIAN, EARTH FIXED
TRAJ_BASIS       = PREDICTED
CENTER_NAME      = EARTH
TRAJ_REF_FRAME   = TOD_EARTH
TRAJ_FRAME_EPOCH = 2022-12-18T14:28:15.1172
TRAJ_TYPE        = CARTPVA
   0.0 2854.5 -2916.2 -5360.7 5.90 4.86 0.52 0.0037 -0.0038 -0.0070
 120.0 5478.6    434.3 -3862.5 2.50 5.87 4.29 0.0072 0.0006 -0.0051
 240.0 4146.0 -1655.8 -5038.3 4.80 5.58 2.16 0.0054 -0.0022 -0.0066
86400 -1553.4 -4848.7 -4406.5 6.73 1.01 -3.53 -0.002 -0.0063 -0.0058
TRAJ_STOP
PHYS_START
COMMENT Spacecraft Physical Characteristics:
DRAG_CONST_AREA =      10.000         [m**2]
DRAG_COEFF_NOM =        2.300
WET_MASS        =    1913.000         [kg]
SRP_CONST_AREA =       10.000         [m**2]
SOLAR_RAD_COEFF =       1.300
PHYS_STOP
COV_START
COV_BASIS       = PREDICTED
COV_REF_FRAME   = J2000
COV_TYPE        = ADBARV
COV_ORDERING    = LTM
COV_UNITS       = [deg, deg, deg, deg, km, km/s]
10.00 3.331349e-04 4.618927e-04 6.782421e-04 -3.070007e-04 -4.221234e-04 3.231931e-04 -3.349365e-07 -4.686084e-07 2.484949e-07 4.296022e-10 -2.211832e-07 -2.864186e-07 1.798098e-07 2.608899e-10 1.767514e-10 -3.041346e-07 -4.989496e-07 3.540310e-07 1.869263e-10 1.008862e-10 6.224444e-10
20.0 3.442450e-04 4.507816e-04 6.893532e-04 -3.060006e-04 -4.110123e-04 3.342042e-04 -3.238254e-07 -4.575073e-07 2.373838e-07 4.307133e-10 -2.100721e-07 -2.753075e-07 1.687087e-07 2.507788e-10 1.878625e-10 -3.030235e-07 -4.878385e-07 3.430200e-07 1.758152e-10 1.007751e-10 6.224444e-10
COV_STOP
COV_START
COV_BASIS       = PREDICTED
COV_REF_FRAME   = FIXED_EARTH
COV_TYPE        = CARTP
COV_UNITS       = [km, km, km]
2022-12-18T14:31:35.1172 3.331349e-04 4.618927e-04 6.782421e-04 -3.070007e-04 -4.221234e-04 3.231931e-04
COV_STOP
PERT_START
COMMENT Perturbations specification
GM             = 398600.4415                       [km**3/s**2]
PERT_STOP
`

func TestDecodeOCMSimple(t *testing.T) {
	m, err := odm.DecodeOCM([]byte(ocmSimple))
	if err != nil {
		t.Fatalf("odm.DecodeOCM: %v", err)
	}

	if m.Header.Version != "3.0" {
		t.Errorf("version = %q", m.Header.Version)
	}
	// Table 6-3 defaults TIME_SYSTEM to UTC and this message says UTC.
	if m.TimeSystem() != "UTC" {
		t.Errorf("TIME_SYSTEM = %q", m.TimeSystem())
	}
	tzero, ok := m.EpochTZero()
	if !ok {
		t.Fatal("EPOCH_TZERO was not read")
	}
	if got := tzero.Format("2006-01-02T15:04:05.0000"); got != "2022-12-18T14:28:15.1172" {
		t.Errorf("EPOCH_TZERO = %s", got)
	}

	if len(m.Trajectories) != 1 {
		t.Fatalf("read %d trajectory blocks, want 1", len(m.Trajectories))
	}
	traj := m.Trajectories[0]
	if len(traj.Rows) != 4 {
		t.Fatalf("read %d rows, want 4", len(traj.Rows))
	}

	// Clause 6.2.1.3: the block gives neither TRAJ_TYPE nor a frame default,
	// so the recipient adopts the table's own. TRAJ_REF_FRAME is given.
	if traj.TrajType() != "CARTPV" {
		t.Errorf("TRAJ_TYPE = %q, want the table default", traj.TrajType())
	}
	if traj.RefFrame() != "ITRF2000" {
		t.Errorf("TRAJ_REF_FRAME = %q", traj.RefFrame())
	}
	if traj.CenterName() != "EARTH" {
		t.Errorf("CENTER_NAME = %q", traj.CenterName())
	}

	// The rows carry relative time tags, resolved against EPOCH_TZERO.
	if !traj.Rows[0].IsRelative() {
		t.Error("the first row was read as an absolute time")
	}
	at, err := traj.Rows[1].TimeTag(tzero)
	if err != nil {
		t.Fatalf("TimeTag: %v", err)
	}
	if got := at.Format("2006-01-02T15:04:05.0000"); got != "2022-12-18T14:30:15.1172" {
		t.Errorf("row 2 time = %s, want 120 seconds past T-zero", got)
	}
}

// The rows of figure G-15 carry nine values apiece, which is a Cartesian
// position, velocity and acceleration — CARTPVA, not the CARTPV that
// table 6-4 defaults TRAJ_TYPE to and that the note under the figure claims.
//
// The package cannot catch that, and the test says so rather than pretending
// otherwise. Clause 6.2.5.11 draws the element sets from the SANA registry, so
// nothing in the Blue Book says how many numbers a CARTPV row holds. The one
// row width that is checkable is the manoeuvre's, whose fields table 6-8 and
// table 6-9 print.
func TestOCMTrajectoryRowWidthIsNotCheckable(t *testing.T) {
	m, err := odm.DecodeOCM([]byte(ocmSimple))
	if err != nil {
		t.Fatalf("odm.DecodeOCM: %v", err)
	}

	row := m.Trajectories[0].Rows[0]
	values, err := row.Values()
	if err != nil {
		t.Fatalf("Values: %v", err)
	}
	if len(values) != 9 {
		t.Fatalf("row holds %d values, want 9", len(values))
	}
	if m.Trajectories[0].TrajType() != "CARTPV" {
		t.Error("the default TRAJ_TYPE changed; the point of this test is that it disagrees with the row")
	}
}

func TestDecodeOCMCharacteristics(t *testing.T) {
	m, err := odm.DecodeOCM([]byte(ocmCharacteristics))
	if err != nil {
		t.Fatalf("odm.DecodeOCM: %v", err)
	}

	if len(m.Header.Comments) != 2 {
		t.Errorf("header comments = %q, want 2", m.Header.Comments)
	}
	if m.Header.Classification != "SBU" || m.Header.MessageID != "OCM 201113719185" {
		t.Errorf("header = %+v", m.Header)
	}
	if m.ObjectName() != "OSPREY 5" {
		t.Errorf("OBJECT_NAME = %q", m.ObjectName())
	}
	if m.TimeSystem() != "UT1" {
		t.Errorf("TIME_SYSTEM = %q", m.TimeSystem())
	}
	if len(m.Metadata.Fields) != 13 {
		t.Errorf("metadata holds %d keywords, want 13", len(m.Metadata.Fields))
	}

	if len(m.Trajectories) != 1 || len(m.Trajectories[0].Comments) != 2 {
		t.Errorf("trajectory block = %+v", m.Trajectories)
	}
	if m.Physical == nil || len(m.Physical.Fields) != 11 {
		t.Errorf("physical section = %+v", m.Physical)
	}
	if m.Perturbations == nil || len(m.Perturbations.Fields) != 7 {
		t.Errorf("perturbations section = %+v", m.Perturbations)
	}
	if m.OrbitDetermination != nil {
		t.Error("an orbit determination section was invented")
	}

	if len(m.UserDefined) != 2 {
		t.Fatalf("read %d user-defined parameters, want 2", len(m.UserDefined))
	}
	if m.UserDefined[0].Name != "CONSOLE_POC" || m.UserDefined[0].Value != "MAXWELL RAFERTY" {
		t.Errorf("user-defined parameter = %+v", m.UserDefined[0])
	}

	// Clause 7.7.1.1 keeps the units as documentation, so they stay on the
	// value rather than being parsed away.
	if got := m.Metadata.GetOr("TAIMUTC_AT_TZERO", ""); got != "36      [s]" {
		t.Errorf("TAIMUTC_AT_TZERO = %q", got)
	}
}

func TestDecodeOCMCovariances(t *testing.T) {
	m, err := odm.DecodeOCM([]byte(ocmCovariances))
	if err != nil {
		t.Fatalf("odm.DecodeOCM: %v", err)
	}

	if len(m.Covariances) != 2 {
		t.Fatalf("read %d covariance blocks, want 2", len(m.Covariances))
	}

	// The first block is a 6x6 lower triangle on relative time tags.
	first := m.Covariances[0]
	if first.CovType() != "ADBARV" || first.CovOrdering() != odm.CovLTM {
		t.Errorf("first block = %s / %s", first.CovType(), first.CovOrdering())
	}
	if !first.Rows[0].IsRelative() {
		t.Error("the first block was read as absolute times")
	}
	matrix, err := first.CovMatrix(first.Rows[0])
	if err != nil {
		t.Fatalf("CovMatrix: %v", err)
	}
	if len(matrix) != 6 {
		t.Fatalf("matrix is %dx%d, want 6x6", len(matrix), len(matrix))
	}
	if matrix[0][0] != 3.331349e-04 {
		t.Errorf("element [1,1] = %g", matrix[0][0])
	}
	// Figure 6-1 orders an LTM [1,1], [2,1], [2,2], [3,1] and so on, so the
	// second and third values are the second row of the matrix.
	if matrix[1][0] != 4.618927e-04 || matrix[1][1] != 6.782421e-04 {
		t.Errorf("row 2 = %g %g", matrix[1][0], matrix[1][1])
	}
	// An LTM is symmetric, so the upper triangle is filled from the lower one.
	if matrix[0][1] != matrix[1][0] {
		t.Errorf("the matrix is not symmetric: [1,2] = %g, [2,1] = %g", matrix[0][1], matrix[1][0])
	}

	// The second block is a 3x3 on absolute time tags, with COV_ORDERING left
	// out so the LTM default applies.
	second := m.Covariances[1]
	if second.Rows[0].IsRelative() {
		t.Error("the second block was read as relative times")
	}
	if second.CovOrdering() != odm.CovLTM {
		t.Errorf("COV_ORDERING = %q, want the LTM default", second.CovOrdering())
	}
	small, err := second.CovMatrix(second.Rows[0])
	if err != nil {
		t.Fatalf("CovMatrix: %v", err)
	}
	if len(small) != 3 {
		t.Errorf("matrix is %dx%d, want 3x3", len(small), len(small))
	}
}

// The covariance orderings of clause 6.2.7.12.3, checked against the four
// figures that define them.
func TestCovMatrixOrderings(t *testing.T) {
	// A 3x3 whose entries say where they came from: 11, 21, 22, 31, 32, 33.
	tests := []struct {
		ordering string
		values   string
		want     [3][3]float64
	}{
		{
			// Figure 6-1: [1,1], [2,1], [2,2], [3,1], [3,2], [3,3].
			ordering: odm.CovLTM,
			values:   "11 21 22 31 32 33",
			want:     [3][3]float64{{11, 21, 31}, {21, 22, 32}, {31, 32, 33}},
		},
		{
			// Figure 6-2: [1,1], [1,2], [1,3], [2,2], [2,3], [3,3].
			ordering: odm.CovUTM,
			values:   "11 12 13 22 23 33",
			want:     [3][3]float64{{11, 12, 13}, {12, 22, 23}, {13, 23, 33}},
		},
		{
			// Figure 6-3: the whole matrix, row by row.
			ordering: odm.CovFull,
			values:   "11 12 13 21 22 23 31 32 33",
			want:     [3][3]float64{{11, 12, 13}, {21, 22, 23}, {31, 32, 33}},
		},
		{
			// Figure 6-4: the lower triangle holds covariances and the upper
			// one correlations, so the result is not symmetric and must not be
			// made so.
			ordering: odm.CovLTMWCC,
			values:   "11 12 13 21 22 23 31 32 33",
			want:     [3][3]float64{{11, 12, 13}, {21, 22, 23}, {31, 32, 33}},
		},
	}

	for _, tt := range tests {
		t.Run(tt.ordering, func(t *testing.T) {
			section := odm.OCMSection{Fields: []odm.Field{{Keyword: "COV_ORDERING", Value: tt.ordering}}}
			row := odm.DataRow{Fields: append([]string{"0.0"}, strings.Fields(tt.values)...)}

			got, err := section.CovMatrix(row)
			if err != nil {
				t.Fatalf("CovMatrix: %v", err)
			}
			for i := range tt.want {
				for j := range tt.want[i] {
					if got[i][j] != tt.want[i][j] {
						t.Errorf("[%d,%d] = %g, want %g", i+1, j+1, got[i][j], tt.want[i][j])
					}
				}
			}
		})
	}
}

func TestCovMatrixRejects(t *testing.T) {
	tests := []struct {
		name     string
		ordering string
		values   string
		want     error
	}{
		{"an ordering the standard does not define", "DIAGONAL", "1 2 3", odm.ErrUnknownCovOrdering},
		{"a triangle that is not a whole triangle", odm.CovLTM, "1 2 3 4", odm.ErrCovRowWidth},
		{"a full matrix that is not square", odm.CovFull, "1 2 3", odm.ErrCovRowWidth},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			section := odm.OCMSection{Fields: []odm.Field{{Keyword: "COV_ORDERING", Value: tt.ordering}}}
			row := odm.DataRow{Fields: append([]string{"0.0"}, strings.Fields(tt.values)...)}
			if _, err := section.CovMatrix(row); !errors.Is(err, tt.want) {
				t.Errorf("CovMatrix = %v, want %v", err, tt.want)
			}
		})
	}
}

func TestOCMRoundTrip(t *testing.T) {
	for _, source := range []struct {
		name string
		text string
	}{
		{"simple", ocmSimple},
		{"characteristics", ocmCharacteristics},
		{"covariances", ocmCovariances},
	} {
		t.Run(source.name, func(t *testing.T) {
			first, err := odm.DecodeOCM([]byte(source.text))
			if err != nil {
				t.Fatalf("odm.DecodeOCM: %v", err)
			}
			encoded, err := first.Encode()
			if err != nil {
				t.Fatalf("Encode: %v", err)
			}
			second, err := odm.DecodeOCM(encoded)
			if err != nil {
				t.Fatalf("odm.DecodeOCM on our own output: %v\n%s", err, encoded)
			}

			again, err := second.Encode()
			if err != nil {
				t.Fatalf("Encode: %v", err)
			}
			if string(again) != string(encoded) {
				t.Errorf("the second encoding differs:\n%s\n---\n%s", encoded, again)
			}
		})
	}
}

// Encode writes the sections in the order table 6-1 fixes, whatever order the
// caller built them in.
func TestEncodeOCMSectionOrder(t *testing.T) {
	m, err := odm.DecodeOCM([]byte(ocmCharacteristics))
	if err != nil {
		t.Fatalf("odm.DecodeOCM: %v", err)
	}
	encoded, err := m.Encode()
	if err != nil {
		t.Fatalf("Encode: %v", err)
	}

	text := string(encoded)
	previous := -1
	for _, delimiter := range []string{
		"META_START", "TRAJ_START", "PHYS_START", "PERT_START", "USER_START",
	} {
		at := strings.Index(text, delimiter)
		if at < 0 {
			t.Fatalf("%s is missing:\n%s", delimiter, text)
		}
		if at < previous {
			t.Errorf("%s came too early:\n%s", delimiter, text)
		}
		previous = at
	}
}

// The manoeuvre blocks of figure G-17, transcribed with the '<cont.>'
// demarcations joined as the note under figure G-19 requires.
//
// The second block is printed in the Blue Book as
//
//	MAN_COMPOSITION = TIME_ABSOLUTE, MAN_DURA, THR_X, THR_Y, THR_Z, THR_EFFIC, THR_INTERP,
//	ISP THR_MAG_SIGMA
//
// which is two mistakes in one line: the comma between the last two fields is
// missing, and table 6-8 names the field THR_ISP rather than ISP. As printed
// it names eight fields, the last of them 'ISP THR_MAG_SIGMA', while the row
// beneath it carries nine values. It is corrected here, and
// TestManCompositionRejectsFigureG17AsPrinted keeps the original.
const ocmManeuvers = `CCSDS_OCM_VERS = 3.0
CREATION_DATE = 2022-11-06T09:23:57
ORIGINATOR = JAPAN AEROSPACE EXPLORATION AGENCY
META_START
TIME_SYSTEM = UTC
EPOCH_TZERO = 2022-12-18T14:28:15.1172
META_STOP
MAN_START
COMMENT          Ten 1kg objects deployed at 1 m/s from 190kg host over 90 s time
COMMENT          20 deg off of back-track direction
MAN_ID           = E_W_20160305B
MAN_BASIS        = CANDIDATE
MAN_DEVICE_ID    = DEPLOY
MAN_PURPOSE      = DEPLOY
MAN_REF_FRAME    = RSW_ROTATING
MAN_COMPOSITION = TIME_RELATIVE, DEPLOY_ID, DEPLOY_DV_X, DEPLOY_DV_Y, DEPLOY_DV_Z, DEPLOY_MASS, DEPLOY_DV_SIGMA, DEPLOY_DV_RATIO, DEPLOY_DV_CDA
MAN_UNITS        = [n/a, km/s, km/s, km/s, kg, %, n/a, m**2]
500.0 CUBESAT_10 2.8773E-4 -9.3969E-4 1.8491E-4 -1.0 5.0 -0.005025 0.033
510.0 CUBESAT_11 1.4208E-4 -9.3969E-4 3.1111E-4 -1.0 5.0 -0.005051 0.033
520.0 CUBESAT_12 -4.8670E-5 -9.3969E-4 3.3854E-4 -1.0 5.0 -0.005076 0.033
MAN_STOP
MAN_START
COMMENT           100 s of 0.5N +in-track thr, Isp=300s, 5% 1-sigma error
MAN_ID            = E_W_20160305B
MAN_BASIS         = CANDIDATE
MAN_DEVICE_ID     = THR_01
MAN_PURPOSE       = ORBIT
MAN_REF_FRAME     = RSW_ROTATING
MAN_COMPOSITION = TIME_ABSOLUTE, MAN_DURA, THR_X, THR_Y, THR_Z, THR_EFFIC, THR_INTERP, THR_ISP, THR_MAG_SIGMA
MAN_UNITS         = [n/a, s, N, N, N, n/a, n/a, s, %]
2022-12-18T14:36:35.1172 100.0 0.0 0.5 0.0 0.95 ON 300.0 5.0
MAN_STOP
PERT_START
COMMENT Perturbations specification
GM              = 398600.4415             [km**3/s**2]
PERT_STOP
OD_START
COMMENT Orbit Determination information
OD_ID            = OD #10059
OD_PREV_ID       = OD #10058
OD_METHOD        = SF: ODTK
OD_EPOCH         = 2022-12-18T11:17:33
OBS_USED         = 273
TRACKS_USED      = 91
OD_STOP
`

func TestDecodeOCMManeuvers(t *testing.T) {
	m, err := odm.DecodeOCM([]byte(ocmManeuvers))
	if err != nil {
		t.Fatalf("odm.DecodeOCM: %v", err)
	}

	if len(m.Maneuvers) != 2 {
		t.Fatalf("read %d manoeuvre blocks, want 2", len(m.Maneuvers))
	}

	// The first block deploys objects, so its fields come from table 6-9.
	deploy := m.Maneuvers[0]
	fields, err := deploy.ManFields()
	if err != nil {
		t.Fatalf("ManFields: %v", err)
	}
	if len(fields) != 9 {
		t.Fatalf("MAN_COMPOSITION names %d fields, want 9", len(fields))
	}
	// Clause 8.11.18 says a manoeuvre line holds one field per
	// MAN_COMPOSITION entry "plus one for the timetag". That is a
	// double-count: clause 6.2.8.18 already puts the time tag in
	// MAN_COMPOSITION as its first entry, and every row of this figure has
	// exactly as many fields as the keyword names.
	for i, row := range deploy.Rows {
		if len(row.Fields) != len(fields) {
			t.Errorf("row %d holds %d fields, MAN_COMPOSITION names %d",
				i+1, len(row.Fields), len(fields))
		}
	}
	if fields[0] != "TIME_RELATIVE" || !deploy.Rows[0].IsRelative() {
		t.Errorf("the first block should be on relative time tags")
	}
	if deploy.DutyCycle() != "CONTINUOUS" {
		t.Errorf("DC_TYPE = %q, want the table default", deploy.DutyCycle())
	}

	// The second block thrusts, so its fields come from table 6-8.
	thrust := m.Maneuvers[1]
	if _, err := thrust.ManFields(); err != nil {
		t.Fatalf("ManFields: %v", err)
	}
	if thrust.Rows[0].IsRelative() {
		t.Error("the second block should be on absolute time tags")
	}
	if m.OrbitDetermination == nil || m.Perturbations == nil {
		t.Error("the orbit determination and perturbations sections were not read")
	}
}

// Figure G-17 as the Blue Book prints it does not parse.
//
// This is not a complaint about the standard, and it is not something the
// package should work around: MAN_COMPOSITION is the only statement of what a
// manoeuvre row's columns mean, so accepting one that names eight fields for a
// nine-field row would hand the caller columns shifted by one, silently.
func TestManCompositionRejectsFigureG17AsPrinted(t *testing.T) {
	asPrinted := strings.Replace(ocmManeuvers,
		"MAN_COMPOSITION = TIME_ABSOLUTE, MAN_DURA, THR_X, THR_Y, THR_Z, THR_EFFIC, THR_INTERP, THR_ISP, THR_MAG_SIGMA",
		"MAN_COMPOSITION = TIME_ABSOLUTE, MAN_DURA, THR_X, THR_Y, THR_Z, THR_EFFIC, THR_INTERP, ISP THR_MAG_SIGMA",
		1)

	if _, err := odm.DecodeOCM([]byte(asPrinted)); !errors.Is(err, odm.ErrMalformedManComposition) {
		t.Errorf("odm.DecodeOCM = %v, want %v", err, odm.ErrMalformedManComposition)
	}
}

func TestManFieldsRejects(t *testing.T) {
	tests := []struct {
		name        string
		composition string
	}{
		{
			// Clause 6.2.8.15: the two tables must not be commingled.
			name:        "propulsive and deployment fields together",
			composition: "TIME_RELATIVE, DEPLOY_ID, THR_X",
		},
		{
			// Clause 6.2.8.16 fixes the order within a table.
			name:        "fields out of the order the table fixes",
			composition: "TIME_RELATIVE, THR_X, MAN_DURA",
		},
		{
			// Clause 6.2.8.18: the first entry is the time tag.
			name:        "no time tag first",
			composition: "MAN_DURA, THR_X",
		},
		{
			name:        "a field no manoeuvre table holds",
			composition: "TIME_RELATIVE, THRUST_DIRECTION",
		},
		{
			name:        "a time tag and nothing else",
			composition: "TIME_RELATIVE",
		},
		{
			// Both time tags is two first entries, which clause 6.2.8.18 does
			// not allow; it is also out of order for neither table repeats.
			name:        "both time tag kinds",
			composition: "TIME_ABSOLUTE, TIME_RELATIVE, MAN_DURA",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			section := odm.OCMSection{Fields: []odm.Field{{Keyword: "MAN_COMPOSITION", Value: tt.composition}}}
			if _, err := section.ManFields(); !errors.Is(err, odm.ErrMalformedManComposition) {
				t.Errorf("ManFields = %v, want %v", err, odm.ErrMalformedManComposition)
			}
		})
	}
}

func TestDecodeOCMRejects(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  error
	}{
		{
			name:  "another message type",
			input: strings.Replace(ocmSimple, "CCSDS_OCM_VERS", "CCSDS_OPM_VERS", 1),
			want:  ndm.ErrWrongMessageType,
		},
		{
			// Clause 6.2.4 makes EPOCH_TZERO mandatory with no default,
			// because a relative time tag has nothing to be relative to
			// without it.
			name:  "no EPOCH_TZERO",
			input: strings.Replace(ocmSimple, "EPOCH_TZERO = 2022-12-18T14:28:15.1172\n", "", 1),
			want:  odm.ErrMissingKeyword,
		},
		{
			// Clause 6.2.2.1 fixes the keyword order.
			name: "metadata keywords out of the table's order",
			input: strings.Replace(ocmSimple,
				"TIME_SYSTEM = UTC\nEPOCH_TZERO = 2022-12-18T14:28:15.1172",
				"EPOCH_TZERO = 2022-12-18T14:28:15.1172\nTIME_SYSTEM = UTC", 1),
			want: odm.ErrKeywordsOutOfOrder,
		},
		{
			// Clause 6.2.5.1 and its siblings: only the keywords in that
			// section's table may be used there.
			name:  "a covariance keyword in a trajectory block",
			input: strings.Replace(ocmSimple, "CENTER_NAME = EARTH", "COV_TYPE = CARTPV", 1),
			want:  odm.ErrUnknownKeyword,
		},
		{
			name:  "a keyword that belongs to no table at all",
			input: strings.Replace(ocmSimple, "CENTER_NAME = EARTH", "CENTRE_NAME = EARTH", 1),
			want:  odm.ErrUnknownKeyword,
		},
		{
			name:  "the same keyword twice in one section",
			input: strings.Replace(ocmSimple, "CENTER_NAME = EARTH", "CENTER_NAME = EARTH\nCENTER_NAME = MARS", 1),
			want:  odm.ErrDuplicateKeyword,
		},
		{
			// Table 6-1 fixes the section order: physical characteristics come
			// after the trajectories, not before them.
			name: "sections out of table 6-1's order",
			input: strings.Replace(ocmSimple, "TRAJ_START",
				"PHYS_START\nWET_MASS = 100.0\nPHYS_STOP\nTRAJ_START", 1),
			want: odm.ErrSectionsOutOfOrder,
		},
		{
			// Clause 6.2.6.2 allows one physical characteristics section.
			name: "two physical characteristics sections",
			input: ocmSimple +
				"PHYS_START\nWET_MASS = 100.0\nPHYS_STOP\nPHYS_START\nDRY_MASS = 90.0\nPHYS_STOP\n",
			want: odm.ErrDuplicateSection,
		},
		{
			// Clause 6.2.4.3 allows one metadata section.
			name:  "two metadata sections",
			input: strings.Replace(ocmSimple, "TRAJ_START", "META_START\nTIME_SYSTEM = UTC\nMETA_STOP\nTRAJ_START", 1),
			want:  odm.ErrDuplicateSection,
		},
		{
			name:  "a section that is never closed",
			input: strings.Replace(ocmSimple, "TRAJ_STOP\n", "", 1),
			want:  odm.ErrUnterminatedSection,
		},
		{
			name:  "a section closed by the wrong delimiter",
			input: strings.Replace(ocmSimple, "TRAJ_STOP", "COV_STOP", 1),
			want:  odm.ErrUnterminatedSection,
		},
		{
			// Clause 6.2.2.5: a block picks relative or absolute and keeps it.
			name:  "relative and absolute time tags in one block",
			input: strings.Replace(ocmSimple, " 240.0 4146.0", " 2022-12-18T14:32:15.1172 4146.0", 1),
			want:  odm.ErrMixedTimeTags,
		},
		{
			// Clause 6.2.2.4.
			name:  "duplicate time tags in one block",
			input: strings.Replace(ocmSimple, " 240.0 4146.0", " 120.0 4146.0", 1),
			want:  odm.ErrDuplicateTimeTag,
		},
		{
			// Clause 6.2.5.6: a trajectory runs forward in time.
			name:  "trajectory rows that go backwards",
			input: strings.Replace(ocmSimple, " 240.0 4146.0", " 60.0 4146.0", 1),
			want:  odm.ErrTimeTagsOutOfOrder,
		},
		{
			// Clause 6.2.10.5.
			name: "orbit determination with no perturbations section",
			input: ocmSimple + "OD_START\nOD_ID = OD #1\nOD_METHOD = SF: ODTK\n" +
				"OD_EPOCH = 2022-12-18T11:17:33\nOD_STOP\n",
			want: odm.ErrMissingPerturbations,
		},
		{
			// Table 6-11 marks all three of these mandatory with no default.
			name: "orbit determination with no OD_METHOD",
			input: ocmSimple + "PERT_START\nGM = 398600.4415\nPERT_STOP\n" +
				"OD_START\nOD_ID = OD #1\nOD_EPOCH = 2022-12-18T11:17:33\nOD_STOP\n",
			want: odm.ErrMissingKeyword,
		},
		{
			// Table 6-3 makes the two clock keywords conditional on SCLK.
			name:  "the SCLK time system without its clock keywords",
			input: strings.Replace(ocmSimple, "TIME_SYSTEM = UTC", "TIME_SYSTEM = SCLK", 1),
			want:  odm.ErrMissingSCLKFields,
		},
		{
			// Table 6-12 marks USER_DEFINED_x mandatory in the section.
			name:  "an empty user-defined section",
			input: ocmSimple + "USER_START\nUSER_STOP\n",
			want:  odm.ErrMissingKeyword,
		},
		{
			name:  "a keyword in the user-defined section without the prefix",
			input: ocmSimple + "USER_START\nEARTH_MODEL = WGS-84\nUSER_STOP\n",
			want:  odm.ErrUnknownKeyword,
		},
		{
			name:  "a data row outside any section",
			input: strings.Replace(ocmSimple, "TRAJ_START\n", "", 1),
			want:  odm.ErrUnexpectedDelimiter,
		},
		{
			// Table 6-1 puts a section's data rows after its keywords, and
			// clause 6.2.2.1 fixes the keyword order.
			name:  "a keyword after a data row",
			input: strings.Replace(ocmSimple, "TRAJ_STOP", "TRAJ_TYPE = CARTPVA\nTRAJ_STOP", 1),
			want:  odm.ErrKeywordsOutOfOrder,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := odm.DecodeOCM([]byte(tt.input)); !errors.Is(err, tt.want) {
				t.Errorf("odm.DecodeOCM = %v, want %v", err, tt.want)
			}
		})
	}
}

// Clause 6.2.1.1's note calls an OCM with no data blocks a degenerate case and
// says it was an intentional choice: the metadata alone carries contact
// details, links to other messages and timing sources.
func TestOCMWithNoDataBlocks(t *testing.T) {
	const text = `CCSDS_OCM_VERS = 3.0
CREATION_DATE = 2022-11-06T09:23:57
ORIGINATOR = JAXA
META_START
TECH_POC = Mr. Rodgers
TECH_EMAIL = email@email.XXX
EPOCH_TZERO = 2022-12-18T14:28:15.1172
META_STOP
`
	m, err := odm.DecodeOCM([]byte(text))
	if err != nil {
		t.Fatalf("odm.DecodeOCM: %v", err)
	}
	if len(m.Trajectories) != 0 || m.Physical != nil || len(m.Covariances) != 0 {
		t.Error("data blocks were invented")
	}
	if got := m.Metadata.GetOr("TECH_POC", ""); got != "Mr. Rodgers" {
		t.Errorf("TECH_POC = %q", got)
	}
	// TIME_SYSTEM was left out, and table 6-3 defaults it to UTC.
	if m.TimeSystem() != "UTC" {
		t.Errorf("TIME_SYSTEM = %q, want the UTC default", m.TimeSystem())
	}
}

func TestOCMHumanize(t *testing.T) {
	m, err := odm.DecodeOCM([]byte(ocmCovariances))
	if err != nil {
		t.Fatalf("odm.DecodeOCM: %v", err)
	}

	text := m.Humanize()
	for _, want := range []string{
		"Orbit Comprehensive Message 3.0",
		"OSPREY 5",
		"Trajectory 1",
		"CARTPVA in TOD_EARTH",
		"Covariance 1",
		"ADBARV ordered LTM",
	} {
		if !strings.Contains(text, want) {
			t.Errorf("Humanize is missing %q:\n%s", want, text)
		}
	}
}
