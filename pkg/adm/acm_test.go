package adm_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/ravisuhag/astro/internal/ndm"
	"github.com/ravisuhag/astro/pkg/adm"
)

// Figure G-6 of CCSDS 504.0-B-2, the smallest ACM the standard prints: the
// three mandatory header keywords, a metadata section, and one attitude block.
//
// The editorial '< additional data records omitted here >' line is left out;
// it is not part of the message.
const acmSimple = `CCSDS_ACM_VERS = 2.0
CREATION_DATE   = 1998-11-06T09:23:57
ORIGINATOR      = JAXA
MESSAGE_ID = A7015Z4

META_START
OBJECT_NAME     = EUROBIRD-4A
INTERNATIONAL_DESIGNATOR = 2000-052A
TIME_SYSTEM     = UTC
EPOCH_TZERO     = 1998-12-18T14:28:15.1172
META_STOP

ATT_START
REF_FRAME_A    = J2000
REF_FRAME_B    = SC_BODY_1
NUMBER_STATES = 4
ATT_TYPE = QUATERNION

0.0     0.73566       -0.50547          0.41309         0.180707
0.25    0.73529       -0.50531          0.41375         0.181158
0.50    0.73492       -0.50515          0.41441         0.181610
ATT_STOP
`

// Figure G-7: an attitude history with gyro bias, a momentum management
// manoeuvre, and an attitude determination section holding four sensor blocks.
//
// One correction. The figure's second sensor block writes SENSOR_USED_ with a
// trailing underscore, which table 5-8 does not list; it is SENSOR_USED here,
// and TestSensorKeywordTypoIsRefused keeps the original.
const acmManeuver = `CCSDS_ACM_VERS    = 2.0

CREATION_DATE    = 2017-12-01T00:00:00
ORIGINATOR       = NASA
MESSAGE_ID = A7015Z5

META_START
OBJECT_NAME      = SDO
INTERNATIONAL_DESIGNATOR = 2010-005A
TIME_SYSTEM      = UTC
EPOCH_TZERO      = 2017-12-26T19:40:00.000
META_STOP


ATT_START
COMMENT             OBC Attitude and Bias during momentum management maneuver
REF_FRAME_A       = J2000
REF_FRAME_B       = SC_BODY_1
NUMBER_STATES = 7
ATT_TYPE          = QUATERNION
RATE_TYPE         = GYRO_BIAS

0.000000 0.1153 -0.1424 0.8704 0.4571 2.271e-06 -4.405e-06 -3.785e-06
2.000000 0.1153 -0.1424 0.8704 0.4571 2.271e-06 -4.405e-06 -3.785e-06
99.80183 0.1017 -0.1332 0.8806 0.4433 2.587e-06 8.769e-06 5.436e-06
599.80275 0.1152 -0.1423 0.8704 0.4571 2.48e-06 -4.350e-06 -3.779e-06
ATT_STOP


MAN_START
COMMENT             Momentum management maneuver
MAN_PURPOSE      = MOM_DESAT
MAN_BEGIN_TIME   = 100.0
MAN_DURATION     = 450.0
ACTUATOR_USED    = ATT-THRUSTER
TARGET_MOMENTUM = 1.30 -16.400 -11.350
TARGET_MOM_FRAME = J2000
MAN_STOP


AD_START
COMMENT             SDO Onboard Filter
AD_METHOD         = EKF
ATTITUDE_SOURCE   = OBC
ATTITUDE_STATES   = QUATERNION
REF_FRAME_A       = J2000
REF_FRAME_B       = SC_BODY_1
SENSOR_START
SENSOR_NUMBER     = 1
SENSOR_USED       = AST
SENSOR_STOP
SENSOR_START
SENSOR_NUMBER     = 2
SENSOR_USED       = AST
SENSOR_STOP
SENSOR_START
SENSOR_NUMBER     = 5
SENSOR_USED       = DSS
SENSOR_STOP
SENSOR_START
SENSOR_NUMBER     = 6
SENSOR_USED       = IMU
SENSOR_STOP
AD_STOP
`

