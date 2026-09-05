package cfdp_test

import (
	"bytes"
	"errors"
	"testing"

	"github.com/ravisuhag/astro/pkg/cfdp"
)

// newOpenReceiver returns a receiver whose Metadata PDU has already arrived,
// declaring a file of the given size, so tests can go straight to sending
// File Data PDUs and inspecting how the received-byte-range set folds them
// together.
func newOpenReceiver(t *testing.T, dstFS cfdp.Filestore, config cfdp.ReceiverConfig, fileSize uint64) *cfdp.Receiver {
	t.Helper()
	receiver := cfdp.NewReceiver(dstFS, config)
	meta := &cfdp.MetadataPDU{
		ChecksumType:        cfdp.ChecksumModular,
		FileSize:            fileSize,
		SourceFileName:      cfdp.LV{Value: []byte("src.dat")},
		DestinationFileName: cfdp.LV{Value: []byte("dst.dat")},
	}
	body, err := meta.Encode(false)
	if err != nil {
		t.Fatal(err)
	}
	if err := receiver.HandlePDU(&cfdp.PDU{Header: toReceiver(config.Acknowledged, false), Data: body}); err != nil {
		t.Fatal(err)
	}
	return receiver
}

// sendData pushes one File Data PDU covering [start, end) of content.
func sendData(t *testing.T, receiver *cfdp.Receiver, content []byte, start, end uint64) {
	t.Helper()
	fd := &cfdp.FileDataPDU{Offset: start, Data: content[start:end]}
	body, err := fd.Encode(false, false)
	if err != nil {
		t.Fatal(err)
	}
	if err := receiver.HandlePDU(&cfdp.PDU{Header: toReceiver(false, true), Data: body}); err != nil {
		t.Fatal(err)
	}
}

// These characterize recordSegment's merge behavior through the public
// surface (MissingSegments) before its rebuild-per-call implementation is
// replaced with a binary-search insert (S10), so the refactor can be checked
// against the same set of scenarios both before and after.

// Two ranges overlapping in the middle must fold into one contiguous run.
func TestRecordSegmentOverlapping(t *testing.T) {
	content := make([]byte, 200)
	dstFS := cfdp.NewMemoryFilestore()
	receiver := newOpenReceiver(t, dstFS, receiverConfig(false), 200)

	sendData(t, receiver, content, 0, 120)
	sendData(t, receiver, content, 80, 200)

	missing := receiver.MissingSegments()
	if len(missing) != 0 {
		t.Fatalf("missing = %+v, want none: [0,120) and [80,200) overlap and cover [0,200)", missing)
	}
}

// Two ranges that exactly touch, with no overlap, must merge the same as an
// overlapping pair does.
func TestRecordSegmentAdjacentTouching(t *testing.T) {
	content := make([]byte, 200)
	dstFS := cfdp.NewMemoryFilestore()
	receiver := newOpenReceiver(t, dstFS, receiverConfig(false), 200)

	sendData(t, receiver, content, 0, 50)
	sendData(t, receiver, content, 50, 200)

	missing := receiver.MissingSegments()
	if len(missing) != 0 {
		t.Fatalf("missing = %+v, want none: [0,50) and [50,200) touch and cover [0,200)", missing)
	}
}

// The same range arriving twice (a plausible retransmission) must not double
// count: the checksum is the tell, since folding it in twice would corrupt it.
func TestRecordSegmentExactDuplicate(t *testing.T) {
	content := make([]byte, 200)
	for i := range content {
		content[i] = byte(i * 13)
	}
	dstFS := cfdp.NewMemoryFilestore()
	receiver := newOpenReceiver(t, dstFS, receiverConfig(false), 200)

	sendData(t, receiver, content, 0, 100)
	sendData(t, receiver, content, 0, 100) // exact duplicate

	eof := &cfdp.EOFPDU{FileChecksum: modularSum(content), FileSize: 200}
	// The second half completes the file so the checksum can be checked.
	sendData(t, receiver, content, 100, 200)
	body, err := eof.Encode(false)
	if err != nil {
		t.Fatal(err)
	}
	if err := receiver.HandlePDU(&cfdp.PDU{Header: toReceiver(false, false), Data: body}); err != nil {
		t.Fatal(err)
	}
	if got := receiver.ConditionCode(); got != cfdp.CondNoError {
		t.Errorf("condition = %s, want no error; the duplicate must not have been folded into the checksum twice", got)
	}
}

// A range fully inside one already received must not be written or summed
// again.
func TestRecordSegmentFullyContained(t *testing.T) {
	content := make([]byte, 200)
	for i := range content {
		content[i] = byte(i * 17)
	}
	dstFS := cfdp.NewMemoryFilestore()
	receiver := newOpenReceiver(t, dstFS, receiverConfig(false), 200)

	sendData(t, receiver, content, 0, 200)
	sendData(t, receiver, content, 30, 60) // fully inside [0, 200)

	if missing := receiver.MissingSegments(); len(missing) != 0 {
		t.Fatalf("missing = %+v, want none", missing)
	}

	eof := &cfdp.EOFPDU{FileChecksum: modularSum(content), FileSize: 200}
	body, err := eof.Encode(false)
	if err != nil {
		t.Fatal(err)
	}
	if err := receiver.HandlePDU(&cfdp.PDU{Header: toReceiver(false, false), Data: body}); err != nil {
		t.Fatal(err)
	}
	if got := receiver.ConditionCode(); got != cfdp.CondNoError {
		t.Errorf("condition = %s, want no error; the contained range must not have been summed twice", got)
	}
	delivered, err := dstFS.Read("dst.dat")
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(delivered, content) {
		t.Error("delivered contents differ")
	}
}

