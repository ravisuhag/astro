package pxdl_test

import (
	"bytes"
	"errors"
	"testing"

	"github.com/ravisuhag/astro/pkg/pxdl"
)

func TestFrameRoundTrip(t *testing.T) {
	data := []byte("proximity payload")
	f, err := pxdl.NewTransferFrame(42, 3, data)
	if err != nil {
		t.Fatal(err)
	}

	encoded, err := f.Encode()
	if err != nil {
		t.Fatal(err)
	}
	if len(encoded) != pxdl.HeaderSize+len(data) {
		t.Fatalf("encoded %d octets, want %d", len(encoded), pxdl.HeaderSize+len(data))
	}

	got, err := pxdl.DecodeTransferFrame(encoded)
	if err != nil {
		t.Fatal(err)
	}
	if got.Header.SCID != 42 {
		t.Errorf("SCID = %d, want 42", got.Header.SCID)
	}
	if got.Header.PortID != 3 {
		t.Errorf("port ID = %d, want 3", got.Header.PortID)
	}
	if !bytes.Equal(got.DataField, data) {
		t.Errorf("data = %q, want %q", got.DataField, data)
	}
}

func TestVersionBitsAreBinaryTen(t *testing.T) {
	// Clause 3.2.2.2.2: the version field contains binary '10'.
	f, err := pxdl.NewTransferFrame(1, 0, []byte("x"))
	if err != nil {
		t.Fatal(err)
	}
	encoded, err := f.Encode()
	if err != nil {
		t.Fatal(err)
	}
	if version := encoded[0] >> 6; version != pxdl.Version {
		t.Errorf("version bits = %02b, want 10", version)
	}
}

func TestFrameLengthIsCountLessOne(t *testing.T) {
	// Clause 3.2.2.10.2: the field holds C = total octets - 1.
	data := make([]byte, 100)
	f, err := pxdl.NewTransferFrame(1, 0, data)
	if err != nil {
		t.Fatal(err)
	}
	encoded, err := f.Encode()
	if err != nil {
		t.Fatal(err)
	}

	total := pxdl.HeaderSize + len(data)
	count := uint16(encoded[2]&0x07)<<8 | uint16(encoded[3])
	if int(count) != total-1 {
		t.Errorf("length count = %d, want %d (total %d less one)", count, total-1, total)
	}

	got, err := pxdl.DecodeTransferFrame(encoded)
	if err != nil {
		t.Fatal(err)
	}
	if int(got.Header.FrameLength) != total {
		t.Errorf("decoded frame length = %d, want %d", got.Header.FrameLength, total)
	}
}

func TestHeaderFieldPlacement(t *testing.T) {
	// Every field at the bit position figure 3-3 gives it. Getting one wrong
	// silently corrupts the ones beside it.
	h := pxdl.Header{
		QoS:                 pxdl.Expedited,
		PDUType:             pxdl.UserData,
		DFCID:               pxdl.DFCUserDefined,
		SCID:                0x2AB, // 10 bits, both octets involved
		PCID:                1,
		PortID:              5,
		SourceOrDest:        pxdl.SCIDIsSource,
		FrameLength:         pxdl.HeaderSize,
		FrameSequenceNumber: 0xC7,
	}
	encoded, err := h.Encode()
	if err != nil {
		t.Fatal(err)
	}

	var got pxdl.Header
	if err := got.Decode(encoded); err != nil {
		t.Fatal(err)
	}
	if got.QoS != pxdl.Expedited {
		t.Errorf("QoS = %s, want expedited", got.QoS)
	}
	if got.DFCID != pxdl.DFCUserDefined {
		t.Errorf("DFC ID = %s, want user defined", got.DFCID)
	}
	if got.SCID != 0x2AB {
		t.Errorf("SCID = %#x, want 0x2AB", got.SCID)
	}
	if got.PCID != 1 {
		t.Errorf("PCID = %d, want 1", got.PCID)
	}
	if got.PortID != 5 {
		t.Errorf("port ID = %d, want 5", got.PortID)
	}
	if got.SourceOrDest != pxdl.SCIDIsSource {
		t.Errorf("source-or-dest = %s, want source", got.SourceOrDest)
	}
	if got.FrameSequenceNumber != 0xC7 {
		t.Errorf("sequence number = %#x, want 0xC7", got.FrameSequenceNumber)
	}
}

