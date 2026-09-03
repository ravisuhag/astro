package xtce_test

import (
	"bytes"
	"errors"
	"fmt"
	"github.com/ravisuhag/astro/internal/vectors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ravisuhag/astro/pkg/xtce"
)

// load reads a fixture or fails the test.
func load(t *testing.T, name string) *xtce.SpaceSystem {
	t.Helper()
	db, err := xtce.LoadFile(filepath.Join(vectors.Root(), "xtce", name))
	if err != nil {
		t.Fatalf("LoadFile(%s) = %v", name, err)
	}
	return db
}

// TestNamespaceIsTheSchemaTargetNamespace pins the constant to the
// targetNamespace attribute of the XTCE 1.2 XSD. The date in the URI is the
// schema's publication, not a separate version.
func TestNamespaceIsTheSchemaTargetNamespace(t *testing.T) {
	const want = "http://www.omg.org/spec/XTCE/20180204"
	if xtce.Namespace != want {
		t.Errorf("Namespace = %q, want %q", xtce.Namespace, want)
	}
}

func TestLoadCCSDSHeader(t *testing.T) {
	db := load(t, "ccsds-header.xml")

	if db.Name != "CCSDS" {
		t.Errorf("root name = %q, want CCSDS", db.Name)
	}
	if db.Header == nil || db.Header.Version != "1.0" {
		t.Errorf("header = %+v, want version 1.0", db.Header)
	}

	params := db.Parameters()
	if len(params) != 7 {
		t.Fatalf("%d parameters, want 7", len(params))
	}

	// The primary header's fields, in wire order, with the widths of
	// CCSDS 133.0-B-2 as pkg/spp implements them.
	want := []struct {
		name string
		bits uint
	}{
		{"Version", 3},
		{"PacketType", 1},
		{"SecondaryHeaderFlag", 1},
		{"APID", 11},
		{"SequenceFlags", 2},
		{"SequenceCount", 14},
		{"PacketLength", 16},
	}

	total := uint(0)
	for i, expected := range want {
		if params[i].Name != expected.name {
			t.Errorf("parameter %d = %q, want %q", i, params[i].Name, expected.name)
			continue
		}
		paramType, err := db.ResolveParameterType(params[i].ParameterTypeRef)
		if err != nil {
			t.Errorf("%s: %v", expected.name, err)
			continue
		}
		bits, ok := paramType.Encoding().SizeInBits()
		if !ok {
			t.Errorf("%s: encoding has no fixed size", expected.name)
			continue
		}
		if bits != expected.bits {
			t.Errorf("%s is %d bits, want %d", expected.name, bits, expected.bits)
		}
		total += bits
	}

	// 48 bits is spp.PrimaryHeaderSize, six octets.
	if total != 48 {
		t.Errorf("the header adds up to %d bits, want 48", total)
	}
}

// TestEntryOrderIsPreserved is the test that justifies EntryList's hand-written
// unmarshaller. Entry order is packet order, and a decoder that grouped
// entries by element name would lose it.
func TestEntryOrderIsPreserved(t *testing.T) {
	db := load(t, "ccsds-header.xml")

	containers := db.Containers()
	if len(containers) != 1 {
		t.Fatalf("%d containers, want 1", len(containers))
	}
	container := containers[0]
	if !container.Abstract {
		t.Error("PrimaryHeader should be abstract")
	}

	want := []string{
		"Version", "PacketType", "SecondaryHeaderFlag",
		"APID", "SequenceFlags", "SequenceCount", "PacketLength",
	}
	if len(container.EntryList.Entries) != len(want) {
		t.Fatalf("%d entries, want %d", len(container.EntryList.Entries), len(want))
	}
	for i, expected := range want {
		entry := container.EntryList.Entries[i]
		if entry.Kind != xtce.EntryParameterRef {
			t.Errorf("entry %d is %v, want a parameter reference", i, entry.Kind)
		}
		if entry.Ref != expected {
			t.Errorf("entry %d references %q, want %q", i, entry.Ref, expected)
		}
	}
}

// TestMixedEntryKindsKeepTheirOrder checks that a container mixing entry kinds
// keeps them interleaved, which is the case a per-element-name decoder gets
// wrong.
func TestMixedEntryKindsKeepTheirOrder(t *testing.T) {
	db := load(t, "nested.xml")

	container, err := db.FindContainer("/Spacecraft/Power/PowerPacket")
	if err != nil {
		t.Fatalf("FindContainer() = %v", err)
	}

	entries := container.EntryList.Entries
	if len(entries) != 3 {
		t.Fatalf("%d entries, want 3", len(entries))
	}
	if entries[0].Kind != xtce.EntryContainerRef {
		t.Errorf("entry 0 is %v, want a container reference", entries[0].Kind)
	}
	if entries[1].Kind != xtce.EntryParameterRef || entries[1].Ref != "BusVoltage" {
		t.Errorf("entry 1 = %v %q, want a parameter reference to BusVoltage",
			entries[1].Kind, entries[1].Ref)
	}
	if entries[2].Ref != "BusCurrent" {
		t.Errorf("entry 2 references %q, want BusCurrent", entries[2].Ref)
	}
}