// Segments arriving out of offset order must still end up merged the same
// as if they had arrived in order.
func TestRecordSegmentReverseOrderArrival(t *testing.T) {
	content := make([]byte, 200)
	dstFS := cfdp.NewMemoryFilestore()
	receiver := newOpenReceiver(t, dstFS, receiverConfig(false), 200)

	sendData(t, receiver, content, 150, 200)
	sendData(t, receiver, content, 80, 150)
	sendData(t, receiver, content, 0, 80)

	if missing := receiver.MissingSegments(); len(missing) != 0 {
		t.Fatalf("missing = %+v, want none: three adjacent runs delivered in reverse order should still merge", missing)
	}
}

// A File Data PDU carrying no data is a no-op: it must not appear as a
// received range, or otherwise disturb the set.
func TestRecordSegmentZeroLength(t *testing.T) {
	content := make([]byte, 200)
	dstFS := cfdp.NewMemoryFilestore()
	receiver := newOpenReceiver(t, dstFS, receiverConfig(false), 200)

	sendData(t, receiver, content, 0, 100)
	before := receiver.MissingSegments()

	fd := &cfdp.FileDataPDU{Offset: 100, Data: nil}
	body, err := fd.Encode(false, false)
	if err != nil {
		t.Fatal(err)
	}
	if err := receiver.HandlePDU(&cfdp.PDU{Header: toReceiver(false, true), Data: body}); err != nil {
		t.Fatal(err)
	}

	after := receiver.MissingSegments()
	if len(before) != len(after) {
		t.Fatalf("missing changed from %+v to %+v after an empty File Data PDU", before, after)
	}
	for i := range before {
		if before[i] != after[i] {
			t.Fatalf("missing changed from %+v to %+v after an empty File Data PDU", before, after)
		}
	}
}

// S10: nothing bounds how many distinct, non-adjacent ranges a peer can
// force the receiver to track, absent MaxSegments. A low cap must trip once
// a genuinely new, non-touching range would exceed it.
func TestMaxSegmentsCapEnforced(t *testing.T) {
	config := receiverConfig(false)
	config.MaxSegments = 5

	content := make([]byte, 100)
	dstFS := cfdp.NewMemoryFilestore()
	receiver := newOpenReceiver(t, dstFS, config, 100)

	// Five one-byte segments at offsets 0, 2, 4, 6, 8 -- all non-adjacent,
	// each its own distinct entry -- fill the cap exactly.
	for i := uint64(0); i < 5; i++ {
		sendData(t, receiver, content, 2*i, 2*i+1)
	}
	if missing := receiver.MissingSegments(); len(missing) == 0 {
		t.Fatal("expected gaps between the five one-byte segments")
	}

	// A sixth, still non-adjacent, must be rejected rather than tracked.
	fd := &cfdp.FileDataPDU{Offset: 10, Data: content[10:11]}
	body, err := fd.Encode(false, false)
	if err != nil {
		t.Fatal(err)
	}
	err = receiver.HandlePDU(&cfdp.PDU{Header: toReceiver(false, true), Data: body})
	if !errors.Is(err, cfdp.ErrFilestoreRejection) {
		t.Fatalf("HandlePDU(6th distinct segment) = %v, want ErrFilestoreRejection", err)
	}
	if got := receiver.ConditionCode(); got != cfdp.CondFilestoreRejection {
		t.Errorf("condition = %s, want filestore rejection", got)
	}
	if !receiver.Done() {
		t.Error("receiver should have ended the transaction once MaxSegments was exceeded")
	}
}

// The cap must not get in the way of an ordinary transfer where every
// arriving segment touches or overlaps what came before: merging never grows
// the tracked count, so it must never trip MaxSegments.
func TestMaxSegmentsCapDoesNotImpedeContiguousTransfer(t *testing.T) {
	config := receiverConfig(false)
	config.MaxSegments = 3

	content := make([]byte, 50)
	for i := range content {
		content[i] = byte(i * 5)
	}
	dstFS := cfdp.NewMemoryFilestore()
	receiver := newOpenReceiver(t, dstFS, config, 50)

	// Fifty one-byte, strictly sequential writes: each touches the one
	// before it, so they all fold into a single tracked range regardless of
	// how low MaxSegments is set.
	for i := uint64(0); i < 50; i++ {
		sendData(t, receiver, content, i, i+1)
	}

	if got := receiver.ConditionCode(); got != cfdp.CondNoError {
		t.Fatalf("condition = %s, want no error", got)
	}
	eof := &cfdp.EOFPDU{FileChecksum: modularSum(content), FileSize: 50}
	body, err := eof.Encode(false)
	if err != nil {
		t.Fatal(err)
	}
	if err := receiver.HandlePDU(&cfdp.PDU{Header: toReceiver(false, false), Data: body}); err != nil {
		t.Fatal(err)
	}
	if !receiver.Complete() {
		t.Fatalf("transfer incomplete; missing %+v", receiver.MissingSegments())
	}
	delivered, err := dstFS.Read("dst.dat")
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(delivered, content) {
		t.Error("delivered contents differ")
	}
}
