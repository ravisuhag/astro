---
title: Orbit Data Messages
short: ODM
description: "ICS proforma: what this package implements, item by item."
order: 210
---

## Conformance Statement for `pkg/odm`, CCSDS 502.0-B-3

CCSDS 502.0-B-3 annex A ships an Implementation Conformance Statement
proforma. This fills in the Orbit Parameter Message requirements list of
A2.5.1. The requirements lists for the OMM, the OEM and the OCM are not filled
in, because those messages are not implemented.

---

## A2.1 IDENTIFICATION OF ICS

| Field | Value |
|---|---|
| Date of Statement (DD/MM/YYYY) | 04/09/2026 |
| ICS Serial Number | ASTRO-ODM-ICS-001 |
| System Conformance Statement Cross-Reference | This document |

## A2.2 IDENTIFICATION OF IMPLEMENTATION UNDER TEST

| Field | Value |
|---|---|
| Implementation Name | astro/pkg/odm |
| Implementation Version | See `go.mod` / latest commit on `main` |
| Special Configuration | None |
| Other Information | Go library reading and writing the Orbit Parameter Message in 'keyword = value' notation. The OMM, OEM and OCM are not implemented, nor is the XML form of any message. No orbital mechanics: nothing propagates, converts frames, or derives one element set from another. |

## A2.3 IDENTIFICATION OF SUPPLIER

| Field | Value |
|---|---|
| Supplier | Ravi Suhag |
| Contact Point for Queries | GitHub, github.com/ravisuhag/astro |
| Implementation Name(s) and Version(s) | astro/pkg/odm (Go package) |
| System Name(s) | Astro |

## A2.4 DOCUMENT VERSIONS

| Field | Value |
|---|---|
| Specification | CCSDS 502.0-B-3 (Orbit Data Messages, Blue Book, April 2023), also published as ISO 26900 |
| Time formats | CCSDS 301.0-B-4 ASCII time codes A and B, via `pkg/tcf` |
| Have any exceptions been required? | Yes [X] No [ ], see A2.6 |

---

## A2.5.1 Orbit Parameter Message Requirements List

Status is the standard's own: **M** mandatory, **O** optional, **C**
conditional.

| Item | Feature | Keyword | Status | Support |
|--:|---|---|:-:|---|
| 1 | OPM Header | N/A | M | Y |
| 2 | OPM Version | `CCSDS_OPM_VERS` | M | Y: the value is carried as written, not checked against "3.0" |
| 3 | Comment | `COMMENT` | O | Y: header comments only immediately after the version keyword, per clause 7.8.7 |
| 4 | Message classification | `CLASSIFICATION` | O | Y |
| 5 | Message creation date and time | `CREATION_DATE` | M | Y: UTC, per clause 7.5.11, whatever `TIME_SYSTEM` says |
| 6 | Message originator | `ORIGINATOR` | M | Y: not checked against the SANA registry |
| 7 | Unique message identifier | `MESSAGE_ID` | O | Y |
| 8 | OPM Metadata | N/A | M | Y |
| 9 | Comment | `COMMENT` | O | Y |
| 10 | Name of space object | `OBJECT_NAME` | M | Y |
| 11 | Identifier of space object | `OBJECT_ID` | M | Y: not checked against the UN designator index |
| 12 | Orbit center | `CENTER_NAME` | M | Y: not checked against annex B |
| 13 | Reference frame | `REF_FRAME` | M | Y: clause 3.2.3.3 values not enforced, see A2.6 |
| 14 | Epoch of reference frame | `REF_FRAME_EPOCH` | C | Y: read and written when present; the condition is a property of the frame definition, which this package does not model |
| 15 | Time system | `TIME_SYSTEM` | M | Y: clause 3.2.3.2 values not enforced, see A2.6 |
| 16 | OPM Data | N/A | M | Y |
| 17 | State Vector logical block | N/A | M | Y |
| 18 | Comment | `COMMENT` | O | Y |
| 19 | Epoch of the state vector | `EPOCH` | M | Y: both time formats of clause 7.5.10 |
| 20–22 | Position components | `X`, `Y`, `Z` | M | Y |
| 23–25 | Velocity components | `X_DOT`, `Y_DOT`, `Z_DOT` | M | Y |
| 26 | Keplerian Elements block | N/A | O | Y |
| 27 | Comment | `COMMENT` | O | Y |
| 28 | Semi-major axis | `SEMI_MAJOR_AXIS` | C | Y |
| 29 | Eccentricity | `ECCENTRICITY` | C | Y |
| 30 | Inclination | `INCLINATION` | C | Y |
| 31 | Right ascension of ascending node | `RA_OF_ASC_NODE` | C | Y |
| 32 | Argument of pericenter | `ARG_OF_PERICENTER` | C | Y |
| 33 | True or mean anomaly | `TRUE_ANOMALY` or `MEAN_ANOMALY` | C | Y: giving both is refused with `ErrBothAnomalies` |
| 34 | Gravitational coefficient | `GM` | C | Y |
| 35 | Spacecraft Parameters block | N/A | O | Y |
| 36 | Comment | `COMMENT` | O | Y |
| 37 | Mass | `MASS` | C | Y: presence tracked separately from a zero value, since clause 3.2.4.9 makes it mandatory once a manoeuvre is present |
| 38 | Solar radiation area | `SOLAR_RAD_AREA` | C | Y |
| 39 | Solar radiation coefficient | `SOLAR_RAD_COEFF` | C | Y: zero preserved, per clause 3.2.4.5 |
| 40 | Drag area | `DRAG_AREA` | C | Y |
| 41 | Drag coefficient | `DRAG_COEFF` | C | Y: zero preserved, per clause 3.2.4.6 |
| 42 | Position/velocity covariance block | N/A | O | Y |
| 43 | Comment | `COMMENT` | O | Y |
| 44 | Covariance reference frame | `COV_REF_FRAME` | C | Y: omitted when the same as `REF_FRAME`, as table 3-3 allows |
| 45–65 | Covariance matrix, 21 lower triangular elements | `CX_X` … `CZ_DOT_Z_DOT` | C | Y: written in the order clause 3.2.4.10 fixes; the symmetric upper triangle is filled in on decode |
| 66 | Maneuver Parameters block | N/A | O | Y: repeats, per clause 3.2.4.8 |
| 67 | Comment | `COMMENT` | O | Y |
| 68 | Time of maneuver start | `MAN_EPOCH_IGNITION` | O | Y: also what starts a new maneuver block |
| 69 | Duration of maneuver | `MAN_DURATION` | O | Y: zero read as impulsive, per clause 3.2.4.7 |
| 70 | Change of mass | `MAN_DELTA_MASS` | O | Y: a non-negative value is refused, per clause 3.2.4.7 |
| 71 | Maneuver reference frame | `MAN_REF_FRAME` | O | Y: clause 3.2.4.11 values not enforced, see A2.6 |
| 72–74 | Velocity increment components | `MAN_DV_1`, `MAN_DV_2`, `MAN_DV_3` | O | Y |
| 75 | User-Defined Parameters block | N/A | O | Y |
| 76 | User-defined parameter | `USER_DEFINED_x` | O | Y: carried as a name and a raw value; the meaning is an ICD matter |

