package pxdl_test

import (
	"bytes"
	"errors"
	"testing"

	"github.com/ravisuhag/astro/pkg/pxdl"
)

func TestSegmentHeaderRoundTrip(t *testing.T) {
	for _, flags := range []pxdl.SequenceFlags{
		pxdl.SegmentFirst, pxdl.SegmentContinuing, pxdl.SegmentLast, pxdl.SegmentUnsegmented,
	} {
		h := pxdl.SegmentHeader{SequenceFlags: flags, PseudoPacketID: 42}
		encoded, err := h.Encode()
		if err != nil {
			t.Fatal(err)
		}
		if len(encoded) != pxdl.SegmentHeaderSize {
			t.Fatalf("encoded %d octets, want 1", len(encoded))
		}

		var got pxdl.SegmentHeader
		if err := got.Decode(encoded); err != nil {
			t.Fatal(err)
		}
		if got.SequenceFlags != flags {
			t.Errorf("flags = %s, want %s", got.SequenceFlags, flags)
		}
		if got.PseudoPacketID != 42 {
			t.Errorf("pseudo packet ID = %d, want 42", got.PseudoPacketID)
		}
	}
}

func TestSequenceFlagValues(t *testing.T) {
	// Table 3-4. The values are not in the order you would guess: '01' is
	// first, '00' is continuing.
	tests := []struct {
		flags pxdl.SequenceFlags
		value uint8
	}{
		{pxdl.SegmentContinuing, 0},
		{pxdl.SegmentFirst, 1},
		{pxdl.SegmentLast, 2},
		{pxdl.SegmentUnsegmented, 3},
	}
	for _, tt := range tests {
		if uint8(tt.flags) != tt.value {
			t.Errorf("%s = %d, want %d", tt.flags, uint8(tt.flags), tt.value)
		}
	}
}

func TestPseudoPacketIDSixBits(t *testing.T) {
	h := pxdl.SegmentHeader{PseudoPacketID: 0x40} // one past 6 bits
	if err := h.Validate(); !errors.Is(err, pxdl.ErrInvalidSegment) {
		t.Errorf("error = %v, want ErrInvalidSegment", err)
	}
}

func TestSegmentizeAndReassemble(t *testing.T) {
	packet := make([]byte, 500)
	for i := range packet {
		packet[i] = byte(i * 3)
	}

	segments, err := pxdl.Segmentize(packet, 7, 100)
	if err != nil {
		t.Fatal(err)
	}
	if len(segments) != 5 {
		t.Fatalf("got %d segments, want 5", len(segments))
	}
	if segments[0].Header.SequenceFlags != pxdl.SegmentFirst {
		t.Error("the first segment is not flagged first")
	}
	if segments[4].Header.SequenceFlags != pxdl.SegmentLast {
		t.Error("the last segment is not flagged last")
	}
	for i := 1; i < 4; i++ {
		if segments[i].Header.SequenceFlags != pxdl.SegmentContinuing {
			t.Errorf("segment %d is not flagged continuing", i)
		}
	}

	r := pxdl.NewReassembler()
	var complete []byte
	for i, seg := range segments {
		out, err := r.Accept(0, 1, seg)
		if err != nil {
			t.Fatalf("segment %d: %v", i, err)
		}
		if out != nil {
			complete = out
		}
	}
	if complete == nil {
		t.Fatal("reassembly never completed")
	}
	if !bytes.Equal(complete, packet) {
		t.Error("the reassembled packet differs from the original")
	}
	if r.Pending() != 0 {
		t.Errorf("%d partial packets left over", r.Pending())
	}
}

func TestSegmentizeSmallPacketIsUnsegmented(t *testing.T) {
	// A packet that fits in one segment gets the '11' flag.
	segments, err := pxdl.Segmentize([]byte("small"), 1, 100)
	if err != nil {
		t.Fatal(err)
	}
	if len(segments) != 1 {
		t.Fatalf("got %d segments, want 1", len(segments))
	}
	if segments[0].Header.SequenceFlags != pxdl.SegmentUnsegmented {
		t.Errorf("flags = %s, want unsegmented", segments[0].Header.SequenceFlags)
	}

	r := pxdl.NewReassembler()
	out, err := r.Accept(0, 0, segments[0])
	if err != nil {
		t.Fatal(err)
	}
	if string(out) != "small" {
		t.Errorf("delivered %q, want small", out)
	}
}

func TestReassemblyInterleavesByRoutingID(t *testing.T) {
	// Clause 3.2.3.3.2 c): segments of different packets may interleave when their
	// PCID or Port ID differs.
	first, err := pxdl.Segmentize([]byte("packet one contents"), 1, 8)
	if err != nil {
		t.Fatal(err)
	}
	second, err := pxdl.Segmentize([]byte("packet two contents"), 2, 8)
	if err != nil {
		t.Fatal(err)
	}

	r := pxdl.NewReassembler()
	var gotFirst, gotSecond []byte

	// Interleave them on different ports.
	maxLen := len(first)
	if len(second) > maxLen {
		maxLen = len(second)
	}
	for i := 0; i < maxLen; i++ {
		if i < len(first) {
			out, err := r.Accept(0, 1, first[i])
			if err != nil {
				t.Fatal(err)
			}
			if out != nil {
				gotFirst = out
			}
		}
		if i < len(second) {
			out, err := r.Accept(0, 2, second[i])
			if err != nil {
				t.Fatal(err)
			}
			if out != nil {
				gotSecond = out
			}
		}
	}

	if string(gotFirst) != "packet one contents" {
		t.Errorf("first packet = %q", gotFirst)
	}
	if string(gotSecond) != "packet two contents" {
		t.Errorf("second packet = %q", gotSecond)
	}
}

