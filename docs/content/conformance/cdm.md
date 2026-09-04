---
title: Conjunction Data Message
short: CDM
description: "Coverage matrix: what this package implements, clause by clause."
order: 225
---

## Conformance Statement for `pkg/cdm`, CCSDS 508.0-B-1

CCSDS 508.0-B-1 annex A ships an Implementation Conformance Statement. What
follows fills in its shape for both forms: the key-value notation of section 3
and the XML form of section 4.

---

## A1 IDENTIFICATION

| Field | Value |
|---|---|
| Date of Statement (DD/MM/YYYY) | 04/09/2026 |
| ICS Serial Number | ASTRO-CDM-ICS-001 |
| Implementation Name | astro/pkg/cdm |
| Implementation Version | See `go.mod` / latest commit on `main` |
| Supplier | Ravi Suhag |
| Contact Point for Queries | GitHub, github.com/ravisuhag/astro |
| Specification | CCSDS 508.0-B-1 (Conjunction Data Message, Blue Book, June 2013, with updates through Corrigendum 2) |
| Time formats | CCSDS 301.0-B-4 ASCII time codes A and B, via `pkg/tcf` |
| Have any exceptions been required? | Yes [X] No [ ], see A3 |

---

## A2 REQUIREMENTS

### Structure

| Feature | Reference | Status | Support |
|---|---|:-:|---|
| Plain text, one keyword per line | 3.1.1, 6.3.1.4 | M | Y |
| XML form | 4, CCSDS 505.0-B-3 | M | Y: `EncodeXML` and `DecodeXML`; units become attributes and the data section nests into four blocks |
| Exactly two segments in XML | 505.0-B-3 clause 3.4.2 | M | Y: `ErrMissingObject` |
| `relativeMetadataData` before the first segment | 505.0-B-3 clause 3.4.2 | M | Y |
| NDM combined instantiation | 505.0-B-3 clause 4.11 | O | Y: implemented by [`pkg/ndm`](/conformance/ndm). CCSDS 508.0-B-1 does not repeat the rules the way the ODM and ADM do, but clause 4.11.7 of the XML specification allows a CDM among the constituents. |
| Header, relative metadata/data, then two object sections | 3.1.1 | M | Y |
| Data for a single conjunction event | 3.1.2 | M | Y: both objects required, `ErrMissingObject` |
| Keyword order fixed by the standard | 6.3.1.9 | M | Partial: read in any order, written in the order read, see A3 |
| `OBJECT` separates the object sections | Table 3-3 | M | Y: a keyword's section is checked, so an object keyword before any `OBJECT` is refused |

### Header, table 3-1

| Feature | Status | Support |
|---|:-:|---|
| `CCSDS_CDM_VERS` | M | Y |
| `COMMENT` | O | Y |
| `CREATION_DATE` | M | Y: UTC |
| `ORIGINATOR` | M | Y: not checked against the SANA registry, see A3 |
| `MESSAGE_FOR` | O | Y: exists in no other navigation message |
| `MESSAGE_ID` | M | Y: obligatory here, optional everywhere else |
| No `CLASSIFICATION` | — | Y: refused, since table 3-1 does not list it |

### Relative metadata and data, table 3-2

| Feature | Status | Support |
|---|:-:|---|
| `TCA` | M | Y |
| `MISS_DISTANCE` | M | Y: metres |
| `RELATIVE_SPEED`, relative position and velocity in RTN | O | Y |
| Screening period, volume shape, frame and extents | O | Y |
| `SCREEN_ENTRY_TIME`, `SCREEN_EXIT_TIME` | O | Y |
| `COLLISION_PROBABILITY` | O | Y |
| `COLLISION_PROBABILITY_METHOD` obligatory with the probability | C | Y: `ErrMissingKeyword` |

### Object metadata, table 3-3

| Feature | Status | Support |
|---|:-:|---|
| `OBJECT`, one of `OBJECT1` or `OBJECT2` | M | Y: `ErrObjectValue`, `ErrObjectRepeated` |
| `OBJECT_DESIGNATOR`, `CATALOG_NAME`, `OBJECT_NAME`, `INTERNATIONAL_DESIGNATOR` | M | Y |
| `EPHEMERIS_NAME`, `COVARIANCE_METHOD`, `MANEUVERABLE`, `REF_FRAME` | M | Y |
| `OBJECT_TYPE`, operator contact keywords, `ORBIT_CENTER` | O | Y |
| Force model keywords: gravity, atmosphere, n-body, SRP, tides, thrust | O | Y |

