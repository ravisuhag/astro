package adm_test

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/ravisuhag/astro/pkg/adm"
)

// Figure G-1 of CCSDS 504.0-B-2: the simplest APM, a quaternion and nothing
// else.
const figureG1 = `CCSDS_APM_VERS = 2.0
CREATION_DATE = 2003-09-30T19:23:57
ORIGINATOR = GSFC

OBJECT_NAME   = TRMM
OBJECT_ID     = 1997-062A
CENTER_NAME   = EARTH
TIME_SYSTEM   = UTC

COMMENT       Current attitude for orbit 335
COMMENT       Attitude state quaternion
COMMENT       Accuracy of this attitude is 0.02 deg RSS.

EPOCH     = 2003-09-30T14:28:15.1172

QUAT_START
REF_FRAME_A    = SC_BODY_1
REF_FRAME_B    = ITRF1997

Q1        = 0.00005
Q2        = 0.87543
Q3        = 0.40949
QC        = 0.25678
QUAT_STOP
`

func TestDecodeAPM(t *testing.T) {
	m, err := adm.DecodeAPM([]byte(figureG1))
	if err != nil {
		t.Fatalf("DecodeAPM: %v", err)
	}

	if m.Header.Version != "2.0" || m.Header.Originator != "GSFC" {
		t.Errorf("header = %+v", m.Header)
	}
	md := m.Metadata
	if md.ObjectName != "TRMM" || md.ObjectID != "1997-062A" || md.TimeSystem != "UTC" {
		t.Errorf("metadata = %+v", md)
	}

	wantEpoch := time.Date(2003, 9, 30, 14, 28, 15, 117200000, time.UTC)
	if !m.Epoch.Equal(wantEpoch) {
		t.Errorf("Epoch = %v, want %v", m.Epoch, wantEpoch)
	}
	// The comments sit between TIME_SYSTEM and EPOCH, so they head the data
	// section rather than the metadata.
	if len(m.Comments) != 3 {
		t.Errorf("data comments = %q, want 3", m.Comments)
	}

	q := m.Quaternion
	if q == nil {
		t.Fatal("no quaternion block")
	}
	if q.FrameA != "SC_BODY_1" || q.FrameB != "ITRF1997" {
		t.Errorf("frames = %q to %q", q.FrameA, q.FrameB)
	}
	// The scalar is last on the wire. Reading these four into a slice and
	// handing it to a library that puts the scalar first gives a rotation that
	// is wrong and looks plausible.
	if q.Q1 != 0.00005 || q.Q2 != 0.87543 || q.Q3 != 0.40949 || q.QC != 0.25678 {
		t.Errorf("quaternion = %+v", q.Quaternion)
	}
	if q.HasDerivative {
		t.Error("HasDerivative is true for a block with no derivatives")
	}

	if m.Euler != nil || m.Spin != nil || m.Inertia != nil || len(m.Maneuvers) != 0 {
		t.Error("blocks were read from a message that has none")
	}
}

// Figure G-3, trimmed: Euler angles, inertia and a manoeuvre, with units
// written on most values as clause 6.6 allows.
const figureG3 = `CCSDS_APM_VERS = 2.0
CREATION_DATE = 2006-03-13T13:13:33
ORIGINATOR     = GSFC
MESSAGE_ID     = A7015Z2

OBJECT_NAME   = GOES-P
OBJECT_ID     = 2006-003A
CENTER_NAME   = EARTH
TIME_SYSTEM   = UTC

COMMENT Attitude given by Euler angles

EPOCH = 2006-03-12T09:56:39.4987

EULER_START
COMMENT Euler angles
REF_FRAME_A   = BODY_FRAME_A
REF_FRAME_B   = ITRF1997
EULER_ROT_SEQ = YXY
ANGLE_1    = -26.78 [deg]
ANGLE_2    = 3.71 [deg]
ANGLE_3    = -11.16 [deg]
EULER_STOP

INERTIA_START
INERTIA_REF_FRAME = SC_BODY_1
IXX       = 6080.0 [kg*m**2]
IYY       = 5245.5 [kg*m**2]
IZZ       = 8067.3 [kg*m**2]
IXY       = -135.9 [kg*m**2]
IXZ       = 89.3   [kg*m**2]
IYZ       = -90.7 [kg*m**2]
INERTIA_STOP

MAN_START
COMMENT Data follows for 1 planned maneuver.
COMMENT Impulsive, torque direction fixed in body frame
MAN_EPOCH_START = 2004-02-14T14:29:00.5098
MAN_DURATION = 3     [s]
MAN_REF_FRAME = ICRF
MAN_TOR_X     = -1.25 [N*m]
MAN_TOR_Y     = -0.5   [N*m]
MAN_TOR_Z     = 0.5   [N*m]
MAN_STOP
`

