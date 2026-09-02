package xtce_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/ravisuhag/astro/pkg/xtce"
)

// A mission that describes one packet header and three packet types that
// extend it, selected the way a real database does: by APID and by a service
// type within it.
const matchDB = `
  <ParameterTypeSet>
    <IntegerParameterType name="U8" signed="false">
      <IntegerDataEncoding sizeInBits="8" encoding="unsigned"/>
    </IntegerParameterType>
    <IntegerParameterType name="U16" signed="false">
      <IntegerDataEncoding sizeInBits="16" encoding="unsigned"/>
    </IntegerParameterType>
    <EnumeratedParameterType name="ModeType">
      <IntegerDataEncoding sizeInBits="8" encoding="unsigned"/>
      <EnumerationList>
        <Enumeration value="0" label="SAFE"/>
        <Enumeration value="1" label="SCIENCE"/>
      </EnumerationList>
    </EnumeratedParameterType>
  </ParameterTypeSet>
  <ParameterSet>
    <Parameter name="APID" parameterTypeRef="U8"/>
    <Parameter name="Service" parameterTypeRef="U8"/>
    <Parameter name="Mode" parameterTypeRef="ModeType"/>
    <Parameter name="Voltage" parameterTypeRef="U16"/>
    <Parameter name="Current" parameterTypeRef="U16"/>
    <Parameter name="Message" parameterTypeRef="U16"/>
  </ParameterSet>
  <ContainerSet>
    <SequenceContainer name="Packet" abstract="true">
      <EntryList>
        <ParameterRefEntry parameterRef="APID"/>
        <ParameterRefEntry parameterRef="Service"/>
        <ParameterRefEntry parameterRef="Mode"/>
      </EntryList>
    </SequenceContainer>

    <SequenceContainer name="Housekeeping">
      <EntryList><ParameterRefEntry parameterRef="Voltage"/></EntryList>
      <BaseContainer containerRef="Packet">
        <RestrictionCriteria>
          <Comparison parameterRef="APID" value="10"/>
        </RestrictionCriteria>
      </BaseContainer>
    </SequenceContainer>

    <SequenceContainer name="DetailedHousekeeping">
      <EntryList><ParameterRefEntry parameterRef="Current"/></EntryList>
      <BaseContainer containerRef="Housekeeping">
        <RestrictionCriteria>
          <Comparison parameterRef="Service" value="3"/>
        </RestrictionCriteria>
      </BaseContainer>
    </SequenceContainer>

    <SequenceContainer name="Event">
      <EntryList><ParameterRefEntry parameterRef="Message"/></EntryList>
      <BaseContainer containerRef="Packet">
        <RestrictionCriteria>
          <ComparisonList>
            <Comparison parameterRef="APID" value="20"/>
            <Comparison parameterRef="Service" value="5"/>
          </ComparisonList>
        </RestrictionCriteria>
      </BaseContainer>
    </SequenceContainer>
  </ContainerSet>`

// root returns the abstract Packet container every test matches from.
func root(t *testing.T, db *xtce.SpaceSystem) *xtce.SequenceContainer {
	t.Helper()
	container, err := db.FindContainer("/Sat/Packet")
	if err != nil {
		t.Fatalf("FindContainer() = %v", err)
	}
	return container
}

func TestMatchSelectsByComparison(t *testing.T) {
	db := parse(t, wrap("Sat", matchDB))
	base := root(t, db)

	tests := []struct {
		name   string
		packet []byte
		want   string
	}{
		// APID 10, service 1, mode SAFE, then a 16-bit voltage.
		{"housekeeping", []byte{10, 1, 0, 0x12, 0x34}, "Housekeeping"},
		// APID 10 and service 3 goes one level deeper.
		{"detailed housekeeping", []byte{10, 3, 0, 0x12, 0x34, 0x56, 0x78}, "DetailedHousekeeping"},
		// APID 20 and service 5, which needs both comparisons to hold.
		{"event", []byte{20, 5, 1, 0xAB, 0xCD}, "Event"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := db.MatchFrom(base, test.packet)
			if err != nil {
				t.Fatalf("MatchFrom() = %v", err)
			}
			if got.Name != test.want {
				t.Errorf("matched %q, want %q", got.Name, test.want)
			}
		})
	}
}

