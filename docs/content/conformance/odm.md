---
title: Orbit Data Messages
short: ODM
description: "ICS proforma: what this package implements, item by item."
order: 210
---

## Conformance Statement for `pkg/odm`, CCSDS 502.0-B-3

CCSDS 502.0-B-3 annex A ships an Implementation Conformance Statement
proforma. This fills in all four requirements lists: the Orbit Parameter
Message of A2.5.1, the Orbit Mean Elements Message of A2.5.2, the Orbit
Ephemeris Message of A2.5.3 and the Orbit Comprehensive Message of A2.5.4.

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
| Other Information | Go library reading and writing all four orbit data messages, in both the 'keyword = value' notation of section 7 and the XML form of section 8. No orbital mechanics: nothing propagates, converts frames, interpolates, or derives one element set from another. |

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

## A2.5 XML FORM

| Feature | Reference | Status | Support |
|---|---|:-:|---|
| Root element and its id and version attributes | 8.3, 8.8.2–8.8.4 | M | Y: one per message type |
| Schema instance namespace, exactly as given | 505.0-B-3 clause 4.3.3 | M | Y: http, not https — the string names a namespace |
| Header, body, segment, metadata, data | 505.0-B-3 clauses 3.2–3.4 | M | Y: shared across all four navigation packages |
| One segment for OPM, OMM and OCM | 505.0-B-3 clause 3.3 | M | Y: the OCM has one metadata section, per clause 6.2.4.3 |
| One or more segments for OEM | 505.0-B-3 clause 3.4 | M | Y |
| Keyword tags in upper case | 8.10.9 | M | Y |
| Units as an attribute, matching section 5 | 8.10.10, 8.10.11 | O | Y |
| Block elements for the logical blocks | 8.8.12–8.8.15, 8.9, 8.10.13 | M | Y |
| Each ephemeris line as a named `stateVector` | 8.10.14 | M | Y |
| OEM covariance with the OPM's named keywords | 8.10.19 | M | Y |
| `USER_DEFINED` with the name in an attribute | 8.10, annex G | O | Y: a parameter with an empty name attribute is refused, since its key-value form would be the bare `USER_DEFINED_` |
| OCM section tags and data line tags | 8.11.13, table 8-9 | M | Y: `traj`, `phys`, `cov`, `man`, `pert`, `od`, `user` and their `trajLine`, `covLine`, `manLine` |
| OCM data lines as `xsd:string` | 8.11.15 | M | Y: kept as rows, split by the reader, as the clause intends |
| NDM combined instantiation | 8.12 | O | Y: implemented by [`pkg/ndm`](/conformance/ndm), since a combined file may hold messages from other standards too |

---

## A2.5.1 Orbit Parameter Message Requirements List

Status is the standard's own: **M** mandatory, **O** optional, **C**
conditional.

