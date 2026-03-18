package tcf

import "time"

// CCSDS 301.0-B-4 Time Code Formats.
//
// +--------+--------+-------------------------------------------+
// | P-Field (Preamble)   | T-Field (Time Code)                  |
// | 1 or 2 octets        | Variable length                      |
// +--------+--------+-------------------------------------------+
//
// P-Field first octet:
// +---+-------+-------------------+
// | E | ID(3) | Format-specific(4)|
// +---+-------+-------------------+
//   E = Extension flag (0 = last octet, 1 = another octet follows)
//   ID = Time code identification
//
// Supported formats:
//   CUC  - CCSDS Unsegmented Time Code (binary counter)
//   CDS  - CCSDS Day Segmented Time Code (day + ms + optional sub-ms)
//   CCS  - CCSDS Calendar Segmented Time Code (BCD calendar fields)
//   ASCII - Text-based time codes (Type A and Type B)

// CCSDSEpoch is the CCSDS recommended epoch: 1958-01-01T00:00:00 TAI.
// This is used as the reference for Level 1 CUC and CDS time codes.
var CCSDSEpoch = time.Date(1958, 1, 1, 0, 0, 0, 0, time.UTC)

// TAIUTCOffset is the current TAI-UTC offset in seconds (leap seconds).
// As of 2025, TAI is 37 seconds ahead of UTC. Update this value when
// new leap seconds are announced by the IERS.
const TAIUTCOffset = 37

// Time code identification values (P-field bits 1-3) per Table B-3.
const (
	TimeCodeCUCLevel1 uint8 = 0x01 // 001: CUC with CCSDS epoch (Level 1)
	TimeCodeCUCLevel2 uint8 = 0x02 // 010: CUC with agency-defined epoch (Level 2)
	TimeCodeCDS       uint8 = 0x04 // 100: CDS (Level 1 or 2, determined by bit 4)
	TimeCodeCCS       uint8 = 0x05 // 101: CCS (always Level 1, UTC)
)
