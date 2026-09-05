package epp_test

import (
	"bytes"
	"errors"
	"strconv"
	"testing"

	"github.com/ravisuhag/astro/pkg/epp"
)

func TestNewIdlePacket(t *testing.T) {
	pkt, err := epp.NewIdlePacket()
	if err != nil {
		t.Fatalf("NewIdlePacket failed: %v", err)
	}
	if !pkt.IsIdle() {
		t.Error("Expected IsIdle()=true")
	}
	if pkt.Header.ProtocolID != epp.ProtocolIDIdle {
		t.Errorf("ProtocolID = %d, want %d", pkt.Header.ProtocolID, epp.ProtocolIDIdle)
	}
	if len(pkt.Data) != 0 {
		t.Errorf("Expected empty data, got %d bytes", len(pkt.Data))
	}
}

// TestIdlePacketWireVector pins the spec-derived 1-octet idle packet: 0xE0.
func TestIdlePacketWireVector(t *testing.T) {
	pkt, err := epp.NewIdlePacket()
	if err != nil {
		t.Fatalf("NewIdlePacket failed: %v", err)
	}

	encoded, err := pkt.Encode()
	if err != nil {
		t.Fatalf("Encode failed: %v", err)
	}
	if !bytes.Equal(encoded, []byte{0xE0}) {
		t.Fatalf("1-octet idle = % X, want E0", encoded)
	}

	decoded, err := epp.Decode(encoded)
	if err != nil {
		t.Fatalf("Decode failed: %v", err)
	}
	if !decoded.IsIdle() {
		t.Error("Decoded packet should be idle")
	}
}

// TestPacketGoldenVectors pins complete packets to spec-derived wire bytes.
func TestPacketGoldenVectors(t *testing.T) {
	ipe, err := epp.NewIPEPacket([]byte{0x61, 0x62, 0x63, 0x64})
	if err != nil {
		t.Fatalf("NewIPEPacket failed: %v", err)
	}
	got, err := ipe.Encode()
	if err != nil {
		t.Fatal(err)
	}
	// PVN '111', PID '010', LoL '01' -> 0xE9; total length 6.
	want := []byte{0xE9, 0x06, 0x61, 0x62, 0x63, 0x64}
	if !bytes.Equal(got, want) {
		t.Errorf("IPE packet = % X, want % X", got, want)
	}

	ltp, err := epp.NewLTPPacket([]byte{0xAA})
	if err != nil {
		t.Fatalf("NewLTPPacket failed: %v", err)
	}
	got, err = ltp.Encode()
	if err != nil {
		t.Fatal(err)
	}
	// PVN '111', PID '001' (LTP per SANA), LoL '01' -> 0xE5; total length 3.
	want = []byte{0xE5, 0x03, 0xAA}
	if !bytes.Equal(got, want) {
		t.Errorf("LTP packet = % X, want % X", got, want)
	}

	mission, err := epp.NewMissionPacket([]byte{0x01, 0x02})
	if err != nil {
		t.Fatalf("NewMissionPacket failed: %v", err)
	}
	got, err = mission.Encode()
	if err != nil {
		t.Fatal(err)
	}
	// PVN '111', PID '111', LoL '01' -> 0xFD; total length 4.
	want = []byte{0xFD, 0x04, 0x01, 0x02}
	if !bytes.Equal(got, want) {
		t.Errorf("Mission packet = % X, want % X", got, want)
	}

	ext, err := epp.NewPacket(epp.ProtocolIDExtended, []byte{0x0A, 0x0B}, epp.WithExtendedProtocolID(3))
	if err != nil {
		t.Fatalf("NewPacket(extended) failed: %v", err)
	}
	got, err = ext.Encode()
	if err != nil {
		t.Fatal(err)
	}
	// PVN '111', PID '110', LoL '10' -> 0xFA; octet 1 = UDF 0 | PIE 3; length 6.
	want = []byte{0xFA, 0x03, 0x00, 0x06, 0x0A, 0x0B}
	if !bytes.Equal(got, want) {
		t.Errorf("Extended packet = % X, want % X", got, want)
	}
}

