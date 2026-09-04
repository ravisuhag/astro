package adm

import (
	"strings"

	"github.com/ravisuhag/astro/internal/ndm"
)

// acmSections is the section order table 5-1 fixes, which clause 5.3.1.2 makes
// mandatory, with how many times each may appear.
var acmSections = []struct {
	prefix   string
	repeated bool
}{
	{acmMeta, false},
	{acmAtt, true},
	{acmPhys, false},
	{acmCov, true},
	{acmMan, true},
	{acmAD, false},
	{acmUser, false},
}

// DecodeACM reads an Attitude Comprehensive Message in 'keyword = value'
// notation.
//
// The structure is a header, one metadata section, then any number of data
// sections in the order of table 5-1, each wrapped in its own *_START and
// *_STOP delimiters.
func DecodeACM(data []byte) (*ACM, error) {
	// Clause 6.6.1 caps an APM or AEM line at 254 characters and clause 6.6.2
	// exempts the ACM outright, the same split CCSDS 502.0-B-3 makes between
	// its first three messages and the OCM.
	s := ndm.NewScanner(data, false)

	header, err := ndm.ReadHeader(s, headerSpec("CCSDS_ACM_VERS"))
	if err != nil {
		return nil, err
	}

	m := &ACM{Header: headerFromNDM(header)}
	if err := readACMSections(s, m); err != nil {
		return nil, err
	}
	if err := m.Validate(); err != nil {
		return nil, err
	}
	return m, nil
}

// readACMSections walks the delimited sections after the header.
func readACMSections(s *ndm.Scanner, m *ACM) error {
	previous := -1
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
			// A data row outside any section has nothing to belong to.
			return ndm.At(line.Number, ErrUnexpectedDelimiter)
		}

		keyword, _, err := line.Assignment()
		if err != nil {
			return ndm.At(line.Number, err)
		}
		prefix, ok := strings.CutSuffix(keyword, "_START")
		if !ok {
			return ndm.At(line.Number, ErrUnexpectedDelimiter)
		}

		at := acmSectionRank(prefix)
		if at < 0 {
			return ndm.At(line.Number, ErrUnexpectedDelimiter)
		}
		if at < previous {
			return ndm.At(line.Number, ErrSectionsOutOfOrder)
		}
		if at == previous && !acmSections[at].repeated {
			return ndm.At(line.Number, ErrDuplicateSection)
		}
		previous = at

		if err := readACMSection(s, m, prefix, pending); err != nil {
			return err
		}
		pending = nil
	}
	return s.Err()
}

// acmSectionRank returns a section's place in table 5-1, or -1.
func acmSectionRank(prefix string) int {
	for i, section := range acmSections {
		if section.prefix == prefix {
			return i
		}
	}
	return -1
}

// readACMSection reads one section's body and files it on the message.
func readACMSection(s *ndm.Scanner, m *ACM, prefix string, comments []string) error {
	if prefix == acmUser {
		return readACMUserSection(s, m, comments)
	}

	section, err := readACMBody(s, prefix, comments)
	if err != nil {
		return err
	}

	switch prefix {
	case acmMeta:
		if len(m.Metadata.Fields) > 0 || len(m.Metadata.Comments) > 0 {
			return ErrDuplicateSection
		}
		m.Metadata = section
	case acmAtt:
		m.Attitudes = append(m.Attitudes, section)
	case acmCov:
		m.Covariances = append(m.Covariances, section)
	case acmMan:
		m.Maneuvers = append(m.Maneuvers, section)
	case acmPhys:
		m.Physical = &section
	case acmAD:
		m.AttitudeDetermination = &section
	}
	return nil
}

// readACMBody reads the keywords, comments, data rows and — in the attitude
// determination section — sensor sub-blocks, up to the section's *_STOP.
func readACMBody(s *ndm.Scanner, prefix string, comments []string) (ACMSection, error) {
	section := ACMSection{Comments: comments}
	stop := prefix + "_STOP"

	for s.Scan() {
		line := s.Line()

		switch line.Kind {
		case ndm.Blank:
			continue
		case ndm.Comment:
			section.Comments = append(section.Comments, line.Value)
			continue
		case ndm.Free:
			fields := strings.Fields(line.Text)
			if len(fields) == 0 {
				return section, ndm.At(line.Number, ErrMalformedDataRow)
			}
			section.Rows = append(section.Rows, DataRow{Fields: fields})
			continue
		}

		keyword, value, err := line.Assignment()
		if err != nil {
			return section, ndm.At(line.Number, err)
		}
		if keyword == stop {
			return section, nil
		}

		// The sensor sub-block is the one nesting the standard allows, and
		// only inside the attitude determination section (clause 5.3.9.6).
		if keyword == acmSensor+"_START" {
			if prefix != acmAD {
				return section, ndm.At(line.Number, ErrUnexpectedDelimiter)
			}
			sensor, err := readACMSensor(s)
			if err != nil {
				return section, err
			}
			section.Sensors = append(section.Sensors, sensor)
			continue
		}
		if strings.HasSuffix(keyword, "_START") || strings.HasSuffix(keyword, "_STOP") {
			return section, ndm.At(line.Number, ErrUnterminatedBlock)
		}
		// Table 5-1 puts the data rows after the keywords in every section.
		if len(section.Rows) > 0 {
			return section, ndm.At(line.Number, ErrKeywordsOutOfOrder)
		}
		section.Fields = append(section.Fields, Field{Keyword: keyword, Value: value})
	}
	if err := s.Err(); err != nil {
		return section, err
	}
	return section, ErrUnterminatedBlock
}

