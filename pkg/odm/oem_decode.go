package odm

import (
	"strings"

	"github.com/ravisuhag/astro/internal/ndm"
)

// oemMetadataKeywords is table 5-3 minus the two delimiters.
var oemMetadataKeywords = map[string]bool{
	"OBJECT_NAME": true, "OBJECT_ID": true, "CENTER_NAME": true,
	"REF_FRAME": true, "REF_FRAME_EPOCH": true, "TIME_SYSTEM": true,
	"START_TIME": true, "USEABLE_START_TIME": true, "USEABLE_STOP_TIME": true,
	"STOP_TIME": true, "INTERPOLATION": true, "INTERPOLATION_DEGREE": true,
}

// DecodeOEM reads an Orbit Ephemeris Message in 'keyword = value' notation.
//
// The structure is a header, then one or more metadata groups each followed by
// its ephemeris data and an optional covariance section. Clause 5.2.4.6 makes
// a second metadata group meaningful rather than merely allowed: it tells a
// consumer not to interpolate across the boundary.
func DecodeOEM(data []byte) (*OEM, error) {
	// Clause 7.3.2 caps a line at 254 characters for the OEM as for the OPM;
	// only the OCM is exempt.
	s := ndm.NewScanner(data, true)

	header, err := ndm.ReadHeader(s, oemHeaderSpec)
	if err != nil {
		return nil, err
	}

	m := &OEM{Header: headerFromNDM(header)}
	if err := readEphemerisBlocks(s, m); err != nil {
		return nil, err
	}
	if err := m.Validate(); err != nil {
		return nil, err
	}
	return m, nil
}

// readEphemerisBlocks walks the metadata groups and their data.
func readEphemerisBlocks(s *ndm.Scanner, m *OEM) error {
	var pending []string

	for s.Scan() {
		line := s.Line()

		switch line.Kind {
		case ndm.Blank:
			continue
		case ndm.Comment:
			pending = append(pending, line.Value)
			continue
		case ndm.Free:
			// An ephemeris row outside any block. Clause 5.2.3.3 requires a
			// metadata group before each ephemeris block, so there is nothing
			// this row could belong to.
			return ndm.At(line.Number, ErrUnexpectedDelimiter)
		}

		keyword, _, err := line.Assignment()
		if err != nil {
			return ndm.At(line.Number, err)
		}
		if keyword != keywordMetaStart {
			return ndm.At(line.Number, ErrUnexpectedDelimiter)
		}

		block := EphemerisBlock{Metadata: OEMMetadata{Comments: pending}}
		pending = nil
		if err := readMetadata(s, &block.Metadata); err != nil {
			return err
		}
		if err := readEphemerisData(s, &block); err != nil {
			return err
		}
		m.Blocks = append(m.Blocks, block)
	}
	return s.Err()
}

// readMetadata reads the keywords between META_START and META_STOP.
func readMetadata(s *ndm.Scanner, md *OEMMetadata) error {
	seen := make(map[string]bool)

	for s.Scan() {
		line := s.Line()

		switch line.Kind {
		case ndm.Blank:
			continue
		case ndm.Comment:
			md.Comments = append(md.Comments, line.Value)
			continue
		case ndm.Free:
			return ndm.At(line.Number, ErrUnterminatedBlock)
		}

		keyword, value, err := line.Assignment()
		if err != nil {
			return ndm.At(line.Number, err)
		}
		switch keyword {
		case keywordMetaStop:
			return nil
		case keywordMetaStart:
			return ndm.At(line.Number, ErrUnexpectedDelimiter)
		}
		if !oemMetadataKeywords[keyword] {
			return ndm.At(line.Number, ErrUnknownKeyword)
		}
		if seen[keyword] {
			return ndm.At(line.Number, ErrDuplicateKeyword)
		}
		seen[keyword] = true

		if err := assignOEMMetadata(md, keyword, value); err != nil {
			return ndm.At(line.Number, err)
		}
	}
	if err := s.Err(); err != nil {
		return err
	}
	return ErrUnterminatedBlock
}

func assignOEMMetadata(md *OEMMetadata, keyword, value string) error {
	switch keyword {
	case "OBJECT_NAME":
		name, err := ndm.ParseTextRequired(value)
		if err != nil {
			return err
		}
		md.ObjectName = name
	case "OBJECT_ID":
		id, err := ndm.ParseTextRequired(value)
		if err != nil {
			return err
		}
		md.ObjectID = id
	case "CENTER_NAME":
		center, err := ndm.ParseTextRequired(value)
		if err != nil {
			return err
		}
		md.CenterName = center
	case "REF_FRAME":
		md.RefFrame = value
	case "TIME_SYSTEM":
		md.TimeSystem = value
	case "INTERPOLATION":
		md.Interpolation = value
	case "INTERPOLATION_DEGREE":
		degree, err := ndm.ParseInt(value)
		if err != nil {
			return err
		}
		md.InterpolationDegree = degree
	case "START_TIME", "STOP_TIME", "REF_FRAME_EPOCH",
		"USEABLE_START_TIME", "USEABLE_STOP_TIME":
		t, err := ndm.ParseEpoch(value)
		if err != nil {
			return err
		}
		switch keyword {
		case "START_TIME":
			md.StartTime = t
		case "STOP_TIME":
			md.StopTime = t
		case "REF_FRAME_EPOCH":
			md.RefFrameEpoch = &t
		case "USEABLE_START_TIME":
			md.UseableStartTime = &t
		case "USEABLE_STOP_TIME":
			md.UseableStopTime = &t
		}
	}
	return nil
}