func TestIdleFillPacket(t *testing.T) {
	tests := []struct {
		total      int
		wantHeader int
	}{
		{1, 1},
		{2, 2},
		{3, 2},
		{255, 2},
		{256, 4},
		{300, 4},
		{65535, 4},
		{65536, 8},
	}

	for _, tt := range tests {
		pkt, err := epp.NewIdleFillPacket(tt.total, 0xAA)
		if err != nil {
			t.Fatalf("NewIdleFillPacket(%d) failed: %v", tt.total, err)
		}
		if !pkt.IsIdle() {
			t.Errorf("NewIdleFillPacket(%d): expected idle", tt.total)
		}
		if got := pkt.Header.Size(); got != tt.wantHeader {
			t.Errorf("NewIdleFillPacket(%d): header size = %d, want %d", tt.total, got, tt.wantHeader)
		}
		encoded, err := pkt.Encode()
		if err != nil {
			t.Fatalf("Encode(%d) failed: %v", tt.total, err)
		}
		if len(encoded) != tt.total {
			t.Errorf("NewIdleFillPacket(%d): encoded %d bytes", tt.total, len(encoded))
		}
		for _, b := range encoded[tt.wantHeader:] {
			if b != 0xAA {
				t.Errorf("NewIdleFillPacket(%d): fill byte = 0x%02X, want 0xAA", tt.total, b)
				break
			}
		}
		decoded, err := epp.Decode(encoded)
		if err != nil {
			t.Fatalf("Decode idle fill (%d) failed: %v", tt.total, err)
		}
		if !decoded.IsIdle() {
			t.Errorf("Decoded idle fill (%d) not idle", tt.total)
		}
	}

	if _, err := epp.NewIdleFillPacket(0, 0x00); !errors.Is(err, epp.ErrInvalidIdleLength) {
		t.Errorf("NewIdleFillPacket(0) = %v, want ErrInvalidIdleLength", err)
	}
}

func TestIdleFillPacketWireVector(t *testing.T) {
	pkt, err := epp.NewIdleFillPacket(6, 0xFF)
	if err != nil {
		t.Fatal(err)
	}
	encoded, err := pkt.Encode()
	if err != nil {
		t.Fatal(err)
	}
	// PVN '111', PID '000', LoL '01' -> 0xE1; total length 6; 4 fill octets.
	want := []byte{0xE1, 0x06, 0xFF, 0xFF, 0xFF, 0xFF}
	if !bytes.Equal(encoded, want) {
		t.Errorf("idle fill = % X, want % X", encoded, want)
	}
}

func TestIdlePacketWithDataAllowed(t *testing.T) {
	// EPP-F5: multi-octet idle packets with fill data are legal.
	pkt, err := epp.NewPacket(epp.ProtocolIDIdle, []byte{0x00, 0x00, 0x00})
	if err != nil {
		t.Fatalf("NewPacket(idle, data) failed: %v", err)
	}
	if !pkt.IsIdle() {
		t.Error("Expected idle")
	}
	if pkt.Header.LengthOfLength != epp.LoL1Octet {
		t.Errorf("LoL = %d, want %d", pkt.Header.LengthOfLength, epp.LoL1Octet)
	}

	encoded, err := pkt.Encode()
	if err != nil {
		t.Fatal(err)
	}
	decoded, err := epp.Decode(encoded)
	if err != nil {
		t.Fatal(err)
	}
	if !decoded.IsIdle() || len(decoded.Data) != 3 {
		t.Errorf("Decoded idle fill: idle=%v dataLen=%d", decoded.IsIdle(), len(decoded.Data))
	}
}

func TestNewIPEPacket(t *testing.T) {
	data := []byte{0x45, 0x00, 0x00, 0x14} // IPv4 header start
	pkt, err := epp.NewIPEPacket(data)
	if err != nil {
		t.Fatalf("NewIPEPacket failed: %v", err)
	}

	if pkt.Header.ProtocolID != epp.ProtocolIDIPE {
		t.Errorf("ProtocolID = %d, want %d", pkt.Header.ProtocolID, epp.ProtocolIDIPE)
	}
	if pkt.Header.Size() != epp.HeaderSize2 {
		t.Errorf("Header size = %d, want 2", pkt.Header.Size())
	}
	if pkt.Header.PacketLength != uint32(2+len(data)) {
		t.Errorf("PacketLength = %d, want %d", pkt.Header.PacketLength, 2+len(data))
	}
}

