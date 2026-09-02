package xtce_test

import (
	"encoding/xml"
	"errors"
	"math"
	"strings"
	"testing"

	"github.com/ravisuhag/astro/pkg/xtce"
)

// mathXML wraps an operand run in the element the schema names, so the tests
// exercise the real unmarshaller rather than a hand-built struct.
func mathXML(inner string) string {
	return `<MathOperationCalibrator xmlns="http://www.omg.org/spec/XTCE/20180204">` +
		inner + `</MathOperationCalibrator>`
}

func decodeMath(t *testing.T, inner string) *xtce.MathOperationCalibrator {
	t.Helper()

	var calibrator xtce.MathOperationCalibrator
	if err := xml.Unmarshal([]byte(mathXML(inner)), &calibrator); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	return &calibrator
}

// apply is the common shape: decode an expression, evaluate it at raw, and
// require a value close to want.
func apply(t *testing.T, inner string, raw, want float64) {
	t.Helper()

	got, err := decodeMath(t, inner).Apply(raw)
	if err != nil {
		t.Fatalf("Apply(%v) on %s: %v", raw, inner, err)
	}
	if math.Abs(got-want) > 1e-9 {
		t.Errorf("Apply(%v) on %s = %v, want %v", raw, inner, got, want)
	}
}

// The schema's own worked example: "the stack, 4 8 /, would result as 0.5".
func TestMathOperationSchemaExample(t *testing.T) {
	apply(t, `<ValueOperand>4</ValueOperand>
		<ValueOperand>8</ValueOperand>
		<Operator>/</Operator>`, 0, 0.5)
}

// Operand order is the whole meaning of a postfix expression, so the
// unmarshaller has to keep the document's order across the four element names.
func TestMathOperationKeepsDocumentOrder(t *testing.T) {
	calibrator := decodeMath(t, `<ValueOperand>1</ValueOperand>
		<ThisParameterOperand/>
		<Operator>+</Operator>
		<ParameterInstanceRefOperand parameterRef="/S/Other"/>
		<Operator>*</Operator>`)

	want := []xtce.MathOperandKind{
		xtce.MathValue,
		xtce.MathThisParameter,
		xtce.MathOperator,
		xtce.MathParameterInstance,
		xtce.MathOperator,
	}
	if len(calibrator.Operands) != len(want) {
		t.Fatalf("got %d operands, want %d", len(calibrator.Operands), len(want))
	}
	for i, kind := range want {
		if calibrator.Operands[i].Kind != kind {
			t.Errorf("operand %d is %v, want %v", i, calibrator.Operands[i].Kind, kind)
		}
	}
}

// ThisParameterOperand is the value being calibrated, which is what makes a
// calibrator a function of its input rather than a constant.
func TestMathOperationUsesTheRawValue(t *testing.T) {
	// (raw * 2) + 1
	expr := `<ThisParameterOperand/>
		<ValueOperand>2</ValueOperand>
		<Operator>*</Operator>
		<ValueOperand>1</ValueOperand>
		<Operator>+</Operator>`

	apply(t, expr, 10, 21)
	apply(t, expr, 0, 1)
	apply(t, expr, -3, -5)
}

// The two-operand operators take the top of the stack as the right-hand side,
// which matters for every operator that is not commutative.
func TestMathOperationBinaryOperandOrder(t *testing.T) {
	pair := `<ValueOperand>10</ValueOperand><ValueOperand>3</ValueOperand>`

	for _, tc := range []struct {
		operator string
		want     float64
	}{
		{"-", 7},                     // (x1 x2 -- x1-x2)
		{"/", 10.0 / 3.0},            // (x1 x2 -- x1/x2)
		{"%", 1},                     // 10 mod 3
		{"^", 1000},                  // (x1 x2 -- x1**x2)
		{"y^x", math.Pow(3, 10)},     // (x1 x2 -- x2**x1), reversed
		{"min", 3},                   // (x1 x2 -- min(x1, x2))
		{"max", 10},                  // (x1 x2 -- max(x1, x2))
		{"&lt;", 0},                  // 10 < 3 is false
		{"&gt;", 1},                  // 10 > 3 is true
		{"atan2", math.Atan2(3, 10)}, // (x1 x2 -- atan2(x2, x1))
	} {
		apply(t, pair+`<Operator>`+tc.operator+`</Operator>`, 0, tc.want)
	}
}

