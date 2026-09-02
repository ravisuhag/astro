package xtce

import (
	"encoding/xml"
	"fmt"
	"io"
)

// Parameter types and data encodings.
//
// A parameter type answers two separate questions, and keeping them apart is
// the key to reading this file:
//
//   - What is the value? An integer, a float, an enumeration label, a string.
//     That is the parameter type.
//   - How is it written in the packet? So many bits, this signedness, this
//     byte order. That is the data encoding hanging off it.
//
// The two need not match. A 12-bit unsigned field on the wire can be a float
// parameter once a calibrator has been applied to it, which is exactly what
// most analogue telemetry looks like.

// ParameterTypeSet holds the parameter types a SpaceSystem defines.
//
// The schema makes this an unordered choice of ten element kinds. Seven are
// modeled in full. ArrayParameterType, AggregateParameterType and
// RelativeTimeParameterType are kept opaque: their names are decoded so a
// parameter pointing at one still resolves and TypeKind says what it found,
// but their contents stay raw and Layout refuses a parameter of such a type.
// The coverage matrix records this.
type ParameterTypeSet struct {
	IntegerTypes      []*IntegerParameterType      `xml:"http://www.omg.org/spec/XTCE/20180204 IntegerParameterType"`
	FloatTypes        []*FloatParameterType        `xml:"http://www.omg.org/spec/XTCE/20180204 FloatParameterType"`
	EnumeratedTypes   []*EnumeratedParameterType   `xml:"http://www.omg.org/spec/XTCE/20180204 EnumeratedParameterType"`
	StringTypes       []*StringParameterType       `xml:"http://www.omg.org/spec/XTCE/20180204 StringParameterType"`
	BinaryTypes       []*BinaryParameterType       `xml:"http://www.omg.org/spec/XTCE/20180204 BinaryParameterType"`
	BooleanTypes      []*BooleanParameterType      `xml:"http://www.omg.org/spec/XTCE/20180204 BooleanParameterType"`
	AbsoluteTimeTypes []*AbsoluteTimeParameterType `xml:"http://www.omg.org/spec/XTCE/20180204 AbsoluteTimeParameterType"`

	ArrayTypes        []*ArrayParameterType        `xml:"http://www.omg.org/spec/XTCE/20180204 ArrayParameterType"`
	AggregateTypes    []*AggregateParameterType    `xml:"http://www.omg.org/spec/XTCE/20180204 AggregateParameterType"`
	RelativeTimeTypes []*RelativeTimeParameterType `xml:"http://www.omg.org/spec/XTCE/20180204 RelativeTimeParameterType"`
}

// ParameterType is what every modeled parameter type has in common.
type ParameterType interface {
	// TypeName is the type's name, which parameters reference.
	TypeName() string
	// TypeKind names the kind for display: "integer", "float" and so on.
	TypeKind() string
	// Encoding returns how the value is written in the packet, or nil when the
	// type does not say.
	Encoding() *DataEncoding
}

// baseDataType is the schema's BaseDataType: the description, the units, and
// the choice of one data encoding.
type baseDataType struct {
	Name             string `xml:"name,attr"`
	ShortDescription string `xml:"shortDescription,attr"`
	// BaseType lets one type extend another.
	BaseType string `xml:"baseType,attr"`

	LongDescription string   `xml:"http://www.omg.org/spec/XTCE/20180204 LongDescription"`
	UnitSet         *UnitSet `xml:"http://www.omg.org/spec/XTCE/20180204 UnitSet"`

	IntegerDataEncoding *IntegerDataEncoding `xml:"http://www.omg.org/spec/XTCE/20180204 IntegerDataEncoding"`
	FloatDataEncoding   *FloatDataEncoding   `xml:"http://www.omg.org/spec/XTCE/20180204 FloatDataEncoding"`
	StringDataEncoding  *StringDataEncoding  `xml:"http://www.omg.org/spec/XTCE/20180204 StringDataEncoding"`
	BinaryDataEncoding  *BinaryDataEncoding  `xml:"http://www.omg.org/spec/XTCE/20180204 BinaryDataEncoding"`
}

// TypeName returns the type's name.
func (b *baseDataType) TypeName() string { return b.Name }

