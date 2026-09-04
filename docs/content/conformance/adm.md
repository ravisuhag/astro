---
title: Attitude Data Messages
short: ADM
description: "Coverage matrix: what this package implements, clause by clause."
order: 220
---

## Conformance Statement for `pkg/adm`, CCSDS 504.0-B-2

Two of the three messages are implemented: the Attitude Parameter Message of
section 3 and the Attitude Ephemeris Message of section 4. The Attitude
Comprehensive Message of section 5 is not.

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

---

## A3 EXCEPTIONS AND UNSUPPORTED FEATURES

**The ACM is not implemented.** Section 5's Attitude Comprehensive Message is
roughly three times the size of sections 3 and 4 together, arrived with this
2024 issue of the standard, and has thin adoption next to the APM and the AEM.
The same reasoning applied to the ODM's OCM. A message naming
`CCSDS_ACM_VERS` is refused rather than half-read.

**The XML form is not implemented.** Section 7 defines it and CCSDS 505.0-B-3
specifies the container.

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
| Line length | 254 characters | Clause 6.4 |
| Digits in a non-integer value | 16 | Clause 6.5 |
| Maneuvers per APM | bounded by the input | No ceiling imposed |
| Records per AEM block | bounded by the input | Read into memory rather than streamed |
| Attitude types | 9 | Table 4-4 |

---

## Wire test vectors

The files backing this statement live in the [vector corpus](https://github.com/ravisuhag/astro/tree/main/vectors/adm) — 3 decode vectors and 3 corpus files.

| File | |
|---|---|
| [`adm/attitude.json`](https://github.com/ravisuhag/astro/blob/main/vectors/adm/attitude.json) | 3 vectors |
| `adm/apm-*.kvn`, `adm/aem-*.kvn` | the annex G examples as readable files |

All are **published text rather than derived values**: annex G of the Blue Book prints them.

The vectors assert the shape — which blocks an APM carried, the frames a rotation goes between, and for an AEM the attitude type together with the line width it implies. The numbers are floats, which a vector field cannot hold, and are checked in `pkg/adm` against the same published text.

See [`CONTRACT.md`](https://github.com/ravisuhag/astro/blob/main/vectors/CONTRACT.md) for how to consume these, and [how this is verified](/docs/reference/verification) for what rests on a published vector versus a reading of the clause.
