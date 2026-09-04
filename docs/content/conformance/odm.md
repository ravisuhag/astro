---
title: Orbit Data Messages
short: ODM
description: "ICS proforma: what this package implements, item by item."
order: 210
---

## Conformance Statement for `pkg/odm`, CCSDS 502.0-B-3

CCSDS 502.0-B-3 annex A ships an Implementation Conformance Statement
proforma. This fills in the Orbit Parameter Message requirements list of
A2.5.1, the Orbit Mean Elements Message list of A2.5.2, and the Orbit
Ephemeris Message list of A2.5.3. The list for the OCM is not filled in,
because that message is not implemented.

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
| Other Information | Go library reading and writing the Orbit Parameter Message, the Orbit Mean-Elements Message and the Orbit Ephemeris Message in 'keyword = value' notation. The OCM is not implemented, nor is the XML form of any message. No orbital mechanics: nothing propagates, converts frames, interpolates, or derives one element set from another. |

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

## A2.5.2 Orbit Mean Elements Message Requirements List

The three paired slots of table 4-3 are the part of this list worth reading.
Each accepts two keyword names, which name applies is decided by
`MEAN_ELEMENT_THEORY`, and the two halves carry different units.

| Item | Feature | Keyword | Status | Support |
|--:|---|---|:-:|---|
| — | OMM Header | `CCSDS_OMM_VERS`, `COMMENT`, `CLASSIFICATION`, `CREATION_DATE`, `ORIGINATOR`, `MESSAGE_ID` | M/O | Y: same table as the OPM's 3-1 |
| — | Metadata | `OBJECT_NAME`, `OBJECT_ID`, `CENTER_NAME`, `REF_FRAME`, `TIME_SYSTEM` | M | Y |
| — | Epoch of reference frame | `REF_FRAME_EPOCH` | C | Y |
| — | Mean element theory | `MEAN_ELEMENT_THEORY` | M | Y: also decides which paired keywords apply |
| — | Epoch | `EPOCH` | M | Y |
| — | Orbit size | `SEMI_MAJOR_AXIS` or `MEAN_MOTION` | M | Y: which arrived is recorded; both is refused |
| — | Eccentricity, inclination, RAAN, argument of pericenter, mean anomaly | `ECCENTRICITY` … `MEAN_ANOMALY` | M | Y |
| — | Gravitational coefficient | `GM` | O | Y: optional here, unlike the OPM |
| — | Spacecraft parameters | `MASS` … `DRAG_COEFF` | O | Y: same block as table 3-3 |
| — | TLE block | `EPHEMERIS_TYPE`, `CLASSIFICATION_TYPE`, `NORAD_CAT_ID`, `ELEMENT_SET_NO`, `REV_AT_EPOCH` | O | Y: defaults of 0 and "U" applied per clause 4.2.4.7 |
| — | Drag term | `BSTAR` or `BTERM` | C | Y: which arrived is recorded; both is refused |
| — | First derivative of mean motion | `MEAN_MOTION_DOT` | C | Y |
| — | Second derivative or solar radiation | `MEAN_MOTION_DDOT` or `AGOM` | C | Y: which arrived is recorded; both is refused |
| — | Covariance matrix | `COV_REF_FRAME`, `CX_X` … `CZ_DOT_Z_DOT` | O | Y: 21 named keywords, unlike the OEM's positional rows |
| — | User-defined parameters | `USER_DEFINED_x` | O | Y |

The four conventions clause 4.2.4.6 fixes for a TLE-based OMM — `EARTH`,
`TEME`, `UTC` and `MEAN_MOTION` — are enforced, and clause 4.2.4.9's converse
rule that `TEME` may be used for nothing else is enforced too.

---

## A2.5.3 Orbit Ephemeris Message Requirements List

