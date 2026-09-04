package bpsec_test

import (
	"errors"
	"testing"

	"github.com/ravisuhag/astro/internal/vectors"
	"github.com/ravisuhag/astro/pkg/bp"
	"github.com/ravisuhag/astro/pkg/bpsec"
)

// The wire vectors for this package live in vectors/bpsec/security.json.
//
// Almost all of them are published rather than derived. RFC 9173 appendix A
// prints four worked examples with their keys, their canonical forms and the
// blocks that come out, so the IPPT, the AAD, the BIB and the BCB are each
// checked against octets a different working group wrote.
//
// Which structure a vector's octets are comes from its config, not from the
// octets. An IPPT and an AAD are both bare CBOR sequences with no framing to
// tell them apart, and a BIB and a BCB differ only in a type code that is not
// part of the abstract security block at all.
//
// Not covered here: the block interaction rules of RFC 9172 clause 3.9, which
// are about what may be added to a bundle rather than about octets, and the
// verify and decrypt directions, which produce a verdict rather than a wire
// form. Both are covered by the package's own tests.
// errUnknownVectorStructure means a fixture named a structure this harness
// does not know how to build.
var errUnknownVectorStructure = errors.New("bpsec: vector names an unknown structure")

func TestBPSecVectors(t *testing.T) {
	vectors.RunFile(t, "bpsec/security.json", vectors.Impl{
		EncodeFn: encodeVector,
	})
}

func encodeVector(f, config vectors.Fields) ([]byte, error) {
	structure, err := config.Str("structure")
	if err != nil {
		return nil, err
	}

	bundle, err := bundleFromFields(f)
	if err != nil {
		return nil, err
	}
	scope, err := scopeFromFields(f)
	if err != nil {
		return nil, err
	}
	targets, err := targetsFromFields(f)
	if err != nil {
		return nil, err
	}

	switch structure {
	case "ippt", "aad":
		security, err := securityBlockFromFields(f)
		if err != nil {
			return nil, err
		}
		if structure == "ippt" {
			return bpsec.IPPT(bundle, scope, targets[0], security)
		}
		return bpsec.AAD(bundle, scope, targets[0], security)

	case "bib":
		block, err := integrityFromFields(f, scope)
		if err != nil {
			return nil, err
		}
		bib, err := block.Add(bundle, targets...)
		if err != nil {
			return nil, err
		}
		return bib.Encode()

	case "bcb":
		block, err := confidentialityFromFields(f, scope)
		if err != nil {
			return nil, err
		}
		bcb, err := block.Add(bundle, targets...)
		if err != nil {
			return nil, err
		}
		return bcb.Encode()
	}
	return nil, errUnknownVectorStructure
}

// bundleFromFields decodes the bundle a vector operates on. Every vector needs
// one: a security operation has no meaning outside the bundle it is bound to.
func bundleFromFields(f vectors.Fields) (*bp.Bundle, error) {
	raw, err := f.Hex("bundle")
	if err != nil {
		return nil, err
	}
	return bp.Decode(raw)
}

func scopeFromFields(f vectors.Fields) (bpsec.ScopeFlags, error) {
	scope, err := f.Uint("scope")
	if err != nil {
		return 0, err
	}
	return bpsec.ScopeFlags(scope), nil
}

// targetsFromFields reads the target block numbers. A vector names one target
// as "target" and an optional second as "target_2"; no published example needs
// a third, and the vector fields are flat by design.
func targetsFromFields(f vectors.Fields) ([]uint64, error) {
	first, err := f.Uint("target")
	if err != nil {
		return nil, err
	}
	targets := []uint64{first}

	if f.Has("target_2") {
		second, err := f.Uint("target_2")
		if err != nil {
			return nil, err
		}
		targets = append(targets, second)
	}
	return targets, nil
}

// securityBlockFromFields builds the header of the security block whose
// canonical form a scope flag pulls in. Only the type code, block number and
// processing control flags matter to the IPPT and the AAD.
func securityBlockFromFields(f vectors.Fields) (*bp.CanonicalBlock, error) {
	typeCode, err := f.Uint("security_type")
	if err != nil {
		return nil, err
	}
	number, err := f.Uint("security_number")
	if err != nil {
		return nil, err
	}
	flags, err := f.Uint("security_flags")
	if err != nil {
		return nil, err
	}
	return &bp.CanonicalBlock{
		Type:   bp.BlockType(typeCode),
		Number: number,
		Flags:  bp.BlockControlFlags(flags),
	}, nil
}

func integrityFromFields(f vectors.Fields, scope bpsec.ScopeFlags) (bpsec.Integrity, error) {
	number, flags, source, err := commonSecurityFields(f)
	if err != nil {
		return bpsec.Integrity{}, err
	}
	variant, err := f.Uint("variant")
	if err != nil {
		return bpsec.Integrity{}, err
	}
	key, err := f.Hex("key")
	if err != nil {
		return bpsec.Integrity{}, err
	}
	kek, err := f.HexOr("kek", nil)
	if err != nil {
		return bpsec.Integrity{}, err
	}

	return bpsec.Integrity{
		BlockNumber: number,
		Flags:       flags,
		Source:      source,
		Variant:     bpsec.SHAVariant(variant),
		Scope:       scope,
		Key:         key,
		KEK:         kek,
	}, nil
}

func confidentialityFromFields(f vectors.Fields, scope bpsec.ScopeFlags) (bpsec.Confidentiality, error) {
	number, flags, source, err := commonSecurityFields(f)
	if err != nil {
		return bpsec.Confidentiality{}, err
	}
	variant, err := f.Uint("variant")
	if err != nil {
		return bpsec.Confidentiality{}, err
	}
	key, err := f.Hex("key")
	if err != nil {
		return bpsec.Confidentiality{}, err
	}
	iv, err := f.Hex("iv")
	if err != nil {
		return bpsec.Confidentiality{}, err
	}
	kek, err := f.HexOr("kek", nil)
	if err != nil {
		return bpsec.Confidentiality{}, err
	}

	return bpsec.Confidentiality{
		BlockNumber: number,
		Flags:       flags,
		Source:      source,
		Variant:     bpsec.AESVariant(variant),
		Scope:       scope,
		Key:         key,
		IV:          iv,
		KEK:         kek,
	}, nil
}

// commonSecurityFields reads what a BIB and a BCB both carry.
func commonSecurityFields(f vectors.Fields) (number uint64, flags bp.BlockControlFlags, source bp.EID, err error) {
	if number, err = f.Uint("block_number"); err != nil {
		return 0, 0, bp.EID{}, err
	}
	raw, err := f.Uint("block_flags")
	if err != nil {
		return 0, 0, bp.EID{}, err
	}
	node, err := f.Uint("source_node")
	if err != nil {
		return 0, 0, bp.EID{}, err
	}
	service, err := f.Uint("source_service")
	if err != nil {
		return 0, 0, bp.EID{}, err
	}
	return number, bp.BlockControlFlags(raw), bp.IPN(node, service), nil
}
