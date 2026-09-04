package bpsec

import (
	"encoding/hex"
	"fmt"
	"strings"

	"github.com/ravisuhag/astro/pkg/bp"
)

// Humanize returns a human-readable summary of the security block.
//
// Parameters and results are named where the security context is one of the
// two RFC 9173 defines, and shown by number where it is not. A block from an
// unknown context still dumps: the Abstract Security Block structure is common
// to every context, so the shape is always readable even when the meaning is
// not.
func (a *ASB) Humanize() string {
	var sb strings.Builder

	fmt.Fprintf(&sb, "Security Context .. %s (%d)\n", a.ContextID, a.ContextID)
	fmt.Fprintf(&sb, "  Source .......... %s\n", a.Source)
	fmt.Fprintf(&sb, "  Targets ......... %s\n", humanizeTargets(a.Targets))

	if len(a.Parameters) > 0 {
		fmt.Fprintf(&sb, "  Parameters ...... %d", len(a.Parameters))
		for _, p := range a.Parameters {
			fmt.Fprintf(&sb, "\n    %s", a.humanizeParameter(p))
		}
		sb.WriteString("\n")
	}

	fmt.Fprintf(&sb, "  Results ......... %d set(s)", len(a.Results))
	for i, set := range a.Results {
		fmt.Fprintf(&sb, "\n    target %d:", a.Targets[i])
		for _, r := range set {
			fmt.Fprintf(&sb, " %s", a.humanizeResult(r))
		}
	}
	return sb.String()
}

// humanizeTargets lists the target block numbers, naming the primary and
// payload blocks since those two numbers are fixed by RFC 9171 clause 4.1.
func humanizeTargets(targets []uint64) string {
	parts := make([]string, 0, len(targets))
	for _, t := range targets {
		switch t {
		case bp.PrimaryBlockNumber:
			parts = append(parts, "0 (primary)")
		case bp.PayloadBlockNumber:
			parts = append(parts, "1 (payload)")
		default:
			parts = append(parts, fmt.Sprintf("%d", t))
		}
	}
	return strings.Join(parts, ", ")
}

func (a *ASB) humanizeParameter(p Parameter) string {
	switch a.ContextID {
	case ContextBIBHMACSHA2:
		switch p.ID {
		case ParamSHAVariant:
			if v, err := decodeUint(p.Value); err == nil {
				return fmt.Sprintf("SHA variant: %s", SHAVariant(v))
			}
		case ParamBIBWrappedKey:
			return fmt.Sprintf("wrapped key: %s", humanizeByteString(p.Value))
		case ParamIntegrityScope:
			if v, err := decodeUint(p.Value); err == nil {
				return fmt.Sprintf("integrity scope: %s", ScopeFlags(v).Humanize())
			}
		}
	case ContextBCBAESGCM:
		switch p.ID {
		case ParamIV:
			return fmt.Sprintf("IV: %s", humanizeByteString(p.Value))
		case ParamAESVariant:
			if v, err := decodeUint(p.Value); err == nil {
				return fmt.Sprintf("AES variant: %s", AESVariant(v))
			}
		case ParamBCBWrappedKey:
			return fmt.Sprintf("wrapped key: %s", humanizeByteString(p.Value))
		case ParamAADScope:
			if v, err := decodeUint(p.Value); err == nil {
				return fmt.Sprintf("AAD scope: %s", ScopeFlags(v).Humanize())
			}
		}
	}
	return fmt.Sprintf("parameter %d: %x", p.ID, p.Value)
}

func (a *ASB) humanizeResult(r Result) string {
	switch {
	case a.ContextID == ContextBIBHMACSHA2 && r.ID == ResultExpectedHMAC:
		return fmt.Sprintf("HMAC %s", humanizeByteString(r.Value))
	case a.ContextID == ContextBCBAESGCM && r.ID == ResultAuthenticationTag:
		return fmt.Sprintf("tag %s", humanizeByteString(r.Value))
	}
	return fmt.Sprintf("result %d: %x", r.ID, r.Value)
}

// humanizeByteString shows the contents of a CBOR byte string, abbreviating
// anything long enough to bury the rest of the line.
func humanizeByteString(raw []byte) string {
	b, err := decodeByteString(raw)
	if err != nil {
		return fmt.Sprintf("(unreadable: %x)", raw)
	}
	if len(b) <= 16 {
		return hex.EncodeToString(b)
	}
	return fmt.Sprintf("%x… (%d octets)", b[:16], len(b))
}

// Humanize names the scope flags that are set.
func (f ScopeFlags) Humanize() string {
	if f == 0 {
		return "none (target contents only)"
	}

	var parts []string
	if f.Has(ScopePrimaryBlock) {
		parts = append(parts, "primary block")
	}
	if f.Has(ScopeTargetHeader) {
		parts = append(parts, "target header")
	}
	if f.Has(ScopeSecurityHeader) {
		parts = append(parts, "security header")
	}
	if unknown := f &^ ScopeAll; unknown != 0 {
		parts = append(parts, fmt.Sprintf("unassigned 0x%04x", uint16(unknown)))
	}
	return strings.Join(parts, ", ")
}

// Humanize summarises a BIB or a BCB, decoding the Abstract Security Block it
// carries. A block whose data does not decode says so rather than failing:
// this is a dump, and an unreadable block is the interesting case.
func Humanize(b *bp.CanonicalBlock) string {
	if !IsSecurityBlock(b) {
		return fmt.Sprintf("block %d is not a security block", b.Number)
	}

	name := "BIB"
	if b.Type == BlockTypeConfidentiality {
		name = "BCB"
	}

	asb, err := DecodeASB(b.Data)
	if err != nil {
		return fmt.Sprintf("%s (block %d): unreadable: %v", name, b.Number, err)
	}
	return fmt.Sprintf("%s (block %d)\n%s", name, b.Number, asb.Humanize())
}
