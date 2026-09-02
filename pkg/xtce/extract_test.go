package xtce_test

import (
	"bytes"
	"errors"
	"math"
	"strings"
	"testing"

	"github.com/ravisuhag/astro/pkg/xtce"
)

// parse reads a document written inline by a test. The fixture-based load in
// load_test.go covers the files on disk; these tests want the database and the
// packet side by side.
func parse(t *testing.T, document string) *xtce.SpaceSystem {
	t.Helper()

	db, err := xtce.Load(strings.NewReader(document))
	if err != nil {
		t.Fatalf("Load() = %v", err)
	}
	return db
}

// wrap puts a fragment inside a SpaceSystem with the right namespace, so each
// test only has to write the part it cares about.
func wrap(name, body string) string {
	return `<?xml version="1.0" encoding="UTF-8"?>
<SpaceSystem xmlns="http://www.omg.org/spec/XTCE/20180204" name="` + name + `">
  <TelemetryMetaData>` + body + `</TelemetryMetaData>
</SpaceSystem>`
}

// A database with one packet: an 8-bit unsigned count, a 16-bit signed
// temperature with a polynomial calibrator, a 3-bit enumeration and a 1-bit
// flag. The last two share an octet with five bits of padding, which is what
// makes it a bit-level layout rather than an octet one.
const basicDB = `
  <ParameterTypeSet>
    <IntegerParameterType name="CountType" signed="false">
      <IntegerDataEncoding sizeInBits="8" encoding="unsigned"/>
    </IntegerParameterType>
    <FloatParameterType name="TempType" sizeInBits="32">
      <IntegerDataEncoding sizeInBits="16" encoding="twosComplement">
        <DefaultCalibrator>
          <PolynomialCalibrator>
            <Term coefficient="-40" exponent="0"/>
            <Term coefficient="0.25" exponent="1"/>
          </PolynomialCalibrator>
        </DefaultCalibrator>
      </IntegerDataEncoding>
    </FloatParameterType>
    <EnumeratedParameterType name="ModeType">
      <IntegerDataEncoding sizeInBits="3" encoding="unsigned"/>
      <EnumerationList>
        <Enumeration value="0" label="SAFE"/>
        <Enumeration value="1" label="NOMINAL"/>
        <Enumeration value="2" maxValue="4" label="SCIENCE"/>
      </EnumerationList>
    </EnumeratedParameterType>
    <BooleanParameterType name="FlagType" oneStringValue="ON" zeroStringValue="OFF">
      <IntegerDataEncoding sizeInBits="1" encoding="unsigned"/>
    </BooleanParameterType>
  </ParameterTypeSet>
  <ParameterSet>
    <Parameter name="Count" parameterTypeRef="CountType"/>
    <Parameter name="Temp" parameterTypeRef="TempType"/>
    <Parameter name="Mode" parameterTypeRef="ModeType"/>
    <Parameter name="Heater" parameterTypeRef="FlagType"/>
  </ParameterSet>
  <ContainerSet>
    <SequenceContainer name="Housekeeping">
      <EntryList>
        <ParameterRefEntry parameterRef="Count"/>
        <ParameterRefEntry parameterRef="Temp"/>
        <ParameterRefEntry parameterRef="Mode"/>
        <ParameterRefEntry parameterRef="Heater"/>
      </EntryList>
    </SequenceContainer>
  </ContainerSet>`

