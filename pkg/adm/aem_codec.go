package adm

import (
	"strings"
	"time"

	"github.com/ravisuhag/astro/internal/ndm"
)

// aemMetadataKeywords is table 4-3, minus the two delimiters.
var aemMetadataKeywords = map[string]bool{
	"OBJECT_NAME": true, "OBJECT_ID": true, "CENTER_NAME": true,
	"REF_FRAME_A": true, "REF_FRAME_B": true, "TIME_SYSTEM": true,
	"START_TIME": true, "STOP_TIME": true,
	"USEABLE_START_TIME": true, "USEABLE_STOP_TIME": true,
	"ATTITUDE_TYPE": true, "EULER_ROT_SEQ": true, "ANGVEL_FRAME": true,
	"INTERPOLATION_METHOD": true, "INTERPOLATION_DEGREE": true,
}

// DecodeAEM reads an Attitude Ephemeris Message in 'keyword = value' notation.
func DecodeAEM(data []byte) (*AEM, error) {
	s := ndm.NewScanner(data, true)

	header, err := ndm.ReadHeader(s, headerSpec("CCSDS_AEM_VERS"))
	if err != nil {
		return nil, err
	}

	m := &AEM{Header: headerFromNDM(header)}
	if err := readAEMBlocks(s, m); err != nil {
		return nil, err
	}
	if err := m.Validate(); err != nil {
		return nil, err
	}
	return m, nil
}

func readAEMBlocks(s *ndm.Scanner, m *AEM) error {
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
			return ndm.At(line.Number, ErrUnexpectedDelimiter)
		}

		keyword, _, err := line.Assignment()
		if err != nil {
			return ndm.At(line.Number, err)
		}
		if keyword != keywordMetaStart {
			return ndm.At(line.Number, ErrUnexpectedDelimiter)
		}

		block := AttitudeBlock{Metadata: AEMMetadata{Comments: pending}}
		pending = nil
		if err := readAEMMetadata(s, &block.Metadata); err != nil {
			return err
		}
		if err := readAEMData(s, &block); err != nil {
			return err
		}
		m.Blocks = append(m.Blocks, block)
	}
	return s.Err()
}

func readAEMMetadata(s *ndm.Scanner, md *AEMMetadata) error {
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
		case keywordMetaStart, keywordDataStart, keywordDataStop:
			return ndm.At(line.Number, ErrUnexpectedDelimiter)
		}
		if !aemMetadataKeywords[keyword] {
			return ndm.At(line.Number, ErrUnknownKeyword)
		}
		if seen[keyword] {
			return ndm.At(line.Number, ErrDuplicateKeyword)
		}
		seen[keyword] = true

		if err := assignAEMMetadata(md, keyword, value); err != nil {
			return ndm.At(line.Number, err)
		}
	}
	if err := s.Err(); err != nil {
		return err
	}
	return ErrUnterminatedBlock
}

func assignAEMMetadata(md *AEMMetadata, keyword, value string) error {
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
		// Table 4-3 leaves CENTER_NAME optional, so a blank value here is not
		// refused the way the mandatory fields above are.
		md.CenterName = ndm.ParseText(value)
	case "REF_FRAME_A":
		md.FrameA = value
	case "REF_FRAME_B":
		md.FrameB = value
	case "TIME_SYSTEM":
		md.TimeSystem = value
	case "ATTITUDE_TYPE":
		// Table 4-3 prints the values in upper case and the annex G example
		// writes INTERPOLATION_METHOD in lower case, so case is not a reliable
		// signal in this message. The type is normalised because its value
		// decides how wide a data line is.
		md.Type = AttitudeType(strings.ToUpper(value))
		if !md.Type.Valid() {
			return ErrUnknownAttitudeType
		}
	case "EULER_ROT_SEQ":
		md.RotSeq = strings.ToUpper(value)
	case "ANGVEL_FRAME":
		md.AngVelFrame = value
	case "INTERPOLATION_METHOD":
		md.InterpolationMethod = value
	case "INTERPOLATION_DEGREE":
		degree, err := ndm.ParseInt(value)
		if err != nil {
			return err
		}
		md.InterpolationDegree = degree
	case "START_TIME", "STOP_TIME", "USEABLE_START_TIME", "USEABLE_STOP_TIME":
		t, err := ndm.ParseEpoch(value)
		if err != nil {
			return err
		}
		switch keyword {
		case "START_TIME":
			md.StartTime = t
		case "STOP_TIME":
			md.StopTime = t
		case "USEABLE_START_TIME":
			md.UseableStartTime = &t
		case "USEABLE_STOP_TIME":
			md.UseableStopTime = &t
		}
	}
	return nil
}

