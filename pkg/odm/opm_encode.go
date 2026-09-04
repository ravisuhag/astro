package odm

import (
	"strings"

	"github.com/ravisuhag/astro/internal/ndm"
)

// covarianceElement pairs a covariance keyword with its place in the matrix.
// Clause 3.2.4.10 fixes the order: lower triangular, row by row, left to
// right, from [1,1] to [6,6].
type covarianceElement struct {
	keyword  string
	row, col int
	units    string
}

// covarianceElements is table 3-3's covariance block in wire order. The units
// depend on how many velocity components the element is built from, which is
// why they are not all the same.
var covarianceElements = []covarianceElement{
	{"CX_X", 0, 0, "km**2"},
	{"CY_X", 1, 0, "km**2"},
	{"CY_Y", 1, 1, "km**2"},
	{"CZ_X", 2, 0, "km**2"},
	{"CZ_Y", 2, 1, "km**2"},
	{"CZ_Z", 2, 2, "km**2"},
	{"CX_DOT_X", 3, 0, "km**2/s"},
	{"CX_DOT_Y", 3, 1, "km**2/s"},
	{"CX_DOT_Z", 3, 2, "km**2/s"},
	{"CX_DOT_X_DOT", 3, 3, "km**2/s**2"},
	{"CY_DOT_X", 4, 0, "km**2/s"},
	{"CY_DOT_Y", 4, 1, "km**2/s"},
	{"CY_DOT_Z", 4, 2, "km**2/s"},
	{"CY_DOT_X_DOT", 4, 3, "km**2/s**2"},
	{"CY_DOT_Y_DOT", 4, 4, "km**2/s**2"},
	{"CZ_DOT_X", 5, 0, "km**2/s"},
	{"CZ_DOT_Y", 5, 1, "km**2/s"},
	{"CZ_DOT_Z", 5, 2, "km**2/s"},
	{"CZ_DOT_X_DOT", 5, 3, "km**2/s**2"},
	{"CZ_DOT_Y_DOT", 5, 4, "km**2/s**2"},
	{"CZ_DOT_Z_DOT", 5, 5, "km**2/s**2"},
}

// userDefinedPrefix is what clause 3.2.4.12 puts in front of every
// user-defined keyword.
const userDefinedPrefix = "USER_DEFINED_"

// Encode writes the message in 'keyword = value' notation.
//
// Units are written for every item table 3-3 gives one, which clause 7.7.1.1
// allows "for documentation purposes and clarity only". They carry no meaning
// a reader needs, and a reader that ignores them reads the same message.
func (m *OPM) Encode() ([]byte, error) {
	if err := m.Validate(); err != nil {
		return nil, err
	}

	var w ndm.Writer
	if err := m.Header.toNDM().Write(&w, opmHeaderSpec); err != nil {
		return nil, err
	}

	m.writeMetadata(&w)
	m.writeData(&w)
	return w.Bytes(), nil
}

func (m *OPM) writeMetadata(w *ndm.Writer) {
	md := m.Metadata

	w.Comments(md.Comments)
	w.Assign("OBJECT_NAME", md.ObjectName)
	w.Assign("OBJECT_ID", md.ObjectID)
	w.Assign("CENTER_NAME", md.CenterName)
	w.Assign("REF_FRAME", md.RefFrame)
	if md.RefFrameEpoch != nil {
		if epoch, err := ndm.FormatEpoch(*md.RefFrameEpoch, 0); err == nil {
			w.Assign("REF_FRAME_EPOCH", epoch)
		}
	}
	w.Assign("TIME_SYSTEM", md.TimeSystem)
}

func (m *OPM) writeData(w *ndm.Writer) {
	m.writeStateVector(w)

	if k := m.Data.Keplerian; k != nil {
		w.Comments(k.Comments)
		w.AssignUnits("SEMI_MAJOR_AXIS", formatValue(k.SemiMajorAxis), "km")
		w.Assign("ECCENTRICITY", formatValue(k.Eccentricity))
		w.AssignUnits("INCLINATION", formatValue(k.Inclination), "deg")
		w.AssignUnits("RA_OF_ASC_NODE", formatValue(k.RAOfAscNode), "deg")
		w.AssignUnits("ARG_OF_PERICENTER", formatValue(k.ArgOfPericenter), "deg")
		anomaly := "TRUE_ANOMALY"
		if k.AnomalyIsMean {
			anomaly = "MEAN_ANOMALY"
		}
		w.AssignUnits(anomaly, formatValue(k.Anomaly), "deg")
		w.AssignUnits("GM", formatValue(k.GM), "km**3/s**2")
	}

	if s := m.Data.Spacecraft; s != nil {
		writeSpacecraftParameters(w, s)
	}
	if c := m.Data.Covariance; c != nil {
		writeCovarianceKeywords(w, c)
	}

	for _, man := range m.Data.Maneuvers {
		w.Comments(man.Comments)
		if epoch, err := ndm.FormatEpoch(man.EpochIgnition, 1); err == nil {
			w.Assign("MAN_EPOCH_IGNITION", epoch)
		}
		w.AssignUnits("MAN_DURATION", formatValue(man.Duration), "s")
		w.AssignUnits("MAN_DELTA_MASS", formatValue(man.DeltaMass), "kg")
		w.Assign("MAN_REF_FRAME", man.RefFrame)
		for i, dv := range man.DV {
			w.AssignUnits("MAN_DV_"+string(rune('1'+i)), formatValue(dv), "km/s")
		}
	}

	for _, u := range m.Data.UserDefined {
		w.Assign(userDefinedPrefix+strings.ToUpper(u.Name), u.Value)
	}
}

func (m *OPM) writeStateVector(w *ndm.Writer) {
	sv := m.Data.StateVector

	w.Comments(sv.Comments)
	if epoch, err := ndm.FormatEpoch(sv.Epoch, epochPrecision(sv.Epoch)); err == nil {
		w.Assign("EPOCH", epoch)
	}
	w.AssignUnits("X", formatValue(sv.X), "km")
	w.AssignUnits("Y", formatValue(sv.Y), "km")
	w.AssignUnits("Z", formatValue(sv.Z), "km")
	w.AssignUnits("X_DOT", formatValue(sv.XDot), "km/s")
	w.AssignUnits("Y_DOT", formatValue(sv.YDot), "km/s")
	w.AssignUnits("Z_DOT", formatValue(sv.ZDot), "km/s")
}
