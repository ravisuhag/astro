package xtce_test

import (
	"errors"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/ravisuhag/astro/pkg/xtce"
)

func TestValidateAcceptsGoodFixtures(t *testing.T) {
	for _, name := range []string{"ccsds-header.xml", "ccsds-header-prefixed.xml", "nested.xml"} {
		t.Run(name, func(t *testing.T) {
			if err := load(t, name).Validate(); err != nil {
				t.Errorf("Validate() = %v", err)
			}
		})
	}
}

// TestValidateCatchesEachDefect pairs one fixture with one sentinel.
func TestValidateCatchesEachDefect(t *testing.T) {
	tests := []struct {
		fixture string
		want    error
	}{
		{"invalid-unresolved-type.xml", xtce.ErrUnresolvedReference},
		{"invalid-unresolved-entry.xml", xtce.ErrUnresolvedReference},
		{"invalid-container-cycle.xml", xtce.ErrContainerCycle},
		{"invalid-duplicate-name.xml", xtce.ErrDuplicateName},
	}

	for _, test := range tests {
		t.Run(test.fixture, func(t *testing.T) {
			// The file loads: Load checks shape, Validate checks meaning.
			db := load(t, test.fixture)

			err := db.Validate()
			if err == nil {
				t.Fatal("Validate() accepted a broken database")
			}
			if !errors.Is(err, test.want) {
				t.Errorf("Validate() = %v, want %v", err, test.want)
			}
		})
	}
}

// TestValidateReportsEveryProblem checks that validation collects rather than
// stopping at the first fault, which is what someone repairing a database
// needs.
func TestValidateReportsEveryProblem(t *testing.T) {
	const doc = `<SpaceSystem xmlns="http://www.omg.org/spec/XTCE/20180204" name="Broken">
	  <TelemetryMetaData>
	    <ParameterSet>
	      <Parameter name="A" parameterTypeRef="Missing1_t"/>
	      <Parameter name="B" parameterTypeRef="Missing2_t"/>
	      <Parameter name="C" parameterTypeRef="Missing3_t"/>
	    </ParameterSet>
	  </TelemetryMetaData>
	</SpaceSystem>`

	db, err := xtce.Load(strings.NewReader(doc))
	if err != nil {
		t.Fatalf("Load() = %v", err)
	}

	validationErr := db.Validate()
	if validationErr == nil {
		t.Fatal("Validate() accepted three dangling references")
	}

	var problems xtce.ValidationErrors
	if !errors.As(validationErr, &problems) {
		t.Fatalf("Validate() returned %T, want ValidationErrors", validationErr)
	}
	if len(problems) != 3 {
		t.Errorf("reported %d problems, want 3: %v", len(problems), problems)
	}
	for _, problem := range problems {
		if problem.SpaceSystem != "/Broken" {
			t.Errorf("problem names system %q, want /Broken", problem.SpaceSystem)
		}
		if !errors.Is(problem, xtce.ErrUnresolvedReference) {
			t.Errorf("problem %v does not carry ErrUnresolvedReference", problem)
		}
	}
}

// TestValidateFindsProblemsInSubsystems checks that validation walks the whole
// tree, not just the root.
func TestValidateFindsProblemsInSubsystems(t *testing.T) {
	const doc = `<SpaceSystem xmlns="http://www.omg.org/spec/XTCE/20180204" name="Root">
	  <SpaceSystem name="Child">
	    <TelemetryMetaData>
	      <ParameterSet>
	        <Parameter name="A" parameterTypeRef="Missing_t"/>
	      </ParameterSet>
	    </TelemetryMetaData>
	  </SpaceSystem>
	</SpaceSystem>`

	db, err := xtce.Load(strings.NewReader(doc))
	if err != nil {
		t.Fatalf("Load() = %v", err)
	}

	var problems xtce.ValidationErrors
	if !errors.As(db.Validate(), &problems) {
		t.Fatal("Validate() found nothing wrong in the subsystem")
	}
	if problems[0].SpaceSystem != "/Root/Child" {
		t.Errorf("problem names %q, want /Root/Child", problems[0].SpaceSystem)
	}
}

