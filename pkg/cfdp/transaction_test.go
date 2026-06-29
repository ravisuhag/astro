package cfdp_test

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/ravisuhag/astro/pkg/cfdp"
)

// link carries encoded PDUs between two entities, so every test exercises the
// real wire format rather than passing structs around.
type link struct {
	pdus [][]byte
	// dropAt drops the nth PDU pushed, to simulate loss. -1 drops nothing.
	dropAt int
	pushed int
}

func newLink() *link { return &link{dropAt: -1} }

func (l *link) push(t *testing.T, pdu *cfdp.PDU) {
	t.Helper()
	encoded, err := pdu.Encode()
	if err != nil {
		t.Fatalf("encoding PDU: %v", err)
	}
	if l.pushed == l.dropAt {
		l.pushed++
		return // lost in transit
	}
	l.pushed++
	l.pdus = append(l.pdus, encoded)
}

func (l *link) drain(t *testing.T) []*cfdp.PDU {
	t.Helper()
	var out []*cfdp.PDU
	for _, raw := range l.pdus {
		pdu, err := cfdp.DecodePDU(raw)
		if err != nil {
			t.Fatalf("decoding PDU: %v", err)
		}
		out = append(out, pdu)
	}
	l.pdus = nil
	return out
}

func senderConfig(acknowledged bool) cfdp.SenderConfig {
	return cfdp.SenderConfig{
		Source:              cfdp.EntityID{Value: 1, Width: 1},
		Destination:         cfdp.EntityID{Value: 2, Width: 1},
		TransactionSeq:      cfdp.EntityID{Value: 7, Width: 2},
		Acknowledged:        acknowledged,
		SegmentSize:         64,
		SourceFileName:      "src.dat",
		DestinationFileName: "dst.dat",
		ChecksumType:        cfdp.ChecksumModular,
	}
}

func receiverConfig(acknowledged bool) cfdp.ReceiverConfig {
	return cfdp.ReceiverConfig{
		Source:         cfdp.EntityID{Value: 1, Width: 1},
		Destination:    cfdp.EntityID{Value: 2, Width: 1},
		TransactionSeq: cfdp.EntityID{Value: 7, Width: 2},
		Acknowledged:   acknowledged,
	}
}

// testFile is long enough to need several segments at the 64-octet size above.
func testFile() []byte {
	data := make([]byte, 500)
	for i := range data {
		data[i] = byte(i * 7)
	}
	return data
}

func TestClass1UnacknowledgedTransfer(t *testing.T) {
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

	toReceiver := newLink()
	for {
		pdu, ok, err := sender.NextPDU()
		if err != nil {
			t.Fatal(err)
		}
		if !ok {
			break
		}
		toReceiver.push(t, pdu)
	}

	for _, pdu := range toReceiver.drain(t) {
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
		t.Errorf("delivered %d octets, want %d; contents differ", len(delivered), len(content))
	}
}

func TestClass1WithClosureRequested(t *testing.T) {
	content := []byte("a short file")

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

	toReceiver := newLink()
	for {
		pdu, ok, err := sender.NextPDU()
		if err != nil {
			t.Fatal(err)
		}
		if !ok {
			break
		}
		toReceiver.push(t, pdu)
	}
	for _, pdu := range toReceiver.drain(t) {
		if err := receiver.HandlePDU(pdu); err != nil {
			t.Fatal(err)
		}
	}

	// §5.2.5: closure requested means a Finished PDU comes back.
	back, ok, err := receiver.NextPDU()
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Fatal("expected a Finished PDU when closure was requested")
	}
	code, err := cfdp.DirectiveCodeOf(back.Data)
	if err != nil {
		t.Fatal(err)
	}
	if code != cfdp.DirectiveFinished {
		t.Fatalf("got %s, want Finished", code)
	}

	fin, err := cfdp.DecodeFinishedPDU(back.Data)
	if err != nil {
		t.Fatal(err)
	}
	if fin.DeliveryCode != cfdp.DeliveryDataComplete {
		t.Errorf("delivery code = %d, want data complete", fin.DeliveryCode)
	}
	if fin.ConditionCode != cfdp.CondNoError {
		t.Errorf("condition = %s, want no error", fin.ConditionCode)
	}
}

