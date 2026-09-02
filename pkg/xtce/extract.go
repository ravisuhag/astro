package xtce

import (
	"fmt"
	"strconv"
	"strings"
)

// Extraction: decoding a real packet against the database.
//
// This is what a mission database is for. Load and Validate say what the file
// means; Extract uses it. Given a container and the octets of a packet, it
// returns each parameter the container says is in there, with both the number
// the packet carried and the value an operator should see.
//
// The two are kept apart deliberately. An operator wants "23.4 °C" but an
// engineer chasing a fault wants the count that produced it, and a system that
// only keeps one of them cannot answer both questions.

// Value is one parameter read out of a packet.
type Value struct {
	// Field is where this came from: the name, the parameter, the type, and
	// the position in the packet.
	Field Field

	// Raw is the value exactly as the packet carried it, before calibration.
	// It is a uint64, an int64, a float64, a string or a []byte, depending on
	// the data encoding.
	Raw any

	// Engineering is the value after calibration and after the parameter type
	// has had its say: a float64 for a calibrated number, a string for an
	// enumeration label or a boolean's word, and whatever Raw held for the
	// types that have no further meaning to apply.
	Engineering any

	// Err is set when this one field could not be decoded, and the other two
	// are then meaningless. Extract keeps going past a bad field so that one
	// unsupported encoding in the middle of a packet does not hide everything
	// after it.
	Err error
}

// Name is the parameter's qualified name.
func (v Value) Name() string { return v.Field.Name }

// String renders the value the way an operator would read it.
func (v Value) String() string {
	if v.Err != nil {
		return fmt.Sprintf("%s: %v", v.Field.Name, v.Err)
	}
	return fmt.Sprintf("%s = %s", v.Field.Name, formatValue(v.Engineering))
}

// Float returns the engineering value as a number, and whether it is one.
func (v Value) Float() (float64, bool) {
	if v.Err != nil {
		return 0, false
	}
	return toFloat(v.Engineering)
}

// Text returns the engineering value as text, and whether it is text. An
// enumeration label and a boolean's word both come back this way.
func (v Value) Text() (string, bool) {
	if v.Err != nil {
		return "", false
	}
	text, ok := v.Engineering.(string)
	return text, ok
}

// Bytes returns the value's octets, and whether it is a binary field.
func (v Value) Bytes() ([]byte, bool) {
	if v.Err != nil {
		return nil, false
	}
	raw, ok := v.Raw.([]byte)
	return raw, ok
}

// Packet is everything read out of one packet.
type Packet struct {
	// Layout is the container the packet was read against.
	Layout *Layout

	// Values are in packet order, one per field, including the ones that
	// failed.
	Values []Value
}

// Get returns the value of a parameter by name.
//
// The name may be the qualified one or the bare parameter name. A bare name
// that two SpaceSystems both define returns the first in packet order, so
// qualify it when that is a possibility.
func (p *Packet) Get(name string) (Value, bool) {
	for _, value := range p.Values {
		if value.Field.Name == name {
			return value, true
		}
	}
	for _, value := range p.Values {
		if value.Field.Parameter != nil && value.Field.Parameter.Name == name {
			return value, true
		}
	}
	return Value{}, false
}

// Err returns the first field-level error, or nil when every field decoded.
//
// Extract itself only fails when nothing could be read at all, so this is how
// a caller who wants all-or-nothing gets it.
func (p *Packet) Err() error {
	for _, value := range p.Values {
		if value.Err != nil {
			return value.Err
		}
	}
	return nil
}

// String renders the packet as one line per parameter.
func (p *Packet) String() string {
	var b strings.Builder
	for _, value := range p.Values {
		b.WriteString(value.String())
		b.WriteByte('\n')
	}
	return b.String()
}

// Extract reads a packet against this layout.
//
// It returns an error only when the packet is too short for the layout to
// apply at all. A field that cannot be decoded on its own (an encoding this
// package does not support, a BCD nibble that is not a digit) is reported in
// that field's Err and the rest of the packet is still read.
func (l *Layout) Extract(packet []byte) (*Packet, error) {
	if bits := uint(len(packet)) * 8; bits < l.BitSize {
		return nil, fmt.Errorf("%w: the layout needs %d bits and the packet has %d",
			ErrPacketTooShort, l.BitSize, bits)
	}

	reader := bitReader{data: packet}
	result := &Packet{Layout: l, Values: make([]Value, 0, len(l.Fields))}

	for _, field := range l.Fields {
		result.Values = append(result.Values, extractField(reader, field))
	}
	return result, nil
}

