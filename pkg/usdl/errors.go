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
	ErrInvalidMAPID = errors.New("invalid MAP ID: must be in range 0-15 (4 bits)")

	// ErrInvalidFrameLength indicates the frame length field is out of range.
	ErrInvalidFrameLength = errors.New("invalid frame length: exceeds maximum of 65536 bytes")

	// ErrFrameLengthMismatch indicates the decoded frame length field does
	// not match the length of the delivered frame buffer.
	ErrFrameLengthMismatch = errors.New("frame length field does not match buffer length")

	// ErrInvalidVCFCountLen indicates the VCF Count length exceeds 7 octets.
	ErrInvalidVCFCountLen = errors.New("invalid VCF count length: must be in range 0-7 octets")

	// ErrInvalidVCFCount indicates the VCF Count does not fit its field width.
	ErrInvalidVCFCount = errors.New("invalid VCF count: exceeds configured field width")

	// ErrInvalidHeaderSpare indicates the reserved spare bits of the primary
	// header are not zero.
	ErrInvalidHeaderSpare = errors.New("invalid primary header: reserved spare bits must be 00")

	// ErrTruncatedFrameFields indicates a truncated frame carries fields it
	// cannot have (insert zone, OCF, FECF, or a pointer-carrying rule).
	ErrTruncatedFrameFields = errors.New("truncated frame cannot carry insert zone, OCF, FECF, or a pointer")

	// ErrInvalidFECSize indicates the FECF size is not 0 or 2 octets. The
	// USLP FECF, when present, is always the 16-bit CRC of §4.1.6.2.2.
	ErrInvalidFECSize = errors.New("invalid FECF size: must be 0 or 2 octets (USLP has only the 16-bit FECF)")

	// ErrTruncatedFrameTooShort indicates a truncated frame with an empty
	// TFDZ (annex D1.3.2 note 2: minimum 6 octets in total).
	ErrTruncatedFrameTooShort = errors.New("truncated frame TFDZ must carry at least one octet (minimum frame length 6)")

	// ErrTruncatedFrameTooLong indicates a truncated frame over 32 octets
	// (annex D1.3.2 note 3 and D1.4.2.4).
	ErrTruncatedFrameTooLong = errors.New("truncated frame exceeds the 32-octet maximum length")

	// ErrNoOCFSupplier indicates the channel is configured with HasOCF but
	// no OCF supplier is installed; the OCF content must come from the OCF
	// service user (§4.1.5) rather than being fabricated as zeros.
	ErrNoOCFSupplier = errors.New("channel requires an OCF but no supplier is set: call SetOCFSupplier")

	// ErrOctetStreamFixedLength indicates an octet stream was sent on a
	// fixed-length channel (CCSDS 732.1-B-3 §4.2.4.1 forbids it).
	ErrOctetStreamFixedLength = errors.New("octet stream service requires variable-length transfer frames")

	// ErrInvalidConstructionRule indicates an invalid TFDZ construction rule.
	ErrInvalidConstructionRule = errors.New("invalid TFDZ construction rule: must be in range 0-7 (3 bits)")

	// ErrInvalidPointer indicates the FHP/LVOP is out of range for the TFDZ.
	ErrInvalidPointer = errors.New("invalid pointer: exceeds data zone length")

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
