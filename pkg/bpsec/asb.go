package bpsec

import (
	"github.com/ravisuhag/astro/internal/cbor"
	"github.com/ravisuhag/astro/pkg/bp"
)

// Block type codes for the two security blocks (RFC 9172 clause 11.1). Both
// are registered for version 7 only.
const (
	// BlockTypeIntegrity is the Block Integrity Block, which carries a keyed
	// hash over its targets.
	BlockTypeIntegrity bp.BlockType = 11
	// BlockTypeConfidentiality is the Block Confidentiality Block, which
	// replaces the contents of its targets with ciphertext.
	BlockTypeConfidentiality bp.BlockType = 12
)

// ContextID names the security context a security block uses
// (RFC 9172 clause 11.3).
type ContextID uint64

const (
	// ContextBIBHMACSHA2 is the default integrity context of RFC 9173
	// clause 3: a keyed hash built from HMAC over SHA-2.
	ContextBIBHMACSHA2 ContextID = 1
	// ContextBCBAESGCM is the default confidentiality context of RFC 9173
	// clause 4: AES in Galois/Counter Mode.
	ContextBCBAESGCM ContextID = 2
)

// String names the context for a person reading a dump.
func (c ContextID) String() string {
	switch c {
	case ContextBIBHMACSHA2:
		return "BIB-HMAC-SHA2"
	case ContextBCBAESGCM:
		return "BCB-AES-GCM"
	}
	return "unknown security context"
}

// ContextFlags says which optional fields a security block carries
// (RFC 9172 clause 3.6). Bit numbering runs from the low-order bit.
type ContextFlags uint64

// ContextFlagParametersPresent is bit 0. Clause 3.6 reserves every other bit,
// requires a writer to leave them zero, and requires a reader to ignore them.
const ContextFlagParametersPresent ContextFlags = 1 << 0

// Has reports whether every flag in mask is set.
func (f ContextFlags) Has(mask ContextFlags) bool { return f&mask == mask }

// Parameter is one security context parameter: an identifier and a value
// (RFC 9172 clause 3.6).
//
// Value holds the raw CBOR item, not a decoded Go value. RFC 9172 clause 3.10
// leaves the CBOR encoding of every parameter to the security context that
// defines it, so an Abstract Security Block cannot know what shape a given
// identifier takes. The typed views — IntegrityParameters and
// ConfidentialityParameters — read these for the two contexts RFC 9173
// defines. A block naming some other context still decodes, with its
// parameters intact and uninterpreted.
type Parameter struct {
	ID    uint64
	Value []byte
}

// Result is one security result: an identifier and a value
// (RFC 9172 clause 3.6). Value holds the raw CBOR item, for the same reason
// Parameter.Value does.
type Result struct {
	ID    uint64
	Value []byte
}

// ASB is the Abstract Security Block of RFC 9172 clause 3.6, the structure a
// BIB and a BCB share.
//
// It is never a block on its own. It is what sits inside the
// block-type-specific data field of a security block, so it is a bare CBOR
// sequence rather than an array: five or six items one after another, with no
// enclosing head.
type ASB struct {
	// Targets holds the block number of each block this security block covers,
	// in the order its results are given.
	Targets []uint64
	// ContextID names the security context.
	ContextID ContextID
	// ContextFlags says whether Parameters is present.
	ContextFlags ContextFlags
	// Source names the node that added this security block.
	Source bp.EID
	// Parameters configures the security context. It is present only when
	// ContextFlags sets ContextFlagParametersPresent.
	Parameters []Parameter
	// Results holds one set of security results per entry in Targets, in the
	// same order.
	Results [][]Result
}

