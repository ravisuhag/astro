package bpsec

import (
	"crypto/aes"
	"crypto/cipher"

	"github.com/ravisuhag/astro/internal/cbor"
	"github.com/ravisuhag/astro/internal/keywrap"
	"github.com/ravisuhag/astro/pkg/bp"
)

// AESVariant selects the AES key length (RFC 9173 clause 4.3.2). The values
// are the algorithm numbers of RFC 8152 table 9.
//
// There are two, not three. The number 2, which in RFC 8152 is A192GCM, is not
// assigned here: RFC 9173 table 4 lists only 1 and 3, so a 192-bit key has no
// way to be named in a BCB.
type AESVariant uint64

const (
	// AES128GCM is A128GCM: a 16-octet key.
	AES128GCM AESVariant = 1
	// AES256GCM is A256GCM: a 32-octet key.
	AES256GCM AESVariant = 3
)

// DefaultAESVariant is what RFC 9173 clause 4.3.2 says to assume when a BCB
// carries no AES variant parameter.
const DefaultAESVariant = AES256GCM

// Security context parameter identifiers for BCB-AES-GCM
// (RFC 9173 clause 4.3.5).
//
// They do not line up with the BIB-HMAC-SHA2 numbers. The wrapped key is 3
// here and 2 there; the scope flags are 4 here and 3 there. Read the security
// context identifier before reading any parameter.
const (
	ParamIV            = 1
	ParamAESVariant    = 2
	ParamBCBWrappedKey = 3
	ParamAADScope      = 4
)

// ResultAuthenticationTag is the only security result BCB-AES-GCM defines
// (RFC 9173 clause 4.4.2).
const ResultAuthenticationTag = 1

// TagSize is the length of the authentication tag, fixed at 128 bits for both
// variants (RFC 9173 clause 4.4.1).
const TagSize = 16

// RecommendedIVSize is the initialisation vector length RFC 9173 clause 4.3.1
// says to use unless local policy needs another. The clause allows 8 to 16.
const RecommendedIVSize = 12

// KeySize is the key length in octets this variant needs.
func (v AESVariant) KeySize() int {
	switch v {
	case AES128GCM:
		return 16
	case AES256GCM:
		return 32
	}
	return 0
}

// Valid reports whether this is one of the two variants clause 4.3.2 defines.
func (v AESVariant) Valid() bool { return v.KeySize() != 0 }

// String names the variant the way RFC 9173 table 4 does.
func (v AESVariant) String() string {
	switch v {
	case AES128GCM:
		return "A128GCM"
	case AES256GCM:
		return "A256GCM"
	}
	return "unknown AES variant"
}

// Confidentiality adds a Block Confidentiality Block using the BCB-AES-GCM
// security context of RFC 9173 clause 4.
type Confidentiality struct {
	// BlockNumber is the block number the BCB will carry.
	BlockNumber uint64
	// Flags are the BCB's own block processing control flags. RFC 9172
	// clause 3.8 fixes two of them: the discard flag must be clear, and a BCB
	// targeting the payload must set the replicate-in-every-fragment flag.
	Flags bp.BlockControlFlags
	// CRCType is the checksum on the BCB itself.
	CRCType bp.CRCType
	// Source names the node adding the block.
	Source bp.EID
	// Variant selects the AES key length. The zero value is not valid; use
	// DefaultAESVariant for what RFC 9173 says to assume.
	Variant AESVariant
	// Scope says how much beyond the target's contents is authenticated.
	Scope ScopeFlags
	// Key is the symmetric content encryption key. Its length must match
	// Variant.
	Key []byte
	// IV is the initialisation vector, 8 to 16 octets.
	//
	// The caller supplies it, and must never reuse one with the same key.
	// RFC 9173 clause 4.6 is explicit about the cost: a single repeat of an IV
	// and key pair loses the integrity protection, not merely the
	// confidentiality. A library that generated IVs from its own state would
	// take that decision away from the mission that has to live with it.
	IV []byte
	// KEK, when set, wraps Key into the block as the wrapped key parameter.
	KEK []byte
}

