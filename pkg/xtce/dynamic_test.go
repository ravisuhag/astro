package xtce_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/ravisuhag/astro/pkg/xtce"
)

// dynamicDB wraps a container set in a database with the types the dynamic
// tests need.
func dynamicDB(t *testing.T, containers string) *xtce.SpaceSystem {
	t.Helper()

	return parse(t, wrap("Sat", `
  <ParameterTypeSet>
    <IntegerParameterType name="U8" signed="false">
      <IntegerDataEncoding sizeInBits="8" encoding="unsigned"/>
    </IntegerParameterType>
    <IntegerParameterType name="U16" signed="false">
      <IntegerDataEncoding sizeInBits="16" encoding="unsigned"/>
    </IntegerParameterType>
    <!-- A binary field whose width another parameter states. -->
    <BinaryParameterType name="Blob">
      <BinaryDataEncoding>
        <SizeInBits>
          <DynamicValue>
            <ParameterInstanceRef parameterRef="Length"/>
          </DynamicValue>
        </SizeInBits>
      </BinaryDataEncoding>
    </BinaryParameterType>
    <!-- The same, but the length field counts octets rather than bits. -->
    <BinaryParameterType name="OctetBlob">
      <BinaryDataEncoding>
        <SizeInBits>
          <DynamicValue>
            <ParameterInstanceRef parameterRef="Length"/>
            <LinearAdjustment slope="8"/>
          </DynamicValue>
        </SizeInBits>
      </BinaryDataEncoding>
    </BinaryParameterType>
    <!-- A string ending at a null. -->
    <StringParameterType name="CString">
      <StringDataEncoding encoding="UTF-8">
        <SizeInBits>
          <TerminationChar>00</TerminationChar>
        </SizeInBits>
      </StringDataEncoding>
    </StringParameterType>
  </ParameterTypeSet>
  <ParameterSet>
    <Parameter name="Length" parameterTypeRef="U8"/>
    <Parameter name="Count" parameterTypeRef="U8"/>
    <Parameter name="Data" parameterTypeRef="Blob"/>
    <Parameter name="Octets" parameterTypeRef="OctetBlob"/>
    <Parameter name="Text" parameterTypeRef="CString"/>
    <Parameter name="Sample" parameterTypeRef="U8"/>
    <Parameter name="Trailer" parameterTypeRef="U16"/>
  </ParameterSet>
  <ContainerSet>`+containers+`</ContainerSet>`))
}

// A binary field sized by an earlier parameter. Layout cannot settle this;
// ResolveLayout reads the length out of the packet.
func TestResolveDynamicBinaryWidth(t *testing.T) {
	db := dynamicDB(t, `
    <SequenceContainer name="C">
      <EntryList>
        <ParameterRefEntry parameterRef="Length"/>
        <ParameterRefEntry parameterRef="Data"/>
      </EntryList>
    </SequenceContainer>`)

	container, err := db.FindContainer("/Sat/C")
	if err != nil {
		t.Fatalf("FindContainer: %v", err)
	}

	// The static path has to refuse it, or this test proves nothing.
	if _, err := container.Layout(); !errors.Is(err, xtce.ErrDynamicSize) {
		t.Fatalf("Layout() = %v, want ErrDynamicSize", err)
	}

	// Length 24 bits, then three octets of blob.
	packet := []byte{24, 0xAA, 0xBB, 0xCC}

	layout, err := container.ResolveLayout(packet)
	if err != nil {
		t.Fatalf("ResolveLayout: %v", err)
	}
	if len(layout.Fields) != 2 {
		t.Fatalf("got %d fields, want 2", len(layout.Fields))
	}
	if got := layout.Fields[1].BitSize; got != 24 {
		t.Errorf("the blob is %d bits, want the 24 the packet said", got)
	}
	if got := layout.Fields[1].BitOffset; got != 8 {
		t.Errorf("the blob starts at bit %d, want 8", got)
	}

	// And the same container reads a different shape from a different packet,
	// which is the whole point.
	shorter, err := container.ResolveLayout([]byte{8, 0xEE})
	if err != nil {
		t.Fatalf("ResolveLayout on the shorter packet: %v", err)
	}
	if got := shorter.Fields[1].BitSize; got != 8 {
		t.Errorf("the blob is %d bits in the second packet, want 8", got)
	}
}

