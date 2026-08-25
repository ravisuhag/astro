package epp

import "errors"

var (
	// ErrInvalidPVN indicates the Packet Version Number is not 7 ('111').
	ErrInvalidPVN = errors.New("invalid PVN: must be 7 ('111') for encapsulation packets")

	// ErrInvalidProtocolID indicates the Protocol ID is out of range.
	ErrInvalidProtocolID = errors.New("invalid protocol ID: must be in the range 0-7")

	// ErrInvalidLengthOfLength indicates the Length of Length field is not 0-3.
	ErrInvalidLengthOfLength = errors.New("invalid length of length: must be in the range 0-3")

	// ErrInvalidUserDefined indicates the User Defined Field does not fit in 4 bits.
	ErrInvalidUserDefined = errors.New("invalid user defined field: must be in the range 0-15")

	// ErrInvalidExtendedProtocolID indicates the Protocol ID Extension does not fit in 4 bits.
	ErrInvalidExtendedProtocolID = errors.New("invalid protocol ID extension: must be in the range 0-15")

	// ErrNonIdleOneOctetHeader indicates a 1-octet header (LoL '00') with a
	// non-idle Protocol ID, which CCSDS 133.1-B-3 4.1.2.4.4 forbids.
	ErrNonIdleOneOctetHeader = errors.New("length of length '00' requires protocol ID '000' (idle)")

	// ErrExtendedNeedsLongHeader indicates Protocol ID '110' with a header too
	// short to carry the Protocol ID Extension field.
	ErrExtendedNeedsLongHeader = errors.New("protocol ID '110' requires a 4- or 8-octet header")

	// ErrExtensionMustBeZero indicates a non-zero Protocol ID Extension while
	// the Protocol ID is not '110' (CCSDS 133.1-B-3 4.1.2.6.3).
	ErrExtensionMustBeZero = errors.New("protocol ID extension must be zero unless protocol ID is '110'")

	// ErrFieldNeedsLongerHeader indicates a header field is set that does not
	// exist in the selected header size (e.g. a CCSDS Defined value with a
	// 4-octet header).
	ErrFieldNeedsLongerHeader = errors.New("header field not present in the selected header size")

	// ErrIdleWithData indicates a 1-octet idle packet was given a data zone.
	ErrIdleWithData = errors.New("1-octet idle packet has no data zone")

	// ErrEmptyData indicates a non-idle packet has no data
	// (CCSDS 133.1-B-3 4.1.3.1.5).
	ErrEmptyData = errors.New("non-idle packet must contain data")

	// ErrDataTooShort indicates the provided data is too short for decoding.
	ErrDataTooShort = errors.New("provided data is too short to decode the packet")

	// ErrPacketLengthMismatch indicates the declared packet length does not match the actual size.
	ErrPacketLengthMismatch = errors.New("packet length field does not match actual packet size")

	// ErrPacketTooLarge indicates the packet exceeds the maximum size for its header format.
	ErrPacketTooLarge = errors.New("packet size exceeds the maximum for the selected header format")

	// ErrInvalidIdleLength indicates an idle fill packet was requested with an
	// unrepresentable total length.
	ErrInvalidIdleLength = errors.New("idle packet total length must be at least 1 octet")

	// ErrNilPacket indicates a nil packet was provided.
	ErrNilPacket = errors.New("packet must not be nil")
)
