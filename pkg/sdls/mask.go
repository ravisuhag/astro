package sdls

// Baseline authentication bit mask constructors.
//
// Clause 4.2.2.6.2 of CCSDS 355.0-B-2 sets per-frame-type rules for the
// authentication bit mask: some header fields must be covered (the Virtual
// Channel ID, the Security Header, the USLP MAP ID, the TC Segment Header)
// and some must not be, because they change legitimately between the sender
// applying security and the receiver checking it. The mandatory exclusions
// are:
//
//   - the TM Master Channel Frame Count, rewritten when frames from several
//     virtual channels are multiplexed onto a master channel;
//   - the AOS Frame Header Error Control, computed downstream of security;
//   - the Insert Zone of AOS and USLP frames, which is not secured;
//   - the Initialization Vector inside the Security Header (clause 4.2.2.6.2 h).
//
// Each constructor here returns the strictest mask that honours those
// exclusions for one frame type: every other header octet is covered with
// ones. Missions that want to exclude more (clause 4.2.2.6.2 permits leaving
// unspecified fields out) can clear further octets in the returned slice.
//
// The frame header the mask applies to is the same byte string handed to
// ApplySecurity and ProcessSecurity: the carrier frame's octets from the
// start of the Transfer Frame Primary Header to where the Security Header
// begins, including any secondary header, frame error control position, or
// insert zone the caller passes.

// Frame header geometry the constructors rely on.
const (
	// tmPrimaryHeaderSize is the TM Transfer Frame Primary Header width
	// (CCSDS 132.0-B clause 4.1.2).
	tmPrimaryHeaderSize = 6
	// tmMCFCOffset is the octet offset of the TM Master Channel Frame Count
	// within the primary header (CCSDS 132.0-B clause 4.1.2.4).
	tmMCFCOffset = 2
	// tcPrimaryHeaderSize is the TC Transfer Frame Primary Header width
	// (CCSDS 232.0-B clause 4.1.2).
	tcPrimaryHeaderSize = 5
	// tcSegmentHeaderSize is the optional TC Segment Header width
	// (CCSDS 232.0-B clause 4.1.3.2).
	tcSegmentHeaderSize = 1
	// aosPrimaryHeaderSize is the AOS Transfer Frame Primary Header width
	// without the optional Frame Header Error Control (CCSDS 732.0-B clause 4.1.2).
	aosPrimaryHeaderSize = 6
	// aosFHECSize is the optional AOS Frame Header Error Control width
	// (CCSDS 732.0-B clause 4.1.2.6).
	aosFHECSize = 2
)

// BaselineAuthMaskTM builds the clause 4.2.2.6.2 authentication bit mask for a TM
// Transfer Frame whose header is the 6-octet primary header followed by
// secondaryHeaderLen octets of Frame Secondary Header (zero when absent).
//
// The mask covers every header octet except the Master Channel Frame Count,
// which multiplexing rewrites after security is applied, and the
// Initialization Vector inside the Security Header. fl is the SA's field
// layout, used to size and place the security header portion of the mask.
func BaselineAuthMaskTM(secondaryHeaderLen int, fl FieldLengths) []byte {
	mask := onesMask(tmPrimaryHeaderSize+secondaryHeaderLen, fl)
	mask[tmMCFCOffset] = 0
	return mask
}

// BaselineAuthMaskTC builds the clause 4.2.2.6.2 authentication bit mask for a TC
// Transfer Frame: the 5-octet primary header, plus the 1-octet Segment Header
// when the virtual channel uses segmentation. Clause 4.2.2.6.2 requires the Segment
// Header to be covered, and TC has no mandatorily excluded header field, so
// the whole header is ones; only the Initialization Vector position inside
// the Security Header is cleared (zero octets wide under the clause E2 CMAC
// baseline anyway).
func BaselineAuthMaskTC(hasSegmentHeader bool, fl FieldLengths) []byte {
	size := tcPrimaryHeaderSize
	if hasSegmentHeader {
		size += tcSegmentHeaderSize
	}
	return onesMask(size, fl)
}

// BaselineAuthMaskAOS builds the clause 4.2.2.6.2 authentication bit mask for an
// AOS Transfer Frame: the 6-octet primary header, the 2-octet Frame Header
// Error Control when the mission uses it, and insertZoneLen octets of Insert
// Zone (zero when absent).
//
// The mask covers the primary header and clears the mandatory exclusions:
// the FHEC (computed downstream of security), the Insert Zone (not part of
// the secured data), and the Initialization Vector inside the Security
// Header.
func BaselineAuthMaskAOS(hasFHEC bool, insertZoneLen int, fl FieldLengths) []byte {
	size := aosPrimaryHeaderSize
	fhecStart := size
	if hasFHEC {
		size += aosFHECSize
	}
	insertStart := size
	size += insertZoneLen

	mask := onesMask(size, fl)
	if hasFHEC {
		zeroRange(mask, fhecStart, aosFHECSize)
	}
	zeroRange(mask, insertStart, insertZoneLen)
	return mask
}

// BaselineAuthMaskUSLP builds the clause 4.2.2.6.2 authentication bit mask for a
// USLP Transfer Frame whose primary header (variable in USLP, 4 to 14
// octets) is primaryHeaderLen octets, followed by insertZoneLen octets of
// Insert Zone (zero when absent).
//
// The mask covers the whole primary header, including the MAP ID that
// Clause 4.2.2.6.2 requires, and clears the mandatory exclusions: the Insert Zone
// and the Initialization Vector inside the Security Header.
func BaselineAuthMaskUSLP(primaryHeaderLen, insertZoneLen int, fl FieldLengths) []byte {
	mask := onesMask(primaryHeaderLen+insertZoneLen, fl)
	zeroRange(mask, primaryHeaderLen, insertZoneLen)
	return mask
}

// onesMask returns a mask of ones over frameHeaderLen octets of frame header
// plus the security header described by fl, with the Initialization Vector
// position cleared per clause 4.2.2.6.2 h).
func onesMask(frameHeaderLen int, fl FieldLengths) []byte {
	mask := make([]byte, frameHeaderLen+fl.HeaderSize())
	for i := range mask {
		mask[i] = 0xFF
	}
	zeroRange(mask, frameHeaderLen+SPISize, fl.IV)
	return mask
}

// zeroRange clears length octets of mask starting at offset.
func zeroRange(mask []byte, offset, length int) {
	for i := offset; i < offset+length && i < len(mask); i++ {
		mask[i] = 0
	}
}
