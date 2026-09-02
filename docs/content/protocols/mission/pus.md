---
title: Packet Utilization Standard
description: ECSS-E-ST-70-41C — what goes inside a telemetry or telecommand packet.
order: 70
---

> **ECSS-E-ST-70-41C (PUS-C)** · [Standard](https://ecss.nl/standard/ecss-e-st-70-41c-space-engineering-telemetry-and-telecommand-packet-utilization-15-april-2016/) · [`pkg/pus`](https://github.com/ravisuhag/astro/tree/main/pkg/pus)

## Overview

PUS says what goes *inside* a space packet. Where `pkg/spp` gives you a packet
with an application-defined payload, PUS defines that payload: a secondary
header naming a **service** and a **subtype**, followed by the request or
report that pair implies.

Nearly every ESA-heritage mission speaks it. Without PUS you can frame and
packetize, but you cannot build a single real telecommand.

Services are numbered. A few you will meet constantly:

| Service | Name | What it does |
|---|---|---|
| ST[01] | Request verification | Reports whether your telecommand was accepted, started, progressed, completed |
| ST[03] | Housekeeping | Defines periodic parameter reports and carries their values |
| ST[05] | Event reporting | On-board events at four severities |
| ST[17] | Test | "Are you alive?" |

A message type is written `TC[3,1]` for a telecommand and `TM[1,2]` for
telemetry: service 3 subtype 1, service 1 subtype 2.

### Where PUS fits

```
┌─────────────────────────────────────────────┐
│  Mission operations (commanding, telemetry) │
├─────────────────────────────────────────────┤
│  PUS — services, requests, reports          │  ← this package
│  secondary header │ application data        │
├─────────────────────────────────────────────┤
│  Space Packet Protocol (pkg/spp)            │  ← carries it
├─────────────────────────────────────────────┤
│  TM / TC / AOS / USLP Transfer Frame        │
└─────────────────────────────────────────────┘
```

## Scope

**Implemented.** Four services — ST[01] request verification, ST[03] housekeeping, ST[05] event reporting, and ST[17] test — plus the PUS secondary headers, mission profiles, and the time field.

**Not here yet.**

This package ships four services. The standard defines twenty-plus, and the
rest are deliberate follow-ups:

ST[02] device access, ST[04] parameter statistics, ST[06] memory management,
ST[08] function management, ST[09] time management, ST[11] time-based
scheduling, ST[12] on-board monitoring, ST[13] large packet transfer, ST[14]
real-time forwarding, ST[15] storage and retrieval, ST[18] on-board control
procedures, ST[19] event-action, ST[20] parameter management, ST[21] request
sequencing, ST[22] position-based scheduling, ST[23] file management.

Also absent: on-board scheduling semantics of any kind, and CLI subcommands.

## Mission profiles

This is the first thing to understand about PUS, and the thing that catches
people out.

**PUS is a tailoring standard.** Several field widths are declared per mission
rather than fixed by the text. Two spacecraft can both speak valid PUS-C and be
completely unable to read each other's packets, because one uses a two-octet
event ID and the other uses four.

So every codec here takes a `MissionProfile`. There is no package-level default
and no implicit fallback: you pass one, explicitly, every time.

```go
profile := pus.MissionProfile{
    TimeFormat:                   pus.TimeCUC,
    CUCCoarseBytes:               4,
    CUCFineBytes:                 2,
    StepIDBytes:                  2,
    FailureCodeBytes:             2,
    EventDefinitionIDBytes:       2,
    HousekeepingStructureIDBytes: 1,
    ParameterIDBytes:             2,
    CollectionIntervalBytes:      4,
    CountBytes:                   1,
    APIDBytes:                    2, // ST[17] APID width; zero also means 2
}
if err := profile.Validate(); err != nil {
    log.Fatal(err)
}
```

Two optional profile fields help catch mistakes early. `APIDBytes` sizes the
APID field of TC[17,3] and TM[17,4], which the standard marks enumerated
without a width; leaving it zero keeps the two-octet width almost every
mission uses. `WordSizeBytes` declares the mission word size: when it is
non-zero, `Validate` refuses a profile whose secondary headers are not a
whole number of words, so a wrong spare-byte count fails at setup rather
than on the wire. Zero skips the check.

`pus.DefaultProfile()` returns the widths most European missions pick. It is a
convenience for tooling and tests — **not** a standard-mandated default. The
standard states none.

### What is *not* tailorable

Worth knowing, because plenty of documentation gets this wrong. Figures 7-7 and
7-9 give these explicit bit counts, so they are constants here, not profile
fields:

- TC source ID — **16 bits**
- TM message type counter — **16 bits**
- TM destination ID — **16 bits**

## Building a telecommand

```go
import (
    "github.com/ravisuhag/astro/pkg/pus"
    "github.com/ravisuhag/astro/pkg/spp"
)

header := profile.NewTCHeader(
    pus.ServiceTest,          // 17
    pus.SubtypeAreYouAlive,   // 1
    sourceID,
    pus.AckAcceptance|pus.AckCompletion,
)

body, _ := pus.AreYouAliveRequest{}.Encode()

packet, err := spp.NewTCPacket(apid, body, spp.WithSecondaryHeader(header))
```

That is the whole integration. `TCHeader` implements `spp.SecondaryHeader`, so
neither package needs to know about the other.

### Acknowledgement flags

The four bits in a TC header ask the spacecraft for ST[01] reports back:

| Flag | Bit | You get back |
|---|---|---|
| `AckAcceptance` | 3 | TM[1,1] on success, TM[1,2] on failure |
| `AckStart` | 2 | TM[1,3] / TM[1,4] |
| `AckProgress` | 1 | TM[1,5] / TM[1,6] |
| `AckCompletion` | 0 | TM[1,7] / TM[1,8] |

The bit positions are fixed by clause 7.4.4.1d. Getting them backwards means
asking for the wrong reports, which is why there is a test pinning each one.

## Reading telemetry

```go
header := &pus.TMHeader{Profile: profile}
packet, err := spp.Decode(raw, spp.WithDecodeSecondaryHeader(header))
if err != nil {
    return err
}

registry, _ := pus.NewDefaultRegistry(profile)

report, err := registry.DecodeReport(header.Key(), packet.UserData)
if err != nil {
    return err // unknown message type, or a malformed body
}

switch r := report.(type) {
case *pus.EventReport:
    log.Printf("event %d, %s", r.EventDefinitionID, r.Severity)
case *pus.VerificationReport:
    log.Printf("command %d %s", r.RequestID.SequenceCount, r.Key())
}
```

The registry maps message types to codecs. Anything unregistered returns
`ErrUnknownMessageType` rather than being guessed at — which matters, because
PUS lets missions define their own services in the ranges the standard leaves
open.

## The time field

The TM secondary header carries a time tag whose format the mission declares
(clause 7.4.3.1j). Table 7-10 lists the codes, and there is a detail here that
is easy to get wrong.

**PFC 3 to 46: the P-field is implicit.** The time field carries the coarse and
fine octets alone. The PFC already says how wide they are, so repeating that in
a P-field would be redundant. This is `pus.TimeCUC`, and it is what almost
everyone uses.

**PFC 0: the P-field is explicit** and travels with the value. This is
`pus.TimeCUCExplicit`.

The two differ by exactly one octet on the wire. Pick the wrong one and every
field after the time shifts.

```go
profile.TimeFormat = pus.TimeCUC       // implicit, coarse+fine only
profile.TimeFormat = pus.TimeCUCExplicit // explicit, with P-field
profile.TimeFormat = pus.TimeRaw       // opaque, mission-defined elsewhere
profile.TimeFormat = pus.TimeNone      // no time field at all
```

An agency-defined epoch works too — set `CUCEpoch`, and the CUC time code level
follows automatically.

## Request verification, ST[01]

Nine reports tracking a telecommand through acceptance, start, progress, and
completion: four success and four failure pairs, TM[1,1] to TM[1,8], plus
TM[1,10], the failed routing verification report a node sends when it cannot
route a request onward.

Every one carries a **request ID** naming the command it concerns. That ID is
laid out as exactly the first four octets of a CCSDS primary header (Figure
8-1), so it is built straight from the packet you sent.

Note what it does *not* contain: the source of the request. As the standard
points out, that comes from the destination ID of the report's own secondary
header.

The odd subtypes are successes, the even ones failures — TM[1,10] included,
whose body is a request ID and a failure notice like TM[1,2]. Only subtypes 5
and 6, the progress reports, carry a step ID. There is no TM[1,9]; the decoder
rejects it.

## Housekeeping, ST[03]

`TC[3,1]` defines a report structure: which parameters to sample, how often,
and which of them to super-commutate (sample several times per interval, for
values that change faster than the report rate).

`TM[3,25]` carries the sampled values.

**This package does not sample anything.** It frames the structure definitions
and the reports; the values are bytes the caller supplies and interprets, since
only the flight software knows what a parameter means.

## Event reporting, ST[05]

Four report subtypes, one per severity: informative, low, medium, high. Each
carries an event definition ID and optional auxiliary data whose shape that ID
implies.

`TC[5,5]` and `TC[5,6]` enable and disable generation for a list of events.
`TC[5,7]` — an empty-bodied request — asks which events are disabled, and
`TM[5,8]` answers with the list.

One decoding rule worth knowing: messages whose size is fully determined by
their structure are checked exactly. A body with octets left over decodes to
`ErrTrailingBytes` rather than being silently truncated, matching the PUS
acceptance checks. Bodies that end in caller-interpreted data — failure data,
auxiliary data, parameter values — carry those octets verbatim by design.

## Reference

- [ECSS-E-ST-70-41C](https://ecss.nl/standard/ecss-e-st-70-41c-space-engineering-telemetry-and-telecommand-packet-utilization-15-april-2016/) — Telemetry and telecommand packet utilization
- [CCSDS 301.0-B-4](https://public.ccsds.org/Pubs/301x0b4e1.pdf) — Time Code Formats, for the CUC time field
- [Conformance](/conformance/pus)
