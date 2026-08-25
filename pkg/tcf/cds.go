package tcf

import (
	"encoding/binary"
	"strconv"
	"strings"
	"time"
)

// CDS represents a CCSDS Day Segmented Time Code per CCSDS 301.0-B-4 §3.3.
//
// The T-field is segmented into a day count, milliseconds of day, and optional
// sub-millisecond precision.
//
//	+-------------+-----------------+----------------------+
//	| Day (16/24) | Milliseconds(32)| Sub-ms (0/16/32 bit) |
//	+-------------+-----------------+----------------------+
//
// Sub-millisecond precision:
//
//	0 bytes: none
//	2 bytes: microseconds within the millisecond (0-999)
//	4 bytes: picoseconds within the millisecond (0-999999999)
//
// Time scale and leap seconds: CDS conversions are purely arithmetic in both
// levels — the day count is elapsed 86400-second days since the epoch and no
// leap-second table is applied. The segmentation cannot represent the
// inserted leap second itself: milliseconds-of-day is capped at 86399999, so
// UTC 23:59:60 has no encoding, and on a leap-second day the code is
// effectively a UTC day/time-of-day label rather than a true elapsed-time
// count (across a real leap second the arithmetic day boundary and the UTC
// midnight differ by the inserted second). This matches the common
// convention of treating CDS day/millisecond fields as UTC; missions
// requiring true TAI elapsed time should use CUC Level 1 instead.
type CDS struct {
	PField          PField    // Preamble field
	Day             uint32    // Day count since epoch (16 or 24 bits)
	Milliseconds    uint32    // Milliseconds of day (0-86399999)
	Submilliseconds uint32    // Sub-millisecond value (interpretation depends on SubmsBytes)
	DayBytes        uint8     // Day segment width: 2 (16-bit) or 3 (24-bit)
	SubmsBytes      uint8     // Sub-millisecond width: 0, 2, or 4
	Epoch           time.Time // Reference epoch (CCSDSEpoch for Level 1)
}

// CDSOption configures a CDS time code.
type CDSOption func(*CDS) error

// WithCDSDayBytes sets the day segment width (2 for 16-bit, 3 for 24-bit).
func WithCDSDayBytes(n uint8) CDSOption {
	return func(c *CDS) error {
		if n != 2 && n != 3 {
			return ErrInvalidDaySegment
		}
		c.DayBytes = n
		return nil
	}
}

// WithCDSSubmsBytes sets the sub-millisecond width (0, 2, or 4 bytes).
func WithCDSSubmsBytes(n uint8) CDSOption {
	return func(c *CDS) error {
		if n != 0 && n != 2 && n != 4 {
			return ErrInvalidCalendarTime
		}
		c.SubmsBytes = n
		return nil
	}
}

// WithCDSEpoch sets a custom epoch for Level 2 CDS codes.
func WithCDSEpoch(epoch time.Time) CDSOption {
	return func(c *CDS) error {
		c.Epoch = epoch
		return nil
	}
}

// NewCDS creates a CDS time code from a Go time.Time value.
// Defaults to Level 1 (CCSDS epoch), 16-bit day, no sub-milliseconds.
// The conversion is purely arithmetic; see the CDS type documentation for
// leap-second-day behavior.
func NewCDS(t time.Time, opts ...CDSOption) (*CDS, error) {
	c := &CDS{
		DayBytes:   2,
		SubmsBytes: 0,
		Epoch:      CCSDSEpoch,
	}

	for _, opt := range opts {
		if err := opt(c); err != nil {
			return nil, err
		}
	}

	// Split arithmetic on Unix seconds avoids the ~292-year range limit of
	// time.Duration for large day counts.
	secs, nanos := epochDelta(t, c.Epoch)
	if secs < 0 {
		return nil, ErrOverflow
	}

	const secsPerDay = 86400
	day := secs / secsPerDay

	// Check day fits in configured width before narrowing.
	maxDay := int64(1)<<(uint(c.DayBytes)*8) - 1
	if day > maxDay {
		return nil, ErrOverflow
	}
	c.Day = uint32(day)
	c.Milliseconds = uint32((secs%secsPerDay)*1000 + nanos/int64(time.Millisecond))

	// Calculate sub-milliseconds within the current millisecond.
	fracNanos := nanos % int64(time.Millisecond)
	switch c.SubmsBytes {
	case 2:
		// Microseconds within the current millisecond (0-999)
		c.Submilliseconds = uint32(fracNanos / int64(time.Microsecond))
	case 4:
		// Picoseconds within the current millisecond (0-999999999)
		c.Submilliseconds = uint32(fracNanos * 1000) // ns to ps
	}

	if err := c.buildPField(); err != nil {
		return nil, err
	}

	return c, nil
}

// Encode serializes the CDS time code into bytes (P-field + T-field).
func (c *CDS) Encode() ([]byte, error) {
	pBytes, err := c.PField.Encode()
	if err != nil {
		return nil, err
	}

	tField, err := c.EncodeTField()
	if err != nil {
		return nil, err
	}

	return append(pBytes, tField...), nil
}

