package aos_test

import (
	"bytes"
	"encoding/hex"
	"errors"
	"testing"

	"github.com/ravisuhag/astro/internal/vectors"
	"github.com/ravisuhag/astro/pkg/aos"
	"github.com/ravisuhag/astro/pkg/spp"
)

// The wire vectors for this package live in vectors/aos/. They are the
// reference; this file only wires them to the Go API. See
// vectors/README.md for the format and the honesty rule behind it.
//
// What stays here as Go tests: the stateful services and the independent
// re-derivations. A vector cannot express a multiplexing service filling
// a partial frame, and the FHEC corruption test needs an independent CRC
// to re-seal the frame it corrupts.

// frameConfig turns a vector's config object into a ChannelConfig.
func frameConfig(config vectors.Fields) (aos.ChannelConfig, error) {
	var c aos.ChannelConfig
	fhec, err := config.BoolOr("has_fhec", false)
	if err != nil {
		return c, err
	}
	fecf, err := config.BoolOr("has_fecf", false)
	if err != nil {
		return c, err
	}
	ocf, err := config.BoolOr("has_ocf", false)
	if err != nil {
		return c, err
	}
	izl, err := config.UintOr("insert_zone_len", 0)
	if err != nil {
		return c, err
	}
	c.HasFHEC, c.HasFECF, c.HasOCF, c.InsertZoneLen = fhec, fecf, ocf, int(izl)
	return c, nil
}

// buildFrame constructs a TransferFrame from a vector's fields and the
// channel agreement in its config.
func buildFrame(f, config vectors.Fields) (*aos.TransferFrame, error) {
	scid, err := f.Uint("scid")
	if err != nil {
		return nil, err
	}
	vcid, err := f.Uint("vcid")
	if err != nil {
		return nil, err
	}
	data, err := f.HexOr("data", nil)
	if err != nil {
		return nil, err
	}

	var opts []aos.FrameOption
	if f.Has("vc_frame_count") {
		n, err := f.Uint("vc_frame_count")
		if err != nil {
			return nil, err
		}
		opts = append(opts, aos.WithVCFrameCount(uint32(n)))
	}
	if f.Has("vc_frame_count_cycle") {
		// WithVCFCUsage sets the usage flag and the cycle together, which
		// is why vcfc_usage_flag is not a separate option here.
		cycle, err := f.Uint("vc_frame_count_cycle")
		if err != nil {
			return nil, err
		}
		opts = append(opts, aos.WithVCFCUsage(uint8(cycle)))
	}
	if ok, err := f.BoolOr("replay_flag", false); err != nil {
		return nil, err
	} else if ok {
		opts = append(opts, aos.WithReplayFlag())
	}
	// FECF and FHEC presence is channel agreement, so it arrives in
	// config rather than fields.
	if on, err := config.BoolOr("has_fecf", false); err != nil {
		return nil, err
	} else if on {
		opts = append(opts, aos.WithFECF())
	}
	if on, err := config.BoolOr("has_fhec", false); err != nil {
		return nil, err
	} else if on {
		opts = append(opts, aos.WithFHEC())
	}
	if ocf, err := config.HexOr("ocf", nil); err != nil {
		return nil, err
	} else if ocf != nil {
		opts = append(opts, aos.WithOCF(ocf))
	}

	return aos.NewTransferFrame(uint8(scid), uint8(vcid), data, opts...)
}