// A LinearAdjustment scales the value, which is how a length field that
// counts octets sizes a field measured in bits.
func TestResolveLinearAdjustment(t *testing.T) {
	db := dynamicDB(t, `
    <SequenceContainer name="C">
      <EntryList>
        <ParameterRefEntry parameterRef="Length"/>
        <ParameterRefEntry parameterRef="Octets"/>
      </EntryList>
    </SequenceContainer>`)

	container, err := db.FindContainer("/Sat/C")
	if err != nil {
		t.Fatalf("FindContainer: %v", err)
	}

	// Length 3, meaning three octets, so 24 bits.
	layout, err := container.ResolveLayout([]byte{3, 0xAA, 0xBB, 0xCC})
	if err != nil {
		t.Fatalf("ResolveLayout: %v", err)
	}
	if got := layout.Fields[1].BitSize; got != 24 {
		t.Errorf("the blob is %d bits, want 3 octets scaled to 24", got)
	}
}

// A string that ends at a terminator. The terminator counts toward the field,
// because it takes up packet space.
func TestResolveTerminatedString(t *testing.T) {
	db := dynamicDB(t, `
    <SequenceContainer name="C">
      <EntryList>
        <ParameterRefEntry parameterRef="Text"/>
        <ParameterRefEntry parameterRef="Trailer"/>
      </EntryList>
    </SequenceContainer>`)

	container, err := db.FindContainer("/Sat/C")
	if err != nil {
		t.Fatalf("FindContainer: %v", err)
	}

	// "OK" then a null, then a two-octet trailer.
	packet := []byte{'O', 'K', 0x00, 0xDE, 0xAD}

	layout, err := container.ResolveLayout(packet)
	if err != nil {
		t.Fatalf("ResolveLayout: %v", err)
	}
	// Three octets: the two characters and the terminator.
	if got := layout.Fields[0].BitSize; got != 24 {
		t.Errorf("the string is %d bits, want 24 including the terminator", got)
	}
	// So the trailer starts after it, which is what proves the terminator was
	// counted rather than skipped.
	if got := layout.Fields[1].BitOffset; got != 24 {
		t.Errorf("the trailer starts at bit %d, want 24", got)
	}

	// A longer string moves the trailer.
	longer, err := container.ResolveLayout([]byte{'H', 'E', 'L', 'L', 'O', 0x00, 0xBE, 0xEF})
	if err != nil {
		t.Fatalf("ResolveLayout on the longer packet: %v", err)
	}
	if got := longer.Fields[1].BitOffset; got != 48 {
		t.Errorf("the trailer starts at bit %d, want 48", got)
	}
}

// A terminator that never appears means the packet is truncated, which is
// worth saying rather than reading to the end and calling it a string.
func TestResolveTerminatedStringWithoutTerminator(t *testing.T) {
	db := dynamicDB(t, `
    <SequenceContainer name="C">
      <EntryList><ParameterRefEntry parameterRef="Text"/></EntryList>
    </SequenceContainer>`)

	container, err := db.FindContainer("/Sat/C")
	if err != nil {
		t.Fatalf("FindContainer: %v", err)
	}

	if _, err := container.ResolveLayout([]byte{'N', 'O', 'P', 'E'}); !errors.Is(err, xtce.ErrPacketTooShort) {
		t.Errorf("ResolveLayout = %v, want ErrPacketTooShort", err)
	}
}

