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
// modeled; ArrayParameterType, AggregateParameterType and
// RelativeTimeParameterType are not, and a file using them still loads —
// their types simply do not appear here, so a parameter pointing at one fails
// Validate with an unresolved reference. The coverage matrix records this.
type ParameterTypeSet struct {
	IntegerTypes      []*IntegerParameterType      `xml:"IntegerParameterType"`
	FloatTypes        []*FloatParameterType        `xml:"FloatParameterType"`
	EnumeratedTypes   []*EnumeratedParameterType   `xml:"EnumeratedParameterType"`
	StringTypes       []*StringParameterType       `xml:"StringParameterType"`
	BinaryTypes       []*BinaryParameterType       `xml:"BinaryParameterType"`
	BooleanTypes      []*BooleanParameterType      `xml:"BooleanParameterType"`
	AbsoluteTimeTypes []*AbsoluteTimeParameterType `xml:"AbsoluteTimeParameterType"`
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

	LongDescription string   `xml:"LongDescription"`
	UnitSet         *UnitSet `xml:"UnitSet"`

	IntegerDataEncoding *IntegerDataEncoding `xml:"IntegerDataEncoding"`
	FloatDataEncoding   *FloatDataEncoding   `xml:"FloatDataEncoding"`
	StringDataEncoding  *StringDataEncoding  `xml:"StringDataEncoding"`
	BinaryDataEncoding  *BinaryDataEncoding  `xml:"BinaryDataEncoding"`
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
	Units []Unit `xml:"Unit"`
}

// Unit is one unit, with an optional power and factor.
type Unit struct {
	Power       float64 `xml:"power,attr"`
	Factor      string  `xml:"factor,attr"`
	Description string  `xml:"description,attr"`
	Value       string  `xml:",chardata"`
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

// FloatParameterType is a real-number parameter.
type FloatParameterType struct {
	baseDataType
	// SizeInBits is 32 or 64, defaulting to 32.
	SizeInBits   uint   `xml:"sizeInBits,attr"`
	InitialValue string `xml:"initialValue,attr"`
}

// TypeKind names the kind.
func (t *FloatParameterType) TypeKind() string { return "float" }

// EnumeratedParameterType maps raw values to labels.
type EnumeratedParameterType struct {
	baseDataType
	InitialValue string `xml:"initialValue,attr"`
	// EnumerationList is required by the schema.
	EnumerationList EnumerationList `xml:"EnumerationList"`
}

// TypeKind names the kind.
func (t *EnumeratedParameterType) TypeKind() string { return "enumerated" }

// EnumerationList holds the value-to-label mapping.
type EnumerationList struct {
	Enumerations []Enumeration `xml:"Enumeration"`
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
	// OneStringValue defaults to "True" and ZeroStringValue to "False".
	OneStringValue  string `xml:"oneStringValue,attr"`
	ZeroStringValue string `xml:"zeroStringValue,attr"`
}

// TypeKind names the kind.
func (t *BooleanParameterType) TypeKind() string { return "boolean" }

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

	LongDescription string `xml:"LongDescription"`

	// Encoding wraps the data encoding with the scaling that turns a raw count
	// into a time.
	Encoding_ *TimeEncoding `xml:"Encoding"`
	// ReferenceTime says what the count is measured from.
	ReferenceTime *ReferenceTime `xml:"ReferenceTime"`
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

// TimeEncoding is the schema's EncodingType: a data encoding plus the units,
// scale and offset that turn the raw count into a time.
type TimeEncoding struct {
	// Units defaults to "seconds".
	Units string `xml:"units,attr"`
	// Scale defaults to 1 and Offset to 0. Both are pointers so that an
	// explicit zero is distinguishable from absent.
	Scale  *float64 `xml:"scale,attr"`
	Offset *float64 `xml:"offset,attr"`

	IntegerDataEncoding *IntegerDataEncoding `xml:"IntegerDataEncoding"`
	FloatDataEncoding   *FloatDataEncoding   `xml:"FloatDataEncoding"`
	StringDataEncoding  *StringDataEncoding  `xml:"StringDataEncoding"`
	BinaryDataEncoding  *BinaryDataEncoding  `xml:"BinaryDataEncoding"`
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
	Epoch string `xml:"Epoch"`
	// OffsetFrom names another time parameter to count from. Kept raw.
	OffsetFrom *RawXML `xml:"OffsetFrom"`
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

	DefaultCalibrator *Calibrator `xml:"DefaultCalibrator"`
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

	DefaultCalibrator *Calibrator `xml:"DefaultCalibrator"`
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

	SizeInBits *StringSize `xml:"SizeInBits"`
	// Variable is the delimited form, kept raw.
	Variable *RawXML `xml:"Variable"`
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
	Fixed *int64 `xml:"Fixed>FixedValue"`
	// TerminationChar and LeadingSize are the other forms the schema allows.
	TerminationChar string  `xml:"TerminationChar"`
	LeadingSize     *RawXML `xml:"LeadingSize"`
}

// BinaryDataEncoding writes raw octets.
type BinaryDataEncoding struct {
	commonEncoding
	// SizeInBits is required by the schema and may be dynamic.
	SizeInBits *IntegerValue `xml:"SizeInBits"`
}

// Calibrator turns a raw value into an engineering one.
type Calibrator struct {
	Name string `xml:"name,attr"`

	Polynomial *PolynomialCalibrator `xml:"PolynomialCalibrator"`
	Spline     *SplineCalibrator     `xml:"SplineCalibrator"`
	// MathOperation is the third form. It is out of scope — evaluating an
	// expression tree is a different job — so it is kept raw and Kind reports
	// it so a caller is not misled into thinking the type has no calibrator.
	MathOperation *RawXML `xml:"MathOperationCalibrator"`
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
		return "math operation (not modeled)"
	default:
		return "empty"
	}
}

// PolynomialCalibrator is a sum of terms: coefficient times raw to the power
// of exponent.
type PolynomialCalibrator struct {
	Terms []Term `xml:"Term"`
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
	Points []SplinePoint `xml:"SplinePoint"`
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

	LocationInContainerInBits *LocationInContainer `xml:"LocationInContainerInBits"`
	RepeatEntry               *Repeat              `xml:"RepeatEntry"`
	IncludeCondition          *RawXML              `xml:"IncludeCondition"`
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