func TestFrameVectors(t *testing.T) {
	vectors.RunFile(t, "aos/frame.json", vectors.Impl{
		EncodeFn: func(f, config vectors.Fields) ([]byte, error) {
			frame, err := buildFrame(f, config)
			if err != nil {
				return nil, err
			}
			return frame.Encode()
		},

		ConstructFn: func(f, config vectors.Fields) error {
			frame, err := buildFrame(f, config)
			if err != nil {
				return err
			}
			// Range rules are enforced on the way out, so a reject has to
			// reach Encode rather than stopping at the constructor.
			_, err = frame.Encode()
			return err
		},

		DecodeFn: func(input []byte, config vectors.Fields) (vectors.Fields, error) {
			c, err := frameConfig(config)
			if err != nil {
				return nil, err
			}
			// The fhec-of-primary-header vector pins ComputeFHEC on its
			// own, without a surrounding frame.
			if len(input) == aos.PrimaryHeaderSize && !c.HasFECF {
				fhec, err := aos.ComputeFHEC(input)
				if err != nil {
					return nil, err
				}
				return vectors.Fields{"fhec": fhec}, nil
			}

			frame, err := aos.DecodeTransferFrameWithConfig(input, c)
			if err != nil {
				return nil, err
			}
			h := frame.Header
			return vectors.Fields{
				"tfvn":                 h.TFVN,
				"scid":                 h.SCID,
				"vcid":                 h.VCID,
				"vc_frame_count":       h.VCFrameCount,
				"replay_flag":          h.ReplayFlag,
				"vcfc_usage_flag":      h.VCFCUsageFlag,
				"vc_frame_count_cycle": h.VCFrameCountCycle,
				"fhec":                 frame.FHEC,
				"data":                 frame.DataField,
			}, nil
		},
	})
}

func TestMPDUVectors(t *testing.T) {
	vectors.RunFile(t, "aos/mpdu.json", vectors.Impl{
		EncodeFn: func(f, _ vectors.Fields) ([]byte, error) {
			fhp, err := f.Uint("fhp")
			if err != nil {
				return nil, err
			}
			data, err := f.HexOr("data", nil)
			if err != nil {
				return nil, err
			}
			return aos.PackMPDUDataField(uint16(fhp), data)
		},
	})
}

// TestMPDUSpecialFHPConstants keeps the named constants pinned. The
// vectors above prove the encoding; this proves the package exports the
// right names for the two special values (clause 4.1.4.2.3.4-5).
func TestMPDUSpecialFHPConstants(t *testing.T) {
	if aos.FHPNoPacketStart != 0x07FF {
		t.Errorf("FHPNoPacketStart = 0x%04X, want 0x07FF ('all ones')", aos.FHPNoPacketStart)
	}
	if aos.FHPAllIdle != 0x07FE {
		t.Errorf("FHPAllIdle = 0x%04X, want 0x07FE ('all ones minus one')", aos.FHPAllIdle)
	}
}

// TestFHECDetectsHeaderCorruption is not a vector: it corrupts a frame and
// re-seals the FECF with an independent CRC, so only the FHEC can trip.
// Expressing that needs a computation, not a fixture.
func TestFHECDetectsHeaderCorruption(t *testing.T) {
	good, _ := hex.DecodeString("6aea00010243ce8edeadbeef3934")
	config := aos.ChannelConfig{HasFHEC: true, HasFECF: true}

	if _, err := aos.DecodeTransferFrameWithConfig(good, config); err != nil {
		t.Fatalf("the good frame must decode: %v", err)
	}

	bad := append([]byte{}, good...)
	bad[1] ^= 0x01 // flip a VCID bit inside the protected header
	sum := crcCCITT(bad[:len(bad)-2])
	bad[len(bad)-2] = byte(sum >> 8)
	bad[len(bad)-1] = byte(sum)

	if _, err := aos.DecodeTransferFrameWithConfig(bad, config); !errors.Is(err, aos.ErrFHECMismatch) {
		t.Errorf("corrupted protected header: got %v, want ErrFHECMismatch", err)
	}
}

// crcCCITT is an independent CRC-16-CCITT used to re-seal test frames.
func crcCCITT(data []byte) uint16 {
	crc := uint16(0xFFFF)
	for _, b := range data {
		crc ^= uint16(b) << 8
		for range 8 {
			if crc&0x8000 != 0 {
				crc = crc<<1 ^ 0x1021
			} else {
				crc <<= 1
			}
		}
	}
	return crc
}

// A flushed partial M_PDU packet zone is completed with a real SPP idle
// packet (APID 0x7FF), and the receive side discards it by APID. Stateful:
// send, flush, then read back through a fresh receiver.
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
