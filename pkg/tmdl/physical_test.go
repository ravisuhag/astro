package tmdl_test

import (
	"bytes"
	"errors"
	"testing"

	"github.com/ravisuhag/astro/pkg/tmdl"
)

// --- ChannelConfig Tests ---

func TestChannelConfig_DataFieldCapacity(t *testing.T) {
	tests := []struct {
		name               string
		config             tmdl.ChannelConfig
		secondaryHeaderLen int
		want               int
	}{
		{"minimal", tmdl.ChannelConfig{FrameLength: 100}, 0, 94},
		{"with OCF", tmdl.ChannelConfig{FrameLength: 100, HasOCF: true}, 0, 90},
		{"with FEC", tmdl.ChannelConfig{FrameLength: 100, HasFEC: true}, 0, 92},
		{"OCF+FEC", tmdl.ChannelConfig{FrameLength: 100, HasOCF: true, HasFEC: true}, 0, 88},
		{"secondary header", tmdl.ChannelConfig{FrameLength: 100, HasOCF: true, HasFEC: true}, 3, 84},
		{"CCSDS 1115-byte", tmdl.ChannelConfig{FrameLength: 1115, HasOCF: true, HasFEC: true}, 0, 1103},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.config.DataFieldCapacity(tt.secondaryHeaderLen)
			if got != tt.want {
				t.Errorf("DataFieldCapacity(%d) = %d, want %d", tt.secondaryHeaderLen, got, tt.want)
			}
		})
	}
}

// --- PhysicalChannel MC Multiplexing Tests ---

func TestPhysicalChannel_AddMasterChannel(t *testing.T) {
	config := tmdl.ChannelConfig{FrameLength: 28, HasFEC: true}
	pc := tmdl.NewPhysicalChannel("X-band", config)

	mc := tmdl.NewMasterChannel(933, config)
	pc.AddMasterChannel(mc, 1)

	if pc.Len() != 1 {
		t.Errorf("Len = %d, want 1", pc.Len())
	}
	if pc.Name != "X-band" {
		t.Errorf("Name = %q, want 'X-band'", pc.Name)
	}
}

func TestPhysicalChannel_MCMultiplexing(t *testing.T) {
	config := tmdl.ChannelConfig{FrameLength: 28, HasFEC: true}
	pc := tmdl.NewPhysicalChannel("downlink", config)

	mc1 := tmdl.NewMasterChannel(100, config)
	vc1 := tmdl.NewVirtualChannel(1, 100)
	mc1.AddVirtualChannel(vc1, 1)

	mc2 := tmdl.NewMasterChannel(200, config)
	vc2 := tmdl.NewVirtualChannel(1, 100)
	mc2.AddVirtualChannel(vc2, 1)

	pc.AddMasterChannel(mc1, 1)
	pc.AddMasterChannel(mc2, 1)

	svc1 := tmdl.NewVirtualChannelPacketService(100, 1, vc1, config, nil)
	svc2 := tmdl.NewVirtualChannelPacketService(200, 1, vc2, config, nil)

	if err := svc1.Send([]byte("sc100")); err != nil {
		t.Fatal(err)
	}
	if err := svc2.Send([]byte("sc200")); err != nil {
		t.Fatal(err)
	}

	f1, _ := pc.GetNextFrame()
	if f1.Header.SpacecraftID != 100 {
		t.Errorf("Frame 1 SCID = %d, want 100", f1.Header.SpacecraftID)
	}
	f2, _ := pc.GetNextFrame()
	if f2.Header.SpacecraftID != 200 {
		t.Errorf("Frame 2 SCID = %d, want 200", f2.Header.SpacecraftID)
	}
}

func TestPhysicalChannel_MCPriorityWeighting(t *testing.T) {
	config := tmdl.ChannelConfig{FrameLength: 28, HasFEC: true}
	pc := tmdl.NewPhysicalChannel("downlink", config)

	mc1 := tmdl.NewMasterChannel(100, config)
	vc1 := tmdl.NewVirtualChannel(1, 100)
	mc1.AddVirtualChannel(vc1, 1)

	mc2 := tmdl.NewMasterChannel(200, config)
	vc2 := tmdl.NewVirtualChannel(1, 100)
	mc2.AddVirtualChannel(vc2, 1)

	pc.AddMasterChannel(mc1, 2)
	pc.AddMasterChannel(mc2, 1)

	svc1 := tmdl.NewVirtualChannelPacketService(100, 1, vc1, config, nil)
	svc2 := tmdl.NewVirtualChannelPacketService(200, 1, vc2, config, nil)

	for range 3 {
		if err := svc1.Send([]byte("a")); err != nil {
			t.Fatal(err)
		}
		if err := svc2.Send([]byte("b")); err != nil {
			t.Fatal(err)
		}
	}

	expected := []uint16{100, 100, 200, 100, 200, 200}
	for i, want := range expected {
		frame, err := pc.GetNextFrame()
		if err != nil {
			t.Fatalf("Frame %d: %v", i, err)
		}
		if frame.Header.SpacecraftID != want {
			t.Errorf("Frame %d: SCID = %d, want %d", i, frame.Header.SpacecraftID, want)
		}
	}
}

