package bp

import (
	"testing"

	"github.com/ravisuhag/astro/internal/cbor"
)

// Both algorithms have a published check value: the CRC of the nine ASCII
// digits "123456789". Pinning it proves the polynomial, the initial value, the
// reflection and the final XOR are all right at once, which a round trip
// cannot do — a wrong constant checks out against itself perfectly.
func TestCRCCheckValues(t *testing.T) {
	const check = "123456789"

	if got, want := crc16X25([]byte(check)), uint16(0x906E); got != want {
		t.Errorf("X-25 CRC-16 check value = 0x%04X, want 0x%04X", got, want)
	}
	if got, want := crc32Castagnoli([]byte(check)), uint32(0xE3069283); got != want {
		t.Errorf("CRC-32C check value = 0x%08X, want 0x%08X", got, want)
	}
}

// The CCSDS CRC-16 in pkg/crc is a different algorithm. If these ever agree,
// the comment in crc.go explaining why this package carries its own is wrong.
func TestCRC16IsNotTheCCSDSOne(t *testing.T) {
	// CCSDS CRC-16 over "123456789" is 0x29B1, the value pkg/crc pins.
	if crc16X25([]byte("123456789")) == 0x29B1 {
		t.Error("X-25 CRC-16 produced the CCSDS check value; one of them is wrong")
	}
}

func TestCRCTypeSizes(t *testing.T) {
	tests := []struct {
		crcType CRCType
		size    int
		valid   bool
	}{
		{CRCNone, 0, true},
		{CRC16X25, 2, true},
		{CRC32C, 4, true},
		{CRCType(3), 0, false},
		{CRCType(255), 0, false},
	}
	for _, tt := range tests {
		if got := tt.crcType.size(); got != tt.size {
			t.Errorf("CRCType(%d).size() = %d, want %d", tt.crcType, got, tt.size)
		}
		if got := tt.crcType.valid(); got != tt.valid {
			t.Errorf("CRCType(%d).valid() = %v, want %v", tt.crcType, got, tt.valid)
		}
	}
}

// The checksum covers its own field, zero-filled. Round-tripping fill and
// check proves the two halves agree; the vectors in the block tests prove they
// agree with the RFC.
func TestCRCFillAndCheck(t *testing.T) {
	for _, crcType := range []CRCType{CRC16X25, CRC32C} {
		block := cbor.AppendArrayHeader(nil, 3)
		block = cbor.AppendUint(block, 7)
		block = cbor.AppendUint(block, uint64(crcType))
		block = appendZeroCRC(block, crcType)

		n := crcType.size()
		fillCRC(block, crcType)

		if err := checkCRC(block, crcType, block[len(block)-n:]); err != nil {
			t.Errorf("CRCType(%d): checkCRC after fillCRC: %v", crcType, err)
		}

		// Flip a content octet and the checksum must stop matching.
		block[1] ^= 0xFF
		if err := checkCRC(block, crcType, block[len(block)-n:]); err == nil {
			t.Errorf("CRCType(%d): a corrupted block still checked out", crcType)
		}
	}
}