| Item | Feature | Keyword | Status | Support |
|--:|---|---|:-:|---|
| ODM-1 | OPM Header | N/A | M | Y |
| ODM-2 | OPM Version | `CCSDS_OPM_VERS` | M | Y: the value is carried as written, not checked against "3.0" |
| ODM-3 | Comment | `COMMENT` | O | Y: header comments only immediately after the version keyword, per clause 7.8.7 |
| ODM-4 | Message classification | `CLASSIFICATION` | O | Y |
| ODM-5 | Message creation date and time | `CREATION_DATE` | M | Y: UTC, per clause 7.5.11, whatever `TIME_SYSTEM` says |
| ODM-6 | Message originator | `ORIGINATOR` | M | Y: not checked against the SANA registry |
| ODM-7 | Unique message identifier | `MESSAGE_ID` | O | Y |
| ODM-8 | OPM Metadata | N/A | M | Y |
| ODM-9 | Comment | `COMMENT` | O | Y |
| ODM-10 | Name of space object | `OBJECT_NAME` | M | Y |
| ODM-11 | Identifier of space object | `OBJECT_ID` | M | Y: not checked against the UN designator index |
| ODM-12 | Orbit center | `CENTER_NAME` | M | Y: not checked against annex B |
| ODM-13 | Reference frame | `REF_FRAME` | M | Y: clause 3.2.3.3 values not enforced, see A2.6 |
| ODM-14 | Epoch of reference frame | `REF_FRAME_EPOCH` | C | Y: read and written when present; the condition is a property of the frame definition, which this package does not model |
| ODM-15 | Time system | `TIME_SYSTEM` | M | Y: clause 3.2.3.2 values not enforced, see A2.6 |
| ODM-16 | OPM Data | N/A | M | Y |
| ODM-17 | State Vector logical block | N/A | M | Y |
| ODM-18 | Comment | `COMMENT` | O | Y |
| ODM-19 | Epoch of the state vector | `EPOCH` | M | Y: both time formats of clause 7.5.10 |
| ODM-20 | Position components | `X`, `Y`, `Z` | M | Y |
| ODM-21 | Velocity components | `X_DOT`, `Y_DOT`, `Z_DOT` | M | Y |
| ODM-22 | Keplerian Elements block | N/A | O | Y |
| ODM-23 | Comment | `COMMENT` | O | Y |
| ODM-24 | Semi-major axis | `SEMI_MAJOR_AXIS` | C | Y |
| ODM-25 | Eccentricity | `ECCENTRICITY` | C | Y |
| ODM-26 | Inclination | `INCLINATION` | C | Y |
| ODM-27 | Right ascension of ascending node | `RA_OF_ASC_NODE` | C | Y |
| ODM-28 | Argument of pericenter | `ARG_OF_PERICENTER` | C | Y |
| ODM-29 | True or mean anomaly | `TRUE_ANOMALY` or `MEAN_ANOMALY` | C | Y: giving both is refused with `ErrBothAnomalies` |
| ODM-30 | Gravitational coefficient | `GM` | C | Y |
| ODM-31 | Spacecraft Parameters block | N/A | O | Y |
| ODM-32 | Comment | `COMMENT` | O | Y |
| ODM-33 | Mass | `MASS` | C | Y: presence tracked separately from a zero value, since clause 3.2.4.9 makes it mandatory once a manoeuvre is present |
| ODM-34 | Solar radiation area | `SOLAR_RAD_AREA` | C | Y |
| ODM-35 | Solar radiation coefficient | `SOLAR_RAD_COEFF` | C | Y: zero preserved, per clause 3.2.4.5 |
| ODM-36 | Drag area | `DRAG_AREA` | C | Y |
| ODM-37 | Drag coefficient | `DRAG_COEFF` | C | Y: zero preserved, per clause 3.2.4.6 |
| ODM-38 | Position/velocity covariance block | N/A | O | Y |
| ODM-39 | Comment | `COMMENT` | O | Y |
| ODM-40 | Covariance reference frame | `COV_REF_FRAME` | C | Y: omitted when the same as `REF_FRAME`, as table 3-3 allows |
| ODM-41 | Covariance matrix, 21 lower triangular elements | `CX_X` … `CZ_DOT_Z_DOT` | C | Y: written in the order clause 3.2.4.10 fixes; the symmetric upper triangle is filled in on decode |
| ODM-42 | Maneuver Parameters block | N/A | O | Y: repeats, per clause 3.2.4.8 |
| ODM-43 | Comment | `COMMENT` | O | Y |
| ODM-44 | Time of maneuver start | `MAN_EPOCH_IGNITION` | O | Y: also what starts a new maneuver block |
| ODM-45 | Duration of maneuver | `MAN_DURATION` | O | Y: zero read as impulsive, per clause 3.2.4.7 |
| ODM-46 | Change of mass | `MAN_DELTA_MASS` | O | Y: a non-negative value is refused, per clause 3.2.4.7 |
| ODM-47 | Maneuver reference frame | `MAN_REF_FRAME` | O | Y: clause 3.2.4.11 values not enforced, see A2.6 |
| ODM-48 | Velocity increment components | `MAN_DV_1`, `MAN_DV_2`, `MAN_DV_3` | O | Y |
| ODM-49 | User-Defined Parameters block | N/A | O | Y |
| ODM-50 | User-defined parameter | `USER_DEFINED_x` | O | Y: carried as a name and a raw value; the meaning is an ICD matter |

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
| ODM-51 | OEM Header | N/A | M | Y |
| ODM-52 | OEM Version | `CCSDS_OEM_VERS` | M | Y |
| ODM-53 | Comment | `COMMENT` | O | Y: header comments only immediately after the version keyword |
| ODM-54 | Message classification | `CLASSIFICATION` | O | Y |
| ODM-55 | Message creation date and time | `CREATION_DATE` | M | Y |
| ODM-56 | Message originator | `ORIGINATOR` | M | Y |
| ODM-57 | Unique message identifier | `MESSAGE_ID` | O | Y |
| ODM-58 | Metadata logical block | N/A | M | Y: several per message, per clause 5.2.3.3 |
| ODM-59 | Start of OEM Metadata | `META_START` | M | Y |
| ODM-60 | Comment | `COMMENT` | O | Y |
| ODM-61 | Name of space object | `OBJECT_NAME` | M | Y |
| ODM-62 | Identifier of space object | `OBJECT_ID` | M | Y |
| ODM-63 | Orbit center | `CENTER_NAME` | M | Y: may be a spacecraft, which table 5-3 allows and the OPM's table does not |
| ODM-64 | Reference frame | `REF_FRAME` | M | Y: clause 3.2.3.3 values not enforced, see A2.6 |
| ODM-65 | Epoch of reference frame | `REF_FRAME_EPOCH` | C | Y |
| ODM-66 | Time system | `TIME_SYSTEM` | M | Y: a change between groups is refused, per clause 5.2.4.5 |
| ODM-67 | Start of TOTAL time span | `START_TIME` | M | Y |
| ODM-68 | Start of useable span | `USEABLE_START_TIME` | O | Y: read and preserved, never used to trim, see A2.6 |
| ODM-69 | End of useable span | `USEABLE_STOP_TIME` | O | Y: as above |
| ODM-70 | End of TOTAL time span | `STOP_TIME` | M | Y |
| ODM-71 | Recommended interpolation method | `INTERPOLATION` | O | Y: carried, not acted on, see A2.6 |
| ODM-72 | Recommended interpolation degree | `INTERPOLATION_DEGREE` | C | Y: absence alongside a method is refused, per table 5-3 |
| ODM-73 | End of OEM Metadata | `META_STOP` | M | Y |
| ODM-74 | OEM Data logical block | N/A | M | Y |
| ODM-75 | Ephemeris data lines | positional | M | Y: 7 or 10 fields, per clauses 5.2.4.1 and 5.2.4.2 |
| ODM-76 | OEM Covariance logical block | N/A | O | Y |
| ODM-77 | Start of OEM Covariance | `COVARIANCE_START` | M | Y |
| ODM-78 | Epoch of the covariance | `EPOCH` | C | Y: required, since it is what separates one matrix from the next |
| ODM-79 | Reference frame of the covariance | `COV_REF_FRAME` | C | Y: omitted when the same as the ephemeris frame |
| ODM-80 | Covariance matrix lines | positional | O | Y: 21 lower triangular values, over any number of lines |
| ODM-81 | End of OEM Covariance | `COVARIANCE_STOP` | M | Y |