// A repeat count read from the packet, which is how an array of samples says
// how many it has.
func TestResolveDynamicRepeatCount(t *testing.T) {
	db := dynamicDB(t, `
    <SequenceContainer name="C">
      <EntryList>
        <ParameterRefEntry parameterRef="Count"/>
        <ParameterRefEntry parameterRef="Sample">
          <RepeatEntry>
            <Count>
              <DynamicValue>
                <ParameterInstanceRef parameterRef="Count"/>
              </DynamicValue>
            </Count>
          </RepeatEntry>
        </ParameterRefEntry>
      </EntryList>
    </SequenceContainer>`)

	container, err := db.FindContainer("/Sat/C")
	if err != nil {
		t.Fatalf("FindContainer: %v", err)
	}

	if _, err := container.Layout(); !errors.Is(err, xtce.ErrDynamicSize) {
		t.Fatalf("Layout() = %v, want ErrDynamicSize", err)
	}

	for _, count := range []int{0, 1, 4} {
		packet := append([]byte{byte(count)}, make([]byte, count)...)

		layout, err := container.ResolveLayout(packet)
		if err != nil {
			t.Fatalf("ResolveLayout with count %d: %v", count, err)
		}
		// The count field plus one field per sample.
		if want := 1 + count; len(layout.Fields) != want {
			t.Errorf("count %d gave %d fields, want %d", count, len(layout.Fields), want)
		}
	}
}

// An entry positioned by a value read from the packet.
func TestResolveDynamicLocation(t *testing.T) {
	db := dynamicDB(t, `
    <SequenceContainer name="C">
      <EntryList>
        <ParameterRefEntry parameterRef="Length"/>
        <ParameterRefEntry parameterRef="Sample">
          <LocationInContainerInBits referenceLocation="containerStart">
            <DynamicValue>
              <ParameterInstanceRef parameterRef="Length"/>
            </DynamicValue>
          </LocationInContainerInBits>
        </ParameterRefEntry>
      </EntryList>
    </SequenceContainer>`)

	container, err := db.FindContainer("/Sat/C")
	if err != nil {
		t.Fatalf("FindContainer: %v", err)
	}

	// The first octet says the sample sits at bit 32.
	layout, err := container.ResolveLayout([]byte{32, 0, 0, 0, 0x7F})
	if err != nil {
		t.Fatalf("ResolveLayout: %v", err)
	}
	if got := layout.Fields[1].BitOffset; got != 32 {
		t.Errorf("the sample is at bit %d, want the 32 the packet said", got)
	}
}

// An entry positioned relative to the end of the container, which with a
// packet in hand is the end of the packet.
func TestResolveContainerEnd(t *testing.T) {
	db := dynamicDB(t, `
    <SequenceContainer name="C">
      <EntryList>
        <ParameterRefEntry parameterRef="Length"/>
        <ParameterRefEntry parameterRef="Trailer">
          <LocationInContainerInBits referenceLocation="containerEnd">
            <FixedValue>-16</FixedValue>
          </LocationInContainerInBits>
        </ParameterRefEntry>
      </EntryList>
    </SequenceContainer>`)

	container, err := db.FindContainer("/Sat/C")
	if err != nil {
		t.Fatalf("FindContainer: %v", err)
	}

	// The static path refuses this: the container's end is a forward
	// reference it cannot resolve.
	if _, err := container.Layout(); !errors.Is(err, xtce.ErrUnsupportedEntry) {
		t.Fatalf("Layout() = %v, want ErrUnsupportedEntry", err)
	}

	// Eight octets, so the trailer sits at bit 48: sixteen bits before the end.
	layout, err := container.ResolveLayout(make([]byte, 8))
	if err != nil {
		t.Fatalf("ResolveLayout: %v", err)
	}
	if got := layout.Fields[1].BitOffset; got != 48 {
		t.Errorf("the trailer is at bit %d, want 48 in an eight-octet packet", got)
	}

	// A longer packet moves it, which is what containerEnd means.
	longer, err := container.ResolveLayout(make([]byte, 12))
	if err != nil {
		t.Fatalf("ResolveLayout on the longer packet: %v", err)
	}
	if got := longer.Fields[1].BitOffset; got != 80 {
		t.Errorf("the trailer is at bit %d, want 80 in a twelve-octet packet", got)
	}
}

