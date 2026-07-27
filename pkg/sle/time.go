package sle

import (
	"encoding/binary"
	"fmt"
	"time"
)

// CCSDS time as SLE carries it.
//
// The RAF ASN.1 module (CCSDS 911.1-B-5 annex A2.1) defines TimeCCSDS as an
// eight-octet OCTET STRING and says the P-field is implicit:
//
//	2 octets: days since 1958/01/01
//	4 octets: milliseconds of the day
//	2 octets: microseconds of the millisecond
//
// pkg/tcf implements the same CCSDS Day Segmented time code, but its Encode
// always emits a P-field first. SLE wants the bare T-field, so the eight-octet
// form is built here rather than borrowed and trimmed.

// TimeCCSDSSize is the width of the SLE time field in octets.
const TimeCCSDSSize = 8

// TimeCCSDSPicoSize is the width of the picosecond-resolution variant.
const TimeCCSDSPicoSize = 10

// CCSDSEpoch is 1958-01-01 00:00:00 UTC, the epoch the day count runs from.
var CCSDSEpoch = time.Date(1958, 1, 1, 0, 0, 0, 0, time.UTC)

// Time is a CCSDS Day Segmented time as SLE encodes it.
type Time struct {
	// Days since the 1958 epoch.
	Days uint16
	// Milliseconds of the day.
	Milliseconds uint32
	// Microseconds of the millisecond. Zero when unused.
	Microseconds uint16
}

// NewTime converts a Go time to the SLE representation.
func NewTime(t time.Time) (Time, error) {
	elapsed := t.UTC().Sub(CCSDSEpoch)
	if elapsed < 0 {
		return Time{}, ErrDataTooShort
	}

	days := int64(elapsed / (24 * time.Hour))
	if days > 0xFFFF {
		return Time{}, ErrIntegerOverflow
	}
	rest := elapsed - time.Duration(days)*24*time.Hour

	ms := int64(rest / time.Millisecond)
	sub := rest - time.Duration(ms)*time.Millisecond

	return Time{
		Days:         uint16(days),
		Milliseconds: uint32(ms),
		Microseconds: uint16(sub / time.Microsecond),
	}, nil
}

// Time converts back to a Go time.
func (t Time) Time() time.Time {
	return CCSDSEpoch.
		AddDate(0, 0, int(t.Days)).
		Add(time.Duration(t.Milliseconds) * time.Millisecond).
		Add(time.Duration(t.Microseconds) * time.Microsecond)
}

// Encode serializes the eight-octet T-field.
func (t Time) Encode() []byte {
	out := make([]byte, TimeCCSDSSize)
	binary.BigEndian.PutUint16(out[0:2], t.Days)
	binary.BigEndian.PutUint32(out[2:6], t.Milliseconds)
	binary.BigEndian.PutUint16(out[6:8], t.Microseconds)
	return out
}

// DecodeTime parses an eight-octet T-field.
func DecodeTime(data []byte) (Time, error) {
	if len(data) < TimeCCSDSSize {
		return Time{}, ErrDataTooShort
	}
	return Time{
		Days:         binary.BigEndian.Uint16(data[0:2]),
		Milliseconds: binary.BigEndian.Uint32(data[2:6]),
		Microseconds: binary.BigEndian.Uint16(data[6:8]),
	}, nil
}

// Humanize returns a human-readable summary.
func (t Time) Humanize() string {
	return fmt.Sprintf("%s (day %d, %d ms, %d us)",
		t.Time().Format(time.RFC3339Nano), t.Days, t.Milliseconds, t.Microseconds)
}

// AppendTimeChoice writes an SLE Time CHOICE, taking the [0] ccsdsFormat
// alternative that the eight-octet form uses.
func AppendTimeChoice(dst []byte, t Time) []byte {
	return AppendElement(dst, ClassContext, false, 0, t.Encode())
}

// DecodeTimeChoice reads an SLE Time CHOICE.
//
// Alternative [0] is the eight-octet form and [1] the ten-octet picosecond
// one. This package reads both but keeps only microsecond resolution: the
// extra precision has nowhere to go in a Go time.Time at these magnitudes.
func DecodeTimeChoice(e *Element) (Time, error) {
	switch {
	case e.IsContext(0):
		return DecodeTime(e.Bytes)
	case e.IsContext(1):
		if len(e.Bytes) < TimeCCSDSPicoSize {
			return Time{}, ErrDataTooShort
		}
		// 2 octets days, 4 octets milliseconds, 4 octets picoseconds.
		picos := binary.BigEndian.Uint32(e.Bytes[6:10])
		return Time{
			Days:         binary.BigEndian.Uint16(e.Bytes[0:2]),
			Milliseconds: binary.BigEndian.Uint32(e.Bytes[2:6]),
			Microseconds: uint16(picos / 1_000_000),
		}, nil
	default:
		return Time{}, ErrInvalidTag
	}
}