// TestSelfInheritingContainerIsACycle covers the shortest cycle there is.
func TestSelfInheritingContainerIsACycle(t *testing.T) {
	const doc = `<SpaceSystem xmlns="http://www.omg.org/spec/XTCE/20180204" name="Broken">
	  <TelemetryMetaData>
	    <ContainerSet>
	      <SequenceContainer name="Loop">
	        <EntryList/>
	        <BaseContainer containerRef="Loop"/>
	      </SequenceContainer>
	    </ContainerSet>
	  </TelemetryMetaData>
	</SpaceSystem>`

	db, err := xtce.Load(strings.NewReader(doc))
	if err != nil {
		t.Fatalf("Load() = %v", err)
	}
	if err := db.Validate(); !errors.Is(err, xtce.ErrContainerCycle) {
		t.Fatalf("Validate() = %v, want ErrContainerCycle", err)
	}
}

// TestLongInheritanceChainIsFine checks the cycle detector does not mistake
// depth for a loop.
func TestLongInheritanceChainIsFine(t *testing.T) {
	var b strings.Builder
	b.WriteString(`<SpaceSystem xmlns="http://www.omg.org/spec/XTCE/20180204" name="Deep">
	  <TelemetryMetaData><ContainerSet>`)
	const chain = 20
	b.WriteString(`<SequenceContainer name="C0"><EntryList/></SequenceContainer>`)
	for i := 1; i < chain; i++ {
		b.WriteString(`<SequenceContainer name="C` + itoa(i) + `"><EntryList/>` +
			`<BaseContainer containerRef="C` + itoa(i-1) + `"/></SequenceContainer>`)
	}
	b.WriteString(`</ContainerSet></TelemetryMetaData></SpaceSystem>`)

	db, err := xtce.Load(strings.NewReader(b.String()))
	if err != nil {
		t.Fatalf("Load() = %v", err)
	}
	if err := db.Validate(); err != nil {
		t.Errorf("Validate() = %v, want a chain of %d to be fine", err, chain)
	}
}

// TestSameContainerNameInTwoSystemsIsNotACycle guards the reason the cycle
// check compares pointers rather than names.
func TestSameContainerNameInTwoSystemsIsNotACycle(t *testing.T) {
	const doc = `<SpaceSystem xmlns="http://www.omg.org/spec/XTCE/20180204" name="Root">
	  <TelemetryMetaData>
	    <ContainerSet>
	      <SequenceContainer name="Common"><EntryList/></SequenceContainer>
	    </ContainerSet>
	  </TelemetryMetaData>
	  <SpaceSystem name="Child">
	    <TelemetryMetaData>
	      <ContainerSet>
	        <SequenceContainer name="Common">
	          <EntryList/>
	          <BaseContainer containerRef="/Root/Common"/>
	        </SequenceContainer>
	      </ContainerSet>
	    </TelemetryMetaData>
	  </SpaceSystem>
	</SpaceSystem>`

	db, err := xtce.Load(strings.NewReader(doc))
	if err != nil {
		t.Fatalf("Load() = %v", err)
	}
	if err := db.Validate(); err != nil {
		t.Errorf("Validate() = %v; two containers sharing a name in different systems is legal", err)
	}
}

// TestUnresolvedBaseContainerIsReported covers the other half of the
// inheritance check.
func TestUnresolvedBaseContainerIsReported(t *testing.T) {
	const doc = `<SpaceSystem xmlns="http://www.omg.org/spec/XTCE/20180204" name="Broken">
	  <TelemetryMetaData>
	    <ContainerSet>
	      <SequenceContainer name="Child">
	        <EntryList/>
	        <BaseContainer containerRef="NoSuchBase"/>
	      </SequenceContainer>
	    </ContainerSet>
	  </TelemetryMetaData>
	</SpaceSystem>`

	db, err := xtce.Load(strings.NewReader(doc))
	if err != nil {
		t.Fatalf("Load() = %v", err)
	}
	if err := db.Validate(); err == nil {
		t.Fatal("Validate() accepted a base container that does not resolve")
	}
}

// TestDuplicateSiblingSystemsAreRejected: two subsystems with one name make
// any path through them ambiguous.
func TestDuplicateSiblingSystemsAreRejected(t *testing.T) {
	const doc = `<SpaceSystem xmlns="http://www.omg.org/spec/XTCE/20180204" name="Root">
	  <SpaceSystem name="Twin"/>
	  <SpaceSystem name="Twin"/>
	</SpaceSystem>`

	db, err := xtce.Load(strings.NewReader(doc))
	if err != nil {
		t.Fatalf("Load() = %v", err)
	}
	if err := db.Validate(); !errors.Is(err, xtce.ErrDuplicateName) {
		t.Fatalf("Validate() = %v, want ErrDuplicateName", err)
	}
}