// Encoding returns whichever data encoding the type carries.
func (b *baseDataType) Encoding() *DataEncoding {
	switch {
	case b.IntegerDataEncoding != nil:
		return &DataEncoding{Integer: b.IntegerDataEncoding}
	case b.FloatDataEncoding != nil:
		return &DataEncoding{Float: b.FloatDataEncoding}
	case b.StringDataEncoding != nil:
		return &DataEncoding{String: b.StringDataEncoding}
	case b.BinaryDataEncoding != nil:
		return &DataEncoding{Binary: b.BinaryDataEncoding}
	default:
		return nil
	}
}

// UnitSet lists the units a value is in.
type UnitSet struct {
	Units []Unit `xml:"http://www.omg.org/spec/XTCE/20180204 Unit"`
}

// Unit is one unit, with an optional power and factor.
type Unit struct {
	// Power defaults to 1. It is a pointer so an absent power is not read as
	// zero, which would say the unit does not appear at all; read it through
	// PowerOrDefault.
	Power       *float64 `xml:"power,attr"`
	Factor      string   `xml:"factor,attr"`
	Description string   `xml:"description,attr"`
	Value       string   `xml:",chardata"`
}

// PowerOrDefault returns the unit's exponent, applying the schema's default
// of 1.
func (u *Unit) PowerOrDefault() float64 {
	if u.Power == nil {
		return 1
	}
	return *u.Power
}

// IntegerParameterType is a whole-number parameter.
type IntegerParameterType struct {
	baseDataType
	// SizeInBits is the value's width, defaulting to 32. This is the width of
	// the value, which is not necessarily the width on the wire: the encoding
	// has its own sizeInBits.
	SizeInBits uint `xml:"sizeInBits,attr"`
	// Signed defaults to true. It is a pointer because false has to be
	// distinguishable from absent.
	Signed       *bool  `xml:"signed,attr"`
	InitialValue string `xml:"initialValue,attr"`
}

// TypeKind names the kind.
func (t *IntegerParameterType) TypeKind() string { return "integer" }

// IsSigned reports the signedness, applying the schema's default of true.
func (t *IntegerParameterType) IsSigned() bool { return t.Signed == nil || *t.Signed }

// Size returns the value's width in bits, applying the schema's default of
// 32. This is the width of the value, not the width on the wire — the
// encoding has its own Size.
func (t *IntegerParameterType) Size() uint {
	if t.SizeInBits == 0 {
		return 32
	}
	return t.SizeInBits
}

// FloatParameterType is a real-number parameter.
type FloatParameterType struct {
	baseDataType
	// SizeInBits is 32 or 64, defaulting to 32.
	SizeInBits   uint   `xml:"sizeInBits,attr"`
	InitialValue string `xml:"initialValue,attr"`
}

// TypeKind names the kind.
func (t *FloatParameterType) TypeKind() string { return "float" }

// Size returns the value's width in bits, applying the schema's default of
// 32. The encoding's width on the wire is the encoding's own Size.
func (t *FloatParameterType) Size() uint {
	if t.SizeInBits == 0 {
		return 32
	}
	return t.SizeInBits
}

// EnumeratedParameterType maps raw values to labels.
type EnumeratedParameterType struct {
	baseDataType
	InitialValue string `xml:"initialValue,attr"`
	// EnumerationList is required by the schema.
	EnumerationList EnumerationList `xml:"http://www.omg.org/spec/XTCE/20180204 EnumerationList"`
}

// TypeKind names the kind.
func (t *EnumeratedParameterType) TypeKind() string { return "enumerated" }

// EnumerationList holds the value-to-label mapping.
type EnumerationList struct {
	Enumerations []Enumeration `xml:"http://www.omg.org/spec/XTCE/20180204 Enumeration"`
}

// Enumeration is one label. MaxValue, when present, makes it a range rather
// than a single value.
type Enumeration struct {
	Value            int64  `xml:"value,attr"`
	MaxValue         *int64 `xml:"maxValue,attr"`
	Label            string `xml:"label,attr"`
	ShortDescription string `xml:"shortDescription,attr"`
}

// StringParameterType is a text parameter.
type StringParameterType struct {
	baseDataType
	InitialValue       string `xml:"initialValue,attr"`
	RestrictionPattern string `xml:"restrictionPattern,attr"`
	CharacterWidth     string `xml:"characterWidth,attr"`
}

