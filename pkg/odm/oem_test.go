package odm_test

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/ravisuhag/astro/pkg/odm"
)

// Figure G-11 of CCSDS 502.0-B-3, transcribed with the omitted records left
// out as the document leaves them out. Two metadata groups, which clause
// 5.2.4.6 makes a statement rather than a convenience: a consumer must not
// interpolate across the boundary, and the second block's comment says why —
// a trajectory correction manoeuvre sits between them.
const figureG11 = `CCSDS_OEM_VERS = 3.0
CREATION_DATE = 1996-11-04T17:22:31
ORIGINATOR = NASA/JPL

META_START
OBJECT_NAME         = MARS GLOBAL SURVEYOR
OBJECT_ID           = 1996-062A
CENTER_NAME         = MARS BARYCENTER
REF_FRAME           = EME2000
TIME_SYSTEM         = UTC
START_TIME          = 2019-12-18T12:00:00.331
USEABLE_START_TIME = 2019-12-18T12:10:00.331
USEABLE_STOP_TIME   = 2019-12-28T21:23:00.331
STOP_TIME           = 2019-12-28T21:28:00.331
INTERPOLATION       = HERMITE
INTERPOLATION_DEGREE = 7
META_STOP
COMMENT This file was produced by M.R. Pigs, OSAR NAV/JPL, 2019NOV 04. It is
COMMENT to be used for DSN scheduling purposes only.

2019-12-18T12:00:00.331   2789.619 -280.045 -1746.755    4.73372 -2.49586 -1.04195
2019-12-18T12:01:00.331   2783.419 -308.143 -1877.071    5.18604 -2.42124 -1.99608
2019-12-18T12:02:00.331   2776.033 -336.859 -2008.682    5.63678 -2.33951 -1.94687
2019-12-28T21:28:00.331 -3881.024 563.959 -682.773     -3.28827 -3.66735 1.63861


META_START
OBJECT_NAME          = MARS GLOBAL SURVEYOR
OBJECT_ID            = 1996-062A
CENTER_NAME          = MARS BARYCENTER
REF_FRAME            = EME2000
TIME_SYSTEM          = UTC
START_TIME           = 2019-12-28T21:29:07.267
USEABLE_START_TIME   = 2019-12-28T22:08:02.5
USEABLE_STOP_TIME    = 2019-12-30T01:18:02.5
STOP_TIME            = 2019-12-30T01:28:02.267
INTERPOLATION        = HERMITE
INTERPOLATION_DEGREE = 7
META_STOP

COMMENT   This block begins after trajectory correction maneuver TCM-3.

2019-12-28T21:29:07.267 -2432.166 -063.042 1742.754     7.33702 -3.495867 -1.041945
2019-12-30T01:28:02.267 2164.375 1115.811 -688.131     -3.53328 -2.88452 0.88535
`

