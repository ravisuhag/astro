---
title: Build a PUS service model
short: PUS services
description: The layer above packets, where a mission says what its telemetry and telecommands mean.
order: 5
---

The [downlink](/docs/guides/downlink) and [uplink](/docs/guides/uplink) guides move bytes. They never say what the bytes mean. [PUS](/protocols/mission/pus) is the layer that does: a small header inside every Space Packet naming a service and a subtype, and a body that pair defines.

This walks five services working together. The ground enables a housekeeping report, sets a limit on the battery, and schedules a command for later. The spacecraft acknowledges, reports, notices the battery going low, and raises an event.

The complete program is [`examples/pus`](https://github.com/ravisuhag/astro/tree/main/examples/pus). Run it:

```bash
go run ./examples/pus/
```

## What we are building

```
Ground                                              Spacecraft
──────                                              ──────────
TC[3,5]  enable housekeeping ──────────────────►  ST[03] starts reporting
TC[12,5] watch the battery ────────────────────►  ST[12] starts checking
TC[11,4] release this later ───────────────────►  ST[11] holds it

         ◄────────────────────  TM[1,1]  accepted        ST[01]
         ◄────────────────────  TM[1,7]  completed       ST[01]
         ◄────────────────────  TM[3,25] housekeeping    ST[03]
         ◄────────────────────  TM[12,12] out of limits  ST[12]
         ◄────────────────────  TM[5,2]  low battery     ST[05]
```

Five services. Every message is a real Space Packet with a PUS secondary header.

## The mission profile comes first

PUS is a tailoring standard. It leaves dozens of field widths to the mission rather than fixing them, so there is no such thing as decoding a PUS packet without knowing the mission. Every codec in `pkg/pus` takes a profile, and there is no default hiding anywhere:

```go
profile := pus.DefaultProfile()
profile.TCSpareBytes = 0
profile.TMSpareBytes = 0
profile.PerDefinitionMonitoringInterval = true
```

`DefaultProfile` fills in the widths most missions pick. The interesting part is the flags, because they are not widths at all. `PerDefinitionMonitoringInterval` decides whether a monitoring definition carries its own interval. Set it on one end and not the other, and every field after the interval reads at the wrong offset. Nothing on the wire will tell you.

Call `Validate` once at startup:

```go
if err := profile.Validate(); err != nil {
    log.Fatalf("mission profile: %v", err)
}
```

## The registry decodes what arrives

A PUS receiver does not know what is coming. It reads the service and subtype out of the secondary header, then looks up the codec:

```go
registry, err := pus.NewDefaultRegistry(profile,
    pus.WithParameterResolver(parameters))
```

`NewDefaultRegistry` registers every service this package implements. Anything unregistered comes back as `ErrUnknownMessageType` rather than being guessed at, which matters because PUS reserves ranges for missions to define their own services.

## Some widths are not in the profile either

ST[12] compares a parameter against a limit. To read the limit it needs to know how wide the parameter's value is, and nothing in the message says. That comes from the mission's parameter table:

```go
func parameters(id uint64) (pus.ParameterLayout, error) {
    switch id {
    case paramBattery:
        return pus.ParameterLayout{ValueBytes: 2, MaskBytes: 2}, nil
    default:
        return pus.ParameterLayout{}, fmt.Errorf("unknown parameter %#x", id)
    }
}
```

That is the same table [XTCE](/docs/guides/xtce-database) holds for a real mission. Here it is two lines because there is one parameter.

## Sending a request

A PUS telecommand is a Space Packet whose secondary header is a `pus.TCHeader`. Because that type implements `spp.SecondaryHeader`, the two packages compose without either knowing about the other:

```go
header := profile.NewTCHeader(service, subtype, groundSourceID,
    pus.AckAcceptance|pus.AckCompletion)

packet, err := spp.NewTCPacket(apidCommanding, body,
    spp.WithSecondaryHeader(header),
    spp.WithSequenceCount(seq),
    spp.WithErrorControl(),
)
```

The two ack flags are what ask for the reports. There are four of them, acceptance, start, progress and completion, and a telecommand that sets none is executed silently. Asking for all four on routine traffic is a good way to fill your downlink with acknowledgements.

The body is whatever the service and subtype define. For "start reporting":

```go
enable := &pus.HousekeepingControlRequest{
    Profile:      profile,
    Subtype:      pus.SubtypeEnableHKGeneration,
    StructureIDs: []uint64{hkStructureID},
}
```

## Reading one back

The receiving end hands an empty header to the decoder and lets it fill in:

```go
header := &pus.TCHeader{Profile: profile}
packet, err := spp.Decode(encoded,
    spp.WithDecodeSecondaryHeader(header),
    spp.WithDecodeErrorControl(),
)

request, err := registry.DecodeRequest(header.Key(), packet.UserData)
```

`header.Key()` is the `TC[3,5]` pair. `DecodeRequest` turns the body into a typed value.

`WithDecodeErrorControl` is easy to forget. Without it the two CRC octets stay on the end of `UserData`, and a fixed-size PUS body then fails to decode with "trailing octets". The error is clear once you have seen it and baffling the first time.

## Verifying a telecommand

ST[01] is the service that answers "did my command work?". A verification report names the telecommand it concerns by repeating the first two words of that command's primary header:

```go
report := &pus.VerificationReport{
    Profile:   profile,
    Subtype:   pus.SubtypeAcceptSuccess,
    RequestID: requestID(commandHeader),
}
```

Note what that means for the spacecraft: it has to keep the command's primary header around until it has finished reporting on it. Acceptance comes in milliseconds, completion can come minutes later.

The request ID does not name who sent the command. That comes from the destination ID in the report's own header.

## Housekeeping, monitoring, and events

`TM[3,25]` carries a structure ID and then the sampled values back to back:

```go
housekeeping := &pus.HousekeepingReport{
    Profile:         profile,
    StructureID:     hkStructureID,
    ParameterValues: millivolts(28.1),
}
```

The values are moved verbatim. Their layout comes from the report structure, which both ends already agreed on when `TC[3,1]` created it, so putting the layout on the wire again would be a chance to disagree.

ST[12] watches a parameter and reports when its status changes:

```go
transition := pus.CheckTransitionReport{
    Profile: profile,
    Resolve: parameters,
    Subtype: pus.SubtypeCheckTransitionReport,
    Transitions: []pus.CheckTransition{{
        PMONID:                 pmonBattery,
        MonitoredParameterID:   paramBattery,
        CheckType:              pus.CheckLimit,
        ParameterValue:         millivolts(25.4),
        LimitCrossed:           millivolts(26.0),
        PreviousCheckingStatus: pus.PMONNominal,
        CurrentCheckingStatus:  pus.PMONBelowLowLimit,
        TransitionTime:         now,
    }},
}
```

The `RepetitionNumber` on the definition is why this is a transition and not a twitch: three consecutive checks have to agree before the status changes.

The limit check named an event definition ID, and that is the link to ST[05]. ST[12] detects, ST[05] announces:

```go
event := &pus.EventReport{
    Profile:           profile,
    Severity:          pus.Severity(pus.SubtypeLowSeverity),
    EventDefinitionID: eventLowBatt,
    AuxiliaryData:     millivolts(25.4),
}
```

## Scheduling a command for later

ST[11] holds telecommands on board and releases them at a time. A scheduled activity carries a **whole telecommand packet**, primary header included:

```go
schedule := pus.InsertActivitiesRequest{
    Profile:       profile,
    SubScheduleID: 1,
    Activities: []pus.ScheduledActivity{{
        GroupID:     1,
        ReleaseTime: time.Date(2026, 3, 14, 10, 0, 0, 0, time.UTC),
        Request:     later,
    }},
}
```

So this is a command wrapped in a command. The length of the inner one comes from its own length field, not from anything in the ST[11] message, which is why a malformed inner packet breaks the whole list rather than one entry.

## Running it

```
--- Spacecraft Side: decoding requests ---

  [3,5] accepted from APID 100
    PUS TC[3,5] enable periodic generation of housekeeping report structures
      Structures ... 1
  [12,5] accepted from APID 100
    PUS TC[12,5] add parameter monitoring definitions
      Definitions ... 1
        PMON 16 on parameter 257: limit-checking
  [11,4] accepted from APID 100
    PUS TC[11,4] insert activities into the time-based schedule
      Sub-schedule .. 1
      Activities .... 1
        release 2026-03-14T10:00:00Z, group 1, 15 octet request

--- Ground Side: decoding reports ---

  [1,1] from APID 200 at 2026-03-14T09:26:53Z
    PUS TM[1,1] verification report
      Request APID .. 100
      Sequence ...... 0
  [1,7] from APID 200 at 2026-03-14T09:26:55Z
    PUS TM[1,7] verification report
      Request APID .. 100
      Sequence ...... 0
  [3,25] from APID 200 at 2026-03-14T09:26:57Z
    PUS TM[3,25] housekeeping parameter report
      Structure ID .. 1
      Values ........ 2 octets
  [12,12] from APID 200 at 2026-03-14T09:27:07Z
    PUS TM[12,12] check transition report
      Transitions ... 1
        PMON 16: within limits -> below low limit at 2026-03-14T09:27:07Z
  [5,2] from APID 200 at 2026-03-14T09:27:07Z
    PUS TM[5,2] low severity anomaly
      Event ID ....... 8193
      Auxiliary data . 2 octets
```

`Humanize` prints any decoded message. It is the fastest way to see whether a capture means what you think.

## Things that will bite you

**The profile is part of the wire format.** Two missions with different profiles cannot read each other's packets, and the failure is silent field shifting rather than an error. Keep one profile value, build it once, and never mutate it after a codec has seen it.

**The capability flags shift fields.** `SupportsSubSchedules`, `SupportsGroups`, `SupportsConditionalChecking`, `PerDefinitionMonitoringInterval` and their siblings decide whether a field exists. They are worse than a wrong width, because a wrong width usually fails loudly and a missing field just slides everything along.

**Forgetting `WithDecodeErrorControl` breaks fixed-size bodies.** You get "trailing octets after a fixed-size PUS message body", which does not sound like a CRC problem.

**PUS does not frame anything.** These are Space Packets. Getting them to the ground is still [TM frames and CADUs](/docs/guides/downlink). PUS sits above that whole stack and is unaware of it.

**The parameter resolver is not optional for ST[12].** A registry built without one decodes ST[01], ST[03] and ST[05] fine, then fails on the first monitoring message. Register it even if the table has one entry.

## Next

- [Decode from a mission database](/docs/guides/xtce-database), where the parameter table comes from
- [Build a downlink](/docs/guides/downlink), how these packets reach the ground
- [PUS protocol page](/protocols/mission/pus) | [Conformance](/conformance/pus) | [CLI](/cli/pus)
