package usdl_test

import (
	"testing"

	"github.com/ravisuhag/astro/pkg/usdl"
)

func TestMAPPacketService_VariableLength(t *testing.T) {
	vc := usdl.NewVirtualChannel(1, 100)
	config := usdl.ChannelConfig{} // FrameLength=0 → variable-length mode
	counter := usdl.NewFrameCounter()

	svc := usdl.NewMAPPacketService(100, 1, 0, vc, config, counter)

	data := []byte{0x01, 0x02, 0x03, 0x04, 0x05}
	if err := svc.Send(data); err != nil {
		t.Fatalf("Send() error = %v", err)
	}

	got, err := svc.Receive()
	if err != nil {
		t.Fatalf("Receive() error = %v", err)
	}

	if len(got) != len(data) {
		t.Fatalf("Receive() len = %d, want %d", len(got), len(data))
	}
	for i, b := range got {
		if b != data[i] {
			t.Errorf("Receive()[%d] = 0x%02X, want 0x%02X", i, b, data[i])
		}
	}
}

func TestMAPPacketService_EmptyData(t *testing.T) {
	vc := usdl.NewVirtualChannel(1, 100)
	config := usdl.ChannelConfig{}
	svc := usdl.NewMAPPacketService(100, 1, 0, vc, config, nil)

	err := svc.Send([]byte{})
	if err != usdl.ErrEmptyData {
		t.Errorf("expected ErrEmptyData, got %v", err)
	}
}

func TestMAPPacketService_SequenceCounter(t *testing.T) {
	vc := usdl.NewVirtualChannel(1, 100)
	config := usdl.ChannelConfig{}
	counter := usdl.NewFrameCounter()

	svc := usdl.NewMAPPacketService(100, 1, 0, vc, config, counter)

	// Send two packets
	if err := svc.Send([]byte{0x01}); err != nil {
		t.Fatalf("Send(1) error = %v", err)
	}
	if err := svc.Send([]byte{0x02}); err != nil {
		t.Fatalf("Send(2) error = %v", err)
	}

	f1, _ := vc.Next()
	f2, _ := vc.Next()

	if f1.DataFieldHeader.SequenceNumber != 0 {
		t.Errorf("frame1 seq = %d, want 0", f1.DataFieldHeader.SequenceNumber)
	}
	if f2.DataFieldHeader.SequenceNumber != 1 {
		t.Errorf("frame2 seq = %d, want 1", f2.DataFieldHeader.SequenceNumber)
	}
}

func TestMAPAccessService_VariableLength(t *testing.T) {
	vc := usdl.NewVirtualChannel(1, 100)
	config := usdl.ChannelConfig{} // variable-length
	counter := usdl.NewFrameCounter()

	svc := usdl.NewMAPAccessService(100, 1, 0, 8, vc, config, counter)

	data := make([]byte, 8)
	for i := range data {
		data[i] = byte(i)
	}

	if err := svc.Send(data); err != nil {
		t.Fatalf("Send() error = %v", err)
	}

	got, err := svc.Receive()
	if err != nil {
		t.Fatalf("Receive() error = %v", err)
	}

	if len(got) != 8 {
		t.Fatalf("Receive() len = %d, want 8", len(got))
	}
}

func TestMAPAccessService_SizeMismatch(t *testing.T) {
	vc := usdl.NewVirtualChannel(1, 100)
	config := usdl.ChannelConfig{} // variable-length
	svc := usdl.NewMAPAccessService(100, 1, 0, 8, vc, config, nil)

	err := svc.Send([]byte{0x01}) // wrong size
	if err != usdl.ErrSizeMismatch {
		t.Errorf("expected ErrSizeMismatch, got %v", err)
	}
}

func TestMAPOctetStreamService_VariableLength(t *testing.T) {
	vc := usdl.NewVirtualChannel(1, 100)
	config := usdl.ChannelConfig{}
	counter := usdl.NewFrameCounter()

	svc := usdl.NewMAPOctetStreamService(100, 1, 0, vc, config, counter)

	data := []byte{0xAA, 0xBB, 0xCC}
	if err := svc.Send(data); err != nil {
		t.Fatalf("Send() error = %v", err)
	}

	got, err := svc.Receive()
	if err != nil {
		t.Fatalf("Receive() error = %v", err)
	}

	if len(got) != len(data) {
		t.Fatalf("Receive() len = %d, want %d", len(got), len(data))
	}
	for i, b := range got {
		if b != data[i] {
			t.Errorf("data[%d] = 0x%02X, want 0x%02X", i, b, data[i])
		}
	}
}

