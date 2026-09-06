package ndm_test

import (
	"strings"
	"testing"
	"time"

	"github.com/ravisuhag/astro/pkg/adm"
	"github.com/ravisuhag/astro/pkg/cdm"
	"github.com/ravisuhag/astro/pkg/odm"
)

// TestEpochUnitsAgreeAcrossMessageTypes is the regression test for a drift
// that existed between pkg/odm, pkg/adm and pkg/cdm: whether an epoch value
// may carry a unit suffix such as "[s]".
//
// CCSDS 502.0-B-3 clause 7.5.10 spells out the epoch grammar
// (YYYY-MM-DDThh:mm:ss[.d→d][Z], or the ordinal form) with no unit suffix in
// it. Clause 7.7.1.1's unit allowance is scoped to "OPM/OMM UNITS" and
// requires the suffix to "exactly match the units ... as specified in tables
// 3-3 and 4-3" — and in both tables the EPOCH row's Units column is blank, so
// there is no unit text a suffix on an epoch could ever match. So an epoch
// with a unit suffix is not valid CCSDS input, and every message type should
// refuse it the same way.
//
// Before this package's helpers were consolidated into internal/ndm,
// pkg/odm rejected a unit suffix on an epoch (it called ndm.ParseEpoch
// directly) while pkg/adm and pkg/cdm both stripped the suffix first (they
// called ndm.SplitUnits before ndm.ParseEpoch) and so silently accepted it.
// This test decodes the same epoch line, with the same suffix, through all
// three message types and requires them to agree.
//
// The field checked for each message type is one that goes through the
// package's own epoch-parsing code rather than internal/ndm's shared header
// reader (which has always called ParseEpoch directly for CREATION_DATE, in
// every package, so it would not show the drift): OPM's EPOCH, APM's EPOCH,
// and CDM's TCA.
func TestEpochUnitsAgreeAcrossMessageTypes(t *testing.T) {
	const suffixedEpoch = "2022-12-18T14:28:15.1172Z [s]"

	opmSrc := strings.Replace(figureG1OPMForEpochTest, "2022-12-18T14:28:15.1172", suffixedEpoch, 1)
	apmSrc := strings.Replace(figureG1APMForEpochTest, "2003-09-30T14:28:15.1172", suffixedEpoch, 1)
	cdmSrc := strings.Replace(clause362CDMForEpochTest, "2010-03-13T22:37:52.618", suffixedEpoch, 1)

	_, odmErr := odm.DecodeOPM([]byte(opmSrc))
	_, admErr := adm.DecodeAPM([]byte(apmSrc))
	_, cdmErr := cdm.Decode([]byte(cdmSrc))

	// The stricter reading (clause 7.5.10 has no unit-suffix grammar) means
	// every one of these must reject the suffixed epoch, not just odm.
	if odmErr == nil {
		t.Errorf("odm.DecodeOPM accepted an epoch with a unit suffix; want a rejection")
	}
	if admErr == nil {
		t.Errorf("adm.DecodeAPM accepted an epoch with a unit suffix; want a rejection")
	}
	if cdmErr == nil {
		t.Errorf("cdm.Decode accepted an epoch with a unit suffix; want a rejection")
	}

	agree := (odmErr == nil) == (admErr == nil) && (admErr == nil) == (cdmErr == nil)
	if !agree {
		t.Fatalf("epoch-units handling disagrees across message types: odm err=%v, adm err=%v, cdm err=%v",
			odmErr, admErr, cdmErr)
	}
}

// TestEpochUnitsBareFormAgreesAcrossMessageTypes checks the unsuffixed
// spelling still decodes to the same instant everywhere, so the fix for the
// test above has not made a valid epoch stop working.
func TestEpochUnitsBareFormAgreesAcrossMessageTypes(t *testing.T) {
	opm, err := odm.DecodeOPM([]byte(figureG1OPMForEpochTest))
	if err != nil {
		t.Fatalf("odm.DecodeOPM: %v", err)
	}
	apm, err := adm.DecodeAPM([]byte(figureG1APMForEpochTest))
	if err != nil {
		t.Fatalf("adm.DecodeAPM: %v", err)
	}
	c, err := cdm.Decode([]byte(clause362CDMForEpochTest))
	if err != nil {
		t.Fatalf("cdm.Decode: %v", err)
	}

	wantOPM := time.Date(2022, 12, 18, 14, 28, 15, 117200000, time.UTC)
	wantAPM := time.Date(2003, 9, 30, 14, 28, 15, 117200000, time.UTC)
	wantCDM := time.Date(2010, 3, 13, 22, 37, 52, 618000000, time.UTC)

	if !opm.Data.StateVector.Epoch.Equal(wantOPM) {
		t.Errorf("odm.DecodeOPM epoch = %v, want %v", opm.Data.StateVector.Epoch, wantOPM)
	}
	if !apm.Epoch.Equal(wantAPM) {
		t.Errorf("adm.DecodeAPM epoch = %v, want %v", apm.Epoch, wantAPM)
	}
	tca, ok := c.TCA()
	if !ok {
		t.Fatalf("cdm.Decode: TCA missing")
	}
	if !tca.Equal(wantCDM) {
		t.Errorf("cdm.Decode TCA = %v, want %v", tca, wantCDM)
	}
}

// figureG1OPMForEpochTest is figure G-1 of CCSDS 502.0-B-3, transcribed (see
// pkg/odm's own copy in opm_test.go for the annotated original).
const figureG1OPMForEpochTest = `CCSDS_OPM_VERS = 3.0
CREATION_DATE = 2022-11-06T09:23:57
ORIGINATOR     = JAXA

COMMENT          GEOCENTRIC, CARTESIAN, EARTH FIXED
OBJECT_NAME    = OSPREY 5
OBJECT_ID      = 1998-999A
CENTER_NAME    = EARTH
REF_FRAME      = ITRF2000
TIME_SYSTEM    = UTC

EPOCH =           2022-12-18T14:28:15.1172
X =               6503.514000
Y =               1239.647000
Z =               -717.490000
X_DOT =             -0.873160
Y_DOT =              8.740420
Z_DOT =             -4.191076
MASS =            3000.000000
SOLAR_RAD_AREA =    18.770000
SOLAR_RAD_COEFF =    1.000000
DRAG_AREA =         18.770000
DRAG_COEFF =         2.500000
`

// figureG1APMForEpochTest is figure G-1 of CCSDS 504.0-B-2, transcribed (see
// pkg/adm's own copy in adm_test.go for the annotated original).
const figureG1APMForEpochTest = `CCSDS_APM_VERS = 2.0
CREATION_DATE = 2003-09-30T19:23:57
ORIGINATOR = GSFC

OBJECT_NAME   = TRMM
OBJECT_ID     = 1997-062A
CENTER_NAME   = EARTH
TIME_SYSTEM   = UTC

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

// clause362CDMForEpochTest is the obligatory-keywords CDM fixture that CCSDS
// 508.0-B-1 clause 3.6.2 and vectors/cdm/obligatory-keywords.kvn both carry:
// two objects, each with the mandatory state vector and full covariance.
const clause362CDMForEpochTest = `CCSDS_CDM_VERS                          = 1.0
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
