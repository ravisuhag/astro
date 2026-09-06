package xtce_test

import (
	"errors"
	"testing"

	"github.com/ravisuhag/astro/pkg/xtce"
)

// hugeCountDB is a container whose repeat count comes from a 32-bit packet
// field, wide enough to name a count in the billions -- the shape the CVE in
// plan 011 describes: a hostile packet naming a repeat count near 2^32.
func hugeCountDB(t *testing.T) *xtce.SpaceSystem {
	t.Helper()

	return parse(t, wrap("Sat", `
  <ParameterTypeSet>
    <IntegerParameterType name="U32" signed="false">
      <IntegerDataEncoding sizeInBits="32" encoding="unsigned"/>
    </IntegerParameterType>
    <IntegerParameterType name="U8" signed="false">
      <IntegerDataEncoding sizeInBits="8" encoding="unsigned"/>
    </IntegerParameterType>
  </ParameterTypeSet>
  <ParameterSet>
    <Parameter name="Count" parameterTypeRef="U32"/>
    <Parameter name="Sample" parameterTypeRef="U8"/>
  </ParameterSet>
  <ContainerSet>
    <SequenceContainer name="C">
      <EntryList>
        <ParameterRefEntry parameterRef="Count"/>
        <ParameterRefEntry parameterRef="Sample">
          <RepeatEntry>
            <Count><DynamicValue><ParameterInstanceRef parameterRef="Count"/></DynamicValue></Count>
          </RepeatEntry>
        </ParameterRefEntry>
      </EntryList>
    </SequenceContainer>
  </ContainerSet>`))
}

// be32 encodes n as four big-endian octets, the wire shape of the U32 fields
// hugeCountDB and hugeWidthDB use.
func be32(n uint32) []byte {
	return []byte{byte(n >> 24), byte(n >> 16), byte(n >> 8), byte(n)}
}

// A repeat count named by the packet as close to 2^32 must be refused
// immediately rather than looped over. Before plan 011's Step 1 this appended
// one Field per repetition with nothing checking the loop against the packet
// length: on this repository's hardware that is a hang and an unbounded
// allocation, not a slow success.
func TestResolveRefusesHugeRepeatCount(t *testing.T) {
	db := hugeCountDB(t)
	container, err := db.FindContainer("/Sat/C")
	if err != nil {
		t.Fatalf("FindContainer: %v", err)
	}

	packet := be32(4294967295) // 2^32 - 1
	if _, err := container.ResolveLayout(packet); !errors.Is(err, xtce.ErrUnsupportedEntry) {
		t.Fatalf("ResolveLayout = %v, want ErrUnsupportedEntry", err)
	}
}

// A count past MaxRepeatCount is refused even when the packet has plenty of
// bits left for it, which is what proves this is the absolute ceiling and not
// just the remaining-bits check below.
func TestResolveRefusesRepeatCountAboveCeiling(t *testing.T) {
	db := hugeCountDB(t)
	container, err := db.FindContainer("/Sat/C")
	if err != nil {
		t.Fatalf("FindContainer: %v", err)
	}

	count := uint32(xtce.MaxRepeatCount + 1)
	packet := append(be32(count), make([]byte, count)...) // ample remaining bits
	if _, err := container.ResolveLayout(packet); !errors.Is(err, xtce.ErrUnsupportedEntry) {
		t.Fatalf("ResolveLayout = %v, want ErrUnsupportedEntry", err)
	}
}

// A count under MaxRepeatCount is still refused when the packet plainly does
// not have that many bits left, which is the cheap physical check: a
// repetition takes at least one bit.
func TestResolveRefusesRepeatCountPastPacketEnd(t *testing.T) {
	db := hugeCountDB(t)
	container, err := db.FindContainer("/Sat/C")
	if err != nil {
		t.Fatalf("FindContainer: %v", err)
	}

	packet := append(be32(1000), byte(0)) // one octet left, nowhere near 1000 bits
	if _, err := container.ResolveLayout(packet); !errors.Is(err, xtce.ErrUnsupportedEntry) {
		t.Fatalf("ResolveLayout = %v, want ErrUnsupportedEntry", err)
	}
}