// TestMatchRequiresEveryComparison is what makes a ComparisonList mean
// anything: APID 20 alone is not an Event.
func TestMatchRequiresEveryComparison(t *testing.T) {
	db := parse(t, wrap("Sat", matchDB))
	base := root(t, db)

	// APID 20 but service 9, so the list does not hold.
	if _, err := db.MatchFrom(base, []byte{20, 9, 0, 0, 0}); !errors.Is(err, xtce.ErrNoMatch) {
		t.Errorf("MatchFrom(APID 20, service 9) = %v, want ErrNoMatch", err)
	}
}

func TestMatchReportsNoMatch(t *testing.T) {
	db := parse(t, wrap("Sat", matchDB))
	base := root(t, db)

	if _, err := db.MatchFrom(base, []byte{99, 0, 0, 0, 0}); !errors.Is(err, xtce.ErrNoMatch) {
		t.Errorf("MatchFrom(unknown APID) = %v, want ErrNoMatch", err)
	}
}

// TestMatchExtracts is the whole engine end to end: octets in, named values
// out, with no caller having to say what the packet was.
func TestMatchExtracts(t *testing.T) {
	db := parse(t, wrap("Sat", matchDB))
	base := root(t, db)

	packet, err := db.Match(base, []byte{10, 3, 1, 0x12, 0x34, 0x56, 0x78})
	if err != nil {
		t.Fatalf("Match() = %v", err)
	}
	if err := packet.Err(); err != nil {
		t.Fatalf("a field failed: %v", err)
	}

	if packet.Layout.Container.Name != "DetailedHousekeeping" {
		t.Errorf("extracted against %q", packet.Layout.Container.Name)
	}

	// The inherited header fields come first, then Housekeeping's Voltage,
	// then this container's own Current.
	want := []struct {
		name string
		eng  any
	}{
		{"/Sat/APID", uint64(10)},
		{"/Sat/Service", uint64(3)},
		{"/Sat/Mode", "SCIENCE"},
		{"/Sat/Voltage", uint64(0x1234)},
		{"/Sat/Current", uint64(0x5678)},
	}

	if len(packet.Values) != len(want) {
		t.Fatalf("got %d values, want %d:\n%s", len(packet.Values), len(want), packet)
	}
	for i, expected := range want {
		got := packet.Values[i]
		if got.Name() != expected.name || got.Engineering != expected.eng {
			t.Errorf("value %d = %s, want %s = %v", i, got, expected.name, expected.eng)
		}
	}
}

// TestMatchIgnoresAbstractContainers checks that an abstract container is
// never the answer, even when nothing derived from it matches.
func TestMatchIgnoresAbstractContainers(t *testing.T) {
	db := parse(t, wrap("Sat", matchDB))
	base := root(t, db)

	got, err := db.MatchFrom(base, []byte{99, 0, 0, 0, 0})
	if err == nil {
		t.Fatalf("MatchFrom returned the abstract container %q", got.Name)
	}
}

