package ocsc_test

import (
	"testing"

	"github.com/ravisuhag/astro/pkg/ocsc"
)

func FuzzConditionRoundTrip(f *testing.F) {
	f.Add([]byte("a transfer frame"), uint8(0))
	f.Add([]byte{}, uint8(1))
	f.Add(make([]byte, 300), uint8(2))

	f.Fuzz(func(t *testing.T, frame []byte, rateSel uint8) {
		// Property: whatever goes through the conditioning chain comes back
		// out unchanged, and nothing panics along the way.
		if len(frame) == 0 || len(frame) > 4096 {
			return
		}
		rate := ocsc.CodeRate(rateSel % 3)

		blocks, err := ocsc.Condition([][]byte{frame}, rate)
		if err != nil {
			return
		}
		for _, b := range blocks {
			if b.Len() != rate.EncoderInputSize() {
				t.Fatalf("block is %d digits, want k-hat = %d", b.Len(), rate.EncoderInputSize())
			}
		}

		recovered, bad, err := ocsc.Recover(blocks, rate, len(frame))
		if err != nil {
			t.Fatalf("recovering our own output failed: %v", err)
		}
		if len(bad) != 0 {
			t.Fatalf("blocks %v failed their CRC on a clean round trip", bad)
		}
		if len(recovered) != 1 {
			t.Fatalf("recovered %d frames, want 1", len(recovered))
		}
		if string(recovered[0].Data) != string(frame) {
			t.Fatal("the recovered frame differs from the original")
		}
		if !recovered[0].Valid {
			t.Fatal("a frame from clean blocks came back marked invalid")
		}
		if recovered[0].Gap {
			t.Fatal("a lone frame at the start of the stream reports a gap")
		}
	})
}

func FuzzRandomizer(f *testing.F) {
	f.Add([]byte{}, 0)
	f.Add([]byte("data"), 32)
	f.Add(make([]byte, 100), 800)

	f.Fuzz(func(t *testing.T, data []byte, bitLen int) {
		// Property: randomizing twice is the identity, at any bit length.
		if bitLen < 0 || bitLen > len(data)*8 {
			return
		}
		block := ocsc.BitStringFromBits(data, bitLen)

		once := ocsc.Randomize(block)
		twice := ocsc.Derandomize(once)

		if !twice.Equal(block) {
			t.Fatal("randomizing twice did not return the original block")
		}
	})
}

func FuzzCRCVerify(f *testing.F) {
	f.Add([]byte{}, 0)
	f.Add([]byte("message"), 56)
	f.Add(make([]byte, 64), 500)

	f.Fuzz(func(t *testing.T, data []byte, bitLen int) {
		// Property: a block plus its own CRC always verifies, at any bit
		// length including the non-byte-aligned ones the real chain uses.
		if bitLen < 0 || bitLen > len(data)*8 {
			return
		}
		block := ocsc.BitStringFromBits(data, bitLen)

		withCRC := ocsc.AttachCRC(block)
		body, ok := ocsc.VerifyCRC(withCRC)
		if !ok {
			t.Fatal("a block built with our own CRC failed verification")
		}
		if !body.Equal(block) {
			t.Fatal("the recovered body differs from the original")
		}
	})
}