// extractField reads and interprets one field.
func extractField(reader bitReader, field Field) Value {
	raw, err := decodeField(reader, field)
	if err != nil {
		return Value{Field: field, Err: err}
	}

	engineering, err := interpret(field.Type, raw)
	if err != nil {
		return Value{Field: field, Raw: raw, Err: err}
	}
	return Value{Field: field, Raw: raw, Engineering: engineering}
}

// interpret applies the parameter type's meaning to a raw value.
func interpret(t ParameterType, raw any) (any, error) {
	switch typed := t.(type) {
	case *EnumeratedParameterType:
		return typed.label(raw)

	case *BooleanParameterType:
		return typed.word(raw), nil

	case *AbsoluteTimeParameterType:
		return typed.seconds(raw)

	case *StringParameterType, *BinaryParameterType:
		// Nothing to calibrate and nothing to look up.
		return raw, nil

	default:
		// Integer and float parameters, which are the ones a calibrator
		// applies to.
		return calibrate(t, raw)
	}
}

// calibrate applies the type's default calibrator, if it has one.
//
// An uncalibrated parameter keeps its raw value's type rather than being
// widened to a float64. A 64-bit counter loses precision as a float64, and
// there is no reason to spend that when no arithmetic is being done.
func calibrate(t ParameterType, raw any) (any, error) {
	calibrator := defaultCalibrator(t)
	if calibrator == nil {
		return raw, nil
	}

	number, ok := toFloat(raw)
	if !ok {
		return nil, fmt.Errorf("%w: a calibrator on a %T value", ErrUnsupportedCalibrator, raw)
	}
	return calibrator.Calibrate(number)
}

// label looks up the raw value in the enumeration list.
//
// An unlisted value is not an error. Missions do send values their database
// does not list, and an operator is better served by seeing the number than by
// seeing the parameter disappear.
func (t *EnumeratedParameterType) label(raw any) (any, error) {
	value, ok := toInt(raw)
	if !ok {
		return nil, fmt.Errorf("%w: an enumeration over a %T value", ErrUnsupportedEncoding, raw)
	}

	for _, enum := range t.EnumerationList.Enumerations {
		if enum.MaxValue != nil {
			if value >= enum.Value && value <= *enum.MaxValue {
				return enum.Label, nil
			}
			continue
		}
		if value == enum.Value {
			return enum.Label, nil
		}
	}
	return strconv.FormatInt(value, 10), nil
}

// word returns the boolean's spelling for the value it carries.
func (t *BooleanParameterType) word(raw any) string {
	if isZero(raw) {
		return t.ZeroStringValueOrDefault()
	}
	return t.OneStringValueOrDefault()
}

// seconds turns a raw clock reading into a number of the encoding's units,
// applying the scale and offset the type carries.
//
// It does not turn it into a Go time. That needs the epoch, and ReferenceTime
// spells its epoch in several ways including a reference to another parameter.
// Resolving those belongs with the caller, who has the Epoch string from the
// type and the codecs in pkg/tcf.
func (t *AbsoluteTimeParameterType) seconds(raw any) (any, error) {
	if t.Encoding_ == nil {
		return raw, nil
	}

	count, ok := toFloat(raw)
	if !ok {
		return nil, fmt.Errorf("%w: a time scaled from a %T value", ErrUnsupportedEncoding, raw)
	}
	return count*t.Encoding_.ScaleOrDefault() + t.Encoding_.OffsetOrDefault(), nil
}

// toFloat widens a raw value to a float64 where that makes sense.
func toFloat(raw any) (float64, bool) {
	switch value := raw.(type) {
	case uint64:
		return float64(value), true
	case int64:
		return float64(value), true
	case float64:
		return value, true
	default:
		return 0, false
	}
}

// toInt narrows a raw value to an int64 where that makes sense.
func toInt(raw any) (int64, bool) {
	switch value := raw.(type) {
	case uint64:
		return int64(value), true
	case int64:
		return value, true
	default:
		return 0, false
	}
}

// isZero reports whether a raw value is the false state of a boolean.
func isZero(raw any) bool {
	switch value := raw.(type) {
	case uint64:
		return value == 0
	case int64:
		return value == 0
	case float64:
		return value == 0
	case string:
		return value == ""
	case []byte:
		for _, b := range value {
			if b != 0 {
				return false
			}
		}
		return true
	default:
		return false
	}
}

// formatValue renders a value for display.
func formatValue(value any) string {
	switch typed := value.(type) {
	case nil:
		return "?"
	case float64:
		return strconv.FormatFloat(typed, 'g', -1, 64)
	case []byte:
		return fmt.Sprintf("%X", typed)
	case string:
		return typed
	default:
		return fmt.Sprint(typed)
	}
}
