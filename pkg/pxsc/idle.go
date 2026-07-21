package pxsc

// Idle data, per CCSDS 211.2-B-3 §3.3.
//
// A Proximity-1 link does not run continuously. PLTUs arrive in bursts with
// gaps between them, and the receiver would lose bit lock across a gap if
// nothing were transmitted. So the sender fills the silence with a repeating
// pseudo-noise pattern.
//
// The same pattern serves three purposes (§3.3.1):
//
//	Acquisition sequence — sent when transmission starts, so the receiver
//	                       can lock on before real data arrives
//	Idle sequence        — sent whenever no PLTU is ready
//	Tail sequence        — sent before the transmitter goes quiet, so the
//	                       receiver can finish decoding the last unit

// IdlePattern is the PN sequence of §3.3.2.2: hexadecimal 352EF853, repeated
// as needed.
var IdlePattern = [4]byte{0x35, 0x2E, 0xF8, 0x53}

// IdlePatternSize is the width of one repetition in octets.
const IdlePatternSize = 4

// IdleData returns n octets of idle data.
//
// §3.3.2.4: whenever the end of the PN sequence is reached it repeats from the
// first bit, so the output is the pattern tiled to length.
func IdleData(n int) []byte {
	if n <= 0 {
		return nil
	}
	out := make([]byte, n)
	for i := range out {
		out[i] = IdlePattern[i%IdlePatternSize]
	}
	return out
}

// IsIdleData reports whether data is a run of the idle pattern starting at the
// beginning of the sequence.
//
// A receiver uses this to tell filler from a PLTU that lost its sync marker.
func IsIdleData(data []byte) bool {
	if len(data) == 0 {
		return false
	}
	for i, b := range data {
		if b != IdlePattern[i%IdlePatternSize] {
			return false
		}
	}
	return true
}

// AcquisitionSequence returns n octets for the acquisition sequence of §3.3.3.
//
// It is the same pattern as any other idle data. The distinction is when it is
// sent, not what it contains: §3.3.3.1 has the transmitter radiate carrier
// first, then this, so the receiver can reach a reliable symbol stream before
// real data starts.
//
// The duration comes from the mission's Acquisition_Idle_Duration parameter,
// which is why the caller passes a length rather than this package choosing one.
func AcquisitionSequence(n int) []byte { return IdleData(n) }

// IdleSequence returns n octets to send while no PLTU is ready (§3.3.4).
func IdleSequence(n int) []byte { return IdleData(n) }

// TailSequence returns n octets for the tail sequence of §3.3.5, sent before
// the transmitter stops.
//
// Its length comes from the mission's Tail_Idle_Duration parameter.
func TailSequence(n int) []byte { return IdleData(n) }