// y^x and atan2 both read their operands in the opposite order to the rest,
// which the schema states in its stack notation and nowhere else. Getting
// either backwards is silent: the expression still evaluates, to the wrong
// number.
func TestMathOperationReversedOperators(t *testing.T) {
	// 2 3 y^x is 3**2, not 2**3.
	apply(t, `<ValueOperand>2</ValueOperand>
		<ValueOperand>3</ValueOperand>
		<Operator>y^x</Operator>`, 0, 9)

	// 0 1 atan2 is atan2(1, 0), a quarter turn, not atan2(0, 1) which is 0.
	apply(t, `<ValueOperand>0</ValueOperand>
		<ValueOperand>1</ValueOperand>
		<Operator>atan2</Operator>`, 0, math.Pi/2)
}

func TestMathOperationUnaryFunctions(t *testing.T) {
	for _, tc := range []struct {
		operator string
		input    float64
		want     float64
	}{
		{"ln", math.E, 1},
		{"log", 1000, 3},
		{"e^x", 1, math.E},
		{"1/x", 4, 0.25},
		{"abs", -7.5, 7.5},
		{"int", -7.9, -7}, // the integer part, so toward zero
		{"x!", 5, 120},
		{"sin", 0, 0},
		{"cos", 0, 1},
		{"asin", 1, math.Pi / 2},
		{"acos", 1, 0},
		{"atan", 1, math.Pi / 4},
		{"sinh", 0, 0},
		{"cosh", 0, 1},
		{"tanh", 0, 0},
		{"asinh", 0, 0},
		{"acosh", 1, 0},
		{"atanh", 0, 0},
	} {
		apply(t, `<ThisParameterOperand/><Operator>`+tc.operator+`</Operator>`, tc.input, tc.want)
	}
}

func TestMathOperationStackManipulation(t *testing.T) {
	// swap: 10 3 swap - is 3 - 10.
	apply(t, `<ValueOperand>10</ValueOperand>
		<ValueOperand>3</ValueOperand>
		<Operator>swap</Operator>
		<Operator>-</Operator>`, 0, -7)

	// drop: 1 2 drop leaves 1.
	apply(t, `<ValueOperand>1</ValueOperand>
		<ValueOperand>2</ValueOperand>
		<Operator>drop</Operator>`, 0, 1)

	// dup: 5 dup * squares.
	apply(t, `<ValueOperand>5</ValueOperand>
		<Operator>dup</Operator>
		<Operator>*</Operator>`, 0, 25)

	// over copies the second item, not the top: 7 2 over is 7 2 7, so
	// subtracting twice gives 7 - (2 - 7).
	apply(t, `<ValueOperand>7</ValueOperand>
		<ValueOperand>2</ValueOperand>
		<Operator>over</Operator>
		<Operator>-</Operator>
		<Operator>-</Operator>`, 0, 12)
}

// over is the one operator whose prose in the schema contradicts its stack
// notation: the description repeats dup's. If over were implemented as dup,
// this expression would leave a different value.
func TestMathOperationOverIsNotDup(t *testing.T) {
	overResult, err := decodeMath(t, `<ValueOperand>7</ValueOperand>
		<ValueOperand>2</ValueOperand>
		<Operator>over</Operator>
		<Operator>drop</Operator>
		<Operator>drop</Operator>`).Apply(0)
	if err != nil {
		t.Fatalf("over: %v", err)
	}
	// 7 2 over -> 7 2 7; two drops leave 7.
	if overResult != 7 {
		t.Errorf("over left %v, want 7 — it copied the top rather than the second item", overResult)
	}
}

func TestMathOperationBitwise(t *testing.T) {
	for _, tc := range []struct {
		expr string
		want float64
	}{
		{`<ValueOperand>12</ValueOperand><ValueOperand>10</ValueOperand><Operator>&amp;</Operator>`, 8},
		{`<ValueOperand>12</ValueOperand><ValueOperand>10</ValueOperand><Operator>|</Operator>`, 14},
		{`<ValueOperand>12</ValueOperand><ValueOperand>10</ValueOperand><Operator>xor</Operator>`, 6},
		{`<ValueOperand>1</ValueOperand><ValueOperand>4</ValueOperand><Operator>&lt;&lt;</Operator>`, 16},
		{`<ValueOperand>16</ValueOperand><ValueOperand>4</ValueOperand><Operator>&gt;&gt;</Operator>`, 1},
		// The schema calls both shifts signed, so the sign bit is kept.
		{`<ValueOperand>-16</ValueOperand><ValueOperand>2</ValueOperand><Operator>&gt;&gt;</Operator>`, -4},
	} {
		apply(t, tc.expr, 0, tc.want)
	}
}