// A forward reference cannot be resolved in one pass, and reading a field
// before it has arrived would be worse than failing.
func TestResolveRefusesForwardReference(t *testing.T) {
	db := dynamicDB(t, `
    <SequenceContainer name="C">
      <EntryList>
        <ParameterRefEntry parameterRef="Data"/>
        <ParameterRefEntry parameterRef="Length"/>
      </EntryList>
    </SequenceContainer>`)

	container, err := db.FindContainer("/Sat/C")
	if err != nil {
		t.Fatalf("FindContainer: %v", err)
	}

	_, err = container.ResolveLayout([]byte{8, 0xAA})
	if !errors.Is(err, xtce.ErrDynamicSize) {
		t.Fatalf("ResolveLayout = %v, want ErrDynamicSize", err)
	}
	if !strings.Contains(err.Error(), "not decoded before this point") {
		t.Errorf("the error does not explain the forward reference: %v", err)
	}
}

// Extracting through the dynamic path gives the values, and the blob is the
// width the length field stated.
func TestExtractDynamic(t *testing.T) {
	db := dynamicDB(t, `
    <SequenceContainer name="C">
      <EntryList>
        <ParameterRefEntry parameterRef="Length"/>
        <ParameterRefEntry parameterRef="Data"/>
      </EntryList>
    </SequenceContainer>`)

	container, err := db.FindContainer("/Sat/C")
	if err != nil {
		t.Fatalf("FindContainer: %v", err)
	}

	packet, err := container.ExtractDynamic([]byte{16, 0xAA, 0xBB})
	if err != nil {
		t.Fatalf("ExtractDynamic: %v", err)
	}

	value, ok := packet.Get("/Sat/Data")
	if !ok {
		t.Fatal("the blob is not in the extracted packet")
	}
	if value.Err != nil {
		t.Fatalf("the blob did not decode: %v", value.Err)
	}

	raw, ok := value.Raw.([]byte)
	if !ok {
		t.Fatalf("the blob decoded to a %T, want octets", value.Raw)
	}
	if len(raw) != 2 || raw[0] != 0xAA || raw[1] != 0xBB {
		t.Errorf("the blob is %x, want aabb", raw)
	}
}

// A container with no dynamic parts must resolve to the same thing Layout
// gives, so a caller that does not know which kind it has can always use the
// dynamic path.
func TestResolveMatchesStaticLayout(t *testing.T) {
	db := dynamicDB(t, `
    <SequenceContainer name="C">
      <EntryList>
        <ParameterRefEntry parameterRef="Length"/>
        <ParameterRefEntry parameterRef="Count"/>
        <ParameterRefEntry parameterRef="Trailer"/>
      </EntryList>
    </SequenceContainer>`)

	container, err := db.FindContainer("/Sat/C")
	if err != nil {
		t.Fatalf("FindContainer: %v", err)
	}

	static, err := container.Layout()
	if err != nil {
		t.Fatalf("Layout: %v", err)
	}
	dynamic, err := container.ResolveLayout([]byte{1, 2, 0, 0})
	if err != nil {
		t.Fatalf("ResolveLayout: %v", err)
	}

	if static.BitSize != dynamic.BitSize {
		t.Errorf("sizes differ: static %d bits, dynamic %d", static.BitSize, dynamic.BitSize)
	}
	if len(static.Fields) != len(dynamic.Fields) {
		t.Fatalf("field counts differ: static %d, dynamic %d",
			len(static.Fields), len(dynamic.Fields))
	}
	for i := range static.Fields {
		if static.Fields[i].Name != dynamic.Fields[i].Name {
			t.Errorf("field %d: static %q, dynamic %q",
				i, static.Fields[i].Name, dynamic.Fields[i].Name)
		}
		if static.Fields[i].BitOffset != dynamic.Fields[i].BitOffset {
			t.Errorf("field %d offset: static %d, dynamic %d",
				i, static.Fields[i].BitOffset, dynamic.Fields[i].BitOffset)
		}
		if static.Fields[i].BitSize != dynamic.Fields[i].BitSize {
			t.Errorf("field %d width: static %d, dynamic %d",
				i, static.Fields[i].BitSize, dynamic.Fields[i].BitSize)
		}
	}
}

