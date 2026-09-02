package xtce

import (
	"encoding/hex"
	"errors"
	"fmt"
	"strconv"
	"strings"
)

// Deciding which container a packet is.
//
// A ground station receives octets, not labelled packets. Working out what it
// just received is a search: start at the abstract container every packet
// extends, look at the concrete containers derived from it, and take the one
// whose RestrictionCriteria the packet satisfies. A mission says "APID 42 and
// type 3 means a housekeeping packet" exactly this way.
//
// The criteria test parameters that the *base* container defines, so they can
// be checked before the derived container's own fields are read. That is what
// makes the search possible at all.

// layoutAgainst returns a container's layout, resolving it against the packet
// when the database alone cannot settle the shape.
//
// Matching used to use Layout only, which meant a container whose shape its
// own contents decide could neither be searched past nor selected: the walk
// stopped at ErrDynamicSize. Falling back to ResolveLayout lets a packet be
// identified among dynamic containers too, at the cost of a walk per
// candidate rather than a cached layout per container.
//
// Only the two errors that mean "the packet decides this" trigger the
// fallback. Anything else (an unresolved reference, a cycle) is a broken
// database and is reported as it was.
func layoutAgainst(container *SequenceContainer, packet []byte) (*Layout, error) {
	layout, err := container.Layout()
	if err == nil {
		return layout, nil
	}
	if !errors.Is(err, ErrDynamicSize) && !errors.Is(err, ErrUnsupportedEntry) {
		return nil, err
	}
	return container.ResolveLayout(packet)
}

// Match finds the container in this SpaceSystem tree that a packet satisfies,
// and extracts it.
//
// The search starts at root, follows each derived container whose criteria the
// packet meets, and goes as deep as it can. The deepest match wins: a packet
// that satisfies both "is a telemetry packet" and "is a housekeeping telemetry
// packet" is the latter.
//
// A container only matches if the packet is long enough to hold it, so a
// truncated packet does not match the type it was on its way to being.
//
// It returns ErrNoMatch when nothing matches, which is a normal thing to
// happen to a ground station and not necessarily a fault.
func (s *SpaceSystem) Match(root *SequenceContainer, packet []byte) (*Packet, error) {
	if root == nil {
		return nil, fmt.Errorf("%w: no root container to match against", ErrNotFound)
	}

	best, err := s.matchFrom(root, packet)
	if err != nil {
		return nil, err
	}
	if best == nil {
		return nil, fmt.Errorf("%w: no container derived from %q fits the packet", ErrNoMatch, root.Name)
	}

	layout, err := layoutAgainst(best, packet)
	if err != nil {
		return nil, err
	}
	return layout.Extract(packet)
}

// MatchFrom returns the container a packet matches, without extracting it.
//
// Use it when the answer you want is "what is this packet", or to hold on to
// the layout yourself.
func (s *SpaceSystem) MatchFrom(root *SequenceContainer, packet []byte) (*SequenceContainer, error) {
	match, err := s.matchFrom(root, packet)
	if err != nil {
		return nil, err
	}
	if match == nil {
		return nil, fmt.Errorf("%w: no container derived from %q fits the packet", ErrNoMatch, root.Name)
	}
	return match, nil
}

// matchFrom walks down from root and returns the deepest matching concrete
// container, or nil when there is none.
func (s *SpaceSystem) matchFrom(root *SequenceContainer, packet []byte) (*SequenceContainer, error) {
	for _, child := range s.derivedFrom(root) {
		ok, err := s.satisfies(child, packet)
		if err != nil {
			return nil, err
		}
		if !ok {
			continue
		}

		// The packet is one of these. Something derived from it may fit
		// better still, and if nothing does, this recursion returns child.
		match, err := s.matchFrom(child, packet)
		if err != nil {
			return nil, err
		}
		if match != nil {
			return match, nil
		}
	}

	// Nothing derived matched, so root itself is the answer, unless it is
	// abstract, which means it was never meant to describe a packet alone.
	if root.Abstract {
		return nil, nil
	}

	// And unless the packet is too short to be one. Passing the criteria that
	// happen to be testable is not enough: a truncated packet can satisfy a
	// comparison on a field near the front while lacking most of the
	// container, and calling that a match would hand the caller a layout that
	// cannot be extracted.
	layout, err := layoutAgainst(root, packet)
	if err != nil {
		return nil, err
	}
	if uint(len(packet))*8 < layout.BitSize {
		return nil, nil
	}
	return root, nil
}

