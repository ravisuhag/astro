package aos_test

import (
	"bytes"
	"encoding/hex"
	"testing"

	"github.com/ravisuhag/astro/pkg/aos"
	"github.com/ravisuhag/astro/pkg/spp"
)

// Golden wire vectors, hand-computed from the CCSDS 732.0-B-4 field
// layouts.
//
// Header: TFVN=01, SCID=0xAB, VCID=42, VCFC=0x000102, replay=0,
// VCFC usage=1, cycle=3.
//
//	byte 0 = 01<<6 | 0xAB>>2        = 0x6A
//	byte 1 = (0xAB&0x3)<<6 | 42     = 0xEA
//	bytes 2-4 = 00 01 02
//	byte 5 = 0x40 | 0x03            = 0x43
//
// FECF is CRC-16-CCITT (poly 0x1021, init 0xFFFF) over the whole frame,
// computed with an independent implementation.
func TestGoldenVector_FrameWithFECF(t *testing.T) {
	want, _ := hex.DecodeString("6aea00010243deadbeef9e2c")

	frame, err := aos.NewTransferFrame(0xAB, 42, []byte{0xDE, 0xAD, 0xBE, 0xEF},
		aos.WithVCFrameCount(0x000102),
		aos.WithVCFCUsage(3),
		aos.WithFECF(),
	)
	if err != nil {
		t.Fatalf("NewTransferFrame() error = %v", err)
	}
	got, err := frame.Encode()
	if err != nil {
		t.Fatalf("Encode() error = %v", err)
	}
	if !bytes.Equal(got, want) {
		t.Fatalf("wire mismatch:\n got %x\nwant %x", got, want)
	}

	decoded, err := aos.DecodeTransferFrame(want, 0, false, true)
	if err != nil {
		t.Fatalf("DecodeTransferFrame() error = %v", err)
	}
	if decoded.Header.SCID != 0xAB || decoded.Header.VCID != 42 ||
		decoded.Header.VCFrameCount != 0x000102 ||
		!decoded.Header.VCFCUsageFlag || decoded.Header.VCFrameCountCycle != 3 {
		t.Errorf("decoded header mismatch: %+v", decoded.Header)
	}
}

// The FHEC check symbols for header 6A EA 00 01 02 43 were computed with
// an independent GF(2^4) RS(10,6) implementation (field poly x^4+x+1,
// generator roots α^6..α^9): info symbols [6,10,14,10,4,3] give parity
// [12,14,8,14] = 0xCE8E. The FECF then covers the FHEC octets too.
func TestGoldenVector_FrameWithFHEC(t *testing.T) {
	hdr, _ := hex.DecodeString("6aea00010243")
	fhec, err := aos.ComputeFHEC(hdr)
	if err != nil {
		t.Fatalf("ComputeFHEC() error = %v", err)
	}
	if hex.EncodeToString(fhec) != "ce8e" {
		t.Fatalf("FHEC = %x, want ce8e", fhec)
	}

	want, _ := hex.DecodeString("6aea00010243ce8edeadbeef3934")
	frame, err := aos.NewTransferFrame(0xAB, 42, []byte{0xDE, 0xAD, 0xBE, 0xEF},
		aos.WithVCFrameCount(0x000102),
		aos.WithVCFCUsage(3),
		aos.WithFHEC(),
		aos.WithFECF(),
	)
	if err != nil {
		t.Fatalf("NewTransferFrame() error = %v", err)
	}
	got, err := frame.Encode()
	if err != nil {
		t.Fatalf("Encode() error = %v", err)
	}
	if !bytes.Equal(got, want) {
		t.Fatalf("wire mismatch:\n got %x\nwant %x", got, want)
	}

	config := aos.ChannelConfig{HasFHEC: true, HasFECF: true}
	decoded, err := aos.DecodeFrame(want, config)
	if err != nil {
		t.Fatalf("DecodeFrame() error = %v", err)
	}
	if !bytes.Equal(decoded.FHEC, fhec) {
		t.Errorf("decoded FHEC = %x, want %x", decoded.FHEC, fhec)
	}
	if !bytes.Equal(decoded.DataField, []byte{0xDE, 0xAD, 0xBE, 0xEF}) {
		t.Errorf("decoded data field = %x", decoded.DataField)
	}

	// A corrupted protected header octet must be caught by the FHEC.
	// Corrupt the VCID bits and refresh the FECF so only the FHEC trips.
	bad := append([]byte{}, want...)
	bad[1] ^= 0x01
	badFrame := append([]byte{}, bad[:len(bad)-2]...)
	sum := crcCCITT(badFrame)
	bad[len(bad)-2] = byte(sum >> 8)
	bad[len(bad)-1] = byte(sum)
	if _, err := aos.DecodeFrame(bad, config); err != aos.ErrFHECMismatch {
		t.Errorf("expected ErrFHECMismatch, got %v", err)
	}
}

// crcCCITT is an independent CRC-16-CCITT used to re-seal test frames.
func crcCCITT(data []byte) uint16 {
	crc := uint16(0xFFFF)
	for _, b := range data {
		crc ^= uint16(b) << 8
		for i := 0; i < 8; i++ {
			if crc&0x8000 != 0 {
				crc = crc<<1 ^ 0x1021
			} else {
				crc <<= 1
			}
		}
	}
	return crc
}

