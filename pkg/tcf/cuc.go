package tcf

import (
	"strconv"
	"strings"
	"time"
)

// CUC represents a CCSDS Unsegmented Time Code per CCSDS 301.0-B-4 §3.2.
//
// The T-field is a single binary counter split into coarse time (seconds since
// epoch) and fine time (fractional seconds as binary fractions of a second).
//
// +------------------+------------------+
// | Coarse (1-4 oct) | Fine (0-3 oct)   |
// +------------------+------------------+
//
// Fine time resolution:
//
//	0 octets: 1 s
//	1 octet:  ~3.9 ms   (2^-8 s)
//	2 octets: ~15.3 µs  (2^-16 s)
//	3 octets: ~59.6 ns  (2^-24 s)
type CUC struct {
	PField      PField    // Preamble field
	CoarseTime  uint64    // Seconds since epoch
	FineTime    uint32    // Fractional seconds (binary fraction)
	CoarseBytes uint8     // Number of coarse time octets (1-4, up to 7 with extension)
	FineBytes   uint8     // Number of fine time octets (0-3, up to 6 with extension)
	Epoch       time.Time // Reference epoch (CCSDSEpoch for Level 1)
}

// CUCOption configures a CUC time code.
type CUCOption func(*CUC) error

// WithCUCFineBytes sets the number of fine time octets (0-3 basic, up to 6 with extension).
func WithCUCFineBytes(n uint8) CUCOption {
	return func(c *CUC) error {
		if n > 6 {
			return ErrInvalidFineOctets
		}
		c.FineBytes = n
		return nil
	}
}

// WithCUCCoarseBytes sets the number of coarse time octets (1-4 basic, up to 7 with extension).
func WithCUCCoarseBytes(n uint8) CUCOption {
	return func(c *CUC) error {
		if n < 1 || n > 7 {
			return ErrInvalidCoarseOctets
		}
		c.CoarseBytes = n
		return nil
	}
}

// WithCUCEpoch sets a custom epoch for Level 2 CUC codes.
func WithCUCEpoch(epoch time.Time) CUCOption {
	return func(c *CUC) error {
		c.Epoch = epoch
		return nil
	}
}

// NewCUC creates a CUC time code from a Go time.Time value.
// Defaults to Level 1 (CCSDS epoch), 4 coarse octets, 0 fine octets.
func NewCUC(t time.Time, opts ...CUCOption) (*CUC, error) {
	c := &CUC{
		CoarseBytes: 4,
		FineBytes:   0,
		Epoch:       CCSDSEpoch,
	}

	for _, opt := range opts {
		if err := opt(c); err != nil {
			return nil, err
		}
	}

	// Compute coarse and fine time from the time value
	elapsed := t.Sub(c.Epoch)
	if elapsed < 0 {
		return nil, ErrOverflow
	}

	c.CoarseTime = uint64(elapsed.Seconds())
	if c.FineBytes > 0 {
		frac := elapsed - time.Duration(c.CoarseTime)*time.Second
		// Convert fractional part to binary fraction with the configured precision
		totalFineBits := uint(c.FineBytes) * 8
		c.FineTime = uint32((frac.Nanoseconds() << totalFineBits) / int64(time.Second))
	}

	// Check coarse time fits in the configured width
	maxCoarse := uint64(1)<<(uint(c.CoarseBytes)*8) - 1
	if c.CoarseTime > maxCoarse {
		return nil, ErrOverflow
	}

	// Build P-field
	if err := c.buildPField(); err != nil {
		return nil, err
	}

	return c, nil
}

// Encode serializes the CUC time code into bytes (P-field + T-field).
func (c *CUC) Encode() ([]byte, error) {
	if err := c.Validate(); err != nil {
		return nil, err
	}

	pBytes, err := c.PField.Encode()
	if err != nil {
		return nil, err
	}

	tField := make([]byte, 0, c.CoarseBytes+c.FineBytes)

	// Encode coarse time (big-endian)
	for i := int(c.CoarseBytes) - 1; i >= 0; i-- {
		tField = append(tField, byte(c.CoarseTime>>(uint(i)*8)))
	}

	// Encode fine time (big-endian, most significant octets)
	for i := int(c.FineBytes) - 1; i >= 0; i-- {
		tField = append(tField, byte(c.FineTime>>(uint(i)*8)))
	}

	return append(pBytes, tField...), nil
}

