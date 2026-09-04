package ndm

import "time"

// Header keywords shared across the navigation data messages. The version
// keyword differs per message type and is named by a HeaderSpec.
const (
	KeywordComment        = "COMMENT"
	KeywordClassification = "CLASSIFICATION"
	KeywordCreationDate   = "CREATION_DATE"
	KeywordOriginator     = "ORIGINATOR"
	KeywordMessageFor     = "MESSAGE_FOR"
	KeywordMessageID      = "MESSAGE_ID"
)

// Presence says how a standard's header table treats a keyword: not listed at
// all, listed as optional, or listed as mandatory. The four standards differ,
// which is why this is a parameter rather than a constant.
type Presence int

const (
	// Absent means the keyword is not in this standard's header table, so it
	// must not appear.
	Absent Presence = iota
	// Optional means the table lists it with status O.
	Optional
	// Mandatory means the table lists it with status M.
	Mandatory
)

// Header is the opening section of a navigation data message.
//
// The four standards do not agree on which fields belong here. CLASSIFICATION
// is in the orbit and attitude tables and in neither of the others.
// MESSAGE_FOR is in the conjunction table alone. MESSAGE_ID is optional in
// three and mandatory in the fourth. This struct carries the union; a
// HeaderSpec says which of them the message being read or written may have.
type Header struct {
	// Version is the value of the CCSDS_*_VERS keyword, in the form 'x.y'.
	Version string
	// Comments are the COMMENT lines that follow the version keyword. Every
	// standard allows them in the header only immediately after the version.
	Comments []string
	// Classification is free text, and its values are for the exchanging
	// parties to agree between themselves.
	Classification string
	// CreationDate is when the file was made. Its time system is always UTC,
	// whatever the message's TIME_SYSTEM says (clause 7.5.11).
	CreationDate time.Time
	// Originator is the creating agency or operator, drawn from the SANA
	// organizations registry.
	Originator string
	// MessageFor names the spacecraft a conjunction message is addressed to.
	MessageFor string
	// MessageID uniquely identifies a message from a given originator. Its
	// format is the originator's choice.
	MessageID string
}

// HeaderSpec describes one standard's header table.
type HeaderSpec struct {
	// VersionKeyword is the CCSDS_*_VERS keyword this message type opens with,
	// such as CCSDS_OPM_VERS.
	VersionKeyword string
	// Classification, MessageFor and MessageID say how this standard's table
	// treats those three keywords. CREATION_DATE and ORIGINATOR are mandatory
	// in all four, and COMMENT optional in all four, so neither is a parameter.
	Classification Presence
	MessageFor     Presence
	MessageID      Presence
}

// ReadHeader reads the header section.
//
// It stops at the first keyword the header table does not list and puts that
// line back, so the caller can carry on with the metadata section. A keyword
// that belongs to no section at all is therefore reported by the caller, not
// here, which is why an unknown keyword only becomes ErrUnknownHeaderKeyword
// when it appears before any header keyword this spec does allow.
func ReadHeader(s *Scanner, spec HeaderSpec) (Header, error) {
	var (
		h    Header
		seen = make(map[string]bool)
		// Clause 7.8.7 and its siblings place header comments "immediately
		// after" the version keyword. Once any other header keyword has been
		// read, a COMMENT is no longer part of the header: it is the one every
		// standard allows at the start of the metadata section. Figure G-1 of
		// CCSDS 502.0-B-3 is exactly that case, and reading it as a header
		// comment would swallow the line the next section needs.
		sawOtherKeyword bool
	)

	// Clause 7.3.6: the first header line must be the first non-blank line.
	for {
		if !s.Scan() {
			if err := s.Err(); err != nil {
				return h, err
			}
			return h, ErrNoVersionLine
		}
		if s.Line().Kind != Blank {
			break
		}
	}

	line := s.Line()
	keyword, value, err := line.Assignment()
	if err != nil {
		return h, At(line.Number, err)
	}
	if keyword != spec.VersionKeyword {
		if isVersionKeyword(keyword) {
			return h, At(line.Number, ErrWrongMessageType)
		}
		return h, At(line.Number, ErrNoVersionLine)
	}
	if value == "" {
		return h, At(line.Number, ErrEmptyValue)
	}
	h.Version = value

	for s.Scan() {
		line := s.Line()

		switch line.Kind {
		case Blank:
			continue
		case Comment:
			if sawOtherKeyword {
				s.Unread()
				return h, finishHeader(h, spec, seen)
			}
			h.Comments = append(h.Comments, line.Value)
			continue
		case Free:
			s.Unread()
			return h, finishHeader(h, spec, seen)
		}

		keyword, value, err := line.Assignment()
		if err != nil {
			return h, At(line.Number, err)
		}

		presence, field := headerField(&h, keyword, spec)
		if presence == Absent {
			// Not a header keyword for this standard. It belongs to whatever
			// section comes next.
			s.Unread()
			return h, finishHeader(h, spec, seen)
		}
		if seen[keyword] {
			return h, At(line.Number, ErrDuplicateHeaderKeyword)
		}
		seen[keyword] = true
		sawOtherKeyword = true

		if keyword == KeywordCreationDate {
			t, err := ParseEpoch(value)
			if err != nil {
				return h, At(line.Number, err)
			}
			h.CreationDate = t
			continue
		}
		if value == "" && presence == Mandatory {
			return h, At(line.Number, ErrEmptyValue)
		}
		*field = value
	}

	if err := s.Err(); err != nil {
		return h, err
	}
	return h, finishHeader(h, spec, seen)
}

