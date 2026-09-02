package xtce_test

import (
	"errors"
	"testing"

	"github.com/ravisuhag/astro/pkg/xtce"
)

// exprDB builds a database whose base container carries three octets — two
// identifiers and a mode — and whose derived container is selected by
// whatever criteria the test supplies.
func exprDB(t *testing.T, criteria string) (*xtce.SpaceSystem, *xtce.SequenceContainer) {
	t.Helper()

	db := parse(t, wrap("Sat", `
  <ParameterTypeSet>
    <IntegerParameterType name="U8" signed="false">
      <IntegerDataEncoding sizeInBits="8" encoding="unsigned"/>
    </IntegerParameterType>
    <EnumeratedParameterType name="Mode">
      <IntegerDataEncoding sizeInBits="8" encoding="unsigned"/>
      <EnumerationList>
        <Enumeration value="0" label="SAFE"/>
        <Enumeration value="1" label="NOMINAL"/>
      </EnumerationList>
    </EnumeratedParameterType>
  </ParameterTypeSet>
  <ParameterSet>
    <Parameter name="A" parameterTypeRef="U8"/>
    <Parameter name="B" parameterTypeRef="U8"/>
    <Parameter name="M" parameterTypeRef="Mode"/>
    <Parameter name="Payload" parameterTypeRef="U8"/>
  </ParameterSet>
  <ContainerSet>
    <SequenceContainer name="Base" abstract="true">
      <EntryList>
        <ParameterRefEntry parameterRef="A"/>
        <ParameterRefEntry parameterRef="B"/>
        <ParameterRefEntry parameterRef="M"/>
      </EntryList>
    </SequenceContainer>
    <SequenceContainer name="Derived">
      <EntryList><ParameterRefEntry parameterRef="Payload"/></EntryList>
      <BaseContainer containerRef="Base">
        <RestrictionCriteria>`+criteria+`</RestrictionCriteria>
      </BaseContainer>
    </SequenceContainer>
  </ContainerSet>`))

	base, err := db.FindContainer("/Sat/Base")
	if err != nil {
		t.Fatalf("FindContainer: %v", err)
	}
	return db, base
}

// matches reports whether a packet is selected by the criteria, failing the
// test on any error other than a plain non-match.
func matches(t *testing.T, criteria string, packet []byte) bool {
	t.Helper()

	db, base := exprDB(t, criteria)
	container, err := db.MatchFrom(base, packet)
	switch {
	case err == nil:
		return container.Name == "Derived"
	case errors.Is(err, xtce.ErrNoMatch):
		return false
	default:
		t.Fatalf("MatchFrom(%v): %v", packet, err)
		return false
	}
}

func condition(ref, operator, value string) string {
	return `<Condition>
		<ParameterInstanceRef parameterRef="` + ref + `"/>
		<ComparisonOperator>` + operator + `</ComparisonOperator>
		<Value>` + value + `</Value>
	</Condition>`
}

// A single Condition behaves like a Comparison.
func TestBooleanExpressionSingleCondition(t *testing.T) {
	criteria := `<BooleanExpression>` + condition("A", "==", "7") + `</BooleanExpression>`

	if !matches(t, criteria, []byte{7, 0, 0, 0}) {
		t.Error("A == 7 did not match a packet with A = 7")
	}
	if matches(t, criteria, []byte{8, 0, 0, 0}) {
		t.Error("A == 7 matched a packet with A = 8")
	}
}

func TestBooleanExpressionANDedConditions(t *testing.T) {
	criteria := `<BooleanExpression><ANDedConditions>` +
		condition("A", "==", "1") + condition("B", "==", "2") +
		`</ANDedConditions></BooleanExpression>`

	if !matches(t, criteria, []byte{1, 2, 0, 0}) {
		t.Error("both conditions held but the packet did not match")
	}
	if matches(t, criteria, []byte{1, 3, 0, 0}) {
		t.Error("only the first condition held, yet the packet matched")
	}
	if matches(t, criteria, []byte{9, 2, 0, 0}) {
		t.Error("only the second condition held, yet the packet matched")
	}
}