// EncodeTField serializes only the T-field (no P-field). Use this for
// implicit-P-field contexts, e.g. Space Packet secondary headers, where the
// format parameters are agreed out of band.
func (c *CDS) EncodeTField() ([]byte, error) {
	if err := c.Validate(); err != nil {
		return nil, err
	}

	tFieldLen := int(c.DayBytes) + 4 + int(c.SubmsBytes)
	tField := make([]byte, tFieldLen)
	offset := 0

	// Encode day (big-endian, 2 or 3 bytes)
	if c.DayBytes == 3 {
		tField[0] = byte(c.Day >> 16)
		tField[1] = byte(c.Day >> 8)
		tField[2] = byte(c.Day)
		offset = 3
	} else {
		binary.BigEndian.PutUint16(tField[0:2], uint16(c.Day))
		offset = 2
	}

	// Encode milliseconds (32-bit big-endian)
	binary.BigEndian.PutUint32(tField[offset:offset+4], c.Milliseconds)
	offset += 4

	// Encode sub-milliseconds
	switch c.SubmsBytes {
	case 2:
		binary.BigEndian.PutUint16(tField[offset:offset+2], uint16(c.Submilliseconds))
	case 4:
		binary.BigEndian.PutUint32(tField[offset:offset+4], c.Submilliseconds)
	}

	return tField, nil
}

// DecodeCDS parses a byte slice into a CDS time code (P-field + T-field).
// If epoch is zero-value, Level 1 (CCSDS epoch) is assumed.
//
// The reserved sub-millisecond code '11' (§3.3.2) is rejected with
// ErrReservedSubmsCode: its T-field length is undefined, so decoding cannot
// proceed safely.
func DecodeCDS(data []byte, epoch time.Time) (*CDS, error) {
	if len(data) < 2 {
		return nil, ErrDataTooShort
	}

	c := &CDS{}

	if err := c.PField.Decode(data); err != nil {
		return nil, err
	}

	if c.PField.TimeCodeID != TimeCodeCDS {
		return nil, ErrInvalidTimeCodeID
	}

	// The CDS P-field is a single octet (§3.3.2): the extension flag is
	// always zero. A set flag would misalign the T-field.
	if c.PField.Extension {
		return nil, ErrInvalidPField
	}

	// Extract format from P-field detail bits (§3.3.2)
	// Bit 4 (Detail bit 3): epoch identification (0=Level 1, 1=Level 2)
	// Bit 5 (Detail bit 2): day segment length (0=16-bit, 1=24-bit)
	// Bits 6-7 (Detail bits 1-0): sub-ms precision (00=none, 01=µs, 10=ps, 11=reserved)
	isLevel2 := (c.PField.Detail>>3)&0x01 == 1
	if (c.PField.Detail>>2)&0x01 == 1 {
		c.DayBytes = 3
	} else {
		c.DayBytes = 2
	}

	switch c.PField.Detail & 0x03 {
	case 0:
		c.SubmsBytes = 0
	case 1:
		c.SubmsBytes = 2
	case 2:
		c.SubmsBytes = 4
	case 3:
		return nil, ErrReservedSubmsCode
	}

	// Set epoch
	if isLevel2 {
		if epoch.IsZero() {
			return nil, ErrEpochRequired
		}
		c.Epoch = epoch
	} else {
		c.Epoch = CCSDSEpoch
	}

	if err := c.decodeTField(data[c.PField.Size():]); err != nil {
		return nil, err
	}

	return c, c.Validate()
}

// DecodeCDSTField parses a T-field-only (implicit P-field) CDS time code.
// The format parameters — day segment width (2 or 3), sub-millisecond width
// (0, 2, or 4) and the epoch — must be supplied by the caller, as they are
// in contexts like Space Packet secondary headers where no P-field is
// transmitted.
//
// If epoch is zero-value, Level 1 (CCSDS epoch) is assumed; otherwise the
// code is treated as Level 2 with the given agency-defined epoch.
func DecodeCDSTField(data []byte, dayBytes, submsBytes uint8, epoch time.Time) (*CDS, error) {
	if dayBytes != 2 && dayBytes != 3 {
		return nil, ErrInvalidDaySegment
	}
	if submsBytes != 0 && submsBytes != 2 && submsBytes != 4 {
		return nil, ErrInvalidCalendarTime
	}

	c := &CDS{
		DayBytes:   dayBytes,
		SubmsBytes: submsBytes,
		Epoch:      epoch,
	}
	if epoch.IsZero() {
		c.Epoch = CCSDSEpoch
	}

	if err := c.buildPField(); err != nil {
		return nil, err
	}

	if err := c.decodeTField(data); err != nil {
		return nil, err
	}

	return c, c.Validate()
}

