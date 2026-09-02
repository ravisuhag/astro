---
title: Packet Utilization Standard
short: PUS
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
| ST[08] | Function management | Tells a process to run one of its own functions |
| ST[11] | Time-based scheduling | Holds telecommands to release at a given time |
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

**Implemented.** Six services — ST[01] request verification, ST[03] housekeeping, ST[05] event reporting, ST[08] function management, ST[11] time-based scheduling, and ST[17] test — plus the PUS secondary headers, mission profiles, and both time fields.

**Not here yet.**

The standard defines twenty-plus services, and the rest are deliberate
follow-ups:

ST[02] device access, ST[04] parameter statistics, ST[06] memory management,
ST[09] time management, ST[12] on-board monitoring, ST[13] large packet
transfer, ST[14] real-time forwarding, ST[15] storage and retrieval, ST[18]
on-board control procedures, ST[19] event-action, ST[20] parameter management,
ST[21] request sequencing, ST[22] position-based scheduling, ST[23] file
management.

Also absent: the schedule itself. ST[11] gives you the wire format of all
twenty-seven message types; running the schedule — sub-schedule and group
state, the release window, interlocks between activities — is flight
software's job, and clause 6.11 is where its rules live.

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

## Function management, ST[08]

One message type: `TC[8,1]` tells an application process to run one of the
functions it declares. Everything interesting is outside the standard — which
functions exist, what their arguments mean, what running one does — so the
envelope is all there is.

The argument group is optional, and nothing in the message flags it. Clause
6.8.4c makes it conditional on the function taking arguments, so a function
that takes none carries no count field at all rather than a count of zero. The
decoder works this out from the body length, which is the one thing it can read
without the mission's function declarations.

Each argument value is "deduced" too, and that width really does come from the
mission. So the argument block travels verbatim, and `SplitArguments` splits it
against a width function you supply:

```go
args, err := request.Arguments.SplitArguments(profile, func(id uint64) (int, error) {
    return myFunctionDeclaration[id], nil
})
```

The count in the message is a claim to check, not a length to trust. A block
holding a different number of arguments is an error, because that means the two
ends disagree about the declaration.

The standard says missions should prefer their own service types, and that
ST[08] "remains in this version of the Standard for backward compatibility
reasons". It is here because packets carrying it still fly.

## Time-based scheduling, ST[11]

Twenty-seven message types, all implemented. The schedule holds telecommands
with a release time; the ground inserts them, shifts them, deletes them and
asks what is in there.

```go
request := pus.InsertActivitiesRequest{
    Profile:       profile,
    SubScheduleID: 3,
    Activities: []pus.ScheduledActivity{
        {GroupID: 7, ReleaseTime: releaseAt, Request: tcPacketBytes},
    },
}
```

The `Request` field is a whole CCSDS telecommand packet, primary header
included. Its length comes from the packet's own length field, which is what
makes a list of variable-length activities splittable at all.

### Two capabilities decide the layout

Clause 6.11.4.1 says whether a subservice supports sub-schedules and groups is
declared per mission. Those two declarations decide whether the sub-schedule ID
and group ID fields are present, so they are in the profile:

```go
profile.SupportsSubSchedules = true
profile.SupportsGroups = true
```

Get them wrong and every activity in a list is split at the wrong offset. There
is no flag in the message to fall back on.

### Filters

Four of the requests select activities by filter rather than by name: a time
window, then optionally a list of sub-schedules, then optionally a list of
groups. The three parts are intersected, not unioned (clause 6.11.10.2.5).

The window has four types, and which time tags travel depends on the type:

| Type | Meaning | Tags on the wire |
|---|---|---|
| `WindowSelectAll` | everything | none |
| `WindowFromTo` | between and including both | from, then to |
| `WindowFrom` | at and after the from tag | from only |
| `WindowTo` | before and at the to tag | **to only** |

That last row is the one to get right. `WindowTo` carries its tag in the *to*
slot with the from slot absent — clause 6.11.10.3c item (c) — not a single tag
in the first slot. A codec that put it first would encode a `WindowFrom`
message's bytes under a `WindowTo` type.

### An N of zero means all

`TC[11,20]`, `TC[11,21]`, `TC[11,23]`, `TC[11,24]` and `TC[11,25]` all say that
a count of zero applies to every sub-schedule or group. So an empty ID list is
not an empty request, and `IsAll()` says so rather than leaving you to
remember.

### Two different "request ID" fields

Figure 8-1 and Figure 8-92 both call their field a request ID and they are not
the same field. `pus.RequestID`, the ST[01] one, is a bit-packed 32-bit copy of
the CCSDS primary header. `pus.ScheduleRequestID`, the ST[11] one, carries a
source ID as well and uses whole octets at mission-declared widths.

Figure 8-1's own note explains why: its request ID "cannot be used to identify
the request since it does not contain the identifier of the source of that
request". Figure 8-92 fixes exactly that. The two are separate Go types
because using one where the other belongs produces wrong bytes and no error.

### Time offsets

A shift carries a relative time, which clause 7.3.11 makes signed — a negative
offset is the two's complement of the positive one, over the whole
coarse-and-fine field.

`RelativeTime` stores the field as ticks rather than a `time.Duration`, because
the two do not round-trip. Three fine octets resolve about 60 ns, and the
nearest whole nanosecond is a different number: a `Duration` would lose the low
bits and re-encode different octets. `Duration()` is there for arithmetic, not
for storage.

The same caution applies to the absolute time field, and there the loss is not
avoidable here. `pkg/tcf` truncates in both directions by design — rounding to
nearest can carry the fine field past its width — so a CUC field of two or
three fine octets can come back one tick lower than it went out. It matters if
you decode a scheduled release time and re-encode it: compare the octets, not
the `time.Time`.

## Reference

- [ECSS-E-ST-70-41C](https://ecss.nl/standard/ecss-e-st-70-41c-space-engineering-telemetry-and-telecommand-packet-utilization-15-april-2016/) — Telemetry and telecommand packet utilization
- [CCSDS 301.0-B-4](https://public.ccsds.org/Pubs/301x0b4e1.pdf) — Time Code Formats, for the CUC time field
- [Conformance](/conformance/pus)