### Object data, tables 3-4 to 3-8

| Feature | Status | Support |
|---|:-:|---|
| OD parameters: last observation times, spans, counts, residuals, weighted RMS | O | Y |
| Additional parameters: areas, mass, area over mass, thrust acceleration, SEDR | O | Y |
| State vector, six components | M | Y: km and km/s |
| Covariance, the obligatory 6×6 lower triangle in RTN | M | Y: 21 elements, metres |
| Covariance rows 7 to 9: drag, solar radiation, thrust | O | Y: `CovarianceOrder` reports how many were present |

---

## A3 EXCEPTIONS AND UNSUPPORTED FEATURES

**No conjunction analysis.** Nothing propagates either object, recomputes the
miss distance or relative velocity from the two state vectors, or calculates a
collision probability. Every one of those is the originator's work, reported in
the message. A library that recomputed them would be substituting its own
force models for the originator's, which is the opposite of what a warning is
for.

**Keyword order is not enforced on read.** Clause 6.3.1.9 fixes the order. This
package accepts the keywords of a section in any order and writes them in the
order it read them, so a decoded message re-encodes in the standard's order and
a hand-built one comes out in the order its fields were appended. Refusing an
out-of-order file would refuse messages that are otherwise unambiguous, and the
section boundaries — which do carry meaning — are enforced.

**The covariance is not checked for positive definiteness.** A message whose
matrix is not a valid covariance is structurally valid and is read without
complaint. Whether it is usable is a question for whatever computes a
probability from it.

**Registry values are not checked.** `ORIGINATOR` points at the SANA
organizations registry filtered to the Conjunction Data Message Originator
role, and `CATALOG_NAME` at the SANA `CATALOG_NAME` registry. Neither is
validated: checking would mean shipping a copy of a registry that changes
without this package.

**Enumerated values are not enforced beyond `OBJECT`.** `MANEUVERABLE`,
`COVARIANCE_METHOD`, `OBJECT_TYPE`, `SCREEN_VOLUME_SHAPE` and the rest have
normative value sets. They are carried as written. `Maneuverable` reports
`YES` as true and anything else as false, and reports separately whether the
keyword was present at all, so `N/A` is distinguishable from absent.

---

## A4 IMPLEMENTATION LIMITS

| Limit | Value | Source |
|---|---|---|
| Line length | 254 characters | Clause 6.3.1 |
| Objects per message | exactly 2 | Clause 3.1.2 |
| Covariance order | 6 to 9 | Table 3-8 |
| Keywords per section | bounded by the input | No ceiling imposed |

---

## Wire test vectors

The files backing this statement live in the [vector corpus](https://github.com/ravisuhag/astro/tree/main/vectors/cdm) — 1 decode vector and 1 corpus file.

| File | |
|---|---|
| [`cdm/conjunction.json`](https://github.com/ravisuhag/astro/blob/main/vectors/cdm/conjunction.json) | 1 vector |
| `cdm/obligatory-keywords.kvn` | the clause 3.6.2 example as a readable file |

Both are **published text rather than derived values**: clause 3.6.2 prints the example.

One transcription change is recorded in the vector's note. The Blue Book renders its minus signs as U+2212 and the corpus file uses ASCII hyphen-minus, because clause 6.3.1 allows only printable ASCII — the typography in the PDF is a rendering artefact rather than what a real file holds.

The vector asserts the conjunction, the identity of each object, whether each can manoeuvre, and how many rows of covariance each carried. That last one matters because it is not recoverable from the numbers: an absent row and a row of zeroes are identical in the matrix. The floats are checked in `pkg/cdm` against the same text.

See [`CONTRACT.md`](https://github.com/ravisuhag/astro/blob/main/vectors/CONTRACT.md) for how to consume these, and [how this is verified](/docs/reference/verification) for what rests on a published vector versus a reading of the clause.