// This is what a BooleanExpression can express and the other two criteria
// forms cannot: an alternative.
func TestBooleanExpressionORedConditions(t *testing.T) {
	criteria := `<BooleanExpression><ORedConditions>` +
		condition("A", "==", "1") + condition("A", "==", "5") +
		`</ORedConditions></BooleanExpression>`

	if !matches(t, criteria, []byte{1, 0, 0, 0}) {
		t.Error("the first alternative held but the packet did not match")
	}
	if !matches(t, criteria, []byte{5, 0, 0, 0}) {
		t.Error("the second alternative held but the packet did not match")
	}
	if matches(t, criteria, []byte{3, 0, 0, 0}) {
		t.Error("neither alternative held, yet the packet matched")
	}
}

// The schema nests the opposite kind inside each group, which is how a
// database writes "A is 1, and B is either 2 or 3" without parentheses.
func TestBooleanExpressionNestedGroups(t *testing.T) {
	criteria := `<BooleanExpression><ANDedConditions>` +
		condition("A", "==", "1") +
		`<ORedConditions>` + condition("B", "==", "2") + condition("B", "==", "3") + `</ORedConditions>` +
		`</ANDedConditions></BooleanExpression>`

	if !matches(t, criteria, []byte{1, 2, 0, 0}) {
		t.Error("A = 1, B = 2 did not match")
	}
	if !matches(t, criteria, []byte{1, 3, 0, 0}) {
		t.Error("A = 1, B = 3 did not match")
	}
	if matches(t, criteria, []byte{1, 4, 0, 0}) {
		t.Error("A = 1, B = 4 matched, but B was neither alternative")
	}
	if matches(t, criteria, []byte{2, 2, 0, 0}) {
		t.Error("A = 2 matched, but the outer AND required A = 1")
	}
}

// An OR inside an OR, and an AND inside an AND. The schema only nests the
// opposite kind, but both joins are associative, so a document that nests the
// same kind means what it reads as rather than being an error.
func TestBooleanExpressionNestsTheSameKind(t *testing.T) {
	sameKindOR := `<BooleanExpression><ORedConditions>` +
		condition("A", "==", "1") +
		`<ORedConditions>` + condition("A", "==", "2") + condition("A", "==", "3") + `</ORedConditions>` +
		`</ORedConditions></BooleanExpression>`

	for _, value := range []byte{1, 2, 3} {
		if !matches(t, sameKindOR, []byte{value, 0, 0, 0}) {
			t.Errorf("A = %d did not match any of the three alternatives", value)
		}
	}
	if matches(t, sameKindOR, []byte{4, 0, 0, 0}) {
		t.Error("A = 4 matched, but it is none of the three alternatives")
	}
}

// The right-hand side may be a second parameter, which is the other thing a
// Comparison cannot say: a container selected by two fields agreeing.
func TestBooleanExpressionComparesTwoParameters(t *testing.T) {
	criteria := `<BooleanExpression>
		<Condition>
			<ParameterInstanceRef parameterRef="A"/>
			<ComparisonOperator>==</ComparisonOperator>
			<ParameterInstanceRef parameterRef="B"/>
		</Condition>
	</BooleanExpression>`

	if !matches(t, criteria, []byte{4, 4, 0, 0}) {
		t.Error("A == B did not match a packet where both are 4")
	}
	if matches(t, criteria, []byte{4, 5, 0, 0}) {
		t.Error("A == B matched a packet where they differ")
	}
}

