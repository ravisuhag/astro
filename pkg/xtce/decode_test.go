package xtce_test

import (
	"errors"
	"math"
	"strings"
	"testing"

	"github.com/ravisuhag/astro/pkg/xtce"
)

// oneField builds a database with a single parameter, so a test can state an
// encoding and the octets it should read.
func oneField(t *testing.T, typeXML string) *xtce.Layout {
	t.Helper()

	db := parse(t, wrap("Sat", `
  <ParameterTypeSet>`+typeXML+`</ParameterTypeSet>
  <ParameterSet><Parameter name="Value" parameterTypeRef="T"/></ParameterSet>
  <ContainerSet>
    <SequenceContainer name="C">
      <EntryList><ParameterRefEntry parameterRef="Value"/></EntryList>
    </SequenceContainer>
  </ContainerSet>`))

	layout, err := db.LayoutOf("/Sat/C")
	if err != nil {
		t.Fatalf("LayoutOf() = %v", err)
	}
	return layout
}

// readOne extracts the single field and returns it.
func readOne(t *testing.T, layout *xtce.Layout, packet []byte) xtce.Value {
	t.Helper()

	result, err := layout.Extract(packet)
	if err != nil {
		t.Fatalf("Extract() = %v", err)
	}
	return result.Values[0]
}

// TestIntegerEncodings walks the six integer encodings the schema lists.
//
// The same octet reads as a different number under each, which is the point:
// getting the encoding wrong is silent, and no round trip through this package
// would catch it.
func TestIntegerEncodings(t *testing.T) {
	tests := []struct {
		name     string
		encoding string
		bits     int
		packet   []byte
		want     any
	}{
		{"unsigned", "unsigned", 8, []byte{0xFF}, uint64(255)},
		{"twos complement positive", "twosComplement", 8, []byte{0x7F}, int64(127)},
		{"twos complement negative", "twosComplement", 8, []byte{0xFF}, int64(-1)},
		{"twos complement most negative", "twosComplement", 8, []byte{0x80}, int64(-128)},

		// 0x81 is sign 1, magnitude 1.
		{"sign magnitude negative", "signMagnitude", 8, []byte{0x81}, int64(-1)},
		{"sign magnitude positive", "signMagnitude", 8, []byte{0x01}, int64(1)},
		// Negative zero exists in sign magnitude and is still zero.
		{"sign magnitude negative zero", "signMagnitude", 8, []byte{0x80}, int64(0)},

		// In ones complement, -1 is the complement of 1: 0xFE.
		{"ones complement negative", "onesComplement", 8, []byte{0xFE}, int64(-1)},
		{"ones complement positive", "onesComplement", 8, []byte{0x01}, int64(1)},
		{"ones complement negative zero", "onesComplement", 8, []byte{0xFF}, int64(0)},

		// Four bits per decimal digit.
		{"BCD", "BCD", 16, []byte{0x12, 0x34}, int64(1234)},
		// Packed BCD spends its last nibble on the sign; 0xD is negative.
		{"packed BCD positive", "packedBCD", 16, []byte{0x12, 0x3C}, int64(123)},
		{"packed BCD negative", "packedBCD", 16, []byte{0x12, 0x3D}, int64(-123)},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			layout := oneField(t, `
    <IntegerParameterType name="T">
      <IntegerDataEncoding sizeInBits="`+itoa(test.bits)+`" encoding="`+test.encoding+`"/>
    </IntegerParameterType>`)

			value := readOne(t, layout, test.packet)
			if value.Err != nil {
				t.Fatalf("Err = %v", value.Err)
			}
			if value.Raw != test.want {
				t.Errorf("raw = %#v, want %#v", value.Raw, test.want)
			}
		})
	}
}

func TestBCDRejectsNonDigits(t *testing.T) {
	layout := oneField(t, `
    <IntegerParameterType name="T">
      <IntegerDataEncoding sizeInBits="8" encoding="BCD"/>
    </IntegerParameterType>`)

	// 0xA is not a decimal digit.
	if value := readOne(t, layout, []byte{0x1A}); value.Err == nil {
		t.Errorf("0x1A decoded as BCD %v", value.Raw)
	}
}

func TestBCDRejectsPartialNibbles(t *testing.T) {
	layout := oneField(t, `
    <IntegerParameterType name="T">
      <IntegerDataEncoding sizeInBits="6" encoding="BCD"/>
    </IntegerParameterType>`)

	if value := readOne(t, layout, []byte{0x12}); !errors.Is(value.Err, xtce.ErrUnsupportedEncoding) {
		t.Errorf("a 6-bit BCD field gave %v, want ErrUnsupportedEncoding", value.Err)
	}
}