| Item | Feature | Keyword | Status | Support |
|--:|---|---|:-:|---|
| 1 | OEM Header | N/A | M | Y |
| 2 | OEM Version | `CCSDS_OEM_VERS` | M | Y |
| 3 | Comment | `COMMENT` | O | Y: header comments only immediately after the version keyword |
| 4 | Message classification | `CLASSIFICATION` | O | Y |
| 5 | Message creation date and time | `CREATION_DATE` | M | Y |
| 6 | Message originator | `ORIGINATOR` | M | Y |
| 7 | Unique message identifier | `MESSAGE_ID` | O | Y |
| 8 | Metadata logical block | N/A | M | Y: several per message, per clause 5.2.3.3 |
| 9 | Start of OEM Metadata | `META_START` | M | Y |
| 10 | Comment | `COMMENT` | O | Y |
| 11 | Name of space object | `OBJECT_NAME` | M | Y |
| 12 | Identifier of space object | `OBJECT_ID` | M | Y |
| 13 | Orbit center | `CENTER_NAME` | M | Y: may be a spacecraft, which table 5-3 allows and the OPM's table does not |
| 14 | Reference frame | `REF_FRAME` | M | Y: clause 3.2.3.3 values not enforced, see A2.6 |
| 15 | Epoch of reference frame | `REF_FRAME_EPOCH` | C | Y |
| 16 | Time system | `TIME_SYSTEM` | M | Y: a change between groups is refused, per clause 5.2.4.5 |
| 17 | Start of TOTAL time span | `START_TIME` | M | Y |
| 18 | Start of useable span | `USEABLE_START_TIME` | O | Y: read and preserved, never used to trim, see A2.6 |
| 19 | End of useable span | `USEABLE_STOP_TIME` | O | Y: as above |
| 20 | End of TOTAL time span | `STOP_TIME` | M | Y |
| 21 | Recommended interpolation method | `INTERPOLATION` | O | Y: carried, not acted on, see A2.6 |
| 22 | Recommended interpolation degree | `INTERPOLATION_DEGREE` | C | Y: absence alongside a method is refused, per table 5-3 |
| 23 | End of OEM Metadata | `META_STOP` | M | Y |
| 24 | OEM Data logical block | N/A | M | Y |
| 25 | Ephemeris data lines | positional | M | Y: 7 or 10 fields, per clauses 5.2.4.1 and 5.2.4.2 |
| 26 | OEM Covariance logical block | N/A | O | Y |
| 27 | Start of OEM Covariance | `COVARIANCE_START` | M | Y |
| 28 | Epoch of the covariance | `EPOCH` | C | Y: required, since it is what separates one matrix from the next |
| 29 | Reference frame of the covariance | `COV_REF_FRAME` | C | Y: omitted when the same as the ephemeris frame |
| 30 | Covariance matrix lines | positional | O | Y: 21 lower triangular values, over any number of lines |
| 31 | End of OEM Covariance | `COVARIANCE_STOP` | M | Y |

---

## A2.5.4

| Requirements list | Support |
|---|---|
| A2.5.4 Orbit Comprehensive Message | N: not implemented |

---

## A2.6 EXCEPTIONS AND UNSUPPORTED FEATURES

**Only the OPM, the OMM and the OEM are implemented, and only in key-value
notation.** The OCM is not, and neither is the XML form described in section 8
of the Blue Book and specified by CCSDS 505.0-B-3. Clause 1.1 leaves the choice
of notation to the exchanging parties, so a partner who sends XML cannot be
read today.

**Interpolation is not performed.** `INTERPOLATION` and
`INTERPOLATION_DEGREE` are read, preserved and reported, and nothing here acts
on them. Interpolating an ephemeris is orbital mechanics, and clause 5.2.4.6
attaches a rule to it this package cannot enforce on a caller's behalf: a
consumer must not interpolate across a metadata group boundary. `OEM.Blocks`
keeps the groups separate so that a caller can respect that; whether it does is
the caller's business.

**The useable span is not applied.** `USEABLE_START_TIME` and
`USEABLE_STOP_TIME` are read and preserved. Records outside them are not
dropped, because table 5-3 makes those bounds advice to the consumer about
where a producer padded the data with fictitious interpolation nodes, and
silently discarding records would change what the file says.

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

**The TLE derivative scaling is not applied.** Note 2 under clause 4.2.4.7
says that if `MEAN_MOTION_DOT` and `MEAN_MOTION_DDOT` came from a TLE, or are
intended to be used as one, they must be divided by 2 and 6 respectively to
match the SGP Taylor series terms. Nothing in a message records whether that
has been done, so this package carries the values as written and leaves the
question to the interface control document.

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
| Ephemeris records per message | bounded by the input | No ceiling is imposed; records are read into memory rather than streamed |
| Metadata groups per OEM | bounded by the input | No ceiling is imposed |

---

## Wire test vectors

The files backing this statement live in the [vector corpus](https://github.com/ravisuhag/astro/tree/main/vectors/odm) — 6 decode vectors and 6 corpus files.

| File | |
|---|---|
| [`odm/messages.json`](https://github.com/ravisuhag/astro/blob/main/vectors/odm/messages.json) | 6 vectors |
| `odm/opm-*.kvn`, `odm/omm-*.kvn`, `odm/oem-*.kvn` | the annex G examples as readable files |

Both are **published text rather than derived values**: annex G of the Blue Book prints them, so a second working group wrote them.

The vectors assert the text and integer content — version, originator, object identifiers, frames, epoch, and the counts. For an OEM the counts carry most of the weight: how many metadata groups, how many records, whether a record has acceleration, and how many covariance matrices are what a consumer must read correctly before any single number matters. The numeric state vector is not asserted there, because a vector field has no float accessor and pinning floats as formatted strings would test this package's number formatting rather than the standard. Those values are checked in `pkg/odm` against the same published text.

See [`CONTRACT.md`](https://github.com/ravisuhag/astro/blob/main/vectors/CONTRACT.md) for how to consume these, and [how this is verified](/docs/reference/verification) for what rests on a published vector versus a reading of the clause.
