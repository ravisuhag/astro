package usdl

import (
	"errors"

	"github.com/ravisuhag/astro/pkg/sdl"
)

var (
	// ErrDataTooShort indicates the provided data is too short for decoding.
	ErrDataTooShort = errors.New("provided data is too short to decode")

	// ErrInvalidVersion indicates the TFVN is not 12 (0b1100) for USLP.
	ErrInvalidVersion = errors.New("invalid version: TFVN must be 12 (0b1100) for USLP")

	// ErrInvalidSpacecraftID indicates the spacecraft ID is out of range.
	ErrInvalidSpacecraftID = errors.New("invalid spacecraft ID: must be in range 0-65535 (16 bits)")

	// ErrInvalidVCID indicates the virtual channel ID is out of range.
	ErrInvalidVCID = errors.New("invalid virtual channel ID: must be in range 0-63 (6 bits)")

	// ErrInvalidMAPID indicates the MAP ID is out of range.
	ErrInvalidMAPID = errors.New("invalid MAP ID: must be in range 0-63 (6 bits)")

	// ErrInvalidFrameLength indicates the frame length field is out of range.
	ErrInvalidFrameLength = errors.New("invalid frame length: exceeds maximum of 65536 bytes")

	// ErrInvalidConstructionRule indicates an invalid TFDZ construction rule.
	ErrInvalidConstructionRule = errors.New("invalid TFDZ construction rule: must be in range 0-7 (3 bits)")

	// ErrInvalidFirstHeaderOffset indicates the first header offset is out of range.
	ErrInvalidFirstHeaderOffset = errors.New("invalid first header offset: exceeds data field length")

	// ErrCRCMismatch indicates the received CRC does not match the computed CRC.
	ErrCRCMismatch = errors.New("CRC mismatch: received CRC does not match computed CRC")

	// ErrDataTooLarge indicates the data field exceeds the maximum frame length.
	ErrDataTooLarge = errors.New("data field exceeds maximum frame length")

	// ErrEmptyData indicates that the provided data is empty.
	ErrEmptyData = errors.New("data cannot be empty")

	// ErrSizeMismatch indicates the data size does not match the expected fixed size.
	ErrSizeMismatch = errors.New("data size does not match expected fixed size")

	// ErrInvalidOCFLength indicates the OCF is not exactly 4 bytes.
	ErrInvalidOCFLength = errors.New("operational control field must be exactly 4 bytes when present")

	// ErrInvalidInsertZoneLength indicates the insert zone length is invalid.
	ErrInvalidInsertZoneLength = errors.New("insert zone length exceeds maximum")

	// ErrInvalidSequenceNumber indicates the sequence number is out of range.
	ErrInvalidSequenceNumber = errors.New("invalid sequence number: exceeds field width")

	// ErrNoFramesAvailable aliases sdl.ErrNoFramesAvailable.
	ErrNoFramesAvailable = sdl.ErrNoFramesAvailable

	// ErrBufferFull aliases sdl.ErrBufferFull.
	ErrBufferFull = sdl.ErrBufferFull

	// ErrSCIDMismatch indicates the frame SCID does not match the master channel SCID.
	ErrSCIDMismatch = errors.New("frame SCID does not match master channel SCID")

	// ErrServiceNotFound aliases sdl.ErrServiceNotFound.
	ErrServiceNotFound = sdl.ErrServiceNotFound

	// ErrMasterChannelNotFound aliases sdl.ErrMasterChannelNotFound.
	ErrMasterChannelNotFound = sdl.ErrMasterChannelNotFound

	// ErrNoVirtualChannels aliases sdl.ErrNoChannels.
	ErrNoVirtualChannels = sdl.ErrNoChannels

	// ErrVirtualChannelNotFound indicates no virtual channel exists for the given VCID.
	ErrVirtualChannelNotFound = errors.New("virtual channel not found for specified VCID")

	// ErrDataFieldTooSmall indicates the data field capacity is too small for framing.
	ErrDataFieldTooSmall = errors.New("data field capacity too small")

	// ErrNoPacketSizer indicates no PacketSizer has been set on the MAP Packet service.
	ErrNoPacketSizer = errors.New("no PacketSizer configured: call SetPacketSizer before Receive")

	// ErrNoMasterChannels aliases sdl.ErrNoMasterChannels.
	ErrNoMasterChannels = sdl.ErrNoMasterChannels
)