---

## A2.5.4 Orbit Comprehensive Message Requirements List

The OCM's eight sections hold something over two hundred keywords, so this
lists them by section rather than one row apiece. The full tables are in
`pkg/odm/ocm_keywords.go`, in the order the Blue Book prints them, which is
also the order clause 6.2.2.1 requires them to arrive in.

| Item | Feature | Keyword | Status | Support |
|--:|---|---|:-:|---|
| — | OCM Header | `CCSDS_OCM_VERS`, `COMMENT`, `CLASSIFICATION`, `CREATION_DATE`, `ORIGINATOR`, `MESSAGE_ID` | M/O | Y: table 6-2, in the order clause 6.2.3.3 fixes |
| — | Metadata section | `META_START` … `META_STOP` | M | Y: one only, per clause 6.2.4.3 |
| — | Metadata keywords | 48 keywords of table 6-3 | M/O/C | Y: only `EPOCH_TZERO` is mandatory with no default; the rest are optional or default, per clause 6.2.1.3 |
| — | Spacecraft clock keywords | `SCLK_OFFSET_AT_EPOCH`, `SCLK_SEC_PER_SI_SEC` | C | Y: required when `TIME_SYSTEM` is `SCLK`, as table 6-3 states |
| — | Trajectory sections | `TRAJ_START` … `TRAJ_STOP` | O | Y: any number, per clause 6.2.5.3 |
| — | Trajectory keywords | 18 keywords of table 6-4 | M/O/C | Y: `CENTER_NAME`, `TRAJ_REF_FRAME` and `TRAJ_TYPE` default to `EARTH`, `ICRF3` and `CARTPV` |
| — | Trajectory data lines | positional | M | Y: kept as text fields; the width comes from `TRAJ_TYPE`, see A2.6 |
| — | Physical characteristics section | `PHYS_START` … `PHYS_STOP` | O | Y: one only, per clause 6.2.6.2 |
| — | Physical characteristics keywords | 50 keywords of table 6-5 | O/C | Y |
| — | Covariance sections | `COV_START` … `COV_STOP` | O | Y: any number, per clause 6.2.7.3 |
| — | Covariance keywords | 13 keywords of table 6-6 | M/O/C | Y: `COV_REF_FRAME`, `COV_TYPE` and `COV_ORDERING` default to `TNW_INERTIAL`, `CARTPV` and `LTM` |
| — | Covariance data lines | positional | M | Y: one matrix per line, per clause 6.2.7.12; `CovMatrix` folds a line back into a square matrix |
| — | Covariance orderings | `LTM`, `UTM`, `FULL`, `LTMWCC`, `UTMWCC` | M | Y: all five of clause 6.2.7.12.3; the two `WCC` forms are returned unmirrored, see A2.6 |
| — | Manoeuvre sections | `MAN_START` … `MAN_STOP` | O | Y: any number, per clause 6.2.8.4 |
| — | Manoeuvre keywords | 30 keywords of table 6-7 | M/O/C | Y: `MAN_ID`, `MAN_DEVICE_ID` and `MAN_COMPOSITION` are required; `DC_TYPE` defaults to `CONTINUOUS` |
| — | Manoeuvre composition | `MAN_COMPOSITION` | M | Y: names from tables 6-8 and 6-9, not commingled (6.2.8.15), in the fixed order (6.2.8.16), starting with one time tag (6.2.8.18) |
| — | Manoeuvre data lines | positional | M | Y: the field count must match `MAN_COMPOSITION` |
| — | Perturbations section | `PERT_START` … `PERT_STOP` | C | Y: one only (6.2.9.2), and required alongside an orbit determination section (6.2.10.5) |
| — | Perturbations keywords | 29 keywords of table 6-10 | O | Y |
| — | Orbit determination section | `OD_START` … `OD_STOP` | O | Y: one only, per clause 6.2.10.2 |
| — | Orbit determination keywords | 29 keywords of table 6-11 | M/O | Y: `OD_ID`, `OD_METHOD` and `OD_EPOCH` are required |
| — | User-defined section | `USER_START` … `USER_STOP` | O | Y: one only (6.2.11.2), and at least one parameter, as table 6-12 requires |
| — | User-defined parameter | `USER_DEFINED_x` | M | Y: within the section |
| — | Section order | table 6-1 | M | Y: a section out of order is refused |
| — | Keyword order within a section | 6.2.2.1 | M | Y: a keyword out of its table's order is refused |
| — | Relative or absolute time tags | 6.2.2.3 | M | Y: `DataRow.TimeTag` resolves either against `EPOCH_TZERO` |
| — | No duplicate time tags in a block | 6.2.2.4 | M | Y |
| — | One time tag kind per block | 6.2.2.5 | M | Y |
| — | Monotonic time in trajectory and covariance blocks | 6.2.5.6, 6.2.7.6 | M | Y: manoeuvre blocks are exempt, since clause 6.2.8.5 lets them overlap |
| — | A message with no data blocks | 6.2.1.1 note | O | Y: valid, and read as such |

