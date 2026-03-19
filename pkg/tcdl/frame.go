package tcdl

import (
	"encoding/binary"

	"github.com/ravisuhag/astro/pkg/crc"
)

const (
	// PrimaryHeaderSize is the size of the TC primary header in bytes.
	PrimaryHeaderSize = 5
	// MaxFrameLength is the maximum total TC frame length in bytes.
	MaxFrameLength = 1024
	// FECSize is the size of the Frame Error Control field in bytes.
	FECSize = 2
)

// TCTransferFrame represents a CCSDS TC Space Data Link Protocol Transfer Frame.
type TCTransferFrame struct {
	Header            PrimaryHeader
	SegmentHeader     *SegmentHeader // optional, present when MAP sublayer is used
	DataField         []byte         // Frame Data Field
	FrameErrorControl uint16         // 16-bit CRC-16-CCITT
}

// FrameOption configures optional fields on a TCTransferFrame.
type FrameOption func(*TCTransferFrame)

// WithBypass sets the Bypass Flag to 1 (Type-B expedited frame).
func WithBypass() FrameOption {
	return func(f *TCTransferFrame) {
		f.Header.BypassFlag = 1
	}
}

// WithControlCommand sets the Control Command Flag to 1.
func WithControlCommand() FrameOption {
	return func(f *TCTransferFrame) {
		f.Header.ControlCommandFlag = 1
	}
}

// WithSegmentHeader attaches a segment header to the frame.
func WithSegmentHeader(sh SegmentHeader) FrameOption {
	return func(f *TCTransferFrame) {
		f.SegmentHeader = &sh
	}
}

// WithSequenceNumber sets the Frame Sequence Number (N(S) for COP-1).
func WithSequenceNumber(n uint8) FrameOption {
	return func(f *TCTransferFrame) {
		f.Header.FrameSequenceNum = n
	}
}

// NewTCTransferFrame creates a new TC Transfer Frame.
// The frame length is automatically computed. CRC is auto-calculated.
func NewTCTransferFrame(scid uint16, vcid uint8, data []byte, opts ...FrameOption) (*TCTransferFrame, error) {
	frame := &TCTransferFrame{
		Header: PrimaryHeader{
			VersionNumber:    0,
			SpacecraftID:     scid & 0x03FF,
			VirtualChannelID: vcid & 0x3F,
		},
		DataField: data,
	}

	for _, opt := range opts {
		opt(frame)
	}

	// Compute total frame length
	totalLen := PrimaryHeaderSize + len(data) + FECSize
	if frame.SegmentHeader != nil {
		totalLen++
	}
	if totalLen > MaxFrameLength {
		return nil, ErrDataTooLarge
	}
	frame.Header.FrameLength = uint16(totalLen - 1)

	// Compute CRC
	encoded, err := frame.EncodeWithoutFEC()
	if err != nil {
		return nil, err
	}
	frame.FrameErrorControl = crc.ComputeCRC16(encoded)

	return frame, nil
}

// Encode converts the TC Transfer Frame to a byte slice including CRC.
func (tf *TCTransferFrame) Encode() ([]byte, error) {
	frameData, err := tf.EncodeWithoutFEC()
	if err != nil {
		return nil, err
	}
	crcBytes := make([]byte, 2)
	binary.BigEndian.PutUint16(crcBytes, tf.FrameErrorControl)
	return append(frameData, crcBytes...), nil
}

// EncodeWithoutFEC converts the frame to bytes excluding the CRC field.
func (tf *TCTransferFrame) EncodeWithoutFEC() ([]byte, error) {
	header, err := tf.Header.Encode()
	if err != nil {
		return nil, err
	}

	frameData := make([]byte, 0, PrimaryHeaderSize+1+len(tf.DataField))
	frameData = append(frameData, header...)

	if tf.SegmentHeader != nil {
		sh, err := tf.SegmentHeader.Encode()
		if err != nil {
			return nil, err
		}
		frameData = append(frameData, sh...)
	}

	frameData = append(frameData, tf.DataField...)
	return frameData, nil
}

// DecodeTCTransferFrame parses a byte slice into a TC Transfer Frame.
// Verifies CRC integrity.
func DecodeTCTransferFrame(data []byte) (*TCTransferFrame, error) {
	if len(data) < PrimaryHeaderSize+FECSize {
		return nil, ErrDataTooShort
	}

	// Decode primary header
	var header PrimaryHeader
	if err := header.Decode(data[:PrimaryHeaderSize]); err != nil {
		return nil, err
	}

	// Verify frame length matches data
	expectedLen := int(header.FrameLength) + 1
	if len(data) < expectedLen {
		return nil, ErrDataTooShort
	}

	// Verify CRC
	receivedCRC := binary.BigEndian.Uint16(data[expectedLen-FECSize : expectedLen])
	computedCRC := crc.ComputeCRC16(data[:expectedLen-FECSize])
	if receivedCRC != computedCRC {
		return nil, ErrCRCMismatch
	}

	// Extract data field (between header and CRC)
	dataStart := PrimaryHeaderSize
	dataEnd := expectedLen - FECSize
	dataField := make([]byte, dataEnd-dataStart)
	copy(dataField, data[dataStart:dataEnd])

	return &TCTransferFrame{
		Header:            header,
		DataField:         dataField,
		FrameErrorControl: receivedCRC,
	}, nil
}

// IsControlFrame reports whether the frame is a control command frame.
func IsControlFrame(frame *TCTransferFrame) bool {
	return frame.Header.ControlCommandFlag == 1
}

// IsBypass reports whether the frame is a Type-B (bypass/expedited) frame.
func IsBypass(frame *TCTransferFrame) bool {
	return frame.Header.BypassFlag == 1
}

// recomputeCRC re-encodes the frame (without FEC) and updates FrameErrorControl.
func recomputeCRC(frame *TCTransferFrame) error {
	encoded, err := frame.EncodeWithoutFEC()
	if err != nil {
		return err
	}
	frame.FrameErrorControl = crc.ComputeCRC16(encoded)
	return nil
}