// TestByteOrder checks that leastSignificantByteFirst swaps whole octets, and
// that a field which is not a whole number of octets is left alone rather than
// mangled.
func TestByteOrder(t *testing.T) {
	tests := []struct {
		name   string
		order  string
		bits   int
		packet []byte
		want   uint64
	}{
		{"big endian default", "", 16, []byte{0x12, 0x34}, 0x1234},
		{"big endian explicit", "mostSignificantByteFirst", 16, []byte{0x12, 0x34}, 0x1234},
		{"little endian 16", "leastSignificantByteFirst", 16, []byte{0x12, 0x34}, 0x3412},
		{"little endian 32", "leastSignificantByteFirst", 32, []byte{0x12, 0x34, 0x56, 0x78}, 0x78563412},
		// One octet has nothing to swap.
		{"little endian 8", "leastSignificantByteFirst", 8, []byte{0xAB}, 0xAB},
		// Twelve bits is not a whole number of octets, so the order does not
		// apply and the value is read as it stands.
		{"little endian 12", "leastSignificantByteFirst", 12, []byte{0x12, 0x30}, 0x123},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			attr := ""
			if test.order != "" {
				attr = ` byteOrder="` + test.order + `"`
			}
			layout := oneField(t, `
    <IntegerParameterType name="T" signed="false">
      <IntegerDataEncoding sizeInBits="`+itoa(test.bits)+`" encoding="unsigned"`+attr+`/>
    </IntegerParameterType>`)

			value := readOne(t, layout, test.packet)
			if value.Err != nil {
				t.Fatalf("Err = %v", value.Err)
			}
			if value.Raw != test.want {
				t.Errorf("raw = %#x, want %#x", value.Raw, test.want)
			}
		})
	}
}

// TestBitOrder checks that leastSignificantBitFirst reverses the field.
func TestBitOrder(t *testing.T) {
	layout := oneField(t, `
    <IntegerParameterType name="T" signed="false">
      <IntegerDataEncoding sizeInBits="8" encoding="unsigned" bitOrder="leastSignificantBitFirst"/>
    </IntegerParameterType>`)

	// 0b1000_0000 read least significant bit first is 1.
	value := readOne(t, layout, []byte{0x80})
	if value.Err != nil {
		t.Fatal(value.Err)
	}
	if value.Raw != uint64(1) {
		t.Errorf("raw = %#v, want uint64(1)", value.Raw)
	}
}

func TestStringEncodings(t *testing.T) {
	tests := []struct {
		name     string
		encoding string
		bits     int
		packet   []byte
		want     string
	}{
		{"UTF-8", "UTF-8", 40, []byte("HELLO"), "HELLO"},
		{"UTF-8 NUL padded", "UTF-8", 64, []byte("HI\x00\x00\x00\x00\x00\x00"), "HI"},
		{"UTF-16BE", "UTF-16BE", 32, []byte{0x00, 0x48, 0x00, 0x69}, "Hi"},
		{"UTF-16LE", "UTF-16LE", 32, []byte{0x48, 0x00, 0x69, 0x00}, "Hi"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			layout := oneField(t, `
    <StringParameterType name="T">
      <StringDataEncoding encoding="`+test.encoding+`">
        <SizeInBits><Fixed><FixedValue>`+itoa(test.bits)+`</FixedValue></Fixed></SizeInBits>
      </StringDataEncoding>
    </StringParameterType>`)

			value := readOne(t, layout, test.packet)
			if value.Err != nil {
				t.Fatalf("Err = %v", value.Err)
			}
			if text, ok := value.Text(); !ok || text != test.want {
				t.Errorf("= %q, want %q", value.Engineering, test.want)
			}
		})
	}
}

func TestStringRejectsInvalidUTF8(t *testing.T) {
	layout := oneField(t, `
    <StringParameterType name="T">
      <StringDataEncoding encoding="UTF-8">
        <SizeInBits><Fixed><FixedValue>16</FixedValue></Fixed></SizeInBits>
      </StringDataEncoding>
    </StringParameterType>`)

	if value := readOne(t, layout, []byte{0xFF, 0xFE}); !errors.Is(value.Err, xtce.ErrUnsupportedEncoding) {
		t.Errorf("invalid UTF-8 gave %v, want ErrUnsupportedEncoding", value.Err)
	}
}

