package bp

// BlockType names what a block carries (RFC 9171 clause 9.1). The registry is
// shared with version 6, and the codes it assigned to version 6 only — 2, 3,
// 4, 5, 8 and 9 — mean nothing here.
type BlockType uint64

const (
	// BlockTypePayload carries the application data unit, or one contiguous
	// piece of it. Every bundle has exactly one (clause 4.1).
	BlockTypePayload BlockType = 1
	// BlockTypePreviousNode names the node that forwarded this bundle here
	// (clause 4.4.1).
	BlockTypePreviousNode BlockType = 6
	// BlockTypeBundleAge carries how long the bundle has existed, in
	// milliseconds, for nodes without a working clock (clause 4.4.2).
	BlockTypeBundleAge BlockType = 7
	// BlockTypeHopCount carries a hop limit and a hop count, to stop a bundle
	// looping forever (clause 4.4.3).
	BlockTypeHopCount BlockType = 10
)

// firstPrivateBlockType is where clause 9.1 stops assigning and lets a
// deployment use codes for whatever it likes. Codes at or above it are never
// treated as one of the types above.
const firstPrivateBlockType BlockType = 192

// BlockControlFlags say what to do with a block a node cannot process
// (RFC 9171 clause 4.2.4). Bit numbering runs from the low-order bit, as it
// does for the bundle flags.
type BlockControlFlags uint64

const (
	// BlockFlagReplicateInEveryFragment copies this block into every fragment.
	BlockFlagReplicateInEveryFragment BlockControlFlags = 1 << 0
	// BlockFlagReportIfUnprocessable asks for a status report if this block
	// cannot be processed.
	BlockFlagReportIfUnprocessable BlockControlFlags = 1 << 1
	// BlockFlagDeleteBundleIfUnprocessable deletes the whole bundle if this
	// block cannot be processed.
	BlockFlagDeleteBundleIfUnprocessable BlockControlFlags = 1 << 2
	// BlockFlagDiscardBlockIfUnprocessable drops just this block if it cannot
	// be processed, and forwards the rest.
	BlockFlagDiscardBlockIfUnprocessable BlockControlFlags = 1 << 4
)

// Has reports whether every flag in mask is set.
func (f BlockControlFlags) Has(mask BlockControlFlags) bool {
	return f&mask == mask
}

// PayloadBlockNumber is the block number the payload block always carries
// (RFC 9171 clause 4.1). The primary block's number is implicitly zero, so
// extension blocks start at 2.
const PayloadBlockNumber = 1

// CanonicalBlock is every block except the primary one
// (RFC 9171 clause 4.3.2).
//
// Data holds the block-type-specific field: the contents of the CBOR byte
// string, not the byte string itself. What is inside depends on the type. For
// a payload block it is the application data, raw. For the three extension
// blocks RFC 9171 defines it is itself CBOR — a second layer — which is why
// those types have their own constructors and accessors below rather than
// leaving callers to peel it.
//
// A block of a type this package does not know keeps its Data untouched and
// round-trips byte for byte. Clause 4.4 requires that: a node has to forward
// what it cannot parse, honouring the block's flags.
type CanonicalBlock struct {
	Type    BlockType
	Number  uint64
	Flags   BlockControlFlags
	CRCType CRCType
	Data    []byte
}

// NewPayloadBlock builds the payload block. Its number is fixed at 1.
func NewPayloadBlock(data []byte) *CanonicalBlock {
	return &CanonicalBlock{
		Type:   BlockTypePayload,
		Number: PayloadBlockNumber,
		Data:   data,
	}
}

// NewPreviousNodeBlock builds a Previous Node block naming the forwarder
// (RFC 9171 clause 4.4.1).
func NewPreviousNodeBlock(number uint64, node EID) (*CanonicalBlock, error) {
	if err := checkExtensionBlockNumber(number); err != nil {
		return nil, err
	}
	data, err := appendEID(nil, node)
	if err != nil {
		return nil, err
	}
	return &CanonicalBlock{Type: BlockTypePreviousNode, Number: number, Data: data}, nil
}

// NewBundleAgeBlock builds a Bundle Age block (RFC 9171 clause 4.4.2). A
// bundle whose creation time is unknown must carry exactly one of these.
func NewBundleAgeBlock(number uint64, ageMilliseconds uint64) (*CanonicalBlock, error) {
	if err := checkExtensionBlockNumber(number); err != nil {
		return nil, err
	}
	return &CanonicalBlock{
		Type:   BlockTypeBundleAge,
		Number: number,
		Data:   appendUint(nil, ageMilliseconds),
	}, nil
}

// NewHopCountBlock builds a Hop Count block (RFC 9171 clause 4.4.3). The limit
// must be 1 to 255; the count normally starts at zero.
func NewHopCountBlock(number uint64, limit, count uint64) (*CanonicalBlock, error) {
	if err := checkExtensionBlockNumber(number); err != nil {
		return nil, err
	}
	if limit < 1 || limit > 255 {
		return nil, ErrHopLimitOutOfRange
	}
	data := appendArrayHeader(nil, 2)
	data = appendUint(data, limit)
	data = appendUint(data, count)
	return &CanonicalBlock{Type: BlockTypeHopCount, Number: number, Data: data}, nil
}