// TestMatchOnCalibratedValue checks the schema's default of
// useCalibratedValue="true": a comparison against an enumerated parameter is
// against its label.
func TestMatchOnCalibratedValue(t *testing.T) {
	db := parse(t, wrap("Sat", `
  <ParameterTypeSet>
    <EnumeratedParameterType name="ModeType">
      <IntegerDataEncoding sizeInBits="8" encoding="unsigned"/>
      <EnumerationList>
        <Enumeration value="0" label="SAFE"/>
        <Enumeration value="1" label="SCIENCE"/>
      </EnumerationList>
    </EnumeratedParameterType>
    <IntegerParameterType name="U8" signed="false">
      <IntegerDataEncoding sizeInBits="8" encoding="unsigned"/>
    </IntegerParameterType>
  </ParameterTypeSet>
  <ParameterSet>
    <Parameter name="Mode" parameterTypeRef="ModeType"/>
    <Parameter name="Payload" parameterTypeRef="U8"/>
  </ParameterSet>
  <ContainerSet>
    <SequenceContainer name="Base" abstract="true">
      <EntryList><ParameterRefEntry parameterRef="Mode"/></EntryList>
    </SequenceContainer>
    <SequenceContainer name="SciencePacket">
      <EntryList><ParameterRefEntry parameterRef="Payload"/></EntryList>
      <BaseContainer containerRef="Base">
        <RestrictionCriteria>
          <Comparison parameterRef="Mode" value="SCIENCE"/>
        </RestrictionCriteria>
      </BaseContainer>
    </SequenceContainer>
  </ContainerSet>`))

	base, err := db.FindContainer("/Sat/Base")
	if err != nil {
		t.Fatal(err)
	}

	got, err := db.MatchFrom(base, []byte{1, 0xFF})
	if err != nil {
		t.Fatalf("MatchFrom(mode 1) = %v, want SciencePacket by its label", err)
	}
	if got.Name != "SciencePacket" {
		t.Errorf("matched %q", got.Name)
	}

	if _, err := db.MatchFrom(base, []byte{0, 0xFF}); !errors.Is(err, xtce.ErrNoMatch) {
		t.Errorf("mode 0 matched SciencePacket")
	}
}

// TestMatchOnRawValue checks useCalibratedValue="false", where the same
// enumerated parameter is compared as a number instead.
func TestMatchOnRawValue(t *testing.T) {
	db := parse(t, wrap("Sat", `
  <ParameterTypeSet>
    <EnumeratedParameterType name="ModeType">
      <IntegerDataEncoding sizeInBits="8" encoding="unsigned"/>
      <EnumerationList><Enumeration value="1" label="SCIENCE"/></EnumerationList>
    </EnumeratedParameterType>
    <IntegerParameterType name="U8" signed="false">
      <IntegerDataEncoding sizeInBits="8" encoding="unsigned"/>
    </IntegerParameterType>
  </ParameterTypeSet>
  <ParameterSet>
    <Parameter name="Mode" parameterTypeRef="ModeType"/>
    <Parameter name="Payload" parameterTypeRef="U8"/>
  </ParameterSet>
  <ContainerSet>
    <SequenceContainer name="Base" abstract="true">
      <EntryList><ParameterRefEntry parameterRef="Mode"/></EntryList>
    </SequenceContainer>
    <SequenceContainer name="SciencePacket">
      <EntryList><ParameterRefEntry parameterRef="Payload"/></EntryList>
      <BaseContainer containerRef="Base">
        <RestrictionCriteria>
          <Comparison parameterRef="Mode" value="1" useCalibratedValue="false"/>
        </RestrictionCriteria>
      </BaseContainer>
    </SequenceContainer>
  </ContainerSet>`))

	base, _ := db.FindContainer("/Sat/Base")
	if _, err := db.MatchFrom(base, []byte{1, 0}); err != nil {
		t.Errorf("MatchFrom against the raw value = %v", err)
	}
}

// TestComparisonRadixPrefixes pins the schema's note that a value is base ten
// unless it starts with 0x, 0o or 0b.
func TestComparisonRadixPrefixes(t *testing.T) {
	tests := []struct {
		value string
		octet byte
	}{
		{"255", 255},
		{"0xFF", 255},
		{"0o377", 255},
		{"0b11111111", 255},
		{"0x2A", 42},
	}

	for _, test := range tests {
		t.Run(test.value, func(t *testing.T) {
			db := parse(t, wrap("Sat", `
  <ParameterTypeSet>
    <IntegerParameterType name="U8" signed="false">
      <IntegerDataEncoding sizeInBits="8" encoding="unsigned"/>
    </IntegerParameterType>
  </ParameterTypeSet>
  <ParameterSet>
    <Parameter name="ID" parameterTypeRef="U8"/>
    <Parameter name="Payload" parameterTypeRef="U8"/>
  </ParameterSet>
  <ContainerSet>
    <SequenceContainer name="Base" abstract="true">
      <EntryList><ParameterRefEntry parameterRef="ID"/></EntryList>
    </SequenceContainer>
    <SequenceContainer name="Derived">
      <EntryList><ParameterRefEntry parameterRef="Payload"/></EntryList>
      <BaseContainer containerRef="Base">
        <RestrictionCriteria>
          <Comparison parameterRef="ID" value="`+test.value+`"/>
        </RestrictionCriteria>
      </BaseContainer>
    </SequenceContainer>
  </ContainerSet>`))

			base, _ := db.FindContainer("/Sat/Base")
			if _, err := db.MatchFrom(base, []byte{test.octet, 0}); err != nil {
				t.Errorf("value %q did not match octet %d: %v", test.value, test.octet, err)
			}
		})
	}
}

