package aos_test

import (
	"testing"

	"github.com/ravisuhag/astro/pkg/aos"
)

// TestFlushSmallPacketZoneDoesNotPanic covers a packet zone too small to hold
// a whole idle packet in one frame, and too small for one extra frame to be
// enough either. Growing the idle packet by a single capacity used to leave it
// under the seven-octet minimum, and building it then allocated a negative
// length and panicked.
func TestFlushSmallPacketZoneDoesNotPanic(t *testing.T) {
	for _, capacity := range []int{1, 2, 3, 5, 6} {
		frameLen := aos.PrimaryHeaderSize + aos.MPDUHeaderSize + capacity

		vc := aos.NewVirtualChannel(1, 32)
		config := aos.ChannelConfig{FrameLength: frameLen}
		svc := aos.NewMultiplexingService(50, 1, vc, config, aos.NewFrameCounter())

		if err := svc.Send([]byte{0x01}); err != nil {
			t.Fatalf("capacity %d: Send failed: %v", capacity, err)
		}
		if err := svc.Flush(); err != nil {
			t.Fatalf("capacity %d: Flush failed: %v", capacity, err)
		}

		// Every emitted frame must be a whole frame of the configured length.
		frames := 0
		for {
			frame, err := vc.Next()
			if err != nil {
				break
			}
			encoded, err := frame.Encode()
			if err != nil {
				t.Fatalf("capacity %d: encoding frame %d: %v", capacity, frames, err)
			}
			if len(encoded) != frameLen {
				t.Errorf("capacity %d: frame %d is %d octets, want %d",
					capacity, frames, len(encoded), frameLen)
			}
			frames++
		}
		if frames == 0 {
			t.Errorf("capacity %d: Flush emitted no frames", capacity)
		}
	}
}

// TestFlushExactlyFullPacketZoneEmitsNoIdleFrame checks that data filling the
// packet zone exactly is flushed as one frame. Flush used to run the idle path
// with nothing left to fill, which produced a spurious extra idle frame.
func TestFlushExactlyFullPacketZoneEmitsNoIdleFrame(t *testing.T) {
	const frameLen = 64
	capacity := frameLen - aos.PrimaryHeaderSize - aos.MPDUHeaderSize

	vc := aos.NewVirtualChannel(1, 32)
	config := aos.ChannelConfig{FrameLength: frameLen}
	svc := aos.NewMultiplexingService(50, 1, vc, config, aos.NewFrameCounter())

	if err := svc.Send(make([]byte, capacity)); err != nil {
		t.Fatalf("Send failed: %v", err)
	}
	if err := svc.Flush(); err != nil {
		t.Fatalf("Flush failed: %v", err)
	}

	frames := 0
	for {
		if _, err := vc.Next(); err != nil {
			break
		}
		frames++
	}
	if frames != 1 {
		t.Errorf("emitted %d frames for one full packet zone, want 1", frames)
	}
}