func TestBooleanExpressionOrdersTwoParameters(t *testing.T) {
	criteria := `<BooleanExpression>
		<Condition>
			<ParameterInstanceRef parameterRef="A"/>
			<ComparisonOperator>&lt;</ComparisonOperator>
			<ParameterInstanceRef parameterRef="B"/>
		</Condition>
	</BooleanExpression>`

	if !matches(t, criteria, []byte{1, 9, 0, 0}) {
		t.Error("A < B did not match 1 < 9")
	}
	if matches(t, criteria, []byte{9, 1, 0, 0}) {
		t.Error("A < B matched 9 < 1")
	}
	if matches(t, criteria, []byte{5, 5, 0, 0}) {
		t.Error("A < B matched two equal values")
	}
}

// The six operators the schema's ComparisonOperatorsType allows.
func TestBooleanExpressionOperators(t *testing.T) {
	for _, tc := range []struct {
		operator string
		a        byte
		want     bool
	}{
		{"==", 5, true},
		{"==", 6, false},
		{"!=", 6, true},
		{"!=", 5, false},
		{"&lt;", 4, true},
		{"&lt;", 5, false},
		{"&lt;=", 5, true},
		{"&lt;=", 6, false},
		{"&gt;", 6, true},
		{"&gt;", 5, false},
		{"&gt;=", 5, true},
		{"&gt;=", 4, false},
	} {
		criteria := `<BooleanExpression>` + condition("A", tc.operator, "5") + `</BooleanExpression>`
		if got := matches(t, criteria, []byte{tc.a, 0, 0, 0}); got != tc.want {
			t.Errorf("A %s 5 with A = %d = %v, want %v", tc.operator, tc.a, got, tc.want)
		}
	}
}

// A condition reads the engineering value unless it says otherwise, so an
// enumeration compares by label. The schema defaults useCalibratedValue to
// true.
func TestBooleanExpressionUsesCalibratedValueByDefault(t *testing.T) {
	criteria := `<BooleanExpression>` + condition("M", "==", "NOMINAL") + `</BooleanExpression>`

	if !matches(t, criteria, []byte{0, 0, 1, 0}) {
		t.Error("M == NOMINAL did not match the raw value the label maps from")
	}
	if matches(t, criteria, []byte{0, 0, 0, 0}) {
		t.Error("M == NOMINAL matched the SAFE value")
	}
}

// With useCalibratedValue="false" the raw count is compared instead, so the
// same field is tested by number rather than by label.
func TestBooleanExpressionCanUseTheRawValue(t *testing.T) {
	criteria := `<BooleanExpression>
		<Condition>
			<ParameterInstanceRef parameterRef="M" useCalibratedValue="false"/>
			<ComparisonOperator>==</ComparisonOperator>
			<Value>1</Value>
		</Condition>
	</BooleanExpression>`

	if !matches(t, criteria, []byte{0, 0, 1, 0}) {
		t.Error("the raw comparison did not match a raw 1")
	}
	if matches(t, criteria, []byte{0, 0, 0, 0}) {
		t.Error("the raw comparison matched a raw 0")
	}
}

// A packet that ends before the field a condition tests is a failed match,
// not a broken database: a truncated packet is a normal thing to receive.
func TestBooleanExpressionShortPacketIsNotAMatch(t *testing.T) {
	criteria := `<BooleanExpression>` + condition("M", "==", "NOMINAL") + `</BooleanExpression>`
	db, base := exprDB(t, criteria)

	// One octet, so M at bit 16 is not there.
	if _, err := db.MatchFrom(base, []byte{0}); !errors.Is(err, xtce.ErrNoMatch) {
		t.Errorf("MatchFrom(one octet) = %v, want ErrNoMatch", err)
	}
}