// M_PDU golden vector: FHP special values sit at the top of the 11-bit
// range (§4.1.4.2.3.4-5): 0x7FF = no packet starts, 0x7FE = only idle.
func TestGoldenVector_MPDUSpecialFHPValues(t *testing.T) {
	if aos.FHPNoPacketStart != 0x07FF {
		t.Errorf("FHPNoPacketStart = 0x%04X, want 0x07FF ('all ones')", aos.FHPNoPacketStart)
	}
	if aos.FHPAllIdle != 0x07FE {
		t.Errorf("FHPAllIdle = 0x%04X, want 0x07FE ('all ones minus one')", aos.FHPAllIdle)
	}
	df, err := aos.PackMPDUDataField(aos.FHPNoPacketStart, []byte{0xAA})
	if err != nil {
		t.Fatalf("PackMPDUDataField() error = %v", err)
	}
	if df[0] != 0x07 || df[1] != 0xFF {
		t.Errorf("encoded M_PDU header = %02X %02X, want 07 FF", df[0], df[1])
	}
}

// Signaling-field reserved spares (bits 42-43) must be zero on decode.
func TestDecode_RejectsSignalingSpares(t *testing.T) {
	raw, _ := hex.DecodeString("6aea00010273deadbeef") // byte 5 = 0x73: spare bit set
	var hdr aos.PrimaryHeader
	if err := hdr.Decode(raw); err != aos.ErrInvalidSignalingSpare {
		t.Errorf("expected ErrInvalidSignalingSpare, got %v", err)
	}
}

// A flushed partial M_PDU packet zone is completed with a real SPP idle
// packet (APID 0x7FF), and the receive side discards it by APID.
func TestMultiplexingService_FlushFillsWithIdlePacket(t *testing.T) {
	config := aos.ChannelConfig{FrameLength: 64, HasFECF: true}
	vc := aos.NewVirtualChannel(1, 100)
	tx := aos.NewMultiplexingService(50, 1, vc, config, aos.NewFrameCounter())

	pkt, err := spp.NewTMPacket(100, []byte{0x01, 0x02, 0x03, 0x04})
	if err != nil {
		t.Fatalf("NewTMPacket() error = %v", err)
	}
	pktBytes, _ := pkt.Encode()
	if err := tx.Send(pktBytes); err != nil {
		t.Fatalf("Send() error = %v", err)
	}
	if err := tx.Flush(); err != nil {
		t.Fatalf("Flush() error = %v", err)
	}

	frame, err := vc.Next()
	if err != nil {
		t.Fatalf("no frame emitted: %v", err)
	}
	zone := frame.DataField[aos.MPDUHeaderSize:]
	fill := zone[len(pktBytes):]

	idle, err := spp.Decode(fill)
	if err != nil {
		t.Fatalf("fill does not parse as a Space Packet: %v", err)
	}
	if !idle.IsIdle() {
		t.Errorf("fill packet APID = %d, want idle (0x7FF)", idle.PrimaryHeader.APID)
	}
	if got := spp.PacketSizer(fill); got != len(fill) {
		t.Errorf("idle packet length = %d, want %d (must exactly complete the zone)", got, len(fill))
	}

	// The receiver must deliver only the real packet and then run dry.
	rxVC := aos.NewVirtualChannel(1, 100)
	_ = rxVC.Add(frame)
	rx := aos.NewMultiplexingService(50, 1, rxVC, config, nil)
	rx.SetPacketSizer(spp.PacketSizer)
	got, err := rx.Receive()
	if err != nil {
		t.Fatalf("Receive() error = %v", err)
	}
	if !bytes.Equal(got, pktBytes) {
		t.Errorf("received packet mismatch:\n got %x\nwant %x", got, pktBytes)
	}
	if extra, err := rx.Receive(); err == nil {
		t.Errorf("idle fill delivered as user data: %x", extra)
	}
}

// The configured idle pattern shows up in OID frames, and the OID virtual
// channel keeps its own frame count.
func TestIdleFrames_PatternAndCounter(t *testing.T) {
	config := aos.ChannelConfig{FrameLength: 32, HasFECF: true, IdlePattern: []byte{0xA5, 0x5A}}
	mc := aos.NewMasterChannel(7, config)

	first, err := mc.GetNextFrameOrIdle()
	if err != nil {
		t.Fatalf("GetNextFrameOrIdle() error = %v", err)
	}
	second, err := mc.GetNextFrameOrIdle()
	if err != nil {
		t.Fatalf("GetNextFrameOrIdle() error = %v", err)
	}
	if !aos.IsIdleFrame(first) || !aos.IsIdleFrame(second) {
		t.Fatal("expected OID frames")
	}
	if first.Header.VCFrameCount != 0 || second.Header.VCFrameCount != 1 {
		t.Errorf("OID frame counts = %d, %d; want 0, 1",
			first.Header.VCFrameCount, second.Header.VCFrameCount)
	}
	for i, b := range first.DataField {
		want := []byte{0xA5, 0x5A}[i%2]
		if b != want {
			t.Errorf("idle fill[%d] = 0x%02X, want 0x%02X", i, b, want)
			break
		}
	}
}
