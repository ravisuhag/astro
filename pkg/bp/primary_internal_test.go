package bp

import (
	"encoding/hex"
	"errors"
	"testing"
)

// The vector is RFC 9173 appendix A.1.1.1, which prints the primary block of a
// worked example bundle in both diagnostic notation and hex. It is outside
// corroboration rather than a value derived from the clause layout: the bytes
// were published by a different working group to seed interoperability suites.
//
//	[
//	  7,           / BP version            /
//	  0,           / flags                 /
//	  0,           / CRC type              /
//	  [2, [1,2]],  / destination (ipn:1.2) /
//	  [2, [2,1]],  / source      (ipn:2.1) /
//	  [2, [2,1]],  / report-to   (ipn:2.1) /
//	  [0, 40],     / timestamp             /
//	  1000000      / lifetime              /
//	]
const rfc9173PrimaryBlock = "88070000820282010282028202018202820201820018281a000f4240"

func TestPrimaryBlockRFC9173Vector(t *testing.T) {
	p := &PrimaryBlock{
		Flags:       0,
		CRCType:     CRCNone,
		Destination: IPN(1, 2),
		Source:      IPN(2, 1),
		ReportTo:    IPN(2, 1),
		Timestamp:   CreationTimestamp{Time: DTNTimeUnknown, Sequence: 40},
		Lifetime:    1000000,
	}

	got, err := appendPrimaryBlock(nil, p)
	if err != nil {
		t.Fatalf("appendPrimaryBlock: %v", err)
	}
	if hex.EncodeToString(got) != rfc9173PrimaryBlock {
		t.Fatalf("encoded  %s\nwant     %s", hex.EncodeToString(got), rfc9173PrimaryBlock)
	}

	back, err := newDecoder(mustHex(t, rfc9173PrimaryBlock)).primaryBlock()
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	if *back != *p {
		t.Errorf("round trip = %+v\nwant        %+v", *back, *p)
	}
}

// Clause 4.3.1 fixes the array length by two facts: whether the bundle is a
// fragment, and whether a checksum is present. Check all four combinations,
// and check the checksum verifies on the way back in.
func TestPrimaryBlockShapes(t *testing.T) {
	base := PrimaryBlock{
		Destination: IPN(1, 2),
		Source:      IPN(2, 1),
		ReportTo:    IPN(2, 1),
		Timestamp:   CreationTimestamp{Time: 757382400000, Sequence: 7},
		Lifetime:    3600000,
	}

	tests := []struct {
		name      string
		mutate    func(*PrimaryBlock)
		wantItems byte
	}{
		{"plain", func(p *PrimaryBlock) {}, 8},
		{"with CRC-16", func(p *PrimaryBlock) { p.CRCType = CRC16X25 }, 9},
		{"with CRC-32C", func(p *PrimaryBlock) { p.CRCType = CRC32C }, 9},
		{"fragment", func(p *PrimaryBlock) {
			p.Flags = FlagIsFragment
			p.FragmentOffset, p.TotalADULength = 1024, 4096
		}, 10},
		{"fragment with CRC-32C", func(p *PrimaryBlock) {
			p.Flags = FlagIsFragment
			p.FragmentOffset, p.TotalADULength = 1024, 4096
			p.CRCType = CRC32C
		}, 11},
	}

	for _, tt := range tests {
		p := base
		tt.mutate(&p)

		encoded, err := appendPrimaryBlock(nil, &p)
		if err != nil {
			t.Errorf("%s: encode: %v", tt.name, err)
			continue
		}
		// A definite array of 8 to 11 items has head byte 0x80|n.
		if encoded[0] != 0x80|tt.wantItems {
			t.Errorf("%s: array head = 0x%02X, want 0x%02X", tt.name, encoded[0], 0x80|tt.wantItems)
		}

		back, err := newDecoder(encoded).primaryBlock()
		if err != nil {
			t.Errorf("%s: decode: %v", tt.name, err)
			continue
		}
		if *back != p {
			t.Errorf("%s round trip = %+v, want %+v", tt.name, *back, p)
		}
	}
}

