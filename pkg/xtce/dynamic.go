package xtce

import (
	"bytes"
	"fmt"
)

// Layouts that depend on the packet.
//
// Layout settles a container's fields once, ahead of any packet, which is
// what makes it cheap: build it per packet type and reuse it forever. Most
// containers work that way, because most missions fix every field's width in
// the database.
//
// Some do not. XTCE lets a container's shape depend on its own contents:
//
//   - a string delimited by a terminator, or prefixed with its own length
//   - a binary field whose width another parameter states
//   - an entry that repeats a number of times the packet decides
//   - an entry positioned by a value read from the packet
//   - an entry positioned relative to the end of the container
//
// None of those can be settled before the packet arrives, so Layout refuses
// them with ErrDynamicSize rather than guessing. ResolveLayout is the other
// path: it walks the packet and the container together, decoding each field
// as it places it so that a later field can be sized or positioned by an
// earlier one's value.
//
// The result is an ordinary Layout, valid for that one packet and no other.
// That is the trade: correctness for a packet-shaped container costs a walk
// per packet, where a fixed container costs a walk per packet type.

// ResolveLayout builds the layout of this container for one specific packet.
//
// Use it when Layout returns ErrDynamicSize. For a container whose fields are
// all fixed it produces the same answer Layout does, so a caller that does
// not know which kind it has can use this and pay the extra walk.
//
// The layout it returns describes this packet only. A field's offset or width
// may differ in the next one, which is the whole point.
func (c *SequenceContainer) ResolveLayout(packet []byte) (*Layout, error) {
	if c.owner == nil {
		return nil, fmt.Errorf(
			"%w: the container did not come from Load, so its references cannot resolve",
			ErrUnresolvedReference)
	}

	layout := &Layout{Container: c}

	resolver := &layoutResolver{
		layout: layout,
		packet: packet,
		values: make(map[*Parameter]Value),
	}
	if err := resolver.container(c, 0); err != nil {
		return nil, err
	}
	return layout, nil
}

// ResolveLayoutOf builds the named container's layout for one packet.
func (s *SpaceSystem) ResolveLayoutOf(qualifiedName string, packet []byte) (*Layout, error) {
	container, err := s.FindContainer(qualifiedName)
	if err != nil {
		return nil, err
	}
	return container.ResolveLayout(packet)
}

// ExtractDynamic resolves the layout against a packet and extracts it.
//
// It is ResolveLayout followed by Extract, which is what a caller wanting the
// values rather than the shape would write.
func (c *SequenceContainer) ExtractDynamic(packet []byte) (*Packet, error) {
	layout, err := c.ResolveLayout(packet)
	if err != nil {
		return nil, err
	}
	return layout.Extract(packet)
}

// layoutResolver walks a container and a packet together.
//
// It is the same walk layoutBuilder does, with two additions: it decodes each
// field as it is placed, and it consults those decoded values when a width,
// a repeat count or a position is not in the database.
type layoutResolver struct {
	layout *Layout
	packet []byte

	// cursor is the bit just past the last entry placed.
	cursor uint

	// seen guards against a container splicing itself.
	seen []*SequenceContainer

	// values holds what has been decoded so far, so a DynamicValue can read
	// it. Keyed by parameter rather than by name because two SpaceSystems can
	// each define a parameter of the same name.
	values map[*Parameter]Value
}

// container appends the fields of one container, base entries first.
func (r *layoutResolver) container(c *SequenceContainer, depth int) error {
	if depth > maxSpliceDepth {
		return fmt.Errorf("%w: containers nest more than %d deep from %s",
			ErrContainerCycle, maxSpliceDepth, r.layout.Container.Name)
	}
	for _, previous := range r.seen {
		if previous == c {
			return fmt.Errorf("%w: container %q includes itself", ErrContainerCycle, c.Name)
		}
	}
	r.seen = append(r.seen, c)
	defer func() { r.seen = r.seen[:len(r.seen)-1] }()

	// Inherited entries first, as in a static layout: the base describes the
	// header and each derived container adds its body after it.
	if c.BaseContainer != nil {
		base, err := c.owner.ResolveContainer(c.BaseContainer.ContainerRef)
		if err != nil {
			return fmt.Errorf("container %q: %w", c.Name, err)
		}
		if err := r.container(base, depth+1); err != nil {
			return err
		}
	}

	containerStart := r.cursor

	for _, entry := range c.EntryList.Entries {
		if err := r.entry(c, entry, containerStart, depth); err != nil {
			return err
		}
	}
	return nil
}

