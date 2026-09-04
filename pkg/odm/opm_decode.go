package odm

import (
	"strings"

	"github.com/ravisuhag/astro/internal/ndm"
)

// block names the logical section a keyword belongs to. Table 3-2 is one
// block and table 3-3 is six more, and knowing which one a keyword starts is
// what lets a comment be attached to the right thing: every standard here puts
// comments "at the beginning of a logical block".
type block int

const (
	blockNone block = iota
	blockMetadata
	blockStateVector
	blockKeplerian
	blockSpacecraft
	blockCovariance
	blockManeuver
	blockUserDefined
)

// keywordBlock maps every keyword tables 3-2 and 3-3 define to its block.
// USER_DEFINED_x is not here because its keyword is only known by its prefix.
var keywordBlock = map[string]block{
	"OBJECT_NAME": blockMetadata, "OBJECT_ID": blockMetadata,
	"CENTER_NAME": blockMetadata, "REF_FRAME": blockMetadata,
	"REF_FRAME_EPOCH": blockMetadata, "TIME_SYSTEM": blockMetadata,

	"EPOCH": blockStateVector,
	"X":     blockStateVector, "Y": blockStateVector, "Z": blockStateVector,
	"X_DOT": blockStateVector, "Y_DOT": blockStateVector, "Z_DOT": blockStateVector,

	"SEMI_MAJOR_AXIS": blockKeplerian, "ECCENTRICITY": blockKeplerian,
	"INCLINATION": blockKeplerian, "RA_OF_ASC_NODE": blockKeplerian,
	"ARG_OF_PERICENTER": blockKeplerian, "TRUE_ANOMALY": blockKeplerian,
	"MEAN_ANOMALY": blockKeplerian, "GM": blockKeplerian,

	"MASS": blockSpacecraft, "SOLAR_RAD_AREA": blockSpacecraft,
	"SOLAR_RAD_COEFF": blockSpacecraft, "DRAG_AREA": blockSpacecraft,
	"DRAG_COEFF": blockSpacecraft,

	"COV_REF_FRAME": blockCovariance,

	"MAN_EPOCH_IGNITION": blockManeuver, "MAN_DURATION": blockManeuver,
	"MAN_DELTA_MASS": blockManeuver, "MAN_REF_FRAME": blockManeuver,
	"MAN_DV_1": blockManeuver, "MAN_DV_2": blockManeuver, "MAN_DV_3": blockManeuver,
}

// covarianceIndex maps a covariance keyword to its place in the matrix.
var covarianceIndex = func() map[string][2]int {
	m := make(map[string][2]int, len(covarianceElements))
	for _, e := range covarianceElements {
		m[e.keyword] = [2]int{e.row, e.col}
	}
	return m
}()

func init() {
	for keyword := range covarianceIndex {
		keywordBlock[keyword] = blockCovariance
	}
}

// DecodeOPM reads an Orbit Parameter Message in 'keyword = value' notation.
func DecodeOPM(data []byte) (*OPM, error) {
	// Clause 7.3.2 caps an OPM line at 254 characters. Only the OCM is exempt.
	s := ndm.NewScanner(data, true)

	header, err := ndm.ReadHeader(s, opmHeaderSpec)
	if err != nil {
		return nil, err
	}

	d := &opmDecoder{
		message: &OPM{Header: headerFromNDM(header)},
		seen:    make(map[string]bool),
	}
	if err := d.run(s); err != nil {
		return nil, err
	}
	if err := d.message.Validate(); err != nil {
		return nil, err
	}
	return d.message, nil
}

// opmDecoder carries the state of one decode.
type opmDecoder struct {
	message *OPM
	seen    map[string]bool

	pending []string // comments waiting for the block they introduce
	current block
}

