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
| **ACM** | Everything at once. Not implemented. |

## Scope

**Implemented.** The APM and the AEM, in key-value notation.

**Not implemented: the ACM.** Section 5 is roughly three times the size of
sections 3 and 4 together, it arrived with this 2024 issue, and adoption is
thin next to the other two. That is the same call this project made about the
ODM's OCM.

**Also implemented: the XML form** of section 7. `EncodeXML` sits beside
`Encode`, and `DecodeXMLAPM` and `DecodeXMLAEM` beside their key-value
counterparts.

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

The ADM also names a **different schema** from the ODM: `ndmxml-4.0.0` where
the orbit messages give `3.0.0`. The numbers track each standard's own schema
issue, not the NDM/XML document.

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

## Reference

- [CCSDS 504.0-B-2, Attitude Data Messages](https://ccsds.org/Pubs/504x0b2.pdf)
- [CCSDS 500.0-G-4, Navigation Data Messages Overview](https://ccsds.org/Pubs/500x2g3.pdf) (Green Book)
- [Conformance](/conformance/adm) | [Orbit Data Messages](/protocols/mission/odm) | [The stack](/docs/start/concepts)