// TestNamespacePrefixAndDefaultAgree checks that the same content decodes the
// same whether the namespace is defaulted or prefixed.
func TestNamespacePrefixAndDefaultAgree(t *testing.T) {
	prefixed := load(t, "ccsds-header-prefixed.xml")

	if prefixed.Name != "CCSDS" {
		t.Errorf("root name = %q, want CCSDS", prefixed.Name)
	}
	params := prefixed.Parameters()
	if len(params) != 1 || params[0].Name != "Version" {
		t.Fatalf("parameters = %+v, want one called Version", params)
	}
	containers := prefixed.Containers()
	if len(containers) != 1 || len(containers[0].EntryList.Entries) != 1 {
		t.Fatalf("containers = %+v, want one with one entry", containers)
	}
	if containers[0].EntryList.Entries[0].Ref != "Version" {
		t.Errorf("entry references %q, want Version", containers[0].EntryList.Entries[0].Ref)
	}

	paramType, err := prefixed.ResolveParameterType("Version_t")
	if err != nil {
		t.Fatalf("ResolveParameterType() = %v", err)
	}
	if bits, ok := paramType.Encoding().SizeInBits(); !ok || bits != 3 {
		t.Errorf("Version_t is %d bits (known %v), want 3", bits, ok)
	}
}

func TestLoadNestedTree(t *testing.T) {
	db := load(t, "nested.xml")

	if len(db.SubSystems) != 2 {
		t.Fatalf("%d subsystems, want 2", len(db.SubSystems))
	}

	power := db.SubSystems[0]
	if power.QualifiedName() != "/Spacecraft/Power" {
		t.Errorf("qualified name = %q, want /Spacecraft/Power", power.QualifiedName())
	}
	if power.Parent() != db {
		t.Error("the parent link was not set during Load")
	}
	if power.Root() != db {
		t.Error("Root() did not reach the top")
	}

	visited := 0
	db.Walk(func(*xtce.SpaceSystem) bool {
		visited++
		return true
	})
	if visited != 3 {
		t.Errorf("Walk visited %d systems, want 3", visited)
	}
}

// TestWalkStops checks that returning false ends the walk.
func TestWalkStops(t *testing.T) {
	db := load(t, "nested.xml")

	visited := 0
	db.Walk(func(*xtce.SpaceSystem) bool {
		visited++
		return false
	})
	if visited != 1 {
		t.Errorf("Walk visited %d systems after being told to stop, want 1", visited)
	}
}

func TestLoadDecodesCalibrators(t *testing.T) {
	db := load(t, "nested.xml")

	voltsType, err := db.FindParameterType("/Spacecraft/Power/Volts_t")
	if err != nil {
		t.Fatalf("FindParameterType() = %v", err)
	}
	encoding := voltsType.Encoding()
	if encoding == nil || encoding.Integer == nil {
		t.Fatalf("Volts_t encoding = %+v, want an integer encoding", encoding)
	}
	// A float parameter carried as a 12-bit integer on the wire: the
	// calibrator is what makes it volts.
	if bits, _ := encoding.SizeInBits(); bits != 12 {
		t.Errorf("Volts_t is %d bits on the wire, want 12", bits)
	}

	calibrator := encoding.Integer.DefaultCalibrator
	if calibrator == nil || calibrator.Polynomial == nil {
		t.Fatalf("calibrator = %+v, want a polynomial", calibrator)
	}
	if calibrator.Kind() != "polynomial" {
		t.Errorf("Kind() = %q, want polynomial", calibrator.Kind())
	}
	if len(calibrator.Polynomial.Terms) != 2 {
		t.Fatalf("%d terms, want 2", len(calibrator.Polynomial.Terms))
	}
	if got := calibrator.Polynomial.Terms[1].Coefficient; got != 0.00732 {
		t.Errorf("first-order coefficient = %v, want 0.00732", got)
	}

	ampsType, err := db.FindParameterType("/Spacecraft/Power/Amps_t")
	if err != nil {
		t.Fatalf("FindParameterType() = %v", err)
	}
	spline := ampsType.Encoding().Integer.DefaultCalibrator.Spline
	if spline == nil {
		t.Fatal("Amps_t has no spline calibrator")
	}
	if len(spline.Points) != 3 {
		t.Fatalf("%d spline points, want 3", len(spline.Points))
	}
	if spline.Points[2].Calibrated != 9.5 {
		t.Errorf("last spline point calibrates to %v, want 9.5", spline.Points[2].Calibrated)
	}
}

// TestLoadDecodesAbsoluteTime covers the type whose encoding sits one level
// deeper than the others', inside an Encoding element with its own scaling.
func TestLoadDecodesAbsoluteTime(t *testing.T) {
	db := load(t, "nested.xml")

	timeType, err := db.FindParameterType("/Spacecraft/Thermal/Timestamp_t")
	if err != nil {
		t.Fatalf("FindParameterType() = %v", err)
	}
	if timeType.TypeKind() != "absolute time" {
		t.Errorf("TypeKind() = %q, want absolute time", timeType.TypeKind())
	}

	bits, ok := timeType.Encoding().SizeInBits()
	if !ok || bits != 32 {
		t.Errorf("timestamp is %d bits (known %v), want 32", bits, ok)
	}

	absolute, isAbsolute := timeType.(*xtce.AbsoluteTimeParameterType)
	if !isAbsolute {
		t.Fatalf("type is %T, want *AbsoluteTimeParameterType", timeType)
	}
	if absolute.ReferenceTime == nil || absolute.ReferenceTime.Epoch != "TAI" {
		t.Errorf("reference time = %+v, want the TAI epoch", absolute.ReferenceTime)
	}
	if got := absolute.Encoding_.ScaleOrDefault(); got != 1 {
		t.Errorf("scale = %v, want 1", got)
	}
	// The offset was absent, so the schema's default of 0 applies.
	if got := absolute.Encoding_.OffsetOrDefault(); got != 0 {
		t.Errorf("offset = %v, want 0", got)
	}
}

