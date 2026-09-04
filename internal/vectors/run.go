package vectors

import (
	"bytes"
	"encoding/hex"
	"errors"
	"fmt"
	"testing"
)

// Impl is what a package supplies to run its own vectors. Every function
// is optional: a package that has no reject-at-construction vectors
// leaves Construct nil, and those vectors are then reported as skipped
// rather than silently passing.
type Impl struct {
	// EncodeFn builds the structure from fields and config, then encodes
	// it, returning the octets that go on the wire.
	EncodeFn func(fields, config Fields) ([]byte, error)

	// DecodeFn decodes octets and reports the fields it found. It returns
	// only the fields the vector asked about; anything else is ignored.
	DecodeFn func(input []byte, config Fields) (Fields, error)

	// ConstructFn builds the structure without encoding it. It exists so a
	// reject vector can assert that bad field values are refused at
	// construction, which is where most range rules live.
	ConstructFn func(fields, config Fields) error

	// MachineFn builds a state machine from a sequence vector's init and
	// config, ready to be driven step by step. A package with no state
	// machine leaves it nil, and its sequence vectors report as skipped.
	MachineFn func(init, config Fields) (Machine, error)

	// Caps names the vector capabilities this implementation has. An API
	// that always allocates has none, and vectors requiring
	// encode_into skip.
	Caps []string
}

// Machine is a state machine under test, driven one call at a time.
//
// A sequence vector is the only kind that can say anything about ordering,
// which is what a state machine is made of: the same call gives different
// answers depending on what came before. Encode and decode vectors cannot
// express that, because they have nowhere to put the "before".
type Machine interface {
	// Step performs one named call. It returns any octets the call puts on
	// the wire — nil when the call emits nothing — and the machine's state
	// afterwards, holding at least the variables the vector asks about.
	Step(call string, fields Fields) (out []byte, state Fields, err error)
}