func TestMAPOctetStreamService_EmptyData(t *testing.T) {
	vc := usdl.NewVirtualChannel(1, 100)
	config := usdl.ChannelConfig{}
	svc := usdl.NewMAPOctetStreamService(100, 1, 0, vc, config, nil)

	err := svc.Send([]byte{})
	if err != usdl.ErrEmptyData {
		t.Errorf("expected ErrEmptyData, got %v", err)
	}
}

func TestFrameCounter(t *testing.T) {
	fc := usdl.NewFrameCounter()

	seq1 := fc.Next(1)
	seq2 := fc.Next(1)
	seq3 := fc.Next(2) // different VC

	if seq1 != 0 {
		t.Errorf("seq1 = %d, want 0", seq1)
	}
	if seq2 != 1 {
		t.Errorf("seq2 = %d, want 1", seq2)
	}
	if seq3 != 0 {
		t.Errorf("seq3 = %d, want 0 (different VC)", seq3)
	}
}

func TestMAPAccessService_Flush(t *testing.T) {
	vc := usdl.NewVirtualChannel(1, 100)
	config := usdl.ChannelConfig{}
	svc := usdl.NewMAPAccessService(100, 1, 0, 8, vc, config, nil)

	// Flush should be a no-op
	if err := svc.Flush(); err != nil {
		t.Errorf("Flush() error = %v", err)
	}
}

func TestMAPPacketService_FixedLength_SequenceNumbers(t *testing.T) {
	vc := usdl.NewVirtualChannel(1, 100)
	config := usdl.ChannelConfig{
		FrameLength: 64,
		HasFECF:     true,
	}
	counter := usdl.NewFrameCounter()
	svc := usdl.NewMAPPacketService(100, 1, 0, vc, config, counter)

	// Send several small packets that each fit in one frame
	capacity := config.DataFieldCapacity(0)
	for i := range 5 {
		data := make([]byte, capacity/2)
		data[0] = byte(i)
		if err := svc.Send(data); err != nil {
			t.Fatalf("Send(%d) error = %v", i, err)
		}
	}
	if err := svc.Flush(); err != nil {
		t.Fatalf("Flush() error = %v", err)
	}

	// Verify frames have sequential sequence numbers and correct header
	lastSeq := -1
	for {
		frame, err := vc.Next()
		if err != nil {
			break
		}
		seq := int(frame.DataFieldHeader.SequenceNumber)
		if lastSeq >= 0 && seq != lastSeq+1 {
			t.Errorf("sequence gap: expected %d, got %d", lastSeq+1, seq)
		}
		lastSeq = seq

		// Fixed-length frames should have EndOfFPH=true
		if !frame.Header.EndOfFPH {
			t.Error("expected EndOfFPH=true for fixed-length frame")
		}

		// Verify frame encodes to the correct total length
		encoded, err := frame.Encode()
		if err != nil {
			t.Fatalf("Encode() error = %v", err)
		}
		if len(encoded) != config.FrameLength {
			t.Errorf("encoded frame length = %d, want %d", len(encoded), config.FrameLength)
		}

		// Verify CRC is valid by re-decoding
		_, err = usdl.DecodeTransferFrame(encoded, usdl.FECSize16, 0)
		if err != nil {
			t.Fatalf("DecodeTransferFrame() error = %v (CRC or structure mismatch)", err)
		}
	}

	if lastSeq < 0 {
		t.Fatal("no frames were emitted")
	}
}

func TestMAPAccessService_FixedLength(t *testing.T) {
	vc := usdl.NewVirtualChannel(1, 100)
	config := usdl.ChannelConfig{
		FrameLength: 48,
		HasFECF:     true,
	}
	counter := usdl.NewFrameCounter()
	svc := usdl.NewMAPAccessService(100, 1, 0, 16, vc, config, counter)

	data := make([]byte, 16)
	for i := range data {
		data[i] = byte(i + 0x10)
	}

	if err := svc.Send(data); err != nil {
		t.Fatalf("Send() error = %v", err)
	}

	frame, err := vc.Next()
	if err != nil {
		t.Fatalf("Next() error = %v", err)
	}

	// Verify EndOfFPH is set for fixed-length mode
	if !frame.Header.EndOfFPH {
		t.Error("expected EndOfFPH=true for fixed-length frame")
	}

	// Verify encoding round-trip
	encoded, err := frame.Encode()
	if err != nil {
		t.Fatalf("Encode() error = %v", err)
	}
	_, err = usdl.DecodeTransferFrame(encoded, usdl.FECSize16, 0)
	if err != nil {
		t.Fatalf("DecodeTransferFrame() error = %v", err)
	}
}