// Figure G-8: the space object physical characteristics section.
const acmPhysical = `CCSDS_ACM_VERS     = 2.0

CREATION_DATE              = 1998-11-06T09:23:57
ORIGINATOR                 = JAXA
MESSAGE_ID = A7015Z6

META_START
OBJECT_NAME                = TEST_SAT
ORIGINATOR_POC             = Ms. Rodgers, (719)555-5555, email@email.XXX
TIME_SYSTEM                = TAI
EPOCH_TZERO                = 1998-12-18T14:28:15.1172
TAIMUTC_AT_TZERO           = 36       [s]
META_STOP

PHYS_START
COMMENT               Spacecraft Physical Parameters
WET_MASS            = 1916   [kg]
CP_REF_FRAME        = SC_BODY_1
CP                  = 0.04 -0.78 -0.023 [m]
IXX                 = 752    [kg*m**2]
IYY                 = 1305   [kg*m**2]
IZZ                 = 1490   [kg*m**2]
IXY                 = 81.1   [kg*m**2]
IXZ                 = -25.7 [kg*m**2]
IYZ                 = 74.1   [kg*m**2]
PHYS_STOP
`

// Figure G-9: a covariance time history beside the attitude determination
// section that produced it.
const acmCovariance = `CCSDS_ACM_VERS   = 2.0

CREATION_DATE   = 2017-12-30T00:00:00
ORIGINATOR      = NASA
MESSAGE_ID = A7015Z7

META_START
OBJECT_NAME     = LRO
INTERNATIONAL_DESIGNATOR = 2009-031A
TIME_SYSTEM     = UTC
EPOCH_TZERO     = 2017-12-30T00:00:00.0
ACM_DATA_ELEMENTS = COV, AD
META_STOP

COV_START
COMMENT Diagonal Covariance for LRO Onboard Kalman Filter
COV_BASIS     = DETERMINED_OBC
COV_REF_FRAME = SC_BODY_1
COV_TYPE      = ANGLE_GYROBIAS

0.0     6.74E-11 8.10E-11 9.22E-11 1.11E-15 1.11E-15 1.12E-15
1.096694 6.74E-11 8.10E-11 9.22E-11 1.11E-15 1.11E-15 1.12E-15
59.896697 6.74E-11 8.10E-11 9.22E-11 1.11E-15 1.11E-15 1.12E-15
COV_STOP


AD_START
COMMENT LRO Onboard Filter, A Multiplicative Extended Kalman Filter
AD_METHOD        = EKF
ATTITUDE_SOURCE  = OBC
NUMBER_STATES    = 7
ATTITUDE_STATES  = QUATERNION
COV_TYPE         = ANGLE_GYROBIAS
REF_FRAME_A      = EME2000
REF_FRAME_B      = SC_BODY_1
RATE_STATES      = GYRO_BIAS
SENSOR_START
SENSOR_NUMBER    = 2
SENSOR_USED      = AST
SENSOR_STOP
SENSOR_START
SENSOR_NUMBER    = 4
SENSOR_USED      = AST
SENSOR_STOP
SENSOR_START
SENSOR_NUMBER    = 7
SENSOR_USED      = IMU
SENSOR_STOP
AD_STOP
`

func TestDecodeACMSimple(t *testing.T) {
	m, err := adm.DecodeACM([]byte(acmSimple))
	if err != nil {
		t.Fatalf("DecodeACM: %v", err)
	}

	if m.Header.Version != "2.0" || m.Header.MessageID != "A7015Z4" {
		t.Errorf("header = %+v", m.Header)
	}
	if m.ObjectName() != "EUROBIRD-4A" || m.TimeSystem() != "UTC" {
		t.Errorf("metadata = %q / %q", m.ObjectName(), m.TimeSystem())
	}
	tzero, ok := m.EpochTZero()
	if !ok {
		t.Fatal("EPOCH_TZERO was not read")
	}

	if len(m.Attitudes) != 1 {
		t.Fatalf("read %d attitude blocks, want 1", len(m.Attitudes))
	}
	att := m.Attitudes[0]
	if att.AttitudeType() != "QUATERNION" {
		t.Errorf("ATT_TYPE = %q", att.AttitudeType())
	}
	// RATE_TYPE is absent, which means no rate data rather than an unknown
	// type: table 5-4 lists NONE among its values.
	if att.RateType() != "NONE" {
		t.Errorf("RATE_TYPE = %q, want NONE", att.RateType())
	}
	if from, to := att.Frames(); from != "J2000" || to != "SC_BODY_1" {
		t.Errorf("frames = %q to %q", from, to)
	}

	states, ok := att.StateCount()
	if !ok || states != 4 {
		t.Fatalf("StateCount = %d, %v; want 4", states, ok)
	}
	if len(att.Rows) != 3 {
		t.Fatalf("read %d rows, want 3", len(att.Rows))
	}
	if len(att.Rows[0].Fields) != 5 {
		t.Errorf("a row holds %d fields, want a time tag and four states", len(att.Rows[0].Fields))
	}

	// The rows carry relative time tags, resolved against EPOCH_TZERO.
	if !att.Rows[0].IsRelative() {
		t.Error("the first row was read as an absolute time")
	}
	at, err := att.Rows[1].TimeTag(tzero)
	if err != nil {
		t.Fatalf("TimeTag: %v", err)
	}
	if got := at.Format("2006-01-02T15:04:05.0000"); got != "1998-12-18T14:28:15.3672" {
		t.Errorf("row 2 time = %s, want a quarter second past T-zero", got)
	}
}

