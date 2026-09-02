---
title: Time Code Formats
description: "Coverage matrix: what pkg/tcf implements from CCSDS 301.0-B-4, and what it does not."
order: 200
---

## Coverage matrix for `pkg/tcf` — CCSDS 301.0-B-4

**CCSDS 301.0-B-4 ships no PICS proforma.** It is a format definition
standard, not a protocol with a conformance annex, so there is no official
table to fill in. What follows is a coverage matrix written against the
normative text: each row names a feature, the clause that defines it, and what
this package does with it.

Rows marked **derived** were checked against the standard's text rather than a
published test vector, because the standard publishes none for that item. The
distinction matters: a derived row is one implementer's reading, and a
round-trip test cannot tell you it is wrong.

---

## A. General information

| | |
|---|---|
| Implementation | `github.com/ravisuhag/astro/pkg/tcf` |
| Standard | CCSDS 301.0-B-4, Time Code Formats |
| Scope | Encoding and decoding of the four time code formats, with the P-field and bare T-field forms |
| Not in scope | Time correlation, clock discipline, and onboard time management, none of which this standard defines |

---

## B. P-field

| Feature | Clause | Status | Notes |
|---|---|---|---|
| First octet: extension flag, 3-bit ID, 4 detail bits | §3.2.2 and per-format | Y | `PField` |
| Second octet when the extension flag is set | §3.2.2 | Y | 7 detail bits |
| Bit 0 of the second octet reserved for a third octet | §3.2.2 | Y | No third octet is defined, so a set bit is rejected with `ErrInvalidPField` rather than misparsing the next octet as T-field data. **Derived** — the standard reserves the bit without stating a receiver rule. |
| Time code ID `001` — CUC Level 1 | §3.2 | Y | `TimeCodeCUCLevel1` |
| Time code ID `010` — CUC Level 2 | §3.2 | Y | `TimeCodeCUCLevel2` |
| Time code ID `100` — CDS | §3.3 | Y | `TimeCodeCDS`; level from detail bit 4 |
| Time code ID `101` — CCS | §3.4 | Y | `TimeCodeCCS`; Level 1 only |
| Unrecognised time code ID | — | Y | `ErrInvalidTimeCodeID` |
| Implicit P-field (bare T-field) | — | Y | `EncodeTField`, `DecodeCUCTField`, `DecodeCDSTField`, `DecodeCCSTField`. The standard permits an agreed format without a preamble; the octet counts, variant, and epoch are then caller-supplied. |

---

## C. CUC — Unsegmented Time Code (§3.2)

| Feature | Clause | Status | Notes |
|---|---|---|---|
| Coarse time, 1–4 basic octets | §3.2.2 | Y | `WithCUCCoarseBytes` |
| Fine time, 0–3 basic octets | §3.2.2 | Y | `WithCUCFineBytes` |
| Extension octet: additional coarse (2 bits), additional fine (3 bits) | §3.2.2 | Y | Coarse to 7 octets, fine to 10 |
| Reserved bits 6–7 of the extension octet | §3.2.2 | Y | A non-zero value is rejected |
| Out-of-range octet counts | §3.2.2 | Y | `ErrInvalidCoarseOctets`, `ErrInvalidFineOctets` |
| Level 1 — CCSDS epoch, 1958-01-01 TAI | §3.2 | Y | Coarse count is **true TAI seconds**: the offset in effect at the encoded instant is added on encode and removed on decode |
| Level 2 — agency-defined epoch | §3.2 | Y | Purely arithmetic, no leap-second correction. `WithCUCEpoch`; `ErrEpochRequired` if omitted |
| Coarse value exceeding the configured width | §3.2.2 | Y | `ErrOverflow` |
| Fine-time wire fidelity across an encode/decode cycle | §3.2.2 | Y | Fine octets are preserved as transmitted, not re-derived from a rounded duration |

---

## D. CDS — Day Segmented Time Code (§3.3)

| Feature | Clause | Status | Notes |
|---|---|---|---|
| Day segment, 16-bit | §3.3.2 | Y | `WithCDSDayBytes(2)` |
| Day segment, 24-bit | §3.3.2 | Y | `WithCDSDayBytes(3)` |
| Milliseconds of day, 0–86399999 | §3.3.2 | Y | `ErrInvalidMilliseconds` outside range |
| Sub-millisecond absent (code `00`) | §3.3.2 | Y | `WithCDSSubmsBytes(0)` |
| Sub-millisecond microseconds (code `01`, 2 octets) | §3.3.2 | Y | `WithCDSSubmsBytes(2)` |
| Sub-millisecond picoseconds (code `10`, 4 octets) | §3.3.2 | Y | `WithCDSSubmsBytes(4)` |
| Reserved sub-millisecond code `11` | §3.3.2 | Y | Rejected with `ErrReservedSubmsCode` |
| Sub-millisecond value outside the declared resolution | §3.3.2 | Y | `ErrInvalidSubmilliseconds` |
| Extension flag on a CDS P-field | §3.3.2 | Y | The CDS P-field is a single octet; a set extension flag is rejected |
| Level 1 and Level 2 epochs | §3.3 | Y | `WithCDSEpoch` |
| Leap seconds | §3.3 | N/A | Day and millisecond counting is purely arithmetic. The fields act as a UTC day and time-of-day label; no leap-second table is applied. |