// Add encrypts the named targets and inserts a BCB describing the operation.
//
// It mutates the bundle, and it mutates the targets: each target's
// block-type-specific data is replaced by ciphertext of the same length, and
// any checksum on it is removed first (RFC 9173 clause 4.8.1). The
// authentication tag for each target goes into the BCB as a security result
// rather than being appended to the ciphertext, which is the choice
// clause 4.4 leaves open and the one every worked example in appendix A makes.
//
// Targets are named by block number, and their order fixes the order of the
// security results.
func (c Confidentiality) Add(b *bp.Bundle, targets ...uint64) (*bp.CanonicalBlock, error) {
	if b == nil || b.Primary == nil {
		return nil, bp.ErrNoPrimaryBlock
	}
	if !c.Variant.Valid() {
		return nil, ErrUnknownAESVariant
	}
	if len(c.Key) != c.Variant.KeySize() {
		return nil, ErrKeyLength
	}
	if err := checkIVLength(c.IV); err != nil {
		return nil, err
	}
	if err := c.Scope.Validate(); err != nil {
		return nil, err
	}
	if err := checkNewBlockNumber(b, c.BlockNumber); err != nil {
		return nil, err
	}
	if err := checkConfidentialityTargets(b, targets, c.Flags); err != nil {
		return nil, err
	}

	bcb := &bp.CanonicalBlock{
		Type:    BlockTypeConfidentiality,
		Number:  c.BlockNumber,
		Flags:   c.Flags,
		CRCType: c.CRCType,
	}

	asb := &ASB{
		Targets:      targets,
		ContextID:    ContextBCBAESGCM,
		ContextFlags: ContextFlagParametersPresent,
		Source:       c.Source,
	}
	asb.Parameters = append(asb.Parameters, Parameter{
		ID:    ParamIV,
		Value: cbor.AppendByteString(nil, c.IV),
	})
	asb.Parameters = append(asb.Parameters, Parameter{
		ID:    ParamAESVariant,
		Value: cbor.AppendUint(nil, uint64(c.Variant)),
	})
	if len(c.KEK) > 0 {
		wrapped, err := keywrap.Wrap(c.KEK, c.Key)
		if err != nil {
			return nil, err
		}
		asb.Parameters = append(asb.Parameters, Parameter{
			ID:    ParamBCBWrappedKey,
			Value: cbor.AppendByteString(nil, wrapped),
		})
	}
	asb.Parameters = append(asb.Parameters, Parameter{
		ID:    ParamAADScope,
		Value: cbor.AppendUint(nil, uint64(c.Scope)),
	})

	gcm, err := newGCM(c.Key, len(c.IV))
	if err != nil {
		return nil, err
	}

	// The AAD for every target has to be built before any target is encrypted.
	// A BCB may cover a BIB as well as the block that BIB protects, and once
	// the BIB is ciphertext its header is still readable but this ordering
	// keeps the two steps independent of each other.
	aads := make([][]byte, len(targets))
	for i, target := range targets {
		clearTargetCRC(b, target)

		aad, err := AAD(b, c.Scope, target, bcb)
		if err != nil {
			return nil, err
		}
		aads[i] = aad
	}

	for i, target := range targets {
		blk := b.Block(target)

		// Clause 4.7.1: the plaintext is the value of the block-type-specific
		// data field without the CBOR byte string head that frames it on the
		// wire. That is the opposite of the IPPT, which includes the head.
		sealed := gcm.Seal(nil, c.IV, blk.Data, aads[i])

		ciphertext := sealed[:len(blk.Data)]
		tag := sealed[len(blk.Data):]

		blk.Data = ciphertext
		asb.Results = append(asb.Results, []Result{{
			ID:    ResultAuthenticationTag,
			Value: cbor.AppendByteString(nil, tag),
		}})
	}

	data, err := asb.Encode()
	if err != nil {
		return nil, err
	}
	bcb.Data = data

	insertSecurityBlock(b, bcb)
	return bcb, nil
}

// ConfidentialityParameters is the BCB-AES-GCM parameter set as a decrypter
// sees it (RFC 9173 clause 4.3), with the RFC's defaults applied to whatever
// the block left out.
type ConfidentialityParameters struct {
	// IV is the initialisation vector. It has no default: clause 4.8.2 lets a
	// node treat a missing one as an error, and this package does.
	IV []byte
	// Variant is the AES key length, defaulting to DefaultAESVariant.
	Variant AESVariant
	// WrappedKey is the AES-wrapped content encryption key, or nil.
	WrappedKey []byte
	// Scope defaults to ScopeAll.
	Scope ScopeFlags
}