// Validate checks the Abstract Security Block against the rules of
// RFC 9172 clause 3.6 that do not need the rest of the bundle. Rules about
// what a target may be live with the block that has the target — see
// Integrity.Add and Confidentiality.Add.
func (a *ASB) Validate() error {
	if len(a.Targets) == 0 {
		return ErrNoTargets
	}
	seen := make(map[uint64]struct{}, len(a.Targets))
	for _, t := range a.Targets {
		if _, dup := seen[t]; dup {
			return ErrDuplicateTarget
		}
		seen[t] = struct{}{}
	}
	if len(a.Results) != len(a.Targets) {
		return ErrResultCountMismatch
	}
	if a.ContextFlags&^ContextFlagParametersPresent != 0 {
		return ErrReservedContextFlag
	}
	if a.ContextFlags.Has(ContextFlagParametersPresent) != (len(a.Parameters) > 0) {
		return ErrParametersFlagDisagrees
	}
	return a.Source.Validate()
}

// Encode writes the Abstract Security Block as the CBOR sequence of
// RFC 9172 clause 3.6.
func (a *ASB) Encode() ([]byte, error) {
	if err := a.Validate(); err != nil {
		return nil, err
	}

	dst := cbor.AppendArrayHeader(nil, uint64(len(a.Targets)))
	for _, t := range a.Targets {
		dst = cbor.AppendUint(dst, t)
	}

	dst = cbor.AppendUint(dst, uint64(a.ContextID))
	dst = cbor.AppendUint(dst, uint64(a.ContextFlags))

	source, err := a.Source.Encode()
	if err != nil {
		return nil, err
	}
	dst = append(dst, source...)

	if a.ContextFlags.Has(ContextFlagParametersPresent) {
		dst = cbor.AppendArrayHeader(dst, uint64(len(a.Parameters)))
		for _, p := range a.Parameters {
			dst = cbor.AppendArrayHeader(dst, 2)
			dst = cbor.AppendUint(dst, p.ID)
			dst = append(dst, p.Value...)
		}
	}

	dst = cbor.AppendArrayHeader(dst, uint64(len(a.Results)))
	for _, set := range a.Results {
		dst = cbor.AppendArrayHeader(dst, uint64(len(set)))
		for _, r := range set {
			dst = cbor.AppendArrayHeader(dst, 2)
			dst = cbor.AppendUint(dst, r.ID)
			dst = append(dst, r.Value...)
		}
	}
	return dst, nil
}

// DecodeASB reads an Abstract Security Block from the block-type-specific data
// field of a security block. It rejects any octet left over.
func DecodeASB(data []byte) (*ASB, error) {
	d := cbor.NewDecoder(data)

	a, err := decodeASB(d)
	if err != nil {
		return nil, err
	}
	if !d.AtEnd() {
		return nil, ErrTrailingBytes
	}
	return a, nil
}

func decodeASB(d *cbor.Decoder) (*ASB, error) {
	a := &ASB{}

	n, indefinite, err := d.ArrayHeader()
	if err != nil {
		return nil, err
	}
	if indefinite {
		return nil, ErrMalformedASB
	}
	// A target is one CBOR unsigned integer, so it cannot be shorter than an
	// octet. Checking the claimed count against what is left stops a huge
	// length from driving a large allocation before the read fails.
	if n > uint64(d.Remaining()) {
		return nil, cbor.ErrTruncated
	}
	a.Targets = make([]uint64, 0, n)
	for i := uint64(0); i < n; i++ {
		t, err := d.Uint()
		if err != nil {
			return nil, err
		}
		a.Targets = append(a.Targets, t)
	}

	contextID, err := d.Uint()
	if err != nil {
		return nil, err
	}
	a.ContextID = ContextID(contextID)

	flags, err := d.Uint()
	if err != nil {
		return nil, err
	}
	a.ContextFlags = ContextFlags(flags)

	// The security source is an endpoint ID, whose scheme rules belong to
	// pkg/bp. Lift the whole item out and hand it over rather than writing
	// them a second time here.
	raw, err := d.Skip()
	if err != nil {
		return nil, err
	}
	if a.Source, err = bp.DecodeEID(raw); err != nil {
		return nil, err
	}

	if a.ContextFlags.Has(ContextFlagParametersPresent) {
		if a.Parameters, err = decodeParameters(d); err != nil {
			return nil, err
		}
	}

	sets, indefinite, err := d.ArrayHeader()
	if err != nil {
		return nil, err
	}
	if indefinite {
		return nil, ErrMalformedASB
	}
	if sets > uint64(d.Remaining()) {
		return nil, cbor.ErrTruncated
	}
	a.Results = make([][]Result, 0, sets)
	for i := uint64(0); i < sets; i++ {
		set, err := decodeResults(d)
		if err != nil {
			return nil, err
		}
		a.Results = append(a.Results, set)
	}

	if err := a.Validate(); err != nil {
		return nil, err
	}
	return a, nil
}