func TestDecodeOEMWithTwoBlocks(t *testing.T) {
	m, err := odm.DecodeOEM([]byte(figureG11))
	if err != nil {
		t.Fatalf("DecodeOEM: %v", err)
	}

	if m.Header.Originator != "NASA/JPL" {
		t.Errorf("Originator = %q, want NASA/JPL", m.Header.Originator)
	}
	if len(m.Blocks) != 2 {
		t.Fatalf("read %d blocks, want 2", len(m.Blocks))
	}

	first := m.Blocks[0]
	md := first.Metadata
	if md.ObjectName != "MARS GLOBAL SURVEYOR" || md.ObjectID != "1996-062A" {
		t.Errorf("object = %q / %q", md.ObjectName, md.ObjectID)
	}
	// CENTER_NAME here is two words, so the blank-collapsing rule of
	// clause 7.5.9 has to leave the single blank alone.
	if md.CenterName != "MARS BARYCENTER" {
		t.Errorf("CenterName = %q, want MARS BARYCENTER", md.CenterName)
	}
	if md.Interpolation != "HERMITE" || md.InterpolationDegree != 7 {
		t.Errorf("interpolation = %q degree %d", md.Interpolation, md.InterpolationDegree)
	}

	wantStart := time.Date(2019, 12, 18, 12, 0, 0, 331000000, time.UTC)
	if !md.StartTime.Equal(wantStart) {
		t.Errorf("StartTime = %v, want %v", md.StartTime, wantStart)
	}
	if md.UseableStartTime == nil || md.UseableStopTime == nil {
		t.Fatal("the useable span was not read")
	}
	wantUseable := time.Date(2019, 12, 18, 12, 10, 0, 331000000, time.UTC)
	if !md.UseableStartTime.Equal(wantUseable) {
		t.Errorf("UseableStartTime = %v, want %v", md.UseableStartTime, wantUseable)
	}

	// The comments follow META_STOP, so they head the ephemeris data section
	// rather than the metadata group (clause 7.8.9).
	if len(md.Comments) != 0 {
		t.Errorf("metadata comments = %q, want none", md.Comments)
	}
	if len(first.Comments) != 2 {
		t.Errorf("ephemeris comments = %q, want 2", first.Comments)
	}

	if len(first.Lines) != 4 {
		t.Fatalf("read %d ephemeris lines, want 4", len(first.Lines))
	}
	line := first.Lines[0]
	if line.X != 2789.619 || line.Y != -280.045 || line.Z != -1746.755 {
		t.Errorf("first position = %v %v %v", line.X, line.Y, line.Z)
	}
	if line.XDot != 4.73372 || line.YDot != -2.49586 || line.ZDot != -1.04195 {
		t.Errorf("first velocity = %v %v %v", line.XDot, line.YDot, line.ZDot)
	}
	if line.HasAcceleration {
		t.Error("acceleration was read from a 7-field line")
	}

	// Leading zeroes are legal in a fixed-point value (clause 7.5.6), so
	// -063.042 in the second block is -63.042.
	second := m.Blocks[1]
	if got := second.Lines[0].Y; got != -63.042 {
		t.Errorf("second block Y = %v, want -63.042", got)
	}
	if len(second.Comments) != 1 {
		t.Errorf("second block comments = %q, want 1", second.Comments)
	}

	if got := m.Records(); got != 6 {
		t.Errorf("Records = %d, want 6", got)
	}
	start, stop := m.Span()
	if !start.Equal(wantStart) {
		t.Errorf("Span start = %v, want %v", start, wantStart)
	}
	wantStop := time.Date(2019, 12, 30, 1, 28, 2, 267000000, time.UTC)
	if !stop.Equal(wantStop) {
		t.Errorf("Span stop = %v, want %v", stop, wantStop)
	}
}

// Figure G-12: the same shape with the optional acceleration terms, which
// clause 5.2.4.2 allows and which turn a 7-field row into a 10-field one.
const figureG12 = `CCSDS_OEM_VERS = 3.0

COMMENT   OEM WITH OPTIONAL ACCELERATIONS

CREATION_DATE = 2019-11-04T17:22:31
ORIGINATOR = NASA/JPL

META_START
OBJECT_NAME         = MARS GLOBAL SURVEYOR
OBJECT_ID           = 1996-028A
CENTER_NAME         = MARS BARYCENTER
REF_FRAME           = EME2000
TIME_SYSTEM         = UTC
START_TIME          = 2019-12-18T12:00:00.331
STOP_TIME           = 2019-12-28T21:28:00.331
INTERPOLATION       = HERMITE
INTERPOLATION_DEGREE = 7
META_STOP

2019-12-18T12:00:00.331   2789.6 -280.0 -1746.8   4.73 -2.50 -1.04    0.008 0.001 -0.159
2019-12-18T12:01:00.331   2783.4 -308.1 -1877.1   5.19 -2.42 -2.00    0.008 0.001 0.001
`