// TestValidationErrorMessageNamesTheProblem checks the message is useful
// enough to find the fault in the file.
func TestValidationErrorMessageNamesTheProblem(t *testing.T) {
	db := load(t, "invalid-unresolved-type.xml")

	err := db.Validate()
	if err == nil {
		t.Fatal("Validate() accepted a broken database")
	}
	message := err.Error()
	for _, want := range []string{"/Broken", "Parameter", "Reading", "NoSuchType_t"} {
		if !strings.Contains(message, want) {
			t.Errorf("the message does not mention %q: %s", want, message)
		}
	}
}

// itoa is strconv.Itoa under a shorter name, for readability in the fixture
// builders above.
func itoa(i int) string {
	return strconv.Itoa(i)
}

// TestValidateScalesLinearly guards against the shape of bug that made the
// first version of the inheritance check cubic: it re-walked the tree to find
// each base container's home, so a long chain of containers took time
// proportional to the cube of their number. An 80 KB file took 200 ms, and the
// size cap allows files hundreds of times larger, which made Validate a way to
// hang a process with a document.
//
// The threshold is loose on purpose. This test is not measuring speed, it is
// noticing a return to super-linear growth, and a loose bound does that
// without failing on a busy machine.
func TestValidateScalesLinearly(t *testing.T) {
	chain := func(n int) *xtce.SpaceSystem {
		t.Helper()
		var b strings.Builder
		b.WriteString(`<SpaceSystem xmlns="` + xtce.Namespace + `" name="D">` +
			`<TelemetryMetaData><ContainerSet>`)
		b.WriteString(`<SequenceContainer name="C0"><EntryList/></SequenceContainer>`)
		for i := 1; i < n; i++ {
			b.WriteString(`<SequenceContainer name="C` + itoa(i) + `"><EntryList/>` +
				`<BaseContainer containerRef="C` + itoa(i-1) + `"/></SequenceContainer>`)
		}
		b.WriteString(`</ContainerSet></TelemetryMetaData></SpaceSystem>`)

		db, err := xtce.Load(strings.NewReader(b.String()))
		if err != nil {
			t.Fatalf("Load() = %v", err)
		}
		return db
	}

	db := chain(2000)
	start := time.Now()
	if err := db.Validate(); err != nil {
		t.Fatalf("Validate() = %v", err)
	}
	elapsed := time.Since(start)

	// The cubic version took about 200 ms at 800 containers, so at 2000 it
	// would take minutes. Anything under a second means the growth is sane.
	if elapsed > time.Second {
		t.Errorf("validating a 2000-container chain took %v; the check has gone super-linear again", elapsed)
	}
}

// TestCycleFoundInALongChain checks the colouring still catches a cycle at the
// far end of a chain, where the shortcut for already-proved containers could
// hide it.
func TestCycleFoundInALongChain(t *testing.T) {
	var b strings.Builder
	const n = 50
	b.WriteString(`<SpaceSystem xmlns="` + xtce.Namespace + `" name="D">` +
		`<TelemetryMetaData><ContainerSet>`)
	// C0 extends C49, closing the loop at the far end.
	b.WriteString(`<SequenceContainer name="C0"><EntryList/>` +
		`<BaseContainer containerRef="C` + itoa(n-1) + `"/></SequenceContainer>`)
	for i := 1; i < n; i++ {
		b.WriteString(`<SequenceContainer name="C` + itoa(i) + `"><EntryList/>` +
			`<BaseContainer containerRef="C` + itoa(i-1) + `"/></SequenceContainer>`)
	}
	b.WriteString(`</ContainerSet></TelemetryMetaData></SpaceSystem>`)

	db, err := xtce.Load(strings.NewReader(b.String()))
	if err != nil {
		t.Fatalf("Load() = %v", err)
	}
	if err := db.Validate(); !errors.Is(err, xtce.ErrContainerCycle) {
		t.Fatalf("Validate() = %v, want ErrContainerCycle", err)
	}
}

