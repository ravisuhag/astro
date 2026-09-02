---
title: XTCE
short: XTCE
description: XTCE 1.2, reading the mission database that says what the octets mean.
identifiers:
  - "XTCE 1.2 (OMG) * XTCE"
  - "pkg/xtce * astro xtce"
order: 80
---

> **XTCE 1.2 (OMG)** | [Spec](https://www.omg.org/spec/XTCE/) | [CCSDS 660.1-G-2](https://public.ccsds.org/Pubs/660x1g2.pdf) as the guide to it | [`pkg/xtce`](https://github.com/ravisuhag/astro/tree/main/pkg/xtce) | [`astro xtce`](/cli/xtce)

## Overview

Every other package in this library moves bytes. This one moves none.

A telemetry frame arrives and `pkg/tmdl` takes it apart. Inside is a packet,
and `pkg/spp` takes that apart too. What comes out is a run of octets that
means something (a bus voltage, a thruster state, a clock reading) and
nothing in the frame says which. That knowledge lives in the mission database,
and XTCE is the format missions write it in.

```
   ┌──────────┐   ┌──────────┐   ┌──────────┐
   │  frame   │──►│  packet  │──►│  octets  │
   └──────────┘   └──────────┘   └──────────┘
    pkg/tmdl       pkg/spp            │
                                      │  what do they mean?
                                      ▼
                              ┌───────────────┐
                              │ mission       │
                              │ database      │ ◄── pkg/xtce
                              └───────────────┘
```

XTCE is an XML schema published by the OMG. Ground systems exchange these
files routinely (NASA, ESA, Yamcs and the commercial operators all read and
write them) so being able to load one is what connects this library's
decoders to a real mission.

## Scope

**Implemented.** Four things:

| | |
|---|---|
| `Load`, `LoadFile` | parse a database |
| `Validate` | check that it hangs together |
| `FindParameter`, `ResolveParameter`, `Walk` | navigate it |
| `Humanize` | print it |

There is no `Encode`. Writing XTCE back out is a decision made against, not an
omission: this package reads databases, and databases are written by editors.
A writer would mean committing to a round-trip fidelity nothing here needs.

**Not modeled**: algorithms, alarms, streams, command semantics, and a few
parameter types. The reasoning and the full list are in
[what is deliberately not modeled](#what-is-deliberately-not-modeled) below,
and the element-by-element detail is in the
[coverage matrix](/conformance/xtce).

## The shape of a database

A `SpaceSystem` is the root, and also the only recursive element: a SpaceSystem
contains SpaceSystems. The tree is a namespace, so a parameter's full name is
the path down to it.

```
/Spacecraft
├── PacketID                    a parameter defined at the top
├── /Spacecraft/Power
│   ├── BusVoltage
│   └── BusCurrent
└── /Spacecraft/Thermal
    └── SampleTime
```

Under each system sit three sets that matter:

- **ParameterTypeSet**: the types. A type says what a value *is* and how it is
  written on the wire.
- **ParameterSet**: the parameters. A parameter is little more than a name and
  a pointer to its type.
- **ContainerSet**: the containers. A container is a packet layout: an ordered
  list of entries naming parameters.

### Types and encodings are two different things

This is the part that surprises people. A parameter type answers two questions
separately:

```xml
<FloatParameterType name="Volts_t" sizeInBits="32">
  <IntegerDataEncoding encoding="unsigned" sizeInBits="12">
    <DefaultCalibrator>
      <PolynomialCalibrator>
        <Term coefficient="0" exponent="0"/>
        <Term coefficient="0.00732" exponent="1"/>
      </PolynomialCalibrator>
    </DefaultCalibrator>
  </IntegerDataEncoding>
</FloatParameterType>
```

That is a **float** parameter, 32 bits wide in software, carried as a **12-bit
unsigned integer** on the downlink, turned into volts by multiplying by
0.00732. Which is what most analogue telemetry looks like: an ADC reading with
a scale factor attached.

So `Encoding().SizeInBits()` gives the wire width, and that is the number a
packet decoder needs. `Humanize` prints it for the same reason.

### Entry order is packet order

A container's EntryList is a list of mixed entry kinds, and the order they
appear in is the order they appear in the packet:

```xml
<SequenceContainer name="PowerPacket">
  <EntryList>
    <ContainerRefEntry containerRef="../Common"/>
    <ParameterRefEntry parameterRef="BusVoltage"/>
    <ParameterRefEntry parameterRef="BusCurrent"/>
  </EntryList>
</SequenceContainer>
```

Go's `encoding/xml` cannot keep that order across separate struct fields, give
it one slice per element name and the interleaving is lost. So `EntryList`
decodes itself, and every entry lands in one ordered `[]Entry`. It is the only
hand-written unmarshaller in the package, and this is why.

## What is deliberately not modeled

XTCE has hundreds of elements. This package covers the ones needed to describe
a packet layout and read it. The full list is in
[the coverage matrix](/conformance/xtce); the short version:

- **Algorithms** of every kind, math-operation calibrators, input and output
  algorithms. Evaluating an expression tree is a different job.
- **Alarms**: parsed as far as the schema allows, not modeled.
- **Streams**, **messages**, **services**.
- **Command semantics**: verifiers, transmission constraints, significance.
  Commands are a skeleton: names and argument types.
- **Array, aggregate and relative-time parameter types**: kept as named
  opaque entries, so references to them resolve and `TypeKind()` says what was
  found, but their contents stay raw and `Layout` refuses parameters of these
  types.

Elements this package does not model are not silently dropped where dropping
them would mislead. An unmodeled entry kind still occupies its place in an
EntryList, because removing it would make the surrounding entries look adjacent
when they are not. `IncludeCondition` and `CustomAlgorithm` are kept as raw XML
so a caller can parse them.

## No XSD validation

The Go standard library has no XSD validator and this package takes no
dependencies, so `Validate` means semantic checks written in Go, not schema
conformance. It covers the five faults that make a database unusable:

1. a reference that names nothing
2. a container inheriting in a circle
3. two things sharing a name in one SpaceSystem
4. a malformed reference
5. an encoding attribute (`encoding`, `bitOrder`, `byteOrder`) that is not
   one of the schema's enumeration members

A file that breaks the schema some other way will load and pass. If you need
real conformance, run `xmllint --schema SpaceSystem.xsd` over the file first.

One subtlety in the duplicate check: the schema's `parameterNameKey` selects
from `TelemetryMetaData/ParameterSet` **and** `CommandMetaData/ParameterSet`
together, so the two sides share one namespace and a name used on both
collides.

## Security

Go's `encoding/xml` does not fetch DTDs and does not expand external entities.
So the classic XML attacks do not apply here: no XXE, no entity expansion bomb,
no network callback from a document.

What remains is plain resource abuse, and two limits cover it:

- `MaxDocumentSize` (64 MiB) caps the input.
- `MaxDepth` (100) caps element nesting. The check runs as a token scan
  **before** decoding, because SpaceSystem is recursive: checking a decoded
  tree would mean the recursion had already happened.

`Validate` is also written to stay linear in the number of containers. It did
not start that way, the first version re-walked the tree to find each base
container's home, which made it cubic, and an 80 KB file took 200 ms. Since the
size cap allows files hundreds of times larger, that was a way to hang a
process with a document. `TestValidateScalesLinearly` guards the fix.

## Loading

```go
db, err := xtce.LoadFile("mission.xml")
if err != nil {
    return err
}
if err := db.Validate(); err != nil {
    log.Printf("database has problems:\n%v", err)
}
fmt.Println(db.Humanize())
```

Load and Validate are separate, and that matters. Load says the file is
well-formed XTCE. Validate says it is coherent, every reference resolves,
nothing inherits in a circle, no two things share a name, every encoding
attribute is a legal enumeration member. A database being edited usually has
references that do not resolve yet, and a loader that refused to read those
would be useless during authoring.

Load's errors say which of three things went wrong: `ErrMalformedXML` for
broken XML, `ErrNotSpaceSystem` for a document whose root is not an XTCE 1.2
SpaceSystem, and `ErrInvalidValue` for a real database with one unreadable
value in it. A `FixedValue` that is not a number, say. Fixed integers accept
every spelling the schema's `FixedIntegerValueType` allows: decimal, `0x` hex,
`0o` octal and `0b` binary.

Validate returns a `ValidationErrors` list rather than the first fault, because
someone repairing a database wants the whole list. `errors.Is` finds any
sentinel inside it:

```go
if errors.Is(err, xtce.ErrContainerCycle) {
    // ...
}

var problems xtce.ValidationErrors
if errors.As(err, &problems) {
    for _, problem := range problems {
        fmt.Println(problem.SpaceSystem, problem.Element, problem.Detail)
    }
}
```

## Name references

XTCE names things by path, and the paths behave the way file paths do:

| Form | Example | Resolves |
|---|---|---|
| absolute | `/Spacecraft/Power/BusVoltage` | from the root |
| relative | `../Power/BusVoltage` | from the referencing system |
| bare | `BusVoltage` | here, then each ancestor up to the root |

The bare form has the wrinkle. It is not just "in this SpaceSystem". The
search continues up the tree, which is what lets a mission define a type once
near the root and use it everywhere below. A path never searches upwards: it
says exactly where to look, so a miss is a miss.

Absolute references have a wrinkle of their own: files in the wild spell them
two ways. The schema's example writes the root system's name out (
`/Spacecraft/Power/BusVoltage`) but some tools treat `/` as already being the
root, so the first segment names one of its children:
`/Power/BusVoltage` for the same parameter. This package accepts both. When
the first segment matches the root's name it is read as the spelled-out form
and skipped; otherwise it is looked up among the root's children. The one
ambiguous case is a child of the root that shares the root's name, which the
spelled-out reading wins.

Two entry points, and the difference is worth knowing:

- `ResolveParameter(ref)` follows a **reference**, from the SpaceSystem you call
  it on, with all three forms above. It is what Validate uses.
- `FindParameter(qualifiedName)` follows a **full path** and nothing else. It is
  what you want when the name came from a display page or a configuration file.

## Worked example: the CCSDS primary header

`testdata/ccsds-header.xml` describes the six-octet Space Packet primary header
that `pkg/spp` implements, seven fields, 3 + 1 + 1 + 11 + 2 + 14 + 16 = 48
bits. Loading and printing it gives:

```
SpaceSystem /CCSDS
  7 parameters, 7 types, 1 containers, 0 commands
  Parameters
    Version -> Version_t (integer, 3 bits)
    PacketType -> PacketType_t (enumerated, 1 bits)
    SecondaryHeaderFlag -> SecondaryHeaderFlag_t (boolean, 1 bits)
    APID -> APID_t (integer, 11 bits)
    SequenceFlags -> SequenceFlags_t (enumerated, 2 bits)
    SequenceCount -> SequenceCount_t (integer, 14 bits)
    PacketLength -> PacketLength_t (integer, 16 bits)
  Containers
    PrimaryHeader (abstract), 7 entries
      ParameterRefEntry Version
      ParameterRefEntry PacketType
      ...
```

The container is marked abstract, because a primary header is never a packet on
its own, real packet types extend it through `BaseContainer`.

## Extracting packets

The point of a mission database is decoding real packets with it, and that is
what `Layout` and `Extract` do.

A `Layout` is a container flattened into the fields a packet of that shape
carries: inheritance worked through, referenced containers spliced in, and a
bit offset and width worked out for each field. It depends only on the
database, so build one per packet type at startup and reuse it.

```go
layout, err := db.LayoutOf("/Sat/Housekeeping")

packet, err := layout.Extract(octets)
for _, value := range packet.Values {
    fmt.Println(value)     // /Sat/Temp = 23.4
}

temp, ok := packet.Get("Temp")
degrees, ok := temp.Float()
```

Every value comes with both readings:

- `Raw` is what the packet carried, a `uint64`, `int64`, `float64`, `string`
  or `[]byte`, depending on the encoding.
- `Engineering` is what an operator should see: the calibrated number, the
  enumeration's label, the boolean's word.

Keeping both matters. An operator wants "23.4 °C" and an engineer chasing a
fault wants the count that produced it, and a system that stores one of them
cannot answer the other question later.

A field that cannot be decoded sets that value's `Err` and the rest of the
packet is still read, so one unsupported encoding in the middle does not hide
everything after it. `packet.Err()` returns the first failure for a caller who
wants all-or-nothing.

### Working out what a packet is

A ground station receives octets, not labelled packets. `Match` does the
search: start at the abstract container every packet extends, and follow each
derived container whose `RestrictionCriteria` the packet satisfies.

```go
base, err := db.FindContainer("/Sat/Packet")

packet, err := db.Match(base, octets)
fmt.Println(packet.Layout.Container.Name)   // DetailedHousekeeping
```

The deepest match wins, so a packet that is both "a telemetry packet" and "a
housekeeping telemetry packet" is read as the latter. `MatchFrom` returns just
the container when the answer you want is "what is this".

Three things to know.

**A container with no criteria never matches.** Inheriting without saying what
distinguishes you means nothing selects you, and treating that as "always true"
would make the first such container swallow every packet.

**A packet too short for the container is not a match.** A truncated packet can
satisfy a comparison on a field near the front while lacking most of the
container, and calling that a match would hand you a layout that cannot be
extracted.

**Comparisons run against the engineering value by default.** The schema's
`useCalibratedValue` defaults to `true`, so a comparison on an enumerated
parameter is against its label, `value="SCIENCE"`, not `value="1"`. Set the
attribute to `false` for the raw number.

Comparison values follow the schema's spelling: base ten unless the text starts
with `0x`, `0o` or `0b`, and truncated to the parameter's width only when they
do not already fit it.

### What extraction does not do

`Layout` refuses rather than guessing when the database makes a packet's shape
depend on the packet:

| Refused by `Layout` | Resolved by `ResolveLayout` |
|---|---|
| delimited or dynamically sized fields | yes, the packet states the width |
| `RepeatEntry` with a dynamic count | yes, the packet states the count |
| an entry positioned by a `DynamicValue` | yes |
| `referenceLocation="containerEnd"` | yes, against the packet length |

`Layout` settles a container once, ahead of any packet, which is what makes it cheap to build per packet type and reuse. When a container's shape depends on its own contents it returns `ErrDynamicSize`, and `ResolveLayout` is the other path: it walks the packet and the container together, decoding each field as it places it so a later field can be sized or positioned by an earlier one's value. The layout it returns describes that one packet.

These stay refused on both paths:

| Refused | Why |
|---|---|
| `RepeatEntry` with an `Offset` | the gap between repetitions is not modeled, and packing them without it would place them wrongly |
| `referenceLocation="nextEntry"` | it positions the *following* entry; treating it as `previousEntry` would silently misplace the field |
| a `LeadingSize` string | the width of the size field is an attribute of an element kept raw, so there is no way to know how far to skip |
| a `DiscreteLookupList` size, count or position | a table of comparisons rather than a single reference |
| a forward `DynamicValue` reference | one pass cannot read a field that has not arrived |
| entry kinds the model folds into `EntryOther` | their width is not modeled, so everything after them would be misplaced |

`Match` refuses a `CustomAlgorithm` in the criteria, and a `Comparison` or
`Condition` with a non-zero `instance`, rather than quietly reading them as
false, a criterion silently treated as false misroutes packets. An algorithm
is not in the file, and a non-zero instance is a value from another packet,
which one packet cannot answer.

Unsupported encodings are reported the same way: `MILSTD_1750A` and the decimal
float forms, and IEEE 754 widths other than 16, 32 and 64.

Splines are limited the same way. Order 1 is a straight line between points,
which is what a measured calibration curve is. Higher orders are refused
because the schema does not say which spline it means, and guessing at a curve
would put a wrong number in front of an operator. Outside the measured range
the value is clamped, unless `extrapolate` says to extend the end segment.

## Schema defaults

The XSD gives many attributes defaults, and `encoding/xml` knows nothing about
them. Rather than mutate the decoded tree, this package exposes accessors that
apply the default:

| Attribute | Default | Accessor |
|---|---|---|
| `IntegerParameterType/@sizeInBits` | 32 | `Size()` |
| `FloatParameterType/@sizeInBits` | 32 | `Size()` |
| `IntegerDataEncoding/@sizeInBits` | 8 | `Size()` |
| `IntegerDataEncoding/@encoding` | `unsigned` | `EncodingOrDefault()` |
| `FloatDataEncoding/@sizeInBits` | 32 | `Size()` |
| `FloatDataEncoding/@encoding` | `IEEE754_1985` | `EncodingOrDefault()` |
| `StringDataEncoding/@encoding` | `UTF-8` | `EncodingOrDefault()` |
| `@bitOrder` | `mostSignificantBitFirst` | `BitOrderOrDefault()` |
| `@byteOrder` | `mostSignificantByteFirst` | `ByteOrderOrDefault()` |
| `IntegerDataType/@signed` | `true` | `IsSigned()` |
| `@oneStringValue`, `@zeroStringValue` | `True`, `False` | `OneStringValueOrDefault()`, `ZeroStringValueOrDefault()` |
| `Encoding/@units` | `seconds` | `UnitsOrDefault()` |
| `Encoding/@scale`, `@offset` | 1, 0 | `ScaleOrDefault()`, `OffsetOrDefault()` |
| `Unit/@power` | 1 | `PowerOrDefault()` |
| `@referenceLocation` | `previousEntry` | `ReferenceLocationOrDefault()` |

Note the two sizeInBits defaults differ: a bare integer *type* is 32 bits wide
in software while its bare integer *encoding* is 8 bits on the wire.

`signed` is the one that needs care: `false` and absent are different, so the
field is a `*bool` and reading it directly is a mistake. `power` and
`changeThreshold` are pointers for the same reason: an absent power is not a
power of zero, and an absent threshold means any change is significant.

## Reference

- [XTCE 1.2](https://www.omg.org/spec/XTCE/), the OMG specification and its
  XSD. The target namespace is `http://www.omg.org/spec/XTCE/20180204`; the
  date is the schema's publication, not a version of its own.
- [CCSDS 660.1-G-2](https://public.ccsds.org/Pubs/660x1g2.pdf), the Green Book
  guide. Informational, and the clearest explanation of the semantics.
- [CLI](/cli/xtce) | [Conformance](/conformance/xtce) | [The stack](/docs/start/concepts)
