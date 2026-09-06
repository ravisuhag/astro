package tdm

import (
	"strings"

	"github.com/ravisuhag/astro/internal/ndm"
)

// Decode reads a Tracking Data Message in 'keyword = value' notation.
func Decode(data []byte) (*TDM, error) {
	// Clause 4.2.2 caps a TDM line at 254 characters, as the ODM does.
	s := ndm.NewScanner(data, true)

	header, err := ndm.ReadHeader(s, headerSpec)
	if err != nil {
		return nil, err
	}

	m := &TDM{Header: Header{
		Version:      header.Version,
		Comments:     header.Comments,
		CreationDate: header.CreationDate,
		Originator:   header.Originator,
		MessageID:    header.MessageID,
	}}
	if err := readSegments(s, m); err != nil {
		return nil, err
	}
	if err := m.Validate(); err != nil {
		return nil, err
	}
	return m, nil
}

// readSegments walks the body: metadata section, data section, repeat.
func readSegments(s *ndm.Scanner, m *TDM) error {
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

		segment := Segment{Metadata: Metadata{Comments: pending}}
		pending = nil
		if err := readMetadata(s, &segment.Metadata); err != nil {
			return err
		}
		if err := readDataSection(s, &segment); err != nil {
			return err
		}
		m.Segments = append(m.Segments, segment)
	}
	return s.Err()
}

// readMetadata reads between META_START and META_STOP.
func readMetadata(s *ndm.Scanner, md *Metadata) error {
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
		if !isMetadataKeyword(keyword) {
			return ndm.At(line.Number, ErrUnknownKeyword)
		}
		if seen[keyword] {
			return ndm.At(line.Number, ErrDuplicateKeyword)
		}
		seen[keyword] = true

		if index, ok := participantIndex(keyword); ok && (index < 1 || index > 5) {
			return ndm.At(line.Number, ErrParticipantIndex)
		}
		md.Fields = append(md.Fields, Field{Keyword: keyword, Value: value})
	}
	if err := s.Err(); err != nil {
		return err
	}
	return ErrUnterminatedBlock
}

// readDataSection reads between DATA_START and DATA_STOP.
//
// Clause 3.3.1.3 pairs a data section with the metadata section before it, so
// a metadata section with nothing after it is an error rather than an empty
// segment.
func readDataSection(s *ndm.Scanner, segment *Segment) error {
	started := false

	for s.Scan() {
		line := s.Line()

		switch line.Kind {
		case ndm.Blank:
			continue
		case ndm.Comment:
			segment.Comments = append(segment.Comments, line.Value)
			continue
		case ndm.Free:
			return ndm.At(line.Number, ErrMalformedRecord)
		}

		keyword, value, err := line.Assignment()
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
		case keywordMetaStart, keywordMetaStop:
			return ndm.At(line.Number, ErrMissingDataSection)
		}

		if !started {
			return ndm.At(line.Number, ErrMissingDataSection)
		}
		if !isDataKeyword(keyword) {
			return ndm.At(line.Number, ErrUnknownKeyword)
		}

		obs, err := parseRecord(keyword, value)
		if err != nil {
			return ndm.At(line.Number, err)
		}
		segment.Observations = append(segment.Observations, obs)
	}
	if err := s.Err(); err != nil {
		return err
	}
	if started {
		return ErrUnterminatedBlock
	}
	return ErrMissingDataSection
}

// parseRecord reads the value of a Tracking Data Record: a timetag and one
// measurement, separated by at least one blank (clauses 3.4.3 and 3.4.4).
func parseRecord(keyword, value string) (Observation, error) {
	fields := strings.Fields(value)
	if len(fields) != 2 {
		return Observation{}, ErrMalformedRecord
	}

	epoch, err := ndm.ParseEpoch(fields[0])
	if err != nil {
		return Observation{}, err
	}
	measurement, err := ndm.ParseFloat(fields[1])
	if err != nil {
		return Observation{}, err
	}
	return Observation{Keyword: keyword, Epoch: epoch, Value: measurement}, nil
}

// Encode writes the message in 'keyword = value' notation.
func (m *TDM) Encode() ([]byte, error) {
	if err := m.Validate(); err != nil {
		return nil, err
	}

	var w ndm.Writer
	header := ndm.Header{
		Version:      m.Header.Version,
		Comments:     m.Header.Comments,
		CreationDate: m.Header.CreationDate,
		Originator:   m.Header.Originator,
		MessageID:    m.Header.MessageID,
	}
	if err := header.Write(&w, headerSpec); err != nil {
		return nil, err
	}

	for i := range m.Segments {
		segment := &m.Segments[i]
		w.Blank()

		w.Section(keywordMetaStart)
		w.Comments(segment.Metadata.Comments)
		for _, f := range segment.Metadata.Fields {
			w.Assign(f.Keyword, f.Value)
		}
		w.Section(keywordMetaStop)

		w.Blank()
		w.Section(keywordDataStart)
		w.Comments(segment.Comments)
		for _, obs := range segment.Observations {
			epoch, err := ndm.FormatEpoch(obs.Epoch, ndm.EpochPrecision(obs.Epoch))
			if err != nil {
				return nil, err
			}
			w.Assign(obs.Keyword, epoch+" "+ndm.FormatValue(obs.Value))
		}
		w.Section(keywordDataStop)
	}
	return w.Bytes(), nil
}