// TestSchemaDefaultsApply covers the attributes the XSD gives defaults to.
// encoding/xml does not know about XSD defaults, so each one needs an
// accessor and this test is what keeps them honest.
func TestSchemaDefaultsApply(t *testing.T) {
	const doc = `<SpaceSystem xmlns="http://www.omg.org/spec/XTCE/20180204" name="D">
	  <TelemetryMetaData>
	    <ParameterTypeSet>
	      <IntegerParameterType name="Bare_t">
	        <IntegerDataEncoding/>
	      </IntegerParameterType>
	      <FloatParameterType name="BareFloat_t">
	        <FloatDataEncoding/>
	      </FloatParameterType>
	      <StringParameterType name="BareString_t">
	        <StringDataEncoding/>
	      </StringParameterType>
	    </ParameterTypeSet>
	  </TelemetryMetaData>
	</SpaceSystem>`

	db, err := xtce.Load(strings.NewReader(doc))
	if err != nil {
		t.Fatalf("Load() = %v", err)
	}

	integerType, err := db.ResolveParameterType("Bare_t")
	if err != nil {
		t.Fatalf("ResolveParameterType() = %v", err)
	}
	integer, ok := integerType.(*xtce.IntegerParameterType)
	if !ok {
		t.Fatalf("type is %T", integerType)
	}
	// The XSD says signed defaults to true and IntegerDataEncoding's
	// sizeInBits to 8.
	if !integer.IsSigned() {
		t.Error("signed defaulted to false, want true")
	}
	if got := integer.IntegerDataEncoding.Size(); got != 8 {
		t.Errorf("integer encoding size = %d, want the default 8", got)
	}
	if got := integer.IntegerDataEncoding.EncodingOrDefault(); got != "unsigned" {
		t.Errorf("integer encoding = %q, want the default unsigned", got)
	}
	if got := integer.IntegerDataEncoding.BitOrderOrDefault(); got != "mostSignificantBitFirst" {
		t.Errorf("bit order = %q, want the default", got)
	}
	if got := integer.IntegerDataEncoding.ByteOrderOrDefault(); got != "mostSignificantByteFirst" {
		t.Errorf("byte order = %q, want the default", got)
	}

	floatType, _ := db.ResolveParameterType("BareFloat_t")
	floating := floatType.(*xtce.FloatParameterType)
	if got := floating.FloatDataEncoding.Size(); got != 32 {
		t.Errorf("float encoding size = %d, want the default 32", got)
	}
	if got := floating.FloatDataEncoding.EncodingOrDefault(); got != "IEEE754_1985" {
		t.Errorf("float encoding = %q, want the default", got)
	}

	stringType, _ := db.ResolveParameterType("BareString_t")
	text := stringType.(*xtce.StringParameterType)
	if got := text.StringDataEncoding.EncodingOrDefault(); got != "UTF-8" {
		t.Errorf("string encoding = %q, want the default UTF-8", got)
	}
	// A string with no fixed size reports so rather than guessing.
	if _, ok := stringType.Encoding().SizeInBits(); ok {
		t.Error("an unsized string encoding reported a known size")
	}
}

// TestSignedFalseIsDistinguishableFromAbsent is why Signed is a pointer.
func TestSignedFalseIsDistinguishableFromAbsent(t *testing.T) {
	const doc = `<SpaceSystem xmlns="http://www.omg.org/spec/XTCE/20180204" name="D">
	  <TelemetryMetaData>
	    <ParameterTypeSet>
	      <IntegerParameterType name="Unsigned_t" signed="false">
	        <IntegerDataEncoding sizeInBits="4"/>
	      </IntegerParameterType>
	    </ParameterTypeSet>
	  </TelemetryMetaData>
	</SpaceSystem>`

	db, err := xtce.Load(strings.NewReader(doc))
	if err != nil {
		t.Fatalf("Load() = %v", err)
	}
	resolved, _ := db.ResolveParameterType("Unsigned_t")
	if resolved.(*xtce.IntegerParameterType).IsSigned() {
		t.Error("an explicit signed=false was read as signed")
	}
}

func TestLoadRejectsNonSpaceSystemRoot(t *testing.T) {
	if _, err := xtce.LoadFile(filepath.Join(vectors.Root(), "xtce", "invalid-root.xml")); !errors.Is(err, xtce.ErrNotSpaceSystem) {
		t.Fatalf("LoadFile() = %v, want ErrNotSpaceSystem", err)
	}
}

func TestLoadRejectsMalformedXML(t *testing.T) {
	_, err := xtce.Load(strings.NewReader(`<SpaceSystem xmlns="` + xtce.Namespace + `" name="X">`))
	if !errors.Is(err, xtce.ErrMalformedXML) && !errors.Is(err, xtce.ErrNotSpaceSystem) {
		t.Fatalf("Load() = %v, want a malformed-XML or not-a-SpaceSystem error", err)
	}
}

func TestLoadRejectsEmptyInput(t *testing.T) {
	if _, err := xtce.Load(bytes.NewReader(nil)); err == nil {
		t.Fatal("Load() accepted an empty document")
	}
}

