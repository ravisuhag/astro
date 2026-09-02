package bp

import (
	"fmt"

	"github.com/ravisuhag/astro/pkg/sdnv"
)

// BlockType is the 8-bit type code of a canonical block, per RFC 5050 clause 4.5.2.
type BlockType uint8

const (
	// BlockTypePayload is the bundle payload block. It is the only type
	// RFC 5050 assigns.
	BlockTypePayload BlockType = 1

	// BlockTypeECOS is the Extended Class of Service block that
	// CCSDS 734.2-B-1 clause 3.3 requires. The code is IANA-assigned; 19 is the
	// value in the IANA Bundle Block Types registry.
	BlockTypeECOS BlockType = 19
)

// String names the block type.
func (b BlockType) String() string {
	switch b {
	case BlockTypePayload:
		return "payload"
	case BlockTypeECOS:
		return "extended class of service"
	default:
		if b >= 192 {
			return fmt.Sprintf("private(%d)", uint8(b))
		}
		return fmt.Sprintf("reserved(%d)", uint8(b))
	}
}

// BlockFlags are the block processing control flags of RFC 5050 clause 4.5.2.
type BlockFlags uint64

const (
	// BlockReplicate asks that this block be copied into every fragment (bit 0).
	BlockReplicate BlockFlags = 1 << 0
	// BlockReportIfUnprocessed asks for a status report when the block cannot
	// be processed (bit 1).
	BlockReportIfUnprocessed BlockFlags = 1 << 1
	// BlockDeleteIfUnprocessed deletes the bundle when the block cannot be
	// processed (bit 2).
	BlockDeleteIfUnprocessed BlockFlags = 1 << 2
	// BlockLast marks the final block of the bundle (bit 3).
	BlockLast BlockFlags = 1 << 3
	// BlockDiscardIfUnprocessed drops the block when it cannot be processed
	// (bit 4).
	BlockDiscardIfUnprocessed BlockFlags = 1 << 4
	// BlockForwarded records that the block passed through a node that could
	// not process it (bit 5).
	BlockForwarded BlockFlags = 1 << 5
	// BlockHasEIDRefs marks a block carrying EID references (bit 6).
	BlockHasEIDRefs BlockFlags = 1 << 6
)

// Has reports whether every flag in want is set.
func (f BlockFlags) Has(want BlockFlags) bool { return f&want == want }

// EIDReference is a pair of dictionary offsets naming an endpoint from within
// a canonical block, per clause 4.5.2.
type EIDReference struct {
	SchemeOffset uint64
	SSPOffset    uint64
}

// DefaultMaxBlockLength bounds a decoded block body when DecodeOptions leaves
// MaxBlockLength at zero: 16 MiB.
//
// RFC 5050 sets no ceiling and a block length is an SDNV reaching 2^64, so
// without a cap one corrupt bundle would size an allocation from a bogus
// length.
const DefaultMaxBlockLength = 16 << 20

// CanonicalBlock is any block other than the primary one, per clause 4.5.2.
type CanonicalBlock struct {
	Type  BlockType
	Flags BlockFlags

	// EIDReferences are present only when BlockHasEIDRefs is set. They point
	// into the primary block's dictionary.
	EIDReferences []EIDReference

	// Data is the block-type-specific body.
	Data []byte
}

// IsLast reports whether this block carries the last-block flag.
func (b *CanonicalBlock) IsLast() bool { return b.Flags.Has(BlockLast) }

// Validate checks the block against clause 4.5.2.
func (b *CanonicalBlock) Validate() error {
	hasRefs := b.Flags.Has(BlockHasEIDRefs)
	if hasRefs != (len(b.EIDReferences) > 0) {
		// Clause 4.5.2 ties the flag and the field together: the field is present
		// "if and only if" the flag is set.
		return ErrInvalidEndpointID
	}
	return nil
}

// Encode serializes the canonical block.
func (b *CanonicalBlock) Encode() ([]byte, error) {
	if err := b.Validate(); err != nil {
		return nil, err
	}

	out := []byte{byte(b.Type)}
	out = sdnv.AppendEncode(out, uint64(b.Flags))

	if b.Flags.Has(BlockHasEIDRefs) {
		out = sdnv.AppendEncode(out, uint64(len(b.EIDReferences)))
		for _, ref := range b.EIDReferences {
			out = sdnv.AppendEncode(out, ref.SchemeOffset)
			out = sdnv.AppendEncode(out, ref.SSPOffset)
		}
	}

	out = sdnv.AppendEncode(out, uint64(len(b.Data)))
	return append(out, b.Data...), nil
}

// DecodeCanonicalBlock parses a canonical block from the front of data,
// returning the block and the octets consumed.
//
// maxBlockLength caps the body; pass zero for DefaultMaxBlockLength.
func DecodeCanonicalBlock(data []byte, maxBlockLength uint64) (*CanonicalBlock, int, error) {
	if maxBlockLength == 0 {
		maxBlockLength = DefaultMaxBlockLength
	}
	if len(data) < 1 {
		return nil, 0, ErrDataTooShort
	}

	b := &CanonicalBlock{Type: BlockType(data[0])}
	offset := 1

	flags, n, err := sdnv.Decode(data[offset:])
	if err != nil {
		return nil, 0, ErrDataTooShort
	}
	b.Flags = BlockFlags(flags)
	offset += n

	if b.Flags.Has(BlockHasEIDRefs) {
		count, n, err := sdnv.Decode(data[offset:])
		if err != nil {
			return nil, 0, ErrDataTooShort
		}
		offset += n

		// A reference is two SDNVs, so at least two octets. Refuse a count
		// the remaining bytes cannot hold before allocating for it.
		if count > uint64(len(data)-offset)/2 {
			return nil, 0, ErrDataTooShort
		}
		for i := uint64(0); i < count; i++ {
			pair, n, err := sdnv.DecodeN(data[offset:], 2)
			if err != nil {
				return nil, 0, ErrDataTooShort
			}
			b.EIDReferences = append(b.EIDReferences, EIDReference{
				SchemeOffset: pair[0], SSPOffset: pair[1],
			})
			offset += n
		}
	}

	length, n, err := sdnv.Decode(data[offset:])
	if err != nil {
		return nil, 0, ErrDataTooShort
	}
	offset += n

	if length > maxBlockLength {
		return nil, 0, ErrBlockTooLarge
	}
	if uint64(len(data)-offset) < length {
		return nil, 0, ErrDataTooShort
	}
	if length > 0 {
		b.Data = make([]byte, length)
		copy(b.Data, data[offset:offset+int(length)])
	}
	return b, offset + int(length), nil
}

