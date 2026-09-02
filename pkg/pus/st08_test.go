package pus_test

import (
	"bytes"
	"errors"
	"testing"

	"github.com/ravisuhag/astro/pkg/pus"
)

// TestPerformFunctionWithoutArgumentsIsJustTheName checks the case figure 8-87
// marks optional: a function that takes no arguments carries no count field at
// all, not a count of zero.
func TestPerformFunctionWithoutArgumentsIsJustTheName(t *testing.T) {
	p := pus.DefaultProfile()
	request := pus.PerformFunctionRequest{Profile: p, FunctionID: "DEPLOY"}

	encoded, err := request.Encode()
	if err != nil {
		t.Fatal(err)
	}
	if len(encoded) != p.FunctionIDSize() {
		t.Fatalf("encoded %d octets, want exactly the %d-octet function ID",
			len(encoded), p.FunctionIDSize())
	}
	// The name is padded with NUL out to the fixed width.
	want := []byte("DEPLOY\x00\x00")
	if !bytes.Equal(encoded, want) {
		t.Errorf("encoded %q, want %q", encoded, want)
	}

	got, err := pus.DecodePerformFunctionRequest(p, encoded)
	if err != nil {
		t.Fatal(err)
	}
	if got.FunctionID != "DEPLOY" {
		t.Errorf("function ID = %q, want %q; the NUL padding is not part of the name", got.FunctionID, "DEPLOY")
	}
	if got.Arguments != nil {
		t.Errorf("arguments = %+v, want nil for a body that is exactly the function ID", got.Arguments)
	}
}

// TestPerformFunctionCountZeroIsNotTheSameAsAbsent guards the distinction the
// optional group creates: N present and zero is one octet longer than N absent,
// and the two must not collapse into each other.
func TestPerformFunctionCountZeroIsNotTheSameAsAbsent(t *testing.T) {
	p := pus.DefaultProfile()

	absent := pus.PerformFunctionRequest{Profile: p, FunctionID: "F"}
	present := pus.PerformFunctionRequest{
		Profile:    p,
		FunctionID: "F",
		Arguments:  &pus.FunctionArguments{Count: 0},
	}

	a, err := absent.Encode()
	if err != nil {
		t.Fatal(err)
	}
	b, err := present.Encode()
	if err != nil {
		t.Fatal(err)
	}
	if len(b) != len(a)+p.FunctionArgumentCountSize() {
		t.Fatalf("count-present body is %d octets and count-absent is %d; want a %d-octet difference",
			len(b), len(a), p.FunctionArgumentCountSize())
	}

	decodedAbsent, err := pus.DecodePerformFunctionRequest(p, a)
	if err != nil {
		t.Fatal(err)
	}
	if decodedAbsent.Arguments != nil {
		t.Error("absent group decoded to a present one")
	}

	decodedPresent, err := pus.DecodePerformFunctionRequest(p, b)
	if err != nil {
		t.Fatal(err)
	}
	if decodedPresent.Arguments == nil {
		t.Fatal("present group decoded to nil")
	}
	if decodedPresent.Arguments.Count != 0 {
		t.Errorf("count = %d, want 0", decodedPresent.Arguments.Count)
	}
}

// TestPerformFunctionRoundTripsArguments checks the argument block travels
// verbatim, which is what "deduced" argument values force.
func TestPerformFunctionRoundTripsArguments(t *testing.T) {
	p := pus.DefaultProfile()
	// Two arguments: ID 1 with a two-octet value, ID 7 with a one-octet value.
	raw := []byte{0x01, 0xBE, 0xEF, 0x07, 0x2A}

	request := pus.PerformFunctionRequest{
		Profile:    p,
		FunctionID: "SETMODE",
		Arguments:  &pus.FunctionArguments{Count: 2, Raw: raw},
	}
	encoded, err := request.Encode()
	if err != nil {
		t.Fatal(err)
	}

	got, err := pus.DecodePerformFunctionRequest(p, encoded)
	if err != nil {
		t.Fatal(err)
	}
	if got.FunctionID != "SETMODE" {
		t.Errorf("function ID = %q, want SETMODE", got.FunctionID)
	}
	if got.Arguments == nil {
		t.Fatal("arguments = nil")
	}
	if got.Arguments.Count != 2 {
		t.Errorf("count = %d, want 2", got.Arguments.Count)
	}
	if !bytes.Equal(got.Arguments.Raw, raw) {
		t.Errorf("raw = % x, want % x", got.Arguments.Raw, raw)
	}
}

// TestPerformFunctionRefusesAnOverlongName checks a name wider than the fixed
// field is refused rather than truncated: a truncated name can name a
// different function.
func TestPerformFunctionRefusesAnOverlongName(t *testing.T) {
	p := pus.DefaultProfile()
	p.FunctionIDBytes = 4

	request := pus.PerformFunctionRequest{Profile: p, FunctionID: "TOOLONG"}
	if _, err := request.Encode(); !errors.Is(err, pus.ErrValueTooLarge) {
		t.Errorf("err = %v, want ErrValueTooLarge", err)
	}
}

