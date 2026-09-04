package bpsec

import (
	"crypto/hmac"
	"crypto/sha256"
	"crypto/sha512"
	"hash"

	"github.com/ravisuhag/astro/internal/cbor"
	"github.com/ravisuhag/astro/internal/keywrap"
	"github.com/ravisuhag/astro/pkg/bp"
)

// SHAVariant selects which SHA-2 size the keyed hash uses
// (RFC 9173 clause 3.3.1). The values are the HMAC algorithm numbers of
// RFC 8152 table 7, not the digest sizes, which is why they run 5, 6, 7 rather
// than 256, 384, 512.
type SHAVariant uint64

const (
	// HMACSHA256 is HMAC 256/256: a 32-octet keyed hash.
	HMACSHA256 SHAVariant = 5
	// HMACSHA384 is HMAC 384/384: a 48-octet keyed hash.
	HMACSHA384 SHAVariant = 6
	// HMACSHA512 is HMAC 512/512: a 64-octet keyed hash.
	HMACSHA512 SHAVariant = 7
)

// DefaultSHAVariant is what RFC 9173 clause 3.3.1 says to assume when a BIB
// carries no SHA variant parameter.
const DefaultSHAVariant = HMACSHA384

// Security context parameter identifiers for BIB-HMAC-SHA2
// (RFC 9173 clause 3.3.4).
//
// These numbers are not shared with BCB-AES-GCM. The wrapped key is parameter
// 2 here and parameter 3 there, so a reader that keys off the number without
// first checking the security context identifier will pull the wrong field.
const (
	ParamSHAVariant     = 1
	ParamBIBWrappedKey  = 2
	ParamIntegrityScope = 3
)

// ResultExpectedHMAC is the only security result BIB-HMAC-SHA2 defines: the
// keyed hash computed at the security source (RFC 9173 clause 3.4).
const ResultExpectedHMAC = 1

// Size is the length in octets of the keyed hash this variant produces. The
// output "MUST be equal to the size of the SHA2 hashing function"
// (RFC 9173 clause 3.1).
func (v SHAVariant) Size() int {
	switch v {
	case HMACSHA256:
		return sha256.Size
	case HMACSHA384:
		return sha512.Size384
	case HMACSHA512:
		return sha512.Size
	}
	return 0
}

// Valid reports whether this is one of the three variants clause 3.3.1 defines.
func (v SHAVariant) Valid() bool { return v.Size() != 0 }

// String names the variant the way RFC 9173 table 1 does.
func (v SHAVariant) String() string {
	switch v {
	case HMACSHA256:
		return "HMAC 256/256"
	case HMACSHA384:
		return "HMAC 384/384"
	case HMACSHA512:
		return "HMAC 512/512"
	}
	return "unknown SHA variant"
}

// new returns the hash constructor for this variant.
func (v SHAVariant) new() func() hash.Hash {
	switch v {
	case HMACSHA256:
		return sha256.New
	case HMACSHA384:
		return sha512.New384
	case HMACSHA512:
		return sha512.New
	}
	return nil
}

// Keys says where a verifier or a decrypter should get its symmetric key.
//
// A security block may carry its key wrapped under a key encryption key
// (RFC 9173 clauses 3.3.2 and 4.3.3). When it does and KEK is set, the wrapped
// key wins, because it is the key the security source actually used. When the
// block carries no wrapped key, Key is used. Where either comes from is a
// question RFC 9172 clause 6 leaves to the deployment.
type Keys struct {
	// Key is the symmetric key to use when the security block carries no
	// wrapped key.
	Key []byte
	// KEK unwraps the wrapped key parameter when the block carries one.
	KEK []byte
}

// Integrity adds a Block Integrity Block using the BIB-HMAC-SHA2 security
// context of RFC 9173 clause 3.
type Integrity struct {
	// BlockNumber is the block number the BIB will carry. It must be free in
	// the bundle and at least 2.
	BlockNumber uint64
	// Flags are the BIB's own block processing control flags. RFC 9172
	// clause 3.7 notes that setting one that discards the block or deletes the
	// bundle deserves some thought, and leaves the choice to policy.
	Flags bp.BlockControlFlags
	// CRCType is the checksum on the BIB itself. The default, no checksum, is
	// what the worked examples in RFC 9173 appendix A use.
	CRCType bp.CRCType
	// Source names the node adding the block (RFC 9172 clause 3.6).
	Source bp.EID
	// Variant selects the SHA-2 size. The zero value is not valid; use
	// DefaultSHAVariant for what RFC 9173 says to assume.
	Variant SHAVariant
	// Scope says how much beyond the target's contents the hash binds.
	Scope ScopeFlags
	// Key is the symmetric HMAC key.
	Key []byte
	// KEK, when set, wraps Key into the block as the wrapped key parameter so
	// that a receiver holding the same key encryption key can recover it.
	KEK []byte
}

