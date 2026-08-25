package tmsc

// Dual (Berlekamp) basis conversion per CCSDS 131.0-B-5 4.3.9 and annex F.
//
// The standard specifies Reed-Solomon symbols in the dual basis over
// GF(2^8): the bytes on the wire are dual-basis representations, while
// the encoder and decoder arithmetic runs in the conventional basis.
// Both directions are linear maps over GF(2), so the full 256-entry
// tables are built at init from the images of the eight basis bits.
//
// The basis images match the transform in CCSDS 131.0-B-5 4.3.9.3 (and
// libfec's Taltab/Tal1tab, used by gr-satellites and other ground software).

var (
	talTab  [256]byte // conventional -> dual basis
	tal1Tab [256]byte // dual -> conventional basis
)

func init() {
	// Images of bit j (value 1<<j) under each transform.
	talBits := [8]byte{0x7b, 0xaf, 0x99, 0xfa, 0x86, 0xec, 0xef, 0x8d}
	tal1Bits := [8]byte{0xcc, 0xac, 0x79, 0xf0, 0xfd, 0x2e, 0x42, 0xc5}

	for x := range 256 {
		var t, u byte
		for j := range 8 {
			if x&(1<<j) != 0 {
				t ^= talBits[j]
				u ^= tal1Bits[j]
			}
		}
		talTab[x] = t
		tal1Tab[x] = u
	}
}

// toConventional converts dual-basis bytes to conventional basis in place.
func toConventional(b []byte) {
	for i, v := range b {
		b[i] = tal1Tab[v]
	}
}

// toDual converts conventional-basis bytes to dual basis in place.
func toDual(b []byte) {
	for i, v := range b {
		b[i] = talTab[v]
	}
}