func TestDecodeACMManeuver(t *testing.T) {
	m, err := adm.DecodeACM([]byte(acmManeuver))
	if err != nil {
		t.Fatalf("DecodeACM: %v", err)
	}

	// A quaternion and a gyro bias: 4 + 3 states, so 8 fields on a row.
	att := m.Attitudes[0]
	states, ok := att.StateCount()
	if !ok || states != 7 {
		t.Fatalf("StateCount = %d, %v; want 7", states, ok)
	}
	if len(att.Rows[0].Fields) != 8 {
		t.Errorf("a row holds %d fields, want 8", len(att.Rows[0].Fields))
	}
	if len(att.Comments) != 1 {
		t.Errorf("attitude comments = %q, want 1", att.Comments)
	}

	if len(m.Maneuvers) != 1 {
		t.Fatalf("read %d manoeuvre blocks, want 1", len(m.Maneuvers))
	}
	man := m.Maneuvers[0]
	if man.GetOr("MAN_PURPOSE", "") != "MOM_DESAT" {
		t.Errorf("MAN_PURPOSE = %q", man.GetOr("MAN_PURPOSE", ""))
	}
	if man.GetOr("TARGET_MOMENTUM", "") != "1.30 -16.400 -11.350" {
		t.Errorf("TARGET_MOMENTUM = %q", man.GetOr("TARGET_MOMENTUM", ""))
	}

	ad := m.AttitudeDetermination
	if ad == nil {
		t.Fatal("the attitude determination section was not read")
	}
	if len(ad.Sensors) != 4 {
		t.Fatalf("read %d sensor blocks, want 4", len(ad.Sensors))
	}
	if n, _ := ad.Sensors[2].Get("SENSOR_NUMBER"); n != "5" {
		t.Errorf("the third sensor is number %q, want 5", n)
	}
	// Table 5-8's note says the numbers need not be sequential, and this
	// message's are 1, 2, 5, 6.
	if n, _ := ad.Sensors[3].Get("SENSOR_NUMBER"); n != "6" {
		t.Errorf("the fourth sensor is number %q, want 6", n)
	}
}

func TestDecodeACMPhysical(t *testing.T) {
	m, err := adm.DecodeACM([]byte(acmPhysical))
	if err != nil {
		t.Fatalf("DecodeACM: %v", err)
	}

	if m.TimeSystem() != "TAI" {
		t.Errorf("TIME_SYSTEM = %q", m.TimeSystem())
	}
	if m.Physical == nil {
		t.Fatal("the physical characteristics section was not read")
	}
	if len(m.Physical.Fields) != 9 {
		t.Errorf("physical section holds %d keywords, want 9", len(m.Physical.Fields))
	}
	// The centre of pressure is three numbers with a unit suffix, which
	// clause 6.9.1 makes documentation, so the value is carried as written.
	if got := m.Physical.GetOr("CP", ""); got != "0.04 -0.78 -0.023 [m]" {
		t.Errorf("CP = %q", got)
	}
	if len(m.Attitudes) != 0 || len(m.Covariances) != 0 {
		t.Error("data blocks were invented")
	}
}