// Run executes every vector in the file against impl. Each vector becomes
// a subtest named after it, so a failure names the vector and the file.
func Run(t *testing.T, f *File, impl Impl) {
	t.Helper()

	var ran, skipped int

	for _, v := range f.Encode {
		v := v
		t.Run("encode/"+v.Name, func(t *testing.T) {
			if !impl.supports(v.Requires) {
				t.Skipf("needs %v, which the Go implementation does not have", v.Requires)
			}
			if impl.EncodeFn == nil {
				t.Skip("package supplies no EncodeFn")
			}
			want, err := hex.DecodeString(v.Want)
			if err != nil {
				t.Fatalf("vector want is not hex: %v", err)
			}
			got, err := impl.EncodeFn(v.Fields, v.Config)
			if err != nil {
				t.Fatalf("encode failed: %v\n  %s", err, cite(v.Clause, v.Note))
			}
			if !bytes.Equal(got, want) {
				t.Fatalf("wire bytes differ\n  got  %x\n  want %x\n  %s",
					got, want, cite(v.Clause, v.Note))
			}
		})
		if impl.supports(v.Requires) && impl.EncodeFn != nil {
			ran++
		} else {
			skipped++
		}
	}

	for _, v := range f.Decode {
		v := v
		t.Run("decode/"+v.Name, func(t *testing.T) {
			if !impl.supports(v.Requires) {
				t.Skipf("needs %v, which the Go implementation does not have", v.Requires)
			}
			if impl.DecodeFn == nil {
				t.Skip("package supplies no DecodeFn")
			}
			input, err := hex.DecodeString(v.Input)
			if err != nil {
				t.Fatalf("vector input is not hex: %v", err)
			}
			got, err := impl.DecodeFn(input, v.Config)
			if err != nil {
				t.Fatalf("decode failed: %v\n  %s", err, cite(v.Clause, v.Note))
			}
			if bad := diff(v.Fields, got); len(bad) > 0 {
				t.Fatalf("decoded fields differ\n  %s\n  %s",
					joinLines(bad), cite(v.Clause, v.Note))
			}
		})
		if impl.supports(v.Requires) && impl.DecodeFn != nil {
			ran++
		} else {
			skipped++
		}
	}

	for _, v := range f.Reject {
		v := v
		t.Run("reject/"+v.Name, func(t *testing.T) {
			if !impl.supports(v.Requires) {
				t.Skipf("needs %v, which the Go implementation does not have", v.Requires)
			}
			switch {
			case v.Input != nil:
				if impl.DecodeFn == nil {
					t.Skip("package supplies no DecodeFn")
				}
				input, err := hex.DecodeString(*v.Input)
				if err != nil {
					t.Fatalf("vector input is not hex: %v", err)
				}
				if _, err := impl.DecodeFn(input, v.Config); err == nil {
					t.Fatalf("decode accepted octets the standard requires it to refuse (%s)\n  input %s\n  %s",
						v.Error, *v.Input, cite(v.Clause, v.Note))
				}
			default:
				if impl.ConstructFn == nil {
					t.Skip("package supplies no ConstructFn")
				}
				if err := impl.ConstructFn(v.Fields, v.Config); err == nil {
					t.Fatalf("construction accepted values the standard requires it to refuse (%s)\n  %s",
						v.Error, cite(v.Clause, v.Note))
				}
			}
		})
		ran++
	}

	for _, v := range f.Sequence {
		v := v
		t.Run("sequence/"+v.Name, func(t *testing.T) {
			if impl.MachineFn == nil {
				t.Skip("package supplies no MachineFn")
			}
			m, err := impl.MachineFn(v.Init, v.Config)
			if err != nil {
				t.Fatalf("building the machine: %v\n  %s", err, cite(v.Clause, v.Note))
			}

			for i, step := range v.Steps {
				// Name the step in every failure. A sequence fails at a
				// point, and which point is most of the diagnosis.
				where := fmt.Sprintf("step %d, %s", i+1, step.Call)

				out, state, err := m.Step(step.Call, step.Fields)

				if step.Error != "" {
					if err == nil {
						t.Fatalf("%s: the call was accepted, and the standard requires it to be refused (%s)\n  %s",
							where, step.Error, cite(v.Clause, v.Note))
					}
					continue
				}
				if err != nil {
					t.Fatalf("%s: %v\n  %s", where, err, cite(v.Clause, v.Note))
				}

				if step.Want != "" {
					want, err := hex.DecodeString(step.Want)
					if err != nil {
						t.Fatalf("%s: vector want is not hex: %v", where, err)
					}
					if !bytes.Equal(out, want) {
						t.Fatalf("%s: emitted %x, want %s\n  %s",
							where, out, step.Want, cite(v.Clause, v.Note))
					}
				}

				if len(step.WantState) > 0 {
					if bad := diff(step.WantState, state); len(bad) > 0 {
						t.Fatalf("%s: state differs\n  %s\n  %s",
							where, joinLines(bad), cite(v.Clause, v.Note))
					}
				}
			}
		})
		if impl.MachineFn != nil {
			ran++
		} else {
			skipped++
		}
	}

	t.Logf("%s: %d vectors run, %d skipped", f.Path, ran, skipped)
}

// RunFile loads a vector file and runs it. This is the one call a
// package's vectors_test.go needs.
func RunFile(t *testing.T, rel string, impl Impl) {
	t.Helper()
	f, err := Load(rel)
	if err != nil {
		t.Fatalf("loading vectors: %v", err)
	}
	Run(t, f, impl)
}

func (i Impl) supports(reqs []string) bool {
	for _, r := range reqs {
		if !hasCap(i.Caps, r) {
			return false
		}
	}
	return true
}

// cite renders the clause and derivation for a failure message. A test
// that fails should say what the bytes were supposed to be and why.
func cite(clause, note string) string {
	switch {
	case clause != "" && note != "":
		return "clause " + clause + ": " + note
	case note != "":
		return note
	case clause != "":
		return "clause " + clause
	default:
		return "(no clause cited — this vector is marked unverified)"
	}
}

func joinLines(ss []string) string {
	out := ""
	for i, s := range ss {
		if i > 0 {
			out += "\n  "
		}
		out += s
	}
	return out
}

// ErrIs is a helper for packages that want to assert a specific sentinel
// alongside a reject vector. The vocabulary deliberately does not map to
// sentinels centrally; this just saves repeating errors.Is at call sites.
func ErrIs(err error, targets ...error) bool {
	for _, t := range targets {
		if errors.Is(err, t) {
			return true
		}
	}
	return false
}