// A bitwise operator on a fraction has no meaning, so it fails rather than
// rounding to something plausible.
func TestMathOperationBitwiseRejectsFractions(t *testing.T) {
	_, err := decodeMath(t, `<ValueOperand>1.5</ValueOperand>
		<ValueOperand>2</ValueOperand>
		<Operator>&amp;</Operator>`).Apply(0)
	if !errors.Is(err, xtce.ErrInvalidMathOperation) {
		t.Errorf("err = %v, want ErrInvalidMathOperation", err)
	}
}

func TestMathOperationLogical(t *testing.T) {
	for _, tc := range []struct {
		expr string
		want float64
	}{
		{`<ValueOperand>1</ValueOperand><ValueOperand>0</ValueOperand><Operator>&amp;&amp;</Operator>`, 0},
		{`<ValueOperand>1</ValueOperand><ValueOperand>0</ValueOperand><Operator>||</Operator>`, 1},
		{`<ValueOperand>0</ValueOperand><Operator>!</Operator>`, 1},
		{`<ValueOperand>5</ValueOperand><Operator>!</Operator>`, 0},
	} {
		apply(t, tc.expr, 0, tc.want)
	}
}

// An expression that does not balance must fail. Taking whatever is left on
// the stack would put a number in front of an operator that the database did
// not ask for.
func TestMathOperationRejectsUnbalancedExpressions(t *testing.T) {
	for name, inner := range map[string]string{
		"two values left":  `<ValueOperand>1</ValueOperand><ValueOperand>2</ValueOperand>`,
		"nothing left":     `<ValueOperand>1</ValueOperand><Operator>drop</Operator>`,
		"empty expression": ``,
		"operator starved": `<ValueOperand>1</ValueOperand><Operator>+</Operator>`,
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := decodeMath(t, inner).Apply(0); !errors.Is(err, xtce.ErrInvalidMathOperation) {
				t.Errorf("err = %v, want ErrInvalidMathOperation", err)
			}
		})
	}
}

// The domains the schema calls out as undefined are refused rather than
// returning a NaN or an infinity that would read as a real measurement.
func TestMathOperationRejectsUndefinedResults(t *testing.T) {
	for name, inner := range map[string]string{
		"division by zero":     `<ValueOperand>1</ValueOperand><ValueOperand>0</ValueOperand><Operator>/</Operator>`,
		"modulo by zero":       `<ValueOperand>1</ValueOperand><ValueOperand>0</ValueOperand><Operator>%</Operator>`,
		"ln of zero":           `<ValueOperand>0</ValueOperand><Operator>ln</Operator>`,
		"ln of negative":       `<ValueOperand>-1</ValueOperand><Operator>ln</Operator>`,
		"log of negative":      `<ValueOperand>-1</ValueOperand><Operator>log</Operator>`,
		"inverse of zero":      `<ValueOperand>0</ValueOperand><Operator>1/x</Operator>`,
		"asin out of range":    `<ValueOperand>2</ValueOperand><Operator>asin</Operator>`,
		"acosh below one":      `<ValueOperand>0</ValueOperand><Operator>acosh</Operator>`,
		"negative factorial":   `<ValueOperand>-1</ValueOperand><Operator>x!</Operator>`,
		"fractional factorial": `<ValueOperand>2.5</ValueOperand><Operator>x!</Operator>`,
		"factorial overflow":   `<ValueOperand>1000</ValueOperand><Operator>x!</Operator>`,
	} {
		t.Run(name, func(t *testing.T) {
			got, err := decodeMath(t, inner).Apply(0)
			if err == nil {
				t.Fatalf("no error, got %v", got)
			}
			if !errors.Is(err, xtce.ErrInvalidMathOperation) {
				t.Errorf("err = %v, want ErrInvalidMathOperation", err)
			}
		})
	}
}

