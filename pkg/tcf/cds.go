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
// +-------------+-----------------+----------------------+
// | Day (16/24) | Milliseconds(32)| Sub-ms (0/16/32 bit) |
// +-------------+-----------------+----------------------+
//
// Sub-millisecond precision:
//
//	0 bytes: none
//	2 bytes: microseconds (0-999)
//	4 bytes: picoseconds (0-999999999)
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

	elapsed := t.Sub(c.Epoch)
	if elapsed < 0 {
		return nil, ErrOverflow
	}

	// Calculate day and milliseconds
	totalMs := elapsed.Milliseconds()
	msPerDay := int64(86400000)
	c.Day = uint32(totalMs / msPerDay)
	c.Milliseconds = uint32(totalMs % msPerDay)

	// Check day fits in configured width
	maxDay := uint32(1)<<(uint(c.DayBytes)*8) - 1
	if c.Day > maxDay {
		return nil, ErrOverflow
	}

	// Calculate sub-milliseconds
	switch c.SubmsBytes {
	case 2:
		// Microseconds within the current millisecond (0-999)
		fracNanos := elapsed.Nanoseconds() - (totalMs * int64(time.Millisecond))
		c.Submilliseconds = uint32(fracNanos / int64(time.Microsecond))
	case 4:
		// Picoseconds within the current millisecond (0-999999999)
		fracNanos := elapsed.Nanoseconds() - (totalMs * int64(time.Millisecond))
		c.Submilliseconds = uint32(fracNanos * 1000) // ns to ps
	}

	if err := c.buildPField(); err != nil {
		return nil, err
	}

	return c, nil
}

// Encode serializes the CDS time code into bytes (P-field + T-field).
func (c *CDS) Encode() ([]byte, error) {
	if err := c.Validate(); err != nil {
		return nil, err
	}

	pBytes, err := c.PField.Encode()
	if err != nil {
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

	return append(pBytes, tField...), nil
}

// DecodeCDS parses a byte slice into a CDS time code.
// If epoch is zero-value, Level 1 (CCSDS epoch) is assumed.
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

	// Extract format from P-field detail bits (§3.3.2)
	// Bit 4 (Detail bit 3): epoch identification (0=Level 1, 1=Level 2)
	// Bit 5 (Detail bit 2): day segment length (0=16-bit, 1=24-bit)
	// Bits 6-7 (Detail bits 1-0): sub-ms precision (00=none, 01=µs, 10=ps)
	isLevel2 := (c.PField.Detail>>3)&0x01 == 1
	if (c.PField.Detail>>2)&0x01 == 1 {
		c.DayBytes = 3
	} else {
		c.DayBytes = 2
	}

	submsPrecision := c.PField.Detail & 0x03
	switch submsPrecision {
	case 0:
		c.SubmsBytes = 0
	case 1:
		c.SubmsBytes = 2
	case 2:
		c.SubmsBytes = 4
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

	// Parse T-field
	offset := c.PField.Size()
	tFieldLen := int(c.DayBytes) + 4 + int(c.SubmsBytes)
	if len(data) < offset+tFieldLen {
		return nil, ErrDataTooShort
	}

	// Decode day
	if c.DayBytes == 3 {
		c.Day = uint32(data[offset])<<16 | uint32(data[offset+1])<<8 | uint32(data[offset+2])
		offset += 3
	} else {
		c.Day = uint32(binary.BigEndian.Uint16(data[offset : offset+2]))
		offset += 2
	}

	// Decode milliseconds
	c.Milliseconds = binary.BigEndian.Uint32(data[offset : offset+4])
	offset += 4

	// Decode sub-milliseconds
	switch c.SubmsBytes {
	case 2:
		c.Submilliseconds = uint32(binary.BigEndian.Uint16(data[offset : offset+2]))
	case 4:
		c.Submilliseconds = binary.BigEndian.Uint32(data[offset : offset+4])
	}

	return c, c.Validate()
}

// Time converts the CDS time code to a Go time.Time value.
func (c *CDS) Time() time.Time {
	t := c.Epoch
	t = t.Add(time.Duration(c.Day) * 24 * time.Hour)
	t = t.Add(time.Duration(c.Milliseconds) * time.Millisecond)

	switch c.SubmsBytes {
	case 2:
		t = t.Add(time.Duration(c.Submilliseconds) * time.Microsecond)
	case 4:
		// Picoseconds: Go time only has nanosecond precision
		t = t.Add(time.Duration(c.Submilliseconds/1000) * time.Nanosecond)
	}

	return t
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
	if c.SubmsBytes != 0 && c.SubmsBytes != 2 && c.SubmsBytes != 4 {
		return ErrInvalidCalendarTime
	}
	return nil
}

// Humanize returns a human-readable representation of the CDS time code.
func (c *CDS) Humanize() string {
	level := "Level 1 (CCSDS epoch)"
	if c.Epoch != CCSDSEpoch {
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
	if c.Epoch != CCSDSEpoch {
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
