---
title: Attitude Data Messages
short: ADM
description: "Coverage matrix: what this package implements, clause by clause."
order: 220
---

## Conformance Statement for `pkg/adm`, CCSDS 504.0-B-2

All three messages are implemented: the Attitude Parameter Message of
section 3, the Attitude Ephemeris Message of section 4 and the Attitude
Comprehensive Message of section 5, each in both the key-value notation of
section 6 and the XML form of section 7.

---

## A1 IDENTIFICATION

| Field | Value |
|---|---|
| Date of Statement (DD/MM/YYYY) | 04/09/2026 |
| ICS Serial Number | ASTRO-ADM-ICS-001 |
| Implementation Name | astro/pkg/adm |
| Implementation Version | See `go.mod` / latest commit on `main` |
| Supplier | Ravi Suhag |
| Contact Point for Queries | GitHub, github.com/ravisuhag/astro |
| Specification | CCSDS 504.0-B-2 (Attitude Data Messages, Blue Book, January 2024) |
| Time formats | CCSDS 301.0-B-4 ASCII time codes A and B, via `pkg/tcf` |
| Have any exceptions been required? | Yes [X] No [ ], see A3 |

---

## A2 REQUIREMENTS

### APM, section 3

| Feature | Reference | Status | Support |
|---|---|:-:|---|
| Header keywords | Table 3-1 | M/O | Y: including `CLASSIFICATION`, which the TDM's table lacks |
| Metadata: `OBJECT_NAME`, `OBJECT_ID`, `TIME_SYSTEM` | Table 3-2 | M | Y |
| Metadata: `CENTER_NAME` | Table 3-2 | O | Y: optional here, unlike the orbit messages |
| No metadata delimiters | Table 3-2 | — | Y: the section ends at `EPOCH` |
| Only the keywords of table 3-2 in metadata | 3.2.3.2 | M | Y |
| `EPOCH` | Table 3-3 | M | Y |
| Quaternion block, `QUAT_START`/`QUAT_STOP` | Table 3-3 | O | Y: `Q1`…`QC` mandatory, the four derivatives optional as a group |
| Euler block, `EULER_START`/`EULER_STOP` | Table 3-3 | O | Y: `EULER_ROT_SEQ` and the three angles mandatory, the rates optional |
| Angular velocity block | Table 3-3 | O | Y |
| Spin block | Table 3-3 | O | Y: nutation and momentum groups each conditional as a whole |
| Inertia block | Table 3-3 | O | Y |
| Maneuver block, repeatable | Table 3-3, 3.2.4 | O | Y: a second `MAN_START` appends |
| All mandatory elements present if a block is present | Table 3-3 headings | M | Y: `ErrMissingKeyword` |
| A stop must match its start | Table 3-3 | M | Y: `ErrUnexpectedDelimiter` |
| Only the keywords of table 3-3 in data | 3.2.4.2 | M | Y: checked per block, so an inertia keyword inside a quaternion block is refused |

### AEM, section 4

| Feature | Reference | Status | Support |
|---|---|:-:|---|
| Header keywords | Table 4-1 | M/O | Y |
| `META_START` and `META_STOP` around each metadata group | Table 4-3 | M | Y |
| `DATA_START` and `DATA_STOP` around each data block | Table 4-3 | M | Y |
| One or more segments | 4.1 | M | Y: `ErrNoSegment` |
| Metadata: object, frames, time system, span | Table 4-3 | M | Y: `REF_FRAME_A` and `REF_FRAME_B` both required |
| `USEABLE_START_TIME`, `USEABLE_STOP_TIME` | Table 4-3 | O | Y: read and preserved, never used to trim, see A3 |
| `ATTITUDE_TYPE`, one of nine values | Table 4-3, 4-4 | M | Y: `ErrUnknownAttitudeType` |
| `EULER_ROT_SEQ` when the type is Euler | Table 4-3 | C | Y: `ErrEulerRotSeqMissing` |
| `ANGVEL_FRAME` when the type pairs an attitude with angular velocity | Table 4-3 | C | Y: read and preserved, not required |
| `INTERPOLATION_DEGREE` mandatory with `INTERPOLATION_METHOD` | Table 4-3 | C | Y |
| Data line width per attitude type | Table 4-4 | M | Y: every line checked; `ErrAttitudeLineFields` |
| Positional data lines, space separated | 4.2 | M | Y |

### ACM, section 5

The ACM's six sections hold something over a hundred keywords, so this lists
them by section rather than one row apiece. The full tables are in
`pkg/adm/acm_keywords.go`, in the order the Blue Book prints them, which is
also the order clauses 5.3.3.5 and 5.3.4.1 require them to arrive in.