---

## A2.6 EXCEPTIONS AND UNSUPPORTED FEATURES

Both notations are implemented for all four messages: the key-value form of
section 7 and the XML form of section 8, with the structure of
CCSDS 505.0-B-3. Clause 1.1 leaves the choice to the exchanging parties, so
both are needed.

**An OCM row's width is not checked, except for a manoeuvre.** A trajectory
row's columns come from `TRAJ_TYPE` and a covariance row's from `COV_TYPE`,
and clauses 6.2.5.11 and 6.2.7.12.1 draw both from the SANA registry rather
than from the Blue Book. Nothing here says how many numbers a `CARTPV` row
holds, so the rows are carried as text fields and the caller reads the columns.
The exception is `MAN_COMPOSITION`, whose field names are printed in tables
6-8 and 6-9: those are checked, and a manoeuvre row whose width disagrees with
its composition is refused.

The Blue Book's own figure G-15 shows why this cannot be tightened. It leaves
`TRAJ_TYPE` out, so the default `CARTPV` applies, and then prints rows of nine
values — a position, a velocity and an acceleration, which is `CARTPVA`. A
reader that trusted the registry would have to refuse a published example.

**The `LTMWCC` and `UTMWCC` covariance matrices are not made symmetric.**
Clauses 6.2.7.12.3.4 and 6.2.7.12.3.5 put correlations rather than covariances
in one triangle of each, so the matrix is not symmetric. `CovMatrix` returns
those two as they were written. Mirroring them the way an `LTM` is mirrored
would silently scale half the entries by the product of two standard
deviations.