func TestAutoHeaderSizing(t *testing.T) {
	// 300 bytes of data cannot fit a 1-octet length field; the constructor
	// must pick the 4-octet header on its own (4.1.2.1.2 NOTE).
	data := make([]byte, 300)
	pkt, err := epp.NewIPEPacket(data)
	if err != nil {
		t.Fatalf("NewIPEPacket(300) failed: %v", err)
	}
	if pkt.Header.Size() != epp.HeaderSize4 {
		t.Errorf("Header size = %d, want 4", pkt.Header.Size())
	}
	if pkt.Header.PacketLength != 304 {
		t.Errorf("PacketLength = %d, want 304", pkt.Header.PacketLength)
	}

	// Beyond 65535 total, the 8-octet header is required.
	big := make([]byte, 70000)
	pkt, err = epp.NewIPEPacket(big)
	if err != nil {
		t.Fatalf("NewIPEPacket(70000) failed: %v", err)
	}
	if pkt.Header.Size() != epp.HeaderSize8 {
		t.Errorf("Header size = %d, want 8", pkt.Header.Size())
	}
}

func TestWithLongLength(t *testing.T) {
	pkt, err := epp.NewIPEPacket([]byte{0x01, 0x02}, epp.WithLongLength())
	if err != nil {
		t.Fatalf("NewIPEPacket failed: %v", err)
	}
	if pkt.Header.Size() != epp.HeaderSize4 {
		t.Errorf("Header size = %d, want 4", pkt.Header.Size())
	}

	encoded, err := pkt.Encode()
	if err != nil {
		t.Fatal(err)
	}
	decoded, err := epp.Decode(encoded)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(decoded.Data, []byte{0x01, 0x02}) {
		t.Error("Data mismatch after round-trip")
	}
}

func TestWithUserDefined(t *testing.T) {
	data := []byte{0x01, 0x02}
	pkt, err := epp.NewMissionPacket(data, epp.WithUserDefined(0xE))
	if err != nil {
		t.Fatalf("Failed: %v", err)
	}

	if pkt.Header.Size() != epp.HeaderSize4 {
		t.Errorf("Header size = %d, want 4", pkt.Header.Size())
	}
	if pkt.Header.UserDefined != 0xE {
		t.Errorf("UserDefined = 0x%X, want 0xE", pkt.Header.UserDefined)
	}

	encoded, err := pkt.Encode()
	if err != nil {
		t.Fatal(err)
	}
	decoded, err := epp.Decode(encoded)
	if err != nil {
		t.Fatal(err)
	}
	if decoded.Header.UserDefined != 0xE {
		t.Errorf("Decoded UserDefined = 0x%X, want 0xE", decoded.Header.UserDefined)
	}

	// The field is 4 bits wide.
	if _, err := epp.NewMissionPacket(data, epp.WithUserDefined(0x10)); !errors.Is(err, epp.ErrInvalidUserDefined) {
		t.Errorf("WithUserDefined(16) = %v, want ErrInvalidUserDefined", err)
	}
}

func TestWithExtendedProtocolID(t *testing.T) {
	data := []byte{0x01, 0x02, 0x03}
	pkt, err := epp.NewPacket(epp.ProtocolIDExtended, data, epp.WithExtendedProtocolID(12))
	if err != nil {
		t.Fatalf("NewPacket failed: %v", err)
	}

	if pkt.Header.Size() != epp.HeaderSize4 {
		t.Errorf("Header size = %d, want 4", pkt.Header.Size())
	}

	encoded, err := pkt.Encode()
	if err != nil {
		t.Fatal(err)
	}
	decoded, err := epp.Decode(encoded)
	if err != nil {
		t.Fatal(err)
	}
	if decoded.Header.ExtendedProtocolID != 12 {
		t.Errorf("ExtendedProtocolID = %d, want 12", decoded.Header.ExtendedProtocolID)
	}
	if !bytes.Equal(decoded.Data, data) {
		t.Errorf("Data mismatch")
	}

	// The field is 4 bits wide.
	if _, err := epp.NewPacket(epp.ProtocolIDExtended, data, epp.WithExtendedProtocolID(16)); !errors.Is(err, epp.ErrInvalidExtendedProtocolID) {
		t.Errorf("WithExtendedProtocolID(16) = %v, want ErrInvalidExtendedProtocolID", err)
	}
}

