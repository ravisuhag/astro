package pxdl

import "errors"

// Sentinel errors returned by the Proximity-1 data link codecs.
var (
	// ErrDataTooShort indicates the input is shorter than the fields it must contain.
	ErrDataTooShort = errors.New("data too short for the Proximity-1 field being read")

	// ErrInvalidVersion indicates a Transfer Frame Version Number other than
	// binary '10', which CCSDS 211.0-B-6 §3.2.2.2.2 requires for Version-3.
	ErrInvalidVersion = errors.New("invalid transfer frame version: Version-3 requires binary '10'")

	// ErrInvalidFrameLength indicates a frame length outside the 5 to 2048
	// octet range of §3.2.2.10.2.
	ErrInvalidFrameLength = errors.New("invalid frame length: must be 5 to 2048 octets")

	// ErrDataTooLarge indicates a data field beyond the 2043 octets §3.2.1 allows.
	ErrDataTooLarge = errors.New("data field exceeds the maximum of 2043 octets")

	// ErrInvalidSCID indicates a spacecraft identifier beyond the 10-bit field.
	ErrInvalidSCID = errors.New("invalid spacecraft ID: must fit 10 bits")

	// ErrInvalidPortID indicates a port identifier beyond the 3-bit field.
	ErrInvalidPortID = errors.New("invalid port ID: must fit 3 bits")

	// ErrPortIDOnSupervisoryFrame indicates a P-frame carrying a non-zero
	// Port ID, which CCSDS 211.0-B-6 §3.2.2.8.2 forbids.
	ErrPortIDOnSupervisoryFrame = errors.New("port ID must be zero on a supervisory frame")

	// ErrInvalidPCID indicates a physical channel identifier beyond one bit.
	ErrInvalidPCID = errors.New("invalid physical channel ID: must be 0 or 1")

	// ErrInvalidDFCID indicates a Data Field Construction ID that is reserved,
	// or non-zero on a P-frame where §3.2.2.5.2 requires '00'.
	ErrInvalidDFCID = errors.New("invalid data field construction ID")

	// ErrNotUserFrame indicates a user-data operation on a P-frame.
	ErrNotUserFrame = errors.New("frame carries supervisory data, not user data")

	// ErrNotSupervisoryFrame indicates a supervisory operation on a U-frame.
	ErrNotSupervisoryFrame = errors.New("frame carries user data, not supervisory data")

	// ErrInvalidQoS indicates an SPDU on the Sequence Controlled service.
	// §3.2.4.1 allows SPDUs only on Expedited.
	ErrInvalidQoS = errors.New("supervisory PDUs may travel only on the Expedited service")

	// ErrInvalidSPDU indicates a malformed supervisory PDU.
	ErrInvalidSPDU = errors.New("invalid supervisory PDU")

	// ErrSPDUDataTooLarge indicates a variable-length SPDU data field beyond
	// the 15 octets its 4-bit length field can describe.
	ErrSPDUDataTooLarge = errors.New("supervisory PDU data field exceeds 15 octets")

	// ErrInvalidSegment indicates a segment header or reassembly failure.
	ErrInvalidSegment = errors.New("invalid packet segment")

	// ErrSegmentOutOfOrder indicates a continuing or last segment arriving
	// before a first segment for its routing ID (§3.2.3.3.5 b).
	ErrSegmentOutOfOrder = errors.New("segment arrived before the first segment of its packet")

	// ErrReassemblyTooLarge indicates an accumulating packet beyond the
	// configured maximum.
	ErrReassemblyTooLarge = errors.New("reassembled packet exceeds the maximum size")
)
