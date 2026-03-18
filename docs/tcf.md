# Time Code Formats (TCF)

The `tcf` package implements CCSDS 301.0-B-4 Time Code Formats — the standard time encoding schemes used in spacecraft telemetry and command systems for timestamping data with configurable precision and epoch reference.

## Quick Start

```go
import "github.com/ravisuhag/astro/pkg/tcf"

// Encode current time as CUC (binary counter)
cuc, _ := tcf.NewCUC(time.Now(), tcf.WithCUCFineBytes(2))
encoded, _ := cuc.Encode()

// Decode back
decoded, _ := tcf.DecodeCUC(encoded, time.Time{})
fmt.Println(decoded.Time()) // Go time.Time
```

## Supported Formats

| Format | Description | Encoding | Use Case |
|--------|-------------|----------|----------|
| **CUC** | Unsegmented Time Code | Binary counter (seconds + fraction) | High-rate telemetry, onboard clocks |
| **CDS** | Day Segmented Time Code | Day + milliseconds + optional sub-ms | Ground systems, event logging |
| **CCS** | Calendar Segmented Time Code | BCD-encoded calendar fields | Human-readable binary timestamps |
| **ASCII** | Text Time Codes (Type A/B) | ISO 8601-derived strings | Logs, displays, interchange |

All binary formats share a common structure: **P-field** (preamble, 1–2 bytes) identifying the format, followed by a **T-field** (time data, variable length).

```
+----------------------+--------------------------+
| P-Field (Preamble)   | T-Field (Time Code)      |
| 1 or 2 octets        | Variable length           |
+----------------------+--------------------------+
```

## Epoch and TAI

```go
// CCSDS reference epoch: 1958-01-01T00:00:00 TAI
tcf.CCSDSEpoch // time.Time

// Current TAI-UTC offset (leap seconds, update when IERS announces new ones)
tcf.TAIUTCOffset // 37 (as of 2025)
```

**Level 1** time codes use `CCSDSEpoch`. **Level 2** time codes use an agency-defined custom epoch.

## CUC — Unsegmented Time Code

Binary counter split into coarse time (seconds since epoch) and fine time (binary fraction of a second).

```
+------------------+------------------+
| Coarse (1-4 oct) | Fine (0-3 oct)   |
+------------------+------------------+
```

**Fine time resolution:**

| Fine Octets | Resolution |
|-------------|------------|
| 0 | 1 s |
| 1 | ~3.9 ms (2^-8 s) |
| 2 | ~15.3 us (2^-16 s) |
| 3 | ~59.6 ns (2^-24 s) |

Up to 7 coarse and 6 fine octets with the P-field extension.

### Creating

```go
// Default: Level 1 (CCSDS epoch), 4 coarse octets, 0 fine octets
cuc, err := tcf.NewCUC(time.Now())

// With sub-second precision
cuc, err := tcf.NewCUC(time.Now(), tcf.WithCUCFineBytes(2))

// With custom coarse width
cuc, err := tcf.NewCUC(time.Now(), tcf.WithCUCCoarseBytes(2))

// Level 2 with agency-defined epoch
missionEpoch := time.Date(2000, 1, 1, 0, 0, 0, 0, time.UTC)
cuc, err := tcf.NewCUC(time.Now(),
    tcf.WithCUCEpoch(missionEpoch),
    tcf.WithCUCFineBytes(3),
)
```

### Encoding and Decoding

```go
// Encode to bytes (P-field + T-field)
encoded, err := cuc.Encode()

// Decode — pass zero time for Level 1, or the agency epoch for Level 2
decoded, err := tcf.DecodeCUC(encoded, time.Time{})

// Convert to Go time
t := decoded.Time()

// Debug output
fmt.Println(decoded.Humanize())
```

## CDS — Day Segmented Time Code

Day count since epoch plus milliseconds of day, with optional sub-millisecond precision.

```
+-------------+-----------------+----------------------+
| Day (16/24) | Milliseconds(32)| Sub-ms (0/16/32 bit) |
+-------------+-----------------+----------------------+
```

**Sub-millisecond precision:**

| Sub-ms Bytes | Resolution |
|--------------|------------|
| 0 | 1 ms |
| 2 | microseconds (0–999) |
| 4 | picoseconds (0–999999999) |

### Creating

```go
// Default: Level 1, 16-bit day, no sub-milliseconds
cds, err := tcf.NewCDS(time.Now())

// 24-bit day (supports 16M+ days) with microsecond precision
cds, err := tcf.NewCDS(time.Now(),
    tcf.WithCDSDayBytes(3),
    tcf.WithCDSSubmsBytes(2),
)

// Level 2 with custom epoch and picosecond precision
cds, err := tcf.NewCDS(time.Now(),
    tcf.WithCDSEpoch(missionEpoch),
    tcf.WithCDSSubmsBytes(4),
)
```

### Encoding and Decoding

```go
// Encode to bytes
encoded, err := cds.Encode()

// Decode — pass zero time for Level 1, or the agency epoch for Level 2
decoded, err := tcf.DecodeCDS(encoded, time.Time{})

// Convert to Go time
t := decoded.Time()

// Debug output
fmt.Println(decoded.Humanize())
```