func TestDecodeACMCovariance(t *testing.T) {
	m, err := adm.DecodeACM([]byte(acmCovariance))
	if err != nil {
		t.Fatalf("DecodeACM: %v", err)
	}

	if len(m.Covariances) != 1 {
		t.Fatalf("read %d covariance blocks, want 1", len(m.Covariances))
	}
	cov := m.Covariances[0]
	if cov.CovarianceType() != "ANGLE_GYROBIAS" {
		t.Errorf("COV_TYPE = %q", cov.CovarianceType())
	}

	// Annex B6 makes ANGLE_GYROBIAS a 6x6, and clause 5.3.7.6 puts only the
	// main diagonal on the line: six numbers after the time tag, not the
	// twenty-one an OCM would write for a full lower triangle.
	elements, ok := cov.CovarianceCount()
	if !ok || elements != 6 {
		t.Fatalf("CovarianceCount = %d, %v; want 6", elements, ok)
	}
	for i, row := range cov.Rows {
		if len(row.Fields) != 7 {
			t.Errorf("row %d holds %d fields, want 7", i+1, len(row.Fields))
		}
	}

	ad := m.AttitudeDetermination
	if ad == nil || len(ad.Sensors) != 3 {
		t.Fatalf("attitude determination = %+v", ad)
	}
	if ad.GetOr("AD_METHOD", "") != "EKF" {
		t.Errorf("AD_METHOD = %q", ad.GetOr("AD_METHOD", ""))
	}
}

