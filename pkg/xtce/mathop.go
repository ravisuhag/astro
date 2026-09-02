package xtce

import (
	"encoding/xml"
	"fmt"
	"math"
	"strconv"
)

// MathOperationCalibrator: the third calibrator form, a postfix expression.
//
// The other two calibrators are arithmetic the schema states outright — a
// polynomial's terms, a spline's points. This one is a program. The schema's
// MathOperationCalibratorType holds a run of operands and operators in
// Reverse Polish order: operands are pushed as they appear, and each operator
// pops what it needs and pushes its result. "4 8 /" leaves 0.5. Postfix is
// used so the format never has to spell parentheses.
//
// The operator set and each operator's effect on the stack come from the
// schema's MathOperatorsType, which documents every one in the form
// "(before -- after)". Those notations are quoted against each case below,
// because several are not what the operator's name would suggest: atan2 takes
// its arguments in the opposite order to the stack, and y^x raises the second
// operand to the first.

// MathOperandKind says which of the four things an operand slot holds.
type MathOperandKind int

const (
	// MathValue is a ValueOperand: a constant written in the database.
	MathValue MathOperandKind = iota
	// MathThisParameter is a ThisParameterOperand: the value being
	// calibrated. The element has no content.
	MathThisParameter
	// MathParameterInstance is a ParameterInstanceRefOperand: the last known
	// value of some other parameter, which the calibrator cannot know on its
	// own.
	MathParameterInstance
	// MathOperator is an Operator.
	MathOperator
)

// String names the kind.
func (k MathOperandKind) String() string {
	switch k {
	case MathValue:
		return "value"
	case MathThisParameter:
		return "this parameter"
	case MathParameterInstance:
		return "parameter instance"
	case MathOperator:
		return "operator"
	default:
		return "unknown"
	}
}

// MathOperand is one step of a postfix expression.
type MathOperand struct {
	Kind MathOperandKind

	// Value is the constant, when Kind is MathValue.
	Value float64
	// Parameter names the parameter, when Kind is MathParameterInstance.
	Parameter ParameterInstanceRef
	// Operator is the operator's symbol, when Kind is MathOperator.
	Operator string
}

// ParameterInstanceRef is the schema's ParameterInstanceRefType: a reference
// to a parameter's value, possibly in an earlier or later packet.
type ParameterInstanceRef struct {
	// ParameterRef names the parameter, as a path across the space system
	// tree.
	ParameterRef string `xml:"parameterRef,attr"`
	// Instance selects an occurrence other than this one: positive is forward
	// in time, negative is backward, and 0 means the current value. The
	// schema's default is 0.
	Instance int64 `xml:"instance,attr"`
	// UseCalibratedValue reads the engineering value rather than the raw one.
	// The schema's default is true, so this is a pointer: false has to be
	// distinguishable from absent.
	UseCalibratedValue *bool `xml:"useCalibratedValue,attr"`
}

// Calibrated reports whether the reference is to the engineering value,
// applying the schema's default of true.
func (p *ParameterInstanceRef) Calibrated() bool {
	return p.UseCalibratedValue == nil || *p.UseCalibratedValue
}

// MathOperationCalibrator is a postfix expression that turns a raw value into
// an engineering one.
type MathOperationCalibrator struct {
	// Operands is the expression, in the order the document wrote it.
	Operands []MathOperand
}

// UnmarshalXML reads the operand run in document order.
//
// The schema makes the four operand elements an unbounded choice, so their
// order carries the whole meaning of the expression. Go's struct-field
// unmarshalling would sort them into four separate slices and lose that, so
// the tokens are walked by hand instead.
func (m *MathOperationCalibrator) UnmarshalXML(decoder *xml.Decoder, start xml.StartElement) error {
	for {
		token, err := decoder.Token()
		if err != nil {
			return err
		}

		switch element := token.(type) {
		case xml.StartElement:
			operand, err := decodeMathOperand(decoder, element)
			if err != nil {
				return err
			}
			m.Operands = append(m.Operands, operand)

		case xml.EndElement:
			if element.Name == start.Name {
				return nil
			}
		}
	}
}

