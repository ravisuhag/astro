package tcf

import (
	"strconv"
	"strings"
	"time"
)

// CCS represents a CCSDS Calendar Segmented Time Code per CCSDS 301.0-B-4 §3.4.
//
// All segments use Binary Coded Decimal (BCD) encoding where each 8-bit
// segment represents two decimal digits.
//
// Day-of-Year variant (bit 4 = 1):
// +----------+--------+------+------+------+------------------+
// | Year(16) | DOY(16)| H(8) | M(8) | S(8) | Sub-s(0-6 oct)   |
// +----------+--------+------+------+------+------------------+
//
// Month/Day variant (bit 4 = 0):
// +----------+------+------+------+------+------+------------------+
// | Year(16) | Mo(8)| Dom(8)| H(8) | M(8) | S(8) | Sub-s(0-6 oct)   |
// +----------+------+------+------+------+------+------------------+
//
// Sub-second resolution: 0 to 6 additional octets, each containing 2 BCD
// digits, giving 10^-2 to 10^-12 second resolution.
type CCS struct {
	PField      PField   // Preamble field
	Year        uint16   // Calendar year
	Month       uint8    // Month (1-12), only for Month/Day variant
	DayOfMonth  uint8    // Day of month (1-31), only for Month/Day variant
	DayOfYear   uint16   // Day of year (1-366), only for Day-of-Year variant
	Hour        uint8    // Hour (0-23)
	Minute      uint8    // Minute (0-59)
	Second      uint8    // Second (0-60, 60 for leap second)
	SubSecond   [6]uint8 // Sub-second BCD octets (each holds 2 decimal digits, 0-99)
	SubSecBytes uint8    // Number of sub-second octets (0-6)
	MonthDay    bool     // true = Month/Day variant (bit 4=0), false = Day-of-Year variant (bit 4=1)
}

// CCSOption configures a CCS time code.
type CCSOption func(*CCS) error

// WithCCSMonthDay selects the Month/Day variant instead of Day-of-Year.
func WithCCSMonthDay() CCSOption {
	return func(c *CCS) error {
		c.MonthDay = true
		return nil
	}
}

// WithCCSSubSecBytes sets the number of sub-second octets (0-6).
// Each octet provides two additional decimal digits of precision.
func WithCCSSubSecBytes(n uint8) CCSOption {
	return func(c *CCS) error {
		if n > 6 {
			return ErrInvalidCalendarTime
		}
		c.SubSecBytes = n
		return nil
	}
}

// NewCCS creates a CCS time code from a Go time.Time value.
// Defaults to Day-of-Year variant with no sub-second precision.
func NewCCS(t time.Time, opts ...CCSOption) (*CCS, error) {
	c := &CCS{
		MonthDay:    false,
		SubSecBytes: 0,
	}

	for _, opt := range opts {
		if err := opt(c); err != nil {
			return nil, err
		}
	}

	t = t.UTC()
	c.Year = uint16(t.Year())
	c.Hour = uint8(t.Hour())
	c.Minute = uint8(t.Minute())
	c.Second = uint8(t.Second())

	if c.MonthDay {
		c.Month = uint8(t.Month())
		c.DayOfMonth = uint8(t.Day())
	} else {
		c.DayOfYear = uint16(t.YearDay())
	}

	// Compute sub-second BCD values from nanoseconds.
	// Each octet represents 2 decimal digits (centiseconds, then 10^-4, etc.)
	if c.SubSecBytes > 0 {
		ns := t.Nanosecond()
		// Convert nanoseconds to a 12-digit decimal fraction of a second.
		// ns has 9 digits of precision; we extend with zeros for picoseconds.
		// frac represents the sub-second value in units of 10^-12.
		frac := int64(ns) * 1000 // now in picoseconds (10^-12)
		for i := range int(c.SubSecBytes) {
			// Each BCD octet represents 2 digits.
			// Digit pair i covers 10^(-(2*i+2)) resolution.
			// divisor for digit pair i: 10^(12 - 2*(i+1)) = 10^(10-2i)
			divisor := int64(1)
			for j := range 10 - 2*i {
				_ = j
				divisor *= 10
			}
			pair := uint8((frac / divisor) % 100)
			c.SubSecond[i] = pair
		}
	}

	if err := c.buildPField(); err != nil {
		return nil, err
	}

	if err := c.Validate(); err != nil {
		return nil, err
	}

	return c, nil
}