func TestLayoutPlacesFields(t *testing.T) {
	db := parse(t, wrap("Sat", basicDB))

	layout, err := db.LayoutOf("/Sat/Housekeeping")
	if err != nil {
		t.Fatalf("LayoutOf() = %v", err)
	}

	want := []struct {
		name          string
		offset, width uint
	}{
		{"/Sat/Count", 0, 8},
		{"/Sat/Temp", 8, 16},
		{"/Sat/Mode", 24, 3},
		{"/Sat/Heater", 27, 1},
	}

	if len(layout.Fields) != len(want) {
		t.Fatalf("layout has %d fields, want %d: %v", len(layout.Fields), len(want), layout.Fields)
	}
	for i, expected := range want {
		got := layout.Fields[i]
		if got.Name != expected.name || got.BitOffset != expected.offset || got.BitSize != expected.width {
			t.Errorf("field %d = %s, want %s at bit %d, %d bits",
				i, got, expected.name, expected.offset, expected.width)
		}
	}

	if layout.BitSize != 28 {
		t.Errorf("layout is %d bits, want 28", layout.BitSize)
	}
}

func TestExtractBasic(t *testing.T) {
	db := parse(t, wrap("Sat", basicDB))

	layout, err := db.LayoutOf("/Sat/Housekeeping")
	if err != nil {
		t.Fatal(err)
	}

	// Count 200; Temp 0x0190 = 400, calibrated to -40 + 0.25*400 = 60;
	// Mode 2 (SCIENCE by range); Heater 1 (ON). The last two share an octet:
	// 010 1 0000.
	packet := []byte{200, 0x01, 0x90, 0b0101_0000}

	got, err := layout.Extract(packet)
	if err != nil {
		t.Fatalf("Extract() = %v", err)
	}
	if err := got.Err(); err != nil {
		t.Fatalf("a field failed: %v", err)
	}

	tests := []struct {
		name string
		raw  any
		eng  any
	}{
		{"/Sat/Count", uint64(200), uint64(200)},
		{"/Sat/Temp", int64(400), float64(60)},
		{"/Sat/Mode", uint64(2), "SCIENCE"},
		{"/Sat/Heater", uint64(1), "ON"},
	}

	for _, test := range tests {
		value, ok := got.Get(test.name)
		if !ok {
			t.Errorf("%s is missing", test.name)
			continue
		}
		if value.Raw != test.raw {
			t.Errorf("%s raw = %#v, want %#v", test.name, value.Raw, test.raw)
		}
		if value.Engineering != test.eng {
			t.Errorf("%s engineering = %#v, want %#v", test.name, value.Engineering, test.eng)
		}
	}
}

// TestExtractNegativeTemperature checks the two's complement path and that the
// calibrator runs on the signed value, not the bit pattern.
func TestExtractNegativeTemperature(t *testing.T) {
	db := parse(t, wrap("Sat", basicDB))
	layout, err := db.LayoutOf("/Sat/Housekeeping")
	if err != nil {
		t.Fatal(err)
	}

	// 0xFF38 is -200, so -40 + 0.25*-200 = -90.
	got, err := layout.Extract([]byte{0, 0xFF, 0x38, 0})
	if err != nil {
		t.Fatal(err)
	}

	value, _ := got.Get("Temp")
	if value.Raw != int64(-200) {
		t.Errorf("raw = %#v, want int64(-200)", value.Raw)
	}
	if temp, ok := value.Float(); !ok || temp != -90 {
		t.Errorf("engineering = %v, want -90", value.Engineering)
	}
}

// TestGetByBareName checks the fallback in Get.
func TestGetByBareName(t *testing.T) {
	db := parse(t, wrap("Sat", basicDB))
	layout, _ := db.LayoutOf("/Sat/Housekeeping")

	packet, err := layout.Extract([]byte{7, 0, 0, 0})
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := packet.Get("Count"); !ok {
		t.Error("Get by bare name found nothing")
	}
	if _, ok := packet.Get("NoSuchParameter"); ok {
		t.Error("Get invented a parameter")
	}
}

func TestExtractRejectsShortPacket(t *testing.T) {
	db := parse(t, wrap("Sat", basicDB))
	layout, _ := db.LayoutOf("/Sat/Housekeeping")

	if _, err := layout.Extract([]byte{1, 2}); !errors.Is(err, xtce.ErrPacketTooShort) {
		t.Errorf("Extract(short) = %v, want ErrPacketTooShort", err)
	}
}

