package bp

import "time"

// dtnEpochUnix is 2000-01-01T00:00:00Z as a Unix second count. The DTN epoch
// (RFC 9171 clause 4.2.6) starts there rather than at 1970.
const dtnEpochUnix = 946684800

// DTNTime counts milliseconds since the DTN epoch, 2000-01-01T00:00:00Z
// (RFC 9171 clause 4.2.6).
//
// Two things about it catch people out. It ignores leap seconds, so it is not
// a count of elapsed SI seconds and converting through UTC is the only correct
// route. And it is milliseconds, where Bundle Protocol version 6 counted
// seconds — a bundle whose timestamp is read with the wrong unit still parses,
// still round-trips inside one implementation, and is wrong by a factor of a
// thousand on the wire.
//
// Values pass 2^32 in ordinary use, which is why this is a uint64 and why
// RFC 9171 clause 4.2.6 warns implementers about it directly.
type DTNTime uint64

// DTNTimeUnknown is the value a node without a working clock sends. It means
// the creation time is not known, not that the epoch itself is meant
// (RFC 9171 clause 4.2.6). A bundle carrying it must also carry a Bundle Age
// block (clause 4.4.2).
const DTNTimeUnknown DTNTime = 0

// NewDTNTime converts a wall-clock time to DTN time, rounding down to the
// millisecond. Times before the DTN epoch cannot be represented and return
// DTNTimeUnknown with false.
func NewDTNTime(t time.Time) (DTNTime, bool) {
	ms := t.UTC().UnixMilli() - dtnEpochUnix*1000
	if ms < 0 {
		return DTNTimeUnknown, false
	}
	return DTNTime(ms), true
}

// Time converts back to a wall-clock time in UTC. The result for
// DTNTimeUnknown is the epoch itself, so check against DTNTimeUnknown first
// where the difference matters.
func (t DTNTime) Time() time.Time {
	return time.UnixMilli(int64(t) + dtnEpochUnix*1000).UTC()
}

// CreationTimestamp identifies a bundle together with its source node ID, and
// with the fragment fields when the bundle is a fragment
// (RFC 9171 clause 4.2.7).
type CreationTimestamp struct {
	// Time is when the transmission request arrived, or DTNTimeUnknown from a
	// node with no accurate clock.
	Time DTNTime
	// Sequence distinguishes bundles created within the same millisecond. A
	// node with no clock never resets it, since it is all that keeps two
	// bundles apart.
	Sequence uint64
}

// appendCreationTimestamp writes the two-item array of RFC 9171 clause 4.2.7.
func appendCreationTimestamp(dst []byte, ts CreationTimestamp) []byte {
	dst = appendArrayHeader(dst, 2)
	dst = appendUint(dst, uint64(ts.Time))
	return appendUint(dst, ts.Sequence)
}

// creationTimestamp reads a creation timestamp.
func (d *decoder) creationTimestamp() (CreationTimestamp, error) {
	n, indefinite, err := d.arrayHeader()
	if err != nil {
		return CreationTimestamp{}, err
	}
	if indefinite || n != 2 {
		return CreationTimestamp{}, ErrMalformedTimestamp
	}

	dtn, err := d.uint()
	if err != nil {
		return CreationTimestamp{}, err
	}
	seq, err := d.uint()
	if err != nil {
		return CreationTimestamp{}, err
	}
	return CreationTimestamp{Time: DTNTime(dtn), Sequence: seq}, nil
}