func TestLoadRejectsNamelessRoot(t *testing.T) {
	_, err := xtce.Load(strings.NewReader(`<SpaceSystem xmlns="` + xtce.Namespace + `"/>`))
	if !errors.Is(err, xtce.ErrNotSpaceSystem) {
		t.Fatalf("Load() = %v, want ErrNotSpaceSystem", err)
	}
}

// TestLoadRejectsOversizedInput checks the size cap.
func TestLoadRejectsOversizedInput(t *testing.T) {
	doc := `<SpaceSystem xmlns="` + xtce.Namespace + `" name="Big"/>`

	if _, err := xtce.LoadWithLimit(strings.NewReader(doc), 10); !errors.Is(err, xtce.ErrInputTooLarge) {
		t.Fatalf("LoadWithLimit(10) = %v, want ErrInputTooLarge", err)
	}
	// The same document within a generous limit loads.
	if _, err := xtce.LoadWithLimit(strings.NewReader(doc), 1<<20); err != nil {
		t.Fatalf("LoadWithLimit(1 MiB) = %v", err)
	}
}

// TestLoadRejectsDeepNesting is the stack guard. Without the pre-scan, deeply
// nested SpaceSystems would recurse during decoding.
func TestLoadRejectsDeepNesting(t *testing.T) {
	var b strings.Builder
	const depth = 200

	b.WriteString(`<SpaceSystem xmlns="` + xtce.Namespace + `" name="S0">`)
	for i := 1; i < depth; i++ {
		fmt.Fprintf(&b, `<SpaceSystem name="S%d">`, i)
	}
	for range depth {
		b.WriteString(`</SpaceSystem>`)
	}

	if _, err := xtce.Load(strings.NewReader(b.String())); !errors.Is(err, xtce.ErrTooDeep) {
		t.Fatalf("Load() = %v, want ErrTooDeep", err)
	}
}

// TestModestNestingLoads checks the depth guard does not refuse ordinary
// files.
func TestModestNestingLoads(t *testing.T) {
	var b strings.Builder
	const depth = 10

	b.WriteString(`<SpaceSystem xmlns="` + xtce.Namespace + `" name="S0">`)
	for i := 1; i < depth; i++ {
		fmt.Fprintf(&b, `<SpaceSystem name="S%d">`, i)
	}
	for range depth {
		b.WriteString(`</SpaceSystem>`)
	}

	db, err := xtce.Load(strings.NewReader(b.String()))
	if err != nil {
		t.Fatalf("Load() = %v", err)
	}
	systems := 0
	db.Walk(func(*xtce.SpaceSystem) bool {
		systems++
		return true
	})
	if systems != depth {
		t.Errorf("loaded %d systems, want %d", systems, depth)
	}
}

func TestLoadFileMissing(t *testing.T) {
	if _, err := xtce.LoadFile(filepath.Join(vectors.Root(), "xtce", "does-not-exist.xml")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("LoadFile() = %v, want a not-exist error", err)
	}
}

// TestFixedIntegerValueForms covers the FixedIntegerValueType union: the
// schema allows decimal, hex, octal and binary spellings anywhere a fixed
// integer is written, and a loader that reads them with a base-ten parser
// rejects the whole document.
func TestFixedIntegerValueForms(t *testing.T) {
	const doc = `<SpaceSystem xmlns="http://www.omg.org/spec/XTCE/20180204" name="Radix">
	  <TelemetryMetaData>
	    <ParameterTypeSet>
	      <IntegerParameterType name="U8_t" signed="false">
	        <IntegerDataEncoding sizeInBits="8"/>
	      </IntegerParameterType>
	      <StringParameterType name="Tag_t">
	        <StringDataEncoding>
	          <SizeInBits><Fixed><FixedValue>0o40</FixedValue></Fixed></SizeInBits>
	        </StringDataEncoding>
	      </StringParameterType>
	      <BinaryParameterType name="Blob_t">
	        <BinaryDataEncoding>
	          <SizeInBits><FixedValue>0x20</FixedValue></SizeInBits>
	        </BinaryDataEncoding>
	      </BinaryParameterType>
	    </ParameterTypeSet>
	    <ParameterSet>
	      <Parameter name="Sample" parameterTypeRef="U8_t"/>
	      <Parameter name="Tag" parameterTypeRef="Tag_t"/>
	      <Parameter name="Blob" parameterTypeRef="Blob_t"/>
	    </ParameterSet>
	    <ContainerSet>
	      <SequenceContainer name="Packet">
	        <EntryList>
	          <ParameterRefEntry parameterRef="Sample">
	            <LocationInContainerInBits referenceLocation="containerStart">
	              <FixedValue>0x10</FixedValue>
	            </LocationInContainerInBits>
	          </ParameterRefEntry>
	          <ParameterRefEntry parameterRef="Sample">
	            <RepeatEntry>
	              <Count><FixedValue>0b11</FixedValue></Count>
	            </RepeatEntry>
	          </ParameterRefEntry>
	        </EntryList>
	      </SequenceContainer>
	    </ContainerSet>
	  </TelemetryMetaData>
	</SpaceSystem>`

	db, err := xtce.Load(strings.NewReader(doc))
	if err != nil {
		t.Fatalf("Load() = %v; hex, octal and binary FixedValues are legal FixedIntegerValueType forms", err)
	}

	// 0o40 = 32 bits of fixed string size.
	tagType, err := db.ResolveParameterType("Tag_t")
	if err != nil {
		t.Fatalf("ResolveParameterType(Tag_t) = %v", err)
	}
	if bits, ok := tagType.Encoding().SizeInBits(); !ok || bits != 32 {
		t.Errorf("Tag_t is %d bits (known %v), want 32 from 0o40", bits, ok)
	}

	// 0x20 = 32 bits of binary encoding size.
	blobType, err := db.ResolveParameterType("Blob_t")
	if err != nil {
		t.Fatalf("ResolveParameterType(Blob_t) = %v", err)
	}
	if bits, ok := blobType.Encoding().SizeInBits(); !ok || bits != 32 {
		t.Errorf("Blob_t is %d bits (known %v), want 32 from 0x20", bits, ok)
	}

	container, err := db.FindContainer("/Radix/Packet")
	if err != nil {
		t.Fatalf("FindContainer() = %v", err)
	}
	entries := container.EntryList.Entries

	// 0x10 = bit 16 from the container start.
	location := entries[0].LocationInContainerInBits
	if location == nil || location.FixedValue == nil || location.FixedValue.Int64() != 16 {
		t.Errorf("location = %+v, want a FixedValue of 16 from 0x10", location)
	}

	// 0b11 = a repeat count of 3.
	repeat := entries[1].RepeatEntry
	if repeat == nil || repeat.Count == nil || repeat.Count.FixedValue == nil ||
		repeat.Count.FixedValue.Int64() != 3 {
		t.Errorf("repeat = %+v, want a Count of 3 from 0b11", repeat)
	}

	// The layout must see the same numbers: Sample at bit 16, then three
	// repeats of eight bits each.
	layout, err := container.Layout()
	if err != nil {
		t.Fatalf("Layout() = %v", err)
	}
	if len(layout.Fields) != 4 {
		t.Fatalf("%d fields, want 4 (one placed, three repeated)", len(layout.Fields))
	}
	if layout.Fields[0].BitOffset != 16 {
		t.Errorf("first field at bit %d, want 16", layout.Fields[0].BitOffset)
	}
}