func TestHeaderValidation(t *testing.T) {
	valid := func() pxdl.Header {
		return pxdl.Header{SCID: 1, FrameLength: pxdl.HeaderSize}
	}
	tests := []struct {
		name    string
		mutate  func(*pxdl.Header)
		wantErr error
	}{
		{"valid", func(*pxdl.Header) {}, nil},
		{"SCID past 10 bits", func(h *pxdl.Header) { h.SCID = 0x400 }, pxdl.ErrInvalidSCID},
		{"PCID past 1 bit", func(h *pxdl.Header) { h.PCID = 2 }, pxdl.ErrInvalidPCID},
		{"port ID past 3 bits", func(h *pxdl.Header) { h.PortID = 8 }, pxdl.ErrInvalidPortID},
		{"frame shorter than the header", func(h *pxdl.Header) { h.FrameLength = 4 }, pxdl.ErrInvalidFrameLength},
		{"frame past the maximum", func(h *pxdl.Header) { h.FrameLength = 2049 }, pxdl.ErrInvalidFrameLength},
		// Clause 3.2.2.5.2: a P-frame's DFC ID is '00'.
		{"P-frame with a DFC ID", func(h *pxdl.Header) {
			h.PDUType = pxdl.SupervisoryData
			h.QoS = pxdl.Expedited
			h.DFCID = pxdl.DFCPackets + 1
		}, pxdl.ErrInvalidDFCID},
		// Clause 3.2.4.1: SPDUs travel only on Expedited.
		{"P-frame on sequence controlled", func(h *pxdl.Header) {
			h.PDUType = pxdl.SupervisoryData
			h.QoS = pxdl.SequenceControlled
		}, pxdl.ErrInvalidQoS},
		// Table 3-1: DFC ID '10' is reserved for future CCSDS definition.
		{"U-frame with the reserved DFC ID", func(h *pxdl.Header) {
			h.DFCID = pxdl.DFCReserved
		}, pxdl.ErrInvalidDFCID},
		// Clause 3.2.2.8.2: a P-frame's Port ID is '0'.
		{"P-frame with a port ID", func(h *pxdl.Header) {
			h.PDUType = pxdl.SupervisoryData
			h.QoS = pxdl.Expedited
			h.PortID = 5
		}, pxdl.ErrPortIDOnSupervisoryFrame},
		// Clause 3.2.2.8.3: a U-frame's Port ID is the routing target, untouched.
		{"U-frame with a port ID", func(h *pxdl.Header) {
			h.PortID = 5
		}, nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := valid()
			tt.mutate(&h)
			err := h.Validate()
			if tt.wantErr == nil {
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				return
			}
			if !errors.Is(err, tt.wantErr) {
				t.Errorf("error = %v, want %v", err, tt.wantErr)
			}
		})
	}
}

func TestDecodeRejectsWrongVersion(t *testing.T) {
	f, err := pxdl.NewTransferFrame(1, 0, []byte("x"))
	if err != nil {
		t.Fatal(err)
	}
	encoded, err := f.Encode()
	if err != nil {
		t.Fatal(err)
	}
	encoded[0] &^= 0xC0 // version '00', not Version-3

	if _, err := pxdl.DecodeTransferFrame(encoded); !errors.Is(err, pxdl.ErrInvalidVersion) {
		t.Errorf("error = %v, want ErrInvalidVersion", err)
	}
}

func TestDecodeRejectsShortInput(t *testing.T) {
	f, err := pxdl.NewTransferFrame(1, 0, []byte("payload"))
	if err != nil {
		t.Fatal(err)
	}
	encoded, err := f.Encode()
	if err != nil {
		t.Fatal(err)
	}
	for cut := 0; cut < len(encoded); cut++ {
		if _, err := pxdl.DecodeTransferFrame(encoded[:cut]); err == nil {
			t.Errorf("length %d: expected an error, got nil", cut)
		}
	}
}

func TestDataFieldSizeLimit(t *testing.T) {
	// Clause 3.2.1 b): up to 2043 octets.
	if _, err := pxdl.NewTransferFrame(1, 0, make([]byte, pxdl.MaxDataFieldSize)); err != nil {
		t.Errorf("the maximum data field was rejected: %v", err)
	}
	if _, err := pxdl.NewTransferFrame(1, 0, make([]byte, pxdl.MaxDataFieldSize+1)); !errors.Is(err, pxdl.ErrDataTooLarge) {
		t.Errorf("error = %v, want ErrDataTooLarge", err)
	}
}

