package bpsec_test

import (
	"bytes"
	"errors"
	"strings"
	"testing"

	"github.com/ravisuhag/astro/pkg/bp"
	"github.com/ravisuhag/astro/pkg/bpsec"
)

// Every abstract security block RFC 9173 appendix A prints must decode to the
// same octets it came from. The appendix is the only published source for this
// structure, so a round trip over its four examples is the widest check there
// is.
func TestASBRoundTrip(t *testing.T) {
	tests := []struct {
		name string
		hex  string
	}{
		{
			name: "A.1 BIB, one target, two parameters",
			hex: "810101018202820201828201078203008181820158403bdc69b3a34a2b5d3a8554368bd1e808" +
				"f606219d2a10a846eae3886ae4ecc83c4ee550fdfb1cc636b904e2f1a73e303dcd4b6ccece003e95e8164dcc89a156e1",
		},
		{
			name: "A.2 BCB, four parameters including a wrapped key",
			hex: "8101020182028202018482014c5477656c766531323132313282020182035818" +
				"69c411276fecddc4780df42c8a2af89296fabf34d7fae7008204008181820150efa4b5ac0108e3816c5606479801bc04",
		},
		{
			name: "A.3 BIB, two targets and two result sets",
			hex: "8200020101820282030082820105820300828182015820cac6ce8e4c5dae57988b" +
				"757e49a6dd1431dc04763541b2845098265bc817241b81820158203ed614c0d97f49" +
				"b3633627779aa18a338d212bf3c92b97759d9739cd50725596",
		},
		{
			name: "A.4 BCB, two targets, security source ipn:2.1",
			hex: "820301020182028202018382014c5477656c7665313231323132820203820407" +
				"8281820150220ffc45c8a901999ecc60991dd78b2981820150d2c51cb2481792dae8b21d848cede99b",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			want := mustHex(t, tt.hex)

			asb, err := bpsec.DecodeASB(want)
			if err != nil {
				t.Fatalf("DecodeASB: %v", err)
			}
			got, err := asb.Encode()
			if err != nil {
				t.Fatalf("Encode: %v", err)
			}
			if !bytes.Equal(got, want) {
				t.Errorf("round trip =\n\t%x\nwant\n\t%x", got, want)
			}
			// A dump must not panic or return nothing useful.
			if asb.Humanize() == "" {
				t.Error("Humanize returned an empty string")
			}
		})
	}
}

// A security context this package does not implement still decodes. The
// abstract security block structure is common to every context
// (RFC 9172 clause 3.6), so only the meaning of the parameters is unknown.
func TestASBDecodesAnUnknownContext(t *testing.T) {
	asb := &bpsec.ASB{
		Targets:      []uint64{1},
		ContextID:    99,
		ContextFlags: bpsec.ContextFlagParametersPresent,
		Source:       bp.IPN(2, 1),
		Parameters:   []bpsec.Parameter{{ID: 7, Value: []byte{0x18, 0x2a}}},
		Results:      [][]bpsec.Result{{{ID: 4, Value: []byte{0x43, 0x01, 0x02, 0x03}}}},
	}
	encoded, err := asb.Encode()
	if err != nil {
		t.Fatalf("Encode: %v", err)
	}

	back, err := bpsec.DecodeASB(encoded)
	if err != nil {
		t.Fatalf("DecodeASB on an unknown context: %v", err)
	}
	if back.ContextID != 99 {
		t.Errorf("context id = %d, want 99", back.ContextID)
	}
	if raw, ok := back.Parameter(7); !ok || !bytes.Equal(raw, []byte{0x18, 0x2a}) {
		t.Errorf("parameter 7 = %x %v, want 182a true", raw, ok)
	}

	// Nothing will process it, though, and both context readers say so rather
	// than guessing at the parameter numbering.
	if _, err := bpsec.DecodeIntegrityParameters(back); !errors.Is(err, bpsec.ErrUnknownContext) {
		t.Errorf("DecodeIntegrityParameters = %v, want ErrUnknownContext", err)
	}
	if _, err := bpsec.DecodeConfidentialityParameters(back); !errors.Is(err, bpsec.ErrUnknownContext) {
		t.Errorf("DecodeConfidentialityParameters = %v, want ErrUnknownContext", err)
	}
}

