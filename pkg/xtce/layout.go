package xtce

import (
	"fmt"
	"strings"
)

// Flattening a container into a packet layout.
//
// A container does not describe a packet on its own. It names a base container
// whose entries come first, it can splice another container's entries into the
// middle of its own, and each entry may be placed somewhere other than
// straight after the one before. A Layout is what falls out once all of that
// has been worked through: a flat, ordered list of fields with a bit offset
// and a bit width each.
//
// Building it is separate from extraction on purpose. The layout depends only
// on the database, not on any packet, so a ground system builds one per packet
// type at startup and reuses it for every packet that arrives.

// maxSpliceDepth bounds how deep ContainerRefEntry splicing may go.
//
// Validate already refuses a BaseContainer cycle, but a container that splices
// itself through a ContainerRefEntry is a different loop and is not something
// Validate checks. This is what stops it recursing forever.
const maxSpliceDepth = 64

// Field is one parameter at a known place in a packet.
type Field struct {
	// Name is the parameter's qualified name, so two fields from different
	// SpaceSystems are distinguishable.
	Name string

	// Parameter and Type are what the entry resolved to.
	Parameter *Parameter
	Type      ParameterType

	// BitOffset is where the field starts, counted from the first bit of the
	// packet. BitSize is how wide it is.
	BitOffset uint
	BitSize   uint

	// Container is the container whose EntryList this entry came from, which
	// for an inherited field is the base rather than the container asked for.
	Container *SequenceContainer

	// Entry is the entry itself, for a caller that needs the parts the layout
	// does not act on — an IncludeCondition, say.
	Entry Entry
}

// String renders a field for a log line.
func (f Field) String() string {
	return fmt.Sprintf("%s at bit %d, %d bits", f.Name, f.BitOffset, f.BitSize)
}

// Layout is a container flattened into the fields a packet of that shape
// carries, in packet order.
type Layout struct {
	// Container is the container this layout was built from.
	Container *SequenceContainer

	// Fields are in packet order, which is not always offset order: an entry
	// placed with LocationInContainerInBits can point backwards.
	Fields []Field

	// BitSize is the offset just past the furthest field, which is the
	// smallest packet this layout can be read from. A packet may be longer;
	// XTCE containers do not have to cover every bit.
	BitSize uint
}

// Layout flattens a container into the fields a packet of that shape carries.
//
// Every field must have a width the database states outright. A delimited
// string, a binary field sized by another parameter, or a repeat count read
// from the packet all make the layout depend on the packet's contents, and
// this returns ErrDynamicSize rather than guessing. See docs/content/protocols/xtce/index.md.
func (c *SequenceContainer) Layout() (*Layout, error) {
	if c.owner == nil {
		return nil, fmt.Errorf("%w: the container did not come from Load, so its references cannot resolve",
			ErrUnresolvedReference)
	}

	layout := &Layout{Container: c}

	builder := &layoutBuilder{layout: layout}
	if err := builder.container(c, 0); err != nil {
		return nil, err
	}
	return layout, nil
}

// LayoutOf flattens the named container.
func (s *SpaceSystem) LayoutOf(qualifiedName string) (*Layout, error) {
	container, err := s.FindContainer(qualifiedName)
	if err != nil {
		return nil, err
	}
	return container.Layout()
}

// layoutBuilder walks the container tree, appending fields and tracking where
// the next one goes.
type layoutBuilder struct {
	layout *Layout

	// cursor is the bit just past the last entry placed, which is where an
	// entry with no LocationInContainerInBits starts.
	cursor uint

	// seen guards against a container splicing itself.
	seen []*SequenceContainer
}

// container appends the fields of one container, base entries first.
func (b *layoutBuilder) container(c *SequenceContainer, depth int) error {
	if depth > maxSpliceDepth {
		return fmt.Errorf("%w: containers nest more than %d deep from %s",
			ErrContainerCycle, maxSpliceDepth, b.layout.Container.Name)
	}
	for _, previous := range b.seen {
		if previous == c {
			return fmt.Errorf("%w: container %q includes itself", ErrContainerCycle, c.Name)
		}
	}
	b.seen = append(b.seen, c)
	defer func() { b.seen = b.seen[:len(b.seen)-1] }()

	// A container's inherited entries come first. That is what makes
	// inheritance useful: the base describes the packet header once and each
	// derived container adds its own body after it.
	if c.BaseContainer != nil {
		base, err := c.owner.ResolveContainer(c.BaseContainer.ContainerRef)
		if err != nil {
			return fmt.Errorf("base of container %q: %w", c.Name, err)
		}
		if err := b.container(base, depth+1); err != nil {
			return err
		}
	}

	// containerStart is relative to this container, not to the packet, so the
	// start has to be remembered before the entries move the cursor.
	start := b.cursor

	for i := range c.EntryList.Entries {
		if err := b.entry(c, c.EntryList.Entries[i], start, depth); err != nil {
			return err
		}
	}
	return nil
}