func (d *opmDecoder) run(s *ndm.Scanner) error {
	for s.Scan() {
		line := s.Line()

		switch line.Kind {
		case ndm.Blank:
			continue
		case ndm.Comment:
			d.pending = append(d.pending, line.Value)
			continue
		case ndm.Free:
			// Table 3-3 has no positional data rows: every line in an OPM is
			// an assignment or a comment.
			return ndm.At(line.Number, ErrUnknownKeyword)
		}

		keyword, value, err := line.Assignment()
		if err != nil {
			return ndm.At(line.Number, err)
		}
		if err := d.assign(keyword, value); err != nil {
			return ndm.At(line.Number, err)
		}
	}
	return s.Err()
}

// assign routes one keyword to its block.
func (d *opmDecoder) assign(keyword, value string) error {
	if name, ok := strings.CutPrefix(keyword, userDefinedPrefix); ok {
		d.enter(blockUserDefined)
		d.message.Data.UserDefined = append(d.message.Data.UserDefined,
			UserDefined{Name: name, Value: value})
		return nil
	}

	target, known := keywordBlock[keyword]
	if !known {
		// Clause 3.2.4.2: only the keywords in the tables shall be used.
		return ErrUnknownKeyword
	}

	// A second MAN_EPOCH_IGNITION starts another manoeuvre rather than
	// repeating the first (clause 3.2.4.8).
	if keyword == "MAN_EPOCH_IGNITION" {
		d.startManeuver()
	} else if d.seen[keyword] {
		return ErrDuplicateKeyword
	}
	d.seen[keyword] = true

	d.enter(target)

	switch target {
	case blockMetadata:
		return d.assignMetadata(keyword, value)
	case blockStateVector:
		return d.assignStateVector(keyword, value)
	case blockKeplerian:
		return d.assignKeplerian(keyword, value)
	case blockSpacecraft:
		return assignSpacecraftKeyword(d.spacecraft(), keyword, value)
	case blockCovariance:
		return assignCovarianceKeyword(d.covariance(), keyword, value)
	case blockManeuver:
		return d.assignManeuver(keyword, value)
	}
	return ErrUnknownKeyword
}

// enter moves to a block, handing it any comments that were waiting.
func (d *opmDecoder) enter(target block) {
	if target == d.current {
		return
	}
	d.current = target
	if len(d.pending) == 0 {
		return
	}

	comments := d.pending
	d.pending = nil

	switch target {
	case blockMetadata:
		d.message.Metadata.Comments = append(d.message.Metadata.Comments, comments...)
	case blockStateVector:
		d.message.Data.StateVector.Comments = append(d.message.Data.StateVector.Comments, comments...)
	case blockKeplerian:
		d.keplerian().Comments = append(d.keplerian().Comments, comments...)
	case blockSpacecraft:
		d.spacecraft().Comments = append(d.spacecraft().Comments, comments...)
	case blockCovariance:
		d.covariance().Comments = append(d.covariance().Comments, comments...)
	case blockManeuver:
		if n := len(d.message.Data.Maneuvers); n > 0 {
			m := &d.message.Data.Maneuvers[n-1]
			m.Comments = append(m.Comments, comments...)
		}
	}
}

// startManeuver opens a new manoeuvre, moving out of whatever block came
// before so that pending comments land on it.
func (d *opmDecoder) startManeuver() {
	d.current = blockNone
	d.message.Data.Maneuvers = append(d.message.Data.Maneuvers, Maneuver{})

	// Every manoeuvre repeats all seven keywords (clause 3.2.4.8), so the
	// duplicate check starts again for each one.
	for _, keyword := range []string{
		"MAN_EPOCH_IGNITION", "MAN_DURATION", "MAN_DELTA_MASS",
		"MAN_REF_FRAME", "MAN_DV_1", "MAN_DV_2", "MAN_DV_3",
	} {
		delete(d.seen, keyword)
	}
}

func (d *opmDecoder) keplerian() *KeplerianElements {
	if d.message.Data.Keplerian == nil {
		d.message.Data.Keplerian = &KeplerianElements{}
	}
	return d.message.Data.Keplerian
}