// TestBadValueIsNotMisreportedAsWrongRoot pins the error split: a document
// with a real SpaceSystem root and one unreadable value fails with
// ErrInvalidValue, not ErrNotSpaceSystem.
func TestBadValueIsNotMisreportedAsWrongRoot(t *testing.T) {
	const doc = `<SpaceSystem xmlns="http://www.omg.org/spec/XTCE/20180204" name="Bad">
	  <TelemetryMetaData>
	    <ContainerSet>
	      <SequenceContainer name="Packet">
	        <EntryList>
	          <ParameterRefEntry parameterRef="X">
	            <LocationInContainerInBits>
	              <FixedValue>banana</FixedValue>
	            </LocationInContainerInBits>
	          </ParameterRefEntry>
	        </EntryList>
	      </SequenceContainer>
	    </ContainerSet>
	  </TelemetryMetaData>
	</SpaceSystem>`

	_, err := xtce.Load(strings.NewReader(doc))
	if !errors.Is(err, xtce.ErrInvalidValue) {
		t.Fatalf("Load() = %v, want ErrInvalidValue", err)
	}
	if errors.Is(err, xtce.ErrNotSpaceSystem) {
		t.Fatalf("Load() = %v; a bad value must not be reported as a wrong root", err)
	}
}

// TestMetaCommandSetKeepsRefsAndBlocks covers the two MetaCommandSet members
// that are not plain MetaCommands. Both used to vanish silently.
func TestMetaCommandSetKeepsRefsAndBlocks(t *testing.T) {
	const doc = `<SpaceSystem xmlns="http://www.omg.org/spec/XTCE/20180204" name="Cmd">
	  <CommandMetaData>
	    <MetaCommandSet>
	      <MetaCommand name="Reset"/>
	      <MetaCommandRef>/Cmd/Sub/Reboot</MetaCommandRef>
	      <BlockMetaCommand name="SafeMode">
	        <MetaCommandStepList>
	          <MetaCommandStep metaCommandRef="Reset"/>
	        </MetaCommandStepList>
	      </BlockMetaCommand>
	    </MetaCommandSet>
	  </CommandMetaData>
	</SpaceSystem>`

	db, err := xtce.Load(strings.NewReader(doc))
	if err != nil {
		t.Fatalf("Load() = %v", err)
	}
	set := db.CommandMetaData.MetaCommandSet

	if len(set.MetaCommands) != 1 || set.MetaCommands[0].Name != "Reset" {
		t.Errorf("MetaCommands = %+v, want one called Reset", set.MetaCommands)
	}
	if len(set.MetaCommandRefs) != 1 || set.MetaCommandRefs[0].Ref != "/Cmd/Sub/Reboot" {
		t.Errorf("MetaCommandRefs = %+v, want one referencing /Cmd/Sub/Reboot", set.MetaCommandRefs)
	}
	if len(set.BlockMetaCommands) != 1 || set.BlockMetaCommands[0].Name != "SafeMode" {
		t.Fatalf("BlockMetaCommands = %+v, want one called SafeMode", set.BlockMetaCommands)
	}
	block := set.BlockMetaCommands[0]
	if block.MetaCommandStepList == nil || !strings.Contains(string(block.MetaCommandStepList.Inner), "Reset") {
		t.Errorf("step list = %+v, want the raw steps kept", block.MetaCommandStepList)
	}
}