// TypeKind names the kind.
func (t *StringParameterType) TypeKind() string { return "string" }

// BinaryParameterType is a run of raw octets.
type BinaryParameterType struct {
	baseDataType
	// InitialValue is hex.
	InitialValue string `xml:"initialValue,attr"`
}

// TypeKind names the kind.
func (t *BinaryParameterType) TypeKind() string { return "binary" }

// BooleanParameterType is a flag, with the words to print for each state.
type BooleanParameterType struct {
	baseDataType
	InitialValue string `xml:"initialValue,attr"`
	// OneStringValue defaults to "True" and ZeroStringValue to "False"; read
	// them through the OrDefault accessors.
	OneStringValue  string `xml:"oneStringValue,attr"`
	ZeroStringValue string `xml:"zeroStringValue,attr"`
}

// TypeKind names the kind.
func (t *BooleanParameterType) TypeKind() string { return "boolean" }

// OneStringValueOrDefault returns the word for the true state, applying the
// schema's default of "True".
func (t *BooleanParameterType) OneStringValueOrDefault() string {
	if t.OneStringValue == "" {
		return "True"
	}
	return t.OneStringValue
}

// ZeroStringValueOrDefault returns the word for the false state, applying the
// schema's default of "False".
func (t *BooleanParameterType) ZeroStringValueOrDefault() string {
	if t.ZeroStringValue == "" {
		return "False"
	}
	return t.ZeroStringValue
}

// AbsoluteTimeParameterType is a point in time.
//
// It does not extend BaseDataType like the others: the schema gives it its own
// base, where the encoding sits inside an Encoding element that adds units, a
// scale and an offset. So a spacecraft clock reading is described as "this
// many bits, in these units, counted from this epoch" — which is what
// pkg/tcf's CUC and CDS codes need to turn it into a time.
type AbsoluteTimeParameterType struct {
	Name             string `xml:"name,attr"`
	ShortDescription string `xml:"shortDescription,attr"`
	BaseType         string `xml:"baseType,attr"`
	InitialValue     string `xml:"initialValue,attr"`

	LongDescription string `xml:"http://www.omg.org/spec/XTCE/20180204 LongDescription"`

	// Encoding wraps the data encoding with the scaling that turns a raw count
	// into a time.
	Encoding_ *TimeEncoding `xml:"http://www.omg.org/spec/XTCE/20180204 Encoding"`
	// ReferenceTime says what the count is measured from.
	ReferenceTime *ReferenceTime `xml:"http://www.omg.org/spec/XTCE/20180204 ReferenceTime"`
}

// TypeName returns the type's name.
func (t *AbsoluteTimeParameterType) TypeName() string { return t.Name }

// TypeKind names the kind.
func (t *AbsoluteTimeParameterType) TypeKind() string { return "absolute time" }

// Encoding returns the wrapped data encoding, or nil when the type does not
// say how the time is written.
func (t *AbsoluteTimeParameterType) Encoding() *DataEncoding {
	if t.Encoding_ == nil {
		return nil
	}
	return t.Encoding_.DataEncoding()
}

// ArrayParameterType is an array of another type. It is kept opaque: the
// name is decoded so references to it resolve, the contents stay raw, and it
// has no encoding — so Layout refuses a parameter of this type rather than
// guessing at its width.
type ArrayParameterType struct {
	Name             string `xml:"name,attr"`
	ShortDescription string `xml:"shortDescription,attr"`
	// ArrayTypeRef names the element type.
	ArrayTypeRef string `xml:"arrayTypeRef,attr"`

	// Raw holds the type's contents — the dimension list — undecoded.
	Raw []byte `xml:",innerxml"`
}

// TypeName returns the type's name.
func (t *ArrayParameterType) TypeName() string { return t.Name }

// TypeKind names the kind, marking that only the identity is modeled.
func (t *ArrayParameterType) TypeKind() string { return "array (not modeled)" }

// Encoding returns nil: an opaque type does not say how it is written.
func (t *ArrayParameterType) Encoding() *DataEncoding { return nil }

