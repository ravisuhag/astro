package ltp

import "fmt"

// Segment is one complete LTP segment: a header, type-specific content, and
// any trailer extensions.
//
// Exactly one of the content fields is set, chosen by the header's type.
type Segment struct {
	Header *Header

	// Data is set for the data segment types.
	Data *DataSegment
	// Report is set for TypeReport.
	Report *ReportSegment
	// ReportAck is set for TypeReportAck.
	ReportAck *ReportAckSegment
	// Cancel is set for the cancel segment types. Cancel acknowledgments have
	// no content at all, so every field stays nil for those.
	Cancel *CancelSegment
}

// Validate checks that the content matches the header's type.
func (s *Segment) Validate() error {
	if s.Header == nil {
		return ErrDataTooShort
	}
	if err := s.Header.Validate(); err != nil {
		return err
	}

	switch t := s.Header.Type; {
	case t.IsData():
		if s.Data == nil {
			return ErrWrongSegmentType
		}
	case t == TypeReport:
		if s.Report == nil {
			return ErrWrongSegmentType
		}
	case t == TypeReportAck:
		if s.ReportAck == nil {
			return ErrWrongSegmentType
		}
	case t.IsCancel():
		if s.Cancel == nil {
			return ErrWrongSegmentType
		}
	case t.IsCancelAck():
		// No content, per §3.2.5.
	default:
		return ErrUndefinedSegmentType
	}
	return nil
}

// Encode serializes the whole segment: header, content, trailer extensions.
func (s *Segment) Encode() ([]byte, error) {
	if err := s.Validate(); err != nil {
		return nil, err
	}

	out, err := s.Header.Encode()
	if err != nil {
		return nil, err
	}

	var content []byte
	switch t := s.Header.Type; {
	case t.IsData():
		content, err = s.Data.Encode(t.IsCheckpoint())
	case t == TypeReport:
		content, err = s.Report.Encode()
	case t == TypeReportAck:
		content, err = s.ReportAck.Encode()
	case t.IsCancel():
		content, err = s.Cancel.Encode()
	}
	if err != nil {
		return nil, err
	}
	out = append(out, content...)

	// §3.1.4: trailer extensions follow the content.
	for _, e := range s.Header.TrailerExtensions {
		out = append(out, e.Encode()...)
	}
	return out, nil
}

// DecodeSegment parses one complete LTP segment.
func DecodeSegment(data []byte) (*Segment, error) {
	header, offset, trailerCount, err := DecodeHeader(data)
	if err != nil {
		return nil, err
	}

	s := &Segment{Header: header}
	var consumed int

	switch t := header.Type; {
	case t.IsData():
		s.Data, consumed, err = DecodeDataSegment(data[offset:], t.IsCheckpoint())
	case t == TypeReport:
		s.Report, consumed, err = DecodeReportSegment(data[offset:])
	case t == TypeReportAck:
		s.ReportAck, consumed, err = DecodeReportAckSegment(data[offset:])
	case t.IsCancel():
		s.Cancel, consumed, err = DecodeCancelSegment(data[offset:])
	case t.IsCancelAck():
		// §3.2.5: no content.
	default:
		return nil, ErrUndefinedSegmentType
	}
	if err != nil {
		return nil, err
	}
	offset += consumed

	for i := 0; i < trailerCount; i++ {
		e, n, err := decodeExtension(data[offset:])
		if err != nil {
			return nil, err
		}
		header.TrailerExtensions = append(header.TrailerExtensions, e)
		offset += n
	}
	return s, nil
}

// Humanize returns a human-readable summary of the whole segment.
func (s *Segment) Humanize() string {
	if s.Header == nil {
		return "LTP Segment (empty)"
	}
	out := s.Header.Humanize()
	switch {
	case s.Data != nil:
		out += "\n" + s.Data.Humanize()
	case s.Report != nil:
		out += "\n" + s.Report.Humanize()
	case s.ReportAck != nil:
		out += "\n" + s.ReportAck.Humanize()
	case s.Cancel != nil:
		out += "\n" + s.Cancel.Humanize()
	}
	return out
}

// String renders a one-line description.
func (s *Segment) String() string {
	if s.Header == nil {
		return "LTP segment (empty)"
	}
	return fmt.Sprintf("LTP %s, session %s", s.Header.Type, s.Header.SessionID)
}