func TestASBValidationRules(t *testing.T) {
	tests := []struct {
		name string
		asb  bpsec.ASB
		want error
	}{
		{
			name: "no targets",
			asb: bpsec.ASB{
				ContextID: bpsec.ContextBIBHMACSHA2,
				Source:    bp.IPN(2, 1),
			},
			want: bpsec.ErrNoTargets,
		},
		{
			name: "the same target twice",
			asb: bpsec.ASB{
				Targets:   []uint64{1, 1},
				ContextID: bpsec.ContextBIBHMACSHA2,
				Source:    bp.IPN(2, 1),
				Results:   [][]bpsec.Result{{}, {}},
			},
			want: bpsec.ErrDuplicateTarget,
		},
		{
			name: "fewer result sets than targets",
			asb: bpsec.ASB{
				Targets:   []uint64{1, 2},
				ContextID: bpsec.ContextBIBHMACSHA2,
				Source:    bp.IPN(2, 1),
				Results:   [][]bpsec.Result{{}},
			},
			want: bpsec.ErrResultCountMismatch,
		},
		{
			name: "a reserved context flag",
			asb: bpsec.ASB{
				Targets:      []uint64{1},
				ContextID:    bpsec.ContextBIBHMACSHA2,
				ContextFlags: 0x02,
				Source:       bp.IPN(2, 1),
				Results:      [][]bpsec.Result{{}},
			},
			want: bpsec.ErrReservedContextFlag,
		},
		{
			name: "parameters present without the flag",
			asb: bpsec.ASB{
				Targets:    []uint64{1},
				ContextID:  bpsec.ContextBIBHMACSHA2,
				Source:     bp.IPN(2, 1),
				Parameters: []bpsec.Parameter{{ID: 1, Value: []byte{0x07}}},
				Results:    [][]bpsec.Result{{}},
			},
			want: bpsec.ErrParametersFlagDisagrees,
		},
		{
			name: "the flag set with no parameters",
			asb: bpsec.ASB{
				Targets:      []uint64{1},
				ContextID:    bpsec.ContextBIBHMACSHA2,
				ContextFlags: bpsec.ContextFlagParametersPresent,
				Source:       bp.IPN(2, 1),
				Results:      [][]bpsec.Result{{}},
			},
			want: bpsec.ErrParametersFlagDisagrees,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := tt.asb.Validate(); !errors.Is(err, tt.want) {
				t.Errorf("Validate = %v, want %v", err, tt.want)
			}
			if _, err := tt.asb.Encode(); !errors.Is(err, tt.want) {
				t.Errorf("Encode = %v, want %v", err, tt.want)
			}
		})
	}
}

func TestDecodeASBRejectsMalformedInput(t *testing.T) {
	tests := []struct {
		name  string
		input string
	}{
		{"empty", ""},
		{"targets are not an array", "0101018202820201"},
		{"truncated after the target list", "8101"},
		{"a parameter that is not a two-item array", "81010101820282020181810181818201582000"},
		{"a security source that is not an endpoint ID", "810101018201018181820140"},
		{
			name: "octets after the end",
			input: "810101018202820201828201078203008181820158403bdc69b3a34a2b5d3a8554368bd1e808" +
				"f606219d2a10a846eae3886ae4ecc83c4ee550fdfb1cc636b904e2f1a73e303dcd4b6ccece003e95e8164dcc89a156e1" +
				"ff",
		},
	}

	// The sentinel varies with where the read gives up — a truncated array
	// head is a CBOR error, a two-item rule broken is an ASB error — so what
	// is asserted here is that nothing malformed comes back as a usable block.
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			asb, err := bpsec.DecodeASB(mustHex(t, tt.input))
			if err == nil {
				t.Fatal("DecodeASB accepted malformed input")
			}
			if asb != nil {
				t.Error("DecodeASB returned a block alongside the error")
			}
		})
	}
}

// A target list claiming more entries than the input could possibly hold must
// be refused before anything is allocated for it.
func TestDecodeASBRejectsHugeCounts(t *testing.T) {
	for _, input := range []string{
		"9bffffffffffffffff",
		"81011bffffffffffffffff",
		"8101010182028202018bffffffffffffffff",
	} {
		if _, err := bpsec.DecodeASB(mustHex(t, input)); err == nil {
			t.Errorf("DecodeASB(%s) accepted a length field larger than the input", input)
		}
	}
}

// S8: a count that is not astronomically large -- so a bound comparing it
// only to the octets left would pass it -- must still be refused once it asks
// for more elements than the real per-element minimum size allows. This is
// the case TestDecodeASBRejectsHugeCounts does not cover: both inputs below
// carry enough padding to satisfy "count <= octets remaining", and both must
// still be rejected because a target costs 8 bytes of Go allocation apiece
// (past maxTargets) and a parameter or result pair cannot be shorter than
// three octets on the wire.
func TestDecodeASBRejectsInflatedCounts(t *testing.T) {
	// A target count of maxTargets+1 (1025), with 1025 filler octets
	// following -- enough to satisfy a bound of one octet per element. Pinned
	// to ErrMalformedASB specifically -- not just any error -- because a
	// bound that only compared the count to the octets left would still fail
	// this input eventually (it runs out of bytes reading the fields after
	// the target list), just later and for an unrelated reason.
	hugeTargets := "990401" + strings.Repeat("00", 1025)
	if _, err := bpsec.DecodeASB(mustHex(t, hugeTargets)); !errors.Is(err, bpsec.ErrMalformedASB) {
		t.Errorf("a target count past maxTargets: err = %v, want ErrMalformedASB", err)
	}

	// One target, context 1, flags 1 (parameters present), source ipn:2.1,
	// then a parameters count of 10 with only 20 octets following -- enough
	// for a bound of one octet per element, not enough for the real minimum
	// of three.
	inflatedPairs := "8101" + "01" + "01" + "8202820201" + "8a" + strings.Repeat("00", 20)
	if _, err := bpsec.DecodeASB(mustHex(t, inflatedPairs)); err == nil {
		t.Error("a parameter count past the true per-element minimum was accepted")
	}

	// The tightened bounds must not disturb a legitimately small ASB: reuse
	// the A.1 vector, one target and two parameters.
	small := "810101018202820201828201078203008181820158403bdc69b3a34a2b5d3a8554368bd1e808" +
		"f606219d2a10a846eae3886ae4ecc83c4ee550fdfb1cc636b904e2f1a73e303dcd4b6ccece003e95e8164dcc89a156e1"
	if _, err := bpsec.DecodeASB(mustHex(t, small)); err != nil {
		t.Errorf("a legitimately small ASB was rejected: %v", err)
	}
}
