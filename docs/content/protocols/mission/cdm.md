---
title: Conjunction Data Message
short: CDM
description: CCSDS 508.0-B-1, a warning that two objects will pass close.
identifiers:
  - "CCSDS 508.0-B-1 * Conjunction Data Message"
  - "pkg/cdm"
order: 36
---

> **CCSDS 508.0-B-1** | [Blue Book](https://ccsds.org/Pubs/508x0b1e2c2.pdf) | [`pkg/cdm`](https://github.com/ravisuhag/astro/tree/main/pkg/cdm)

## Overview

A CDM is a warning. It says two objects in orbit are going to pass close to
each other, when, how close, and how well each object's position is known.

It is the only navigation message that describes a *risk* rather than a
spacecraft, and the only one whose reader has a decision to make: whether to
spend propellant getting out of the way. One message covers one conjunction
between exactly two objects.

## Scope

**Implemented.** Reading and writing, in key-value notation. All the keywords
of tables 3-1 through 3-8.

**Not yet implemented.** The XML form of section 4.

**Deliberately absent: conjunction analysis.** Nothing here propagates either
object, recomputes a miss distance, or calculates a collision probability.
Clause 1.1 makes the message a report of the originator's analysis rather than
an input to yours.

## Both objects, or neither

There are no block delimiters. What separates the three sections is the
`OBJECT` keyword: everything before the first one is relative metadata and
data, everything after `OBJECT = OBJECT1` belongs to the first object, and
everything after `OBJECT = OBJECT2` to the second.

A message with one object section is not a conjunction and is refused. So is a
state vector written before any `OBJECT` — read loosely it would become a
property of the conjunction rather than of an object, and the message would
look complete. That case gets its own error, `ErrObjectOutOfOrder`, rather than
"unknown keyword", because it is a real keyword in the wrong place.

## The two objects are not symmetric

One is usually the operator's spacecraft; the other is usually debris, and its
covariance will be far larger. Nothing in the format says which is which.
`MESSAGE_FOR` names the spacecraft the warning was sent to and is optional.

What does mark the asymmetry is `MANEUVERABLE`, and it is the first thing an
operator looks at: a conjunction with an unmanoeuvrable object is one only one
side can resolve.

```go
for i := range message.Objects {
    canMove, stated := message.Objects[i].Maneuverable()
    fmt.Println(i+1, message.Objects[i].Name(), canMove, stated)
}
```

## The covariance is nine by nine

The obligatory part is the 21 lower triangular elements of a 6×6 in the RTN
frame. Three optional rows extend it to 9×9, adding the uncertainties in drag,
solar radiation pressure and thrust.

`Covariance` returns the full 9×9 with absent rows left at zero. **Use
`CovarianceOrder` to find how many rows were really there** — a zero row and an
absent one look identical in the matrix, and they mean different things to
anyone computing a probability from it.

Note the units are metres here, not the kilometres the orbit messages use.

## A probability without its method is unusable

Table 3-2 makes `COLLISION_PROBABILITY_METHOD` obligatory whenever
`COLLISION_PROBABILITY` is given, and this package enforces it. The reason is
that the number is not comparable between methods: the same conjunction gives
different probabilities under Foster-1992 and Alfano-2005, and a bare figure
cannot be acted on or compared with yesterday's.

`CollisionProbability` returns both, or nothing.

## Re-encoding reproduces the values exactly

Unlike [`pkg/odm`](/protocols/mission/odm), a decoded CDM re-encodes to the
same values it came in with, spelling included. A `Section` keeps the raw
string each keyword arrived with and writes it out again.

That is deliberate. An orbit message is data a caller assembles and edits; a
conjunction warning is a report a caller reads, forwards and archives, and
re-emitting it unchanged is worth more than a tidy number format.

## Two header keywords are unique to this message

| Keyword | Here | Elsewhere |
|---|---|---|
| `MESSAGE_ID` | obligatory | optional |
| `MESSAGE_FOR` | optional | does not exist |
| `CLASSIFICATION` | does not exist | optional in ODM and ADM |

`MESSAGE_ID` is obligatory because a conjunction warning has to be referable
when someone asks about it a week later.

## Using the package

```go
message, err := cdm.Decode(data)

tca, _ := message.TCA()
miss, _ := message.MissDistance()
fmt.Printf("%.0f m at %s\n", miss, tca)

if p, method, ok := message.CollisionProbability(); ok {
    fmt.Printf("%.2e by %s\n", p, method)
}

for i := range message.Objects {
    o := message.Objects[i]
    fmt.Println(o.Name(), o.CovarianceOrder(), "rows of covariance")
}
```

### Errors

| Error | Means |
|---|---|
| `ErrMissingObject` | The message does not describe both objects. |
| `ErrObjectValue` | An `OBJECT` value other than `OBJECT1` or `OBJECT2`. |
| `ErrObjectRepeated` | The same object named twice. |
| `ErrObjectOutOfOrder` | An object keyword before any `OBJECT` says which object it belongs to. |
| `ErrMissingKeyword` | An obligatory keyword is absent — including a probability with no method. |
| `ErrUnknownKeyword` | A keyword none of the tables lists, or a relative keyword inside an object section. |

## Reference

- [CCSDS 508.0-B-1, Conjunction Data Message](https://ccsds.org/Pubs/508x0b1e2c2.pdf)
- [CCSDS 500.0-G-4, Navigation Data Messages Overview](https://ccsds.org/Pubs/500x2g3.pdf) (Green Book)
- [Conformance](/conformance/cdm) | [Orbit Data Messages](/protocols/mission/odm) | [The stack](/docs/start/concepts)
