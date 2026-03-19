package tcdl

import (
	"errors"

	"github.com/ravisuhag/astro/pkg/sdl"
)

var (
	// ErrDataTooShort indicates the provided data is too short for decoding.
	ErrDataTooShort = errors.New("provided data is too short to decode")

	// ErrInvalidVersion indicates the version number is not 0 for TC Transfer Frame.
	ErrInvalidVersion = errors.New("invalid version: must be 0 for TC Transfer Frame")

	// ErrInvalidSpacecraftID indicates the spacecraft ID is out of range.
	ErrInvalidSpacecraftID = errors.New("invalid spacecraft ID: must be in range 0-1023 (10 bits)")

	// ErrInvalidVCID indicates the virtual channel ID is out of range.
	ErrInvalidVCID = errors.New("invalid virtual channel ID: must be in range 0-63 (6 bits)")

	// ErrInvalidFrameLength indicates the frame length field is out of range.
	ErrInvalidFrameLength = errors.New("invalid frame length: exceeds maximum of 1024 bytes")

	// ErrInvalidReservedBits indicates the reserved bits are not zero.
	ErrInvalidReservedBits = errors.New("invalid reserved bits: must be 00")

	// ErrInvalidMAPID indicates the MAP ID is out of range.
	ErrInvalidMAPID = errors.New("invalid MAP ID: must be in range 0-63 (6 bits)")

	// ErrInvalidSequenceFlags indicates the segment sequence flags are out of range.
	ErrInvalidSequenceFlags = errors.New("invalid sequence flags: must be in range 0-3 (2 bits)")

	// ErrCRCMismatch indicates the received CRC does not match the computed CRC.
	ErrCRCMismatch = errors.New("CRC mismatch: received CRC does not match computed CRC")

	// ErrDataTooLarge indicates the data exceeds the maximum TC frame capacity.
	ErrDataTooLarge = errors.New("data exceeds maximum TC frame capacity")

	// ErrEmptyData indicates that the provided data is empty.
	ErrEmptyData = errors.New("data cannot be empty")

	// ErrNoFramesAvailable aliases sdl.ErrNoFramesAvailable.
	ErrNoFramesAvailable = sdl.ErrNoFramesAvailable

	// ErrBufferFull aliases sdl.ErrBufferFull.
	ErrBufferFull = sdl.ErrBufferFull

	// ErrSCIDMismatch indicates the frame SCID does not match the master channel SCID.
	ErrSCIDMismatch = errors.New("frame SCID does not match master channel SCID")

	// ErrServiceNotFound aliases sdl.ErrServiceNotFound.
	ErrServiceNotFound = sdl.ErrServiceNotFound

	// ErrMasterChannelNotFound indicates the requested master channel was not found.
	ErrMasterChannelNotFound = errors.New("master channel not found for specified SCID")

	// ErrNoVirtualChannels aliases sdl.ErrNoChannels.
	ErrNoVirtualChannels = sdl.ErrNoChannels

	// ErrVirtualChannelNotFound indicates no virtual channel exists for the given VCID.
	ErrVirtualChannelNotFound = errors.New("virtual channel not found for specified VCID")

	// ErrNoMasterChannels indicates no master channels are registered on the physical channel.
	ErrNoMasterChannels = errors.New("no master channels registered")

	// ErrNoPacketSizer indicates no PacketSizer has been set on the MAP Packet service.
	ErrNoPacketSizer = errors.New("no PacketSizer configured: call SetPacketSizer before Receive")

	// ErrIncompleteSegment indicates a segment reassembly is incomplete.
	ErrIncompleteSegment = errors.New("incomplete segment: missing continuation or last segment")
)