// Inheritance still works on the dynamic path: the base's entries come
// first, and a derived container's dynamic field can be sized by one of them.
func TestResolveAcrossInheritance(t *testing.T) {
	db := dynamicDB(t, `
    <SequenceContainer name="Base" abstract="true">
      <EntryList><ParameterRefEntry parameterRef="Length"/></EntryList>
    </SequenceContainer>
    <SequenceContainer name="Derived">
      <EntryList><ParameterRefEntry parameterRef="Data"/></EntryList>
      <BaseContainer containerRef="Base"/>
    </SequenceContainer>`)

	container, err := db.FindContainer("/Sat/Derived")
	if err != nil {
		t.Fatalf("FindContainer: %v", err)
	}

	layout, err := container.ResolveLayout([]byte{16, 0xAA, 0xBB})
	if err != nil {
		t.Fatalf("ResolveLayout: %v", err)
	}
	if len(layout.Fields) != 2 {
		t.Fatalf("got %d fields, want the inherited one plus the blob", len(layout.Fields))
	}
	if layout.Fields[0].Name != "/Sat/Length" {
		t.Errorf("the first field is %q, want the inherited /Sat/Length", layout.Fields[0].Name)
	}
	if got := layout.Fields[1].BitSize; got != 16 {
		t.Errorf("the blob is %d bits, want the 16 the inherited field stated", got)
	}
}

// A negative width is a broken database, not a zero-width field.
func TestResolveRefusesNegativeWidth(t *testing.T) {
	db := parse(t, wrap("Sat", `
  <ParameterTypeSet>
    <IntegerParameterType name="S8" signed="true">
      <IntegerDataEncoding sizeInBits="8" encoding="twosComplement"/>
    </IntegerParameterType>
    <BinaryParameterType name="Blob">
      <BinaryDataEncoding>
        <SizeInBits>
          <DynamicValue><ParameterInstanceRef parameterRef="Length"/></DynamicValue>
        </SizeInBits>
      </BinaryDataEncoding>
    </BinaryParameterType>
  </ParameterTypeSet>
  <ParameterSet>
    <Parameter name="Length" parameterTypeRef="S8"/>
    <Parameter name="Data" parameterTypeRef="Blob"/>
  </ParameterSet>
  <ContainerSet>
    <SequenceContainer name="C">
      <EntryList>
        <ParameterRefEntry parameterRef="Length"/>
        <ParameterRefEntry parameterRef="Data"/>
      </EntryList>
    </SequenceContainer>
  </ContainerSet>`))

	container, err := db.FindContainer("/Sat/C")
	if err != nil {
		t.Fatalf("FindContainer: %v", err)
	}

	// 0xF8 read as two's complement is -8.
	if _, err := container.ResolveLayout([]byte{0xF8, 0}); !errors.Is(err, xtce.ErrDynamicSize) {
		t.Errorf("ResolveLayout = %v, want ErrDynamicSize for a negative width", err)
	}
}