func TestExtendedPIDRequiresLongHeader(t *testing.T) {
	// EPP-F7: PID '110' needs the extension field, so the constructor must
	// pick at least the 4-octet header even without options.
	pkt, err := epp.NewPacket(epp.ProtocolIDExtended, []byte{0x01})
	if err != nil {
		t.Fatalf("NewPacket(extended) failed: %v", err)
	}
	if pkt.Header.Size() < epp.HeaderSize4 {
		t.Errorf("Header size = %d, want >= 4", pkt.Header.Size())
	}
}

func TestWithCCSDSDefined(t *testing.T) {
	data := []byte{0x01, 0x02, 0x03, 0x04, 0x05}
	pkt, err := epp.NewPacket(epp.ProtocolIDExtended, data,
		epp.WithExtendedProtocolID(9),
		epp.WithCCSDSDefined(0x9876),
	)
	if err != nil {
		t.Fatalf("NewPacket failed: %v", err)
	}

	if pkt.Header.Size() != epp.HeaderSize8 {
		t.Errorf("Header size = %d, want 8", pkt.Header.Size())
	}

	encoded, err := pkt.Encode()
	if err != nil {
		t.Fatal(err)
	}
	decoded, err := epp.Decode(encoded)
	if err != nil {
		t.Fatal(err)
	}
	if decoded.Header.ExtendedProtocolID != 9 {
		t.Errorf("ExtendedProtocolID = %d, want 9", decoded.Header.ExtendedProtocolID)
	}
	if decoded.Header.CCSDSDefined != 0x9876 {
		t.Errorf("CCSDSDefined = 0x%04X, want 0x9876", decoded.Header.CCSDSDefined)
	}
	if !bytes.Equal(decoded.Data, data) {
		t.Error("Data mismatch")
	}
}

func TestMax2OctetHeaderData(t *testing.T) {
	// 2-octet header + data, total <= 255
	data := make([]byte, 253) // 2 + 253 = 255
	pkt, err := epp.NewIPEPacket(data)
	if err != nil {
		t.Fatalf("Max 2-octet header packet failed: %v", err)
	}

	if pkt.Header.Size() != epp.HeaderSize2 {
		t.Errorf("Header size = %d, want 2", pkt.Header.Size())
	}

	encoded, err := pkt.Encode()
	if err != nil {
		t.Fatal(err)
	}
	if len(encoded) != 255 {
		t.Errorf("Encoded length = %d, want 255", len(encoded))
	}

	decoded, err := epp.Decode(encoded)
	if err != nil {
		t.Fatal(err)
	}
	if len(decoded.Data) != 253 {
		t.Errorf("Decoded data length = %d, want 253", len(decoded.Data))
	}
}

func TestNonIdleEmptyDataFails(t *testing.T) {
	_, err := epp.NewPacket(epp.ProtocolIDIPE, nil)
	if !errors.Is(err, epp.ErrEmptyData) {
		t.Errorf("Expected ErrEmptyData, got %v", err)
	}

	_, err = epp.NewPacket(epp.ProtocolIDIPE, []byte{})
	if !errors.Is(err, epp.ErrEmptyData) {
		t.Errorf("Expected ErrEmptyData, got %v", err)
	}
}

func TestInvalidProtocolID(t *testing.T) {
	_, err := epp.NewPacket(8, []byte{0x01})
	if !errors.Is(err, epp.ErrInvalidProtocolID) {
		t.Errorf("Expected ErrInvalidProtocolID, got %v", err)
	}
}

func TestDecodeDataTooShort(t *testing.T) {
	_, err := epp.Decode(nil)
	if !errors.Is(err, epp.ErrDataTooShort) {
		t.Errorf("Expected ErrDataTooShort, got %v", err)
	}

	_, err = epp.Decode([]byte{})
	if !errors.Is(err, epp.ErrDataTooShort) {
		t.Errorf("Expected ErrDataTooShort, got %v", err)
	}
}

func TestDecodeTruncatedPacket(t *testing.T) {
	// Create a valid 2-octet-header packet, then truncate
	pkt, _ := epp.NewIPEPacket([]byte{0x01, 0x02, 0x03})
	encoded, _ := pkt.Encode()

	// Truncate: only header, no data
	_, err := epp.Decode(encoded[:2])
	if !errors.Is(err, epp.ErrDataTooShort) {
		t.Errorf("Expected ErrDataTooShort for truncated packet, got %v", err)
	}
}