// decodeMathOperand reads one operand element.
func decodeMathOperand(decoder *xml.Decoder, element xml.StartElement) (MathOperand, error) {
	switch element.Name.Local {
	case "ValueOperand":
		var text string
		if err := decoder.DecodeElement(&text, &element); err != nil {
			return MathOperand{}, err
		}
		value, err := strconv.ParseFloat(text, 64)
		if err != nil {
			return MathOperand{}, fmt.Errorf("%w: ValueOperand %q is not a number",
				ErrInvalidMathOperation, text)
		}
		return MathOperand{Kind: MathValue, Value: value}, nil

	case "ThisParameterOperand":
		// The schema fixes this element's content to the empty string, so
		// there is nothing to read but the end tag.
		if err := decoder.Skip(); err != nil {
			return MathOperand{}, err
		}
		return MathOperand{Kind: MathThisParameter}, nil

	case "ParameterInstanceRefOperand":
		var ref ParameterInstanceRef
		if err := decoder.DecodeElement(&ref, &element); err != nil {
			return MathOperand{}, err
		}
		return MathOperand{Kind: MathParameterInstance, Parameter: ref}, nil

	case "Operator":
		var symbol string
		if err := decoder.DecodeElement(&symbol, &element); err != nil {
			return MathOperand{}, err
		}
		return MathOperand{Kind: MathOperator, Operator: symbol}, nil

	default:
		// BaseCalibratorType contributes nothing this evaluator needs, and an
		// unknown element is not this package's business to reject.
		if err := decoder.Skip(); err != nil {
			return MathOperand{}, err
		}
		return MathOperand{}, fmt.Errorf("%w: unexpected element %q in a MathOperationCalibrator",
			ErrInvalidMathOperation, element.Name.Local)
	}
}

// ParameterValueFunc reports the current value of another parameter.
//
// A math operation may reference one, and a calibrator has no way to know it:
// the value lives in whatever is tracking the stream. Returning false means
// the value is not available, which fails the calibration rather than
// substituting a zero.
type ParameterValueFunc func(ref ParameterInstanceRef) (float64, bool)

// Apply evaluates the expression with raw as the value being calibrated.
//
// It fails if the expression references another parameter, because nothing
// here can supply one. Use ApplyWith for that.
func (m *MathOperationCalibrator) Apply(raw float64) (float64, error) {
	return m.ApplyWith(raw, nil)
}

// ApplyWith evaluates the expression, using lookup for any reference to
// another parameter.
func (m *MathOperationCalibrator) ApplyWith(raw float64, lookup ParameterValueFunc) (float64, error) {
	stack := make([]float64, 0, len(m.Operands))

	for i, operand := range m.Operands {
		switch operand.Kind {
		case MathValue:
			stack = append(stack, operand.Value)

		case MathThisParameter:
			stack = append(stack, raw)

		case MathParameterInstance:
			if lookup == nil {
				return 0, fmt.Errorf("%w: the expression reads parameter %q, which needs a value source",
					ErrUnsupportedCalibrator, operand.Parameter.ParameterRef)
			}
			value, ok := lookup(operand.Parameter)
			if !ok {
				return 0, fmt.Errorf("%w: no value available for parameter %q",
					ErrUnsupportedCalibrator, operand.Parameter.ParameterRef)
			}
			stack = append(stack, value)

		case MathOperator:
			next, err := applyMathOperator(operand.Operator, stack)
			if err != nil {
				return 0, fmt.Errorf("%w (operand %d)", err, i+1)
			}
			stack = next

		default:
			return 0, fmt.Errorf("%w: operand %d has no kind", ErrInvalidMathOperation, i+1)
		}
	}

	// A well-formed expression leaves exactly its result. Anything else means
	// the operand run does not balance, and picking a value out of what is
	// left would put a wrong number in front of an operator.
	switch len(stack) {
	case 1:
		return stack[0], nil
	case 0:
		return 0, fmt.Errorf("%w: the expression left nothing on the stack", ErrInvalidMathOperation)
	default:
		return 0, fmt.Errorf("%w: the expression left %d values on the stack, want 1",
			ErrInvalidMathOperation, len(stack))
	}
}