func TestMaximumFrameFitsTheLengthField(t *testing.T) {
	// The 11-bit count tops out at 2047, so a 2048-octet frame is the largest.
	f, err := pxdl.NewTransferFrame(0x3FF, 7, make([]byte, pxdl.MaxDataFieldSize))
	if err != nil {
		t.Fatal(err)
	}
	encoded, err := f.Encode()
	if err != nil {
		t.Fatal(err)
	}
	if len(encoded) != pxdl.MaxFrameSize {
		t.Fatalf("encoded %d octets, want %d", len(encoded), pxdl.MaxFrameSize)
	}
	got, err := pxdl.DecodeTransferFrame(encoded)
	if err != nil {
		t.Fatal(err)
	}
	if got.Header.FrameLength != pxdl.MaxFrameSize {
		t.Errorf("frame length = %d, want %d", got.Header.FrameLength, pxdl.MaxFrameSize)
	}
}

func TestSupervisoryFrameForcesExpedited(t *testing.T) {
	// Clause 3.2.4.1 and clause 3.2.2.5.2 both constrain a P-frame, so the constructor
	// applies them rather than letting a caller build an invalid frame.
	f, err := pxdl.NewSupervisoryFrame(42, 0, []byte{0x80, 0x00},
		pxdl.WithQoS(pxdl.SequenceControlled), pxdl.WithDFCID(pxdl.DFCUserDefined))
	if err != nil {
		t.Fatal(err)
	}
	if f.Header.QoS != pxdl.Expedited {
		t.Error("a supervisory frame must be on the expedited service")
	}
	if f.Header.DFCID != 0 {
		t.Error("a supervisory frame must have a zero DFC ID")
	}
	if !f.IsSupervisoryFrame() {
		t.Error("IsSupervisoryFrame() = false")
	}
}

func TestSupervisoryFrameRejectsEmptySPDURun(t *testing.T) {
	// Clause 3.2.4.1: a P-frame exists to carry SPDUs, so an empty run is refused.
	if _, err := pxdl.NewSupervisoryFrame(42, 0, nil); !errors.Is(err, pxdl.ErrInvalidSPDU) {
		t.Errorf("error = %v, want ErrInvalidSPDU", err)
	}
	if _, err := pxdl.NewSupervisoryFrame(42, 0, []byte{}); !errors.Is(err, pxdl.ErrInvalidSPDU) {
		t.Errorf("error = %v, want ErrInvalidSPDU", err)
	}
}

func TestSupervisoryFrameRejectsPortID(t *testing.T) {
	// Clause 3.2.2.8.2: in a P-frame the Port ID is not used and is set to '0'.
	// A port belongs to a U-frame, so the constructor refuses it instead of
	// zeroing it and sending a frame the caller did not ask for.
	for _, portID := range []uint8{1, 5, 7} {
		_, err := pxdl.NewSupervisoryFrame(42, portID, []byte{0x80, 0x00})
		if !errors.Is(err, pxdl.ErrPortIDOnSupervisoryFrame) {
			t.Errorf("port ID %d: error = %v, want ErrPortIDOnSupervisoryFrame", portID, err)
		}
	}
}

func TestEncodeRejectsPortIDSetOnAPFrameAfterConstruction(t *testing.T) {
	// PDUType and PortID are exported, so the illegal pair can be assembled
	// past the constructor. Encode is the last place to stop it reaching the
	// link (clause 3.2.2.8.2).
	f, err := pxdl.NewSupervisoryFrame(42, 0, []byte{0x80, 0x00})
	if err != nil {
		t.Fatal(err)
	}
	f.Header.PortID = 5
	if _, err := f.Encode(); !errors.Is(err, pxdl.ErrPortIDOnSupervisoryFrame) {
		t.Errorf("error = %v, want ErrPortIDOnSupervisoryFrame", err)
	}

	// The same pair reached from the other side: a U-frame with a port, then
	// switched to supervisory.
	u, err := pxdl.NewTransferFrame(42, 5, []byte("payload"))
	if err != nil {
		t.Fatal(err)
	}
	u.Header.PDUType = pxdl.SupervisoryData
	u.Header.QoS = pxdl.Expedited
	u.Header.DFCID = 0
	if _, err := u.Encode(); !errors.Is(err, pxdl.ErrPortIDOnSupervisoryFrame) {
		t.Errorf("error = %v, want ErrPortIDOnSupervisoryFrame", err)
	}
}

