package csts

import "github.com/ravisuhag/astro/internal/ber"

// The common operations of CCSDS 921.1-B-2 clause 3 and annex F3.4.
//
// Four of them are the standard header and nothing else — START and STOP
// returns, the GET return, the PROCESS-DATA return — because a confirmed
// operation's answer is a result and a diagnostic, and the header carries
// both. The invocations that add fields add few.

// StartInvocation begins a procedure (clause 3.7). The framework's own START
// carries nothing but the header; a procedure extends it through the Extended
// field, which this package writes as 'notUsed'.
type StartInvocation struct {
	Header InvocationHeader
}

// StopInvocation ends a procedure (clause 3.8).
type StopInvocation struct {
	Header InvocationHeader
}

// GetInvocation asks for parameter values (clause 3.12).
//
// The list of parameters is a CHOICE with eight alternatives — a named list, a
// functional resource type or name, a procedure type or name, explicit labels,
// explicit names, or empty for the default list. This package carries the
// encoded choice rather than modelling all eight, because six of them are
// built from SANA-registered identifiers whose meaning is not in this
// document.
type GetInvocation struct {
	Header InvocationHeader
	// ListOfParameters is the complete encoded ListOfParametersEvents CHOICE
	// element: tag, length and content. encode appends it verbatim, so it
	// must carry its own tag and length rather than content alone.
	ListOfParameters []byte
}

// NotifyInvocation reports an event (clause 3.11).
type NotifyInvocation struct {
	Header InvocationHeader
	// EventTime is the complete encoded Time CHOICE element (tag, length and
	// content): a CCSDS day-segmented time code in either the millisecond or
	// the picosecond resolution. encode appends it verbatim.
	EventTime []byte
	// EventName is the complete encoded Name element, which pairs a
	// functional resource or procedure with a published identifier. encode
	// appends it verbatim.
	EventName []byte
	// EventValue is the complete encoded EventValue CHOICE element. encode
	// appends it verbatim.
	EventValue []byte
}

// TransferDataInvocation moves one data unit from provider to user
// (clause 3.9).
type TransferDataInvocation struct {
	Header InvocationHeader
	// GenerationTime is the complete encoded Time CHOICE element (tag,
	// length and content). encode appends it verbatim.
	GenerationTime  []byte
	SequenceCounter uint32
	// Data is the complete encoded AbstractChoice element: an opaque octet
	// string, or a complex type a procedure defines. encode appends it
	// verbatim.
	Data []byte
}

// ProcessDataInvocation moves one data unit from user to provider
// (clause 3.10).
type ProcessDataInvocation struct {
	Header InvocationHeader
	// DataUnitID identifies the unit, so a confirmed PROCESS-DATA's return
	// can name what it is answering about.
	DataUnitID uint32
	// Data is the complete encoded AbstractChoice element (tag, length and
	// content). encode appends it verbatim.
	Data []byte
}

// StandardReturn is any of the operation responses that annex F3.4 defines as
// the standard return header alone: StartReturn, StopReturn, GetReturn,
// ProcessDataReturn, ExecuteDirectiveReturn and ExecuteDirectiveAcknowledge.
//
// They are one Go type because they are one ASN.1 type. Which operation a
// given one answers is the framework PDU tag, not anything inside it — and
// clause 3.3.1.3's note says the same of telling an acknowledgement from a
// return.
type StandardReturn struct {
	Header ReturnHeader
}

// encode writes the header's fields without a SEQUENCE around them. Each of
// these alternatives is defined as 'X ::= StandardReturnHeader', so the
// context tag of annex F3.15 replaces the header's SEQUENCE tag.
func (s *StandardReturn) encode() ([]byte, error) {
	return appendReturnHeaderContent(nil, s.Header)
}

func decodeStandardReturn(content []byte) (*StandardReturn, error) {
	h, err := decodeReturnHeaderContent(content)
	if err != nil {
		return nil, err
	}
	return &StandardReturn{Header: h}, nil
}

// headerOnlyInvocation encodes the invocations that carry a header and an
// unused extension: START and STOP.
func headerOnlyInvocation(h InvocationHeader) ([]byte, error) {
	content, err := appendInvocationHeader(nil, h)
	if err != nil {
		return nil, err
	}
	return appendExtendedNotUsed(content), nil
}

func decodeHeaderOnlyInvocation(content []byte) (InvocationHeader, error) {
	e, err := ber.NewDecoder(content).Next()
	if err != nil {
		return InvocationHeader{}, err
	}
	return decodeInvocationHeader(e)
}