// TestExtractIgnoresTrailingOctets checks that a container need not cover the
// whole packet, which XTCE does not require.
func TestExtractIgnoresTrailingOctets(t *testing.T) {
	db := parse(t, wrap("Sat", basicDB))
	layout, _ := db.LayoutOf("/Sat/Housekeeping")

	packet, err := layout.Extract([]byte{9, 0, 0, 0, 0xDE, 0xAD, 0xBE, 0xEF})
	if err != nil {
		t.Fatal(err)
	}
	if len(packet.Values) != 4 {
		t.Errorf("got %d values, want 4", len(packet.Values))
	}
}

// TestInheritanceOrdersBaseFirst is the point of container inheritance: the
// base's entries come before the derived container's own.
func TestInheritanceOrdersBaseFirst(t *testing.T) {
	db := parse(t, wrap("Sat", `
  <ParameterTypeSet>
    <IntegerParameterType name="U8" signed="false">
      <IntegerDataEncoding sizeInBits="8" encoding="unsigned"/>
    </IntegerParameterType>
  </ParameterTypeSet>
  <ParameterSet>
    <Parameter name="APID" parameterTypeRef="U8"/>
    <Parameter name="Body" parameterTypeRef="U8"/>
  </ParameterSet>
  <ContainerSet>
    <SequenceContainer name="Header" abstract="true">
      <EntryList><ParameterRefEntry parameterRef="APID"/></EntryList>
    </SequenceContainer>
    <SequenceContainer name="Telemetry">
      <EntryList><ParameterRefEntry parameterRef="Body"/></EntryList>
      <BaseContainer containerRef="Header"/>
    </SequenceContainer>
  </ContainerSet>`))

	layout, err := db.LayoutOf("/Sat/Telemetry")
	if err != nil {
		t.Fatal(err)
	}
	if len(layout.Fields) != 2 {
		t.Fatalf("got %d fields, want 2", len(layout.Fields))
	}
	if layout.Fields[0].Name != "/Sat/APID" || layout.Fields[0].BitOffset != 0 {
		t.Errorf("first field is %s, want the base's APID at bit 0", layout.Fields[0])
	}
	if layout.Fields[1].Name != "/Sat/Body" || layout.Fields[1].BitOffset != 8 {
		t.Errorf("second field is %s, want Body at bit 8", layout.Fields[1])
	}
}

// TestContainerRefEntrySplices checks that a referenced container's entries go
// in at the point of reference rather than being appended.
func TestContainerRefEntrySplices(t *testing.T) {
	db := parse(t, wrap("Sat", `
  <ParameterTypeSet>
    <IntegerParameterType name="U8" signed="false">
      <IntegerDataEncoding sizeInBits="8" encoding="unsigned"/>
    </IntegerParameterType>
  </ParameterTypeSet>
  <ParameterSet>
    <Parameter name="First" parameterTypeRef="U8"/>
    <Parameter name="Middle" parameterTypeRef="U8"/>
    <Parameter name="Last" parameterTypeRef="U8"/>
  </ParameterSet>
  <ContainerSet>
    <SequenceContainer name="Inner">
      <EntryList><ParameterRefEntry parameterRef="Middle"/></EntryList>
    </SequenceContainer>
    <SequenceContainer name="Outer">
      <EntryList>
        <ParameterRefEntry parameterRef="First"/>
        <ContainerRefEntry containerRef="Inner"/>
        <ParameterRefEntry parameterRef="Last"/>
      </EntryList>
    </SequenceContainer>
  </ContainerSet>`))

	layout, err := db.LayoutOf("/Sat/Outer")
	if err != nil {
		t.Fatal(err)
	}

	var names []string
	for _, field := range layout.Fields {
		names = append(names, field.Name)
	}
	want := []string{"/Sat/First", "/Sat/Middle", "/Sat/Last"}
	if strings.Join(names, ",") != strings.Join(want, ",") {
		t.Errorf("fields are %v, want %v", names, want)
	}
}

