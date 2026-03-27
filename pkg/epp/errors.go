package epp

import "errors"

var (
	// ErrInvalidPVN indicates the Packet Version Number is not 7 (0111).
	ErrInvalidPVN = errors.New("invalid PVN: must be 7 for encapsulation packets")

	// ErrInvalidProtocolID indicates the Protocol ID is out of range.
	ErrInvalidProtocolID = errors.New("invalid protocol ID: must be in the range 0-7")

	// ErrIdleWithData indicates an idle packet was created with a non-empty data zone.
	ErrIdleWithData = errors.New("idle packet must not contain user data")

	// ErrEmptyData indicates a non-idle packet has no data.
	ErrEmptyData = errors.New("non-idle packet must contain data")

	// ErrDataTooShort indicates the provided data is too short for decoding.
	ErrDataTooShort = errors.New("provided data is too short to decode the packet")

	// ErrPacketLengthMismatch indicates the declared packet length does not match the actual size.
	ErrPacketLengthMismatch = errors.New("packet length field does not match actual packet size")

	// ErrPacketTooLarge indicates the packet exceeds the maximum size for its header format.
	ErrPacketTooLarge = errors.New("packet size exceeds the maximum for the selected header format")

	// ErrNilPacket indicates a nil packet was provided.
	ErrNilPacket = errors.New("packet must not be nil")

	// ErrInvalidLengthOfLength indicates the Length of Length field is not 0 or 1.
	ErrInvalidLengthOfLength = errors.New("invalid length of length: must be 0 or 1")
)
