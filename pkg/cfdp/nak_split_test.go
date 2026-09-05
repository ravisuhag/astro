package cfdp_test

import (
	"testing"

	"github.com/ravisuhag/astro/pkg/cfdp"
)

// S9: a NAK PDU's requests each cost 8 octets (16 in large-file mode), and
// PDU.Encode refuses any data field over 0xFFFF octets. Around 8,000
// non-adjacent gaps, one NAK naming all of them will not encode -- and NAKs
// are the receiver's only way to say what is missing, so an unsplit NAK
// would stall the transfer rather than merely shrink it. This test builds a
// receiver with far more gaps than that and checks the NAK PDUs it queues
// all encode, and together name every gap.
func TestOversizedNAKSplitsAcrossPDUs(t *testing.T) {
	// One-byte segments at every even offset, leaving the odd offset between
	// each pair as its own one-byte gap: plenty of small, non-adjacent gaps
	// without needing to construct or hold a large file in memory.
	const segments = 8500

	dstFS := cfdp.NewMemoryFilestore()
	receiver := cfdp.NewReceiver(dstFS, receiverConfig(true))

	meta := &cfdp.MetadataPDU{
		ChecksumType:        cfdp.ChecksumModular,
		FileSize:            2 * segments,
		SourceFileName:      cfdp.LV{Value: []byte("src.dat")},
		DestinationFileName: cfdp.LV{Value: []byte("dst.dat")},
	}
	body, err := meta.Encode(false)
	if err != nil {
		t.Fatal(err)
	}
	if err := receiver.HandlePDU(&cfdp.PDU{Header: toReceiver(true, false), Data: body}); err != nil {
		t.Fatal(err)
	}

	for i := uint64(0); i < segments; i++ {
		fd := &cfdp.FileDataPDU{Offset: 2 * i, Data: []byte{byte(i)}}
		fdBody, err := fd.Encode(false, false)
		if err != nil {
			t.Fatal(err)
		}
		if err := receiver.HandlePDU(&cfdp.PDU{Header: toReceiver(true, true), Data: fdBody}); err != nil {
			t.Fatalf("segment %d: %v", i, err)
		}
	}

	// Every odd offset between two received bytes is its own gap: enough to
	// force more than one NAK PDU (roughly 8,190 requests fit in one, at the
	// default non-large-file, no-CRC size computed in maxNAKRequests).
	wantGaps := receiver.MissingSegments()
	if len(wantGaps) < 8191 {
		t.Fatalf("set up %d gaps, want > 8190 so this exercises the split", len(wantGaps))
	}

	if err := receiver.RequestNAK(); err != nil {
		t.Fatal(err)
	}

	var naks []*cfdp.PDU
	for {
		pdu, ok, err := receiver.NextPDU()
		if err != nil {
			t.Fatal(err)
		}
		if !ok {
			break
		}
		naks = append(naks, pdu)
	}
	if len(naks) < 2 {
		t.Fatalf("got %d NAK PDU(s), want the gaps split across at least 2", len(naks))
	}

	var allRequests []cfdp.SegmentRequest
	for i, pdu := range naks {
		code, err := cfdp.DirectiveCodeOf(pdu.Data)
		if err != nil {
			t.Fatal(err)
		}
		if code != cfdp.DirectiveNAK {
			t.Fatalf("PDU %d: directive = %s, want NAK", i, code)
		}
		// This is where the unfixed code fails: an oversized NAK cannot be
		// encoded to the wire.
		if _, err := pdu.Encode(); err != nil {
			t.Fatalf("PDU %d failed to encode: %v", i, err)
		}
		nak, err := cfdp.DecodeNAKPDU(pdu.Data, false)
		if err != nil {
			t.Fatal(err)
		}
		allRequests = append(allRequests, nak.Requests...)
	}

	if len(allRequests) != len(wantGaps) {
		t.Fatalf("got %d requests across %d NAK PDUs, want %d", len(allRequests), len(naks), len(wantGaps))
	}
	for i, req := range allRequests {
		if req != wantGaps[i] {
			t.Fatalf("request %d = %+v, want %+v", i, req, wantGaps[i])
		}
	}
}

// A NAK that already fits in one PDU must still come out as exactly one PDU,
// scoped 0 to the limit, matching the pre-split behavior.
func TestSmallNAKStaysOnePDU(t *testing.T) {
	dstFS := cfdp.NewMemoryFilestore()
	receiver := cfdp.NewReceiver(dstFS, receiverConfig(true))

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
	if err := receiver.HandlePDU(&cfdp.PDU{Header: toReceiver(true, false), Data: body}); err != nil {
		t.Fatal(err)
	}
	fd := &cfdp.FileDataPDU{Offset: 50, Data: []byte{1, 2, 3, 4}}
	fdBody, err := fd.Encode(false, false)
	if err != nil {
		t.Fatal(err)
	}
	if err := receiver.HandlePDU(&cfdp.PDU{Header: toReceiver(true, true), Data: fdBody}); err != nil {
		t.Fatal(err)
	}

	if err := receiver.RequestNAK(); err != nil {
		t.Fatal(err)
	}

	pdu, ok, err := receiver.NextPDU()
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Fatal("expected a NAK PDU")
	}
	if _, ok, err := receiver.NextPDU(); err != nil || ok {
		t.Fatalf("expected exactly one NAK PDU, got a second (ok=%v err=%v)", ok, err)
	}

	nak, err := cfdp.DecodeNAKPDU(pdu.Data, false)
	if err != nil {
		t.Fatal(err)
	}
	if nak.StartOfScope != 0 || nak.EndOfScope != 54 {
		t.Errorf("scope = [%d, %d), want [0, 54)", nak.StartOfScope, nak.EndOfScope)
	}
}
