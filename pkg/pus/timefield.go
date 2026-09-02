package pus

import (
	"time"

	"github.com/ravisuhag/astro/pkg/tcf"
)

// Absolute and relative time fields.
//
// The TM secondary header carries an absolute time, and so do several message
// bodies — a scheduled activity's release time, a time window's tags. They all
// use the same field, so the codec lives here rather than on the header.
//
// Relative time is a different field type with its own PTC and its own PFC
// table, and clause 7.3.11 makes it signed. It is here too.

// encodeAbsoluteTime serializes an absolute time field per the profile
// (clause 7.4.3.1j and Table 7-10).
//
// raw supplies the octets when the profile declares TimeRaw, where this
// package moves a field it does not interpret.
func encodeAbsoluteTime(p MissionProfile, t time.Time, raw []byte) ([]byte, error) {
	switch p.TimeFormat {
	case TimeNone:
		return nil, nil
	case TimeRaw:
		if len(raw) != p.TimeRawBytes {
			return nil, ErrInvalidProfile
		}
		return raw, nil
	case TimeCUC, TimeCUCExplicit:
		c, err := tcf.NewCUC(t, p.cucOptions()...)
		if err != nil {
			return nil, err
		}
		encoded, err := c.Encode()
		if err != nil {
			return nil, err
		}
		if p.TimeFormat == TimeCUCExplicit {
			return encoded, nil
		}
		// PFC 3 to 46 carry the T-field alone; pkg/tcf always prefixes the
		// P-field, so strip it back off (Table 7-10).
		pSize := c.PField.Size()
		if len(encoded) < pSize {
			return nil, ErrUnsupportedTimeFormat
		}
		return encoded[pSize:], nil
	default:
		return nil, ErrUnsupportedTimeFormat
	}
}

// decodeAbsoluteTime parses an absolute time field from the front of data,
// returning the instant, the raw octets when the profile declares TimeRaw, and
// how many octets were consumed.
func decodeAbsoluteTime(p MissionProfile, data []byte) (time.Time, []byte, int, error) {
	size := p.TimeSize()
	if len(data) < size {
		return time.Time{}, nil, 0, ErrDataTooShort
	}

	switch p.TimeFormat {
	case TimeNone:
		return time.Time{}, nil, 0, nil
	case TimeRaw:
		raw := make([]byte, size)
		copy(raw, data[:size])
		return time.Time{}, raw, size, nil
	case TimeCUCExplicit:
		c, err := tcf.DecodeCUC(data[:size], p.epoch())
		if err != nil {
			return time.Time{}, nil, 0, err
		}
		return c.Time(), nil, size, nil
	case TimeCUC:
		// Rebuild the P-field the PFC implies, then hand the whole thing to
		// pkg/tcf (Table 7-10: "the P-field is implicit and derived from the
		// PFC").
		field := make([]byte, 0, cucPFieldSize+size)
		field = append(field, p.implicitCUCPField())
		field = append(field, data[:size]...)

		c, err := tcf.DecodeCUC(field, p.epoch())
		if err != nil {
			return time.Time{}, nil, 0, err
		}
		return c.Time(), nil, size, nil
	default:
		return time.Time{}, nil, 0, ErrUnsupportedTimeFormat
	}
}

// RelativeTime is a PTC 10 relative time offset: a signed number of seconds
// and fractions of a second (clause 7.3.11).
//
// Clause 7.3.11b's note says "a negative time offset is expressed as the
// '2's complement' of the corresponding positive time offset" — of the whole
// coarse-and-fine field, not of the coarse part alone. So the field is one
// two's-complement integer, and that is what Ticks holds. Table 7-11's PFC 3
// to 18 fix the split: coarse octets are (PFC+1)/4 and fine octets are
// (PFC+1) mod 4.
//
// Ticks rather than a time.Duration because the two do not round-trip. A fine
// field of three octets resolves 2^-24 of a second, about 60 ns, and the
// nearest whole nanosecond is not the same number: a Duration would lose the
// low bits and re-encode to different octets. Duration is offered as a
// convenience for arithmetic, not as the stored form.
type RelativeTime struct {
	// Ticks is the coarse-and-fine field read as one signed integer, in units
	// of 2^-(8*FineBytes) seconds.
	Ticks int64

	// FineBytes is how many of the field's octets are fraction. Zero means
	// Ticks is whole seconds.
	FineBytes int
}

