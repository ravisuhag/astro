package vectors

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math/big"
	"sort"
	"strconv"
	"strings"
)

// Fields is a vector's field map. Values arrive as json.Number, string,
// bool, or nested containers; the accessors below convert them and report
// what went wrong rather than panicking, so one bad fixture names itself.
type Fields map[string]any

// Has reports whether the field is present.
func (f Fields) Has(name string) bool { _, ok := f[name]; return ok }

// Names returns the field names in sorted order, for stable output.
func (f Fields) Names() []string {
	out := make([]string, 0, len(f))
	for k := range f {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// Hex reads a field holding lowercase hex octets.
func (f Fields) Hex(name string) ([]byte, error) {
	v, ok := f[name]
	if !ok {
		return nil, fmt.Errorf("field %q is missing", name)
	}
	s, ok := v.(string)
	if !ok {
		return nil, fmt.Errorf("field %q is %T, want a hex string", name, v)
	}
	if !hexRE.MatchString(s) {
		return nil, fmt.Errorf("field %q = %q is not lowercase hex octet pairs", name, s)
	}
	b, err := hex.DecodeString(s)
	if err != nil {
		return nil, fmt.Errorf("field %q: %w", name, err)
	}
	return b, nil
}

// Uint reads an unsigned integer. A JSON number carries values up to
// 2^53; anything wider is written as a decimal string, so both are
// accepted here and the distinction stays invisible to callers.
func (f Fields) Uint(name string) (uint64, error) {
	v, ok := f[name]
	if !ok {
		return 0, fmt.Errorf("field %q is missing", name)
	}
	switch t := v.(type) {
	case json.Number:
		u, err := strconv.ParseUint(t.String(), 10, 64)
		if err != nil {
			return 0, fmt.Errorf("field %q = %s is not an unsigned integer", name, t)
		}
		return u, nil
	case string:
		u, err := strconv.ParseUint(t, 10, 64)
		if err != nil {
			return 0, fmt.Errorf("field %q = %q is not a decimal unsigned integer", name, t)
		}
		return u, nil
	default:
		return 0, fmt.Errorf("field %q is %T, want a number or decimal string", name, v)
	}
}

// Bool reads a boolean field.
func (f Fields) Bool(name string) (bool, error) {
	v, ok := f[name]
	if !ok {
		return false, fmt.Errorf("field %q is missing", name)
	}
	b, ok := v.(bool)
	if !ok {
		return false, fmt.Errorf("field %q is %T, want a boolean", name, v)
	}
	return b, nil
}

// Str reads a string field.
func (f Fields) Str(name string) (string, error) {
	v, ok := f[name]
	if !ok {
		return "", fmt.Errorf("field %q is missing", name)
	}
	s, ok := v.(string)
	if !ok {
		return "", fmt.Errorf("field %q is %T, want a string", name, v)
	}
	return s, nil
}

// UintOr reads an unsigned integer, returning def when the field is absent.
func (f Fields) UintOr(name string, def uint64) (uint64, error) {
	if !f.Has(name) {
		return def, nil
	}
	return f.Uint(name)
}

// BoolOr reads a boolean, returning def when the field is absent.
func (f Fields) BoolOr(name string, def bool) (bool, error) {
	if !f.Has(name) {
		return def, nil
	}
	return f.Bool(name)
}

// HexOr reads hex octets, returning def when the field is absent.
func (f Fields) HexOr(name string, def []byte) ([]byte, error) {
	if !f.Has(name) {
		return def, nil
	}
	return f.Hex(name)
}

// diff compares the fields listed in want against got, and returns a
// description of every mismatch. Fields present in got but not in want
// are unconstrained and ignored: a vector states what it means to pin,
// and a decoder may expose more than any one vector cares about.
func diff(want, got Fields) []string {
	var out []string
	for _, name := range want.Names() {
		g, ok := got[name]
		if !ok {
			out = append(out, fmt.Sprintf("%s: decoder did not report this field", name))
			continue
		}
		if !sameValue(want[name], g) {
			out = append(out, fmt.Sprintf("%s: got %s, want %s", name, show(g), show(want[name])))
		}
	}
	return out
}

// sameValue compares a vector value against a decoded one across the
// representations each side naturally uses: a JSON number or decimal
// string against any Go integer, a hex string against a byte slice.
//
// The decoded value's type is consulted first, and that ordering is
// load-bearing. A hex string made only of decimal digits — "112233" — is
// also a valid wide-integer decimal string, so the two forms cannot be
// told apart by looking at the vector alone. What disambiguates them is
// the field's type, which the decoder already knows: if it handed back
// octets, the vector's string is hex. This is why CONTRACT.md requires a
// field dictionary per package.
func sameValue(want, got any) bool {
	// Octets first: a vector writes hex, a decoder returns a byte slice.
	if g, ok := got.([]byte); ok {
		w, ok := want.(string)
		if !ok {
			return false
		}
		if !hexRE.MatchString(w) {
			return false
		}
		b, err := hex.DecodeString(w)
		return err == nil && string(b) == string(g)
	}
	// Numbers: compare as big integers so width and signedness do not matter.
	if w, ok := asBig(want); ok {
		if g, ok := asBig(got); ok {
			return w.Cmp(g) == 0
		}
		return false
	}
	if w, ok := want.(bool); ok {
		g, ok := got.(bool)
		return ok && w == g
	}
	if w, ok := want.(string); ok {
		if g, ok := got.(string); ok {
			return w == g
		}
		if g, ok := got.([]byte); ok {
			return w == string(g)
		}
	}
	return fmt.Sprint(want) == fmt.Sprint(got)
}

// asBig converts any integer representation to a big.Int.
func asBig(v any) (*big.Int, bool) {
	switch t := v.(type) {
	case json.Number:
		b, ok := new(big.Int).SetString(t.String(), 10)
		return b, ok
	case int:
		return big.NewInt(int64(t)), true
	case int8:
		return big.NewInt(int64(t)), true
	case int16:
		return big.NewInt(int64(t)), true
	case int32:
		return big.NewInt(int64(t)), true
	case int64:
		return big.NewInt(t), true
	case uint:
		return new(big.Int).SetUint64(uint64(t)), true
	case uint8:
		return new(big.Int).SetUint64(uint64(t)), true
	case uint16:
		return new(big.Int).SetUint64(uint64(t)), true
	case uint32:
		return new(big.Int).SetUint64(uint64(t)), true
	case uint64:
		return new(big.Int).SetUint64(t), true
	case string:
		// A decimal string is a wide integer; a hex string is not a number.
		if t == "" || strings.ContainsAny(t, "abcdefABCDEF") {
			return nil, false
		}
		b, ok := new(big.Int).SetString(t, 10)
		return b, ok
	}
	return nil, false
}

// show renders a value for a failure message.
func show(v any) string {
	switch t := v.(type) {
	case []byte:
		return hex.EncodeToString(t)
	case string:
		return t
	default:
		return fmt.Sprint(v)
	}
}