// AggregateParameterType is a struct of members. Kept opaque the same way as
// ArrayParameterType.
type AggregateParameterType struct {
	Name             string `xml:"name,attr"`
	ShortDescription string `xml:"shortDescription,attr"`

	// Raw holds the member list, undecoded.
	Raw []byte `xml:",innerxml"`
}

// TypeName returns the type's name.
func (t *AggregateParameterType) TypeName() string { return t.Name }

// TypeKind names the kind, marking that only the identity is modeled.
func (t *AggregateParameterType) TypeKind() string { return "aggregate (not modeled)" }

// Encoding returns nil: an opaque type does not say how it is written.
func (t *AggregateParameterType) Encoding() *DataEncoding { return nil }

// RelativeTimeParameterType is a duration rather than an instant. Kept opaque
// the same way as ArrayParameterType.
type RelativeTimeParameterType struct {
	Name             string `xml:"name,attr"`
	ShortDescription string `xml:"shortDescription,attr"`

	// Raw holds the type's contents, undecoded.
	Raw []byte `xml:",innerxml"`
}

// TypeName returns the type's name.
func (t *RelativeTimeParameterType) TypeName() string { return t.Name }

// TypeKind names the kind, marking that only the identity is modeled.
func (t *RelativeTimeParameterType) TypeKind() string { return "relative time (not modeled)" }

// Encoding returns nil: an opaque type does not say how it is written.
func (t *RelativeTimeParameterType) Encoding() *DataEncoding { return nil }

// TimeEncoding is the schema's EncodingType: a data encoding plus the units,
// scale and offset that turn the raw count into a time.
type TimeEncoding struct {
	// Units defaults to "seconds"; read it through UnitsOrDefault.
	Units string `xml:"units,attr"`
	// Scale defaults to 1 and Offset to 0. Both are pointers so that an
	// explicit zero is distinguishable from absent.
	Scale  *float64 `xml:"scale,attr"`
	Offset *float64 `xml:"offset,attr"`

	IntegerDataEncoding *IntegerDataEncoding `xml:"http://www.omg.org/spec/XTCE/20180204 IntegerDataEncoding"`
	FloatDataEncoding   *FloatDataEncoding   `xml:"http://www.omg.org/spec/XTCE/20180204 FloatDataEncoding"`
	StringDataEncoding  *StringDataEncoding  `xml:"http://www.omg.org/spec/XTCE/20180204 StringDataEncoding"`
	BinaryDataEncoding  *BinaryDataEncoding  `xml:"http://www.omg.org/spec/XTCE/20180204 BinaryDataEncoding"`
}

// DataEncoding returns whichever encoding the element carries.
func (e *TimeEncoding) DataEncoding() *DataEncoding {
	switch {
	case e.IntegerDataEncoding != nil:
		return &DataEncoding{Integer: e.IntegerDataEncoding}
	case e.FloatDataEncoding != nil:
		return &DataEncoding{Float: e.FloatDataEncoding}
	case e.StringDataEncoding != nil:
		return &DataEncoding{String: e.StringDataEncoding}
	case e.BinaryDataEncoding != nil:
		return &DataEncoding{Binary: e.BinaryDataEncoding}
	default:
		return nil
	}
}

// UnitsOrDefault returns what one count of the encoding means, applying the
// schema's default of "seconds".
func (e *TimeEncoding) UnitsOrDefault() string {
	if e.Units == "" {
		return "seconds"
	}
	return e.Units
}

// ScaleOrDefault returns the scale, applying the schema's default of 1.
func (e *TimeEncoding) ScaleOrDefault() float64 {
	if e.Scale == nil {
		return 1
	}
	return *e.Scale
}

// OffsetOrDefault returns the offset, applying the schema's default of 0.
func (e *TimeEncoding) OffsetOrDefault() float64 {
	if e.Offset == nil {
		return 0
	}
	return *e.Offset
}

// ReferenceTime says what an absolute time is measured from: a named epoch, or
// another parameter's value.
type ReferenceTime struct {
	// Epoch is a date, a dateTime, or one of the schema's named epochs —
	// TAI, J2000, UNIX, GPS.
	Epoch string `xml:"http://www.omg.org/spec/XTCE/20180204 Epoch"`
	// OffsetFrom names another time parameter to count from. Kept raw.
	OffsetFrom *RawXML `xml:"http://www.omg.org/spec/XTCE/20180204 OffsetFrom"`
}

