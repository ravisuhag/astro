---
title: Time Code Formats
short: TCF
description: CCSDS 301.0-B-4 — how spacecraft timestamps are encoded, and what leap seconds do to them.
order: 60
---

> **CCSDS 301.0-B-4** | [Blue Book](https://public.ccsds.org/Pubs/301x0b4e1.pdf) | [`pkg/tcf`](https://github.com/ravisuhag/astro/tree/main/pkg/tcf) | [`astro time`](/cli/time)

When a sensor reading, an image, or a command acknowledgement needs a timestamp, this is how it gets written. The standard gives four formats with different trade-offs between size, precision, and how easy they are to read.

Time codes turn up inside [Space Packet](/protocols/transport/spp) secondary headers, [TM frame](/protocols/data-link/tmdl) secondary headers, onboard event logs, and ground metadata.

Most of what is hard here is not the encoding. It is [leap seconds](#tai-utc-and-leap-seconds).

## Scope

**Implemented.** All four formats — CUC, CDS, CCS, and ASCII — at both epoch levels where the format allows, with the full P-field, and with bare T-field encode and decode for streams that carry no preamble.

**Included.** The complete integer TAI-UTC leap second table, applied automatically for CUC Level 1. `TAIUTCOffsetAt(t)` exposes it.

**For the Go API** — constructors, options, and per-format detail — see the [API page](/protocols/mission/tcf).

## Field map: the P-field

Every binary time code is a **P-field** (preamble, 1 or 2 octets) followed by a **T-field** (the timestamp). The P-field is self-describing: a decoder reads it and knows the format, the precision, and the epoch level.

```
First octet:
┌───┬───────────┬───────────────────────┐
│ E │  ID (3b)  │  Format-specific (4b) │
└───┴───────────┴───────────────────────┘

Second octet (only when E = 1):
┌───┬───────────────────────────────────┐
│ 0 │     Extension-specific (7b)       │
└───┴───────────────────────────────────┘
```

| ID | Binary | Format | Go |
|---|---|---|---|
| 1 | `001` | CUC Level 1 — unsegmented, CCSDS epoch | `tcf.CUC` |
| 2 | `010` | CUC Level 2 — unsegmented, agency epoch | `tcf.CUC` |
| 4 | `100` | CDS — day segmented, either level | `tcf.CDS` |
| 5 | `101` | CCS — calendar segmented, always Level 1 UTC | `tcf.CCS` |

**Level 1** means the CCSDS epoch, 1958-01-01T00:00:00 TAI. Anyone can decode it. **Level 2** means a mission-defined epoch, and the decoder has to be told which one out of band.

## Choosing a format

| | CUC | CDS | CCS | ASCII |
|---|---|---|---|---|
| Size | 2-19 B | 7-12 B | 8-14 B | 20-30+ chars |
| Human-readable | No | Partly | Yes (BCD) | Yes |
| Epoch | Level 1 or 2 | Level 1 or 2 | Level 1 only (UTC) | UTC |
| Precision | 1 s to 2^-80 s | 1 ms to 1 ps | 1 s to 1 ps | 1 s to 1 ns |
| Hardware cost | Trivial — it is a counter | Moderate | High (BCD) | N/A |

Rules of thumb: **CUC** if it runs on a spacecraft processor. **CDS** if it runs on the ground and you want to debug it. **CCS** if it has to be calendar-readable while still binary. **ASCII Type A** (calendar) or **Type B** (ordinal) if it is text.

## Gotchas

**Many real streams carry no P-field at all.** A Space Packet secondary header agrees the format once in mission documentation and then sends bare T-fields forever. Use `EncodeTField()` to write one, and `DecodeCUCTField` / `DecodeCDSTField` / `DecodeCCSTField` to read one — you supply the octet counts, variant, and epoch yourself.

**CUC Level 1 and Level 2 treat leap seconds differently, on purpose.** Level 1 is true TAI, so Astro applies the offset. Level 2 is pure arithmetic with no correction at all. Mixing up which one your mission uses shifts every timestamp by 37 seconds.

**CCS second 60 is real.** It encodes the leap second itself. `Time()` normalizes it, because Go's `time.Time` cannot represent it.

**CDS does not touch the leap second table.** Its day and millisecond fields are a UTC day and time-of-day label, counted arithmetically. That is the standard's model, not a shortcut.

## TAI, UTC, and leap seconds

This is where time codes actually go wrong.

**TAI** is continuous atomic time. No adjustments, ever. **UTC** is civil time, kept within 0.9 seconds of Earth's rotation by inserting leap seconds every year or three. During one, the clock reads `23:59:59 -> 23:59:60 -> 00:00:00`.

Since 1972 the offset has been a whole number of seconds, growing from 10 to **37** at 2017-01-01, where it still stands. The CCSDS epoch of 1958 predates all of it.

### What Astro does with each format

The full table of integer TAI-UTC offsets is embedded in the package. They are historical facts and cannot change.

- **CUC Level 1** — the coarse count is **true TAI seconds**. `NewCUC` adds the offset in effect at that instant and `Time()` subtracts it again. A UTC instant round-trips exactly, and the count on the wire matches what real TAI-referenced hardware produces.
- **CUC Level 2** — **purely arithmetic**. Elapsed seconds between your epoch and the instant, no correction either way. If your mission epoch is itself TAI-referenced, apply your own convention on top.
- **CDS**, both levels — purely arithmetic day and millisecond counting. No table applied.
- **CCS** — always UTC calendar fields.
- **ASCII** — always UTC, marked by the `Z` suffix. Second 60 is accepted only at 23:59:60.

### Edge cases, by design

**Before 1972**, UTC used fractional "rubber second" adjustments with no integer representation. Astro treats TAI and UTC as identical in that era, offset 0. TAI counts landing inside the 10-second step at 1972-01-01 do not round-trip exactly.

**Inside a leap second**, a CUC Level 1 count that lands on UTC 23:59:60 decodes to the following 00:00:00. Go's `time.Time` has no way to say 60.

**Future leap seconds** are not predictable. The table ends at 37 s and none has been announced since. If the IERS announces one, `pkg/tcf/leapseconds.go` needs a one-line addition.

**Practical advice.** Store TAI or mission elapsed time onboard. Convert to UTC only at the ground system boundary, where the leap second table is available and someone is keeping it current.

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

All binary formats share a common structure: **P-field** (preamble, 1-2 bytes) identifying the format, followed by a **T-field** (time data, variable length).

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
tcf.TAIUTCOffsetAt(t) // 37 for any t after 2017-01-01
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
| 2 | microseconds (0-999) |
| 4 | picoseconds (0-999999999) |

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

Sub-second precision: 0-6 octets, each containing 2 BCD digits, giving 10^-2 to 10^-12 second resolution. The Second field allows value 60 for leap seconds.

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
// -> "2026-077T14:30:15.123456Z"

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
| `ErrInvalidCoarseOctets` | Coarse time octets out of range (1-4 basic, up to 7 with extension) |
| `ErrInvalidFineOctets` | Fine time octets out of range (0-3 basic, up to 6 with extension) |
| `ErrInvalidDaySegment` | Day count out of range |
| `ErrInvalidMilliseconds` | Milliseconds-of-day outside 0-86399999 |
| `ErrInvalidCalendarTime` | Calendar field value out of range |
| `ErrInvalidASCIIFormat` | ASCII time string format mismatch |
| `ErrEpochRequired` | Agency-defined epoch required for Level 2 but not provided |
| `ErrOverflow` | Time value exceeds representable range for configured width |

## Notes

Commentary, not sourced from the standard.

**Why four formats instead of one?** They serve genuinely different consumers. A spacecraft counter wants CUC because incrementing a binary number is free in hardware. A human reading an event log wants CCS or ASCII. Forcing either to use the other's format costs real money in one case and real debugging time in the other.

**Why is the P-field optional in practice?** It is 1-2 bytes on every timestamp, and a mission with a fixed format already knows what it is. On a link where a housekeeping packet is 30 bytes, dropping the preamble is a measurable win.

**Why did CCSDS pick a 1958 epoch?** It predates the standard by decades and lines up with the start of the atomic time record. Choosing it meant no mission would ever need a negative timestamp.

## Reference

- [CCSDS 301.0-B-4](https://public.ccsds.org/Pubs/301x0b4e1.pdf) — Time Code Formats (Blue Book)
- [CCSDS 301.0-G-1](https://public.ccsds.org/Pubs/301x0g1.pdf) — Time Code Formats Summary (Green Book)
- [CLI](/cli/time) | [Conformance](/conformance/tcf) | [The stack](/docs/start/concepts)