// entry places one entry, repeated as many times as it says.
func (r *layoutResolver) entry(c *SequenceContainer, entry Entry, containerStart uint, depth int) error {
	count, err := r.repeatCount(c, entry)
	if err != nil {
		return fmt.Errorf("entry %s of container %q: %w", entry, c.Name, err)
	}

	for range count {
		start, err := r.place(c, entry, containerStart)
		if err != nil {
			return fmt.Errorf("entry %s of container %q: %w", entry, c.Name, err)
		}
		r.cursor = start

		switch entry.Kind {
		case EntryParameterRef:
			if err := r.parameter(c, entry); err != nil {
				return err
			}

		case EntryContainerRef:
			spliced, err := c.owner.ResolveContainer(entry.Ref)
			if err != nil {
				return fmt.Errorf("entry %s of container %q: %w", entry, c.Name, err)
			}
			if err := r.container(spliced, depth+1); err != nil {
				return err
			}

		default:
			// An entry kind this package does not model. Its width is
			// unknown, so everything after it would be misplaced.
			return fmt.Errorf("%w: entry %s of container %q", ErrUnsupportedEntry, entry, c.Name)
		}
	}
	return nil
}

// parameter places one parameter and decodes it.
//
// The decode is what makes the walk work: a later field's width or position
// may name this parameter, and only a decoded value can answer that.
func (r *layoutResolver) parameter(c *SequenceContainer, entry Entry) error {
	if len(r.layout.Fields) >= MaxFields {
		return fmt.Errorf("%w: the layout has grown past %d fields", ErrUnsupportedEntry, MaxFields)
	}

	param, err := c.owner.ResolveParameter(entry.Ref)
	if err != nil {
		return fmt.Errorf("entry %s of container %q: %w", entry, c.Name, err)
	}

	paramType, err := c.owner.ResolveParameterType(param.ParameterTypeRef)
	if err != nil {
		return fmt.Errorf("parameter %q of container %q: %w", param.Name, c.Name, err)
	}

	size, err := r.fieldWidth(c, param, paramType)
	if err != nil {
		return err
	}

	field := Field{
		Name:      qualify(c.owner, param.Name),
		Parameter: param,
		Type:      paramType,
		BitOffset: r.cursor,
		BitSize:   size,
		Container: c,
		Entry:     entry,
	}
	r.layout.Fields = append(r.layout.Fields, field)

	// Decode it now, so a later dynamic reference can read it. A field that
	// will not decode is not fatal here: Extract reports per-field errors,
	// and only a field another field depends on actually blocks the walk.
	//
	// Written in subtraction form rather than field.BitOffset+field.BitSize
	// >= packetBits: a width taken from the packet (a dynamically-sized
	// binary field) is otherwise unchecked here, and an addition that
	// overflows could wrap past a small packet's bit length and make this
	// look safe when it is not.
	packetBits := uint(len(r.packet)) * 8
	if field.BitOffset <= packetBits && field.BitSize <= packetBits-field.BitOffset {
		r.values[param] = extractField(bitReader{data: r.packet}, field)
	}

	r.cursor += size
	if r.cursor > r.layout.BitSize {
		r.layout.BitSize = r.cursor
	}
	return nil
}