// NewRelativeTime converts a duration to a relative time at the profile's
// declared widths.
//
// The conversion truncates toward zero when the duration is finer than the
// fine field resolves. It is the caller's arithmetic that decides whether that
// matters; nothing here is stored lossily.
func NewRelativeTime(p MissionProfile, d time.Duration) (RelativeTime, error) {
	fine := p.RelativeFineSize()
	scale := int64(1) << (8 * fine)

	// d is nanoseconds. Ticks = d * scale / 1e9, computed so the multiply
	// cannot overflow for the widths Table 7-11 allows.
	seconds := int64(d / time.Second)
	remainder := int64(d % time.Second)

	if seconds > (1<<62)/scale || seconds < -(1<<62)/scale {
		return RelativeTime{}, ErrValueTooLarge
	}
	ticks := seconds*scale + remainder*scale/int64(time.Second)

	r := RelativeTime{Ticks: ticks, FineBytes: fine}
	if !r.fits(p.RelativeCoarseSize() + fine) {
		return RelativeTime{}, ErrValueTooLarge
	}
	return r, nil
}

// fits reports whether Ticks is representable in a two's-complement field of
// width octets.
func (r RelativeTime) fits(width int) bool {
	if width <= 0 || width > 8 {
		return false
	}
	if width == 8 {
		return true
	}
	limit := int64(1) << (8*width - 1)
	return r.Ticks >= -limit && r.Ticks < limit
}

// Duration converts the offset to a time.Duration, rounding toward zero when
// the fine field resolves finer than a nanosecond.
func (r RelativeTime) Duration() time.Duration {
	if r.FineBytes <= 0 {
		return time.Duration(r.Ticks) * time.Second
	}
	scale := int64(1) << (8 * r.FineBytes)
	whole := r.Ticks / scale
	frac := r.Ticks % scale
	return time.Duration(whole)*time.Second +
		time.Duration(frac*int64(time.Second)/scale)
}

// encodeRelativeTime serializes a relative time field per the profile.
func encodeRelativeTime(p MissionProfile, r RelativeTime) ([]byte, error) {
	width := p.RelativeTimeSize()
	if r.FineBytes != p.RelativeFineSize() {
		// A value carrying a different fine width would encode to a different
		// number than it holds.
		return nil, ErrInvalidProfile
	}
	if !r.fits(width) {
		return nil, ErrValueTooLarge
	}
	// Two's complement of the whole field: the low width octets of the
	// integer, big-endian, are the representation for both signs.
	out := make([]byte, width)
	v := uint64(r.Ticks)
	for i := width - 1; i >= 0; i-- {
		out[i] = byte(v)
		v >>= 8
	}
	return out, nil
}

// decodeRelativeTime parses a relative time field from the front of data.
func decodeRelativeTime(p MissionProfile, data []byte) (RelativeTime, int, error) {
	width := p.RelativeTimeSize()
	if len(data) < width {
		return RelativeTime{}, 0, ErrDataTooShort
	}

	var v uint64
	for i := 0; i < width; i++ {
		v = v<<8 | uint64(data[i])
	}
	// Sign-extend from the field's width, which is what makes the value
	// negative rather than merely large.
	if width < 8 && v&(uint64(1)<<(8*width-1)) != 0 {
		v |= ^uint64(0) << (8 * width)
	}

	return RelativeTime{Ticks: int64(v), FineBytes: p.RelativeFineSize()}, width, nil
}
