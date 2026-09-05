package cfdp_test

import (
	"errors"
	"math"
	"testing"

	"github.com/ravisuhag/astro/pkg/cfdp"
)

// sendSmallMetadata opens the transaction with a Metadata PDU too small to
// interact with any of the size limits under test.
func sendSmallMetadata(t *testing.T, receiver *cfdp.Receiver) {
	t.Helper()
	meta := &cfdp.MetadataPDU{
		ChecksumType:        cfdp.ChecksumModular,
		FileSize:            10,
		SourceFileName:      cfdp.LV{Value: []byte("src.dat")},
		DestinationFileName: cfdp.LV{Value: []byte("dst.dat")},
	}
	body, err := meta.Encode(false)
	if err != nil {
		t.Fatal(err)
	}
	if err := receiver.HandlePDU(&cfdp.PDU{Header: toReceiver(false, false), Data: body}); err != nil {
		t.Fatal(err)
	}
}

// A peer controls a File Data PDU's offset and, before Metadata arrives, how
// many PDUs it sends. Nothing in CCSDS 727.0-B-5 bounds either, so without a
// cap a single crafted PDU could make the receiver try to allocate an
// unreasonable amount of memory, and a peer withholding Metadata forever
// could buffer data without limit. These tests exercise the MaxFileSize
// ceiling added to guard against both.

// A File Data PDU naming an offset far past MaxFileSize must fault instead of
// letting the filestore try to grow to match it.
func TestFileDataOffsetPastMaxFileSizeFaults(t *testing.T) {
	dstFS := cfdp.NewMemoryFilestore()
	receiver := cfdp.NewReceiver(dstFS, receiverConfig(false))
	sendSmallMetadata(t, receiver)

	fd := &cfdp.FileDataPDU{Offset: 1 << 40, Data: []byte{1, 2, 3, 4}}
	body, err := fd.Encode(false, true)
	if err != nil {
		t.Fatal(err)
	}
	header := toReceiver(false, true)
	header.LargeFile = true

	err = receiver.HandlePDU(&cfdp.PDU{Header: header, Data: body})
	if !errors.Is(err, cfdp.ErrFileSizeError) {
		t.Fatalf("HandlePDU(huge offset) = %v, want ErrFileSizeError", err)
	}
	if got := receiver.ConditionCode(); got != cfdp.CondFileSizeError {
		t.Errorf("condition = %s, want file size error", got)
	}
}

// An offset near 2^64 makes fd.End() (offset + data length) wrap around; the
// receiver must fault, not panic on a corrupted slice range.
func TestFileDataOffsetOverflowNoPanic(t *testing.T) {
	dstFS := cfdp.NewMemoryFilestore()
	receiver := cfdp.NewReceiver(dstFS, receiverConfig(false))
	sendSmallMetadata(t, receiver)

	fd := &cfdp.FileDataPDU{Offset: math.MaxUint64 - 2, Data: make([]byte, 8)}
	body, err := fd.Encode(false, true)
	if err != nil {
		t.Fatal(err)
	}
	header := toReceiver(false, true)
	header.LargeFile = true

	err = receiver.HandlePDU(&cfdp.PDU{Header: header, Data: body})
	if !errors.Is(err, cfdp.ErrFileSizeError) {
		t.Fatalf("HandlePDU(offset near 2^64) = %v, want ErrFileSizeError", err)
	}
	if got := receiver.ConditionCode(); got != cfdp.CondFileSizeError {
		t.Errorf("condition = %s, want file size error", got)
	}
}

// A peer that never sends Metadata cannot buffer file data forever: once the
// running total held in the pre-metadata buffer passes MaxFileSize, the
// receiver must stop accepting more of it.
func TestPreMetadataBufferingBounded(t *testing.T) {
	dstFS := cfdp.NewMemoryFilestore()
	config := receiverConfig(false)
	config.MaxFileSize = 100
	receiver := cfdp.NewReceiver(dstFS, config)

	chunk := make([]byte, 20)
	for i := 0; i < 5; i++ {
		fd := &cfdp.FileDataPDU{Offset: 0, Data: chunk}
		body, err := fd.Encode(false, false)
		if err != nil {
			t.Fatal(err)
		}
		if err := receiver.HandlePDU(&cfdp.PDU{Header: toReceiver(false, true), Data: body}); err != nil {
			t.Fatalf("chunk %d: unexpected error %v", i, err)
		}
	}

	// The 6th chunk pushes the buffered total from 100 to 120, past the cap,
	// even though this one segment's own offset and length are unremarkable.
	fd := &cfdp.FileDataPDU{Offset: 0, Data: chunk}
	body, err := fd.Encode(false, false)
	if err != nil {
		t.Fatal(err)
	}
	err = receiver.HandlePDU(&cfdp.PDU{Header: toReceiver(false, true), Data: body})
	if !errors.Is(err, cfdp.ErrFileSizeError) {
		t.Fatalf("HandlePDU(6th chunk) = %v, want ErrFileSizeError", err)
	}
	if got := receiver.ConditionCode(); got != cfdp.CondFileSizeError {
		t.Errorf("condition = %s, want file size error", got)
	}
	if !receiver.Done() {
		t.Error("receiver should have ended the transaction once buffered data passed MaxFileSize")
	}
}

// MemoryFilestore is a public type usable outside a Receiver, so it must
// reject a dangerous offset on its own rather than trust the caller.
func TestMemoryFilestoreWriteAtBounds(t *testing.T) {
	fs := cfdp.NewMemoryFilestore()

	if err := fs.WriteAt("a", math.MaxUint64-2, make([]byte, 8)); !errors.Is(err, cfdp.ErrFileTooLarge) {
		t.Errorf("WriteAt(offset near 2^64) = %v, want ErrFileTooLarge", err)
	}
	if err := fs.WriteAt("b", cfdp.DefaultMaxFileSize, []byte{1}); !errors.Is(err, cfdp.ErrFileTooLarge) {
		t.Errorf("WriteAt(offset past the ceiling) = %v, want ErrFileTooLarge", err)
	}
	if err := fs.WriteAt("c", 0, []byte("hello")); err != nil {
		t.Errorf("WriteAt(ordinary small write) = %v, want nil", err)
	}
}

// The new ceiling must not get in the way of an ordinary small transfer.
func TestSmallTransferStillCompletesWithDefaultLimits(t *testing.T) {
	content := testFile()
	srcFS := cfdp.NewMemoryFilestore()
	if err := srcFS.WriteAt("src.dat", 0, content); err != nil {
		t.Fatal(err)
	}
	dstFS := cfdp.NewMemoryFilestore()

	sender, err := cfdp.NewSender(srcFS, senderConfig(false))
	if err != nil {
		t.Fatal(err)
	}
	receiver := cfdp.NewReceiver(dstFS, receiverConfig(false))

	for _, pdu := range collectPDUs(t, sender) {
		if err := receiver.HandlePDU(pdu); err != nil {
			t.Fatalf("receiver rejected a PDU: %v", err)
		}
	}

	if !receiver.Complete() {
		t.Fatalf("transfer incomplete; missing %+v", receiver.MissingSegments())
	}
	if got := receiver.ConditionCode(); got != cfdp.CondNoError {
		t.Errorf("condition = %s, want no error", got)
	}
}
