---
title: Tracking Data Message
short: TDM
description: "Coverage matrix: what this package implements, clause by clause."
order: 215
---

## Conformance Statement for `pkg/tdm`, CCSDS 503.0-B-2

CCSDS 503.0-B-2 annex A ships an Implementation Conformance Statement. What
follows fills in its shape for the key-value form; the XML form of section 5 is
not implemented.

---

## A1 IDENTIFICATION

| Field | Value |
|---|---|
| Date of Statement (DD/MM/YYYY) | 04/09/2026 |
| ICS Serial Number | ASTRO-TDM-ICS-001 |
| Implementation Name | astro/pkg/tdm |
| Implementation Version | See `go.mod` / latest commit on `main` |
| Supplier | Ravi Suhag |
| Contact Point for Queries | GitHub, github.com/ravisuhag/astro |
| Specification | CCSDS 503.0-B-2 (Tracking Data Message, Blue Book, June 2020, with updates) |
| Time formats | CCSDS 301.0-B-4 ASCII time codes A and B, via `pkg/tcf` |
| Have any exceptions been required? | Yes [X] No [ ], see A3 |

---

## A2 REQUIREMENTS

### Structure

| Feature | Reference | Status | Support |
|---|---|:-:|---|
| TDM is ASCII text | 3.1.1 | M | Y |
| Header, then a body of one or more segments | 3.1.3 | M | Y |
| Segment is a metadata section plus a data section | 3.1.2 | M | Y |
| A data section holds at least one record | 3.1.3 | M | Y: `ErrNoRecords` |
| No limit on segments | 3.1.3 | M | Y: bounded by the input only |
| A metadata section precedes each data section | 3.3.1.3 | M | Y: `ErrMissingDataSection` |
| A new segment on any metadata change | 3.3.1.4 | M | Y: segments are kept separate, never flattened |
| `META_START` and `META_STOP` first and last in a metadata section | 3.3.1.5 | M | Y |
| `DATA_START` and `DATA_STOP` first and last in a data section | 3.4.7, 3.5.1 | M | Y |
| Exchange as a stream or a file | 3.1.5 | M | N: a file only, see A3 |

### Header

| Feature | Reference | Status | Support |
|---|---|:-:|---|
| `CCSDS_TDM_VERS` | Table 3-2 | M | Y |
| `COMMENT` | Table 3-2 | O | Y |
| `CREATION_DATE` | Table 3-2 | M | Y: UTC |
| `ORIGINATOR` | Table 3-2 | M | Y: not checked against the SANA registry |
| `MESSAGE_ID` | Table 3-2 | O | Y |
| No `CLASSIFICATION` | Table 3-2 | — | Y: refused, since table 3-2 does not list it |

### Metadata

| Feature | Reference | Status | Support |
|---|---|:-:|---|
| `keyword = value` on every line but a comment | 3.3.1.2 | M | Y |
| Only the keywords of table 3-3 | 3.3.1.7 | M | Y: `ErrUnknownKeyword` |
| `TIME_SYSTEM` | Table 3-3 | M | Y: `ErrMissingTimeSystem` |
| `PARTICIPANT_n`, at least one, index 1 to 5 | Table 3-3 | M | Y: `ErrMissingParticipant`, `ErrParticipantIndex` |
| The other forty-odd optional keywords | Table 3-3 | O | Y: carried in wire order, reachable by `Get` |
| Indexed families `TRANSMIT_DELAY_n`, `RECEIVE_DELAY_n` | Table 3-3 | O | Y |
| Default values applied where the table defines one | 3.3.1.7 | M | Partial: `RANGE_UNITS` only, see A3 |

### Data

| Feature | Reference | Status | Support |
|---|---|:-:|---|
| Record is `keyword = timetag measurement` | 3.4.1, Table 3-4 | M | Y |
| One record per line | 3.4.2 | M | Y |
| A timetag and one observable, both required | 3.4.3 | M | Y: `ErrMalformedRecord` |
| At least one blank between them | 3.4.4 | M | Y: any run of blanks |
| No mandatory data keywords beyond the delimiters | 3.4.6 | M | Y |
| Only the keywords of table 3-5 | 3.5.1 | M | Y |
| Indexed families to index 5 | Table 3-5 | O | Y: including `TRANSMIT_FREQ_RATE_n`, which shares a prefix with `TRANSMIT_FREQ_n` |
| `RECEIVE_FREQ` legal bare and indexed | Table 3-5 | O | Y: `TRANSMIT_FREQ` bare is refused, as the table lists only the indexed form |