func TestDecodeOEMWithAcceleration(t *testing.T) {
	m, err := odm.DecodeOEM([]byte(figureG12))
	if err != nil {
		t.Fatalf("DecodeOEM: %v", err)
	}

	// This example's comment sits between the version keyword and
	// CREATION_DATE, so it is a header comment.
	if len(m.Header.Comments) != 1 {
		t.Errorf("header comments = %q, want 1", m.Header.Comments)
	}

	line := m.Blocks[0].Lines[0]
	if !line.HasAcceleration {
		t.Fatal("acceleration was not read from a 10-field line")
	}
	if line.XDDot != 0.008 || line.YDDot != 0.001 || line.ZDDot != -0.159 {
		t.Errorf("acceleration = %v %v %v", line.XDDot, line.YDDot, line.ZDDot)
	}
}

// Figure G-13: the covariance section. Two matrices, each 21 values spread
// over six lines, delimited by COVARIANCE_START and COVARIANCE_STOP — not the
// COV_START that clause 7.8.9 calls it.
const figureG13Covariance = `CCSDS_OEM_VERS = 3.0
CREATION_DATE = 1996-11-04T17:22:31
ORIGINATOR = NASA/JPL

META_START
OBJECT_NAME = MARS GLOBAL SURVEYOR
OBJECT_ID = 1996-062A
CENTER_NAME = MARS BARYCENTER
REF_FRAME = EME2000
TIME_SYSTEM = UTC
START_TIME = 2019-12-28T21:29:07.267
STOP_TIME = 2019-12-30T01:28:02.267
META_STOP

2019-12-28T21:29:07.267 -2432.166 -63.042 1742.754 7.33702 -3.495867 -1.041945

COVARIANCE_START
EPOCH = 2019-12-28T21:29:07.267
COV_REF_FRAME = EME2000
 3.3313494e-04
 4.6189273e-04 6.7824216e-04
-3.0700078e-04 -4.2212341e-04 3.2319319e-04
-3.3493650e-07 -4.6860842e-07 2.4849495e-07       4.2960228e-10
-2.2118325e-07 -2.8641868e-07 1.7980986e-07       2.6088992e-10   1.7675147e-10
-3.0413460e-07 -4.9894969e-07 3.5403109e-07       1.8692631e-10   1.0088625e-10   6.2244443e-10

EPOCH = 2019-12-29T21:00:00
COV_REF_FRAME = EME2000
 3.4424505e-04
 4.5078162e-04 6.8935327e-04
-3.0600067e-04 -4.1101230e-04   3.3420420e-04
-3.2382549e-07 -4.5750731e-07   2.3738384e-07     4.3071339e-10
-2.1007214e-07 -2.7530757e-07   1.6870875e-07     2.5077881e-10   1.8786258e-10
-3.0302350e-07 -4.8783858e-07   3.4302008e-07     1.7581520e-10   1.0077514e-10   6.2244443e-10
COVARIANCE_STOP
`

func TestDecodeOEMWithCovariance(t *testing.T) {
	m, err := odm.DecodeOEM([]byte(figureG13Covariance))
	if err != nil {
		t.Fatalf("DecodeOEM: %v", err)
	}

	block := m.Blocks[0]
	if len(block.Covariances) != 2 {
		t.Fatalf("read %d covariance matrices, want 2", len(block.Covariances))
	}

	first := block.Covariances[0]
	if first.RefFrame != "EME2000" {
		t.Errorf("COV_REF_FRAME = %q, want EME2000", first.RefFrame)
	}
	wantEpoch := time.Date(2019, 12, 28, 21, 29, 7, 267000000, time.UTC)
	if !first.Epoch.Equal(wantEpoch) {
		t.Errorf("epoch = %v, want %v", first.Epoch, wantEpoch)
	}

	// Clause 5.2.5.4 puts the values in from [1,1] to [6,6], row by row.
	if first.Matrix[0][0] != 3.3313494e-04 {
		t.Errorf("[1,1] = %v, want 3.3313494e-04", first.Matrix[0][0])
	}
	if first.Matrix[1][0] != 4.6189273e-04 {
		t.Errorf("[2,1] = %v, want 4.6189273e-04", first.Matrix[1][0])
	}
	if first.Matrix[5][5] != 6.2244443e-10 {
		t.Errorf("[6,6] = %v, want 6.2244443e-10", first.Matrix[5][5])
	}
	// Symmetric: the upper triangle is filled in on decode.
	if first.Matrix[0][1] != first.Matrix[1][0] {
		t.Errorf("matrix is not symmetric: [1,2] = %v, [2,1] = %v",
			first.Matrix[0][1], first.Matrix[1][0])
	}

	if second := block.Covariances[1]; second.Matrix[0][0] != 3.4424505e-04 {
		t.Errorf("second matrix [1,1] = %v, want 3.4424505e-04", second.Matrix[0][0])
	}
}