| Feature | Reference | Status | Support |
|---|---|:-:|---|
| Header keywords | Table 5-2 | M/O | Y: the same six as the APM's table 3-1 |
| Metadata section, one only | 5.3.3.4 | M | Y |
| Metadata keywords | Table 5-3 | M/O | Y: `OBJECT_NAME`, `TIME_SYSTEM` and `EPOCH_TZERO` are mandatory |
| Section order | Table 5-1, 5.3.1.2 | M | Y: a section out of order is refused |
| Keyword order within a section | 5.3.3.5, 5.3.4.1 | M | Y: a keyword out of its table's order is refused |
| Attitude state sections | Table 5-4, 5.3.5.4 | O | Y: any number |
| `ATT_TYPE` and `RATE_TYPE` element counts | Annex B4 | M | Y: `QUATERNION` 4, `EULER_ANGLES` 3, `DCM` 9; `ANGVEL` 3, `Q_DOT` 4, `EULER_RATE` 3, `GYRO_BIAS` 3, `NONE` 0 |
| `NUMBER_STATES` agrees with the types | Table 5-4 | M | Y: a disagreement is refused, not resolved either way |
| Attitude data line width | Table 5-4 | M | Y: one time tag plus `NUMBER_STATES` values |
| `EULER_ROT_SEQ` when the type is Euler angles | Table 5-4 | C | Y |
| Physical characteristics section, one only | 5.3.6.3 | O | Y |
| `CP_REF_FRAME` present if `CP` is | Table 5-5 | C | Y |
| Covariance sections | Table 5-6, 5.3.7.3 | O | Y: any number |
| `COV_TYPE` matrix dimensions | Annex B6 | M | Y: all six values |
| Covariance line is the main diagonal | 5.3.7.6 | M | Y: one time tag plus the dimension |
| Covariance time ordered increasing | 5.3.7.5 | M | Y: the attitude section has no such rule and is not held to one |
| Manoeuvre sections | Table 5-7, 5.3.8.4 | O | Y: any number |
| `MAN_END_TIME` or `MAN_DURATION`, not both | Table 5-7 | C | Y |
| `TARGET_MOM_FRAME` present if `TARGET_MOMENTUM` is | Table 5-7 | C | Y |
| Vector component counts | Tables 5-5, 5-7 | M | Y: `CP` and `TARGET_MOMENTUM` three, `TARGET_ATTITUDE` four |
| Attitude determination section, one only | 5.3.9.2 | O | Y |
| `AD_METHOD` estimator types | Annex B5 | O | Y: all six of `EKF`, `TRIAD`, `QUEST`, `BATCH`, `Q_METHOD`, `FILTER_SMOOTHER` |
| Sensor sub-blocks, delimited and unique | 5.3.9.5, 5.3.9.6, Table 5-8 | O | Y: `SENSOR_START` to `SENSOR_STOP` inside the section and nowhere else; numbers must be unique |
| `SENSOR_NOISE_STDDEV` matches `NUMBER_SENSOR_NOISE_COVARIANCE` | Table 5-8 | O | Y |
| User-defined section, one only, at least one parameter | 5.3.10.4, Table 5-9 | O | Y |
| Relative or absolute time tags | 5.3.4.3 | M | Y: `DataRow.TimeTag` resolves either against `EPOCH_TZERO` |
| No duplicate time tags in a block | 5.3.4.4 | M | Y |
| One time tag kind per block | 5.3.4.5 | M | Y |

### XML form, section 7

| Feature | Reference | Status | Support |
|---|---|:-:|---|
| Root element with id and version attributes | 7.4.2.8–7.4.2.10 | M | Y: version 2.0 |
| Schema instance namespace, exactly as given | 7.4.2.3 | M | Y |
| Master schema for the ADM | 7.4.2.5 | O | Y: `ndmxml-4.0.0-master-4.0.xsd`, not the ODM's 3.0.0 |
| `quaternionState` wrapping frames, `quaternion` and `quaternionDot` | 7.5.11, 7.5.12 | M | Y |
| `eulerAngleState`, `angularVelocity`, `spin`, `inertia`, `maneuverParameters` | 7.5.11 | O | Y |
| `attitudeState` with the type's own inner element | 7.6.11, Table 7-5 | M | Y: all nine; a disagreement between type and element is refused |
| Units as attributes | 7.4, 7.7.10–7.7.12 | O | Y |
| ACM root, one segment, one metadata section | 7.7.1–7.7.7 | M | Y |
| ACM section tags and data line tags | Table 7-7 | M | Y: `att`, `phys`, `cov`, `man`, `ad`, `user` and their `attLine`, `covLine` |
| ACM data lines as `xsd:string` | 7.7.13.3 | M | Y: kept as rows, split by the reader, as the clause intends |
| `sensorData` elements inside `ad` | 7.7.14 | O | Y: shown by example rather than stated; one outside the attitude determination block is refused |
| NDM combined instantiation | 7.8 | O | Y: implemented by [`pkg/ndm`](/conformance/ndm), since a combined file may hold messages from other standards too |