---

## A2.5.2 to A2.5.4

| Requirements list | Support |
|---|---|
| A2.5.2 Orbit Mean Elements Message | N: not implemented |
| A2.5.3 Orbit Ephemeris Message | N: not implemented |
| A2.5.4 Orbit Comprehensive Message | N: not implemented |

---

## A2.6 EXCEPTIONS AND UNSUPPORTED FEATURES

**Only the OPM is implemented, and only in key-value notation.** The OMM, the
OEM and the OCM are not, and neither is the XML form described in section 8 of
the Blue Book and specified by CCSDS 505.0-B-3. Clause 1.1 leaves the choice of
notation to the exchanging parties, so a partner who sends XML cannot be read
today.

**Enumerated values are not enforced.** Clauses 3.2.3.2, 3.2.3.3 and 3.2.4.11
list the expected values for `TIME_SYSTEM`, `REF_FRAME` and the manoeuvre and
covariance frames, and each says values outside the set "should be documented
in an ICD". Refusing them would refuse conforming messages, so an unrecognised
value is carried through unchanged and the caller decides.

**Registry values are not checked.** `ORIGINATOR`, `OBJECT_NAME`,
`OBJECT_ID` and `CENTER_NAME` point at the SANA registries and the UN Office of
Outer Space Affairs designator index. Checking them would mean shipping a copy
of a registry that changes without this package, and the tables recommend
rather than require those sources.

**Nothing is validated against physics.** Clause 1.2 puts orbit accuracy
outside the standard. A message whose eccentricity exceeds one, or whose
position is inside the central body, is structurally valid and is read without
complaint.

**Clause 7.5.7(b) is not enforced on read.** The sub-clause requires a
floating-point mantissa to carry its decimal point in the second position, so
`1.5E+03` conforms and `15.0E+02` does not. Real messages break this
constantly. Reading is lenient; writing produces the conforming form.

**Re-encoding does not reproduce the input octets.** Clause 7.5.6 makes
trailing zeroes optional and clauses 7.4.5 to 7.4.7 make the surrounding white
space insignificant, so one message has many spellings. Values round-trip;
spelling does not.

---

## A2.7 IMPLEMENTATION LIMITS

| Limit | Value | Source |
|---|---|---|
| Line length | 254 characters | Clause 7.3.2. Clause 7.3.3 exempts the OCM, which is not implemented. |
| Integer range | −2 147 483 648 to 2 147 483 647 | Clause 7.5.4 |
| Digits in a non-integer value | 16 | Clauses 7.5.6 and 7.5.7 |
| Line terminators accepted | CR, LF, CR/LF, LF/CR | Clause 7.3.7 |
| Character set | Printable ASCII and blanks | Clause 7.3.4 |
| Maneuvers per message | bounded by the input | No ceiling is imposed |

---

## Wire test vectors

The files backing this statement live in the [vector corpus](https://github.com/ravisuhag/astro/tree/main/vectors/odm) — 2 decode vectors and 2 corpus files.

| File | |
|---|---|
| [`odm/opm.json`](https://github.com/ravisuhag/astro/blob/main/vectors/odm/opm.json) | 2 vectors |
| `odm/opm-simple.kvn`, `odm/opm-maneuvers.kvn` | the annex G examples as readable files |

Both are **published text rather than derived values**: annex G of the Blue Book prints them, so a second working group wrote them.

The vectors assert the text and integer content — version, originator, object identifiers, frames, epoch, maneuver count. The numeric state vector is not asserted there, because a vector field has no float accessor and pinning floats as formatted strings would test this package's number formatting rather than the standard. Those values are checked in `pkg/odm` against the same published text.

See [`CONTRACT.md`](https://github.com/ravisuhag/astro/blob/main/vectors/CONTRACT.md) for how to consume these, and [how this is verified](/docs/reference/verification) for what rests on a published vector versus a reading of the clause.