func TestClass2AcknowledgedTransfer(t *testing.T) {
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

	// Pump both directions until neither has anything to say.
	for round := 0; round < 50; round++ {
		progressed := false

		for {
			pdu, ok, err := sender.NextPDU()
			if err != nil {
				t.Fatal(err)
			}
			if !ok {
				break
			}
			progressed = true
			raw, err := pdu.Encode()
			if err != nil {
				t.Fatal(err)
			}
			decoded, err := cfdp.DecodePDU(raw)
			if err != nil {
				t.Fatal(err)
			}
			if err := receiver.HandlePDU(decoded); err != nil {
				t.Fatalf("receiver rejected a PDU: %v", err)
			}
		}

		for {
			pdu, ok, err := receiver.NextPDU()
			if err != nil {
				t.Fatal(err)
			}
			if !ok {
				break
			}
			progressed = true
			raw, err := pdu.Encode()
			if err != nil {
				t.Fatal(err)
			}
			decoded, err := cfdp.DecodePDU(raw)
			if err != nil {
				t.Fatal(err)
			}
			if err := sender.HandlePDU(decoded); err != nil {
				t.Fatalf("sender rejected a PDU: %v", err)
			}
		}

		// The sender owes an ACK for the Finished PDU.
		if ack, ok, err := sender.AckFinished(); err == nil && ok {
			raw, err := ack.Encode()
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
			progressed = true
		}

		if !progressed {
			break
		}
	}

	if !receiver.Complete() {
		t.Fatalf("transfer incomplete; missing %+v", receiver.MissingSegments())
	}
	if !sender.Done() {
		t.Errorf("sender state = %s, want finished", sender.State())
	}
	if !receiver.Done() {
		t.Errorf("receiver state = %s, want finished", receiver.State())
	}

	delivered, err := dstFS.Read("dst.dat")
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(delivered, content) {
		t.Error("delivered contents differ from the source")
	}
}