func TestOEMRoundTrip(t *testing.T) {
	for name, input := range map[string]string{
		"figure G-11": figureG11,
		"figure G-12": figureG12,
		"figure G-13": figureG13Covariance,
	} {
		t.Run(name, func(t *testing.T) {
			first, err := odm.DecodeOEM([]byte(input))
			if err != nil {
				t.Fatalf("DecodeOEM: %v", err)
			}
			encoded, err := first.Encode()
			if err != nil {
				t.Fatalf("Encode: %v", err)
			}
			second, err := odm.DecodeOEM(encoded)
			if err != nil {
				t.Fatalf("DecodeOEM on our own output: %v\n%s", err, encoded)
			}

			if len(second.Blocks) != len(first.Blocks) {
				t.Fatalf("block count changed: %d then %d", len(first.Blocks), len(second.Blocks))
			}
			if second.Records() != first.Records() {
				t.Errorf("record count changed: %d then %d", first.Records(), second.Records())
			}
			for i := range first.Blocks {
				a, b := first.Blocks[i], second.Blocks[i]
				if len(a.Covariances) != len(b.Covariances) {
					t.Errorf("block %d covariance count changed: %d then %d",
						i, len(a.Covariances), len(b.Covariances))
				}
				for j := range a.Lines {
					if a.Lines[j] != b.Lines[j] {
						t.Errorf("block %d line %d changed:\n\t%+v\n\t%+v", i, j, a.Lines[j], b.Lines[j])
					}
				}
			}
		})
	}
}

func TestDecodeOEMRejects(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  error
	}{
		{
			name:  "an interpolation method with no degree",
			input: strings.Replace(figureG12, "INTERPOLATION_DEGREE = 7\n", "", 1),
			want:  odm.ErrInterpolationDegreeMissing,
		},
		{
			name:  "a time system that changes between blocks",
			input: strings.Replace(figureG11, "TIME_SYSTEM          = UTC", "TIME_SYSTEM          = TAI", 1),
			want:  odm.ErrTimeSystemChanged,
		},
		{
			name:  "an ephemeris row with eight fields",
			input: figureG12 + "2019-12-18T12:03:00.331 1.0 2.0 3.0 4.0 5.0 6.0 7.0\n",
			want:  odm.ErrEphemerisLineFields,
		},
		{
			name:  "a metadata group that is never closed",
			input: "CCSDS_OEM_VERS = 3.0\nCREATION_DATE = 1996-11-04T17:22:31\nORIGINATOR = X\nMETA_START\nOBJECT_NAME = A\n",
			want:  odm.ErrUnterminatedBlock,
		},
		{
			name:  "no metadata group at all",
			input: "CCSDS_OEM_VERS = 3.0\nCREATION_DATE = 1996-11-04T17:22:31\nORIGINATOR = X\n",
			want:  odm.ErrNoEphemerisBlock,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := odm.DecodeOEM([]byte(tt.input)); !errors.Is(err, tt.want) {
				t.Errorf("DecodeOEM = %v, want %v", err, tt.want)
			}
		})
	}
}

// Clause 5.2.5.7 requires covariance matrices ordered by increasing epoch.
func TestOEMCovarianceOrdering(t *testing.T) {
	swapped := strings.Replace(figureG13Covariance,
		"EPOCH = 2019-12-29T21:00:00", "EPOCH = 2019-12-27T21:00:00", 1)

	if _, err := odm.DecodeOEM([]byte(swapped)); !errors.Is(err, odm.ErrCovarianceOutOfOrder) {
		t.Errorf("DecodeOEM = %v, want ErrCovarianceOutOfOrder", err)
	}
}