// Encode serializes the CCS time code into bytes (P-field + T-field).
// All segments are BCD-encoded per §3.4.1.
func (c *CCS) Encode() ([]byte, error) {
	if err := c.Validate(); err != nil {
		return nil, err
	}

	pBytes, err := c.PField.Encode()
	if err != nil {
		return nil, err
	}

	var tField []byte

	// Year: 16-bit BCD (4 digits)
	tField = append(tField, toBCD16(c.Year)...)

	if c.MonthDay {
		tField = append(tField, toBCD8(c.Month), toBCD8(c.DayOfMonth))
	} else {
		// DOY: 16-bit, upper 4 bits zero, lower 12 bits BCD for 3 digits
		tField = append(tField, toBCD16DOY(c.DayOfYear)...)
	}

	tField = append(tField, toBCD8(c.Hour), toBCD8(c.Minute), toBCD8(c.Second))

	// Sub-second octets (already BCD pairs)
	for i := range int(c.SubSecBytes) {
		tField = append(tField, toBCD8(c.SubSecond[i]))
	}

	return append(pBytes, tField...), nil
}

// DecodeCCS parses a byte slice into a CCS time code.
func DecodeCCS(data []byte) (*CCS, error) {
	if len(data) < 2 {
		return nil, ErrDataTooShort
	}

	c := &CCS{}

	if err := c.PField.Decode(data); err != nil {
		return nil, err
	}

	if c.PField.TimeCodeID != TimeCodeCCS {
		return nil, ErrInvalidTimeCodeID
	}

	// Extract format from P-field detail bits (§3.4.2)
	// Bit 4 (Detail bit 3): calendar variation (0=Month/Day, 1=Day-of-Year)
	// Bits 5-7 (Detail bits 2-0): resolution (number of sub-second octets)
	c.MonthDay = (c.PField.Detail>>3)&0x01 == 0
	c.SubSecBytes = c.PField.Detail & 0x07

	// Parse T-field
	offset := c.PField.Size()

	// Minimum T-field: year(2) + day-variant(2) + H(1) + M(1) + S(1) = 7
	minLen := 7 + int(c.SubSecBytes)
	if len(data) < offset+minLen {
		return nil, ErrDataTooShort
	}

	// Year: 16-bit BCD
	c.Year = fromBCD16(data[offset], data[offset+1])
	offset += 2

	if c.MonthDay {
		c.Month = fromBCD8(data[offset])
		c.DayOfMonth = fromBCD8(data[offset+1])
	} else {
		c.DayOfYear = fromBCD16DOY(data[offset], data[offset+1])
	}
	offset += 2

	c.Hour = fromBCD8(data[offset])
	c.Minute = fromBCD8(data[offset+1])
	c.Second = fromBCD8(data[offset+2])
	offset += 3

	for i := range int(c.SubSecBytes) {
		c.SubSecond[i] = fromBCD8(data[offset+i])
	}

	return c, c.Validate()
}

// Time converts the CCS time code to a Go time.Time value.
func (c *CCS) Time() time.Time {
	var t time.Time
	if c.MonthDay {
		t = time.Date(int(c.Year), time.Month(c.Month), int(c.DayOfMonth),
			int(c.Hour), int(c.Minute), int(c.Second), 0, time.UTC)
	} else {
		// Day-of-Year: start at Jan 1 and add days
		t = time.Date(int(c.Year), 1, 1, int(c.Hour), int(c.Minute), int(c.Second), 0, time.UTC)
		t = t.AddDate(0, 0, int(c.DayOfYear)-1)
	}

	// Add sub-second precision from BCD pairs.
	// Each octet is 2 decimal digits. Octet 0 = centiseconds, octet 1 = 10^-4 s, etc.
	if c.SubSecBytes > 0 {
		var picos int64
		for i := range int(c.SubSecBytes) {
			// Octet i contributes at scale 10^(-(2*i+2)) seconds = 10^(10-2*i) picoseconds
			scale := int64(1)
			for j := range 10 - 2*i {
				_ = j
				scale *= 10
			}
			picos += int64(c.SubSecond[i]) * scale
		}
		// Go time has nanosecond precision; convert picoseconds to nanoseconds
		t = t.Add(time.Duration(picos/1000) * time.Nanosecond)
	}

	return t
}