func (s *StartInvocation) encode() ([]byte, error) { return headerOnlyInvocation(s.Header) }
func (s *StopInvocation) encode() ([]byte, error)  { return headerOnlyInvocation(s.Header) }

func (g *GetInvocation) encode() ([]byte, error) {
	if len(g.ListOfParameters) == 0 {
		return nil, ErrMissingField
	}
	content, err := appendInvocationHeader(nil, g.Header)
	if err != nil {
		return nil, err
	}
	content = append(content, g.ListOfParameters...)
	return appendExtendedNotUsed(content), nil
}

func decodeGetInvocation(content []byte) (*GetInvocation, error) {
	g := &GetInvocation{}
	d := ber.NewDecoder(content)

	header, err := d.Next()
	if err != nil {
		return nil, err
	}
	if g.Header, err = decodeInvocationHeader(header); err != nil {
		return nil, err
	}

	list, err := d.Next()
	if err != nil {
		return nil, err
	}
	g.ListOfParameters = list.Raw()
	return g, nil
}

func (n *NotifyInvocation) encode() ([]byte, error) {
	if len(n.EventTime) == 0 || len(n.EventName) == 0 || len(n.EventValue) == 0 {
		return nil, ErrMissingField
	}
	content, err := appendInvocationHeader(nil, n.Header)
	if err != nil {
		return nil, err
	}
	content = append(content, n.EventTime...)
	content = append(content, n.EventName...)
	content = append(content, n.EventValue...)
	return appendExtendedNotUsed(content), nil
}

func decodeNotifyInvocation(content []byte) (*NotifyInvocation, error) {
	n := &NotifyInvocation{}
	d := ber.NewDecoder(content)

	header, err := d.Next()
	if err != nil {
		return nil, err
	}
	if n.Header, err = decodeInvocationHeader(header); err != nil {
		return nil, err
	}

	for _, into := range []*[]byte{&n.EventTime, &n.EventName, &n.EventValue} {
		element, err := d.Next()
		if err != nil {
			return nil, err
		}
		*into = element.Raw()
	}
	return n, nil
}

func (t *TransferDataInvocation) encode() ([]byte, error) {
	if len(t.GenerationTime) == 0 || len(t.Data) == 0 {
		return nil, ErrMissingField
	}
	content, err := appendInvocationHeader(nil, t.Header)
	if err != nil {
		return nil, err
	}
	content = append(content, t.GenerationTime...)
	content = ber.AppendInteger(content, int64(t.SequenceCounter))
	content = append(content, t.Data...)
	return appendExtendedNotUsed(content), nil
}

func decodeTransferDataInvocation(content []byte) (*TransferDataInvocation, error) {
	t := &TransferDataInvocation{}
	d := ber.NewDecoder(content)

	header, err := d.Next()
	if err != nil {
		return nil, err
	}
	if t.Header, err = decodeInvocationHeader(header); err != nil {
		return nil, err
	}

	generation, err := d.Next()
	if err != nil {
		return nil, err
	}
	t.GenerationTime = generation.Raw()

	counter, err := d.Next()
	if err != nil {
		return nil, err
	}
	value, err := counter.Uint64()
	if err != nil {
		return nil, err
	}
	if value > 0xFFFFFFFF {
		return nil, ErrIntegerRange
	}
	t.SequenceCounter = uint32(value)

	data, err := d.Next()
	if err != nil {
		return nil, err
	}
	t.Data = data.Raw()
	return t, nil
}

func (p *ProcessDataInvocation) encode() ([]byte, error) {
	if len(p.Data) == 0 {
		return nil, ErrMissingField
	}
	content, err := appendInvocationHeader(nil, p.Header)
	if err != nil {
		return nil, err
	}
	content = ber.AppendInteger(content, int64(p.DataUnitID))
	content = append(content, p.Data...)
	return appendExtendedNotUsed(content), nil
}

func decodeProcessDataInvocation(content []byte) (*ProcessDataInvocation, error) {
	p := &ProcessDataInvocation{}
	d := ber.NewDecoder(content)

	header, err := d.Next()
	if err != nil {
		return nil, err
	}
	if p.Header, err = decodeInvocationHeader(header); err != nil {
		return nil, err
	}

	id, err := d.Next()
	if err != nil {
		return nil, err
	}
	value, err := id.Uint64()
	if err != nil {
		return nil, err
	}
	if value > 0xFFFFFFFF {
		return nil, ErrIntegerRange
	}
	p.DataUnitID = uint32(value)

	data, err := d.Next()
	if err != nil {
		return nil, err
	}
	p.Data = data.Raw()
	return p, nil
}