func TestIsIdle(t *testing.T) {
	idle, _ := epp.NewIdlePacket()
	if !idle.IsIdle() {
		t.Error("Expected IsIdle()=true for idle packet")
	}

	fill, _ := epp.NewIdleFillPacket(10, 0x00)
	if !fill.IsIdle() {
		t.Error("Expected IsIdle()=true for idle fill packet")
	}

	nonIdle, _ := epp.NewIPEPacket([]byte{0x01})
	if nonIdle.IsIdle() {
		t.Error("Expected IsIdle()=false for IPE packet")
	}
}

func TestPacketSizer(t *testing.T) {
	// Idle packet
	idle, _ := epp.NewIdlePacket()
	idleBytes, _ := idle.Encode()
	if got := epp.PacketSizer(idleBytes); got != 1 {
		t.Errorf("PacketSizer(idle) = %d, want 1", got)
	}

	// 2-octet header packet
	f2, _ := epp.NewIPEPacket([]byte{0x01, 0x02, 0x03})
	f2Bytes, _ := f2.Encode()
	if got := epp.PacketSizer(f2Bytes); got != len(f2Bytes) {
		t.Errorf("PacketSizer(2-octet header) = %d, want %d", got, len(f2Bytes))
	}

	// 4-octet header packet
	f4, _ := epp.NewIPEPacket([]byte{0x01, 0x02}, epp.WithLongLength())
	f4Bytes, _ := f4.Encode()
	if got := epp.PacketSizer(f4Bytes); got != len(f4Bytes) {
		t.Errorf("PacketSizer(4-octet header) = %d, want %d", got, len(f4Bytes))
	}

	// 8-octet header packet
	f8, _ := epp.NewPacket(epp.ProtocolIDExtended, []byte{0x01},
		epp.WithExtendedProtocolID(10), epp.WithCCSDSDefined(0))
	f8Bytes, _ := f8.Encode()
	if got := epp.PacketSizer(f8Bytes); got != len(f8Bytes) {
		t.Errorf("PacketSizer(8-octet header) = %d, want %d", got, len(f8Bytes))
	}

	// Idle fill packet
	fill, _ := epp.NewIdleFillPacket(12, 0x55)
	fillBytes, _ := fill.Encode()
	if got := epp.PacketSizer(fillBytes); got != 12 {
		t.Errorf("PacketSizer(idle fill) = %d, want 12", got)
	}

	// Too short
	if got := epp.PacketSizer(nil); got != -1 {
		t.Errorf("PacketSizer(nil) = %d, want -1", got)
	}
	if got := epp.PacketSizer([]byte{}); got != -1 {
		t.Errorf("PacketSizer(empty) = %d, want -1", got)
	}

	// Wrong PVN
	if got := epp.PacketSizer([]byte{0x70, 0x06}); got != -1 {
		t.Errorf("PacketSizer(wrong PVN) = %d, want -1", got)
	}
}

func TestPacketSizerMalformedLength(t *testing.T) {
	// 2-octet header with PacketLength=0 (less than header size 2) -> -1
	data := []byte{0xE9, 0x00}
	if got := epp.PacketSizer(data); got != -1 {
		t.Errorf("PacketSizer(malformed) = %d, want -1", got)
	}

	// 2-octet header with PacketLength=1 (less than header size 2) -> -1
	data = []byte{0xE9, 0x01}
	if got := epp.PacketSizer(data); got != -1 {
		t.Errorf("PacketSizer(malformed) = %d, want -1", got)
	}
}

func TestPacketSizerTruncatedHeader(t *testing.T) {
	// 4-octet header needs 4 bytes, provide only 1
	data := []byte{0xFA} // PVN=7, PID=6, LoL='10'
	if got := epp.PacketSizer(data); got != -1 {
		t.Errorf("PacketSizer(truncated 4-octet header) = %d, want -1", got)
	}
}

