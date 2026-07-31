package xtce_test

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ravisuhag/astro/pkg/xtce"
)

// load reads a fixture or fails the test.
func load(t *testing.T, name string) *xtce.SpaceSystem {
	t.Helper()
	db, err := xtce.LoadFile(filepath.Join("testdata", name))
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
	if _, err := xtce.LoadFile(filepath.Join("testdata", "invalid-root.xml")); !errors.Is(err, xtce.ErrNotSpaceSystem) {
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
	if _, err := xtce.LoadFile(filepath.Join("testdata", "does-not-exist.xml")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("LoadFile() = %v, want a not-exist error", err)
	}
}
