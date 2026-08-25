# Time Code Formats

> CCSDS 301.0-B-4 — Time Code Formats

## Overview

Time Code Formats define how timestamps are encoded in spacecraft telemetry, telecommand, and onboard data systems. When a sensor reading, a star tracker image, or a command acknowledgment needs a timestamp, that timestamp is encoded using one of the CCSDS time code formats.

Getting time right in space systems is deceptively complex. Spacecraft clocks drift. Light-speed delays between Earth and a spacecraft can range from milliseconds (LEO) to hours (deep space). Different subsystems need different precision — a housekeeping temperature sensor sampled once per second does not need nanosecond timestamps, but a science instrument correlating events across multiple spacecraft does. The CCSDS time code formats address this by providing a family of encodings with configurable precision and compact binary representations.

### Where Time Codes Are Used

```
┌─────────────────────────────────────┐
│  Application Data (sensor readings, │
│  images, events, commands)          │
│  → "When was this data acquired?"   │
│  → Time code embedded in packet     │
│    secondary header or data field   │
├─────────────────────────────────────┤
│  Space Packet Protocol              │
│  → Packets carry timestamped data   │
├─────────────────────────────────────┤
│  TM Space Data Link Protocol        │
│  → Frame secondary header may       │
│    carry a time code                │
├─────────────────────────────────────┤
│  Onboard Time Management            │
│  → Spacecraft clock generates       │
│    time codes for correlation       │
└─────────────────────────────────────┘
```

Time codes appear at multiple levels: inside packet secondary headers, in Transfer Frame secondary headers, in onboard event logs, and in ground system metadata. The CCSDS standard ensures all these timestamps can be unambiguously interpreted by any system that knows the format.

### Key Concepts

**Epoch**: The reference point from which time is measured. CCSDS defines a standard epoch (1958-01-01 TAI) but also allows missions to define their own.

**TAI vs UTC**: The standard epoch is in **TAI (International Atomic Time)**, which does not have leap seconds. UTC periodically adds leap seconds to stay synchronized with Earth's rotation. Since 2017-01-01, TAI is 37 seconds ahead of UTC. This distinction matters when converting between onboard time (often TAI-based) and ground time (often UTC-based). This library embeds the full leap-second table and applies it automatically for CUC Level 1 — see [TAI, UTC, and Leap Seconds](#tai-utc-and-leap-seconds).

**P-field and T-field**: All binary time codes consist of a **P-field** (preamble) that describes the format, followed by a **T-field** (time data) that contains the actual timestamp. The P-field is self-describing — a decoder can determine the format, precision, and epoch level by reading 1–2 bytes.

## P-Field Structure

The P-field (Preamble Field) is 1 or 2 bytes and identifies everything a decoder needs to interpret the T-field:

```
First octet:
┌───┬───────────┬───────────────────────┐
│ E │  ID (3b)  │  Format-specific (4b) │
└───┴───────────┴───────────────────────┘
 bit 0   bits 1-3       bits 4-7

Second octet (if E=1):
┌───┬───────────────────────────────────┐
│ 0 │     Extension-specific (7b)       │
└───┴───────────────────────────────────┘
```

**Extension flag (E)**: When set, a second P-field octet follows, providing additional configuration (e.g., extended precision for CUC).

**Time Code ID (3 bits)**: Identifies the format:

| ID | Binary | Format | Description |
|----|--------|--------|-------------|
| 1 | `001` | CUC Level 1 | Unsegmented, CCSDS epoch |
| 2 | `010` | CUC Level 2 | Unsegmented, agency-defined epoch |
| 4 | `100` | CDS | Day Segmented (Level 1 or 2) |
| 5 | `101` | CCS | Calendar Segmented (always Level 1 UTC) |

**Format-specific bits (4 bits)**: Meaning depends on the time code type — they encode precision levels, epoch choice, and variant selection.

### Level 1 vs Level 2

- **Level 1**: Uses the CCSDS standard epoch (1958-01-01T00:00:00 TAI). Any system that knows the format can decode the absolute time without additional information.
- **Level 2**: Uses a mission-defined epoch. The decoder must know which epoch was used — this information is communicated out-of-band (in mission documentation, database, etc.). Level 2 is common for missions where the CCSDS epoch would cause overflow or where a mission-specific epoch is more natural.

### Implicit P-Field

Many real streams do not transmit the P-field at all: Space Packet secondary headers, for example, agree on the time format once in mission documentation and then carry bare T-fields. The library supports this directly — `EncodeTField()` on each code type emits just the T-field, and `DecodeCUCTField`, `DecodeCDSTField`, and `DecodeCCSTField` parse one given the out-of-band format parameters (octet counts, variant, epoch).