// entry places one entry and advances the cursor past it.
func (b *layoutBuilder) entry(c *SequenceContainer, entry Entry, containerStart uint, depth int) error {
	repeats, err := repeatCount(entry)
	if err != nil {
		return fmt.Errorf("entry %s of container %q: %w", entry, c.Name, err)
	}

	for range repeats {
		offset, err := b.place(entry, containerStart)
		if err != nil {
			return fmt.Errorf("entry %s of container %q: %w", entry, c.Name, err)
		}
		b.cursor = offset

		switch entry.Kind {
		case EntryParameterRef:
			if err := b.parameter(c, entry); err != nil {
				return err
			}
		case EntryContainerRef:
			spliced, err := c.owner.ResolveContainer(entry.Ref)
			if err != nil {
				return fmt.Errorf("entry %s of container %q: %w", entry, c.Name, err)
			}
			if err := b.container(spliced, depth+1); err != nil {
				return err
			}
		default:
			// An entry kind the model folds into EntryOther. Its width is not
			// modeled, so everything after it would be placed wrongly.
			return fmt.Errorf("%w: container %q has a %s, whose width this package does not model",
				ErrUnsupportedEntry, c.Name, entry.ElementName)
		}
	}
	return nil
}

// parameter resolves a ParameterRefEntry and appends it as a field.
func (b *layoutBuilder) parameter(c *SequenceContainer, entry Entry) error {
	param, err := c.owner.ResolveParameter(entry.Ref)
	if err != nil {
		return fmt.Errorf("entry %s of container %q: %w", entry, c.Name, err)
	}

	paramType, err := c.owner.ResolveParameterType(param.ParameterTypeRef)
	if err != nil {
		return fmt.Errorf("parameter %q of container %q: %w", param.Name, c.Name, err)
	}

	size, ok := paramType.Encoding().SizeInBits()
	if !ok {
		return fmt.Errorf("%w: parameter %q is encoded with a width the database does not state",
			ErrDynamicSize, param.Name)
	}

	b.layout.Fields = append(b.layout.Fields, Field{
		Name:      qualify(c.owner, param.Name),
		Parameter: param,
		Type:      paramType,
		BitOffset: b.cursor,
		BitSize:   size,
		Container: c,
		Entry:     entry,
	})

	b.cursor += size
	if b.cursor > b.layout.BitSize {
		b.layout.BitSize = b.cursor
	}
	return nil
}

// place works out where an entry starts, applying LocationInContainerInBits.
func (b *layoutBuilder) place(entry Entry, containerStart uint) (uint, error) {
	location := entry.LocationInContainerInBits
	if location == nil {
		// The schema's default: straight after the entry before.
		return b.cursor, nil
	}
	if location.DynamicValue != nil || location.DiscreteLookupList != nil {
		return 0, fmt.Errorf("%w: the entry is placed by a value read from the packet", ErrDynamicSize)
	}
	if location.FixedValue == nil {
		return 0, fmt.Errorf("%w: LocationInContainerInBits has no FixedValue", ErrDynamicSize)
	}
	offset := location.FixedValue.Int64()

	switch reference := location.ReferenceLocationOrDefault(); reference {
	case "previousEntry", "nextEntry":
		// nextEntry positions the *following* entry rather than this one. The
		// model keeps the distinction but honouring it would mean placing an
		// entry from its successor's attribute, which no database in practice
		// uses and which the schema itself calls unusual. Treating it as
		// previousEntry would silently misplace the field, so it is refused.
		if reference == "nextEntry" {
			return 0, fmt.Errorf("%w: referenceLocation nextEntry", ErrUnsupportedEntry)
		}
		return addOffset(b.cursor, offset)

	case "containerStart":
		return addOffset(containerStart, offset)

	case "containerEnd":
		// Relative to the end of the container, which is not known until every
		// entry has been placed. A forward reference like that cannot be
		// resolved in one pass.
		return 0, fmt.Errorf("%w: referenceLocation containerEnd", ErrUnsupportedEntry)

	default:
		return 0, fmt.Errorf("%w: referenceLocation %q", ErrUnsupportedEntry, reference)
	}
}

// addOffset applies a signed bit offset to a position, refusing one that would
// take it before the start of the packet.
func addOffset(base uint, offset int64) (uint, error) {
	result := int64(base) + offset
	if result < 0 {
		return 0, fmt.Errorf("%w: an offset of %d from bit %d lands before the packet starts",
			ErrUnsupportedEntry, offset, base)
	}
	return uint(result), nil
}

// repeatCount returns how many times an entry repeats, which is one unless a
// RepeatEntry says otherwise.
func repeatCount(entry Entry) (uint, error) {
	if entry.RepeatEntry == nil || entry.RepeatEntry.Count == nil {
		return 1, nil
	}
	count := entry.RepeatEntry.Count
	if count.FixedValue == nil {
		return 0, fmt.Errorf("%w: the entry repeats a number of times read from the packet", ErrDynamicSize)
	}
	if *count.FixedValue < 0 {
		return 0, fmt.Errorf("%w: a repeat count of %d", ErrUnsupportedEntry, count.FixedValue.Int64())
	}
	if entry.RepeatEntry.Offset != nil {
		// The gap between repetitions. Supporting it is easy enough, but only
		// once there is a database to check it against.
		return 0, fmt.Errorf("%w: RepeatEntry with an Offset", ErrUnsupportedEntry)
	}
	return uint(*count.FixedValue), nil
}

// qualify returns a name prefixed with the SpaceSystem that holds it.
func qualify(system *SpaceSystem, name string) string {
	path := system.QualifiedName()
	if path == "" || path == "/" {
		return "/" + name
	}
	return strings.TrimSuffix(path, "/") + "/" + name
}
