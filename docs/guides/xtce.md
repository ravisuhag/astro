# XML Telemetric and Command Exchange

> XTCE 1.2 (OMG) — with CCSDS 660.1-G-2 as the guide to it

## Overview

Every other package in this library moves bytes. This one moves none.

A telemetry frame arrives and `pkg/tmdl` takes it apart. Inside is a packet,
and `pkg/spp` takes that apart too. What comes out is a run of octets that
means something — a bus voltage, a thruster state, a clock reading — and
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
files routinely — NASA, ESA, Yamcs and the commercial operators all read and
write them — so being able to load one is what connects this library's
decoders to a real mission.

## What this package does

Four things:

| | |
|---|---|
| `Load`, `LoadFile` | parse a database |
| `Validate` | check that it hangs together |
| `FindParameter`, `ResolveParameter`, `Walk` | navigate it |
| `Humanize` | print it |

There is no `Encode`. Writing XTCE back out is a decision made against, not an
omission: this package reads databases, and databases are written by editors.
A writer would mean committing to a round-trip fidelity nothing here needs.

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
well-formed XTCE. Validate says it is coherent — every reference resolves,
nothing inherits in a circle, no two things share a name. A database being
edited usually has references that do not resolve yet, and a loader that
refused to read those would be useless during authoring.

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

- **ParameterTypeSet** — the types. A type says what a value *is* and how it is
  written on the wire.
- **ParameterSet** — the parameters. A parameter is little more than a name and
  a pointer to its type.
- **ContainerSet** — the containers. A container is a packet layout: an ordered
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

Go's `encoding/xml` cannot keep that order across separate struct fields — give
it one slice per element name and the interleaving is lost. So `EntryList`
decodes itself, and every entry lands in one ordered `[]Entry`. It is the only
hand-written unmarshaller in the package, and this is why.

## Name references

XTCE names things by path, and the paths behave the way file paths do:

| Form | Example | Resolves |
|---|---|---|
| absolute | `/Spacecraft/Power/BusVoltage` | from the root |
| relative | `../Power/BusVoltage` | from the referencing system |
| bare | `BusVoltage` | here, then each ancestor up to the root |

The bare form has the wrinkle. It is not just "in this SpaceSystem" — the
search continues up the tree, which is what lets a mission define a type once
near the root and use it everywhere below. A path never searches upwards: it
says exactly where to look, so a miss is a miss.

Two entry points, and the difference is worth knowing:

- `ResolveParameter(ref)` follows a **reference**, from the SpaceSystem you call
  it on, with all three forms above. It is what Validate uses.
- `FindParameter(qualifiedName)` follows a **full path** and nothing else. It is
  what you want when the name came from a display page or a configuration file.

## Worked example: the CCSDS primary header

`testdata/ccsds-header.xml` describes the six-octet Space Packet primary header
that `pkg/spp` implements — seven fields, 3 + 1 + 1 + 11 + 2 + 14 + 16 = 48
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
its own — real packet types extend it through `BaseContainer`.

## What is deliberately not modeled

XTCE has hundreds of elements. This package covers the ones needed to describe
a packet layout and read it. The full list is in
[the coverage matrix](../pics/xtce-coverage.md); the short version:

- **Algorithms** of every kind — math-operation calibrators, input and output
  algorithms. Evaluating an expression tree is a different job.
- **Alarms** — parsed as far as the schema allows, not modeled.
- **Streams**, **messages**, **services**.
- **Command semantics** — verifiers, transmission constraints, significance.
  Commands are a skeleton: names and argument types.
- **Array and aggregate parameter types.**

Elements this package does not model are not silently dropped where dropping
them would mislead. An unmodeled entry kind still occupies its place in an
EntryList, because removing it would make the surrounding entries look adjacent
when they are not. `RestrictionCriteria` and `IncludeCondition` are kept as raw
XML so a caller can parse them.

## No XSD validation

The Go standard library has no XSD validator and this package takes no
dependencies, so `Validate` means semantic checks written in Go, not schema
conformance. It covers the four faults that make a database unusable:

1. a reference that names nothing
2. a container inheriting in a circle
3. two things sharing a name in one SpaceSystem
4. a malformed reference

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
not start that way — the first version re-walked the tree to find each base
container's home, which made it cubic, and an 80 KB file took 200 ms. Since the
size cap allows files hundreds of times larger, that was a way to hang a
process with a document. `TestValidateScalesLinearly` guards the fix.

## This is the layer under an extraction engine

The point of a mission database is decoding real packets with it: take a
`pkg/spp` packet, walk a container's EntryList, and pull each parameter out bit
by bit. That engine is not in this package and needs its own design.

What this package owes it, and keeps:

| The engine needs | Where it is |
|---|---|
| entry order | `EntryList.Entries`, in document order |
| bit widths | `ParameterType.Encoding().SizeInBits()` |
| explicit positions | `Entry.LocationInContainerInBits` |
| repeats | `Entry.RepeatEntry` |
| bit and byte order | `BitOrderOrDefault()`, `ByteOrderOrDefault()` |
| signedness and encoding | `IsSigned()`, `EncodingOrDefault()` |
| calibrators | `DefaultCalibrator`, polynomial and spline |
| time epochs | `AbsoluteTimeParameterType.ReferenceTime`, for `pkg/tcf` |
| inheritance | `BaseContainer`, with `RestrictionCriteria` kept raw |

`testdata/ccsds-header.xml` is its first test vector.

## Schema defaults

The XSD gives many attributes defaults, and `encoding/xml` knows nothing about
them. Rather than mutate the decoded tree, this package exposes accessors that
apply the default:

| Attribute | Default | Accessor |
|---|---|---|
| `IntegerDataEncoding/@sizeInBits` | 8 | `Size()` |
| `IntegerDataEncoding/@encoding` | `unsigned` | `EncodingOrDefault()` |
| `FloatDataEncoding/@sizeInBits` | 32 | `Size()` |
| `FloatDataEncoding/@encoding` | `IEEE754_1985` | `EncodingOrDefault()` |
| `StringDataEncoding/@encoding` | `UTF-8` | `EncodingOrDefault()` |
| `@bitOrder` | `mostSignificantBitFirst` | `BitOrderOrDefault()` |
| `@byteOrder` | `mostSignificantByteFirst` | `ByteOrderOrDefault()` |
| `IntegerDataType/@signed` | `true` | `IsSigned()` |
| `Encoding/@scale`, `@offset` | 1, 0 | `ScaleOrDefault()`, `OffsetOrDefault()` |

`signed` is the one that needs care: `false` and absent are different, so the
field is a `*bool` and reading it directly is a mistake.

## Reference

- [XTCE 1.2](https://www.omg.org/spec/XTCE/) — the OMG specification and its
  XSD. The target namespace is `http://www.omg.org/spec/XTCE/20180204`; the
  date is the schema's publication, not a version of its own.
- [CCSDS 660.1-G-2](https://public.ccsds.org/Pubs/660x1g2.pdf) — the Green Book
  guide. Informational, and the clearest explanation of the semantics.
- [Coverage matrix](../pics/xtce-coverage.md) — element by element.