func TestPhysicalChannel_Demultiplex(t *testing.T) {
	pc := tmdl.NewPhysicalChannel("uplink", tmdl.ChannelConfig{})

	mc1 := tmdl.NewMasterChannel(100, tmdl.ChannelConfig{})
	vc1 := tmdl.NewVirtualChannel(1, 100)
	mc1.AddVirtualChannel(vc1, 1)

	mc2 := tmdl.NewMasterChannel(200, tmdl.ChannelConfig{})
	vc2 := tmdl.NewVirtualChannel(1, 100)
	mc2.AddVirtualChannel(vc2, 1)

	pc.AddMasterChannel(mc1, 1)
	pc.AddMasterChannel(mc2, 1)

	f1, _ := tmdl.NewTMTransferFrame(100, 1, []byte("for-sc100"), nil, nil)
	f2, _ := tmdl.NewTMTransferFrame(200, 1, []byte("for-sc200"), nil, nil)

	if err := pc.AddFrame(f1); err != nil {
		t.Fatal(err)
	}
	if err := pc.AddFrame(f2); err != nil {
		t.Fatal(err)
	}

	got1, _ := vc1.GetNextFrame()
	if string(got1.DataField) != "for-sc100" {
		t.Errorf("MC1 got %q, want 'for-sc100'", got1.DataField)
	}
	got2, _ := vc2.GetNextFrame()
	if string(got2.DataField) != "for-sc200" {
		t.Errorf("MC2 got %q, want 'for-sc200'", got2.DataField)
	}
}

func TestPhysicalChannel_DemuxUnknownSCID(t *testing.T) {
	pc := tmdl.NewPhysicalChannel("test", tmdl.ChannelConfig{})
	frame, _ := tmdl.NewTMTransferFrame(999, 1, []byte("data"), nil, nil)
	err := pc.AddFrame(frame)
	if !errors.Is(err, tmdl.ErrMasterChannelNotFound) {
		t.Errorf("Expected ErrMasterChannelNotFound, got %v", err)
	}
}

func TestPhysicalChannel_GetNextFrameOrIdle(t *testing.T) {
	config := tmdl.ChannelConfig{FrameLength: 28, HasFEC: true}
	pc := tmdl.NewPhysicalChannel("test", config)

	mc := tmdl.NewMasterChannel(933, config)
	vc := tmdl.NewVirtualChannel(1, 100)
	mc.AddVirtualChannel(vc, 1)
	pc.AddMasterChannel(mc, 1)

	frame, err := pc.GetNextFrameOrIdle()
	if err != nil {
		t.Fatal(err)
	}
	if !tmdl.IsIdleFrame(frame) {
		t.Error("Expected idle frame when no data available")
	}

	svc := tmdl.NewVirtualChannelPacketService(933, 1, vc, config, nil)
	if err := svc.Send([]byte("data")); err != nil {
		t.Fatal(err)
	}
	frame, _ = pc.GetNextFrameOrIdle()
	if tmdl.IsIdleFrame(frame) {
		t.Error("Expected data frame, got idle")
	}
}

func TestPhysicalChannel_NoMasterChannels(t *testing.T) {
	pc := tmdl.NewPhysicalChannel("empty", tmdl.ChannelConfig{})
	_, err := pc.GetNextFrame()
	if !errors.Is(err, tmdl.ErrNoMasterChannels) {
		t.Errorf("Expected ErrNoMasterChannels, got %v", err)
	}
}

func TestPhysicalChannel_HasPendingFrames(t *testing.T) {
	pc := tmdl.NewPhysicalChannel("test", tmdl.ChannelConfig{})
	mc := tmdl.NewMasterChannel(933, tmdl.ChannelConfig{})
	vc := tmdl.NewVirtualChannel(1, 100)
	mc.AddVirtualChannel(vc, 1)
	pc.AddMasterChannel(mc, 1)

	if pc.HasPendingFrames() {
		t.Error("Expected false with no data")
	}
	if err := vc.AddFrame(&tmdl.TMTransferFrame{}); err != nil {
		t.Fatal(err)
	}
	if !pc.HasPendingFrames() {
		t.Error("Expected true with pending data")
	}
}

// --- Wrap/Unwrap (ASM + Randomization) Tests ---

func TestPhysicalChannel_Wrap_DefaultASM(t *testing.T) {
	pc := tmdl.NewPhysicalChannel("test", tmdl.ChannelConfig{})

	frame, err := tmdl.NewTMTransferFrame(933, 1, []byte("data"), nil, nil)
	if err != nil {
		t.Fatal(err)
	}

	cadu, err := pc.Wrap(frame)
	if err != nil {
		t.Fatal(err)
	}

	// CADU should start with default ASM
	if !bytes.Equal(cadu[:4], tmdl.DefaultASM) {
		t.Errorf("ASM = %x, want %x", cadu[:4], tmdl.DefaultASM)
	}

	// Encoded frame follows ASM
	encoded, _ := frame.Encode()
	if !bytes.Equal(cadu[4:], encoded) {
		t.Error("Frame data after ASM does not match encoded frame")
	}
}

