---
title: Orbit Data Messages
short: ODM
description: CCSDS 502.0-B-3, the text files agencies swap to say where a spacecraft is.
identifiers:
  - "CCSDS 502.0-B-3 * Orbit Data Messages"
  - "ISO 26900"
  - "pkg/odm"
order: 33
---

> **CCSDS 502.0-B-3** | [Blue Book](https://public.ccsds.org/Pubs/502x0b3e1.pdf) | [`pkg/odm`](https://github.com/ravisuhag/astro/tree/main/pkg/odm)

## Overview

An Orbit Data Message says where a spacecraft is. Agencies and operators send
these files to each other to plan tracking passes, hand over navigation
support, compare orbits, and screen for collisions.

Unlike every other standard here, an ODM does not travel on a space link. It
travels between organisations, usually as a file. That is why it is plain text
rather than packed octets, and why the standard is published jointly with ISO
as ISO 26900.

There are four messages, and which one you send depends on what you are saying.

| Message | Says |
|---|---|
| **OPM** | One state vector at one time, with optional elements, spacecraft parameters, covariance and planned manoeuvres. |
| **OMM** | Mean orbital elements. This is what a two-line element set becomes when written as a CCSDS message. |
| **OEM** | A table of state vectors over a span, with the interpolation to use between them. |
| **OCM** | Everything: trajectory, manoeuvres, physical properties, covariance, perturbation models. |

Each may be written in key-value notation or in XML, and clause 1.1 leaves the
choice to the two parties exchanging the file.

## Scope

**Implemented.** All four messages, in key-value notation. Reading, writing,
and the structural rules the standard states.

**Also implemented: the XML form** of section 8, for all four messages.
`EncodeXML` sits beside `Encode`, and `DecodeXMLOPM`, `DecodeXMLOMM`,
`DecodeXMLOEM` and `DecodeXMLOCM` beside their key-value counterparts.

**Carried, not checked: the SANA-registered values.** `TRAJ_TYPE` and
`COV_TYPE` name an OCM row's columns, and clauses 6.2.5.11 and 6.2.7.12.1 draw
their values from the SANA registry rather than from the Blue Book. So this
package cannot say how wide such a row should be. See the OCM section below.

**Deliberately absent: orbital mechanics.** Nothing here propagates a state
vector, converts between reference frames, or turns mean elements into a
position. Those need models the standard does not specify and this package has
no business choosing. Clause 1.2 puts orbit accuracy outside the standard
altogether, so a message placing a spacecraft one metre from the centre of the
Earth is a well-formed message and this package will read it without complaint.

**Left to the caller: what the values mean.** Reference frame and time system
values are carried through as written. Clauses 3.2.3.2 and 3.2.3.3 list the
expected sets and then say anything else "should be documented in an ICD", so
an unrecognised value is not an error.

## Field map

### OEM

An OEM is a header followed by one or more metadata groups, each with its
ephemeris rows and an optional covariance section.

```
CCSDS_OEM_VERS = 3.0
CREATION_DATE  = ...
ORIGINATOR     = ...

META_START
  OBJECT_NAME ... INTERPOLATION_DEGREE
META_STOP
  <epoch> <x> <y> <z> <x_dot> <y_dot> <z_dot> [<x_ddot> <y_ddot> <z_ddot>]
  ...
  COVARIANCE_START
    EPOCH = ...
    [COV_REF_FRAME = ...]
    <21 lower triangular values over as many lines as you like>
  COVARIANCE_STOP
```

| Piece | Go | Notes |
|---|---|---|
| Metadata group | `EphemerisBlock.Metadata` | `OBJECT_NAME`, `OBJECT_ID`, `CENTER_NAME`, `REF_FRAME`, `TIME_SYSTEM`, `START_TIME`, `STOP_TIME` are mandatory. |
| Ephemeris row | `EphemerisLine` | Positional, not key-value. Seven fields, or ten with acceleration. |
| Covariance | `EphemerisBlock.Covariances` | Any number, ordered by increasing epoch (clause 5.2.5.7). |

### OPM

The OPM data section is six logical blocks, of which only the state vector is
mandatory.

| Block | Keywords | Go | Status |
|---|---|---|---|
| Metadata | `OBJECT_NAME` … `TIME_SYSTEM` | `OPMMetadata` | M |
| State vector | `EPOCH`, `X`…`Z_DOT` | `OPMData.StateVector` | M |
| Keplerian elements | `SEMI_MAJOR_AXIS` … `GM` | `OPMData.Keplerian` | none or all |
| Spacecraft parameters | `MASS` … `DRAG_COEFF` | `OPMData.Spacecraft` | O |
| Covariance | `COV_REF_FRAME`, `CX_X` … `CZ_DOT_Z_DOT` | `OPMData.Covariance` | none or all |
| Manoeuvres | `MAN_EPOCH_IGNITION` … `MAN_DV_3` | `OPMData.Maneuvers` | repeats |
| User-defined | `USER_DEFINED_x` | `OPMData.UserDefined` | O |

The covariance matrix goes on the wire as 21 values in lower triangular form,
row by row (clause 3.2.4.10). `Covariance.Matrix` is the full symmetric 6×6:
the upper triangle is filled in on decode, because a covariance matrix is
symmetric by definition and a caller indexing `Matrix[1][2]` should not get a
zero.

### OCM

An OCM is a header, one metadata section, and any number of data sections in
the order table 6-1 fixes. Every section is delimited.

```
CCSDS_OCM_VERS = 3.0
CREATION_DATE  = ...
ORIGINATOR     = ...

META_START   ... META_STOP     one, mandatory
TRAJ_START   ... TRAJ_STOP     any number
PHYS_START   ... PHYS_STOP     at most one
COV_START    ... COV_STOP      any number
MAN_START    ... MAN_STOP      any number
PERT_START   ... PERT_STOP     at most one
OD_START     ... OD_STOP       at most one
USER_START   ... USER_STOP     at most one
```

| Piece | Go | Notes |
|---|---|---|
| Metadata | `OCM.Metadata` | 48 keywords, of which only `EPOCH_TZERO` has no default. |
| Trajectory | `OCM.Trajectories` | Keywords, then positional rows. `TRAJ_TYPE` names the columns. |
| Physical | `OCM.Physical` | 50 keywords describing the object itself. |
| Covariance | `OCM.Covariances` | `COV_TYPE` and `COV_ORDERING` say how a row folds into a matrix. |
| Manoeuvre | `OCM.Maneuvers` | `MAN_COMPOSITION` names the columns of each row. |
| Perturbations | `OCM.Perturbations` | The force models. Required alongside an OD section. |
| Orbit determination | `OCM.OrbitDetermination` | How the orbit was determined, and how well. |
| User-defined | `OCM.UserDefined` | `USER_DEFINED_x`, as in the OPM. |

The sections are held as ordered keyword lists with typed accessors, not as
struct fields. There are over two hundred keywords, most optional and most
describing how an orbit was determined rather than what it is. Two hundred
pointer fields would help nobody, and a caller meeting an unfamiliar keyword
could not see it. `Get` reaches anything.

## An OCM time tag may be a date or a number

This is what most distinguishes the OCM from the rest of the family. Clause
6.2.2.3 lets a time tag be an absolute time **or** a signed count of SI seconds
from `EPOCH_TZERO`:

```
2018-11-13T11:13:20.5Z    absolute
20157.26                  relative to EPOCH_TZERO
```

A reader that assumes absolute times parses nothing. One that assumes relative
times reads a date as a malformed number. `DataRow.TimeTag` resolves either
against the message's `EPOCH_TZERO`, and `IsRelative` says which arrived.

Clause 6.2.2.5 makes a block pick one and keep it, so the two never mix inside
a block — but two blocks in one message may disagree, and figure G-19 of the
Blue Book is exactly that.

## Three OCM keywords say what a row's numbers mean

An OCM data row is positional, and nothing in the row says how wide it is or
what each column holds. Three keywords do, and only one of them can be checked
here.

| Keyword | Section | Values from |
|---|---|---|
| `TRAJ_TYPE` | trajectory | SANA registry (clause 6.2.5.11) |
| `COV_TYPE` with `COV_ORDERING` | covariance | SANA registry; the orderings are in clause 6.2.7.12.3 |
| `MAN_COMPOSITION` | manoeuvre | Tables 6-8 and 6-9 |

`MAN_COMPOSITION` is the checkable one. Its field names are printed in the Blue
Book, so `ManFields` returns the column names and refuses a composition that
mixes the propulsive fields of table 6-8 with the deployment fields of table
6-9, lists them out of the fixed order, or does not start with a time tag.

`COV_ORDERING` is checkable in a different way: the element set is registered
elsewhere, but clause 6.2.7.12.3 defines exactly five layouts, so `CovMatrix`
can fold a row back into a square matrix and work out its size from how many
numbers arrived.

**`LTMWCC` and `UTMWCC` are not symmetric.** Their off-diagonal half carries
correlations rather than covariances — a covariance divided by the product of
two standard deviations. `CovMatrix` returns those two as written. Mirroring
them the way an `LTM` is mirrored would silently scale half the matrix.

## An OCM with no data at all is valid

Clause 6.2.1.1's note calls it a "degenerate case" and says it was an
intentional choice. The metadata alone carries contact details, links to other
messages and timing source information, all of which are useful without a
single state vector. So a header, a metadata section and nothing else is a
well-formed OCM, and this package reads it as one.

## The XML form is a different shape, not a reformatting

Section 8 says the rules for which keywords may appear are the same as for the
key-value form. Three things around them are not.

**Units become an attribute.** `X = 6503.514 [km]` becomes
`<X units="km">6503.514</X>`. Clause 8.10.10 makes them optional in XML as
clause 7.7.1.1 does in KVN.

**A user-defined parameter changes shape.** In the key-value form the name is
part of the keyword; in XML the element is always `USER_DEFINED` and the name
is an attribute:

```
USER_DEFINED_EARTH_MODEL = WGS-84
<USER_DEFINED parameter="EARTH_MODEL">WGS-84</USER_DEFINED>
```

**Repetition becomes repeated elements.** Two manoeuvres in an OPM are two runs
of the same keywords in KVN and two `<maneuverParameters>` blocks in XML.

### The OCM keeps its rows

Clause 8.11.15 gives `<trajLine>`, `<covLine>` and `<manLine>` the type
`xsd:string`. The schema does not look inside them: the recipient still splits
the line on blanks and reads the columns by whatever `TRAJ_TYPE`, `COV_TYPE` or
`MAN_COMPOSITION` says. That is the opposite of the OEM below.

What does change is the delimiters, which become the block elements `<traj>`,
`<phys>`, `<cov>`, `<man>`, `<pert>`, `<od>` and `<user>` of table 8-9.

### The OEM diverges most

Its ephemeris records are positional lines in KVN and one `<stateVector>` block
each in XML, with every component named (clause 8.10.14). Its covariance
matrices are positional rows between `COVARIANCE_START` and `COVARIANCE_STOP`
in KVN, and the OPM's named `CX_X` family in XML (clause 8.10.19). The
delimiters disappear entirely — the blocks are the delimiters.

So the two forms of one OEM carry the same numbers in a genuinely different
shape. Worth knowing before comparing two files by eye and concluding they
disagree.

## Several messages in one file

Clause 8.12 lets any number of orbit messages be aggregated into a single XML
file under an `<ndm>` root — a constellation's ephemerides together, or a full
OEM with the OPMs that mark its important states. That file is a **combined
instantiation**, and [`pkg/ndm`](/protocols/mission/ndm) reads and writes it.
It is a separate package because clause 4.11.7 of CCSDS 505.0-B-3 lets one file
mix the standards, so an orbit message may sit beside an attitude message.

## Three OMM keyword slots take two names each

Table 4-3 gives three pairs of alternatives, and which name applies is decided
by `MEAN_ELEMENT_THEORY`:

| Slot | SGP / SGP4 | SGP4-XP | Units differ? |
|---|---|---|---|
| Orbit size | `MEAN_MOTION` | `MEAN_MOTION` | rev/day vs km for `SEMI_MAJOR_AXIS` |
| Drag | `BSTAR` | `BTERM` | 1/[Earth radii] vs m²/kg |
| Second derivative | `MEAN_MOTION_DDOT` | `AGOM` | rev/day³ vs m²/kg |

They are not two spellings of one number. `BSTAR` is an SGP4 drag term and
`BTERM` is a ballistic coefficient CD·A/m; `MEAN_MOTION_DDOT` is a rate and
`AGOM` is a solar radiation coefficient. Reading one as the other gives a
plausible number with the wrong meaning and the wrong units.

`MeanElements.UsesMeanMotion`, `TLEParameters.UsesBTerm` and
`TLEParameters.UsesAgom` record which arrived, and a message giving both halves
of a pair is refused rather than silently resolved.

## A TLE-based OMM has four fixed conventions

Clause 4.2.4.6 requires all of these when the mean element theory is one of the
SGP family:

- `CENTER_NAME` is `EARTH`
- `REF_FRAME` is `TEME`
- `TIME_SYSTEM` is `UTC`
- `MEAN_MOTION` is used, not `SEMI_MAJOR_AXIS`

An SGP4 propagator assumes all four. Get one wrong and the message is accepted
and mispropagated, so this package refuses it.

The rule runs the other way too. Clause 4.2.4.9 allows `TEME` "only for OMMs
based on NORAD Two Line Element sets, and in no other circumstances", because
no international convention pins the frame down. `TEME` on a `DSST` message is
refused for that reason.

There is one more trap the standard mentions and this package cannot check.
Note 2 under clause 4.2.4.7 says that if `MEAN_MOTION_DOT` and
`MEAN_MOTION_DDOT` came from a TLE, or are meant to be used as one, they must
be divided by 2 and 6 to match the SGP Taylor series terms. Nothing in the
message says whether that has been done.

## An OMM cannot describe a manoeuvre

Clause 4.2.4.8 says so outright: send several OMMs at different epochs instead.
The OPM's `MAN_*` keywords are simply not in table 4-3, so they come back as
`ErrUnknownKeyword` here.

## A second metadata group is a fence

An OEM may carry several metadata groups, and clause 5.2.4.6 gives that
meaning rather than merely permission: **a consumer must not interpolate
across the boundary.** It is how a producer marks a manoeuvre, an eclipse
entry, or any other discontinuity — figure G-11 of the Blue Book uses it for a
trajectory correction manoeuvre and says so in a comment.

A reader that flattens the blocks into one list of records throws that away,
and will happily interpolate a spacecraft straight through a burn.
`OEM.Blocks` keeps them separate for that reason. `Records` and `Span` are
there for the summary a caller usually wants, and neither of them is an
invitation to concatenate.

## The useable span can be narrower than the total

`START_TIME` and `STOP_TIME` bound what is in the block. `USEABLE_START_TIME`
and `USEABLE_STOP_TIME` bound what a consumer should use.

They differ because table 5-3 lets a producer pad the ends with smooth,
fictitious nodes so that an interpolator needing more than two points has
something to work with at the edges. Those nodes are not measurements. If the
useable span is given and you use the total span instead, you are using made-up
data.

This package reads both and trims neither.

## Clause 7.8.9 names a keyword that does not exist

Clause 7.8.9 lists where comments may appear in an OEM, and calls the covariance
delimiter `COV_START`. Table 5-4 defines `COVARIANCE_START`.

`COV_` is the OPM's family — `COV_REF_FRAME`, `CX_X` and the rest live in
table 3-3. The OEM spells its delimiters out in full. A reader built from
clause 7.8.9 alone will not find the section.

## An underscore is a blank

Clause 7.5.9 makes an underscore equivalent to a single blank in any text
value, and collapses runs of blanks to one. `MARS_PATHFINDER` and
`MARS PATHFINDER` are the same object, and this package normalises both to the
second.

The cost is that a name which really contains an underscore cannot be written
at all. There is no escape.

## Units are documentation, not data

Clause 7.7.1.1 lets a value carry its units in square brackets, "for
documentation purposes and clarity only". These two lines say the same thing:

```
X = 6655.9942 [km]
X = 6655.9942
```

This package reads the units and discards them. It writes them for every item
table 3-3 gives one, because a file a human may open is easier to read with
them. Clause 7.7.1.3 forbids the literal `[n/a]` that the tables print for a
dimensionless item, so a dimensionless value simply carries no suffix.

## Zero is a value, absent is not

Clause 3.2.4.5 says a solar radiation coefficient of zero means no solar
radiation pressure is to be considered. Clause 3.2.4.6 says the same for drag.
Those are instructions, not missing data, so they survive a round trip and
`Humanize` spells them out rather than printing a bare `0`.

Mass is the one parameter where absent and zero genuinely differ, because
clause 3.2.4.9 makes it mandatory once a manoeuvre is present. Set it through
`SetMass` and read `HasMass` rather than testing the field against zero.

## Re-encoding does not reproduce the file you read

Clause 7.5.6 makes trailing zeroes optional, and clauses 7.4.5 to 7.4.7 make
the white space around a keyword, an equals sign and the end of a line
insignificant. The same message therefore has many spellings, and the worked
examples in annex G lean on that heavily — they align their equals signs in a
column.

Decoding and re-encoding gives you the same *message*, not the same octets.
Preserving the original spelling would mean storing the text beside every
number, and a message is defined by its values. If you need the bytes you were
given, keep the bytes you were given.

## A comment belongs to the block after it

Clause 7.8.7 places comments at the beginning of a logical block. That has two
consequences worth knowing.

A comment in the header is one that comes **immediately** after the version
keyword. Figure G-1 of the Blue Book puts its `COMMENT` after `ORIGINATOR`,
which makes it the first line of the metadata section rather than a header
comment.

And a comment meant for a group of blocks cannot say so. Figure G-2 writes
`COMMENT 2 planned maneuvers` ahead of the first manoeuvre, meaning it for
both. Each manoeuvre is its own block, so all of it attaches to the first one.
No reader can recover the author's intent here, and this package does not
pretend to.

## Using the package

### Quick start

```go
message, err := odm.DecodeOPM(data)
fmt.Println(message.Data.StateVector.X)
fmt.Println(message.Humanize())

out, err := message.Encode()
```

Mean elements:

```go
elements, err := odm.DecodeOMM(data)
if elements.Metadata.IsTLEBased() {
    // Clause 4.2.4.6 guarantees EARTH, TEME, UTC and MEAN_MOTION here.
    fmt.Println(elements.Data.Elements.MeanMotion, "rev/day")
}
```

An ephemeris, keeping the blocks apart:

```go
ephemeris, err := odm.DecodeOEM(data)
start, stop := ephemeris.Span()

for _, block := range ephemeris.Blocks {
    // Do not interpolate from one block into the next (clause 5.2.4.6).
    for _, row := range block.Lines {
        fmt.Println(row.Epoch, row.X, row.Y, row.Z)
    }
}
```

### Errors

| Error | Means |
|---|---|
| `ErrUnknownKeyword` | A keyword none of the tables in section 3 lists. Clause 3.2.4.2 says only those shall be used. |
| `ErrDuplicateKeyword` | A keyword given twice in a block that allows it once. |
| `ErrMissingKeyword` | A mandatory keyword is absent. |
| `ErrIncompleteKeplerian` | Some but not all of the Keplerian block, which table 3-3 makes all-or-nothing. |
| `ErrBothAnomalies` | Both `TRUE_ANOMALY` and `MEAN_ANOMALY`, which table 3-3 offers as alternatives. |
| `ErrManeuverWithoutMass` | A manoeuvre with no `MASS`, which clause 3.2.4.9 requires. |
| `ErrDeltaMassNotNegative` | A `MAN_DELTA_MASS` of zero or more. Clause 3.2.4.7 requires it to be negative: a manoeuvre spends propellant. |
| `ErrKeywordOutOfOrder` | A manoeuvre parameter before the `MAN_EPOCH_IGNITION` that starts its block. |

OEM-specific:

| Error | Means |
|---|---|
| `ErrNoEphemerisBlock` | An OEM with no metadata group at all. |
| `ErrUnterminatedBlock` | A `META_START` or `COVARIANCE_START` that never closes. |
| `ErrUnexpectedDelimiter` | A delimiter where clause 5.2.3.3 does not allow one, including an ephemeris row before any metadata group. |
| `ErrEphemerisLineFields` | A data row without 7 or 10 fields. |
| `ErrInterpolationDegreeMissing` | `INTERPOLATION` without `INTERPOLATION_DEGREE`, which table 5-3 makes mandatory alongside it. |
| `ErrTimeSystemChanged` | Two metadata groups naming different time systems, which clause 5.2.4.5 forbids. |
| `ErrCovarianceOutOfOrder` | Covariance epochs that do not increase (clause 5.2.5.7). |
| `ErrCovarianceValueCount` | A matrix that is not 21 lower triangular values. |

OMM-specific:

| Error | Means |
|---|---|
| `ErrSizeKeywordMissing` | Neither `SEMI_MAJOR_AXIS` nor `MEAN_MOTION`. Table 4-3 makes the pair mandatory. |
| `ErrBothSizeKeywords` | Both of them, which does not say which the receiver should believe. |
| `ErrBothDragKeywords` | Both `BSTAR` and `BTERM`, or both `MEAN_MOTION_DDOT` and `AGOM`. |
| `ErrTLEConventions` | A TLE-based message breaking one of the four conventions of clause 4.2.4.6. |
| `ErrTEMEWithoutTLE` | `TEME` on a message that is not TLE-based (clause 4.2.4.9). |

Line syntax errors come from `internal/ndm` and carry the line number.

## Notes

The four messages exist because the alternatives are worse. An OEM of ten
thousand state vectors is the honest way to hand over a reconstructed orbit,
and an OPM is the honest way to hand over one epoch — sending either in place
of the other means the receiver either drowns or interpolates something you did
not sanction. Annex E of the Blue Book gives the working group's own reasoning
and is worth reading before choosing.

## Reference

- [CCSDS 502.0-B-3, Orbit Data Messages](https://public.ccsds.org/Pubs/502x0b3e1.pdf)
- [CCSDS 500.0-G-4, Navigation Data Messages Overview](https://ccsds.org/Pubs/500x2g3.pdf) (Green Book)
- [Conformance](/conformance/odm) | [Time code formats](/protocols/mission/tcf) | [The stack](/docs/start/concepts)