## CUC — CCSDS Unsegmented Time Code

The simplest and most compact binary format. CUC represents time as a single binary counter split into two parts:

```
┌──────────────────────┬──────────────────────┐
│   Coarse Time        │     Fine Time        │
│  (seconds since      │  (binary fraction    │
│   epoch)             │   of a second)       │
│  1-4 octets (basic)  │  0-3 octets (basic)  │
│  up to 7 (extended)  │  up to 10 (extended) │
└──────────────────────┴──────────────────────┘
```

### How It Works

**Coarse time** is simply the integer number of seconds elapsed since the epoch. With 4 octets (32 bits), this covers approximately 136 years — enough for most missions using the 1958 epoch.

**Fine time** represents the fractional second as a **binary fraction**. Each additional octet divides the second into finer increments by powers of 256:

| Fine Octets | Divisions | Resolution | Practical Meaning |
|-------------|-----------|------------|-------------------|
| 0 | 1 | 1 second | Housekeeping data |
| 1 | 256 | ~3.9 ms | Low-rate telemetry |
| 2 | 65,536 | ~15.3 us | Medium-rate instruments |
| 3 | 16,777,216 | ~59.6 ns | High-rate science, timing |
| ... | ... | ... | ... |
| 10 | 2^80 | 2^-80 s | Maximum the spec allows |

The binary fraction approach means fine time is computed as:

```
fractional_seconds = fine_time_value / (2 ^ (fine_octets × 8))
```

For example, with 2 fine octets: a fine time value of 32,768 represents 32768/65536 = 0.5 seconds.

Two practical notes on how this library handles fine time:

- **Truncation, not rounding.** When encoding, the fractional second is cut off (truncated toward zero) at the configured resolution. It is never rounded up. With 1 fine octet, 0.9999999 s encodes as fine value 255, not 256. The same applies on decode: fine bits below one nanosecond are dropped, because Go's `time.Time` stops at nanoseconds.
- **More than 8 fine octets.** The spec allows up to 10 fine octets (80 bits), which is wider than a 64-bit integer. The `CUC` struct splits the counter: `FineTime` holds the most significant 8 octets and `FineTimeExt` holds octets 9-10. Decode and re-encode preserve all 10 octets exactly, even though the sub-2^-64 precision is far below what `time.Time` can express.

### P-Field Detail Bits

```
Bits 4-5: (coarse_octets - 1)    → 0-3 maps to 1-4 octets
Bits 6-7: fine_octets             → 0-3

Extension octet (if needed):
Bit 0:    further extension flag (must be 0; no third octet is defined)
Bits 1-2: additional coarse octets (0-3, total up to 7)
Bits 3-5: additional fine octets (0-7, total up to 10)
Bits 6-7: reserved (must be 0)
```

The decoder rejects a set further-extension bit and any set reserved bits with an error instead of guessing at the T-field layout.

### Why CUC?

CUC is the format of choice for **onboard spacecraft clocks**. Its advantages:
- Extremely compact (as few as 2 bytes for a timestamp with 1-second resolution)
- Simple hardware implementation — just a binary counter
- No calendar calculations needed onboard
- Monotonically increasing — no ambiguity from leap seconds, daylight saving, or calendar irregularities

The trade-off: CUC timestamps are not human-readable without conversion.

### Example

A CUC Level 1 timestamp with 4 coarse + 2 fine octets at time 2024-01-15T12:30:00.500 UTC (generated with `tcf.NewCUC(t, tcf.WithCUCFineBytes(2))`):

```
UTC seconds from 1958-01-01 to 2024-01-15T12:30:00: 2,084,013,000
TAI-UTC offset in effect at that instant:          +37
Coarse (TAI seconds since the 1958 TAI epoch):      2,084,013,037

Coarse: 0x7C3783ED  (4 bytes, big-endian)
Fine:   0x8000      (2 bytes: 32768/65536 = 0.5 seconds)

P-field: 0x1E  (no extension=0, CUC Level 1=001, 4 coarse=11, 2 fine=10
                → 0 001 11 10 = 0x1E)
Encoded: [0x1E] [0x7C 0x37 0x83 0xED] [0x80 0x00]
          P-field    Coarse time          Fine time
```

Decoding those 7 bytes returns exactly `2024-01-15T12:30:00.5Z`: the decoder subtracts the same 37-second offset when converting back to UTC.

## CDS — CCSDS Day Segmented Time Code

