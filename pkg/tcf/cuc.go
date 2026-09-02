package tcf

import (
	"strconv"
	"strings"
	"time"
)

// CUC represents a CCSDS Unsegmented Time Code per CCSDS 301.0-B-4 clause 3.2.
//
// The T-field is a single binary counter split into coarse time (seconds
// since epoch) and fine time (fractional seconds as binary fractions of a
// second).
//
//	+------------------+------------------+
//	| Coarse (1-4 oct) | Fine (0-3 oct)   |
//	| up to 7 extended | up to 10 extended|
//	+------------------+------------------+
//
// Fine time resolution is 2^-(8*n) seconds for n fine octets:
//
//	0 octets: 1 s
//	1 octet:  ~3.9 ms   (2^-8 s)
//	2 octets: ~15.3 µs  (2^-16 s)
//	3 octets: ~59.6 ns  (2^-24 s)
//	...
//	10 octets: 2^-80 s
//
// Because up to 10 fine octets (80 bits) are allowed, the fine counter is
// held in two fields: FineTime carries the most significant octets (up to
// 8), and FineTimeExt carries the remaining least significant octets (only
// used when FineBytes > 8).
//
// Time scale: with the CCSDS epoch (Level 1) the coarse count is true TAI
// seconds since 1958-01-01T00:00:00 TAI. NewCUC and Time apply the embedded
// leap-second table (see TAIUTCOffsetAt and the package documentation) when
// converting to and from Go's UTC-based time.Time. With an agency-defined
// epoch (Level 2) the count is purely arithmetic elapsed seconds and no
// leap-second correction is applied.
//
// Rounding: NewCUC truncates the fractional second toward zero to the
// configured fine-time resolution; it does not round to nearest. Time
// likewise truncates sub-nanosecond fine time (including all of FineTimeExt,
// whose resolution is below 2^-64 s) toward zero.
type CUC struct {
	PField      PField    // Preamble field
	CoarseTime  uint64    // Seconds since epoch (TAI seconds for Level 1)
	FineTime    uint64    // Most significant fine octets (up to 8)
	FineTimeExt uint16    // Least significant fine octets 9-10 (FineBytes > 8 only)
	CoarseBytes uint8     // Number of coarse time octets (1-4, up to 7 with extension)
	FineBytes   uint8     // Number of fine time octets (0-3, up to 10 with extension)
	Epoch       time.Time // Reference epoch (CCSDSEpoch for Level 1)
}

// CUCOption configures a CUC time code.
type CUCOption func(*CUC) error