// TestComparisonOperators walks the six operators the schema lists.
func TestComparisonOperators(t *testing.T) {
	tests := []struct {
		operator string
		id       byte
		match    bool
	}{
		{"==", 10, true}, {"==", 11, false},
		{"!=", 11, true}, {"!=", 10, false},
		{"&lt;", 9, true}, {"&lt;", 10, false},
		{"&lt;=", 10, true}, {"&lt;=", 11, false},
		{">", 11, true}, {">", 10, false},
		{">=", 10, true}, {">=", 9, false},
	}

	for _, test := range tests {
		t.Run(test.operator+"/"+itoa(int(test.id)), func(t *testing.T) {
			db := parse(t, wrap("Sat", `
  <ParameterTypeSet>
    <IntegerParameterType name="U8" signed="false">
      <IntegerDataEncoding sizeInBits="8" encoding="unsigned"/>
    </IntegerParameterType>
  </ParameterTypeSet>
  <ParameterSet>
    <Parameter name="ID" parameterTypeRef="U8"/>
    <Parameter name="Payload" parameterTypeRef="U8"/>
  </ParameterSet>
  <ContainerSet>
    <SequenceContainer name="Base" abstract="true">
      <EntryList><ParameterRefEntry parameterRef="ID"/></EntryList>
    </SequenceContainer>
    <SequenceContainer name="Derived">
      <EntryList><ParameterRefEntry parameterRef="Payload"/></EntryList>
      <BaseContainer containerRef="Base">
        <RestrictionCriteria>
          <Comparison parameterRef="ID" value="10" comparisonOperator="`+test.operator+`"/>
        </RestrictionCriteria>
      </BaseContainer>
    </SequenceContainer>
  </ContainerSet>`))

			base, _ := db.FindContainer("/Sat/Base")
			_, err := db.MatchFrom(base, []byte{test.id, 0})

			if matched := err == nil; matched != test.match {
				t.Errorf("id %d against %s 10: matched = %v, want %v (%v)",
					test.id, test.operator, matched, test.match, err)
			}
		})
	}
}

// TestComparisonAgainstSignedField checks that a negative comparison value
// matches a negative decoded value, rather than being truncated into a large
// positive one.
func TestComparisonAgainstSignedField(t *testing.T) {
	db := parse(t, wrap("Sat", `
  <ParameterTypeSet>
    <IntegerParameterType name="S8">
      <IntegerDataEncoding sizeInBits="8" encoding="twosComplement"/>
    </IntegerParameterType>
    <IntegerParameterType name="U8" signed="false">
      <IntegerDataEncoding sizeInBits="8" encoding="unsigned"/>
    </IntegerParameterType>
  </ParameterTypeSet>
  <ParameterSet>
    <Parameter name="Offset" parameterTypeRef="S8"/>
    <Parameter name="Payload" parameterTypeRef="U8"/>
  </ParameterSet>
  <ContainerSet>
    <SequenceContainer name="Base" abstract="true">
      <EntryList><ParameterRefEntry parameterRef="Offset"/></EntryList>
    </SequenceContainer>
    <SequenceContainer name="Derived">
      <EntryList><ParameterRefEntry parameterRef="Payload"/></EntryList>
      <BaseContainer containerRef="Base">
        <RestrictionCriteria>
          <Comparison parameterRef="Offset" value="-1"/>
        </RestrictionCriteria>
      </BaseContainer>
    </SequenceContainer>
  </ContainerSet>`))

	base, _ := db.FindContainer("/Sat/Base")

	// 0xFF read as two's complement is -1.
	if _, err := db.MatchFrom(base, []byte{0xFF, 0}); err != nil {
		t.Errorf("value \"-1\" did not match a field holding -1: %v", err)
	}
	if _, err := db.MatchFrom(base, []byte{0x01, 0}); !errors.Is(err, xtce.ErrNoMatch) {
		t.Errorf("value \"-1\" matched a field holding 1")
	}
}

