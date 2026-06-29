package cfdp

import "fmt"

// RecordContinuationState says how a File Data PDU's payload sits relative to
// record boundaries, per CCSDS 727.0-B-5 §5.3. It is present only when the
// header's segment metadata flag is set.
type RecordContinuationState uint8

const (
	// RecordNeitherStartNorEnd means the payload holds no record boundary. With
	// segmentation control set it continues a record from an earlier PDU;
	// otherwise the file simply has no records.
	RecordNeitherStartNorEnd RecordContinuationState = 0
	// RecordStartOnly means the payload starts a record that does not end here.
	RecordStartOnly RecordContinuationState = 1
	// RecordEndOnly means the payload ends a record that began earlier.
	RecordEndOnly RecordContinuationState = 2
	// RecordStartAndEnd means the payload holds one or more whole records.
	RecordStartAndEnd RecordContinuationState = 3
)

// String names the continuation state.
func (r RecordContinuationState) String() string {
	switch r {
	case RecordNeitherStartNorEnd:
		return "neither start nor end"
	case RecordStartOnly:
		return "start of record"
	case RecordEndOnly:
		return "end of record"
	default:
		return "start and end of record"
	}
}

// MaxSegmentMetadataSize is the widest segment metadata a 6-bit length field
// can describe (§5.3, table 5-14).
const MaxSegmentMetadataSize = 63

// FileDataPDU carries a run of file octets at a given offset.
type FileDataPDU struct {
	// RecordContinuation and SegmentMetadata are present only when the
	// header's segment metadata flag is set.
	RecordContinuation RecordContinuationState
	SegmentMetadata    []byte

	// Offset is where this data belongs in the file, in octets from the start.
	Offset uint64 // FSS
	// Data is the file content itself.
	Data []byte
}

// Encode serializes the File Data PDU data field.
//
// segmentMetadataPresent and largeFile must match the flags in the PDU header
// that will carry this data field; the wire format is not self-describing.
func (p *FileDataPDU) Encode(segmentMetadataPresent, largeFile bool) ([]byte, error) {
	var out []byte

	if segmentMetadataPresent {
		if len(p.SegmentMetadata) > MaxSegmentMetadataSize {
			return nil, ErrSegmentTooLarge
		}
		// Record continuation state (2 bits) | segment metadata length (6 bits).
		out = append(out, byte(p.RecordContinuation&0x03)<<6|byte(len(p.SegmentMetadata)&0x3F))
		out = append(out, p.SegmentMetadata...)
	}

	out = appendFSS(out, p.Offset, largeFile)
	return append(out, p.Data...), nil
}

// DecodeFileDataPDU parses a File Data PDU data field. The flags must match
// the header that carried it.
func DecodeFileDataPDU(data []byte, segmentMetadataPresent, largeFile bool) (*FileDataPDU, error) {
	p := &FileDataPDU{}
	offset := 0

	if segmentMetadataPresent {
		if len(data) < 1 {
			return nil, ErrDataTooShort
		}
		p.RecordContinuation = RecordContinuationState(data[0] >> 6 & 0x03)
		n := int(data[0] & 0x3F)
		offset = 1
		if len(data) < offset+n {
			return nil, ErrDataTooShort
		}
		if n > 0 {
			p.SegmentMetadata = make([]byte, n)
			copy(p.SegmentMetadata, data[offset:offset+n])
		}
		offset += n
	}

	fileOffset, n, err := readFSS(data[offset:], largeFile)
	if err != nil {
		return nil, err
	}
	p.Offset = fileOffset
	offset += n

	// The remainder is file data. Copy it so the PDU never aliases the caller's
	// buffer, matching how the frame decoders in this repo behave.
	p.Data = make([]byte, len(data)-offset)
	copy(p.Data, data[offset:])
	return p, nil
}

// End returns the file offset just past this segment's last octet.
func (p *FileDataPDU) End() uint64 {
	return p.Offset + uint64(len(p.Data))
}

// Humanize returns a human-readable summary.
func (p *FileDataPDU) Humanize() string {
	return fmt.Sprintf("CFDP File Data PDU\n  Offset ... %d\n  Length ... %d octets",
		p.Offset, len(p.Data))
}