// readAEMData reads between DATA_START and DATA_STOP.
//
// Every line is checked against the width the segment's ATTITUDE_TYPE implies,
// so a file that changed type without saying so fails here rather than
// producing attitudes that are silently wrong.
func readAEMData(s *ndm.Scanner, block *AttitudeBlock) error {
	want, ok := block.Metadata.Type.Fields()
	if !ok {
		return ErrUnknownAttitudeType
	}

	started := false
	for s.Scan() {
		line := s.Line()

		switch line.Kind {
		case ndm.Blank:
			continue
		case ndm.Comment:
			block.Comments = append(block.Comments, line.Value)
			continue
		case ndm.Free:
			if !started {
				return ndm.At(line.Number, ErrMissingDataSection)
			}
			row, err := parseAttitudeLine(line.Text, want)
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
		case keywordDataStart:
			if started {
				return ndm.At(line.Number, ErrUnexpectedDelimiter)
			}
			started = true
			continue
		case keywordDataStop:
			if !started {
				return ndm.At(line.Number, ErrUnexpectedDelimiter)
			}
			return nil
		case keywordMetaStart:
			return ndm.At(line.Number, ErrMissingDataSection)
		}
		return ndm.At(line.Number, ErrUnknownKeyword)
	}
	if err := s.Err(); err != nil {
		return err
	}
	if started {
		return ErrUnterminatedBlock
	}
	return ErrMissingDataSection
}

// parseAttitudeLine reads one positional data row: an epoch and exactly the
// number of values the segment's type calls for.
func parseAttitudeLine(text string, want int) (AttitudeLine, error) {
	fields := strings.Fields(text)
	if len(fields) != want+1 {
		return AttitudeLine{}, ErrAttitudeLineFields
	}

	epoch, err := ndm.ParseEpoch(fields[0])
	if err != nil {
		return AttitudeLine{}, err
	}

	values := make([]float64, want)
	for i, field := range fields[1:] {
		v, err := ndm.ParseFloat(field)
		if err != nil {
			return AttitudeLine{}, err
		}
		values[i] = v
	}
	return AttitudeLine{Epoch: epoch, Values: values}, nil
}

// Encode writes the message in 'keyword = value' notation.
func (m *AEM) Encode() ([]byte, error) {
	if err := m.Validate(); err != nil {
		return nil, err
	}

	var w ndm.Writer
	if err := m.Header.toNDM().Write(&w, headerSpec("CCSDS_AEM_VERS")); err != nil {
		return nil, err
	}

	for i := range m.Blocks {
		block := &m.Blocks[i]
		md := &block.Metadata

		w.Blank()
		w.Section(keywordMetaStart)
		w.Comments(md.Comments)
		w.Assign("OBJECT_NAME", md.ObjectName)
		w.Assign("OBJECT_ID", md.ObjectID)
		if md.CenterName != "" {
			w.Assign("CENTER_NAME", md.CenterName)
		}
		writeFrames(&w, md.frames)
		w.Assign("TIME_SYSTEM", md.TimeSystem)

		if err := writeEpochKeyword(&w, "START_TIME", md.StartTime); err != nil {
			return nil, err
		}
		if md.UseableStartTime != nil {
			if err := writeEpochKeyword(&w, "USEABLE_START_TIME", *md.UseableStartTime); err != nil {
				return nil, err
			}
		}
		if md.UseableStopTime != nil {
			if err := writeEpochKeyword(&w, "USEABLE_STOP_TIME", *md.UseableStopTime); err != nil {
				return nil, err
			}
		}
		if err := writeEpochKeyword(&w, "STOP_TIME", md.StopTime); err != nil {
			return nil, err
		}

		w.Assign("ATTITUDE_TYPE", string(md.Type))
		if md.Type.IsEuler() {
			w.Assign("EULER_ROT_SEQ", md.RotSeq)
		}
		if md.AngVelFrame != "" {
			w.Assign("ANGVEL_FRAME", md.AngVelFrame)
		}
		if md.InterpolationMethod != "" {
			w.Assign("INTERPOLATION_METHOD", md.InterpolationMethod)
			w.Assign("INTERPOLATION_DEGREE", ndm.FormatInt(md.InterpolationDegree))
		}
		w.Section(keywordMetaStop)

		w.Blank()
		w.Section(keywordDataStart)
		w.Comments(block.Comments)
		for _, line := range block.Lines {
			epoch, err := ndm.FormatEpoch(line.Epoch, ndm.EpochPrecision(line.Epoch))
			if err != nil {
				return nil, err
			}
			fields := make([]string, 0, len(line.Values)+1)
			fields = append(fields, epoch)
			for _, v := range line.Values {
				fields = append(fields, ndm.FormatValue(v))
			}
			w.Raw(strings.Join(fields, " "))
		}
		w.Section(keywordDataStop)
	}
	return w.Bytes(), nil
}

func writeEpochKeyword(w *ndm.Writer, keyword string, t time.Time) error {
	value, err := ndm.FormatEpoch(t, ndm.EpochPrecision(t))
	if err != nil {
		return err
	}
	w.Assign(keyword, value)
	return nil
}