// Two operators are defined self-contradictorily in the schema. Guessing
// which reading was meant would change every value they produce, so they are
// refused by name.
func TestMathOperationRefusesContradictoryOperators(t *testing.T) {
	for _, operator := range []string{"~", "div"} {
		inner := `<ValueOperand>1</ValueOperand><Operator>` + operator + `</Operator>`
		_, err := decodeMath(t, inner).Apply(0)
		if !errors.Is(err, xtce.ErrUnsupportedCalibrator) {
			t.Errorf("operator %q: err = %v, want ErrUnsupportedCalibrator", operator, err)
		}
	}
}

func TestMathOperationRejectsUnknownOperator(t *testing.T) {
	inner := `<ValueOperand>1</ValueOperand><Operator>frobnicate</Operator>`
	if _, err := decodeMath(t, inner).Apply(0); !errors.Is(err, xtce.ErrInvalidMathOperation) {
		t.Errorf("err = %v, want ErrInvalidMathOperation", err)
	}
}

// A reference to another parameter cannot be answered by a calibrator alone,
// so Apply says so instead of treating the value as zero.
func TestMathOperationNeedsAValueSourceForOtherParameters(t *testing.T) {
	inner := `<ThisParameterOperand/>
		<ParameterInstanceRefOperand parameterRef="/Sat/Bus/Temperature"/>
		<Operator>+</Operator>`

	_, err := decodeMath(t, inner).Apply(1)
	if !errors.Is(err, xtce.ErrUnsupportedCalibrator) {
		t.Fatalf("err = %v, want ErrUnsupportedCalibrator", err)
	}
	if !strings.Contains(err.Error(), "/Sat/Bus/Temperature") {
		t.Errorf("the error does not name the parameter: %v", err)
	}
}

func TestMathOperationApplyWithSuppliesOtherParameters(t *testing.T) {
	inner := `<ThisParameterOperand/>
		<ParameterInstanceRefOperand parameterRef="/Sat/Bus/Offset"/>
		<Operator>+</Operator>`

	got, err := decodeMath(t, inner).ApplyWith(10, func(ref xtce.ParameterInstanceRef) (float64, bool) {
		if ref.ParameterRef == "/Sat/Bus/Offset" {
			return 5, true
		}
		return 0, false
	})
	if err != nil {
		t.Fatalf("ApplyWith: %v", err)
	}
	if got != 15 {
		t.Errorf("ApplyWith = %v, want 15", got)
	}
}

// A lookup that cannot answer fails the calibration rather than substituting
// a zero, which would read as a real measurement.
func TestMathOperationApplyWithMissingValue(t *testing.T) {
	inner := `<ThisParameterOperand/>
		<ParameterInstanceRefOperand parameterRef="/Sat/Bus/Offset"/>
		<Operator>+</Operator>`

	_, err := decodeMath(t, inner).ApplyWith(10, func(xtce.ParameterInstanceRef) (float64, bool) {
		return 0, false
	})
	if !errors.Is(err, xtce.ErrUnsupportedCalibrator) {
		t.Errorf("err = %v, want ErrUnsupportedCalibrator", err)
	}
}

// The schema's defaults for a parameter instance reference: the current
// occurrence, and the calibrated value.
func TestParameterInstanceRefDefaults(t *testing.T) {
	calibrator := decodeMath(t, `<ParameterInstanceRefOperand parameterRef="/S/P"/>`)
	ref := calibrator.Operands[0].Parameter

	if ref.Instance != 0 {
		t.Errorf("Instance = %d, want 0", ref.Instance)
	}
	if !ref.Calibrated() {
		t.Error("Calibrated() = false, want true — the schema defaults useCalibratedValue to true")
	}

	explicit := decodeMath(t,
		`<ParameterInstanceRefOperand parameterRef="/S/P" useCalibratedValue="false" instance="-1"/>`)
	ref = explicit.Operands[0].Parameter

	if ref.Calibrated() {
		t.Error("Calibrated() = true, want false when the attribute says so")
	}
	if ref.Instance != -1 {
		t.Errorf("Instance = %d, want -1", ref.Instance)
	}
}

// A ValueOperand that is not a number is a broken database, not a zero.
func TestMathOperationRejectsNonNumericValueOperand(t *testing.T) {
	var calibrator xtce.MathOperationCalibrator
	err := xml.Unmarshal([]byte(mathXML(`<ValueOperand>seven</ValueOperand>`)), &calibrator)
	if !errors.Is(err, xtce.ErrInvalidMathOperation) {
		t.Errorf("err = %v, want ErrInvalidMathOperation", err)
	}
}

