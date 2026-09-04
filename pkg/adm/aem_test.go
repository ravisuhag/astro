package adm_test

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/ravisuhag/astro/pkg/adm"
)

// Figure G-4 of CCSDS 504.0-B-2, with the omitted records left out. Two
// metadata groups, the second beginning after a trajectory correction
// manoeuvre.
//
// Note the second group's OBJECT_NAME is lower case where the first is upper.
// The document writes it that way, and clause 6.5 makes case insignificant
// only in comments and free text, so both are carried as written.
const figureG4 = `CCSDS_AEM_VERS = 2.0
CREATION_DATE = 2002-11-04T17:22:31
ORIGINATOR = NASA/JPL
MESSAGE_ID = A7015Z3

META_START
COMMENT This file was produced by M.R. Somebody, MSOO NAV/JPL.
COMMENT It is to be used for attitude reconstruction only.
OBJECT_NAME     = MARS GLOBAL SURVEYOR
OBJECT_ID      = 1996-062A
CENTER_NAME     = MARS BARYCENTER
REF_FRAME_A     = EME2000
REF_FRAME_B     = SC_BODY_1
TIME_SYSTEM     = UTC
START_TIME      = 1996-11-28T21:29:07.2555
USEABLE_START_TIME = 1996-11-28T22:08:02.5555
USEABLE_STOP_TIME = 1996-11-30T01:18:02.5555
STOP_TIME      = 1996-11-30T01:28:02.5555
ATTITUDE_TYPE    = QUATERNION
INTERPOLATION_METHOD = hermite
INTERPOLATION_DEGREE = 7
META_STOP

DATA_START
1996-11-28T21:29:07.2555 0.56748 0.03146 0.45689 0.68427
1996-11-28T22:08:03.5555 0.42319 -0.45697 0.23784 0.74533
1996-11-30T01:28:02.5555 0.74563 -0.45375 0.36875 0.31964
DATA_STOP

META_START
COMMENT This block begins after trajectory correction maneuver TCM-3.
OBJECT_NAME     = mars global surveyor
OBJECT_ID      = 1996-062A
CENTER_NAME     = MARS BARYCENTER
REF_FRAME_A     = EME2000
REF_FRAME_B       = SC_BODY_1
TIME_SYSTEM     = UTC
START_TIME      = 1996-12-18T12:05:00.5555
STOP_TIME      = 1996-12-28T21:28:00.5555
ATTITUDE_TYPE    = QUATERNION
META_STOP

DATA_START
1996-12-18T12:05:00.5555 0.72501 -0.64585 0.018542 0.23854
DATA_STOP
`

func TestDecodeAEM(t *testing.T) {
	m, err := adm.DecodeAEM([]byte(figureG4))
	if err != nil {
		t.Fatalf("DecodeAEM: %v", err)
	}

	if m.Header.Originator != "NASA/JPL" || m.Header.MessageID != "A7015Z3" {
		t.Errorf("header = %+v", m.Header)
	}
	if len(m.Blocks) != 2 {
		t.Fatalf("read %d blocks, want 2", len(m.Blocks))
	}

	first := m.Blocks[0]
	md := first.Metadata
	if md.ObjectName != "MARS GLOBAL SURVEYOR" || md.CenterName != "MARS BARYCENTER" {
		t.Errorf("metadata = %+v", md)
	}
	if md.FrameA != "EME2000" || md.FrameB != "SC_BODY_1" {
		t.Errorf("frames = %q to %q", md.FrameA, md.FrameB)
	}
	if md.Type != adm.Quaternion4 {
		t.Errorf("ATTITUDE_TYPE = %q, want QUATERNION", md.Type)
	}
	// The document writes the method in lower case, which is legal.
	if md.InterpolationMethod != "hermite" || md.InterpolationDegree != 7 {
		t.Errorf("interpolation = %q degree %d", md.InterpolationMethod, md.InterpolationDegree)
	}

	wantStart := time.Date(1996, 11, 28, 21, 29, 7, 255500000, time.UTC)
	if !md.StartTime.Equal(wantStart) {
		t.Errorf("StartTime = %v, want %v", md.StartTime, wantStart)
	}
	if md.UseableStartTime == nil || md.UseableStopTime == nil {
		t.Error("the useable span was not read")
	}

	if len(first.Lines) != 3 {
		t.Fatalf("read %d lines, want 3", len(first.Lines))
	}
	line := first.Lines[0]
	// QUATERNION means four values after the epoch, in the order Q1 Q2 Q3 QC.
	if len(line.Values) != 4 {
		t.Fatalf("read %d values, want 4", len(line.Values))
	}
	if line.Values[0] != 0.56748 || line.Values[3] != 0.68427 {
		t.Errorf("values = %v", line.Values)
	}
	if !line.Epoch.Equal(wantStart) {
		t.Errorf("first epoch = %v, want %v", line.Epoch, wantStart)
	}

	// The second group writes the object name in lower case. Clause 6.5 makes
	// case insignificant only in comments and free text, so it is carried.
	if got := m.Blocks[1].Metadata.ObjectName; got != "mars global surveyor" {
		t.Errorf("second block object name = %q", got)
	}
	if got := m.Records(); got != 4 {
		t.Errorf("Records = %d, want 4", got)
	}
}