func (d *opmDecoder) spacecraft() *SpacecraftParameters {
	if d.message.Data.Spacecraft == nil {
		d.message.Data.Spacecraft = &SpacecraftParameters{}
	}
	return d.message.Data.Spacecraft
}

func (d *opmDecoder) covariance() *Covariance {
	if d.message.Data.Covariance == nil {
		d.message.Data.Covariance = &Covariance{}
	}
	return d.message.Data.Covariance
}

func (d *opmDecoder) assignMetadata(keyword, value string) error {
	md := &d.message.Metadata

	switch keyword {
	case "OBJECT_NAME":
		md.ObjectName = ndm.ParseText(value)
	case "OBJECT_ID":
		md.ObjectID = ndm.ParseText(value)
	case "CENTER_NAME":
		md.CenterName = ndm.ParseText(value)
	case "REF_FRAME":
		md.RefFrame = value
	case "TIME_SYSTEM":
		md.TimeSystem = value
	case "REF_FRAME_EPOCH":
		t, err := parseEpochValue(value)
		if err != nil {
			return err
		}
		md.RefFrameEpoch = &t
	}
	return nil
}

func (d *opmDecoder) assignStateVector(keyword, value string) error {
	sv := &d.message.Data.StateVector

	if keyword == "EPOCH" {
		t, err := parseEpochValue(value)
		if err != nil {
			return err
		}
		sv.Epoch = t
		return nil
	}

	v, err := parseValue(value)
	if err != nil {
		return err
	}
	switch keyword {
	case "X":
		sv.X = v
	case "Y":
		sv.Y = v
	case "Z":
		sv.Z = v
	case "X_DOT":
		sv.XDot = v
	case "Y_DOT":
		sv.YDot = v
	case "Z_DOT":
		sv.ZDot = v
	}
	return nil
}

func (d *opmDecoder) assignKeplerian(keyword, value string) error {
	k := d.keplerian()

	v, err := parseValue(value)
	if err != nil {
		return err
	}
	switch keyword {
	case "SEMI_MAJOR_AXIS":
		k.SemiMajorAxis = v
	case "ECCENTRICITY":
		k.Eccentricity = v
	case "INCLINATION":
		k.Inclination = v
	case "RA_OF_ASC_NODE":
		k.RAOfAscNode = v
	case "ARG_OF_PERICENTER":
		k.ArgOfPericenter = v
	case "TRUE_ANOMALY", "MEAN_ANOMALY":
		// Table 3-3 offers the two as alternatives, so a message that gives
		// both has not said which one it means.
		if d.seen["TRUE_ANOMALY"] && d.seen["MEAN_ANOMALY"] {
			return ErrBothAnomalies
		}
		k.Anomaly = v
		k.AnomalyIsMean = keyword == "MEAN_ANOMALY"
	case "GM":
		k.GM = v
	}
	return nil
}

func (d *opmDecoder) assignManeuver(keyword, value string) error {
	if len(d.message.Data.Maneuvers) == 0 {
		// Clause 3.2.4.8 repeats all the parameters for each manoeuvre, in the
		// order table 3-3 fixes, and MAN_EPOCH_IGNITION comes first.
		return ErrKeywordOutOfOrder
	}
	m := &d.message.Data.Maneuvers[len(d.message.Data.Maneuvers)-1]

	if keyword == "MAN_EPOCH_IGNITION" {
		t, err := parseEpochValue(value)
		if err != nil {
			return err
		}
		m.EpochIgnition = t
		return nil
	}
	if keyword == "MAN_REF_FRAME" {
		m.RefFrame = value
		return nil
	}

	v, err := parseValue(value)
	if err != nil {
		return err
	}
	switch keyword {
	case "MAN_DURATION":
		m.Duration = v
	case "MAN_DELTA_MASS":
		m.DeltaMass = v
	case "MAN_DV_1":
		m.DV[0] = v
	case "MAN_DV_2":
		m.DV[1] = v
	case "MAN_DV_3":
		m.DV[2] = v
	}
	return nil
}