// A checksum that covers its own zeroed field is easy to get subtly wrong, and
// a wrong version agrees with itself. Corrupt a byte and the decoder must
// refuse.
func TestPrimaryBlockDetectsCorruption(t *testing.T) {
	for _, crcType := range []CRCType{CRC16X25, CRC32C} {
		p := &PrimaryBlock{
			CRCType:     crcType,
			Destination: IPN(1, 2),
			Source:      IPN(2, 1),
			ReportTo:    IPN(2, 1),
			Timestamp:   CreationTimestamp{Time: 1, Sequence: 1},
			Lifetime:    1000,
		}
		encoded, err := appendPrimaryBlock(nil, p)
		if err != nil {
			t.Fatalf("encode: %v", err)
		}

		for i := range encoded {
			corrupt := make([]byte, len(encoded))
			copy(corrupt, encoded)
			corrupt[i] ^= 0x01

			if _, err := newDecoder(corrupt).primaryBlock(); err == nil {
				t.Errorf("CRCType(%d): flipping a bit at offset %d went unnoticed", crcType, i)
			}
		}
	}
}

// Clause 4.2.3 states two rules that no single field can satisfy on its own.
// Both are the kind that a round trip inside one implementation never catches,
// because the encoder and decoder agree with each other while the bundle is
// invalid.
func TestPrimaryBlockCrossFieldRules(t *testing.T) {
	tests := []struct {
		name string
		p    PrimaryBlock
		want error
	}{
		{
			"administrative record asking for a delivery report",
			PrimaryBlock{
				Flags:       FlagAdminRecord | FlagReportDelivery,
				Destination: IPN(1, 2), Source: IPN(2, 1), ReportTo: IPN(2, 1),
			},
			ErrAdminRecordWantsReports,
		},
		{
			"anonymous bundle without the must-not-fragment flag",
			PrimaryBlock{
				Destination: IPN(1, 2), Source: NullEID(), ReportTo: IPN(2, 1),
			},
			ErrAnonymousBundleFragmentable,
		},
		{
			"anonymous bundle asking for a reception report",
			PrimaryBlock{
				Flags:       FlagMustNotFragment | FlagReportReception,
				Destination: IPN(1, 2), Source: NullEID(), ReportTo: IPN(2, 1),
			},
			ErrAnonymousBundleWantsReports,
		},
		{
			"undefined CRC type code",
			PrimaryBlock{
				CRCType:     CRCType(3),
				Destination: IPN(1, 2), Source: IPN(2, 1), ReportTo: IPN(2, 1),
			},
			ErrInvalidCRCType,
		},
	}

	for _, tt := range tests {
		if _, err := appendPrimaryBlock(nil, &tt.p); !errors.Is(err, tt.want) {
			t.Errorf("%s: err = %v, want %v", tt.name, err, tt.want)
		}
	}

	// An anonymous bundle that follows both rules is fine.
	ok := PrimaryBlock{
		Flags:       FlagMustNotFragment,
		Destination: IPN(1, 2), Source: NullEID(), ReportTo: NullEID(),
	}
	if _, err := appendPrimaryBlock(nil, &ok); err != nil {
		t.Errorf("a conforming anonymous bundle was refused: %v", err)
	}
}

func TestPrimaryBlockRejects(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  error
	}{
		{"version 6", "88060000820282010282028202018202820201820018281a000f4240", ErrUnsupportedVersion},
		{"array of seven items", "87" + "070000" + "8202820102" + "8202820201" + "8202820201" + "82001828", ErrMalformedPrimaryBlock},
		// The fragment flag is set, so clause 4.3.1 wants ten items, but the
		// array declares eight. Encoder and decoder that both ignored the
		// array length would agree with each other and be wrong on the wire.
		{"fragment flag without the fragment fields", "88" + "070100" + "8202820102" + "8202820201" + "8202820201" + "82001828" + "1a000f4240", ErrPrimaryBlockLengthMismatch},
		{"indefinite-length array", "9f070000820282010282028202018202820201820018281a000f4240ff", ErrMalformedPrimaryBlock},
		{"undefined CRC type code", "88070003820282010282028202018202820201820018281a000f4240", ErrInvalidCRCType},
	}

	for _, tt := range tests {
		_, err := newDecoder(mustHex(t, tt.input)).primaryBlock()
		if !errors.Is(err, tt.want) {
			t.Errorf("%s: err = %v, want %v", tt.name, err, tt.want)
		}
	}
}