func TestPacketSizerRejectsHugeLoL4PacketLength(t *testing.T) {
	// An 8-octet header (LoL '11') only requires PacketLength >= HeaderSize8;
	// nothing caps it below the full 32-bit range, so a peer can declare
	// MaxPacketLength8 (0xFFFFFFFF). On a 32-bit build (int is 32 bits),
	// that value cannot be represented as an int, so PacketSizer must
	// report indeterminate (-1) rather than a spurious wrapped negative.
	// On a 64-bit build the value fits, so PacketSizer must return it
	// unchanged. Either way, the result must never be a negative value
	// other than the -1 sentinel.
	data := make([]byte, epp.HeaderSize8)
	data[0] = (epp.PVN << 5) | (epp.ProtocolIDMission << 2) | epp.LoL4Octet
	data[4], data[5], data[6], data[7] = 0xFF, 0xFF, 0xFF, 0xFF

	got := epp.PacketSizer(data)
	if got < -1 {
		t.Fatalf("PacketSizer(huge LoL4 length) = %d, is a wrapped negative, not the -1 sentinel", got)
	}

	want := -1
	if strconv.IntSize == 64 {
		want = epp.MaxPacketLength8
	}
	if got != want {
		t.Errorf("PacketSizer(huge LoL4 length) = %d, want %d", got, want)
	}
}

func TestHumanize(t *testing.T) {
	pkt, _ := epp.NewIPEPacket([]byte{0x01, 0x02})
	s := pkt.Humanize()
	if s == "" {
		t.Error("Humanize returned empty string")
	}
}

func TestAllProtocolIDsEncodeDecode(t *testing.T) {
	// All PIDs except the extension PID ('110', which needs the extension
	// field) build directly; reserved values 3-5 still encode/decode.
	pids := []uint8{
		epp.ProtocolIDLTP,
		epp.ProtocolIDIPE,
		3, 4, 5,
		epp.ProtocolIDMission,
	}

	for _, pid := range pids {
		data := []byte{0x01, 0x02}
		pkt, err := epp.NewPacket(pid, data)
		if err != nil {
			t.Fatalf("NewPacket(PID=%d) failed: %v", pid, err)
		}
		encoded, err := pkt.Encode()
		if err != nil {
			t.Fatalf("Encode(PID=%d) failed: %v", pid, err)
		}
		decoded, err := epp.Decode(encoded)
		if err != nil {
			t.Fatalf("Decode(PID=%d) failed: %v", pid, err)
		}
		if decoded.Header.ProtocolID != pid {
			t.Errorf("ProtocolID = %d, want %d", decoded.Header.ProtocolID, pid)
		}
		if !bytes.Equal(decoded.Data, data) {
			t.Errorf("Data mismatch for PID %d", pid)
		}
	}
}

func TestLargePacket(t *testing.T) {
	// A packet that requires the 32-bit length field (> 65535 bytes)
	data := make([]byte, 70000)
	data[0] = 0xAA
	data[69999] = 0xBB

	pkt, err := epp.NewPacket(epp.ProtocolIDExtended, data,
		epp.WithExtendedProtocolID(1), epp.WithCCSDSDefined(0))
	if err != nil {
		t.Fatalf("NewPacket failed: %v", err)
	}

	if pkt.Header.Size() != epp.HeaderSize8 {
		t.Errorf("Header size = %d, want 8", pkt.Header.Size())
	}

	encoded, err := pkt.Encode()
	if err != nil {
		t.Fatal(err)
	}

	decoded, err := epp.Decode(encoded)
	if err != nil {
		t.Fatal(err)
	}
	if decoded.Data[0] != 0xAA || decoded.Data[69999] != 0xBB {
		t.Error("Large packet data corrupted")
	}
}

func TestValidateMismatchedLength(t *testing.T) {
	pkt := &epp.EncapsulationPacket{
		Header: epp.Header{
			PVN:            epp.PVN,
			ProtocolID:     epp.ProtocolIDIPE,
			LengthOfLength: epp.LoL1Octet,
			PacketLength:   100, // wrong, doesn't match data
		},
		Data: []byte{0x01, 0x02},
	}
	if err := pkt.Validate(); err == nil {
		t.Error("Expected validation error for mismatched length")
	}
}

func TestDecodeExtraTrailingData(t *testing.T) {
	// Valid packet followed by extra bytes, Decode should only consume the packet
	pkt, _ := epp.NewIPEPacket([]byte{0x01, 0x02})
	encoded, _ := pkt.Encode()

	withExtra := append(encoded, 0xFF, 0xFF, 0xFF)
	decoded, err := epp.Decode(withExtra)
	if err != nil {
		t.Fatalf("Decode failed: %v", err)
	}
	if !bytes.Equal(decoded.Data, []byte{0x01, 0x02}) {
		t.Error("Decoded data should match original, ignoring trailing bytes")
	}
}

