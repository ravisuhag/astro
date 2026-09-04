---
title: Tracking Data Message
short: TDM
description: CCSDS 503.0-B-2, what a ground station measured while watching a spacecraft.
identifiers:
  - "CCSDS 503.0-B-2 * Tracking Data Message"
  - "pkg/tdm"
order: 34
---

> **CCSDS 503.0-B-2** | [Blue Book](https://ccsds.org/Pubs/503x0b2c1.pdf) | [`pkg/tdm`](https://github.com/ravisuhag/astro/tree/main/pkg/tdm)

## Overview

A TDM carries what a ground station measured while it was watching something:
range, Doppler, angles, signal levels, and the weather and clock corrections
that go with them. It is what one agency sends another after tracking a
spacecraft on their behalf, and it is normally one pass per file.

Where an [ODM](/protocols/mission/odm) says where a spacecraft *is*, a TDM says
what a station *saw*. The first is a conclusion; the second is evidence.

The shape is a header and then one or more **segments**. A segment is a
metadata section describing how the measurements were taken, followed by a data
section of Tracking Data Records.

## Scope

**Implemented.** Reading and writing, in key-value notation. Every metadata
keyword table 3-3 allows and every data keyword table 3-5 allows.

**Not yet implemented.** The XML form of section 5.

**Deliberately absent: tracking mathematics.** Nothing here differences a
range, unwraps an ambiguous one, applies a media or clock correction, or
converts an angle between frames. Those need the interface control document
the standard keeps deferring to, and clause 3.1.7 puts even the exchange
method outside its own scope.

## A measurement means nothing without its metadata

This is the trap. A Tracking Data Record is a keyword, a timetag and a number:

```
RANGE = 2010-215T20:04:24.000   65249.6771931631
```

Nothing in that line says what the number is in. Clause 3.5.2.7 puts the units
in the segment's `RANGE_UNITS`, which may be `km`, `s` or `RU` — and **if the
keyword is absent the default is km**. The record above came from a segment
declaring `RU`. Read as kilometres it is wrong by orders of magnitude, and the
record itself would never tell you.

Three keywords work this way:

| Keyword | Without it | Go |
|---|---|---|
| `RANGE_UNITS` | A range is assumed to be km (clause 3.5.2.7) | `Metadata.RangeUnits` |
| `RANGE_MODULUS` | A non-zero value means the range is *ambiguous* and "does not represent the actual range to the spacecraft" until a calculation using the modulus is done | `Metadata.RangeModulus` |
| `ANGLE_TYPE` | `ANGLE_1` and `ANGLE_2` are two numbers with no frame | `Metadata.AngleType` |

`RangeUnits` returns the default rather than an empty string, and `Humanize`
says out loud when the value was defaulted rather than stated.

## A segment boundary is a configuration change

Clause 3.3.1.4 requires a new segment whenever any metadata value changes. A
switch from one-way to two-way tracking, a different band, a different station:
each ends a segment and starts another.

So the segments are not a packaging convenience, and two segments in one file
may disagree about the units their measurements are in. Flattening them into
one list of observations loses the only thing that says how to read each
number.

## The metadata is a list, not a struct

`Metadata` holds an ordered list of keyword-value pairs rather than forty named
fields. That is what table 3-3 is: only `TIME_SYSTEM` and `PARTICIPANT_n` are
mandatory, and the other forty-odd are optional station configuration whose
meaning lives in an ICD.

A struct would be forty pointers, and a caller meeting an unfamiliar keyword
would have no way to see it. `Get` reaches anything; the accessors cover what
changes how a number must be read.

## Two keyword families overlap by prefix

`TRANSMIT_FREQ_n` and `TRANSMIT_FREQ_RATE_n` are both in table 3-5. Matching
the shorter prefix first refuses `TRANSMIT_FREQ_RATE_1`, on the grounds that
`RATE_1` is not an index between 1 and 5.

`RECEIVE_FREQ` is the other oddity: table 3-5 lists it both bare and indexed,
so `RECEIVE_FREQ` and `RECEIVE_FREQ_1` are both legal. `TRANSMIT_FREQ` bare is
not.

## Using the package

```go
message, err := tdm.Decode(data)

for _, segment := range message.Segments {
    // These come from the segment, never from a record.
    units := segment.Metadata.RangeUnits()
    modulus, ambiguous := segment.Metadata.RangeModulus()

    for _, obs := range segment.Observations {
        if obs.Keyword == "RANGE" && ambiguous {
            // Clause 3.5.2.7: not the range until the modulus is applied.
            _ = modulus
        }
        fmt.Println(obs.Keyword, obs.Epoch, obs.Value, units)
    }
}
```

### Errors

| Error | Means |
|---|---|
| `ErrNoSegment` | A message with no segment (clause 3.1.3). |
| `ErrMissingTimeSystem` | A metadata section without `TIME_SYSTEM`, the one keyword table 3-3 makes mandatory. |
| `ErrMissingParticipant` | No `PARTICIPANT_n`; table 3-3 requires at least one. |
| `ErrParticipantIndex` | An index outside 1 to 5. |
| `ErrMissingDataSection` | A metadata section with no data section after it (clause 3.3.1.3). |
| `ErrNoRecords` | A data section with no records. |
| `ErrMalformedRecord` | A record whose value is not a timetag and one measurement (clause 3.4.3). |
| `ErrUnknownKeyword` | A keyword neither table 3-3 nor table 3-5 lists. |
| `ErrUnterminatedBlock` | A `META_START` or `DATA_START` that never closes. |

## Reference

- [CCSDS 503.0-B-2, Tracking Data Message](https://ccsds.org/Pubs/503x0b2c1.pdf)
- [CCSDS 500.0-G-4, Navigation Data Messages Overview](https://ccsds.org/Pubs/500x2g3.pdf) (Green Book)
- [Conformance](/conformance/tdm) | [Orbit Data Messages](/protocols/mission/odm) | [The stack](/docs/start/concepts)