// WithCUCFineBytes sets the number of fine time octets (0-3 basic, up to 10
// with extension, per clause 3.2.2).
func WithCUCFineBytes(n uint8) CUCOption {
	return func(c *CUC) error {
		if n > 10 {
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
//
// For Level 1 the TAI-UTC offset in effect at t is added so that the coarse
// count is true TAI seconds since the 1958 TAI epoch; see the package
// documentation. Fractional seconds are truncated toward zero to the
// configured fine-time resolution.
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

	// Split arithmetic on Unix seconds avoids the ~292-year range limit of
	// time.Duration.
	secs, nanos := epochDelta(t, c.Epoch)

	// Level 1 counts TAI seconds: add the leap seconds accumulated between
	// the UTC and TAI scales at the encoded instant. Level 2 stays purely
	// arithmetic.
	if isCCSDSEpoch(c.Epoch) {
		secs += TAIUTCOffsetAt(t)
	}

	if secs < 0 {
		return nil, ErrOverflow
	}
	c.CoarseTime = uint64(secs)

	if c.FineBytes > 0 {
		// Convert the fractional part to a binary fraction with the
		// configured precision (truncating, not rounding). Successive
		// doubling avoids int64 overflow for large bit counts.
		fracNs := nanos
		secNs := int64(time.Second)
		totalBits := int(c.FineBytes) * 8
		hiBits := totalBits
		if hiBits > 64 {
			hiBits = 64
		}
		var hi uint64
		var lo uint16
		for i := range totalBits {
			fracNs *= 2
			var bit uint64
			if fracNs >= secNs {
				bit = 1
				fracNs -= secNs
			}
			if i < hiBits {
				hi = (hi << 1) | bit
			} else {
				lo = (lo << 1) | uint16(bit)
			}
		}
		c.FineTime = hi
		c.FineTimeExt = lo
	}

	// Check coarse time fits in the configured width
	maxCoarse := maxCoarseFor(c.CoarseBytes)
	if c.CoarseTime > maxCoarse {
		return nil, ErrOverflow
	}

	// Build P-field
	if err := c.buildPField(); err != nil {
		return nil, err
	}

	return c, nil
}

// maxCoarseFor returns the largest coarse count representable in n octets.
func maxCoarseFor(n uint8) uint64 {
	if n >= 8 {
		return ^uint64(0)
	}
	return uint64(1)<<(uint(n)*8) - 1
}

// fineSplit returns how many fine octets live in FineTime (hi) and
// FineTimeExt (lo) for the configured FineBytes.
func (c *CUC) fineSplit() (hiOctets, loOctets int) {
	hiOctets = int(c.FineBytes)
	if hiOctets > 8 {
		hiOctets = 8
	}
	return hiOctets, int(c.FineBytes) - hiOctets
}

// Encode serializes the CUC time code into bytes (P-field + T-field).
func (c *CUC) Encode() ([]byte, error) {
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
func (c *CUC) EncodeTField() ([]byte, error) {
	if err := c.Validate(); err != nil {
		return nil, err
	}

	tField := make([]byte, 0, int(c.CoarseBytes)+int(c.FineBytes))

	// Encode coarse time (big-endian)
	for i := int(c.CoarseBytes) - 1; i >= 0; i-- {
		tField = append(tField, byte(c.CoarseTime>>(uint(i)*8)))
	}

	// Encode fine time (big-endian): FineTime holds the most significant
	// octets, FineTimeExt the least significant (FineBytes > 8 only).
	hiOctets, loOctets := c.fineSplit()
	for i := hiOctets - 1; i >= 0; i-- {
		tField = append(tField, byte(c.FineTime>>(uint(i)*8)))
	}
	for i := loOctets - 1; i >= 0; i-- {
		tField = append(tField, byte(c.FineTimeExt>>(uint(i)*8)))
	}

	return tField, nil
}

// DecodeCUC parses a byte slice into a CUC time code (P-field + T-field).
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

	// Extract basic octet counts from P-field detail bits (clause 3.2.2)
	// Bits 4-5: number of coarse octets minus one (0-3 -> 1-4)
	// Bits 6-7: number of fine octets (0-3)
	c.CoarseBytes = ((c.PField.Detail >> 2) & 0x03) + 1
	c.FineBytes = c.PField.Detail & 0x03

	// Handle extension octet (clause 3.2.2 Octet 2)
	// Bits 1-2: additional coarse octets
	// Bits 3-5: additional fine octets
	// Bits 6-7: reserved, must be zero
	if c.PField.Extension {
		if c.PField.ExtDetail&0x03 != 0 {
			return nil, ErrInvalidPField
		}
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

	if err := c.decodeTField(data[c.PField.Size():]); err != nil {
		return nil, err
	}

	return c, c.Validate()
}

// DecodeCUCTField parses a T-field-only (implicit P-field) CUC time code.
// The format parameters (coarse and fine octet counts and the epoch) must
// be supplied by the caller, as they are in contexts like Space Packet
// secondary headers where no P-field is transmitted.
//
// If epoch is zero-value, Level 1 (CCSDS epoch) is assumed; otherwise the
// code is treated as Level 2 with the given agency-defined epoch.
func DecodeCUCTField(data []byte, coarseBytes, fineBytes uint8, epoch time.Time) (*CUC, error) {
	if coarseBytes < 1 || coarseBytes > 7 {
		return nil, ErrInvalidCoarseOctets
	}
	if fineBytes > 10 {
		return nil, ErrInvalidFineOctets
	}

	c := &CUC{
		CoarseBytes: coarseBytes,
		FineBytes:   fineBytes,
		Epoch:       epoch,
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

// decodeTField parses the coarse and fine counters from tf, which must start
// at the first T-field octet.
func (c *CUC) decodeTField(tf []byte) error {
	tFieldLen := int(c.CoarseBytes) + int(c.FineBytes)
	if len(tf) < tFieldLen {
		return ErrDataTooShort
	}

	// Decode coarse time
	c.CoarseTime = 0
	offset := 0
	for range int(c.CoarseBytes) {
		c.CoarseTime = (c.CoarseTime << 8) | uint64(tf[offset])
		offset++
	}

	// Decode fine time: most significant octets into FineTime, remaining
	// least significant octets (9-10) into FineTimeExt.
	c.FineTime = 0
	c.FineTimeExt = 0
	hiOctets, loOctets := c.fineSplit()
	for range hiOctets {
		c.FineTime = (c.FineTime << 8) | uint64(tf[offset])
		offset++
	}
	for range loOctets {
		c.FineTimeExt = (c.FineTimeExt << 8) | uint16(tf[offset])
		offset++
	}

	return nil
}

// Time converts the CUC time code to a Go time.Time value in UTC.
//
// For Level 1 codes the coarse count is TAI seconds since the 1958 TAI
// epoch; the TAI-UTC offset in effect at the decoded instant is subtracted
// (see the package documentation, including the treatment of pre-1972 times
// and of instants inside an inserted leap second). Level 2 codes are
// converted arithmetically.
//
// Fine time below one nanosecond is truncated toward zero: Go's time.Time
// carries nanoseconds, so only the most significant fine bits contribute,
// and FineTimeExt (resolution below 2^-64 s) never does.
func (c *CUC) Time() time.Time {
	secs := int64(c.CoarseTime)
	if isCCSDSEpoch(c.Epoch) {
		secs -= taiOffsetAtTAISeconds(c.CoarseTime)
	}

	var fracNs int64
	if c.FineBytes > 0 {
		// Reconstruct fractional nanoseconds using successive halving.
		hiOctets, _ := c.fineSplit()
		totalBits := hiOctets * 8
		secNs := int64(time.Second)
		for i := range totalBits {
			bit := (c.FineTime >> uint64(totalBits-1-i)) & 1
			secNs /= 2
			if bit == 1 {
				fracNs += secNs
			}
		}
	}

	// time.Unix-based split arithmetic: a 7-octet coarse count (~2^56 s)
	// would overflow time.Duration, but fits comfortably in int64 seconds.
	return time.Unix(c.Epoch.Unix()+secs, int64(c.Epoch.Nanosecond())+fracNs).UTC()
}

// Validate checks that the CUC fields conform to CCSDS 301.0-B-4.
func (c *CUC) Validate() error {
	if c.CoarseBytes < 1 || c.CoarseBytes > 7 {
		return ErrInvalidCoarseOctets
	}
	if c.FineBytes > 10 {
		return ErrInvalidFineOctets
	}
	if c.CoarseTime > maxCoarseFor(c.CoarseBytes) {
		return ErrOverflow
	}
	hiOctets, loOctets := c.fineSplit()
	if hiOctets < 8 {
		if c.FineTime > uint64(1)<<(uint(hiOctets)*8)-1 {
			return ErrOverflow
		}
	}
	switch loOctets {
	case 0:
		if c.FineTimeExt != 0 {
			return ErrOverflow
		}
	case 1:
		if c.FineTimeExt > 0xFF {
			return ErrOverflow
		}
	}
	return nil
}

// Humanize returns a human-readable representation of the CUC time code.
func (c *CUC) Humanize() string {
	level := "Level 1 (CCSDS epoch)"
	if c.PField.TimeCodeID == TimeCodeCUCLevel2 {
		level = "Level 2 (agency-defined epoch)"
	}
	lines := []string{
		"CUC Time Code:",
		"  " + level,
		"  Coarse Time: " + strconv.FormatUint(c.CoarseTime, 10) + " s",
		"  Fine Time: " + strconv.FormatUint(c.FineTime, 10),
	}
	if _, loOctets := c.fineSplit(); loOctets > 0 {
		lines = append(lines, "  Fine Time Ext: "+strconv.FormatUint(uint64(c.FineTimeExt), 10))
	}
	lines = append(lines,
		"  Coarse Octets: "+strconv.Itoa(int(c.CoarseBytes)),
		"  Fine Octets: "+strconv.Itoa(int(c.FineBytes)),
		"  Time: "+c.Time().UTC().Format(time.RFC3339Nano),
	)
	return strings.Join(lines, "\n")
}

// buildPField constructs the P-field from the CUC configuration.
func (c *CUC) buildPField() error {
	// Determine Level
	id := TimeCodeCUCLevel1
	if !isCCSDSEpoch(c.Epoch) {
		id = TimeCodeCUCLevel2
	}

	// Basic octets fit in first P-field octet
	basicCoarse := c.CoarseBytes
	basicFine := c.FineBytes

	if basicCoarse > 4 || basicFine > 3 {
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
		// Octet 2 (clause 3.2.2): bits 1-2 = additional coarse, bits 3-5 = additional fine, bits 6-7 = reserved
		c.PField = PField{
			Extension:  true,
			TimeCodeID: id,
			Detail:     ((basicCoarse - 1) << 2) | basicFine,
			ExtDetail:  (addCoarse << 5) | (addFine << 2),
		}
		return nil
	}

	c.PField = PField{
		Extension:  false,
		TimeCodeID: id,
		Detail:     ((basicCoarse - 1) << 2) | basicFine,
	}

	return nil
}
