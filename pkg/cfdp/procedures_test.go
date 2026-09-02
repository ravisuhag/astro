package cfdp_test

import (
	"bytes"
	"errors"
	"testing"

	"github.com/ravisuhag/astro/pkg/cfdp"
)

// toReceiver builds the header of a PDU heading to the file receiver for the
// transaction the test configs above describe.
func toReceiver(acknowledged, isFileData bool) *cfdp.PDUHeader {
	return &cfdp.PDUHeader{
		IsFileData:     isFileData,
		Direction:      cfdp.TowardReceiver,
		Acknowledged:   acknowledged,
		Source:         cfdp.EntityID{Value: 1, Width: 1},
		TransactionSeq: cfdp.EntityID{Value: 7, Width: 2},
		Destination:    cfdp.EntityID{Value: 2, Width: 1},
	}
}

// collectPDUs drains a sender into a slice.
func collectPDUs(t *testing.T, sender *cfdp.Sender) []*cfdp.PDU {
	t.Helper()
	var out []*cfdp.PDU
	for {
		pdu, ok, err := sender.NextPDU()
		if err != nil {
			t.Fatal(err)
		}
		if !ok {
			return out
		}
		out = append(out, pdu)
	}
}

// drainDirectives drains a receiver and returns the directive codes queued.
func drainDirectives(t *testing.T, receiver *cfdp.Receiver) ([]cfdp.DirectiveCode, []*cfdp.PDU) {
	t.Helper()
	var codes []cfdp.DirectiveCode
	var pdus []*cfdp.PDU
	for {
		pdu, ok, err := receiver.NextPDU()
		if err != nil {
			t.Fatal(err)
		}
		if !ok {
			return codes, pdus
		}
		code, err := cfdp.DirectiveCodeOf(pdu.Data)
		if err != nil {
			t.Fatal(err)
		}
		codes = append(codes, code)
		pdus = append(pdus, pdu)
	}
}

func modularSum(data []byte) uint32 {
	sum, _ := cfdp.NewChecksum(cfdp.ChecksumModular)
	sum.Update(0, data)
	return sum.Sum()
}