// DataEncoding is whichever of the four encodings a type carries. Exactly one
// field is set.
type DataEncoding struct {
	Integer *IntegerDataEncoding
	Float   *FloatDataEncoding
	String  *StringDataEncoding
	Binary  *BinaryDataEncoding
}

// SizeInBits returns the encoded width in bits and whether it is known.
//
// It is not always known. A string may be delimited rather than fixed, and a
// binary encoding may take its size from another parameter. Those cases return
// false rather than a guess.
func (d *DataEncoding) SizeInBits() (uint, bool) {
	switch {
	case d == nil:
		return 0, false
	case d.Integer != nil:
		return d.Integer.Size(), true
	case d.Float != nil:
		return d.Float.Size(), true
	case d.String != nil && d.String.SizeInBits != nil && d.String.SizeInBits.Fixed != nil:
		return uint(*d.String.SizeInBits.Fixed), true
	case d.Binary != nil && d.Binary.SizeInBits != nil && d.Binary.SizeInBits.FixedValue != nil:
		return uint(*d.Binary.SizeInBits.FixedValue), true
	default:
		return 0, false
	}
}

// Kind names which encoding this is.
func (d *DataEncoding) Kind() string {
	switch {
	case d == nil:
		return "none"
	case d.Integer != nil:
		return "integer"
	case d.Float != nil:
		return "float"
	case d.String != nil:
		return "string"
	default:
		return "binary"
	}
}

// HasContextCalibrators reports whether the encoding carries a
// ContextCalibratorList — calibration that depends on another parameter's
// value, which this package keeps raw. When it is true, the default
// calibrator alone may be the wrong curve for a given packet, so a consumer
// computing engineering values should not trust the default blindly.
func (d *DataEncoding) HasContextCalibrators() bool {
	switch {
	case d == nil:
		return false
	case d.Integer != nil:
		return d.Integer.ContextCalibratorList != nil
	case d.Float != nil:
		return d.Float.ContextCalibratorList != nil
	default:
		return false
	}
}

// commonEncoding is the schema's DataEncodingType: the bit and byte order
// every encoding shares.
type commonEncoding struct {
	// BitOrder defaults to mostSignificantBitFirst and ByteOrder to
	// mostSignificantByteFirst.
	BitOrder  string `xml:"bitOrder,attr"`
	ByteOrder string `xml:"byteOrder,attr"`
}

// BitOrderOrDefault applies the schema's default.
func (c commonEncoding) BitOrderOrDefault() string {
	if c.BitOrder == "" {
		return "mostSignificantBitFirst"
	}
	return c.BitOrder
}

// ByteOrderOrDefault applies the schema's default.
func (c commonEncoding) ByteOrderOrDefault() string {
	if c.ByteOrder == "" {
		return "mostSignificantByteFirst"
	}
	return c.ByteOrder
}

// IntegerDataEncoding writes a number as a field of bits.
type IntegerDataEncoding struct {
	commonEncoding
	// Encoding is unsigned, signMagnitude, twosComplement, onesComplement, BCD
	// or packedBCD. It defaults to unsigned.
	Encoding string `xml:"encoding,attr"`
	// SizeInBits defaults to 8.
	SizeInBits uint `xml:"sizeInBits,attr"`
	// ChangeThreshold is the smallest change in value that is significant.
	// Absent or zero means any change is. It is a pointer so an explicit zero
	// is distinguishable from absent.
	ChangeThreshold *uint64 `xml:"changeThreshold,attr"`

	DefaultCalibrator *Calibrator `xml:"http://www.omg.org/spec/XTCE/20180204 DefaultCalibrator"`
	// ContextCalibratorList is calibration that depends on another
	// parameter's value. It is kept raw, and its presence matters: when it is
	// set, the default calibrator alone may be the wrong curve for a given
	// packet. HasContextCalibrators on DataEncoding reports it.
	ContextCalibratorList *RawXML `xml:"http://www.omg.org/spec/XTCE/20180204 ContextCalibratorList"`
}