// Add builds a BIB over the named targets and inserts it into the bundle.
//
// It mutates the bundle. The BIB goes in immediately before the payload block,
// which RFC 9171 clause 4.1 requires to stay last. Any checksum on a target is
// removed first, as RFC 9173 clause 3.8.1 requires: a target covered by a BIB
// no longer needs the weaker unkeyed check, and RFC 9171 clause 4.3.2 stops
// requiring one. That removal does not change the keyed hash — a block's CRC
// is not part of the IPPT — so a verifier is unaffected either way.
//
// Targets are named by block number. Block number 0 is the primary block.
func (i Integrity) Add(b *bp.Bundle, targets ...uint64) (*bp.CanonicalBlock, error) {
	if b == nil || b.Primary == nil {
		return nil, bp.ErrNoPrimaryBlock
	}
	if !i.Variant.Valid() {
		return nil, ErrUnknownSHAVariant
	}
	if err := i.Scope.Validate(); err != nil {
		return nil, err
	}
	if len(i.Key) == 0 {
		return nil, ErrNoKey
	}
	if err := checkNewBlockNumber(b, i.BlockNumber); err != nil {
		return nil, err
	}
	if err := checkIntegrityTargets(b, targets); err != nil {
		return nil, err
	}

	bib := &bp.CanonicalBlock{
		Type:    BlockTypeIntegrity,
		Number:  i.BlockNumber,
		Flags:   i.Flags,
		CRCType: i.CRCType,
	}

	asb := &ASB{
		Targets:      targets,
		ContextID:    ContextBIBHMACSHA2,
		ContextFlags: ContextFlagParametersPresent,
		Source:       i.Source,
	}

	// Parameters go out in ascending identifier order, which is the order
	// every worked example in RFC 9173 appendix A prints them in.
	asb.Parameters = append(asb.Parameters, Parameter{
		ID:    ParamSHAVariant,
		Value: cbor.AppendUint(nil, uint64(i.Variant)),
	})
	if len(i.KEK) > 0 {
		wrapped, err := keywrap.Wrap(i.KEK, i.Key)
		if err != nil {
			return nil, err
		}
		asb.Parameters = append(asb.Parameters, Parameter{
			ID:    ParamBIBWrappedKey,
			Value: cbor.AppendByteString(nil, wrapped),
		})
	}
	asb.Parameters = append(asb.Parameters, Parameter{
		ID:    ParamIntegrityScope,
		Value: cbor.AppendUint(nil, uint64(i.Scope)),
	})

	for _, target := range targets {
		clearTargetCRC(b, target)

		mac, err := keyedHash(b, i.Scope, target, bib, i.Variant, i.Key)
		if err != nil {
			return nil, err
		}
		asb.Results = append(asb.Results, []Result{{
			ID:    ResultExpectedHMAC,
			Value: cbor.AppendByteString(nil, mac),
		}})
	}

	data, err := asb.Encode()
	if err != nil {
		return nil, err
	}
	bib.Data = data

	insertSecurityBlock(b, bib)
	return bib, nil
}

// IntegrityParameters is the BIB-HMAC-SHA2 parameter set as a verifier sees it
// (RFC 9173 clause 3.3), with the RFC's defaults already applied to whatever
// the block left out.
type IntegrityParameters struct {
	// Variant is the SHA-2 size, defaulting to DefaultSHAVariant.
	Variant SHAVariant
	// WrappedKey is the AES-wrapped HMAC key, or nil when the block carries
	// none.
	WrappedKey []byte
	// Scope defaults to ScopeAll.
	Scope ScopeFlags
}

// DecodeIntegrityParameters reads the parameters of a BIB-HMAC-SHA2 block.
//
// An absent parameter takes the default RFC 9173 clause 3.3 states, so the
// result always describes a complete configuration. Both defaults are
// overridable by local policy in the RFC; this package does not know a
// deployment's policy, so it applies the RFC's values and leaves a caller that
// needs otherwise to read the raw parameters through ASB.Parameter.
func DecodeIntegrityParameters(a *ASB) (IntegrityParameters, error) {
	p := IntegrityParameters{Variant: DefaultSHAVariant, Scope: ScopeAll}

	if a.ContextID != ContextBIBHMACSHA2 {
		return p, ErrUnknownContext
	}
	if raw, ok := a.Parameter(ParamSHAVariant); ok {
		v, err := decodeUint(raw)
		if err != nil {
			return p, err
		}
		p.Variant = SHAVariant(v)
		if !p.Variant.Valid() {
			return p, ErrUnknownSHAVariant
		}
	}
	if raw, ok := a.Parameter(ParamBIBWrappedKey); ok {
		b, err := decodeByteString(raw)
		if err != nil {
			return p, err
		}
		p.WrappedKey = b
	}
	if raw, ok := a.Parameter(ParamIntegrityScope); ok {
		v, err := decodeUint(raw)
		if err != nil {
			return p, err
		}
		// Clause 3.3.3 caps the field at 16 bits, so a larger value is not a
		// scope this context can express.
		if v > 0xFFFF {
			return p, ErrReservedScopeFlag
		}
		p.Scope = ScopeFlags(v)
		if err := p.Scope.Validate(); err != nil {
			return p, err
		}
	}
	return p, nil
}