// TestUnalignedBinaryField checks readBytes on an offset that is not a
// multiple of eight, which the aligned fast path does not cover.
func TestUnalignedBinaryField(t *testing.T) {
	db := parse(t, wrap("Sat", `
  <ParameterTypeSet>
    <IntegerParameterType name="U4" signed="false">
      <IntegerDataEncoding sizeInBits="4" encoding="unsigned"/>
    </IntegerParameterType>
    <BinaryParameterType name="Blob">
      <BinaryDataEncoding><SizeInBits><FixedValue>16</FixedValue></SizeInBits></BinaryDataEncoding>
    </BinaryParameterType>
  </ParameterTypeSet>
  <ParameterSet>
    <Parameter name="Nibble" parameterTypeRef="U4"/>
    <Parameter name="Data" parameterTypeRef="Blob"/>
  </ParameterSet>
  <ContainerSet>
    <SequenceContainer name="C">
      <EntryList>
        <ParameterRefEntry parameterRef="Nibble"/>
        <ParameterRefEntry parameterRef="Data"/>
      </EntryList>
    </SequenceContainer>
  </ContainerSet>`))

	layout, err := db.LayoutOf("/Sat/C")
	if err != nil {
		t.Fatal(err)
	}

	// 0xA then the 16 bits BCDE, which start at bit 4.
	packet, err := layout.Extract([]byte{0xAB, 0xCD, 0xE0})
	if err != nil {
		t.Fatal(err)
	}

	raw, ok := packet.Values[1].Bytes()
	if !ok {
		t.Fatalf("not binary: %v", packet.Values[1].Err)
	}
	if len(raw) != 2 || raw[0] != 0xBC || raw[1] != 0xDE {
		t.Errorf("unaligned binary = %X, want BCDE", raw)
	}
}

func TestSplineCalibrator(t *testing.T) {
	const spline = `
    <FloatParameterType name="T" sizeInBits="64">
      <IntegerDataEncoding sizeInBits="8" encoding="unsigned">
        <DefaultCalibrator>
          <SplineCalibrator order="1"%s>
            <SplinePoint raw="0" calibrated="0"/>
            <SplinePoint raw="100" calibrated="10"/>
            <SplinePoint raw="200" calibrated="50"/>
          </SplineCalibrator>
        </DefaultCalibrator>
      </IntegerDataEncoding>
    </FloatParameterType>`

	tests := []struct {
		name        string
		extrapolate bool
		raw         byte
		want        float64
	}{
		{"on a point", false, 100, 10},
		{"first segment", false, 50, 5},
		// The second segment is steeper: 10 to 50 over 100 to 200.
		{"second segment", false, 150, 30},
		// Beyond the last point, clamped by default.
		{"clamped high", false, 250, 50},
		{"clamped low", false, 0, 0},
		// With extrapolate, the end segment is extended: 50 + 50*0.4 = 70.
		{"extrapolated high", true, 250, 70},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			attr := ""
			if test.extrapolate {
				attr = ` extrapolate="true"`
			}
			layout := oneField(t, strings.Replace(spline, "%s", attr, 1))

			value := readOne(t, layout, []byte{test.raw})
			if value.Err != nil {
				t.Fatalf("Err = %v", value.Err)
			}
			got, ok := value.Float()
			if !ok {
				t.Fatalf("engineering value %#v is not a number", value.Engineering)
			}
			if math.Abs(got-test.want) > 1e-9 {
				t.Errorf("raw %d calibrated to %v, want %v", test.raw, got, test.want)
			}
		})
	}
}

// TestSplineNeedsTwoPoints checks the refusal rather than an interpolation
// between a point and nothing.
func TestSplineNeedsTwoPoints(t *testing.T) {
	layout := oneField(t, `
    <FloatParameterType name="T" sizeInBits="64">
      <IntegerDataEncoding sizeInBits="8" encoding="unsigned">
        <DefaultCalibrator>
          <SplineCalibrator><SplinePoint raw="0" calibrated="0"/></SplineCalibrator>
        </DefaultCalibrator>
      </IntegerDataEncoding>
    </FloatParameterType>`)

	if value := readOne(t, layout, []byte{5}); !errors.Is(value.Err, xtce.ErrUnsupportedCalibrator) {
		t.Errorf("a one-point spline gave %v, want ErrUnsupportedCalibrator", value.Err)
	}
}

// TestSplinePointsNeedNotBeSorted checks that the schema's silence on ordering
// is handled, rather than the search walking off a jumbled list.
func TestSplinePointsNeedNotBeSorted(t *testing.T) {
	layout := oneField(t, `
    <FloatParameterType name="T" sizeInBits="64">
      <IntegerDataEncoding sizeInBits="8" encoding="unsigned">
        <DefaultCalibrator>
          <SplineCalibrator>
            <SplinePoint raw="200" calibrated="50"/>
            <SplinePoint raw="0" calibrated="0"/>
            <SplinePoint raw="100" calibrated="10"/>
          </SplineCalibrator>
        </DefaultCalibrator>
      </IntegerDataEncoding>
    </FloatParameterType>`)

	value := readOne(t, layout, []byte{150})
	if value.Err != nil {
		t.Fatal(value.Err)
	}
	if got, _ := value.Float(); math.Abs(got-30) > 1e-9 {
		t.Errorf("= %v, want 30", got)
	}
}