CDS represents time using human-meaningful segments: **day count** since epoch, **milliseconds** within that day, and optional **sub-millisecond** precision.

```
┌─────────────┬──────────────────┬────────────────────────┐
│    Day      │  Milliseconds    │  Sub-milliseconds      │
│  (16 or 24  │    of day        │  (optional)            │
│   bits)     │   (32 bits)      │  (0, 16, or 32 bits)  │
└─────────────┴──────────────────┴────────────────────────┘
```

### How It Works

**Day count**: Number of complete days elapsed since the epoch. With 16 bits (default), this covers 65,535 days (~179 years). With 24 bits, it covers 16,777,215 days (~45,000 years).

**Milliseconds of day**: An integer from 0 to 86,399,999 (there are 86,400,000 milliseconds in a day). This directly encodes the time of day with millisecond resolution.

**Sub-milliseconds** (optional): Additional precision within the current millisecond:
- **2 bytes**: Microseconds (0–999) — giving overall microsecond resolution
- **4 bytes**: Picoseconds (0–999,999,999) — giving overall picosecond resolution

The decoder range-checks both segments: milliseconds must be 0-86,399,999, microseconds 0-999, picoseconds 0-999,999,999. Out-of-range values are rejected instead of being silently folded into the next unit.

### P-Field Detail Bits

```
Bit 4: epoch (0 = CCSDS Level 1, 1 = agency-defined Level 2)
Bit 5: day segment length (0 = 16-bit, 1 = 24-bit)
Bits 6-7: sub-ms precision (00 = none, 01 = microseconds, 10 = picoseconds,
                            11 = reserved — rejected with an error)
```

The reserved code `11` leaves the T-field length undefined, so the decoder refuses it (`ErrReservedSubmsCode`) rather than misreading the stream.

### Leap-Second Days

CDS conversions are purely arithmetic in both levels: the day count is elapsed 86,400-second days since the epoch, and no leap-second table is applied. Because milliseconds-of-day tops out at 86,399,999, the inserted leap second **UTC 23:59:60 has no CDS encoding**. On a leap-second day, treat the day/millisecond fields as a UTC day and time-of-day label (the common convention), not as a true elapsed-time count — across a real leap second the arithmetic day boundary and UTC midnight differ by the inserted second. Missions that need true TAI elapsed time should use CUC Level 1.

### Why CDS?

CDS is popular for **ground systems and event logging** because:
- Day-based counting is intuitive for operations ("day 100 of the mission")
- Millisecond-of-day is easy to convert to hours:minutes:seconds
- The segments are large enough to inspect visually in hex dumps
- Level 2 with a mission epoch makes day counts directly meaningful ("mission day 42")

CDS is slightly less compact than CUC (minimum 7 bytes vs 2 bytes) but much easier to interpret during debugging.

### Example

A CDS timestamp for 2024-01-15T12:30:00.123456 UTC with microsecond precision (generated with `tcf.NewCDS(t, tcf.WithCDSSubmsBytes(2))`):

```
Days since 1958-01-01 to 2024-01-15:
  = 24,120 days

Milliseconds of day for 12:30:00.123:
  = (12 × 3600 + 30 × 60 + 0) × 1000 + 123 = 45,000,123

Microseconds within the millisecond:
  = 456

P-field: 0x41 (CDS Level 1, 16-bit day, microseconds
               → 0 100 0 0 01 = 0x41)
Day:          0x5E38      (24120, 2 bytes)
Milliseconds: 0x02AEA5BB  (45000123, 4 bytes)
Sub-ms:       0x01C8      (456, 2 bytes)

Encoded: [0x41] [0x5E 0x38] [0x02 0xAE 0xA5 0xBB] [0x01 0xC8]
```