// decodeParameters reads the security context parameters array: a list of
// two-item [id, value] arrays (RFC 9172 clause 3.6, figure 1).
func decodeParameters(d *cbor.Decoder) ([]Parameter, error) {
	ids, values, err := decodePairs(d)
	if err != nil {
		return nil, err
	}
	out := make([]Parameter, len(ids))
	for i := range ids {
		out[i] = Parameter{ID: ids[i], Value: values[i]}
	}
	return out, nil
}

// decodeResults reads one target's security results: a list of two-item
// [id, value] arrays (RFC 9172 clause 3.6, figure 2).
func decodeResults(d *cbor.Decoder) ([]Result, error) {
	ids, values, err := decodePairs(d)
	if err != nil {
		return nil, err
	}
	out := make([]Result, len(ids))
	for i := range ids {
		out[i] = Result{ID: ids[i], Value: values[i]}
	}
	return out, nil
}

// decodePairs reads an array of two-item [id, value] arrays. Parameters and
// results have the same shape, so they have the same reader.
func decodePairs(d *cbor.Decoder) (ids []uint64, values [][]byte, err error) {
	n, indefinite, err := d.ArrayHeader()
	if err != nil {
		return nil, nil, err
	}
	if indefinite {
		return nil, nil, ErrMalformedASB
	}
	// The shortest possible pair is a two-item array head, an identifier and a
	// value: three octets. Bounding the count by what is left stops a bogus
	// length from driving a large allocation before the read fails.
	if n > uint64(d.Remaining()) {
		return nil, nil, cbor.ErrTruncated
	}

	ids = make([]uint64, 0, n)
	values = make([][]byte, 0, n)
	for i := uint64(0); i < n; i++ {
		items, indefinite, err := d.ArrayHeader()
		if err != nil {
			return nil, nil, err
		}
		if indefinite || items != 2 {
			return nil, nil, ErrMalformedASB
		}
		id, err := d.Uint()
		if err != nil {
			return nil, nil, err
		}
		raw, err := d.Skip()
		if err != nil {
			return nil, nil, err
		}
		// Copy: Skip aliases the input, and a decoded block outlives it.
		value := make([]byte, len(raw))
		copy(value, raw)

		ids = append(ids, id)
		values = append(values, value)
	}
	return ids, values, nil
}

// Parameter returns the raw CBOR value of the parameter with the given
// identifier, and whether the block carried it.
func (a *ASB) Parameter(id uint64) ([]byte, bool) {
	for _, p := range a.Parameters {
		if p.ID == id {
			return p.Value, true
		}
	}
	return nil, false
}

// Result returns the raw CBOR value of the result with the given identifier
// for the target at index i, and whether the block carried it.
func (a *ASB) Result(i int, id uint64) ([]byte, bool) {
	if i < 0 || i >= len(a.Results) {
		return nil, false
	}
	for _, r := range a.Results[i] {
		if r.ID == id {
			return r.Value, true
		}
	}
	return nil, false
}

// TargetIndex returns the position of a block number in the target list, or -1
// if this security block does not cover it. Results are ordered to match, so
// the index is how a caller pairs a target with its results.
func (a *ASB) TargetIndex(blockNumber uint64) int {
	for i, t := range a.Targets {
		if t == blockNumber {
			return i
		}
	}
	return -1
}