// F1: file data arriving before the Metadata PDU must be buffered and
// replayed, not counted as received while it cannot be written.
func TestFileDataBeforeMetadataIsReplayed(t *testing.T) {
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

	pdus := collectPDUs(t, sender)
	// The worst ordering: every data PDU and the EOF first, metadata last.
	reordered := append(append([]*cfdp.PDU{}, pdus[1:]...), pdus[0])
	for _, pdu := range reordered {
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
	delivered, err := dstFS.Read("dst.dat")
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(delivered, content) {
		t.Error("delivered contents differ after data-before-metadata delivery")
	}
}

// F2: the last NAK-recovered segment must itself trigger the Finished PDU;
// close-out must not wait for another EOF.
func TestLastRecoveredSegmentTriggersFinished(t *testing.T) {
	content := testFile()
	srcFS := cfdp.NewMemoryFilestore()
	if err := srcFS.WriteAt("src.dat", 0, content); err != nil {
		t.Fatal(err)
	}
	dstFS := cfdp.NewMemoryFilestore()

	sender, err := cfdp.NewSender(srcFS, senderConfig(true))
	if err != nil {
		t.Fatal(err)
	}
	receiver := cfdp.NewReceiver(dstFS, receiverConfig(true))

	for i, pdu := range collectPDUs(t, sender) {
		if i == 2 {
			continue // one data segment lost in transit
		}
		if err := receiver.HandlePDU(pdu); err != nil {
			t.Fatal(err)
		}
	}

	// The receiver owes an ACK of the EOF and a NAK for the gap.
	codes, pdus := drainDirectives(t, receiver)
	if len(codes) != 2 || codes[0] != cfdp.DirectiveACK || codes[1] != cfdp.DirectiveNAK {
		t.Fatalf("after EOF with a gap, queued %v, want [ACK NAK]", codes)
	}
	if err := sender.HandlePDU(pdus[1]); err != nil {
		t.Fatal(err)
	}

	// The sender retransmits the missing segment; handling it must queue the
	// Finished PDU immediately.
	resent, ok, err := sender.NextPDU()
	if err != nil || !ok {
		t.Fatalf("sender did not retransmit after the NAK (ok=%t err=%v)", ok, err)
	}
	if err := receiver.HandlePDU(resent); err != nil {
		t.Fatal(err)
	}

	codes, pdus = drainDirectives(t, receiver)
	if len(codes) != 1 || codes[0] != cfdp.DirectiveFinished {
		t.Fatalf("after the gap closed, queued %v, want [Finished]", codes)
	}
	fin, err := cfdp.DecodeFinishedPDU(pdus[0].Data)
	if err != nil {
		t.Fatal(err)
	}
	if fin.DeliveryCode != cfdp.DeliveryDataComplete || fin.ConditionCode != cfdp.CondNoError {
		t.Errorf("Finished = condition %s delivery %d, want no error / complete", fin.ConditionCode, fin.DeliveryCode)
	}
}

// F3: an EOF carrying a fault condition is an EOF (cancel): the receiver must
// ACK it, answer with Finished (delivery incomplete), and stop NAKing.
func TestRemoteCancelTerminatesReceive(t *testing.T) {
	content := testFile()
	srcFS := cfdp.NewMemoryFilestore()
	if err := srcFS.WriteAt("src.dat", 0, content); err != nil {
		t.Fatal(err)
	}
	dstFS := cfdp.NewMemoryFilestore()

	sender, err := cfdp.NewSender(srcFS, senderConfig(true))
	if err != nil {
		t.Fatal(err)
	}
	receiver := cfdp.NewReceiver(dstFS, receiverConfig(true))

	// Metadata and two data segments arrive, then the sender cancels.
	for i, pdu := range collectPDUs(t, sender) {
		if i >= 3 {
			break
		}
		if err := receiver.HandlePDU(pdu); err != nil {
			t.Fatal(err)
		}
	}
	eof := &cfdp.EOFPDU{ConditionCode: cfdp.CondCancelRequestReceived, FileSize: 128}
	body, err := eof.Encode(false)
	if err != nil {
		t.Fatal(err)
	}
	if err := receiver.HandlePDU(&cfdp.PDU{Header: toReceiver(true, false), Data: body}); err != nil {
		t.Fatal(err)
	}

	codes, pdus := drainDirectives(t, receiver)
	if len(codes) != 2 || codes[0] != cfdp.DirectiveACK || codes[1] != cfdp.DirectiveFinished {
		t.Fatalf("after EOF (cancel), queued %v, want [ACK Finished] and no NAK", codes)
	}
	fin, err := cfdp.DecodeFinishedPDU(pdus[1].Data)
	if err != nil {
		t.Fatal(err)
	}
	if fin.ConditionCode != cfdp.CondCancelRequestReceived {
		t.Errorf("Finished condition = %s, want cancel request received", fin.ConditionCode)
	}
	if fin.DeliveryCode != cfdp.DeliveryDataIncomplete {
		t.Error("Finished after a cancel must report delivery incomplete")
	}
	if got := receiver.ConditionCode(); got != cfdp.CondCancelRequestReceived {
		t.Errorf("condition = %s, want cancel request received", got)
	}

	// Clause 4.11.1.2: the partial file is discarded and no more NAKs go out.
	if dstFS.Exists("dst.dat") {
		t.Error("the partial file survived the cancel")
	}
	if err := receiver.RequestNAK(); err != nil {
		t.Fatal(err)
	}
	if _, ok, _ := receiver.NextPDU(); ok {
		t.Error("a cancelled receiver still emits NAKs")
	}
}

// F9: the receiver can cancel a transaction itself.
func TestReceiverCancel(t *testing.T) {
	content := testFile()
	srcFS := cfdp.NewMemoryFilestore()
	if err := srcFS.WriteAt("src.dat", 0, content); err != nil {
		t.Fatal(err)
	}
	dstFS := cfdp.NewMemoryFilestore()

	sender, err := cfdp.NewSender(srcFS, senderConfig(true))
	if err != nil {
		t.Fatal(err)
	}
	receiver := cfdp.NewReceiver(dstFS, receiverConfig(true))

	pdus := collectPDUs(t, sender)
	for _, pdu := range pdus[:3] {
		if err := receiver.HandlePDU(pdu); err != nil {
			t.Fatal(err)
		}
	}

	receiver.Cancel()

	codes, finPDUs := drainDirectives(t, receiver)
	if len(codes) != 1 || codes[0] != cfdp.DirectiveFinished {
		t.Fatalf("after Cancel, queued %v, want [Finished]", codes)
	}
	fin, err := cfdp.DecodeFinishedPDU(finPDUs[0].Data)
	if err != nil {
		t.Fatal(err)
	}
	if fin.ConditionCode != cfdp.CondCancelRequestReceived || fin.DeliveryCode != cfdp.DeliveryDataIncomplete {
		t.Errorf("Finished = condition %s delivery %d, want cancel / incomplete", fin.ConditionCode, fin.DeliveryCode)
	}

	// The Finished (cancel) stops the sender mid-stream.
	if err := sender.HandlePDU(finPDUs[0]); err != nil {
		t.Fatal(err)
	}
	if !sender.Done() || sender.State() != cfdp.StateCancelled {
		t.Errorf("sender state = %s, want cancelled", sender.State())
	}

	// And the close-out handshake still completes.
	ack, ok, err := sender.AckFinished()
	if err != nil || !ok {
		t.Fatalf("sender owes an ACK of the Finished PDU (ok=%t err=%v)", ok, err)
	}
	if err := receiver.HandlePDU(ack); err != nil {
		t.Fatal(err)
	}
	if !receiver.Done() || receiver.State() != cfdp.StateCancelled {
		t.Errorf("receiver state = %s, want cancelled", receiver.State())
	}
}

// F4: the four fault dispositions of clause 4.8, exercised on a checksum failure.
func TestFaultDispositions(t *testing.T) {
	run := func(t *testing.T, handler cfdp.FaultHandler, configure bool) (*cfdp.Receiver, *cfdp.MemoryFilestore) {
		t.Helper()
		content := testFile()
		srcFS := cfdp.NewMemoryFilestore()
		if err := srcFS.WriteAt("src.dat", 0, content); err != nil {
			t.Fatal(err)
		}
		dstFS := cfdp.NewMemoryFilestore()

		sender, err := cfdp.NewSender(srcFS, senderConfig(true))
		if err != nil {
			t.Fatal(err)
		}
		config := receiverConfig(true)
		if configure {
			config.FaultHandlers = map[cfdp.ConditionCode]cfdp.FaultHandler{
				cfdp.CondFileChecksumFailure: handler,
			}
		}
		receiver := cfdp.NewReceiver(dstFS, config)

		for _, pdu := range collectPDUs(t, sender) {
			if pdu.Header.IsFileData {
				pdu.Data[len(pdu.Data)-1] ^= 0xFF // corrupt in flight
			}
			if err := receiver.HandlePDU(pdu); err != nil {
				t.Fatal(err)
			}
		}
		return receiver, dstFS
	}

	t.Run("default cancel", func(t *testing.T) {
		receiver, dstFS := run(t, 0, false)
		if got := receiver.ConditionCode(); got != cfdp.CondFileChecksumFailure {
			t.Errorf("condition = %s, want file checksum failure", got)
		}
		codes, _ := drainDirectives(t, receiver)
		if len(codes) != 2 || codes[0] != cfdp.DirectiveACK || codes[1] != cfdp.DirectiveFinished {
			t.Errorf("queued %v, want [ACK Finished]", codes)
		}
		if dstFS.Exists("dst.dat") {
			t.Error("the corrupt file survived the cancel")
		}
	})

	t.Run("ignore", func(t *testing.T) {
		receiver, dstFS := run(t, cfdp.FaultHandlerIgnore, true)
		if got := receiver.ConditionCode(); got != cfdp.CondNoError {
			t.Errorf("condition = %s, want no error when the fault is ignored", got)
		}
		_, pdus := drainDirectives(t, receiver)
		fin, err := cfdp.DecodeFinishedPDU(pdus[len(pdus)-1].Data)
		if err != nil {
			t.Fatal(err)
		}
		if fin.DeliveryCode != cfdp.DeliveryDataComplete || fin.FileStatus != cfdp.FileRetainedSuccessfully {
			t.Error("an ignored checksum failure must still deliver the file")
		}
		if !dstFS.Exists("dst.dat") {
			t.Error("the file was not retained")
		}
	})

	t.Run("abandon", func(t *testing.T) {
		receiver, _ := run(t, cfdp.FaultHandlerAbandon, true)
		if codes, _ := drainDirectives(t, receiver); len(codes) != 0 {
			t.Errorf("an abandoned transaction emitted %v, want nothing", codes)
		}
		if !receiver.Done() || receiver.State() != cfdp.StateCancelled {
			t.Errorf("state = %s, want cancelled", receiver.State())
		}
	})

	t.Run("suspend", func(t *testing.T) {
		receiver, _ := run(t, cfdp.FaultHandlerSuspend, true)
		if !receiver.Suspended() {
			t.Error("the suspend handler did not suspend the transaction")
		}
		if got := receiver.ConditionCode(); got != cfdp.CondFileChecksumFailure {
			t.Errorf("condition = %s, want file checksum failure", got)
		}
		if _, ok, _ := receiver.NextPDU(); ok {
			t.Error("a suspended transaction emitted a PDU")
		}
	})
}

// F4: a fault handler override TLV in the Metadata PDU changes the
// receiver's disposition.
func TestFaultHandlerOverrideTLVApplied(t *testing.T) {
	content := testFile()
	srcFS := cfdp.NewMemoryFilestore()
	if err := srcFS.WriteAt("src.dat", 0, content); err != nil {
		t.Fatal(err)
	}
	dstFS := cfdp.NewMemoryFilestore()

	config := senderConfig(false)
	config.FaultHandlerOverrides = map[cfdp.ConditionCode]cfdp.FaultHandler{
		cfdp.CondFileChecksumFailure: cfdp.FaultHandlerIgnore,
	}
	sender, err := cfdp.NewSender(srcFS, config)
	if err != nil {
		t.Fatal(err)
	}
	receiver := cfdp.NewReceiver(dstFS, receiverConfig(false))

	for _, pdu := range collectPDUs(t, sender) {
		if pdu.Header.IsFileData {
			pdu.Data[len(pdu.Data)-1] ^= 0xFF
		}
		raw, err := pdu.Encode()
		if err != nil {
			t.Fatal(err)
		}
		decoded, err := cfdp.DecodePDU(raw)
		if err != nil {
			t.Fatal(err)
		}
		if err := receiver.HandlePDU(decoded); err != nil {
			t.Fatal(err)
		}
	}

	if got := receiver.ConditionCode(); got != cfdp.CondNoError {
		t.Errorf("condition = %s, want no error. The override TLV was not applied", got)
	}
	if !receiver.Complete() {
		t.Error("transfer incomplete")
	}
	if !dstFS.Exists("dst.dat") {
		t.Error("the file was not retained under the ignore override")
	}
}

// F5: the caller can expire the check limit, forcing the Finished PDU a
// Class 1 closure-requested transaction owes even though the EOF never came.
func TestExpireCheckLimitForcesFinished(t *testing.T) {
	content := testFile()
	srcFS := cfdp.NewMemoryFilestore()
	if err := srcFS.WriteAt("src.dat", 0, content); err != nil {
		t.Fatal(err)
	}
	dstFS := cfdp.NewMemoryFilestore()

	config := senderConfig(false)
	config.ClosureRequested = true
	sender, err := cfdp.NewSender(srcFS, config)
	if err != nil {
		t.Fatal(err)
	}
	receiver := cfdp.NewReceiver(dstFS, receiverConfig(false))

	pdus := collectPDUs(t, sender)
	for _, pdu := range pdus[:len(pdus)-1] { // the EOF is lost
		if err := receiver.HandlePDU(pdu); err != nil {
			t.Fatal(err)
		}
	}

	receiver.ExpireCheckLimit()

	codes, finPDUs := drainDirectives(t, receiver)
	if len(codes) != 1 || codes[0] != cfdp.DirectiveFinished {
		t.Fatalf("after the check limit expired, queued %v, want [Finished]", codes)
	}
	fin, err := cfdp.DecodeFinishedPDU(finPDUs[0].Data)
	if err != nil {
		t.Fatal(err)
	}
	if fin.ConditionCode != cfdp.CondCheckLimitReached || fin.DeliveryCode != cfdp.DeliveryDataIncomplete {
		t.Errorf("Finished = condition %s delivery %d, want check limit reached / incomplete", fin.ConditionCode, fin.DeliveryCode)
	}
	if !receiver.Done() {
		t.Errorf("state = %s, want a terminal state", receiver.State())
	}
}

// F6: retransmissions that overlap data already received must fold only the
// new sub-ranges into the file and the checksum.
func TestOverlappingRetransmissionsDoNotCorruptChecksum(t *testing.T) {
	content := make([]byte, 200)
	for i := range content {
		content[i] = byte(i * 13)
	}
	dstFS := cfdp.NewMemoryFilestore()
	receiver := cfdp.NewReceiver(dstFS, receiverConfig(false))

	meta := &cfdp.MetadataPDU{
		ChecksumType:        cfdp.ChecksumModular,
		FileSize:            200,
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

	// Two segments overlapping on [80, 120).
	for _, seg := range []struct{ start, end uint64 }{{0, 120}, {80, 200}} {
		fd := &cfdp.FileDataPDU{Offset: seg.start, Data: content[seg.start:seg.end]}
		body, err := fd.Encode(false, false)
		if err != nil {
			t.Fatal(err)
		}
		if err := receiver.HandlePDU(&cfdp.PDU{Header: toReceiver(false, true), Data: body}); err != nil {
			t.Fatal(err)
		}
	}

	eof := &cfdp.EOFPDU{FileChecksum: modularSum(content), FileSize: 200}
	body, err = eof.Encode(false)
	if err != nil {
		t.Fatal(err)
	}
	if err := receiver.HandlePDU(&cfdp.PDU{Header: toReceiver(false, false), Data: body}); err != nil {
		t.Fatal(err)
	}

	if got := receiver.ConditionCode(); got != cfdp.CondNoError {
		t.Errorf("condition = %s, want no error. The overlap was folded in twice", got)
	}
	delivered, err := dstFS.Read("dst.dat")
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(delivered, content) {
		t.Error("delivered contents differ after overlapping delivery")
	}
}

// F8: PDUs carrying a foreign source entity ID or transaction sequence number
// must not be applied to this transaction.
func TestForeignPDUsAreIgnored(t *testing.T) {
	content := testFile()
	srcFS := cfdp.NewMemoryFilestore()
	if err := srcFS.WriteAt("src.dat", 0, content); err != nil {
		t.Fatal(err)
	}
	dstFS := cfdp.NewMemoryFilestore()

	sender, err := cfdp.NewSender(srcFS, senderConfig(true))
	if err != nil {
		t.Fatal(err)
	}
	receiver := cfdp.NewReceiver(dstFS, receiverConfig(true))

	pdus := collectPDUs(t, sender)
	for _, pdu := range pdus[:2] { // metadata and the first segment
		if err := receiver.HandlePDU(pdu); err != nil {
			t.Fatal(err)
		}
	}

	foreign := toReceiver(true, true)
	foreign.Source = cfdp.EntityID{Value: 9, Width: 1}

	// Foreign file data over the next gap, with different content: if it were
	// accepted, the checksum below would fail.
	junk := bytes.Repeat([]byte{0xFF}, 64)
	fd := &cfdp.FileDataPDU{Offset: 64, Data: junk}
	body, err := fd.Encode(false, false)
	if err != nil {
		t.Fatal(err)
	}
	if err := receiver.HandlePDU(&cfdp.PDU{Header: foreign, Data: body}); err != nil {
		t.Fatal(err)
	}

	// A foreign EOF must draw no ACK and close nothing.
	foreignEOF := toReceiver(true, false)
	foreignEOF.Source = cfdp.EntityID{Value: 9, Width: 1}
	eof := &cfdp.EOFPDU{FileChecksum: 1, FileSize: 64}
	body, err = eof.Encode(false)
	if err != nil {
		t.Fatal(err)
	}
	if err := receiver.HandlePDU(&cfdp.PDU{Header: foreignEOF, Data: body}); err != nil {
		t.Fatal(err)
	}
	if receiver.Complete() {
		t.Fatal("a foreign EOF completed the transaction")
	}
	if _, ok, _ := receiver.NextPDU(); ok {
		t.Fatal("a foreign EOF was acknowledged")
	}

	// The genuine stream still delivers cleanly.
	for _, pdu := range pdus[2:] {
		if err := receiver.HandlePDU(pdu); err != nil {
			t.Fatal(err)
		}
	}
	if !receiver.Complete() || receiver.ConditionCode() != cfdp.CondNoError {
		t.Fatalf("clean delivery failed: condition %s, missing %+v", receiver.ConditionCode(), receiver.MissingSegments())
	}

	// The sender ignores a foreign NAK too.
	foreignNAK := &cfdp.NAKPDU{EndOfScope: 500, Requests: []cfdp.SegmentRequest{{StartOffset: 0, EndOffset: 64}}}
	body, err = foreignNAK.Encode(false)
	if err != nil {
		t.Fatal(err)
	}
	header := &cfdp.PDUHeader{
		Direction:      cfdp.TowardSender,
		Acknowledged:   true,
		Source:         cfdp.EntityID{Value: 9, Width: 1},
		TransactionSeq: cfdp.EntityID{Value: 7, Width: 2},
		Destination:    cfdp.EntityID{Value: 2, Width: 1},
	}
	if err := sender.HandlePDU(&cfdp.PDU{Header: header, Data: body}); err != nil {
		t.Fatal(err)
	}
	if _, ok, _ := sender.NextPDU(); ok {
		t.Error("the sender retransmitted for a foreign NAK")
	}
}

// F7: a cancelled sender's EOF reports the transaction's progress, not the
// full file size and checksum.
func TestCancelledEOFCarriesProgress(t *testing.T) {
	content := testFile()
	srcFS := cfdp.NewMemoryFilestore()
	if err := srcFS.WriteAt("src.dat", 0, content); err != nil {
		t.Fatal(err)
	}

	sender, err := cfdp.NewSender(srcFS, senderConfig(false))
	if err != nil {
		t.Fatal(err)
	}

	// Metadata plus two 64-octet segments go out, then the cancel.
	for i := 0; i < 3; i++ {
		if _, ok, _ := sender.NextPDU(); !ok {
			t.Fatal("expected a PDU")
		}
	}
	sender.Cancel()

	pdu, ok, err := sender.NextPDU()
	if err != nil || !ok {
		t.Fatalf("expected the EOF (cancel) (ok=%t err=%v)", ok, err)
	}
	eof, err := cfdp.DecodeEOFPDU(pdu.Data, pdu.Header.LargeFile)
	if err != nil {
		t.Fatal(err)
	}
	if eof.ConditionCode != cfdp.CondCancelRequestReceived {
		t.Errorf("condition = %s, want cancel request received", eof.ConditionCode)
	}
	if eof.FileSize != 128 {
		t.Errorf("EOF (cancel) file size = %d, want the progress 128", eof.FileSize)
	}
	if want := modularSum(content[:128]); eof.FileChecksum != want {
		t.Errorf("EOF (cancel) checksum = %#08x, want %#08x over the data sent", eof.FileChecksum, want)
	}
	if !sender.Done() || sender.State() != cfdp.StateCancelled {
		t.Errorf("sender state = %s, want cancelled", sender.State())
	}
}

// F12: in acknowledged mode the closure request bit is transmitted as '0'
// (table 5-9).
func TestAcknowledgedModeClearsClosureBit(t *testing.T) {
	srcFS := cfdp.NewMemoryFilestore()
	if err := srcFS.WriteAt("src.dat", 0, testFile()); err != nil {
		t.Fatal(err)
	}
	config := senderConfig(true)
	config.ClosureRequested = true
	sender, err := cfdp.NewSender(srcFS, config)
	if err != nil {
		t.Fatal(err)
	}

	pdu, ok, err := sender.NextPDU()
	if err != nil || !ok {
		t.Fatal("expected the metadata PDU")
	}
	meta, err := cfdp.DecodeMetadataPDU(pdu.Data, pdu.Header.LargeFile)
	if err != nil {
		t.Fatal(err)
	}
	if meta.ClosureRequested {
		t.Error("the closure request bit was transmitted as '1' in acknowledged mode")
	}
}

// F13: limit faults the caller's timers detect can now be raised, and take
// the table 4-1 route.
func TestDeclareFault(t *testing.T) {
	t.Run("sender positive ACK limit", func(t *testing.T) {
		srcFS := cfdp.NewMemoryFilestore()
		if err := srcFS.WriteAt("src.dat", 0, testFile()); err != nil {
			t.Fatal(err)
		}
		sender, err := cfdp.NewSender(srcFS, senderConfig(true))
		if err != nil {
			t.Fatal(err)
		}
		collectPDUs(t, sender) // metadata, data, EOF: now awaiting the EOF ACK

		sender.DeclareFault(cfdp.CondPositiveACKLimitReached)

		pdu, ok, err := sender.NextPDU()
		if err != nil || !ok {
			t.Fatalf("expected an EOF (cancel) (ok=%t err=%v)", ok, err)
		}
		eof, err := cfdp.DecodeEOFPDU(pdu.Data, pdu.Header.LargeFile)
		if err != nil {
			t.Fatal(err)
		}
		if eof.ConditionCode != cfdp.CondPositiveACKLimitReached {
			t.Errorf("condition = %s, want positive ACK limit reached", eof.ConditionCode)
		}
	})

	t.Run("receiver NAK limit", func(t *testing.T) {
		content := testFile()
		srcFS := cfdp.NewMemoryFilestore()
		if err := srcFS.WriteAt("src.dat", 0, content); err != nil {
			t.Fatal(err)
		}
		dstFS := cfdp.NewMemoryFilestore()
		sender, err := cfdp.NewSender(srcFS, senderConfig(true))
		if err != nil {
			t.Fatal(err)
		}
		receiver := cfdp.NewReceiver(dstFS, receiverConfig(true))
		for _, pdu := range collectPDUs(t, sender)[:3] {
			if err := receiver.HandlePDU(pdu); err != nil {
				t.Fatal(err)
			}
		}

		receiver.DeclareFault(cfdp.CondNAKLimitReached)

		codes, pdus := drainDirectives(t, receiver)
		if len(codes) != 1 || codes[0] != cfdp.DirectiveFinished {
			t.Fatalf("queued %v, want [Finished]", codes)
		}
		fin, err := cfdp.DecodeFinishedPDU(pdus[0].Data)
		if err != nil {
			t.Fatal(err)
		}
		if fin.ConditionCode != cfdp.CondNAKLimitReached || fin.DeliveryCode != cfdp.DeliveryDataIncomplete {
			t.Errorf("Finished = condition %s delivery %d, want NAK limit reached / incomplete", fin.ConditionCode, fin.DeliveryCode)
		}
	})
}

// F10: data received beyond the file size the EOF declares is a file size
// error even when it arrived before the EOF.
func TestFileSizeErrorDetectedAtEOF(t *testing.T) {
	dstFS := cfdp.NewMemoryFilestore()
	receiver := cfdp.NewReceiver(dstFS, receiverConfig(false))

	meta := &cfdp.MetadataPDU{
		ChecksumType:        cfdp.ChecksumModular,
		FileSize:            100,
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

	fd := &cfdp.FileDataPDU{Offset: 0, Data: make([]byte, 150)}
	body, err = fd.Encode(false, false)
	if err != nil {
		t.Fatal(err)
	}
	if err := receiver.HandlePDU(&cfdp.PDU{Header: toReceiver(false, true), Data: body}); err != nil {
		t.Fatal(err)
	}

	eof := &cfdp.EOFPDU{FileChecksum: 0, FileSize: 100}
	body, err = eof.Encode(false)
	if err != nil {
		t.Fatal(err)
	}
	err = receiver.HandlePDU(&cfdp.PDU{Header: toReceiver(false, false), Data: body})
	if !errors.Is(err, cfdp.ErrFileSizeError) {
		t.Fatalf("HandlePDU(EOF) = %v, want ErrFileSizeError", err)
	}
	if got := receiver.ConditionCode(); got != cfdp.CondFileSizeError {
		t.Errorf("condition = %s, want file size error", got)
	}
}

// F11: an unsupported checksum type raises the fault; under the default
// handler the transaction cancels consistently, and under an ignore override
// the transfer proceeds unverified with the condition reported.
func TestUnsupportedChecksumTypeFault(t *testing.T) {
	metaBody := func(t *testing.T) []byte {
		t.Helper()
		meta := &cfdp.MetadataPDU{
			ClosureRequested:    true,
			ChecksumType:        9, // not implemented here
			FileSize:            8,
			SourceFileName:      cfdp.LV{Value: []byte("src.dat")},
			DestinationFileName: cfdp.LV{Value: []byte("dst.dat")},
		}
		body, err := meta.Encode(false)
		if err != nil {
			t.Fatal(err)
		}
		return body
	}

	t.Run("default cancel", func(t *testing.T) {
		dstFS := cfdp.NewMemoryFilestore()
		receiver := cfdp.NewReceiver(dstFS, receiverConfig(true))

		err := receiver.HandlePDU(&cfdp.PDU{Header: toReceiver(true, false), Data: metaBody(t)})
		if !errors.Is(err, cfdp.ErrUnsupportedChecksumType) {
			t.Fatalf("HandlePDU(metadata) = %v, want ErrUnsupportedChecksumType", err)
		}

		// Data after the cancel must not be written anywhere.
		fd := &cfdp.FileDataPDU{Offset: 0, Data: []byte("12345678")}
		body, err := fd.Encode(false, false)
		if err != nil {
			t.Fatal(err)
		}
		if err := receiver.HandlePDU(&cfdp.PDU{Header: toReceiver(true, true), Data: body}); err != nil {
			t.Fatal(err)
		}
		if dstFS.Exists("dst.dat") {
			t.Error("the file was written despite the cancel")
		}

		codes, pdus := drainDirectives(t, receiver)
		if len(codes) != 1 || codes[0] != cfdp.DirectiveFinished {
			t.Fatalf("queued %v, want [Finished]", codes)
		}
		fin, err := cfdp.DecodeFinishedPDU(pdus[0].Data)
		if err != nil {
			t.Fatal(err)
		}
		if fin.ConditionCode != cfdp.CondUnsupportedChecksumType || fin.DeliveryCode != cfdp.DeliveryDataIncomplete {
			t.Errorf("Finished = condition %s delivery %d, want unsupported checksum type / incomplete", fin.ConditionCode, fin.DeliveryCode)
		}
		if fin.FaultLocation != nil {
			t.Error("table 5-7 omits the fault location for an unsupported checksum type")
		}
	})

	t.Run("ignore proceeds with the null checksum", func(t *testing.T) {
		content := []byte("12345678")
		dstFS := cfdp.NewMemoryFilestore()
		config := receiverConfig(false)
		config.FaultHandlers = map[cfdp.ConditionCode]cfdp.FaultHandler{
			cfdp.CondUnsupportedChecksumType: cfdp.FaultHandlerIgnore,
		}
		receiver := cfdp.NewReceiver(dstFS, config)

		if err := receiver.HandlePDU(&cfdp.PDU{Header: toReceiver(false, false), Data: metaBody(t)}); err != nil {
			t.Fatal(err)
		}
		fd := &cfdp.FileDataPDU{Offset: 0, Data: content}
		body, err := fd.Encode(false, false)
		if err != nil {
			t.Fatal(err)
		}
		if err := receiver.HandlePDU(&cfdp.PDU{Header: toReceiver(false, true), Data: body}); err != nil {
			t.Fatal(err)
		}
		eof := &cfdp.EOFPDU{FileChecksum: 0xDEADBEEF, FileSize: 8}
		body, err = eof.Encode(false)
		if err != nil {
			t.Fatal(err)
		}
		if err := receiver.HandlePDU(&cfdp.PDU{Header: toReceiver(false, false), Data: body}); err != nil {
			t.Fatal(err)
		}

		if !receiver.Complete() {
			t.Fatal("the ignored fault did not let the transfer complete")
		}
		codes, pdus := drainDirectives(t, receiver)
		if len(codes) != 1 || codes[0] != cfdp.DirectiveFinished {
			t.Fatalf("queued %v, want [Finished]", codes)
		}
		fin, err := cfdp.DecodeFinishedPDU(pdus[0].Data)
		if err != nil {
			t.Fatal(err)
		}
		if fin.ConditionCode != cfdp.CondUnsupportedChecksumType {
			t.Errorf("Finished condition = %s, want unsupported checksum type", fin.ConditionCode)
		}
		if fin.DeliveryCode != cfdp.DeliveryDataComplete || fin.FileStatus != cfdp.FileRetainedSuccessfully {
			t.Error("the unverified file must still be delivered complete and retained")
		}
		delivered, err := dstFS.Read("dst.dat")
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(delivered, content) {
			t.Error("delivered contents differ")
		}
	})
}

// The override TLV round-trips.
func TestFaultHandlerOverrideTLVRoundTrip(t *testing.T) {
	tlv, err := cfdp.FaultHandlerOverrideTLV(cfdp.CondNAKLimitReached, cfdp.FaultHandlerSuspend)
	if err != nil {
		t.Fatal(err)
	}
	cond, handler, err := cfdp.DecodeFaultHandlerOverride(tlv)
	if err != nil {
		t.Fatal(err)
	}
	if cond != cfdp.CondNAKLimitReached || handler != cfdp.FaultHandlerSuspend {
		t.Errorf("round trip = (%s, %s)", cond, handler)
	}

	if _, err := cfdp.FaultHandlerOverrideTLV(cfdp.CondNoError, cfdp.FaultHandler(0xC)); !errors.Is(err, cfdp.ErrInvalidFaultHandler) {
		t.Errorf("encoding an invalid handler = %v, want ErrInvalidFaultHandler", err)
	}
}
