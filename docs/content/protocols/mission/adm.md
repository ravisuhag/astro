---
title: Attitude Data Messages
short: ADM
description: CCSDS 504.0-B-2, which way a spacecraft is pointing.
identifiers:
  - "CCSDS 504.0-B-2 * Attitude Data Messages"
  - "pkg/adm"
order: 35
---

> **CCSDS 504.0-B-2** | [Blue Book](https://ccsds.org/Pubs/504x0b2.pdf) | [`pkg/adm`](https://github.com/ravisuhag/astro/tree/main/pkg/adm)

## Overview

An attitude message says which way a spacecraft is pointing. Where an
[orbit message](/protocols/mission/odm) gives a position, these give an
orientation: a quaternion, three Euler angles, or a spin axis, plus the two
frames the rotation goes between.

| Message | Says |
|---|---|
| **APM** | One attitude at one epoch, with optional angular velocity, spin, inertia and planned manoeuvres. |
| **AEM** | A table of attitudes over a span, with the interpolation to use between them. |
| **ACM** | Everything at once: attitude histories, physical properties, covariance, manoeuvres, and how the attitude was determined. |

## Scope

**Implemented.** All three messages, in key-value notation.

**Also implemented: the XML form** of section 7, for all three. `EncodeXML`
sits beside `Encode`, and `DecodeXMLAPM`, `DecodeXMLAEM` and `DecodeXMLACM`
beside their key-value counterparts.

**Deliberately absent: attitude mathematics.** Nothing here normalises a
quaternion, converts one to Euler angles, composes two rotations, or
interpolates between attitudes. The conventions those depend on are in annex F
of the Blue Book rather than in the wire format, and getting them wrong is a
different kind of mistake from getting a file format wrong.

## The scalar component comes last

A quaternion goes on the wire as `Q1`, `Q2`, `Q3`, `QC`. The first three are
the vector part — each is *e*ₙ·sin(φ/2) — and `QC` is the scalar, cos(φ/2).

A great many quaternion libraries take the scalar **first**. Reading these four
numbers into a `[4]float64` and handing it to one of them gives a rotation that
is wrong and looks entirely plausible. That is why `Quaternion` names its
fields instead of indexing them:

```go
type Quaternion struct {
    Q1, Q2, Q3 float64 // vector part
    QC         float64 // scalar part
}
```

## An AEM data line has no fixed width

This is the AEM's central trap. The number of values on an attitude line comes
from the segment's `ATTITUDE_TYPE` and from nowhere else. Table 4-4 gives nine
layouts:

| `ATTITUDE_TYPE` | Values after the epoch |
|---|--:|
| `EULER_ANGLE` | 3 |
| `QUATERNION`, `SPIN` | 4 |
| `EULER_ANGLE/DERIVATIVE`, `EULER_ANGLE/ANGVEL` | 6 |
| `QUATERNION/ANGVEL`, `SPIN/NUTATION`, `SPIN/NUTATION_MOM` | 7 |
| `QUATERNION/DERIVATIVE` | 8 |

Four numbers after an epoch are a quaternion under one type and a spin state
under another. Nothing in the line says which. `AttitudeType.Fields` reports
the width, and every line is checked against it — a file whose type and width
disagree is refused rather than read as something else.

`AttitudeLine.Values` is a slice rather than named fields for the same reason:
the fourth value is `QC` for a quaternion and `SPIN_ANGLE_VEL` for a spin
state, so naming it would be naming a lie for eight of the nine types.

## Every attitude block names two frames

`REF_FRAME_A` and `REF_FRAME_B` are the start and end of the transformation. An
attitude with no frames is not an attitude — it is four numbers.

Both are mandatory in every APM block that describes a rotation, and in every
AEM metadata group. `Quaternion.rotation()` is what `Humanize` prints, and the
vectors record both.

## Three angles are not a rotation

A Euler block also needs `EULER_ROT_SEQ`: `YXY` and `ZXZ` describe different
orientations from the same three angles. It is mandatory in an APM Euler block,
and in an AEM whenever `ATTITUDE_TYPE` is one of the three Euler variants.

## The APM has no metadata delimiters

Unlike the AEM and the OEM, an APM's metadata section is not wrapped in
`META_START` and `META_STOP`. It simply ends at `EPOCH`, which is the first
data keyword. Everything after that is either a block delimiter or inside a
block.

The six data blocks *are* delimited, each with its own pair — `QUAT_START` and
`QUAT_STOP`, `EULER_START` and `EULER_STOP`, and so on. A block closed by
another block's stop keyword is refused.

## The ACM

An ACM is a header, one metadata section, and any number of data sections in
the order table 5-1 fixes. Every section is delimited, the same shape the
ODM's OCM has.

```
CCSDS_ACM_VERS = 2.0
CREATION_DATE  = ...
ORIGINATOR     = ...

META_START  ... META_STOP    one, mandatory
ATT_START   ... ATT_STOP     any number
PHYS_START  ... PHYS_STOP    at most one
COV_START   ... COV_STOP     any number
MAN_START   ... MAN_STOP     any number
AD_START    ... AD_STOP      at most one
USER_START  ... USER_STOP    at most one
```

| Piece | Go | Notes |
|---|---|---|
| Metadata | `ACM.Metadata` | 20 keywords. `OBJECT_NAME`, `TIME_SYSTEM` and `EPOCH_TZERO` are mandatory. |
| Attitude | `ACM.Attitudes` | Keywords, then positional rows. `ATT_TYPE` and `RATE_TYPE` name the columns. |
| Physical | `ACM.Physical` | Mass, centre of pressure, inertia tensor. |
| Covariance | `ACM.Covariances` | `COV_TYPE` gives the matrix size; only the diagonal is on the wire. |
| Manoeuvre | `ACM.Maneuvers` | Purpose, start time, actuator, and a target for each purpose. |
| Attitude determination | `ACM.AttitudeDetermination` | The estimator and its sensors. |
| User-defined | `ACM.UserDefined` | `USER_DEFINED_x`. |

The sections are held as ordered keyword lists with typed accessors, not as
struct fields — the same choice `pkg/odm` makes for the OCM, and for the same
reason.

## An ACM row's width is checkable; an OCM row's is not

Both messages have positional rows whose columns come from a keyword. The
difference is where the keyword's values are written down.

| | ACM | OCM |
|---|---|---|
| Attitude / trajectory | `ATT_TYPE`, annex B4 | `TRAJ_TYPE`, SANA registry |
| Covariance | `COV_TYPE`, annex B6 | `COV_TYPE`, SANA registry |
| Declared count | `NUMBER_STATES`, mandatory | none |

Annex B4 prints the component count of every attitude and rate type:

| `ATT_TYPE` | Components | | `RATE_TYPE` | Components |
|---|--:|---|---|--:|
| `QUATERNION` | 4 | | `ANGVEL` | 3 |
| `EULER_ANGLES` | 3 | | `Q_DOT` | 4 |
| `DCM` | 9 | | `EULER_RATE` | 3 |
| | | | `GYRO_BIAS` | 3 |
| | | | `NONE` | 0 |

So a block says how wide its rows are twice: once through its types, and once
through `NUMBER_STATES`. This package checks that the two agree, and that the
rows match. A message where they disagree is one where a producer and a
consumer would read different columns, so it is refused rather than resolved
either way.

## An ACM covariance row is only the diagonal

Clause 5.3.7.6 puts the main diagonal on the line and nothing else, so a 6×6
`ANGLE_GYROBIAS` covariance is six numbers after the time tag. An OCM
covariance of the same size writes twenty-one — the whole lower triangle.

Clause 5.3.7.7 says the off-diagonal terms, if anyone needs them, go in a
user-defined block. That is a real limit of the format rather than of this
package: an ACM cannot carry a full attitude covariance in its covariance
section.

## An ACM time tag may be a date or a number

Clause 5.3.4.3 lets a time tag be an absolute time or a signed count of SI
seconds from `EPOCH_TZERO`, exactly as the OCM does. Every example in annex G
uses relative tags. `DataRow.TimeTag` resolves either, and `IsRelative` says
which arrived.

Clause 5.3.4.5 makes a block pick one and keep it. Clause 5.3.7.5 additionally
requires a covariance history to run forward in time — and says nothing of the
sort about attitude histories, so this package does not impose it there.

## The XML form nests further than the orbit messages

An APM quaternion is not four keywords in a row. Clause 7.5.11 puts the frames
and the components in a `<quaternionState>`, the components themselves in a
`<quaternion>` inside that, and the optional derivatives in a sibling
`<quaternionDot>`.

The AEM goes further. Table 7-5 gives each of the nine attitude types its own
element inside `<attitudeState>`:

| `ATTITUDE_TYPE` | Element |
|---|---|
| `QUATERNION` | `quaternionEphemeris` |
| `QUATERNION/DERIVATIVE` | `quaternionDerivative` |
| `QUATERNION/ANGVEL` | `quaternionAngVel` |
| `EULER_ANGLE` | `eulerAngle` |
| `SPIN/NUTATION_MOM` | `spinNutationMom` |

So the choice the key-value form expresses as a line width becomes a choice of
element name. A file whose `ATTITUDE_TYPE` and inner element disagree is
refused — it is the XML form of a line of the wrong width.

### The ACM keeps its rows

The ACM is the exception. Clause 7.7.13.3 gives `<attLine>` and `<covLine>` the
type `xsd:string`: the schema does not look inside them, and the recipient
still splits the line and reads its columns by `ATT_TYPE`, `RATE_TYPE` or
`COV_TYPE`. Its sections become the block elements `<att>`, `<phys>`, `<cov>`,
`<man>`, `<ad>` and `<user>` of table 7-7.

The sensor sub-blocks nested inside the attitude determination section become
`<sensorData>` elements. Clause 7.7.14 shows that rather than stating it — it
is the one part of the ACM mapping the standard leaves to an example.

The ADM also names a **different schema** from the ODM: `ndmxml-4.0.0` where
the orbit messages give `3.0.0`. The numbers track each standard's own schema
issue, not the NDM/XML document.

## Several messages in one file

Clause 7.8 lets any number of attitude messages be aggregated into a single XML
file under an `<ndm>` root, and clause 5.2.2 points the ACM straight at it.
That file is a **combined instantiation**, and
[`pkg/ndm`](/protocols/mission/ndm) reads and writes it. It is a separate
package because clause 4.11.7 of CCSDS 505.0-B-3 lets one file mix the
standards, so an attitude message may sit beside the orbit it depends on.

## Using the package

```go
attitude, err := adm.DecodeAPM(data)
if q := attitude.Quaternion; q != nil {
    // Scalar last. Do not pass these four as a bare slice.
    fmt.Println(q.Q1, q.Q2, q.Q3, q.QC, "from", q.FrameA, "to", q.FrameB)
}

ephemeris, err := adm.DecodeAEM(data)
for _, block := range ephemeris.Blocks {
    width, _ := block.Metadata.Type.Fields()
    fmt.Println(block.Metadata.Type, "gives", width, "values per line")
}

comprehensive, err := adm.DecodeACM(data)
tzero, _ := comprehensive.EpochTZero()
for _, block := range comprehensive.Attitudes {
    states, _ := block.StateCount()
    fmt.Println(block.AttitudeType(), "with", block.RateType(), "is", states, "states")
    for _, row := range block.Rows {
        // A time tag may be a date or seconds from T-zero. This resolves both.
        at, _ := row.TimeTag(tzero)
        fmt.Println(at, row.Fields[1:])
    }
}
```

### Errors

| Error | Means |
|---|---|
| `ErrNoAttitude` | An APM with no quaternion, no Euler angles and no spin — nothing that says where it points. |
| `ErrUnterminatedBlock` | A `*_START` with no matching `*_STOP`. |
| `ErrUnexpectedDelimiter` | A stop with no start, a nested start, or a block closed by another block's keyword. |
| `ErrUnknownAttitudeType` | An `ATTITUDE_TYPE` outside table 4-4's nine values. |
| `ErrAttitudeLineFields` | A data line whose width does not match the segment's type. |
| `ErrEulerRotSeqMissing` | A Euler type with no `EULER_ROT_SEQ`. |
| `ErrInterpolationDegreeMissing` | `INTERPOLATION_METHOD` without `INTERPOLATION_DEGREE`. |
| `ErrIncompleteNutation` | Some but not all of a spin block's nutation keywords. |
| `ErrStateCountMismatch` | An ACM attitude block whose `NUMBER_STATES` disagrees with its `ATT_TYPE` and `RATE_TYPE`. |
| `ErrCovarianceLineFields` | An ACM covariance row that is not the diagonal its `COV_TYPE` implies. |
| `ErrSectionsOutOfOrder` | ACM sections in an order other than table 5-1's. |
| `ErrKeywordsOutOfOrder` | An ACM keyword out of its table's order. |
| `ErrMissingFrame` | A `CP` with no `CP_REF_FRAME`, or a `TARGET_MOMENTUM` with no `TARGET_MOM_FRAME`. |
| `ErrMixedTimeTags` | An ACM block mixing relative and absolute time tags. |

## Reference

- [CCSDS 504.0-B-2, Attitude Data Messages](https://ccsds.org/Pubs/504x0b2.pdf)
- [CCSDS 500.0-G-4, Navigation Data Messages Overview](https://ccsds.org/Pubs/500x2g3.pdf) (Green Book)
- [Conformance](/conformance/adm) | [Orbit Data Messages](/protocols/mission/odm) | [The stack](/docs/start/concepts)