// headerField maps a keyword to how this standard treats it and where its
// value goes. CREATION_DATE has no string field, so it returns a discard.
func headerField(h *Header, keyword string, spec HeaderSpec) (Presence, *string) {
	var discard string

	switch keyword {
	case KeywordCreationDate:
		return Mandatory, &discard
	case KeywordOriginator:
		return Mandatory, &h.Originator
	case KeywordClassification:
		return spec.Classification, &h.Classification
	case KeywordMessageFor:
		return spec.MessageFor, &h.MessageFor
	case KeywordMessageID:
		return spec.MessageID, &h.MessageID
	}
	return Absent, &discard
}

// finishHeader checks that every mandatory keyword turned up.
func finishHeader(h Header, spec HeaderSpec, seen map[string]bool) error {
	if !seen[KeywordCreationDate] || !seen[KeywordOriginator] {
		return ErrMissingHeaderField
	}
	if spec.MessageID == Mandatory && !seen[KeywordMessageID] {
		return ErrMissingHeaderField
	}
	if spec.MessageFor == Mandatory && !seen[KeywordMessageFor] {
		return ErrMissingHeaderField
	}
	if spec.Classification == Mandatory && !seen[KeywordClassification] {
		return ErrMissingHeaderField
	}
	_ = h
	return nil
}

// isVersionKeyword reports whether a keyword looks like some other message
// type's version line, so that handing an OPM to an OEM reader says so rather
// than complaining about a missing version.
func isVersionKeyword(keyword string) bool {
	return len(keyword) > len("CCSDS__VERS") &&
		keyword[:6] == "CCSDS_" &&
		keyword[len(keyword)-5:] == "_VERS"
}

// Write writes the header section in the order the standards' tables fix.
func (h Header) Write(w *Writer, spec HeaderSpec) error {
	if h.Version == "" || h.Originator == "" {
		return ErrMissingHeaderField
	}

	w.Assign(spec.VersionKeyword, h.Version)
	w.Comments(h.Comments)

	if h.Classification != "" {
		if spec.Classification == Absent {
			return ErrUnknownHeaderKeyword
		}
		w.Assign(KeywordClassification, h.Classification)
	}

	// Clause 7.5.11 fixes the time system for CREATION_DATE at UTC whatever
	// the rest of the message uses, so the value is converted rather than
	// written in whatever zone the caller happened to hold.
	created, err := FormatEpoch(h.CreationDate.UTC(), 0)
	if err != nil {
		return err
	}
	w.Assign(KeywordCreationDate, created)
	w.Assign(KeywordOriginator, h.Originator)

	if h.MessageFor != "" {
		if spec.MessageFor == Absent {
			return ErrUnknownHeaderKeyword
		}
		w.Assign(KeywordMessageFor, h.MessageFor)
	} else if spec.MessageFor == Mandatory {
		return ErrMissingHeaderField
	}

	if h.MessageID != "" {
		if spec.MessageID == Absent {
			return ErrUnknownHeaderKeyword
		}
		w.Assign(KeywordMessageID, h.MessageID)
	} else if spec.MessageID == Mandatory {
		return ErrMissingHeaderField
	}
	return nil
}