// checkExtensionBlockNumber enforces the numbering of clause 4.1: zero belongs
// to the primary block and one to the payload, so an extension block starts
// at two.
func checkExtensionBlockNumber(number uint64) error {
	if number <= PayloadBlockNumber {
		return ErrReservedBlockNumber
	}
	return nil
}

// PreviousNode reads the node ID from a Previous Node block.
func (b *CanonicalBlock) PreviousNode() (EID, error) {
	if b.Type != BlockTypePreviousNode {
		return EID{}, ErrWrongBlockType
	}
	return newDecoder(b.Data).eid()
}

// BundleAge reads the age in milliseconds from a Bundle Age block.
func (b *CanonicalBlock) BundleAge() (uint64, error) {
	if b.Type != BlockTypeBundleAge {
		return 0, ErrWrongBlockType
	}
	return newDecoder(b.Data).uint()
}

// HopCount reads the limit and count from a Hop Count block.
func (b *CanonicalBlock) HopCount() (limit, count uint64, err error) {
	if b.Type != BlockTypeHopCount {
		return 0, 0, ErrWrongBlockType
	}
	d := newDecoder(b.Data)
	n, indefinite, err := d.arrayHeader()
	if err != nil {
		return 0, 0, err
	}
	if indefinite || n != 2 {
		return 0, 0, ErrMalformedBlockData
	}
	if limit, err = d.uint(); err != nil {
		return 0, 0, err
	}
	if count, err = d.uint(); err != nil {
		return 0, 0, err
	}
	if limit < 1 || limit > 255 {
		return 0, 0, ErrHopLimitOutOfRange
	}
	return limit, count, nil
}

// Validate checks what can be checked about a block on its own. Rules that
// need the whole bundle — one payload, one bundle age block — live in
// Bundle.Validate.
func (b *CanonicalBlock) Validate() error {
	if !b.CRCType.valid() {
		return ErrInvalidCRCType
	}
	if b.Type == BlockTypePayload && b.Number != PayloadBlockNumber {
		return ErrPayloadBlockNumber
	}
	if b.Type != BlockTypePayload && b.Number <= PayloadBlockNumber {
		return ErrReservedBlockNumber
	}
	if b.Type == 0 {
		// Clause 9.1 marks code 0 reserved, so nothing may claim it.
		return ErrReservedBlockType
	}
	return nil
}

// appendCanonicalBlock writes a canonical block, checksum included.
func appendCanonicalBlock(dst []byte, b *CanonicalBlock) ([]byte, error) {
	if err := b.Validate(); err != nil {
		return nil, err
	}

	items := uint64(5)
	if b.CRCType != CRCNone {
		items++
	}

	start := len(dst)
	dst = appendArrayHeader(dst, items)
	dst = appendUint(dst, uint64(b.Type))
	dst = appendUint(dst, b.Number)
	dst = appendUint(dst, uint64(b.Flags))
	dst = appendUint(dst, uint64(b.CRCType))
	dst = appendByteString(dst, b.Data)

	if b.CRCType != CRCNone {
		dst = appendZeroCRC(dst, b.CRCType)
		fillCRC(dst[start:], b.CRCType)
	}
	return dst, nil
}

// canonicalBlock reads a canonical block and verifies its checksum.
func (d *decoder) canonicalBlock() (*CanonicalBlock, error) {
	start := d.pos

	items, indefinite, err := d.arrayHeader()
	if err != nil {
		return nil, err
	}
	if indefinite || items < 5 || items > 6 {
		return nil, ErrMalformedCanonicalBlock
	}

	typeCode, err := d.uint()
	if err != nil {
		return nil, err
	}
	number, err := d.uint()
	if err != nil {
		return nil, err
	}
	flags, err := d.uint()
	if err != nil {
		return nil, err
	}
	crcCode, err := d.uint()
	if err != nil {
		return nil, err
	}
	crcType := CRCType(crcCode)
	if !crcType.valid() {
		return nil, ErrInvalidCRCType
	}

	// The array length has to agree with the CRC type, the same way the
	// primary block's does.
	wantItems := uint64(5)
	if crcType != CRCNone {
		wantItems++
	}
	if items != wantItems {
		return nil, ErrCanonicalBlockLengthMismatch
	}

	data, err := d.byteString()
	if err != nil {
		return nil, err
	}

	if crcType != CRCNone {
		got, err := d.byteString()
		if err != nil {
			return nil, err
		}
		if err := checkCRC(d.buf[start:d.pos], crcType, got); err != nil {
			return nil, err
		}
	}

	// Copy the data out of the input buffer: a decoded block outlives the
	// bytes it came from, and byteString aliases them.
	owned := make([]byte, len(data))
	copy(owned, data)

	b := &CanonicalBlock{
		Type:    BlockType(typeCode),
		Number:  number,
		Flags:   BlockControlFlags(flags),
		CRCType: crcType,
		Data:    owned,
	}
	if err := b.Validate(); err != nil {
		return nil, err
	}
	return b, nil
}