// TestLocationInContainerPlacesEntries checks the fixed-offset placements:
// containerStart absolute, and previousEntry as a relative skip.
func TestLocationInContainerPlacesEntries(t *testing.T) {
	db := parse(t, wrap("Sat", `
  <ParameterTypeSet>
    <IntegerParameterType name="U8" signed="false">
      <IntegerDataEncoding sizeInBits="8" encoding="unsigned"/>
    </IntegerParameterType>
  </ParameterTypeSet>
  <ParameterSet>
    <Parameter name="A" parameterTypeRef="U8"/>
    <Parameter name="B" parameterTypeRef="U8"/>
    <Parameter name="C" parameterTypeRef="U8"/>
  </ParameterSet>
  <ContainerSet>
    <SequenceContainer name="Spaced">
      <EntryList>
        <ParameterRefEntry parameterRef="A"/>
        <ParameterRefEntry parameterRef="B">
          <LocationInContainerInBits referenceLocation="previousEntry">
            <FixedValue>16</FixedValue>
          </LocationInContainerInBits>
        </ParameterRefEntry>
        <ParameterRefEntry parameterRef="C">
          <LocationInContainerInBits referenceLocation="containerStart">
            <FixedValue>8</FixedValue>
          </LocationInContainerInBits>
        </ParameterRefEntry>
      </EntryList>
    </SequenceContainer>
  </ContainerSet>`))

	layout, err := db.LayoutOf("/Sat/Spaced")
	if err != nil {
		t.Fatal(err)
	}

	// A at 0..8. B skips 16 bits past the cursor, so 24. C is 8 bits from the
	// container's start, so 8, entries may point backwards.
	want := []uint{0, 24, 8}
	for i, offset := range want {
		if layout.Fields[i].BitOffset != offset {
			t.Errorf("field %d is at bit %d, want %d", i, layout.Fields[i].BitOffset, offset)
		}
	}
	// BitSize is past the furthest field, which is B at 24 plus 8.
	if layout.BitSize != 32 {
		t.Errorf("layout is %d bits, want 32", layout.BitSize)
	}
}

func TestRepeatEntry(t *testing.T) {
	db := parse(t, wrap("Sat", `
  <ParameterTypeSet>
    <IntegerParameterType name="U8" signed="false">
      <IntegerDataEncoding sizeInBits="8" encoding="unsigned"/>
    </IntegerParameterType>
  </ParameterTypeSet>
  <ParameterSet><Parameter name="Sample" parameterTypeRef="U8"/></ParameterSet>
  <ContainerSet>
    <SequenceContainer name="Samples">
      <EntryList>
        <ParameterRefEntry parameterRef="Sample">
          <RepeatEntry><Count><FixedValue>4</FixedValue></Count></RepeatEntry>
        </ParameterRefEntry>
      </EntryList>
    </SequenceContainer>
  </ContainerSet>`))

	layout, err := db.LayoutOf("/Sat/Samples")
	if err != nil {
		t.Fatal(err)
	}
	if len(layout.Fields) != 4 {
		t.Fatalf("got %d fields, want 4", len(layout.Fields))
	}
	for i, field := range layout.Fields {
		if field.BitOffset != uint(i)*8 {
			t.Errorf("repeat %d is at bit %d, want %d", i, field.BitOffset, i*8)
		}
	}

	packet, err := layout.Extract([]byte{10, 20, 30, 40})
	if err != nil {
		t.Fatal(err)
	}
	for i, want := range []uint64{10, 20, 30, 40} {
		if packet.Values[i].Raw != want {
			t.Errorf("sample %d = %v, want %d", i, packet.Values[i].Raw, want)
		}
	}
}