// Size applies the schema's default of 8 bits.
func (e *IntegerDataEncoding) Size() uint {
	if e.SizeInBits == 0 {
		return 8
	}
	return e.SizeInBits
}

// EncodingOrDefault applies the schema's default of unsigned.
func (e *IntegerDataEncoding) EncodingOrDefault() string {
	if e.Encoding == "" {
		return "unsigned"
	}
	return e.Encoding
}

// FloatDataEncoding writes a real number.
type FloatDataEncoding struct {
	commonEncoding
	// Encoding defaults to IEEE754_1985. The alternatives are MILSTD_1750A and
	// the two decimal forms.
	Encoding string `xml:"encoding,attr"`
	// SizeInBits defaults to 32. The schema allows 16, 32, 40, 48, 64, 80 and
	// 128.
	SizeInBits uint `xml:"sizeInBits,attr"`
	// ChangeThreshold is the smallest change in value that is significant.
	// Absent or zero means any change is.
	ChangeThreshold *float64 `xml:"changeThreshold,attr"`

	DefaultCalibrator *Calibrator `xml:"http://www.omg.org/spec/XTCE/20180204 DefaultCalibrator"`
	// ContextCalibratorList is kept raw, the same way as on
	// IntegerDataEncoding.
	ContextCalibratorList *RawXML `xml:"http://www.omg.org/spec/XTCE/20180204 ContextCalibratorList"`
}

// Size applies the schema's default of 32 bits.
func (e *FloatDataEncoding) Size() uint {
	if e.SizeInBits == 0 {
		return 32
	}
	return e.SizeInBits
}

// EncodingOrDefault applies the schema's default.
func (e *FloatDataEncoding) EncodingOrDefault() string {
	if e.Encoding == "" {
		return "IEEE754_1985"
	}
	return e.Encoding
}

// StringDataEncoding writes text. Its size is either fixed or delimited.
type StringDataEncoding struct {
	commonEncoding
	// Encoding defaults to UTF-8.
	Encoding string `xml:"encoding,attr"`

	SizeInBits *StringSize `xml:"http://www.omg.org/spec/XTCE/20180204 SizeInBits"`
	// Variable is the delimited form, kept raw.
	Variable *RawXML `xml:"http://www.omg.org/spec/XTCE/20180204 Variable"`
}

// EncodingOrDefault applies the schema's default of UTF-8.
func (e *StringDataEncoding) EncodingOrDefault() string {
	if e.Encoding == "" {
		return "UTF-8"
	}
	return e.Encoding
}

// StringSize is a string's fixed width.
type StringSize struct {
	Fixed *FixedInteger `xml:"http://www.omg.org/spec/XTCE/20180204 Fixed>FixedValue"`
	// TerminationChar and LeadingSize are the other forms the schema allows.
	TerminationChar string  `xml:"http://www.omg.org/spec/XTCE/20180204 TerminationChar"`
	LeadingSize     *RawXML `xml:"http://www.omg.org/spec/XTCE/20180204 LeadingSize"`
}

// BinaryDataEncoding writes raw octets.
type BinaryDataEncoding struct {
	commonEncoding
	// SizeInBits is required by the schema and may be dynamic.
	SizeInBits *IntegerValue `xml:"http://www.omg.org/spec/XTCE/20180204 SizeInBits"`
}

// Calibrator turns a raw value into an engineering one.
type Calibrator struct {
	Name string `xml:"name,attr"`

	Polynomial *PolynomialCalibrator `xml:"http://www.omg.org/spec/XTCE/20180204 PolynomialCalibrator"`
	Spline     *SplineCalibrator     `xml:"http://www.omg.org/spec/XTCE/20180204 SplineCalibrator"`
	// MathOperation is the third form: a postfix expression rather than
	// arithmetic the schema states outright.
	MathOperation *MathOperationCalibrator `xml:"http://www.omg.org/spec/XTCE/20180204 MathOperationCalibrator"`
}

// Kind names which calibrator this is.
func (c *Calibrator) Kind() string {
	switch {
	case c == nil:
		return "none"
	case c.Polynomial != nil:
		return "polynomial"
	case c.Spline != nil:
		return "spline"
	case c.MathOperation != nil:
		return "math operation"
	default:
		return "empty"
	}
}

