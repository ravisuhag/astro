package adm

import (
	"strings"

	"github.com/ravisuhag/astro/internal/ndm"
)

// apmMetadataKeywords is table 3-2.
var apmMetadataKeywords = map[string]bool{
	"OBJECT_NAME": true, "OBJECT_ID": true,
	"CENTER_NAME": true, "TIME_SYSTEM": true,
}

// apmBlocks lists the six data blocks of table 3-3 with the keywords each one
// allows. A block delimiter is the block name plus _START or _STOP.
var apmBlocks = map[string]map[string]bool{
	blockQuaternion: {
		"REF_FRAME_A": true, "REF_FRAME_B": true,
		"Q1": true, "Q2": true, "Q3": true, "QC": true,
		"Q1_DOT": true, "Q2_DOT": true, "Q3_DOT": true, "QC_DOT": true,
	},
	blockEuler: {
		"REF_FRAME_A": true, "REF_FRAME_B": true, "EULER_ROT_SEQ": true,
		"ANGLE_1": true, "ANGLE_2": true, "ANGLE_3": true,
		"ANGLE_1_DOT": true, "ANGLE_2_DOT": true, "ANGLE_3_DOT": true,
	},
	blockAngVel: {
		"REF_FRAME_A": true, "REF_FRAME_B": true, "ANGVEL_FRAME": true,
		"ANGVEL_X": true, "ANGVEL_Y": true, "ANGVEL_Z": true,
	},
	blockSpin: {
		"REF_FRAME_A": true, "REF_FRAME_B": true,
		"SPIN_ALPHA": true, "SPIN_DELTA": true,
		"SPIN_ANGLE": true, "SPIN_ANGLE_VEL": true,
		"NUTATION": true, "NUTATION_PER": true, "NUTATION_PHASE": true,
		"MOMENTUM_ALPHA": true, "MOMENTUM_DELTA": true, "NUTATION_VEL": true,
	},
	blockInertia: {
		"INERTIA_REF_FRAME": true,
		"IXX":               true, "IYY": true, "IZZ": true,
		"IXY": true, "IXZ": true, "IYZ": true,
	},
	blockManeuver: {
		"MAN_EPOCH_START": true, "MAN_DURATION": true, "MAN_REF_FRAME": true,
		"MAN_TOR_X": true, "MAN_TOR_Y": true, "MAN_TOR_Z": true,
	},
}

// DecodeAPM reads an Attitude Parameter Message in 'keyword = value' notation.
func DecodeAPM(data []byte) (*APM, error) {
	s := ndm.NewScanner(data, true)

	header, err := ndm.ReadHeader(s, headerSpec("CCSDS_APM_VERS"))
	if err != nil {
		return nil, err
	}

	m := &APM{Header: headerFromNDM(header)}
	if err := readAPMBody(s, m); err != nil {
		return nil, err
	}
	if err := m.Validate(); err != nil {
		return nil, err
	}
	return m, nil
}

// readAPMBody reads the metadata section, then EPOCH, then the blocks.
//
// The APM has no META_START and META_STOP, unlike the AEM and the OEM: the
// metadata section simply ends at the first data keyword. EPOCH is what marks
// the change.
func readAPMBody(s *ndm.Scanner, m *APM) error {
	var (
		pending []string
		seen    = make(map[string]bool)
		inData  bool
	)

	for s.Scan() {
		line := s.Line()

		switch line.Kind {
		case ndm.Blank:
			continue
		case ndm.Comment:
			pending = append(pending, line.Value)
			continue
		case ndm.Free:
			return ndm.At(line.Number, ErrUnknownKeyword)
		}

		keyword, value, err := line.Assignment()
		if err != nil {
			return ndm.At(line.Number, err)
		}

		if name, ok := blockDelimiter(keyword, "_START"); ok {
			inData = true
			if err := readAPMBlock(s, m, name, pending); err != nil {
				return err
			}
			pending = nil
			continue
		}
		if _, ok := blockDelimiter(keyword, "_STOP"); ok {
			return ndm.At(line.Number, ErrUnexpectedDelimiter)
		}

		if keyword == "EPOCH" {
			if seen[keyword] {
				return ndm.At(line.Number, ErrDuplicateKeyword)
			}
			seen[keyword] = true
			inData = true
			t, err := parseEpoch(value)
			if err != nil {
				return ndm.At(line.Number, err)
			}
			m.Epoch = t
			m.Comments = append(m.Comments, pending...)
			pending = nil
			continue
		}

		if inData || !apmMetadataKeywords[keyword] {
			return ndm.At(line.Number, ErrUnknownKeyword)
		}
		if seen[keyword] {
			return ndm.At(line.Number, ErrDuplicateKeyword)
		}
		seen[keyword] = true

		m.Metadata.Comments = append(m.Metadata.Comments, pending...)
		pending = nil

		switch keyword {
		case "OBJECT_NAME":
			m.Metadata.ObjectName = ndm.ParseText(value)
		case "OBJECT_ID":
			m.Metadata.ObjectID = ndm.ParseText(value)
		case "CENTER_NAME":
			m.Metadata.CenterName = ndm.ParseText(value)
		case "TIME_SYSTEM":
			m.Metadata.TimeSystem = value
		}
	}
	return s.Err()
}

// blockDelimiter reports the block name behind a *_START or *_STOP keyword.
func blockDelimiter(keyword, suffix string) (string, bool) {
	name, ok := strings.CutSuffix(keyword, suffix)
	if !ok {
		return "", false
	}
	if _, known := apmBlocks[name]; !known {
		return "", false
	}
	return name, true
}

// readAPMBlock reads one delimited block, up to its own *_STOP.
//
// The stop keyword must match the start. A QUAT_START closed by EULER_STOP is
// refused rather than accepted as the end of something.
func readAPMBlock(s *ndm.Scanner, m *APM, name string, comments []string) error {
	allowed := apmBlocks[name]
	fields := make(map[string]string)
	seen := make(map[string]bool)
	blockComments := comments

	for s.Scan() {
		line := s.Line()

		switch line.Kind {
		case ndm.Blank:
			continue
		case ndm.Comment:
			blockComments = append(blockComments, line.Value)
			continue
		case ndm.Free:
			return ndm.At(line.Number, ErrUnterminatedBlock)
		}

		keyword, value, err := line.Assignment()
		if err != nil {
			return ndm.At(line.Number, err)
		}

		if stopped, ok := blockDelimiter(keyword, "_STOP"); ok {
			if stopped != name {
				return ndm.At(line.Number, ErrUnexpectedDelimiter)
			}
			return buildAPMBlock(m, name, fields, blockComments)
		}
		if _, ok := blockDelimiter(keyword, "_START"); ok {
			return ndm.At(line.Number, ErrUnexpectedDelimiter)
		}

		if !allowed[keyword] {
			return ndm.At(line.Number, ErrUnknownKeyword)
		}
		if seen[keyword] {
			return ndm.At(line.Number, ErrDuplicateKeyword)
		}
		seen[keyword] = true
		fields[keyword] = value
	}
	if err := s.Err(); err != nil {
		return err
	}
	return ErrUnterminatedBlock
}