func TestReassemblyRejectsSegmentBeforeStart(t *testing.T) {
	// Clause 3.2.3.3.5 b): the first segment for a routing ID must be a start
	// segment. Anything else is discarded rather than guessed at.
	r := pxdl.NewReassembler()
	stray := &pxdl.Segment{
		Header: pxdl.SegmentHeader{SequenceFlags: pxdl.SegmentContinuing, PseudoPacketID: 3},
		Data:   []byte("orphan"),
	}
	if _, err := r.Accept(0, 0, stray); !errors.Is(err, pxdl.ErrSegmentOutOfOrder) {
		t.Errorf("error = %v, want ErrSegmentOutOfOrder", err)
	}
}

func TestReassemblyDeliversOnlyCompletePackets(t *testing.T) {
	// Clause 3.2.3.3.4.
	segments, err := pxdl.Segmentize(make([]byte, 300), 1, 100)
	if err != nil {
		t.Fatal(err)
	}

	r := pxdl.NewReassembler()
	for _, seg := range segments[:len(segments)-1] {
		out, err := r.Accept(0, 0, seg)
		if err != nil {
			t.Fatal(err)
		}
		if out != nil {
			t.Fatal("a partial packet was delivered")
		}
	}
	if r.Pending() != 1 {
		t.Errorf("pending = %d, want 1", r.Pending())
	}
}

func TestReassemblyBoundsPacketSize(t *testing.T) {
	// A run of continuing segments that never ends must not grow without limit.
	r := pxdl.NewReassembler()
	r.MaxPacketSize = 100

	first := &pxdl.Segment{
		Header: pxdl.SegmentHeader{SequenceFlags: pxdl.SegmentFirst},
		Data:   make([]byte, 60),
	}
	if _, err := r.Accept(0, 0, first); err != nil {
		t.Fatal(err)
	}

	more := &pxdl.Segment{
		Header: pxdl.SegmentHeader{SequenceFlags: pxdl.SegmentContinuing},
		Data:   make([]byte, 60),
	}
	if _, err := r.Accept(0, 0, more); !errors.Is(err, pxdl.ErrReassemblyTooLarge) {
		t.Errorf("error = %v, want ErrReassemblyTooLarge", err)
	}
	if r.Pending() != 0 {
		t.Error("the oversized partial packet was not discarded")
	}
}

func TestNewFirstSegmentAbandonsThePrevious(t *testing.T) {
	// Clause 3.2.3.3.5 b): a start segment arriving mid-packet restarts.
	r := pxdl.NewReassembler()

	if _, err := r.Accept(0, 0, &pxdl.Segment{
		Header: pxdl.SegmentHeader{SequenceFlags: pxdl.SegmentFirst},
		Data:   []byte("abandoned"),
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := r.Accept(0, 0, &pxdl.Segment{
		Header: pxdl.SegmentHeader{SequenceFlags: pxdl.SegmentFirst},
		Data:   []byte("restart"),
	}); err != nil {
		t.Fatal(err)
	}

	out, err := r.Accept(0, 0, &pxdl.Segment{
		Header: pxdl.SegmentHeader{SequenceFlags: pxdl.SegmentLast},
		Data:   []byte("ed"),
	})
	if err != nil {
		t.Fatal(err)
	}
	if string(out) != "restarted" {
		t.Errorf("delivered %q, want restarted", out)
	}
}

func TestReassemblerAcceptFrame(t *testing.T) {
	segments, err := pxdl.Segmentize([]byte("carried in frames"), 5, 8)
	if err != nil {
		t.Fatal(err)
	}

	r := pxdl.NewReassembler()
	var complete []byte

	for _, seg := range segments {
		body, err := seg.Encode()
		if err != nil {
			t.Fatal(err)
		}
		f, err := pxdl.NewTransferFrame(42, 3, body, pxdl.WithDFCID(pxdl.DFCSegment))
		if err != nil {
			t.Fatal(err)
		}

		encoded, err := f.Encode()
		if err != nil {
			t.Fatal(err)
		}
		decoded, err := pxdl.DecodeTransferFrame(encoded)
		if err != nil {
			t.Fatal(err)
		}

		out, err := r.AcceptFrame(decoded)
		if err != nil {
			t.Fatal(err)
		}
		if out != nil {
			complete = out
		}
	}

	if string(complete) != "carried in frames" {
		t.Errorf("delivered %q, want 'carried in frames'", complete)
	}
}

func TestAcceptFrameRejectsWrongDFCID(t *testing.T) {
	f, err := pxdl.NewTransferFrame(1, 0, []byte("not a segment"), pxdl.WithDFCID(pxdl.DFCPackets))
	if err != nil {
		t.Fatal(err)
	}
	r := pxdl.NewReassembler()
	if _, err := r.AcceptFrame(f); !errors.Is(err, pxdl.ErrInvalidDFCID) {
		t.Errorf("error = %v, want ErrInvalidDFCID", err)
	}
}