// applyMathOperator pops what one operator needs and pushes its result.
//
// Every stack notation quoted here is the schema's own, from
// MathOperatorsType. Where the notation and the operator's prose disagree the
// notation is followed, and the disagreement is called out.
func applyMathOperator(symbol string, stack []float64) ([]float64, error) {
	// pop2 takes the top two as (a, b) where b was on top, which is the
	// notation's (x1 x2) with x2 uppermost.
	pop2 := func() (a, b float64, err error) {
		if len(stack) < 2 {
			return 0, 0, fmt.Errorf("%w: operator %q needs two operands, the stack holds %d",
				ErrInvalidMathOperation, symbol, len(stack))
		}
		return stack[len(stack)-2], stack[len(stack)-1], nil
	}
	pop1 := func() (x float64, err error) {
		if len(stack) < 1 {
			return 0, fmt.Errorf("%w: operator %q needs an operand, the stack is empty",
				ErrInvalidMathOperation, symbol)
		}
		return stack[len(stack)-1], nil
	}
	binary := func(result float64) []float64 {
		return append(stack[:len(stack)-2], result)
	}
	unary := func(result float64) []float64 {
		return append(stack[:len(stack)-1], result)
	}

	switch symbol {

	// --- arithmetic ---

	case "+": // (x1 x2 -- x1+x2)
		a, b, err := pop2()
		if err != nil {
			return nil, err
		}
		return binary(a + b), nil

	case "-": // (x1 x2 -- x1-x2)
		a, b, err := pop2()
		if err != nil {
			return nil, err
		}
		return binary(a - b), nil

	case "*": // (x1 x2 -- x1*x2)
		a, b, err := pop2()
		if err != nil {
			return nil, err
		}
		return binary(a * b), nil

	case "/": // (x1 x2 -- x1/x2), undefined if x2 is 0
		a, b, err := pop2()
		if err != nil {
			return nil, err
		}
		if b == 0 {
			return nil, fmt.Errorf("%w: division by zero", ErrInvalidMathOperation)
		}
		return binary(a / b), nil

	case "%":
		// (x1 x2 -- x3) "Divide x1 by x2, giving the modulo x3". The schema's
		// own appinfo says implementations should verify modulo versus
		// remainder behaviour, so it does not settle the sign for negative
		// operands. This is the remainder: the sign follows x1, as in C and
		// Go. A database relying on the other reading will disagree here.
		a, b, err := pop2()
		if err != nil {
			return nil, err
		}
		if b == 0 {
			return nil, fmt.Errorf("%w: modulo by zero", ErrInvalidMathOperation)
		}
		return binary(math.Mod(a, b)), nil

	case "^": // (x1 x2 -- x1**x2)
		a, b, err := pop2()
		if err != nil {
			return nil, err
		}
		return binary(math.Pow(a, b)), nil

	case "y^x": // (x1 x2 -- x2**x1) — reversed, per the notation
		a, b, err := pop2()
		if err != nil {
			return nil, err
		}
		return binary(math.Pow(b, a)), nil

	case "abs": // (x1 -- abs(x1))
		x, err := pop1()
		if err != nil {
			return nil, err
		}
		return unary(math.Abs(x)), nil

	case "int": // (x1 -- int(x1)) — the integer part, so toward zero
		x, err := pop1()
		if err != nil {
			return nil, err
		}
		return unary(math.Trunc(x)), nil

	case "min": // (x1 x2 -- min(x1, x2))
		a, b, err := pop2()
		if err != nil {
			return nil, err
		}
		return binary(math.Min(a, b)), nil

	case "max": // (x1 x2 -- max(x1, x2))
		a, b, err := pop2()
		if err != nil {
			return nil, err
		}
		return binary(math.Max(a, b)), nil

	case "x!": // (x -- x!), undefined if x is less than 0
		x, err := pop1()
		if err != nil {
			return nil, err
		}
		result, err := factorial(x)
		if err != nil {
			return nil, err
		}
		return unary(result), nil

	// --- logarithms and exponentials ---

	case "ln": // (x -- ln(x)), undefined for x <= 0
		x, err := pop1()
		if err != nil {
			return nil, err
		}
		if x <= 0 {
			return nil, fmt.Errorf("%w: ln of %v", ErrInvalidMathOperation, x)
		}
		return unary(math.Log(x)), nil

	case "log": // (x -- log(x)), base ten, undefined for x <= 0
		x, err := pop1()
		if err != nil {
			return nil, err
		}
		if x <= 0 {
			return nil, fmt.Errorf("%w: log of %v", ErrInvalidMathOperation, x)
		}
		return unary(math.Log10(x)), nil

	case "e^x": // (x -- exp(x))
		x, err := pop1()
		if err != nil {
			return nil, err
		}
		return unary(math.Exp(x)), nil

	case "1/x": // (x -- 1/x)
		x, err := pop1()
		if err != nil {
			return nil, err
		}
		if x == 0 {
			return nil, fmt.Errorf("%w: inversion of zero", ErrInvalidMathOperation)
		}
		return unary(1 / x), nil

	// --- trigonometry, in radians ---

	case "sin":
		return applyUnaryFunc(stack, symbol, math.Sin, nil)
	case "cos":
		return applyUnaryFunc(stack, symbol, math.Cos, nil)
	case "tan":
		return applyUnaryFunc(stack, symbol, math.Tan, nil)
	case "asin":
		return applyUnaryFunc(stack, symbol, math.Asin, inUnitInterval)
	case "acos":
		return applyUnaryFunc(stack, symbol, math.Acos, inUnitInterval)
	case "atan":
		return applyUnaryFunc(stack, symbol, math.Atan, nil)
	case "sinh":
		return applyUnaryFunc(stack, symbol, math.Sinh, nil)
	case "cosh":
		return applyUnaryFunc(stack, symbol, math.Cosh, nil)
	case "tanh":
		return applyUnaryFunc(stack, symbol, math.Tanh, nil)
	case "asinh":
		return applyUnaryFunc(stack, symbol, math.Asinh, nil)
	case "acosh":
		return applyUnaryFunc(stack, symbol, math.Acosh, atLeastOne)
	case "atanh":
		return applyUnaryFunc(stack, symbol, math.Atanh, insideUnitInterval)

	case "atan2":
		// (x1 x2 -- atan2(x2, x1)). The arguments are the other way round
		// from the stack: the value pushed last is atan2's first argument.
		a, b, err := pop2()
		if err != nil {
			return nil, err
		}
		return binary(math.Atan2(b, a)), nil

	// --- comparisons, which push 1 or 0 ---

	case "==": // (x1 x2 -- x1 == x2)
		a, b, err := pop2()
		if err != nil {
			return nil, err
		}
		return binary(boolValue(a == b)), nil

	case "!=":
		a, b, err := pop2()
		if err != nil {
			return nil, err
		}
		return binary(boolValue(a != b)), nil

	case "<":
		a, b, err := pop2()
		if err != nil {
			return nil, err
		}
		return binary(boolValue(a < b)), nil

	case "<=":
		a, b, err := pop2()
		if err != nil {
			return nil, err
		}
		return binary(boolValue(a <= b)), nil

	case ">":
		a, b, err := pop2()
		if err != nil {
			return nil, err
		}
		return binary(boolValue(a > b)), nil

	case ">=":
		a, b, err := pop2()
		if err != nil {
			return nil, err
		}
		return binary(boolValue(a >= b)), nil

	// --- logical ---

	case "&&": // (x1 x2 -- x1 && x2)
		a, b, err := pop2()
		if err != nil {
			return nil, err
		}
		return binary(boolValue(a != 0 && b != 0)), nil

	case "||": // (x1 x2 -- x1 || x2)
		a, b, err := pop2()
		if err != nil {
			return nil, err
		}
		return binary(boolValue(a != 0 || b != 0)), nil

	case "!":
		// The schema calls this "logical not" but prints the stack effect as
		// "(x1 x2 -- x1 ! x2)", two operands, which no negation has. The
		// notation is a copy of the binary template above it; the name and
		// description agree with each other and with every other language, so
		// it is applied to one operand.
		x, err := pop1()
		if err != nil {
			return nil, err
		}
		return unary(boolValue(x == 0)), nil

	// --- bitwise, on values that must be whole numbers ---

	case "<<": // (x1 x2 -- x1 << x2), signed
		return applyShift(stack, symbol, true)
	case ">>": // (x1 x2 -- x1 >> x2), signed
		return applyShift(stack, symbol, false)

	case "&": // (x1 x2 -- x1 & x2)
		return applyBitwise(stack, symbol, func(a, b int64) int64 { return a & b })
	case "|": // (x1 x2 -- x1 | x2)
		return applyBitwise(stack, symbol, func(a, b int64) int64 { return a | b })
	case "xor": // (x1 x2 -- x1 xor x2)
		return applyBitwise(stack, symbol, func(a, b int64) int64 { return a ^ b })

	// --- stack manipulation ---

	case "swap": // (x1 x2 -- x2 x1)
		if len(stack) < 2 {
			return nil, fmt.Errorf("%w: swap needs two operands, the stack holds %d",
				ErrInvalidMathOperation, len(stack))
		}
		top := len(stack) - 1
		stack[top], stack[top-1] = stack[top-1], stack[top]
		return stack, nil

	case "drop": // (x -- )
		if len(stack) < 1 {
			return nil, fmt.Errorf("%w: drop needs an operand, the stack is empty",
				ErrInvalidMathOperation)
		}
		return stack[:len(stack)-1], nil

	case "dup": // (x -- x x)
		x, err := pop1()
		if err != nil {
			return nil, err
		}
		return append(stack, x), nil

	case "over":
		// (x1 x2 -- x1 x2 x1): a copy of the second item goes on top. The
		// schema's prose says "duplicate top item on the stack", which is the
		// description of dup repeated by mistake; the notation is the one that
		// distinguishes over from dup, so it is what this follows.
		if len(stack) < 2 {
			return nil, fmt.Errorf("%w: over needs two operands, the stack holds %d",
				ErrInvalidMathOperation, len(stack))
		}
		return append(stack, stack[len(stack)-2]), nil

	// --- operators the schema does not pin down ---

	case "~":
		// Called "Bitwise not operation" with the effect "(x1 x2 -- x1 ~ x2)
		// The result of this can only be 0 or 1". Those cannot all hold: a
		// bitwise complement takes one operand and does not yield 0 or 1.
		// Guessing between bitwise and logical would change every value it
		// produces, so it is refused.
		return nil, fmt.Errorf("%w: operator %q, whose definition in the schema is self-contradictory",
			ErrUnsupportedCalibrator, symbol)

	case "div":
		// "Euclidean division quotient (x1 -- div(x1))". Euclidean division
		// needs a divisor, which the notation's single operand does not
		// provide. Refused for the same reason.
		return nil, fmt.Errorf("%w: operator %q, whose definition in the schema is self-contradictory",
			ErrUnsupportedCalibrator, symbol)

	default:
		return nil, fmt.Errorf("%w: operator %q is not in the schema's MathOperatorsType",
			ErrInvalidMathOperation, symbol)
	}
}