// Calibrate routes the third calibrator form to the evaluator, so a caller
// using the ordinary entry point gets the calibrated value.
func TestCalibrateAppliesAMathOperation(t *testing.T) {
	var calibrator xtce.Calibrator
	doc := `<Calibrator xmlns="http://www.omg.org/spec/XTCE/20180204">
			<MathOperationCalibrator>
				<ThisParameterOperand/>
				<ValueOperand>100</ValueOperand>
				<Operator>*</Operator>
			</MathOperationCalibrator>
		</Calibrator>`
	if err := xml.Unmarshal([]byte(doc), &calibrator); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if kind := calibrator.Kind(); kind != "math operation" {
		t.Errorf("Kind() = %q, want %q", kind, "math operation")
	}

	got, err := calibrator.Calibrate(0.25)
	if err != nil {
		t.Fatalf("Calibrate: %v", err)
	}
	if got != 25 {
		t.Errorf("Calibrate(0.25) = %v, want 25", got)
	}
}

// schemaOperators is every value of MathOperatorsType in XTCE 1.2, taken from
// the schema at https://www.omg.org/spec/XTCE/20180204/SpaceSystem.xsd.
//
// It is spelled out so that an operator the schema defines and this package
// forgot cannot pass unnoticed: a missing one reports "not in the schema's
// MathOperatorsType", which is the one answer this test refuses.
var schemaOperators = []string{
	"+", "-", "*", "/", "%", "^", "y^x",
	"ln", "log", "e^x", "1/x", "x!",
	"tan", "cos", "sin", "atan", "atan2", "acos", "asin",
	"tanh", "cosh", "sinh", "atanh", "acosh", "asinh",
	"swap", "drop", "dup", "over",
	"<<", ">>", "&", "|", "&&", "||", "!",
	"abs", "div", "int",
	">", ">=", "<", "<=", "==", "!=",
	"min", "max", "xor", "~",
}

// escapeXML spells an operator so it can sit in element content.
func escapeXML(symbol string) string {
	symbol = strings.ReplaceAll(symbol, "&", "&amp;")
	symbol = strings.ReplaceAll(symbol, "<", "&lt;")
	return strings.ReplaceAll(symbol, ">", "&gt;")
}

// Every operator the schema defines is accounted for: either it evaluates, or
// it is refused by name as unsupported. What must not happen is an operator
// falling through to "not in the schema's set", which would mean this package
// had simply missed one.
func TestMathOperationCoversEverySchemaOperator(t *testing.T) {
	if len(schemaOperators) != 49 {
		t.Fatalf("the operator list holds %d entries, want the schema's 49", len(schemaOperators))
	}

	// Three operands are enough for every arity in the set, and the values
	// are chosen to be inside every operator's domain.
	const operands = `<ValueOperand>1</ValueOperand>
		<ValueOperand>1</ValueOperand>
		<ValueOperand>1</ValueOperand>`

	evaluated, refused := 0, 0

	for _, symbol := range schemaOperators {
		inner := operands + `<Operator>` + escapeXML(symbol) + `</Operator>`

		var calibrator xtce.MathOperationCalibrator
		if err := xml.Unmarshal([]byte(mathXML(inner)), &calibrator); err != nil {
			t.Fatalf("operator %q: unmarshal: %v", symbol, err)
		}

		_, err := calibrator.Apply(0)
		switch {
		case err == nil:
			// It ran. Whether the stack balanced is a separate question, so
			// an unbalanced-stack error is fine here too.
			evaluated++
		case errors.Is(err, xtce.ErrUnsupportedCalibrator):
			refused++
		case errors.Is(err, xtce.ErrInvalidMathOperation):
			if strings.Contains(err.Error(), "not in the schema's MathOperatorsType") {
				t.Errorf("operator %q is defined by the schema but this package does not know it", symbol)
				continue
			}
			// An arity or stack complaint means the operator is implemented.
			evaluated++
		default:
			t.Errorf("operator %q gave an unexpected error: %v", symbol, err)
		}
	}

	if refused != 2 {
		t.Errorf("%d operators were refused, want 2 (~ and div, whose schema definitions contradict themselves)", refused)
	}
	if evaluated != len(schemaOperators)-2 {
		t.Errorf("%d operators evaluated, want %d", evaluated, len(schemaOperators)-2)
	}
}