// derivedFrom returns every container in the tree whose BaseContainer is base.
func (s *SpaceSystem) derivedFrom(base *SequenceContainer) []*SequenceContainer {
	var derived []*SequenceContainer

	s.Root().Walk(func(system *SpaceSystem) bool {
		if system.TelemetryMetaData == nil || system.TelemetryMetaData.ContainerSet == nil {
			return true
		}
		for _, candidate := range system.TelemetryMetaData.ContainerSet.SequenceContainers {
			if candidate.BaseContainer == nil || candidate.owner == nil {
				continue
			}
			// Resolve rather than compare names: two systems can each define a
			// container called Header.
			parent, err := candidate.owner.ResolveContainer(candidate.BaseContainer.ContainerRef)
			if err == nil && parent == base {
				derived = append(derived, candidate)
			}
		}
		return true
	})
	return derived
}

// satisfies reports whether a packet meets a container's restriction criteria.
//
// A container with no criteria is not a match. Inheriting without saying what
// distinguishes you means nothing selects you, and treating that as "always
// true" would make the first such container swallow every packet.
func (s *SpaceSystem) satisfies(container *SequenceContainer, packet []byte) (bool, error) {
	if container.BaseContainer == nil || container.BaseContainer.RestrictionCriteria == nil {
		return false, nil
	}
	criteria := container.BaseContainer.RestrictionCriteria

	switch {
	case criteria.Comparison != nil:
		return s.compare(container, criteria.Comparison, packet)

	case criteria.ComparisonList != nil:
		// The schema calls the "and" between them implicit: all must hold.
		for i := range criteria.ComparisonList.Comparisons {
			ok, err := s.compare(container, &criteria.ComparisonList.Comparisons[i], packet)
			if err != nil || !ok {
				return false, err
			}
		}
		return len(criteria.ComparisonList.Comparisons) > 0, nil

	case criteria.BooleanExpression != nil:
		return s.satisfiesExpression(container, criteria.BooleanExpression, packet)

	case criteria.CustomAlgorithm != nil:
		return false, fmt.Errorf("%w: container %q is selected by a CustomAlgorithm",
			ErrUnsupportedCriteria, container.Name)

	default:
		// Only NextContainer, which is about the stream rather than the
		// packet, so this packet alone cannot satisfy it.
		return false, nil
	}
}

// compare evaluates one Comparison against a packet.
//
// The parameter it names belongs to the base container, so it is read from the
// base's layout. The derived container's own layout cannot be built before we
// know the packet is one of those.
func (s *SpaceSystem) compare(container *SequenceContainer, comparison *Comparison, packet []byte) (bool, error) {
	if comparison.Instance != 0 {
		// A reference to the parameter's value in a different packet. Deciding
		// it needs the stream, not this packet.
		return false, fmt.Errorf("%w: a Comparison with instance %d",
			ErrUnsupportedCriteria, comparison.Instance)
	}

	actual, field, present, err := s.readCriterionParameter(
		container, comparison.ParameterRef, comparison.Calibrated(), packet)
	if err != nil || !present {
		return false, err
	}
	return evaluate(actual, comparison.Operator(), comparison.Value, field)
}

// readCriterionParameter reads the value of a parameter that restriction
// criteria name.
//
// The parameter belongs to the container being inherited from, so it is read
// from the base's layout: the derived container's own layout cannot be built
// before we know the packet is one of those.
//
// present is false when the packet ends before the field. That is a failed
// match rather than a broken database (a truncated packet simply is not the
// container it was on its way to being) so it comes back without an error.
func (s *SpaceSystem) readCriterionParameter(
	container *SequenceContainer, ref string, calibrated bool, packet []byte,
) (actual any, field Field, present bool, err error) {
	base, err := container.owner.ResolveContainer(container.BaseContainer.ContainerRef)
	if err != nil {
		return nil, Field{}, false, err
	}
	layout, err := layoutAgainst(base, packet)
	if err != nil {
		return nil, Field{}, false, err
	}

	param, err := container.owner.ResolveParameter(ref)
	if err != nil {
		return nil, Field{}, false, err
	}

	field, ok := findField(layout, param)
	if !ok {
		return nil, Field{}, false, fmt.Errorf(
			"%w: container %q tests %q, which container %q does not carry",
			ErrUnresolvedReference, container.Name, ref, base.Name)
	}

	if uint(len(packet))*8 < field.BitOffset+field.BitSize {
		return nil, field, false, nil
	}

	value := extractField(bitReader{data: packet}, field)
	if value.Err != nil {
		return nil, field, false, value.Err
	}

	if calibrated {
		return value.Engineering, field, true, nil
	}
	return value.Raw, field, true, nil
}

// findField returns the field a parameter occupies in a layout.
func findField(layout *Layout, param *Parameter) (Field, bool) {
	for _, field := range layout.Fields {
		if field.Parameter == param {
			return field, true
		}
	}
	return Field{}, false
}