---

## E. CCS — Calendar Segmented Time Code (§3.4)

| Feature | Clause | Status | Notes |
|---|---|---|---|
| Month/Day variant (detail bit 4 = 0) | §3.4.2 | Y | `WithCCSMonthDay` |
| Day-of-Year variant (detail bit 4 = 1) | §3.4.2 | Y | Default |
| BCD encoding of every segment | §3.4.1 | Y | |
| Upper 4 bits of the year's first octet are zero | §3.4.1.2 | Y | Enforced on encode and decode |
| Non-BCD nibble (value above 9) | §3.4.1 | Y | `ErrInvalidBCD` |
| Sub-second octets, 0–6 | §3.4.2 | Y | `WithCCSSubSecBytes`; each octet holds two decimal digits |
| Second value 60 (the leap second) | §3.4 | Y | Accepted and flagged; `Time()` normalizes it, because Go's `time.Time` cannot represent second 60 |
| Calendar field range checks | §3.4.1 | Y | Month 1–12, day of month 1–31, day of year 1–366, hour 0–23, minute 0–59; `ErrInvalidCalendarTime` |
| Cross-checks between calendar fields | §3.4.1 | Y | **Derived** — the standard gives per-field ranges; consistency between them is checked here as a decoder safeguard |
| Level 2 | §3.4 | N/A | CCS is Level 1 UTC only, by definition |

---

## F. ASCII time codes (§3.5)

| Feature | Clause | Status | Notes |
|---|---|---|---|
| Type A — `YYYY-MM-DDThh:mm:ss.d…dZ` | §3.5.1.1 | Y | `ASCIITypeA` |
| Type B — `YYYY-DDDThh:mm:ss.d…dZ` | §3.5 | Y | `ASCIITypeB` |
| Fractional seconds, 1–9 digits | §3.5 | Y | `WithASCIIPrecision`; default 3 |
| Optional `Z` terminator | §3.5 | Y | Accepted on decode, written on encode |
| Fixed field widths and mandatory separators | §3.5 | Y | Decode enforces the subset strictly and rejects anything ISO 8601 allows but §3.5 does not |
| Second 60 | §3.5 | Y | Accepted only at 23:59:60 |
| P-field | §3.5 | N/A | ASCII codes carry no P-field |

---

## G. Time scales and leap seconds

| Feature | Status | Notes |
|---|---|---|
| Integer TAI–UTC offset table | Y | Embedded in `leapseconds.go`; `TAIUTCOffsetAt(t)` returns the offset in effect at any instant. The values are historical facts and cannot change. |
| Current offset | Y | 37 s from 2017-01-01. No leap second has been announced since. |
| Pre-1972 era | Partial | UTC used fractional "rubber second" adjustments with no integer representation. TAI and UTC are treated as identical (offset 0). TAI counts falling inside the 10-second step at 1972-01-01 do not round-trip exactly. **Documented limitation.** |
| An instant inside a leap second | Partial | A CUC Level 1 count landing on UTC 23:59:60 decodes to the following 00:00:00. Go's `time.Time` cannot represent second 60. **Documented limitation.** |
| Future leap seconds | N | Not predictable. A new IERS announcement needs a one-line addition to `leapseconds.go`. |

---

## H. Not implemented

| Item | Reason |
|---|---|
| Time correlation and clock discipline | Not defined by this standard |
| Agency-specific epoch registries | A mission supplies its own epoch through `WithCUCEpoch` or `WithCDSEpoch` |
| P-field octets beyond the second | None are defined by CCSDS 301.0-B-4 |

---

## I. Verification

The conformance regressions live in `pkg/tcf/audit_test.go`, tagged `TCF-n`
per defect. They cover the TAI–UTC table against its historical values, CUC
Level 1 leap-second handling, CUC Level 2 arithmetic behaviour, fine-time wire
fidelity, the rejected P-field extension bit, the reserved CDS sub-millisecond
code, CCS BCD and calendar validation, strict ASCII decoding, bare T-field
round trips, and overflow guards on large coarse and day values.

`pkg/tcf/fuzz_test.go` fuzzes the decoders: arbitrary bytes must never panic
and never allocate from a length field an attacker controls.

## Reference

- [CCSDS 301.0-B-4](https://public.ccsds.org/Pubs/301x0b4e1.pdf) — Time Code Formats (Blue Book)
- [Protocol page](/protocols/mission/tcf) · [CLI](/cli/time)