// TestSharedBaseIsWalkedOnce is the other half of the linear claim: many
// containers extending one base must not each re-walk it.
func TestSharedBaseIsWalkedOnce(t *testing.T) {
	var b strings.Builder
	const leaves = 500
	b.WriteString(`<SpaceSystem xmlns="` + xtce.Namespace + `" name="D">` +
		`<TelemetryMetaData><ContainerSet>`)
	b.WriteString(`<SequenceContainer name="Base"><EntryList/></SequenceContainer>`)
	for i := range leaves {
		b.WriteString(`<SequenceContainer name="L` + itoa(i) + `"><EntryList/>` +
			`<BaseContainer containerRef="Base"/></SequenceContainer>`)
	}
	b.WriteString(`</ContainerSet></TelemetryMetaData></SpaceSystem>`)

	db, err := xtce.Load(strings.NewReader(b.String()))
	if err != nil {
		t.Fatalf("Load() = %v", err)
	}
	if err := db.Validate(); err != nil {
		t.Fatalf("Validate() = %v", err)
	}
}

// TestValidateRejectsIllegalEncodingEnums covers the enumerated encoding
// attributes. A misspelled member used to be invisible until decode time,
// where it surfaced on every packet as an unsupported encoding.
func TestValidateRejectsIllegalEncodingEnums(t *testing.T) {
	const doc = `<SpaceSystem xmlns="http://www.omg.org/spec/XTCE/20180204" name="Enums">
	  <TelemetryMetaData>
	    <ParameterTypeSet>
	      <IntegerParameterType name="BadKind_t">
	        <IntegerDataEncoding encoding="unsgined" sizeInBits="8"/>
	      </IntegerParameterType>
	      <FloatParameterType name="BadFloat_t">
	        <FloatDataEncoding encoding="IEEE754_2008" sizeInBits="32"/>
	      </FloatParameterType>
	      <StringParameterType name="BadText_t">
	        <StringDataEncoding encoding="EBCDIC"/>
	      </StringParameterType>
	      <IntegerParameterType name="BadBitOrder_t">
	        <IntegerDataEncoding sizeInBits="8" bitOrder="littleEndian"/>
	      </IntegerParameterType>
	      <IntegerParameterType name="BadByteOrder_t">
	        <IntegerDataEncoding sizeInBits="8" byteOrder="sideways"/>
	      </IntegerParameterType>
	    </ParameterTypeSet>
	  </TelemetryMetaData>
	</SpaceSystem>`

	db, err := xtce.Load(strings.NewReader(doc))
	if err != nil {
		t.Fatalf("Load() = %v; enum membership is Validate's job, not Load's", err)
	}

	err = db.Validate()
	if !errors.Is(err, xtce.ErrInvalidEncoding) {
		t.Fatalf("Validate() = %v, want ErrInvalidEncoding", err)
	}

	var problems xtce.ValidationErrors
	if !errors.As(err, &problems) {
		t.Fatalf("Validate() returned %T, want ValidationErrors", err)
	}
	if len(problems) != 5 {
		t.Errorf("%d problems, want 5 (one per illegal attribute):\n%v", len(problems), err)
	}
	// The message must name the type, so the fault is findable in the file.
	if !strings.Contains(err.Error(), "BadKind_t") || !strings.Contains(err.Error(), "unsgined") {
		t.Errorf("the error does not name the type and the bad value:\n%v", err)
	}
}

// TestValidateAcceptsLegalEncodingEnums makes sure the check accepts every
// spelling the schema allows, including the arbitrary byte-list order.
func TestValidateAcceptsLegalEncodingEnums(t *testing.T) {
	const doc = `<SpaceSystem xmlns="http://www.omg.org/spec/XTCE/20180204" name="Enums">
	  <TelemetryMetaData>
	    <ParameterTypeSet>
	      <IntegerParameterType name="BCD_t">
	        <IntegerDataEncoding encoding="packedBCD" sizeInBits="16"
	                             bitOrder="leastSignificantBitFirst" byteOrder="leastSignificantByteFirst"/>
	      </IntegerParameterType>
	      <FloatParameterType name="Mil_t">
	        <FloatDataEncoding encoding="MILSTD_1750A" sizeInBits="32" byteOrder="3,2,1,0"/>
	      </FloatParameterType>
	      <StringParameterType name="Ascii_t">
	        <StringDataEncoding encoding="US-ASCII"/>
	      </StringParameterType>
	    </ParameterTypeSet>
	  </TelemetryMetaData>
	</SpaceSystem>`

	db, err := xtce.Load(strings.NewReader(doc))
	if err != nil {
		t.Fatalf("Load() = %v", err)
	}
	if err := db.Validate(); err != nil {
		t.Fatalf("Validate() = %v; every attribute here is a legal member", err)
	}
}