// Humanize returns a human-readable summary.
func (b *CanonicalBlock) Humanize() string {
	return fmt.Sprintf("Bundle Canonical Block\n  Type ....... %s\n  Last ....... %t\n  Length ..... %d octets",
		b.Type, b.IsLast(), len(b.Data))
}

// --- Extended Class of Service, CCSDS 734.2-B-1 annex C ---

// ECOSFlags are the flags byte of an Extended Class of Service block, per
// annex C, item C2 f).
type ECOSFlags uint8

const (
	// ECOSCritical asks that one copy go along every path that might reach
	// the destination (0x01).
	ECOSCritical ECOSFlags = 0x01
	// ECOSStreaming asks for best-efforts forwarding, without retransmission
	// (0x02).
	ECOSStreaming ECOSFlags = 0x02
	// ECOSFlowLabelPresent says a flow label SDNV follows the ordinal byte
	// (0x04).
	ECOSFlowLabelPresent ECOSFlags = 0x04
	// ECOSReliable asks for a convergence layer that retransmits on loss
	// (0x08).
	ECOSReliable ECOSFlags = 0x08
)

// ECOSCustodySignalOrdinal is the ordinal value annex C reserves for custody
// signals (C3.1.4).
const ECOSCustodySignalOrdinal uint8 = 255

// ECOS is the Extended Class of Service block CCSDS 734.2-B-1 clause 3.3 requires
// conformant implementations to support.
//
// RFC 5050 gives a bundle three priority levels. Space operations need more:
// a finer ordinal ranking within the expedited class, a way to mark emergency
// traffic that should go by every route at once, and a way to ask for or
// refuse convergence-layer retransmission.
type ECOS struct {
	Flags ECOSFlags

	// Ordinal ranks this bundle among other expedited ones: 100 is more
	// urgent than 99. It has no significance unless the bundle's class of
	// service is expedited. Value 255 is reserved for custody signals.
	Ordinal uint8

	// FlowLabel is an opaque value for the convergence layer, present only
	// when ECOSFlowLabelPresent is set.
	FlowLabel uint64
}

// Validate checks the block against annex C, C3.1.
func (e *ECOS) Validate() error {
	// C3.1.3 ties the flag and the field together.
	if e.Flags&ECOSFlowLabelPresent == 0 && e.FlowLabel != 0 {
		return ErrInvalidECOS
	}
	return nil
}

// Encode serializes the ECOS block data: a flags byte, an ordinal byte, and
// optionally a flow label SDNV (C2 d to h).
func (e *ECOS) Encode() ([]byte, error) {
	if err := e.Validate(); err != nil {
		return nil, err
	}
	out := []byte{byte(e.Flags), e.Ordinal}
	if e.Flags&ECOSFlowLabelPresent != 0 {
		out = sdnv.AppendEncode(out, e.FlowLabel)
	}
	return out, nil
}

// DecodeECOS parses ECOS block data.
func DecodeECOS(data []byte) (*ECOS, error) {
	// C2 d): the block data length is 2 + N.
	if len(data) < 2 {
		return nil, ErrDataTooShort
	}
	e := &ECOS{Flags: ECOSFlags(data[0]), Ordinal: data[1]}

	if e.Flags&ECOSFlowLabelPresent != 0 {
		label, _, err := sdnv.Decode(data[2:])
		if err != nil {
			return nil, ErrDataTooShort
		}
		e.FlowLabel = label
	}
	return e, nil
}

// Block wraps the ECOS data in a canonical block.
//
// Annex C requires bit 0 of the block processing flags (replicate in every
// fragment) and forbids EID references (C2 b and c).
func (e *ECOS) Block() (*CanonicalBlock, error) {
	data, err := e.Encode()
	if err != nil {
		return nil, err
	}
	return &CanonicalBlock{
		Type:  BlockTypeECOS,
		Flags: BlockReplicate,
		Data:  data,
	}, nil
}

// Humanize returns a human-readable summary.
func (e *ECOS) Humanize() string {
	out := fmt.Sprintf("Extended Class of Service\n  Ordinal .... %d", e.Ordinal)
	if e.Flags&ECOSCritical != 0 {
		out += "\n  Critical ... every available route"
	}
	if e.Flags&ECOSStreaming != 0 {
		out += "\n  Streaming .. best efforts, no retransmission"
	}
	if e.Flags&ECOSReliable != 0 {
		out += "\n  Reliable ... retransmitting convergence layer"
	}
	if e.Flags&ECOSFlowLabelPresent != 0 {
		out += fmt.Sprintf("\n  Flow label . %d", e.FlowLabel)
	}
	return out
}