## CCS — Calendar Segmented Time Code

Human-readable binary format using BCD-encoded calendar fields. Always Level 1 (UTC).

Two calendar variants:

**Day-of-Year variant** (default):
```
+----------+--------+------+------+------+------------------+
| Year(16) | DOY(16)| H(8) | M(8) | S(8) | Sub-s (0-6 oct) |
+----------+--------+------+------+------+------------------+
```

**Month/Day variant:**
```
+----------+------+-------+------+------+------+------------------+
| Year(16) | Mo(8)| Dom(8)| H(8) | M(8) | S(8) | Sub-s (0-6 oct) |
+----------+------+-------+------+------+------+------------------+
```

Sub-second precision: 0–6 octets, each containing 2 BCD digits, giving 10^-2 to 10^-12 second resolution. The Second field allows value 60 for leap seconds.

### Creating

```go
// Default: Day-of-Year variant, no sub-second precision
ccs, err := tcf.NewCCS(time.Now())

// Month/Day variant with centisecond precision
ccs, err := tcf.NewCCS(time.Now(),
    tcf.WithCCSMonthDay(),
    tcf.WithCCSSubSecBytes(1),
)

// Day-of-Year with high sub-second resolution
ccs, err := tcf.NewCCS(time.Now(), tcf.WithCCSSubSecBytes(3))
```

### Encoding and Decoding

```go
// Encode to bytes (BCD-encoded)
encoded, err := ccs.Encode()

// Decode — no epoch needed (CCS is always Level 1 UTC)
decoded, err := tcf.DecodeCCS(encoded)

// Convert to Go time
t := decoded.Time()

// Debug output
fmt.Println(decoded.Humanize())
```

## ASCII — Text Time Codes

Human-readable text formats derived from ISO 8601.

- **Type A** (calendar): `YYYY-MM-DDThh:mm:ss.dddZ`
- **Type B** (ordinal): `YYYY-DDDThh:mm:ss.dddZ`

### Creating and Using

```go
// Type A with default 3 fractional digits
ascii, err := tcf.NewASCIITime(tcf.ASCIITypeA)

// Type B with 6 fractional digits
ascii, err := tcf.NewASCIITime(tcf.ASCIITypeB, tcf.WithASCIIPrecision(6))

// Encode time to string
s, err := ascii.Encode(time.Now())
// → "2026-077T14:30:15.123456Z"

// Decode string back to time
t, err := ascii.Decode("2026-03-18T14:30:15.123Z")
```

The `Z` terminator is always appended on encode and is optional on decode.

## P-Field (Preamble)

The P-field is managed automatically by the format constructors. For advanced use cases, it can be inspected directly:

```go
// Inspect a decoded time code's P-field
cuc, _ := tcf.DecodeCUC(data, time.Time{})
fmt.Println(cuc.PField.TimeCodeID) // e.g., tcf.TimeCodeCUCLevel1
fmt.Println(cuc.PField.Extension)  // true if 2-byte P-field
fmt.Println(cuc.PField.Size())     // 1 or 2
```

**P-field layout:**
```
First octet:
+---+-------+-------------------+
| E | ID(3) | Format-specific(4)|
+---+-------+-------------------+

Second octet (if E=1):
+---+-------------------------------+
| 0 | Extension-specific (7 bits)   |
+---+-------------------------------+
```

**Time Code IDs:**

| Constant | Value | Format |
|----------|-------|--------|
| `TimeCodeCUCLevel1` | `0x01` | CUC with CCSDS epoch |
| `TimeCodeCUCLevel2` | `0x02` | CUC with agency-defined epoch |
| `TimeCodeCDS` | `0x04` | CDS (Level 1 or 2) |
| `TimeCodeCCS` | `0x05` | CCS (always Level 1, UTC) |

## Errors

All errors are exported package-level variables, suitable for use with `errors.Is`:

| Error | Meaning |
|-------|---------|
| `ErrDataTooShort` | Data too short to decode time code |
| `ErrInvalidPField` | P-field doesn't conform to CCSDS 301.0-B-4 |
| `ErrInvalidTimeCodeID` | Unrecognized time code identification |
| `ErrInvalidCoarseOctets` | Coarse time octets out of range (1–4 basic, up to 7 with extension) |
| `ErrInvalidFineOctets` | Fine time octets out of range (0–3 basic, up to 6 with extension) |
| `ErrInvalidDaySegment` | Day count out of range |
| `ErrInvalidMilliseconds` | Milliseconds-of-day outside 0–86399999 |
| `ErrInvalidCalendarTime` | Calendar field value out of range |
| `ErrInvalidASCIIFormat` | ASCII time string format mismatch |
| `ErrEpochRequired` | Agency-defined epoch required for Level 2 but not provided |
| `ErrOverflow` | Time value exceeds representable range for configured width |

## Reference

- [CCSDS 301.0-B-4](https://public.ccsds.org/Pubs/301x0b4e1.pdf) — Time Code Formats Blue Book