// readEphemerisData reads the rows after META_STOP, and the covariance section
// if one follows. It stops at the next META_START and hands it back.
func readEphemerisData(s *ndm.Scanner, block *EphemerisBlock) error {
	for s.Scan() {
		line := s.Line()

		switch line.Kind {
		case ndm.Blank:
			continue
		case ndm.Comment:
			block.Comments = append(block.Comments, line.Value)
			continue
		case ndm.Free:
			row, err := parseEphemerisLine(line.Text)
			if err != nil {
				return ndm.At(line.Number, err)
			}
			block.Lines = append(block.Lines, row)
			continue
		}

		keyword, _, err := line.Assignment()
		if err != nil {
			return ndm.At(line.Number, err)
		}
		switch keyword {
		case keywordMetaStart:
			// The next block. Hand it back to the caller.
			s.Unread()
			return nil
		case keywordCovarianceStart:
			if err := readCovarianceSection(s, block); err != nil {
				return err
			}
			continue
		}
		return ndm.At(line.Number, ErrUnexpectedDelimiter)
	}
	return s.Err()
}

// readCovarianceSection reads between COVARIANCE_START and COVARIANCE_STOP.
//
// A new EPOCH starts a new matrix (clause 5.2.5.6 allows any number of them),
// and the 21 values of each are spread over as many lines as the producer
// chose, so they are accumulated rather than read a row at a time.
func readCovarianceSection(s *ndm.Scanner, block *EphemerisBlock) error {
	var (
		current *OEMCovariance
		values  []float64
	)

	finish := func() error {
		if current == nil {
			return nil
		}
		if len(values) != len(covarianceOrder) {
			return ErrCovarianceValueCount
		}
		for i, at := range covarianceOrder {
			// Symmetric, so both triangles are filled.
			current.Matrix[at[0]][at[1]] = values[i]
			current.Matrix[at[1]][at[0]] = values[i]
		}
		block.Covariances = append(block.Covariances, *current)
		current, values = nil, nil
		return nil
	}

	for s.Scan() {
		line := s.Line()

		switch line.Kind {
		case ndm.Blank:
			continue
		case ndm.Comment:
			block.CovarianceComments = append(block.CovarianceComments, line.Value)
			continue
		case ndm.Free:
			if current == nil {
				// Clause 5.2.5.3 requires the epoch, and it comes first.
				return ndm.At(line.Number, ErrMissingKeyword)
			}
			for _, field := range strings.Fields(line.Text) {
				v, err := ndm.ParseFloat(field)
				if err != nil {
					return ndm.At(line.Number, err)
				}
				values = append(values, v)
			}
			if len(values) > len(covarianceOrder) {
				return ndm.At(line.Number, ErrCovarianceValueCount)
			}
			continue
		}

		keyword, value, err := line.Assignment()
		if err != nil {
			return ndm.At(line.Number, err)
		}
		switch keyword {
		case keywordCovarianceStop:
			if err := finish(); err != nil {
				return ndm.At(line.Number, err)
			}
			return nil
		case "EPOCH":
			if err := finish(); err != nil {
				return ndm.At(line.Number, err)
			}
			t, err := ndm.ParseEpoch(value)
			if err != nil {
				return ndm.At(line.Number, err)
			}
			current = &OEMCovariance{Epoch: t}
		case "COV_REF_FRAME":
			if current == nil {
				return ndm.At(line.Number, ErrKeywordOutOfOrder)
			}
			current.RefFrame = value
		default:
			return ndm.At(line.Number, ErrUnknownKeyword)
		}
	}
	if err := s.Err(); err != nil {
		return err
	}
	return ErrUnterminatedBlock
}

// parseEphemerisLine reads one positional data row.
//
// Clause 5.2.4.1 fixes the order and clause 5.2.4.2 makes acceleration
// optional, so a row has either 7 fields or 10 and nothing between.
func parseEphemerisLine(text string) (EphemerisLine, error) {
	var line EphemerisLine

	fields := strings.Fields(text)
	if len(fields) != 7 && len(fields) != 10 {
		return line, ErrEphemerisLineFields
	}

	epoch, err := ndm.ParseEpoch(fields[0])
	if err != nil {
		return line, err
	}
	line.Epoch = epoch

	values := make([]float64, len(fields)-1)
	for i, field := range fields[1:] {
		v, err := ndm.ParseFloat(field)
		if err != nil {
			return line, err
		}
		values[i] = v
	}

	line.X, line.Y, line.Z = values[0], values[1], values[2]
	line.XDot, line.YDot, line.ZDot = values[3], values[4], values[5]
	if len(values) == 9 {
		line.XDDot, line.YDDot, line.ZDDot = values[6], values[7], values[8]
		line.HasAcceleration = true
	}
	return line, nil
}