// TestPerformFunctionRefusesAShortBody checks a body that cannot hold the
// function ID is refused.
func TestPerformFunctionRefusesAShortBody(t *testing.T) {
	p := pus.DefaultProfile()
	short := make([]byte, p.FunctionIDSize()-1)
	if _, err := pus.DecodePerformFunctionRequest(p, short); !errors.Is(err, pus.ErrDataTooShort) {
		t.Errorf("err = %v, want ErrDataTooShort", err)
	}
}

// TestSplitArgumentsUsesTheMissionDeclaration checks the split against a
// width function, which is what clause 6.8.3.1b makes it.
func TestSplitArgumentsUsesTheMissionDeclaration(t *testing.T) {
	p := pus.DefaultProfile()
	args := &pus.FunctionArguments{
		Count: 2,
		Raw:   []byte{0x01, 0xBE, 0xEF, 0x07, 0x2A},
	}
	widths := map[uint64]int{1: 2, 7: 1}
	width := func(id uint64) (int, error) {
		w, ok := widths[id]
		if !ok {
			return 0, pus.ErrInvalidProfile
		}
		return w, nil
	}

	got, err := args.SplitArguments(p, width)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 {
		t.Fatalf("got %d arguments, want 2", len(got))
	}
	if got[0].ID != 1 || !bytes.Equal(got[0].Value, []byte{0xBE, 0xEF}) {
		t.Errorf("argument 0 = %+v, want ID 1 value BEEF", got[0])
	}
	if got[1].ID != 7 || !bytes.Equal(got[1].Value, []byte{0x2A}) {
		t.Errorf("argument 1 = %+v, want ID 7 value 2A", got[1])
	}
}

// TestSplitArgumentsRefusesACountThatDisagrees checks the count is treated as a
// claim to verify rather than a length to trust. The two ends disagreeing about
// the declaration is exactly the failure this catches.
func TestSplitArgumentsRefusesACountThatDisagrees(t *testing.T) {
	p := pus.DefaultProfile()
	width := func(uint64) (int, error) { return 1, nil }

	// Three arguments' worth of octets, but the count claims two.
	args := &pus.FunctionArguments{Count: 2, Raw: []byte{1, 10, 2, 20, 3, 30}}
	if _, err := args.SplitArguments(p, width); !errors.Is(err, pus.ErrTrailingBytes) {
		t.Errorf("err = %v, want ErrTrailingBytes", err)
	}

	// A block that ends mid-argument.
	args = &pus.FunctionArguments{Count: 2, Raw: []byte{1, 10, 2}}
	if _, err := args.SplitArguments(p, width); !errors.Is(err, pus.ErrDataTooShort) {
		t.Errorf("err = %v, want ErrDataTooShort", err)
	}
}

// TestSplitArgumentsRefusesAHugeCountWithoutAllocating checks a hostile count
// cannot drive an allocation: the octets bound the work, not the count.
func TestSplitArgumentsRefusesAHugeCountWithoutAllocating(t *testing.T) {
	p := pus.DefaultProfile()
	width := func(uint64) (int, error) { return 1, nil }

	args := &pus.FunctionArguments{Count: 1 << 60, Raw: []byte{1, 10}}
	if _, err := args.SplitArguments(p, width); !errors.Is(err, pus.ErrTrailingBytes) {
		t.Errorf("err = %v, want ErrTrailingBytes", err)
	}
}

// TestSplitArgumentsOnAnAbsentGroup checks the nil receiver, which is the
// shape a no-argument function decodes to.
func TestSplitArgumentsOnAnAbsentGroup(t *testing.T) {
	var args *pus.FunctionArguments
	got, err := args.SplitArguments(pus.DefaultProfile(), func(uint64) (int, error) { return 1, nil })
	if err != nil {
		t.Fatal(err)
	}
	if got != nil {
		t.Errorf("got %+v, want nil", got)
	}
}

// TestST08IsInTheDefaultRegistry checks the registration wiring.
func TestST08IsInTheDefaultRegistry(t *testing.T) {
	p := pus.DefaultProfile()
	r, err := pus.NewDefaultRegistry(p)
	if err != nil {
		t.Fatal(err)
	}

	encoded, err := pus.PerformFunctionRequest{
		Profile:    p,
		FunctionID: "PING",
		Arguments:  &pus.FunctionArguments{Count: 1, Raw: []byte{0x03, 0xFF}},
	}.Encode()
	if err != nil {
		t.Fatal(err)
	}

	got, err := r.DecodeRequest(pus.MessageKey{
		Service: pus.ServiceFunctionManagement,
		Subtype: pus.SubtypePerformFunction,
	}, encoded)
	if err != nil {
		t.Fatal(err)
	}
	request, ok := got.(*pus.PerformFunctionRequest)
	if !ok {
		t.Fatalf("decoded to %T, want *pus.PerformFunctionRequest", got)
	}
	if request.FunctionID != "PING" {
		t.Errorf("function ID = %q, want PING", request.FunctionID)
	}
}