// applyUnaryFunc pops one operand, checks its domain, and pushes f of it.
func applyUnaryFunc(stack []float64, symbol string, f func(float64) float64, domain func(float64) bool) ([]float64, error) {
	if len(stack) < 1 {
		return nil, fmt.Errorf("%w: operator %q needs an operand, the stack is empty",
			ErrInvalidMathOperation, symbol)
	}
	x := stack[len(stack)-1]
	if domain != nil && !domain(x) {
		return nil, fmt.Errorf("%w: %s of %v is not a real number", ErrInvalidMathOperation, symbol, x)
	}
	return append(stack[:len(stack)-1], f(x)), nil
}

// inUnitInterval is the domain of asin and acos.
func inUnitInterval(x float64) bool { return x >= -1 && x <= 1 }

// insideUnitInterval is the domain of atanh, which is open at both ends.
func insideUnitInterval(x float64) bool { return x > -1 && x < 1 }

// atLeastOne is the domain of acosh.
func atLeastOne(x float64) bool { return x >= 1 }

// applyBitwise pops two operands as whole numbers and pushes f of them.
//
// The stack is floating point because most of the operators are, but a
// bitwise operator on a fraction has no meaning, so a non-integral operand is
// an error rather than something to round.
func applyBitwise(stack []float64, symbol string, f func(a, b int64) int64) ([]float64, error) {
	if len(stack) < 2 {
		return nil, fmt.Errorf("%w: operator %q needs two operands, the stack holds %d",
			ErrInvalidMathOperation, symbol, len(stack))
	}
	a, err := wholeNumber(stack[len(stack)-2], symbol)
	if err != nil {
		return nil, err
	}
	b, err := wholeNumber(stack[len(stack)-1], symbol)
	if err != nil {
		return nil, err
	}
	return append(stack[:len(stack)-2], float64(f(a, b))), nil
}