func TestPortIDBitsOnTheWire(t *testing.T) {
	// Clause 3.2.2.8.1 puts the Port ID in bits 17-19, which are bits 4-6 of octet
	// 2. A P-frame must show '000' there; a U-frame must show what it was
	// given, so the zeroing rule cannot quietly spread to user data.
	p, err := pxdl.NewSupervisoryFrame(1, 0, []byte{0x80, 0x00})
	if err != nil {
		t.Fatal(err)
	}
	encoded, err := p.Encode()
	if err != nil {
		t.Fatal(err)
	}
	if encoded[0]>>4&1 != 1 {
		t.Fatalf("octet 0 = %#02x, expected the PDU type bit set for a P-frame", encoded[0])
	}
	if got := encoded[2] >> 4 & 0x07; got != 0 {
		t.Errorf("P-frame port ID bits = %03b, want 000", got)
	}

	u, err := pxdl.NewTransferFrame(1, 5, []byte("payload"))
	if err != nil {
		t.Fatal(err)
	}
	encoded, err = u.Encode()
	if err != nil {
		t.Fatal(err)
	}
	if got := encoded[2] >> 4 & 0x07; got != 5 {
		t.Errorf("U-frame port ID bits = %03b, want 101", got)
	}
	got, err := pxdl.DecodeTransferFrame(encoded)
	if err != nil {
		t.Fatal(err)
	}
	if got.Header.PortID != 5 {
		t.Errorf("decoded U-frame port ID = %d, want 5", got.Header.PortID)
	}
}

func TestSourceOrDestPolarityOnTheWire(t *testing.T) {
	// Clause 3.2.2.9.2, table 3-2: '0' means the SCID names the source spacecraft,
	// '1' means it names the destination. Bit 20 of the header is bit 3 of
	// octet 2. Getting this backwards misroutes every frame, so the wire bit
	// is pinned here, not just round-tripped.
	source := pxdl.Header{SCID: 1, FrameLength: pxdl.HeaderSize,
		SourceOrDest: pxdl.SCIDIsSource}
	encoded, err := source.Encode()
	if err != nil {
		t.Fatal(err)
	}
	if encoded[2]>>3&1 != 0 {
		t.Error("SCIDIsSource encoded bit 20 as 1, table 3-2 says '0' = source")
	}

	dest := pxdl.Header{SCID: 1, FrameLength: pxdl.HeaderSize,
		SourceOrDest: pxdl.SCIDIsDestination}
	encoded, err = dest.Encode()
	if err != nil {
		t.Fatal(err)
	}
	if encoded[2]>>3&1 != 1 {
		t.Error("SCIDIsDestination encoded bit 20 as 0, table 3-2 says '1' = destination")
	}
}

func TestManagedParameters(t *testing.T) {
	m := pxdl.DefaultManagedParameters()
	if err := m.Validate(); err != nil {
		t.Fatalf("default parameters invalid: %v", err)
	}
	if m.SendMaximumFrameLength != pxdl.MaxFrameSize ||
		m.ReceiveMaximumFrameLength != pxdl.MaxFrameSize {
		t.Error("default maximum frame lengths are not the Version-3 bound")
	}

	bad := m
	bad.LocalSpacecraftID = 0x400
	if err := bad.Validate(); !errors.Is(err, pxdl.ErrInvalidSCID) {
		t.Errorf("error = %v, want ErrInvalidSCID", err)
	}

	bad = m
	bad.ReceiveMaximumFrameLength = pxdl.MaxFrameSize + 1
	if err := bad.Validate(); !errors.Is(err, pxdl.ErrInvalidFrameLength) {
		t.Errorf("error = %v, want ErrInvalidFrameLength", err)
	}
}

func TestFrameOptions(t *testing.T) {
	f, err := pxdl.NewTransferFrame(100, 2, []byte("data"),
		pxdl.WithQoS(pxdl.Expedited),
		pxdl.WithDFCID(pxdl.DFCSegment),
		pxdl.WithPCID(1),
		pxdl.WithSourceSCID(),
		pxdl.WithSequenceNumber(77))
	if err != nil {
		t.Fatal(err)
	}

	encoded, err := f.Encode()
	if err != nil {
		t.Fatal(err)
	}
	got, err := pxdl.DecodeTransferFrame(encoded)
	if err != nil {
		t.Fatal(err)
	}
	if got.Header.QoS != pxdl.Expedited {
		t.Error("QoS option lost")
	}
	if got.Header.DFCID != pxdl.DFCSegment {
		t.Error("DFC ID option lost")
	}
	if got.Header.PCID != 1 {
		t.Error("PCID option lost")
	}
	if got.Header.SourceOrDest != pxdl.SCIDIsSource {
		t.Error("source SCID option lost")
	}
	if got.Header.FrameSequenceNumber != 77 {
		t.Error("sequence number option lost")
	}
}