// Verify checks every security operation in a BIB against the bundle it sits
// in (RFC 9173 clause 3.8.2).
//
// It reports the first target whose keyed hash does not match. A bundle that
// passes has not been altered in any way the BIB's scope covers; what to do
// with one that fails is a policy decision RFC 9172 clause 7 leaves to the
// deployment, so nothing is removed or repaired here.
//
// Verify does not remove the BIB. A security acceptor that has finished with a
// security operation is required to strip it, but only the caller knows
// whether this node is the acceptor.
func Verify(b *bp.Bundle, bib *bp.CanonicalBlock, keys Keys) error {
	if b == nil || b.Primary == nil {
		return bp.ErrNoPrimaryBlock
	}
	if bib == nil || bib.Type != BlockTypeIntegrity {
		return ErrWrongSecurityBlockType
	}

	asb, err := DecodeASB(bib.Data)
	if err != nil {
		return err
	}
	params, err := DecodeIntegrityParameters(asb)
	if err != nil {
		return err
	}
	key, err := keys.resolve(params.WrappedKey)
	if err != nil {
		return err
	}

	for i, target := range asb.Targets {
		// Clause 3.9: a BIB result must not be checked when the target has
		// since been encrypted, because the octets in the block are no longer
		// the ones that were hashed.
		if encryptedBy(b, target) != nil {
			return ErrIntegrityAfterConfidentiality
		}

		raw, ok := asb.Result(i, ResultExpectedHMAC)
		if !ok {
			return ErrMissingHMAC
		}
		expected, err := decodeByteString(raw)
		if err != nil {
			return err
		}
		if len(expected) != params.Variant.Size() {
			return ErrHMACLength
		}

		got, err := keyedHash(b, params.Scope, target, bib, params.Variant, key)
		if err != nil {
			return err
		}
		// Clause 3.6 asks for a constant-time comparison, so that a caller
		// cannot learn a valid tag by timing how far the check got.
		if !hmac.Equal(got, expected) {
			return ErrIntegrityCheckFailed
		}
	}
	return nil
}

// keyedHash builds the IPPT for one target and runs the HMAC over it.
//
// The key length is deliberately not checked. RFC 9173 clause 3.5 says an HMAC
// key "MUST have a key length equal to the output of the HMAC", and then
// appendix A.1 signs with HMAC 512/512 under a 16-octet key. Enforcing the
// clause would reject the document's own worked example, and every
// implementation that pinned itself to it. HMAC accepts any key length by
// construction (RFC 2104), so a short key works and is simply weaker than the
// clause intends.
func keyedHash(b *bp.Bundle, scope ScopeFlags, target uint64, security *bp.CanonicalBlock, variant SHAVariant, key []byte) ([]byte, error) {
	ippt, err := IPPT(b, scope, target, security)
	if err != nil {
		return nil, err
	}
	mac := hmac.New(variant.new(), key)
	mac.Write(ippt)
	return mac.Sum(nil), nil
}

// resolve picks the key for a security operation: the wrapped one if the block
// carried it and a key encryption key is available, otherwise the caller's.
func (k Keys) resolve(wrapped []byte) ([]byte, error) {
	if len(wrapped) > 0 {
		if len(k.KEK) == 0 {
			return nil, ErrNoKey
		}
		return keywrap.Unwrap(k.KEK, wrapped)
	}
	if len(k.Key) == 0 {
		return nil, ErrNoKey
	}
	return k.Key, nil
}

// decodeUint reads a raw CBOR unsigned integer parameter or result value.
func decodeUint(raw []byte) (uint64, error) {
	d := cbor.NewDecoder(raw)
	v, err := d.Uint()
	if err != nil {
		return 0, err
	}
	if !d.AtEnd() {
		return 0, ErrTrailingBytes
	}
	return v, nil
}

// decodeByteString reads a raw CBOR byte string parameter or result value.
func decodeByteString(raw []byte) ([]byte, error) {
	d := cbor.NewDecoder(raw)
	v, err := d.ByteString()
	if err != nil {
		return nil, err
	}
	if !d.AtEnd() {
		return nil, ErrTrailingBytes
	}
	return v, nil
}