// TestLayoutRefusesDynamicSize is the honest failure: a delimited string means
// the layout depends on the packet, which a layout built ahead of time cannot
// settle.
func TestLayoutRefusesDynamicSize(t *testing.T) {
	db := parse(t, wrap("Sat", `
  <ParameterTypeSet>
    <StringParameterType name="TextType">
      <StringDataEncoding encoding="UTF-8">
        <SizeInBits><TerminationChar>00</TerminationChar></SizeInBits>
      </StringDataEncoding>
    </StringParameterType>
  </ParameterTypeSet>
  <ParameterSet><Parameter name="Label" parameterTypeRef="TextType"/></ParameterSet>
  <ContainerSet>
    <SequenceContainer name="Labelled">
      <EntryList><ParameterRefEntry parameterRef="Label"/></EntryList>
    </SequenceContainer>
  </ContainerSet>`))

	if _, err := db.LayoutOf("/Sat/Labelled"); !errors.Is(err, xtce.ErrDynamicSize) {
		t.Errorf("LayoutOf(delimited string) = %v, want ErrDynamicSize", err)
	}
}

// TestLayoutRefusesSelfSplice guards the recursion.
func TestLayoutRefusesSelfSplice(t *testing.T) {
	db := parse(t, wrap("Sat", `
  <ParameterTypeSet/>
  <ParameterSet/>
  <ContainerSet>
    <SequenceContainer name="Loop">
      <EntryList><ContainerRefEntry containerRef="Loop"/></EntryList>
    </SequenceContainer>
  </ContainerSet>`))

	if _, err := db.LayoutOf("/Sat/Loop"); !errors.Is(err, xtce.ErrContainerCycle) {
		t.Errorf("LayoutOf(self-splicing container) = %v, want ErrContainerCycle", err)
	}
}

// TestExtractKeepsGoingPastABadField checks that one unsupported encoding does
// not hide the fields after it.
func TestExtractKeepsGoingPastABadField(t *testing.T) {
	db := parse(t, wrap("Sat", `
  <ParameterTypeSet>
    <IntegerParameterType name="U8" signed="false">
      <IntegerDataEncoding sizeInBits="8" encoding="unsigned"/>
    </IntegerParameterType>
    <IntegerParameterType name="BadBCD" signed="false">
      <IntegerDataEncoding sizeInBits="8" encoding="BCD"/>
    </IntegerParameterType>
  </ParameterTypeSet>
  <ParameterSet>
    <Parameter name="Bad" parameterTypeRef="BadBCD"/>
    <Parameter name="Good" parameterTypeRef="U8"/>
  </ParameterSet>
  <ContainerSet>
    <SequenceContainer name="Mixed">
      <EntryList>
        <ParameterRefEntry parameterRef="Bad"/>
        <ParameterRefEntry parameterRef="Good"/>
      </EntryList>
    </SequenceContainer>
  </ContainerSet>`))

	layout, err := db.LayoutOf("/Sat/Mixed")
	if err != nil {
		t.Fatal(err)
	}

	// 0xFF is not two decimal digits, so the BCD field fails.
	packet, err := layout.Extract([]byte{0xFF, 42})
	if err != nil {
		t.Fatalf("Extract() = %v, want the packet with one bad field", err)
	}
	if packet.Values[0].Err == nil {
		t.Error("the BCD field decoded 0xFF as digits")
	}
	if packet.Values[1].Raw != uint64(42) {
		t.Errorf("the field after the bad one is %v, want 42", packet.Values[1].Raw)
	}
	if packet.Err() == nil {
		t.Error("Err() reported nothing despite a failed field")
	}
}

func TestValueString(t *testing.T) {
	db := parse(t, wrap("Sat", basicDB))
	layout, _ := db.LayoutOf("/Sat/Housekeeping")

	packet, err := layout.Extract([]byte{200, 0x01, 0x90, 0b0101_0000})
	if err != nil {
		t.Fatal(err)
	}

	text := packet.String()
	for _, want := range []string{"/Sat/Count = 200", "/Sat/Temp = 60", "/Sat/Mode = SCIENCE", "/Sat/Heater = ON"} {
		if !strings.Contains(text, want) {
			t.Errorf("packet listing is missing %q:\n%s", want, text)
		}
	}
}

