package odm

import (
	"strings"

	"github.com/ravisuhag/astro/internal/ndm"
)

// ocmHeaderSpec is how table 6-2 treats the shared header keywords.
var ocmHeaderSpec = ndm.HeaderSpec{
	VersionKeyword: "CCSDS_OCM_VERS",
	Classification: ndm.Optional,
	MessageFor:     ndm.Absent,
	MessageID:      ndm.Optional,
}

// ocmSections is the section order table 6-1 fixes, with how many times each
// may appear. A section that arrives out of this order, or a second copy of
// one the standard allows once, is refused.
var ocmSections = []struct {
	prefix   string
	repeated bool
}{
	{ocmMeta, false},
	{ocmTraj, true},
	{ocmPhys, false},
	{ocmCov, true},
	{ocmMan, true},
	{ocmPert, false},
	{ocmOD, false},
	{ocmUser, false},
}

// DecodeOCM reads an Orbit Comprehensive Message in 'keyword = value'
// notation.
//
// The structure is a header, one metadata section, then any number of data
// sections in the order of table 6-1, each wrapped in its own *_START and
// *_STOP delimiters. A message with no data sections at all is well formed —
// see the note on OCM.
func DecodeOCM(data []byte) (*OCM, error) {
	// Clause 7.3.3 exempts the OCM from the 254-character line limit that
	// clause 7.3.2 puts on the other three. A covariance row alone can run
	// past it: a 6x6 lower triangle is 21 numbers on one line, and
	// clause 6.2.7.12 requires them all on one line.
	s := ndm.NewScanner(data, false)

	header, err := ndm.ReadHeader(s, ocmHeaderSpec)
	if err != nil {
		return nil, err
	}

	m := &OCM{Header: headerFromNDM(header)}
	if err := readOCMSections(s, m); err != nil {
		return nil, err
	}
	if err := m.Validate(); err != nil {
		return nil, err
	}
	return m, nil
}

// readOCMSections walks the delimited sections after the header.
func readOCMSections(s *ndm.Scanner, m *OCM) error {
	// Where in table 6-1 the last section sat. A section may repeat only if
	// its table 6-1 row says so, so an equal rank is allowed for those and
	// refused for the rest.
	previous := -1
	var pending []string

	for s.Scan() {
		line := s.Line()

		switch line.Kind {
		case ndm.Blank:
			continue
		case ndm.Comment:
			// A comment between two sections. It belongs to the one about to
			// open, which is where every OCM example puts it.
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

		at := ocmSectionRank(prefix)
		if at < 0 {
			return ndm.At(line.Number, ErrUnexpectedDelimiter)
		}
		if at < previous {
			return ndm.At(line.Number, ErrSectionsOutOfOrder)
		}
		if at == previous && !ocmSections[at].repeated {
			return ndm.At(line.Number, ErrDuplicateSection)
		}
		previous = at

		if err := readOCMSection(s, m, prefix, pending); err != nil {
			return err
		}
		pending = nil
	}
	return s.Err()
}

// ocmSectionRank returns a section's place in table 6-1, or -1.
func ocmSectionRank(prefix string) int {
	for i, section := range ocmSections {
		if section.prefix == prefix {
			return i
		}
	}
	return -1
}

// readOCMSection reads one section's body and files it on the message.
func readOCMSection(s *ndm.Scanner, m *OCM, prefix string, comments []string) error {
	if prefix == ocmUser {
		return readOCMUserSection(s, m, comments)
	}

	section, err := readOCMBody(s, prefix, comments)
	if err != nil {
		return err
	}

	switch prefix {
	case ocmMeta:
		if len(m.Metadata.Fields) > 0 || len(m.Metadata.Comments) > 0 {
			return ErrDuplicateSection
		}
		m.Metadata = section
	case ocmTraj:
		m.Trajectories = append(m.Trajectories, section)
	case ocmCov:
		m.Covariances = append(m.Covariances, section)
	case ocmMan:
		m.Maneuvers = append(m.Maneuvers, section)
	case ocmPhys:
		m.Physical = &section
	case ocmPert:
		m.Perturbations = &section
	case ocmOD:
		m.OrbitDetermination = &section
	}
	return nil
}

// readOCMBody reads the keywords, comments and data rows up to the section's
// *_STOP delimiter.
func readOCMBody(s *ndm.Scanner, prefix string, comments []string) (OCMSection, error) {
	section := OCMSection{Comments: comments}
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
		if strings.HasSuffix(keyword, "_START") || strings.HasSuffix(keyword, "_STOP") {
			// Another section opened, or a mismatched close. Either way this
			// one was never terminated.
			return section, ndm.At(line.Number, ErrUnterminatedSection)
		}
		// Table 6-1 puts the data rows last in every section, and
		// clause 6.2.2.1 fixes the keyword order, so a keyword after a row is
		// out of place.
		if len(section.Rows) > 0 {
			return section, ndm.At(line.Number, ErrKeywordsOutOfOrder)
		}
		section.Fields = append(section.Fields, Field{Keyword: keyword, Value: value})
	}
	if err := s.Err(); err != nil {
		return section, err
	}
	return section, ErrUnterminatedSection
}