func TestClass2RecoversLostSegment(t *testing.T) {
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

	// Drop the third PDU: metadata, data, [lost], data, ...
	dropped := false
	index := 0
	for {
		pdu, ok, err := sender.NextPDU()
		if err != nil {
			t.Fatal(err)
		}
		if !ok {
			break
		}
		if index == 2 && !dropped {
			dropped = true
			index++
			continue // lost in transit
		}
		index++

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
	if !dropped {
		t.Fatal("test did not drop anything")
	}

	// The receiver must notice the gap.
	missing := receiver.MissingSegments()
	if len(missing) == 0 {
		t.Fatal("receiver reported no gap after a segment was lost")
	}
	if receiver.Complete() {
		t.Fatal("receiver claims completion despite a lost segment")
	}

	// Pump the recovery exchange.
	for round := 0; round < 50; round++ {
		progressed := false

		for {
			pdu, ok, err := receiver.NextPDU()
			if err != nil {
				t.Fatal(err)
			}
			if !ok {
				break
			}
			progressed = true
			raw, _ := pdu.Encode()
			decoded, err := cfdp.DecodePDU(raw)
			if err != nil {
				t.Fatal(err)
			}
			if err := sender.HandlePDU(decoded); err != nil {
				t.Fatal(err)
			}
		}

		for {
			pdu, ok, err := sender.NextPDU()
			if err != nil {
				t.Fatal(err)
			}
			if !ok {
				break
			}
			progressed = true
			raw, _ := pdu.Encode()
			decoded, err := cfdp.DecodePDU(raw)
			if err != nil {
				t.Fatal(err)
			}
			if err := receiver.HandlePDU(decoded); err != nil {
				t.Fatal(err)
			}
		}

		if ack, ok, err := sender.AckFinished(); err == nil && ok {
			raw, _ := ack.Encode()
			decoded, err := cfdp.DecodePDU(raw)
			if err != nil {
				t.Fatal(err)
			}
			if err := receiver.HandlePDU(decoded); err != nil {
				t.Fatal(err)
			}
			progressed = true
		}

		if !progressed {
			break
		}
	}

	if !receiver.Complete() {
		t.Fatalf("recovery failed; still missing %+v", receiver.MissingSegments())
	}
	delivered, err := dstFS.Read("dst.dat")
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(delivered, content) {
		t.Error("recovered file differs from the source")
	}
}

func TestReceiverDetectsChecksumFailure(t *testing.T) {
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

	// Corrupt one file data PDU in flight, leaving lengths intact so the gap
	// map still closes and only the checksum can catch it.
	for {
		pdu, ok, err := sender.NextPDU()
		if err != nil {
			t.Fatal(err)
		}
		if !ok {
			break
		}
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

	if got := receiver.ConditionCode(); got != cfdp.CondFileChecksumFailure {
		t.Errorf("condition = %s, want file checksum failure", got)
	}
}

func TestSenderSuspendAndResume(t *testing.T) {
	srcFS := cfdp.NewMemoryFilestore()
	if err := srcFS.WriteAt("src.dat", 0, testFile()); err != nil {
		t.Fatal(err)
	}
	sender, err := cfdp.NewSender(srcFS, senderConfig(false))
	if err != nil {
		t.Fatal(err)
	}

	if _, ok, _ := sender.NextPDU(); !ok {
		t.Fatal("expected a PDU before suspending")
	}

	sender.Suspend()
	if !sender.Suspended() {
		t.Error("Suspended() = false after Suspend()")
	}
	if _, ok, _ := sender.NextPDU(); ok {
		t.Error("a suspended sender emitted a PDU")
	}

	sender.Resume()
	if _, ok, _ := sender.NextPDU(); !ok {
		t.Error("a resumed sender emitted nothing")
	}
}

func TestSenderCancel(t *testing.T) {
	srcFS := cfdp.NewMemoryFilestore()
	if err := srcFS.WriteAt("src.dat", 0, testFile()); err != nil {
		t.Fatal(err)
	}
	sender, err := cfdp.NewSender(srcFS, senderConfig(false))
	if err != nil {
		t.Fatal(err)
	}

	// Metadata out, then cancel mid-stream.
	if _, ok, _ := sender.NextPDU(); !ok {
		t.Fatal("expected the metadata PDU")
	}
	sender.Cancel()

	pdu, ok, err := sender.NextPDU()
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Fatal("expected an EOF PDU after cancelling")
	}
	eof, err := cfdp.DecodeEOFPDU(pdu.Data, pdu.Header.LargeFile)
	if err != nil {
		t.Fatal(err)
	}
	// Table 5-5: cancellation carries condition code '1111'.
	if eof.ConditionCode != cfdp.CondCancelRequestReceived {
		t.Errorf("condition = %s, want cancel request received", eof.ConditionCode)
	}
	if eof.FaultLocation == nil {
		t.Error("a cancelled EOF must carry a fault location")
	}
}

func TestMetadataOnlyTransaction(t *testing.T) {
	// §5.2.5: an empty source filename means no file travels, which is how
	// pure filestore requests are carried.
	srcFS := cfdp.NewMemoryFilestore()
	dstFS := cfdp.NewMemoryFilestore()
	if err := dstFS.Create("doomed.dat"); err != nil {
		t.Fatal(err)
	}

	config := senderConfig(false)
	config.SourceFileName = ""
	config.DestinationFileName = ""
	config.ClosureRequested = true
	config.FilestoreRequests = []cfdp.FilestoreRequest{{
		Action:        cfdp.ActionDeleteFile,
		FirstFileName: cfdp.LV{Value: []byte("doomed.dat")},
	}}

	sender, err := cfdp.NewSender(srcFS, config)
	if err != nil {
		t.Fatal(err)
	}
	receiver := cfdp.NewReceiver(dstFS, receiverConfig(false))

	for {
		pdu, ok, err := sender.NextPDU()
		if err != nil {
			t.Fatal(err)
		}
		if !ok {
			break
		}
		raw, _ := pdu.Encode()
		decoded, err := cfdp.DecodePDU(raw)
		if err != nil {
			t.Fatal(err)
		}
		if err := receiver.HandlePDU(decoded); err != nil {
			t.Fatal(err)
		}
	}

	if dstFS.Exists("doomed.dat") {
		t.Error("the filestore request did not delete the file")
	}

	back, ok, err := receiver.NextPDU()
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Fatal("expected a Finished PDU")
	}
	fin, err := cfdp.DecodeFinishedPDU(back.Data)
	if err != nil {
		t.Fatal(err)
	}
	// Table 5-7: one response TLV per request.
	if len(fin.FilestoreResponses) != 1 {
		t.Fatalf("got %d filestore responses, want 1", len(fin.FilestoreResponses))
	}
	resp, err := cfdp.DecodeFilestoreResponse(fin.FilestoreResponses[0])
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != cfdp.StatusSuccessful {
		t.Errorf("status = %d, want successful", resp.StatusCode)
	}
}

