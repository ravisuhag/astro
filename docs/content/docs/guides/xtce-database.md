---
title: Decode from a mission database
short: Mission database
description: Put the packet layout in a file instead of in Go structs, and let the ground system read it.
order: 6
---

Every other guide here hand-writes a struct for each packet. That works, and it means the layout of your telemetry lives in Go code, compiled into one program. A real mission cannot do that. The layout has to be shared with the people who build the ground displays, the people who write the limit checks, and the people who will look at an archive in ten years.

[XTCE](/protocols/mission/xtce) is the file format for that, and this guide shows a ground system learning a mission from one.

The complete program is [`examples/xtce`](https://github.com/ravisuhag/astro/tree/main/examples/xtce). Run it:

```bash
go run ./examples/xtce/
```

## What we are building

```
mission.xml                       octets off the antenna
     │                                     │
     ▼                                     ▼
┌──────────┐   LayoutOf    ┌────────┐  Extract   ┌──────────────┐
│ Database │──────────────►│ Layout │───────────►│ 28.14 V      │
│          │               └────────┘            │ 4.2 A        │
│          │      Match (what is this?)          │ Mode Science │
│          │────────────────────────────────────►└──────────────┘
└──────────┘
```

No struct definitions. The Go program never mentions "bus voltage".

## Loading is not validating

```go
db, err := xtce.Load(file)
if err != nil {
    log.Fatalf("loading the database: %v", err)
}

if err := db.Validate(); err != nil {
    log.Fatalf("validating the database: %v", err)
}
```

Two steps on purpose. `Load` says the file is well-formed XTCE. `Validate` says it is coherent: the references resolve, inheritance does not loop, names do not collide.

A database being edited usually has references that do not resolve yet, and a loader that refused to parse those would be useless while you are writing one. So load during authoring and validate before flight.

`pkg/xtce` has no `Encode`. It reads databases and does not write them, because writing them is a database editor's job and doing it properly means a round-trip fidelity this package does not need.

## Describe the header once

The interesting part of the file is that the CCSDS primary header appears once:

```xml
<SequenceContainer name="PrimaryHeader" abstract="true">
  <EntryList>
    <ParameterRefEntry parameterRef="Version"/>
    <ParameterRefEntry parameterRef="PacketType"/>
    <ParameterRefEntry parameterRef="SecondaryHeaderFlag"/>
    <ParameterRefEntry parameterRef="APID"/>
    <ParameterRefEntry parameterRef="SequenceFlags"/>
    <ParameterRefEntry parameterRef="SequenceCount"/>
    <ParameterRefEntry parameterRef="PacketLength"/>
  </EntryList>
</SequenceContainer>
```

`abstract="true"` means no packet is ever just a header, so this container is only ever inherited from. Then each real packet type extends it and says which APID picks it:

```xml
<SequenceContainer name="PowerReport">
  <EntryList>
    <ParameterRefEntry parameterRef="BusVoltage"/>
    <ParameterRefEntry parameterRef="BusCurrent"/>
    <ParameterRefEntry parameterRef="Mode"/>
  </EntryList>
  <BaseContainer containerRef="PrimaryHeader">
    <RestrictionCriteria>
      <Comparison parameterRef="APID" value="100"/>
    </RestrictionCriteria>
  </BaseContainer>
</SequenceContainer>
```

`RestrictionCriteria` is the part worth slowing down on. Without it, inheritance is just a way of sharing a header. With it, the base container becomes a decision: given these octets, which reading is the right one?

## Calibration is the payoff

A spacecraft sends counts. An operator needs volts. That conversion belongs in the database, and it is the single best reason to have one:

```xml
<FloatParameterType name="Volts_t" sizeInBits="32">
  <UnitSet><Unit description="volts">V</Unit></UnitSet>
  <IntegerDataEncoding encoding="unsigned" sizeInBits="16">
    <DefaultCalibrator>
      <PolynomialCalibrator>
        <Term coefficient="0" exponent="0"/>
        <Term coefficient="0.0005" exponent="1"/>
      </PolynomialCalibrator>
    </DefaultCalibrator>
  </IntegerDataEncoding>
</FloatParameterType>
```

Sixteen unsigned bits on the wire, half a millivolt each, and a 32-bit float to the user. When the sensor is recalibrated you edit the file, not the ground software.

A polynomial fits a linear or near-linear sensor. For one that bends, use a spline and give it the points you measured:

```xml
<SplineCalibrator order="1">
  <SplinePoint raw="0" calibrated="0.0"/>
  <SplinePoint raw="32768" calibrated="5.0"/>
  <SplinePoint raw="65535" calibrated="9.5"/>
</SplineCalibrator>
```

Enumerations and booleans work the same way. `Mode` is 8 bits on the wire and the word `Science` to a human.

## Layout, then extract

A layout is the container flattened: inheritance worked through, and a bit offset and width settled for every field.

```go
layout, err := db.LayoutOf("/Demosat/PowerReport")
```

It depends only on the database, so build it once per packet type at startup and reuse it for every packet. Here is what it works out:

```
  10 fields, 88 bits

  bit   0   3 wide  /Demosat/Version
  bit   3   1 wide  /Demosat/PacketType
  bit   4   1 wide  /Demosat/SecondaryHeaderFlag
  bit   5  11 wide  /Demosat/APID
  bit  16   2 wide  /Demosat/SequenceFlags
  bit  18  14 wide  /Demosat/SequenceCount
  bit  32  16 wide  /Demosat/PacketLength
  bit  48  16 wide  /Demosat/BusVoltage
  bit  64  16 wide  /Demosat/BusCurrent
  bit  80   8 wide  /Demosat/Mode
```

Ten fields, and the file only listed three. The other seven came from the base container.

Then read a packet against it:

```go
packet, err := layout.Extract(power)
if err := packet.Err(); err != nil {
    fmt.Printf("  warning: %v\n", err)
}

voltage, ok := packet.Get("BusVoltage")
volts, _ := voltage.Float()
```

`Extract` returning no error does not mean every field decoded. It keeps going past a bad field, so one unsupported encoding in the middle of a packet does not hide everything after it. `Err()` is how you ask, and each `Value` has its own `Err` too.

Every value carries both forms. `Raw` is what the packet held, `Engineering` is what it means:

```
  BusVoltage raw 56280 becomes 28.140 V
```

## Matching an unknown packet

A ground station pulling packets off an antenna does not know what each one is. That is what `Match` is for:

```go
root, err := db.FindContainer("/Demosat/PrimaryHeader")
matched, err := db.Match(root, unknown)

fmt.Println(matched.Layout.Container.Name)
```

It starts at the container you name, follows every child whose `RestrictionCriteria` the packet satisfies, and takes the deepest one that fits.

```
  11 octets matched PowerReport
    APID           100
    BusVoltage     28.14
    BusCurrent     4.1999817
    Mode           Science

  11 octets matched ThermalReport
    APID           101
    RadiatorTemp   -12.75
    BatteryTemp    21.3
    HeaterState    On
```

Same length, same header, and the database told them apart from the APID alone.

Note `4.1999817` rather than `4.2`. A count of 27524 through that spline does not land on a round number of amperes, and the value is honest about it. Sensors are quantised; a display should round, a decoder should not.

## Things that will bite you

**There is no XSD validation.** The Go standard library has no XSD validator and this package takes no dependencies, so `Validate` runs semantic checks written in Go rather than checking the schema. A file that breaks the XSD in a way those checks miss will load. Run `xmllint` over it first if that matters.

**A container does not have to cover the packet.** `layout.BitSize` is the smallest packet it can be read from, not the packet length. A longer packet is fine and XTCE does not require every bit be described.

**Fields are in packet order, not offset order.** An entry placed with `LocationInContainerInBits` can point backwards. If you are walking `layout.Fields` and assuming offsets increase, you will be wrong on the databases that use that feature.

**`Match` needs restriction criteria to work with.** A database whose containers all inherit without restricting cannot be matched against, because nothing distinguishes them. Getting a match you did not expect usually means two criteria overlap.

**Qualified names, not bare ones.** `LayoutOf("/Demosat/PowerReport")` needs the path. `Get("BusVoltage")` takes the short name, because a packet has already narrowed the namespace.

## Next

- [Build a PUS service model](/docs/guides/pus-services), which is where the parameter table this replaces was hand-written
- [Debug a real capture](/docs/guides/debug-a-capture), the same work from a terminal
- [XTCE protocol page](/protocols/mission/xtce) | [Conformance](/conformance/xtce) | [CLI](/cli/xtce)