// TestMatchRefusesUnsupportedCriteria checks that a CustomAlgorithm is
// reported rather than quietly treated as false, which would silently
// misroute packets. It is the one criteria form that stays out of reach: the
// algorithm is by definition not in the file.
func TestMatchRefusesUnsupportedCriteria(t *testing.T) {
	db := parse(t, wrap("Sat", `
  <ParameterTypeSet>
    <IntegerParameterType name="U8" signed="false">
      <IntegerDataEncoding sizeInBits="8" encoding="unsigned"/>
    </IntegerParameterType>
  </ParameterTypeSet>
  <ParameterSet>
    <Parameter name="ID" parameterTypeRef="U8"/>
    <Parameter name="Payload" parameterTypeRef="U8"/>
  </ParameterSet>
  <ContainerSet>
    <SequenceContainer name="Base" abstract="true">
      <EntryList><ParameterRefEntry parameterRef="ID"/></EntryList>
    </SequenceContainer>
    <SequenceContainer name="Derived">
      <EntryList><ParameterRefEntry parameterRef="Payload"/></EntryList>
      <BaseContainer containerRef="Base">
        <RestrictionCriteria>
          <CustomAlgorithm name="PickIt"/>
        </RestrictionCriteria>
      </BaseContainer>
    </SequenceContainer>
  </ContainerSet>`))

	base, _ := db.FindContainer("/Sat/Base")
	if _, err := db.MatchFrom(base, []byte{1, 0}); !errors.Is(err, xtce.ErrUnsupportedCriteria) {
		t.Errorf("MatchFrom(CustomAlgorithm) = %v, want ErrUnsupportedCriteria", err)
	}
}

// TestShortPacketIsNotAMatch checks that a packet too short to hold the field
// a criterion tests fails the match rather than erroring: a truncated packet
// is a normal thing to receive.
func TestShortPacketIsNotAMatch(t *testing.T) {
	db := parse(t, wrap("Sat", matchDB))
	base := root(t, db)

	// One octet, so Service at bit 8 is not there to compare.
	if _, err := db.MatchFrom(base, []byte{10}); !errors.Is(err, xtce.ErrNoMatch) {
		t.Errorf("MatchFrom(one octet) = %v, want ErrNoMatch", err)
	}
}

func TestMatchRejectsNilRoot(t *testing.T) {
	db := parse(t, wrap("Sat", matchDB))
	if _, err := db.Match(nil, []byte{1}); !errors.Is(err, xtce.ErrNotFound) {
		t.Errorf("Match(nil) = %v, want ErrNotFound", err)
	}
}

func FuzzMatchNeverPanics(f *testing.F) {
	f.Add([]byte{10, 1, 0, 0x12, 0x34})
	f.Add([]byte{20, 5, 1, 0, 0})
	f.Add([]byte{})

	db, err := xtce.Load(strings.NewReader(wrap("Sat", matchDB)))
	if err != nil {
		f.Fatal(err)
	}
	base, err := db.FindContainer("/Sat/Packet")
	if err != nil {
		f.Fatal(err)
	}

	f.Fuzz(func(t *testing.T, packet []byte) {
		if len(packet) > 4096 {
			return
		}
		// Any outcome is fine; a panic or a hang is not.
		if result, err := db.Match(base, packet); err == nil {
			_ = result.String()
		}
	})
}