// decodeTField parses the day, millisecond, and sub-millisecond segments
// from tf, which must start at the first T-field octet.
func (c *CDS) decodeTField(tf []byte) error {
	tFieldLen := int(c.DayBytes) + 4 + int(c.SubmsBytes)
	if len(tf) < tFieldLen {
		return ErrDataTooShort
	}

	offset := 0

	// Decode day
	if c.DayBytes == 3 {
		c.Day = uint32(tf[0])<<16 | uint32(tf[1])<<8 | uint32(tf[2])
		offset = 3
	} else {
		c.Day = uint32(binary.BigEndian.Uint16(tf[0:2]))
		offset = 2
	}

	// Decode milliseconds
	c.Milliseconds = binary.BigEndian.Uint32(tf[offset : offset+4])
	offset += 4

	// Decode sub-milliseconds
	switch c.SubmsBytes {
	case 2:
		c.Submilliseconds = uint32(binary.BigEndian.Uint16(tf[offset : offset+2]))
	case 4:
		c.Submilliseconds = binary.BigEndian.Uint32(tf[offset : offset+4])
	}

	return nil
}

// Time converts the CDS time code to a Go time.Time value in UTC.
// The conversion is purely arithmetic (no leap-second correction); see the
// CDS type documentation. Picosecond sub-milliseconds are truncated toward
// zero to Go's nanosecond resolution.
func (c *CDS) Time() time.Time {
	// time.Unix-based split arithmetic: a 24-bit day count (~46,000 years)
	// would overflow time.Duration.
	secs := c.Epoch.Unix() + int64(c.Day)*86400 + int64(c.Milliseconds)/1000
	nsec := int64(c.Epoch.Nanosecond()) + (int64(c.Milliseconds)%1000)*int64(time.Millisecond)

	switch c.SubmsBytes {
	case 2:
		nsec += int64(c.Submilliseconds) * int64(time.Microsecond)
	case 4:
		// Picoseconds: Go time only has nanosecond precision
		nsec += int64(c.Submilliseconds) / 1000
	}

	return time.Unix(secs, nsec).UTC()
}

// Validate checks that the CDS fields conform to CCSDS 301.0-B-4.
func (c *CDS) Validate() error {
	if c.DayBytes != 2 && c.DayBytes != 3 {
		return ErrInvalidDaySegment
	}
	maxDay := uint32(1)<<(uint(c.DayBytes)*8) - 1
	if c.Day > maxDay {
		return ErrInvalidDaySegment
	}
	if c.Milliseconds > 86399999 {
		return ErrInvalidMilliseconds
	}
	switch c.SubmsBytes {
	case 0:
		if c.Submilliseconds != 0 {
			return ErrInvalidSubmilliseconds
		}
	case 2:
		// 16-bit field carries microseconds within the millisecond (0-999).
		if c.Submilliseconds > 999 {
			return ErrInvalidSubmilliseconds
		}
	case 4:
		// 32-bit field carries picoseconds within the millisecond
		// (0-999999999).
		if c.Submilliseconds > 999999999 {
			return ErrInvalidSubmilliseconds
		}
	default:
		return ErrInvalidCalendarTime
	}
	return nil
}

// Humanize returns a human-readable representation of the CDS time code.
func (c *CDS) Humanize() string {
	level := "Level 1 (CCSDS epoch)"
	if !isCCSDSEpoch(c.Epoch) {
		level = "Level 2 (agency-defined epoch)"
	}

	parts := []string{
		"CDS Time Code:",
		"  " + level,
		"  Day: " + strconv.FormatUint(uint64(c.Day), 10),
		"  Milliseconds: " + strconv.FormatUint(uint64(c.Milliseconds), 10),
		"  Day Octets: " + strconv.Itoa(int(c.DayBytes)),
	}

	if c.SubmsBytes > 0 {
		var label string
		switch c.SubmsBytes {
		case 2:
			label = "Microseconds"
		case 4:
			label = "Picoseconds"
		default:
			label = "Submilliseconds"
		}
		parts = append(parts, "  "+label+": "+strconv.FormatUint(uint64(c.Submilliseconds), 10))
	}

	parts = append(parts, "  Time: "+c.Time().UTC().Format(time.RFC3339Nano))

	return strings.Join(parts, "\n")
}

// buildPField constructs the P-field from the CDS configuration (§3.3.2).
// Bit 4 = epoch (0=Level 1, 1=Level 2)
// Bit 5 = day segment length (0=16-bit, 1=24-bit)
// Bits 6-7 = sub-millisecond resolution
func (c *CDS) buildPField() error {
	var epochBit uint8
	if !isCCSDSEpoch(c.Epoch) {
		epochBit = 1
	}

	var dayBit uint8
	if c.DayBytes == 3 {
		dayBit = 1
	}

	var submsBits uint8
	switch c.SubmsBytes {
	case 2:
		submsBits = 1
	case 4:
		submsBits = 2
	}

	c.PField = PField{
		Extension:  false,
		TimeCodeID: TimeCodeCDS,
		Detail:     (epochBit << 3) | (dayBit << 2) | submsBits,
	}

	return nil
}
