package aos

import (
	"errors"

	"github.com/ravisuhag/astro/pkg/sdl"
)

var (
	// ErrDataTooShort indicates the provided data is too short for decoding.
	ErrDataTooShort = errors.New("provided data is too short to decode")

	// ErrInvalidVersion indicates the TFVN is not 1 (0b01) for AOS.
	ErrInvalidVersion = errors.New("invalid version: TFVN must be 1 (0b01) for AOS")

	// ErrInvalidSpacecraftID indicates the spacecraft ID is out of range.
	ErrInvalidSpacecraftID = errors.New("invalid spacecraft ID: must be in range 0-255 (8 bits)")

	// ErrInvalidVCID indicates the virtual channel ID is out of range.
	ErrInvalidVCID = errors.New("invalid virtual channel ID: must be in range 0-63 (6 bits)")

	// ErrInvalidVCFrameCount indicates the VC frame count exceeds 24 bits.
	ErrInvalidVCFrameCount = errors.New("invalid VC frame count: exceeds 24 bits")

	// ErrInvalidVCFrameCountCycle indicates the VC frame count cycle is out of range.
	ErrInvalidVCFrameCountCycle = errors.New("invalid VC frame count cycle: must be in range 0-15 (4 bits)")

	// ErrInvalidFirstHeaderPointer indicates the M_PDU first header pointer is out of range.
	ErrInvalidFirstHeaderPointer = errors.New("invalid first header pointer: exceeds 11 bits")

	// ErrInvalidBitstreamDataPointer indicates the B_PDU bitstream data pointer is out of range.
	ErrInvalidBitstreamDataPointer = errors.New("invalid bitstream data pointer: exceeds 14 bits")

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

	// ErrInvalidInsertZoneLength indicates the insert zone length does not match channel config.
	ErrInvalidInsertZoneLength = errors.New("insert zone length does not match configured length")

	// ErrSCIDMismatch indicates the frame SCID does not match the master channel SCID.
	ErrSCIDMismatch = errors.New("frame SCID does not match master channel SCID")

	// ErrVirtualChannelNotFound indicates no virtual channel exists for the given VCID.
	ErrVirtualChannelNotFound = errors.New("virtual channel not found for specified VCID")

	// ErrDataFieldTooSmall indicates the data field capacity is too small for framing.
	ErrDataFieldTooSmall = errors.New("data field capacity too small")

	// ErrNoPacketSizer indicates no PacketSizer has been set on the M_PDU service.
	ErrNoPacketSizer = errors.New("no PacketSizer configured: call SetPacketSizer before Receive")

	// ErrNoFramesAvailable aliases sdl.ErrNoFramesAvailable.
	ErrNoFramesAvailable = sdl.ErrNoFramesAvailable

	// ErrBufferFull aliases sdl.ErrBufferFull.
	ErrBufferFull = sdl.ErrBufferFull

	// ErrServiceNotFound aliases sdl.ErrServiceNotFound.
	ErrServiceNotFound = sdl.ErrServiceNotFound

	// ErrMasterChannelNotFound aliases sdl.ErrMasterChannelNotFound.
	ErrMasterChannelNotFound = sdl.ErrMasterChannelNotFound

	// ErrNoVirtualChannels aliases sdl.ErrNoChannels.
	ErrNoVirtualChannels = sdl.ErrNoChannels

	// ErrNoMasterChannels aliases sdl.ErrNoMasterChannels.
	ErrNoMasterChannels = sdl.ErrNoMasterChannels
)
