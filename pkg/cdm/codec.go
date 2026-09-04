package cdm

import (
	"github.com/ravisuhag/astro/internal/ndm"
)

// Decode reads a Conjunction Data Message in 'keyword = value' notation.
//
// The KVN form has no delimiters. What separates the sections is the OBJECT
// keyword: everything before the first one is relative metadata and data, and
// everything after an OBJECT belongs to the object it names.
func Decode(data []byte) (*CDM, error) {
	// Clause 6.3.1 caps a line at 254 characters, as the other navigation
	// messages do.
	s := ndm.NewScanner(data, true)

	header, err := ndm.ReadHeader(s, headerSpec)
	if err != nil {
		return nil, err
	}

	m := &CDM{Header: Header{
		Version:      header.Version,
		Comments:     header.Comments,
		CreationDate: header.CreationDate,
		Originator:   header.Originator,
		MessageFor:   header.MessageFor,
		MessageID:    header.MessageID,
	}}
	if err := readBody(s, m); err != nil {
		return nil, err
	}
	if err := m.Validate(); err != nil {
		return nil, err
	}
	return m, nil
}

func readBody(s *ndm.Scanner, m *CDM) error {
	var (
		pending []string
		// current is -1 while reading the relative section, then 0 or 1.
		current = -1
		seen    = [3]map[string]bool{{}, {}, {}}
		named   [2]bool
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

		if keyword == "OBJECT" {
			index, err := objectIndex(value)
			if err != nil {
				return ndm.At(line.Number, err)
			}
			if named[index] {
				return ndm.At(line.Number, ErrObjectRepeated)
			}
			named[index] = true
			current = index

			object := &m.Objects[index]
			object.Comments = append(object.Comments, pending...)
			pending = nil
			object.Fields = append(object.Fields, Field{Keyword: keyword, Value: value})
			continue
		}

		if err := checkPlacement(current, keyword); err != nil {
			return ndm.At(line.Number, err)
		}
		target := m.section(current)
		bucket := seen[current+1]
		if bucket[keyword] {
			return ndm.At(line.Number, ErrDuplicateKeyword)
		}
		bucket[keyword] = true

		target.Comments = append(target.Comments, pending...)
		pending = nil
		target.Fields = append(target.Fields, Field{Keyword: keyword, Value: value})
	}
	if err := s.Err(); err != nil {
		return err
	}
	if !named[0] || !named[1] {
		return ErrMissingObject
	}
	return nil
}

// section returns the section a keyword at this point belongs to.
func (m *CDM) section(current int) *Section {
	switch current {
	case -1:
		return &m.Relative
	case 0, 1:
		return &m.Objects[current].Section
	}
	return nil
}

// checkPlacement reports whether a keyword may appear where it appeared.
//
// The relative section takes table 3-2 and an object section takes tables 3-3
// through 3-8. Keeping them apart is what catches a state vector written
// before any OBJECT keyword: read loosely, it would become a property of the
// conjunction rather than of an object, and the message would look complete.
//
// An object keyword in the relative section gets its own error rather than
// "unknown keyword", because it is a real keyword in the wrong place and
// saying so is the difference between a five-minute fix and an afternoon.
func checkPlacement(current int, keyword string) error {
	inRelative := relativeKeywords[keyword]
	inObject := objectMetadataKeywords[keyword] || objectDataKeywords[keyword]

	if current == -1 {
		switch {
		case inRelative:
			return nil
		case inObject:
			return ErrObjectOutOfOrder
		}
		return ErrUnknownKeyword
	}
	if inObject {
		return nil
	}
	return ErrUnknownKeyword
}

// objectIndex reads an OBJECT value.
func objectIndex(value string) (int, error) {
	switch value {
	case "OBJECT1":
		return 0, nil
	case "OBJECT2":
		return 1, nil
	}
	return 0, ErrObjectValue
}

// Encode writes the message in 'keyword = value' notation.
//
// Keywords go out in the order they were read, which for a decoded message is
// the order the standard fixes (clause 6.3.1.9 and the note under 3.1.1). A
// message built by hand goes out in the order its fields were appended, so a
// producer that wants the standard's order appends in it.
func (m *CDM) Encode() ([]byte, error) {
	if err := m.Validate(); err != nil {
		return nil, err
	}

	var w ndm.Writer
	header := ndm.Header{
		Version:      m.Header.Version,
		Comments:     m.Header.Comments,
		CreationDate: m.Header.CreationDate,
		Originator:   m.Header.Originator,
		MessageFor:   m.Header.MessageFor,
		MessageID:    m.Header.MessageID,
	}
	if err := header.Write(&w, headerSpec); err != nil {
		return nil, err
	}

	writeSection(&w, m.Relative)
	for i := range m.Objects {
		w.Blank()
		writeSection(&w, m.Objects[i].Section)
	}
	return w.Bytes(), nil
}

func writeSection(w *ndm.Writer, s Section) {
	w.Comments(s.Comments)
	for _, f := range s.Fields {
		w.Assign(f.Keyword, f.Value)
	}
}