// A MathOperationCalibrator on a field is evaluated like any other
// calibrator, so the engineering value comes out of the expression.
func TestMathOperationCalibratorIsApplied(t *testing.T) {
	layout := oneField(t, `
    <FloatParameterType name="T" sizeInBits="64">
      <IntegerDataEncoding sizeInBits="8" encoding="unsigned">
        <DefaultCalibrator>
          <MathOperationCalibrator>
            <ThisParameterOperand/>
            <ValueOperand>0.5</ValueOperand>
            <Operator>*</Operator>
          </MathOperationCalibrator>
        </DefaultCalibrator>
      </IntegerDataEncoding>
    </FloatParameterType>`)

	value := readOne(t, layout, []byte{5})
	if value.Err != nil {
		t.Fatalf("a MathOperationCalibrator gave %v", value.Err)
	}
	if value.Engineering != 2.5 {
		t.Errorf("Engineering = %v, want 2.5", value.Engineering)
	}
}

// An expression that does not balance is still refused, because there is no
// value it could honestly produce.
func TestMathOperationCalibratorRefusesAnUnbalancedExpression(t *testing.T) {
	layout := oneField(t, `
    <FloatParameterType name="T" sizeInBits="64">
      <IntegerDataEncoding sizeInBits="8" encoding="unsigned">
        <DefaultCalibrator>
          <MathOperationCalibrator><ValueOperand>1</ValueOperand><ValueOperand>2</ValueOperand></MathOperationCalibrator>
        </DefaultCalibrator>
      </IntegerDataEncoding>
    </FloatParameterType>`)

	if value := readOne(t, layout, []byte{5}); !errors.Is(value.Err, xtce.ErrInvalidMathOperation) {
		t.Errorf("an unbalanced expression gave %v, want ErrInvalidMathOperation", value.Err)
	}
}

// TestAbsoluteTimeScaling checks that a clock reading is scaled and offset by
// what the Encoding element says.
func TestAbsoluteTimeScaling(t *testing.T) {
	layout := oneField(t, `
    <AbsoluteTimeParameterType name="T">
      <Encoding units="seconds" scale="0.001" offset="946684800">
        <IntegerDataEncoding sizeInBits="32" encoding="unsigned"/>
      </Encoding>
      <ReferenceTime><Epoch>UNIX</Epoch></ReferenceTime>
    </AbsoluteTimeParameterType>`)

	// A count of 2000 milliseconds past the offset.
	value := readOne(t, layout, []byte{0x00, 0x00, 0x07, 0xD0})
	if value.Err != nil {
		t.Fatal(value.Err)
	}
	got, ok := value.Float()
	if !ok {
		t.Fatalf("engineering value %#v is not a number", value.Engineering)
	}
	if want := 946684802.0; math.Abs(got-want) > 1e-6 {
		t.Errorf("= %v, want %v", got, want)
	}
}

func TestUnsupportedFloatEncodingIsReported(t *testing.T) {
	layout := oneField(t, `
    <FloatParameterType name="T" sizeInBits="32">
      <FloatDataEncoding sizeInBits="32" encoding="MILSTD_1750A"/>
    </FloatParameterType>`)

	if value := readOne(t, layout, []byte{0, 0, 0, 0}); !errors.Is(value.Err, xtce.ErrUnsupportedEncoding) {
		t.Errorf("MILSTD_1750A gave %v, want ErrUnsupportedEncoding", value.Err)
	}
}

func FuzzExtractNeverPanics(f *testing.F) {
	f.Add([]byte{200, 0x01, 0x90, 0x50})
	f.Add([]byte{})
	f.Add([]byte{0xFF, 0xFF, 0xFF, 0xFF, 0xFF})

	db, err := xtce.Load(strings.NewReader(wrap("Sat", basicDB)))
	if err != nil {
		f.Fatal(err)
	}
	layout, err := db.LayoutOf("/Sat/Housekeeping")
	if err != nil {
		f.Fatal(err)
	}

	f.Fuzz(func(t *testing.T, packet []byte) {
		if len(packet) > 4096 {
			return
		}
		result, err := layout.Extract(packet)
		if err != nil {
			return
		}
		if len(result.Values) != len(layout.Fields) {
			t.Fatalf("got %d values for %d fields", len(result.Values), len(layout.Fields))
		}
		_ = result.String()
	})
}