func TestDuplicateSegmentsDoNotCorruptChecksum(t *testing.T) {
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

	// Deliver every PDU twice.
	for {
		pdu, ok, err := sender.NextPDU()
		if err != nil {
			t.Fatal(err)
		}
		if !ok {
			break
		}
		raw, _ := pdu.Encode()
		for i := 0; i < 2; i++ {
			decoded, err := cfdp.DecodePDU(raw)
			if err != nil {
				t.Fatal(err)
			}
			if err := receiver.HandlePDU(decoded); err != nil {
				t.Fatal(err)
			}
		}
	}

	if got := receiver.ConditionCode(); got != cfdp.CondNoError {
		t.Errorf("condition = %s, want no error — duplicates were folded in twice", got)
	}
	delivered, err := dstFS.Read("dst.dat")
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(delivered, content) {
		t.Error("delivered contents differ after duplicate delivery")
	}
}

func TestOSFilestoreRoundTrip(t *testing.T) {
	fs := cfdp.NewOSFilestore(t.TempDir())

	if err := fs.WriteAt("a/b.dat", 0, []byte("hello")); err != nil {
		t.Fatal(err)
	}
	if err := fs.WriteAt("a/b.dat", 5, []byte(" world")); err != nil {
		t.Fatal(err)
	}

	got, err := fs.Read("a/b.dat")
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "hello world" {
		t.Errorf("read %q, want %q", got, "hello world")
	}

	size, err := fs.Size("a/b.dat")
	if err != nil {
		t.Fatal(err)
	}
	if size != 11 {
		t.Errorf("size = %d, want 11", size)
	}

	if err := fs.Rename("a/b.dat", "c.dat"); err != nil {
		t.Fatal(err)
	}
	if fs.Exists("a/b.dat") {
		t.Error("the old name still exists after a rename")
	}
	if err := fs.Delete("c.dat"); err != nil {
		t.Fatal(err)
	}
	if fs.Exists("c.dat") {
		t.Error("the file still exists after a delete")
	}
}

func TestOSFilestoreContainsPathEscapes(t *testing.T) {
	// A filename arriving over a radio link must never write outside the
	// configured root. Leading traversal is stripped rather than rejected, so
	// the check is that nothing lands in the parent directory.
	parent := t.TempDir()
	root := filepath.Join(parent, "store")
	if err := os.MkdirAll(root, 0o750); err != nil {
		t.Fatal(err)
	}
	fs := cfdp.NewOSFilestore(root)

	for _, name := range []string{"../escape.dat", "a/../../escape.dat", "../../escape.dat"} {
		_ = fs.WriteAt(name, 0, []byte("x"))

		if _, err := os.Stat(filepath.Join(parent, "escape.dat")); err == nil {
			t.Fatalf("%q wrote outside the filestore root", name)
		}
	}

	// Everything written must sit under the root.
	entries, err := os.ReadDir(parent)
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		if e.Name() != "store" {
			t.Errorf("unexpected entry outside the root: %s", e.Name())
		}
	}
}
