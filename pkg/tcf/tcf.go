// Package tcf implements CCSDS Time Code Formats per CCSDS 301.0-B-4.
//
// A time code is a P-field (preamble, 1 or 2 octets) followed by a T-field
// (the time value):
//
//	+----------------------+--------------------------------------+
//	| P-Field (Preamble)   | T-Field (Time Code)                  |
//	| 1 or 2 octets        | Variable length                      |
//	+----------------------+--------------------------------------+
//
// P-field first octet:
//
//	+---+-------+-------------------+
//	| E | ID(3) | Format-specific(4)|
//	+---+-------+-------------------+
//	  E = Extension flag (0 = last octet, 1 = another octet follows)
//	  ID = Time code identification
//
// Supported formats:
//
//	CUC   - CCSDS Unsegmented Time Code (binary counter)
//	CDS   - CCSDS Day Segmented Time Code (day + ms + optional sub-ms)
//	CCS   - CCSDS Calendar Segmented Time Code (BCD calendar fields)
//	ASCII - Text-based time codes (Type A and Type B)
//
// Each binary format also has T-field-only ("implicit P-field") APIs (
// EncodeTField and DecodeCUCTField / DecodeCDSTField / DecodeCCSTField) for
// contexts such as Space Packet secondary headers where the format is agreed
// out of band and no P-field is transmitted.
//
// # TAI, UTC, and leap seconds
//
// The CCSDS recommended (Level 1) epoch is 1958-01-01T00:00:00 on the TAI
// time scale. TAI is continuous; UTC inserts leap seconds, so the TAI-UTC
// offset has grown from 10 s (1972) to 37 s (since 2017-01-01).
//
// CUC Level 1 codes count true TAI seconds. This package embeds the full
// historical table of integer TAI-UTC offsets (see TAIUTCOffsetAt) and
// applies it automatically:
//
//   - Encoding (NewCUC with the CCSDS epoch): the coarse count is the UTC
//     elapsed seconds since 1958-01-01 plus the TAI-UTC offset in effect at
//     the encoded instant.
//   - Decoding (CUC.Time for Level 1): the offset in effect at the decoded
//     instant is subtracted again, yielding UTC.
//
// Boundary behavior, by design:
//
//   - Instants before 1972-01-01 UTC use an offset of 0. Between 1958 and
//     1972 UTC used fractional "rubber-second" adjustments that have no
//     integer representation; this package treats TAI and UTC as identical
//     in that era. Consequently TAI second counts that fall inside the 10 s
//     step at 1972-01-01 do not round-trip exactly.
//   - A TAI instant that falls inside an inserted leap second (UTC 23:59:60)
//     is reported by CUC.Time as the following 00:00:00 UTC, because Go's
//     time.Time cannot represent second 60.
//
// CUC Level 2 (agency-defined epoch) codes are purely arithmetic: the coarse
// count is the elapsed seconds between the epoch and the instant with no
// leap-second correction in either direction.
//
// CDS codes are day/millisecond arithmetic against their epoch in both
// levels; no leap-second table is applied. See the CDS type documentation
// for leap-second-day behavior. CCS and ASCII codes carry UTC calendar
// fields directly.
package tcf

import "time"

// CCSDSEpoch is the CCSDS recommended epoch: 1958-01-01T00:00:00 TAI.
// It is the reference for Level 1 CUC and CDS time codes.
//
// The value is expressed here as a Go time.Time in UTC. Leap-second
// corrections between the TAI and UTC scales are applied by the CUC Level 1
// encode/decode paths (see the package documentation); CDS treats the epoch
// arithmetically.
var CCSDSEpoch = time.Date(1958, 1, 1, 0, 0, 0, 0, time.UTC)

// Time code identification values (P-field bits 1-3) per Table B-3.
const (
	TimeCodeCUCLevel1 uint8 = 0x01 // 001: CUC with CCSDS epoch (Level 1)
	TimeCodeCUCLevel2 uint8 = 0x02 // 010: CUC with agency-defined epoch (Level 2)
	TimeCodeCDS       uint8 = 0x04 // 100: CDS (Level 1 or 2, determined by bit 4)
	TimeCodeCCS       uint8 = 0x05 // 101: CCS (always Level 1, UTC)
)

// isCCSDSEpoch reports whether t is the CCSDS 1958 epoch. The comparison
// strips any monotonic clock reading and ignores the location, so epochs
// constructed in different zones (or read from a wall clock) compare
// correctly.
func isCCSDSEpoch(t time.Time) bool {
	return t.Round(0).Equal(CCSDSEpoch)
}

// epochDelta returns the whole seconds and remaining nanoseconds (0..1e9-1)
// from epoch to t using split integer arithmetic, avoiding the ~292-year
// range limit of time.Duration.
func epochDelta(t, epoch time.Time) (secs int64, nanos int64) {
	secs = t.Unix() - epoch.Unix()
	nanos = int64(t.Nanosecond()) - int64(epoch.Nanosecond())
	if nanos < 0 {
		secs--
		nanos += int64(time.Second)
	}
	return secs, nanos
}