### Syntax

| Feature | Reference | Status | Support |
|---|---|:-:|---|
| Line length ceiling | 4.2 | M | Y: 254 characters |
| Printable ASCII and blanks only | 4.2 | M | Y |
| Integer and non-integer value forms | 4.3 | M | Y: shared with the ODM through `internal/ndm` |
| Timetag formats | 4.3.9 | M | Y: both ASCII time codes, via `pkg/tcf` |
| Comment placement | 4.5 | O | Y: header, metadata section, data section |

### XML form, section 5

| Feature | Reference | Status | Support |
|---|---|:-:|---|
| Root element with id and version attributes | 5 | M | Y |
| Master schema for the TDM | 5 | O | Y: `ndmxml-2.0.0-master-2.0.xsd` |
| NDM namespace declared | 505.0-B-3 clause 4.3.4 | M | Y: the TDM's own example declares it |
| One `observation` element per record | 5 | M | Y |
| A timetag and exactly one observable per observation | 3.4.3 | M | Y: `ErrMalformedRecord` for two or none |
| Metadata as a flat element list | 5 | M | Y |

---

## A3 EXCEPTIONS AND UNSUPPORTED FEATURES

**Streaming is not supported.** Clause 3.1.5 says a TDM may be exchanged as a
real-time stream or as a file. `Decode` takes a complete message. A stream
would need the segment structure delivered incrementally, and the delimiters
make that possible, but nothing here does it.

**No tracking mathematics.** Nothing differences a range, unwraps an ambiguous
one, applies a media, clock or delay correction, or converts an angle between
frames. The metadata that describes those corrections is carried faithfully and
acted on by nobody. This matters most for two keywords: a non-zero
`RANGE_MODULUS` means clause 3.5.2.7's ambiguous range, which "does not
represent the actual range to the spacecraft" until the modulus is applied, and
`CORRECTIONS_APPLIED` tells a reader whether the producer has already folded
its corrections in.

**Only one default is applied.** Clause 3.3.1.7 says a keyword with a defined
default should be assumed when absent. `RANGE_UNITS` is implemented that way,
because reading a range in the wrong units is a silent order-of-magnitude
error. The other defaults are left to the caller: applying them would mean
this package deciding what a segment meant, and `Get` reports absence
truthfully so a caller can decide.

**Enumerated values are not enforced.** `MODE`, `RANGE_MODE`, `TIMETAG_REF`,
`DATA_QUALITY` and the rest have normative value sets in table 3-3. They are
carried as written. Refusing an unlisted value would refuse a message a later
issue of the standard makes legal.

**Registry values are not checked.** `PARTICIPANT_n` may be a station name, a
spacecraft designator or a quasar catalogue name; `ORIGINATOR` points at the
SANA organizations registry. Neither is validated.

---

## A4 IMPLEMENTATION LIMITS

| Limit | Value | Source |
|---|---|---|
| Line length | 254 characters | Clause 4.2 |
| Participant index | 1 to 5 | Table 3-3 |
| Data keyword index | 1 to 5 | Table 3-5 |
| Segments per message | bounded by the input | Clause 3.1.3 imposes none |
| Records per segment | bounded by the input | Records are read into memory rather than streamed |

---

## Wire test vectors

The files backing this statement live in the [vector corpus](https://github.com/ravisuhag/astro/tree/main/vectors/tdm) — 1 decode vector and 1 corpus file.

| File | |
|---|---|
| [`tdm/tracking.json`](https://github.com/ravisuhag/astro/blob/main/vectors/tdm/tracking.json) | 1 vector |
| `tdm/two-way-range.kvn` | the annex E example as a readable file |

Both are **published text rather than derived values**: annex E of the Blue Book prints the example.

The vector asserts the metadata that decides how a measurement must be read — the time system, the participants, the mode and path, and above all the range units and whether they were stated or defaulted — plus the record counts. The measurements are floats, which a vector field cannot hold, and are checked in `pkg/tdm` against the same published text.

See [`CONTRACT.md`](https://github.com/ravisuhag/astro/blob/main/vectors/CONTRACT.md) for how to consume these, and [how this is verified](/docs/reference/verification) for what rests on a published vector versus a reading of the clause.