// A LeadingSize string is refused by name rather than guessed at: the width
// of the size field itself is an attribute this package keeps raw, so there
// is no way to know how many octets to skip.
func TestResolveRefusesLeadingSizeString(t *testing.T) {
	db := parse(t, wrap("Sat", `
  <ParameterTypeSet>
    <StringParameterType name="PString">
      <StringDataEncoding>
        <SizeInBits><LeadingSize sizeInBitsOfSizeTag="8"/></SizeInBits>
      </StringDataEncoding>
    </StringParameterType>
  </ParameterTypeSet>
  <ParameterSet><Parameter name="Text" parameterTypeRef="PString"/></ParameterSet>
  <ContainerSet>
    <SequenceContainer name="C">
      <EntryList><ParameterRefEntry parameterRef="Text"/></EntryList>
    </SequenceContainer>
  </ContainerSet>`))

	container, err := db.FindContainer("/Sat/C")
	if err != nil {
		t.Fatalf("FindContainer: %v", err)
	}

	_, err = container.ResolveLayout([]byte{2, 'O', 'K'})
	if !errors.Is(err, xtce.ErrUnsupportedEntry) {
		t.Errorf("ResolveLayout = %v, want ErrUnsupportedEntry", err)
	}
}

// A dynamic reference to another packet cannot be answered by a walk that has
// only this one.
func TestResolveRefusesOtherInstances(t *testing.T) {
	db := parse(t, wrap("Sat", `
  <ParameterTypeSet>
    <IntegerParameterType name="U8" signed="false">
      <IntegerDataEncoding sizeInBits="8" encoding="unsigned"/>
    </IntegerParameterType>
    <BinaryParameterType name="Blob">
      <BinaryDataEncoding>
        <SizeInBits>
          <DynamicValue>
            <ParameterInstanceRef parameterRef="Length" instance="-1"/>
          </DynamicValue>
        </SizeInBits>
      </BinaryDataEncoding>
    </BinaryParameterType>
  </ParameterTypeSet>
  <ParameterSet>
    <Parameter name="Length" parameterTypeRef="U8"/>
    <Parameter name="Data" parameterTypeRef="Blob"/>
  </ParameterSet>
  <ContainerSet>
    <SequenceContainer name="C">
      <EntryList>
        <ParameterRefEntry parameterRef="Length"/>
        <ParameterRefEntry parameterRef="Data"/>
      </EntryList>
    </SequenceContainer>
  </ContainerSet>`))

	container, err := db.FindContainer("/Sat/C")
	if err != nil {
		t.Fatalf("FindContainer: %v", err)
	}

	if _, err := container.ResolveLayout([]byte{8, 0}); !errors.Is(err, xtce.ErrUnsupportedEntry) {
		t.Errorf("ResolveLayout = %v, want ErrUnsupportedEntry", err)
	}
}

// ResolveLayoutOf is the by-name entry point.
func TestResolveLayoutOf(t *testing.T) {
	db := dynamicDB(t, `
    <SequenceContainer name="C">
      <EntryList>
        <ParameterRefEntry parameterRef="Length"/>
        <ParameterRefEntry parameterRef="Data"/>
      </EntryList>
    </SequenceContainer>`)

	layout, err := db.ResolveLayoutOf("/Sat/C", []byte{16, 0xAA, 0xBB})
	if err != nil {
		t.Fatalf("ResolveLayoutOf: %v", err)
	}
	if got := layout.Fields[1].BitSize; got != 16 {
		t.Errorf("the blob is %d bits, want 16", got)
	}

	if _, err := db.ResolveLayoutOf("/Sat/Nope", nil); err == nil {
		t.Error("ResolveLayoutOf accepted an unknown container")
	}
}

// The LinearAdjustment defaults: an absent slope is one, not zero, because
// zero would throw the parameter away.
func TestLinearAdjustmentDefaults(t *testing.T) {
	var none *xtce.LinearAdjustment
	if got := none.Apply(7); got != 7 {
		t.Errorf("a nil adjustment gave %d, want the identity 7", got)
	}

	noSlope := &xtce.LinearAdjustment{Intercept: 3}
	if got := noSlope.Apply(7); got != 10 {
		t.Errorf("an absent slope gave %d, want 7 plus the intercept", got)
	}

	slope := 2.0
	both := &xtce.LinearAdjustment{Slope: &slope, Intercept: 1}
	if got := both.Apply(7); got != 15 {
		t.Errorf("slope 2 intercept 1 on 7 gave %d, want 15", got)
	}
}