// PolynomialCalibrator is a sum of terms: coefficient times raw to the power
// of exponent.
type PolynomialCalibrator struct {
	Terms []Term `xml:"http://www.omg.org/spec/XTCE/20180204 Term"`
}

// Term is one term of a polynomial.
type Term struct {
	Coefficient float64 `xml:"coefficient,attr"`
	Exponent    uint    `xml:"exponent,attr"`
}

// SplineCalibrator interpolates between measured points.
type SplineCalibrator struct {
	// Order defaults to 1, meaning straight lines between points.
	Order uint `xml:"order,attr"`
	// Extrapolate says whether values outside the points are calibrated by
	// extending the end segments.
	Extrapolate bool `xml:"extrapolate,attr"`
	// Points must number at least two, per the schema.
	Points []SplinePoint `xml:"http://www.omg.org/spec/XTCE/20180204 SplinePoint"`
}

// SplinePoint is one measured pair.
type SplinePoint struct {
	Raw        float64 `xml:"raw,attr"`
	Calibrated float64 `xml:"calibrated,attr"`
	Order      uint    `xml:"order,attr"`
}

// EntryList is a container's entries, in packet order.
type EntryList struct {
	Entries []Entry
}

// entryPayload decodes the parts of an entry that are elements rather than
// attributes. It is shared by every entry kind because the schema's
// SequenceEntryType gives them all the same ones.
type entryPayload struct {
	ParameterRef     string `xml:"parameterRef,attr"`
	ContainerRef     string `xml:"containerRef,attr"`
	ShortDescription string `xml:"shortDescription,attr"`

	LocationInContainerInBits *LocationInContainer `xml:"http://www.omg.org/spec/XTCE/20180204 LocationInContainerInBits"`
	RepeatEntry               *Repeat              `xml:"http://www.omg.org/spec/XTCE/20180204 RepeatEntry"`
	IncludeCondition          *RawXML              `xml:"http://www.omg.org/spec/XTCE/20180204 IncludeCondition"`
}

// UnmarshalXML decodes an EntryList, keeping the entries in document order.
//
// This is hand-written because encoding/xml cannot do it. Given a struct with
// one slice field per element name, the decoder fills each slice with the
// elements of that name and the interleaving is lost — and for a packet
// layout the interleaving is the whole point. So the tokens are walked
// directly and everything lands in one slice.
func (l *EntryList) UnmarshalXML(d *xml.Decoder, start xml.StartElement) error {
	for {
		token, err := d.Token()
		if err == io.EOF {
			return io.ErrUnexpectedEOF
		}
		if err != nil {
			return err
		}

		switch t := token.(type) {
		case xml.StartElement:
			// An element from another namespace is not an XTCE entry at all,
			// so it is skipped rather than kept as EntryOther: the order
			// argument below is about XTCE's own entry kinds.
			if t.Name.Space != Namespace {
				if err := d.Skip(); err != nil {
					return err
				}
				continue
			}

			var payload entryPayload
			if err := d.DecodeElement(&payload, &t); err != nil {
				return err
			}

			entry := Entry{
				ElementName:               t.Name.Local,
				ShortDescription:          payload.ShortDescription,
				LocationInContainerInBits: payload.LocationInContainerInBits,
				RepeatEntry:               payload.RepeatEntry,
				IncludeCondition:          payload.IncludeCondition,
			}

			switch t.Name.Local {
			case "ParameterRefEntry":
				entry.Kind = EntryParameterRef
				entry.Ref = payload.ParameterRef
			case "ContainerRefEntry":
				entry.Kind = EntryContainerRef
				entry.Ref = payload.ContainerRef
			default:
				// A kind this package does not model. It is kept in the list so
				// that entry order stays honest: dropping it would make the
				// remaining entries look adjacent when they are not.
				entry.Kind = EntryOther
				entry.Ref = payload.ParameterRef
				if entry.Ref == "" {
					entry.Ref = payload.ContainerRef
				}
			}
			l.Entries = append(l.Entries, entry)

		case xml.EndElement:
			return nil
		}
	}
}

// String renders an entry for a log line.
func (e Entry) String() string {
	if e.Ref == "" {
		return e.ElementName
	}
	return fmt.Sprintf("%s %s", e.ElementName, e.Ref)
}
