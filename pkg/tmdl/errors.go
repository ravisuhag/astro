package tmdl

import (
	"errors"

	"github.com/ravisuhag/astro/pkg/sdl"
)

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

	// ErrFrameTooLong indicates a frame length past the 2048-octet maximum of
	// ECSS-E-ST-50-03C 5.1b.
	ErrFrameTooLong = errors.New("frame length exceeds the 2048-octet ECSS maximum")

	// ErrInvalidHeaderLength indicates the secondary header length is out of range.
	ErrInvalidHeaderLength = errors.New("invalid header length: must be in range 0-63 (6 bits)")

	// ErrCRCMismatch indicates the received CRC does not match the computed CRC.
	ErrCRCMismatch = errors.New("CRC mismatch: received CRC does not match computed CRC")

	// ErrDataTooLarge indicates the data field exceeds the maximum frame length.
	ErrDataTooLarge = errors.New("data field exceeds maximum frame length")

	// ErrEmptyData indicates that the provided data is empty.
	ErrEmptyData = errors.New("data cannot be empty")

	// ErrNoFramesAvailable aliases sdl.ErrNoFramesAvailable.
	ErrNoFramesAvailable = sdl.ErrNoFramesAvailable

	// ErrBufferFull aliases sdl.ErrBufferFull.
	ErrBufferFull = sdl.ErrBufferFull

	// ErrSCIDMismatch indicates the frame SCID does not match the master channel SCID.
	ErrSCIDMismatch = errors.New("frame SCID does not match master channel SCID")

	// ErrSizeMismatch indicates the data size does not match the expected fixed size.
	ErrSizeMismatch = errors.New("data size does not match expected fixed size")

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

	// ErrNoPacketSizer indicates no PacketSizer has been set on the VCP service.
	ErrNoPacketSizer = errors.New("no PacketSizer configured: call SetPacketSizer before Receive")

	// ErrNoMasterChannels aliases sdl.ErrNoMasterChannels.
	ErrNoMasterChannels = sdl.ErrNoMasterChannels

	// ErrInvalidOCFLength indicates the Operational Control Field is not exactly 4 bytes.
	ErrInvalidOCFLength = errors.New("operational control field must be exactly 4 bytes when OCF flag is set")

	// ErrFrameLengthMismatch indicates a frame whose encoded length differs
	// from the fixed ChannelConfig.FrameLength of its physical channel.
	ErrFrameLengthMismatch = errors.New("frame length does not match the fixed channel frame length")

	// ErrFSHNotPresent indicates an FSH_SDU was supplied for a channel whose
	// frames carry no Transfer Frame Secondary Header. Set
	// ChannelConfig.FSHDataLength so the frames have a header to fill.
	ErrFSHNotPresent = errors.New("frame secondary header service configured but frames carry no secondary header")

	// ErrOCFNotPresent indicates an OCF_SDU was supplied for a channel whose
	// frames carry no Operational Control Field. Set ChannelConfig.HasOCF.
	ErrOCFNotPresent = errors.New("operational control field service configured but frames carry no OCF")

	// ErrFSHSizeMismatch indicates an FSH_SDU whose length differs from the
	// channel's fixed ChannelConfig.FSHDataLength. CCSDS 132.0-B-3 clause 4.1.3.1.6
	// fixes the secondary header length for the channel, so a differently
	// sized SDU cannot be carried.
	ErrFSHSizeMismatch = errors.New("FSH_SDU length does not match the channel's fixed secondary header length")
)