func TestDecodeMalformedPacketLengthLessThanHeader(t *testing.T) {
	// 2-octet header: PVN=7, PID=2, LoL='01', PacketLength=1 (< header size 2)
	data := []byte{0xE9, 0x01}
	_, err := epp.Decode(data)
	if !errors.Is(err, epp.ErrPacketLengthMismatch) {
		t.Errorf("Expected ErrPacketLengthMismatch for PacketLength < headerSize, got %v", err)
	}
}

func TestIdleWithLongLength(t *testing.T) {
	// PID=0 with a forced longer header yields a header-only idle packet
	// (packet length equals the header size; data field absent, 4.1.3.1.4 b).
	pkt, err := epp.NewPacket(epp.ProtocolIDIdle, nil, epp.WithLongLength())
	if err != nil {
		t.Fatalf("NewPacket failed: %v", err)
	}
	if !pkt.IsIdle() {
		t.Error("Expected IsIdle()=true")
	}
	if pkt.Header.Size() != epp.HeaderSize4 {
		t.Errorf("Header size = %d, want 4", pkt.Header.Size())
	}
	if pkt.Header.PacketLength != epp.HeaderSize4 {
		t.Errorf("PacketLength = %d, want 4", pkt.Header.PacketLength)
	}

	encoded, err := pkt.Encode()
	if err != nil {
		t.Fatal(err)
	}
	decoded, err := epp.Decode(encoded)
	if err != nil {
		t.Fatal(err)
	}
	if !decoded.IsIdle() || len(decoded.Data) != 0 {
		t.Errorf("Decoded: idle=%v dataLen=%d, want idle with no data", decoded.IsIdle(), len(decoded.Data))
	}
}

func TestHeaderOnlyNonIdleRejected(t *testing.T) {
	// A packet whose length equals the header size has no data field, which
	// requires the idle protocol ID (4.1.3.1.5).
	data := []byte{0xE9, 0x02} // IPE, 2-octet header, total length 2
	_, err := epp.Decode(data)
	if !errors.Is(err, epp.ErrEmptyData) {
		t.Errorf("Expected ErrEmptyData, got %v", err)
	}
}

func TestDecodeRejectsHugeLoL4PacketLength(t *testing.T) {
	// An 8-octet header (LoL '11') only requires PacketLength >= HeaderSize8;
	// nothing caps it below the full 32-bit range, so a peer can declare
	// 0xFFFFFFFF. Decode must reject this as too short for the data on hand
	// rather than slicing off the end of the buffer: int(header.PacketLength)
	// on a 32-bit build turns 0xFFFFFFFF negative, which would otherwise pass
	// `len(data) < totalSize` and then panic on data[headerSize:totalSize].
	data := make([]byte, epp.HeaderSize8)
	data[0] = (epp.PVN << 5) | (epp.ProtocolIDMission << 2) | epp.LoL4Octet
	data[4], data[5], data[6], data[7] = 0xFF, 0xFF, 0xFF, 0xFF

	pkt, err := epp.Decode(data)
	if !errors.Is(err, epp.ErrDataTooShort) {
		t.Fatalf("Decode() error = %v, want ErrDataTooShort", err)
	}
	if pkt != nil {
		t.Errorf("Decode() packet = %+v, want nil", pkt)
	}
}

func TestHumanizeAllHeaderSizes(t *testing.T) {
	idle, _ := epp.NewIdlePacket()
	if s := idle.Humanize(); s == "" {
		t.Error("Humanize(idle) returned empty")
	}

	f4, _ := epp.NewIPEPacket([]byte{0x01}, epp.WithUserDefined(0xB))
	if s := f4.Humanize(); s == "" {
		t.Error("Humanize(4-octet header) returned empty")
	}

	f8, _ := epp.NewPacket(epp.ProtocolIDExtended, []byte{0x01},
		epp.WithExtendedProtocolID(2), epp.WithCCSDSDefined(0x1234))
	if s := f8.Humanize(); s == "" {
		t.Error("Humanize(8-octet header) returned empty")
	}
}
