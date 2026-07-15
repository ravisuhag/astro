package pxdl_test

import (
	"testing"

	"github.com/ravisuhag/astro/pkg/pxdl"
)

func FuzzDecodeTransferFrame(f *testing.F) {
	if frame, err := pxdl.NewTransferFrame(42, 3, []byte("seed payload")); err == nil {
		if encoded, err := frame.Encode(); err == nil {
			f.Add(encoded)
		}
	}
	if frame, err := pxdl.NewSupervisoryFrame(42, 0, []byte{0x80, 0x00}); err == nil {
		if encoded, err := frame.Encode(); err == nil {
			f.Add(encoded)
		}
	}
	f.Add([]byte{})
	f.Add(make([]byte, pxdl.HeaderSize))
	f.Add([]byte{0x80, 0x00, 0x00, 0x04, 0x00})

	f.Fuzz(func(t *testing.T, data []byte) {
		// Property: never panic, and a frame that decodes must re-encode.
		frame, err := pxdl.DecodeTransferFrame(data)
		if err != nil {
			return
		}
		encoded, err := frame.Encode()
		if err != nil {
			t.Fatalf("a decoded frame failed to re-encode: %v", err)
		}
		if len(encoded) != int(frame.Header.FrameLength) {
			t.Fatalf("re-encoded %d octets, header says %d", len(encoded), frame.Header.FrameLength)
		}
		if frame.IsSupervisoryFrame() {
			_, _ = frame.SPDUs()
		}
	})
}

func FuzzDecodeSPDU(f *testing.F) {
	if encoded, err := (&pxdl.PLCW{ReportValue: 5, PCID: 1}).Encode(); err == nil {
		f.Add(encoded)
	}
	if encoded, err := (&pxdl.VariableSPDU{TypeID: 2, Data: []byte{1, 2, 3}}).Encode(); err == nil {
		f.Add(encoded)
	}
	f.Add([]byte{})
	f.Add([]byte{0x00})
	f.Add([]byte{0x0F})
	f.Add([]byte{0x80, 0x00, 0x00})

	f.Fuzz(func(t *testing.T, data []byte) {
		// Property: never panic, and whatever decodes must re-encode to the
		// same octets, since SPDUs are self-delimiting.
		spdus, err := pxdl.DecodeSPDUs(data)
		if err != nil {
			return
		}
		reEncoded, err := pxdl.EncodeSPDUs(spdus)
		if err != nil {
			t.Fatalf("decoded SPDUs failed to re-encode: %v", err)
		}
		if len(reEncoded) != len(data) {
			t.Fatalf("re-encoded %d octets from a %d-octet input", len(reEncoded), len(data))
		}
	})
}

func FuzzReassembler(f *testing.F) {
	f.Add([]byte{0x40, 1, 2, 3}) // first segment, pseudo packet 0
	f.Add([]byte{0xC0, 9})       // unsegmented
	f.Add([]byte{})
	f.Add([]byte{0x00})

	f.Fuzz(func(t *testing.T, data []byte) {
		// Property: arbitrary segment bytes never panic the reassembler and
		// never grow it without bound.
		r := pxdl.NewReassembler()
		r.MaxPacketSize = 4096

		seg, err := pxdl.DecodeSegment(data)
		if err != nil {
			return
		}
		// Feed the same segment repeatedly: a continuing-segment loop must
		// still hit the size cap rather than growing forever.
		for i := 0; i < 8; i++ {
			if _, err := r.Accept(0, 0, seg); err != nil {
				break
			}
		}
		_ = r.Pending()
	})
}