// fieldWidth works out how wide a field is, reading the packet when the
// database does not say.
func (r *layoutResolver) fieldWidth(c *SequenceContainer, param *Parameter, paramType ParameterType) (uint, error) {
	encoding := paramType.Encoding()

	// The fixed case, which is most of them.
	if size, ok := encoding.SizeInBits(); ok {
		return size, nil
	}

	switch {
	case encoding == nil:
		return 0, fmt.Errorf("%w: parameter %q has no data encoding",
			ErrDynamicSize, param.Name)

	case encoding.String != nil:
		return r.stringWidth(param, encoding.String)

	case encoding.Binary != nil && encoding.Binary.SizeInBits != nil:
		bits, err := r.integerValue(c, encoding.Binary.SizeInBits,
			fmt.Sprintf("the width of binary parameter %q", param.Name))
		if err != nil {
			return 0, err
		}
		if bits < 0 {
			return 0, fmt.Errorf("%w: parameter %q was given a width of %d bits",
				ErrDynamicSize, param.Name, bits)
		}
		// An integer or float field is capped at 64 bits by bitReader.read.
		// A binary field has no such fixed cap -- a blob can legitimately be
		// far wider -- so instead it is capped against the one packet it is
		// being sized from. Two such fields side by side could otherwise
		// each carry a huge, non-overflowing width whose sum still overflows
		// the cursor that places the next entry.
		packetBits := uint64(len(r.packet)) * 8
		if uint64(bits) > packetBits {
			return 0, fmt.Errorf("%w: parameter %q was given a width of %d bits, more than the %d-bit packet holds",
				ErrPacketTooShort, param.Name, bits, packetBits)
		}
		return uint(bits), nil

	default:
		return 0, fmt.Errorf("%w: parameter %q is encoded with a width the database does not state",
			ErrDynamicSize, param.Name)
	}
}

// stringWidth measures a string whose width the packet decides.
//
// The schema allows three forms. Fixed is handled by SizeInBits before this
// is reached. The other two are here: a terminator that ends the string, and
// a leading size field that states its length.
func (r *layoutResolver) stringWidth(param *Parameter, encoding *StringDataEncoding) (uint, error) {
	size := encoding.SizeInBits
	if size == nil {
		return 0, fmt.Errorf("%w: string parameter %q states no size at all",
			ErrDynamicSize, param.Name)
	}

	switch {
	case size.TerminationChar != "":
		return r.terminatedStringWidth(param, size.TerminationChar)

	case size.LeadingSize != nil:
		// The leading size field's own width is an attribute of an element
		// this package keeps raw, so the size of the size is not known.
		// Reading the string would mean guessing how many octets to skip.
		return 0, fmt.Errorf(
			"%w: string parameter %q is prefixed with a leading size field, which this package does not model",
			ErrUnsupportedEntry, param.Name)

	default:
		return 0, fmt.Errorf("%w: string parameter %q has no usable size form",
			ErrDynamicSize, param.Name)
	}
}

// terminatedStringWidth finds the terminator and returns the width up to and
// including it.
//
// The terminator counts toward the field: it occupies packet space, so the
// next field starts after it. The schema writes it as hex octets.
func (r *layoutResolver) terminatedStringWidth(param *Parameter, terminator string) (uint, error) {
	marker, err := parseHexOctets(terminator)
	if err != nil {
		return 0, fmt.Errorf("%w: the terminator of string parameter %q: %w",
			ErrDynamicSize, param.Name, err)
	}
	if len(marker) == 0 {
		return 0, fmt.Errorf("%w: string parameter %q has an empty terminator",
			ErrDynamicSize, param.Name)
	}

	// A delimited string has to start on an octet boundary for a search over
	// octets to mean anything.
	if r.cursor%8 != 0 {
		return 0, fmt.Errorf(
			"%w: string parameter %q is delimited but starts at bit %d, which is not an octet boundary",
			ErrDynamicSize, param.Name, r.cursor)
	}

	from := r.cursor / 8
	if from > uint(len(r.packet)) {
		return 0, fmt.Errorf("%w: string parameter %q starts past the end of the packet",
			ErrPacketTooShort, param.Name)
	}

	index := bytes.Index(r.packet[from:], marker)
	if index < 0 {
		return 0, fmt.Errorf(
			"%w: the terminator of string parameter %q does not appear in the rest of the packet",
			ErrPacketTooShort, param.Name)
	}
	return uint(index+len(marker)) * 8, nil
}