---

## A3 EXCEPTIONS AND UNSUPPORTED FEATURES

**An ACM's keywords are not typed.** Its sections are held as ordered keyword
lists with typed accessors for the keywords that change how the data must be
read. There are something over a hundred of them across six sections, most
optional, so there would be little to parse a value into and no way for a
caller to see an unfamiliar keyword if it were dropped. `Get` reaches anything;
the values are carried as text. That is the same choice `pkg/odm` makes for
the OCM.

**An ACM covariance carries only its diagonal.** Clause 5.3.7.6 puts the main
diagonal on the line and nothing else, and clause 5.3.7.7 sends anyone who
needs the off-diagonal terms to a user-defined block. That is a limit of the
format, not of this package: `CovarianceCount` reports the dimension, and there
is no matrix to rebuild.

**No attitude mathematics.** Nothing normalises a quaternion, converts between
representations, composes rotations, or interpolates. `INTERPOLATION_METHOD`
and `INTERPOLATION_DEGREE` are carried and acted on by nobody. The conventions
those operations depend on are in annex F rather than in the wire format, and a
library that guessed at them would be guessing about spacecraft attitude.

**A quaternion is not normalised or checked.** A message whose four components
do not describe a unit quaternion is structurally valid and is read without
complaint. Whether to normalise, and how to treat one that is far from unit, is
a decision for whatever uses the numbers.

**The useable span is not applied.** As in the OEM, `USEABLE_START_TIME` and
`USEABLE_STOP_TIME` are read and preserved and no records are dropped.

**`AttitudeLine.Values` is not unpacked into named fields.** Table 4-4 gives
the same position different meanings under different types — the fourth value
is `QC` for a quaternion and `SPIN_ANGLE_VEL` for a spin state. Naming them
would mean naming eight of the nine layouts wrongly. `AttitudeType.Fields`
reports the width and the caller reads the table.

**Enumerated values are not enforced beyond `ATTITUDE_TYPE`.** Frame names,
`EULER_ROT_SEQ` axis orders and interpolation methods have value sets in annex
B. They are carried as written, because annex B is a registry that changes
without this package.

---

## A4 IMPLEMENTATION LIMITS

| Limit | Value | Source |
|---|---|---|
| Line length, APM and AEM | 254 characters | Clause 6.6.1 |
| Line length, ACM | unbounded | Clause 6.6.2 exempts the ACM outright, the same split CCSDS 502.0-B-3 makes for the OCM |
| Digits in a non-integer value | 16 | Clause 6.5 |
| Maneuvers per APM | bounded by the input | No ceiling imposed |
| Records per AEM block | bounded by the input | Read into memory rather than streamed |
| Attitude types, AEM | 9 | Table 4-4 |
| Attitude and rate types, ACM | 3 and 5 | Annex B4 |
| Covariance types, ACM | 6 | Annex B6 |
| Data sections per ACM | bounded by the input | No ceiling on the repeating sections; the rest are limited to one by the standard |
| Sensor blocks per ACM | bounded by the input | Clause 5.3.9.5 asks for as many as there are sensors |

---

## Wire test vectors

The files backing this statement live in the [vector corpus](https://github.com/ravisuhag/astro/tree/main/vectors/adm) — 8 decode vectors and 8 corpus files.

| File | |
|---|---|
| [`adm/attitude.json`](https://github.com/ravisuhag/astro/blob/main/vectors/adm/attitude.json) | 8 vectors |
| `adm/apm-*.kvn`, `adm/aem-*.kvn`, `adm/acm-*.kvn` | the annex G examples as readable files |
| `adm/acm-xml.xml` | the ACM of figure G-12 in the XML form of section 7 |

All are **published text rather than derived values**: annex G of the Blue Book prints them.

The vectors assert the shape — which blocks an APM carried, the frames a rotation goes between, and for an AEM the attitude type together with the line width it implies. For an ACM the shape is self-checking, and the vectors carry every part of it: the types, the `NUMBER_STATES` the producer declared, and the row width all three have to agree on. The numbers are floats, which a vector field cannot hold, and are checked in `pkg/adm` against the same published text.

See [`CONTRACT.md`](https://github.com/ravisuhag/astro/blob/main/vectors/CONTRACT.md) for how to consume these, and [how this is verified](/docs/reference/verification) for what rests on a published vector versus a reading of the clause.