// evaluate applies a comparison operator to a decoded value and the
// criterion's text.
//
// It serves both forms of criteria: a Comparison's value attribute and a
// Condition's Value element are the same thing spelled two ways.
func evaluate(actual any, operator string, text string, field Field) (bool, error) {
	// Text compares as text. An enumeration's calibrated value is its label,
	// and a label has no order, so only equality means anything there.
	if got, ok := actual.(string); ok {
		switch operator {
		case "==":
			return got == text, nil
		case "!=":
			return got != text, nil
		default:
			return false, fmt.Errorf("%w: operator %q on the text value %q",
				ErrUnsupportedCriteria, operator, got)
		}
	}

	// Binary compares as hex, again only for equality.
	if raw, ok := actual.([]byte); ok {
		want, err := hex.DecodeString(strings.TrimPrefix(text, "0x"))
		if err != nil {
			return false, fmt.Errorf("%w: %q is not hex", ErrInvalidComparison, text)
		}
		equal := len(raw) == len(want) && string(raw) == string(want)
		switch operator {
		case "==":
			return equal, nil
		case "!=":
			return !equal, nil
		default:
			return false, fmt.Errorf("%w: operator %q on a binary value", ErrUnsupportedCriteria, operator)
		}
	}

	got, ok := toFloat(actual)
	if !ok {
		return false, fmt.Errorf("%w: comparing a %T", ErrUnsupportedCriteria, actual)
	}

	// Whether the field reads as signed decides what a truncated value means,
	// so it is taken from the value that was actually decoded.
	_, signed := actual.(int64)

	want, err := parseComparisonValue(text, field.BitSize, signed)
	if err != nil {
		return false, err
	}

	switch operator {
	case "==":
		return got == want, nil
	case "!=":
		return got != want, nil
	case "<":
		return got < want, nil
	case "<=":
		return got <= want, nil
	case ">":
		return got > want, nil
	case ">=":
		return got >= want, nil
	default:
		return false, fmt.Errorf("%w: comparison operator %q", ErrUnsupportedCriteria, operator)
	}
}

// parseComparisonValue reads a comparison's value attribute as a number.
//
// The schema is explicit about the spelling: base ten unless the text starts
// with 0x, 0o or 0b. It also says the value is "truncated to use the least
// significant bits that match the bit size of the Parameter being compared
// to", which is how a database writes 0xFF for a field narrower than eight
// bits.
//
// Truncation only kicks in for a value that does not fit. A value that fits is
// left alone, because truncating it could only change it into something the
// database did not write, turning -1 into 255 against a signed field, say,
// which would then never match.
func parseComparisonValue(text string, width uint, signed bool) (float64, error) {
	text = strings.TrimSpace(text)

	// A boolean parameter's value attribute is written as xs:boolean.
	switch text {
	case "true":
		return 1, nil
	case "false":
		return 0, nil
	}

	if base, digits, prefixed := radix(text); prefixed {
		value, err := strconv.ParseUint(digits, base, 64)
		if err != nil {
			return 0, fmt.Errorf("%w: %q is not a base %d number", ErrInvalidComparison, text, base)
		}
		return fit(value, width, signed), nil
	}

	// Base ten, which may be a float once a calibrator is involved.
	if value, err := strconv.ParseInt(text, 10, 64); err == nil {
		if fitsField(value, width, signed) {
			return float64(value), nil
		}
		return fit(uint64(value), width, signed), nil
	}
	value, err := strconv.ParseFloat(text, 64)
	if err != nil {
		return 0, fmt.Errorf("%w: %q is not a number", ErrInvalidComparison, text)
	}
	return value, nil
}

// fitsField reports whether a value is already representable in a field of the
// given width, so that truncating it would change it.
func fitsField(value int64, width uint, signed bool) bool {
	if width == 0 || width >= 64 {
		return true
	}
	if signed {
		limit := int64(1) << (width - 1)
		return value >= -limit && value < limit
	}
	return value >= 0 && uint64(value) < uint64(1)<<width
}

// fit truncates a value to the field's width and reads the result the way the
// field does, signed or not.
func fit(value uint64, width uint, signed bool) float64 {
	truncated := truncate(value, width)
	if signed {
		return float64(signExtend(truncated, width))
	}
	return float64(truncated)
}

// radix splits off a 0x, 0o or 0b prefix.
func radix(text string) (base int, digits string, prefixed bool) {
	if len(text) < 3 {
		return 10, text, false
	}
	switch strings.ToLower(text[:2]) {
	case "0x":
		return 16, text[2:], true
	case "0o":
		return 8, text[2:], true
	case "0b":
		return 2, text[2:], true
	default:
		return 10, text, false
	}
}

// truncate keeps the least significant width bits, which is what the schema
// says to do with a comparison value.
func truncate(value uint64, width uint) uint64 {
	if width == 0 || width >= 64 {
		return value
	}
	return value & (1<<width - 1)
}