// Table 4-4 fixes the width of a data line per attitude type, and nothing in
// the line says which type it is. This is the AEM's central trap.
func TestAEMLineWidthComesFromTheType(t *testing.T) {
	tests := []struct {
		attitudeType adm.AttitudeType
		fields       int
	}{
		{adm.Quaternion4, 4},
		{adm.QuaternionDerivative, 8},
		{adm.QuaternionAngVel, 7},
		{adm.EulerAngle, 3},
		{adm.EulerAngleDerivative, 6},
		{adm.EulerAngleAngVel, 6},
		{adm.SpinType, 4},
		{adm.SpinNutation, 7},
		{adm.SpinNutationMomentum, 7},
	}

	for _, tt := range tests {
		t.Run(string(tt.attitudeType), func(t *testing.T) {
			if got, ok := tt.attitudeType.Fields(); !ok || got != tt.fields {
				t.Fatalf("Fields = %d, %v, want %d, true", got, ok, tt.fields)
			}

			values := make([]string, tt.fields)
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

			m, err := adm.DecodeAEM([]byte(input))
			if err != nil {
				t.Fatalf("DecodeAEM: %v", err)
			}
			if got := len(m.Blocks[0].Lines[0].Values); got != tt.fields {
				t.Errorf("read %d values, want %d", got, tt.fields)
			}

			// One value too many must be refused rather than truncated.
			wide := strings.Replace(input,
				strings.Join(values, " "), strings.Join(values, " ")+" 0.5", 1)
			if _, err := adm.DecodeAEM([]byte(wide)); !errors.Is(err, adm.ErrAttitudeLineFields) {
				t.Errorf("a line with one extra value = %v, want ErrAttitudeLineFields", err)
			}
		})
	}
}

func TestAEMRoundTrip(t *testing.T) {
	first, err := adm.DecodeAEM([]byte(figureG4))
	if err != nil {
		t.Fatalf("DecodeAEM: %v", err)
	}
	encoded, err := first.Encode()
	if err != nil {
		t.Fatalf("Encode: %v", err)
	}
	second, err := adm.DecodeAEM(encoded)
	if err != nil {
		t.Fatalf("DecodeAEM on our own output: %v\n%s", err, encoded)
	}

	if len(second.Blocks) != len(first.Blocks) || second.Records() != first.Records() {
		t.Fatalf("shape changed: %d blocks %d records, then %d and %d",
			len(first.Blocks), first.Records(), len(second.Blocks), second.Records())
	}
	for i := range first.Blocks {
		a, b := first.Blocks[i], second.Blocks[i]
		if a.Metadata.Type != b.Metadata.Type {
			t.Errorf("block %d attitude type changed: %q then %q", i, a.Metadata.Type, b.Metadata.Type)
		}
		for j := range a.Lines {
			if !a.Lines[j].Epoch.Equal(b.Lines[j].Epoch) {
				t.Errorf("block %d line %d epoch changed", i, j)
			}
			for k := range a.Lines[j].Values {
				if a.Lines[j].Values[k] != b.Lines[j].Values[k] {
					t.Errorf("block %d line %d value %d changed: %v then %v",
						i, j, k, a.Lines[j].Values[k], b.Lines[j].Values[k])
				}
			}
		}
	}
}