// Match now works for a container whose shape its own contents decide. It
// used to stop at ErrDynamicSize, so such a packet could be neither
// identified nor searched past.
func TestMatchSelectsADynamicContainer(t *testing.T) {
	db := parse(t, wrap("Sat", `
  <ParameterTypeSet>
    <IntegerParameterType name="U8" signed="false">
      <IntegerDataEncoding sizeInBits="8" encoding="unsigned"/>
    </IntegerParameterType>
    <BinaryParameterType name="Blob">
      <BinaryDataEncoding>
        <SizeInBits>
          <DynamicValue>
            <ParameterInstanceRef parameterRef="Length"/>
            <LinearAdjustment slope="8"/>
          </DynamicValue>
        </SizeInBits>
      </BinaryDataEncoding>
    </BinaryParameterType>
  </ParameterTypeSet>
  <ParameterSet>
    <Parameter name="Type" parameterTypeRef="U8"/>
    <Parameter name="Length" parameterTypeRef="U8"/>
    <Parameter name="Payload" parameterTypeRef="Blob"/>
    <Parameter name="Fixed" parameterTypeRef="U8"/>
  </ParameterSet>
  <ContainerSet>
    <SequenceContainer name="Packet" abstract="true">
      <EntryList><ParameterRefEntry parameterRef="Type"/></EntryList>
    </SequenceContainer>
    <!-- Selected by Type 1, and its payload is sized by its own Length. -->
    <SequenceContainer name="Variable">
      <EntryList>
        <ParameterRefEntry parameterRef="Length"/>
        <ParameterRefEntry parameterRef="Payload"/>
      </EntryList>
      <BaseContainer containerRef="Packet">
        <RestrictionCriteria>
          <Comparison parameterRef="Type" value="1"/>
        </RestrictionCriteria>
      </BaseContainer>
    </SequenceContainer>
    <SequenceContainer name="Plain">
      <EntryList><ParameterRefEntry parameterRef="Fixed"/></EntryList>
      <BaseContainer containerRef="Packet">
        <RestrictionCriteria>
          <Comparison parameterRef="Type" value="2"/>
        </RestrictionCriteria>
      </BaseContainer>
    </SequenceContainer>
  </ContainerSet>`))

	root, err := db.FindContainer("/Sat/Packet")
	if err != nil {
		t.Fatalf("FindContainer: %v", err)
	}

	// Type 1, length 2 octets, then the payload.
	container, err := db.MatchFrom(root, []byte{1, 2, 0xAA, 0xBB})
	if err != nil {
		t.Fatalf("MatchFrom on the dynamic container: %v", err)
	}
	if container.Name != "Variable" {
		t.Errorf("matched %q, want Variable", container.Name)
	}

	// And the fixed sibling still matches, so the fallback did not break the
	// ordinary path.
	container, err = db.MatchFrom(root, []byte{2, 0x7F})
	if err != nil {
		t.Fatalf("MatchFrom on the fixed container: %v", err)
	}
	if container.Name != "Plain" {
		t.Errorf("matched %q, want Plain", container.Name)
	}

	// Matching and extracting together gives the payload at the width the
	// packet stated.
	packet, err := db.Match(root, []byte{1, 3, 0x11, 0x22, 0x33})
	if err != nil {
		t.Fatalf("Match: %v", err)
	}
	value, ok := packet.Get("/Sat/Payload")
	if !ok {
		t.Fatal("the payload is not in the extracted packet")
	}
	raw, ok := value.Raw.([]byte)
	if !ok {
		t.Fatalf("the payload decoded to a %T, want octets", value.Raw)
	}
	if len(raw) != 3 {
		t.Errorf("the payload is %d octets, want the 3 the length field stated", len(raw))
	}
}