// The same container with an ordinary, small repeat count still extracts
// correctly. The caps above must not make a normal packet fail.
func TestResolveOrdinaryRepeatCountStillWorks(t *testing.T) {
	db := hugeCountDB(t)
	container, err := db.FindContainer("/Sat/C")
	if err != nil {
		t.Fatalf("FindContainer: %v", err)
	}

	const count = 5
	const want = 1 + count // the count field plus one field per sample
	packet := append(be32(count), make([]byte, count)...)
	layout, err := container.ResolveLayout(packet)
	if err != nil {
		t.Fatalf("ResolveLayout: %v", err)
	}
	if len(layout.Fields) != want {
		t.Fatalf("got %d fields, want %d", len(layout.Fields), want)
	}

	packet2, err := container.ExtractDynamic(packet)
	if err != nil {
		t.Fatalf("ExtractDynamic: %v", err)
	}
	if got := len(packet2.Values); got != want {
		t.Errorf("got %d values, want %d", got, want)
	}
}

// A FixedValue repeat count from the database itself is bound by the same
// ceiling on the static path, where a hostile or corrupt database supplies it
// directly rather than by way of a packet.
func TestLayoutRefusesHugeStaticRepeatCount(t *testing.T) {
	db := parse(t, wrap("Sat", `
  <ParameterTypeSet>
    <IntegerParameterType name="U8" signed="false">
      <IntegerDataEncoding sizeInBits="8" encoding="unsigned"/>
    </IntegerParameterType>
  </ParameterTypeSet>
  <ParameterSet><Parameter name="Sample" parameterTypeRef="U8"/></ParameterSet>
  <ContainerSet>
    <SequenceContainer name="C">
      <EntryList>
        <ParameterRefEntry parameterRef="Sample">
          <RepeatEntry><Count><FixedValue>4294967295</FixedValue></Count></RepeatEntry>
        </ParameterRefEntry>
      </EntryList>
    </SequenceContainer>
  </ContainerSet>`))

	container, err := db.FindContainer("/Sat/C")
	if err != nil {
		t.Fatalf("FindContainer: %v", err)
	}

	if _, err := container.Layout(); !errors.Is(err, xtce.ErrUnsupportedEntry) {
		t.Fatalf("Layout() = %v, want ErrUnsupportedEntry", err)
	}
}

// hugeWidthDB is a container whose binary field's width comes from a 32-bit
// packet field, so it can name a width in the billions of bits.
func hugeWidthDB(t *testing.T) *xtce.SpaceSystem {
	t.Helper()

	return parse(t, wrap("Sat", `
  <ParameterTypeSet>
    <IntegerParameterType name="U32" signed="false">
      <IntegerDataEncoding sizeInBits="32" encoding="unsigned"/>
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
    <Parameter name="Length" parameterTypeRef="U32"/>
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
}

// A dynamically-sized binary parameter whose width the packet names as close
// to 2^32 bits must be refused on a small packet rather than attempting to
// allocate that many octets.
func TestResolveRefusesHugeBinaryWidth(t *testing.T) {
	db := hugeWidthDB(t)
	container, err := db.FindContainer("/Sat/C")
	if err != nil {
		t.Fatalf("FindContainer: %v", err)
	}

	packet := append(be32(4294967295), 0, 0, 0, 0) // width near 2^32 bits, four spare octets
	if _, err := container.ResolveLayout(packet); !errors.Is(err, xtce.ErrPacketTooShort) {
		t.Fatalf("ResolveLayout = %v, want ErrPacketTooShort", err)
	}

	// Extracting through the combined path must likewise error, not panic.
	if _, err := container.ExtractDynamic(packet); err == nil {
		t.Fatal("ExtractDynamic accepted a binary width near 2^32 bits on an eight-octet packet")
	}
}

// An ordinary binary width from the same container still extracts correctly.
func TestResolveOrdinaryBinaryWidthStillWorks(t *testing.T) {
	db := hugeWidthDB(t)
	container, err := db.FindContainer("/Sat/C")
	if err != nil {
		t.Fatalf("FindContainer: %v", err)
	}

	packet := append(be32(16), 0xAA, 0xBB)
	layout, err := container.ResolveLayout(packet)
	if err != nil {
		t.Fatalf("ResolveLayout: %v", err)
	}
	if got := layout.Fields[1].BitSize; got != 16 {
		t.Errorf("the blob is %d bits, want 16", got)
	}
}