// repeatCount returns how many times an entry repeats, reading the packet
// when the count is dynamic.
func (r *layoutResolver) repeatCount(c *SequenceContainer, entry Entry) (uint, error) {
	if entry.RepeatEntry == nil || entry.RepeatEntry.Count == nil {
		return 1, nil
	}
	if entry.RepeatEntry.Offset != nil {
		// The gap between repetitions. Not modeled, and placing repetitions
		// without it would pack them where they do not belong.
		return 0, fmt.Errorf("%w: RepeatEntry with an Offset", ErrUnsupportedEntry)
	}

	count, err := r.integerValue(c, entry.RepeatEntry.Count, "a repeat count")
	if err != nil {
		return 0, err
	}
	if count < 0 {
		return 0, fmt.Errorf("%w: a repeat count of %d", ErrUnsupportedEntry, count)
	}
	if count > MaxRepeatCount {
		return 0, fmt.Errorf("%w: a repeat count of %d exceeds the limit of %d",
			ErrUnsupportedEntry, count, MaxRepeatCount)
	}

	// A repetition takes up at least one bit, so a count above what is left
	// of the packet cannot possibly be right. Checking this before the loop
	// that places each repetition is what keeps a huge count from appending
	// one Field per repetition regardless of whether the packet could ever
	// hold that many.
	packetBits := uint(len(r.packet)) * 8
	var remaining uint
	if r.cursor < packetBits {
		remaining = packetBits - r.cursor
	}
	if uint(count) > remaining {
		return 0, fmt.Errorf("%w: a repeat count of %d needs more bits than the %d left in the packet",
			ErrUnsupportedEntry, count, remaining)
	}
	return uint(count), nil
}

// place works out where an entry starts.
func (r *layoutResolver) place(c *SequenceContainer, entry Entry, containerStart uint) (uint, error) {
	location := entry.LocationInContainerInBits
	if location == nil {
		return r.cursor, nil
	}

	offset, err := r.locationOffset(c, location)
	if err != nil {
		return 0, err
	}

	switch reference := location.ReferenceLocationOrDefault(); reference {
	case "previousEntry":
		return addOffset(r.cursor, offset)

	case "nextEntry":
		// This positions the *following* entry, not this one. Honouring it
		// would mean placing an entry from its successor's attribute; the
		// schema itself calls it unusual, and treating it as previousEntry
		// would silently misplace the field.
		return 0, fmt.Errorf("%w: referenceLocation nextEntry", ErrUnsupportedEntry)

	case "containerStart":
		return addOffset(containerStart, offset)

	case "containerEnd":
		// Relative to the end of the container. With a packet in hand the
		// end is knowable: the container being resolved extends to the end of
		// the packet, which is the only reading that does not require having
		// already placed every entry.
		//
		// It is refused for a spliced inner container, whose end depends on
		// entries not yet walked. Guessing there would misplace the field.
		if c != r.layout.Container {
			return 0, fmt.Errorf(
				"%w: referenceLocation containerEnd in the spliced container %q, whose end is not yet known",
				ErrUnsupportedEntry, c.Name)
		}
		return addOffset(uint(len(r.packet))*8, offset)

	default:
		return 0, fmt.Errorf("%w: referenceLocation %q", ErrUnsupportedEntry, reference)
	}
}

// locationOffset reads an entry's bit offset.
//
// LocationInContainerInBits carries the same three forms as an
// IntegerValueType but as its own element rather than by reference, so it
// needs its own reader.
func (r *layoutResolver) locationOffset(c *SequenceContainer, location *LocationInContainer) (int64, error) {
	switch {
	case location.FixedValue != nil:
		return location.FixedValue.Int64(), nil

	case location.DynamicValue != nil:
		return r.dynamicValue(c, location.DynamicValue, "an entry position")

	case location.DiscreteLookupList != nil:
		return 0, fmt.Errorf("%w: an entry position comes from a DiscreteLookupList",
			ErrUnsupportedEntry)

	default:
		return 0, fmt.Errorf("%w: LocationInContainerInBits states no value", ErrDynamicSize)
	}
}

// integerValue reads one of the schema's IntegerValueType forms: a fixed
// number, or one taken from a parameter already decoded.
func (r *layoutResolver) integerValue(c *SequenceContainer, value *IntegerValue, what string) (int64, error) {
	switch {
	case value == nil:
		return 0, fmt.Errorf("%w: %s is not stated at all", ErrDynamicSize, what)

	case value.FixedValue != nil:
		return value.FixedValue.Int64(), nil

	case value.DynamicValue != nil:
		return r.dynamicValue(c, value.DynamicValue, what)

	case value.DiscreteLookupList != nil:
		// A table of comparisons rather than a single reference. Evaluating
		// it needs the comparison machinery the restriction criteria use, and
		// the model keeps it raw.
		return 0, fmt.Errorf("%w: %s comes from a DiscreteLookupList", ErrUnsupportedEntry, what)

	default:
		return 0, fmt.Errorf("%w: %s has none of the forms the schema allows", ErrDynamicSize, what)
	}
}