// TestBaseMetaCommandKeepsArgumentAssignments covers the assignments that
// narrow a base command. Dropping them made a derived command look identical
// to its base.
func TestBaseMetaCommandKeepsArgumentAssignments(t *testing.T) {
	const doc = `<SpaceSystem xmlns="http://www.omg.org/spec/XTCE/20180204" name="Cmd">
	  <CommandMetaData>
	    <MetaCommandSet>
	      <MetaCommand name="SetMode" abstract="true"/>
	      <MetaCommand name="SetSafeMode">
	        <BaseMetaCommand metaCommandRef="SetMode">
	          <ArgumentAssignmentList>
	            <ArgumentAssignment argumentName="Mode" argumentValue="SAFE"/>
	            <ArgumentAssignment argumentName="Delay" argumentValue="0"/>
	          </ArgumentAssignmentList>
	        </BaseMetaCommand>
	      </MetaCommand>
	    </MetaCommandSet>
	  </CommandMetaData>
	</SpaceSystem>`

	db, err := xtce.Load(strings.NewReader(doc))
	if err != nil {
		t.Fatalf("Load() = %v", err)
	}

	derived := db.MetaCommands()[1]
	base := derived.BaseMetaCommand
	if base == nil || base.MetaCommandRef != "SetMode" {
		t.Fatalf("BaseMetaCommand = %+v, want a reference to SetMode", base)
	}
	if base.ArgumentAssignmentList == nil {
		t.Fatal("the ArgumentAssignmentList was dropped")
	}
	assignments := base.ArgumentAssignmentList.Assignments
	if len(assignments) != 2 {
		t.Fatalf("%d assignments, want 2", len(assignments))
	}
	if assignments[0].Name != "Mode" || assignments[0].Value != "SAFE" {
		t.Errorf("assignment 0 = %+v, want Mode=SAFE", assignments[0])
	}
	if assignments[1].Name != "Delay" || assignments[1].Value != "0" {
		t.Errorf("assignment 1 = %+v, want Delay=0", assignments[1])
	}
}

// TestOpaqueParameterTypesResolve covers the three type kinds kept as opaque
// entries: array, aggregate and relative time. Their names must resolve so a
// parameter using one passes Validate, and TypeKind must say what was found.
func TestOpaqueParameterTypesResolve(t *testing.T) {
	const doc = `<SpaceSystem xmlns="http://www.omg.org/spec/XTCE/20180204" name="Opaque">
	  <TelemetryMetaData>
	    <ParameterTypeSet>
	      <IntegerParameterType name="U8_t"><IntegerDataEncoding/></IntegerParameterType>
	      <ArrayParameterType name="Samples_t" arrayTypeRef="U8_t">
	        <DimensionList><Dimension><StartingIndex><FixedValue>0</FixedValue></StartingIndex>
	          <EndingIndex><FixedValue>7</FixedValue></EndingIndex></Dimension></DimensionList>
	      </ArrayParameterType>
	      <AggregateParameterType name="Position_t">
	        <MemberList><Member name="X" typeRef="U8_t"/></MemberList>
	      </AggregateParameterType>
	      <RelativeTimeParameterType name="Elapsed_t">
	        <Encoding units="seconds"><IntegerDataEncoding sizeInBits="32"/></Encoding>
	      </RelativeTimeParameterType>
	    </ParameterTypeSet>
	    <ParameterSet>
	      <Parameter name="Samples" parameterTypeRef="Samples_t"/>
	      <Parameter name="Position" parameterTypeRef="Position_t"/>
	      <Parameter name="Elapsed" parameterTypeRef="Elapsed_t"/>
	    </ParameterSet>
	  </TelemetryMetaData>
	</SpaceSystem>`

	db, err := xtce.Load(strings.NewReader(doc))
	if err != nil {
		t.Fatalf("Load() = %v", err)
	}

	// The definitions leave a trace: references to them resolve and Validate
	// passes, rather than reporting a phantom unresolved reference.
	if err := db.Validate(); err != nil {
		t.Fatalf("Validate() = %v; parameters of opaque types must resolve", err)
	}

	want := map[string]string{
		"Samples_t":  "array (not modeled)",
		"Position_t": "aggregate (not modeled)",
		"Elapsed_t":  "relative time (not modeled)",
	}
	for name, kind := range want {
		resolved, err := db.ResolveParameterType(name)
		if err != nil {
			t.Errorf("ResolveParameterType(%s) = %v", name, err)
			continue
		}
		if resolved.TypeKind() != kind {
			t.Errorf("%s TypeKind() = %q, want %q", name, resolved.TypeKind(), kind)
		}
		if resolved.Encoding() != nil {
			t.Errorf("%s has an encoding; opaque types must not claim one", name)
		}
	}

	array, _ := db.ResolveParameterType("Samples_t")
	if ref := array.(*xtce.ArrayParameterType).ArrayTypeRef; ref != "U8_t" {
		t.Errorf("arrayTypeRef = %q, want U8_t", ref)
	}
	if raw := array.(*xtce.ArrayParameterType).Raw; !strings.Contains(string(raw), "DimensionList") {
		t.Error("the array type's contents were not kept raw")
	}
}