// DecodeCUC parses a byte slice into a CUC time code.
// If epoch is zero-value, Level 1 (CCSDS epoch) is assumed.
func DecodeCUC(data []byte, epoch time.Time) (*CUC, error) {
	if len(data) < 2 {
		return nil, ErrDataTooShort
	}

	c := &CUC{}

	if err := c.PField.Decode(data); err != nil {
		return nil, err
	}

	id := c.PField.TimeCodeID
	if id != TimeCodeCUCLevel1 && id != TimeCodeCUCLevel2 {
		return nil, ErrInvalidTimeCodeID
	}

	// Extract basic octet counts from P-field detail bits (§3.2.2)
	// Bits 4-5: number of coarse octets minus one (0-3 → 1-4)
	// Bits 6-7: number of fine octets (0-3)
	c.CoarseBytes = ((c.PField.Detail >> 2) & 0x03) + 1
	c.FineBytes = c.PField.Detail & 0x03

	// Handle extension octets (§3.2.2 Octet 2)
	// Bits 1-2: additional coarse octets
	// Bits 3-5: additional fine octets
	// Bits 6-7: reserved
	if c.PField.Extension {
		addCoarse := (c.PField.ExtDetail >> 5) & 0x03
		addFine := (c.PField.ExtDetail >> 2) & 0x07
		c.CoarseBytes += addCoarse
		c.FineBytes += addFine
	}

	// Set epoch
	if id == TimeCodeCUCLevel2 {
		if epoch.IsZero() {
			return nil, ErrEpochRequired
		}
		c.Epoch = epoch
	} else {
		c.Epoch = CCSDSEpoch
	}

	// Parse T-field
	offset := c.PField.Size()
	tFieldLen := int(c.CoarseBytes + c.FineBytes)
	if len(data) < offset+tFieldLen {
		return nil, ErrDataTooShort
	}

	// Decode coarse time
	c.CoarseTime = 0
	for i := range int(c.CoarseBytes) {
		c.CoarseTime = (c.CoarseTime << 8) | uint64(data[offset+i])
	}
	offset += int(c.CoarseBytes)

	// Decode fine time
	c.FineTime = 0
	for i := range int(c.FineBytes) {
		c.FineTime = (c.FineTime << 8) | uint32(data[offset+i])
	}

	return c, c.Validate()
}

// Time converts the CUC time code to a Go time.Time value.
func (c *CUC) Time() time.Time {
	t := c.Epoch.Add(time.Duration(c.CoarseTime) * time.Second)
	if c.FineBytes > 0 {
		totalFineBits := uint(c.FineBytes) * 8
		fracNanos := (int64(c.FineTime) * int64(time.Second)) >> totalFineBits
		t = t.Add(time.Duration(fracNanos))
	}
	return t
}

// Validate checks that the CUC fields conform to CCSDS 301.0-B-4.
func (c *CUC) Validate() error {
	if c.CoarseBytes < 1 || c.CoarseBytes > 7 {
		return ErrInvalidCoarseOctets
	}
	if c.FineBytes > 6 {
		return ErrInvalidFineOctets
	}
	maxCoarse := uint64(1)<<(uint(c.CoarseBytes)*8) - 1
	if c.CoarseTime > maxCoarse {
		return ErrOverflow
	}
	return nil
}

// Humanize returns a human-readable representation of the CUC time code.
func (c *CUC) Humanize() string {
	level := "Level 1 (CCSDS epoch)"
	if c.PField.TimeCodeID == TimeCodeCUCLevel2 {
		level = "Level 2 (agency-defined epoch)"
	}
	return strings.Join([]string{
		"CUC Time Code:",
		"  " + level,
		"  Coarse Time: " + strconv.FormatUint(c.CoarseTime, 10) + " s",
		"  Fine Time: " + strconv.FormatUint(uint64(c.FineTime), 10),
		"  Coarse Octets: " + strconv.Itoa(int(c.CoarseBytes)),
		"  Fine Octets: " + strconv.Itoa(int(c.FineBytes)),
		"  Time: " + c.Time().UTC().Format(time.RFC3339Nano),
	}, "\n")
}

// buildPField constructs the P-field from the CUC configuration.
func (c *CUC) buildPField() error {
	// Determine Level
	id := TimeCodeCUCLevel1
	if c.Epoch != CCSDSEpoch {
		id = TimeCodeCUCLevel2
	}

	// Basic octets fit in first P-field octet
	basicCoarse := c.CoarseBytes
	basicFine := c.FineBytes
	needsExt := false

	if basicCoarse > 4 || basicFine > 3 {
		needsExt = true
		addCoarse := uint8(0)
		addFine := uint8(0)
		if basicCoarse > 4 {
			addCoarse = basicCoarse - 4
			basicCoarse = 4
		}
		if basicFine > 3 {
			addFine = basicFine - 3
			basicFine = 3
		}
		// Octet 2 (§3.2.2): bits 1-2 = additional coarse, bits 3-5 = additional fine, bits 6-7 = reserved
		c.PField = PField{
			Extension:  true,
			TimeCodeID: id,
			Detail:     ((basicCoarse - 1) << 2) | basicFine,
			ExtDetail:  (addCoarse << 5) | (addFine << 2),
		}
	}

	if !needsExt {
		c.PField = PField{
			Extension:  false,
			TimeCodeID: id,
			Detail:     ((basicCoarse - 1) << 2) | basicFine,
		}
	}

	return nil
}