// DecodeConfidentialityParameters reads the parameters of a BCB-AES-GCM block.
func DecodeConfidentialityParameters(a *ASB) (ConfidentialityParameters, error) {
	p := ConfidentialityParameters{Variant: DefaultAESVariant, Scope: ScopeAll}

	if a.ContextID != ContextBCBAESGCM {
		return p, ErrUnknownContext
	}
	if raw, ok := a.Parameter(ParamIV); ok {
		iv, err := decodeByteString(raw)
		if err != nil {
			return p, err
		}
		if err := checkIVLength(iv); err != nil {
			return p, err
		}
		p.IV = iv
	}
	if raw, ok := a.Parameter(ParamAESVariant); ok {
		v, err := decodeUint(raw)
		if err != nil {
			return p, err
		}
		p.Variant = AESVariant(v)
		if !p.Variant.Valid() {
			return p, ErrUnknownAESVariant
		}
	}
	if raw, ok := a.Parameter(ParamBCBWrappedKey); ok {
		b, err := decodeByteString(raw)
		if err != nil {
			return p, err
		}
		p.WrappedKey = b
	}
	if raw, ok := a.Parameter(ParamAADScope); ok {
		v, err := decodeUint(raw)
		if err != nil {
			return p, err
		}
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

// Decrypt reverses a BCB: it authenticates and decrypts every target, puts the
// plaintext back, and removes the BCB from the bundle
// (RFC 9173 clause 4.8.2).
//
// The BCB is removed because RFC 9172 clause 5.1.1 requires it once every
// security operation in the block has been processed, and Decrypt processes
// all of them. A BCB left beside plaintext would tell the next node its
// targets are still encrypted.
//
// On failure the bundle is left untouched. Decryption is attempted against
// every target first and written back only if all of them authenticate, so a
// bundle that fails is still the bundle that arrived. RFC 9172 clause 5.1.1
// requires a node that cannot decrypt an encrypted payload to discard the
// bundle entirely, and that decision needs the original.
func Decrypt(b *bp.Bundle, bcb *bp.CanonicalBlock, keys Keys) error {
	if b == nil || b.Primary == nil {
		return bp.ErrNoPrimaryBlock
	}
	if bcb == nil || bcb.Type != BlockTypeConfidentiality {
		return ErrWrongSecurityBlockType
	}

	asb, err := DecodeASB(bcb.Data)
	if err != nil {
		return err
	}
	params, err := DecodeConfidentialityParameters(asb)
	if err != nil {
		return err
	}
	if len(params.IV) == 0 {
		return ErrMissingIV
	}
	key, err := keys.resolve(params.WrappedKey)
	if err != nil {
		return err
	}
	if len(key) != params.Variant.KeySize() {
		return ErrKeyLength
	}
	gcm, err := newGCM(key, len(params.IV))
	if err != nil {
		return err
	}

	plaintexts := make([][]byte, len(asb.Targets))
	for i, target := range asb.Targets {
		blk := b.Block(target)
		if blk == nil {
			return ErrTargetNotInBundle
		}

		raw, ok := asb.Result(i, ResultAuthenticationTag)
		if !ok {
			return ErrMissingTag
		}
		tag, err := decodeByteString(raw)
		if err != nil {
			return err
		}
		if len(tag) != TagSize {
			return ErrTagLength
		}

		aad, err := AAD(b, params.Scope, target, bcb)
		if err != nil {
			return err
		}

		sealed := make([]byte, 0, len(blk.Data)+len(tag))
		sealed = append(sealed, blk.Data...)
		sealed = append(sealed, tag...)

		plaintext, err := gcm.Open(nil, params.IV, sealed, aad)
		if err != nil {
			// AEAD cannot say whether the key, the IV, the ciphertext or the
			// additional authenticated data was wrong, so neither does this.
			return ErrDecryptionFailed
		}
		plaintexts[i] = plaintext
	}

	for i, target := range asb.Targets {
		b.Block(target).Data = plaintexts[i]
	}
	Remove(b, bcb)
	return nil
}

// checkIVLength enforces the 8 to 16 octet range of RFC 9173 clause 4.3.1.
func checkIVLength(iv []byte) error {
	if len(iv) < 8 || len(iv) > 16 {
		return ErrIVLength
	}
	return nil
}

// newGCM builds the AEAD for a key and IV length. The tag size is Go's
// default, 16 octets, which is the 128 bits clause 4.4.1 requires.
func newGCM(key []byte, ivLen int) (cipher.AEAD, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	return cipher.NewGCMWithNonceSize(block, ivLen)
}