// TestContextCalibratorListIsKept makes sure a context calibrator leaves a
// marker. A consumer who cannot see it would apply the default curve to every
// packet and compute wrong engineering values.
func TestContextCalibratorListIsKept(t *testing.T) {
	const doc = `<SpaceSystem xmlns="http://www.omg.org/spec/XTCE/20180204" name="Ctx">
	  <TelemetryMetaData>
	    <ParameterTypeSet>
	      <FloatParameterType name="Volts_t">
	        <IntegerDataEncoding sizeInBits="12">
	          <DefaultCalibrator>
	            <PolynomialCalibrator><Term coefficient="0.5" exponent="1"/></PolynomialCalibrator>
	          </DefaultCalibrator>
	          <ContextCalibratorList>
	            <ContextCalibrator>
	              <ContextMatch><Comparison parameterRef="Mode" value="1"/></ContextMatch>
	              <Calibrator><PolynomialCalibrator><Term coefficient="0.25" exponent="1"/></PolynomialCalibrator></Calibrator>
	            </ContextCalibrator>
	          </ContextCalibratorList>
	        </IntegerDataEncoding>
	      </FloatParameterType>
	      <FloatParameterType name="Plain_t">
	        <IntegerDataEncoding sizeInBits="12"/>
	      </FloatParameterType>
	    </ParameterTypeSet>
	  </TelemetryMetaData>
	</SpaceSystem>`

	db, err := xtce.Load(strings.NewReader(doc))
	if err != nil {
		t.Fatalf("Load() = %v", err)
	}

	withContext, _ := db.ResolveParameterType("Volts_t")
	if !withContext.Encoding().HasContextCalibrators() {
		t.Error("HasContextCalibrators() = false for an encoding that has one")
	}
	raw := withContext.Encoding().Integer.ContextCalibratorList
	if raw == nil || !strings.Contains(string(raw.Inner), "ContextCalibrator") {
		t.Errorf("ContextCalibratorList = %+v, want the raw XML kept", raw)
	}

	plain, _ := db.ResolveParameterType("Plain_t")
	if plain.Encoding().HasContextCalibrators() {
		t.Error("HasContextCalibrators() = true for an encoding without one")
	}
}

// TestChangeThresholdIsCarried covers the attribute on both numeric
// encodings. Absent means any change is significant, so it is a pointer.
func TestChangeThresholdIsCarried(t *testing.T) {
	const doc = `<SpaceSystem xmlns="http://www.omg.org/spec/XTCE/20180204" name="Delta">
	  <TelemetryMetaData>
	    <ParameterTypeSet>
	      <IntegerParameterType name="Counts_t">
	        <IntegerDataEncoding sizeInBits="16" changeThreshold="5"/>
	      </IntegerParameterType>
	      <FloatParameterType name="Volts_t">
	        <FloatDataEncoding sizeInBits="32" changeThreshold="0.25"/>
	      </FloatParameterType>
	      <IntegerParameterType name="Any_t">
	        <IntegerDataEncoding sizeInBits="16"/>
	      </IntegerParameterType>
	    </ParameterTypeSet>
	  </TelemetryMetaData>
	</SpaceSystem>`

	db, err := xtce.Load(strings.NewReader(doc))
	if err != nil {
		t.Fatalf("Load() = %v", err)
	}

	counts, _ := db.ResolveParameterType("Counts_t")
	if got := counts.Encoding().Integer.ChangeThreshold; got == nil || *got != 5 {
		t.Errorf("integer changeThreshold = %v, want 5", got)
	}
	volts, _ := db.ResolveParameterType("Volts_t")
	if got := volts.Encoding().Float.ChangeThreshold; got == nil || *got != 0.25 {
		t.Errorf("float changeThreshold = %v, want 0.25", got)
	}
	any, _ := db.ResolveParameterType("Any_t")
	if got := any.Encoding().Integer.ChangeThreshold; got != nil {
		t.Errorf("absent changeThreshold = %v, want nil (any change is significant)", got)
	}
}