// readOCMUserSection reads the USER_START to USER_STOP section of table 6-12.
//
// It is the one section whose keywords are not from a table: every one carries
// a USER_DEFINED_ prefix and the rest of the keyword is the parameter's name.
func readOCMUserSection(s *ndm.Scanner, m *OCM, comments []string) error {
	if len(m.UserDefined) > 0 {
		return ErrDuplicateSection
	}
	if m.UserComments == nil {
		m.UserComments = comments
	}

	for s.Scan() {
		line := s.Line()

		switch line.Kind {
		case ndm.Blank:
			continue
		case ndm.Comment:
			m.UserComments = append(m.UserComments, line.Value)
			continue
		case ndm.Free:
			return ndm.At(line.Number, ErrUnterminatedSection)
		}

		keyword, value, err := line.Assignment()
		if err != nil {
			return ndm.At(line.Number, err)
		}
		if keyword == ocmUser+"_STOP" {
			// Table 6-12 marks USER_DEFINED_x mandatory, so an empty section
			// says nothing and is not what the table describes.
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
	return ErrUnterminatedSection
}

// Encode writes the message in 'keyword = value' notation.
//
// Values go out as they arrived. An OCM section is held as text because its
// keywords are not typed here — over two hundred of them, most drawn from the
// SANA registry — so there is nothing to reformat and nothing to lose. What
// this does guarantee is the structure: the sections in table 6-1's order,
// each delimited, with the keywords in the order they were read.
func (m *OCM) Encode() ([]byte, error) {
	if err := m.Validate(); err != nil {
		return nil, err
	}

	var w ndm.Writer
	if err := writeOCMHeader(&w, m.Header); err != nil {
		return nil, err
	}

	writeOCMSection(&w, ocmMeta, m.Metadata)
	for _, section := range m.Trajectories {
		writeOCMSection(&w, ocmTraj, section)
	}
	if m.Physical != nil {
		writeOCMSection(&w, ocmPhys, *m.Physical)
	}
	for _, section := range m.Covariances {
		writeOCMSection(&w, ocmCov, section)
	}
	for _, section := range m.Maneuvers {
		writeOCMSection(&w, ocmMan, section)
	}
	if m.Perturbations != nil {
		writeOCMSection(&w, ocmPert, *m.Perturbations)
	}
	if m.OrbitDetermination != nil {
		writeOCMSection(&w, ocmOD, *m.OrbitDetermination)
	}
	if len(m.UserDefined) > 0 {
		w.Section(ocmUser + "_START")
		w.Comments(m.UserComments)
		for _, u := range m.UserDefined {
			w.Assign(ndm.KeywordUserDefined+"_"+u.Name, u.Value)
		}
		w.Section(ocmUser + "_STOP")
	}
	return w.Bytes(), nil
}

// writeOCMHeader writes the six keywords of table 6-2, in that order.
func writeOCMHeader(w *ndm.Writer, h Header) error {
	w.Assign(ocmHeaderSpec.VersionKeyword, h.Version)
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

// writeOCMSection writes one delimited section.
func writeOCMSection(w *ndm.Writer, prefix string, section OCMSection) {
	w.Section(prefix + "_START")
	w.Comments(section.Comments)
	for _, f := range section.Fields {
		w.Assign(f.Keyword, f.Value)
	}
	for _, row := range section.Rows {
		w.Raw(strings.Join(row.Fields, " "))
	}
	w.Section(prefix + "_STOP")
}