func TestACMRoundTrip(t *testing.T) {
	for _, source := range []struct{ name, text string }{
		{"simple", acmSimple},
		{"maneuver", acmManeuver},
		{"physical", acmPhysical},
		{"covariance", acmCovariance},
	} {
		t.Run(source.name, func(t *testing.T) {
			first, err := adm.DecodeACM([]byte(source.text))
			if err != nil {
				t.Fatalf("DecodeACM: %v", err)
			}
			encoded, err := first.Encode()
			if err != nil {
				t.Fatalf("Encode: %v", err)
			}
			second, err := adm.DecodeACM(encoded)
			if err != nil {
				t.Fatalf("DecodeACM on our own output: %v\n%s", err, encoded)
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

// Figure G-7 as the Blue Book prints it does not parse.
//
// Its second sensor block writes SENSOR_USED_ with a trailing underscore.
// Table 5-8 lists SENSOR_USED, and clause 5.3.9.1's "only those keywords shown
// in table 5-8 shall be used" leaves no room for a variant spelling.
func TestSensorKeywordTypoIsRefused(t *testing.T) {
	asPrinted := strings.Replace(acmManeuver,
		"SENSOR_NUMBER     = 2\nSENSOR_USED       = AST",
		"SENSOR_NUMBER     = 2\nSENSOR_USED_      = AST", 1)

	if _, err := adm.DecodeACM([]byte(asPrinted)); !errors.Is(err, adm.ErrUnknownKeyword) {
		t.Errorf("DecodeACM = %v, want %v", err, adm.ErrUnknownKeyword)
	}
}

// NUMBER_STATES says how wide a row is, and so do ATT_TYPE and RATE_TYPE. A
// message where the two disagree is one where a producer and a consumer would
// read different columns, so it is refused rather than resolved either way.
func TestStateCountMustAgreeWithTheTypes(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  error
	}{
		{
			name:  "NUMBER_STATES too small for the types",
			input: strings.Replace(acmManeuver, "NUMBER_STATES = 7", "NUMBER_STATES = 4", 1),
			want:  adm.ErrStateCountMismatch,
		},
		{
			name:  "NUMBER_STATES too large for the types",
			input: strings.Replace(acmSimple, "NUMBER_STATES = 4", "NUMBER_STATES = 7", 1),
			want:  adm.ErrStateCountMismatch,
		},
		{
			name:  "a row narrower than the states declared",
			input: strings.Replace(acmSimple, "0.25    0.73529       -0.50531          0.41375         0.181158", "0.25 0.73529 -0.50531 0.41375", 1),
			want:  adm.ErrAttitudeLineFields,
		},
		{
			name:  "an ATT_TYPE annex B4 does not define",
			input: strings.Replace(acmSimple, "ATT_TYPE = QUATERNION", "ATT_TYPE = MRP", 1),
			want:  adm.ErrUnknownAttitudeType,
		},
		{
			// Three angles with no rotation sequence do not define a rotation.
			name: "Euler angles with no rotation sequence",
			input: strings.NewReplacer(
				"ATT_TYPE = QUATERNION", "ATT_TYPE = EULER_ANGLES",
				"NUMBER_STATES = 4", "NUMBER_STATES = 3",
				"0.0     0.73566       -0.50547          0.41309         0.180707", "0.0 1.0 2.0 3.0",
				"0.25    0.73529       -0.50531          0.41375         0.181158", "0.25 1.0 2.0 3.0",
				"0.50    0.73492       -0.50515          0.41441         0.181610", "0.50 1.0 2.0 3.0",
			).Replace(acmSimple),
			want: adm.ErrEulerRotSeqMissing,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := adm.DecodeACM([]byte(tt.input)); !errors.Is(err, tt.want) {
				t.Errorf("DecodeACM = %v, want %v", err, tt.want)
			}
		})
	}
}

func TestDecodeACMRejects(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  error
	}{
		{
			name:  "another message type",
			input: strings.Replace(acmSimple, "CCSDS_ACM_VERS", "CCSDS_APM_VERS", 1),
			want:  ndm.ErrWrongMessageType,
		},
		{
			name:  "no EPOCH_TZERO",
			input: strings.Replace(acmSimple, "EPOCH_TZERO     = 1998-12-18T14:28:15.1172\n", "", 1),
			want:  adm.ErrMissingKeyword,
		},
		{
			// Clauses 5.3.3.5 and 5.3.4.1 fix the keyword order.
			name: "metadata keywords out of the table's order",
			input: strings.Replace(acmSimple,
				"TIME_SYSTEM     = UTC\nEPOCH_TZERO     = 1998-12-18T14:28:15.1172",
				"EPOCH_TZERO     = 1998-12-18T14:28:15.1172\nTIME_SYSTEM     = UTC", 1),
			want: adm.ErrKeywordsOutOfOrder,
		},
		{
			name:  "a covariance keyword in an attitude block",
			input: strings.Replace(acmSimple, "REF_FRAME_A    = J2000", "COV_BASIS = PREDICTED", 1),
			want:  adm.ErrUnknownKeyword,
		},
		{
			name:  "the same keyword twice in one section",
			input: strings.Replace(acmSimple, "ATT_TYPE = QUATERNION", "ATT_TYPE = QUATERNION\nATT_TYPE = DCM", 1),
			want:  adm.ErrDuplicateKeyword,
		},
		{
			// Clause 5.3.1.2 makes table 5-1's section order mandatory.
			name: "sections out of table 5-1's order",
			input: strings.Replace(acmSimple, "ATT_START",
				"PHYS_START\nWET_MASS = 100.0\nPHYS_STOP\nATT_START", 1),
			want: adm.ErrSectionsOutOfOrder,
		},
		{
			name:  "two metadata sections",
			input: strings.Replace(acmSimple, "ATT_START", "META_START\nOBJECT_NAME = A\nMETA_STOP\nATT_START", 1),
			want:  adm.ErrDuplicateSection,
		},
		{
			name:  "a section that is never closed",
			input: strings.Replace(acmSimple, "ATT_STOP\n", "", 1),
			want:  adm.ErrUnterminatedBlock,
		},
		{
			name:  "a sensor block outside the attitude determination section",
			input: strings.Replace(acmSimple, "ATT_TYPE = QUATERNION", "ATT_TYPE = QUATERNION\nSENSOR_START\nSENSOR_NUMBER = 1\nSENSOR_STOP", 1),
			want:  adm.ErrUnexpectedDelimiter,
		},
		{
			name:  "relative and absolute time tags in one block",
			input: strings.Replace(acmSimple, "0.50    0.73492", "1998-12-18T14:28:16Z 0.73492", 1),
			want:  adm.ErrMixedTimeTags,
		},
		{
			name:  "duplicate time tags in one block",
			input: strings.Replace(acmSimple, "0.50    0.73492", "0.25    0.73492", 1),
			want:  adm.ErrDuplicateTimeTag,
		},
		{
			// Clause 5.3.7.5, which binds the covariance section alone.
			name:  "covariance rows that go backwards",
			input: strings.Replace(acmCovariance, "59.896697 6.74E-11", "0.5 6.74E-11", 1),
			want:  adm.ErrTimeTagsOutOfOrder,
		},
		{
			name:  "a COV_TYPE annex B6 does not define",
			input: strings.Replace(acmCovariance, "COV_TYPE      = ANGLE_GYROBIAS", "COV_TYPE      = ANGLE_MRP", 1),
			want:  adm.ErrUnknownCovarianceType,
		},
		{
			name:  "a covariance row that is not the matrix diagonal",
			input: strings.Replace(acmCovariance, "COV_TYPE      = ANGLE_GYROBIAS", "COV_TYPE      = ANGLE", 1),
			want:  adm.ErrCovarianceLineFields,
		},
		{
			name:  "an AD_METHOD annex B5 does not enumerate",
			input: strings.Replace(acmCovariance, "AD_METHOD        = EKF", "AD_METHOD        = UKF", 1),
			want:  adm.ErrUnknownEstimator,
		},
		{
			name:  "two sensor blocks with the same number",
			input: strings.Replace(acmCovariance, "SENSOR_NUMBER    = 4", "SENSOR_NUMBER    = 2", 1),
			want:  adm.ErrDuplicateSensorNumber,
		},
		{
			// Table 5-7: give one of them, not both.
			name:  "a manoeuvre with both an end time and a duration",
			input: strings.Replace(acmManeuver, "MAN_DURATION     = 450.0", "MAN_END_TIME     = 550.0\nMAN_DURATION     = 450.0", 1),
			want:  adm.ErrBothManeuverEnds,
		},
		{
			// Table 5-7: the frame is conditional on the vector.
			name:  "a target momentum with no frame",
			input: strings.Replace(acmManeuver, "TARGET_MOM_FRAME = J2000\n", "", 1),
			want:  adm.ErrMissingFrame,
		},
		{
			// Table 5-5: likewise for the centre of pressure.
			name:  "a centre of pressure with no frame",
			input: strings.Replace(acmPhysical, "CP_REF_FRAME        = SC_BODY_1\n", "", 1),
			want:  adm.ErrMissingFrame,
		},
		{
			name:  "a centre of pressure with two components",
			input: strings.Replace(acmPhysical, "CP                  = 0.04 -0.78 -0.023 [m]", "CP                  = 0.04 -0.78 [m]", 1),
			want:  adm.ErrVectorWidth,
		},
		{
			name:  "a target momentum with four components",
			input: strings.Replace(acmManeuver, "TARGET_MOMENTUM = 1.30 -16.400 -11.350", "TARGET_MOMENTUM = 1.30 -16.400 -11.350 0.0", 1),
			want:  adm.ErrVectorWidth,
		},
		{
			name:  "a data row outside any section",
			input: strings.Replace(acmSimple, "ATT_START\n", "", 1),
			want:  adm.ErrUnexpectedDelimiter,
		},
		{
			name:  "an empty user-defined section",
			input: acmSimple + "USER_START\nUSER_STOP\n",
			want:  adm.ErrMissingKeyword,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := adm.DecodeACM([]byte(tt.input)); !errors.Is(err, tt.want) {
				t.Errorf("DecodeACM = %v, want %v", err, tt.want)
			}
		})
	}
}

// A sensor's noise vector has to hold as many numbers as the keyword beside it
// declares. They are two separate keywords, so nothing but this stops them
// disagreeing (table 5-8).
func TestSensorNoiseCount(t *testing.T) {
	withNoise := strings.Replace(acmCovariance,
		"SENSOR_NUMBER    = 2\nSENSOR_USED      = AST",
		"SENSOR_NUMBER    = 2\nSENSOR_USED      = AST\n"+
			"NUMBER_SENSOR_NOISE_COVARIANCE = 3\nSENSOR_NOISE_STDDEV = 0.003 0.003 0.010 [deg]", 1)

	if _, err := adm.DecodeACM([]byte(withNoise)); err != nil {
		t.Fatalf("DecodeACM: %v", err)
	}

	mismatched := strings.Replace(withNoise,
		"NUMBER_SENSOR_NOISE_COVARIANCE = 3", "NUMBER_SENSOR_NOISE_COVARIANCE = 2", 1)
	if _, err := adm.DecodeACM([]byte(mismatched)); !errors.Is(err, adm.ErrSensorNoiseCount) {
		t.Errorf("DecodeACM = %v, want %v", err, adm.ErrSensorNoiseCount)
	}
}