func TestDecodeAPMWithSeveralBlocks(t *testing.T) {
	m, err := adm.DecodeAPM([]byte(figureG3))
	if err != nil {
		t.Fatalf("DecodeAPM: %v", err)
	}

	e := m.Euler
	if e == nil {
		t.Fatal("no Euler block")
	}
	// Without the rotation sequence the three angles are just numbers: YXY
	// and ZXZ describe different orientations from the same values.
	if e.RotSeq != "YXY" {
		t.Errorf("EULER_ROT_SEQ = %q, want YXY", e.RotSeq)
	}
	if e.Angle1 != -26.78 || e.Angle2 != 3.71 || e.Angle3 != -11.16 {
		t.Errorf("angles = %v %v %v", e.Angle1, e.Angle2, e.Angle3)
	}
	if e.HasRates {
		t.Error("HasRates is true for a block with no rates")
	}

	in := m.Inertia
	if in == nil {
		t.Fatal("no inertia block")
	}
	if in.Frame != "SC_BODY_1" || in.IXX != 6080.0 || in.IYZ != -90.7 {
		t.Errorf("inertia = %+v", in.Inertia)
	}

	if len(m.Maneuvers) != 1 {
		t.Fatalf("read %d maneuvers, want 1", len(m.Maneuvers))
	}
	man := m.Maneuvers[0]
	if man.Duration != 3 || man.RefFrame != "ICRF" {
		t.Errorf("maneuver = %+v", man)
	}
	if man.TorqueX != -1.25 || man.TorqueY != -0.5 || man.TorqueZ != 0.5 {
		t.Errorf("torque = %v %v %v", man.TorqueX, man.TorqueY, man.TorqueZ)
	}
	if len(man.Comments) != 2 {
		t.Errorf("maneuver comments = %q, want 2", man.Comments)
	}
}

func TestAPMRoundTrip(t *testing.T) {
	for name, input := range map[string]string{"figure G-1": figureG1, "figure G-3": figureG3} {
		t.Run(name, func(t *testing.T) {
			first, err := adm.DecodeAPM([]byte(input))
			if err != nil {
				t.Fatalf("DecodeAPM: %v", err)
			}
			encoded, err := first.Encode()
			if err != nil {
				t.Fatalf("Encode: %v", err)
			}
			second, err := adm.DecodeAPM(encoded)
			if err != nil {
				t.Fatalf("DecodeAPM on our own output: %v\n%s", err, encoded)
			}

			if !second.Epoch.Equal(first.Epoch) {
				t.Errorf("epoch changed: %v then %v", first.Epoch, second.Epoch)
			}
			if (second.Quaternion == nil) != (first.Quaternion == nil) {
				t.Error("the quaternion block appeared or vanished")
			}
			if first.Quaternion != nil && second.Quaternion.Quaternion != first.Quaternion.Quaternion {
				t.Errorf("quaternion changed: %+v then %+v",
					first.Quaternion.Quaternion, second.Quaternion.Quaternion)
			}
			if first.Euler != nil && second.Euler.EulerAngles != first.Euler.EulerAngles {
				t.Errorf("Euler angles changed: %+v then %+v",
					first.Euler.EulerAngles, second.Euler.EulerAngles)
			}
			if len(second.Maneuvers) != len(first.Maneuvers) {
				t.Errorf("maneuver count changed: %d then %d",
					len(first.Maneuvers), len(second.Maneuvers))
			}
		})
	}
}

func TestDecodeAPMRejects(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  error
	}{
		{
			name:  "no attitude at all",
			input: strings.Split(figureG1, "QUAT_START")[0],
			want:  adm.ErrNoAttitude,
		},
		{
			name:  "a block closed by the wrong keyword",
			input: strings.Replace(figureG1, "QUAT_STOP", "EULER_STOP", 1),
			want:  adm.ErrUnexpectedDelimiter,
		},
		{
			name:  "a block that never closes",
			input: strings.Replace(figureG1, "QUAT_STOP\n", "", 1),
			want:  adm.ErrUnterminatedBlock,
		},
		{
			name:  "a nested block",
			input: strings.Replace(figureG1, "Q1        = 0.00005", "EULER_START", 1),
			want:  adm.ErrUnexpectedDelimiter,
		},
		{
			name:  "a stop with no start",
			input: figureG1 + "SPIN_STOP\n",
			want:  adm.ErrUnexpectedDelimiter,
		},
		{
			name:  "a keyword in the wrong block",
			input: strings.Replace(figureG1, "Q1        = 0.00005", "IXX = 1.0", 1),
			want:  adm.ErrUnknownKeyword,
		},
		{
			name:  "an incomplete quaternion",
			input: strings.Replace(figureG1, "QC        = 0.25678\n", "", 1),
			want:  adm.ErrMissingKeyword,
		},
		{
			name:  "Euler angles with no rotation sequence",
			input: strings.Replace(figureG3, "EULER_ROT_SEQ = YXY\n", "", 1),
			want:  adm.ErrMissingKeyword,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := adm.DecodeAPM([]byte(tt.input)); !errors.Is(err, tt.want) {
				t.Errorf("DecodeAPM = %v, want %v", err, tt.want)
			}
		})
	}
}