// readACMSensor reads one SENSOR_START to SENSOR_STOP sub-block.
func readACMSensor(s *ndm.Scanner) (ACMSensor, error) {
	var sensor ACMSensor

	for s.Scan() {
		line := s.Line()

		switch line.Kind {
		case ndm.Blank:
			continue
		case ndm.Comment:
			sensor.Comments = append(sensor.Comments, line.Value)
			continue
		case ndm.Free:
			return sensor, ndm.At(line.Number, ErrUnterminatedBlock)
		}

		keyword, value, err := line.Assignment()
		if err != nil {
			return sensor, ndm.At(line.Number, err)
		}
		if keyword == acmSensor+"_STOP" {
			return sensor, nil
		}
		if strings.HasSuffix(keyword, "_START") || strings.HasSuffix(keyword, "_STOP") {
			return sensor, ndm.At(line.Number, ErrUnterminatedBlock)
		}
		sensor.Fields = append(sensor.Fields, Field{Keyword: keyword, Value: value})
	}
	if err := s.Err(); err != nil {
		return sensor, err
	}
	return sensor, ErrUnterminatedBlock
}

// readACMUserSection reads the USER_START to USER_STOP section of table 5-9.
func readACMUserSection(s *ndm.Scanner, m *ACM, comments []string) error {
	if len(m.UserDefined) > 0 {
		return ErrDuplicateSection
	}
	m.UserComments = append(m.UserComments, comments...)

	for s.Scan() {
		line := s.Line()

		switch line.Kind {
		case ndm.Blank:
			continue
		case ndm.Comment:
			m.UserComments = append(m.UserComments, line.Value)
			continue
		case ndm.Free:
			return ndm.At(line.Number, ErrUnterminatedBlock)
		}

		keyword, value, err := line.Assignment()
		if err != nil {
			return ndm.At(line.Number, err)
		}
		if keyword == acmUser+"_STOP" {
			// Table 5-9 marks USER_DEFINED_x mandatory within the section, so
			// an empty one says nothing.
			if len(m.UserDefined) == 0 {
				return ndm.At(line.Number, ErrMissingKeyword)
			}
			return nil
		}
		name, ok := strings.CutPrefix(keyword, ndm.KeywordUserDefined+"_")
		if !ok {
			return ndm.At(line.Number, ErrUnknownKeyword)
		}
		if name == "" {
			// A bare USER_DEFINED_ names no parameter.
			return ndm.At(line.Number, ndm.ErrEmptyKeyword)
		}
		m.UserDefined = append(m.UserDefined, UserDefined{Name: name, Value: value})
	}
	if err := s.Err(); err != nil {
		return err
	}
	return ErrUnterminatedBlock
}

// Encode writes the message in 'keyword = value' notation.
//
// Values go out as they arrived. An ACM section is held as text because its
// keywords are not typed here, so there is nothing to reformat and nothing to
// lose. What this does guarantee is the structure: the sections in table 5-1's
// order, each delimited, with the keywords in the order they were read.
func (m *ACM) Encode() ([]byte, error) {
	if err := m.Validate(); err != nil {
		return nil, err
	}

	var w ndm.Writer
	if err := writeACMHeader(&w, m.Header); err != nil {
		return nil, err
	}

	writeACMSection(&w, acmMeta, m.Metadata)
	for _, section := range m.Attitudes {
		writeACMSection(&w, acmAtt, section)
	}
	if m.Physical != nil {
		writeACMSection(&w, acmPhys, *m.Physical)
	}
	for _, section := range m.Covariances {
		writeACMSection(&w, acmCov, section)
	}
	for _, section := range m.Maneuvers {
		writeACMSection(&w, acmMan, section)
	}
	if m.AttitudeDetermination != nil {
		writeACMSection(&w, acmAD, *m.AttitudeDetermination)
	}
	if len(m.UserDefined) > 0 {
		w.Section(acmUser + "_START")
		w.Comments(m.UserComments)
		for _, u := range m.UserDefined {
			w.Assign(ndm.KeywordUserDefined+"_"+u.Name, u.Value)
		}
		w.Section(acmUser + "_STOP")
	}
	return w.Bytes(), nil
}

// writeACMHeader writes the six keywords of table 5-2, in that order.
func writeACMHeader(w *ndm.Writer, h Header) error {
	w.Assign("CCSDS_ACM_VERS", h.Version)
	w.Comments(h.Comments)
	if h.Classification != "" {
		w.Assign(ndm.KeywordClassification, h.Classification)
	}
	created, err := ndm.FormatEpoch(h.CreationDate, epochPrecision(h.CreationDate))
	if err != nil {
		return err
	}
	w.Assign(ndm.KeywordCreationDate, created)
	w.Assign(ndm.KeywordOriginator, h.Originator)
	if h.MessageID != "" {
		w.Assign(ndm.KeywordMessageID, h.MessageID)
	}
	return nil
}

// writeACMSection writes one delimited section, its sensor sub-blocks
// included.
func writeACMSection(w *ndm.Writer, prefix string, section ACMSection) {
	w.Section(prefix + "_START")
	w.Comments(section.Comments)
	for _, f := range section.Fields {
		w.Assign(f.Keyword, f.Value)
	}
	for _, sensor := range section.Sensors {
		w.Section(acmSensor + "_START")
		w.Comments(sensor.Comments)
		for _, f := range sensor.Fields {
			w.Assign(f.Keyword, f.Value)
		}
		w.Section(acmSensor + "_STOP")
	}
	for _, row := range section.Rows {
		w.Raw(strings.Join(row.Fields, " "))
	}
	w.Section(prefix + "_STOP")
}