func TestDecodeAEMRejects(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  error
	}{
		{
			name:  "no metadata group",
			input: "CCSDS_AEM_VERS = 2.0\nCREATION_DATE = 2002-11-04T17:22:31\nORIGINATOR = X\n",
			want:  adm.ErrNoSegment,
		},
		{
			name:  "an unknown attitude type",
			input: strings.Replace(figureG4, "ATTITUDE_TYPE    = QUATERNION", "ATTITUDE_TYPE = NOPE", 1),
			want:  adm.ErrUnknownAttitudeType,
		},
		{
			name:  "a data line of the wrong width",
			input: strings.Replace(figureG4, "1996-11-28T21:29:07.2555 0.56748 0.03146 0.45689 0.68427", "1996-11-28T21:29:07.2555 0.56748 0.03146 0.45689", 1),
			want:  adm.ErrAttitudeLineFields,
		},
		{
			name:  "an interpolation method with no degree",
			input: strings.Replace(figureG4, "INTERPOLATION_DEGREE = 7\n", "", 1),
			want:  adm.ErrInterpolationDegreeMissing,
		},
		{
			name:  "a metadata group with no data block",
			input: strings.Replace(figureG4, "DATA_START\n", "", 1),
			want:  adm.ErrMissingDataSection,
		},
		{
			name:  "a metadata group that never closes",
			input: "CCSDS_AEM_VERS = 2.0\nCREATION_DATE = 2002-11-04T17:22:31\nORIGINATOR = X\nMETA_START\nOBJECT_NAME = A\n",
			want:  adm.ErrUnterminatedBlock,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := adm.DecodeAEM([]byte(tt.input)); !errors.Is(err, tt.want) {
				t.Errorf("DecodeAEM = %v, want %v", err, tt.want)
			}
		})
	}
}

// A Euler type without EULER_ROT_SEQ is refused: three angles with no
// rotation sequence do not define a rotation.
//
// The input is built rather than edited from figure G-4, because the width
// check runs per line as the data is read and would fire first on any block
// whose lines still carried four values.
func TestAEMEulerNeedsRotationSequence(t *testing.T) {
	const input = `CCSDS_AEM_VERS = 2.0
CREATION_DATE = 2002-11-04T17:22:31
ORIGINATOR = NASA/JPL

META_START
OBJECT_NAME = MARS GLOBAL SURVEYOR
OBJECT_ID = 1996-062A
REF_FRAME_A = EME2000
REF_FRAME_B = SC_BODY_1
TIME_SYSTEM = UTC
START_TIME = 1996-11-28T21:29:07
STOP_TIME = 1996-11-30T01:28:02
ATTITUDE_TYPE = EULER_ANGLE
META_STOP

DATA_START
1996-11-28T21:29:07 1.0 2.0 3.0
DATA_STOP
`

	if _, err := adm.DecodeAEM([]byte(input)); !errors.Is(err, adm.ErrEulerRotSeqMissing) {
		t.Errorf("DecodeAEM = %v, want ErrEulerRotSeqMissing", err)
	}

	// With the sequence it reads.
	withSeq := strings.Replace(input, "ATTITUDE_TYPE = EULER_ANGLE\n",
		"ATTITUDE_TYPE = EULER_ANGLE\nEULER_ROT_SEQ = ZXZ\n", 1)
	if _, err := adm.DecodeAEM([]byte(withSeq)); err != nil {
		t.Errorf("DecodeAEM with EULER_ROT_SEQ: %v", err)
	}
}
