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

**TAI vs UTC**: The standard epoch is in **TAI (International Atomic Time)**, which does not have leap seconds. UTC periodically adds leap seconds to stay synchronized with Earth's rotation. As of 2025, TAI is 37 seconds ahead of UTC. This distinction matters when converting between onboard time (often TAI-based) and ground time (often UTC-based).

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

## CUC — CCSDS Unsegmented Time Code

The simplest and most compact binary format. CUC represents time as a single binary counter split into two parts:

```
┌──────────────────────┬──────────────────────┐
│   Coarse Time        │     Fine Time        │
│  (seconds since      │  (binary fraction    │
│   epoch)             │   of a second)       │
│  1-4 octets (basic)  │  0-3 octets (basic)  │
│  up to 7 (extended)  │  up to 6 (extended)  │
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

The binary fraction approach means fine time is computed as:

```
fractional_seconds = fine_time_value / (2 ^ (fine_octets × 8))
```

For example, with 2 fine octets: a fine time value of 32,768 represents 32768/65536 = 0.5 seconds.

### P-Field Detail Bits

```
Bits 4-5: (coarse_octets - 1)    → 0-3 maps to 1-4 octets
Bits 6-7: fine_octets             → 0-3

Extension octet (if needed):
Bits 1-2: additional coarse octets (0-3)
Bits 3-5: additional fine octets (0-7)
```

### Why CUC?

CUC is the format of choice for **onboard spacecraft clocks**. Its advantages:
- Extremely compact (as few as 2 bytes for a timestamp with 1-second resolution)
- Simple hardware implementation — just a binary counter
- No calendar calculations needed onboard
- Monotonically increasing — no ambiguity from leap seconds, daylight saving, or calendar irregularities

The trade-off: CUC timestamps are not human-readable without conversion.

### Example

A CUC timestamp with 4 coarse + 2 fine octets at time 2024-01-15T12:30:00.500 UTC:

```
Seconds since 1958-01-01 to 2024-01-15T12:30:00:
  = 2,084,439,037 (approximately, adjusted for TAI offset)

Coarse: 0x7C3E1F1D (4 bytes, big-endian)
Fine:   0x8000      (2 bytes: 32768/65536 = 0.5 seconds)

P-field: 0x3E  (no extension, CUC Level 1, 4 coarse, 2 fine)
Encoded: [0x3E] [0x7C 0x3E 0x1F 0x1D] [0x80 0x00]
          P-field    Coarse time          Fine time
```

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

### P-Field Detail Bits

```
Bit 4: epoch (0 = CCSDS Level 1, 1 = agency-defined Level 2)
Bit 5: day segment length (0 = 16-bit, 1 = 24-bit)
Bits 6-7: sub-ms precision (00 = none, 01 = microseconds, 10 = picoseconds)
```

### Why CDS?

CDS is popular for **ground systems and event logging** because:
- Day-based counting is intuitive for operations ("day 100 of the mission")
- Millisecond-of-day is easy to convert to hours:minutes:seconds
- The segments are large enough to inspect visually in hex dumps
- Level 2 with a mission epoch makes day counts directly meaningful ("mission day 42")

CDS is slightly less compact than CUC (minimum 7 bytes vs 2 bytes) but much easier to interpret during debugging.

### Example

A CDS timestamp for 2024-01-15T12:30:00.123456 UTC with microsecond precision:

```
Days since 1958-01-01 to 2024-01-15:
  = 24,121 days

Milliseconds of day for 12:30:00.123:
  = (12 × 3600 + 30 × 60 + 0) × 1000 + 123 = 45,000,123

Microseconds within the millisecond:
  = 456

P-field: 0x41 (CDS Level 1, 16-bit day, microseconds)
Day:          0x5E39      (24121, 2 bytes)
Milliseconds: 0x02AEA77B  (45000123, 4 bytes)
Sub-ms:       0x01C8       (456, 2 bytes)

Encoded: [0x41] [0x5E 0x39] [0x02 0xAE 0xA7 0x7B] [0x01 0xC8]
```

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

The Second field allows value 60 to represent **leap seconds** — a rare but important edge case for UTC-based time codes.

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
| **Size** | 2–9 bytes | 7–11 bytes | 8–14 bytes | 20–30+ chars |
| **Human-readable** | No | Partially | Yes (BCD) | Yes |
| **Epoch** | Level 1 or 2 | Level 1 or 2 | Level 1 only (UTC) | UTC |
| **Best for** | Onboard clocks, compact TM | Ground systems, event logs | Calendar displays, UTC-critical | Logs, displays, text |
| **Hardware impl.** | Trivial (counter) | Moderate | Complex (BCD) | N/A |
| **Precision range** | 1s to ~60ns | 1ms to ps | 1s to 1ps | 1s to 1ns |
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

**The difference**: As of 2025, TAI = UTC + 37 seconds. This offset increases by 1 each time a leap second is added. The CCSDS epoch (1958-01-01) predates the introduction of UTC leap seconds in 1972.

**Implications for time codes:**
- **CUC Level 1**: Counts TAI seconds since 1958 epoch. To convert to UTC, subtract the TAI-UTC offset applicable at that time.
- **CDS Level 1**: The epoch is TAI-based, but the day/millisecond representation is often treated as UTC by convention. Check mission documentation.
- **CCS**: Always UTC. The Second field allows value 60 specifically to represent the leap second.
- **ASCII**: Always UTC (indicated by the `Z` suffix).

**Practical advice**: Store times in TAI or mission elapsed time onboard. Convert to UTC only at the ground system boundary where the leap second table is available and up to date.

## Reference

- [CCSDS 301.0-B-4](https://public.ccsds.org/Pubs/301x0b4e1.pdf) — Time Code Formats (Blue Book)
- [CCSDS 301.0-G-1](https://public.ccsds.org/Pubs/301x0g1.pdf) — Time Code Formats Summary (Green Book)
- [CCSDS 320.0-B-7](https://public.ccsds.org/Pubs/320x0b7.pdf) — CCSDS Global Spacecraft Identification Field
