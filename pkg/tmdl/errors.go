package tmdl

import "errors"

var (
	// ErrDataTooShort indicates the provided data is too short for decoding.
	ErrDataTooShort = errors.New("provided data is too short to decode")

	// ErrInvalidVersion indicates the version number is not 0 for TM Transfer Frame.
	ErrInvalidVersion = errors.New("invalid version: must be 0 for TM Transfer Frame")

	// ErrInvalidSpacecraftID indicates the spacecraft ID is out of range.
	ErrInvalidSpacecraftID = errors.New("invalid spacecraft ID: must be in range 0-1023 (10 bits)")

	// ErrInvalidVCID indicates the virtual channel ID is out of range.
	ErrInvalidVCID = errors.New("invalid virtual channel ID: must be in range 0-7 (3 bits)")

	// ErrInvalidPacketOrderFlag indicates the packet order flag is set when sync flag is 0.
	ErrInvalidPacketOrderFlag = errors.New("invalid packet order flag: must be 0 when sync flag is 0")

	// ErrInvalidSegmentLengthID indicates the segment length ID is invalid for the current sync flag.
	ErrInvalidSegmentLengthID = errors.New("invalid segment length ID: must be 3 (0b11) when sync flag is 0")

	// ErrInvalidFirstHeaderPtr indicates the first header pointer is out of range or inconsistent.
	ErrInvalidFirstHeaderPtr = errors.New("invalid first header pointer: must be in range 0-2047 (11 bits)")

	// ErrInvalidSecondaryHeaderVersion indicates the secondary header version is not 0.
	ErrInvalidSecondaryHeaderVersion = errors.New("invalid secondary header version: must be 0 for Version 1")

	// ErrInvalidHeaderLength indicates the secondary header length is out of range.
	ErrInvalidHeaderLength = errors.New("invalid header length: must be in range 0-63 (6 bits)")

	// ErrCRCMismatch indicates the received CRC does not match the computed CRC.
	ErrCRCMismatch = errors.New("CRC mismatch: received CRC does not match computed CRC")

	// ErrDataTooLarge indicates the data field exceeds the maximum frame length.
	ErrDataTooLarge = errors.New("data field exceeds maximum frame length")

	// ErrEmptyData indicates that the provided data is empty.
	ErrEmptyData = errors.New("data cannot be empty")

	// ErrNoFramesAvailable indicates there are no frames to retrieve.
	ErrNoFramesAvailable = errors.New("no frames available")

	// ErrBufferFull indicates the virtual channel buffer is full.
	ErrBufferFull = errors.New("virtual channel buffer full")

	// ErrSCIDMismatch indicates the frame SCID does not match the master channel SCID.
	ErrSCIDMismatch = errors.New("frame SCID does not match master channel SCID")

	// ErrSizeMismatch indicates the data size does not match the expected fixed size.
	ErrSizeMismatch = errors.New("data size does not match expected fixed size")

	// ErrServiceNotFound indicates the requested service was not found.
	ErrServiceNotFound = errors.New("service not found for specified VCID and service type")

	// ErrMasterChannelNotFound indicates the requested master channel was not found.
	ErrMasterChannelNotFound = errors.New("master channel service not found for specified SCID")

	// ErrNoVirtualChannels indicates no virtual channels are registered.
	ErrNoVirtualChannels = errors.New("no virtual channels available")

	// ErrVirtualChannelNotFound indicates no virtual channel exists for the given VCID.
	ErrVirtualChannelNotFound = errors.New("virtual channel not found for specified VCID")

	// ErrPacketTooLarge indicates the packet exceeds the maximum size for length-prefixed framing.
	ErrPacketTooLarge = errors.New("packet exceeds maximum reassembly size (65535 bytes)")

	// ErrDataFieldTooSmall indicates the data field capacity is too small for the length prefix.
	ErrDataFieldTooSmall = errors.New("data field capacity too small for length prefix")

	// ErrIncompletePacket indicates reassembly failed due to unexpected frame sequence.
	ErrIncompletePacket = errors.New("incomplete packet: unexpected frame during reassembly")

	// ErrInvalidPVN indicates the packet version number is not in the valid set.
	ErrInvalidPVN = errors.New("invalid packet version number")

	// ErrNoMasterChannels indicates no master channels are registered on the physical channel.
	ErrNoMasterChannels = errors.New("no master channels registered")
)