// Validate checks that the CCS fields conform to CCSDS 301.0-B-4.
func (c *CCS) Validate() error {
	if c.SubSecBytes > 6 {
		return ErrInvalidCalendarTime
	}
	if c.Hour > 23 {
		return ErrInvalidCalendarTime
	}
	if c.Minute > 59 {
		return ErrInvalidCalendarTime
	}
	if c.Second > 60 { // 60 allowed for leap second
		return ErrInvalidCalendarTime
	}
	if c.MonthDay {
		if c.Month < 1 || c.Month > 12 {
			return ErrInvalidCalendarTime
		}
		if c.DayOfMonth < 1 || c.DayOfMonth > 31 {
			return ErrInvalidCalendarTime
		}
	} else {
		if c.DayOfYear < 1 || c.DayOfYear > 366 {
			return ErrInvalidCalendarTime
		}
	}
	for i := range int(c.SubSecBytes) {
		if c.SubSecond[i] > 99 {
			return ErrInvalidCalendarTime
		}
	}
	return nil
}

// Humanize returns a human-readable representation of the CCS time code.
func (c *CCS) Humanize() string {
	variant := "Day-of-Year"
	if c.MonthDay {
		variant = "Month-Day"
	}

	parts := []string{
		"CCS Time Code:",
		"  Variant: " + variant,
		"  Year: " + strconv.Itoa(int(c.Year)),
	}

	if c.MonthDay {
		parts = append(parts,
			"  Month: "+strconv.Itoa(int(c.Month)),
			"  Day: "+strconv.Itoa(int(c.DayOfMonth)),
		)
	} else {
		parts = append(parts, "  Day of Year: "+strconv.Itoa(int(c.DayOfYear)))
	}

	parts = append(parts,
		"  Hour: "+strconv.Itoa(int(c.Hour)),
		"  Minute: "+strconv.Itoa(int(c.Minute)),
		"  Second: "+strconv.Itoa(int(c.Second)),
		"  Sub-second Octets: "+strconv.Itoa(int(c.SubSecBytes)),
		"  Time: "+c.Time().UTC().Format(time.RFC3339Nano),
	)

	return strings.Join(parts, "\n")
}

// buildPField constructs the P-field from the CCS configuration (§3.4.2).
// Bit 4: calendar variation (0=Month/Day, 1=Day-of-Year)
// Bits 5-7: number of sub-second octets (resolution)
func (c *CCS) buildPField() error {
	var calBit uint8
	if !c.MonthDay {
		calBit = 1 // 1 = day-of-year variation
	}

	c.PField = PField{
		Extension:  false,
		TimeCodeID: TimeCodeCCS,
		Detail:     (calBit << 3) | (c.SubSecBytes & 0x07),
	}

	return nil
}

// BCD encoding/decoding helpers.

// toBCD8 converts a value 0-99 to a BCD byte.
func toBCD8(v uint8) byte {
	return ((v / 10) << 4) | (v % 10)
}

// fromBCD8 converts a BCD byte to a value 0-99.
func fromBCD8(b byte) uint8 {
	return (b>>4)*10 + (b & 0x0F)
}

// toBCD16 converts a 4-digit value to 2 BCD bytes (e.g., year 2024 → 0x20 0x24).
func toBCD16(v uint16) []byte {
	hi := uint8(v / 100)
	lo := uint8(v % 100)
	return []byte{toBCD8(hi), toBCD8(lo)}
}

// fromBCD16 converts 2 BCD bytes to a 4-digit value.
func fromBCD16(b0, b1 byte) uint16 {
	return uint16(fromBCD8(b0))*100 + uint16(fromBCD8(b1))
}

// toBCD16DOY converts a day-of-year (1-366) to 2 BCD bytes.
// Upper 4 bits of first byte are zero per §3.4.1.2.
func toBCD16DOY(v uint16) []byte {
	hundreds := uint8(v / 100)
	tens := uint8((v % 100) / 10)
	ones := uint8(v % 10)
	return []byte{hundreds & 0x0F, (tens << 4) | ones}
}

// fromBCD16DOY converts 2 BCD bytes to a day-of-year value.
// Upper 4 bits of first byte are zero per §3.4.1.2.
func fromBCD16DOY(b0, b1 byte) uint16 {
	hundreds := uint16(b0 & 0x0F)
	tens := uint16(b1 >> 4)
	ones := uint16(b1 & 0x0F)
	return hundreds*100 + tens*10 + ones
}