func TestPhysicalChannel_Wrap_CustomASM(t *testing.T) {
	pc := tmdl.NewPhysicalChannel("test", tmdl.ChannelConfig{})
	pc.ASM = []byte{0xAA, 0xBB}

	frame, _ := tmdl.NewTMTransferFrame(933, 1, []byte("data"), nil, nil)
	cadu, err := pc.Wrap(frame)
	if err != nil {
		t.Fatal(err)
	}

	if !bytes.Equal(cadu[:2], []byte{0xAA, 0xBB}) {
		t.Errorf("Custom ASM = %x, want AABB", cadu[:2])
	}
}

func TestPhysicalChannel_WrapUnwrap_RoundTrip(t *testing.T) {
	pc := tmdl.NewPhysicalChannel("test", tmdl.ChannelConfig{})

	frame, _ := tmdl.NewTMTransferFrame(933, 1, []byte("round trip"), nil, nil)
	cadu, err := pc.Wrap(frame)
	if err != nil {
		t.Fatal(err)
	}

	decoded, err := pc.Unwrap(cadu)
	if err != nil {
		t.Fatal(err)
	}

	if !bytes.Equal(decoded.DataField, []byte("round trip")) {
		t.Errorf("DataField = %q, want 'round trip'", decoded.DataField)
	}
}

func TestPhysicalChannel_WrapUnwrap_WithRandomization(t *testing.T) {
	pc := tmdl.NewPhysicalChannel("test", tmdl.ChannelConfig{})
	pc.Randomize = true

	frame, _ := tmdl.NewTMTransferFrame(933, 1, []byte("secret data"), nil, nil)

	// Wrap with randomization
	cadu, err := pc.Wrap(frame)
	if err != nil {
		t.Fatal(err)
	}

	// Frame bytes after ASM should differ from unrandomized encoding
	encoded, _ := frame.Encode()
	randomized := cadu[4:]
	if bytes.Equal(randomized, encoded) {
		t.Error("Randomized data should differ from plain encoded frame")
	}

	// Unwrap should recover original frame
	decoded, err := pc.Unwrap(cadu)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(decoded.DataField, []byte("secret data")) {
		t.Errorf("DataField = %q, want 'secret data'", decoded.DataField)
	}
	if decoded.Header.SpacecraftID != 933 {
		t.Errorf("SCID = %d, want 933", decoded.Header.SpacecraftID)
	}
}

func TestPhysicalChannel_Unwrap_BadASM(t *testing.T) {
	pc := tmdl.NewPhysicalChannel("test", tmdl.ChannelConfig{})

	badCADU := []byte{0x00, 0x00, 0x00, 0x00, 0x01, 0x02, 0x03}
	_, err := pc.Unwrap(badCADU)
	if !errors.Is(err, tmdl.ErrSyncMarkerMismatch) {
		t.Errorf("Expected ErrSyncMarkerMismatch, got %v", err)
	}
}

func TestPhysicalChannel_Unwrap_TooShort(t *testing.T) {
	pc := tmdl.NewPhysicalChannel("test", tmdl.ChannelConfig{})
	_, err := pc.Unwrap([]byte{0x1A})
	if !errors.Is(err, tmdl.ErrDataTooShort) {
		t.Errorf("Expected ErrDataTooShort, got %v", err)
	}
}

func TestPNSequence_FirstByte(t *testing.T) {
	// The CCSDS PN sequence always starts with 0xFF (LFSR initialized to all 1s)
	pc := tmdl.NewPhysicalChannel("test", tmdl.ChannelConfig{})
	pc.Randomize = true

	// Create a frame, wrap it, and verify randomization XORs with 0xFF first
	frame, _ := tmdl.NewTMTransferFrame(0, 0, []byte{0x00}, nil, nil)
	encoded, _ := frame.Encode()
	cadu, _ := pc.Wrap(frame)

	firstFrameByte := cadu[4] // after 4-byte ASM
	// If PN first byte is 0xFF, then XOR(encoded[0], 0xFF) should give us the randomized byte
	if firstFrameByte != encoded[0]^0xFF {
		t.Errorf("First randomized byte = 0x%02X, expected 0x%02X (encoded 0x%02X XOR 0xFF)",
			firstFrameByte, encoded[0]^0xFF, encoded[0])
	}
}

func TestPNSequence_Deterministic(t *testing.T) {
	pc := tmdl.NewPhysicalChannel("test", tmdl.ChannelConfig{})
	pc.Randomize = true

	frame, _ := tmdl.NewTMTransferFrame(933, 1, []byte("deterministic"), nil, nil)

	cadu1, _ := pc.Wrap(frame)
	cadu2, _ := pc.Wrap(frame)

	if !bytes.Equal(cadu1, cadu2) {
		t.Error("PN sequence should be deterministic — same frame should produce same CADU")
	}
}