// Malformed expressions are reported rather than read as true, which would
// let the container swallow every packet, or as false, which would silently
// misroute them.
func TestBooleanExpressionRejectsMalformedForms(t *testing.T) {
	for name, criteria := range map[string]string{
		"empty expression": `<BooleanExpression/>`,
		"empty AND group":  `<BooleanExpression><ANDedConditions/></BooleanExpression>`,
		"empty OR group":   `<BooleanExpression><ORedConditions/></BooleanExpression>`,
		"no parameter": `<BooleanExpression><Condition>
			<ComparisonOperator>==</ComparisonOperator><Value>1</Value></Condition></BooleanExpression>`,
		"no operator": `<BooleanExpression><Condition>
			<ParameterInstanceRef parameterRef="A"/><Value>1</Value></Condition></BooleanExpression>`,
		"no right-hand side": `<BooleanExpression><Condition>
			<ParameterInstanceRef parameterRef="A"/>
			<ComparisonOperator>==</ComparisonOperator></Condition></BooleanExpression>`,
		"unknown operator": `<BooleanExpression><Condition>
			<ParameterInstanceRef parameterRef="A"/>
			<ComparisonOperator>~=</ComparisonOperator><Value>1</Value></Condition></BooleanExpression>`,
	} {
		t.Run(name, func(t *testing.T) {
			db, base := exprDB(t, criteria)
			_, err := db.MatchFrom(base, []byte{1, 2, 3, 4})
			if !errors.Is(err, xtce.ErrUnsupportedCriteria) {
				t.Errorf("MatchFrom = %v, want ErrUnsupportedCriteria", err)
			}
		})
	}
}

// An instance other than 0 refers to another packet in the stream, which one
// packet cannot answer. Reading it as the current value would compare the
// wrong thing silently.
func TestBooleanExpressionRefusesOtherInstances(t *testing.T) {
	for name, criteria := range map[string]string{
		"on the left": `<BooleanExpression><Condition>
			<ParameterInstanceRef parameterRef="A" instance="-1"/>
			<ComparisonOperator>==</ComparisonOperator><Value>1</Value></Condition></BooleanExpression>`,
		"on the right": `<BooleanExpression><Condition>
			<ParameterInstanceRef parameterRef="A"/>
			<ComparisonOperator>==</ComparisonOperator>
			<ParameterInstanceRef parameterRef="B" instance="-1"/></Condition></BooleanExpression>`,
	} {
		t.Run(name, func(t *testing.T) {
			db, base := exprDB(t, criteria)
			_, err := db.MatchFrom(base, []byte{1, 1, 0, 0})
			if !errors.Is(err, xtce.ErrUnsupportedCriteria) {
				t.Errorf("MatchFrom = %v, want ErrUnsupportedCriteria", err)
			}
		})
	}
}

// Comparing a label against a number is a database error, not a false
// result: nothing sensible can come of it.
func TestBooleanExpressionRefusesMismatchedKinds(t *testing.T) {
	criteria := `<BooleanExpression>
		<Condition>
			<ParameterInstanceRef parameterRef="M"/>
			<ComparisonOperator>==</ComparisonOperator>
			<ParameterInstanceRef parameterRef="A"/>
		</Condition>
	</BooleanExpression>`

	db, base := exprDB(t, criteria)
	if _, err := db.MatchFrom(base, []byte{1, 1, 1, 0}); !errors.Is(err, xtce.ErrUnsupportedCriteria) {
		t.Errorf("MatchFrom = %v, want ErrUnsupportedCriteria", err)
	}
}

// A label has no order, so an ordering operator on two of them means
// nothing.
func TestBooleanExpressionRefusesOrderedLabels(t *testing.T) {
	criteria := `<BooleanExpression>
		<Condition>
			<ParameterInstanceRef parameterRef="M"/>
			<ComparisonOperator>&lt;</ComparisonOperator>
			<ParameterInstanceRef parameterRef="M"/>
		</Condition>
	</BooleanExpression>`

	db, base := exprDB(t, criteria)
	if _, err := db.MatchFrom(base, []byte{0, 0, 1, 0}); !errors.Is(err, xtce.ErrUnsupportedCriteria) {
		t.Errorf("MatchFrom = %v, want ErrUnsupportedCriteria", err)
	}
}