**An OCM's keywords are not typed.** Its sections are held as ordered keyword
lists with typed accessors for the keywords that change how the data must be
read. There are over two hundred keywords, most optional and most drawn from
the SANA registry, so there is nothing to parse a value into and no way for a
caller to see an unfamiliar keyword if it were dropped. `Get` reaches anything;
the values are carried as text.

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
| Line length, OPM, OMM and OEM | 254 characters | Clause 7.3.2 |
| Line length, OCM | unbounded | Clause 7.3.3 exempts the OCM outright, and a 6x6 covariance matrix is 21 numbers that clause 6.2.7.12 requires on one line |
| Integer range | −2 147 483 648 to 2 147 483 647 | Clause 7.5.4 |
| Digits in a non-integer value | 16 | Clauses 7.5.6 and 7.5.7 |
| Line terminators accepted | CR, LF, CR/LF, LF/CR | Clause 7.3.7 |
| Character set | Printable ASCII and blanks | Clause 7.3.4 |
| Maneuvers per message | bounded by the input | No ceiling is imposed |
| Ephemeris records per message | bounded by the input | No ceiling is imposed; records are read into memory rather than streamed |
| Metadata groups per OEM | bounded by the input | No ceiling is imposed |
| Data sections per OCM | bounded by the input | No ceiling is imposed on the repeating sections; the rest are limited to one by the standard |
| Covariance matrix size in an OCM | derived from the row | The dimension comes from how many values the row holds under its `COV_ORDERING` |

---

## Wire test vectors

The files backing this statement live in the [vector corpus](https://github.com/ravisuhag/astro/tree/main/vectors/odm) — 11 decode vectors and 11 corpus files.

| File | |
|---|---|
| [`odm/messages.json`](https://github.com/ravisuhag/astro/blob/main/vectors/odm/messages.json) | 11 vectors |
| `odm/opm-*.kvn`, `odm/omm-*.kvn`, `odm/oem-*.kvn`, `odm/ocm-*.kvn` | the annex G examples as readable files |
| `odm/ocm-xml.xml` | figure G-20, the same OCM in the XML form of section 8 |

Both are **published text rather than derived values**: annex G of the Blue Book prints them, so a second working group wrote them.

The vectors assert the text and integer content — version, originator, object identifiers, frames, epoch, and the counts. For the OEM and the OCM the counts carry most of the weight: how many metadata groups, how many records, whether a record has acceleration, and how many covariance matrices are what a consumer must read correctly before any single number matters. The same goes for an OCM, whose section shape is what says how to read its rows at all, and whose vectors also assert the defaults clause 6.2.1.3 lets a producer leave out. The numeric state vector is not asserted there, because a vector field has no float accessor and pinning floats as formatted strings would test this package's number formatting rather than the standard. Those values are checked in `pkg/odm` against the same published text.

See [`CONTRACT.md`](https://github.com/ravisuhag/astro/blob/main/vectors/CONTRACT.md) for how to consume these, and [how this is verified](/docs/reference/verification) for what rests on a published vector versus a reading of the clause.