Decoding those 9 bytes returns exactly `2024-01-15T12:30:00.123456Z`. Note there is no 37-second adjustment here: CDS is arithmetic day/millisecond counting (see [Leap-Second Days](#leap-second-days) above).

## CCS — CCSDS Calendar Segmented Time Code

CCS encodes time using **calendar fields** (year, month/day or day-of-year, hour, minute, second) in **Binary Coded Decimal (BCD)** format. It is always Level 1 (UTC).

### Variants

**Day-of-Year variant:**
```
┌──────────┬──────────┬──────┬──────┬──────┬───────────────────┐
│ Year     │ Day of   │ Hour │ Min  │ Sec  │ Sub-second        │
│ (2B BCD) │ Year     │ (1B) │ (1B) │ (1B) │ (0-6 octets BCD) │
│          │ (2B BCD) │      │      │      │                   │
└──────────┴──────────┴──────┴──────┴──────┴───────────────────┘
```

**Month/Day variant:**
```
┌──────────┬───────┬──────┬──────┬──────┬──────┬───────────────────┐
│ Year     │ Month │ Day  │ Hour │ Min  │ Sec  │ Sub-second        │
│ (2B BCD) │ (1B)  │ (1B) │ (1B) │ (1B) │ (1B) │ (0-6 octets BCD) │
└──────────┴───────┴──────┴──────┴──────┴──────┴───────────────────┘
```

### BCD Encoding

Binary Coded Decimal encodes each decimal digit in 4 bits (a nibble). Each byte holds two decimal digits:

```
Value 42 → BCD byte: 0100 0010 → 0x42
Value 15 → BCD byte: 0001 0101 → 0x15
Year 2024 → BCD bytes: 0x20 0x24
```

BCD is less space-efficient than pure binary but has the advantage of being human-readable in hex dumps — the hex representation directly shows the decimal value.

### Sub-Second Precision

CCS supports 0–6 additional sub-second octets. Each octet holds 2 BCD digits (representing 0–99), progressively refining the fractional second:

| Octets | Digits | Resolution | Example |
|--------|--------|------------|---------|
| 0 | 0 | 1 second | — |
| 1 | 2 | 10 ms (centisecond) | `.12` = 120 ms |
| 2 | 4 | 100 us | `.1234` = 123.4 ms |
| 3 | 6 | 1 us | `.123456` = 123.456 ms |
| 4 | 8 | 10 ns | `.12345678` |
| 5 | 10 | 100 ps | `.1234567890` |
| 6 | 12 | 1 ps | `.123456789012` |

The Second field allows value 60 to represent **leap seconds** — a rare but important edge case for UTC-based time codes. This library accepts 60 only at 23:59:60 (the only instant a positive leap second can occur). Check `IsLeapSecond()` before calling `Time()`: Go's `time.Time` cannot hold second 60, so `Time()` normalizes a leap-second code to 00:00:00 of the next day.

The decoder is strict about the wire format: any nibble greater than 9 is not a decimal digit and is rejected with `ErrInvalidBCD`, and calendar fields are cross-checked (year at most 9999, day valid for the month and leap year, day-of-year at most 365 or 366 by leap status). February 31 does not decode, and it does not encode either.

### P-Field Detail Bits

```
Bit 4: calendar variation (0 = Month/Day, 1 = Day-of-Year)
Bits 5-7: number of sub-second octets (0-6)
```

### Why CCS?

CCS is ideal when:
- Timestamps need to be **human-readable in binary dumps** (BCD values match their decimal representation)
- The time reference is **UTC** (CCS is always UTC, never TAI)
- Calendar dates are more meaningful than elapsed time for the application
- Compatibility with **ISO 8601-like** representations is needed

CCS is the least compact of the binary formats (8–14 bytes) but the most immediately interpretable.

## ASCII Time Codes

For contexts where binary encoding is unnecessary or inconvenient (log files, displays, text protocols), CCSDS defines two ASCII time code formats derived from ISO 8601:

### Type A — Calendar Date-Time

```
YYYY-MM-DDThh:mm:ss.d...dZ
```

Examples:
```
2024-01-15T12:30:00Z
2024-01-15T12:30:00.123Z
2024-01-15T12:30:00.123456789Z
```

### Type B — Ordinal Date-Time

```
YYYY-DDDThh:mm:ss.d...dZ
```

Where `DDD` is the day of year (001–366).

Examples:
```
2024-015T12:30:00Z
2024-015T12:30:00.123Z
2024-015T12:30:00.123456789Z
```

### Format Details

- The `T` separator between date and time is mandatory
- The `Z` suffix indicates UTC and is always appended (optional on decode)
- Fractional seconds can have 0–9 digits
- Type B is common in mission operations where "day of year" is the standard reference

These are **fixed-width subsets** of ISO 8601, and the decoder enforces them strictly: every field must have its exact width and contain only digits (`2024-1-01` is rejected, so is a signed or padded number), separators must be in their fixed positions, and values are range-checked against the calendar (month 1–12, day valid for the month and leap year, day-of-year 1–365/366, hour 0–23, minute 0–59). Second 60 is accepted only at 23:59:60 and — like CCS — normalizes to 00:00:00 of the next day, since Go's `time.Time` cannot represent it.

### When to Use ASCII

ASCII time codes are used in:
- Ground system logs and displays
- Text-based command interfaces
- Human-readable file metadata
- Situations where parsing libraries for ISO 8601 are readily available

They are not used in flight data (too large, too expensive to parse with onboard processors).

## Choosing a Format

| Criterion | CUC | CDS | CCS | ASCII |
|-----------|-----|-----|-----|-------|
| **Size** | 2–19 bytes (P-field 1–2 + coarse 1–7 + fine 0–10) | 7–12 bytes (P-field 1 + day 2–3 + ms 4 + sub-ms 0–4) | 8–14 bytes | 20–30+ chars |
| **Human-readable** | No | Partially | Yes (BCD) | Yes |
| **Epoch** | Level 1 or 2 | Level 1 or 2 | Level 1 only (UTC) | UTC |
| **Best for** | Onboard clocks, compact TM | Ground systems, event logs | Calendar displays, UTC-critical | Logs, displays, text |
| **Hardware impl.** | Trivial (counter) | Moderate | Complex (BCD) | N/A |
| **Precision range** | 1 s to 2^-80 s | 1 ms to 1 ps | 1 s to 1 ps | 1 s to 1 ns |
| **Calendar calc** | Needed for display | Day math only | Built-in | Built-in |

**Rules of thumb:**
- If it runs on a spacecraft processor → **CUC**
- If it runs on the ground and needs to be debuggable → **CDS**
- If it needs to be calendar-readable in binary → **CCS**
- If it is text → **ASCII Type A** (calendar) or **Type B** (ordinal)

## TAI, UTC, and Leap Seconds

Understanding the time scale distinction is critical for correct time code interpretation:

**TAI (International Atomic Time)**: A continuous, monotonic time scale based on atomic clocks. It never has discontinuities or adjustments. TAI seconds are SI seconds.

**UTC (Coordinated Universal Time)**: Civil time scale that stays within 0.9 seconds of the Earth's rotation. To achieve this, **leap seconds** are occasionally inserted (about every 1–3 years). When a leap second occurs, the time goes `23:59:59 → 23:59:60 → 00:00:00`.

**The difference**: Since 1972 the TAI-UTC offset has been a whole number of seconds, growing from 10 s (1972-01-01) to 37 s (2017-01-01, still current). The CCSDS epoch (1958-01-01) predates the introduction of integer UTC leap seconds in 1972.

### What this library does

The full table of integer TAI-UTC offsets is embedded in the package (they are historical facts and cannot change). `tcf.TAIUTCOffsetAt(t)` returns the offset in effect at any instant. The formats use it as follows:

- **CUC Level 1** (CCSDS 1958 TAI epoch): the coarse count is **true TAI seconds**. `NewCUC` adds the offset in effect at the encoded instant; `Time()` subtracts it again on decode. A UTC instant therefore round-trips exactly, and the on-wire count matches what a real TAI-referenced system produces.
- **CUC Level 2** (agency-defined epoch): **purely arithmetic**. The coarse count is the elapsed seconds between epoch and instant, with no leap-second correction in either direction. If your mission epoch is itself TAI-referenced, apply your own convention on top.
- **CDS** (both levels): purely arithmetic day/millisecond counting; no leap-second table is applied. The day/millisecond fields act as a UTC day and time-of-day label. See [Leap-Second Days](#leap-second-days).
- **CCS**: always UTC calendar fields. Second 60 encodes the leap second itself; `Time()` normalizes it (see the CCS section).
- **ASCII**: always UTC (indicated by the `Z` suffix). Second 60 accepted only at 23:59:60.

### Edge cases (by design)

- **Before 1972**: UTC used fractional "rubber-second" adjustments that have no integer representation. The library treats TAI and UTC as identical in that era (offset 0). TAI counts falling inside the 10-second step at 1972-01-01 do not round-trip exactly.
- **Inside a leap second**: a CUC Level 1 count that lands on UTC 23:59:60 decodes to the following 00:00:00, because Go's `time.Time` cannot represent second 60.
- **Future leap seconds**: the table ends at 37 s (2017-01-01) and none has been announced since. If the IERS ever announces one, the table (`pkg/tcf/leapseconds.go`) needs a one-line addition.

**Practical advice**: Store times in TAI or mission elapsed time onboard. Convert to UTC only at the ground system boundary where the leap second table is available and up to date.

## Reference

- [CCSDS 301.0-B-4](https://public.ccsds.org/Pubs/301x0b4e1.pdf) — Time Code Formats (Blue Book)
- [CCSDS 301.0-G-1](https://public.ccsds.org/Pubs/301x0g1.pdf) — Time Code Formats Summary (Green Book)
- [CCSDS 320.0-B-7](https://public.ccsds.org/Pubs/320x0b7.pdf) — CCSDS Global Spacecraft Identification Field