// applyShift is the same for the two shifts, whose right operand also has to
// be a sane shift distance.
func applyShift(stack []float64, symbol string, left bool) ([]float64, error) {
	if len(stack) < 2 {
		return nil, fmt.Errorf("%w: operator %q needs two operands, the stack holds %d",
			ErrInvalidMathOperation, symbol, len(stack))
	}
	value, err := wholeNumber(stack[len(stack)-2], symbol)
	if err != nil {
		return nil, err
	}
	distance, err := wholeNumber(stack[len(stack)-1], symbol)
	if err != nil {
		return nil, err
	}
	if distance < 0 || distance > 63 {
		return nil, fmt.Errorf("%w: operator %q by %d places", ErrInvalidMathOperation, symbol, distance)
	}

	// The schema calls both shifts signed, so the right shift keeps the sign
	// bit, which is what Go's >> on a signed integer does.
	var result int64
	if left {
		result = value << uint(distance)
	} else {
		result = value >> uint(distance)
	}
	return append(stack[:len(stack)-2], float64(result)), nil
}

// wholeNumber converts a stack value to an integer, refusing a fraction.
func wholeNumber(x float64, symbol string) (int64, error) {
	if math.IsNaN(x) || math.IsInf(x, 0) || x != math.Trunc(x) {
		return 0, fmt.Errorf("%w: operator %q needs whole numbers, got %v",
			ErrInvalidMathOperation, symbol, x)
	}
	if x > math.MaxInt64 || x < math.MinInt64 {
		return 0, fmt.Errorf("%w: operator %q operand %v does not fit a 64-bit integer",
			ErrInvalidMathOperation, symbol, x)
	}
	return int64(x), nil
}

// factorial computes x! for a whole, non-negative x.
//
// The schema says a negative x is undefined. A non-integer is refused too:
// the gamma function would extend it, but the schema says factorial, and
// answering a different function's value would be worse than failing.
func factorial(x float64) (float64, error) {
	if x < 0 || x != math.Trunc(x) {
		return 0, fmt.Errorf("%w: factorial of %v", ErrInvalidMathOperation, x)
	}

	result := 1.0
	for i := 2.0; i <= x; i++ {
		result *= i
		if math.IsInf(result, 0) {
			return 0, fmt.Errorf("%w: factorial of %v overflows", ErrInvalidMathOperation, x)
		}
	}
	return result, nil
}

// boolValue is how the comparison and logical operators put a truth value on
// a stack of numbers.
func boolValue(b bool) float64 {
	if b {
		return 1
	}
	return 0
}