// TestUnlistedEnumerationValue checks the decision to show the number rather
// than fail: missions do send values their database does not list.
func TestUnlistedEnumerationValue(t *testing.T) {
	db := parse(t, wrap("Sat", basicDB))
	layout, _ := db.LayoutOf("/Sat/Housekeeping")

	// Mode 7, which no Enumeration covers.
	packet, err := layout.Extract([]byte{0, 0, 0, 0b1110_0000})
	if err != nil {
		t.Fatal(err)
	}
	value, _ := packet.Get("Mode")
	if value.Err != nil {
		t.Fatalf("an unlisted enumeration value failed: %v", value.Err)
	}
	if text, _ := value.Text(); text != "7" {
		t.Errorf("unlisted value rendered as %q, want \"7\"", text)
	}
}

func TestBytesAccessor(t *testing.T) {
	db := parse(t, wrap("Sat", `
  <ParameterTypeSet>
    <BinaryParameterType name="BlobType">
      <BinaryDataEncoding><SizeInBits><FixedValue>24</FixedValue></SizeInBits></BinaryDataEncoding>
    </BinaryParameterType>
  </ParameterTypeSet>
  <ParameterSet><Parameter name="Blob" parameterTypeRef="BlobType"/></ParameterSet>
  <ContainerSet>
    <SequenceContainer name="WithBlob">
      <EntryList><ParameterRefEntry parameterRef="Blob"/></EntryList>
    </SequenceContainer>
  </ContainerSet>`))

	layout, err := db.LayoutOf("/Sat/WithBlob")
	if err != nil {
		t.Fatal(err)
	}
	packet, err := layout.Extract([]byte{0xDE, 0xAD, 0xBE})
	if err != nil {
		t.Fatal(err)
	}

	raw, ok := packet.Values[0].Bytes()
	if !ok {
		t.Fatal("Bytes() said the binary field is not binary")
	}
	if !bytes.Equal(raw, []byte{0xDE, 0xAD, 0xBE}) {
		t.Errorf("Bytes() = %X, want DEADBE", raw)
	}
}

func TestFloatEncodings(t *testing.T) {
	tests := []struct {
		name  string
		bits  uint
		bytes []byte
		want  float64
	}{
		{"binary32", 32, []byte{0x40, 0x49, 0x0F, 0xDB}, float64(float32(math.Pi))},
		{"binary64", 64, []byte{0x40, 0x09, 0x21, 0xFB, 0x54, 0x44, 0x2D, 0x18}, math.Pi},
		{"binary16", 16, []byte{0x3C, 0x00}, 1.0},
		{"binary16 subnormal", 16, []byte{0x00, 0x01}, math.Pow(2, -24)},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			db := parse(t, wrap("Sat", `
  <ParameterTypeSet>
    <FloatParameterType name="F" sizeInBits="64">
      <FloatDataEncoding sizeInBits="`+itoa(int(test.bits))+`"/>
    </FloatParameterType>
  </ParameterTypeSet>
  <ParameterSet><Parameter name="Value" parameterTypeRef="F"/></ParameterSet>
  <ContainerSet>
    <SequenceContainer name="C">
      <EntryList><ParameterRefEntry parameterRef="Value"/></EntryList>
    </SequenceContainer>
  </ContainerSet>`))

			layout, err := db.LayoutOf("/Sat/C")
			if err != nil {
				t.Fatal(err)
			}
			packet, err := layout.Extract(test.bytes)
			if err != nil {
				t.Fatal(err)
			}
			got, ok := packet.Values[0].Float()
			if !ok {
				t.Fatalf("not a float: %v", packet.Values[0].Err)
			}
			if got != test.want {
				t.Errorf("= %v, want %v", got, test.want)
			}
		})
	}
}