// dynamicValue reads a value from a parameter this walk has already decoded.
func (r *layoutResolver) dynamicValue(c *SequenceContainer, dynamic *DynamicValue, what string) (int64, error) {
	if dynamic.Parameter == nil {
		return 0, fmt.Errorf("%w: %s names no parameter", ErrDynamicSize, what)
	}
	ref := dynamic.Parameter

	if ref.Instance != 0 {
		// A value from a different packet. This walk has only this one.
		return 0, fmt.Errorf("%w: %s reads %q with instance %d, which is in another packet",
			ErrUnsupportedEntry, what, ref.ParameterRef, ref.Instance)
	}

	param, err := c.owner.ResolveParameter(ref.ParameterRef)
	if err != nil {
		return 0, fmt.Errorf("%s: %w", what, err)
	}

	value, ok := r.values[param]
	if !ok {
		// Either the parameter is not in this container, or it comes after
		// the field that needs it. A forward reference cannot be resolved in
		// one pass, and reading a field before it arrives would be worse.
		return 0, fmt.Errorf(
			"%w: %s reads %q, which this packet has not decoded before this point",
			ErrDynamicSize, what, ref.ParameterRef)
	}
	if value.Err != nil {
		return 0, fmt.Errorf("%w: %s reads %q, which did not decode: %w",
			ErrDynamicSize, what, ref.ParameterRef, value.Err)
	}

	// A length or a count is a number. Which of the two representations it
	// arrives in depends on the reference's own calibrated flag: the schema
	// defaults useCalibratedValue to true, and a calibrated length is what a
	// database means when it scales a raw count into octets.
	source := value.Raw
	if ref.Calibrated() {
		source = value.Engineering
	}

	number, ok := toFloat(source)
	if !ok {
		return 0, fmt.Errorf("%w: %s reads %q, which decoded to a %T rather than a number",
			ErrDynamicSize, what, ref.ParameterRef, source)
	}

	return dynamic.Adjustment.Apply(int64(number)), nil
}

// parseHexOctets reads the hex text the schema uses for a terminator.
func parseHexOctets(text string) ([]byte, error) {
	trimmed := trimHexPrefix(text)
	if len(trimmed)%2 != 0 {
		return nil, fmt.Errorf("%q is not a whole number of octets", text)
	}

	out := make([]byte, 0, len(trimmed)/2)
	for i := 0; i < len(trimmed); i += 2 {
		high, err := hexDigit(trimmed[i])
		if err != nil {
			return nil, err
		}
		low, err := hexDigit(trimmed[i+1])
		if err != nil {
			return nil, err
		}
		out = append(out, high<<4|low)
	}
	return out, nil
}

// trimHexPrefix drops a 0x or 0X prefix and any surrounding space.
func trimHexPrefix(text string) string {
	trimmed := text
	for len(trimmed) > 0 && (trimmed[0] == ' ' || trimmed[0] == '\t' || trimmed[0] == '\n' || trimmed[0] == '\r') {
		trimmed = trimmed[1:]
	}
	for len(trimmed) > 0 {
		last := trimmed[len(trimmed)-1]
		if last != ' ' && last != '\t' && last != '\n' && last != '\r' {
			break
		}
		trimmed = trimmed[:len(trimmed)-1]
	}
	if len(trimmed) >= 2 && trimmed[0] == '0' && (trimmed[1] == 'x' || trimmed[1] == 'X') {
		trimmed = trimmed[2:]
	}
	return trimmed
}

func hexDigit(c byte) (byte, error) {
	switch {
	case c >= '0' && c <= '9':
		return c - '0', nil
	case c >= 'a' && c <= 'f':
		return c - 'a' + 10, nil
	case c >= 'A' && c <= 'F':
		return c - 'A' + 10, nil
	default:
		return 0, fmt.Errorf("%q is not a hex digit", string(c))
	}
}