// TestMoreSchemaDefaultsApply covers the defaults added by the audit: the
// parameter types' own sizeInBits, the boolean words, the time encoding's
// units, the unit's power, and the entry location's anchor.
func TestMoreSchemaDefaultsApply(t *testing.T) {
	const doc = `<SpaceSystem xmlns="http://www.omg.org/spec/XTCE/20180204" name="D">
	  <TelemetryMetaData>
	    <ParameterTypeSet>
	      <IntegerParameterType name="Bare_t">
	        <UnitSet>
	          <Unit>V</Unit>
	          <Unit power="2">m</Unit>
	        </UnitSet>
	        <IntegerDataEncoding/>
	      </IntegerParameterType>
	      <FloatParameterType name="BareFloat_t"><FloatDataEncoding/></FloatParameterType>
	      <IntegerParameterType name="Wide_t" sizeInBits="64"><IntegerDataEncoding/></IntegerParameterType>
	      <BooleanParameterType name="Flag_t"><IntegerDataEncoding sizeInBits="1"/></BooleanParameterType>
	      <BooleanParameterType name="Valve_t" oneStringValue="OPEN" zeroStringValue="SHUT">
	        <IntegerDataEncoding sizeInBits="1"/>
	      </BooleanParameterType>
	      <AbsoluteTimeParameterType name="Time_t">
	        <Encoding><IntegerDataEncoding sizeInBits="32"/></Encoding>
	      </AbsoluteTimeParameterType>
	    </ParameterTypeSet>
	    <ParameterSet>
	      <Parameter name="Bare" parameterTypeRef="Bare_t"/>
	    </ParameterSet>
	    <ContainerSet>
	      <SequenceContainer name="Packet">
	        <EntryList>
	          <ParameterRefEntry parameterRef="Bare">
	            <LocationInContainerInBits><FixedValue>8</FixedValue></LocationInContainerInBits>
	          </ParameterRefEntry>
	        </EntryList>
	      </SequenceContainer>
	    </ContainerSet>
	  </TelemetryMetaData>
	</SpaceSystem>`

	db, err := xtce.Load(strings.NewReader(doc))
	if err != nil {
		t.Fatalf("Load() = %v", err)
	}

	// The parameter type's own sizeInBits defaults to 32, distinct from the
	// encoding's, which defaults to 8 for integers.
	bare, _ := db.ResolveParameterType("Bare_t")
	if got := bare.(*xtce.IntegerParameterType).Size(); got != 32 {
		t.Errorf("integer type Size() = %d, want the default 32", got)
	}
	bareFloat, _ := db.ResolveParameterType("BareFloat_t")
	if got := bareFloat.(*xtce.FloatParameterType).Size(); got != 32 {
		t.Errorf("float type Size() = %d, want the default 32", got)
	}
	wide, _ := db.ResolveParameterType("Wide_t")
	if got := wide.(*xtce.IntegerParameterType).Size(); got != 64 {
		t.Errorf("explicit sizeInBits Size() = %d, want 64", got)
	}

	// Boolean words default to True and False.
	flag, _ := db.ResolveParameterType("Flag_t")
	boolean := flag.(*xtce.BooleanParameterType)
	if boolean.OneStringValueOrDefault() != "True" || boolean.ZeroStringValueOrDefault() != "False" {
		t.Errorf("boolean words = %q/%q, want True/False",
			boolean.OneStringValueOrDefault(), boolean.ZeroStringValueOrDefault())
	}
	valve, _ := db.ResolveParameterType("Valve_t")
	spelled := valve.(*xtce.BooleanParameterType)
	if spelled.OneStringValueOrDefault() != "OPEN" || spelled.ZeroStringValueOrDefault() != "SHUT" {
		t.Errorf("boolean words = %q/%q, want OPEN/SHUT",
			spelled.OneStringValueOrDefault(), spelled.ZeroStringValueOrDefault())
	}

	// A time encoding's units default to seconds.
	clock, _ := db.ResolveParameterType("Time_t")
	if got := clock.(*xtce.AbsoluteTimeParameterType).Encoding_.UnitsOrDefault(); got != "seconds" {
		t.Errorf("time units = %q, want the default seconds", got)
	}

	// A unit's power defaults to 1; an explicit power is kept.
	units := bare.(*xtce.IntegerParameterType).UnitSet.Units
	if got := units[0].PowerOrDefault(); got != 1 {
		t.Errorf("absent power = %v, want the default 1", got)
	}
	if got := units[1].PowerOrDefault(); got != 2 {
		t.Errorf("explicit power = %v, want 2", got)
	}

	// An entry location's anchor defaults to previousEntry.
	container, _ := db.FindContainer("/D/Packet")
	location := container.EntryList.Entries[0].LocationInContainerInBits
	if got := location.ReferenceLocationOrDefault(); got != "previousEntry" {
		t.Errorf("referenceLocation = %q, want the default previousEntry", got)
	}
}

// TestForeignNamespaceChildrenAreIgnored is the other half of namespace
// handling: matching child elements by local name alone would let another
// vocabulary's TelemetryMetaData masquerade as XTCE's.
func TestForeignNamespaceChildrenAreIgnored(t *testing.T) {
	const doc = `<SpaceSystem xmlns="http://www.omg.org/spec/XTCE/20180204" name="NS">
	  <TelemetryMetaData xmlns="http://example.com/not-xtce">
	    <ParameterTypeSet>
	      <IntegerParameterType name="Fake_t"><IntegerDataEncoding/></IntegerParameterType>
	    </ParameterTypeSet>
	  </TelemetryMetaData>
	</SpaceSystem>`

	db, err := xtce.Load(strings.NewReader(doc))
	if err != nil {
		t.Fatalf("Load() = %v; a foreign element is ignored, not fatal", err)
	}
	if db.TelemetryMetaData != nil {
		t.Errorf("TelemetryMetaData = %+v, want nil: it is in a foreign namespace", db.TelemetryMetaData)
	}
	if types := db.ParameterTypes(); len(types) != 0 {
		t.Errorf("ParameterTypes() = %+v, want none", types)
	}
}

// TestForeignNamespaceEntriesAreSkipped covers the hand-written EntryList
// decoder, which sees raw tokens and has to check the namespace itself.
func TestForeignNamespaceEntriesAreSkipped(t *testing.T) {
	const doc = `<SpaceSystem xmlns="http://www.omg.org/spec/XTCE/20180204"
	                          xmlns:alien="http://example.com/not-xtce" name="NS">
	  <TelemetryMetaData>
	    <ContainerSet>
	      <SequenceContainer name="Packet">
	        <EntryList>
	          <ParameterRefEntry parameterRef="A"/>
	          <alien:ParameterRefEntry parameterRef="Ghost"/>
	          <ParameterRefEntry parameterRef="B"/>
	        </EntryList>
	      </SequenceContainer>
	    </ContainerSet>
	  </TelemetryMetaData>
	</SpaceSystem>`

	db, err := xtce.Load(strings.NewReader(doc))
	if err != nil {
		t.Fatalf("Load() = %v", err)
	}
	entries := db.Containers()[0].EntryList.Entries
	if len(entries) != 2 {
		t.Fatalf("%d entries, want 2: the foreign one is not an XTCE entry", len(entries))
	}
	if entries[0].Ref != "A" || entries[1].Ref != "B" {
		t.Errorf("entries reference %q and %q, want A and B", entries[0].Ref, entries[1].Ref)
	}
}
